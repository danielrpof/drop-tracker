---
phase: 26-bulk-edit-remove
plan: 03
subsystem: ui
tags: [react, react-router, base-ui, vitest, bulk-actions, watchlist]

requires:
  - phase: 26-bulk-edit-remove
    provides: "POST /watchlist/bulk/delete answering {removed, already_gone}; 400 bulk_too_many_ids carrying max (plan 26-01)"
provides:
  - "bulkRemoveWatchlist / BulkRemoveOutcome in api.ts"
  - "formatNameList and formatAlreadyRemoved in format.ts"
  - "Always-visible per-card Select checkbox with Shift captured on pointer-down"
  - "BulkActionBar: sticky region with count, Remove and Clear"
  - "Route-owned selectedIds, derived effective selection, route-hosted destructive ConfirmDialog, close-then-clear success order"
  - "showBulkFailure(error, fallback) with the bulk_too_many_ids branch, reusable by later bulk actions"
affects: [26-04, 26-06, 26-08, 26-10, 26-11, 26-12]

actuals:
  tokens: 6600
  tasks: 2
  commits: 3
plan_head_before: fe4ad05bc627c81f86bffaccfb9fe3d86ad7da8d

tech-stack:
  added: []
  patterns:
    - "Route hosts the confirm dialog; confirmRemove only records an outcome in a ref, and the dialog's close handler applies it (close, then clear)"
    - "Effective selection = visible.filter(selectedIds.has), never trimming selectedIds"
    - "toast.success called with one argument when there is no description so call assertions stay exact"

key-files:
  created:
    - web/app/components/watchlist/BulkActionBar.tsx
    - web/app/routes/watchlist.bulk.test.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/lib/format.ts
    - web/app/lib/format.test.ts
    - web/app/components/watchlist/WatchlistRow.tsx
    - web/app/routes/watchlist.tsx

key-decisions:
  - "confirmRemove never throws or mutates selection/entries; the outcome rides removeOutcomeRef and is applied in handleRemoveDialogOpenChange(false) (U1)"
  - "bulkInFlight resets on every dialog close, success, failure or cancel"
  - "Shift state is captured in onPointerDownCapture on the label and reset on keydown, so keyboard toggles never range (RESEARCH A1/F5)"

requirements-completed: [BULK-01, BULK-04, BULK-05]

coverage:
  - id: D1
    description: "Every card has a Select {artist} checkbox; checking cards mounts the Selection actions bar with the visible-selection count"
    requirement: BULK-01
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#removes two checked cards with one confirmed request"
        status: pass
    human_judgment: false
  - id: D2
    description: "Remove opens a count-stating destructive dialog with names and the notes/tags/pending-alert/seed-mode disclosure, singular and plural"
    requirement: BULK-04
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#words the dialog for a single artist"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#lists three names and a remainder for fourteen selected artists"
        status: pass
    human_judgment: false
  - id: D3
    description: "One POST removes the selection; cards disappear, selection clears, list refreshes in place; gone ids are counted and noted in the toast with no Undo"
    requirement: BULK-05
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#counts already-gone ids in the toast, which carries no action"
        status: pass
      - kind: unit
        ref: "web/app/lib/api.test.ts#bulkRemoveWatchlist POSTs the ids to /watchlist/bulk/delete"
        status: pass
    human_judgment: false
  - id: D4
    description: "Failure, id-cap rejection and Cancel keep the selection and remove nothing"
    requirement: BULK-04
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#keeps the cards and selection when the request fails"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#reports the server's id cap and keeps the selection"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#sends nothing on Cancel and keeps the selection"
        status: pass
    human_judgment: false
  - id: D5
    description: "Three 60-character unbroken names wrap inside the confirm description with the disclosure sentence fully visible"
    verification: []
    human_judgment: true
    rationale: "Long-text wrapping is real-browser layout that jsdom cannot show (UI-SPEC backstop B7 long-text); not run here"

duration: 8min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 03: Bulk remove client Summary

**Per-card selection checkboxes, a sticky count bar, and a route-hosted destructive confirm that removes the whole selection in one POST /watchlist/bulk/delete and refreshes in place.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-10-16T22:18:56Z
- **Completed:** 2026-10-16T22:27:00Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Tracer: check two cards, confirm the count-stating dialog, one request removes both, bar unmounts, list refreshes with no skeleton flash.
- `formatNameList` / `formatAlreadyRemoved` and `bulkRemoveWatchlist` with unit tests (including the CSRF header).
- Every remove copy line pinned verbatim from UI-SPEC: singular and plural dialogs, 14-name remainder, gone note, no `action` on the toast, generic and too-many failures, Cancel, Clear.

## Task Commits

1. **Task 1 (tracer): check cards and bulk-remove in one confirmed request** - `5d4ad8c` (feat)
2. **Task 2 RED: pin remove copy, gone note and failure paths** - `b35d2e0` (test)
3. **Task 2 GREEN: server id cap branch in showBulkFailure, 14-card test timeout** - `1dfd1ed` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] 14-card route test exceeded the 5s default under full-suite load**
- **Found during:** Task 2
- **Issue:** Selecting fourteen cards re-renders heavy rows each time; userEvent clicks timed out, and fireEvent still took about 16s in the full suite.
- **Fix:** Selected cards with `fireEvent.click` and gave that one test a 30s timeout.
- **Files modified:** web/app/routes/watchlist.bulk.test.tsx
- **Commit:** 1dfd1ed

**Total deviations:** 1 auto-fixed (1 blocking). **Impact:** test-only, no behavior change.

## Known Stubs

None. `showBulkFailure`'s fallback and `handleSelectChange` ignoring the Shift flag are intentional seams for plan 26-04.

## Issues Encountered

None beyond the test timeout above.

## Human Check Outstanding

Plan Task 2 `<human-check>` (60-character names wrapping in the confirm description) needs a real browser against the dev server. Not run; carried to UAT.

## Next Phase Readiness

Ready for 26-04 (selection model: `Select all visible`, hidden-selection rules, Shift ranges, in-flight locking). The embedded SPA is not rebuilt (plan 26-12).

## Self-Check: PASSED

- Created files exist: BulkActionBar.tsx, watchlist.bulk.test.tsx, this SUMMARY.
- Commits 5d4ad8c, b35d2e0, 1dfd1ed exist; `git rev-list --count` from plan_head_before gives 3.
- Typecheck, prettier check and full web suite (633 tests, coverage thresholds met) pass.
