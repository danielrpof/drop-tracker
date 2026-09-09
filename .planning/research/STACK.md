# Stack Research

**Domain:** Operator-observability features on an already-shipped Go 1.26 + React/Vite single-binary service (drop-tracker v1.4)
**Researched:** 2026-09-09
**Confidence:** HIGH

## Headline

**No new dependencies — Go or JavaScript — are needed for any of the four v1.4 features.**

Every capability the milestone asks for is covered by what is already in `go.mod`, `web/package.json`, and the stdlib:

| Feature | Covered by | New dep? |
|---------|-----------|----------|
| `/ready` readiness endpoint | `chi/v5` routing + existing `golang-migrate/migrate/v4` (`source.Driver` walk, already done in `internal/db/migrate.go:326`) + `pgx/v5` or a sqlc query against `schema_migrations` | No |
| `poll_runs` table + `RunRecorder` seam | new migration file + `golang-migrate/migrate/v4` (unchanged) + `sqlc` v1.31.1 codegen + `pgx/v5` + consumer-defined interface (same pattern as `EventRecorder` in `internal/poller/poller.go:86`) + `sync/atomic` (stdlib, already imported) | No |
| `GET /status` gated JSON endpoint | `chi/v5` + existing `authgate` Group (`registerDataRoutes`, `server.go:197`) + `encoding/json` (stdlib) + new sqlc queries | No |
| React "System" view | new `react-router` route + existing `web/app/lib/api.ts` fetch pattern + existing `@base-ui/react` / shadcn components + `lucide-react` + `sonner` + `Intl` for time formatting | No |
| Integration tests for `poll_runs` | existing `internal/testutil` (`NewTestPool` / `NewIsolatedTestPool`) against the docker-compose / CI-service Postgres | No |
| Frontend tests for System view | existing Vitest 4 + RTL + `createRoutesStub` + mocked `api.ts` | No |

The rest of this document is the evidence for that conclusion and the list of things to explicitly **not** add.

## Recommended Stack

### Core Technologies (all already present — versions from `go.mod` / `package.json`)

| Technology | Version | Purpose in v1.4 | Why it already covers this |
|------------|---------|-----------------|----------------------------|
| `github.com/go-chi/chi/v5` | v5.3.1 | Register `/ready` (public, root router) and `/status` (inside the gated `chi.Group`) | `/ready` mirrors the existing exact-path `/health` registration (`server.go:164`); `/status` is one more line in `registerDataRoutes` (`server.go:197-204`) so it inherits `authgate.Authenticate` + `RequireCSRFHeader` for free |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 | Compute "expected schema version" = max embedded migration version, for the `/ready` schema check | `internal/db/migrate.go` **already** does exactly this walk: `maxSourceVersion(src)` (line 326) drives `src.First()`/`src.Next()` to the highest version, and `m.Version()` (line 298) reads the applied version + dirty flag. `/ready` reuses this logic — no new import, just a new exported helper in package `db` |
| `github.com/jackc/pgx/v5` | v5.10.0 | Read `schema_migrations` for `/ready`; INSERT/SELECT/prune `poll_runs` | Already the process pool driver (`internal/db/pool.go`). `/ready`'s applied-version read is a one-row query; no need to build a `migrate.Migrate` instance at request time (that opens a fresh `database/sql` handle — avoid it) |
| `github.com/sqlc-dev/sqlc` (CLI) | v1.31.1 (pinned, `make sqlc-check`) | Generate type-safe accessors for the new `poll_runs` queries | `sqlc.yaml` is already configured (`sql_package: pgx/v5`, `emit_interface: true`, `emit_pointers_for_null_types: true`). New file `queries/poll_runs.sql` → `sqlc generate` → done. The retention prune is expressible as one plain SQL `DELETE` with a `row_number() OVER (PARTITION BY source ORDER BY started_at DESC)` subquery — sqlc handles it |
| `robfig/cron/v3` | v3.0.1 | Unchanged — the poll cycle that a `poll_runs` row records | The `RunRecorder` call sits in `runCycle` (`poller.go:270`) after `wg.Wait()`, not in the cron wiring |
| `log/slog` | stdlib | Unchanged structured logging for the new endpoints | `httplog/v3` middleware already attaches request context; `/ready` follows `/health`'s pattern of `httplog.SetAttrs` for the error detail (`health.go:38`) |
| `encoding/json` | stdlib | Marshal the `/status` response body | Same as every existing handler (`health.go:43`, events handler) |
| React | 19.2.6 | "System" route component | Existing SPA |
| `react-router` | 7.18.2 | Add `route("system", "routes/system.tsx", ...)` to `web/app/routes.ts` | Exactly the shape of the existing `history` route (`routes.ts`) |

### Supporting Libraries (all already present)

| Library | Version | Purpose in v1.4 | Note |
|---------|---------|-----------------|------|
| `sync/atomic` | stdlib (already imported in `poller.go:23`) | Aggregate per-cycle counters (artists checked / errored / events recorded) across the bounded worker-pool goroutines before the `RunRecorder` call | The workers already run concurrently; the cycle already uses `atomic.Bool` guards and an `atomic.Uint64` cycle counter. Add `atomic.Int64` counters incremented in `fetchAndRecord`, read once after `wg.Wait()` |
| `time` | stdlib | `started_at` / `finished_at` timestamps for `poll_runs`; `Intl`/`toLocaleString` on the frontend for "last run 3m ago" | No date library on the frontend today (`package.json` has none) — keep it that way; `Intl.RelativeTimeFormat` + `Date.toLocaleString` are sufficient |
| `@base-ui/react` + shadcn-derived components in `web/app/components` | ^1.7.0 | System view layout (cards / table / badges) | Same component vocabulary as Watchlist/History |
| `lucide-react` | ^1.31.0 | Status icons (check / alert / clock) | Already the icon set |
| `sonner` | ^2.0.8 | Toast on `/status` fetch failure | Already the toast lib |
| `web/app/lib/api.ts` | — | `apiFetch('/status')` — carries the gate cookie + `X-Instance-Gated` handling automatically | The System view must call `/status` through this wrapper, not bare `fetch`, so it inherits the auth-gate 401 handling |

### Development / Test Tools (all already present)

| Tool | Purpose in v1.4 | Notes |
|------|-----------------|-------|
| `internal/testutil` | Real-Postgres integration coverage for the `poll_runs` DB layer | Use **`NewIsolatedTestPool(t, "pollruns")`**, not `NewTestPool`. The `/status` "last run per source" query is a global, unfiltered `DISTINCT ON (source)` — exactly the cross-package-contamination hazard the `testutil` docstring calls out for `NotifyPending`. A package-scoped schema keeps row counts deterministic under Go's default package-level test parallelism |
| `sqlc generate` + `make sqlc-check` | Catch drift between `queries/poll_runs.sql` and committed generated code | Already a local gate (no CI counterpart — run it before commit per CLAUDE.md) |
| `cmd/migration-check` (Phase 16) | Will inspect the new `000008_poll_runs` migration | `CREATE TABLE` + `CREATE INDEX` are non-destructive → passes clean. The retention `DELETE` lives in `queries/`, not in a migration, so it is not DDL and not in scope for the N-1 check |
| `internal/db/migrations/README.md` | Expand/contract rule for the new migration | New additive table = pure "expand"; no contract step, no N-1 concern |
| Vitest 4.1.10 + RTL 16.3.2 + `createRoutesStub` | System view component test with mocked `api.ts` | Same harness as the 5 existing frontend test files; 70% frontend coverage gate applies |
| `go test` + `internal/testutil` | `RunRecorder` seam test with a fake (no DB) + sqlc-impl test (real DB) | Mirrors how `EventRecorder` is tested with a fake in `internal/poller` and the real impl is tested in its own package |

## Installation

```bash
# Nothing to install. No `go get`, no `pnpm add`.

# The only codegen step (after writing queries/poll_runs.sql + the new migration):
sqlc generate
make sqlc-check
```

## Concrete integration notes (for the roadmapper)

### (a) `/ready` — reading applied vs. expected migration version

`internal/db/migrate.go` already contains both halves:

- **Expected version** = `maxSourceVersion(src)` (line 326-341) — walks the embedded `source.Driver` via `First()`/`Next()` to the highest migration number. Currently unexported and takes a `source.Driver`.
- **Applied version + dirty** = `m.Version()` (line 298) from a `migrate.Migrate` instance.

Recommended shape: add an exported helper to package `db`, e.g. `func ExpectedSchemaVersion() (uint, error)` (wraps `iofs.New(migrationsFS, "migrations")` + `maxSourceVersion`) and let `internal/httpserver` read the **applied** version with a plain pooled query — `SELECT version, dirty FROM schema_migrations` — rather than constructing a `migrate.Migrate` per request (that opens a fresh `database/sql` handle via `sql.Open("pgx", dsn)`, `migrate.go:273`, which is wasteful on a health-probe hot path). A sqlc query (`queries/health.sql` already exists as the home for this kind of infra query) is the clean option.

`/ready` = 200 iff: DB `Ping` succeeds **and** `dirty == false` **and** `applied == expected`. Return 503 otherwise, with the discriminating detail on the log line (`httplog.SetAttrs`), never in the body — same discipline as `/health` (`health.go:22`, DSN/credential non-leak).

New seam consideration: `httpserver.Server` currently holds `db Pinger` (a 1-method interface, `server.go:25`). `/ready` needs one more DB call (read `schema_migrations`) plus the embedded-source max. Either widen with a second narrow interface (`SchemaReader`) the same way `watchlist.Store` / `events.Store` were added as separate fields (`server.go:32-33`), or pass a small `func(ctx) (readyState, error)` closure. Do **not** widen `Pinger` itself — `server.go:84` explicitly warns that breaks `stubPinger`.

### (b) `poll_runs` + `RunRecorder`

- **Migration**: `000008_poll_runs.up.sql` / `.down.sql` — `CREATE TABLE poll_runs (id bigserial pk, source text not null, started_at timestamptz not null, finished_at timestamptz not null, artists_checked int not null, artists_errored int not null, events_recorded int not null, outcome text not null)` + `CREATE INDEX ON poll_runs (source, started_at DESC)`. Purely additive.
- **Seam**: declare `RunRecorder` **in `internal/poller`** (the consumer), next to `EventRecorder`/`Notifier` (`poller.go:80-101`), e.g. `RecordRun(ctx, RunResult) error`. `RunResult` is a poller-owned struct. This keeps the poller DB-connection-free exactly as the docstring promises (`poller.go:12-14`).
- **Wiring**: `runCycle` (`poller.go:270`) already computes `cycleStart` (line 286) and `len(entries)` (line 380). Add `atomic.Int64` counters incremented inside the `fetchAndRecord` closures on error / on events recorded, then one `p.runs.RecordRun(...)` call after `wg.Wait()` (line 363), before/after `NotifyPending`. A recorder failure should be logged, not propagated — same rule as `NotifyPending` (`poller.go:394`).
- **Impl**: sqlc-backed, lives in `cmd/server` (matching where the real `EventRecorder`/`Notifier` impls are assembled). `InsertPollRun` + the prune run in one transaction, or prune as a second statement right after insert. Prune query:
  ```sql
  DELETE FROM poll_runs
  WHERE id IN (
    SELECT id FROM (
      SELECT id, row_number() OVER (PARTITION BY source ORDER BY started_at DESC) AS rn
      FROM poll_runs
    ) ranked
    WHERE rn > @keep_per_source
  );
  ```
  `@keep_per_source` is a compile-time constant in Go (PROJECT.md: "no new env var").

### (c) `GET /status`

- One line in `registerDataRoutes` (`server.go:197`): `r.Get("/status", s.handleStatus)`. Inherits the gate automatically in the gated branch and is flat in the inert branch — matches the existing six data routes.
- New sqlc queries in `queries/poll_runs.sql`: `LastRunPerSource` (`SELECT DISTINCT ON (source) ... ORDER BY source, started_at DESC`), `RecentRuns` (`... ORDER BY started_at DESC LIMIT @n`). Watchlist size: reuse or add a `COUNT(*)` query in `queries/watchlist.sql`. Poll interval: not in the DB — inject the `time.Duration` from `config.Config` into the handler (constructor arg on `Server` or a closure), render as e.g. `"15m0s"` or seconds.
- Response body: a hand-written struct + `json.NewEncoder(w).Encode(...)`, same as `healthResponse` (`health.go:23`).

### (d) Frontend "System" view

- `web/app/routes.ts`: add `route("system", "routes/system.tsx", { id: "system-path" })`.
- `web/app/routes/system.tsx`: `useEffect` → `apiFetch('/status')` (via `web/app/lib/api.ts`, **not** bare `fetch` — needed for gate-cookie + 401 handling), render with existing card/table components from `web/app/components`, `lucide-react` icons, `sonner` toast on failure.
- Time formatting: `Intl.RelativeTimeFormat` / `Date.prototype.toLocaleString` — **no date library**.
- Nav: add a "System" link wherever the Watchlist/History tabs are rendered.
- Test: `web/app/routes/system.test.tsx` with `createRoutesStub` + `vi.mock` on `api.ts` returning a canned `/status` payload — identical pattern to the existing 5 test files. Watch the 70% frontend coverage gate.

## Alternatives Considered

| Recommended | Alternative | When the alternative would make sense |
|-------------|-------------|--------------------------------------|
| Reuse `golang-migrate` source walk for expected version | `embed.FS` + hand-rolled filename parse in `httpserver` | Never — the walk already exists in package `db`; duplicating the "parse `NNNNNN_name.up.sql`" logic elsewhere invites drift |
| Read `schema_migrations` with a pooled query / sqlc | Build a `migrate.Migrate` instance per `/ready` call and use `m.Version()` | Never on the request path — it opens a fresh `database/sql` connection each call. Fine only for a one-shot CLI check |
| Consumer-defined `RunRecorder` interface in `internal/poller` | Pass `*sqlc.Queries` (or `pgxpool.Pool`) straight into the poller | Never — it breaks the "poller holds no DB connection" invariant (`poller.go:12`) and the established seam pattern that every poller test depends on |
| Plain SQL window-function prune | A separate retention goroutine / cron entry | If prune-on-insert ever shows up as write-latency in profiling. Not a v1.4 concern — the table is tiny (N rows per source) |
| `NewIsolatedTestPool` for `poll_runs` tests | `NewTestPool` (shared `public` schema) | Only for tests that never assert exact row counts. `/status`'s global `DISTINCT ON (source)` query does assert them, so isolation is required |
| `Intl` for relative time | `date-fns` / `dayjs` | If the System view grows a real time-series/calendar UI later. A single "N minutes ago" column does not justify a dependency |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `github.com/prometheus/client_golang` (or any metrics client) | Prometheus `/metrics` + Grafana are **explicitly deferred** in PROJECT.md (v1.4 "Out of scope", and the v1 "Out of Scope" list). `/status` is a human-facing JSON snapshot, not a metrics scrape target | `encoding/json` + sqlc queries |
| `recharts`, `visx`, `chart.js`, `d3`, `nivo`, `victory` | Historical charts/graphs are **explicitly out of scope** this cycle (PROJECT.md). The System view is a status panel: last run, recent history table, counts | Plain tables + badges from the existing component set |
| `date-fns`, `dayjs`, `luxon`, `moment` | No date library in `web/` today; a single relative-time column doesn't earn one; adds to the 70% coverage surface and bundle | `Intl.RelativeTimeFormat`, `Date.toLocaleString` |
| An ORM (`gorm`, `bun`, `ent`, `sqlboiler`) | PROJECT.md constraint: sqlc for DB access. `poll_runs` is four trivial queries | `sqlc` v1.31.1 (already configured) |
| `testcontainers-go` | `internal/testutil`'s own docstring rejected it: keeps the test dep set to what's in `go.mod`, needs no Docker socket from inside the test process, works unchanged against the CI Postgres service container | `internal/testutil.NewTestPool` / `NewIsolatedTestPool` |
| A dedicated health/readiness library (`tavsec/gin-healthcheck`, `hellofresh/health-go`, `alexliesenfeld/health`) | `/health` is already a 15-line hand-rolled handler (`health.go`); `/ready` is the same plus a version comparison. A framework adds config surface and a dependency for ~25 lines of code | Hand-rolled handler mirroring `health.go` |
| `golang.org/x/sync/errgroup` for the per-cycle counter aggregation | The cycle already has its own `sync.WaitGroup` + semaphore machinery (`poller.go:305`); counters are `atomic.Int64` reads after `wg.Wait()`. errgroup solves error-propagation, which the poller deliberately does **not** want per-artist (`poller.go` PERF-03) | `sync/atomic` (already imported) |
| A new env var for `poll_runs` retention N | PROJECT.md: "retention by pruning to last N rows per source on insert (**no new env var**)" | A `const` in the recorder impl |
| Constructing `migrate` with the generic `postgres://` database driver | `migrate.go:257` note: golang-migrate's generic "postgres" driver pulls in `lib/pq`, which CLAUDE.md forbids | The existing `database/pgx/v5` migrate driver (`pgxmigrate`), already imported |

## Stack Patterns by Variant

**If the `/ready` schema check needs to run before the pool is fully warmed (startup race):**
- Have `/ready` return 503 (not 500) while `schema_migrations` is unreadable, and let the probe retry
- Because a deploy health-gate (future Phase 17) polls `/ready` in a loop; a transient 503 during boot is the correct "not ready yet" signal, and `RunMigrations` already has a bounded retry loop (`migrate.go:226`) so the window is short

**If `poll_runs` "last run per source" queries show contamination in CI:**
- Switch the poller's DB-layer tests to `NewIsolatedTestPool(t, "pollruns_<pkg>")`
- Because Go runs package test binaries in parallel against the one shared fixture Postgres, and a global `DISTINCT ON (source)` cannot tell another package's row from its own (documented at length in `internal/testutil/postgres.go`)

**If the System view later needs live updates:**
- Poll `/status` on an interval from the React component (`setInterval` + cleanup)
- Because there is no WebSocket/SSE infrastructure in the app and adding one for an operator panel is disproportionate; a 10-30s poll is fine

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `sqlc` v1.31.1 | `pgx/v5` v5.10.0 | Already proven — `sqlc.yaml` sets `sql_package: "pgx/v5"`, `make sqlc-check` green today. New `poll_runs` queries use the same generator path |
| `golang-migrate/migrate/v4` v4.19.1 | embedded `iofs` source + `pgx/v5` database driver | Already the exact wiring in `migrate.go`; `maxSourceVersion` relies only on the documented `os.ErrNotExist` end-of-source signal from `source.Driver.Next`, not on library error-text matching (`migrate.go:320`) |
| `chi/v5` v5.3.1 | `authgate.Manager` Group middleware | `/status` added inside `registerDataRoutes` inherits `Authenticate` + `RequireCSRFHeader` with zero new wiring (`server.go:172-178`) |
| `react-router` 7.18.2 (SPA mode, `ssr:false`) | new `routes/system.tsx` | `go:embed all:build/client` picks up the new route's chunk automatically; no Go-side change for the route itself |
| Vitest 4.1.10 + `@vitest/coverage-v8` 4.1.10 | `createRoutesStub` from `react-router` 7.18.2 | Existing test harness; 70% coverage threshold enforced in `web/vitest.config.ts` |
| Go 1.26 toolchain | all of the above | `go.mod` already on `go 1.26`; no toolchain bump needed |

## Sources

- `C:/CodeProjects/drop-tracker/go.mod` — full Go dependency set (chi v5.3.1, httplog/v3 v3.4.0, migrate/v4 v4.19.1, pgx/v5 v5.10.0, cron/v3 v3.0.1, caarlos0/env/v11 v11.4.1, x/time v0.15.0) — HIGH confidence, source of truth
- `C:/CodeProjects/drop-tracker/web/package.json` — full frontend dep set (React 19.2.6, react-router 7.18.2, @base-ui/react 1.7.0, lucide-react 1.31.0, sonner 2.0.8, Vitest 4.1.10, RTL 16.3.2; no date/chart library present) — HIGH confidence, source of truth
- `C:/CodeProjects/drop-tracker/internal/db/migrate.go` — `maxSourceVersion` (line 326) source-walk and `m.Version()` (line 298) applied-version read already implemented; generic-postgres/lib/pq prohibition (line 257) — HIGH confidence
- `C:/CodeProjects/drop-tracker/internal/poller/poller.go` — `EventRecorder`/`Notifier` consumer-defined seam pattern (lines 80-101), `runCycle` counter/timing hooks (lines 270-399), `sync/atomic` already imported — HIGH confidence
- `C:/CodeProjects/drop-tracker/internal/httpserver/server.go` + `health.go` — exact-path route registration, `registerDataRoutes` gated Group, narrow-interface (`Pinger`) discipline, credential-non-leak response contract — HIGH confidence
- `C:/CodeProjects/drop-tracker/sqlc.yaml` + `queries/health.sql` — sqlc v2 config, `sql_package: pgx/v5`, `emit_interface`, home for infra queries — HIGH confidence
- `C:/CodeProjects/drop-tracker/internal/testutil/postgres.go` — `NewTestPool` / `NewIsolatedTestPool`, explicit rejection of testcontainers-go, cross-package global-query contamination hazard — HIGH confidence
- `C:/CodeProjects/drop-tracker/.planning/PROJECT.md` — v1.4 scope and "Out of scope" (Prometheus/metrics, charts, no new env var for retention); sqlc/migrate/chi constraints — HIGH confidence

---
*Stack research for: v1.4 Operator Observability (drop-tracker)*
*Researched: 2026-09-09*
