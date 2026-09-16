---
phase: 07
slug: containerization-ci-cd-pipeline
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-12
---

# Phase 07 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (existing) — this phase adds no new test framework; its "tests" are largely the pipeline's own behavior, exercisable only by a real GitHub Actions run |
| **Config file** | none — `.github/workflows/full-pipeline.yml` (new) is the artifact under test |
| **Quick run command** | `make test-short` |
| **Full suite command** | `make test-integration` |
| **Estimated runtime** | ~30s quick (mirrors prior phases); full integration suite unverified this phase — CI's own run time is the real unknown until the pipeline exists |

---

## Sampling Rate

- **After every task commit:** `make test-short`
- **After every plan wave:** `make test-integration`
- **Before `/gsd-verify-work`:** Full suite green, PLUS a real PR opened against this branch (exercises the full `checks` + PR-time `build-scan-push` path) and, once merged, a real push to `main` (exercises the version/SBOM/push path) — both required before CICD-01 through CICD-10 can be honestly closed
- **Max feedback latency:** 30 seconds (local); CI-gated requirements (see honest caveat below) are latency-bound by GitHub Actions run time, not local sampling

---

## Per-Task Verification Map

*Task IDs are `07-TBD` pre-planning placeholders — the planner maps these to real `07-{plan}-T{task}` IDs when PLAN.md files are created.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 07-TBD | TBD | TBD | CICD-01 | T-07-* | lint/vet/test run on every push, before build/publish | CI smoke (real push/PR) | `golangci-lint run`, `go vet ./...`, `make test-integration` individually pass today; gate *ordering* only provable by a real workflow run | ❌ Wave 0 — `.golangci.yml` doesn't exist yet | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-02 | T-07-* | gitleaks blocks pipeline on detected secret | CI smoke + local pre-commit (already proven, quick/260806-hfn) | `pre-commit run gitleaks --all-files` (local); CI half needs a real push | ✅ pre-commit half proven; ❌ CI workflow file itself | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-03 | T-07-* | multi-stage image builds, non-root, contains full stack | Manual/smoke | `docker build -t drop-tracker:test . && docker run --rm drop-tracker:test whoami` | ❌ Wave 0 — Dockerfile doesn't exist | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-04 | T-07-* | Trivy blocks on CRITICAL/HIGH | CI smoke | `trivy image --severity CRITICAL,HIGH --exit-code 1 drop-tracker:test` (runnable locally with Trivy CLI, or via the CI step itself) | ❌ Wave 0 | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-05 | — | SBOM generated for built image | CI smoke | `syft ghcr.io/danielrpof/drop-tracker:vX.Y.Z -o spdx-json` (post-push verification) | ❌ Wave 0 | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-06 | — | semver computed + tagged on merge to main | CI smoke (requires real merge) | `svu next` (locally runnable against real git history to sanity-check) | ❌ Wave 0 | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-07 | T-07-* | image pushed to ghcr.io tagged with semver | CI smoke + manual pull | `docker pull ghcr.io/danielrpof/drop-tracker:vX.Y.Z` post-merge | ❌ Wave 0 | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-08 | T-07-* | all security-sensitive Actions pinned to commit SHA, not tag | Static/grep check | `grep -n "uses: .*@[0-9a-f]\{40\}" .github/workflows/*.yml` matches every third-party `uses:` line; any bare-tag `uses:` fails this grep | ❌ Wave 0 | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-09 | — | docker-compose brings up app + postgres, app reaches DB | Manual/smoke | `docker compose up --wait && curl -f http://localhost:8080/health` | ❌ Wave 0 — `app:` service doesn't exist yet | ⬜ pending |
| 07-TBD | TBD | TBD | CICD-10 | T-07-* | pre-commit runs golangci-lint + gitleaks locally before commit | Local hook run | `pre-commit run --all-files` | ✅ gitleaks half proven (quick/260806-hfn); ❌ golangci-lint hook entry | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Honest caveat (from 07-RESEARCH.md):** Unlike application-behavior requirements in prior phases, most of CICD-01 through CICD-10 describe *pipeline* behavior that genuinely cannot be unit-tested — the only real verification is triggering an actual GitHub Actions run (a PR, then a merge to main) and observing the checks. The plan should budget an explicit "first real PR" and "first real merge to main" verification step rather than expecting `go test` to prove these requirements.

---

## Wave 0 Requirements

- [ ] `.golangci.yml` (v2 schema) — covers CICD-01, does not exist yet
- [ ] `Dockerfile` + `.dockerignore` — covers CICD-03, greenfield
- [ ] `.github/workflows/full-pipeline.yml` — covers CICD-01/02/04/05/06/07/08, greenfield
- [ ] `docker-compose.yml` `app:` service — covers CICD-09
- [ ] `.pre-commit-config.yaml` golangci-lint hook entry — covers CICD-10 (second half; gitleaks half already done)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| First real PR exercises lint/vet/test/gitleaks/PR-title checks + PR-time build-scan (no push) | CICD-01, CICD-02, CICD-04, CICD-08 | Gate *ordering* and actual GitHub Actions execution cannot be proven by any local command — only a real workflow run confirms the pipeline behaves as configured | Open a PR against this branch after the workflow file lands; confirm all checks run and a deliberately-broken lint/secret/vuln fails the corresponding check |
| First real merge to main exercises version/SBOM/push path | CICD-05, CICD-06, CICD-07 | Semver computation, SBOM generation, and ghcr.io push only happen on a real merge-to-main event | Merge the PR; confirm a new semver tag is created, an SBOM artifact is attached/generated, and `docker pull ghcr.io/danielrpof/drop-tracker:vX.Y.Z` succeeds |
| `docker-compose up` full local dev loop | CICD-09 | Requires a running Docker daemon and observing real container health, not just config parsing | `docker compose up --wait`, confirm both `postgres` and `app` report healthy, then `curl -f http://localhost:8080/health` returns 200 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s (local); CI-gated requirements documented as Manual-Only above
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending — draft seeded from 07-RESEARCH.md's Validation Architecture section at plan-phase time; plan-checker confirms Dimension 8 coverage once PLAN.md files exist, real Task IDs replace `07-TBD` at that point.
