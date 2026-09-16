---
phase: 12-cleanup-coverart-reset-search-popularity-ranking
plan: 02
subsystem: api
tags: [go, deezer, search, sorting, slices, stdlib]

# Dependency graph
requires:
  - phase: 03-external-clients-search
    provides: "internal/deezer.Client.SearchArtists and the Artist struct this plan extends"
provides:
  - "deezer.Artist.NbFan field carrying Deezer's upstream fan-count popularity signal"
  - "Client.SearchArtists results ranked by fan count descending, ties broken by Deezer's own relevance order"
affects: [12-03, watchlist search UI ordering]

# Actuals (#2632)
actuals:
  tokens: 1864
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns: ["slices.SortStableFunc + cmp.Compare for a descending, tie-order-preserving sort over a small bounded slice"]

key-files:
  created: []
  modified:
    - internal/deezer/search.go
    - internal/deezer/search_test.go

key-decisions:
  - "Used slices.SortStableFunc instead of PATTERNS.md's suggested slices.SortFunc, per the plan's own recorded deviation -- SortFunc is unstable and would scramble Deezer's relevance order among artists sharing a fan count (very common, since an unmatched artist decodes to the zero value); SortStableFunc keeps popularity as the primary key while preserving upstream order as the tiebreaker."

patterns-established:
  - "Popularity-sort placement: the sort call sits after the decodeChecked error return and after the artists slice is built, so an HTTP-200 in-body upstream error always short-circuits before any sorting happens."

requirements-completed: [D-03, D-04]

coverage:
  - id: D1
    description: "deezer.Artist gains NbFan int (json:nb_fan), decoded and round-tripped from the live Drake fixture's 24047501 value"
    requirement: "D-03"
    verification:
      - kind: unit
        ref: "internal/deezer/search_test.go#TestSearchArtists_DecodesFixture"
        status: pass
    human_judgment: false
  - id: D2
    description: "Client.SearchArtists sorts results by fan count descending, proven against a fixture whose upstream order deliberately disagrees with fan-count order"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/deezer/search_test.go#TestSearchArtists_SortsByFanCountDescending"
        status: pass
    human_judgment: false
  - id: D3
    description: "Artists sharing a fan count keep Deezer's own upstream relative order (tie-order guarantee behind the stable sort choice)"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/deezer/search_test.go#TestSearchArtists_EqualFanCountsPreserveUpstreamOrder"
        status: pass
    human_judgment: false
  - id: D4
    description: "An HTTP-200 in-body Deezer quota error still returns a *APIError and a nil slice -- sorting never runs on a failed decode"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "internal/deezer/search_test.go#TestSearchArtists_QuotaErrorInBodyWithHTTP200"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-08-19
status: complete
---

# Phase 12 Plan 02: Deezer Search Popularity Ranking Summary

**Deezer's `/search/artist` fan-count signal now drives result ordering — `Client.SearchArtists` returns the most popular matching artist first, via a stable descending sort that preserves Deezer's own relevance order for ties.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-08-19T02:42:55Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added `NbFan int` (tagged `nb_fan`) to `deezer.Artist`, decoding Deezer's upstream popularity number that the client previously threw away
- `Client.SearchArtists` now sorts its results by `NbFan` descending using `slices.SortStableFunc`, so the artist the user almost certainly meant surfaces first instead of Deezer's raw catalogue order
- Ties (very common — an unmatched artist decodes `NbFan` to zero) keep Deezer's own upstream relevance order rather than being scrambled, proven by a dedicated three-artist test
- Rewrote the stale "this method never sorts" doc comment on `SearchArtists` to describe the new guarantee
- Replaced the now-false `TestSearchArtists_PreservesUpstreamOrderNoSorting` with `TestSearchArtists_SortsByFanCountDescending`, backed by a fixture whose upstream order deliberately disagrees with fan-count order so the test cannot pass vacuously

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end — Deezer search returns fan-count-ranked results** - `a3e4916` (feat)
2. **Task 2: Prove the decode and the tie-order guarantee** - `ded4dbf` (test)

## Files Created/Modified

- `internal/deezer/search.go` - Added `NbFan int` field to `Artist`; added `cmp`/`slices` imports; sort `Client.SearchArtists` results by fan count descending (stable) after the decode-error return, before the final `return`; rewrote the `SearchArtists` doc comment
- `internal/deezer/search_test.go` - Renamed `twoArtistsSearchFixture` → `twoArtistsSearchFixtureRanked` with deliberately out-of-order fan counts; renamed/rewrote `TestSearchArtists_PreservesUpstreamOrderNoSorting` → `TestSearchArtists_SortsByFanCountDescending`; extended `TestSearchArtists_DecodesFixture` with an `NbFan` assertion; added `threeArtistsTiedFanFixture` and `TestSearchArtists_EqualFanCountsPreserveUpstreamOrder`

## Decisions Made

- Followed the plan's own recorded deviation from `12-PATTERNS.md`: used `slices.SortStableFunc` instead of the pattern map's suggested `slices.SortFunc`. `SortFunc` is not stable, so every artist sharing a fan count (the common case — an unmatched artist decodes to the zero value) would have its Deezer relevance order scrambled arbitrarily among ties. `SortStableFunc` keeps fan count as the primary sort key while preserving Deezer's own relevance order as the tiebreaker, which is what D-04's "rank by popularity" intent actually wants. Cost is nil on a slice bounded by `clampLimit`.

## Deviations from Plan

None beyond the plan's own explicitly pre-recorded `SortStableFunc` deviation (already captured above and in the plan's own `<objective>` block as a deliberate, traceable choice, not something discovered during execution).

## Issues Encountered

- `go test -race` is unusable on this Windows dev machine (documented pre-existing environmental limitation — ThreadSanitizer allocation failure under memory pressure; see `.planning/WINDOWS.md` and `PROJECT.md` Context). All plan verification commands specifying `-race` were run without that flag instead; every test passed.
- `gofmt -l internal/deezer` reports every file in the package (including files this plan never touched, e.g. `client.go`) as needing reformatting. This is a pre-existing Windows checkout artifact from this machine's `core.autocrlf=true` git config (no `.gitattributes` pins line endings), not a formatting defect: stripping the CRLF artifact and re-running `gofmt -l` on the edited files' content confirms both `search.go` and `search_test.go` are gofmt-clean. `golangci-lint`'s pre-commit hook (which normalizes line endings) passed clean on both commits. This is the same category of pre-existing, environmental, non-code-defect limitation already documented for `-race`.

## Non-Vacuity Confirmation

Per the plan's acceptance criteria, the sort call was temporarily neutralized (comparator forced to return 0, a guaranteed stable no-op) and `TestSearchArtists_SortsByFanCountDescending` was re-run — it failed as expected (`order = ["Artist A", "Artist B"], want [Artist B, Artist A]`), confirming the test is not vacuous. The real sort was then restored and the full targeted verification re-run clean.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- D-03 and D-04 are closed: the fan-count field decodes and `Client.SearchArtists` ranks by it, ties broken by upstream order.
- D-05 (fan count never crosses the HTTP boundary) is explicitly out of scope for this plan and is asserted by plan 12-03, which this plan does not block.
- No file outside `internal/deezer` was touched — `git diff --name-only` from the phase base confirms exactly `internal/deezer/search.go` and `internal/deezer/search_test.go`.

---
*Phase: 12-cleanup-coverart-reset-search-popularity-ranking*
*Completed: 2026-08-19*

## Self-Check: PASSED

- FOUND: internal/deezer/search.go
- FOUND: internal/deezer/search_test.go
- FOUND: .planning/phases/12-cleanup-coverart-reset-search-popularity-ranking/12-02-SUMMARY.md
- FOUND: commit a3e4916 (feat(12-02): rank Deezer search results by fan-count popularity)
- FOUND: commit ded4dbf (test(12-02): assert fan-count decode and tie-order stability)
- FOUND: commit 8882b6b (docs(12-02): add plan summary)
