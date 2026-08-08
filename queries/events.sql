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

-- name: ListUnnotified :many
-- D-11's Phase 5 groundwork: SELECT WHERE notified_at IS NULL, ORDER BY
-- created_at ASC, id ASC for a deterministic total order (a plain
-- created_at ordering alone is not unique -- a seed cycle's rows share one
-- timestamp, see seedNotifiedAt). This is also the instrument plan 04-02's
-- own tests use to prove seeded rows are excluded (D-13).
SELECT * FROM events WHERE notified_at IS NULL ORDER BY created_at ASC, id ASC;

-- name: GroupTrackCountBaseline :one
-- Plan 04-04's baseline lookup for a release-group's deluxe-change
-- comparison (D-01/D-02), option-a resolution (04-01's Task 1 checkpoint):
-- track_count lives directly on the events row, not a second table.
-- has_baseline distinguishes "no baseline recorded yet" (COUNT is 0, this
-- group has never had track_count populated) from "baseline recorded as
-- zero" -- collapsing those two into one COALESCE(...,0) is exactly
-- 04-RESEARCH.md Pitfall #1's false-positive mechanism: the caller MUST
-- branch on has_baseline before comparing, never compare against baseline
-- alone.
SELECT COALESCE(MAX(track_count), 0)::int AS baseline,
       COUNT(track_count) > 0 AS has_baseline
FROM events
WHERE source = 'musicbrainz' AND release_group_mbid = $1;

-- name: SetGroupTrackCountBaseline :execrows
-- Mutates track_count on the group's own new_release row -- this is
-- operational baseline state, not the D-12 display snapshot (title/
-- artist_name/release_date/cover_art_url), which stays write-once via
-- InsertEvent's ON CONFLICT DO NOTHING per D-20. No snapshot column is
-- ever written twice by this statement.
UPDATE events
SET track_count = $2
WHERE event_type = 'new_release' AND source = 'musicbrainz' AND external_id = $1;
