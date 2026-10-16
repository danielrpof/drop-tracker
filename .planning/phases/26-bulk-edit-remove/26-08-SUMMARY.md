---
phase: 26-bulk-edit-remove
plan: 08
subsystem: ui
tags: [react, vitest, bulk-actions, tags, combobox, sonner]

requires:
  - phase: 26-bulk-edit-remove
    provides: "POST /watchlist/bulk/tags/add (26-02), BulkActionBar, bulkSelection helpers, formatNameList/formatAlreadyRemoved, showBulkFailure and the focus/in-flight plumbing (26-03, 26-04, 26-06)"
provides:
  - "bulkAddTag client plus BulkSkip, BulkOutcome, BulkAddTagOutcome types"
  - "tagsOnEvery(selected): tags carried by every selected artist"
  - "TagCombobox optional inputLabel, alreadyOnMessage, disabled, inputClassName props (entry now optional)"
  - "BulkActionBar Add tag button and inline add-tag mode with vocabulary-aware commit wrapper"
  - "Watchlist handleBulkAddTag, markStickyMany, reportBulkAddOutcome (cap/gone toast matrix)"
affects: [26-09, 26-10, 26-11, 26-12]

actuals:
  tokens: 9000
  tasks: 2
  commits: 3
plan_head_before: 9302d86064cac9f25145ed44f4ffcd571466a34a

tech-stack:
  added: []
  patterns:
    - "The vocabulary-aware commit wrapper lives in BulkActionBar, which renders inside TagVocabularyProvider; the route cannot call useTagVocabulary"
    - "Cap-skip names are ordered by index in the ids snapshot sent, so toasts follow visible order rather than server id order"

key-files:
  created: []
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/lib/bulkSelection.ts
    - web/app/components/watchlist/TagCombobox.tsx
    - web/app/components/watchlist/BulkActionBar.tsx
    - web/app/components/watchlist/BulkActionBar.test.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.bulk.test.tsx

key-decisions:
  - "Cancel exits on pointer-down as well as click, because the input's blur would otherwise unmount Cancel before its click fires"
  - "The bar's picker onClose is ignored while a request is pending, since disabling the input blurs it"
  - "remember(tag) runs only when applied is non-empty, so an all-at-cap or all-gone outcome (tag.id 0) never enters the vocabulary"

patterns-established:
  - "reportBulkAddOutcome(outcome, sentIds) is the add-tag cap/gone copy that 26-11's Undo reuses"

requirements-completed: [BULK-02, BULK-05]

coverage:
  - id: D1
    description: "Selecting two artists and creating a tag from the bar's inline picker sends one POST, refreshes, clears the selection and returns focus to Select all visible"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#tags two selected artists from the inline picker in one request"
        status: pass
    human_judgment: false
  - id: D2
    description: "Toast matrix: plain success, partial cap warning, cap names in visible order, all-at-cap, gone appended, all-gone zero, with 8000ms warning duration"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist bulk add tag"
        status: pass
    human_judgment: false
  - id: D3
    description: "Request failure and bulk_too_many_ids keep the selection and the bar, return focus to Add tag, and fire the right error toast"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#keeps the selection and returns focus to Add tag when the request fails"
        status: pass
    human_judgment: false
  - id: D4
    description: "Affected cards stay visible past an active tag filter until the next filter change (sticky)"
    requirement: BULK-05
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#keeps an affected card visible past the tag filter until the next filter change"
        status: pass
    human_judgment: false
  - id: D5
    description: "bulkAddTag wire contract: POST /watchlist/bulk/tags/add with {ids,name} and the CSRF header"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/lib/api.test.ts#bulkAddTag POSTs the ids and name"
        status: pass
    human_judgment: false
  - id: D6
    description: "Long cap-skip toast wrapping and 32-character tag overflow in the sm:w-60 inline input at 1280px and 375px"
    requirement: BULK-02
    verification: []
    human_judgment: true
    rationale: "Text wrapping and input overflow need real layout; UI-SPEC backstop rows B4 long-text and B8 toast long-text"

duration: 11min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 08: Bulk add-tag client Summary

**The bar's `Add tag` opens the card's `TagCombobox` inline, sends one `POST /watchlist/bulk/tags/add` for the whole selection, marks applied cards sticky, refreshes in place, and reports cap and gone skips by name in visible order.**

## Performance

- **Duration:** ~11 min
- **Completed:** 2026-10-16
- **Tasks:** 2 (1 tracer, 1 TDD)
- **Files modified:** 8

## Accomplishments

- `bulkAddTag`, the shared `BulkOutcome` types and `tagsOnEvery` are in place. `TagCombobox` takes optional label, already-on copy, `disabled` and input-class props and keeps every card test green unedited.
- `BulkActionBar` gained `Add tag` (`TagPlus`), the inline picker row (second row below `sm`), `Cancel`, an `Adding…` pending state, lazy `ensureLoaded()` on first activation, and an Escape guard so Esc in the picker closes only the picker.
- `Watchlist.handleBulkAddTag` snapshots the effective ids, makes exactly one request, then `markStickyMany(applied)`, `refresh()` and the outcome toast. Failure routes through `showBulkFailure` and keeps the selection.
- `reportBulkAddOutcome` implements the full UI-SPEC matrix: success, partial cap warning, all-at-cap, gone appended, all-gone zero.

## Task Commits

1. **Task 1: Tag two selected artists from the bar's inline picker (tracer)** - `5742ce9` (feat)
2. **Task 2 RED: Pin the add-tag toast matrix, at-cap names and stickiness** - `0877757` (test)
3. **Task 2 GREEN: Report cap and gone skips in visible order** - `81445f0` (feat)

## Verification

- Typecheck, `prettier --check`, and the full web suite pass: 34 files, 700 tests, coverage 95.27 / 90.9 / 95.81 / 97.09.
- Acceptance greps: `/watchlist/bulk/tags/add` once in `api.ts`; `useTagVocabulary()` once in the bar and zero in the route; `is already on every selected artist` once in the bar; `every selected artist already has 10 tags.` once and `duration: 8000` twice in `watchlist.tsx`; `bulkAddTag` three times in `api.test.ts`.
- RED confirmed: 5 of the 9 new route cases failed before the toast matrix existed (the others covered behavior the tracer already shipped).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] BulkActionBar.test.tsx typed defaults missed the new required props**
- **Found during:** Task 1 typecheck
- **Issue:** `renderBar` builds a `BulkActionBarProps` object, so the new required `onAddTag` and `onSucceeded` props broke `tsc`. The plan expected the file to stay unedited, but its verify step names this exact failure.
- **Fix:** Added `onAddTag: vi.fn()` and `onSucceeded: vi.fn()` to the defaults; no case changed.
- **Files modified:** `web/app/components/watchlist/BulkActionBar.test.tsx`
- **Commit:** `5742ce9`

**2. [Rule 1 - Bug, test only] Gone-plus-cap test reused `Create “rap”` after the vocabulary learned `rap`**
- **Found during:** Task 2 GREEN
- **Issue:** After the first add in one test, `remember(tag)` correctly put `rap` in the vocabulary, so the second flow offers the existing option, not Create.
- **Fix:** The second flow picks the existing `rap` option.
- **Files modified:** `web/app/routes/watchlist.bulk.test.tsx`
- **Commit:** `81445f0`

**Total deviations:** 2 auto-fixed (1 blocking, 1 test bug). **Impact:** none on scope.

## Issues Encountered

- One full-suite run timed out the first test of `watchlist.bulk.test.tsx` (5086ms against the 5000ms default) under 34-file parallel load. It passed alone and on the next full run. This is a cold-start flake on a pre-existing test, not a regression; raising its timeout would be a separate change.
- Shell heredocs carrying curly-quote test bodies failed; the tests were applied with the Edit tool instead.
- `make` is not installed; web gates were run directly with pnpm.

## Known Stubs

None.

## Threat Flags

None. The new call goes through `apiFetch` (CSRF header asserted in `api.test.ts`), names render as plain JSX or sonner strings, and only the effective-selection snapshot is sent (asserted by the visible-order test: `[3, 1, 4, 2]`).

## Next Phase Readiness

26-09 can make `TagCombobox` entry-free (the card's strings are unchanged) and pin the inline mode's keyboard, pending and vocabulary interactions. The held-out visual rows (long cap-skip toast, 32-character tag in the inline input) still need a human pass against the dev server.

## Self-Check: PASSED

Modified files exist; commits `5742ce9`, `0877757` and `81445f0` exist.
