# Phase 7: Containerization & CI/CD Pipeline - Research

**Researched:** 2026-08-12
**Domain:** GitHub Actions CI/CD pipeline, multi-stage Docker builds, container security scanning, semantic versioning
**Confidence:** MEDIUM-HIGH (version/SHA facts VERIFIED via direct registry/API/git calls this session; workflow-composition patterns CITED from official docs; a few ecosystem judgment calls remain ASSUMED — see Assumptions Log)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Final Dockerfile stage uses `alpine` (not distroless, not debian-slim) — small footprint, retains a shell for debugging via `docker exec`, well-documented, works cleanly with the existing `CGO_ENABLED=0` static Go build.
- **D-02:** Non-root user uses a fixed numeric UID/GID (e.g. `10001:10001`) via explicit `addgroup -g 10001 app && adduser -D -u 10001 -G app app` — not an auto-assigned UID. Deterministic across rebuilds, referenceable later if a VPS/orchestrator SecurityContext is added.
- **D-03:** Dockerfile includes a `HEALTHCHECK` instruction against the existing `/health` endpoint (Phase 1) — alpine's busybox `wget` is sufficient (`wget -qO- http://localhost:$PORT/health || exit 1`). Makes `docker ps` / compose show real container health, not just process-alive.
- **D-04:** First release is tagged `v0.1.0`, manually seeded once the pipeline lands; every subsequent merge to main lets `svu` compute the next tag from conventional-commit prefixes in the merged history. This signals "evolving pre-1.0 portfolio project," not a completed v1.0.0 release, even though STATE.md's milestone label is "v1.0" (that label is about the requirements milestone, not the semver line).
- **D-05:** The ghcr.io image package is public. The GitHub repo (`danielrpof/drop-tracker`) is confirmed already public (`gh repo view`), so the package inherits public visibility by default with no extra manual step.
- **D-06:** CI adds a lightweight PR-title conventional-commit check (`amannn/action-semantic-pull-request`), not full per-commit commitlint and not zero enforcement.
- **D-07:** Trivy blocks the pipeline on `CRITICAL,HIGH` (matches CLAUDE.md's Technology Stack doc exactly — not a deviation).
- **D-08:** Escape hatch for an unfixable CRITICAL/HIGH finding (no upstream patch available) is a committed `.trivyignore` file, with each entry carrying the CVE ID plus a one-line dated reason. Mirrors the precedent already set by `quick/260806-hfn`'s documented-acceptance pattern for the 4 pre-existing gitleaks findings (no silent suppression, no history rewrite).
- **D-09:** PRs also build the full multi-stage image and run Trivy against it (build-only, no push to ghcr.io) — not just lint/vet/test/gitleaks-on-source. Push to ghcr.io + SBOM + semver tag still only happens on merge to main.
- **D-10:** The app service in `docker-compose.yml` builds from the local Dockerfile every `docker-compose up` (`build: .` context), not a prebuilt/pulled ghcr.io tag.
- **D-11:** The app service loads env vars via `env_file: .env` — the same gitignored file `.env.example` already documents and that `go run` uses locally.
- **D-12:** The app service's `environment:` block explicitly overrides `DATABASE_URL` to `postgres://drop_tracker:drop_tracker@postgres:5432/drop_tracker?sslmode=disable`, layered on top of `env_file: .env`, because `.env`'s `DATABASE_URL` points at `localhost:5433` (the host-mapped port). Comment this override in docker-compose.yml.

### Claude's Discretion

- Exact multi-stage Dockerfile layout (number/order of stages) — follows the existing `Makefile web` target's build-then-embed convention, adapted so the image builds the SPA itself.
- Exact GitHub Actions workflow file/job structure (single "Full Pipeline" workflow vs. split jobs within it) — PROJECT.md names it "GitHub Actions 'Full Pipeline'" (singular); job/step decomposition is discretionary.
- SBOM format (spdx-json vs cyclonedx-json) — default to spdx-json unless research surfaces a reason otherwise.
- Exact golangci-lint pre-commit hook scope (lint only changed/staged files vs. whole repo) — follows whatever `.pre-commit-config.yaml`'s existing gitleaks-hook precedent suggests structurally.
- Pinning every security-sensitive GitHub Action to a commit SHA (CICD-08) — mechanical, no decision needed.
- Whether the CI test job runs `make test-integration` directly or an equivalent explicit `-p 1` invocation — behavior is locked (Folded Todos), wire-level command is discretionary.

### Folded Todos

- **`.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md`** — folded into this phase's scope. The CI test step must not use bare `go test ./...`; it must use `make test-integration` (already pins `-p 1`) or an equivalent explicit `-p 1` invocation.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope. (Three unrelated frontend bugs were reviewed and explicitly NOT folded into this phase: SearchBox AbortController leak, EventCard unrecognized event_type crash, guestFeatureHref missing encodeURIComponent.)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CICD-01 | Every push runs golangci-lint, go vet, and the Go test suite before any build/publish step | Standard Stack (golangci-lint-action v9.3.0), Code Examples "Lint/Vet/Test Job", Common Pitfalls #1 (no `.golangci.yml` exists yet) |
| CICD-02 | Every push runs gitleaks to scan for committed secrets, blocking on detection | Standard Stack (gitleaks-action — STALE, see State of the Art), Code Examples "Gitleaks Job" |
| CICD-03 | Multi-stage Docker image (slim base, non-root) containing API/scheduler/notifier/embedded frontend | Architecture Patterns "Multi-Stage Dockerfile", Code Examples "Dockerfile" |
| CICD-04 | Trivy scans built image, blocks publishing on critical vulnerabilities | Code Examples "Build-Scan-Push Job", Common Pitfalls #4 |
| CICD-05 | Pipeline generates an SBOM for the built image | Code Examples "SBOM Step" |
| CICD-06 | Pipeline computes semantic version and tags a release automatically on merge to main | Architecture Patterns "svu bootstrap", Code Examples "Version Job" |
| CICD-07 | Pipeline pushes built, scanned image to ghcr.io tagged with the semantic version | Code Examples "Build-Scan-Push Job" |
| CICD-08 | Third-party Actions pinned to commit SHAs, not tags | Package Legitimacy Audit (SHA table), all Code Examples use pinned SHAs |
| CICD-09 | `docker-compose` brings up app + local Postgres | Code Examples "docker-compose.yml app service" |
| CICD-10 | Pre-commit runs golangci-lint (new) and gitleaks (existing) locally before commit | Code Examples ".pre-commit-config.yaml addition" |
</phase_requirements>

## Summary

This phase adds no new Go/npm/pip dependencies — its entire surface is GitHub Actions workflow YAML, a new root `Dockerfile`, `.dockerignore`, an `app:` service in the existing `docker-compose.yml`, and one new pre-commit hook entry. The riskiest part of "getting this right" is not writing Docker/YAML syntax (well-trodden) but **verifying that CLAUDE.md's pinned tool versions are still current** — three of the eight pinned Action versions in CLAUDE.md are now stale as of this research date and must be corrected during planning (see State of the Art below): `gitleaks-action@v2` → `v3` (v2 loses Node20 runner support Sept 16, 2026), `docker/build-push-action@v6` → `v7`, and `docker/login-action` (unpinned major in CLAUDE.md) should target `v4`. `golangci-lint` v2.12.2, `golangci-lint-action` v9.3.0, `trivy-action` v0.36.0, `anchore/sbom-action` v0.24.0, and `gitleaks` CLI v8.30.1 are all confirmed still current — no changes needed there.

For CICD-08 (SHA pinning), this research directly resolved and verified (via `git ls-remote`, not LLM guessing) the exact 40-character commit SHA for every Action this phase needs, including correctly peeling annotated tags (`golangci-lint-action`, `trivy-action`, `svu` are annotated; the rest are lightweight tags where the ref SHA already is the commit SHA) — a common pinning mistake is using the *tag object* SHA instead of the *commit* SHA for annotated tags, which silently resolves to the wrong ref. These SHAs are given directly in the Standard Stack section below; the planner does not need to re-derive them, but should re-verify at implementation time if more than a few days elapse (`git ls-remote <repo> refs/tags/<tag> refs/tags/<tag>^{}`).

The single trickiest sequencing question this phase's CONTEXT.md leaves open (D-09: PR-time scan-but-no-push vs. merge-time scan-then-push) has a well-established single-job answer: `docker/build-push-action` supports `push: false, load: true` to build into the local Docker daemon for Trivy to scan, then a second `build-push-action` step with `push: true` re-runs against the same Dockerfile and — because GitHub Actions cache (`cache-from`/`cache-to: type=gha`) already holds every layer from the scan build — the second "build" is nearly free. This avoids splitting the pipeline into a build job and a separate push job with artifact-passing between them.

**Primary recommendation:** Single `.github/workflows/full-pipeline.yml` with a `checks` job (lint, vet, test, gitleaks, PR-title lint — parallelizable as separate jobs or matrix steps within one job, planner's discretion) that gates a `build-scan-push` job; on `pull_request`, `build-scan-push` builds+scans but does not push; on `push` to `main`, it additionally computes the `svu` version, generates the SBOM, and pushes to ghcr.io.

## Architectural Responsibility Map

This phase is CI/CD infrastructure, not application request-flow — the standard Browser/SSR/API/CDN/DB tiers don't directly apply. Tiers below are adapted to the build/release pipeline's own layering.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Lint/vet/test gate | CI Pipeline (GitHub Actions) | — | Must run before any build step exists; pure CI concern, no runtime component |
| Secret scanning (source) | CI Pipeline (GitHub Actions) + Local (pre-commit) | — | Gitleaks runs identically in both places — CICD-02 (CI backstop) and CICD-10 (local pre-commit) are the same tool, two enforcement points |
| Image build | Container Build (Dockerfile, multi-stage) | CI Pipeline (invokes it) | The multi-stage Dockerfile is itself the build authority; CI only invokes `docker buildx build`, it doesn't reimplement build logic |
| Embedded SPA build | Container Build (Dockerfile Node stage) | — | Per D-10's rationale, the image builds `web/` itself rather than trusting the committed `internal/webassets/build/client/` tree — this SPA-build responsibility moves from `Makefile web` (local/committed-tree convenience) into the Dockerfile's own Node stage |
| Image vulnerability scan | CI Pipeline (Trivy step) | — | Runs against the built image, blocks the pipeline; not a runtime concern |
| SBOM generation | CI Pipeline (sbom-action) | Registry (ghcr.io, as release asset) | Generated in CI, may be attached to a GitHub Release for downstream consumption |
| Semantic version compute | CI Pipeline (svu) | Registry (tag on git + image) | `svu` reads git history (CI-local operation), result becomes both a git tag and an image tag |
| Image publish | Registry (ghcr.io) | CI Pipeline (pushes it) | ghcr.io is the tier of record; CI is merely the writer |
| Runtime container (API+scheduler+notifier+SPA) | Runtime Container (single Go binary, non-root) | API/Backend (chi router inside) | Single-process architecture (PROJECT.md) — the "Backend" tier and "container" tier are the same process here, no split |
| Local dev orchestration | Local Orchestration (docker-compose) | Database/Storage (postgres service) | `docker-compose.yml` coordinates `app` + `postgres`; `app`'s `depends_on` health-gates against `postgres`'s existing `pg_isready` check |

## Standard Stack

### Core

| Tool | Version (pin) | Purpose | Currency |
|------|---------|---------|--------------|
| `golangci-lint` (CLI) | v2.12.2 | Go linting, v2 config schema | [VERIFIED: GitHub API repos/golangci/golangci-lint/releases/latest → tag_name v2.12.2, published 2026-05-06T11:36:47Z] Matches CLAUDE.md exactly — current, not stale |
| `golangci/golangci-lint-action` | v9.3.0 | Runs golangci-lint in CI | [VERIFIED: GitHub API repos/golangci/golangci-lint-action/releases/latest → tag_name v9.3.0] Matches CLAUDE.md exactly — current. Requires `node24` runtime (ambient on current GitHub-hosted runners). Compatible with golangci-lint ≥ v2.1.0 |
| `gitleaks` (CLI, via pre-commit) | v8.30.1 | Secret scanning | [VERIFIED: `git ls-remote https://github.com/gitleaks/gitleaks.git refs/tags/v8.3*` — no tag newer than v8.30.1 exists] Matches CLAUDE.md exactly — current |
| `gitleaks/gitleaks-action` | **v3.0.0** (CLAUDE.md's `v2` is STALE) | CI secret-scan backstop | [VERIFIED: `git ls-remote` + WebFetch release notes] v3.0.0 released; v2 workflows require an extra env var once GitHub Actions runners default to Node24 (2026-06-02) and **stop working entirely 2026-09-16** when Node20 is removed. v3 is a drop-in replacement — "No changes to inputs, outputs, or behavior." See State of the Art. |
| `aquasecurity/trivy-action` | v0.36.0 | Image vulnerability scan | [VERIFIED: GitHub API repos/aquasecurity/trivy-action/releases/latest → tag_name v0.36.0, published 2026-04-22] Matches CLAUDE.md exactly — current |
| `anchore/sbom-action` | v0.24.0 | SBOM generation (syft v1.42.3 bundled) | [CITED: github.com/anchore/sbom-action releases page] Matches CLAUDE.md exactly — current, still latest |
| `docker/build-push-action` | **v7.3.0** (CLAUDE.md's `v6` is STALE) | Build + push to ghcr.io | [VERIFIED: GitHub API repos/docker/build-push-action/releases/latest → tag_name v7.3.0, published 2026-07-01] v7 superseded v6; no known breaking `with:` input changes for this project's usage (image-ref/tags/push/context inputs unchanged), but confirm via changelog at implementation time |
| `docker/login-action` | v4.6.0 (CLAUDE.md leaves unpinned) | Auth to ghcr.io | [VERIFIED: GitHub API repos/docker/login-action/releases/latest → tag_name v4.6.0, published 2026-07-29] |
| `docker/setup-buildx-action` | v4.2.0 (not named in CLAUDE.md but required) | Enables Buildx (needed for `cache-from/to: type=gha` and multi-step build+scan+push pattern) | [VERIFIED: GitHub API repos/docker/setup-buildx-action/releases/latest → tag_name v4.2.0, published 2026-07-02] **CLAUDE.md's Installation table omits this action entirely — it is required** by the `docker/build-push-action` README for GHA cache usage. Flag as a gap to add during planning. |
| `caarlos0/svu` (CLI, `go install`) | v3.4.1 (CLAUDE.md names the tool but not a version) | Semver computation from conventional commits | [VERIFIED: GitHub API repos/caarlos0/svu/releases/latest → tag_name v3.4.1, published 2026-05-01] **No official GitHub Action wrapper exists** (no `action.yml` at repo root — confirmed 404). Install via `go install github.com/caarlos0/svu/v3@v3.4.1` (note the `/v3` in the module path — v2→v3 renamed most CLI flags per the README; don't copy v2-era `svu` invocation examples). Command: `svu next`. |
| `amannn/action-semantic-pull-request` | v6.1.1 | PR-title conventional-commit gate (D-06) | [VERIFIED: GitHub API repos/amannn/action-semantic-pull-request/releases/latest → tag_name v6.1.1, published 2025-08-22] |
| `actions/checkout` | v7.0.1 | Repo checkout (used by every job) | [VERIFIED: GitHub API + git ls-remote, published 2026-07-20] Not named in CLAUDE.md's table but required by every job |
| `actions/setup-go` | v7.0.0 | Go toolchain setup | [VERIFIED: GitHub API + git ls-remote, published 2026-07-16] Not named in CLAUDE.md's table but required by lint/test jobs |

### Docker base images

| Image | Tag | Verified |
|-------|-----|----------|
| Go build stage | `golang:1.26-alpine3.24` (or floating `golang:1.26-alpine`) | [VERIFIED: Docker Hub registry API `hub.docker.com/v2/repositories/library/golang/tags?name=1.26` returned `1.26-alpine3.24`, `1.26-alpine3.23`, `1.26-alpine`, `1.26.5-alpine3.24`, `1.26.5-alpine3.23`, `1.26.5-alpine`] Matches `go.mod`'s `go 1.26` directive [VERIFIED: C:/CodeProjects/drop-tracker/go.mod:3 `go 1.26`]. Local dev toolchain is `go1.26.5 windows/amd64` [VERIFIED: `go version` this session] — prefer the pinned `golang:1.26.5-alpine3.24` over the floating `1.26-alpine` tag for build reproducibility. |
| Node build stage | `node:26-alpine3.24` (or pinned `node:26.7.0-alpine3.24`) | [VERIFIED: Docker Hub registry API `hub.docker.com/v2/repositories/library/node/tags?name=26-alpine` returned `26-alpine3.24`, `26-alpine3.23`, `26-alpine`, `26-alpine3.22`; `name=26` also returned `26.7.0-alpine3.24`] **Node 26 does not bundle corepack** [CITED: multiple 2026 sources — Node TSC voted to stop distributing corepack starting with Node 25] — do not rely on `corepack enable` in this stage; install pnpm explicitly (see Pitfall below). |
| Final runtime stage | `alpine:3.24` (or `alpine:3.24.1`) | [VERIFIED: Docker Hub registry API `hub.docker.com/v2/repositories/library/alpine/tags` returned `3.24.1`, `3.24`, `3.23.5`, `3.23`, `3.22.5`, `3.22`, `latest`, `edge`] D-01 requires plain `alpine`, not a language-specific variant, for the final stage. |
| pnpm (installed in Node stage) | v11.8.0 (major v11) | [VERIFIED: `pnpm --version` on this dev machine → 11.8.0, this session] `web/pnpm-lock.yaml` declares `lockfileVersion: '9.0'` [VERIFIED: web/pnpm-lock.yaml:1] — pnpm 11.8.0 is what this repo's lockfile is actually tested against locally; pin the Dockerfile's pnpm install to this same version (`npm install -g pnpm@11.8.0`) rather than an untested newer major. |

**Installation (CI job, no go.mod/package.json changes — nothing to `npm install`/`go get` for the pipeline itself):**
```bash
# svu, installed as a build tool inside the version-compute job, not a project dependency
go install github.com/caarlos0/svu/v3@v3.4.1
```

**Version verification command used this session (repeatable at implementation time):**
```bash
# Actions — resolve tag to commit SHA (peel annotated tags with ^{})
git ls-remote https://github.com/<owner>/<repo>.git refs/tags/<tag> refs/tags/<tag>^{}

# Go/Node/Alpine base image tags
curl -s "https://hub.docker.com/v2/repositories/library/golang/tags?name=1.26&page_size=25"
curl -s "https://hub.docker.com/v2/repositories/library/node/tags?name=26-alpine&page_size=25"

# GitHub release currency
curl -s "https://api.github.com/repos/<owner>/<repo>/releases/latest"  # (or via `gh api`)
```

## Package Legitimacy Audit

**Not applicable in the standard npm/pypi/crates sense** — this phase installs zero new entries into `go.mod`, `web/package.json`, or `web/pnpm-lock.yaml`. Its entire external-dependency surface is GitHub Actions (pulled by tag/SHA at workflow-run time, not vendored) and Docker Hub base images. The equivalent supply-chain risk for *this* phase is Action/image provenance, which is addressed directly by CICD-08 (SHA pinning) and the version table above rather than a package-registry legitimacy check.

| Action / Image | Verdict | Disposition |
|---|---|---|
| All 11 GitHub Actions listed in Standard Stack | Official first-party publishers (`actions/*`, `docker/*`, `golangci/*`, `gitleaks/*`, `aquasecurity/*`, `anchore/*`, `amannn/*`) — all have long-lived, high-star, actively-maintained repos | Approved — pin to the exact commit SHAs given below |
| `golang`, `node`, `alpine` base images | Docker Official Images (`library/*` namespace) | Approved |

**Exact commit SHAs for CICD-08 (verified this session via `git ls-remote`, not guessed):**

| Action | Tag | Commit SHA (use this in `uses:`) | Tag type |
|---|---|---|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` | lightweight (ref SHA = commit SHA) |
| `actions/setup-go` | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` | lightweight |
| `golangci/golangci-lint-action` | v9.3.0 | `ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a` | **annotated — this is the peeled `^{}` commit SHA, not the raw ref SHA (`d583c34f...`)** |
| `gitleaks/gitleaks-action` | v3.0.0 | `e0c47f4f8be36e29cdc102c57e68cb5cbf0e8d1e` | lightweight |
| `aquasecurity/trivy-action` | v0.36.0 | `ed142fd0673e97e23eac54620cfb913e5ce36c25` | **annotated — peeled `^{}` SHA (not `a9c7b0f0...`)** |
| `anchore/sbom-action` | v0.24.0 | `e22c389904149dbc22b58101806040fa8d37a610` | lightweight |
| `docker/build-push-action` | v7.3.0 | `53b7df96c91f9c12dcc8a07bcb9ccacbed38856a` | lightweight |
| `docker/login-action` | v4.6.0 | `dbcb813823bdd20940b903addbd779551569679f` | lightweight |
| `docker/setup-buildx-action` | v4.2.0 | `bb05f3f5519dd87d3ba754cc423b652a5edd6d2c` | lightweight |
| `amannn/action-semantic-pull-request` | v6.1.1 | `48f256284bd46cdaab1048c3721360e808335d50` | lightweight |

**Important:** these SHAs are correct as of this research session (2026-08-12). Commit-SHA pinning is only as good as the moment it was resolved — if the planner/executor session is materially later, re-run the `git ls-remote` command above rather than trusting this table blindly. Do **not** guess a SHA from memory; a wrong 40-hex-char guess fails silently differently than a real supply-chain compromise but is just as broken (workflow simply can't resolve the ref, hard build failure — annoying but at least not a silent security hole).

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────┐     push/PR      ┌──────────────────────────────────────────┐
│  Developer   │ ───────────────▶│           GitHub Actions: checks          │
│ (git push /  │                  │  ┌────────┐ ┌────────┐ ┌──────────────┐ │
│  open PR)    │                  │  │lint+vet│ │  test  │ │   gitleaks   │ │
└──────┬───────┘                  │  │(golangci-  make    │ │(secret scan) │ │
       │                          │  │ lint)  │ │test-integ)│              │ │
       │ (pre-commit hook,        │  └────────┘ └────────┘ └──────────────┘ │
       │  local, before commit    │  ┌──────────────────────────────────┐   │
       │  reaches the pipeline)   │  │ PR-title conventional-commit gate│   │
       │                          │  │ (amannn/action, PR events only)  │   │
       ▼                          │  └──────────────────────────────────┘   │
┌─────────────┐                   └──────────────────┬─────────────────────┘
│golangci-lint │                                      │ needs: [checks]
│  + gitleaks  │                                      ▼
│(git commit)  │                   ┌──────────────────────────────────────────┐
└─────────────┘                    │   GitHub Actions: build-scan-push job     │
                                    │  1. docker buildx build --load (no push) │
                                    │     (Node stage → Go stage → alpine)     │
                                    │  2. Trivy scan the loaded image          │
                                    │     severity=CRITICAL,HIGH exit-code=1   │
                                    │     ── FAILS HERE ON PR OR ON MAIN ──    │
                                    │        (blocks job, nothing pushed)      │
                                    │                                          │
                                    │  ── only if event == push to main: ──   │
                                    │  3. svu next → compute semver            │
                                    │  4. git tag + push tag                   │
                                    │  5. docker buildx build --push (reuses   │
                                    │     GHA cache from step 1, near-free)    │
                                    │     tag: ghcr.io/.../drop-tracker:vX.Y.Z │
                                    │  6. anchore/sbom-action → spdx-json      │
                                    │     against the pushed image             │
                                    └──────────────────┬───────────────────────┘
                                                        │ (PR: stops after step 2)
                                                        ▼ (main: continues)
                                    ┌──────────────────────────────────────────┐
                                    │         ghcr.io/danielrpof/drop-tracker   │
                                    │         :vX.Y.Z (public image + SBOM)     │
                                    └──────────────────┬───────────────────────┘
                                                        │ docker pull / docker-compose
                                                        ▼
                                    ┌──────────────────────────────────────────┐
                                    │  Local dev: docker-compose up             │
                                    │  ┌────────────┐        ┌───────────────┐ │
                                    │  │   app      │──────▶│   postgres    │ │
                                    │  │ build: .   │ (depends_on:          │ │
                                    │  │ env_file: .env      │ healthy)      │ │
                                    │  │ + DATABASE_URL      └───────────────┘ │
                                    │  │   override │                          │
                                    │  └────────────┘                          │
                                    └────────────────────────────────────────┘
```

### Recommended Project Structure

```
drop-tracker/
├── Dockerfile                       # new — multi-stage: node-build → go-build → alpine runtime
├── .dockerignore                    # new — exclude .git, web/node_modules, bin/, .env
├── .trivyignore                     # new, ONLY if/when an actual unfixable finding requires it (D-08) — not pre-created
├── .golangci.yml                    # new — v2 config schema; does not exist yet, gates CICD-01
├── .pre-commit-config.yaml          # existing — add golangci-lint hook entry alongside gitleaks
├── docker-compose.yml               # existing — add `app:` service (D-10/D-11/D-12)
└── .github/
    └── workflows/
        └── full-pipeline.yml        # new, greenfield — job structure per Code Examples below
```

### Pattern 1: Node → Go → Alpine three-stage Dockerfile

**What:** Stage 1 (`node:26-alpine`) builds the SPA (`web/` → `web/build/client`), mirroring `Makefile`'s `web` target exactly [VERIFIED: C:/CodeProjects/drop-tracker/Makefile:67-72 `web:` target — `cd web && pnpm install --frozen-lockfile`, `cd web && pnpm run build`, then copies `web/build/client` into `internal/webassets/build/client`]. Stage 2 (`golang:1.26-alpine`) copies the Stage-1 output into `internal/webassets/build/client` *before* `go build`, then runs a static `CGO_ENABLED=0` build — this ordering is load-bearing: `go:embed all:build/client` [VERIFIED: C:/CodeProjects/drop-tracker/internal/webassets/embed.go:20 `//go:embed all:build/client`] resolves at *compile time*, so the SPA files must exist on disk inside the Go build stage's filesystem before `go build ./cmd/server` runs, or the build fails with "pattern build/client: no matching files found." Stage 3 (`alpine`) copies only the compiled binary, creates the fixed-UID user (D-02), sets `USER 10001:10001`, and adds the `HEALTHCHECK` (D-03).

**When to use:** This is the only stage layout that satisfies D-10's requirement that the image build the SPA itself (not trust the committed tree) while keeping the final image `alpine`-based per D-01.

**Example:**
```dockerfile
# Source: pattern synthesized from Makefile's `web` target (verified) +
# internal/webassets/embed.go's go:embed directive (verified) + D-01/D-02/D-03.
# ---- Stage 1: build the SPA ----
FROM node:26-alpine3.24 AS web-build
WORKDIR /src/web
# Node 26 alpine does not bundle corepack — install pnpm explicitly, pinned
# to the version verified against this repo's lockfile (11.8.0).
RUN npm install -g pnpm@11.8.0
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

# ---- Stage 2: build the Go binary ----
FROM golang:1.26.5-alpine3.24 AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The SPA build output must land here BEFORE `go build`, because
# //go:embed all:build/client (internal/webassets/embed.go) resolves at
# compile time, not at runtime.
COPY --from=web-build /src/web/build/client ./internal/webassets/build/client
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
    -o /out/server ./cmd/server

# ---- Stage 3: minimal non-root runtime ----
FROM alpine:3.24
RUN addgroup -g 10001 app && adduser -D -u 10001 -G app app
COPY --from=go-build /out/server /usr/local/bin/server
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
```

### Pattern 2: Build-once, scan, then conditionally push (single job, D-09)

**What:** `docker/build-push-action` runs twice inside the same job — first with `push: false, load: true` (loads into the local Docker daemon so Trivy's `image-ref` can find it), then Trivy scans; only on a `push`-to-`main` event does a second `build-push-action` step run with `push: true`, reusing the GitHub Actions cache backend (`cache-from`/`cache-to: type=gha`) populated by the first build so the second "build" mostly resolves from cache rather than rebuilding.

**When to use:** Satisfies D-09 (PRs build+scan, no push) and CICD-04 (scan blocks publish) without needing a second job with artifact hand-off (`docker save`/`docker load` between jobs, or a registry-based staging push). One job, two conditional build steps, cache glue between them.

**Example:**
```yaml
# Source: pattern verified via aquasecurity/trivy-action + docker/build-push-action
# usage guidance (CITED, multiple 2026 sources on the load:true/push:false pattern)
- name: Set up Buildx
  uses: docker/setup-buildx-action@bb05f3f5519dd87d3ba754cc423b652a5edd6d2c # v4.2.0

- name: Build image (scan-only, never pushed)
  uses: docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a # v7.3.0
  with:
    context: .
    push: false
    load: true
    tags: drop-tracker:scan
    cache-from: type=gha
    cache-to: type=gha,mode=max

- name: Scan with Trivy
  uses: aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25 # v0.36.0
  with:
    image-ref: drop-tracker:scan
    severity: CRITICAL,HIGH
    exit-code: '1'
    trivyignores: .trivyignore

# --- everything below only runs on push to main ---
- name: Compute next version
  if: github.ref == 'refs/heads/main' && github.event_name == 'push'
  run: |
    go install github.com/caarlos0/svu/v3@v3.4.1
    echo "VERSION=$(svu next)" >> "$GITHUB_ENV"

- name: Login to ghcr.io
  if: github.ref == 'refs/heads/main' && github.event_name == 'push'
  uses: docker/login-action@dbcb813823bdd20940b903addbd779551569679f # v4.6.0
  with:
    registry: ghcr.io
    username: ${{ github.actor }}
    password: ${{ secrets.GITHUB_TOKEN }}

- name: Build and push (reuses scan build's cache)
  if: github.ref == 'refs/heads/main' && github.event_name == 'push'
  uses: docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a # v7.3.0
  with:
    context: .
    push: true
    tags: ghcr.io/danielrpof/drop-tracker:${{ env.VERSION }}
    cache-from: type=gha
    cache-to: type=gha,mode=max

- name: Generate SBOM
  if: github.ref == 'refs/heads/main' && github.event_name == 'push'
  uses: anchore/sbom-action@e22c389904149dbc22b58101806040fa8d37a610 # v0.24.0
  with:
    image: ghcr.io/danielrpof/drop-tracker:${{ env.VERSION }}
    format: spdx-json
```

### Anti-Patterns to Avoid

- **Running the PR-title check on `pull_request_target` when it doesn't need repo checkout:** `amannn/action-semantic-pull-request` reads the PR title via the GitHub API — it never needs to check out the PR's (possibly untrusted, fork-originated) code. Use the plain `pull_request` trigger for this job, not `pull_request_target`; the latter grants base-repo-level token/secret access to a job that has no reason to need it (a classic GHA privilege-escalation footgun when combined with checking out untrusted PR code — this job specifically avoids that combination by never checking anything out).
- **Bare `go test ./...` in CI:** already root-caused in the folded todo — Go's default per-package parallelism against the single shared integration Postgres instance causes `internal/db`'s from-scratch-schema-reset tests to race other packages' tests. CI must call `make test-integration` (already `-p 1` [VERIFIED: C:/CodeProjects/drop-tracker/Makefile:40 `TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./... -race -count=1 -p 1`]) or explicitly pass `-p 1`.
- **`corepack enable` on the `node:26-alpine` build stage:** silently fails or errors — Node 26 no longer bundles corepack. Install pnpm via `npm install -g pnpm@<pinned-version>` instead.
- **Copying `.env` into the image, or baking `DATABASE_URL`/`DISCORD_WEBHOOK_URL` as a Dockerfile `ENV`/`ARG`:** breaks the project's "secrets via env vars only, never committed, never in an image layer" invariant [VERIFIED: C:/CodeProjects/drop-tracker/internal/config/config.go:1-5 package doc: "the process environment is the single source of truth"]. Add `.env` and `.env.*` to `.dockerignore`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Semantic version calculation from commit history | A custom script parsing `git log` for `feat:`/`fix:`/`BREAKING CHANGE` prefixes | `caarlos0/svu` (`svu next`) | Already locked by CLAUDE.md/PROJECT.md; correctly handles edge cases (merge commits, `!` breaking-change suffix, monotonic-tag safety) a hand-rolled regex would miss |
| SBOM generation | Hand-written dependency manifest | `anchore/sbom-action` (syft under the hood) | Produces a standards-compliant SPDX document that downstream tooling (Dependabot alerts, container registries, compliance scanners) can actually parse; a hand-written list isn't a real SBOM |
| Container image vulnerability scanning | Manually cross-referencing `apk list` output against a CVE feed | `aquasecurity/trivy-action` | Trivy's DB is continuously updated and covers OS packages + language-level dependencies (Go modules, npm) in one pass — CLAUDE.md already locks this |
| GitHub Action version pinning verification | Trusting a cached/remembered SHA | `git ls-remote <repo> refs/tags/<tag> refs/tags/<tag>^{}` at implementation time | A guessed or stale SHA either hard-fails the workflow (ref doesn't resolve) or, worse, silently pins to the wrong commit if reused from a different tag by mistake |

**Key insight:** Every tool this phase needs is already named in CLAUDE.md's locked Technology Stack — the actual research risk here isn't "which tool" but "is the locked version still the current one, and what is its exact commit SHA." Three of eight were stale; this research corrected all three.

## Common Pitfalls

### Pitfall 1: `.golangci.yml` does not exist yet
**What goes wrong:** CICD-01 requires "every push runs golangci-lint" but running `golangci-lint run` with zero config uses golangci-lint's built-in defaults, which is a valid but *unintentional* choice — the plan should not silently rely on defaults without a planner decision.
**Why it happens:** No repo currently has `.golangci.yml`, `.golangci.yaml`, `.golangci.toml`, or `.golangci.json` at the root [VERIFIED: `find . -maxdepth 1 -iname ".golangci*"` returned nothing, this session].
**How to avoid:** Planner must include a task to create `.golangci.yml` using the **v2 config schema** (`version: "2"` at the top — the v1 schema is legacy and golangci-lint v2.12.2 will not parse a v1-era config correctly per CLAUDE.md's own warning).
**Warning signs:** `golangci-lint run` in CI either silently uses defaults (surprising drift from local pre-commit behavior) or fails to find a config the plan assumed existed.

### Pitfall 2: Annotated-tag SHA vs. commit SHA confusion
**What goes wrong:** `git ls-remote <repo> refs/tags/<tag>` returns the **tag object's own SHA** for an annotated tag (not the commit it points to). Pinning `uses: owner/repo@<that-SHA>` either fails to resolve at all, or — worse — happens to resolve to something unintended.
**Why it happens:** GitHub Actions' three affected tags in this phase — `golangci-lint-action@v9.3.0`, `trivy-action@v0.36.0`, `svu@v3.4.1` — are annotated; the other eight (`checkout`, `setup-go`, `gitleaks-action`, `sbom-action`, `build-push-action`, `login-action`, `setup-buildx-action`, `amannn/action-semantic-pull-request`) are lightweight, where ref SHA == commit SHA.
**How to avoid:** Always query both `refs/tags/<tag>` and `refs/tags/<tag>^{}`; if a second (peeled) line appears, use *that* SHA, not the first.
**Warning signs:** A pinned Action fails at workflow-run time with a "could not resolve reference" or checkout error referencing a SHA that doesn't correspond to any file tree matching the expected action.yml.

### Pitfall 3: `pull_request` vs `pull_request_target` for the Trivy PR-time scan
**What goes wrong:** If the image-build-and-scan job is triggered on `pull_request_target` (to get write-level secrets access for some later step), it also checks out and builds the PR's (possibly forked, untrusted) branch code with elevated permissions — a known GHA privilege-escalation pattern.
**Why it happens:** Some tutorials default every job to `pull_request_target` "to be safe" without realizing the risk profile differs per job.
**How to avoid:** The `checks`/`build-scan-push` jobs need no write-level secrets on PR runs (they don't push); trigger them on plain `pull_request` (read-only `GITHUB_TOKEN`, runs with the PR branch's own permissions, standard and safe). Only the merge-to-main path (`push` event on `main`, never attacker-controlled) needs the `packages: write` permission for the ghcr.io push.
**Warning signs:** A workflow that grants `permissions: packages: write` at the top level (workflow-wide) rather than scoped to the specific job/step that pushes — over-broad permissions on a job that also builds untrusted PR code.

### Pitfall 4: Trivy image-ref must reference the *locally loaded* tag, not a registry tag, for the PR-time scan
**What goes wrong:** Using `image-ref: ghcr.io/danielrpof/drop-tracker:pr-123` on a PR run — the image was never pushed (D-09 explicitly forbids pushing on PRs), so Trivy either errors trying to pull a nonexistent registry tag, or (if it falls back to some cached state) scans the wrong image entirely.
**Why it happens:** Copy-pasting a merge-to-main scan step's `image-ref` value into the PR-time step without changing it to the locally-`load`ed tag.
**How to avoid:** Use a plain local tag (e.g. `drop-tracker:scan`) for the `load: true` build + Trivy step; only the merge-to-main build+push step uses the real `ghcr.io/...:vX.Y.Z` tag.
**Warning signs:** PR checks pass suspiciously fast (Trivy scanning nothing / an old cached image) or fail with a registry-auth/not-found error.

### Pitfall 5: `svu next` run against a shallow checkout returns wrong results
**What goes wrong:** `actions/checkout` defaults to `fetch-depth: 1` (shallow clone) — `svu` needs the full commit history (and all tags) to correctly walk conventional-commit prefixes since the last tag.
**Why it happens:** GitHub Actions' checkout default optimizes for speed, not full-history tools.
**How to avoid:** The version-compute step's `actions/checkout` must set `fetch-depth: 0` and `fetch-tags: true` (or rely on the default full fetch if `fetch-depth: 0` already implies tags — verify at implementation time).
**Warning signs:** `svu next` returns the *current* version instead of an incremented one, or errors that no tags are found, even though `v0.1.0` was seeded per D-04.

## Code Examples

### `.pre-commit-config.yaml` addition (golangci-lint hook)
```yaml
# Source: golangci-lint's own .pre-commit-hooks.yaml, fetched and read directly
# this session (raw.githubusercontent.com/golangci/golangci-lint/master/.pre-commit-hooks.yaml)
# — three hook ids exist: `golangci-lint` (changed files only, --new-from-rev HEAD --fix),
# `golangci-lint-full` (whole repo), `golangci-lint-config-verify`. The existing gitleaks
# hook in this repo scans the staged diff by default, so `golangci-lint` (not `-full`)
# is the structurally-matching choice per CONTEXT.md's discretion note.
repos:
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.30.1
    hooks:
      - id: gitleaks
  - repo: https://github.com/golangci/golangci-lint
    rev: v2.12.2
    hooks:
      - id: golangci-lint
```

### `.trivyignore` format (D-08 — file created only when an actual finding requires it)
```
# Source: aquasecurity/trivy docs (CITED, corroborated across multiple 2026 sources)
# Format: bare CVE-ID line; a `#` comment on the line above documents the reason
# and date, mirroring D-08's "CVE ID plus a one-line dated reason" requirement.
# Trivy 0.50+ also supports an `exp:YYYY-MM-DD` suffix for a self-expiring entry.

# CVE-2024-XXXXX: no upstream fix available in alpine 3.24's musl package as of
# 2026-08-12; low exploitability in this container's network posture (no inbound
# ports beyond 8080/health). Re-evaluate when alpine 3.25 ships. — 2026-08-12
CVE-2024-XXXXX exp:2026-11-12
```

### docker-compose.yml `app:` service (D-10/D-11/D-12)
```yaml
# Source: pattern derived from this repo's own existing postgres service healthcheck
# convention (docker-compose.yml, verified) + D-10/D-11/D-12's exact wording.
services:
  postgres:
    # ...existing service, unchanged...

  app:
    build: .
    env_file: .env
    environment:
      # .env's DATABASE_URL points at localhost:5433 (the host-mapped port `go run`
      # uses locally, per Makefile's TEST_DATABASE_URL comment). Inside the compose
      # network, the app must reach postgres via the service name and its internal
      # (unmapped) port — do NOT "fix" this back to match .env; that would break the
      # container-network path. See 07-CONTEXT.md D-12.
      DATABASE_URL: postgres://drop_tracker:drop_tracker@postgres:5432/drop_tracker?sslmode=disable
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
```

### golangci-lint v2 minimal config (needed before CICD-01's CI step is meaningful — Pitfall 1)
```yaml
# Source: golangci-lint.run/docs/configuration/file/ (CITED, v2 schema per CLAUDE.md's
# own warning about v1-vs-v2 schema incompatibility)
version: "2"
linters:
  default: standard
run:
  timeout: 5m
```

## State of the Art

| Old Approach (CLAUDE.md, as written) | Current Approach (verified this session) | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `gitleaks/gitleaks-action@v2` | `gitleaks/gitleaks-action@v3` | v3.0.0 released 2026-05-30 [VERIFIED: WebFetch of releases page] | **Must update before implementation.** v2 stops working entirely 2026-09-16 (Node20 runner removal); "no changes to inputs/outputs/behavior" so the swap is mechanical — just the SHA and comment |
| `docker/build-push-action@v6` | `docker/build-push-action@v7.3.0` | v7.3.0 published 2026-07-01 [VERIFIED: GitHub API] | Should update; no known breaking input changes for this project's usage (image-ref/tags/push/context/cache-from/cache-to all stable across the v6→v7 boundary per changelog skim), but the planner/executor should sanity-check the v7 changelog if using any less-common input |
| `docker/login-action` (unpinned in CLAUDE.md) | `docker/login-action@v4.6.0` | published 2026-07-29 [VERIFIED: GitHub API] | CLAUDE.md never named a version — this research supplies one |
| `docker/setup-buildx-action` (absent from CLAUDE.md entirely) | `docker/setup-buildx-action@v4.2.0` required | — | CLAUDE.md's Installation table omits this Action, but it's required by `docker/build-push-action`'s GHA-cache pattern used in this phase's recommended job structure |
| `svu` v2-era flags (any copied tutorial predating v3) | `svu` v3.x — "most flags renamed" [CITED: caarlos0/svu README] | v3.0.0 (module path became `github.com/caarlos0/svu/v3`) | If the planner or executor copies an older `svu` CLI example from memory/search results, flags may not match v3.4.1's actual interface — verify against the v3 README, not older blog posts |

**Deprecated/outdated:**
- `gitleaks-action@v2`: functionally fine today but on a fixed retirement date (2026-09-16) that falls within this project's likely active-development window — treat the v2→v3 swap as in-scope now rather than a follow-up.
- Corepack-based pnpm installation in Node ≥ 26 images: no longer works (corepack removed from the Node distribution starting with Node 25) — use `npm install -g pnpm@<version>` instead.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `docker/build-push-action` v6→v7 has no breaking `with:` input changes relevant to this project's usage (image-ref/tags/push/context/cache-from/cache-to) | State of the Art, Standard Stack | Low — if wrong, the build-scan-push job fails fast at implementation/first-CI-run time with a clear action-input error, not a silent misbehavior; easy to catch and fix during planning verification |
| A2 | `pnpm@11.8.0` (this dev machine's installed version) is the correct version to pin inside the Dockerfile's Node stage, rather than a newer pnpm major | Standard Stack, Pattern 1 | Low-medium — if a newer pnpm major has an incompatible lockfile-version bump beyond `lockfileVersion: '9.0'`, `pnpm install --frozen-lockfile` fails loudly in CI; the fix is simply re-pinning, no silent data corruption |
| A3 | `docker/build-push-action`'s `load: true` + local-tag pattern, then a second `push: true` step reusing `type=gha` cache, is the correct/idiomatic way to satisfy D-09's "scan blocks push" within one job (vs. splitting into two jobs with artifact hand-off) | Architecture Patterns Pattern 2, Summary | Low — this is a widely-documented community pattern (multiple independent 2026 sources agree), but was not confirmed against `docker/build-push-action`'s own official README example set; if GHA cache reuse behaves unexpectedly, worst case is a slower (not incorrect) pipeline — the second build step still produces a correct image even if it doesn't hit cache |
| A4 | `golangci-lint` v2 pre-commit hook id `golangci-lint` (changed-files-only) is the right structural match for this repo's existing gitleaks hook, vs. `golangci-lint-full` | Code Examples | Low — this was explicitly left to Claude's discretion in CONTEXT.md; if the planner disagrees, swapping the hook id is a one-line change |

**If this table is empty:** N/A — see rows above. All high-risk claims (exact commit SHAs, exact current tool versions, exact `/health` route, exact env var names, exact Makefile target behavior) were tool-verified this session (`git ls-remote`, GitHub API, Docker Hub registry API, or direct `Read` of source files) and are tagged `[VERIFIED: ...]` inline above, not listed here.

## Open Questions

1. **Should the "checks" gate (lint/vet/test/gitleaks/PR-title) be one job with sequential steps or four parallel jobs?**
   - What we know: CONTEXT.md explicitly leaves this to Claude's discretion; PROJECT.md names the workflow "Full Pipeline" (singular file), not singular job.
   - What's unclear: Whether solo-dev CI-minutes economy (fewer, sequential steps = less runner spin-up overhead) or fail-fast parallelism (four independent jobs = a lint failure doesn't wait on a full `test-integration` run) is more valuable for this project's actual usage pattern.
   - Recommendation: Default to 4 parallel jobs under one workflow file (`lint`, `test`, `gitleaks`, `pr-title`), all required as branch-protection status checks, with `build-scan-push` gated on `needs: [lint, test, gitleaks]` (PR-title doesn't gate the build — it only needs to be correct by merge time, not before every push). This fails fast and matches how GitHub's own branch-protection UI expects discrete named checks.

2. **Does `actions/setup-go`'s `go-version` input need to exactly match `1.26` or should it read from `go.mod` via `go-version-file: go.mod`?**
   - What we know: `go.mod` pins `go 1.26` [VERIFIED: go.mod:3]; `actions/setup-go` supports a `go-version-file` input that reads the directive directly, avoiding version drift between `go.mod` and the workflow file.
   - What's unclear: Nothing substantive — this is a minor implementation choice, not a real gap.
   - Recommendation: Use `go-version-file: go.mod` so a future `go.mod` bump doesn't require a matching workflow edit.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | Local image build/test (D-10, docker-compose) | ✓ | 29.6.2 [VERIFIED: `docker --version` this session] | — |
| Docker Compose | `docker-compose up` local dev loop | ✓ | v5.3.1 (compose plugin) [VERIFIED: `docker compose version`] | — |
| git | SHA pinning verification, svu | ✓ | 2.41.0.windows.3 [VERIFIED] | — |
| make | All CI steps that call into Makefile targets | ✓ | GNU Make 4.4.1 [VERIFIED] | — |
| go | Go build stage parity check | ✓ | go1.26.5 windows/amd64 [VERIFIED] — matches `golang:1.26.5-alpine3.24` base image choice | — |
| node | Local reference only (Docker's own `node:26-alpine` stage is independent) | ✓ | v22.21.1 [VERIFIED] — does not need to match the container's Node 26; only matters if anyone runs `pnpm build` outside Docker | Container build is self-contained regardless |
| pnpm | Confirms which pnpm version this repo's lockfile is actually tested against | ✓ | 11.8.0 [VERIFIED] | Pin Dockerfile's `npm install -g pnpm@11.8.0` to match |
| gh CLI | Ad-hoc SHA/version re-verification at implementation time | ✓ | 2.93.0 [VERIFIED] | `git ls-remote` works without `gh` if unavailable elsewhere |

**Missing dependencies with no fallback:** None — every tool this phase needs locally is already installed on this dev machine.

**Missing dependencies with fallback:** None.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (existing) — this phase adds no new test framework; its "tests" are largely the pipeline's own behavior, exercisable only by a real GitHub Actions run |
| Config file | none — `.github/workflows/full-pipeline.yml` (new) is the artifact under test |
| Quick run command | `make test-short` |
| Full suite command | `make test-integration` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CICD-01 | lint/vet/test run on every push, before build | CI smoke (real push/PR) | none locally exercisable — `golangci-lint run`, `go vet ./...`, `make test-integration` each pass individually today; the *gate ordering* is only provable by a real workflow run | ❌ Wave 0 — `.golangci.yml` doesn't exist yet (Pitfall 1) |
| CICD-02 | gitleaks blocks on detected secret | CI smoke + local pre-commit (already proven, quick/260806-hfn) | `pre-commit run gitleaks --all-files` (local); CI half needs a real push | ✅ pre-commit half proven; ❌ CI workflow file itself |
| CICD-03 | multi-stage image builds, non-root, contains full stack | Manual/smoke: `docker build .` then `docker run --rm <img> whoami` should print a non-root user | `docker build -t drop-tracker:test . && docker run --rm drop-tracker:test whoami` | ❌ Wave 0 — Dockerfile doesn't exist |
| CICD-04 | Trivy blocks on CRITICAL/HIGH | CI smoke | `trivy image --severity CRITICAL,HIGH --exit-code 1 drop-tracker:test` (runnable locally with Trivy CLI installed, or via the CI step itself) | ❌ Wave 0 |
| CICD-05 | SBOM generated | CI smoke | `syft ghcr.io/danielrpof/drop-tracker:vX.Y.Z -o spdx-json` (post-push verification) | ❌ Wave 0 |
| CICD-06 | semver computed + tagged on merge | CI smoke (requires a real merge to main) | `svu next` (locally runnable against real git history to sanity-check before relying on it in CI) | ❌ Wave 0 |
| CICD-07 | image pushed to ghcr.io with semver tag | CI smoke + `docker pull ghcr.io/danielrpof/drop-tracker:vX.Y.Z` post-merge | manual verification after first real merge | ❌ Wave 0 |
| CICD-08 | Actions pinned to SHA | Static/grep check | `grep -n "uses: .*@[0-9a-f]\{40\}" .github/workflows/*.yml` should match every third-party `uses:` line; any `uses: owner/repo@v[0-9]` (bare tag) should fail this grep | ❌ Wave 0 |
| CICD-09 | docker-compose brings up app + postgres | Manual/smoke | `docker compose up --wait && curl -f http://localhost:8080/health` | ❌ Wave 0 — `app:` service doesn't exist yet |
| CICD-10 | pre-commit runs golangci-lint + gitleaks | Local hook run | `pre-commit run --all-files` | ✅ gitleaks half proven (quick/260806-hfn); ❌ golangci-lint hook entry |

**Honest caveat:** Unlike application-behavior requirements, most of CICD-01 through CICD-10 describe *pipeline* behavior that genuinely cannot be unit-tested — the only real verification is triggering an actual GitHub Actions run (a PR, then a merge to main) and observing the checks. The plan should budget an explicit "first real PR" and "first real merge to main" verification step rather than expecting `go test` to prove these requirements.

### Sampling Rate
- **Per task commit:** `make test-short`
- **Per wave merge:** `make test-integration`
- **Phase gate:** A real PR opened against this branch (exercises the full `checks` + PR-time `build-scan-push` path) and, once merged, a real push to `main` (exercises the version/SBOM/push path) — both are required before `/gsd-verify-work` can honestly close CICD-01 through CICD-10.

### Wave 0 Gaps
- [ ] `.golangci.yml` (v2 schema) — covers CICD-01, does not exist yet
- [ ] `Dockerfile` + `.dockerignore` — covers CICD-03, greenfield
- [ ] `.github/workflows/full-pipeline.yml` — covers CICD-01/02/04/05/06/07/08, greenfield
- [ ] `docker-compose.yml` `app:` service — covers CICD-09
- [ ] `.pre-commit-config.yaml` golangci-lint hook entry — covers CICD-10 (second half)

## Security Domain

### Applicable ASVS Categories

ASVS is primarily an *application* input/auth/session standard; this phase is a *build pipeline* — the closer-fitting lens is OWASP's CI/CD Security Top 10 / supply-chain integrity, cross-referenced against ASVS where it does apply (secrets handling, dependency integrity).

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V14 Configuration | yes | Non-root container user with fixed UID/GID (D-02); no secrets baked into image layers or Dockerfile `ARG`/`ENV` |
| V10 Malicious Code | yes | GitHub Action commit-SHA pinning (CICD-08) — the direct ASVS-adjacent control against a compromised/hijacked third-party build dependency |
| V1 Architecture | yes (partial) | Documented `.trivyignore` escape hatch (D-08) — no silent vulnerability suppression, matches ASVS's "documented risk acceptance" spirit |
| V2 Authentication | no | No auth surface introduced by this phase |
| V5 Input Validation | no | No new user-facing input surface |
| V6 Cryptography | no | No new cryptographic operations |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Compromised/hijacked third-party GitHub Action (tag re-pointed to malicious code) | Tampering | Pin every third-party Action to a verified commit SHA (CICD-08, table above), not a mutable tag |
| Secret exfiltration via `pull_request_target` + untrusted-code checkout combo | Elevation of Privilege / Information Disclosure | Use plain `pull_request` for jobs that don't need write-level secrets and don't check out untrusted code with elevated permissions (PR-title check, PR-time scan-only build) — see Pitfall 3 |
| Secret baked into a committed image layer (e.g. accidental `ENV DISCORD_WEBHOOK_URL=...` in Dockerfile, or `.env` copied into build context) | Information Disclosure | `.dockerignore` excludes `.env`/`.env.*`; config is read exclusively from the process environment at runtime [VERIFIED: internal/config/config.go doc comment], never baked at build time |
| Vulnerable base-image or transitive OS/Go-module package (CVE in `alpine`, `golang`, or a Go dependency) | Tampering | Trivy scan blocking on `CRITICAL,HIGH` (D-07), gated before any push (D-09) |
| Container running as root inside a compromised process | Elevation of Privilege | Fixed non-root UID/GID `10001:10001` (D-02) — even if the process is compromised, it cannot write outside its own permissions or bind privileged ports |
| Committed secret in source (API key, webhook URL, DB password) reaching a public repo | Information Disclosure | gitleaks at both pre-commit (local, CICD-10) and CI (backstop, CICD-02) — two independent enforcement points, mirroring the precedent from quick/260806-hfn |

## Sources

### Primary (HIGH confidence — VERIFIED this session via direct tool calls)
- `git ls-remote` against 10 GitHub Action repos — exact commit SHAs for CICD-08 pinning table
- GitHub REST API `repos/{owner}/{repo}/releases/latest` — current version confirmation for `golangci-lint`, `golangci-lint-action`, `trivy-action`, `docker/build-push-action`, `docker/login-action`, `docker/setup-buildx-action`, `svu`, `amannn/action-semantic-pull-request`, `actions/checkout`, `actions/setup-go`
- Docker Hub registry API `hub.docker.com/v2/repositories/library/{golang,node,alpine}/tags` — exact base image tag confirmation
- `raw.githubusercontent.com/golangci/golangci-lint/master/.pre-commit-hooks.yaml` — exact pre-commit hook ids read directly
- Local shell probes (`docker --version`, `docker compose version`, `git --version`, `make --version`, `go version`, `node --version`, `pnpm --version`, `gh --version`) — Environment Availability table
- `Read` of `C:/CodeProjects/drop-tracker/go.mod`, `Makefile`, `docker-compose.yml`, `.pre-commit-config.yaml`, `internal/config/config.go`, `internal/httpserver/health.go`, `internal/httpserver/server.go`, `internal/webassets/embed.go`, `cmd/server/main.go`, `web/package.json`, `web/pnpm-workspace.yaml`, `web/pnpm-lock.yaml` — all in-repo claims tagged `[VERIFIED: <path>:<line>]` above

### Secondary (MEDIUM confidence — CITED from official docs via WebFetch/WebSearch)
- `github.com/aquasecurity/trivy-action` README — `image-ref`/`severity`/`exit-code`/`trivyignores` input names
- `github.com/golangci/golangci-lint-action` README — `version`/`working-directory`/`args` usage example
- `github.com/gitleaks/gitleaks-action` v2.md and release notes — `GITLEAKS_LICENSE` requirement (org-only), v2→v3 migration notice
- `github.com/anchore/sbom-action` README/action.yml — `image`/`format`/`output-file` inputs
- `github.com/caarlos0/svu` README — `svu next`, v3 module path, flag-rename note
- Multiple 2026-dated community sources (triangulated) — `docker/build-push-action` `load:true`/`push:false` scan-then-push pattern, `.trivyignore` `exp:YYYY-MM-DD` format, Node 26 corepack removal

### Tertiary (LOW confidence — flagged for planner awareness only)
- None retained as load-bearing — all findings that started as single-source WebSearch summaries were either cross-verified via a direct API/registry call or explicitly logged in the Assumptions table above.

## Metadata

**Confidence breakdown:**
- Standard stack (versions/SHAs): HIGH — every version number and commit SHA in the Standard Stack / Package Legitimacy Audit tables was confirmed via direct tool call (`git ls-remote`, GitHub API, Docker Hub API) this session, not inferred from training data
- Architecture (Dockerfile stage layout, job structure): MEDIUM — synthesized from verified in-repo facts (Makefile, embed.go, config.go) plus well-established, cross-corroborated community patterns for the build-scan-push sequencing; not verified against this specific repo's actual CI run (doesn't exist yet)
- Pitfalls: MEDIUM-HIGH — five of five pitfalls trace to either a verified in-repo fact (folded todo, embed.go ordering) or a verified external fact (corepack removal, annotated-tag SHA behavior, pull_request_target risk pattern)

**Research date:** 2026-08-12
**Valid until:** 7 days for the exact commit SHAs and "current version" claims (fast-moving — Actions publish frequently; re-verify via `git ls-remote`/GitHub API before implementation if more than a week has passed); 30 days for architectural patterns (Dockerfile stage layout, job sequencing) which are stable regardless of tool version churn
