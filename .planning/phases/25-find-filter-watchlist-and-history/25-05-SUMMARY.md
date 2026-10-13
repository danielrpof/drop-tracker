---
phase: 25-find-filter-watchlist-and-history
plan: 05
subsystem: web
tags: [watchlist, sort, intl-collator, url-state, react, vitest]
requires:
  - phase: 25-01
    provides: "latest_release_date / next_release_date on GET /watchlist"
  - phase: 25-04
    provides: "Watchlist toolbar, useUrlParams, watchlistView.ts, sticky cards"
provides:
  - "SortKey, DEFAULT_SORT, SORT_OPTIONS, compareForSort, canonicalWatchlistParams in watchlistView.ts"
  - "selectVisible sorts after filtering"
  - "Sort by Select in WatchlistToolbar (vendored Select, six options)"
  - "handleSortChange (push, clears sticky, announces) and the D-13 canonicalization effect (replace)"
affects: [25-07, 25-08]
plan_head_before: 20bbfad2ee01eb47426a2358f72e31b57543e19b
actuals:
  tokens: 9500
  tasks: 2
  commits: 3
tech-stack:
  added: []
  patterns:
    - "One comparator per direction instead of reversing an ascending sort, so id ties and null-last survive"
    - "canonicalWatchlistParams owns its keys only; later plans extend it"
key-files:
  created: []
  modified:
    - web/app/lib/watchlistView.ts
    - web/app/lib/watchlistView.test.ts
    - web/app/components/watchlist/WatchlistToolbar.tsx
    - web/app/components/watchlist/WatchlistToolbar.test.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
key-decisions:
  - "Latest release compares raw text with code-unit < / > so 2024 sorts before 2024-05; nulls sort last in both directions"
  - "Sort change cancels any pending count announcement and announces Sorted by {label}. immediately"
  - "The canonicalization effect copies only the sort key into the mutated params, never touching keys it does not own"
requirements-completed: [WLVW-02, WLVW-03]
duration: 5 min
completed: 2026-10-10
status: complete
commits: 3
---

# Phase 25 Plan 05: Watchlist Sort Summary

Six-mode Sort by control on the Watchlist (name, date added, latest release, each in both directions) with artist-id-stable ties, null-last latest-release ordering, URL round-trip, invalid-sort cleanup, and a live announcement.

## What was built

- `watchlistView.ts`: `SortKey`, `DEFAULT_SORT` (`name-asc`), `SORT_OPTIONS` (UI-SPEC order, en-dash labels), `parseWatchlistParams` now yields `sort` (unknown maps to default), `compareForSort`, `canonicalWatchlistParams`, and `selectVisible` sorts after filtering.
- Name modes use a single module-level `Intl.Collator(undefined, { sensitivity: "base" })`; descending negates only the collator term. Date-added compares `Date.parse(created_at)`. Latest-release compares raw text and ignores `next_release_date`. Every mode ends on `artist_id` ascending; no `.reverse()` is used.
- `WatchlistToolbar`: `Sort by` label (`#watchlist-sort-label`) and vendored `Select` built from `SORT_OPTIONS`, with `sort` / `onSortChange` props.
- `watchlist.tsx`: `handleSortChange` (push write, `sort` omitted for the default, clears sticky, cancels the count timer, announces) and a `useEffect` that rewrites non-canonical params with replace.

## TDD / task flow

- Task 1 (tracer): `0c914ef` feat. Tracer re-verified end to end (route tests + typecheck) before expanding.
- Task 2 RED: `19bea20` test. Failed on the target assertions: `canonicalWatchlistParams is not a function`, bogus/default sort left in the URL, and missing `Sorted by` announcement. Comparator-table tests already passed because Task 1 implemented the comparators (expected for the tracer slice).
- Task 2 GREEN: `5fad3d0` feat. No refactor commit.

## Deviations from Plan

None - plan executed exactly as written. Existing `selectVisible` / `WatchlistToolbar` tests were updated for the new required `sort` field/prop (in-scope test maintenance; one sticky-order expectation changed from input order to sorted order).

## Known Stubs

None.

## Threat Flags

None. T-25-15 mitigated: `sort` is allow-listed against the six keys and rewritten away with replace when unknown.

## Verification

- `corepack pnpm test`: 28 files, 477 tests passed; `typecheck` and `prettier --check` clean.
- Human check (perceived stability on real data) remains for phase verification: flip each direction on a list with shared names / latest dates and `No releases yet` artists.

## Self-Check: PASSED

Files modified exist; commits 0c914ef, 19bea20, 5fad3d0 present.
