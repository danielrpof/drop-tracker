---
phase: 25-find-filter-watchlist-and-history
plan: 04
subsystem: web
tags: [watchlist, name-filter, url-state, sticky-cards, react, vitest]
requires:
  - phase: 25-03
    provides: "makeEntry fixture and the Watchlist card list this plan wraps in a section"
provides:
  - "watchlistView.ts: foldName, matchesName, parseWatchlistParams, selectVisible, isFiltered, formatArtistCount, CLEAR_FILTER_KEYS"
  - "useUrlParams: the single URL writer (ref-based, preventScrollReset always on)"
  - "WatchlistToolbar with the Filter watchlist name filter and Clear filters"
  - "renderRoute initialEntry parameter and LocationProbe test helper"
  - "Your artists section with N of M count, filtered-to-zero state, count announcements, and the G4 sticky set"
affects: [25-05, 25-07, 25-08]
tech-stack:
  added: []
  patterns:
    - "Pure view-model module with no runtime import from ~/lib/api (route tests automock it)"
    - "All URL writes through one ref-based helper so same-tick writes compose and scroll never resets"
    - "Local input state filters instantly; debounced replace write to the URL; resync only when the URL differs from the last own write"
key-files:
  created:
    - web/app/lib/watchlistView.ts
    - web/app/lib/watchlistView.test.ts
    - web/app/lib/useUrlParams.ts
    - web/app/lib/useUrlParams.test.tsx
    - web/app/components/watchlist/WatchlistToolbar.tsx
    - web/app/components/watchlist/WatchlistToolbar.test.tsx
  modified:
    - web/app/lib/test/routeStub.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
key-decisions:
  - "q is filtered from local state on every keystroke and written to the URL 300ms trailing with replace; the input resyncs from the URL only when view.q differs from lastWrittenQ (Back/forward), never on an echo"
  - "refresh() now resolves with the loaded rows (null on failure) so the 409 add path can mark the existing artist sticky; it never touches the sticky set"
  - "Sticky set is route state keyed by id, cleared only by a search text change or Clear filters"
requirements-completed: [WLVW-01, WLVW-06]
status: complete
commits: 5
plan_head_before: f444b94b66b0a27598e5b5647f743005fb997f6e
actuals:
  tokens: 23000
  tasks: 3
  commits: 5
duration: 18 min
completed: 2026-10-10
coverage:
  - deliverable: "Name filter narrows the list per keystroke over name only, with N of M count and no extra requests"
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#Watchlist name filter (WLVW-01)"
        status: pass
    human_judgment: false
  - deliverable: "Fold table (accents, A$AP, Mø, Ænima, Straße) and blank-query behavior"
    verification:
      - kind: test
        ref: "web/app/lib/watchlistView.test.ts#matchesName"
        status: pass
    human_judgment: false
  - deliverable: "URL writer composes same-tick writes, keeps external params, and always sets preventScrollReset"
    verification:
      - kind: test
        ref: "web/app/lib/useUrlParams.test.tsx"
        status: pass
    human_judgment: false
  - deliverable: "Name filter clear x and Esc keep focus; Enter does nothing"
    verification:
      - kind: test
        ref: "web/app/components/watchlist/WatchlistToolbar.test.tsx"
        status: pass
    human_judgment: false
  - deliverable: "Clear filters, filtered-to-zero state, announcements, and sticky adds/Undo (G4)"
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#Watchlist filtered state (WLVW-06)"
        status: pass
    human_judgment: false
  - deliverable: "No request during search and no browser history throttling on key hold (real browser network panel)"
    verification: []
    human_judgment: true
    rationale: "Network-panel request counting and history throttling are only observable in a real browser; deferred to end-of-phase UAT per the plan's human-check"
---

# Phase 25 Plan 04: Watchlist name filter, count, URL state, and sticky cards Summary

The Watchlist gains a `Your artists` section with a live `Filter watchlist` name filter (accent and `$ø æ ß` folding), an always-visible `N of M artists` count, debounced replace-mode URL state through a single scroll-safe writer, a Clear filters flow that preserves sort, and a sticky set so adds and Undo restores never succeed invisibly.

## Accomplishments

- `watchlistView.ts` is a pure view-model (`foldName`, `matchesName`, `selectVisible`, `isFiltered`, `formatArtistCount`, `CLEAR_FILTER_KEYS`) with no runtime import from `~/lib/api`, so it survives route-test automocking.
- `useUrlParams` is the one URL writer: it builds on a ref so two same-tick writes compose, keeps params from external navigation, and always passes `preventScrollReset: true`.
- `WatchlistToolbar` renders the `Filter and sort artists` group with the name filter (label, `Filter by name…`, `ListFilter` icon), a `Clear name filter` x, Esc-to-clear, and the ghost `Clear filters` button.
- The route filters from local state on every keystroke with zero requests, writes `q` 300ms trailing with replace, and resyncs the input only on Back/forward or external URL changes.
- Filtered-to-zero shows `No artists match these filters` with a single default `Clear filters`; the toolbar button renders only when filtered and N > 0, so no duplicate buttons.
- Clear filters deletes `q`, `tags`, `muted`, `custom` in one push, keeps `sort`, focuses the input, and announces `Filters cleared. Showing {M} of {M} artists.`; search changes announce the count 500ms after the last keystroke through the existing status region.
- In-card edits (entry change, tag add/remove), add-from-search (including the 409 path), and Undo restores mark their card sticky; a search text change or Clear filters empties the set; `refresh()` never does.

## Task Commits

1. Task 1 (tracer): `115e13f` feat(25-04): filter the Watchlist by name with live count and URL state
2. Task 2 RED: `3062fd9` test(25-04): pin fold table, URL writer, and name filter keys
3. Task 2 GREEN: `e11bb20` feat(25-04): add the name filter clear button and Esc handling
4. Task 3 RED: `e431c13` test(25-04): pin clear filters, announcements, and sticky adds
5. Task 3 GREEN: `882a047` feat(25-04): add Clear filters, filtered-to-zero state, announcements, sticky cards

## Deviations from Plan

### Notes

**1. [TDD ordering] Fold-table and URL-writer tests were green on first run**
- **Issue:** `foldName`, `matchesName`, `useUrlParams`, and `formatArtistCount` were already built in the Task 1 tracer, so those Task 2 tests pinned existing behavior and could not be observed RED. The toolbar clear x and Esc cases were genuinely RED (2 failing on the planned assertions) and drove the Task 2 feature commit.

**2. [Test harness] LocationProbe's `<output>` has an implicit status role**
- **Found during:** Task 3 GREEN
- **Issue:** with the probe rendered beside the route, `getByRole("status")` matched two elements.
- **Fix:** the test helper selects the route status region by `aria-atomic="true"`. Production markup is unchanged; the "exactly one status region" test renders without the probe and still passes.
- **Commit:** `882a047`

**3. [Scope] No test for "in-card edit keeps a card that stopped matching"**
- **Issue:** with only the name axis, an in-card edit cannot change whether a card matches, so the behavior is unobservable until tag and preference filters land (plans 25-07/08). `markSticky` is wired into `handleEntryChange`, `addTag`, `removeTag` (grep shows 6 call sites plus the definition) and should be covered in those plans.

**Total deviations:** 0 auto-fixed, 3 notes. **Impact:** none on behavior or scope.

## Verification

- `corepack pnpm test` (full web suite with coverage): 28 files, 432 tests passed
- `corepack pnpm run typecheck`: exit 0
- `prettier --check "**/*.{ts,tsx}"`: clean
- Acceptance greps: `preventScrollReset: true` x1, `useUrlParams()` x1, `watchlist-artists-heading` x1, `LocationProbe` export x1, `Clear name filter` x1, `A$AP Rocky` in tests, `No artists match these filters` x2, `Filters cleared. Showing` x1, `markSticky(` x7
- Pending human check: network panel on a 50+ artist dev DB (one `GET /watchlist`, no requests while typing, no history throttling warning). Deferred to end-of-phase UAT.

## Known Stubs

None.

## Threat Flags

None. T-25-11 (`q` rendered only as controlled input value and JSX text), T-25-12 (300ms debounced replace writes), T-25-13 (route test asserts one `listWatchlist` call and no `listTags`/`listEvents`), and T-25-14 (sticky set keyed by POST response id, add and Undo tests) are mitigated as planned.

## Self-Check: PASSED

- FOUND: web/app/lib/watchlistView.ts, web/app/lib/useUrlParams.ts, web/app/components/watchlist/WatchlistToolbar.tsx, and the three new test files
- FOUND commits 115e13f, 3062fd9, e11bb20, e431c13, 882a047
- All task acceptance criteria re-run and passing
