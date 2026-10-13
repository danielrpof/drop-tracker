---
phase: 25-find-filter-watchlist-and-history
plan: 13
subsystem: ui
tags: [react, tailwind, vitest, watchlist, search, responsive]
requires:
  - phase: 06-search-and-add
    provides: SearchResultsColumns Deezer row with disabled "Search MusicBrainz to add" button
provides:
  - Deezer search-result button capped at 40% of the row below lg, with a truncating label
  - Truncating type line in search-result rows
affects: [25-UAT test 9 held-out 375/640/1024px visual check]
actuals:
  tokens: 1500
  tasks: 2
  commits: 4
plan_head_before: fc7d23df0d0954bbeeb7807658d40b9a9bca2003
tech-stack:
  added: []
  patterns:
    - "Cap a trailing flex control with max-w-[N%] + inner truncate span so the flex-1 min-w-0 text column keeps the row"
key-files:
  created: []
  modified:
    - web/app/components/watchlist/SearchResultsColumns.tsx
    - web/app/components/watchlist/SearchResultsColumns.test.tsx
key-decisions:
  - "Cap is 40% of the row below lg and lifted at lg (1024px+); only the Deezer button changes, Add to Watchlist and Already watching stay as-is"
requirements-completed: [WLVW-01]
coverage:
  - id: D1
    description: "Deezer row button is capped (max-w-[40%] lg:max-w-none) with its label in a truncating span; accessible name, title, and disabled state unchanged"
    requirement: WLVW-01
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#caps the Deezer button and truncates its label so the name keeps the row (G-25-9)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Type line truncates in Deezer and MusicBrainz rows; Add to Watchlist and Already watching buttons uncapped"
    requirement: WLVW-01
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#truncates the type line in Deezer and MusicBrainz rows (G-25-9)"
        status: pass
      - kind: unit
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#leaves the Add to Watchlist and Already watching buttons uncapped"
        status: pass
    human_judgment: false
  - id: D3
    description: "At 375px and 640px a Deezer row shows a readable artist name beside a truncated button label; from 1024px the full label shows"
    requirement: WLVW-01
    verification: []
    human_judgment: true
    rationale: "jsdom has no layout engine; real widths need a browser (UAT test 9, end-of-phase human verify)"
duration: 5min
completed: 2026-10-11
status: complete
---

# Phase 25 Plan 13: Deezer Row Width Cap Summary

**Deezer search-result button capped at 40% of the row below lg with a truncating label, plus a truncating type line, so the artist name no longer collapses to one letter at 375px (G-25-9)**

## Performance

- **Duration:** 5 min
- **Completed:** 2026-10-11T20:28:15Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- The disabled `Search MusicBrainz to add` button now has `max-w-[40%] shrink-0 lg:max-w-none` and its label sits in an inner `<span className="truncate">`. The name column gets the remaining row width. The accessible name, `title`, and disabled state are unchanged.
- The `artist.type` span now truncates like the name and secondary label, so it can't run under the button.
- `ui/button.tsx`, the `Add to Watchlist` branch, and the `Already watching` branch are untouched.

## Task Commits

1. **Task 1: Deezer row's name gets the row** - `ef6d74c` (test, RED), `a027206` (feat, GREEN)
2. **Task 2: Truncate the type line** - `71a9d4a` (test, RED), `1ef7bc5` (feat, GREEN)

**Plan metadata:** committed separately (docs: complete plan)

## Files Created/Modified
- `web/app/components/watchlist/SearchResultsColumns.tsx` - capped Deezer button with truncating label span, truncating type span
- `web/app/components/watchlist/SearchResultsColumns.test.tsx` - three new class-level tests for G-25-9

## Decisions Made
None - followed plan as specified.

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

RED (`test(25-13)`) precedes GREEN (`feat(25-13)`) for both tasks. Task 1's RED failed on the `max-w-[40%]` class assertion. Task 2's type-line test failed on `truncate`. The "uncapped buttons" test in Task 2 passed on first run because it guards against regression rather than adding behavior.

## Issues Encountered
None

## Verification
- `vitest run SearchResultsColumns.test.tsx watchlist.test.tsx`: 82 passed
- `pnpm --dir web run typecheck` and `prettier --check "**/*.{ts,tsx}"`: exit 0
- `git diff -- web/app/components/ui/button.tsx`: empty
- Tracer gate: automated verify re-run passed, so expansion continued.

## Known Stubs
None.

## Threat Flags
None. The change is class names plus a wrapping span on already-rendered text (T-25-36 accepted; T-25-37 mitigated, since the button stays disabled with an unchanged accessible name and title).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
The 375/640/1024px visual check (Task 2 `human-check`) is queued for UAT test 9 at end of phase.

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-11*

## Self-Check: PASSED
