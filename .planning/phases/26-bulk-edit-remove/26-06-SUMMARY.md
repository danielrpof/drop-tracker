---
phase: 26-bulk-edit-remove
plan: 06
subsystem: ui
tags: [react, base-ui, vitest, a11y, focus, live-region, bulk-actions, watchlist]

requires:
  - phase: 26-bulk-edit-remove
    provides: "Selection model, Select all visible, Shift ranges, in-flight lock (plan 26-04); bar, route-hosted remove dialog, close-then-clear order (plan 26-03)"
provides:
  - "ConfirmDialog.finalFocus typed as the AlertDialog popup's full finalFocus union"
  - "Route focus machinery: resolveFallbackTarget, pendingFocus effect, clearSelection, removeCloseKindRef; finishBulkSuccess sets pendingFocus"
  - "Live announcements for selection start, select-all, clear, and a selected-count suffix on the filter message"
  - "Bar Escape-to-clear, removeButtonRef, and the --bulk-bar-clearance measurement"
  - "Toaster offsets, html scroll-padding-bottom and page bottom padding bound to the clearance"
affects: [26-08, 26-10, 26-11, 26-12]

actuals:
  tokens: 14000
  tasks: 3
  commits: 5
plan_head_before: ae2a6a902115f2639f193383d5462f6c6b539996

tech-stack:
  added: []
  patterns:
    - "finalFocus function reads a ref set in the close handler, so the target is chosen at close time (U1)"
    - "pendingFocus effect gated on bar unmounted and nothing in flight is the backstop for the same target"
    - "Bar publishes one CSS custom property on <html>; page padding, Toaster offsets and scroll padding all consume it with a 0px fallback"

key-files:
  created:
    - web/app/components/watchlist/BulkActionBar.test.tsx
  modified:
    - web/app/components/common/ConfirmDialog.tsx
    - web/app/components/common/ConfirmDialog.test.tsx
    - web/app/components/watchlist/BulkActionBar.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.bulk.test.tsx
    - web/app/root.tsx
    - web/app/app.css

key-decisions:
  - "Success focus path: base-ui evaluates the finalFocus function after the selection-clearing commit in jsdom, so it returns resolveFallbackTarget() directly. The pendingFocus effect also focuses the same target and is a verified backstop (disabling either one alone still passes the success focus tests)."
  - "Announcements are computed from the pre- and post-change effective selection counts in handleSelectChange, no longer inside a functional setState updater"

requirements-completed: [BULK-01, BULK-04, BULK-05]

coverage:
  - id: D1
    description: "Every remove outcome and Clear leaves focus on the UI-SPEC target, never <body>"
    requirement: BULK-05
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist focus after bulk actions"
        status: pass
      - kind: unit
        ref: "web/app/components/common/ConfirmDialog.test.tsx#moves focus to the element a function finalFocus returns on close"
        status: pass
    human_judgment: false
  - id: D2
    description: "Selection changes are announced per the UI-SPEC table; Esc in the bar clears and refocuses"
    requirement: BULK-01
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist selection announcements"
        status: pass
    human_judgment: false
  - id: D3
    description: "The bar publishes and removes --bulk-bar-clearance; page padding, Toaster offsets and html scroll-padding consume it"
    requirement: BULK-04
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/BulkActionBar.test.tsx#BulkActionBar clearance variable"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist bar clearance"
        status: pass
    human_judgment: false
  - id: D4
    description: "Last card, toasts and focused checkboxes clear the bar at 1280px and 375px; one-row bar with full count at 375px"
    verification: []
    human_judgment: true
    rationale: "Real layout, scroll padding, toast stacking and viewport fit are not computed by jsdom (UI-SPEC backstop B3); not run here"

duration: 12min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 06: Focus, announcements and bar clearance Summary

**Focus lands on the next useful control after every bulk remove or Clear, selection changes are announced through the one status region, and the sticky bar publishes its measured height so the page, toasts and focused checkboxes never sit behind it.**

## Performance

- **Duration:** ~12 min
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Tracer: success focuses `Select all visible`, then `Clear filters` (filtered to zero), then the `Watchlist` h1 (empty watchlist); Cancel, Esc and a failed request return focus to the bar's `Remove`; Clear focuses `Select all visible`.
- `ConfirmDialog.finalFocus` accepts base-ui's full union with existing ref callers unchanged (ManageTagsDialog tests pass).
- Announcements: first selection, `Selected {N} visible artist{s}.`, `Selection cleared.` (Clear, bar Esc, select-all deselect), and ` {k} selected.` appended to the 500ms count message. Bulk success never announces.
- `--bulk-bar-clearance` (height + bottom inset + 8px) measured with an optional ResizeObserver and removed on unmount; Toaster, `html` scroll padding and the page container read it.

## Task Commits

1. **Task 1 (tracer): land focus after bulk remove and clear** - `f90f22d` (feat)
2. **Task 2 RED: pin announcements and Esc-to-clear** - `7e0b7f1` (test)
3. **Task 2 GREEN: announce selection changes and clear on Esc** - `924c564` (feat)
4. **Task 3 RED: pin bar markup and page clearance variable** - `ac3d5fb` (test)
5. **Task 3 GREEN: keep page, toasts and focus clear of the bar** - `deddf81` (feat)

## Deviations from Plan

### Auto-fixed Issues

None to code. Two test-scope notes:

**1. The "Shift range that starts the selection announces" behavior line has no dedicated test.**
- A Shift range needs an anchor, and the anchor is cleared whenever the selection is empty or the view changes, so a range can never take the effective selection from 0 to at least 1 through the UI.
- `handleSelectChange` still announces from the generic before/after count comparison, which the first-card test exercises. No behavior gap.

**2. The success focus path is covered twice.** The `finalFocus` function and the `pendingFocus` effect both resolve the same target; each was verified alone to keep the tests green.

**Total deviations:** 0 auto-fixed. **Impact:** none.

## Known Stubs

None.

## Human Check Outstanding

Task 3 `<human-check>` (30+ artists at 1280px and 375px: last card clear of the bar, focused checkbox not behind it, failure toast stacks above the bar, single-row icon-only bar with a 3 to 4 digit count and no horizontal scroll) needs a real browser. Not run; carried to UAT. The embedded SPA is not rebuilt (plan 26-12).

## Next Phase Readiness

Plans 26-08, 26-10 and 26-11 can call `finishBulkSuccess()` (sets pendingFocus) and extend the bar's Escape guard (a comment marks the spot).

## Self-Check: PASSED

- Created files exist: BulkActionBar.test.tsx, this SUMMARY.
- Commits f90f22d, 7e0b7f1, 924c564, ac3d5fb, deddf81 exist; `git rev-list --count` from plan_head_before gives 5.
- Typecheck, prettier check and the full web suite (34 files, 689 tests, coverage thresholds met) pass.
