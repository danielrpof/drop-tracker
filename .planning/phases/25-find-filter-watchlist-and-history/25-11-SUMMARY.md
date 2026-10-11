---
phase: 25-find-filter-watchlist-and-history
plan: 11
subsystem: ui
tags: [react, react-router, scroll-anchoring, useLayoutEffect, vitest, watchlist]

requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: chip click-to-filter (D-08) and the tags URL param
provides:
  - useScrollAnchor hook that keeps a clicked element at its on-screen position across a filter re-render
  - Chip label passes its own button to toggleFilterTag as the scroll anchor
affects: [25-14, UAT test 4 re-check]

plan_head_before: 362978e7cc657c9a36cb9b3a1335f68f413cfe20
actuals:
  tokens: 2940
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Capture an element's rect top before a URL write; a layout effect keyed on the filter key scrolls by the remaining drift"

key-files:
  created:
    - web/app/lib/useScrollAnchor.ts
    - web/app/lib/useScrollAnchor.test.tsx
  modified:
    - web/app/components/watchlist/TagChips.tsx
    - web/app/components/watchlist/TagChips.test.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx

key-decisions:
  - "Only the chip-label path anchors; the Has muted events toggle sits in the non-sticky toolbar near the top, so it has no card to keep in place"
  - "No expiry on an unconsumed anchor: every capture is followed by a filterKey change that consumes it, and a timer or same-key clear would race the router's commit"
  - "Correction measures the element's final rect, so browser clamping and Chrome scroll anchoring are covered because only remaining drift is corrected"

patterns-established:
  - "useScrollAnchor(key) returns a stable capture(el); the layout effect clears the anchor before measuring, so it fires at most once per capture"

requirements-completed: [WLVW-04]

coverage:
  - id: D1
    description: "Chip-label click records the chip's on-screen top and scrolls the window by the drift after the filtered re-render, in both the shrink and grow directions"
    requirement: WLVW-04
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.test.tsx#chip click keeps the clicked card on screen (G-25-4)"
        status: pass
      - kind: unit
        ref: "web/app/lib/useScrollAnchor.test.tsx"
        status: pass
    human_judgment: false
  - id: D2
    description: "Disconnected anchor and zero drift call no scroll API; an anchor is consumed by one key change only"
    requirement: WLVW-04
    verification:
      - kind: unit
        ref: "web/app/lib/useScrollAnchor.test.tsx"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.test.tsx#skips the correction when the clicked card leaves the list"
        status: pass
    human_judgment: false
  - id: D3
    description: "Real-browser behavior: the clicked chip stays within a few pixels on a tall filtered page, and the page clamps with the card still visible on a short one"
    requirement: WLVW-04
    verification: []
    human_judgment: true
    rationale: "jsdom has no layout; real clamping and scroll anchoring only show in a browser (UAT test 4, human_verify_mode end-of-phase)"

duration: 5min
completed: 2026-10-11
status: complete
---

# Phase 25 Plan 11: Chip-Click Scroll Anchoring Summary

**`useScrollAnchor` layout-effect hook plus a chip label that passes itself as the anchor, so a chip-label filter keeps the clicked card where it was on screen (G-25-4)**

## Performance

- **Duration:** 5 min
- **Started:** 2026-10-11T16:53:30Z
- **Completed:** 2026-10-11T16:58:30Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- `useScrollAnchor(key)` returns a stable `capture(el)`; a `useLayoutEffect` keyed on the filter key scrolls by the element's drift before paint, and skips when the anchor is gone or the drift is 0.
- `TagChips` passes `e.currentTarget` to `toggleFilterTag(tag, anchor?)`; `watchlist.tsx` captures it before writing the tags param. `useUrlParams` is untouched and still always passes `preventScrollReset: true`.
- Tests pin the shrink and grow directions, the clicked card leaving the list, zero drift, no capture, and one-shot consumption. A mutation check (removing the `isConnected` guard) made both disconnected-case tests fail, so they discriminate.

## Task Commits

1. **Task 1: tracer, clicked card stays put** - `c52f2e1` (test, RED), `f1e03c7` (feat, GREEN)
2. **Task 2: pin both directions and edge cases** - `a0f885b` (test)

**Plan metadata:** committed separately (docs: complete plan)

## Files Created/Modified
- `web/app/lib/useScrollAnchor.ts` - capture plus layout-effect scroll correction
- `web/app/lib/useScrollAnchor.test.tsx` - five hook cases (a-e)
- `web/app/components/watchlist/TagChips.tsx` - label onClick passes its button as the anchor
- `web/app/components/watchlist/TagChips.test.tsx` - `toHaveBeenCalledWith` now includes the label element
- `web/app/routes/watchlist.tsx` - `useScrollAnchor(filterKey)` and `toggleFilterTag` captures before `handleTagsChange`
- `web/app/routes/watchlist.test.tsx` - G-25-4 describe with shrink, grow, and disconnected cases

## Decisions Made
See key-decisions; all follow the plan and the binding 2026-10-11 user decision on G-25-4.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TagChips.test.tsx assertion moved into the Task 1 commit**
- **Found during:** Task 1 (feat commit)
- **Issue:** Widening `toggleFilterTag` to pass the label made the existing one-argument `toHaveBeenCalledWith` fail. Leaving it for Task 2 would have committed a red suite.
- **Fix:** Made the planned one-line edit (add `label` as the second argument) in the Task 1 feat commit instead of Task 2.
- **Files modified:** web/app/components/watchlist/TagChips.test.tsx
- **Committed in:** f1e03c7

**2. [Rule 1 - Bug] Disconnected-case tests did not discriminate on first draft**
- **Found during:** Task 2 (mutation check)
- **Issue:** The route test stubbed the rect on `aria-pressed`, which never changes on a detached label, and the hook test used equal before/after tops, so both passed even without the `isConnected` guard.
- **Fix:** Hook test sets a different top before the unmount; route test keys the stub on read order (400 first, then 120). Both now fail when the guard is removed.
- **Files modified:** web/app/lib/useScrollAnchor.test.tsx, web/app/routes/watchlist.test.tsx
- **Committed in:** a0f885b

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug in test design)
**Impact on plan:** No source behavior change and no scope creep.

## TDD Gate Compliance

RED (`c52f2e1`, test failed on the `scrollBy` call-count assertion for the target test) precedes GREEN (`f1e03c7`). No refactor commit.

## Issues Encountered

None.

## Human Check Pending

Task 2's `<human-check>` (UAT test 4 re-run in a real browser, tall and short filtered pages, both directions) is queued for end-of-phase verification. No headless-Chrome measurement was run.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- 25-14 can reword UI-SPEC D-12 and the UAT truth; `web/package.json` and `web/pnpm-lock.yaml` are unchanged, as its blast-radius gate expects.
- Full web suite: 31 files, 576 tests pass; typecheck and prettier check exit 0.

## Self-Check: PASSED

- Created files exist: `useScrollAnchor.ts`, `useScrollAnchor.test.tsx`.
- Commits `c52f2e1`, `f1e03c7`, `a0f885b` are on the branch (measured count 3 from `plan_head_before`).
- Acceptance greps: `useScrollAnchor(filterKey)` count is 1; empty diff for `useUrlParams.ts`, `useUrlParams.test.tsx`, `root.tsx`, `package.json`, `pnpm-lock.yaml`.

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-11*
