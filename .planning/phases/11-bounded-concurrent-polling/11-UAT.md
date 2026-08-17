---
status: testing
phase: 11-bounded-concurrent-polling
source: [11-VERIFICATION.md]
started: 2026-08-17T06:35:00Z
updated: 2026-08-17T06:35:00Z
---

## Current Test

number: 1
name: DB connection-pool sizing under concurrent polling on the real deployment target
expected: |
  The pool's connection ceiling should comfortably exceed the maximum
  simultaneous DB-touching goroutines both cycles' worker pools can
  produce, or you knowingly accept pool-acquire queueing (a throughput
  slowdown, not a correctness break) as an acceptable v1.1 tradeoff.
awaiting: user response

## Tests

### 1. DB connection-pool sizing under concurrent polling on the real deployment target
expected: |
  On the project's actual deployment target (a small VPS per DPLY-01),
  check `runtime.NumCPU()` and compare it against
  `MusicBrainzPollWorkers + DeezerPollWorkers` (default 3 + 5 = 8).
  `internal/db/pool.go`'s `PoolConfig` never sets `pgxpool.Config.MaxConns`
  explicitly, so it inherits pgxpool's own default of
  `max(4, runtime.NumCPU())`. On a 1-2 vCPU VPS that defaults the pool to
  4 connections, below the 8 that could contend for one under full
  concurrent polling. The pool's connection ceiling should comfortably
  exceed the maximum simultaneous DB-touching goroutines both worker
  pools can produce, or you knowingly accept pool-acquire queueing (a
  throughput slowdown, not a correctness break — no deadlock, no data
  loss) as an acceptable v1.1 tradeoff.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0

## Gaps
