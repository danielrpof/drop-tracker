---
phase: 25-find-filter-watchlist-and-history
plan: 02
subsystem: api
tags: [go, chi, sqlc, postgres, events, tags, retention]

requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "25-01 sqlc regeneration (serialized to keep sqlc-check clean)"
provides:
  - "GET /events?tag_id=N filtered by an artist_tags EXISTS predicate inside ListEvents and HasOlderEvents"
  - "events.ListParams.TagID passed to both queries from one retention value"
  - "tag_id validation (400 invalid tag_id) before the store is called"
affects: [25-06 History Tag control, listEvents({ tagId }) client]

actuals:
  tokens: 5340
  tasks: 2
  commits: 2
plan_head_before: 66054460b461e37f24f92bad404ff72ef5523624
commits: 2

tech-stack:
  added: []
  patterns:
    - "Optional sqlc.narg filter as an EXISTS semi-join, repeated verbatim in the list query and its has-older companion"

key-files:
  created: []
  modified:
    - queries/events.sql
    - internal/db/sqlc/events.sql.go
    - internal/events/service.go
    - internal/httpserver/events.go
    - internal/httpserver/events_test.go

key-decisions:
  - "Tag predicate keys on events.artist_id via artist_tags, so removed artists (no watchlist row) still match (TAG-07)."
  - "Reused parseOptionalPositiveInt64 for tag_id: absent/empty is no filter, malformed/<1 is 400 invalid tag_id."

patterns-established:
  - "Any new History filter goes into both ListEvents and HasOlderEvents so retention and has_older_events stay scoped together."

requirements-completed: [HIST-02]

coverage:
  - id: D1
    description: "GET /events?tag_id=N lists only events of artists carrying tag N, including artists with no watchlist row"
    requirement: HIST-02
    verification:
      - kind: integration
        ref: "internal/httpserver/events_test.go#TestListEvents_TagFilter"
        status: pass
    human_judgment: false
  - id: D2
    description: "No aged-out event appears under a tag filter on the first page, any cursor page, or in has_older_events"
    requirement: HIST-02
    verification:
      - kind: integration
        ref: "internal/httpserver/events_test.go#TestRetention_TagFilterComposesWithCutoff"
        status: pass
    human_judgment: false
  - id: D3
    description: "tag_id composes with artist_id and event_type; unknown tag id gives an empty page with has_older_events false"
    requirement: HIST-02
    verification:
      - kind: integration
        ref: "internal/httpserver/events_test.go#TestListEvents_TagFilterComposesWithOtherFilters"
        status: pass
    human_judgment: false
  - id: D4
    description: "Malformed tag_id returns 400 invalid tag_id without calling the store; TagID reaches both queries"
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "internal/httpserver/events_test.go#TestHandleListEvents_Validation"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-10-10
status: complete
---

# Phase 25 Plan 02: History tag filter API Summary

**GET /events accepts tag_id via an artist_tags EXISTS predicate placed inside the retention-aware ListEvents and HasOlderEvents queries, so a tag filter can never surface aged-out events.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-10-10T16:12:00Z
- **Completed:** 2026-10-10T16:37:06Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Tag predicate added to both queries; sqlc regenerated with `TagID *int64` on both param structs.
- `events.ListParams.TagID` flows into both queries alongside the single retention cutoff.
- `handleListEvents` parses `tag_id` with the fixed `invalid tag_id` 400 message.
- Tests prove removed-artist matching, retention on the first page and every cursor page, `has_older_events` scoping, AND composition, the unknown-tag empty page, and the validation contract.

## Task Commits

1. **Task 1: Filter GET /events by one tag, end to end** - `c421197` (feat)
2. **Task 2: Retention, paging, composition, and validation under a tag filter** - `00e0ada` (test)

**Plan metadata:** committed separately (docs: complete plan)

_Note: Task 2 is tdd="true", but the Task 1 tracer already shipped the implementation, so its tests passed on first run with no separate RED commit._

## Files Created/Modified
- `queries/events.sql` - `sqlc.narg('tag_id')` EXISTS predicate in ListEvents and HasOlderEvents
- `internal/db/sqlc/events.sql.go` - regenerated; `TagID` on both param structs
- `internal/events/service.go` - `ListParams.TagID`, passed to both queries
- `internal/httpserver/events.go` - `tag_id` parsing and doc comment
- `internal/httpserver/events_test.go` - tag/link helpers, three integration tests, validation cases, `recordingQuerier.gotOlderParams`

## Decisions Made
None - followed plan as specified.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `make` is not on PATH and cgo is disabled, so the Makefile recipes were run directly without `-race`. `sqlc-check` was run as `git add internal/db/sqlc`, `sqlc generate`, `git diff --exit-code` (clean); the sqlc version matches the pinned v1.31.1.
- Coverage gate recipe run directly: backend total 90.25% (floor 80%). Go printed `no such tool "covdata"` for `internal/testutil` (no test files); the profile and total are unaffected.
- `gofmt -l` lists five pre-existing files (`artistart/match_test.go`, `httpserver/server.go`, `settings_test.go`, `notifier/timeout_test.go`, `sqlscan/lex.go`). None were touched by this plan.

## Known Stubs
None.

## Threat Flags
None - the only new surface is the `tag_id` query parameter covered by T-25-05 to T-25-07. It is validated before the store and bound as a sqlc parameter.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `GET /events?tag_id=N` is ready for 25-06's History Tag control (`listEvents({ tagId })`).

## Self-Check: PASSED

- FOUND: queries/events.sql, internal/db/sqlc/events.sql.go, internal/events/service.go, internal/httpserver/events.go, internal/httpserver/events_test.go
- FOUND commits: c421197, 00e0ada

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-10*
