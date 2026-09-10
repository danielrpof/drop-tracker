# Requirements: drop-tracker

**Defined:** 2026-09-09
**Core Value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.

Milestones v1.0–v1.3 are shipped (v1.3 partial — Phase 17 deferred); their requirements are archived under `.planning/milestones/`. This file scopes **v1.4 Operator Observability**.

---

## v1.4 Requirements — Operator Observability

Make the scheduler observable without reading container logs, and let a deploy or uptime monitor tell "process up" from "ready to serve". Research: `.planning/research/SUMMARY.md` (+ STACK / FEATURES / ARCHITECTURE / PITFALLS).

### Readiness

- [x] **RDY-01**: `GET /ready` returns `200` when the database is reachable **and** the applied schema version is current (at or ahead of the binary's expected migration version) and not dirty; it returns `503` otherwise, with a minimal JSON body carrying no secrets or raw driver error text.
- [x] **RDY-02**: `/ready` is reachable unauthenticated at that exact path in both gate-configured and inert modes (mirroring `/health`), bounds its database check with a short timeout, queries the shared pool (no new connection), and has no side effects.
- [x] **RDY-03**: `/health` keeps its existing v1.3 behaviour and contract unchanged — readiness is a new, separate endpoint, not a change to liveness.

### Poll Run History

- [ ] **RUN-01**: Every poll-cycle invocation records exactly one run entry for its source, capturing: source, started/finished timestamps, artists checked, artists skipped, artists errored, events recorded, an outcome (`ok`, `cancelled`, `error`), and a short outcome summary that contains no DSN, webhook URL, internal path, or raw driver error string.
- [x] **RUN-02**: A cycle skipped by the overlap guard is recorded as a per-source signal (last-skipped timestamp + consecutive-skip count) surfaced in `/status`, not as a run entry; a cycle interrupted by shutdown still records its run entry with a `cancelled` outcome.
- [ ] **RUN-03**: The poller records runs through a seam and holds no database handle itself; a recorder failure is logged and never fails, blocks, or delays the poll cycle.
- [x] **RUN-04**: Run history is bounded to the last N entries per source (a compile-time constant, no new environment variable), correct under the two sources recording near-simultaneously.

> Storage note (see `docs/adr/0001-in-process-ring-buffer-for-poll-run-history.md`): run history lives in an in-process ring buffer, not a `poll_runs` table. RUN-01/03/04 are worded storage-agnostically; history resets on process restart by design.

### Status API

- [x] **STAT-01**: `GET /status`, behind the passphrase gate, returns JSON with: the last run per source, the last N runs, the current watchlist size, and the configured poll interval.
- [x] **STAT-02**: `/status` exposes counts, timestamps, and enum values only — never a DSN, webhook URL, internal path, or raw driver error string.

### System UI

- [ ] **SYS-01**: A "System" view, reachable from the SPA's main navigation, shows per source: the last run's time, outcome, duration, and counts, plus the time since the last successful run.
- [ ] **SYS-02**: The System view also shows a recent-runs history table, the watchlist size, the poll interval, and an about block (app version, schema version, database reachable).
- [ ] **SYS-03**: The view fetches on mount with a manual Refresh control and no fast auto-polling, and reuses the existing empty-state and `401` → passphrase-screen handling.

## Future Requirements

Deferred, tracked, not in the v1.4 roadmap.

### Observability

- **OBS-01**: Auto-refresh / live updates on the System view (polling or WebSocket) instead of manual Refresh
- **OBS-02**: A manual "poll now" trigger for a source from the System view
- **OBS-03**: Paginated / filterable poll-run history beyond the last N
- **OBS-04**: Poll-failure alerting (a Discord alert when a source errors for M consecutive cycles)
- ~~**OBS-05**: `events_recorded` sourced by widening the `EventRecorder` seam~~ — resolved into v1.4 scope (Phase 18.1): the seam is widened to `(int, error)`; the downstream-count alternative was rejected (clock-skew undercount)

### Operations / Deployment (carried from v1.3)

- **DPLY-01…08**: Automated VPS deploy with health-gated rollback (Phase 17) — deferred, blocked on a provisioned VPS + domain. The v1.4 `/ready` probe is the endpoint its deploy gate should poll.
- **OPS-04**: Automated, scheduled Postgres backups with off-box retention
- **OPS-05**: ghcr.io image-retention / cleanup job
- **GATE-08**: Session signing-key rotation without logging every browser out
- **CICD-15**: Patch/diff-level coverage in the PR comment

## Out of Scope

| Feature | Reason |
|---------|--------|
| Prometheus `/metrics` endpoint / Grafana stack | Deferred since v1.0; the milestone deliberately delivers a bespoke `/status` + UI instead, not a metrics pipeline |
| Time-series charts, sparklines, graphs on the System view | Adds a charting dependency for marginal value on a single-instance tool; the history table conveys the same information |
| Changing `/health` to a pure non-DB liveness probe | `/health` is a shipped contract with existing consumers (compose healthcheck, monitors); `/ready` is added alongside rather than repurposing it |
| A run-history retention **env var** | The milestone fixes N as a compile-time constant; an operator knob is unjustified for a bounded in-process buffer |
| A `poll_runs` database table | ADR-0001 — an in-process ring buffer avoids a migration plus the prune-on-insert and cross-source-deadlock hazards; cross-restart persistence is not needed for a single-instance tool |
| Recording per-artist detail entries | Per-artist outcomes already go to the correlated structured logs; a run entry is one summary per cycle |
| Distributed tracing / OpenTelemetry | Single process, single instance — no span graph to build |

## Traceability

Mapped during roadmap creation.

Phase 18 was split into **18** (readiness, status surface, app version — additive/low-risk) and **18.1** (poll-cycle instrumentation — the `runCycle` change) after a design grilling. See ROADMAP.md.

| Requirement | Phase | Status |
|-------------|-------|--------|
| RDY-01 | Phase 18 | Complete |
| RDY-02 | Phase 18 | Complete |
| RDY-03 | Phase 18 | Complete |
| RUN-01 | Phase 18.1 | Pending |
| RUN-02 | Phase 18 (skip signal) + Phase 18.1 (cancelled entry) | Complete |
| RUN-03 | Phase 18.1 | Pending |
| RUN-04 | Phase 18 | Complete |
| STAT-01 | Phase 18 | Complete |
| STAT-02 | Phase 18 | Complete |
| SYS-01 | Phase 19 | Pending |
| SYS-02 | Phase 19 | Pending |
| SYS-03 | Phase 19 | Pending |

**Coverage:**

- v1.4 requirements: 12 total
- Mapped to phases: 12
- Unmapped: 0

---
*Requirements defined: 2026-09-09*
