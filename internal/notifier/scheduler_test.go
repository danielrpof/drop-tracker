package notifier_test

// scheduler_test.go covers DigestScheduler's lifecycle against a fake Sink
// and a manually-driven (or never-firing) tick source injected through
// WithTickSource -- no test here relies on a wall-clock sleep to decide
// pass/fail; synchronisation is on channels the fake sink signals or closes.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/notifier"
)

// syncBuffer guards bytes.Buffer with a mutex -- unlike every other test's
// newTestLogger buffer, this scheduler test keeps a loop goroutine logging
// (check/Stop) while the test goroutine reads, which a bare *bytes.Buffer
// cannot survive under -race.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newSyncTestLogger mirrors newTestLogger (JSON handler, LevelDebug) but
// writes into a syncBuffer, for tests that read log output while the
// scheduler's loop goroutine is still live.
func newSyncTestLogger() (*slog.Logger, *syncBuffer) {
	buf := &syncBuffer{}
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), buf
}

// recordedDigestCall is one (ctx, now) pair fakeSink.SendDigestIfDue was
// invoked with.
type recordedDigestCall struct {
	ctx context.Context
	now time.Time
}

// fakeSink is a controllable double for notifier.Sink, recording every
// SendDigestIfDue call and signalling a buffered notify channel per call so
// a test can synchronise on "N calls happened" without polling or sleeping.
type fakeSink struct {
	mu     sync.Mutex
	calls  []recordedDigestCall
	fn     func(ctx context.Context, logger *slog.Logger, now time.Time) error
	notify chan struct{}
}

func newFakeSink() *fakeSink {
	return &fakeSink{notify: make(chan struct{}, 256)}
}

// NotifyPending is unused by these tests but required to satisfy
// notifier.Sink.
func (f *fakeSink) NotifyPending(ctx context.Context, logger *slog.Logger) error { return nil }

func (f *fakeSink) SendDigestIfDue(ctx context.Context, logger *slog.Logger, now time.Time) error {
	f.mu.Lock()
	f.calls = append(f.calls, recordedDigestCall{ctx: ctx, now: now})
	f.mu.Unlock()
	select {
	case f.notify <- struct{}{}:
	default:
	}
	if f.fn != nil {
		return f.fn(ctx, logger, now)
	}
	return nil
}

func (f *fakeSink) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeSink) callAt(i int) recordedDigestCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[i]
}

var _ notifier.Sink = (*fakeSink)(nil)

// waitForCallCount blocks until sink has recorded at least n calls, or
// fails the test after timeout -- driven by fakeSink's notify channel, not
// a polling sleep loop.
func waitForCallCount(t *testing.T, sink *fakeSink, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if sink.callCount() >= n {
			return
		}
		select {
		case <-sink.notify:
		case <-deadline:
			t.Fatalf("timed out waiting for %d calls, got %d", n, sink.callCount())
		}
	}
}

// neverFiringTickSource returns a tick channel nobody ever sends to, for
// tests proving the immediate first check and nothing else.
func neverFiringTickSource() (<-chan time.Time, func()) {
	return make(chan time.Time), func() {}
}

func TestDigestScheduler_Start_RunsImmediateCheckBeforeFirstTick(t *testing.T) {
	sink := newFakeSink()
	logger, _ := newTestLogger()

	sched := notifier.NewDigestScheduler(sink, logger, time.Now, notifier.WithTickSource(neverFiringTickSource))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	waitForCallCount(t, sink, 1, 2*time.Second)

	if got := sink.callCount(); got != 1 {
		t.Fatalf("call count = %d, want 1 (tick source never fires)", got)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := sched.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v, want nil", err)
	}
}

func TestDigestScheduler_TickSource_NPlusOneCalls(t *testing.T) {
	sink := newFakeSink()
	logger, _ := newTestLogger()
	tickCh := make(chan time.Time)
	manual := func() (<-chan time.Time, func()) { return tickCh, func() {} }

	sched := notifier.NewDigestScheduler(sink, logger, time.Now, notifier.WithTickSource(manual))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	waitForCallCount(t, sink, 1, 2*time.Second) // the immediate check

	const n = 3
	for i := 0; i < n; i++ {
		tickCh <- time.Now()
		waitForCallCount(t, sink, 2+i, 2*time.Second)
	}

	if got := sink.callCount(); got != n+1 {
		t.Fatalf("call count = %d, want %d (1 immediate + %d ticks)", got, n+1, n)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := sched.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v, want nil", err)
	}
}

func TestDigestScheduler_ChecksReceiveInjectedClockValue(t *testing.T) {
	sink := newFakeSink()
	logger, _ := newTestLogger()
	want := time.Date(2026, 3, 1, 0, 5, 0, 0, time.UTC)

	sched := notifier.NewDigestScheduler(sink, logger, func() time.Time { return want }, notifier.WithTickSource(neverFiringTickSource))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	waitForCallCount(t, sink, 1, 2*time.Second)

	if got := sink.callAt(0).now; !got.Equal(want) {
		t.Fatalf("SendDigestIfDue now = %v, want %v (the injected clock's value, not time.Now())", got, want)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	_ = sched.Stop(stopCtx)
}

func TestDigestScheduler_Stop_ReturnsNilAfterInFlightCheckFinishes(t *testing.T) {
	sink := newFakeSink()
	release := make(chan struct{})
	sink.fn = func(ctx context.Context, logger *slog.Logger, now time.Time) error {
		<-release
		return nil
	}
	logger, _ := newTestLogger()

	sched := notifier.NewDigestScheduler(sink, logger, time.Now, notifier.WithTickSource(neverFiringTickSource))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	waitForCallCount(t, sink, 1, 2*time.Second) // the in-flight (blocked) check has started

	stopDone := make(chan error, 1)
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	go func() { stopDone <- sched.Stop(stopCtx) }()

	close(release) // unblock the in-flight check

	select {
	case err := <-stopDone:
		if err != nil {
			t.Fatalf("Stop: %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Stop to return after the in-flight check finished")
	}
}

func TestDigestScheduler_Stop_ExpiredDrainCtx_ReturnsErrAndCancelsRunCtx(t *testing.T) {
	sink := newFakeSink()
	started := make(chan struct{})
	ctxDone := make(chan struct{})
	sink.fn = func(ctx context.Context, logger *slog.Logger, now time.Time) error {
		close(started)
		<-ctx.Done()
		close(ctxDone)
		return ctx.Err()
	}
	logger, _ := newTestLogger()

	sched := notifier.NewDigestScheduler(sink, logger, time.Now, notifier.WithTickSource(neverFiringTickSource))
	sched.Start(context.Background())

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the in-flight check to start")
	}

	// Already-expired: constructed with a deadline in the past, so Stop's
	// select takes the ctx.Done() branch on its first pass.
	expiredCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	err := sched.Stop(expiredCtx)
	if !errors.Is(err, expiredCtx.Err()) {
		t.Fatalf("Stop error = %v, want %v", err, expiredCtx.Err())
	}

	select {
	case <-ctxDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the in-flight check's ctx to report Done after Stop cancelled the retained context")
	}
}

func TestDigestScheduler_CheckError_LoggedAndLoopContinues(t *testing.T) {
	sink := newFakeSink()
	sink.fn = func(ctx context.Context, logger *slog.Logger, now time.Time) error {
		return errors.New("boom")
	}
	logger, buf := newSyncTestLogger()
	tickCh := make(chan time.Time)
	manual := func() (<-chan time.Time, func()) { return tickCh, func() {} }

	sched := notifier.NewDigestScheduler(sink, logger, time.Now, notifier.WithTickSource(manual))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	waitForCallCount(t, sink, 1, 2*time.Second)

	tickCh <- time.Now()
	waitForCallCount(t, sink, 2, 2*time.Second)

	// Stop's completed drain is the only happens-before edge proving the loop
	// goroutine is done writing to buf: the sink notifies before fn() runs,
	// so waitForCallCount(2) can return mid-check.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := sched.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v, want nil (a non-nil error means the loop goroutine is still live and still writing)", err)
	}

	if !strings.Contains(buf.String(), "digest due-check failed") {
		t.Fatalf("expected due-check-failed Error log, got: %s", buf.String())
	}
	if got := sink.callCount(); got != 2 {
		t.Fatalf("call count = %d, want 2 (the loop kept going after the error)", got)
	}
}

func TestDigestScheduler_ContextCancelled_LoopExitsWithoutStop(t *testing.T) {
	sink := newFakeSink()
	logger, _ := newTestLogger()

	sched := notifier.NewDigestScheduler(sink, logger, time.Now, notifier.WithTickSource(neverFiringTickSource))
	ctx, cancel := context.WithCancel(context.Background())

	sched.Start(ctx)
	waitForCallCount(t, sink, 1, 2*time.Second)

	cancel() // cancel the ctx passed to Start -- not Stop

	// If the loop had not already exited on its own, Stop would have to wait
	// out its own drain path; a prompt nil here is evidence the goroutine
	// had already returned and closed done.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := sched.Stop(stopCtx); err != nil {
		t.Fatalf("Stop after Start's ctx was cancelled: %v, want nil (loop should have already exited)", err)
	}
}

func TestDigestScheduler_Stop_BeforeStart_ReturnsNil(t *testing.T) {
	sink := newFakeSink()
	logger, _ := newTestLogger()
	sched := notifier.NewDigestScheduler(sink, logger, time.Now)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := sched.Stop(stopCtx); err != nil {
		t.Fatalf("Stop before Start: %v, want nil", err)
	}
}
