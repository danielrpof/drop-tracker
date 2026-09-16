# Phase 1: Foundation — Data Layer, Config & Health - Pattern Map

**Mapped:** 2026-08-04
**Files analyzed:** 15 (new)
**Analogs found:** 0 / 15 — greenfield repository, no existing code

## Greenfield Notice

This is the first phase of a new repository. Prior to this phase only `README.md`, `LICENSE`, `.gitignore`, `.claude/`, and `.planning/` exist — confirmed via directory listing (`ls -la` shows no `cmd/`, `internal/`, `go.mod`, or any `.go` files). **There are zero existing code analogs to copy patterns from.**

Because there is no in-repo precedent, the pattern reference for this phase is **RESEARCH.md** (`.planning/phases/01-foundation-data-layer-config-health/01-RESEARCH.md`), specifically its `## Architecture Patterns` and `## Code Examples` sections, which give concrete, sourced code shapes for every file below (config, logging, migrate-on-boot, health handler, sqlc wiring). These are external/official-doc-derived patterns (chi, pgx, sqlc, golang-migrate, caarlos0/env, httplog READMEs/docs), not codebase analogs — treat RESEARCH.md's `Pattern 1`–`Pattern 4` and the `sqlc.yaml` / `.env.example` code blocks as the canonical shape planners should follow.

All subsequent phases (2+) will be able to use Phase 1's own output (e.g. `internal/config/config.go`, `internal/httpserver/server.go`) as real in-repo analogs — this phase is the one that creates that precedent.

## File Classification

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|-----------------|----------------|
| `cmd/server/main.go` | entrypoint | request-response (wires everything) | none | no analog |
| `internal/config/config.go` | config | transform (env → struct) | none | no analog |
| `internal/config/config_test.go` | test | transform | none | no analog |
| `internal/logging/logging.go` | utility/provider | event-driven (log emission) | none | no analog |
| `internal/db/pool.go` | service | CRUD (connection mgmt) | none | no analog |
| `internal/db/migrate.go` | migration | batch (schema apply) | none | no analog |
| `internal/db/migrate_test.go` | test | batch | none | no analog |
| `internal/db/migrations/000001_init.up.sql` | migration | batch | none | no analog |
| `internal/db/migrations/000001_init.down.sql` | migration | batch | none | no analog |
| `internal/db/sqlc/*.go` (generated) | model/service | CRUD | none | no analog |
| `queries/health.sql` | model (sqlc source) | CRUD | none | no analog |
| `internal/httpserver/server.go` | controller/route | request-response | none | no analog |
| `internal/httpserver/health.go` | controller | request-response | none | no analog |
| `internal/httpserver/health_test.go` | test | request-response | none | no analog |
| `internal/httpserver/server_test.go` | test | request-response | none | no analog |
| `sqlc.yaml` | config | transform | none | no analog |
| `.env.example` | config | — | none | no analog |

## Pattern Assignments

Since no in-repo analog exists for any file, each assignment below points to the RESEARCH.md section that serves as the pattern source. Planners should treat these as "copy this shape" references.

### `internal/config/config.go` (config, transform)

**Source:** RESEARCH.md → `Pattern 1: Fail-fast config with caarlos0/env/v11` (lines 189-216)

**Core pattern:**
```go
type Config struct {
    DatabaseURL string `env:"DATABASE_URL,notEmpty"`
    HTTPPort    int    `env:"HTTP_PORT" envDefault:"8080"`
    LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
    LogFormat   string `env:"LOG_FORMAT" envDefault:"json"`

    DiscordWebhookURL string        `env:"DISCORD_WEBHOOK_URL"`
    PollInterval      time.Duration `env:"POLL_INTERVAL" envDefault:"15m"`
    MusicBrainzUA     string        `env:"MUSICBRAINZ_USER_AGENT" envDefault:"drop-tracker/0.1.0"`
}

func Load() (*Config, error) {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        return nil, err // caller logs err (AggregateError) and os.Exit(1)
    }
    return cfg, nil
}
```

**Gotcha (must apply):** use `notEmpty` not just `required` on `DATABASE_URL` — `required` alone permits an empty-string value to pass validation (RESEARCH.md Pitfall/Anti-Pattern list, line 216 and 327).

---

### `internal/logging/logging.go` (provider, event-driven)

**Source:** RESEARCH.md → `Pattern 2: Structured JSON logging with request-ID correlation` (lines 218-240)

**Core pattern:**
```go
logFormat := httplog.SchemaECS.Concise(cfg.LogFormat != "json")
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    ReplaceAttr: logFormat.ReplaceAttr,
})).With(slog.String("service", "drop-tracker"))

r := chi.NewRouter()
r.Use(middleware.RequestID)
r.Use(httplog.RequestLogger(logger, &httplog.Options{
    Level:         slog.LevelInfo,
    Schema:        logFormat,
    RecoverPanics: true,
}))
r.Use(middleware.Recoverer)
```

**Forward-compatibility note for Phase 3:** reuse `middleware.NextRequestID()` for poll-cycle log correlation rather than inventing a second ID scheme — don't build this in Phase 1, but design `logging.go` so the logger construction is reusable outside an HTTP context (e.g. exported `NewLogger(cfg)` returning `*slog.Logger`, not tightly coupled to the router).

---

### `internal/db/migrate.go` (migration, batch)

**Source:** RESEARCH.md → `Pattern 3: Migrate-on-boot with retry/backoff` (lines 242-289)

**Core pattern:**
```go
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
                return nil
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

**Critical gotcha (must apply):** `errors.Is(err, migrate.ErrNoChange)` must be treated as success — every restart after the first successful migration returns this sentinel, not nil (RESEARCH.md Pitfall 1).

---

### `internal/httpserver/health.go` (controller, request-response)

**Source:** RESEARCH.md → `Pattern 4: Health handler — DB ping, timeout-bounded, 503 on failure` (lines 291-320)

**Core pattern:**
```go
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
        httplog.LogEntrySetField(r.Context(), "db_error", slog.StringValue(err.Error()))
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(resp)
}
```

**Gotcha (must apply):** `pgxpool.New()` is lazy and doesn't validate connectivity — the pool must be constructed once at startup (shared, not per-request), and the timeout on `Ping` is mandatory to avoid a hung DB path hanging the health request itself.

---

### `sqlc.yaml` / `queries/health.sql` (config / model source)

**Source:** RESEARCH.md → `Code Examples: sqlc.yaml (D-02)` and `Trivial health-backing query (D-02)` (lines 380-402)

```yaml
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

```sql
-- queries/health.sql
-- name: Ping :one
SELECT 1;
```

**Interpretation note (RESEARCH.md A2, flag to planner):** D-01 says the initial migration creates only what golang-migrate itself needs. In practice `golang-migrate` auto-creates `schema_migrations` on first `Up()` — it's not written by a `.sql` file. Research recommends zero/no-op domain migration files in Phase 1, with the `SELECT 1` query above satisfying D-02 independent of any domain schema. Confirm this interpretation during planning if a placeholder migration file is preferred instead.

---

### `.env.example` (config)

**Source:** RESEARCH.md → `.env.example shape (D-06/D-07)` (lines 406-419)

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

---

### `cmd/server/main.go` (entrypoint)

**Source:** RESEARCH.md → `System Architecture Diagram` and `Recommended Project Structure` (lines 112-187)

**Wiring order (no code excerpt exists yet — this is the sequencing contract):**
1. `config.Load()` → fail fast, `exit(1)` on error (Pattern 1)
2. `logging.New(cfg)` → construct `*slog.Logger`
3. `db.NewPool(ctx, cfg.DatabaseURL)` + `db.RunMigrations(...)` with retry/backoff (Pattern 3) — must complete before HTTP server starts listening (D-09)
4. `httpserver.New(pool, logger)` → chi router with `middleware.RequestID`, `httplog.RequestLogger`, `middleware.Recoverer`, `/health` route (Pattern 2 + Pattern 4)
5. `http.ListenAndServe`

**Anti-pattern (must avoid):** do not create `cmd/api/` + `cmd/worker/` as separate entrypoints — single `cmd/server/main.go` only, per PROJECT.md's locked single-binary architecture.

---

## Shared Patterns

### Fail-fast, exit(1) on misconfiguration
**Source:** RESEARCH.md Pattern 1, D-05
**Apply to:** `internal/config/config.go`, `cmd/server/main.go`
Never boot in a half-configured state — log the full `AggregateError` and exit non-zero immediately.

### Never log secrets (DSN)
**Source:** RESEARCH.md Pitfall 3, Security Domain
**Apply to:** `internal/logging/logging.go`, `internal/db/pool.go`, `internal/db/migrate.go`, `internal/httpserver/health.go`
Never `slog.String("dsn", cfg.DatabaseURL)` or log raw pgx connection errors that embed the DSN — log host/dbname only, or a redacted form.

### Request-ID correlation via chi middleware
**Source:** RESEARCH.md Pattern 2
**Apply to:** `internal/httpserver/server.go` (now), Phase 3 cron scheduler (later, via `middleware.NextRequestID()`)
Use chi's built-in ID scheme everywhere — don't introduce a second UUID-based correlation mechanism.

### Timeout-bounded external calls
**Source:** RESEARCH.md Pattern 4, Security Domain (DoS mitigation)
**Apply to:** `internal/httpserver/health.go` (`pool.Ping` with 3s timeout)
Every DB call reachable from an HTTP handler must be timeout-bounded to prevent hung goroutines under an outage.

## No Analog Found

All files in this phase have no in-repo analog — this is expected and correct for a greenfield first phase.

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| all 15 files listed above | various | various | No application code exists anywhere in the repository prior to this phase (verified via directory listing: only `README.md`, `LICENSE`, `.gitignore`, `.claude/`, `.planning/` present) |

**Planner guidance:** use RESEARCH.md's `Pattern 1`–`Pattern 4` and Code Examples sections (cited above with line numbers) as the pattern source for every file's action section, in place of a codebase analog.

## Metadata

**Analog search scope:** entire repository root (`ls -la`, confirmed no `cmd/`, `internal/`, or `.go` files exist)
**Files scanned:** 0 Go files found (greenfield)
**Pattern extraction date:** 2026-08-04
</content>
