---
phase: 10-event-retention-window
plan: "02"
subsystem: events-retention
tags: [sql, http-api, react, empty-states, testing]
status: complete
dependency-graph:
  requires:
    - queries/events.sql (ListEvents, HasAnyEvent idiom)
    - internal/events/service.go (Service, Page, List — widened by 10-01 with retentionDays/cutoff)
    - internal/httpserver/events.go (eventsResponse, handleListEvents)
    - web/app/lib/api.ts (EventsPage)
    - web/app/routes/history.tsx (isFiltered branch, EmptyState calls)
    - web/app/components/common/EmptyState.tsx
  provides:
    - HasOlderEvents sqlc query + Querier.HasOlderEvents + HasOlderEventsParams
    - Page.HasOlderEvents / eventsResponse.HasOlderEvents / EventsPage.has_older_events wire field
    - history.tsx's third empty-state branch (retention-hidden feed)
  affects:
    - internal/httpserver/events_test.go (recordingQuerier gained a required stub method)
    - web/app/lib/api.test.ts, web/app/routes/history.test.tsx (all EventsPage literals widened)
tech-stack:
  added: []
  patterns:
    - HasOlderEvents mirrors ListEvents' two optional sqlc.narg filters exactly but omits the cursor/pagination-position parameter -- it answers a property of the whole filtered scope, not of one page
    - Page's second store round trip (HasOlderEvents) is computed unconditionally on every List call, not gated on "page came back empty" -- one code path, and the query is a short-circuiting EXISTS so the always-on cost is negligible
    - Frontend three-way empty-state selection extracted into a small emptyStateCopy(hasOlderEvents, isFiltered) helper returning {heading, body}, keeping the locked branch order (retention first) legible instead of a triple-nested ternary
key-files:
  created: []
  modified:
    - queries/events.sql
    - internal/db/sqlc/events.sql.go
    - internal/db/sqlc/querier.go
    - internal/events/service.go
    - internal/httpserver/events.go
    - internal/httpserver/events_test.go
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/routes/history.tsx
    - web/app/routes/history.test.tsx
decisions:
  - "has_older_events wire shape: a plain, non-nullable JSON boolean on the existing GET /events envelope (not a new endpoint, not a count) -- matches D-06's explicit 'exact shape is Claude's discretion' latitude, chosen because it's the minimum surface that answers the frontend's actual question."
  - "10-PATTERNS.md's isFiltered-first branch ordering is explicitly superseded by 10-UI-SPEC.md's hasOlderEvents-first ordering, per the plan's own conflict-resolution section -- confirmed live by temporarily swapping the two checks in emptyStateCopy and observing the new priority test fail, then reverting."
  - "HasOlderEvents query comment avoids the literal word 'cursor' (writes 'pagination-position parameter' instead) so the plan's own grep-based acceptance criterion (no cursor reference inside the HasOlderEvents SQL comment block) passes; the query itself never referenced cursor, only an early draft of its comment did."
metrics:
  duration: ~50m
  completed: 2026-08-13
actuals:
  tokens: 6407
  tasks: 2
  commits: 2
---

# Phase 10 Plan 02: Event Retention Window Summary

Added a server-computed `has_older_events` boolean to `GET /events` and a third `History` empty-state branch keyed off it, so a retention-emptied feed tells the user "your history isn't empty, it's just outside the window" instead of falsely claiming no release activity ever happened or that a filter didn't match.

## What Was Built

**Task 1:** Added a `HasOlderEvents :one` sqlc query to `queries/events.sql`, placed adjacent to `ListEvents`, mirroring its two optional `artist_id`/`event_type` filters exactly but deliberately omitting the pagination-position parameter -- the question is a property of the whole filtered scope, not of the current page, so a "Load more" click must never change the answer. Shaped as `EXISTS(... created_at < cutoff ...)` following `HasAnyEvent`'s existing idiom (no `LIMIT` inside the `EXISTS`), with the strict `<` as the exact complement of `ListEvents`' `>=`, keeping D-04's boundary semantics consistent across both queries. Regenerated sqlc output (`HasOlderEventsParams`, `Queries.HasOlderEvents`, updated `Querier` interface). Widened `events.Page` with a `HasOlderEvents bool` field and `Service.List` to call the new query unconditionally on every request, reusing the same `cutoff` variable `ListEvents` already computes so both queries in one request describe the same instant. Wired `eventsResponse.HasOlderEvents` (`has_older_events` JSON tag) straight from `page.HasOlderEvents`. Added an explicit `HasOlderEvents` stub method to `recordingQuerier` in `events_test.go` -- without it, the pre-existing "limit above the maximum is clamped" subtest would nil-panic through the embedded nil `sqlc.Querier` the moment `Service.List` called the new interface method. Added `TestHandleListEvents_HasOlderEventsSignal` (three named states via a real httptest server + decoded envelope: empty table, only in-window events, at least one aged-out event) and `TestListEvents_HasOlderEventsRespectsFilters` (artist A has an aged-out event, artist B has only in-window events; the `artist_id=B` case is what proves the query applies its own filter rather than answering a table-wide question).

**Task 2:** Added `has_older_events: boolean` (non-nullable) to `EventsPage` in `web/app/lib/api.ts`, then fixed all seven pre-existing `EventsPage`-typed literals across `web/app/routes/history.test.tsx` (five: two `mockResolvedValueOnce`/`mockResolvedValue` calls, two explicit-typed `const ...: EventsPage` literals, two more `mockResolvedValue` empty-state fixtures) and `web/app/lib/api.test.ts` (two: one explicit `EventsPage` literal, one inline `JSON.stringify` fixture) -- each set to `false`, preserving every test's pre-retention intent unchanged. Added a `hasOlderEvents` boolean state to `history.tsx`, threaded exactly like `nextCursor`: reset in the filter-change effect's reset block, set from `page.has_older_events` in both the initial-fetch `.then` and `handleLoadMore`'s `.then`. Extracted the three-way empty-state selection into `emptyStateCopy(hasOlderEvents, isFiltered)`, checking `hasOlderEvents` first per `10-UI-SPEC.md`'s locked ordering (explicitly superseding `10-PATTERNS.md`'s opposite ordering, per the plan's own conflict-resolution section) -- the retention branch renders through the existing `EmptyState` component with no `action`, using the exact locked copy ("Older than your retention window" / "There's release history for this view — it's just outside your retention window. Nothing was deleted."), never naming `EVENT_RETENTION_DAYS` or a day count. Added two new tests: the retention branch renders when the feed is empty and `has_older_events` is true (no filter), and the retention branch wins over the filtered branch when both conditions hold simultaneously -- the second test's negative assertion (`No matching events` must be absent) is what actually pins the priority order, not just "a retention heading is present."

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] Local test Postgres on a non-standard port**
- **Found during:** Task 1, before running any DB-backed test
- **Issue:** `make db-up` failed -- port 5433 (this repo's fixed `docker-compose.yml` mapping, matching `Makefile`'s `TEST_DATABASE_URL` default) was already bound by a sibling worktree's own Postgres container (`agent-a446b5e4e1603cdbd-postgres-1`), a known risk this project's own `docker-compose.yml` comment documents ("binding 5432 on a dev box that already has any other Postgres on it does not fail in a way anyone notices... this cost a full debug session").
- **Fix:** Started a standalone `postgres:16` container scoped to this worktree, published on port 5434 (not 5433), and pointed `TEST_DATABASE_URL` at it for every test run in this session. `docker-compose.yml` itself was never touched.
- **Files modified:** none (environment-only; no committed file changed)
- **Commit:** n/a

**2. [Documentation-only] `HasOlderEvents` query comment reworded to avoid the literal word "cursor"**
- **Found during:** Task 1, acceptance-criteria verification
- **Issue:** The plan's own acceptance criterion asserts the `HasOlderEvents` comment block contains no `cursor` reference (proving the query never took a pagination-position parameter). An early draft of the comment explained the omission using the word "cursor" in prose ("deliberately does NOT take a cursor"), which the literal `grep -c "cursor"` check would itself match -- a self-defeating comment.
- **Fix:** Reworded to "deliberately omits ListEvents' pagination-position parameter," same meaning, no literal match. Regenerated sqlc output after the edit.
- **Verification:** `sed -n '/name: HasOlderEvents/,/^$/p' queries/events.sql | grep -c "cursor"` returns 0.
- **Files modified:** `queries/events.sql`, `internal/db/sqlc/events.sql.go`, `internal/db/sqlc/querier.go`
- **Commit:** 773c269

No other deviations. Every other acceptance criterion (the `created_at < ` count, the four detection-state queries staying `cutoff`-free, `sqlc-check`/`go vet`/`golangci-lint` clean, all seven frontend literals fixed, exact copy strings present, branch-order priority test, zero `dangerouslySetInnerHTML` matches) was verified exactly as written.

## Verification Results

- `go build ./...`: clean
- `go vet ./...`: clean
- `golangci-lint run` scoped to touched packages (`./internal/events/...`, `./internal/httpserver/...`, `./internal/db/sqlc/...`): 0 issues. An unscoped repo-wide `golangci-lint run ./...` reproduces the same stale-cached-path pollution from an unrelated sibling worktree already documented in `10-01-SUMMARY.md` ("a separate worktree's stale cached path polluted an unscoped repo-wide run's gosec output") -- confirmed unrelated to this plan's files by inspecting the reported path, which points at a different worktree entirely.
- `make sqlc-check`: exits 0 (generated `internal/db/sqlc/` output matches `queries/events.sql`, verified post-commit)
- `go test ./internal/httpserver/... -run "TestHandleListEvents|TestListEvents" -count=1 -p 1`: PASS, including both new tests and every pre-existing subtest (notably "limit above the maximum is clamped, not rejected," proving `recordingQuerier`'s new stub doesn't nil-panic)
- `go test ./... -count=1 -p 1` (full backend suite, no `-race` -- this Windows dev box's cgo/ThreadSanitizer toolchain break is pre-existing and documented for Phase 01-02/01-03/10-01): all packages PASS, no regressions
- `cd web && pnpm exec tsc --noEmit` (after `pnpm exec react-router typegen`, the step `pnpm run typecheck` performs and the plan's literal `tsc --noEmit` command omits): exits 0
- `cd web && pnpm run test` (full suite, 9 files / 43 tests): PASS; coverage 78.92% statements / 71.57% branches / 76.23% functions / 80.62% lines, all above the 70% frontend floor
- `grep -c "has_older_events" web/app/routes/history.test.tsx`: 9 (>= 7 required)
- Exact-copy checks: `Older than your retention window` and `Nothing was deleted.` each appear exactly once in `history.tsx`
- `grep -rc "dangerouslySetInnerHTML" web/app/`: 0 matches everywhere (Phase 6 invariant preserved)
- Branch-order priority pin: temporarily swapped `emptyStateCopy` to check `isFiltered` before `hasOlderEvents` -- the new "retention wins over filtered" test failed as expected (asserting the retention heading, which no longer rendered); reverted, full suite green again
- `sed -n '/name: HasOlderEvents/,/^$/p' queries/events.sql`: exactly 1 `created_at < ` match, 0 `cursor` matches
- `sed -n '/name: ListExternalIDs/,/name: ListEvents/p' queries/events.sql`: 0 `cutoff` matches -- the four detection-state queries remain unfiltered

## Key Findings for Downstream Plans

- **`has_older_events` wire shape confirmed:** a plain JSON boolean on the existing `GET /events` envelope, alongside `events`/`next_cursor` -- no new endpoint, no count, no exposed day value.
- **Local Postgres port conflicts across sibling worktrees are a live, recurring risk** on this dev machine, not just a documented historical incident -- `docker-compose.yml`'s fixed 5433 mapping collides the moment two worktrees' `make db-up` run concurrently. Each worktree needing its own DB port going forward should expect to work around this the same way (standalone container, explicit `TEST_DATABASE_URL` override) rather than assume `make db-up` will succeed.

## Self-Check: PASSED

- FOUND: queries/events.sql (HasOlderEvents query, ListEvents unchanged elsewhere)
- FOUND: internal/db/sqlc/events.sql.go (HasOlderEventsParams, Queries.HasOlderEvents)
- FOUND: internal/db/sqlc/querier.go (Querier.HasOlderEvents)
- FOUND: internal/events/service.go (Page.HasOlderEvents, Service.List wiring)
- FOUND: internal/httpserver/events.go (eventsResponse.HasOlderEvents / has_older_events)
- FOUND: internal/httpserver/events_test.go (recordingQuerier.HasOlderEvents stub, two new test funcs)
- FOUND: web/app/lib/api.ts (EventsPage.has_older_events)
- FOUND: web/app/lib/api.test.ts (two fixed literals)
- FOUND: web/app/routes/history.tsx (hasOlderEvents state, emptyStateCopy helper, third branch)
- FOUND: web/app/routes/history.test.tsx (five fixed literals, two new tests)
- FOUND commit 773c269 (Task 1)
- FOUND commit d34ed3f (Task 2)
