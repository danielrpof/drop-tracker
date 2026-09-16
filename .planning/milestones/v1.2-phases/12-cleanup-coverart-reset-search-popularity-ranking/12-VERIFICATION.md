---
phase: 12-cleanup-coverart-reset-search-popularity-ranking
verified: 2026-08-24T00:00:00Z
status: passed
score: 10/10 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking Verification Report

**Phase Goal:** Close two loose ends left after v1.1 closes: (1) `CoverArt.tsx`'s image-load-error state never resets when `src` changes on a retained component instance, so a component that once failed to load keeps showing the placeholder forever even if a later `src` would succeed; (2) search results aren't ranked by popularity and same-named artists (e.g. multiple "Drake"s) are hard to disambiguate, since MusicBrainz's search API doesn't rank by popularity and its `disambiguation` field is often blank.
**Verified:** 2026-08-24
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

Truths are the locked decisions D-01 through D-10 from `12-CONTEXT.md`, traced through the three plans' `requirements` fields (this phase has no REQ-IDs — D-01..D-10 are the authoritative scope contract).

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | D-01: a retained `CoverArt` instance showing the failure placeholder renders the real `<img>` again once `src` changes to a different value, via an effect (not a remount) | ✓ VERIFIED | `web/app/components/common/CoverArt.tsx:32-34` — `useEffect(() => { setFailed(false) }, [src])`; test `clears the failed placeholder when src changes` passes (`go`... N/A, ran `pnpm test`: 56/56 pass) |
| 2 | D-02: a regression test proves failing src → onError → new src → placeholder clears | ✓ VERIFIED | `web/app/components/common/CoverArt.test.tsx` — dedicated test present and passing; SUMMARY documents non-vacuity confirmation (effect removal breaks the test) |
| 3 | Zero call-site changes — fix lives entirely in `CoverArt.tsx`, no `key` prop introduced | ✓ VERIFIED | `git log --name-only` across phase commits confirms `WatchlistRow.tsx`/`EventCard.tsx`/`SearchResultsColumns.tsx` not touched by plan 12-01; only `CoverArt.tsx`/`CoverArt.test.tsx` modified |
| 4 | D-03: `deezer.Artist` carries the upstream fan count (`nb_fan`) after decode | ✓ VERIFIED | `internal/deezer/search.go:27` — `NbFan int \`json:"nb_fan"\`` alongside `NbAlbum`; `TestSearchArtists_DecodesFixture` asserts `NbFan == 24047501` against untouched live fixture — PASS |
| 5 | D-04: `Client.SearchArtists` sorts results by fan count descending, inside `internal/deezer` itself, ties keep upstream order | ✓ VERIFIED | `internal/deezer/search.go:103-105` — `slices.SortStableFunc` after `decodeChecked`; `TestSearchArtists_SortsByFanCountDescending` and `TestSearchArtists_EqualFanCountsPreserveUpstreamOrder` — both PASS (ran directly) |
| 6 | D-06/D-07: no ranking/sorting exists on the MusicBrainz path; `GET /search`'s musicbrainz column preserves the client's own order end to end | ✓ VERIFIED | `internal/musicbrainz/search.go` contains no `sort`/`slices.Sort` call; `internal/httpserver/search.go`'s `musicBrainzSource.SearchArtists` has no sort; `TestNewMusicBrainzSource_PreservesOrder` (4-artist order that matches neither name nor score order) — PASS (ran directly) |
| 7 | D-09: `musicbrainz.Artist` decodes the upstream `country` field | ✓ VERIFIED | `internal/musicbrainz/search.go:45` — `Country string \`json:"country"\`` positioned after `Disambiguation`; `TestSearchArtists_DecodesFixture` extended with `Country == "CA"` assertion against untouched fixture — package tests PASS |
| 8 | D-10: `GET /search` wire body carries a nullable `country` per artist — populated by the musicbrainz adapter, left null by the deezer adapter | ✓ VERIFIED | `internal/httpserver/search.go:39` — `Country *string`; musicbrainz adapter blank-to-nil block (lines 95-99); deezer adapter `Country: nil` (line 152); `TestNewMusicBrainzSource_MapsFields`, `TestNewDeezerSource_MapsFields`, `TestSearch_MusicBrainzCountryReachesResponseBody` all PASS |
| 9 | D-08/D-10: `SearchResultRow` shows disambiguation when present, falls back to country code in the same slot, no new UI element | ✓ VERIFIED | `web/app/components/watchlist/SearchResultsColumns.tsx:160` — `artist.disambiguation ?? artist.country` rendered in the existing `<span>`; frontend tests `falls back to the country code...`, `prefers disambiguation over country...`, `renders no secondary label when...both absent` all present in `SearchResultsColumns.test.tsx`; full suite 56/56 PASS |
| 10 | D-05: Deezer's fan-count popularity signal never crosses the `GET /search` HTTP boundary | ✓ VERIFIED | `internal/httpserver/search.go`'s `SearchArtist` struct has exactly 7 fields (source/id/name/disambiguation/country/type/image_url), no fan-count field; `TestSearchArtist_WireShapeKeySet` (asserts exact 7-key JSON set) — PASS (ran directly); `web/app/lib/api.ts`'s `SearchArtist` interface mirrors the same 7 fields |

**Score:** 10/10 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/app/components/common/CoverArt.tsx` | `useEffect([src])` resetting `failed` | ✓ VERIFIED | Present exactly as specified, comment locks the deliberate effect-over-remount decision |
| `web/app/components/common/CoverArt.test.tsx` | new file, 3 tests | ✓ VERIFIED | New file, 3 tests covering reset / unchanged-src guard / null-src placeholder — all pass |
| `internal/deezer/search.go` | `NbFan` field + sort inside client | ✓ VERIFIED | Field present, `SortStableFunc` present after decode-error return; doc comment rewritten (no stale "never sorts" claim) |
| `internal/deezer/search_test.go` | sort + tie tests, decode assertion | ✓ VERIFIED | `TestSearchArtists_SortsByFanCountDescending`, `TestSearchArtists_EqualFanCountsPreserveUpstreamOrder` present and passing; `TestSearchArtists_DecodesFixture` extended; old contradicting test (`PreservesUpstreamOrderNoSorting`) confirmed gone via negative grep |
| `internal/musicbrainz/search.go` | `Country` field, no sort added | ✓ VERIFIED | Field present after `Disambiguation`; zero `sort.`/`slices.Sort` occurrences |
| `internal/httpserver/search.go` | `SearchArtist.Country`, adapters wired | ✓ VERIFIED | Nullable field present; musicbrainz adapter populates, deezer adapter nils it explicitly with comment |
| `internal/httpserver/search_test.go` | 4 new tests (mapping, order, wire-key-set, e2e) | ✓ VERIFIED | `stubMusicBrainzArtistSearcher`, `TestSearch_MusicBrainzCountryReachesResponseBody`, `TestNewMusicBrainzSource_MapsFields`, `TestNewMusicBrainzSource_PreservesOrder`, `TestSearchArtist_WireShapeKeySet` all present and passing |
| `web/app/lib/api.ts` | `country: string \| null` field | ✓ VERIFIED | Present, mirrors `disambiguation` |
| `web/app/components/watchlist/SearchResultsColumns.tsx` | secondary-label fallback | ✓ VERIFIED | `secondaryLabel = artist.disambiguation ?? artist.country`, rendered in the existing `<span>` slot |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/deezer.Client.SearchArtists` | sort call | `slices.SortStableFunc` placed after `decodeChecked` error return | ✓ WIRED | Sort sits after the error return (`internal/deezer/search.go:84-105`); `TestSearchArtists_QuotaErrorInBodyWithHTTP200` still passes — sort never runs on a failed decode |
| `musicbrainz.Artist.Country` | `httpserver.SearchArtist.Country` | `musicBrainzSource.SearchArtists` adapter, blank-to-nil pointer convention | ✓ WIRED | `TestSearch_MusicBrainzCountryReachesResponseBody` proves country survives client → adapter → JSON encode → HTTP body |
| `httpserver.SearchArtist.country` | `web/app/lib/api.ts` `SearchArtist.country` | manual TS mirror (no shared codegen) | ✓ WIRED | Both sides carry `country: string \| null` / `Country *string` with `json:"country"`; `pnpm typecheck` clean, all 4 frontend `SearchArtist` literals updated |
| `SearchArtist.country`/`.disambiguation` | `SearchResultRow` secondary `<span>` | nullish-coalescing fallback in render | ✓ WIRED | `secondaryLabel = artist.disambiguation ?? artist.country` renders into the existing span; RTL tests confirm both directions and the both-absent case |
| Deezer `nb_fan` | `GET /search` HTTP body | (deliberately NOT wired — D-05 negative guarantee) | ✓ WIRED (absence proven) | `TestSearchArtist_WireShapeKeySet` pins exactly 7 JSON keys, no fan-count key possible |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Backend build compiles | `go build ./...` | exit 0, no output | ✓ PASS |
| Backend phase-scoped tests | `go test ./internal/deezer/... ./internal/musicbrainz/... ./internal/httpserver/... -count=1` | `ok` × 3 packages | ✓ PASS |
| Deezer sort/tie tests (named) | `go test ./internal/deezer/... -run '^TestSearchArtists_SortsByFanCountDescending$\|^TestSearchArtists_EqualFanCountsPreserveUpstreamOrder$' -v` | both `--- PASS` | ✓ PASS |
| MusicBrainz country e2e / order / wire-key-set (named) | `go test ./internal/httpserver/... -run '^TestSearch_MusicBrainzCountryReachesResponseBody$\|^TestNewMusicBrainzSource_PreservesOrder$\|^TestSearchArtist_WireShapeKeySet$' -v` | all three `--- PASS` | ✓ PASS |
| Frontend full suite | `cd web && pnpm test` | 10 files, 56/56 tests pass, coverage thresholds (statements 83.23%, branches 74.35%, functions 81.03%, lines 84.09%) all clear the 70% gate | ✓ PASS |
| Frontend typecheck | `cd web && pnpm typecheck` | exit 0, no output | ✓ PASS |

Note: `-race` was not re-run in this verification pass because it is a documented pre-existing environmental limitation on this Windows dev machine (ThreadSanitizer allocation failure — `.planning/WINDOWS.md`), consistent with both plan SUMMARYs' own documented workaround.

### Requirements Coverage

This phase has no REQ-IDs (`Requirements: TBD` in ROADMAP.md). CONTEXT.md's D-01 through D-10 serve as the authoritative scope contract and are traced through each plan's `requirements` frontmatter field:

| Decision | Source Plan | Description | Status | Evidence |
|----------|------------|-------------|--------|----------|
| D-01 | 12-01 | CoverArt effect-based reset | ✓ SATISFIED | See Truth #1 |
| D-02 | 12-01 | Regression test for the reset | ✓ SATISFIED | See Truth #2 |
| D-03 | 12-02 | Deezer `NbFan` field capture | ✓ SATISFIED | See Truth #4 |
| D-04 | 12-02 | Deezer descending fan-count sort, inside client | ✓ SATISFIED | See Truth #5 |
| D-05 | 12-03 | Fan count never crosses HTTP boundary | ✓ SATISFIED | See Truth #10 |
| D-06 | 12-03 | No new MusicBrainz ranking logic | ✓ SATISFIED | See Truth #6 |
| D-07 | 12-03 | Pipeline-order test for MusicBrainz path | ✓ SATISFIED | See Truth #6 |
| D-08 | 12-03 | Country as disambiguation fallback | ✓ SATISFIED | See Truth #9 |
| D-09 | 12-03 | MusicBrainz `country` decoded | ✓ SATISFIED | See Truth #7 |
| D-10 | 12-03 | Country in wire shape + same UI slot | ✓ SATISFIED | See Truth #8, #9 |

No orphaned decisions — all 10 are traced through plans and verified against the codebase.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/deezer/search.go` | 93-102 | Duplicated 5-line comment block above the sort call (identical text repeated twice) | ℹ️ Info | Cosmetic only — the sort logic itself appears exactly once and is correct. Already flagged as `IN-01` in `12-REVIEW.md`, not a functional defect, does not block the goal. |

No debt markers (`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`) found in any of the 12 files this phase touched (verified by direct grep — zero matches). `12-REVIEW.md`'s `WR-01` (misleading "showing X results only" copy in `SearchResultsColumns.tsx`) is a pre-existing bug in code this phase's diff did not touch (confirmed: it sits a few lines above the `secondaryLabel` edit and predates D-10) — not a regression introduced by this phase, and not a truth this phase claimed to fix.

### Human Verification Required

None. All ten truths are either presence/wiring facts (struct fields, wire shapes, no-sort guarantees) or state-transition/UI-render facts directly exercised by non-vacuous automated tests (confirmed via each plan's documented mutation-and-restore non-vacuity checks, and independently re-run in this verification pass for the highest-risk named tests). No visual, real-time, or external-service behavior requires manual confirmation for this phase's scope.

### Gaps Summary

None. All 10 locked decisions (D-01 through D-10) are implemented, tested with non-vacuous regression tests, and independently re-verified in this pass by rebuilding, re-running the backend package tests, re-running the specific named tests, and re-running the full frontend suite plus typecheck — all green. The two code-review findings (`WR-01`, `IN-01`) are non-blocking: one is a cosmetic duplicate comment, the other is a pre-existing bug in unmodified code outside this phase's claimed scope.

---

_Verified: 2026-08-24_
_Verifier: Claude (gsd-verifier)_
