# Phase 2: Watchlist Core - Context

**Gathered:** 2026-08-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can fully manage their watchlist — add, remove, list, and configure per-artist alert preferences — through a tested API service layer. This phase owns the entire watchlist schema (artists + watchlist tables) since Phase 1 deliberately created no domain tables. Live search against MusicBrainz/Deezer (WLST-01, CLNT-03) is Phase 3 — Phase 2's "add" endpoint accepts a client-supplied MBID + name (as if a search result were already selected), not a live lookup. No UI (Phase 6), no MusicBrainz/Deezer polling or detection (Phase 3/4).

</domain>

<decisions>
## Implementation Decisions

### Artist identity model
- **D-01:** MusicBrainz ID (MBID) is required to add an artist — it's the stable external key the whole pipeline (search in Phase 3, polling/detection in Phase 4) keys off of. Name-only or Deezer-only entries are not supported. — **Reversibility:** one-way — changing the primary external key later means a data migration across artists, watchlist, and any Phase 4 detection-event rows that reference it.
- **D-02:** Store a nullable `deezer_id` column on the artist record now, even though Phase 3's Deezer client is what populates it. Avoids a Phase 3 schema migration.
- **D-03:** `artists` is a separate master-data table (mbid, deezer_id, name, metadata) from `watchlist` (the user's entry: artist_id FK + preferences). Phase 4 (detection events) and Phase 6 (release history) will both need to reference "artist" independent of whether it's currently watchlisted. — **Reversibility:** one-way — collapsing back to a flat table after Phase 4/6 have FKs into `artists` would require a schema rewrite touching multiple tables.
- **D-04:** Both MBID and name are required fields on add (a MusicBrainz search result always has both, per WLST-02's "add from search results" framing).

### Preferences model
- **D-05:** Release-type filters (WLST-05) and mute preferences (WLST-06) are two distinct axes, not one unified structure. Release-type filter = which release TYPES (album/single/EP/deluxe) the artist is watched for at all (catalog scope). Mute preference = which EVENT CATEGORIES (new release / guest feature / deluxe-tracklist-change — the three DTCT event kinds from Phase 4) actually produce a Discord post (alert noise control). — **Reversibility:** costly — merging them later means migrating two independent per-entry structures into one and reconciling any conflicting states already saved.
- **D-06:** Release-type filter stored as a Postgres array column (`release_types text[]` or a Postgres enum array) on the watchlist row — adding a new release type later is a value change, not a schema migration.
- **D-07:** Mute preference stored the same way — `muted_event_types text[]` — for representational consistency with D-06.
- **D-08:** Default on add: everything on (all release types enabled, nothing muted). Adding an artist means "notify me about everything from them" until the user narrows it down; opt-in-by-default would make a freshly added artist silent until configured.

### Duplicate-add / removal behavior
- **D-09:** Re-adding an artist already on the watchlist returns `409 Conflict` with a clear error message. Not idempotent, not treated as an implicit preferences update — the caller must use the dedicated update path to change preferences.
- **D-10:** Removal is a hard delete of the watchlist row. Phase 6's release history (HIST-01) is about detected events, not the watchlist entry itself, so nothing downstream needs a tombstoned watchlist row. Re-adding later is a fresh insert (with the 409 rule above only applying while still present).

### API resource shape
- **D-11:** Single `/watchlist` resource with preferences embedded — `POST /watchlist` (add, with optional initial preferences), `GET /watchlist` (list, preferences inline), `PATCH /watchlist/{id}` (update preferences), `DELETE /watchlist/{id}` (remove). No separate `/watchlist/{id}/preferences` sub-resource — the CRUD surface is small enough that splitting it adds endpoints without a benefit at this scope.
- **D-12:** Request/response bodies are plain JSON objects (the resource itself, e.g. `{"id":..,"mbid":..,"name":..,"release_types":[..],"muted_event_types":[..]}`) — no `{"data": ...}` envelope. Matches typical Go/chi convention; no pagination/metadata need yet.
- **D-13:** Validation/error responses are `{"error": "message"}` JSON bodies with the appropriate HTTP status code (400/404/409) — not RFC 7807 Problem Details. Consistent with the plain-JSON body convention (D-12).
- **D-14:** No `/api` prefix — routes are flat (`/watchlist`, not `/api/watchlist`), matching Phase 1's `/health` convention. The app is a single binary with the SPA embedded later (Phase 6), not a namespaced API service.

### Claude's Discretion
- Exact column names/types beyond what's specified above (e.g., timestamps, any additional artist metadata fields like image URL or disambiguation comment) — not discussed, left to planning/research.
- Whether the release-type/event-category values are a Postgres native `enum` type vs. plain `text[]` with application-level validation — D-06/D-07 settled on array representation; enum-vs-text is an implementation detail.
- Exact validation error messages and Go error-handling patterns (following Phase 1's established conventions).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & Roadmap
- `.planning/REQUIREMENTS.md` — WLST-02 through WLST-06 (this phase's requirement set); note WLST-01 (search) belongs to Phase 3, not this phase.
- `.planning/ROADMAP.md` §"Phase 2: Watchlist Core" — goal and success criteria.
- `.planning/PROJECT.md` — constraints (chi router, sqlc, golang-migrate, single Go binary) and Key Decisions table.

### Existing schema/config conventions (Phase 1)
- `internal/db/migrations/000001_init.up.sql` — the current (no-op) migration; Phase 2's migration is the first real schema.
- `sqlc.yaml` — sqlc config: `pgx/v5`, schema at `internal/db/migrations`, queries at `queries/`, output `internal/db/sqlc`, JSON tags emitted.
- `internal/httpserver/server.go` — chi router wiring (middleware order: RequestID → echoRequestID → httplog → Recoverer); new watchlist routes register here following the same pattern as `/health`.

No external specs beyond the above — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/db/pool.go` — pgx pool setup Phase 2's queries will run against.
- `internal/httpserver/server.go`'s `Pinger`-style seam pattern — Phase 1 defines narrow interfaces for testability (e.g., `Pinger` for `*pgxpool.Pool`); Phase 2's handlers should follow the same pattern for DB access so tests don't need a real Postgres for every case.
- `internal/config/config.go` — `caarlos0/env`-based Config struct; no new env vars are expected for this phase (no external API keys needed — Phase 2 is DB + HTTP only).

### Established Patterns
- Handlers live under `internal/httpserver/`, one file per concern (`health.go` pattern) — likely `watchlist.go` following the same shape.
- sqlc-generated queries live under `internal/db/sqlc/`, hand-written `.sql` under `queries/` per `sqlc.yaml`.
- Structured JSON logging via `go-chi/httplog/v3` with `request_id` correlation is already wired at the router level — no per-handler logging setup needed.
- Migrations are plain up/down `.sql` files under `internal/db/migrations/`, embedded via `go:embed`, applied at boot with a retry policy (`internal/db/migrate.go`).

### Integration Points
- New routes register in `internal/httpserver/server.go`'s `New()` function, same place `/health` is registered.
- New migration file `000002_watchlist.up.sql` / `.down.sql` follows the `NNNNNN_description` naming already established by `000001_init`.

</code_context>

<specifics>
## Specific Ideas

No specific UI/visual references (Phase 2 has no UI — API only). The MBID-as-primary-key decision (D-01) was explicitly reasoned from the roadmap's stated MusicBrainz-primary / Deezer-secondary detection strategy, not an arbitrary preference.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.

</deferred>

---

*Phase: 2-Watchlist Core*
*Context gathered: 2026-08-05*
