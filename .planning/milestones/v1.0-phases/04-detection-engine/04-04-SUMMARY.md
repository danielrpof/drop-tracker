---
phase: 04-detection-engine
plan: 04
subsystem: detection
tags: [musicbrainz, postgres, sqlc, detection, pagination, slog]

# Dependency graph
requires:
  - phase: 04-detection-engine (plan 01)
    provides: "events table's mutable track_count column (04-01's Task 1 checkpoint, option-a), the seen-store/event-log foundation, insertEvent/nullableString helpers"
  - phase: 04-detection-engine (plan 03)
    provides: "DetectMusicBrainz's guest-feature pass and the RecordingSource seam this plan's ReleaseDetailSource seam mirrors; deluxeDetectionEnabled already available from plan 04-02"
provides:
  - "internal/musicbrainz/releases.go: ReleasesByReleaseGroup -- bounded-pagination release-detail browse (D-01, inc=media) plus Release.TrackCount() summing every medium"
  - "queries/events.sql: GroupTrackCountBaseline / SetGroupTrackCountBaseline over the option-a track_count column"
  - "internal/detection.ReleaseDetailSource seam + widened New(q, recordings, releases) -- DetectMusicBrainz's deluxe-change dependency"
  - "internal/detection/musicbrainz.go: detectDeluxeChanges -- establish-then-compare baseline logic preventing 04-RESEARCH.md Pitfall #1's false positive, wired into DetectMusicBrainz's pass ordering via a preCycleSeenGroups set captured once at the top of the method"
affects: [05-discord-notifications, 06-frontend-release-history]

# Actuals (#2632)
actuals:
  tokens: 21649
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Baseline establish-then-compare: a release-group's first-ever release-detail fetch silently sets track_count as the baseline (no event); only a fetch after a baseline already exists compares and can fire deluxe_change -- prevents 04-RESEARCH.md Pitfall #1's false positive that a naive COALESCE(MAX(track_count),0) would produce"
    - "preCycleSeenGroups captured once at the top of DetectMusicBrainz, before the new_release pass inserts anything, and threaded unchanged into detectDeluxeChanges -- guarantees D-04 (a group discovered this very cycle never gets a release-detail fetch in the same cycle)"
    - "Per-group error isolation in the deluxe pass: a ReleasesByReleaseGroup error for one release-group is logged and skipped, letting the loop continue to the next group and DetectMusicBrainz still return nil"

key-files:
  created:
    - internal/musicbrainz/releases.go
    - internal/musicbrainz/releases_test.go
  modified:
    - queries/events.sql
    - internal/db/sqlc/events.sql.go
    - internal/db/sqlc/querier.go
    - internal/detection/detector.go
    - internal/detection/musicbrainz.go
    - internal/detection/detector_test.go
    - internal/detection/deezer_test.go
    - internal/detection/filter_test.go
    - internal/poller/poller_test.go
    - cmd/server/main.go

key-decisions:
  - "groupBaseline returns (baseline int, hasBaseline bool, error) rather than collapsing 'no baseline' and 'baseline is zero' into one value -- distinguishing the two is the entire mechanism preventing Pitfall #1's false positive"
  - "TestDetectDeezer_NeverProducesDeluxeChange placed in deezer_test.go (not detector_test.go, where Task 2's file list technically points) since it pins deezer.go's own architectural guarantee (D-03: DetectDeezer has no deluxe-change branch at all) alongside DetectDeezer's other TestDetectDeezer_* tests"
  - "MaxReleaseBrowseItems exported from internal/musicbrainz/releases.go (releasePageSize * maxReleasePages), mirroring MaxRecordingBrowseItems's existing convention, so the deluxe pass's page_ceiling_reached log attribute doesn't reach into unexported pagination constants"

requirements-completed: [DTCT-02]

coverage:
  - id: D1
    description: "A release inside an already-seen release-group whose total track count exceeds the recorded baseline is recorded as a deluxe_change event, keyed on the release's own MBID, with the parent group's MBID and the winning release's title/date/cover-art snapshot"
    requirement: "DTCT-02"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_FiresOnIncrease"
        status: pass
    human_judgment: false
  - id: D2
    description: "A release-group's first-ever release-detail measurement establishes the baseline silently and fires zero deluxe_change rows -- the false positive 04-RESEARCH.md Pitfall #1 exists to prevent"
    requirement: "DTCT-02"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_FirstComparisonEstablishesBaseline"
        status: pass
    human_judgment: false
  - id: D3
    description: "An equal or lower fresh track count never fires an event and never lowers the recorded baseline"
    requirement: "DTCT-02"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_NoEventOnEqualCount"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_NoEventOnDecrease"
        status: pass
    human_judgment: false
  - id: D4
    description: "A release-group discovered for the first time in the current cycle triggers zero release-detail fetches (D-04) -- it is a new_release event and nothing else"
    requirement: "DTCT-02"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_SkipsBrandNewGroup"
        status: pass
    human_judgment: false
  - id: D5
    description: "A multi-disc release compares on the sum of every medium's track count, not any single disc's; a response with absent/empty media or media entries with no track-count reports/leaves a total of 0 and is treated as no usable data, never lowering or establishing a baseline"
    requirement: "DTCT-02"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/releases_test.go#TestRelease_TrackCountSumsMedia"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_EmptyMediaLeavesBaseline"
        status: pass
    human_judgment: false
  - id: D6
    description: "The comparison uses the maximum total track count across every release in the group, independent of the order MusicBrainz returned them"
    requirement: "DTCT-02"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_UsesGroupMaximumNotOrder"
        status: pass
    human_judgment: false
  - id: D7
    description: "An artist whose release_types preference omits 'deluxe' costs zero release-detail requests; an artist with deluxe_change muted also costs zero requests while new_release rows still land; deluxe-change detection is MusicBrainz-only (D-03), never fired by a Deezer cycle"
    requirement: "DTCT-02"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_RequiresDeluxePreference"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_Muted"
        status: pass
      - kind: integration
        ref: "internal/detection/deezer_test.go#TestDetectDeezer_NeverProducesDeluxeChange"
        status: pass
    human_judgment: false
  - id: D8
    description: "A release already recorded as deluxe_change never refires on a later cycle; a release-detail error for one release-group is isolated so a sibling group's deluxe_change still lands and DetectMusicBrainz still returns nil"
    requirement: "DTCT-02"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_DoesNotRefireForSameRelease"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_PerGroupErrorIsolated"
        status: pass
    human_judgment: false
  - id: D9
    description: "Every ReleasesByReleaseGroup page request routes through the shared doRequest helper (limiter + User-Agent), never the transport directly, and the pagination loop is bounded so a runaway release-count cannot drive requests forever"
    requirement: "DTCT-02"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/releases_test.go#TestReleasesByReleaseGroup_RequestShape"
        status: pass
      - kind: unit
        ref: "internal/musicbrainz/releases_test.go#TestReleasesByReleaseGroup_StopsAtPageCeiling"
        status: pass
      - kind: other
        ref: "grep -q 'httpClient' internal/musicbrainz/releases.go (exits non-zero)"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-08-08
status: complete
---

# Phase 4 Plan 4: Deluxe/Tracklist-Change Detection Summary

**MusicBrainz release-detail track-count browse (`ReleasesByReleaseGroup`, D-01) plus an establish-then-compare baseline algorithm (option-a `track_count` column) that fires `deluxe_change` only on a genuine increase, closing the phase's highest-risk correctness gap: a group's first real comparison cycle silently sets the baseline instead of firing a false alarm.**

## Performance

- **Duration:** ~40 min (coding + verification; excludes context-gathering read time)
- **Tasks:** 2 (Task 1: end-to-end browse/baseline/compare/insert slice; Task 2: hardening against malformed media, order-dependence, and per-group errors)
- **Files modified:** 12 (2 created, 10 modified)

## Accomplishments

- `internal/musicbrainz/releases.go`: `ReleasesByReleaseGroup` — bounded-pagination browse (`maxReleasePages=10` x `releasePageSize=100`, D-01) of a release-group's releases with per-medium track-count detail (`inc=media`), structurally mirroring `ReleaseGroupsByArtist`/`RecordingsByArtist`. `Release.TrackCount()` sums every medium by range, required for D-02's multi-disc-safe total comparison.
- `queries/events.sql` + regenerated sqlc bindings: `GroupTrackCountBaseline` (`:one`, returns both the `COALESCE(MAX(track_count),0)` baseline AND a `has_baseline` boolean so "no baseline yet" is never collapsed with "baseline is zero") and `SetGroupTrackCountBaseline` (`:execrows`, mutates only the operational `track_count` column, never the D-12 write-once display snapshot).
- `internal/detection.ReleaseDetailSource`: the narrow seam `detectDeluxeChanges` depends on; `New` widens to `New(q, recordings, releases)`, with `mbClient` supplying both `RecordingSource` and `ReleaseDetailSource` in `cmd/server/main.go`.
- `internal/detection/musicbrainz.go`: `detectDeluxeChanges` implements the establish-then-compare algorithm — no baseline recorded yet silently sets it (no event); an existing baseline exceeded by the fresh maximum fires exactly one `deluxe_change` row keyed on the winning release's own MBID; equal or lower never fires and never lowers the baseline. `DetectMusicBrainz` was restructured so the pre-cycle seen release-group set (`preCycleSeenGroups`) is captured exactly once, before the `new_release` pass inserts anything, and handed unchanged to the deluxe pass — this is what guarantees D-04 (a group discovered this very cycle is never fetched for release detail in the same cycle).
- Both preference gates run before any fetch: `deluxeDetectionEnabled` (the `deluxe` boolean gate on `release_types`, never a MusicBrainz type match per Pitfall #7) and `eventTypeMuted(entry, "deluxe_change")`.
- A per-group `ReleasesByReleaseGroup` error is logged and that group skipped; the loop continues to the next group and the whole method still returns `nil`, so one unreachable release-group never discards the cycle's other events.
- The deluxe pass's `detection result` log line carries `detail_fetch_count`, `baseline_established_count`, `inserted_count`, and `page_ceiling_reached` (against the new exported `musicbrainz.MaxReleaseBrowseItems`), matching the truncation-visibility convention the recording pass already established.

## Task Commits

1. **Task 1: end-to-end slice (browse, baseline, compare, insert)** — `ddff993` (feat)
2. **Task 2: hardening against malformed media, order, and per-group errors** — `564719a` (test)

_Note: Task 1's implementation and its `httptest.Server`-fixture/real-Postgres tests were built together as one inseparable feature slice (the establish-then-compare algorithm has no meaningful partial state to test in isolation), landing in a single `feat` commit rather than a separate RED/GREEN split — matching plan 04-03's own documented precedent for tracer-shaped slices. Task 2 is test-only: every hardening behavior it proves already held against Task 1's implementation with no production code changes required._

## Files Created/Modified

- `internal/musicbrainz/releases.go` — `ReleasesByReleaseGroup`, `Release`, `Medium`, `MaxReleaseBrowseItems`
- `internal/musicbrainz/releases_test.go` — `httptest.Server` fixture coverage: envelope decode, `TrackCount()` summing, request shape, pagination, empty MBID, non-200 status, page-ceiling truncation
- `queries/events.sql`, `internal/db/sqlc/events.sql.go`, `internal/db/sqlc/querier.go` — `GroupTrackCountBaseline`, `SetGroupTrackCountBaseline`
- `internal/detection/detector.go` — `ReleaseDetailSource` interface, `Detector.releases` field, widened `New`, `groupBaseline`/`setGroupBaseline`
- `internal/detection/musicbrainz.go` — `detectDeluxeChanges`, `eventTypeDeluxeChange`, restructured `DetectMusicBrainz` (single `preCycleSeenGroups` capture)
- `internal/detection/detector_test.go` — `fakeReleaseDetailSource`/`mkRelease` helpers, 11 new deluxe-change tests, updated all pre-existing `New(...)` call sites
- `internal/detection/deezer_test.go`, `internal/detection/filter_test.go`, `internal/poller/poller_test.go` — updated pre-existing `New(...)`/`detection.New(...)` call sites for the widened signature (local no-op `ReleaseDetailSource` doubles); `deezer_test.go` also gained `TestDetectDeezer_NeverProducesDeluxeChange`
- `cmd/server/main.go` — `detection.New(sqlc.New(pool), mbClient, mbClient)`

## Decisions Made

- `groupBaseline` returns `(baseline int, hasBaseline bool, error)` rather than a single int — see key-decisions above; this is the load-bearing distinction that prevents Pitfall #1.
- `TestDetectDeezer_NeverProducesDeluxeChange` placed in `deezer_test.go` alongside the file's other `TestDetectDeezer_*` tests, even though Task 2's literal file list names `detector_test.go` — mirrors the codebase's existing per-file test organization convention.
- `MaxReleaseBrowseItems` exported from `internal/musicbrainz/releases.go`, mirroring `MaxRecordingBrowseItems`'s existing convention, so `page_ceiling_reached` never reaches into unexported pagination constants.

## Deviations from Plan

None — plan executed exactly as written. All required call-site updates for the widened `New` signature were anticipated by the plan itself (`files_modified` already listed `cmd/server/main.go` and `internal/detection/detector_test.go`); the additional touches to `deezer_test.go`, `filter_test.go`, and `poller_test.go` were the same category of mechanical compile-fix already documented as necessary in plan 04-03's own summary, not a new class of deviation.

## Issues Encountered

- `go test ./... -race` fails on this Windows dev machine with a pre-existing, already-documented cgo/ThreadSanitizer allocation failure (see STATE.md's Phase 01-02/01-03 decisions on the mingw64 gcc toolchain break) — unrelated to this plan's changes. Verified the plan's actual `<verify>` commands (which do not pass `-race`) both without and with `TEST_DATABASE_URL` set; both are green across every package.
- MusicBrainz's `/ws/2/release?inc=media` envelope shape (04-RESEARCH.md Assumption A1) remains `[ASSUMED]`/unverified against a live response this session — consistent with plans 04-01 through 04-03's own documented MusicBrainz-unreachable limitation (see PROJECT.md's WSL2 TLS note and 04-RESEARCH.md's broader March-2026 bot-blocking finding). All coverage is `httptest.Server`-fixture-driven per CLAUDE.md's no-live-external-calls-in-CI constraint. Re-verify `releases.go`'s field names against one real response if MusicBrainz becomes reachable in a future session.

## User Setup Required

None — no external service configuration required. This plan installs zero new dependencies (`go mod tidy` leaves `go.mod`/`go.sum` unchanged, confirmed via `git diff --exit-code -- go.mod go.sum`).

## Next Phase Readiness

- DTCT-01 (new_release), DTCT-02 (deluxe_change), DTCT-03 (guest_feature), DTCT-04 (idempotent seen store), and DTCT-05 (overlap guard) are now all implemented — Phase 4 (detection-engine) is functionally complete pending phase-level verification/UAT.
- `deluxe_change` rows carry the winning release's own title/date and the parent release-group's deterministic Cover Art Archive URL (D-12), matching `new_release`'s snapshot shape — Phase 5's Discord embed can render all three event types without a second external call.
- The MusicBrainz release-detail envelope shape is unverified against a live response (04-RESEARCH.md Assumption A1) — flagged for re-verification, not blocking.
- No blockers.

---
*Phase: 04-detection-engine*
*Completed: 2026-08-08*

## Self-Check: PASSED

All 2 created files verified present on disk; both commits (`ddff993`, `564719a`) verified present in `git log`.
