package notifier

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// dueCheckInterval is the fixed gap between due-checks (D-08/D-09): there is
// no legitimate operator reason to want a different check cadence, so this
// is deliberately not a SchedulerOption and deliberately not an env var --
// 5 minutes catches a missed 00:05 fire within minutes at negligible
// background cost.
const dueCheckInterval = 5 * time.Minute

// tickSource returns a channel that fires once per tick and a stop func
// releasing the underlying ticker -- the seam WithTickSource substitutes for
// tests, mirroring Sender/SettingsReader's narrow-seam shape.
type tickSource func() (<-chan time.Time, func())

// defaultTickSource wraps time.NewTicker(dueCheckInterval), the production
// tick source every DigestScheduler uses unless a test overrides it via
// WithTickSource.
func defaultTickSource() (<-chan time.Time, func()) {
	ticker := time.NewTicker(dueCheckInterval)
	return ticker.C, ticker.Stop
}

// DigestScheduler drives Sink.SendDigestIfDue on a fixed interval, modeled
// on poller.Poller's Start/Stop lifecycle (D-18): a retained child context
// set in Start, and a Stop bounded by the caller's drain context. The one
// divergence from Poller is that there is no cron.Cron to .Stop() -- the
// drain signal here is this scheduler's own done channel, closed when the
// loop goroutine exits.
type DigestScheduler struct {
	sink   Sink
	logger *slog.Logger
	now    func() time.Time
	ticks  tickSource

	runCtx    context.Context
	runCancel context.CancelFunc
	done      chan struct{}

	// stopCh/stopOnce are Stop's own "exit the loop" signal, distinct from
	// runCtx: closing stopCh asks the loop goroutine to return after its
	// CURRENT check finishes (graceful), while runCancel forcibly cancels
	// the context an in-flight check is running under (used only once
	// Stop's own drain deadline expires). Sharing one signal for both would
	// mean a graceful Stop could only ever return nil by cancelling the
	// very check it is supposed to let finish.
	stopCh   chan struct{}
	stopOnce sync.Once
}

// SchedulerOption customises a DigestScheduler at construction.
type SchedulerOption func(*DigestScheduler)

// WithTickSource overrides the default 5-minute ticker with a caller-
// supplied tick source -- test injection only, so a fake-clock/manually-
// driven test can drive checks without a wall-clock wait. The interval
// itself stays fixed and unexported; this seam substitutes the SOURCE of
// ticks, never the interval (D-08).
func WithTickSource(ts func() (<-chan time.Time, func())) SchedulerOption {
	return func(s *DigestScheduler) { s.ticks = ts }
}

// NewDigestScheduler builds a DigestScheduler driving sink's
// SendDigestIfDue. now defaults to time.Now when nil.
func NewDigestScheduler(sink Sink, logger *slog.Logger, now func() time.Time, opts ...SchedulerOption) *DigestScheduler {
	if now == nil {
		now = time.Now
	}
	s := &DigestScheduler{sink: sink, logger: logger, now: now, ticks: defaultTickSource}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start launches the due-check loop. One check runs immediately (D-10 --
// deliberate, no analog in this codebase: the poller's own cron-based
// @every scheduling and authgate.Manager.sweepLoop both wait out the first
// interval, and this one must not, because it is what delivers a restart's
// catch-up), then every dueCheckInterval thereafter until ctx (or a later
// Stop) ends it.
func (s *DigestScheduler) Start(ctx context.Context) {
	s.runCtx, s.runCancel = context.WithCancel(ctx)
	s.logger.Info("digest scheduler starting", slog.Duration("interval", dueCheckInterval))
	s.done = make(chan struct{})
	s.stopCh = make(chan struct{})
	s.stopOnce = sync.Once{}

	go func() {
		defer close(s.done)
		ticks, stop := s.ticks()
		defer stop()

		s.check()
		for {
			select {
			case <-ticks:
				s.check()
			case <-s.runCtx.Done():
				return
			case <-s.stopCh:
				return
			}
		}
	}()
}

// check runs one SendDigestIfDue call. A non-nil error is logged and the
// loop continues -- a failing check never kills the loop; the per-tick
// outcome logging itself lives inside SendDigestIfDue (D-24), not here.
func (s *DigestScheduler) check() {
	if err := s.sink.SendDigestIfDue(s.runCtx, s.logger, s.now()); err != nil {
		s.logger.Error("digest due-check failed", slog.String("error", err.Error()))
	}
}

// Stop asks the loop to exit after its current check finishes (closing
// stopCh, never runCtx -- an in-flight check is allowed to complete
// normally), then waits for that exit, bounded by ctx. This mirrors
// Poller.Stop's shape (stop scheduling new work, then wait for the
// in-flight work to drain); the one divergence is that there is no
// cron.Cron to .Stop(), so the drain signal is this scheduler's own done
// channel rather than a context handed back by cron. If ctx expires first,
// runCancel is invoked to force the still-in-flight check's context to
// report Done rather than blocking forever. Guarded against being called
// before Start (nil done): returns nil rather than blocking forever.
func (s *DigestScheduler) Stop(ctx context.Context) error {
	s.logger.Info("digest scheduler stopping")
	if s.done == nil {
		return nil
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		if s.runCancel != nil {
			s.runCancel()
		}
		return ctx.Err()
	}
}
