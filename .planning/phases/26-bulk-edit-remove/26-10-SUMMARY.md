---
phase: 26-bulk-edit-remove
plan: 10
subsystem: ui
tags: [react, vitest, bulk-actions, preferences, popover, base-ui]

requires:
  - phase: 26-bulk-edit-remove
    provides: "PATCH /watchlist/bulk/preferences per-type set/unset delta (26-05); BulkActionBar with inline add-tag mode and entry-free TagCombobox (26-08, 26-09)"
provides:
  - "Vendored popover.tsx wrapping @base-ui/react/popover (w-80, text-heading font-semibold)"
  - "bulkUpdatePreferences and BulkAxisDelta in api.ts"
  - "TriState, EVENT_TYPE_ORDER, initialTriStates, cycleTriState, buildAxisDelta in bulkSelection.ts"
  - "Preferences popover editor in BulkActionBar; handleBulkPreferences in the route"
affects: [26-11, 26-12]

actuals:
  tokens: 10500
  tasks: 3
  commits: 3
plan_head_before: d9d073190ee6bcc3465b5c4f6e2252a88d35e20e

tech-stack:
  added: []
  patterns:
    - "Tri-state boxes are compared to their initial state; only moved types become set/unset, so untouched mixed values stay per-artist"
    - "base-ui checkboxes are span role=checkbox: tests assert aria-disabled, and query names by regex because the wrapping label's text joins the aria-label"

key-files:
  created:
    - web/app/components/ui/popover.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/lib/bulkSelection.ts
    - web/app/lib/bulkSelection.test.ts
    - web/app/components/watchlist/BulkActionBar.tsx
    - web/app/components/watchlist/BulkActionBar.test.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.bulk.test.tsx

key-decisions:
  - "Hand-wrote popover.tsx from the registry source (viewed via shadcn --dry-run --view) instead of running shadcn add: the registry item declares a bogus `cn` dependency and the CLI tried `pnpm add cn`"
  - "Add tag is disabled while the popover is open, which enforces the one-surface rule from the other side"

requirements-completed: [BULK-03, BULK-05]

coverage:
  - id: D1
    description: "Preferences popover opens with tri-state boxes from the effective selection; Apply sends one PATCH carrying only the changed types, then refreshes, toasts and clears the selection"
    requirement: BULK-03
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist bulk preferences mutes one type on a mixed selection with a one-type delta"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist bulk preferences sends only Deluxe when only Deluxe was toggled, in one request"
        status: pass
    human_judgment: false
  - id: D2
    description: "Tri-state machine and delta builder: mixed start cycles three ways, uniform start toggles, canonical order, returned-to-initial omitted"
    requirement: BULK-03
    verification:
      - kind: unit
        ref: "web/app/lib/bulkSelection.test.ts#initialTriStates / cycleTriState / buildAxisDelta"
        status: pass
    human_judgment: false
  - id: D3
    description: "Popover interaction contract: focus on Album, Apply gating, Cancel/Esc/outside dismissal, pending lock, failure keeps choices and focuses Apply, empty release set allowed, one-surface rules, Esc never clears"
    requirement: BULK-03
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/BulkActionBar.test.tsx#BulkActionBar preferences popover"
        status: pass
    human_judgment: false
  - id: D4
    description: "Outcomes: gone-skip description, failure retention with identical retry, too-many-ids toast, stickiness under the muted filter"
    requirement: BULK-05
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.bulk.test.tsx#Watchlist bulk preferences"
        status: pass
      - kind: unit
        ref: "web/app/lib/api.test.ts#bulkUpdatePreferences PATCHes the snake_case delta"
        status: pass
    human_judgment: false
  - id: D5
    description: "Popover fits and stays on screen at 375px; network panel shows one PATCH omitting the untouched mixed type"
    requirement: BULK-03
    human_judgment: true
    rationale: "Collision handling and the dash glyph need a real browser (UI-SPEC B6 at 375px); jsdom has no layout"

duration: 35min
completed: 2026-10-16
status: complete
---

# Phase 26 Plan 10: Bulk Preferences popover Summary

**The bar's Preferences popover edits release types and mutes across the whole selection with tri-state boxes, and Apply sends one `PATCH /watchlist/bulk/preferences` carrying only the types the user actually changed.**

## Accomplishments

- Vendored `popover.tsx` (Root, Trigger, Portal+Positioner+Popup content, Header, Title, Description) with the UI-SPEC overrides: `w-80` and `text-heading font-semibold`. `package.json` and the lockfile are untouched.
- `bulkUpdatePreferences` plus pure helpers: `initialTriStates` (checked / unchecked / indeterminate over the effective selection), `cycleTriState` (mixed start cycles three ways, uniform start toggles), `buildAxisDelta` (changed types only, canonical order).
- `BulkActionBar` gained the `Preferences` trigger before `Remove`, the controlled popover with focus on `Album`, Apply gated on a real difference, `Applying…` with every control locked, kept choices and refocused Apply on failure, and an Esc guard that never clears while the popover is open.
- `handleBulkPreferences` mirrors `handleBulkAddTag`: one request on the effective-ids snapshot, `markStickyMany`, `refresh()`, success toast with the gone-ids description, and the failure and too-many toasts.
- 32 new tests across helpers, popover contract, route outcomes, stickiness and the wire shape. Full web suite: 34 files, 750 tests, coverage 95.62 / 91.18 / 96.33 / 97.38.

## Task Commits

1. **Task 1: Mute one type on a mixed selection (tracer)** - `dfa025a` (feat)
2. **Task 2: Tri-state machine and popover contract** - `f797d7d` (test)
3. **Task 3: Preference outcomes, kept choices, stickiness** - `a515695` (test)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] shadcn CLI could not vendor the popover safely**
- **Found during:** Task 1
- **Issue:** `shadcn add popover --yes` resolved a dependency named `cn` (the registry file imports `{ cn } from "cn"`) and ran `pnpm add -- cn`, which failed because bare `pnpm` is not on PATH. Nothing changed on disk, and installing a package named `cn` would have been an unvetted install.
- **Fix:** Took the file content from `shadcn add popover --dry-run --view`, repointed the import to `~/lib/utils`, applied the two overrides, and wrote it by hand as the plan's fallback allows. No package was installed.
- **Files modified:** `web/app/components/ui/popover.tsx`
- **Commit:** `dfa025a`

**2. [Rule 1 - Test assumption] Checkbox names and disabled state in tests**
- **Found during:** Tasks 1 and 2
- **Issue:** Each box sits in a label, so its computed name is `Album release type Album`, and the plan's exact-name queries fail. base-ui checkboxes are `span role=checkbox`, so `toBeDisabled()` fails too.
- **Fix:** Tests query by `^name` regex (as the existing `PreferenceToggles` tests do) and assert `aria-disabled="true"`.
- **Commit:** `dfa025a`, `f797d7d`

**3. [Rule 1 - Test assumption] Ids are sent in visible order**
- **Found during:** Task 3
- **Issue:** The test expected `[1, 2, 3]`, but the effective selection is sent in visible (name) order, `[3, 1, 2]`.
- **Fix:** Corrected the expectation and noted why. Behavior unchanged.
- **Commit:** `a515695`

**4. [Rule 1 - Process] Amended a local commit after a missed typecheck failure**
- **Found during:** Task 2
- **Issue:** Two test literals were typed `string` instead of `TriState`; the chained gate command did not stop on the error and the commit went through.
- **Fix:** Typed the literals and amended the unpushed commit, `f797d7d`.

**TDD note:** Task 1 shipped the behavior with the tracer, so Tasks 2 and 3 pin existing behavior and have no failing RED commit. The first full run of the new tests failed on test assumptions only (items 2 and 3); no source behavior needed fixing in Tasks 2 or 3.

**Total deviations:** 4 (1 blocking, 3 test or process fixes). **Impact:** none on scope.

## Issues Encountered

- `make` and a web lint script are not available; web gates were run directly with pnpm.
- Full web suite takes over 120s here, so it was run in the background.

## Known Stubs

None.

## Threat Flags

None. T-26-36: the call goes through `apiFetch` (`X-Requested-With` asserted in `api.test.ts`). T-26-37: `buildAxisDelta` emits only moved types and the body has no full sets or notes; route tests assert the exact body. T-26-38: the effective-ids snapshot is sent and the initial states are computed over the same selection. T-26-SC: no package added, `package.json` and lockfile unchanged.

## Next Phase Readiness

26-11 can insert `Remove tag` between `Add tag` and `Preferences` under the same one-surface rule. The human UAT item (popover fit at 375px, dash glyph, single PATCH in the network panel) is left for verification.

## Self-Check: PASSED

`popover.tsx` exists; commits `dfa025a`, `f797d7d`, `a515695` exist; all acceptance greps pass; `web/package.json` and `web/pnpm-lock.yaml` unchanged.
