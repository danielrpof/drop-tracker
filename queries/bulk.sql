-- name: BulkDeleteWatchlist :many
-- One statement is all-or-nothing; an id it does not return was already gone (D-03).
DELETE FROM watchlist
WHERE id = ANY(sqlc.arg('ids')::bigint[])
RETURNING id;

-- name: LockWatchlistArtistsForBulkTag :many
-- Locks the cap trigger's artist rows in id order before any insert (ADR 0004 bulk amendment).
SELECT w.id AS entry_id, a.id AS artist_id, a.name
FROM watchlist w JOIN artists a ON a.id = w.artist_id
WHERE w.id = ANY(@entry_ids::bigint[])
ORDER BY a.id
FOR NO KEY UPDATE OF a;

-- name: BulkAttachTag :many
-- Inserts only where the artist has room; the literal 10 mirrors MaxTagsPerArtist.
WITH targets AS (
  SELECT unnest(@artist_ids::bigint[]) AS artist_id
), had AS (
  SELECT l.artist_id FROM artist_tags l
  WHERE l.tag_id = @tag_id::bigint AND l.artist_id = ANY(@artist_ids::bigint[])
), ins AS (
  INSERT INTO artist_tags (artist_id, tag_id)
  SELECT t.artist_id, @tag_id::bigint FROM targets t
  WHERE t.artist_id NOT IN (SELECT artist_id FROM had)
    AND (SELECT count(*) FROM artist_tags l WHERE l.artist_id = t.artist_id) < 10
  ON CONFLICT (artist_id, tag_id) DO NOTHING
  RETURNING artist_id
)
SELECT t.artist_id::bigint AS artist_id,
       (t.artist_id IN (SELECT artist_id FROM had))::boolean AS had_tag,
       (t.artist_id IN (SELECT artist_id FROM ins))::boolean AS inserted
FROM targets t ORDER BY t.artist_id;
