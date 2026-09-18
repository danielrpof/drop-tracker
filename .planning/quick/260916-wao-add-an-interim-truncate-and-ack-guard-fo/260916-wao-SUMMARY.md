---
phase: quick/260916-wao
plan: 01
subsystem: notifier
tags: [go, discord, digest, truncation, notifier]

requires:
  - phase: 22-scheduled-digest-send
    provides: buildDigestEmbed, SendDigestIfDue, the digest outbox ack path
provides:
  - Whole-Description cap on buildDigestEmbed (discordDescriptionLimit, 4096 runes)
  - Line-boundary truncation with an operator-visible "... N more events" note
  - Proof that SendDigestIfDue acks every event in an oversized batch (rendered or truncated-out)
affects: [22-scheduled-digest-send, notifier]

actuals:
  tokens: 4691
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Segment-then-assemble digest rendering: buildDigestEmbed builds one string segment per event (digestLine) instead of writing into a shared strings.Builder, so a downstream assembler (assembleDescription) can truncate on a whole-segment boundary."
    - "Exact cumulative-rune-count scan for truncation: assembleDescription precomputes a prefix-sum array of segment rune counts and scans downward for the largest prefix that fits alongside the trailing note, checked exactly against the limit rather than estimated."

key-files:
  created: []
  modified:
    - internal/notifier/digest_format.go
    - internal/notifier/digest_format_test.go
    - internal/notifier/digest.go
    - internal/notifier/digest_test.go

key-decisions:
  - "Kept truncationNoteReserve (40 runes) as a builder-capacity preallocation hint only, not a second hard limit -- the real invariant (prefix-runes + note-runes <= discordDescriptionLimit) is always checked exactly per the design contract's explicit instruction."
  - "digestLine takes eventType as an explicit parameter (not read from ev.EventType) to match the plan's locked design contract, even though the two always agree at call sites today."

patterns-established: []

requirements-completed: [22-REVIEW-CR-01, 22-SECURITY-T-22-15]

coverage:
  - id: D1
    description: "buildDigestEmbed's whole Description never exceeds Discord's 4096-rune limit; an oversized batch truncates at a full line boundary and appends a human-readable truncation note"
    requirement: "22-SECURITY-T-22-15"
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestAssembleDescription_ExceedsLimitTruncatesOnSegmentBoundary"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_OversizedBatchTruncatesAndReconciles"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestAssembleDescription_FitsUnderLimitReturnsJoinUnchanged"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_SmallOrdinaryBatchNoTruncationNote"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every event id in an oversized digest batch -- rendered or truncated-out -- is acked in the same successful send, so digest_last_slot_at/digest_last_sent_at advance and the backlog cannot retry the same batch forever"
    requirement: "22-SECURITY-T-22-15"
    verification:
      - kind: integration
        ref: "internal/notifier/digest_test.go#TestSendDigestIfDue_OversizedBatch_TruncatesDescriptionAndAcksEveryEvent"
        status: pass
    human_judgment: false
  - id: D3
    description: "No regression to the 100-rune title cap, markdown escaping, heading grouping/ordering, or collated sort for any digest that already fit under the limit -- buildDigestEmbed's exported signature is unchanged"
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go (all 16 pre-existing test functions, run unmodified against the new implementation)"
        status: pass
    human_judgment: false

duration: ~30min
completed: 2026-09-16
status: complete
---

# Quick Task 260916-wao: Interim Truncate-and-Ack Guard for Oversized Digests Summary

**Whole-Description cap on `buildDigestEmbed` (4096 runes, line-boundary truncation + "... N more events" note) plus proof that `SendDigestIfDue` still acks every event in an oversized batch, closing T-22-15/CR-01's unbounded-outbox retry loop.**

## Performance

- **Duration:** ~30 min
- **Completed:** 2026-09-16
- **Tasks:** 2 (both tracer/auto, tdd=true)
- **Files modified:** 4

## Accomplishments
- `buildDigestEmbed` now builds one string segment per event via the extracted `digestLine`, and `assembleDescription` joins them with an exact whole-Description cap: byte-identical output when everything fits, otherwise the largest whole-segment prefix plus a `truncationNote` ("... N more events", singular/plural correct), never cutting mid-line.
- New unit tests directly cover `digestLine`, `assembleDescription`, and `truncationNote`, plus two `buildDigestEmbed`-level tests (small batch stays note-free; a 200-event synthetic batch truncates and reconciles rendered-line-count + omitted-count = total events).
- Proved end-to-end against real Postgres that `SendDigestIfDue` acks every event id in a 200-event oversized batch -- including the ones truncated out of the rendered Description -- in exactly one send, and both `digest_last_slot_at`/`digest_last_sent_at` advance. This is the assertion that closes the retry-loop threat: `digest.go` needed zero logic changes because its ack step already walked the full `sendable` slice unconditionally; Task 1 is what stops `Send` from 400-ing on an oversized payload in the first place.
- Added a short doc comment at `digest.go`'s `sentIDs` construction recording why that list already covers truncated-out events.

## Task Commits

Each task was committed atomically, following this repo's established RED/GREEN split for `tdd="true"` tasks:

1. **Task 1 (RED): add failing tests for the whole-Description cap** - `c21fbde` (test) -- new tests against compiling stub implementations of `digestLine`/`assembleDescription`/`truncationNote`; every pre-existing `digest_format_test.go` assertion still passed unchanged since `buildDigestEmbed`'s body was untouched in this commit.
2. **Task 1 (GREEN): cap buildDigestEmbed's whole Description at 4096 runes** - `57850c6` (feat) -- real implementation; all new and pre-existing tests pass.
3. **Task 2: prove SendDigestIfDue acks every event in an oversized batch** - `741dc93` (test) -- new real-Postgres test plus the `digest.go` doc comment; no production logic change needed (see Deviations below for why this is a single commit, not RED-then-GREEN).

No separate plan-metadata commit -- this quick task's orchestrator (Step 8) handles the docs commit for `SUMMARY.md`/`STATE.md`.

## Files Created/Modified
- `internal/notifier/digest_format.go` - Added `discordDescriptionLimit`, `truncationNoteReserve`, `digestLine`, `assembleDescription`, `truncationNote`; rewired `buildDigestEmbed` to build segments and call `assembleDescription` instead of writing straight into a shared `strings.Builder`.
- `internal/notifier/digest_format_test.go` - Added `TestDigestLine_RendersLabelURLAndDeluxeSuffix`, `TestAssembleDescription_FitsUnderLimitReturnsJoinUnchanged`, `TestAssembleDescription_ExceedsLimitTruncatesOnSegmentBoundary`, `TestTruncationNote_SingularAndPlural`, `TestBuildDigestEmbed_SmallOrdinaryBatchNoTruncationNote`, `TestBuildDigestEmbed_OversizedBatchTruncatesAndReconciles`.
- `internal/notifier/digest.go` - Added a 3-line comment at the `sentIDs` construction documenting why the full `sendable` slice is always acked regardless of what `digest_format.go` rendered.
- `internal/notifier/digest_test.go` - Added `TestSendDigestIfDue_OversizedBatch_TruncatesDescriptionAndAcksEveryEvent` (200-event real-Postgres fixture).

## Decisions Made
- **Fixture size for forcing truncation:** 200 loop-generated `new_release` events (unique external_id/title per iteration, padded titles) in both the pure-function test (`digest_format_test.go`) and the real-Postgres test (`digest_test.go`). This reliably renders well past 4096 runes (each line is roughly 90-110 runes) while staying a realistic backlog size, not an absurd one. Both tests assert `rendered` is strictly between `0` and `n` as a fixture-sanity guard, not just that a note is present.
- **`truncationNoteReserve` usage:** kept as a `strings.Builder.Grow` capacity hint only (per the design contract's explicit "not a hard second limit" instruction), rather than folding it into the truncation-boundary search itself. The boundary search checks the exact invariant (`prefix runes + note runes <= discordDescriptionLimit`) on every candidate.
- **RED/GREEN split for Task 1:** followed this repo's established pattern (see `78cdfdf`, `b046d9f` in phase 22) of shipping compiling stub implementations in the RED commit so `go build`/`golangci-lint` stay green at every commit, rather than a non-compiling RED commit.
- **Task 2 as a single `test` commit:** the plan's own action text predicted the new oversized-batch test would pass against Task 1's changes with zero `digest.go` behavior changes -- confirmed true on the first run. Per this repo's precedent for "test proves pre-existing behavior" work (e.g. Phase 05 Rule 2 deviations), this is a single test-plus-comment commit rather than an artificial RED-then-GREEN split with no real GREEN payload.

## Deviations from Plan

None - plan executed exactly as written. Task 2's action text explicitly anticipated the "test passes with zero digest.go logic changes" outcome and that is what happened; this is not a deviation, it's the plan's own predicted result.

## Issues Encountered
- `make test` (which runs `go test ./... -race`) fails to build on this Windows dev machine due to a pre-existing, previously documented cgo/mingw64 toolchain break (`runtime/cgo: cgo.exe: exit status 2`) that has nothing to do with this change -- consistent with STATE.md's repeated prior notes on this exact limitation (Phase 01-04, 11.1-04, 15-02). Substituted plain `go test ./...` (no `-race`) per the established precedent.
- A first plain `go test ./...` run (default parallelism) failed one test in `internal/detection` (`TestDetectDeezer_ReDetectionInsertsNothing`) that this plan never touches. Re-ran with `go test ./... -p 1` (also an established precedent per STATE.md Phase 15-02's "-p 1 needed for the flaky poller DB test") and every one of the 24 tested packages passed. Confirmed in isolation (3x `-count=3` run) that the test passes reliably alone -- this is pre-existing cross-test Postgres-schema contention unrelated to this plan's notifier-only changes, not a regression introduced here.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- T-22-15 (22-SECURITY.md, currently `open — BLOCKING`, `threats_open: 1`) and CR-01 (22-REVIEW.md) are functionally closed by this plan's implementation and tests, but **`22-SECURITY.md`'s frontmatter itself still needs a `/gsd-secure-phase 22` re-run** to flip `threats_open: 0` / `status: verified` in that document -- explicitly out of this plan's scope per its `<threat_model>` section, and the most obvious immediate follow-up.
- This is an interim guard only (per the plan's objective) -- DGST-12's full multi-message split for an oversized digest remains Phase 23's job. No blockers for that future work; `assembleDescription`'s segment-list shape (one string per event, already grouped/ordered/rendered) is the natural input for a future multi-message splitter.
- `make sqlc-check` was skipped per the plan's own action text (no migration or query file touched by this fix) and `web/` prettier/vitest were skipped (no `web/` files changed).
- Full Definition of Done verified clean: `go build ./...`, `go vet ./...`, `golangci-lint run` (0 issues), `go test ./... -p 1` (24/24 packages pass, `-race` substituted out per the documented Windows toolchain limitation), `make coverage-gate` (91.38% backend, 80% floor).

---
*Phase: quick/260916-wao*
*Completed: 2026-09-16*

## Self-Check: PASSED

All 4 modified files and this SUMMARY.md confirmed present on disk. All 3 task commits (`c21fbde`, `57850c6`, `741dc93`) confirmed present in `git log`.
