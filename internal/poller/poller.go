// Package poller runs the scheduled polling cycles that satisfy CLNT-01
// (MusicBrainz) and CLNT-02 (Deezer): each cycle reads the live watchlist
// through the existing watchlist.Store seam (D-05) and calls its source for
// every entry, bounded by a per-source configurable worker pool (PERF-01;
// both RunMusicBrainzCycle and RunDeezerCycle fan out as of Phase 11) --
// the per-source rate.Limiter, not this bound, is what still caps actual
// outbound request rate regardless of how many workers are in flight
// (D-07) -- and logs one structured result per artist. Each cycle then
// hands its
// fetched results to the EventRecorder seam, which diffs them against the
// seen store and records previously-unseen releases as event rows (Phase 4,
// DTCT-01; the Deezer half of this wiring is plan 04-02) -- this package
// still performs no diffing and holds no database connection itself, it
// only calls the seam.
package poller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// ErrCycleInProgress is returned by RunMusicBrainzCycle or RunDeezerCycle
// when a previous cycle for that same source is still running. A tick that
// arrives during a run is skipped, not queued behind it (D-09) -- queuing
// would eventually run every missed cycle back to back, which is exactly
// the compounding behavior the overlap guard exists to prevent.
var ErrCycleInProgress = errors.New("poller: cycle already in progress")

const (
	// deezerAlbumPageSize bounds the page size ArtistAlbums is called with
	// for every artist in the Deezer poll cycle.
	deezerAlbumPageSize = 50

	sourceMusicBrainz = "musicbrainz"
	sourceDeezer      = "deezer"

	// defaultMusicBrainzPollWorkers is the fan-out ceiling RunMusicBrainzCycle
	// uses when no WithMusicBrainzWorkers option is supplied (D-02). Mirrors
	// config.Config's own MUSICBRAINZ_POLL_WORKERS envDefault so a Poller
	// built without config.Load (e.g. directly in a test) still gets the
	// same locked default.
	defaultMusicBrainzPollWorkers = 3

	// defaultDeezerPollWorkers is RunDeezerCycle's own fan-out ceiling (D-02),
	// mirroring config.Config's DEEZER_POLL_WORKERS envDefault the same way
	// defaultMusicBrainzPollWorkers mirrors MUSICBRAINZ_POLL_WORKERS.
	defaultDeezerPollWorkers = 5
)

// ReleaseGroupSource is the narrow seam RunMusicBrainzCycle depends on,
// mirroring musicbrainz.ReleaseGroupLister -- declared here, in the
// consumer, exactly as httpserver.Pinger and watchlist.Store are declared
// in their own consumers (D-11), so a test can substitute a fake with no
// real HTTP client.
type ReleaseGroupSource interface {
	ReleaseGroupsByArtist(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error)
}

var _ ReleaseGroupSource = (*musicbrainz.Client)(nil)

// AlbumSource is the narrow seam RunDeezerCycle depends on, mirroring
// deezer.AlbumLister.
type AlbumSource interface {
	ArtistAlbums(ctx context.Context, artistID string, limit int) ([]deezer.Album, error)
}

var _ AlbumSource = (*deezer.Client)(nil)

// EventRecorder is the narrow seam RunMusicBrainzCycle and RunDeezerCycle
// depend on to diff a cycle's fetched results against the seen store and
// record previously-unseen items as event rows -- declared here, in the
// consumer, exactly as ReleaseGroupSource and AlbumSource are, so a test can
// substitute a fake with no real database connection (Phase 4, DTCT-01;
// DetectDeezer added in plan 04-02).
type EventRecorder interface {
	DetectMusicBrainz(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error
	DetectDeezer(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, albums []deezer.Album) error
}

// Notifier is the narrow seam RunMusicBrainzCycle and RunDeezerCycle depend
// on to drain the events outbox at the end of their own per-artist loop
// (D-05) -- declared here, in the consumer, exactly as EventRecorder is, so
// a test can substitute a fake with no real Discord client or database
// connection. It has no disabled-state concept: cmd/server/main.go's
// notifier.Select owns D-10's gate and always hands New a non-nil Notifier
// (a real one, or notifier.NoOp), so neither cycle method ever nil-checks
// this field.
type Notifier interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
}

// Option customizes a Poller's construction, mirroring internal/db's
// RetryOption functional-option pattern (New builds a Poller from its
// defaults and then applies each supplied option in order, so the
// production call site in cmd/server/main.go stays a simple additive
// argument rather than a signature rewrite whenever a new option is added).
type Option func(*Poller)

// WithMusicBrainzWorkers overrides the default MusicBrainz poll-cycle
// fan-out ceiling (D-02, D-03). n must be greater than zero -- New rejects a
// non-positive value the same way it rejects a non-positive interval.
func WithMusicBrainzWorkers(n int) Option {
	return func(p *Poller) { p.mbWorkers = n }
}

// WithDeezerWorkers overrides the default Deezer poll-cycle fan-out ceiling
// (D-02, D-03), mirroring WithMusicBrainzWorkers. n must be greater than
// zero -- New rejects a non-positive value the same way it rejects a
// non-positive interval.
func WithDeezerWorkers(n int) Option {
	return func(p *Poller) { p.dzWorkers = n }
}

// nextCycleID is a package-level counter rendered into each cycle's
// correlation id (musicbrainz-<n> / deezer-<n>) so every log line emitted
// within one cycle can be grouped, and two successive cycles for the same
// source are always distinguishable (OPS-02).
var nextCycleID atomic.Uint64

// Poller runs the MusicBrainz and Deezer poll cycles. mbRunning and
// dzRunning are separate atomic.Bool fields, never one shared mutex,
// because a shared guard would reintroduce exactly the cross-source
// coupling D-08 rejects -- MusicBrainz's slower pace must never delay or
// block Deezer's faster one.
type Poller struct {
	store    watchlist.Store
	mb       ReleaseGroupSource
	dz       AlbumSource
	events   EventRecorder
	notifier Notifier

	logger   *slog.Logger
	interval time.Duration

	cron *cron.Cron

	// runCtx/runCancel are set once, by Start, before cron.Start() begins
	// dispatching ticks -- the write happens-before any goroutine cron
	// spawns to run a job, per the Go memory model's goroutine-creation
	// rule, so no separate synchronization is needed to read them from a
	// cron job closure. Stop cancels runCancel so an in-flight cycle
	// unwinds instead of racing the caller's next step (e.g. closing the
	// database pool) against a request still in flight.
	runCtx    context.Context
	runCancel context.CancelFunc

	mbRunning atomic.Bool
	dzRunning atomic.Bool

	// mbWorkers bounds RunMusicBrainzCycle's per-artist fan-out (PERF-01).
	// Set from defaultMusicBrainzPollWorkers unless overridden by
	// WithMusicBrainzWorkers.
	mbWorkers int

	// dzWorkers bounds RunDeezerCycle's per-artist fan-out (PERF-01). Set
	// from defaultDeezerPollWorkers unless overridden by WithDeezerWorkers.
	dzWorkers int
}

// New builds a Poller over store, mb, dz, events and notifier, and
// registers two independent cron entries -- one per source -- on the spec
// "@every <interval>". Registering them as two separate AddFunc calls, each
// closing over one cycle method, is what guarantees MusicBrainz's slower
// pace can never delay or block Deezer's faster one (D-08). interval must
// be greater than zero; New returns a non-nil error and registers no entry
// otherwise.
func New(store watchlist.Store, mb ReleaseGroupSource, dz AlbumSource, events EventRecorder, notifier Notifier, interval time.Duration, logger *slog.Logger, opts ...Option) (*Poller, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("poller: interval must be greater than zero, got %s", interval)
	}

	p := &Poller{
		store:     store,
		mb:        mb,
		dz:        dz,
		events:    events,
		notifier:  notifier,
		logger:    logger,
		interval:  interval,
		cron:      cron.New(),
		mbWorkers: defaultMusicBrainzPollWorkers,
		dzWorkers: defaultDeezerPollWorkers,
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.mbWorkers <= 0 {
		return nil, fmt.Errorf("poller: mbWorkers must be greater than zero, got %d", p.mbWorkers)
	}
	if p.dzWorkers <= 0 {
		return nil, fmt.Errorf("poller: dzWorkers must be greater than zero, got %d", p.dzWorkers)
	}

	spec := fmt.Sprintf("@every %s", interval.String())

	if _, err := p.cron.AddFunc(spec, func() {
		if err := p.RunMusicBrainzCycle(p.runCtx); err != nil && !errors.Is(err, ErrCycleInProgress) {
			p.logger.Error("musicbrainz poll cycle failed", slog.String("poller_error", err.Error()))
		}
	}); err != nil {
		return nil, fmt.Errorf("poller: register musicbrainz cron job: %w", err)
	}

	if _, err := p.cron.AddFunc(spec, func() {
		if err := p.RunDeezerCycle(p.runCtx); err != nil && !errors.Is(err, ErrCycleInProgress) {
			p.logger.Error("deezer poll cycle failed", slog.String("poller_error", err.Error()))
		}
	}); err != nil {
		return nil, fmt.Errorf("poller: register deezer cron job: %w", err)
	}

	return p, nil
}

// Start begins scheduling both cron entries. ctx bounds the lifetime of
// every cycle this Poller runs from now on -- Stop cancels the retained
// child context derived from it, not ctx itself, so a caller can Stop
// independently of whatever ctx they originally started with.
func (p *Poller) Start(ctx context.Context) {
	p.runCtx, p.runCancel = context.WithCancel(ctx)
	p.logger.Info("poller starting", slog.Duration("interval", p.interval))
	p.cron.Start()
}

// Stop stops scheduling new ticks and waits for any in-flight cycle to
// finish, bounded by ctx. p.cron.Stop() itself only stops scheduling and
// returns immediately with a context the caller must consume to observe
// drain completion -- consuming it here, rather than ignoring it, is what
// prevents an in-flight poll cycle from racing the database pool being
// closed underneath it after Stop returns (03-RESEARCH.md pitfall 4). If
// ctx expires first, the retained cycle context is cancelled so in-flight
// requests unwind, and ctx's error is returned rather than blocking
// forever on a hung upstream.
func (p *Poller) Stop(ctx context.Context) error {
	p.logger.Info("poller stopping")
	stopCtx := p.cron.Stop()
	select {
	case <-stopCtx.Done():
		return nil
	case <-ctx.Done():
		if p.runCancel != nil {
			p.runCancel()
		}
		return ctx.Err()
	}
}

// runCycle carries the bounded-fan-out-with-overlap-guard mechanics shared
// by RunMusicBrainzCycle and RunDeezerCycle: it CAS-guards against an
// overlapping cycle for the same source, reads the live watchlist,
// dispatches every shouldDispatch-approved entry across a worker pool
// bounded by workers (calling fetchAndRecord for each dispatched entry),
// and drains the notifier outbox once the pool has fully drained. source is
// used both as the guard's identity for logging and as the cycle_id prefix
// (D-08's per-source independence lives in the caller's choice of running
// pointer and workers count, not in this method).
func (p *Poller) runCycle(ctx context.Context, running *atomic.Bool, source string, workers int, shouldDispatch func(entry watchlist.Entry, logger *slog.Logger) bool, fetchAndRecord func(ctx context.Context, logger *slog.Logger, entry watchlist.Entry)) error {
	// Compare-and-swap, not a mutex: a tick that arrives during a run must
	// be *skipped*, not queued behind it (D-09) -- a mutex would serialise
	// ticks into a backlog and eventually run every missed cycle back to
	// back. Released via defer, not a store at the end of the function, so
	// the guard also releases on an error return *and* on a panic -- a
	// wedged flag would silently stop this source polling for the
	// process's lifetime.
	if !running.CompareAndSwap(false, true) {
		p.logger.Warn("skipping poll cycle: previous cycle still in progress", slog.String("source", source))
		return ErrCycleInProgress
	}
	defer running.Store(false)

	cycleID := fmt.Sprintf("%s-%d", source, nextCycleID.Add(1))
	logger := p.logger.With(slog.String("source", source), slog.String("cycle_id", cycleID))
	cycleStart := time.Now()

	entries, err := p.store.List(ctx)
	if err != nil {
		return fmt.Errorf("poller: list watchlist: %w", err)
	}

	// Bounded fan-out (PERF-01): sem is a buffered-channel semaphore sized
	// to workers, created fresh for this one cycle invocation and
	// discarded when it returns -- no persistent pool, no lifecycle wiring
	// into Start/Stop (11-RESEARCH.md "Don't Hand-Roll"). Acquiring a slot
	// is a select against ctx.Done() rather than a plain blocking send: if
	// the context is cancelled while the dispatch loop is waiting for a
	// free slot, dispatching stops immediately instead of blocking until a
	// slot frees up. This deliberately differs from 11-RESEARCH.md Pattern
	// 1's snippet, which returns directly out of the loop on cancellation --
	// doing that here would abandon any already-dispatched goroutines and
	// skip wg.Wait() below, so instead the cancellation error is recorded
	// and the loop breaks, always reaching wg.Wait().
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var cycleErr error

dispatch:
	for _, entry := range entries {
		if !shouldDispatch(entry, logger) {
			continue
		}

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			cycleErr = ctx.Err()
			break dispatch
		}

		wg.Add(1)
		go func(entry watchlist.Entry) {
			defer wg.Done()
			defer func() { <-sem }()
			// A panic inside this goroutine can never be recovered by a
			// caller's own defer/recover -- panics do not cross goroutine
			// boundaries in Go, unlike the sequential loop this replaces,
			// where a panic on one artist would unwind through the same
			// call stack the caller's own would run on. Left unrecovered
			// here, one artist's panic (e.g. a malformed upstream response
			// triggering a nil pointer/index-out-of-range) would crash the
			// entire process instead of costing only that artist's own
			// result -- the exact failure mode PERF-03's per-artist
			// isolation exists to prevent, just via panic instead of a
			// returned error.
			defer func() {
				if r := recover(); r != nil {
					logger.Error("poll worker panicked",
						slog.String("artist_mbid", entry.MBID),
						slog.String("artist_name", entry.Name),
						slog.Any("panic_value", r),
					)
				}
			}()

			// A worker whose slot was acquired before cancellation but
			// which has not yet started its fetch must still not issue
			// one -- when workers is large enough that the dispatch loop
			// above never blocks on sem (e.g. worker count >= entry
			// count), a cancellation racing the dispatch loop would
			// otherwise go unobserved by every already-dispatched worker,
			// silently fetching all of them despite the cancelled
			// context. This mirrors the sequential loop's own
			// per-iteration ctx.Err() check.
			if err := ctx.Err(); err != nil {
				return
			}

			fetchAndRecord(ctx, logger, entry)
		}(entry)
	}
	wg.Wait()

	// cycleErr is only set when the dispatch loop itself observed
	// cancellation while waiting for a semaphore slot -- when workers is
	// large enough that dispatch never blocks (e.g. worker count >= entry
	// count, as in this cycle's default configuration over a small
	// watchlist), every entry can dispatch before any worker's own
	// in-flight ctx.Err() check (above) has a chance to run, so the
	// dispatch loop's own select never observes the cancellation even
	// though a worker did. Re-checking ctx.Err() here, after the join,
	// catches that case too -- a cancelled cycle must always report the
	// context error, regardless of which of the two checks caught it.
	if cycleErr == nil {
		cycleErr = ctx.Err()
	}

	logger.Info("poll cycle complete",
		slog.Int("artist_count", len(entries)),
		slog.Int64("duration_ms", time.Since(cycleStart).Milliseconds()),
	)

	if cycleErr != nil {
		return cycleErr
	}

	// D-05: notify inline, at the end of the cycle, using this cycle's own
	// logger so notifier lines inherit the source/cycle_id correlation
	// attributes rather than starting a fresh logger. A delivery failure is
	// logged, not returned -- it must never turn an otherwise-successful
	// detection cycle into a failed one, mirroring how a per-artist
	// detection error above is logged rather than propagated.
	if err := p.notifier.NotifyPending(ctx, logger); err != nil {
		logger.Error("notify pending failed", slog.String("notifier_error", err.Error()))
	}

	return nil
}

// RunMusicBrainzCycle reads the live watchlist and calls
// ReleaseGroupsByArtist once per entry, fanned out over a bounded worker
// pool sized by p.mbWorkers (PERF-01), then hands the fetched results to
// the EventRecorder seam so previously-unseen releases are recorded (Phase
// 4, DTCT-01). MusicBrainz has no skip case -- every watchlist entry is
// dispatched, unlike RunDeezerCycle's nil-DeezerID skip. A per-artist fetch
// or detection error is logged inside its own worker and never propagated
// to any sibling worker or the caller (PERF-03) -- one unreachable or
// misbehaving artist must not cost the rest of the cycle. ErrCycleInProgress
// is returned when a previous MusicBrainz cycle is still running -- see
// runCycle for the shared overlap-guard/fan-out mechanics.
func (p *Poller) RunMusicBrainzCycle(ctx context.Context) error {
	shouldDispatch := func(entry watchlist.Entry, logger *slog.Logger) bool {
		return true
	}

	fetchAndRecord := func(ctx context.Context, logger *slog.Logger, entry watchlist.Entry) {
		groups, err := p.mb.ReleaseGroupsByArtist(ctx, entry.MBID)
		if err != nil {
			logger.Error("poll artist failed",
				slog.String("artist_mbid", entry.MBID),
				slog.String("artist_name", entry.Name),
				slog.String("musicbrainz_error", err.Error()),
			)
			return
		}

		logger.Info("poll result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("artist_name", entry.Name),
			slog.Int("item_count", len(groups)),
		)

		if err := p.events.DetectMusicBrainz(ctx, logger, entry, groups); err != nil {
			logger.Error("detection failed",
				slog.String("artist_mbid", entry.MBID),
				slog.String("artist_name", entry.Name),
				slog.String("detection_error", err.Error()),
			)
			return
		}
	}

	return p.runCycle(ctx, &p.mbRunning, sourceMusicBrainz, p.mbWorkers, shouldDispatch, fetchAndRecord)
}

// RunDeezerCycle reads the live watchlist and calls ArtistAlbums once per
// entry that carries a non-nil DeezerID, fanned out over a bounded worker
// pool sized by p.dzWorkers (PERF-01), then hands the fetched albums to the
// EventRecorder seam so previously-unseen albums are recorded as
// new_release events in Deezer's own id namespace (Phase 4, plan 04-02). An
// entry with a nil DeezerID is skipped with a logged reason and no HTTP
// request, no recorder call, and no row -- there is no name-search fallback
// to backfill it (D-06); this check happens before the semaphore is
// acquired, so a skipped entry never occupies a worker slot or spawns a
// goroutine. This skip is Deezer-only; RunMusicBrainzCycle dispatches every
// entry unconditionally. A per-artist fetch or detection error is logged
// inside its own worker and never propagated to any sibling worker or the
// caller (PERF-03). ErrCycleInProgress is returned when a previous Deezer
// cycle is still running -- see runCycle for the shared overlap-guard/
// fan-out mechanics (dzRunning is a wholly independent guard from mbRunning
// (D-08), so an overlapping MusicBrainz cycle never blocks or delays a
// Deezer tick).
func (p *Poller) RunDeezerCycle(ctx context.Context) error {
	shouldDispatch := func(entry watchlist.Entry, logger *slog.Logger) bool {
		if entry.DeezerID == nil {
			logger.Info("skipping deezer poll: no deezer_id",
				slog.String("artist_mbid", entry.MBID),
				slog.String("artist_name", entry.Name),
			)
			return false
		}
		return true
	}

	fetchAndRecord := func(ctx context.Context, logger *slog.Logger, entry watchlist.Entry) {
		albums, err := p.dz.ArtistAlbums(ctx, *entry.DeezerID, deezerAlbumPageSize)
		if err != nil {
			logger.Error("poll artist failed",
				slog.String("artist_mbid", entry.MBID),
				slog.String("artist_name", entry.Name),
				slog.String("deezer_error", err.Error()),
			)
			return
		}

		logger.Info("poll result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("artist_name", entry.Name),
			slog.Int("item_count", len(albums)),
		)

		if err := p.events.DetectDeezer(ctx, logger, entry, albums); err != nil {
			logger.Error("detection failed",
				slog.String("artist_mbid", entry.MBID),
				slog.String("artist_name", entry.Name),
				slog.String("detection_error", err.Error()),
			)
			return
		}
	}

	return p.runCycle(ctx, &p.dzRunning, sourceDeezer, p.dzWorkers, shouldDispatch, fetchAndRecord)
}
