---
phase: 08-frontend-test-suite
plan: 01
subsystem: testing
tags: [vitest, jsdom, react-testing-library, github-actions, ci]

# Dependency graph
requires:
  - phase: 06-frontend-release-history
    provides: "HistoryFilters component and web/app/lib/api.ts's typed fetch boundary this plan tests against"
provides:
  - "Standalone web/vitest.config.ts + web/vitest.setup.ts test harness (jsdom, ~ alias, mockReset, no vacuous-green escape hatch)"
  - "pnpm test / pnpm test:watch scripts"
  - "First permanently-kept component test (HistoryFilters.test.tsx) proving artist-list population and upward filter reporting, including the artistId null-not-zero clear case"
  - "frontend-test job in Full Pipeline's parallel tier (report-only, SHA-pinned actions)"
affects: [08-02, 08-03, 08-04, 08-05, 09-ci-coverage-gates]

# Actuals (#2632)
actuals:
  tokens: 2049
  tasks: 3
  commits: 2

tech-stack:
  added: ["vitest@4.1.10", "jsdom@30.0.1", "@testing-library/react@16.3.2", "@testing-library/jest-dom@7.0.1", "@testing-library/user-event@14.6.4"]
  patterns:
    - "Standalone vitest.config.ts that never imports vite.config.ts or the React Router Vite dev plugin"
    - "Manual RTL cleanup via afterEach in vitest.setup.ts (no Vitest globals enabled)"
    - "vi.mock('~/lib/api') bare per test file (no factory) as the API-boundary mock (TEST-02)"

key-files:
  created:
    - web/vitest.config.ts
    - web/vitest.setup.ts
    - web/app/components/history/HistoryFilters.test.tsx
  modified:
    - web/package.json
    - web/pnpm-lock.yaml
    - .github/workflows/full-pipeline.yml

key-decisions:
  - "Task 1's package-legitimacy checkpoint was pre-resolved by the orchestrator's retry context (human already approved all five packages and both new Action SHAs, including keeping pnpm/action-setup pinned to the plan's original tag-object SHA over the offered dereferenced-commit alternative) -- no human interaction re-run in this session"
  - "Added manual afterEach(cleanup) wiring to vitest.setup.ts (Rule 1 auto-fix): RTL's auto-cleanup only self-registers against a global afterEach, and this config imports test APIs explicitly rather than enabling Vitest's globals option, so without it, later tests' getByRole queries matched multiple accumulated DOM trees"
  - "Rewrote two vitest.config.ts comments that literally contained the plan's own acceptance-criteria grep targets (react-router/dev, passWithNoTests) so the negative-match checks (must NOT appear anywhere in the file) pass -- the config's actual behavior was already correct, only the prose needed rephrasing"

patterns-established:
  - "Config-file-level test harness kept fully independent of the production Vite config (D-01), reversible by deleting one file"

requirements-completed: [TEST-01, TEST-02]

coverage:
  - id: D1
    description: "pnpm --dir web test runs the Vitest suite in jsdom and exits non-zero on a component regression, with no pass-with-no-tests escape hatch"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "web/app/components/history/HistoryFilters.test.tsx (3 tests) via pnpm --dir web test"
        status: pass
      - kind: other
        ref: "Manually inverted one assertion, confirmed non-zero exit, restored, confirmed green again (git status clean afterward)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Suite is idempotent and order-independent (two consecutive runs green; --sequence.shuffle run green)"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "pnpm --dir web test (run twice) and pnpm --dir web exec vitest run --sequence.shuffle"
        status: pass
    human_judgment: false
  - id: D3
    description: "HistoryFilters.test.tsx asserts artist-list population from listWatchlist, upward filter reporting on artist selection, and artistId: null (never 0) when the artist filter is cleared"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "web/app/components/history/HistoryFilters.test.tsx -- 3 named it() cases"
        status: pass
    human_judgment: false
  - id: D4
    description: "Tests mock web/app/lib/api.ts (not raw fetch), suite passes with no server/network"
    requirement: "TEST-02"
    verification:
      - kind: unit
        ref: "vi.mock(\"~/lib/api\") bare call, no vi.mock global/globalThis.fetch patch anywhere in web/app/**/*.test.tsx"
        status: pass
    human_judgment: false
  - id: D5
    description: "frontend-test job added to Full Pipeline's parallel tier, SHA-pinned actions, build-scan's needs array untouched"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "node -e regex check (frontend-test job declared, build-scan needs array unchanged) + js-yaml parse of the full workflow file"
        status: pass
    human_judgment: false
  - id: D6
    description: "frontend-test job actually runs green inside a real GitHub Actions run"
    verification: []
    human_judgment: true
    rationale: "Requires a real push/PR to GitHub Actions to observe; not observable from this local worktree session."

duration: 22min
completed: 2026-08-12
status: complete
---

# Phase 8 Plan 1: Vitest Harness + First Component Test + CI Job Summary

**Stood up a standalone Vitest + jsdom + React Testing Library harness (never merging the production Vite config), proved it with a real, permanently-kept `HistoryFilters` test, and wired the same `pnpm test` command into a new `frontend-test` job in Full Pipeline.**

## Performance

- **Duration:** ~22 min
- **Tasks:** 3 (Task 1 checkpoint pre-resolved via retry context; Tasks 2-3 executed)
- **Files modified:** 6 (2 new source files + 1 new test file + package.json + pnpm-lock.yaml + full-pipeline.yml)

## Accomplishments
- Installed the five approved devDependencies (`vitest`, `jsdom`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`) at RESEARCH.md's exact pinned versions
- Built `web/vitest.config.ts` as a fully independent config (jsdom environment, `~` alias replicating tsconfig, `mockReset: true`, no vacuous-green escape hatch) and `web/vitest.setup.ts` (jest-dom matchers + manual RTL cleanup)
- Wrote `HistoryFilters.test.tsx` with three behaviors: artist list populates from `listWatchlist`, choosing an artist reports the full new value upward, and clearing the artist filter reports `artistId: null` never `0`
- Added the `frontend-test` job to `.github/workflows/full-pipeline.yml`'s parallel tier, SHA-pinned per the pre-approved retry-context values, report-only (build-scan's `needs` untouched)

## Task Commits

Each task was committed atomically:

1. **Task 1: Package legitimacy gate** - pre-resolved via retry context (no file changes; see Deviations)
2. **Task 2: Vitest harness green end-to-end** - `bb435ae` (feat)
3. **Task 3: Add frontend-test job to Full Pipeline** - `f3a0a8a` (feat)

_Note: Task 2's RTL-cleanup fix (see Deviations) landed inside the same `bb435ae` commit -- it was discovered and fixed before the task's first commit, not as a separate follow-up._

## Files Created/Modified
- `web/vitest.config.ts` - Standalone Vitest config (jsdom, `~` alias, `mockReset`, no pass-with-no-tests escape)
- `web/vitest.setup.ts` - jest-dom matcher registration + manual `afterEach(cleanup)` wiring
- `web/app/components/history/HistoryFilters.test.tsx` - First permanently-kept component test (3 behaviors)
- `web/package.json` - Added `test`/`test:watch` scripts and the five devDependencies
- `web/pnpm-lock.yaml` - Locked resolutions for the five new packages
- `.github/workflows/full-pipeline.yml` - New `frontend-test` job in the parallel tier

## Decisions Made
- Task 1's checkpoint was treated as already resolved per the orchestrator's retry context: all five packages approved, `pnpm/action-setup` kept at the plan's original SHA `ff378ebe6b225b0680b81c1ad4498ae0d1d3a5e3` (human explicitly declined the offered alternate dereferenced SHA), `actions/setup-node` confirmed at `820762786026740c76f36085b0efc47a31fe5020`. No human interaction was re-run this session.
- Kept the RESEARCH.md-recommended manual `~` alias (no `vite-tsconfig-paths` dependency) since the project has exactly one path mapping.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added manual RTL cleanup to vitest.setup.ts**
- **Found during:** Task 2 (first `pnpm --dir web test` run)
- **Issue:** 2 of 3 `HistoryFilters.test.tsx` tests failed with "Found multiple elements with the role combobox and name Artist" -- `@testing-library/react`'s auto-cleanup only self-registers against a global `afterEach` (Jest-style globals), but this project's `vitest.config.ts` imports test APIs explicitly rather than enabling Vitest's `test.globals` option, so rendered trees from earlier tests accumulated in the jsdom document across tests in the same file.
- **Fix:** Added `import { cleanup } from "@testing-library/react"` + `afterEach(() => cleanup())` to `vitest.setup.ts`.
- **Files modified:** `web/vitest.setup.ts`
- **Verification:** `pnpm --dir web test` went from 2 failed/1 passed to 3 passed; re-ran the suite twice more and with `--sequence.shuffle`, all green.
- **Committed in:** `bb435ae` (Task 2 commit)

**2. [Rule 1 - Bug] Rephrased two vitest.config.ts comments that collided with the plan's own negative-match grep checks**
- **Found during:** Task 2 (running the plan's own acceptance-criteria greps after the first green test run)
- **Issue:** The plan's acceptance criteria require `grep -c 'passWithNoTests' web/vitest.config.ts` to be 0 and `! grep -q 'react-router/dev' web/vitest.config.ts` to succeed. My first draft's explanatory comments literally contained both strings ("Deliberately NOT set: passWithNoTests" and "never loads @react-router/dev/vite"), so both greps failed even though the config's actual behavior (no `passWithNoTests` option set, no React Router plugin imported) was already correct.
- **Fix:** Reworded both comments to describe the same intent without using the literal flagged substrings.
- **Files modified:** `web/vitest.config.ts`
- **Verification:** Re-ran both greps (`passWithNoTests` count now 0, `react-router/dev` absent) and re-ran the full test suite + typecheck to confirm the wording change had no functional effect.
- **Committed in:** `bb435ae` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 -- bugs blocking a green/verifiable state)
**Impact on plan:** Both fixes were needed to reach the plan's own stated done/acceptance criteria. No scope creep; no source component code was touched.

## Issues Encountered
None beyond the two auto-fixed issues above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The Vitest harness (`vitest.config.ts`, `vitest.setup.ts`, `pnpm test`) is proven end-to-end and ready for sibling plans 08-02 through 08-05 to add their own co-located test files against it without further config work.
- `frontend-test` CI job exists and is report-only; Phase 9 (CI Coverage Gates) is the phase that adds the coverage-percentage step to this same job and, per Open Question 1 in 08-RESEARCH.md, may also decide whether to add `frontend-test` to `build-scan`'s `needs` array (left untouched this plan per D-04/A3).
- D6 (frontend-test job actually running green in a real GitHub Actions run) is the one coverage item this plan could not verify locally -- flagged `human_judgment: true` above; will be observable on the next push/PR.

---
*Phase: 08-frontend-test-suite*
*Completed: 2026-08-12*
