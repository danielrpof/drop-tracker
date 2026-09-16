---
phase: 21-real-time-digest-mutual-exclusion
plan: 01

subsystem: notifications
tags: [go, notifier, digest-mode, settings, slog, postgres]

requires:
  - phase: 20-digest-settings-operator-control
    provides: internal/settings.Store (Get/Update over the singleton notification_settings row), the seam this plan's SettingsReader reads through
provides:
  - "internal/notifier.SettingsReader: the narrow digest-mode gate seam, required (not optional) on notifier.New/Select"
  - "A dbOpTimeout-bounded readSettings helper, called as NotifyPending's first decision after the CAS guard succeeds and before listUnnotified"
  - "Fail-closed posture on a settings-read error: one distinct Warn, nil return, zero sends/acks; silent on ctx cancellation"
  - "cmd/server/main.go wiring the same settingsStore instance into notifier.Select that httpserver.WithSettings already uses"
affects: [22-digest-scheduler-sender, 23-digest-grouping-and-formatting]

actuals:
  tokens: 9915
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Consumer-declared narrow interface seam (SettingsReader), mirroring the existing Sender/Sink pattern in the same file"
    - "Required positional constructor argument instead of a functional Option, deliberately breaking the file's own Option convention where a forgotten option would silently ship an ungated notifier (D-05)"
    - "Bounded read helper (readSettings) following the exact listUnnotified/markNotified shape"

key-files:
  created: []
  modified:
    - internal/notifier/notifier.go
    - internal/notifier/notifier_test.go
    - internal/notifier/suppress_test.go
    - internal/notifier/timeout_test.go
    - cmd/server/main.go
    - internal/detection/detector_test.go

key-decisions:
  - "SettingsReader declared in internal/notifier (not imported from settings.Store) and returns the full settings.Settings, not just a bool, because Phase 22 needs DigestCadence/DigestLastSentAt from the same read (D-05)"
  - "erroringSettings test double deferred from Task 1 into Task 3 (where it is first used) -- an unused top-level function fails golangci-lint's unused check, and CLAUDE.md's Definition of Done gate runs before every commit, not just at the plan's final verify step"

patterns-established:
  - "Digest-mode gate: read settings under dbOpTimeout immediately after the notifying CAS succeeds, before any row is fetched -- never a post-hoc filter on already-listed rows"

requirements-completed: [DGST-13, DGST-14]

coverage:
  - id: D1
    description: "Digest mode on: a notify pass over pending rows of all three event types (new_release, guest_feature, deluxe_change) issues zero Discord requests and leaves every row pending"
    requirement: DGST-13
    verification:
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_DigestModeOn_SendsNothingAndLeavesRowsPending"
        status: pass
    human_judgment: false
  - id: D2
    description: "Digest mode off: notifier behavior is byte-for-byte v1.4 -- the pre-existing notifier test suite passes unmodified apart from the new constructor argument"
    verification:
      - kind: integration
        ref: "go test ./internal/notifier/ ./internal/detection/ -count=1 (full pre-existing suite, green)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Toggling digest mode on then off delivers every queued event exactly once, in ListUnnotified order, on the next pass on the SAME running Notifier instance (no restart, no per-pass caching)"
    requirement: DGST-14
    verification:
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_DigestToggleRoundTrip_QueuedEventsFlushExactlyOnce"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_DigestOffFlush_OrderNoDuplication"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_DigestOffFlush_IdempotentSecondPass"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_DigestRealSettingsStore_TogglesWithoutRestart"
        status: pass
    human_judgment: false
  - id: D4
    description: "A settings-read error fails closed: nil return, exactly one distinctly-worded Warn, zero sends, zero acks; a ctx-cancelled read is silent (zero Warns); a wedged read is bounded by dbOpTimeout and releases the notifying lock"
    verification:
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_SettingsReadFails_FailsClosedWithOneWarnAndNoSends"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_SettingsReadCtxCancelled_NoWarnLogged"
        status: pass
      - kind: integration
        ref: "internal/notifier/timeout_test.go#TestNotifyPending_SettingsReadUnresponsive_ReturnsInsteadOfWedgingLock"
        status: pass
    human_judgment: false

duration: ~22min
completed: 2026-09-16
status: complete
---

# Phase 21 Plan 01: Digest-Mode Gate on the Real-Time Notify Pass Summary

**`notifier.SettingsReader` as a required constructor argument gates `NotifyPending` on the singleton digest-mode row, read under `dbOpTimeout` before any pending event is listed, failing closed on error.**

## Performance

- **Duration:** ~22 min
- **Started:** 2026-09-16T16:23:38Z
- **Completed:** 2026-09-16T16:45:08Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- With digest mode on, a real notify pass over pending `new_release`/`guest_feature`/`deluxe_change` rows sends zero Discord requests and leaves every row's `notified_at` NULL (DGST-13), proven end-to-end against real Postgres.
- With digest mode off, the notify path is unchanged from v1.4: the entire pre-existing notifier test suite (spacing, retries, mark-notified failure handling, cross-cycle recovery, the CAS guard) passes unmodified apart from threading the new argument through every call site.
- Events queued while digest mode was on are delivered exactly once, in `ListUnnotified` order, by the next pass on the same `Notifier` instance once the mode reads off — proven both with a fake reader and with the real `settings.Service` over an isolated pool (DGST-14).
- A settings-read error fails closed: the pass stops before any row is listed, logs exactly one `skipping notify pass: digest settings read failed` Warn, and returns nil so `poller.go`'s log-and-continue call site never mistakes it for a delivery failure. A read failure caused by the caller's own context cancellation logs nothing.
- Every settings read is bounded by the existing `dbOpTimeout`, using the same helper shape as `listUnnotified`/`markNotified` — a wedged read cannot park the pass or strand the `notifying` sender lock.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end digest gate — settings row through the notify pass to zero Discord sends** - `2353138` (feat)
2. **Task 2: Prove the off-flush — events queued during digest mode drain through the ordinary path** - `c863466` (test)
3. **Task 3: Prove the fail-closed posture and the bounded read** - `553d6d4` (test)

**Plan metadata:** commit pending (docs: complete plan)

## Files Created/Modified
- `internal/notifier/notifier.go` - `SettingsReader` interface + compile-time assertion, `settingsReader` field, required constructor argument on `New`/`Select`, `readSettings` bounded helper, the top-of-pass gate in `NotifyPending`
- `internal/notifier/notifier_test.go` - `fakeSettingsReader`/`stubSettings`/`erroringSettings`/`toggleableSettings` doubles, `insertPendingEventTyped`, the tracer test, the off-flush suite (4 tests), the fail-closed suite (2 tests), `logRecord`/`decodeLogRecords` JSON log-assertion helper, and the new argument threaded through all 16 pre-existing `notifier.New`/`Select` call sites
- `internal/notifier/timeout_test.go` - `settingsDigestOff` and `wedgingSettingsReader` local stubs (whitebox package, unreachable from `notifier_test.go`'s doubles), the bounded-settings-read wedge test, and the new argument on the file's 3 pre-existing `New` calls
- `internal/notifier/suppress_test.go` - the new argument (`nil`) on the file's 2 pre-existing `New` calls, which never call `NotifyPending`
- `cmd/server/main.go` - `notifier.Select` now passes `settingsStore`, the same instance `httpserver.WithSettings` already reads/writes through
- `internal/detection/detector_test.go` - `stubDigestOff` local stub and the new argument on this package's one `notifier.New` call

## Decisions Made
- `SettingsReader` returns the full `settings.Settings` rather than a bool, since Phase 22's digest sender needs `DigestCadence`/`DigestLastSentAt` from the same read (D-05).
- `SettingsReader` is a required positional parameter on `New`/`Select`, never a functional `Option` — deliberately breaking this file's own `Option` convention, because a forgotten option would compile, pass every test, and silently ship an ungated notifier.
- The fail-closed branch distinguishes a caller-shutdown cancellation (`ctx.Err() != nil` and the error wraps `context.Canceled`/`context.DeadlineExceeded`) from an ordinary `dbOpTimeout` firing on a live context — only the latter logs a Warn.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Deferred `erroringSettings` test double from Task 1 to Task 3**
- **Found during:** Task 1, running `golangci-lint run` before committing
- **Issue:** The plan's Task 1 action text asks for both `stubSettings` and `erroringSettings` constructors to be added alongside `fakeSettingsReader`. `erroringSettings` is not called by any Task 1 test (it is first used by Task 3's fail-closed tests), so golangci-lint's `unused` linter flags it as an unused top-level function. Project CLAUDE.md's Definition of Done requires `golangci-lint run` clean before every commit, not only at the plan's own final verify step — this is a hard constraint that takes precedence over the plan's task-grouping text.
- **Fix:** Added `stubSettings` in Task 1 (used immediately by the tracer test and 16 call-site updates); added `erroringSettings` in Task 3, right where its first caller (`TestNotifyPending_SettingsReadFails_FailsClosedWithOneWarnAndNoSends`) is introduced. No behavior difference — same function, same signature, committed one task later.
- **Files modified:** internal/notifier/notifier_test.go
- **Verification:** `golangci-lint run ./...` reports 0 issues at every task's commit point.
- **Committed in:** `2353138` (stubSettings, Task 1), `553d6d4` (erroringSettings, Task 3)

---

**Total deviations:** 1 auto-fixed (1 blocking — a lint-gate ordering constraint, not a logic change)
**Impact on plan:** No scope change; the same two test-double constructors exist, split across two commits instead of one so every commit independently clears the lint gate.

## Issues Encountered
- Windows dev-machine limitation (pre-existing, documented in `.planning/WINDOWS.md`): `make test` invokes `go test ./... -race`, and `-race` cannot build on this machine (mingw64 `cc1.exe` cannot execute). Substituted the same `go test ./...` invocation without `-race` (same coverage flags, same `-coverprofile=coverage.out`), matching the precedent set in Phase 11.1's plan 04. All 24 packages passed; `make coverage-gate` then ran against the resulting `coverage.out` and reported 90.83% (required 80%). CI runs the real `-race` suite on ubuntu-latest per `.planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md`'s D-07 note.

## Next Phase Readiness
- The `notifying` CAS guard is now provably the one point where digest mode is consulted before any send, which is exactly what ADR 0002 requires Phase 22's digest-send method to share.
- `notifier.New`/`notifier.Select`'s constructor signature is stable for Phase 22 to build on — the digest sender will take the same `SettingsReader` and can reuse `readSettings` directly.
- No blockers. Per the ROADMAP's "Deploy sequencing" note, this phase's branch is not merged to `main` until Phase 22 is also ready — no PR was opened from this plan.

## Self-Check: PASSED

All 6 files-modified paths and the SUMMARY.md itself confirmed present on disk; all 3 task commit hashes (`2353138`, `c863466`, `553d6d4`) confirmed present in `git log --oneline --all`.

---
*Phase: 21-real-time-digest-mutual-exclusion*
*Completed: 2026-09-16*
