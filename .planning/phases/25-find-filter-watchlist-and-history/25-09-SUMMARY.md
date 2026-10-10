---
phase: 25-find-filter-watchlist-and-history
plan: 09
subsystem: web
tags: [history, url-state, react-router, filters, vitest]
requires:
  - phase: 25-04
    provides: "useUrlParams single URL writer; routeStub/LocationProbe test seam"
  - phase: 25-06
    provides: "History Tag filter, degraded Tag trigger labels, tag empty state"
provides:
  - "historyParams: HISTORY_EVENT_TYPES, parseHistoryParams, canonicalHistoryParams"
  - "History Artist/Event type/Tag filters backed by artist/type/tag URL params (push on change)"
  - "Once-per-source stale tag and artist cleanup with replace; never on a failed fetch"
  - "HistoryFilters onArtistsLoaded prop and Loading artists… / Artist unavailable trigger labels"
affects: [25-10]
plan_head_before: b1ff19cac5ea8a2a17efc62ac964a0103307337d
actuals:
  tokens: 4100
  tasks: 2
  commits: 3
tech-stack:
  added: []
  patterns:
    - "Latest-callback ref for a mount-once effect that reports upward (onArtistsLoadedRef)"
    - "Canonicalization effect rewrites only owned keys with replace; foreign keys untouched"
key-files:
  created:
    - web/app/lib/historyParams.ts
    - web/app/lib/historyParams.test.ts
  modified:
    - web/app/routes/history.tsx
    - web/app/routes/history.test.tsx
    - web/app/components/history/HistoryFilters.tsx
    - web/app/components/history/HistoryFilters.test.tsx
key-decisions:
  - "Artist id check runs inside onArtistsLoaded against a filtersRef, so it sees the URL at load time, not at first render"
  - "Stale-id cleanup is gated by tagIdChecked/artistIdChecked refs: one check per source per mount"
requirements-completed: [HIST-02]
duration: 4 min
completed: 2026-10-10
status: complete
commits: 3

coverage:
  - id: D1
    description: "History filters live in the URL: reload with a tag reproduces the filtered feed before GET /tags resolves; control changes push and keep other params"
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "web/app/routes/history.test.tsx#History URL state (HIST-02)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Malformed artist/type/tag params parse to null and are dropped with replace; foreign keys kept"
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "web/app/lib/historyParams.test.ts"
        status: pass
      - kind: unit
        ref: "web/app/routes/history.test.tsx#drops malformed params with replace and never requests with them"
        status: pass
    human_judgment: false
  - id: D3
    description: "Stale tag/artist ids dropped once their source loads; a failed or pending fetch never drops them and the trigger says Tag unavailable / Artist unavailable / Loading ..."
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "web/app/routes/history.test.tsx#History stale and malformed params (D-13)"
        status: pass
      - kind: unit
        ref: "web/app/components/history/HistoryFilters.test.tsx#Artist source states"
        status: pass
    human_judgment: false
  - id: D4
    description: "Multi-page History across the retention edge with a real dataset; Back/forward restores the previous filter combination in a real browser"
    verification: []
    human_judgment: true
    rationale: "VALIDATION.md manual-only item: needs a real dataset and real browser history; not assertable in jsdom"
---

# Phase 25 Plan 09: History Filters in the URL Summary

**History's Artist, Event type, and Tag filters now live in `artist`/`type`/`tag` URL params so a filtered view survives reload and Back, with malformed or stale ids dropped via replace once their source has loaded and never because a fetch failed.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-10-10T16:57:33Z
- **Completed:** 2026-10-10T17:01:00Z
- **Tasks:** 2
- **Files modified:** 6 (2 created)

## Accomplishments
- `historyParams.ts` parses `artist`/`tag` with `^[1-9]\d*$` and `type` against a three-value allow-list; `canonicalHistoryParams` rewrites only those keys.
- `history.tsx` reads filters from `useUrlParams()` (the `useState` is gone); control changes and `Show all tags` write with push, and page one fetches with `tag_id` from the URL while `GET /tags` is still pending.
- A canonicalization effect drops malformed params with replace; `tagIdChecked`/`artistIdChecked` refs drop a stale tag or artist once, only after a successful load.
- `HistoryFilters` gained `onArtistsLoaded` (success only) and Artist trigger labels `Loading artists…` / `Artist unavailable`, mirroring the locked Tag copy.
- Added the `this tag` fallback test that 25-06 deferred: with `GET /tags` failed and `tag=3` in the URL the empty state reads "No events for “this tag”".

## Task Commits

1. **Task 1: Reload History with a tag filter in the URL (tracer)** - `7bafd67` (feat)
2. **Task 2: Drop malformed and stale History params (TDD)** - `e050de4` (test, RED), `578b5fc` (feat, GREEN)

**Plan metadata:** committed with this summary (docs).

## Files Created/Modified
- `web/app/lib/historyParams.ts` - parse/canonical helpers and the event-type allow-list
- `web/app/lib/historyParams.test.ts` - parse and canonical cases
- `web/app/routes/history.tsx` - URL-backed filters, canonicalization effect, once-per-source stale checks
- `web/app/routes/history.test.tsx` - URL-state and stale/malformed route tests (adds a `sonner` mock to assert no toast)
- `web/app/components/history/HistoryFilters.tsx` - `onArtistsLoaded`, `artistsStatus`, Artist trigger label
- `web/app/components/history/HistoryFilters.test.tsx` - Artist source-state tests

## Decisions Made
- The artist stale check reads `filtersRef` inside `onArtistsLoaded`, so it uses the URL at load time rather than the first render's closure; `HistoryFilters` keeps the latest callback in a ref so the mount-once fetch effect never goes stale.
- Cleanup is once per source per mount; a later Back to a URL carrying a stale id is not re-checked (matches the plan's "once-per-source" wording).

## Deviations from Plan

### Auto-fixed Issues

None - plan executed exactly as written.

### Other notes

- **TDD RED partially green:** `parseHistoryParams`/`canonicalHistoryParams` shipped in Task 1 as the plan's interfaces block requires, so `historyParams.test.ts` and the "tag kept on GET /tags failure" and `this tag` route tests passed at RED time. The other nine new tests failed on assertions as intended.
- **Copy outside the locked contract (plan flag A3):** `Loading artists…` / `Artist unavailable` have no UI-SPEC row. Flag for `/gsd-ui-review`.

---

**Total deviations:** 0 auto-fixed
**Impact on plan:** None.

## Issues Encountered
None.

## Known Stubs
None.

## Threat Flags
None. T-25-26/27/28 mitigations are in place: only `^[1-9]\d*$` ids and the three-value type allow-list survive parsing, and param values are never rendered.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Manual-only check remains for UAT: pick a tag spanning the retention window, press Load more to the end, reload, then Back (VALIDATION.md).
- Full web suite: 29 files, 506 tests pass; typecheck and prettier --check clean.

## Self-Check: PASSED
- Created/modified files exist; commits `7bafd67`, `e050de4`, `578b5fc` present (3 measured from the plan ledger).
- Acceptance greps: `useUrlParams()` 1, `useState<HistoryFiltersValue>` 0, `export function parseHistoryParams` 1, `onArtistsLoaded` 5, `Artist unavailable` 1, `tagIdChecked` 3.

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-10*
