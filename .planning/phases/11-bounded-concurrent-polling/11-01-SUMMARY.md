---
phase: 11-bounded-concurrent-polling
plan: 01
subsystem: infra
tags: [go, concurrency, worker-pool, config, poller, musicbrainz]

# Dependency graph
requires:
  - phase: 03-external-clients-search
    provides: robfig/cron-scheduled RunMusicBrainzCycle/RunDeezerCycle with independent per-source overlap guards and rate limiters
provides:
  - "config.Config.MusicBrainzPollWorkers / DeezerPollWorkers (env-configurable, validated positive)"
  - "poller.Option functional-option type + poller.WithMusicBrainzWorkers"
  - "Bounded buffered-channel semaphore fan-out inside RunMusicBrainzCycle, replacing the sequential per-artist loop"
  - "poll cycle complete structured log line (artist_count, duration_ms)"
  - "Panic recovery inside the worker goroutine, preventing one artist's panic from crashing the process"
affects: [11-02-deezer-bounded-fanout, 11-03, 11-04]

# Actuals (#2632)
actuals:
  tokens: 8916
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Buffered-channel semaphore + sync.WaitGroup created fresh per cycle invocation for bounded concurrent fan-out (no new dependency, no persistent pool lifecycle)"
    - "Functional-option constructor (poller.Option) mirroring internal/db's existing RetryOption pattern"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/poller/poller.go
    - internal/poller/poller_test.go
    - cmd/server/main.go

key-decisions:
  - "Cancellation is checked in two places, not one: the dispatch loop's own select (for when the semaphore is genuinely full) and inside each worker before its fetch call (for when worker count >= entry count, where the dispatch loop never blocks and so never observes a mid-cycle cancellation on its own)."
  - "Worker goroutines recover their own panics and log them, since a panic inside a spawned goroutine cannot be caught by any caller's defer/recover -- left unrecovered it would crash the whole process instead of costing only that one artist's result."
  - "Pre-existing tests that implicitly assumed sequential-only dispatch (fixed per-artist call order, or a literal in-flight/call count of 1 while a cycle is mid-flight) were corrected to concurrency-safe assertions (set-membership order checks; store.listCalls-based proof that a skipped cycle performs zero work) rather than left flaky."

requirements-completed: []  # PERF-01/02/03 span all 4 plans in this phase (MusicBrainz + Deezer); this plan proves the pattern on MusicBrainz only. Left unmarked in REQUIREMENTS.md pending Deezer's own bounded fan-out (11-02+).

coverage:
  - id: D1
    description: "MUSICBRAINZ_POLL_WORKERS / DEEZER_POLL_WORKERS parsed with defaults 3/5, validated positive, documented in .env.example"
    requirement: "PERF-01"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestLoad_PollWorkerDefaults"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestLoad_PollWorkerOverrides"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestLoad_RejectsNonPositivePollWorkers"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestEnvExampleCompleteness"
        status: pass
    human_judgment: false
  - id: D2
    description: "RunMusicBrainzCycle fans out over a bounded worker pool, reaching but never exceeding the configured ceiling"
    requirement: "PERF-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_ConcurrencyBoundedByWorkerCount"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_WorkerCountOneIsSequential"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_WorkerCountAboveEntryCountFansOutToEntryCount"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_SingleEntryReachesOneInFlight"
        status: pass
    human_judgment: false
  - id: D3
    description: "A per-artist fetch/detection error is logged inside its own worker and never reaches the caller or any sibling worker"
    requirement: "PERF-03"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_PerArtistErrorContinuesCycle"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestPoller_RunMusicBrainzCycle_DetectionErrorIsolatedPerArtist"
        status: pass
    human_judgment: false
  - id: D4
    description: "A cancelled context stops dispatch, joins in-flight workers, and returns the context error with no leaked goroutine"
    requirement: "PERF-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_ContextCancelledStopsIteration"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every cycle emits one poll cycle complete line carrying artist_count and truncated integer duration_ms"
    requirement: "PERF-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_LogsCycleDurationAndArtistCount"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_EmptyWatchlistNoCallsNilError"
        status: pass
    human_judgment: false
  - id: D6
    description: "-race-clean concurrent poller test suite (PERF-01 verification requirement) and the full project test suite still compile/pass with the widened poller.New signature"
    verification: []
    human_judgment: true
    rationale: "go test -race is unavailable on this Windows dev machine (ThreadSanitizer allocation failure via cgo, the same pre-existing environmental limitation documented in Phase 01's decisions). Verified instead via go test ./internal/poller/... ./internal/config/... -count=5 (25 total repetitions across the two changed packages, zero failures) and a full go test ./... pass. DB-backed integration tests (TestPoller_RunMusicBrainzCycle_RecordsNewRelease etc.) auto-skip without TEST_DATABASE_URL -- not run in this session because port 5433 was occupied by a concurrently-running sibling worktree's own Postgres container, and connecting to it risked corrupting that agent's in-progress test state. A CI run with -race and a real, isolated Postgres instance is the recommended follow-up verification."

duration: 100min
completed: 2026-08-17
status: complete
---

# Phase 11 Plan 01: Bounded MusicBrainz Poll-Cycle Fan-Out (Tracer) Summary

**Bounded worker-pool concurrency for `RunMusicBrainzCycle` via a per-cycle buffered-channel semaphore, driven by new `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` env vars (defaults 3/5), with per-artist panic isolation and a `poll cycle complete` observability log line.**

## Performance

- **Duration:** ~100 min
- **Started:** 2026-08-17T03:50:00Z (approx.)
- **Completed:** 2026-08-17T05:31:06Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- End-to-end tracer slice: `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` env vars → `config.Load` validation → `cmd/server/main.go` wiring → `poller.Option`/`WithMusicBrainzWorkers` → bounded `RunMusicBrainzCycle` fan-out → `poll cycle complete` log line, all committed and passing under repeated (non-`-race`) test runs
- `RunMusicBrainzCycle`'s sequential per-artist loop replaced with a buffered-channel semaphore (`make(chan struct{}, p.mbWorkers)`) + `sync.WaitGroup`, created fresh per cycle invocation, mirroring `internal/httpserver/search.go`'s existing fan-out house style
- Full config contract test coverage: defaults (3/5), explicit overrides, non-positive rejection (0 and -1 for both vars), and the `MUSICBRAINZ_POLL_WORKERS=1` boundary
- Complete poller_test.go migration off the old hard-coded "artists must be polled one at a time" guarantee: `TestMusicBrainzCycle_Sequential` deleted, four new tests pin the boundary/empty/single-entry/log-precision edges PERF-01/02 require, and every other pre-existing test whose assertions implicitly depended on sequential dispatch was corrected to a concurrency-safe assertion

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "operator sets a worker count and the MusicBrainz cycle honours it" — one path only** - `34a7b66` (feat)
2. **Task 2: Config contract tests for both worker-count variables** - `af1f703` (test)
3. **Task 3: Replace the stale sequential-only guarantee and pin the concurrency edge cases** - `0978411` (test)

_Note: this plan carried `tdd="true"` on Tasks 2 and 3; both landed as single test-authoring commits (RED+GREEN combined) since the behavior under test — config validation, and the already-implemented Task 1 concurrency primitive — existed at commit time and the new tests were written directly against it, not iterated through a separate failing-then-passing cycle._

## Files Created/Modified
- `internal/config/config.go` - Adds `MusicBrainzPollWorkers`/`DeezerPollWorkers` fields (env-tagged, defaults 3/5) and their non-positive rejection in `Load`
- `internal/config/config_test.go` - `TestLoad_PollWorkerDefaults`, `TestLoad_PollWorkerOverrides`, `TestLoad_RejectsNonPositivePollWorkers`, `TestLoad_PollWorkersOneIsValid`
- `internal/poller/poller.go` - `Option` type, `WithMusicBrainzWorkers`, `defaultMusicBrainzPollWorkers`, `Poller.mbWorkers`, bounded fan-out + panic recovery inside `RunMusicBrainzCycle`, `poll cycle complete` log line
- `internal/poller/poller_test.go` - New concurrency-bound/edge-case tests, deleted `TestMusicBrainzCycle_Sequential`, corrected several pre-existing tests' sequential-order/count assumptions
- `cmd/server/main.go` - Threads `cfg.MusicBrainzPollWorkers` into `poller.New` via `WithMusicBrainzWorkers`

## Decisions Made
- Cancellation observability needed two check points (dispatch-loop `select` + an in-worker `ctx.Err()` check before fetching), because with the default pool size (3) matching a 3-entry test watchlist, the dispatch loop never actually blocks on the semaphore and so can never observe a cancellation racing ahead of it on its own — proven empirically non-deterministic under this machine's default `GOMAXPROCS` before the fix (traced with `GOMAXPROCS=1`, which made it 100% deterministic, confirming the root cause).
- Worker goroutines recover their own panics (log-and-continue), because Go panics cannot cross goroutine boundaries — an unrecovered panic in a spawned worker would crash the entire process, not just that one artist's result, which is a strictly worse outcome than the sequential loop's original behavior for this specific failure mode.
- `TestMusicBrainzCycle_ContextCancelledStopsIteration` was changed to explicitly construct the poller with `WithMusicBrainzWorkers(1)` rather than relying on the default 3-worker pool over 3 entries, so the semaphore genuinely blocks and cancellation is observed via a real happens-before edge (cancel() always precedes the deferred semaphore release inside the same worker) instead of goroutine-scheduling luck.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `TestMusicBrainzCycle_ContextCancelledStopsIteration` was non-deterministic under the concurrent design as originally structured**
- **Found during:** Task 1, verifying the plan's own `<verify>` block
- **Issue:** With the default 3-worker pool matching `threeEntries()`'s 3 entries, the dispatch loop's `select` on the semaphore never actually blocks, so it can never observe a cancellation triggered from inside an already-dispatched worker. Empirically confirmed non-deterministic (~50% failure rate over 50 repeated runs under this machine's default `GOMAXPROCS`; 100% pass under `GOMAXPROCS=1`, isolating the exact cause).
- **Fix:** Added a `ctx.Err()` check inside each worker before its fetch call (so a worker whose slot was acquired before cancellation still skips a redundant fetch), a post-join `ctx.Err()` fallback so the cycle still returns the context error even when the dispatch loop's own select never observed it, and reconstructed the specific test with `WithMusicBrainzWorkers(1)` so the semaphore genuinely blocks and the ordering is deterministic via a real happens-before edge.
- **Files modified:** `internal/poller/poller.go`, `internal/poller/poller_test.go`
- **Verification:** 100/100 repeated runs pass; `go build`/`go vet` clean.
- **Committed in:** `34a7b66` (Task 1 commit)

**2. [Rule 1/2 - Bug / Missing critical] Unrecovered panic inside a worker goroutine crashes the whole process**
- **Found during:** Task 1, running the full pre-existing poller test suite as a regression check
- **Issue:** `TestMusicBrainzCycle_GuardReleasesOnPanic` wraps its call to `RunMusicBrainzCycle` in its own `defer recover()`, which worked under the old sequential design (same goroutine) but cannot catch a panic raised inside a spawned worker goroutine (Go panics never cross goroutine boundaries) — the whole `go test` binary crashed, aborting every remaining test in the package.
- **Fix:** Added a `defer func() { if r := recover(); r != nil { ... } }()` inside the worker closure, logging the panic value with the same artist context fields other errors use, then returning normally instead of crashing the process.
- **Files modified:** `internal/poller/poller.go`
- **Verification:** `TestMusicBrainzCycle_GuardReleasesOnPanic` passes; full poller suite no longer crashes.
- **Committed in:** `34a7b66` (Task 1 commit)

**3. [Rule 1 - Bug] Several pre-existing poller tests implicitly assumed sequential-only dispatch, beyond the one test the plan explicitly named**
- **Found during:** Task 3, following the plan's own instruction to "grep the whole test file for any other assertion or comment asserting one-at-a-time / sequential-only polling"
- **Issue:** `TestMusicBrainzCycle_CallsSourceOncePerEntry` and `TestPoller_RunMusicBrainzCycle_DetectionErrorIsolatedPerArtist` asserted a fixed per-artist call order (`[mbid-1, mbid-2, mbid-3]`), which concurrent dispatch cannot guarantee. `TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning` and `TestMusicBrainzCycle_OverlapGuard_SkipsWhileInFlight` asserted a literal in-flight call count of 1 while a cycle was blocked mid-flight — with the default 3-worker pool, all 3 entries can be concurrently blocked by the time the test's synchronization signal fires, so the literal-1 assertion failed non-deterministically (confirmed via 10 repeated runs).
- **Fix:** Order-dependent assertions were changed to set-membership checks (`sort.Strings` + compare). The two "skipped tick performs zero calls" tests were changed to assert `store.listCalls == 1` instead of a source/detection call-count delta — `store.List` is the first thing the cycle body does after the CAS guard succeeds, so it stays a deterministic, non-racy proof that the second (skipped) call performed zero work of any kind, without depending on the first cycle's own still-in-flight concurrent workers.
- **Files modified:** `internal/poller/poller_test.go`
- **Verification:** `go test ./internal/poller/... -count=5` (25 repetitions), zero failures.
- **Committed in:** `0978411` (Task 3 commit)

---

**Total deviations:** 3 auto-fixed (2 Rule 1 bugs surfaced by the concurrency refactor itself, 1 Rule 1/2 missing-panic-safety fix)
**Impact on plan:** All three were necessary for correctness and test-suite determinism under the new concurrent design; none expand scope beyond what Task 1/3 already required (a working, `-race`-worthy concurrent implementation and a test suite free of stale sequential assumptions).

## Issues Encountered
- `go test -race` is unavailable on this Windows dev machine (ThreadSanitizer fails to allocate memory via the cgo toolchain) — the same pre-existing environmental limitation documented in Phase 01's decisions (mingw64 gcc `cc1.exe` cannot execute). Substituted with repeated non-`-race` runs (`-count=5`, 25 total repetitions across `internal/poller` and `internal/config`, zero failures) as the best available substitute in this environment; flagged as a deferred human-judgment item (D6) for a CI run with `-race` and a real Postgres instance.
- The default `TEST_DATABASE_URL` (`localhost:5433`) was occupied by a concurrently-running sibling worktree agent's own Postgres container during this session. Rather than risk corrupting that agent's in-progress test state, DB-backed integration tests were left in their existing auto-skip state (already verified to skip cleanly without `TEST_DATABASE_URL`) instead of forcing a connection.

## User Setup Required

None - no external service configuration required. `.env.example` already carried both new keys (`MUSICBRAINZ_POLL_WORKERS=3`, `DEEZER_POLL_WORKERS=5`) from a prep commit ahead of this plan's dispatch.

## Next Phase Readiness
- The bounded-fan-out pattern (semaphore + WaitGroup, functional-option pool-size configuration, panic-safe worker closures) is proven end-to-end on MusicBrainz and ready to be mirrored onto `RunDeezerCycle` in the phase's next plan(s).
- `PERF-01`/`PERF-02`/`PERF-03` remain unmarked in REQUIREMENTS.md — both source's requirement text is explicit ("MusicBrainz, Deezer"), and this plan only delivers the MusicBrainz half. Mark complete once Deezer's own bounded fan-out lands.
- `PERF-04` (atomic deluxe-change baseline compare-and-set) is untouched by this plan — out of scope for 11-01, tracked separately per the phase's own requirement list.
- No blockers for the next plan in this phase.

---
*Phase: 11-bounded-concurrent-polling*
*Completed: 2026-08-17*
