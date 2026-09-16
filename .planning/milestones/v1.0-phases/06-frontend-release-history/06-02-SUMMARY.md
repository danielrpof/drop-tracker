---
phase: 06-frontend-release-history
plan: 02
subsystem: fullstack
tags: [go, chi, react, react-router, tailwindcss, shadcn, keyset-pagination]

# Dependency graph
requires:
  - phase: 06-frontend-release-history
    provides: "plan 06-01's GET /events tracer endpoint, internal/events.Service, web/app/lib/api.ts's listEvents()/listWatchlist() wrappers, CoverArt/EmptyState shared components, and the app.css theme tokens"
provides:
  - "GET /events artist_id/event_type/cursor/limit query-param validation and filtering (HIST-01, T-06-06 through T-06-10)"
  - "web/app/components/history/EventCard.tsx -- type-dispatching event card (new_release/guest_feature/deluxe_change bodies)"
  - "web/app/components/history/HistoryFilters.tsx -- artist + event-type filter controls"
  - "History route rewritten around filters, cursor pagination, and full loading/error/empty/partial state coverage"
affects: [06-03-watchlist-tab, 06-04-release-notes-or-polish]

# Actuals (#2632)
actuals:
  tokens: 8719
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Query-param validation order (artist_id, event_type, cursor, limit) rejects before any store call, mirroring parseWatchlistID's reject-below-1 shape and search.go's read-trim-validate ordering"
    - "Over-max limit is passed through unclamped from the HTTP layer; the domain service (events.Service.List) owns the DoS clamp -- no HTTP-layer double-clamp"
    - "Frontend list-fetch pattern: separate initialLoading/appendLoading/error/appendError state, a reloadToken counter to re-run the fetch effect on Retry without touching filter state, and id-based de-dupe on append as a defensive backstop against a double-fired Load more"

key-files:
  created:
    - web/app/components/history/EventCard.tsx
    - web/app/components/history/HistoryFilters.tsx
  modified:
    - internal/httpserver/events.go
    - internal/httpserver/events_test.go
    - web/app/routes/history.tsx

key-decisions:
  - "limit=100000's clamp assertion (TestHandleListEvents_Validation) is tested against a real events.Service wrapping a recordingQuerier double, not the bare stubEventsStore other subtests use -- the clamp lives in Service.List, so a bare-stub test that bypasses the domain service can't observe it"
  - "guest_feature's card body link is built from event.source + event.external_id (musicbrainz.org/recording/{id} or deezer.com/track/{id}), never a stored URL string -- the events table has no URL column for this event type"
  - "Load-more append de-dupes fetched events by id defensively (filters out ids already in state) even though the disabled-button guard should already prevent a double request"

patterns-established:
  - "Query-param parse helpers (parseOptionalPositiveInt64, parseOptionalPageSize) are file-local to internal/httpserver/events.go, mirroring how watchlist.go and search.go keep their own parsing helpers local rather than shared across handler files"

requirements-completed: [HIST-01, UI-03]

coverage:
  - id: D1
    description: "GET /events validates artist_id/event_type/cursor/limit, rejecting every malformed value with a 400 {\"error\": \"...\"} before the store is ever called, and clamps an over-large limit via events.Service.List rather than rejecting it"
    requirement: HIST-01
    verification:
      - kind: unit
        ref: "internal/httpserver/events_test.go#TestHandleListEvents_Validation"
        status: pass
      - kind: integration
        ref: "internal/httpserver/events_test.go#TestListEvents_Filters"
        status: pass
    human_judgment: false
  - id: D2
    description: "History tab renders type-specific EventCards (new_release/guest_feature/deluxe_change), each with hero art, per-type badge color/emoji, and the correct partial-state fallback for null cover art, null release_date, and null previous_track_count"
    requirement: UI-03
    verification: []
    human_judgment: true
    rationale: "Visual card rendering, badge colors, truncation/tooltip behavior, and image fallback require a human looking at the rendered UI in a browser -- no automated test exercises the React tree end-to-end in this plan"
  - id: D3
    description: "History feed filters by artist and event type (both independently and composed), pages via Load more with a disabled/spinner state while a fetch is in flight, and shows the correct distinct copy for global-empty, filtered-empty, and fetch-failed states"
    requirement: HIST-01
    verification: []
    human_judgment: true
    rationale: "Client-side filter/pagination interaction and empty/error state rendering require a human clicking through the running SPA against a live GET /events -- covered by pnpm build/tsc only, not a browser-level test"

duration: ~70min
completed: 2026-08-11
status: complete
---

# Phase 6 Plan 2: History Filters, Type-Specific Cards, and Query-Param Validation Summary

**GET /events now validates and applies artist_id/event_type/cursor/limit query params with a domain-owned page-size clamp, and the History tab renders type-specific event cards (new_release/guest_feature/deluxe_change) behind composable artist/event-type filters with cursor-based Load more.**

## Performance

- **Duration:** ~70 min
- **Completed:** 2026-08-11
- **Tasks:** 2 of 2 (both required implementation)
- **Files modified/created:** 5 (2 created, 3 modified)

## Accomplishments

- `GET /events` reads and validates `artist_id`, `event_type`, `cursor`, and `limit`: each malformed value (non-numeric, below 1, an event_type outside `watchlist.EventTypes`) is rejected with a fixed `{"error": "..."}` 400 before `events.Store.List` is ever called; an empty param is treated as absent, not malformed; an over-large `limit` is passed through unclamped so `events.Service.List`'s existing `MaxPageSize` clamp reduces it (T-06-06).
- `internal/httpserver/events_test.go` gained `TestHandleListEvents_Validation` (every rejection case, plus proof the store is never called for a rejected request, plus a `recordingQuerier`-backed subtest proving the limit clamp reaches the real domain service) and `TestListEvents_Filters` (artist_id and event_type filters apply independently and compose, seeded across two artists and all three event types, plus a cursor-page-shares-no-row-with-its-predecessor check).
- `web/app/components/history/EventCard.tsx`: a single `EventCard` export dispatching on `event_type` to three in-file bodies -- `new_release` (release type + date, "Release date unknown" fallback), `guest_feature` (recording title linked out via a URL built from `source`/`external_id`, never a stored URL string), `deluxe_change` (previous → current track count, or current alone when `previous_track_count` is null). All three share the hero `CoverArt`, a 2-line-clamped title with a hover tooltip, and a badge chip using the three Discord-verbatim event colors from `app.css` -- the only place those colors are used; the indigo accent never touches a badge.
- `web/app/components/history/HistoryFilters.tsx`: artist options from `listWatchlist()` (called independently of the Watchlist tab per D-03), event-type options from the fixed three-value set, each with an "All ..." option; either control's change is reported upward as a full new filter value.
- `web/app/routes/history.tsx` rewritten around filters + cursor pagination: separate `initialLoading`/`appendLoading` flags drive skeleton cards on first load and on Load-more append; a `reloadToken` counter re-runs the exact same fetch on Retry without touching filter state; Load more is hidden once `next_cursor` is null, shows an inline spinner and is disabled while in flight, and de-dupes appended events by id as a defensive backstop; the empty state branches on whether either filter is active (`No matching events` / `Try a different artist or event type.`) versus the global case (`No release activity yet` / the watchlist call-to-action copy), and a fetch failure replaces the feed with `Couldn't load release history.` plus a working Retry button.

## Task Commits

Each task was committed atomically (Task 1 followed the TDD RED → GREEN split; Task 2 was a single implementation commit):

1. **Task 1 RED: failing tests for GET /events validation and filters** - `73fb994` (test)
2. **Task 1 GREEN: validate and apply GET /events query params** - `fc0750a` (feat)
3. **Task 2: type-specific History cards, filters, and load-more** - `c9702f0` (feat)

**Plan metadata:** committed separately below (SUMMARY.md + REQUIREMENTS.md, worktree mode).

## Files Created/Modified

- `internal/httpserver/events.go` - `parseOptionalPositiveInt64`, `parseOptionalPageSize` helpers; `handleListEvents` now reads and validates all four query params before calling `s.events.List`
- `internal/httpserver/events_test.go` - `recordingQuerier` (sqlc.Querier double), `insertTestEventTyped`, `TestHandleListEvents_Validation`, `TestListEvents_Filters`
- `web/app/components/history/EventCard.tsx` - new: `EventCard` plus `NewReleaseBody`/`GuestFeatureBody`/`DeluxeChangeBody` and `guestFeatureHref`
- `web/app/components/history/HistoryFilters.tsx` - new: `HistoryFilters`, `HistoryFiltersValue`
- `web/app/routes/history.tsx` - rewritten: filters, cursor-paginated fetch, skeleton/error/empty states, Load more with spinner + de-dupe

## Decisions Made

- The `limit=100000` clamp assertion in `TestHandleListEvents_Validation` uses a real `events.Service` wrapping a `recordingQuerier` double (a minimal `sqlc.Querier` that embeds a nil interface and overrides only `ListEvents`), not the file's bare `stubEventsStore` -- the clamp lives inside `events.Service.List`, so a test that bypasses the domain service entirely (as `stubEventsStore` does) cannot observe it. This is a one-off pattern local to that single subtest.
- `guestFeatureHref` builds the recording link from `event.source` + `event.external_id` rather than any stored URL field, since `events.Event` has no URL column for `guest_feature` rows -- this also satisfies the plan's "never pass a stored URL string straight into an anchor" instruction by construction (there is no such string to pass).
- Kept `parseOptionalPositiveInt64`/`parseOptionalPageSize` file-local to `events.go` rather than promoting them to a shared helpers file, matching how `watchlist.go` and `search.go` each keep their own parsing helpers local instead of sharing one cross-handler utility file.

## Deviations from Plan

None - plan executed exactly as written. No Rule 1-3 auto-fixes were needed; the one design choice worth flagging (the `recordingQuerier` test double) is documented above under Decisions Made since it was a plan-execution detail, not a deviation from what the plan specified.

## Issues Encountered

- **This worktree's own `docker compose up -d --wait postgres` failed to bind host port 5432** (`Bind for 0.0.0.0:5432 failed: port is already allocated`) because the parallel plan-06-03 executor's worktree already had a Postgres container bound to that port on this shared Windows dev host. Ran all DB-backed tests against that already-running, already-migrated container instead (same `drop_tracker` schema/credentials) rather than modifying `docker-compose.yml`'s hardcoded port mapping, which is out of this plan's scope and would risk colliding with the parallel worktree's own compose state.
- **`go test ./... -count=1` (default parallel package execution) intermittently failed `TestNotifyPending_CrossCycleRecoveryAfterOutage`** in `internal/notifier` (`successful request count = 3, want 1`) when run as part of the full suite -- confirmed environmental, not a regression: the test passes in isolation and passes when the full suite is run with `-p 1` (sequential package execution). Root cause is the same shared-Postgres-across-two-parallel-worktrees situation above: two executors' test suites writing to one shared `events`/`notifier`-adjacent state at once. Verified this plan's own `internal/httpserver` and `internal/events` packages are fully green both in isolation and as part of the full sequential suite.
- **`testMBID(t)` derives its value from `t.Name()` alone**, so two calls with the same `*testing.T` return the identical string -- `TestListEvents_Filters` initially hit a `duplicate key value violates unique constraint "artists_mbid_key"` until fixed by deriving artist A's and artist B's mbids as `testMBID(t) + "-a"` / `"-b"` instead of calling `testMBID(t)` twice.
- **`pnpm exec tsc --noEmit -p tsconfig.json` alone fails** with `Cannot find module './+types/root'` on a fresh `pnpm install` until React Router's typegen has run at least once -- used the project's own `pnpm run typecheck` script (`react-router typegen && tsc`) instead, which is what the plan's `<verify>` step effectively relies on via the prior build.

## User Setup Required

None - no external service configuration required. Local dev Postgres was already running (shared with the parallel plan-06-03 worktree, see Issues Encountered above).

## Next Phase Readiness

- `internal/webassets/build/client/` (the committed `go:embed` SPA output) was deliberately **not** refreshed by this plan -- this plan's `files_modified` scope is frontend source only. Before the phase's own end-to-end `make web && go build -o ./bin/server ./cmd/server` verification step (or before shipping), someone needs to re-run `make web` so the embedded binary serves this plan's updated History UI rather than plan 06-01's tracer-slice markup.
- `web/app/lib/api.ts`, `web/app/components/common/CoverArt.tsx`, and `web/app/components/common/EmptyState.tsx` remain untouched (verified via `git diff --stat` against the 06-01 commit), so plan 06-03 (Watchlist tab), running in parallel against the same files, has a stable, unmodified base to build against.
- No blockers. `web/app/routes.ts` and `root.tsx` (tab bar, route registration) are still plan 06-03's responsibility to extend with `/watchlist`; this plan did not touch either file.

---
*Phase: 06-frontend-release-history*
*Completed: 2026-08-11*

## Self-Check: PASSED

All 6 claimed files verified present on disk (`internal/httpserver/events.go`, `internal/httpserver/events_test.go`, `web/app/components/history/EventCard.tsx`, `web/app/components/history/HistoryFilters.tsx`, `web/app/routes/history.tsx`, this SUMMARY.md). Commits `73fb994`, `fc0750a`, `c9702f0` verified present in `git log`.
