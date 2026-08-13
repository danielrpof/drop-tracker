# Phase 7: Containerization & CI/CD Pipeline - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-12
**Phase:** 07-containerization-ci-cd-pipeline
**Areas discussed:** Docker base image & non-root setup, Versioning bootstrap & image visibility, Vulnerability gate strictness, docker-compose dev-loop shape

---

## Pending Todos Cross-Reference

| Todo | Score | Decision |
|------|-------|----------|
| Fix flaky tests under parallel `go test ./...` | 0.6 | Folded — CI test step must use `-p 1` (via `make test-integration`) |
| SearchBox AbortController never cancels the underlying fetch | 0.7 | Reviewed, not folded — frontend bug, out of scope for this phase |
| EventCard crashes History route on unrecognized event_type | 0.5 | Reviewed, not folded — frontend bug, out of scope for this phase |
| guestFeatureHref missing encodeURIComponent on external_id | 0.5 | Reviewed, not folded — frontend bug, out of scope for this phase |

---

## Docker base image & non-root setup

| Option | Description | Selected |
|--------|-------------|----------|
| distroless | Smallest attack surface, no shell, built-in :nonroot user (65532) | |
| alpine | Small (~5MB), musl libc, has a shell for debugging, needs explicit adduser | ✓ |
| debian-slim | Larger (~80MB), glibc matches Go builder exactly, full apt tooling | |

**User's choice:** alpine
**Notes:** Chosen for debuggability of the demo (docker exec sh) balanced against distroless's smaller footprint.

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed numeric UID/GID (e.g. 10001:10001) | Deterministic across rebuilds, referenceable in future SecurityContext | ✓ |
| Named user, auto-assigned UID | Simpler line, UID not guaranteed stable | |

**User's choice:** Fixed numeric UID/GID (10001:10001)

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, add HEALTHCHECK | Uses existing /health endpoint via wget; docker ps/compose show real health | ✓ |
| No, skip it | /health already exposed for external orchestration; Docker-level check redundant | |

**User's choice:** Yes, add HEALTHCHECK

---

## Versioning bootstrap & image visibility

| Option | Description | Selected |
|--------|-------------|----------|
| Seed v0.1.0 manually, let svu take it from there | Signals "evolving pre-1.0," typical for a growing solo project | ✓ |
| Let svu auto-compute from empty history | Same end state as seeding, no manual step | |
| Seed v1.0.0 directly | Matches STATE.md's "v1.0" milestone label; signals complete v1 | |

**User's choice:** Seed v0.1.0 manually

| Option | Description | Selected |
|--------|-------------|----------|
| Public | Anyone can docker pull without auth — useful for portfolio review | ✓ |
| Private | Only accounts with explicit access can pull | |

**User's choice:** Public
**Notes:** Confirmed via `gh repo view` that the GitHub repo is already public, so the ghcr.io package inherits public visibility by default — no extra manual step.

**Follow-up question from user:** "What is the standard in a production environment?" (re: conventional commit enforcement)
**Claude's answer:** Multi-contributor teams typically enforce via commitlint on PRs (often PR-title only with squash-merge); solo devs often skip enforcement entirely since they control their own commit hygiene; a common middle ground for public/portfolio repos is a PR-title-only conventional-commit check (e.g. `amannn/action-semantic-pull-request`), validating only the message that actually drives the version bump.

| Option | Description | Selected |
|--------|-------------|----------|
| Best-effort, no enforcement | svu parses whatever exists; simplest, but a bad commit could silently mis-bump | |
| PR-title-only conventional-commit check | Validates the squash-merge message without gating every commit | ✓ |
| Full commitlint on every commit | Most rigorous, most friction for a solo dev | |

**User's choice:** PR-title-only conventional-commit check

---

## Vulnerability gate strictness

| Option | Description | Selected |
|--------|-------------|----------|
| CRITICAL + HIGH | Matches CLAUDE.md's Technology Stack doc exactly; more realistic gate | ✓ |
| CRITICAL only | Fewer false-alarm blocks, but deviates from what CLAUDE.md already documents | |

**User's choice:** CRITICAL + HIGH

| Option | Description | Selected |
|--------|-------------|----------|
| .trivyignore with documented entries (CVE ID + reason) | Visible, auditable, mirrors the gitleaks documented-acceptance precedent | ✓ |
| No escape hatch — pipeline stays red until a fix exists | Maximum rigor, could genuinely block progress on an unrelated third-party CVE | |

**User's choice:** .trivyignore with documented entries

| Option | Description | Selected |
|--------|-------------|----------|
| Build + scan on PRs too, push only on merge to main | Catches image vulnerabilities before merge (shift-left) | ✓ |
| Image build/scan only on merge to main | Faster PR feedback, vulnerable image could block main only after merge | |

**User's choice:** Build + scan on PRs too, push only on merge to main

---

## docker-compose dev-loop shape

| Option | Description | Selected |
|--------|-------------|----------|
| build: . every time | Exercises the real multi-stage build locally, catches Dockerfile bugs early | ✓ |
| Smoke-test only — pull/run a prebuilt tag | Day-to-day dev stays on make run; compose proves shipped image works | |

**User's choice:** build: . every time

| Option | Description | Selected |
|--------|-------------|----------|
| env_file: .env | One file for both go run and docker-compose, no duplication | ✓ |
| Inline environment: block with placeholder values | More visible in compose file, risks a real secret being pasted in | |

**User's choice:** env_file: .env

| Option | Description | Selected |
|--------|-------------|----------|
| Override DATABASE_URL in the app service's environment: block | One clearly-commented override on top of env_file: .env | ✓ |
| Separate .env.docker file | A full second file to keep in sync | |

**User's choice:** Override DATABASE_URL in the app service's environment: block
**Notes:** Needed because `.env`'s DATABASE_URL points at the host-mapped `localhost:5433`, but inside the compose network the app must reach Postgres via the service name and internal port `postgres:5432`.

---

## Claude's Discretion

- Exact multi-stage Dockerfile layout (stage count/order for Node/web build, Go build, alpine runtime)
- Exact GitHub Actions workflow file/job structure within the single "Full Pipeline" workflow
- SBOM format (spdx-json vs cyclonedx-json) — defaults to spdx-json per CLAUDE.md unless research says otherwise
- Exact golangci-lint pre-commit hook scope (changed files vs. whole repo)
- Mechanical execution of pinning every security-sensitive Action to a commit SHA (CICD-08)
- Exact CI test invocation (make test-integration vs. equivalent explicit -p 1 command)

## Deferred Ideas

None — discussion stayed within phase scope. Three reviewed-but-not-folded todos (frontend bugs) remain in the pending todos backlog; see Pending Todos Cross-Reference above and CONTEXT.md's Deferred section.
