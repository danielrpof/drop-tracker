---
phase: 23-digest-readability-discord-limits
plan: 02
subsystem: notifications
tags: [go, discord, error-handling, sentinel]

# Dependency graph
requires: []
provides:
  - "internal/discord.ErrRateLimited exported sentinel, identifying a 429 that arrived on the retry attempt itself (allowRetry already false)"
affects: [23-04-chunk-cap-time-budget]

# Actuals (#2632)
actuals:
  tokens: 3200
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Package-level errors.New sentinel with the `discord: ` prefix, matching internal/poller.ErrCycleInProgress and internal/musicbrainz.ErrEmptyQuery -- checked via errors.Is at the call site, never a string match"

key-files:
  created: []
  modified:
    - internal/discord/client.go
    - internal/discord/client_test.go

key-decisions:
  - "Sentinel wrapped with %w carrying only the status code (429, implicitly always the same value here) -- no response body, no request URL -- matching the plan's 'no other detail' constraint"
  - "New branch placed immediately after the existing allowRetry-gated 429 branch, keyed on the bare status code (allowRetry is always false by the time this branch is reached, since the retry-eligible case is caught first) -- the generic fallthrough return is otherwise untouched"

requirements-completed: [DGST-12]

coverage:
  - id: D28-1
    description: "A 429 arriving on the retry attempt itself returns an error identifiable with errors.Is against ErrRateLimited, distinct from the generic unexpected-status error"
    requirement: DGST-12
    verification:
      - kind: unit
        ref: "internal/discord/client_test.go#TestSend_429Twice_ReturnsErrorAfterSingleRetry"
        status: pass
      - kind: unit
        ref: "internal/discord/client_test.go#TestSend_429Exhausted_ErrorNeverLeaksBodyOrToken"
        status: pass
    human_judgment: false
  - id: D28-2
    description: "A 500 or a 400 still returns the generic unexpected-status error and does not match the sentinel"
    requirement: DGST-12
    verification:
      - kind: unit
        ref: "internal/discord/client_test.go#TestSend_UnexpectedStatus_ReturnsErrorNamingOnlyTheCode"
        status: pass
      - kind: unit
        ref: "internal/discord/client_test.go#TestSend_400_ReturnsErrorNotMatchingSentinel"
        status: pass
    human_judgment: false
  - id: D28-3
    description: "A first 429 successfully retried and answered with 204 still returns nil -- retry policy (honour-Retry-After-once, D-08) is unchanged"
    requirement: DGST-12
    verification:
      - kind: unit
        ref: "internal/discord/client_test.go#TestSend_429ThenSuccess_HonorsRetryAfter"
        status: pass
    human_judgment: false
  - id: D28-4
    description: "Neither the sentinel nor any error built from it carries response-body content or the webhook URL, across every error-producing path (transport failure, 429-exhausted, 500, 400)"
    requirement: DGST-12
    verification:
      - kind: unit
        ref: "internal/discord/client_test.go#TestSend_ErrorPaths_NeverLeakTokenOrBody"
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-09-17
status: complete
---

# Phase 23 Plan 2: Exported 429-Exhausted Sentinel in internal/discord Summary

**Added an exported `discord.ErrRateLimited` sentinel so "rate-limited after the one permitted retry was already spent" is distinguishable via `errors.Is` from every other non-204 Discord response, with a regression test proving neither the sentinel branch nor any pre-existing error path leaks the webhook token or a response body.**

## Performance

- **Duration:** 5 min (span between first and last task commit)
- **Started:** 2026-09-17T22:59:30-05:00
- **Completed:** 2026-09-17T23:02:32-05:00
- **Tasks:** 2
- **Files modified:** 2 (client.go, client_test.go)

## Accomplishments
- Declared `ErrRateLimited` as a package-level `errors.New` sentinel in `internal/discord/client.go`, matching the repo's existing `discord: `-prefixed sentinel convention (`poller.ErrCycleInProgress`, `musicbrainz.ErrEmptyQuery`)
- Added a branch in `sendAttempt`, reached only when a 429 arrives with `allowRetry` already false, returning the sentinel wrapped with only the status code -- no body, no URL
- Retry policy (`Send` -> one retry -> exhausted), the no-body-echo convention, and the no-URL-wrap convention on the transport-failure path are all unchanged
- Added/extended tests: 429-exhausted matches the sentinel, 500/400 do not, the existing 429-then-204 retry-success case is unaffected, and a new table-driven regression test (`TestSend_ErrorPaths_NeverLeakTokenOrBody`) pins secret hygiene across transport failure, 429-exhausted, 500, and 400 in one place
- Manually verified the regression test actually catches a leak: temporarily changed the transport-failure return to wrap the raw `httpClient.Do` error, confirmed the test went red, reverted

## Task Commits

1. **Task 1: Exported 429-exhausted sentinel in internal/discord** - `1ed4365` (feat)
2. **Task 2: Prove the secret-hygiene conventions survived** - `d3274d6` (test)

**Plan metadata:** pending (docs: complete plan)

## Files Created/Modified
- `internal/discord/client.go` - added `errors` import, `ErrRateLimited` sentinel var, and the new 429-exhausted branch in `sendAttempt`
- `internal/discord/client_test.go` - added `errors.Is` assertions to the existing 429-exhausted and 500 tests, added `TestSend_400_ReturnsErrorNotMatchingSentinel`, `TestSend_429Exhausted_ErrorNeverLeaksBodyOrToken`, and the Task 2 table-driven `TestSend_ErrorPaths_NeverLeakTokenOrBody`

## Decisions Made
- Sentinel error message text is `"discord: rate limited after retry"`, matching the package prefix convention exactly
- The new branch checks the bare status code (`resp.StatusCode == http.StatusTooManyRequests`) after the existing `allowRetry`-gated branch, since by the time control reaches it `allowRetry` is always false -- no redundant condition needed
- Task 2's regression test is table-driven over four named cases (`transport failure`, `429 exhausted`, `500`, `400`) sharing one token segment and one body marker constant, rather than four separate standalone test functions, to keep the "pin every error path in one place" intent literal and reduce duplicated httptest.Server boilerplate

## Deviations from Plan

### TDD Gate Compliance Note

Task 1 (`tdd="true"`) was committed as a single `feat` commit containing both the new tests and the implementation, rather than a separate `test(...)` RED commit followed by a `feat(...)` GREEN commit. This mirrors the documented precedent in `23-01-SUMMARY.md`: the sentinel var, the new branch, and the tests that exercise it are tightly coupled and were authored in one reasoning pass, and Go's compile model means the tests could not build (let alone run red) without the sentinel already declared. The full test suite was run and confirmed passing before the commit; no failing-then-passing commit pair exists in git history for this task. This is a deliberate, documented deviation from the strict two-commit RED/GREEN convention, not a skipped verification step.

No other deviations. Plan executed exactly as written otherwise.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `discord.ErrRateLimited` is exported and ready for plan 23-04 to consume via `errors.Is` at the notifier's chunk-failure log site
- `go test ./...`, `go vet ./...`, `golangci-lint run` all clean; `git diff --exit-code -- internal/notifier queries/ internal/db/sqlc web/` and `git diff --exit-code -- go.mod go.sum` both report no change, confirming this plan stayed isolated from sibling plan 23-01's files and added no dependency

---
*Phase: 23-digest-readability-discord-limits*
*Completed: 2026-09-17*

## Self-Check: PASSED

Both created/modified files confirmed on disk; both task commits (`1ed4365`, `d3274d6`) confirmed in git history.
