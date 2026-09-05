-- Distinctive excerpt covering sqlc.arg/sqlc.narg and @param forms (D-15,
-- Task 2 step 10) -- these are parameter names, never asserted as columns
-- of any table, even though "page_size" and "set_release_types" are not
-- real column names on any table in this schema.

-- name: ListEvents :many
SELECT id, artist_id, release_date
FROM events
WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id')::bigint)
  AND created_at >= sqlc.arg('cutoff')::timestamptz
ORDER BY release_date DESC NULLS LAST, id DESC
LIMIT sqlc.arg('page_size');

-- name: UpdateWatchlistPreferences :one
UPDATE watchlist
SET release_types = CASE
        WHEN @set_release_types::boolean THEN @release_types::text[]
        ELSE watchlist.release_types
    END
WHERE watchlist.id = @id
RETURNING watchlist.id;
