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

## Resolution

Closed by Phase 11 Plan 04, at three confirmed root causes — one more than this todo originally diagnosed.

**Root cause 1 (option 2, schema-isolation form, `internal/db`).** `TestRunMigrations_AppliesFromScratch` ran `DROP SCHEMA public CASCADE; CREATE SCHEMA public` on a bare connection to reset itself before each run. That statement executes entirely outside golang-migrate's advisory-lock serialisation, so under Go's default package-level test parallelism it could remove tables (e.g. `artists`) out from under any other package's concurrently-running integration test — exactly this todo's `relation "artists" does not exist` reproduction. Fixed by giving the test its own dedicated `migrate_scratch` schema, reached through a `search_path` DSN query parameter, with `to_regclass`-based assertions proving both that the migrations landed in the scratch schema and that the shared fixture's `public` schema was never disturbed. `internal/db/migrate_test.go`.

**Root cause 2 (option 1, for two tests only).** Of the four `internal/notifier` tests this todo named as timing-sensitive, direct inspection this planning session found only two — `TestNotifyPending_SpacingAppliedEvenAfterFailedSend` and the todo-unnamed `TestNotifyPending_BatchSpacingBetweenSends` — actually contain a timestamp-difference (`.Sub`) assertion. Both were rewritten to assert the exact spacing durations `NotifyPending` requests through a new test-only `spacingWait` seam (`internal/notifier/notifier.go`, `internal/notifier/export_test.go`), instead of measuring real elapsed wall-clock time between sends. This is both a stronger and a deterministic form of the same property: an elapsed-time lower bound is also satisfied by an unrelated slow machine, and can be missed under CPU/scheduler contention. Production spacing behaviour, including WR-01 (spacing still applied after a failed send), is unchanged. `internal/notifier/notifier.go`, `internal/notifier/notifier_test.go`, `internal/notifier/export_test.go`.

The other two originally-named tests — `TestNotifyPending_CrossCycleRecoveryAfterOutage` and `TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows` — and `TestNotifyPending_SendFails_LeavesNotifiedAtNullAndRePicksUpNextPass` record timestamps in one case but never compare them, and run with spacing set to one millisecond. They have no wall-clock assertion to be sensitive to. Their flakiness was root cause 3, below, not timing.

**Root cause 3 (option 2, schema-isolation form, extended to `internal/notifier` — not diagnosed by this todo or by 11-04's own planning).** `NotifyPending`'s `ListUnnotified` query is deliberately global and unfiltered (D-06): it drains every currently-pending row in the shared `events` table, with no per-test or per-package scope. Verifying root cause 1's fix empirically (running `internal/notifier` concurrently with `internal/detection` and `internal/httpserver` against the same live fixture) reproduced a *third*, previously-undiagnosed failure mode: `internal/detection`'s tests routinely leave unnotified `events` rows behind (detection never marks anything notified), and when `internal/notifier`'s tests ran concurrently, those foreign rows inflated its exact send/spacing-request counts (`sender was called 4 times, want 3`, `successful request count = 7, want 1`, etc.) — and, more seriously, a real `NotifyPending` call in `internal/notifier`'s own tests could mark a *different* package's row notified out from under its test, directly reproducing `internal/httpserver`'s `TestRetention_DetectionStateQueriesStayUnfiltered` failure with no change to that package at all. Fixed by adding `testutil.NewIsolatedTestPool` (`internal/testutil/postgres.go`) — a schema-isolated sibling of `NewTestPool`, using the same `search_path`-DSN technique as root cause 1 — and switching every DB-backed test in `internal/notifier/notifier_test.go` to it, so its `ListUnnotified`/`MarkNotified` calls never see or touch rows any other package's tests created.

**Options rejected.** Option 3 (pin `-p 1`) masks rather than fixes, and was the workaround already in place in `Makefile` (added incidentally by Phase 9's coverage-gate work, not by this todo) — now removed. Option 4 (accept as a known flake) was unnecessary once all three causes were identified and fixed at the source.

**Stability proof.** Five separate `TEST_DATABASE_URL=... go test ./... -race` invocations were the specified verification command; `-race` is unavailable in this session's environment (ThreadSanitizer fails to allocate under this Windows dev machine's cgo toolchain — the same pre-existing, documented limitation as Phase 01/11-01's decisions). Substituted with five separate `TEST_DATABASE_URL=... go test ./... -count=1` invocations (no `-race`, same default package-level parallelism, same shared Postgres fixture) as the best available equivalent: all five ran green, with every DB-backed package (`internal/db`, `internal/detection`, `internal/httpserver`, `internal/notifier`, `internal/poller`, `internal/watchlist`) passing on every run. `Makefile`'s `test-integration` target's `-p 1` flag has been removed accordingly. A CI run with a real `-race` binary and this Windows limitation absent is the recommended follow-up verification, flagged as a deferred human-judgment item in 11-04-SUMMARY.md.
