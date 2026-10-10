---
phase: 25-find-filter-watchlist-and-history
plan: 08
subsystem: web
tags: [watchlist, preference-filter, tag-chips, url-state, react, vitest]
requires:
  - phase: 25-04
    provides: "useUrlParams writer, sticky set, Clear filters deleting muted/custom via CLEAR_FILTER_KEYS"
  - phase: 25-07
    provides: "tags URL param, handleTagsChange, tagNamesRef, toolbar tag filter"
provides:
  - "watchlistView.ts: DEFAULT_RELEASE_TYPES, muted/custom view axes, hasMutedEvents, hasCustomReleaseTypes, AND composition in selectVisible and isFiltered"
  - "WatchlistToolbar: Show only fieldset with two aria-pressed toggles"
  - "TagChips: chip label is a Filter by button; data-chip-remove focus targeting; TagActions.filterTagIds and toggleFilterTag"
  - "Route: handleFlagChange for muted/custom, toggleFilterTag, filterTagIds set passed to each row"
affects: []
tech-stack:
  added: []
  patterns:
    - "Chip focus logic selects [data-chip-remove] so extra buttons inside a chip cannot shift the index"
    - "Boolean URL flags honor only the value 1; canonicalization effect rewrites anything else away with replace"
key-files:
  created: []
  modified:
    - web/app/lib/watchlistView.ts
    - web/app/lib/watchlistView.test.ts
    - web/app/components/watchlist/WatchlistToolbar.tsx
    - web/app/components/watchlist/WatchlistToolbar.test.tsx
    - web/app/components/watchlist/TagChips.tsx
    - web/app/components/watchlist/TagChips.test.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
key-decisions:
  - "Custom release types is a set comparison against the default four, so order and duplicates are ignored and an empty list counts as custom"
  - "The existing canonicalization effect owns muted and custom too; no separate cleanup effect"
  - "Chip label click reuses handleTagsChange (push, clear sticky, count announcement) and seeds tagNamesRef first so the toolbar chip has its name"
requirements-completed: [WLVW-05, WLVW-04, WLVW-06]
status: complete
commits: 4
plan_head_before: 3aea8c3ab3caa79d0d0b22f897661f833f681b24
actuals:
  tokens: 28000
  tasks: 2
  commits: 4
duration: 25 min
completed: 2026-10-10
coverage:
  - deliverable: "Has muted events and Custom release types toggles narrow the list, round-trip through the URL, keep focus, and AND-compose with name and tags"
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#Watchlist preference toggles (WLVW-05)"
        status: pass
      - kind: test
        ref: "web/app/lib/watchlistView.test.ts#muted and custom axes"
        status: pass
      - kind: test
        ref: "web/app/components/watchlist/WatchlistToolbar.test.tsx#WatchlistToolbar Show only"
        status: pass
    human_judgment: false
  - deliverable: "In-card edit keeps a card that stopped matching until the next filter, sort, or search change"
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#keeps a card whose last mute was just unchecked until the next filter change"
        status: pass
    human_judgment: false
  - deliverable: "Chip label toggles the shared tag filter without attach or detach; x still detaches; Phase 24 chip focus contract holds"
    verification:
      - kind: test
        ref: "web/app/components/watchlist/TagChips.test.tsx#chip label filter toggle (D-08)"
        status: pass
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#Watchlist card chip click-to-filter (D-08)"
        status: pass
    human_judgment: false
  - deliverable: "Scroll position holds on chip and toggle clicks in a 50+ artist list; pressed-toggle Check meets 3:1 against #27272a; 32-character label with Check truncates at 375px"
    verification: []
    human_judgment: true
    rationale: "Scroll restoration, measured contrast, and truncation at width are only observable in a browser; deferred to end-of-phase UAT per the plan's human-check"
---

# Phase 25 Plan 08: Preference toggles and chip click-to-filter Summary

The Watchlist gains `Show only` toggles for `Has muted events` and `Custom release types` (URL params `muted=1` / `custom=1`, AND-composed with name, tags, and sort), and each card's tag chip label becomes a one-click `Filter by {tag}` toggle that shares the toolbar's `tags` filter while the x remains the only detach.

## Accomplishments

- `watchlistView.ts` adds `DEFAULT_RELEASE_TYPES`, the `muted`/`custom` view fields (only `1` is honored), `hasMutedEvents`, and `hasCustomReleaseTypes` (set comparison). `selectVisible` ANDs both axes and `isFiltered` counts them, so `Clear filters` shows for a lone toggle.
- `WatchlistToolbar` renders a `Show only` fieldset between the tag filter and `Clear filters`: two default-height `Button`s, `outline` off and `secondary` on, `aria-pressed`, and a leading `Check` only when on.
- The route writes the flags with push, clears sticky cards, and schedules the count announcement. The existing canonicalization effect now also drops invalid `muted`/`custom` values with replace.
- `TagChips` focus now targets `[data-chip-remove]` (both confirmed and pending x buttons), so the new label buttons cannot shift focus on removal. All Phase 24 focus tests stayed green.
- Confirmed chips carry a `Filter by {tag}` button with `aria-pressed`; an active tag shows a `Check` and `ring-1 ring-inset ring-foreground/30`. Pending chips keep plain-text labels.
- The route passes `filterTagIds` and `toggleFilterTag` on `tagActions`; the label click reuses `handleTagsChange` and seeds `tagNamesRef` first.

## Task Commits

1. Task 1 (tracer): `d888568` feat(25-08): filter the Watchlist to muted events and custom release types
2. Task 2 refactor: `e9e8370` refactor(25-08): target chip focus by data-chip-remove and wire tag filter actions
3. Task 2 RED: `30cdd64` test(25-08): pin the chip label filter toggle and its separation from detach
4. Task 2 GREEN: `f7d460b` feat(25-08): let a card's tag chip label toggle the tag filter

## Deviations from Plan

### Auto-fixed Issues

None.

### Notes

**1. [Ordering] Focus fix and action wiring landed as a behavior-neutral refactor commit before RED**
- The plan puts the `data-chip-remove` fix first with `TagChips.test.tsx` green before touching markup. Making `filterTagIds` and `toggleFilterTag` required on `TagActions` forced the route and existing test helpers to change at the same time, so that step became its own `refactor` commit. The full chip and route suites were green on it.

**2. [Test harness] Tailwind class ordering**
- `prettier-plugin-tailwindcss` reorders the active-ring classes, so the test asserts each of `ring-1`, `ring-inset`, `ring-foreground/30` on the badge classList rather than one joined string.

**3. [Test harness] Checkbox accessible names**
- The mute checkbox name includes its visible label, so the route test queries it with `/^Mute guest feature alerts/`.

**4. [Unresolved by design] WLVW-05 edge row and sticky chip deselect**
- Per the plan's flagged assumptions, the WLVW-05 edge row stays unresolved; empty sets, set equality, and AND composition are pinned by unit tests. When a chip click deselects a tag on a card that was visible only because it was sticky, the card can leave the view and focus is left to the browser. Flagged for UI review.

**Total deviations:** 0 auto-fixed, 4 notes. **Impact:** none on scope.

## Verification

- `corepack pnpm test` (full web suite with coverage): 30 files, 566 tests passed; coverage 95.18 / 90.57 / 95.75 / 97.18
- `corepack pnpm run typecheck`: exit 0
- `prettier --check "**/*.{ts,tsx}"`: clean
- Acceptance greps: `Has muted events` x1 and `Custom release types` x1 in the toolbar, `aria-pressed` present, `export function hasCustomReleaseTypes` x1, `data-chip-remove` x3, `Filter by ` x1, `toggleFilterTag` x2 in `TagChips.tsx`
- Pending human check: scroll position on 50+ artists, adding an artist under an active toggle, `Check` contrast against `#27272a`, and a 32-character tag with `Check` at 375px. Deferred to end-of-phase UAT.

## Known Stubs

None.

## Threat Flags

None. T-25-23 (label only calls `toggleFilterTag`; tests assert `attachTag`/`detachTag` are never called from it; distinct names and hover fills), T-25-24 (only `1` honored, anything else rewritten away with replace), and T-25-25 (tag names only as JSX text and escaped attributes) are mitigated as planned.

## Self-Check: PASSED

- FOUND commits d888568, e9e8370, 30cdd64, f7d460b
- FOUND modified files listed in key-files
- All task acceptance criteria re-run and passing
