---
phase: 26-bulk-edit-remove
plan: 05
subsystem: api
tags: [go, chi, sqlc, postgres, bulk-actions, preferences, locking]

requires:
  - phase: 26-bulk-edit-remove
    provides: "WatchlistBulkStore seam, validateBulkIDs / writeBulkIDsError / maxBulkBodyBytes, bulkSkipWire, missingIDs, newBulkServer / fakeBulkStore"
  - phase: 02-watchlist
    provides: "ReleaseTypes / EventTypes allow-lists, normalizeSet, domainError / writeDomainError"
provides:
  - "PATCH /watchlist/bulk/preferences: per-type set/unset delta applied to many entries in one statement; outcome {applied, skipped:[{id, reason:gone}]}"
  - "watchlist.Service.BulkUpdatePreferences, AxisDelta, PreferencesDelta, BulkUpdateResult, ErrPreferenceConflict"
  - "WatchlistBulkStore.BulkUpdatePreferences, bulkOutcomeResponse and skipReasonGone reused by later bulk routes"
affects: [26-06, 26-07, 26-10]

actuals:
  tokens: 14000
  tasks: 3
  commits: 3
plan_head_before: 6bf4540ba35b74ebd044a68a15c9152c004c9401

tech-stack:
  added: []
  patterns:
    - "MATERIALIZED lock CTE ORDER BY id FOR UPDATE, then rebuild each array in canonical order and update only IS DISTINCT FROM rows"
    - "Every array parameter is COALESCEd to '{}' in SQL and built from normalizeSet's non-nil output in Go"

key-files:
  created: []
  modified:
    - queries/bulk.sql
    - internal/db/sqlc/bulk.sql.go
    - internal/db/sqlc/querier.go
    - internal/watchlist/bulk.go
    - internal/watchlist/bulk_test.go
    - internal/httpserver/bulk.go
    - internal/httpserver/bulk_test.go
    - internal/httpserver/server.go

key-decisions:
  - "Verb is PATCH and an omitted axis object means empty lists on that axis (RESEARCH Open Question 3)"
  - "Validation (invalid type, empty delta, set/unset conflict) runs before the empty-ids shortcut so a bad delta is a 400 regardless of ids"
  - "watchlist.Store gains no method; the delta lives on the bulk seam only (D-20)"

patterns-established:
  - "Bulk outcome wire: bulkOutcomeResponse{applied, skipped} with gone skips carrying no name"

requirements-completed: [BULK-03]

coverage:
  - id: D1
    description: "One PATCH applies a per-type delta to every existing selected artist in canonical order, skips a gone id, and leaves untouched types and unselected artists alone"
    requirement: BULK-03
    verification:
      - kind: integration
        ref: "internal/httpserver/bulk_test.go#TestBulk_PreferencesEndToEnd"
        status: pass
      - kind: integration
        ref: "internal/watchlist/bulk_test.go#TestService_BulkUpdatePreferences"
        status: pass
    human_judgment: false
  - id: D2
    description: "A nil set/unset slice never wipes release types; an empty resulting release set is allowed; notes are untouched; no-op rows keep updated_at"
    requirement: BULK-03
    verification:
      - kind: integration
        ref: "internal/watchlist/bulk_test.go#TestService_BulkUpdatePreferences_NilSlicesNeverWipe"
        status: pass
      - kind: integration
        ref: "internal/watchlist/bulk_test.go#TestService_BulkUpdatePreferences"
        status: pass
    human_judgment: false
  - id: D3
    description: "Invalid types, set/unset conflicts and empty deltas answer 400 with their D-21 codes; an over-posted note, the old full-set shape and unknown nested keys answer invalid request body with zero store calls"
    requirement: BULK-03
    verification:
      - kind: integration
        ref: "internal/watchlist/bulk_test.go#TestService_BulkUpdatePreferences_Validation"
        status: pass
      - kind: integration
        ref: "internal/httpserver/bulk_test.go#TestBulk_PreferencesValidation"
        status: pass
    human_judgment: false
  - id: D4
    description: "A concurrent single-axis write is not lost and a concurrently deleted row becomes gone"
    requirement: BULK-03
    verification:
      - kind: integration
        ref: "internal/watchlist/bulk_test.go#TestService_BulkUpdatePreferences_ConcurrentAxisWriteIsNotLost"
        status: pass
      - kind: integration
        ref: "internal/watchlist/bulk_test.go#TestService_BulkUpdatePreferences_ConcurrentDeleteIsGone"
        status: pass
    human_judgment: false
  - id: D5
    description: "The route is gated (401/403, store never called), 503 without the option, reaches the bulk handler not PATCH /watchlist/{id}, and encodes the skip wire shape"
    requirement: BULK-03
    verification:
      - kind: unit
        ref: "internal/httpserver/bulk_test.go#TestBulk_PreferencesGated"
        status: pass
      - kind: unit
        ref: "internal/httpserver/bulk_test.go#TestBulk_PreferencesUnavailableWithoutOption"
        status: pass
      - kind: unit
        ref: "internal/httpserver/bulk_test.go#TestBulk_PreferencesReachesBulkHandler"
        status: pass
      - kind: unit
        ref: "internal/httpserver/bulk_test.go#TestBulk_PreferencesWireShape"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 05: Bulk preferences server half Summary

**PATCH /watchlist/bulk/preferences applies a per-type set/unset delta to many artists in one locking statement, rebuilding each array in canonical order so untouched types, notes and unchanged rows are never disturbed.**

## Performance

- **Duration:** ~12 min
- **Completed:** 2026-10-16
- **Tasks:** 3 (1 tracer, 2 TDD-flagged test tasks)
- **Files modified:** 8

## Accomplishments

- `BulkUpdateWatchlistPreferences :many` in `queries/bulk.sql` taken from RESEARCH Example 4: `MATERIALIZED` lock CTE, canonical-order rebuild via `unnest ... WITH ORDINALITY`, `IS DISTINCT FROM` update guard, four `COALESCE(...::text[], '{}')` wrappers. sqlc generated the seven-field params struct.
- `Service.BulkUpdatePreferences` normalizes all four lists through `normalizeSet`, rejects empty deltas (`ErrNoPreferencesSupplied`) and set/unset overlap (`ErrPreferenceConflict`) before any database call, and derives `Gone` through `missingIDs`.
- `handleBulkPreferences` with a note-free DTO, `bulkPreferenceErrors` mapping to `bulk_no_preferences`, `bulk_preference_conflict`, `invalid_release_type`, `invalid_event_type`, and the route registered in `registerDataRoutes` so it inherits the gate and CSRF check.
- Tests pin the delta semantics, canonical order, no-op rows, the empty release set, the nil-slice guard, validation, both held-lock concurrency cases, and the HTTP contract.

## Task Commits

1. **Task 1: Apply a per-type preference delta to a mixed selection over HTTP (tracer)** - `034b281` (feat)
2. **Task 2: Pin the delta semantics, the nil-slice trap, validation, and concurrency** - `0aef309` (test)
3. **Task 3: Pin the bulk preferences HTTP contract** - `697ae05` (test)

## Verification

- `go build ./...`, `go vet ./...`, `golangci-lint run` (0 issues); `sqlc generate` leaves `internal/db/sqlc/` identical to the committed files.
- Full suite `go test ./...` passes against real Postgres; backend coverage 90.61% (gate 80%).
- Acceptance greps: 4 `COALESCE(@`, 1 `AS MATERIALIZED`, 2 `IS DISTINCT FROM`, 1 route registration, 0 `json:"note` in the request DTO, `Store` still 5 `ctx` methods, 5 `TestService_BulkUpdatePreferences*` funcs, 6 `TestBulk_Preferences*` funcs.

## Decisions Made

- Used the RESEARCH Example 4 SQL verbatim; PATCH verb and omitted-axis-means-empty as settled in the plan.
- Validation precedes the empty-ids shortcut, so `{"ids":[]}` with a bad delta is still a delta error at the service layer (the handler rejects empty ids first with `bulk_ids_required`).

## Deviations from Plan

None - plan executed exactly as written.

TDD note: Tasks 2 and 3 carry `tdd="true"` but their behavior shipped in the Task 1 tracer, so the tests passed on first run and no separate RED commit was possible; no production change was needed after Task 1.

## Issues Encountered

- `go test -race` is unavailable on this Windows box (no cgo); CI's Linux job is the race gate.
- `make` is not installed; the Makefile target bodies (`sqlc-check`, tests, `coverage-gate`) were run directly.
- The first attempt to append the Task 2 tests with a long inline heredoc failed in the shell before writing anything; the tests were appended from a scratch file instead.
- `COALESCE` in SQL is a second layer behind Go's non-nil `normalizeSet` output, so the nil-slice test passes nil at the service boundary but cannot reach SQL as NULL from Go; the guard is covered by construction and by the omitted-`unset` end-to-end case.

## Known Stubs

None.

## Threat Flags

None. The new route is the one in the plan's threat model (T-26-19 to T-26-24), covered by the tests above.

## Next Phase Readiness

Ready for the SPA plans. `PATCH /watchlist/bulk/preferences` returns `{applied, skipped}` and the 400 codes above for the UI to surface; `bulkOutcomeResponse` and `skipReasonGone` are available to 26-07.

## Self-Check: PASSED

Modified files exist; commits `034b281`, `0aef309`, `697ae05` exist.
