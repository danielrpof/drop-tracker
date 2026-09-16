package notifier

// export_test.go is compiled only into the test binary (Go's own convention
// for files named _test.go), so the exported helper below reaches the
// external package notifier_test tests without adding any production API
// surface -- mirroring timeout_test.go's save-swap-restore shape for
// dbOpTimeout, applied to the spacingWait seam instead.

import (
	"testing"
	"time"
)

// SetSpacingWaitForTest swaps NotifyPending's inter-send spacingWait seam
// for fn for the duration of the calling test, restoring the original via
// t.Cleanup. A test typically installs a recording fn that appends each
// requested duration to a slice and returns an already-fired channel, so
// the select in NotifyPending's send loop fires immediately instead of
// actually waiting -- turning an elapsed-wall-clock assertion into a
// deterministic requested-duration assertion.
func SetSpacingWaitForTest(t *testing.T, fn func(time.Duration) <-chan time.Time) {
	t.Helper()
	orig := spacingWait
	spacingWait = fn
	t.Cleanup(func() { spacingWait = orig })
}
