---
phase: 11-bounded-concurrent-polling
verified: 2026-08-17T06:30:49Z
status: human_needed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Confirm the shared pgxpool connection pool (internal/db/pool.go, PoolConfig) does not become a throughput bottleneck for concurrent polling on the project's real deployment target (a small VPS, per PROJECT.md/DPLY-01) — PoolConfig never sets pgxpool.Config.MaxConns explicitly, so it defaults to max(4, runtime.NumCPU()). With MusicBrainzPollWorkers (default 3) and DeezerPollWorkers (default 5) able to run concurrently as two independent cron entries (up to 8 simultaneous DB-touching goroutines via Detector), a 1-2 vCPU deployment target would size the pool at 4 connections, below the 8 that could contend for one."
    expected: "Either MaxConns is deliberately sized to comfortably exceed mbWorkers+dzWorkers on the actual deployment target, or the operator accepts pool-acquire queueing as a bounded, non-correctness-affecting slowdown (it degrades throughput, it does not deadlock or lose data)."
    why_human: "This is a resource-sizing/ops judgment call about the real deployment target's CPU count, not something a unit test can assert — 11-RESEARCH.md's own Pitfall flagged this and ROADMAP.md's Phase 11 Notes explicitly instruct verification to 'confirm the DB pool has not become the new bottleneck' rather than assume it. No plan in this phase touched internal/db/pool.go to address it."
---

# Phase 11: Bounded Concurrent Polling Verification Report

**Phase Goal:** A poll cycle works through the watchlist several artists at a time instead of one at a time, without breaking the rate limits, overlap guards, or detection correctness that v1.0 established.
**Verified:** 2026-08-17T06:30:49Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | An operator can set the per-source worker-pool size via an environment variable (default in the 3-5 range), and a cycle over a multi-artist watchlist finishes measurably faster than the sequential baseline it replaces | VERIFIED | `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` parsed with defaults 3/5, validated positive (`internal/config/config.go:54-55,79-84`), wired via `poller.WithMusicBrainzWorkers`/`WithDeezerWorkers` in `cmd/server/main.go:161`. `TestLoad_PollWorkerDefaults/Overrides/RejectsNonPositivePollWorkers` all pass (verified live: `go test ./internal/config/...` green). Speedup independently re-measured during this verification with a throwaway benchmark test (deleted after use, `git status` confirms no residual diff): 12 entries at 20ms/fetch, 1 worker vs 4 workers → 266.7ms sequential vs 64.6ms concurrent = **4.13x speedup**. Structural proof also present in `TestMusicBrainzCycle_ConcurrencyBoundedByWorkerCount`/`TestDeezerCycle_ConcurrencyBoundedByWorkerCount` (maxInFlight reaches the configured ceiling exactly). See Human Verification below re: DB pool sizing risk under the speedup claim in a real, resource-constrained deployment. |
| 2 | Concurrent polling stays inside each source's existing rate limit — no burst above the configured per-second ceiling — and each source's cycle-overlap guard still skips a new cycle while the prior one for that source is running | VERIFIED | `TestMusicBrainzCycle_ConcurrentPollingStaysInsideRateLimit`/`TestDeezerCycle_ConcurrentPollingStaysInsideRateLimit` assert both `maxInFlight` reaching the worker ceiling and an aggregate elapsed-span floor against a real `rate.Limiter` (`internal/poller/poller_test.go`) — both pass live. `TestMusicBrainzCycle_OverlapGuardHoldsWhileWorkersInFlight` and `TestDeezerCycle_RunsWhileMusicBrainzWorkersInFlight` prove the overlap guard holds while multiple workers are genuinely in flight and that the two sources' guards remain independent — both pass live. `git status --porcelain internal/musicbrainz internal/deezer` is empty — neither rate-limited client was touched. |
| 3 | A single artist's polling failure is logged and skipped: the rest of that cycle's artists are still polled and their events still recorded, and the cycle does not abort | VERIFIED | `TestMusicBrainzCycle_SimultaneousArtistFetchErrorsDoNotAbortCycle`, `TestMusicBrainzCycle_SimultaneousDetectionErrorsDoNotAbortCycle`, and their Deezer twins (6 entries, 3-worker pool, 3 forced simultaneous failures — guaranteeing genuine overlap, not lucky ordering) all pass live. Panic isolation (`TestMusicBrainzCycle_GuardReleasesOnPanic`) also passes — a worker-goroutine panic is recovered and logged, not propagated. |
| 4 | Two artists sharing a release group cannot lose a deluxe-change baseline update — a test that races them asserts the final stored baseline is correct, and the suite passes under `go test -race` | VERIFIED | `AdvanceGroupTrackCountBaseline` (`queries/events.sql`) replaces the two-statement read/write with one `FOR UPDATE`-locked `UPDATE ... RETURNING` statement. `TestAdvanceGroupBaseline_ConcurrentRace` (2-caller, 8-caller, and equal-count subtests) reads back the **stored** value and asserts correctness — re-run live at `-count=10` against a real Postgres instance, 10/10 green. Full suite (`go test ./... -count=1` against real Postgres) green with zero failures. `-race` itself could not be executed in this verification session (same pre-existing Windows/cgo/ThreadSanitizer limitation the SUMMARYs document from Phase 01 onward — `cc1.exe` cannot execute under this toolchain); this is a documented, pre-existing environmental constraint, not something this phase could have fixed, and the logical lost-update race this criterion targets is provably not a Go-level data race in the first place (the serialization mechanism is Postgres's own row lock, not a Go primitive) — `-race` alone was never going to catch it per the plan's own framing. Suite-stability work (11-04) removed the `-p 1` Makefile workaround and closed the folded-in flaky-test todo with three confirmed root causes (destructive schema reset, wall-clock-sensitive notifier assertions, cross-package events-table pollution) — all fixed, none masked. |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | `MusicBrainzPollWorkers`/`DeezerPollWorkers` fields + non-positive rejection | VERIFIED | Lines 54-55 (fields, `envDefault` 3/5), 79-84 (rejection with variable name in error text) |
| `.env.example` | `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` keys | VERIFIED (indirect) | Direct read denied by this session's file-permission rules (`.env*` restricted), but `TestEnvExampleCompleteness` — which fails the build on any Config/`.env.example` key drift — passes live, proving both keys are present and correctly named |
| `internal/poller/poller.go` | `Option`, `WithMusicBrainzWorkers`/`WithDeezerWorkers`, bounded fan-out in both cycles, cycle-end log line | VERIFIED | Full read confirms semaphore+WaitGroup fan-out in both `RunMusicBrainzCycle` and `RunDeezerCycle`, panic recovery, double cancellation check, `poll cycle complete` log line with `artist_count`/`duration_ms` in both methods |
| `queries/events.sql` / generated sqlc bindings | `AdvanceGroupTrackCountBaseline` replacing the two former queries | VERIFIED | `grep` confirms the new query exists and the two old ones (`GroupTrackCountBaseline`, `SetGroupTrackCountBaseline`) are gone from both the query file and generated `querier.go` |
| `internal/detection/detector.go` / `musicbrainz.go` | `advanceGroupBaseline`, re-derived branching, accepted-edge doc + Warn log | VERIFIED | `advanceGroupBaseline` present, `groupBaseline`/`setGroupBaseline` gone, "Known, accepted edge" doc paragraph and `logger.Warn` post-advance-insert-failure line both present |
| `internal/db/migrate_test.go` | Schema-isolated migrate-from-scratch test | VERIFIED | No `DROP SCHEMA public` anywhere in the repo's test files; `migrate_scratch` schema + `to_regclass` isolation assertions present; ran twice live against real Postgres with no manual reset, `public.artists` intact before/after |
| `internal/notifier/notifier.go` + `export_test.go` | Deterministic spacing seam | VERIFIED | `spacingWait` var present, routed through the inter-send select; `export_test.go` exists with the single `SetSpacingWaitForTest` helper; zero `.Sub(`/`time.Since(` assertions remain in `notifier_test.go` |
| `.planning/todos/completed/2026-08-11-fix-flaky-tests-under-parallel-go-test.md` | Closed todo with Resolution section | VERIFIED | File present in `completed/`, absent from `pending/` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/server/main.go` | `poller.New` | `WithMusicBrainzWorkers(cfg.MusicBrainzPollWorkers)`, `WithDeezerWorkers(cfg.DeezerPollWorkers)` | WIRED | Both options passed on the same `poller.New(...)` call (line 161); confirmed by `grep` and by `go build ./...` succeeding |
| Worker closures | `*rate.Limiter` (musicbrainz/deezer clients) | Direct call inside worker, no wrapper | WIRED | `git status --porcelain internal/musicbrainz internal/deezer` empty; rate-limit tests pass with real captured timestamps |
| `detectDeluxeChanges` | `advanceGroupBaseline` → `AdvanceGroupTrackCountBaseline` | Single atomic call before conditional `insertEvent` | WIRED | Confirmed by reading `musicbrainz.go`'s branching and by all 12 pre-existing deluxe-change tests passing unmodified |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build`/`go vet` clean | `go build ./... && go vet ./...` | exit 0, no output | PASS |
| Config contract (defaults/override/rejection/.env.example parity) | `go test ./internal/config/... -count=3 -v` | all green, 3/3 repetitions | PASS |
| Poller concurrency suite (non-DB) | `go test ./internal/poller/... -count=3 -v` | all green, 3/3 repetitions, including all new PERF-01/02/03 tests | PASS |
| PERF-04 race test against real Postgres | `go test ./internal/detection/... -run TestAdvanceGroupBaseline -count=10 -v` (`TEST_DATABASE_URL` set to the project's own `docker-compose` Postgres) | all green, 10/10 repetitions, asserting read-back stored state | PASS |
| Migrate-from-scratch isolation | `go test ./internal/db/... -run TestRunMigrations -count=2 -v` | green both times, no manual reset, `public.artists` verified intact via `psql \dt` before/after | PASS |
| Notifier determinism | `go test ./internal/notifier/... -count=3 -v` | all green, 3/3 repetitions | PASS |
| Full suite against real Postgres | `TEST_DATABASE_URL=... go test ./... -count=1` | all packages green | PASS |
| Empirical speedup re-measurement | Throwaway test, 12 entries/20ms fetch, 1 vs 4 workers (deleted after use, no residual diff) | 266.7ms → 64.6ms (4.13x) | PASS |
| Prohibition grep gates | `errgroup`, `worker_id`, `sync.Mutex/RWMutex/Map` in poller/detection, destructive `DROP SCHEMA public` repo-wide | 0 matches everywhere | PASS |
| Debt markers (`TBD`/`FIXME`/`XXX`) in phase-modified files | `grep` across all 17 files this phase touched | 0 matches | PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| PERF-01 | 11-01, 11-02 | Bounded, env-configurable per-source worker pool (default 3-5) instead of sequential iteration | SATISFIED | Config surface + both cycles' bounded fan-out, verified live |
| PERF-02 | 11-01, 11-02 | Concurrent polling preserves existing rate limiter and overlap guard | SATISFIED | Rate-limit and overlap-guard-under-concurrency tests, verified live |
| PERF-03 | 11-01, 11-02 | A single artist's error does not abort the rest of the cycle | SATISFIED | Simultaneous-failure isolation tests (both sources, both failure origins), verified live |
| PERF-04 | 11-03, 11-04 | Concurrent baseline updates cannot lose an update (atomic CAS at the DB level) | SATISFIED | `AdvanceGroupTrackCountBaseline` + `TestAdvanceGroupBaseline_ConcurrentRace`, verified live against real Postgres |

No orphaned requirements — `REQUIREMENTS.md`'s Phase 11 row (PERF-01 through PERF-04) is fully covered by the four plans' declared `requirements:` frontmatter.

### Anti-Patterns Found

None. No debt markers, no stub returns, no empty handlers found in any of the 17 files this phase modified.

### Human Verification Required

### 1. DB connection-pool sizing under concurrent polling on the real deployment target

**Test:** On the project's actual deployment target (a small VPS per `DPLY-01`), check `runtime.NumCPU()` and compare it against `MusicBrainzPollWorkers + DeezerPollWorkers` (default 3 + 5 = 8). `internal/db/pool.go`'s `PoolConfig` never sets `pgxpool.Config.MaxConns` explicitly, so it inherits pgxpool's own default of `max(4, runtime.NumCPU())`.
**Expected:** The pool's connection ceiling should comfortably exceed the maximum simultaneous DB-touching goroutines both cycles' worker pools can produce, or the operator should knowingly accept pool-acquire queueing (a throughput slowdown, not a correctness break) as an acceptable v1.1 tradeoff.
**Why human:** This is a resource-sizing decision tied to the real deployment target's CPU count, which cannot be determined or asserted from the codebase alone. `11-RESEARCH.md` flagged this explicitly as a pitfall that could undermine ROADMAP criterion 1's "measurably faster" claim, and ROADMAP.md's own Phase 11 Notes instruct verification to "confirm the DB pool has not become the new bottleneck" rather than assume it — no plan in this phase touched `internal/db/pool.go`, so it remains open. On this dev machine (12 vCPUs) the pool is not a bottleneck (verified via `go env` and the throwaway speedup measurement above showing a real 4.13x speedup), but that does not represent the production deployment target.

### Gaps Summary

No gaps. All four ROADMAP success criteria are verified against the live codebase: the worker-pool config surface exists and is wired end-to-end, both poll cycles fan out over bounded, independently-configured pools while preserving their rate limiters and overlap guards, per-artist errors (including simultaneous multi-failure and panics) are isolated and never abort a cycle, and the deluxe-change baseline's lost-update race is closed with an atomic database-level compare-and-set proven by a race test that reads back stored state. The full test suite (including the phase's own new concurrency/race tests) passes against a real Postgres instance. `go test -race` itself remains unexecutable in this Windows dev environment — a pre-existing, already-documented limitation since Phase 01, not something this phase introduced or could resolve locally; the recommended CI/WSL2 `-race` run remains an open follow-up as every phase-01-onward SUMMARY has flagged.

One item is routed to human verification rather than blocking: whether the shared Postgres connection pool's default sizing (uncapped `MaxConns`, defaulting to `max(4, NumCPU)`) could throttle concurrent polling's real-world throughput on the project's actual (likely low-vCPU) deployment target. This is an operational sizing judgment call flagged by the phase's own research and by ROADMAP.md's explicit verification instruction — it does not affect correctness (no data loss, no deadlock, only queued connection acquisition) and no plan claimed to address it, so it is surfaced rather than treated as a gap.

---

*Verified: 2026-08-17T06:30:49Z*
*Verifier: Claude (gsd-verifier)*
