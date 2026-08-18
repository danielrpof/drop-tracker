---
status: resolved
phase: 11-bounded-concurrent-polling
source: [11-VERIFICATION.md]
started: 2026-08-17T06:35:00Z
updated: 2026-08-17T11:45:00Z
---

## Current Test

[testing complete]

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
result: issue
reported: "Pre-emptively fix it now: no VPS is provisioned yet (deploy is deferred per PROJECT.md), so size MaxConns explicitly in internal/db/pool.go so the pool ceiling comfortably exceeds the worker ceiling (MusicBrainzPollWorkers + DeezerPollWorkers, default 8) regardless of the eventual deployment target's vCPU count, instead of deferring the decision."
severity: minor

## Summary

total: 1
passed: 0
issues: 1
pending: 0
skipped: 0

## Gaps

- gap_id: G-11-1
  truth: "The pool's connection ceiling should comfortably exceed the maximum simultaneous DB-touching goroutines both cycles' worker pools can produce, or the operator should knowingly accept pool-acquire queueing as an acceptable v1.1 tradeoff."
  status: resolved
  reason: "User reported: Pre-emptively fix it now: no VPS is provisioned yet, so size MaxConns explicitly rather than deferring to deployment."
  severity: minor
  test: 1
  root_cause: "internal/db/pool.go's PoolConfig (lines 92-105) parses the DSN via pgxpool.ParseConfig and sets ConnectTimeout, PingTimeout, and MaxConnIdleTime explicitly, but never sets cfg.MaxConns. pgxpool therefore falls back to its own default, max(4, runtime.NumCPU()), evaluated on whatever host the process runs on. MusicBrainzPollWorkers + DeezerPollWorkers default to 3 + 5 = 8 concurrent DB-touching goroutines (internal/config/config.go:54-55), which can exceed that default pool ceiling on any host with fewer than 8 vCPUs."
  artifacts:
    - path: "internal/db/pool.go"
      issue: "PoolConfig never sets cfg.MaxConns, so pool size is not sized against the worker-pool ceiling it must serve"
    - path: "internal/config/config.go"
      issue: "MusicBrainzPollWorkers/DeezerPollWorkers define the concurrency ceiling PoolConfig should be sized against, but nothing wires the two together"
  missing:
    - "Set cfg.MaxConns in PoolConfig to a value that comfortably exceeds MusicBrainzPollWorkers + DeezerPollWorkers, independent of runtime.NumCPU() on the deploy host"
    - "Keep operator override intact: PoolConfig already skips overriding ConnectTimeout when the DSN sets connect_timeout explicitly (pool.go:98) — MaxConns should follow the same precedent (an operator-set pool_max_conns in the DSN wins) rather than force-overwriting it unconditionally"
  debug_session: ""
