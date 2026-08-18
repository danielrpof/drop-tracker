# drop-tracker

## What This Is

A Go-based release tracker for hip-hop, reggaeton, and R&B: users maintain a watchlist of artists (and later albums/producers) via a React UI, a scheduler polls MusicBrainz and Deezer for those artists, diffs results against a Postgres "seen" store to detect new releases, guest features, and deluxe/tracklist changes, and posts alerts to a Discord webhook. The primary purpose is a portfolio piece for practicing real CI/CD and DevOps pipelines — the music-tracking domain is the vehicle, the pipeline maturity is the point.

## Core Value

A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice (lint, test, security scan, SBOM, versioned image publish, and eventually automated deploy).

## Current State

**Shipped:** v1.1 Hardening & Scale Readiness (2026-08-17)

v1.0's four peer-reviewed gaps are closed without changing user-facing behavior: the frontend has a real Vitest + RTL test suite, CI now blocks merges on coverage regressions (80% backend / 70% frontend), event history has a configurable retention window with zero data loss to detection state, and polling runs several artists at a time per source through a bounded, race-safe worker pool. A follow-on tech-debt phase (11.1) closed everything the milestone audit flagged, including a real accessibility bug in the History filter UI.

## Next Milestone Goals

Not yet scoped — run `/gsd-new-milestone` to define v1.2. Candidates carried in ROADMAP.md's Backlog:
- Phase 999.1: Search-result popularity sorting and same-name artist disambiguation (captured during Phase 6 UAT)
- Phase 999.2: Deezer artist-art backfill for MusicBrainz-only-matched artists (captured 2026-08-12)
- v2 requirements tracked in the outgoing REQUIREMENTS.md archive: VPS SSH deploy (DPLY-01), producer watchlist entities (WLST-07), PR coverage-diff comments (CICD-13), Playwright E2E suite (TEST-03)

<details>
<summary>Previous Milestone: v1.1 Hardening & Scale Readiness (shipped 2026-08-17)</summary>

**Goal:** Close four peer-reviewed gaps from v1.0 — frontend test coverage, CI coverage enforcement, events data retention, and concurrent polling — without changing user-facing behavior.

**Target features:**
- Vitest + React Testing Library unit/component test suite for the frontend
- CI coverage gates: 80% threshold for backend (Go), 70% threshold for frontend (Vitest)
- Events table retention: soft-delete/filter — rows older than 90 days hidden from display/API, detection state (dedup keys, deluxe-change baselines, seed-mode) left intact
- Bounded worker-pool concurrent per-artist polling (env-configurable pool size, default 3-5), replacing sequential polling, still respecting existing rate limiters

</details>

## Requirements

### Validated

- ✓ `/health` endpoint for liveness/readiness — Phase 01
- ✓ sqlc-generated type-safe DB queries + golang-migrate schema migrations — Phase 01
- ✓ Structured (slog-based) JSON logging — Phase 01
- ✓ `.env.example` documenting all config, secrets via env vars only — nothing real committed — Phase 01
- ✓ Watchlist CRUD API — add/remove/list artists, DB-backed in Postgres, with per-artist release-type filters and mutable notification-type preferences — Phase 02
- ✓ Search-proxy API endpoints — live search against MusicBrainz/Deezer so the UI can look up artists to add — Phase 03
- ✓ Scheduler (robfig/cron) polls MusicBrainz + Deezer per watchlist entry on a configurable interval — Phase 03
- ✓ Diff engine compares poll results against the Postgres "seen" store to detect: new releases, new guest features, deluxe/tracklist changes — Phase 04
- ✓ Notifier posts detected changes to a Discord webhook — Phase 05
- ✓ React (Vite) SPA UI for browsing/managing the watchlist, built and embedded into the Go binary via `go:embed` — single deployable image — Phase 06
- ✓ Multi-stage Dockerfile: slim (Alpine) base image, non-root user, single final image containing API+scheduler+notifier+UI — Phase 07
- ✓ docker-compose for local dev (app + Postgres) — Phase 07
- ✓ pre-commit hooks: golangci-lint, gitleaks — Phase 07
- ✓ GitHub Actions "Full Pipeline": golangci-lint (with gosec) + go vet + unit tests (httptest.Server-mocked MusicBrainz/Deezer) → Trivy fs + image scan → gitleaks secret scan → SBOM generation → svu semantic versioning/tagging → push image to GitHub Container Registry (ghcr.io) — Phase 07
- ✓ Vitest + React Testing Library frontend test suite — watchlist, preference-toggle, search, and history/event-card surfaces each covered, API boundary (`web/app/lib/api.ts`) mocked (no real network calls), shared `createRoutesStub` router-context helper — Phase 08
- ✓ CI coverage gate — 80% threshold for backend (Go), 70% threshold for frontend (Vitest); a coverage drop on either language blocks `build-scan`/`release` via `full-pipeline.yml`'s `needs:` graph — Phase 09
- ✓ Events table retention — soft-delete/filter; `EVENT_RETENTION_DAYS` (default 90) hides aged-out rows from `GET /events` and the History UI while detection-state queries (dedup keys, deluxe-change baselines, seed-mode signal) stay unfiltered against the full table — Phase 10
- ✓ Bounded worker-pool concurrent per-artist polling — env-configurable pool size (`MusicBrainzPollWorkers`/`DeezerPollWorkers`, default 3/5) for both MusicBrainz and Deezer poll cycles, still respecting existing rate limiters and overlap guards; deluxe-change baseline detection made race-safe with a single atomic CTE; connection pool `MaxConns` sized against the combined worker ceiling rather than `runtime.NumCPU()` — Phase 11
- ✓ v1.1 tech-debt cleanup (13 locked decisions closing residual items from `.planning/v1.1-MILESTONE-AUDIT.md`) — frontend test-coverage gaps filled, History filter dropdown legibility fixed (native `<select>` replaced with a hand-rolled combobox after a CSS-only fix was found not to work on Windows Chromium), `prettier --check` now gates CI, notification-loss window made log-observable, `PoolConfig`'s two parse errors differentiated, boot test hardened, coverage-filter regex anchored, Postgres port revert (5433→5432) committed with full history, and Phases 08/09/10's Nyquist `VALIDATION.md` files reconciled out of `draft` — Phase 11.1

### Active

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
- Current codebase size (as of v1.1 close, 2026-08-17): ~23,200 LOC Go across 72 files, ~3,500 LOC TypeScript/TSX across 33 files (`web/app/`, excludes generated build output). Backend coverage 83.5%+, frontend coverage 70%+, both CI-enforced.
- One pre-existing, non-blocking UI bug noted at v1.1 close (v1.1-MILESTONE-AUDIT.md, not introduced by any v1.1 phase): `CoverArt.tsx`'s image-load-error state never resets when `src` changes on a retained component instance, so a component that once failed to load keeps showing the placeholder even if a later `src` would succeed. Affects both History and Watchlist rows. Left open deliberately rather than fixed outside its own scoped phase — candidate for a small future cleanup phase.
- Windows dev-machine limitations remain (`go test -race` unusable — ThreadSanitizer allocation failure under memory pressure; musicbrainz.org TLS handshake fails over WSL2). Both are documented, waived, environmental, not code defects. See `.planning/WINDOWS.md`.

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
| React (Vite) SPA embedded via go:embed | Keeps deployable to a single Go binary/image while still using a real frontend stack | Validated Phase 06 — React Router SPA Mode (`ssr: false`), `internal/webassets` embeds `build/client` via `//go:embed all:build/client`, chi `NotFound` fallback serves `index.html` for client-side routes while explicit API routes still resolve first |
| Single Go binary/service architecture | Simpler CI/CD to start; still exercises full pipeline without microservice complexity | Validated Phase 07 — single multi-stage Dockerfile (Node SPA build → Go static build → Alpine runtime) is the sole deployable artifact; docker-compose runs it alongside Postgres for local dev |
| DB-backed watchlist with CRUD API | More realistic surface, more to test/lint/scan than a static config file | Validated Phase 02 — add/remove/list/update-preferences all live behind `internal/watchlist.Store`, the reusable domain surface later phases (search-proxy, poller) build on |
| Live search-proxy endpoints | Lets the UI look up artists/albums against MusicBrainz/Deezer directly, not just local DB | Validated Phase 03 — `GET /search` fans out concurrently to both sources, source-tagged, D-03 partial-failure contract (one source down never fails the whole request) |
| httptest.Server for HTTP mocking in tests | Stdlib-only, no extra test dependency | Validated Phase 03 — `internal/musicbrainz`/`internal/deezer` clients tested exclusively against `httptest.Server` fixtures per CLAUDE.md's no-live-calls-in-CI constraint |
| "Full Pipeline" CI/CD depth (lint+test+scan+SBOM+semantic-release+push) | Matches the project's primary goal of practicing real DevOps pipelines | Validated Phase 07 — single `full-pipeline.yml` workflow: vet/lint(gosec)/test/gitleaks/trivy-fs gates → build-scan (Trivy image scan, blocking) → release (svu version, ghcr.io push, SBOM, git tag), verified end-to-end via a real PR + merge (v0.2.1 published) |
| ghcr.io as image registry | Free, zero extra secrets, tightly integrated with GitHub Actions | Validated Phase 07 — `release` job pushes via the ambient `GITHUB_TOKEN`, package confirmed public and pullable without auth |
| Structured logging only for v1 (no Prometheus/Grafana yet) | Keeps initial scope tight; metrics can be layered on later | Validated Phase 01 — `log/slog` JSON logging via `go-chi/httplog`, wired with a secret-redaction pattern for the DB DSN |
| Phased deploy: local-only now, VPS SSH deploy later | Avoids committing to live infra before the app is feature-stable | — Pending |
| DSN/secret redaction on every error path that could reach logs or stderr | Connection-failure errors routinely embed the raw DSN with its password; Phase 01's security review (T-01-01) required scrubbing it before it reaches `slog` or a returned error | Validated Phase 01 — `redactDSN`/`redactError` helpers in `internal/db/migrate.go`, asserted by `TestRunMigrations_NeverLogsDSN` |
| Graceful shutdown via `signal.NotifyContext` + bounded `httpSrv.Shutdown` timeout | A container orchestrator stops the process with SIGTERM; without this, in-flight requests and the deferred `pool.Close()` are skipped | Validated Phase 01 — confirmed end-to-end under a real SIGTERM in WSL2 (UAT test 1) |
| Vitest + React Testing Library for frontend tests, jsdom environment | Matches the existing Vite toolchain; RTL steers toward user-visible-behavior assertions over implementation detail | Validated Phase 08 — 5 test files / 16 tests, `mockReset: true`, no `passWithNoTests` escape; the `frontend-test` CI job runs in `full-pipeline.yml`'s parallel tier but is deliberately not yet wired into `build-scan`'s `needs:` — that blocking wiring is Phase 09's job (CI Coverage Gates), which edits the same file |
| Hand-rolled Makefile coverage gate (backend) + Vitest `coverage.thresholds` (frontend), no third-party coverage-gating action | Both languages already have a coverage-producing invocation from Phase 07/08; a single greppable threshold literal per side is simpler to audit than a new GitHub Action | Validated Phase 09 — `Makefile`'s `coverage-gate` recipe fails closed on missing/empty/unparseable profiles; `web/vitest.config.ts` thresholds fail `pnpm test` non-zero below 70% on any of 4 axes; `build-scan.needs` extended to include `test`+`frontend-test`, confirmed live on a real GitHub Actions run (backend-red, frontend-red, and full-green cases all directly observed, `build-scan` correctly reported `skipped` on both red cases) |
| Soft-delete/filter retention, not hard delete | Rows must stay in the table permanently so dedup keys, deluxe-change baselines, and the per-source seed-mode signal all survive; hard delete would have reintroduced all three failure modes | Validated Phase 10 — zero `DELETE`/`TRUNCATE` in `queries/events.sql`; `TestRetention_DetectionStateQueriesStayUnfiltered` proves dedup keys, seed-mode signal, and deluxe baseline all survive retention filtering while `GET /events`/History UI correctly hide aged-out rows |
| Buffered-channel semaphore for bounded per-source polling concurrency, not a third-party worker-pool library | Stdlib-only; the concurrency shape needed (N-wide fan-out per cycle, wait for drain, per-artist panic isolation) is a handful of lines with `chan struct{}` | Validated Phase 11 — `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` (defaults 3/5) drive both cycles; existing per-source `rate.Limiter` and overlap guard proven to still hold under concurrency via real captured request timestamps |
| Atomic `FOR UPDATE`-locked `UPDATE...RETURNING` CTE for the deluxe-change baseline, replacing check-then-act read/write | Concurrent polling made the prior two-statement baseline update a real lost-update race once two artists could share a release group in-flight simultaneously | Validated Phase 11 — `AdvanceGroupTrackCountBaseline`, proven via a deliberate two-caller race test asserting final stored state is correct |
| Connection pool `MaxConns` sized against `MusicBrainzPollWorkers + DeezerPollWorkers` (+ headroom), not `runtime.NumCPU()` default | Bounded concurrency raises the number of simultaneous DB connections polling can need; the old default sizing was blind to the new worker-count knob | Validated Phase 11 (gap closure 11-05) — `poolMaxConnsForWorkers` clamps and computes explicitly, still honoring an operator's own `pool_max_conns` override |
| Hand-rolled accessible combobox to replace the native `<select>` for History filters | A CSS-only contrast fix for the native dropdown was disproven on Windows Chromium during Phase 11.1 UAT — the underlying legibility bug needed a real component fix, not styling | Validated Phase 11.1 — `aria-activedescendant`-wired combobox verified live in-browser on the failing platform |
| Blocking `prettier --check` added to the existing `frontend-test` CI job, not a new job | Formatting drift (40 files) had accumulated silently with no CI signal; reusing the job that already runs on every push avoids adding pipeline surface for a lint-adjacent check | Validated Phase 11.1 — `web/` tree reformatted once, gate now fails the build on any future drift |

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
*Last updated: 2026-08-17 after v1.1 milestone completion*
