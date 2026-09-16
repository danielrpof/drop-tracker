---
phase: 04-detection-engine
plan: 03
subsystem: detection
tags: [musicbrainz, guest-feature, pagination, slog, sqlc, postgres]

# Dependency graph
requires:
  - phase: 04-detection-engine (plan 02)
    provides: "filter.go's eventTypeMuted/releaseTypeAllowed predicates, isSeedMode/seedNotifiedAt, DetectMusicBrainz's new_release pass this plan extends"
provides:
  - "internal/musicbrainz/recordings.go: RecordingsByArtist -- bounded-pagination browse of every recording an artist is credited on (D-05), mirroring ReleaseGroupsByArtist"
  - "internal/detection.RecordingSource seam + widened New(q, recordings) -- DetectMusicBrainz's guest-feature dependency, declared in the consumer per this codebase's narrow-interface convention"
  - "internal/detection/musicbrainz.go: isGuestFeature/displayArtistName/detectGuestFeatures -- D-06's positional guest-feature rule wired into the same MusicBrainz poll cycle as new_release (D-07), with page_ceiling_reached truncation visibility"
  - "seedMode/notifiedAt now computed once at the top of DetectMusicBrainz and shared across both the new_release and guest_feature passes -- a newly-watched artist's whole musicbrainz snapshot seeds under one timestamp"
affects: [05-discord-notifications, 06-frontend-release-history]

# Actuals (#2632)
actuals:
  tokens: 15140
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "detectGuestFeatures receives seedMode/notifiedAt as parameters rather than recomputing isSeedMode itself -- two event types now share one (artist_id, source) seed-mode scope, and a second independent isSeedMode call inside the same DetectMusicBrainz invocation would see the first pass's freshly-inserted rows and read a flipped answer mid-call"
    - "A page_ceiling_reached boolean (len(recordings) >= musicbrainz.MaxRecordingBrowseItems) rides on every guest_feature 'detection result' log line, mirroring the truncation-visibility convention releasegroups.go already established for new_release"
    - "ArtistCreditEntry.Artist is an anonymous nested struct (per 04-RESEARCH.md Pattern 3); test fixtures build it field-by-field via a small mkCredit/creditFor helper rather than duplicating the anonymous struct's exact field/tag shape at every call site"

key-files:
  created:
    - internal/musicbrainz/recordings.go
    - internal/musicbrainz/recordings_test.go
    - internal/detection/musicbrainz_test.go
  modified:
    - internal/detection/detector.go
    - internal/detection/musicbrainz.go
    - internal/detection/detector_test.go
    - internal/detection/deezer_test.go
    - internal/detection/filter_test.go
    - internal/poller/poller_test.go
    - cmd/server/main.go

key-decisions:
  - "seedMode/notifiedAt moved to a single computation at the top of DetectMusicBrainz, shared by both the new_release and guest_feature passes, rather than each pass independently calling isSeedMode -- avoids a genuine bug where the guest-feature pass's own isSeedMode call would see rows the new_release pass had just inserted in the same call and read seed mode as already-over, leaving a newly-watched artist's guest-feature catalogue unseeded on their very first cycle"
  - "isGuestFeature/displayArtistName unit tests live in a new internal/detection/musicbrainz_test.go (package detection, whitebox) rather than detector_test.go (package detection_test) -- isGuestFeature is unexported, so it can only be tested from within the package, mirroring filter_test.go's existing convention for testing unexported predicates"
  - "Both tasks' RED-phase tests were written and committed together (one test commit covering both), matching 04-02's own documented precedent for combining RED phases across tightly-coupled tasks; each task's behavior (basic guest detection plus truncation/dedup/malformed-credit hardening) was implemented as one inseparable GREEN commit since the two tasks build one feature slice end-to-end"

requirements-completed: [DTCT-03]

coverage:
  - id: D1
    description: "A recording where the watched artist is credited but not first in the artist-credit list produces exactly one guest_feature event, with NULL release_group_mbid/release_date/cover_art_url and artist_name set to the primary credit's artist"
    requirement: "DTCT-03"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature"
        status: pass
      - kind: unit
        ref: "internal/detection/musicbrainz_test.go#TestIsGuestFeature_Positional"
        status: pass
    human_judgment: false
  - id: D2
    description: "A recording where the watched artist is the first (primary) artist-credit entry never becomes a guest_feature event, even though RecordingsByArtist returns it (the endpoint is not pre-filtered to guests)"
    requirement: "DTCT-03"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_SkipsOwnPrimaryCredit"
        status: pass
    human_judgment: false
  - id: D3
    description: "A nil or empty artist-credit array is skipped without panicking; an empty first-credit artist id is treated as a guest (errs toward an extra alert, not a silently missed feature)"
    requirement: "DTCT-03"
    verification:
      - kind: unit
        ref: "internal/detection/musicbrainz_test.go#TestIsGuestFeature_EmptyCredit"
        status: pass
      - kind: unit
        ref: "internal/detection/musicbrainz_test.go#TestIsGuestFeature_MissingArtistID"
        status: pass
    human_judgment: false
  - id: D4
    description: "RecordingsByArtist's bounded pagination (maxRecordingPages=10 x recordingPageSize=100) stops at the page ceiling for a runaway/hostile recording-count, and a fetch at that ceiling is visible as page_ceiling_reached=true in the guest_feature detection-result log line"
    requirement: "DTCT-03"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/recordings_test.go#TestRecordingsByArtist_StopsAtPageCeiling"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_LogsTruncation"
        status: pass
    human_judgment: false
  - id: D5
    description: "The same recording MBID appearing twice within one browse result dedups to exactly one row; a muted guest_feature preference blocks only guest rows while new_release rows still land; a recording-source error is logged and swallowed, leaving the same cycle's new_release rows intact and DetectMusicBrainz returning nil"
    requirement: "DTCT-03"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_DedupesRepeatedMBID"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_Muted"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_SourceErrorPreservesNewReleases"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every RecordingsByArtist page request routes through the shared c.doRequest helper (limiter wait + User-Agent header) -- no direct transport reference exists in recordings.go"
    requirement: "DTCT-03"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/recordings_test.go#TestRecordingsByArtist_RequestShape"
        status: pass
      - kind: other
        ref: "grep -q 'httpClient' internal/musicbrainz/recordings.go (exits non-zero)"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-08-08
status: complete
---

# Phase 4 Plan 3: Guest-Feature Detection Summary

**MusicBrainz recording-by-artist-credit browse (D-05) plus a positional guest-feature filter (D-06), wired into the same DetectMusicBrainz poll cycle as new_release with a shared seed-mode/timestamp decision and truncation-visible structured logging.**

## Performance

- **Duration:** ~45 min (coding + verification; excludes context-gathering read time)
- **Tasks:** 2 (Task 1: end-to-end browse/filter/insert slice; Task 2: hardening against over-detection, truncation blindness, and malformed credits)
- **Files modified:** 10 (3 created, 7 modified)

## Accomplishments

- `internal/musicbrainz/recordings.go`: `RecordingsByArtist` -- bounded-pagination browse (`maxRecordingPages=10` x `recordingPageSize=100`, D-05) of every recording MusicBrainz has linked to an artist, in any credit position, structurally mirroring `ReleaseGroupsByArtist`. Exports `MaxRecordingBrowseItems` so a caller can detect truncation without reaching into unexported constants.
- `internal/detection.RecordingSource`: the narrow seam `DetectMusicBrainz`'s guest-feature pass depends on, declared in the consumer per this codebase's established pattern; `New` widens to `New(q sqlc.Querier, recordings RecordingSource) *Detector`, with `mbClient` supplying it in `cmd/server/main.go` (construction moved below `mbClient`'s own).
- `internal/detection/musicbrainz.go`: `isGuestFeature` implements D-06's positional rule with a load-bearing length guard against a malformed/empty artist-credit array; `displayArtistName` picks the primary credit's artist name for the `artist_name` column; `detectGuestFeatures` diffs the fetch against the seen store and inserts `guest_feature` rows with `release_group_mbid`/`release_date`/`cover_art_url` all NULL (D-05 locks the browse to `inc=artist-credits`, which carries none of those fields).
- `DetectMusicBrainz` restructured so `seedMode`/`notifiedAt` are computed exactly once, before either the `new_release` or `guest_feature` pass runs, and threaded into both -- closes a real bug where a second independent `isSeedMode` call for guest features would have seen the new_release pass's own just-inserted rows and read seed mode as already over, leaving a newly-watched artist's entire guest-feature catalogue unseeded (and therefore queued for alerting) on their very first poll cycle.
- The `guest_feature` "detection result" log line carries `recording_count` and `page_ceiling_reached` (`len(recordings) >= musicbrainz.MaxRecordingBrowseItems`), so a truncated fetch for a prolific frequent-collaborator artist is visible in structured logs rather than silently under-detected.
- A recording-source error is logged and the guest pass returns `nil` without propagating, so a failed recording browse never discards the same cycle's already-recorded `new_release` events.

## Task Commits

1. **Task 1 + 2: RED -- failing tests for guest-feature browse, detection, and hardening** - `0050af3` (test)
2. **Task 1 + 2: GREEN -- recording browse, positional filter, and detection wiring** - `b65d4e7` (feat)

**Plan metadata:** pending (this commit)

_Note: both tasks' TDD RED phases were combined into one test commit (matching 04-02's own precedent for combining RED across tightly-coupled tasks); the GREEN implementation for both tasks landed together since the truncation/dedup/malformed-credit hardening was inseparable from the initial slice as actually implemented -- see Deviations below._

## Files Created/Modified

- `internal/musicbrainz/recordings.go` -- `RecordingsByArtist`, `ArtistCreditEntry`, `Recording`, `recordingEnvelope`, `MaxRecordingBrowseItems`
- `internal/musicbrainz/recordings_test.go` -- `httptest.Server` fixture coverage: envelope decode, request shape, pagination, empty MBID, non-200 status, page-ceiling truncation
- `internal/detection/musicbrainz_test.go` -- whitebox unit tests for `isGuestFeature`'s edge cases (nil/empty credit, missing artist id, positional)
- `internal/detection/detector.go` -- `RecordingSource` interface, `Detector.recordings` field, widened `New`
- `internal/detection/musicbrainz.go` -- `isGuestFeature`, `displayArtistName`, `detectGuestFeatures`, restructured `DetectMusicBrainz`
- `internal/detection/detector_test.go` -- `fakeRecordingSource`/`mkCredit` helpers, 6 new guest-feature tests, updated pre-existing `New(...)` call sites
- `internal/detection/deezer_test.go`, `internal/detection/filter_test.go` -- updated pre-existing `New(...)`/`detection.New(...)` call sites for the widened signature
- `internal/poller/poller_test.go` -- local `fakeRecordingSource` double, updated the two real-Postgres integration tests' `detection.New(...)` calls
- `cmd/server/main.go` -- `detector := detection.New(sqlc.New(pool), mbClient)`, construction moved below `mbClient`'s

## Decisions Made

- `seedMode`/`notifiedAt` centralized to one computation shared by both event-type passes inside `DetectMusicBrainz` -- see key-decisions above for the bug this closes.
- `isGuestFeature`/`displayArtistName` tests placed in a new whitebox `internal/detection/musicbrainz_test.go` rather than the plan's literal `detector_test.go` file list, since `isGuestFeature` is unexported and untestable from `detector_test.go`'s external `detection_test` package -- mirrors `filter_test.go`'s existing convention for unexported predicates.
- Both tasks' RED-phase tests committed together and both tasks' GREEN implementation committed together (2 commits total, not 4) -- the plan's two tasks build one inseparable feature slice; splitting the already-unified implementation after the fact into an artificial task-1-only/task-2-only pair would not reflect how the code was actually developed and risked introducing a regression while unwinding it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Centralized seed-mode computation across new_release and guest_feature passes**
- **Found during:** Task 1 (wiring `detectGuestFeatures` into `DetectMusicBrainz`)
- **Issue:** The plan's action text described `detectGuestFeatures` calling `d.isSeedMode` independently. Since `isSeedMode` checks "any existing event row for (artist_id, source)" without an event-type filter, a guest-feature pass that computed its own seed-mode after the new_release pass had already inserted rows in the same call would see those fresh rows and read seed mode as already over -- silently unseeding a newly-watched artist's entire guest-feature catalogue on their very first cycle (turning what should be a pre-notified seed event into a queued alert).
- **Fix:** Moved the `isSeedMode` call to the top of `DetectMusicBrainz`, before either pass runs, and threaded the resulting `seedMode`/`notifiedAt` into `detectGuestFeatures` as parameters instead of it recomputing them.
- **Files modified:** `internal/detection/musicbrainz.go`
- **Verification:** `TestDetector_SeedRowsShareOneTimestamp` (pre-existing, still passes) plus the new guest-feature tests confirm both event types insert successfully under one shared seed decision.
- **Committed in:** `b65d4e7` (feat commit)

**2. [Rule 3 - Blocking] Updated every pre-existing `detection.New(...)` call site for the widened signature**
- **Found during:** Task 1 (widening `New` to accept a `RecordingSource`)
- **Issue:** `detection.New`'s signature change broke compilation of `internal/detection/deezer_test.go`, `internal/detection/filter_test.go`, `internal/poller/poller_test.go`, and `cmd/server/main.go` -- none of which are in this plan's declared `files_modified` list, but all of which are required for `go vet ./...` and `go test ./...` to pass module-wide.
- **Fix:** Added a local no-op `RecordingSource` double (`fakeRecordingSource`/`noRecordingSource`) to each affected test package and updated every call site; `cmd/server/main.go` now passes the real `mbClient`.
- **Files modified:** `internal/detection/deezer_test.go`, `internal/detection/filter_test.go`, `internal/poller/poller_test.go`, `cmd/server/main.go`
- **Verification:** `go build ./... && go vet ./...` clean; `go test ./... -short -count=1` and `TEST_DATABASE_URL=... go test ./... -count=1` both green.
- **Committed in:** `b65d4e7` (feat commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both fixes were necessary for correctness (the seed-mode bug) and for the build to compile at all (the widened-signature call sites). No scope creep -- neither fix touches functionality outside DTCT-03's guest-feature slice.

## Issues Encountered

None -- MusicBrainz's recording-browse envelope shape (04-RESEARCH.md Assumption A2) remains `[ASSUMED]`/unverified against a live response; this session did not attempt a live MusicBrainz call (see PROJECT.md's documented WSL2 TLS limitation and the broader March-2026 bot-blocking trend noted in 04-RESEARCH.md). All coverage is `httptest.Server`-fixture-driven, per CLAUDE.md's no-live-external-calls-in-CI constraint. If MusicBrainz becomes reachable in a future session, re-verify `recordings.go`'s field names against one real `/ws/2/recording?inc=artist-credits` response before trusting the fixtures further.

## User Setup Required

None -- no external service configuration required. This plan installs zero new dependencies (`go mod tidy` leaves `go.mod`/`go.sum` unchanged, confirmed via `git diff --exit-code -- go.mod go.sum`).

## Next Phase Readiness

- DTCT-01 (new_release), DTCT-03 (guest_feature) and DTCT-04 (idempotent seen store) are now fully implemented across both MusicBrainz and Deezer poll cycles where applicable; DTCT-02 (deluxe/tracklist-change) remains for plan 04-04, which already has `deluxeDetectionEnabled` available from plan 04-02.
- `guest_feature` rows carry no cover art or release date by design (D-05/D-12) -- Phase 5's Discord embed should render these distinctly per NTFY-02, without expecting those fields.
- The MusicBrainz recording-browse envelope shape is unverified against a live response (04-RESEARCH.md Assumption A2) -- flagged for re-verification, not blocking.
- No blockers.

---
*Phase: 04-detection-engine*
*Completed: 2026-08-08*

## Self-Check: PASSED

All 3 created files verified present on disk; both commits (`0050af3`, `b65d4e7`) verified present in `git log`.
