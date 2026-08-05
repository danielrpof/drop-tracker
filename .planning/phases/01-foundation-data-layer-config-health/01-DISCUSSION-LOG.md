# Phase 1: Foundation — Data Layer, Config & Health - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-04
**Phase:** 01-foundation-data-layer-config-health
**Areas discussed:** Schema scope, Health check depth, Config & startup failure behavior, Migration execution model

---

## Schema scope

| Option | Description | Selected |
|--------|-------------|----------|
| Bookkeeping only | Just what golang-migrate needs (schema_migrations table), no domain tables yet | ✓ |
| Minimal placeholder table | One trivial domain table so /health and sqlc have something real to query | |
| Scaffold ahead to watchlist schema | Design and migrate the actual artists/watchlist tables now | |

**User's choice:** Bookkeeping only
**Notes:** Phase 2 (Watchlist Core) owns the domain schema.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, wire it up now | Configure sqlc.yaml, add a trivial health-check query, add CI drift-check now | ✓ |
| Defer to Phase 2 | Use pgx directly for the health check; introduce sqlc when Phase 2 needs real queries | |

**User's choice:** Yes, wire it up now
**Notes:** Avoids retrofitting the sqlc CI step later.

---

## Health check depth

| Option | Description | Selected |
|--------|-------------|----------|
| DB ping only | Simple pgx Ping, returns {"status", "db"} | ✓ |
| DB ping + migration version | Also reports currently applied migration version | |
| Liveness vs readiness split | Two endpoints, k8s-idiomatic | |

**User's choice:** DB ping only
**Notes:** Project targets VPS SSH deploy, not k8s — readiness/liveness split not warranted.

| Option | Description | Selected |
|--------|-------------|----------|
| 503 on DB down | 200 healthy, 503 when DB ping fails | ✓ |
| Always 200, status in body | Always 200 OK, status field in JSON body | |

**User's choice:** 503 on DB down
**Notes:** Lets monitors gate on status code alone.

---

## Config & startup failure behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Fail fast, exit non-zero | Validate all config up front, exit(1) on any missing/invalid value | ✓ |
| Fail fast, but only for critical vars | Hard-fail only on essential vars, others default with a warning | |

**User's choice:** Fail fast, exit non-zero

| Option | Description | Selected |
|--------|-------------|----------|
| Phase 1 settings only | .env.example covers only DB_DSN, PORT, LOG_LEVEL, LOG_FORMAT | |
| Stub future settings too | Also documents Discord webhook, poll intervals, etc. now | ✓ |

**User's choice:** Stub future settings too

| Option | Description | Selected |
|--------|-------------|----------|
| Struct fields with defaults | Future settings become real Config struct fields with defaults now | ✓ |
| .env.example comments only | Document as commented placeholders, no struct fields yet | |

**User's choice:** Struct fields with defaults

| Option | Description | Selected |
|--------|-------------|----------|
| info + json | Production-sane defaults; dev overrides text format explicitly | ✓ |
| debug + text | Dev-ergonomic defaults; prod overrides explicitly | |

**User's choice:** info + json

---

## Migration execution model

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-run on startup | Service runs migrate Up() before serving traffic | ✓ |
| Manual/separate step | Migrations run via separate CLI/make target | |

**User's choice:** Auto-run on startup

| Option | Description | Selected |
|--------|-------------|----------|
| Exit non-zero immediately | Log error, exit(1), no retry | |
| Retry with backoff, then exit | Retry a few times with backoff, then exit(1) if still failing | ✓ |

**User's choice:** Retry with backoff, then exit
**Notes:** Handles Postgres container still starting up under docker-compose.

---

## Claude's Discretion

- Exact project layout (`cmd/`, `internal/` package structure)
- Specific retry/backoff parameters (attempt count, delay curve) for migration/DB connect failures
- Precise request-ID header name and structured log field naming

## Deferred Ideas

None — discussion stayed within phase scope.
