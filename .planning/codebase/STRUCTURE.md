# Codebase Structure

**Analysis Date:** 2026-08-12

## Directory Layout

```
drop-tracker/
├── cmd/
│   └── server/
│       ├── main.go                          # Single entry point: boot, config, wiring, graceful shutdown
│       └── [*_test.go files]                # Integration tests via e2e boot patterns
│
├── internal/                                # All non-exported Go packages
│   ├── config/
│   │   ├── config.go                        # Environment variable parsing via caarlos0/env
│   │   └── config_test.go
│   │
│   ├── db/
│   │   ├── pool.go                          # Postgres pgxpool setup, connection timeouts
│   │   ├── migrate.go                       # golang-migrate runner (runs before startup)
│   │   ├── migrations/                      # SQL migration files (000001_init, etc.)
│   │   │   ├── *.up.sql                     # Schema creation/alteration
│   │   │   └── *.down.sql                   # Schema rollback
│   │   ├── sqlc/                            # sqlc-generated code (type-safe queries)
│   │   │   ├── db.go                        # Querier interface wrapper
│   │   │   ├── models.go                    # Generated SQL row struct types
│   │   │   ├── artists.sql.go               # Artist queries
│   │   │   ├── watchlist.sql.go             # Watchlist queries
│   │   │   ├── events.sql.go                # Events queries
│   │   │   ├── health.sql.go                # Health check query
│   │   │   └── querier.go                   # Querier interface definition
│   │   ├── [*_test.go files]
│   │   └── [*.sql.go files have no .go source - they're generated from queries/]
│   │
│   ├── watchlist/
│   │   ├── service.go                       # Watchlist Store impl: Add/List/Update/Remove artists
│   │   ├── normalize_test.go                # Helper tests for pref validation
│   │   └── service_test.go
│   │
│   ├── events/
│   │   ├── service.go                       # Events Store impl: List with pagination/filtering
│   │   └── events_test.go
│   │
│   ├── httpserver/
│   │   ├── server.go                        # chi router wiring, middleware stack
│   │   ├── health.go                        # GET /health handler
│   │   ├── search.go                        # GET /search handler
│   │   ├── watchlist.go                     # POST/GET/PATCH/DELETE /watchlist handlers
│   │   ├── events.go                        # GET /events handler
│   │   ├── spa_test.go                      # Tests for SPA 404 fallback
│   │   ├── boot_e2e_test.go                 # Full e2e HTTP tests
│   │   └── [*_test.go files]
│   │
│   ├── poller/
│   │   ├── poller.go                        # Cron scheduler, poll cycle orchestration
│   │   └── poller_test.go
│   │
│   ├── musicbrainz/
│   │   ├── client.go                        # MusicBrainz API client, rate limiter binding
│   │   ├── search.go                        # SearchArtists endpoint
│   │   ├── releasegroups.go                 # ReleaseGroupsByArtist endpoint
│   │   ├── recordings.go                    # RecordingsByArtist endpoint
│   │   ├── releases.go                      # ReleasesByReleaseGroup endpoint
│   │   └── [*_test.go files]                # HTTP mocking via httptest.Server
│   │
│   ├── deezer/
│   │   ├── client.go                        # Deezer API client, rate limiter binding
│   │   ├── search.go                        # SearchArtists endpoint
│   │   ├── albums.go                        # ArtistAlbums endpoint
│   │   └── [*_test.go files]
│   │
│   ├── detection/
│   │   ├── detector.go                      # Event detection: new release, features, deluxe changes
│   │   ├── musicbrainz.go                   # MusicBrainz-specific detection passes
│   │   ├── deezer.go                        # Deezer-specific detection passes
│   │   ├── filter.go                        # Preference-based event filtering
│   │   └── [*_test.go files]
│   │
│   ├── notifier/
│   │   ├── notifier.go                      # Drain events outbox, guard against concurrent runs
│   │   ├── format.go                        # Format events as Discord embeds
│   │   └── [*_test.go files]
│   │
│   ├── discord/
│   │   ├── client.go                        # Discord webhook POST client
│   │   └── [*_test.go files]                # 429 retry logic, rate limit tests
│   │
│   ├── logging/
│   │   └── logging.go                       # Structured slog setup (JSON or text)
│   │
│   ├── webassets/
│   │   ├── embed.go                         # go:embed for React SPA, NotFound fallback
│   │   └── build/client/                    # Embedded SPA build output
│   │       ├── index.html                   # SPA entry point (fallback destination)
│   │       ├── assets/                      # Vite-emitted JavaScript/CSS bundles
│   │       └── [other static assets]
│   │
│   └── testutil/
│       └── [test helpers, mocks, fixtures]
│
├── web/                                     # React + Vite frontend (built to internal/webassets/build/client/)
│   ├── app/
│   │   ├── routes/                          # React Router v7 route definitions
│   │   │   ├── watchlist.tsx                # Watchlist page
│   │   │   ├── history.tsx                  # Event history page
│   │   │   └── [other route pages]
│   │   ├── components/
│   │   │   ├── watchlist/                   # Watchlist-specific UI components
│   │   │   ├── history/                     # History-specific UI components
│   │   │   ├── common/                      # Shared components (header, nav, etc.)
│   │   │   └── ui/                          # Base UI primitives (buttons, inputs, modals)
│   │   ├── lib/                             # Frontend utilities, API client wrappers
│   │   ├── app.tsx                          # Root app component
│   │   └── main.tsx                         # Vite entry point
│   │
│   ├── public/                              # Static assets (favicons, etc.) not processed by Vite
│   ├── build/                               # Vite output (gitignored, generated by `make web`)
│   │   └── client/                          # Copied here by Makefile before Go build
│   │
│   ├── package.json                         # pnpm dependencies and build scripts
│   ├── pnpm-lock.yaml                       # Lockfile for reproducible builds
│   ├── pnpm-workspace.yaml                  # pnpm workspaces config (if monorepo)
│   ├── vite.config.ts                       # Vite build configuration
│   ├── react-router.config.ts               # React Router v7 config
│   ├── tsconfig.json                        # TypeScript configuration
│   ├── components.json                      # shadcn/ui configuration (if used)
│   └── README.md                            # Frontend docs
│
├── queries/                                 # Raw SQL files for sqlc codegen
│   ├── artists.sql                          # Artist queries
│   ├── watchlist.sql                        # Watchlist queries
│   ├── events.sql                           # Events queries
│   └── [other .sql files]
│
├── .github/
│   └── workflows/
│       ├── ci.yml                           # Go test, lint, scan, build pipeline
│       └── [other CI/CD workflows]
│
├── .planning/                               # GSD planning artifacts
│   ├── codebase/                            # This directory (ARCHITECTURE.md, STRUCTURE.md, etc.)
│   └── [other phase docs]
│
├── Makefile                                 # Build targets: go build, web, migrations, etc.
├── Dockerfile                               # Multi-stage build: Go + embedded SPA → single binary
├── go.mod / go.sum                          # Go module declaration and dependency lock
├── .gitignore                               # Excludes Go build artifacts, node_modules, env files
├── .env.example                             # Environment variable template (secrets NOT here)
├── README.md                                # Project overview
└── PROJECT.md                               # Project constraints and decisions
```

## Directory Purposes

**cmd/server/:**
- Purpose: Single entry point for the entire application
- Contains: `main.go` with boot sequence (config → logging → migrations → pool → wiring → graceful shutdown)
- Key pattern: Calls into `internal/` packages in a specific order, ensures no step is skipped on error

**internal/config/:**
- Purpose: Parse and validate environment variables at startup
- Contains: Config struct with environment tags, Load() function
- Used by: main.go first thing, before any other initialization

**internal/db/:**
- Purpose: Database connectivity and schema management
- Contains: Pool setup with defensive timeouts, migration runner, sqlc-generated queries
- Key insight: Pool is single-instance, shared across all services; migrations run before listener starts

**internal/watchlist/:**
- Purpose: Domain logic for artist watchlist management
- Contains: Service implementing Store interface, CRUD methods, preference validation
- Key pattern: narrow Store interface so tests can stub without a DB

**internal/events/:**
- Purpose: Domain logic for event history retrieval
- Contains: Service implementing Store interface, keyset-paginated List method
- Key pattern: mirrors watchlist/service.go's structure (Store interface, Service impl)

**internal/httpserver/:**
- Purpose: HTTP API layer with chi router and request handlers
- Contains: Router setup, middleware stack, all endpoint handlers
- Key pattern: Every handler depends on narrow interfaces (Store, SearchSource, Pinger) not concrete types

**internal/poller/:**
- Purpose: Scheduled polling via cron
- Contains: Poller struct, MusicBrainz and Deezer cycle methods
- Key pattern: Independent atomic guards per source, cycles skip (not queue) overlapping runs

**internal/musicbrainz/:**
- Purpose: MusicBrainz API client
- Contains: Client with methods for each endpoint (Search, ReleaseGroups, Recordings, Releases)
- Key pattern: Rate limiter injected at construction, User-Agent set on every request

**internal/deezer/:**
- Purpose: Deezer API client
- Contains: Client with methods for each endpoint (Search, ArtistAlbums)
- Key pattern: Rate limiter injected at construction, no User-Agent requirement

**internal/detection/:**
- Purpose: Event detection engine (diff API results against seen store)
- Contains: Detector with three detection passes (MusicBrainz, Deezer), InsertEvent logic
- Key pattern: Uses ON CONFLICT DO NOTHING for idempotency, separate pass logic per source

**internal/notifier/:**
- Purpose: Drain pending events and send Discord notifications
- Contains: Notifier with NotifyPending method, per-send spacing control, guard against concurrent runs
- Key pattern: Sender interface allows Discord stub in tests

**internal/discord/:**
- Purpose: Discord webhook client
- Contains: Client with Send method, Embed type definitions
- Key pattern: Hand-rolled webhook POST client, no Discord SDK dependency

**internal/logging/:**
- Purpose: Structured logging setup
- Contains: New() function that creates slog logger (JSON or text format)
- Used by: main.go at startup, passed to all components

**internal/webassets/:**
- Purpose: Serve embedded React SPA
- Contains: go:embed directive for React build output, Handler() returns http.Handler with index.html fallback
- Key insight: Vite build output is copied here by Makefile, embedded at compile time

**web/:**
- Purpose: React + Vite frontend source (separate from Go binary)
- Contains: React Router pages, UI components, API client wrappers
- Key pattern: Builds to `build/client/`, then copied to `internal/webassets/build/client/` for embedding

**queries/:**
- Purpose: Raw SQL files for sqlc code generation
- Contains: *.sql files with named SQL statements (-- name: StatementName :many, etc.)
- Key pattern: sqlc parses these at dev time, generates `internal/db/sqlc/*.sql.go` (checked into git)

## Key File Locations

**Entry Points:**
- `cmd/server/main.go`: Single Go entry point, orchestrates boot sequence
- `web/app/main.tsx`: React/Vite entry point, imports App component
- `internal/webassets/embed.go`: SPA static serve, chi NotFound handler

**Configuration:**
- `internal/config/config.go`: Environment variable parsing (no config files, env-only)
- `.env.example`: Template for required env vars (secrets NOT checked in)
- `PROJECT.md`: Architectural decisions and constraints
- `CLAUDE.md`: Implementation guidelines and tech stack decisions

**Core Logic:**
- `cmd/server/main.go`: Boot, wiring, graceful shutdown
- `internal/httpserver/server.go`: chi router setup
- `internal/httpserver/{health,search,watchlist,events}.go`: Endpoint handlers
- `internal/poller/poller.go`: Cron scheduler and poll cycles
- `internal/detection/detector.go`: Event diff and recording
- `internal/notifier/notifier.go`: Discord notification outbox drain

**Database:**
- `internal/db/pool.go`: Postgres pool, connection limits
- `internal/db/migrate.go`: Schema migration runner
- `internal/db/migrations/*.up.sql`: Schema definitions
- `queries/*.sql`: Raw SQL for sqlc codegen
- `internal/db/sqlc/*.sql.go`: Generated type-safe queries

**External APIs:**
- `internal/musicbrainz/client.go`: MusicBrainz client, rate limiter
- `internal/deezer/client.go`: Deezer client, rate limiter
- `internal/discord/client.go`: Discord webhook client

**Testing:**
- `internal/httpserver/*_test.go`: HTTP handler tests
- `internal/watchlist/service_test.go`: Watchlist service tests
- `internal/poller/poller_test.go`: Poller cycle tests
- `internal/musicbrainz/*_test.go`: MusicBrainz client tests (httptest.Server)
- `internal/deezer/*_test.go`: Deezer client tests (httptest.Server)

## Naming Conventions

**Files:**
- Go source: `lowercase_with_underscores.go` (e.g., `pool.go`, `migrate.go`, `watchlist.go`)
- Go tests: `{name}_test.go` (e.g., `pool_test.go`)
- SQL migrations: `{sequence}_{slug}.{up|down}.sql` (e.g., `000001_init.up.sql`, `000002_watchlist.down.sql`)
- Raw SQL for sqlc: `{resource_name}.sql` (e.g., `artists.sql`, `watchlist.sql`)
- React routes: `{page_name}.tsx` (e.g., `watchlist.tsx`, `history.tsx`)
- React components: PascalCase (e.g., `WatchlistCard.tsx`, `EventList.tsx`)

**Directories:**
- Internal packages: lowercase, single-concept names (e.g., `watchlist`, `events`, `httpserver`, `poller`)
- Package organization: By domain/concern (e.g., `httpserver` for all HTTP, `detection` for all diff logic)
- Tests: Alongside source in same directory (co-located pattern: `service.go` and `service_test.go` in same dir)

**Go Code:**
- Interfaces: Narrow, named after the behavior (e.g., `watchlist.Store`, `poller.ReleaseGroupSource`, `httpserver.Pinger`)
- Types: Descriptive names matching their role (e.g., `Detector`, `Poller`, `Entry`, `Event`)
- Constants: UPPER_CASE for package-level, lower_case within functions
- Functions: camelCase (e.g., `NewService`, `List`, `RunMusicBrainzCycle`)
- Struct fields: camelCase, unexported by default (only exported if part of API)

**Environment Variables:**
- Format: UPPER_SNAKE_CASE (e.g., `DATABASE_URL`, `HTTP_PORT`, `DISCORD_WEBHOOK_URL`, `MUSICBRAINZ_USER_AGENT`)
- Required vars: Defined in `config.struct` with `notEmpty` tag
- Optional vars: Defined with `envDefault:"value"` tag
- Secrets: Never committed; only env var names documented in `.env.example`

## Where to Add New Code

**New Feature (e.g., artist stats endpoint):**
- Handler: `internal/httpserver/{feature_name}.go` with test file alongside
- Domain: `internal/{feature_name}/service.go` if it's a cross-cutting concern, else extend existing service
- DB: Add queries to `queries/{table}.sql`, regenerate via `sqlc generate`, commit changes to `internal/db/sqlc/*.sql.go`
- Tests: `internal/{package}/{name}_test.go` co-located with source

**New API Client (e.g., Genius.com artist lookup):**
- Location: `internal/{api_name}/` (e.g., `internal/genius/`)
- Structure: Follow `internal/musicbrainz/` pattern: `client.go` for base client, endpoint files (e.g., `search.go`, `lyrics.go`), test files with httptest.Server stubs
- Wiring: Pass client instance to httpserver.New and/or poller.New depending on usage (search vs. poll)
- Rate limiting: Create a separate `rate.Limiter` per API in main.go, pass to NewClient

**New Poll Cycle (e.g., YouTube)::**
- Location: Extend `internal/poller/poller.go` with a new cycle method (e.g., `RunYoutubeCycle`)
- Pattern: Create separate `ytRunning atomic.Bool` guard; use `fmt.Sprintf("youtube-%d", nextCycleID.Add(1))` for cycle id
- Register cron: Add `p.cron.AddFunc(spec, func() { ... p.RunYoutubeCycle(...) })` in New
- Client: Create `internal/youtube/client.go` with rate limiter, pass instance to Poller constructor
- Detection: Extend `internal/detection/detector.go` with `DetectYoutube` method, call from cycle
- Wiring: Update `cmd/server/main.go` to create YouTube client/limiter and pass to poller.New

**New Notification Destination (e.g., Slack):**
- Location: `internal/{service_name}/` (e.g., `internal/slack/`)
- Structure: Implement `notifier.Sender` interface (Send method accepting discord.Embed... or create adapter)
- Wiring: Extend `notifier.Select` in `internal/notifier/notifier.go` to return Slack client when SLACK_WEBHOOK_URL is set
- Rate limiting: Discord webhook limit is 5 req/2s; adjust `defaultSpacing` if Slack is faster/slower, or create separate notifier instances

**Database Schema Change:**
- Create migration: `internal/db/migrations/{next_sequence}_{description}.{up|down}.sql`
- Run migrations: `make migrate` (locally) or part of CI before deployment
- Update queries: Add/modify statements in `queries/*.sql`
- Regenerate: `sqlc generate`, commit the new `internal/db/sqlc/*.sql.go` files
- Update models: Generated via sqlc; no manual struct definitions needed

**React Component:**
- Location: `web/app/components/{domain}/` (e.g., `web/app/components/watchlist/WatchlistCard.tsx`)
- Pattern: Use existing UI primitives from `web/app/components/ui/`
- Styling: CSS Modules or Tailwind classes (follow existing pattern)
- Tests: Collocate test file (`.test.tsx`) or use `web/app/__tests__/` directory

**Test Utilities:**
- Location: `internal/testutil/` for reusable test helpers
- Examples: Stub implementations of seam interfaces, fixtures for common test data
- Pattern: Helpers prefixed with "Stub" or "Fake" (e.g., `StubWatchlistStore`, `FakeMusicBrainzClient`)

## Special Directories

**internal/webassets/build/client/:**
- Purpose: Embedded React SPA build output
- Generated: Yes (built via `npm run build` in web/, copied by Makefile)
- Committed: No (gitignored; `build/client/` is copied fresh from `web/build/` before Go build)
- Key: Vite emits hashed bundles (index-abc123.js); embedded filesystem serves them alongside index.html fallback

**internal/db/migrations/:**
- Purpose: SQL schema versioning and migration files
- Generated: No (hand-written SQL)
- Committed: Yes (all migrations checked in for reproducible deploys)
- Key: Numbered sequence (000001, 000002, etc.), never edited once committed (create new files for changes)

**queries/:**
- Purpose: Raw SQL for sqlc code generation
- Generated: No (hand-written SQL with sqlc directives)
- Committed: Yes (source of truth for type-safe generated code)
- Key: Each query needs a `-- name: StatementName :many/:one/:exec` directive

**internal/db/sqlc/:**
- Purpose: Generated type-safe query code
- Generated: Yes (output of `sqlc generate` from queries/)
- Committed: Yes (checked in so developers don't need sqlc CLI locally)
- Key: Never edit manually; regenerate by running `sqlc generate`

**web/build/:**
- Purpose: Vite build output (JavaScript, CSS, assets)
- Generated: Yes (output of `npm run build`)
- Committed: No (gitignored; rebuilt every deploy)
- Note: Output is copied to `internal/webassets/build/client/` by Makefile before Go build

---

*Structure analysis: 2026-08-12*
