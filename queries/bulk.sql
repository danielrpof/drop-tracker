-- name: BulkDeleteWatchlist :many
-- One statement is all-or-nothing; an id it does not return was already gone (D-03).
DELETE FROM watchlist
WHERE id = ANY(sqlc.arg('ids')::bigint[])
RETURNING id;
