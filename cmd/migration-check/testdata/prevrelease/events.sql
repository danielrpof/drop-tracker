-- Distinctive excerpt of the previous release's queries/events.sql (D-15,
-- Task 2) -- not a copy of the whole file, just the constructs the
-- extractor must handle: an explicit INSERT column list, an ON CONFLICT
-- target list, a single-table bare SELECT *, a CTE whose own body selects a
-- bare column from a real table, and a flattened EXISTS subquery.

-- name: InsertEvent :execrows
INSERT INTO events (
    artist_id, source, event_type, external_id, release_group_mbid,
    title, artist_name, release_date, cover_art_url, track_count, notified_at,
    previous_track_count, release_type, watched_artist_name
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
ON CONFLICT (event_type, source, external_id) DO NOTHING;

-- name: ListUnnotified :many
SELECT * FROM events WHERE notified_at IS NULL ORDER BY created_at ASC, id ASC;

-- name: AdvanceGroupTrackCountBaseline :many
WITH existing AS (
    SELECT track_count FROM events
    WHERE event_type = 'new_release' AND source = 'musicbrainz' AND external_id = $1
    FOR UPDATE
)
UPDATE events e
SET track_count = $2
FROM existing
WHERE e.event_type = 'new_release' AND e.source = 'musicbrainz' AND e.external_id = $1
  AND (existing.track_count IS NULL OR $2::int > existing.track_count)
RETURNING existing.track_count AS previous_track_count;

-- name: HasAnyEvent :one
SELECT EXISTS(
    SELECT 1 FROM events WHERE artist_id = $1
) AS has_any;
