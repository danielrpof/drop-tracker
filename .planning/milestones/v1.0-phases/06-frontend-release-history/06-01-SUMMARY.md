---
phase: 06-frontend-release-history
plan: 01
subsystem: fullstack
tags: [go, chi, sqlc, pgx, react, react-router, tailwindcss, shadcn, go-embed, vite]

# Dependency graph
requires:
  - phase: 05-discord-notifications
    provides: the events table schema (previous_track_count, release_type columns) and its Go domain conventions (Store interfaces, writeError, httplog.SetAttrs)
provides:
  - "GET /events keyset-paginated history endpoint backed by internal/events.Service"
  - "internal/webassets: go:embed + chi SPA fallback wiring, the single-Go-binary serving pattern the whole frontend depends on"
  - "web/: React Router SPA Mode scaffold, Tailwind v4 dark theme tokens, shadcn component inventory, typed api.ts wrappers for all five backend endpoints, shared CoverArt/EmptyState components"
affects: [06-02-history-filters-and-cards, 06-03-watchlist-tab, 06-04-release-notes-or-polish]

# Actuals (#2632)
actuals:
  tokens: 250448
  tasks: 1
  commits: 1

tech-stack:
  added: [react@19.2, react-router@7.15 (SPA Mode), vite@8, tailwindcss@4, "@tailwindcss/vite", shadcn CLI, lucide-react, sonner, next-themes, typescript@5.9.3, pnpm]
  patterns:
    - "internal/events mirrors internal/watchlist's Store/Service/NewService shape exactly -- new domain packages in this codebase follow one established template"
    - "keyset pagination on id DESC (never created_at) for any feed a background writer inserts into concurrently"
    - "go:embed all:build/client + chi r.NotFound fallback, registered after every explicit route, for single-binary SPA serving"
    - "web/app/lib/api.ts is the one file every frontend plan imports from, never edits -- wave-2 plans (06-02/06-03) run in parallel against it"

key-files:
  created:
    - internal/events/service.go
    - internal/httpserver/events.go
    - internal/httpserver/events_test.go
    - internal/httpserver/spa_test.go
    - internal/webassets/embed.go
    - internal/webassets/build/client/ (committed SPA build output)
    - web/ (full React Router SPA scaffold -- app.css, root.tsx, routes.ts, lib/api.ts, components/common/{CoverArt,EmptyState}.tsx, routes/history.tsx, shadcn ui/ components)
  modified:
    - queries/events.sql (added ListEvents :many)
    - internal/db/sqlc/events.sql.go, internal/db/sqlc/querier.go (sqlc-generated)
    - internal/httpserver/server.go (widened New, registered /events + SPA fallback)
    - cmd/server/main.go (constructs events.NewService)
    - internal/httpserver/{health,search,server,watchlist,boot_e2e}_test.go (updated New() call sites)
    - Makefile (web target)
    - .gitignore (web/build/, web/.react-router/)

key-decisions:
  - "Kept @react-router/node as a dependency (react-router typegen self-restored isbot after I removed it, confirming both are load-bearing for the build-time root prerender even in SPA Mode) but removed the scaffold's @react-router/serve dependency and package.json start script -- both point at a Node server (build/server/index.js) that ssr:false guarantees will never exist, and CLAUDE.md forbids a Node runtime in production anyway"
  - "Deleted the shadcn scaffold's generated web/Dockerfile and .dockerignore -- they build and run a standalone Node server, directly contradicting the single-Go-binary/no-Node-runtime-in-production architecture; Phase 7's own multi-stage Dockerfile (noted in the Makefile's web target comment) is the real deploy path"
  - "Pinned typescript to 5.9.3 (npm's newest stable 5.x) instead of the scaffold's default ^6, since npm has no stable 6.x release (only a 6.0.0-beta tag) and latest resolves to a 7.x pre-release -- exactly the risk 06-RESEARCH.md and Task 1's checkpoint flagged"
  - "Applied class=\"dark\" unconditionally on <html> in root.tsx rather than wiring a theme toggle -- D-13 specifies dark-themed as the app's only visual target, no light-mode toggle exists or is planned"
  - "routes.ts registers both index() and route(\"history\", ...) pointing at the same history.tsx file with explicit distinct id options -- React Router derives an id from the file path by default and collided when the same file backed two routes"
  - "Tailwind v4 @theme tokens only add Typography (4 custom text-* tokens, since 28px has no default Tailwind size) and the Color accent/event-badge hex values -- the Spacing Scale needs no custom tokens because UI-SPEC's 4/8/16/24/32/48/64px steps are exactly Tailwind's own default 1/2/4/6/8/12/16 scale"

patterns-established:
  - "New domain package template: Store interface + Service + NewService + var _ Store = (*Service)(nil), page-size/limit clamps live in the domain not the HTTP layer"
  - "web/app/lib/api.ts as the single frontend API surface -- shared, never forked per-route"

requirements-completed: [HIST-01, UI-03]

coverage:
  - id: D1
    description: "GET /events returns a keyset-paginated, newest-first JSON envelope ({\"events\": [...], \"next_cursor\": ...}), backed by a new sqlc ListEvents query and internal/events.Service"
    requirement: HIST-01
    verification:
      - kind: integration
        ref: "internal/httpserver/events_test.go#TestListEvents_OrderedNewestFirstAndKeysetPaginates"
        status: pass
      - kind: integration
        ref: "internal/httpserver/events_test.go#TestListEvents_NoMatchingRowsReturnsNonNilEmptySlice"
        status: pass
      - kind: unit
        ref: "internal/httpserver/events_test.go#TestHandleListEvents_EmptyReturnsEmptyArrayAndNullCursor"
        status: pass
      - kind: unit
        ref: "internal/httpserver/events_test.go#TestHandleListEvents_StoreErrorReturns500WithFixedMessage"
        status: pass
    human_judgment: false
  - id: D2
    description: "Opening the single Go binary's root URL renders a History page fetching real events from GET /events, embedded via go:embed with no Node process and no second server"
    requirement: UI-03
    verification:
      - kind: integration
        ref: "internal/httpserver/spa_test.go#TestSPA_RootPathReturns200WithHTML"
        status: pass
      - kind: integration
        ref: "internal/httpserver/spa_test.go#TestSPA_UnregisteredPathFallsBackToIndexHTML"
        status: pass
      - kind: integration
        ref: "internal/httpserver/spa_test.go#TestSPA_APIRoutesStillReachTheirOwnHandlers"
        status: pass
      - kind: manual_procedural
        ref: "go build -o ./bin/server ./cmd/server && DATABASE_URL=... HTTP_PORT=8099 ./bin/server; curl / -> 200 text/html; curl /events -> 200 {\"events\":[],\"next_cursor\":null}"
        status: pass
    human_judgment: false

duration: ~100min (approximate -- not precisely timestamped at session start; includes a mid-session pause for an unrelated plan-mode toggle)
completed: 2026-08-11
status: complete
---

# Phase 6 Plan 1: End-to-End Release History Slice Summary

**A new sqlc `ListEvents` keyset query backs `internal/events.Service`, exposed as `GET /events`, and served next to a React Router SPA (SPA Mode, no Node runtime) embedded into the single Go binary via `go:embed` -- opening the binary's root URL renders a real History page fetching live Postgres data.**

## Performance

- **Duration:** ~100 min (approximate)
- **Completed:** 2026-08-11
- **Tasks:** 1 of 2 plan tasks required implementation (Task 1 was a package-legitimacy checkpoint, pre-approved by the human before this executor ran); Task 2 (the tracer) is the sole implementation task, committed atomically as a single unit per the plan's explicit hard-ordering constraint (sqlc codegen must precede consuming Go code, and the frontend build must exist before the embed can resolve)
- **Files modified/created:** 66 (see commit `4966208`)

## Accomplishments

- Backend: `queries/events.sql`'s new `ListEvents :many` (keyset-paginated on `id DESC`, `sqlc.narg` optional `artist_id`/`event_type`/`cursor` filters), `internal/events` domain package (`Event`/`ListParams`/`Page`/`Store`/`Service`, page size clamped to 24 default / 100 max), and `GET /events` wired into `internal/httpserver` following every existing handler convention (`writeError`, `httplog.SetAttrs`, non-nil-slice backstop).
- `internal/webassets` (new package): `//go:embed all:build/client` plus a chi `NotFound`-registered SPA fallback that serves `index.html` for any unmatched client-side route while every explicit API route still resolves to its own handler.
- `web/`: a genuinely greenfield React Router SPA Mode scaffold (confirmed `ssr: false` produces `build/client/` and never `build/server/`), Tailwind v4 dark theme tokens transcribed from `06-UI-SPEC.md`, the full locked shadcn component inventory, a typed `lib/api.ts` covering all five backend endpoints (`listEvents`, `listWatchlist`, `addWatchlist`, `updateWatchlistPreferences`, `removeWatchlist`, `searchArtists`), shared `CoverArt`/`EmptyState` components, and a `History` route rendering real server data with the locked empty-state copy.
- `Makefile`'s new `web` target builds the SPA and replaces the committed `internal/webassets/build/client/` tree, so `go build`/`go vet`/`go test ./...` all work on a fresh clone that has never run the Node toolchain (verified by inspection, not just by convention).
- Proved the entire slice end-to-end by hand: `go build -o ./bin/server ./cmd/server` then running it against the real dev Postgres -- `GET /` returns 200 HTML, `GET /history` falls back to the same `index.html` (client-side routing), `GET /events` returns `{"events":[],"next_cursor":null}`, and `/health` still resolves to its own handler.

## Task Commits

Task 1 (package-legitimacy checkpoint) required no commit -- it built nothing, only gated Task 2's installs, and was approved by the human via the orchestrator before this executor was spawned.

1. **Task 2: End-to-end "browse release history" -- one path through every layer** - `4966208` (feat)

**Plan metadata:** committed separately below (SUMMARY.md + REQUIREMENTS.md, worktree mode).

## Files Created/Modified

- `queries/events.sql` - new `ListEvents :many` keyset-paginated query
- `internal/db/sqlc/events.sql.go`, `internal/db/sqlc/querier.go` - sqlc-generated bindings for `ListEvents`
- `internal/events/service.go` - new domain package: `Event`, `ListParams`, `Page`, `Store`, `Service`, `NewService`, `DefaultPageSize`, `MaxPageSize`
- `internal/httpserver/events.go` - `eventsResponse`, `handleListEvents`
- `internal/httpserver/server.go` - `Server.events` field, widened `New(...)`, `/events` route, `r.NotFound(webassets.Handler()...)`
- `internal/httpserver/events_test.go` - `stubEventsStore`, handler-level and real-Postgres keyset-pagination tests
- `internal/httpserver/spa_test.go` - root/fallback/API-route-precedence tests for the embed wiring
- `internal/httpserver/{health,search,server,watchlist,boot_e2e}_test.go` - updated every `httpserver.New(...)` call site for the new `events.Store` parameter
- `internal/webassets/embed.go` - `//go:embed all:build/client`, `Handler()`
- `internal/webassets/build/client/` - committed SPA build output (go:embed resolves at compile time)
- `cmd/server/main.go` - constructs `events.NewService(sqlc.New(pool))`, threads it into `httpserver.New`
- `Makefile` - `web` target
- `.gitignore` - `web/build/`, `web/.react-router/`
- `web/` (new tree) - full frontend scaffold: `react-router.config.ts`, `vite.config.ts`, `app.css`, `root.tsx`, `routes.ts`, `lib/api.ts`, `components/common/{CoverArt,EmptyState}.tsx`, `routes/history.tsx`, shadcn `components/ui/*`

## Decisions Made

- Kept `@react-router/node` and let `react-router typegen` self-restore `isbot` after I removed it -- both proved load-bearing for the build-time root prerender even under `ssr: false`, confirmed by the tool re-adding `isbot` on its own and by a successful rebuild.
- Removed the scaffold's `@react-router/serve` dependency and `package.json`'s `start` script (`react-router-serve ./build/server/index.js`) -- `build/server` never exists under SPA Mode, and CLAUDE.md forbids a Node runtime in production regardless.
- Deleted the scaffold-generated `web/Dockerfile` and `web/.dockerignore` -- they build and run a standalone Node server, directly contradicting the single-Go-binary architecture; Phase 7 owns the real Docker build path (noted in the `Makefile`'s `web` target comment).
- Pinned `typescript` to `5.9.3` (verified via `npm view typescript dist-tags`) instead of the scaffold's default `^6`, since there is no stable `6.x` release (only a `6.0.0-beta` tag) and `latest` resolves to a `7.x` pre-release -- exactly the risk `06-RESEARCH.md` and Task 1's checkpoint flagged.
- Applied `class="dark"` unconditionally on `<html>` in `root.tsx` rather than wiring a light/dark toggle -- `06-UI-SPEC.md` D-13 specifies dark-themed as this app's only visual target.
- `routes.ts` gives `index()` and `route("history", ...)` explicit distinct `id` options since both point at the same `history.tsx` file and React Router's default file-path-derived id collided.
- Tailwind v4 `@theme` additions are minimal: four custom `text-*` tokens (Typography, since 28px has no default Tailwind size) plus the Color accent/event-badge hex values -- the Spacing Scale needed no custom tokens because UI-SPEC's 4/8/16/24/32/48/64px steps are exactly Tailwind's default 1/2/4/6/8/12/16 scale.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed dead `start` script and `@react-router/serve` dependency**
- **Found during:** Task 2, Step 5 (frontend scaffold)
- **Issue:** The shadcn `--template react-router` scaffold's default `package.json` ships a `start` script (`react-router-serve ./build/server/index.js`) that would fail immediately -- `ssr: false` (this task's own Step 5 fix) guarantees `build/server/` is never produced.
- **Fix:** Removed the `start` script and the now-unused `@react-router/serve` dependency; verified `pnpm install` and `pnpm run build` still succeed.
- **Files modified:** `web/package.json`, `web/pnpm-lock.yaml`
- **Committed in:** `4966208` (part of the single task commit)

**2. [Rule 2 - Missing critical] Removed the scaffold's Node-runtime Dockerfile**
- **Found during:** Task 2, Step 5
- **Issue:** The generated `web/Dockerfile` builds and runs a standalone Node server (`CMD ["npm", "run", "start"]`), directly contradicting CLAUDE.md's single-Go-binary/no-Node-runtime-in-production constraint. Left in place, it would be a discoverable, contradictory, and misleading artifact for Phase 7's own containerization work.
- **Fix:** Deleted `web/Dockerfile` and `web/.dockerignore`.
- **Files modified:** `web/Dockerfile` (removed), `web/.dockerignore` (removed)
- **Committed in:** `4966208`

**3. [Rule 1 - Bug] Corrected `typescript` version pin**
- **Found during:** Task 2, Step 5
- **Issue:** The scaffold's default `package.json` pinned `typescript: "^6"`, but npm has no stable `6.x` release (only a `6.0.0-beta` pre-release tag) -- this range would either fail to resolve a stable version or silently resolve to a pre-release.
- **Fix:** Pinned to `5.9.3` (verified newest stable `5.x` via `npm view typescript dist-tags`), per Task 1's checkpoint condition and `06-RESEARCH.md` Assumption A4.
- **Files modified:** `web/package.json`
- **Committed in:** `4966208`

---

**Total deviations:** 3 auto-fixed (2 Rule 1 bug fixes, 1 Rule 2 missing-critical cleanup)
**Impact on plan:** All three are corrections to the shadcn/create-react-router scaffold's defaults that would otherwise ship dead or architecture-contradicting tooling; none change the plan's locked architecture (SPA Mode, go:embed, single binary), and none required a new checkpoint since Task 1's package-legitimacy approval already covered this exact scaffold command.

## Issues Encountered

- **Windows line-ending noise (pre-existing, out of scope):** `sqlc generate` and `gofmt` both flag five pre-existing generated files (`artists.sql.go`, `db.go`, `health.sql.go`, `models.go`, `watchlist.sql.go`) as "modified" purely due to CRLF/LF normalization on this Windows dev box -- confirmed via `git diff --ignore-space-at-eol` showing zero real content changes. Left unstaged; this matches prior phases' documented Windows-toolchain quirks (STATE.md) and is unrelated to this plan's changes.
- **`-race` unusable on this dev box (pre-existing, documented elsewhere):** `go test ./... -race` fails with ThreadSanitizer allocation errors across every package, including ones untouched by this plan -- the same pre-existing cgo/TSan toolchain break already documented in STATE.md for phases 01-02/01-03. Verified correctness instead via the full non-race suite (`go test ./... -count=1`), which is fully green.
- **shadcn init scaffolded into a nested `web/react-router-app/` subdirectory** instead of `web/` directly (its interactive project-name prompt defaulted to `react-router-app`) -- moved all files up to `web/` and removed the nested `.git` the scaffold's own tooling initializes (which would otherwise shadow this worktree's git tracking for that subtree).
- **`react-router build` internally builds a transient server bundle even under `ssr: false`**, then deletes it as its final build step ("Removing the server build ... due to ssr:false") -- this is the tool's own documented behavior, not a bug; verified `build/server/` does not exist on disk after the build completes.

## User Setup Required

None - no external service configuration required. (Docker/Postgres for local dev and testing were already documented and running via `make db-up`, unchanged from prior phases.)

## Next Phase Readiness

- `internal/events.Store`/`ListParams`/`Page`, `web/app/lib/api.ts`'s full six-wrapper surface, and `web/app/app.css`'s theme tokens are all in place for plans 06-02 (History filters/type-specific cards), 06-03 (Watchlist tab), and 06-04 to build against in parallel without editing each other's files, per the plan's stated goal.
- `web/app/routes.ts` currently registers only the History route (index + `/history`) -- 06-03 adds `/watchlist` alongside it and extends `root.tsx`'s tab bar with the second `NavLink`.
- No blockers. The `GET /events` handler intentionally reads no query parameters yet (plan 06-02's scope); the domain (`events.ListParams`) and wire types (`web/app/lib/api.ts`'s `listEvents(params?)`) are already shaped to accept `artistId`/`eventType`/`cursor` so that plan is additive, not a rewrite.

---
*Phase: 06-frontend-release-history*
*Completed: 2026-08-11*

## Self-Check: PASSED

All 11 claimed key files verified present on disk (`internal/events/service.go`, `internal/httpserver/events.go`, `internal/httpserver/events_test.go`, `internal/httpserver/spa_test.go`, `internal/webassets/embed.go`, `internal/webassets/build/client/index.html`, `web/app/lib/api.ts`, `web/app/routes/history.tsx`, `web/app/components/common/CoverArt.tsx`, `web/app/components/common/EmptyState.tsx`, this SUMMARY.md). Commit `4966208` verified present in `git log`.
