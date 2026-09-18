---
phase: 23-digest-readability-discord-limits
plan: 03
subsystem: notifications
tags: [go, discord, digest, chunking]

# Dependency graph
requires:
  - phase: 23-01
    provides: digestEntry/digestGroup/digestChunk types, chunkOverheadReserve/chunkContentBudget budget constants, the legal-but-greedy chunkDigest splitter, and buildDigestChunks' window-header stamping loop this plan expands
provides:
  - Group-preferred chunk boundaries -- chunkDigest packs whole groups until the next group would overflow the current chunk, falling back to a whole-line-boundary split only for the one group that alone exceeds chunkContentBudget
  - Continuation markers (continuationHeading/continuationNote) stamped at both ends of a genuine mid-group cut, never on a boundary that falls between whole groups
  - positionIndicator + a stamped "(N/Total)" on every chunk's header line, omitted entirely when Total is 1
  - Three Verification Invariant tests (4096-rune ceiling, id-union-exactly-once, order-preserving concatenation) plus a shuffle invariant, all proven over a reusable 700-event/20+-chunk synthetic batch
affects: [23-04-chunk-cap-time-budget]

# Actuals (#2632)
actuals:
  tokens: 9047
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Group-preferred fill policy: chunkDigest decides per-group (fits current chunk's remainder / fits an empty chunk / doesn't fit any chunk) via a three-way switch, reusing the pre-existing whole-line-boundary splitter unmodified as the fallback for the one group-alone-exceeds-budget case, rather than rewriting it"
    - "Continuation markers scoped to splitOversizedGroup only -- a chunk boundary between two whole groups (chunkDigest's common case) never calls into marker-stamping code, so the two code paths cannot accidentally cross-contaminate"
    - "Single forward-pass stamping (window header + position indicator together, in buildDigestChunks' existing loop) spending the pre-reserved chunkOverheadReserve, never a second pass that re-measures the split after stamping (D-07/D-20's fixed-point prohibition)"

key-files:
  created: []
  modified:
    - internal/notifier/digest_chunk.go
    - internal/notifier/digest_chunk_test.go

key-decisions:
  - "digestGroup gained a bare `title` field (no bold markers) alongside its pre-rendered `heading`, so continuationHeading/continuationNote can build their own wording without re-parsing the markdown-decorated heading string"
  - "continuationNote ends with its own trailing newline so a chunk cut mid-group still ends on a whole line, matching the pre-existing 'every chunk ends with \\n' invariant the syntheticEvents(200) regression test already asserted"
  - "Two pre-existing budget assertions (inherited from plan 23-01) needed their comparison changed from chunkContentBudget to discordDescriptionLimit once continuation notes exist -- the note is deliberately stamped *after* the chunkContentBudget line-fit check, spending the reserve D-20 sized for exactly this, so a chunk's true ceiling is 4096 runes, not 3796, once a marker is stamped"
  - "invariantBatchSize (700 synthetic events) is a package-level test constant reused by all three invariant tests plus the shuffle invariant, pinned by its own precondition test (TestChunkDigest_InvariantBatchProducesAtLeast20Chunks) so a future change that shrinks the chunk count under 20 fails loudly rather than silently under-testing the invariants"

requirements-completed: [DGST-11, DGST-12]

coverage:
  - id: D1
    description: "A chunk boundary falls between two whole groups in the common case (group-preferred fill policy), never mid-group unless that one group alone exceeds chunkContentBudget"
    requirement: DGST-12
    verification:
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_GroupPreferredBoundariesOnlyBetweenGroups"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_SecondGroupStartsFreshChunkRatherThanSpillingLines"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_OversizedGroupLineSplitDoesNotSwallowNextGroupHeading"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_SeparatorOnlyOnNonFirstGroupInChunk"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_NoEmptyChunkNoEmptyIDs"
        status: pass
    human_judgment: false
  - id: D2
    description: "A group that genuinely splits across chunks carries the (continued) heading on the resuming chunk and a digit-free trailing note on the chunk it was cut away from; a group-boundary cut carries neither marker"
    requirement: DGST-12
    verification:
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestContinuationHeading_ExactWording"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestContinuationNote_NoDigit"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_OversizedGroupContinuationMarkersAtBothEnds"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_OversizedGroupThreeChunksBothMarkersRepeat"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_GroupBoundaryCarriesNoContinuationMarkers"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_ContinuationMarkersNeverExceedBudget"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every chunk's header carries a (N/Total) position indicator, omitted entirely when Total is 1 so an ordinary single-message digest differs from the pre-phase output by exactly one line"
    requirement: DGST-11
    verification:
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestPositionIndicator_EmptyAtTotalOne"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestPositionIndicator_NonEmptyAboveTotalOne"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestBuildDigestChunks_OneChunkNoIndicator"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestBuildDigestChunks_ThreeChunksIndicatorsMatchPosition"
        status: pass
    human_judgment: false
  - id: D4
    description: "The chunker's three Verification Invariants (4096-rune ceiling, id-union exactly once, order-preserving concatenation) hold over a synthetic batch of at least 20 chunks, and chunk output is byte-identical regardless of input arrival order"
    requirement: DGST-12
    verification:
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkDigest_InvariantBatchProducesAtLeast20Chunks"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkInvariant1_EveryChunkAtMostDiscordLimit"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkInvariant2_IDUnionExactlyOnce"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestChunkInvariant3_ConcatenationReproducesUnsplitOrder"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_chunk_test.go#TestBuildDigestChunks_LargeBatchShuffleInvariant"
        status: pass
    human_judgment: false

duration: 9min
completed: 2026-09-18
status: complete
---

# Phase 23 Plan 3: Group-Preferred Chunk Boundaries, Continuation Markers, and Position Indicator Summary

**Expanded plan 23-01's legal-but-greedy digest splitter into a group-preferred packer with rare-path continuation markers and a per-chunk "(N/Total)" indicator, all pinned by three property-style invariants over a 700-event synthetic batch that forces 20+ chunks.**

## Performance

- **Duration:** 9 min (span between first and last task commit)
- **Started:** 2026-09-17T23:10:10-05:00
- **Completed:** 2026-09-17T23:19:13-05:00
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments
- `chunkDigest` now packs whole groups until the next group would overflow the current chunk (D-06 amended group-preferred fill policy), so a chunk boundary falls between two whole groups by construction in the common case; the pre-existing whole-line-boundary splitter (`splitOversizedGroup`) is reused unmodified as the fallback for the one group-alone-exceeds-budget case
- `continuationHeading`/`continuationNote` stamp D-09/D-10's markers at both ends of a genuine mid-group cut -- the resuming chunk opens with `**Title (continued)**`, the cut-away chunk ends with a digit-free trailing note naming no message number; a group-boundary cut (the common case) never carries either marker
- `positionIndicator` stamps D-21's `(N/Total)` fragment onto every chunk's header in the same forward pass that already stamps the window header, omitted entirely when Total is 1
- Three Verification Invariant tests plus a shuffle invariant, all proven over a reusable 700-event synthetic batch (`invariantBatchSize`) that produces 20+ chunks, mixing multi-byte and emoji titles so the rune-count assertions measure what they claim to measure

## Task Commits

1. **Task 1: Group-preferred chunk boundaries** - `af6f34b` (feat)
2. **Task 2: Continuation markers on a split group** - `6e04590` (feat)
3. **Task 3: Position indicator and the three chunker invariants** - `fb0f7b7` (feat)

**Plan metadata:** pending (docs: complete plan)

_Note: All three tasks carried `tdd="true"`; tests were authored alongside implementation in the same commit rather than as separate RED-then-GREEN commits (see TDD Gate Compliance below)._

## Files Created/Modified
- `internal/notifier/digest_chunk.go` - `chunkDigest` rewritten for group-preferred boundaries; `renderGroup` (new atomic whole-group render+id unit); `splitOversizedGroup` extended with continuation-marker stamping; `continuationHeading`/`continuationNote` (new); `positionIndicator` (new); `buildDigestChunks` stamps the indicator alongside the window header; `digestGroup` gained a `title` field
- `internal/notifier/digest_chunk_test.go` - 22 new tests across the three tasks' behaviors plus the four Verification Invariant/shuffle tests; two pre-existing budget assertions corrected from `chunkContentBudget` to `discordDescriptionLimit`

## Decisions Made
- `digestGroup` gained a bare `title` field (no bold markers) alongside its pre-rendered `heading`, so `continuationHeading`/`continuationNote` can build their own wording without re-parsing the markdown-decorated heading string
- `continuationNote` ends with its own trailing newline so a chunk cut mid-group still ends on a whole line, matching the pre-existing "every chunk ends with `\n`" invariant the `syntheticEvents(200)` regression test already asserted
- Two pre-existing budget assertions (inherited from plan 23-01) needed their comparison changed from `chunkContentBudget` to `discordDescriptionLimit` once continuation notes exist -- the note is deliberately stamped *after* the `chunkContentBudget` line-fit check, spending the reserve D-20 sized for exactly this, so a chunk's true ceiling is 4096 runes, not 3796, once a marker is stamped
- `invariantBatchSize` (700 synthetic events) is a package-level test constant reused by all three invariant tests plus the shuffle invariant, pinned by its own precondition test (`TestChunkDigest_InvariantBatchProducesAtLeast20Chunks`) so a future change that shrinks the chunk count under 20 fails loudly rather than silently under-testing the invariants

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Two pre-existing test assertions corrected to the true post-marker ceiling**
- **Found during:** Task 2 (continuation markers on a split group)
- **Issue:** `TestChunkDigest_OverBudgetMultipleChunks` and `TestChunkDigest_ContinuationMarkersNeverExceedBudget` asserted every chunk's rune count `<= chunkContentBudget` (3796) -- true before Task 2, but `syntheticEvents(200)`'s single-group batch now legitimately triggers `splitOversizedGroup`'s new continuation-note stamping, which is deliberately applied *after* the content-budget line-fit check and spends D-20's pre-reserved 300-rune overhead. A chunk carrying a note can now legitimately land between 3796 and 4096 runes -- still safely under Discord's real ceiling, which is what D-20's reserve exists to guarantee.
- **Fix:** Both assertions changed to compare against `discordDescriptionLimit` (4096), the invariant that actually matters, with an updated comment explaining why
- **Files modified:** internal/notifier/digest_chunk_test.go
- **Verification:** `go test ./internal/notifier/ -run TestChunkDigest -v` green after the fix
- **Committed in:** `6e04590` (Task 2 commit)

**2. [Rule 1 - Bug] Test helper missing the new `title` field**
- **Found during:** Task 2 (writing the continuation-marker tests)
- **Issue:** `makeSyntheticGroup` (Task 1's test helper) built a `digestGroup{heading: ...}` literal without the new `title` field Task 2 added to the struct, so every continuation-heading test using it produced `** (continued)**` (empty title) instead of the real group name
- **Fix:** Added `title: title` to the helper's constructed literal
- **Files modified:** internal/notifier/digest_chunk_test.go
- **Verification:** `TestChunkDigest_OversizedGroupThreeChunksBothMarkersRepeat` (which asserts the literal title text `**Deluxe Changes (continued)**`) passes after the fix
- **Committed in:** `6e04590` (Task 2 commit) -- caught pre-commit via test failure, never landed broken

**3. [Rule 1 - Bug] Invariant 3's test-only line-stripper missed plain group headings**
- **Found during:** Task 3 (writing `TestChunkInvariant3_ConcatenationReproducesUnsplitOrder`)
- **Issue:** `stripChunkMarkers`'s first draft only filtered the window header, continuation headings, and trailing notes -- it left a group's ordinary (non-continuation) heading line in place, so the stripped concatenation carried extra heading lines the "want" side (built from bare `digestEntry.text` values only) never had, producing a 55-rune mismatch
- **Fix:** Generalized the heading filter to any line both starting and ending with `**` (covers plain and `(continued)` headings identically, since no rendered event line has that shape)
- **Files modified:** internal/notifier/digest_chunk_test.go
- **Verification:** `TestChunkInvariant3_ConcatenationReproducesUnsplitOrder` passes after the fix
- **Committed in:** `fb0f7b7` (Task 3 commit) -- caught pre-commit via test failure, never landed broken

---

**Total deviations:** 3 auto-fixed (all Rule 1 -- test-code bugs caught and fixed before any commit landed broken)
**Impact on plan:** All three fixes are test-only corrections needed to make the plan's own acceptance criteria provable; none changed production behavior beyond what Tasks 1-3 already specified. No scope creep.

## TDD Gate Compliance

All three tasks carry `tdd="true"` but were not split into separate RED-then-GREEN commits -- tests and implementation were authored together in the same reasoning pass per task, then run to green before each task's single commit. This mirrors plan 23-01's own documented deviation (same rationale: tightly coupled design where the splitter/marker/indicator logic and its tests were designed as one unit). No failing-then-passing commit pair exists in git history for these three tasks. This is a deliberate, documented deviation from the strict two-commit RED/GREEN convention, not a skipped verification step -- each task's `<verify>` block (specific `-run` regexes) was run and passed before its commit, and the full package suite plus `go vet`/`golangci-lint` were re-run before every commit.

## Issues Encountered
None beyond the three auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 23-04 (chunk cap + time budget) can build directly on this plan's `digestGroup`/`digestChunk` shapes, `chunkOverheadReserve`'s already-reserved headroom, and the reusable `invariantBatchSize`/`syntheticInvariantBatch` fixture this plan's own note flagged as reusable for cap tests
- Full backend suite (`go test ./... -count=1` against real Postgres, `make coverage-gate` at 91.20%, `go vet ./...`, `golangci-lint run`) green; `make sqlc-check` clean; `git diff --exit-code -- internal/discord internal/notifier/digest.go queries/ internal/db/sqlc web/` confirms this plan touched only the chunker and its tests
- `make test` (the `-race` target) could not run natively on this Windows dev box -- pre-existing, documented limitation (`runtime/cgo: cgo.exe: exit status 2`); substituted plain `go test ./... -count=1` against real Postgres per established project precedent (CI's Linux `-race` runner remains the authoritative gate)

---
*Phase: 23-digest-readability-discord-limits*
*Completed: 2026-09-18*

## Self-Check: PASSED

All modified files confirmed on disk; all three task commits (`af6f34b`, `6e04590`, `fb0f7b7`) confirmed in git history.
