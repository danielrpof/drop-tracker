// Package detection implements Phase 4's diff-based event detection (DTCT-01
// through DTCT-05): it diffs a poll cycle's fresh results against the events
// table -- the "seen" store (D-09) -- and records each unseen item as an event
// row, idempotent via InsertEvent's ON CONFLICT DO NOTHING (D-20). Detector
// implements poller.EventRecorder.
package detection

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
)

// RecordingSource is the guest-feature pass's narrow seam (DTCT-03), declared in
// the consumer so a test can stub it with no HTTP client. ReleasesForRecording
// (D-01) is called once per newly-detected recording, only on insert.
type RecordingSource interface {
	RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error)
	ReleasesForRecording(ctx context.Context, mbid string) ([]musicbrainz.RecordingRelease, error)
}

// ReleaseDetailSource is the deluxe-change pass's narrow seam (DTCT-02), declared
// in the consumer like RecordingSource. Called only for release-groups already
// in the seen store (D-04), never one discovered in the same cycle.
type ReleaseDetailSource interface {
	ReleasesByReleaseGroup(ctx context.Context, groupMBID string) ([]musicbrainz.Release, error)
}

// DefaultNotifyMaxReleaseAgeDays is how old a release may be and still be
// delivered to Discord (see notifyGate for why the gate exists). Not zero: the
// MusicBrainz edit lag, the poll interval plus guest-feature lookup cap, and
// timezone-less release_date would each silently drop a real alert. Seven days
// absorbs all three while still suppressing back catalogue (242 of 249 sends
// suppressed; 04-RESEARCH.md). Operators can set NOTIFY_MAX_RELEASE_AGE_DAYS=0.
const DefaultNotifyMaxReleaseAgeDays = 7

// Detector wraps sqlc.Querier, a RecordingSource and a ReleaseDetailSource. The
// consuming package declares its own narrower poller.EventRecorder.
type Detector struct {
	q                       sqlc.Querier
	recordings              RecordingSource
	releases                ReleaseDetailSource
	notifyMaxReleaseAgeDays int
}

// Option customises a Detector at construction -- a variadic option func
// mirroring poller.Option, the codebase idiom for a defaulted setting main.go overrides.
type Option func(*Detector)

// WithNotifyMaxReleaseAgeDays overrides DefaultNotifyMaxReleaseAgeDays. Zero is
// meaningful ("only releases dated today"), not "unset"; negatives rejected at config parse.
func WithNotifyMaxReleaseAgeDays(days int) Option {
	return func(d *Detector) { d.notifyMaxReleaseAgeDays = days }
}

// New builds a Detector backed by q for the seen-store, recordings for DTCT-03's
// guest-feature pass, and releases for DTCT-02's deluxe-change pass.
func New(q sqlc.Querier, recordings RecordingSource, releases ReleaseDetailSource, opts ...Option) *Detector {
	d := &Detector{
		q:                       q,
		recordings:              recordings,
		releases:                releases,
		notifyMaxReleaseAgeDays: DefaultNotifyMaxReleaseAgeDays,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// insertEvent calls InsertEvent and reports whether the row was newly inserted.
// 0 affected rows is not an error -- the (event_type, source, external_id) dedup
// key already existed (D-20): "already seen", not a failure.
func (d *Detector) insertEvent(ctx context.Context, params sqlc.InsertEventParams) (newlyDetected bool, err error) {
	affected, err := d.q.InsertEvent(ctx, params)
	if err != nil {
		return false, fmt.Errorf("detection: insert event: %w", err)
	}
	return affected > 0, nil
}

// isSeedMode reports whether artistID's first-ever detection cycle for source has
// arrived -- D-14's implicit detection (zero event rows for the (artist_id,
// source) pair, no seeded_at column), scoped per source (D-15) so a late
// deezer_id backfill seeds independently.
// Accepted edge: the dedup key excludes artist_id (D-10), so an artist whose
// whole catalogue was first recorded under a collaborator stays in seed mode.
func (d *Detector) isSeedMode(ctx context.Context, artistID int64, source string) (bool, error) {
	hasAny, err := d.q.HasAnyEvent(ctx, sqlc.HasAnyEventParams{ArtistID: artistID, Source: source})
	if err != nil {
		return false, fmt.Errorf("detection: has any event: %w", err)
	}
	return !hasAny, nil
}

// notifyGate decides the notified_at every row from one Detect* call carries:
// zero-value pgtype.Timestamptz (SQL NULL, "queued for Discord") or a real
// timestamp ("acked, history only"). It replaces seedNotifiedAt, which judged
// INSERT TIMING -- the AND-gate root cause of
// .planning/debug/resolved/backlog-songs-trigger-discord.md, where a multi-cycle
// backlog kept draining to Discord because seed mode is a one-shot latch. The fix
// judges a STABLE PER-ROW PROPERTY (is the release recent?); seedMode is kept
// only as a first-cycle belt-and-braces suppressor.
type notifyGate struct {
	seedMode bool
	cutoff   string
	now      pgtype.Timestamptz
}

// newNotifyGate captures now ONCE per Detect* call, never per row (D-13): every
// row a seed cycle inserts shares one notified_at, which also makes
// ListUnnotified's (created_at, id) order well-defined and stops a
// midnight-straddling pass judging its rows by two cutoffs.
func newNotifyGate(seedMode bool, maxAgeDays int, now time.Time) notifyGate {
	now = now.UTC()
	return notifyGate{
		seedMode: seedMode,
		cutoff:   now.AddDate(0, 0, -maxAgeDays).Format(time.DateOnly),
		now:      pgtype.Timestamptz{Time: now, Valid: true},
	}
}

// notifiedAt returns the pre-acked timestamp for a row that must NOT reach
// Discord, or the zero value (SQL NULL) for one that must. Suppression gates
// delivery only -- the row is still inserted and still shows in History.
func (g notifyGate) notifiedAt(releaseDate string) pgtype.Timestamptz {
	if g.seedMode || !onOrAfterCutoff(releaseDate, g.cutoff) {
		return g.now
	}
	return pgtype.Timestamptz{}
}

// onOrAfterCutoff reports whether releaseDate is a full YYYY-MM-DD at or after
// cutoff, compared as strings (MusicBrainz dates are legitimately partial, and
// zero-padded ISO-8601 sorts chronologically). A partial or absent date returns
// false (suppress) -- conservative, opposite isGuestFeature/withinDeluxeRecheckWindow,
// because an undated row is absence of evidence. Kept byte-equivalent to
// notifier.suppresses' negation; both sides are pinned to one test table.
func onOrAfterCutoff(releaseDate, cutoff string) bool {
	return len(releaseDate) == len(time.DateOnly) && releaseDate >= cutoff
}

// nullableString turns "" into a nil *string. MusicBrainz returns "" (never
// omits) for a group's undated first-release-date, and this project's *string
// columns treat SQL NULL, not "", as "no value".
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// advanceGroupBaseline atomically compares count against groupMBID's committed
// track-count baseline (04-01: a mutable track_count column on the group's
// new_release row) and writes it only on a genuine increase -- one round trip,
// closing the SELECT+UPDATE gap PERF-04 targets (11-RESEARCH.md Pattern 2).
// Distinguishing "no baseline yet" from "baseline is zero" prevents the
// 04-RESEARCH.md Pitfall #1 false positive. This is operational state, not the
// write-once D-12 display snapshot (D-20); no snapshot column is touched.
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
