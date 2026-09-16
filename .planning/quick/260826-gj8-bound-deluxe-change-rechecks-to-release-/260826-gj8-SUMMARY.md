---
phase: quick/260826-gj8
plan: 01
subsystem: detection
tags: [go, musicbrainz, rate-limiting, detection-engine]

requires: []
provides:
  - "deluxeRecheckWindowDays constant (90 days) bounding detectDeluxeChanges's release-detail fetch age"
  - "withinDeluxeRecheckWindow pure helper -- precision-aware MusicBrainz partial-date window comparison"
  - "window_skipped_count aggregate log field on the deluxe_change detection result log line"
affects: [poller, musicbrainz-client, search]

actuals:
  tokens: 4327
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Precision-aware opaque-date prefix comparison (min(len(a), len(b)) then string compare) for MusicBrainz partial dates, mirroring earlierDate's existing year-prefix technique -- never time.Parse a MusicBrainz date."

key-files:
  created: []
  modified:
    - internal/detection/musicbrainz.go
    - internal/detection/musicbrainz_test.go
    - internal/detection/detector_test.go

key-decisions:
  - "deluxeRecheckWindowDays = 90, hardcoded (not env-configurable) -- matches the plan's scope; adjustable in one place if a future need arises."
  - "Every ambiguous date input (empty, <4 chars, non-numeric, over-long) resolves toward 'still check', never 'skip' -- mirrors isGuestFeature's existing doctrine of erring toward an extra alert rather than a silently missed one."
  - "The window-skip gate sits directly behind the existing D-04 preCycleSeen check, before detailFetchCount++, so a brand-new group is never double-counted as a window skip and detailFetchCount keeps meaning 'fetches actually issued'."

patterns-established:
  - "Aggregate-only logging for high-cardinality per-cycle loops: window_skipped_count is one field on the existing per-artist log line, with no per-group log line added -- avoids the log-spam failure mode a 200-group catalogue would otherwise produce."

requirements-completed: [DTCT-02]

coverage:
  - id: D1
    description: "withinDeluxeRecheckWindow correctly gates on the recheck window, including the inclusive boundary and undated/malformed-date fallback to 'still check'"
    requirement: "DTCT-02"
    verification:
      - kind: unit
        ref: "internal/detection/musicbrainz_test.go#TestWithinDeluxeRecheckWindow"
        status: pass
    human_judgment: false
  - id: D2
    description: "detectDeluxeChanges issues zero fetches for an outside-window group and exactly one for an inside-window or undated group, with all pre-existing deluxe tests unaffected"
    requirement: "DTCT-02"
    verification:
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_SkipsGroupOutsideRecheckWindow"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_ChecksGroupInsideRecheckWindow"
        status: pass
      - kind: integration
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_DeluxeChange_UndatedGroupIsStillChecked"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-08-26
status: complete
---

# Quick Task 260826-gj8: Bound deluxe-change rechecks to a rolling release-date window Summary

**Added a 90-day rolling recheck window to `detectDeluxeChanges` so already-seen release-groups older than the window are skipped at zero request cost, removing the largest unbounded consumer of the shared MusicBrainz rate limiter.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-08-26T12:08:12-05:00
- **Completed:** 2026-08-26T12:15:53-05:00
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- `deluxeRecheckWindowDays` (90) added as a documented named constant explaining the shared-rate-limiter rationale.
- `withinDeluxeRecheckWindow` added as a pure, precision-aware string-prefix comparison over MusicBrainz's opaque partial dates (never `time.Parse`), placed next to `earlierDate`.
- `detectDeluxeChanges`'s fetch loop gated: an already-seen release-group whose `FirstReleaseDate` falls outside the window is skipped with no fetch, no baseline read/write, no event, and no per-group log line -- identical in shape to the existing D-04 not-in-`preCycleSeen` skip.
- One aggregate `window_skipped_count` field added to the existing `detection result` log line for `deluxe_change`; no per-group log line was added (avoids log-spam on a large catalogue).
- Every ambiguous date (empty, malformed, non-numeric, over-long) resolves toward "still check" -- real production data is never silently dropped from detection.

## Task Commits

Each task followed RED-then-GREEN, per the plan's `tdd="true"` tasks:

1. **Task 1: Add the recheck-window constant and helper**
   - `34f79b3` (test): RED -- `TestWithinDeluxeRecheckWindow`, confirmed failing to compile without the helper.
   - `9dc2b61` (feat): GREEN -- `deluxeRecheckWindowDays` and `withinDeluxeRecheckWindow` added, not yet wired in.
2. **Task 2: Gate the fetch loop and report the aggregate skip count**
   - `61e7489` (test): RED -- three new real-Postgres window tests, confirmed `SkipsGroupOutsideRecheckWindow` failing (1 fetch, want 0) while the other two and every pre-existing deluxe test already passed.
   - `0b07d90` (feat): GREEN -- gate wired into `detectDeluxeChanges`, `window_skipped_count` log field added, doc comment extended.

**Plan metadata:** committed separately by the orchestrator (per constraints, this executor does not commit docs artifacts).

## Files Created/Modified
- `internal/detection/musicbrainz.go` - Added `deluxeRecheckWindowDays` constant, `withinDeluxeRecheckWindow` helper, wired the gate into `detectDeluxeChanges`'s fetch loop, added `window_skipped_count` to the terminal log line, extended the method's doc comment.
- `internal/detection/musicbrainz_test.go` - Added `TestWithinDeluxeRecheckWindow`, a table-driven whitebox test covering all 15 cases from the plan's behavior spec (inclusive boundary, undated, malformed, non-numeric, over-long, previous-calendar-year cutoff).
- `internal/detection/detector_test.go` - Added three real-Postgres tests (`SkipsGroupOutsideRecheckWindow`, `ChecksGroupInsideRecheckWindow`, `UndatedGroupIsStillChecked`) reusing the existing two-cycle arrangement pattern.

## Decisions Made
- `deluxeRecheckWindowDays` is a hardcoded named constant, not environment-configurable -- matches the plan's explicit scope ("adjustable in one place").
- The cutoff is computed exactly once per `detectDeluxeChanges` call (not per group), mirroring `seedNotifiedAt`'s existing single-capture-per-call reasoning, so a long-running pass cannot straddle midnight and judge two groups by two different rules.
- No fixture edits were needed to any pre-existing deluxe test -- every one already leaves `FirstReleaseDate` at its empty zero value, and the undated policy resolves that to "still check," exactly as key fact #4 in the plan predicted.

## Deviations from Plan

None - plan executed exactly as written. Both tasks matched their `<behavior>`/`<action>`/`<done>` specs with no Rule 1-4 auto-fixes needed.

## Issues Encountered

**Pre-existing, out-of-scope test flakiness discovered (not caused by this change):** `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` failed intermittently during full-suite verification (`go test ./... -count=1`), with a varying "server received N requests, want exactly 1" count (observed 93, then 9/4/2/2/4 in isolated repeated runs, then 5 in a full-suite run). Root-caused via direct DB inspection:

- This dev machine's `TEST_DATABASE_URL` points at the same `docker compose` Postgres instance (`localhost:5432`) that the **currently-running `drop-tracker-app-1` container** also uses for its live, real watchlist.
- `SELECT * FROM artists` during investigation showed real watchlisted artists (Playboi Carti, Don Toliver, Vory, Gunna, De La Rose) with real, genuinely-unnotified `guest_feature` events sitting in the table.
- The failing test's `notifier.NotifyPending` call queries **all** `notified_at IS NULL` rows in the database, not just its own test artist's rows -- so it non-deterministically sweeps up whatever real pending notifications the live app has produced at that moment (and, worse, marks them notified against a fake `httptest` webhook, meaning the real Discord notification for those events will never actually be sent).
- **Confirmed pre-existing and unrelated to this quick task**: reproduced the identical flakiness (5 consecutive failures, varying request counts) on the pre-task tree (`git checkout e1ad539` for the three touched files, i.e. before any of this task's commits).
- Every test this plan's `<verify>` blocks name explicitly (`TestWithinDeluxeRecheckWindow`, all `DeluxeChange` tests, `internal/detection` + `internal/poller` combined) passed reliably and repeatedly (5/5 runs) when run without this unrelated pre-existing flaky test's noise, and passed in 3/3 full-package reruns immediately after the one observed full-suite failure.
- **Not fixed** per the deviation rules' scope boundary (pre-existing failure in a file/test unrelated to this task's `<files>` list). Logged here for visibility; the underlying issue (test suite sharing a database with a live app container) is a local dev-environment configuration concern, not a `detection` package defect this plan's `<files>` scope covers.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `DTCT-02`'s deluxe/tracklist-change detection now bounds its MusicBrainz request volume by release-group age; the shared rate limiter should stop starving interactive `GET /search` behind bulk deluxe re-fetch traffic.
- **Flagged for follow-up (not blocking this task):** the local dev `docker-compose` stack runs the real app and the test suite against the same Postgres database/port. Running `go test ./...` locally while `docker compose up` is active can cause `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` to intermittently fail, and worse, can cause a real pending Discord notification to be silently consumed by a test's fake webhook server without ever reaching the real Discord channel. Recommend stopping the `app` service (`docker compose stop app`) before running the local test suite, or provisioning a dedicated test-only database/schema, as a separate follow-up task.

---
*Phase: quick/260826-gj8*
*Completed: 2026-08-26*

## Self-Check: PASSED

All modified files present on disk (internal/detection/musicbrainz.go, internal/detection/musicbrainz_test.go, internal/detection/detector_test.go, this SUMMARY.md). All four task commits (34f79b3, 9dc2b61, 61e7489, 0b07d90) confirmed present in `git log --oneline --all`.
