---
phase: 22-scheduled-digest-send
plan: 04
subsystem: digest-scheduling
tags: [dst, timezone, scheduler, ci, tzdata, notifier, testing]

# Dependency graph
requires:
  - phase: 22-02
    provides: DigestScheduler/NewDigestScheduler/Start/Stop/WithTickSource, Notifier.SendDigestIfDue implementing D-17's fixed sequence
  - phase: 22-03
    provides: buildDigestEmbed's escaping/collation -- exercised incidentally by every send in this plan's tests, unchanged by this plan
provides:
  - "internal/notifier/digest_schedule_test.go -- the DST and grace-window behavioural matrix, driving a real DigestScheduler over a real Notifier through an injected mutable clock and manually-driven tick source (never SendDigestIfDue directly)"
  - "the build-scan boot step -- proves the shipped Alpine image resolves America/New_York against the exact image this CI run built, closing DGST-07 end-to-end (code path proven in 22-01, boot proven here)"
affects: []

# Actuals (#2632)
actuals:
  tokens: 7916
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "signalingSink: a notifier.Sink wrapper that signals a done channel after every SendDigestIfDue call returns, letting a test drive a real *DigestScheduler through hundreds of steps synchronised on channel receives -- never a wall-clock sleep, even though most steps are not-due and do not call Sender.Send."
    - "mutableClock: a mutex-guarded now-source set immediately before each tick is delivered (or before Start), relying on the tick channel's own send/receive (or the go statement's happens-before edge for the immediate first check) to make the new clock value visible to the check that reads it -- no atomic needed."
    - "seedNotificationSettings: a raw SQL UPDATE against the singleton notification_settings row, used only to establish an 'already handled up to this slot' starting state directly -- deliberately bypassing settings.Service.Update's own D-14 re-anchor logic, which would recompute its own slot instead of accepting the test's chosen one."
    - "insertPendingEventTypedRaw: a t-free twin of insertPendingEventTyped for use inside a fakeSender callback, which runs on the DigestScheduler's own background goroutine -- calling t.Fatalf there would violate testing's single-goroutine FailNow contract; errors are returned and checked back on the test goroutine after the channel-synchronised sweep step completes."

key-files:
  created:
    - internal/notifier/digest_schedule_test.go
  modified:
    - .github/workflows/full-pipeline.yml

key-decisions:
  - "Task 1's sweep helper drives a *real* DigestScheduler (not a fake), wrapping the real *Notifier in a signalingSink -- the plan's key_links explicitly required the sweep to go through DigestScheduler's injected clock and tick source rather than calling SendDigestIfDue directly, so the scheduler's own Start-immediate-check and tick-dispatch logic is exercised, not bypassed."
  - "For the sweep tests (Task 1) the SettingsReader is the real settings.Service reading the live DB row, so the ack's write to digest_last_slot_at is visible to the *next* check in the same sweep -- a fixed fakeSettingsReader (as used elsewhere in this package) would have kept reporting a stale slot for all 276-2016 subsequent steps. The single-shot grace tests (Task 2, excluding restart-catch-up and nothing-lost) reuse the existing digestSettingsReader fake instead, since each of those tests makes only one or two direct SendDigestIfDue calls and doesn't need live-DB continuity."
  - "The 2026-11-01 fall-back slot-record proof inserts its second event inside the fake Sender's own callback (triggered on the first Send call) rather than pausing the sweep mid-flight to insert externally -- the insert lands strictly after sentIDs was already computed from the outbox read that preceded the Send call, so it cannot accidentally get swept into the same ack regardless of timing."
  - "The zone-resolution CI proof boots drop-tracker:scan (the image build-scan just built) against a throwaway Postgres, mirroring n1-boot's bring-up pattern with distinct container names (drop-tracker-zone / drop-tracker-zone-pg) so the two jobs cannot collide on a shared runner; no migration step is added since the container runs its own boot migrations against the empty database."

requirements-completed: [DGST-05, DGST-06, DGST-07, DGST-15]

coverage:
  - id: D1
    description: "The daily cadence produces exactly one Discord send per calendar day across all four 2026/2027 America/New_York DST transition dates (2026-03-08, 2026-11-01, 2027-03-14, 2027-11-07), with digest_last_slot_at advancing to that day's 00:05 slot and digest_last_sent_at non-NULL afterward"
    requirement: DGST-06
    verification:
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_DST_Daily"
        status: pass
    human_judgment: false
  - id: D2
    description: "The weekly cadence produces exactly one send per Friday-to-Friday local week across each of the four weeks containing a DST transition date, with digest_last_slot_at advancing to that week's Friday 00:05 slot"
    requirement: DGST-06
    verification:
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_DST_Weekly"
        status: pass
    human_judgment: false
  - id: D3
    description: "On the 25-hour 2026-11-01 fall-back day, inserting a second pending event the instant the first send is observed still yields exactly one send for the day, and the second event stays un-acked -- proving the slot record, not an empty outbox, is what prevents the second fire"
    requirement: DGST-06
    verification:
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_DST_Daily_FallBack_SlotRecordNotEmptyOutboxPreventsSecondSend"
        status: pass
    human_judgment: false
  - id: D4
    description: "A scheduler restarted with its clock reading 2 hours past an unhandled daily slot sends exactly once from its immediate first check, before any tick -- the restart catch-up delivering ROADMAP success criterion 3"
    requirement: DGST-05
    verification:
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_Catchup_RestartInsideGrace"
        status: pass
    human_judgment: false
  - id: D5
    description: "The grace-window boundary is exactly inclusive for both cadences: now-slot == grace (12h daily / 48h weekly) still sends, one minute past does not send, logs exactly one Warn, and leaves both settings columns byte-identical to their pre-check values"
    requirement: DGST-05
    verification:
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_Grace_InsideWindow_SlotPlus11h59m"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_Grace_Boundary_SlotPlus12h00m_StillSends"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_Grace_Boundary_SlotPlus12h01m_DoesNotSend"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_Grace_WeeklyBoundary_SlotPlus48h00m_StillSends"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_Grace_WeeklyBoundary_SlotPlus48h01m_DoesNotSend"
        status: pass
    human_judgment: false
  - id: D6
    description: "Events pending through a grace-expired skip are not dropped: once the next daily slot arrives, a single check sends exactly once and acks both the originally-pending event and one accumulated during the skipped period"
    requirement: DGST-15
    verification:
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_Grace_NothingLost_CarriesForwardToNextSlot"
        status: pass
    human_judgment: false
  - id: D7
    description: "A not-due check is silent at Info level and above (Debug only) and writes neither settings column"
    requirement: DGST-15
    verification:
      - kind: unit
        ref: "internal/notifier/digest_schedule_test.go#TestDigestScheduler_NotDue_Quiet_NoLogsNoWrites"
        status: pass
    human_judgment: false
  - id: D8
    description: "build-scan boots drop-tracker:scan -- the exact image this CI run just built, never a ghcr.io/ pull -- against a throwaway Postgres, and turns the pipeline red unless the boot log carries \"msg\":\"digest zone resolved\" and \"zone\":\"America/New_York\"; a container that dies before logging that line fails immediately rather than timing out silently"
    requirement: DGST-07
    verification: []
    human_judgment: true
    rationale: "This is a CI-runtime proof (a container boot inside a GitHub Actions job) that cannot be exercised by a local go test run -- Go consults the host's own zoneinfo before the binary's embedded time/tzdata, so the same test would pass on this dev machine or any zoneinfo-carrying runner regardless of whether the embedded database is actually present. The step's shape was verified locally (YAML parses, needs unchanged, no new uses:, literals present, steps sit inside build-scan between the job key and release:), but the live green/red behavior is only provable by an actual CI run of this workflow -- deferred to the first push that reaches build-scan."

# Metrics
duration: 20min
completed: 2026-09-16
status: complete
---

# Phase 22 Plan 4: DST and Grace-Window Matrix, Alpine Zone Boot Proof Summary

**A real DigestScheduler swept in 5-minute steps across all four 2026/2027 America/New_York DST transitions proves exactly one digest per period for both cadences, a restart/grace-window matrix proves bounded catch-up with nothing lost, and a new build-scan CI step boots the shipped image to assert it resolves the zone it schedules against.**

## Performance

- **Duration:** 20 min
- **Started:** 2026-09-16T21:34:00-05:00 (approx.)
- **Completed:** 2026-09-16T21:53:35-05:00
- **Tasks:** 3
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments
- `internal/notifier/digest_schedule_test.go` drives a real `*DigestScheduler` over a real `*Notifier` through a `mutableClock` and a manually-driven tick source (`signalingSink` synchronising every step on a channel, never a sleep), sweeping all four DST transition dates in both 2026 and 2027 for the daily cadence (276/300-step 23h/25h days) and the weekly cadence (2004/2028-step Friday-to-Friday weeks) -- every sweep asserts exactly one send and the correct advanced `digest_last_slot_at`
- A dedicated fall-back-day subtest inserts a second pending event the instant the first send is observed (inside the fake Sender's own callback) and continues sweeping to day's end, proving the slot record -- not an empty outbox -- prevents the second fire, closing the specific gap CONTEXT.md's `<specifics>` block calls out
- The grace-window matrix covers restart catch-up (send from the immediate first check, before any tick), the inclusive 12h/48h boundary on both sides, a nothing-lost carry-forward through a grace-expired skip, and a silent not-due path (Debug-only, zero column writes)
- `build-scan` gained three new steps (`Start throwaway Postgres for the zone boot check`, `Boot the built image and assert it resolves the digest zone`, `Clean up zone boot check containers`) between the provenance-verification step and the Trivy scan, booting `drop-tracker:scan` -- the image this same job just built -- against a throwaway Postgres and polling its boot log for up to 60s for the exact `"msg":"digest zone resolved"` / `"zone":"America/New_York"` pair, failing immediately (not timing out silently) if the container dies first

## Task Commits

Each task was committed atomically:

1. **Task 1: The DST matrix -- exactly one send per period across both transitions, both cadences, both years** - `fa7e4d2` (test)
2. **Task 2: The grace-window matrix -- catch up after a brief outage, refuse after a long one, lose nothing either way** - `c301ebd` (test)
3. **Task 3: Prove the shipped Alpine image resolves America/New_York, in CI, against the image this run just built** - `d09b8cf` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE.md + ROADMAP.md)

_Note: both Task 1 and Task 2 extend the same file (`internal/notifier/digest_schedule_test.go`); each was written, verified against its own full `<verify>` chain (build/vet/lint, the task's own test filter, the broader `internal/notifier`/`internal/detection`/`internal/settings` suite, and `make coverage-gate`), and committed independently before the next task began, so `digest_schedule_test.go`'s two commits are each fully self-contained and individually revertable._

## Files Created/Modified
- `internal/notifier/digest_schedule_test.go` - DST matrix (Task 1: `TestDigestScheduler_DST_Daily`, `TestDigestScheduler_DST_Daily_FallBack_SlotRecordNotEmptyOutboxPreventsSecondSend`, `TestDigestScheduler_DST_Weekly`, plus the `mutableClock`/`signalingSink`/`sweep`/`seedNotificationSettings`/`insertPendingEventTypedRaw` helpers) and grace-window matrix (Task 2: `TestDigestScheduler_Catchup_RestartInsideGrace`, `TestDigestScheduler_Grace_InsideWindow_SlotPlus11h59m`, `TestDigestScheduler_Grace_Boundary_SlotPlus12h00m_StillSends`, `TestDigestScheduler_Grace_Boundary_SlotPlus12h01m_DoesNotSend`, `TestDigestScheduler_Grace_WeeklyBoundary_SlotPlus48h00m_StillSends`, `TestDigestScheduler_Grace_WeeklyBoundary_SlotPlus48h01m_DoesNotSend`, `TestDigestScheduler_Grace_NothingLost_CarriesForwardToNextSlot`, `TestDigestScheduler_NotDue_Quiet_NoLogsNoWrites`)
- `.github/workflows/full-pipeline.yml` - three new steps inside the existing `build-scan` job proving the shipped image resolves `America/New_York`; `needs:` and every existing step untouched

## Decisions Made
- Both DST-matrix subtests and every grace-window test seed `notification_settings` via a new `seedNotificationSettings` raw-SQL helper rather than `settings.Service.Update`, since `Update`'s own D-14 re-anchor logic would recompute and overwrite the exact "already handled up to this slot" starting state each test needs to establish deliberately.
- The sweep tests use the real `settings.Service` (live DB reads) as `SendDigestIfDue`'s `SettingsReader` so a send's ack is visible to the very next check in the same sweep; the simpler single-shot grace tests keep using the package's existing `digestSettingsReader` fake, since they make only one or two direct calls and don't need that live-DB continuity.
- `insertPendingEventTypedRaw` (a `t`-free twin of the existing `insertPendingEventTyped`) exists solely because the fall-back slot-record proof needs to insert a row from inside a fake `Sender` callback, which executes on the `DigestScheduler`'s own background goroutine -- calling `t.Fatalf` there would violate `testing`'s single-goroutine `FailNow` contract, so the insert returns an error checked back on the test goroutine after the channel-synchronised sweep step completes.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. `go test -race` remains unusable on this Windows dev box (pre-existing, documented, waived limitation -- see `.planning/WINDOWS.md` and every prior phase's SUMMARY since 05-UAT). Substituted `go test ./... -count=1 -p 1 -coverprofile=coverage.out -coverpkg=$(COVER_PKGS)` (the same command `test-integration` runs, minus `-race`) for `make coverage-gate`'s input -- full 24-package suite green (0 failures), `make coverage-gate` reported 91.27%, well above the 80% floor. The full DST + grace-window suite (17 test functions across both tasks) runs in about 14 seconds against local Docker Postgres -- driven entirely by real DB round-trips per 5-minute step, never a wall-clock sleep.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All four of this plan's requirements (DGST-05, DGST-06, DGST-07, DGST-15) now have direct automated proof, closing Phase 22's ROADMAP success criteria 3 and 4.
- DGST-07's CI-side proof (the `build-scan` boot step) is verified locally for shape (YAML parses, `needs:` unchanged, no new `uses:`, literals present, steps correctly placed inside `build-scan`) but its actual green/red behavior against a live GitHub Actions runner is unobserved until the next push reaches `build-scan` -- flagged as `human_judgment: true` in the coverage block above (D8), matching the precedent 22-01-SUMMARY.md and 22-02-SUMMARY.md already recorded for this same zone-resolution proof.
- This is the last plan in Phase 22 (wave 4 of 4). No blockers. `go build ./...`, `go vet ./...`, `golangci-lint run` (0 issues), the full `go test ./... -count=1` suite (24 packages, 0 failures, `-race` substituted per the documented Windows limitation), `make coverage-gate` (91.27%), and `make sqlc-check` all pass clean on the final commit. `git diff --exit-code main...HEAD -- internal/poller web/ internal/discord internal/config` shows only Phase 21's pre-existing `DigestSettings.tsx` helper-text commit (`c3e5c14`), already documented as not a regression by 22-02-SUMMARY.md and 22-03-SUMMARY.md and confirmed here via `git log` to predate this plan.

---
*Phase: 22-scheduled-digest-send*
*Completed: 2026-09-16*

## Self-Check: PASSED
- Created file verified present: `internal/notifier/digest_schedule_test.go`.
- Modified file verified present: `.github/workflows/full-pipeline.yml` (new steps confirmed via `grep -n` between the `build-scan:` and `release:` job keys).
- Commit hashes verified in `git log`: `fa7e4d2`, `c301ebd`, `d09b8cf`.
- All task-level `<acceptance_criteria>` re-verified against the final commits (grep checks, `go build`/`go vet`, `golangci-lint run`, `go test` per-task-filter and full-suite, `make coverage-gate`, `make sqlc-check`, YAML parse, `needs:`/`uses:` diff checks) -- all pass.
