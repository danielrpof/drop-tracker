# Phase 18: Backend — Readiness, Status Surface & App Version - Pattern Map

**Mapped:** 2026-09-09
**Files analyzed:** 16 (7 new, 9 modified)
**Analogs found:** 16 / 16 (all in-repo; the phase is almost entirely composition of existing patterns)

All analog paths below are git-tracked source (verified: `internal/`, `queries/`, `cmd/`,
`Dockerfile`, `.github/workflows/` all tracked; there is no gitignored mirror tree in this repo —
`internal/webassets/build/client/` is the only committed-generated dir and is not an analog here).

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| NEW `internal/httpserver/ready.go` | controller (HTTP handler) | request-response (probe) | `internal/httpserver/health.go` | exact |
| NEW `internal/httpserver/status.go` | controller (HTTP handler) | request-response (read aggregate) | `internal/httpserver/events.go` (`handleListEvents`) | role-match (read, no query params) |
| NEW `internal/pollruns/pollruns.go` | store (in-process) | event-driven append + snapshot read | `internal/notifier/notifier.go` (`NoOp`/`Select`) + `internal/events/service.go` (`Store` seam) | role-match |
| NEW `internal/buildinfo/buildinfo.go` | config/constant (link-time var) | n/a (build-time fact) | `internal/httpclient/httpclient.go` (single-purpose pkg layout) | layout-only |
| MOD `internal/db/migrate.go` | model / DB helper | file-I/O (embed FS walk) + request-response (pgx row read) | existing `maxSourceVersion` walk + ahead-of-source guard `m.Version()` read (`migrate.go:298-302,320-341`) | exact (same file) |
| MOD `internal/httpserver/server.go` | route wiring / DI | request-response | existing `Pinger` seam + `Option`/`serverConfig` + `WithAuthGate` + `/health` registration split (`server.go:20-51,66,161-204`) | exact (same file) |
| MOD `internal/poller/poller.go` | service seam declaration | event-driven | `Notifier` interface + `WithMusicBrainzWorkers` option + non-nil default (`poller.go:99-123,178-198`) | exact (same file) |
| MOD `internal/poller/poller_test.go` | test double | event-driven | `fakeEventRecorder` / `fakeNotifier` (`poller_test.go:179-241`) | exact (same file) |
| MOD `queries/watchlist.sql` | query (sqlc) | CRUD (count) | `queries/health.sql` (`Ping :one`) + `queries/events.sql` `HasAnyEvent :one` shape | exact |
| MOD `cmd/server/main.go` | composition root | wiring | existing `notifier.Select` / `eventsStore` / `poller.New` wiring block (`main.go:215-262`) | exact (same file) |
| MOD `Dockerfile` | build config | build-time | builder stage `go build -trimpath -ldflags="-w -s"` (`Dockerfile:60-61`) + top comment (`:15-19`) | exact (same file) |
| MOD `.github/workflows/full-pipeline.yml` | CI config | build-time | `build-scan` job `docker/build-push-action` step (`:576-583`) | exact (same file) |
| NEW `internal/httpserver/ready_test.go` | test | request-response | `health_test.go` (`stubPinger`, leak loop, `TestHealth_DownOnTimeout`) | exact |
| NEW `internal/httpserver/status_test.go` | test | request-response | `events_test.go` (`stubEventsStore`) + `server_test.go` (`newGatedServer`) | exact |
| NEW `internal/pollruns/pollruns_test.go` | test | event-driven concurrency | `poller_test.go` `fakeReleaseGroupSource` in-flight/CAS counting (`poller_test.go:150-177`) | role-match |
| NEW `internal/db/schema_version_test.go` | test | file-I/O + integration | `internal/db/migrate_ahead_test.go` / `migrate_test.go` (embed FS + `testutil.NewTestPool`) | role-match |
| NEW `internal/buildinfo/buildinfo_test.go` | test | n/a | `internal/httpclient/httpclient_test.go` (tiny pkg test) | layout-only |

---

## Shared Patterns

### Leak-free error handling (applies to `ready.go`, `status.go`, `pollruns.go`)

**Source:** `internal/httpserver/health.go:35-39` and `internal/httpserver/events.go:124-128`

Raw cause goes to the request-scoped structured log only; the response body is fixed fields.

```go
// health.go:35-39
if err := s.db.Ping(ctx); err != nil {
    resp.Status, resp.DB = "degraded", "down"
    status = http.StatusServiceUnavailable
    httplog.SetAttrs(r.Context(), slog.String("db_error", err.Error()))
}
```

```go
// events.go:124-127 — the 500 path status.go copies for the watchlist-count failure
if err != nil {
    httplog.SetAttrs(r.Context(), slog.String("events_error", err.Error()))
    writeError(w, http.StatusInternalServerError, "internal error")
    return
}
```

`writeError` is the shared helper — **`internal/httpserver/watchlist.go:62-66`**:
```go
func writeError(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(errorResponse{Error: msg})
}
```

Idiom: attr key is `<endpoint>_<subject>_error` (e.g. `ready_db_error`, `ready_schema_error`,
`status_watchlist_error`, `status_schema_error`). Never put `err.Error()`, a DSN, a path, or a
webhook URL in a response struct field. `RunResult` carries **no** free-text error field — only
the deterministically composed `Summary` string.

### JSON response encoding (all handlers)

**Source:** every handler in the package — `health.go:41-43`, `events.go:144-146`
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(status)
_ = json.NewEncoder(w).Encode(resp)
```
Response envelope is a typed struct with `json:"..."` tags mirroring the exact contract by field
name (see `healthResponse` doc comment `health.go:19-26`, `eventsResponse` `events.go:29-33`).

### Consumer-declared narrow seam (applies to `SchemaVersioner`, `StatusStore`, `WatchlistCounter`, `RunRecorder`)

**Source:** `internal/httpserver/server.go:20-27` (`Pinger`), `internal/events/service.go:78-84` (`Store`),
`internal/poller/poller.go:61-101` (`ReleaseGroupSource` / `EventRecorder` / `Notifier`).

```go
// server.go:20-27
// Pinger is the minimal surface Server needs from a database handle.
// *pgxpool.Pool satisfies it. Defining this seam (rather than depending on
// *pgxpool.Pool directly) lets tests exercise the database-down branch ...
type Pinger interface {
    Ping(ctx context.Context) error
}
```
```go
// events/service.go:78-84
// Store is the minimal surface internal/httpserver needs for the events
// resource -- narrower than sqlc.Querier so a stub can implement it in
// tests without a live Postgres connection, mirroring watchlist.Store and
// httpserver.Pinger.
type Store interface {
    List(ctx context.Context, p ListParams) (Page, error)
}
```
Idiom: interface declared in the **consuming** package, one `var _ Iface = (*Concrete)(nil)`
compile assertion next to it, concrete type injected at `cmd/server/main.go`. Do **not** widen
`watchlist.Store` for the count — declare a separate `WatchlistCounter` in `httpserver`.

### Functional option keeps `New` additive (applies to `server.go`, `poller.go`)

**Source:** `internal/httpserver/server.go:42-72`, `internal/poller/poller.go:103-123`,
`internal/notifier/notifier.go:76-84`

```go
// server.go:42-51
type serverConfig struct {
    gatePassphrase    string
    gateAlerter       authgate.Alerter
    trustProxyHeaders bool
}
type Option func(*serverConfig)

// server.go:66-72
func WithAuthGate(passphrase string, trustProxyHeaders bool, alerter authgate.Alerter) Option {
    return func(c *serverConfig) {
        c.gatePassphrase = passphrase
        c.trustProxyHeaders = trustProxyHeaders
        c.gateAlerter = alerter
    }
}
```
```go
// poller.go:108-115
type Option func(*Poller)
func WithMusicBrainzWorkers(n int) Option {
    return func(p *Poller) { p.mbWorkers = n }
}
```
Note the two option shapes differ: `httpserver.Option` mutates a `serverConfig` struct that `New`
reads *after* the loop (`server.go:106-117`); `poller.Option` mutates the `*Poller` directly
(`poller.go:196-198`). `WithReadiness`/`WithStatus` follow `httpserver`'s `serverConfig` shape;
`WithRunRecorder` follows `poller`'s `*Poller` shape.

### Non-nil default so the call site never nil-checks (applies to `RunRecorder`)

**Source:** `internal/notifier/notifier.go:47-62` + `internal/poller/poller.go:91-98`

```go
// notifier.go:57-62
// NoOp is D-10's inert Sink, returned by Select when DISCORD_WEBHOOK_URL is
// unset, so poller.go's Notifier seam is always non-nil.
type NoOp struct{}
func (NoOp) NotifyPending(ctx context.Context, logger *slog.Logger) error { return nil }
```
```go
// poller.go:95-98 (doc on the Notifier seam)
// ... always hands New a non-nil Notifier (a real one, or notifier.NoOp),
// so neither cycle method ever nil-checks this field.
```
Apply: a private `noopRunRecorder{}` assigned in `poller.New` before the opts loop, so
`p.runs` is never nil even when `WithRunRecorder` is absent. `WithRunRecorder` guards `if r != nil`.

---

## Pattern Assignments

### `internal/httpserver/ready.go` (controller, request-response)

**Analog:** `internal/httpserver/health.go` (entire file, 44 lines — near-copy).

**Full template** (`health.go:13-44`):
```go
const healthPingTimeout = 3 * time.Second

type healthResponse struct {
    Status string `json:"status"`
    DB     string `json:"db"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
    defer cancel()

    resp := healthResponse{Status: "ok", DB: "up"}
    status := http.StatusOK

    if err := s.db.Ping(ctx); err != nil {
        resp.Status, resp.DB = "degraded", "down"
        status = http.StatusServiceUnavailable
        httplog.SetAttrs(r.Context(), slog.String("db_error", err.Error()))
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(resp)
}
```

**What `handleReady` adds on top:**
- Reuse `healthPingTimeout` (3s) — do not add a new const unless a sibling name reads better.
- After a successful `s.db.Ping(ctx)`, call `s.schema.SchemaVersion(ctx)` (new `SchemaVersioner` seam).
- Reason mapping: `ping err → "db_unreachable"`, `schema read err → "db_unreachable"`,
  `dirty → "schema_dirty"`, `applied < expected → "schema_behind"`, else `"ready"`.
- **D-01: `applied >= expected` is ready — NEVER `==`.** Put the Phase-16 rationale
  (`migrate.go:298-302` ahead-of-source guard lets a rolled-back binary serve a newer additive
  schema) in a 1-3 line handler comment per CLAUDE.md comment discipline.
- Body shape locked (D-03): `readyResponse{Status, SchemaApplied *uint (null when DB unreachable),
  SchemaExpected uint, Reason string json:"reason,omitempty"}`. See 18-RESEARCH.md lines 716-772
  for the full worked handler.
- `pgx.ErrNoRows` (empty `schema_migrations`) → `schema_behind` with `schema_applied: null`.
- When `WithReadiness` was not supplied (`s.schema == nil`) → `503 {"reason":"not_configured"}`
  so the route table is byte-identical in both option states.

**Seam to declare in this file** (mirrors `Pinger`):
```go
type SchemaVersioner interface {
    SchemaVersion(ctx context.Context) (version uint, dirty bool, err error)
}
```

---

### `internal/httpserver/status.go` (controller, request-response read-aggregate)

**Analog:** `internal/httpserver/events.go` `handleListEvents` — specifically the store-error tail
(`events.go:118-146`) and the non-nil-slice defensive backstop (`events.go:130-133`).

**Error tail to copy** (`events.go:124-146`):
```go
if err != nil {
    httplog.SetAttrs(r.Context(), slog.String("events_error", err.Error()))
    writeError(w, http.StatusInternalServerError, "internal error")
    return
}
// ...
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
_ = json.NewEncoder(w).Encode(eventsResponse{Events: evs, ...})
```

**`handleStatus` specifics:**
- Reads three seams: `s.watchlistCounter.Count(ctx)` (→ 500 `"internal error"` on error, like
  `handleListEvents`), `s.schema.SchemaVersion(ctx)` (best-effort — on error log to
  `httplog.SetAttrs` and still return 200 with `schema_applied: null`, per RESEARCH Open Q4),
  `s.statusStore.Snapshot()` (in-memory, cannot fail).
- `poll_interval` from `s.pollInterval` (passed via `WithStatus`); render as int seconds
  (`900` for the 15m default) — key `poll_interval_seconds`.
- `instance` block: `{app_version: buildinfo.Version (or buildinfo.Short()), schema_applied,
  schema_expected}` — reuse `/ready`'s key names `schema_applied`/`schema_expected` (D-11 discretion,
  RESEARCH recommends reuse).
- Empty `runs`/`history` on a fresh instance = valid **200** (D-11, Pitfall 3). Encode `sources`
  as an object keyed by source string, each `{last_run: null, history: [], last_skipped_at: null,
  consecutive_skips: 0}`.
- **STAT-02:** no `error`/`detail`/`message`/`last_error` field anywhere in the envelope.
- **Frozen contract** — write the exact JSON shape into the plan and a contract note for Phase 19.
  RESEARCH lines 774-826 give the recommended frozen envelope; the planner must lock key names.

**Seams to declare here** (mirror `events.Store`):
```go
type StatusStore interface {
    Snapshot() map[string]pollruns.SourceSnapshot
}
type WatchlistCounter interface {
    Count(ctx context.Context) (int64, error)
}
```

---

### `internal/pollruns/pollruns.go` (store, event-driven)

**Analogs:**
- Package shape / narrow-seam philosophy: `internal/events/service.go` (a package that owns one
  domain concept behind a `Store` interface + `Service` impl).
- No-op default pattern for the poller seam: `internal/notifier/notifier.go:57-62` (`NoOp`).
- Mutex-guarded shared state under concurrent producers: `internal/notifier/notifier.go:68-74`
  (`notifying atomic.Bool`) and `internal/poller/poller.go:131-159` (separate `mbRunning`/
  `dzRunning atomic.Bool`, "never one shared mutex" reasoning — here one `sync.Mutex` on the Store
  is correct because the two sources write different map keys and D-08 explicitly allows it).

**Full skeleton is in 18-RESEARCH.md lines 861-971** — copy it. Key points:
- `const N = 50` with a 1-line comment on the number (CLAUDE.md: comment the *why*).
- `RunResult` struct: `Source, CycleID, StartedAt, FinishedAt time.Time, DurationMS int64,
  ArtistsChecked, ArtistsSkipped, ArtistsErrored, EventsRecorded int, Outcome string, Summary string`.
  No error field.
- `RecordRun(_ context.Context, r RunResult) error` — mutex append, trim to N, reset
  `consecutiveSkips = 0`. `ctx` is vestigial (D-07) but kept for seam symmetry.
- `RecordSkip(source string)` — bump `consecutiveSkips`, stamp `lastSkippedAt = time.Now()`.
- `Snapshot() map[string]SourceSnapshot` — **deep-copy the ring slice while holding the lock**
  (Pitfall 4: returning the field directly is a data race). `RunResult` is all value types so
  `copy` / element assignment is a full deep copy. Return newest-first.
- `SourceSnapshot{LastRun *RunResult, History []RunResult, LastSkippedAt *time.Time, ConsecutiveSkips int}`.

**Compile assertion** (in `internal/poller` or a test): `var _ poller.RunRecorder = (*pollruns.Store)(nil)`.

**Package layering (RESEARCH recommendation, lines 485-498):** `RunResult`, `N`, `Store` all in
`internal/pollruns`; `poller` declares only the `RunRecorder` *interface* and imports `pollruns`
for the `RunResult` type — exactly as `poller` already imports `internal/musicbrainz` /
`internal/deezer` purely for DTO types in its seams (`poller.go:28-30,66-89`). `pollruns` imports
nothing from this repo.

---

### `internal/buildinfo/buildinfo.go` (config/constant)

**Analog:** `internal/httpclient/httpclient.go` — as a *layout* reference only (small,
single-purpose `internal/` package with a package doc comment explaining why it exists separately).

Content is ~3 lines:
```go
// Package buildinfo holds the build-time version string injected at link time
// via -ldflags -X (see Dockerfile). "dev" is the value for any build without
// that flag (local `go build`, `make build`, `go test`).
package buildinfo

var Version = "dev"

// Short returns Version truncated to 12 chars for display in /status.
func Short() string {
    if len(Version) > 12 {
        return Version[:12]
    }
    return Version
}
```
Module path for the `-X` flag: `github.com/danielrpof/drop-tracker/internal/buildinfo.Version`
(module path verified `internal/httpserver/server.go:14`). A wrong path is silently ignored
(Pitfall 7) — unit-test that the zero value is `"dev"`.

---

### `internal/db/migrate.go` (MOD — model / DB helper)

**Analogs (same file):**
- `ExpectedSchemaVersion()` reuses `iofs.New(migrationsFS, "migrations")` (the source
  `RunMigrations` builds, `migrate.go` embed at `:22-23`) + `maxSourceVersion` walk (`:326-341`).
- `SchemaVersion(ctx, q)` mirrors the ahead-of-source guard's read (`migrate.go:298-302`):
  ```go
  if cur, dirty, verr := m.Version(); verr == nil && !dirty {
      if smax, ok := maxSourceVersion(src); ok && cur > smax {
          return nil
      }
  }
  ```
  — same `(version, dirty, err)` triple, but hand-rolled pgx instead of `migrate.NewWithInstance`
  (which opens a fresh `database/sql` conn — `migrate.go:280-291` — the thing we must NOT do).

**`maxSourceVersion` verbatim** (`migrate.go:320-341`):
```go
func maxSourceVersion(src source.Driver) (uint, bool) {
    v, err := src.First()
    if err != nil {
        return 0, false
    }
    for {
        next, err := src.Next(v)
        if errors.Is(err, os.ErrNotExist) {
            return v, true
        }
        if err != nil {
            return 0, false
        }
        v = next
    }
}
```

**New code** — full worked versions in 18-RESEARCH.md lines 656-712. `SchemaVersion` goes in a new
file `internal/db/schema_version.go` with a `RowQuerier` interface (`QueryRow(ctx, sql, args...)
pgx.Row`) that `*pgxpool.Pool` satisfies; SQL is `SELECT version, dirty FROM schema_migrations`;
`errors.Is(err, pgx.ErrNoRows)` → `(0, false, nil)`. Error wrapping idiom in this file:
`fmt.Errorf("read schema_migrations: %w", err)` / `fmt.Errorf("load embedded migrations: %w", err)`.
`ExpectedSchemaVersion()` returns `7` today (highest migration on disk is `000007`).

---

### `internal/httpserver/server.go` (MOD — route wiring / DI)

**Analogs (same file):**

**`Server` struct** gets new fields next to `db Pinger` (`server.go:29-37`):
```go
type Server struct {
    db        Pinger
    watchlist watchlist.Store
    events    events.Store
    // + schema           SchemaVersioner
    // + expectedSchema   uint
    // + statusStore      StatusStore
    // + watchlistCounter WatchlistCounter
    // + pollInterval     time.Duration
    ...
}
```

**`serverConfig` + new options** (`server.go:42-51`, `WithAuthGate` at `:66-72`) — add
`WithReadiness(sv SchemaVersioner, expected uint)` and `WithStatus(store StatusStore, counter
WatchlistCounter, sv SchemaVersioner, expected uint, pollInterval time.Duration)` as new `Option`s
that stash onto `serverConfig`; `New` copies them onto `s` after the opts loop (`server.go:109-117`).
Every existing `httpserver.New(...)` call compiles unchanged (this is the ~60-call-site
constraint — see the `New` doc comment `server.go:100-105`).

**`/ready` registration** — copy the `/health` line and its rationale comment
(`server.go:161-164`), place `/ready` **immediately after** it, still outside `if gate != nil`:
```go
r.Get("/health", s.handleHealth)
r.Get("/ready", s.handleReady)   // NEW — same structural exemption, both branches

if gate != nil {
    ...
    r.Group(func(pr chi.Router) {
        pr.Use(gate.Authenticate)
        pr.Use(gate.RequireCSRFHeader)
        registerDataRoutes(pr, s)
    })
} else {
    registerDataRoutes(r, s)
}
```

**`/status` registration** — add to `registerDataRoutes` (`server.go:197-204`) exactly like
`/events`:
```go
func registerDataRoutes(r chi.Router, s *Server) {
    r.Get("/search", s.handleSearch)
    ...
    r.Get("/events", s.handleListEvents)
    r.Get("/status", s.handleStatus)   // NEW — inherits gate.Authenticate + CSRF header + X-Instance-Gated; GET so CSRF-exempt
}
```

---

### `internal/poller/poller.go` (MOD — seam declaration, additive only)

**Analog (same file):** the `Notifier` interface + `WithMusicBrainzWorkers` option +
always-non-nil default.

**`Notifier` seam** (`poller.go:91-101`):
```go
type Notifier interface {
    NotifyPending(ctx context.Context, logger *slog.Logger) error
}
```
**`WithMusicBrainzWorkers`** (`poller.go:110-115`):
```go
type Option func(*Poller)
func WithMusicBrainzWorkers(n int) Option {
    return func(p *Poller) { p.mbWorkers = n }
}
```
**Default assignment in `New`** (`poller.go:183-198`): fields set in the `&Poller{...}` literal,
then `for _, opt := range opts { opt(p) }`.

**Add (all additive, `runCycle` and both cycle methods UNTOUCHED):**
```go
type RunRecorder interface {
    RecordRun(ctx context.Context, result pollruns.RunResult) error
    RecordSkip(source string)
}

type noopRunRecorder struct{}
func (noopRunRecorder) RecordRun(context.Context, pollruns.RunResult) error { return nil }
func (noopRunRecorder) RecordSkip(string) {}
var _ RunRecorder = noopRunRecorder{}

func WithRunRecorder(r RunRecorder) Option {
    return func(p *Poller) { if r != nil { p.runs = r } }
}
```
- New field `runs RunRecorder` on `Poller` (near `notifier Notifier`, `poller.go:141`).
- In `New`, set `p.runs = noopRunRecorder{}` before the opts loop (or in the struct literal).
- Import `internal/pollruns` alongside `internal/musicbrainz` / `internal/deezer` (`poller.go:28-30`).
- **Do NOT** add `RunRecorder` as a positional `New` param (40+ test call sites — `poller.go:178`).
- **Do NOT** call `p.runs.RecordRun` / `RecordSkip` anywhere this phase (that is Phase 18.1).

---

### `internal/poller/poller_test.go` (MOD — test double)

**Analog (same file):** `fakeEventRecorder` (`poller_test.go:183-219`) and `fakeNotifier`
(`poller_test.go:221-241`).

**`fakeNotifier` verbatim** (`poller_test.go:227-241`):
```go
type fakeNotifier struct {
    fn func(ctx context.Context, logger *slog.Logger) error
    calls int32
}
func (f *fakeNotifier) NotifyPending(ctx context.Context, logger *slog.Logger) error {
    atomic.AddInt32(&f.calls, 1)
    if f.fn != nil {
        return f.fn(ctx, logger)
    }
    return nil
}
var _ Notifier = (*fakeNotifier)(nil)
```
Add `fakeRunRecorder` in the same shape: `atomic.AddInt32` call counters for `RecordRun` /
`RecordSkip`, optional `fn` hooks, `var _ RunRecorder = (*fakeRunRecorder)(nil)`. Add a test that
`WithRunRecorder` wires it and that after a full `RunMusicBrainzCycle` / `RunDeezerCycle` the
recorder's call count is **still 0** this phase (proves the seam is inert).

---

### `queries/watchlist.sql` (MOD — sqlc query)

**Analog:** `queries/health.sql` (`-- name: Ping :one` / `SELECT 1;`) for the trivial `:one`
shape; `queries/events.sql:31-36` `HasAnyEvent :one` for the comment idiom (1-3 line `-- name:`
header explaining the *why*).

Add:
```sql
-- name: CountWatchlist :one
-- Backs /status watchlist_size (STAT-01). A count(*), not len(ListWatchlist)
-- in Go -- ListWatchlist JOINs artists and returns every row's full projection.
SELECT count(*) FROM watchlist;
```
Then `sqlc generate` (pinned v1.31.1, `Makefile:17`) and commit the regenerated
`internal/db/sqlc/watchlist.sql.go` + `querier.go` deltas together. **`make sqlc-check` is
local-only — no CI counterpart** (Pitfall 5 / CLAUDE.md DoD item 5). `emit_interface: true`
(`sqlc.yaml`) means `CountWatchlist(ctx) (int64, error)` lands on `sqlc.Querier`; it does **not**
ripple into `internal/watchlist.Store` unless deliberately widened — don't.

---

### `cmd/server/main.go` (MOD — composition root)

**Analog (same file):** the existing wiring block `main.go:215-262` — `store` / `eventsStore` /
`srv := httpserver.New(...)` / `notif := notifier.Select(...)` / `pollr, err := poller.New(...)`.
Each dependency is constructed, commented with its *why*, then threaded into `New`.

**`httpserver.New` call** (`main.go:235-238`) — append options:
```go
srv := httpserver.New(pool, store, eventsStore, []httpserver.SearchSource{
    httpserver.NewMusicBrainzSource(mbClient),
    httpserver.NewDeezerSource(dzClient),
}, logger,
    httpserver.WithAuthGate(cfg.InstancePassphrase, cfg.TrustProxyHeaders, authgate.SelectAlerter(cfg.DiscordWebhookURL, logger)),
    httpserver.WithReadiness(schemaVersioner, expected),
    httpserver.WithStatus(runs, watchlistCounter, schemaVersioner, expected, buildinfo.Version, cfg.PollInterval),
)
```
**`poller.New` call** (`main.go:258`) — append `poller.WithRunRecorder(runs)`.

**New wiring lines:**
- `expected, err := db.ExpectedSchemaVersion()` right after `db.RunMigrations` (`main.go:137-139`),
  same `return fmt.Errorf(...)` error style.
- `runs := pollruns.NewStore()` — used as BOTH `poller.WithRunRecorder(runs)` and
  `httpserver.WithStatus(runs, ...)` (one instance, mirrors how `mbClient` is shared between
  `httpserver.New` and `poller.New`, see `main.go:252-257` comment).
- `schemaVersioner` — a thin adapter over `pool` implementing `SchemaVersion(ctx)` by calling
  `db.SchemaVersion(ctx, pool)` (RESEARCH suggests `db.SchemaVersionerFor(pool)` helper).
- `watchlistCounter` — 3-line adapter over `sqlc.New(pool)` calling `CountWatchlist`, mirroring the
  "`sqlc.Queries` is a stateless wrapper over the shared pool, a Nth instance is fine" pattern
  (`main.go:217-223` eventsStore comment).

---

### `Dockerfile` (MOD — build config)

**Analog (same file):** builder stage `Dockerfile:60-61`:
```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
    -o /out/server ./cmd/server
```
Change to:
```dockerfile
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-w -s -X github.com/danielrpof/drop-tracker/internal/buildinfo.Version=$VERSION" \
    -o /out/server ./cmd/server
```
**Pitfall 6:** the top comment `Dockerfile:15-19` explicitly forbids `ARG` carrying a config value.
In the **same commit**, amend that block to carve out the exception — `VERSION` is non-secret build
provenance injected at link time, never read at runtime as configuration. D-13 sanctions this.

---

### `.github/workflows/full-pipeline.yml` (MOD — CI config)

**Analog (same file):** the `build-scan` job's `docker/build-push-action` step
(`full-pipeline.yml:575-583`):
```yaml
      - name: Build image
        uses: docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a # v7.3.0
        with:
          context: .
          push: false
          load: true
          tags: drop-tracker:scan
          cache-from: type=gha
          cache-to: type=gha,mode=max
```
Add one key:
```yaml
          build-args: |
            VERSION=${{ github.sha }}
```
**Only this step.** The `release` job (`full-pipeline.yml:608+`) `docker load`s the saved tarball
and only tags/pushes — it must stay byte-for-byte untouched (D-13, single-build guarantee, 07-REVIEW
CR-02). The precedent for how Phases 15/16 appended to this shared file: append to `needs:` lists
and add steps, never a job-level `if:` on `build-scan`/`release` (see the `needs:` comment
`full-pipeline.yml:557-564`).

---

## Test Pattern Assignments

### `internal/httpserver/ready_test.go` & `status_test.go`

**Analogs:** `internal/httpserver/health_test.go` (entire file) and
`internal/httpserver/server_test.go:224-236` (`newGatedServer`).

- **`stubPinger`** (`health_test.go:35-47`) — file-local double, `pingFunc func(context.Context) error`
  field, `var _ httpserver.Pinger = stubPinger{}`. Add `fakeSchemaVersioner`, `fakeStatusStore`,
  `fakeWatchlistCounter` in the same shape (func-field + compile assertion).
- **Leak assertion loop** (`health_test.go:113-118`):
  ```go
  raw := string(data)
  for _, leak := range []string{pingErr.Error(), "postgres://", "password"} {
      if strings.Contains(raw, leak) {
          t.Fatalf("response body leaked %q: %s", leak, raw)
      }
  }
  ```
  For `TestStatus_NoLeak` feed a DSN-bearing (`postgres://u:p@h/db`) and webhook-bearing error
  through the `WatchlistCounter` / `SchemaVersioner` fakes.
- **Timeout test** (`health_test.go:121-147` `TestHealth_DownOnTimeout`): `pingFunc` blocks on
  `<-ctx.Done()`, assert 503 and the handler never optimistically reports ready.
- **Body decoded by field name, not raw string** (`healthBody` `health_test.go:30-33`).
- **`discardLogger()`** (`health_test.go:51-53`) — `logging.NewWithWriter(&config.Config{...},
  io.Discard)`.
- **Gated non-401 test** (RDY-02, Pitfall 8): build with `newGatedServer(t, "passphrase", false)`
  (`server_test.go:228-236`), `GET /ready` with no cookie, assert status `!= 401`.
- **Gated 401 test** (STAT-01): `handleStatus` on a `newGatedServer` with no cookie → assert 401,
  mirroring the `gatedRoutes` loop (`server_test.go:238-248,282-291`).
- `httptest.NewServer(srv.Router())` + `t.Cleanup`/`defer ts.Close()`; `httpserver.New(stub,
  stubStore{}, stubEventsStore{}, nil, discardLogger(), <opts>)` construction.

### `internal/pollruns/pollruns_test.go`

**Analog:** `internal/poller/poller_test.go:150-177` (`fakeReleaseGroupSource`'s atomic
in-flight/`maxInFlight` CAS tracking) for the concurrency-assertion style. Framework is stdlib
`testing`, table-driven, `t.Fatalf` — no testify.

- `TestStore_RingBound`: record 60 for one source, assert `len(history) == 50`, newest-first.
- `TestStore_Skip`: `RecordSkip` bumps `consecutive_skips` + stamps `last_skipped_at`; `RecordRun`
  resets `consecutive_skips` to 0.
- `TestStore_TwoSourceConcurrent`: looped (~1000×) — two goroutines each `RecordRun` 60× for their
  own source, assert each source ends with exactly 50 and no torn `RunResult`. This is the stated
  `-race` substitute (`-race` unavailable on this box — 18-RESEARCH.md line 1062).

### `internal/db/schema_version_test.go`

**Analogs:** `internal/db/migrate_ahead_test.go` / `internal/db/migrate_test.go` — embed-FS unit
tests + `testutil.NewTestPool(t)` integration tests gated on `make db-up`.

- `TestExpectedSchemaVersion` — unit, no DB, asserts `7` (embedded max).
- `TestSchemaVersion_Integration` — `make db-up`, real `schema_migrations` read (version + dirty).
- `CountWatchlist` integration test may live in an existing watchlist-query test file.

### `internal/buildinfo/buildinfo_test.go`

**Analog:** `internal/httpclient/httpclient_test.go` (minimal package test). One assertion:
`buildinfo.Version == "dev"` by default; optionally `Short()` truncation behaviour.

---

## No Analog Found

None. Every file maps to an existing in-repo pattern. The single genuinely new primitive is the
mutex-guarded ring buffer in `internal/pollruns` (~40 lines), and even its no-op-default and
narrow-seam scaffolding mirror `internal/notifier` and `internal/events`.

---

## Metadata

**Analog search scope:** `internal/httpserver/`, `internal/poller/`, `internal/notifier/`,
`internal/events/`, `internal/db/`, `internal/httpclient/`, `queries/`, `cmd/server/`, `Dockerfile`,
`.github/workflows/full-pipeline.yml`
**Files scanned:** ~20 (health.go, health_test.go, server.go, server_test.go, events.go,
events.sql, watchlist.sql, health.sql, poller.go, poller_test.go, notifier.go, events/service.go,
migrate.go, main.go, Dockerfile, full-pipeline.yml, httpclient.go, watchlist.go, plus the phase
CONTEXT + RESEARCH)
**Tracked-source gate:** all analog paths verified git-tracked; no gitignored mirror tree exists in
this repo.
**Pattern extraction date:** 2026-09-09
