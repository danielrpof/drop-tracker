---
status: testing
phase: 01-foundation-data-layer-config-health
source: [01-VERIFICATION.md]
started: 2026-08-05T18:17:40Z
updated: 2026-08-05T18:17:40Z
---

## Current Test

number: 1
name: Graceful shutdown under a real SIGTERM (WR-03)
expected: |
  The process logs "shutdown signal received, shutting down gracefully",
  httpSrv.Shutdown drains or bounds in-flight requests within the 10s timeout,
  the deferred pool.Close() runs, and the process exits cleanly (0) rather
  than being killed mid-request.
awaiting: user response

## Tests

### 1. Graceful shutdown under a real SIGTERM (WR-03)
expected: |
  Run the built binary in a real POSIX environment (Linux container or Phase 7's CI),
  send it a genuine SIGTERM (e.g. `docker stop` on a container running the image, or
  `kill -TERM <pid>` on Linux) while an in-flight request is outstanding. Expect the
  process to log "shutdown signal received, shutting down gracefully", drain/bound
  in-flight requests within the 10s shutdown timeout, close the DB pool, and exit 0 —
  not be killed mid-request.
  Why human: Windows (this dev sandbox) has no true POSIX SIGTERM — `kill <pid>` here
  terminates the process directly without invoking Go's signal.NotifyContext handler,
  so this code path (present and correctly wired per source review) has never been
  exercised end-to-end.
result: [pending]

### 2. Race-detector confirmation of the migration-cancellation goroutine (WR-01)
expected: |
  Run `go test -race ./internal/db/... -run TestRunMigrations -v -count=1` on a machine
  or CI runner with a working C toolchain (e.g. Phase 7's Linux GitHub Actions runner).
  Expect all TestRunMigrations_* tests to pass with -race and no data race reported,
  confirming the background goroutine running m.Up() does not race with the deferred
  sqlDB.Close() on the context-cancellation path.
  Why human: This Windows machine's MSYS2/mingw64 gcc toolchain cannot execute cc1.exe,
  breaking `go test -race` at the toolchain level for any package. The underlying
  behavior is structurally argued to be safe and all tests pass without -race, but the
  specific concurrency claim introduced by the WR-01 fix has never been confirmed by an
  actual race detector.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
