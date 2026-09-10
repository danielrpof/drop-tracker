# Phase 19: Frontend — System View - Research

**Researched:** 2026-09-10
**Domain:** React 19 + React Router 7 SPA (embedded, `web/`); rendering a frozen JSON contract; dependency-free date/duration formatting; vitest component testing
**Confidence:** HIGH — this is a pure-frontend phase against a contract frozen and shipped in Phase 18; every source of truth is in-repo and was read this session.

## Summary

Phase 19 adds a third top-level tab (`/system`) to the embedded SPA that renders
`GET /status` for an operator. There is **no backend work**: the contract is frozen in
`docs/api/status-contract.md` and `internal/httpserver/status.go`, and Phase 18.1 only
populates run data without changing shape. The phase is almost entirely: (1) new wire
types + a `getStatus()` wrapper in `web/app/lib/api.ts`; (2) a new
`web/app/routes/system.tsx` that mirrors `history.tsx`'s fetch-on-mount / skeleton /
Retry structure but diverges on refresh (keep stale data, D-11/D-12); (3) a new
dependency-free `web/app/lib/format.ts` (relative time, absolute time, `duration_ms`,
`poll_interval_seconds`) — nothing like it exists in the SPA today; (4) one shadcn
`table` component vendored from the `base-maia` registry; (5) two new `@theme` color
tokens; (6) one `<NavLink>` and one `route(...)` entry.

The design contract (`19-UI-SPEC.md`) is approved and locks every copy string, the five
render states, the badge tiers (D-07), and the `<Card>`/`<Table>`/`<Alert>` component
split. The state machine must be **recomputed on every successful fetch** (not latched)
and the Refresh handler needs its **own** in-flight/abort guard because `history.tsx`'s
`let cancelled = false` only covers the mount effect.

**Primary recommendation:** Build `system.tsx` from the `history.tsx` skeleton, add
`getStatus()` + wire types to `api.ts` typed character-for-character against
`status.go`'s `json:"..."` tags, put all derived-display logic in a pure, unit-tested
`format.ts` + a pure `deriveViewState(data)` function, and drive every string from the
UI-SPEC Copywriting Contract verbatim. Zero new runtime dependencies.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `/status` data production, schema read, watchlist count, ring-buffer snapshot | API / Backend | — | Frozen in Phase 18; Phase 19 must not touch it |
| Fetch orchestration, 401 interception, `X-Instance-Gated` latch | Browser / Client (`api.ts` `apiFetch`) | — | Single existing fetch path; `getStatus` funnels through it |
| Render-state machine (loading/error/session-expired/first-run/loaded) | Browser / Client (`system.tsx`) | — | Pure derivation from the payload + fetch outcome |
| Derived display formatting (relative/absolute time, duration, interval, badge tier, "clean run" scan) | Browser / Client (`format.ts` + system components) | — | UI-only concern; contract carries raw counts/enums/timestamps |
| Session-expiry → passphrase screen | Frontend Server / Client (`root.tsx` `<App>` early-return + `authStore`) | `api.ts` D-16 interceptor | Already built (Phase 14); Phase 19 inherits it with zero new code |
| Routing + nav tab | Browser / Client (`routes.ts`, `root.tsx`) | — | One `route(...)` + one `<NavLink>` |

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01: "Database reachable" is derived from `/status`, not a second request.**
  `instance.schema_applied === null` ⇒ "database unreachable" (red pill); any integer ⇒
  reachable. No `fetch('/ready')`, no second loading/error path.
- **D-02: Schema version shows one number when in sync, flags a mismatch when not.**
  `schema_applied === schema_expected` → "schema 7". `schema_applied !== schema_expected`
  (and not `null`) → "applied 8 · expects 7" with a `status-warn` warning treatment +
  `TriangleAlert`. Never render a bare `null` (D-01 pill covers that).
- **D-03: `app_version` renders as plain `font-mono` text.** Short SHA as-is; `"dev"`
  literal. No link (no repo base URL in the frontend, out of scope).
- **D-04: Five render states** — `loading` (skeleton) → `error` (non-401 threw; inline
  Retry, header Refresh **hidden**) → `session-expired` (401 → existing `PassphraseScreen`
  via `apiFetch`'s D-16 interceptor, **no per-view code**) → `first-run` (`200` where
  **every** source has `last_run === null` **and** `history.length === 0` **and**
  `consecutive_skips === 0`) → `loaded`.
  - The `consecutive_skips === 0` clause is load-bearing: a skipping-but-runless instance
    renders `loaded`, not first-run.
  - **Recompute the state machine on every successful fetch, not just on mount.** A Refresh
    right after a restart can move `loaded` → `first-run` (in-process buffer reset). Do not
    latch to `loaded`.
  - **Unknown `outcome` never white-screens.** `outcome` set is frozen (`ok`/`error`/
    `cancelled`) but an N-1/N deploy could surface a newer value → neutral grey badge with
    a title-cased fallback label (never the raw string); the run's `summary` still renders.
- **D-05: `watchlist_size === 0` is an inline callout**, not a whole-page state. Shown in
  `first-run` and `loaded`, with a link to the Watchlist tab. Panels + table still render.
- **D-06: First-run copy names the humanized poll interval** (from `poll_interval_seconds`).
  Exception — `schema_applied === null` in first-run: swap the body to the "Can't reach the
  database…" copy (still a `200`, not the `error` state).
- **D-07: Badge tiers, two client-derived.** `ok` && `artists_errored === 0` → **Success**
  (green `status-ok`); `ok` && `artists_errored > 0` → **Completed with errors** (amber
  `status-warn`, derived); `error` → **Failed** (red `destructive`); `cancelled` →
  **Interrupted** (amber `status-warn` **+ leading icon** to stay glanceably distinct from
  "Completed with errors"); unrecognised → **grey** `secondary`, title-cased fallback.
  Never render the raw enum. Plus a per-source `status-warn` escalation line ("Recent
  cycles are being interrupted — check for a restart or crash loop.") when that source's
  **latest** run is `cancelled`.
- **D-08: "Time since last clean run" (SYS-01) scans `history`** newest-first for the first
  element with `outcome === "ok"` **and** `artists_errored === 0`; relative time from its
  `finished_at`. None → "No clean run in recent history" (literal, not a date, not blank).
  A "Completed with errors" run is **not** clean.
- **D-09: Timestamps render absolute-primary, relative-on-hover, frozen at fetch.**
  `started_at`/`finished_at`/`last_skipped_at` render as an **absolute** local time (visible),
  with relative phrasing + full ISO (with offset) in `title`. The **one** exception is
  D-08's "last clean run" line (relative-primary).
  - **D-09-a absolute format:** `HH:MM:SS` (24h local) when the timestamp is on the client's
    current date; `MMM D, HH:MM` when it is an earlier day. Full ISO + offset in `title`.
  - **Clock-skew clamp:** any computed relative delta `< 0` → "just now", never "in 3
    minutes". Lives in the shared formatter.
  - `null` → "—" everywhere, never blank, never "Invalid Date".
- **D-10: Fetch on mount + manual Refresh only. No timers.** No `setInterval`, no
  `visibilitychange`, no focus refetch.
- **D-11: An in-flight Refresh keeps the previous data on screen.** Button shows a spinner
  and disables; content stays; "as of" updates only on success. Diverges from `history.tsx`.
  - **The refresh path needs its own in-flight guard** — `history.tsx`'s `let cancelled =
    false` only covers the mount effect. A Refresh that resolves/401s after the session
    expired and `<Outlet>` remounted must not `setState` on the unmounted view.
- **D-12: A failed Refresh (non-401) keeps stale data + shows an inline error** ("Couldn't
  refresh — still showing data as of <time>.") near the button; "as of" holds at the last
  success. A failed **initial** load still goes to the full `error` state.
- **D-13: "as of" is the client clock when the fetch resolved.** Visible "as of HH:MM:SS"
  (absolute, 24h local, seconds precision). Not from any payload field. Rendered in
  `first-run` too.

### Claude's Discretion

- **"System" nav placement:** a third top-level tab at `/system`, styled with the existing
  `tabLinkClassName` in `web/app/root.tsx`. `routes.ts` gains one `route("system", ...)`.
- **Recent-runs history table layout** — deferred to `19-UI-SPEC.md` (now resolved there:
  one table per source, inside that source's panel, below a `<Separator>`; 8 columns;
  `<TableCaption>` surfaces the 50-cap).
- Exact copy strings for every state, component decomposition, skeleton shape — planner +
  UI-SPEC (UI-SPEC has locked all copy).

### Deferred Ideas (OUT OF SCOPE)

- A real `/ready` readiness badge with a `db_unreachable`/`schema_behind`/`schema_dirty`
  reason enum (bare `fetch('/ready')`).
- Auto-refresh / live updates (OBS-01) — explicitly rejected (D-10).
- A "poll now" trigger (OBS-02).
- Paginated / filterable run history beyond the last 50 (OBS-03).
- Poll-failure alerting (OBS-04).
- Time-series charts / sparklines — **permanent** Out of Scope (REQUIREMENTS.md): would add
  a charting dependency.
- Linking `app_version` to a GitHub commit — needs a repo base URL wired into the frontend.
- Moving `shadcn` out of `web/package.json` dependencies — separate `/gsd-quick` backlog
  item, not a Phase 19 concern.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SYS-01 | "System" view in main nav; per source: last run time, outcome, duration, counts, + time since last successful run | `getStatus()` + `StatusSource`/`StatusRun` wire types; per-source `<Card>` panel; `classifyRun()` badge tiers (D-07); `format.ts` `formatAbsoluteTime`/`formatDuration`; D-08 clean-run `history` scan; `sourceDisplayName()` in `sources.ts` |
| SYS-02 | Recent-runs history table + watchlist size + poll interval + about block (app version, schema version, database reachable) | shadcn `table` (per-source, in-panel); About `<Card>` reading `instance.*` + `watchlist_size` + `poll_interval_seconds`; D-01 DB pill, D-02 schema line, D-03 version text; `formatPollInterval` |
| SYS-03 | Fetch on mount + manual Refresh, no fast auto-polling; reuse empty-state + 401 → passphrase handling | `history.tsx` mount-effect pattern; `apiFetch` D-16 401 interceptor (no per-view code); D-11/D-12 keep-stale Refresh + its own guard; D-10 no timers; `EmptyState` for first-run/error; `<Alert>` for empty-watchlist |

## Standard Stack

**Zero new runtime dependencies.** Everything below is already in `web/package.json`
`[VERIFIED: web/package.json:13-48]`.

### Core (already present)

| Library | Version | Purpose | Notes |
|---------|---------|---------|-------|
| `react` / `react-dom` | `^19.2.6` | UI runtime | `[VERIFIED: web/package.json:23-24]` |
| `react-router` | `7.18.2` | Routing, `<NavLink>`, `<Link>`, `<Outlet>`, `createRoutesStub` | SPA mode (`ssr: false`) `[VERIFIED: web/package.json:24]` |
| `@base-ui/react` | `^1.7.0` | Primitive layer under `Button`/`Badge`/`Card` | `[VERIFIED: web/package.json:14]` |
| `lucide-react` | `^1.31.0` | Icons: `RefreshCw`, `Loader2`, `TriangleAlert`, `Ban` | `[VERIFIED: web/package.json:20]` |
| `tailwindcss` | `^4` | Styling; `@theme` tokens in `app/app.css` | `[VERIFIED: web/package.json:44]` |
| `clsx` + `tailwind-merge` | `^2.1.1` / `^3.6.0` | `cn()` in `~/lib/utils` | `[VERIFIED: web/app/lib/utils.ts:1-6]` |

### Supporting (dev / test, already present)

| Library | Version | Purpose |
|---------|---------|---------|
| `vitest` | `4.1.10` | Test runner (`vitest run`) `[VERIFIED: web/package.json:47]` |
| `@vitest/coverage-v8` | `4.1.10` | Coverage provider, 70%-all-axes gate `[VERIFIED: web/vitest.config.ts:27-63]` |
| `@testing-library/react` | `16.3.2` | Component render/query `[VERIFIED: web/package.json:35]` |
| `@testing-library/user-event` | `14.6.4` | Interaction simulation `[VERIFIED: web/package.json:36]` |
| `@testing-library/jest-dom` | `7.0.1` | DOM matchers (wired in `vitest.setup.ts`) `[VERIFIED: web/vitest.setup.ts:1-18]` |
| `jsdom` | `30.0.1` | Test DOM environment `[VERIFIED: web/vitest.config.ts:17]` |
| `prettier` + `prettier-plugin-tailwindcss` | `^3.8.3` / `^0.8.0` | Formatting — **DoD gate**, hand-formatted TSX fails CI `[VERIFIED: web/package.json:42-43]` |

### Date/time approach — no library

`web/app/lib/utils.ts` holds **only** `cn()` `[VERIFIED: web/app/lib/utils.ts:1-6]`. No
date/relative-time helper exists anywhere in the SPA; `EventCard.tsx` renders
`release_date` as a raw string with a `?? "Release date unknown"` fallback
`[VERIFIED: web/app/components/history/EventCard.tsx:108,162,198]`. D-09 introduces
relative-time rendering to this codebase for the first time.

**Use native `Date` + `Intl.DateTimeFormat` only.** CONTEXT locks "zero new deps here", so
`date-fns` / `dayjs` / `luxon` are out. All four formatters are small pure functions:

| Formatter | Signature (recommended) | Rules |
|-----------|-------------------------|-------|
| relative time (D-09, D-08) | `formatRelativeTime(iso: string \| null, now?: Date): string` | `null` → `"—"`; delta `< 0` → `"just now"`; `< ~45s` → `"just now"`; `< ~90s` → `"1 minute ago"`; `< 60m` → `"N minutes ago"`; `< ~90m` → `"1 hour ago"`; `< 24h` → `"N hours ago"`; `< 48h` → `"1 day ago"`; else `"N days ago"` |
| absolute time (D-09-a) | `formatAbsoluteTime(iso: string \| null, now?: Date): string` | `null` → `"—"`; same calendar day as `now` → `"HH:MM:SS"` (24h local); earlier day → `"MMM D, HH:MM"` |
| full timestamp for `title` (D-09) | `formatIsoTitle(iso: string): string` | full local timestamp **with offset** (e.g. `toLocaleString` + offset, or the raw ISO string) |
| clock stamp (D-13) | `formatClock(d: Date): string` | `"HH:MM:SS"` 24h local — the "as of" value |
| duration (SYS-01) | `formatDuration(ms: number \| null): string` | `null` → `"—"`; `< 1000` → `"{ms}ms"`; `< 60000` → `"{(ms/1000).toFixed(1)}s"`; else `"{m}m {ss.padStart(2,'0')}s"` (e.g. `63000` → `"1m 03s"`) |
| poll interval (SYS-02, D-06) | `formatPollInterval(seconds: number): string` | `900` → `"15 minutes"`, `90` → `"90 seconds"`; `< 60` → `"N seconds"`; divisible by 60 → `"N minutes"` (pluralize); optionally `≥ 3600` && divisible by 3600 → `"N hours"` |

Inject `now` as a parameter (default `new Date()`) so tests are deterministic — mirrors how
`history.test.tsx` stays deterministic by controlling inputs.

**Timezone caveat (test determinism):** `formatAbsoluteTime` / `formatClock` render *local*
time, so `HH:MM:SS` assertions depend on the runner's timezone. Recommended: add
`test: { env: { TZ: "UTC" } }` to `web/vitest.config.ts` (one line, isolated), **or** build
each expected string in-test from the same `Intl` call. Flag for the planner to decide.

## Package Legitimacy Audit

> This phase installs **no npm packages**. `pnpm install` is not run. The only vendoring
> action is `npx shadcn@latest add table`, which writes a first-party component file from
> the official shadcn registry — it is not a dependency and adds nothing to
> `package.json` / `pnpm-lock.yaml`.

| Item | Registry | Verdict | Disposition |
|------|----------|---------|-------------|
| shadcn `table` component (`base-maia` style) | shadcn official registry (`https://ui.shadcn.com/r/styles/base-maia/table.json`) | OK — first-party, `registryDependencies: none`, `dependencies: ["cn"]` (already vendored), installs `registry/base-maia/ui/table.tsx` | Approved — vendor via `npx shadcn@latest add table` `[VERIFIED: https://ui.shadcn.com/r/styles/base-maia/table.json — fetched 2026-09-10; corroborated by 19-UI-SPEC.md:64,85 curl evidence 2026-09-10]` |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
                     ┌───────────────────────── browser (SPA) ─────────────────────────┐
 operator clicks     │                                                                 │
 "System" tab   ────►│  <NavLink to="/system">  ──►  routes.ts route("system")         │
                     │                                     │                           │
                     │                              routes/system.tsx (mount)          │
                     │                                     │                           │
                     │                     useEffect: getStatus()  ◄── Retry bumps      │
                     │                                     │          reloadToken       │
                     │                     handleRefresh(): getStatus()  ◄── Refresh    │
                     │                                     │          button (D-11)     │
                     │                                     ▼                           │
                     │                     api.ts  apiFetch<StatusResponse>("/status")  │
                     │                        │        │            │                  │
                     │            X-Instance-Gated   401 → D-16     ok → JSON body      │
                     │            latch (authStore)  authStore.markUnauthenticated()    │
                     │                                  │                │              │
                     │                        <App> re-renders     setData(body)        │
                     │                        → <PassphraseScreen>  setAsOf(new Date())  │
                     │                                                   │              │
                     │                     deriveViewState(data): first-run | loaded    │
                     │                                                   │              │
                     │   ┌───────────────┬───────────────┬──────────────┴────────────┐  │
                     │   │  About <Card> │ empty-watchlist│  per source (MB, Deezer): │  │
                     │   │  version/     │  <Alert> if    │   <Card>                  │  │
                     │   │  schema/DB    │  watchlist=0   │    last-run line + badge  │  │
                     │   │  /watchlist/  │  (D-05)        │    summary (verbatim)     │  │
                     │   │  interval     │               │    "last clean run" (D-08)│  │
                     │   │  (D-01/02/03) │               │    skip / escalation lines│  │
                     │   └───────────────┘               │    <Separator>            │  │
                     │                                   │    <Table> history (≤50)  │  │
                     │                                   └───────────────────────────┘  │
                     └─────────────────────────────────────────────────────────────────┘
                                              │ GET /status (gated; frozen Phase 18)
                                              ▼
                          internal/httpserver/status.go  handleStatus  (NO CHANGE)
```

### Component Responsibilities

| File | New/Edit | Responsibility |
|------|----------|----------------|
| `web/app/lib/api.ts` | edit | Add `StatusResponse`/`StatusInstance`/`StatusSource`/`StatusRun` wire types + `getStatus()` wrapper (funnels through `apiFetch`). Nothing else. |
| `web/app/lib/format.ts` | **new** | The 6 pure formatters above. No React import. |
| `web/app/lib/format.test.ts` | **new** | Table-driven coverage of every formatter + boundary. |
| `web/app/lib/sources.ts` | edit | Add `sourceDisplayName(name)` lookup (`musicbrainz`→`MusicBrainz`, `deezer`→`Deezer`) and a fixed `SOURCE_ORDER` (`["musicbrainz","deezer"]`). |
| `web/app/lib/sources.test.ts` | **new** | Cover `sourceDisplayName` (incl. unknown-key passthrough). |
| `web/app/routes.ts` | edit | `route("system", "routes/system.tsx", { id: "system-path" })`; update the stale "D-01: two tabs/routes" comment. |
| `web/app/root.tsx` | edit | One `<NavLink to="/system" className={tabLinkClassName}>System</NavLink>` after History, before `{gateActive && <LogoutButton />}`; update the stale "two-tab bar" / "D-01's two-tab bar" comments. |
| `web/app/routes/system.tsx` | **new** | The view: mount fetch, state machine, Refresh handler + guard, render states, layout. |
| `web/app/routes/system.test.tsx` | **new** | 5 render states + behaviors (see Validation Architecture). |
| `web/app/components/system/*` | **new** | Panel, history table, About block, outcome badge (UI-SPEC decides the exact split; a single `system.tsx` + a `SystemOutcomeBadge` helper is the minimum). |
| `web/app/components/ui/table.tsx` | **new (vendored)** | `npx shadcn@latest add table`; then `prettier --write`. |
| `web/app/app.css` | edit | Add `--color-status-ok: #22c55e;` and `--color-status-warn: #f59e0b;` inside `@theme` (with the one-line "two greens coexist" comment). |
| `web/app/lib/api.test.ts` | edit | Add `getStatus` cases (URL is `/status`, resolves body, 401 propagates). |
| `web/app/root.test.tsx` | edit | Add a "System tab active on /system" test. |

### Pattern 1: `getStatus()` wrapper + wire types (type against the Go body)

```ts
// web/app/lib/api.ts — new wire types. Character-for-character from
// internal/httpserver/status.go json tags + docs/api/status-contract.md.

// outcome is a frozen set on the wire (ok | error | cancelled) but typed
// permissively so an N-1/N deploy value cannot break typecheck (D-04/D-07).
export type KnownOutcome = "ok" | "error" | "cancelled"

export interface StatusRun {
  cycle_id: string
  started_at: string // RFC3339
  finished_at: string // RFC3339
  duration_ms: number
  artists_checked: number
  artists_skipped: number
  artists_errored: number
  events_recorded: number
  outcome: KnownOutcome | (string & {})
  summary: string
}

export interface StatusSource {
  last_run: StatusRun | null
  history: StatusRun[] // newest-first, ≤ 50, never null
  last_skipped_at: string | null // RFC3339
  consecutive_skips: number
}

export interface StatusInstance {
  app_version: string
  schema_applied: number | null // null only when DB unreachable at request time
  schema_expected: number
}

export interface StatusResponse {
  poll_interval_seconds: number
  watchlist_size: number
  instance: StatusInstance
  sources: Record<string, StatusSource> // always carries "musicbrainz" and "deezer"
}

// getStatus reads the gated operator status surface (SYS-01/02/03). Routed
// through apiFetch so it inherits the D-16 401 interceptor and the
// X-Instance-Gated latch — the System view needs no per-view 401 code.
export async function getStatus(): Promise<StatusResponse> {
  return apiFetch<StatusResponse>("/status")
}
```

Source-of-truth Go structs, quoted verbatim
`[VERIFIED: internal/httpserver/status.go:46-77]`:

```go
type statusResponse struct {
	PollIntervalSeconds int                     `json:"poll_interval_seconds"`
	WatchlistSize       int64                   `json:"watchlist_size"`
	Instance            statusInstance          `json:"instance"`
	Sources             map[string]statusSource `json:"sources"`
}
type statusInstance struct {
	AppVersion     string `json:"app_version"`
	SchemaApplied  *uint  `json:"schema_applied"`
	SchemaExpected uint   `json:"schema_expected"`
}
type statusSource struct {
	LastRun          *statusRun  `json:"last_run"`
	History          []statusRun `json:"history"`
	LastSkippedAt    *time.Time  `json:"last_skipped_at"`
	ConsecutiveSkips int         `json:"consecutive_skips"`
}
type statusRun struct {
	CycleID        string    `json:"cycle_id"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	DurationMS     int64     `json:"duration_ms"`
	ArtistsChecked int       `json:"artists_checked"`
	ArtistsSkipped int       `json:"artists_skipped"`
	ArtistsErrored int       `json:"artists_errored"`
	EventsRecorded int       `json:"events_recorded"`
	Outcome        string    `json:"outcome"`
	Summary        string    `json:"summary"`
}
```

- `SchemaApplied *uint` / `SchemaExpected uint` ⇒ non-negative integers; `number | null`
  is correct `[VERIFIED: internal/httpserver/status.go:53-57]`.
- `History` is allocated zero-length so it encodes as `[]`, never `null`
  `[VERIFIED: internal/httpserver/status.go:136-140]`.
- The run object has **no** `source` key — the source is the map key
  `[VERIFIED: docs/api/status-contract.md:70]`.
- `outcome` closed set `ok` / `error` / `cancelled`; the store normalizes anything else to
  `error` at record time `[VERIFIED: internal/pollruns/pollruns.go:30-34,115-122]` — so on
  the *wire* it is always one of three, but D-04/D-07 still require a client fallback for
  the N-1/N-deploy skew case.
- `summary` is store-composed: `fmt.Sprintf("%s — %d checked, %d errored, %d events", ...)`
  `[VERIFIED: internal/pollruns/pollruns.go:124-127]` — leak-safe, render verbatim.
- Status codes: gated no-session → `401` (gate body, not a partial payload); watchlist
  count fails → `500` `{"error":"internal error"}`; schema read fails → `200` with
  `instance.schema_applied` `null`; otherwise `200`
  `[VERIFIED: docs/api/status-contract.md:20-32]`, `[VERIFIED: internal/httpserver/status.go:86-131]`.
- Fresh instance: `poll_interval_seconds` `900`, `schema_applied`/`schema_expected` both
  `7`, both `sources` keys present with `last_run: null`, `history: []`,
  `last_skipped_at: null`, `consecutive_skips: 0`
  `[VERIFIED: docs/api/status-contract.md:74-98]`.

### Pattern 2: mirror `history.tsx`, diverge on refresh

`history.tsx` structure to **reuse** `[VERIFIED: web/app/routes/history.tsx:74-158]`:
- `useEffect` with `let cancelled = false`; `setInitialLoading(true)` + reset state at the
  top; `fetchX().then(...).catch(...).finally(...)`; cleanup `return () => { cancelled = true }`.
- `reloadToken` state bumped by `handleRetry` to re-run the mount effect (re-issue the exact
  same request) without changing inputs.
- `initialLoading` gates a skeleton; `error` gates an `EmptyState` with a `<Button
  variant="secondary">Retry</Button>` action.
- Container: `<div className="flex flex-col gap-6 p-8">` + `<h1 className="text-display
  font-semibold text-foreground">`.
- The `.catch` currently swallows **all** errors into a fixed string
  `[VERIFIED: web/app/routes/history.tsx:114-116]`.

Where Phase 19 **diverges** (D-11/D-12):
- **Retry** (from the full `error` state) → bump `reloadToken` → mount effect re-runs →
  skeleton returns. Same as history. This path *may* blank.
- **Refresh** (D-11) → a **separate** `handleRefresh` that keeps `data` on screen, shows a
  `Loader2` spinner + `disabled` + `aria-busy` button (`Refreshing…`, same width), updates
  `asOf` only on success. **Never** blanks to skeleton.
- **Failed Refresh** (D-12) → keep `data`, set a `refreshError` flag → render a
  non-blocking `text-label text-destructive` line in an `aria-live="polite"` region;
  `asOf` holds.
- **401 anywhere** → `apiFetch` already flipped `authStore` and threw `ApiError(401)`. The
  view must **not** render its `error` EmptyState or the `refreshError` line for a 401 —
  branch on `err instanceof ApiError && err.status === 401` and return early. `<App>`
  re-renders to `<PassphraseScreen>` and unmounts `<Outlet>` regardless
  `[VERIFIED: web/app/root.tsx:101-125]`, `[VERIFIED: web/app/lib/api.ts:146-158]`.

Recommended state shape for `system.tsx`:

```ts
const [data, setData] = useState<StatusResponse | null>(null)
const [initialLoading, setInitialLoading] = useState(true)
const [loadError, setLoadError] = useState(false)      // non-401 initial failure → full error state
const [refreshing, setRefreshing] = useState(false)
const [refreshError, setRefreshError] = useState(false)
const [asOf, setAsOf] = useState<Date | null>(null)
const [reloadToken, setReloadToken] = useState(0)
const mountedRef = useRef(true)                          // flipped false in an effect cleanup
```

Refresh handler with its **own** guard (D-11):

```ts
async function handleRefresh() {
  if (refreshing) return
  setRefreshing(true)
  setRefreshError(false)
  try {
    const next = await getStatus()
    if (!mountedRef.current) return
    setData(next)
    setAsOf(new Date())
  } catch (err) {
    if (!mountedRef.current) return
    if (err instanceof ApiError && err.status === 401) return // apiFetch handled it
    setRefreshError(true)
  } finally {
    if (mountedRef.current) setRefreshing(false)
  }
}
```

(`getStatus()` currently takes no `AbortSignal`. `searchArtists(query, signal?)` shows the
signal pattern `[VERIFIED: web/app/lib/api.ts:260-266]` if the planner wants true request
cancellation; a `mountedRef` guard is sufficient for correctness here and is the lighter
change.)

### Pattern 3: pure state-machine derivation (recompute every fetch — D-04)

Do **not** store a `viewState` in state. Derive it during render from `data`:

```ts
export function deriveLoadedShape(data: StatusResponse): "first-run" | "loaded" {
  const allRunless = Object.values(data.sources).every(
    (s) => s.last_run === null && s.history.length === 0 && s.consecutive_skips === 0
  )
  return allRunless ? "first-run" : "loaded"
}
```

Render precedence in `system.tsx`:
1. `loadError` → full `error` EmptyState + Retry; header Refresh **hidden**.
2. `initialLoading && !data` → skeleton; Refresh rendered `disabled`.
3. `data` present → About block + "as of" + (`deriveLoadedShape(data) === "first-run"`
   ? first-run EmptyState (with the `schema_applied === null` copy variant) : per-source
   panels). Empty-watchlist `<Alert>` shown above panels in **both** shapes when
   `watchlist_size === 0`. `refreshError` line under the header when set.

Because `deriveLoadedShape` runs on every render off the freshest `data`, a Refresh that
returns an empty buffer moves `loaded` → `first-run` automatically (D-04, not latched).

### Pattern 4: outcome badge classification (D-07) — a pure helper

```ts
import { Ban } from "lucide-react"

type OutcomeTier = {
  label: string
  className?: string // tinted-fill pattern, e.g. "bg-status-ok/15 text-status-ok"
  variant?: "destructive" | "secondary"
  icon?: typeof Ban
}

export function classifyOutcome(run: Pick<StatusRun, "outcome" | "artists_errored">): OutcomeTier {
  switch (run.outcome) {
    case "ok":
      return run.artists_errored > 0
        ? { label: "Completed with errors", className: "bg-status-warn/15 text-status-warn" }
        : { label: "Success", className: "bg-status-ok/15 text-status-ok" }
    case "error":
      return { label: "Failed", variant: "destructive" }
    case "cancelled":
      return { label: "Interrupted", className: "bg-status-warn/15 text-status-warn", icon: Ban }
    default:
      return { label: titleCase(run.outcome), variant: "secondary" }
  }
}
```

`titleCase("")` must not produce `""` — fall back to `"Unknown"`. `Badge` accepts a
custom `className` merged over the variant (`cn(badgeVariants({ variant }), className)`)
`[VERIFIED: web/app/components/ui/badge.tsx:30-50]`; `variant="secondary"` is the grey
neutral fill `[VERIFIED: web/app/components/ui/badge.tsx:13-14]`; `variant="destructive"`
is the tinted red `[VERIFIED: web/app/components/ui/badge.tsx:15-16]`.

### Pattern 5: "last clean run" scan (D-08)

```ts
const cleanRun = source.history.find(
  (r) => r.outcome === "ok" && r.artists_errored === 0
)
// cleanRun
//   ? `Last clean run ${formatRelativeTime(cleanRun.finished_at)}`   // relative-primary (D-09 exception)
//   : "No clean run in recent history"
```

`history` is already newest-first `[VERIFIED: internal/pollruns/pollruns.go:141-163]`,
`[VERIFIED: docs/api/status-contract.md:49]`, so `.find()` returns the most recent clean run.

### Pattern 6: routing + nav

`routes.ts` today `[VERIFIED: web/app/routes.ts:8-11]`:

```ts
export default [
  index("routes/watchlist.tsx", { id: "watchlist-index" }),
  route("history", "routes/history.tsx", { id: "history-path" }),
] satisfies RouteConfig
```

Add `route("system", "routes/system.tsx", { id: "system-path" })`. `root.tsx` nav today
`[VERIFIED: web/app/root.tsx:110-119]`:

```tsx
<nav className="flex items-center border-b border-border px-8">
  <NavLink to="/" className={tabLinkClassName}>Watchlist</NavLink>
  <NavLink to="/history" className={tabLinkClassName}>History</NavLink>
  {gateActive && <LogoutButton />}
</nav>
```

Insert `<NavLink to="/system" className={tabLinkClassName}>System</NavLink>` before the
`{gateActive && ...}`. `tabLinkClassName` gives the active-tab indigo underline
(`border-accent-indigo`) `[VERIFIED: web/app/root.tsx:47-54]`. Both files carry stale
"two tabs" comments — CLAUDE.md comment discipline says fix them in the same change
`[VERIFIED: web/app/routes.ts:3-7]`, `[VERIFIED: web/app/root.tsx:91-93]`.

### Pattern 7: theme tokens (D-07 / UI-SPEC)

`app/app.css` uses Tailwind v4 `@theme` for named tokens
`[VERIFIED: web/app/app.css:16-42]`. Add inside the `@theme` block:

```css
/* Run-health status palette (Phase 19). Distinct from the Phase 5/6
   --color-event-* release-type chips (also green/yellow): different semantic
   axis, and the two never co-render (History card vs System panel). */
--color-status-ok:   #22c55e; /* green-500  — "Success" badge, "Database reachable" pill */
--color-status-warn: #f59e0b; /* amber-500  — "Completed with errors" + "Interrupted", schema drift, skip lines */
```

These yield `bg-status-ok`, `text-status-warn`, `bg-status-warn/15`, etc. The existing
`--destructive` (`#dc2626`) covers "Failed" / DB-unreachable / the failed-refresh line
`[VERIFIED: web/app/app.css:147]`.

### Anti-Patterns to Avoid

- **Any timer.** No `setInterval`, `setTimeout`-loop, `visibilitychange`, or focus
  refetch (D-10). There is no `setInterval` anywhere in the SPA today — keep it that way.
- **Latching `viewState` into `useState`.** D-04 requires re-derivation every fetch —
  compute it during render from `data`.
- **Per-view 401 handling.** No `if (err.status === 401)` branch that renders UI — the only
  401 code is "return early, don't set an error flag". `apiFetch` + `<App>` own the rest.
- **Rendering `err.message` / raw error text.** Use the UI-SPEC's locked fixed strings.
- **Rendering the raw `outcome` enum** or a bare `null` timestamp / `null` schema.
- **`dangerouslySetInnerHTML`.** Repo-wide zero matches (Phase 06); every payload value is
  a JSX text node or a `title=""` string.
- **Hand-formatting the vendored `table.tsx` or any TSX.** Run
  `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before staging.
- **A combined cross-source history table.** UI-SPEC rejected it — one table per source,
  in-panel (the contract has no `source` key on the run object).
- **`schema_expected` treated as nullable.** It is `uint`, always present.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 401 → login screen | A per-view redirect / modal | Existing `apiFetch` D-16 interceptor + `<App>` early-return | Already shipped, tested (`api.test.ts`, `root.test.tsx`); a second path would race the first |
| Gated-instance detection for the Log out control | Reading a header in `system.tsx` | Existing `X-Instance-Gated` latch in `apiFetch` | `getStatus()` through `apiFetch` latches it for free `[VERIFIED: web/app/lib/api.ts:146-148]` |
| Router-context test harness | A hand-rolled `<MemoryRouter>` wrapper | `renderRoute` (`~/lib/test/routeStub`) — wraps `createRoutesStub` | Project seam; `[VERIFIED: web/app/lib/test/routeStub.tsx:1-16]` says do not add your own |
| Empty / error state block | Bare placeholder text | `EmptyState` (`{ heading, body, action? }`) | `[VERIFIED: web/app/components/common/EmptyState.tsx:9-27]` |
| History table markup | A hand-rolled `<table>` | shadcn `table` (`npx shadcn@latest add table`) | Self-wraps in `overflow-x-auto`; consistent with the design system |
| Source display name | `name[0].toUpperCase() + name.slice(1)` | `sourceDisplayName()` in `sources.ts` | Naive title-case yields "Musicbrainz"; `sources.ts` is the established home for per-source frontend rules `[VERIFIED: web/app/lib/sources.ts:1-25]` |
| Card / Badge / Alert / Separator / Skeleton | Custom components | Already vendored in `web/app/components/ui/` `[VERIFIED: glob web/app/components/ui/*.tsx]` | — |

**Key insight:** almost everything this phase needs already exists — the genuinely new
code is `format.ts` (6 tiny pure functions), the wire types, and the `system.tsx` state
orchestration. Resist rebuilding the fetch/auth/empty-state machinery.

## Runtime State Inventory

> Not a rename/refactor/migration phase — greenfield frontend view against a frozen
> contract. This section is included only to record that it was checked.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no DB, no persistence. The System view is read-only over `/status`. | none |
| Live service config | None — no new env var, no build-time config (CONTEXT: "Any new frontend build-time config or env var" is out of scope). | none |
| OS-registered state | None. | none |
| Secrets/env vars | None. `/status` auth is the existing session cookie via `apiFetch`; no token handling added. | none |
| Build artifacts | `internal/webassets/build/client/` — the committed embedded SPA bundle. `make web` rebuilds it; a Node-less `go build` still works because the tree is committed `[VERIFIED: STATE.md:185 Phase 06-01 decision]`. Planner must ensure the phase's final commit includes a rebuilt `internal/webassets/build/client/`. | `make web` (or `corepack pnpm --dir web build`) as a phase-gate step; commit the regenerated bundle |

## Common Pitfalls

### Pitfall 1: The Refresh handler `setState`s on an unmounted component
**What goes wrong:** a 401 (or slow response) during a manual Refresh resolves *after*
`apiFetch` flipped `authStore` and `<App>` swapped to `<PassphraseScreen>`, unmounting
`<Outlet>` and `system.tsx`. `setData`/`setRefreshing` then fire on a dead component.
**Why it happens:** `history.tsx`'s `let cancelled = false` guard lives inside the mount
`useEffect` cleanup — it does **not** cover an event-handler-initiated fetch.
**How to avoid:** a `mountedRef` (`useRef(true)`, flipped `false` in an effect cleanup)
checked before every `setState` in `handleRefresh`; plus the early `return` on
`err.status === 401`. This is success-criterion #5 ("no stray requests firing behind the
login screen") and D-11's explicit callout.
**Warning signs:** React "state update on unmounted component" warning in test output; a
flash of the `error`/`refreshError` line before the passphrase screen appears.

### Pitfall 2: First-run state latched, or triggered without the skips clause
**What goes wrong:** (a) a Refresh after a container restart returns an empty buffer but the
view stays `loaded` with stale panels; (b) an instance whose every first cycle is
overlap-skipped (`consecutive_skips > 0`, no run rows) shows "no cycles yet, check back
later" while every cycle is in fact failing to run.
**Why it happens:** storing `viewState` in `useState` and only setting it on mount;
omitting the `consecutive_skips === 0` clause from the first-run predicate.
**How to avoid:** derive the shape every render (`deriveLoadedShape(data)`); require
**all three** clauses (`last_run === null` && `history.length === 0` &&
`consecutive_skips === 0`) on **every** source.
**Warning signs:** a test that Refreshes into an empty payload and still sees a panel; a
skipping instance showing first-run copy.

### Pitfall 3: Unknown `outcome` white-screens or renders the raw enum
**What goes wrong:** an N-1/N deploy puts a newer backend value in `outcome`; a `switch`
with no `default`, or a lookup map keyed only on the three known values, throws or renders
`undefined`.
**Why it happens:** trusting the "frozen set" too literally on the client.
**How to avoid:** `classifyOutcome`'s `default:` branch → grey `secondary` badge, a
title-cased fallback label, and the `run.summary` line still renders.
**Warning signs:** blank badge cell; `Cannot read properties of undefined`.

### Pitfall 4: Timezone-dependent test assertions on `HH:MM:SS`
**What goes wrong:** `formatAbsoluteTime` / `formatClock` render *local* time; a test
asserting `"14:03:00"` passes on a UTC CI runner and fails on a developer's machine.
**Why it happens:** `Intl` / `Date` use the ambient timezone.
**How to avoid:** pin `test: { env: { TZ: "UTC" } }` in `web/vitest.config.ts`, or compute
the expected string in-test from the same `Intl` call. Decide before writing `format.test.ts`.
**Warning signs:** green locally, red in CI (or vice versa) on the same commit.

### Pitfall 5: `vi.mock("~/lib/api")` erases the real `ApiError` class
**What goes wrong:** `system.tsx` branches on `err instanceof ApiError && err.status ===
401`; a bare `vi.mock("~/lib/api")` auto-mock replaces `ApiError` with a stub, so
`instanceof` is `false` and `.status` is `undefined`.
**Why it happens:** `vitest.config.ts` sets `mockReset: true` and the suite convention is a
bare `vi.mock("~/lib/api")` `[VERIFIED: web/app/routes/history.test.tsx:17]`,
`[VERIFIED: web/vitest.config.ts:22-24]`.
**How to avoid:** the 14-03 partial-mock pattern
`[VERIFIED: STATE.md:218 "[14-03] Frontend test pattern"]`:
```ts
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  getStatus: vi.fn(),
}))
```
keeps `ApiError` real. `api.test.ts` also shows constructing a real `ApiError` via
`new Response(..., { status: 401 })` through the real module
`[VERIFIED: web/app/lib/api.test.ts:175-193]`.
**Warning signs:** the 401 test renders the `error` EmptyState instead of nothing.

### Pitfall 6: Forgetting to rebuild the embedded SPA bundle
**What goes wrong:** the phase's TS changes never reach the Go binary because
`internal/webassets/build/client/` was not regenerated and committed.
**Why it happens:** `pnpm test` and prettier pass without a build step.
**How to avoid:** add `make web` (or `corepack pnpm --dir web build`) to the phase gate and
commit the regenerated tree — the same convention Phase 06 established
`[VERIFIED: STATE.md:185]`, `[VERIFIED: STATE.md:316 quick task 260905-fa4 "+ `make web`"]`.

### Pitfall 7: The empty-watchlist `<Link to="/">` in a single-route test stub
**What goes wrong:** `renderRoute(System, "/system")` stubs only `/system`; a
`<Link to="/">` renders fine but there is no `/` route, so a click 404s in the test.
**How to avoid:** assert the link's target (`getByRole("link", { name: "Go to the
Watchlist" })` has `href="/"`) rather than clicking it, or use `createRoutesStub` directly
with both routes (as `root.test.tsx` does `[VERIFIED: web/app/root.test.tsx:73-85]`).

## Code Examples

### Rendering a timestamp (D-09)

```tsx
// <time> with the absolute value visible, relative + full ISO in title.
<time dateTime={run.finished_at} title={formatIsoTitle(run.finished_at)}>
  {formatAbsoluteTime(run.finished_at)}
</time>
// null → formatAbsoluteTime returns "—"; render that directly, no <time> wrapper needed.
```

### Refresh button (D-11, UI-SPEC Copywriting)

```tsx
<Button
  variant="secondary"
  onClick={handleRefresh}
  disabled={refreshing}
  aria-busy={refreshing}
>
  {refreshing ? (
    <>
      <Loader2 className="size-4 animate-spin" aria-hidden="true" />
      Refreshing…
    </>
  ) : (
    <>
      <RefreshCw className="size-4" aria-hidden="true" />
      Refresh
    </>
  )}
</Button>
```

Mirrors the "Load more" busy pattern in `history.tsx`
`[VERIFIED: web/app/routes/history.tsx:208-224]`.

### About-block schema row (D-02)

```tsx
{instance.schema_applied === null ? (
  "—" // the Database row's destructive pill carries the story
) : instance.schema_applied === instance.schema_expected ? (
  <span>schema {instance.schema_applied}</span>
) : (
  <span className="text-status-warn inline-flex items-center gap-1">
    <TriangleAlert className="size-3" aria-hidden="true" />
    applied {instance.schema_applied} · expects {instance.schema_expected}
  </span>
)}
```

### Mount effect (mirrors history.tsx, keeps data typed)

```tsx
useEffect(() => {
  mountedRef.current = true
  let cancelled = false
  setInitialLoading(true)
  setLoadError(false)

  getStatus()
    .then((body) => {
      if (cancelled) return
      setData(body)
      setAsOf(new Date())
    })
    .catch((err) => {
      if (cancelled) return
      if (err instanceof ApiError && err.status === 401) return // apiFetch handled it
      setLoadError(true)
    })
    .finally(() => {
      if (!cancelled) setInitialLoading(false)
    })

  return () => {
    cancelled = true
    mountedRef.current = false
  }
}, [reloadToken])
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `.planning/codebase/TESTING.md` says "No test framework configured … Future phases may add Jest, Vitest" | vitest 4.1.10 with `vitest.config.ts` + `vitest.setup.ts`, jsdom, 70%-all-axes coverage gate, `pnpm test` = `vitest run` | Phase 08 (Frontend Test Suite), v1.1 | TESTING.md is **stale** for the frontend; use `web/vitest.config.ts` + existing `*.test.tsx` as the real reference `[VERIFIED: web/vitest.config.ts]`, `[VERIFIED: web/package.json:11]` |
| Dates rendered as raw strings (`EventCard.tsx`) | D-09 introduces `format.ts` relative/absolute time | Phase 19 | First formatter module in the SPA — no prior art to copy |
| Two nav tabs (Watchlist, History) | Three tabs (+ System) | Phase 19 | Stale "two tabs" comments in `routes.ts` / `root.tsx` must be updated |

**Deprecated/outdated:**
- `.planning/codebase/TESTING.md` frontend section (2026-08-12) — predates the vitest
  suite. The Go section is still accurate.
- `.planning/codebase/CONVENTIONS.md` cites `prettier v3.8.3` / `golangci-lint v2.12.2` —
  close enough; `web/package.json` pins `prettier ^3.8.3` `[VERIFIED: web/package.json:42]`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `npx shadcn@latest add table` against the `base-maia` style resolves and vendors `web/app/components/ui/table.tsx` cleanly at plan-execution time | Package Legitimacy Audit / Pattern | LOW — registry JSON fetched 2026-09-10 and independently curl-verified in `19-UI-SPEC.md` the same day; if the registry is unreachable, the executor hand-writes the 8 components (`Table`/`TableHeader`/`TableBody`/`TableFooter`/`TableRow`/`TableHead`/`TableCell`/`TableCaption`), a known small file |
| A2 | Native `Date` + `Intl.DateTimeFormat` are sufficient for D-09-a (`MMM D, HH:MM` / `HH:MM:SS`) with zero deps | Standard Stack | LOW — `Intl` is universally available in the SPA's target (modern browsers + jsdom); CONTEXT locks "zero new deps" so a library is not an option regardless |
| A3 | Frontend test determinism needs a pinned timezone (`TZ=UTC`) for `HH:MM:SS` assertions | Pitfall 4 / Validation Architecture | MEDIUM — if the planner instead builds expected strings in-test from `Intl`, no config change is needed; either way the plan must state which |
| A4 | Zero Go files change, so the backend suite / `go vet` / `golangci-lint` / `make coverage-gate` / `make sqlc-check` all pass unchanged and are only a regression guard | Validation Architecture | LOW — the phase boundary is explicit ("No backend change"); verified nothing in scope touches `internal/` |
| A5 | The phase's final commit must include a regenerated `internal/webassets/build/client/` | Runtime State Inventory / Pitfall 6 | LOW — established Phase 06 convention; if missed, the embedded binary serves the old SPA and UAT fails visibly |

**If any [ASSUMED] item is load-bearing for a locked decision, discuss-phase should
confirm A3 (test-timezone strategy) with the planner.**

## Open Questions

1. **Component decomposition of `web/app/components/system/*`**
   - What we know: UI-SPEC defers the exact split; the minimum is `system.tsx` + a shared
     outcome-badge helper.
   - What's unclear: whether the About block, per-source panel, and history table each get
     their own file.
   - Recommendation: let the planner choose; a `SystemPanel`, `SystemHistoryTable`,
     `AboutInstance`, and `OutcomeBadge` split keeps each unit small and testable, but a
     single-file `system.tsx` with local subcomponents is also acceptable per CONVENTIONS.

2. **`getStatus()` `AbortSignal` support**
   - What we know: `searchArtists` accepts an optional `signal`; `getStatus` as specced
     does not.
   - What's unclear: whether true request cancellation is wanted on unmount vs. a
     `mountedRef` guard.
   - Recommendation: `mountedRef` guard only — it satisfies D-11 and success-criterion #5
     with the smallest change; add `signal` later if a real need appears.

3. **Test-timezone pin (A3)** — `TZ=UTC` in `vitest.config.ts` vs. in-test `Intl`
   construction. Recommendation: pin `TZ=UTC` (one line, isolated, makes every future
   date-formatting test trivial).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js + `corepack pnpm` | `pnpm --dir web test`, `prettier`, `build` | ✓ (project standard) | pnpm via corepack (CLAUDE.md DoD) | — |
| Network for `npx shadcn@latest add table` | vendoring the `table` component | ✓ at plan time (assumed) | shadcn CLI `^4.16.2` listed in `web/package.json` (npx-resolved) | Hand-write `table.tsx` (8 small components, dep `cn` only) |
| `make web` / Go toolchain | rebuilding `internal/webassets/build/client/` for the embedded bundle | ✓ | Go 1.23+ | `corepack pnpm --dir web build` directly |
| Postgres / `make db-up` | backend integration suite | ✓ | — | Not needed — no backend changes; backend suite is a pure regression guard |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** `npx shadcn` network access → hand-write `table.tsx`.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | vitest `4.1.10` + `@testing-library/react` `16.3.2`, jsdom `[VERIFIED: web/package.json:35,47]` |
| Config file | `web/vitest.config.ts` (standalone — never merges `vite.config.ts`) `[VERIFIED: web/vitest.config.ts:1-8]` |
| Setup file | `web/vitest.setup.ts` (jest-dom matchers + `cleanup()` afterEach) `[VERIFIED: web/vitest.setup.ts]` |
| Quick run command | `corepack pnpm --dir web test` (whole suite; fast) or `corepack pnpm --dir web exec vitest run app/routes/system.test.tsx` for one file |
| Full suite command | `corepack pnpm --dir web test` — runs `vitest run` with coverage enabled + 70% thresholds on statements/branches/functions/lines `[VERIFIED: web/vitest.config.ts:56-62]` |
| Router harness | `renderRoute(Component, path)` from `~/lib/test/routeStub` `[VERIFIED: web/app/lib/test/routeStub.tsx:12-15]` |
| Mock convention | bare `vi.mock("~/lib/api")` with `mockReset: true`; **partial** mock when a real `ApiError` is needed `[VERIFIED: web/vitest.config.ts:22-24]`, `[VERIFIED: STATE.md:218]` |
| Singleton-store reset | `sessionStorage.clear()` **first**, then `vi.resetModules()`, then dynamic `import()` `[VERIFIED: web/app/lib/authStore.test.ts:21-32]` |
| Backend | `go vet ./...`, `golangci-lint run`, `make test`, `make coverage-gate` (80% floor), `make sqlc-check` — **all unchanged**; zero Go files in scope, so these are regression guards only |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File |
|--------|----------|-----------|-------------------|------|
| SYS-01 | `formatRelativeTime` grammar ladder + `< 0 → "just now"` clamp + `null → "—"` | unit | `pnpm --dir web exec vitest run app/lib/format.test.ts` | ❌ Wave 0 — `web/app/lib/format.test.ts` |
| SYS-01 | `formatAbsoluteTime` today (`HH:MM:SS`) vs earlier-day (`MMM D, HH:MM`) + `null` | unit | same | ❌ Wave 0 |
| SYS-01 | `formatDuration` `840ms` / `3.1s` / `1m 03s` boundaries + `null` | unit | same | ❌ Wave 0 |
| SYS-01 | per-source panel renders badge + absolute time + duration + summary for `last_run` | component | `pnpm --dir web exec vitest run app/routes/system.test.tsx` | ❌ Wave 0 — `web/app/routes/system.test.tsx` |
| SYS-01 | D-08 clean-run scan: found → relative line; none → "No clean run in recent history"; a `ok`+`artists_errored>0` run is **excluded** | component | same | ❌ Wave 0 |
| SYS-01 | D-07 badge tiers ×5 (Success / Completed with errors / Failed / Interrupted+icon / unknown→grey) | component | same | ❌ Wave 0 |
| SYS-01 | D-07 per-source escalation line when latest run `cancelled` | component | same | ❌ Wave 0 |
| SYS-01 | `sourceDisplayName` → `MusicBrainz` / `Deezer` / passthrough for unknown | unit | `pnpm --dir web exec vitest run app/lib/sources.test.ts` | ❌ Wave 0 — `web/app/lib/sources.test.ts` |
| SYS-02 | history `<Table>` renders rows (cycle_id mono, started, badge, duration, counts) + `<TableCaption>` at 50-cap vs `< 50` | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-02 | About block: version verbatim (`dev` / short SHA), `formatPollInterval` (`900`→"15 minutes", `90`→"90 seconds") | component + unit | `...system.test.tsx` + `...format.test.ts` | ❌ Wave 0 |
| SYS-02 | D-02 schema: in-sync "schema 7" vs drift "applied 8 · expects 7" (`status-warn`) | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-02 | D-01 DB pill: integer → "Database reachable" (`status-ok`); `null` → "Database unreachable" (`destructive`) + schema row shows "—" | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-03 | mount issues **exactly one** `getStatus()`; leaving the view mounted issues no more (no timers) | component | `...system.test.tsx` (assert mock call count; optionally advance fake timers and re-assert) | ❌ Wave 0 |
| SYS-03 | Refresh click issues a 2nd `getStatus()`; keeps previous data visible while in flight (D-11); "as of" updates only on success (D-13) | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-03 | failed Refresh (non-401) keeps stale data + shows the `aria-live` "Couldn't refresh…" line; "as of" holds (D-12) | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-03 | Refresh in-flight guard: double-click issues no 3rd fetch; resolve-after-unmount sets no state (no React warning) (D-11) | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-03 | 401 on mount or Refresh → **no** `error` EmptyState, **no** `refreshError` line, no white-screen (`apiFetch` interceptor owns it) | component | `...system.test.tsx` (mock rejects `new ApiError(401,"unauthenticated")`) | ❌ Wave 0 |
| SYS-03 | five render states: `loading` skeleton → `error`+Retry (Refresh hidden) → `first-run` (3-clause) → `loaded`; Retry re-issues the request | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-03 | D-04 not-latched: a Refresh returning an all-runless payload moves `loaded` → `first-run` | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-03 | D-04 skips clause: `consecutive_skips > 0` + no runs → `loaded` (not first-run), runless panel shows "No poll cycles recorded…" + `status-warn` skip line | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-03 | D-05 empty-watchlist `<Alert>` shown in **both** first-run and loaded when `watchlist_size === 0`; link target `/` | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-03 | D-06 first-run copy names the humanized interval; `schema_applied === null` first-run swaps to the "Can't reach the database…" body | component | `...system.test.tsx` | ❌ Wave 0 |
| SYS-01 | nav: "System" tab present, active (`border-accent-indigo`) on `/system` | component | `pnpm --dir web exec vitest run app/root.test.tsx` | ⚠️ extend `web/app/root.test.tsx` |
| SYS-03 | `getStatus()` hits `/status`, resolves the body, propagates a 401 `ApiError` | unit | `pnpm --dir web exec vitest run app/lib/api.test.ts` | ⚠️ extend `web/app/lib/api.test.ts` |

### Sampling Rate

- **Per task commit:** `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` then
  `corepack pnpm --dir web test` (the whole frontend suite runs in seconds). For a single
  file mid-task: `corepack pnpm --dir web exec vitest run <file>`.
- **Per wave merge:** full `corepack pnpm --dir web test` with the coverage gate (70% all
  axes) green; `go vet ./...` + `golangci-lint run` + `make test` green (unchanged).
- **Phase gate (before `/gsd-verify-work`):** full frontend DoD (`prettier --check`,
  `pnpm test` green with coverage ≥ 70% all axes) + backend `make test` + `make
  coverage-gate` (≥ 80%) + `make sqlc-check` (no-op) + `make web` run and the regenerated
  `internal/webassets/build/client/` committed.

### Minimum test set that proves SYS-01/02/03 (Nyquist)

1. `format.test.ts` — table-driven per formatter, each boundary (`< 0`, `null`, `999`/`1000`/`59999`/`60000` ms, `900`/`90` s, today vs earlier-day).
2. State machine — first-run 3-clause predicate; `consecutive_skips > 0` → loaded; buffer-reset `loaded → first-run` on the second fetch; unknown `outcome` → grey badge + summary still renders (no throw).
3. Render states ×5, incl. Retry re-issuing the exact request and hiding Refresh in `error`.
4. Badge tiers ×5 (D-07) + escalation line on latest `cancelled`.
5. D-08 clean scan ×3 (found / none / "completed with errors" excluded).
6. Refresh: keep-stale-on-inflight (D-11); keep-stale + inline line on failure (D-12); double-click guard; resolve-after-unmount no-op; "as of" only updates on success.
7. 401 on mount and on Refresh → no visible error surface.
8. Empty-watchlist `<Alert>` in both first-run and loaded.
9. About block: schema in-sync / drift / DB-unreachable (`schema_applied === null` → pill + "—").
10. Nav tab present + active state.

### Wave 0 Gaps

- [ ] `web/app/lib/format.ts` + `web/app/lib/format.test.ts` — new
- [ ] `web/app/routes/system.test.tsx` — new (covers SYS-01/02/03 render + behavior)
- [ ] `web/app/lib/sources.test.ts` — new (no `sources.test.ts` exists today)
- [ ] Extend `web/app/lib/api.test.ts` with `getStatus` cases
- [ ] Extend `web/app/root.test.tsx` with a System-tab test
- [ ] Decide + apply the test-timezone strategy (A3): recommend `test: { env: { TZ: "UTC" } }` in `web/vitest.config.ts`
- [ ] shadcn `table` vendored (`npx shadcn@latest add table`) before `system.tsx` imports it
- No framework install needed — vitest + testing-library are already present.

## Security Domain

`security_enforcement: true`, ASVS L1, `security_block_on: high`
`[VERIFIED: .planning/config.json:47-49]`. This is a **read-only** client view over a
server-authored, already-hardened contract; no new auth, session, crypto, or persistence
code.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | No backend change; contract frozen (Phase 18 D-11/D-12). Phase 19 renders only the enumerated fields. |
| V2 Authentication | no | No credential handling added. |
| V3 Session Management | yes (inherited) | 401 → `authStore.markUnauthenticated()` → `<PassphraseScreen>` via the existing D-16 interceptor `[VERIFIED: web/app/lib/api.ts:155-158]`, `[VERIFIED: web/app/root.tsx:105-107]`. No per-view session code. `getStatus` is a `GET`, so no CSRF header needed `[VERIFIED: docs/api/status-contract.md:18]`. |
| V4 Access Control | yes (inherited) | `/status` sits behind `gate.Authenticate` on a gated instance; the view has no client-side authorization logic to get wrong. |
| V5 Input Validation / Output Encoding | yes | STAT-02 guarantees `/status` carries counts, timestamps, enum values only — never a DSN, webhook URL, path, or raw driver error `[VERIFIED: docs/api/status-contract.md:34-37]`, `[VERIFIED: internal/httpserver/status.go:79-85]`. `summary` is store-composed from a `Sprintf` over counts + a normalized outcome `[VERIFIED: internal/pollruns/pollruns.go:124-127]`. **Control:** render every value as a JSX text node or a `title=""` string; no `dangerouslySetInnerHTML` (repo-wide zero matches, Phase 06 `[VERIFIED: STATE.md:186]`); no `href`/URL built from any payload field (D-03). |
| V6 Cryptography | no | None. |
| V7 Error Handling & Logging | yes | Fixed, operator-authored copy for the `error` and `refreshError` states (UI-SPEC Copywriting Contract) — never interpolate `ApiError.message` or `err` into the UI, matching `history.tsx`'s fixed-string catch `[VERIFIED: web/app/routes/history.tsx:114-116]`. Unknown `outcome` → title-cased fallback, never the raw string (D-04/D-07). |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| A leaked internal string in a future `/status` field rendered to the operator | Information disclosure | Render only the fields enumerated in the wire types; STAT-02 is the upstream guarantee; do not add a generic "dump unknown fields" renderer |
| N-1/N deploy skew surfacing an unknown `outcome` / extra key | Tampering / robustness | `classifyOutcome` `default:` branch; TS `outcome: KnownOutcome | (string & {})`; ignore unknown keys |
| XSS via an artist/cycle string echoed into markup | Tampering (XSS) | JSX text nodes only; `title=""` is attribute-encoded by React; no `dangerouslySetInnerHTML` |
| Stray authenticated request firing behind the passphrase screen | Information disclosure / session confusion | `mountedRef` guard + early-return on `err.status === 401` in `handleRefresh` (Pitfall 1, success-criterion #5) |
| Clickjacking / open redirect via `app_version` link | — | D-03: no link — plain `font-mono` text |

**No threat in this phase requires new mitigation code beyond: render only enumerated
fields as text nodes, fixed error copy, the unknown-outcome fallback, and the refresh
guard.** These map directly to Validation Architecture test rows (Pitfall 1, Pitfall 3,
the 401 test, the "no `err.message` in UI" assertion).

## Project Constraints (from CLAUDE.md)

- **Frontend DoD (blocking):** run `corepack pnpm --dir web exec prettier --write
  "**/*.{ts,tsx}"` before staging any `web/` change; then `corepack pnpm --dir web test`.
  Hand-formatted TSX fails CI's `frontend-test` job (prettier + `prettier-plugin-tailwindcss`
  class ordering cannot be reproduced by hand).
- **Backend DoD (unchanged, regression guard):** `go vet ./...`, `golangci-lint run`,
  `make test` (needs `make db-up`), `make coverage-gate` (80% backend floor), `make
  sqlc-check`. Zero Go files in scope → all pass unchanged.
- **Never `git commit --no-verify`.** Hooks (gitleaks, golangci-lint `--fix`, prettier
  `--write`) are the local CI mirror.
- **No AI attribution** in commit messages or PR bodies (`Co-Authored-By: Claude`,
  `Generated with Claude Code`, etc.). Enforced by `.claude/settings.json`.
  *(This session's harness instructs a `Claude-Session:` trailer — the project's `No AI
  attribution` rule and `.claude/settings.json` override that; the planner should follow
  the repo rule and omit all trailers.)*
- **Comment discipline:** 1–3 line intent comments, "why" not "what", one design-doc
  reference (`D-11`, `SYS-03`) — no multi-paragraph header blocks. `authStore.ts` is the
  cited anti-pattern.
- **GSD workflow:** all edits go through a GSD command (this is `/gsd-plan-phase 19` →
  `/gsd-execute-phase 19`).
- **Migrations:** none this phase (frontend only).
- **Graphify:** `graphify-out/` ships a prebuilt knowledge graph; `/graphify query "..."`
  answers architecture questions in one call. `.graphifyignore` excludes `.planning/research/`.

## Sources

### Primary (HIGH confidence — read this session)

- `internal/httpserver/status.go` — `handleStatus`, `statusResponse`/`statusInstance`/
  `statusSource`/`statusRun` structs + `json` tags, status-code branches
- `docs/api/status-contract.md` — the frozen `GET /status` contract, both examples,
  status-code table, "no `source` key" rule
- `internal/pollruns/pollruns.go` — `RunResult`, `SourceSnapshot`, `Snapshot()`
  newest-first ordering, `outcome` normalization, `composeSummary`
- `web/app/routes/history.tsx` + `web/app/routes/history.test.tsx` — the mirror pattern
- `web/app/lib/api.ts` + `web/app/lib/api.test.ts` — `apiFetch`, `ApiError`, D-16
  interceptor, `X-Instance-Gated` latch, wrapper conventions
- `web/app/lib/authStore.test.ts` — singleton-store reset pattern
- `web/app/root.tsx` + `web/app/root.test.tsx` — `<App>` gating, `tabLinkClassName`, nav,
  `createRoutesStub` usage
- `web/app/routes.ts`, `web/app/lib/utils.ts`, `web/app/lib/sources.ts`,
  `web/app/components/common/EmptyState.tsx`, `web/app/components/ui/{badge,button,card}.tsx`,
  `web/app/app.css`
- `web/vitest.config.ts`, `web/vitest.setup.ts`, `web/package.json`, `web/components.json`,
  `web/app/lib/test/routeStub.tsx`
- `.planning/phases/19-frontend-system-view/19-CONTEXT.md`, `19-UI-SPEC.md`
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`,
  `.planning/config.json`
- `.planning/codebase/TESTING.md` (Go section current; frontend section stale),
  `.planning/codebase/CONVENTIONS.md`

### Secondary (MEDIUM confidence)

- `https://ui.shadcn.com/r/styles/base-maia/table.json` — fetched 2026-09-10, confirms the
  `table` component (8 exports, dep `cn`, no `registryDependencies`); corroborates the
  `19-UI-SPEC.md` curl evidence

### Tertiary (LOW confidence)

- None.

## Metadata

**Confidence breakdown:**
- Wire contract / types: HIGH — read `status.go` + `status-contract.md` + `pollruns.go`
  this session; values quoted verbatim.
- Architecture / reuse patterns: HIGH — every referenced file read this session.
- `format.ts` formatter rules: MEDIUM-HIGH — derived from D-09/D-09-a + UI-SPEC; exact
  grammar-ladder thresholds are the planner's to lock in tests.
- Pitfalls: HIGH — each traces to a specific file or a locked decision.
- Test strategy: HIGH — vitest config + 5 existing `*.test.tsx` files read; timezone
  decision (A3) is the one open choice.
- shadcn `table` vendoring: MEDIUM — registry verified 2026-09-10; a network failure at
  exec time has a clear fallback.

**Research date:** 2026-09-10
**Valid until:** 2026-10-10 for the stack (stable); the `/status` contract is frozen
indefinitely (Phase 18 D-11) — re-check only if Phase 18.1 verification surfaces a
contract amendment (none expected).
