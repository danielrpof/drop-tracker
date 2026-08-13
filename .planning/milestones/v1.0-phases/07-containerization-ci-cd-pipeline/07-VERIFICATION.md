---
phase: 07-containerization-ci-cd-pipeline
verified: 2026-08-12T16:33:07Z
status: passed
score: 10/10 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 07: Containerization & CI/CD Pipeline Verification Report

**Phase Goal:** Every push is automatically linted, tested, and security-scanned, and every merge to main produces a versioned, non-root, single-image build published to a container registry — with the full stack (API, scheduler, notifier, embedded SPA) also runnable locally via docker-compose.
**Verified:** 2026-08-12T16:33:07Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every push runs golangci-lint, go vet, and the full Go test suite before any build/publish step | ✓ VERIFIED | `.github/workflows/full-pipeline.yml`: `vet`, `lint`, `test` are independent jobs with no `paths:`/`branches:` filter on `push`/`pull_request` triggers; `build-scan` declares `needs: [vet, lint, test, gitleaks, trivy-fs]`. Confirmed locally: `go vet ./...` exit 0, `golangci-lint run ./...` "0 issues", `golangci-lint config verify` exit 0. Real remote run 31615730026 shows all jobs green. |
| 2 | Every push is scanned for secrets (gitleaks) and the built image is scanned for CRITICAL/HIGH (Trivy) — either finding blocks the pipeline | ✓ VERIFIED | `gitleaks` job (`fetch-depth: 0`, `gitleaks-action@e0c47f4f8be36e29cdc102c57e68cb5cbf0e8d1e` = v3.0.0, verified against live `git ls-remote`); `trivy-fs` job scans filesystem (`scan-type: fs`, `severity: CRITICAL,HIGH`, `exit-code: '1'`); `build-scan` job scans the built image (`image-ref: drop-tracker:scan`, same severity/exit-code). Local reproduction: Trivy image scan of the built image reports 0 vulnerabilities; Trivy fs scan of `web/` (pnpm-lock.yaml) reports 0 vulnerabilities. Real CI run 31615730026 shows `gitleaks` and `trivy-fs` both green. |
| 3 | A merge to main computes a semantic version, generates an SBOM, and pushes the built image to ghcr.io tagged with that version | ✓ VERIFIED | `release` job: `needs: [build-scan]`, `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`, svu version compute with explicit fail on no-bump, `docker push ghcr.io/danielrpof/drop-tracker:${VERSION}`, `anchore/sbom-action` with `format: spdx-json` + `output-file` + `upload-artifact`. Confirmed on real remote: run 31615730026 published `ghcr.io/danielrpof/drop-tracker:v0.2.1`, `sbom-v0.2.1` artifact attached. Verified directly in this session: `docker pull ghcr.io/danielrpof/drop-tracker:v0.2.1` succeeds after `docker logout ghcr.io` (public, no auth), and the image runs as uid 10001. |
| 4 | The full application (API + scheduler + notifier + embedded SPA) runs as a single non-root multi-stage Docker image, reproducible locally via `docker compose up` alongside Postgres | ✓ VERIFIED | `Dockerfile`: three stages (`node:26-alpine3.24` → `golang:1.26.5-alpine3.24` → `alpine:3.24`, all pinned by digest), `USER 10001:10001`, `HEALTHCHECK` against `/health`. Verified directly: `docker build -t drop-tracker:verify .` exits 0; `docker image inspect --format '{{.Config.User}}'` = `10001:10001`; `docker compose up -d --wait` brings both `postgres` and `app` to `healthy`; `curl http://localhost:8080/health` returns `{"status":"ok","db":"up"}`; `curl http://localhost:8080/` returns the embedded SPA's `<!DOCTYPE html>`; `docker exec drop-tracker-app-1 id` reports `uid=10001(app) gid=10001(app)`. |
| 5 | All security-sensitive third-party GitHub Actions are pinned to commit SHAs, and a pre-commit hook runs golangci-lint and gitleaks locally before any commit | ✓ VERIFIED | `grep -cE '^\s*uses:'` = 23, `grep -cE '^\s*uses: [^ ]+@[0-9a-f]{40}'` = 23 (100% pinned). Every one of the 12 distinct SHAs re-verified in this session against the live remote via `git ls-remote` (including correctly using the peeled `^{}` SHA for annotated tags `golangci-lint-action`, `trivy-action`) — all match exactly. `actionlint` (pinned container) reports zero errors. `.pre-commit-config.yaml` has both `gitleaks` (rev v8.30.1) and `golangci-lint` (rev v2.12.2) hooks; `python -m pre_commit run --all-files` passes both locally in this session. |

**Score:** 5/5 roadmap success criteria verified (10/10 combined plan must-haves — see artifact/requirements sections below)

### Post-Execution Hardening Verification (per orchestrator's flagged context)

| Claim | Verified | Evidence |
|-------|----------|----------|
| Two real data races fixed (`internal/httpserver/search.go`, `internal/db/pool_timeout_test.go`), commit `669ea5d` | ✓ VERIFIED | Both files read directly: `search.go`'s `handleSearch` now collects `sourceErrs` under a mutex and calls `httplog.SetAttrs` only after `wg.Wait()`, in a single goroutine, with an explanatory comment citing the race. `pool_timeout_test.go`'s `blackHoleAddr` cleanup now does `ln.Close()` → `wg.Wait()` → `close(held)`, eliminating the close-vs-send race. Commit `669ea5d` present in `git log`. |
| 8 code review issues (CR-01, CR-02, WR-01..04, IN-01, IN-02) all fixed, commits `6bab55a` + `f6647ec` | ✓ VERIFIED | CR-01: `lint` job now has `actions/setup-go` before `golangci-lint-action`. CR-02: `build-scan` saves/uploads `scanned-image` artifact; `release` downloads/loads/tags/pushes that exact artifact via `docker load` + `docker tag` + `docker push` (no second `docker/build-push-action` invocation). WR-01: `trivy-fs` job added (`scan-type: fs`). WR-02: `.golangci.yml` enables `gosec` with a `_test\.go$` exclusion; 5 inline `//nolint:gosec` justifications found in production code (`internal/detection/detector.go`, `internal/detection/musicbrainz.go` x2, `internal/httpserver/events.go`, `internal/db/migrate.go`), each with a linter name + reason. WR-03: SBOM step has `output-file: sbom.spdx.json` + a dedicated `Upload SBOM artifact` step. WR-04: `release` job has its own `concurrency: group: release-${{ github.ref }}, cancel-in-progress: false`. IN-01: all three `FROM` lines in `Dockerfile` pinned by `@sha256:...` digest. IN-02: every job in the workflow has `timeout-minutes` (5-20 min range). Both commits present in `git log --oneline main`. |
| Real GitHub Actions run 31615730026 — 8 jobs green, published v0.2.1, uploaded SBOM | ✓ VERIFIED | `gh run view 31615730026` confirms: `vet`, `test`, `lint`, `build-scan`, `release`, `gitleaks`, `trivy-fs` all ✓; `pr-title` correctly skipped (push event, not PR); artifacts include `sbom-v0.2.1`, `scanned-image`. `docker pull ghcr.io/danielrpof/drop-tracker:v0.2.1` succeeds unauthenticated in this session; runs as uid 10001. |
| Web dependency bumps (nanoid, postcss, react-router) found by trivy-fs | ✓ VERIFIED | `web/package.json` shows `react-router: 7.18.2` (direct bump). `web/pnpm-workspace.yaml` has `overrides: { nanoid: '>=3.3.17', postcss: '>=8.5.18' }` with a dated comment citing WR-01. `web/pnpm-lock.yaml` resolves `nanoid@6.0.1` and `postcss@8.5.26`, both satisfying the overrides. Local Trivy `fs` scan of `web/` (pnpm-lock.yaml) in this session reports 0 vulnerabilities. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `Dockerfile` | 3-stage non-root build, digest-pinned bases | ✓ VERIFIED | Builds clean, runs as 10001:10001, Trivy scan 0 findings |
| `.dockerignore` | Excludes `.env`, `.git/`, webassets tree | ✓ VERIFIED | All required exclusions present |
| `docker-compose.yml` | `app:` service, health-gated on `postgres` | ✓ VERIFIED | `docker compose up -d --wait` brings both services healthy |
| `.golangci.yml` | v2 schema, standard set + gosec | ✓ VERIFIED | `config verify` exit 0, `run ./...` exit 0 (0 issues) |
| `.pre-commit-config.yaml` | gitleaks + golangci-lint hooks | ✓ VERIFIED | `pre_commit run --all-files` passes both |
| `.github/workflows/full-pipeline.yml` | vet/lint/test/gitleaks/trivy-fs/pr-title → build-scan → release | ✓ VERIFIED | actionlint 0 errors; real remote run all-green |
| `.trivyignore` | Only if unfixable finding | ✓ VERIFIED (absent) | No file exists — no finding required it, consistent with 0-vulnerability scans |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `build-scan` job | `Dockerfile` | `docker/build-push-action` local build + Trivy scan | ✓ WIRED | `push: false, load: true, tags: drop-tracker:scan` |
| `build-scan` job | `release` job | Saved image artifact (`docker save`/`upload-artifact` → `download-artifact`/`docker load`) | ✓ WIRED | Fixes CR-02 — exact scanned bytes reach ghcr.io, no second build |
| `release` job | `build-scan` job | `needs: [build-scan]` + `if: push && ref == main` | ✓ WIRED | Elevated token unreachable from PRs; confirmed no `release` run on PR event 31609787719 |
| `.pre-commit-config.yaml` | `.golangci.yml` | golangci-lint hook reads repo-root config | ✓ WIRED | Same `v2.12.2` pin as CI |
| Dockerfile `HEALTHCHECK` | `internal/httpserver/health.go` | `wget` against `/health` | ✓ WIRED | `docker inspect` reports `healthy`; `/health` returns `{"status":"ok","db":"up"}` |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Image builds and runs non-root | `docker build .` + `docker image inspect --format '{{.Config.User}}'` | `10001:10001` | ✓ PASS |
| `/health` reports DB up | `curl http://localhost:8080/health` | `{"status":"ok","db":"up"}` | ✓ PASS |
| `/` serves embedded SPA | `curl http://localhost:8080/` | `<!DOCTYPE html>...` | ✓ PASS |
| `docker compose up --wait` brings full stack healthy | `docker compose up -d --wait` | both `postgres` and `app` → `healthy` | ✓ PASS |
| Trivy image scan clean | `trivy image drop-tracker:verify --severity CRITICAL,HIGH --exit-code 1` | 0 vulnerabilities, exit 0 | ✓ PASS |
| Trivy fs scan of web deps clean | `trivy fs web/ --scanners vuln` | `pnpm-lock.yaml`: 0 vulnerabilities | ✓ PASS |
| `go vet` / `golangci-lint run` clean | `go vet ./...` / `golangci-lint run ./...` | both exit 0 | ✓ PASS |
| actionlint on workflow | `docker run rhysd/actionlint:latest` | zero errors | ✓ PASS |
| Pre-commit hooks pass | `python -m pre_commit run --all-files` | gitleaks + golangci-lint both Passed | ✓ PASS |
| Published image is public/pullable, non-root | `docker logout ghcr.io && docker pull ghcr.io/.../drop-tracker:v0.2.1` + `id -u` | pulls unauthenticated; `10001` | ✓ PASS |
| Real CI run (all jobs) | `gh run view 31615730026` | vet/test/lint/build-scan/release/gitleaks/trivy-fs all ✓, pr-title correctly skipped | ✓ PASS |
| Real PR run (no publish on PR) | `gh run view 31609787719` | 6 jobs green, no `release` job, no scanned-image artifact on PR run | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CICD-01 | 07-02, 07-03 | golangci-lint, go vet, test suite gate every push | ✓ SATISFIED | Three independent jobs, no paths/branches filter, `needs:` on `build-scan` |
| CICD-02 | 07-03 | gitleaks scans every push, blocks on detection | ✓ SATISFIED | `gitleaks` job with `fetch-depth: 0`, gitleaks-action v3 |
| CICD-03 | 07-01 | Multi-stage Docker image, non-root, full app | ✓ SATISFIED | Dockerfile verified end to end |
| CICD-04 | 07-03 | Trivy scans built image, blocks on CRITICAL/HIGH | ✓ SATISFIED | `build-scan` job, local + real-run scan both clean |
| CICD-05 | 07-04 | SBOM generated for built image | ✓ SATISFIED | spdx-json artifact `sbom-v0.2.1` confirmed on real run |
| CICD-06 | 07-04 | Semantic version computed, release tagged on merge | ✓ SATISFIED | svu-computed `v0.2.1` tag exists locally + on remote |
| CICD-07 | 07-04 | Built/scanned image pushed to ghcr.io tagged with semver | ✓ SATISFIED | `ghcr.io/danielrpof/drop-tracker:v0.2.1` pulled and verified in this session |
| CICD-08 | 07-03 | Third-party Actions pinned to commit SHAs | ✓ SATISFIED | 23/23 `uses:` lines SHA-pinned, all re-verified against live remote |
| CICD-09 | 07-01 | `docker-compose` brings up app + Postgres locally | ✓ SATISFIED | `docker compose up -d --wait` verified healthy in this session |
| CICD-10 | 07-02 | Pre-commit runs golangci-lint + gitleaks locally | ✓ SATISFIED (codebase); ⚠️ REQUIREMENTS.md stale | `.pre-commit-config.yaml` has both hooks, both pass locally in this session — see note below |

**Note on CICD-10 / REQUIREMENTS.md staleness:** `.planning/REQUIREMENTS.md` line 67 still shows `- [ ] **CICD-10**` with a note "golangci-lint half deferred to Phase 07," and its traceability table (line 140) marks CICD-10 "Partial." This is stale documentation — plan `07-02` demonstrably added the golangci-lint pre-commit hook (verified directly: `.pre-commit-config.yaml` contains the hook, `python -m pre_commit run --all-files` passes both hooks in this session, `07-02-SUMMARY.md` and `07-REVIEW.md` both confirm it shipped). This is a documentation-tracking gap, not a functional gap — the phase goal's fifth success criterion ("a pre-commit hook runs golangci-lint and gitleaks locally") is met in the actual codebase. Flagged as a non-blocking documentation fix (update the checkbox and traceability table), not a phase gap.

### Anti-Patterns Found

None. No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers in the phase's workflow, Dockerfile, or config files. The 5 `//nolint:gosec` directives found in production Go code all name the specific linter and carry a same-line justification (per plan requirement), and were reviewed as genuinely benign (bounded int32 conversions, values re-read from the app's own previously-stored data).

## Gaps Summary

No gaps found. All 5 roadmap success criteria and all 10 plan-level must-haves across the four plans (07-01 through 07-04) are verified directly against the codebase and, where the plans required it, against the real GitHub remote (a real PR run and a real merge-to-main run). The post-execution hardening described in the task context (two data-race fixes, 8 code-review fixes, the trivy-fs addition and resulting dependency bumps) is all present and correctly wired in the current `main` branch — not merely claimed in SUMMARY.md files.

The only discrepancy found is a stale checkbox/table entry in `.planning/REQUIREMENTS.md` for CICD-10, which understates work that is actually complete in the codebase. This does not block phase completion; it is a documentation housekeeping item.

---

_Verified: 2026-08-12T16:33:07Z_
_Verifier: Claude (gsd-verifier)_
