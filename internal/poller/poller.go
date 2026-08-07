// Package poller runs the scheduled polling cycles that satisfy CLNT-01
// (MusicBrainz) and CLNT-02 (Deezer): each cycle reads the live watchlist
// through the existing watchlist.Store seam (D-05), calls its source for
// every entry sequentially -- one outbound request at a time, so the
// configured per-source rate is never multiplied by concurrency (D-07) --
// and logs one structured result per artist. This phase performs no
// diffing and writes nothing to the database (D-04); Phase 4 replaces the
// log statement with real diff logic against the "seen" store.
package poller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	store watchlist.Store
	mb    ReleaseGroupSource
	dz    AlbumSource

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
}

// New builds a Poller over store, mb and dz and registers two independent
// cron entries -- one per source -- on the spec "@every <interval>".
// Registering them as two separate AddFunc calls, each closing over one
// cycle method, is what guarantees MusicBrainz's slower pace can never
// delay or block Deezer's faster one (D-08). interval must be greater than
// zero; New returns a non-nil error and registers no entry otherwise.
func New(store watchlist.Store, mb ReleaseGroupSource, dz AlbumSource, interval time.Duration, logger *slog.Logger) (*Poller, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("poller: interval must be greater than zero, got %s", interval)
	}

	p := &Poller{
		store:    store,
		mb:       mb,
		dz:       dz,
		logger:   logger,
		interval: interval,
		cron:     cron.New(),
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

// RunMusicBrainzCycle reads the live watchlist and calls
// ReleaseGroupsByArtist once per entry, sequentially. A per-artist error is
// logged and the cycle continues to the next artist -- one unreachable
// artist must not cost the rest of the cycle. The cycle itself writes
// nothing to the database (D-04): it only logs.
func (p *Poller) RunMusicBrainzCycle(ctx context.Context) error {
	// Compare-and-swap, not a mutex: a tick that arrives during a run must
	// be *skipped*, not queued behind it (D-09) -- a mutex would serialise
	// ticks into a backlog and eventually run every missed cycle back to
	// back. Released via defer, not a store at the end of the function, so
	// the guard also releases on an error return *and* on a panic -- a
	// wedged flag would silently stop this source polling for the
	// process's lifetime.
	if !p.mbRunning.CompareAndSwap(false, true) {
		p.logger.Warn("skipping poll cycle: previous cycle still in progress", slog.String("source", sourceMusicBrainz))
		return ErrCycleInProgress
	}
	defer p.mbRunning.Store(false)

	cycleID := fmt.Sprintf("musicbrainz-%d", nextCycleID.Add(1))
	logger := p.logger.With(slog.String("source", sourceMusicBrainz), slog.String("cycle_id", cycleID))

	entries, err := p.store.List(ctx)
	if err != nil {
		return fmt.Errorf("poller: list watchlist: %w", err)
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		groups, err := p.mb.ReleaseGroupsByArtist(ctx, entry.MBID)
		if err != nil {
			logger.Error("poll artist failed",
				slog.String("artist_mbid", entry.MBID),
				slog.String("artist_name", entry.Name),
				slog.String("musicbrainz_error", err.Error()),
			)
			continue
		}

		logger.Info("poll result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("artist_name", entry.Name),
			slog.Int("item_count", len(groups)),
		)
	}

	return nil
}

// RunDeezerCycle reads the live watchlist and calls ArtistAlbums once per
// entry that carries a non-nil DeezerID, sequentially. An entry with a nil
// DeezerID is skipped with a logged reason and no HTTP request -- there is
// no name-search fallback to backfill it (D-06). A per-artist error is
// logged and the cycle continues. The cycle writes nothing to the database
// (D-04).
func (p *Poller) RunDeezerCycle(ctx context.Context) error {
	// See RunMusicBrainzCycle's comment on this same pattern -- dzRunning is
	// a wholly independent guard from mbRunning (D-08), so an overlapping
	// MusicBrainz cycle never blocks or delays a Deezer tick.
	if !p.dzRunning.CompareAndSwap(false, true) {
		p.logger.Warn("skipping poll cycle: previous cycle still in progress", slog.String("source", sourceDeezer))
		return ErrCycleInProgress
	}
	defer p.dzRunning.Store(false)

	cycleID := fmt.Sprintf("deezer-%d", nextCycleID.Add(1))
	logger := p.logger.With(slog.String("source", sourceDeezer), slog.String("cycle_id", cycleID))

	entries, err := p.store.List(ctx)
	if err != nil {
		return fmt.Errorf("poller: list watchlist: %w", err)
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		if entry.DeezerID == nil {
			logger.Info("skipping deezer poll: no deezer_id",
				slog.String("artist_mbid", entry.MBID),
				slog.String("artist_name", entry.Name),
			)
			continue
		}

		albums, err := p.dz.ArtistAlbums(ctx, *entry.DeezerID, deezerAlbumPageSize)
		if err != nil {
			logger.Error("poll artist failed",
				slog.String("artist_mbid", entry.MBID),
				slog.String("artist_name", entry.Name),
				slog.String("deezer_error", err.Error()),
			)
			continue
		}

		logger.Info("poll result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("artist_name", entry.Name),
			slog.Int("item_count", len(albums)),
		)
	}

	return nil
}
