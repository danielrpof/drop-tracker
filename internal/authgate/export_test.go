package authgate

// export_test.go is compiled only into the test binary, so these setters
// reach the external authgate_test package without adding production API
// surface -- mirroring internal/notifier/export_test.go's save-swap-restore
// shape for its shrinkable timing vars.

import (
	"time"

	"golang.org/x/time/rate"
)

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

// --- plan 14-02 brute-force-defense + audit tunables ---
//
// Each setter swaps its var(s) and returns a restore func, matching the
// save-swap-restore shape above. Shrinking these keeps the throttle, delay,
// semaphore and global-counter tests fast and deterministic.

// SetLoginRateForTest swaps loginRate (the per-IP token-bucket refill rate).
func SetLoginRateForTest(r rate.Limit) (restore func()) {
	orig := loginRate
	loginRate = r
	return func() { loginRate = orig }
}

// SetLoginBurstForTest swaps loginBurst (attempts allowed before throttling).
func SetLoginBurstForTest(n int) (restore func()) {
	orig := loginBurst
	loginBurst = n
	return func() { loginBurst = orig }
}

// SetLoginDelayForTest swaps loginDelayMin and loginDelayJitter together.
func SetLoginDelayForTest(minDelay, maxJitter time.Duration) (restore func()) {
	om, oj := loginDelayMin, loginDelayJitter
	loginDelayMin, loginDelayJitter = minDelay, maxJitter
	return func() { loginDelayMin, loginDelayJitter = om, oj }
}

// SetLoginSleepForTest swaps the loginSleep seam so a test can block a login
// in-flight (the maxConcurrentLogins shed test) or count delay invocations.
func SetLoginSleepForTest(fn func(time.Duration)) (restore func()) {
	orig := loginSleep
	loginSleep = fn
	return func() { loginSleep = orig }
}

// SetLimiterSweepIntervalForTest swaps limiterSweepInterval (the sweeper tick).
func SetLimiterSweepIntervalForTest(d time.Duration) (restore func()) {
	orig := limiterSweepInterval
	limiterSweepInterval = d
	return func() { limiterSweepInterval = orig }
}

// SetLimiterIdleTTLForTest swaps limiterIdleTTL (idle eviction threshold).
func SetLimiterIdleTTLForTest(d time.Duration) (restore func()) {
	orig := limiterIdleTTL
	limiterIdleTTL = d
	return func() { limiterIdleTTL = orig }
}

// SetMaxConcurrentLoginsForTest swaps maxConcurrentLogins. A Manager reads it
// in NewManager, so call this BEFORE constructing the Manager under test.
func SetMaxConcurrentLoginsForTest(n int) (restore func()) {
	orig := maxConcurrentLogins
	maxConcurrentLogins = n
	return func() { maxConcurrentLogins = orig }
}

// SetGlobalCounterForTest swaps globalWindow, globalThreshold and alertCooldown
// together -- the three vars behind the D-12 brute-force alert.
func SetGlobalCounterForTest(window time.Duration, threshold int, cooldown time.Duration) (restore func()) {
	ow, ot, oc := globalWindow, globalThreshold, alertCooldown
	globalWindow, globalThreshold, alertCooldown = window, threshold, cooldown
	return func() { globalWindow, globalThreshold, alertCooldown = ow, ot, oc }
}
