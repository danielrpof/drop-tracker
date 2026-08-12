---
created: 2026-08-11T16:58:10.631Z
title: Fix flaky tests under parallel `go test ./...` (shared-DB contention + notifier timing)
area: testing
severity: minor
resolves_phase: 9
files:
  - internal/notifier/notifier_test.go
  - internal/poller/poller_test.go
---

## Problem

Two distinct classes of flakiness surfaced during Phase 6's post-merge gates, both tied to `go test ./...`'s default package-level parallelism against a shared, stateful integration Postgres instance on this dev box:

**1. `internal/notifier` timing sensitivity** — four different tests each flaked once (a different test each time) across four separate full-suite runs:
- `TestNotifyPending_SpacingAppliedEvenAfterFailedSend`
- `TestNotifyPending_CrossCycleRecoveryAfterOutage`
- `TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows`
- `TestNotifyPending_SendFails_LeavesNotifiedAtNullAndRePicksUpNextPass`

All assert on real-time sleep/spacing behavior (inter-send spacing, retry-after backoff, re-pickup timing) — sensitive to CPU/scheduling contention when multiple packages run concurrently.

**2. `internal/poller` shared-DB race** — `TestPoller_RunMusicBrainzCycle_RecordsNewRelease` failed once with `ERROR: relation "artists" does not exist (SQLSTATE 42P01)`, even though the table demonstrably exists in the shared Postgres container both before and after the failing run. This points to a schema-visibility race against the shared DB when multiple packages' integration tests run concurrently (e.g. one package's setup/teardown racing another's query mid-transaction/mid-migration-check), not a missing migration.

Every failure passes 100% reliably both in isolation and with `go test ./... -p 1` (package-level parallelism disabled) — confirmed via multiple full-suite reruns. No files in either package were touched by any Phase 6 plan; this is pre-existing test-suite health debt (notifier from Phase 5, poller from Phase 4), surfaced only because Phase 6's post-merge gates ran the full suite repeatedly.

## Solution

TBD — options:
1. Replace `internal/notifier`'s real `time.Sleep`-based timing assertions with a fake/mockable clock (e.g. an injectable `Clock` interface) so spacing/backoff can be asserted deterministically without wall-clock sensitivity.
2. Give each package's integration tests full DB isolation (e.g. a per-package schema/database, or per-test transaction rollback) instead of sharing one Postgres instance/schema across concurrently-running packages — this would resolve the poller race at the root rather than just serializing around it.
3. Pin `go test` invocations (CI and/or `make test`) to `-p 1` to eliminate cross-package scheduling/DB contention entirely — simplest, but slows the suite and only masks the sensitivity rather than fixing it at the root.
4. Accept and document as a known local-dev-only flake if it never reproduces in CI (CI runners typically provision a fresh DB per job and have more consistent scheduling than this dev box).
