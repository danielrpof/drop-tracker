---
phase: 25-find-filter-watchlist-and-history
plan: 03
subsystem: web
tags: [watchlist, release-dates, formatter, react, vitest]
requires:
  - phase: 25-01
    provides: "GET /watchlist latest_release_date / next_release_date fields"
provides:
  - "WatchlistEntry.latest_release_date / next_release_date (string | null)"
  - "formatReleaseDate(raw): stored-precision, timezone-free date formatter"
  - "makeEntry(overrides) test fixture builder for later Phase 25 plans"
  - "Latest release / Upcoming lines on every Watchlist card"
affects: [25-04, 25-05, 25-07, 25-08]
tech-stack:
  added: []
  patterns:
    - "String-only date formatting (fixed month table, anchored regexes, no Date)"
    - "Shared test fixture builder under app/lib/test/"
key-files:
  created:
    - web/app/lib/test/fixtures.ts
  modified:
    - web/app/lib/api.ts
    - web/app/lib/format.ts
    - web/app/lib/format.test.ts
    - web/app/components/watchlist/WatchlistRow.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
    - web/app/lib/api.test.ts
    - web/app/components/history/HistoryFilters.test.tsx
    - web/app/components/watchlist/ArtistNote.test.tsx
    - web/app/components/watchlist/PreferenceToggles.test.tsx
    - web/app/components/watchlist/SearchResultsColumns.test.tsx
    - web/app/components/watchlist/TagChips.test.tsx
    - web/app/components/watchlist/TagCombobox.test.tsx
key-decisions:
  - "Release dates render via string-only formatReleaseDate; malformed input passes through unchanged as plain JSX text"
  - "Client never compares dates to today; server decides what is upcoming (G2/G3)"
requirements-completed: [WLVW-03]
status: complete
commits: 2
plan_head_before: f6687a68fcafd1397bb02e8c2c130c9cacf80d69
actuals:
  tokens: 9000
  tasks: 2
  commits: 2
duration: 8 min
completed: 2026-10-10
coverage:
  - deliverable: "Watchlist cards show latest release or 'No releases yet' plus conditional Upcoming"
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#renders each card's latest release and a conditional Upcoming segment (D-03)"
        status: pass
    human_judgment: false
  - deliverable: "formatReleaseDate pins year / year-month / full-date table and passes malformed input through"
    verification:
      - kind: test
        ref: "web/app/lib/format.test.ts#formatReleaseDate"
        status: pass
    human_judgment: false
  - deliverable: "Release dates render as literal text (no markup injection)"
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#renders a release date containing markup as literal text (T-25-09)"
        status: pass
    human_judgment: false
  - deliverable: "makeEntry fixture builder and outdated route comment replaced"
    verification:
      - kind: command
        ref: "corepack pnpm --dir web run typecheck"
        status: pass
    human_judgment: false
---

# Phase 25 Plan 03: Latest and upcoming release on Watchlist cards Summary

Each Watchlist card now shows `Latest release: {date}` (or `No releases yet`) and, only when the server sends a next date, `Upcoming: {date}`, formatted at stored precision by a string-only `formatReleaseDate` with no Date construction.

## Accomplishments

- `WatchlistEntry` gained `latest_release_date` and `next_release_date` (`string | null`), matching plan 25-01's JSON contract; all existing fixtures updated and typecheck is green.
- `formatReleaseDate` renders `2024`, `May 2024`, `17 May 2024` from a fixed English month table and returns anything else unchanged.
- `WatchlistRow` renders a single wrapping line in the name column, with the Upcoming segment conditional on `next_release_date`.
- `makeEntry` fixture builder added in `web/app/lib/test/fixtures.ts` for the rest of the phase.
- The `watchlist.tsx` module comment no longer claims "never re-sorts" or "no release activity"; replaced with a 3-line Phase 25 D-02/D-03 note.

## Task Commits

1. Task 1 (tracer): `aa50c00` feat(25-03): show latest and upcoming release on Watchlist cards
2. Task 2 (tdd): `07e47ee` test(25-03): pin release-date table, add makeEntry fixture, refresh route comment

## Deviations from Plan

### Notes

**1. [Plan criterion quirk] Date-construction grep matches the function signature**
- **Found during:** Task 1 acceptance check
- **Issue:** `awk '/export function formatReleaseDate/,/^}/' format.ts | grep -c 'Date('` prints `1`, not `0`, because the signature line `formatReleaseDate(raw: string)` contains the substring `Date(`. No Date object is constructed anywhere in the function body.
- **Fix:** None needed; intent (no Date construction) is met. Verified by reading the function.

**2. [TDD ordering] Task 2 formatter tests were green on first run**
- **Issue:** `formatReleaseDate` was already implemented in the Task 1 tracer (the tracer's route test needed it), so Task 2's table tests could not be observed RED. The tests pin the table and were confirmed passing.

**Total deviations:** 0 auto-fixed, 2 notes. **Impact:** none on behavior or scope.

## Verification

- `vitest run app/lib/format.test.ts app/routes/watchlist.test.tsx`: 69 passed
- `corepack pnpm run typecheck`: exit 0
- `prettier --check "**/*.{ts,tsx}"`: clean
- `corepack pnpm test` (full web suite with coverage): passed

## Known Stubs

None.

## Threat Flags

None. T-25-09 (XSS) and T-25-10 (date integrity) mitigated as planned: plain JSX text nodes, literal-`<b>` route test, no Date or timezone math.

## Self-Check: PASSED

- FOUND: web/app/lib/test/fixtures.ts
- FOUND commits aa50c00, 07e47ee
- Task 1 and Task 2 acceptance criteria re-run and passing (see note 1)
