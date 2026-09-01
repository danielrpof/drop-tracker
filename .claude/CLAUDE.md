<!-- Hand-maintained section — it sits above every GSD marker block below so `generate-claude-md` never overwrites it. Keep it first. -->

## Definition of Done — run before every commit

Every commit must clear the same gates CI enforces. Run them locally first, don't outsource the check to CI.

**Format frontend first.** If anything under `web/` changed, run `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before staging. Prettier's output and `prettier-plugin-tailwindcss` class ordering cannot be reproduced by hand, so hand-formatted TSX fails CI's `frontend-test` job.

1. `go vet ./...`
2. `golangci-lint run`
3. `make test` — integration suite; run `make db-up` first
4. `make coverage-gate` — 80% backend floor
5. `make sqlc-check` — local-only, no CI counterpart; CI will not catch sqlc drift
6. Web changes only: `cd web && corepack pnpm exec prettier --check "**/*.{ts,tsx}"` and `corepack pnpm test`

(Where bare `pnpm` is on PATH, drop the `corepack` prefix.)

**Never bypass the hooks with `git commit --no-verify`.** The hooks (gitleaks, golangci-lint `--fix`, prettier `--write`) are the fast local mirror of CI — skipping them only relocates the failure somewhere slower and more public. Install them with `make hooks`.

<!-- GSD:project-start source:PROJECT.md -->

## Project

**drop-tracker**

A Go-based release tracker for hip-hop, reggaeton, and R&B: users maintain a watchlist of artists (and later albums/producers) via a React UI, a scheduler polls MusicBrainz and Deezer for those artists, diffs results against a Postgres "seen" store to detect new releases, guest features, and deluxe/tracklist changes, and posts alerts to a Discord webhook. The primary purpose is a portfolio piece for practicing real CI/CD and DevOps pipelines — the music-tracking domain is the vehicle, the pipeline maturity is the point.

**Core Value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice (lint, test, security scan, SBOM, versioned image publish, and eventually automated deploy).

### Constraints

- **Tech stack**: Go (not Python) — chosen for portfolio differentiation and closer fit to systems/DevOps practice
- **Web framework**: chi router — stdlib-idiomatic, minimal dependency footprint
- **DB access**: sqlc for type-safe generated queries; golang-migrate for schema migrations
- **Scheduler**: robfig/cron for periodic polling
- **UI**: React + Vite, built and embedded into the Go binary via `go:embed` (no separate frontend deploy pipeline)
- **Architecture**: single Go binary/service (API, scheduler, notifier all in one process) — not split microservices
- **Registry**: GitHub Container Registry (ghcr.io) — uses `GITHUB_TOKEN`, no extra registry secret needed
- **Security**: all secrets via environment variables only; nothing real ever committed; gitleaks enforced in pre-commit and CI
- **Testing**: unit tests use `httptest.Server` to mock MusicBrainz/Deezer, no live external calls in CI

<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->

## Technology Stack

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.23+ (1.25 recommended) | Language/runtime | Locked by PROJECT.md. Use whatever the current stable toolchain is at project start — chi, sqlc, and golangci-lint all track recent Go releases closely. |
| `github.com/go-chi/chi/v5` | v5.3.1 | HTTP router | Already locked in PROJECT.md. v5 is the actively maintained, go.mod-native line (v1–v4 predate modules — do not use). 100% `net/http`-compatible, ~1000 LOC, zero external deps — every middleware in this stack (`middleware.RequestID`, `httplog`) composes on plain `http.Handler`. |
| `github.com/robfig/cron/v3` | v3.0.1 | Cron-based background poller | Already locked in PROJECT.md. Pinned at v3.0.1 since Jan 2020 — this is a **stable, feature-complete** package, not an abandoned one; its scope (parse cron expressions, run jobs on schedule) is small and hasn't needed churn. Still the de facto standard Go scheduler. |
| `github.com/sqlc-dev/sqlc` (CLI) | v1.31.1 | Type-safe generated Postgres queries | Already locked in PROJECT.md. Codegen from hand-written SQL avoids an ORM's runtime reflection/magic while keeping full SQL expressiveness — a good CI showcase (generate step is itself lint/test-able). |
| `github.com/jackc/pgx/v5` | v5.10.0 | Postgres driver | Not yet locked in PROJECT.md — needed as the underlying driver for sqlc. `pgx/v5` is the standard, highest-performance pure-Go Postgres driver and sqlc's primary supported target (vs. `database/sql` + `lib/pq`, which is legacy and in maintenance mode only). |
| `github.com/golang-migrate/migrate/v4` (CLI + `database/postgres` driver) | v4.19.1 | Schema migrations | Already locked in PROJECT.md. Plain up/down `.sql` files, CLI-driven, trivial to wire into a `make migrate` target and a CI step that runs migrations against a throwaway Postgres service container. |
| React + Vite | React 19.x, Vite 6.x/7.x (check `npm create vite@latest` at build time) | SPA UI | Already locked in PROJECT.md. Vite's `dist/` output is a plain static bundle — the only requirement for `go:embed` compatibility, no SSR/Node runtime needed in the final image. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/caarlos0/env/v11` | v11.4.1 | Config/settings (pydantic-settings equivalent) | **Use this.** Struct-tag driven env-var parsing (`env:"PORT" envDefault:"8080" required:"true"`), zero dependencies, ~1,000 downstream users. This is the closest Go analog to `pydantic-settings`: define one `Config` struct, call `env.Parse(&cfg)` once at startup, get typed values + required/default validation. See "What NOT to Use" for why not viper/envconfig. |
| `log/slog` (stdlib) | Go 1.21+ | Structured logging core | Use `slog.NewJSONHandler(os.Stdout, opts)` as the base logger for production JSON logs; `slog.NewTextHandler` for local dev readability (gate on an env var). No third-party logging library needed for the core — slog is the modern Go standard as of 1.21+. |
| `github.com/go-chi/httplog/v3` | latest (v3 module) | HTTP request logging middleware + request-id correlation | Maintained by the **chi org itself**, zero external deps, wraps `log/slog`, auto-integrates with chi's `middleware.RequestID`/`middleware.Recoverer`, outputs ECS/OpenTelemetry-compatible JSON. Prefer this over `samber/slog-chi` (third-party, less directly aligned with chi's own conventions) for a chi-based project. |
| `github.com/go-chi/chi/v5/middleware` (`RequestID`, `Recoverer`, `Logger` stdlib parts) | bundled with chi | Request ID generation, panic recovery | `middleware.RequestID` generates/propagates the id used for log correlation; pair with `httplog` so every log line in a request's lifecycle (API handler → poller-triggered notifier call, if applicable) carries the same id via `context.Context`. |
| MusicBrainz client (hand-rolled) | — | External API client | No official/canonical Go client exists worth adopting wholesale for this use case — write a small internal client (`internal/musicbrainz`) wrapping `net/http` directly. Set `User-Agent: drop-tracker/0.1.0 (+contact-url-or-email)` on every request (mandatory — see Pitfalls doc), and self-rate-limit to ~1 req/sec using `golang.org/x/time/rate`. |
| Deezer client (hand-rolled) | — | External API client | Same rationale — hand-roll `internal/deezer`. No auth needed for search/catalog endpoints. Self-rate-limit to Deezer's documented ~50 req/5s using `golang.org/x/time/rate`. |
| `golang.org/x/time/rate` | latest | Token-bucket rate limiting | Use one `rate.Limiter` per external API client (MusicBrainz, Deezer) to enforce the self-imposed request ceilings above — critical since both APIs will 429/503 or silently throttle non-compliant clients. |
| Discord notifier (hand-rolled, plain `net/http` POST) | — | Discord webhook posting | No dependency needed — a webhook POST is one `http.Post` call with a JSON body (`{"embeds": [...]}`). A library adds no value here; hand-roll `internal/discord` with a typed `Embed` struct matching Discord's schema (see PITFALLS.md for exact field constraints and rate limits). |
| `google/uuid` or stdlib `crypto/rand`-based id (chi's `middleware.RequestID` already provides one) | — | Request ID generation | chi's built-in `middleware.RequestID` is sufficient — don't add a UUID dependency purely for request-id purposes. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `golangci-lint` | Aggregated linting | Latest **v2.12.2** (v2 config format — v1 config schema is legacy/deprecated, don't copy old `.golangci.yml` examples that predate v2). Pin the exact version in CI via `golangci-lint-action@v9.3.0`'s `version:` input for reproducibility. |
| `go vet` | Compiler-adjacent static analysis | Runs as part of `go build`/`go test` implicitly, but also run explicitly in CI as its own fast-fail step before the slower `golangci-lint` pass — this matches the PROJECT.md pipeline description. |
| `sqlc` (CLI) | SQL → Go codegen | `sqlc generate` as a `make` target and a CI check (`sqlc diff` / regenerate-and-git-diff) to catch drift between `.sql` queries and committed generated code. |
| `Trivy` (`aquasecurity/trivy-action`) | Container image + filesystem/dependency vuln scanning | Latest **v0.36.0** (wraps Trivy v0.70.0). Run twice in CI: `scan-type: fs` against the repo (catches vulnerable `go.sum` deps) and `image-ref:` against the built image (catches OS package + base-image CVEs). Gate on `severity: CRITICAL,HIGH` with `exit-code: 1`. |
| `gitleaks` | Secret scanning | Latest **v8.30.1**. Two layers: local pre-commit hook (`.pre-commit-config.yaml` pointing at the gitleaks repo, pinned tag) to block secrets before they're committed, plus `gitleaks/gitleaks-action@v2` in CI as the backstop (catches force-pushes, non-hook commits, contributors without hooks installed). |
| `syft` via `anchore/sbom-action` | SBOM generation | Latest **v0.24.0** (bundles syft v1.42.3). Point it at the built container image (`image:` input) or the Go module directory; output `spdx-json` (most broadly tooling-compatible) or `cyclonedx-json`. Auto-attaches as a release asset when the workflow runs on a release/tag event. |
| `svu` (`caarlos0/svu`) | Semantic version calculation from conventional commits | Go-native, single-binary, no Node/npm dependency (unlike JS `semantic-release`). Computes next tag from commit history (`feat:` → minor, `fix:` → patch, `BREAKING CHANGE:`/`!` → major). Run as `svu next`, then `git tag` + push — this is the tagging half of the pipeline. |
| GoReleaser (optional, or plain `docker/build-push-action`) | Build + publish at the computed tag | GoReleaser does **not** create tags itself — it consumes whatever tag already exists (from `svu`) and builds/publishes artifacts. For this project (single Docker image to ghcr.io, not multi-platform binaries), a plain `docker/build-push-action@v6` step tagged with the `svu`-computed version is simpler than pulling in GoReleaser's full binary-release machinery — reserve GoReleaser for if/when you also want signed multi-arch binaries, checksums, changelogs published as GitHub Releases. |
| `docker/build-push-action` + `docker/login-action` | Build & push to ghcr.io | Standard GitHub Actions building blocks; auth via the ambient `GITHUB_TOKEN` (already has `packages: write` scope when granted in workflow permissions) — no extra registry secret, matching the PROJECT.md decision. |
| `pre-commit` framework | Local hook runner | Runs `golangci-lint` and `gitleaks` locally before commit, per PROJECT.md's "pre-commit hooks: golangci-lint, gitleaks" requirement. Python-based tool but language-agnostic in what it orchestrates — fine to depend on it purely as a dev-time tool (not shipped). |

## Installation

# Core Go dependencies

# Dev-only tools (CLI installs, not go.mod deps — install via `go install` in CI or Dockerfile build stage)

# golangci-lint: install via the official install script pinned to a version, or use golangci-lint-action@v9.3.0 in CI

# gitleaks: install via package manager, Go install, or download the release binary

# Frontend

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| `caarlos0/env/v11` for config | `spf13/viper` | If the project later needs config *files* (YAML/TOML) layered with env vars, remote config stores (etcd/Consul), or CLI-flag binding in the same precedence chain. Pure env-var config for a small service doesn't need viper's weight/complexity — don't pull it in "just in case." |
| `caarlos0/env/v11` for config | `kelseyhightower/envconfig` | Functionally similar (struct-tag env parsing); `envconfig` is fine too, but is less actively maintained and has a slightly less ergonomic API for required/default handling. Either is a reasonable pick — `caarlos0/env` is the current community favorite. |
| `go-chi/httplog/v3` for logging middleware | `samber/slog-chi` | If you want built-in OpenTelemetry trace/span-ID correlation out of the box (samber/slog-chi has `WithSpanID`/`WithTraceID` options) and are already running an OTel collector. For a v1 with no distributed tracing, `httplog`'s simpler chi-native integration is the better fit. |
| Plain `docker/build-push-action` for publishing | GoReleaser | Once the project wants multi-arch binaries, GitHub Release changelogs, checksums/signing (cosign), or distributing a CLI alongside the server. Overkill for "build one Docker image, push to ghcr.io." |
| `svu` for version calculation | `go-semantic-release/semantic-release` | If you want a single tool that both computes the version *and* handles changelog generation/GitHub Release creation in one step (closer to JS `semantic-release`'s all-in-one behavior) rather than composing `svu` + a separate tag/push/build pipeline. `svu` is lighter-weight and more composable; `go-semantic-release` is more batteries-included. |
| `pgx/v5` as Postgres driver | `database/sql` + `lib/pq` | Never, for new code — `lib/pq` is in maintenance-only mode (no new features, security fixes only). Only relevant if inheriting a legacy codebase already on `lib/pq`. |
| Hand-rolled MusicBrainz/Deezer clients | A generated/wrapped OpenAPI client | Neither API publishes an OpenAPI spec suitable for clean codegen, and the surface area needed (search + a couple of lookup endpoints) is small enough that a hand-rolled client with `httptest.Server`-backed tests is simpler and more debuggable than fighting a generated client's abstractions. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|--------------|
| `github.com/go-chi/chi` (no `/v5` suffix, i.e. v1–v4) | Pre-Go-modules legacy versions; not what "chi" means in any current tutorial or the project's own PROJECT.md decision. | `github.com/go-chi/chi/v5` |
| `github.com/lib/pq` | Maintenance-mode only — the maintainers have stated no new features will be added; `pgx` has been the community's recommended driver for years and is what sqlc's docs lead with. | `github.com/jackc/pgx/v5` |
| `golangci-lint` v1 config schema (`.golangci.yml` examples with `run:`/`linters-settings:` from pre-2025 blog posts) | v2 changed the config file schema; copying v1-era config into a v2 install will fail to parse or silently ignore settings. | Use v2 config schema (`version: "2"` at the top of `.golangci.yml`), verified against the current `golangci-lint.run/docs/configuration/file/` page at implementation time. |
| Node.js `semantic-release` (the original JS tool) | Requires a Node/npm toolchain in the CI job purely for versioning in an otherwise pure-Go project — adds an entire second language runtime, `node_modules` install, and plugin ecosystem to maintain for a single-purpose task. | `svu` (Go-native, single binary) or `go-semantic-release/semantic-release` |
| `spf13/viper` for env-var-only config | Pulls in a large dependency tree (fsnotify, multiple format parsers, etc.) for a task that's just "read env vars into a struct." Known for subtle case-sensitivity/precedence surprises when mixing sources. | `caarlos0/env/v11` |
| Ignoring MusicBrainz's User-Agent requirement / using Go's default `http.Client` UA | MusicBrainz explicitly throttles/blocks "anonymous" requests (default or missing User-Agent) — this will manifest as intermittent 503s that look like flaky network issues, not an obvious auth error. | Always set a descriptive `User-Agent: <app>/<version> (<contact>)` header on every MusicBrainz request. |
| A generic "webhook client" library for Discord | Discord's webhook API is one `POST` with a JSON body; wrapping it in a dependency adds indirection for zero real benefit, and most such libraries are actually full Discord *bot* SDKs (gateway, slash commands, etc.) — far more than a webhook-only notifier needs. | Hand-rolled `internal/discord` package using stdlib `net/http` + `encoding/json`. |

## Stack Patterns by Variant

- Switch from `go-chi/httplog` to `samber/slog-chi` (or add OTel middleware alongside `httplog`)
- Because `samber/slog-chi` has built-in `WithTraceID`/`WithSpanID` config hooks that `httplog` doesn't emphasize as strongly
- Reconsider `viper`, or layer a `.env` file loader (`joho/godotenv`) in front of `caarlos0/env` for local dev only (never in production/CI)
- Because `caarlos0/env` intentionally stays env-var-only; don't bolt file-parsing onto it
- Revisit `robfig/cron` — it has no built-in distributed-lock/leader-election support
- Because running multiple instances of `robfig/cron` naively will double-poll; would need an external lock (Postgres advisory lock is the natural fit given the existing DB) before scaling beyond one instance

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `sqlc` v1.31.1 | `pgx/v5` v5.10.0 | sqlc's Postgres codegen defaults to producing `pgx/v5`-shaped code when `sql_package: "pgx/v5"` is set in `sqlc.yaml`; confirm this setting explicitly rather than relying on the default, since sqlc also supports plain `database/sql` output. |
| `golang-migrate/migrate` v4.19.1 | Postgres via `pgx` or `lib/pq` under the hood | golang-migrate's own Postgres driver historically wraps `lib/pq`-style connection strings (`postgres://...`) — this is independent of which driver your *application* code uses (pgx), since migrate runs as a separate CLI/step, not in-process. |
| `golangci-lint` v2.12.2 | `golangci-lint-action` v9.3.0 | v9.x of the action is built for v2 config schema; if you ever pin an older golangci-lint version, also check the action's version compatibility table, since v1-targeting action versions (v6 and earlier) won't understand v2 config. |
| `go-chi/chi/v5` | `go-chi/httplog/v3` | Both maintained under the `go-chi` GitHub org and designed together; httplog v3 expects to sit alongside chi's own `middleware.RequestID`/`middleware.Recoverer`. |
| Go toolchain | `sqlc`, `golangci-lint`, `chi` | All three track recent Go releases; pin your `go.mod` `go` directive to a specific minor version (e.g. `go 1.23`) and match your Docker builder-stage base image (`golang:1.23-alpine` or similar) to avoid drift between local dev, CI, and the build container. |

## Sources

- pkg.go.dev — `github.com/go-chi/chi/v5` (v5.3.1), `github.com/caarlos0/env/v11` (v11.4.1), `github.com/robfig/cron/v3` (v3.0.1), `github.com/jackc/pgx/v5` (v5.10.0) — versions verified directly, confidence MEDIUM
- GitHub Releases — `golang-migrate/migrate` (v4.19.1), `sqlc-dev/sqlc` (v1.31.1), `gitleaks/gitleaks` (v8.30.1), `golangci/golangci-lint` (v2.12.2), `golangci/golangci-lint-action` (v9.3.0), `aquasecurity/trivy-action` (v0.36.0), `anchore/sbom-action` (v0.24.0) — versions verified directly, confidence MEDIUM
- `musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting`, `wiki.musicbrainz.org/MusicBrainz_API` — rate limits, User-Agent requirement, pagination — confidence MEDIUM
- Deezer developer docs / community references (`support.deezer.com`, `deezer-python.readthedocs.io`) — rate limits, pagination, auth — confidence MEDIUM (no single canonical current official Deezer API rate-limit doc found; corroborated across multiple independent sources)
- Discord webhook ecosystem guides (multiple independent 2026-dated sources) on payload schema, embed limits, 30 req/min rate limit — confidence MEDIUM
- `golangci-lint.run/docs/configuration/file/`, `golangci-lint.run/docs/welcome/install/ci/` — v2 config schema, CI installation guidance — confidence MEDIUM
- `github.com/go-chi/httplog`, `github.com/samber/slog-chi` — package READMEs — confidence MEDIUM
- `anchore/sbom-action` README, `aquasecurity/trivy-action` README — GitHub Action usage patterns — confidence MEDIUM
- `caarlos0/svu`, `goreleaser.com/cookbooks/semantic-release`, `go-semantic-release/semantic-release` — versioning tool comparison — confidence MEDIUM

<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->

## Codebase Questions — Query the Knowledge Graph First

This repo ships a prebuilt graphify knowledge graph in `graphify-out/` (`graph.json`, `GRAPH_REPORT.md`, `graph.html`; everything but `cache/` is committed). For any question about architecture, file relationships, or where a behavior lives, query the graph before grepping or reading files one by one — one query returns targeted context instead of spending tokens on exploratory reads.

- Ask a question: `/graphify query "<question>"` (`--dfs` to trace a single path, `--budget <n>` to cap answer size)
- Connect two concepts: `/graphify path "<A>" "<B>"`
- Explain one node: `/graphify explain "<name>"`

Use the graph to narrow the search, then read the specific files it points at before editing anything. If it looks stale relative to recent commits, refresh with `/graphify . --update`.

A root `.graphifyignore` keeps completed and ephemeral planning docs out of the graph, excluding the closed `.planning/milestones/v1.0-phases/` directory plus `.planning/quick/`, `.planning/debug/`, `.planning/todos/`, and `.planning/research/`. These are archival churn, and indexing them buries current architecture and decisions under stale planning nodes. `PROJECT.md`, `ROADMAP.md`, `STATE.md`, `RETROSPECTIVE.md`, `MILESTONES.md`, the codebase map directory, the active phases directory, and the current milestone's phase directory all stay indexed. When a milestone completes, or quick-task and debug dirs pile up, add the newly-closed milestone's phase directory to `.graphifyignore` before the next `/graphify . --update` rebuild.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context layout — one `CONTEXT.md` + `docs/adr/` at the repo root (not yet created; created lazily by `/domain-modeling`). See `docs/agents/domain.md`.

*Hand-maintained section — it sits outside the GSD marker blocks above so `generate-claude-md` never overwrites it. Keep it last.*
