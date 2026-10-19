---
phase: 26-bulk-edit-remove
plan: 01
subsystem: api
tags: [go, chi, sqlc, postgres, bulk-actions]

requires:
  - phase: 24-artist-tags-and-notes
    provides: "WithTags option shape, domainError/writeErrorCode helpers, tags/notes tables"
provides:
  - "POST /watchlist/bulk/delete: one atomic DELETE ... = ANY RETURNING id, outcome {removed, already_gone}"
  - "httpserver.WatchlistBulkStore + WithWatchlistBulk seam (503 when absent), watchlist.Store untouched"
  - "validateBulkIDs / writeBulkIDsError / maxBulkBodyBytes / newBulkServer helpers reused by 26-02, 26-05, 26-07"
  - "watchlist.BulkRemoveResult, Service.BulkRemove, missingIDs"
affects: [26-02, 26-03, 26-05, 26-07]

actuals:
  tokens: 7000
  tasks: 3
  commits: 3
plan_head_before: db223423c069ee43bc4d6f7e79994557d0107ce6

tech-stack:
  added: []
  patterns:
    - "Bulk routes are literal POST paths inside registerDataRoutes: inherit gate and CSRF, outrank /watchlist/{id}"
    - "Single-statement set delete reported via RETURNING; ids not returned are already gone"

key-files:
  created:
    - queries/bulk.sql
    - internal/db/sqlc/bulk.sql.go
    - internal/watchlist/bulk.go
    - internal/watchlist/bulk_test.go
    - internal/httpserver/bulk.go
    - internal/httpserver/bulk_test.go
  modified:
    - internal/db/sqlc/querier.go
    - internal/httpserver/server.go
    - cmd/server/main.go

key-decisions:
  - "Path POST /watchlist/bulk/delete, at most 1000 ids and a 64 KiB body cap; the 400 body carries max so the SPA never hardcodes the bound"
  - "The id bound is checked on the raw array before dedupe so it applies to what was sent"
  - "Bulk remove deletes watchlist rows only: artists, artist_tags and undelivered events survive, the note goes with the row"

patterns-established:
  - "WatchlistBulkStore: narrow per-feature store interface with a With* option, mirroring TagStore/WithTags"

requirements-completed: [BULK-04]

coverage:
  - id: D1
    description: "POST /watchlist/bulk/delete removes the selected rows in one statement and reports removed and already-gone ids"
    requirement: BULK-04
    verification:
      - kind: integration
        ref: "internal/httpserver/bulk_test.go#TestBulk_DeleteEndToEnd"
        status: pass
      - kind: integration
        ref: "internal/watchlist/bulk_test.go#TestService_BulkRemove"
        status: pass
    human_judgment: false
  - id: D2
    description: "Bulk remove keeps the artist row, tag links and pending events; the note goes with the entry"
    requirement: BULK-04
    verification:
      - kind: integration
        ref: "internal/watchlist/bulk_test.go#TestService_BulkRemove_KeepsArtistTagsAndPendingEvents"
        status: pass
    human_judgment: false
  - id: D3
    description: "The route is gated (401/403), 503 without the option, reaches the bulk handler, and enforces id and body bounds"
    requirement: BULK-04
    verification:
      - kind: unit
        ref: "internal/httpserver/bulk_test.go#TestBulk_Gated401NoCookie"
        status: pass
      - kind: unit
        ref: "internal/httpserver/bulk_test.go#TestBulk_GatedForbiddenWithoutCSRFHeader"
        status: pass
      - kind: unit
        ref: "internal/httpserver/bulk_test.go#TestBulk_DeleteIDValidation"
        status: pass
    human_judgment: false
  - id: D4
    description: "Two bulk deletes racing over overlapping ids both succeed with each id in exactly one removed list (backstop truth; a rare 40P01 surfaces as a retryable 500)"
    requirement: BULK-04
    verification: []
    human_judgment: true
    rationale: "Accepted residual risk (RESEARCH A2) with no automated race test; single-user app, plan marks it backstop"

duration: 14min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 01: Bulk delete server half Summary

**POST /watchlist/bulk/delete deletes a bounded, deduped set of watchlist entries in one `DELETE ... = ANY RETURNING id` statement behind the auth gate and CSRF check, reporting already-gone ids and keeping artists, tags and pending events.**

## Performance

- **Duration:** ~14 min
- **Completed:** 2026-10-16
- **Tasks:** 3 (1 tracer, 2 TDD-flagged test tasks)
- **Files modified:** 9 (6 created, 3 modified)

## Accomplishments

- `BulkDeleteWatchlist :many` in `queries/bulk.sql`, generated with sqlc v1.31.1; `Service.BulkRemove` sorts `Removed` and derives `AlreadyGone` through `missingIDs` (no transaction, no database call for empty input).
- `internal/httpserver/bulk.go` adds the `WatchlistBulkStore` seam, `WithWatchlistBulk`, the shared id validator (raw-array bound, ids >= 1, first-occurrence dedupe) and `handleBulkDelete`; the route sits inside `registerDataRoutes` so it inherits `Authenticate` and `RequireCSRFHeader`; `cmd/server/main.go` wires the same `*watchlist.Service`.
- Tests pin survivors, already-gone reporting, idempotency, gate/CSRF, the 503 fallback, literal-path precedence, every id/body bound, empty-array serialization and the 500 no-leak rule.

## Task Commits

1. **Task 1: Bulk-delete three selected entries over HTTP in one statement (tracer)** - `22d2d0f` (feat)
2. **Task 2: Pin what bulk remove deletes, keeps, and reports** - `2bdedc4` (test)
3. **Task 3: Pin the bulk route's gate, fallback, precedence, and bounds** - `aabb51d` (test)

## Verification

- Tracer verify re-run after commit: `TestBulk_DeleteEndToEnd` passes; `go build`, `go vet`, `golangci-lint run` (0 issues), sqlc regenerate shows no diff in `internal/db/sqlc/`.
- Full suite `go test ./...` passes against real Postgres; backend coverage 90.49% (gate 80%).
- All task acceptance greps pass (1/1/1, `Store` still 5 `ctx` methods, route and wiring 1/1, 6 `TestBulk_*` names, 1 interface assertion, 2 `TestService_BulkRemove*`, 5 subtests).

## Decisions Made

- Path, 1000-id bound and 64 KiB body cap as planned (RESEARCH Example 7, A4).
- `watchlist.Store` gained no method; the bulk seam is its own interface (D-20).

## Deviations from Plan

None - plan executed exactly as written.

TDD note: Tasks 2 and 3 carry `tdd="true"` but their implementation shipped in the Task 1 tracer, so their tests passed on first run and no separate RED commit was possible; both are `test(...)` commits, no production change needed.

## Issues Encountered

- `go test -race` is unavailable on this Windows box (`-race requires cgo`); the full suite and coverage run were done without it. CI's Linux job is the race gate.
- `make` is not installed; the Makefile target bodies (`sqlc-check`, `test-integration` without `-race`, `coverage-gate`) were run directly.
- `gofmt -l` lists `internal/httpserver/server.go` (struct field alignment already off before this plan, likely line endings); left as is, `golangci-lint` is clean.

## Known Stubs

None.

## Threat Flags

None. New surface (the bulk route) is the one in the plan's threat model (T-26-01 to T-26-07); T-26-01/02/03/04/05/06 are covered by the tests above, T-26-07 is accepted.

## Next Phase Readiness

Ready for 26-02. `validateBulkIDs`, `writeBulkIDsError`, `maxBulkBodyBytes`, `missingIDs` and `newBulkServer` are in place for the sibling bulk routes.

## Self-Check: PASSED

Created files exist (`queries/bulk.sql`, `internal/db/sqlc/bulk.sql.go`, `internal/watchlist/bulk.go`, `internal/watchlist/bulk_test.go`, `internal/httpserver/bulk.go`, `internal/httpserver/bulk_test.go`); commits `22d2d0f`, `2bdedc4`, `aabb51d` exist.
