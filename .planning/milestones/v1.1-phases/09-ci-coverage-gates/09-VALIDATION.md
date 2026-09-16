---
phase: 09
slug: ci-coverage-gates
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-13
reconciled: 2026-08-17
reconciled_note: >
  This file already carried real task IDs/commands (reconciled against the five PLAN.md files
  at creation), but every row was left at status: pending and nyquist_compliant: false because
  no pass ever flipped them after execution. 09-VERIFICATION.md (2026-08-13, amended, status:
  human_needed -> functionally passed, 5/5 truths, both gates independently re-executed and
  confirmed green) supplies the evidence for every automated row below. The one Manual-Only
  item (live CI run proving the coverage gate blocks build-scan) was outstanding in
  09-VERIFICATION.md's own body but is independently confirmed complete by STATE.md's
  `[Phase 09 UAT, closed 2026-08-13]` entry (three real GitHub Actions runs on the
  since-deleted scratch branch `test/coverage-gate-ci-check`). Performed as part of Phase
  11.1's D-09 cleanup item.
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
| T1 (tracer) | 09-01 | 1 | CICD-11 | T-09-01, T-09-02 | Gate parses the profile, compares decimals, and fails closed on a missing/unparseable profile | CI gate mechanism (infra behavior) | `make test-integration` then `make coverage-gate COVERAGE_THRESHOLD_BACKEND=0` (green path) and `=100` (red path) | ✅ | ✅ green |
| T2 | 09-01 | 1 | CICD-11 | T-09-03 | Backend baseline measured and recorded before enforcement (ROADMAP SC4) | Recorded artifact | `test -s .planning/phases/09-ci-coverage-gates/09-BASELINE-BACKEND.md` | ✅ | ✅ green |
| T1 | 09-02 | 1 | CICD-12 | T-09-SC, T-09-06 | Coverage collected over an honest first-party denominator that includes untested files | Config behavior | `pnpm --dir web test` (coverage table lists `history.tsx` and `api.ts`) | ✅ | ✅ green |
| T2 | 09-02 | 1 | CICD-12 | — | Frontend baseline measured and recorded before enforcement (ROADMAP SC4) | Recorded artifact | `test -s .planning/phases/09-ci-coverage-gates/09-BASELINE-FRONTEND.md` | ✅ | ✅ green |
| T1 | 09-03 | 2 | CICD-11 | T-09-11, T-09-12, T-09-13 | Boot path fails fast on bad config and shuts down gracefully on context cancellation | Go integration test (real Postgres) | `go test ./cmd/server/ -race -count=1` | ✅ | ✅ green |
| T2 | 09-03 | 2 | CICD-11 | T-09-09 | Log-level fallback and the embedded SPA handler's static-file branch are asserted | Go unit test | `go test ./internal/logging/ ./internal/webassets/ -race -count=1` | ✅ | ✅ green |
| T3 | 09-03 | 2 | CICD-11 | T-09-10 | Backend aggregate reaches 80% with the threshold and measured set unchanged | CI gate (infra behavior) | `make test-integration` then `make coverage-gate` (no override) | ✅ | ✅ green |
| T1 | 09-04 | 2 | CICD-12 | T-09-15, T-09-18 | History route's error/retry, append-with-dedupe, and two empty states are asserted | Vitest route test | `pnpm --dir web exec vitest run app/routes/history.test.tsx` | ✅ | ✅ green |
| T2 | 09-04 | 2 | CICD-12 | T-09-14 | Shared fetch path's typed-error, status-text-fallback, no-content, and success branches are asserted | Vitest unit test | `pnpm --dir web exec vitest run app/lib/api.test.ts` | ✅ | ✅ green |
| T3 | 09-04 | 2 | CICD-12 | T-09-16, T-09-17 | `pnpm test` exits non-zero below 70% on any of the four axes, with the denominator unchanged | CI gate via Vitest built-in threshold | `pnpm --dir web test` (and raise all four axes to 100 to observe red) | ✅ | ✅ green |
| T1 | 09-05 | 3 | CICD-11 | T-09-20, T-09-21, T-09-23 | `test` job runs the gate with no escape hatch, on a measured timeout budget | Workflow structure assertion | `python -c "import yaml; ..."` on `jobs.test.steps` (see plan) | ✅ | ✅ green |
| T2 | 09-05 | 3 | CICD-11, CICD-12 | T-09-19, T-09-24 | `build-scan` requires all six upstream jobs; `release` still requires only `build-scan` | Workflow structure assertion + end-of-phase human check | `python -c "import yaml; ..."` on `jobs.build-scan.needs` (see plan) | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky. Reconciled against the five PLAN.md files created 2026-08-13. Threat refs point at each plan's own `<threat_model>` register.*

---

## Wave 0 Requirements

All Wave 0 gaps are owned by wave-1 plans 09-01 and 09-02, which run before any task depends on them:

- [x] `Makefile` — `COVER_PKGS` variable + `-coverprofile`/`-coverpkg` flags on `test-integration`, plus `coverage-gate` target → **09-01 Task 1**
- [x] `web/vitest.config.ts` — `coverage` block (`provider: 'v8'`, `include`, `exclude`) → **09-02 Task 1**; `thresholds` added in **09-04 Task 3** after the baseline was recorded
- [x] `web/package.json` — `@vitest/coverage-v8@4.1.10` devDependency, exact-version-matched to pinned `vitest@4.1.10` → **09-02 Task 1**
- [x] Baseline measurement for both sides, recorded before enforcement → `09-BASELINE-BACKEND.md` (starting 83.5%, closing 87.1%) and `09-BASELINE-FRONTEND.md` (starting 39.77/25.26/38.38/41.29, closing 78.06/71.57/75.75/79.75)
- [x] `coverage.include` explicit on the frontend, confirmed live in `web/vitest.config.ts` — `history.tsx` and `api.ts` rows present in the coverage table → **09-02 Task 1**

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Resolution |
|----------|-------------|------------|-------------|
| A coverage failure on either side blocks `build-scan`/`release` | CICD-11, CICD-12 | `needs:` graph behavior — not exercised by a unit test; 09-VERIFICATION.md (2026-08-13) explicitly routed this to Human Verification as a backstop-tier truth requiring a real GitHub Actions run | **Resolved 2026-08-13.** Confirmed via three real GitHub Actions runs (31724487315, 31724744670, 31724954534) on the scratch branch `test/coverage-gate-ci-check`: an under-threshold push on each side turned the corresponding job red with `build-scan` reported skipped, and restoring both thresholds to 80%/70% reached `build-scan` normally. Recorded in STATE.md's `[Phase 09 UAT, closed 2026-08-13]` entry and cross-referenced in `.planning/v1.1-MILESTONE-AUDIT.md`'s amendment note. Scratch branch deleted afterward; run IDs remain independently checkable against GitHub's own history. |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < N/A (CI-cadence gated, no new latency path)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** reconciled 2026-08-17 against 09-VERIFICATION.md (5/5 truths, both gates independently re-executed and confirmed green) plus STATE.md's 2026-08-13 live-CI-run closure evidence for the one Manual-Only item.

---

## Validation Audit 2026-08-17

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 (none needed — 09-VERIFICATION.md already confirmed functional coverage; this pass only flipped pending rows to green using that evidence, and closed the one outstanding Manual-Only item using STATE.md's 2026-08-13 CI-run record) |
| Escalated | 0 |
| Rows flipped from pending to green | 12 |
