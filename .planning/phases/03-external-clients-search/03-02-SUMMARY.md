---
phase: 03-external-clients-search
plan: 02
subsystem: api
tags: [deezer, net-http, rate-limiting, chi, golang.org/x/time]

# Dependency graph
requires:
  - phase: 03-external-clients-search
    plan: 01
    provides: "musicbrainz client shape (doRequest choke point, cancelReadCloser, clampLimit, decodeChecked-equivalent pattern), the source-keyed GET /search envelope, and the httpserver.SearchSource/NewXSource adapter seam this plan's Deezer package and adapter mirror exactly"
provides:
  - "internal/deezer: rate-limited Deezer JSON API client with SearchArtists (/search/artist), ArtistAlbums (/artist/{id}/albums, D-12), the ArtistSearcher/AlbumLister narrow seams, and HTTP-200 in-body error detection via decodeChecked"
  - "GET /search now fans out to both musicbrainz and deezer sources, returning two independent source-tagged artist lists with no cross-source merge/dedupe (D-01, D-02), either able to fail without taking the request down (D-03)"
  - "exactly one process-wide *deezer.Client and rate.Limiter constructed in cmd/server/main.go, sized from DeezerRateLimitPer5s, ready for plan 03-04's Deezer poll cycle to reuse (D-07, D-08)"
affects: [03-03-musicbrainz-poll-client, 03-04-scheduler]

# Actuals (#2632)
actuals:
  tokens: 11800
  tasks: 3
  commits: 6

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "decodeChecked: unmarshal into a probe struct carrying only the error key first, return that error if present, only then decode into the caller's real success type -- so an HTTP-200 in-body error can never surface as a zero-valued success (mirrors musicbrainz's discipline, adds the in-body check musicbrainz's upstream doesn't need)"
    - "AlbumLister as a second narrow seam alongside ArtistSearcher on the same *Client, so a poller consumer (plan 03-04) can depend on ArtistAlbums alone without pulling in search"
    - "ReleaseDate/dates from a third-party API kept as raw strings all the way through the client -- never parsed into time.Time at the client boundary, since the upstream returns partial dates the client cannot faithfully round-trip"

key-files:
  created:
    - internal/deezer/client.go
    - internal/deezer/search.go
    - internal/deezer/search_test.go
    - internal/deezer/albums.go
    - internal/deezer/albums_test.go
  modified:
    - internal/httpserver/search.go
    - internal/httpserver/search_test.go
    - cmd/server/main.go

key-decisions:
  - "Task 2 was recovered from a stalled prior executor run: internal/deezer/albums.go, albums_test.go, and the AlbumLister addition to client.go were already written but uncommitted. Verified against the plan's <behavior>/<action>/<acceptance_criteria> before trusting it -- correct, reused task 1's doRequest/decodeChecked/clampLimit unchanged, matched every behavior case -- then ran the full acceptance-criteria grep suite and go test -short ./... myself before committing, preserving the RED-then-GREEN commit split (test file alone, then albums.go + client.go's AlbumLister addition together, since both were authored in the same uncommitted batch)."
  - "This plan closes WLST-01 and CLNT-03, deferred by 03-01 pending Deezer landing (both requirement descriptions name MusicBrainz AND Deezer); CLNT-02 (Deezer client existing) also closes here."

patterns-established:
  - "Pattern: a second external-client package (internal/deezer) mirrors an established sibling package (internal/musicbrainz) field-for-field, with only its protocol-specific differences (no User-Agent requirement, in-body HTTP-200 error envelope) diverging -- confirms the plan 03-01 shape generalizes rather than being MusicBrainz-specific."
  - "Pattern: adding a second SearchSource to GET /search required zero changes to handleSearch, httpserver.New, or Server -- confirms 03-01's source-keyed envelope design was genuinely additive as intended."

requirements-completed: [WLST-01, CLNT-02, CLNT-03]

coverage:
  - id: D-12
    description: "ArtistAlbums(ctx, artistID, limit) fetches an artist's albums from Deezer's /artist/{id}/albums, refuses an empty/whitespace id before any request is built, and treats a nonexistent artist's empty result as success not error"
    requirement: "CLNT-02"
    verification:
      - kind: unit
        ref: "internal/deezer/albums_test.go#TestArtistAlbums_DecodesFixture"
        status: pass
      - kind: unit
        ref: "internal/deezer/albums_test.go#TestArtistAlbums_EmptyArtistIDReturnsErrorWithZeroRequests"
        status: pass
      - kind: unit
        ref: "internal/deezer/albums_test.go#TestArtistAlbums_NonexistentArtistReturnsEmptyNonNilNoError"
        status: pass
    human_judgment: false
  - id: D-01/D-02
    description: "GET /search?q=... returns both sources.musicbrainz and sources.deezer, each independently status-tagged, with no cross-source merge/dedupe"
    requirement: "WLST-01"
    verification:
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestSearch_BothSourcesOK"
        status: pass
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestNewDeezerSource_MapsFields"
        status: pass
      - kind: integration
        ref: "manual run: local binary against real Postgres + live api.deezer.com, GET /search?q=drake -- observed 200 with sources.deezer.status:ok carrying 10 real Deezer artists with correctly-mapped id/image_url, while sources.musicbrainz.status:error (this sandbox's outbound TLS to musicbrainz.org fails, same finding as 03-01) with no leaked upstream text in the body"
        status: pass
    human_judgment: false
  - id: D-03
    description: "Either source failing still returns 200 with the other source's results intact; both sources failing returns 200 with both statuses error, never a 5xx"
    requirement: "CLNT-03"
    verification:
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestSearch_DeezerOnlyFailure"
        status: pass
      - kind: unit
        ref: "internal/httpserver/search_test.go#TestSearch_BothSourcesFailed"
        status: pass
      - kind: integration
        ref: "manual run: live GET /search?q=drake with real deezer ok / real musicbrainz error -- observed the D-03 mirror-image shape live, matching the unit tests"
        status: pass
    human_judgment: false

duration: 30min
completed: 2026-08-07
status: complete
---

# Phase 3 Plan 2: Deezer Client and Two-Source Search Summary

**`internal/deezer` (search + albums, int64 ids, HTTP-200 in-body error detection) joined onto `GET /search` as a second independent source, completing WLST-01/CLNT-02/CLNT-03; live-tested against real Deezer with the exact expected id/image_url mapping.**

## Performance

- **Duration:** ~30 min total wall-clock across two sessions (task 1 and part of task 2 landed before a prior executor run stalled mid-task-2; this session verified/completed task 2 and executed task 3 in ~10 min of active work)
- **Started:** 2026-08-07 17:05:56 (task 1)
- **Completed:** 2026-08-07 17:34:16 (task 3)
- **Tasks:** 3 completed
- **Files modified:** 8 (5 created, 3 modified)

## Accomplishments
- `internal/deezer`: a hand-rolled, rate-limited `net/http` client for Deezer's public JSON API, mirroring `internal/musicbrainz`'s shape exactly (single `doRequest` choke point, `cancelReadCloser`, `clampLimit`) but with no User-Agent field (Deezer needs none) and an added `decodeChecked` probe that catches Deezer's HTTP-200-with-in-body-error quota signal before it can silently decode into a zero-valued success.
- `SearchArtists` (`/search/artist`) and `ArtistAlbums` (`/artist/{id}/albums`, D-12) both decode `int64` ids with no float64 precision loss, return non-nil zero-length slices for empty results, preserve upstream order with no client-side sorting, and never merge duplicate-title entries.
- `ArtistAlbums` refuses an empty/whitespace artist id before constructing any URL (`ErrEmptyArtistID`), so a nil `watchlist.Entry.DeezerID` can never produce a doubled-slash request path (D-06 pitfall 3, T-03-10) -- proven by a zero-requests-reached-the-fake-server assertion.
- `GET /search` now fans out to two sources concurrently: `NewDeezerSource` adapts `deezer.ArtistSearcher` into `SearchSource` exactly like the existing MusicBrainz adapter, mapping `Artist.ID` through `strconv.FormatInt` (so a Deezer numeric id is never presented under the `musicbrainz` key, T-03-11) and `Picture` into `ImageURL` only when non-empty.
- `cmd/server/main.go` constructs exactly one `*deezer.Client` and one `rate.Limiter` (sized `DeezerRateLimitPer5s`-per-5-seconds) for the whole process, deliberately separate from the MusicBrainz limiter so neither source's pace throttles the other (D-07, D-08) -- ready for plan 03-04's poller to reuse directly.
- Ran the built binary end-to-end against real Postgres and live `api.deezer.com`: `GET /search?q=drake` returned `sources.deezer.status: "ok"` with 10 correctly-shaped real artists (numeric-string ids, `picture`-derived `image_url`), while `sources.musicbrainz.status: "error"` with no leaked upstream text -- this sandbox's outbound TLS to `musicbrainz.org` fails (same finding as 03-01), independently confirming the D-03 partial-failure contract live rather than only in unit tests.

## Task Commits

Each task was committed atomically, with a RED test commit ahead of its GREEN implementation commit per `tdd="true"`:

1. **Task 1: Deezer client and artist search, with in-body error detection**
   - `eb70fed` test(03-02): add failing test for deezer artist search client
   - `fac8a7b` feat(03-02): add rate-limited deezer artist search client
2. **Task 2: Deezer artist-albums fetch path (D-12)**
   - `9f62039` test(03-02): add failing test for deezer artist-albums fetch
   - `c70982e` feat(03-02): add deezer artist-albums fetch path (D-12)
3. **Task 3: Join Deezer to the /search fan-out and wire it in main**
   - `2f7af22` test(03-02): add failing tests for deezer search adapter and fan-out
   - `e6d2905` feat(03-02): join deezer to the /search fan-out and wire it in main

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/deezer/client.go` - `Client`, `NewClient`, `ArtistSearcher`/`AlbumLister` seams, `APIError`, `decodeChecked` (HTTP-200 in-body error probe), `doRequest`, `cancelReadCloser`, `clampLimit`
- `internal/deezer/search.go` - `Artist`, `SearchArtists` against `/search/artist`, `ErrEmptyQuery`
- `internal/deezer/search_test.go` - httptest.Server-backed fixtures covering decode shape, large-id precision, empty/ordering, quota error, upstream error, Accept header, rate pacing, empty query
- `internal/deezer/albums.go` - `Album`, `ArtistAlbums` against `/artist/{id}/albums`, `ErrEmptyArtistID`
- `internal/deezer/albums_test.go` - fixtures covering decode shape, empty/whitespace id (zero requests), nonexistent artist (empty-not-error), duplicate-title distinctness, quota error, partial release dates
- `internal/httpserver/search.go` - `deezerSource` adapter, `NewDeezerSource`
- `internal/httpserver/search_test.go` - `stubDeezerArtistSearcher` double, `TestNewDeezerSource_MapsFields`, `TestSearch_BothSourcesOK`, `TestSearch_DeezerOnlyFailure`, `TestSearch_BothSourcesFailed`
- `cmd/server/main.go` - constructs the process-wide `deezer.Client`/`rate.Limiter` pair and appends `NewDeezerSource` to the `[]httpserver.SearchSource` literal

## Decisions Made
- Recovered task 2 from a stalled prior executor run rather than redoing it: read `albums.go`/`albums_test.go`/the `client.go` diff in full, cross-checked every line against the plan's `<behavior>`/`<action>`/`<acceptance_criteria>`, ran the acceptance-criteria greps and `go test -short ./...` myself, and only then committed -- preserving the plan's RED-then-GREEN commit convention (test file alone, then `albums.go` + `client.go`'s `AlbumLister` addition together, since the stalled agent had authored both in one uncommitted batch with no natural split point).
- This plan marks WLST-01, CLNT-02, and CLNT-03 complete in REQUIREMENTS.md -- 03-01 deliberately left WLST-01/CLNT-03 unmarked because both descriptions name MusicBrainz AND Deezer; this plan supplies the Deezer half.

## Deviations from Plan

None - plan executed exactly as written. Task 2's implementation, though authored by a prior stalled run, matched the plan's behavior/action/acceptance-criteria specification exactly on inspection and required no fixes.

## Issues Encountered
- **Same sandbox TLS-to-musicbrainz.org limitation documented in 03-01** resurfaced during this plan's live human-check: `sources.musicbrainz` came back `status: "error"` (outbound TLS handshake failure) while `sources.deezer` succeeded against the real `api.deezer.com`. This is environmental, not an application defect -- it independently confirms this plan's D-03 contract (Deezer's success is untouched by MusicBrainz's failure) live, complementing 03-01's unit-test coverage of the same contract.
- **A prior executor run stalled mid-Task-2** (600s watchdog timeout) after finishing its own implementation and tests but before running the plan's acceptance-criteria greps or committing. Recovered per the resume protocol: verified the uncommitted work against the plan spec before trusting it, then completed the normal commit/verify flow.

## User Setup Required

None - no external service configuration required. (Deezer's `/search/artist` and `/artist/{id}/albums` endpoints need no API key.)

## Next Phase Readiness
- `internal/deezer.AlbumLister` seam is ready for plan 03-04's Deezer poll cycle to consume directly, with the process-wide `*deezer.Client`/`rate.Limiter` pair already constructed in `cmd/server/main.go`.
- `GET /search` is feature-complete for this phase's scope: two independent source-tagged lists, D-01/D-02/D-03 all proven both in unit tests and live against real Deezer.
- Full test suite (`go test -short ./... -count=1`) is green; `go build ./...` and `go vet ./...` are clean.
- Known accepted debt (recorded in 03-02-PLAN.md's assumption-delta checkpoint): a Deezer-only search result has no MBID and cannot be added to the watchlist through the Phase 2 add path. Phase 6 (UI-01) must account for this when rendering "add" affordances on Deezer-only results.

---
*Phase: 03-external-clients-search*
*Completed: 2026-08-07*

## Self-Check: PASSED

All 8 claimed files (`internal/deezer/client.go`, `internal/deezer/search.go`, `internal/deezer/search_test.go`, `internal/deezer/albums.go`, `internal/deezer/albums_test.go`, `internal/httpserver/search.go`, `internal/httpserver/search_test.go`, `cmd/server/main.go`) exist on disk. All 6 claimed commit hashes (`eb70fed`, `fac8a7b`, `9f62039`, `c70982e`, `2f7af22`, `e6d2905`) resolve in `git log --oneline --all`.
