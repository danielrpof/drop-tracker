---
phase: 09-ci-coverage-gates
plan: 04
subsystem: testing
tags: [vitest, coverage-v8, react-testing-library, react-router, frontend-testing]

# Dependency graph
requires:
  - phase: 09-ci-coverage-gates (plan 02)
    provides: The frontend coverage denominator (include glob + three D-06 exclusions) and the recorded starting baseline this plan closes against
provides:
  - "The 70% frontend coverage threshold enforced by vitest.config.ts's own coverage.thresholds, with no separate check script and no CI YAML change (D-08)"
  - "Route-level tests for history.tsx's error/retry, load-more append-with-dedupe, and filtered-vs-unfiltered empty states"
  - "Unit tests for api.ts's shared fetch path (typed error, status-text fallback, no-content, OK) and listEvents' query-string construction"
  - "Gap-closing tests for root.tsx (ErrorBoundary, App nav), watchlist.tsx (error/retry/empty/remove-failure), and SearchResultsColumns.tsx (per-source error, no-matches, three control states, pending/aria-busy)"
affects: [09-05 (CI pipeline wiring), any future phase raising the frontend threshold further]

# Actuals (#2632)
actuals:
  tokens: 6080
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "vi.fn() held as a local reference for a global stub (fetch), rather than vi.mocked(), to keep 'vi.mock' greppable as a strict module-mock-boundary marker"
    - "Duck-typed route-error-response fixtures ({status, statusText, internal, data}) for testing React Router's isRouteErrorResponse-gated ErrorBoundary without a router context"
    - "createRoutesStub used directly (not through the flat renderRoute helper) for nested-route rendering (App + Outlet + children) — same underlying primitive, different shape"

key-files:
  created:
    - web/app/routes/history.test.tsx
    - web/app/lib/api.test.ts
    - web/app/root.test.tsx
    - web/app/components/watchlist/SearchResultsColumns.test.tsx
  modified:
    - web/vitest.config.ts
    - web/app/routes/watchlist.test.tsx
    - .planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md

key-decisions:
  - "Extended watchlist.test.tsx and added root.test.tsx + SearchResultsColumns.test.tsx beyond the plan's originally listed two test files, because the two D-09-priority files alone (api.ts, history.tsx) landed the aggregate at 67.65/52.63/62.62/68.42, still under 70 on all four axes with branches as the binding constraint — Task 3's own action text explicitly authorizes 'keep adding real tests until it does' in D-09 order, so root.tsx (#3), watchlist.tsx (#4), and SearchResultsColumns.tsx (#5) were added next"
  - "Used a local vi.fn() reference instead of vi.mocked(fetch) in api.test.ts so a literal grep for 'vi.mock' returns 0 — vi.mocked() is a substring match of vi.mock(), which would otherwise falsely read as module-mocking the file under test"
  - "Left root.tsx's Layout export untested — it is static SSR document shell markup (html/head/Scripts) with no branch or error logic, the lowest-value target in the file per D-09, and testing it meaningfully would need a much heavier SSR harness than the ErrorBoundary/App tests already written"

patterns-established:
  - "Mutation-style 'has teeth' verification (temporarily break the assertion or the source, confirm red, then restore) run manually during execution for the two highest-value assertions per task, per plan prohibition against theatre tests"

requirements-completed: [CICD-12]

coverage:
  - id: D1
    description: "history.tsx's error/retry, load-more append-with-dedupe, and filtered-vs-unfiltered empty states are covered by route-level tests"
    requirement: "CICD-12"
    verification:
      - kind: unit
        ref: "web/app/routes/history.test.tsx#History route"
        status: pass
    human_judgment: false
  - id: D2
    description: "api.ts's shared fetch path (typed error w/ JSON message, status-text fallback, no-content, OK) and listEvents' query-string construction are covered by unit tests stubbing the runtime fetch"
    requirement: "CICD-12"
    verification:
      - kind: unit
        ref: "web/app/lib/api.test.ts#apiFetch (via the exported endpoint wrappers)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The 70% coverage threshold is committed on all four axes in vitest.config.ts and proven to actually fire (not configured-but-ignored) by a temporary raise to 100 producing a non-zero exit, then a restore to 70 producing a clean one"
    requirement: "CICD-12"
    verification:
      - kind: unit
        ref: "pnpm --dir web test (manual raise-to-100/restore-to-70 verification during execution)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The remaining coverage gap after D1/D2 is closed with real tests targeting root.tsx's ErrorBoundary/App nav, watchlist.tsx's error/retry/empty/remove-failure paths, and SearchResultsColumns.tsx's per-source error/no-matches/three-control-state/pending logic, in D-09 priority order"
    requirement: "CICD-12"
    verification:
      - kind: unit
        ref: "web/app/root.test.tsx, web/app/routes/watchlist.test.tsx, web/app/components/watchlist/SearchResultsColumns.test.tsx"
        status: pass
    human_judgment: false

duration: 75min
completed: 2026-08-13
status: complete
---

# Phase 9 Plan 4: Frontend Coverage Gap Closure and 70% Threshold Summary

**Closed the frontend coverage gap with real tests on `history.tsx`, `api.ts`, `root.tsx`'s ErrorBoundary/nav, `watchlist.tsx`'s error paths, and `SearchResultsColumns.tsx`, then committed vitest's `coverage.thresholds` at 70 on all four axes — verified to actually fire via a temporary raise to 100.**

## Performance

- **Duration:** ~75 min
- **Completed:** 2026-08-13T15:56:52Z
- **Tasks:** 3
- **Files modified:** 7 (4 created, 3 modified)

## Accomplishments
- `web/app/routes/history.test.tsx` (new): failed-initial-fetch error state + retry re-issuing the request, load-more append-with-de-duplication-by-id, and the filtered-vs-unfiltered empty state distinction. Took `history.tsx` from 0% to 94.23/88.37/95/97.77 (stmts/branch/func/lines).
- `web/app/lib/api.test.ts` (new): the shared fetch path's typed-error-with-JSON-message branch, the status-text-fallback-on-malformed-body branch, the no-content short-circuit, the OK/parsed-body branch, and `listEvents`' three-case query-string construction (including the artistId→artist_id / eventType→event_type wire-name mapping). Took `api.ts` from 0% to 82.75/92.85/50/80.76.
- `web/vitest.config.ts`: added the `coverage.thresholds` block (statements/branches/functions/lines, all 70), commented with CICD-12/D-08. Verified the gate actually fires (not just configured) by temporarily raising all four axes to 100, confirming a non-zero exit with all four axes reported unmet, then restoring to 70 and confirming a clean exit.
- Gap-closing beyond the two priority files (needed because branches was still at 52.63% after D1+D2): `web/app/root.test.tsx` (new — `ErrorBoundary`'s 404/non-404/status-text-fallback/unknown-error branches and `App`'s active-tab nav logic), `web/app/routes/watchlist.test.tsx` (extended — failed-initial-fetch error+retry, empty state, remove-failure-triggers-refresh), `web/app/components/watchlist/SearchResultsColumns.test.tsx` (new — per-source error message, no-matches message, the three trailing-control states, and the pending/aria-busy lifecycle around a click).
- Final aggregate: **78.06% statements / 71.57% branches / 75.75% functions / 79.75% lines** — all four axes clear 70%, verified by running `pnpm --dir web test` twice back to back with exit 0 both times.
- Appended a closing-measurement section to `09-BASELINE-FRONTEND.md`, leaving the original starting-baseline section untouched.

## Task Commits

Each task was committed atomically:

1. **Task 1: Route-level tests for the history feed's error, pagination, and empty states** - `fe8b94a` (test)
2. **Task 2: Unit tests for the shared fetch path's error, no-content, and success branches** - `cbff701` (test)
3. **Task 3: Commit the 70% threshold and drive the frontend gate to green** - `76abb4e` (feat — includes the gap-closing test files it required)

_No separate plan-metadata commit in worktree mode — STATE.md/ROADMAP.md are owned by the orchestrator after wave merge; this SUMMARY and REQUIREMENTS.md updates land in a follow-up commit per the worktree protocol._

## Files Created/Modified
- `web/app/routes/history.test.tsx` - New route-level test file for the History route
- `web/app/lib/api.test.ts` - New unit test file for the shared fetch path
- `web/vitest.config.ts` - Added `coverage.thresholds` (70/70/70/70)
- `web/app/root.test.tsx` - New test file for `root.tsx`'s `ErrorBoundary` and `App` nav
- `web/app/routes/watchlist.test.tsx` - Extended with 3 more cases (error/retry, empty state, remove-failure)
- `web/app/components/watchlist/SearchResultsColumns.test.tsx` - New unit test file for the search-results column component
- `.planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md` - Appended closing-measurement section

## Decisions Made
- Task 1 and Task 2's two files alone (targeting `api.ts` and `history.tsx`, the D-09 #1 and #2 priorities) brought the aggregate to 67.65/52.63/62.62/68.42 — under 70 on all four axes, branches the binding constraint by a wide margin (need +18 more covered branches of 190). Task 3's own action text explicitly authorizes continuing in D-09 order ("keep adding real tests until it does... choose targets in D-09 order"), so `root.tsx` (#3), `watchlist.tsx`'s error/loader paths (#4), and `SearchResultsColumns.tsx` (#5) were added next, in that priority order, until all four axes cleared 70.
- Used a local `vi.fn()` reference (`fetchMock`) instead of `vi.mocked(fetch)` throughout `api.test.ts`, so a literal `grep -c 'vi.mock' web/app/lib/api.test.ts` returns exactly 0 as the plan's acceptance criteria require — `vi.mocked(...)` is a substring match of `vi.mock`, which would otherwise falsely register as this file module-mocking the boundary it exists to test directly.
- Left `root.tsx`'s `Layout` export (the SSR document shell: `<html>`/`<head>`/`<Scripts>`) untested. It has no branch or error logic — it is static markup wrapping `children` — making it the lowest-value target in the file per D-09, and exercising `<Meta>`/`<Links>`/`<Scripts>` meaningfully would require a much heavier SSR test harness for no behavioral payoff. `ErrorBoundary` and `App`'s nav logic (the file's only real branches) are fully covered instead.
- Constructed `ErrorBoundary` test fixtures as plain objects matching `isRouteErrorResponse`'s duck-typed shape (`{status, statusText, internal, data}`) rather than importing an internal, non-exported `ErrorResponseImpl` class — confirmed against the installed `react-router` source that the check is structural, not `instanceof`-based.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `Route.ErrorBoundaryProps` requires a `params` prop the plan's read_first notes didn't call out**
- **Found during:** Task 3 (typecheck after writing `root.test.tsx`)
- **Issue:** `pnpm typecheck` failed with `Property 'params' is missing` on all four `<ErrorBoundary error={...} />` renders — the generated `Route.ErrorBoundaryProps` type requires both `params` and `error`, not just `error`.
- **Fix:** Added `params={{}}` to each `<ErrorBoundary>` render call.
- **Files modified:** `web/app/root.test.tsx`
- **Verification:** `pnpm --dir web typecheck` exits 0.
- **Committed in:** `76abb4e` (Task 3 commit)

**2. [Rule 1 - Bug] Task 3's own new test files were needed to close the coverage gap, extending beyond the plan's originally-listed `<files>`**
- **Found during:** Task 3 (after committing the threshold, `pnpm --dir web test` failed on functions and branches at 65.65%/60.52%, still short of 70 after adding `root.test.tsx` alone)
- **Issue:** The plan's frontmatter `files_modified` listed only `web/vitest.config.ts` and the baseline doc for Task 3, but D-09's priority list and the action text's own "keep adding real tests until it does" instruction required more files to actually close the gap.
- **Fix:** Extended `watchlist.test.tsx` (3 new cases) and added `SearchResultsColumns.test.tsx` (5 new cases), per D-09 priorities #4 and #5.
- **Files modified:** `web/app/routes/watchlist.test.tsx`, `web/app/components/watchlist/SearchResultsColumns.test.tsx`
- **Verification:** Full suite (`pnpm --dir web test`) exits 0 with 78.06/71.57/75.75/79.75 on all four axes, run twice back to back with no flakiness.
- **Committed in:** `76abb4e` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking type error, 1 scope expansion explicitly authorized by the plan's own Task 3 instructions)
**Impact on plan:** Both necessary to satisfy the plan's stated success criteria (typecheck passing, all four coverage axes at 70%). No scope creep beyond what D-09 and Task 3's action text already called for.

## Issues Encountered
- Running a single new test file alone via `pnpm --dir web exec vitest run <file>` now exits 1 (coverage-threshold failure) rather than 0, once Task 3 commits the global `coverage.thresholds`. This is expected, unavoidable behavior — Vitest couples the test-pass/fail exit code with the coverage-threshold exit code for the whole collected run, and no single file in this codebase reaches 70% of the ~269-statement denominator alone. All individual test *assertions* still pass when run alone (confirmed via `Test Files N passed`/`Tests N passed` in each isolated run); only the coverage-threshold check fails, which is exactly what it's designed to do outside a full-suite run. The full `pnpm --dir web test` invocation — the one CI actually runs — exits 0. Tasks 1 and 2's own isolated-file runs were verified to exit 0 *before* Task 3 committed the threshold, satisfying their acceptance criteria as written at the time those tasks executed.

## Next Phase Readiness
- The frontend coverage gate is fully wired: `pnpm --dir web test` enforces 70% on all four axes with no CI YAML change needed (D-08) — plan 09-05 only needs to add `frontend-test` to `build-scan`'s `needs:` array.
- `PreferenceToggles.tsx` (43.75/25/25/46.66) and `SearchBox.tsx` (78.37/57.14/88.88/84.37) still carry uncovered branches per the D-09 priority list, left as the next targets if a future phase raises the threshold further — not needed for this plan's 70% goal.

---
*Phase: 09-ci-coverage-gates*
*Completed: 2026-08-13*

## Self-Check: PASSED

All created files verified present on disk (`web/app/routes/history.test.tsx`, `web/app/lib/api.test.ts`,
`web/app/root.test.tsx`, `web/app/components/watchlist/SearchResultsColumns.test.tsx`,
`web/vitest.config.ts`, `.planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md`,
`.planning/phases/09-ci-coverage-gates/09-04-SUMMARY.md`). All three task commits verified present
in `git log` (`fe8b94a`, `cbff701`, `76abb4e`).
