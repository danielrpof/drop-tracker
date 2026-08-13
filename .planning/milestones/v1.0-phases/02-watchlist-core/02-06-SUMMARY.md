---
phase: 02-watchlist-core
plan: 06
subsystem: watchlist-api
tags: [postgres, pgx, sqlc, watchlist, concurrency, gap-closure]

requires:
  - phase: 02-watchlist-core (plan 01)
    provides: artists/watchlist schema, sqlc query surface
  - phase: 02-watchlist-core (plan 04)
    provides: PATCH /watchlist/{id} handler and the two-statement UpdatePreferences this plan replaces
provides:
  - "UpdateWatchlistPreferences as a single data-modifying CTE: each preference axis resolves under its own CASE/ELSE against the row version the UPDATE itself locked, closing the lost-update and not-found races"
  - "Service.UpdatePreferences as a one-round-trip method that translates pgx.ErrNoRows to watchlist.ErrNotFound directly, with no separate unlocked pre-read"
  - "Deterministic held-row-lock test pattern (pool.Begin + uncommitted Exec + goroutine + time.After) for proving concurrency fixes without depending on real scheduler races"
affects: []

actuals:
  tokens: 7401
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Partial-update merge for independent columns lives inside a data-modifying CTE, not in application code: an explicit boolean 'set this axis' parameter per column plus CASE ... ELSE <qualified column> reads the untouched value from the row version the UPDATE itself locked, making the merge safe under concurrent writers without an explicit transaction in the service layer"
    - "Deterministic concurrency tests hold a row lock via an uncommitted pool.Begin()/Exec() transaction and assert (via time.After) that the code under test is blocked, rather than racing goroutines and hoping for a particular interleaving"

key-files:
  created: []
  modified:
    - queries/watchlist.sql
    - internal/db/sqlc/watchlist.sql.go
    - internal/db/sqlc/querier.go
    - internal/watchlist/service.go
    - internal/watchlist/service_test.go
    - internal/httpserver/watchlist_test.go

key-decisions:
  - "Qualified every column reference inside the CTE's UPDATE (watchlist.id, watchlist.release_types, etc.) instead of bare column names -- sqlc's analyzer reported 'column reference \"id\" is ambiguous' for the unqualified WHERE clause even though a bare UPDATE...WHERE id = ... would be unambiguous to Postgres itself; qualifying every reference resolved it without changing the query's semantics"
  - "Both new axes carried as always-non-nil []string{} in UpdateWatchlistPreferencesParams even when the CASE branch will never read them -- keeps the pgx array parameter encoding unambiguous and keeps the call site's intent readable, per the plan's explicit instruction"

requirements-completed: [WLST-05, WLST-06]

coverage:
  - id: D1
    description: "PATCH /watchlist/{id} for a row deleted a moment before the write lands returns 404 with the D-13 error body, never 500 (G-02-2b, WR-02)"
    requirement: "WLST-05"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500"
        status: pass
    human_judgment: false
  - id: D2
    description: "Two concurrent PATCH calls touching different preference axes both take effect -- neither reverts the other's axis (D-05)"
    requirement: "WLST-06"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every pre-existing UpdatePreferences/Patch behaviour (partial update, empty-vs-absent, dedup/canonicalisation, 400 on invalid value, 404 on unknown id) is unchanged"
    requirement: "WLST-05"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go (all pre-existing TestService_UpdatePreferences_* tests, unedited)"
        status: pass
      - kind: integration
        ref: "internal/httpserver full suite (all pre-existing TestWatchlist_Patch_* tests, unedited)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Committed sqlc output regenerated from the edited query source with no working-tree diff; no Go module added/removed/upgraded"
    requirement: "WLST-05"
    verification:
      - kind: automated
        ref: "sqlc generate followed by git diff --exit-code -- internal/db/sqlc/ (make unavailable in this shell; ran sqlc-check's underlying commands directly)"
        status: pass
      - kind: automated
        ref: "git diff --exit-code -- go.mod go.sum"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-06
status: complete
---

# Phase 2 Plan 06: Close Gap G-02-2b — Single-Statement UpdatePreferences Summary

`UpdateWatchlistPreferences` is now one data-modifying CTE instead of an unlocked `ListWatchlist` read followed by a separate `UPDATE`: each preference axis resolves under a `CASE` whose `ELSE` reads the row version the statement itself locked, so a blocked concurrent writer re-evaluates against the newly committed row instead of clobbering it, and a row deleted mid-write now surfaces as `pgx.ErrNoRows` translated directly to `watchlist.ErrNotFound` (404) instead of a wrapped 500. Two deterministic held-row-lock tests at the service layer and two 25-iteration end-to-end HTTP tests pin both properties.

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-08-06
- **Tasks:** 2
- **Files modified:** 6 (0 created, 6 modified)

## Accomplishments

- `queries/watchlist.sql`'s `UpdateWatchlistPreferences` rewritten as a `WITH updated AS (UPDATE ... RETURNING ...) SELECT ... FROM updated JOIN artists`: each axis's `SET` clause is a `CASE WHEN @set_<axis>::boolean THEN @<axis>::text[] ELSE watchlist.<axis> END`, and the outer `SELECT` joins `artists` and aliases every column the same way `ListWatchlist` does, so the returned row is a self-contained `ListWatchlistRow`-shaped struct
- Regenerated `internal/db/sqlc/watchlist.sql.go` and `internal/db/sqlc/querier.go`: `UpdateWatchlistPreferencesParams` gained `SetReleaseTypes`/`SetMutedEventTypes bool` fields, and the return type changed from `Watchlist` to a new `UpdateWatchlistPreferencesRow` carrying the joined artist fields
- `Service.UpdatePreferences` rewritten to one round trip: the `ListWatchlist` pre-read, the linear id scan, and the two substitution blocks are all deleted; validation still runs first, then a single `UpdateWatchlistPreferences` call, with `errors.Is(err, pgx.ErrNoRows)` translated to `ErrNotFound` before the existing wrap-and-return-other-errors path
- `TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost` and `TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound` (real Postgres, held row lock via an uncommitted `pool.Begin()`/`Exec()` transaction, `time.After(300ms)` asserting the update is genuinely blocked): both proven red against the old implementation for distinct reasons, then green against the rewrite
- `TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive` and `TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500` (real Postgres + `httptest.NewServer`, 25 iterations each, barrier-released goroutines): end-to-end analogues proving the same two properties survive the handler and the network round trip, with the 404 path additionally asserted to carry a scrubbed D-13 body
- Full suite green (`go test ./... -count=1` and `go test ./... -short`), `make sqlc-check`'s underlying commands clean, `go.mod`/`go.sum`/`internal/httpserver/watchlist.go` all byte-identical to before this plan

## Task Commits

Task 1 followed the RED/GREEN TDD cycle; task 2 added the end-to-end guard tests against task 1's fix (no separate GREEN commit needed, since it makes no production change):

1. **Task 1 (RED):** `a2a1e7d` (test) — `TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost` and `TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound`, confirmed failing for distinct reasons against the two-statement implementation
2. **Task 1 (GREEN):** `a429717` (feat) — single-statement CTE rewrite, regenerated sqlc output, one-round-trip `Service.UpdatePreferences`
3. **Task 2:** `12943f8` (test) — `TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive` and `TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500`, passing against task 1's fix on first run; full suite re-run confirmed no regression

## Files Created/Modified

- `queries/watchlist.sql` — `UpdateWatchlistPreferences` rewritten as a single data-modifying CTE with per-axis boolean set-flags
- `internal/db/sqlc/watchlist.sql.go`, `internal/db/sqlc/querier.go` — regenerated (`sqlc generate`, committed clean per `make sqlc-check`'s equivalent commands)
- `internal/watchlist/service.go` — `UpdatePreferences` rewritten to one round trip; `pgx.ErrNoRows` translated to `ErrNotFound`
- `internal/watchlist/service_test.go` — two new deterministic held-lock tests
- `internal/httpserver/watchlist_test.go` — two new 25-iteration end-to-end concurrency tests

## Decisions Made

- **Qualified every column reference inside the CTE's `UPDATE`** (`watchlist.id`, `watchlist.release_types`, `watchlist.muted_event_types`, `watchlist.artist_id`, `watchlist.created_at`, `watchlist.updated_at`) rather than leaving them bare. sqlc's analyzer rejected the first draft with `column reference "id" is ambiguous` on the bare `WHERE id = @id` inside the CTE, even though that clause alone (scoped only to `watchlist`) would be unambiguous to Postgres directly — sqlc's own type-inference pass appears to consider the full statement's namespace including the outer join. Qualifying every reference resolved it with no change to the query's runtime semantics.
- **Both preference arrays always sent as non-nil `[]string{}`** in the params even for the untouched axis, per the plan's explicit instruction — the `CASE` branch never reads the untouched axis's array value, but a non-nil value keeps the pgx array parameter encoding unambiguous and keeps the call site legible.

## Deviations from Plan

None — both tasks executed exactly as written, including the exact CTE shape, the explicit boolean set-flag design over a NULL-means-untouched `COALESCE`, and the held-row-lock test structure specified in `<behavior>`. The one implementation adjustment (qualifying column references to satisfy sqlc's analyzer) is a mechanical SQL-syntax fix within the same statement shape the plan specified, not a deviation from its design.

## Issues Encountered

`make` remains unavailable on this execution shell's `PATH` (consistent with plan 02-05's note); ran the `sqlc-check` and `sqlc` Makefile targets' underlying commands directly (`sqlc generate`, `git diff --exit-code -- internal/db/sqlc/`) instead, which is behaviorally identical. `docker ps` confirmed `drop-tracker-postgres-1` was already running and `sqlc version` reported `v1.31.1`, so task 1's `<precondition>` was met without any setup step.

## Next Phase Readiness

- Gap `G-02-2b` is closed: both `must_haves.truths` conditions (404 not 500 on delete-mid-write, both axes survive concurrent independent PATCHes) are covered by deterministic service-layer tests and end-to-end HTTP tests
- `DELETE` (plan 02-04, `:execrows`) and `PATCH` (this plan, locked CTE) are now both honest under concurrency; `POST` (plan 02-05) has its own gap (G-02-2a) already closed. Phase 2's mutation surface has no known outstanding concurrency gaps.
- Phase 4's detection engine can write to `watchlist`-adjacent tables on a poll schedule without reopening either of this plan's two race conditions on the preferences path

---
*Phase: 02-watchlist-core*
*Completed: 2026-08-06*

## Self-Check: PASSED

- `queries/watchlist.sql` — FOUND
- `internal/db/sqlc/watchlist.sql.go` — FOUND
- `internal/db/sqlc/querier.go` — FOUND
- `internal/watchlist/service.go` — FOUND
- `internal/watchlist/service_test.go` — FOUND
- `internal/httpserver/watchlist_test.go` — FOUND
- `.planning/phases/02-watchlist-core/02-06-SUMMARY.md` — FOUND
- Commit `a2a1e7d` — FOUND in `git log --oneline --all`
- Commit `a429717` — FOUND in `git log --oneline --all`
- Commit `12943f8` — FOUND in `git log --oneline --all`
