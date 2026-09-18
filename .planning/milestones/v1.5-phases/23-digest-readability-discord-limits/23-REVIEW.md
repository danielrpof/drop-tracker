---
phase: 23-digest-readability-discord-limits
reviewed: 2026-09-18T15:34:57Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - cmd/server/main.go
  - internal/db/sqlc/notification_settings.sql.go
  - internal/db/sqlc/querier.go
  - internal/discord/client.go
  - internal/discord/client_test.go
  - internal/notifier/digest.go
  - internal/notifier/digest_chunk.go
  - internal/notifier/digest_chunk_test.go
  - internal/notifier/digest_format.go
  - internal/notifier/digest_format_test.go
  - internal/notifier/digest_test.go
  - internal/notifier/export_test.go
  - queries/notification_settings.sql
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 23: Code Review Report

**Reviewed:** 2026-09-18T15:34:57Z
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

## Summary

Reviewed the full digest-chunking/readability rework: `digest.go`'s per-chunk
send/ack loop, `digest_chunk.go`'s grouping/splitting/cap/header logic,
`digest_format.go`'s line rendering, the new `AckEventsOnly` query, and the
`discord.ErrRateLimited` sentinel addition. Traced the rune-budget arithmetic
(`chunkContentBudget` = 4096 − `chunkOverheadReserve`(300)) against every
place text gets stamped onto a chunk after the content-splitting pass
(window header, position indicator, remainder marker, trailing continuation
note) and against the pass itself (continuation heading), and confirmed the
budget can never be exceeded for any combination the code can actually
produce — this matches the project's own invariant tests
(`TestChunkInvariant1_EveryChunkAtMostDiscordLimit`,
`TestChunkInvariant4_CappedRunEveryChunkAtMostDiscordLimit`,
`TestChunkDigest_ContinuationMarkersNeverExceedBudget`). Traced the
ack-path branching in `SendDigestIfDue` (`ackEventsOnly` vs `ackDigestBatch`,
gated by `i < len(chunks)-1 || deferred > 0`) against the id-accounting
invariant (kept-chunk ids + deferred == len(sendable)) and found it holds.
`go build ./...` and `go vet` are clean for the reviewed packages.

No Critical/Blocker findings. Two Warnings (one dead-code correctness smell
in the oversized-group splitter, one test-reliability concern around
magic empirically-tuned fixture sizes) and two Info-level nits (a grammar
slip in a user-facing string, and a theoretical unguarded event-type
mismatch that was already present before this phase and is not a
regression).

## Warnings

### WR-01: Dead `resuming = true` assignment inside `flushMidGroup` never takes effect

**File:** `internal/notifier/digest_chunk.go:239-249` (closure), consumed at `digest_chunk.go:272-278`

**Issue:** `splitOversizedGroup`'s `flushMidGroup` closure sets `resuming = true`
before returning:

```go
flushMidGroup := func() {
    if curLen == 0 {
        return
    }
    cur.WriteString(continuationNote(g.title))
    chunks = append(chunks, digestChunk{description: cur.String(), ids: curIDs})
    cur.Reset()
    curIDs = nil
    curLen = 0
    resuming = true
}
```

But the call site immediately overwrites it, unconditionally, in the very
same loop iteration, before the next iteration ever gets a chance to read
it:

```go
if curLen > 0 && curLen+n > chunkContentBudget {
    flushMidGroup()
    heading = continuationHeading(g.title) + "\n"   // heading already set directly here
    unit = heading + entry.text
    n = utf8.RuneCountInString(unit)
}
resuming = false   // <-- clobbers flushMidGroup's write on every call
```

Since `flushMidGroup` can only ever be invoked while `curLen > 0`, and the
only other place that leaves `resuming == true` for the *next* iteration is
the oversized-single-line branch (which requires `curLen == 0` and always
`continue`s before this reset line runs), the assignment inside
`flushMidGroup` can never be observed by anything. It is currently harmless
only because the call site *also* recomputes `heading` directly instead of
relying on the `case resuming:` switch arm — but that duplication is itself
the risk: a future edit that removes the seemingly-redundant explicit
`heading = continuationHeading(...)` recompute (believing the `resuming`
flag already handles it) would silently drop the continuation heading on
every ordinary mid-group split, not just the rare oversized-single-line one.

**Fix:** Either delete the dead assignment and rely solely on the explicit
`heading = continuationHeading(...)` recompute at the call site (simplest,
matches current behavior), or remove the redundant explicit recompute and
let the *next* iteration's `switch` pick up `case resuming:` — which
requires restructuring the loop so the reset doesn't run before the next
iteration reads it. The former is the lower-risk fix:

```go
flushMidGroup := func() {
    if curLen == 0 {
        return
    }
    cur.WriteString(continuationNote(g.title))
    chunks = append(chunks, digestChunk{description: cur.String(), ids: curIDs})
    cur.Reset()
    curIDs = nil
    curLen = 0
    // resuming is set by the caller directly (heading recompute below);
    // no need to also flip it here.
}
```

### WR-02: Chunk-count fixtures in `digest_test.go` depend on undocumented, unpinned magic constants

**File:** `internal/notifier/digest_test.go:139-166`

**Issue:** `chunkForcingEventCount = 75` and `capForcingEventCount = 600` are
justified only by code comments claiming they were "empirically confirmed
... to split into exactly 3 chunks" / "22 uncapped chunks", against a
private whitebox probe that isn't checked into the repo and isn't re-run by
CI. `digest_chunk_test.go`'s own `invariantBatchSize` constant, by contrast,
has a dedicated precondition-pinning test
(`TestChunkDigest_InvariantBatchProducesAtLeast20Chunks`) that fails with a
clear, actionable message ("raise invariantBatchSize") if the assumption
ever stops holding. `chunkForcingEventCount`/`capForcingEventCount` have no
equivalent guard: if any upstream change shifts per-line rendering length
(e.g. widening `digestWindowHeader`, changing `digestTitleLimit`, or editing
unrelated padding text in `syntheticEvents`/`insertPendingEventsForChunking`),
these external-package integration tests will fail with a bare
`sender.calls = N, want 3` (or `want 20`) with no hint that the fixture size
itself needs adjusting, rather than that production logic regressed.

**Fix:** Add a small precondition test in `digest_test.go` (or reuse the
whitebox `chunkDigest`/`buildDigestGroups` helpers via a package-internal
probe) that asserts `chunkForcingEventCount` produces exactly 3 chunks and
`capForcingEventCount` produces exactly 22 uncapped chunks, with a failure
message that says which constant to adjust — mirroring
`TestChunkDigest_InvariantBatchProducesAtLeast20Chunks`'s pattern.

## Info

### IN-01: `remainderMarker` renders an incorrect singular/plural ("1 events")

**File:** `internal/notifier/digest_chunk.go:355-357`

**Issue:**

```go
func remainderMarker(remaining int) string {
	return fmt.Sprintf(" · %d events still pending, continuing in the next digest", remaining)
}
```

When exactly one event is deferred past the `maxDigestChunks` cap, this
renders "· 1 events still pending, continuing in the next digest" — a minor
but user-visible grammar defect in an operator-facing Discord message.

**Fix:**

```go
func remainderMarker(remaining int) string {
	noun := "events"
	if remaining == 1 {
		noun = "event"
	}
	return fmt.Sprintf(" · %d %s still pending, continuing in the next digest", remaining, noun)
}
```

### IN-02: An event with an unrecognized `EventType` is silently dropped from grouping and never acked (pre-existing, not a regression)

**File:** `internal/notifier/digest_chunk.go:119-142` (`buildDigestGroups`)

**Issue:** `buildDigestGroups` only iterates the three fixed
`digestHeadings` entries when building groups/ids; an `sqlc.Event` whose
`EventType` doesn't match `eventTypeNewRelease`/`eventTypeGuestFeature`/
`eventTypeDeluxeChange` is bucketed into `grouped[...]` but never read back
out, so its id never appears in any `digestChunk.ids`. Combined with
`SendDigestIfDue`'s new per-chunk id-accounting invariant
(kept-chunk-ids + deferred == len(sendable)), such an event would be
counted in `len(sendable)` for the "digest sent" summary log's
`sent_count` field but never actually sent or acked, and would silently
reappear in every future digest's `listUnnotified` result forever. This
exact filtering behavior already existed in the pre-phase-23
`buildDigestEmbed`, so it is not a regression introduced by this phase, and
is presumably unreachable in practice given the `event_type` column's
application-level enum discipline — flagging only because the new
per-chunk ack/id-sum invariant this phase introduces makes the failure mode
(silent, permanent non-ack) slightly worse than the old failure mode
(dropped from one embed's text but still acked as part of the whole-batch
`ackDigestBatch` call). Not asking for a fix in this phase; noting for
awareness given the surrounding code now leans harder on "ids in a chunk"
matching "ids in sendable".

---

_Reviewed: 2026-09-18T15:34:57Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
