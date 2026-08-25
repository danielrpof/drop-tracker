# Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-18
**Phase:** 12-cleanup-coverart-reset-search-popularity-ranking
**Areas discussed:** CoverArt reset mechanism, Deezer popularity signal, MusicBrainz ranking strategy, Same-name disambiguation

---

## CoverArt reset mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| useEffect reset on src change | `useEffect(() => setFailed(false), [src])` — minimal, surgical, no remount cost | ✓ |
| key={src} forces remount | Caller passes `key={src}`, forces full unmount/remount at every call site | |
| You decide | Let researcher/planner pick | |

**User's choice:** useEffect reset on src change

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, add a test | Component test rendering failing src → onError → new src → assert placeholder clears | ✓ |
| No test, fix only | Ship the fix without a dedicated regression test | |
| You decide | Let planner decide | |

**User's choice:** Yes, add a test
**Notes:** Fix lives entirely inside `CoverArt.tsx`, so it automatically covers all three consumers (`WatchlistRow.tsx`, `EventCard.tsx`, `SearchResultsColumns.tsx`) — confirmed via grep before closing this area.

---

## Deezer popularity signal

| Option | Description | Selected |
|--------|-------------|----------|
| Sort inside internal/deezer client | SearchArtists sorts by nb_fan descending before returning | ✓ |
| Sort in httpserver adapter | deezerSource.SearchArtists sorts after calling the client | |
| You decide | Let planner pick based on existing patterns | |

**User's choice:** Sort inside internal/deezer client

| Option | Description | Selected |
|--------|-------------|----------|
| Sort key only, not exposed | Fan count stays internal; SearchArtist wire shape unchanged | ✓ |
| Expose it in the API response | Add a fan_count field to SearchArtist, touches wire contract + frontend | |
| You decide | Let planner decide | |

**User's choice:** Sort key only, not exposed

---

## MusicBrainz ranking strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Trust MB's existing order, no change | MB already returns relevance-sorted results; leave as-is | ✓ |
| Fetch extra signal per top-N result | Extra per-result API call as a popularity proxy — added latency/quota cost | |
| You decide | Let researcher investigate and recommend | |

**User's choice:** Trust MB's existing order, no change

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, add a pipeline-order test | Prove GET /search's musicbrainz column preserves client order end-to-end | ✓ |
| No, not worth a dedicated test | Existing search_test.go coverage is implicitly sufficient | |
| You decide | Let planner judge | |

**User's choice:** Yes, add a pipeline-order test

---

## Same-name disambiguation

| Option | Description | Selected |
|--------|-------------|----------|
| Ranking is sufficient, nothing else | No new disambiguation data; ranking work alone closes the gap | |
| Add a supplementary hint for blank disambiguation | Show something extra (e.g. country, life-span) when disambiguation is blank | ✓ |
| You decide | Let planner judge | |

**User's choice:** Add a supplementary hint for blank disambiguation

| Option | Description | Selected |
|--------|-------------|----------|
| Country code | Already in MB's base search response, zero extra API-call cost | ✓ |
| Life-span / founding year | More distinguishing, but needs an extra per-result API call | |
| You decide | Let researcher confirm what's free and recommend | |

**User's choice:** Country code

| Option | Description | Selected |
|--------|-------------|----------|
| Replace disambiguation line with country | Same slot: MB disambiguation when present, country fallback when blank | ✓ |
| Show country alongside/separately | New distinct UI element regardless of disambiguation presence | |
| You decide | Let planner/UI pass decide | |

**User's choice:** Replace disambiguation line with country

---

## Claude's Discretion

None — every gray area reached an explicit user decision this session.

## Deferred Ideas

None. Two candidate additions were explicitly considered and rejected (not deferred) for cost reasons:
- MusicBrainz per-result popularity lookup (extra API call per search-as-you-type keystroke)
- Life-span/founding-year disambiguation hint (not in MB's base search response, needs a separate lookup)
