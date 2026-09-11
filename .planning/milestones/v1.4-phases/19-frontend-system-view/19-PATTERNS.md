# Phase 19: Frontend — System View - Pattern Map

**Mapped:** 2026-09-10
**Files analyzed:** 14 (7 new, 7 modified)
**Analogs found:** 14 / 14 (all in-repo, all git-tracked)

Every analog below is tracked source under `web/app/` — no gitignored mirror
paths. `internal/webassets/build/client/` is a generated bundle (rebuild + commit
as a phase gate per RESEARCH Pitfall 6); it is not an analog.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/app/routes/system.tsx` | route (view) | request-response (fetch-on-mount + manual refresh) | `web/app/routes/history.tsx` | role + flow, near-exact (diverges on refresh per D-11/D-12) |
| `web/app/routes/system.test.tsx` | test (route) | — | `web/app/routes/history.test.tsx` | exact |
| `web/app/lib/format.ts` | utility (pure formatters) | transform | `web/app/lib/sources.ts` (pure rule module, no React) | role-match (no date helper exists to copy) |
| `web/app/lib/format.test.ts` | test (unit, table-driven) | — | `web/app/lib/authStore.test.ts` / `web/app/lib/api.test.ts` | role-match |
| `web/app/lib/api.ts` (modify) | client (typed fetch wrapper) | request-response | existing `listEvents` / `EventsPage` in same file | exact (in-file precedent) |
| `web/app/lib/api.test.ts` (modify) | test (unit) | — | existing 401/200 cases in same file | exact (in-file precedent) |
| `web/app/lib/sources.ts` (modify) | utility (per-source rules) | transform | existing `isAddableSource` / `identityField` in same file | exact (in-file precedent) |
| `web/app/lib/sources.test.ts` | test (unit) | — | `web/app/lib/authStore.test.ts` | role-match |
| `web/app/routes.ts` (modify) | config (route table) | — | existing `route("history", ...)` line | exact |
| `web/app/root.tsx` (modify) | provider (app layout / nav) | — | existing `<NavLink to="/history">` | exact |
| `web/app/root.test.tsx` (modify) | test (component) | — | existing nav tests in same file | exact |
| `web/app/app.css` (modify) | config (theme tokens) | — | existing `--color-event-*` block in `@theme` | exact |
| `web/app/components/ui/table.tsx` | component (vendored shadcn) | — | `web/app/components/ui/card.tsx` / `badge.tsx` | shape-match (CLI-generated, then prettier) |
| `web/app/components/system/*` | component (presentational) | — | `web/app/components/common/EmptyState.tsx` + `web/app/components/ui/badge.tsx` | role-match (decomposition is planner's call) |

## Pattern Assignments

### `web/app/routes/system.tsx` (route, request-response)

**Analog:** `web/app/routes/history.tsx`

**Imports + container pattern** (`history.tsx:1-12`, `159-161`):
```tsx
import { Loader2 } from "lucide-react"
import { useEffect, useState } from "react"
import { EmptyState } from "~/components/common/EmptyState"
import { Button } from "~/components/ui/button"
import { Skeleton } from "~/components/ui/skeleton"
import { listEvents, type EventItem } from "~/lib/api"
// ...
<div className="flex flex-col gap-6 p-8">
  <h1 className="text-display font-semibold text-foreground">History</h1>
```
For system: swap to `getStatus, ApiError, type StatusResponse` from `~/lib/api`;
`<h1>` copy is `System` (UI-SPEC Copywriting → Page chrome). Add
`Card/CardHeader/CardContent`, `Alert`, `Separator`, `Table*`, `Badge` imports,
`RefreshCw`/`TriangleAlert`/`Ban` from `lucide-react`, and `Link` from
`react-router` for the empty-watchlist callout.

**Mount-effect pattern to copy** (`history.tsx:98-125`):
```tsx
useEffect(() => {
  let cancelled = false
  setInitialLoading(true)
  setError(null)
  // reset accumulated state
  fetchHistoryPage(filters, null)
    .then((page) => { if (cancelled) return; /* setData */ })
    .catch(() => { if (!cancelled) setError("Couldn't load release history.") })
    .finally(() => { if (!cancelled) setInitialLoading(false) })
  return () => { cancelled = true }
}, [filters.artistId, filters.eventType, reloadToken])
```
Divergences required (RESEARCH Pattern 2 + Code Examples "Mount effect"):
- deps array is `[reloadToken]` only.
- `.catch` must branch: `if (err instanceof ApiError && err.status === 401) return`
  before `setLoadError(true)` — do NOT swallow all errors into one string the way
  `history.tsx:114-116` does (that pattern hides the 401).
- also flip a `mountedRef` in the cleanup for the refresh handler's guard.

**Retry (reloadToken) pattern** — copy verbatim (`history.tsx:92-93`, `153-155`, `165-175`):
```tsx
const [reloadToken, setReloadToken] = useState(0)
const handleRetry = () => { setReloadToken((t) => t + 1) }
// ...
<EmptyState heading="…" body="…" action={
  <Button variant="secondary" onClick={handleRetry}>Retry</Button>
} />
```

**Refresh button — busy pattern** adapted from the "Load more" button
(`history.tsx:199-224`):
```tsx
{appendLoading && <p className="text-label text-destructive">{appendError}</p>}
<Button onClick={handleLoadMore} disabled={appendLoading} aria-busy={appendLoading}>
  {appendLoading ? (<><Loader2 className="size-4 animate-spin" aria-hidden="true" />Loading…</>) : ("Load more")}
</Button>
```
For system: label idle `Refresh` (leading `RefreshCw`), busy `Refreshing…`
(leading `Loader2`), `variant="secondary"`, same width. The `text-label
text-destructive` line becomes the D-12 failed-refresh line wrapped in
`aria-live="polite"`. See RESEARCH Code Examples "Refresh button (D-11)".

**Refresh handler with own guard** — new, no analog; RESEARCH Pattern 2 gives the
full body. Key: `mountedRef.current` check before every `setState`, early
`return` on `ApiError` 401, `if (refreshing) return` re-entrancy guard.

**State-machine derivation** — new pure helper, do NOT store in `useState`
(RESEARCH Pattern 3):
```ts
export function deriveLoadedShape(data: StatusResponse): "first-run" | "loaded" {
  const allRunless = Object.values(data.sources).every(
    (s) => s.last_run === null && s.history.length === 0 && s.consecutive_skips === 0)
  return allRunless ? "first-run" : "loaded"
}
```

**Render-precedence** (mirrors `history.tsx:165-226` guard-chain style):
1. `loadError` → full `error` EmptyState + Retry; Refresh control hidden.
2. `initialLoading && !data` → skeleton; Refresh rendered `disabled`.
3. `data` → About `<Card>` + "as of" + empty-watchlist `<Alert>` (both shapes when
   `watchlist_size === 0`) + (`deriveLoadedShape` → first-run EmptyState | per-source panels).

---

### `web/app/routes/system.test.tsx` (test, route)

**Analog:** `web/app/routes/history.test.tsx`

**Partial-mock pattern (keeps real `ApiError`)** — MUST use this, not the bare
`vi.mock("~/lib/api")` at `history.test.tsx:17` (RESEARCH Pitfall 5):
```ts
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  getStatus: vi.fn(),
}))
const mockGetStatus = vi.mocked(getStatus)
```

**Render + query pattern** (`history.test.tsx:1-3`, `47-70`):
```ts
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { renderRoute } from "~/lib/test/routeStub"
// ...
renderRoute(History, "/history")
await screen.findByRole("heading", { name: "Couldn't load release history." })
const retryButton = screen.getByRole("button", { name: "Retry" })
mockListEvents.mockResolvedValueOnce({ ... })
await userEvent.click(retryButton)
```
For system: `renderRoute(System, "/system")`. Cover the 5 render states + refresh
keeps-stale + failed-refresh line + 401-renders-nothing. Use a `makeStatus()`
fixture builder mirroring `makeEvent()` (`history.test.tsx:25-45`).

**Empty-watchlist link** — assert `href="/"` on the `Go to the Watchlist` link,
don't click it (RESEARCH Pitfall 7 — single-route stub has no `/`).

---

### `web/app/lib/format.ts` (utility, transform) — NEW, no direct analog

**Analog for module shape:** `web/app/lib/sources.ts` — a pure, React-free rule
module with a doc-comment header enumerating each exported rule and its call
sites:
```ts
// sources.ts is the single source of truth for the frontend's
// per-search-source business rules... Two rules live here:
//   1. isAddableSource -- ...
//   2. identityField -- ...
export function isAddableSource(sourceName: string): boolean {
  return sourceName === "musicbrainz"
}
```
Copy that structure: header comment listing the 6 formatters + where each is
consumed, then one small exported pure function each. No React import.

**Formatter contract** (RESEARCH "Date/time approach" table) — implement exactly:
`formatRelativeTime(iso, now?)`, `formatAbsoluteTime(iso, now?)`,
`formatIsoTitle(iso)`, `formatClock(d)`, `formatDuration(ms)`,
`formatPollInterval(seconds)`. Inject `now`/`d` params (default `new Date()`) for
deterministic tests. `null` → `"—"`; relative delta `< 0` → `"just now"`;
`duration_ms` buckets `840ms` / `3.1s` / `1m 03s`; interval `900` → `"15 minutes"`.

---

### `web/app/lib/format.test.ts` (test, unit) — NEW

**Analog:** `web/app/lib/authStore.test.ts` (unit, no router) + `api.test.ts`
table-driven style. Use `describe`/`it`/`expect` from vitest, one `describe` per
formatter, table-driven `it.each` for the boundary cases. No `renderRoute`, no
mocking. Timezone: either pin `test: { env: { TZ: "UTC" } }` in
`web/vitest.config.ts` or build expected strings in-test from the same `Intl`
call (RESEARCH Pitfall 4 — planner decides).

---

### `web/app/lib/api.ts` (client) — MODIFY

**Analog:** the existing `EventsPage` / `listEvents` precedent in the same file.

**Wire-type + wrapper pattern** (`api.ts:1-7`, `43-57`, `189-199`):
```ts
// header comment: "Every wire shape below is typed against the real Go
// response bodies read directly from internal/httpserver's handlers... not
// guessed from documentation"
export interface EventsPage {
  events: EventItem[]
  next_cursor: string | null
  has_older_events: boolean
}
export async function listEvents(params?: {...}): Promise<EventsPage> {
  return apiFetch<EventsPage>(`/events${qs ? `?${qs}` : ""}`)
}
```
Add `StatusRun` / `StatusSource` / `StatusInstance` / `StatusResponse` +
`KnownOutcome` typed character-for-character against
`internal/httpserver/status.go:46-77` json tags (quoted in RESEARCH Pattern 1),
plus:
```ts
export async function getStatus(): Promise<StatusResponse> {
  return apiFetch<StatusResponse>("/status")
}
```
`apiFetch` (`api.ts:123-179`) already does the D-16 401 flip (`api.ts:155-158`)
and the `X-Instance-Gated` latch (`api.ts:146-148`) — funnel through it, add zero
per-view auth code.

---

### `web/app/lib/api.test.ts` (test) — MODIFY

**Analog:** existing cases in the same file (`api.test.ts:164-204`).
```ts
function jsonResponse(body, status = 200, headers = {}) {
  return new Response(JSON.stringify(body), {
    status, headers: { "Content-Type": "application/json", ...headers },
  })
}
// 401 case:
fetchSpy.mockResolvedValueOnce(new Response(JSON.stringify({ error: "unauthenticated" }), { status: 401 }))
const err = await api.listWatchlist().then(() => { throw ... }, (e) => e)
expect(err).toBeInstanceOf(api.ApiError)
expect((err as InstanceType<typeof api.ApiError>).status).toBe(401)
```
Add `getStatus` cases: URL is `/status`, resolves the body on 200, 401
propagates as `ApiError`. Uses the `vi.resetModules()` + re-import beforeEach
already set up at `api.test.ts:152-158`.

---

### `web/app/lib/sources.ts` (utility) — MODIFY

**Analog:** the same file. Append alongside `isAddableSource` / `identityField`
(`sources.ts:19-25`), extend the header comment to a third numbered rule:
```ts
export function sourceDisplayName(name: string): string {
  // musicbrainz -> "MusicBrainz", deezer -> "Deezer"; unknown key passes through
}
export const SOURCE_ORDER = ["musicbrainz", "deezer"] as const
```

---

### `web/app/lib/sources.test.ts` (test) — NEW

**Analog:** `web/app/lib/authStore.test.ts` — plain vitest unit file. Cover
`sourceDisplayName` known keys + unknown-key passthrough, and `SOURCE_ORDER`.
(`sources.ts` has no test today — this adds first coverage.)

---

### `web/app/routes.ts` (config) — MODIFY

**Analog:** the same file (`routes.ts:1-11`):
```ts
export default [
  index("routes/watchlist.tsx", { id: "watchlist-index" }),
  route("history", "routes/history.tsx", { id: "history-path" }),
] satisfies RouteConfig
```
Add `route("system", "routes/system.tsx", { id: "system-path" })`. Rewrite the
stale `// D-01: two tabs/routes` comment (`routes.ts:3-7`) to name three tabs —
CLAUDE.md comment discipline, same change.

---

### `web/app/root.tsx` (provider / nav) — MODIFY

**Analog:** the same file (`root.tsx:110-119`):
```tsx
<nav className="flex items-center border-b border-border px-8">
  <NavLink to="/" className={tabLinkClassName}>Watchlist</NavLink>
  <NavLink to="/history" className={tabLinkClassName}>History</NavLink>
  {gateActive && <LogoutButton />}
</nav>
```
Insert `<NavLink to="/system" className={tabLinkClassName}>System</NavLink>`
before `{gateActive && <LogoutButton />}`. `tabLinkClassName`
(`root.tsx:47-54`) already gives the active indigo underline — reuse as-is. Fix
the stale `D-01's two-tab bar` comment at `root.tsx:91-93` in the same change.

---

### `web/app/root.test.tsx` (test) — MODIFY

**Analog:** existing nav-active tests in the same file (RESEARCH notes
`root.test.tsx:73-85` uses `createRoutesStub` directly with multiple routes). Add
"System tab active on /system".

---

### `web/app/app.css` (config, theme) — MODIFY

**Analog:** the `--color-event-*` block inside `@theme` (`app.css:35-41`):
```css
/* 06-UI-SPEC.md Color: event-type badge colors... used only for the small
   color chip/badge on each History event card, never for buttons/nav/focus rings. */
--color-event-new-release: #57f287;
--color-event-guest-feature: #fee75c;
--color-event-deluxe-change: #eb459e;
```
Add inside the same `@theme` block (verbatim from UI-SPEC Color / RESEARCH
Pattern 7), with the one-line "two greens coexist" rationale comment:
```css
--color-status-ok:   #22c55e; /* green-500 — "Success" badge, "Database reachable" pill */
--color-status-warn: #f59e0b; /* amber-500 — "Completed with errors" + "Interrupted", schema drift, skip lines */
```

---

### `web/app/components/ui/table.tsx` (component, vendored) — NEW

**Analog for shape:** `web/app/components/ui/card.tsx` — a `cn()`-only vendored
shadcn primitive exporting a family of sub-components:
```tsx
import * as React from "react"
import { cn } from "~/lib/utils"
function Card({ className, ...props }: React.ComponentProps<"div">) {
  return <div data-slot="card" className={cn("...", className)} {...props} />
}
export { Card, CardHeader, CardFooter, CardTitle, CardAction, CardDescription, CardContent }
```
Generate via `npx shadcn@latest add table` (base-maia registry, dep `cn` only),
then `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` — do NOT
hand-format (RESEARCH anti-patterns). Exports `Table, TableHeader, TableBody,
TableRow, TableHead, TableCell, TableCaption`.

---

### `web/app/components/system/*` (components, presentational) — NEW

Decomposition is the planner's call (RESEARCH: minimum is `system.tsx` + a
`SystemOutcomeBadge` helper). Analogs:

**EmptyState block** — `web/app/components/common/EmptyState.tsx` (whole file, 27
lines): `{ heading, body, action? }` prop shape, `bg-card px-6 py-16` centered
layout, `text-heading` h2 + `text-body text-muted-foreground` p. Reuse the
component directly for first-run / error states — do not re-build.

**Outcome badge** — `web/app/components/ui/badge.tsx`:
```tsx
className: cn(badgeVariants({ variant }), className)   // badge.tsx:40 — custom className merges over variant
// variant="secondary" -> grey neutral fill (badge.tsx:13-14)
// variant="destructive" -> tinted red (badge.tsx:15-16)
// [&>svg]:size-3! built in -> leading <Ban/> icon just works (badge.tsx:8)
```
Implement `classifyOutcome(run)` as a pure helper (RESEARCH Pattern 4) returning
`{ label, className?, variant?, icon? }`; render
`<Badge className="bg-status-ok/15 text-status-ok">` etc. per UI-SPEC Color table.
`titleCase("")` must fall back to `"Unknown"`, never `""`.

**Per-source panel / About block** — `web/app/components/ui/card.tsx` family.
`CardTitle` ships `text-base font-medium` (`card.tsx:40`) — override to
`text-heading` or use a bare `<h2 className="text-heading">` in `CardHeader`
(UI-SPEC Typography note).

**History table caption / timestamp cell** — RESEARCH Code Examples:
```tsx
<time dateTime={run.finished_at} title={formatIsoTitle(run.finished_at)}>
  {formatAbsoluteTime(run.finished_at)}
</time>
```

## Shared Patterns

### Authentication / 401 handling
**Source:** `web/app/lib/api.ts:123-158` (`apiFetch` D-16 interceptor + gate latch)
+ `web/app/root.tsx:101-125` (`<App>` early-return to `<PassphraseScreen>`).
**Apply to:** `system.tsx` (mount effect + refresh handler), `getStatus()`.
**Rule:** route through `apiFetch`; the only 401 code in the view is
`if (err instanceof ApiError && err.status === 401) return` — no per-view UI.
```ts
if (res.status === 401) {
  authStore.markUnauthenticated()
  throw new ApiError(401, "unauthenticated")
}
```

### Error type
**Source:** `web/app/lib/api.ts:107-115`
```ts
export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message); this.name = "ApiError"; this.status = status
  }
}
```
**Apply to:** every `.catch` in `system.tsx` — branch on `err instanceof ApiError
&& err.status === 401`. Tests must partial-mock `~/lib/api` to keep this class real.

### Empty / error / first-run state blocks
**Source:** `web/app/components/common/EmptyState.tsx` — `{ heading, body, action? }`.
**Apply to:** `error` state (with `<Button variant="secondary">Retry</Button>`
action), `first-run` state (no action). Copy strings verbatim from
`19-UI-SPEC.md` Copywriting Contract.

### Router-context test harness
**Source:** `web/app/lib/test/routeStub.tsx` — `renderRoute(Component, path)`.
**Apply to:** `system.test.tsx`. Do not hand-roll a `<MemoryRouter>`.

### Fetch-on-mount + reloadToken Retry
**Source:** `web/app/routes/history.tsx:98-125`, `92-93`, `153-155`.
**Apply to:** `system.tsx` mount effect and full-error Retry path (the Refresh
path deliberately diverges — keep-stale, own guard).

### Container / heading chrome
**Source:** `web/app/routes/history.tsx:159-161`
```tsx
<div className="flex flex-col gap-6 p-8">
  <h1 className="text-display font-semibold text-foreground">History</h1>
```
**Apply to:** `system.tsx` (heading copy `System`).

### Vendored shadcn component shape
**Source:** `web/app/components/ui/card.tsx`, `web/app/components/ui/badge.tsx` —
`cn()`-only, `data-slot`, sub-component family export.
**Apply to:** `table.tsx` (CLI-generated), any `components/system/*` wrappers.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `web/app/lib/format.ts` | utility | transform | No date/relative-time/duration helper exists anywhere in the SPA (`web/app/lib/utils.ts` holds only `cn()`; `EventCard.tsx` renders dates as raw strings). Module *shape* copies `sources.ts`; the formatter *logic* is greenfield — implement to the RESEARCH "Date/time approach" table. |

Partial-analog note: `web/app/components/system/*` has no existing `components/`
subdirectory that renders a fetched status contract — closest are the
presentational primitives (`ui/*`) and `EmptyState`. Decomposition is the
planner's call.

## Metadata

**Analog search scope:** `web/app/routes/`, `web/app/lib/`, `web/app/lib/test/`,
`web/app/components/ui/`, `web/app/components/common/`, `web/app/app.css`,
`web/app/root.tsx`, `web/app/routes.ts`; cross-checked against
`internal/httpserver/status.go` (wire source of truth, quoted in RESEARCH).
**Files scanned:** ~16
**Tracked-source gate:** all analog paths are tracked files under `web/app/` — no
gitignored install/runtime mirrors involved.
**Pattern extraction date:** 2026-09-10
