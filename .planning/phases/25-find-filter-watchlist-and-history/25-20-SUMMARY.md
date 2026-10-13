---
phase: 25-find-filter-watchlist-and-history
plan: 20
subsystem: ui
tags: [react, focus-management, useEffectEvent, watchlist, gap-closure]

requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "anchorCandidates returns chip then nearest-first sibling cards (25-19)"
provides:
  - "useScrollAnchor(key, onFocusLost?) with capture(el, viaKeyboard?) reporting the surviving card, or null, when a keyboard capture's focused element left the DOM"
  - "data-chip-label marker on each chip's filter-toggle button; e.detail === 0 passed as viaKeyboard"
  - "Route focus target: fallback card's pressed chip label, then any chip label, then the tabIndex -1 'Your artists' heading"
  - "25-UI-SPEC focus table row 'Card chip label | Toggled, card leaves'"
affects: [25-21]

actuals:
  tokens: 9000
  tasks: 3
  commits: 3
plan_head_before: 742f5794c565954d336f2c51341ed2ddc7f4b4c5

tech-stack:
  added: []
  patterns:
    - "Keyboard activation detected statelessly via click detail === 0"
    - "Hook reports lost focus through useEffectEvent so the layout effect still depends on key alone"

key-files:
  created: []
  modified:
    - web/app/components/watchlist/TagChips.tsx
    - web/app/components/watchlist/TagChips.test.tsx
    - web/app/lib/useScrollAnchor.ts
    - web/app/lib/useScrollAnchor.test.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
    - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md

key-decisions:
  - "G-25-12 (user decision 2026-10-12): when a toggled chip's card leaves, focus moves to a chip label on the fallback anchor card (the card the scroll anchor uses), preferring a pressed label."
  - "Keyboard only (user decision 2026-10-12): a click with detail 0 (Enter or Space) triggers the move; a mouse or touch click leaves focus alone."
  - "No surviving card: focus the 'Your artists' heading (tabIndex -1, always mounted) rather than the empty state's Clear filters button."
  - "onFocusLost wrapped in React 19.2 useEffectEvent so a new callback identity never re-runs the layout effect or consumes the anchor early."

requirements-completed: [WLVW-04]

coverage:
  - id: D1
    description: "Keyboard toggle that removes the card lands focus on the fallback card's pressed chip label (next or previous card), never <body>"
    requirement: WLVW-04
    verification:
      - kind: integration
        ref: "web/app/routes/watchlist.test.tsx#chip toggle moves focus to the fallback card when its card leaves (G-25-12)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Mouse click that removes the card moves no focus; card-stays focus behaviour unchanged"
    requirement: WLVW-04
    verification:
      - kind: integration
        ref: "web/app/routes/watchlist.test.tsx#a mouse click that removes the card moves no focus"
        status: pass
      - kind: unit
        ref: "web/app/lib/useScrollAnchor.test.tsx#useScrollAnchor onFocusLost (G-25-12)"
        status: pass
    human_judgment: false
  - id: D3
    description: "No surviving card focuses the 'Your artists' heading; run of leavers lands on the scroll-anchor card"
    requirement: WLVW-04
    verification:
      - kind: integration
        ref: "web/app/routes/watchlist.test.tsx#focuses the Your artists heading when no card survives"
        status: pass
      - kind: integration
        ref: "web/app/routes/watchlist.test.tsx#lands on the same card the scroll anchor uses when a run of cards leaves"
        status: pass
    human_judgment: false
  - id: D4
    description: "UI-SPEC focus table states the 'Toggled, card leaves' row with keyboard-only scope"
    verification:
      - kind: other
        ref: "Task 3 grep and git-diff verify (one removed, two added UI-SPEC lines cumulative)"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-10-12
status: complete
---

# Phase 25 Plan 20: Focus after a keyboard toggle removes its card Summary

**A keyboard chip toggle that removes its card now lands focus on a chip label of the fallback anchor card (or the `Your artists` heading when nothing survives), detected via click `detail === 0` and reported by `useScrollAnchor` through `useEffectEvent`; mouse clicks move no focus.**

## Performance

- **Duration:** about 10 min
- **Completed:** 2026-10-12T22:17Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- Closed G-25-12 (review WR-03): focus no longer falls to `<body>` for a keyboard user whose toggled card leaves.
- `TagChips` marks the label button `data-chip-label` and passes `e.detail === 0` as a third `toggleFilterTag` argument.
- `useScrollAnchor` records `viaKeyboard && el.contains(document.activeElement)` at capture and, after the scroll correction, calls the focus-lost event when the captured element is disconnected and focus fell to the document.
- The route resolves the target with constant selectors only (`[data-chip-label][aria-pressed="true"]`, then `[data-chip-label]`), falls back to the `tabIndex={-1}` heading, and focuses with `{ preventScroll: true }`.
- UI-SPEC focus table row `Toggled, card leaves` added, tagged `G-25-12, user decision 2026-10-12`.

## Task Commits

1. **Task 1: Enter on a leaving card's chip lands focus on the fallback card's pressed label** - `c9ea53c` (fix, tracer)
2. **Task 2: Heading fallback and hook gate tests** - `5ae0b5b` (fix)
3. **Task 3: UI-SPEC focus table row** - `e5c7d8e` (docs)

## RED evidence

Before the change, `vitest run app/routes/watchlist.test.tsx app/components/watchlist/TagChips.test.tsx` gave 4 failed:
- both route Enter cases failed `expect(element).toHaveFocus()` (focus on BODY after the card unmounted)
- both TagChips cases failed `toHaveBeenCalledWith(tag, label, ...)` because no third argument was passed

The mouse route case passed before the change, as expected, pinning the keyboard-only scope. Task 2's no-survivor route case and the six hook cases were written after the heading change (the hook gates were built in Task 1), so no separate RED run was recorded for them.

## Decisions Made
- `detail === 0` was chosen over pointerdown/keydown tracking: stateless, per event, no document listeners. A programmatic `click()` also reports 0, so the held-focus and focus-fell-to-document guards remain.
- `useEffectEvent` (already in locked React 19.2.6) keeps the layout effect keyed on `[key]` alone.
- The embedded SPA (`internal/webassets/build/client`) is not rebuilt here; that happens in plan 25-21.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None. Selectors are constant literals (T-25-53); focus moves only for a keyboard toggle with focus-fell-to-document (T-25-54); `useUrlParams.ts`, `components/ui`, `web/package.json`, and `web/pnpm-lock.yaml` unchanged since `9d3413f` (T-25-55).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 25-21 rebuilds the embedded SPA so the Go binary serves the new focus behaviour.

## Self-Check: PASSED

- Modified files exist; commits `c9ea53c`, `5ae0b5b`, `e5c7d8e` exist on the branch; `git rev-list --count 742f579..HEAD` = 3.
- Typecheck, `prettier --check`, and the full web suite (621 tests) pass; the unchanged-file guards and UI-SPEC diff (one removed, two added lines) verified.

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-12*
