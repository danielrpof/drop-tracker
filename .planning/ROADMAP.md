# Roadmap: drop-tracker

## Overview

drop-tracker starts from an empty repo and builds outward from the data layer: a Postgres schema, config, and health-checked service skeleton first, then a fully tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications for those events, then the React UI that ties watchlist management and release history together, and finally the single-image containerization and full GitHub Actions CI/CD pipeline (lint, test, security scan, SBOM, semantic versioning, ghcr.io publish) that is the actual point of the project. Each phase produces something a user (or operator) can directly observe working before the next phase builds on it.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation — Data Layer, Config & Health** - Postgres schema/migrations, sqlc, env-based config, structured logging, and a `/health` endpoint the rest of the app is built on (completed 2026-08-05)
- [ ] **Phase 2: Watchlist Core** - Users can add, remove, list, and configure per-artist alert preferences through a tested watchlist API
- [ ] **Phase 3: External Clients & Search** - Rate-limited MusicBrainz/Deezer clients power a live search-proxy and scheduled polling
- [ ] **Phase 4: Detection Engine** - Poll results are diffed against a "seen" store to reliably detect new releases, guest features, and deluxe/tracklist changes without duplicates or overlapping runs
- [ ] **Phase 5: Discord Notifications** - Detected events are posted to Discord with distinct formatting per event type, honoring mute preferences
- [ ] **Phase 6: Frontend & Release History** - Users manage their watchlist and browse detected release history entirely through a web UI
- [ ] **Phase 7: Containerization & CI/CD Pipeline** - The app ships as a single scanned, versioned, non-root Docker image via an automated GitHub Actions pipeline, with docker-compose for local dev

## Phase Details

### Phase 1: Foundation — Data Layer, Config & Health

**Goal**: The service boots reliably from environment configuration, persists to a migrated Postgres schema, and reports its own health — the foundation every later phase is built on.
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: OPS-01, OPS-02, OPS-03
**Success Criteria** (what must be TRUE):

  1. Operator can query `/health` and see accurate service and database connectivity status
  2. Every HTTP request and poll cycle emits a structured JSON log line with a correlating request ID
  3. The service starts entirely from environment variables (via `.env.example` documenting every setting), with no real secret ever committed to the repo

**Plans**: 5/5 plans executed

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Tracer: scaffold the module and wire env config → migrated Postgres → chi → `GET /health` end-to-end

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-02-PLAN.md — Health degraded/timeout branches, concurrent polling, and `X-Request-Id` correlation proven against the log line
- [x] 01-03-PLAN.md — Complete the `Config` surface through Phase 5, `.env.example` parity, and fail-fast rejection coverage
- [x] 01-04-PLAN.md — Wire sqlc end-to-end (config, query, committed codegen, execution test) plus the `make sqlc-check` drift gate
- [x] 01-05-PLAN.md — Injectable migrate-on-boot retry policy with apply/idempotency/exhaustion/cancellation/redaction coverage

### Phase 2: Watchlist Core

**Goal**: Users can fully manage their watchlist — add, remove, list, and configure per-artist alert preferences — through a tested API service layer.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: WLST-02, WLST-03, WLST-04, WLST-05, WLST-06
**Success Criteria** (what must be TRUE):

  1. User can add an artist to the watchlist
  2. User can remove an artist from the watchlist
  3. User can list all artists currently on the watchlist
  4. User can set per-artist release-type filters (album/single/EP/deluxe) that control which release types trigger alerts
  5. User can mute specific notification types per artist (e.g., deluxe/reissue alerts)

**Plans**: 6/6 plans executed

Plans:
**Wave 1**

- [x] 02-01-PLAN.md — Tracer: `POST /watchlist` end-to-end — artists/watchlist schema, sqlc codegen, `internal/watchlist` Store seam, widened `httpserver.New`

**Wave 2** *(blocked on Wave 1)*

- [x] 02-02-PLAN.md — Add-path completeness: 409 duplicate via SQLSTATE 23505, optional initial preferences with allow-list validation, request-size and field-length bounds

**Wave 3** *(blocked on Wave 2)*

- [x] 02-03-PLAN.md — `GET /watchlist` stably ordered with `[]` on empty, and `DELETE /watchlist/{id}` hard delete with honest 404 under concurrency

**Wave 4** *(blocked on Wave 3)*

- [x] 02-04-PLAN.md — `PATCH /watchlist/{id}` independent preference axes with partial updates, CHECK-constraint backstop proof, phase-closing gate

**Gap closure — Wave 1** *(from 02-UAT.md test 2: "fix both WRs")*

- [x] 02-05-PLAN.md — G-02-2a: widen `UpsertArtist`'s `ON CONFLICT` SET list so a re-add refreshes `disambiguation`/`image_url` instead of silently discarding them

**Gap closure — Wave 2** *(blocked on Gap closure Wave 1)*

- [x] 02-06-PLAN.md — G-02-2b: collapse `UpdatePreferences` into one locked statement — honest 404 on the deleted-mid-write race, no lost update between concurrent PATCH calls

### Phase 3: External Clients & Search

**Goal**: The service can search and poll MusicBrainz and Deezer safely within their rate limits, and users can search those catalogs live to find artists to watch.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: WLST-01, CLNT-01, CLNT-02, CLNT-03
**Success Criteria** (what must be TRUE):

  1. User can search MusicBrainz and Deezer catalogs via a live search-proxy endpoint and see matching artists to add
  2. System polls MusicBrainz for each watchlisted artist on a configurable schedule without exceeding MusicBrainz's rate limit
  3. System polls Deezer for each watchlisted artist on a configurable schedule without exceeding Deezer's rate limit

**Plans**: TBD

### Phase 4: Detection Engine

**Goal**: The system reliably detects new releases, guest features, and deluxe/tracklist changes for watched artists, with no duplicate or overlapping detection runs.
**Mode:** mvp
**Depends on**: Phase 2, Phase 3
**Requirements**: DTCT-01, DTCT-02, DTCT-03, DTCT-04, DTCT-05
**Success Criteria** (what must be TRUE):

  1. A new release-group for a watchlisted artist is detected and recorded as a "new release" event
  2. A new release inside an existing release-group with an expanded tracklist is detected and recorded as a "deluxe/tracklist-change" event
  3. A recording where a watchlisted artist appears as a non-primary artist-credit is detected and recorded as a "guest feature" event
  4. The system never re-records or re-notifies for a release/change it has already seen
  5. The system never runs two poll cycles for the same source concurrently, even if a prior cycle is still running

**Plans**: TBD

### Phase 5: Discord Notifications

**Goal**: Users are notified in Discord immediately and distinctly when a detected event matches their preferences.
**Mode:** mvp
**Depends on**: Phase 4
**Requirements**: NTFY-01, NTFY-02, NTFY-03, NTFY-04
**Success Criteria** (what must be TRUE):

  1. User receives a Discord webhook message for each new-release event including title, artist, cover art, release date, and release type
  2. User receives a visually distinct Discord webhook message for guest-feature events
  3. User receives a visually distinct Discord webhook message for deluxe/tracklist-change events
  4. User does not receive notifications for artists/release-types they've muted via their preferences

**Plans**: TBD

### Phase 6: Frontend & Release History

**Goal**: Users can manage their watchlist and review detected release activity entirely through a web UI, without touching the API directly.
**Mode:** mvp
**Depends on**: Phase 2, Phase 3, Phase 4
**Requirements**: UI-01, UI-02, UI-03, HIST-01
**Success Criteria** (what must be TRUE):

  1. User can search for and add an artist to the watchlist via the web UI
  2. User can view and manage (remove, set preferences on) their watchlist via the web UI
  3. User can browse a feed/history of detected release events per artist via the web UI, including what changed

**Plans**: TBD
**UI hint**: yes

### Phase 7: Containerization & CI/CD Pipeline

**Goal**: Every push is automatically linted, tested, and security-scanned, and every merge to main produces a versioned, non-root, single-image build published to a container registry — with the full stack (API, scheduler, notifier, embedded SPA) also runnable locally via docker-compose.
**Mode:** mvp
**Depends on**: Phase 6
**Requirements**: CICD-01, CICD-02, CICD-03, CICD-04, CICD-05, CICD-06, CICD-07, CICD-08, CICD-09, CICD-10
**Success Criteria** (what must be TRUE):

  1. Every push runs golangci-lint, go vet, and the full Go test suite (MusicBrainz/Deezer calls mocked via `httptest.Server`) before any build or publish step
  2. Every push is scanned for committed secrets (gitleaks) and, once built, the image is scanned for critical vulnerabilities (Trivy) — either finding blocks the pipeline
  3. A merge to main computes a semantic version, generates an SBOM, and pushes the built image to ghcr.io tagged with that version
  4. The full application (API + scheduler + notifier + embedded SPA) runs as a single non-root multi-stage Docker image, reproducible locally via `docker-compose up` alongside Postgres
  5. All security-sensitive third-party GitHub Actions are pinned to commit SHAs, and a pre-commit hook runs golangci-lint and gitleaks locally before any commit reaches the pipeline

**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation — Data Layer, Config & Health | 5/5 | Complete    | 2026-08-05 |
| 2. Watchlist Core | 6/6 | In Progress|  |
| 3. External Clients & Search | 0/TBD | Not started | - |
| 4. Detection Engine | 0/TBD | Not started | - |
| 5. Discord Notifications | 0/TBD | Not started | - |
| 6. Frontend & Release History | 0/TBD | Not started | - |
| 7. Containerization & CI/CD Pipeline | 0/TBD | Not started | - |
