---
created: 2026-09-18T17:25:03.367Z
title: Fix singular/plural grammar in digest remainder marker
area: notifier
severity: cosmetic
files:
  - internal/notifier/digest_chunk.go:355-357
---

## Problem

```go
func remainderMarker(remaining int) string {
	return fmt.Sprintf(" · %d events still pending, continuing in the next digest", remaining)
}
```

When exactly one event is deferred past the `maxDigestChunks` cap, this renders "· 1 events still pending, continuing in the next digest" — a minor but user-visible grammar defect in an operator-facing Discord message.

Surfaced by Phase 23 code review (IN-01, `.planning/milestones/v1.5-phases/23-digest-readability-discord-limits/23-REVIEW.md`) and left unfixed at v1.5 milestone close (2026-09-18).

## Solution

```go
func remainderMarker(remaining int) string {
	noun := "events"
	if remaining == 1 {
		noun = "event"
	}
	return fmt.Sprintf(" · %d %s still pending, continuing in the next digest", remaining, noun)
}
```
