// Package notifier drains the outbox Phase 4 already built: events rows
// with notified_at IS NULL are the queue (ListUnnotified is the dequeue),
// and marking notified_at after a confirmed Discord send is the ack. It
// owns D-06's shared cross-cycle guard, D-07's inter-send spacing, and
// D-10's disabled/no-op selection -- everything architectural in this
// phase's delivery path lives here, mirroring internal/detection's
// separation from internal/poller (this package's own analog is
// detection.Detector: a narrow-seam struct wrapping sqlc.Querier plus one
// collaborator interface).
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

// defaultSpacing is the inter-send pause between consecutive events in one
// NotifyPending pass (D-07): inside the decision's stated 350-500ms band.
// Discord's documented per-webhook rate limit is 5 requests per 2 seconds
// (~150/min, not the ~30/min this comment previously and incorrectly
// cited). 400ms yields ~2.5 sends/second sustained, i.e. ~150/min --
// right at that ceiling, which is why this constant sits inside the
// decision's band rather than at its lower (safer) end.
const defaultSpacing = 400 * time.Millisecond

// dbOpTimeout bounds each individual database call inside NotifyPending.
//
// The ctx NotifyPending receives is the poll cycle's own context, derived
// from signal.NotifyContext -- it carries no deadline and is not Done until
// the process is shutting down. pgx's only cancellation mechanism is a
// context watcher that sets a socket deadline once ctx becomes Done, so
// against a socket that is TCP-ESTABLISHED but never answers, an
// unbounded ctx means the query below blocks for the lifetime of the
// process. That is not a hypothetical: it is exactly what wedged this
// function in production, and because the notifying CAS guard is held for
// the whole call, one such block silently stopped every future notify pass
// (.planning/debug/resolved/notify-pass-hangs-forever.md).
//
// The bound is applied per database call rather than to the pass as a whole
// on purpose: a large backlog legitimately takes len(events)*spacing to
// drain, so a whole-pass deadline would abort healthy work. Wrapping the
// sqlc call itself is safe precisely because the generated ListUnnotified
// fully drains and closes its rows before returning -- the deadline can
// never be cancelled out from under an in-flight row iteration.
//
// Declared as a var, not a const, so notifier_test.go can shrink it and
// keep the regression test fast, mirroring discord.maxRetryAfter's own
// rationale.
var dbOpTimeout = 10 * time.Second

// spacingWait returns the channel NotifyPending's inter-send select waits
// on, given the configured spacing duration. Declared as a var, not called
// directly as time.After, so notifier_test.go can substitute a recording
// implementation that returns an already-fired channel -- mirroring
// dbOpTimeout's identical rationale a few lines above: production code calls
// through the seam unconditionally, and only the test binary ever
// overwrites it (via export_test.go's exported setter, so no production API
// surface is added). Initialised to time.After itself, so the production
// call site's behaviour is unchanged.
var spacingWait = time.After

// Sender is the narrow seam NotifyPending depends on for outbound delivery,
// declared here in the consumer (mirroring detection.RecordingSource) so a
// test can substitute a fake with no real HTTP client.
type Sender interface {
	Send(ctx context.Context, embed discord.Embed) error
}

var _ Sender = (*discord.Client)(nil)

// Sink is what poller.Notifier is declared against on the poller side --
// re-declared here as the type Notifier and NoOp both implement, so
// notifier.Select's return type does not force callers to import
// poller.Notifier for the interface itself.
type Sink interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
}

var _ Sink = (*Notifier)(nil)
var _ Sink = NoOp{}

// NoOp is D-10's inert Sink: returned by Select when DISCORD_WEBHOOK_URL is
// unset, so poller.go's Notifier seam is always non-nil and no cycle method
// ever nil-checks it -- exactly as EventRecorder has no disabled-state
// concept either.
type NoOp struct{}

// NotifyPending on NoOp issues no request and touches no row.
func (NoOp) NotifyPending(ctx context.Context, logger *slog.Logger) error { return nil }

// Notifier drains the events outbox: fetch pending rows, format each into a
// discord.Embed, send serially with spacing, and mark each row notified on
// success. notifying is D-06's shared CAS-skip guard -- one guard for both
// poll cycles, because ListUnnotified is a global query and an
// uncoordinated MusicBrainz/Deezer cycle could otherwise race the same
// pending rows.
type Notifier struct {
	q         sqlc.Querier
	sender    Sender
	spacing   time.Duration
	notifying atomic.Bool
}

// New builds a Notifier backed by q for the outbox, sender for delivery, and
// spacing between consecutive sends within one pass, mirroring
// detection.New's constructor shape.
func New(q sqlc.Querier, sender Sender, spacing time.Duration) *Notifier {
	return &Notifier{q: q, sender: sender, spacing: spacing}
}

// Select returns the Sink cmd/server/main.go should wire into poller.New:
// D-10's gate lives here, behind an exported function, rather than inline
// in main.go, so it is unit-testable without booting the process. An empty
// webhookURL logs one Info line stating Discord notifications are disabled
// and returns NoOp{}; otherwise it returns a real Notifier over
// discord.NewClient(webhookURL, httpClient) and defaultSpacing.
func Select(webhookURL string, q sqlc.Querier, httpClient *http.Client, logger *slog.Logger) Sink {
	if webhookURL == "" {
		logger.Info("discord notifications disabled: DISCORD_WEBHOOK_URL not set")
		return NoOp{}
	}
	return New(q, discord.NewClient(webhookURL, httpClient), defaultSpacing)
}

// listUnnotified calls q.ListUnnotified under a dbOpTimeout deadline derived
// from ctx, so a wedged connection surfaces as an ordinary error instead of
// parking this goroutine forever. Deriving from ctx (rather than from
// context.Background()) keeps shutdown cancellation propagating: whichever
// of the two fires first wins.
func listUnnotified(ctx context.Context, q sqlc.Querier) ([]sqlc.Event, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	return q.ListUnnotified(opCtx)
}

// markNotified calls q.MarkNotified under the same bound as listUnnotified.
// A timeout here lands on NotifyPending's existing WR-03 path: Discord has
// already accepted the send, so the row stays pending and the next pass
// re-sends it -- a visible duplicate, which is the documented and preferred
// outcome versus a process that never notifies again.
func markNotified(ctx context.Context, q sqlc.Querier, id int64) (int64, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	return q.MarkNotified(opCtx, id)
}

// NotifyPending drains every currently-pending events row (notified_at IS
// NULL), in ListUnnotified's deterministic order, sending each as one
// Discord message and marking it notified on success.
//
// The notifying.CompareAndSwap guard mirrors poller.Poller's
// mbRunning/dzRunning idiom verbatim, including releasing via defer so the
// flag also clears on an error return or a panic -- CAS-skip, not a
// blocking sync.Mutex, because a mutex here would reintroduce exactly the
// cross-source coupling poller.go's own guards reject: a slow,
// rate-limited send burst triggered by one cycle must never stall the
// other cycle's own call into this same shared step (D-06).
//
// A ListUnnotified error is a hard failure and is returned (mirrors
// detector.go treating insertEvent/groupBaseline DB errors as hard
// failures). A per-event Send error is logged and the loop continues --
// the row keeps notified_at NULL and the next pass re-selects it (D-09);
// one bad event must not cost the rest of the batch, mirroring the
// per-artist error-continue loop already in both poll cycles. A
// MarkNotified error, by contrast, is a hard failure that returns --
// unlike a send failure, there is no well-defined "keep going" outcome for
// a row Discord has already accepted but this process failed to
// acknowledge in the database.
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

	for i, ev := range events {
		embed := formatEmbed(ev)
		if err := n.sender.Send(ctx, embed); err != nil {
			logger.Error("notify send failed",
				slog.Int64("event_id", ev.ID),
				slog.String("event_type", ev.EventType),
				slog.String("error", err.Error()),
			)
		} else if _, err := markNotified(ctx, n.q, ev.ID); err != nil {
			// WR-03: Discord has already durably accepted this send, but
			// this process failed to acknowledge it in the DB -- the next
			// pass will re-select and re-send this row, producing a
			// visible duplicate notification. Log at Warn (distinct from
			// a generic ListUnnotified-level DB outage) before returning,
			// so this specific failure mode is identifiable in production
			// logs rather than indistinguishable from any other DB error.
			logger.Warn("mark notified failed after a successful send: next pass will re-send this event",
				slog.Int64("event_id", ev.ID),
				slog.String("event_type", ev.EventType),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("notifier: mark notified: %w", err)
		}

		// Apply the spacing wait unconditionally, including after a failed
		// Send (WR-01): D-07's whole purpose is to keep this project's
		// outbound rate under Discord's per-webhook ceiling, and skipping
		// the wait on the error path would fire the rest of a backlog
		// back-to-back with zero pacing during exactly the scenario --
		// a Discord outage or sustained rate-limit condition -- where
		// hammering the upstream is most likely to make things worse.
		if i < len(events)-1 {
			select {
			case <-spacingWait(n.spacing):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}
