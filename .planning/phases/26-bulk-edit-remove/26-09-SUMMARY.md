---
phase: 26-bulk-edit-remove
plan: 09
subsystem: ui
tags: [react, vitest, bulk-actions, tags, combobox]

requires:
  - phase: 26-bulk-edit-remove
    provides: "BulkActionBar inline add-tag mode, TagCombobox optional label/copy props, tagsOnEvery (26-08)"
provides:
  - "Entry-free TagCombobox: inputLabel and alreadyOnMessage required, entry prop removed"
  - "TagChips passes the card's unchanged strings"
  - "tagsOnEvery cases and inline add-tag mode cases (labels, Esc/Cancel focus, pending, failure, remember-on-applied)"
affects: [26-10, 26-11, 26-12]

actuals:
  tokens: 6500
  tasks: 2
  commits: 2
plan_head_before: c5de8d4175c05e762289914dde216eee4032d6c8

tech-stack:
  added: []
  patterns:
    - "Bar tests query controls outside the open combobox popup with hidden:true, getByText or getByTitle, because the popup aria-hides the rest of the page"

key-files:
  created: []
  modified:
    - web/app/components/watchlist/TagCombobox.tsx
    - web/app/components/watchlist/TagCombobox.test.tsx
    - web/app/components/watchlist/TagChips.tsx
    - web/app/lib/bulkSelection.test.ts
    - web/app/components/watchlist/BulkActionBar.test.tsx
    - web/app/routes/watchlist.bulk.test.tsx

key-decisions:
  - "Vocabulary memory is observed through the reopened picker (listTags mocked, then typing a prefix) rather than a provider spy, which exercises the real remember path"

patterns-established:
  - "Mutation-check a negative test (applied.length > 0 forced true) to prove it is not vacuous"

requirements-completed: [BULK-02]

coverage:
  - id: D1
    description: "TagCombobox is entry-free; the card and the bar both add tags through it with unchanged strings"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/TagCombobox.test.tsx"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#tags two selected artists from the inline picker in one request"
        status: pass
    human_judgment: false
  - id: D2
    description: "Inline add-tag mode: count-pluralized label, Esc/Cancel close without sending or clearing and focus Add tag, pending shows Adding... with the input disabled, failure refocuses, tag remembered only when applied is non-empty"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/BulkActionBar.test.tsx#BulkActionBar inline add-tag mode"
        status: pass
    human_judgment: false
  - id: D3
    description: "tagsOnEvery returns shared tags in the first entry's order, [] for empty or disjoint selections"
    requirement: BULK-02
    verification:
      - kind: unit
        ref: "web/app/lib/bulkSelection.test.ts#tagsOnEvery"
        status: pass
    human_judgment: false

duration: 14min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 09: Entry-free TagCombobox and inline add-tag pinning Summary

**`TagCombobox` no longer knows about `WatchlistEntry`: the card and the bulk bar both pass `inputLabel` and `alreadyOnMessage`, and the bar's inline add-tag mode is pinned by component tests for labels, Esc/Cancel focus, pending, failure and vocabulary memory.**

## Accomplishments

- Removed the `entry` prop, its fallback strings and the `WatchlistEntry` import from `TagCombobox`; `TagChips` passes `Add tag to {artist}` and `“{name}” is already on this artist`, so every existing card assertion passes unedited.
- Added four prop cases (accessible name, already-on copy, `disabled`, `inputClassName`).
- Added `tagsOnEvery` cases and ten inline-mode bar cases, including the Esc guard (idle Esc still clears; Esc in the picker calls neither `onAddTag` nor `onClear`).
- No behavior in `BulkActionBar.tsx` needed fixing; the tests pin what 26-08 shipped.

## Task Commits

1. **Task 1: Make TagCombobox entry-free (tracer)** - `f2bd371` (refactor)
2. **Task 2: Pin the bar's inline add-tag mode** - `55ded0e` (test)

## Verification

- Typecheck, `prettier --check`, and the full web suite pass: 34 files, 718 tests, coverage 95.53 / 91.1 / 96.19 / 97.27.
- Acceptance greps: `inputLabel: string` once and `WatchlistEntry` zero times in `TagComboboxProps`; `inputLabel=` once in `TagChips.tsx`.
- Mutation check: forcing `remember` to always run made `does not remember a tag that landed nowhere` fail; source restored.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test assumption] `inputClassName` lands on the input-group wrapper**
- **Found during:** Task 1
- **Issue:** `ComboboxInput` applies `className` to its `input-group` wrapper, not the `<input>`, so asserting the class on the input failed.
- **Fix:** The test asserts the class on the input's `[data-slot="input-group"]` ancestor. Behavior unchanged.
- **Commit:** `f2bd371`

**2. [Rule 3 - Blocking] Open popup aria-hides the bar's other controls**
- **Found during:** Task 2
- **Issue:** Role/name queries for `Cancel` and `Clear selection` fail while the combobox popup is open because the rest of the page is `aria-hidden` and hidden names compute empty.
- **Fix:** Queries use `getByText("Cancel")`, `getByTitle("Clear selection")`, and `hidden: true` for the absence checks (so they are not vacuous).
- **Commit:** `55ded0e`

**3. [Rule 3 - Blocking] Known cold-start flake in `watchlist.bulk.test.tsx`**
- **Found during:** Task 2 full-suite run (5170ms against the 5000ms default)
- **Fix:** Gave `removes two checked cards with one confirmed request` an explicit 30000ms timeout, as the 14-card test has.
- **Commit:** `55ded0e`

**TDD note:** Task 2 is marked `tdd`, but the behavior it pins shipped in 26-08, so there was no failing RED to commit; the mutation check above stands in as proof the new tests can fail.

**Total deviations:** 3 auto-fixed. **Impact:** none on scope.

## Issues Encountered

- `make` and a web lint script are not available; web gates were run directly with pnpm.

## Known Stubs

None.

## Threat Flags

None. T-26-46: both new props return plain strings rendered as JSX text or attribute values. T-26-47: asserted by the Esc test (no `onAddTag`, no `onClear`).

## Next Phase Readiness

26-10 can extend `BulkActionBar.tsx` and its tests with the inline-mode cases as a regression net.

## Self-Check: PASSED

Modified files exist; commits `f2bd371` and `55ded0e` exist.
