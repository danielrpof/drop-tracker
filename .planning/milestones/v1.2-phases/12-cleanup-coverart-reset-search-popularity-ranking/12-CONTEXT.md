# Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking - Context

**Gathered:** 2026-08-18
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase closes two independent, pre-scoped loose ends left after v1.1:

1. **CoverArt reset bug** — `CoverArt.tsx`'s image-load-error state (`failed`, a `useState`) never clears when `src` changes on a retained component instance, so a row that once failed to load keeps showing the placeholder forever even if a later `src` would succeed. Shared by History rows, Watchlist rows, and search-result rows (`SearchResultsColumns.tsx`) — all three consumers.
2. **Search popularity ranking / same-name disambiguation** — MusicBrainz search results aren't ranked by popularity (only text-match relevance) and same-named artists (e.g. multiple "Drake"s) are hard to tell apart when MusicBrainz's `disambiguation` field is blank. Deezer results similarly aren't ranked by any popularity signal even though the data (`nb_fan`) already exists in Deezer's response and is simply not captured.

Both items were explicitly named in ROADMAP.md's Phase 12 entry — no new capabilities beyond these two are in scope.

</domain>

<decisions>
## Implementation Decisions

### CoverArt reset mechanism
- **D-01:** Fix via `useEffect` keyed on `src` that resets `failed` to `false` — not a `key={src}` remount. Keeps the component instance retained; no call-site changes needed at any of the three consumers (`WatchlistRow.tsx`, `EventCard.tsx`, `SearchResultsColumns.tsx`) since the fix lives entirely inside `CoverArt.tsx`.
- **D-02:** Add a regression test proving the reset: render with a failing `src`, trigger `onError`, rerender with a new `src`, assert the placeholder clears.

### Deezer popularity signal
- **D-03:** Add `NbFan` (`nb_fan` JSON field — already present in Deezer's live `/search/artist` response, confirmed in `internal/deezer/search_test.go`'s fixture) to `internal/deezer.Artist`.
- **D-04:** Sort `SearchArtists` results by fan count descending **inside `internal/deezer`'s client itself**, not in the `httpserver` adapter (`deezerSource.SearchArtists` in `internal/httpserver/search.go`). — **Reversibility:** reversible — pure internal sort logic, no wire-format change.
- **D-05:** `SearchArtist`'s wire shape (source/id/name/disambiguation/type/image_url) stays **unchanged** — fan count is a server-side sort key only, never exposed to the frontend.
- **Note for planner:** `internal/deezer/search.go`'s existing doc comment ("Results are returned in Deezer's own order — this method never sorts") will need updating to reflect the new sort behavior.

### MusicBrainz ranking strategy
- **D-06:** No new ranking logic for MusicBrainz. `internal/musicbrainz.SearchArtists` already returns results pre-sorted by MusicBrainz's own relevance `score` — trust that order as-is. MusicBrainz's search API has no true popularity signal in its base response, and fetching one would require an extra per-result API call on a rate-limited, search-as-you-type endpoint — explicitly rejected as not worth the latency/quota cost.
- **D-07:** Add a pipeline-order test proving `GET /search`'s musicbrainz column preserves the client's returned order end-to-end (insurance against the Deezer sort work in D-04 accidentally leaking into the MusicBrainz path).

### Same-name disambiguation
- **D-08:** Ranking alone was judged insufficient — add a supplementary fallback hint for when MusicBrainz's `disambiguation` field is blank.
- **D-09:** Use MusicBrainz's `country` field (e.g. `"CA"`) as the fallback. It is already present in MusicBrainz's base `/ws/2/artist` search response at zero extra API-call cost (live-verified in `.planning/milestones/v1.0-phases/03-external-clients-search/03-RESEARCH.md`'s Drake example: `"country": "CA"` returned alongside `"disambiguation": "Canadian rapper"` with no extra `inc` param). Life-span/founding-year was considered and rejected — not present in the base response, would need a separate per-result lookup (same cost tradeoff D-06 already rejected).
- **D-10:** Render the fallback in the **same UI slot** as disambiguation in `SearchResultRow` (`SearchResultsColumns.tsx`) — show MusicBrainz's `disambiguation` text when present, fall back to the country code when blank. No new UI element added.
- **Note for planner:** `SearchArtist`'s wire shape will need a new field to carry country (currently only `Disambiguation *string` exists) — this is additive, not a breaking change to the existing shape.

### Claude's Discretion
None — every gray area reached an explicit user decision this session.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### CoverArt bug origin
- `.planning/v1.1-MILESTONE-AUDIT.md` — original flag of the CoverArt.tsx reset bug as pre-existing, non-blocking tech debt

### Search popularity / disambiguation origin
- `.planning/milestones/v1.0-phases/06-frontend-release-history/06-04-SUMMARY.md` §"UAT" — original capture of the popularity/disambiguation gap during Phase 6 UAT, accepted as valid-but-out-of-scope, promoted to backlog Phase 999.1 and later folded into this phase
- `.planning/milestones/v1.0-phases/03-external-clients-search/03-RESEARCH.md` — live-verified MusicBrainz `/ws/2/artist` search response shape (includes `score`, `country`, `disambiguation` fields with no extra `inc` param); live-verified Deezer pitfalls (error-in-200-body pattern relevant if this phase's Deezer client changes touch response handling)

### Roadmap
- `.planning/ROADMAP.md` §"Phase 12" — locked phase goal and context notes this discussion elaborated on

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/app/components/common/CoverArt.tsx` — the single shared component; fixing it here fixes all three consumers (`WatchlistRow.tsx`, `EventCard.tsx`, `SearchResultsColumns.tsx`) with zero call-site changes
- `internal/deezer/search_test.go` — existing fixture already contains a live-shaped `nb_fan` value (24047501), useful as a starting point for the new sort test
- `internal/httpserver/search.go`'s `SearchArtist` wire struct and `musicBrainzSource`/`deezerSource` adapters — the seam where a new `Country` field (D-10) and the (internal-only) Deezer sort (D-04) both attach

### Established Patterns
- `internal/deezer.Artist` and `internal/musicbrainz.Artist` are small first-class structs with `encoding/json` tags, decoded directly — matches the project's typed-domain-object convention (no untyped `map[string]any`)
- `SearchArtist`'s wire shape uses nullable `*string` fields (`Disambiguation *string`, `ImageURL *string`) for optional upstream data — a new `Country *string` field should follow the same nullability convention
- Both `internal/deezer` and `internal/musicbrainz` search clients currently document "never sorts" — D-04 changes this guarantee for Deezer only; the doc comment must be updated accordingly

### Integration Points
- `internal/httpserver/search.go`'s `deezerSource.SearchArtists` and `musicBrainzSource.SearchArtists` adapter methods — where `SearchArtist.Country` gets populated from MusicBrainz's `country` field
- `web/app/components/watchlist/SearchResultsColumns.tsx`'s `SearchResultRow` — where the disambiguation-or-country fallback render logic (D-10) lands
- `web/app/lib/api.ts` — frontend `SearchArtist` TypeScript type will need the new `country` field mirrored from the Go wire shape

</code_context>

<specifics>
## Specific Ideas

No additional specific UI/UX references beyond what's captured in Decisions above — both fixes stay tightly scoped to the exact gaps ROADMAP.md named.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (Life-span/founding-year disambiguation and MusicBrainz per-result popularity lookups were considered and explicitly rejected for cost reasons, not deferred as future work — see D-06 and D-09.)

</deferred>

---

*Phase: 12-cleanup-coverart-reset-search-popularity-ranking*
*Context gathered: 2026-08-18*
