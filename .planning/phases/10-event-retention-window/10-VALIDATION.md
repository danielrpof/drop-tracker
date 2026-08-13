---
phase: 10
slug: event-retention-window
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-13
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Backend: Go stdlib `testing` + real-Postgres integration tests via `testutil.NewTestPool`. Frontend: Vitest + React Testing Library |
| **Config file** | `Makefile` (test targets), `sqlc.yaml`; Vitest config implicit in `web/` (pre-existing, proven working per Phase 8/9) |
| **Quick run command** | Backend: `go test ./internal/config/... ./internal/events/... -short -race -count=1`. Frontend: `cd web && pnpm run test -- history.test.tsx` |
| **Full suite command** | Backend: `make test-integration` (requires `make db-up` first). Frontend: `cd web && pnpm run test` |
| **Estimated runtime** | Not independently measured this phase — existing suite, both already run in CI today; retention filter adds one indexed-comparable predicate, negligible overhead |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/config/... ./internal/events/... -short -race -count=1` (backend) and `cd web && pnpm run test -- history.test.tsx` (frontend) for touched packages
- **After every plan wave:** `make test-integration` (full backend suite against real Postgres) + `cd web && pnpm run test` (full frontend suite)
- **Before `/gsd-verify-work`:** Full suite green (`make test-integration` + frontend `pnpm run test`)
- **Max feedback latency:** Not latency-sensitive — no new async/streaming path introduced

---

## Per-Task Verification Map

*Task IDs not yet assigned — plans do not exist yet (VALIDATION.md is seeded before planning). Rows below are keyed by requirement per RESEARCH.md's Phase Requirements → Test Map; reconcile Task/Plan/Wave columns once PLAN.md files exist.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | DATA-01 | — | `EVENT_RETENTION_DAYS` parses, defaults to 90, fails fast on `<= 0` | unit | `go test ./internal/config/... -run TestLoad -short` | ✅ `internal/config/config_test.go` exists — extend with new cases | ⬜ pending |
| TBD | TBD | TBD | DATA-01 | — | `.env.example` documents the new key | unit | `go test ./internal/config/... -run TestEnvExampleCompleteness -short` | ✅ existing test, no new file | ⬜ pending |
| TBD | TBD | TBD | DATA-02 | — | `ListEvents` excludes rows with `created_at < cutoff`, includes rows exactly at cutoff (D-04 inclusive boundary) | integration (real Postgres) | `make test-integration` then `go test ./internal/httpserver/... -run TestListEvents -race -count=1 -p 1` | ✅ `internal/httpserver/events_test.go` exists — extend with retention-boundary cases | ⬜ pending |
| TBD | TBD | TBD | DATA-02 | — | `ListExternalIDs`/`HasAnyEvent`/`GroupTrackCountBaseline`/`ListUnnotified` remain unfiltered (proves success criteria 3-5: no re-notify, no seed-mode reset, no baseline loss) | integration | New test seeding an aged-out row and calling the four untouched queries directly | ❌ Wave 0 — no existing test asserts this cross-query invariant | ⬜ pending |
| TBD | TBD | TBD | DATA-02 | — | `GET /events` `has_older_events` signal is correct in all three states (no events ever / all filtered by user filter / all filtered by retention) | integration + component | Backend: extend `TestHandleListEvents_*`. Frontend: extend `web/app/routes/history.test.tsx` | ❌ Wave 0 for both — new response field, new UI branch | ⬜ pending |
| TBD | TBD | TBD | DATA-02 | — | History UI shows the correct one of three empty-state messages | component (Vitest + RTL) | `cd web && pnpm run test -- history.test.tsx` | ✅ `web/app/routes/history.test.tsx` exists — extend with the retention-empty-state case | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] New integration test in `internal/httpserver/events_test.go` proving `ListExternalIDs`/`HasAnyEvent`/`GroupTrackCountBaseline`/`ListUnnotified` return an aged-out row's data unfiltered — direct automated proof of success criteria 3-5
- [ ] New unit test cases in `internal/config/config_test.go` for `EVENT_RETENTION_DAYS` default/override/`<=0`-rejection, mirroring the existing `DatabaseURL` fail-fast tests
- [ ] New boundary test in `internal/httpserver/events_test.go` asserting `created_at` exactly at the cutoff is included (D-04's inclusive-boundary rule)
- [ ] New Vitest case in `web/app/routes/history.test.tsx` for the third (retention) empty-state message, keyed off the new `has_older_events`-style API field

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < N/A (no latency-sensitive path introduced)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
