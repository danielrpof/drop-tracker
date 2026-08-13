# Phase 2: Watchlist Core - Pattern Map

**Mapped:** 2026-08-05
**Files analyzed:** 11 (new/modified)
**Analogs found:** 11 / 11 (Phase 1 has direct analogs for every role needed; no "no analog" files this phase)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/db/migrations/000002_watchlist.up.sql` | migration | batch (DDL) | `internal/db/migrations/000001_init.up.sql` | role-match (structure only; 000001 is a no-op) |
| `internal/db/migrations/000002_watchlist.down.sql` | migration | batch (DDL) | `internal/db/migrations/000001_init.down.sql` | role-match |
| `queries/artists.sql` | model (sqlc query source) | CRUD | `queries/health.sql` | role-match |
| `queries/watchlist.sql` | model (sqlc query source) | CRUD | `queries/health.sql` | role-match |
| `sqlc.yaml` (modified) | config | — | itself (existing) | exact (edit in place) |
| `internal/watchlist/service.go` | service | CRUD | `internal/httpserver/server.go`'s `Pinger` interface pattern | role-match (interface-seam pattern, not a service file itself — no existing `internal/<domain>` service package yet) |
| `internal/watchlist/service_test.go` | test | CRUD | `internal/httpserver/health_test.go` (integration half, `testutil.NewTestPool`) | role-match |
| `internal/httpserver/watchlist.go` | controller (handler) | request-response | `internal/httpserver/health.go` | exact |
| `internal/httpserver/watchlist_test.go` | test | request-response | `internal/httpserver/health_test.go` | exact |
| `internal/httpserver/server.go` (modified — add `store` param, register routes) | controller (router wiring) | request-response | itself (existing) | exact (edit in place) |
| `cmd/server/main.go` (modified — new `httpserver.New` call site) | config/wiring | request-response | itself (existing, line 80) | exact (edit in place) |

## Pattern Assignments

### `internal/watchlist/service.go` (service, CRUD)

**Analog:** `internal/httpserver/server.go` (interface-seam pattern) + patterns cited in RESEARCH.md Code Examples (this phase introduces the first true service package, so the seam shape — not a literal file — is what to copy)

**Interface-seam pattern to replicate** (`internal/httpserver/server.go:15-22`):
```go
// Pinger is the minimal surface Server needs from a database handle.
// *pgxpool.Pool satisfies it. Defining this seam (rather than depending on
// *pgxpool.Pool directly) lets tests exercise the database-down branch with
// a fake that never dials a real database, without changing Server's
// exported signature.
type Pinger interface {
	Ping(ctx context.Context) error
}
```
Apply the same narrowing to `watchlist.Store`: name only `Add`, `List`, `UpdatePreferences`, `Remove` — never expose the full sqlc `Querier`.

**Error translation pattern** (from RESEARCH.md Code Examples, Pattern 3 — not yet in repo, this is new code but the import path warning is load-bearing):
```go
// import "github.com/jackc/pgx/v5/pgconn" — NOT the older "github.com/jackc/pgconn"
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "watchlist_artist_id_key" {
	return Entry{}, ErrDuplicate
}
```

**Sentinel error pattern:** follow stdlib `errors.New` sentinels (`ErrDuplicate`, `ErrNotFound`), matching how Phase 1 handlers already use `errors.Is`-style checks (see handler pattern below).

---

### `internal/httpserver/watchlist.go` (controller, request-response)

**Analog:** `internal/httpserver/health.go` (full file, 44 lines — read completely, no partial reads needed)

**Imports pattern** (`internal/httpserver/health.go:1-11`):
```go
package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/httplog/v3"
)
```
For watchlist.go, add `"errors"` for `errors.Is(err, watchlist.ErrDuplicate)` and the `internal/watchlist` package import.

**Handler shape (context timeout + typed response + Content-Type + JSON encode)** (`internal/httpserver/health.go:28-44`):
```go
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
Key conventions to copy exactly:
- Errors are attached to the structured log via `httplog.SetAttrs(r.Context(), slog.String("<field>_error", err.Error()))`, **never** included in the JSON response body (D-13 compliance — response body stays a clean `{"error": "..."}`, no leaked DB detail).
- `w.Header().Set("Content-Type", "application/json")` set before `WriteHeader`.
- Response struct is a small purpose-built type with `json` tags (`healthResponse` → for watchlist: `addWatchlistRequest`/`watchlistEntryResponse` etc.) — never decode/encode the sqlc model directly (anti-pattern flagged in RESEARCH.md).

**Full add-handler skeleton already drafted in RESEARCH.md** (Code Examples section, cites `health.go:28-44` as its shape source) — use directly as the `handleAddWatchlist` starting point; replicate the same switch-on-sentinel-error shape for `handleRemoveWatchlist` (404 via `ErrNotFound`) and `handleUpdateWatchlist` (400 on invalid values, 404 on missing id).

**writeError helper (new, not yet in repo):** RESEARCH.md's "Don't Hand-Roll" table specifies a single `writeError(w, status, msg)` helper — add this once in `internal/httpserver` (e.g. new small file or top of `watchlist.go`) and use it from both future health.go edits and all watchlist handlers, for D-13 consistency.

---

### `internal/httpserver/watchlist_test.go` (test, request-response)

**Analog:** `internal/httpserver/health_test.go` (full file, 196 lines)

**Stub-double pattern** (`internal/httpserver/health_test.go:35-47`):
```go
type stubPinger struct {
	pingFunc func(context.Context) error
}

func (s stubPinger) Ping(ctx context.Context) error {
	return s.pingFunc(ctx)
}

var _ httpserver.Pinger = stubPinger{}
```
Replicate as a `stubStore` implementing `watchlist.Store` with per-method func fields (`addFunc`, `listFunc`, `updateFunc`, `removeFunc`), each settable per test case — mirrors this exact shape.

**httptest.Server wiring pattern** (`internal/httpserver/health_test.go:55-65`):
```go
func TestHealth_Up(t *testing.T) {
	pool := testutil.NewTestPool(t)
	srv := httpserver.New(pool, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	...
}
```
For watchlist tests: `srv := httpserver.New(stubPinger{...}, stubStore{...}, discardLogger())` (new 2nd param per Pitfall 5), then `http.Post`/`http.NewRequest(http.MethodPatch, ...)`/`http.NewRequest(http.MethodDelete, ...)` as needed.

**Response-shape verification via typed struct, not raw string match** (`internal/httpserver/health_test.go:27-33, 74-80`):
```go
type healthBody struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}
...
var body healthBody
if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
	t.Fatalf("decode response body: %v", err)
}
if body.Status != "ok" || body.DB != "up" {
	t.Fatalf("body = %+v, want {Status:ok DB:up}", body)
}
```

**Error-leak assertion pattern** (`internal/httpserver/health_test.go:113-118`) — apply the same idea to watchlist error paths (e.g., ensure a 500 body never leaks a raw pg error string):
```go
raw := string(data)
for _, leak := range []string{pingErr.Error(), "postgres://", "password"} {
	if strings.Contains(raw, leak) {
		t.Fatalf("response body leaked %q: %s", leak, raw)
	}
}
```

**discardLogger helper** (`internal/httpserver/health_test.go:51-53`) — reuse as-is, no need to redefine in `watchlist_test.go` (same package `httpserver_test`):
```go
func discardLogger() *slog.Logger {
	return logging.NewWithWriter(&config.Config{LogLevel: "info", LogFormat: "text"}, io.Discard)
}
```

---

### `internal/httpserver/server.go` (modified: add `store` param + register routes)

**Analog:** itself, current state (full file, 81 lines)

**Current constructor signature to extend** (`internal/httpserver/server.go:39-62`):
```go
func New(db Pinger, logger *slog.Logger) *Server {
	s := &Server{db: db}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(echoRequestID)
	r.Use(httplog.RequestLogger(logger, &httplog.Options{ ... }))
	r.Use(middleware.Recoverer)

	r.Get("/health", s.handleHealth)

	s.router = r
	return s
}
```
Per Pitfall 5 (RESEARCH.md), change to `New(db Pinger, store watchlist.Store, logger *slog.Logger) *Server`, add `store: store` to the `Server{}` literal, and register the four new routes alongside `/health`:
```go
r.Post("/watchlist", s.handleAddWatchlist)
r.Get("/watchlist", s.handleListWatchlist)
r.Patch("/watchlist/{id}", s.handleUpdateWatchlist)
r.Delete("/watchlist/{id}", s.handleRemoveWatchlist)
```
Do **not** widen `Pinger` itself — add a second, separate interface parameter (`Store`), exactly as RESEARCH.md's Anti-Patterns section warns.

**All 5 call sites that must be updated in the same commit** (`grep` results this session):
- `cmd/server/main.go:80` — `srv := httpserver.New(pool, logger)`
- `internal/httpserver/health_test.go:57,86,126,151` — four `httpserver.New(...)` calls (pass a trivial no-op `stubStore{}` or `nil` since these tests never touch watchlist routes)
- `internal/httpserver/server_test.go:83,203` — two more `httpserver.New(...)` calls

---

### `queries/artists.sql`, `queries/watchlist.sql` (model, CRUD)

**Analog:** `queries/health.sql` (1 file, 2 lines — trivial but establishes the sqlc query-source convention: one `-- name: X :annotation` comment per query, plain `.sql`, no ORM abstraction)
```sql
-- name: Ping :one
SELECT 1;
```
Full query bodies for `artists.sql`/`watchlist.sql` are already drafted in RESEARCH.md's "Code Examples" section (`UpsertArtist`, `CreateWatchlistEntry`, `ListWatchlist`, `UpdateWatchlistPreferences`, `DeleteWatchlistEntry`) — copy directly from there; they already account for Pitfall 4 (aliased `id`/`artist_id` columns) and Pattern 2 (`:execrows` for delete).

---

### `internal/db/migrations/000002_watchlist.up.sql` / `.down.sql` (migration, batch)

**Analog:** `internal/db/migrations/000001_init.up.sql` (establishes the naming convention `NNNNNN_description.{up,down}.sql`; content itself is a no-op placeholder, so only the **naming/embed convention** carries over, not the SQL body)

Full schema (both tables, constraints, defaults) is already drafted in RESEARCH.md's Code Examples section — use directly, it already incorporates D-01/02/03/04/06/07/08 and Pitfall 1's `text[] + CHECK` guidance instead of native enums.

**down migration:** not present in RESEARCH.md examples — planner must add a matching `DROP TABLE watchlist; DROP TABLE artists;` (reverse order of creation, respecting the FK) for `000002_watchlist.down.sql`.

---

### `sqlc.yaml` (config, modify in place)

**Current state** (`sqlc.yaml:1-12`):
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
Add two keys under `gen.go` per RESEARCH.md:
```yaml
        emit_interface: true
        emit_pointers_for_null_types: true
```

## Shared Patterns

### Structured error logging without leaking to response body
**Source:** `internal/httpserver/health.go:35-39`
**Apply to:** All watchlist handlers (`handleAddWatchlist`, `handleListWatchlist`, `handleUpdateWatchlist`, `handleRemoveWatchlist`)
```go
if err := s.db.Ping(ctx); err != nil {
	resp.Status, resp.DB = "degraded", "down"
	status = http.StatusServiceUnavailable
	httplog.SetAttrs(r.Context(), slog.String("db_error", err.Error()))
}
```
Use `httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))` on the internal-error branch only; the client-facing body stays `{"error": "internal error"}` (D-13).

### JSON response contract
**Source:** `internal/httpserver/health.go:41-43`
**Apply to:** All controller files
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(status)
_ = json.NewEncoder(w).Encode(resp)
```

### Interface-seam testability (Pinger → Store)
**Source:** `internal/httpserver/server.go:15-22`, `internal/httpserver/health_test.go:35-47`
**Apply to:** `internal/watchlist/service.go` (define `Store`), `internal/httpserver/watchlist_test.go` (define `stubStore`)
Narrow interface named exactly with the methods the consumer needs — never the full generated `Querier`.

### pgx error-code translation (new pattern this phase, no existing analog in repo — first place pgconn/pgerrcode are used directly)
**Source:** RESEARCH.md Code Examples, Pattern 3
**Apply to:** `internal/watchlist/service.go` only
```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "watchlist_artist_id_key" {
	return Entry{}, ErrDuplicate
}
```
Must import `github.com/jackc/pgx/v5/pgconn`, not the legacy `github.com/jackc/pgconn` (Pitfall 2).

### Purpose-built request/response DTOs (never decode into sqlc model)
**Source:** RESEARCH.md Anti-Patterns section (no existing repo precedent since Phase 1 has no domain models yet — this is the first place the anti-pattern becomes possible)
**Apply to:** All watchlist handler request bodies
Decode into `addWatchlistRequest{MBID, Name, DeezerID, ReleaseTypes, MutedEventTypes}` etc., then construct sqlc params explicitly — never `json.Decode(&sqlcModel)` directly, to avoid over-posting `id`/`artist_id`/`created_at`.

## No Analog Found

None — every new file in this phase has a role-equivalent Phase 1 analog (health check controller/test/router-wiring) or a fully-drafted code example in RESEARCH.md to use as the literal starting point.

## Metadata

**Analog search scope:** `internal/httpserver/`, `internal/db/`, `queries/`, `cmd/server/`, `sqlc.yaml`
**Files scanned:** `server.go`, `health.go`, `health_test.go`, `boot_e2e_test.go`, `server_test.go`, `sqlc.yaml`, `queries/health.sql`, `internal/db/migrations/000001_init.up.sql`, `cmd/server/main.go` (grep for call sites)
**Pattern extraction date:** 2026-08-05
