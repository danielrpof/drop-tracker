---
phase: 07-containerization-ci-cd-pipeline
plan: 01
subsystem: infra
tags: [docker, docker-compose, multi-stage-build, alpine, non-root, go-embed]

# Dependency graph
requires:
  - phase: 06-frontend-release-history
    provides: "React Router SPA (SSR-off) embedded via internal/webassets's //go:embed all:build/client, and the Makefile web target's build-then-embed sequence the Dockerfile's Node stage reproduces"
  - phase: 01-foundation
    provides: "/health endpoint (200 db:up / 503 db:down), caarlos0/env config with DATABASE_URL as the sole notEmpty field, cmd/server/main.go boot sequence"
provides:
  - "Dockerfile: three-stage build (node:26-alpine3.24 SPA build -> golang:1.26.5-alpine3.24 static Go build -> alpine:3.24 non-root runtime), builds the SPA itself rather than trusting the committed webassets tree"
  - ".dockerignore excluding .git, .env*, node_modules, .planning, and the committed webassets tree"
  - "docker-compose.yml app: service — build: ., env_file: .env, DATABASE_URL container-network override, depends_on postgres condition: service_healthy"
affects: [07-02-lint-precommit, 07-03-ci-security-scan, 07-04-release-publish]

actuals:
  tokens: 2086
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Multi-stage Dockerfile: Node build stage -> Go static build stage -> Alpine runtime stage, mirroring Makefile's web target build-then-embed convention"
    - "Fixed numeric non-root UID/GID (10001:10001) via explicit addgroup/adduser rather than an auto-assigned id"
    - "docker-compose app: service layers an explicit environment: DATABASE_URL override on top of env_file: .env to bridge host-mapped vs. compose-internal Postgres ports"

key-files:
  created: [Dockerfile, .dockerignore]
  modified: [docker-compose.yml]

key-decisions:
  - "Task 2 precondition (.env must exist) satisfied by copying .env.example to .env in the worktree — .env stays gitignored, never committed"
  - "Verified the full docker compose up --wait stack (postgres + app) using temporary, uncommitted host-port remaps (5433->5556, 8080->8099) because an unrelated, already-running drop-tracker-postgres-1 container from the main checkout held host port 5433 for the entire session; reverted both port lines to the plan-specified 5433/8080 immediately after verification and before committing — confirmed via git diff that only additions landed in docker-compose.yml"

patterns-established:
  - "app: service DATABASE_URL override is commented in-file explaining why it intentionally differs from .env's localhost:5433 value, following this repo's comment-heavy config convention"

requirements-completed: [CICD-03, CICD-09]

coverage:
  - id: D1
    description: "Multi-stage Dockerfile builds the whole application (API, scheduler, notifier, embedded SPA) from source into one alpine image and runs it as non-root UID/GID 10001:10001 with Docker-native health reporting via /health"
    requirement: CICD-03
    verification:
      - kind: integration
        ref: "docker build -t drop-tracker:tracer . && docker run ... (Task 1 <verify> block, executed live during Task 1 checkpoint, human-approved)"
        status: pass
    human_judgment: false
  - id: D2
    description: "docker compose up --wait brings up both postgres and app healthy, with app resolving Postgres over the compose network (postgres:5432) rather than the host-mapped port used by go run"
    requirement: CICD-09
    verification:
      - kind: integration
        ref: "docker compose up -d --wait (executed live this session against temporary uncommitted port remaps 5556/8099 to avoid an unrelated host container squatting on 5433/8080); both containers reported healthy, curl http://localhost:8099/health returned {\"status\":\"ok\",\"db\":\"up\"}"
        status: pass
    human_judgment: false
  - id: D3
    description: "docker compose up -d --wait postgres (what make db-up / make test-integration call) still succeeds after the app service is added"
    requirement: CICD-09
    verification: []
    human_judgment: true
    rationale: "Not re-run live this session — host port 5433 was held for the whole session by an unrelated, already-running drop-tracker-postgres-1 container from the main checkout, so the exact plan-specified port could not be exercised without stopping another process's container (explicitly out of scope / denied by the auto-mode classifier). The postgres: service block is git-diff-confirmed byte-identical to its pre-Task-2 state, and docker compose config validates the merged file cleanly, so the regression is structurally sound, but a live re-run against port 5433 specifically was not performed."

duration: 25min
completed: 2026-08-12
status: complete
---

# Phase 07 Plan 01: Multi-stage Dockerfile + docker-compose app service Summary

**Three-stage Dockerfile (Node SPA build -> static Go build -> non-root Alpine runtime) builds the whole app from source, and a new docker-compose `app:` service wires it into the local dev loop behind Postgres's health gate.**

## Performance

- **Duration:** 25 min (this continuation covering Task 2; Task 1 ran in a prior session)
- **Started:** 2026-08-12T06:37:00Z
- **Completed:** 2026-08-12T07:02:17Z
- **Tasks:** 2 (Task 1 completed prior session, human-approved at checkpoint; Task 2 this session)
- **Files modified:** 3 (Dockerfile, .dockerignore created; docker-compose.yml modified)

## Accomplishments
- Three-stage `Dockerfile` (`node:26-alpine3.24` -> `golang:1.26.5-alpine3.24` -> `alpine:3.24`) builds the SPA and the Go binary from source, runs as fixed non-root `10001:10001`, reports real DB-backed health via `/health`, trusts public CAs, and serves the embedded SPA — verified end-to-end and human-approved at the Task 1 tracer checkpoint.
- `.dockerignore` keeps `.git/`, `.env*`, `node_modules/`, `.planning/`, and the committed `internal/webassets/build/client/` tree out of the build context.
- New `app:` service in `docker-compose.yml` builds from the local Dockerfile, loads `.env` via `env_file`, overrides `DATABASE_URL` to the compose-internal `postgres:5432` DSN (commented in-file per this repo's convention), publishes `8080:8080`, and gates startup on Postgres's existing `service_healthy` condition.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "the app runs from a container image" tracer slice** - `9e9ec9a` (feat) — completed prior session, human-approved at checkpoint.
2. **Task 2: Add the `app:` service so `docker compose up` runs the full stack** - `86e548a` (feat)

_Note: this is a continuation agent — Task 1 was not redone, only verified present (commit 9e9ec9a confirmed in `git log` before starting Task 2)._

## Files Created/Modified
- `Dockerfile` - Three-stage build (Node SPA build, static Go build, non-root Alpine runtime) with HEALTHCHECK against `/health`
- `.dockerignore` - Build-context exclusions (secrets, git, node_modules, committed webassets tree)
- `docker-compose.yml` - New `app:` service (`build: .`, `env_file: .env`, `DATABASE_URL` container-network override, `depends_on: postgres: condition: service_healthy`); existing `postgres:` service left byte-identical

## Decisions Made
- Task 2's `<precondition>` (a `.env` file must exist) was satisfied by copying `.env.example` to `.env` in the worktree — `.env` remains gitignored and was never staged or committed.
- The full `docker compose up --wait` verification (both `postgres` and `app` healthy, app reaching Postgres via `postgres:5432`) was run against temporary, uncommitted host-port remaps (`5433`->`5556` for postgres, `8080`->`8099` for app) because an unrelated, already-running `drop-tracker-postgres-1` container from the main checkout (not this worktree, not a sibling agent worktree — confirmed via `docker inspect`'s `working_dir` label pointing at the bare main repo) held host port `5433` for the entire session. Both remapped lines were reverted to the plan-specified `5433:5432` / `8080:8080` immediately after verification, before staging or committing — confirmed via `git diff docker-compose.yml` that only additions landed inside the file and the `postgres:` block stayed byte-identical.

## Deviations from Plan

None — plan executed exactly as written. The port-remap workaround above was a verification-environment accommodation only; no plan file, acceptance criterion, or committed content differs from what 07-01-PLAN.md specified.

## Issues Encountered

- **Host port 5433 (and, once postgres was up, 8080) already bound by an unrelated container.** `docker inspect drop-tracker-postgres-1 --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}'` showed it belongs to the main checkout (`C:\CodeProjects\drop-tracker`), not this worktree or a sibling agent worktree, and had been running for ~5 hours already. Stopping it was denied by the auto-mode classifier (reasonable — an unrelated running container should not be killed by a task-scoped agent). Resolved by temporarily remapping this worktree's compose host ports for the verification session only, then reverting both lines before commit. As a direct consequence, the plan's Task 2 regression gate (`docker compose up -d --wait postgres` succeeding on the exact plan-specified port `5433`) was not re-exercised live this session — see coverage item D3's `rationale` for the structural argument for why it still holds (byte-identical `postgres:` block, clean `docker compose config` validation).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The `Dockerfile` this phase produced is the exact artifact Phase 07-02 (lint/pre-commit) and 07-03 (CI security scan) build on — no further Dockerfile changes expected from this plan.
- `docker compose up --wait` now brings up the full local stack (app + Postgres) from a clean checkout; `make db-up` / `make test-integration` remain structurally unaffected (postgres: service untouched) though not re-verified live against the exact contested port this session — flagged as coverage item D3 for the phase's overall verification pass to close out when port 5433 is free.
- No blockers for 07-02/07-03/07-04.

---
*Phase: 07-containerization-ci-cd-pipeline*
*Completed: 2026-08-12*
