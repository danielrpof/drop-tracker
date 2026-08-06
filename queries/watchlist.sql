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
-- The partial-update merge happens inside this statement, not in Go: each
-- axis is resolved by a CASE whose ELSE names the column itself, so the
-- value carried forward for an untouched axis is read from the row version
-- this UPDATE itself locked. Under READ COMMITTED a writer blocked behind
-- another writer's uncommitted change re-evaluates its WHERE clause and SET
-- expressions against the newly committed row once it unblocks -- so the
-- second writer physically cannot persist a value it observed before the
-- first writer committed (G-02-2b, T-02-19). The zero-row case (no matching
-- id, including one deleted concurrently) surfaces as pgx.ErrNoRows, which
-- Service.UpdatePreferences translates to ErrNotFound in one round trip
-- (T-02-20) -- there is no separate unlocked read whose result can go stale.
--
-- Each axis's "am I being set" signal is carried by its own explicit boolean
-- parameter rather than multiplexed onto the array parameter's nullability:
-- an empty array and an absent key are two different client intents (D-11),
-- and a NULL-means-untouched COALESCE would collapse that distinction.
WITH updated AS (
    UPDATE watchlist
    SET release_types = CASE
            WHEN @set_release_types::boolean THEN @release_types::text[]
            ELSE watchlist.release_types
        END,
        muted_event_types = CASE
            WHEN @set_muted_event_types::boolean THEN @muted_event_types::text[]
            ELSE watchlist.muted_event_types
        END,
        updated_at = now()
    WHERE watchlist.id = @id
    RETURNING watchlist.id, watchlist.artist_id, watchlist.release_types, watchlist.muted_event_types, watchlist.created_at, watchlist.updated_at
)
SELECT u.id AS id, a.id AS artist_id, a.mbid, a.name, a.deezer_id,
       a.disambiguation, a.image_url,
       u.release_types, u.muted_event_types, u.created_at, u.updated_at
FROM updated u
JOIN artists a ON a.id = u.artist_id;

-- name: DeleteWatchlistEntry :execrows
-- :execrows returns the affected row count in one round trip, which is what
-- lets the service distinguish "deleted" from "there was nothing to delete"
-- without a preceding existence SELECT. A check-then-delete pair would open
-- a window where a concurrent delete lands between the two statements,
-- turning what should be one 204 and one 404 into two 204s; Postgres's
-- row-level lock on this single statement is what makes the split
-- deterministic under concurrency (T-02-15).
DELETE FROM watchlist WHERE id = $1;
