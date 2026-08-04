# drop-tracker

## What This Is

A Go-based release tracker for hip-hop, reggaeton, and R&B: users maintain a watchlist of artists (and later albums/producers) via a React UI, a scheduler polls MusicBrainz and Deezer for those artists, diffs results against a Postgres "seen" store to detect new releases, guest features, and deluxe/tracklist changes, and posts alerts to a Discord webhook. The primary purpose is a portfolio piece for practicing real CI/CD and DevOps pipelines — the music-tracking domain is the vehicle, the pipeline maturity is the point.

## Core Value

A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice (lint, test, security scan, SBOM, versioned image publish, and eventually automated deploy).

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Watchlist CRUD API — add/remove/list artists (later albums, producers), DB-backed in Postgres
- [ ] Search-proxy API endpoints — live search against MusicBrainz/Deezer so the UI can look up artists to add
- [ ] Scheduler (robfig/cron) polls MusicBrainz + Deezer per watchlist entry on a configurable interval
- [ ] Diff engine compares poll results against the Postgres "seen" store to detect: new releases, new guest features, deluxe/tracklist changes
- [ ] Notifier posts detected changes to a Discord webhook
- [ ] `/health` endpoint for liveness/readiness
- [ ] React (Vite) SPA UI for browsing/managing the watchlist, built and embedded into the Go binary via `go:embed` — single deployable image
- [ ] sqlc-generated type-safe DB queries + golang-migrate schema migrations
- [ ] Structured (slog-based) JSON logging
- [ ] Multi-stage Dockerfile: slim base image, non-root user, single final image containing API+UI
- [ ] docker-compose for local dev (app + Postgres)
- [ ] `.env.example` documenting all config, secrets via env vars only — nothing real committed
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
| chi router | stdlib-idiomatic, minimal footprint for a small API surface | — Pending |
| sqlc for DB access | Type-safe generated queries; codegen step is itself a nice CI showcase | — Pending |
| golang-migrate for migrations | Widely used, plain SQL up/down files, simple CI integration | — Pending |
| robfig/cron for scheduling | Closest equivalent to APScheduler; configurable per-source poll intervals | — Pending |
| React (Vite) SPA embedded via go:embed | Keeps deployable to a single Go binary/image while still using a real frontend stack | — Pending |
| Single Go binary/service architecture | Simpler CI/CD to start; still exercises full pipeline without microservice complexity | — Pending |
| DB-backed watchlist with CRUD API | More realistic surface, more to test/lint/scan than a static config file | — Pending |
| Live search-proxy endpoints | Lets the UI look up artists/albums against MusicBrainz/Deezer directly, not just local DB | — Pending |
| httptest.Server for HTTP mocking in tests | Stdlib-only, no extra test dependency | — Pending |
| "Full Pipeline" CI/CD depth (lint+test+scan+SBOM+semantic-release+push) | Matches the project's primary goal of practicing real DevOps pipelines | — Pending |
| ghcr.io as image registry | Free, zero extra secrets, tightly integrated with GitHub Actions | — Pending |
| Structured logging only for v1 (no Prometheus/Grafana yet) | Keeps initial scope tight; metrics can be layered on later | — Pending |
| Phased deploy: local-only now, VPS SSH deploy later | Avoids committing to live infra before the app is feature-stable | — Pending |

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
*Last updated: 2026-08-04 after initialization*
