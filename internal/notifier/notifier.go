// Package notifier drains the events outbox: rows with notified_at IS NULL are
// the queue (ListUnnotified dequeues), marking notified_at after a confirmed
// Discord send is the ack. It owns D-06's cross-cycle guard, D-07's inter-send
// spacing, and D-10's no-op selection.
package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
)

// defaultSpacing is the inter-send pause within one NotifyPending pass (D-07).
// Discord's limit is 5 req / 2s (~150/min); 400ms sustains ~150/min, at the ceiling.
const defaultSpacing = 400 * time.Millisecond

// defaultMaxReleaseAgeDays mirrors detection.DefaultNotifyMaxReleaseAgeDays,
// restated rather than imported to avoid a compile-time dep on internal/detection
// (same reasoning format.go records). main.go feeds both from one config value.
const defaultMaxReleaseAgeDays = 7

// dbOpTimeout bounds each individual DB call in NotifyPending -- without it a
// wedged socket parks the call forever while holding the notifying guard
// (.planning/debug/resolved/notify-pass-hangs-forever.md). Per-call, not
// per-pass; a var so tests can shrink it.
var dbOpTimeout = 10 * time.Second

// spacingWait is the seam NotifyPending's inter-send select waits on. A var, not
// time.After directly, so notifier_test.go can substitute an already-fired
// channel (same rationale as dbOpTimeout). Initialised to time.After.
var spacingWait = time.After

// Sender is the narrow outbound-delivery seam NotifyPending depends on, declared
// in the consumer so a test can substitute a fake with no HTTP client.
type Sender interface {
	Send(ctx context.Context, embed discord.Embed) error
}

var _ Sender = (*discord.Client)(nil)

// Sink is what poller.Notifier is declared against -- re-declared here as the
// type both Notifier and NoOp implement, so notifier.Select's return type does
// not force callers to import poller.Notifier.
type Sink interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
}

var _ Sink = (*Notifier)(nil)
var _ Sink = NoOp{}

// NoOp is D-10's inert Sink, returned by Select when DISCORD_WEBHOOK_URL is
// unset, so poller.go's Notifier seam is always non-nil.
type NoOp struct{}

// NotifyPending on NoOp issues no request and touches no row.
func (NoOp) NotifyPending(ctx context.Context, logger *slog.Logger) error { return nil }

// Notifier drains the events outbox: fetch pending rows, format each to a
// discord.Embed, send serially with spacing, mark notified on success. notifying
// is D-06's shared CAS-skip guard -- one guard for both poll cycles, since
// ListUnnotified is a global query they could otherwise race.
type Notifier struct {
	q          sqlc.Querier
	sender     Sender
	spacing    time.Duration
	maxAgeDays int
	notifying  atomic.Bool
}

// Option customises a Notifier at construction, mirroring detection.Option so
// both halves of the freshness gate are configured the same way.
type Option func(*Notifier)

// WithMaxReleaseAgeDays overrides defaultMaxReleaseAgeDays. Zero is meaningful
// ("only releases dated today"), not "unset"; negatives are rejected at config parse.
func WithMaxReleaseAgeDays(days int) Option {
	return func(n *Notifier) { n.maxAgeDays = days }
}

// New builds a Notifier backed by q for the outbox, sender for delivery, and
// spacing between consecutive sends, mirroring detection.New.
func New(q sqlc.Querier, sender Sender, spacing time.Duration, opts ...Option) *Notifier {
	n := &Notifier{q: q, sender: sender, spacing: spacing, maxAgeDays: defaultMaxReleaseAgeDays}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// Select returns the Sink main.go wires into poller.New: D-10's gate behind an
// exported function so it is unit-testable without booting the process. Empty
// webhookURL logs one Info line and returns NoOp{}; otherwise a real Notifier.
func Select(webhookURL string, q sqlc.Querier, httpClient *http.Client, logger *slog.Logger, opts ...Option) Sink {
	if webhookURL == "" {
		logger.Info("discord notifications disabled: DISCORD_WEBHOOK_URL not set")
		return NoOp{}
	}
	return New(q, discord.NewClient(webhookURL, httpClient), defaultSpacing, opts...)
}

// suppresses reports whether ev must be acked without sending, because its
// release date is outside the freshness window -- the delivery-side half of
// detection.notifyGate, the exact negation of detection.onOrAfterCutoff. A
// second gate drains the pre-fix backlog (inserted before any gate existed) and
// is defence in depth: .planning/debug/resolved/backlog-songs-trigger-discord.md.
// An absent or partial date SUPPRESSES -- conservative by design, opposite the
// usual "err toward an extra alert", because an undated row is absence of
// evidence, not evidence of freshness.
func (n *Notifier) suppresses(ev sqlc.Event) bool {
	cutoff := time.Now().UTC().AddDate(0, 0, -n.maxAgeDays).Format(time.DateOnly)
	return staleReleaseDate(ev.ReleaseDate, cutoff)
}

// staleReleaseDate is suppresses' clock-free core, split out (like
// detection.onOrAfterCutoff) so it is table-testable against a FIXED cutoff.
// releaseDate is *string per sqlc's nullable column; nil (SQL NULL) suppresses.
// Compared as strings, never parsed -- must stay onOrAfterCutoff's exact negation.
func staleReleaseDate(releaseDate *string, cutoff string) bool {
	if releaseDate == nil {
		return true
	}
	return len(*releaseDate) != len(time.DateOnly) || *releaseDate < cutoff
}

// listUnnotified calls q.ListUnnotified under a dbOpTimeout derived from ctx, so
// a wedged connection surfaces as an error instead of parking forever; shutdown
// cancellation still propagates.
func listUnnotified(ctx context.Context, q sqlc.Querier) ([]sqlc.Event, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	return q.ListUnnotified(opCtx)
}

// markNotified calls q.MarkNotified under the same bound. A timeout here lands on
// the WR-03 path: Discord already accepted the send, so the row stays pending
// and the next pass re-sends -- a visible duplicate, the preferred outcome.
func markNotified(ctx context.Context, q sqlc.Querier, id int64) (int64, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	return q.MarkNotified(opCtx, id)
}

// NotifyPending drains every currently-pending events row in ListUnnotified's
// order, sending each as one Discord message and marking it notified on success.
// The notifying CAS guard mirrors poller's mbRunning/dzRunning (CAS-skip, not a
// mutex, to avoid cross-source coupling -- D-06). A ListUnnotified or
// MarkNotified error is a hard failure and returns; a per-event Send error is
// logged and the loop continues so the next pass re-selects the row (D-09).
func (n *Notifier) NotifyPending(ctx context.Context, logger *slog.Logger) error {
	if !n.notifying.CompareAndSwap(false, true) {
		logger.Info("skipping notify pass: already in progress")
		return nil
	}
	defer n.notifying.Store(false)

	events, err := listUnnotified(ctx, n.q)
	if err != nil {
		return fmt.Errorf("notifier: list unnotified: %w", err)
	}

	suppressed := 0
	for i, ev := range events {
		// Ack without sending or a spacing wait -- no Discord request is made.
		// MarkNotified failing is a hard return, as on the send path below.
		if n.suppresses(ev) {
			if _, err := markNotified(ctx, n.q, ev.ID); err != nil {
				return fmt.Errorf("notifier: mark suppressed event: %w", err)
			}
			suppressed++
			continue
		}
		embed := formatEmbed(ev)
		if err := n.sender.Send(ctx, embed); err != nil {
			logger.Error("notify send failed",
				slog.Int64("event_id", ev.ID),
				slog.String("event_type", ev.EventType),
				slog.String("error", err.Error()),
			)
		} else if _, err := markNotified(ctx, n.q, ev.ID); err != nil {
			// WR-03: Discord durably accepted this send but the DB ack
			// failed, so the next pass re-sends -- a visible duplicate.
			// Warn (not a generic DB error) so this mode is identifiable
			// in production logs.
			logger.Warn("mark notified failed after a successful send: next pass will re-send this event",
				slog.Int64("event_id", ev.ID),
				slog.String("event_type", ev.EventType),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("notifier: mark notified: %w", err)
		}

		// Apply the spacing wait unconditionally, including after a failed
		// Send (WR-01): skipping it on the error path would fire the rest of
		// a backlog with no pacing during exactly a Discord outage or
		// rate-limit condition.
		if i < len(events)-1 {
			select {
			case <-spacingWait(n.spacing):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// One summary line per pass, not per row (the first post-gate pass
	// suppresses a multi-hundred-row backlog). Emitted only when something was
	// suppressed, so over-suppression -- the one real risk -- stays visible
	// without burying every other log line.
	if suppressed > 0 {
		logger.Info("notify pass suppressed stale events",
			slog.Int("suppressed_count", suppressed),
			slog.Int("pending_count", len(events)),
			slog.Int("max_release_age_days", n.maxAgeDays),
		)
	}

	return nil
}
