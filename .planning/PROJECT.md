# drop-tracker

## What This Is

A Go-based release tracker for hip-hop, reggaeton, and R&B: users maintain a watchlist of artists (and later albums/producers) via a React UI, a scheduler polls MusicBrainz and Deezer for those artists, diffs results against a Postgres "seen" store to detect new releases, guest features, and deluxe/tracklist changes, and posts alerts to a Discord webhook. The primary purpose is a portfolio piece for practicing real CI/CD and DevOps pipelines — the music-tracking domain is the vehicle, the pipeline maturity is the point.

## Core Value

A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice (lint, test, security scan, SBOM, versioned image publish, and eventually automated deploy).

## Requirements

### Validated

- ✓ `/health` endpoint for liveness/readiness — Phase 01
- ✓ sqlc-generated type-safe DB queries + golang-migrate schema migrations — Phase 01
- ✓ Structured (slog-based) JSON logging — Phase 01
- ✓ `.env.example` documenting all config, secrets via env vars only — nothing real committed — Phase 01
- ✓ Watchlist CRUD API — add/remove/list artists, DB-backed in Postgres, with per-artist release-type filters and mutable notification-type preferences — Phase 02
- ✓ Search-proxy API endpoints — live search against MusicBrainz/Deezer so the UI can look up artists to add — Phase 03
- ✓ Scheduler (robfig/cron) polls MusicBrainz + Deezer per watchlist entry on a configurable interval — Phase 03

### Active

- [ ] Diff engine compares poll results against the Postgres "seen" store to detect: new releases, new guest features, deluxe/tracklist changes
- [ ] Notifier posts detected changes to a Discord webhook
- [ ] React (Vite) SPA UI for browsing/managing the watchlist, built and embedded into the Go binary via `go:embed` — single deployable image
- [ ] Multi-stage Dockerfile: slim base image, non-root user, single final image containing API+UI
- [ ] docker-compose for local dev (app + Postgres) — partial: Postgres service exists (Phase 01), app service not yet added
- [ ] pre-commit hooks: golangci-lint, gitleaks
- [ ] GitHub Actions "Full Pipeline": golangci-lint + go vet + unit tests (httptest.Server-mocked MusicBrainz/Deezer) → Trivy image/dependency scan → gitleaks secret scan → SBOM generation → semantic-release versioning/tagging → push image to GitHub Container Registry (ghcr.io)
- [ ] VPS SSH-based deploy step (added once the app is feature-stable — not part of initial phases)

### Out of Scope

- Python implementation — considered, rejected in favor of Go for portfolio differentiation and better fit with the CI/CD/DevOps practice goal
- Producer tracking — mentioned as a future watchlist entity, not part of initial scope
- Prometheus/Grafana observability stack — deferred; structured logging only for v1, metrics endpoint can be added later without a redesign
- Kubernetes/Helm deployment — deferred in favor of a simpler VPS SSH deploy once the app is stable; revisit if more DevOps surface is wanted later
- Split microservices (separate API/scheduler/UI containers) — rejected; single Go binary/service keeps v1 CI/CD simpler while still exercising the full pipeline
- Terraform/IaC-provisioned infra — deferred past the "GitOps / Full DevOps" tier; VPS SSH deploy is the current ceiling

## Context

- Primary motivation is hands-on practice with production-style CI/CD and DevOps pipelines (linting, testing, security scanning, SBOMs, semantic versioning, container registries, staged deployment) — the music-release-tracking domain was chosen because it's genuinely useful and has natural external API integrations (MusicBrainz, Deezer) and a natural notification sink (Discord) to build the pipeline around.
- Greenfield repo — currently contains only `README.md`, `LICENSE`, and `.gitignore`.
- Deployment is intentionally phased: local-only (docker-compose) while the app is being built out, then a self-hosted VPS via an SSH-based deploy action once there's a stable, feature-complete version worth demoing live.
- MusicBrainz and Deezer clients should be real, testable HTTP clients — tests mock the external calls with `httptest.Server`, not fake/stub business logic.
- musicbrainz.org's TLS handshake fails with an `unexpected eof`/server `decode_error` alert from this developer's WSL2 network path specifically — reproduced identically with plain `curl` (bypassing drop-tracker's Go client entirely), confirmed environmental (not a code defect) during Phase 03 UAT. Deezer is unaffected. If a future phase's live testing hits the same MusicBrainz-only TLS failure on this machine, this is already a known, accepted limitation — see `.planning/phases/03-external-clients-search/03-VERIFICATION.md` Acknowledged Gaps and Broken Windows Ledger entry #3 (waived).
- Config/settings library (pydantic-settings equivalent — e.g. envconfig/viper) and exact structured-logging setup are implementation details left to phase research/planning rather than locked here.

## Constraints

- **Tech stack**: Go (not Python) — chosen for portfolio differentiation and closer fit to systems/DevOps practice
- **Web framework**: chi router — stdlib-idiomatic, minimal dependency footprint
- **DB access**: sqlc for type-safe generated queries; golang-migrate for schema migrations
- **Scheduler**: robfig/cron for periodic polling
- **UI**: React + Vite, built and embedded into the Go binary via `go:embed` (no separate frontend deploy pipeline)
- **Architecture**: single Go binary/service (API, scheduler, notifier all in one process) — not split microservices
- **Registry**: GitHub Container Registry (ghcr.io) — uses `GITHUB_TOKEN`, no extra registry secret needed
- **Security**: all secrets via environment variables only; nothing real ever committed; gitleaks enforced in pre-commit and CI
- **Testing**: unit tests use `httptest.Server` to mock MusicBrainz/Deezer, no live external calls in CI

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over Python | Better portfolio differentiation; closer fit to systems/DevOps skills being practiced | — Pending |
| chi router | stdlib-idiomatic, minimal footprint for a small API surface | Validated Phase 01 — `/health` route + `go-chi/httplog` request logging wired |
| sqlc for DB access | Type-safe generated queries; codegen step is itself a nice CI showcase | Validated Phase 01 — `make sqlc-check` regenerates and diffs committed output; version-pinned via `sqlc-version-check` |
| golang-migrate for migrations | Widely used, plain SQL up/down files, simple CI integration | Validated Phase 01 — embedded (`go:embed`) migrations run at boot with a bounded retry loop and context-cancellation support |
| robfig/cron for scheduling | Closest equivalent to APScheduler; configurable per-source poll intervals | Validated Phase 03 — two independent cron entries (MusicBrainz/Deezer), each behind its own CAS overlap guard so one source's pace never blocks the other |
| React (Vite) SPA embedded via go:embed | Keeps deployable to a single Go binary/image while still using a real frontend stack | — Pending |
| Single Go binary/service architecture | Simpler CI/CD to start; still exercises full pipeline without microservice complexity | — Pending |
| DB-backed watchlist with CRUD API | More realistic surface, more to test/lint/scan than a static config file | Validated Phase 02 — add/remove/list/update-preferences all live behind `internal/watchlist.Store`, the reusable domain surface later phases (search-proxy, poller) build on |
| Live search-proxy endpoints | Lets the UI look up artists/albums against MusicBrainz/Deezer directly, not just local DB | Validated Phase 03 — `GET /search` fans out concurrently to both sources, source-tagged, D-03 partial-failure contract (one source down never fails the whole request) |
| httptest.Server for HTTP mocking in tests | Stdlib-only, no extra test dependency | Validated Phase 03 — `internal/musicbrainz`/`internal/deezer` clients tested exclusively against `httptest.Server` fixtures per CLAUDE.md's no-live-calls-in-CI constraint |
| "Full Pipeline" CI/CD depth (lint+test+scan+SBOM+semantic-release+push) | Matches the project's primary goal of practicing real DevOps pipelines | — Pending |
| ghcr.io as image registry | Free, zero extra secrets, tightly integrated with GitHub Actions | — Pending |
| Structured logging only for v1 (no Prometheus/Grafana yet) | Keeps initial scope tight; metrics can be layered on later | Validated Phase 01 — `log/slog` JSON logging via `go-chi/httplog`, wired with a secret-redaction pattern for the DB DSN |
| Phased deploy: local-only now, VPS SSH deploy later | Avoids committing to live infra before the app is feature-stable | — Pending |
| DSN/secret redaction on every error path that could reach logs or stderr | Connection-failure errors routinely embed the raw DSN with its password; Phase 01's security review (T-01-01) required scrubbing it before it reaches `slog` or a returned error | Validated Phase 01 — `redactDSN`/`redactError` helpers in `internal/db/migrate.go`, asserted by `TestRunMigrations_NeverLogsDSN` |
| Graceful shutdown via `signal.NotifyContext` + bounded `httpSrv.Shutdown` timeout | A container orchestrator stops the process with SIGTERM; without this, in-flight requests and the deferred `pool.Close()` are skipped | Validated Phase 01 — confirmed end-to-end under a real SIGTERM in WSL2 (UAT test 1) |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-08-08 after Phase 03 (external-clients-search)*
