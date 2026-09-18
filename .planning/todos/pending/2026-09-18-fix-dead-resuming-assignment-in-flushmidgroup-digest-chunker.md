---
created: 2026-09-18T17:25:03.367Z
title: Fix dead `resuming = true` assignment in flushMidGroup digest chunker
area: notifier
severity: minor
files:
  - internal/notifier/digest_chunk.go:239-249
  - internal/notifier/digest_chunk.go:272-278
---

## Problem

`splitOversizedGroup`'s `flushMidGroup` closure (`internal/notifier/digest_chunk.go:239-249`) sets `resuming = true` before returning:

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

But the call site immediately overwrites it, unconditionally, in the very same loop iteration, before the next iteration ever gets a chance to read it:

```go
if curLen > 0 && curLen+n > chunkContentBudget {
    flushMidGroup()
    heading = continuationHeading(g.title) + "\n"   // heading already set directly here
    unit = heading + entry.text
    n = utf8.RuneCountInString(unit)
}
resuming = false   // clobbers flushMidGroup's write on every call
```

Since `flushMidGroup` can only ever be invoked while `curLen > 0`, and the only other place that leaves `resuming == true` for the *next* iteration is the oversized-single-line branch (which requires `curLen == 0` and always `continue`s before this reset line runs), the assignment inside `flushMidGroup` can never be observed by anything.

It is currently harmless only because the call site *also* recomputes `heading` directly instead of relying on the `case resuming:` switch arm — but that duplication is itself the risk: a future edit that removes the seemingly-redundant explicit `heading = continuationHeading(...)` recompute (believing the `resuming` flag already handles it) would silently drop the continuation heading on every ordinary mid-group split, not just the rare oversized-single-line one.

Surfaced by Phase 23 code review (WR-01, `.planning/milestones/v1.5-phases/23-digest-readability-discord-limits/23-REVIEW.md`) and left unfixed at v1.5 milestone close (2026-09-18).

## Solution

Either delete the dead assignment and rely solely on the explicit `heading = continuationHeading(...)` recompute at the call site (simplest, matches current behavior):

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

Or remove the redundant explicit recompute and let the *next* iteration's `switch` pick up `case resuming:` — requires restructuring the loop so the reset doesn't run before the next iteration reads it. The former is the lower-risk fix.
