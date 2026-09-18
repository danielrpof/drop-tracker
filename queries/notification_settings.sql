-- name: GetNotificationSettings :one
SELECT * FROM notification_settings WHERE id = 1;

-- name: UpdateNotificationSettings :one
-- Full-object PUT semantics (no partial-update ambiguity to resolve), and the
-- fixed id = 1 predicate is what makes replaying the same body a no-op.
-- Phase 22 (D-14): digest_last_slot_at re-anchors to the caller-computed slot
-- only when digest mode is being turned on, or the cadence is changing while
-- it is already on -- never on a plain no-op replay and never on turning
-- digest mode off. The unqualified digest_enabled/digest_cadence on the
-- right of the WHEN condition are Postgres's pre-UPDATE row values (a SET
-- list's right-hand side evaluates against the row as it was before this
-- statement), which is exactly what the re-anchor decision needs.
UPDATE notification_settings
SET digest_enabled      = sqlc.arg('digest_enabled'),
    digest_cadence       = sqlc.arg('digest_cadence'),
    digest_last_slot_at  = CASE
        WHEN sqlc.arg('digest_enabled') AND (NOT digest_enabled OR digest_cadence <> sqlc.arg('digest_cadence'))
            THEN sqlc.narg('reanchor_slot')::timestamptz
        ELSE digest_last_slot_at
    END,
    updated_at           = now()
WHERE id = 1
RETURNING *;

-- name: AckEventsOnly :exec
-- Phase 23 (D-14): acks a chunk's event ids only -- runs for every delivered
-- chunk except the last. Mirrors AckDigestBatch's idempotent predicate but
-- touches no notification_settings column; only the final chunk moves
-- instance state (see AckDigestBatch below). docs/adr/0003.
UPDATE events SET notified_at = now()
WHERE id = ANY(sqlc.arg('ids')::bigint[]) AND notified_at IS NULL;

-- name: AckDigestBatch :exec
-- Phase 22 (D-16): the digest send's single-statement batch ack. Acks every
-- sent/suppressed event id (idempotent via the same AND notified_at IS NULL
-- predicate MarkNotified uses) and advances the singleton row's slot record
-- (always) and last-sent watermark (only when something was actually sent --
-- sent_at is nil on an empty/suppressed-only skip, so COALESCE leaves the
-- watermark untouched) in one atomic statement. No updated_at bump here on
-- purpose: updated_at tracks operator writes through PUT
-- /settings/notifications, and a scheduler-driven ack is not one. The
-- `acked` CTE is deliberately unreferenced by the outer UPDATE: Postgres
-- executes every data-modifying CTE in a statement exactly once regardless
-- of whether it is read from, so this still acks the events and updates the
-- settings row as one atomic statement with no Go transaction --
-- sqlc.Querier (internal/db/sqlc/db.go) exposes WithTx only on the concrete
-- *Queries, never on the interface every production caller is typed against.
WITH acked AS (
    UPDATE events SET notified_at = now()
    WHERE id = ANY(sqlc.arg('ids')::bigint[]) AND notified_at IS NULL
    RETURNING id
)
UPDATE notification_settings
SET digest_last_slot_at = sqlc.arg('slot')::timestamptz,
    digest_last_sent_at = COALESCE(sqlc.narg('sent_at')::timestamptz, digest_last_sent_at)
WHERE id = 1;
