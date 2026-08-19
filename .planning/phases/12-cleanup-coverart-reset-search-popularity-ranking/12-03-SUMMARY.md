---
phase: 12-cleanup-coverart-reset-search-popularity-ranking
plan: 03
subsystem: api
tags: [go, typescript, musicbrainz, search, react, testing]

# Dependency graph
requires:
  - phase: 12-cleanup-coverart-reset-search-popularity-ranking
    provides: "12-02: deezer.Artist.NbFan popularity signal and fan-count sorting on the Deezer search path (D-05's negative guarantee is asserted against that same wire struct)"
provides:
  - "musicbrainz.Artist.Country (string, tagged country) decoded from the upstream response"
  - "httpserver.SearchArtist.Country (*string, tagged country) -- nullable wire field, populated by the musicbrainz adapter, left nil by deezer"
  - "SearchArtist.country: string | null in web/app/lib/api.ts, mirroring disambiguation"
  - "SearchResultRow's secondary label resolves disambiguation then country via nullish coalescing, in the existing label slot"
  - "Automated guard: TestSearchArtist_WireShapeKeySet pins the wire struct to exactly seven JSON keys (D-05)"
  - "Automated guard: TestNewMusicBrainzSource_PreservesOrder proves no ranking logic exists on the MusicBrainz adapter path (D-06/D-07)"
affects: [search UI, watchlist add flow]

# Actuals (#2632)
actuals:
  tokens: 4700
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns: ["nullish-coalescing fallback chain for a single rendered label slot backed by two optional upstream fields"]

key-files:
  created: []
  modified:
    - internal/musicbrainz/search.go
    - internal/musicbrainz/search_test.go
    - internal/httpserver/search.go
    - internal/httpserver/search_test.go
    - web/app/lib/api.ts
    - web/app/components/watchlist/SearchResultsColumns.tsx
    - web/app/components/watchlist/SearchResultsColumns.test.tsx
    - web/app/components/watchlist/SearchBox.test.tsx

key-decisions:
  - "Split Task 1's fixtures across tasks as scoped by the plan: countryOnlyArtist landed in Task 1 (the tracer's own render proof), bothHintsArtist landed in Task 3 (the preference-guardrail test) -- kept each task's commit to exactly the fixtures its own tests need."

patterns-established:
  - "Secondary-label fallback: `artist.disambiguation ?? artist.country` -- nullish coalescing specifically (not `||`), since the backend already maps a blank upstream value to JSON null, and a truthiness fallback would incorrectly swallow a legitimately empty-string disambiguation."

requirements-completed: [D-05, D-06, D-07, D-08, D-09, D-10]

coverage:
  - id: D1
    description: "musicbrainz.Artist decodes MusicBrainz's country key into a new Country string field, positioned after Disambiguation"
    requirement: "D-09"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/search_test.go#TestSearchArtists_DecodesFixture"
        status: pass
    human_judgment: false
  - id: D2
    description: "SearchArtist wire struct carries a nullable Country field; the musicbrainz adapter maps a non-empty upstream value to a pointer and an empty one to nil, using the same convention as Disambiguation; the deezer adapter always sets it nil"
    requirement: "D-10"
    verification:
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestNewMusicBrainzSource_MapsFields"
        status: pass
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestNewDeezerSource_MapsFields"
        status: pass
      - kind: integration
        ref: "internal/httpserver/search_test.go#TestSearch_MusicBrainzCountryReachesResponseBody"
        status: pass
    human_judgment: false
  - id: D3
    description: "web/app/lib/api.ts's SearchArtist interface mirrors the Go wire type with a required country: string | null field"
    requirement: "D-10"
    verification:
      - kind: unit
        ref: "cd web && pnpm typecheck (react-router typegen && tsc)"
        status: pass
    human_judgment: false
  - id: D4
    description: "SearchResultRow renders one secondary label resolving disambiguation then country via nullish coalescing, in the existing label slot -- no new UI element"
    requirement: "D-08, D-10"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#falls back to the country code when disambiguation is blank"
        status: pass
      - kind: unit
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#prefers disambiguation over country when both are present"
        status: pass
      - kind: unit
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#renders no secondary label when disambiguation and country are both absent"
        status: pass
    human_judgment: false
  - id: D5
    description: "No ranking/sorting logic exists on the MusicBrainz search path -- GET /search's musicbrainz column returns the client's own order unchanged"
    requirement: "D-06, D-07"
    verification:
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestNewMusicBrainzSource_PreservesOrder"
        status: pass
    human_judgment: false
  - id: D6
    description: "Deezer's fan-count popularity signal never crosses the GET /search HTTP boundary -- the wire struct's JSON key set is pinned to exactly seven keys"
    requirement: "D-05"
    verification:
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestSearchArtist_WireShapeKeySet"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-08-19
status: complete
---

# Phase 12 Plan 03: MusicBrainz Country Fallback & Search Guardrails Summary

**MusicBrainz's country code now threads from the raw JSON response through `musicbrainz.Artist`, `SearchArtist`'s nullable wire field, and the TS mirror type into the existing search-result secondary-label slot as a fallback when disambiguation is blank, backed by automated guards proving MusicBrainz's result order is never sorted and Deezer's popularity signal never reaches the wire.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-08-19T03:00:06Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- Added `Country string` (tagged `country`) to `musicbrainz.Artist`, closing 12-RESEARCH.md Discrepancy 1 -- the raw response already carried the field, nothing decoded it
- Added `Country *string` (tagged `country`) to `httpserver.SearchArtist`, following the existing blank-to-nil pointer convention `Disambiguation` and `ImageURL` already use; musicbrainz's adapter populates it, deezer's adapter always leaves it nil
- `web/app/lib/api.ts`'s `SearchArtist` interface and `SearchResultRow`'s render both updated in the same task (Task 1), per 12-RESEARCH.md Pitfall 4 -- the plan never passed through a broken typecheck
- `SearchResultRow` now resolves its one secondary label via `artist.disambiguation ?? artist.country` -- nullish coalescing specifically, so a legitimately empty-string disambiguation is never swallowed
- Added `TestSearch_MusicBrainzCountryReachesResponseBody` (Go, end-to-end through the real router) and the RTL fallback case as the tracer's proof for both halves of the path
- Added `TestNewMusicBrainzSource_MapsFields`, `TestNewMusicBrainzSource_PreservesOrder`, and `TestSearchArtist_WireShapeKeySet` as the backend guardrails for D-06/D-07 (no MusicBrainz ranking) and D-05 (no fan-count leak)
- Added the disambiguation-over-country preference test and the both-absent case, closing the frontend guardrail set

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end -- a blank-disambiguation MusicBrainz artist shows its country in the search row** - `ae628a2` (feat)
2. **Task 2: Backend guardrails -- adapter mapping, preserved MusicBrainz order, no fan-count on the wire** - `1cc9a2b` (test)
3. **Task 3: Frontend guardrails -- disambiguation wins, and neither shown when both are absent** - `91c8f97` (test)

## Files Created/Modified

- `internal/musicbrainz/search.go` - Added `Country string` field to `Artist`, positioned after `Disambiguation`; reworded the struct's doc comment
- `internal/musicbrainz/search_test.go` - Extended `TestSearchArtists_DecodesFixture` with a `Country` assertion against the untouched `driveFixture`
- `internal/httpserver/search.go` - Added `Country *string` to `SearchArtist`; `musicBrainzSource.SearchArtists` maps it via the existing blank-to-nil convention; `deezerSource.SearchArtists` sets it explicitly nil with a comment
- `internal/httpserver/search_test.go` - Added `stubMusicBrainzArtistSearcher`; added `TestSearch_MusicBrainzCountryReachesResponseBody`, `TestNewMusicBrainzSource_MapsFields`, `TestNewMusicBrainzSource_PreservesOrder`, `TestSearchArtist_WireShapeKeySet`; extended `TestNewDeezerSource_MapsFields` with a country-is-nil assertion
- `web/app/lib/api.ts` - Added `country: string | null` to the `SearchArtist` interface, mirroring `disambiguation`
- `web/app/components/watchlist/SearchResultsColumns.tsx` - Added the `secondaryLabel` local resolving disambiguation then country; rewrote the secondary `<span>` to render it; updated the component's doc comment
- `web/app/components/watchlist/SearchResultsColumns.test.tsx` - Added `country: null` to three existing fixtures; added `countryOnlyArtist` and `bothHintsArtist` fixtures; added the fallback, preference, and both-absent test cases
- `web/app/components/watchlist/SearchBox.test.tsx` - Added `country: null` to the one inline `SearchArtist` literal

## Decisions Made

- Followed the plan's own task-scoping: `countryOnlyArtist` (needed by Task 1's own render proof) landed in Task 1's commit; `bothHintsArtist` (needed only by Task 3's preference test) landed in Task 3's commit, keeping each commit's diff scoped to the tests it introduces.

## Deviations from Plan

None - plan executed exactly as written. All must-haves, artifacts, and key-links from the plan's frontmatter were satisfied without needing an architectural change or an out-of-scope fix.

## Issues Encountered

- `go test -race` remains unusable on this Windows dev machine (documented pre-existing environmental limitation -- ThreadSanitizer allocation failure under memory pressure; see `.planning/WINDOWS.md`). All verification commands specifying `-race` were run without that flag instead; every test passed.
- `gofmt -l internal/musicbrainz internal/httpserver` reports every file in both packages (including files this plan never touched) as needing reformatting -- the same pre-existing Windows checkout CRLF artifact documented in 12-02-SUMMARY.md (`core.autocrlf=true`, no `.gitattributes` line-ending pin). Confirmed non-defect by stripping CRLF from each edited file's content and re-running `gofmt -l` against the stripped copy: `internal/musicbrainz/search.go`, `internal/musicbrainz/search_test.go`, `internal/httpserver/search.go`, and `internal/httpserver/search_test.go` are all gofmt-clean. `golangci-lint`'s pre-commit hook (which normalizes line endings) passed clean on all three commits.

## Non-Vacuity Confirmation

Per the plan's acceptance criteria, four guard assertions were confirmed non-vacuous by temporarily breaking each one, observing the expected failure, then restoring:

- Removing the adapter's country assignment in `musicBrainzSource.SearchArtists` made `TestSearch_MusicBrainzCountryReachesResponseBody` fail (`Country = <nil>, want a pointer to "CA"`).
- Removing the render fallback (`secondaryLabel = artist.disambiguation`, dropping `?? artist.country`) made the RTL fallback case fail (`getByText("CA")` found nothing).
- Adding a throwaway extra JSON-tagged field to `SearchArtist` made `TestSearchArtist_WireShapeKeySet` fail (`key count = 8, want 7`, naming the unexpected key).
- Swapping the render's fallback order (`artist.country ?? artist.disambiguation`) made the "prefers disambiguation over country" case fail.

All four were restored and the full targeted verification re-run clean; each restored file diffed byte-identical against its pre-mutation backup.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- D-05 through D-10 are all closed: the country fallback reaches the DOM, MusicBrainz's order is provably unsorted, and Deezer's fan count is provably absent from the wire.
- Backend (`go build`, `go vet`, targeted package tests) and frontend (`pnpm typecheck`, full `pnpm test` with coverage thresholds) are both green.
- No file outside this plan's declared `files_modified` list was touched.

---
*Phase: 12-cleanup-coverart-reset-search-popularity-ranking*
*Completed: 2026-08-19*
