---
phase: 03-external-clients-search
plan: 03
subsystem: api
tags: [musicbrainz, net-http, rate-limiting, pagination]

# Dependency graph
requires:
  - phase: 03-external-clients-search
    provides: "03-01's internal/musicbrainz Client, doRequest limiter-and-header choke point, ArtistSearcher seam, and NewClient/newTestClient test helpers this plan extends"
provides:
  - "internal/musicbrainz.ReleaseGroupsByArtist: bounded, limiter-paced, sequential pagination over a watchlisted artist's release-groups (CLNT-01, D-10)"
  - "internal/musicbrainz.ReleaseGroupLister: the narrow seam plan 03-04's poller depends on"
affects: [03-04-scheduler]

# Actuals (#2632)
actuals:
  tokens: 6942
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "fetchReleaseGroupPage: an unexported per-offset request helper wrapped by ReleaseGroupsByArtist's pagination loop, so the loop and the single request-and-decode step stay independently testable and task 2 could extend the loop without rewriting the request path"
    - "offset advances by entries-actually-returned, not requested page size, so a short page can never skip records"
    - "page-count ceiling (maxReleaseGroupPages) returns accumulated data with a nil error on hit, rather than an error -- a truncated fetch is a data-completeness limit, not a failure"

key-files:
  created:
    - internal/musicbrainz/releasegroups.go
    - internal/musicbrainz/releasegroups_test.go
  modified:
    - internal/musicbrainz/client.go

key-decisions:
  - "ReleaseGroupsByArtist sends no `type` filter -- Phase 4's per-artist release-type preferences (Phase 2 WLST-05) are applied at detection time, so filtering at fetch time would make a preference change silently invisible until the next full refetch (per plan's explicit instruction)"
  - "maxReleaseGroupPages fixed at 10 (1000 release-groups per artist per poll cycle at releaseGroupPageSize=100) -- bounds a hostile/malformed release-group-count from ever driving an unbounded request loop against a free public API (T-03-12)"
  - "Hitting the page ceiling returns the accumulated slice and a nil error, not an error -- one prolific artist's truncated fetch must never abort the whole poll cycle"

patterns-established:
  - "Pattern: bounded pagination loop over an upstream-reported count, terminating on count-reached OR empty-page OR a fixed page ceiling, with every page issued sequentially through the same rate-limited request helper -- reusable for internal/deezer's own paginated endpoints if a future plan needs them"

requirements-completed: [CLNT-01]

coverage:
  - id: D1
    description: "ReleaseGroupsByArtist fetches a watchlisted artist's release-groups from /ws/2/release-group?artist=, through the same limiter-gated, User-Agent-carrying request path as artist search"
    requirement: "CLNT-01"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/releasegroups_test.go#TestReleaseGroupsByArtist_DecodesFixture"
        status: pass
      - kind: unit
        ref: "internal/musicbrainz/releasegroups_test.go#TestReleaseGroupsByArtist_RequestShape"
        status: pass
    human_judgment: false
  - id: D2
    description: "Pagination is bounded, sequential, and paced; a runaway release-group-count cannot drive an unbounded request loop"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/releasegroups_test.go#TestReleaseGroupsByArtist_PaginationCollectsAllPagesInOrder"
        status: pass
      - kind: unit
        ref: "internal/musicbrainz/releasegroups_test.go#TestReleaseGroupsByArtist_PageCapStopsRunawayCount"
        status: pass
      - kind: unit
        ref: "internal/musicbrainz/releasegroups_test.go#TestReleaseGroupsByArtist_PacingAppliesAcrossPages"
        status: pass
      - kind: unit
        ref: "internal/musicbrainz/releasegroups_test.go#TestReleaseGroupsByArtist_ZeroEntryPageTerminatesLoop"
        status: pass
    human_judgment: false
  - id: D3
    description: "No retry loop exists anywhere in this file; a mid-fetch error or cancellation stops immediately"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/releasegroups_test.go#TestReleaseGroupsByArtist_MidFetchErrorStopsWithNoRetry"
        status: pass
      - kind: unit
        ref: "internal/musicbrainz/releasegroups_test.go#TestReleaseGroupsByArtist_CancellationBetweenPagesAborts"
        status: pass
    human_judgment: false

duration: 30min
completed: 2026-08-07
status: complete
---

# Phase 3 Plan 3: MusicBrainz Release-Groups Browse-by-Artist Summary

**`ReleaseGroupsByArtist` added to the MusicBrainz client: bounded, sequential, rate-limiter-paced pagination over `/ws/2/release-group`, with dates kept as opaque strings and no retry logic anywhere.**

## Performance

- **Duration:** ~30 min
- **Tasks:** 2 completed
- **Files modified:** 3

## Accomplishments
- `internal/musicbrainz/releasegroups.go`: `ReleaseGroup` typed struct (verbatim `FirstReleaseDate` string, never parsed into `time.Time`), the unexported `releaseGroupEnvelope`, sentinel `ErrEmptyMBID`, and `ReleaseGroupsByArtist(ctx, mbid)` -- browses every release-group for an artist through the same `doRequest` limiter-and-User-Agent choke point `SearchArtists` already uses.
- Pagination advances by entries-actually-returned (not requested page size), terminates on reaching `release-group-count`, an empty page, or the `maxReleaseGroupPages` (10) ceiling -- a malformed or hostile count from MusicBrainz can no longer drive an unbounded request loop (T-03-12). Hitting the ceiling returns accumulated data with a nil error, never an error, so one prolific artist can't abort a whole poll cycle.
- Pages are fetched strictly sequentially (`grep -c 'go func'` = 0) with no retry logic anywhere in the file (`grep -Ec 'for .*(retry|attempt)'` = 0), so a multi-page fetch can never exceed the operator-configured rate or amplify a 503 into a retry storm.
- `ReleaseGroupLister` seam added to `client.go` with the compile-time assertion, ready for plan 03-04's poller to depend on instead of `*Client` directly.

## Task Commits

RED test ahead of two GREEN implementation commits (per `tdd="true"`, split so task 1's single-page fetch and task 2's pagination loop each landed as their own verifiable step):

1. **Task 1 + Task 2: RED tests for the full behavior surface**
   - `3b3b263` test(03-03): add failing tests for release-groups browse-by-artist
2. **Task 1: single-page fetch (D-10)**
   - `5d851fa` feat(03-03): add release-groups browse-by-artist to the musicbrainz client
3. **Task 2: bounded, paced pagination**
   - `82f4af2` feat(03-03): bound and pace release-group pagination across pages (T-03-12/14)

## Files Created/Modified
- `internal/musicbrainz/releasegroups.go` - `ReleaseGroup`, `ReleaseGroupsByArtist`, unexported `fetchReleaseGroupPage`/`releaseGroupEnvelope`, `ErrEmptyMBID`, `maxReleaseGroupPages`/`releaseGroupPageSize` consts
- `internal/musicbrainz/releasegroups_test.go` - httptest.Server-backed fixtures covering decode shape, request shape, no-type-filter, duplicate-title/partial-date preservation, empty result, empty MBID, upstream error/no-retry, cancellation, multi-page collection, pacing, count-reached termination, page-cap termination, zero-entry termination, mid-fetch error, cancellation between pages, and a direct `rate.Limiter` fractional-rate assertion
- `internal/musicbrainz/client.go` - `ReleaseGroupLister` interface + `var _ ReleaseGroupLister = (*Client)(nil)`

## Decisions Made
- Both TDD tasks' RED tests were written and committed together in one `test(03-03)` commit (drafting happened in one pass, as in 03-02), but each task's GREEN implementation still landed in its own separate `feat` commit -- task 1's commit implements only the single-page fetch (task 2's pagination tests fail against it, confirmed before committing), task 2's commit extends it into the bounded pagination loop.
- `releaseGroupFixture`'s `release-group-count` was corrected from the live-verified `61` to `1` for the single-page tests (`DecodesFixture`, `RequestShape`, etc.): the fixture's static single-item body never changes across repeated requests, so once real pagination existed (task 2), a `count` of 61 with only 1 entry per page kept the loop re-fetching the same page until hitting the page ceiling instead of stopping after one request. See Deviations below.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] releaseGroupFixture's count/entries mismatch broke single-page tests once pagination landed**
- **Found during:** Task 2, running `go test ./internal/musicbrainz/...` after implementing the pagination loop
- **Issue:** `releaseGroupFixture` (used by `DecodesFixture`, `RequestShape`, `NoTypeFilterSent`, `EmptyMBID`) carried the live-verified `"release-group-count": 61` alongside a fixture body containing only 1 release-group. Task 1's single-page-only implementation never noticed this mismatch. Once task 2's real pagination loop compared `len(groups)` against `env.Count`, it kept re-requesting the same static page (always 1 entry, count still 61) until hitting `maxReleaseGroupPages`, producing 10 accumulated entries and a request offset of 9 instead of the expected single request at offset 0.
- **Fix:** Changed `releaseGroupFixture`'s `release-group-count` from `61` to `1`, matching the fixture's actual single-entry body. The multi-page pagination behavior is separately and correctly exercised by `pagedReleaseGroupServer`, which computes a real per-offset response server-side rather than serving a static body.
- **Files modified:** `internal/musicbrainz/releasegroups_test.go`
- **Verification:** `go test ./internal/musicbrainz/... -short -count=1` green after the fix; `go test -short ./... -count=1` green.
- **Committed in:** `82f4af2` (task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 bug, test-only)
**Impact on plan:** Test-fixture correctness fix surfaced by the plan's own required full-suite verification step before committing; no production-code scope creep.

## Issues Encountered
None beyond the fixture bug documented above. This plan touches only `internal/musicbrainz/*`, files untouched by plan 03-02 (which landed just before this plan started), so there was no merge or conflict risk.

## User Setup Required
None - no external service configuration required. (MusicBrainz's `/ws/2/release-group` endpoint needs no API key.)

## Next Phase Readiness
- `ReleaseGroupLister` is ready for plan 03-04's poller to depend on directly, matching D-10's "browse release-groups only" scope for Phase 3 -- recordings-by-artist-credit (guest-feature detection) stays deferred to Phase 4 as planned.
- Full test suite (`go test -short ./...`) is green; `go build ./...` and `go vet ./...` are clean. This plan added files only inside `internal/musicbrainz` and did not affect any other package (confirmed by the full-suite run).
- Live-network verification against real `musicbrainz.org` was not attempted in this sandbox (per 03-01-SUMMARY.md's documented "no outbound TLS egress" environment constraint) -- all coverage here is `httptest.Server`-backed per CLAUDE.md's testing constraint, which is the required and sufficient bar for this plan's `<verify>` section (it does not call for a live check).

---
*Phase: 03-external-clients-search*
*Completed: 2026-08-07*

## Self-Check: PASSED

All 3 claimed files (`internal/musicbrainz/releasegroups.go`, `internal/musicbrainz/releasegroups_test.go`, `internal/musicbrainz/client.go`) exist on disk. All 3 claimed commit hashes (`3b3b263`, `5d851fa`, `82f4af2`) resolve in `git log --oneline --all`.
