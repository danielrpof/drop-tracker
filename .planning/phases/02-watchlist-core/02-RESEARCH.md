# Phase 2: Watchlist Core - Research

**Researched:** 2026-08-05
**Domain:** Go/chi CRUD API over Postgres (sqlc + pgx/v5), array-typed preference columns, httptest-based handler testing
**Confidence:** MEDIUM-HIGH (in-repo conventions VERIFIED by reading Phase 1 source; sqlc/pgx array & error-handling behavior CITED from official docs fetched this session; two items flagged ASSUMED pending planner/user confirmation)

## Summary

Phase 2 adds the first two domain tables (`artists`, `watchlist`) and a `/watchlist` CRUD resource on top of a Phase 1 skeleton that already has the exact seam pattern this phase needs: `httpserver.Pinger` is a narrow, single-method interface that lets `health_test.go` swap in a stub instead of a live Postgres connection `[VERIFIED: internal/httpserver/server.go:15-22]`. This phase should mirror that pattern with a second, equally narrow interface for watchlist persistence — a `Store` (or similarly named) interface defined in a new `internal/watchlist` package, implemented by a thin wrapper around the sqlc-generated `*sqlc.Queries`, and consumed by `internal/httpserver/watchlist.go` exactly the way `s.db Pinger` is consumed by `handleHealth`.

The two CONTEXT.md-deferred implementation questions both have clear, low-risk answers backed by official documentation fetched this session: (1) use plain `text[]` columns with a Postgres `CHECK ... <@ ARRAY[...]` constraint, not a native `enum` type, for both `release_types` and `muted_event_types` — sqlc's own datatype reference confirms arrays map cleanly to `[]string` while enum-array support is unaddressed/unclear in its docs, and the wider Postgres community consensus (multiple independent sources) is that `CHECK`-constrained text avoids the `ALTER TYPE ... ADD VALUE` transaction-visibility restriction that native enums carry — a real risk here since `release_types`/`muted_event_types` are exactly the kind of value set likely to grow later; (2) sqlc's `emit_interface: true` config option (currently unset in `sqlc.yaml` `[VERIFIED: sqlc.yaml:1-12]`) generates a `Querier` interface automatically, which is the natural foundation for the `Store` seam described above.

**Primary recommendation:** Add `emit_interface: true` and `emit_pointers_for_null_types: true` to `sqlc.yaml`; model `artists`/`watchlist` as two tables with `text[]` + `CHECK` constraint columns (not enums); build a thin `internal/watchlist` service package that wraps `*sqlc.Queries`, translates `pgconn.PgError` (code `23505`) into a sentinel `ErrDuplicate` for the 409 path, and is consumed by `internal/httpserver/watchlist.go` through a narrow interface — following, not deviating from, Phase 1's `Pinger` precedent.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Watchlist CRUD HTTP handlers (routing, request decode/validate, status codes) | API / Backend | — | `internal/httpserver` is the only HTTP-facing tier in this single-binary service `[VERIFIED: internal/httpserver/server.go:1-13]` |
| Duplicate-add / not-found business rules (409/404 semantics) | API / Backend | Database / Storage | Enforced in Go (translating pg error codes) with a DB constraint as the backstop — same defense-in-depth shape as Phase 1's redaction logic |
| Artist master-data identity (mbid uniqueness, upsert-on-known-mbid) | Database / Storage | API / Backend | Postgres `UNIQUE` constraint is the source of truth; Go code only orchestrates the upsert call |
| Release-type / mute preference value validation | API / Backend | Database / Storage | App-level allow-list check gives a friendly 400 message; DB `CHECK` constraint is the non-bypassable backstop |
| Schema migration (new tables) | Database / Storage | — | `internal/db/migrations`, applied via the existing `RunMigrations` retry-loop machinery `[VERIFIED: internal/db/migrate.go:111-165]` — no changes needed to that machinery itself |

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WLST-02 | Add artist to watchlist | Upsert-artist-by-mbid + insert-watchlist-row pattern (Code Examples); 409 on duplicate via `pgconn.PgError` code `23505` (Pitfall 2, Code Examples) |
| WLST-03 | Remove artist from watchlist | Hard `DELETE` using sqlc `:execrows` annotation to detect 0-rows-affected → 404 (Code Examples) |
| WLST-04 | List all watchlisted artists | `JOIN watchlist a ON artists` query with explicitly aliased columns to avoid `id` collision (Pitfall 4) |
| WLST-05 | Per-artist release-type filters | `release_types text[]` column + Go/DB allow-list validation (Standard Stack, Architecture Patterns) |
| WLST-06 | Per-artist mute preferences | `muted_event_types text[]` column, same validation pattern as WLST-05 |

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: MusicBrainz ID (MBID) is required to add an artist — the stable external key the whole pipeline keys off of. Name-only or Deezer-only entries are not supported. Reversibility: one-way.
- D-02: Store a nullable `deezer_id` column on the artist record now, even though Phase 3's Deezer client is what populates it. Avoids a Phase 3 schema migration.
- D-03: `artists` is a separate master-data table (mbid, deezer_id, name, metadata) from `watchlist` (the user's entry: artist_id FK + preferences). Reversibility: one-way.
- D-04: Both MBID and name are required fields on add.
- D-05: Release-type filters (WLST-05) and mute preferences (WLST-06) are two distinct axes, not one unified structure. Reversibility: costly.
- D-06: Release-type filter stored as a Postgres array column (`release_types text[]` or a Postgres enum array) on the watchlist row.
- D-07: Mute preference stored the same way — `muted_event_types text[]`.
- D-08: Default on add: everything on (all release types enabled, nothing muted).
- D-09: Re-adding an artist already on the watchlist returns `409 Conflict` with a clear error message. Not idempotent.
- D-10: Removal is a hard delete of the watchlist row.
- D-11: Single `/watchlist` resource with preferences embedded — `POST /watchlist`, `GET /watchlist`, `PATCH /watchlist/{id}`, `DELETE /watchlist/{id}`.
- D-12: Request/response bodies are plain JSON objects — no `{"data": ...}` envelope.
- D-13: Validation/error responses are `{"error": "message"}` JSON bodies with the appropriate HTTP status code (400/404/409) — not RFC 7807.
- D-14: No `/api` prefix — routes are flat (`/watchlist`, not `/api/watchlist`).

### Claude's Discretion
- Exact column names/types beyond what's specified above (e.g., timestamps, any additional artist metadata fields like image URL or disambiguation comment).
- Whether the release-type/event-category values are a Postgres native `enum` type vs. plain `text[]` with application-level validation.
- Exact validation error messages and Go error-handling patterns (following Phase 1's established conventions).

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.
</user_constraints>

## Standard Stack

No new external Go modules are required for this phase — every dependency needed is already in `go.mod` `[VERIFIED: go.mod:1-20]`.

### Core (existing, extended)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/jackc/pgx/v5` | v5.10.0 | Postgres driver + array/text encoding | Already locked; `pgconn.PgError` is how a unique-violation (409 source) is detected `[CITED: github.com/jackc/pgx issue tracker + Medium writeup on errors.As(&pgErr)]` |
| `github.com/sqlc-dev/sqlc` (CLI) | v1.31.1 | Codegen for the two new tables' queries | Already locked and version-pinned via `sqlc-version-check` `[VERIFIED: Makefile:9-13,31-35]`; confirmed still installed at v1.31.1 this session (`sqlc version` → `v1.31.1`) |
| `github.com/go-chi/chi/v5` | v5.3.1 | Route registration for `/watchlist` | Already locked; `r.Get/Post/Patch/Delete("/watchlist...", handler)` + `chi.URLParam(r, "id")` is the idiomatic pattern `[CITED: go-chi/chi README]` |
| `github.com/jackc/pgerrcode` | (indirect → promote to direct) | Named constant for SQLSTATE `23505` (`pgerrcode.UniqueViolation`) | Already an indirect dependency (pulled in transitively) at `v0.0.0-20220416144525-469b46aa5efa` `[VERIFIED: go.mod:14, go list -m all output]`; a newer pseudo-version (`v0.0.0-20250907135507-afb5586c32a6`, Sept 2025) exists upstream `[CITED: pkg.go.dev/github.com/jackc/pgerrcode]` — planner should `go get -u` it when promoting to a direct import so `go.mod` reflects an explicit, current version rather than a stale transitively-pinned one |

### sqlc.yaml additions needed
| Option | Current value | Recommended value | Why |
|--------|---------------|--------------------|-----|
| `emit_interface` | unset (defaults `false`) `[VERIFIED: sqlc.yaml:6-12]` | `true` | Generates a `Querier` interface covering every query method (including the existing `Ping`) — the foundation for the `internal/watchlist` seam `[CITED: docs.sqlc.dev/en/stable/reference/config.html — "If true, output a Querier interface in the generated package. Defaults to false."]` |
| `emit_pointers_for_null_types` | unset (defaults `false`) | `true` | Nullable columns (`artists.deezer_id`, `disambiguation`, `image_url`) generate as `*string` instead of `pgtype.Text`, which marshals to plain JSON `null`/string automatically — matches D-12's plain-JSON-object convention without a custom `MarshalJSON` `[CITED: docs.sqlc.dev config reference — "generated types for nullable columns are emitted as pointers (ie. *string) instead of database/sql null types"]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `text[]` + `CHECK` constraint for release/event types | Native Postgres `enum` type (array-of-enum) | Enum gives 4-byte storage and DB-level type safety, but `ALTER TYPE ... ADD VALUE` has transaction-visibility restrictions in older Postgres and sqlc's own docs don't clearly address enum-array codegen — not worth the risk for a value set explicitly expected to be revisited (WLST-05/06 already name 3-4 values today) |
| Thin `internal/watchlist` service wrapping sqlc `Queries` | Handlers call `*sqlc.Queries` directly | Direct calls are less code short-term, but break the `Pinger`-style testability seam Phase 1 established and CONTEXT.md explicitly asked this research to preserve — handler tests would then require a live Postgres for every case, including 400/409 validation branches that shouldn't need one |
| `BIGSERIAL` integer PKs for `artists`/`watchlist` | UUID PKs | D-11's example JSON body shows a plain `"id"` field distinct from `"mbid"`; nothing in the requirements needs UUID's distributed-generation property for a single-operator app, and integer PKs are simpler to reason about in `PATCH/DELETE /watchlist/{id}` path params |

**Installation:** No `go get` needed for new packages. Run `sqlc generate` after editing `sqlc.yaml` and adding the new `.sql` query files (see Code Examples).

## Package Legitimacy Audit

No new external packages are introduced by this phase. The one dependency this phase newly *imports directly* (`github.com/jackc/pgerrcode`) is already present in `go.sum` as a transitive dependency since Phase 1 `[VERIFIED: go.mod:14]` — pulled in via the `jackc/*` module family already vetted for `pgx`/`pgconn`. Promoting it from indirect to direct is not a new-package install; it is authored and maintained under the same `jackc` GitHub org as `pgx` itself.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none
**Packages requiring `checkpoint:human-verify`:** none — no new package installs this phase

## Architecture Patterns

### System Architecture Diagram

```
Client (curl / future SPA)
        │
        │ HTTP JSON (POST/GET/PATCH/DELETE /watchlist...)
        ▼
┌───────────────────────────────────────────┐
│ chi router (internal/httpserver)          │
│  middleware.RequestID → echoRequestID →    │
│  httplog.RequestLogger → Recoverer         │  [VERIFIED: internal/httpserver/server.go:42-56]
└───────────────────┬───────────────────────┘
                     │ decode + validate JSON body
                     ▼
┌───────────────────────────────────────────┐
│ watchlist.go handlers (internal/httpserver)│
│  - handleAddWatchlist                      │
│  - handleListWatchlist                     │
│  - handleUpdateWatchlist                   │
│  - handleRemoveWatchlist                   │
│  depend on narrow `watchlist.Store`        │
│  interface (this phase's Pinger-analog)    │
└───────────────────┬───────────────────────┘
                     │ domain calls (AddArtist, List, UpdatePrefs, Remove)
                     ▼
┌───────────────────────────────────────────┐
│ internal/watchlist (new package)          │
│  - translates pgconn.PgError 23505 →      │
│    ErrDuplicate (→ 409)                   │
│  - translates 0-rows-affected →           │
│    ErrNotFound (→ 404)                    │
│  - validates release_types/               │
│    muted_event_types against allow-list   │
└───────────────────┬───────────────────────┘
                     │ sqlc-generated Queries (Querier interface)
                     ▼
┌───────────────────────────────────────────┐
│ internal/db/sqlc (generated)               │
│  Ping, CreateArtist (upsert), CreateWatch- │
│  listEntry, ListWatchlist, UpdateWatch-    │
│  listPrefs, DeleteWatchlistEntry           │
└───────────────────┬───────────────────────┘
                     │ pgx/v5 (DBTX)
                     ▼
              Postgres (artists, watchlist tables)
```

### Recommended Project Structure
```
internal/
├── watchlist/                 # NEW — domain/service layer for this phase
│   ├── service.go             # Store interface, sentinel errors, Service{queries *sqlc.Queries}
│   └── service_test.go        # real-Postgres tests via testutil.NewTestPool (mirrors sqlc_test.go pattern)
├── httpserver/
│   ├── watchlist.go           # NEW — handlers, following health.go's shape
│   └── watchlist_test.go      # NEW — httptest.Server + stub watchlist.Store (mirrors health_test.go's stubPinger pattern)
├── db/
│   ├── migrations/
│   │   ├── 000002_watchlist.up.sql    # NEW
│   │   └── 000002_watchlist.down.sql  # NEW
│   └── sqlc/                  # regenerated: models.go, artists.sql.go, watchlist.sql.go, querier.go
queries/
├── artists.sql                # NEW
└── watchlist.sql              # NEW
```

### Pattern 1: Narrow store interface (Pinger-analog)
**What:** Define a small interface in `internal/watchlist` naming exactly the operations `internal/httpserver` needs (not sqlc's full generated `Querier`), so handler tests can substitute a stub without a live Postgres — exactly how `httpserver.Pinger` lets `health_test.go` avoid a real DB for the down/timeout branches `[VERIFIED: internal/httpserver/server.go:15-22, internal/httpserver/health_test.go:39-47]`.

**When to use:** Every handler that needs DB access in this phase.

**Example (new code, following the verified Pinger shape):**
```go
// internal/watchlist/service.go
package watchlist

import "context"

// Store is the minimal surface internal/httpserver needs for the watchlist
// resource — narrower than sqlc's generated Querier so a stub can implement
// it in tests without a live Postgres connection.
type Store interface {
	Add(ctx context.Context, p AddParams) (Entry, error)
	List(ctx context.Context) ([]Entry, error)
	UpdatePreferences(ctx context.Context, id int64, p PreferencesParams) (Entry, error)
	Remove(ctx context.Context, id int64) error
}

var (
	ErrDuplicate = errors.New("artist already on watchlist")
	ErrNotFound  = errors.New("watchlist entry not found")
)
```

### Pattern 2: sqlc `:execrows` for hard-delete existence check
**What:** Use the `:execrows` query annotation (returns `int64` affected-row count) so the service layer can distinguish "deleted" from "nothing to delete" without a separate `SELECT` first `[CITED: docs.sqlc.dev/en/stable/reference/query-annotations.html — ":execrows will return the number of affected rows from the result returned by ExecContext"]`.

**When to use:** `DELETE /watchlist/{id}` (WLST-03) and any future single-row update that must 404 on a missing id.

### Pattern 3: Upsert-by-mbid + insert-with-unique-constraint for 409
**What:** `artists` upserts by `mbid` (idempotent — re-adding the same artist never fails at the artist-identity layer); `watchlist` insert relies on a `UNIQUE (artist_id)` constraint, and the service layer catches SQLSTATE `23505` specifically on that constraint to return `ErrDuplicate` (D-09).

**Example (Go error translation, following the `errors.As` pattern confirmed against pgx v5's package layout):**
```go
// import "github.com/jackc/pgx/v5/pgconn" — NOT the older "github.com/jackc/pgconn"
// package, which causes errors.As to silently return false even though the
// type name looks identical. [CITED: Medium — "Why errors.As(err, &pgErr)
// returned false in my Go + PostgreSQL stack"]
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "watchlist_artist_id_key" {
	return Entry{}, ErrDuplicate
}
```

### Anti-Patterns to Avoid
- **Decoding the HTTP request body directly into the sqlc-generated model struct:** allows a client to set `id`, `artist_id`, `created_at` fields the server should own (over-posting/mass-assignment). Always decode into a purpose-built request DTO (`addRequest{MBID, Name, DeezerID, ReleaseTypes, MutedEventTypes}`) and construct the sqlc params explicitly.
- **Checking only `pgErr.Code == pgerrcode.UniqueViolation` without also checking `ConstraintName`:** the `watchlist` table may eventually have more than one unique constraint (e.g., if a per-user scope is added later); a bare code check would misattribute an unrelated unique violation to "artist already on watchlist."
- **Widening `httpserver.New`'s existing `Pinger` parameter into a fatter interface** (e.g., merging DBTX methods onto it) to avoid adding a constructor parameter: this breaks `stubPinger` in `health_test.go`, which today only implements `Ping` `[VERIFIED: internal/httpserver/health_test.go:39-47]`. Add a *second*, separate parameter for the watchlist store instead (see Pitfall 5).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Detecting a Postgres unique-constraint violation | String-matching `err.Error()` for "duplicate key" | `errors.As(err, &pgconn.PgError)` + `pgerrcode.UniqueViolation` constant | Error message text is not a stable API across Postgres versions/locales; the SQLSTATE code is `[CITED: pkg.go.dev/github.com/jackc/pgerrcode]` |
| Validating array membership against a fixed value set | A Go `switch` per value scattered across handlers | A single `map[string]bool` allow-list constant, checked once, reused by both the request-validation layer and (as a DB `CHECK` constraint) the schema | Centralizes the value set in one place per axis, matching D-05's "two distinct axes" decision |
| JSON `{"error": "..."}` response helper | A generic ad-hoc `http.Error`/`fmt.Fprintf` per handler | A single small `writeError(w, status, msg)` helper in `internal/httpserver` (new, but trivial) reused across `health.go`-style and `watchlist.go` handlers | Consistency with D-13's error-body contract; avoids drift between handlers |

**Key insight:** Nothing in this phase's domain (basic CRUD + array preference columns) needs a third-party helper library — chi + sqlc + pgx's own error types cover the entire surface. The risk here is not missing a library, it's under-using the seam pattern Phase 1 already established (see Pitfall 5).

## Common Pitfalls

### Pitfall 1: Native Postgres enum for a growing value set
**What goes wrong:** Adding a 5th release type later requires `ALTER TYPE release_type ADD VALUE 'x'`, which cannot be used in the same transaction it was added in (pre-Postgres 12 forbids it inside a transaction block entirely; even on 12+ the new value isn't visible until the adding transaction commits).
**Why it happens:** Enum values are stored as an ordered catalog entry, not just a check against a literal list.
**How to avoid:** Use `text[]` + `CHECK (col <@ ARRAY[...]::text[])` as recommended above — adding a value is a plain `ALTER TABLE ... DROP CONSTRAINT ... ADD CONSTRAINT ...` migration with no enum-catalog semantics.
**Warning signs:** A migration that touches `pg_enum` or uses `ALTER TYPE` should be a signal to re-check this decision.

### Pitfall 2: Wrong `pgconn` import path breaks `errors.As`
**What goes wrong:** `errors.As(err, &pgErr)` silently returns `false` even though the returned error genuinely is a `*pgconn.PgError`, because the code imported `github.com/jackc/pgconn` (the old, pre-v5 standalone module) instead of `github.com/jackc/pgx/v5/pgconn`.
**Why it happens:** Both modules define a type named `PgError`; only the v5-nested one is what pgx v5 actually returns.
**How to avoid:** Always import `"github.com/jackc/pgx/v5/pgconn"` — never add `github.com/jackc/pgconn` to `go.mod` for this purpose `[CITED: Medium — "Why errors.As(err, &pgErr) returned false in my Go + PostgreSQL stack (and how I fixed it)"]`.
**Warning signs:** A 500 response where a 409 was expected on a duplicate-add test.

### Pitfall 3: `pgtype.Text` leaking pgx internals into the JSON response
**What goes wrong:** Without `emit_pointers_for_null_types: true`, sqlc's pgx/v5 output represents nullable text columns (`deezer_id`, `disambiguation`, `image_url`) as `pgtype.Text{String, Valid}`. Whether this struct marshals to a plain JSON string/`null` or to `{"String":"...","Valid":true}` was not conclusively confirmed by this session's WebSearch — flagged in Assumptions Log (A1) rather than asserted either way.
**Why it happens:** `pgtype`'s JSON-marshaling behavior for scalar wrapper types has had inconsistent coverage across pgx v5 minor versions per multiple GitHub issue threads found this session.
**How to avoid:** Set `emit_pointers_for_null_types: true` in `sqlc.yaml` so nullable columns are plain `*string` — Go's stdlib `encoding/json` marshals a nil `*string` to `null` and a non-nil one to a plain string with zero custom code, sidestepping the question entirely.
**Warning signs:** A `GET /watchlist` response body containing a nested `{"String":..., "Valid":...}` object instead of a plain string/`null` for `deezer_id`.

### Pitfall 4: Ambiguous `id` column when joining `watchlist` and `artists`
**What goes wrong:** Both tables have a column named `id`. An unaliased `SELECT * FROM watchlist w JOIN artists a ON ...` produces two same-named result columns, and sqlc will either error or generate a struct with a collision/only one `ID` field, silently dropping the other.
**Why it happens:** SQL result-column naming is positional/label-based, not table-qualified, unless aliased.
**How to avoid:** Explicitly alias every column in the `ListWatchlist`/`Add`-returning queries — e.g., `w.id AS id, a.id AS artist_id, a.mbid, a.name, w.release_types, w.muted_event_types`.
**Warning signs:** `sqlc generate` producing a struct with fewer fields than expected, or two fields both named `ID`.

### Pitfall 5: Breaking `httpserver.New`'s existing call sites
**What goes wrong:** Adding watchlist DB access requires `Server` to hold a second dependency. Changing `New`'s signature (adding a parameter, or widening `Pinger` itself) breaks every existing call site: `cmd/server/main.go:80`, and four separate `httpserver.New(...)` calls inside `health_test.go` (`TestHealth_Up`, `TestHealth_Down`, `TestHealth_DownOnTimeout`, `TestHealth_Concurrent`), plus `boot_e2e_test.go:50` `[VERIFIED: cmd/server/main.go:80, internal/httpserver/health_test.go:57,86,126,151, internal/httpserver/boot_e2e_test.go:50]`.
**Why it happens:** Phase 1 designed `New(db Pinger, logger)` for a single dependency; this phase is the first to need a second one.
**How to avoid:** Add a *second, separate* parameter (e.g., `New(db Pinger, store watchlist.Store, logger *slog.Logger)`) rather than widening `Pinger`'s method set — this keeps `stubPinger` (which only implements `Ping`) valid for the four existing health tests, which can pass `nil` (or a trivial no-op stub) for the new `store` parameter since they never touch watchlist routes. Plan a task that updates all five call sites in the same commit as the `New` signature change so the build stays green.
**Warning signs:** `go build ./...` failing across multiple `_test.go` files after only touching `server.go`.

## Code Examples

### Migration (new tables)
```sql
-- Source: this session's design, following sqlc datatype/CHECK guidance
-- cited above; column choices per D-01/D-02/D-03/D-04/D-06/D-07/D-08.
CREATE TABLE artists (
    id             BIGSERIAL PRIMARY KEY,
    mbid           TEXT NOT NULL UNIQUE,
    deezer_id      TEXT,
    name           TEXT NOT NULL,
    disambiguation TEXT,
    image_url      TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE watchlist (
    id                BIGSERIAL PRIMARY KEY,
    artist_id         BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    release_types     TEXT[] NOT NULL DEFAULT ARRAY['album','single','ep','deluxe']::text[],
    muted_event_types TEXT[] NOT NULL DEFAULT '{}'::text[],
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT watchlist_artist_id_key UNIQUE (artist_id),
    CONSTRAINT watchlist_release_types_valid
        CHECK (release_types <@ ARRAY['album','single','ep','deluxe']::text[]),
    CONSTRAINT watchlist_muted_event_types_valid
        CHECK (muted_event_types <@ ARRAY['new_release','guest_feature','deluxe_change']::text[])
);
```

### sqlc queries
```sql
-- queries/artists.sql
-- name: UpsertArtist :one
INSERT INTO artists (mbid, deezer_id, name, disambiguation, image_url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (mbid) DO UPDATE
    SET name = EXCLUDED.name,
        deezer_id = COALESCE(EXCLUDED.deezer_id, artists.deezer_id),
        updated_at = now()
RETURNING *;

-- queries/watchlist.sql
-- name: CreateWatchlistEntry :one
INSERT INTO watchlist (artist_id, release_types, muted_event_types)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListWatchlist :many
SELECT w.id AS id, a.id AS artist_id, a.mbid, a.name, a.deezer_id,
       a.disambiguation, a.image_url,
       w.release_types, w.muted_event_types, w.created_at, w.updated_at
FROM watchlist w
JOIN artists a ON a.id = w.artist_id
ORDER BY a.name;

-- name: UpdateWatchlistPreferences :one
UPDATE watchlist
SET release_types = $2, muted_event_types = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteWatchlistEntry :execrows
DELETE FROM watchlist WHERE id = $1;
```
Source: this session's design; array/enum type-mapping and `:execrows` semantics `[CITED: docs.sqlc.dev/en/stable/reference/datatypes.html, docs.sqlc.dev/en/stable/reference/query-annotations.html]`.

### Handler skeleton (following health.go's shape)
```go
// internal/httpserver/watchlist.go — mirrors handleHealth's structure:
// context timeout, typed response struct, Content-Type header, JSON encode.
func (s *Server) handleAddWatchlist(w http.ResponseWriter, r *http.Request) {
	var req addWatchlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.MBID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "mbid and name are required")
		return
	}
	entry, err := s.watchlist.Add(r.Context(), req.toParams())
	switch {
	case errors.Is(err, watchlist.ErrDuplicate):
		writeError(w, http.StatusConflict, "artist already on watchlist")
		return
	case err != nil:
		httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(entry)
}
```
Source: derived from the verified `handleHealth` shape `[VERIFIED: internal/httpserver/health.go:28-44]` plus the error-translation pattern `[CITED: pgx/pgerrcode sources above]`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `pgtype.Text`'s default JSON marshaling behavior (without `emit_pointers_for_null_types`) was not conclusively confirmed this session — the recommendation to set `emit_pointers_for_null_types: true` sidesteps the question but the underlying claim about default behavior is unverified | Pitfall 3, Standard Stack | Low — the mitigation (set the config flag) is safe regardless of which way the underlying default actually behaves; risk is only wasted investigation time if the planner tries to verify the claim itself |
| A2 | "deluxe" as a WLST-05 release-type filter value does not correspond to an official MusicBrainz release-group primary-type (Album/Single/EP/Broadcast/Other) or documented secondary-type — it is being treated here as a Phase-2-local catalog-scope label that Phase 4's detection logic (DTCT-02, "deluxe/tracklist-change") must map onto its own semantics, not as a literal MusicBrainz type this phase queries against | Code Examples (CHECK constraint value list), Open Questions | Medium — if Phase 3/4 research finds MusicBrainz naming that conflicts with the `'deluxe'` string chosen here, the CHECK constraint's literal value list needs a migration to rename it |
| A3 | `ON DELETE CASCADE` from `watchlist.artist_id` to `artists.id` is a safe default even though no artist-delete endpoint exists in this phase (dead code today) — not discussed in CONTEXT.md | Code Examples (migration) | Low — no code path currently deletes an `artists` row, so this constraint has no observable effect until/unless a future phase adds one; `ON DELETE RESTRICT` would be an equally valid alternate default |
| A4 | The event-type string values chosen for `muted_event_types` (`new_release`, `guest_feature`, `deluxe_change`) are this session's own naming, anticipating Phase 4's three DTCT event kinds (DTCT-01/02/03) — Phase 4 has not been planned yet and could choose different literal names | Code Examples (CHECK constraint), Open Questions | Medium — if Phase 4 planning picks different string literals, this phase's CHECK constraint and any seeded test data need a follow-up migration to realign |

**If this table is empty:** N/A — see items above.

## Open Questions

1. **Do the `release_types` and `muted_event_types` literal string values need to be locked now, or can Phase 4 planning revise them?**
   - What we know: WLST-05 names album/single/EP/deluxe; the ROADMAP names "new release," "guest feature," and "deluxe/tracklist-change" as the three Phase 4 event kinds.
   - What's unclear: The exact string literals Phase 4's detection code will emit/compare against are not yet locked anywhere.
   - Recommendation: Treat the literal values proposed in this research (`album`/`single`/`ep`/`deluxe` and `new_release`/`guest_feature`/`deluxe_change`) as this phase's working set, documented clearly in the migration's SQL comments so Phase 4 planning can either adopt them as-is or file a small follow-up migration to rename — do not block Phase 2 on Phase 4 not existing yet.

2. **Should `PATCH /watchlist/{id}` accept partial preference updates (only `release_types` OR only `muted_event_types`) or require both fields every time?**
   - What we know: D-11 describes a single `PATCH` for "update preferences" without specifying partial-vs-full semantics.
   - What's unclear: CONTEXT.md doesn't distinguish these.
   - Recommendation: Support partial updates (only update the fields present in the JSON body, leave the other axis untouched) — this is the more common REST PATCH convention and avoids forcing a caller to re-send the axis it isn't changing; the planner should confirm this interpretation is acceptable or lock it as an explicit decision.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Postgres (via `docker compose up -d --wait postgres`) | Integration tests, `RunMigrations` | ✓ (Docker confirmed available this session) | — (compose-managed) | — |
| sqlc CLI | `make sqlc` / `make sqlc-check` | ✓ | v1.31.1 (matches `SQLC_VERSION` pin `[VERIFIED: Makefile:9-13]`) | — |
| Go toolchain | build/test | ✓ | go1.26.5 (module declares `go 1.26` `[VERIFIED: go.mod:3]`) | — |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** none

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test`), no third-party test framework |
| Config file | `Makefile` targets — no separate test-config file `[VERIFIED: Makefile:1-36]` |
| Quick run command | `go test ./... -short -race -count=1` (skips DB-backed tests per `testutil.RequirePostgresDSN`'s `testing.Short()` check `[VERIFIED: internal/testutil/postgres.go:26-31]`) |
| Full suite command | `make test` → `db-up` then `TEST_DATABASE_URL=... go test ./... -race -count=1` `[VERIFIED: Makefile:27-29]` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WLST-02 | Add artist (success, 400 invalid body, 409 duplicate) | unit (stub `watchlist.Store`) + integration (real Postgres) | `go test ./internal/httpserver/... ./internal/watchlist/... -race -count=1` | ❌ Wave 0 |
| WLST-03 | Remove artist (204, 404 on missing id) | unit + integration | same as above | ❌ Wave 0 |
| WLST-04 | List watchlist (shape, ordering, empty list) | integration (needs joined rows) | same as above | ❌ Wave 0 |
| WLST-05 | Release-type filter update + invalid-value rejection | unit (validation) + integration (persistence) | same as above | ❌ Wave 0 |
| WLST-06 | Mute preference update + invalid-value rejection | unit (validation) + integration (persistence) | same as above | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./... -short -race -count=1`
- **Per wave merge:** `make test` (full suite against real Postgres)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/db/migrations/000002_watchlist.up.sql` / `.down.sql` — new schema
- [ ] `queries/artists.sql`, `queries/watchlist.sql` — new sqlc query sources
- [ ] `sqlc.yaml` edits (`emit_interface: true`, `emit_pointers_for_null_types: true`) + `sqlc generate` regeneration
- [ ] `internal/watchlist/service.go` + `service_test.go` — new package, no existing tests to build on
- [ ] `internal/httpserver/watchlist.go` + `watchlist_test.go` — new handlers, following `health.go`/`health_test.go` shape
- [ ] Updates to the 5 existing `httpserver.New(...)` call sites for the new constructor parameter (Pitfall 5)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Single-operator deployment, no auth in v1 per REQUIREMENTS.md Out of Scope ("Multi-user auth / accounts / SSO") |
| V3 Session Management | No | No sessions exist in this phase |
| V4 Access Control | No | No multi-user access boundary in v1 |
| V5 Input Validation | Yes | App-level allow-list check for `release_types`/`muted_event_types` values + non-empty `mbid`/`name`, backed by DB `CHECK` constraints as a non-bypassable second layer; explicit request DTOs (not decoding straight into the sqlc model) to prevent over-posting of server-owned fields (`id`, `artist_id`, `created_at`) |
| V6 Cryptography | No | Nothing cryptographic is introduced in this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via handler-constructed queries | Tampering | sqlc-generated parameterized queries only — no raw string concatenation into SQL anywhere in `internal/watchlist` |
| Mass assignment / over-posting (client sets `id`/`artist_id`/`created_at` via JSON body) | Tampering | Decode into purpose-built request DTOs; server constructs sqlc params explicitly, never a straight `json.Decode(&sqlcModel)` |
| Unbounded/malformed array values written to `release_types`/`muted_event_types` | Tampering / Denial of Service | Both app-level allow-list validation and a DB `CHECK ... <@ ARRAY[...]` constraint bound the value set to the known-small list, naturally capping array length too |
| Sequential integer PKs exposed via API (minor enumeration/info-disclosure surface) | Information Disclosure | Accepted risk for a single-operator v1 deployment with no auth boundary to enumerate across — revisit only if multi-user auth (WLST-07/v2 territory) is ever added |

## Sources

### Primary (HIGH confidence)
- `internal/httpserver/server.go`, `health.go`, `health_test.go`, `boot_e2e_test.go` — read directly this session, quoted verbatim where cited
- `internal/db/pool.go`, `migrate.go`, `sqlc/db.go`, `sqlc/models.go`, `sqlc/health.sql.go` — read directly this session
- `sqlc.yaml`, `go.mod`, `Makefile`, `queries/health.sql`, `cmd/server/main.go` — read directly this session
- `.planning/phases/02-watchlist-core/02-CONTEXT.md`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/PROJECT.md`, `.planning/STATE.md` — read directly this session
- `go list -m all` / `sqlc version` / `docker info` — commands run directly this session

### Secondary (fetched via WebSearch/WebFetch this session — Context7/Ref MCP tools were unavailable in this session's toolset, so official documentation pages were retrieved directly instead)
- docs.sqlc.dev/en/stable/reference/datatypes.html — array→`[]string`, enum→aliased string type mapping
- docs.sqlc.dev/en/stable/reference/config.html — `emit_interface`, `emit_pointers_for_null_types`, `sql_package` options
- docs.sqlc.dev/en/stable/reference/query-annotations.html — `:execrows` semantics
- pkg.go.dev/github.com/jackc/pgerrcode — current version, `UniqueViolation` constant
- github.com/go-chi/chi README (raw) — `chi.URLParam`, per-verb route registration
- Medium — "Why errors.As(err, &pgErr) returned false in my Go + PostgreSQL stack (and how I fixed it)" — import-path pitfall
- Crunchy Data Blog / boringSQL / making.close.com — enum-vs-CHECK-constraint tradeoff consensus

### Tertiary (LOW confidence, flagged in Assumptions Log)
- `pgtype.Text` JSON-marshaling default behavior (A1) — search results were inconclusive; mitigated via config rather than asserted as fact

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages; existing versions confirmed live against the installed toolchain (`sqlc version`, `go list -m all`)
- Architecture (seam pattern, project structure): HIGH — directly derived from Phase 1's verified, already-tested `Pinger` pattern
- Array/enum & error-handling specifics: MEDIUM — sourced from official docs via WebSearch/WebFetch (Context7 unavailable this session); direct quotes captured, but this session's tooling scores WebFetch/WebSearch as a lower-reliability provider tier than a dedicated docs API
- Pitfalls: MEDIUM-HIGH — four of five pitfalls are either directly verified against this repo's code or backed by a specific, quoted official-doc/GitHub-issue source; one (Pitfall 3 / A1) is explicitly flagged as unconfirmed

**Research date:** 2026-08-05
**Valid until:** 30 days (stable, slow-moving stack — chi/sqlc/pgx/Postgres semantics don't shift quickly)
