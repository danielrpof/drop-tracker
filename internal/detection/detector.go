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
// HTTP client. ReleasesForRecording (D-01) is called once per
// newly-detected, previously-unseen recording, only on insert -- never for a
// recording already in the seen store.
type RecordingSource interface {
	RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error)
	ReleasesForRecording(ctx context.Context, mbid string) ([]musicbrainz.RecordingRelease, error)
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

// DefaultNotifyMaxReleaseAgeDays is how old a release may be, by its own
// release date, and still be delivered to Discord. See notifyGate for why
// this gate exists at all; this constant is only about choosing the
// window's width.
//
// Zero would be the literal reading of "only notify me about releases
// dated today," but it is the wrong default, because it converts a
// false-positive bug into a silent-loss bug -- strictly the worse of the
// two. Three independent lags sit between "a release came out" and "this
// process inserts a row for it": MusicBrainz is community-edited, so a
// release routinely lands in the database hours or days after it actually
// dropped; the poll cycle itself runs on PollInterval (15m by default) and
// skips overlapping ticks, and the guest-feature pass caps itself at
// maxNewGuestFeatureLookupsPerCycle so one artist's new items can legally
// arrive a cycle or more later; and release_date carries no timezone at
// all, so a release that is "today" in one region is already "yesterday"
// in UTC. Under a zero-day window every one of those normal delays
// silently and permanently drops a real alert -- the row is stamped
// notified and never reconsidered.
//
// Seven days absorbs all three lags while still suppressing back
// catalogue completely: measured against the production data that
// prompted this gate, a 7-day window suppresses 242 of the 249
// back-catalogue notifications sent in one day (the remainder are undated
// rows, deliberately let through per notifyGate's doctrine). The asymmetry
// mirrors deluxeRecheckWindowDays' own reasoning -- guessing too short
// risks a permanently missed alert, guessing too long costs only a small
// number of extra alerts on the first cycle after this ships -- so the
// window errs generous. Operators who genuinely want the strict reading
// can set NOTIFY_MAX_RELEASE_AGE_DAYS=0.
const DefaultNotifyMaxReleaseAgeDays = 7

// Detector wraps sqlc.Querier, a RecordingSource and a ReleaseDetailSource
// -- the consuming package declares its own narrower interface
// (poller.EventRecorder) rather than this type depending on one, mirroring
// watchlist.Store/Service's split.
type Detector struct {
	q                       sqlc.Querier
	recordings              RecordingSource
	releases                ReleaseDetailSource
	notifyMaxReleaseAgeDays int
}

// Option customises a Detector at construction. Declared as a variadic
// option func rather than as extra New parameters, mirroring
// poller.Option/WithMusicBrainzWorkers -- the established idiom in this
// codebase for a setting with a sane default that only main.go overrides.
type Option func(*Detector)

// WithNotifyMaxReleaseAgeDays overrides DefaultNotifyMaxReleaseAgeDays.
// Zero is a meaningful, accepted value ("only releases dated today are
// delivered") and is deliberately NOT treated as "unset"; a negative value
// is rejected at config-parse time, not silently clamped here.
func WithNotifyMaxReleaseAgeDays(days int) Option {
	return func(d *Detector) { d.notifyMaxReleaseAgeDays = days }
}

// New builds a Detector backed by q for the seen-store, recordings for
// DTCT-03's guest-feature pass, and releases for DTCT-02's deluxe-change
// pass.
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

// notifyGate decides the notified_at value every row inserted during one
// Detect* call carries: the zero-value pgtype.Timestamptz (SQL NULL, i.e.
// "queued for Discord") or a real timestamp (already acked, so the row is
// recorded in history but never delivered).
//
// It replaces the former seedNotifiedAt, which asked only "is this the
// artist's first-ever cycle for this source?" -- a decision based purely on
// INSERT TIMING. That is the AND-gate root cause of
// .planning/debug/resolved/backlog-songs-trigger-discord.md: seed mode is a
// one-shot latch that flips off the instant the first event row for an
// (artist_id, source) pair exists, but detectGuestFeatures is deliberately
// MULTI-cycle (maxNewGuestFeatureLookupsPerCycle caps one artist at 20
// lookups per cycle, and per-recording lookup errors defer their recordings
// too). An artist with a large back catalogue therefore kept inserting
// backlog rows on later, non-seed cycles, each born notified_at = NULL, and
// ListUnnotified applies no release-date predicate -- so the whole back
// catalogue drained to Discord 20 rows per poll cycle, indefinitely. In
// production that sent 242 back-catalogue releases reaching back to 2015 in
// a single day.
//
// The fix is to judge a STABLE PER-ROW PROPERTY (is this release actually
// recent?) instead of a fragile temporal latch. Because freshness is
// re-evaluated for every inserted row, correctness no longer depends on
// whether an artist's backlog happened to fit inside one cycle -- it is
// immune to the lookup cap, per-recording lookup errors, rate-limiter
// stalls, cycle boundaries and process restarts, every one of which can and
// does push backlog rows past the seed window. seedMode is retained
// unchanged as a first-cycle belt-and-braces suppressor, not as the
// mechanism.
type notifyGate struct {
	seedMode bool
	cutoff   string
	now      pgtype.Timestamptz
}

// newNotifyGate captures now ONCE per Detect* call, never per row. That is
// D-13: every row a single seed cycle inserts must share one identical
// notified_at, which is also what makes ListUnnotified's
// (created_at, id) total order well-defined (see queries/events.sql). It is
// the same single-capture-per-call reasoning detectDeluxeChanges applies to
// its own recheck-window cutoff, and it additionally stops a long-running
// pass that straddles midnight from judging its first and last rows by two
// different cutoffs.
func newNotifyGate(seedMode bool, maxAgeDays int, now time.Time) notifyGate {
	now = now.UTC()
	return notifyGate{
		seedMode: seedMode,
		cutoff:   now.AddDate(0, 0, -maxAgeDays).Format(time.DateOnly),
		now:      pgtype.Timestamptz{Time: now, Valid: true},
	}
}

// notifiedAt returns the pre-acked timestamp for a row that must NOT reach
// Discord, or the zero value (SQL NULL) for one that must. Suppression
// never suppresses DETECTION -- the row is still inserted and still shows
// up in the History feed; only delivery is gated.
func (g notifyGate) notifiedAt(releaseDate string) pgtype.Timestamptz {
	if g.seedMode || !onOrAfterCutoff(releaseDate, g.cutoff) {
		return g.now
	}
	return pgtype.Timestamptz{}
}

// onOrAfterCutoff reports whether releaseDate is a full YYYY-MM-DD date at
// or after cutoff. Both are compared as strings, never parsed into
// time.Time, for the same reason withinDeluxeRecheckWindow does the same:
// MusicBrainz release dates are legitimately partial (year-only "2015",
// year-month "2015-06") and time.Parse would reject them, so a parse-based
// gate would have to invent a policy for a parse error anyway.
// Zero-padded ISO-8601 dates sort lexicographically exactly as they sort
// chronologically, which is what makes the plain >= correct here.
//
// A partial or absent date returns false (suppress) via the length check.
// This is deliberately the conservative reading and the opposite of
// isGuestFeature/withinDeluxeRecheckWindow's "err toward an extra alert"
// doctrine: 64% of guest_feature rows carry no release date at all, so an
// undated row is absence of evidence, not evidence of freshness, and
// admitting them would re-open the flood this gate exists to stop. Kept
// byte-for-byte equivalent to notifier.suppresses' negation -- the two
// halves of this gate must agree, and notifygate_test.go plus
// suppress_test.go pin both sides to the same table of cases.
func onOrAfterCutoff(releaseDate, cutoff string) bool {
	return len(releaseDate) == len(time.DateOnly) && releaseDate >= cutoff
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
