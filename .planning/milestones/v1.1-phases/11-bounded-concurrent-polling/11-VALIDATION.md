---
phase: 11
slug: bounded-concurrent-polling
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-16
validated: 2026-08-17
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `go test -race` |
| **Config file** | none — plain `go test ./...` per `Makefile`'s existing `test`/`test-short`/`test-integration` targets |
| **Quick run command** | `go test ./internal/poller/... ./internal/detection/... -short` |
| **Full suite command** | `TEST_DATABASE_URL=... go test ./... -race` |
| **Estimated runtime** | not measured this session — full suite includes real-Postgres integration tests, unchanged in kind by this phase |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/poller/... ./internal/detection/... -short`
- **After every plan wave:** Run `TEST_DATABASE_URL=... go test ./... -race`
- **Before `/gsd-verify-work`:** Full suite must be green, including `-race` (Criterion 4's explicit requirement)
- **Max feedback latency:** 1 task commit (per-task quick command; no watch-mode)

---

## Per-Task Verification Map

Reconciled against the phase's actual 5 plans and their SUMMARY.md coverage blocks. All statuses re-run live against a real Postgres fixture during this audit (2026-08-17) — see Validation Audit below.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 01-1 | 11-01 | 1 | PERF-01 | T-11-01, T-11-02, T-11-03 | MusicBrainz cycle fans out over a bounded, env-configurable worker pool (default 3); config validates positive | unit | `go test ./internal/config/... ./internal/poller/... -run 'TestMusicBrainzCycle_ConcurrencyBoundedByWorkerCount\|TestEnvExampleCompleteness' -v` | ✅ | ✅ green |
| 01-2 | 11-01 | 1 | PERF-01 | T-11-01 | `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` default/override/reject-non-positive | unit | `go test ./internal/config/... -run 'TestLoad_PollWorkerDefaults\|TestLoad_PollWorkerOverrides\|TestLoad_RejectsNonPositivePollWorkers' -v` | ✅ | ✅ green |
| 01-3 | 11-01 | 1 | PERF-01, PERF-02 | — | Boundary/empty/single-entry/log-precision edges; no test still asserts sequential-only polling | unit | `go test ./internal/poller/... -run 'TestMusicBrainzCycle_WorkerCountOneIsSequential\|TestMusicBrainzCycle_WorkerCountAboveEntryCountFansOutToEntryCount\|TestMusicBrainzCycle_SingleEntryReachesOneInFlight\|TestMusicBrainzCycle_LogsCycleDurationAndArtistCount' -count=3 -v` | ✅ | ✅ green |
| 02-1 | 11-02 | 2 | PERF-01 | T-11-07, T-11-09 | Deezer cycle mirrors the bounded fan-out; nil-`DeezerID` entries skip without consuming a worker slot | unit | `go test ./internal/poller/... -run 'TestDeezerCycle_ConcurrencyBoundedByWorkerCount\|TestDeezerCycle_WorkerCountEqualToEntryCountRunsAllConcurrently\|TestDeezerCycle_NilDeezerIDConsumesNoWorkerSlot' -v` | ✅ | ✅ green |
| 02-2 | 11-02 | 2 | PERF-02 | T-11-06, T-11-08 | Concurrent workers sharing one rate.Limiter stay inside the per-source rate; overlap guard holds while multiple workers are in flight; sources stay independent | unit + real-timestamp | `go test ./internal/poller/... -run 'RateLimit\|OverlapGuard\|RunsWhileMusicBrainzWorkersInFlight' -count=3 -v` | ✅ | ✅ green |
| 02-3 | 11-02 | 2 | PERF-03 | T-11-10 | Simultaneous multi-artist failures (fetch + detection origin, both sources) don't abort the cycle; per-artist log lines fully labelled under concurrency | unit | `go test ./internal/poller/... -run 'Simultaneous\|ConcurrentLogLinesAreFullyLabelled' -count=5 -v` | ✅ | ✅ green |
| 03-1 | 11-03 | 2 | PERF-04 | T-11-12, T-11-17 | `AdvanceGroupTrackCountBaseline` atomic CAS replaces the two-statement read/write; Phase 10 criterion-5 guard repointed | integration, real Postgres | `go test ./internal/httpserver/... -run TestRetention_DetectionStateQueriesStayUnfiltered -v` | ✅ | ✅ green |
| 03-2 | 11-03 | 2 | PERF-04 | T-11-13, T-11-14 | `detectDeluxeChanges` branching re-derived from the single atomic call; byte-identical outcomes on the full pre-existing deluxe-change suite | unit + integration | `go test ./internal/detection/... -v` | ✅ | ✅ green |
| 03-3 | 11-03 | 2 | PERF-04 | T-11-12 | Two callers racing `advanceGroupBaseline` on the same release group leave the stored baseline at the true maximum — proven by read-back state, falsified against a non-atomic implementation | integration, real Postgres, `-race` | `go test ./internal/detection/... -run TestAdvanceGroupBaseline -count=10 -v` | ✅ | ✅ green (re-run live this audit, -count=3) |
| 04-1 | 11-04 | 2 | PERF-04 (suite stability) | T-11-18 | Destructive `DROP SCHEMA public CASCADE` in the migrate-from-scratch test replaced with dedicated-schema isolation, proven by `to_regclass` assertions | integration, real Postgres | `go test ./internal/db/... -run TestRunMigrations -count=2 -v` | ✅ | ✅ green |
| 04-2 | 11-04 | 2 | PERF-04 (suite stability) | T-11-19 | Notifier inter-send spacing asserted on requested durations via a swappable seam, not elapsed wall time; WR-01 contract preserved | unit | `go test ./internal/notifier/... -count=5 -v` | ✅ | ✅ green |
| 04-3 | 11-04 | 2 | PERF-04 (suite stability) | T-11-20 | Full suite green across 5 separate consecutive runs at default package parallelism, no `-p 1` workaround; flaky-test todo closed with confirmed root causes | integration, full suite | `go test ./... -count=1` | ✅ | ✅ green (re-run live this audit) |
| 05-1 | 11-05 | 2 (gap closure G-11-1) | PERF-01 | T-11-01, T-11-02 | Postgres pool `MaxConns` sized against `MusicBrainzPollWorkers + DeezerPollWorkers` (+4 headroom), independent of host vCPU count; operator's `pool_max_conns` DSN setting stays authoritative even below the computed default | unit | `go test ./internal/db/... -run TestPoolConfig -v` | ✅ | ✅ green |
| 05-2 | 11-05 | 2 (gap closure G-11-1) | PERF-01 | — | The configured worker ceiling actually reaches the constructed pool through the real production boot chain | integration, real Postgres | `go test ./internal/httpserver/... -run TestBootToHealth_EndToEnd -v` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

All Wave 0 items were completed during phase execution (plans 11-01 through 11-04), not deferred to a separate validation pass:

- [x] `internal/poller/poller_test.go` — `TestMusicBrainzCycle_Sequential` deleted outright (its `maxInFlight <= 1` guarantee was deliberately reversed by PERF-01); replaced by `TestMusicBrainzCycle_WorkerCountOneIsSequential` and the Deezer-side twin suite (11-01 Task 3, 11-02 Task 1)
- [x] `internal/poller/poller_test.go` — rate-limit-honored-under-concurrency test using real captured timestamps: `TestMusicBrainzCycle_ConcurrentPollingStaysInsideRateLimit` / `TestDeezerCycle_ConcurrentPollingStaysInsideRateLimit` (11-02 Task 2)
- [x] `internal/detection/baseline_test.go` — `TestAdvanceGroupBaseline_ConcurrentRace`, asserting final stored state, falsified against a non-atomic two-step implementation (11-03 Task 3)
- [x] `internal/notifier` — deterministic `spacingWait` seam + `export_test.go` setter replacing wall-clock timestamp-difference assertions in the 2 genuinely timing-sensitive tests (11-04 Task 2; research corrected the folded-in todo's "4 flaky tests" claim down to 2 real timing assertions plus a 3rd, previously-undiagnosed cross-package DB-pollution root cause, also fixed)
- [x] `internal/db/migrate_test.go` — `TestRunMigrations_AppliesFromScratch` isolated onto a dedicated `migrate_scratch` schema via a `search_path` DSN parameter, replacing the raw `DROP SCHEMA public CASCADE` (11-04 Task 1)

---

## Manual-Only Verifications

All phase behaviors have automated verification. One accepted residual edge is documented rather than tested (cannot be automated without a process-kill harness):

- **PERF-04 crash window**: a process crash in the gap between the baseline-advance commit and the `deluxe_change` event insert permanently loses that one notification (11-RESEARCH.md Pitfall 1). Accepted per plan 11-03's threat model (T-11-14) with compensating controls: a `Known, accepted edge:` doc paragraph in `detectDeluxeChanges`, and a dedicated `logger.Warn` on the reachable (non-crash) half of the failure path. This is a documented backstop-tier truth, not a coverage gap — closing it would require transaction wiring this codebase doesn't have, for a window measured in the time between two commits.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 1 task commit
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-17 (retroactive audit via `/gsd-validate-phase`)

---

## Validation Audit 2026-08-17

Retroactive audit: `11-VALIDATION.md` was still in its plan-phase-seeded `draft` state (task IDs `TBD`) despite all 5 plans having completed and been independently re-verified live in `11-VERIFICATION.md` (status: passed, 5/5 truths, 0 gaps remaining, dated 2026-08-17T11:45:00Z). This audit reconciled the map against the actual plans/summaries and re-ran the key automated proofs live against a running Postgres fixture (`docker` container on `localhost:5432`) as an independent check, rather than trusting the summaries alone.

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 (none needed — all 4 requirements already had complete automated coverage) |
| Escalated | 0 |
| Tests re-run live this session | `go build ./...` clean; `go test ./internal/poller/... ./internal/config/... ./internal/db/... -short` green; `go test ./internal/detection/... -run TestAdvanceGroupBaseline -count=3 -v` green (all subtests); `go test ./... -count=1` (full suite against real Postgres) — every package `ok`, zero failures |

Note: `-race` remains unexecutable natively on Windows dev environments in this project (documented, pre-existing cgo/ThreadSanitizer limitation since Phase 01); plan 11-05's summary records a successful WSL2 `-race` run covering the full suite including this phase's concurrency and race tests, which is the established project workaround and was not re-executed in this audit session.
