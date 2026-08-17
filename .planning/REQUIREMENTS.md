# Requirements: drop-tracker

**Defined:** 2026-08-04
**Core Value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Watchlist

- [x] **WLST-01**: User can search MusicBrainz and Deezer catalogs to find an artist to add to the watchlist
- [x] **WLST-02**: User can add an artist to the watchlist from search results
- [x] **WLST-03**: User can remove an artist from the watchlist
- [x] **WLST-04**: User can list all artists currently on the watchlist
- [x] **WLST-05**: User can set per-artist release-type filters (album/single/EP/deluxe) to control which release types trigger alerts
- [x] **WLST-06**: User can set per-artist notification preferences (e.g. mute deluxe/reissue alerts)

### External Clients

- [x] **CLNT-01**: System polls MusicBrainz for each watchlisted artist on a configurable schedule, respecting MusicBrainz's rate limit
- [x] **CLNT-02**: System polls Deezer for each watchlisted artist on a configurable schedule, respecting Deezer's rate limit
- [x] **CLNT-03**: System exposes a live search-proxy endpoint that queries MusicBrainz and Deezer catalogs in real time

### Detection

- [x] **DTCT-01**: System detects a new release-group for a watchlisted artist and records it as a "new release" event
- [x] **DTCT-02**: System detects a new release inside an existing release-group with an expanded tracklist and records it as a "deluxe/tracklist-change" event
- [x] **DTCT-03**: System detects a recording where a watchlisted artist appears in the artist-credit list without being the primary artist, and records it as a "guest feature" event
- [x] **DTCT-04**: System does not re-notify for a release/change it has already recorded (idempotent "seen" store)
- [x] **DTCT-05**: System does not process a poll cycle concurrently if a prior cycle for the same source is still running (no duplicate/overlapping detection runs)

### Notifications

- [x] **NTFY-01**: System posts a Discord webhook message for each detected new-release event, including title, artist, cover art, release date, and release type
- [x] **NTFY-02**: System posts a visually distinct Discord webhook message for detected guest-feature events
- [x] **NTFY-03**: System posts a visually distinct Discord webhook message for detected deluxe/tracklist-change events
- [x] **NTFY-04**: System suppresses notifications for artists/release-types the user has muted via notification preferences

### History

- [x] **HIST-01**: User can view a history of detected events (new release, guest feature, deluxe change) per artist, including what changed

### Operations

- [x] **OPS-01**: System exposes a `/health` endpoint that reports service and database connectivity status
- [x] **OPS-02**: System emits structured (JSON) logs with request-ID correlation for HTTP requests and poll cycles
- [x] **OPS-03**: All secrets and configuration are supplied via environment variables only; none are committed to the repository

### Frontend

- [x] **UI-01**: User can search for and add an artist to the watchlist via the web UI
- [x] **UI-02**: User can view and manage (remove, set preferences) their watchlist via the web UI
- [x] **UI-03**: User can browse a feed/history of detected release events via the web UI

### CI/CD Pipeline

- [x] **CICD-01**: Every push runs golangci-lint, go vet, and the Go test suite (MusicBrainz/Deezer calls mocked via `httptest.Server`) before any build/publish step
- [x] **CICD-02**: Every push runs gitleaks to scan for committed secrets, blocking the pipeline on detection
- [x] **CICD-03**: The pipeline builds a multi-stage Docker image (slim base, non-root user) containing the API, scheduler, notifier, and embedded frontend
- [x] **CICD-04**: The pipeline scans the built image with Trivy and blocks publishing on critical vulnerabilities
- [x] **CICD-05**: The pipeline generates an SBOM for the built image
- [x] **CICD-06**: The pipeline computes a semantic version and tags a release automatically on merge to main
- [x] **CICD-07**: The pipeline pushes the built, scanned image to GitHub Container Registry (ghcr.io) tagged with the semantic version
- [x] **CICD-08**: Third-party GitHub Actions used for security-sensitive steps are pinned to commit SHAs, not tags
- [x] **CICD-09**: `docker-compose` brings up the app and a local Postgres instance for local development
- [x] **CICD-10**: A pre-commit configuration runs golangci-lint and gitleaks locally before commit (gitleaks half done in quick task 260806-hfn; golangci-lint half added in Phase 07 plan 07-02)

## v1.1 Requirements

Requirements for the "Hardening & Scale Readiness" milestone — closing four peer-reviewed gaps from v1.0. Each maps to roadmap phases.

### Frontend Testing

- [x] **TEST-01**: Frontend has a Vitest + React Testing Library test suite covering the watchlist list/row, preference-toggle, search, and history/event-filter component and route surface
- [x] **TEST-02**: Frontend tests mock the app's API boundary (`web/app/lib/api.ts`) rather than intercepting raw fetch/network calls

### CI/CD Pipeline

- [x] **CICD-11**: CI fails the build if backend Go test coverage falls below 80%
- [x] **CICD-12**: CI fails the build if frontend test coverage falls below 70%

### Data Retention

- [x] **DATA-01**: Event-retention window is configurable via environment variable, defaulting to 90 days
- [x] **DATA-02**: History and API queries exclude events older than the retention window, while the underlying rows and detection state (dedup keys, deluxe-change baselines, seed-mode signal) are left untouched

### Polling Performance

- [ ] **PERF-01**: Per-source polling (MusicBrainz, Deezer) uses a bounded, env-configurable concurrent worker pool (default 3-5 workers) instead of strictly sequential per-artist iteration
- [ ] **PERF-02**: Concurrent polling preserves the existing per-source rate limiter and per-source cycle-overlap guard
- [ ] **PERF-03**: A single artist's polling error does not abort the rest of that cycle's batch (errors are logged and skipped, not fatal)
- [x] **PERF-04**: Concurrent updates to a shared release-group's deluxe-change baseline cannot lose an update (baseline compare-and-set is atomic at the database level)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Deployment

- **DPLY-01**: Pipeline deploys the built image to a self-hosted VPS via an SSH-based deploy step, added once the app is feature-stable

### Watchlist

- **WLST-07**: User can add producers as a watchlist entity type (in addition to artists)

### CI/CD Pipeline

- **CICD-13**: CI posts a PR coverage-diff/report comment (backend and/or frontend) once the v1.1 coverage gates are proven stable

### Frontend Testing

- **TEST-03**: E2E test suite (Playwright) covering critical user flows, if a future milestone specifically calls for integration/E2E coverage

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Dual-source (MusicBrainz + Deezer) reconciliation | Real cross-source entity-matching complexity for narrower payoff than nailing core detection first; Deezer stays a secondary/faster signal only |
| Recommendation engine | Orthogonal ML/data problem, zero relevance to the CI/CD practice goal |
| Auto-download / acquisition | Legal gray area; pulls in an entirely separate download/indexer subsystem outside this project's scope |
| Multi-user auth / accounts / SSO | Single-operator deployable per PROJECT.md; OAuth/session/RBAC is an orthogonal complexity spike that doesn't showcase CI/CD skills |
| In-app music playback / streaming integration | Requires licensed streaming SDKs and DRM handling; alerts link out to the source instead |
| Mobile push notifications / native app | Requires APNs/FCM infra and a mobile client; Discord (which already has a mobile app) is the notification sink |
| Full historical backfill / discography mirroring | Turns the tool into a metadata warehouse instead of a forward-looking watcher; seed "seen" store with current state, alert only on changes going forward |
| Audio fingerprinting / ISRC-based dedupe | High implementation cost for a problem release-group-id-based diffing already handles well enough for v1; accept documented residual noise instead |
| Split microservices (separate API/scheduler/UI containers) | Single Go binary/service keeps v1 CI/CD simpler while still exercising the full pipeline |
| Kubernetes/Helm deployment | VPS SSH deploy is the current deployment ceiling; revisit only if more DevOps surface is wanted later |
| Prometheus/Grafana observability stack | Structured logging only for v1; metrics endpoint can be layered on later without a redesign |
| Terraform/IaC-provisioned infra | Deferred past the "Full Pipeline" CI/CD tier; VPS SSH deploy doesn't require infra provisioning |
| Full E2E test suite (Playwright/Cypress) this milestone | Different testing tier than the "unit/component" scope of TEST-01/02; deferred to v2 as TEST-03 if a future milestone calls for it |
| Table partitioning for events retention | No realistic data volume at this project's scale justifies it; revisit only if the events table grows by orders of magnitude |
| Adaptive/dynamic concurrency tuning for polling | Static env-configured pool size (PERF-01) is correct for this project's traffic profile; revisit only if polling needs to scale to many more watched artists |
| `pg_cron` extension for retention scheduling | Requires Postgres extension/superuser setup; the existing in-process `robfig/cron` scheduler already covers this need with zero new infrastructure |
| Per-package/diff coverage gating | Single aggregate threshold per side (CICD-11/12) is sufficient; no legacy coverage debt exists to route around |
| Mutation testing | Disproportionate CI runtime/tooling surface for this project's size; line/branch coverage thresholds are the accepted rigor level |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| OPS-01 | Phase 1 | Complete |
| OPS-02 | Phase 1 | Complete |
| OPS-03 | Phase 1 | Complete |
| WLST-02 | Phase 2 | Complete |
| WLST-03 | Phase 2 | Complete |
| WLST-04 | Phase 2 | Complete |
| WLST-05 | Phase 2 | Complete |
| WLST-06 | Phase 2 | Complete |
| WLST-01 | Phase 3 | Complete |
| CLNT-01 | Phase 3 | Complete |
| CLNT-02 | Phase 3 | Complete |
| CLNT-03 | Phase 3 | Complete |
| DTCT-01 | Phase 4 | Complete |
| DTCT-02 | Phase 4 | Complete |
| DTCT-03 | Phase 4 | Complete |
| DTCT-04 | Phase 4 | Complete |
| DTCT-05 | Phase 4 | Complete |
| NTFY-01 | Phase 5 | Complete |
| NTFY-02 | Phase 5 | Complete |
| NTFY-03 | Phase 5 | Complete |
| NTFY-04 | Phase 5 | Complete |
| UI-01 | Phase 6 | Complete |
| UI-02 | Phase 6 | Complete |
| UI-03 | Phase 6 | Complete |
| HIST-01 | Phase 6 | Complete |
| CICD-01 | Phase 7 | Complete |
| CICD-02 | Phase 7 | Complete |
| CICD-03 | Phase 7 | Complete |
| CICD-04 | Phase 7 | Complete |
| CICD-05 | Phase 7 | Complete |
| CICD-06 | Phase 7 | Complete |
| CICD-07 | Phase 7 | Complete |
| CICD-08 | Phase 7 | Complete |
| CICD-09 | Phase 7 | Complete |
| CICD-10 | Phase 7 | Complete |
| TEST-01 | Phase 8 | Complete |
| TEST-02 | Phase 8 | Complete |
| CICD-11 | Phase 9 | Complete |
| CICD-12 | Phase 9 | Complete |
| DATA-01 | Phase 10 | Complete |
| DATA-02 | Phase 10 | Complete |
| PERF-01 | Phase 11 | Pending |
| PERF-02 | Phase 11 | Pending |
| PERF-03 | Phase 11 | Pending |
| PERF-04 | Phase 11 | Complete |

**Coverage:**

- v1 requirements: 35 total
- Mapped to phases: 35
- Unmapped: 0 ✓

- v1.1 requirements: 10 total
- Mapped to phases: 10
- Unmapped: 0 ✓

---
*Requirements defined: 2026-08-04*
*Last updated: 2026-08-12 after v1.1 roadmap creation (Phases 8-11, full coverage)*
