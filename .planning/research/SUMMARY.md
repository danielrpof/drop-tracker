# Research Summary: v1.4 Operator Observability

**Project:** drop-tracker  
**Milestone:** v1.4 — Operator Observability  
**Domain:** Observability endpoints and persistence layer for a Go single-binary background-polling service  
**Researched:** 2026-09-09  
**Confidence:** HIGH

---

## Executive Summary

v1.4 adds four lightweight observability features: a readiness probe (`/ready`), poll_runs persistence with RunRecorder seam, gated `GET /status` endpoint, and React System view. **No new dependencies needed** — entire milestone expressible in existing stack (Go, chi, sqlc, React, Postgres).

**Recommended approach:** Two-phase split. **Phase 18 (Backend)** wires poll_runs + RunRecorder into poller cycle machinery, adds /ready as unauthenticated readiness probe reusing golang-migrate version-walk, and exposes gated /status JSON endpoint. **Phase 19 (UI)** renders System view by fetching /status through existing auth layer.

**Highest architectural risk:** Counter aggregation across worker goroutines in runCycle without -race detection available in CI. Fix is straightforward (sync/atomic counters) but critical: plain int shared between goroutines is undefined behavior.

**Three cross-cutting decisions flagged for phase planners to lock:**
1. **/ready ready-condition:** strict db.version == expectedMax (safest) vs db.version >= expectedMax (correct for Phase 16 ahead-of-source guard). PITFALLS argues >= because rolled-back binary should report ready, else Phase 17 deploy gate will flap.
2. **events_recorded source:** widen EventRecorder.Detect* to return (int, error) (cleaner, minimal poller touch) vs compute downstream in pollruns.Recorder via count-after-insert subquery (avoids seam change).
3. **Retention count N:** value unspecified; research recommends 50–100 per source as compile-time constant.

---

## Key Findings

### Recommended Stack

**No new dependencies required.** All v1.4 features covered by existing go.mod/package.json plus stdlib.

**Core technologies:**
- chi/v5 (v5.3.1) — /ready (public exact-path) and /status (gated group)
- golang-migrate/v4 (v4.19.1) — maxSourceVersion walk + m.Version() for schema check
- pgx/v5 (v5.10.0) — schema_migrations read; poll_runs queries
- sqlc (v1.31.1) — codegen for poll_runs INSERT/SELECT/prune
- sync/atomic (stdlib) — counters across worker goroutines
- React 19.2.6 + Vite 6.x + react-router 7.18.2 — System route + nav + apiFetch
- Vitest 4.1.10 + RTL 16.3.2 — existing test harness; 70% coverage gate

### Expected Features

**Table stakes:**
1. GET /ready — public, unauthenticated, DB-reachable + schema-at-expected-version, returns 200/503
2. poll_runs table — one row per completed poll cycle per source: timing + outcome enum (ok/error/cancelled) + work counters
3. RunRecorder seam — narrow interface in internal/poller, wired into runCycle via functional option
4. GET /status JSON — gated, returns per-source last run + recent history + watchlist count + poll interval
5. React System view — new SPA route + nav tab, mirrors existing History pattern

**Explicitly deferred:** Prometheus metrics, time-series charts, poll-failure alerting, manual "poll now", live-updating dashboard.

### Architecture Approach

Features attach via consumer-declared narrow seams (established pattern). New components:
- db.ExpectedSchemaVersion() — exported helper walks embedded migrationsFS once at boot
- httpserver.ReadinessChecker interface — reads schema_migrations via shared pool
- poller.RunRecorder interface — new consumer seam RecordRun(ctx, RunResult)
- internal/pollruns.Recorder — sqlc-backed, INSERT + prune in transaction, keeps N rows per source
- httpserver.StatusStore interface — returns recent runs grouped by source
- poll_runs migration — 000008_poll_runs.up.sql, purely additive, index on (source, started_at DESC)

### Critical Pitfalls (Top 5)

1. **Counter aggregation data race — -race unavailable in CI** (CRITICAL)
   - Plain int shared between goroutines undefined behavior
   - Fix: sync/atomic.Int64, looped exact-assertion test

2. **Prune-on-insert races between two sources** (CRITICAL)
   - Missing source scope or wrong lock ordering causes cross-source deletion/deadlock
   - Fix: Atomic CTE with INSERT+DELETE, source-scoped, keyset cutoff

3. **RunRecorder failure or hang** (CRITICAL)
   - Propagating error or hanging blocks overlap guard
   - Fix: Log-not-return error, bounded context with WithoutCancel+5s

4. **Recording row for overlap-skipped cycle** (CRITICAL)
   - ErrCycleInProgress ticks write junk rows, evicting real history
   - Fix: Defer inside runCycle after CAS, never for skipped cycles

5. **/ready version == breaks rollback** (HIGH)
   - Rolled-back instances report not-ready forever; Phase 17 flap-loops
   - Fix: Use db.version >= expectedMax && !dirty, document Phase 16 rationale

---

## Implications for Roadmap

### Phase 18 — Backend (four ordered sub-phases)

**18.1: GET /ready readiness probe**
- Rationale: Independent value, low risk, unblocks Phase 17
- Delivers: Public probe checking DB + schema version

**18.2: poll_runs migration + sqlc codegen**
- Rationale: Foundation for downstream
- Delivers: poll_runs table + type-safe queries

**18.3: RunRecorder seam + runCycle instrumentation**
- Rationale: Riskiest change, focused review; runCycle is shared machinery
- Delivers: Poll-cycle summary rows; poller stays DB-free
- **Research flags:** HIGH — explicit concurrency reasoning; looped counter invariant test required

**18.4: StatusStore + gated GET /status**
- Rationale: Thin layer; freezes contract for Phase 19
- Delivers: Gated JSON endpoint with per-source runs + history + watchlist

### Phase 19 — Frontend: System View

- Rationale: Depends on Phase 18.4; UI-only; mirrors History pattern
- Delivers: System view rendering /status with status cards + recent-run table

### Cross-Cutting Decisions

1. **/ready ready-condition:** == vs >= — **Recommendation: >= (Phase 16 safe rollback scenario)**
2. **events_recorded source:** seam widening vs downstream — **Either valid; seam widening cleaner**
3. **Retention count N:** 50-100 per source — **No strong signal; 50 conservative, 100 generous**

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|----------|-------|
| Stack | HIGH | All dependencies verified; no new imports |
| Features | HIGH | Clearly scoped; industry-standard patterns |
| Architecture | HIGH | Integration traced to source; patterns established |
| Pitfalls | HIGH | Grounded in codebase mechanics |

**Overall: HIGH**

---

*Research synthesis completed: 2026-09-09*  
*Ready for roadmap: yes*
