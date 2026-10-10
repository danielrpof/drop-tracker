---
phase: 25-find-filter-watchlist-and-history
plan: 06
subsystem: web
tags: [history, tag-filter, combobox, empty-state, react, vitest]
requires:
  - phase: 25-02
    provides: "tag_id query param on GET /events"
  - phase: 25-03
    provides: "tag-aware watchlist cards and GET /tags consumers"
provides:
  - "listEvents({ tagId }) sending tag_id"
  - "sortTagsByName: base-collation tag ordering with id tie-break"
  - "History Tag combobox after Event type, fed by one GET /tags per mount"
  - "Combobox<T> hardening: max-h-72 scroll, scrollIntoView on highlight, max-w-64 truncated titled trigger, triggerLabel/triggerRef props"
  - "Tag trigger degraded labels (Loading tags…, Tag unavailable)"
  - "Tag-specific History empty state with Show all tags"
affects: [25-07, 25-09]
plan_head_before: c6c7e46e46a8d1a1ccf6887d24861adfb257ae73
actuals:
  tokens: 7700
  tasks: 3
  commits: 5
tech-stack:
  added: []
  patterns:
    - "Hand-rolled History Combobox<T> stays alongside the base-ui Combobox used on Watchlist (G7 add-alongside)"
    - "Optional-call scrollIntoView?.() so jsdom needs only a beforeEach stub"
key-files:
  created: []
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/lib/tags.ts
    - web/app/lib/tags.test.ts
    - web/app/components/history/HistoryFilters.tsx
    - web/app/components/history/HistoryFilters.test.tsx
    - web/app/routes/history.tsx
    - web/app/routes/history.test.tsx
key-decisions:
  - "Tag degraded trigger labels apply only when a tag is selected; with no tag selected the trigger always reads All tags, matching the plan's behavior block over its looser prose"
  - "showAllTags focuses the Tag trigger synchronously through tagTriggerRef, since the trigger never unmounts"
  - "Unknown tag name falls back to this tag in the route; that path is not UI-reachable until plan 25-09 moves filter state into the URL"
requirements-completed: [HIST-02]
duration: 14 min
completed: 2026-10-10
status: complete
commits: 5

coverage:
  - id: D1
    description: "Picking a tag on History sends tag_id composed with Artist and Event type; Load more carries it; GET /tags is requested once per mount"
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "web/app/routes/history.test.tsx#lists tags from GET /tags once, sorted by name, and a picked tag narrows the feed and survives Load more"
        status: pass
      - kind: unit
        ref: "web/app/lib/api.test.ts#sends tagId as tag_id"
        status: pass
    human_judgment: false
  - id: D2
    description: "Tag trigger never silently reads All tags while a selected tag is loading or failed; failed tags offer only All tags"
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "web/app/components/history/HistoryFilters.test.tsx#Tag control"
        status: pass
    human_judgment: false
  - id: D3
    description: "Shared combobox listbox scrolls (max-h-72) and keeps the highlighted option in view; trigger truncates with a title"
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "web/app/components/history/HistoryFilters.test.tsx#listbox hardening"
        status: pass
    human_judgment: false
  - id: D4
    description: "Visual truncation of a 32-character tag or long artist name inside the max-w-64 trigger at 375px"
    verification: []
    human_judgment: true
    rationale: "Pixel-level truncation at a 375px viewport is not asserted by jsdom; the plan marks it a held-out backstop check"
  - id: D5
    description: "Tag-specific empty state with precedence over unfiltered copy, under retention copy, and a Show all tags action that refocuses the Tag trigger"
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "web/app/routes/history.test.tsx#History tag empty state"
        status: pass
    human_judgment: false
---

# Phase 25 Plan 06: History Tag Filter Summary

**History gains a Tag combobox that sends tag_id alongside Artist and Event type, with a hardened scrollable/truncating shared combobox, honest loading/unavailable trigger labels, and a tag-specific empty state with a Show all tags way out.**

## Performance

- **Duration:** 14 min
- **Started:** 2026-10-10T16:36:00Z
- **Completed:** 2026-10-10T16:50:00Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- `listEvents` forwards `tagId` as `tag_id`; `sortTagsByName` orders tags with one module-level `Intl.Collator` and an id tie-break (consumed by 25-07).
- History fetches `GET /tags` once per mount, feeds a third `Tag` combobox (`All tags` first), and resets/refetches page one on a tag change; Load more keeps the tag.
- The module-private `Combobox<T>` now caps its listbox at `max-h-72` with scroll, scrolls the highlighted option into view, truncates a titled label inside a `max-w-64` trigger, and accepts `triggerLabel`/`triggerRef`.
- Empty state precedence is retention, then tag, then filtered, then unfiltered, with the two tag body variants and a secondary `Show all tags` button that clears only the tag and refocuses the trigger.

## Task Commits

1. **Task 1: Pick a tag on History and the feed narrows to it (tracer)** - `cac69cd` (feat)
2. **Task 2: Harden the shared combobox and Tag trigger labels (TDD)** - `9cb0416` (test, RED), `200b13f` (feat, GREEN)
3. **Task 3: Tag-specific empty state with Show all tags (TDD)** - `227222c` (test, RED), `fa9483f` (feat, GREEN)

**Plan metadata:** committed with this summary (docs).

## Files Created/Modified
- `web/app/lib/api.ts` - `tagId` param on `listEvents`
- `web/app/lib/tags.ts` - `sortTagsByName`
- `web/app/components/history/HistoryFilters.tsx` - Tag control, `Combobox<T>` hardening, degraded labels
- `web/app/routes/history.tsx` - single `listTags` mount effect, `tagId` in requests/deps, `emptyStateCopy` tag branch, `showAllTags`
- Matching `*.test.ts(x)` files extended for each behavior; existing HistoryFilters tests updated for the new required props

## Decisions Made
- Degraded trigger labels (`Loading tags…`, `Tag unavailable`) apply only while a tag is selected, so a failed GET /tags with no selection still reads `All tags`.
- `Show all tags` focuses the trigger synchronously through the ref; the trigger stays mounted so no effect or rAF is needed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Existing HistoryFilters tests needed the new required props and `tagId`**
- **Found during:** Task 1
- **Issue:** `HistoryFiltersValue.tagId` and the `tags`/`tagsStatus` props made the existing test file fail typecheck.
- **Fix:** Added `tagId: null` to fixtures and a shared `baseProps`.
- **Files modified:** `web/app/components/history/HistoryFilters.test.tsx`
- **Committed in:** `cac69cd`

### Other notes

- **Acceptance grep not literally satisfiable:** `grep -c 'max-h-72 overflow-y-auto'` expects adjacent classes, but `prettier-plugin-tailwindcss` orders them `max-h-72 min-w-full overflow-y-auto`. Both classes are present and asserted with `toHaveClass("max-h-72", "overflow-y-auto")`.
- **Tag-name fallback untested:** the `this tag` fallback is implemented in the route, but History filter state is `useState` here, so a selected tag always has a known name. It becomes reachable once 25-09 moves state to the URL; add the test there.
- **`sortTagsByName` test green at RED time:** the helper shipped in Task 1 as the plan specifies, so its Task 2 test case passed immediately. The other six Task 2 tests failed on assertions as intended.

---

**Total deviations:** 1 auto-fixed (1 blocking), 3 notes
**Impact on plan:** No scope creep.

## Issues Encountered
None.

## Known Stubs
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- 25-07 can reuse `sortTagsByName`; 25-09 can lift `HistoryFiltersValue` (now including `tagId`) into the URL and should add the unknown-tag-name fallback test.
- Held-out visual check for 375px truncation remains for UAT.

## Self-Check: PASSED
- All eight modified files exist; commits `cac69cd`, `9cb0416`, `200b13f`, `227222c`, `fa9483f` present.
- Acceptance greps pass (see note on the adjacency grep). Full web suite: 28 files, 445 tests pass; typecheck and prettier --check clean.

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-10*
