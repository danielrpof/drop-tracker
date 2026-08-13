# Phase 7: Containerization & CI/CD Pipeline - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

The app ships as a single, scanned, versioned, non-root Docker image (API + scheduler + notifier + embedded SPA) built via a multi-stage Dockerfile, published through a GitHub Actions pipeline that lints, vets, tests, and security-scans every push, and produces a semantically-versioned, SBOM-accompanied image on ghcr.io on every merge to main. `docker-compose` brings the full stack (app + Postgres) up for local dev. A pre-commit hook runs golangci-lint (joining the already-installed gitleaks hook) locally before any commit reaches the pipeline. No VPS/SSH deploy step (deferred, v2/DPLY-01), no Kubernetes, no split microservices, no Prometheus/Grafana — this phase is build/scan/publish/local-run only, per PROJECT.md's locked constraints.

</domain>

<decisions>
## Implementation Decisions

### Docker base image & non-root setup
- **D-01:** Final Dockerfile stage uses `alpine` (not distroless, not debian-slim) — small footprint, retains a shell for debugging via `docker exec`, well-documented, works cleanly with the existing `CGO_ENABLED=0` static Go build.
- **D-02:** Non-root user uses a fixed numeric UID/GID (e.g. `10001:10001`) via explicit `addgroup -g 10001 app && adduser -D -u 10001 -G app app` — not an auto-assigned UID. Deterministic across rebuilds, referenceable later if a VPS/orchestrator SecurityContext is added.
- **D-03:** Dockerfile includes a `HEALTHCHECK` instruction against the existing `/health` endpoint (Phase 1) — alpine's busybox `wget` is sufficient (`wget -qO- http://localhost:$PORT/health || exit 1`). Makes `docker ps` / compose show real container health, not just process-alive.

### Versioning bootstrap & image visibility
- **D-04:** First release is tagged `v0.1.0`, manually seeded once the pipeline lands; every subsequent merge to main lets `svu` compute the next tag from conventional-commit prefixes in the merged history. This signals "evolving pre-1.0 portfolio project," not a completed v1.0.0 release, even though STATE.md's milestone label is "v1.0" (that label is about the requirements milestone, not the semver line).
- **D-05:** The ghcr.io image package is public. The GitHub repo (`danielrpof/drop-tracker`) is confirmed already public (`gh repo view`), so the package inherits public visibility by default with no extra manual step — a reviewer/recruiter can `docker pull` without authenticating.
- **D-06:** CI adds a lightweight PR-title conventional-commit check (e.g. `amannn/action-semantic-pull-request`), not full per-commit commitlint and not zero enforcement. Rationale discussed live: full commitlint (pre-commit + CI) is standard for multi-contributor teams but is friction with low payoff for a solo dev; best-effort/no-gate risks svu computing a silently wrong version bump from a malformed message; PR-title-only validates the one message that actually drives the squash-merge version bump, without gating every individual local commit.

### Vulnerability gate strictness
- **D-07:** Trivy blocks the pipeline on `CRITICAL,HIGH` (matches CLAUDE.md's Technology Stack doc exactly — not a deviation).
- **D-08:** Escape hatch for an unfixable CRITICAL/HIGH finding (no upstream patch available) is a committed `.trivyignore` file, with each entry carrying the CVE ID plus a one-line dated reason. Mirrors the precedent already set by `quick/260806-hfn`'s documented-acceptance pattern for the 4 pre-existing gitleaks findings (no silent suppression, no history rewrite).
- **D-09:** PRs also build the full multi-stage image and run Trivy against it (build-only, no push to ghcr.io) — not just lint/vet/test/gitleaks-on-source. Catches an image-level vulnerability before merge ("shift-left"), at the cost of PR checks taking image-build time. Push to ghcr.io + SBOM + semver tag still only happens on merge to main.

### docker-compose dev-loop shape
- **D-10:** The app service in `docker-compose.yml` builds from the local Dockerfile every `docker-compose up` (`build: .` context), not a prebuilt/pulled ghcr.io tag. Exercises the real multi-stage build (including the Node/web stage) locally, surfacing Dockerfile bugs before CI does, at the cost of a slower iteration loop than `make run`/`go run` outside Docker (both remain valid/expected for fast day-to-day dev).
- **D-11:** The app service loads env vars via `env_file: .env` — the same gitignored file `.env.example` already documents and that `go run` uses locally. No second env file to maintain.
- **D-12:** The app service's `environment:` block explicitly overrides `DATABASE_URL` to the container-network DSN (`postgres://drop_tracker:drop_tracker@postgres:5432/drop_tracker?sslmode=disable`), layered on top of `env_file: .env`. This is required because `.env`'s `DATABASE_URL` points at `localhost:5433` (the host-mapped port `go run` uses per the existing docker-compose.yml/Makefile comment about avoiding port collisions) — inside the compose network the app must reach Postgres via the service name and its internal (unmapped) port, `postgres:5432`, not `localhost:5433`. Comment this override in docker-compose.yml so a future reader doesn't "fix" it back to matching `.env`.

### Claude's Discretion
- Exact multi-stage Dockerfile layout (number/order of stages: Node/pnpm build stage for `web/`, Go build stage, final alpine runtime stage) — follows the existing `Makefile web` target's build-then-embed convention, adapted so the image builds the SPA itself rather than trusting the committed `internal/webassets/build/client/` tree.
- Exact GitHub Actions workflow file/job structure (single "Full Pipeline" workflow vs. split jobs within it) — PROJECT.md's Active items name it "GitHub Actions 'Full Pipeline'" (singular), implementation detail is job/step decomposition within that.
- SBOM format (spdx-json vs cyclonedx-json) — CLAUDE.md's Technology Stack doc lists spdx-json as most broadly tooling-compatible; no strong preference expressed, default to that unless research surfaces a reason otherwise.
- Exact golangci-lint pre-commit hook scope (lint only changed/staged files vs. whole repo) — standard golangci-lint pre-commit integration behavior, follows whatever `.pre-commit-config.yaml`'s existing gitleaks-hook precedent suggests structurally.
- Pinning every security-sensitive GitHub Action to a commit SHA (CICD-08) — mechanical, no decision needed, just execution discipline during planning/implementation.
- Whether the CI test job runs `make test-integration` directly or an equivalent explicit `-p 1` invocation — behavior is locked (see Folded Todos below), wire-level command is Claude's discretion.

### Folded Todos
- **`.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md`** ("Fix flaky tests under parallel `go test ./...`") — folded into this phase's scope. The CI test step must not use bare `go test ./...` (Go's default package-level parallelism against the shared integration Postgres instance); it must use `make test-integration` (which already pins `-p 1`) or an equivalent explicit `-p 1` invocation, so the known notifier-timing/poller-shared-DB flakiness documented in the todo never surfaces in the pipeline. The todo's own root-cause options (fake clock, per-package DB isolation) remain open/not required — `-p 1` in CI is accepted as sufficient for this phase, not a full fix of the underlying sensitivity.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & Roadmap
- `.planning/REQUIREMENTS.md` §"CI/CD Pipeline" — CICD-01 through CICD-10 (this phase's full requirement set), including CICD-10's note that gitleaks pre-commit is already done and only golangci-lint pre-commit remains.
- `.planning/ROADMAP.md` §"Phase 7: Containerization & CI/CD Pipeline" — goal and the 5 success criteria (test/lint gate, secret+vuln scan gate, semver+SBOM+push on merge, single non-root multi-stage image runnable via docker-compose, pinned Actions + pre-commit).
- `.planning/PROJECT.md` — Constraints (registry = ghcr.io/GITHUB_TOKEN, single Go binary/service, secrets via env only), Active requirements list, Out of Scope (no k8s, no Terraform, no split microservices, VPS SSH deploy deferred to v2/DPLY-01).

### Tech stack (already locked — see CLAUDE.md's Technology Stack section for full rationale/version table)
- `.claude/CLAUDE.md` — golangci-lint v2.12.2 (v2 config schema) + `golangci-lint-action@v9.3.0`; gitleaks v8.30.1 + `gitleaks/gitleaks-action@v2`; `aquasecurity/trivy-action` v0.36.0 (fs scan + image scan, `severity: CRITICAL,HIGH`, `exit-code: 1` — matches D-07); `anchore/sbom-action` v0.24.0 (syft v1.42.3); `caarlos0/svu` for semver from conventional commits; plain `docker/build-push-action@v6` + `docker/login-action` to ghcr.io (GoReleaser explicitly rejected as overkill); all third-party Actions pinned to commit SHAs (CICD-08).

### Existing local assets this phase wires into CI
- `Makefile` — `test-integration` target (already `-p 1`, D-13/Folded Todos), `sqlc-check`, `web` target (build-then-embed convention for the SPA), `hooks` target (installs the `pre-commit` framework + git hook shim).
- `.pre-commit-config.yaml` — existing gitleaks hook (rev `v8.30.1`) with an explicit comment marking golangci-lint as "intentionally deferred to Phase 07" — this phase adds that second hook to the same file.
- `docker-compose.yml` — existing `postgres` service (port remapped to 5433 per the documented port-collision incident in `.planning/debug/resolved/notify-pass-hangs-forever.md`); this phase adds the `app` service (D-10/D-11/D-12).
- `.env.example` — documents every config var the app service's env_file needs (D-11); `DATABASE_URL` there is the host-mapped `localhost:5433` form D-12's override exists to correct for the container network.
- `web/package.json` — pnpm-based React Router 7 SPA build (`react-router build` → `web/build/client`), consumed today by `Makefile`'s `web` target and `internal/webassets`'s `go:embed`; the Dockerfile's Node build stage reproduces this build step instead of trusting the committed `internal/webassets/build/client/` tree.
- `go.mod` — `go 1.26` toolchain directive; the Dockerfile's Go build stage must match.

### Related prior-phase context
- `.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md` — folded, see Folded Todos above.
- `.planning/debug/resolved/notify-pass-hangs-forever.md` — background for why docker-compose's Postgres is on 5433, relevant to D-12's DSN override reasoning.

No additional external specs/ADRs beyond the above — CLAUDE.md's Technology Stack section already functions as the locked tooling reference for this phase.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Makefile`'s `test-integration` target — already solves the flaky-parallel-test problem (`-p 1`); CI's test step should invoke this directly rather than reinventing the flag.
- `Makefile`'s `web` target — the exact build steps (`pnpm install --frozen-lockfile`, `pnpm run build`, copy `web/build/client`) the Dockerfile's Node stage needs to mirror.
- `.pre-commit-config.yaml` — existing structure/comments already anticipate the golangci-lint hook being added here; extend, don't replace.
- `docker-compose.yml`'s `postgres` service — healthcheck pattern (`pg_isready`, interval/timeout/retries) is a direct model for D-03's app-service `HEALTHCHECK`.

### Established Patterns
- Every prior phase's Makefile targets are the source of truth CI steps should call into (not reimplement) — e.g. `sqlc-check`, `test-integration` — keeping "works the same locally and in CI" a real property, not just a goal.
- Config is env-var-only everywhere (`caarlos0/env`), secrets never committed — the Dockerfile/compose env handling (D-11/D-12) must not break this by baking any secret into an image layer or a committed compose value.

### Integration Points
- New `.github/workflows/*.yml` (doesn't exist yet — greenfield for this phase).
- New `Dockerfile` at repo root (doesn't exist yet).
- `docker-compose.yml` gets a new `app:` service added alongside the existing `postgres:` service.
- `.pre-commit-config.yaml` gets a second hook entry (golangci-lint) alongside the existing gitleaks hook.
- New `.trivyignore` at repo root (D-08), created only if/when an actual unfixable finding requires it — not pre-created empty.

</code_context>

<specifics>
## Specific Ideas

The recurring theme across this discussion was "make the pipeline demonstrate real DevOps judgment, not just tick boxes" — several answers explicitly traded a small amount of rigor for solo-dev practicality (PR-title-only commit linting instead of full commitlint; alpine instead of distroless for debuggability) while others held the line on rigor where the project's own stated purpose (CI/CD practice) demands it (CRITICAL+HIGH Trivy gate matching CLAUDE.md exactly, PR-time image scanning rather than only-at-merge, a documented `.trivyignore` rather than silent suppression). The user asked mid-discussion what's actually standard in production for commit-message enforcement, wanting the real-world tradeoff before deciding — the middle-ground PR-title check was chosen specifically because it demonstrates the underlying understanding (what actually drives svu's version math) without adding friction to every local commit.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

### Reviewed Todos (not folded)
- **SearchBox AbortController never cancels the underlying fetch** — frontend bug (Phase 6 area), unrelated to containerization/CI-CD. Left in pending todos for a future quick-task.
- **EventCard crashes History route on unrecognized event_type** — frontend bug (Phase 6 area), unrelated to this phase. Left in pending todos.
- **guestFeatureHref missing encodeURIComponent on external_id** — frontend bug (Phase 6 area), unrelated to this phase. Left in pending todos.

</deferred>

---

*Phase: 7-Containerization & CI/CD Pipeline*
*Context gathered: 2026-08-12*
