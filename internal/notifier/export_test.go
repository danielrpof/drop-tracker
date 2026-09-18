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

// SetDigestChunkWaitForTest swaps SendDigestIfDue's inter-chunk
// digestChunkWait seam for fn for the duration of the calling test,
// restoring the original via t.Cleanup -- same shape as
// SetSpacingWaitForTest, applied to the digest-specific chunk-pacing seam
// (D-23) instead of the real-time spacingWait one.
func SetDigestChunkWaitForTest(t *testing.T, fn func(time.Duration) <-chan time.Time) {
	t.Helper()
	orig := digestChunkWait
	digestChunkWait = fn
	t.Cleanup(func() { digestChunkWait = orig })
}

// SetDigestNowForTest swaps SendDigestIfDue's digestSendBudget clock seam
// (digestNow) for fn for the duration of the calling test, restoring the
// original via t.Cleanup -- same shape as SetSpacingWaitForTest/
// SetDigestChunkWaitForTest, applied to the wall-clock read the budget
// check (D-25) compares against its deadline.
func SetDigestNowForTest(t *testing.T, fn func() time.Time) {
	t.Helper()
	orig := digestNow
	digestNow = fn
	t.Cleanup(func() { digestNow = orig })
}
