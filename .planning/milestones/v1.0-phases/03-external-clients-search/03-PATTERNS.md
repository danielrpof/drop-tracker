# Phase 3: External Clients & Search - Pattern Map

**Mapped:** 2026-08-07
**Files analyzed:** 12
**Analogs found:** 12 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|-----------------|---------------|
| `internal/musicbrainz/client.go` | service (external API client) | request-response | `internal/httpserver/server.go` (`Pinger` narrow-interface seam) | role-match |
| `internal/musicbrainz/search.go` | service | request-response | `internal/watchlist/service.go` (`Store`/`Service` method shape) | role-match |
| `internal/musicbrainz/releasegroups.go` | service | request-response | `internal/watchlist/service.go` (`Service.List`) | role-match |
| `internal/musicbrainz/client_test.go` | test | request-response | `internal/httpserver/watchlist_test.go` (stub-double pattern) | role-match |
| `internal/deezer/client.go` | service (external API client) | request-response | `internal/musicbrainz/client.go` (sibling client, same phase) | exact (once built) |
| `internal/deezer/search.go` | service | request-response | `internal/musicbrainz/search.go` | exact (once built) |
| `internal/deezer/albums.go` | service | request-response | `internal/musicbrainz/releasegroups.go` | exact (once built) |
| `internal/deezer/client_test.go` | test | request-response | `internal/musicbrainz/client_test.go` | exact (once built) |
| `internal/httpserver/search.go` | controller (HTTP handler) | request-response | `internal/httpserver/watchlist.go` (`handleListWatchlist`, `writeError`, `decodeJSONBody`) | exact |
| `internal/httpserver/search_test.go` | test | request-response | `internal/httpserver/watchlist_test.go` (`stubStore` double + httptest) | exact |
| `internal/httpserver/server.go` (modified — register `/search` route + wire clients) | controller wiring | request-response | itself (existing `New()`) | exact |
| `internal/poller/poller.go` | service (background scheduler) | event-driven (cron-triggered) | `cmd/server/main.go` (`run()` sequencing, signal/shutdown pattern) | partial-match (no existing cron/background-job analog in repo) |
| `internal/poller/poller_test.go` | test | event-driven | `internal/httpserver/watchlist_test.go` (stub-double + table-driven pattern) | role-match |
| `cmd/server/main.go` (modified — wire clients + poller start/stop) | config/wiring | request-response + event-driven | itself (existing `run()`) | exact |

## Pattern Assignments

### `internal/musicbrainz/client.go` / `internal/deezer/client.go` (service, request-response)

**Analog:** `internal/httpserver/server.go` (narrow-interface seam) + `internal/config/config.go` (already-stubbed config fields)

**Narrow-interface seam pattern** (`internal/httpserver/server.go` lines 17-24):
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
Apply the same shape for `musicbrainz.ArtistSearcher` / `musicbrainz.ReleaseGroupLister` and `deezer.ArtistSearcher` / `deezer.AlbumLister` — small interfaces the search handler and poller depend on, not concrete `*Client` types. This mirrors `watchlist.Store` (`internal/watchlist/service.go` lines 83-92) exactly — same seam, third instance of the pattern in this codebase.

**Config already stubbed for these clients** (`internal/config/config.go` lines 27-34):
```go
MusicBrainzUserAgent       string  `env:"MUSICBRAINZ_USER_AGENT" envDefault:"drop-tracker/0.1.0 (+https://github.com/danielrpof/drop-tracker)"`
MusicBrainzRateLimitPerSec float64 `env:"MUSICBRAINZ_RATE_LIMIT_PER_SEC" envDefault:"1"`
DeezerRateLimitPer5s       int     `env:"DEEZER_RATE_LIMIT_PER_5S" envDefault:"50"`
PollInterval               time.Duration `env:"POLL_INTERVAL" envDefault:"15m"`
```
`NewClient` constructors read these directly from `*config.Config` fields passed in by `cmd/server/main.go` — no new config fields needed for Phase 3 (per CONTEXT.md code_context).

**Doc-comment convention:** every exported type/func in this codebase carries a multi-sentence doc comment explaining *why*, not just *what* (see `Pinger`, `Store`, `Service.Add` above) — reference decision IDs (e.g. "D-07", "D-11") in comments the same way existing files cite "T-02-05", "D-09", etc.

---

### `internal/musicbrainz/search.go`, `internal/musicbrainz/releasegroups.go` (and Deezer equivalents) (service, request-response)

**Analog:** `internal/watchlist/service.go` — `Service` struct wrapping a narrow dependency, exported methods returning typed structs + error, sentinel errors for expected failure modes.

**Core method pattern** (`internal/watchlist/service.go` lines 180-203, `List`):
```go
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.q.ListWatchlist(ctx)
	if err != nil {
		return nil, fmt.Errorf("list watchlist: %w", err)
	}

	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, Entry{ /* field mapping */ })
	}
	return entries, nil
}
```
Apply this shape to `SearchArtists`/`ReleaseGroupsByArtist`/`ArtistAlbums`: always return a non-nil, `make([]T, 0, ...)`-allocated slice, wrap errors with `fmt.Errorf("<verb> <noun>: %w", err)`, decode into small first-class structs (never `map[string]any`) per RESEARCH.md's "Don't Hand-Roll" guidance.

**Rate-limited request helper** (from RESEARCH.md Pattern 2, to be implemented net-new — no existing analog since this is the first outbound-HTTP-client package in the repo):
```go
func (c *Client) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent) // MusicBrainz only
	req.Header.Set("Accept", "application/json")
	return c.httpClient.Do(req)
}
```

---

### `internal/httpserver/search.go` (controller, request-response)

**Analog:** `internal/httpserver/watchlist.go` — `handleListWatchlist`, `writeError`, `decodeJSONBody`.

**Imports pattern** (`internal/httpserver/watchlist.go` lines 1-19):
```go
package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/watchlist"
)
```
`search.go` follows the same import shape, swapping `watchlist` for `musicbrainz`/`deezer`.

**Error response shape (D-13, reuse verbatim — do not redefine)** (`internal/httpserver/watchlist.go` lines 52-66):
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
This helper is already package-level in `httpserver` — `search.go` calls it directly, no duplication.

**Simple-GET-handler pattern to mirror for combined search response** (`internal/httpserver/watchlist.go` lines 253-267, `handleListWatchlist`):
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
For `GET /search` (D-03 partial-results-on-failure), call both clients, capture each error into a per-source status field instead of failing the whole request, and log the raw error via `httplog.SetAttrs(..., slog.String("musicbrainz_error", err.Error()))` / `"deezer_error"` — never put raw upstream error text in the JSON response body (matches V13 threat mitigation in RESEARCH.md and the `writeError`/`SetAttrs` split already used everywhere else in this file).

**Input validation / length-cap pattern to reuse for the `q` param** (`internal/httpserver/watchlist.go` lines 26-33, 142-158):
```go
const maxNameRunes = 512
...
name := strings.TrimSpace(req.Name)
if name == "" { ... }
if utf8.RuneCountInString(name) > maxNameRunes { ... }
```
Apply identically to the `q` query param: trim, reject empty, rune-count cap (mirrors V5 Input Validation control in RESEARCH.md's Security Domain section — mitigates the "unbounded q param" DoS pattern).

---

### `internal/httpserver/server.go` (modified — route registration)

**Analog:** itself, existing `New()` (lines 46-73).

**Route registration pattern** (lines 65-69):
```go
r.Get("/health", s.handleHealth)
r.Post("/watchlist", s.handleAddWatchlist)
r.Get("/watchlist", s.handleListWatchlist)
r.Patch("/watchlist/{id}", s.handleUpdateWatchlist)
r.Delete("/watchlist/{id}", s.handleRemoveWatchlist)
```
Add `r.Get("/search", s.handleSearch)` in the same flat, unprefixed style (D-01 explicitly calls out matching this convention). `Server` struct (lines 27-31) gains two new fields — narrow-interface types (`musicbrainz.ArtistSearcher`, `deezer.ArtistSearcher`) — following the existing pattern of `db Pinger` / `watchlist watchlist.Store` as separate, independently-typed dependencies rather than one widened interface (explicitly called out in the doc comment at lines 43-45 as the reason `store` is kept separate from `Pinger`).

---

### `internal/poller/poller.go` (service, event-driven / cron-triggered)

**Analog:** `cmd/server/main.go`'s `run()` — closest existing analog for "long-running background component with graceful start/stop tied to `ctx`," even though no cron/background-job package exists yet in this repo (RESEARCH.md's Pattern 3 code example is the primary source here since there's no direct in-repo precedent).

**Graceful shutdown pattern to mirror** (`cmd/server/main.go` lines 63-70, 101-115):
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
...
select {
case err := <-serveErr:
	...
case <-ctx.Done():
	logger.Info("shutdown signal received, shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
```
The poller's `Stop()` sequencing (per RESEARCH.md Pitfall 4) should mirror this exact shape: `stopCtx := cronScheduler.Stop(); <-stopCtx.Done()` inserted into `cmd/server/main.go`'s shutdown branch, before `pool.Close()` (deferred at line 80) runs — same "signal → drain → close resources" ordering already established.

**Overlap-guard + independent-cron-job pattern:** no in-repo analog; use RESEARCH.md's Pattern 3 code example verbatim (`atomic.Bool` `CompareAndSwap` guard per source, two separate `c.AddFunc` registrations, `@every <duration>` spec from `cfg.PollInterval`).

**Structured log-only poll result (D-04)** — follow `httplog.SetAttrs`/`slog` field-naming convention already used for `watchlist_error`/`db_error`: use `logger.Info("poll result", "artist", artist.Name, "source", "musicbrainz", "item_count", len(groups))` — plain `*slog.Logger` calls (not `httplog.SetAttrs`, since the poller has no HTTP request context) per CONTEXT.md's Claude's Discretion note.

**Deezer nil-`deezer_id` skip (D-06, Pitfall 3)** — `watchlist.Entry.DeezerID` is already `*string` (`internal/watchlist/service.go` line 54):
```go
if artist.DeezerID == nil {
	logger.Debug("skipping deezer poll: no deezer_id", "artist", artist.Name)
	continue
}
```

---

### `cmd/server/main.go` (modified — wire clients + poller)

**Analog:** itself, existing `run()` (lines 55-116).

**Dependency-construction-and-wiring pattern** (lines 82-83):
```go
store := watchlist.NewService(sqlc.New(pool))
srv := httpserver.New(pool, store, logger)
```
Extend identically: construct `mbClient := musicbrainz.NewClient(cfg.MusicBrainzUserAgent, mbLimiter, nil)`, `dzClient := deezer.NewClient(dzLimiter, nil)`, pass both into `httpserver.New(pool, store, mbClient, dzClient, logger)`, then construct and `Start()` the poller with `store`, both clients, `cfg.PollInterval`, and `logger`. Insert the poller's `Stop()`/drain call into the existing `case <-ctx.Done():` branch (line 107-114), before `pool.Close()`'s deferred call fires.

---

### Test files (`*_test.go` for all new packages)

**Analog:** `internal/httpserver/watchlist_test.go` — stub-double pattern (lines 41-80).

**Stub-double pattern**:
```go
type stubStore struct {
	addFunc func(ctx context.Context, p watchlist.AddParams) (watchlist.Entry, error)
	...
}

func (s stubStore) Add(ctx context.Context, p watchlist.AddParams) (watchlist.Entry, error) {
	if s.addFunc != nil {
		return s.addFunc(ctx, p)
	}
	return watchlist.Entry{}, nil
}
...
var _ watchlist.Store = stubStore{}
```
Apply the same shape for `stubArtistSearcher`/`stubReleaseGroupLister` in `internal/httpserver/search_test.go` (search handler tests use stubs, not real clients — httptest.Server-backed fakes belong in `internal/musicbrainz/client_test.go` / `internal/deezer/client_test.go` per CLAUDE.md's testing constraint). `testMBID(t)` helper (lines 31-39, sha256-of-test-name) is reusable verbatim for any test needing a unique-per-test identifier.

**httptest.Server fake pattern for the client packages** — no direct in-repo precedent (this is the first package hitting a real external HTTP API); use RESEARCH.md's live-verified JSON response bodies (MusicBrainz artist-search, release-group-browse; Deezer artist-search, artist-albums) as fixture literals inside `httptest.NewServer(http.HandlerFunc(...))` handlers, following the same "stub returns canned response, assert client parses correctly" shape as `stubStore`.

## Shared Patterns

### Error response / no-leak-of-downstream-errors
**Source:** `internal/httpserver/watchlist.go` lines 52-66 (`writeError`) and lines 238, 256, 322, 349 (`httplog.SetAttrs(..., slog.String("<domain>_error", err.Error()))` before every 500)
**Apply to:** `internal/httpserver/search.go` — raw MusicBrainz/Deezer error text must never reach the JSON response body (D-03 "per-source status/error flag", RESEARCH.md V13 control); log the real error via `httplog.SetAttrs`, return a generic per-source flag/message in the body.

### Narrow-interface dependency seam
**Source:** `internal/httpserver/server.go` lines 17-24 (`Pinger`), `internal/watchlist/service.go` lines 83-92 (`Store`)
**Apply to:** `internal/musicbrainz`, `internal/deezer` client packages (`ArtistSearcher`, etc.), `internal/poller` (depends on `watchlist.Store` + both narrow client interfaces, never concrete `*Client` types).

### Structured logging with slog + httplog.SetAttrs
**Source:** `internal/httpserver/watchlist.go` (`httplog.SetAttrs` used only inside HTTP handlers, request-scoped); `internal/logging/logging.go` (base `*slog.Logger` construction, not read in full this pass but referenced by `cmd/server/main.go` line 61 `logging.New(cfg)`)
**Apply to:** `internal/httpserver/search.go` uses `httplog.SetAttrs`; `internal/poller` (non-HTTP, cron-triggered) uses plain `logger.Info`/`logger.Warn`/`logger.Debug` calls instead, since there is no request context to attach attrs to (per CONTEXT.md code_context: "it needs its own slog call sites rather than relying on the HTTP middleware chain").

### Config-driven construction, no new required fields
**Source:** `internal/config/config.go` lines 24-34
**Apply to:** All new client/poller wiring in `cmd/server/main.go` — `MusicBrainzUserAgent`, `MusicBrainzRateLimitPerSec`, `DeezerRateLimitPer5s`, `PollInterval` already exist and are optional-with-defaults; Phase 3 is the first consumer, no `config.go` edits expected.

### Doc-comment style: explain "why", cite decision IDs
**Source:** pervasive across `internal/watchlist/service.go`, `internal/httpserver/watchlist.go`, `cmd/server/main.go`
**Apply to:** All new exported types/functions — reference the relevant CONTEXT.md decision ID (D-07, D-09, D-10, etc.) in the doc comment the way existing code cites "D-09", "T-02-05", "WR-03".

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/poller/poller.go` (cron wiring, overlap guard) | service | event-driven | No existing background/cron job exists in the repo — `cmd/server/main.go`'s `run()` is the closest structural analog (graceful shutdown shape) but nothing here today registers a recurring job; RESEARCH.md's Pattern 3 code example is the primary source instead. |
| `internal/musicbrainz/client.go`, `internal/deezer/client.go` (outbound HTTP + rate limiting mechanics) | service | request-response | First outbound external-HTTP-client packages in the repo — no existing `net/http.Client`-wrapping code or `rate.Limiter` usage to copy from; RESEARCH.md Pattern 2 (`doRequest` helper) is the primary source. |

## Metadata

**Analog search scope:** `internal/httpserver/`, `internal/watchlist/`, `internal/config/`, `cmd/server/`
**Files scanned:** `server.go`, `health.go`, `watchlist.go`, `watchlist_test.go`, `service.go`, `config.go`, `main.go` (7 files read in full)
**Pattern extraction date:** 2026-08-07
