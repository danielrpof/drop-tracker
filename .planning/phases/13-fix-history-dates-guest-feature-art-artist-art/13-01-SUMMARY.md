---
phase: 13-fix-history-dates-guest-feature-art-artist-art
plan: 01
subsystem: detection
tags: [musicbrainz, go, react, vitest, event-detection, history-cards]

# Dependency graph
requires:
  - phase: 04-detection-engine
    provides: RecordingSource interface, detectGuestFeatures pass, guest_feature event insert wiring
  - phase: 06-frontend-release-history
    provides: EventCard.tsx's three event-type bodies (NewReleaseBody/GuestFeatureBody/DeluxeChangeBody), EventItem type
provides:
  - internal/musicbrainz.ReleasesForRecording: single-entity recording lookup (ws/2/recording/{mbid}?inc=releases+release-groups)
  - internal/detection: widened RecordingSource interface; detectGuestFeatures now sources release_date/release_group_mbid/cover_art_url per newly-detected recording, with D-02's precision-aware earliest-date rule, D-03's no-releases fallback, OQ-02's per-recording error isolation, and D-13's per-cycle lookup cap (maxNewGuestFeatureLookupsPerCycle = 20)
  - web/app/components/history/EventCard.tsx: GuestFeatureBody and DeluxeChangeBody both render a release date line with the shared "Release date unknown" fallback
affects: [phase-13-plan-02, future-guest-feature-work, history-tab-ui]

# Actuals (#2632)
actuals:
  tokens: 12414
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single-entity MusicBrainz lookup shape (ws/2/{entity}/{mbid}?inc=...) as a new client pattern alongside the existing browse-by-query-param shape -- no *-count/*-offset pagination envelope, one GET, still routed through the shared doRequest rate-limiter/User-Agent seam"
    - "Precision-aware date comparison (earlierDate): compare year first, then treat a same-year strict-prefix pair as a precision difference where the longer string wins, only falling back to plain lexicographic comparison at equal precision"

key-files:
  created:
    - internal/musicbrainz/recording_lookup.go
    - internal/musicbrainz/recording_lookup_test.go
  modified:
    - internal/detection/detector.go
    - internal/detection/musicbrainz.go
    - internal/detection/musicbrainz_test.go
    - internal/detection/detector_test.go
    - internal/detection/filter_test.go
    - internal/poller/poller_test.go
    - web/app/components/history/EventCard.tsx
    - web/app/components/history/EventCard.test.tsx
    - web/package.json

key-decisions:
  - "Combined RED-test and GREEN-implementation commits for both Go tasks (tasks 1 and 2) instead of the usual separate test()/feat() commit split -- this repo's pre-commit hook runs golangci-lint with full package type-checking, which fails on a commit that references a not-yet-existing symbol (ReleasesForRecording, earliestReleaseDate, guestFeatureArt). Task 3 (frontend-only) kept the standard RED/GREEN split since golangci-lint only runs against Go files."
  - "Fixed a blocking npm install failure (arborist bug in npm 10.9.4: 'Cannot read properties of null (reading edgesOut)') via --legacy-peer-deps, then added the missing peer @testing-library/dom (already required by the pre-existing @testing-library/react devDependency) -- generated web/package-lock.json for the first time in this repo."
  - "guestFeatureArt is deliberately decoupled from earliestReleaseDate: cover art can be sourced from a different release than the one supplying the earliest date, matching the plan's explicit behavior spec (a release with no release-group MBID can still supply the date while another release supplies the art)."

requirements-completed: [D-01, D-02, D-03, D-04, D-05, D-13, OQ-01, OQ-02]

coverage:
  - id: D1
    description: "A genuinely new guest_feature event row gets a non-null release_date and cover_art_url when its recording lookup returns a dated release (D-01)"
    requirement: "D-01"
    verification:
      - kind: unit
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeatureStoresReleaseDateAndCoverArt"
        status: pass
    human_judgment: false
  - id: D2
    description: "Earliest-date selection is precision-aware: a same-year vaguer date never beats a more precise same-year date (D-02, grilling round Q2)"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "internal/detection/musicbrainz_test.go#TestEarliestReleaseDate"
        status: pass
      - kind: unit
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_EarliestDateReachesDB"
        status: pass
    human_judgment: false
  - id: D3
    description: "A per-artist per-cycle guest-feature lookup cap of 20 bounds one artist's contribution to the shared MusicBrainz rate budget (D-13)"
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_PerCycleLookupCap"
        status: pass
    human_judgment: false
  - id: D4
    description: "A recording lookup returning no releases still inserts the guest_feature row with NULL release_date/cover_art_url (D-03)"
    requirement: "D-03"
    verification:
      - kind: unit
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_EmptyReleaseListInsertsWithNulls"
        status: pass
    human_judgment: false
  - id: D5
    description: "One recording's lookup error is isolated -- siblings in the same browse result still process (OQ-02)"
    requirement: "OQ-02"
    verification:
      - kind: unit
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_PerRecordingLookupErrorIsolated"
        status: pass
    human_judgment: false
  - id: D6
    description: "Deluxe-change and guest-feature History cards render a release date, falling back to 'Release date unknown' (D-04, D-05, OQ-01)"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "web/app/components/history/EventCard.test.tsx (guest_feature date cases + 4 deluxe_change date cases)"
        status: pass
    human_judgment: false
  - id: D7
    description: "ReleasesForRecording builds its path via url.PathEscape and reports non-OK status by code only (ASVS V5, T-13-01/T-13-02)"
    verification:
      - kind: unit
        ref: "internal/musicbrainz/recording_lookup_test.go#TestReleasesForRecording_RequestShape, #TestReleasesForRecording_NonOKStatus"
        status: pass
    human_judgment: false
  - id: D8
    description: "The recording_lookup.go response-shape assumption ([ASSUMED] per 13-RESEARCH.md Assumption A1) matches a live MusicBrainz response"
    verification: []
    human_judgment: true
    rationale: "This dev box's WSL2 network path cannot reach musicbrainz.org (documented, waived Phase 3 blocker in STATE.md) -- the plan's Task 1 human-check explicitly requires a machine that CAN reach musicbrainz.org to run the documented curl comparison. Not automatable from this environment; still outstanding."

duration: ~40min
completed: 2026-08-24
status: complete
---

# Phase 13 Plan 01: Guest-Feature Release Date & Cover Art Summary

**MusicBrainz recording lookup (ws/2/recording/{mbid}?inc=releases+release-groups) sources guest-feature release_date/cover_art_url with a precision-aware earliest-date rule, a 20-lookup-per-cycle cap, per-recording error isolation, and matching date rendering on both guest-feature and deluxe-change History cards.**

## Performance

- **Duration:** ~40 min
- **Tasks:** 3
- **Files modified:** 11 (2 created, 9 modified)

## Accomplishments

- New `internal/musicbrainz.ReleasesForRecording` single-entity lookup client method, following the project's shared `doRequest` rate-limiter/User-Agent seam, `url.PathEscape` on the caller-influenced mbid (T-13-01), and status-code-only error reporting (T-13-02)
- `detection.RecordingSource` widened with `ReleasesForRecording` with zero change to `detection.New`'s constructor signature or `cmd/server/main.go`'s wiring line
- `detectGuestFeatures` now sources `release_date`/`release_group_mbid`/`cover_art_url` for every genuinely-new guest-feature recording, using:
  - `earliestReleaseDate` -- D-02's precision-aware earliest-date rule (amended by the grilling round's Q2: a same-year vaguer date never beats a more precise same-year date)
  - `guestFeatureArt` -- independently finds the first release carrying a release-group MBID (decoupled from the date source, per D-03's fallback)
  - Per-recording lookup error isolation (OQ-02): one failing recording is skipped, logged, and retried next cycle; siblings in the same browse result still process
  - `maxNewGuestFeatureLookupsPerCycle = 20` (D-13, grilling round Q5): bounds one artist's contribution to the shared MusicBrainz rate budget per cycle; excess recordings are retried next cycle
  - `release_link_ceiling_count` and `guest_feature_lookup_cap_reached_at` structured log attributes make MusicBrainz's 25-linked-release truncation and the per-cycle cap observable
- `GuestFeatureBody` and `DeluxeChangeBody` in `EventCard.tsx` both render a release date line (`DeluxeChangeBody` ahead of the track-count delta, per D-04), reusing `NewReleaseBody`'s exact `?? "Release date unknown"` fallback expression (D-05/OQ-01) with no new copy invented and no `api.ts` change

## Task Commits

Each task was committed atomically (with the deviation noted below for tasks 1-2):

1. **Task 1: End-to-end guest-feature date and art** -- combined test+feat `b9f0c62` (an initial RED-only commit attempt was rejected by the pre-commit hook before landing -- see Deviations)
2. **Task 2: Earliest-date selection, no-releases fallback, per-recording error isolation, per-cycle lookup cap** -- combined test+feat `2b19467`
3. **Task 3: Deluxe-change card renders its release date** -- RED test `de7ee2d`, GREEN implementation `7a8eb76`

_Tasks 1 and 2 deviated from the strict RED/GREEN commit split -- see Deviations below._

## Files Created/Modified

- `internal/musicbrainz/recording_lookup.go` - New `ReleasesForRecording` single-entity lookup, `RecordingRelease`/`RecordingReleaseGroup` types, `MaxRecordingReleaseLinks` const
- `internal/musicbrainz/recording_lookup_test.go` - httptest coverage: decode, request shape/escaping, empty mbid, non-OK status, malformed JSON, empty result
- `internal/detection/detector.go` - Widened `RecordingSource` interface with `ReleasesForRecording`
- `internal/detection/musicbrainz.go` - `detectGuestFeatures` wiring, `earliestReleaseDate`/`earlierDate`/`guestFeatureArt` helpers, `maxNewGuestFeatureLookupsPerCycle` const
- `internal/detection/musicbrainz_test.go` - Whitebox tests for `earliestReleaseDate`/`guestFeatureArt`
- `internal/detection/detector_test.go` - `fakeRecordingSource` stub extension, `erroringRecordingSource` new test double, 6 new real-Postgres tests
- `internal/detection/filter_test.go` - `noRecordingSource` stub extension (interface-satisfaction only)
- `internal/poller/poller_test.go` - `fakeRecordingSource` stub extension (interface-satisfaction only)
- `web/app/components/history/EventCard.tsx` - `GuestFeatureBody` date line, `DeluxeChangeBody` date prepended to both branches
- `web/app/components/history/EventCard.test.tsx` - 2 guest-feature date cases + 4 deluxe-change date cases
- `web/package.json` / `web/package-lock.json` - Added `@testing-library/dom` peer dependency (blocking-issue fix, see Deviations)

## Decisions Made

- Combined RED-test and GREEN-implementation commits for tasks 1 and 2 (Go) instead of the usual separate `test()`/`feat()` split -- this repo's pre-commit hook runs `golangci-lint` with full package type-checking, which fails on a commit referencing a not-yet-existing symbol. Task 3 (frontend-only, no Go files) kept the standard RED/GREEN split since `golangci-lint` doesn't run against `.tsx` files.
- `guestFeatureArt` is deliberately decoupled from `earliestReleaseDate` -- cover art can be sourced from a different release than the one supplying the earliest date, per the plan's explicit behavior spec.
- Created a temporary isolated Postgres database (`agent_13_01_test`, dropped after use) to run this worktree's `TEST_DATABASE_URL`-gated tests without colliding with a concurrent sibling worktree agent's migration activity against the shared `drop-tracker-postgres-1` container on port 5432 -- no project files were changed for this, it was purely a verification-time workaround.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed npm install failure and added missing @testing-library/dom peer dependency**
- **Found during:** Task 1 (writing RED tests for `EventCard.tsx`)
- **Issue:** `web/node_modules` was never installed in this worktree. A plain `npm install` hit a pre-existing npm 10.9.4 arborist bug (`TypeError: Cannot read properties of null (reading 'edgesOut')`) resolving the peer-dependency graph. `npm install --legacy-peer-deps` worked but silently skipped `@testing-library/dom`, a required peer of the already-declared `@testing-library/react` devDependency, causing `Cannot find package '@testing-library/dom'` at test-run time.
- **Fix:** Installed with `--legacy-peer-deps`, then explicitly added `@testing-library/dom@^10.0.0` (the version `@testing-library/react`'s own `peerDependencies` declares) via `npm install --legacy-peer-deps --save-dev @testing-library/dom@^10.0.0`. This generated `web/package-lock.json` for the first time in this repo.
- **Files modified:** `web/package.json`, `web/package-lock.json`
- **Verification:** `cd web && npm test -- EventCard` runs and reports pass/fail counts correctly afterward.
- **Committed in:** `b9f0c62` (Task 1 commit)

**2. [Rule 3 - Blocking] Extended three other RecordingSource test doubles to satisfy the widened interface**
- **Found during:** Task 1 (widening `detection.RecordingSource`)
- **Issue:** `internal/detection/filter_test.go`'s `noRecordingSource` and `internal/poller/poller_test.go`'s `fakeRecordingSource` (both pre-existing no-op doubles, distinct from `detector_test.go`'s `fakeRecordingSource`) no longer satisfied the widened `RecordingSource` interface once `ReleasesForRecording` was added, breaking `go vet`/compilation for those packages.
- **Fix:** Added a no-op `ReleasesForRecording` method (`return nil, nil`) to both doubles, matching their existing zero-value-no-op convention.
- **Files modified:** `internal/detection/filter_test.go`, `internal/poller/poller_test.go`
- **Verification:** `go vet ./...` clean; `go build ./...` clean.
- **Committed in:** `b9f0c62` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both auto-fixes were necessary infrastructure/compile fixes with zero functional scope creep -- no new production behavior beyond what the plan specified.

## Issues Encountered

- **Transient/environmental test failures during verification, not caused by this plan's code:** While re-running the full `internal/detection` suite for verification, two distinct shared-infrastructure issues surfaced, both traced to this dev environment running multiple concurrent git-worktree agents against the same Postgres container (`drop-tracker-postgres-1` on port 5432):
  1. `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` failed once with "server received 414 requests, want exactly 1" -- caused by a concurrent sibling worktree agent inserting un-notified rows into the shared `events` table during this test's table-wide `NotifyPending` scan. Re-ran in isolation immediately after: passed cleanly.
  2. A later full-suite run failed every DB-backed test in the package with `no migration found for version 5: read down for version 5 migrations: file does not exist` -- a concurrent sibling worktree agent (very likely plan 13-02, which this phase's plan text says "adds the one migration this phase now has") applied migration 000005 to the shared database from its own worktree/branch, but this worktree's checked-out `internal/db/migrations/` only goes up to 000004, so `testutil.NewTestPool`'s migrate-down-then-up reset couldn't find the down file it needed.
  - Neither issue is a defect in this plan's code -- both are pre-existing hazards of the current setup (a single shared Postgres instance reused across concurrent worktree agents; see STATE.md's existing Blockers/Concerns entry about port collisions across worktrees for the same class of issue). Worked around for verification purposes only by creating a temporary isolated database (`agent_13_01_test`, dropped after use); all tests pass cleanly against it. No project files were changed to work around this.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 13-02 (which adds this phase's one schema migration, per 13-CONTEXT.md D-12) can proceed independently -- this plan added zero schema migrations and zero new npm/Go dependencies beyond the `@testing-library/dom` peer-dependency fix.
- **Outstanding manual verification (Task 1's `<human-check>`):** the `RecordingRelease`/`recordingLookupResponse` struct shape is `[ASSUMED]` per 13-RESEARCH.md Assumption A1 and has not been confirmed against a live MusicBrainz response -- this dev box's WSL2 network path cannot reach `musicbrainz.org` (the same documented, waived Phase 3 blocker recorded in STATE.md). From a machine that CAN reach musicbrainz.org, run:
  ```
  curl "https://musicbrainz.org/ws/2/recording/<a-real-recording-mbid>?inc=releases+release-groups&fmt=json"
  ```
  and compare the returned nesting against `RecordingRelease`/`recordingLookupResponse`'s struct tags in `internal/musicbrainz/recording_lookup.go`. A field-name mismatch decodes silently to zero values (Go's `encoding/json` behavior) that look exactly like D-03's no-releases fallback -- this is the one assumption in the phase that cannot be checked in CI.

---
*Phase: 13-fix-history-dates-guest-feature-art-artist-art*
*Completed: 2026-08-24*

## Self-Check: PASSED

All key files (`internal/musicbrainz/recording_lookup.go`, `internal/musicbrainz/recording_lookup_test.go`, `internal/detection/detector.go`, `internal/detection/musicbrainz.go`, `internal/detection/musicbrainz_test.go`, `internal/detection/detector_test.go`, `internal/detection/filter_test.go`, `internal/poller/poller_test.go`, `web/app/components/history/EventCard.tsx`, `web/app/components/history/EventCard.test.tsx`, `web/package.json`) confirmed present on disk. All four commit hashes (`b9f0c62`, `2b19467`, `de7ee2d`, `7a8eb76`) confirmed present in `git log --all`.
