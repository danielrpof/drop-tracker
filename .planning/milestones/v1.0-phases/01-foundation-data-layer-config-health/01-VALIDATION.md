---
phase: 01
slug: foundation-data-layer-config-health
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-04
---

# Phase 01 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `net/http/httptest` (per CLAUDE.md: httptest-based mocking, no live external calls in CI) |
| **Config file** | none — greenfield; Wave 0 creates the first `_test.go` files and shared fixtures |
| **Quick run command** | `go test ./... -short` |
| **Full suite command** | `go test ./... -race` |
| **Estimated runtime** | ~15-30 seconds (small greenfield suite; unit tests + a couple of Postgres-backed integration tests) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -short`
- **After every plan wave:** Run `go test ./... -race`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Task/Plan/Wave assignment is TBD until the planner creates PLAN.md — rows below are seeded from RESEARCH.md's Phase Requirements → Test Map and should be reconciled with real task IDs during planning.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD (health-up) | TBD | TBD | OPS-01 | — | `/health` returns 200 + `{"status":"ok","db":"up"}` when DB reachable | integration | `go test ./internal/httpserver/... -run TestHealth_Up` | ❌ W0 | ⬜ pending |
| TBD (health-down) | TBD | TBD | OPS-01 | — | `/health` returns 503 + `{"status":"degraded","db":"down"}` when DB unreachable | unit | `go test ./internal/httpserver/... -run TestHealth_Down` | ❌ W0 | ⬜ pending |
| TBD (request-id) | TBD | TBD | OPS-02 | — | Every response carries a correlating request ID in header + structured log output | unit | `go test ./internal/httpserver/... -run TestRequestID` | ❌ W0 | ⬜ pending |
| TBD (config-required) | TBD | TBD | OPS-03 | — | Missing required env var (`DATABASE_URL`) fails `Load()` with a clear listing, caller exits non-zero | unit | `go test ./internal/config/... -run TestLoad_MissingRequired` | ❌ W0 | ⬜ pending |
| TBD (env-example-parity) | TBD | TBD | OPS-03 | — | `.env.example` documents every `Config` struct field | static | `go test ./internal/config/... -run TestEnvExampleCompleteness` | ❌ W0 | ⬜ pending |
| TBD (migrate-retry) | TBD | TBD | D-09/D-10 | — | Migrations run automatically on boot; transient DB-unreachable retries with backoff, permanent failure exits after N attempts | integration | `go test ./internal/db/... -run TestRunMigrations` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/httpserver/health_test.go` — covers OPS-01 (both branches)
- [ ] `internal/httpserver/server_test.go` — covers OPS-02 (request-ID presence)
- [ ] `internal/config/config_test.go` — covers OPS-03 (fail-fast + `.env.example` parity)
- [ ] `internal/db/migrate_test.go` — covers D-09/D-10 retry/backoff behavior
- [ ] Test DB fixture/helper (`internal/testutil` or `TestMain`) — decide `testcontainers-go` vs. docker-compose/CI-service-container Postgres; this choice affects every later phase's DB-backed tests
- [ ] `go.mod`/`go.sum` — greenfield repo, Wave 0 deliverable alongside the module scaffold

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
