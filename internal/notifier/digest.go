package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/settings"
)

// SendDigestIfDue implements D-17's fixed digest-send sequence: CAS the
// shared notifying lock (the same lock NotifyPending takes -- ADR-0002, no
// second lock exists in this package), read settings fail-closed, decide
// whether the current digest slot is due, partition the outbox into
// sendable and suppressed rows, build one embed, re-check settings
// immediately before the POST, send, and ack the sent+suppressed rows plus
// both settings columns in one atomic statement (D-16).
func (n *Notifier) SendDigestIfDue(ctx context.Context, logger *slog.Logger, now time.Time) error {
	if !n.notifying.CompareAndSwap(false, true) {
		logger.Info("skipping digest send: already in progress")
		return nil
	}
	defer n.notifying.Store(false)

	// D-23: no substitute location is chosen here or anywhere else on this
	// path -- an unconfigured zone refuses to send rather than schedule
	// against a silently-wrong clock.
	if n.loc == nil {
		logger.Warn("skipping digest send: digest zone not configured")
		return nil
	}

	cfg, err := readSettings(ctx, n.settingsReader)
	if err != nil {
		logSettingsReadFailure(ctx, logger, err)
		return nil
	}
	if !cfg.DigestEnabled {
		return nil
	}

	slot := settings.MostRecentSlot(now, cfg.DigestCadence, n.loc)
	if cfg.DigestLastSlotAt != nil && !cfg.DigestLastSlotAt.Before(slot) {
		logger.Debug("digest not due",
			slog.Time("slot", slot),
			slog.Time("recorded_slot", *cfg.DigestLastSlotAt),
		)
		return nil
	}
	age := now.Sub(slot)
	if grace := settings.GraceFor(cfg.DigestCadence); age > grace {
		// Grace expired: write nothing. The slot simply stops being due, and
		// the pending events carry forward to the next slot -- nothing is
		// lost, because the outbox persists (D-12).
		logger.Warn("digest slot past its grace window: not sending late",
			slog.Time("slot", slot),
			slog.Duration("age", age),
			slog.Duration("grace", grace),
		)
		return nil
	}
	logger.Info("digest due",
		slog.Time("slot", slot),
		slog.String("cadence", string(cfg.DigestCadence)),
	)

	events, err := listUnnotified(ctx, n.q)
	if err != nil {
		return fmt.Errorf("notifier: digest list unnotified: %w", err)
	}

	sendable := make([]sqlc.Event, 0, len(events))
	suppressedIDs := make([]int64, 0, len(events))
	for _, ev := range events {
		if n.suppresses(ev) {
			suppressedIDs = append(suppressedIDs, ev.ID)
			continue
		}
		sendable = append(sendable, ev)
	}

	if len(sendable) == 0 {
		// Nothing sendable: no embed is built and no Discord request is made
		// -- the empty case short-circuits before message assembly. The
		// suppressed ids still ack and the slot still advances; the
		// watermark does not (D-13/T-22-10).
		if err := ackDigestBatch(ctx, n.q, suppressedIDs, slot, nil); err != nil {
			return fmt.Errorf("notifier: ack empty digest slot: %w", err)
		}
		logger.Info("digest slot had nothing to send",
			slog.Int("suppressed_count", len(suppressedIDs)),
		)
		return nil
	}

	embed := buildDigestEmbed(sendable)

	// D-17 step 5: re-read settings immediately before the POST -- a toggle
	// landing between the outbox read and here must abort with no send and
	// no write, exactly as NotifyPending's own mid-pass re-check (D-04/T-22-12).
	cfg, err = readSettings(ctx, n.settingsReader)
	if err != nil {
		logSettingsReadFailure(ctx, logger, err)
		return nil
	}
	if !cfg.DigestEnabled {
		return nil
	}

	if err := n.sender.Send(ctx, embed); err != nil {
		logger.Error("digest send failed",
			slog.Int("sendable_count", len(sendable)),
			slog.String("error", err.Error()),
		)
		return nil
	}

	// sentIDs walks the full sendable slice regardless of what
	// digest_format.go actually rendered into embed.Description -- a
	// buildDigestEmbed truncation never leaves an event stuck pending, since
	// every id here still acks once Send succeeds (closes T-22-15).
	sentIDs := make([]int64, 0, len(sendable)+len(suppressedIDs))
	for _, ev := range sendable {
		sentIDs = append(sentIDs, ev.ID)
	}
	sentIDs = append(sentIDs, suppressedIDs...)

	sentAt := now
	if err := ackDigestBatch(ctx, n.q, sentIDs, slot, &sentAt); err != nil {
		return fmt.Errorf("notifier: ack sent digest batch: %w", err)
	}

	logger.Info("digest sent",
		slog.Int("sent_count", len(sendable)),
		slog.Int("suppressed_count", len(suppressedIDs)),
		slog.Time("slot", slot),
	)

	return nil
}

// ackDigestBatch is D-16's single-statement batch ack, shaped like
// listUnnotified/markNotified but wrapping the ack's context to detach it
// from ctx's own cancellation first -- a shutdown landing after Discord's
// 2xx still acks instead of re-sending the whole batch on the next boot.
// sentAt nil (the empty/suppressed-only skip) leaves digest_last_sent_at
// untouched via the generated query's COALESCE.
func ackDigestBatch(ctx context.Context, q sqlc.Querier, ids []int64, slot time.Time, sentAt *time.Time) error {
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dbOpTimeout)
	defer cancel()

	params := sqlc.AckDigestBatchParams{
		Slot: pgtype.Timestamptz{Time: slot, Valid: true},
		Ids:  ids,
	}
	if sentAt != nil {
		params.SentAt = pgtype.Timestamptz{Time: *sentAt, Valid: true}
	}
	return q.AckDigestBatch(opCtx, params)
}
