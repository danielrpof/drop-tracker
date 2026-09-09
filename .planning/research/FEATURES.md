# Feature Research

**Domain:** Operator observability for a single-instance, self-hosted background-polling service (Go single binary: API + robfig/cron poller + Discord notifier + embedded React SPA)
**Researched:** 2026-09-09
**Confidence:** HIGH for the health/readiness split (well-documented industry convention); MEDIUM–HIGH for job-run-history modelling and retention (converging practice across pg_cron, Jenkins, Sonarr/Radarr, Confluence scheduler); MEDIUM for the operator-panel scope call (judgement against comparable self-hosted tools).

This milestone (v1.4) has a fixed four-feature target. This document does **not** re-open that scope — it establishes the *expected behaviour* of each feature so requirements stay tight, and draws the table-stakes / differentiator / anti-feature lines the roadmap and REQUIREMENTS.md Out-of-Scope section need.

---

## Behavioural conventions (the load-bearing part)

### (a) The health-vs-readiness split

The industry convention is the Kubernetes three-probe model (liveness / readiness / startup). It is the right frame here even though drop-tracker does not run on Kubernetes, because Phase 17's future SSH deploy health-gate is exactly a readiness poller, and the convention is what any future reviewer will measure the endpoints against.

| Probe | Question it answers | Failure action | May check dependencies? | drop-tracker mapping |
|-------|--------------------|----------------|-------------------------|----------------------|
| **Liveness** | "Is this process wedged and only a restart will fix it?" | Kill + restart the container | **No** — process-internal only | The existing `/health` is closest to this role, but today it *does* ping the DB (D-03/D-04). See note below. |
| **Readiness** | "Can this instance serve correct responses *right now*?" | Remove from load balancer / fail the deploy gate; **no restart** | **Yes** — hard dependencies belong here | **The new `/ready`**: DB reachable AND schema at expected migration version. |
| **Startup** | "Is a slow-starting process still initialising (don't judge it yet)?" | Suppress liveness/readiness until it passes | n/a | Not needed. Boot migrations already run under a bounded retry loop; revisit only if boot time grows. |

**HTTP status codes (Kubernetes convention, and what a deploy gate will assume):**
- Any code `>= 200 and < 400` = success/ready.
- Any other code = failure/not-ready.
- Use **`200 OK` when ready, `503 Service Unavailable` when not** — this matches the existing `/health` handler, which already returns `503` on a failed DB ping. Do not invent a 4xx for "not ready"; 4xx implies the caller did something wrong.

**What `/ready` MUST NOT do (all standard readiness-probe hygiene):**
- **No authentication.** Register it as an exact registered path *outside* the passphrase-gated `chi.Group`, exactly as `server.go` already does for `/health` (`r.Get("/health", …)` sits outside the `gate.Authenticate` group; D-03's exact-path exemption). A deploy gate and an uptime monitor cannot present a passphrase.
- **No fan-out to third parties.** Never touch MusicBrainz or Deezer. Readiness is about *this* instance's ability to serve, not upstream weather.
- **Cheap and bounded.** A `context.WithTimeout` around the DB ping (the `/health` handler already uses a 3 s `healthPingTimeout` — reuse that pattern) and a single indexed-row read for the migration version. No table scans, no counting rows, no `COUNT(*)` over events.
- **Idempotent GET, no side effects.** No row writes, no cache warming.
- **Safe to poll every few seconds** without accumulating goroutines or connections (the `/health` handler's bounded-context comment block spells out this exact hazard).

**The "expected migration version" check — concrete:**
- `internal/db` already has the two halves internally: golang-migrate's `m.Version()` (current applied version + dirty flag) and the unexported `maxSourceVersion(src)` (highest embedded migration). Phase 18 adds a small **exported** accessor (e.g. `db.SchemaStatus(ctx) (current, expected uint, dirty bool, err error)`) that `/ready` calls alongside `db.Ping`.
- Ready when: DB ping succeeds **AND** schema not dirty **AND** `current == expected`.
- **Design decision to settle in the phase (flag for the plan):** what to do when `current > expected` (the N-1 rollback / "DB ahead of binary" case that Phase 16's ahead-of-source no-op guard deliberately makes bootable). Two defensible answers: (1) strict `current == expected` — simplest, and a deploy gate wants proof the new migration actually applied; the N-1 binary then reports not-ready for the brief operator-controlled rollback window, which is acceptable. (2) `current >= expected` — consistent with Phase 16 having already ruled an ahead schema safe to serve against. **Recommendation: strict `==`**, with a one-line comment citing the tradeoff, because `/ready`'s primary consumer is the deploy gate and "the schema is exactly what this build expects" is the property that gate needs.

**Note on the existing `/health` (in scope to decide, not necessarily to change):** a textbook liveness probe does *not* check the database — a DB outage should not cause a restart loop and a simultaneous thundering-herd reconnect on DB recovery. drop-tracker runs one container with no orchestrator restarting it on `/health` failure today, so the current DB-pinging `/health` is not actively harmful. **Recommendation: leave `/health` exactly as-is** (changing a shipped, documented contract for a hypothetical is not worth it) and let `/ready` be the dependency-aware endpoint. Write one sentence into the docs: *"deploy/uptime health-gates poll `/ready`; `/health` is liveness."* Categorise any actual `/health` behaviour change as an explicit non-goal for this milestone.

### (b) What a "last poll run" / job-run-history row captures

Converging schema across pg_cron's `cron.job_run_details`, generic `cron_executions` tables, Jenkins build history, and Confluence's `scheduler_run_details`:

| Field group | Fields | Purpose |
|-------------|--------|---------|
| **Identity** | `id`, `source` (`musicbrainz` \| `deezer`) | One row per cycle invocation per source. The poller already runs two fully independent cron entries (D-08). |
| **Timing** | `scheduled_at` (tick time) vs `started_at` (actual start), `finished_at` | The `scheduled_at`→`started_at` gap is scheduler lag / contention — cheap to record, genuinely diagnostic. `finished_at - started_at` = duration (derive in the query or the UI, don't store redundantly). |
| **Outcome** | `outcome` enum + `error_summary` (nullable text) | See state list below. `error_summary` is a short string, never a stack trace or raw upstream body. |
| **Work counters** | `artists_checked`, `artists_errored`, `events_recorded` | The domain-specific payload — this is what turns a generic "job succeeded" log into something an operator of *this* app can act on. |

**Sensible outcome states for a cron invocation:**

| Outcome | When | Counters | Notes |
|---------|------|----------|-------|
| `ok` | Cycle ran to completion | populated | `artists_errored > 0` is still `ok` — the per-artist errors are isolated by design (PERF-03). The counter carries the nuance; do **not** add a separate `partial`/`degraded` state (extra enum value, more UI branching, no new information). |
| `skipped_overlap` | CAS overlap guard fired — previous cycle for this source still running (`ErrCycleInProgress`, already a `Warn` log line today) | null | **Record this as a first-class row.** It is the single most useful non-obvious thing this table surfaces: it answers "why didn't the 12:00 cycle run?" ("the 11:55 cycle overran"). `started_at == finished_at ≈ tick time`. |
| `cancelled` | Context cancelled mid-run — process shutting down (`ctx.Err()`, poller already returns this) | populated with whatever completed | Distinct from `error` so a graceful deploy restart doesn't look like a failure. |
| `error` | The cycle function itself returned a non-`ErrCycleInProgress` error (e.g. `store.List` failed) | may be zero | Rare — per-artist failures don't reach here. |

**Explicitly not a stored state:** `missed` (a tick that never fired at all). You cannot record a row from inside a job that didn't run. Detecting it requires comparing `now` against `last_run + interval` — that is scheduler-lag *monitoring*, which is out of scope this cycle. `/status` can *derive* "last run was N minutes ago, interval is M" for display, but there is no `missed` row.

**`RunRecorder` seam — implementation shape:** mirror `EventRecorder` / `Notifier` exactly (narrow interface declared in the *consumer* `internal/poller`, e.g. `RecordRun(ctx, RunResult) error`; the poller holds no DB connection, D-11). The one wrinkle: `runCycle` has **multiple early-return paths** (CAS failure → `skipped_overlap`; `store.List` error → `error`; cancellation → `cancelled`; normal end → `ok`). The recorder must fire on *all* of them — use a single deferred call at the top of `runCycle` that reads a mutable `RunResult` struct, so no exit path can skip it. A failed `RecordRun` is logged, never propagated (same rule as `NotifyPending`: observability must not break polling).

### (c) What the operator status panel should show (and what over-builds it)

Comparable single-instance self-hosted tools: Sonarr / Radarr ("System → Status" + "System → Tasks"), Miniflux, Syncthing, Uptime Kuma, Healthchecks.io. The consistent useful core:

**Genuinely useful (belongs in `/status` + the System view):**
- **Per source: last run** — relative time ("3 min ago"), `outcome`, duration, the three work counters, and the next scheduled run (derive from `last started_at + interval`) / the configured interval.
- **Time since last *successful* (`ok`) run, per source** — the headline health signal. If MusicBrainz has been `skipped_overlap` or `error` for six cycles, that one line says so.
- **Recent run history** — last N rows per source (N ≈ 10–20) as a plain table. Enough to see a pattern; not a log.
- **Watchlist size** — "am I tracking what I think I'm tracking" (reuse `watchlist.Store`; a `COUNT` query is fine here — it's a gated, human-triggered endpoint, not a probe).
- **Poll interval** — from `config.Config`, so the operator knows cadence without shelling in to read env vars.
- **An "About" block** — app version / build info, current schema migration version, DB reachable yes/no. Standard in every tool listed; cheap; also makes `/status` a useful "is my deploy actually the version I pushed" check.

**Over-building — do NOT put in the panel this cycle (candidates for REQUIREMENTS.md Out of Scope):**
- Time-series charts / sparklines of run duration or counts — explicitly out of scope for the milestone, and low signal at a multi-minute poll cadence anyway.
- Full paginated history with date-range filters — "last N rows" is the whole point of the retention design; pagination implies keeping more than N.
- Per-artist drill-down from the panel — structured logs already carry `cycle_id` + `artist_mbid`; the History screen already shows per-artist *outcomes* (events).
- In-browser log viewer — large scope; `docker logs` / `docker compose logs` exists.
- Live-updating dashboard (WebSocket / SSE / sub-30 s auto-refresh) — this is a screen looked at occasionally; a manual refresh, or at most a 30–60 s poll while the tab is open, is sufficient. Live updates add reconnect logic and server push for near-zero value.
- Manual "poll now" trigger button — tempting (Sonarr has one), but it adds a mutating authenticated endpoint that has to interact correctly with the overlap guard and the CSRF middleware, for marginal benefit (wait one interval, or restart). Defer; possible future differentiator.
- Alerting-rule / threshold configuration in the UI — that is the deferred poll-failure-alerting feature.
- SLO / uptime-percentage tracking — out of scope; needs the `missed`-run inference this milestone deliberately doesn't do.

### (d) Retention / pruning for `poll_runs`

**The table is small and slowly-growing:** 2 sources, one row per poll interval per source. At a 15-minute interval that's ~192 rows/day total; even at a 5-minute interval, ~576/day. The only real risk is unbounded growth over months/years — there is no performance pressure.

| Approach | Fit here |
|----------|----------|
| **Prune-on-insert, keep last N per source** (`DELETE … WHERE source = $1 AND id NOT IN (SELECT id … WHERE source = $1 ORDER BY started_at DESC LIMIT N)`, run in the same statement/transaction as the insert) | **Recommended — and what the milestone already specifies.** Self-contained (lives entirely in the `RunRecorder` insert path), deterministic hard bound (exactly N per source), no scheduler wiring, no new env var. N is a compile-time constant (≈ 50–100 per source). |
| Scheduled sweep (a third cron entry) | Rejected — adds scheduler surface and a lifecycle to manage for a table that a one-line `DELETE` on insert keeps bounded. |
| Time-window (`WHERE started_at < now() - INTERVAL 'D days'`, à la `EVENT_RETENTION_DAYS`) | Rejected — ties history *depth* to wall-clock, so changing the poll interval silently changes how many runs you can see. Count-based gives a hard size guarantee regardless of interval. (Jenkins "keep last N builds", Confluence `scheduler_run_details`, pg_cron users all land on count-based for run history.) |

**Why not reuse the events-retention pattern (D-10, soft-delete/filter):** events rows must survive forever because detection state (dedup keys, deluxe-change baselines, seed-mode signal) is read from the full table — hence soft-delete. `poll_runs` rows have **no downstream reader**; nothing computes state from them. A hard `DELETE` is correct and simpler here. State this contrast explicitly in the plan, because "we have a retention precedent" will make a reviewer reach for the wrong tool.

**Details:**
- Index on `(source, started_at DESC)` — serves both the prune sub-query and `/status`'s "last N per source" read.
- The DELETE-per-insert dead-tuple / WAL concern from general retention literature (TimescaleDB `drop_chunks`, batched deletes) is irrelevant at ~1 deleted row per insert; autovacuum handles it.
- `skipped_overlap` rows count against N. That's acceptable — if a source is overrunning constantly, the history *should* be full of skip rows. If it ever proves annoying, prune could exclude skips from the count, but don't build that up front (YAGNI).

---

## Feature Landscape

### Table Stakes (expected of an "operator observability" milestone)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `/ready` readiness probe (public, unauthenticated, DB + schema-version, 200/503) | Any service intended to sit behind a deploy or uptime health-gate needs a dependency-aware readiness endpoint distinct from liveness — this is *the* standard convention | **LOW** | Reuses `db.Ping` + `healthPingTimeout` pattern; needs one new exported `db.SchemaStatus` accessor; register outside the gate group like `/health` |
| `poll_runs` table (one row per cycle per source: timing, outcome, work counters) | Without a persisted record there is nothing for `/status` or the UI to show — it is the substrate of the whole milestone | **MEDIUM** | New migration; `(source, started_at DESC)` index; outcome enum with 4 states |
| `RunRecorder` seam wired into `poller.runCycle` on every exit path | The poller must stay DB-connection-free in principle (D-11); recording must not be skippable by an early return | **MEDIUM** | Mirror `EventRecorder`; single deferred recorder call over a mutable result struct; failure logged not propagated |
| `GET /status` — gated JSON (last run per source, recent history, watchlist size, poll interval, about-block) | The operator-facing read surface; JSON so the SPA (and curl) can consume it | **LOW–MEDIUM** | Register via `registerDataRoutes` *inside* the gated group — NOT exempt like `/health` |
| React "System" view rendering `/status` | The milestone's user-visible deliverable; operators expect a screen, not curl | **MEDIUM** | New SPA route + nav entry; own UI-SPEC (Phase 19) |
| "Time since last successful run, per source" (derived, shown on the panel) | The one-glance health signal every comparable tool surfaces | **LOW** | Pure derivation from `poll_runs`; no storage |
| App version / build + current schema version in `/status` | Standard "About" content; doubles as a deploy-verification check | **LOW** | Version already available from the build; schema version from the same accessor `/ready` uses |

### Differentiators (cheap, in-scope, above a plain success/fail cron log)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Domain work-counters (`artists_checked`, `artists_errored`, `events_recorded`) per run | Turns "job ran" into "checked 42 artists, 1 errored, found 3 releases" — actionable for *this* app | **LOW** (once the row exists) | Counters already computable inside `runCycle` from existing loop state |
| `skipped_overlap` recorded as a first-class outcome row | Answers "why didn't the poll run at 12:00?" — most self-hosted tools leave this invisible in logs only | **LOW** | The CAS branch already logs a `Warn`; add a recorder call there |
| `scheduled_at` vs `started_at` gap (scheduler-lag visibility) | Early signal that cycles are starting to bunch up / contend before they actually overlap | **LOW** | Record the cron tick time alongside actual start |
| `cancelled` distinguished from `error` | A graceful deploy restart mid-cycle doesn't masquerade as a failure in the history | **LOW** | Poller already returns `ctx.Err()` distinctly |

### Anti-Features (write into REQUIREMENTS.md → Out of Scope, with the reason)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Prometheus `/metrics` / OpenMetrics export | "Real observability means metrics" | Explicitly deferred at the project level; pulls in a metrics library + a scrape target + eventually Grafana — a whole tier this milestone is deliberately below | Structured logs + `poll_runs` + `/status` cover the single-operator need |
| Time-series charts / sparklines of run duration or counts | Dashboards "should" have graphs | Charting lib + storing more than last-N history; near-zero signal at a multi-minute cadence | The last-N table shows trend well enough at this scale |
| Poll-failure alerting / paging / "missed run" detection | "Tell me when polling breaks" | Requires scheduler-lag inference (`now` vs `last_run + interval`), an alert-state machine, and a delivery channel decision — a feature in its own right; explicitly out of scope | Discord notifier already fires on the brute-force-gate path; poll-failure alerting is a future milestone. `/status` shows staleness on demand. |
| Manual "poll now" button / endpoint | "I don't want to wait for the next tick" | New mutating authenticated endpoint that must interact correctly with the overlap guard + CSRF middleware, for marginal benefit | Wait one interval, or `docker compose restart` |
| Live-updating dashboard (WebSocket / SSE / auto-refresh < 30 s) | "Real-time status" | Server push + client reconnect logic for a screen viewed occasionally | Manual refresh; optionally a 30–60 s poll while the tab is visible |
| Full paginated run history + date-range filters | "I want to see everything" | Contradicts the count-based retention design (would need to keep > N rows); pagination UI + query params | Last N per source is the deliberate bound |
| In-browser log viewer | "Show me the logs in the UI" | Large scope (log capture, streaming, filtering, redaction); duplicates `docker logs` | `docker compose logs`; logs already carry `cycle_id` correlation |
| Per-source configurable retention env var (`POLL_RUN_RETENTION_*`) | Consistency with `EVENT_RETENTION_DAYS` | The milestone explicitly says no new env var; the table is tiny and a constant N is sufficient forever | Compile-time constant (≈ 50–100/source) |
| Storing per-artist poll results as rows | "Full detail in the DB" | Explodes row count; that's what structured logs are for; `poll_runs` is a per-cycle *summary* | One summary row per cycle + logs for per-artist detail |
| Adding a DB check to a liveness probe / making `/health` "smarter" | "Health should mean healthy" | A DB-checking liveness probe under an orchestrator causes restart loops + thundering-herd reconnects on DB recovery | `/ready` is the dependency-aware endpoint; `/health` stays as liveness |
| SLO / uptime-percentage tracking on the panel | "What's my uptime?" | Needs the `missed`-run inference this milestone doesn't do; implies historical retention beyond last-N | Out of scope; a monitoring concern |
| `startup` probe / boot-progress endpoint | Kubernetes has three probes | Not on Kubernetes; boot migrations already retry under a bounded loop; no slow-start problem today | Revisit only if boot time becomes a real issue |

---

## Feature Dependencies

```
poll_runs table (migration)
    └──required by──> RunRecorder seam ──wired into──> poller.runCycle
                          └──required by──> GET /status ──required by──> React "System" view

/ready
    └──requires──> db.SchemaStatus() accessor (new, exported, in internal/db)
                       └──reuses──> golang-migrate m.Version() + maxSourceVersion(src)  [both already exist internally, Phase 16]
    └──reuses──> db.Ping + healthPingTimeout pattern  [already in /health handler]
    └──registered like──> /health  (exact path, OUTSIDE the passphrase-gate chi.Group — server.go D-03 pattern)

GET /status
    └──registered via──> registerDataRoutes  (INSIDE the gated group — unlike /health and /ready)
    └──reads──> poll_runs, watchlist.Store (count), config.Config (poll interval), build version, db.SchemaStatus

poll_runs retention (prune-on-insert, count-based, hard DELETE)
    └──contrasts with──> events retention (D-10 soft-delete/filter)   [DIFFERENT approach, deliberately]
```

### Dependency Notes

- **`/status` and the System view require `poll_runs` + `RunRecorder`:** they have nothing to render until runs are being recorded. This is what forces the Phase 18 (backend) → Phase 19 (UI) ordering.
- **`RunRecorder` follows the established poller-seam pattern:** `EventRecorder` and `Notifier` are already narrow interfaces declared in `internal/poller`, with the poller holding no DB handle (D-11). `RunRecorder` is the same construction — a reviewer will expect it to look identical.
- **`/ready` reuses Phase 16 machinery:** `maxSourceVersion` and `m.Version()` already exist in `internal/db/migrate.go` (unexported). Phase 18's only new plumbing is one exported accessor over them plus the `schema_migrations` / library read.
- **`/status` depends on the passphrase gate (`internal/authgate`):** it carries operational detail (watchlist size, error summaries, version) and must sit behind `gate.Authenticate` + `RequireCSRFHeader` via `registerDataRoutes`. `/ready` and `/health` are the *only* exact-path exemptions — do not add `/status` to that list.
- **`poll_runs` retention deliberately does NOT reuse the events-retention pattern:** events use soft-delete because detection state reads the full table (D-10); `poll_runs` has no downstream reader, so hard `DELETE` on insert is correct and simpler.

---

## MVP Definition

### Phase 18 — Backend (launch with)

- [ ] `poll_runs` migration + `(source, started_at DESC)` index — substrate for everything else
- [ ] `RunRecorder` seam, wired into `poller.runCycle` on **all** exit paths (ok / skipped_overlap / cancelled / error) via a deferred call
- [ ] Prune-on-insert, count-based, per-source, hard DELETE, N = compile-time constant
- [ ] `db.SchemaStatus()` exported accessor
- [ ] `GET /ready` — public, unauthenticated, `db.Ping` + schema-version, 200 / 503, bounded context
- [ ] `GET /status` — gated JSON: last run per source, recent history (last N), watchlist size, poll interval, about-block (version + schema version + DB up)
- [ ] One doc sentence: deploy/uptime gates poll `/ready`; `/health` is liveness

### Phase 19 — UI (launch with)

- [ ] "System" SPA route + nav entry
- [ ] Panel: per-source last-run cards (relative time, outcome, duration, counters, next run), "time since last success", recent-run table, watchlist size, poll interval, about-block
- [ ] UI-SPEC for the above (via `/gsd-ui-phase`)
- [ ] Vitest coverage for the new view, `/status` API boundary mocked

### Explicitly deferred (future milestone / never)

- [ ] Prometheus `/metrics` + Grafana — deferred at project level
- [ ] Poll-failure alerting / missed-run detection — its own feature
- [ ] Manual "poll now" trigger — revisit if operators actually ask
- [ ] Charts / time-series of run metrics — low value at this cadence

---

## Feature Prioritization Matrix

| Feature | User (operator) Value | Implementation Cost | Priority |
|---------|-----------------------|---------------------|----------|
| `poll_runs` + `RunRecorder` seam | HIGH | MEDIUM | P1 |
| `GET /status` | HIGH | LOW–MEDIUM | P1 |
| React "System" view | HIGH | MEDIUM | P1 |
| `/ready` probe | HIGH (unblocks Phase 17) | LOW | P1 |
| `skipped_overlap` as a first-class outcome | MEDIUM–HIGH | LOW | P1 (fold into `poll_runs`) |
| Domain work-counters in the row | HIGH | LOW | P1 (fold into `poll_runs`) |
| `scheduled_at` vs `started_at` lag | MEDIUM | LOW | P2 |
| `cancelled` vs `error` distinction | MEDIUM | LOW | P2 |
| Prune-on-insert retention | MEDIUM (hygiene) | LOW | P1 |
| Manual "poll now" | LOW | MEDIUM | P3 (defer) |
| Charts / live updates / alerting | LOW–MEDIUM | HIGH | P3 (out of scope) |

**Priority key:** P1 = must have for the milestone · P2 = cheap, include if it falls out naturally · P3 = defer / out of scope

---

## Comparable-tool feature analysis

| Feature | Sonarr / Radarr | pg_cron | Uptime Kuma | drop-tracker approach |
|---------|-----------------|---------|-------------|----------------------|
| Readiness vs liveness split | health checks page (dependency warnings) | n/a | HTTP/keyword monitors | `/ready` (deps) + `/health` (liveness), k8s convention |
| Job/run history | "System → Tasks": last execution, last duration, next execution, interval | `cron.job_run_details`: start/end/status/return_message | ping history + status pages | `poll_runs`: timing + 4-state outcome + domain counters |
| "Skipped because still running" | not surfaced | not surfaced (overlap = deadlock risk) | n/a | first-class `skipped_overlap` outcome row |
| Run-history retention | keep-last-N per task | **grows unbounded** (common complaint) | configurable retention days | count-based prune-on-insert, N per source |
| Operator status screen | "System → Status" (version, disk, health) + "Tasks" | SQL only | dashboard + status page | single "System" view over `/status` JSON |
| Charts / metrics | minimal | none | response-time graphs | **deliberately none** this cycle |

---

## Sources

- Kubernetes docs — Configure Liveness, Readiness and Startup Probes (`kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/`): probe roles, HTTP 200–399 = success convention, "keep probes lightweight", readiness for dependencies. Confidence HIGH.
- "Kubernetes Liveness Probe Anti-Patterns That Cause Cascading Failures" (alexandre-vazquez.com); Codefresh "Kubernetes Deployment Antipatterns part 3"; DEV Community "Kubernetes Health Probes Done Right (2026)" — liveness must not check external deps; DB-in-liveness → restart loop + thundering herd. Confidence HIGH (consistent across independent sources).
- pg_cron / `cron.job_run_details` (AWS RDS & Aurora pg_cron docs; myDBA.dev "pg_cron Monitoring") — run-history field set (start_time, end_time, status, return_message); unbounded growth as a known operational problem. Confidence MEDIUM–HIGH.
- Atlassian support — "Table scheduler_run_details keeps growing" — real-world run-history table growth, count-based purge as the fix. Confidence MEDIUM.
- Tiger Data / TimescaleDB retention, "Time-based retention strategies in Postgres" (Sequin), Sophia Willows "Efficient data retention policies" — DELETE dead-tuple/WAL cost at scale (not relevant here), scheduled-sweep vs on-write tradeoffs. Confidence MEDIUM.
- Sonarr / Radarr "System → Status" and "System → Tasks" UI (last execution / last duration / next execution / interval; keep-last-N task history) as the closest single-instance self-hosted analogue. Confidence MEDIUM (product familiarity, not re-verified this session).
- drop-tracker codebase: `internal/poller/poller.go` (`ErrCycleInProgress`, `runCycle` exit paths, EventRecorder/Notifier seam pattern, D-08 per-source independence), `internal/httpserver/health.go` (`healthPingTimeout`, 503-on-ping-fail), `internal/httpserver/server.go` (`/health` exact-path gate exemption, `registerDataRoutes`, gated `chi.Group`), `internal/db/migrate.go` (`m.Version()`, `maxSourceVersion`), `internal/config/config.go` (`EVENT_RETENTION_DAYS` precedent), `.planning/PROJECT.md` (D-03/D-04, D-10, D-11, Phase 16 ahead-of-source guard). Confidence HIGH.

---
*Feature research for: operator observability of a single-instance self-hosted polling service*
*Researched: 2026-09-09*
