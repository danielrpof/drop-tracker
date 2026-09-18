---
phase: 21-real-time-digest-mutual-exclusion
plan: 02

subsystem: notifications
tags: [go, notifier, digest-mode, settings, slog, postgres]

requires:
  - phase: 21-real-time-digest-mutual-exclusion
    provides: "plan 01's notifier.SettingsReader seam, the top-of-pass digest-mode gate, and readSettings' dbOpTimeout-bounded helper, all extended (not reworked) by this plan"
provides:
  - "A per-send readSettings re-read inside NotifyPending's loop, so a digest-mode toggle landing mid-pass stops delivery at the next send boundary instead of only at the top of the pass"
  - "observeDigestMode: the single emitter of a digest mode changed Info line, firing only when the observed mode differs from the last one recorded, wired at the three points a mode is ever observed (top-of-pass on, top-of-pass off with the flush count, mid-loop stop with the count left pending)"
  - "suppresses' freshness cutoff anchored to ev.CreatedAt instead of time.Now(), plus one day of D-02 slack, so time an event spends pending never ages it out of delivery"
affects: [22-digest-scheduler-sender, 23-digest-grouping-and-formatting]

actuals:
  tokens: 7309
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Deterministic k-th-call SettingsReader test doubles (flippingSettings/erroringFrom), extending fakeSettingsReader's existing fn/calls shape, for exact-count mid-pass toggle and read-error assertions instead of timing invariants"
    - "Single-emission-point transition logging (observeDigestMode) gated on a plain (non-atomic) last-observed-mode field pair, race-free under the existing notifying CAS lock's happens-before guarantee"
    - "A shared fail-closed helper (logSettingsReadFailure) so the top-of-pass gate and the per-send re-read apply byte-identical Warn/nil-return behavior instead of duplicating the branch"

key-files:
  created: []
  modified:
    - internal/notifier/notifier.go
    - internal/notifier/notifier_test.go
    - internal/notifier/suppress_test.go

key-decisions:
  - "The per-send re-read sits strictly between the suppression-ack branch (which continues, exempt from re-reads per D-04) and the Discord send -- a suppression ack issues no Discord request and so can neither duplicate a message nor violate the mode."
  - "lastDigestMode/lastDigestModeSet are plain bool fields, not atomics: they are read and written only by the goroutine holding the notifying CAS lock, and notifying.Store(false) at the end of one pass happens-before the next successful CompareAndSwap (sync/atomic sequential consistency), so no mutex is needed."
  - "observeDigestMode is the one place the transition line is emitted; NotifyPending's send/ack decisions never read n.lastDigestMode -- it is logging-only, wired at exactly three call sites (top-of-pass on/off, mid-loop stop)."
  - "suppresses' cutoff is ev.CreatedAt.Time when Valid, falling back to time.Now() only for the zero-value sqlc.Event a hand-built test literal can produce (the DB column is NOT NULL DEFAULT now(), so a real row can never hit that fallback), minus maxAgeDays and one extra day of D-02 slack for the gap between detection's captured now() and the DB's own now() at insert."

patterns-established:
  - "Per-send settings re-read: readSettings is called once at the top of the pass and again immediately before every individual Discord send, with the identical fail-closed treatment on error (D-04)."
  - "created_at-anchored freshness gate: the delivery-side suppression cutoff derives from the row's own detection timestamp, not the clock at delivery time (D-02) -- the pattern Phase 22's weekly-cadence digest send will depend on."

requirements-completed: [DGST-13, DGST-14]

coverage:
  - id: D1
    description: "A digest-mode toggle to on that lands mid-pass stops the pass at the next send boundary: the event already in flight is acked (the accepted residual, ADR 0002), every not-yet-sent event stays pending, and a stale-event suppression ack costs no settings read"
    requirement: DGST-13
    verification:
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_MidPass_StopsPassAtNextSendBoundary"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_MidPass_SuppressionAcksDoNotReRead"
        status: pass
    human_judgment: false
  - id: D2
    description: "A settings-read error observed mid-pass is indistinguishable from one observed at the top of the pass: nil return, exactly one distinctly-worded Warn, remaining rows left pending"
    requirement: DGST-13
    verification:
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_MidPass_ReadErrorStopsPassIdenticallyToTopOfPass"
        status: pass
    human_judgment: false
  - id: D3
    description: "Digest mode transitions are logged exactly once per change, not once per pass: a restart always logs the operative mode, the on-to-off line carries the flush count, a mid-loop stop logs the count left pending, and a failed read neither fabricates a transition nor swallows the next real one"
    requirement: DGST-13
    verification:
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_ModeTransitionLog_BootAlwaysLogsOperativeMode"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_ModeTransitionLog_SteadyStateLogsOnce"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_ModeTransitionLog_OnToOffCarriesFlushCount"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_ModeTransitionLog_MidLoopFlipLogsPendingCount"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_ModeTransitionLog_FailedReadDoesNotUpdateLastObserved"
        status: pass
    human_judgment: false
  - id: D4
    description: "An event's stale-release cutoff is anchored to its own created_at (plus one day of slack): an event detected 60 days ago with a release date just before that created_at is still delivered end-to-end through NotifyPending, a 2015 backlog row detected the same way is still suppressed, and the shared detection/notifier truth table (staleReleaseDate/gateCases) is unchanged"
    requirement: DGST-14
    verification:
      - kind: unit
        ref: "internal/notifier/suppress_test.go#TestNotifierSuppresses_WiresMaxAgeDays"
        status: pass
      - kind: integration
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_StaleAnchor_EventDetectedLongAgoStillDelivered"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-09-16
status: complete
---

# Phase 21 Plan 02: Per-Send Re-Read, Transition Logging, and the created_at Freshness Anchor Summary

**`NotifyPending` now re-reads digest mode before every individual Discord send (not just once per pass), logs exactly one line per observed mode change, and anchors its stale-release cutoff to each event's own `created_at` instead of the clock at delivery time.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-16T16:58:00Z
- **Completed:** 2026-09-16T17:17:18Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- A digest-mode toggle to "on" that lands mid-pass now stops delivery at the next send boundary: the event already in flight is acked (the accepted residual per ADR 0002), and every not-yet-sent event stays pending — proven with exact send/ack/pending counts against a deterministic k-th-call mode flip, not a timing invariant.
- Stale-event suppression acks issue no Discord request and are exempt from the per-send re-read (D-04) — proven by an exact reader-call-count assertion (1 top-of-pass read + one per genuinely fresh row).
- A settings-read error observed mid-pass is byte-for-byte identical to one observed at the top of the pass: `NotifyPending` returns nil, exactly one `skipping notify pass: digest settings read failed` Warn is logged, and the remaining rows stay pending.
- Digest mode holding steady across many poll cycles now produces exactly one `digest mode changed` Info line total, not one per pass — the whole point of D-01 replacing the earlier per-pass standdown line. A restart always logs the operative mode on its first successful read; the on-to-off line carries the exact count about to be flushed (taken from the list the pass already fetched, no new COUNT query); a mid-loop stop logs the count left pending; and a failed read updates nothing, so it can neither fabricate a transition nor swallow the next real one.
- The stale-release cutoff in `suppresses` now derives from each event's own `created_at` (minus `maxAgeDays` and one day of clock-skew slack) instead of `time.Now()`: an event detected 60 days ago with a release date just before that `created_at` is still delivered, both in isolation and end-to-end through a real `NotifyPending` pass against real Postgres. A 2015 backlog row detected the same way is still suppressed — the pre-fix backlog guard survives the re-anchor because its release date is old relative to its own detection time, not merely relative to "now."
- `staleReleaseDate` and the `gateCases` truth table it shares with `internal/detection/notifygate_test.go` are byte-for-byte unchanged — only the cutoff input moving, exactly as D-02 specified.

## Task Commits

Each task was committed atomically:

1. **Task 1: Re-read the mode before every send so a mid-pass toggle stops the pass** - `4666554` (feat)
2. **Task 2: Log one line when the observed digest mode changes, and only then** - `b6fc7aa` (feat)
3. **Task 3: Anchor the stale-release cutoff to the event's created_at** - `861d448` (fix)

**Plan metadata:** commit pending (docs: complete plan)

## Files Created/Modified
- `internal/notifier/notifier.go` - per-send `readSettings` re-read in `NotifyPending`'s loop; shared `logSettingsReadFailure` helper; `lastDigestMode`/`lastDigestModeSet` fields; `observeDigestMode` transition-logging method wired at its three call sites; `suppresses`' cutoff re-anchored to `ev.CreatedAt` with the D-02 slack
- `internal/notifier/notifier_test.go` - `flippingSettings`/`erroringFrom` k-th-call test doubles; three mid-pass tests (stop boundary + residual, suppression-exemption call count, mid-pass read error); `modeTransitionRecord`/`decodeModeTransitionRecords` log-assertion helpers; five mode-transition-log tests; `insertPendingEventBackdated` helper; the end-to-end `created_at`-anchor delivery test
- `internal/notifier/suppress_test.go` - `TestNotifierSuppresses_WiresMaxAgeDays` rebuilt around one fixed anchor instant with an explicit `CreatedAt` per case, plus the new 60-days-pending-delivered / backlog-still-suppressed / slack-boundary cases; `pgtype` import added

## Decisions Made
- The per-send re-read is placed strictly between the suppression-ack branch and the Discord send, never inside the suppression branch itself — a suppression ack makes no Discord request, so re-reading there would add one DB round trip per stale row on exactly the multi-hundred-row backlog pass this codebase already has to handle cheaply.
- `lastDigestMode`/`lastDigestModeSet` are plain (non-atomic) fields rather than `atomic.Bool`s: they are read and written only by the goroutine currently holding the `notifying` CAS lock, and `sync/atomic`'s sequential consistency means `notifying.Store(false)` at the end of one pass happens-before the next successful `CompareAndSwap` — a mutex would be redundant.
- `suppresses`' cutoff falls back to `time.Now()` only for the structurally-unreachable zero-value `sqlc.Event.CreatedAt` a hand-built test literal can produce; the real `events.created_at` column is `NOT NULL DEFAULT now()`, so a genuine row can never hit that branch.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered
- Windows dev-machine limitation (pre-existing, documented in `.planning/WINDOWS.md`): `make test` invokes `go test ./... -race`, and `-race` cannot build on this machine (mingw64 `cc1.exe` cannot execute). Substituted the same `go test ./...` invocation without `-race` (same coverage flags, same `-coverprofile=coverage.out`), matching the precedent set in plan 21-01 and Phase 11.1's plan 04. All packages passed; `make coverage-gate` then ran against the resulting `coverage.out` and reported 90.83% (required 80%). CI runs the real `-race` suite on ubuntu-latest per this phase's D-07 note.

## Next Phase Readiness
- Every outbox send now serializes on a per-send digest-mode check, exactly what ADR 0002 requires before Phase 22's digest-send method can safely share the same `notifying` lock.
- The `created_at`-anchored freshness gate is the mechanism Phase 22's weekly cadence depends on: an event that waited a full week in the outbox will still be judged fresh relative to when it was detected, not relative to the clock at flush time.
- No blockers. Per the ROADMAP's "Deploy sequencing" note, this phase's branch is not merged to `main` until Phase 22 is also ready — no PR was opened from this plan.

## Self-Check: PASSED

All 3 files-modified paths and this SUMMARY.md confirmed present on disk; all 3 task commit hashes (`4666554`, `b6fc7aa`, `861d448`) confirmed present in `git log --oneline --all`.

---
*Phase: 21-real-time-digest-mutual-exclusion*
*Completed: 2026-09-16*
