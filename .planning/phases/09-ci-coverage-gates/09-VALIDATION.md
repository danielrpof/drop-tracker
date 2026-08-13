---
phase: 09
slug: ci-coverage-gates
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-13
---

# Phase 09 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Backend: Go stdlib `testing` + `go tool cover` (no assertion library). Frontend: Vitest 4.1.10 + `@vitest/coverage-v8` (added this phase) |
| **Config file** | Backend: `Makefile` `test-integration` target (no separate coverage config file). Frontend: `web/vitest.config.ts` |
| **Quick run command** | Backend: `go test ./... -short -race -count=1` (no coverage — coverage only runs on the full `test-integration` path, per D-02). Frontend: `pnpm test` |
| **Full suite command** | Backend: `make test-integration` (extended this phase with `-coverprofile`/`-coverpkg`). Frontend: `pnpm test` |
| **Estimated runtime** | Not independently measured this phase — backend runs against a real Postgres instance (`-p 1`), frontend is the existing Vitest suite; both already run in CI today, coverage instrumentation adds negligible overhead |

---

## Sampling Rate

- **After every task commit:** Backend gap-closing tests via `go test ./... -short -race -count=1` (fast path, no coverage). Frontend via `pnpm test` (coverage collects once `@vitest/coverage-v8` is configured).
- **After every plan wave:** `make test-integration` (full backend suite + coverage) and `pnpm test` (full frontend suite + coverage) both green.
- **Before `/gsd-verify-work`:** Both coverage gates enforced in `full-pipeline.yml`; a locally-reproduced failing-then-passing transition (temporarily lower threshold, confirm red; restore to 80%/70%, confirm green) is the closest available test of the gate mechanism itself — CICD-11/12 are CI-infrastructure behaviors, not unit-testable application code.
- **Max feedback latency:** N/A — gated on the existing `test`/`frontend-test` CI job cadence, no new latency-sensitive path introduced.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| T1 (tracer) | 09-01 | 1 | CICD-11 | T-09-01, T-09-02 | Gate parses the profile, compares decimals, and fails closed on a missing/unparseable profile | CI gate mechanism (infra behavior) | `make test-integration` then `make coverage-gate COVERAGE_THRESHOLD_BACKEND=0` (green path) and `=100` (red path) | ❌ — new Makefile target this phase | ⬜ pending |
| T2 | 09-01 | 1 | CICD-11 | T-09-03 | Backend baseline measured and recorded before enforcement (ROADMAP SC4) | Recorded artifact | `test -s .planning/phases/09-ci-coverage-gates/09-BASELINE-BACKEND.md` | ❌ — new file this phase | ⬜ pending |
| T1 | 09-02 | 1 | CICD-12 | T-09-SC, T-09-06 | Coverage collected over an honest first-party denominator that includes untested files | Config behavior | `pnpm --dir web test` (coverage table lists `history.tsx` and `api.ts`) | ❌ — new `vitest.config.ts` coverage block this phase | ⬜ pending |
| T2 | 09-02 | 1 | CICD-12 | — | Frontend baseline measured and recorded before enforcement (ROADMAP SC4) | Recorded artifact | `test -s .planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md` | ❌ — new file this phase | ⬜ pending |
| T1 | 09-03 | 2 | CICD-11 | T-09-11, T-09-12, T-09-13 | Boot path fails fast on bad config and shuts down gracefully on context cancellation | Go integration test (real Postgres) | `go test ./cmd/server/ -race -count=1` | ❌ — new `cmd/server/main_test.go` | ⬜ pending |
| T2 | 09-03 | 2 | CICD-11 | T-09-09 | Log-level fallback and the embedded SPA handler's static-file branch are asserted | Go unit test | `go test ./internal/logging/ ./internal/webassets/ -race -count=1` | ❌ — two new `_test.go` files | ⬜ pending |
| T3 | 09-03 | 2 | CICD-11 | T-09-10 | Backend aggregate reaches 80% with the threshold and measured set unchanged | CI gate (infra behavior) | `make test-integration` then `make coverage-gate` (no override) | ❌ — depends on 09-01's target | ⬜ pending |
| T1 | 09-04 | 2 | CICD-12 | T-09-15, T-09-18 | History route's error/retry, append-with-dedupe, and two empty states are asserted | Vitest route test | `pnpm --dir web exec vitest run app/routes/history.test.tsx` | ❌ — new `history.test.tsx` | ⬜ pending |
| T2 | 09-04 | 2 | CICD-12 | T-09-14 | Shared fetch path's typed-error, status-text-fallback, no-content, and success branches are asserted | Vitest unit test | `pnpm --dir web exec vitest run app/lib/api.test.ts` | ❌ — new `api.test.ts` | ⬜ pending |
| T3 | 09-04 | 2 | CICD-12 | T-09-16, T-09-17 | `pnpm test` exits non-zero below 70% on any of the four axes, with the denominator unchanged | CI gate via Vitest built-in threshold | `pnpm --dir web test` (and raise all four axes to 100 to observe red) | ❌ — new `thresholds` block | ⬜ pending |
| T1 | 09-05 | 3 | CICD-11 | T-09-20, T-09-21, T-09-23 | `test` job runs the gate with no escape hatch, on a measured timeout budget | Workflow structure assertion | `python -c "import yaml; ..."` on `jobs.test.steps` (see plan) | ❌ — new CI step | ⬜ pending |
| T2 | 09-05 | 3 | CICD-11, CICD-12 | T-09-19, T-09-24 | `build-scan` requires all six upstream jobs; `release` still requires only `build-scan` | Workflow structure assertion + end-of-phase human check | `python -c "import yaml; ..."` on `jobs.build-scan.needs` (see plan) | ❌ — one added `needs` entry | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky. Reconciled against the five PLAN.md files created 2026-08-13. Threat refs point at each plan's own `<threat_model>` register.*

---

## Wave 0 Requirements

All Wave 0 gaps are owned by wave-1 plans 09-01 and 09-02, which run before any task depends on them:

- [ ] `Makefile` — add `COVER_PKGS` variable + `-coverprofile`/`-coverpkg` flags to `test-integration`, plus a `coverage-gate` target → **09-01 Task 1**
- [ ] `web/vitest.config.ts` — add `coverage` block (`provider: 'v8'`, `include`, `exclude`) → **09-02 Task 1**; the `thresholds` key is deliberately deferred to **09-04 Task 3** so the baseline is recorded before enforcement
- [ ] `web/package.json` — add `@vitest/coverage-v8@4.1.10` devDependency (exact-version-matched to pinned `vitest@4.1.10`) → **09-02 Task 1**
- [ ] Baseline measurement for both sides, recorded BEFORE the threshold is committed as an enforced gate (CONTEXT.md success criterion 4) → **09-01 Task 2** (`09-BASELINE-BACKEND.md`) and **09-02 Task 2** (`09-BASELINE-FRONTEND.md`)
- [ ] Confirm `coverage.include` is explicit on the frontend — Vitest 4 removed the measure-everything option, so an unlisted file (e.g. `app/routes/history.tsx`, currently 0% covered) would silently never enter the report, producing a passing gate that measures almost nothing → **09-02 Task 1**, asserted by requiring `history.tsx` and `api.ts` rows in the produced coverage table

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| A coverage failure on either side blocks `build-scan`/`release` | CICD-11, CICD-12 | `needs:` graph behavior — not exercised by a unit test | Temporarily lower a threshold below the measured baseline, push, confirm the job goes red and `build-scan` does not run; then restore the threshold to 80%/70% |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < N/A (CI-cadence gated, no new latency path)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
