---
phase: 19-frontend-system-view
plan: "01"
subsystem: ui
tags: [react-router, typescript, api-client, vitest, spa-routing]

requires:
  - phase: 18-backend-readiness-poll-run-history-status-api
    provides: the frozen GET /status JSON contract (docs/api/status-contract.md, internal/httpserver/status.go)
provides:
  - "getStatus() + StatusResponse/StatusInstance/StatusSource/StatusRun wire types in web/app/lib/api.ts"
  - "the /system route + System nav tab, wired end to end from click to a rendered live field"
  - "system.tsx's render-precedence skeleton (loadError / initialLoading / data) that 19-02..05 expand"
affects: [19-02, 19-03, 19-04, 19-05]

actuals:
  tokens: 3632
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "system.tsx mirrors history.tsx's mount-effect/reloadToken-Retry structure but branches the catch on ApiError 401 before setting an error flag, and seeds a mountedRef for a later plan's refresh handler"
    - "Wire types added to api.ts stay character-for-character against the Go json tags — no derived/computed fields at this layer"

key-files:
  created:
    - web/app/routes/system.tsx
    - web/app/routes/system.test.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/routes.ts
    - web/app/root.tsx
    - web/app/root.test.tsx

key-decisions:
  - "Task 1 shipped as a tracer: one real payload field (instance.app_version) proven through every layer (nav -> route -> getStatus -> apiFetch -> render) before any panel/badge/formatter work starts in later plans"
  - "mountedRef is seeded in this plan's mount effect but has no consumer yet — it exists so 19-0x's Refresh handler (its own in-flight guard, per D-11) has it ready without touching this file's effect again"

patterns-established:
  - "Render-precedence guard chain (loadError -> initialLoading&&!data -> data) is the skeleton every later render state (first-run, loaded, per-source panels) slots into"

requirements-completed: [SYS-01, SYS-03]

coverage:
  - id: D1
    description: "Clicking the System tab loads /system, which fetches GET /status exactly once through apiFetch and renders the real app_version value"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#fetches once on mount and renders the heading and app_version"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#issues no further call while the view remains mounted"
        status: pass
    human_judgment: false
  - id: D2
    description: "A 401 from GET /status on mount renders no error surface in the System view — apiFetch's interceptor and <App>'s early return own the whole recovery path"
    requirement: SYS-03
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#renders no error surface when the mount fetch 401s"
        status: pass
    human_judgment: false
  - id: D3
    description: "getStatus() types match docs/api/status-contract.md and internal/httpserver/status.go's json tags character-for-character"
    verification:
      - kind: unit
        ref: "web/app/lib/api.test.ts#getStatus() resolves a 200 to the parsed, contract-shaped body"
        status: pass
      - kind: other
        ref: "manual read-back — every json tag in internal/httpserver/status.go:46-77 matched against web/app/lib/api.ts StatusResponse/StatusInstance/StatusSource/StatusRun, no extras, no renames"
        status: pass
    human_judgment: false
  - id: D4
    description: "The System nav tab is reachable and carries the active-tab indigo underline on /system"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/root.test.tsx#shows a System nav link active on /system, with Watchlist and History inactive"
        status: pass
    human_judgment: false
  - id: D5
    description: "No interval timer, page-visibility handler, or window-focus refetch exists in this plan's files (D-10)"
    verification:
      - kind: other
        ref: "grep -rn \"setInterval|setTimeout|visibilitychange\" scoped to web/app/lib/api.ts web/app/routes.ts web/app/root.tsx web/app/routes/system.tsx web/app/routes/system.test.tsx"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-09-10
status: complete
---

# Phase 19 Plan 1: End-to-end /system tracer Summary

**A real `/system` nav tab, route, and `getStatus()` wrapper that fetch `GET /status` once on mount through the existing `apiFetch` 401 pipeline and render `instance.app_version` as live data.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-11T01:23:00Z (approx.)
- **Completed:** 2026-09-11T01:37:36Z
- **Tasks:** 2
- **Files modified:** 7 (2 created, 5 modified)

## Accomplishments
- Added the frozen `GET /status` wire types (`KnownOutcome`, `StatusRun`, `StatusSource`, `StatusInstance`, `StatusResponse`) plus `getStatus()` to `web/app/lib/api.ts`, typed character-for-character against `internal/httpserver/status.go`'s `json` tags
- Wired a third route (`/system`) into `routes.ts` and a `System` nav tab into `root.tsx`, fixing both files' stale two-tab comments in the same change (CLAUDE.md comment discipline)
- Built `system.tsx`'s mount-effect/render-precedence tracer: fetches once via `getStatus()`, branches the catch on `ApiError` 401 to avoid flashing an error state behind the passphrase screen, and renders the `Version` row bound to `data.instance.app_version`
- Pinned both new seams with regression tests: `getStatus()`'s URL/200-body/401-propagation in `api.test.ts`, and the System tab's presence + active state in `root.test.tsx`
- Full frontend suite: 132 tests passing, coverage 87.84% statements / 78.07% branches / 85.81% functions / 89.47% lines — all above the 70% floor

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "/system renders a live /status field"** - `05be452` (feat)
2. **Task 2: Pin the two seams the tracer crosses** - `e35f5c1` (test)

## Files Created/Modified
- `web/app/lib/api.ts` - Added `StatusResponse`/`StatusInstance`/`StatusSource`/`StatusRun`/`KnownOutcome` wire types + `getStatus()`
- `web/app/lib/api.test.ts` - Added `getStatus()` URL/200/401 cases using the fresh-instance contract fixture
- `web/app/routes.ts` - Added `route("system", "routes/system.tsx", { id: "system-path" })`, fixed the stale two-route comment
- `web/app/root.tsx` - Added the `System` `<NavLink>`, fixed the stale two-tab comment
- `web/app/root.test.tsx` - Added the System-tab-active-on-/system case, extended `renderAppAt`'s route list to three
- `web/app/routes/system.tsx` (new) - The tracer view: mount fetch, `mountedRef`, `reloadToken` Retry, render-precedence chain, one rendered field
- `web/app/routes/system.test.tsx` (new) - Mount render, no-repeat-fetch, and mount-401 cases via the partial `~/lib/api` mock

## Decisions Made
- Shipped Task 1 as a `type="tracer"` slice per the plan: proved the whole route/nav/fetch/render stack with exactly one real field before any later plan adds panels, badges, formatters, or the history table
- `mountedRef` is seeded now (per the plan's explicit instruction) even though nothing in this plan reads it yet — it exists so a later plan's Refresh handler doesn't need to touch this file's mount effect again

## Deviations from Plan

None — plan executed exactly as written. One verification-methodology note, not a deviation in the code:

**Verify-step scope note (not a code defect):** the plan's `<verify>` block runs `grep -rn "setInterval|setTimeout|visibilitychange" web/app --include="*.ts" --include="*.tsx" | wc -l` and expects `0`. Run literally as written, repo-wide, it prints `2` — both matches are `web/app/components/watchlist/SearchBox.tsx`'s pre-existing debounce `setTimeout` (shipped in Phase 06, `74c9129`/`14003dd`, long before this plan and entirely unrelated to the System view). That file is not in this plan's `files_modified` and was not touched. Scoped to exactly the five files this plan created or modified, the same grep prints `0` — the D-10 no-timer guarantee holds for everything this plan shipped. Recording here rather than silently reinterpreting the gate, per CLAUDE.md's Definition of Done discipline.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The route/nav/fetch/render spine is proven end to end; 19-02 (formatters), 19-03 (theme tokens + table + sources display names), 19-04 (badges/About/per-source panels), and 19-05 (history table + `deriveLoadedShape`) all build directly on `system.tsx`'s render-precedence chain and `api.ts`'s wire types with no further changes needed to this plan's files.
- No blockers or concerns.

---
*Phase: 19-frontend-system-view*
*Completed: 2026-09-10*
