---
phase: 09-ci-coverage-gates
plan: 02
subsystem: testing
tags: [vitest, coverage-v8, frontend-testing, ci]

# Dependency graph
requires:
  - phase: 08-frontend-test-suite
    provides: The existing 5-file / 16-test Vitest + RTL suite this plan measures coverage over
provides:
  - "@vitest/coverage-v8@4.1.10 installed and pinned, matching vitest's exact version"
  - "web/vitest.config.ts coverage block: v8 provider, honest app/**/*.{ts,tsx} include glob, D-06 exclusions, text-only reporter, enabled: true"
  - "Frontend starting baseline recorded in 09-BASELINE-FRONTEND.md: all four axes below 70%"
affects: [09-04-frontend-coverage-gap-closure, 09-05-ci-coverage-gate-wiring]

# Actuals (#2632)
actuals:
  tokens: 5600
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: ["@vitest/coverage-v8@4.1.10"]
  patterns:
    - "coverage.include as the mechanism that forces untested first-party files into the denominator (Vitest 4 removed coverage.all)"
    - "coverage.enabled: true required in config -- config presence alone does not activate coverage collection on vitest@4.1.10"

key-files:
  created:
    - .planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md
  modified:
    - web/package.json
    - web/pnpm-lock.yaml
    - web/vitest.config.ts
    - .gitignore

key-decisions:
  - "Resolved 09-RESEARCH.md Open Question 1 empirically: vitest run does NOT activate coverage from config presence alone on vitest@4.1.10 -- added coverage.enabled: true as the plan's specified fallback, keeping pnpm test a single unchanged vitest run invocation"
  - "No coverage.thresholds added in this plan -- deferred to plan 09-04 after the baseline is recorded, per plan scope and confirmed by grep -c 'thresholds' returning 0"

patterns-established:
  - "Frontend coverage denominator is app/**/*.{ts,tsx} minus exactly three D-06 carve-outs (shadcn ui primitives, test-only helpers, test files) -- never narrow this glob to improve a number"

requirements-completed: [CICD-12]

coverage:
  - id: D1
    description: "@vitest/coverage-v8@4.1.10 installed as an exact-pinned devDependency matching vitest's own pin, with the lockfile committed for --frozen-lockfile CI installs"
    requirement: CICD-12
    verification:
      - kind: unit
        ref: "pnpm --dir web install --frozen-lockfile"
        status: pass
      - kind: other
        ref: "grep -c '\"@vitest/coverage-v8\": \"4.1.10\"' web/package.json == 1"
        status: pass
    human_judgment: false
  - id: D2
    description: "web/vitest.config.ts coverage block measures every first-party app/** source file (including files no test imports, e.g. history.tsx and api.ts), excludes exactly D-06's three carve-outs, and adds no thresholds"
    requirement: CICD-12
    verification:
      - kind: unit
        ref: "pnpm --dir web test (coverage table includes history.tsx and api.ts rows, excludes components/ui/, lib/test/, *.test.* rows)"
        status: pass
      - kind: other
        ref: "grep -c 'thresholds' web/vitest.config.ts == 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "Frontend starting baseline measured from real command output and recorded in 09-BASELINE-FRONTEND.md, with all four threshold axes stated separately and a D-09-ordered target list for plan 09-04"
    requirement: CICD-12
    verification:
      - kind: other
        ref: ".planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md (131 lines, contains verbatim table + All files row + four axes vs 70 + D-09 target list)"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-08-13
status: complete
---

# Phase 09 Plan 02: Frontend Coverage Provider + Baseline Summary

**Installed `@vitest/coverage-v8@4.1.10` with an explicit `coverage.include` glob that forces every first-party `app/**` file into the denominator, then measured and recorded a frontend starting baseline of 39.77% statements / 25.26% branches / 38.38% functions / 41.29% lines — all four below the 70% threshold plan 09-04 will close.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-08-13
- **Tasks:** 2
- **Files modified:** 5 (4 in Task 1, 1 new in Task 2)

## Accomplishments

- `@vitest/coverage-v8@4.1.10` installed as an exact-pinned devDependency, matching the already-audited `vitest@4.1.10` pin
- `web/vitest.config.ts` `coverage` block added: `v8` provider, `text`-only reporter (writes nothing to disk), `include: ["app/**/*.{ts,tsx}"]` (the load-bearing fix for Vitest 4's `coverage.all` removal — 09-RESEARCH.md Pitfall 3), and exactly D-06's three exclusion globs
- Resolved 09-RESEARCH.md Open Question 1 empirically: a bare `pnpm test` printed no coverage table with only config presence — `coverage.enabled: true` was required and added as the plan's specified fallback, with `pnpm test`'s script and the CI step left unchanged (D-08)
- `.gitignore` updated with a `web/coverage/` backstop entry
- Frontend starting baseline measured and recorded in `.planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md`: verbatim coverage table, all four axes stated against 70%, files-near-zero list with genuine-vs-structural classification, and a D-09-ordered target list for plan 09-04

## Task Commits

Each task was committed atomically:

1. **Task 1: Install the v8 coverage provider and configure an honest first-party denominator** - `0f8390a` (feat)
2. **Task 2: Measure and record the frontend starting baseline** - `8f85384` (docs)

_Note: SUMMARY.md and this plan's metadata are committed separately by the orchestrator after wave completion (worktree mode)._

## Files Created/Modified

- `web/package.json` - Added `@vitest/coverage-v8` devDependency, exact pin `4.1.10`
- `web/pnpm-lock.yaml` - Regenerated lockfile entry for the new devDependency
- `web/vitest.config.ts` - New `coverage` block: `enabled`, `provider`, `reporter`, `include`, `exclude`
- `.gitignore` - Added `web/coverage/` entry alongside the existing `web/build/`/`web/.react-router/` entries
- `.planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md` - New file: the recorded frontend starting baseline

## Decisions Made

- **`coverage.enabled: true` added to config, not a `--coverage` CLI flag or a `package.json` script change.** Empirically confirmed `vitest run` does not activate coverage from config presence alone on `vitest@4.1.10`; the plan pre-specified this exact fallback, so no deviation from plan intent — this satisfies D-08's "no CI or script change" requirement while still collecting coverage.
- **No `coverage.thresholds` added.** Deliberately deferred to plan 09-04 so the baseline is recorded from real, unenforced output first, per the plan's explicit scope boundary and ROADMAP success criterion 4.

## Deviations from Plan

None - plan executed exactly as written. Task 1 step 3's `enabled: true` fallback and Task 2's baseline recording both matched the plan's pre-specified contingency and content requirements exactly; no Rule 1-4 auto-fixes were needed.

## Issues Encountered

- **Investigated why three first-party files (`app/components/common/EmptyState.tsx`, `app/lib/utils.ts`, `app/routes.ts`) don't get individual rows in the default `text` reporter table**, even though the plan's honest-denominator requirement demands they be counted. Diagnosed via a temporary (uncommitted) `json-summary` reporter run: all three are at 100% coverage across every axis (the `text` reporter's tree view folds fully-covered leaf files into their parent directory's aggregate rather than printing a separate row for them — an `istanbul` display behavior, not a config gap). Confirmed the "All files" row's totals (`107/269` statements etc.) exactly match the sum of all 14 first-party files' individual coverage data, proving the denominator is honest despite the display omission. Reverted the diagnostic `json-summary` reporter addition before committing Task 1 (kept `reporter: ["text"]` only, per the plan's spec) and removed the resulting `web/coverage/` directory it wrote to disk during the investigation — working tree confirmed clean (`git status --short` empty) before Task 2 began.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 09-04 (frontend coverage gap closure) has a concrete, D-09-ordered target list to work from: `app/lib/api.ts` first, then `app/routes/history.tsx`, `app/root.tsx`, `app/routes/watchlist.tsx` error paths, `SearchResultsColumns.tsx`, `PreferenceToggles.tsx` remaining branches.
- Plan 09-05 (CI coverage gate wiring) can rely on `pnpm test` already collecting coverage with the correct denominator — adding `coverage.thresholds` in 09-04 is the only remaining config change before the `frontend-test` CI job starts gating on it.
- No blockers.

---
*Phase: 09-ci-coverage-gates*
*Completed: 2026-08-13*

## Self-Check: PASSED

All created/modified files (`web/package.json`, `web/vitest.config.ts`, `.gitignore`,
`.planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md`) confirmed present on disk.
Both task commits (`0f8390a`, `8f85384`) confirmed present in `git log`.
