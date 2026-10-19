---
phase: 26-bulk-edit-remove
plan: 04
subsystem: ui
tags: [react, base-ui, vitest, bulk-actions, watchlist, selection]

requires:
  - phase: 26-bulk-edit-remove
    provides: "selectedIds, bulkInFlight, Shift pointer-down capture, route-hosted remove dialog, bulk test harness (plan 26-03)"
provides:
  - "bulkSelection.ts: pure effectiveSelection, selectAllState, toggleAllVisible, applyRange, SelectAllState"
  - "Vendored Checkbox renders indeterminate with the checked fill and a Minus glyph"
  - "Heading-row tri-state Select all visible next to the artist count"
  - "Shift+click ranges in visible order, anchor reset on search/filter/sort change and empty selection"
affects: [26-06, 26-08, 26-10, 26-11, 26-12]

actuals:
  tokens: 5300
  tasks: 3
  commits: 4
plan_head_before: 2c827cf3eb4c33ab778164a3cee3b1a0b3199d2f

tech-stack:
  added: []
  patterns:
    - "Effective selection is derived from the visible list; selectedIds is never trimmed (D-11)"
    - "Select-all ignores the box's own next value and applies toggleAllVisible"
    - "Selection helpers are pure with type-only api imports because route tests automock ~/lib/api"

key-files:
  created:
    - web/app/lib/bulkSelection.ts
    - web/app/lib/bulkSelection.test.ts
  modified:
    - web/app/components/ui/checkbox.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.bulk.test.tsx

key-decisions:
  - "Select all visible takes its accessible name from the wrapping label text only; an extra aria-label doubled the name"
  - "Disabled Base UI checkboxes expose data-disabled, not the disabled attribute, so tests assert that"

requirements-completed: [BULK-01]

coverage:
  - id: D1
    description: "Select all visible selects exactly the filtered view; a hidden selection is neither counted nor sent, and a bulk remove empties it"
    requirement: BULK-01
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#selects only the filtered view and removes exactly it"
        status: pass
    human_judgment: false
  - id: D2
    description: "Tri-state, uncheck keeps hidden selections, N=0 disabled, one-card never mixed, Clear empties hidden selections"
    requirement: BULK-01
    verification:
      - kind: unit
        ref: "web/app/lib/bulkSelection.test.ts#selectAllState"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#keeps a hidden selection when select all is checked then unchecked"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#empties hidden selections too when Clear is pressed"
        status: pass
    human_judgment: false
  - id: D3
    description: "Shift+click selects the inclusive range in visible order; anchor resets on filter or sort change"
    requirement: BULK-01
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#selects the range in visible order, not id order"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#resets the anchor when the sort changes"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every card checkbox, Select all visible and the dialog buttons are disabled while a bulk request is in flight; the filter input stays usable"
    requirement: BULK-01
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#locks every selection control while a bulk request is in flight"
        status: pass
    human_judgment: false
  - id: D5
    description: "Shift+click through the 44px label carries shiftKey in a real browser, and the range resets on filter or sort change"
    verification: []
    human_judgment: true
    rationale: "Whether a label-synthesized click carries shiftKey is real-browser behavior jsdom cannot show (RESEARCH A1, UI-SPEC B1 shift-click); not run here"

duration: 10min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 04: Select all visible and Shift ranges Summary

**A heading-row tri-state Select all visible, pure selection helpers that keep hidden selections inert, and Shift+click ranges over the visible order with an anchor that resets on any view change.**

## Performance

- **Duration:** ~10 min
- **Completed:** 2026-10-16T22:37:00Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Tracer: select SZA, filter to `dra`, Select all visible counts 1, clearing the filter shows 2 and `mixed`, removing sends exactly `[1]`, and SZA is unchecked afterward.
- `effectiveSelection(visible, selectedIds)` replaced 26-03's inline filter; `selectedIds` is never trimmed to visibility.
- `Checkbox` gained the indeterminate fill and Minus/Check glyph swap; existing PreferenceToggles tests pass unchanged.
- `applyRange` spans only the visible ids, so a range cannot reach a hidden artist (T-26-45).

## Task Commits

1. **Task 1 (tracer): select all visible and remove only what is shown** - `b1f55c0` (feat)
2. **Task 2: pin tri-state and hidden-selection rules** - `bc4ce65` (test)
3. **Task 3 RED: pin shift ranges, anchor reset and in-flight lock** - `9c6c74c` (test)
4. **Task 3 GREEN: shift ranges in visible order with anchor reset** - `9c0d6b6` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Existing 14-card test clicked every checkbox, now including Select all visible**
- **Found during:** Task 1
- **Issue:** `getAllByRole("checkbox")` picked up the new select-all box and toggled the selection back off.
- **Fix:** Scoped the query to `/^Select Artist/`.
- **Files modified:** web/app/routes/watchlist.bulk.test.tsx
- **Commit:** b1f55c0

**2. [Rule 1 - Bug] Duplicate accessible name on Select all visible**
- **Found during:** Task 1
- **Issue:** An `aria-label` on top of the wrapping label text produced "Select all visible Select all visible".
- **Fix:** Dropped the `aria-label`; the label text is the name.
- **Commit:** b1f55c0

**Total deviations:** 2 auto-fixed (test-only and a same-task correction). **Impact:** none on behavior.

Task 2 needed no production change, as the plan expected; its tests passed on first run against Task 1's helpers.

## Known Stubs

None.

## Human Check Outstanding

Plan Task 3 `<human-check>` (Shift+click through the 44px label in a real browser, then after a sort change) needs the dev server. Not run; carried to UAT with 26-03's wrapping check.

## Next Phase Readiness

Ready for 26-06 (attach `selectAllRef`, announce through `handleSelectAllVisible`) and the add-tag/mute plans that extend `bulkSelection.ts`. The embedded SPA is not rebuilt (plan 26-12).

## Self-Check: PASSED

- Created files exist: bulkSelection.ts, bulkSelection.test.ts, this SUMMARY.
- Commits b1f55c0, bc4ce65, 9c6c74c, 9c0d6b6 exist; `git rev-list --count` from plan_head_before gives 4.
- Typecheck, prettier check and the full web suite (666 tests, coverage thresholds met) pass.
