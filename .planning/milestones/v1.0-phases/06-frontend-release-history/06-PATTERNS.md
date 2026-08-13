# Phase 6: Frontend & Release History - Pattern Map

**Mapped:** 2026-08-10
**Files analyzed:** 12 (backend: 6 new/modified, frontend: greenfield — patterns given as conventions, not file analogs)
**Analogs found:** 6 / 6 backend files. Frontend has no in-repo analog (genuinely greenfield) — patterns sourced from RESEARCH.md/UI-SPEC.md instead, noted explicitly below.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `queries/events.sql` (add `ListEvents`) | model (sqlc query) | CRUD (paginated read) | `queries/watchlist.sql` (`ListWatchlist`, `UpdateWatchlistPreferences`) | exact — same file, established convention |
| `internal/db/sqlc/*` (generated) | model | CRUD | generated from `queries/events.sql` | n/a — codegen, not hand-written |
| `internal/events/service.go` (new package, mirrors `internal/watchlist`) | service | CRUD (read-only) | `internal/watchlist/service.go` | exact — same Store-interface pattern, same repo |
| `internal/httpserver/events.go` | controller (HTTP handler) | request-response | `internal/httpserver/watchlist.go` (`handleListWatchlist`) + `search.go` (query-param parsing) | exact — same package, same handler shape |
| `internal/httpserver/server.go` (modify) | route wiring | request-response | itself (existing `New()`) | exact — same file, add one route + `r.NotFound` |
| `internal/httpserver/events_test.go` | test | request-response | `internal/httpserver/watchlist_test.go` (stub-store + real-Postgres dual pattern) | exact |
| `cmd/server/main.go` (modify — wire events service + static embed) | config/wiring | request-response | itself (existing `run()`) | exact |
| `internal/webassets/embed.go` (or similar new package) | middleware/static-serving | file-I/O | none in-repo (new capability) | no analog — synthesized pattern from RESEARCH.md Pattern 3 |
| `web/app/routes/watchlist.tsx` | component (route) | request-response | none in-repo (greenfield frontend) | no analog — use RESEARCH.md Code Examples + UI-SPEC |
| `web/app/routes/history.tsx` | component (route) | request-response, streaming-like (infinite scroll) | none in-repo | no analog |
| `web/app/lib/api.ts` | utility (typed fetch wrapper) | request-response | none in-repo | no analog — must match backend wire shapes exactly (see Shared Patterns) |
| `web/app/components/watchlist/*`, `web/app/components/history/*` | component | request-response | none in-repo | no analog |

## Pattern Assignments

### `queries/events.sql` — add `ListEvents` query (model, CRUD)

**Analog:** `queries/watchlist.sql` (`UpdateWatchlistPreferences`) and `queries/events.sql`'s own `ListUnnotified` comment convention.

**Convention: heavily-commented, explains non-obvious predicates** (`queries/events.sql` lines 34-40):
```sql
-- name: ListUnnotified :many
-- D-11's Phase 5 groundwork: SELECT WHERE notified_at IS NULL, ORDER BY
-- created_at ASC, id ASC for a deterministic total order (a plain
-- created_at ordering alone is not unique -- a seed cycle's rows share one
-- timestamp, see seedNotifiedAt).
SELECT * FROM events WHERE notified_at IS NULL ORDER BY created_at ASC, id ASC;
```

**Optional-filter pattern via named params** (`queries/watchlist.sql` lines 38-51, `UpdateWatchlistPreferences`):
```sql
WITH updated AS (
    UPDATE watchlist
    SET release_types = CASE
            WHEN @set_release_types::boolean THEN @release_types::text[]
            ELSE watchlist.release_types
        END,
        ...
    WHERE watchlist.id = @id
    RETURNING ...
)
```
This repo uses `@name`-style sqlc named params in `watchlist.sql` but `sqlc.narg()`/positional `$N` in `events.sql` — RESEARCH.md's proposed `ListEvents` query (already drafted, copy verbatim) uses `sqlc.narg()` + `IS NULL OR`, consistent with `events.sql`'s existing `$N` positional style:
```sql
-- name: ListEvents :many
SELECT id, artist_id, source, event_type, external_id, release_group_mbid,
       title, artist_name, release_date, cover_art_url, track_count,
       previous_track_count, release_type, notified_at, created_at
FROM events
WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id'))
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type'))
  AND (sqlc.narg('cursor')::bigint IS NULL OR id < sqlc.narg('cursor'))
ORDER BY id DESC
LIMIT sqlc.arg('page_size');
```
**Ordering pitfall to document in the new query's comment** (copy the existing `ListUnnotified` reasoning, adapted for `id DESC`, per RESEARCH.md Pattern 2 / Pitfall 2).

---

### `internal/events/service.go` (new package) (service, CRUD read-only)

**Analog:** `internal/watchlist/service.go`

**Package doc + Store-seam pattern** (lines 1-18):
```go
// Package watchlist implements the watchlist domain... It wraps the
// sqlc-generated Queries behind a narrow Store interface -- ...so handler
// tests can substitute a stub instead of a live Postgres connection.
package watchlist

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)
```
Mirror this exactly for a new `internal/events` package: `Store` interface (`List(ctx, ListParams) (Page, error)`), `Service` struct wrapping `sqlc.Querier`, `NewService(q sqlc.Querier) *Service`, `var _ Store = (*Service)(nil)`.

**List method shape — non-nil slice guarantee** (lines 180-203, `List`):
```go
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.q.ListWatchlist(ctx)
	if err != nil {
		return nil, fmt.Errorf("list watchlist: %w", err)
	}
	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, Entry{ /* field-by-field mapping */ })
	}
	return entries, nil
}
```
Apply the same shape to `events.Service.List`: wrap the sqlc error with `fmt.Errorf("list events: %w", err)`, always return a non-nil slice, map each `sqlc.ListEventsRow` field-by-field into an `events.Event` API struct (mirrors `toEntry` at lines 341-355).

**Row-struct → API-struct field mapping convention** (lines 341-355, `toEntry`):
```go
func toEntry(artist sqlc.Artist, w sqlc.Watchlist) Entry {
	return Entry{
		ID:              w.ID,
		ArtistID:        artist.ID,
		...
	}
}
```

---

### `internal/httpserver/events.go` (controller, request-response)

**Analog:** `internal/httpserver/watchlist.go` (`handleListWatchlist`) for the list-response shape; `internal/httpserver/search.go` (`handleSearch`) for query-param parsing/validation style.

**Imports pattern** (`watchlist.go` lines 1-19):
```go
package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/watchlist"
)
```

**Query-param validation pattern** (`search.go` lines 161-170, `handleSearch`):
```go
q := strings.TrimSpace(r.URL.Query().Get("q"))
if q == "" {
    writeError(w, http.StatusBadRequest, "q is required")
    return
}
if utf8.RuneCountInString(q) > maxSearchQueryRunes {
    writeError(w, http.StatusBadRequest, "q is too long")
    return
}
```
Apply the same style to `GET /events`'s `artist_id` (parse int64, mirrors `parseWatchlistID` in `watchlist.go` lines 101-108), `event_type` (validate against `watchlist.EventTypes` allow-list, mirrors lines 204-211), `cursor` (parse int64, optional), `limit`/`page_size` (parse int, clamp to a max — new logic, no direct analog but same "validate before store call" style).

**List-response, bare-envelope pattern** (`watchlist.go` lines 253-267, `handleListWatchlist`):
```go
func (s *Server) handleListWatchlist(w http.ResponseWriter, r *http.Request) {
	entries, err := s.watchlist.List(r.Context())
	if err != nil {
		httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if entries == nil {
		entries = []watchlist.Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(entries)
}
```
`handleListEvents` follows this exactly but wraps the slice in an envelope (`eventsResponse{Events, NextCursor}` per RESEARCH.md Code Examples) instead of a bare array — the one deliberate deviation, needed because pagination requires a `next_cursor` field alongside the array.

**Error handling / `errorResponse` shape — reuse directly, do not redefine** (`watchlist.go` lines 52-66):
```go
type errorResponse struct {
	Error string `json:"error"`
}
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: msg})
}
```
`events.go` calls the existing `writeError` — already package-scoped in `httpserver`, no new definition needed.

---

### `internal/httpserver/server.go` (route wiring)

**Analog:** itself — extend `New()`.

**Route registration pattern** (lines 70-76):
```go
r.Get("/health", s.handleHealth)
r.Get("/search", s.handleSearch)
r.Post("/watchlist", s.handleAddWatchlist)
r.Get("/watchlist", s.handleListWatchlist)
r.Patch("/watchlist/{id}", s.handleUpdateWatchlist)
r.Delete("/watchlist/{id}", s.handleRemoveWatchlist)
```
Add `r.Get("/events", s.handleListEvents)` in this same block, then `r.NotFound(spaHandler)` after all explicit routes are registered (per RESEARCH.md Pattern 3 — explicit routes always match first, no ordering conflict). `Server` struct (lines 26-32) gains an `events events.Store` field alongside `watchlist watchlist.Store`, and `New()`'s signature gains an `eventsStore events.Store` parameter — mirrors how `watchlist.Store` was already threaded through.

---

### `internal/httpserver/events_test.go` (test)

**Analog:** `internal/httpserver/watchlist_test.go`

**Stub-double pattern** (lines 41-80):
```go
type stubStore struct {
	addFunc    func(ctx context.Context, p watchlist.AddParams) (watchlist.Entry, error)
	listFunc   func(ctx context.Context) ([]watchlist.Entry, error)
	...
}
func (s stubStore) List(ctx context.Context) ([]watchlist.Entry, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx)
	}
	return nil, nil
}
var _ watchlist.Store = stubStore{}
```
Build a file-local `stubEventsStore` in `events_test.go` following this exact func-field-per-method shape, plus `var _ events.Store = stubEventsStore{}`.

**Dual-mode testing (unit against stub + integration against real Postgres via `testutil.NewTestPool`)** — same file imports `internal/testutil` and `crypto/sha256`-derived unique test data helper (`testMBID`, lines 31-39); mirror with a `testArtistName`/similar helper if event fixtures need uniqueness per test.

---

### `cmd/server/main.go` (wiring, modify)

**Analog:** itself.

**Service construction + wiring pattern** (lines 94, 129-132):
```go
store := watchlist.NewService(sqlc.New(pool))
...
srv := httpserver.New(pool, store, []httpserver.SearchSource{
	httpserver.NewMusicBrainzSource(mbClient),
	httpserver.NewDeezerSource(dzClient),
}, logger)
```
Add `eventsStore := events.NewService(sqlc.New(pool))` alongside `store`, then thread it into `httpserver.New(...)`'s new parameter. Follows the established "one `sqlc.New(pool)` instance per domain service, sharing the connection pool" convention already used three times in this file (store, detector, notifier).

---

### `internal/webassets/embed.go` (or similar — new, no in-repo analog)

**No analog found.** Use RESEARCH.md Pattern 3 verbatim as the starting point (already vetted against chi + go:embed conventions, cross-referenced across multiple external sources since no chi-specific canonical source exists):
```go
//go:embed all:build/client
var webFS embed.FS

func spaHandler() (http.Handler, error) {
    sub, err := fs.Sub(webFS, "build/client")
    if err != nil {
        return nil, err
    }
    fileServer := http.FileServer(http.FS(sub))
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if _, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/")); err != nil {
            r = cloneRequestWithPath(r, "/")
        }
        fileServer.ServeHTTP(w, r)
    }), nil
}
```
Registered via `r.NotFound(spaHandlerInstance)` in `server.go`'s `New()`.

---

### Frontend files (`web/app/**`) — no in-repo analog, greenfield

No existing frontend code exists in this repo. Do not search for analogs further — RESEARCH.md's Recommended Project Structure, Pattern 1 (SPA Mode config), and Code Examples (optimistic PATCH with rollback, `ListEvents` handler shape) are the canonical source for these files. `06-UI-SPEC.md`'s Design System/Color/Typography/Spacing/Copywriting sections are the canonical source for all visual/copy decisions. Treat both documents as authoritative in place of a codebase analog.

## Shared Patterns

### Error response shape (backend)
**Source:** `internal/httpserver/watchlist.go` lines 52-66 (`errorResponse`, `writeError`)
**Apply to:** `events.go` — reuse the existing package-scoped `writeError`, do not redefine.

### httplog error logging (never leak raw error text)
**Source:** `internal/httpserver/watchlist.go` lines 237-240, 255-258 (repeated pattern)
```go
httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))
writeError(w, http.StatusInternalServerError, "internal error")
```
**Apply to:** `handleListEvents` — use `slog.String("events_error", err.Error())` as the attr key, fixed "internal error" message to the client.

### Store-interface seam (narrow interface over sqlc.Querier)
**Source:** `internal/watchlist/service.go` lines 83-104 (`Store`, `Service`, `NewService`, `var _ Store = (*Service)(nil)`)
**Apply to:** New `internal/events` package — same shape, same "narrower than sqlc's generated Querier so tests can substitute a stub" rationale.

### Non-nil slice / empty-array-not-null JSON guarantee
**Source:** `internal/watchlist/service.go` lines 175-203 (`List`); `internal/httpserver/watchlist.go` lines 260-262
**Apply to:** `events.Service.List` and `handleListEvents` — an empty events page must encode as `"events": []`, never `null`.

### sqlc optional-filter query pattern (`IS NULL OR` / `sqlc.narg`)
**Source:** `queries/watchlist.sql` lines 38-56 (`@`-named-param CASE style); RESEARCH.md Pattern 2 (`sqlc.narg()` + `IS NULL OR` style, consistent with `events.sql`'s existing `$N` convention)
**Apply to:** `queries/events.sql`'s new `ListEvents` query — use the `sqlc.narg()` + `IS NULL OR` form since that matches `events.sql`'s existing style more closely than `watchlist.sql`'s `@name` form.

### Keyset pagination on `id`, never `created_at` alone
**Source:** `queries/events.sql` lines 34-40 (`ListUnnotified`'s documented pitfall — seed-mode batches share one `created_at`)
**Apply to:** `ListEvents` — order/cursor on `id DESC` only.

### Frontend: plain fetch + component state, optimistic update + rollback
**Source:** RESEARCH.md Code Examples ("Optimistic preference toggle with rollback (D-12)") — no in-repo analog since frontend is greenfield.
**Apply to:** `web/app/components/watchlist/PreferenceToggles.tsx` and any other component performing a PATCH/DELETE with optimistic UI.

### Frontend: SPA Mode config (`ssr: false`) — mandatory first task
**Source:** RESEARCH.md Pattern 1 / Pitfall 1.
**Apply to:** `web/react-router.config.ts` — must be verified/set immediately after `shadcn init`, before any route code is written.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/webassets/embed.go` (naming TBD) | middleware/static-serving | file-I/O | No `go:embed`-based static serving exists yet anywhere in this repo — first such wiring. Use RESEARCH.md Pattern 3 (synthesized from external sources) as the canonical source. |
| `web/app/**` (entire frontend tree — routes, components, `lib/api.ts`, `app.css`) | component/utility/route | request-response, streaming-like | Confirmed greenfield: no `web/`, `frontend/`, or `ui/` directory, no `package.json`, no React/Vite/Tailwind anywhere in this repo prior to this phase. Use RESEARCH.md's Recommended Project Structure, Pattern 1, Code Examples, and `06-UI-SPEC.md`'s full Design System/Copywriting Contract as the canonical source instead of a codebase analog. |
| `internal/httpserver` dev-mode CORS/proxy handling | config | request-response | No dev-server proxy or CORS middleware exists in this backend-only-until-now repo; RESEARCH.md Pitfall 4 recommends a Vite dev-server proxy (frontend-side, zero backend changes) over adding CORS middleware — treat this as a frontend `vite.config.ts` concern, not a backend pattern to map. |

## Metadata

**Analog search scope:** `internal/httpserver/`, `internal/watchlist/`, `queries/`, `internal/db/sqlc/`, `cmd/server/`
**Files scanned:** `server.go`, `search.go`, `watchlist.go`, `watchlist_test.go`, `events.sql`, `watchlist.sql`, `service.go` (watchlist), `models.go` (sqlc), `main.go`
**Pattern extraction date:** 2026-08-10
