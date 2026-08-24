---
phase: quick-260824-1gy
plan: 01
subsystem: api
tags: [go, http-client, rate-limiting, refactor, tdd]

# Dependency graph
requires:
  - phase: 03-01
    provides: internal/musicbrainz's original doRequest (timeout-wrap + limiter.Wait + cancel-on-close), the pattern this plan extracted
  - phase: 03-02
    provides: internal/deezer's byte-identical copy of the same doRequest pattern
provides:
  - "internal/httpclient package with a tested Do(ctx, req, limiter, httpClient, component) function"
  - "Single seam for rate-limited, timeout-bounded outbound HTTP requests, reusable by any future external API client"
affects: [musicbrainz, deezer, future external-api-client work]

# Actuals (#2632)
actuals:
  tokens: 4093
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared internal/httpclient.Do seam for rate-limited HTTP requests -- new external clients should call this instead of reimplementing timeout-wrap/limiter.Wait/cancel-on-close logic"

key-files:
  created:
    - internal/httpclient/httpclient.go
    - internal/httpclient/httpclient_test.go
  modified:
    - internal/musicbrainz/client.go
    - internal/deezer/client.go

key-decisions:
  - "Did not extract clampLimit -- musicbrainz's and deezer's limit ceilings are independently-documented external API constants that happen to match today (100/25) by coincidence, not by shared contract; extracting it would wrongly couple them"
  - "RED-phase test commit stubs Do as a not-yet-implemented function (rather than leaving the test file alone in a compile-broken state) so the golangci-lint pre-commit hook -- added in phase 07-02, after this project's earlier TDD commits were written -- can typecheck the package at every commit; tests fail on assertions, not on a build error"
  - "Added a #nosec G704 justification comment on httpClient.Do inside the new shared package -- gosec's SSRF taint check flags exported functions taking *http.Request as a parameter since it cannot trace the caller's origin across a package boundary; both callers build req from a hardcoded base URL, an invariant unchanged by this extraction and not flagged in either caller's own prior (now-removed) doRequest"

requirements-completed: [QUICK-260824-1gy]

coverage:
  - id: D1
    description: "internal/httpclient.Do owns the timeout-wrap, limiter-wait, cancel-on-error, and cancel-on-close logic, proven by dedicated tests for all four cases"
    requirement: "QUICK-260824-1gy"
    verification:
      - kind: unit
        ref: "internal/httpclient/httpclient_test.go#TestDo_Success"
        status: pass
      - kind: unit
        ref: "internal/httpclient/httpclient_test.go#TestDo_LimiterWaitErrorOnCancelledContext"
        status: pass
      - kind: unit
        ref: "internal/httpclient/httpclient_test.go#TestDo_TimeoutReturnsWrappedDeadlineExceeded"
        status: pass
      - kind: unit
        ref: "internal/httpclient/httpclient_test.go#TestDo_CloseCancelsTheDerivedContext"
        status: pass
    human_judgment: false
  - id: D2
    description: "internal/musicbrainz and internal/deezer's doRequest both delegate to httpclient.Do, with no duplicate copy of the extracted logic and no test files modified"
    requirement: "QUICK-260824-1gy"
    verification:
      - kind: unit
        ref: "go test ./internal/musicbrainz/... ./internal/deezer/... (all pre-existing tests pass unchanged)"
        status: pass
      - kind: other
        ref: "grep -c cancelReadCloser internal/musicbrainz/client.go internal/deezer/client.go -> 0, 0"
        status: pass
      - kind: other
        ref: "git diff --name-only -- internal/discord -> empty"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-08-24
status: complete
---

# Quick Task 260824-1gy: Extract Shared Rate-Limited HTTP Client Summary

**New `internal/httpclient` package with a tested `Do` function replaces the byte-identical `doRequest` logic that `internal/musicbrainz` and `internal/deezer` each independently implemented.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-08-24
- **Tasks:** 3
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments
- Created `internal/httpclient` with an exported `Do(ctx, req, limiter, httpClient, component)` function owning the timeout-wrap + `limiter.Wait` + cancel-on-error + cancel-on-close body-wrapping logic, following full RED/GREEN TDD gates
- `internal/musicbrainz.Client.doRequest` shrunk from ~45 lines to header-setting + a single delegated call
- `internal/deezer.Client.doRequest` shrunk the same way
- All pre-existing tests in both client packages pass unchanged, now exercising the shared path indirectly
- `internal/discord` confirmed untouched (no rate limiter, out of scope)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create internal/httpclient with the shared Do helper and its test suite**
   - `2bf9720` (test) - RED: failing tests against a stubbed `Do`
   - `fc29b60` (feat) - GREEN: real implementation, all four tests pass
2. **Task 2: Point internal/musicbrainz.Client.doRequest at the shared helper** - `efb8544` (refactor)
3. **Task 3: Point internal/deezer.Client.doRequest at the shared helper** - `7e7058e` (refactor)

_Task 1 is TDD (tdd="true"): RED commit `2bf9720` then GREEN commit `fc29b60`, per the plan-level TDD gate sequence._

## Files Created/Modified
- `internal/httpclient/httpclient.go` - Shared `Do` function + unexported `cancelReadCloser` body wrapper
- `internal/httpclient/httpclient_test.go` - Four tests: success, limiter-wait-error, timeout, Close-cancels-context
- `internal/musicbrainz/client.go` - `doRequest` now sets headers and delegates to `httpclient.Do`; removed local timeout/limiter/cancel logic and `cancelReadCloser`
- `internal/deezer/client.go` - Same change; `fmt` import retained (still used by `APIError.Error()` and `decodeChecked`)

## Decisions Made
- Did not extract `clampLimit` (see planning findings) -- kept duplicated per-package since the two constants are independently-documented external API limits, not a shared contract
- RED-phase commit for Task 1 used a stubbed `Do` (returns a fixed "not yet implemented" error) rather than a compile-broken test file, so the pre-commit `golangci-lint` hook -- which type-checks on every commit -- could still pass while the four tests failed on assertions instead of a build error
- Added a `#nosec G704` justification comment on the `httpClient.Do(req)` call inside the new package (see Deviations below)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Pre-commit `golangci-lint` hook blocks a compile-broken RED-phase test commit**
- **Found during:** Task 1 (RED phase)
- **Issue:** The plan's TDD flow implies committing a failing test before any implementation exists. Since `internal/httpclient` was a brand-new package, a test-only commit referencing an undefined `Do` function fails to compile, and this repo's `golangci-lint` pre-commit hook (added in phase 07-02, after earlier TDD commits in this codebase were written before that hook existed) type-checks every commit and rejects it.
- **Fix:** Added a minimal stubbed `Do` (returns a fixed "not yet implemented" error) alongside the test file in the RED commit, so the package compiles and all four tests fail on assertions rather than a build error -- true TDD RED (tests fail) without violating the pre-commit hook's compile-clean invariant.
- **Files modified:** internal/httpclient/httpclient.go, internal/httpclient/httpclient_test.go
- **Verification:** `go build ./internal/httpclient/...` succeeds; `go test ./internal/httpclient/...` shows all four tests failing on assertions (RED confirmed) before the GREEN commit replaced the stub with the real implementation.
- **Committed in:** `2bf9720` (Task 1 RED commit)

**2. [Rule 1 - Bug/lint finding] gosec G704 (SSRF taint) flagged `httpClient.Do(req)` in the new shared package**
- **Found during:** Task 1 (GREEN phase)
- **Issue:** `golangci-lint run ./internal/httpclient/...` reported a G704 (SSRF via taint analysis) finding on the `httpClient.Do(req)` call. This did not fire in either caller's own prior `doRequest` (confirmed lint-clean before this plan) -- gosec's taint tracker cannot trace `req`'s origin across `Do`'s exported, generic-package boundary the way it could within each caller's own file.
- **Fix:** Added a `#nosec G704` comment directly above the call, explaining that `Do` never builds or mutates `req.URL` itself and only forwards a caller-constructed request; both current callers build `req` from a package-const base URL never derived from external input (the same invariant that already exists undisturbed in `internal/musicbrainz`/`internal/deezer`).
- **Files modified:** internal/httpclient/httpclient.go
- **Verification:** `golangci-lint run ./...` reports 0 issues after the fix; `go test` and `go vet` still pass.
- **Committed in:** `fc29b60` (Task 1 GREEN commit)

---

**Total deviations:** 2 auto-fixed (1 blocking pre-commit-hook constraint, 1 lint finding from the extraction itself)
**Impact on plan:** Both fixes were necessary to land the plan's own required tasks under this repo's existing pre-commit/lint gates. No scope creep -- no behavior changed beyond what the plan specified.

## Issues Encountered
None beyond the two auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/httpclient.Do` is available as the single seam for any future rate-limited external HTTP client this project adds
- No public API changes in `internal/musicbrainz` or `internal/deezer` -- `ArtistSearcher`, `AlbumLister`, `ReleaseGroupLister`, and both `NewClient` signatures are unchanged
- No blockers

---
*Phase: quick-260824-1gy*
*Completed: 2026-08-24*

## Self-Check: PASSED

All created/modified files found on disk; all four task commits (`2bf9720`, `fc29b60`, `efb8544`, `7e7058e`) found in git history.
