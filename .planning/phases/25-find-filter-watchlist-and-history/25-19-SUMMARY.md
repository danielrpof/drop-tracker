---
phase: 25-find-filter-watchlist-and-history
plan: 19
subsystem: ui
tags: [react, scroll-anchor, vitest, watchlist, gap-closure]

requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "useScrollAnchor two-neighbour fallback (25-17, WR-02)"
provides:
  - "anchorCandidates: chip, then every sibling card nearest-first (next before previous at each distance)"
  - "Hook, ordering and route tests for a run of adjacent leaving cards"
  - "25-UI-SPEC D-12 scroll bullet records the widened fallback rule"
affects: [25-20, 25-21]

actuals:
  tokens: 4600
  tasks: 2
  commits: 2
plan_head_before: 5969c99c9414223a4f965d8f67367548873e78b5

tech-stack:
  added: []
  patterns:
    - "Scroll anchor measures all sibling cards at capture and corrects by the first still-connected one"

key-files:
  created: []
  modified:
    - web/app/lib/useScrollAnchor.ts
    - web/app/lib/useScrollAnchor.test.tsx
    - web/app/routes/watchlist.test.tsx
    - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md

key-decisions:
  - "G-25-11 (user decision 2026-10-12): fallback widens to the nearest surviving card, with no distance cap (a cap would bring back the G-25-4 collapse for a longer run)."
  - "At each distance the next card is tried before the previous one, keeping the 25-17 next-then-previous order as the distance-1 case."

requirements-completed: [WLVW-04]

coverage:
  - id: D1
    description: "A run of adjacent leaving cards anchors on the nearest surviving card instead of collapsing the page"
    requirement: WLVW-04
    verification:
      - kind: unit
        ref: "web/app/lib/useScrollAnchor.test.tsx#useScrollAnchor run of leaving cards (G-25-11)"
        status: pass
      - kind: integration
        ref: "web/app/routes/watchlist.test.tsx#corrects by the nearest survivor's drift when a run of cards leaves (G-25-11)"
        status: pass
    human_judgment: false
  - id: D2
    description: "UI-SPEC D-12 scroll bullet states the widened fallback rule"
    verification:
      - kind: other
        ref: "Task 2 grep and git-diff verify (exactly one removed and one added line)"
        status: pass
    human_judgment: false

duration: 2min
completed: 2026-10-12
status: complete
---

# Phase 25 Plan 19: Widen the scroll-anchor fallback Summary

**`anchorCandidates` now returns the chip plus every sibling card nearest-first (next before previous at each distance), so un-pressing a chip that removes a run of adjacent cards still anchors on the nearest survivor.**

## Performance

- **Duration:** 2 min
- **Started:** 2026-10-12T22:06:18Z
- **Completed:** 2026-10-12T22:08:50Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Closed G-25-11 (review WR-01): the fallback no longer stops at the two immediate neighbours, so a run of leaving cards cannot bring back the G-25-4 collapse.
- Capture API unchanged (`useScrollAnchor(key): (el) => void`), so `watchlist.tsx` and `TagChips.tsx` are untouched.
- Route test at `/?tags=3,7` (Arcangel, Bad Bunny, Chencho Corleone, Daddy Yankee, Eladio Carrion): un-pressing Chencho's reggaeton chip gives `?tags=7`, shows Arcangel and Eladio Carrion, and scrolls once by Eladio's drift (0, -600).
- UI-SPEC D-12 bullet rewritten with the widened rule, tagged `G-25-11, user decision 2026-10-12`.

## Task Commits

1. **Task 1: Run of leaving cards anchors on the nearest survivor** - `a4f8b09` (fix, TDD tracer; RED and GREEN in one commit per plan)
2. **Task 2: Record the widened rule in UI-SPEC D-12** - `9c51992` (docs)

**Plan metadata:** committed separately (docs: complete plan).

## RED evidence

Before the `anchorCandidates` rewrite, `vitest run app/lib/useScrollAnchor.test.tsx app/routes/watchlist.test.tsx` gave 5 failed, 89 passed:
- hook "corrects by the next-side survivor when it ties on distance": `expected "bound " to be called 1 times, but got 0 times`
- hook "falls back to the previous side when the next side runs out": same (scrollBy never called)
- `anchorCandidates` "lists every sibling card nearest-first": received `[chip, D4, B2]`, missing `E5, A1`
- `anchorCandidates` "continues on the next side when the previous side runs out": received `[chip, C3, A1]`, missing `D4, E5`
- route "corrects by the nearest survivor's drift when a run of cards leaves": `scrollBy` called 0 times

After the rewrite all 94 tests in the two files pass; the full web suite, typecheck and prettier check pass.

## Files Created/Modified
- `web/app/lib/useScrollAnchor.ts` - `anchorCandidates` walks outward over `card.parentElement.children`
- `web/app/lib/useScrollAnchor.test.tsx` - `RunHarness`, two run-case hook tests, two ordering tests
- `web/app/routes/watchlist.test.tsx` - route case for a run of leaving cards
- `25-UI-SPEC.md` - D-12 scroll bullet final sentence

## Decisions Made
- Recorded under key-decisions above: no distance cap; next before previous at each distance.
- 25-CONTEXT.md left unchanged, as the plan directs (no CONTEXT line governs the fallback order).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None. Client-only layout logic; no new endpoints, auth paths or schema.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 25-20 (G-25-12) can use the same fallback card as its focus target.
- The embedded SPA (`internal/webassets/build/client`) is not rebuilt here; that happens in plan 25-21.

## Self-Check: PASSED

- Files modified exist and match the plan's `files_modified`.
- Commits `a4f8b09` and `9c51992` exist on the branch; `git rev-list --count 5969c99..HEAD` = 2.
- Task 1 and Task 2 acceptance criteria re-run green (tests, typecheck, prettier check, full web test, unchanged-file guards, UI-SPEC one-line diff).

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-12*
