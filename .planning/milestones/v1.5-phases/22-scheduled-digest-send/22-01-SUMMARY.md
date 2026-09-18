---
phase: 22-scheduled-digest-send
plan: 01
subsystem: digest-scheduling
tags: [postgres, sqlc, golang-migrate, timezone, dst, slot-math]

# Dependency graph
requires:
  - phase: 20-digest-settings-operator-control
    provides: notification_settings singleton row, settings.Service Get/Update, Cadence type
  - phase: 21-real-time-digest-mutual-exclusion
    provides: notifier.SettingsReader seam, fail-closed settings reads, notifying CAS lock pattern
provides:
  - Migration 000009 (nullable notification_settings.digest_last_slot_at)
  - Regenerated sqlc AckDigestBatchParams and UpdateNotificationSettingsParams (with ReanchorSlot)
  - internal/settings.MostRecentSlot / GraceFor / ZoneName calendar-based slot math
  - settings.Service.Update's D-14 re-anchor-on-enable/cadence-change
  - settings.NewService's required *time.Location parameter and WithClock test seam
  - cmd/server's fail-fast America/New_York zone resolution and "digest zone resolved" boot log line
affects: [22-02, 22-03, 22-04]

# Actuals (#2632)
actuals:
  tokens: 11458
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Calendar-based slot math: time.Date on an AddDate-stepped calendar date, never duration arithmetic, for DST-safe fixed-local-time scheduling"
    - "SQL CASE reading pre-UPDATE row values on the right-hand side to make a conditional re-anchor atomic with the triggering write"
    - "Unreferenced data-modifying CTE (WITH ... UPDATE ... RETURNING, never selected from) to run two independent table writes as one atomic statement without a Go transaction"
    - "Required positional zone/clock dependency (not a functional Option) mirroring notifier.New's SettingsReader precedent"

key-files:
  created:
    - internal/db/migrations/000009_digest_last_slot_at.up.sql
    - internal/db/migrations/000009_digest_last_slot_at.down.sql
    - internal/settings/slot.go
    - internal/settings/slot_test.go
  modified:
    - queries/notification_settings.sql
    - internal/db/sqlc/models.go
    - internal/db/sqlc/notification_settings.sql.go
    - internal/db/sqlc/querier.go
    - internal/db/migrate_test.go
    - internal/db/schema_version_test.go
    - internal/settings/settings.go
    - internal/settings/settings_test.go
    - internal/httpserver/settings_test.go
    - internal/notifier/notifier_test.go
    - cmd/server/main.go

key-decisions:
  - "Generated sqlc param field names recorded verbatim for plan 22-02: UpdateNotificationSettingsParams{DigestEnabled bool, DigestCadence string, ReanchorSlot pgtype.Timestamptz}; AckDigestBatchParams{Slot pgtype.Timestamptz, SentAt pgtype.Timestamptz, Ids []int64} -- all three timestamp fields are pgtype.Timestamptz, not *time.Time, matching this codebase's existing convention for emit_pointers_for_null_types against pgtype types (see NotificationSetting.DigestLastSentAt)."
  - "Task 3's tdd=true was not split into separate RED/GREEN commits -- delivered as one feat commit touching 6 interdependent files (settings.go, 3 test files, main.go). Documented as a deviation below rather than redone."

patterns-established:
  - "internal/settings/slot.go: MostRecentSlot/GraceFor/ZoneName -- the canonical DST-safe fixed-local-time slot calculation, reused unchanged by plan 22-02's due-check scheduler."

requirements-completed: [DGST-05, DGST-06, DGST-07, DGST-15]

coverage:
  - id: D1
    description: "Migration 000009 adds notification_settings.digest_last_slot_at as an additive nullable column; a from-scratch migration run lands on schema version 9"
    requirement: "DGST-15"
    verification:
      - kind: integration
        ref: "internal/db/migrate_test.go#TestRunMigrations_AppliesFromScratch"
        status: pass
      - kind: unit
        ref: "internal/db/schema_version_test.go#TestExpectedSchemaVersion"
        status: pass
    human_judgment: false
  - id: D2
    description: "queries/notification_settings.sql gains the D-14 re-anchor CASE on UpdateNotificationSettings and the new D-16 AckDigestBatch single-statement batch ack; sqlc output regenerated with no drift"
    requirement: "DGST-15"
    verification:
      - kind: other
        ref: "make sqlc-check"
        status: pass
    human_judgment: false
  - id: D3
    description: "settings.MostRecentSlot computes calendar-based daily/weekly digest slots correct across all four 2026/2027 DST transition dates, each yielding exactly one 00:05-local slot"
    requirement: "DGST-06"
    verification:
      - kind: unit
        ref: "internal/settings/slot_test.go#TestMostRecentSlot_DailyDSTTransitions"
        status: pass
      - kind: unit
        ref: "internal/settings/slot_test.go#TestMostRecentSlot_WeeklyDSTTransitions"
        status: pass
    human_judgment: false
  - id: D4
    description: "settings.GraceFor returns the D-12 catch-up grace window (12h daily / 48h weekly)"
    requirement: "DGST-06"
    verification:
      - kind: unit
        ref: "internal/settings/slot_test.go#TestGraceFor"
        status: pass
    human_judgment: false
  - id: D5
    description: "Enabling digest mode from off, or changing cadence while already on, re-anchors digest_last_slot_at to MostRecentSlot(clock(), cadence, loc); every other Update path (replay, turn-off, cadence-change-while-off) leaves it byte-identical"
    requirement: "DGST-05"
    verification:
      - kind: integration
        ref: "internal/settings/settings_test.go#TestService_UpdateReanchorsSlot_EnablingFromOff"
        status: pass
      - kind: integration
        ref: "internal/settings/settings_test.go#TestService_UpdateReanchorsSlot_CadenceChangeWhileOn"
        status: pass
      - kind: integration
        ref: "internal/settings/settings_test.go#TestService_UpdateLeavesSlotUntouched"
        status: pass
    human_judgment: false
  - id: D6
    description: "The GET/PUT /settings/notifications wire contract is byte-identical to before this plan -- exactly four keys, no slot field"
    requirement: "DGST-05"
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#TestSettings_RoundTrip"
        status: pass
    human_judgment: false
  - id: D7
    description: "cmd/server fails to boot (non-zero exit) when America/New_York cannot be resolved, with no fallback location on any path, and logs exactly one 'digest zone resolved' Info line on success"
    requirement: "DGST-07"
    verification:
      - kind: unit
        ref: "grep -n 'digest zone resolved' cmd/server/main.go"
        status: pass
    human_judgment: true
    rationale: "The fail-fast boot path itself (process exiting non-zero on an unresolvable zone) has no automated test in this plan -- plan 22-04's build-scan CI step is what actually boots the built image and greps its log for this exact line. This plan proves the code path exists and compiles/lints clean; end-to-end boot proof is deferred to 22-04 by design (see plan frontmatter)."

# Metrics
duration: 25min
completed: 2026-09-16
status: complete
---

# Phase 22 Plan 1: Digest Schema, Slot Math, and Zone Fail-Fast Summary

**Migration 000009 plus a D-14 re-anchoring UpdateNotificationSettings and a new D-16 AckDigestBatch, calendar-based (never duration-based) DST-safe slot math in internal/settings/slot.go, and a fail-fast America/New_York zone resolution at cmd/server boot.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-09-16T22:00:18Z
- **Completed:** 2026-09-16T22:13:31Z
- **Tasks:** 3
- **Files modified:** 15

## Accomplishments
- Migration 000009 adds `notification_settings.digest_last_slot_at` (nullable timestamptz, additive) with a paired down migration; sqlc regenerated cleanly with `AckDigestBatch` and the widened `UpdateNotificationSettingsParams`
- `internal/settings/slot.go`'s `MostRecentSlot`/`GraceFor` implement D-11's calendar-based slot math, proven correct across all four 2026/2027 US DST transition dates for both cadences via a full RED-then-GREEN TDD cycle
- `settings.Service.Update` now computes and supplies the re-anchor slot on every call; the SQL `CASE` decides whether it applies -- enabling digest mode or changing cadence while on re-anchors, every other path leaves the column byte-identical
- `cmd/server` blank-imports `time/tzdata`, resolves `America/New_York` once at boot with no fallback branch, and logs exactly one `"digest zone resolved"` Info line -- the literal contract plan 22-04's CI boot step will grep for

## Task Commits

1. **Task 1: Migration 000009 plus both notification_settings queries, regenerated and drift-checked** - `79b3ae7` (feat)
2. **Task 2: Slot math in internal/settings** - `78cdfdf` (test, RED) → `d13cc6a` (feat, GREEN)
3. **Task 3: Re-anchor on enable/cadence-change, injectable clock and location, and the boot-time zone fail-fast** - `2138db0` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE.md + ROADMAP.md)

_Note: Task 2 ran the full RED (compiling stub, all assertions fail) → GREEN (real implementation, all pass) cycle. Task 3 was also plan-annotated `tdd="true"` but was delivered as a single commit -- see Deviations below._

## Files Created/Modified
- `internal/db/migrations/000009_digest_last_slot_at.up.sql` / `.down.sql` - D-13's slot record column
- `queries/notification_settings.sql` - D-14 re-anchor CASE + new D-16 `AckDigestBatch` CTE
- `internal/db/sqlc/{models,querier,notification_settings.sql}.go` - regenerated, drift-checked clean
- `internal/db/migrate_test.go`, `internal/db/schema_version_test.go` - from-scratch schema version bumped 8→9
- `internal/settings/slot.go` - `ZoneName`, `MostRecentSlot`, `GraceFor`, `GraceDaily`, `GraceWeekly`
- `internal/settings/slot_test.go` - DST + cadence table tests
- `internal/settings/settings.go` - `Settings.DigestLastSlotAt`, `NewService(q, loc, opts...)`, `WithClock`, `Update`'s re-anchor
- `internal/settings/settings_test.go`, `internal/httpserver/settings_test.go`, `internal/notifier/notifier_test.go` - updated `NewService` call sites + new re-anchor/untouched-slot/4-key coverage
- `cmd/server/main.go` - `time/tzdata` blank import, zone fail-fast, `"digest zone resolved"` log line, `settingsStore` now constructed with the resolved zone

## Decisions Made
- Generated sqlc param field names, recorded verbatim per the plan's `<output>` requirement (plan 22-02 constructs these by name):
  - `UpdateNotificationSettingsParams{DigestEnabled bool, DigestCadence string, ReanchorSlot pgtype.Timestamptz}`
  - `AckDigestBatchParams{Slot pgtype.Timestamptz, SentAt pgtype.Timestamptz, Ids []int64}`
  - All three timestamp fields are `pgtype.Timestamptz`, not `*time.Time` as the plan's action text guessed -- `emit_pointers_for_null_types: true` does not affect `pgtype.Timestamptz` columns in this codebase's existing sqlc output (see `NotificationSetting.DigestLastSentAt`, unaffected by this plan). The generated names win per the plan's own instruction.
- Slot-boundary-bounded DST sweep tests: rather than sweeping each test date from local midnight to next midnight (which would straddle two distinct calendar dates' slots at the very first sample), `slot_test.go`'s sweeps are bounded `[this date's own slot instant, the next date's own slot instant)` -- a faithful, unambiguous proof of "exactly one slot per date" that the literal "00:00 to 00:00" reading in `22-CONTEXT.md`/the plan text would not itself produce. Documented inline in the test file.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Bumped `internal/db/schema_version_test.go`'s drift-alarm constant**
- **Found during:** Task 3's `make test` run
- **Issue:** `TestExpectedSchemaVersion` hardcodes the highest on-disk migration version as a deliberate drift alarm (per its own doc comment: "bump it in the same commit that adds a migration file"). Task 1 added migration 000009 but this file was not in Task 1's `<files>` list and was missed.
- **Fix:** Bumped `expectedSchemaVersionOnDisk` from 8 to 9.
- **Files modified:** `internal/db/schema_version_test.go`
- **Verification:** `go test ./internal/db/` green afterward; full suite green.
- **Committed in:** `2138db0` (Task 3 commit, since it was discovered during that task's `make test` run)

### Process Deviation (not a Rule 1-3 auto-fix)

**Task 3's `tdd="true"` RED/GREEN commits were not split.** Task 3 touched 6 interdependent files (`settings.go`, three test files whose `NewService` call sites all needed the same signature change to even compile, and `main.go`). Splitting this into a compiling-but-wrong RED stub (as Task 2 did) followed by a GREEN implementation was possible but would have required an artificial stub layer with no behavioral difference from Task 2's already-demonstrated RED/GREEN discipline. Delivered as one `feat(22-01)` commit instead, with tests written and passing together. All of Task 3's acceptance criteria and `<verify>` commands pass. Flagged here rather than silently deviating from the plan's `tdd="true"` marker.

---

**Total deviations:** 1 auto-fixed (Rule 1 bug), 1 process deviation (documented above).
**Impact on plan:** The auto-fix was necessary for the test suite to pass and is a direct, in-scope consequence of Task 1's migration. The process deviation has no functional impact -- all acceptance criteria and verify commands pass -- and is disclosed for TDD-gate-compliance transparency.

## Issues Encountered
None beyond the deviation documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Migration 000009, the revised `UpdateNotificationSettings`, and the new `AckDigestBatch` are committed and regenerated -- plan 22-02 can build the due-check scheduler and digest send directly against this schema and the recorded sqlc param names.
- `internal/settings/slot.go`'s `MostRecentSlot`/`GraceFor`/`ZoneName` are the canonical slot-math surface plan 22-02's `DigestScheduler` calls unchanged.
- `cmd/server`'s resolved `digestLoc` is available at the composition root for plan 22-02 to thread into the new scheduler construction.
- No blockers. `make sqlc-check`, `go vet ./...`, `golangci-lint run`, `make test` (without `-race`, per this Windows dev box's documented limitation -- see `.planning/WINDOWS.md`), and `make coverage-gate` (90.90%, well above the 80% floor) all pass clean on the final commit.

---
*Phase: 22-scheduled-digest-send*
*Completed: 2026-09-16*

## Self-Check: PASSED
- Created files verified present: `internal/db/migrations/000009_digest_last_slot_at.up.sql`, `.down.sql`, `internal/settings/slot.go`, `internal/settings/slot_test.go`.
- Commit hashes verified in `git log`: `79b3ae7`, `78cdfdf`, `d13cc6a`, `2138db0`.
- All task-level `<acceptance_criteria>` re-verified against the final commit (grep checks, `make sqlc-check`, `go build`/`go vet`, `golangci-lint run`, `make test`, `make coverage-gate`) -- all pass.
