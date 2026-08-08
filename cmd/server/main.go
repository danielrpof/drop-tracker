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

	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/db"
	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/detection"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/logging"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
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
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run sequences the boot path: config, then logging, then migrations, then
// the connection pool, then the HTTP server. Migrations complete before the
// listener starts (D-09). Any failure before listening returns a non-nil
// error so main exits non-zero — the service never reaches a
// running-but-broken state.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(cfg)

	// WR-03: derive ctx from signal.NotifyContext so a SIGTERM/SIGINT (the
	// normal way a container orchestrator stops this process) is observable
	// throughout run() -- by the migration retry loop (WR-01), and by the
	// select below that triggers httpSrv.Shutdown -- rather than killing the
	// process immediately and skipping the deferred pool.Close() and the
	// in-flight-request drain entirely.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.RunMigrations(ctx, cfg.DatabaseURL, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	store := watchlist.NewService(sqlc.New(pool))

	// detector is the EventRecorder poller.New wires into RunMusicBrainzCycle
	// (Phase 4, DTCT-01) -- a second sqlc.New(pool) instance, matching store's
	// own pattern above: sqlc.Queries is a thin, stateless wrapper over pool,
	// so a second instance shares the same connection pool without any extra
	// coordination.
	detector := detection.New(sqlc.New(pool))

	// Exactly one *musicbrainz.Client and one rate.Limiter for the whole
	// process (D-07) -- plan 03-04's poller reuses this same instance, so
	// total outbound MusicBrainz rate stays bounded across search traffic
	// and poll traffic together, not per-caller (T-03-08). Deliberately a
	// separate limiter instance from dzLimiter below (D-08): MusicBrainz's
	// ~1/sec pace must never throttle Deezer's faster pace, and vice versa.
	mbLimiter := rate.NewLimiter(rate.Limit(cfg.MusicBrainzRateLimitPerSec), 1)
	mbClient := musicbrainz.NewClient(cfg.MusicBrainzUserAgent, mbLimiter, nil)

	// Exactly one *deezer.Client and one rate.Limiter for the whole process
	// (D-07, D-08) -- plan 03-04's Deezer poll cycle reuses this same
	// instance, keeping every outbound Deezer request under a single shared
	// budget. The rate is the per-second equivalent of Deezer's documented
	// 50-per-5-second ceiling; the burst is the full five-second allowance,
	// so a short burst is admitted and sustained traffic settles to the
	// documented average.
	dzLimiter := rate.NewLimiter(rate.Limit(float64(cfg.DeezerRateLimitPer5s)/5.0), cfg.DeezerRateLimitPer5s)
	dzClient := deezer.NewClient(dzLimiter, nil)

	srv := httpserver.New(pool, store, []httpserver.SearchSource{
		httpserver.NewMusicBrainzSource(mbClient),
		httpserver.NewDeezerSource(dzClient),
	}, logger)

	// pollr reuses the same mbClient/dzClient instances handed to
	// httpserver.New above rather than constructing its own -- sharing the
	// instance is what makes each source's rate.Limiter a whole-process
	// budget: search traffic and poll traffic draw from the same token
	// bucket, so a burst of /search calls can never push the combined
	// outbound rate past what the operator configured (D-07).
	pollr, err := poller.New(store, mbClient, dzClient, detector, cfg.PollInterval, logger)
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
