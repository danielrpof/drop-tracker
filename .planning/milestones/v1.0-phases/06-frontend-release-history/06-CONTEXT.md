# Phase 6: Frontend & Release History - Context

**Gathered:** 2026-08-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Users manage their watchlist (search, add, remove, set per-artist release-type/mute preferences) and browse a history of detected release events (new release, guest feature, deluxe/tracklist change), entirely through a React (Vite) SPA built and embedded into the Go binary via `go:embed` — no direct API use required. This phase covers both the frontend build (UI-01, UI-02, UI-03) and the backend history-query surface HIST-01 needs, since no events/history API endpoint exists yet — only the `events` table (Phase 4/5) that stores everything the history feed needs to render (title, artist_name, release_date, cover_art_url, track_count, previous_track_count, release_type, event_type, notified_at). No new detection logic (Phase 4), no changes to Discord notification behavior (Phase 5) — this phase is read/manage surface only, on top of already-built backend state.

</domain>

<decisions>
## Implementation Decisions

### Layout & navigation
- **D-01:** Two tabs/routes: "Watchlist" and "History". Watchlist tab handles search/add/remove/manage; History tab is a dedicated, purely read-only feed. No single combined dashboard, no three-way split with a per-artist detail page.
- **D-02:** The artist-search box lives at the top of the Watchlist tab (not a separate modal or global entry point) — search-as-you-type results render inline below it.
- **D-03:** History tab has no cross-links back to the Watchlist tab (e.g. clicking an event's artist does not jump/scroll to that artist's watchlist row). Kept fully decoupled for v1.
- **D-04:** The Watchlist tab shows no history-derived hints (e.g. "3 new since last visit," unseen-event badges). It is purely a management list — no "last viewed" concept is introduced. All release-activity signals live exclusively on the History tab.

### History feed shape (HIST-01)
- **D-05:** One global chronological feed across all watched artists, newest first — not grouped/collapsed by artist. Backing API is a single `GET /events`-style endpoint, not a per-artist drill-down.
- **D-06:** The feed supports filtering by both artist and event type (`new_release` / `guest_feature` / `deluxe_change`). This is locked-in scope, not a stretch add — REQUIREMENTS.md's HIST-01 text explicitly says "per artist," so artist-scoping is part of the requirement itself, not new capability. The history API needs query params for both axes.
- **D-07:** Long lists use infinite scroll / "load more" (fetch a bounded page, e.g. 20–30 events, append on scroll or button click) — not numbered pagination, not fetch-everything. The history API needs cursor- or offset-based pagination.
- **D-08:** Each event card renders type-specific detail, not a uniform minimal card: `new_release` shows cover art + release type + date; `guest_feature` shows the recording title + link; `deluxe_change` shows the track-count delta (e.g. "12 → 18 tracks," using `track_count`/`previous_track_count`). This mirrors the distinct Discord embed shapes from Phase 5 (D-01/D-02/D-03 in `05-CONTEXT.md`) so the UI and Discord notifications tell the same story about each event type.

### Add-artist & preferences flow
- **D-09:** MusicBrainz and Deezer search results render as two labeled columns/sections, side by side — matching the existing `GET /search` response shape exactly (source-tagged, no cross-source merge per Phase 3 D-02). No interleaved/merged single list.
- **D-10:** Adding an artist uses `Service.Add`'s existing defaults (Phase 2 D-08) with a single click on a search result — release-type filters and mute preferences are NOT set inline at add-time. They're edited afterward through the same preference-editing UI used for existing watchlist entries (D-12). One preference UI to build, not two.
- **D-11:** An already-watchlisted artist's search result shows "Already watching" (disabled/greyed, no add action) — determined client-side by cross-referencing search results against the already-fetched `GET /watchlist` list. The existing 409 Conflict from `POST /watchlist` (Phase 2 D-09) becomes a defensive backstop only, never the primary UX signal.
- **D-12:** Per-artist preferences (release types, muted event types) are edited via inline checkboxes/toggles directly in each watchlist row — changes `PATCH /watchlist/{id}` immediately on toggle, no modal, no separate save step. Matches the existing partial-update semantics (Phase 2 D-11: nil axis = untouched, explicit empty array = "mute/watch nothing" on that axis).

### Visual style & album art
- **D-13:** Visual design target is "clean, minimal, dark-themed — functional but polished," not bare-bones/utilitarian and not a high-design custom-branded identity. Reflects that this project's Core Value is CI/CD pipeline maturity (Phase 7), not UI craft — the UI should read as competently built without competing for the project's remaining time budget.
- **D-14:** Cover art is large and hero-style — the History feed and Watchlist list are visually art-forward (closer to an album-cover wall than a plain text list), using `cover_art_url` (events) and `image_url` (artists/watchlist), both already captured per Phase 5 D-12 / Phase 2 schema. Needs a graceful fallback (placeholder) for null art. Note: this is a deliberately more visually ambitious choice than D-13's "minimal" framing might suggest by itself — the user explicitly wants large art despite the otherwise-restrained theme; planning should treat hero-art layout as real, non-trivial UI work, not an afterthought.
- **D-15:** Styling approach is Tailwind CSS utility classes — no hand-written plain CSS/CSS Modules design system, no heavier component library (Mantine/Chakra, etc.).
- **D-16:** Empty states (no watchlist entries yet; no history events yet) get a friendly, styled message plus a clear call-to-action (e.g. "No artists yet — search above to add one"), consistent with the dark theme — not bare placeholder text.

### Claude's Discretion
- Exact history API route/param names (e.g. `GET /events?artist_id=&event_type=&cursor=`) and pagination mechanism (cursor vs. offset) — D-06/D-07 lock the *behavior*, not the wire shape.
- Exact new sqlc query/queries needed for the paginated, filterable history read (new migration if any index is needed for query performance) — implementation detail, follows the established `queries/*.sql` → `internal/db/sqlc/` codegen convention.
- React project layout (routing library choice for the two tabs — e.g. plain state-based tab switch vs. a router), data-fetching approach (plain `fetch`/`useState` vs. a library like TanStack Query), and component decomposition — not discussed, left to planning/research.
- Whether the frontend polls for new data automatically or relies on manual refresh — not raised during discussion; default to manual refresh (simplest) unless research surfaces a strong reason otherwise.
- Exact Tailwind color palette/dark theme tokens, spacing scale, and hero-art grid/card sizing — D-13/D-14/D-15 lock the *approach*, not the specific values.
- Exact `go:embed` wiring point (which Go file embeds the built `dist/` output, how the SPA is served for non-API routes/client-side routing fallback) — architecture detail, standard `go:embed` + chi static-serving pattern.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & Roadmap
- `.planning/REQUIREMENTS.md` — UI-01, UI-02, UI-03, HIST-01 (this phase's requirement set); HIST-01's literal text ("per artist") is why D-06 locks artist-filtering as in-scope, not a stretch add.
- `.planning/ROADMAP.md` §"Phase 6: Frontend & Release History" — goal and success criteria; `UI hint: yes` flag noted (a `/gsd-ui-phase` UI-SPEC.md pass may be warranted before/alongside planning, given the visual decisions D-13/D-14/D-15 above).
- `.planning/PROJECT.md` — constraints (React + Vite via `go:embed`, single Go binary/service, Core Value is CI/CD pipeline maturity — shapes D-13's "don't over-invest in UI craft" framing) and Key Decisions table.

### Tech stack / library choices (already locked)
- `.claude/CLAUDE.md` — "UI: React + Vite, built and embedded into the Go binary via `go:embed` (no separate frontend deploy pipeline)."

### Prior phase decisions this phase builds directly on
- `.planning/phases/05-discord-notifications/05-CONTEXT.md` — D-01/D-02/D-03 (per-event-type visual distinction: color+emoji for new_release/guest_feature/deluxe_change) directly informs D-08's type-specific history cards; D-04 (`previous_track_count` column exists specifically so a track-count delta can be shown) is what D-08's deluxe_change card renders.
- `.planning/phases/03-external-clients-search/03-CONTEXT.md` — D-01/D-02 (single `GET /search` endpoint, source-tagged results, no cross-source merge/dedup) directly shapes D-09's side-by-side-columns decision.
- `.planning/phases/02-watchlist-core/02-CONTEXT.md` — D-08 (default preferences on add), D-09 (409 on duplicate add — now a backstop per D-11), D-11 (PATCH partial-update semantics, nil vs. empty-array axis meaning — directly shapes D-12's inline-toggle behavior).

### Existing code (Phase 1–5)
- `internal/httpserver/server.go` — chi router wiring (`New()`); the new history endpoint registers here alongside `/health`, `/search`, `/watchlist`.
- `internal/httpserver/search.go` — `GET /search` handler, `SearchArtist`/`searchResponse` wire shapes (D-01/D-02/D-03 from Phase 3) — the exact per-source response shape D-09's UI renders directly.
- `internal/httpserver/watchlist.go` — full CRUD handler set (`POST`/`GET`/`PATCH`/`DELETE /watchlist`), `errorResponse` `{"error": "..."}` shape, `addWatchlistRequest`/`updateWatchlistRequest` DTOs — the exact contracts the Watchlist tab's add/edit/remove UI calls into.
- `internal/db/sqlc/models.go` — `Event` struct (`ID`, `ArtistID`, `Source`, `EventType`, `ExternalID`, `ReleaseGroupMbid`, `Title`, `ArtistName`, `ReleaseDate`, `CoverArtUrl`, `TrackCount`, `NotifiedAt`, `CreatedAt`, `PreviousTrackCount`, `ReleaseType`) — every field the history feed's cards (D-08) need is already present; no new columns required, only a new read query.
- `queries/events.sql` — existing queries (`InsertEvent`, `ListExternalIDs`, `HasAnyEvent`, `ListUnnotified`, baseline queries, `MarkNotified`) establish the sqlc query-file convention; a new `ListEvents`-style paginated/filterable query (D-05/D-06/D-07) follows the same file and naming conventions.
- `internal/watchlist/service.go` — `Store`/`Entry`/`ReleaseTypes`/`EventTypes` allow-lists — the exact preference vocabulary D-12's inline toggles must render and submit.

No external specs beyond the above — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `GET /search`'s `searchResponse{Query, Sources: map[string]sourceResult}` shape — D-09's side-by-side columns map directly onto this: one column per key in `Sources`.
- `GET /watchlist`'s bare-array response (`[]watchlist.Entry`, D-12 from Phase 2 — no envelope) — the Watchlist tab's list-rendering is a direct consumer, no client-side unwrapping needed.
- `watchlist.ReleaseTypes` / `watchlist.EventTypes` — the single Go-side allow-lists (mirroring DB CHECK constraints) that D-12's inline preference toggles must render as checkbox options; already validated server-side by `PATCH /watchlist/{id}`.

### Established Patterns
- Handlers live under `internal/httpserver/`, one file per concern (`health.go`, `search.go`, `watchlist.go`) — a new `events.go` (or similar) handler for the history endpoint follows the same shape.
- `errorResponse{Error string}` is the single error-body shape across every existing handler — the new history endpoint should return errors the same way for frontend error-handling consistency.
- sqlc queries live under `queries/`, generated into `internal/db/sqlc/` — a new `ListEvents`-style query in `queries/events.sql` follows the existing file's conventions (explicit `:many`/`:one` annotations, comments explaining non-obvious predicates).
- No frontend code, build tooling, or `go:embed` wiring exists yet anywhere in the repo — this phase is genuinely greenfield for the entire frontend half (confirmed via `find` — no `web/`, `frontend/`, or `ui/` directory, no `package.json`, no React/Vite/Tailwind in `go.mod` or anywhere else).

### Integration Points
- New history/events read endpoint registers in `internal/httpserver/server.go`'s `New()`, alongside the existing five routes.
- New `queries/events.sql` query (paginated, artist+event-type filterable) feeds the new handler; may need a new index depending on query shape decided during planning (D-06/D-07's Claude's Discretion).
- New Vite-built React app lives in its own top-level directory (not yet created); its `dist/` output is embedded via `go:embed` into `cmd/server` (or a new package) per CLAUDE.md's locked architecture — exact wiring point is Claude's Discretion above.

</code_context>

<specifics>
## Specific Ideas

The recurring theme across this discussion was "let the UI tell the same story Discord notifications already tell" — D-08's type-specific event cards deliberately mirror Phase 5's distinct new_release/guest_feature/deluxe_change embed shapes (color+emoji there, cover art + delta info here), rather than inventing a new visual vocabulary. The one real design tension surfaced was between D-13 (restrained, minimal, "don't over-invest in UI craft" given this project's CI/CD-focused Core Value) and D-14 (explicitly large, hero-style album art) — the user wants a visually striking release-history feed specifically, while keeping everything else (chrome, controls, typography) minimal. Planning/research should treat D-14 as real layout work, not a small styling tweak, despite D-13's overall "don't over-invest" framing.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion; the one option that added scope beyond the minimum (D-06's artist+event-type filtering) was confirmed to already be covered by HIST-01's literal requirement text, not new capability.

</deferred>

---

*Phase: 6-Frontend & Release History*
*Context gathered: 2026-08-10*
