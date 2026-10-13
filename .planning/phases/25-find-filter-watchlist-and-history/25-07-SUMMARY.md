---
phase: 25-find-filter-watchlist-and-history
plan: 07
subsystem: web
tags: [watchlist, tag-filter, combobox, url-state, react, vitest]
requires:
  - phase: 25-04
    provides: "useUrlParams writer, sticky set, Clear filters that already deletes tags"
  - phase: 25-05
    provides: "param canonicalization effect and sort"
  - phase: 25-06
    provides: "sortTagsByName and the hardened combobox vendored parts"
provides:
  - "watchlistView.ts: tagIds axis, matchesAnyTag, tagFilterOptions, TagFilterOption, tags parse and canonicalization"
  - "WatchlistTagFilter: any-of multi-select on the vendored base-ui Combobox with chips"
  - "ComboboxChip removeLabel prop and contrast-safe remove button"
  - "Route tag-id lifecycle: first-load validation, Manage tags delete/merge sync, Back resurrection handling"
affects: [25-08]
tech-stack:
  added: []
  patterns:
    - "Tag names cached by id in a ref so a chip outlives its last carrier"
    - "Retired-id map (deleted -> null, merged -> target) consulted by the canonicalization effect so Back cannot resurrect a dead tag"
    - "Filter options derived from the loaded payload; GET /tags is never called"
key-files:
  created:
    - web/app/components/watchlist/WatchlistTagFilter.tsx
    - web/app/components/watchlist/WatchlistTagFilter.test.tsx
  modified:
    - web/app/components/ui/combobox.tsx
    - web/app/components/watchlist/WatchlistToolbar.tsx
    - web/app/components/watchlist/WatchlistToolbar.test.tsx
    - web/app/lib/watchlistView.ts
    - web/app/lib/watchlistView.test.ts
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
key-decisions:
  - "Esc on a closed popup makes base-ui clear every chip through onValueChange (reason escape-key); the filter cancels that reason so a second Esc does nothing"
  - "Multiple mode already keeps the popup open on pick in jsdom (A2 held), so the item-press onOpenChange cancel is a defensive pin rather than a fix"
  - "A1 held: base-ui's default filter is a case-insensitive contains, so no explicit filter prop was added"
  - "tags in the URL is written through URLSearchParams, so commas serialize as %2C (?tags=5%2C9); the decoded value is the ascending comma-separated id list"
requirements-completed: [WLVW-04, WLVW-06]
status: complete
commits: 5
plan_head_before: 27d472fe2dd9026e0d194a355762bf969cdb172e
actuals:
  tokens: 30000
  tasks: 3
  commits: 5
duration: 40 min
completed: 2026-10-10
coverage:
  - deliverable: "Tags: any of narrows the list to carriers (OR), empty selection is no constraint, options come from the payload with one request"
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#Watchlist tag filter (WLVW-04)"
        status: pass
    human_judgment: false
  - deliverable: "Multi-select contract: stays open and focused after a pick, deselect, Backspace and chip x removal, Esc, placeholder, sorted chips, counts, empty copy"
    verification:
      - kind: test
        ref: "web/app/components/watchlist/WatchlistTagFilter.test.tsx"
        status: pass
    human_judgment: false
  - deliverable: "Tag axis in the view model (parse cap and shape, canonicalization, matchesAnyTag, tagFilterOptions ordering, selectVisible AND)"
    verification:
      - kind: test
        ref: "web/app/lib/watchlistView.test.ts#tag axis"
        status: pass
    human_judgment: false
  - deliverable: "First-load validation, Manage tags delete/merge sync, Back resurrection, detach and refresh leave the filter alone"
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#Watchlist tag filter consistency (D-13)"
        status: pass
    human_judgment: false
  - deliverable: "Chip x meets 3:1 contrast against the chip fill, and a 32-character tag truncates at 375px"
    verification: []
    human_judgment: true
    rationale: "Contrast and truncation at width are only measurable in the rendered app; deferred to end-of-phase UAT per the plan's human-check"
---

# Phase 25 Plan 07: Watchlist tag filter Summary

The Watchlist gains a `Tags: any of` multi-select (vendored base-ui Combobox with chips) fed by tags the loaded artists carry, narrowing the list with OR semantics and a `tags=` URL param that stays truthful across first load, Manage tags delete and merge, Back, detaches, and refreshes.

## Accomplishments

- `watchlistView.ts` owns the tag axis: `parseWatchlistParams` reads at most 200 `tags` tokens matching `^[1-9]\d*$` (ascending, de-duplicated), `canonicalWatchlistParams` rewrites or deletes the param, `matchesAnyTag` and `tagFilterOptions` (collator then id order) are pure, `selectVisible` ANDs the tag axis with name, and `isFiltered` counts it.
- `WatchlistTagFilter` is built only from vendored parts in `multiple` mode (no TagCombobox, no create option). Chips are `h-6` neutral and truncate with `title`; options show a `tabular-nums` carrier count; placeholder `All tags` only while empty; `No tags yet` and `No tags match` copy through `ComboboxEmpty`.
- `ComboboxChip` takes `removeLabel` (the remove button's aria-label) and uses `text-muted-foreground hover:text-foreground` instead of 50% opacity.
- The route caches tag names by id, so a filter chip survives its last carrier leaving. Picking pushes `tags`, clears sticky, and schedules the count announcement.
- First successful load drops URL tag ids no artist carries (replace) exactly once. Manage tags delete and merge rewrite `tags` with replace (merge swaps source for target, de-duplicated). A retired-id map lets the canonicalization effect drop or swap a deleted or merged id that Back brings into the URL. `refresh()` and chip detach never touch `tags`.

## Task Commits

1. Task 1 (tracer): `438144f` feat(25-07): filter the Watchlist by any of several tags
2. Task 2 RED: `a10a412` test(25-07): pin the tag filter's keep-open, keys, and empty copy
3. Task 2 GREEN: `5267ea8` feat(25-07): keep the tag filter open on pick and explain empty results
4. Task 3 RED: `4fb8ef1` test(25-07): pin tag filter validation, Manage tags sync, and Back
5. Task 3 GREEN: `beef586` feat(25-07): keep the tag filter consistent with Manage tags, Back, and first load

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Esc on a closed popup cleared every selected tag**
- **Found during:** Task 2 (RED test "keeps the selection when Esc is pressed with the popup closed")
- **Issue:** base-ui fires `onValueChange` with reason `escape-key` and an empty list when Escape is pressed in the input with the popup closed, so the second Esc wiped the filter. The plan's "a second Esc does nothing" requires otherwise.
- **Fix:** `onValueChange` cancels the `escape-key` reason before calling `onChange`.
- **Files modified:** `web/app/components/watchlist/WatchlistTagFilter.tsx`
- **Commit:** `5267ea8`

### Notes

**2. [Assumption result] A1 and A2 both held**
- The default filter is a case-insensitive contains, and multiple mode keeps the popup open on pick without help. The `item-press` close cancel (required by the plan's acceptance grep) is kept as a defensive pin; no controlled `open` state or explicit `filter` was needed.

**3. [Test harness] Accessible name of the chips input**
- The input carries both `aria-labelledby="watchlist-tags-label"` and `aria-label="Filter by tags, any of"` as the UI-SPEC specifies, so its computed role name is `Tags: any of`. Tests locate it with `getByLabelText("Filter by tags, any of")`.

**4. [Test harness] Open popup hides the page from role queries**
- While the combobox or Manage tags dialog is open, the rest of the page is aria-hidden, so route tests that read cards or filter chips behind them query with `hidden: true`.

**5. [Scope] `%2C` in the URL**
- `URLSearchParams` serializes the comma, so multi-tag URLs read `?tags=5%2C9`. A hand-typed `?tags=5,9` is normalized to that form with a single replace on load. Single-tag URLs (`?tags=3`) are unaffected.

**6. [TDD ordering] View-model cases and the detach/refresh route cases were green on first run**
- The Task 2 view-model tests and Task 3's detach-keeps-filter and add-refresh-keeps-filter tests pin behavior already delivered by Task 1's tracer and the 25-04 sticky design. The genuinely RED tests (empty copy, Esc clears chips, first-load validation, delete/merge sync, Back) drove the GREEN commits.

**Total deviations:** 1 auto-fixed, 5 notes. **Impact:** none on scope.

## Verification

- `corepack pnpm test` (full web suite with coverage): 30 files, 539 tests passed
- `corepack pnpm run typecheck`: exit 0
- `prettier --check "**/*.{ts,tsx}"`: clean
- Acceptance greps: `opacity-50 hover:opacity-100` x0, `removeLabel` x3 in `combobox.tsx`, `multiple` present and `TagCombobox` x0 in `WatchlistTagFilter.tsx`, `id="watchlist-tags-label"` x1, `item-press` present, `No tags yet — add one with` x1, `No tags match` x1, `tagIdsValidatedRef` x4, `retiredTagIdsRef` x4
- Pending human check: measure the toolbar chip x against the `#27272a` chip fill (expected about 5.8:1), confirm a selected tag survives an add-and-refresh in the running app, and eyeball a 32-character tag at 375px. Deferred to end-of-phase UAT.

## Known Stubs

None.

## Threat Flags

None. T-25-20 (200-token cap, shape filter, first-load intersection), T-25-21 (tag names only as JSX text and escaped attributes) and T-25-22 (filter never calls attach/detach/delete; Manage tags only reports) are mitigated as planned.

## Self-Check: PASSED

- FOUND: web/app/components/watchlist/WatchlistTagFilter.tsx and WatchlistTagFilter.test.tsx
- FOUND commits 438144f, a10a412, 5267ea8, 4fb8ef1, beef586
- All task acceptance criteria re-run and passing
