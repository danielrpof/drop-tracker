---
phase: 26-bulk-edit-remove
plan: 11
subsystem: ui
tags: [react, vitest, bulk-actions, tags, undo, base-ui, combobox]

requires:
  - phase: 26-bulk-edit-remove
    provides: "POST /watchlist/bulk/tags/remove returning applied/carried/skipped (26-07); bulk add-tag client and server as the Undo restore path (26-02, 26-08); BulkActionBar with inline add-tag mode and Preferences popover (26-08, 26-10)"
provides:
  - "bulkRemoveTag and BulkRemoveTagOutcome in api.ts"
  - "removableTags and RemovableTag in bulkSelection.ts"
  - "Remove tag button and inline single-value remove-tag picker in BulkActionBar"
  - "handleBulkRemoveTag, undoRemoveTag and a restore variant of reportBulkAddOutcome in the route"
affects: [26-12]

actuals:
  tokens: 10000
  tasks: 3
  commits: 3
plan_head_before: 85b7fb206a8072316e9769faa6ea17b6afcc4139

tech-stack:
  added: []
  patterns:
    - "Picker owns its query and popup state and filters its own items with filter={null}, so the empty-state branch can tell no tags from no match"
    - "Undo outlives the render that built the toast, so it reads the live visible order through a ref"

key-files:
  created: []
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/lib/bulkSelection.ts
    - web/app/lib/bulkSelection.test.ts
    - web/app/components/watchlist/BulkActionBar.tsx
    - web/app/components/watchlist/BulkActionBar.test.tsx
    - web/app/components/watchlist/TagCombobox.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.bulk.test.tsx

key-decisions:
  - "Exported DISMISS_REASONS from TagCombobox instead of duplicating the base-ui close-reason set in the remove-tag picker"
  - "Bar focus-after-idle generalized to one ref naming the target button (add-tag or remove-tag) rather than a second boolean ref"
  - "Undo sends the applied ids unchanged; only the toast's cap-name ordering uses visible order, with hidden ids appended ascending"

requirements-completed: [BULK-02, BULK-05]

coverage:
  - id: D1
    description: "Remove tag opens an inline picker of carried tags with N-of-M counts; picking one sends one request, sticky-marks applied ids, refreshes, clears the selection and toasts with an 8000ms Undo"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist bulk remove tag removes one carried tag from the selection in one request"
        status: pass
    human_judgment: false
  - id: D2
    description: "Undo re-adds by name to exactly the applied ids in one request, leaves a newer selection alone, marks restored ids sticky, and shows restore, cap-warning, all-at-cap and failure toasts"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist bulk remove tag"
        status: pass
    human_judgment: false
  - id: D3
    description: "A 404 tag_not_found fails cleanly: failure toast, refresh, selection kept, focus on Remove tag; other failures skip the refresh"
    requirement: BULK-05
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist bulk remove tag fails cleanly with a refresh when the tag no longer exists"
        status: pass
    human_judgment: false
  - id: D4
    description: "Picker contract: label by count, contains filtering, both empty states, max-h-72 and truncate/shrink-0 classes, Esc and Cancel focus, pending lock, bar control order and one-surface rule"
    requirement: BULK-05
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/BulkActionBar.test.tsx#BulkActionBar inline remove-tag mode"
        status: pass
      - kind: unit
        ref: "web/app/lib/bulkSelection.test.ts#removableTags"
        status: pass
      - kind: unit
        ref: "web/app/lib/api.test.ts#bulkRemoveTag POSTs the ids and tag id"
        status: pass
    human_judgment: false
  - id: D5
    description: "32-character tag truncates with an ellipsis while the count stays visible at sm:w-60 and 375px; list caps near 288px and scrolls the highlighted option into view"
    requirement: BULK-05
    human_judgment: true
    rationale: "Truncation and listbox scrolling need real layout (UI-SPEC B5 long-text, U3 second row); jsdom has no layout"

duration: 40min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 11: Bulk remove tag with Undo Summary

**The bar's Remove tag picker lists only tags the selection carries with `{c} of {N}` counts, removes the picked one from every carrier in one request, and offers an 8000ms Undo that re-adds it by name to exactly the artists that lost it.**

## Accomplishments

- `bulkRemoveTag(ids, tagId)` posts `{ids, tag_id}` through `apiFetch`; `removableTags(selected)` returns the union of carried tags with carrier counts in `sortTagsByName` order.
- `BulkActionBar` gained `Remove tag` between `Add tag` and `Preferences`, a `remove-tag` mode, and an internal `RemoveTagPicker` (single-value base-ui `Combobox`, own contains filter, `max-h-72 overflow-y-auto`, truncating names, `shrink-0` counts, both empty states). Only one surface exists at a time; the bar now carries all five controls in order.
- `handleBulkRemoveTag` mirrors the add-tag handler: one request on the effective-ids snapshot, `markStickyMany(applied)`, `refresh()`, `Removed “tag” from n artists.` with `Undo` only when `applied` is non-empty, gone sentence as description, and a refresh on 404.
- `undoRemoveTag` sends one `bulkAddTag(applied, name)` without touching selection, in-flight state or the bar; `reportBulkAddOutcome` gained a `restore` variant for the titles.
- 30 new tests. Full web suite: 34 files, 780 tests, coverage 95.6 / 91.2 / 96.47 / 97.37.

## Task Commits

1. **Task 1: Remove one tag through the inline picker (tracer)** - `6dcd447` (feat)
2. **Task 2: Undo, 404 and stickiness outcomes** - `c7e2f8b` (test)
3. **Task 3: Picker options, counts, empty states and keyboard contract** - `f5cf372` (test)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test assumption] Undo invocation in tests**
- **Found during:** Task 2
- **Issue:** Wrapping `action.onClick()` in `act()` returned a promise into a sync `act`, which broke every later render in the file; sonner's `Action.onClick` also takes an event, so the typed cast failed typecheck.
- **Fix:** Call `onClick()` directly and cast through `unknown`.
- **Commit:** `c7e2f8b`

**2. [Rule 3 - Blocking] Shared close-reason set**
- **Found during:** Task 1
- **Issue:** The picker needs the same base-ui dismiss reasons as `TagCombobox`.
- **Fix:** Exported `DISMISS_REASONS` from `TagCombobox.tsx` rather than copying it.
- **Commit:** `6dcd447`

**TDD note:** Task 1's tracer shipped Undo and the 404 path alongside the picker, so Tasks 2 and 3 pin existing behavior and have no failing RED commit. Their first runs failed only on test-harness issues (item 1), never on source behavior.

**Total deviations:** 2 (1 test fix, 1 blocking). **Impact:** none on scope.

## Issues Encountered

- `make` is not installed; web gates were run directly with pnpm, each as a separate command with its exit code checked.
- The full web suite takes about two minutes here.

## Known Stubs

None.

## Threat Flags

None. T-26-39: tag names render only as JSX text and sonner string content. T-26-40: both calls go through `apiFetch` (`X-Requested-With` asserted in `api.test.ts`). T-26-41: Undo sends only `out.applied`; the route test asserts `([1, 2], "rap")`. T-26-42: a 404 gives the failure toast, a `refresh()` and a kept selection. No package added.

## Next Phase Readiness

26-12 can rebuild the embedded bundle and run the phase gate. The human UAT item (32-character tag truncation at 1280px and 375px, 20-tag list scrolling, second-row layout on phones) is left for verification.

## Self-Check: PASSED

Commits `6dcd447`, `c7e2f8b`, `f5cf372` exist; all acceptance greps pass (`/watchlist/bulk/tags/remove` 1, `tag_id: tagId` 1, `None of the selected artists have tags.` 1, `title="Remove tag"` 1, `max-h-72` 1, `every artist already has 10 tags.` 1, `Couldn't restore “` 2); `package.json` and lockfile untouched.
