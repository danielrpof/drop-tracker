# drop-tracker

## What This Is

A Go-based release tracker for hip-hop, reggaeton, and R&B: users maintain a watchlist of artists (and later albums/producers) via a React UI, a scheduler polls MusicBrainz and Deezer for those artists, diffs results against a Postgres "seen" store to detect new releases, guest features, and deluxe/tracklist changes, and posts alerts to a Discord webhook. The primary purpose is a portfolio piece for practicing real CI/CD and DevOps pipelines — the music-tracking domain is the vehicle, the pipeline maturity is the point.

## Core Value

A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice (lint, test, security scan, SBOM, versioned image publish, and eventually automated deploy).

## Current State

**Shipped:** v1.2 Cleanup & Display Fixes (2026-08-24)

v1.0's four peer-reviewed gaps are closed without changing user-facing behavior: the frontend has a real Vitest + RTL test suite, CI now blocks merges on coverage regressions (80% backend / 70% frontend), event history has a configurable retention window with zero data loss to detection state, and polling runs several artists at a time per source through a bounded, race-safe worker pool. A follow-on tech-debt phase (11.1) closed everything the milestone audit flagged, including a real accessibility bug in the History filter UI.

v1.2 then closed the backlog and outstanding display bugs: Phase 12 fixed the `CoverArt.tsx` error-state-never-resets bug and added Deezer-fan-count-based search popularity ranking with MusicBrainz country-fallback (absorbing backlog Phase 999.1). Phase 13 fixed three more user-facing display/data bugs — History cards with no release date, guest-feature cards with no album art, and MusicBrainz artist art never rendering — the last via a new hand-rolled `internal/artistart` matcher (strict close-name + guarded tie-break, fail-closed by default) wired into both add-time and a cooldown-bounded startup backfill sweep (absorbing backlog Phase 999.2).

## Current Milestone: v1.3 Continuous Deployment

**Goal:** Ship the app automatically to a self-hosted VPS on every merge to main, behind a passphrase gate, and close the last CI reporting gap.

**Target features:**
- GitHub Actions deploy job: after the release job publishes the versioned image to ghcr.io, SSH to the VPS, `docker compose pull` + `up -d` the new pinned tag, poll `/health`, auto-rollback to the previous image on failure (DPLY-01)
- Instance passphrase gate — chi middleware, one env-var passphrase + session cookie, applied to all routes except `/health` — protects the watchlist and Discord webhook on a public URL
- Boot-time migrations remain the migration path; the `/health` deploy gate + rollback catches a bad migration; migrations must stay backward-compatible (expand/contract) so a rollback is safe
- PR comment reporting backend + frontend coverage diff vs. the main baseline (CICD-13)

**Considered and rejected this cycle:** a multi-user profile system (accounts, per-user watchlists, cross-device sync). Self-host-per-person plus the existing server-side Postgres already delivers data isolation and cross-device access once DPLY-01 lands; multi-user auth stays out of scope (see below).

<details>
<summary>Previous Milestone: v1.2 Cleanup & Display Fixes (shipped 2026-08-24)</summary>

**Goal:** Close the two remaining backlog items plus a round of History-tab display bugs found in everyday use, without adding new capability.

**Target features:**
- `CoverArt.tsx` stale-placeholder fix, shared across History/Watchlist/search rows
- Deezer fan-count-based search popularity ranking, MusicBrainz country-code disambiguation fallback (absorbed backlog Phase 999.1)
- History-card release dates for guest-feature and deluxe-change events
- Guest-feature release cards render album art
- MusicBrainz artist art via a fail-closed `internal/artistart` matcher, wired into add-time and a startup backfill sweep (absorbed backlog Phase 999.2)

</details>

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
- ✓ `CoverArt.tsx` error-state reset on `src` change, plus Deezer-fan-count-based search popularity ranking and MusicBrainz country fallback (absorbing backlog Phase 999.1) — Phase 12
- ✓ History-card release dates (single/feature/deluxe), guest-feature album art via a new per-recording MusicBrainz lookup, and MusicBrainz/Deezer artist art via a hand-rolled `internal/artistart` matcher wired into both add-time and a cooldown-bounded startup backfill sweep (absorbing backlog Phase 999.2) — Phase 13
- ✓ Instance passphrase gate — optional `INSTANCE_PASSPHRASE` enables an HMAC-SHA256 signed-cookie session gate over every route except `/health`, `POST`/`DELETE /session`, and the SPA shell; inert (no gate, no CSRF middleware) when unset so local dev, docker-compose, CI, and the test suites are unchanged (GATE-07); per-IP `rate.Limiter` + fixed comparison delay + a global brute-force counter that fires exactly one Discord alert (count + window only); React passphrase screen conforming to the phase UI-SPEC, plus a browser-session-scoped Log out control gated on a self-identifying `X-Instance-Gated` response marker — Phase 14
- ✓ PR coverage-diff comment — every same-repo PR carries one sticky, always-current, never-blocking comment showing backend + frontend coverage and the signed pp delta vs. the main baseline; a stdlib-only `cmd/coverage-report` tool renders the body from compile-time literals + tool-computed numbers only (no path/JSON interpolation), the `coverage-gate` now measures via that same tool so gate and comment share one algorithm (D-17), the main baseline is published as a SHA-keyed Actions cache entry (D-20) with no PR job ever recomputing main's coverage, and the `coverage-comment` job is report-only (in no `needs:` graph, job-level `continue-on-error`) so it can never block a merge (CICD-13, CICD-14) — Phase 15
- ✓ Rollback-safe migrations — an ahead-of-source no-op guard in `internal/db/migrate.go` (`maxSourceVersion` + `runMigrationsWithSource`, proven against real Postgres) lets the previous release's binary boot healthy against a schema the newer release already applied; a stdlib-only `cmd/migration-check` CI guard reds destructive DDL (`DROP`/`RENAME`/type-narrowing/`ADD COLUMN NOT NULL`) with an N-1-rule message, and a non-overridable cross-reference against the previous release's `queries/*.sql` catches drops of still-referenced objects; two unconditional CI jobs (`migration-check`, `n1-boot`) with step-gated expensive work, plus `internal/db/migrations/README.md` documenting the expand/contract rule where a migration author meets it (MGRT-01, MGRT-02) — Phase 16

### Active

- [ ] VPS SSH-based deploy step, automated on merge to main, with `/health`-gated auto-rollback (v1.3 — DPLY-01)

### Out of Scope

- Python implementation — considered, rejected in favor of Go for portfolio differentiation and better fit with the CI/CD/DevOps practice goal
- Multi-user auth / accounts / per-user profiles / cross-device real-time sync — evaluated in v1.3 scoping and rejected; single-operator self-host per person plus server-side Postgres already covers isolation and multi-device access, and OAuth/session/RBAC is an orthogonal complexity spike that doesn't showcase the CI/CD practice goal
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
- Current codebase size (as of Phase 13 close, 2026-08-24): ~26,900 LOC Go across 82 files, ~4,000 LOC TypeScript/TSX across 38 files (`web/app/`, excludes generated build output).
- The `CoverArt.tsx` image-load-error-never-resets bug (noted at v1.1 close as pre-existing, non-blocking tech debt) was fixed in Phase 12.
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
| Strict close-name equality + guarded-containment title tie-break for MusicBrainz→Deezer artist-art matching, fail-closed on any ambiguity, popularity (`NbFan`) explicitly excluded as a signal | A wrong-artist photo misrepresents identity and is worse than no photo — the primary risk this plan's own threat model (T-13-06) identified; unguarded fuzzy matching or "take the most popular candidate" would silently violate that | Validated Phase 13 — `internal/artistart/match.go`, `NbFan` asserted absent from executable code by grep criterion, extensive fail-closed test coverage |
| Shared `ActivityGate` priority-yielding primitive instead of a second rate budget for coordinating add-time matching and the backfill sweep | A second independent rate budget would let combined outbound traffic exceed MusicBrainz's real ~1 req/sec external ceiling; yielding priority to interactive adds keeps total traffic bounded by the existing single limiter | Validated Phase 13 — `internal/artistart/activity.go`, atomic-counter + `sync.Once`-guarded, proven via concurrent stress tests |
| Optional single-secret instance gate (HMAC-signed cookie), not multi-user auth | v1.3 scoping rejected accounts/RBAC as orthogonal complexity; the deployment threat is "the instance is briefly public-and-open", which one shared passphrase closes. `INSTANCE_PASSPHRASE` unset ⇒ `gate == nil` ⇒ data routes registered flat with no middleware (GATE-07 inert path) | Validated Phase 14 — `internal/authgate`; structural route exemptions (never path-string matched); server 401 is the sole enforcement, all client auth/gate flags are presentation-only |
| Client discovers "instance is gated" from a self-identifying `X-Instance-Gated: 1` response marker on gate-passing 2xx responses, latched one-way into `authStore.gateActive` | A browser session carrying a valid `dt_session` cookie sees no 401 and types no passphrase, so the SPA otherwise had no signal to render the Log out control (G-14-3). Marker's only write site is inside `Authenticate`, registered solely in the `gate != nil` branch, so an ungated instance emits nothing structurally (D-18) | Validated Phase 14 (gap closure 14-07) — Go marker matrix + jsdom header→apiFetch→store→rendered-control end-to-end; operator real-browser UAT PASS |
| Config forwarded through docker-compose `env_file: .env` **and** `environment: ${INSTANCE_PASSPHRASE:-}`, with a secret-free boot-status log line | G-14-1: editing `.env.example` or a host-shell `export` silently did nothing — the container only ever read `.env`, so the gate shipped inert unnoticed | Validated Phase 14 (gap closure 14-05) — `logInstanceGateStatus` reports ACTIVE/INERT on every start, prints no secret; compose regression test |
| One coverage algorithm for both the merge-blocking gate and the PR comment — `coverage-gate` shells `go run ./cmd/coverage-report --mode=total` instead of scraping `go tool cover -func` (D-17) | A separate measurement path for the report would let the gate and the comment disagree on the same commit; sharing the tool keeps them provably identical, and the threshold literal stays greppable in the Makefile (Phase 09 posture) | Validated Phase 15 — two greppable call sites; `backendTotalPct` merges duplicate profile blocks so it matches `go tool cover -func` on a real merged `go test ./...` profile; cutover margin measured at 90.03% vs. the 80 floor |
| Main coverage baseline published as a SHA-keyed Actions cache entry, restored by bare-prefix restore-key; presence decided by `cache-matched-key` + on-disk sidecar, never `cache-hit` (D-20) | The next PR needs main's numbers to compute a delta without any PR job re-running main's test suite; a cache entry written only on green pushes to `refs/heads/main` gives that with no extra infra | Validated Phase 15 (live, PR #3) — merge run writes `coverage-baseline-main-{backend,frontend}-<sha>`; a fresh PR restores it via the prefix key and shows numeric deltas + a `baseline: main@<sha>` footer; no PR job recomputes main's coverage |
| `coverage-comment` is a report-only CI job — real job with job-scoped `pull-requests: write`, but in no `needs:` graph and carrying job-level `continue-on-error: true` | The comment must never gate a merge (CICD-13's "never-blocking" property); the producing gate jobs (`test`/`frontend-test`) still go red on a real coverage drop, but the comment posts the lower number with a warning glyph and stays green | Validated Phase 15 (live, PR #2) — a deliberate coverage drop turned `frontend-test` red while `coverage-comment` stayed green and the PR stayed mergeable |
| Comment renderer emits only compile-time literals, tool-computed numbers, a tool-generated RFC3339 timestamp, and a 7–40-lowercase-hex-validated short SHA — no file path, profile line, or unvalidated JSON field is interpolated (T-15-03) | The rendered body is posted to a PR by a token-bearing job; interpolating untrusted profile/JSON content into it is a markdown/command-injection surface | Validated Phase 15 — golden-file tests (`TestRenderComment_NoUntrustedInterpolation`, `TestSHAValidation`); comment mode never returns a non-nil error, every bad input degrades to an "unavailable" row |
| Ahead-of-source migrations no-op instead of erroring — an explicit numeric `cur > maxSourceVersion` guard in `runMigrationsOnce`, gated on `verr == nil && !dirty` so a dirty schema still surfaces `ErrDirty` | Research Finding 1 (verified against pinned golang-migrate v4.19.1): `Up()` against a schema ahead of the binary's embedded set returns a hard error, not `ErrNoChange` — so the previous release's binary could not boot at all after a rollback. This is the phase's one deliberate runtime-behaviour change, relaxing 16-CONTEXT.md's "no runtime behaviour change" boundary (developer-approved) | Validated Phase 16 — `TestRunMigrationsWithSource_*` (5 cases) against real Postgres; guard is a numeric comparison, never a string-match on library error text |
| N-1 safety enforced by CI, not by memory — `cmd/migration-check` (stdlib-only SQL tokenizer, two finding classes, `allow-destructive` file-scoped annotation) + a non-overridable cross-reference against the previous release's `queries/*.sql`, plus the `n1-boot` job that boots the actual N-1 image against the branch schema | PITFALLS.md #8 (a non-backward-compatible migration turning a routine rollback into data loss) is the milestone's highest-cost failure and binds every future migration; it had to land before auto-rollback exists | Validated Phase 16 — `migration-check`/`n1-boot` unconditional jobs with step-gated expensive work (a job-level `if:` would skip `build-scan`); live-confirmed on scratch-branch runs 33974299591 / 33979094225 |
| All three Phase 16 CI jobs unconditional, expensive steps step-gated on `needs.changes.outputs.migrations_changed` | Research Finding 2 (verified live): a skipped `needs:` job *skips* its dependents rather than counting as success — a job-level `if:` on the guards would have skipped `build-scan` and `release` on ~95% of pushes | Validated Phase 16 — confirmed on a docs-only scratch push: both guards finished `success` (not `skipped`) and `build-scan` ran |
| `n1-boot` guard-adoption skip-green (`guardcheck` step) — statically inspects the resolved N-1 tag's `migrate.go` for the ahead-of-source guard token; if absent, skip-greens with a `::notice::` and self-clears once a guard-carrying release is N-1 | G-16-1: the N-1 image (`v1.7.0`) predates the guard Phase 16 introduced, so it *always* fails golang-migrate's ahead-of-source check regardless of migration safety — `n1-boot` gave no discriminating signal until a guarded release ships | Validated Phase 16 (gap closure, quick task 260905-et1) — confirmed live on runs 33978945980 / 33979094225; flagged as security UF-1 for a Phase 17 follow-up to re-assert the probe |

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
*Last updated: 2026-09-05 after Phase 16*
