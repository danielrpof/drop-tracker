---
created: 2026-09-18T17:25:03.367Z
title: Pin chunk-count test fixture constants with a precondition test
area: notifier
severity: minor
files:
  - internal/notifier/digest_test.go:139-166
  - internal/notifier/digest_chunk_test.go
---

## Problem

`chunkForcingEventCount = 75` and `capForcingEventCount = 600` in `internal/notifier/digest_test.go:139-166` are justified only by code comments claiming they were "empirically confirmed ... to split into exactly 3 chunks" / "22 uncapped chunks", against a private whitebox probe that isn't checked into the repo and isn't re-run by CI.

`digest_chunk_test.go`'s own `invariantBatchSize` constant, by contrast, has a dedicated precondition-pinning test (`TestChunkDigest_InvariantBatchProducesAtLeast20Chunks`) that fails with a clear, actionable message ("raise invariantBatchSize") if the assumption ever stops holding. `chunkForcingEventCount`/`capForcingEventCount` have no equivalent guard: if any upstream change shifts per-line rendering length (e.g. widening `digestWindowHeader`, changing `digestTitleLimit`, or editing unrelated padding text in `syntheticEvents`/`insertPendingEventsForChunking`), these external-package integration tests will fail with a bare `sender.calls = N, want 3` (or `want 20`) with no hint that the fixture size itself needs adjusting, rather than that production logic regressed.

Surfaced by Phase 23 code review (WR-02, `.planning/milestones/v1.5-phases/23-digest-readability-discord-limits/23-REVIEW.md`) and left unfixed at v1.5 milestone close (2026-09-18).

## Solution

Add a small precondition test in `digest_test.go` (or reuse the whitebox `chunkDigest`/`buildDigestGroups` helpers via a package-internal probe) that asserts `chunkForcingEventCount` produces exactly 3 chunks and `capForcingEventCount` produces exactly 22 uncapped chunks, with a failure message that says which constant to adjust — mirroring `TestChunkDigest_InvariantBatchProducesAtLeast20Chunks`'s pattern.
