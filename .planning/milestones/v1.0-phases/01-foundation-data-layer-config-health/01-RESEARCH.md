# Phase 1: Foundation — Data Layer, Config & Health - Research

**Researched:** 2026-08-04
**Domain:** Go service bootstrap — env config, Postgres migrations/sqlc wiring, structured logging with request-ID correlation, health checks
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** The initial migration creates only what golang-migrate itself needs (`schema_migrations` bookkeeping) — no domain tables (artists/watchlist) in Phase 1. Phase 2 owns the watchlist schema.
- **D-02:** sqlc is wired up in Phase 1 anyway — `sqlc.yaml` configured, `sql_package: "pgx/v5"`, a trivial query added (e.g. backing the health check) so `sqlc generate` produces real output, and the CI drift-check (`sqlc generate` + `git diff`) is added now rather than retrofitted in Phase 2.
- **D-03:** `/health` does a DB ping only (pgx `Ping`) — no migration-version reporting, no liveness/readiness split. Response body: `{"status": "ok"|"degraded", "db": "up"|"down"}`.
- **D-04:** Returns `503 Service Unavailable` when the DB ping fails, `200 OK` when healthy — lets uptime monitors/load balancers gate on status code alone, not just body content.
- **D-05:** Config uses `caarlos0/env/v11` with `required` tags on critical settings (DB DSN). Missing/invalid required config → log a clear error listing what's missing → `exit(1)` immediately. Never boot in a half-configured state.
- **D-06:** `.env.example` and the `Config` struct stub settings for *future* phases too (Discord webhook URL, poll intervals, MusicBrainz/Deezer settings), not just Phase 1's own needs — Reversibility: reversible — adding/removing struct fields later is a cheap, local change.
- **D-07:** Stubbed future-phase settings are real `Config` struct fields with sane defaults (optional, not `required`) — the struct is the single source of truth from day one; later phases just start reading the field instead of adding it fresh.
- **D-08:** Defaults when unset: `LOG_LEVEL=info`, `LOG_FORMAT=json` (production-sane out of the box). Local dev overrides `LOG_FORMAT=text` explicitly via `.env`.
- **D-09:** Migrations auto-run on service startup — the service invokes golang-migrate's `Up()` against the configured DSN before it starts serving traffic. No separate manual `make migrate` step required for normal operation.
- **D-10:** If the DB is unreachable or a migration fails at startup, the service retries with backoff (handles the common case of the Postgres container still starting under docker-compose), then `exit(1)` if still failing after retries — consistent with the fail-fast philosophy in D-05.

### Claude's Discretion
- Exact project layout (`cmd/`, `internal/` package structure), the specific retry/backoff parameters (attempt count, delay curve) for D-10, and the precise request-ID header/log field naming for structured logging are left to planning/implementation — these are architecture details, not decisions the user needed to make.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|--------------------|
| OPS-01 | System exposes a `/health` endpoint that reports service and database connectivity status | Pattern 4 (health handler), pgxpool `Ping` behavior, D-03/D-04 response contract, Validation Architecture test map |
| OPS-02 | System emits structured (JSON) logs with request-ID correlation for HTTP requests and poll cycles | Pattern 2 (httplog + `middleware.RequestID`/`NextRequestID`), Open Question 1 (poll-cycle scoping nuance) |
| OPS-03 | All secrets and configuration are supplied via environment variables only; none are committed to the repository | Pattern 1 (`caarlos0/env/v11` fail-fast config), `.env.example` shape (Code Examples), Security Domain V14 |
</phase_requirements>

## Summary

Phase 1 is pure infrastructure plumbing with no domain logic: a Go binary that (1) parses and validates its own config from environment variables and fails fast if misconfigured, (2) connects to Postgres, runs golang-migrate migrations on boot with retry/backoff, and has `sqlc` wired end-to-end via one trivial query, (3) emits JSON `log/slog` lines correlated by a request ID for every HTTP request, and (4) exposes `/health` returning `200`/`503` based on a live DB ping. Every library involved (`chi`, `pgx/v5`, `sqlc`, `golang-migrate`, `caarlos0/env/v11`, `go-chi/httplog/v3`) is already locked in `.claude/CLAUDE.md` — this research confirms exact current versions via the authoritative Go module proxy and fills in the concrete wiring/parameter gaps the CONTEXT.md left to Claude's discretion (D-10 retry params, project layout, request-ID field naming).

The single biggest structural risk for planning is `cmd/` layout: generic Go tutorials (and this session's own web search) default to a `cmd/api/` + `cmd/worker/` split for "API + scheduler" services — that is **wrong for this project**. PROJECT.md locks a single-binary, single-process architecture (API, scheduler, notifier all in one `main.go`), so Phase 1 must establish exactly one `cmd/` entrypoint. Getting this wrong in Phase 1 would require a disruptive restructure in Phase 3 when the scheduler is added.

The second structural nuance: OPS-02 requires poll-cycle log correlation, but no poll cycle exists until Phase 3 (robfig/cron isn't wired until then). Phase 1 can only prove the *pattern* — chi's `middleware.RequestID`/`middleware.NextRequestID()` on the HTTP side — and should document/reuse that same ID-generation mechanism for cron cycles later, rather than inventing a second correlation-ID scheme in Phase 3.

**Primary recommendation:** Single `cmd/server/main.go` entrypoint; `internal/config`, `internal/db` (pool + migrate + generated `sqlc` output), `internal/httpserver` (chi router + health handler), `internal/logging` (slog + httplog wiring) as the internal package split; migrations embedded via `//go:embed` + `iofs`, applied via `migrate.Up()` in a small hand-rolled exponential-backoff retry loop (no retry library needed) before the HTTP server starts listening.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Config load & fail-fast validation | API/Backend | — | Single-process Go binary; config is an in-process concern, no separate config service |
| Structured JSON logging + request-ID correlation | API/Backend | — | Cross-cutting middleware inside the same process that serves HTTP and (later) runs the cron scheduler |
| Postgres connection pool (pgx/v5) | API/Backend | Database/Storage | Backend owns the client/pool object; Postgres itself is the storage tier being connected to |
| Schema migrations (golang-migrate) | Database/Storage | API/Backend | Migrations define the schema (data tier's concern) but are executed by the backend process at boot, not a separate migration service |
| Generated queries (sqlc) | API/Backend | Database/Storage | Data-access layer lives in the backend process; sqlc just codegens type-safe SQL calls against the storage tier's schema |
| `/health` endpoint | API/Backend | — | An HTTP handler exposed by the same chi router as everything else — no separate health-check sidecar in a single-binary architecture |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.3.1 [VERIFIED: proxy.golang.org] | HTTP router | Already locked in CLAUDE.md; confirmed current via Go module proxy `@latest` |
| `github.com/jackc/pgx/v5` | v5.10.0 [VERIFIED: proxy.golang.org] | Postgres driver (`pgxpool`) | Already locked; sqlc's primary supported target when `sql_package: "pgx/v5"` |
| `github.com/sqlc-dev/sqlc` (CLI) | v1.31.1 [VERIFIED: api.github.com/repos/sqlc-dev/sqlc/releases/latest] | SQL → Go codegen | Already locked; confirmed via GitHub Releases API |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 [VERIFIED: proxy.golang.org] | Schema migrations | Already locked; confirmed current |
| `github.com/caarlos0/env/v11` | v11.4.1 [VERIFIED: proxy.golang.org] | Env-var → struct config parsing | Already locked; confirmed current |
| `github.com/go-chi/httplog/v3` | v3.4.0 [VERIFIED: proxy.golang.org] | Structured HTTP request logging on `log/slog` | CLAUDE.md said "latest (v3 module)" without pinning — v3.4.0 is the current tag, pin it explicitly in `go.mod` |
| `log/slog` (stdlib) | Go 1.21+ | JSON structured logging core | No third-party logging library needed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/go-chi/chi/v5/middleware` | bundled with chi | `RequestID`, `Recoverer` | `RequestID` generates the correlation ID; `middleware.NextRequestID()` (same package) generates an ID using the identical scheme *outside* an HTTP request — this is the hook to reuse for Phase 3 poll-cycle logging (see Architecture Patterns) |
| Go toolchain | 1.23+ (1.25 recommended, per CLAUDE.md) | Compiler/runtime | Not detected on this machine's PATH — see Environment Availability |

### Alternatives Considered
No new alternatives surfaced beyond what CLAUDE.md already evaluated (viper, lib/pq, envconfig, etc.) — Phase 1's stack is fully pre-locked. This research only fills in wiring detail, not stack choice.

**Installation:**
```bash
go mod init github.com/danielrpof/drop-tracker
go get github.com/go-chi/chi/v5@v5.3.1
go get github.com/go-chi/httplog/v3@v3.4.0
go get github.com/jackc/pgx/v5@v5.10.0
go get github.com/caarlos0/env/v11@v11.4.1
go get github.com/golang-migrate/migrate/v4@v4.19.1
# sqlc is a separate CLI, not a go.mod dependency:
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
```

**Version verification:** All six package versions above were confirmed live against `proxy.golang.org/<module>/@latest` (the authoritative Go module proxy) and the sqlc GitHub Releases API during this research session — not from training data. All match the versions already locked in `.claude/CLAUDE.md` exactly.

## Package Legitimacy Audit

> The `gsd-tools package-legitimacy check` seam only supports `npm|pypi|crates` ecosystems — Go is not covered. Verification below was performed manually against the authoritative Go module proxy (`proxy.golang.org`) and GitHub, which is the correct authoritative source for this ecosystem.

| Package | Registry | Origin confirmed | Source Repo | Verdict | Disposition |
|---------|----------|-------------------|--------------|---------|-------------|
| `github.com/go-chi/chi/v5` | Go proxy | v5.3.1, tag `refs/tags/v5.3.1` [VERIFIED: proxy.golang.org] | github.com/go-chi/chi (org-maintained) | OK | Approved |
| `github.com/jackc/pgx/v5` | Go proxy | v5.10.0, tag `refs/tags/v5.10.0` [VERIFIED: proxy.golang.org] | github.com/jackc/pgx | OK | Approved |
| `github.com/golang-migrate/migrate/v4` | Go proxy | v4.19.1 [VERIFIED: proxy.golang.org] | github.com/golang-migrate/migrate | OK | Approved |
| `github.com/caarlos0/env/v11` | Go proxy | v11.4.1, tag `refs/tags/v11.4.1` [VERIFIED: proxy.golang.org] | github.com/caarlos0/env | OK | Approved |
| `github.com/go-chi/httplog/v3` | Go proxy | v3.4.0, tag `refs/tags/v3.4.0` [VERIFIED: proxy.golang.org] | github.com/go-chi/httplog (org-maintained) | OK | Approved |
| `github.com/sqlc-dev/sqlc` (CLI, not a go.mod dep) | GitHub Releases | v1.31.1, `html_url: .../releases/tag/v1.31.1` [VERIFIED: api.github.com] | github.com/sqlc-dev/sqlc | OK | Approved |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none — all six packages are org-maintained (go-chi, jackc, golang-migrate, sqlc-dev, caarlos0) with long-established tag histories on the authoritative Go module proxy; none discovered via unverified web search alone (each was cross-checked against `proxy.golang.org` directly).

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────┐
                         │   cmd/server/main.go         │
                         │   (single process entrypoint)│
                         └──────────────┬───────────────┘
                                        │
              ┌─────────────────────────┼──────────────────────────┐
              ▼                         ▼                          ▼
   ┌────────────────────┐   ┌───────────────────────┐   ┌──────────────────────┐
   │ 1. internal/config  │   │ 2. internal/logging    │   │ 3. internal/db        │
   │ env.Parse(&cfg)     │   │ slog.New(JSONHandler)  │   │ pgxpool.New(dsn)      │
   │ required/notEmpty   │   │ + httplog.RequestLogger│   │ retry+backoff loop:   │
   │ tags → fail fast    │   │ (wired to chi router)  │   │  pool.Ping(ctx)       │
   │ exit(1) on error    │   │                        │   │  migrate.Up(pool DSN) │
   └─────────┬───────────┘   └───────────┬────────────┘   │  via embed.FS + iofs  │
             │                           │                │  exit(1) after N tries│
             │                           │                └───────────┬───────────┘
             └─────────────┬─────────────┘                            │
                           ▼                                          ▼
                ┌─────────────────────────┐               ┌───────────────────────┐
                │ 4. internal/httpserver   │◄──────────────┤ sqlc-generated queries │
                │ chi.NewRouter()          │  DB handle    │ internal/db/sqlc/*.go  │
                │ r.Use(RequestID,         │               │ (health ping query)    │
                │        httplog logger,   │               └───────────────────────┘
                │        Recoverer)        │
                │ r.Get("/health", ...)    │
                └────────────┬─────────────┘
                             │
                             ▼
                   ┌───────────────────┐
                   │ GET /health        │
                   │ 200 {"status":"ok",│
                   │  "db":"up"}         │
                   │ 503 on DB-down     │
                   └───────────────────┘
```

Trace: request enters chi router → `middleware.RequestID` stamps a correlation ID into `context.Context` → `httplog.RequestLogger` emits a structured start/end JSON log line carrying that ID → the `/health` handler calls `pool.Ping(ctx)` with a short timeout → response status code (200/503) and JSON body are written → `httplog` logs the outcome with the same correlation ID.

### Recommended Project Structure
```
drop-tracker/
├── cmd/
│   └── server/
│       └── main.go            # single entrypoint — wires config, logging, db, http server
├── internal/
│   ├── config/
│   │   └── config.go           # Config struct, env.Parse, fail-fast error formatting
│   ├── logging/
│   │   └── logging.go          # slog.Logger construction (JSON/text), httplog.Options
│   ├── db/
│   │   ├── pool.go             # pgxpool.New + Ping-with-retry
│   │   ├── migrate.go          # embed.FS + iofs.New + migrate.Up with backoff
│   │   ├── migrations/
│   │   │   ├── 000001_init.up.sql
│   │   │   └── 000001_init.down.sql
│   │   └── sqlc/                # sqlc `out:` target — generated, do not hand-edit
│   │       ├── db.go
│   │       ├── models.go
│   │       └── health.sql.go
│   └── httpserver/
│       ├── server.go            # chi.NewRouter(), middleware wiring, route registration
│       └── health.go            # GET /health handler
├── queries/
│   └── health.sql               # sqlc query source (`-- name: Ping :one SELECT 1;`)
├── sqlc.yaml
├── .env.example
├── go.mod
└── go.sum
```

**Anti-pattern flagged by this research:** do **not** create `cmd/api/` and `cmd/worker/` as separate entrypoints. That split is the generic-web-search default for "API + scheduler" Go services, but it directly contradicts PROJECT.md's locked single-binary/single-process architecture. One `cmd/server/main.go` runs the HTTP server now and will run the cron scheduler in-process starting Phase 3.

### Pattern 1: Fail-fast config with `caarlos0/env/v11`
**What:** Struct-tag driven parsing with `required`/`notEmpty` tags; `env.Parse` returns an `AggregateError` listing every problem at once (not just the first).
**When to use:** All Phase 1 config, and every future phase's config additions (D-06/D-07 — struct is the source of truth from day one).
**Example:**
```go
// Source: https://pkg.go.dev/github.com/caarlos0/env/v11 (WebFetch, MEDIUM confidence)
type Config struct {
    // Phase 1
    DatabaseURL string `env:"DATABASE_URL,notEmpty"` // required AND non-empty
    HTTPPort    int    `env:"HTTP_PORT" envDefault:"8080"`
    LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
    LogFormat   string `env:"LOG_FORMAT" envDefault:"json"`

    // Stubbed for future phases (D-06/D-07) — optional, sane defaults, not `required`
    DiscordWebhookURL string        `env:"DISCORD_WEBHOOK_URL"`
    PollInterval      time.Duration `env:"POLL_INTERVAL" envDefault:"15m"`
    MusicBrainzUA     string        `env:"MUSICBRAINZ_USER_AGENT" envDefault:"drop-tracker/0.1.0"`
}

func Load() (*Config, error) {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        return nil, err // caller logs err (an AggregateError) and os.Exit(1)
    }
    return cfg, nil
}
```
**Important gotcha:** plain `required` only checks the var is *set*, not non-empty — `DATABASE_URL=` (set to empty string) passes `required` but silently produces an empty DSN. Use `notEmpty` for the DB DSN specifically, per D-05's "never boot in a half-configured state."

### Pattern 2: Structured JSON logging with request-ID correlation
**What:** `log/slog` JSON handler + `go-chi/httplog/v3` request logger, layered on chi's `middleware.RequestID`.
**When to use:** Every HTTP request (OPS-02, Phase 1). Reuse the same ID-generation call for poll-cycle logging once Phase 3 adds the scheduler.
**Example:**
```go
// Source: github.com/go-chi/httplog README + pkg.go.dev/.../chi/v5/middleware (WebFetch, MEDIUM confidence)
logFormat := httplog.SchemaECS.Concise(cfg.LogFormat != "json") // concise/text for local dev
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    ReplaceAttr: logFormat.ReplaceAttr,
})).With(
    slog.String("service", "drop-tracker"),
)

r := chi.NewRouter()
r.Use(middleware.RequestID) // generates + stores the correlation ID in ctx
r.Use(httplog.RequestLogger(logger, &httplog.Options{
    Level:         slog.LevelInfo,
    Schema:        logFormat,
    RecoverPanics: true,
}))
r.Use(middleware.Recoverer)
```
**For future poll-cycle correlation (Phase 3, not built now):** generate the cycle's ID the same way chi does internally — `id := middleware.NextRequestID()` — and pass it into `logger.With(slog.Uint64("request_id", id))` for that cycle's scoped logger, so poll-cycle log lines use the identical ID scheme/field name as HTTP request logs rather than inventing a second correlation mechanism (e.g. a random UUID) later.

### Pattern 3: Migrate-on-boot with retry/backoff (D-10)
**What:** Embed migration `.sql` files into the binary, retry `migrate.Up()` with exponential backoff so the service tolerates a Postgres container that's still starting under `docker-compose`/local `docker run`, then `exit(1)` if still failing.
**When to use:** Startup sequence, before the HTTP server begins listening (D-09).
**Example:**
```go
// Source: golang-migrate iofs example (WebFetch, MEDIUM confidence) + hand-rolled backoff (no library needed)
//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(ctx context.Context, dsn string, logger *slog.Logger) error {
    src, err := iofs.New(migrationsFS, "migrations")
    if err != nil {
        return fmt.Errorf("load embedded migrations: %w", err)
    }

    const (
        maxAttempts = 6
        baseDelay   = 500 * time.Millisecond
        maxDelay    = 8 * time.Second
    )
    var lastErr error
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
        if err == nil {
            err = m.Up()
            if err != nil && !errors.Is(err, migrate.ErrNoChange) {
                lastErr = err
            } else {
                return nil // success, or nothing to migrate — both are OK
            }
        } else {
            lastErr = err
        }

        delay := min(baseDelay*time.Duration(1<<(attempt-1)), maxDelay)
        logger.Warn("migration attempt failed, retrying",
            slog.Int("attempt", attempt), slog.Duration("delay", delay), slog.Any("error", lastErr))
        select {
        case <-time.After(delay):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return fmt.Errorf("migrations failed after %d attempts: %w", maxAttempts, lastErr)
}
```
**Parameters (Claude's discretion per CONTEXT.md, recommended here):** 6 attempts, 500ms base delay doubling to an 8s cap → worst case ~total 20s of waiting before `exit(1)`. This comfortably covers a Postgres container's typical 2-10s cold-start window without hanging indefinitely on a genuinely dead DB. `[ASSUMED]` — no single authoritative "correct" number exists for this; these are reasonable, commonly-seen defaults, not a verified external spec. Flag for user confirmation if a different startup SLA is desired.
**Critical gotcha:** `migrate.Up()` returns the sentinel `migrate.ErrNoChange` when there is nothing new to apply — this must be treated as **success**, not an error, or every restart after the first will incorrectly retry/fail.

### Pattern 4: Health handler — DB ping, timeout-bounded, 503 on failure
**What:** `GET /health` pings the DB with a short, request-scoped timeout and maps the result to D-03/D-04's exact response contract.
**When to use:** OPS-01.
**Example:**
```go
// Source: pgxpool docs (WebFetch, MEDIUM confidence) + general Go health-check pattern (WebSearch, LOW confidence — shape cross-checked against D-03/D-04, which are locked decisions)
type healthResponse struct {
    Status string `json:"status"` // "ok" | "degraded"
    DB     string `json:"db"`     // "up" | "down"
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    resp := healthResponse{Status: "ok", DB: "up"}
    status := http.StatusOK
    if err := s.pool.Ping(ctx); err != nil {
        resp.Status, resp.DB = "degraded", "down"
        status = http.StatusServiceUnavailable
        // log the real error server-side; never put it in the response body (avoids leaking DSN/host details — see Security Domain)
        httplog.LogEntrySetField(r.Context(), "db_error", slog.StringValue(err.Error()))
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(resp)
}
```
**Why a bounded timeout matters:** `pgxpool.New()` does **not** validate connectivity — it returns immediately and lazily connects. Without a timeout on `Ping`, a hung network path to Postgres would hang the `/health` request itself (defeating the purpose of a health check that's supposed to fail fast).

### Anti-Patterns to Avoid
- **`cmd/api/` + `cmd/worker/` split:** contradicts the locked single-binary architecture — see above.
- **Creating a new `pgxpool.Pool` per request or per health check:** the pool must be constructed once at startup and shared; the health handler just calls `.Ping()` on the existing pool.
- **Treating `migrate.ErrNoChange` as a failure:** breaks every restart after the first successful migration.
- **Logging the raw DSN:** `DATABASE_URL` contains the Postgres password — never `slog.String("dsn", cfg.DatabaseURL)`; log a redacted form (host/dbname only) if the DSN needs to appear in logs at all.
- **`required` without `notEmpty` on secrets:** `required` alone permits an env var set to an empty string, silently producing a blank DSN/webhook URL.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Env var parsing + required/default validation | A custom `os.Getenv` + manual `if == ""` checks scattered across `main.go` | `caarlos0/env/v11` struct tags | Already locked in CLAUDE.md; aggregates all missing-var errors into one report instead of failing on the first one found |
| Schema migration tracking/versioning | Hand-rolled "has this SQL file run yet" bookkeeping table | `golang-migrate/migrate/v4` | Battle-tested version tracking, up/down semantics, `ErrNoChange` handling |
| SQL-to-Go type mapping | Hand-written `rows.Scan(&x, &y, ...)` boilerplate per query | `sqlc generate` | Compile-time-checked, regenerated automatically from `.sql` files; CI drift-check (D-02) catches divergence |
| HTTP request logging + correlation ID plumbing | A custom logging middleware that generates its own request ID | `go-chi/httplog/v3` + `chi/middleware.RequestID` | Already integrates with chi's own `Recoverer`/context conventions; avoids two incompatible ID schemes existing side-by-side |

**Exception — DO hand-roll:** the startup retry/backoff loop for migrations (Pattern 3). This is ~20 lines of `for` + `time.Sleep`-equivalent logic; pulling in a retry/backoff library (e.g. `cenkalti/backoff`) for something this small and single-purpose adds a dependency for no real benefit, consistent with CLAUDE.md's stated preference for hand-rolling small, well-understood pieces (Discord webhook, MusicBrainz/Deezer clients) over adding libraries.

**Key insight:** every "don't hand-roll" item above is already a locked CLAUDE.md decision — Phase 1's job is wiring them together correctly, not choosing between alternatives.

## Runtime State Inventory

Not applicable — this is a greenfield phase (first phase of a new repo), not a rename/refactor/migration phase. No existing runtime state, stored data, or OS-registered state to inventory. `git status` confirms only `README.md`, `LICENSE`, `.gitignore`, `.claude/CLAUDE.md`, and `.planning/` exist prior to this phase.

## Common Pitfalls

### Pitfall 1: `migrate.ErrNoChange` treated as a real error
**What goes wrong:** After the first successful deploy, every subsequent restart calls `migrate.Up()` again; if the code treats *any* non-nil error from `Up()` as fatal, the service will `exit(1)` on every restart once there's nothing left to migrate.
**Why it happens:** `Up()`'s "nothing to do" signal is a sentinel error value (`migrate.ErrNoChange`), not a nil return — easy to miss if you only check `if err != nil`.
**How to avoid:** Always `errors.Is(err, migrate.ErrNoChange)` and treat that case as success.
**Warning signs:** Service works on first boot, then fails to start on every subsequent restart/redeploy.

### Pitfall 2: `pgxpool.New` returning success even when Postgres is unreachable
**What goes wrong:** `pgxpool.New(ctx, dsn)` validates the DSN string but does not eagerly connect — it can return a non-nil `*Pool` with no error even if Postgres is completely down. If startup logic checks only `err != nil` from `New()`, the service will report "started successfully" and then fail on the first real query/health check.
**Why it happens:** `pgxpool` is a lazy connection pool by design (connections are established on first use).
**How to avoid:** Follow `New()` with an explicit, timeout-bounded `pool.Ping(ctx)` as part of the same startup retry loop that drives migrations (Pattern 3) — don't consider startup complete until `Ping` succeeds.
**Warning signs:** Health check works, but the very first real request/log line after boot shows a fresh connection error that "should have" been caught at startup.

### Pitfall 3: Logging the raw `DATABASE_URL`
**What goes wrong:** The Postgres DSN embeds the password (`postgres://user:password@host:5432/db`). If it's ever passed to `slog` directly (e.g., in a "config loaded" debug log, or in an error wrapping the DSN), the password lands in plaintext in JSON log output — which may flow to a log aggregator, Discord (if logs are ever piped there), or CI artifacts.
**Why it happens:** It's the single most convenient value to log when debugging "why can't I connect," and Go's `%v`/`%s` formatting doesn't redact anything automatically.
**How to avoid:** Never log `cfg.DatabaseURL` (or `err` values that embed it, e.g. some pgx connection errors include the DSN) directly; log only host/dbname, or wrap/strip the error before logging.
**Warning signs:** `grep -r "postgres://" logs/` returns anything.

### Pitfall 4: `sqlc generate` drift going undetected
**What goes wrong:** A developer edits `queries/*.sql` or `sqlc.yaml` but forgets to run `sqlc generate`, so the committed generated code in `internal/db/sqlc/` no longer matches the SQL source — this compiles today but silently diverges from the schema.
**Why it happens:** `sqlc generate` is a manual step with no build-time enforcement unless a CI check exists.
**How to avoid:** Per D-02, add the `sqlc generate && git diff --exit-code` drift check now, even though it's technically a CI concern (CICD-01, Phase 7) — establishing the `make`/script target in Phase 1 costs nothing and documents the expected workflow immediately (this can run as a local `make sqlc-check` target in Phase 1 even before the GitHub Actions job exists in Phase 7).
**Warning signs:** Generated code and `.sql` query files reference different columns/types after a schema change.

### Pitfall 5: `golangci-lint` v1 vs v2 config schema confusion
**What goes wrong:** Copying a `.golangci.yml` from an older blog post/tutorial (pre-2025, v1 schema — `run:`/`linters-settings:` top-level keys) into a v2 install either fails to parse or silently ignores the settings.
**Why it happens:** v2 completely redefined the config file schema (now requires `version: "2"` and a `formatters:` block separate from `linters:`).
**How to avoid:** Not a strict Phase 1 deliverable (linting enforcement is CICD-01, Phase 7), but if a `.golangci.yml` is added opportunistically in Phase 1 for local dev hygiene, use the v2 schema from the start.
**Warning signs:** `golangci-lint run` errors on startup with a config parse error, or runs with zero linters enabled.

## Code Examples

### sqlc.yaml (D-02)
```yaml
# Source: docs.sqlc.dev/en/stable/guides/using-go-and-pgx.html (WebFetch, MEDIUM confidence)
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries"
    schema: "internal/db/migrations"
    gen:
      go:
        package: "sqlc"
        out: "internal/db/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
```

### Trivial health-backing query (D-02)
```sql
-- queries/health.sql
-- name: Ping :one
SELECT 1;
```
This query intentionally references no table, so it has no dependency on any domain schema existing (consistent with D-01 — no domain tables in Phase 1) while still forcing `sqlc generate` to produce real, working, type-checked output.

**Note on D-01/D-02 interaction (flagged as an interpretation, not a verbatim decision):** CONTEXT.md's D-01 says "the initial migration creates only what golang-migrate itself needs (schema_migrations bookkeeping)." In practice, `golang-migrate` creates its own `schema_migrations` version-tracking table automatically the first time `Up()` runs — it is not something a migration `.sql` file writes. The most literal way to satisfy D-01 is therefore to ship **zero domain migration files** in Phase 1 (an empty `migrations/` directory, or a single no-op placeholder migration) and let `migrate.Up()` return `ErrNoChange` on a truly empty migration set — while D-02's "trivial query" is satisfied by a schema-independent `SELECT 1`, as above. `[ASSUMED]` — this reconciles the two decisions but was not spelled out verbatim in CONTEXT.md; confirm with the user during planning if a placeholder migration file is preferred over zero migration files for demonstrating the pipeline end-to-end.

### `.env.example` shape (D-06/D-07)
```bash
# --- Phase 1: required ---
DATABASE_URL=postgres://drop_tracker:changeme@localhost:5432/drop_tracker?sslmode=disable
HTTP_PORT=8080
LOG_LEVEL=info      # debug|info|warn|error
LOG_FORMAT=json     # json|text — use text for local dev readability

# --- Stubbed for future phases (D-06/D-07): optional, defaults are safe placeholders ---
DISCORD_WEBHOOK_URL=
POLL_INTERVAL=15m
MUSICBRAINZ_USER_AGENT=drop-tracker/0.1.0 (contact: you@example.com)
DEEZER_RATE_LIMIT_PER_5S=50
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `database/sql` + `lib/pq` for Postgres | `pgx/v5` direct (no `database/sql` wrapper) | `lib/pq` maintenance-mode for several years | sqlc's `pgx/v5` codegen target uses pgx types (`pgx.Rows`, `pgxpool.Pool`) directly, not `database/sql.DB` — don't mix the two idioms |
| `golangci-lint` v1 `.golangci.yml` schema | v2 schema (`version: "2"`, `formatters:` block) | golangci-lint v2 release | Any config copied from older tutorials will misparse |
| Manual `context.Value` request-ID plumbing | chi's built-in `middleware.RequestID` + `GetReqID`/`NextRequestID` | chi has had this for years; still the current idiom | No need for a separate UUID dependency or hand-rolled context key |

**Deprecated/outdated:** none specific to this phase beyond the `lib/pq`/v1-config items above, which CLAUDE.md already documents project-wide.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Recommended retry/backoff parameters (6 attempts, 500ms→8s exponential, ~20s worst case) for D-10 | Pattern 3 / Code Examples | If Postgres cold-start regularly exceeds ~20s in the developer's actual environment, startup will exit(1) prematurely — parameters are easily tunable constants, low blast radius |
| A2 | D-01 interpretation: zero/no-op domain migration files in Phase 1, with sqlc's trivial query being schema-independent (`SELECT 1`) rather than querying `schema_migrations` | Code Examples (sqlc.yaml / trivial query note) | If the user actually wanted one concrete placeholder migration file to prove the embed/iofs pipeline end-to-end (rather than an empty migrations dir), the planner should add a trivial placeholder migration instead — low effort to correct either way |
| A3 | Recommended internal package split (`internal/config`, `internal/db`, `internal/httpserver`, `internal/logging`) | Recommended Project Structure | Purely organizational; renaming/splitting packages later is a cheap, local refactor with no runtime-state implications (greenfield repo) |
| A4 | Health-check `Ping` timeout of 3 seconds | Pattern 4 | If too short for the actual DB network path, health checks could false-negative under load; easily tunable constant |

## Open Questions

1. **Does OPS-02's "poll cycle" logging requirement need a Phase 1 stub, or is it fully deferred to Phase 3?**
   - What we know: Phase 1's own scope (CONTEXT.md `<domain>`) explicitly excludes scheduler/external-API logic; `robfig/cron` isn't introduced until Phase 3.
   - What's unclear: ROADMAP.md's Phase 1 success criterion #2 literally says "Every HTTP request **and poll cycle** emits a structured JSON log line with a correlating request ID," which Phase 1 alone cannot fully satisfy since no poll cycle exists yet.
   - Recommendation: Phase 1 should implement and prove the *correlation mechanism* (chi `middleware.RequestID`/`NextRequestID` — Pattern 2) generically enough that Phase 3 reuses it verbatim for cron cycles, and the planner should treat "poll cycle" logging as satisfied-by-pattern in Phase 1 with actual poll-cycle log lines arriving in Phase 3. Flag this scoping nuance explicitly in the PLAN.md verification section so it isn't misread as a Phase 1 gap.

2. **Should Phase 1 include a `docker-compose.yml` for local Postgres, even though CICD-09 formally scopes it to Phase 7?**
   - What we know: D-10's retry/backoff rationale explicitly mentions "the common case of the Postgres container still starting under docker-compose," implying local dev already uses some container-based Postgres before Phase 7 formalizes it.
   - What's unclear: Whether Phase 1 should ship a minimal dev-only `docker-compose.yml` (Postgres only) as a convenience, or whether the developer is expected to `docker run postgres` manually until Phase 7.
   - Recommendation: A minimal, Phase-1-scoped `docker-compose.yml` containing only a `postgres:16` service (no app container yet — that's Phase 7's multi-stage build) is low-risk and directly enables local testing of the migrate-on-boot retry logic. Not a hard requirement — Environment Availability below documents the manual-`docker run` fallback either way.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building/running the service at all | ✗ (not on PATH in this session) | — | Install Go 1.23+ (1.25 recommended per CLAUDE.md) before implementation begins; not installable by this research agent |
| Docker | Local Postgres for dev/testing before Phase 7's docker-compose | ✓ | 29.2.1 | — |
| Postgres server | `DATABASE_URL` target for migrate-on-boot, health check, sqlc trivial query | Not confirmed running (no `pg_isready`/`psql` on PATH) | — | Run via Docker (`docker run postgres:16` or a Phase-1-scoped `docker-compose.yml`, per Open Question 2) |

**Missing dependencies with no fallback:**
- Go toolchain — must be installed by the developer/executor before any code in this phase can be built or tested; this blocks execution, not planning.

**Missing dependencies with fallback:**
- Postgres server — Docker is confirmed available, so a containerized Postgres instance is a viable fallback for both local development and any Phase 1 integration tests that need a real DB.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `net/http/httptest` (per CLAUDE.md testing constraint: `httptest.Server` for mocking, no live external calls in CI) |
| Config file | none — greenfield; Wave 0 must create the first `_test.go` files and any shared test fixtures |
| Quick run command | `go test ./... -short` |
| Full suite command | `go test ./... -race` (add `-run TestMigration` gating for DB-backed tests once a Postgres test fixture exists) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| OPS-01 | `/health` returns 200 + `{"status":"ok","db":"up"}` when DB reachable | integration (real or containerized Postgres) | `go test ./internal/httpserver/... -run TestHealth_Up` | ❌ Wave 0 |
| OPS-01 | `/health` returns 503 + `{"status":"degraded","db":"down"}` when DB unreachable | unit (fake/closed pool or injected ping-failure) | `go test ./internal/httpserver/... -run TestHealth_Down` | ❌ Wave 0 |
| OPS-02 | Each HTTP response includes a correlating request ID in both response header and structured log output | unit (httptest.NewRecorder against the wired router, assert `X-Request-Id` header + capture logger output) | `go test ./internal/httpserver/... -run TestRequestID` | ❌ Wave 0 |
| OPS-03 | Missing required env var (`DATABASE_URL`) causes `Load()` to return an error listing what's missing, and the caller exits non-zero | unit (`t.Setenv` to unset/empty the var, assert error content) | `go test ./internal/config/... -run TestLoad_MissingRequired` | ❌ Wave 0 |
| OPS-03 | `.env.example` documents every `Config` struct field | static/lint check (not a Go test — a small script or table-driven reflection test comparing struct tags to `.env.example` keys) | `go test ./internal/config/... -run TestEnvExampleCompleteness` | ❌ Wave 0 |
| D-09/D-10 | Migrations run automatically on boot; transient DB-unreachable is retried, permanent failure exits after N attempts | integration (real/containerized Postgres for success path; a closeable/unreachable DSN for the retry-then-fail path) | `go test ./internal/db/... -run TestRunMigrations` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./... -short` (skips DB-backed integration tests behind `testing.Short()` guards)
- **Per wave merge:** `go test ./... -race` against a live/containerized Postgres
- **Phase gate:** Full suite green (including DB-backed tests) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/httpserver/health_test.go` — covers OPS-01 (both branches)
- [ ] `internal/httpserver/server_test.go` — covers OPS-02 (request-ID presence)
- [ ] `internal/config/config_test.go` — covers OPS-03 (fail-fast + `.env.example` parity)
- [ ] `internal/db/migrate_test.go` — covers D-09/D-10 retry/backoff behavior
- [ ] Test DB fixture/helper: a small `internal/testutil` (or inline `TestMain`) that spins a `postgres:16` container (or connects to `DATABASE_URL` set by CI's future service container) for integration tests — decide now whether to use `testcontainers-go` or rely on a docker-compose/CI-service-container Postgres, since this choice affects every later phase's DB-backed tests too
- [ ] Framework install: no new test framework needed (stdlib `testing` only); `go.mod`/`go.sum` themselves are Wave 0 deliverables since this is a greenfield repo

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | No auth surface exists in Phase 1 (single-operator deployment per PROJECT.md; auth is explicitly out of v1 scope project-wide) |
| V3 Session Management | No | No sessions in Phase 1 |
| V4 Access Control | No | `/health` is intentionally unauthenticated (standard for uptime-monitor consumption); no other endpoints exist yet |
| V5 Input Validation | Partial | Env var values parsed/typed by `caarlos0/env` (e.g. `int`, `time.Duration` fields fail parse on malformed input) — no HTTP request body/query input exists yet in Phase 1 |
| V6 Cryptography | No | No cryptographic operations in Phase 1; `DATABASE_URL` is a connection credential, not something this phase encrypts (TLS to Postgres via `sslmode` is a config value, not a crypto implementation concern here) |
| V7 Error Handling and Logging | Yes | Structured `log/slog` JSON logging (already the stack) — must not log secrets (see Pitfall 3); must not leak internal error detail (DB error strings, stack traces) into HTTP response bodies |
| V14 Configuration | Yes | Env-var-only config (OPS-03) is itself the ASVS-aligned control — no secrets in source, `.env.example` documents shape without real values, `.gitignore`/gitleaks (project-wide, CLAUDE.md) prevent accidental commits |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Secret (DB password) leakage via structured logs or `/health` error responses | Information Disclosure | Never log/return the raw DSN or raw pgx connection errors that embed it; return only the fixed `{"status","db"}` shape (D-03) to clients, log detail server-side with the DSN stripped/redacted |
| `.env` file accidentally committed | Information Disclosure | `.gitignore` must exclude `.env` (only `.env.example` is committed); gitleaks pre-commit hook + CI backstop are project-wide CLAUDE.md requirements, not new to this phase |
| Unbounded health-check DB call hanging the process under a DB outage | Denial of Service (resource exhaustion under repeated slow health polls) | Timeout-bounded `pool.Ping(ctx)` (Pattern 4) — a monitoring system polling `/health` every few seconds against a hung DB must not accumulate blocked goroutines |
| Migration retry loop retrying forever against a permanently broken DB | Denial of Service (service never becomes ready, but also never fails loudly) | Bounded attempt count with `exit(1)` (D-10) — never loop indefinitely |

## Sources

### Primary (HIGH confidence)
- `proxy.golang.org/<module>/@latest` — version verification for chi, pgx, caarlos0/env, golang-migrate, httplog (VERIFIED, queried live this session)
- `api.github.com/repos/sqlc-dev/sqlc/releases/latest` — sqlc CLI version verification (VERIFIED, queried live this session)

### Secondary (MEDIUM confidence)
- `docs.sqlc.dev/en/stable/guides/using-go-and-pgx.html` (WebFetch) — sqlc.yaml pgx/v5 config shape
- `github.com/golang-migrate/migrate` iofs example (`source/iofs/example_test.go`, WebFetch) — embed.FS + iofs.New wiring
- `pkg.go.dev/github.com/caarlos0/env/v11` (WebFetch) — required/notEmpty tag behavior, AggregateError
- `github.com/go-chi/httplog` README (WebFetch) — RequestLogger/Options wiring
- `pkg.go.dev/github.com/go-chi/chi/v5/middleware` (WebFetch) — RequestID/GetReqID/NextRequestID
- `pgxpool` package docs (WebFetch, jackc/pgx) — `New()` lazy-connect + `Ping` behavior

### Tertiary (LOW confidence)
- General Go project layout guidance (WebSearch, multiple blog sources) — cross-checked against and **overridden** by PROJECT.md's locked single-binary architecture where they conflicted (cmd/api+cmd/worker split rejected)
- Generic Go health-check-library patterns (WebSearch) — shape ultimately governed by locked D-03/D-04, not by the surveyed libraries themselves (none of which are being adopted — the handler is hand-rolled, ~15 lines)
- golangci-lint v2 schema summary (WebSearch) — informational only, not a Phase 1 deliverable

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every package version independently verified against the authoritative Go module proxy/GitHub API this session, matching CLAUDE.md exactly
- Architecture: HIGH — project layout and wiring patterns cross-checked against official docs (chi, httplog, pgx, golang-migrate, sqlc) and corrected against PROJECT.md's locked architecture where generic guidance conflicted
- Pitfalls: MEDIUM — sourced from official docs and well-known Go idioms; retry/backoff exact parameters (A1) and D-01/D-02 reconciliation (A2) are engineering judgment calls flagged as assumptions

**Research date:** 2026-08-04
**Valid until:** 2026-09-03 (30 days — stable, mature libraries; re-verify versions if planning is delayed significantly past this window)
