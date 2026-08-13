# Phase 6: Frontend & Release History - Research

**Researched:** 2026-08-10
**Domain:** React/Vite SPA embedded in a Go binary (go:embed), React Router v7 framework-mode SPA tooling, shadcn/Tailwind v4 UI, and a new cursor-paginated/filterable Postgres read endpoint (sqlc)
**Confidence:** MEDIUM (backend patterns HIGH — verified against this repo's own code; frontend scaffold behavior MEDIUM — verified via official docs/WebFetch, not yet executed in-repo)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Layout & navigation**
- **D-01:** Two tabs/routes: "Watchlist" and "History". Watchlist tab handles search/add/remove/manage; History tab is a dedicated, purely read-only feed. No single combined dashboard, no three-way split with a per-artist detail page.
- **D-02:** The artist-search box lives at the top of the Watchlist tab (not a separate modal or global entry point) — search-as-you-type results render inline below it.
- **D-03:** History tab has no cross-links back to the Watchlist tab (e.g. clicking an event's artist does not jump/scroll to that artist's watchlist row). Kept fully decoupled for v1.
- **D-04:** The Watchlist tab shows no history-derived hints (e.g. "3 new since last visit," unseen-event badges). It is purely a management list — no "last viewed" concept is introduced. All release-activity signals live exclusively on the History tab.

**History feed shape (HIST-01)**
- **D-05:** One global chronological feed across all watched artists, newest first — not grouped/collapsed by artist. Backing API is a single `GET /events`-style endpoint, not a per-artist drill-down.
- **D-06:** The feed supports filtering by both artist and event type (`new_release` / `guest_feature` / `deluxe_change`). This is locked-in scope, not a stretch add — REQUIREMENTS.md's HIST-01 text explicitly says "per artist," so artist-scoping is part of the requirement itself, not new capability. The history API needs query params for both axes.
- **D-07:** Long lists use infinite scroll / "load more" (fetch a bounded page, e.g. 20–30 events, append on scroll or button click) — not numbered pagination, not fetch-everything. The history API needs cursor- or offset-based pagination.
- **D-08:** Each event card renders type-specific detail, not a uniform minimal card: `new_release` shows cover art + release type + date; `guest_feature` shows the recording title + link; `deluxe_change` shows the track-count delta (e.g. "12 → 18 tracks," using `track_count`/`previous_track_count`). This mirrors the distinct Discord embed shapes from Phase 5 so the UI and Discord notifications tell the same story about each event type.

**Add-artist & preferences flow**
- **D-09:** MusicBrainz and Deezer search results render as two labeled columns/sections, side by side — matching the existing `GET /search` response shape exactly (source-tagged, no cross-source merge). No interleaved/merged single list.
- **D-10:** Adding an artist uses `Service.Add`'s existing defaults with a single click on a search result — release-type filters and mute preferences are NOT set inline at add-time. Edited afterward through the same preference-editing UI used for existing watchlist entries (D-12). One preference UI to build, not two.
- **D-11:** An already-watchlisted artist's search result shows "Already watching" (disabled/greyed, no add action) — determined client-side by cross-referencing search results against the already-fetched `GET /watchlist` list. The existing 409 Conflict from `POST /watchlist` becomes a defensive backstop only, never the primary UX signal.
- **D-12:** Per-artist preferences (release types, muted event types) are edited via inline checkboxes/toggles directly in each watchlist row — changes `PATCH /watchlist/{id}` immediately on toggle, no modal, no separate save step.

**Visual style & album art**
- **D-13:** Visual design target is "clean, minimal, dark-themed — functional but polished," not bare-bones/utilitarian and not a high-design custom-branded identity.
- **D-14:** Cover art is large and hero-style — art-forward (closer to an album-cover wall than a plain text list), using `cover_art_url` (events) and `image_url` (artists/watchlist). Needs a graceful fallback (placeholder) for null art. Treat hero-art layout as real, non-trivial UI work.
- **D-15:** Styling approach is Tailwind CSS utility classes — no hand-written plain CSS/CSS Modules design system, no heavier component library (Mantine/Chakra, etc.).
- **D-16:** Empty states get a friendly, styled message plus a clear call-to-action, consistent with the dark theme.

### Claude's Discretion
- Exact history API route/param names and pagination mechanism (cursor vs. offset) — D-06/D-07 lock the *behavior*, not the wire shape.
- Exact new sqlc query/queries needed for the paginated, filterable history read (new migration if any index is needed for query performance).
- React project layout (routing library choice — resolved by UI-SPEC to React Router), data-fetching approach (resolved by UI-SPEC to plain `fetch` + state), and component decomposition.
- Whether the frontend polls for new data automatically or relies on manual refresh — default to manual refresh (simplest) unless research surfaces a strong reason otherwise.
- Exact Tailwind color palette/dark theme tokens, spacing scale, and hero-art grid/card sizing — resolved by `06-UI-SPEC.md` (Design System, Spacing Scale, Typography, Color sections).
- Exact `go:embed` wiring point (which Go file embeds the built `dist/`-equivalent output, how the SPA is served for non-API routes/client-side routing fallback) — architecture detail, standard `go:embed` + chi static-serving pattern.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion; the one option that added scope beyond the minimum (D-06's artist+event-type filtering) was confirmed to already be covered by HIST-01's literal requirement text, not new capability.

**Note:** `06-UI-SPEC.md` (already approved) further locks: shadcn CLI (`pnpm dlx shadcn@latest init --preset bdKQpIA4 --template react-router`), React Router (resolves the routing-library discretion item), plain `fetch` + component state (resolves the data-fetching discretion item), and the full color/spacing/typography contract. This research treats those as locked and focuses on the technical mechanics of executing them correctly — in particular, a material gap the UI-SPEC does not address: the `--template react-router` scaffold produces a **framework-mode** React Router v7 app, not a plain Vite SPA, which has direct consequences for `go:embed` wiring (see Pitfall 1).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UI-01 | User can search for and add an artist to the watchlist via the web UI | Existing `GET /search` and `POST /watchlist` endpoints (verified in `internal/httpserver/search.go`, `watchlist.go`) are consumed as-is; no backend change needed. Frontend: two-column search results (D-09) + "Already watching" client-side cross-reference (D-11) — see Architecture Patterns. |
| UI-02 | User can view and manage (remove, set preferences) their watchlist via the web UI | Existing `GET /watchlist`, `PATCH /watchlist/{id}`, `DELETE /watchlist/{id}` consumed as-is (verified in `watchlist.go`). Frontend: inline preference toggles with optimistic update + rollback (D-12) — see Code Examples. |
| UI-03 | User can browse a feed/history of detected release events via the web UI | New `GET /events` endpoint (this phase builds it) + type-specific event cards (D-08) — see Architecture Patterns, Code Examples. |
| HIST-01 | User can view a history of detected events (new release, guest feature, deluxe change) per artist, including what changed | New sqlc `ListEvents` keyset-paginated, artist+event-type-filterable query — see Architecture Patterns "Pattern 2" and Code Examples. `previous_track_count`/`track_count` (verified present in `internal/db/sqlc/models.go:22-38`) supply the "what changed" delta for `deluxe_change`. |
</phase_requirements>

## Summary

This phase has two halves that must be researched differently. The **backend half** (new `GET /events` endpoint) is low-risk and follows conventions already established and verified in this exact codebase: sqlc query file per resource, `Store`-style narrow interface, `errorResponse{Error string}` JSON shape, and — critically — the same "don't order by `created_at` alone" lesson this codebase's own `ListUnnotified` query comment already documents (seed-mode cycles insert multiple rows sharing one timestamp). The new `ListEvents` query should paginate on `id DESC` (a single monotonic BIGSERIAL column), not `created_at`, and should filter `artist_id`/`event_type` via `sqlc.narg()` nullable parameters using the exact `CASE`/`COALESCE`-free `IS NULL OR` pattern this codebase already applies for optional-axis logic in `UpdateWatchlistPreferences`.

The **frontend half** is genuinely greenfield and carries one significant, UI-SPEC-unaddressed risk: `06-UI-SPEC.md` locks the scaffold command `shadcn@latest init --template react-router`, and this command scaffolds a **React Router v7 framework-mode** project (via `create-react-router` under the hood) — not a plain Vite SPA. Framework mode's default `react-router build` produces both a `build/client/` (static assets) *and* a `build/server/` (a Node.js server module meant to run under Express/Fastify/etc.), which conflicts directly with this project's locked architecture: a single Go binary with no Node runtime in production, embedding only static files via `go:embed`. The fix is well-documented and small — set `ssr: false` in `react-router.config.ts` ("SPA Mode"), which suppresses the server build entirely and produces only `build/client/` (a static `index.html` + assets, functionally equivalent to a Vite SPA's `dist/`) that `go:embed` can consume exactly as any other static SPA build. This must be an explicit, called-out task in the plan — skipping it means the executor discovers a Node server bundle where a pure static folder was expected, likely mid-Dockerfile in Phase 7.

**Primary recommendation:** Scaffold via the UI-SPEC's locked shadcn command, immediately add `ssr: false` to the generated `react-router.config.ts` before writing any application code, embed `build/client` via `go:embed` with a chi catch-all `NotFound` handler that serves `index.html` for any path not matching a static asset or a registered API route, and build the new `GET /events` handler/query following this repo's existing `Store`/sqlc conventions exactly (verified below).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Artist search (typeahead, two-column results) | Browser/Client | API/Backend | Rendering + debounce logic is client-side; `GET /search` (Phase 3, unchanged) does the actual upstream fan-out. |
| Watchlist management (add/remove/preferences) | Browser/Client | API/Backend | UI renders rows and issues optimistic PATCH/DELETE; `internal/httpserver/watchlist.go` (Phase 2, unchanged) owns validation/persistence. |
| Release history feed (list, filter, paginate) | API/Backend | Browser/Client | The keyset-pagination/filter contract is a backend concern (must be race-safe, index-aware); the client only renders pages and requests the next cursor. This is the one NEW backend surface this phase adds. |
| Static asset + SPA serving | API/Backend (embedded) | — | No CDN/edge tier exists in this architecture (single Go binary, PROJECT.md constraint) — the Go process itself serves the built frontend via `go:embed`, acting as its own static host. |
| Client-side routing (`/watchlist`, `/history`) | Browser/Client | — | React Router (SPA Mode) resolves routes entirely in the browser after the initial `index.html` load; the Go server's only routing responsibility is the catch-all fallback. |
| Dark theme / design tokens | Browser/Client | — | Pure CSS (Tailwind v4 `@theme` directives), no server involvement. |
| "Already watching" cross-reference (D-11) | Browser/Client | — | Computed client-side by diffing two already-fetched lists (`GET /search` results vs. `GET /watchlist`); no new backend endpoint needed for this. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `react` | 19.2.8 [VERIFIED: npm registry, homepage react.dev, maintainer `fb <opensource+npm@fb.com>`] | UI library | Locked by CLAUDE.md ("React + Vite"). |
| `react-dom` | 19.2.8 [VERIFIED: npm registry] | DOM renderer | Paired 1:1 with `react`. |
| `react-router` | 8.3.0 [VERIFIED: npm registry, maintainers `brophdawg11`/`mjackson` (react-router core team)] | Routing (framework mode, SPA-configured) | Locked by `06-UI-SPEC.md`'s `--template react-router` preset choice; resolves CONTEXT.md's routing-library discretion item. **Note:** v7+ unified `react-router-dom` into the single `react-router` package — do not install the legacy `react-router-dom` package alongside it. |
| `@react-router/dev` | 8.3.0 [VERIFIED: npm registry] | Build/dev tooling (Vite plugin, CLI) | Required dev dependency for framework-mode projects; provides `react-router build`/`react-router dev`. |
| `@react-router/node` | 8.3.0 [VERIFIED: npm registry] | Node runtime adapters | Required even in SPA Mode per official docs [CITED: reactrouter.com/how-to/spa] — the root route is still server-rendered once at *build time* to produce `index.html`. |
| `typescript` | ~5.x (registry shows a 7.x pre-release tag; **pin to the newest stable 5.x**, not the `7.0.2` "latest" dist-tag) [VERIFIED: npm registry] | Type checking | UI-SPEC recommends TypeScript for shadcn component IntelliSense. **Verify the actual stable tag at execution time** (`npm view typescript dist-tags`) — do not blindly `npm install typescript@latest` without checking, since a 7.x pre-release landing as `latest` would be a breaking, undertested toolchain jump for a portfolio project. |
| `tailwindcss` | 4.3.3 [VERIFIED: npm registry, homepage tailwindcss.com, maintainer `adamwathan`] | Utility-first CSS | Locked by D-15 and `06-UI-SPEC.md`. v4 is CSS-first (no `tailwind.config.js` by default) [CITED: tailwindcss.com/docs/installation/using-vite]. |
| `@tailwindcss/vite` | 4.3.3 [VERIFIED: npm registry] | Vite plugin for Tailwind v4 | Official Vite integration path for v4 [CITED: tailwindcss.com/docs/installation/using-vite]. |
| `shadcn` (CLI, not a runtime dependency) | 4.16.2 [VERIFIED: npm registry, homepage github.com/shadcn-ui/ui, maintainer `shadcn <m@shadcn.com>`] | Component scaffolding CLI | Locked by `06-UI-SPEC.md` (preset `bdKQpIA4`). Copies owned component source into the repo — not an npm runtime dependency itself, satisfying D-15's "no heavier component library" constraint. |
| `lucide-react` | 1.31.0 [VERIFIED: npm registry, homepage lucide.dev] | Icon set | shadcn's default icon library, ships with the preset per `06-UI-SPEC.md`. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `class-variance-authority` | 0.7.1 [VERIFIED: npm registry, OK verdict — established, no "too-new" flag] | Variant-based className composition | Pulled in automatically by shadcn component files (e.g. `Button`'s variant prop). Don't hand-write variant switch statements. |
| `clsx` | 2.1.1 [VERIFIED: npm registry, OK verdict] | Conditional className joining | Same — shadcn components import this directly. |
| `tailwind-merge` | 3.6.0 [VERIFIED: npm registry, OK verdict] | Dedupes conflicting Tailwind classes when composing `className` props | Used inside shadcn's generated `cn()` utility (`lib/utils.ts`). |
| `sonner` | 2.0.8 [VERIFIED: npm registry, homepage sonner.emilkowal.ski] | Toast notifications | Locked by `06-UI-SPEC.md`'s Registry Safety table (Toast (Sonner)) — backs every toast copy in the Copywriting Contract (add-failure, remove-undo, preference-PATCH-failure). Added via `shadcn add sonner`, not installed bare. |
| `@radix-ui/react-*` (per-component, e.g. `react-tabs`, `react-switch`, `react-checkbox`, `react-avatar`, `react-separator`) | resolved per-component by the shadcn CLI at `add` time | Unstyled accessible primitives underlying each shadcn component | Never install directly — let `shadcn add <component>` resolve and pin the correct Radix package per component (Tabs, Switch, Checkbox, Avatar, Separator per the Registry Safety table). |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Plain `fetch` + component state (locked) | TanStack Query | TanStack Query's value (background refetch, cache invalidation, request dedup) is wasted on this phase's manual-refresh, ~5-endpoint surface (locked discretion: "default to manual refresh... unless research surfaces a strong reason otherwise" — it does not here). Revisit only if a later phase adds polling/websocket-driven live updates. |
| React Router SPA Mode (recommended) | Plain Vite + `react-router` in "declarative/data mode" (library mode, no framework CLI) | Avoids the framework-mode-vs-SPA-mode pitfall entirely — a bare `npm create vite@latest` + `react-router` as a library has no server-build concept to opt out of. **Not chosen** because `06-UI-SPEC.md` already locked the `shadcn --template react-router` scaffold command; re-deriving a different scaffold path would contradict an approved, checker-signed-off design contract. Document the SPA Mode fix instead of overriding the lock. |
| Cursor (keyset) pagination on `id` (recommended) | Offset (`LIMIT`/`OFFSET`) pagination | Offset pagination re-scans and discards `OFFSET` rows on every page and is prone to skipped/duplicated rows when new events are inserted between page fetches — exactly the situation this app is in (a background poller inserts new rows while a user scrolls). Keyset avoids both problems and is cheap here since `id` is already the primary key (no extra index needed). |
| `sqlc.narg()` + `IS NULL OR` filter predicates (recommended) | Building the SQL string dynamically in Go (conditionally appending `WHERE` clauses) | sqlc requires a single static SQL string per query — dynamic SQL-string-building defeats sqlc's compile-time-checked-query model and reintroduces manual SQL-injection risk this codebase has avoided everywhere else. The `IS NULL OR` pattern keeps one static query. |

**Installation (run inside the new frontend directory, e.g. `web/`, after `shadcn init` has scaffolded it):**
```bash
# Scaffold (06-UI-SPEC.md's locked command — creates the react-router
# framework-mode project AND shadcn's Tailwind/component wiring in one step)
pnpm dlx shadcn@latest init --preset bdKQpIA4 --template react-router

# Then, before writing any route/component code:
# 1. Edit react-router.config.ts to add `ssr: false` (see Pitfall 1)
# 2. Add components as needed, e.g.:
pnpm dlx shadcn@latest add button card tabs input checkbox switch sonner skeleton badge avatar separator alert
```

**Version verification:** Every version above was checked via `npm view <package> version` against the live npm registry on 2026-08-10 (see Package Legitimacy Audit for full signal detail). Re-run this check at plan-execution time if more than a few days have elapsed — this is a fast-moving ecosystem (React, Vite, and Tailwind all shipped within the last month of research).

## Package Legitimacy Audit

All packages below were checked via the package-legitimacy seam (`gsd-tools query package-legitimacy check --ecosystem npm`) and cross-verified against npm registry `homepage`/`maintainers` fields directly.

| Package | Registry | Age (heuristic) | Downloads/wk | Source Repo / Homepage | Verdict | Disposition |
|---------|----------|------------------|--------------|-------------------------|---------|-------------|
| `react` | npm | flagged "too-new" (latest version published 2026-07-21) | 163,083,190 | react.dev; maintainers `fb`/`react-bot` (Meta official) | SUS → **false positive** | Approved. The "too-new" signal fires on the *latest version's* publish timestamp (a routine patch release), not the package's actual age — 163M weekly downloads and Meta-official maintainer accounts confirm this is the real, canonical React package. |
| `react-dom` | npm | flagged "too-new" (2026-07-21) | 153,891,199 | Same as `react` | SUS → **false positive** | Approved, same reasoning. |
| `react-router` | npm | flagged "too-new" (2026-07-22) | 51,608,522 | github.com/remix-run/react-router; maintainers `brophdawg11`/`mjackson` (react-router core team) | SUS → **false positive** | Approved. |
| `@react-router/dev` | npm | flagged "too-new" (2026-07-22) | 2,023,605 | reactrouter.com; maintainer `mjackson` | SUS → **false positive** | Approved. |
| `@react-router/node` | npm | flagged "too-new" (2026-07-22) | 2,194,602 | Same as `@react-router/dev` | SUS → **false positive** | Approved. |
| `vite` | npm | flagged "too-new" (2026-08-06) | 164,321,250 | vite.dev; maintainers `yyx990803` (Evan You)/`vitebot` | SUS → **false positive** | Approved. |
| `@vitejs/plugin-react` | npm | flagged "too-new" (2026-07-30) | 80,108,572 | github.com/vitejs/vite-plugin-react | SUS → **false positive** | Approved, but **do not install unconditionally** — the `create-react-router` scaffold's own Vite plugin (`@react-router/dev/vite`) may already provide React support; verify `package.json` after scaffold before adding this separately (avoid a duplicate/conflicting React Fast Refresh setup). |
| `tailwindcss` | npm | flagged "too-new" (2026-07-16) | 120,828,649 | tailwindcss.com; maintainers `adamwathan`, `reinink`, `malfaitrobin` (Tailwind Labs) | SUS → **false positive** | Approved. |
| `@tailwindcss/vite` | npm | flagged "too-new" (2026-07-16) | 43,788,313 | Same as `tailwindcss` | SUS → **false positive** | Approved. |
| `shadcn` | npm | flagged "too-new" (2026-08-06) | 7,802,778 | github.com/shadcn-ui/ui; maintainer `shadcn <m@shadcn.com>` | SUS → **false positive** | Approved. Already locked by `06-UI-SPEC.md`. |
| `lucide-react` | npm | flagged "too-new" (2026-08-09) | 97,446,309 | lucide.dev | SUS → **false positive** | Approved. |
| `sonner` | npm | flagged "too-new" (2026-08-09) | 49,587,516 | sonner.emilkowal.ski; maintainer `emilkowalski` | SUS → **false positive** | Approved. |
| `class-variance-authority` | npm | 2024-11-26 | 61,796,857 | github.com/joe-bell/cva | OK | Approved. |
| `clsx` | npm | 2024-04-23 | 116,956,842 | github.com/lukeed/clsx | OK | Approved. |
| `tailwind-merge` | npm | 2026-05-10 | 79,980,725 | github.com/dcastil/tailwind-merge | OK | Approved. |
| `typescript` | npm | 2026-07-08 | 260,311,793 | github.com/microsoft/TypeScript | OK | Approved — **verify dist-tag before install** (see Standard Stack note; `latest` may resolve to an unreleased major). |

**Packages removed due to `[SLOP]` verdict:** none.

**Packages flagged as suspicious `[SUS]`:** All SUS verdicts above are the legitimacy checker's "too-new" heuristic firing on a *recent version bump's* publish timestamp rather than genuine package novelty — every flagged package has 2M–164M weekly downloads and an officially-controlled maintainer account/homepage, which the checker's downloads/repo signals alone would already classify OK if the heuristic weighted version-publish-recency less heavily for high-download packages. **The planner should still insert a lightweight `checkpoint:human-verify` before the `pnpm install`/`shadcn init` step** per the Package Legitimacy Gate protocol, but this checkpoint should be fast to clear — this audit found no red flags beyond the heuristic's known false-positive mode for actively-maintained, high-traffic packages.

*A package's name and existence on the registry were discovered via WebSearch/training knowledge, not a canonical spec, per the package-name provenance rule — hence `[VERIFIED: npm registry]` tags above are paired with the additional maintainer/homepage cross-check performed in this session, not registry-existence alone.*

## Architecture Patterns

### System Architecture Diagram

```
                     ┌─────────────────────────────────────────────┐
                     │              Browser (React SPA)              │
                     │                                                │
                     │  /watchlist route          /history route     │
                     │  ┌──────────────┐          ┌────────────────┐ │
                     │  │ Search box   │          │ Filter controls │ │
                     │  │ (debounced)  │          │ (artist, type)  │ │
                     │  └──────┬───────┘          └────────┬────────┘ │
                     │         │ GET /search               │ GET      │
                     │         │                            │ /events │
                     │  ┌──────▼───────┐          ┌────────▼────────┐ │
                     │  │ Two-column   │          │ Event card list  │ │
                     │  │ results      │          │ (type-specific,  │ │
                     │  │ + "Already   │          │  keyset-paged)   │ │
                     │  │  watching"   │          └────────┬────────┘ │
                     │  │  (D-11, diff │                   │ "Load    │
                     │  │  vs GET      │                   │  more"   │
                     │  │  /watchlist) │                   │ (append) │
                     │  └──────┬───────┘                              │
                     │         │ POST /watchlist                      │
                     │  ┌──────▼───────┐                              │
                     │  │ Watchlist row│                              │
                     │  │ list, inline │                              │
                     │  │ toggles      │──── PATCH /watchlist/{id} ──┐│
                     │  │ (optimistic) │                             ││
                     │  └──────────────┘──── DELETE /watchlist/{id} ─┤│
                     └────────────────────────────────────────────────┘
                                          │  (same-origin fetch, no CORS
                                          │   needed in production)
                     ┌────────────────────▼───────────────────────────┐
                     │            Go binary (single process)           │
                     │                                                  │
                     │  chi router (internal/httpserver)                │
                     │  ┌────────────────────────────────────────┐     │
                     │  │ Registered API routes (exact match):    │     │
                     │  │  GET  /health                            │     │
                     │  │  GET  /search                            │     │
                     │  │  POST/GET/PATCH/DELETE /watchlist[/{id}] │     │
                     │  │  GET  /events   ← NEW (this phase)       │     │
                     │  └───────────────┬──────────────────────────┘     │
                     │                  │ unmatched path falls through   │
                     │  ┌───────────────▼──────────────────────────┐     │
                     │  │ r.NotFound(spaHandler)  ← NEW (this phase)│     │
                     │  │  serves embedded build/client/* via       │     │
                     │  │  http.FileServer(http.FS(...)), falls     │     │
                     │  │  back to index.html for unmatched paths   │     │
                     │  │  (React Router SPA client-side routing)   │     │
                     │  └────────────────────────────────────────┘     │
                     │                                                  │
                     │  events.go handler → watchlist-style Store     │
                     │  interface → sqlc ListEvents query               │
                     └───────────────────────┬──────────────────────────┘
                                              │
                     ┌────────────────────────▼─────────────────────────┐
                     │  Postgres: events table (id BIGSERIAL PK,          │
                     │  artist_id, event_type, title, cover_art_url,      │
                     │  track_count, previous_track_count, ...)           │
                     │  ORDER BY id DESC keyset scan, WHERE artist_id/    │
                     │  event_type optionally filtered                    │
                     └──────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
web/                              # new top-level dir (react-router framework-mode scaffold)
├── app/
│   ├── root.tsx                  # root layout, HydrateFallback for SPA Mode
│   ├── routes.ts                 # route config (react-router's typed routes file)
│   ├── routes/
│   │   ├── watchlist.tsx         # /watchlist — D-01, D-02, D-09..D-12
│   │   └── history.tsx           # /history — D-01, D-03..D-08
│   ├── components/
│   │   ├── ui/                   # shadcn-generated components (owned, editable)
│   │   ├── watchlist/            # SearchBox, SearchColumn, WatchlistRow, PreferenceToggles
│   │   └── history/              # EventCard (+ NewReleaseCard/GuestFeatureCard/DeluxeChangeCard), LoadMoreButton
│   ├── lib/
│   │   ├── api.ts                # typed fetch wrappers for the 5 backend endpoints
│   │   └── utils.ts              # shadcn's cn() helper
│   └── app.css                   # Tailwind v4 @import + @theme tokens (D-13..D-16 values)
├── react-router.config.ts        # MUST set ssr: false (Pitfall 1) — not present by default
├── vite.config.ts                # includes @react-router/dev/vite + @tailwindcss/vite plugins
├── package.json
└── components.json               # shadcn config (registry, aliases) — created by init

internal/httpserver/
├── events.go                     # NEW: handleListEvents (GET /events), mirrors search.go/watchlist.go shape
└── server.go                     # add r.Get("/events", ...) + r.NotFound(spaHandler)

internal/db/
├── static.go (or similar)        # NEW: //go:embed the web/build/client output + spaHandler
queries/
└── events.sql                    # add ListEvents :many query (D-05/D-06/D-07)
```

### Pattern 1: React Router SPA Mode (the go:embed-compatible configuration)

**What:** Framework-mode React Router configured to skip runtime server rendering, producing a plain static `build/client/` directory.
**When to use:** Always, for this phase — the framework-mode default (SSR-capable, dual client+server build) is incompatible with "single Go binary, no Node runtime in production."
**Example:**
```typescript
// react-router.config.ts — create this file if the scaffold didn't (or edit it if it did)
// Source: https://reactrouter.com/how-to/spa (CITED)
import { type Config } from "@react-router/dev/config";

export default {
  ssr: false,
} satisfies Config;
```
With `ssr: false`, `react-router build` no longer emits `build/server/` at all — only `build/client/` (a build-time-prerendered `index.html` for the root route plus all static assets), which is the directory `go:embed` should target. Routes other than the root cannot have a `loader`/`action` in this mode — use `clientLoader`/`clientAction` (or, for this phase's scope, plain `useEffect`/`fetch` in components, consistent with the locked "plain fetch + state" data-fetching decision) [CITED: reactrouter.com/how-to/spa].

### Pattern 2: Keyset (cursor) pagination for `GET /events`

**What:** Paginate the events feed using `id DESC` as the sole ordering/cursor key, with `artist_id`/`event_type` as optional nullable filters.
**When to use:** The new `ListEvents` sqlc query (D-05, D-06, D-07).
**Why `id`, not `created_at`:** This codebase's own `queries/events.sql` already documents why: `ListUnnotified`'s comment states plainly —

> `-- name: ListUnnotified :many`
> `-- D-11's Phase 5 groundwork: SELECT WHERE notified_at IS NULL, ORDER BY`
> `-- created_at ASC, id ASC for a deterministic total order (a plain`
> `-- created_at ordering alone is not unique -- a seed cycle's rows share one`
> `-- timestamp, see seedNotifiedAt).`
[VERIFIED: queries/events.sql:34-40]

A seed cycle (first poll of a newly-watched artist) inserts many rows in one batch, all sharing the same `now()`-derived `created_at`. Ordering the history feed by `created_at` alone would make page boundaries non-deterministic for those rows. `id` (BIGSERIAL) is already unique and monotonic and needs no secondary tiebreak column — simpler than `ListUnnotified`'s `created_at, id` compound key, and sufficient here since the feed doesn't need "sorted by detection time" precision beyond insertion order.

**Example:**
```sql
-- queries/events.sql — add this query
-- name: ListEvents :many
-- Keyset pagination on id DESC (see 06-RESEARCH.md Pattern 2 for why id, not
-- created_at). artist_id and event_type are both optional filters (D-06):
-- sqlc.narg produces a nullable parameter for each; "IS NULL OR" makes an
-- absent filter match every row while keeping one static SQL string sqlc can
-- type-check (no dynamic SQL-string building). cursor is also sqlc.narg'd:
-- absent on the first page, set to the last row's id on subsequent pages.
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
This mirrors `UpdateWatchlistPreferences`'s existing `@`-named-parameter convention in this repo [VERIFIED: queries/watchlist.sql:38-56, quoted: `WHEN @set_release_types::boolean THEN @release_types::text[] ELSE watchlist.release_types`] and sqlc's own documented `sqlc.narg`/`coalesce` idiom for optional filters [CITED: docs.sqlc.dev/en/latest/howto/named_parameters.html, quoted: `name = coalesce(sqlc.narg('name'), name)`]. A response envelope should carry the next cursor explicitly (e.g. `{"events": [...], "next_cursor": 1234}`, `next_cursor: null` when the page returned fewer than `page_size` rows) so the client never has to compute it from the last row itself.

### Pattern 3: `go:embed` + chi SPA fallback

**What:** Serve the built frontend as static files, falling back to `index.html` for any path chi doesn't otherwise match.
**When to use:** Wiring point for the whole embedded-frontend architecture.
**Example (general pattern, synthesized from multiple sources — not copied verbatim from one, since no single canonical Go stdlib doc covers this exact chi+embed combination):**
```go
//go:embed all:build/client
var webFS embed.FS

// spaHandler serves the embedded SPA build: a request matching an actual
// static file (e.g. /assets/index-abc123.js) is served directly; anything
// else falls back to index.html so React Router's client-side router can
// take over (Pattern 1's SPA Mode output has exactly one index.html).
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
Registered as `r.NotFound(spaHandlerInstance)` on the chi router — API routes registered explicitly (`/health`, `/search`, `/watchlist`, `/events`) are matched first and never fall through to this handler, so there is no ordering conflict [CITED: general go:embed+SPA pattern, cross-referenced across nuro.dev, ofeng.org, bindplane.com — see Sources]. Because `all:build/client` (not plain `build/client`) is used in the embed directive, any Vite-emitted asset directory beginning with `.` or `_` is included too — a defensive default even though typical Vite output (`assets/`) doesn't need it.

### Anti-Patterns to Avoid
- **Installing `react-router-dom` alongside `react-router`:** v7 unified the two packages; the framework-mode scaffold (`create-react-router`) will already have the correct single `react-router` dependency. Adding `react-router-dom` on top creates two competing router instances.
- **Running `pnpm dlx shadcn@latest init` without first confirming `ssr: false` afterward:** the default scaffold is framework-mode-with-SSR; skipping the SPA Mode config produces a `build/server/` bundle nothing in this Go-only-production architecture will ever run.
- **Offset pagination (`LIMIT`/`OFFSET`) for the events feed:** the background poller (Phase 4) inserts new rows continuously; offset pagination against a table with concurrent inserts causes skipped or duplicated rows as the user scrolls (classic "phantom row" pagination bug) — keyset pagination on `id` doesn't have this problem.
- **Building a dynamic SQL string in Go for the filter logic:** breaks sqlc's static-query model; use `sqlc.narg()` + `IS NULL OR` instead (Pattern 2).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Toast notifications (add-failure, remove-with-undo, PATCH-failure — Copywriting Contract) | Custom toast/snackbar component + timer/dismiss logic | `sonner` via `shadcn add sonner` (already locked in `06-UI-SPEC.md`) | Undo-action toasts with auto-dismiss timers have real edge cases (stacking, dismiss-on-click-elsewhere, accessibility/`aria-live`) that `sonner` already solves; D-16's dark-theme empty states also lean on the same visual language. |
| Accessible interactive primitives (Tabs, Switch, Checkbox) | Hand-rolled `<div>`-based tab bar / toggle with manual keyboard handling | shadcn's Radix-backed components (already scoped in the UI-SPEC's Registry Safety table) | Radix primitives handle ARIA roles, focus management, and keyboard navigation (arrow-key tab switching, space/enter toggling) that are easy to get subtly wrong by hand and are exactly the kind of "portfolio-grade polish" D-13 wants without extra time investment. |
| SQL string composition for optional filters | Go-side conditional `WHERE` clause building (string concatenation or a query builder library) | `sqlc.narg()` + `IS NULL OR` (Pattern 2) | Preserves sqlc's compile-time-checked, injection-safe query model already used everywhere else in this codebase; a Go-side query builder would be the only one in the codebase and a maintenance outlier. |
| Cursor token encoding | Base64/JSON-encoded opaque cursor with embedded HMAC signature | Raw integer `id` as the cursor query param | At this app's scale (single operator, no multi-tenant data-leak risk — the `id` value reveals nothing sensitive, this isn't a public multi-user API), an opaque/signed cursor is pure complexity with no corresponding threat it defends against. Revisit only if this API ever becomes multi-tenant. |

**Key insight:** Every "don't hand-roll" item above already has a locked, in-repo, or already-approved answer (sqlc conventions from Phases 2–5, shadcn components from `06-UI-SPEC.md`) — this phase's actual net-new hand-rolling surface is small: the `GET /events` handler/query and the `go:embed` wiring, both of which follow established patterns rather than needing a new library.

## Common Pitfalls

### Pitfall 1: `shadcn --template react-router` scaffolds SSR framework mode, not a static SPA
**What goes wrong:** Running the UI-SPEC's locked init command and proceeding straight to `react-router build` produces a `build/server/` Node.js server bundle that this project's Go-only production environment cannot run, alongside a `build/client/` that alone would work fine with `go:embed`.
**Why it happens:** `shadcn init --template react-router` delegates to `create-react-router`, React Router's own official scaffolding tool, whose default output is framework mode (SSR-capable) [CITED: ui.shadcn.com/docs/installation/react-router — the page confirms `create-react-router` is invoked, but does not itself flag the SSR-by-default behavior; SSR-by-default is confirmed via reactrouter.com's own docs].
**How to avoid:** Add (or edit) `react-router.config.ts` with `ssr: false` immediately after scaffolding, before writing any route code (Pattern 1). Verify by running `react-router build` once early and confirming only `build/client/` appears (no `build/server/`).
**Warning signs:** A `build/server/` directory appears after the first build; `package.json`'s `start`/`serve` script references `@react-router/serve` or an Express entrypoint — neither should be needed or invoked in this project's production path.

### Pitfall 2: Ordering the history feed by `created_at` alone breaks keyset pagination
**What goes wrong:** Two events inserted in the same seed-mode batch (identical `created_at`) land unpredictably relative to each other across paginated requests — a row can be skipped or duplicated across a "Load more" click.
**Why it happens:** Postgres does not guarantee stable ordering for rows with equal sort-key values unless the `ORDER BY`/keyset predicate includes a tiebreak column — this codebase's own `ListUnnotified` query hit and documented this exact issue already (see Pattern 2's verbatim quote).
**How to avoid:** Use `id DESC` (unique, monotonic) as the sole cursor/order column for `ListEvents`, not `created_at`.
**Warning signs:** A manual test that seeds several events for one artist in a single poll cycle, then pages through the history feed with a small page size, and observes a row appearing on two consecutive pages or never appearing at all.

### Pitfall 3: `POST /watchlist`'s D-11 "Already watching" state silently breaks if `GET /watchlist` hasn't loaded yet
**What goes wrong:** If the search UI cross-references search results against the watchlist (D-11) but the watchlist hasn't finished its own initial fetch, every search result appears addable — including ones already watched — and clicking "Add" surfaces the 409 backstop as a confusing generic failure toast rather than the intended disabled-button UX.
**Why it happens:** D-11 explicitly makes the 409 a "defensive backstop, never the primary UX signal" — but that assumes the watchlist snapshot used for cross-referencing is actually loaded by the time a search result renders.
**How to avoid:** Fetch `GET /watchlist` on mount of the Watchlist tab (before/alongside rendering the search box), and disable the search box (or show a loading state) until that initial fetch resolves — or accept a brief window where "Already watching" isn't yet enforced and rely on the 409 backstop exactly as D-11 anticipates for that narrow window.
**Warning signs:** A fast typer/clicker on a freshly-loaded Watchlist tab can add a duplicate and see the generic "Couldn't add this artist" toast instead of a disabled button.

### Pitfall 4: Local dev CORS/proxy mismatch between Vite's dev server and the Go API
**What goes wrong:** In production, the SPA is same-origin (served by the same Go binary), so no CORS headers are needed. In local development, `react-router dev` runs its own dev server (typically port 5173/3000) separate from the Go API (its own configured port), making every `fetch` call cross-origin during development only.
**Why it happens:** The dev-vs-prod serving split is real: dev mode never touches `go:embed` at all (Vite's dev server serves the app directly with HMR), while prod mode serves everything from the Go binary.
**How to avoid:** Either add a Vite dev-server proxy (`server.proxy` in `vite.config.ts` forwarding `/health`, `/search`, `/watchlist`, `/events` to the Go server's port) so dev-mode requests are same-origin from the browser's perspective, or add a permissive-but-explicit CORS middleware in the Go server gated to only fire when built with a dev flag/env var (never in the production image). The proxy approach is simpler and doesn't touch backend code at all — prefer it.
**Warning signs:** `fetch` calls succeed when the built app is served by the Go binary but fail with CORS errors only when running `react-router dev` directly against a separately-running `go run ./cmd/server`.

### Pitfall 5: React escapes JSX text by default — don't undo that for externally-sourced titles
**What goes wrong:** Event `title`/`artist_name`/recording titles originate from MusicBrainz/Deezer (external, untrusted-ish input) and are stored verbatim (`InsertEvent`'s `ON CONFLICT DO NOTHING`, write-once, per D-20). If a future change renders any of these fields via `dangerouslySetInnerHTML` (e.g. to support rich formatting), a maliciously-crafted upstream title becomes a stored-XSS vector.
**Why it happens:** JSX's default `{title}` text-node interpolation is auto-escaped and safe; the risk only appears if someone reaches for `dangerouslySetInnerHTML` later (e.g. to bold a search match or render a link inline).
**How to avoid:** Render all event/artist/search-result text fields as plain JSX text nodes (the default). If a future need arises to render a clickable link inside the `guest_feature` card's "recording title + link" (D-08), construct the `<a href>` from a validated/known-scheme URL (or build the link client-side from the MBID rather than trusting a stored raw URL string), never inject raw HTML.
**Warning signs:** Any `dangerouslySetInnerHTML` appearing in a diff touching `history/` or `watchlist/` components should be treated as a review flag.

## Code Examples

### Optimistic preference toggle with rollback (D-12)
```typescript
// app/components/watchlist/PreferenceToggles.tsx (illustrative — plain
// fetch + component state per the locked data-fetching decision)
async function toggleReleaseType(entry: WatchlistEntry, type: string, next: boolean) {
  const previous = entry.release_types;
  const optimistic = next
    ? [...previous, type]
    : previous.filter((t) => t !== type);
  setEntries((rows) =>
    rows.map((r) => (r.id === entry.id ? { ...r, release_types: optimistic } : r))
  );

  try {
    const res = await fetch(`/watchlist/${entry.id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ release_types: optimistic }),
    });
    if (!res.ok) throw new Error("patch failed");
  } catch {
    // Roll back to the pre-toggle state and show the locked toast copy.
    setEntries((rows) =>
      rows.map((r) => (r.id === entry.id ? { ...r, release_types: previous } : r))
    );
    toast.error("Couldn't update preferences — try again.");
  }
}
```
This directly implements D-12's "changes immediately... toggle visually reverts to its prior state, optimistic-update rollback" and the exact toast copy locked in `06-UI-SPEC.md`'s Copywriting Contract.

### `ListEvents` handler shape, mirroring `search.go`/`watchlist.go` conventions
```go
// internal/httpserver/events.go (new file — mirrors search.go/watchlist.go)
type eventsResponse struct {
    Events     []Event `json:"events"`
    NextCursor *int64  `json:"next_cursor"`
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
    // parse ?artist_id=&event_type=&cursor=&limit= — each optional,
    // each validated (event_type against watchlist.EventTypes, same
    // allow-list pattern watchlist.go already applies) before the store call.
    // ... writeError(w, http.StatusBadRequest, "...") on validation failure,
    // matching the errorResponse{Error string} shape every existing handler uses.
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `react-router-dom` as a separate package from `react-router` | Unified single `react-router` package | React Router v6.4+/v7 | Installing both is a common stale-tutorial mistake; only `react-router` is needed. |
| Tailwind `tailwind.config.js` (JS-based theme config) | CSS-first `@theme` directive inside the main CSS file | Tailwind v4.0 | Any tutorial/blog post showing `tailwind.config.js` predates v4 and will not directly apply; `06-UI-SPEC.md`'s color/spacing/typography tokens should be expressed as `@theme` CSS variables. |
| `shadcn-ui` npm package name | `shadcn` (renamed) | shadcn CLI v4-era rename | Older tutorials referencing `npx shadcn-ui@latest` are stale; `06-UI-SPEC.md` already uses the correct current name. |

**Deprecated/outdated:**
- Offset-based (`LIMIT`/`OFFSET`) pagination for feeds with concurrent writers: still works, but keyset pagination is the documented current best practice for exactly this "background writer + scrolling reader" shape this app has.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `create-react-router` scaffold invoked by `shadcn init --template react-router` does not already default to SPA Mode / does not already set `ssr: false` — i.e., Pitfall 1 is a real gap, not something the CLI already handles. | Summary, Pattern 1, Pitfall 1 | If wrong (the scaffold already defaults to SPA mode), the extra `react-router.config.ts` edit is a harmless no-op — low risk either way, but the plan should still verify by inspecting the generated `react-router.config.ts` immediately after scaffolding rather than assuming. |
| A2 | `@vitejs/plugin-react` is not needed as a separate install because `@react-router/dev/vite`'s own Vite plugin already provides React JSX/Fast-Refresh support. | Package Legitimacy Audit (`@vitejs/plugin-react` row) | If wrong, JSX transform/HMR may not work until it's added manually — low risk, easy to diagnose (dev server errors immediately), should be resolved by inspecting the scaffold's generated `vite.config.ts` rather than pre-installing. |
| A3 | A new database index is not required for the `ListEvents` query at this project's expected data scale (single-operator watchlist, likely dozens to low hundreds of events total). | Architecture Patterns (Pattern 2), Don't Hand-Roll | If the artist list or event volume grows much larger than expected, `artist_id`/`event_type` filtered scans without a supporting index could slow down — low risk for this project's stated scale (`06-UI-SPEC.md` itself notes "no virtualization needed at expected single-operator scale"), but the planner should note this as a deferred optimization, not silently skip considering it. |
| A4 | `typescript`'s npm `latest` dist-tag (`7.0.2` at research time) is a pre-release/experimental major version, and a stable `5.x` should be installed instead. | Standard Stack | If wrong (7.x is actually stable by execution time), pinning to an older 5.x unnecessarily forgoes new features — low risk, easily corrected by checking `npm view typescript dist-tags` at execution time as the Standard Stack note already instructs. |

**If this table is empty:** N/A — see entries above.

## Open Questions (RESOLVED)

1. **Does the shadcn `react-router` preset (`bdKQpIA4`) already configure SPA Mode, or is this phase's plan the first place `ssr: false` gets added?** — Resolved by planning: 06-01-T2 inspects the generated `react-router.config.ts` after scaffolding and adds `ssr: false` if not already present, then verifies `build/client` exists and `build/server` does not.
   - What we know: The generic `create-react-router` scaffold defaults to framework mode with SSR; `06-UI-SPEC.md` does not mention SPA Mode or `ssr: false` anywhere, and its own "Execution note" only says `components.json does not yet exist... must be run as the first task."
   - What's unclear: Whether the specific preset `bdKQpIA4` (opaque preset ID, not inspectable without running the CLI) bundles a non-default `react-router.config.ts`.
   - Recommendation: The plan's first task should be "run the init command, then immediately inspect (not assume) the generated `react-router.config.ts` and add `ssr: false` if not already present" — a cheap, always-correct verification step regardless of the preset's actual default.

2. **Should the events response envelope include a `has_more` boolean in addition to `next_cursor`, or is `next_cursor: null` sufficient as the "no more pages" signal?** — Resolved by planning: `next_cursor: null` alone drives "hide Load more" in 06-02; no `has_more` field was added.
   - What we know: D-07 requires "Load more" to stop being clickable/visible once the feed is exhausted; UI-SPEC's Copywriting Contract doesn't specify a distinct "end of feed" message (only per-item empty states).
   - What's unclear: Whether `next_cursor: null` alone is unambiguous enough (it is, functionally — a null cursor means "don't request another page") or whether the UI wants an explicit `has_more` field for clarity.
   - Recommendation: `next_cursor: null` is sufficient and requires no `has_more` field; the plan can note in the design that "no next_cursor" == "hide Load more" without needing a Claude's-Discretion checkpoint on this.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | Frontend build tooling (Vite, React Router CLI) | ✓ | v22.21.1 | — |
| npm | Fallback package manager | ✓ | 10.9.4 | — |
| pnpm | UI-SPEC's locked scaffold command uses `pnpm dlx` | ✓ | 11.8.0 | — |
| Go | Backend build/test | ✓ | go1.26.5 windows/amd64 | — |
| Docker | `db-up`/`db-down` Make targets (Postgres for integration tests) | ✓ | 29.6.2 | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — every tool this phase needs is already installed on this dev machine.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | Go `testing` + `net/http/httptest` + `internal/testutil.NewTestPool` (real Postgres) [VERIFIED: internal/httpserver/watchlist_test.go:1-29, internal/testutil/postgres.go exists] — already established, no new backend framework needed. |
| Frontend framework | none configured yet — greenfield (Wave 0 gap, see below) |
| Config file | Backend: none needed (stdlib `go test`). Frontend: none yet — see Wave 0 Gaps. |
| Quick run command | Backend: `go test ./internal/httpserver/... -short -race -count=1` (mirrors Makefile's `test-short` target, scoped to the new package). |
| Full suite command | `make test` (runs `test-integration`, which brings up Postgres via `db-up` first) [VERIFIED: Makefile:28-31]. |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|-------------|
| HIST-01 | `GET /events` returns keyset-paginated, artist/event-type-filtered results in the documented shape | integration (real Postgres) | `go test ./internal/httpserver/... -run TestHandleListEvents -race -count=1` | ❌ Wave 0 — new file `internal/httpserver/events_test.go` |
| HIST-01 | `ListEvents` sqlc query correctly excludes rows on the wrong side of the cursor, applies both filters independently | unit/integration (real Postgres, via `internal/testutil`) | `go test ./internal/db/... -run TestListEvents -race -count=1` (or co-located with the events package test) | ❌ Wave 0 |
| UI-01, UI-02, UI-03 | Search → add → manage → view history end-to-end via the web UI | manual-only (UAT) | — | N/A — justification: this project's CLAUDE.md testing convention (`httptest.Server`-mocked unit tests) targets backend/API correctness; no frontend test framework exists yet, and CLAUDE.md's stated Core Value is CI/CD pipeline maturity, not UI test coverage. Automating full browser interaction (Playwright/Cypress) for a 2-tab MVP frontend is disproportionate to this project's stated goals — recommend manual UAT via `/gsd-verify-work` instead, unless the planner has budget to also stand up a frontend test runner. |
| UI-01 (D-11) | "Already watching" client-side cross-reference logic | unit (frontend, if a test runner is added) or manual-only | `pnpm test` (if Vitest is added) or manual click-through | ❌ Wave 0 gap, optional — see below |

### Sampling Rate
- **Per task commit:** `go test ./internal/httpserver/... -short -race -count=1` for backend changes; manual click-through in a running dev server for frontend changes.
- **Per wave merge:** `make test` (full backend suite against real Postgres).
- **Phase gate:** Full backend suite green + manual UAT walkthrough of all three success criteria before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/httpserver/events_test.go` — covers HIST-01 (new handler, mirrors `watchlist_test.go`'s stub-store + real-Postgres dual pattern)
- [ ] New sqlc query test for `ListEvents` (co-located per this repo's existing convention — check whether `internal/db/sqlc_test.go` or a package-level test is the established location for query-level tests, since events.sql's other queries don't appear to have a dedicated query-level test file separate from the handler/service tests that exercise them)
- [ ] **Decision needed (planner/discretion):** whether to introduce a frontend test framework (Vitest + `@testing-library/react` is the standard Vite-ecosystem pairing) for D-11's "Already watching" logic and the optimistic-rollback logic, or accept manual-only UAT coverage for the entire frontend given this project's backend/pipeline-focused Core Value. Recommendation: skip a frontend test framework for this phase — it would be the single largest scope addition in the phase and CLAUDE.md's own testing philosophy is backend-focused; rely on manual UAT for the frontend half.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | Explicitly out of scope per REQUIREMENTS.md ("Multi-user auth / accounts / SSO" — single-operator deployable). |
| V3 Session Management | No | No sessions/cookies introduced by this phase. |
| V4 Access Control | No | Same as V2 — no auth boundary exists to enforce. |
| V5 Input Validation | Yes | New `GET /events` query params (`artist_id`, `event_type`, `cursor`, `limit`) must be validated the same way `parseWatchlistID` and the existing `release_types`/`muted_event_types` allow-list checks already do in `watchlist.go` — reject non-numeric `artist_id`/`cursor`, reject `event_type` values outside `watchlist.EventTypes`, cap `limit` to a sane maximum (e.g. 100) to prevent an unbounded page-size DoS via query param. |
| V6 Cryptography | No | No cryptographic operations introduced by this phase. |
| V13 API and Web Service | Yes | Mirror the existing `errorResponse{Error string}` convention — never echo raw DB/driver error text in the `GET /events` response (matches every other handler's `httplog.SetAttrs` + fixed operator-authored message pattern already verified in `watchlist.go`/`search.go`). |

### Known Threat Patterns for React SPA + Go JSON API

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Stored XSS via externally-sourced titles/artist names rendered client-side | Tampering / Elevation of Privilege | React's default JSX text-node escaping (never use `dangerouslySetInnerHTML` on `title`/`artist_name`/recording-title fields) — see Pitfall 5. |
| SQL injection via `artist_id`/`event_type`/`cursor` query params | Tampering | sqlc's parameterized queries (already the pattern for every existing query in this repo) — no string concatenation into SQL. |
| Unbounded `limit` query param causing large result-set DoS | Denial of Service | Clamp `limit` server-side to a fixed maximum regardless of what the client requests (mirrors `searchResultLimit`'s fixed-cap precedent in `search.go`). |
| Dev-mode CORS misconfiguration leaking into production | Information Disclosure | See Pitfall 4 — prefer a Vite dev-server proxy over adding CORS middleware to the Go server at all, so production never has a CORS surface to misconfigure. |
| Untrusted `image_url`/`cover_art_url` rendered as `<img src>` | Spoofing (low severity) | These already pass through Phase 2/5 without format validation by design (`T-02-18`: "rendering-surface validation for image_url is... Phase 6 (React UI)'s responsibility") — browsers do not execute `<img src>` as script, so the residual risk is limited to displaying an unexpected image, not code execution; no additional validation needed for MVP scope beyond the existing null-fallback placeholder (D-14). |

## Sources

### Primary (HIGH confidence — direct in-repo verification)
- `internal/httpserver/server.go`, `search.go`, `watchlist.go` — existing chi router wiring, handler conventions, `errorResponse` shape (read in full this session)
- `internal/db/sqlc/models.go` — `Event` struct field names/types (read in full this session)
- `queries/events.sql`, `queries/watchlist.sql` — existing sqlc query conventions, the `ListUnnotified` ordering-pitfall comment, the `UpdateWatchlistPreferences` `@`-named-parameter pattern (read in full this session)
- `internal/watchlist/service.go` — `Store` interface pattern, `ReleaseTypes`/`EventTypes` allow-lists (read in full this session)
- `internal/db/migrations/000003_events.up.sql`, `000004_events_display_fields.up.sql` — `events` table schema, existing indexes (read in full this session)
- `cmd/server/main.go`, `Makefile` — process wiring, existing test/build targets (read in full this session)
- `.planning/phases/06-frontend-release-history/06-UI-SPEC.md` — approved design contract (read in full this session)

### Secondary (MEDIUM confidence — official docs, WebFetch/WebSearch cross-checked)
- reactrouter.com/how-to/spa — SPA Mode config (`ssr: false`), `build/client` output, `@react-router/node` requirement, loader/clientLoader caveat
- ui.shadcn.com/docs/installation/react-router — confirms `create-react-router` scaffold origin, Tailwind auto-config
- tailwindcss.com/docs/installation/using-vite — v4 + Vite install steps, CSS-first config confirmation
- docs.sqlc.dev/en/latest/howto/named_parameters.html — `sqlc.narg`/`coalesce` optional-parameter pattern
- npm registry (`npm view <pkg> version/homepage/maintainers`) — all Standard Stack version and legitimacy claims

### Tertiary (LOW confidence — WebSearch synthesis, not independently re-verified)
- go:embed + chi SPA fallback pattern (nuro.dev, ofeng.org, bindplane.com, tushar.ch — cross-referenced but the exact chi-specific code was synthesized from the general Go pattern, not copied from a single chi-specific canonical source)
- react-router.config.ts `buildDirectory`/structure detail beyond the `ssr` flag

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH for version numbers (npm-verified) / MEDIUM for exact scaffold behavior (docs-verified but not executed in-repo yet)
- Architecture: HIGH for backend (verified against this repo's actual code) / MEDIUM for frontend (official docs, not yet run)
- Pitfalls: HIGH for Pitfall 1 and 2 (directly sourced from official docs + this repo's own code comments) / MEDIUM for Pitfalls 3-5 (reasoned from locked decisions, not independently tested)

**Research date:** 2026-08-10
**Valid until:** 7 days for the frontend npm package versions (fast-moving: React/Vite/Tailwind all shipped within the last month) — re-run `npm view` checks if planning is delayed; 30 days for the backend/architecture patterns (stable, verified against this repo's own established conventions).
