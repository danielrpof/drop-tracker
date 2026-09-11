# Phase 19: Frontend — System View - Context

**Gathered:** 2026-09-10
**Amended:** 2026-09-10 (post-grill — D-04, D-06, D-07, D-08, D-09, D-11, D-13; see Discussion Log)
**Status:** Ready for planning

<domain>
## Phase Boundary

Add a **"System" view** to the embedded React SPA that renders the frozen
`GET /status` JSON contract for an operator: per-source last-run health, a
recent-runs history table, an about block (app version, schema version,
database reachable), and explicit first-run / empty / error handling. It is
reachable from the SPA's main navigation, fetches on mount, and refreshes
only on a manual Refresh click.

Requirements are the contract: **SYS-01, SYS-02, SYS-03**
(`.planning/REQUIREMENTS.md`).

**In scope:**
- A third top-level nav tab ("System", route `/system`) alongside Watchlist / History
- Per-source panel: last run time, outcome, duration, counts, and time since the last clean run (SYS-01's "last successful run" — see D-08 for the "clean" refinement)
- Recent-runs history table, watchlist size, poll interval, about block (SYS-02)
- Five render states: loading / error / 401 / first-run / loaded (SYS-03)
- Fetch-on-mount + manual Refresh with an explicit "as of" timestamp (SYS-03)
- Reuse of the existing `apiFetch` → global-401 → passphrase-screen flow (SYS-03)

**Not in this phase:**
- Any backend change — `/status` and `/ready` are frozen and shipped (Phases 18 / 18.1)
- Auto-refresh / live updates (OBS-01), a "poll now" trigger (OBS-02),
  paginated history beyond the last 50 (OBS-03), poll-failure alerting (OBS-04)
- Time-series charts, sparklines, graphs (explicit "Out of Scope" in REQUIREMENTS.md)
- Any new frontend build-time config or env var

**Timing note:** Phase 19 depends only on Phase 18 (which froze the `/status`
contract). Phase 18.1 populates real run data but does not change the
contract, so this phase can be built and tested against a fresh instance —
the first-run empty state it must build anyway covers that window.
</domain>

<decisions>
## Implementation Decisions

### About block

- **D-01: "Database reachable" is derived from `/status`, not a second request.**
  Treat `instance.schema_applied === null` as "database unreachable" (render a
  red pill); any integer means reachable. The contract already models this —
  a schema-read failure returns `200` with `schema_applied: null`. No bare
  `fetch('/ready')`, no unwrapped request, no second loading/error path.
  — **Reversibility:** reversible — a `/ready` badge can be added later
  (OBS-adjacent) without disturbing this.

- **D-02: Schema version shows one number when in sync, flags a mismatch when not.**
  `schema_applied === schema_expected` → render a single "schema 7" line.
  `schema_applied !== schema_expected` → render both with a subtle warning
  treatment (e.g. "applied 8 · expects 7"). Drift is exactly what an operator
  opens this panel to catch. Never render a bare `null` — the D-01 unreachable
  pill covers that case.

- **D-03: `app_version` renders as plain monospace text.** The short SHA
  as-is; `"dev"` shown literally. No link — the frontend has no repo base URL
  and wiring one in is new config surface out of scope here.

### Render states

- **D-04: Five states.** `loading` (skeleton) → `error` (`/status` threw a
  non-401; inline Retry, and the header Refresh control is **hidden** — Retry
  is the only recovery affordance) → `session-expired` (401 → the existing
  `PassphraseScreen`, handled automatically by `apiFetch`'s D-16 interceptor,
  no per-view code) → `first-run` (a `200` where **every** source has
  `last_run === null`, `history.length === 0`, **and** `consecutive_skips === 0`)
  → `loaded`.
  - **The `consecutive_skips === 0` clause is load-bearing.** A source whose
    first-ever cycle is slow, hung, or deadlocked records no run row (an
    overlap-skipped tick writes nothing — ROADMAP 18.1) but *does* bump
    `consecutive_skips`. Without the clause that instance renders the
    "no cycles yet, check back after the next cycle" copy while every cycle is
    in fact skipping — the exact failure this view exists to surface. When any
    source has `consecutive_skips > 0`, render `loaded`: the per-source panels
    carry the skip story (D-07 escalation / skip lines), a runless source
    shows its "no cycles recorded" line.
  - **Recompute the state machine on every successful fetch, not just on mount.**
    The run buffer is in-process and resets on restart, so a Refresh right
    after a deploy can turn a `loaded` view into a `first-run`-shaped payload.
    That transition is correct and must happen (the buffer really is empty);
    do not latch the view to `loaded` once it has been there.
  - **Unknown `outcome` never white-screens.** The contract's `outcome` set is
    frozen (`ok` / `error` / `cancelled`), but an N-1/N deploy could put a
    newer backend behind an older SPA bundle. An unrecognised value renders a
    neutral grey badge (a safe title-cased fallback label, never the raw
    string) and the run's `summary` line still renders (D-07).

- **D-05: `watchlist_size === 0` is an inline callout, not a whole-page state.**
  Within `first-run` or `loaded`, show a callout — "No artists on your
  watchlist — the scheduler has nothing to poll." — plus a link to the
  Watchlist tab. Cycles still run and record against an empty watchlist, so
  the source panels and history table still render normally around the callout.

- **D-06: First-run copy names the humanized poll interval.** Use
  `poll_interval_seconds` from the payload, rendered as human text ("15
  minutes"), e.g. "No poll cycles yet. The scheduler runs every 15 minutes —
  first results appear here after the next cycle." Concrete expectation, no
  endless spinner, no "Invalid Date".
  - **Exception — `schema_applied === null` in the first-run state.** The DB is
    unreachable, so the "results appear after the next cycle" promise is false
    (a cycle cannot record against a dead DB). Swap the body to name the real
    problem: "Can't reach the database — poll results won't be recorded until
    it's back. See the About section below." The `200`-with-`null` payload is
    still a real load (not the `error` state); the About block's "Database
    unreachable" pill carries the rest.

### Outcome presentation

- **D-07: Badge tiers, two of them client-derived.** Map from the run object:
  - `outcome === "ok"` && `artists_errored === 0` → **"Success"** (green, `status-ok`)
  - `outcome === "ok"` && `artists_errored > 0` → **"Completed with errors"** (amber, `status-warn`) — *derived label, not stored data (per ROADMAP 18.1 note: no `partial` outcome exists)*
  - `outcome === "error"` → **"Failed"** (red, `destructive`)
  - `outcome === "cancelled"` → **"Interrupted"** (amber, `status-warn`) — carries a
    leading icon the "Completed with errors" tier does not, so the two amber
    tiers stay glanceable apart
  - unrecognised `outcome` → **grey** neutral badge, title-cased fallback label
  Never render the raw enum. Grey is no longer an outcome tier — a `cancelled`
  cycle in a crash loop is an incident, not a footnote, so it gets amber weight.
  - **Per-source escalation line (D-07-adjacent).** In a source's panel, when
    that source's **latest** run has `outcome === "cancelled"`, show a
    `status-warn` line with a leading `TriangleAlert`: "Recent cycles are being
    interrupted — check for a restart or crash loop." Per source, driven by the
    latest run only (a lone shutdown-cancelled run during a deploy trips it
    briefly; a loop keeps it lit).

- **D-08: "Time since last clean run" (SYS-01) scans `history`.** Walk that
  source's `history` array (newest-first, ≤50 entries) for the first element
  with `outcome === "ok"` **and** `artists_errored === 0`; show relative time
  from its `finished_at` (this is the one line that stays relative-primary —
  D-09). None found → the explicit copy "No clean run in recent history" (not
  a date, not blank). "Recent" is honest wording — the ring buffer only holds
  the last N.
  - **"Clean", not "successful".** SYS-01 says "successful", but a run that
    errored some artists still has `outcome === "ok"` and wears the amber
    "Completed with errors" badge — labelling that same run "last successful
    run" in the panel contradicts its own badge. Scanning on
    `artists_errored === 0` and wording it "clean" removes the contradiction.
    A "Completed with errors" run is **not** a clean run for this line.

- **D-09: Timestamps render absolute-primary, relative-on-hover, frozen at fetch.**
  `started_at` / `finished_at` (last-run line and every history-table row) and
  `last_skipped_at` render as an **absolute** local time as the visible value,
  with the relative phrasing ("12 minutes ago") plus the full ISO string
  (with offset) in the `title` attribute. A no-polling panel never re-ticks,
  so a frozen "12 minutes ago" is a lie the moment the operator looks away —
  and an absolute time also sidesteps client/server clock skew. The **one**
  exception is D-08's "last clean run" line, which stays relative-primary
  (staleness matters less there, and "3 days ago" reads faster than a date).
  - **Absolute format (D-09-a):** `HH:MM:SS` (24h, local) when the timestamp
    falls on the client's current date; `MMM D, HH:MM` when it is an earlier
    day — the ≤50-entry buffer can span days on a slow-cadence source, so a
    bare `HH:MM` in a lower row is genuinely ambiguous. Full ISO + offset in
    `title` regardless.
  - **Clock-skew clamp:** any computed relative delta `< 0` (client clock
    behind the server, container drift) renders as "just now", never
    "in 3 minutes". Lives in the shared formatter (see Existing Code Insights).
  - `null` → "—" everywhere, never blank, never "Invalid Date".

### Refresh & freshness

- **D-10: Fetch on mount + manual Refresh only. No timers.** No
  `setInterval`, no `visibilitychange` handler, no focus refetch. Auto-refresh
  is explicitly OBS-01 (deferred) and was offered for rejection — rejected.

- **D-11: An in-flight Refresh keeps the previous data on screen.** The
  Refresh button shows a spinner and disables; content stays; the "as of"
  stamp updates only on success. This deviates from `history.tsx` (which
  blanks to a skeleton on reload) — intentional: History is a feed, this is a
  status dashboard an operator watches.
  - **The refresh path needs its own in-flight guard.** `history.tsx`'s
    `let cancelled = false` cleanup only covers the mount effect. A manual
    Refresh that resolves (or 401s) *after* the session expired and `<Outlet>`
    remounted must not `setState` on the unmounted view and must not leave a
    request in flight behind `<PassphraseScreen>` (success-criterion #5).
    Guard the refresh handler explicitly — an `AbortController`, or an
    ignore/isMounted latch scoped to the handler.

- **D-12: A failed Refresh (non-401) keeps stale data + shows an inline error.**
  Previous data stays, a non-blocking line near the button reads e.g.
  "Couldn't refresh — showing data as of <time>", and "as of" holds at the
  last successful fetch. Retry = press Refresh again. A failed **initial**
  load still goes to the full `error` state (D-04).

- **D-13: "as of" is the client clock when the fetch resolved.** A visible
  "as of HH:MM:SS" (absolute, 24h local — matching D-09-a's precision so the
  freshness anchor and the row times read on the same scale) so the operator
  can judge how stale the frozen times below it are. Not derived from any
  payload field. Rendered in the `first-run` state too (the fetch succeeded —
  it is just empty), not only in `loaded`.

### Claude's Discretion

- **"System" nav placement:** a third top-level tab at route `/system`,
  styled with the existing `tabLinkClassName` in `web/app/root.tsx`, same as
  Watchlist / History. `routes.ts` gains one `route("system", ...)` entry.
  (SYS-01 requires "main navigation" — a peer tab satisfies it; no operator/ops
  visual separation unless UI-SPEC calls for one.)
- **Recent-runs history table layout** — per-source vs one combined table,
  column set, how the 50-entry cap surfaces, degraded-row styling: deferred
  to `19-UI-SPEC.md` (run `/gsd-ui-phase 19` before planning).
- Exact copy strings for every state, component decomposition, skeleton
  shape: planner + UI-SPEC.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### The frozen contract (read first)
- `docs/api/status-contract.md` — the **`GET /status` response contract**,
  frozen by Phase 18. `web/app/lib/api.ts` types directly against this body.
  Every key name, type, and null-behavior the System view relies on is here
  (including: fresh-instance shape, `schema_applied` null on DB blip → still
  `200`, run object fields, `outcome` closed set, `sources` always carries
  both `musicbrainz` and `deezer`).

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — SYS-01 / SYS-02 / SYS-03 are the contract;
  the "Out of Scope" table (no charts/sparklines, no auto-refresh env var)
  binds; OBS-01…04 are the deferred follow-ups.
- `.planning/ROADMAP.md` → "Phase 19: Frontend — System View" → "Notes for the
  phase planner" — the primary brief: mirror `history.tsx` exactly, no
  polling, route through `apiFetch`, `/ready` is unwrapped, guard every
  timestamp, human-phrase outcomes, prettier + `pnpm test` Definition of Done.
- `.planning/ROADMAP.md` → Phase 18.1 "Notes" — the "no `partial` outcome; the
  derived 'degraded' label is Phase 19's job" note that D-07 acts on.

### Frontend patterns this phase mirrors
- `web/app/routes/history.tsx` — the structural template: fetch-on-mount,
  `initialLoading` skeleton, three-way empty/error state, `reloadToken` Retry.
  D-11/D-12 deliberately diverge on refresh behavior (dashboard vs feed).
- `web/app/lib/api.ts` — `apiFetch` core (the single fetch path), `ApiError`
  (carries `status`), the D-16 global-401 interceptor, the `X-Instance-Gated`
  latch. The `/status` wrapper + wire types are added here.
- `web/app/root.tsx` — `App` layout, `tabLinkClassName`, the `NavLink` tab
  bar, the `authed` / `gateActive` gating and the `PassphraseScreen` early
  return (this *is* the 401 re-fetch mechanism — an unauth flip remounts
  `<Outlet/>`).
- `web/app/routes.ts` — the route table; `index` + one `route(...)` today.
- `web/app/components/common/EmptyState.tsx` — `{ heading, body, action? }`
  block for the first-run / empty / error states.

### Backend source of truth for the wire shape
- `internal/httpserver/status.go` — `handleStatus`, the response structs whose
  `json:"..."` tags must stay character-identical to `docs/api/status-contract.md`.
- `internal/pollruns/` — `Store` / `Snapshot()` — what actually populates the
  `sources` map.
- `.planning/phases/18-backend-readiness-poll-run-history-status-api/18-CONTEXT.md`
  — D-11 (contract freeze), D-12 (leaks nothing), the `instance` block rationale.

### Definition of Done
- `C:\CodeProjects\drop-tracker\.claude\CLAUDE.md` "Definition of Done" —
  `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before
  staging, then `corepack pnpm --dir web test`. Hand-formatted TSX fails CI's
  `frontend-test` job.
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/TESTING.md` — house
  frontend conventions and the vitest patterns (singleton-store reset via
  `vi.resetModules()`, partial mock of `~/lib/api` for real `ApiError`).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`apiFetch<T>` (`web/app/lib/api.ts`)** — route the `/status` call through
  it to inherit the D-16 global-401 → `PassphraseScreen` handling and the
  `X-Instance-Gated` latch for free. SYS-03's "reuse existing 401 handling"
  is satisfied structurally, not with new code.
- **`ApiError`** — carries `.status`; a caller can tell a 401 (handled
  upstream) from a 500/503/network failure that drives D-04's `error` state.
- **`history.tsx` state machine** — `initialLoading` / `error` / `reloadToken`
  Retry is the skeleton for D-04; adapt for keep-stale-on-refresh (D-11/D-12).
- **`EmptyState`** — first-run (D-06), watchlist-empty callout (D-05), and
  error (D-04) copy blocks.
- **`tabLinkClassName` + `NavLink` (`root.tsx`)** — the System tab is one more
  `<NavLink to="/system">` in the existing `<nav>`.
- **`web/app/lib/utils.ts`** — confirmed to hold only `cn()`; there is **no**
  date/relative-time helper anywhere in the SPA today (dates are currently
  rendered as raw strings). D-09 introduces relative-time rendering to this
  codebase for the first time — it does not exist to reuse.
- **`web/app/lib/sources.ts`** — the established home for per-source frontend
  rules (`isAddableSource`, `identityField`). Add the display-name lookup here
  (`musicbrainz` → `MusicBrainz`, `deezer` → `Deezer`) — naive title-casing
  the key yields "Musicbrainz".

### Established Patterns
- **Type against the real Go body, never a guess** (`api.ts` header comment).
  The `/status` wire types come from `docs/api/status-contract.md` +
  `internal/httpserver/status.go`, added to `api.ts` as exported interfaces.
- **No polling anywhere in this SPA.** Every route is fetch-on-mount; there is
  no `setInterval` in the codebase. D-10 keeps it that way.
- **Dark theme only** (`root.tsx` D-13) — `class="dark"` unconditional, no
  theme provider. Badge colors (D-07) are dark-theme tokens.
- **Locked copy strings** — prior phases pulled every empty-state string from
  a UI-SPEC "Copywriting Contract". Phase 19's strings come from `19-UI-SPEC.md`.
- **`vi.resetModules()` + partial `~/lib/api` mock** for route tests that need
  a real `ApiError` (from 14-03).

### Integration Points
- `web/app/routes.ts` — one new `route("system", "routes/system.tsx", { id: "system-path" })`.
  Update the stale "D-01: two tabs/routes" comment in the same change.
- `web/app/root.tsx` — one new `<NavLink to="/system">` in the tab bar.
  Update the stale "D-01's two-tab bar" comment in the same change
  (CLAUDE.md comment discipline).
- `web/app/lib/api.ts` — new `getStatus()` wrapper + `StatusResponse` /
  `RunObject` / `SourceStatus` / `InstanceBlock` wire types (`schema_applied`
  typed `number | null`).
- **`web/app/lib/format.ts` (new) + `format.test.ts`** — the single home for
  the derived-display formatters, none of which exist today. Co-located test
  file mirrors `api.test.ts` / `authStore.test.ts`. Must cover:
  - relative time, with the D-09 `< 0 → "just now"` clamp and the grammar
    ladder (just now / minutes / hours / days)
  - absolute time, the D-09-a today-vs-earlier-day rule
  - `duration_ms` → `840ms` (< 1000) / `3.1s` (≥ 1000) / `1m 03s` (≥ 60000) —
    a cycle behind a rate-limited API can run minutes
  - `poll_interval_seconds` → "15 minutes" (900) / "90 seconds" (90)
  - `null` → "—"
- `web/app/lib/sources.ts` — add the source display-name lookup (see above).
- `web/app/routes/system.tsx` — the new view (+ `system.test.tsx`).
- Likely new `web/app/components/system/*` — panel, history table, about
  block, badge (UI-SPEC decides the split).
</code_context>

<specifics>
## Specific Ideas

- **`instance.schema_applied === null` ⇒ "database unreachable" red pill.** The
  single reachability signal, no `/ready` call.
- **Schema in sync:** "schema 7". **Drift:** "applied 8 · expects 7" with a
  warning treatment.
- **Badge tiers:** Success (green) / Completed with errors (amber, derived
  from `outcome==="ok" && artists_errored>0`) / Failed (red) / Interrupted
  (amber + distinguishing icon, from `outcome==="cancelled"`) / unknown
  outcome → grey neutral fallback (title-cased, never the raw enum).
- **Interrupted escalation:** per-source `status-warn` line "Recent cycles are
  being interrupted — check for a restart or crash loop." when that source's
  latest run is `cancelled`.
- **No clean run in buffer:** "No clean run in recent history" (scan
  `outcome==="ok" && artists_errored===0`).
- **First-run:** "No poll cycles yet. The scheduler runs every 15 minutes —
  first results appear here after the next cycle." (interval from
  `poll_interval_seconds`, humanized). **DB-unreachable variant** (`schema_applied
  === null`): "Can't reach the database — poll results won't be recorded until
  it's back. See the About section below."
- **First-run trigger** also requires `consecutive_skips === 0` on both
  sources — a skipping-but-runless instance is `loaded`, not first-run.
- **Empty watchlist callout:** "No artists on your watchlist — the scheduler
  has nothing to poll." + link to Watchlist.
- **Failed-refresh line:** "Couldn't refresh — showing data as of <time>".
- **Freshness:** a visible "as of HH:MM:SS", client clock at fetch resolution,
  shown in first-run and loaded.
- **Timestamps:** absolute-primary (`HH:MM:SS` today / "MMM D, HH:MM" earlier),
  relative + full ISO in `title`. The "last clean run" line is the lone
  relative-primary exception. Frozen at fetch, not live-ticking. Relative
  deltas `< 0` clamp to "just now".
- `null` timestamp → "—" everywhere.
- On a fresh instance `poll_interval_seconds` is `900`; `schema_applied` /
  `schema_expected` are both `7` (no migration since 000007).
- `sources` always has exactly `musicbrainz` and `deezer` keys, even fresh —
  the view can iterate them without guarding for absence.
</specifics>

<deferred>
## Deferred Ideas

- **A real `/ready` readiness badge** (ready / not-ready + `schema_behind` /
  `schema_dirty` / `db_unreachable` reason enum) via a bare unwrapped
  `fetch('/ready')` — considered for the about block, deferred. D-01 derives a
  simpler reachable/unreachable signal from `/status` instead. Revisit if an
  operator wants the dirty/behind distinction surfaced in the UI.
- **Auto-refresh / live updates** on the System view — OBS-01, explicitly
  rejected here (D-10).
- **A "poll now" trigger** for a source from the System view — OBS-02.
- **Paginated / filterable run history** beyond the last 50 — OBS-03.
- **Poll-failure alerting** (Discord alert after M consecutive source errors)
  — OBS-04.
- **Time-series charts / sparklines** on the System view — permanent "Out of
  Scope" (REQUIREMENTS.md): would add a charting dependency for marginal value.
- **Linking `app_version` to the GitHub commit** — needs a repo base URL wired
  into the frontend; out of scope (D-03).

### Reviewed Todos (not folded)
- *Move `shadcn` out of `web/package.json` dependencies* — tooling cleanup,
  not a System-view concern; stays on the `/gsd-quick` backlog.
- *Resolve D-15 prev-release query files from `--prev-tag`* — backend
  `cmd/migration-check` tooling, unrelated.
- *Unify `internal/sqlscan`'s two quote state machines* — backend, unrelated.
</deferred>

---

*Phase: 19-frontend-system-view*
*Context gathered: 2026-09-10*
