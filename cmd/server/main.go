// Command server is drop-tracker's single process entrypoint. It runs the
// HTTP API and the robfig/cron scheduler in this same process — PROJECT.md
// locks a single-binary architecture.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"github.com/danielrpof/drop-tracker/internal/artistart"
	"github.com/danielrpof/drop-tracker/internal/authgate"
	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/db"
	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/detection"
	"github.com/danielrpof/drop-tracker/internal/events"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/logging"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/notifier"
	"github.com/danielrpof/drop-tracker/internal/poller"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests to finish after a SIGTERM/SIGINT before giving up and returning
// (WR-03), so an operator-issued stop cannot hang the process indefinitely
// if a handler never completes.
const shutdownTimeout = 10 * time.Second

// pollDrainTimeout bounds how long shutdown waits for an in-flight poll
// cycle to finish before giving up, mirroring shutdownTimeout's reasoning:
// a hung upstream must not make the process unkillable.
const pollDrainTimeout = 10 * time.Second

// backfillDrainTimeout bounds how long shutdown waits for an in-flight
// artist-art backfill sweep (D-07) to finish before giving up, mirroring
// pollDrainTimeout's reasoning: a hung upstream must not make the process
// unkillable.
const backfillDrainTimeout = 10 * time.Second

// HTTP server timeouts (WR-02): an http.Server with all zero-value timeouts
// lets a client that opens a connection and sends headers/body slowly (or
// never) hold a goroutine and connection open indefinitely -- the classic
// Slowloris-style resource-exhaustion pattern. These are conservative
// defaults appropriate for a JSON API with no large uploads/downloads or
// long-lived streaming responses.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	// WR-03: derive ctx below so a SIGTERM/SIGINT (the normal way a
	// container orchestrator stops this process) is observable throughout
	// run() -- by the migration retry loop (WR-01), and by the select run()
	// uses to trigger httpSrv.Shutdown -- rather than killing the process
	// immediately and skipping the deferred pool.Close() and the
	// in-flight-request drain entirely. stop is called before deciding
	// whether to exit non-zero rather than deferred, since a deferred call
	// would never run before os.Exit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run sequences the boot path: config, then logging, then migrations, then
// the connection pool, then the HTTP server. Migrations complete before the
// listener starts (D-09). Any failure before listening returns a non-nil
// error so main exits non-zero — the service never reaches a
// running-but-broken state. The caller owns signal handling and passes the
// resulting context in, so the shutdown branch is directly testable without
// signalling the test process.
func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(cfg)

	// D-11 boot-time weak-passphrase WARN: if INSTANCE_PASSPHRASE is set and
	// looks weak (too short, or a known default -- including the .env.example
	// placeholder) log exactly one WARN and start normally anyway. The gate is
	// only as strong as this value, but a passphrase-policy edge case must
	// never keep the process from starting: fail-closed enforcement was
	// considered and rejected. This sits after logging.New and before
	// migrations, matching the established config -> logging -> migrations boot
	// order. The reason string carries no part of the passphrase.
	if reason, weak := authgate.IsWeakPassphrase(cfg.InstancePassphrase); weak {
		logger.Warn("INSTANCE_PASSPHRASE looks weak; the instance gate is only as strong as this value", "reason", reason)
	}

	if err := db.RunMigrations(ctx, cfg.DatabaseURL, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// cfg.MusicBrainzPollWorkers + cfg.DeezerPollWorkers is the maximum number
	// of DB-touching goroutines both poll cycles can produce at once (G-11-1)
	// -- passing it through is what lets db.NewPool size the pool's MaxConns
	// against the concurrency it must actually serve rather than against this
	// host's own vCPU count.
	pool, err := db.NewPool(ctx, cfg.DatabaseURL, cfg.MusicBrainzPollWorkers+cfg.DeezerPollWorkers)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	// Exactly one *musicbrainz.Client and one rate.Limiter for the whole
	// process (D-07) -- plan 03-04's poller reuses this same instance, so
	// total outbound MusicBrainz rate stays bounded across search traffic
	// and poll traffic together, not per-caller (T-03-08). Deliberately a
	// separate limiter instance from dzLimiter below (D-08): MusicBrainz's
	// ~1/sec pace must never throttle Deezer's faster pace, and vice versa.
	// Constructed before store below (Phase 13 bug #3, 13-RESEARCH.md
	// Pitfall 3): store's own construction now depends on a matcher that
	// depends on this client, so this block must move ahead of it.
	mbLimiter := rate.NewLimiter(rate.Limit(cfg.MusicBrainzRateLimitPerSec), 1)
	mbClient := musicbrainz.NewClient(cfg.MusicBrainzUserAgent, mbLimiter, nil)

	// detector is the EventRecorder poller.New wires into RunMusicBrainzCycle
	// (Phase 4, DTCT-01/DTCT-02/DTCT-03) -- a second sqlc.New(pool) instance,
	// matching store's own pattern above: sqlc.Queries is a thin, stateless
	// wrapper over pool, so a second instance shares the same connection
	// pool without any extra coordination. mbClient doubles as both the
	// detection.RecordingSource for guest-feature detection's recording
	// browse (D-05) and the detection.ReleaseDetailSource for deluxe-change
	// detection's per-release track-count fetch (D-01) -- the same
	// rate-limited, User-Agent-identified instance RunMusicBrainzCycle
	// already uses for release-groups, so both draw from the same
	// whole-process MusicBrainz budget (D-07), which is why detector
	// construction moves below mbClient's.
	detector := detection.New(sqlc.New(pool), mbClient, mbClient, detection.WithNotifyMaxReleaseAgeDays(cfg.NotifyMaxReleaseAgeDays))

	// Exactly one *deezer.Client and one rate.Limiter for the whole process
	// (D-07, D-08) -- plan 03-04's Deezer poll cycle reuses this same
	// instance, keeping every outbound Deezer request under a single shared
	// budget. The rate is the per-second equivalent of Deezer's documented
	// 50-per-5-second ceiling; the burst is the full five-second allowance,
	// so a short burst is admitted and sustained traffic settles to the
	// documented average. Constructed before store below, mirroring
	// mbClient's own reordering above (Phase 13 bug #3, 13-RESEARCH.md
	// Pitfall 3).
	dzLimiter := rate.NewLimiter(rate.Limit(float64(cfg.DeezerRateLimitPer5s)/5.0), cfg.DeezerRateLimitPer5s)
	dzClient := deezer.NewClient(dzLimiter, nil)

	// artMatcher is the single artistart.Matcher instance serving both of
	// D-06's call sites: the add-time option store's construction wires in
	// below, and the startup backfill sweep further down. One shared
	// instance is what keeps D-08's match rule and D-09's
	// fail-closed policy from ever drifting apart between the two call
	// sites. It reuses these exact dzClient/mbClient instances (rather than
	// constructing separate clients) so both call sites stay inside the
	// process-wide rate limiters above -- no second outbound budget is
	// opened. The WithArtistLinks option below is D-09r's key link: the
	// matcher now also consults MusicBrainz's curated Deezer link (Tier 0)
	// and alias list (Tier 1) before falling back to name search, still
	// reusing these exact process-wide rate-limited clients. Omitting this
	// option would silently disable both new tiers in production while
	// leaving every unit test green.
	artMatcher := artistart.NewMatcher(dzClient, dzClient, mbClient, artistart.WithArtistLinks(mbClient, dzClient), artistart.WithLogger(logger))

	// artActivityGate (D-10, grilling round Q1) coordinates the add-time
	// matcher and the backfill sweep below, both of which share the same
	// rate-limited clients: without this, the sweep would compete with
	// every interactive add for the same budget, worst right after every
	// deploy when the sweep's backlog is largest. This single shared gate
	// is what lets the sweep detect and yield to interactive activity
	// instead.
	artActivityGate := artistart.NewActivityGate()

	store := watchlist.NewService(sqlc.New(pool), watchlist.WithArtistArt(artMatcher, artActivityGate, logger))

	// eventsStore backs GET /events (Phase 6, HIST-01) -- a fourth
	// sqlc.New(pool) instance, matching store/detector/notif's own pattern:
	// sqlc.Queries is a stateless wrapper over the shared pool.
	// cfg.EventRetentionDays (Phase 10, DATA-01) is threaded straight from
	// boot-time config so the retention window an operator sets is what
	// every List call actually applies.
	eventsStore := events.NewService(sqlc.New(pool), cfg.EventRetentionDays)

	// WithAuthGate engages the instance passphrase gate (GATE-01..06) when
	// INSTANCE_PASSPHRASE is set; with it empty the option is inert and every
	// route behaves exactly as v1.2 (GATE-07). Without this argument the gate
	// would never engage in production. TrustProxyHeaders defaults false so
	// middleware.RealIP stays off unless the operator opts in behind a trusted
	// reverse proxy (D-14). The third argument is the D-12 brute-force alert
	// sink, selected by the same disabled-case gate notifier.Select uses -- an
	// empty DISCORD_WEBHOOK_URL yields the inert no-op Alerter and logs one
	// Info line, a set URL yields the Discord-backed one over the same webhook
	// this process already uses for release notifications.
	srv := httpserver.New(pool, store, eventsStore, []httpserver.SearchSource{
		httpserver.NewMusicBrainzSource(mbClient),
		httpserver.NewDeezerSource(dzClient),
	}, logger, httpserver.WithAuthGate(cfg.InstancePassphrase, cfg.TrustProxyHeaders, authgate.SelectAlerter(cfg.DiscordWebhookURL, logger)))
	// Close stops the gate's per-IP limiter-map sweeper goroutine (plan 14-02);
	// a no-op when the gate is disabled. Deferred here so it runs on every
	// return path from run().
	defer srv.Close()

	// notif is the poller.Notifier plan 05-01's Select wires in: it owns
	// D-10's gate (empty DISCORD_WEBHOOK_URL -> notifier.NoOp, so poller.New's
	// notifier argument is always non-nil and neither cycle method ever
	// nil-checks it) rather than branching on cfg.DiscordWebhookURL here. A
	// third sqlc.New(pool) instance, matching store/detector's own pattern
	// above -- sqlc.Queries is a stateless wrapper over the shared pool.
	notif := notifier.Select(cfg.DiscordWebhookURL, sqlc.New(pool), nil, logger, notifier.WithMaxReleaseAgeDays(cfg.NotifyMaxReleaseAgeDays))

	// pollr reuses the same mbClient/dzClient instances handed to
	// httpserver.New above rather than constructing its own -- sharing the
	// instance is what makes each source's rate.Limiter a whole-process
	// budget: search traffic and poll traffic draw from the same token
	// bucket, so a burst of /search calls can never push the combined
	// outbound rate past what the operator configured (D-07).
	pollr, err := poller.New(store, mbClient, dzClient, detector, notif, cfg.PollInterval, logger, poller.WithMusicBrainzWorkers(cfg.MusicBrainzPollWorkers), poller.WithDeezerWorkers(cfg.DeezerPollWorkers))
	if err != nil {
		return fmt.Errorf("build poller: %w", err)
	}
	pollr.Start(ctx)
	// Deferred AFTER defer pool.Close() (above) so Go's LIFO defer ordering
	// guarantees this drain runs *before* the pool closes -- a poll cycle
	// still mid-request when the pool closes would surface as an
	// intermittent connection error at shutdown, indistinguishable from
	// data corruption rather than a shutdown race (03-RESEARCH.md pitfall
	// 4). Do not move this defer earlier in the function: doing so would
	// silently reverse the drain-before-close ordering it depends on.
	defer func() {
		drainCtx, cancel := context.WithTimeout(context.Background(), pollDrainTimeout)
		defer cancel()
		if err := pollr.Stop(drainCtx); err != nil {
			logger.Error("poller drain failed", "poller_error", err.Error())
		}
	}()

	// artistart.Backfill (D-07, cooldown-bounded per D-12, yielding per
	// D-10) sweeps every watchlisted artist with a NULL image and no recent
	// match attempt. Run in its own goroutine, deliberately asynchronously:
	// the sweep's duration scales with the number of image-less watchlisted
	// artists times the rate-limited upstream calls each needs, and
	// blocking the listener on it would make container startup and health
	// checks scale with watchlist size. Its own sqlc.New(pool) instance is
	// the fifth, matching this file's existing deliberate idiom of one
	// stateless sqlc.Queries wrapper per consumer -- do not consolidate the
	// existing ones. Backfill itself already logs one Info summary line
	// carrying its Stats, including the computed match rate (D-11) -- the
	// log call below is only for a non-nil returned error, not a second
	// summary.
	backfillDone := make(chan struct{})
	go func() {
		defer close(backfillDone)
		if _, err := artistart.Backfill(ctx, logger, sqlc.New(pool), artMatcher, artActivityGate); err != nil {
			logger.Error("artist art backfill failed", "backfill_error", err.Error())
		}
	}()
	// Deferred AFTER defer pool.Close() (above), mirroring the poller
	// drain immediately above it: Go's LIFO defer ordering guarantees this
	// drain also runs *before* the pool closes, so a still-in-flight
	// backfill query at shutdown never races the pool closing underneath
	// it (03-RESEARCH.md pitfall 4, the same failure mode the poller drain
	// already guards against). Do not move this defer earlier in the
	// function: doing so would silently reverse the drain-before-close
	// ordering it depends on.
	defer func() {
		select {
		case <-backfillDone:
		case <-time.After(backfillDrainTimeout):
			logger.Warn("artist art backfill drain timed out")
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", addr)
		serveErr <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve http: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutdown signal received, shutting down gracefully")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		return nil
	}
}
