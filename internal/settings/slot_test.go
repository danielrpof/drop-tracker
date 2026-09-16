package settings_test

// Table tests for internal/settings/slot.go's calendar-based slot math
// (D-11, D-15). loc is loaded once per test via settings.ZoneName -- no
// zoneinfo skip here (plan 22-01's own scope, unlike plan 22-04's Alpine
// boot check): the host and CI both carry zoneinfo.

import (
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/settings"
)

func mustLoadNY(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(settings.ZoneName)
	if err != nil {
		t.Fatalf("time.LoadLocation(%q): %v", settings.ZoneName, err)
	}
	return loc
}

func TestMostRecentSlot_Daily(t *testing.T) {
	loc := mustLoadNY(t)

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "mid-morning returns today's slot",
			now:  time.Date(2026, 6, 10, 9, 30, 0, 0, loc),
			want: time.Date(2026, 6, 10, 0, 5, 0, 0, loc),
		},
		{
			name: "one minute before today's slot returns yesterday's",
			now:  time.Date(2026, 6, 10, 0, 4, 0, 0, loc),
			want: time.Date(2026, 6, 9, 0, 5, 0, 0, loc),
		},
		{
			name: "exactly at the slot instant counts as arrived",
			now:  time.Date(2026, 6, 10, 0, 5, 0, 0, loc),
			want: time.Date(2026, 6, 10, 0, 5, 0, 0, loc),
		},
		{
			name: "a UTC-valued now still resolves against loc",
			now:  time.Date(2026, 6, 10, 3, 0, 0, 0, time.UTC), // 23:00 on 2026-06-09 in NY
			want: time.Date(2026, 6, 9, 0, 5, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := settings.MostRecentSlot(tc.now, settings.CadenceDaily, loc)
			if !got.Equal(tc.want) {
				t.Fatalf("MostRecentSlot(%v, daily) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

func TestMostRecentSlot_Weekly(t *testing.T) {
	loc := mustLoadNY(t)

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "mid-week (Wednesday) returns the preceding Friday",
			now:  time.Date(2026, 6, 10, 9, 30, 0, 0, loc),
			want: time.Date(2026, 6, 5, 0, 5, 0, 0, loc),
		},
		{
			name: "Friday before the slot returns the prior Friday",
			now:  time.Date(2026, 6, 12, 0, 4, 0, 0, loc),
			want: time.Date(2026, 6, 5, 0, 5, 0, 0, loc),
		},
		{
			name: "Friday at/after the slot returns this Friday",
			now:  time.Date(2026, 6, 12, 0, 5, 0, 0, loc),
			want: time.Date(2026, 6, 12, 0, 5, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := settings.MostRecentSlot(tc.now, settings.CadenceWeekly, loc)
			if !got.Equal(tc.want) {
				t.Fatalf("MostRecentSlot(%v, weekly) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

func TestGraceFor(t *testing.T) {
	if got := settings.GraceFor(settings.CadenceDaily); got != 12*time.Hour {
		t.Fatalf("GraceFor(daily) = %v, want 12h", got)
	}
	if got := settings.GraceFor(settings.CadenceWeekly); got != 48*time.Hour {
		t.Fatalf("GraceFor(weekly) = %v, want 48h", got)
	}
}

// sweepDistinctSlots walks [start, end) in 5-minute real-time increments,
// calling settings.MostRecentSlot(t, cadence, loc) at each step and
// collecting the distinct results keyed by unix instant. Plan 22-04's
// scheduler-level DST tests reuse this same stepping instrument.
func sweepDistinctSlots(loc *time.Location, cadence settings.Cadence, start, end time.Time) map[int64]time.Time {
	const step = 5 * time.Minute
	slots := make(map[int64]time.Time)
	for cur := start; cur.Before(end); cur = cur.Add(step) {
		got := settings.MostRecentSlot(cur, cadence, loc)
		slots[got.Unix()] = got
	}
	return slots
}

// TestMostRecentSlot_DailyDSTTransitions proves a 23-hour spring-forward day
// and a 25-hour fall-back day each still resolve to exactly one 00:05 local
// slot (D-11). Each sweep is bounded to [this date's own slot instant, the
// next date's own slot instant) -- deliberately not calendar midnight to
// midnight -- so every sampled instant falls within exactly one calendar
// date's territory. Starting the sweep at midnight would let its very first
// sample (00:00, strictly before that date's own 00:05 slot) return the
// PREVIOUS date's slot instead, mixing two distinct dates' slot values into
// what should be a single date's proof.
func TestMostRecentSlot_DailyDSTTransitions(t *testing.T) {
	loc := mustLoadNY(t)

	cases := []struct {
		name       string
		start      time.Time
		end        time.Time
		wantOffset string
	}{
		{
			name:       "2026-03-08 spring-forward (23h day)",
			start:      time.Date(2026, 3, 8, 0, 5, 0, 0, loc),
			end:        time.Date(2026, 3, 9, 0, 5, 0, 0, loc),
			wantOffset: "-05:00",
		},
		{
			name:       "2026-11-01 fall-back (25h day)",
			start:      time.Date(2026, 11, 1, 0, 5, 0, 0, loc),
			end:        time.Date(2026, 11, 2, 0, 5, 0, 0, loc),
			wantOffset: "-04:00",
		},
		{
			name:  "2027-03-14 spring-forward (23h day)",
			start: time.Date(2027, 3, 14, 0, 5, 0, 0, loc),
			end:   time.Date(2027, 3, 15, 0, 5, 0, 0, loc),
		},
		{
			name:  "2027-11-07 fall-back (25h day)",
			start: time.Date(2027, 11, 7, 0, 5, 0, 0, loc),
			end:   time.Date(2027, 11, 8, 0, 5, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slots := sweepDistinctSlots(loc, settings.CadenceDaily, tc.start, tc.end)
			if len(slots) != 1 {
				t.Fatalf("distinct daily slots = %d, want 1: %v", len(slots), slots)
			}
			var only time.Time
			for _, v := range slots {
				only = v
			}
			if !only.Equal(tc.start) {
				t.Fatalf("the one distinct slot = %v, want %v", only, tc.start)
			}
			if tc.wantOffset != "" {
				if got := only.Format("-07:00"); got != tc.wantOffset {
					t.Fatalf("slot offset = %q, want %q", got, tc.wantOffset)
				}
			}
		})
	}
}

// TestMostRecentSlot_WeeklyDSTTransitions proves the weekly cadence resolves
// to exactly one Friday 00:05 slot across each calendar week containing a
// DST transition (D-11), using the same slot-instant-to-slot-instant sweep
// bound as the daily test above, scaled to a Friday-to-Friday week.
func TestMostRecentSlot_WeeklyDSTTransitions(t *testing.T) {
	loc := mustLoadNY(t)

	cases := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{
			name:  "week containing 2026-03-08 spring-forward",
			start: time.Date(2026, 3, 6, 0, 5, 0, 0, loc),
			end:   time.Date(2026, 3, 13, 0, 5, 0, 0, loc),
		},
		{
			name:  "week containing 2026-11-01 fall-back",
			start: time.Date(2026, 10, 30, 0, 5, 0, 0, loc),
			end:   time.Date(2026, 11, 6, 0, 5, 0, 0, loc),
		},
		{
			name:  "week containing 2027-03-14 spring-forward",
			start: time.Date(2027, 3, 12, 0, 5, 0, 0, loc),
			end:   time.Date(2027, 3, 19, 0, 5, 0, 0, loc),
		},
		{
			name:  "week containing 2027-11-07 fall-back",
			start: time.Date(2027, 11, 5, 0, 5, 0, 0, loc),
			end:   time.Date(2027, 11, 12, 0, 5, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slots := sweepDistinctSlots(loc, settings.CadenceWeekly, tc.start, tc.end)
			if len(slots) != 1 {
				t.Fatalf("distinct weekly slots = %d, want 1: %v", len(slots), slots)
			}
			var only time.Time
			for _, v := range slots {
				only = v
			}
			if !only.Equal(tc.start) {
				t.Fatalf("the one distinct slot = %v, want %v", only, tc.start)
			}
		})
	}
}
