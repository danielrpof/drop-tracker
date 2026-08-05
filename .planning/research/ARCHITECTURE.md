# Architecture Research

**Domain:** Single-binary Go service — HTTP API + cron poller + Discord notifier + embedded React SPA, backed by Postgres
**Researched:** 2026-08-04
**Confidence:** MEDIUM (community-consensus patterns from web research; no single canonical spec exists for this exact combination, but each sub-pattern — Go project layout, go:embed+Vite, chi routing, robfig/cron+HTTP coexistence, sqlc layout, multi-stage Docker, GitHub Actions supply-chain pipelines — is well-established and cross-corroborated across multiple independent sources)

## Standard Architecture

### System Overview

```
┌───────────────────────────────────────────────────────────────────────┐
│                         cmd/dropctl (main.go)                          │
│   thin entrypoint: load config → build deps → wire → run → shutdown    │
└───────────────────────────────┬─────────────────────────────────────┘
                                 │ constructs & starts
        ┌────────────────────────┼─────────────────────────┐
        ▼                        ▼                         ▼
┌───────────────┐      ┌──────────────────┐      ┌──────────────────────┐
│  HTTP Server    │      │  Cron Scheduler   │      │  (shared) Service     │
│  (chi.Mux)      │      │  (robfig/cron)     │      │  / Domain Layer       │
│                 │      │                    │      │                       │
│ - /api/watchlist│      │ - poll job(s) per  │      │ - WatchlistService    │
│   CRUD          │      │   source/interval  │      │ - PollService         │
│ - /api/search   │◄────▶│ - triggers         │◄────▶│ - DiffEngine          │
│   (MB/Deezer     │  uses │   PollService      │ uses │ - NotifierService     │
│   proxy)         │      │   for each watched │      │                       │
│ - /health        │      │   entry            │      │  all depend on        │
│ - embedded SPA    │      └──────────────────┘      │  interfaces, not      │
│   fileserver      │                                  │  concrete clients     │
└────────┬────────┘                                  └──────────┬────────────┘
         │                                                       │
         │ serves                                                │ calls (via interfaces)
         ▼                                                       ▼
┌────────────────┐                              ┌──────────────────────────────┐
│ embed.FS         │                              │  Adapters / Infra layer       │
│ (Vite dist/)      │                              │                               │
│ React SPA assets  │                              │ - MusicBrainzClient (http)    │
└────────────────┘                              │ - DeezerClient (http)          │
                                                    │ - DiscordNotifier (webhook)   │
                                                    │ - sqlc Queries (Postgres)     │
                                                    └───────────────┬──────────────┘
                                                                    ▼
                                                          ┌───────────────────┐
                                                          │ Postgres            │
                                                          │ (watchlist, seen    │
                                                          │  releases, etc.)    │
                                                          └───────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| `cmd/dropctl` (main) | Load config, construct all dependencies, wire HTTP server + cron scheduler on a shared context, run, handle graceful shutdown | Minimal `main.go`; no business logic |
| HTTP layer (chi) | Route registration, request/response marshaling, auth/middleware, delegates to service layer | `internal/api` or `internal/http`: `server.go`, `routes.go`, `handlers/*.go` |
| Cron scheduler (robfig/cron) | Ticks on a configured interval per source (MusicBrainz/Deezer), invokes the poll service for each watchlist entry | `internal/scheduler` or `internal/poller`: registers cron jobs, each job is a thin call into `PollService` |
| Service/domain layer | Business logic: watchlist CRUD rules, poll orchestration, diffing, notification triggering — owns no HTTP or cron concerns | `internal/service` or `internal/domain`: `watchlist.go`, `poll.go`, `diff.go`, `notify.go` — pure Go, interface-driven |
| External API clients | Real HTTP clients for MusicBrainz and Deezer, isolated behind interfaces so they can be swapped for `httptest.Server` fakes in tests | `internal/musicbrainz`, `internal/deezer` (or `internal/clients/{musicbrainz,deezer}`) |
| Notifier | Formats and posts Discord webhook messages | `internal/notify/discord.go` implementing a `Notifier` interface |
| Data store (sqlc) | Type-safe generated queries against Postgres; "seen" store and watchlist tables | `internal/db` or `internal/store`: generated `db.go`/`models.go`/`*.sql.go`, hand-written `.sql` query files in `db/queries/`, migrations in `db/migrations/` (golang-migrate) |
| Embedded frontend | React (Vite) SPA served as static assets from the same binary via `go:embed` | `web/` or `ui/` dir: Vite source + `dist/` build output + `embed.go` |
| Config | Env-var driven configuration (poll intervals, DB DSN, Discord webhook URL, port) | `internal/config`: struct + loader (envconfig/viper), validated at startup |

## Recommended Project Structure

```
drop-tracker/
├── cmd/
│   └── dropctl/                 # single binary entrypoint
│       └── main.go              # load config, wire deps, run, graceful shutdown
├── internal/
│   ├── config/                  # env-driven config struct + loader
│   ├── api/                     # HTTP layer (chi)
│   │   ├── server.go            # chi.Mux construction, middleware stack
│   │   ├── routes.go            # route registration/groups
│   │   ├── handlers/            # per-resource handlers (watchlist, search, health)
│   │   └── middleware/          # request logging, recover, etc. (or use chi/middleware)
│   ├── scheduler/                # robfig/cron wiring — registers poll jobs, owns Start/Stop
│   ├── service/                  # domain/business logic — the shared core
│   │   ├── watchlist.go          # watchlist CRUD rules
│   │   ├── poll.go               # orchestrates one poll cycle for one entry
│   │   ├── diff.go               # diff engine: compare fetched vs seen store
│   │   └── notify.go             # decides what to notify, calls Notifier interface
│   ├── musicbrainz/               # real HTTP client + interface, MusicBrainz-specific types
│   ├── deezer/                    # real HTTP client + interface, Deezer-specific types
│   ├── notify/
│   │   └── discord.go             # Discord webhook client implementing Notifier
│   ├── store/                     # sqlc-generated + hand-written repository glue
│   │   ├── sqlc/                  # GENERATED — db.go, models.go, *.sql.go (do not hand-edit)
│   │   └── repository.go          # thin wrapper exposing domain-shaped methods (optional)
│   └── logging/                    # slog setup helpers
├── db/
│   ├── migrations/                 # golang-migrate .up.sql/.down.sql files
│   └── queries/                    # .sql files sqlc compiles from
├── web/                             # React + Vite frontend source
│   ├── src/
│   ├── public/
│   ├── dist/                       # Vite build output (gitignored, generated at build time)
│   ├── embed.go                    # //go:embed dist  -> exposes embed.FS
│   ├── package.json
│   └── vite.config.ts
├── sqlc.yaml                        # sqlc config: queries + migrations in, store/sqlc out
├── Dockerfile                       # multi-stage: node build -> go build -> distroless/alpine runtime
├── docker-compose.yml                # app + Postgres for local dev
├── .github/workflows/pipeline.yml    # Full Pipeline CI/CD
├── .env.example
└── go.mod
```

### Structure Rationale

- **`cmd/dropctl/` (single cmd, not multiple):** Because this is explicitly one process/one binary (API + scheduler + notifier together), there is only one `cmd/` entrypoint — unlike a split-services layout that would have `cmd/api`, `cmd/worker`, etc. Keep `main.go` to wiring only; if it grows past ~100 lines of actual logic, that logic belongs in `internal/`.
- **`internal/service/` as the shared core:** This is the load-bearing boundary. Both `internal/api/handlers` (triggered by HTTP requests) and `internal/scheduler` (triggered by cron ticks) call into `internal/service` — neither the HTTP layer nor the scheduler contain business logic themselves. This is what makes poll/diff/notify independently testable without spinning up chi or cron.
- **`internal/musicbrainz/`, `internal/deezer/`, `internal/notify/` as adapters:** Each external integration lives in its own package behind a small interface (e.g., `type ReleaseSource interface { FetchArtist(ctx, mbid string) (Artist, error) }`). The service layer depends on the interface; production wiring in `main.go` supplies the real HTTP client, tests supply an `httptest.Server`-backed fake or a hand-rolled stub.
- **`internal/store/sqlc/` isolated from hand-written code:** sqlc output is regenerated wholesale on every `sqlc generate` run — keeping it in its own subpackage (vs. mixing into `internal/service`) makes "never hand-edit this" obvious and keeps diffs from codegen clean and reviewable in CI.
- **`db/migrations/` and `db/queries/` at repo root, not under `internal/`:** golang-migrate and sqlc both operate on plain `.sql` files that are naturally not Go packages; keeping them at the top level (sibling to `internal/`, `web/`) matches common sqlc+migrate project layouts and keeps the CI steps that invoke `sqlc generate` / `migrate` simple path-wise.
- **`web/` at repo root, not under `internal/`:** The frontend is a separate toolchain (Node/Vite) with its own `package.json`, so it should not live inside a Go `internal/` tree. `web/dist/` is the single hand-off point between the two toolchains — Vite writes to it, `web/embed.go` embeds it, and the Dockerfile's Go build stage depends on that directory being populated first.
- **`internal/config/`:** Small but worth its own package since every other component (API, scheduler, DB pool, Discord client) reads from it — centralizing env parsing/validation here means `main.go` does one `config.Load()` call and passes typed values down, rather than each component reading `os.Getenv` ad hoc.

## Architectural Patterns

### Pattern 1: Shared Service Layer (Handler/Scheduler → Service → Adapter)

**What:** A single `internal/service` package contains the domain logic (watchlist rules, poll orchestration, diffing, notify decisions). It is the *only* thing that both the HTTP handlers and the cron jobs call into. Handlers and cron jobs are both thin — they translate their respective triggers (HTTP request / cron tick) into a service call and translate the result back (HTTP response / log line).

**When to use:** Any time the same business operation can be triggered from more than one entry point — here, a poll cycle is triggered by cron on a schedule, but the same `PollService.PollOne(ctx, watchlistEntry)` should be callable directly from a test, and potentially later from a manual "poll now" API endpoint without duplicating logic.

**Trade-offs:** Slightly more boilerplate upfront (defining interfaces for `ReleaseSource`, `Notifier`, `Repository`) versus just wiring cron jobs directly against concrete clients. Pays off immediately in testability — `PollService` can be unit tested with `httptest.Server` fakes and an in-memory or test-DB repository, with zero dependency on chi or robfig/cron actually running.

**Example:**
```go
// internal/service/poll.go
type ReleaseSource interface {
    FetchArtist(ctx context.Context, externalID string) (Release, error)
}

type Notifier interface {
    NotifyNewRelease(ctx context.Context, r Release) error
}

type PollService struct {
    sources  map[string]ReleaseSource // "musicbrainz", "deezer"
    store    store.Repository
    notifier Notifier
    logger   *slog.Logger
}

func (p *PollService) PollOne(ctx context.Context, entry WatchlistEntry) error {
    fetched, err := p.sources[entry.Source].FetchArtist(ctx, entry.ExternalID)
    if err != nil {
        return fmt.Errorf("fetch %s: %w", entry.Source, err)
    }
    changes, err := p.store.Diff(ctx, entry.ID, fetched)
    if err != nil {
        return fmt.Errorf("diff: %w", err)
    }
    for _, c := range changes {
        if err := p.notifier.NotifyNewRelease(ctx, c); err != nil {
            p.logger.Error("notify failed", "err", err, "release", c.ID)
            continue // don't let one failed notification abort recording as seen
        }
    }
    return p.store.RecordSeen(ctx, entry.ID, fetched)
}
```

```go
// internal/scheduler/scheduler.go — thin: cron just calls the service
c := cron.New()
c.AddFunc(cfg.PollInterval, func() {
    ctx := context.Background()
    for _, entry := range watchlist.All(ctx) {
        if err := pollService.PollOne(ctx, entry); err != nil {
            logger.Error("poll failed", "entry", entry.ID, "err", err)
        }
    }
})
```

### Pattern 2: Interface-Boundary External Clients (Real HTTP Client + httptest.Server Fakes)

**What:** MusicBrainz and Deezer clients are real `net/http`-based clients implementing a shared `ReleaseSource`-style interface, living in their own packages (`internal/musicbrainz`, `internal/deezer`). Tests use `httptest.Server` to stand up a fake HTTP endpoint returning canned JSON, then point the real client at that test server's URL — exercising the actual HTTP/JSON-marshaling code path, not a hand-rolled mock.

**When to use:** Always, for any external API integration in this project — it's explicitly required by the project constraints (real clients, `httptest.Server` mocking, no live calls in CI).

**Trade-offs:** Slightly more test setup than a pure interface mock (need to construct a test server, register handlers per test case), but catches real bugs in URL building, header setting, JSON decoding, and error-status handling that a hand-rolled fake would hide.

**Example:**
```go
func TestMusicBrainzClient_FetchArtist(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(mbArtistResponse{ /* canned fixture */ })
    }))
    defer ts.Close()

    client := musicbrainz.NewClient(musicbrainz.WithBaseURL(ts.URL))
    release, err := client.FetchArtist(context.Background(), "some-mbid")
    // assert...
}
```

### Pattern 3: Dual-Mode Frontend Serving (embed.FS in prod, Vite proxy in dev)

**What:** The HTTP server serves the frontend in one of two modes, chosen at startup by an env var or build tag: (a) **production** — serve the embedded `web/dist` via `embed.FS` + `http.FileServer`, with an SPA fallback that serves `index.html` for any unmatched non-API, non-file route; (b) **development** — reverse-proxy all non-`/api/*` requests to the Vite dev server (`localhost:5173`), which handles hot module reload.

**When to use:** Needed as soon as you want a fast local frontend dev loop without rebuilding the Go binary on every UI change, while still shipping a single self-contained binary in production/CI.

**Trade-offs:** Adds a small amount of conditional logic in server setup (dev vs prod branch), and requires running two processes locally (`go run ./cmd/dropctl` + `npm run dev`, likely via `docker-compose` or a `Makefile`/`air` combo) instead of one. Worth it — the alternative (rebuild+restart Go binary on every CSS/JSX change) is a much worse dev loop.

**Example:**
```go
// internal/api/server.go
if cfg.Env == "dev" {
    proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: "localhost:5173"})
    r.NotFound(func(w http.ResponseWriter, req *http.Request) {
        if strings.HasPrefix(req.URL.Path, "/api/") {
            http.NotFound(w, req)
            return
        }
        proxy.ServeHTTP(w, req)
    })
} else {
    distFS, _ := fs.Sub(web.DistFS, "dist") // web.DistFS is the //go:embed dist var
    fileServer := http.FileServer(http.FS(distFS))
    r.NotFound(spaFallbackHandler(distFS, fileServer))
}
```

## Data Flow

### Poll → Diff → Notify → Record Cycle

```
[robfig/cron tick, per configured interval]
    ↓
[Scheduler] iterates watchlist entries (from Postgres via sqlc)
    ↓
For each entry:
[PollService.PollOne(entry)]
    ↓
[ReleaseSource.FetchArtist()] → real HTTP call to MusicBrainz or Deezer
    ↓ (Release data: current tracklist/releases/features as of now)
[DiffEngine] compares fetched data against "seen" store rows for this entry
    ↓
    ├─ no delta → [Repository.RecordSeen()] (update last-checked timestamp only) → done
    └─ delta found (new release / new feature / tracklist change)
         ↓
    [NotifierService] formats a Discord message per change
         ↓
    [DiscordNotifier.NotifyNewRelease()] → POST to Discord webhook URL
         ↓ (on success OR after logging failure — do not block recording)
    [Repository.RecordSeen()] persists the new state as "seen" so next cycle diffs against it
```

**Key design point — record-seen ordering:** Recording as "seen" should happen *after* the notify attempt, but a notify failure (Discord webhook down, rate-limited) should not prevent recording as seen — otherwise a persistently-failing notifier would cause the same change to be re-detected and re-attempted every cycle, and (worse) could cause the diff to compound if the external data itself changes again before the failure is resolved. Log notify failures loudly (structured `slog` fields with entry/release IDs) but let `RecordSeen` proceed. Consider a `notified_at` nullable column on the seen/change record if failed notifications need to be retried explicitly later — not required for v1, but avoid designs that make it hard to add later (i.e., write changes to a table row rather than only firing a side effect).

### HTTP Request Flow (CRUD / Search-Proxy)

```
[Browser: React SPA] → fetch('/api/watchlist') / fetch('/api/search?q=...')
    ↓
[chi router] → middleware chain (logging, recover) → route match
    ↓
[Handler] (internal/api/handlers/watchlist.go or search.go)
    ↓ decodes request, calls...
[WatchlistService] / direct call to [ReleaseSource.Search()] for search-proxy endpoints
    ↓
[Repository (sqlc Queries)] ←→ [Postgres]   (for CRUD)
    or
[MusicBrainzClient/DeezerClient] ←→ [external API]   (for search-proxy — live lookup, not DB-backed)
    ↓
[Handler] encodes JSON response
    ↓
[Browser]
```

Note the search-proxy endpoints are a **different data flow** from CRUD: they call external APIs live (not the DB) so the UI can look up artists to add to the watchlist. They should reuse the *same* `MusicBrainzClient`/`DeezerClient` the poller uses (one client implementation, two callers: scheduler for polling, HTTP handler for search) — this is another instance of the shared-adapter pattern, not a separate integration.

### Key Data Flows

1. **Poll cycle (background, cron-driven):** External API → Diff against Postgres seen-store → Discord notify → Record seen in Postgres. Runs unattended on a schedule; no user interaction.
2. **Watchlist management (foreground, HTTP-driven):** Browser SPA → chi API → Postgres CRUD. Standard REST-ish request/response.
3. **Search-proxy (foreground, HTTP-driven, no DB):** Browser SPA → chi API → live external API call → JSON response (results not persisted unless the user then adds one via the watchlist CRUD flow).
4. **Frontend asset serving (foreground, static):** Browser → chi API (or dev proxy) → embedded SPA assets (or Vite dev server in local dev).

## Scaling Considerations

This is a portfolio/single-user-to-small-group project; the honest scaling ceiling is "one Postgres instance, one small VPS, low request volume." Do not over-design for scale beyond that — the value here is in pipeline maturity, not horizontal scalability.

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Personal use / few users (v1 target) | Single binary, single Postgres instance, docker-compose locally then single VPS container. Cron poll interval (e.g. every 15–60 min) is the only real "load" concern, and it's tiny (a handful of watchlist entries × 2 external APIs). |
| Dozens of users / larger watchlist | Watch MusicBrainz/Deezer rate limits before anything else — add per-source request throttling/backoff in the client layer (already isolated behind the adapter interfaces, so this is a localized change). Add a DB connection pool size cap and index the seen-store on (watchlist_entry_id, checked_at). |
| Hundreds+ users / high poll frequency | This is well past this project's stated scope (single binary, no microservices, no k8s per PROJECT.md). If ever needed, the service-layer boundary already established makes it possible to peel the scheduler into its own process later (same `internal/service` code, new `cmd/worker` entrypoint) without a rewrite — but that is explicitly deferred/out of scope here. |

### Scaling Priorities

1. **First real constraint: external API rate limits, not compute.** MusicBrainz in particular has a documented ~1 req/sec courtesy limit. The `ReleaseSource` adapter interface is exactly where to add rate limiting/backoff (e.g. `golang.org/x/time/rate`) without touching the service or scheduler layers.
2. **Second: Postgres connection/query load.** Negligible at this project's scale; standard `pgxpool`/`database/sql` pooling with sane `MaxOpenConns` is more than sufficient. Not a near-term concern.

## Anti-Patterns

### Anti-Pattern 1: Business logic inside cron job closures or HTTP handlers

**What people do:** Write the fetch → diff → notify → record sequence directly inside the `cron.AddFunc` closure (or, symmetrically, inline complex logic directly inside an HTTP handler function).

**Why it's wrong:** Makes the poll logic untestable without actually running cron, and makes it impossible to trigger the same logic from a future "poll now" API endpoint without copy-pasting. It also tends to accumulate error handling and logging concerns tangled with business logic.

**Do this instead:** Cron jobs and HTTP handlers are both *callers* of `internal/service`. They translate their trigger into a service call and translate the result back into their respective output (log line vs HTTP response). Keep them under ~20 lines each.

### Anti-Pattern 2: sqlc-generated code imported directly by handlers/scheduler, bypassing the service layer

**What people do:** Call `store.Queries.GetWatchlistEntry(ctx, id)` directly from an HTTP handler or cron job, skipping the service layer "because it's just a getter."

**Why it's wrong:** Once a few of these creep in, business rules (validation, authorization-if-added-later, diff logic) end up split between handlers and services inconsistently, and it becomes hard to unit-test business rules without a real DB connection.

**Do this instead:** Handlers and the scheduler only ever call into `internal/service`; only `internal/service` (and its repository/store dependency) imports the sqlc package directly. This keeps one clear place to mock/fake for tests (fake the `Repository` interface, not the sqlc `Queries` struct).

### Anti-Pattern 3: Treating a failed Discord notification as a poll-cycle failure

**What people do:** Return an error from the poll job (and skip recording as "seen") whenever the Discord webhook call fails, on the theory that "the user should be notified so we shouldn't mark it done."

**Why it's wrong:** This causes the *same* detected change to be re-diffed and re-attempted every single poll cycle until Discord happens to succeed — and if the external API data shifts again in the meantime, the diff can produce confusing duplicate or compounding "changes." It conflates "did we detect the change" (a data-store concern) with "did we successfully alert about it" (a delivery concern).

**Do this instead:** Log notify failures with full context (`slog` structured fields) and optionally track notify success/failure per change row, but always record the underlying fetched state as seen once diffed, regardless of notify outcome. If retry-on-notify-failure matters later, build it as an explicit retry queue over the "seen" table's changes, not by refusing to advance the "seen" watermark.

### Anti-Pattern 4: Embedding `web/dist` without a `.gitignore` + gating the Go build on it existing

**What people do:** Commit the built `dist/` output to the repo, or (worse) let `go build`/`go run` silently succeed with a stale or empty `embed.FS` when the frontend hasn't been built yet, producing a binary that serves a blank page with no clear error.

**Why it's wrong:** Committed build output causes merge noise and drifts from source; a silently-stale embed makes local dev and CI failures confusing ("why is the UI blank?").

**Do this instead:** `.gitignore` `web/dist/`; add a placeholder/check (e.g., a `//go:build !embed_placeholder` guard, or simply a Makefile target `make build` that always runs `npm run build` before `go build`) so the Go build step fails loudly or is always preceded by a real frontend build, both locally and in the Dockerfile/CI.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| MusicBrainz API | Real HTTP client behind a `ReleaseSource`-style interface in `internal/musicbrainz`; used by both the poller (scheduled fetch) and the search-proxy handler (live lookup) | Has documented courtesy rate limits (~1 req/sec) and requires a descriptive `User-Agent` header — bake this into the client, not caller code. Mock via `httptest.Server` in tests. |
| Deezer API | Same pattern as MusicBrainz, separate package `internal/deezer`, own interface implementation | Different response shape/pagination than MusicBrainz — keep types package-local, don't force a shared DTO across both until the service layer's own domain type (`Release`) is what unifies them. |
| Discord Webhook | Simple outbound HTTP POST via `internal/notify/discord.go` implementing a `Notifier` interface | No auth beyond the webhook URL itself (treat as a secret, env-var only per project constraints); keep message formatting logic here, not in the service layer, so notification format can change independently of diff logic. |
| Postgres | sqlc-generated queries + `pgx` (or `database/sql` + `pgx` stdlib driver) connection pool | Migrations via golang-migrate, run as a startup step or separate CI/deploy step — decide explicitly whether `main.go` auto-migrates on boot (convenient for a single-binary/VPS deploy) or migrations are a separate explicit step (safer for anything beyond solo use). For this project's scale, auto-migrate-on-boot behind a `--migrate` flag or startup check is reasonable. |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `internal/api` (handlers) ↔ `internal/service` | Direct Go function calls (in-process) | Handlers depend on service interfaces, not concrete structs, so handler tests can fake the service layer if needed (though given the service layer itself is well-tested, handler tests can also just use the real service with a fake repository/client). |
| `internal/scheduler` ↔ `internal/service` | Direct Go function calls (in-process) | Same service dependency as the HTTP layer — this is the crux boundary that keeps poll logic decoupled from *how* it's triggered. |
| `internal/service` ↔ `internal/musicbrainz` / `internal/deezer` / `internal/notify` | Interface-typed dependency, injected at construction time in `main.go` | Production wiring supplies real clients; tests supply `httptest.Server`-backed clients or lightweight stubs implementing the same interface. |
| `internal/service` ↔ `internal/store` (sqlc) | Interface-typed `Repository` dependency (recommend wrapping raw sqlc `Queries` in a small repository type exposing domain-shaped methods, e.g. `Diff`, `RecordSeen`, `ListWatchlist`) | Keeps sqlc's generated, DB-shaped types from leaking directly into service/domain logic; makes it easier to fake the repository in service-layer unit tests without a real Postgres connection. |
| `internal/api` (static file serving) ↔ `web/dist` (embed.FS) | Compile-time embed (`//go:embed dist`) in prod; HTTP reverse proxy to Vite dev server in dev | The only place the frontend and backend "touch" at build time — see Pattern 3. |
| CI pipeline ↔ container registry | `docker/build-push-action` + `docker/login-action` against `ghcr.io`, using `GITHUB_TOKEN` (no extra registry secret) | See CI/CD job structure below. |

## CI/CD Job Structure ("Full Pipeline")

The "Full Pipeline" requirement (lint, test, Trivy scan, gitleaks, SBOM, semantic-release, ghcr.io push) maps naturally onto a **multi-job workflow with a fan-out/fan-in dependency graph**, not a single monolithic job — this gives parallelism (faster feedback on PRs) and lets each concern show up as its own PR status check.

```
┌─────────┐   ┌─────────┐   ┌──────────┐
│  lint    │   │  test    │   │ gitleaks  │     (parallel, no dependencies —
│ (golangci│   │ (go test │   │ (secret   │      run on every push/PR)
│ -lint,   │   │  + go    │   │  scan)    │
│ go vet)  │   │  vet)    │   │           │
└────┬────┘   └────┬────┘   └────┬─────┘
     └─────────────┼──────────────┘
                    ▼
          ┌───────────────────┐
          │  build-and-scan     │   (needs: lint, test, gitleaks)
          │  - docker build      │   builds the image once
          │  - Trivy image scan   │   (SARIF → code scanning)
          │  - SBOM generation     │   (Trivy/anchore, format github)
          └──────────┬────────┘
                      ▼
          ┌───────────────────┐
          │  release             │   (needs: build-and-scan;
          │  - semantic-release   │    only on push to main/tags)
          │  - compute next version,
          │    tag, GitHub release
          │  - retag + push image
          │    to ghcr.io (semver,
          │    major, latest tags)
          └───────────────────┘
```

**Rationale for this shape:**
- **lint / test / gitleaks as parallel, independent jobs:** They have no dependency on each other or on Docker, so running them concurrently minimizes PR feedback latency and lets each appear as its own required status check in branch protection rules.
- **Single `docker build` in `build-and-scan`, reused for scan and (conditionally) release:** Building the image once and scanning *that* artifact (rather than rebuilding in the release job) avoids scan/release drift — what gets scanned is exactly what gets pushed. Pass the image as a workflow artifact (`docker save`/`load`, or push to a job-scoped tag first) between jobs, or simply keep build+scan+push all in one job if avoiding the added complexity of image-artifact-passing is preferred at this project's scale (a fine simplification for a solo/portfolio project — the guidance above is the "textbook" separation; collapsing `build-and-scan` and `release` into one job is a reasonable pragmatic choice here).
- **`release` gated to `main`/tags only, and after scan passes:** Prevents pushing a version tag or an image to `ghcr.io` for a build that failed its security scan — semantic-release should not run (and thus no version/tag/release should be created) if `build-and-scan` failed.
- **semantic-release needs a Node runtime step even in a Go-only app:** semantic-release itself is a Node CLI; the release job needs a `setup-node` step regardless of the app's language. This is a good reason to also generate the SBOM/scan steps as their own job rather than interleaving Node setup into the Go build job.
- **Caching:** `actions/setup-go@v5` with `cache: true` (keyed on `go.sum`) covers the Go module cache for lint/test/build jobs; `actions/setup-node@v4` with `cache: 'npm'` (keyed on `web/package-lock.json`) covers the frontend's npm cache for the Docker build's Node stage if the frontend is also built directly in a CI job (e.g. for a separate frontend lint/test step) — inside the Dockerfile's own Node build stage, Docker layer caching (BuildKit cache mounts or `docker/build-push-action`'s `cache-from`/`cache-to`) is the relevant mechanism, not `actions/setup-node`, since that stage runs inside the Docker build, not the workflow's own Node environment. Do not cache `node_modules` directly — cache the npm/Vite/Go module *download* caches, which survive across dependency-version bumps more reliably.

## Build Order / Dependency Graph for Phased Delivery

Given the project's phased-delivery intent (per PROJECT.md), the natural build order — driven by what each component needs to exist and be testable before the next layer can be meaningfully built — is:

```
1. Postgres schema + migrations (golang-migrate) + sqlc config
        ↓ generates
2. sqlc-generated store layer (internal/store/sqlc)
        ↓ wrapped by
3. Repository layer (internal/store) — domain-shaped methods over sqlc
        ↓ used by
4. Service/domain layer (internal/service) — Watchlist CRUD logic first
   (can be fully unit-tested against repository fakes/real Postgres before HTTP exists)
        ↓ exposed via
5. HTTP API layer (chi) — Watchlist CRUD endpoints, /health
   (thin handlers calling into already-tested service layer)
        ↓ in parallel with 6/7 —
6. MusicBrainz + Deezer clients (internal/musicbrainz, internal/deezer)
   (independently buildable/testable with httptest.Server from day one;
    no dependency on 1-5)
        ↓ enables
7. Search-proxy HTTP endpoints (reuse clients from step 6, thin handlers)
        ↓ once 4 (service) + 6 (clients) both exist —
8. Diff engine + PollService (internal/service/poll.go, diff.go)
   (needs: repository from step 3, clients from step 6)
        ↓ enables
9. Discord notifier (internal/notify/discord.go) — small, independently
   buildable in parallel with 6-8, wired into PollService once both exist
        ↓ once 8 + 9 exist —
10. Cron scheduler (internal/scheduler) — thin wiring of PollService on a timer
        ↓ in parallel with 1-10, frontend track —
11. React (Vite) SPA — can start as early as step 5 exists (needs Watchlist
    CRUD + search-proxy API contracts to build against); local dev via Vite
    dev server + API proxy, independent of go:embed until near the end
        ↓ once 5 (or 5+7) and 11 are both reasonably stable —
12. go:embed wiring + dual-mode serving (Pattern 3) — ties frontend into
    the Go binary for the first time
        ↓ once the whole app runs end-to-end locally —
13. Multi-stage Dockerfile + docker-compose (packages 1-12 into one image)
        ↓
14. CI/CD "Full Pipeline" (lint/test/scan/gitleaks/SBOM/semantic-release/
    ghcr.io push) — wraps the whole repo; individual jobs (lint, unit test)
    can actually be scaffolded very early (even before step 4) since they
    only need *some* Go code to exist, but the full build-and-scan/release
    jobs depend on the Dockerfile from step 13 being real.
```

**Key implications for roadmap phase structure:**
- **Data layer (1-3) is the true foundation** — nothing else can be meaningfully tested end-to-end without it, though the external API clients (6) and lint/CI scaffolding (14, partial) can be built in parallel since they have no dependency on Postgres.
- **The service layer (4) should exist and be tested *before* the HTTP layer (5)** is built out in full, since handlers are meant to be thin wrappers — building HTTP-first tends to pull business logic into handlers by default (Anti-Pattern 1/2).
- **Poll/diff/notify (8-10) meaningfully depends on both the service+repository foundation (1-4) and the external clients (6)** — it is a natural "phase 2" once watchlist CRUD (the simpler, more foundational vertical slice) is working, matching a sensible MVP-first roadmap: ship CRUD + search first, then layer in the poller/diff/notify pipeline, then wire the frontend, then containerize/embed, then harden CI/CD.
- **go:embed integration (12) is deliberately late** — dev-mode (Vite dev server + API proxy) means the frontend can be built and iterated on well before the embed/dual-mode-serving code is written, so this shouldn't block frontend feature work.
- **CI/CD depth (14) can be introduced incrementally**: lint + unit test jobs can be scaffolded from the very first commit (cheap, high value, catches regressions immediately); Trivy/gitleaks/SBOM/semantic-release/ghcr.io push naturally arrive once there's a real Dockerfile (step 13) to build and scan — trying to stand up the full pipeline before there's an image to scan/push is not productive.

## Sources

- [golang-standards/project-layout](https://github.com/golang-standards/project-layout) — community-reference Go project layout (cmd/internal/pkg/web/api conventions) — MEDIUM confidence (community consensus repo, not an official Go team spec, but widely adopted)
- [Go Project Structure: Practices & Patterns (Rost Glukhov)](https://www.glukhov.org/app-architecture/code-architecture/go-project-structure/) — MEDIUM confidence
- [Embed Vite app in a Go Binary (Tushar Choudhari)](https://www.tushar.ch/writing/embed-vite-app-in-go-binary) — MEDIUM confidence
- [Embed Vite React in Golang Binary with Live Reload (dev.to)](https://dev.to/danhawkins/embed-vite-react-in-golang-binary-with-live-reload-1k4d) — MEDIUM confidence, cross-checked against the above
- [go:embed draft design doc (Go team)](https://go.googlesource.com/proposal/+/master/design/draft-embed.md) — HIGH confidence (official Go proposal)
- [go-chi/chi](https://github.com/go-chi/chi) — HIGH confidence (official repo/docs)
- [HTTP routing in Go services with chi (Stack Harbor)](https://stackharbor.com/en/knowledge-base/golang-chi-router-pattern/) — MEDIUM confidence
- [Developing and Running Cron Jobs with robfig/cron (Sling Academy)](https://www.slingacademy.com/article/developing-and-running-cron-jobs-with-robfig-cron-in-go/) — MEDIUM confidence
- [Creating a Job Scheduler in Golang with Cron, Graceful Shutdown, and SOLID Principles (Medium)](https://medium.com/@abbasmomeny1994/creating-a-job-scheduler-in-golang-with-cron-graceful-shutdown-and-solid-principles-2a2820078efb) — MEDIUM confidence
- [Using SQLC for ORM alternative in Golang, ft. Go-Migrate & PGX (Gravel Engineering)](https://medium.com/gravel-engineering/using-sqlc-for-orm-alternative-in-golang-ft-go-migrate-pgx-b9e35ec623b2) — MEDIUM confidence
- [sqlc: Type-Safe Querying in Go (dev.to)](https://dev.to/leapcell/sqlc-type-safe-querying-in-go-4a47) — MEDIUM confidence
- [How to implement Clean Architecture in Go / Three Dots Labs](https://threedots.tech/post/introducing-clean-architecture/) — MEDIUM confidence, corroborates service/handler/repository layering
- [Multi-Stage Docker Builds for Fullstack React + Node Apps (dev.to)](https://dev.to/devforgedev/multi-stage-docker-builds-for-fullstack-react-node-apps-1m02) — MEDIUM confidence
- [Docker Build Process — icereed/paperless-gpt (DeepWiki)](https://deepwiki.com/icereed/paperless-gpt/4.1-docker-build-process) — MEDIUM confidence, concrete example of a Node-build-stage → Go-embed pattern
- [aquasecurity/trivy-action](https://github.com/aquasecurity/trivy-action) — HIGH confidence (official Trivy GitHub Action)
- [Automating Docker Image Versioning, Build, Push, and Scanning Using GitHub Actions (dev.to)](https://dev.to/msrabon/automating-docker-image-versioning-build-push-and-scanning-using-github-actions-388n) — MEDIUM confidence
- [Semantic release to npm and/or ghcr without any tooling (dev.to / OpenSauced)](https://dev.to/opensauced/semantic-release-to-npm-andor-ghcr-without-any-tooling-5730) — MEDIUM confidence
- [Dependency caching reference (GitHub Docs)](https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching) — HIGH confidence (official GitHub documentation)
- [Better GitHub Actions caching for Go (Dan Peterson)](https://danp.net/posts/github-actions-go-cache/) — MEDIUM confidence

---
*Architecture research for: Go single-binary service (chi API + robfig/cron poller + Discord notifier + embedded React/Vite SPA + Postgres/sqlc)*
*Researched: 2026-08-04*
