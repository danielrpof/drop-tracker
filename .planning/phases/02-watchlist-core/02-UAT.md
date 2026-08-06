---
status: diagnosed
phase: 02-watchlist-core
source: [02-VERIFICATION.md]
started: 2026-08-06T02:00:00Z
updated: 2026-08-06T18:15:00Z
---

## Current Test

[testing complete]

## Tests

### 1. End-to-end curl walkthrough and JSON ergonomics review
expected: Run `make db-up && make run`, then exercise all four `/watchlist` routes with curl in sequence: add an artist, list it, narrow its release types via PATCH, mute an event category via PATCH, list again to confirm both axes, delete it, list once more to confirm it's gone. JSON response bodies should read the way a future React client (Phase 6) would want to consume them; no error response at any point should contain database internals (DSN, driver text, SQLSTATE codes).
result: pass

### 2. Decide the disposition of WR-01 and WR-02 (code review findings)
expected: Review 02-REVIEW.md's WR-01 (UpsertArtist silently drops disambiguation/image_url on re-add) and WR-02 (UpdatePreferences has an unhandled not-found race — returns 500 instead of 404 when a row is deleted between read and write — and a lost-update race between two concurrent PATCH calls touching different axes of the same entry) and decide whether either needs a fast-follow fix before Phase 3/4 start writing to these same tables, or is an accepted risk for this single-operator v1 deployment.
result: issue
reported: "fix both WRs"
severity: major

## Summary

total: 2
passed: 1
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-02-2a
  truth: "Re-adding an artist (POST /watchlist for an mbid already in the artists table) with a new disambiguation/image_url updates the stored row instead of silently keeping the stale value"
  status: failed
  reason: "User decided: fix both WR-01 and WR-02 rather than accept as risk for v1"
  severity: major
  test: 2
  root_cause: "UpsertArtist's ON CONFLICT (mbid) DO UPDATE clause (queries/artists.sql, generated into internal/db/sqlc/artists.sql.go:12-20) refreshes name (unconditionally) and deezer_id (via COALESCE) but omits disambiguation and image_url from the SET list entirely, so those two columns never update on conflict."
  artifacts:
    - path: "queries/artists.sql"
      issue: "INSERT ... ON CONFLICT (mbid) DO UPDATE SET list is missing disambiguation and image_url"
    - path: "internal/db/sqlc/artists.sql.go:12-20"
      issue: "generated code from the above query, inherits the same gap"
  missing:
    - "Add disambiguation = COALESCE(EXCLUDED.disambiguation, artists.disambiguation) to the ON CONFLICT SET clause"
    - "Add image_url = COALESCE(EXCLUDED.image_url, artists.image_url) to the ON CONFLICT SET clause"
    - "Regenerate with sqlc generate"
    - "Add a test analogous to TestService_Add_ReusesExistingArtistRow asserting a re-add with a new disambiguation/image_url updates the stored row"
  debug_session: ""
  source: "02-REVIEW.md WR-01"

- gap_id: G-02-2b
  truth: "PATCH /watchlist/{id} returns 404 (not 500) when the row is deleted between read and write, and two concurrent PATCH calls touching different preference axes do not lose each other's update"
  status: failed
  reason: "User decided: fix both WR-01 and WR-02 rather than accept as risk for v1"
  severity: major
  test: 2
  root_cause: "UpdatePreferences (internal/watchlist/service.go:225-260) reads the current row via a plain unlocked SELECT (ListWatchlist) then writes via a separate UPDATE ... RETURNING *, as two non-transactional statements. (1) If the row is deleted between the two, pgx.ErrNoRows from the UPDATE's row.Scan is wrapped as a plain error and never translated to watchlist.ErrNotFound, so the handler's switch falls through to its generic 500 branch instead of 404. (2) Because both axes are read once and always written back in full, two concurrent PATCH calls each touching a different axis can each read the same pre-update row and each write back the other's stale value for the axis they didn't intend to touch — whichever write lands second silently reverts the first request's change."
  artifacts:
    - path: "internal/watchlist/service.go:225-260"
      issue: "read-then-write UpdatePreferences is non-transactional: unhandled not-found race + lost-update race between concurrent PATCH calls"
    - path: "internal/httpserver/watchlist.go:240-253"
      issue: "switch only translates errors.Is(err, watchlist.ErrNotFound); the wrapped pgx.ErrNoRows from the race never matches, so it falls to the generic 500 case"
  missing:
    - "Wrap the read-modify-write in a single transaction using SELECT ... FOR UPDATE (or an equivalent single-round-trip CTE)"
    - "Translate a zero-row result from the UPDATE into watchlist.ErrNotFound the same way Remove already does"
    - "Add a concurrent-PATCH test analogous to TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404 that asserts neither axis is lost when two PATCH calls touching different axes race"
  debug_session: ""
  source: "02-REVIEW.md WR-02"
