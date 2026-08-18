---
phase: 11-bounded-concurrent-polling
plan: 05
subsystem: db
tags: [connection-pool, pgxpool, gap-closure]
status: complete

dependency-graph:
  requires: [11-01]
  provides: [pool-maxconns-sizing]
  affects: [cmd/server/main.go, internal/db, internal/testutil, internal/httpserver]

tech-stack:
  added: []
  patterns:
    - "Separate pgx.ParseConfig call to read RuntimeParams for operator-intent detection, since pgxpool.ParseConfig's returned Config already consumed/deleted the DSN key it would otherwise be checked against"

key-files:
  created: []
  modified:
    - internal/db/pool.go
    - internal/db/pool_timeout_test.go
    - cmd/server/main.go
    - internal/testutil/postgres.go
    - internal/httpserver/boot_e2e_test.go

decisions:
  - "PoolConfig/NewPool take pollWorkers as a plain extra int parameter, not a functional option -- matches PoolConfig's existing plain-parameter style and keeps the coupling to the worker ceiling visible at every call site; poller.Option exists for many optional knobs, this is one required, always-relevant input"
  - "Operator-set pool_max_conns in the DSN always wins, even when it sits below the computed worker-ceiling default -- a tight server-side max_connections or PgBouncer limit is a real reason to cap it lower"
  - "Test pools (testutil.NewTestPool / NewIsolatedTestPool) pass pollWorkers=0, since test pools serve no poll cycles; this yields MaxConns=4 (headroom only), same floor pgxpool would apply on a small CI runner"

actuals:
  tokens: 3613
  tasks: 2
  commits: 2
---

# Phase 11 Plan 05: Bounded Concurrent Polling — Pool MaxConns Sizing Summary

Sized the Postgres connection pool's `MaxConns` against the poll-worker concurrency it must actually serve (`MusicBrainzPollWorkers + DeezerPollWorkers` + a 4-connection headroom) instead of letting it default to pgxpool's own `max(4, runtime.NumCPU())`, while keeping an operator's explicit `pool_max_conns` DSN setting authoritative even when it is smaller than the computed default.

## What Was Built

**Task 1 (tracer, TDD):** `internal/db/pool.go`'s `PoolConfig`/`NewPool` now take a `pollWorkers int` argument. A new `poolMaxConnsForWorkers` helper clamps the worker count into `[0, 1000]` (overflow guard only, not a policy ceiling) and returns it plus a documented `pollWorkerHeadroom` of 4. A new `dsnSetsMaxConns` helper detects operator intent via a **separate** `pgx.ParseConfig(dsn)` call checking `RuntimeParams["pool_max_conns"]` — this is the only reliable signal, since `pgxpool.ParseConfig`'s own returned `Config.MaxConns` is always populated (defaulting to `max(4, runtime.NumCPU())`) and the `pool_max_conns` key is deleted from `RuntimeParams` as it's consumed, making an operator's explicit value indistinguishable from an incidental default on that path.

All five call sites were updated: `cmd/server/main.go` passes `cfg.MusicBrainzPollWorkers + cfg.DeezerPollWorkers`; `internal/testutil/postgres.go`'s two constructors pass `0`; `internal/db/pool_timeout_test.go`'s three pre-existing tests pass an explicit literal (`8`, irrelevant to what they assert); `internal/httpserver/boot_e2e_test.go` mirrors `main.go`'s real argument. Two new tests were added: `TestPoolConfig_ComputesMaxConnsFromPollWorkers` (table-driven: worker ceilings 8→12, 64→68, 0→4, pinning the exact value rather than merely "greater than", so a future edit can't quietly reintroduce a `runtime.NumCPU()` dependence) and `TestPoolConfig_RespectsExplicitMaxConnsInDSN` (`pool_max_conns=6` with worker ceiling 8 still yields 6).

**Task 2:** `TestBootToHealth_EndToEnd` gained the key-link assertion: after constructing the pool through the real boot chain, it asserts `pool.Config().MaxConns >= cfg.MusicBrainzPollWorkers + cfg.DeezerPollWorkers` — proving the worker counts `config.Load` produces actually reach the pool the production boot path builds, not just that `PoolConfig` computes the right value in isolation (already pinned by Task 1's unit tests). Then the full `-race` suite and `make coverage-gate` were re-run against the real Postgres fixture to confirm nothing regressed.

## Verification Evidence

- `go build ./...`, `go vet ./...` — clean.
- `go test ./internal/db/ -run 'TestPoolConfig' -race -count=1` — 5/5 pass (run via WSL2 Ubuntu; see Deviations below for why).
- `golangci-lint run ./...` — 0 issues, no new `nolint` directive added.
- `make test` (full suite, `-race`, real Postgres fixture on port 5433) — every package `ok`, zero `FAIL` lines; `TestBootToHealth_EndToEnd` confirmed running (not skipped).
- `make coverage-gate` — **PASS**, 87.0% aggregate backend coverage (required: 80%).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] `-race` fails natively on this Windows dev box; test suite run via WSL2 Ubuntu instead**
- **Found during:** Task 1 verification
- **Issue:** `go test -race` on native Windows failed every package (including packages untouched by this plan, e.g. `internal/config`) with `ThreadSanitizer failed to allocate ... (error code: 87)` — a pre-existing environmental limitation on this dev machine, matching STATE.md's already-documented pattern of `-race`/cgo toolchain breaks on this box (see Phase 01-02/01-03 decisions and the Phase 01 UAT note about verifying WR-03 via WSL2).
- **Fix:** Ran all `-race` verification (`go test ./internal/db/ -run 'TestPoolConfig' -race`, `make test`) via `wsl -d Ubuntu`, which has Go 1.26 and Docker Desktop's shared engine (same containers visible from both sides). All tests passed cleanly there.
- **Files modified:** None (test-execution environment only).
- **Commit:** N/A (no code change).

**2. [Rule 3 - Blocking issue] `TestDotEnvIsNotTracked` fails under WSL2 due to Windows-style `gitdir:` path in the worktree's `.git` file**
- **Found during:** Task 2's `make test` run (WSL2)
- **Issue:** This worktree's `.git` file contains `gitdir: C:/CodeProjects/drop-tracker/.git/worktrees/agent-ac51e1f3c76f08b01` (a Windows-style absolute path, written by Windows git). WSL2's git cannot parse that as an absolute path and mis-resolves it, so `git ls-files .env` (called by the pre-existing, unrelated `internal/config.TestDotEnvIsNotTracked`) failed with `not a git repository`. Confirmed environmental/pre-existing by reproducing the identical failure on plain `git ls-files .env` run directly, unrelated to any file this plan touches.
- **Fix:** Exported `GIT_DIR`/`GIT_WORK_TREE` (pointing at the WSL-mounted equivalents of the same paths) as environment variables for the `make test` invocation only. This is a read-only, non-destructive, test-invocation-scoped override — no files were modified, and the test's own `git ls-files` subprocess inherits the corrected environment and resolves correctly.
- **Files modified:** None.
- **Commit:** N/A (no code change; environment variable only, not committed).

None of the plan's own files needed a Rule 1/2/4 fix — the implementation matched the plan's `<action>` and `<behavior>` specs on the first pass.

### Auth Gates

None encountered.

## Self-Check: PASSED

- FOUND: internal/db/pool.go
- FOUND: internal/db/pool_timeout_test.go
- FOUND: cmd/server/main.go
- FOUND: internal/testutil/postgres.go
- FOUND: internal/httpserver/boot_e2e_test.go
- FOUND commit 36a4174 (feat: size pool MaxConns against the poll-worker ceiling)
- FOUND commit 7cb4ad1 (test: assert the boot-chain pool ceiling covers the worker ceiling)
