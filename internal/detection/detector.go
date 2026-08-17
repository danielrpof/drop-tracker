// Package detection implements the diff-based event detection Phase 4 owns
// (DTCT-01 through DTCT-05): it diffs a poll cycle's freshly fetched
// results against the events table -- the "seen" store (D-09) -- and
// records each previously-unseen item as an event row, idempotently at the
// database level via InsertEvent's ON CONFLICT DO NOTHING (D-20). Detector
// implements poller.EventRecorder, the narrow seam RunMusicBrainzCycle
// calls at the end of its per-artist loop.
package detection

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
)

// RecordingSource is the narrow seam DetectMusicBrainz's guest-feature pass
// depends on (DTCT-03) -- declared here, in the consumer, mirroring
// poller.ReleaseGroupSource/AlbumSource and this package's own reliance on
// sqlc.Querier (an interface, not a concrete *sqlc.Queries) rather than
// *musicbrainz.Client directly, so a test can substitute a stub with no real
// HTTP client.
type RecordingSource interface {
	RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error)
}

// ReleaseDetailSource is the narrow seam DetectMusicBrainz's deluxe-change
// pass depends on (DTCT-02) -- declared here, in the consumer, mirroring
// RecordingSource, so a test can substitute a stub with no real HTTP
// client. Called only for release-groups already in the seen store (D-04);
// detectDeluxeChanges never calls this for a group discovered in the same
// cycle.
type ReleaseDetailSource interface {
	ReleasesByReleaseGroup(ctx context.Context, groupMBID string) ([]musicbrainz.Release, error)
}

// Detector wraps sqlc.Querier, a RecordingSource and a ReleaseDetailSource
// -- the consuming package declares its own narrower interface
// (poller.EventRecorder) rather than this type depending on one, mirroring
// watchlist.Store/Service's split.
type Detector struct {
	q          sqlc.Querier
	recordings RecordingSource
	releases   ReleaseDetailSource
}

// New builds a Detector backed by q for the seen-store, recordings for
// DTCT-03's guest-feature pass, and releases for DTCT-02's deluxe-change
// pass.
func New(q sqlc.Querier, recordings RecordingSource, releases ReleaseDetailSource) *Detector {
	return &Detector{q: q, recordings: recordings, releases: releases}
}

// insertEvent calls InsertEvent and reports whether the row was newly
// inserted. 0 affected rows is not an error -- it means the
// (event_type, source, external_id) dedup key already existed (D-20), i.e.
// "already seen," not "something went wrong."
func (d *Detector) insertEvent(ctx context.Context, params sqlc.InsertEventParams) (newlyDetected bool, err error) {
	affected, err := d.q.InsertEvent(ctx, params)
	if err != nil {
		return false, fmt.Errorf("detection: insert event: %w", err)
	}
	return affected > 0, nil
}

// isSeedMode reports whether artistID's first-ever detection cycle for
// source has arrived -- D-14's implicit detection: zero existing event rows
// for this (artist_id, source) pair means seed mode, with no explicit
// seeded_at column. Scoped per source (D-15) so an artist whose deezer_id is
// backfilled long after being added seeds their Deezer catalogue
// independently instead of having it dumped as new releases the first time
// Deezer data appears.
//
// Known, accepted edge: because the dedup key excludes artist_id (D-10), an
// artist whose entire catalogue was already recorded under a collaborator
// (a different artist's cycle inserted the same external_id first) keeps
// zero rows of their own and therefore stays in seed mode indefinitely.
// That is the documented consequence of D-10 plus D-14, not a defect this
// method works around.
func (d *Detector) isSeedMode(ctx context.Context, artistID int64, source string) (bool, error) {
	hasAny, err := d.q.HasAnyEvent(ctx, sqlc.HasAnyEventParams{ArtistID: artistID, Source: source})
	if err != nil {
		return false, fmt.Errorf("detection: has any event: %w", err)
	}
	return !hasAny, nil
}

// seedNotifiedAt returns the notified_at value a Detect* call should store
// for every row it inserts this cycle: in seed mode, a single
// time.Now().UTC() captured once (never re-read per row, so every row from
// one seed cycle shares one timestamp -- D-13); otherwise the zero-value
// pgtype.Timestamptz, which encodes SQL NULL.
func seedNotifiedAt(seedMode bool) pgtype.Timestamptz {
	if !seedMode {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
}

// nullableString turns an empty string into a nil *string. MusicBrainz
// returns "" (never omits the field) for a group's undated
// first-release-date, and this project's *string column convention treats
// SQL NULL, not an empty string literal, as "no value."
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// advanceGroupBaseline atomically compares count against groupMBID's
// currently-committed track-count baseline (04-01's option-a resolution: a
// mutable track_count column on the group's own new_release event row) and
// writes it as the new baseline only if it is a genuine increase (or no
// baseline exists yet) -- replacing the former groupBaseline (SELECT) +
// setGroupBaseline (UPDATE) pair, whose two-round-trip gap PERF-04 exists
// to close (11-RESEARCH.md Pattern 2). advanced reports whether the write
// landed; hadBaseline/previousBaseline are only meaningful when advanced is
// true.
//
// Distinguishing "no baseline recorded yet" (hadBaseline false) from
// "baseline recorded as zero" is the entire mechanism preventing the
// false-positive 04-RESEARCH.md Pitfall #1 describes -- a caller that
// collapsed both to zero would report a real first-ever fetch as "the count
// increased from 0," firing a spurious deluxe_change on every
// release-group's first real comparison cycle. This is operational
// baseline state, not the D-12 display snapshot (title/artist_name/
// release_date/cover_art_url), which stays write-once via InsertEvent's ON
// CONFLICT DO NOTHING per D-20; no snapshot column is ever touched by this
// call.
func (d *Detector) advanceGroupBaseline(ctx context.Context, groupMBID string, count int) (advanced, hadBaseline bool, previousBaseline int, err error) {
	trackCount := int32(count) //nolint:gosec // count is a real-world album/release track count (always well under int32 range)
	rows, err := d.q.AdvanceGroupTrackCountBaseline(ctx, sqlc.AdvanceGroupTrackCountBaselineParams{
		ExternalID: groupMBID,
		TrackCount: &trackCount,
	})
	if err != nil {
		return false, false, 0, fmt.Errorf("detection: advance group baseline: %w", err)
	}
	if len(rows) == 0 {
		return false, false, 0, nil
	}
	previous := rows[0]
	if previous == nil {
		return true, false, 0, nil
	}
	return true, true, int(*previous), nil
}
