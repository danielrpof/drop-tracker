---
phase: 01-foundation-data-layer-config-health
plan: 01
subsystem: foundation-boot-path
tags: [config, logging, postgres, migrations, http, health-check, tdd]
dependency-graph:
  requires: []
  provides:
    - config.Load
    - logging.New
    - logging.NewWithWriter
    - db.NewPool
    - db.RunMigrations
    - httpserver.New
    - httpserver.Pinger
    - testutil.RequirePostgresDSN
    - testutil.NewTestPool
  affects:
    - cmd/server/main.go
tech-stack:
  added:
    - github.com/go-chi/chi/v5@v5.3.1
    - github.com/go-chi/httplog/v3@v3.4.0
    - github.com/jackc/pgx/v5@v5.10.0
    - github.com/caarlos0/env/v11@v11.4.1
    - github.com/golang-migrate/migrate/v4@v4.19.1
  patterns:
    - "Fail-fast env config via caarlos0/env/v11 notEmpty tags"
    - "Migrate-on-boot with bounded exponential backoff, ErrNoChange as success"
    - "chi RequestID -> echoRequestID -> httplog.RequestLogger -> Recoverer middleware order"
    - "Environment-DSN Postgres test fixture (not testcontainers-go)"
key-files:
  created:
    - go.mod
    - go.sum
    - .env.example
    - docker-compose.yml
    - cmd/server/main.go
    - internal/config/config.go
    - internal/logging/logging.go
    - internal/db/pool.go
    - internal/db/migrate.go
    - internal/db/migrations/000001_init.up.sql
    - internal/db/migrations/000001_init.down.sql
    - internal/httpserver/server.go
    - internal/httpserver/health.go
    - internal/testutil/postgres.go
    - internal/httpserver/boot_e2e_test.go
  modified:
    - .gitignore
decisions:
  - "Used golang-migrate's database/pgx/v5 driver (database/sql + pgx/v5/stdlib + pgxmigrate.WithInstance) instead of its generic database/postgres driver, because the latter imports github.com/lib/pq internally — forbidden by CLAUDE.md. This keeps lib/pq out of the module's dependency graph entirely while still accepting the same postgres:// DSN scheme used everywhere else."
  - "Added an echoRequestID middleware and a request_id LogExtraAttrs field on httplog.RequestLogger, because go-chi/httplog/v3's built-in Schema has no request-ID field of its own — needed to satisfy OPS-02's 'every request carries a correlation ID in its structured log line' and SKELETON.md's X-Request-Id response-header decision."
metrics:
  duration: "~90 min"
  completed: 2026-08-05
actuals:
  tokens: 8162
  tasks: 2
  commits: 3
status: complete
---

# Phase 1 Plan 1: Wire env config to migrated Postgres to GET /health Summary

Stood up the whole walking skeleton in one pass: a Go module that reads env-only config with
fail-fast validation, connects to Postgres via pgx/v5, applies an embedded no-op migration
with bounded retry/backoff (treating `migrate.ErrNoChange` as success), and serves
`GET /health` over a chi router with request-ID-correlated JSON logging — proven end-to-end
against a real docker-compose Postgres, then made repeatable via a shared `internal/testutil`
fixture and a `httptest`-driven boot integration test.

## What Was Built

**Task 1 (tracer):** `github.com/danielrpof/drop-tracker` module with the five pinned Phase 1
dependencies; `internal/config` (`Config`/`Load`, `notEmpty` DSN tag); `internal/logging`
(`New`/`NewWithWriter`, JSON/text `slog` handlers); `internal/db` (`NewPool` with eager
timeout-bounded `Ping`, `RunMigrations` with a 6-attempt/500ms→8s exponential backoff loop and
an embedded no-op `000001_init` migration); `internal/httpserver` (`Pinger` seam, chi router
with `RequestID → echoRequestID → httplog.RequestLogger → Recoverer`, `GET /health`
implementing the exact `{"status","db"}` / 200-or-503 contract); `cmd/server/main.go` (thin
`main()` delegating to `run() error`, sequenced `config.Load → logging.New → RunMigrations →
NewPool → httpserver.New → ListenAndServe`); `docker-compose.yml` (Postgres 16 with a
`pg_isready` healthcheck); `.env.example` documenting the Phase 1 config surface plus stubbed
future-phase fields.

**Task 2 (TDD):** `internal/testutil/postgres.go` (`RequirePostgresDSN`, `NewTestPool`) as the
one shared DB fixture every later DB-backed test reuses, and
`internal/httpserver/boot_e2e_test.go` (package `httpserver_test`) with
`TestBootToHealth_EndToEnd` (drives the real boot chain through `httptest.NewServer`, asserts
200 + `Content-Type: application/json` + decoded `{ok, up}` body) and
`TestBootToHealth_MigrationsAreIdempotent` (calls `RunMigrations` twice, both must return nil).

## Verification Performed

- `docker compose up -d --wait postgres` → healthy; `go build ./... && go vet ./... && go mod
  verify` → all exit 0; `go list -m all` confirms all five deps pinned exactly.
- Live boot: `GET /health` → `200 {"status":"ok","db":"up"}`, `X-Request-Id` response header
  present, matching `request_id` field on the JSON log line.
- `docker compose stop postgres` with the server already listening → `GET /health` →
  `503 {"status":"degraded","db":"down"}`; the underlying ping error is attached to the
  request's log entry only, never the response body.
- `schema_migrations`: `version=1, dirty=f` after first boot.
- Second boot against the already-migrated database still reaches listening
  (`migrate.ErrNoChange` handled as success).
- `DATABASE_URL=''` and unset `DATABASE_URL` both exit 1, stderr names `DATABASE_URL`, before
  listening.
- Captured boot log grepped for `drop_tracker:drop_tracker` (the DSN's user:password pair):
  zero matches.
- `TEST_DATABASE_URL=... go test ./... -count=1` green; `go test ./... -short -count=1` green
  with both DB-backed tests printing `--- SKIP` naming `TEST_DATABASE_URL`.

## TDD Gate Compliance

Task 2 followed the RED → GREEN cycle:
- RED: commit `369e8b6` added `boot_e2e_test.go` referencing `testutil.RequirePostgresDSN`
  before `internal/testutil` existed — `go test ./internal/httpserver/... -run
  TestBootToHealth` failed with a setup/compile error (package not found).
- GREEN: commit `dcc879a` implemented `internal/testutil/postgres.go`; both tests then passed
  against the compose Postgres.
- REFACTOR: not needed — no cleanup required after GREEN.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - CLAUDE.md precedence] Avoided golang-migrate's lib/pq-backed postgres driver**
- **Found during:** Task 1, writing `internal/db/migrate.go`.
- **Issue:** RESEARCH.md's Pattern 3 code example calls
  `migrate.NewWithSourceInstance("iofs", src, dsn)`, which resolves the DSN's `postgres://`
  scheme to golang-migrate's generic `database/postgres` driver — that driver imports
  `github.com/lib/pq` internally (confirmed by reading its source), directly violating
  CLAUDE.md's "What NOT to Use: github.com/lib/pq" rule.
- **Fix:** Used golang-migrate's dedicated `database/pgx/v5` driver instead: open a
  `database/sql.DB` via `sql.Open("pgx", dsn)` (registered by `pgx/v5/stdlib`), then
  `pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})`, then
  `migrate.NewWithInstance("iofs", src, "pgx5", dbDriver)`. This keeps `lib/pq` out of the
  build graph entirely (`go list -deps ./... | grep lib/pq` returns nothing) while accepting
  the identical `postgres://` DSN scheme used by `pgxpool`, `docker-compose.yml`, and
  `.env.example`.
- **Files modified:** `internal/db/migrate.go`.
- **Commit:** `02d7bef`.
- **User-approved:** yes — surfaced at a tracer checkpoint and explicitly approved before
  Task 2 began.

**2. [Rule 2 - missing critical functionality] Added explicit request-ID log correlation**
- **Found during:** Task 1, writing `internal/httpserver/server.go`.
- **Issue:** `go-chi/httplog/v3`'s `Schema` struct (verified via `go doc`) has no built-in
  request-ID field — RESEARCH.md's Pattern 2 code example implies the ID is logged
  automatically, but it is not. Without an explicit fix, OPS-02's "every HTTP request emits a
  structured JSON log line carrying a correlation ID" and SKELETON.md's "echoed to the client
  as the X-Request-Id response header" decision would both go unsatisfied.
- **Fix:** Added an `echoRequestID` middleware (writes `middleware.RequestIDHeader` to the
  response) and a `LogExtraAttrs` function on `httplog.Options` that reads
  `middleware.GetReqID(req.Context())` and attaches it as a `request_id` slog attribute.
  Verified live: both the `X-Request-Id` response header and the `request_id` JSON log field
  are present on every request.
- **Files modified:** `internal/httpserver/server.go`.
- **Commit:** `02d7bef`.

## Known Stubs

None — every file this plan introduces implements real, wired, tested behavior for its stated
scope. `sqlc`/`queries/health.sql` (D-02) and the full `.env.example` config surface (D-06/D-07)
are explicitly deferred to plans 03/04/05 per the phase's plan split, not stubbed here.

## Threat Flags

None beyond what the plan's own `<threat_model>` already covers — no new network endpoints,
auth paths, or schema changes were introduced outside that register.

## Self-Check: PASSED

- FOUND: go.mod, go.sum, .env.example, docker-compose.yml, cmd/server/main.go,
  internal/config/config.go, internal/logging/logging.go, internal/db/pool.go,
  internal/db/migrate.go, internal/db/migrations/000001_init.up.sql,
  internal/db/migrations/000001_init.down.sql, internal/httpserver/server.go,
  internal/httpserver/health.go, internal/testutil/postgres.go,
  internal/httpserver/boot_e2e_test.go
- FOUND: commit 02d7bef, commit 369e8b6, commit dcc879a
