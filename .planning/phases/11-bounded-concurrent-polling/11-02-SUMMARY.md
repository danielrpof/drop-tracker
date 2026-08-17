---
phase: 11-bounded-concurrent-polling
plan: 02
subsystem: infra
tags: [go, concurrency, worker-pool, poller, deezer, rate-limiting]

# Dependency graph
requires:
  - phase: 11-bounded-concurrent-polling
    provides: "plan 01's bounded fan-out pattern for RunMusicBrainzCycle (semaphore + WaitGroup, functional-option pool-size configuration, panic-safe worker closures) and MusicBrainzPollWorkers/DeezerPollWorkers config surface"
provides:
  - "poller.WithDeezerWorkers Option, defaultDeezerPollWorkers=5, Poller.dzWorkers field"
  - "Bounded buffered-channel semaphore fan-out inside RunDeezerCycle, mirroring RunMusicBrainzCycle's pattern exactly, with the nil-DeezerID skip kept outside the concurrent path (D-06)"
  - "poll cycle complete structured log line for RunDeezerCycle (artist_count, duration_ms)"
  - "cmd/server/main.go wires poller.WithDeezerWorkers(cfg.DeezerPollWorkers)"
  - "Empirical proof (real captured timestamps) that concurrent workers sharing one rate.Limiter stay inside the configured per-source rate (PERF-02)"
  - "Empirical proof the per-source cycle-overlap guard holds while multiple workers are genuinely in flight simultaneously, and that the two sources' guards stay independent under concurrent load (D-08)"
  - "Empirical proof that simultaneous per-artist failures (fetch-origin and detection-origin) do not abort a concurrent cycle, for both sources (PERF-03)"
  - "D-07's interleaved-but-labelled log contract pinned by a test that deliberately does not assert emission order"
affects: [11-03, 11-04]

# Actuals (#2632)
actuals:
  tokens: 11265
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Test-local source doubles wrapping a real golang.org/x/time/rate.Limiter (rateLimitedReleaseGroupSource/rateLimitedAlbumSource), tracking inFlight from function entry (before limiter.Wait blocks) so maxInFlight proves the worker pool actually dispatched concurrently, while an aggregate first-to-last timestamp span proves the limiter still serialises callers -- neither assertion alone would be conclusive"
    - "Fixed, deterministic failing-entry sets (map[string]bool literals) in concurrent-failure tests, never a runtime counter shared across worker goroutines -- a counter-based 'fail only the first N' scheme is itself an unsynchronized data race"

key-files:
  created: []
  modified:
    - internal/poller/poller.go
    - internal/poller/poller_test.go
    - cmd/server/main.go

key-decisions:
  - "Two pre-existing Deezer tests written for the old sequential RunDeezerCycle did not survive the move to concurrent dispatch and were fixed using the exact precedent plan 01 established for the equivalent MusicBrainz tests: TestDeezerCycle_SkipsNilDeezerID's fixed-order artist-id assertion (confirmed flaky at 165/200 runs) became a set-membership check, and TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning's events.deezerCalls snapshot (racing the first cycle's own still-in-flight workers) became a store.listCalls-based proof, exactly as plan 01 did for TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning."
  - "TestDeezerCycle_ContextCancelledStopsIteration required the same WithDeezerWorkers(1) reconstruction plan 01 applied to its MusicBrainz counterpart: with the default pool (5) >= the 2 Deezer-capable entries in threeEntries(), the dispatch loop's semaphore send never blocks, making cancellation observation a goroutine-scheduling race (confirmed flaky at 3/100 runs) rather than a deterministic happens-before edge. Forcing genuine semaphore contention (pool size 1) makes the ordering deterministic."
  - "The rate-limited test doubles track inFlight from the start of the call, before limiter.Wait() blocks -- not after, as a first draft (following the plan's literal action text) did. Tracking after Wait() made maxInFlight read 1 on every run, because the limiter's own serialisation meant no two callers were ever concurrently past the Wait() gate with fast, non-sleeping post-limiter work. Tracking from entry instead proves the worker pool actually dispatched N callers concurrently (several genuinely blocked inside Wait() at once), which combined with the elapsed-span assertion is what the plan's PERF-02 proof actually requires."

requirements-completed: [PERF-01, PERF-02, PERF-03]  # PERF-04 (baseline CAS) is out of scope for this plan -- tracked separately

coverage:
  - id: D1
    description: "RunDeezerCycle fans out over a bounded worker pool sized by WithDeezerWorkers/DeezerPollWorkers, reaching but never exceeding the configured ceiling, mirroring RunMusicBrainzCycle's pattern"
    requirement: "PERF-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_ConcurrencyBoundedByWorkerCount"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_WorkerCountEqualToEntryCountRunsAllConcurrently"
        status: pass
    human_judgment: false
  - id: D2
    description: "A nil-DeezerID entry is skipped without consuming a worker slot, spawning a goroutine, issuing an HTTP request, or calling the recorder (D-06)"
    requirement: "PERF-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_NilDeezerIDConsumesNoWorkerSlot"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every completed Deezer cycle emits one poll cycle complete line carrying artist_count and truncated integer duration_ms, and cmd/server/main.go wires WithDeezerWorkers from config"
    requirement: "PERF-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_LogsCycleDurationAndArtistCount"
        status: pass
    human_judgment: false
  - id: D4
    description: "Concurrent workers sharing one rate.Limiter cannot exceed the configured per-source rate, proven with real captured request timestamps rather than trusting the library's documented guarantee alone, for both sources"
    requirement: "PERF-02"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_ConcurrentPollingStaysInsideRateLimit"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_ConcurrentPollingStaysInsideRateLimit"
        status: pass
    human_judgment: false
  - id: D5
    description: "The per-source cycle-overlap guard rejects a second cycle while multiple workers are genuinely in flight simultaneously, and the two sources' guards remain independent under concurrent load (D-08)"
    requirement: "PERF-02"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_OverlapGuardHoldsWhileWorkersInFlight"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_RunsWhileMusicBrainzWorkersInFlight"
        status: pass
    human_judgment: false
  - id: D6
    description: "Multiple artists failing simultaneously in a concurrent cycle (fetch-origin or detection-origin failures) are each logged and skipped; every other artist is still fetched and handed to the EventRecorder, and the cycle returns nil -- for both sources"
    requirement: "PERF-03"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_SimultaneousArtistFetchErrorsDoNotAbortCycle"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_SimultaneousDetectionErrorsDoNotAbortCycle"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_SimultaneousArtistFetchErrorsDoNotAbortCycle"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_SimultaneousDetectionErrorsDoNotAbortCycle"
        status: pass
    human_judgment: false
  - id: D7
    description: "Every poll result / poll artist failed record carries cycle_id, source, artist_mbid, and artist_name under concurrency, all records in one cycle share one cycle_id, and the artist_mbid set matches the polled entries exactly -- emission order is deliberately unasserted (D-07)"
    requirement: "PERF-03"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_ConcurrentLogLinesAreFullyLabelled"
        status: pass
    human_judgment: false
  - id: D8
    description: "-race-clean concurrent poller test suite (PERF-01/02/03 verification requirement) and the full project test suite still compile/pass with the widened RunDeezerCycle implementation"
    verification: []
    human_judgment: true
    rationale: "go test -race is unavailable on this Windows dev machine (ThreadSanitizer fails to allocate memory via the cgo toolchain), the same pre-existing environmental limitation plan 01 documented. Substituted with repeated non-race runs: go test ./internal/poller/... -count=15/-count=20 at multiple points during this plan (zero failures across ~35 total full-suite repetitions) plus targeted -count=100/-count=200 runs used specifically to hunt for and confirm the two concurrency-induced test flakes documented under Deviations. TEST_DATABASE_URL was not set in this session, so DB-backed integration tests ran in their existing auto-skip state (unaffected by this plan's changes, which touch no database code). A CI run with -race and a real, isolated Postgres instance is the recommended follow-up verification, same as plan 01's own open item."

duration: ~60min
completed: 2026-08-17
status: complete
---

# Phase 11 Plan 02: Bounded Deezer Poll-Cycle Fan-Out and Concurrency Proofs Summary

**Deezer's RunDeezerCycle gets the identical bounded worker-pool treatment plan 01 gave MusicBrainz (WithDeezerWorkers, default 5, D-06's nil-DeezerID skip kept outside the concurrent path), plus the phase's PERF-02/PERF-03 empirical proofs: real captured request timestamps showing the shared rate.Limiter still serialises concurrent workers, the overlap guard holding while multiple workers are genuinely in flight, and simultaneous per-artist failures (from both fetch and detection) never aborting a cycle for either source.**

## Performance

- **Duration:** ~60 min
- **Completed:** 2026-08-17T06:01:02Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- `RunDeezerCycle`'s sequential per-artist loop replaced with the same buffered-channel semaphore + `sync.WaitGroup` pattern plan 01 established for `RunMusicBrainzCycle`, including panic-safe worker closures and the double cancellation check (dispatch-loop select + in-worker `ctx.Err()`), with the nil-DeezerID skip kept outside the concurrent path so a skipped entry never occupies a worker slot (D-06)
- `WithDeezerWorkers` option, `defaultDeezerPollWorkers = 5`, `dzWorkers` field, and the `poll cycle complete` log line (`artist_count`, `duration_ms`) added to `RunDeezerCycle`; `cmd/server/main.go` wires `WithDeezerWorkers(cfg.DeezerPollWorkers)` alongside the existing MusicBrainz option
- PERF-02's rate-ceiling claim proven with a test-local `*rate.Limiter`-wrapping source double for both sources: 8 entries through a 5-worker pool against a `rate.NewLimiter(50, 1)` limiter, asserting both `maxInFlight == 5` (proving genuine concurrent dispatch) and an aggregate elapsed span >= 7/50s across real captured timestamps (proving the limiter, not the pool size, is what bounds request rate)
- PERF-02's overlap-guard claim extended from "one worker in flight" to "multiple workers genuinely in flight simultaneously," and D-08's cross-source independence proven under that same concurrent load
- PERF-03's error-isolation claim extended from single-failure to simultaneous-failure: 6 entries over a 3-worker pool with 3 fixed failing entries (guaranteeing overlap) for both fetch-origin and detection-origin failures, on both sources
- D-07's log-labelling contract pinned by a test that decodes every emitted record and asserts full attribution while explicitly declining to assert emission order, with an inline comment naming D-07 so a future contributor does not "fix" the test by adding one

## Task Commits

Each task was committed atomically:

1. **Task 1: Bounded fan-out for the Deezer cycle** - `4799f51` (feat)
2. **Task 2: Prove the rate ceiling and the overlap guard survive concurrency (PERF-02)** - `d7fcb06` (test)
3. **Task 3: Prove per-artist error isolation and D-07's log-labelling contract under concurrency (PERF-03)** - `20d3ed4` (test)

## Files Created/Modified
- `internal/poller/poller.go` - `defaultDeezerPollWorkers`, `WithDeezerWorkers`, `Poller.dzWorkers`, bounded fan-out + panic recovery + `poll cycle complete` log line inside `RunDeezerCycle`; package doc comment updated to reflect both cycles now fan out
- `internal/poller/poller_test.go` - 4 new Task 1 tests, 2 new test-local rate-limited source doubles + 4 new Task 2 tests, 6 new Task 3 tests (`sixEntries`/`failFirstThree`/`recordMBID` helpers), plus 3 pre-existing Deezer tests corrected for concurrency-safe assertions (see Deviations)
- `cmd/server/main.go` - `poller.New(...)` call now passes `poller.WithDeezerWorkers(cfg.DeezerPollWorkers)` as a second trailing option

## Decisions Made
- Applied plan 01's exact precedent to two pre-existing Deezer tests whose implicit sequential-dispatch assumptions the concurrency refactor broke, rather than treating them as new problems requiring new solutions -- see Deviations for the specific fixes and confirmed flake rates.
- Rate-limited test doubles track `inFlight` from function entry, before `limiter.Wait()` blocks, not after -- see key-decisions above for why the after-Wait version was empirically wrong (`maxInFlight` always read 1).
- Used fixed, deterministic `map[string]bool` failing-entry sets in all simultaneous-failure tests rather than any runtime counter, since a counter shared across concurrent worker goroutines without synchronization would itself introduce a data race into the test suite.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `TestDeezerCycle_SkipsNilDeezerID`'s fixed-order artist-id assertion did not survive concurrent dispatch**
- **Found during:** Task 1, running the full pre-existing poller test suite as a regression check after wiring the bounded fan-out
- **Issue:** The test asserted `dz.artistIDs == ["101", "103"]` in strict dispatch order. Under concurrent fan-out (default 5-worker pool over 2 Deezer-capable entries), the two workers can complete in either order. Empirically confirmed severely flaky: 165/200 repeated runs failed.
- **Fix:** Changed to a set-membership assertion (`sort.Strings` + compare), mirroring the identical pattern plan 01 applied to `TestMusicBrainzCycle_CallsSourceOncePerEntry`.
- **Files modified:** `internal/poller/poller_test.go`
- **Verification:** 200/200 repeated runs pass after the fix.
- **Committed in:** `4799f51` (Task 1 commit)

**2. [Rule 1 - Bug] `TestDeezerCycle_ContextCancelledStopsIteration` was non-deterministic under concurrent dispatch**
- **Found during:** Task 1, same regression pass
- **Issue:** With the default 5-worker pool covering only 2 Deezer-capable entries in `threeEntries()`, the dispatch loop's semaphore send never blocks, so it can never reliably observe cancellation from an already-dispatched worker -- the same root cause plan 01 found and fixed for the MusicBrainz equivalent. Empirically confirmed flaky: 3/100 repeated runs failed.
- **Fix:** Reconstructed the test with `WithDeezerWorkers(1)` so the semaphore genuinely blocks and mbid-1's `cancel()`-then-return strictly precedes mbid-3's dispatch via a real happens-before edge, exactly as plan 01 did for `TestMusicBrainzCycle_ContextCancelledStopsIteration`.
- **Files modified:** `internal/poller/poller_test.go`
- **Verification:** 100/100 repeated runs pass after the fix.
- **Committed in:** `4799f51` (Task 1 commit)

**3. [Rule 1 - Bug] `TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning`'s `events.deezerCalls` snapshot raced the first cycle's own in-flight workers**
- **Found during:** Task 1, running the full poller suite with `-count=20` after the Task 1 changes landed
- **Issue:** With the default worker pool, both Deezer-capable entries can be concurrently blocked inside `events.deezerFn` by the time the test's "started" signal fires, and the still-running first cycle's own already-dispatched workers keep incrementing `events.deezerCalls` in the background -- making a snapshot-based comparison inherently racy against the first cycle's own progress rather than a measurement of the second (skipped) call's behavior.
- **Fix:** Changed the assertion to `store.listCalls == 1` -- `store.List` is the first thing the cycle body does after the CAS guard succeeds, so it deterministically proves the second, skipped call performed zero work of any kind. Identical fix shape to plan 01's `TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning`.
- **Files modified:** `internal/poller/poller_test.go`
- **Verification:** `go test ./internal/poller/... -count=20`, zero failures (previously failed reliably on every repeated run).
- **Committed in:** `4799f51` (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (all Rule 1 bugs -- pre-existing tests whose sequential-dispatch assumptions the concurrency refactor broke, fixed using plan 01's own established precedent for the structurally identical MusicBrainz-side issues)
**Impact on plan:** All three were necessary for test-suite determinism under the new concurrent `RunDeezerCycle`; none expand scope beyond what Task 1 already required (a working, concurrency-safe implementation with a deterministic test suite).

## Issues Encountered
- `go test -race` is unavailable on this Windows dev machine (ThreadSanitizer fails to allocate memory via the cgo toolchain) -- the same pre-existing environmental limitation plan 01 documented. Substituted with extensive repeated non-`-race` runs at every verification point in this plan (multiple `-count=15`/`-count=20` full-suite passes, plus targeted `-count=100`/`-count=200` runs used specifically to detect and confirm the two flaky-test deviations above), flagged as a deferred human-judgment item (D8) for a CI run with `-race`.
- `TEST_DATABASE_URL` was not set in this session; DB-backed integration tests ran in their existing auto-skip state. This plan's changes touch no database code, so this is not expected to affect their outcome, but a follow-up CI run against a real Postgres instance is still the recommended verification per plan 01's own open item.
- A first draft of the rate-limited test doubles (Task 2) tracked `inFlight` after `limiter.Wait()` returned, following the plan action text literally -- this made `maxInFlight` read 1 on every run because the limiter's own serialisation meant post-limiter work (an instant mutex-guarded append, no sleep) never overlapped across callers. Caught immediately by running the new tests before committing; fixed by moving the tracking to before `limiter.Wait()`, which correctly proves the worker pool dispatched N callers concurrently (several genuinely blocked inside `Wait()` at once) rather than only ever having one caller inside the method body at a time. Not a plan deviation in the Rule 1-4 sense (no committed code was affected), but recorded here since it affected the design of a canonical PERF-02 proof.

## User Setup Required

None - no external service configuration required. Both `MUSICBRAINZ_POLL_WORKERS` and `DEEZER_POLL_WORKERS` were already present in `.env.example` from plan 01.

## Next Phase Readiness
- Both `RunMusicBrainzCycle` and `RunDeezerCycle` now fan out over independently-configured bounded worker pools, both emit the `poll cycle complete` observability line, and both are wired from configuration in `main.go` -- PERF-01 is now fully satisfied for both sources.
- PERF-02 and PERF-03 are proven with real, empirical concurrency tests (captured timestamps for the rate ceiling, genuine multi-worker blocking for the overlap guard, and simultaneous multi-failure scenarios for error isolation) rather than resting on documented library guarantees alone.
- `PERF-01`, `PERF-02`, and `PERF-03` are marked complete in `REQUIREMENTS.md` by this plan (via the orchestrator's post-wave update) -- their requirement text spans both sources, and this plan completes the Deezer half plan 01 left open.
- `PERF-04` (atomic deluxe-change baseline compare-and-set) remains untouched by this plan, as scoped -- tracked separately in the phase's other plan(s).
- `internal/musicbrainz` and `internal/deezer` remain completely unmodified by this plan (`git status --porcelain` on both is empty) -- concurrency required no change to either rate-limited client, confirming the phase's own design goal.
- No blockers for the remaining plans in this phase.

---
*Phase: 11-bounded-concurrent-polling*
*Completed: 2026-08-17*

## Self-Check: PASSED
- FOUND: `.planning/phases/11-bounded-concurrent-polling/11-02-SUMMARY.md`
- FOUND: `4799f51` (Task 1 commit)
- FOUND: `d7fcb06` (Task 2 commit)
- FOUND: `20d3ed4` (Task 3 commit)
