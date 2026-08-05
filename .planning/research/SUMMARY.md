# Project Research Summary

**Project:** drop-tracker
**Domain:** Self-hosted Go release-tracking service (HTTP API + cron poller + Discord notifier + embedded React SPA), with a CI/CD-pipeline-maturity portfolio goal
**Researched:** 2026-08-04
**Confidence:** MEDIUM

## Executive Summary

drop-tracker is a single-binary Go service that watches a curated list of hip-hop/reggaeton/R&B artists across MusicBrainz and Deezer, detects new releases (and, distinctively, guest features and deluxe/tracklist changes), and pushes alerts to Discord. Experts building this shape of tool (Lidarr, deemon, and comparable arr-stack/self-hosted monitors) converge on the same backbone: key "new release" detection off MusicBrainz release-group ID (not release ID, which churns on remasters/reissues), treat Deezer as a faster-but-flatter secondary signal, and keep the whole thing as one process with a shared service layer that both HTTP handlers and cron jobs call into. The stack is already substantially locked by PROJECT.md (chi, robfig/cron, sqlc, pgx, golang-migrate, React/Vite via go:embed) and research confirms every locked choice is sound and current -- the main additions are pgx/v5 as the concrete driver, caarlos0/env/v11 for config, go-chi/httplog/v3 for logging, and golang.org/x/time/rate for API throttling.

The recommended approach is a phased build: data layer and service layer first (testable without HTTP or cron existing), external API clients in parallel (independently testable via httptest.Server), then the poll-diff-notify pipeline, then the frontend, then containerization, then the CI/CD Full Pipeline. This order matches both the architecture research dependency graph and the feature research MVP definition (search-to-add, watchlist CRUD, scheduled polling, new-release detection, Discord notify, /health -- ship this before the harder guest-feature and deluxe-detection cases).

The dominant risk is not building features but building them unsafely under real-world load: MusicBrainz fully blocks (not throttles) clients exceeding about 1 req/sec, robfig/cron has no overlap protection so a slow poll cycle can double-fire, and naive "have I seen this ID" diffing produces both duplicate notifications (crash mid-cycle) and missed updates (content edits ignored). All three must be designed in from the first phase that touches them -- rate limiting in the client, SkipIfStillRunning in the scheduler, and a content-hash/outbox pattern in the diff engine -- because retrofitting them after the naive version ships is expensive. On the CI/CD side, the main risks are ordering (scan/publish must happen after cheap checks, and gitleaks/Trivy must gate the ghcr.io push) and supply-chain hygiene (SHA-pin third-party security actions, not tags -- this was a real March 2026 incident).

## Key Findings

### Recommended Stack

Go 1.23+ with chi v5, robfig/cron v3, sqlc + pgx/v5, golang-migrate v4, and a React 19/Vite SPA embedded via go:embed -- all already locked in PROJECT.md and confirmed current/sound by research. New additions to fill gaps: caarlos0/env/v11 for env-var config (lighter than viper), log/slog + go-chi/httplog/v3 for structured JSON logging with request-ID correlation, and golang.org/x/time/rate for per-source rate limiting. External API clients (MusicBrainz, Deezer) and the Discord notifier should be hand-rolled thin net/http wrappers -- no viable off-the-shelf client exists for the former two, and a webhook POST needs no library at all. CI/CD tooling is fully specified: golangci-lint v2 (new config schema), Trivy, gitleaks, syft/SBOM, and svu for Go-native semantic versioning (avoids pulling a Node toolchain in just for tagging, though semantic-release GitHub-release step still needs one).

**Core technologies:**
- Go 1.23+/chi v5/robfig/cron v3/sqlc+pgx/golang-migrate -- already locked, versions verified current
- React 19 + Vite, embedded via go:embed -- static bundle, no Node runtime needed at runtime
- caarlos0/env/v11 -- struct-tag env config, avoids viper weight/complexity
- golang.org/x/time/rate -- mandatory per-source rate limiting for MusicBrainz/Deezer

### Expected Features

**Must have (table stakes) -- matches PROJECT.md Active requirements:**
- Search-to-add artist (MusicBrainz + Deezer search proxy)
- Watchlist CRUD (add/remove/list)
- Scheduled polling (robfig/cron)
- New-release detection keyed on MusicBrainz release-group-id
- Discord webhook notification with release metadata (title, artist, cover, date, type)
- /health liveness/readiness endpoint
- Idempotent "seen" store -- no duplicate alerts

**Should have (competitive differentiators):**
- Guest-feature detection as a distinct alert type -- the single most genre-relevant gap no comparable consumer/OSS tool fills for hip-hop/reggaeton/R&B
- Deluxe/tracklist-change detection (track-list-level diff within a release-group)
- Per-artist notification preferences / release-type filtering
- Audit/history view of detected diff events (also a good CI/CD-observability demo)

**Defer (v2+):**
- Dual-source (MusicBrainz + Deezer) reconciliation -- real complexity (cross-source entity matching), narrower payoff than nailing core detection first
- Producer/label tracking -- different data model, already out of scope per PROJECT.md
- Anti-features to actively avoid: recommendation engine, auto-download/acquisition, multi-user auth/SSO, in-app playback, mobile push, full historical backfill, audio fingerprinting

### Architecture Approach

Single cmd/dropctl binary wiring three surfaces (HTTP via chi, cron scheduler via robfig/cron, embedded SPA) that all funnel into one shared internal/service layer -- handlers and cron jobs are both thin translators into service calls, never containing business logic themselves. External integrations (MusicBrainz, Deezer, Discord) live behind small interfaces in their own packages, real HTTP clients in production, httptest.Server-backed fakes in tests. Data access goes through sqlc-generated code wrapped in a repository layer, never imported directly by handlers/scheduler. The frontend is dual-mode: Vite dev server + reverse proxy locally, embed.FS + SPA-fallback routing in production -- the only point where the Go and Node toolchains touch.

**Major components:**
1. internal/service (shared core) -- watchlist CRUD rules, poll orchestration, diff engine, notify decisions; the load-bearing testability boundary
2. internal/musicbrainz / internal/deezer (adapters) -- real HTTP clients behind a ReleaseSource interface, rate-limited, tested via httptest.Server
3. internal/store (sqlc + repository wrapper) -- type-safe generated Postgres queries, domain-shaped methods over them
4. internal/api (chi) + internal/scheduler (robfig/cron) -- thin entry points calling into the service layer, never containing logic themselves
5. internal/notify/discord.go -- Discord webhook formatting/posting behind a Notifier interface
6. web/ (React/Vite) -- embedded via go:embed in prod, proxied to Vite dev server locally

### Critical Pitfalls

1. MusicBrainz rate-limit violations get you fully blocked, not throttled -- build one shared rate.Limiter (1 req/sec) used by every call site (poller AND search-proxy) from day one, plus a descriptive User-Agent.
2. robfig/cron double-fires on slow ticks -- wrap poll jobs with cron.SkipIfStillRunning; never use raw AddFunc.
3. Diff-against-seen-store race conditions cause duplicate or missed notifications -- key diffs off a content hash (not just ID existence), use a transactional-outbox pattern so detection (exactly-once) is decoupled from delivery (retryable).
4. go:embed ships a stale/dev-only frontend build -- enforce npm run build before go build in CI/Dockerfile, add a build-time guard for dist/index.html, implement the SPA fallback route from the start.
5. CI pipeline ordering hides failures / leaks secrets -- order lint-test-gitleaks-build-Trivy-SBOM-release-push; never push to ghcr.io before security gates pass; SHA-pin third-party security actions (real supply-chain incident precedent).

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Foundation -- Data Layer + Config + Service Skeleton
**Rationale:** Nothing else can be meaningfully tested end-to-end without Postgres schema, migrations, and sqlc wiring; establishing the zero-padded migration convention and CI migrate-generate-diff check here is cheap now, expensive to retrofit (Pitfall 9).
**Delivers:** Postgres schema/migrations, sqlc-generated store + repository wrapper, internal/config (caarlos0/env), internal/logging (slog + httplog), project skeleton (cmd/dropctl, internal/service stubs).
**Uses:** golang-migrate, sqlc, pgx/v5, caarlos0/env/v11
**Avoids:** Pitfall 9 (sqlc/migrate drift)

### Phase 2: Watchlist Core (CRUD + HTTP API)
**Rationale:** Service layer should exist and be tested before the HTTP layer is built out in full -- building HTTP-first tends to pull business logic into handlers (Anti-Pattern 1/2). This is also the simplest vertical slice, proving the shared-service pattern end-to-end.
**Delivers:** WatchlistService (business rules), chi HTTP layer with /api/watchlist CRUD + /health, unit tests against repository fakes.
**Addresses:** Watchlist CRUD, /health endpoint (FEATURES.md P1)
**Avoids:** Anti-Pattern 1/2 (logic leaking into handlers)

### Phase 3: External API Clients + Search-Proxy
**Rationale:** Independently buildable/testable with httptest.Server from day one -- no dependency on phases 1-2 DB layer, so can run in parallel, but sequenced here to unblock the diff engine next. Rate limiting must be foundational to the client, not bolted on later.
**Delivers:** internal/musicbrainz, internal/deezer clients (rate-limited, descriptive User-Agent), /api/search proxy endpoint reusing the same clients.
**Addresses:** Search-to-add artist (FEATURES.md P1)
**Avoids:** Pitfall 1 (MusicBrainz rate-limit blocking)

### Phase 4: Diff Engine + Poll Service + Scheduler
**Rationale:** Depends on both the service+repository foundation (Phase 1-2) and external clients (Phase 3) -- the natural "core detection" phase once CRUD and clients both exist. Get the outbox/content-hash design right here since retrofitting after the notifier assumes "detect and notify in one step" means redesigning both.
**Delivers:** New-release detection keyed on release-group-id, content-hash-based "seen" diffing, internal/scheduler with SkipIfStillRunning.
**Addresses:** Scheduled polling, new-release detection (FEATURES.md P1)
**Avoids:** Pitfall 2 (cron overlap), Pitfall 3 (diff race conditions)

### Phase 5: Discord Notifier
**Rationale:** Small, independently buildable in parallel with Phase 4, wired into PollService once both exist. Must be designed as retryable-but-idempotent from the start, tying into Phase 4 outbox key.
**Delivers:** internal/notify/discord.go, serial-queued webhook posting honoring Retry-After, embed-formatted release metadata.
**Addresses:** Discord webhook notification (FEATURES.md P1)
**Avoids:** Pitfall 5 (Discord rate limits/message-size on release bursts)

### Phase 6: Frontend (React/Vite SPA)
**Rationale:** Can start as early as the Watchlist CRUD + search-proxy API contracts (Phases 2-3) exist; develops against the Vite dev server + API proxy, independent of go:embed until near the end.
**Delivers:** Watchlist management UI, search-to-add flow, basic release feed.
**Addresses:** UI surface for all P1 features

### Phase 7: go:embed Integration + Containerization
**Rationale:** Deliberately late -- ties frontend into the Go binary for the first time; should be verified with an image-size/non-root smoke test before CI wires Trivy against it.
**Delivers:** Dual-mode serving (embed.FS in prod / Vite proxy in dev), SPA fallback route, multi-stage distroless Dockerfile, docker-compose for local dev.
**Avoids:** Pitfall 4 (stale/empty embed), Pitfall 8 (non-root/bloat)

### Phase 8: CI/CD Full Pipeline
**Rationale:** Lint/test jobs can be scaffolded from the first commit, but full build-and-scan/release jobs depend on Phase 7 Dockerfile existing. Stage ordering must be correct from the start -- reordering later means rewriting needs across the whole workflow.
**Delivers:** GitHub Actions pipeline: lint-test-gitleaks-build-Trivy scan-SBOM-semantic-release (via svu)-ghcr.io push, all third-party security actions SHA-pinned.
**Avoids:** Pitfall 6 (bad ordering/caching), Pitfall 7 (semantic-release Go misconfig)

### Phase 9 (v1.x, post-validation): Guest-Feature and Deluxe Detection
**Rationale:** Builds on Phase 4 plumbing but requires materially harder query/diff logic (artist-credit parsing, track-list-level diffing within a release-group) -- trigger once core new-release detection is proven stable in practice.
**Delivers:** Guest-feature alert type, deluxe/tracklist-change alert type, per-artist notification preferences, audit/history view.
**Addresses:** FEATURES.md P2 differentiators -- the genre-specific competitive edge

### Phase Ordering Rationale

- Data layer (Phase 1) is the true foundation per the architecture research build-order graph; external clients (Phase 3) have no DB dependency and could run in parallel but are sequenced to keep the roadmap linear for a solo builder.
- Service layer must precede full HTTP buildout (Phase 2 before HTTP depth) to avoid Anti-Pattern 1/2 (logic leaking into handlers/scheduler).
- The three critical concurrency/reliability pitfalls (rate limiting, cron overlap, diff race conditions) are each assigned to the exact phase that first touches that concern, per PITFALLS.md explicit phase mapping -- this avoids the "looks done but isn't" trap of shipping naive versions that do not hold up at real watchlist scale.
- CI/CD depth is deliberately last-but-incremental: lint/test scaffolding is cheap and can start immediately, but the security-gated build/scan/release chain needs a real Dockerfile (Phase 7) to operate on.
- Guest-feature/deluxe detection (the biggest differentiators) are explicitly deferred to v1.x per FEATURES.md MVP definition -- shipping core new-release detection reliably first is the higher-priority validation step.

### Research Flags

Phases likely needing deeper research during planning:
- Phase 4 (Diff Engine): Outbox pattern + content-hash diffing design has no single canonical reference for this exact domain (music release diffing) -- PITFALLS.md notes this is cross-inferred from general idempotent-consumer/outbox patterns, not a documented "how real tools solve this."
- Phase 9 (Guest-Feature/Deluxe Detection): No comparable tool in the researched landscape implements this -- MusicBrainz artist-credit query shape and false-positive mitigation strategy will need hands-on API exploration during planning.
- Phase 8 (CI/CD Pipeline): semantic-release-in-a-Go-repo configuration and SHA-pinning workflow specifics are well-documented individually but the exact svu+Docker tag integration is a novel composition worth validating against current tool versions at plan time.

Phases with standard patterns (skip research-phase):
- Phase 1 (Data Layer): sqlc + golang-migrate + pgx is a well-trodden, thoroughly documented combination.
- Phase 2 (Watchlist CRUD): Standard REST CRUD over chi + Postgres, no novel patterns.
- Phase 3 (External Clients): Rate-limited net/http client pattern is standard; MusicBrainz/Deezer API shapes are documented in FEATURES.md/ARCHITECTURE.md already.
- Phase 7 (Containerization): Multi-stage distroless Docker build is a well-established pattern, fully specified in ARCHITECTURE.md/PITFALLS.md.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM | Versions verified directly against pkg.go.dev/GitHub Releases, not training-data recall; no single HIGH-tier curated-docs source, but cross-checked |
| Features | MEDIUM | Data-model claims (MusicBrainz release-group/release/artist-credit) are solidly sourced from official docs; comparable-tool feature claims are LOW-confidence web search corroborated across 6+ independent products |
| Architecture | MEDIUM | Each sub-pattern (Go layout, go:embed+Vite, chi, sqlc, Docker, GH Actions) is well-established and cross-corroborated, but no single canonical spec exists for this exact combination |
| Pitfalls | MEDIUM | Cross-checked against official docs, GitHub issues/discussions, multiple independent write-ups; no single-source claims treated as authoritative |

**Overall confidence:** MEDIUM

### Gaps to Address

- False-positive/false-negative diffing strategy for deluxe editions: No authoritative "how real tools solve this" documentation exists (LOW confidence sub-finding within FEATURES.md) -- plan to write explicit test scenarios during Phase 9 planning rather than assume a solved pattern.
- Deezer rate-limit numbers: Not officially/canonically documented (corroborated only via community sources) -- build defensive backoff in Phase 3 even without a hard published number, and validate empirically.
- Dual-source (MB+Deezer) reconciliation: Explicitly deferred past v1 as a real complexity/entity-matching problem; if pulled forward, treat as needing its own research pass at that time.
- semantic-release + svu + Docker tag composition: Verify the exact integration approach (svu computes version, feeds into docker/build-push-action tag) against current tool docs when Phase 8 is planned, since this is a novel composition not found as a single documented recipe.

## Sources

### Primary (HIGH confidence)
- go:embed draft design doc (Go team, official proposal)
- go-chi/chi official repo/docs
- aquasecurity/trivy-action official GitHub Action
- GitHub Docs -- dependency caching reference
- MusicBrainz official API/Rate Limiting docs
- Discord Developer Docs -- Rate Limits

### Secondary (MEDIUM confidence)
- pkg.go.dev and GitHub Releases -- direct version verification for chi, cron, pgx, sqlc, golang-migrate, golangci-lint, gitleaks, trivy-action, sbom-action
- golang-standards/project-layout -- community Go layout conventions
- Multiple independent Vite+go:embed integration write-ups (tushar.ch, dev.to)
- Three Dots Labs -- Clean Architecture in Go (corroborates service/handler/repository layering)
- Trivy supply-chain incident advisories (upwind.io, aquasecurity GHSA) -- SHA-pinning rationale
- semantic-release official configuration docs

### Tertiary (LOW confidence)
- Comparable-tool feature claims (MusicHarbor, MusicButler, crabhands, BEEPR, deemon, Releasarr) -- web search, no official specs, corroborated across 6+ sources but individually unverified
- Deezer rate-limit specifics -- no canonical official doc found, pieced together from support docs and community GitHub issue discussion
- "How real tools solve deluxe/false-positive diffing" -- no authoritative source found; inferred from MusicBrainz data-model facts and Lidarr documented behavior

---
*Research completed: 2026-08-04*
*Ready for roadmap: yes*
