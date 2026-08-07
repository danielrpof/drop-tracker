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
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

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

	mbRunning atomic.Bool
	dzRunning atomic.Bool
}

// New builds a Poller over store, mb and dz, polling on interval once
// scheduling is wired up. interval must be greater than zero.
func New(store watchlist.Store, mb ReleaseGroupSource, dz AlbumSource, interval time.Duration, logger *slog.Logger) (*Poller, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("poller: interval must be greater than zero, got %s", interval)
	}
	return &Poller{
		store:    store,
		mb:       mb,
		dz:       dz,
		logger:   logger,
		interval: interval,
	}, nil
}

// RunMusicBrainzCycle reads the live watchlist and calls
// ReleaseGroupsByArtist once per entry, sequentially. A per-artist error is
// logged and the cycle continues to the next artist -- one unreachable
// artist must not cost the rest of the cycle. The cycle itself writes
// nothing to the database (D-04): it only logs.
func (p *Poller) RunMusicBrainzCycle(ctx context.Context) error {
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
