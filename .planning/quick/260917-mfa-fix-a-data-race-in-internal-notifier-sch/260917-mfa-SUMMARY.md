---
phase: quick/260917-mfa
plan: 01
subsystem: internal/notifier
tags: [testing, race-detector, ci]
status: complete
dependency-graph:
  requires: []
  provides: [22-CI-RACE-01]
  affects: [internal/notifier]
tech-stack:
  added: []
  patterns: [mutex-guarded-log-buffer, drain-checked-stop-before-assert]
key-files:
  created: []
  modified:
    - internal/notifier/scheduler_test.go
decisions:
  - "Added a scheduler-test-local syncBuffer + newSyncTestLogger instead of widening the shared newTestLogger helper (60+ call sites, none of which race) -- kept the diff to the one file and the one racing test."
  - "Moved the Stop() call ahead of both log-buffer assertions and checked its error, since Stop's own Info log also writes to the buffer and its completed drain is the only real happens-before edge proving the loop goroutine is done writing."
  - "make test's -race flag is cgo-blocked on this Windows dev box (pre-existing, documented in STATE.md Phase 11.1-04) -- substituted a plain (non -race) go test run with the same TEST_DATABASE_URL/coverpkg for local DoD verification; the actual -race proof came from WSL2 per the plan's design_contract, and CI's own test job (which runs on Linux) is the authoritative -race gate."
metrics:
  duration: ~35min
  completed: 2026-09-17
actuals:
  tokens: 10000
  tasks: 2
  commits: 1
---

# Phase quick/260917-mfa Plan 01: Fix a data race in internal/notifier/scheduler_test.go Summary

Closed a genuine `go test -race` data race in `TestDigestScheduler_CheckError_LoggedAndLoopContinues` by
replacing its bare `*bytes.Buffer` log sink with a mutex-guarded `syncBuffer`, and by moving the test's
`Stop()` call ahead of its log-buffer assertions so the loop goroutine is provably done writing before
the test reads.

## What Was Built

CI run [35272810632](https://github.com/danielrpof/drop-tracker/actions/runs/35272810632) failed the
`test` job under `go test -race`, skipping `build-scan` and blocking the Phase 22 UAT smoke test. The
race was real: `fakeSink.SendDigestIfDue` signals its `notify` channel before calling `fn()`, so
`waitForCallCount(sink, 2, ...)` can unblock while the scheduler's loop goroutine is still inside
`check()`'s `s.logger.Error("digest due-check failed", ...)` write to the test's shared `*bytes.Buffer`.
The test then read that same buffer via `buf.String()` -- a Read/Write race `bytes.Buffer` cannot survive.

Re-ordering alone was not sufficient: `DigestScheduler.Stop()` itself logs (`s.logger.Info("digest
scheduler stopping")`) before it waits on `<-s.done`, so simply moving `Stop()` earlier would only trade
one race for a Write/Write race between `Stop`'s log call and an in-flight `check()`'s log call.

Fix, in `internal/notifier/scheduler_test.go` only:

1. Added `syncBuffer` (mutex-guarded `bytes.Buffer` wrapper, `Write`/`String` only) and
   `newSyncTestLogger` (mirrors the existing `newTestLogger` but binds a `syncBuffer`), scoped to this
   file.
2. `TestDigestScheduler_CheckError_LoggedAndLoopContinues` now builds its logger via
   `newSyncTestLogger`, and calls `sched.Stop(stopCtx)` -- checking its error -- immediately after the
   second `waitForCallCount`, before either of the two log/count assertions. A non-nil `Stop` error now
   fails the test loudly (drain deadline expired, loop still live) instead of silently reading a buffer
   that might still be written to.

No production code changed. `fakeSink`, `waitForCallCount`, `callCount`, `callAt`, and the other seven
`TestDigestScheduler_*` tests are untouched, as is `notifier_test.go`'s `newTestLogger` (60+ call sites,
none of which have a live background writer).

## RED Evidence (before the fix)

`wsl.exe -d Ubuntu -e bash -lc 'cd /mnt/c/CodeProjects/drop-tracker && unset TEST_DATABASE_URL DATABASE_URL && go test ./internal/notifier/... -race -count=5 -run TestDigestScheduler_CheckError_LoggedAndLoopContinues'`
reproduced the race on the very first of 5 iterations:

```
==================
WARNING: DATA RACE
Read at 0x00c000129968 by goroutine 9:
  bytes.(*Buffer).String()
      /usr/lib/go-1.26/src/bytes/buffer.go:77 +0x6e4
  github.com/danielrpof/drop-tracker/internal/notifier_test.TestDigestScheduler_CheckError_LoggedAndLoopContinues()
      /mnt/c/CodeProjects/drop-tracker/internal/notifier/scheduler_test.go:260 +0x6d7
  testing.tRunner()
      /usr/lib/go-1.26/src/testing/testing.go:2036 +0x21c
  testing.(*T).Run.gowrap1()
      /usr/lib/go-1.26/src/testing/testing.go:2101 +0x38

Previous write at 0x00c000129968 by goroutine 10:
  bytes.(*Buffer).grow()
      /usr/lib/go-1.26/src/bytes/buffer.go:172 +0x3b1
  bytes.(*Buffer).Write()
      /usr/lib/go-1.26/src/bytes/buffer.go:197 +0xc4
  log/slog.(*commonHandler).handle()
      /usr/lib/go-1.26/src/log/slog/handler.go:322 +0xa08
  log/slog.(*JSONHandler).Handle()
      /usr/lib/go-1.26/src/log/slog/json_handler.go:89 +0xa8
  log/slog.(*Logger).log()
      /usr/lib/go-1.26/src/log/slog/logger.go:256 +0x2b6
  log/slog.(*Logger).Error()
      /usr/lib/go-1.26/src/log/slog/logger.go:229 +0x1f8
  github.com/danielrpof/drop-tracker/internal/notifier.(*DigestScheduler).check()
      /mnt/c/CodeProjects/drop-tracker/internal/notifier/scheduler.go:119 +0x115
  github.com/danielrpof/drop-tracker/internal/notifier.(*DigestScheduler).Start.func1()
      /mnt/c/CodeProjects/drop-tracker/internal/notifier/scheduler.go:104 +0x124

Goroutine 9 (running) created at: testing.(*T).Run() -> testing.runTests.func1() -> testing.tRunner()
Goroutine 10 (running) created at: DigestScheduler.Start() -> TestDigestScheduler_CheckError_LoggedAndLoopContinues() -> testing.tRunner()
--- FAIL: TestDigestScheduler_CheckError_LoggedAndLoopContinues (0.00s)
FAIL
```

Exactly as the plan predicted: `bytes.(*Buffer).Write` from `slog.Error` from `DigestScheduler.check()`
at `scheduler.go:119`, racing the test goroutine's `buf.String()` read at `scheduler_test.go:260`.

## GREEN Evidence (after the fix)

All four WSL2 `-race` runs clean, zero `WARNING: DATA RACE`:

1. Single test, verbose, once: `PASS`, `ok github.com/danielrpof/drop-tracker/internal/notifier 1.025s`
2. Same test, `-count=50`: `ok github.com/danielrpof/drop-tracker/internal/notifier 1.084s`
3. Whole `TestDigestScheduler` family, `-count=20`: `ok github.com/danielrpof/drop-tracker/internal/notifier 1.195s`
4. Full package, `-race -count=1`: `ok github.com/danielrpof/drop-tracker/internal/notifier 1.280s`

`go vet ./...` and `golangci-lint run` (Windows-native): both clean (`0 issues.`).

## Definition of Done

- `go vet ./...` -- clean
- `golangci-lint run` -- `0 issues.`
- `make db-up` -- Postgres container healthy
- `make test` -- **substituted** with a plain (non-`-race`) `go test ./... -count=1 -coverprofile=coverage.out -coverpkg=...` run using the same `TEST_DATABASE_URL` and `COVER_PKGS` `make test-integration` would use. `go test -race` itself fails to build on this Windows box with `runtime/cgo: cgo.exe: exit status 2` -- the pre-existing cgo toolchain break already documented in STATE.md (Phase 11.1-04), unrelated to this change. All 24 packages passed (`ok`), 0 failures. The actual `-race` proof for the fixed test is the WSL2 evidence above; CI's own `test` job (Linux runner) is the authoritative `-race` gate and is confirmed green below.
- `make coverage-gate` -- **91.30%** (required: 80%), unchanged from the prior baseline (test-only change).
- `make sqlc-check` -- clean, no drift.
- Pre-commit hooks (gitleaks, golangci-lint) ran normally on commit; no `--no-verify` used.

## CI Confirmation

Pushed commit `58ada7a` to `gsd/phase-22-scheduled-digest-send`. Fresh CI run:
[35276004417](https://github.com/danielrpof/drop-tracker/actions/runs/35276004417) -- **all jobs green**,
including `test` (1m19s) and `build-scan` (1m14s, no longer skipped). This unblocks the Phase 22 UAT
smoke test.

## Deviations from Plan

### Auto-fixed Issues

None beyond the documented cgo substitution above (not a Rule 1/2/3 fix -- the substitution is a
verification-method swap, not a code change, and mirrors the identical precedent already recorded in
STATE.md for Phase 11.1-04).

## Self-Check: PASSED

- `internal/notifier/scheduler_test.go` exists and contains `syncBuffer`/`newSyncTestLogger`: confirmed via Edit tool output.
- Commit `58ada7a` exists on `gsd/phase-22-scheduled-digest-send`: confirmed via `git log --oneline -3`.
- CI run `35276004417` is green with `test` and `build-scan` both passed: confirmed via `gh run view`.
