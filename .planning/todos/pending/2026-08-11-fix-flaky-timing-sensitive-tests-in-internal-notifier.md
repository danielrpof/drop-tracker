---
created: 2026-08-11T16:58:10.631Z
title: Fix flaky timing-sensitive tests in internal/notifier
area: testing
severity: minor
files:
  - internal/notifier/notifier_test.go
---

## Problem

Three different tests in `internal/notifier` flaked once each (a different test each time) across three separate full `go test ./...` runs during Phase 6's post-merge gates:

- `TestNotifyPending_SpacingAppliedEvenAfterFailedSend`
- `TestNotifyPending_CrossCycleRecoveryAfterOutage`
- `TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows`

All three assert on real-time sleep/spacing behavior (inter-send spacing, retry-after backoff). Each passes 100% reliably in isolation (5+ reruns each) and the full suite passes cleanly with `go test ./... -p 1` (package-level parallelism disabled). Root cause: these tests are sensitive to CPU/scheduling contention when multiple packages' tests run concurrently on this dev box — not a functional bug in the notifier code itself. No `internal/notifier` files were touched by any Phase 6 plan; this is pre-existing test-suite health debt from Phase 5.

## Solution

TBD — options:
1. Replace real `time.Sleep`-based timing assertions with a fake/mockable clock (e.g. an injectable `Clock` interface) so spacing/backoff can be asserted deterministically without wall-clock sensitivity.
2. Pin `go test` invocations (CI and/or `make test`) to `-p 1` to eliminate cross-package scheduling contention — simpler but slows the suite and only masks the sensitivity rather than fixing it.
3. Accept and document as a known local-dev-only flake if it never reproduces in CI (CI runners typically have more consistent scheduling than this dev box).
