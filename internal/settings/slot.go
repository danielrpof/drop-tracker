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

// GraceFor returns the catch-up grace window for cadence (D-12).
//
// TODO(22-01 GREEN): not yet implemented -- returns 0 so slot_test.go's
// TestGraceFor genuinely fails (RED) rather than compile-failing, which the
// project's pre-commit golangci-lint hook would block.
func GraceFor(cadence Cadence) time.Duration {
	return 0
}

// MostRecentSlot returns the most recent digest slot instant at or before
// now, for cadence, in loc (D-11).
//
// TODO(22-01 GREEN): not yet implemented -- returns the zero time.Time so
// slot_test.go's MostRecentSlot cases genuinely fail (RED) rather than
// compile-failing, which the project's pre-commit golangci-lint hook would
// block.
func MostRecentSlot(now time.Time, cadence Cadence, loc *time.Location) time.Time {
	return time.Time{}
}
