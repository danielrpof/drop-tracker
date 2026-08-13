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
| TBD | TBD | TBD | CICD-11 | — | Backend `test` job fails when aggregate Go coverage < 80% | CI gate (infra behavior, not unit test) | `make test-integration` + coverage-gate step | ❌ — new Makefile target/CI step this phase | ⬜ pending |
| TBD | TBD | TBD | CICD-12 | — | `frontend-test` job fails when aggregate frontend coverage < 70% | CI gate via Vitest built-in threshold | `pnpm test` (unchanged script, D-08) | ❌ — new `vitest.config.ts` coverage block this phase | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky. Task/Plan/Wave IDs are TBD — this document is seeded before PLAN.md exists (plan-phase §5.5 runs before the planner); the planner should reconcile actual task IDs into this map where practical.*

---

## Wave 0 Requirements

- [ ] `Makefile` — add `COVER_PKGS` variable + `-coverprofile`/`-coverpkg` flags to `test-integration`, plus a `coverage-gate` (or equivalently-named) target
- [ ] `web/vitest.config.ts` — add `coverage` block (`provider: 'v8'`, `include`, `exclude`, `thresholds`)
- [ ] `web/package.json` — add `@vitest/coverage-v8@4.1.10` devDependency (exact-version-matched to pinned `vitest@4.1.10`)
- [ ] Baseline measurement for both sides, recorded BEFORE the threshold is committed as an enforced gate (CONTEXT.md success criterion 4)
- [ ] Confirm `coverage.include` is explicit on the frontend — Vitest 4 removed `coverage.all`, so an unlisted file (e.g. `app/routes/history.tsx`, currently 0% covered) would silently never enter the report, producing a passing gate that measures almost nothing

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
