# Requirements: drop-tracker

**Defined:** 2026-09-09
**Core Value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.

Milestones v1.0–v1.3 are shipped (v1.3 partial — Phase 17 deferred); their requirements are archived under `.planning/milestones/`. This file scopes **v1.4 Operator Observability**.

---

## v1.4 Requirements — Operator Observability

Make the scheduler observable without reading container logs, and let a deploy or uptime monitor tell "process up" from "ready to serve". Research: `.planning/research/SUMMARY.md` (+ STACK / FEATURES / ARCHITECTURE / PITFALLS).

### Readiness

- [ ] **RDY-01**: `GET /ready` returns `200` when the database is reachable **and** the applied schema version is current (at or ahead of the binary's expected migration version) and not dirty; it returns `503` otherwise, with a minimal JSON body carrying no secrets or raw driver error text.
- [ ] **RDY-02**: `/ready` is reachable unauthenticated at that exact path in both gate-configured and inert modes (mirroring `/health`), bounds its database check with a short timeout, queries the shared pool (no new connection), and has no side effects.
- [ ] **RDY-03**: `/health` keeps its existing v1.3 behaviour and contract unchanged — readiness is a new, separate endpoint, not a change to liveness.

### Poll Run History

- [ ] **RUN-01**: Every poll-cycle invocation records exactly one `poll_runs` row for its source, capturing: source, started/finished timestamps, artists checked, artists errored, events recorded, an outcome (`ok`, `skipped_overlap`, `cancelled`, `error`), and a short outcome summary that contains no DSN, webhook URL, internal path, or raw driver error string.
- [ ] **RUN-02**: A cycle skipped by the overlap guard records a `skipped_overlap` row; a cycle interrupted by shutdown still records its row with a `cancelled` outcome.
- [ ] **RUN-03**: The poller records runs through a seam and holds no database handle itself; a recorder failure is logged and never fails, blocks, or delays the poll cycle.
- [ ] **RUN-04**: `poll_runs` is bounded — recording a run also prunes that source's history to the last N rows (a compile-time constant, no new environment variable), correct under the two sources recording near-simultaneously.

### Status API

- [ ] **STAT-01**: `GET /status`, behind the passphrase gate, returns JSON with: the last run per source, the last N runs, the current watchlist size, and the configured poll interval.
- [ ] **STAT-02**: `/status` exposes counts, timestamps, and enum values only — never a DSN, webhook URL, internal path, or raw driver error string.

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
- **OBS-05**: `events_recorded` sourced by widening the `EventRecorder` seam to `(int, error)`, if v1.4 ships the downstream-count approach instead

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
| A `poll_runs` retention **env var** | The milestone fixes N as a constant; an operator knob is unjustified for an internal bounded table |
| Recording per-artist detail rows in `poll_runs` | Per-artist outcomes already go to the correlated structured logs; the table is one summary row per cycle |
| Distributed tracing / OpenTelemetry | Single process, single instance — no span graph to build |

## Traceability

Mapped during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| RDY-01 | Phase 18 | Pending |
| RDY-02 | Phase 18 | Pending |
| RDY-03 | Phase 18 | Pending |
| RUN-01 | Phase 18 | Pending |
| RUN-02 | Phase 18 | Pending |
| RUN-03 | Phase 18 | Pending |
| RUN-04 | Phase 18 | Pending |
| STAT-01 | Phase 18 | Pending |
| STAT-02 | Phase 18 | Pending |
| SYS-01 | Phase 19 | Pending |
| SYS-02 | Phase 19 | Pending |
| SYS-03 | Phase 19 | Pending |

**Coverage:**

- v1.4 requirements: 12 total
- Mapped to phases: 12
- Unmapped: 0

---
*Requirements defined: 2026-09-09*
