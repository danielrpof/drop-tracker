<!-- refreshed: 2026-08-12 -->
# Architecture

**Analysis Date:** 2026-08-12

## System Overview

```text
┌──────────────────────────────────────────────────────────────────┐
│                     HTTP API Layer                               │
│  chi Router + Request ID / Logging / Recovery Middleware         │
│  Handlers: /health, /search, /watchlist, /events                 │
│  `internal/httpserver/server.go`                                 │
└────────────────┬──────────────┬─────────────────┬────────────────┘
                 │              │                 │
        ┌────────▼───┐  ┌───────▼──────┐  ┌──────▼────────────┐
        │  Watchlist │  │  Search /    │  │  Events History  │
        │  Service   │  │  Add Artists │  │  Service         │
        │ `service.go`  │ `search.go`  │  │ `events/service` │
        └────────┬───┘  └───────┬──────┘  └──────┬────────────┘
                 │              │                 │
        ┌────────▼──────────────▼─────────────────▼────────────────┐
        │          Domain Seams (Narrow Interfaces)                │
        │  watchlist.Store, events.Store, httpserver.SearchSource  │
        └────────┬──────────────────────────────────────────────────┘
                 │
        ┌────────▼───────────────────────────────────────────────────┐
        │              External API Clients                          │
        │  MusicBrainz Client | Deezer Client                        │
        │  `internal/musicbrainz/client.go`                          │
        │  `internal/deezer/client.go`                               │
        │  Both rate-limited via golang.org/x/time/rate             │
        └────────┬───────────────────────────────────────────────────┘
                 │
┌────────────────▼──────────────────────────────────────────────────────┐
│                     Scheduled Polling Layer                           │
│  robfig/cron v3 scheduler + two independent poll cycles               │
│  MusicBrainz cycle: reads watchlist → calls ReleaseGroupsByArtist    │
│  Deezer cycle: reads watchlist → calls ArtistAlbums                   │
│  `internal/poller/poller.go`                                          │
└────────────────┬──────────────────────────────────────────────────────┘
                 │
        ┌────────▼────────────────────────────────────────────────────┐
        │              Event Detection Engine                         │
        │  Diffs fetched results against events table (seen store)    │
        │  Three pass types: new release, guest features, deluxe     │
        │  Records detected events via ON CONFLICT DO NOTHING        │
        │  `internal/detection/detector.go`                          │
        └────────┬────────────────────────────────────────────────────┘
                 │
        ┌────────▼────────────────────────────────────────────────────┐
        │              Notification Layer                             │
        │  Drains pending events from outbox (events table)           │
        │  Formats as Discord embeds, sends via webhook               │
        │  Marks notified_at after successful send                    │
        │  `internal/notifier/notifier.go`                            │
        └────────┬────────────────────────────────────────────────────┘
                 │
        ┌────────▼────────────────────────────────────────────────────┐
        │              Discord Webhook Client                         │
        │  Hand-rolled webhook POST client                            │
        │  Honors Discord's 5 req/2s rate limit                       │
        │  `internal/discord/client.go`                               │
        └────────┬────────────────────────────────────────────────────┘
                 │
        ┌────────▼────────────────────────────────────────────────────┐
        │              Data Access Layer                              │
        │  Postgres connection pool (pgx/v5)                          │
        │  sqlc-generated type-safe queries                           │
        │  `internal/db/pool.go`, `internal/db/sqlc`                  │
        └────────┬────────────────────────────────────────────────────┘
                 │
                 ▼
        ┌──────────────────────────────────────────────────────────┐
        │              Postgres Database                            │
        │  artists, watchlist, events tables                        │
        │  Schema migrations: internal/db/migrations               │
        └──────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| HTTP Server | Accept API requests, route to handlers, log correlate with request IDs | `internal/httpserver/server.go` |
| Watchlist Service | Add/list/update/remove artists from user watchlist, validate preferences | `internal/watchlist/service.go` |
| Search Handlers | Query MusicBrainz/Deezer via clients, return artist results | `internal/httpserver/search.go` |
| Events Service | Keyset-paginated read of events table, filter by artist/type | `internal/events/service.go` |
| Poller | Schedule cron jobs, run poll cycles independently per source | `internal/poller/poller.go` |
| MusicBrainz Client | Call MusicBrainz API endpoints (search, release groups, recordings, releases) | `internal/musicbrainz/client.go` |
| Deezer Client | Call Deezer API endpoints (search, albums) | `internal/deezer/client.go` |
| Detector | Diff poll results against seen events, record new releases/features/changes | `internal/detection/detector.go` |
| Notifier | Drain pending events, format as Discord embeds, send via webhook | `internal/notifier/notifier.go` |
| Discord Client | POST webhook embeds to Discord | `internal/discord/client.go` |
| Database Pool | Manage Postgres connections, enforce timeouts, detect stale sockets | `internal/db/pool.go` |
| Config | Parse environment variables, validate required settings | `internal/config/config.go` |
| Logging | Structured JSON/text logging with slog | `internal/logging/logging.go` |
| Web Assets | Serve embedded React SPA, fallback index.html for client-side routing | `internal/webassets/embed.go` |

## Pattern Overview

**Overall:** Single-process monolith with layered architecture, seam-based dependency injection for testability.

**Key Characteristics:**
- **Single binary**: HTTP API server and cron-based scheduler run in the same process
- **Seam-based design**: Every layer defines narrow interfaces rather than tight dependencies (e.g., `watchlist.Store`, `poller.ReleaseGroupSource`, `detection.RecordingSource`, `notifier.Sender`)
- **Stateless clients**: MusicBrainz and Deezer clients are thin wrappers around `net/http`, not stateful SDK objects
- **Rate limiting as a cross-cutting concern**: Both external API clients share a single process-wide `rate.Limiter` instance per source, so search traffic and poll traffic draw from the same budget
- **Idempotent event recording**: Events are recorded with `ON CONFLICT DO NOTHING` dedup keys, so any replay of a cycle produces the same schema state
- **Explicit guard semantics**: Poll cycles and notification passes use atomic boolean guards (compare-and-swap) to skip overlapping runs, never queue them—avoids compounding backlog behavior

## Layers

**HTTP API Layer:**
- Purpose: Accept user requests, serve SPA, expose artist search and watchlist/events endpoints
- Location: `internal/httpserver/`
- Contains: chi router setup, request handlers, middleware stack
- Depends on: watchlist.Store, events.Store, SearchSource (clients), logging
- Used by: External clients (browser, curl, etc.)

**Domain Services Layer:**
- Purpose: Encapsulate business logic for watchlist and event history
- Location: `internal/watchlist/`, `internal/events/`
- Contains: Service structs implementing Store/Reader interfaces, domain validation
- Depends on: sqlc.Querier, domain types/errors
- Used by: HTTP handlers, poller

**External API Clients:**
- Purpose: Call third-party APIs (MusicBrainz, Deezer) with rate limiting and user-agent identification
- Location: `internal/musicbrainz/`, `internal/deezer/`
- Contains: Client structs, endpoint-specific methods, request/response unmarshalling
- Depends on: stdlib `net/http`, `golang.org/x/time/rate`
- Used by: HTTP search handlers, poller cycles, detector passes

**Poller Layer:**
- Purpose: Schedule and run poll cycles on a cron interval, read watchlist, call API clients
- Location: `internal/poller/`
- Contains: Poller struct with MusicBrainz and Deezer cycle methods
- Depends on: watchlist.Store, ReleaseGroupSource (client), AlbumSource (client), EventRecorder, Notifier
- Used by: main.go at startup, runs independently per-source with atomic guards

**Detection Layer:**
- Purpose: Diff fetched API results against the events table (seen store), record new releases/features/deluxe changes
- Location: `internal/detection/`
- Contains: Detector struct with three detection passes (MusicBrainz, Deezer), event insertion logic
- Depends on: sqlc.Querier, RecordingSource, ReleaseDetailSource (both clients)
- Used by: Poller cycles after fetching results

**Notification Layer:**
- Purpose: Drain pending events from the outbox, send to Discord, mark as notified
- Location: `internal/notifier/`
- Contains: Notifier struct with NotifyPending method, Discord embed formatting
- Depends on: sqlc.Querier, Sender (Discord client)
- Used by: Poller cycles at end of each cycle

**Data Access Layer:**
- Purpose: Pool connections to Postgres, run migrations, provide type-safe query interface
- Location: `internal/db/`, `internal/db/sqlc/`
- Contains: Pool setup, migration runner, sqlc-generated queries and models
- Depends on: `github.com/jackc/pgx/v5`, `github.com/golang-migrate/migrate/v4`
- Used by: All services and domain logic

**Cross-Cutting:**
- Config: `internal/config/` — parsed from environment variables at startup
- Logging: `internal/logging/` — structured slog logger, JSON or text format
- Web Assets: `internal/webassets/` — embedded React SPA and SPA fallback handler

## Data Flow

### Primary Request Path (API → Watchlist CRUD)

1. User POST `/watchlist` with artist search result → `httpserver.handleAddWatchlist`
2. Handler calls `watchlist.Store.Add` → looks up artist in DB, inserts watchlist row
3. Returns Entry (artist + watchlist joined)

### Search Flow (HTTP Handler → MusicBrainz/Deezer)

1. User GET `/search?q=artist` → `httpserver.handleSearch`
2. Handler iterates `SearchSource` slice (MusicBrainz, Deezer), calls each `SearchArtists`
3. MusicBrainz client rate-limits via `mbLimiter`, calls MusicBrainz API, unmarshals results
4. Deezer client rate-limits via `dzLimiter`, calls Deezer API, unmarshals results
5. Handler merges results, returns to client

### MusicBrainz Poll Cycle

1. Cron tick at `cfg.PollInterval` → `poller.RunMusicBrainzCycle` (guarded by `mbRunning` atomic bool)
2. Cycle reads `watchlist.Store.List` → gets all artists with MBID
3. For each artist, calls `mbClient.ReleaseGroupsByArtist` (rate-limited)
4. Passes results to `detector.DetectMusicBrainz`:
   - Inserts new events for unseen release groups via `InsertEvent`
   - Calls `mbClient.RecordingsByArtist` for guest-feature pass, records guest feature events
   - Calls `mbClient.ReleasesByReleaseGroup` for deluxe-change pass (only seen groups), compares track counts
5. Cycle ends by calling `notifier.NotifyPending` with cycle's logger (correlates logs)
6. Notifier drains pending events, sends Discord embeds, marks `notified_at`

### Deezer Poll Cycle

1. Cron tick → `poller.RunDeezerCycle` (guarded by `dzRunning` atomic bool, independent from `mbRunning`)
2. Cycle reads `watchlist.Store.List` → filters to artists with non-nil `DeezerID`
3. For each artist, calls `dzClient.ArtistAlbums` with `deezerAlbumPageSize=50` (rate-limited)
4. Passes results to `detector.DetectDeezer`:
   - Inserts new events for unseen albums via `InsertEvent`
5. Cycle ends by calling `notifier.NotifyPending`

### Events History Flow (HTTP GET /events)

1. User GET `/events?artist_id=X&event_type=Y&cursor=Z&page_size=24` → `httpserver.handleListEvents`
2. Handler calls `events.Store.List` with ListParams
3. Service queries `ListEvents` with keyset pagination (cursor-based), respects page size clamp (24–100)
4. Returns Events page with NextCursor

**State Management:**
- Watchlist state: `watchlist` table (artist → user preferences)
- Seen state: `events` table with `(event_type, source, external_id)` dedup key — records all detected items once per source
- Notification outbox: Same `events` table, filtered by `notified_at IS NULL` for pending rows
- Per-source rate limiting: Two separate `rate.Limiter` instances (mbLimiter, dzLimiter) shared across all callers within the process — ensures search traffic and poll traffic draw from the same per-source budget
- Poll guard state: `mbRunning` and `dzRunning` atomic bools — independent so MusicBrainz tick delays never block Deezer tick

## Key Abstractions

**watchlist.Store:**
- Purpose: Narrow seam for watchlist CRUD, hides sqlc.Querier
- Examples: `internal/watchlist/service.go` (real), `internal/httpserver/watchlist_test.go` (stub)
- Pattern: Interface-based, implemented by Service struct wrapping sqlc.Queries

**events.Store:**
- Purpose: Narrow seam for event history read, hides sqlc.Querier
- Examples: `internal/events/service.go` (real)
- Pattern: Interface-based, implemented by Service struct wrapping sqlc.Queries

**poller.ReleaseGroupSource, poller.AlbumSource:**
- Purpose: Narrow seams for external API clients, allow stubs in poll cycle tests
- Examples: `internal/musicbrainz/client.go` (MusicBrainz, real), `internal/deezer/client.go` (Deezer, real)
- Pattern: Interfaces defined in consumer (poller), implemented by client packages

**detection.EventRecorder, detection.RecordingSource, detection.ReleaseDetailSource:**
- Purpose: Narrow seams for detection passes, allow MusicBrainz/Deezer stubs in tests
- Examples: `internal/detection/detector.go` (real detector), test doubles in detection_test.go
- Pattern: Interfaces declared in detector (consumer), implemented by clients or test stubs

**notifier.Sender:**
- Purpose: Narrow seam for Discord delivery, allow stub for notifier tests
- Examples: `internal/discord/client.go` (real), test double in notifier_test.go
- Pattern: Interface declared in notifier, implemented by discord.Client

**httpserver.Pinger, httpserver.SearchSource:**
- Purpose: Narrow seams for health check and search, hide pgxpool.Pool and client packages
- Examples: Pinger satisfied by pgxpool.Pool, SearchSource satisfied by MusicBrainz/Deezer clients
- Pattern: Interfaces narrow to only what handlers need, stub-friendly for tests

## Entry Points

**cmd/server/main.go:**
- Location: `cmd/server/main.go`
- Triggers: Process start (single entry point for the entire application)
- Responsibilities:
  1. Parse config from environment (fail fast if required settings missing)
  2. Initialize logger (JSON or text, based on LOG_FORMAT)
  3. Run schema migrations (fail fast if DB down or migrations error)
  4. Create connection pool with defensive timeouts
  5. Instantiate domain services (watchlist, events), API clients (MusicBrainz, Deezer), detector, notifier
  6. Wire all together and pass to httpserver.New and poller.New
  7. Start HTTP listener and poller (both in same process)
  8. Block until SIGTERM/SIGINT, then graceful shutdown: drain in-flight requests, stop poller, close pool
  9. Exit zero on success, non-zero on error

**HTTP Route Entry Points:**
- `GET /health` → `httpserver.handleHealth` — ping database for liveness
- `GET /search?q=...` → `httpserver.handleSearch` — query MusicBrainz/Deezer
- `POST /watchlist` → `httpserver.handleAddWatchlist` — add artist to watchlist
- `GET /watchlist` → `httpserver.handleListWatchlist` — list watchlist entries
- `PATCH /watchlist/{id}` → `httpserver.handleUpdateWatchlist` — update preferences
- `DELETE /watchlist/{id}` → `httpserver.handleRemoveWatchlist` — remove artist
- `GET /events` → `httpserver.handleListEvents` — fetch event history
- `*` (all other paths) → `webassets.Handler()` → serve embedded SPA, fallback to index.html

**Poll Cycle Entry Points:**
- `poller.RunMusicBrainzCycle` — called by cron at interval, reads watchlist, calls MusicBrainz
- `poller.RunDeezerCycle` — called by cron at interval, reads watchlist, calls Deezer

## Architectural Constraints

- **Single binary, single process**: No microservices, no separate frontend deploy. HTTP API and cron scheduler in the same process (cmd/server/main.go). React SPA built and embedded via `go:embed`, served as static assets from the Go binary.

- **Shared rate-limit budget per source**: One `rate.Limiter` per external API source (MusicBrainz, Deezer), shared across search traffic (HTTP handlers) and poll traffic (cron cycles). A burst of search requests draws from the same token bucket as poll requests, preventing one caller from overwhelming the shared pool.

- **Independent poll guards**: MusicBrainz and Deezer poll cycles use separate atomic boolean guards (`mbRunning`, `dzRunning`). A slow MusicBrainz cycle must never block or delay Deezer ticks, and vice versa. Both cycle types skip (not queue) overlapping runs via compare-and-swap.

- **Global sqlc.Queries instances**: The connection pool is shared; sqlc.Querier instances are created once per consumer (watchlist, events, detector, notifier) but all share the same underlying pool. No separate per-instance connection acquisition or release — all go through pool.Acquire (behind sqlc methods).

- **Idempotent event recording**: Events are inserted via `ON CONFLICT DO NOTHING` with a `(event_type, source, external_id)` dedup key. Replaying a cycle never changes the schema state — the same release checked twice still records only once.

- **Context cancellation model**: All long-lived operations (poll cycles, HTTP handlers, database queries) are context-aware. Shutdown triggers `signal.NotifyContext`, which cancels the poller's retained child context, causing in-flight API calls to unwind. Database queries have per-operation timeouts (e.g., notifier's `dbOpTimeout = 10s`) to prevent hung sockets from blocking forever.

- **No global state except the logger, pool, and clients**: Config is loaded at startup, logger is process-wide (created once), pool is single-instance, API clients are single-instance per source. All seams (watchlist.Store, events.Store, etc.) are dependency-injected so tests can substitute fakes without thread safety concerns.

## Anti-Patterns

### Nil Checks on Seam Interfaces

**What happens:** Code checks if a seam interface (e.g., `poller.Notifier`) is nil before calling a method.

**Why it's wrong:** Seam interfaces are always non-nil at runtime because the wiring in `cmd/server/main.go` always passes a non-nil value (a real notifier or `notifier.NoOp` for disabled case). Nil checks suggest a missing layer of indirection (the gate logic should live in a concrete type, not the caller).

**Do this instead:** Use the pattern in `cmd/server/main.go` and `notifier/notifier.go`: `notifier.Select` encapsulates the decision (empty webhook URL → `NoOp`, otherwise real notifier), and poller always receives a non-nil interface. Neither poll cycle method calls nil-check.

### Unbounded Database Operations Under signal.NotifyContext

**What happens:** A database query runs under the poller's signal-derived context (which has no deadline, only cancellation on shutdown) without a per-operation timeout. A hung socket (TCP-ESTABLISHED but never ACKs) blocks forever.

**Why it's wrong:** Signal-NotifyContext is not Done until shutdown. pgx only cancels via context.Done(), which never happens until the process is dying. A socket hanging at that point leaves the notifier draining goroutine blocked indefinitely, preventing graceful shutdown.

**Do this instead:** Wrap each logical database operation in its own context with a timeout (`context.WithTimeout`), as notifier does with `dbOpTimeout = 10s` per ListUnnotified call. Timeouts are per-operation, not per-pass, to allow a large backlog to legitimately take time to drain.

### Multiplexing Multiple API Clients to One rate.Limiter

**What happens:** MusicBrainz search traffic and poll traffic share one limiter, but Deezer gets a separate limiter (because Deezer's rate limit is higher per second).

**Why it's wrong:** The pattern is correct, but mixing multiple unrelated operations under a shared limiter can hide rate-limit violations if the math is wrong or if a new caller is added without updating the limiter budget.

**Do this instead:** Declare limiter construction close to where it's used (search and poll share mbLimiter because they're the only two MusicBrainz callers; dzLimiter is separate because Deezer's limit is different). Document the rate limit constant and the per-second equivalent in comments. If adding a new caller, ensure it draws from the correct shared limiter.

### Assuming CAS is a Sufficient Lock for Multi-Cycle Coordination

**What happens:** Two poll cycles (MusicBrainz and Deezer) each use a separate atomic bool guard, but a caller assumes they can coordinate across sources if needed (e.g., "wait until both are free").

**Why it's wrong:** Compare-and-swap is a non-blocking guard good for skipping (rejecting work), not coordinating across independent state machines. If cross-cycle coordination is needed later (e.g., distributed lock via Postgres advisory lock), per-cycle atomic bools won't help.

**Do this instead:** If coordination becomes necessary, use an external lock (Postgres advisory lock is a natural fit given the DB is already in use). Today, the independent guards are correct: each source is independent, and skipping a tick is always safe.

---

*Architecture analysis: 2026-08-12*
