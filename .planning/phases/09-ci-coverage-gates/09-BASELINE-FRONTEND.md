# Phase 09 Frontend Coverage Baseline

Measured before any threshold is enforced (plan 09-02, Task 2). Enforcement of the 70% gate
is deferred to plan 09-04 per D-08/D-09 and the phase-boundary rule: a baseline under threshold
is closed with real tests, never by lowering the number.

## Command run

```
pnpm --dir web test
```

Resolved `@vitest/coverage-v8` version: **4.1.10** (exact pin, matching `vitest@4.1.10`).

## Verbatim per-file coverage table

```
$ vitest run

 RUN  v4.1.10 web

      Coverage enabled with v8


 Test Files  5 passed (5)
      Tests  16 passed (16)
   Start at  09:53:07
   Duration  9.46s (transform 590ms, setup 5.72s, import 4.18s, tests 4.16s, environment 24.68s)

 % Coverage report from v8
-------------------|---------|----------|---------|---------|-------------------
File               | % Stmts | % Branch | % Funcs | % Lines | Uncovered Line #s
-------------------|---------|----------|---------|---------|-------------------
All files          |   39.77 |    25.26 |   38.38 |   41.29 |
 app               |       0 |        0 |       0 |       0 |
  root.tsx         |       0 |        0 |       0 |       0 | 20-87
 ...ponents/common |      75 |       60 |   66.66 |      75 |
  CoverArt.tsx     |   71.42 |       60 |      50 |   71.42 | 42-48
 ...onents/history |   83.72 |    65.62 |   86.66 |   85.36 |
  EventCard.tsx    |    87.5 |       70 |     100 |    87.5 | 131,139,162
  ...ryFilters.tsx |   78.94 |    58.33 |   77.77 |   82.35 | 47,81-82
 ...ents/watchlist |   52.87 |     23.8 |   40.62 |      55 |
  ...ceToggles.tsx |   43.75 |       25 |      25 |   46.66 | 59-71,83,99-137
  SearchBox.tsx    |   78.37 |    57.14 |   88.88 |   84.37 | 56-62,79-81
  ...tsColumns.tsx |    6.25 |        0 |       0 |    6.25 | 18-163
  WatchlistRow.tsx |     100 |       50 |     100 |     100 | 37
 app/lib           |    3.33 |        0 |   11.11 |     3.7 |
  api.ts           |       0 |        0 |       0 |       0 | 88-208
 app/routes        |   20.22 |     17.5 |      25 |   20.25 |
  history.tsx      |       0 |        0 |       0 |       0 | 19-169
  watchlist.tsx    |   48.64 |    37.83 |   56.25 |   47.05 | ...28-130,140-149
-------------------|---------|----------|---------|---------|-------------------

=============================== Coverage summary ===============================
Statements   : 39.77% ( 107/269 )
Branches     : 25.26% ( 48/190 )
Functions    : 38.38% ( 38/99 )
Lines        : 41.29% ( 102/247 )
================================================================================
```

Note on the truncated-path rows (`...ponents/common`, `...onents/history`, `...ents/watchlist`,
`...ryFilters.tsx`, `...ceToggles.tsx`, `...tsColumns.tsx`): this is `istanbul`'s (the reporting
library `@vitest/coverage-v8` delegates to) own fixed-width column truncation on long paths — not
an artifact of this phase's config. Full paths, left to right: `app/components/common/`,
`app/components/history/`, `app/components/watchlist/`, `HistoryFilters.tsx`,
`PreferenceToggles.tsx`, `SearchResultsColumns.tsx`.

Three first-party `app/**` files present in the denominator do **not** get their own printed row
in the default `text` reporter output, because they are at 100% coverage across all four axes (the
`text` reporter's tree view folds fully-covered leaf nodes into their parent directory's aggregate
rather than printing a separate line for them): `app/components/common/EmptyState.tsx`,
`app/lib/utils.ts`, and `app/routes.ts` (the last has zero coverable statements — a pure
`RouteConfig` declaration — so it is trivially 100%). Verified directly against the coverage
engine's raw per-file summary data (a temporary `json-summary` reporter run, not committed): all
three report `pct: 100` on every axis, and the `"All files"` row's `107/269` statements,
`48/190` branches, `38/99` functions, and `102/247` lines totals match the sum of all 14
first-party files' individual totals exactly — confirming these three files ARE counted in the
denominator, they are just not individually listed by the `text` reporter's default display logic.
This is a reporter cosmetic, not a denominator gap (CICD-12's prohibition is about the
denominator, and the denominator is honest).

## Four aggregate axes vs. the 70% threshold

| Axis | Measured | vs. 70% | Gap |
|------|----------|---------|-----|
| Statements | 39.77% (107/269) | Below | 30.23 points |
| Branches | 25.26% (48/190) | Below | 44.74 points |
| Functions | 38.38% (38/99) | Below | 31.62 points |
| Lines | 41.29% (102/247) | Below | 28.71 points |

All four axes are independently below 70% — Vitest's `coverage.thresholds` check evaluates each
axis separately, so the single lowest axis (branches, 25.26%) is the binding constraint; closing
only the statements/lines/functions gap would still leave the gate failing on branches.

## Files at or near zero coverage, in descending order of uncovered lines

| File | % Lines | Uncovered lines | Genuine gap or structural (TEST-02 mocking)? |
|------|---------|------------------|-----------------------------------------------|
| `app/lib/api.ts` | 0% | 121 lines (88-208) | **Structural.** TEST-02 requires every component/route test to mock `web/app/lib/api.ts` — this module's request/response wiring is never exercised by the current suite by design. Closing this gap means adding tests that call the real fetch functions against a mocked HTTP layer (e.g. an `httptest`-style fetch mock), not mocking the module itself. |
| `app/routes/history.tsx` | 0% | 151 lines (19-169) | **Genuine gap.** No test file exists for this route at all — it is exercised by nothing today. This is the exact file 09-RESEARCH.md Pitfall 3 and this plan's `must_haves.truths` called out as the proof the denominator is honest. |
| `app/root.tsx` | 0% | 68 lines (20-87) | **Genuine gap.** The root layout/error-boundary component has no test file. |
| `app/routes/watchlist.tsx` | 47.05% | ~90 lines (28-130, 140-149) | **Mixed.** The route has a test file (exercising the happy path via the component tree), but its loader/action error paths and edge branches are uncovered — partially structural (API mocking hides request-failure branches) and partially a genuine gap in error-path testing. |
| `app/components/watchlist/SearchResultsColumns.tsx` | 6.25% | 146 lines (18-163) | **Genuine gap.** Column-definition/render-cell logic has almost no direct test coverage even though `SearchBox.tsx` (which likely renders these columns) is tested — the column cell renderers themselves are not separately exercised. |
| `app/components/watchlist/PreferenceToggles.tsx` | 46.66% | ~52 lines (59-71, 83, 99-137) | **Mixed.** Has a test file; toggle-change handlers and some conditional branches are uncovered. |

## Prioritization for plan 09-04 (D-09 order — most meaningful uncovered behavior first)

1. **`app/lib/api.ts`** — the single largest block of untested first-party logic (121 lines) and
   the module every other test mocks around. A dedicated test suite exercising the real fetch/parse/
   error-handling logic (not the mock) closes both the biggest numeric gap and the most
   structurally significant one: this is the API boundary the whole app depends on.
2. **`app/routes/history.tsx`** — a fully untested route (151 lines) surfacing user-visible history
   data; closing this is the direct proof-point this baseline's denominator fix was built to force
   into visibility.
3. **`app/root.tsx`** — the app shell/error-boundary; untested error-boundary behavior is exactly
   the kind of "meaningful uncovered behavior" D-09 prioritizes over an easy numeric win, since a
   broken error boundary degrades every route at once.
4. **`app/routes/watchlist.tsx` error/loader paths** — the happy path is already tested; the
   uncovered branches are specifically the failure-handling logic, which matters more for
   reliability than closing easy branches elsewhere.
5. **`app/components/watchlist/SearchResultsColumns.tsx`** — lowest-percentage component file;
   closing this is a real gap but lower-priority than the above because it is presentation logic
   downstream of the already-partially-tested `SearchBox.tsx`.
6. **`app/components/watchlist/PreferenceToggles.tsx` remaining branches** — smallest remaining
   gap among the flagged files, closed last.

Do not treat `app/lib/api.ts`'s 0% as closeable by relaxing TEST-02's mocking rule for other
tests — the correct fix is a dedicated test file for `api.ts` itself, tested against a real HTTP
mock (e.g. `fetch` intercepted at the network boundary), not the `~/lib/api` module mock every
other test uses.
