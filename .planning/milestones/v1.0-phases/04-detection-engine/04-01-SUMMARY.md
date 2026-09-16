---
phase: 04-detection-engine
plan: 01
subsystem: database
tags: [postgres, sqlc, golang-migrate, detection, poller, musicbrainz]

# Dependency graph
requires:
  - phase: 03-external-clients-search
    provides: internal/musicbrainz.Client.ReleaseGroupsByArtist, internal/poller.Poller (RunMusicBrainzCycle/RunDeezerCycle, mbRunning/dzRunning overlap guards), watchlist.Store seam
provides:
  - "events table (migration 000003) -- the combined seen-store/event-log (D-09), with the events_dedup_key UNIQUE(event_type, source, external_id) constraint enforcing DTCT-04 idempotency at the DB level"
  - "queries/events.sql: InsertEvent (:execrows, ON CONFLICT DO NOTHING), ListExternalIDs, HasAnyEvent -- and their sqlc-generated Go bindings"
  - "internal/detection package: Detector, New(q sqlc.Querier), DetectMusicBrainz -- the new_release diff against the seen store"
  - "poller.EventRecorder seam, wired into RunMusicBrainzCycle's per-artist loop and into cmd/server/main.go"
  - "Task 1 checkpoint resolution: option-a (mutable track_count column on events) -- see Decisions below"
affects: [04-02-guest-feature-and-seed-mode, 04-03-preference-filtering, 04-04-deluxe-change-baseline, 05-discord-notifications, 06-frontend-release-history]

# Actuals (#2632)
actuals:
  tokens: 14443
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "detection package mirrors watchlist's Store/Service split: Detector wraps sqlc.Querier, the consuming package (poller) declares its own narrow EventRecorder interface plus a compile-time var _ EventRecorder = (*detection.Detector)(nil) assertion in the test file, keeping the production poller.go free of a detection import"
    - "InsertEvent's :execrows ON CONFLICT DO NOTHING affected-row count (1 vs 0) is the Go-side 'was this newly detected' signal -- no separate existence check"
    - "Deterministic Cover Art Archive URL construction (no extra HTTP call) for MusicBrainz-sourced event snapshots"

key-files:
  created:
    - internal/db/migrations/000003_events.up.sql
    - internal/db/migrations/000003_events.down.sql
    - queries/events.sql
    - internal/db/sqlc/events.sql.go
    - internal/detection/detector.go
    - internal/detection/musicbrainz.go
    - internal/detection/detector_test.go
  modified:
    - internal/db/sqlc/models.go
    - internal/db/sqlc/querier.go
    - internal/poller/poller.go
    - internal/poller/poller_test.go
    - cmd/server/main.go
    - internal/db/migrate_test.go

key-decisions:
  - "Task 1 checkpoint (blocking, resolved by orchestrator before dispatch): option-a -- mutable track_count INT column on the events table, not a separate release_group_baselines table. Column is created by this plan's migration but left unpopulated; plan 04-04 is the first to write/compare it."
  - "release_group_mbid and track_count are populated/queried starting in plan 04-04 (deluxe-change baseline); this plan only creates the columns and indexes per the migration's one-way-door schema commitment."
  - "insertEvent skips a client-side already-seen check via seenExternalIDs before calling InsertEvent, rather than relying solely on ON CONFLICT DO NOTHING -- avoids a wasted round trip for the majority of a steady-state cycle's already-recorded catalogue."

patterns-established:
  - "Real-Postgres detection tests derive a unique artist mbid from t.Name() and insert a minimal artists row directly via pool.Exec (not via watchlist.Service, which internal/detection must not depend on), matching internal/watchlist/service_test.go's established fixture convention."
  - "poller_test.go's newTestPoller helper took a variadic trailing EventRecorder parameter so New's widened 6-argument constructor did not require touching all ~25 existing call sites -- only the 8 call sites that bypass the helper needed an explicit fakeEventRecorder{} argument."

requirements-completed: [DTCT-01, DTCT-04, DTCT-05]

coverage:
  - id: D1
    description: "A MusicBrainz poll cycle records one new_release event row per previously-unseen release-group MBID, with the D-12 snapshot (title, artist_name, release_date, cover_art_url) populated"
    requirement: "DTCT-01"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_NewRelease"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_NewRelease_UndatedGroup"
        status: pass
      - kind: integration
        ref: "internal/poller/poller_test.go#TestPoller_RunMusicBrainzCycle_RecordsNewRelease"
        status: pass
    human_judgment: false
  - id: D2
    description: "Re-detecting an already-recorded release-group inserts no duplicate row and never mutates the original snapshot (DTCT-04, D-20), proven at 1-then-0 affected rows against real Postgres"
    requirement: "DTCT-04"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestInsertEvent_Idempotent"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestInsertEvent_SnapshotIsWriteOnce"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestInsertEvent_SourceSeparatesNamespaces"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_ReDetectionInsertsNothing"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_PartialCycleResumes"
        status: pass
    human_judgment: false
  - id: D3
    description: "No overlapping poll-cycle runs for the same source, now with detection wired into the cycle -- the overlap guard and per-artist error isolation hold with blocking/erroring inside the detection call itself"
    requirement: "DTCT-05"
    verification:
      - kind: integration
        ref: "internal/poller/poller_test.go#TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning"
        status: pass
      - kind: integration
        ref: "internal/poller/poller_test.go#TestPoller_RunMusicBrainzCycle_GuardReleasedAfterDetectionError"
        status: pass
      - kind: integration
        ref: "internal/poller/poller_test.go#TestPoller_RunMusicBrainzCycle_DetectionErrorIsolatedPerArtist"
        status: pass
      - kind: integration
        ref: "internal/poller/poller_test.go#TestPoller_RunMusicBrainzCycle_EmptyWatchlist"
        status: pass
      - kind: integration
        ref: "internal/poller/poller_test.go#TestPoller_CyclesAreIndependentAcrossSources"
        status: pass
    human_judgment: false

duration: 30min
completed: 2026-08-07
status: complete
---

# Phase 4 Plan 1: Thin End-to-End Detection Slice Summary

**A scheduled MusicBrainz poll cycle now diffs fetched release-groups against a persistent Postgres seen store and records each previously-unseen one as a `new_release` event row, idempotently at the database level via `ON CONFLICT DO NOTHING`, with the overlap guard proven to hold even while detection itself is in flight.**

## Performance

- **Duration:** ~30 min (coding + verification; excludes context-gathering read time)
- **Tasks:** 3 (Task 1: checkpoint decision, resolved by the orchestrator before dispatch; Task 2: tracer; Task 3: idempotency/overlap-guard proof)
- **Files modified:** 13 (7 created, 6 modified)

## Task 1 Decision (Checkpoint, resolved before dispatch)

**Task 1 decision: option-a (mutable `track_count` column on the `events` table).**

The orchestrator presented this blocking decision to the user before spawning this executor and the user selected option-a. Migration `000003_events` therefore adds `track_count INT` directly to the `events` table (not a separate `release_group_baselines` table). The column is created by this plan but left `NULL`/unpopulated on every row this plan writes -- plan 04-04 is the first to populate and compare it for the deluxe-change baseline (D-01/D-02/D-04). This decision shapes 04-04's baseline queries directly: `SELECT MAX(track_count) FROM events WHERE release_group_mbid = $1 AND track_count IS NOT NULL` is the query shape 04-04 should use, not a lookup against a second table.

## Accomplishments

- Migration `000003_events`: the combined seen-store/event-log table (D-09), with `events_dedup_key UNIQUE (event_type, source, external_id)` enforcing DTCT-04's idempotency at the database level, a `source` discriminator disambiguating MusicBrainz vs. Deezer id namespaces, and the three D-11/D-14/D-02 indexes
- `queries/events.sql` + generated sqlc bindings: `InsertEvent` (`:execrows`, `ON CONFLICT DO NOTHING`), `ListExternalIDs`, `HasAnyEvent`
- New `internal/detection` package: `Detector.DetectMusicBrainz` diffs a cycle's fetched release-groups against the seen store and records each unseen MBID as a `new_release` row with the D-12 display snapshot (title, artist name, release date, deterministic Cover Art Archive URL)
- `poller.EventRecorder` seam wired into `RunMusicBrainzCycle`'s per-artist loop (right after the existing "poll result" log line) and into `cmd/server/main.go`'s wiring
- Real-Postgres proof (14 tests across `internal/detection` and `internal/poller`) that: a poll cycle records exactly the expected rows; re-detection and a simulated partial-cycle crash both re-derive cleanly with no duplicates; `InsertEvent` reports 1-then-0 affected rows for the same dedup key; a conflicting re-insert never mutates the stored snapshot; the overlap guard and per-artist error isolation hold even when blocking/erroring happens inside the detection call itself, not just the fetch call

## Task Commits

Each task was committed atomically (TDD RED/GREEN split for the tracer):

1. **Task 2 (tracer) -- RED:** `a3f740a` (test) -- failing tests for `DetectMusicBrainz` and the `EventRecorder`-wired cycle; fails to compile since neither exists yet
2. **Task 2 (tracer) -- GREEN:** `7e8a6c2` (feat) -- migration, queries, `internal/detection`, `poller.EventRecorder`, `cmd/server/main.go` wiring; all tests pass
3. **Task 3:** `a66c40f` (test) -- idempotency and detection-wired overlap-guard proof; test-only, every behavior already held against the tracer's implementation
4. **Auto-fix (Rule 1):** `d9fafe0` (fix) -- updated a pre-existing hardcoded migration-version assertion broken by this plan's own migration addition (see Deviations)

**Tracer feedback gate:** the tracer's full `<verify>` chain (`go build && go vet && sqlc generate+diff && go test -short ./... && TEST_DATABASE_URL=... go test ./internal/detection/... ./internal/poller/...`) was re-run end-to-end immediately after the GREEN commit and passed before Task 3 began.

## Files Created/Modified

- `internal/db/migrations/000003_events.up.sql` / `.down.sql` -- the seen-store/event-log table
- `queries/events.sql` -- `InsertEvent`, `ListExternalIDs`, `HasAnyEvent`
- `internal/db/sqlc/events.sql.go`, `models.go`, `querier.go` -- regenerated sqlc bindings (sqlc v1.31.1)
- `internal/detection/detector.go` -- `Detector`, `New`, `insertEvent`, `nullableString`
- `internal/detection/musicbrainz.go` -- `DetectMusicBrainz`, `seenExternalIDs`, `coverArtURLForReleaseGroup`
- `internal/detection/detector_test.go` -- real-Postgres tests for both Task 2 and Task 3
- `internal/poller/poller.go` -- `EventRecorder` interface, `Poller.events` field, widened `New` signature, detection call in `RunMusicBrainzCycle`, rewritten package doc comment
- `internal/poller/poller_test.go` -- `fakeEventRecorder` double, `var _ EventRecorder = (*detection.Detector)(nil)` assertion, all existing `New`/`newTestPoller` call sites updated, new integration and overlap-guard tests
- `cmd/server/main.go` -- `detection.New(sqlc.New(pool))` wired into `poller.New`
- `internal/db/migrate_test.go` -- hardcoded schema-version assertion bumped from 2 to 3 (Rule 1 auto-fix)

## Decisions Made

- Task 1 checkpoint: option-a, mutable `track_count` column (see Task 1 Decision section above) -- binding on plan 04-04
- `seenExternalIDs` pre-filters already-seen MBIDs client-side before calling `InsertEvent`, rather than relying solely on the DB's `ON CONFLICT DO NOTHING` to no-op them -- avoids a wasted round trip per already-recorded release on every steady-state cycle
- `poller_test.go`'s `newTestPoller` helper took a variadic trailing `EventRecorder` parameter (defaulting to a no-op `fakeEventRecorder`) so the widened `New` constructor did not require touching all ~25 existing call sites that go through the helper -- only the 8 call sites that construct `Poller` directly needed an explicit argument

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated `TestRunMigrations_AppliesFromScratch`'s hardcoded schema version**

- **Found during:** Plan-level verification (running the full real-Postgres suite across all packages, not just `internal/detection`/`internal/poller`)
- **Issue:** `internal/db/migrate_test.go` asserted the from-scratch migration state lands on `version=2` -- true before this plan's migration `000003_events` existed, false after. The test's own comment already documented that Phase 2 had bumped this same assertion from 1 to 2 when it added `000002_watchlist`; this plan needed the identical bump.
- **Fix:** Updated the assertion and comment to `version=3`, matching the now-three-migration schema.
- **Files modified:** `internal/db/migrate_test.go`
- **Verification:** `TEST_DATABASE_URL=... go test ./... -count=1` passes across every package (previously failed only in `internal/db`)
- **Committed in:** `d9fafe0`

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary correctness fix directly caused by this plan's own migration addition; no scope creep, no behavior outside the plan's stated scope.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None -- no external service configuration required. This plan installs zero new dependencies (`go mod tidy` leaves `go.mod`/`go.sum` byte-identical).

## Next Phase Readiness

- The `events` table, `internal/detection` package, and `poller.EventRecorder` seam are all in place for plan 04-02 (guest-feature detection and seed mode) and plan 04-03 (preference filtering) to extend directly -- no schema or wiring changes needed for those, only new detection logic and query filters.
- Plan 04-04 (deluxe-change baseline) has its schema question already resolved: the `track_count` column exists on `events` (option-a) but is unpopulated by this plan -- 04-04 must establish-then-compare it per 04-RESEARCH.md Pitfall #1's warning (a naive `COALESCE(MAX(track_count), 0)` would false-positive on every group's first real comparison).
- `HasAnyEvent` (seed-mode check, D-14) is already implemented in `queries/events.sql` and generated, ready for plan 04-02 to consume -- it was intentionally built in this plan's migration since it's part of the same query contract, even though this plan's `DetectMusicBrainz` does not yet call it (seed mode is out of this plan's scope).
- No blockers.

---
*Phase: 04-detection-engine*
*Completed: 2026-08-07*

## Self-Check: PASSED

All 7 created files verified present on disk; all 4 commits (`a3f740a`, `7e8a6c2`, `a66c40f`, `d9fafe0`) verified present in `git log`.
