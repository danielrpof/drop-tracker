-- name: GetWatchlistArtistID :one
-- Resolves the artist a watchlist entry belongs to, so attach/detach can
-- address artist_tags (which keys on artists.id, TAG-07) through the same
-- entry id every other watchlist route uses (D-20).
SELECT artist_id FROM watchlist WHERE id = $1;

-- name: GetOrCreateTag :one
-- One statement, no fallback SELECT (D-29): the no-op DO UPDATE is what
-- makes RETURNING yield the existing row on a collision, preserving the
-- first-entered casing (TAG-03). Conflict target is the tags_name_lower_idx
-- expression index from migration 000010.
INSERT INTO tags (name) VALUES ($1)
ON CONFLICT ((lower(name))) DO UPDATE SET name = tags.name
RETURNING id, name;

-- name: AttachTag :execrows
-- Rows affected: 1 = new link, 0 = the artist already carried this tag
-- (D-21 idempotent attach). The artist_tags_cap_trigger (migration 000010)
-- enforces the 10-tag cap; this statement never counts anything itself.
INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2)
ON CONFLICT (artist_id, tag_id) DO NOTHING;

-- name: DetachTag :execrows
-- Any affected count is success (D-21): 0 rows means the link was already
-- gone, which is not an error for an idempotent detach.
DELETE FROM artist_tags WHERE artist_id = $1 AND tag_id = $2;

-- name: ListTags :many
-- Watched-only carrier count (D-11): count(w.id) counts a row only when the
-- LEFT JOIN watchlist actually matched, so a tag whose only links belong to
-- removed artists (or no links at all) still surfaces with count 0 (D-12,
-- D-13) instead of being dropped by an inner join. Ordered by lower(name)
-- then id: tags_name_lower_idx makes lower(name) unique, so this is a
-- total, repeatable order (TAG-05).
SELECT t.id, t.name, count(w.id)::bigint AS carrier_count
FROM tags t
LEFT JOIN artist_tags link ON link.tag_id = t.id
LEFT JOIN watchlist w ON w.artist_id = link.artist_id
GROUP BY t.id, t.name
ORDER BY lower(t.name), t.id;

-- name: RenameTag :one
-- A plain rename by id. A collision is never checked here -- it surfaces as
-- a tags_name_lower_idx unique violation, which Service.Rename maps to
-- *CollisionError (D-09, D-22). A case-only rename of the same row cannot
-- conflict with its own index entry.
UPDATE tags SET name = $2 WHERE id = $1 RETURNING id, name;

-- name: GetTagByName :one
-- Used only *after* a tags_name_lower_idx violation, to identify the
-- collider (D-22) -- never a pre-check.
SELECT id, name FROM tags WHERE lower(name) = lower($1);

-- name: CountCarriersForTags :one
-- Watched carriers across one or more tags, each artist counted once even
-- when it carries more than one of the given tags (union count, D-19/SC3).
SELECT count(DISTINCT w.artist_id)::bigint AS carrier_count
FROM artist_tags link
JOIN watchlist w ON w.artist_id = link.artist_id
WHERE link.tag_id = ANY(@tag_ids::bigint[]);

-- name: DeleteTagCountingCarriers :one
-- Both CTEs read the same pre-statement snapshot (Postgres WITH semantics):
-- counted's SELECT never sees deleted's cascade removal of artist_tags, so
-- the reported count is exactly the watched-carrier count the delete
-- removed (TAG-06). Zero rows means the tag did not exist.
WITH counted AS (
    SELECT count(w.id)::bigint AS carrier_count
    FROM artist_tags link
    JOIN watchlist w ON w.artist_id = link.artist_id
    WHERE link.tag_id = $1
), deleted AS (
    DELETE FROM tags WHERE id = $1 RETURNING id
)
SELECT deleted.id, counted.carrier_count FROM deleted, counted;
