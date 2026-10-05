# Phase 25: Find & Filter — Watchlist and History - Research

**Researched:** 2026-10-05
**Domain:** Postgres/sqlc query enrichment (`LEFT JOIN LATERAL`), Go chi handler param, React Router 7 URL state, client-side filter/sort view-model, base-ui multi-select Combobox, hand-rolled History combobox
**Confidence:** HIGH (backend, every SQL shape executed against the dev Postgres and run through `sqlc generate`); MEDIUM-HIGH (frontend; two base-ui behaviours are cited, not exercised — see Assumptions Log)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Watchlist toolbar & layout**
- **D-01:** The add-artist `SearchBox` (MusicBrainz/Deezer) stays at the top of the page, unchanged. A separate list toolbar sits directly above the artist list. It holds the name filter (labelled distinctly, e.g. "Filter watchlist…"), the sort control, the tag filter, the two preference toggles, and the "N of M artists" count. The two inputs must never be confusable by label or placeholder.
- **D-02:** The default sort is **name A–Z**, which matches today's server order (`ListWatchlist` orders by name, then id), so the default view doesn't change.
- **D-03:** Every Watchlist card always shows a muted latest-release line, either at stored precision (D-14) or as "No releases yet" when there is none. This deliberately reverses the Phase 06 D-03/D-04 "no release activity on the Watchlist" posture. Update the module comment in `web/app/routes/watchlist.tsx` that cites it.
- **D-04:** The name search is a case-insensitive, **accent-insensitive** substring match on the artist name only, not the disambiguation (NFD normalize, strip combining marks, lowercase, then `includes`). For example, `beyonce` matches Beyoncé. It filters as the user types.

**Tag & preference filters**
- **D-05:** The multi-tag Watchlist filter is **any-of (OR)**. Label it to match, e.g. "Tags: any of".
- **D-06:** The Watchlist tag filter uses the **base-ui Combobox** Phase 24 vendored (`web/app/components/ui/combobox.tsx`) in `multiple` mode with chips. Reuse or extend `TagCombobox` where it fits, and don't build a third combobox.
- **D-07:** The Watchlist tag filter's options and names come **from the loaded watchlist payload** (the union of `entries[].tags`), not `GET /tags`. That keeps opening the Watchlist to one request (SC5, Phase 24 D-30) and drops tags only removed artists carry, which would match nothing on the Watchlist anyway.
- **D-08:** Clicking a tag chip on a Watchlist card **toggles that tag in the filter**. The chip body becomes a button with an accessible name like "Filter by reggaeton". The existing × still detaches the tag (Phase 24 D-04 deferred this behavior here). Keep the two hit targets visually and accessibly distinct.
- **D-09:** Preference filters are **two independent toggles**: "Has muted events" (`muted_event_types` non-empty) and "Custom release types" (`release_types` ≠ the default full set `album, single, ep, deluxe`, compared as a set). Like every other filter, they combine with AND.
- **D-10:** Search, sort, the tag filter, and the preference toggles all compose. When the combination matches nothing, the view shows a distinct empty state with a single "Clear filters" action that resets search and every filter. Sort is not a filter, so it is preserved. Keep three empty states distinct: no artists on the watchlist, filters matched nothing, and (on History) no events for this tag (FEATURES.md).
- **D-11:** History's tag filter is **single-select** and uses a scalar `tag_id` narg, as the roadmap specifies. It reuses the hand-rolled `Combobox<T>` in `web/app/components/history/HistoryFilters.tsx` so it matches its Artist and Event-type siblings. Its options come from `GET /tags`, which also lists tags only removed artists carry, because History shows events for removed artists.

**State persistence**
- **D-12:** Watchlist search, sort, tag filter, and preference toggles live in **URL query params** via React Router 7 `useSearchParams`. Typing in the search box updates with `replace`, never push, so each keystroke doesn't add a history entry. History's artist, event-type, and new tag filters move to URL params the same way. Param names and the sort value encoding are left to the planner, but they must be stable and human-readable (e.g. `?q=bad&sort=latest-desc&tags=3,7&muted=1&custom=1`, and `/history?tag=3`).
- **D-13:** Stale or invalid params are **dropped silently** and the URL is rewritten with `replace`. This covers unknown tag ids (not present in the payload-derived set), an unknown sort value, and malformed numbers. When Manage tags deletes or merges a tag that is in the active filter, the filter updates: a deleted tag drops out, and a merged tag swaps to the merge target. The Watchlist and History filters stay **independent**, with no cross-route link (Phase 06 D-03/D-04 decoupling).

**Latest-release semantics**
- **D-14:** `latest_release_date` is the max `release_date` over that artist's `event_type = 'new_release'` events, **within the retention window**. It uses the same `created_at >= cutoff` predicate ListEvents uses, so the Watchlist never shows a date History can't show (PITFALLS #8). An artist whose releases have all aged out falls into the "no release" group. Retention keys on detection time (`created_at`), so a newly added artist's seed-mode back catalogue counts until it ages out, exactly as it does in History. — **Reversibility:** reversible — it's a query predicate, and there's no schema change. *(Retention clause superseded by G1 below.)*
- **D-15:** Sort on the raw text lexicographically, which is the property ListEvents already relies on: `2024` sorts as the start of 2024, with ties broken by artist id. Display at stored precision ("2024", "May 2024", "17 May 2024"). Future and announced dates count and display as-is. NULL or none sorts last in both directions (WLVW-03). *("Future dates as-is" superseded by G2/G3 below.)*
- **D-16:** Every sort mode tie-breaks on the artist id. Name sort uses `Intl.Collator` (case-insensitive) and mirrors the digest's collated sort. Date added sorts on the watchlist row's created timestamp.

### Claude's Discretion
- The sort control's shape (a single select of six options vs a field plus a direction toggle), toolbar wrapping at phone width, and copy for labels, empty states, and the count. The UI phase pins these, and the `ui-ux-pro-max` skill applies to visual choices.
- Whether to debounce the search input. Client-side filtering of about 100 rows doesn't need it, but URL writes might.
- The exact `LEFT JOIN LATERAL` form, and whether `EXPLAIN` justifies a pure-additive index migration (roadmap note).
- How the cutoff reaches the `ListWatchlist` query (a `sqlc.arg('cutoff')`, consistent with ListEvents' non-optional cutoff). *(Moot: G1 removes the cutoff entirely.)*

### Post-discuss grill overrides (2026-10-04) — these WIN over D-xx and 25-UI-SPEC.md where they conflict
- **G1 (supersedes D-14's retention clause and the roadmap's PITFALLS #8 note):** `latest_release_date` **ignores retention**. It is the newest date among all of the artist's stored `new_release` events, at any age. ListWatchlist and GetWatchlistEntry take **no cutoff**, and `watchlist.Service` does not get the retention setting. With the 90-day default, a retention-scoped value would leave most artists at "No releases yet" and collapse the sort. The accepted cost is that the Watchlist can show a date History no longer lists, and the routes stay unlinked (D-13).
- **G2 (supersedes D-15's "future dates as-is"):** a date is **upcoming** when it is strictly after UTC `CURRENT_DATE` at its own precision. `2027` and `2026-11` are upcoming on 2026-10-04, but `2026` and `2026-10` are not. Postgres decides this inside the same LATERAL join, so the client never compares dates.
- **G3 (extends D-03/D-14):** the payload carries two fields. `latest_release_date` is the newest non-upcoming date, and it alone drives the latest-release sort. `next_release_date` is the **nearest** upcoming date, shown as a muted `Upcoming: {date}` line, and it never affects the sort. Both come from the single `GET /watchlist` query and the shared POST/PATCH projection (Phase 24 D-26).
- **G4 (supersedes UI-SPEC's card-removal focus flow and announcement):** **sticky cards.** An in-card edit never removes its own card from a filtered view, and the card gets no cue. Stickiness clears on a change to search text, sort, tag selection, a toggle, Clear filters, or a chip-click filter, and on reload. "N of M" counts **visible** cards.
- **G5 (extends D-04):** the name filter adds a fold table after removing accents: `$→s`, `ø→o`, `æ→ae`, `ß→ss`, applied to both the query and the name. `asap` matches A$AP Rocky.
- **G6 (D-12 sequencing):** History's move to URL state stays in scope as **its own plan**, separate from the HIST-02 tag filter, so the tag filter can land and be verified on its own.
- **G7 (confirms D-11):** both comboboxes stay, with the UI-SPEC's hand-rolled `Combobox<T>` hardening. A backlog todo covers unifying them on base-ui.

### Deferred Ideas (OUT OF SCOPE)
- A cross-route "See releases for #tag" link from a single-tag Watchlist filter to `/history?tag=N`. Considered and declined for now to keep the tabs decoupled. It would be cheap later, since the URL params exist.
- An any/all toggle for the multi-tag filter. Any-of only for now.
- Out of scope per CONTEXT domain: server-side paging/sorting (REQUIREMENTS.md Out of Scope), multi-select and bulk actions (Phase 26), and tags on Discord (Phase 28).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WLVW-01 | Search the watchlist by artist name | `foldName()` pure helper (verified fold table, §Code Examples); live filtering over the loaded payload; Pitfall 8 (q debounce vs URL) |
| WLVW-02 | Sort by name A–Z/Z–A or date added, stable tie-break | Comparator design (§Pattern 4): collator for name, `Date.parse` for `created_at`, artist-id **ascending** tie-break in every direction |
| WLVW-03 | Sort by latest release (newest own `new_release` date, not upcoming); none sorts last in both directions | Two `LEFT JOIN LATERAL … LIMIT 1` joins (verified, §Pattern 1); null-last comparator that is NOT `.reverse()`; sqlc nullability trap (Pitfall 1) |
| WLVW-04 | Filter by one or more tags | Any-of OR over payload-derived tag union; base-ui `multiple` chips combobox with `item-press` close cancel (§Pattern 5); chip-click toggle (§Pattern 6) |
| WLVW-05 | Filter to artists with muted event types or non-default release types | Two AND toggles; set comparison against `{album,single,ep,deluxe}` |
| WLVW-06 | Search, sort, filters combine; "N of M artists"; empty state with clear-filters | Pure `selectVisible()` pipeline + sticky set (G4); three distinct empty states; URL-state helper (§Pattern 3) |
| HIST-02 | Filter History by a tag, combined with existing filters, respecting retention | Third `sqlc.narg('tag_id')` `EXISTS` over `artist_tags` in BOTH `ListEvents` and `HasOlderEvents` (verified with sqlc); `TestRetention_*`-style regression; History Tag combobox + hardening |
</phase_requirements>

## Summary

Phase 25 is four mostly independent seams on top of Phase 24: (1) a backend enrichment of the single `GET /watchlist` query with `latest_release_date` and `next_release_date`; (2) a backend `tag_id` predicate inside the existing History queries; (3) a client-side view-model (fold, filter, sort, select-visible) plus a toolbar, URL state, and sticky cards on the Watchlist route; (4) a History Tag control and, as its own plan, History's move to URL state. **No new Go or npm dependencies, and no migration is needed.** I ran the proposed SQL against the dev Postgres: with 9 watched artists and 3,246 events the planner uses the existing `events_artist_source_idx` and the whole enrichment costs about 0.3 ms per artist (a 100-artist extrapolation is ~30 ms, tagged ASSUMED), and the History `EXISTS` is planned as a semi-join on `artist_tags_tag_id_idx`. An index is not justified by `EXPLAIN`, so `internal/db/migrations/README.md` is not triggered.

The single most expensive trap is in sqlc: the "obvious" single-LATERAL aggregate (`max(...) FILTER (...)`) generates `interface{}` without a `::text` cast and a **non-nullable `string` with** one, which would fail at runtime with a pgx NULL-scan error for every artist with no releases. Plain-column `LEFT JOIN LATERAL (SELECT release_date … ORDER BY … LIMIT 1)` — two of them, one for non-upcoming/DESC and one for upcoming/ASC — generates `*string` correctly (verified by running `sqlc generate` v1.31.1 on a scratch copy). The other expensive traps are on the frontend: `vi.mock("~/lib/api")` auto-mocks every export of `api.ts`, so pure helpers must live elsewhere; `TagChips`' focus logic indexes `querySelectorAll("button")` by chip index and silently breaks the moment each chip gains a second button; the debounced `q` URL write captures a stale `searchParams` and will clobber a concurrent sort change; and base-ui's multi-select closes the popup on item-press by default while the UI-SPEC requires it to stay open.

**Primary recommendation:** Land the backend first as two small independent plans (enrichment; History tag filter), build the frontend on a pure, API-free view-model (`web/app/lib/watchlistView.ts`) with one shared URL-write helper that always passes `{ preventScrollReset: true }` and reads params from a ref, and keep History's URL-state migration as its own later plan (G6).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `latest_release_date` / `next_release_date` (incl. "upcoming at own precision") | Database / Storage (Postgres, in the LATERAL) | API / Backend (struct passthrough) | G2: Postgres decides upcoming with UTC today; the client must never compare dates. One round trip regardless of N (SC5, PITFALLS #8) |
| Name search, sort, tag/preference filters, "N of M" | Browser / Client | — | REQUIREMENTS Out of Scope forbids server paging/sorting; whole watchlist is already loaded; instant, zero requests (SC5) |
| Tag filter option list (union of `entries[].tags` + carrier counts) | Browser / Client | — | D-07: derived from the payload, no `GET /tags` on the Watchlist |
| Watchlist/History filter state persistence | Browser / Client (URL query via `useSearchParams`) | — | D-12; reload and Back reproduce the view |
| History tag filter + retention composition | Database / Storage (one `EXISTS` inside the retention-aware query) | API / Backend (parse `tag_id`, validate) | PITFALLS #9: must compose inside `ListEvents` and `HasOlderEvents`, never a second query |
| Tag option source for History | API / Backend (`GET /tags`, includes removed-artist tags) | Browser / Client | D-11: History shows removed artists' events, so it needs the full vocabulary |
| Embedded SPA delivery | CDN / Static (go:embed bundle) | — | `make web` rebuild + commit is part of every web phase |

## Standard Stack

### Core (all already in the tree — this phase adds zero packages)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| sqlc (CLI) | v1.31.1 | Regenerate `ListWatchlistRow`/`GetWatchlistEntryRow`/`ListEventsParams`/`HasOlderEventsParams` | `[VERIFIED: ran ~/go/bin/sqlc.exe version → v1.31.1]`; Makefile pins `SQLC_VERSION := v1.31.1` |
| jackc/pgx/v5 | v5.10.0 | Scans `*string` NULLs for the two new columns | Already the driver (`sql_package: "pgx/v5"` in sqlc.yaml) |
| go-chi/chi/v5 | v5.3.1 | `GET /events?tag_id=` — no new route | Existing router |
| react-router | 7.18.2 | `useSearchParams` / `setSearchParams(next, { replace, preventScrollReset })` | `[VERIFIED: web/node_modules/react-router/package.json "version"; SetURLSearchParams type at dist/development/index-react-server-client-3ykjivgQ.d.ts:3049]` |
| @base-ui/react | 1.7.0 | `Combobox` `multiple` mode, chips; `Select` | `[VERIFIED: web/node_modules/@base-ui/react/package.json]` |
| lucide-react | 1.31.0 | `ListFilter`, `FilterX`, `Check`, `X` icons | `[VERIFIED: node -e typeof require('lucide-react').ListFilter etc. → object]` |
| vitest + @testing-library | 4.1.10 / 16.3.2 / user-event 14.6.4 | Component + unit tests | Existing harness |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `Intl.Collator` (platform) | n/a | Name sort, tag-name sort | `new Intl.Collator(undefined, { sensitivity: "base" })` per UI-SPEC |
| `String.prototype.normalize` + `\p{M}` (platform) | n/a | Accent folding | D-04/G5; fold table verified (§Code Examples) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Two plain `LEFT JOIN LATERAL … LIMIT 1` | One LATERAL with `max()/min() FILTER` | Single join but sqlc emits `interface{}` or non-null `string` — runtime NULL-scan hazard. **Rejected** |
| `useSearchParams` | nuqs / `use-query-state` libs | New dependency; UI-SPEC says zero new packages. Rejected |
| Hand-rolled debounce | lodash.debounce | New dep for ~10 lines. Rejected |

**Installation:** none. `corepack pnpm --dir web install --frozen-lockfile` is only needed on a fresh clone.

**Version verification:** no new packages, so no `npm view` needed. Installed versions confirmed from `node_modules` package.json files (above).

## Package Legitimacy Audit

No external packages are installed by this phase (UI-SPEC "zero npm packages"; no new Go modules). `gsd-tools query package-legitimacy check` was therefore not run.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                       ┌────────────────────────── Browser (SPA) ──────────────────────────┐
 URL ?q&sort&tags&     │  useSearchParams  ──►  parseWatchlistParams()  (drop invalid)      │
 muted&custom ────────►│        ▲                      │                                    │
                       │        │ writeParams()        ▼                                    │
                       │        │ (ref-based,    selectVisible(entries, params, stickyIds)  │
                       │        │  preventScrollReset)   fold ▸ name ▸ tags(OR) ▸ muted ▸   │
                       │        │                        custom ▸ (∪ sticky) ▸ sort          │
                       │   Toolbar controls ◄──────────────┘                                │
                       │   (filter input, Select, tag combobox, toggles, Clear)             │
                       │   Card chip click ─► toggle tag in params (push, clears sticky)    │
                       │   In-card edit ────► stickyIds.add(id)  (no refilter)              │
                       └─────────┬───────────────────────────────────────▲──────────────────┘
          ONE request on mount   │ GET /watchlist                        │ JSON entries[] incl.
                                 ▼                                       │ latest_release_date,
              ┌──────────────── chi router (auth-gated) ───────────────┐ │ next_release_date
              │ handleListWatchlist ─► watchlist.Service.List ─────────┼─┘
              │            └► sqlc ListWatchlist (single query):       │
              │                 watchlist ⋈ artists                    │
              │                 + ARRAY(tags) ×2   (Phase 24)          │
              │                 + LEFT JOIN LATERAL latest (<= today@precision, DESC, LIMIT 1)
              │                 + LEFT JOIN LATERAL upcoming (> today@precision, ASC,  LIMIT 1)
              │                                                        │
 History:     │ handleListEvents ─ parseOptionalPositiveInt64("tag_id")│
 /history?    │      └► events.Service.List ─► sqlc ListEvents  ┐ both carry the SAME
 artist&type& │                              └► sqlc HasOlderEvents┘ artist/type/tag_id/cutoff
 tag          │              WHERE … AND (tag_id IS NULL OR EXISTS(artist_tags …)) AND created_at ≥/< cutoff
              └────────────────────────────────────────────────────────┘
                 Postgres: events_artist_source_idx (artist_id,source) · artist_tags PK + artist_tags_tag_id_idx
```

### Recommended Project Structure (new/changed files only)
```
queries/watchlist.sql                      # + two LATERAL joins on ListWatchlist AND GetWatchlistEntry (byte-identical select lists)
queries/events.sql                         # + tag_id EXISTS in ListEvents AND HasOlderEvents
internal/db/sqlc/*.go                      # regenerated (commit it; see Pitfall 11)
internal/watchlist/service.go              # Entry += LatestReleaseDate/NextReleaseDate *string; entryFromRow maps them
internal/events/service.go                 # ListParams += TagID *int64; passed to both queries
internal/httpserver/events.go              # parse tag_id via parseOptionalPositiveInt64 → 400 "invalid tag_id"
web/app/lib/api.ts                         # WatchlistEntry += 2 fields; listEvents({tagId})  — types/wrappers ONLY
web/app/lib/format.ts                      # + formatReleaseDate(raw)
web/app/lib/watchlistView.ts   (NEW)       # foldName, matchesName, parse/serialize params, selectVisible, comparators, tag union — PURE, no api import
web/app/lib/useUrlParams.ts    (NEW)       # shared writer: ref-based, always preventScrollReset, replace/push
web/app/lib/test/fixtures.ts   (NEW)       # makeEntry() so 8 test files don't each hand-edit fixtures
web/app/components/watchlist/WatchlistToolbar.tsx   (NEW)
web/app/components/watchlist/WatchlistTagFilter.tsx (NEW — multi chips combobox, NOT TagCombobox)
web/app/components/watchlist/TagChips.tsx           # chip label → toggle button; fix button-index focus logic
web/app/components/watchlist/WatchlistRow.tsx       # latest/upcoming lines
web/app/components/ui/combobox.tsx                  # ComboboxChip: text-muted-foreground hover:text-foreground + removeLabel prop
web/app/components/history/HistoryFilters.tsx       # Tag control, tagId in value, Combobox<T> hardening
web/app/routes/watchlist.tsx / history.tsx          # URL state, sticky ids, empty states
internal/webassets/build/client/                    # rebuilt bundle (`make web` equivalent) committed at phase end
```

### Pattern 1: Single-query enrichment with two plain LATERAL joins (VERIFIED)
**What:** Append to BOTH `ListWatchlist` and `GetWatchlistEntry` (the `sqlc.ListWatchlistRow(row)` conversion in `Service.get` only compiles while both select lists are identical — `[VERIFIED: internal/watchlist/service.go:379]`).
**Why two joins, plain columns:** nullability (Pitfall 1). Postgres computes "upcoming" from `(now() AT TIME ZONE 'UTC')::date`, session-timezone independent, no cutoff argument (G1).
**Example (executed against dev Postgres 16 and `sqlc generate` v1.31.1 → `LatestReleaseDate *string`, `NextReleaseDate *string`):**
```sql
       ARRAY( ... )::text[] AS tag_names,
       latest.release_date AS latest_release_date,
       upcoming.release_date AS next_release_date
FROM watchlist w
JOIN artists a ON a.id = w.artist_id
LEFT JOIN LATERAL (
  SELECT e.release_date FROM events e
  WHERE e.artist_id = a.id AND e.event_type = 'new_release' AND e.release_date IS NOT NULL
    AND e.release_date <= left(to_char((now() AT TIME ZONE 'UTC')::date, 'YYYY-MM-DD'), length(e.release_date))
  ORDER BY e.release_date DESC LIMIT 1
) latest ON true
LEFT JOIN LATERAL (
  SELECT e.release_date FROM events e
  WHERE e.artist_id = a.id AND e.event_type = 'new_release' AND e.release_date IS NOT NULL
    AND e.release_date > left(to_char((now() AT TIME ZONE 'UTC')::date, 'YYYY-MM-DD'), length(e.release_date))
  ORDER BY e.release_date ASC LIMIT 1
) upcoming ON true
ORDER BY a.name ASC, a.id ASC;   -- ListWatchlist only; GetWatchlistEntry keeps WHERE w.id = $1
```
**Why the `left(today, length(release_date))` trick is correct:** it compares same-length strings, so the precision rule falls out. Truth table executed on the dev DB on 2026-10-05: `2026→f, 2027→t, 2026-10→f, 2026-11→t, 2026-10-05→f, 2026-10-06→t, 2000→f` `[VERIFIED: psql run this session]`. Mixed-precision `max`/`min` semantics: `ORDER BY release_date DESC` makes `2026-05-01` beat `2026`, and `ASC` makes `2026-12-25` precede `2027` (start-of-period), consistent with `ListEvents`.
**EXPLAIN (dev DB, 9 artists, 3,246 events, 361 events/artist):** `Nested Loop Left Join` → per-artist `Limit → Sort → Bitmap Heap Scan on events using events_artist_source_idx`, `Execution Time: 2.836 ms` total `[VERIFIED: EXPLAIN ANALYZE this session]`. Verdict: **no index, no migration.**

### Pattern 2: History tag filter composed into the retention-aware query (VERIFIED with sqlc)
Add the same predicate to `ListEvents` (before `created_at >= cutoff`) and to the `EXISTS` body of `HasOlderEvents` (before `created_at < cutoff`). `sqlc.narg('tag_id')` appearing twice maps to one param; generated `TagID *int64` on both `ListEventsParams` and `HasOlderEventsParams`.
```sql
  AND (sqlc.narg('tag_id')::bigint IS NULL OR EXISTS (
        SELECT 1 FROM artist_tags link
        WHERE link.artist_id = events.artist_id AND link.tag_id = sqlc.narg('tag_id')::bigint))
```
Planner turns it into a semi-join on `artist_tags_tag_id_idx` `[VERIFIED: EXPLAIN this session]`. Tags key on `artists.id` (TAG-07), so removed artists' events still match (the test artists in `events_test.go` have no `watchlist` row, which is exactly the removed-artist shape). Go side: `events.ListParams.TagID *int64`; pass it to **both** `ListEvents` and `HasOlderEvents` in `Service.List`; handler parses with `parseOptionalPositiveInt64(r, "tag_id")` → `writeError(w, 400, "invalid tag_id")`. An unknown-but-valid id returns an empty page and `has_older_events:false` (no 404).

### Pattern 3: One URL-write helper, ref-based, always `preventScrollReset`
```ts
// web/app/lib/useUrlParams.ts  (sketch)
export function useUrlParams() {
  const [params, setParams] = useSearchParams()
  const latest = useRef(params); latest.current = params
  const write = useCallback((mutate: (p: URLSearchParams) => void, opts: { replace?: boolean } = {}) => {
    const next = new URLSearchParams(latest.current)   // always the freshest params, never a stale closure
    mutate(next)
    latest.current = next                                // second call in same tick builds on the first
    setParams(next, { replace: opts.replace ?? false, preventScrollReset: true })
  }, [setParams])
  return { params, write }
}
```
React Router documents that the functional `setSearchParams` form does **not** queue and "Multiple calls … in the same tick will not build on the prior value" `[CITED: reactrouter.com/api/hooks/useSearchParams]`; the ref above is the workaround. Navigation scroll reset is real (`root.tsx:37` mounts `<ScrollRestoration />`, UI-SPEC "Router & sticky-state rules"); `preventScrollReset` is a supported navigate option `[VERIFIED: react-router index.d.ts:141,218-221]`.

### Pattern 4: Pure `selectVisible()` view-model (no API import)
Pipeline: `entries → name (folded includes) → tags (OR) → muted → custom → ∪ sticky → sort`. Comparator rules that are easy to get wrong:
- Latest-release null-last **in both directions** and id **ascending** in both directions → write explicit comparators; never `sort(asc).reverse()` (that flips ties and floats nulls to the front).
- Latest-release compares the **raw text with code-unit `<`/`>`**, not `localeCompare`/`Intl.Collator` (D-15; the collator is for names only). Well-formed `YYYY[-MM[-DD]]` strings behave identically under Postgres `en_US.utf8` (`'2024-05' > '2024'`, `'2023-12-31' < '2024'`, `'2024-06' > '2024-05-17'` all `t` `[VERIFIED: psql]`).
- Date added: `Date.parse(created_at)` numeric compare. Go marshals RFC 3339 with a variable number of fractional digits, so a string compare is wrong (`…:00.5Z` vs `…:00Z`).
- Name: `Intl.Collator(undefined, { sensitivity: "base" })` per UI-SPEC; tie → id asc.
- Custom release types: `!(rt.length === 4 && DEFAULT.every(t => rt.includes(t)))` — compare as a set against `{album,single,ep,deluxe}` `[VERIFIED: internal/watchlist/service.go:43 ReleaseTypes = []string{"album", "single", "ep", "deluxe"}]`.

### Pattern 5: base-ui multi-select Combobox with chips, popup stays open
Use the vendored `Combobox` (= `ComboboxPrimitive.Root`, generic) with `multiple`, `items`, controlled `value: TagRef[]`, `isItemEqualToValue={(a,b)=>a.id===b.id}`, `itemToStringLabel={t=>t.name}`, `Combobox.Chips` as the anchor via `useComboboxAnchor`. The installed docs example renders chips from `<Combobox.Value>{(value) => …}</Combobox.Value>` `[VERIFIED: node_modules/@base-ui/react/docs/react/components/combobox.md:612-700]`. Default behaviour closes the popup on item press when the input is outside the popup; to keep it open (UI-SPEC "Selecting") cancel the close: `onOpenChange={(open, d) => { if (!open && d.reason === "item-press") d.cancel() }}`. `cancel()` exists on change-event details `[VERIFIED: node_modules/@base-ui/react/internals/createBaseUIEventDetails.d.ts:55]`; the reason string `'item-press'` `[VERIFIED: internals/reason-parts.js:12]`; the keep-open recipe itself is `[CITED: base-ui.com/react/components/combobox]` and **not exercised** — the executor must prove it in a jsdom test (A2). Phase 24's `TagCombobox` is a different shape (single value, creatable, `value={null}`) — it does **not** fit; the filter is a new component built from the same vendored `ui/combobox` parts, which is what D-06 means by "don't build a third combobox" (no new combobox *implementation*).

### Pattern 6: Card chip as filter toggle (touches Phase 24 focus code)
Chip label becomes `<button aria-pressed aria-label="Filter by {name}">`, sibling to the unchanged ×. Pending (optimistic) chips stay plain text. `TagActions` gains `filterTagIds: ReadonlySet<number>` and `toggleFilterTag(tag)`; the route's toggle writes `tags` with `push` and clears the sticky set (G4).

### Pattern 7: Sticky cards
Keep `stickyIds: ReadonlySet<number>` in route **state** (not a ref) so the visible list recomputes. In-card edits (`handleEntryChange`, `addTag`, `removeTag`, note save) add the id; `handleAddSearchResult` and Undo add the id returned by POST. Cleared **only** by: search text change, sort change, tag selection change (toolbar or chip click), either toggle, Clear filters, reload. An `entries` change never clears it (`refresh()` runs after add/undo/failed remove).

### Anti-Patterns to Avoid
- **Per-artist follow-up query or Go loop for latest release** — PITFALLS #8 N+1.
- **A second `ListEventsByTag` query** — PITFALLS #9; the predicate must live inside the retention-aware query and in `HasOlderEvents`.
- **Adding a retention cutoff to `ListWatchlist`** — G1 removed it; do not also thread `retentionDays` into `watchlist.NewService`.
- **Computing "upcoming" in JS** — G2: Postgres decides; the client formats only.
- **Importing helpers from `~/lib/api` in view code** — automocked in tests (Pitfall 3).
- **`Date` constructor in `formatReleaseDate`** — UI-SPEC: fixed English month table, no timezone shift.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Multi-select with chips, keyboard nav, ARIA | A third combobox | Vendored base-ui `Combobox` `multiple` + `ComboboxChips` | D-06; a11y behaviour already vetted in Phase 24 |
| Sort dropdown | Native `<select>` | Vendored `Select` (pattern: `DigestSettings.tsx:165-179`) | Phase 11.1 D-13: native popup illegible on Chromium/Windows dark |
| URL state parsing/serialising | Manual `location.search` / `history.replaceState` | `useSearchParams` + `URLSearchParams` | Already routed through RR 7; Back/forward integrate |
| "Is this release upcoming" | JS date math | The LATERAL predicate | G2, single source of truth, no client clock skew |
| Accent folding | A full transliteration library | `normalize("NFD")` + `\p{M}` strip + 4-entry table | Verified in §Code Examples; `ø æ ß` do not decompose under NFD |
| Latest-release lookup | Per-row query | The two LATERALs | Pitfall #8 |
| Stable sort | Index-tracking decorators | `Array.prototype.sort` (stable) + id tie-break comparator | Tie-break is explicit anyway |

**Key insight:** every piece that "feels" custom here already has an in-repo or platform answer; the real risk is not missing libraries but the seams between them (sqlc nullability, mocked module boundaries, router write semantics).

## Runtime State Inventory

Not a rename/refactor/migration phase — omitted. (Additive API fields; no stored-data, service-config, OS-state, secret, or build-artifact renames. The one build artifact that must be refreshed is the embedded SPA bundle, covered in Pitfall 12.)

## Common Pitfalls

### Pitfall 1: sqlc nullability trap on aggregate-in-LATERAL columns
**What goes wrong:** `max(e.release_date) FILTER (…)::text` generates `string` (non-null) and without the cast generates `interface{}`; a NULL scan into `string` errors at runtime for any artist with no events — every dev DB with a freshly added artist breaks `GET /watchlist` with a 500.
**Why:** sqlc infers nullability from column provenance; casts/aggregates over a LEFT-JOINed subquery are treated as not-null. `[VERIFIED: ran sqlc generate on scratch variants: A→interface{}, C(::text)→string, plain-column LATERAL and scalar subquery → *string]`
**How to avoid:** use the two plain-column LATERAL joins (Pattern 1) and assert `*string` in the generated `ListWatchlistRow`. Add a Go test with an artist that has zero events (expects JSON `null`, HTTP 200).
**Warning signs:** `sqlc generate` output with `LatestReleaseDate string` or `interface{}`; "cannot scan NULL into *string".

### Pitfall 2: `ListWatchlistRow`/`GetWatchlistEntryRow` drift
**What goes wrong:** editing only one of the two queries → `Service.get`'s `sqlc.ListWatchlistRow(row)` conversion stops compiling (good) — but if only the comment/aliases differ the build can pass while semantics diverge.
**How to avoid:** copy the identical select list and joins to both; extend `TestService_ProjectionParity` to include seeded events so `Add`/`UpdatePreferences`/`UpdateNote` results equal the `List` element **including the two new fields**.

### Pitfall 3: `vi.mock("~/lib/api")` automocks everything in `api.ts`
**What goes wrong:** every route/component test file does a bare `vi.mock("~/lib/api")` (e.g. `watchlist.test.tsx:34`, `history.test.tsx:15`), so any pure helper placed in `api.ts` returns `undefined` in tests. Likewise `listTags()` in History now returns `undefined` unless every History test sets `mockListTags.mockResolvedValue([])` — the existing comment in `history.test.tsx:20-22` documents the same hazard for `listWatchlist`.
**How to avoid:** helpers go in `watchlistView.ts`/`format.ts`/`useUrlParams.ts`; api.ts gets types + wrappers only. Wave 0: add `mockListTags` setup to `history.test.tsx`, `HistoryFilters.test.tsx`.

### Pitfall 4: `WatchlistEntry` gains two required fields → 8 test fixtures fail typecheck
**Files that build a `WatchlistEntry` literal** `[VERIFIED: grep muted_event_types]`: `HistoryFilters.test.tsx`, `ArtistNote.test.tsx`, `PreferenceToggles.test.tsx`, `SearchResultsColumns.test.tsx`, `TagChips.test.tsx`, `TagCombobox.test.tsx`, `lib/api.test.ts`, `routes/watchlist.test.tsx`. **How to avoid:** add `web/app/lib/test/fixtures.ts` (`makeEntry(overrides)`, already excluded from coverage by `app/lib/test/**`) and migrate the fixtures in the same plan that changes `api.ts`; `corepack pnpm --dir web run typecheck` is the gate.

### Pitfall 5: `TagChips` focus logic indexes `querySelectorAll("button")`
**What goes wrong:** `TagChips.tsx:94-96` focuses `buttons[index]` where `index` is the chip index, assuming one button (the ×) per chip. Adding the label toggle makes the order `[label0, ×0, label1, ×1, …]`; after removing a chip, focus lands on the wrong control. Also the "grow" request (`index = entry.tags.length - 1`) for the 10th-pick path.
**How to avoid:** select only remove buttons (e.g. add `data-chip-remove` and query `[data-chip-remove]`, or match `aria-label^="Remove tag"`), keep pending chips' × (aria-disabled) ordering after real chips. Re-run `TagChips.test.tsx` (489 lines of focus assertions) before anything else.

### Pitfall 6: Debounced `q` write clobbers concurrent URL changes (stale closure)
**What goes wrong:** the 300 ms trailing `replace` for `q` captured an older `searchParams`; if the user changes sort inside that window the debounced write restores the old `sort`. Second form: after the debounce writes `q="ba"` while the user already typed `"bad"`, a naive `useEffect(() => setLocal(urlQ), [urlQ])` resets the input to `"ba"` and eats keystrokes.
**How to avoid:** the shared writer reads from `latest.current` (Pattern 3); keep a `lastWrittenQ` ref and resync the local input from the URL only when `urlQ !== lastWrittenQ.current` (i.e. a Back/forward or external change). Filtering uses the **local** input value (instant), never the URL value.

### Pitfall 7: Scroll jumps to top on every param write
**Why:** `<ScrollRestoration />` + router navigation (UI-SPEC "Scroll reset"). **How to avoid:** all writes via the one helper that always passes `{ preventScrollReset: true }`; add a test that spies `setSearchParams` options, plus the UAT item (scroll far down, click a chip).

### Pitfall 8: base-ui multi-select closes the popup on item press
See Pattern 5. UI-SPEC requires stay-open + chips input cleared + focus stays. Default clears the typed filter (docs) but closes the popup. Cancel on `item-press`; verify focus stays in the chips input in a test (A2).

### Pitfall 9: Selected tag that no longer exists in the payload-derived option set
**What goes wrong:** per UI-SPEC "Tags that lose their last carrier", the tag stays selected after an in-session detach, but the option list is the live union of `entries[].tags`, so base-ui no longer has that item and the chip has no name source.
**How to avoid:** keep a `tagNameCache: Map<id,name>` fed by every derivation of the union and by `onRenamed`/`onMerged`; build the `value: TagRef[]` for the combobox from URL ids + cache, not from `items`. Order chips by the collator (`sensitivity:"base"`, id tie-break). See Open Question 1 for the fallback label when the cache has no entry.

### Pitfall 10: History `Combobox<T>` shows the wrong trigger label for unknown values
`HistoryFilters.tsx:70-73`: `selectedOption = options[selectedIndex === -1 ? 0 : selectedIndex]` — an unknown value silently displays "All …" while the filter is applied. That is the exact "Loading tags…"/"Tag unavailable" case in the UI-SPEC and also affects a bookmarked `?artist=` for a since-removed artist (artist options come from the **current** watchlist). **How to avoid:** add an optional `triggerLabel` override prop to `Combobox<T>`; decide the stale-artist behaviour (Open Question 3). Also: `scrollIntoView` does not exist in jsdom `[VERIFIED: node + jsdom 30 → typeof Element.prototype.scrollIntoView === "undefined"]`, so the new ArrowDown scroll code needs optional-call (`el?.scrollIntoView?.({ block: "nearest" })`) or a test stub; `vitest.config.ts` has `mockReset: true`, so a `vi.fn()` stub must be assigned in `beforeEach`.

### Pitfall 11: `make sqlc-check` false failure on unstaged generated code
`sqlc-check` runs `sqlc generate` then `git diff --exit-code -- internal/db/sqlc/` (Makefile). `git diff` compares the working tree to the index, so freshly regenerated, **unstaged** files fail. Flow: `sqlc generate` → `git add internal/db/sqlc` → `make sqlc-check`. Also: `make` and `~/go/bin` are not on this machine's bash PATH (`which make` / `which sqlc` empty; `sqlc.exe` is at `~/go/bin`) — invoke accordingly or run from a shell that has them.

### Pitfall 12: The embedded bundle must be rebuilt and committed
`make web` = `pnpm install --frozen-lockfile` → `pnpm run build` → replace `internal/webassets/build/client` (Makefile `web` target). Phase 24 ended with a rebuild commit (`24-REVIEW.md`: "rebuild commit `3d07813`"). CI has no drift check (grep of `full-pipeline.yml` for webassets/bundle: none), so omission ships a stale UI silently. Make it the last task of the last plan, after prettier.

### Pitfall 13: Time-dependent Go tests
The upcoming predicate uses the DB clock. Seed far-fixed dates (`1999`, `2999`, `1999-05`, `2999-05`, `1999-05-17`, `2999-05-17`) for the bulk of cases, and use `time.Now().UTC().Year()` only for the single same-precision boundary assertion (it can only flake across a UTC year rollover). Never seed "today ± 1 day" (races UTC midnight).

### Pitfall 14: `useSearchParams` in Manage-tags async callbacks
`onDeleted/onMerged` fire after awaited network calls; reading `searchParams` from the render closure may be stale. Route them through the same ref-based writer with `replace`.

## Code Examples

### Name fold (G5, D-04) — executed in Node this session
```ts
// web/app/lib/watchlistView.ts
const FOLD: Record<string, string> = { $: "s", ø: "o", æ: "ae", ß: "ss" }
export function foldName(s: string): string {
  return s
    .normalize("NFD")
    .replace(/\p{M}/gu, "")          // strip combining marks (é → e)
    .toLowerCase()                    // BEFORE the table so Ø/Æ fold too
    .replace(/[$øæß]/g, (c) => FOLD[c])
}
// verified: foldName("A$AP Rocky").includes(foldName("asap")), Beyoncé/beyonce, Mø/mo, Ænima/aenima, Straße/strasse
```
`ø æ ß` have no NFD decomposition (`'Ø'.normalize('NFD').length === 1`), which is why the table exists. Trim the query first; a whitespace-only query matches everything.

### formatReleaseDate (UI-SPEC table; no `Date`)
```ts
const MONTHS = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"]
export function formatReleaseDate(raw: string): string {
  let m = /^(\d{4})$/.exec(raw); if (m) return m[1]
  m = /^(\d{4})-(0[1-9]|1[0-2])$/.exec(raw); if (m) return `${MONTHS[+m[2] - 1]} ${m[1]}`
  m = /^(\d{4})-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$/.exec(raw); if (m) return `${+m[3]} ${MONTHS[+m[2] - 1]} ${m[1]}`
  return raw
}
```

### Go: Entry and handler touch points
```go
// internal/watchlist/service.go — Entry
LatestReleaseDate *string `json:"latest_release_date"`
NextReleaseDate   *string `json:"next_release_date"`
// entryFromRow: LatestReleaseDate: row.LatestReleaseDate, NextReleaseDate: row.NextReleaseDate

// internal/httpserver/events.go — beside artist_id
tagID, ok := parseOptionalPositiveInt64(r, "tag_id")
if !ok { writeError(w, http.StatusBadRequest, "invalid tag_id"); return }
// ListParams{..., TagID: tagID}; events.Service.List passes TagID to BOTH ListEvents and HasOlderEvents
```

### Regression test shape (mirror `events_test.go:745-800` and `:1287-1330`)
```go
func TestRetention_TagFilterComposesWithCutoff(t *testing.T) {
	pool := testutil.NewTestPool(t)
	// artist w/ NO watchlist row (removed-artist shape), tag + artist_tags link,
	// one event aged 120d (insertTestEventAt), one 1d. GET /events?tag_id=T:
	//   events == [recent only]; has_older_events == true.
	// second tag on a different artist → absent. tag_id=abc / 0 / -1 → 400.
}
```

### Vitest invocation forms (Phase 24 proven)
```bash
TEST_DATABASE_URL='postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable' \
  go test ./internal/httpserver/ -run 'TestRetention_TagFilter|TestListEvents_TagFilter|TestWatchlist_List_LatestRelease' -count=1 -v
corepack pnpm --dir web exec vitest run app/lib/watchlistView.test.ts --coverage.enabled=false
corepack pnpm --dir web run typecheck
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 06 D-03/D-04: Watchlist shows no release activity; list never re-sorts client-side | Latest/Upcoming lines on every card; client-side sort/filter | This phase (D-02/D-03) | Rewrite the module comment in `watchlist.tsx:26-32` in 1–3 lines (CLAUDE.md comment discipline) |
| PITFALLS #8/#9 recommendation: latest release respects retention | G1: ignores retention; G2/G3 split latest vs upcoming | Grill 2026-10-04 | No cutoff arg on `ListWatchlist`; the roadmap's "respect retention" note is superseded |
| Phase 24 D-04: chip label does nothing | D-08: label toggles the filter | This phase | `TagChips` + focus-index change (Pitfall 5) |

**Deprecated/outdated:** the roadmap's "check EXPLAIN, any index is a pure-additive migration" — EXPLAIN shows none is needed.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | base-ui's default `filter` for `Combobox` gives a case-insensitive *contains* match on `itemToStringLabel` (UI-SPEC copy depends on it) | Pattern 5 | Typing in the tag filter narrows differently than the spec; fix with an explicit `filter` prop |
| A2 | Cancelling `onOpenChange` on `reason === "item-press"` keeps the multi-select popup open and keeps focus in the chips input (cited from base-ui.com docs, not run) | Pattern 5 / Pitfall 8 | Popup closes after each pick; fall back to a controlled `open` state that ignores `item-press` closes (the Phase 24 `TagCombobox` already controls `open`) |
| A3 | Stale `?artist=` (id absent from the current watchlist) should be dropped silently once `listWatchlist()` resolves, like tags | Pitfall 10 / OQ3 | Needs a user call — alternative is a "Removed artist" label; UI-SPEC is silent |
| A4 | Per-artist LATERAL cost scales to ~0.3 ms × 100 artists ≈ 30 ms (extrapolated from the 9-artist dev measurement; real DB could be larger) | Pattern 1 | If a large production events table is slow, an additive partial index `(artist_id, release_date) WHERE event_type='new_release'` is the fallback (read `internal/db/migrations/README.md` first) |
| A5 | Malformed `release_date` strings (lengths other than 4/7/10) are rare and need no SQL guard | Pattern 1 | Odd value could sort as "upcoming"; client falls back to raw display. Optional regex guard if observed |
| A6 | `Intl.Collator` `sensitivity: "base"` (UI-SPEC) vs Go digest `collate.IgnoreCase` (accents significant): differences only for accent-only name ties, which then tie-break on id | Pattern 4 | Cosmetic ordering difference vs the Discord digest |
| A7 | `refresh()` / in-flight chip pending state surviving card unmount when a filter change hides a card mid-attach is acceptable (route-level `addTag` still applies; only the pending badge is lost) | Pattern 7 | Minor UI flicker; sticky cards cover the in-card edit path |

## Open Questions

1. **Selected tag with no name source (all carriers detached, then page state only)**
   - Known: the tag stays selected (UI-SPEC); the option list is the live payload union; names come from a cache.
   - Unclear: what to display if the cache has no entry (e.g. a URL tag id that survived first-load validation only because it was in the payload, then lost the last carrier before the cache populated — practically impossible if the cache is filled on first load).
   - Recommendation: fill the cache from the first payload; if a name is still missing, render `Tag #{id}` and keep it removable. Planner decides whether this needs UI-SPEC copy.
2. **Sort `Select` and `aria-labelledby` + base-ui value typing** — follow `DigestSettings.tsx:165-179` exactly (`SelectValue` render-function child); not a blocker.
3. **History `?artist=` for a since-removed artist** — see A3. Recommendation: validate against the loaded watchlist and drop silently with `replace`; confirm with the user in plan-check if they prefer a labelled fallback.
4. **`next_release_date` for a seeded back-catalogue** — Deezer/MusicBrainz rows exist for the same album from both sources; the LATERALs take the max/min value so duplicates are harmless. No action.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker + Postgres 16 container (`drop-tracker-postgres-1`, healthy, port 5432) | All Go DB tests, `make test` | ✓ | postgres:16, schema version 12 | `make db-up` (docker compose) |
| Go toolchain | Backend | ✓ | go1.26.0 (go.mod `go 1.26`) | — |
| sqlc | `make sqlc` / `sqlc-check` | ✓ (not on bash PATH) | v1.31.1 at `~/go/bin/sqlc.exe` | Call by full path |
| `make` | Makefile targets | ✗ on this bash PATH | — | Run the underlying commands (`sqlc generate`, `go test …`) or use a shell with make |
| golangci-lint | DoD gate 2 | ✓ (not on PATH) | `~/go/bin/golangci-lint.exe` | Full path |
| Node | Frontend | ✓ | v24.16.0 | — |
| corepack / pnpm | Frontend | ✓ | pnpm 11.23.0 via `corepack pnpm` (bare `pnpm` not on PATH) | Use the `corepack pnpm --dir web …` form (CLAUDE.md) |
| psql | EXPLAIN | via `docker exec drop-tracker-postgres-1 psql -U drop_tracker -d drop_tracker` | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** `make` (use underlying commands).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` against real Postgres (`testutil.NewTestPool` / `NewIsolatedTestPool`); Vitest 4.1.10 + Testing Library + jsdom 30 (TZ pinned `UTC`, `mockReset: true`) |
| Config file | `web/vitest.config.ts` (coverage thresholds 70/70/70/70, `app/components/ui/**` and `app/lib/test/**` excluded); Go: Makefile (`COVERAGE_THRESHOLD_BACKEND ?= 80`) |
| Quick run (Go) | `TEST_DATABASE_URL='postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable' go test ./internal/watchlist/ ./internal/events/ ./internal/httpserver/ -run '<names>' -count=1 -v` |
| Quick run (web) | `corepack pnpm --dir web exec vitest run <files> --coverage.enabled=false` |
| Typecheck | `corepack pnpm --dir web run typecheck` (`react-router typegen && tsc`) |
| Full suite | `make db-up && make test && make coverage-gate && make sqlc-check`; `corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}"`; `corepack pnpm --dir web test`; `go vet ./...`; `golangci-lint run` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WLVW-01 | accent/case/`$ø æ ß` fold, trim, name-only (not disambiguation) | unit | `vitest run app/lib/watchlistView.test.ts` | ❌ Wave 0 |
| WLVW-01 | typing narrows list live; "N of M" updates; zero extra requests | component (route) | `vitest run app/routes/watchlist.test.tsx` | ✅ extend |
| WLVW-02 | name A–Z/Z–A collator; date-added via `Date.parse`; id-asc tie-break stable across direction flip | unit | `vitest run app/lib/watchlistView.test.ts` | ❌ Wave 0 |
| WLVW-03 | latest/next date semantics: any age, non-upcoming vs upcoming at own precision, mixed precision max/min, zero-event artist → null | Go integration | `go test ./internal/watchlist/ -run 'TestService_List_LatestRelease\|TestService_ProjectionParity' -count=1 -v` | ❌ Wave 0 (extend `service_test.go`) |
| WLVW-03 | JSON exposes both fields (`null` allowed), GET single query | Go HTTP e2e | `go test ./internal/httpserver/ -run 'TestWatchlist_List_LatestRelease' -count=1 -v` | ❌ Wave 0 |
| WLVW-03 | null-last in both latest directions; `next_release_date` never affects sort | unit | `vitest run app/lib/watchlistView.test.ts` | ❌ Wave 0 |
| WLVW-03 | `formatReleaseDate` over `2024`, `2024-05`, `2024-05-17`, malformed | unit | `vitest run app/lib/format.test.ts` | ✅ extend |
| WLVW-04 | any-of OR; chip click toggles; URL `tags=` ascending; Manage-tags delete/merge update filter | unit + component | `vitest run app/lib/watchlistView.test.ts app/components/watchlist/TagChips.test.tsx app/components/watchlist/WatchlistToolbar.test.tsx app/routes/watchlist.test.tsx` | ❌ Wave 0 (toolbar), ✅ extend others |
| WLVW-05 | muted / custom-release-types toggles, set comparison, AND composition | unit + component | same as above | ❌ Wave 0 |
| WLVW-06 | composition, empty state, Clear filters (preserves sort, focus to filter), sticky cards (G4), add-under-filter sticky, announcements, `refresh()` leaves tag filter | component (route) | `vitest run app/routes/watchlist.test.tsx` | ✅ extend |
| WLVW-06 | URL write helper: always `preventScrollReset`, replace vs push, ref-based (no stale clobber) | unit/component | `vitest run app/lib/useUrlParams.test.tsx` | ❌ Wave 0 |
| HIST-02 | tag filter composes with artist/type/cursor; retention on first page AND `has_older_events`; removed artist included; unknown id → empty; bad id → 400 | Go integration + HTTP | `go test ./internal/httpserver/ -run 'TestRetention_TagFilter\|TestListEvents_TagFilter\|TestHandleListEvents_Validation' -count=1 -v` | ❌ Wave 0 (extend `events_test.go`) |
| HIST-02 | History Tag control: options from `listTags`, loading/failed labels, request carries `tagId`, tag empty state + "Show all tags", precedence | component | `vitest run app/components/history/HistoryFilters.test.tsx app/routes/history.test.tsx app/lib/api.test.ts` | ✅ extend |
| HIST-02 (G6) | History URL state: params drive filters, invalid dropped with replace | component | `vitest run app/routes/history.test.tsx` | ✅ extend |
| SC5 | One `listWatchlist` call on open; no `listTags`/`listEvents` on Watchlist; zero calls while typing/sorting/filtering | component | `vitest run app/routes/watchlist.test.tsx` (assert `toHaveBeenCalledTimes(1)`) | ✅ extend |
| sqlc drift | generated code matches queries | static | `git add internal/db/sqlc && make sqlc-check` | — |

### Sampling Rate
- **Per task commit:** the narrowest Go `-run` / `vitest run <files>` for the files touched, plus `typecheck` for any `web/` change.
- **Per wave merge:** `go test ./internal/watchlist/ ./internal/events/ ./internal/httpserver/ -count=1`, `corepack pnpm --dir web test`, `make sqlc-check`.
- **Phase gate:** full DoD list (vet, golangci-lint, `make test`, `make coverage-gate`, `make sqlc-check`, prettier check, `pnpm test`) green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `web/app/lib/test/fixtures.ts` — `makeEntry()`; migrate the 8 fixture files (Pitfall 4)
- [ ] `web/app/lib/watchlistView.test.ts` — fold/filter/sort/parse/select-visible
- [ ] `web/app/lib/useUrlParams.test.tsx` — write semantics (extend `routeStub.tsx` with an optional `initialEntry` so a test can start at `/?q=bad`; today `renderRoute(Component, path)` uses `path` for both route path and initial entry)
- [ ] `web/app/components/watchlist/WatchlistToolbar.test.tsx` (+ tag-filter keep-open / focus tests, A1/A2; stub `scrollIntoView`)
- [ ] `mockListTags.mockResolvedValue([])` in `history.test.tsx` / `HistoryFilters.test.tsx` setup (Pitfall 3)
- [ ] Go: seed helpers for `new_release` events with explicit `release_date` + `artist_tags` link (existing: `insertTestEventWithDate`, `insertTestEventAt`, `insertTestArtist` in `events_test.go`; need a combined date+created_at helper)
- Framework install: none — existing infrastructure covers all requirements.

## Security Domain

`security_enforcement` is enabled (ASVS L1, block on high).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (no new routes; `GET /watchlist`, `GET /events` already behind the authgate) | existing gate |
| V3 Session Management | no | existing |
| V4 Access Control | no new resources; `tag_id` exposes only the caller's own data | existing gate |
| V5 Input Validation | **yes** | `parseOptionalPositiveInt64` (rejects non-int, ≤0) → 400 fixed message; bound sqlc params only (never interpolated); client: sort allow-list, `tags` ids matched against `/^[1-9]\d*$/` and the payload set, malformed dropped with `replace` |
| V6 Cryptography | no | — |
| V13 API | yes | Fixed operator-authored error strings (T-06-01 posture), no raw DB text |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via `tag_id` | Tampering | `sqlc.narg('tag_id')::bigint` bound param; handler int parse |
| Reflected XSS via URL params (`q`, `tags`, `tag`) | Tampering / Info disclosure | Render `q` only as an input value and in `No tags match “{q}”` as plain JSX text; tag/artist names plain JSX text (Phase 06 posture); never `dangerouslySetInnerHTML` |
| Oversized `tags=` list / unbounded param DoS (client) | DoS | Intersect parsed ids with the payload-derived set before use; cap parse length |
| Info disclosure through `latest_release_date` | Info disclosure | Same data History already serves behind the gate; G1 accepts the retention inconsistency |
| N+1 / expensive query on the hot read path | DoS | Single query, measured ~0.3 ms/artist |

## Project Constraints (from CLAUDE.md)

- **Definition of Done before every commit:** (web changed) `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` first; then `go vet ./...`, `golangci-lint run`, `make test` (after `make db-up`), `make coverage-gate` (80% backend), `make sqlc-check` (local-only; no CI counterpart), and for web `corepack pnpm exec prettier --check "**/*.{ts,tsx}"` + `corepack pnpm test`.
- **Never `git commit --no-verify`;** hooks (gitleaks, golangci-lint `--fix`, prettier `--write`) are the local mirror of CI.
- **No AI attribution** in commits or PRs (no `Co-Authored-By: Claude`, `Claude-Session:`, "Generated with…").
- **Comment discipline:** 1–3 line *why* comments, one design-doc reference, no multi-paragraph blocks (anti-pattern: `authStore.ts`). The existing SQL comments in `queries/*.sql` are long, but new comments added by this phase must stay short; rewrite the `watchlist.tsx` module comment tied to Phase 06 D-03/D-04 (UI-SPEC "Data Contract").
- **Migrations:** read `internal/db/migrations/README.md` first if one is added — not expected this phase (EXPLAIN shows none needed; if A4 forces a partial index it is pure-additive and must be non-`CONCURRENTLY`).
- **Stack locks:** chi, sqlc + golang-migrate, pgx/v5, React + Vite embedded via `go:embed`, secrets via env only, no live external calls in tests.
- **GSD workflow enforcement:** edits go through a GSD command (`/gsd-execute-phase` for this work).
- **Memory:** grill overrides (G1–G7) win over discuss/research/base design; drift warnings don't force re-research.
- **graphify:** a `graphify-out/` graph exists; not needed here because the code seams were read directly.

## Suggested Plan Seams (for the planner; planner owns final slicing)

1. **Backend enrichment (Go):** `queries/watchlist.sql` (both queries) → `sqlc generate` → `Entry` fields + `entryFromRow` → service/HTTP tests (incl. zero-event artist, upcoming matrix, `ProjectionParity`). Independent of everything else.
2. **Backend History tag filter (Go):** `queries/events.sql` (both queries) → regenerate → `ListParams.TagID`, handler parse, `TestRetention_TagFilter*`, validation table cases. Independent of plan 1 (both touch generated files — regenerate once if run in the same wave, to avoid conflicts).
3. **Frontend foundation:** `api.ts` types + `listEvents({tagId})`, `format.ts` `formatReleaseDate`, `watchlistView.ts`, `useUrlParams.ts`, `fixtures.ts`, fixture migration. Pure; unblocks 4–6.
4. **Watchlist UI:** toolbar, tag filter, toggles, count/empty states, sticky cards, URL state, row lines, `TagChips` toggle + focus fix, vendored `combobox.tsx` edit, announcements.
5. **History Tag filter (HIST-02 UI) — still `useState`:** `Combobox<T>` hardening + `triggerLabel`, Tag control, lifted `tags` state, empty states. Lands and verifies on its own (G6).
6. **History URL state (G6):** separate plan, after 5 (both edit `history.tsx`/`HistoryFilters.tsx`).
7. **Close-out:** prettier, rebuild embedded bundle, full DoD gate.

## Sources

### Primary (HIGH confidence)
- Repo files read this session: `queries/watchlist.sql`, `queries/events.sql`, `queries/tags.sql`, `internal/watchlist/service.go`, `internal/events/service.go`, `internal/httpserver/events.go`, `internal/httpserver/events_test.go` (retention test patterns, helpers), `internal/db/migrations/000003/000010 + README.md`, `web/app/routes/{watchlist,history}.tsx`, `web/app/components/{history/HistoryFilters,watchlist/TagChips,watchlist/WatchlistRow,watchlist/PreferenceToggles,watchlist/TagCombobox,ui/combobox,common/EmptyState}.tsx`, `web/app/lib/{api,format,tags}.ts`, `web/app/lib/test/routeStub.tsx`, `web/vitest.config.ts`, `Makefile`, `.github/workflows/full-pipeline.yml`.
- Executed this session: `EXPLAIN (ANALYZE, BUFFERS)` of both LATERAL forms and the tag `EXISTS` on the dev Postgres 16; `sqlc generate` v1.31.1 on a scratch copy for 5 query variants; Postgres truth table for the upcoming predicate; Node run of the fold table; jsdom `scrollIntoView` probe.
- `web/node_modules/@base-ui/react/docs/react/components/combobox.md`, `internals/createBaseUIEventDetails.d.ts`, `internals/reason-parts.js`; `web/node_modules/react-router/dist/development/index-react-server-client-3ykjivgQ.d.ts` and `index.d.ts`.

### Secondary (MEDIUM confidence)
- reactrouter.com/api/hooks/useSearchParams (functional `setSearchParams` does not queue) — fetched, quote used verbatim.
- base-ui.com/react/components/combobox (multi-select input clearing / item-press close behaviour) — fetched summary; recipe not exercised.

### Tertiary (LOW confidence)
- none

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; installed versions read from `node_modules`.
- Architecture (backend): HIGH — SQL executed, sqlc output inspected, EXPLAIN captured.
- Architecture (frontend): MEDIUM-HIGH — repo code read in full; two base-ui behaviours cited but not run (A1, A2).
- Pitfalls: HIGH for 1, 2, 3, 4, 5, 10, 11, 12 (each grounded in a file/line or an executed probe); MEDIUM for 6, 8, 9 (reasoned from code + docs).

**Research date:** 2026-10-05
**Valid until:** 2026-11-04 (stable stack; re-check if `@base-ui/react` or `react-router` are bumped)
