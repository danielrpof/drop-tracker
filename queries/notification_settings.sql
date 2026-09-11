-- name: GetNotificationSettings :one
SELECT * FROM notification_settings WHERE id = 1;

-- name: UpdateNotificationSettings :one
-- A plain positional UPDATE, not watchlist's CASE-based merge: the route is
-- a full-object PUT (no partial-update ambiguity to resolve), and the fixed
-- id = 1 predicate is what makes replaying the same body a no-op.
UPDATE notification_settings
SET digest_enabled = $1,
    digest_cadence  = $2,
    updated_at      = now()
WHERE id = 1
RETURNING *;
