-- name: UpsertArtist :one
-- The SET list below is deliberately exhaustive over every mutable metadata
-- column on artists (WR-01, G-02-2a): a column left out of it is a column
-- this table silently refuses to ever update again on a re-add, and because
-- Service.Add builds the response Entry from this query's returned row, the
-- caller is handed back the stale value with a 201 as though the write
-- succeeded. name is NOT NULL and required on every add (D-04), so it is
-- always refreshed unconditionally. deezer_id, disambiguation and image_url
-- are all nullable, caller-optional metadata, so each uses
-- COALESCE(EXCLUDED.<col>, artists.<col>): a nil/omitted field means "the
-- caller said nothing about this field", never "blank it".
INSERT INTO artists (mbid, deezer_id, name, disambiguation, image_url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (mbid) DO UPDATE
    SET name = EXCLUDED.name,
        deezer_id = COALESCE(EXCLUDED.deezer_id, artists.deezer_id),
        disambiguation = COALESCE(EXCLUDED.disambiguation, artists.disambiguation),
        image_url = COALESCE(EXCLUDED.image_url, artists.image_url),
        updated_at = now()
RETURNING *;
