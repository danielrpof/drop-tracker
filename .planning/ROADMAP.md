# Roadmap: drop-tracker

## Overview

drop-tracker starts from an empty repo and builds outward from the data layer: a Postgres schema, config, and health-checked service skeleton first, then a fully tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications for those events, then the React UI that ties watchlist management and release history together, and finally the single-image containerization and full GitHub Actions CI/CD pipeline (lint, test, security scan, SBOM, semantic versioning, ghcr.io publish) that is the actual point of the project. Each phase produces something a user (or operator) can directly observe working before the next phase builds on it.

**v1.1 Hardening & Scale Readiness** picks up from a shipped, working v1.0 and closes four peer-reviewed gaps without changing what the app does for its user: the React frontend gets the component test suite it never had, the Full Pipeline starts enforcing coverage floors on both languages instead of merely running tests, the events table gets a retention window that hides stale history from display while leaving every detection-critical row in place, and the poller stops walking the watchlist one artist at a time. Ordering is deliberate: tests before the gate that measures them, and the concurrency rewrite last, once a working coverage harness exists to catch what it breaks.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

**v1.0 MVP — shipped 2026-08-12**

- [x] **Phase 1: Foundation — Data Layer, Config & Health** - Postgres schema/migrations, sqlc, env-based config, structured logging, and a `/health` endpoint the rest of the app is built on (completed 2026-08-05)
- [x] **Phase 2: Watchlist Core** - Users can add, remove, list, and configure per-artist alert preferences through a tested watchlist API (completed 2026-08-06)
- [x] **Phase 3: External Clients & Search** - Rate-limited MusicBrainz/Deezer clients power a live search-proxy and scheduled polling (completed 2026-08-07)
- [x] **Phase 4: Detection Engine** - Poll results are diffed against a "seen" store to reliably detect new releases, guest features, and deluxe/tracklist changes without duplicates or overlapping runs (completed 2026-08-08)
- [x] **Phase 5: Discord Notifications** - Detected events are posted to Discord with distinct formatting per event type, honoring mute preferences (completed 2026-08-08)
- [x] **Phase 6: Frontend & Release History** - Users manage their watchlist and browse detected release history entirely through a web UI (completed 2026-08-11)
- [x] **Phase 7: Containerization & CI/CD Pipeline** - The app ships as a single scanned, versioned, non-root Docker image via an automated GitHub Actions pipeline, with docker-compose for local dev (completed 2026-08-12)

**v1.1 Hardening & Scale Readiness — in progress**

- [ ] **Phase 8: Frontend Test Suite** - The watchlist, search, and history React surfaces get a Vitest + React Testing Library suite that mocks the app's own API boundary
- [ ] **Phase 9: CI Coverage Gates** - The Full Pipeline blocks the build when Go coverage drops below 80% or frontend coverage drops below 70%
- [ ] **Phase 10: Event Retention Window** - History and API hide events older than a configurable window (default 90 days) while every row and all detection state stay intact
- [ ] **Phase 11: Bounded Concurrent Polling** - Each source polls several artists at a time through a bounded worker pool, without breaking rate limits, overlap guards, or baseline correctness

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

**Plans**: 8/8 plans executed

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

**Gap closure — Wave 1** *(from 02-UAT.md tests 1 and 2: "fix"; independent, no shared files)*

- [x] 02-07-PLAN.md — G-02-1: move the neither-axis rule into `watchlist.Service` behind an `ErrNoPreferencesSupplied` sentinel (WR-01), and reject a second concatenated JSON value on both body-taking routes through one shared decode path (WR-02)
- [x] 02-08-PLAN.md — G-02-2: give `redactError` keyword/value-form DSN password coverage (CR-01), proven by unit tests against both redaction helpers, and correct the migration test that never exercised the gap

### Phase 3: External Clients & Search

**Goal**: The service can search and poll MusicBrainz and Deezer safely within their rate limits, and users can search those catalogs live to find artists to watch.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: WLST-01, CLNT-01, CLNT-02, CLNT-03
**Success Criteria** (what must be TRUE):

  1. User can search MusicBrainz and Deezer catalogs via a live search-proxy endpoint and see matching artists to add
  2. System polls MusicBrainz for each watchlisted artist on a configurable schedule without exceeding MusicBrainz's rate limit
  3. System polls Deezer for each watchlisted artist on a configurable schedule without exceeding Deezer's rate limit

**Plans**: 4/4 plans executed

Plans:
**Wave 1**

- [x] 03-01-PLAN.md — Tracer: `GET /search?q=` end-to-end via a rate-limited, User-Agent-identified MusicBrainz client, with the source-keyed response envelope and the widened `httpserver.New`

**Wave 2** *(blocked on Wave 1)*

- [x] 03-02-PLAN.md — Deezer client (search + artist albums, HTTP-200 in-body error detection) joined to the `/search` fan-out as an independent second source
- [x] 03-03-PLAN.md — MusicBrainz release-groups browse-by-artist with bounded, limiter-paced pagination and no retry loop

**Wave 3** *(blocked on Wave 2)*

- [x] 03-04-PLAN.md — `internal/poller`: two independent cron cycles with per-source overlap guards, sequential per-artist polling, nil-`deezer_id` skip, and drain-before-pool-close shutdown

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

**Plans**: 4/4 plans executed

Plans:
**Wave 1**

- [x] 04-01-PLAN.md — Tracer: a MusicBrainz poll cycle records previously-unseen release-groups as `new_release` events end-to-end (migration `000003_events`, `queries/events.sql`, `internal/detection`, the `poller.EventRecorder` seam), plus the `ON CONFLICT DO NOTHING` idempotency and overlap-guard proofs

**Wave 2** *(blocked on Wave 1)*

- [x] 04-02-PLAN.md — `new_release` completeness: both preference axes applied at detection time (including the `deluxe` pseudo-type), per-source seed mode with `notified_at` pre-set, and the Deezer cycle as an independent second source

**Wave 3** *(blocked on Wave 2)*

- [x] 04-03-PLAN.md — Guest-feature slice: `RecordingsByArtist` bounded browse plus the positional artist-credit rule, malformed-credit guards, and page-ceiling visibility

**Wave 4** *(blocked on Wave 3)*

- [x] 04-04-PLAN.md — Deluxe/tracklist-change slice: `ReleasesByReleaseGroup` with multi-disc track-count summing and establish-then-compare baseline tracking that fires no false positive on a group's first measurement

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

**Plans**: 3/3 plans executed

- [x] 05-01-PLAN.md
- [x] 05-02-PLAN.md
- [x] 05-03-PLAN.md

### Phase 6: Frontend & Release History

**Goal**: Users can manage their watchlist and review detected release activity entirely through a web UI, without touching the API directly.
**Mode:** mvp
**Depends on**: Phase 2, Phase 3, Phase 4
**Requirements**: UI-01, UI-02, UI-03, HIST-01
**Success Criteria** (what must be TRUE):

  1. User can search for and add an artist to the watchlist via the web UI
  2. User can view and manage (remove, set preferences on) their watchlist via the web UI
  3. User can browse a feed/history of detected release events per artist via the web UI, including what changed

**Plans**: 4/4 plans executed
**UI hint**: yes

Plans:
**Wave 1**

- [x] 06-01-PLAN.md — Tracer: `GET /events` end-to-end — `ListEvents` keyset query, `internal/events` Store seam, the embedded React Router SPA served by the Go binary via `go:embed`, and a History route rendering real rows

**Wave 2** *(blocked on Wave 1)*

- [x] 06-02-PLAN.md — History feed completeness: validated/clamped `artist_id`/`event_type`/`cursor`/`limit` params, and type-specific art-forward event cards with filters, load-more and every empty/loading/error state
- [x] 06-03-PLAN.md — Watchlist tab: the artist list with all its states, inline preference toggles with optimistic rollback, and one-click remove with an honestly-labelled Undo

**Wave 3** *(blocked on Wave 2)*

- [x] 06-04-PLAN.md — Artist search at the top of the Watchlist tab: debounced two-column source results, one-click add, client-side "Already watching", plus the phase-closing manual UAT gate

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

**Plans**: 4/4 plans executed

Plans:
**Wave 1**

- [x] 07-01-PLAN.md — Multi-stage non-root image + docker-compose `app:` service (CICD-03, CICD-09)
- [x] 07-02-PLAN.md — golangci-lint v2 config + pre-commit hook (CICD-01, CICD-10)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 07-03-PLAN.md — Full Pipeline workflow: lint/vet/test/gitleaks/PR-title gates + Trivy-blocked image build (CICD-01, CICD-02, CICD-04, CICD-08)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 07-04-PLAN.md — Release path: svu semver, ghcr.io push, SBOM, seeded v0.1.0 tag (CICD-05, CICD-06, CICD-07)

### Phase 8: Frontend Test Suite

**Goal**: The React frontend's watchlist, search, and history surfaces are covered by a real component test suite, so a regression in the UI is caught by a test run instead of by hand-clicking the app.
**Depends on**: Phase 7 (v1.0 complete)
**Requirements**: TEST-01, TEST-02
**Success Criteria** (what must be TRUE):

  1. A single command runs the frontend suite (Vitest + React Testing Library, jsdom) locally and in CI, and exits non-zero when a component regresses
  2. The watchlist list/row, preference-toggle, search, and history/event-filter surfaces each have at least one passing test asserting user-visible behavior — e.g. the watchlist row's remove control triggers the remove API call, and a preference toggle rolls back its optimistic state when the call fails
  3. Tests mock the app's API boundary (`web/app/lib/api.ts`), not raw `fetch` — no test issues a real network request, and the whole suite passes with no server running
  4. Components needing router context render through one shared helper (React Router's `createRoutesStub`), established once and reused, rather than each test reinventing router wrapping

**Plans**: 4/5 plans executed

Plans:

- [x] 08-01-PLAN.md
- [x] 08-02-PLAN.md
- [x] 08-03-PLAN.md
- [x] 08-04-PLAN.md
- [ ] 08-05-PLAN.md

- [ ] TBD (run /gsd-plan-phase 8 to break down)

Notes: Vitest cannot reuse `web/vite.config.ts` — React Router's Vite plugin is incompatible, so this phase adds a separate `vitest.config.ts`. Test files co-locate beside source (`*.test.tsx`), mirroring Go's `_test.go` convention already used in this repo.

### Phase 9: CI Coverage Gates

**Goal**: The Full Pipeline stops merely running tests and starts enforcing them — a drop in coverage on either language blocks the build before anything is packaged or published.
**Depends on**: Phase 8 (a frontend coverage number is meaningless until a frontend suite exists)
**Requirements**: CICD-11, CICD-12
**Success Criteria** (what must be TRUE):

  1. The backend job produces a Go coverage profile and fails the pipeline when aggregate coverage is below 80%
  2. The frontend job runs Vitest with coverage and fails the pipeline when aggregate coverage is below 70%
  3. A coverage failure on either side blocks the downstream build/scan/release jobs — no image is built, scanned, or pushed to ghcr.io when a gate trips
  4. Both starting baselines are measured and recorded before enforcement, and the thresholds committed to CI are the required 80%/70% — not a number quietly lowered to fit whatever the baseline turned out to be

**Plans**: TBD

Plans:

- [ ] TBD (run /gsd-plan-phase 9 to break down)

Notes: Both gates edit the same file (`.github/workflows/full-pipeline.yml`), which is why they are one phase rather than two. If a measured baseline lands under its threshold, closing that gap with real tests is in scope for this phase; lowering the requirement is not. Backend extends the existing `test` job; frontend is a new job added to the parallel tier and to `build-scan`'s `needs:`.

### Phase 10: Event Retention Window

**Goal**: Users see recent release history instead of an ever-growing scroll, while the system keeps every row it needs to stay correct — nothing is ever deleted.
**Depends on**: Nothing new in v1.1 (builds on Phase 4 detection and Phase 6 history; independent of Phases 8-9)
**Requirements**: DATA-01, DATA-02
**Success Criteria** (what must be TRUE):

  1. An operator can set the retention window with an environment variable, and with it unset the window is 90 days
  2. The History UI and the events API return no event older than the retention window, consistently across every display path (feed, filters, pagination)
  3. No event row is deleted — an event aged past the window is still in the database, and the release it recorded still does not re-notify (dedup key intact)
  4. An artist whose entire visible history has aged out does not fall back into seed mode and does not re-announce its back catalogue on the next poll cycle
  5. A deluxe/tracklist-change baseline recorded before the window still fires a deluxe alert when that release group's tracklist later expands

**Plans**: TBD

**Design decision (locked, do not revisit during planning)**: soft-delete/filter, not hard delete. Retention is a read-side filter on display/API queries; rows stay in the table permanently so dedup keys, deluxe-change baselines (`events.track_count`), and the per-source seed-mode signal all survive. The hard-delete variants explored in research — including the `release_group_baselines` migration needed to make hard delete safe — are rejected. Success criteria 3, 4, and 5 exist specifically to prove the three failure modes hard delete would have reintroduced (dedup-key loss, seed-mode reset, baseline loss).

Plans:

- [ ] TBD (run /gsd-plan-phase 10 to break down)

### Phase 11: Bounded Concurrent Polling

**Goal**: A poll cycle works through the watchlist several artists at a time instead of one at a time, without breaking the rate limits, overlap guards, or detection correctness that v1.0 established.
**Depends on**: Phase 9 (land last, behind working coverage gates, so this milestone's highest-risk change is the one most protected against regression)
**Requirements**: PERF-01, PERF-02, PERF-03, PERF-04
**Success Criteria** (what must be TRUE):

  1. An operator can set the per-source worker-pool size via an environment variable (default in the 3-5 range), and a cycle over a multi-artist watchlist finishes measurably faster than the sequential baseline it replaces
  2. Concurrent polling stays inside each source's existing rate limit — no burst above the configured per-second ceiling — and each source's cycle-overlap guard still skips a new cycle while the prior one for that source is running
  3. A single artist's polling failure is logged and skipped: the rest of that cycle's artists are still polled and their events still recorded, and the cycle does not abort
  4. Two artists sharing a release group cannot lose a deluxe-change baseline update — a test that races them asserts the final stored baseline is correct, and the suite passes under `go test -race`

**Plans**: TBD

Plans:

- [ ] TBD (run /gsd-plan-phase 11 to break down)

Notes: Criterion 4 requires replacing today's check-then-act baseline read/write with a database-level compare-and-set; `-race` alone will not catch this logical race, so the test must assert final-state correctness. Criterion 1's speedup must be measured during verification, not assumed — confirm the DB pool has not become the new bottleneck. Existing poller test doubles must become concurrency-safe.

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation — Data Layer, Config & Health | 5/5 | Complete    | 2026-08-05 |
| 2. Watchlist Core | 8/8 | Complete    | 2026-08-06 |
| 3. External Clients & Search | 4/4 | Complete    | 2026-08-07 |
| 4. Detection Engine | 4/4 | Complete    | 2026-08-08 |
| 5. Discord Notifications | 3/3 | Complete    | 2026-08-08 |
| 6. Frontend & Release History | 4/4 | Complete    | 2026-08-11 |
| 7. Containerization & CI/CD Pipeline | 4/4 | Complete    | 2026-08-12 |
| 8. Frontend Test Suite | 4/5 | In Progress|  |
| 9. CI Coverage Gates | 0/TBD | Not started | - |
| 10. Event Retention Window | 0/TBD | Not started | - |
| 11. Bounded Concurrent Polling | 0/TBD | Not started | - |

## Backlog

### Phase 999.1: Search result popularity sorting and same-name disambiguation (BACKLOG)

**Goal:** [Captured for future planning] — Sort watchlist artist search results by popularity and improve disambiguation between same-named artists (e.g. multiple "Drake"s), so the intended artist isn't buried under less-relevant same-named matches.
**Requirements:** TBD
**Plans:** 0 plans

Context (captured during Phase 6 UAT, 06-04): MusicBrainz's search API doesn't rank by popularity, and its `disambiguation` field is community-sourced and often blank for lesser-known same-named artists. The Watchlist search UI already renders `disambiguation` when present (`SearchResultsColumns.tsx`) — the gap is upstream ranking, not the UI. Likely needs a popularity signal (Deezer search results carry fan-count data not currently captured by `internal/deezer`) and/or better MusicBrainz result ranking in `internal/httpserver/search.go`.

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.2: Deezer artist-art backfill for MusicBrainz-only artists (BACKLOG)

**Goal:** [Captured for future planning] — Homepage artist cards render hero-sized artist art, but MusicBrainz carries no artist images; any watchlisted artist added without a confident Deezer match at add-time has `image_url = NULL` forever and renders with no art, which stands out badly at hero size.
**Requirements:** TBD
**Plans:** 0 plans

Context (captured 2026-08-12, brainstorming session): `artists.deezer_id`/`artists.image_url` already exist and get populated by `UpsertArtist` when a Deezer match was available at add-time (Phase 2) — the gap is a one-time/on-demand backfill for artists that never got a match, not new schema. Proposed approach: Deezer artist-name search (`internal/deezer`) filtered to close name equality as the primary signal; a shared album/release title used only as a tie-breaker when multiple same-named Deezer artists come back, never as the sole match signal (album titles collide across unrelated artists and MB/Deezer titles diverge in casing/edition tags); fail closed to "no art" on low confidence rather than risk attaching the wrong artist's photo. Note: PROJECT.md's Out of Scope explicitly rejects full "dual-source (MusicBrainz + Deezer) reconciliation" as too complex for the payoff — this is a narrower, one-directional, art-only slice (no event/detection logic touched), but sits in the same territory and should be scoped deliberately, not assumed in by default. Intended to be structured into v1.2 alongside Phase 999.1.

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)
