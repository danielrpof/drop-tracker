package artistart

// This file implements D-07's startup backfill sweep: a one-time pass over
// every watchlisted artist with no image and no recent match attempt
// (D-06/D-12's cooldown-aware ListArtistsMissingImage query), delegating
// every match decision to Matcher.Match (D-08/D-09) exactly as the add-time
// path in watchlist.Service.Add does -- neither call site ever reimplements
// any part of the match rule. D-10 adds ActivityGate-based yielding so this
// sweep de-prioritizes itself behind interactive add traffic sharing the
// same rate-limited Deezer/MusicBrainz clients, and D-11 adds a match-rate
// summary log so an operator has a real signal for whether matching is
// actually working in production.

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

// backfillActivityYieldInterval and backfillActivityMaxWait (D-10, grilling
// round Q1) bound how long the sweep will defer to an in-flight add-time
// match before proceeding anyway: long enough to meaningfully de-prioritize
// the sweep behind interactive traffic, short enough that the sweep still
// makes steady forward progress even under sustained add activity -- it is
// not required to ever go fully silent.
const (
	backfillActivityYieldInterval = 250 * time.Millisecond
	backfillActivityMaxWait       = 5 * time.Second
)

// Store is the narrow, consumer-declared seam Backfill depends on, mirroring
// watchlist.ArtistMatcher's own narrow-seam pattern -- a test can satisfy
// this without a live Postgres connection. Widened with RecordArtMatchAttempt
// per D-12 (grilling round Q4): every artist the sweep visits must have its
// attempt recorded, regardless of outcome, or a fail-closed artist would be
// re-queried against Deezer on every single process restart forever.
type Store interface {
	ListArtistsMissingImage(ctx context.Context) ([]sqlc.Artist, error)
	UpsertArtist(ctx context.Context, arg sqlc.UpsertArtistParams) (sqlc.Artist, error)
	RecordArtMatchAttempt(ctx context.Context, mbid string) error
}

// This compile-time guard asserts *sqlc.Queries already satisfies the
// interface above (plan 13-02's three generated methods), so a future
// signature change to any of them breaks the build here rather than
// silently breaking the sweep at runtime.
var _ Store = (*sqlc.Queries)(nil)

// Stats counts one Backfill run's outcomes. Visited counts every artist the
// sweep attempted, regardless of outcome; Matched + Unmatched + Errored sum
// consistently against it (an artist can only ever land in exactly one of
// the three, since every branch below increments exactly one before moving
// to the next artist -- see Backfill's per-artist branches).
type Stats struct {
	Visited   int
	Matched   int
	Unmatched int
	Errored   int
}

// MatchRatePercent returns the percentage of visited artists that resolved
// to a confident match (D-11, grilling round Q3): 0 when Visited is 0 (no
// divide-by-zero), otherwise 100*Matched/Visited.
//
// This is the sweep's single operational signal that matching is actually
// working in production: the summary log line Backfill emits below includes
// it specifically so an operator can eyeball, from logs alone, whether D-08's
// strict matching is resolving real artists. A persistently low rate
// (informal threshold: under ~40%) is a signal to revisit
// normalizeArtistName's folding rules, not evidence the artists aren't on
// Deezer.
func (s Stats) MatchRatePercent() float64 {
	if s.Visited == 0 {
		return 0
	}
	return 100 * float64(s.Matched) / float64(s.Visited)
}

// waitForActivityGate defers to an in-flight add-time match (D-10): if gate
// is nil or not currently active, it returns immediately with no polling and
// no behavior change from the pre-D-10 sweep. Otherwise it polls
// gate.Active() on a backfillActivityYieldInterval ticker, returning as soon
// as the gate reports inactive, or once backfillActivityMaxWait has elapsed
// -- whichever comes first -- so the sweep never blocks on the gate
// indefinitely. A context cancellation observed while polling is returned
// immediately, so a shutdown signal is never delayed behind this wait.
func waitForActivityGate(ctx context.Context, gate *ActivityGate) error {
	if gate == nil || !gate.Active() {
		return nil
	}

	deadline := time.Now().Add(backfillActivityMaxWait)
	ticker := time.NewTicker(backfillActivityYieldInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if !gate.Active() || time.Now().After(deadline) {
				return nil
			}
		}
	}
}

// Backfill visits every artist store.ListArtistsMissingImage returns --
// already scoped to watchlisted, image-less, cooldown-eligible artists by
// D-06/D-07/D-12 -- and, for each, delegates the match decision to
// m.Match, writes a confident match through store.UpsertArtist, and records
// the attempt through store.RecordArtMatchAttempt (D-12) so a future sweep
// can tell "never tried" apart from "tried and failed." gate is optional:
// nil means no coordination with an add-time match (D-10).
//
// A per-artist error from Match, UpsertArtist, or RecordArtMatchAttempt is
// logged and that artist is skipped -- the sweep never aborts because one
// artist was unreachable or ambiguous, mirroring
// detectDeluxeChanges/detectGuestFeatures's own per-item isolation idiom. A
// context cancellation stops the sweep promptly and returns the accumulated
// Stats plus the context error; artists already processed stay committed.
//
// Backfill does not reimplement any part of D-08 (the match rule) or D-09
// (fail-closed policy) -- it decides only which artists to visit and what to
// persist, delegating every match decision to m.Match.
func Backfill(ctx context.Context, logger *slog.Logger, store Store, m *Matcher, gate *ActivityGate) (Stats, error) {
	var stats Stats

	artists, err := store.ListArtistsMissingImage(ctx)
	if err != nil {
		return Stats{}, err
	}

	for _, a := range artists {
		if err := ctx.Err(); err != nil {
			return stats, err
		}

		if err := waitForActivityGate(ctx, gate); err != nil {
			return stats, err
		}

		res, matchErr := m.Match(ctx, a.Mbid, a.Name)
		if matchErr != nil {
			// A transient lookup error is not a considered "we checked and
			// there's no confident match" decision -- do NOT call
			// RecordArtMatchAttempt here. Starting a 24-hour cooldown on a
			// fluke network error would make a genuinely matchable artist
			// wait a full day for no reason.
			stats.Errored++
			logger.Error("artist art backfill: match failed",
				slog.String("artist_mbid", a.Mbid),
				slog.String("artist_name", a.Name),
				slog.String("error", matchErr.Error()),
			)
			stats.Visited++
			continue
		}

		if !res.Matched {
			// D-09 fail-closed: write nothing via UpsertArtist (not even
			// two nil fields, which would touch updated_at for no reason),
			// but the attempt IS a considered decision (D-12) and must be
			// recorded so the cooldown predicate has something to check.
			// A failed RecordArtMatchAttempt counts as Errored, not also
			// Unmatched (WR-02, 13-REVIEW.md) -- each artist increments
			// exactly one of Stats' three counters, per its own doc comment.
			if err := store.RecordArtMatchAttempt(ctx, a.Mbid); err != nil {
				stats.Errored++
				logger.Error("artist art backfill: record attempt failed (unmatched)",
					slog.String("artist_mbid", a.Mbid),
					slog.String("artist_name", a.Name),
					slog.String("error", err.Error()),
				)
			} else {
				stats.Unmatched++
			}
			stats.Visited++
			continue
		}

		// res.Matched: at least one of DeezerID/ImageURL is non-nil.
		// Passing the artist's own already-stored Mbid/Name back unchanged
		// is correct and required -- UpsertArtist refreshes name
		// unconditionally, so supplying the stored value is a no-op write,
		// while Disambiguation: nil is preserved by that query's COALESCE
		// clause.
		if _, err := store.UpsertArtist(ctx, sqlc.UpsertArtistParams{
			Mbid:           a.Mbid,
			Name:           a.Name,
			Disambiguation: nil,
			DeezerID:       res.DeezerID,
			ImageUrl:       res.ImageURL,
		}); err != nil {
			// The write didn't land -- skip RecordArtMatchAttempt too, so a
			// future sweep re-tries this artist rather than treating an
			// unpersisted match as a considered fail-closed decision.
			stats.Errored++
			logger.Error("artist art backfill: upsert artist failed",
				slog.String("artist_mbid", a.Mbid),
				slog.String("artist_name", a.Name),
				slog.String("error", err.Error()),
			)
			stats.Visited++
			continue
		}

		if err := store.RecordArtMatchAttempt(ctx, a.Mbid); err != nil {
			// The UpsertArtist write already landed and is not rolled
			// back -- a missed attempt-timestamp write only costs an extra
			// re-check next sweep, a much smaller problem than losing the
			// art match itself.
			stats.Errored++
			logger.Error("artist art backfill: record attempt failed (matched)",
				slog.String("artist_mbid", a.Mbid),
				slog.String("artist_name", a.Name),
				slog.String("error", err.Error()),
			)
			stats.Visited++
			continue
		}

		stats.Matched++
		stats.Visited++
	}

	logger.Info("artist art backfill complete",
		slog.Int("visited", stats.Visited),
		slog.Int("matched", stats.Matched),
		slog.Int("unmatched", stats.Unmatched),
		slog.Int("errored", stats.Errored),
		slog.Float64("match_rate_percent", stats.MatchRatePercent()),
	)

	return stats, nil
}
