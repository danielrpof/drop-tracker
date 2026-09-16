package notifier

import (
	"context"
	"log/slog"
	"time"
)

// tickSource returns a channel that fires once per tick and a stop func
// releasing the underlying ticker.
type tickSource func() (<-chan time.Time, func())

// DigestScheduler is a RED-phase compiling stub: it accepts the shape the
// tests exercise but does not yet run the due-check loop. GREEN replaces
// this file with the real Start/Stop/check implementation (D-18).
type DigestScheduler struct {
	sink   Sink
	logger *slog.Logger
	now    func() time.Time
	ticks  tickSource
}

// SchedulerOption customises a DigestScheduler at construction.
type SchedulerOption func(*DigestScheduler)

// WithTickSource is the RED-phase stub for the tick-source seam.
func WithTickSource(ts func() (<-chan time.Time, func())) SchedulerOption {
	return func(s *DigestScheduler) { s.ticks = ts }
}

// NewDigestScheduler is the RED-phase stub constructor.
func NewDigestScheduler(sink Sink, logger *slog.Logger, now func() time.Time, opts ...SchedulerOption) *DigestScheduler {
	if now == nil {
		now = time.Now
	}
	s := &DigestScheduler{sink: sink, logger: logger, now: now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start is a RED-phase stub: it does nothing, so no check ever runs.
func (s *DigestScheduler) Start(ctx context.Context) {}

// Stop is a RED-phase stub: it always returns nil immediately.
func (s *DigestScheduler) Stop(ctx context.Context) error { return nil }
