package authgate

// export_test.go is compiled only into the test binary, so these setters
// reach the external authgate_test package without adding production API
// surface -- mirroring internal/notifier/export_test.go's save-swap-restore
// shape for its shrinkable timing vars.

import "time"

// SetSessionWindowForTest swaps sessionWindow for d for the duration of the
// calling test and returns a restore func. Shrinking the window lets a test
// exercise the renewal and expiry boundaries in milliseconds instead of days.
func SetSessionWindowForTest(d time.Duration) (restore func()) {
	orig := sessionWindow
	sessionWindow = d
	return func() { sessionWindow = orig }
}

// SetRenewAfterForTest swaps renewAfter for d and returns a restore func.
func SetRenewAfterForTest(d time.Duration) (restore func()) {
	orig := renewAfter
	renewAfter = d
	return func() { renewAfter = orig }
}

// SetAbsoluteCapForTest swaps absoluteCap for d and returns a restore func.
func SetAbsoluteCapForTest(d time.Duration) (restore func()) {
	orig := absoluteCap
	absoluteCap = d
	return func() { absoluteCap = orig }
}

// SessionWindow exposes the current window to tests that assert on the
// minted cookie's Max-Age.
func SessionWindow() time.Duration { return sessionWindow }
