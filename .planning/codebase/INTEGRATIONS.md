# External Integrations

**Analysis Date:** 2026-08-12

## APIs & External Services

**Music Data:**
- **MusicBrainz** - Artist search and release-group/recording metadata for new-release detection
  - SDK/Client: Hand-rolled `internal/musicbrainz` using stdlib `net/http`
  - Auth: User-Agent header (no API key; rate-limit keyed off User-Agent)
  - Rate limit: `MUSICBRAINZ_RATE_LIMIT_PER_SEC` (default 1.0 req/sec)
  - Endpoints:
    - `GET /ws/2/artist?query=...&limit=...` - Artist search
    - `GET /ws/2/release-group?arid=...&limit=...` - Release-groups by artist MBID
    - `GET /ws/2/recording?arid=...&limit=...` - Recordings by artist credit (Phase 4+, guest-feature detection)
    - `GET /ws/2/release/...?inc=recording-level-rels,...` - Release detail fetch for deluxe/tracklist detection
  - Timeout: 10 seconds per request
  - Base URL: `https://musicbrainz.org/ws/2` (hardcoded, never user-configured)
  - User-Agent: `drop-tracker/0.1.0 (+https://github.com/danielrpof/drop-tracker)` (required; missing UA throttles to anonymous pool)

- **Deezer** - Album search and artist album listing for cross-reference and release confirmation
  - SDK/Client: Hand-rolled `internal/deezer` using stdlib `net/http`
  - Auth: None (public search endpoints require no authentication)
  - Rate limit: `DEEZER_RATE_LIMIT_PER_5S` (default 50 per 5 seconds; token-bucket configured as ~10 req/sec burst)
  - Endpoints:
    - `GET /search?q=...&limit=...` - Artist/album search
    - `GET /artist/{id}/albums?limit=...` - Albums by artist ID
  - Timeout: 10 seconds per request
  - Base URL: `https://api.deezer.com` (hardcoded, never user-configured)
  - Note: Returns HTTP 200 with in-body error envelope on quota breach (not HTTP 429)

**Notifications:**
- **Discord Webhook** - Asynchronous event notifications (new releases, guest features, deluxe changes)
  - SDK/Client: Hand-rolled `internal/discord` using stdlib `net/http`
  - Auth: Webhook token embedded in `DISCORD_WEBHOOK_URL`
  - Rate limit: Discord 30 req/min; enforced by `internal/notifier` (not the client)
  - Endpoint: Single execute-webhook route (POST to `DISCORD_WEBHOOK_URL`)
  - Timeout: 10 seconds per request
  - Payload: JSON with single embed per message (title, description, fields, color, timestamp, thumbnail)
  - Response: HTTP 204 No Content on success
  - Retry: Honors `Retry-After` header on HTTP 429 (max wait 30s to prevent indefinite hangs)
  - Security: Webhook URL's path segment IS the secret token; never logged or echoed in error messages

## Data Storage

**Databases:**
- **PostgreSQL 16** (external service, not bundled)
  - Connection: `DATABASE_URL` env var (DSN format: `postgres://user:password@host:5432/dbname?sslmode=disable`)
  - Client: `github.com/jackc/pgx/v5` (pure Go driver)
  - Schema: Managed by `github.com/golang-migrate/migrate/v4` via `.sql` files in `internal/db/migrations/`
  - Queries: Type-safe via `sqlc` codegen in `internal/db/sqlc/` (generated from `queries/` `.sql` files)
  - Timeouts:
    - Connect timeout: 5s (if not specified in DSN)
    - Ping health timeout: 2s (acquires connection, checks liveness)
    - Max idle time: 1m (prevents stale sockets from NAT/intermediary droppage after 4-5 min typical)
  - Pool: Single `pgxpool.Pool` instance per process, shared across all packages
  - Local dev: `docker-compose.yml` runs Postgres 16 on port 5433 (not 5432, see schema comment)
  - Tables: `artists` (watchlist), `events` (detected releases/features), `release_groups_seen`, `recordings_seen` (deduplication)

**File Storage:**
- None (not used; state lives in Postgres, SPA is embedded into binary)

**Caching:**
- None (explicit; rely on Postgres connection pool and in-process rate limiters)

## Authentication & Identity

**Auth Provider:**
- None (no user authentication; this is a personal tracker)

**Internal Security:**
- Environment-variable-only config (no config files)
- All secrets passed via env vars (DATABASE_URL password, DISCORD_WEBHOOK_URL token)
- Pre-commit gitleaks hook blocks committed secrets before they reach git
- CI gitleaks job as backstop against force-pushes or non-hook contributors
- gosec linting enabled to catch DSN/URL logging/redaction issues
- Discord webhook URL redacted in error messages to avoid logging the token

## Monitoring & Observability

**Error Tracking:**
- None (external service not integrated)
- Errors logged to stdout as structured JSON via `log/slog` with `JSONHandler`
- Production logs include: timestamp, level, message, service name, request-id (from chi `middleware.RequestID`), error details

**Logs:**
- Structured JSON logging (default) or text format (via `LOG_FORMAT=text` env var)
- Log level: Configurable via `LOG_LEVEL` env var (debug, info [default], warn, error)
- Output: Stdout (picked up by Docker container logs or syslog redirect)
- HTTP request middleware: `go-chi/httplog/v3` logs every request with latency, status, method, path, request-id
- Poll cycle logging: Logged by `internal/poller` and `internal/notifier` with service names and cycle metadata

**Metrics:**
- None (external integration not present; rate limiter state is internal)

## CI/CD & Deployment

**Hosting:**
- GitHub Container Registry (ghcr.io)
- Image: `ghcr.io/danielrpof/drop-tracker:${VERSION}` (pushed on every successful build on main)
- Registry auth: GITHUB_TOKEN (already has `packages: write` scope in GitHub Actions)

**CI Pipeline:**
- GitHub Actions (`.github/workflows/full-pipeline.yml`)

**Pipeline Jobs:**
1. `vet` - `go vet ./...` (fast static checks)
2. `lint` - `golangci-lint` v2.12.2 (includes gosec, errcheck, staticcheck, unused)
3. `test` - Integration tests against real Postgres (runs migrations fresh each time)
4. `gitleaks` - Secret scanning on commit history
5. `trivy-fs` - Filesystem scan for vulnerable dependencies in `go.sum`
6. `pr-title` - Semantic commit prefix validation (feat/fix/BREAKING CHANGE required)
7. `build-scan` - Docker build + Trivy image scan (CRITICAL/HIGH severity gate)
8. `release` - (Main branch only) Tag with `svu next`, push image, generate SBOM

**Versioning:**
- Semantic versioning via `svu` (reads commits since last tag, computes next version)
- Commit prefix required: `feat:` → minor, `fix:` → patch, `BREAKING CHANGE:` or `!` → major
- Version calculated in `release` job, committed as git tag, pushed to GitHub
- Build doesn't rebuild; scanned image from `build-scan` is loaded, tagged, and pushed byte-for-byte

**Image Build:**
- Three-stage Dockerfile:
  1. Node build stage: `node:26-alpine3.24` - Builds React Router SPA to `web/build/client/`
  2. Go build stage: `golang:1.26.5-alpine3.24` - Compiles Go binary, embeds SPA via `go:embed`
  3. Runtime stage: `alpine:3.24` - Minimal image with `ca-certificates`, non-root user (UID 10001), health check
- Container entrypoint: `/usr/local/bin/server` (the Go binary)
- Health check: `wget http://127.0.0.1:${HTTP_PORT:-8080}/health` every 10s (503 = database unreachable, 200 = healthy)
- Exposed port: 8080

**Deployment:**
- Manual (push to main → CI builds/scans/tags/publishes to ghcr.io)
- Orchestrator must fetch image from ghcr.io and run with DATABASE_URL, DISCORD_WEBHOOK_URL, HTTP_PORT env vars

## Environment Configuration

**Required env vars:**
- `DATABASE_URL` - Postgres connection DSN (must be set, no default)
- `DISCORD_WEBHOOK_URL` (optional, but empty = notifier disabled; recommended for production)

**Optional env vars with defaults:**
- `HTTP_PORT` - API listen port (default 8080)
- `LOG_LEVEL` - Verbosity (default info; accepts debug, warn, error)
- `LOG_FORMAT` - Output format (default json; accepts text)
- `POLL_INTERVAL` - Cron schedule for MusicBrainz/Deezer polling (default 15m; e.g., "0 */6 * * *" for 6-hourly)
- `MUSICBRAINZ_USER_AGENT` - Identifies this app to MusicBrainz (default `drop-tracker/0.1.0 (+https://github.com/danielrpof/drop-tracker)`)
- `MUSICBRAINZ_RATE_LIMIT_PER_SEC` - Rate limit for MusicBrainz requests (default 1.0)
- `DEEZER_RATE_LIMIT_PER_5S` - Rate limit for Deezer requests (default 50)

**Secrets location:**
- Environment variables only (no .env files in production)
- Local dev: `.env` file (not committed; see `.env.example` for template)
- CI/CD: GitHub Secrets (none currently required; uses GITHUB_TOKEN which is ambient)
- Secrets never logged: DSN passwords, Discord webhook token redacted in error messages

## Webhooks & Callbacks

**Incoming:**
- None (drop-tracker is not a webhook receiver)

**Outgoing:**
- **Discord Webhook** - One-way notifications sent by `internal/notifier.Send()` when events are detected
  - Triggered by: `internal/poller` polling cycle detects new release/feature/deluxe change
  - Payload: JSON with single embed (artist name, release title, release type, detection time, link to MusicBrainz)
  - Frequency: Depends on `POLL_INTERVAL` (default 15m) and event detection rate
  - Guarantee: Fire-and-forget; no acknowledgment or persistence if webhook endpoint is down

---

*Integration audit: 2026-08-12*
