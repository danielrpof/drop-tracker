package settings

import "time"

// ZoneName is the fixed IANA zone the digest schedule operates against
// (D-01). Fixed, not operator-configurable: REQUIREMENTS.md keeps a
// fire-time/timezone picker out of scope for this milestone, and this
// constant is where the fixed choice is documented.
const ZoneName = "America/New_York"

// GraceDaily and GraceWeekly bound how far past a missed slot a catch-up
// send is still eligible (D-12): daily is short so a brief outage still
// delivers promptly; weekly is long so a day-long outage doesn't produce a
// surprise mid-afternoon digest.
const (
	GraceDaily  = 12 * time.Hour
	GraceWeekly = 48 * time.Hour
)

// slotHour and slotMinute are the fixed local time-of-day every digest slot
// fires at (D-02): 00:05 sits outside the ~02:00 local transition window on
// both the spring-forward and fall-back DST transitions.
const (
	slotHour   = 0
	slotMinute = 5
)

// weeklySlotWeekday is the fixed weekday a weekly digest slot fires on
// (D-03). Operator-facing copy for the weekly cadence reads "Friday 00:05
// America/New_York, covering through Thursday night" (D-25) wherever the
// fire time is documented -- this is that documentation site.
const weeklySlotWeekday = time.Friday

// GraceFor returns the catch-up grace window for cadence (D-12).
func GraceFor(cadence Cadence) time.Duration {
	if cadence == CadenceWeekly {
		return GraceWeekly
	}
	return GraceDaily
}

// MostRecentSlot returns the most recent digest slot instant at or before
// now, for cadence, in loc (D-11). now is converted into loc first, so a
// UTC-valued (or any-zone-valued) now still resolves against the digest
// schedule's own zone.
//
// Every candidate is built via time.Date(year, month, day, slotHour,
// slotMinute, 0, 0, loc) on a calendar date reached by AddDate -- never by
// adding a fixed duration (+24h, +168h) to a prior instant. A DST day is 23
// or 25 hours long, so duration arithmetic would land at 23:05 or 01:05
// local instead of 00:05; rebuilding through time.Date after every AddDate
// step is what keeps every slot pinned to the fixed local time-of-day
// regardless of the DST transitions the calendar step crosses.
func MostRecentSlot(now time.Time, cadence Cadence, loc *time.Location) time.Time {
	local := now.In(loc)

	if cadence == CadenceWeekly {
		daysBack := (int(local.Weekday()) - int(weeklySlotWeekday) + 7) % 7
		anchor := local.AddDate(0, 0, -daysBack)
		candidate := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), slotHour, slotMinute, 0, 0, loc)
		if local.Before(candidate) {
			anchor = anchor.AddDate(0, 0, -7)
			candidate = time.Date(anchor.Year(), anchor.Month(), anchor.Day(), slotHour, slotMinute, 0, 0, loc)
		}
		return candidate
	}

	// Daily is the default arm: the digest_cadence column's CHECK constraint
	// (internal/db/migrations/000008_notification_settings.up.sql) guarantees
	// no third value ever reaches here.
	candidate := time.Date(local.Year(), local.Month(), local.Day(), slotHour, slotMinute, 0, 0, loc)
	if local.Before(candidate) {
		prev := local.AddDate(0, 0, -1)
		candidate = time.Date(prev.Year(), prev.Month(), prev.Day(), slotHour, slotMinute, 0, 0, loc)
	}
	return candidate
}
