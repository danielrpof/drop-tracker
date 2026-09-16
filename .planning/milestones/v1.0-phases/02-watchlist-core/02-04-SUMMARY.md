---
phase: 02-watchlist-core
plan: 04
subsystem: watchlist-api
tags: [postgres, pgx, sqlc, chi, watchlist, constraints]

requires:
  - phase: 02-watchlist-core (plan 01)
    provides: internal/watchlist.Store/Service interface (four-method contract), artists/watchlist schema, POST /watchlist tracer
  - phase: 02-watchlist-core (plan 02)
    provides: normalizeSet allow-list validation, ReleaseTypes/EventTypes allow-lists, ErrInvalidReleaseType/ErrInvalidEventType sentinels
  - phase: 02-watchlist-core (plan 03)
    provides: parseWatchlistID shared id-parsing helper, ListWatchlist joined query, GET/DELETE routes
provides:
  - "PATCH /watchlist/{id} -- independent partial-update semantics for release_types and muted_event_types (WLST-05, WLST-06, D-11)"
  - "watchlist.Service.UpdatePreferences -- the fourth and final Store method, closing the interface-first placeholder gate"
  - "UpdateWatchlistPreferences sqlc query -- the phase's fifth and final query, keeping the query surface exactly as scoped"
  - "TestWatchlist_FullLifecycle -- one integration test demonstrating WLST-02 through WLST-06 against a live server"
  - "Raw-SQL proof that watchlist_release_types_valid / watchlist_muted_event_types_valid CHECK constraints hold independent of the Go validation layer"
affects: []

actuals:
  tokens: 12454
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Partial-update via *[]string request/params fields resolved against a full-row read (ListWatchlist filtered by id) rather than a conditional SQL UPDATE -- the SQL always writes both columns; the three-way absent/empty/populated distinction lives entirely in Go pointer semantics"
    - "Reusing an existing list query for a single-row read (no dedicated GetWatchlistEntry query) to keep the phase's sqlc query surface at exactly five"

key-files:
  created: []
  modified:
    - queries/watchlist.sql
    - internal/db/sqlc/watchlist.sql.go
    - internal/db/sqlc/querier.go
    - internal/watchlist/service.go
    - internal/watchlist/service_test.go
    - internal/httpserver/watchlist.go
    - internal/httpserver/watchlist_test.go
    - internal/httpserver/server.go
    - .planning/phases/02-watchlist-core/02-VALIDATION.md

key-decisions:
  - "UpdatePreferences validates both axes first (before any database read), then reads the current row via the existing ListWatchlist query filtered by id in Go -- no dedicated single-row query was added, keeping the phase's sqlc query surface at exactly five (four prior + UpdateWatchlistPreferences), per the plan's explicit instruction"
  - "The errNotImplemented sentinel and its declaration were removed as part of this plan's GREEN commit (not deferred to task 2's placeholder-gate check) since replacing UpdatePreferences's body naturally eliminates the last caller -- task 2's grep-based gate confirms zero remaining references rather than performing the removal itself"
  - "TestWatchlist_FullLifecycle reuses the existing watchlistEntryBody/errorBody decode-by-field-name pattern from the add/list/delete tests rather than introducing a new response DTO, keeping one canonical decode shape across the whole handler test file"

patterns-established:
  - "Pointer-to-slice PreferencesParams fields (*[]string) carry the three-way absent/empty/populated distinction through the full Add-through-UpdatePreferences preferences pipeline -- Add already used *[]string for the same reason (plan 02-02), and this plan extends the pattern to updates"

requirements-completed: [WLST-05, WLST-06]

coverage:
  - id: D1
    description: "PATCH /watchlist/{id} sets either preference axis independently while the other axis is carried forward untouched, and a second call touching only the other axis does not reset the first"
    requirement: "WLST-05"
    verification:
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_SetsReleaseTypes"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_SetsMutedEventTypes"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_AxesAreIndependent"
        status: pass
    human_judgment: false
  - id: D2
    description: "An explicitly empty array is stored and returned as [] and is distinguishable from an omitted key across two separate calls"
    requirement: "WLST-05"
    verification:
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_EmptyArrayIsNotOmission"
        status: pass
    human_judgment: false
  - id: D3
    description: "Duplicate submitted values are collapsed and both arrays always come back in canonical allow-list order regardless of submission order"
    requirement: "WLST-05"
    verification:
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_DeduplicatesAndCanonicalises"
        status: pass
    human_judgment: false
  - id: D4
    description: "Out-of-allow-list release/event type values return the matching sentinel with the stored row byte-for-byte unchanged, and an unknown watchlist id returns ErrNotFound"
    requirement: "WLST-06"
    verification:
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_RejectsUnknownValue"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_UnknownIDReturnsErrNotFound"
        status: pass
    human_judgment: false
  - id: D5
    description: "PATCH /watchlist/{id} handler surface: 200 with the full updated entry, 400 naming an invalid value, 404 for a missing id, 400 for a malformed id (store never called), 400 for an over-posted body"
    requirement: "WLST-05, WLST-06"
    verification:
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_Returns200WithUpdatedEntry"
        status: pass
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_InvalidValueReturns400"
        status: pass
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_MissingReturns404"
        status: pass
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_BadIDReturns400"
        status: pass
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_RejectsUnknownFields"
        status: pass
    human_judgment: false
  - id: D6
    description: "The database CHECK constraints reject out-of-allow-list values written by raw SQL that bypasses the Go layer entirely, and accept an empty set / the full allow-list on both axes"
    requirement: "WLST-05, WLST-06"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestCheckConstraint_RejectsUnknownReleaseType"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestCheckConstraint_RejectsUnknownEventType"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestCheckConstraint_AcceptsEmptyArrays"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestCheckConstraint_AcceptsFullAllowLists"
        status: pass
    human_judgment: false
  - id: D7
    description: "One integration test demonstrates all four /watchlist routes and all five phase requirements (WLST-02 through WLST-06) against a single live server"
    requirement: "WLST-02, WLST-03, WLST-04, WLST-05, WLST-06"
    verification:
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_FullLifecycle"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-08-06
status: complete
---

# Phase 2 Plan 04: PATCH /watchlist/{id} Preferences Update Summary

`PATCH /watchlist/{id}` now sets either preference axis independently with partial-update semantics -- a nil pointer means untouched, an explicit empty array is stored and returned as `[]`, duplicate values collapse, and every response comes back in canonical allow-list order -- while the schema's `CHECK` constraints are proven (via raw SQL that bypasses the Go layer) to reject an out-of-allow-list value regardless of which code path writes it. This closes the phase: all four `/watchlist` routes are registered, no interface-first placeholder remains in `internal/watchlist`, and one lifecycle test demonstrates WLST-02 through WLST-06 against a live server.

## Performance

- **Duration:** ~40 min
- **Completed:** 2026-08-06
- **Tasks:** 2
- **Files modified:** 9 (0 created, 9 modified)

## Accomplishments

- `queries/watchlist.sql` gained its fifth and final query, `UpdateWatchlistPreferences :one`, which always writes both `release_types` and `muted_event_types` -- the partial-update semantics live entirely in Go, which reads the current row first via the existing `ListWatchlist` query (filtered by id) and substitutes only the axis the caller actually sent
- `Service.UpdatePreferences` validates each supplied axis independently through `normalizeSet` before touching the database, so a rejected request never leaves a partially-applied row, and resolves the untouched axis from the row just read -- the two axes are never allowed to influence each other (D-05)
- `handleUpdateWatchlist` adds `updateWatchlistRequest` (`release_types`/`muted_event_types` only, `DisallowUnknownFields` rejects any attempt to set `id`/`artist_id`/`mbid`/`name`/timestamps -- T-02-11), rejects a body supplying neither key with 400, and registers `PATCH /watchlist/{id}` -- completing the four-route D-11 surface (`POST`, `GET`, `PATCH`, `DELETE`, all flat, no `/api` prefix)
- Four raw-SQL tests prove `watchlist_release_types_valid` / `watchlist_muted_event_types_valid` reject out-of-allow-list values (asserting the exact `ConstraintName`, not just "any error") and accept both an empty set and the full allow-list, independent of any Go code path
- `TestWatchlist_FullLifecycle` walks add -> list -> narrow release types -> mute an event category -> reject duplicate (409) -> delete (204) -> confirm gone -> reject repeat delete (404) -> re-add with a fresh id, against one live `httptest.Server` and real Postgres -- demonstrating all five phase requirements in one test
- The `errNotImplemented` placeholder sentinel is gone: all four `Store` methods (`Add`, `List`, `UpdatePreferences`, `Remove`) are now backed by real queries
- `.planning/phases/02-watchlist-core/02-VALIDATION.md` is complete: all seven per-task verification rows show a real automated command against a file that exists, `wave_0_complete: true`, `nyquist_compliant: true`

## Task Commits

Task 1 followed the RED/GREEN TDD cycle (the plan's `tdd="true"` requirement); task 2's tests were included in the same RED commit since both tasks' `<behavior>`/test blocks were drafted in one pass, matching the precedent plan 02-03 set:

1. **Task 1 (RED) + Task 2 tests:** `d2ef528` (test) -- failing tests for `UpdatePreferences` (7 tests across both preference axes), the four raw-SQL CHECK-constraint backstop tests, the five PATCH handler tests, and `TestWatchlist_FullLifecycle`
2. **Task 1 (GREEN): PATCH /watchlist/{id}** -- `0b0cc86` (feat) -- `UpdateWatchlistPreferences` query, `Service.UpdatePreferences`, `updateWatchlistRequest`/`handleUpdateWatchlist`, route registration, `errNotImplemented` sentinel removed
3. **Task 2: validation record** -- `a2fcfa2` (docs) -- `02-VALIDATION.md` completed (`wave_0_complete: true`, `nyquist_compliant: true`, all rows filled)

The four CHECK-constraint backstop tests and `TestWatchlist_FullLifecycle` (task 2's stated deliverables) were already passing at the RED commit, since they exercise pre-existing behavior (raw SQL against the schema, and the already-implemented Add/List/Delete routes) rather than the new `UpdatePreferences` code path -- task 2 had no separate GREEN commit as a result; its remaining work was the validation-record update.

## Files Created/Modified

- `queries/watchlist.sql` -- added `UpdateWatchlistPreferences :one`, the phase's fifth and final query
- `internal/db/sqlc/watchlist.sql.go`, `internal/db/sqlc/querier.go` -- regenerated (`sqlc generate`, committed clean per `sqlc-check`)
- `internal/watchlist/service.go` -- `UpdatePreferences` real implementation; `errNotImplemented` sentinel and its declaration removed
- `internal/watchlist/service_test.go` -- 7 real-Postgres tests for `UpdatePreferences` (independent axes, empty-vs-omitted, dedup/canonicalisation, unknown-value rejection, unknown id) + 4 raw-SQL CHECK-constraint backstop tests
- `internal/httpserver/watchlist.go` -- `updateWatchlistRequest`, `handleUpdateWatchlist`
- `internal/httpserver/watchlist_test.go` -- 5 unit tests for PATCH's 200/400/404/400/400 branches + `TestWatchlist_FullLifecycle`
- `internal/httpserver/server.go` -- registered `PATCH /watchlist/{id}`
- `.planning/phases/02-watchlist-core/02-VALIDATION.md` -- Per-Task Verification Map completed for all four plans, Wave 0 checklist ticked, frontmatter set to `wave_0_complete: true` / `nyquist_compliant: true`

## Decisions Made

- **No dedicated single-row read query:** `UpdatePreferences` reads the current row via the existing `ListWatchlist` query (already proven by plan 02-03) filtered by id in Go, rather than adding a `GetWatchlistEntry` query -- keeps the phase's sqlc query surface at exactly five, per the plan's explicit instruction. This is an O(n) scan over the full watchlist per PATCH call rather than an indexed single-row lookup; acceptable at this phase's scale (a single-user watchlist, no pagination requirement yet) and flagged here in case a later phase's growth makes it worth revisiting.
- **`errNotImplemented` removal folded into the GREEN commit:** the plan's task 2 "placeholder gate" describes confirming the sentinel is gone and removing it "if the compiler has not already flagged it" -- since replacing `UpdatePreferences`'s body in task 1 naturally removes the last reference, the removal itself happened in task 1's commit; task 2's contribution was the `grep -c 'errNotImplemented'` verification (0), not a second removal step.
- **`TestWatchlist_FullLifecycle` reuses `watchlistEntryBody`:** rather than introducing a lifecycle-specific response type, the test decodes every response with the same field-name-based DTO the add/list/delete tests already use, keeping one canonical decode shape in the file.

## Deviations from Plan

None functionally -- both tasks were built to the plan's `<action>` and `<behavior>` specs exactly, including the query staying total (not conditional) and the `PreferencesParams` pointer semantics carrying the absent/empty/populated three-way distinction. One process-level note (see Decisions Made): task 2's four CHECK-constraint tests and `TestWatchlist_FullLifecycle` were written and already passing in the task 1 RED commit (they don't depend on the new `UpdatePreferences` code path), so task 2 had no independent RED/GREEN split of its own -- its remaining, separately-committed work was the `02-VALIDATION.md` update.

## Issues Encountered

None. Postgres fixture was already running (`docker compose ps` confirmed healthy) and `sqlc version` matched the pinned `v1.31.1` before starting, so this plan's `<precondition>` checks passed without any setup step. Manual curl verification against a freshly built binary on port 8099 confirmed all six acceptance-criteria curl commands (dedup+narrow, axis-independence, invalid-value 400, unknown-id 404) exactly as specified.

## Human-Verification Note

Task 2's `<human-check>` verify item ("exercise the four routes with curl end to end... confirm the JSON bodies read the way you would want a future React client to consume them, and that no error response ever contains database internals") was **deferred to end-of-phase UAT** per this project's `human_verify_mode: end-of-phase` config setting, matching the convention plan 02-01's executor followed. All of that item's automated-equivalent checks were run and passed during this execution (see Issues Encountered); the remaining subjective judgment (JSON body ergonomics for a future React client) is left for the phase-level UAT pass.

## Next Phase Readiness

- The phase's four-route `/watchlist` D-11 surface is complete: `POST`, `GET`, `PATCH`, `DELETE`, all flat with no `/api` prefix
- `internal/watchlist.Service` has zero remaining interface-first placeholders -- all four `Store` methods are real
- `.planning/phases/02-watchlist-core/02-VALIDATION.md` is `nyquist_compliant: true` and ready for `/gsd-verify-work`
- Phase 3 (External Clients & Search) can build against a fully-functional watchlist API: it will call `POST /watchlist` with real MusicBrainz/Deezer search results in place of this phase's client-supplied mbid+name, and nothing in this plan's implementation assumes the add path's input is hand-typed

---
*Phase: 02-watchlist-core*
*Completed: 2026-08-06*

## Self-Check: PASSED

- `queries/watchlist.sql` -- FOUND
- `internal/watchlist/service.go` -- FOUND
- `internal/watchlist/service_test.go` -- FOUND
- `internal/httpserver/watchlist.go` -- FOUND
- `internal/httpserver/watchlist_test.go` -- FOUND
- `internal/httpserver/server.go` -- FOUND
- `.planning/phases/02-watchlist-core/02-VALIDATION.md` -- FOUND
- Commit `d2ef528` -- FOUND in `git log --oneline --all`
- Commit `0b0cc86` -- FOUND in `git log --oneline --all`
- Commit `a2fcfa2` -- FOUND in `git log --oneline --all`
