---
phase: 08-frontend-test-suite
plan: 02
subsystem: testing
tags: [vitest, react-testing-library, react-router, watchlist]

# Dependency graph
requires:
  - phase: 08-frontend-test-suite
    plan: "08-01"
    provides: "Vitest + jsdom + RTL harness (vitest.config.ts, vitest.setup.ts) and the pnpm test script"
provides:
  - "web/app/lib/test/routeStub.tsx -- the single shared createRoutesStub-based renderRoute helper (D-03, ROADMAP success criterion 4)"
  - "Route-level test proving the watchlist row's remove control reaches removeWatchlist with the entry's id"
  - "Optimistic-update and rollback-on-failure tests for PreferenceToggles"
affects: [08-03, 08-04, 08-05, 09-ci-coverage-gates]

# Actuals (#2632)
actuals:
  tokens: 1119
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "createRoutesStub-based renderRoute(Component, path) as the one router-context test seam, reused by every future test needing router context"
    - "Regex accessible-name matcher (/^Single release type/) for Base UI checkboxes whose computed accessible name concatenates aria-label with the wrapping <label>'s visible text"

key-files:
  created:
    - web/app/lib/test/routeStub.tsx
    - web/app/routes/watchlist.test.tsx
    - web/app/components/watchlist/PreferenceToggles.test.tsx
  modified: []

key-decisions:
  - "Queried checkboxes with a regex accessible-name matcher (/^Single release type/) rather than the plan's literal string -- Base UI's Checkbox.Root sets both aria-label and aria-labelledby (pointing at the wrapping visible <label> text), so the computed accessible name is the concatenation \"Single release type Single\", not the aria-label string alone. An exact-string getByRole query found zero matches; the regex still asserts the real aria-label text verbatim per the plan's require-verbatim-match acceptance criterion, just without demanding an exact full-string equality against a name RTL computes differently than the plan assumed."

requirements-completed: [TEST-01, TEST-02]

coverage:
  - id: D1
    description: "Clicking the watchlist row's remove control triggers removeWatchlist with the entry's numeric id, proven at the route level through the shared renderRoute helper"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.test.tsx (1 test) via pnpm --dir web exec vitest run app/routes/watchlist.test.tsx"
        status: pass
      - kind: other
        ref: "Temporarily changed the asserted id from 42 to 999, confirmed the test fails with a clear expected/received diff, restored to 42, confirmed green again"
        status: pass
    human_judgment: false
  - id: D2
    description: "A preference toggle rolls back its optimistic state through onEntryChange when the PATCH call fails, proven via the ordered optimistic-then-restore call pair"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/PreferenceToggles.test.tsx (2 tests) via pnpm --dir web exec vitest run app/components/watchlist/PreferenceToggles.test.tsx"
        status: pass
      - kind: other
        ref: "Temporarily commented out the catch branch's onEntryChange(entry.id, { release_types: previous }) restore call in PreferenceToggles.tsx, confirmed the rollback test fails, restored the source, confirmed git diff --stat shows no change to PreferenceToggles.tsx"
        status: pass
    human_judgment: false
  - id: D3
    description: "One shared createRoutesStub helper (renderRoute) is the single router-context seam, established once and reused"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "grep -c 'createRoutesStub' web/app/lib/test/routeStub.tsx (3), grep -c 'export function renderRoute' (1), grep -c 'renderRoute' web/app/routes/watchlist.test.tsx (2), ! grep -q 'MemoryRouter' web/app/routes/watchlist.test.tsx"
        status: pass
    human_judgment: false
  - id: D4
    description: "Both new test files mock web/app/lib/api.ts and pass with no server running, individually and as part of the full parallel suite"
    requirement: "TEST-02"
    verification:
      - kind: unit
        ref: "pnpm --dir web exec vitest run app/routes/watchlist.test.tsx (alone, green); pnpm --dir web exec vitest run app/components/watchlist/PreferenceToggles.test.tsx (alone, green); pnpm --dir web test (full suite, 3 files / 6 tests, green)"
        status: pass
      - kind: unit
        ref: "grep -c 'vi.mock(\"~/lib/api\")' each file (1 each); ! grep -rn 'importActual' web/app --include='*.test.tsx'"
        status: pass
    human_judgment: false
  - id: D5
    description: "pnpm --dir web typecheck exits 0"
    requirement: "TEST-01"
    verification:
      - kind: unit
        ref: "pnpm --dir web typecheck (react-router typegen && tsc)"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-12
status: complete
---

# Phase 8 Plan 2: Shared Router Stub, Watchlist Remove Test, Preference Toggle Tests Summary

**Built the one shared `createRoutesStub`-based `renderRoute` test helper and proved the two watchlist behaviors ROADMAP success criterion 2 names verbatim: the remove control reaches `removeWatchlist`, and a preference toggle rolls back its optimistic state on PATCH failure.**

## Performance

- **Duration:** ~25 min
- **Tasks:** 2 (both executed)
- **Files created:** 3 (`routeStub.tsx`, `watchlist.test.tsx`, `PreferenceToggles.test.tsx`)
- **Files modified:** 0 permanently (source components tested as-is)

## Accomplishments
- Created `web/app/lib/test/routeStub.tsx` exporting exactly one function, `renderRoute`, wrapping React Router's `createRoutesStub` -- the single router-context seam for this suite and future plans
- Wrote `web/app/routes/watchlist.test.tsx`, rendering the `Watchlist` route's default export through `renderRoute` (not a `WatchlistRow`-only unit test, since `WatchlistRow` never imports `~/lib/api`) and asserting `removeWatchlist` is called with the clicked row's exact numeric id
- Wrote `web/app/components/watchlist/PreferenceToggles.test.tsx` with the D-05 floor of exactly two behaviors: optimistic PATCH call with the new array, and rollback to the pre-toggle array (proven via the ordered pair of `onEntryChange` calls, not just final state) when the PATCH rejects
- Verified both files' assertions have teeth: deliberately broke the asserted id and the rollback source line, confirmed both tests fail, restored both

## Task Commits

Each task was committed atomically:

1. **Task 1: Shared createRoutesStub helper plus the route-level remove-triggers-API test** - `0f6309b` (test)
2. **Task 2: Preference-toggle optimistic update and rollback-on-failure tests** - `95da16e` (test)

## Files Created/Modified
- `web/app/lib/test/routeStub.tsx` - `renderRoute(Component, path)`, the single shared `createRoutesStub` seam (D-03)
- `web/app/routes/watchlist.test.tsx` - route-level test proving remove-triggers-`removeWatchlist`, id asserted explicitly
- `web/app/components/watchlist/PreferenceToggles.test.tsx` - optimistic-update and rollback-on-failure tests, checkbox queried by accessible name + `toBeChecked()`-style `aria-checked` semantics (no `.toHaveValue()` against the Base UI checkbox span)

## Decisions Made
- Queried the preference-toggle checkboxes with a regex accessible-name matcher (`/^Single release type/`) instead of an exact string match. Base UI's `Checkbox.Root` sets both `aria-label="Single release type"` and `aria-labelledby` pointing at the wrapping `<label>`'s own visible "Single" text, so RTL's computed accessible name is the concatenation `"Single release type Single"`, not the plan's literal `"Single release type"` string. An exact-match `getByRole` query found zero matches against the real DOM; the regex still requires the verbatim `aria-label` text as a prefix, satisfying the plan's "matching the aria-label strings verbatim" requirement without asserting equality against a longer computed name the plan's example didn't anticipate.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Checkbox accessible-name query changed from exact string to regex prefix match**
- **Found during:** Task 2, first `pnpm --dir web exec vitest run` of `PreferenceToggles.test.tsx`
- **Issue:** `screen.getByRole("checkbox", { name: "Single release type" })` (the plan's and 08-RESEARCH.md's literal example) threw `TestingLibraryElementError: Unable to find an accessible element with the role "checkbox" and name "Single release type"` -- RTL's DOM dump showed the actual computed name is `"Single release type Single"` because Base UI's checkbox wires `aria-labelledby` to the wrapping `<label>` text (`"Single"`) in addition to the explicit `aria-label`.
- **Fix:** Changed both queries in the file to `screen.getByRole("checkbox", { name: /^Single release type/ })`.
- **Files modified:** `web/app/components/watchlist/PreferenceToggles.test.tsx`
- **Verification:** Re-ran the file alone (2 passed) and the full suite (3 files / 6 tests, green); typecheck clean.
- **Committed in:** `95da16e` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 -- test query bug blocking a green run; no source component behavior changed)
**Impact on plan:** No scope creep. `PreferenceToggles.tsx` itself is unmodified (`git diff --stat` confirms). The fix only changed how the test locates the DOM element it was already correctly meant to target.

## Issues Encountered
None beyond the one auto-fixed issue above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `renderRoute` is proven end-to-end and ready for any future plan that needs router context, without re-litigating D-03.
- The full Vitest suite (08-01's `HistoryFilters.test.tsx` plus this plan's two files) runs green together and each file individually, confirming per-file `~/lib/api` mock isolation holds under parallel execution.
- Plans 08-03/08-04/08-05 (search, history/event-filter, and the folded EventCard/SearchBox bug-fix tests) can proceed against the same harness with no further config work.

---
*Phase: 08-frontend-test-suite*
*Completed: 2026-08-12*
