# Phase 19: Frontend — System View - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-10
**Phase:** 19-frontend-system-view
**Areas discussed:** About block & DB-reachability source, Empty / first-run / error taxonomy, Outcome wording & 'degraded' signal, Refresh & freshness behavior

---

## Todo cross-reference

| Todo | Area | Score | Folded |
|------|------|-------|--------|
| Move `shadcn` from dependencies to devDependencies | tooling | 0.6 | no |
| Resolve D-15 prev-release query files from `--prev-tag` | tooling | 0.6 | no |
| Unify `internal/sqlscan`'s two quote state machines | tooling | 0.4 | no |

**User's choice:** Fold none — all three are backend/CI tooling, unrelated to the frontend System view. Reviewed and left on the `/gsd-quick` backlog.

---

## About block & DB-reachability source

### "Database reachable" indicator source

| Option | Description | Selected |
|--------|-------------|----------|
| Derive from /status | `instance.schema_applied === null` ⇒ unreachable pill; zero extra requests | ✓ |
| Bare fetch('/ready') | Separate unauthenticated call, real readiness pill with reason enum | |
| Both | Schema + version from /status, readiness pill from /ready | |

**User's choice:** Derive from /status.
**Notes:** The contract already models a DB blip as `200` + `schema_applied: null`, so no second request or unwrapped fetch is needed. A real `/ready` badge is deferred (see CONTEXT deferred ideas).

### Schema drift display

| Option | Description | Selected |
|--------|-------------|----------|
| Show numbers + flag mismatch | One "schema 7" line when equal; both values + warning treatment when `applied !== expected` | ✓ |
| Just show the numbers | Render both plainly, no special treatment | |
| Single 'schema' line | Only show applied when equal to expected; hide the distinction | |

**User's choice:** Show numbers + flag mismatch.
**Notes:** Drift is what an operator opens this panel to catch.

### app_version rendering

| Option | Description | Selected |
|--------|-------------|----------|
| Plain text, mono | SHA as-is in monospace, "dev" literal, no link | ✓ |
| Link to GitHub commit | Wrap SHA in a commit link; needs a repo base URL in the frontend | |

**User's choice:** Plain text, mono.
**Notes:** Frontend has no repo URL; wiring one in is out-of-scope config surface.

---

## Empty / first-run / error taxonomy

### State count

| Option | Description | Selected |
|--------|-------------|----------|
| Five states | loading / error / session-expired / first-run / loaded; watchlist-0 as inline copy | ✓ |
| Four states | Fold first-run into loaded | |
| Six states | Add a dedicated whole-page watchlist-empty state | |

**User's choice:** Five states.
**Notes:** first-run = `200` where every source has `last_run === null` && `history.length === 0`. Session-expired is handled automatically by `apiFetch`'s D-16 interceptor.

### First-run copy

| Option | Description | Selected |
|--------|-------------|----------|
| Name the interval | Uses `poll_interval_seconds`, humanized: "runs every 15 minutes — first results after the next cycle" | ✓ |
| Generic wait copy | "Check back shortly", no interval math | |

**User's choice:** Name the interval.

### Empty watchlist (`watchlist_size === 0`)

| Option | Description | Selected |
|--------|-------------|----------|
| Inline note + link | Callout "nothing to poll" + link to Watchlist tab, rest of view renders normally | ✓ |
| Standard first-run/empty copy | Don't special-case it | |

**User's choice:** Inline note + link.
**Notes:** Cycles still run and record against an empty watchlist, so the panels/table still make sense around the callout.

---

## Outcome wording & 'degraded' signal

### 'degraded' labelling (outcome ok, artists_errored > 0)

| Option | Description | Selected |
|--------|-------------|----------|
| Distinct 'degraded' treatment | Success (green) / Completed with errors (amber, derived) / Failed (red) / Interrupted (grey) | ✓ |
| Two tiers only | ok → Success regardless of errored count | |

**User's choice:** Distinct 'degraded' treatment.
**Notes:** The amber tier is client-derived — ROADMAP 18.1 note confirms no `partial` outcome is stored.

### "Time since last successful run" (SYS-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Scan history, explicit copy when none | Walk `history` newest-first for first `outcome === "ok"`; none → "No successful run in recent history" | ✓ |
| Use last_run only | Only consider `last_run`; a succeeded-then-failed source shows nothing | |

**User's choice:** Scan history, explicit copy when none.

### Timestamp rendering

| Option | Description | Selected |
|--------|-------------|----------|
| Relative + absolute on hover | Relative primary text, absolute in `title`; frozen at fetch time; null → "—" | ✓ |
| Absolute only | Formatted absolute local timestamp everywhere | |

**User's choice:** Relative + absolute on hover.
**Notes:** Frozen at fetch, not live-ticking — consistent with the no-polling model.

---

## Refresh & freshness behavior

### In-flight Refresh

| Option | Description | Selected |
|--------|-------------|----------|
| Keep previous data, spinner on button | Data stays, button spins + disables, "as of" updates on success only | ✓ |
| Clear to skeleton like history.tsx | Blank content, show skeleton again | |

**User's choice:** Keep previous data.
**Notes:** Deliberate divergence from `history.tsx` — a status dashboard, not a feed.

### Failed Refresh (non-401)

| Option | Description | Selected |
|--------|-------------|----------|
| Keep stale data + inline error | Previous data stays, non-blocking error line near the button, "as of" holds | ✓ |
| Fall back to full error state | Failed refresh replaces the whole view like a failed initial load | |

**User's choice:** Keep stale data + inline error.
**Notes:** A failed *initial* load still goes to the full error state.

### Auto-refresh model

| Option | Description | Selected |
|--------|-------------|----------|
| Mount + manual only, no timers | Fetch on mount, then only on Refresh; no interval / visibilitychange / focus refetch | ✓ |
| Add a slow background refresh | Scope creep into OBS-01 — offered for explicit rejection | |

**User's choice:** Mount + manual only.
**Notes:** Auto-refresh explicitly rejected — it is OBS-01.

---

## Claude's Discretion

- "System" nav placement — a third top-level tab at `/system` styled like Watchlist / History via `tabLinkClassName`.
- Recent-runs history table layout (per-source vs combined, columns, 50-cap surfacing, degraded-row styling) — deferred to `19-UI-SPEC.md`.
- Exact copy strings for each state, component decomposition, skeleton shape — planner + UI-SPEC.

## Deferred Ideas

- A real `/ready` readiness badge (ready/not-ready + reason enum) via a bare unwrapped fetch.
- Auto-refresh / live updates (OBS-01) — rejected here.
- "Poll now" trigger (OBS-02), paginated history (OBS-03), poll-failure alerting (OBS-04).
- Time-series charts / sparklines — permanent Out of Scope.
- Linking `app_version` to a GitHub commit — needs frontend repo config.
