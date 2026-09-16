---
phase: 04-detection-engine
plan: 02
subsystem: database
tags: [postgres, sqlc, detection, poller, musicbrainz, deezer, watchlist]

# Dependency graph
requires:
  - phase: 04-detection-engine (plan 01)
    provides: "events table, InsertEvent/ListExternalIDs/HasAnyEvent, internal/detection package (Detector, DetectMusicBrainz), poller.EventRecorder seam wired into RunMusicBrainzCycle"
provides:
  - "internal/detection/filter.go: releaseTypeAllowed/deluxeDetectionEnabled/eventTypeMuted -- D-17/D-18 preference predicates over watchlist.Entry, applied before any insert"
  - "queries/events.sql: ListUnnotified (:many, WHERE notified_at IS NULL) -- Phase 5's own notify-queue query, also this plan's seed-exclusion instrument"
  - "Detector.isSeedMode/seedNotifiedAt -- D-13/D-14/D-15 per-source implicit seed-mode detection, wired into DetectMusicBrainz"
  - "internal/detection/deezer.go: Detector.DetectDeezer -- new_release-only diff against the seen store in Deezer's own numeric-id namespace"
  - "poller.EventRecorder widened with DetectDeezer, wired into RunDeezerCycle's per-artist loop"
affects: [04-03-deluxe-change-baseline, 05-discord-notifications, 06-frontend-release-history]

# Actuals (#2632)
actuals:
  tokens: 15510
  tasks: 3
  commits: 5

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Preference predicates (filter.go) are small, pure, doc-commented functions over an already-fetched watchlist.Entry -- no database access -- mirroring watchlist.normalizeSet's shape; detectableReleaseTypes is deliberately narrower than watchlist.ReleaseTypes since 'deluxe' is a boolean gate, not a MusicBrainz primary-type value"
    - "Seed mode is derived implicitly from HasAnyEvent (zero existing rows for an artist_id+source pair), decided once per Detect* call before the loop, with the resulting notified_at threaded into every insertEvent call so one seed cycle's rows share an identical timestamp"
    - "DetectDeezer structurally mirrors DetectMusicBrainz (mute check, seed-mode check, seen-set lookup, per-item filter, insert) rather than sharing a generic helper -- the two sources differ enough in id formatting and filter input (RecordType vs PrimaryType) that a shared abstraction would obscure more than it saves"

key-files:
  created:
    - internal/detection/filter.go
    - internal/detection/filter_test.go
    - internal/detection/deezer.go
    - internal/detection/deezer_test.go
  modified:
    - internal/detection/detector.go
    - internal/detection/detector_test.go
    - internal/detection/musicbrainz.go
    - internal/poller/poller.go
    - internal/poller/poller_test.go
    - queries/events.sql
    - internal/db/sqlc/events.sql.go
    - internal/db/sqlc/querier.go

key-decisions:
  - "Existing 04-01 test fixtures (detector_test.go, poller_test.go) updated to carry ReleaseTypes on watchlist.Entry and PrimaryType on musicbrainz.ReleaseGroup, since Task 1's filter now rejects an entry with no ReleaseTypes and a group with no primary-type -- a real watchlist entry always has ReleaseTypes populated per Phase 2's D-08 defaults, so the old bare fixtures no longer represented reachable production state"
  - "The two real-Postgres integration tests specified for Task 1 (TestDetectMusicBrainz_FiltersByReleaseType, TestDetectMusicBrainz_SkipsMutedEventType) live in filter.go's own whitebox test file rather than detector_test.go, matching the task's stated file list -- they duplicate detector_test.go's small fixture-helper convention locally (filterTestMBID/filterTestLogger/insertFilterTestArtist) rather than importing across the internal/external test package split"

patterns-established:
  - "unnotifiedForArtist test helper filters ListUnnotified's global (cross-artist) result down to one artist_id -- ListUnnotified has no per-artist parameter by design (it's Phase 5's cross-artist notify queue), and since no test in this package uses t.Parallel, each test's own artist's rows are the only ones present in the database at the point it queries"

requirements-completed: [DTCT-01, DTCT-04]

coverage:
  - id: D1
    description: "A release-group whose primary-type or an artist's muted_event_types excludes it never becomes an event row -- both preference axes are checked before any insert (D-17, D-18)"
    requirement: "WLST-05"
    verification:
      - kind: unit
        ref: "internal/detection/filter_test.go#TestFilter_ReleaseTypeAllowed"
        status: pass
      - kind: unit
        ref: "internal/detection/filter_test.go#TestFilter_DeluxeIsAGateNotAType"
        status: pass
      - kind: unit
        ref: "internal/detection/filter_test.go#TestFilter_EventTypeMuted"
        status: pass
      - kind: integration
        ref: "internal/detection/filter_test.go#TestDetectMusicBrainz_FiltersByReleaseType"
        status: pass
      - kind: integration
        ref: "internal/detection/filter_test.go#TestDetectMusicBrainz_SkipsMutedEventType"
        status: pass
    human_judgment: false
  - id: D2
    description: "A newly watched artist's existing catalogue is recorded with notified_at pre-set (excluded from ListUnnotified); later cycles leave new events unnotified; seeding is per-source; a watchlist remove-then-re-add resumes rather than re-seeds"
    requirement: "DTCT-01"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetector_SeedMode_FirstCyclePreNotifies"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetector_SecondCycleLeavesNotifiedAtNull"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetector_SeedModeIsPerSource"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetector_ReAddDoesNotReSeed"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetector_SeedModeRespectsFilters"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetector_SeedRowsShareOneTimestamp"
        status: pass
    human_judgment: false
  - id: D3
    description: "The Deezer poll cycle records previously-unseen albums as new_release events in Deezer's own id namespace, honoring both preference axes and per-source seeding, and produces no guest_feature or deluxe_change events"
    requirement: "DTCT-04"
    verification:
      - kind: integration
        ref: "internal/detection/deezer_test.go#TestDetectDeezer_NewRelease"
        status: pass
      - kind: integration
        ref: "internal/detection/deezer_test.go#TestDetectDeezer_ReDetectionInsertsNothing"
        status: pass
      - kind: integration
        ref: "internal/detection/deezer_test.go#TestDetectDeezer_FiltersByRecordType"
        status: pass
      - kind: integration
        ref: "internal/detection/deezer_test.go#TestDetectDeezer_SeedsIndependentlyOfMusicBrainz"
        status: pass
      - kind: integration
        ref: "internal/detection/deezer_test.go#TestDetectDeezer_SameIDDifferentSourceCoexist"
        status: pass
      - kind: integration
        ref: "internal/poller/poller_test.go#TestPoller_RunDeezerCycle_RecordsNewRelease"
        status: pass
      - kind: integration
        ref: "internal/poller/poller_test.go#TestPoller_RunDeezerCycle_SkipsNilDeezerID"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-07
status: complete
---

# Phase 4 Plan 2: Preference Filtering, Seed Mode, and the Deezer Detection Slice Summary

**Both watchlist preference axes now gate event creation before any insert, a newly watched artist's existing catalogue is absorbed into the seen store as already-notified per source, and the Deezer poll cycle independently records new_release events in its own id namespace.**

## Performance

- **Duration:** ~25 min (coding + verification; excludes context-gathering read time)
- **Tasks:** 3 (Task 1: preference filtering; Task 2: seed mode, TDD RED/GREEN; Task 3: Deezer detection slice, TDD RED/GREEN)
- **Files modified:** 13 (5 created, 8 modified)

## Accomplishments

- `internal/detection/filter.go`: `releaseTypeAllowed`, `deluxeDetectionEnabled`, `eventTypeMuted` -- pure predicates applying D-17 (release-type filter, with `deluxe` handled as a boolean gate rather than a MusicBrainz type match, per 04-RESEARCH.md Pitfall #7) and D-18 (mute axis) before any event row is created. Wired into `DetectMusicBrainz` with a `filtered_count`/`seed_mode` log attribute pair so a silently-suppressed preference or an unexpected seed is visible in structured logs.
- `queries/events.sql`'s `ListUnnotified` (`:many`, `WHERE notified_at IS NULL ORDER BY created_at ASC, id ASC`) -- D-11's Phase 5 groundwork, and the instrument this plan's own tests use to prove seeded rows are excluded.
- `Detector.isSeedMode`/`seedNotifiedAt`: D-14's implicit per-source seed detection (zero existing event rows for an `(artist_id, source)` pair means seed mode), decided once per `Detect*` call so every row from one seed cycle shares an identical `notified_at` timestamp (D-13). Seeding is scoped per source (D-15) and survives a watchlist remove-then-re-add because it's derived purely from the `events` table, keyed on `artist_id` (D-16).
- `internal/detection/deezer.go`: `Detector.DetectDeezer` -- the same mute/seed/seen-set/filter/insert structure as `DetectMusicBrainz`, producing `new_release` events only (Deezer has no track/credit-level fetch for guest-feature or deluxe-change, D-03/D-08). `external_id` is `strconv.FormatInt` from the decoded album id (T-04-08); `release_group_mbid` is always `nil`.
- `poller.EventRecorder` widened with `DetectDeezer`, wired into `RunDeezerCycle`'s per-artist loop with the same log-and-continue error isolation `RunMusicBrainzCycle` uses; the nil-`DeezerID` skip is unchanged (no fetch, no recorder call, no row).

## Task Commits

1. **Task 1: Gate event creation on the artist's two preference axes** -- `0f53ba4` (feat)
2. **Task 2: Absorb a newly watched artist's existing catalogue -- RED** -- `58cc38a` (test)
3. **Task 2: Absorb a newly watched artist's existing catalogue -- GREEN** -- `ad5a3c3` (feat)
4. **Task 3: Extend detection to the Deezer poll cycle -- RED** -- `7c5235c` (test)
5. **Task 3: Extend detection to the Deezer poll cycle -- GREEN** -- `da345b8` (feat)

## Files Created/Modified

- `internal/detection/filter.go` -- `releaseTypeAllowed`/`deluxeDetectionEnabled`/`eventTypeMuted`, D-17/D-18 predicates
- `internal/detection/filter_test.go` -- table-driven predicate tests plus two real-Postgres integration assertions on the MusicBrainz path
- `internal/detection/musicbrainz.go` -- wires the mute check, release-type filter, seed-mode decision and `notifiedAt` threading into `DetectMusicBrainz`
- `internal/detection/detector.go` -- `isSeedMode`, `seedNotifiedAt`
- `internal/detection/detector_test.go` -- six new seed-mode tests plus updated fixtures (`ReleaseTypes`/`PrimaryType`) on every pre-existing `DetectMusicBrainz` test
- `internal/detection/deezer.go` -- `Detector.DetectDeezer`
- `internal/detection/deezer_test.go` -- real-Postgres Deezer detection tests
- `internal/poller/poller.go` -- widened `EventRecorder`, `DetectDeezer` call in `RunDeezerCycle`
- `internal/poller/poller_test.go` -- `fakeEventRecorder.DetectDeezer`, two new integration tests, one updated fixture
- `queries/events.sql`, `internal/db/sqlc/events.sql.go`, `internal/db/sqlc/querier.go` -- `ListUnnotified`

## Decisions Made

- Existing 04-01 test fixtures now carry `ReleaseTypes`/`PrimaryType` since Task 1's filter is a genuine behavior change: an entry with no `ReleaseTypes` allows nothing, and a real watchlist entry always has `ReleaseTypes` populated (Phase 2 D-08 defaults), so the old bare fixtures no longer represented reachable production state.
- Task 1's two real-Postgres integration tests live in `filter_test.go` (matching the task's own file list) rather than `detector_test.go`, duplicating a small local fixture-helper set instead of crossing the `detection`/`detection_test` package split.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated pre-existing `DetectMusicBrainz` test fixtures to carry `ReleaseTypes`/`PrimaryType`**
- **Found during:** Task 1 (wiring the preference filter into `DetectMusicBrainz`)
- **Issue:** Every 04-01-era `DetectMusicBrainz` test built a `watchlist.Entry` with no `ReleaseTypes` and a `musicbrainz.ReleaseGroup` with no `PrimaryType`. Once the D-17 filter is wired in, an empty `ReleaseTypes` allows nothing, so every one of those tests would drop from its expected row count to zero.
- **Fix:** Added `ReleaseTypes: []string{"album", "single", "ep", "deluxe"}` to each entry fixture and `PrimaryType: "Album"` to each group fixture across `detector_test.go` and `poller_test.go`'s `TestPoller_RunMusicBrainzCycle_RecordsNewRelease`.
- **Files modified:** `internal/detection/detector_test.go`, `internal/poller/poller_test.go`
- **Verification:** `TEST_DATABASE_URL=... go test ./... -count=1` passes across every package
- **Committed in:** `0f53ba4` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary correctness fix directly caused by Task 1's own filter change; no scope creep, no behavior outside the plan's stated scope.

## Issues Encountered

One transient test failure (`TestDetectDeezer_ReDetectionInsertsNothing` reporting 0 rows instead of 2) appeared once during iterative verification and did not reproduce across three subsequent full-package runs, in isolation, or in the final full-suite run. No code change was needed; treated as a one-off environment flake rather than a defect.

## User Setup Required

None -- no external service configuration required. This plan installs zero new dependencies (`go mod tidy` leaves `go.mod`/`go.sum` unchanged).

## Next Phase Readiness

- All three `new_release` detection paths (MusicBrainz new-release, MusicBrainz seed mode, Deezer new-release) are complete and both preference axes apply uniformly across sources.
- `deluxeDetectionEnabled` is implemented and tested but not yet consumed -- plan 04-04 (deluxe-change baseline) is the first to call it, per 04-01's checkpoint resolution (mutable `track_count` column, option-a).
- `ListUnnotified` is implemented, tested indirectly (seed-mode tests prove it excludes seeded rows), and ready for Phase 5's notify job with no schema or query changes needed.
- No blockers.

---
*Phase: 04-detection-engine*
*Completed: 2026-08-07*

## Self-Check: PASSED

All 5 created files verified present on disk; all 5 commits (`0f53ba4`, `58cc38a`, `ad5a3c3`, `7c5235c`, `da345b8`) verified present in `git log`.
