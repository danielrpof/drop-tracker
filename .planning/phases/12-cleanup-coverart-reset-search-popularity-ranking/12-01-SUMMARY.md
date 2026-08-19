---
phase: 12-cleanup-coverart-reset-search-popularity-ranking
plan: 01
subsystem: ui
tags: [react, vitest, react-testing-library, coverart]

# Dependency graph
requires: []
provides:
  - "CoverArt.tsx resets its load-failure flag via useEffect([src]) when src changes on a retained instance"
  - "CoverArt.test.tsx — first test file for this component, covering reset, unchanged-src guard, and null-src placeholder"
affects: [history, watchlist, search-results]

# Actuals (#2632)
actuals:
  tokens: 1037
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "useEffect([dep]) reset pattern for clearing derived failure state on a retained component instance, chosen over key-based remount to avoid touching call sites"

key-files:
  created:
    - web/app/components/common/CoverArt.test.tsx
  modified:
    - web/app/components/common/CoverArt.tsx

key-decisions:
  - "Used an effect-based reset (useEffect([src]) -> setFailed(false)) rather than a key={src} remount, per D-01's locked decision, to keep the fix scoped to CoverArt.tsx with zero call-site changes"

patterns-established:
  - "useEffect([dep]) reset pattern: when a component derives failure/error UI state from a prop and is retained (not remounted) across prop changes, reset that state in an effect keyed on the prop, with a code comment recording the accepted one-frame-stale tradeoff"

requirements-completed: [D-01, D-02]

coverage:
  - id: D1
    description: "CoverArt resets its failed-load placeholder when src changes on a retained instance (D-01)"
    requirement: "D-01"
    verification:
      - kind: unit
        ref: "web/app/components/common/CoverArt.test.tsx#clears the failed placeholder when src changes"
        status: pass
    human_judgment: false
  - id: D2
    description: "Regression test locks in the D-01 fix and proves it does not mask a genuine failure or break the null-src placeholder (D-02)"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "web/app/components/common/CoverArt.test.tsx#keeps the placeholder when src stays the same after a load error"
        status: pass
      - kind: unit
        ref: "web/app/components/common/CoverArt.test.tsx#renders the placeholder when src is null"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-08-18
status: complete
---

# Phase 12 Plan 01: CoverArt Reset Fix Summary

**Fixed CoverArt's stale-placeholder bug with a `useEffect([src])` reset and locked it in with three Vitest/RTL regression tests.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-08-18T21:35:00Z
- **Completed:** 2026-08-18T21:47:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- `CoverArt.tsx` now clears its `failed` flag in a `useEffect` keyed on `src`, so a row whose cover art once failed to load shows the real image again once a later `src` would succeed
- New `CoverArt.test.tsx` proves the reset fires on a changed `src`, does not re-fire (and mask a real failure) on an unchanged `src`, and preserves the pre-existing null/undefined-`src` placeholder behavior
- Zero call-site changes — `WatchlistRow.tsx`, `EventCard.tsx`, and `SearchResultsColumns.tsx` are untouched; the fix is fully contained in the shared `CoverArt` component

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end — a changed src clears the failed placeholder** - `b550b48` (fix)
2. **Task 2: Guard the reset against masking a real failure** - `c2ee963` (test)

**Plan metadata:** pending (this commit)

_Note: this plan's tasks were type="tracer"/"auto" with tdd="true" and a single combined `<action>` block (no separate RED/GREEN sections), so each task landed as one atomic commit rather than a split test/feat pair — per execute-plan.md's tracer-task commit rule._

## Files Created/Modified
- `web/app/components/common/CoverArt.tsx` - Added `useEffect([src])` immediately after `useState(false)` that calls `setFailed(false)`, with a comment locking in the effect-over-remount decision and its accepted one-frame-stale tradeoff
- `web/app/components/common/CoverArt.test.tsx` - New file; three tests: reset-on-src-change (D-02), no-reset-on-unchanged-src (guard), null-src placeholder (regression guard)

## Decisions Made
- Followed the plan exactly: effect-based reset with dependency array `[src]`, no `key` prop, no call-site changes.
- Query strategy for tests: used `getByAltText`/`queryByAltText` to unambiguously select the real `<img>` (which carries `alt`) versus the placeholder `<div role="img" aria-label>` (which does not carry `alt`), matching the plan's explicit guidance.

## Deviations from Plan

None - plan executed exactly as written.

## Verification Performed

- `cd web && npx vitest run CoverArt` — 3/3 tests pass (scoped run; full-suite `pnpm test -- CoverArt` invocation from the plan resolved to the same file via `vitest run CoverArt`)
- `cd web && pnpm test` (full suite) — 53/53 tests pass, coverage thresholds (statements 83.18%, branches 74.13%, functions 81.03%, lines 84.03%) all clear the 70% gate
- `cd web && pnpm typecheck` — exits 0
- `git diff --name-only` — confirmed exactly `CoverArt.tsx` and `CoverArt.test.tsx` changed, no other call site touched
- Vacuity checks (temporarily reverted, confirmed failure, restored): removing the effect body fails Test 1; widening the dependency array to no-array (effect on every render) fails Test 2 — both regression tests are real guards, not vacuous assertions

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- D-01 and D-02 fully satisfied; the shared `CoverArt` component now correctly recovers from a stale failure across History, Watchlist, and search-result rows.
- No blockers for subsequent plans in this phase (Deezer/MusicBrainz search popularity ranking work is independent of this plan's files).

---
*Phase: 12-cleanup-coverart-reset-search-popularity-ranking*
*Completed: 2026-08-18*
