-- name: InsertEvent :execrows
-- 0 rows affected means the dedup key (event_type, source, external_id)
-- already existed (D-20) -- the caller does not treat this as an error,
-- only as "not newly detected." Deliberately not the
-- COALESCE(EXCLUDED.col, table.col) refresh shape UpsertArtist uses,
-- because a re-detected event must keep its original snapshot (D-20):
-- plain DO NOTHING, no SET clause at all.
INSERT INTO events (
    artist_id, source, event_type, external_id, release_group_mbid,
    title, artist_name, release_date, cover_art_url, track_count, notified_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
ON CONFLICT (event_type, source, external_id) DO NOTHING;

-- name: ListExternalIDs :many
-- Feeds the fresh-vs-seen diff (D-10): a Detector builds a
-- map[string]struct{} from this result and skips any externally-fetched
-- id already present.
SELECT external_id FROM events WHERE artist_id = $1 AND source = $2 AND event_type = $3;

-- name: HasAnyEvent :one
-- D-14's implicit seed-mode check, scoped per-source per D-15: zero
-- existing event rows for this artist+source means seed mode.
SELECT EXISTS(
    SELECT 1 FROM events WHERE artist_id = $1 AND source = $2
) AS has_any;
