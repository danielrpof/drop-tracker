package notifier

// This file is package notifier (whitebox), not notifier_test -- suppresses
// and staleReleaseDate are unexported, mirroring format_test.go's own
// convention in this package (and reusing its strPtr helper). Pure
// table-driven unit tests, no DB, no HTTP.
//
// The bug these tests exist to catch
// (.planning/debug/resolved/backlog-songs-trigger-discord.md): notification
// suppression used to be decided purely by INSERT TIMING -- seed mode, a
// one-shot latch that flips off the instant an (artist_id, source) pair has
// any event row. Because detectGuestFeatures is deliberately multi-cycle,
// an artist's back catalogue kept being inserted on later, non-seed cycles
// with notified_at = NULL, and ListUnnotified applies no release-date
// predicate at all -- so the whole back catalogue drained to Discord. This
// package owns the DELIVERY-side half of the fix.
//
// The case table below is deliberately the same table as
// internal/detection/notifygate_test.go's TestNotifyGate_NotifiedAt, with
// the same cutoff and the same expectations. That is the point: the two
// halves of the freshness gate must agree, and a future edit to one side
// that silently diverges from the other should fail here.

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

// gateTestCutoff is a fixed cutoff, so no case below depends on wall-clock
// time -- a test that computed its own dates relative to now could straddle
// midnight and flake on the boundary rows. It corresponds to detection's
// own fixture: "now" = 2026-08-26 with a 7-day window.
const gateTestCutoff = "2026-08-19"

// gateCases is the shared truth table for the freshness gate.
var gateCases = []struct {
	name         string
	releaseDate  *string
	wantSuppress bool
}{
	{
		name:         "backlog release is suppressed",
		releaseDate:  strPtr("2015-05-21"),
		wantSuppress: true,
	},
	{
		name:         "release dated today is delivered",
		releaseDate:  strPtr("2026-08-26"),
		wantSuppress: false,
	},
	{
		name:         "release inside the window is delivered",
		releaseDate:  strPtr("2026-08-22"),
		wantSuppress: false,
	},
	{
		name:         "release exactly on the cutoff is delivered (inclusive boundary)",
		releaseDate:  strPtr(gateTestCutoff),
		wantSuppress: false,
	},
	{
		name:         "release one day before the cutoff is suppressed",
		releaseDate:  strPtr("2026-08-18"),
		wantSuppress: true,
	},
	{
		name:         "future-dated release is delivered",
		releaseDate:  strPtr("2026-08-28"),
		wantSuppress: false,
	},
	{
		name:         "backlog year-only date is suppressed",
		releaseDate:  strPtr("2015"),
		wantSuppress: true,
	},
	{
		name:         "backlog year-month date is suppressed",
		releaseDate:  strPtr("2015-06"),
		wantSuppress: true,
	},
	// A year-only date for the CURRENT year is pinned separately because it
	// is the case that would survive a dropped length check by accident:
	// the string comparison alone already suppresses it ("2026" sorts before
	// "2026-08-19"), so only an explicit case documents that the length
	// check is what is meant to decide partial dates.
	{
		name:         "current-year year-only date is suppressed",
		releaseDate:  strPtr("2026"),
		wantSuppress: true,
	},
	{
		name:         "malformed short date is suppressed",
		releaseDate:  strPtr("20"),
		wantSuppress: true,
	},
	{
		name:         "empty date string is suppressed",
		releaseDate:  strPtr(""),
		wantSuppress: true,
	},
	// The load-bearing case for this fix's review correction. An undated row
	// (SQL NULL) must SUPPRESS, matching detection.onOrAfterCutoff, whose
	// own "non-seed absent date is suppressed" case pins the identical
	// behaviour on the insert side. An earlier draft of suppresses returned
	// false here ("deliver"), which would have re-flooded Discord on the
	// first post-fix pass: 64% of guest_feature rows carry no release date
	// at all.
	{
		name:         "absent date (SQL NULL) is suppressed",
		releaseDate:  nil,
		wantSuppress: true,
	},
}

func TestStaleReleaseDate(t *testing.T) {
	for _, tt := range gateCases {
		t.Run(tt.name, func(t *testing.T) {
			got := staleReleaseDate(tt.releaseDate, gateTestCutoff)
			if got != tt.wantSuppress {
				verb := map[bool]string{true: "suppressed", false: "delivered"}
				t.Fatalf("staleReleaseDate(%s, %q) was %s, want %s",
					fmtDate(tt.releaseDate), gateTestCutoff, verb[got], verb[tt.wantSuppress])
			}
		})
	}
}

// TestNotifierSuppresses_WiresMaxAgeDays proves suppresses actually consults
// n.maxAgeDays and ev.CreatedAt (D-02), not just staleReleaseDate in
// isolation -- the table test above pins the predicate, this pins the
// wiring. Every case is computed relative to a single fixed anchor instant,
// so no assertion can flake on a midnight straddle even at the exact slack
// boundary D-02 introduces.
func TestNotifierSuppresses_WiresMaxAgeDays(t *testing.T) {
	anchor := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	day := func(offset int) *string {
		s := anchor.AddDate(0, 0, offset).Format(time.DateOnly)
		return &s
	}
	createdAt := func(t time.Time) pgtype.Timestamptz {
		return pgtype.Timestamptz{Time: t, Valid: true}
	}

	tests := []struct {
		name         string
		maxAgeDays   int
		createdAt    time.Time
		releaseDate  *string
		wantSuppress bool
	}{
		{
			name:         "default window delivers a release from yesterday",
			maxAgeDays:   defaultMaxReleaseAgeDays,
			createdAt:    anchor,
			releaseDate:  day(-1),
			wantSuppress: false,
		},
		{
			name:         "default window suppresses a release from 30 days ago",
			maxAgeDays:   defaultMaxReleaseAgeDays,
			createdAt:    anchor,
			releaseDate:  day(-30),
			wantSuppress: true,
		},
		{
			// The strict reading an operator gets from
			// NOTIFY_MAX_RELEASE_AGE_DAYS=0: only a release dated today (or,
			// per D-02's slack, yesterday) survives.
			name:         "zero window suppresses a release from 3 days ago",
			maxAgeDays:   0,
			createdAt:    anchor,
			releaseDate:  day(-3),
			wantSuppress: true,
		},
		{
			name:         "zero window still delivers a release dated today",
			maxAgeDays:   0,
			createdAt:    anchor,
			releaseDate:  day(0),
			wantSuppress: false,
		},
		{
			// A wide window must reach back far enough to deliver a release
			// the default window would have suppressed -- proving the field
			// is read rather than a constant being applied.
			name:         "wide window delivers a release from 30 days ago",
			maxAgeDays:   365,
			createdAt:    anchor,
			releaseDate:  day(-30),
			wantSuppress: false,
		},
		{
			// D-02, the point of the change: an event detected 60 days ago
			// with a release dated 2 days before its own created_at is
			// delivered -- fresh relative to when it was detected. Under a
			// wall-clock anchor this would be suppressed.
			name:         "an event detected 60 days ago with a release just before its own created_at is delivered",
			maxAgeDays:   defaultMaxReleaseAgeDays,
			createdAt:    anchor.AddDate(0, 0, -60),
			releaseDate:  strPtr(anchor.AddDate(0, 0, -62).Format(time.DateOnly)),
			wantSuppress: false,
		},
		{
			// The pre-fix backlog guard survives the re-anchor: a 2015
			// release is old relative to its own created_at, not merely
			// relative to now.
			name:         "a pre-fix backlog row detected 60 days ago with a 2015 release date is still suppressed",
			maxAgeDays:   defaultMaxReleaseAgeDays,
			createdAt:    anchor.AddDate(0, 0, -60),
			releaseDate:  strPtr("2015-05-21"),
			wantSuppress: true,
		},
		{
			// D-02's 1-day slack, inclusive edge: created_at minus
			// (maxAgeDays + 1) days is delivered.
			name:         "a release dated exactly on the slack boundary is delivered",
			maxAgeDays:   7,
			createdAt:    anchor,
			releaseDate:  strPtr(anchor.AddDate(0, 0, -8).Format(time.DateOnly)),
			wantSuppress: false,
		},
		{
			name:         "a release one day beyond the slack boundary is suppressed",
			maxAgeDays:   7,
			createdAt:    anchor,
			releaseDate:  strPtr(anchor.AddDate(0, 0, -9).Format(time.DateOnly)),
			wantSuppress: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(nil, nil, nil, 0, WithMaxReleaseAgeDays(tt.maxAgeDays))
			got := n.suppresses(sqlc.Event{ReleaseDate: tt.releaseDate, CreatedAt: createdAt(tt.createdAt)})
			if got != tt.wantSuppress {
				verb := map[bool]string{true: "suppressed", false: "delivered"}
				t.Fatalf("suppresses(release_date=%s, created_at=%s) with maxAgeDays=%d was %s, want %s",
					fmtDate(tt.releaseDate), tt.createdAt.Format(time.DateOnly), tt.maxAgeDays, verb[got], verb[tt.wantSuppress])
			}
		})
	}
}

// TestNewDefaultsMaxReleaseAgeDays pins that a Notifier built with no
// options is gated, not ungated. A zero-value maxAgeDays would silently
// mean "only today's releases," and -- worse -- an omitted default here
// would not fail any other test in this package, since every other case
// passes an explicit option.
func TestNewDefaultsMaxReleaseAgeDays(t *testing.T) {
	if got := New(nil, nil, nil, 0).maxAgeDays; got != defaultMaxReleaseAgeDays {
		t.Fatalf("New(...).maxAgeDays = %d, want defaultMaxReleaseAgeDays (%d)", got, defaultMaxReleaseAgeDays)
	}
}

// fmtDate renders a *string release date for failure messages, so a nil
// (SQL NULL) case is visually distinct from an empty-string one -- the two
// are different inputs that happen to share an expectation.
func fmtDate(d *string) string {
	if d == nil {
		return "<nil>"
	}
	return `"` + *d + `"`
}
