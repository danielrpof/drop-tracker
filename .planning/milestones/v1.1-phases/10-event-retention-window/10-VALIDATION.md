---
phase: 10
slug: event-retention-window
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-13
reconciled: 2026-08-17
reconciled_note: >
  This file was seeded pre-planning with requirement-keyed placeholder rows (no task IDs, no
  plan/wave columns). Reconciled post-execution against the two real PLAN.md files and
  10-VERIFICATION.md (2026-08-13, status: passed, 5/5 truths, zero gaps, no human-verification
  items -- the cleanest of the three v1.1 phases to reconcile). Performed as part of Phase
  11.1's D-09 cleanup item.
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

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 10-01-T1 | 01 | 1 | DATA-01 | — | `EVENT_RETENTION_DAYS` parses, defaults to 90, fails fast on `<= 0` | unit | `go test ./internal/config/... -run "TestLoad_EventRetentionDays" -v` | ✅ | ✅ green |
| 10-01-T2 | 01 | 1 | DATA-01 | — | `.env.example` documents the new key | unit | `go test ./internal/config/... -run TestEnvExampleCompleteness -short` | ✅ | ✅ green |
| 10-01-T3 | 01 | 1 | DATA-02 | — | `ListEvents` excludes rows with `created_at < cutoff`, includes rows exactly at cutoff (D-04 inclusive boundary), pagination never repeats an id across the boundary | integration (real Postgres) | `go test ./internal/httpserver/... -run "TestListEvents_Retention" -race -count=1 -p 1` | ✅ | ✅ green |
| 10-01-T4 | 01 | 1 | DATA-02 | — | `ListExternalIDs`/`HasAnyEvent`/`GroupTrackCountBaseline`/`ListUnnotified` remain unfiltered (proves success criteria 3-5: dedup key intact, no seed-mode reset, deluxe baseline survives) | integration | `go test ./internal/httpserver/... -run TestRetention_DetectionStateQueriesStayUnfiltered -v` (5 subtests) | ✅ | ✅ green |
| 10-02-T1 | 02 | 2 | DATA-02 | — | `GET /events` `has_older_events` signal is correct in all three states (no events ever / all filtered by user filter / all filtered by retention), and respects `artist_id` filter scope | integration | `go test ./internal/httpserver/... -run TestHandleListEvents_HasOlderEventsSignal -v` (3 subtests) + `TestListEvents_HasOlderEventsRespectsFilters` | ✅ | ✅ green |
| 10-02-T2 | 02 | 2 | DATA-02 | — | History UI shows the correct one of three empty-state messages, retention branch checked before the user-filter branch | component (Vitest + RTL) | `cd web && pnpm run test -- history.test.tsx api.test.ts` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Reconciled 2026-08-17 against the two real PLAN.md files and 10-VERIFICATION.md's Behavioral Spot-Checks (all executed live against a running Postgres 16 container, not read from SUMMARY claims).*

---

## Wave 0 Requirements

- [x] `internal/httpserver/events_test.go` — `TestRetention_DetectionStateQueriesStayUnfiltered` proves `ListExternalIDs`/`HasAnyEvent`/`GroupTrackCountBaseline`/`ListUnnotified` return an aged-out row's data unfiltered (5 subtests, direct proof of success criteria 3-5) → **10-01**
- [x] `internal/config/config_test.go` — `TestLoad_EventRetentionDays{DefaultsTo90,Override,RejectsNonPositive}` mirror the existing `DatabaseURL` fail-fast tests → **10-01**
- [x] `internal/httpserver/events_test.go` — `TestListEvents_RetentionBoundaryIsInclusive` asserts `created_at` exactly at the cutoff is included (D-04) → **10-01**
- [x] `web/app/routes/history.test.tsx` — third (retention) empty-state case, keyed off `has_older_events` → **10-02**

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification, confirmed live during 10-VERIFICATION.md's session (no backstop-tier items, unlike Phase 9's live-CI-run item).*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < N/A (no latency-sensitive path introduced)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** reconciled 2026-08-17 against 10-VERIFICATION.md (5/5 truths, zero gaps, all checks executed live) — no outstanding items.

---

## Validation Audit 2026-08-17

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 (none needed — 10-VERIFICATION.md already confirmed full coverage with zero human-verification items) |
| Escalated | 0 |
| Placeholder rows replaced with real task IDs/commits | 6 |
