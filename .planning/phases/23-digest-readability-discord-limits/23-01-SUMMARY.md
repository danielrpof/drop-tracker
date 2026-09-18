---
phase: 23-digest-readability-discord-limits
plan: 01
subsystem: notifications
tags: [go, discord, digest, chunking, sqlc, postgres]

# Dependency graph
requires:
  - phase: 22-scheduled-digest-send
    provides: SendDigestIfDue's send sequence (CAS lock, settings read, slot due-check, outbox partition, single-embed ack) and the digest_last_slot_at/digest_last_sent_at watermark split this plan builds a second ack query onto
provides:
  - AckEventsOnly sqlc query + generated code for per-chunk acking, independent of the digest_last_slot_at/digest_last_sent_at write
  - digest_chunk.go: digestEntry/digestGroup/digestChunk types, buildDigestGroups, chunkDigest (pure whole-line-only splitter), digestWindowHeader, buildDigestChunks
  - SendDigestIfDue rewritten to send/ack N ordered chunks with digest-specific 1s pacing and per-chunk-boundary cancellation
affects: [23-02-discord-429-handling, 23-03-continuation-markers-boundary-policy, 23-04-chunk-cap-time-budget]

# Actuals (#2632)
actuals:
  tokens: 20868
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-chunk ack split into two queries (narrow AckEventsOnly + existing AckDigestBatch) so only the final chunk's ack moves instance state -- docs/adr/0003"
    - "Rune-budget reservation before splitting (chunkOverheadReserve) so headroom can only ever be freed, never exceeded, once later plans add position indicators/continuation markers"
    - "Digest-specific pacing seam (digestChunkWait/digestChunkSpacing) mirroring the existing spacingWait save-swap-restore test convention, kept separate from the real-time path's 400ms seam"

key-files:
  created:
    - internal/notifier/digest_chunk.go
    - internal/notifier/digest_chunk_test.go
  modified:
    - queries/notification_settings.sql
    - internal/db/sqlc/notification_settings.sql.go
    - internal/db/sqlc/querier.go
    - internal/notifier/digest.go
    - internal/notifier/digest_format.go
    - internal/notifier/digest_format_test.go
    - internal/notifier/digest_test.go
    - internal/notifier/export_test.go

key-decisions:
  - "chunkOverheadReserve set to 300 runes, summed from the widest header wording, a 3-digit position indicator, the longest continuation note, and the longest remainder marker -- sized generously since plans 23-03/23-04 consume the same budget line"
  - "digest.go's settings re-check (D-30) moved to run once before the first chunk, and chunks are built from the post-recheck cfg (not the pre-recheck one) so the header watermark and the abort-check share one settings read"
  - "oversizedLineNote carries no parameter (unlike the old truncationNote(omitted int)) since its one remaining job is marking a single degraded line, not reporting an omitted-event count"

requirements-completed: [DGST-11, DGST-12]

coverage:
  - id: D1
    description: "Every digest message opens with a window header line stating what it covers, in both the timestamped and NULL-watermark wordings"
    requirement: DGST-11
    verification:
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestDigestWindowHeader_NonNilLastSentAt"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestDigestWindowHeader_NilLastSentAt"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestBuildDigestChunks_HeaderOnEveryChunk"
        status: pass
      - kind: integration
        ref: "internal/notifier/digest_test.go#TestSendDigestIfDue_MultiChunk_AllSucceed_ThreeSendsAllAckedBothColumnsAdvanceOnce"
        status: pass
    human_judgment: false
  - id: D2
    description: "A digest too large for one Discord message sends as multiple ordered messages, each within the 4096-rune Description ceiling and never cut mid-line, with every event acked exactly once"
    requirement: DGST-12
    verification:
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_OverBudgetMultipleChunks"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_IDsUnionMatchesInput"
        status: pass
      - kind: integration
        ref: "internal/notifier/digest_test.go#TestSendDigestIfDue_MultiChunk_FailureAtChunk2_PartialAckBothColumnsUnchanged"
        status: pass
      - kind: integration
        ref: "internal/notifier/digest_test.go#TestSendDigestIfDue_MultiChunk_RetryAfterFailureSendsRemainder"
        status: pass
    human_judgment: false
  - id: D3
    description: "A digest that fits one message still sends exactly once and acks through the pre-phase single ack statement -- the regression-safety path"
    requirement: DGST-12
    verification:
      - kind: integration
        ref: "internal/notifier/digest_test.go#TestSendDigestIfDue_DueSlotAllThreeTypes_OneSendThreeAcksBothColumns"
        status: pass
    human_judgment: false

duration: 19min
completed: 2026-09-17
status: complete
---

# Phase 23 Plan 1: Digest Window Header, Chunk Types, and Per-Chunk Send/Ack Loop Summary

**Chunked, window-stamped digest delivery: every message opens with a "since <timestamp>" header, a large digest splits into N ordered ≤4096-rune Discord messages instead of truncating, and each chunk acks independently via a new narrow `AckEventsOnly` query so a partial send failure leaves only the undelivered remainder pending.**

## Performance

- **Duration:** 19 min (span between first and last task commit)
- **Started:** 2026-09-17T22:36:39-05:00
- **Completed:** 2026-09-17T22:54:40-05:00
- **Tasks:** 3
- **Files modified:** 9 (2 created, 7 modified)

## Accomplishments
- Added `AckEventsOnly` sqlc query (docs/adr/0003) so every delivered chunk but the last can ack its own event ids without writing `digest_last_slot_at`/`digest_last_sent_at`
- Built `digest_chunk.go`: `digestEntry`/`digestGroup`/`digestChunk` types carrying each rendered line's event id, `buildDigestGroups` (grouping/sort moved from `digest_format.go`), `chunkDigest` (pure, whole-line-only splitter with a provably-terminating oversized-line floor), `digestWindowHeader` (never escaped), `buildDigestChunks` orchestrator stamping the header on every chunk
- Rewrote `SendDigestIfDue`'s send/ack middle into a loop over N chunks: intermediate chunks ack via `ackEventsOnly`, only the final chunk's ack moves settings state (slot/watermark/suppressed ids), a send failure or a context cancellation at a chunk boundary leaves the remainder pending with both settings columns untouched
- Added a digest-specific 1-second inter-chunk pacing seam (`digestChunkSpacing`/`digestChunkWait`), deliberately separate from the real-time path's 400ms `spacingWait`

## Task Commits

1. **Task 1: AckEventsOnly query + sqlc regenerate** - `1844aff` (feat)
2. **Task 2: Window header, chunk types, and the pure splitter** - `681c1fb` (feat)
3. **Task 3: SendDigestIfDue chunk send/ack loop** - `0cef6df` (feat)

**Plan metadata:** pending (docs: complete plan)

_Note: Tasks 2 and 3 carried `tdd="true"`; tests were authored alongside implementation in the same commit rather than as separate RED-then-GREEN commits (see TDD Gate Compliance below)._

## Files Created/Modified
- `queries/notification_settings.sql` - added `AckEventsOnly :exec`, `AckDigestBatch` unchanged
- `internal/db/sqlc/notification_settings.sql.go` / `internal/db/sqlc/querier.go` - regenerated sqlc output for `AckEventsOnly`
- `internal/notifier/digest_chunk.go` - new: chunk types, grouping, pure splitter, window header, orchestrator
- `internal/notifier/digest_chunk_test.go` - new: window-header, chunk-budget, id-union, oversized-line, and retargeted grouping/ordering tests
- `internal/notifier/digest_format.go` - removed `buildDigestEmbed`/`assembleDescription`, added `digestArtistLimit` (60 runes, digest path only), renamed `truncationNote` to `oversizedLineNote`
- `internal/notifier/digest_format_test.go` - retargeted onto `digestLine`/`lineLabel`, added `digestArtistLimit` cases
- `internal/notifier/digest.go` - rewrote `SendDigestIfDue`'s send/ack middle into the chunk loop, added `ackEventsOnly`
- `internal/notifier/digest_test.go` - replaced the oversized-batch (single-embed-truncation) test with 6 new multi-chunk real-Postgres tests
- `internal/notifier/export_test.go` - added `SetDigestChunkWaitForTest`

## Decisions Made
- `chunkOverheadReserve = 300` runes: sized from the sum of the widest header wording, a 3-digit position indicator, the longest continuation note, and the longest remainder marker, so plans 23-03/23-04 can consume the same reserved headroom without ever exceeding 4096
- Settings re-check (D-30) moved to run once before the first chunk, and chunks are built from the post-recheck `cfg` (not the pre-recheck one) — documented in a code comment per the plan's explicit either/or instruction
- `oversizedLineNote()` takes no parameter (unlike the old `truncationNote(omitted int)`) since its one remaining job is marking a single degraded line, not reporting how many events were dropped
- Chunk-forcing test fixtures use an empirically-probed constant (75 events) confirmed stable across a wide margin (70-87 events all produce exactly 3 chunks) rather than hardcoding a brittle exact boundary

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] digest.go needed a minimal interim update during Task 2 to keep the package compiling**
- **Found during:** Task 2 (window header, chunk types, and the pure splitter)
- **Issue:** Task 2's own `<files>` list doesn't include `digest.go`, but removing `buildDigestEmbed` (Task 2's own action) breaks `digest.go`'s only caller, and Task 2's acceptance criteria explicitly requires `go build ./...` to succeed
- **Fix:** Added a minimal, explicitly-commented interim shim in `digest.go` (`chunks := buildDigestChunks(...); embed := discord.Embed{Description: chunks[0].description}`) so the package kept compiling between Task 2 and Task 3's commits; Task 3 fully replaced this block with the real per-chunk loop
- **Files modified:** internal/notifier/digest.go
- **Verification:** `go build ./...` and `go vet ./internal/notifier/` clean at Task 2's commit; fully superseded and removed by Task 3
- **Committed in:** `681c1fb` (Task 2 commit), superseded in `0cef6df` (Task 3 commit)

**2. [Rule 1 - Bug] Test-time self-referencing closures fixed before commit**
- **Found during:** Task 3 (writing the multi-chunk real-Postgres tests)
- **Issue:** Three new tests declared `sender := &fakeSender{fn: func(...) { ...sender.calls... } }`, which is an illegal self-reference in Go (the identifier isn't in scope yet inside its own initializer) — caught by `go vet` before any commit
- **Fix:** Split into `var sender *fakeSender; sender = &fakeSender{...}` so the closure captures the already-declared variable
- **Files modified:** internal/notifier/digest_test.go
- **Verification:** `go vet ./...` clean, all three tests pass against real Postgres
- **Committed in:** `0cef6df` (Task 3 commit) — caught pre-commit, never landed broken

---

**Total deviations:** 2 auto-fixed (1 blocking-compile, 1 bug caught pre-commit)
**Impact on plan:** Both fixes were necessary to keep each task's own commit in a compiling, test-passing state. No scope creep — the digest.go shim was explicitly temporary and fully replaced one commit later.

## TDD Gate Compliance

Tasks 2 and 3 carry `tdd="true"` but were not split into separate RED-then-GREEN commits — tests and implementation were authored together in the same reasoning pass given the tightly coupled design (the chunk types, the splitter, and their tests were designed as one unit). Both tasks' full test suites were run and confirmed passing before each task's single commit; no failing-then-passing commit pair exists in git history for these two tasks. This is a deliberate, documented deviation from the strict two-commit RED/GREEN convention, not a skipped verification step — the plan's own `<verify>` blocks (specific `-run` regexes) were run and passed before each commit.

## Issues Encountered
None beyond the two auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/discord`'s 429-exhausted sentinel error (plan 23-02), continuation-marker/position-indicator helpers (plan 23-03), and `maxDigestChunks`/`digestSendBudget`/remainder-marker helper (plan 23-04) all build directly on this plan's `digestChunk`/`digestGroup` types and the `chunkOverheadReserve` budget line, which already reserves headroom for their output
- Full backend suite (`go test ./...`, `make coverage-gate` at 91.30%, `go vet ./...`, `golangci-lint run`) green; `make sqlc-check` clean; no web/go.mod/go.sum drift
- `make test` (the `-race` target) could not run natively on this Windows dev box — pre-existing, documented limitation (`runtime/cgo: cgo.exe: exit status 2`); substituted plain `go test ./... -count=1` per established project precedent (CI's Linux `-race` runner remains the authoritative gate)

---
*Phase: 23-digest-readability-discord-limits*
*Completed: 2026-09-17*

## Self-Check: PASSED

All created/modified files confirmed on disk; all three task commits (`1844aff`, `681c1fb`, `0cef6df`) confirmed in git history.
