---
phase: 23-digest-readability-discord-limits
verified: 2026-09-18T00:00:00Z
status: passed
score: 8/8 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 23: Digest Readability & Discord Limits Verification Report

**Phase Goal:** A digest states the window it covers and stays complete — with Phase 22's grouping intact — even when it is large enough to exceed what one Discord message can hold.
**Verified:** 2026-09-18
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every digest message states the window it covers ("since <timestamp>"), unambiguous after a late/skipped/caught-up tick | ✓ VERIFIED | `digestWindowHeader` (`internal/notifier/digest_chunk.go:329-334`) renders `Everything pending since <t:UNIX:R>` or the NULL-watermark wording; `buildDigestChunks` (lines 390-400) stamps it onto **every** chunk, not just the first. Proven by `TestDigestWindowHeader_NonNilLastSentAt`, `TestDigestWindowHeader_NilLastSentAt`, `TestBuildDigestChunks_HeaderOnEveryChunk` — all pass. |
| 2 | A digest exceeding Discord's per-message limits is delivered as multiple ordered messages, spaced by a deliberate inter-chunk delay, with no event silently dropped and no content truncated without a visible marker, Phase 22's grouping preserved (no group silently broken without a continuation marker) | ✓ VERIFIED | `chunkDigest` (group-preferred fill, `digest_chunk.go:157-202`), `splitOversizedGroup` (rare-path line-boundary fallback with `continuationHeading`/`continuationNote` markers, lines 219-297), `digestChunkSpacing` = 1s paced via `digestChunkWait` seam in `SendDigestIfDue`'s loop (`digest.go:147-173`). Proven end-to-end by real-Postgres integration test `TestSendDigestIfDue_MultiChunk_AllSucceed_ThreeSendsAllAckedBothColumnsAdvanceOnce` (3 sends, all acked) and unit invariant tests `TestChunkInvariant1_EveryChunkAtMostDiscordLimit`, `TestChunkInvariant2_IDUnionExactlyOnce`, `TestChunkInvariant3_ConcatenationReproducesUnsplitOrder` over a 700-event/20+-chunk synthetic batch — all pass. |
| 3 | Events are acked per delivered message, not per digest run: a failure partway through leaves the undelivered remainder pending for the next digest instead of losing it or re-sending what already went out | ✓ VERIFIED | `SendDigestIfDue` acks every non-final chunk via `ackEventsOnly` (narrow, settings-columns-untouched) and only the final complete chunk via `ackDigestBatch` (advances `digest_last_slot_at`/`digest_last_sent_at`) — `digest.go:198-226`. Proven by real-Postgres integration test `TestSendDigestIfDue_MultiChunk_FailureAtChunk2_PartialAckBothColumnsUnchanged` (chunk 1 acked, chunks 2-3 still pending, both settings columns byte-identical to pre-call) and `TestSendDigestIfDue_MultiChunk_RetryAfterFailureSendsRemainder` (a later call delivers the remainder). Both pass against real Postgres. |
| 4 | A run is bounded (chunk cap + time budget) so a large digest self-drains across ticks rather than holding the shared notification lock unboundedly, and a chunk failure caused by rate-limiting is distinguishable in the logs | ✓ VERIFIED | `maxDigestChunks` (20) with `remainderMarker`, `digestSendBudget` (~3 min) checked at chunk boundaries only (never as the POST context), `errors.Is(err, discord.ErrRateLimited)` discriminator at the chunk-failure log site, `digest sent` summary with `chunk_count`/`pending_remainder` fields plus a separate cap/budget Warn. Proven by `TestSendDigestIfDue_Cap_ExactlyMaxChunksSentBothColumnsUnchangedRemainderPending`, `TestSendDigestIfDue_Cap_SecondCallDrainsRemainderBothColumnsAdvanceOnce`, `TestSendDigestIfDue_Budget_ExceededAtBoundaryStopsPartialAckBothColumnsUnchanged`, `TestSendDigestIfDue_ChunkFailure_RateLimitedDiscriminatorTrue/False`, `TestSendDigestIfDue_Summary_*` — all pass against real Postgres. |

**Score:** 4/4 phase-level truths verified (all four success criteria from ROADMAP.md), 0 present-but-behavior-unverified.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/notifier/digest_chunk.go` | Chunk types, window header, splitter, chunker invariants, cap, budget | ✓ VERIFIED | Exists, all declared symbols present (`digestEntry`, `digestGroup`, `digestChunk`, `buildDigestGroups`, `chunkDigest`, `digestWindowHeader`, `buildDigestChunks`, `positionIndicator`, `continuationHeading`, `continuationNote`, `remainderMarker`, `maxDigestChunks`, `digestSendBudget`), wired into `digest.go` |
| `internal/notifier/digest_chunk_test.go` | Full test coverage of the above | ✓ VERIFIED | 40+ tests, all pass (`go test -run 'TestDigestWindowHeader|TestChunkDigest|TestBuildDigestChunks|...'`) |
| `queries/notification_settings.sql` + generated sqlc output | `AckEventsOnly :exec` query, byte-unchanged `AckDigestBatch` | ✓ VERIFIED | `make sqlc-check` clean (no drift); `AckDigestBatch` unmodified per `git diff` at plan-01 commit |
| `internal/discord/client.go` | Exported `ErrRateLimited` sentinel, no retry-policy change, no secret leakage | ✓ VERIFIED | Declared at line 39, branch at lines 181-186; `TestSend_429Twice_ReturnsErrorAfterSingleRetry`, `TestSend_ErrorPaths_NeverLeakTokenOrBody` pass |
| `internal/notifier/digest.go` | Per-chunk send/ack loop, cap/budget checks, observability | ✓ VERIFIED | Lines 37-253; matches plan's action blocks exactly |
| `cmd/server/main.go` | Drain-deadline log reworded to Warn (D-27) | ✓ VERIFIED | Line 394: `logger.Warn("digest scheduler drain deadline reached: remainder stays pending for the next check", ...)` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `buildDigestChunks` output | `SendDigestIfDue`'s chunk loop | `digestChunk{description, ids}` carries both rendered text and ack ids | ✓ WIRED | `digest.go:135, 147` |
| `digestChunk.ids` | `ackEventsOnly` (non-final) / `ackDigestBatch` (final) | ack-decision branch on `i < len(chunks)-1 \|\| deferred > 0` | ✓ WIRED | `digest.go:198-226` |
| `internal/discord`'s `ErrRateLimited` sentinel | notifier's chunk-failure log site | `errors.Is` | ✓ WIRED | `digest.go:192` |
| `chunkOverheadReserve` | indicator/continuation/remainder marker stamping | single forward pass, no re-measure | ✓ WIRED | `digest_chunk.go:393-400` |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| DGST-11 | 23-01, 23-03 | Every digest message shows the window it covers | ✓ SATISFIED | `digestWindowHeader` + per-chunk stamping, tested |
| DGST-12 | 23-01, 23-02, 23-03, 23-04 | A digest exceeding Discord limits splits into multiple messages instead of truncating | ✓ SATISFIED | `chunkDigest`/`splitOversizedGroup`/cap/budget, tested end-to-end with real Postgres |

No orphaned requirements — REQUIREMENTS.md maps only DGST-11/DGST-12 to Phase 23, and both are claimed and satisfied across the four plans.

### Anti-Patterns Found

None. `grep` for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` across all phase-modified files (`digest_chunk.go`, `digest.go`, `digest_format.go`, `client.go`, `notification_settings.sql`, `main.go`) returns no matches.

### Behavioral Spot-Checks / Full Verification Suite

| Check | Command | Result | Status |
|-------|---------|--------|--------|
| Build | `go build ./...` | clean | ✓ PASS |
| Vet | `go vet ./...` | clean | ✓ PASS |
| Lint | `golangci-lint run` | 0 issues | ✓ PASS |
| sqlc drift | `make sqlc-check` | clean | ✓ PASS |
| Unit tests (chunker/discord) | `go test ./internal/notifier/... ./internal/discord/... -run '...'` | all pass | ✓ PASS |
| Real-Postgres integration tests | `go test ./internal/notifier/ -run TestSendDigestIfDue -v` (TEST_DATABASE_URL set, `docker compose` Postgres already running) | 25/25 pass, including the partial-failure (success criterion 3), cap-drain, and budget-stop tests | ✓ PASS |
| Full backend suite | `go test ./... -count=1` | all 25 packages ok, 0 FAIL | ✓ PASS |
| Coverage gate | `make coverage-gate` | 91.24% (≥80% required) | ✓ PASS |
| Scope-diff (phase commits only, vs. pre-phase commit `9e87f4a`) | `git diff --exit-code 9e87f4a -- internal/poller web/ internal/db/migrations go.mod go.sum internal/notifier/scheduler.go` | no change | ✓ PASS |

Note: an initial scope-diff attempt against `main` showed differences, but those originate from earlier phases (22 and prior) already merged onto this feature branch, not from Phase 23's own commits — re-run against the pre-phase-23 commit confirms Phase 23 touched no out-of-scope file.

### Human Verification Required

None. All must-haves are verifiable programmatically and were verified against real Postgres, not mocks alone.

### Code Review Findings (Advisory — 23-REVIEW.md)

Two Warnings, two Info items, none blocking:
- **WR-01**: a dead `resuming = true` assignment inside `flushMidGroup` never takes effect (harmless today, flagged as a future-edit risk) — cosmetic/dead-code, not a behavioral gap.
- **WR-02**: `chunkForcingEventCount`/`capForcingEventCount` test fixtures are empirically-tuned magic constants without a precondition-pinning test (unlike `invariantBatchSize`, which has one) — test-maintainability concern, not a functional gap.
- **IN-01**: `remainderMarker` renders "1 events" (singular/plural grammar defect) in a rare edge case — minor UX nit.
- **IN-02**: an event with an unrecognized `EventType` would be silently dropped from grouping/acking — pre-existing behavior carried forward from before Phase 23, not a regression, and the reviewer assessed it as presumably unreachable given the app's enum discipline.

None of these affect the phase's three success criteria; all are appropriately non-blocking per the review's own disposition.

### Gaps Summary

No gaps. All four ROADMAP success criteria are implemented, wired, and independently proven with real-Postgres integration tests (not mocked at the DB layer) that specifically exercise the partial-failure/cap/budget edge cases the phase goal calls out. The full local Definition-of-Done gate suite (build, vet, lint, sqlc-check, real-Postgres test suite, coverage-gate) passes clean.

---

_Verified: 2026-09-18_
_Verifier: Claude (gsd-verifier)_
