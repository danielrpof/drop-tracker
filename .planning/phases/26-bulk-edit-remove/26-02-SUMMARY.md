---
phase: 26-bulk-edit-remove
plan: 02
subsystem: api
tags: [go, chi, sqlc, postgres, bulk-actions, tags, locking]

requires:
  - phase: 26-bulk-edit-remove
    provides: "validateBulkIDs / writeBulkIDsError / maxBulkBodyBytes, queries/bulk.sql, bulk route placement in registerDataRoutes"
  - phase: 24-artist-tags-and-notes
    provides: "tags.Service, GetOrCreateTag, the ADR 0004 cap trigger, TagStore/WithTags"
provides:
  - "POST /watchlist/bulk/tags/add: one transaction, ordered FOR NO KEY UPDATE pre-lock, count-guarded insert; outcome {tag, applied, skipped}"
  - "tags.Service.BulkAttach with SkipReasonCap/SkipReasonGone, BulkSkip, BulkAttachResult"
  - "TagStore.BulkAttach, bulkSkipWire/toSkipWire reused by 26-05 and 26-07"
  - "ADR 0004 bulk-attach amendment"
affects: [26-05, 26-07, 26-08, 26-11]

actuals:
  tokens: 8700
  tasks: 3
  commits: 3
plan_head_before: 1c33bf95294aba59c5ace6dff258773135104f96

tech-stack:
  added: []
  patterns:
    - "Lock artist rows in id order in their own statement, then run the guarded set insert as a second statement so it reads a fresh READ COMMITTED snapshot"
    - "Return without Commit when nothing applied so a tag created on the fly rolls back"

key-files:
  created:
    - internal/tags/bulk.go
    - internal/tags/bulk_test.go
  modified:
    - queries/bulk.sql
    - internal/db/sqlc/bulk.sql.go
    - internal/db/sqlc/querier.go
    - internal/httpserver/bulk.go
    - internal/httpserver/tags.go
    - internal/httpserver/tags_test.go
    - internal/httpserver/server.go
    - docs/adr/0004-per-artist-tag-cap-trigger.md

key-decisions:
  - "Bulk add-tag lives on tags.Service and TagStore; watchlist.Store gains nothing and internal/tags does not import internal/watchlist"
  - "All-capped or all-gone batches roll back and report tag id 0, so no orphan tags row is left (D-19)"
  - "Service dedupes requested ids when computing gone skips, so callers that skip validateBulkIDs still get one skip per id"

patterns-established:
  - "Cap skips carry the artist name; gone skips omit name (omitempty) so the SPA can name only the full artists"

requirements-completed: [BULK-02]

coverage:
  - id: D1
    description: "One request adds a tag to every selected artist with room, treats already-tagged as applied, names at-cap artists as cap skips, and reports gone ids"
    requirement: BULK-02
    verification:
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestBulkTags_AddEndToEnd"
        status: pass
      - kind: integration
        ref: "internal/tags/bulk_test.go#TestService_BulkAttach"
        status: pass
    human_judgment: false
  - id: D2
    description: "All-capped and all-gone batches leave no tags row and report tag id 0"
    requirement: BULK-02
    verification:
      - kind: integration
        ref: "internal/tags/bulk_test.go#TestService_BulkAttach_AllCappedLeavesNoTag"
        status: pass
      - kind: integration
        ref: "internal/tags/bulk_test.go#TestService_BulkAttach_AllGoneLeavesNoTag"
        status: pass
    human_judgment: false
  - id: D3
    description: "Overlapping bulk adds with reversed id lists never deadlock, and an add waiting behind an uncommitted 10th link skips that artist as cap"
    requirement: BULK-02
    verification:
      - kind: integration
        ref: "internal/tags/bulk_test.go#TestService_BulkAttach_ConcurrentOverlappingNoDeadlock"
        status: pass
      - kind: integration
        ref: "internal/tags/bulk_test.go#TestService_BulkAttach_WaitsOnConcurrentTenthLink"
        status: pass
    human_judgment: false
  - id: D4
    description: "The route is gated (401/403, store never called), 503 without WithTags, reaches the bulk handler not /watchlist/{id}/tags, validates ids and body, maps tag errors, and encodes the skip wire shape"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_AddGated401NoCookie"
        status: pass
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_AddGatedForbiddenWithoutCSRFHeader"
        status: pass
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_AddValidation"
        status: pass
      - kind: unit
        ref: "internal/httpserver/tags_test.go#TestBulkTags_AddSkipWireShape"
        status: pass
    human_judgment: false

duration: 14min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 02: Bulk add-tag server half Summary

**POST /watchlist/bulk/tags/add adds one tag to many artists in a single transaction: an ordered `FOR NO KEY UPDATE` pre-lock plus a count-guarded insert keeps it race- and deadlock-free against the 10-tag cap, names every at-cap artist, skips gone ids, and rolls back so a new tag is never orphaned.**

## Performance

- **Duration:** ~14 min
- **Completed:** 2026-10-16
- **Tasks:** 3 (1 tracer, 2 TDD-flagged test/doc tasks)
- **Files modified:** 10 (2 created, 8 modified)

## Accomplishments

- `LockWatchlistArtistsForBulkTag` and `BulkAttachTag` added to `queries/bulk.sql` from the verified RESEARCH examples; sqlc generated `ArtistID int64`, `HadTag`/`Inserted bool` with no `interface{}`.
- `tags.Service.BulkAttach` runs get-or-create, the ordered lock, then one guarded insert; entries map back through the lock rows. `HadTag || Inserted` is applied, otherwise a named `cap` skip; missing ids are `gone`. Results are sorted by id and an empty `Applied` returns without commit.
- `TagStore.BulkAttach`, wire types (`bulkSkipWire` with `name,omitempty`, never-nil `toSkipWire`) and `handleBulkAddTag` registered next to the 26-01 bulk route inside `registerDataRoutes`.
- ADR 0004 gained a "Phase 26: bulk attach" amendment; front matter and existing sections untouched.
- Tests pin the mixed room/cap/carrier/gone outcome end to end, 9 and 10 link adjacency, idempotency, blank name, empty ids, all-capped and all-gone rollback, four rounds of reversed-order concurrent adds (no 40P01), a forced wait behind a concurrent uncommitted 10th link (`pg_blocking_pids`), and the full HTTP contract.

## Task Commits

1. **Task 1: Bulk-add one tag over HTTP to a mixed selection (tracer)** - `4ab80d9` (feat)
2. **Task 2: Pin the cap race, deadlock, idempotency, and orphan-tag guarantees** - `1b8a8e5` (test)
3. **Task 3: Amend ADR 0004 and pin the bulk add-tag HTTP contract** - `1818a4b` (docs)

## Verification

- `go build ./...`, `go vet ./...`, `golangci-lint run` (0 issues); `sqlc generate` leaves `internal/db/sqlc/` clean after commit.
- Full suite `go test ./...` passes against real Postgres, including the `internal/db` ADR 0004 trigger tests; backend coverage 90.52% (gate 80%).
- Acceptance greps: one `FOR NO KEY UPDATE OF a` outside comments, `ArtistID int64` x2, zero `interface{}`, one route registration, zero `internal/watchlist` imports in `internal/tags/bulk.go`, 5 `TestService_BulkAttach*` funcs, 1 `pg_blocking_pids`, 7 `TestBulkTags_Add*` funcs, 6 `TestService_BulkAttach/` subtests passing.

## Decisions Made

- Followed the plan's SQL verbatim, including the `t.artist_id::bigint` cast (Pitfall B2).
- `BulkAttach` dedupes requested ids itself when computing `gone`, so it stays correct without the handler's dedupe.

## Deviations from Plan

None - plan executed exactly as written.

TDD note: Tasks 2 and 3 carry `tdd="true"` but their behavior shipped in the Task 1 tracer, so the tests passed on first run and no separate RED commit was possible; no production change was needed after Task 1. The Task 3 commit is typed `docs` because it carries the ADR amendment alongside the tests.

## Issues Encountered

- `go test -race` is unavailable on this Windows box (no cgo); CI's Linux job is the race gate.
- `make` is not installed; Makefile target bodies (`test-integration` without `-race`, `coverage-gate`, `sqlc-check`) were run directly.
- A first attempt to append the Task 3 tests via a Python heredoc failed in the shell before writing anything; the tests were appended from a scratch file instead.

## Known Stubs

None.

## Threat Flags

None. The new route is the one in the plan's threat model (T-26-08 to T-26-14); T-26-08 to T-26-12 and T-26-14 are covered by the tests above, T-26-13 is accepted.

## Next Phase Readiness

Ready for 26-03 and later siblings. 26-07 can extend `internal/tags/bulk.go`, `TagStore`, and `fakeTagStore`, and reuse `bulkSkipWire`/`toSkipWire`; 26-08 and 26-11 can call `POST /watchlist/bulk/tags/add` from the SPA.

## Self-Check: PASSED

Created files exist (`internal/tags/bulk.go`, `internal/tags/bulk_test.go`); commits `4ab80d9`, `1b8a8e5`, `1818a4b` exist.
