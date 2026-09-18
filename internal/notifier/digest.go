package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
	"github.com/danielrpof/drop-tracker/internal/settings"
)

// SendDigestIfDue implements D-17's fixed digest-send sequence: CAS the
// shared notifying lock (the same lock NotifyPending takes -- ADR-0002, no
// second lock exists in this package), read settings fail-closed, decide
// whether the current digest slot is due, partition the outbox into
// sendable and suppressed rows, re-check settings once before the first
// chunk (D-30), split into one or more window-headered chunks
// (buildDigestChunks), and send/ack each chunk in order: every chunk but
// the last acks only the ids it rendered (ackEventsOnly, D-14), the final
// chunk's ack also carries the suppressed ids and both settings columns
// (ackDigestBatch, D-15) -- the only write in a run that moves instance
// state. A send failure or a cancelled context at a chunk boundary stops
// the loop with neither settings column advanced, so the digest stays due
// and the next check retries from whatever is still un-acked (D-11/D-12).
// A single-chunk digest takes exactly this same path with one iteration,
// which is Phase 22's regression-safety property (D-14).
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

	// D-30: the settings re-check runs once, before the first chunk only --
	// a deliberate divergence from Phase 21's per-send re-read rule. A
	// multi-message digest is one logical delivery; aborting it halfway
	// would produce the hybrid half-digest half-real-time output Phase 21's
	// mutual-exclusion invariant exists to prevent, and the lock is held
	// throughout (D-25). Re-reading here also refreshes cfg.DigestLastSentAt,
	// so chunks are built from this post-recheck cfg (not the pre-recheck
	// one above) -- the header's watermark and this abort-check read the
	// settings row exactly once, together.
	cfg, err = readSettings(ctx, n.settingsReader)
	if err != nil {
		logSettingsReadFailure(ctx, logger, err)
		return nil
	}
	if !cfg.DigestEnabled {
		return nil
	}

	chunks := buildDigestChunks(sendable, cfg.DigestLastSentAt)

	for i, chunk := range chunks {
		if i > 0 {
			// D-23/D-26: digest-specific pacing, deliberately not the
			// real-time path's 400ms inter-send seam. ctx.Done() is observed
			// at this chunk boundary, not mid-POST, so a forced drain stops
			// cleanly between messages.
			select {
			case <-digestChunkWait(digestChunkSpacing):
			case <-ctx.Done():
				logger.Warn("digest chunk loop stopped at a chunk boundary: remainder stays pending, slot un-advanced",
					slog.Int("chunk_index", i),
					slog.Int("chunk_count", len(chunks)),
				)
				return nil
			}
		}

		embed := discord.Embed{Description: chunk.description}
		if err := n.sender.Send(ctx, embed); err != nil {
			// A send failure is fatal to this digest attempt, not the
			// process: neither settings column advances (D-11), so the
			// slot stays due and the next check retries from whatever is
			// still un-acked (D-12) -- chunks 1..i-1 already acked below,
			// this chunk and every chunk after it stay pending.
			logger.Error("digest send failed",
				slog.Int("chunk_index", i),
				slog.Int("chunk_count", len(chunks)),
				slog.Int("sendable_count", len(sendable)),
				slog.String("error", err.Error()),
			)
			return nil
		}

		if i < len(chunks)-1 {
			// D-14: every chunk but the last acks its own ids only, and
			// touches no settings column -- each chunk now acks precisely
			// what it rendered, replacing the pre-phase invariant that
			// walked the full sendable slice regardless of what rendered.
			if err := ackEventsOnly(ctx, n.q, chunk.ids); err != nil {
				return fmt.Errorf("notifier: ack digest chunk: %w", err)
			}
			continue
		}

		// Final chunk: the only write in this phase that moves instance
		// state (D-14). Suppressed ids ride this ack, never an earlier
		// chunk's -- they are never "delivered", so they belong with the
		// write that means "this digest completed" (D-15).
		sentIDs := make([]int64, 0, len(chunk.ids)+len(suppressedIDs))
		sentIDs = append(sentIDs, chunk.ids...)
		sentIDs = append(sentIDs, suppressedIDs...)

		sentAt := now
		if err := ackDigestBatch(ctx, n.q, sentIDs, slot, &sentAt); err != nil {
			return fmt.Errorf("notifier: ack sent digest batch: %w", err)
		}
	}

	logger.Info("digest sent",
		slog.Int("sent_count", len(sendable)),
		slog.Int("suppressed_count", len(suppressedIDs)),
		slog.Int("chunk_count", len(chunks)),
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

// ackEventsOnly is D-14's narrow per-chunk ack (docs/adr/0003): marks ids
// notified and touches no notification_settings column. Runs for every
// delivered chunk except the last -- the final chunk's ack is
// ackDigestBatch, the only write in this phase that moves instance state.
// Same context-detach shape as ackDigestBatch, so a shutdown landing after
// Discord's 2xx still acks this chunk instead of re-sending its content in
// a later digest.
func ackEventsOnly(ctx context.Context, q sqlc.Querier, ids []int64) error {
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dbOpTimeout)
	defer cancel()
	return q.AckEventsOnly(opCtx, ids)
}
