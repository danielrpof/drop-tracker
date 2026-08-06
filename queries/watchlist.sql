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
