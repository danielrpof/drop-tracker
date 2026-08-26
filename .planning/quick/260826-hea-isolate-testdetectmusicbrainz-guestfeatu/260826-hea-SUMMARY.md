---
phase: quick-260826-hea
plan: 01
subsystem: testing
tags: [postgres, pgx, testutil, integration-test, notifier]

# Dependency graph
requires:
  - phase: quick-260826-gj8
    provides: "Prior identification of the shared docker-compose Postgres fixture and its default (public) schema being shared with the live dev app"
provides:
  - "TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier isolated onto its own dedicated Postgres schema (detection_notify_test), no longer able to sweep up the live dev app's real pending Discord notifications"
affects: [internal/detection, internal/notifier, testutil]

# Actuals (#2632)
actuals:
  tokens: 658
  tasks: 1
  commits: 1

tech-stack:
  added: []
  patterns: ["testutil.NewIsolatedTestPool for any test that makes a real NotifyPending/ListUnnotified call"]

key-files:
  created: []
  modified:
    - internal/detection/detector_test.go

key-decisions:
  - "Isolated only the one test that makes a real NotifyPending call (TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier), leaving all 43 other detector_test.go tests on the shared fixture pool, since only this one test's global unfiltered ListUnnotified scan (D-06) can reach rows outside its own inserts"
  - "Schema literal detection_notify_test chosen to be distinct from internal/notifier's existing notifier_test schema, per NewIsolatedTestPool's documented per-package uniqueness rule"
  - "Restored the 45 real live-app pending-notification rows that the RED-half sentinel probe collaterally marked notified_at during diagnosis, since they were genuine dev-app data, not test fixtures -- only the sentinel's own artificial row was left for the plan's specified cleanup step"

patterns-established:
  - "A test that must exercise a real NotifyPending/ListUnnotified call always uses testutil.NewIsolatedTestPool with a schema name unique to its own package, never the shared-fixture testutil.NewTestPool"

requirements-completed: [QUICK-260826-hea]

coverage:
  - id: D1
    description: "TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier routed onto its own dedicated Postgres schema so its real NotifyPending call can no longer mark the live dev app's pending Discord notifications as sent"
    requirement: "QUICK-260826-hea"
    verification:
      - kind: integration
        ref: "RED/GREEN sentinel probe against the shared public schema: notified_at IS NULL returned f before the edit, t after"
        status: pass
      - kind: unit
        ref: "internal/detection/detector_test.go#TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier"
        status: pass
      - kind: integration
        ref: "go test ./internal/detection/ ./internal/notifier/ -count=1"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-26
status: complete
---

# Phase quick-260826-hea Plan 01: Isolate the muted-guest-feature notifier test Summary

**Routed `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier`'s real `NotifyPending` call onto a dedicated `detection_notify_test` Postgres schema via `testutil.NewIsolatedTestPool`, so it can no longer sweep up and falsely-ack the live dev app's real pending Discord notifications sitting in the shared public schema**

## Performance

- **Duration:** 25 min
- **Started:** 2026-08-26T17:36:00Z
- **Completed:** 2026-08-26T18:01:00Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` now uses `testutil.NewIsolatedTestPool(t, "detection_notify_test")` instead of the shared-fixture `testutil.NewTestPool(t)`
- Proved the bug and the fix with a live RED/GREEN sentinel probe against the real docker-compose Postgres fixture: a sentinel `events` row's `notified_at` flipped from NULL to non-NULL when the unmodified test ran (bug reproduced), and stayed NULL after the fix (bug closed)
- Discovered during the RED probe that this exact bug had already caused real collateral damage: the unmodified test's run marked 45 genuine, pre-existing live-app pending Discord notification rows (real artist/track titles from artist_ids 5, 6, 7) as `notified_at` non-NULL. Restored all 45 to NULL before proceeding, since they were real dev-app data, not test fixtures.
- Updated two documentation comments (file top-of-file convention comment, and the target test's own doc comment) to record the isolation exception and its D-06 rationale

## Task Commits

Each task was committed atomically:

1. **Task 1: Route the muted-guest-feature notifier test onto a dedicated Postgres schema** - `acef51e` (fix)

_Note: this was a single-task plan; no TDD RED/GREEN/REFACTOR split applied (the plan's RED/GREEN cycle was a diagnostic sentinel probe against the database, not a test-code RED/GREEN cycle)._

## Files Created/Modified
- `internal/detection/detector_test.go` - One pool-constructor call site changed from `testutil.NewTestPool(t)` to `testutil.NewIsolatedTestPool(t, "detection_notify_test")`; two doc comments updated to record the exception

## Decisions Made
- Isolated only the single test making a real `NotifyPending` call, per the plan's explicit scope boundary — the other 43 tests in the file only assert on their own `artist_id`-scoped rows and don't have this problem
- Restored the 45 collaterally-affected live-app rows discovered during the RED probe (see Deviations below) rather than leaving them permanently marked notified, since that would have converted a diagnostic step into a second occurrence of the exact bug this task exists to fix

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Restored 45 real live-app pending-notification rows collaterally marked notified during the RED probe**
- **Found during:** Task 1, running the RED half of the sentinel probe against the unmodified file
- **Issue:** The plan's RED probe seeds one artificial sentinel row and expects the unmodified test's global `ListUnnotified` scan to flip it — but the shared public schema also held 45 genuine, pre-existing pending Discord notifications from the live dev app (real artist/track data, artist_ids 5/6/7). Running the unmodified test flipped all 46 rows' `notified_at` to non-NULL in one batch (visible as "server received 47 requests" in the test's own output, 46 real + 1 sentinel), which is exactly the data-loss bug this plan exists to prevent — now demonstrated as reachable, not hypothetical.
- **Fix:** Identified the exact timestamp window of the batch this test run produced (`notified_at` between `17:38:00` and `17:39:00` UTC) and reset those 45 non-sentinel rows' `notified_at` back to NULL via a direct `UPDATE`, restoring the live app's pending notifications to their pre-probe state. The sentinel row itself (`external_id = 'hea-sentinel-ext'`) was left for the plan's own specified cleanup step (deleted after the GREEN probe confirmed the fix).
- **Files modified:** None (database-only remediation, no source change)
- **Verification:** Post-restore query confirmed `count(*) WHERE notified_at IS NULL = 45` immediately after the restore, and unchanged at `45` again after the GREEN-half test run completed on the isolated schema — proving the fix does not touch the shared schema at all.
- **Committed in:** N/A (data-only, no commit; the sentinel's own row was deleted per the plan's `<verify>` cleanup step, not committed)

---

**Total deviations:** 1 auto-fixed (1 bug remediation, database-only)
**Impact on plan:** Necessary to leave the shared dev database in the same state it was found in — an omission here would have converted a diagnostic step into a second real occurrence of the exact defect the plan fixes. No scope creep into source code; the fix itself is exactly the one-call-site change the plan specified.

## Issues Encountered
- The RED probe's own test run failed its own internal assertion ("server received 47 requests, want exactly 1") because of the pre-existing 45 real pending rows described above — this is expected: it demonstrates the practical severity of the bug, not a plan defect. The probe's actual signal (the sentinel's `notified_at` flip) was checked independently of the test's pass/fail state, matching the plan's own `<verify>` script structure.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `internal/detection` no longer has any test capable of falsely-acking the live dev app's pending Discord notifications; `internal/notifier` was already correct and served as the reference pattern
- No further isolation work identified — the plan's own scope boundary confirmed all other DB-backed tests in the touched files are already artist-scoped and unaffected by the D-06 global-scan behavior
- No blockers for future phases

---
*Phase: quick-260826-hea*
*Completed: 2026-08-26*

## Self-Check: PASSED
