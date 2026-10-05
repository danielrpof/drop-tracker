# Phase 25: Find & Filter — Watchlist and History - Context

**Gathered:** 2026-10-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Client-side name search, stable sorts (name, date added, latest release), and tag and preference filters over the fully loaded Watchlist. `GET /watchlist` stays a single request and gains `latest_release_date`. History gets a single-tag filter that composes with its existing artist/event-type filters and the retention cutoff. Requirements: WLVW-01..06, HIST-02.

Out of scope: server-side paging/sorting (REQUIREMENTS.md Out of Scope), multi-select and bulk actions (Phase 26), and tags on Discord (Phase 28).

</domain>

<decisions>
## Implementation Decisions

### Watchlist toolbar & layout
- **D-01:** The add-artist `SearchBox` (MusicBrainz/Deezer) stays at the top of the page, unchanged. A separate list toolbar sits directly above the artist list. It holds the name filter (labelled distinctly, e.g. "Filter watchlist…"), the sort control, the tag filter, the two preference toggles, and the "N of M artists" count. The two inputs must never be confusable by label or placeholder.
- **D-02:** The default sort is **name A–Z**, which matches today's server order (`ListWatchlist` orders by name, then id), so the default view doesn't change.
- **D-03:** Every Watchlist card always shows a muted latest-release line, either at stored precision (D-14) or as "No releases yet" when there is none. This deliberately reverses the Phase 06 D-03/D-04 "no release activity on the Watchlist" posture. Update the module comment in `web/app/routes/watchlist.tsx` that cites it.
- **D-04:** The name search is a case-insensitive, **accent-insensitive** substring match on the artist name only, not the disambiguation (NFD normalize, strip combining marks, lowercase, then `includes`). For example, `beyonce` matches Beyoncé. It filters as the user types.

### Tag & preference filters
- **D-05:** The multi-tag Watchlist filter is **any-of (OR)**. Label it to match, e.g. "Tags: any of".
- **D-06:** The Watchlist tag filter uses the **base-ui Combobox** Phase 24 vendored (`web/app/components/ui/combobox.tsx`) in `multiple` mode with chips. Reuse or extend `TagCombobox` where it fits, and don't build a third combobox.
- **D-07:** The Watchlist tag filter's options and names come **from the loaded watchlist payload** (the union of `entries[].tags`), not `GET /tags`. That keeps opening the Watchlist to one request (SC5, Phase 24 D-30) and drops tags only removed artists carry, which would match nothing on the Watchlist anyway.
- **D-08:** Clicking a tag chip on a Watchlist card **toggles that tag in the filter**. The chip body becomes a button with an accessible name like "Filter by reggaeton". The existing × still detaches the tag (Phase 24 D-04 deferred this behavior here). Keep the two hit targets visually and accessibly distinct.
- **D-09:** Preference filters are **two independent toggles**: "Has muted events" (`muted_event_types` non-empty) and "Custom release types" (`release_types` ≠ the default full set `album, single, ep, deluxe`, compared as a set). Like every other filter, they combine with AND.
- **D-10:** Search, sort, the tag filter, and the preference toggles all compose. When the combination matches nothing, the view shows a distinct empty state with a single "Clear filters" action that resets search and every filter. Sort is not a filter, so it is preserved. Keep three empty states distinct: no artists on the watchlist, filters matched nothing, and (on History) no events for this tag (FEATURES.md).
- **D-11:** History's tag filter is **single-select** and uses a scalar `tag_id` narg, as the roadmap specifies. It reuses the hand-rolled `Combobox<T>` in `web/app/components/history/HistoryFilters.tsx` so it matches its Artist and Event-type siblings. Its options come from `GET /tags`, which also lists tags only removed artists carry, because History shows events for removed artists.

### State persistence
- **D-12:** Watchlist search, sort, tag filter, and preference toggles live in **URL query params** via React Router 7 `useSearchParams`. Typing in the search box updates with `replace`, never push, so each keystroke doesn't add a history entry. History's artist, event-type, and new tag filters move to URL params the same way. Param names and the sort value encoding are left to the planner, but they must be stable and human-readable (e.g. `?q=bad&sort=latest-desc&tags=3,7&muted=1&custom=1`, and `/history?tag=3`).
- **D-13:** Stale or invalid params are **dropped silently** and the URL is rewritten with `replace`. This covers unknown tag ids (not present in the payload-derived set), an unknown sort value, and malformed numbers. When Manage tags deletes or merges a tag that is in the active filter, the filter updates: a deleted tag drops out, and a merged tag swaps to the merge target. The Watchlist and History filters stay **independent**, with no cross-route link (Phase 06 D-03/D-04 decoupling).

### Latest-release semantics
- **D-14:** `latest_release_date` is the max `release_date` over that artist's `event_type = 'new_release'` events, **within the retention window**. It uses the same `created_at >= cutoff` predicate ListEvents uses, so the Watchlist never shows a date History can't show (PITFALLS #8). An artist whose releases have all aged out falls into the "no release" group. Retention keys on detection time (`created_at`), so a newly added artist's seed-mode back catalogue counts until it ages out, exactly as it does in History. — **Reversibility:** reversible — it's a query predicate, and there's no schema change.
- **D-15:** Sort on the raw text lexicographically, which is the property ListEvents already relies on: `2024` sorts as the start of 2024, with ties broken by artist id. Display at stored precision ("2024", "May 2024", "17 May 2024"). Future and announced dates count and display as-is. NULL or none sorts last in both directions (WLVW-03).
- **D-16:** Every sort mode tie-breaks on the artist id. Name sort uses `Intl.Collator` (case-insensitive) and mirrors the digest's collated sort. Date added sorts on the watchlist row's created timestamp.

### Claude's Discretion
- The sort control's shape (a single select of six options vs a field plus a direction toggle), toolbar wrapping at phone width, and copy for labels, empty states, and the count. The UI phase pins these, and the `ui-ux-pro-max` skill applies to visual choices.
- Whether to debounce the search input. Client-side filtering of about 100 rows doesn't need it, but URL writes might.
- The exact `LEFT JOIN LATERAL` form, and whether `EXPLAIN` justifies a pure-additive index migration (roadmap note).
- How the cutoff reaches the `ListWatchlist` query (a `sqlc.arg('cutoff')`, consistent with ListEvents' non-optional cutoff).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope & requirements
- `.planning/ROADMAP.md` §"Phase 25" — goal, success criteria, and "Notes for the phase planner" (LATERAL join, the `tag_id` narg in **both** `ListEvents` and `HasOlderEvents`, the `TestRetention_*`-style regression test, three empty states)
- `.planning/REQUIREMENTS.md` — WLVW-01..06, HIST-02, and Out of Scope (no server-side paging/sorting)
- `.planning/phases/24-artist-tags-notes/24-CONTEXT.md` — D-04 (chip click deferred here), D-11/D-13 (vocabulary semantics), D-26 (enrichment projection shared by GET/POST/PATCH), and D-30 (lazy vocabulary, one request on open)

### Research
- `.planning/research/PITFALLS.md` §Pitfall 8 — the latest-release N+1 trap and why retention should apply
- `.planning/research/PITFALLS.md` §Pitfall 9 — retention and the tag filter must compose inside one query
- `.planning/research/PITFALLS.md` §Pitfall 5 — artist-scoped tags are what make History-by-tag work for removed artists
- `.planning/research/FEATURES.md` — the three-way empty-state split and the stable secondary sort keys

### Code contracts
- `queries/events.sql` — `ListEvents` (lexicographic `release_date DESC NULLS LAST, id DESC`, the non-optional `cutoff` on `created_at`) and `HasOlderEvents`, which mirrors its filters
- `internal/httpserver/events.go` — `parseOptionalPositiveInt64` for parsing `tag_id`
- `internal/httpserver/events_test.go` — `TestRetention_*` style for the new tag+retention regression test
- `internal/watchlist/service.go:37` — default `release_types` set

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/app/components/ui/combobox.tsx` and `web/app/components/watchlist/TagCombobox.tsx`: base-ui multi-select combobox with chips, used for the Watchlist tag filter (D-06).
- `web/app/components/history/HistoryFilters.tsx` `Combobox<T>`: the hand-rolled single-select with `aria-activedescendant`, used for History's tag control (D-11).
- `web/app/components/watchlist/TagChips.tsx`: gains the click-to-filter affordance (D-08).
- `EmptyState` (already used in `watchlist.tsx`): for the filtered-to-zero state, with a Clear filters action.
- `ManageTagsDialog`'s `onDeleted` / `onRenamed` / `onMerged` callbacks: hook points for keeping the active filter consistent (D-13).

### Established Patterns
- Both routes keep filter state in `useState` today, so D-12 moves it to `useSearchParams`. History's `HistoryFiltersValue` object makes the swap local.
- `history.tsx` refetches page one on filter change, and the effect deps list `filters.artistId`/`filters.eventType`. The tag must join that list and the `listEvents` params.
- `GET /watchlist` enrichment is a single query with correlated subqueries (Phase 24). Add `latest_release_date` there, and through the shared projection on POST/PATCH (Phase 24 D-26) so `WatchlistEntry` stays accurate everywhere.
- Every new param or route goes through `registerDataRoutes` and `apiFetch`, and tag names render as plain JSX text.

### Integration Points
- `WatchlistEntry` in `web/app/lib/api.ts` gains `latest_release_date: string | null`.
- `listEvents()` in `web/app/lib/api.ts` gains `tagId`.
- `ListEvents` and `HasOlderEvents` in `queries/events.sql` gain `sqlc.narg('tag_id')` `EXISTS (SELECT 1 FROM artist_tags ...)`. Run `make sqlc-check` afterwards.

</code_context>

<specifics>
## Specific Ideas

- The URL shape should look like `?q=bad&sort=latest-desc&tags=3,7&muted=1&custom=1` and `/history?tag=3`, so it's readable and bookmarkable.
- Chip accessible name: "Filter by {tag}".

</specifics>

<deferred>
## Deferred Ideas

- A cross-route "See releases for #tag" link from a single-tag Watchlist filter to `/history?tag=N`. Considered and declined for now to keep the tabs decoupled. It would be cheap later, since the URL params exist.
- An any/all toggle for the multi-tag filter. Any-of only for now.

### Reviewed Todos (not folded)
Six pending todos matched only on generic keywords (`web`, `release`, `tag`, `query`) and are unrelated to find/filter. They are: move shadcn to devDependencies; resolve D-15 prev-release files from `--prev-tag`; unify the sqlscan quote state machines; the dead `resuming = true` in the digest chunker; pin chunk-count fixtures (already Phase 28); and singular/plural remainder grammar.

</deferred>

---

*Phase: 25-find-filter-watchlist-and-history*
*Context gathered: 2026-10-04*
