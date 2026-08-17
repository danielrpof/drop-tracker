-- name: InsertEvent :execrows
-- 0 rows affected means the dedup key (event_type, source, external_id)
-- already existed (D-20) -- the caller does not treat this as an error,
-- only as "not newly detected." Deliberately not the
-- COALESCE(EXCLUDED.col, table.col) refresh shape UpsertArtist uses,
-- because a re-detected event must keep its original snapshot (D-20):
-- plain DO NOTHING, no SET clause at all. previous_track_count and
-- release_type (Phase 5's D-04/Pitfall-3 columns) are appended after the
-- existing eleven columns, as $12/$13, so every pre-existing positional
-- parameter keeps its number -- D-20's write-once guarantee applies to
-- these two snapshot columns exactly as it does to the original nine.
INSERT INTO events (
    artist_id, source, event_type, external_id, release_group_mbid,
    title, artist_name, release_date, cover_art_url, track_count, notified_at,
    previous_track_count, release_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
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

-- name: AdvanceGroupTrackCountBaseline :many
-- Atomic replacement (PERF-04, 11-RESEARCH.md Pattern 2) for the former
-- two-statement GroupTrackCountBaseline SELECT + SetGroupTrackCountBaseline
-- UPDATE -- those two round trips left a check-then-act window where two
-- concurrent callers racing the same release group could both read the old
-- baseline before either wrote, letting the second writer silently clobber
-- the first writer's correct, higher value with its own stale one. This
-- statement closes that window entirely rather than narrowing it: the CTE's
-- FOR UPDATE takes a row lock on the group's own new_release row, so a
-- second concurrent caller racing the same external_id blocks on that lock
-- until the first transaction commits, then re-evaluates against the
-- just-committed value.
--
-- Zero rows returned means no advance happened: the fresh count was not a
-- genuine increase over the already-committed baseline (D-02's "equal or
-- lower: no event" case, strictly greater-than, now enforced by Postgres
-- itself). One row returned means the write landed; previous_track_count
-- NULL vs non-NULL is the has_baseline distinction the removed
-- GroupTrackCountBaseline query used to report, letting the caller keep
-- branching on "silently established" vs "advanced, fire an event" from
-- this one call's result alone.
--
-- Keyed on external_id (not release_group_mbid, which the removed read
-- query used) -- a deliberate narrowing: for a musicbrainz new_release row,
-- external_id and release_group_mbid hold the same release-group MBID (see
-- internal/detection/musicbrainz.go's new_release insert), and it is the
-- new_release row's own track_count that the removed write query mutated,
-- so this statement reads exactly the value the old pair wrote.
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

-- name: MarkNotified :execrows
-- Precondition: only ever called after discord.Client.Send has confirmed a
-- 204 for this row (D-09) -- never before. The AND notified_at IS NULL
-- predicate is load-bearing, not decorative: it makes the ack idempotent, so
-- a second acknowledgement of an already-delivered row affects zero rows
-- instead of overwriting the recorded delivery time.
UPDATE events SET notified_at = now() WHERE id = $1 AND notified_at IS NULL;

-- name: ListEvents :many
-- Phase 6's HIST-01 history feed backing query (D-05): one global
-- chronological read across all watched artists, newest first -- not a
-- per-artist drill-down. Ordered and keyset-paginated on id DESC, not
-- created_at: this file's own ListUnnotified comment already documents why
-- created_at alone is not a unique order -- a seed cycle inserts many rows
-- sharing one created_at timestamp, so ordering by it alone would make page
-- boundaries non-deterministic across a "load more" click (06-RESEARCH.md
-- Pattern 2, Pitfall 2). id (BIGSERIAL) is already unique and monotonic and
-- needs no secondary tiebreak column.
--
-- artist_id, event_type and cursor are all optional sqlc.narg filters, each
-- cast on both sides of its "IS NULL OR" predicate so sqlc's type inference
-- has no ambiguity. "IS NULL OR" keeps one static SQL string sqlc can
-- type-check, instead of building WHERE clauses in Go (06-RESEARCH.md
-- Anti-Patterns). cursor is absent on the first page and set to the previous
-- page's last row's id on subsequent pages.
--
-- Phase 10 (DATA-02, D-01/D-04): this is the ONLY query in this file that
-- ever gets a retention cutoff. cutoff is sqlc.arg, not sqlc.narg -- it is
-- never caller-optional, so there is no code path where a caller passes a
-- null cutoff and gets back unfiltered, out-of-window rows (T-10-03). The
-- comparison is >=, not >: an event exactly at the boundary stays visible
-- (D-04). This is a read-side filter only, nothing is deleted -- an
-- aged-out row stays fully present and fully visible to every query below
-- that intentionally has no cutoff: ListExternalIDs (dedup keys),
-- HasAnyEvent (seed-mode), AdvanceGroupTrackCountBaseline (deluxe
-- baselines), and ListUnnotified (pending notifications). Adding this predicate to any of
-- those four is the exact regression Phase 10's success criteria 3-5 exist
-- to catch -- do not "fix" them to also filter by retention.
SELECT id, artist_id, source, event_type, external_id, release_group_mbid,
       title, artist_name, release_date, cover_art_url, track_count,
       previous_track_count, release_type, notified_at, created_at
FROM events
WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id')::bigint)
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type')::text)
  AND (sqlc.narg('cursor')::bigint IS NULL OR id < sqlc.narg('cursor')::bigint)
  AND created_at >= sqlc.arg('cutoff')::timestamptz
ORDER BY id DESC
LIMIT sqlc.arg('page_size');

-- name: HasOlderEvents :one
-- Phase 10 (DATA-02, D-06): answers a question ListEvents' own result page
-- cannot -- whether this request's artist_id/event_type scope has ANY event
-- older than the retention cutoff, so the frontend can distinguish "no
-- events ever" from "events exist but every one of them aged out" (the
-- History empty state cannot tell those apart from an empty page alone).
-- Mirrors ListEvents' two optional filters exactly (same sqlc.narg casts on
-- both sides), but deliberately omits ListEvents' pagination-position
-- parameter: this answers a property of the whole filtered scope, not of
-- the current page, so a "Load more" click must not change the answer.
-- Uses EXISTS with no LIMIT inside it, following HasAnyEvent's existing
-- idiom in this file --
-- EXISTS already short-circuits on the first matching row. created_at <
-- cutoff (strict less-than) is the exact complement of ListEvents' >=, so a
-- row exactly at the boundary is never double-counted as both visible and
-- older (D-04 stays consistent across both queries).
SELECT EXISTS(
    SELECT 1 FROM events
    WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id')::bigint)
      AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type')::text)
      AND created_at < sqlc.arg('cutoff')::timestamptz
) AS has_older;
