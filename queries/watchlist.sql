-- name: CreateWatchlistEntry :one
INSERT INTO watchlist (artist_id, release_types, muted_event_types)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListWatchlist :many
-- Both watchlist and artists have a column named id -- every selected
-- column is explicitly aliased so sqlc emits a struct carrying both ID
-- (the watchlist entry) and ArtistID (the master artist) rather than
-- silently collapsing them (02-RESEARCH.md Pitfall 4). Name is not unique,
-- so the artist id is a required, not cosmetic, ORDER BY tiebreak: without
-- it, two equally-named artists would come back in whatever order the
-- planner happens to choose, which is non-deterministic across runs.
SELECT w.id AS id, a.id AS artist_id, a.mbid, a.name, a.deezer_id,
       a.disambiguation, a.image_url,
       w.release_types, w.muted_event_types, w.created_at, w.updated_at
FROM watchlist w
JOIN artists a ON a.id = w.artist_id
ORDER BY a.name ASC, a.id ASC;

-- name: UpdateWatchlistPreferences :one
-- Both arrays are always written; the partial-update semantics (leave one
-- axis untouched) live in Go, which reads the current row first and
-- substitutes the untouched axis before calling this query. Keeping the SQL
-- total rather than conditional avoids a COALESCE-per-column expression
-- whose NULL-versus-empty-array behaviour is exactly the distinction this
-- plan has to keep sharp.
UPDATE watchlist
SET release_types = $2, muted_event_types = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteWatchlistEntry :execrows
-- :execrows returns the affected row count in one round trip, which is what
-- lets the service distinguish "deleted" from "there was nothing to delete"
-- without a preceding existence SELECT. A check-then-delete pair would open
-- a window where a concurrent delete lands between the two statements,
-- turning what should be one 204 and one 404 into two 204s; Postgres's
-- row-level lock on this single statement is what makes the split
-- deterministic under concurrency (T-02-15).
DELETE FROM watchlist WHERE id = $1;
