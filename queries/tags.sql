-- name: GetWatchlistArtistID :one
-- Resolves a watchlist entry's artist; artist_tags keys on artists.id (TAG-07).
SELECT artist_id FROM watchlist WHERE id = $1;

-- name: GetOrCreateTag :one
-- No fallback SELECT (D-29): the no-op DO UPDATE makes RETURNING yield the
-- existing row on a collision, keeping its first-entered casing.
INSERT INTO tags (name) VALUES ($1)
ON CONFLICT ((lower(name))) DO UPDATE SET name = tags.name
RETURNING id, name;

-- name: AttachTag :execrows
-- 1 row = new link, 0 = already linked (D-21). The cap trigger enforces the
-- 10-tag limit; this statement never counts.
INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2)
ON CONFLICT (artist_id, tag_id) DO NOTHING;

-- name: DetachTag :execrows
-- Any affected count is success; 0 rows means the link was already gone (D-21).
DELETE FROM artist_tags WHERE artist_id = $1 AND tag_id = $2;

-- name: ListTags :many
-- count(w.id) counts only matched watchlist rows, so tags carried solely by
-- removed artists, or by none, still list with 0 instead of being dropped by
-- an inner join (D-11). lower(name) is unique, so the order is total.
SELECT t.id, t.name, count(w.id)::bigint AS carrier_count
FROM tags t
LEFT JOIN artist_tags link ON link.tag_id = t.id
LEFT JOIN watchlist w ON w.artist_id = link.artist_id
GROUP BY t.id, t.name
ORDER BY lower(t.name), t.id;

-- name: RenameTag :one
-- No collision pre-check: a tags_name_lower_idx unique violation is mapped
-- to *CollisionError by Service.Rename (D-22).
UPDATE tags SET name = $2 WHERE id = $1 RETURNING id, name;

-- name: GetTagByName :one
-- Only after a tags_name_lower_idx violation, to identify the collider (D-22).
SELECT id, name FROM tags WHERE lower(name) = lower($1);

-- name: CountCarriersForTags :one
-- Watched carriers across the given tags, each artist counted once (D-19).
SELECT count(DISTINCT w.artist_id)::bigint AS carrier_count
FROM artist_tags link
JOIN watchlist w ON w.artist_id = link.artist_id
WHERE link.tag_id = ANY(@tag_ids::bigint[]);

-- name: DeleteTagCountingCarriers :one
-- Both CTEs share one snapshot, so counted never sees deleted's cascade and
-- reports the carriers the delete removed (TAG-06). Zero rows = no such tag.
WITH counted AS (
    SELECT count(w.id)::bigint AS carrier_count
    FROM artist_tags link
    JOIN watchlist w ON w.artist_id = link.artist_id
    WHERE link.tag_id = $1
), deleted AS (
    DELETE FROM tags WHERE id = $1 RETURNING id
)
SELECT deleted.id, counted.carrier_count FROM deleted, counted;

-- Merge statement order and why it never inserts: ADR 0004.

-- name: LockTagsForMerge :many
-- Locks both tags in id order so concurrent merges cannot deadlock; it also
-- parks a concurrent attach/rename until commit.
SELECT id, name FROM tags WHERE id = ANY(@ids::bigint[]) ORDER BY id FOR UPDATE;

-- name: DeleteDuplicateSourceLinks :execrows
-- Drops source links for artists already carrying the target, which would
-- otherwise duplicate once RepointSourceLinks runs.
DELETE FROM artist_tags src
WHERE src.tag_id = @source_id
  AND EXISTS (
    SELECT 1 FROM artist_tags dup
    WHERE dup.artist_id = src.artist_id AND dup.tag_id = @target_id
  );

-- name: RepointSourceLinks :execrows
-- Repoints the remaining links via UPDATE, never INSERT (ADR 0004).
UPDATE artist_tags SET tag_id = @target_id WHERE tag_id = @source_id;

-- name: DeleteTag :execrows
DELETE FROM tags WHERE id = $1;

-- name: CountCarriers :one
-- Watched carriers of one tag, for the merge target's post-merge count.
SELECT count(w.id)::bigint AS carrier_count
FROM artist_tags link
JOIN watchlist w ON w.artist_id = link.artist_id
WHERE link.tag_id = $1;
