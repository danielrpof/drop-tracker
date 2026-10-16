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

-- name: BulkUpdateWatchlistPreferences :many
-- Per-type delta: locks targets in id order, rebuilds each axis as (current + set) - unset in
-- canonical order, and updates only changed rows (D-05). COALESCE keeps a nil array from wiping.
WITH locked AS MATERIALIZED (
  SELECT id, release_types, muted_event_types FROM watchlist
  WHERE id = ANY(@ids::bigint[]) ORDER BY id FOR UPDATE
), computed AS (
  SELECT l.id,
    ARRAY(SELECT u.t FROM unnest(@all_release_types::text[]) WITH ORDINALITY AS u(t, ord)
          WHERE (u.t = ANY(l.release_types) OR u.t = ANY(COALESCE(@set_release_types::text[], '{}')))
            AND NOT (u.t = ANY(COALESCE(@unset_release_types::text[], '{}')))
          ORDER BY u.ord)::text[] AS release_types,
    ARRAY(SELECT u.t FROM unnest(@all_event_types::text[]) WITH ORDINALITY AS u(t, ord)
          WHERE (u.t = ANY(l.muted_event_types) OR u.t = ANY(COALESCE(@set_event_types::text[], '{}')))
            AND NOT (u.t = ANY(COALESCE(@unset_event_types::text[], '{}')))
          ORDER BY u.ord)::text[] AS muted_event_types
  FROM locked l
), updated AS (
  UPDATE watchlist w
  SET release_types = c.release_types, muted_event_types = c.muted_event_types, updated_at = now()
  FROM computed c
  WHERE w.id = c.id
    AND (w.release_types IS DISTINCT FROM c.release_types
         OR w.muted_event_types IS DISTINCT FROM c.muted_event_types)
  RETURNING w.id
)
SELECT l.id AS id FROM locked l ORDER BY l.id;

-- name: GetTagByID :one
SELECT id, name FROM tags WHERE id = $1;

-- name: BulkDetachTag :many
-- Deletes the tag's links on the selected entries' artists; removed marks entries that carried it.
WITH existing AS (
  SELECT w.id, w.artist_id FROM watchlist w WHERE w.id = ANY(@entry_ids::bigint[])
), removed AS (
  DELETE FROM artist_tags l
  WHERE l.tag_id = @tag_id::bigint AND l.artist_id IN (SELECT artist_id FROM existing)
  RETURNING l.artist_id
)
SELECT e.id AS entry_id,
       (e.artist_id IN (SELECT artist_id FROM removed))::boolean AS removed
FROM existing e ORDER BY e.id;
