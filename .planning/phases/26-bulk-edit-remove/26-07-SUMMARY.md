---
phase: 26-bulk-edit-remove
plan: 07
subsystem: api
tags: [go, chi, sqlc, postgres, bulk-actions, tags]

requires:
  - phase: 26-bulk-edit-remove
    provides: "validateBulkIDs / writeBulkIDsError / bulkSkipWire, queries/bulk.sql, TagStore.BulkAttach, bulk route block"
provides:
  - "POST /watchlist/bulk/tags/remove: one DELETE scoped to the selected entries' artists; outcome {applied, carried, skipped}"
  - "tags.Service.BulkDetach and BulkDetachResult; shared goneIDs helper also used by BulkAttach"
  - "GetTagByID and BulkDetachTag sqlc statements"
affects: [26-08, 26-11]

actuals:
  tokens: 7500
  tasks: 2
  commits: 2
plan_head_before: a4b81789858a1b41a8eceb063aa0dd52f2e3789f

tech-stack:
  added: []
  patterns:
    - "Single CTE statement (existing + DELETE ... RETURNING) reports which selected entries carried the link; no transaction needed"
    - "Tag existence checked first with GetTagByID so a missing tag is 404 rather than a silent empty success"

key-files:
  created: []
  modified:
    - queries/bulk.sql
    - internal/db/sqlc/bulk.sql.go
    - internal/db/sqlc/querier.go
    - internal/tags/bulk.go
    - internal/tags/bulk_test.go
    - internal/httpserver/bulk.go
    - internal/httpserver/tags.go
    - internal/httpserver/tags_test.go
    - internal/httpserver/server.go

key-decisions:
  - "Bulk remove-tag lives on tags.Service and TagStore; watchlist.Store gains nothing (D-20)"
  - "No transaction: a tag deleted between the existence check and the DELETE yields zero carriers and a harmless 200; one gone earlier is 404 tag_not_found (D-22)"
  - "applied is exactly the entries whose link was deleted, so the client's Undo re-adds only those through the 26-02 add route"

patterns-established:
  - "goneIDs(requested, found) is the single gone-id computation for bulk tag operations"

requirements-completed: [BULK-02]

coverage:
  - id: D1
    description: "One request removes a tag from exactly the selected carriers, reports applied/carried and the gone id, keeps the tag and an unselected carrier's link"
    requirement: BULK-02
    verification:
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestBulkTags_RemoveEndToEnd"
        status: pass
      - kind: integration
        ref: "internal/tags/bulk_test.go#TestService_BulkDetach"
        status: pass
    human_judgment: false
  - id: D2
    description: "Unknown tag id is ErrTagNotFound with every seeded link unchanged; repeat and non-carrier selections apply nothing"
    requirement: BULK-02
    verification:
      - kind: integration
        ref: "internal/tags/bulk_test.go#TestService_BulkDetach_UnknownTag"
        status: pass
    human_judgment: false
  - id: D3
    description: "The route is gated (401/403, store never called), 503 without WithTags, reaches the bulk handler not the single detach, validates tag_id and ids, maps tag errors, and encodes the wire shape"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_RemoveValidation"
        status: pass
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_RemoveGated"
        status: pass
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_RemoveUnavailableWithoutTags"
        status: pass
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_RemoveReachesBulkHandler"
        status: pass
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_RemoveWireShape"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 07: Bulk remove-tag server half Summary

**POST /watchlist/bulk/tags/remove deletes one tag's links on the selected entries' artists in a single CTE statement and reports exactly which entries carried it (`applied`, `carried`), so the client can offer Undo; the tag row and links on unselected artists are untouched, and a missing tag is 404.**

## Performance

- **Duration:** ~12 min
- **Completed:** 2026-10-16
- **Tasks:** 2 (1 tracer, 1 TDD-flagged test task)
- **Files modified:** 9

## Accomplishments

- `GetTagByID` and `BulkDetachTag` appended to `queries/bulk.sql` from RESEARCH Example 3; sqlc generated `BulkDetachTagRow{EntryID int64; Removed bool}`.
- `tags.Service.BulkDetach` resolves the tag (`pgx.ErrNoRows` becomes `ErrTagNotFound`), runs the single statement, and returns ascending `Applied` and `Gone`. The inline gone-id loop in `BulkAttach` was extracted into `goneIDs` and shared.
- `TagStore.BulkDetach`, `handleBulkRemoveTag` (503, bounded body, shared id validation, `tag_id >= 1`, `writeTagError`) and the route inside `registerDataRoutes`.
- Tests pin the mixed carrier / non-carrier / unselected carrier / gone outcome end to end, idempotency, unknown tag, validation, gating, 503, route reach and the wire shape (no `name` key on gone skips, empty arrays never null).

## Task Commits

1. **Task 1: Bulk-remove one tag over HTTP (tracer)** - `e85d0ad` (feat)
2. **Task 2: Pin bulk remove-tag semantics and HTTP contract** - `3580ccd` (test)

## Verification

- `go vet ./...`, `golangci-lint run` (0 issues); `sqlc generate` after `git add internal/db/sqlc` leaves no diff.
- Full `go test ./...` passes against real Postgres; backend coverage 90.72% (gate 80%).
- Acceptance greps: one `GetTagByID :one`, one `BulkDetachTag :many`, one `Removed bool`, one route registration, 2 `TestService_BulkDetach*` funcs, 6 `TestBulkTags_Remove*` funcs.

## Deviations from Plan

None - plan executed exactly as written.

TDD note: Task 2 carries `tdd="true"` but its behavior shipped in the Task 1 tracer, so the tests passed on first run and no separate RED commit was possible; no production change was needed.

## Issues Encountered

- `go test -race` is unavailable on this Windows box (no cgo); CI's Linux job is the race gate. `make` is not installed, so Makefile target bodies were run directly.
- A Python heredoc carrying the large test bodies failed in the shell; the tests were written to scratch files and appended instead.

## Known Stubs

None.

## Threat Flags

None. The new route is the one in the plan's threat model; T-26-28 to T-26-31 are covered by the gating, validation and scoped-delete tests.

## Next Phase Readiness

26-08 and 26-11 can call `POST /watchlist/bulk/tags/remove` and reuse `applied` for Undo through `POST /watchlist/bulk/tags/add`.

## Self-Check: PASSED

Modified files exist; commits `e85d0ad` and `3580ccd` exist.
