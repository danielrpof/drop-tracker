---
phase: 02
slug: watchlist-core
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-05
---

# Phase 02 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`), no third-party test framework |
| **Config file** | `Makefile` targets — no separate test-config file |
| **Quick run command** | `go test ./... -short -race -count=1` (skips DB-backed tests per `testutil.RequirePostgresDSN`'s `testing.Short()` check) |
| **Full suite command** | `make test` → `db-up` then `TEST_DATABASE_URL=... go test ./... -race -count=1` |
| **Estimated runtime** | ~30 seconds (short), ~90 seconds (full, includes DB migration + integration tests) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -short -race -count=1`
- **After every plan wave:** Run `make test` (full suite against real Postgres)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | 0 | WLST-02 | T-02-TBD | Duplicate add returns 409, not a 500 or silent overwrite | unit + integration | `go test ./internal/httpserver/... ./internal/watchlist/... -race -count=1` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | WLST-03 | T-02-TBD | Removing a missing id returns 404, not a 500 | unit + integration | `go test ./internal/httpserver/... ./internal/watchlist/... -race -count=1` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | WLST-04 | — | Joined list query never collapses `watchlist.id`/`artists.id` into one field | integration | `go test ./internal/httpserver/... ./internal/watchlist/... -race -count=1` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | WLST-05 | T-02-TBD | Invalid release-type value rejected at 400 (app layer) and by DB `CHECK` (backstop) | unit + integration | `go test ./internal/httpserver/... ./internal/watchlist/... -race -count=1` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | WLST-06 | T-02-TBD | Invalid mute-event value rejected at 400 (app layer) and by DB `CHECK` (backstop) | unit + integration | `go test ./internal/httpserver/... ./internal/watchlist/... -race -count=1` | ❌ Wave 0 | ⬜ pending |

*Task IDs and threat refs are finalized once PLAN.md and the `<threat_model>` blocks exist — the planner fills these in against this table's requirement rows.*

---

## Wave 0 Requirements

- [ ] `internal/db/migrations/000002_watchlist.up.sql` / `.down.sql` — new schema (artists, watchlist tables)
- [ ] `queries/artists.sql`, `queries/watchlist.sql` — new sqlc query sources
- [ ] `sqlc.yaml` edits (`emit_interface: true`, `emit_pointers_for_null_types: true`) + `sqlc generate` regeneration
- [ ] `internal/watchlist/service.go` + `service_test.go` — new package, no existing tests to build on
- [ ] `internal/httpserver/watchlist.go` + `watchlist_test.go` — new handlers, following `health.go`/`health_test.go` shape
- [ ] Updates to the 5 existing `httpserver.New(...)` call sites for the new constructor parameter (Research Pitfall 5: `cmd/server/main.go:80`, 4 calls in `health_test.go`, 1 in `boot_e2e_test.go:50`)

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification (no UI, no external API calls this phase).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
