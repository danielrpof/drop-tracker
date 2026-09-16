# Technology Stack

**Analysis Date:** 2026-08-12

## Languages

**Primary:**
- Go 1.26 - Core server, API, scheduler, and notifier (single binary)
- TypeScript 5.9.3 - React UI type-safe frontend

**Secondary:**
- JavaScript - React Router framework
- SQL - Postgres schema and sqlc-generated queries

## Runtime

**Environment:**
- Go 1.26 for server-side execution
- Node.js 26 (Alpine 3.24) for frontend build pipeline only (not in production)

**Package Manager:**
- Go modules (`go.mod`/`go.sum`) - Pinned Go dependencies
- pnpm 11.8.0 - Frontend dependencies with strict lockfile (`pnpm-lock.yaml`)

**Lockfiles:**
- `go.sum` - Go module lock (present)
- `pnpm-lock.yaml` - Frontend dependency lock (lockfileVersion: 9.0)

## Frameworks

**Core Backend:**
- `github.com/go-chi/chi/v5` v5.3.1 - HTTP router, stdlib-idiomatic with zero external request-path dependencies
- `github.com/robfig/cron/v3` v3.0.1 - Cron-based background task scheduler for polling
- `github.com/golang-migrate/migrate/v4` v4.19.1 - Schema migrations from `.sql` files

**Database Access:**
- `github.com/sqlc-dev/sqlc` v1.31.1 (CLI) - Type-safe SQL code generation from hand-written `.sql` queries
- `github.com/jackc/pgx/v5` v5.10.0 - Pure Go Postgres driver

**Frontend:**
- React 19.2.6 - UI library
- React Router 7.18.2 - File-based routing (SSR disabled for go:embed)
- Vite 8 (via `@tailwindcss/vite`) - Build tool and dev server
- Tailwind CSS 4 - Utility-first CSS framework
- TypeScript 5.9.3 - Type checking

**UI Components:**
- `@base-ui/react` 1.7.0 - Unstyled, accessible component primitives
- `shadcn` 4.16.2 - Shadcn/ui component collection (built on @base-ui)
- `lucide-react` 1.31.0 - Icon library
- `next-themes` 0.4.6 - Theme toggle (light/dark mode)
- `sonner` 2.0.8 - Toast notifications
- `class-variance-authority` 0.7.1 - CSS class composition utility
- `clsx` 2.1.1 - Conditional class name merging
- `tailwind-merge` 3.6.0 - Tailwind CSS class name merger
- `tw-animate-css` 1.4.0 - CSS animation utilities for Tailwind

## Key Dependencies

**Critical Backend:**
- `golang.org/x/time/rate` v0.15.0 - Token-bucket rate limiting for external APIs (MusicBrainz, Deezer)
- `github.com/caarlos0/env/v11` v11.4.1 - Environment variable configuration parser (replaces viper/envconfig for env-only config)
- `log/slog` (stdlib) - Structured logging (JSON/text handlers)
- `github.com/go-chi/httplog/v3` v3.4.0 - HTTP request logging middleware with chi integration

**Database Access:**
- `github.com/jackc/pgerrcode` v0.0.0-20220416144525-469b46aa5efa - Postgres error code constants
- `github.com/jackc/pgpassfile` v1.0.0 (indirect) - Postgres `.pgpass` file support
- `github.com/jackc/pgservicefile` v0.0.0-20240606120523-5a60cdf6a761 (indirect) - Postgres `.pg_service.conf` support
- `github.com/jackc/puddle/v2` v2.2.2 (indirect) - Connection pool resource management

**Frontend:**
- `@fontsource-variable/inter` 5.3.0 - Inter variable font (local bundled)
- `isbot` 5 - Bot detection for analytics/telemetry
- `@react-router/node` 7.18.2 - Node.js server utilities for React Router
- `@react-router/dev` 7.18.2 - Development tools and build configuration

## Configuration

**Environment:**
- Config sourced from environment variables only (no config files) via `caarlos0/env/v11`
- Required vars: `DATABASE_URL`, `DISCORD_WEBHOOK_URL` (optional)
- Optional vars with defaults: `HTTP_PORT` (8080), `LOG_LEVEL` (info), `LOG_FORMAT` (json), `POLL_INTERVAL` (15m), `MUSICBRAINZ_USER_AGENT`, `MUSICBRAINZ_RATE_LIMIT_PER_SEC` (1.0), `DEEZER_RATE_LIMIT_PER_5S` (50)

**Build:**
- `sqlc.yaml` - sqlc codegen config (PostgreSQL engine, pgx/v5 output)
- `.golangci.yml` - Linting config (v2 schema, golangci-lint v2.12.2)
- `Dockerfile` - Three-stage build (Node → Go → Alpine runtime)
- `docker-compose.yml` - Local dev: Postgres 16 + Go app (port 5433, not 5432)
- `.pre-commit-config.yaml` - Pre-commit hooks: gitleaks v8.30.1, golangci-lint v2.12.2

**Development:**
- `Makefile` - Build targets: `build`, `run`, `test`, `test-integration`, `sqlc`, `sqlc-check`, `web`, `db-up`/`db-down`, `hooks`
- `web/package.json` - Frontend scripts: `dev`, `build`, `typecheck`, `format`
- `web/vite.config.ts` - Vite config with dev proxy to Go API (localhost:8080)
- `web/react-router.config.ts` - React Router with SSR disabled

## Platform Requirements

**Development:**
- Go 1.26+ (locked in `go.mod`)
- Node.js 26+ (for frontend build only; see Dockerfile)
- pnpm 11.8.0 (explicit install in Dockerfile for modern `lockfileVersion: 9.0` support)
- Docker (for Postgres in `docker-compose up`)
- Python 3.x (for pre-commit framework, installed via `pip`)

**Production:**
- Alpine Linux 3.24 (runtime image base)
- Postgres 16 (external, assumed managed separately or in a sidecar)
- No Node.js runtime (SPA built and embedded into Go binary at compile time)

**CI/CD:**
- GitHub Actions (`.github/workflows/full-pipeline.yml`)
- Docker buildx (for multi-stage builds in CI)
- Go 1.26 toolchain
- golangci-lint v2.12.2 (via action v9.3.0)
- Trivy v0.70.0 (via action v0.36.0) for image + filesystem scanning
- gitleaks v8.30.1 (via pre-commit hook and GitHub action)
- svu v3.4.1 (semantic version calculator for releases)
- syft v1.42.3 (via anchore/sbom-action v0.24.0) for SBOM generation

## Version Compatibility

| Component | Version | Notes |
|-----------|---------|-------|
| Go | 1.26 | Locked in `go.mod` directive |
| chi/v5 | 5.3.1 | Latest v5 series |
| pgx/v5 | 5.10.0 | Postgres driver; sqlc generates pgx/v5 code |
| sqlc | v1.31.1 | Pinned in Makefile; regeneration checked in CI via `sqlc diff` |
| golang-migrate | v4.19.1 | Handles Postgres via pgx driver |
| robfig/cron | v3.0.1 | Stable, feature-complete (pinned since Jan 2020) |
| Node | 26 | Dockerfile builder stage base image (`node:26-alpine3.24@sha256:...`) |
| React | 19.2.6 | Latest React 19 |
| React Router | 7.18.2 | Latest v7, pinned in package.json |
| TypeScript | 5.9.3 | Frontend type checking |
| Tailwind CSS | 4 | Latest v4 |
| golangci-lint | v2.12.2 | Action v9.3.0 (v2 config schema) |
| Trivy | v0.70.0 | Via action v0.36.0 |

---

*Stack analysis: 2026-08-12*
