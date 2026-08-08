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
- [ ] **NTFY-02**: System posts a visually distinct Discord webhook message for detected guest-feature events
- [ ] **NTFY-03**: System posts a visually distinct Discord webhook message for detected deluxe/tracklist-change events
- [ ] **NTFY-04**: System suppresses notifications for artists/release-types the user has muted via notification preferences

### History

- [ ] **HIST-01**: User can view a history of detected events (new release, guest feature, deluxe change) per artist, including what changed

### Operations

- [x] **OPS-01**: System exposes a `/health` endpoint that reports service and database connectivity status
- [x] **OPS-02**: System emits structured (JSON) logs with request-ID correlation for HTTP requests and poll cycles
- [x] **OPS-03**: All secrets and configuration are supplied via environment variables only; none are committed to the repository

### Frontend

- [ ] **UI-01**: User can search for and add an artist to the watchlist via the web UI
- [ ] **UI-02**: User can view and manage (remove, set preferences) their watchlist via the web UI
- [ ] **UI-03**: User can browse a feed/history of detected release events via the web UI

### CI/CD Pipeline

- [ ] **CICD-01**: Every push runs golangci-lint, go vet, and the Go test suite (MusicBrainz/Deezer calls mocked via `httptest.Server`) before any build/publish step
- [ ] **CICD-02**: Every push runs gitleaks to scan for committed secrets, blocking the pipeline on detection
- [ ] **CICD-03**: The pipeline builds a multi-stage Docker image (slim base, non-root user) containing the API, scheduler, notifier, and embedded frontend
- [ ] **CICD-04**: The pipeline scans the built image with Trivy and blocks publishing on critical vulnerabilities
- [ ] **CICD-05**: The pipeline generates an SBOM for the built image
- [ ] **CICD-06**: The pipeline computes a semantic version and tags a release automatically on merge to main
- [ ] **CICD-07**: The pipeline pushes the built, scanned image to GitHub Container Registry (ghcr.io) tagged with the semantic version
- [ ] **CICD-08**: Third-party GitHub Actions used for security-sensitive steps are pinned to commit SHAs, not tags
- [ ] **CICD-09**: `docker-compose` brings up the app and a local Postgres instance for local development
- [ ] **CICD-10**: A pre-commit configuration runs golangci-lint and gitleaks locally before commit (gitleaks half done in quick task 260806-hfn; golangci-lint half deferred to Phase 07)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Deployment

- **DPLY-01**: Pipeline deploys the built image to a self-hosted VPS via an SSH-based deploy step, added once the app is feature-stable

### Watchlist

- **WLST-07**: User can add producers as a watchlist entity type (in addition to artists)

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
| NTFY-02 | Phase 5 | Pending |
| NTFY-03 | Phase 5 | Pending |
| NTFY-04 | Phase 5 | Pending |
| UI-01 | Phase 6 | Pending |
| UI-02 | Phase 6 | Pending |
| UI-03 | Phase 6 | Pending |
| HIST-01 | Phase 6 | Pending |
| CICD-01 | Phase 7 | Pending |
| CICD-02 | Phase 7 | Pending |
| CICD-03 | Phase 7 | Pending |
| CICD-04 | Phase 7 | Pending |
| CICD-05 | Phase 7 | Pending |
| CICD-06 | Phase 7 | Pending |
| CICD-07 | Phase 7 | Pending |
| CICD-08 | Phase 7 | Pending |
| CICD-09 | Phase 7 | Pending |
| CICD-10 | Phase 7 | Partial (gitleaks half done in quick/260806-hfn; golangci-lint half pending) |

**Coverage:**

- v1 requirements: 35 total
- Mapped to phases: 35
- Unmapped: 0 ✓

---
*Requirements defined: 2026-08-04*
*Last updated: 2026-08-04 after roadmap creation (7 phases, full coverage)*
