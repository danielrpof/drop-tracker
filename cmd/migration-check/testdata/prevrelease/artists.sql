-- Distinctive excerpt of the previous release's queries/artists.sql (D-15,
-- Task 2): ON CONFLICT DO UPDATE SET with EXCLUDED.col and a
-- table-qualified column (COALESCE(EXCLUDED.x, artists.x)), and a
-- multi-table SELECT with an alias-qualified star and a qualified WHERE
-- column.

-- name: UpsertArtist :one
INSERT INTO artists (mbid, deezer_id, name, disambiguation, image_url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (mbid) DO UPDATE
    SET name = EXCLUDED.name,
        deezer_id = COALESCE(EXCLUDED.deezer_id, artists.deezer_id),
        updated_at = now()
RETURNING *;

-- name: ListArtistsMissingImage :many
SELECT a.*
FROM artists a
JOIN watchlist w ON w.artist_id = a.id
WHERE a.image_url IS NULL
  AND (a.art_match_attempted_at IS NULL OR a.art_match_attempted_at < now() - INTERVAL '24 hours')
ORDER BY a.id ASC;
