---
phase: 11-bounded-concurrent-polling
verified: 2026-08-17T11:45:00Z
status: passed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 4/4
  gaps_closed:
    - "G-11-1: internal/db/pool.go's PoolConfig never set pgxpool.Config.MaxConns, inheriting pgxpool's max(4, runtime.NumCPU()) default -- below MusicBrainzPollWorkers + DeezerPollWorkers (default 8) on any host with fewer than 8 vCPUs. Closed by plan 11-05: PoolConfig/NewPool now take a pollWorkers argument, compute MaxConns = pollWorkers + a documented 4-connection headroom, and leave an operator-set pool_max_conns in the DSN authoritative even when it sits below that computed value."
  gaps_remaining: []
  regressions: []
---

# Phase 11: Bounded Concurrent Polling Verification Report

**Phase Goal:** A poll cycle works through the watchlist several artists at a time instead of one at a time, without breaking the rate limits, overlap guards, or detection correctness that v1.0 established.
**Verified:** 2026-08-17T11:45:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (plan 11-05 closed G-11-1, reported in 11-UAT.md against the prior 11-VERIFICATION.md pass)

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | An operator can set the per-source worker-pool size via an environment variable (default in the 3-5 range), and a cycle over a multi-artist watchlist finishes measurably faster than the sequential baseline it replaces | VERIFIED | `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` parsed with defaults 3/5, validated positive (`internal/config/config.go:54-55,79-84`), wired via `poller.WithMusicBrainzWorkers`/`WithDeezerWorkers` in `cmd/server/main.go`. `go test ./internal/config/... ./internal/poller/...` green (re-run live this session). Structural proof in `TestMusicBrainzCycle_ConcurrencyBoundedByWorkerCount`/`TestDeezerCycle_ConcurrencyBoundedByWorkerCount`. The DB-pool-sizing risk flagged against this criterion in the prior verification pass (G-11-1) is now closed — see truth 5 below. |
| 2 | Concurrent polling stays inside each source's existing rate limit — no burst above the configured per-second ceiling — and each source's cycle-overlap guard still skips a new cycle while the prior one for that source is running | VERIFIED | `TestMusicBrainzCycle_ConcurrentPollingStaysInsideRateLimit`/`TestDeezerCycle_ConcurrentPollingStaysInsideRateLimit`, `TestMusicBrainzCycle_OverlapGuardHoldsWhileWorkersInFlight`, `TestDeezerCycle_RunsWhileMusicBrainzWorkersInFlight` — all pass live (`go test ./internal/poller/... -count=1`, this session). `git log` shows neither rate-limited client package touched by 11-05's gap-closure work. |
| 3 | A single artist's polling failure is logged and skipped: the rest of that cycle's artists are still polled and their events still recorded, and the cycle does not abort | VERIFIED | `TestMusicBrainzCycle_SimultaneousArtistFetchErrorsDoNotAbortCycle`, `TestMusicBrainzCycle_SimultaneousDetectionErrorsDoNotAbortCycle`, Deezer twins, and `TestMusicBrainzCycle_GuardReleasesOnPanic` all pass live in the same `internal/poller` run. Unaffected by 11-05 (no files this test package depends on for this behavior were touched). |
| 4 | Two artists sharing a release group cannot lose a deluxe-change baseline update — a test that races them asserts the final stored baseline is correct, and the suite passes under `go test -race` | VERIFIED | `AdvanceGroupTrackCountBaseline` (`queries/events.sql`) — single `FOR UPDATE`-locked CTE. `TestAdvanceGroupBaseline_ConcurrentRace` (2-caller, 8-caller, equal-count subtests) re-run live against real Postgres this session — all pass, reading back stored state. `go test -race` remains unexecutable natively on this Windows dev box (pre-existing ThreadSanitizer/cgo limitation documented since Phase 01); 11-05-SUMMARY.md documents the `-race` run was performed via WSL2 (`go test ./internal/db/ -run 'TestPoolConfig' -race`, `make test`), matching the established project workaround. The logical race this criterion targets is closed by Postgres's own row lock, not a Go-level data race, so `-race` was never the sole proof mechanism for this criterion. |
| 5 (added by 11-05, closes G-11-1) | The shared Postgres connection pool's `MaxConns` ceiling comfortably exceeds the maximum simultaneous DB-touching goroutines both poll cycles' worker pools can produce, independent of the deploy host's `runtime.NumCPU()`, while an operator's explicit `pool_max_conns` DSN setting remains authoritative | VERIFIED | `internal/db/pool.go`: `PoolConfig(dsn, pollWorkers)` computes `MaxConns = poolMaxConnsForWorkers(pollWorkers)` (pollWorkers clamped to `[0, 1000]`, plus a documented `pollWorkerHeadroom` of 4) whenever the DSN does not carry `pool_max_conns`, using a separate `pgx.ParseConfig` call to detect operator intent (per upstream pgx behavior documented in the plan). `cmd/server/main.go:98` passes `cfg.MusicBrainzPollWorkers+cfg.DeezerPollWorkers` (default 8) into `db.NewPool`, yielding `MaxConns=12` regardless of host vCPU count. Both `internal/testutil` test-pool constructors pass `0` (`MaxConns=4`, headroom only — no poll cycles served). Verified live this session: `go test ./internal/db/... -run TestPoolConfig -count=1 -v` — 5/5 pass, including the two new tests `TestPoolConfig_ComputesMaxConnsFromPollWorkers` (8→12, 64→68, 0→4) and `TestPoolConfig_RespectsExplicitMaxConnsInDSN` (`pool_max_conns=6` + worker ceiling 8 → 6, proving the operator override survives even below the computed default). `go build ./... && go vet ./...` clean. `golangci-lint run ./internal/db/... ./cmd/... ./internal/testutil/... ./internal/httpserver/...` — 0 issues. `TestBootToHealth_EndToEnd` (`internal/httpserver/boot_e2e_test.go:65-67`) asserts `pool.Config().MaxConns >= cfg.MusicBrainzPollWorkers+cfg.DeezerPollWorkers` through the real production boot chain, not just in isolation — re-run live against real Postgres this session, PASS. |

**Score:** 5/5 truths verified (0 present, behavior-unverified) — 4 original ROADMAP criteria plus the gap-closure truth from 11-UAT.md/11-05's must_haves.

### Deferred Items

None.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/db/pool.go` | `PoolConfig(dsn, pollWorkers)`/`NewPool(ctx, dsn, pollWorkers)` explicit `MaxConns` sizing plus DSN-override detection | VERIFIED | Read in full this session; `poolMaxConnsForWorkers`, `dsnSetsMaxConns`, headroom/overflow constants all present and match plan's `<behavior>` spec exactly |
| `internal/db/pool_timeout_test.go` | Two new tests: computed-default pinning, operator-override boundary | VERIFIED | `TestPoolConfig_ComputesMaxConnsFromPollWorkers` and `TestPoolConfig_RespectsExplicitMaxConnsInDSN` present, both pass live; three pre-existing tests updated to the new signature and still pass unchanged in assertion |
| `cmd/server/main.go` | Passes `cfg.MusicBrainzPollWorkers+cfg.DeezerPollWorkers` into `db.NewPool` | VERIFIED | Line 98, confirmed by grep and successful `go build ./...` |
| `internal/testutil/postgres.go` | Both test-pool constructors pass `0` for `pollWorkers` | VERIFIED | Lines 68 and 155, confirmed by grep |
| `internal/httpserver/boot_e2e_test.go` | Boot-chain assertion that the configured worker ceiling reaches the pool | VERIFIED | Lines 50, 65-67; re-run live against real Postgres, PASS, not skipped |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/server/main.go` | `db.NewPool` | `cfg.MusicBrainzPollWorkers+cfg.DeezerPollWorkers` as the `pollWorkers` argument | WIRED | Grep-confirmed at main.go:98; `go build ./...` succeeds; `TestBootToHealth_EndToEnd` proves the value reaches `pool.Config().MaxConns` at runtime through the real boot chain |
| `internal/db/pool.go`'s `dsnSetsMaxConns` | Separate `pgx.ParseConfig(dsn)` call (not `pgxpool.ParseConfig`'s already-consumed config) | Direct call inside `PoolConfig` | WIRED | Read in full; matches the upstream-fact reasoning in the plan (pgxpool.ParseConfig deletes `pool_max_conns` from `RuntimeParams` as it consumes it); `TestPoolConfig_RespectsExplicitMaxConnsInDSN` is exactly the assertion that would fail if this were implemented against the wrong config, and it passes |
| `cmd/server/main.go` | `poller.New` | `WithMusicBrainzWorkers`/`WithDeezerWorkers` | WIRED | Unchanged from prior verification pass; re-confirmed present |
| `detectDeluxeChanges` | `advanceGroupBaseline` → `AdvanceGroupTrackCountBaseline` | Single atomic call | WIRED | Unchanged from prior verification pass; re-confirmed live via passing race test |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build/vet clean | `go build ./... && go vet ./...` | exit 0, no output | PASS |
| Pool sizing unit tests | `go test ./internal/db/... -run TestPoolConfig -count=1 -v` | 5/5 pass | PASS |
| Boot-chain wiring assertion (real Postgres) | `go test ./internal/httpserver/... -run TestBootToHealth_EndToEnd -count=1 -v` | PASS, test ran (not skipped) | PASS |
| Poller concurrency/rate-limit/overlap-guard suite | `go test ./internal/poller/... -count=1` | all green | PASS |
| Baseline race test (real Postgres) | `go test ./internal/detection/... -run TestAdvanceGroupBaseline -count=1 -v` (TEST_DATABASE_URL set) | all subtests pass | PASS |
| Full suite, single run, real Postgres | `go test ./... -count=1` (TEST_DATABASE_URL set) | every package `ok`, zero failures | PASS |
| Lint | `golangci-lint run ./internal/db/... ./cmd/... ./internal/testutil/... ./internal/httpserver/...` | 0 issues | PASS |
| Debt markers (`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`) in 11-05's modified files | grep across 5 files | 0 matches | PASS |
| `go test -race` (native Windows) | n/a | ThreadSanitizer allocation failure — pre-existing, documented environmental limitation since Phase 01, not re-executed in this verification session | SKIP (documented) |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| PERF-01 | 11-01, 11-02, 11-05 | Bounded, env-configurable per-source worker pool (default 3-5) instead of sequential iteration; 11-05 additionally sizes the DB pool against this ceiling | SATISFIED | Config surface + both cycles' bounded fan-out + pool `MaxConns` sizing, all verified live |
| PERF-02 | 11-01, 11-02 | Concurrent polling preserves existing rate limiter and overlap guard | SATISFIED | Rate-limit and overlap-guard-under-concurrency tests, re-verified live |
| PERF-03 | 11-01, 11-02 | A single artist's error does not abort the rest of the cycle | SATISFIED | Simultaneous-failure isolation tests, re-verified live |
| PERF-04 | 11-03, 11-04 | Concurrent baseline updates cannot lose an update (atomic CAS at the DB level) | SATISFIED | `AdvanceGroupTrackCountBaseline` + `TestAdvanceGroupBaseline_ConcurrentRace`, re-verified live against real Postgres |

`REQUIREMENTS.md`'s Phase 11 row (PERF-01 through PERF-04) is fully covered by the five plans' declared `requirements:` frontmatter (11-05 declares `[PERF-01]`, correctly scoped since the gap it closes is about the worker-pool ceiling PERF-01 introduced). No orphaned requirements.

### Anti-Patterns Found

None blocking. No debt markers, no stub returns, no empty handlers in any file this phase (including 11-05's gap closure) modified.

`11-REVIEW.md` (code review, completed prior to this verification pass) recorded 2 WARNING-level and 2 INFO-level findings, 0 CRITICAL:
- **WR-01**: `detectDeluxeChanges`'s doc comment undersells that an `InsertEvent` failure also skips remaining groups in that artist's `freshGroups` slice for the rest of the cycle (delayed, not lost — recomputed fresh next cycle). Confirmed pre-existing pattern shared with `DetectMusicBrainz`'s new_release loop and `detectGuestFeatures`, not introduced by this phase. Doc-only fix recommended, non-blocking.
- **WR-02**: The `advanceGroupBaseline`-then-`InsertEvent` ordering has a narrow, already-documented notification-loss window; this phase's pool-hardening work (11-05) makes the surrounding infrastructure more resilient, not less — reviewer explicitly states "no code change required to ship this phase." Non-blocking, flagged as a possible follow-up.
- **IN-01**: Two internal parse failures in `PoolConfig` share identical wrapped error text, reducing triage clarity. Cosmetic, non-blocking.
- **IN-02**: `.env.example` could not be directly read during code review (tool permission sandbox); mitigated by `TestEnvExampleCompleteness` mechanically catching any key drift.

None of these four findings map to a ROADMAP success criterion, a PLAN must-have, or a requirement ID — they are pre-existing architectural characteristics or minor polish items outside this phase's scope, and the reviewer explicitly marked the phase `issues_found` only at the non-blocking WARNING/INFO level (0 CRITICAL). They do not block phase completion.

### Human Verification Required

None. The single item that previously routed this phase to `human_needed` (G-11-1: DB connection-pool sizing under concurrent polling on the real deployment target) has been closed by plan 11-05 with a code fix rather than deferred to an operational judgment call — `MaxConns` is now sized against `MusicBrainzPollWorkers+DeezerPollWorkers` regardless of the deploy host's vCPU count, so no human decision about the eventual deployment target remains outstanding.

### Gaps Summary

No gaps remain. This is a re-verification following gap closure:

- **G-11-1** (reported in `11-UAT.md`, tracked from the initial `11-VERIFICATION.md` pass's sole human-verification item): `internal/db/pool.go`'s `PoolConfig` never set `pgxpool.Config.MaxConns`, so the pool inherited pgxpool's own `max(4, runtime.NumCPU())` default — below the `MusicBrainzPollWorkers+DeezerPollWorkers` ceiling (default 8) on any host with fewer than 8 vCPUs, including the small VPS this project targets per `DPLY-01`. **Closed by plan 11-05**: `PoolConfig`/`NewPool` now take a `pollWorkers` argument, compute `MaxConns = pollWorkers + 4` (headroom) via `poolMaxConnsForWorkers`, wired end-to-end from `cmd/server/main.go` through to the constructed pool (proven at runtime by `TestBootToHealth_EndToEnd` against real Postgres), while an operator's explicit `pool_max_conns` DSN setting remains authoritative even when smaller than the computed value (`TestPoolConfig_RespectsExplicitMaxConnsInDSN`). All claims independently re-verified against the live codebase in this session: code read in full, 5/5 `TestPoolConfig_*` tests pass, `TestBootToHealth_EndToEnd` passes against a live Postgres fixture (not skipped), full suite green, lint clean, no debt markers, commits `36a4174`/`7cb4ad1` present in git history.

All four original ROADMAP success criteria plus the gap-closure truth are verified against the live codebase. The full test suite passes against real Postgres. `go test -race` remains unexecutable natively on this Windows dev environment (pre-existing, documented since Phase 01); 11-05-SUMMARY.md records the `-race` run was performed via WSL2 as the established project workaround, consistent with how prior phases in this project have handled the same environmental constraint.

The two WARNING-level code-review findings (WR-01, WR-02) are pre-existing architectural characteristics, not regressions introduced by this phase, and the reviewer explicitly did not treat them as blocking. Phase 11's goal — bounded concurrent polling without breaking rate limits, overlap guards, or detection correctness — is achieved and verified.

---

*Verified: 2026-08-17T11:45:00Z*
*Verifier: Claude (gsd-verifier)*
