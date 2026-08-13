# Phase 3: External Clients & Search - Context

**Gathered:** 2026-08-06
**Status:** Ready for planning

<domain>
## Phase Boundary

The service can safely search and poll MusicBrainz and Deezer within their documented rate limits, and users can live-search those catalogs to find artists to watch. This phase builds the hand-rolled `internal/musicbrainz` and `internal/deezer` HTTP clients (rate-limited via `golang.org/x/time/rate`), the `GET /search` proxy endpoint (WLST-01/CLNT-03), and wires `robfig/cron` to poll each watchlisted artist on a configurable schedule (CLNT-01/CLNT-02). Diffing poll results against a "seen" store, and all three detection event types (new release, guest feature, deluxe/tracklist-change), are Phase 4 — this phase's poll handler only fetches and logs. No UI (Phase 6), no Discord notifications (Phase 5).

</domain>

<decisions>
## Implementation Decisions

### Search-proxy contract
- **D-01:** Single combined endpoint, `GET /search?q=...`, queries MusicBrainz and Deezer server-side and returns one response. No separate `/search/musicbrainz` and `/search/deezer` resources — matches Phase 2's flat, unprefixed route convention (D-14 from Phase 2).
- **D-02:** No cross-source dedup/merge. Response contains MusicBrainz results and Deezer results as separate, source-tagged lists — no fuzzy-matching by name. Consistent with REQUIREMENTS.md explicitly listing dual-source reconciliation as out of scope; would be inconsistent to build name-matching just for search when it's rejected for detection. — **Reversibility:** reversible — merging later is additive, doesn't break the existing per-source shape.
- **D-03:** Partial results on source failure. If one upstream (e.g. Deezer) errors or times out, the endpoint returns whatever succeeded plus a per-source status/error flag — never fails the whole request because one source is degraded.

### Scheduler scope (Phase 3 vs Phase 4)
- **D-04:** `robfig/cron` is wired up in Phase 3, not deferred to Phase 4. The poll job runs on `Config.PollInterval`, calls the MusicBrainz/Deezer clients per watchlisted artist, and the handler logs a structured line per artist/source (artist, source, item count) rather than doing anything with the data. Phase 4 replaces the log statement with real diff logic against the "seen" store — the scheduler plumbing, rate-limiting-under-real-schedule behavior, and overlap guard (D-07) are de-risked now instead of being built alongside the diff engine.
- **D-05:** The poll cycle reads the current watchlist via the existing `watchlist.Store` interface (`internal/watchlist/service.go`) — no new dedicated poll-list query. Reuses Phase 2's list query and DB seam as-is.
- **D-06:** An artist with a null `deezer_id` (added via MusicBrainz-only search, Phase 2 D-02) is skipped for the Deezer half of the poll cycle — no name-search fallback to opportunistically backfill `deezer_id`.

### Rate-limiting & concurrency shape
- **D-07:** One `rate.Limiter` per client (MusicBrainz, Deezer), each sized from `Config.MusicBrainzRateLimitPerSec` / `Config.DeezerRateLimitPer5s` (already stubbed in `internal/config/config.go`). The poll cycle iterates watchlisted artists **sequentially**, calling `limiter.Wait(ctx)` before each request — no worker pool. The limiter itself enforces pacing; correctness doesn't depend on manual concurrency bookkeeping.
- **D-08:** MusicBrainz and Deezer run as **independent poll cycles** (separate cron jobs or separate goroutines), each gated only by its own limiter. MusicBrainz's slow ~1/sec pace never throttles Deezer's faster ~50/5s pace, matching CLNT-01/CLNT-02 as two distinct requirements with independent success criteria.
- **D-09:** Each per-source poll cycle has an in-process overlap guard (atomic/mutex "cycle in progress" flag) — if a cron tick fires while the previous cycle for that source is still running, the tick is skipped and logged as a warning. Built now at the scheduler level so Phase 4's DTCT-05 ("no overlapping detection runs") inherits this property instead of retrofitting it. — **Reversibility:** reversible — a future distributed-lock replacement (noted as a Phase-7+ consideration in the tech stack doc) swaps this flag out without changing calling code.

### MusicBrainz / Deezer entity scope
- **D-10:** The MusicBrainz poll client fetches **release-groups only** (browse-by-artist) in Phase 3. Enough to satisfy CLNT-01 and gives Phase 4 what it needs for new-release/deluxe detection (DTCT-01/DTCT-02) immediately. Recordings-by-artist-credit (needed for guest-feature detection, DTCT-03) is a different query shape with no consumer until Phase 4 — that fetch path is built in Phase 4 alongside the diff logic that uses it.
- **D-11:** The MusicBrainz search-proxy queries the `/ws/2/artist?query=` artist-search endpoint only — returns matching artists (MBID, name, disambiguation, type). No combined artist+release-group search; no requirement (WLST-01/UI-01) asks for searching by release/song title.
- **D-12:** The Deezer poll client fetches **artist albums/releases only** (by `deezer_id`), mirroring the release-groups-only scope decided for MusicBrainz. No track-level data — Deezer is documented in REQUIREMENTS.md as a secondary/faster signal, not the primary guest-feature detection source, so track-level fetch has no Phase 4 consumer.

### Claude's Discretion
- Exact `GET /search` query-param shape (result limit, pagination if any), and per-request timeout values for the MusicBrainz/Deezer HTTP clients — not discussed, left to planning/research.
- Whether the overlap guard (D-09) is a `sync.Mutex`, `atomic.Bool`, or similar — implementation detail.
- Exact structured-log field names for the log-only poll handler (D-04) — follow existing `request_id`/slog conventions from Phase 1/2.
- Go project layout for the two new client packages (`internal/musicbrainz`, `internal/deezer`) — matches CLAUDE.md's stated package naming, no further decisions needed.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & Roadmap
- `.planning/REQUIREMENTS.md` — WLST-01, CLNT-01, CLNT-02, CLNT-03 (this phase's requirement set); notes DTCT-01/02/03 (Phase 4) and dual-source reconciliation as explicitly out of scope.
- `.planning/ROADMAP.md` §"Phase 3: External Clients & Search" — goal and success criteria.
- `.planning/PROJECT.md` — constraints (chi router, single Go binary, robfig/cron, hand-rolled clients) and Key Decisions table.

### Tech stack / library choices (already locked)
- `.claude/CLAUDE.md` — "Supporting Libraries" table: hand-rolled MusicBrainz/Deezer clients (`internal/musicbrainz`, `internal/deezer`) using plain `net/http`, `User-Agent` requirement, `golang.org/x/time/rate` for self-imposed rate limiting, `robfig/cron/v3` for scheduling.

### Existing config/schema/patterns (Phase 1 & 2)
- `internal/config/config.go` — `Config` struct already stubs `MusicBrainzUserAgent`, `MusicBrainzRateLimitPerSec`, `DeezerRateLimitPer5s`, `PollInterval` — Phase 3 reads these, does not add new required fields for the client/scheduler surface.
- `internal/watchlist/service.go` — `watchlist.Store` interface the poller (D-05) and search endpoint depend on; also defines the artist record shape (`mbid`, nullable `deezer_id`) relevant to D-06/D-11/D-12.
- `internal/httpserver/server.go` — chi router wiring and middleware order (`RequestID` → `echoRequestID` → `httplog` → `Recoverer`); the new `GET /search` route registers here following the same pattern as `/health` and `/watchlist`.
- `queries/artists.sql`, `internal/db/migrations/000002_watchlist.up.sql` — current `artists` table shape (mbid required, deezer_id nullable) from Phase 2 D-01/D-02.
- `.planning/phases/02-watchlist-core/02-CONTEXT.md` — D-01/D-02 (MBID-as-primary-key, nullable deezer_id) directly shape D-06/D-11/D-12 above.

No external specs beyond the above — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go` — rate-limit and poll-interval config already stubbed (D-06/D-07 from Phase 1); Phase 3 is the first phase to actually read `MusicBrainzUserAgent`, `MusicBrainzRateLimitPerSec`, `DeezerRateLimitPer5s`, `PollInterval`.
- `internal/watchlist/service.go` (`watchlist.Store`) — reused directly by the poller (D-05), avoiding a new query.
- `internal/httpserver/server.go`'s `Pinger`-style narrow-interface seam pattern — Phase 3's MusicBrainz/Deezer clients should follow the same testability pattern (narrow interfaces, `httptest.Server`-backed tests per CLAUDE.md's testing constraint) rather than depending on concrete client types.

### Established Patterns
- Handlers live under `internal/httpserver/`, one file per concern (`health.go`, `watchlist.go`) — the new `search.go` handler follows the same shape.
- External API clients live under their own `internal/<name>` package (`internal/musicbrainz`, `internal/deezer`) per CLAUDE.md's "Supporting Libraries" table — not nested under `httpserver`.
- Structured JSON logging via `go-chi/httplog/v3` with `request_id` correlation is wired at the router level; the poll handler (D-04) is background/cron-triggered, not HTTP-triggered, so it needs its own `slog` call sites rather than relying on the HTTP middleware chain.
- Migrations are plain up/down `.sql` under `internal/db/migrations/`, embedded via `go:embed`.

### Integration Points
- New `GET /search` route registers in `internal/httpserver/server.go`'s `New()`, alongside existing `/health` and `/watchlist` routes.
- `robfig/cron` scheduler is a new top-level wiring concern — likely started from `cmd/` alongside the HTTP server, sharing the same `watchlist.Store` and client instances.
- No new migration expected — Phase 2 already added the nullable `deezer_id` column (D-02) this phase needed.

</code_context>

<specifics>
## Specific Ideas

No particular UI/visual references (Phase 3 is API + scheduler only, no UI — that's Phase 6). The "log-only fetch, real diff in Phase 4" split (D-04) was explicitly reasoned to avoid building scheduler plumbing and diff logic in the same phase, and to let Phase 4 inherit the overlap guard (D-09) rather than retrofit it.

</specifics>

<deferred>
## Deferred Ideas

- **Distributed/multi-instance scheduler locking** — the in-process overlap guard (D-09) is explicitly single-instance only; a Postgres-advisory-lock-based guard for running multiple app instances is noted in the tech stack doc as a later consideration, not this phase.
- **Deezer name-search fallback to backfill `deezer_id`** — considered and rejected for the poll cycle (D-06); could resurface as a small enhancement in a later phase if users frequently add MusicBrainz-only artists.
- **Combined artist+release-group search** — considered and rejected for the search-proxy (D-11); no current requirement needs searching by release/song title, would be its own phase if ever wanted.

</deferred>

---

*Phase: 3-External Clients & Search*
*Context gathered: 2026-08-06*
