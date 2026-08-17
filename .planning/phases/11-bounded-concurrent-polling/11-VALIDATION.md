---
phase: 11
slug: bounded-concurrent-polling
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-16
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

Task IDs are assigned when `/gsd-plan-phase` creates PLAN.md files; this map is seeded from RESEARCH.md's Phase Requirements → Test Map and should be reconciled against actual task IDs once plans exist.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | PERF-01 | — | Worker-pool size is env-configurable (3-5 default); a multi-artist cycle finishes measurably faster than sequential | unit + integration (timing) | `go test ./internal/poller/... -run TestMusicBrainzCycle_Concurrent -v` | ❌ W0 — new test | ⬜ pending |
| TBD | TBD | TBD | PERF-02 | — | Concurrent polling stays inside the rate limit; overlap guard still works | unit + real-timestamp-burst | `go test ./internal/poller/... -run TestMusicBrainzCycle_RateLimitHonored -v` | ❌ W0 — new test (overlap-guard half already covered by existing `TestMusicBrainzCycle_OverlapGuard_SkipsWhileInFlight`) | ⬜ pending |
| TBD | TBD | TBD | PERF-03 | — | A single artist's error doesn't abort the rest of the cycle | unit | Existing `TestMusicBrainzCycle_PerArtistErrorContinuesCycle` / `TestPoller_RunMusicBrainzCycle_DetectionErrorIsolatedPerArtist`, extended to run under concurrency | ✅ existing — extend | ⬜ pending |
| TBD | TBD | TBD | PERF-04 | T-11-01 (TOCTOU baseline race) | Baseline CAS cannot lose an update under concurrent racing artists | integration, real Postgres, `-race` | `go test ./internal/detection/... -run TestAdvanceGroupBaseline_ConcurrentRace -race -v` | ❌ W0 — new test (canonical proof for Criterion 4) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/poller/poller_test.go` — replace `TestMusicBrainzCycle_Sequential`'s `maxInFlight <= 1` assertion with a concurrency-bound assertion; add the Deezer-side twin if one exists
- [ ] `internal/poller/poller_test.go` — new rate-limit-honored-under-concurrency test using real captured timestamps, not just trusting `rate.Limiter`'s documented safety
- [ ] `internal/detection/detector_test.go` (or a new file) — `TestAdvanceGroupBaseline_ConcurrentRace`: two goroutines racing `advanceGroupBaseline` on the same `groupMBID` with different counts, asserting the final stored `track_count` equals the true maximum, run under `-race`
- [ ] `internal/notifier` — `Clock` interface + fake implementation for the 4 flaky timing-sensitive tests (folded-in todo)
- [ ] `internal/db/migrate_test.go` — schema isolation fix for `TestRunMigrations_AppliesFromScratch`'s raw `DROP SCHEMA public CASCADE` (folded-in todo)

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 1 task commit
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
