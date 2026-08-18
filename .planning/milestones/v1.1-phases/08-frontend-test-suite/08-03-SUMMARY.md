---
phase: 08-frontend-test-suite
plan: 03
subsystem: testing
tags: [vitest, react-testing-library, abortcontroller, search]

# Dependency graph
requires:
  - phase: 08-frontend-test-suite (plan 01)
    provides: "Standalone Vitest + jsdom + React Testing Library harness this plan's test file runs against"
provides:
  - "SearchBox.test.tsx: debounced single-call test plus a RED-then-GREEN pair proving supersession cancellation is real at the request level"
  - "searchArtists(query, signal?) widened to accept and forward an AbortSignal"
  - "SearchBox.runSearch threads its per-search controller's signal into searchArtists, closing the folded AbortController bug"
affects: [08-04, 08-05, 09-ci-coverage-gates]

# Actuals (#2632)
actuals:
  tokens: 1384
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "RED-then-GREEN commit pair for a folded bug fix (D-07): test(08-03) commit intentionally leaves the suite red, fix(08-03) commit restores it, diff between the two touches only the two source files"
    - "Cast a mock's recorded call args through unknown[] when asserting on a not-yet-existing parameter during the RED phase, so tsc stays green while the test fails at runtime for the right reason"

key-files:
  created:
    - web/app/components/watchlist/SearchBox.test.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/components/watchlist/SearchBox.tsx

key-decisions:
  - "Used expect.any(AbortSignal) as the primary matcher for the RED/GREEN AbortSignal assertion rather than the plan's suggested aborted/addEventListener fallback -- it worked reliably against this project's installed jsdom/Vitest/Node combination in both RED and GREEN runs, so the fallback was not needed"

patterns-established:
  - "Folded-bug RED/GREEN pairs keep the RED commit's diff to test-file-only and the GREEN commit's diff to source-file-only, verified via git diff --stat between the two commits before moving on"

requirements-completed: [TEST-01, TEST-02]

coverage:
  - id: D1
    description: "Search surface has a passing test proving a keystroke burst collapses into exactly one debounced searchArtists call for the settled query, and the response reaches onResults"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/SearchBox.test.tsx#collapses a keystroke burst into exactly one debounced searchArtists call and forwards the response"
        status: pass
    human_judgment: false
  - id: D2
    description: "A superseded search's stale response never overwrites a newer query's results, and the superseded request is actually cancelled (AbortSignal reaches searchArtists and reports aborted on supersession), not merely discarded at the callback level"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/SearchBox.test.tsx#passes an AbortSignal as the second argument to searchArtists"
        status: pass
      - kind: unit
        ref: "web/app/components/watchlist/SearchBox.test.tsx#aborts the superseded search's signal when a newer keystroke supersedes it"
        status: pass
    human_judgment: false
  - id: D3
    description: "The test mocks web/app/lib/api.ts (bare vi.mock, no factory/passthrough), not raw fetch, and passes with no server running"
    requirement: "TEST-02"
    verification:
      - kind: unit
        ref: "grep -c 'vi.mock(\"~/lib/api\")' web/app/components/watchlist/SearchBox.test.tsx == 1; no importActual/global.fetch/globalThis.fetch anywhere in web/app/**/*.test.tsx"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-08-12
status: complete
---

# Phase 8 Plan 3: Search Surface Test + AbortController Fix Summary

**Added a passing SearchBox debounce/forwarding test and closed the folded AbortController bug via a RED-then-GREEN pair: `searchArtists` now accepts and forwards an `AbortSignal`, so a superseded search is cancelled at the request level instead of only having its resolved value discarded.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 3 (all executed)
- **Files modified:** 3 (1 new test file, 2 source files)

## Accomplishments
- Task 1: `SearchBox.test.tsx` created with a bare `vi.mock("~/lib/api")` and one test proving a three-keystroke burst collapses into exactly one debounced `searchArtists` call for the settled query, with the fixture response forwarded to `onResults`
- Task 2 (RED): Two failing tests appended proving `SearchBox`'s own doc-comment supersession claim was only half true -- `searchArtists` was called with the query alone (no signal), and a superseded call's (nonexistent) recorded signal never reported `aborted`
- Task 3 (GREEN): `searchArtists` widened to `(query, signal?)`, forwarding the signal into `apiFetch`'s `init` object (`apiFetch` itself untouched -- it already forwards `init` to `fetch`); `SearchBox.runSearch` passes `controller.signal` as the second argument, making all three search tests pass

## Task Commits

Each task was committed atomically:

1. **Task 1: Search surface test — one debounced call, response forwarded upward** - `394acad` (test)
2. **Task 2 (RED): failing tests proving the superseded search is never cancelled at the request level** - `43e1b40` (test)
3. **Task 3 (GREEN): thread the AbortSignal from SearchBox through searchArtists into the request** - `14003dd` (fix)

_Note: This plan's frontmatter TDD task (Task 2/3) is itself a RED/GREEN pair distinct from Task 1's ordinary test-first commit -- three commits total, no combined commits._

## Files Created/Modified
- `web/app/components/watchlist/SearchBox.test.tsx` - New test file: one D-05-floor debounce/forwarding test, plus two supersession-cancellation tests (RED-then-GREEN)
- `web/app/lib/api.ts` - `searchArtists` widened to `(query: string, signal?: AbortSignal)`, forwarding `signal` into `apiFetch`'s init object
- `web/app/components/watchlist/SearchBox.tsx` - `runSearch` now passes `controller.signal` as `searchArtists`' second argument

## Decisions Made
- Used `expect.any(AbortSignal)` as the primary constructor-based matcher for both the RED and GREEN assertions rather than falling back to the plan's `aborted`/`addEventListener` duck-typed check -- it behaved reliably (correct RED failure reason, correct GREEN pass) against this project's installed jsdom 30.0.1 / Vitest 4.1.10 / Node combination, so RESEARCH.md's flagged fallback path was not needed.
- To keep `pnpm --dir web typecheck` green during the RED commit (searchArtists' pre-fix signature is single-argument), cast the recorded mock call through `unknown[]` before indexing the not-yet-existing second element, rather than adding an `@ts-expect-error` or loosening the mock's type globally.

## Deviations from Plan

None - plan executed exactly as written. Both auto-mode acceptance-criteria manual checks (temporarily raising `SEARCH_DEBOUNCE_MS` to confirm Task 1's test is debounce-sensitive; temporarily reverting the `SearchBox.tsx` line to confirm Task 3's Test 2 fails again) were run and restored cleanly, leaving no diff.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Search surface now has a passing behavior test (ROADMAP SC2) and the AbortController folded bug (todo `2026-08-11-searchbox-abortcontroller-never-cancels-the-underlying-fetch.md`) is closed at the source level -- the todo file itself was left in place in `.planning/todos/pending/` since archival is outside this plan's file scope; a future phase-close pass can move it.
- `web/app/lib/api.ts`'s other four endpoint wrappers were deliberately left untouched -- only `searchArtists` had a supersession problem.
- 08-04 and 08-05 (remaining phase 08 plans) can proceed independently; this plan's `depends_on: [08-01]` is satisfied and no new shared config was touched.

---
*Phase: 08-frontend-test-suite*
*Completed: 2026-08-12*
