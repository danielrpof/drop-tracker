---
phase: 02-watchlist-core
reviewed: 2026-08-05T00:00:00Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/server/main.go
  - go.mod
  - internal/db/migrate_test.go
  - internal/db/migrations/000002_watchlist.down.sql
  - internal/db/migrations/000002_watchlist.up.sql
  - internal/db/sqlc/artists.sql.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc/querier.go
  - internal/db/sqlc/watchlist.sql.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/health_test.go
  - internal/httpserver/server.go
  - internal/httpserver/server_test.go
  - internal/httpserver/watchlist.go
  - internal/httpserver/watchlist_test.go
  - internal/watchlist/normalize_test.go
  - internal/watchlist/service.go
  - internal/watchlist/service_test.go
  - queries/artists.sql
  - queries/watchlist.sql
  - sqlc.yaml
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-08-05T00:00:00Z
**Depth:** standard
**Files Reviewed:** 20
**Status:** issues_found

## Summary

Reviewed the watchlist-core phase: the `artists`/`watchlist` migrations, the sqlc-generated query layer, `internal/watchlist.Service` (Add/List/UpdatePreferences/Remove), and the `internal/httpserver` watchlist HTTP handlers, plus their tests. SQL is fully parameterized (no injection surface), error messages are consistently scrubbed before reaching the client (verified by dedicated leak tests), the D-08 default/empty-vs-nil preference semantics are correctly implemented and well tested, and the DELETE-vs-DELETE concurrency race is explicitly handled and tested (T-02-15).

Two real correctness gaps were found, both in write paths that are less thoroughly exercised by the test suite than `Add`/`Remove`/`List`: `UpsertArtist`'s ON CONFLICT clause silently drops caller-submitted `disambiguation`/`image_url` updates, and `Service.UpdatePreferences`'s read-then-write (not transactional, not row-locked) sequence has an unhandled `pgx.ErrNoRows` path and a lost-update race under concurrent PATCH calls. Neither is exercised by any existing test. No critical/security issues were found.

## Warnings

### WR-01: UpsertArtist silently drops disambiguation/image_url on every add after the first insert

**File:** `internal/db/sqlc/artists.sql.go:12-20` (generated from `queries/artists.sql:1-8`)
**Issue:** The `ON CONFLICT (mbid) DO UPDATE` clause only refreshes `name` (unconditionally) and `deezer_id` (via `COALESCE`). `disambiguation` and `image_url` are absent from the `SET` list entirely, so once an `artists` row exists for a given `mbid`, any `disambiguation`/`image_url` value supplied on a later `Add` call (e.g. re-adding after a `Remove`, per `TestService_Remove_ThenReAddSucceeds` / `TestWatchlist_FullLifecycle`, or a client that legitimately wants to refresh metadata) is accepted by `addWatchlistRequest`, forwarded into `AddParams`/`UpsertArtistParams`, and then silently discarded — no error, no partial-update indication, `Entry` in the response even reflects the *old* stale value read back from the row. This asymmetry with `deezer_id`'s explicit `COALESCE`-based refresh looks like an oversight rather than a documented design choice; no test in this phase exercises re-adding with a changed `disambiguation`/`image_url`, so the gap is currently unguarded.
**Fix:** Mirror the `deezer_id` treatment for the other two nullable metadata columns, e.g.:
```sql
INSERT INTO artists (mbid, deezer_id, name, disambiguation, image_url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (mbid) DO UPDATE
    SET name = EXCLUDED.name,
        deezer_id = COALESCE(EXCLUDED.deezer_id, artists.deezer_id),
        disambiguation = COALESCE(EXCLUDED.disambiguation, artists.disambiguation),
        image_url = COALESCE(EXCLUDED.image_url, artists.image_url),
        updated_at = now()
RETURNING *;
```
then regenerate with `sqlc generate` and add a test analogous to `TestService_Add_ReusesExistingArtistRow` that asserts a re-add with a new `disambiguation`/`image_url` actually updates the stored row.

### WR-02: UpdatePreferences has an unhandled not-found race and a lost-update race under concurrent PATCH

**File:** `internal/watchlist/service.go:225-260`
**Issue:** `UpdatePreferences` reads the current row via `ListWatchlist` (a plain, un-locked `SELECT`), then writes via `UpdateWatchlistPreferences` (`UPDATE ... WHERE id = $1 RETURNING *`) as two separate, non-transactional statements. Two concrete failure modes follow:
1. **Unhandled not-found race:** if the watchlist row is deleted (e.g. via `DELETE /watchlist/{id}`) after the `ListWatchlist` read but before the `UPDATE` runs, `UpdateWatchlistPreferences`'s `:one` query returns `pgx.ErrNoRows` from `row.Scan` (confirmed: `pgx.ErrNoRows` is not referenced or translated anywhere in the codebase). That error is wrapped with `fmt.Errorf("update watchlist preferences: %w", err)` and returned as a plain error — it does **not** satisfy `errors.Is(err, watchlist.ErrNotFound)`. The handler's switch in `internal/httpserver/watchlist.go:240-253` therefore falls through to the generic `case err != nil` branch and returns `500 internal error` instead of the correct `404`. Contrast this with `Remove`, which explicitly distinguishes "0 rows affected" via `:execrows` and is covered by `TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404` — no equivalent test or handling exists for PATCH.
2. **Lost update between two concurrent PATCH calls:** because both axes (`release_types`, `muted_event_types`) are read once from `current` and then always written back in full, two concurrent `PATCH /watchlist/{id}` calls — one changing only `release_types`, the other changing only `muted_event_types` — can each read the same pre-update `current` row, then each write back their own changed axis plus the *other's stale, pre-update* value for the untouched axis. Whichever write lands second silently overwrites the first request's change to the axis it didn't intend to touch. `TestService_UpdatePreferences_AxesAreIndependent` proves axis independence for *sequential* calls only; no test exercises concurrent calls the way `TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404` does for DELETE.
**Fix:** Make the read-modify-write atomic, e.g. wrap in a single transaction using `SELECT ... FOR UPDATE` (or a single SQL statement that reads current values via a CTE and writes in one round trip), and translate a zero-row result from the update into `ErrNotFound` the same way `Remove` does today:
```go
tx, err := pool.Begin(ctx)
...
row, err := tx.Query(ctx, `SELECT release_types, muted_event_types FROM watchlist WHERE id = $1 FOR UPDATE`, id)
// ... merge axes, then UPDATE ... RETURNING *, using tx
// if err is pgx.ErrNoRows anywhere in this path, return ErrNotFound
```

## Info

### IN-01: JSON decoder does not reject trailing content after the top-level object

**File:** `internal/httpserver/watchlist.go:90-96`, `223-229`
**Issue:** `dec.Decode(&req)` only decodes the first JSON value from the body; `DisallowUnknownFields` guards against unexpected keys within that object, but a body like `{"mbid":"x","name":"y"}{"anything":1}` (or any trailing garbage after the first valid object) is accepted silently, since nothing checks for `io.EOF` after the decode call.
**Fix:** After `dec.Decode(&req)` succeeds, call `dec.Decode(&struct{}{})` (or `dec.More()`/read one more token) and reject with 400 if it doesn't return `io.EOF`, to fully close the strict-decoding loop that `DisallowUnknownFields` starts.

### IN-02: No length bound on deezer_id/disambiguation/image_url, unlike mbid/name

**File:** `internal/httpserver/watchlist.go:69-77, 107-114`
**Issue:** `maxMBIDRunes`/`maxNameRunes` explicitly cap `mbid` and `name` (T-02-05), but `DeezerID`, `Disambiguation`, and `ImageURL` — all free-text `*string` fields accepted from the client — have no analogous per-field bound; they're only implicitly constrained by the 64KB whole-request-body ceiling (`maxAddWatchlistBodyBytes`). This is an inconsistency in the validation story rather than an exploitable issue (Postgres `TEXT` has no practical size problem here), but worth deliberately scoping or documenting as accepted.
**Fix:** Either apply a similar rune-count cap to these fields for consistency, or add a one-line comment noting the omission is intentional (mirroring the existing `maxMBIDRunes`/`maxNameRunes` doc comments).

### IN-03: Stray blank line inside the error-sentinel var block

**File:** `internal/watchlist/service.go:28-41`
**Issue:** The `var (...)` block declaring `ErrDuplicate`/`ErrNotFound`/`ErrInvalidReleaseType`/`ErrInvalidEventType` has a trailing blank line before the closing `)` (line 40-41), inconsistent with the rest of the file's formatting.
**Fix:** Remove the stray blank line.

---

_Reviewed: 2026-08-05T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
