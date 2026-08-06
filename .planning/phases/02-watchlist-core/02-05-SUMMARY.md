---
phase: 02-watchlist-core
plan: 05
subsystem: watchlist-api
tags: [postgres, pgx, sqlc, watchlist, gap-closure]

requires:
  - phase: 02-watchlist-core (plan 01)
    provides: artists/watchlist schema, UpsertArtist query, Service.Add
  - phase: 02-watchlist-core (plan 04)
    provides: phase-complete /watchlist surface and 02-UAT.md's WR-01/WR-02 findings this plan closes one half of
provides:
  - "UpsertArtist ON CONFLICT SET list covering all three nullable metadata columns (deezer_id, disambiguation, image_url), each via COALESCE(EXCLUDED.<col>, artists.<col>)"
  - "Real-Postgres tests pinning both halves of the COALESCE contract: a supplied value refreshes on re-add, an omitted value is preserved"
affects: []

actuals:
  tokens: 2700
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Nullable metadata columns in an ON CONFLICT SET list use COALESCE(EXCLUDED.<col>, table.<col>) uniformly -- a caller omitting an optional field is 'no opinion', never 'clear it'; only a NOT NULL, always-required column (name) gets a bare EXCLUDED assignment"

key-files:
  created: []
  modified:
    - queries/artists.sql
    - internal/db/sqlc/artists.sql.go
    - internal/db/sqlc/querier.go
    - internal/watchlist/service_test.go

key-decisions:
  - "internal/db/sqlc/querier.go was included in the sqlc-regeneration commit alongside artists.sql.go even though the plan's files_modified list only named artists.sql.go -- sqlc propagates the query's new leading comment into the Querier interface's doc comment, so querier.go is also regenerated output and must travel with it or make sqlc-check would fail on a real machine with make available"

requirements-completed: [WLST-02]

coverage:
  - id: D1
    description: "Re-adding an artist with a changed disambiguation or image_url stores the new value -- the API response reflects it and a direct SELECT confirms it (G-02-2a, WR-01)"
    requirement: "WLST-02"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Add_RefreshesArtistMetadataOnReAdd"
        status: pass
    human_judgment: false
  - id: D2
    description: "Re-adding an artist that omits disambiguation, image_url or deezer_id leaves the stored value intact -- an omitted field is not a blanking instruction"
    requirement: "WLST-02"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Add_OmittedMetadataSurvivesReAdd"
        status: pass
    human_judgment: false
  - id: D3
    description: "The artists master row is reused rather than duplicated on re-add -- one row per mbid, whatever metadata arrives with it (D-03)"
    requirement: "WLST-02"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Add_RefreshesArtistMetadataOnReAdd (artists row count == 1 assertion)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Committed sqlc output is regenerated from the edited query source, no working-tree diff under internal/db/sqlc/"
    requirement: "WLST-02"
    verification:
      - kind: manual
        ref: "sqlc generate followed by git diff --exit-code -- internal/db/sqlc/ (make unavailable in this shell; ran the two commands make sqlc-check composes, directly)"
        status: pass
    human_judgment: false
  - id: D5
    description: "No Go module is added, removed or upgraded by this plan"
    requirement: "WLST-02"
    verification:
      - kind: automated
        ref: "git diff --exit-code -- go.mod go.sum"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-08-06
status: complete
---

# Phase 2 Plan 05: Close Gap G-02-2a — Re-add Metadata Refresh Summary

`UpsertArtist`'s `ON CONFLICT (mbid) DO UPDATE` clause now refreshes `disambiguation` and `image_url` the same way it already refreshed `deezer_id` — via `COALESCE(EXCLUDED.<col>, artists.<col>)` — so a re-add carrying changed metadata updates the stored row and the response `Entry` built from it, while a re-add that omits a field leaves what's stored alone. Two real-Postgres tests pin both directions of this contract.

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-08-06
- **Tasks:** 2
- **Files modified:** 4 (0 created, 4 modified)

## Accomplishments

- `queries/artists.sql`'s `UpsertArtist` SET list widened from two clauses (`name`, `deezer_id`) to five, adding `disambiguation = COALESCE(EXCLUDED.disambiguation, artists.disambiguation)` and `image_url = COALESCE(EXCLUDED.image_url, artists.image_url)`, with a leading comment recording why the list must stay exhaustive and why `name` alone gets a bare `EXCLUDED` assignment (`NOT NULL`, always required, D-04)
- Regenerated `internal/db/sqlc/artists.sql.go` and `internal/db/sqlc/querier.go` (the interface doc comment picks up the query's new leading comment) — `sqlc generate` followed by a clean `git diff` under `internal/db/sqlc/` confirms the committed codegen matches the edited source
- `TestService_Add_RefreshesArtistMetadataOnReAdd` (real Postgres): re-adds an artist with a changed disambiguation and image URL after deleting the intervening watchlist row (mirroring `TestService_Add_ReusesExistingArtistRow`'s re-add path), and asserts the new values land in both the returned `Entry` and a direct `SELECT`, while `artists` still holds exactly one row for the mbid (D-03)
- `TestService_Add_OmittedMetadataSurvivesReAdd` (real Postgres): re-adds with all three optional pointers nil after first populating `deezer_id`, `disambiguation` and `image_url`, and asserts all three survive unchanged in both the stored row and the returned `Entry` — three independent assertions per field per the plan's instruction, so a regression names which column broke
- Full test suite (`go test ./...`) green, including the three pre-existing re-add tests (`TestService_Add_ReusesExistingArtistRow`, `TestService_Remove_ThenReAddSucceeds`, `TestWatchlist_FullLifecycle`) — none of which asserted on metadata before this plan and none of which regressed

## Task Commits

Task 1 followed the RED/GREEN TDD cycle; task 2 added a guard test that was expected to (and did) pass on first run against task 1's fix, so it has no separate GREEN commit of its own:

1. **Task 1 (RED):** `79fbe06` (test) — `TestService_Add_RefreshesArtistMetadataOnReAdd`, confirmed failing on the stale `Disambiguation` value (not a compile error) before the query was touched
2. **Task 1 (GREEN):** `a9924ae` (feat) — widened `UpsertArtist`'s `ON CONFLICT` SET list, regenerated `artists.sql.go` and `querier.go`
3. **Task 2 (guard test):** `2f5fe49` (test) — `TestService_Add_OmittedMetadataSurvivesReAdd`, passing against task 1's fix; full suite re-run confirmed no regression

## Files Created/Modified

- `queries/artists.sql` — `UpsertArtist`'s `ON CONFLICT` SET list widened to five clauses, exhaustive over every mutable metadata column
- `internal/db/sqlc/artists.sql.go`, `internal/db/sqlc/querier.go` — regenerated (`sqlc generate`, committed clean per the `make sqlc-check` equivalent)
- `internal/watchlist/service_test.go` — two new real-Postgres tests plus a file-local `strptr` helper (`func strptr(s string) *string { return &s }`), used by both new tests instead of taking the address of a loop variable or shared literal inline

## Decisions Made

- **`querier.go` traveled with the sqlc regeneration commit** even though the plan's `files_modified` frontmatter only listed `artists.sql.go` — sqlc mirrors a query's leading comment into the `Querier` interface's method doc comment, so `querier.go` changed too. Omitting it would have left `make sqlc-check` (which diffs the whole `internal/db/sqlc/` directory) failing on any machine with `make` installed. Applied as a Rule 3 auto-fix (blocking issue: incomplete regenerated output).
- **`make` unavailable in this execution shell** (Windows Git Bash `sandbox`, prior phase's STATE.md notes it was installed via `winget` for interactive dev use but is not on this shell's `PATH`) — ran `sqlc generate` directly followed by `git diff --exit-code -- internal/db/sqlc/`, which is exactly what `make sqlc-check`'s two lines (`sqlc generate` then the same `git diff`) compose to, and separately ran `sqlc version` to confirm `v1.31.1` per the task's `<precondition>`.

## Deviations from Plan

**1. [Rule 3 - Blocking] `internal/db/sqlc/querier.go` included in the sqlc-regeneration commit**
- **Found during:** Task 1, after running `sqlc generate`
- **Issue:** The plan's `files_modified` frontmatter and task 1's `<files>` list named only `queries/artists.sql` and `internal/db/sqlc/artists.sql.go`, but `sqlc generate` also modified `internal/db/sqlc/querier.go` (the query's new leading comment propagates into the `Querier` interface's `UpsertArtist` doc comment).
- **Fix:** Staged and committed `querier.go` in the same commit as `artists.sql.go` — regenerated sqlc output must never be split across commits, which is exactly what `sqlc-check`'s working-tree diff gate enforces.
- **Files modified:** `internal/db/sqlc/querier.go`
- **Commit:** `a9924ae`

No other deviations — both tasks otherwise executed exactly as written, including the COALESCE treatment mirroring `deezer_id`'s existing pattern and the three-independent-assertions structure in task 2's test.

## Issues Encountered

`make` is not on this execution shell's `PATH` (a prior phase noted it was installed via `winget` for interactive dev use, but this non-interactive shell does not see it). Ran the `sqlc-check` and `sqlc` Makefile targets' underlying commands directly (`sqlc generate`, `git diff --exit-code -- internal/db/sqlc/`) instead, which is behaviorally identical. `docker ps` confirmed `drop-tracker-postgres-1` was already running and `sqlc version` reported `v1.31.1`, so the task's `<precondition>` was met without any setup step.

## Next Phase Readiness

- Gap `G-02-2a` is closed: both `must_haves.truths` conditions (refresh on supplied value, preserve on omitted value) are covered by real-Postgres tests, and the query source and its committed codegen are in sync
- Gap `G-02-2b` (WR-02: `UpdatePreferences` not-found race and lost-update race) remains open — out of this plan's scope, tracked separately per `02-UAT.md`
- Phase 3 (External Clients & Search) and Phase 6 (Frontend) can now re-submit refreshed artist metadata for an existing mbid without the table silently discarding half of it

---
*Phase: 02-watchlist-core*
*Completed: 2026-08-06*
