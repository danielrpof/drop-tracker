-- name: CreateWatchlistEntry :one
INSERT INTO watchlist (artist_id, release_types, muted_event_types)
VALUES ($1, $2, $3)
RETURNING *;
