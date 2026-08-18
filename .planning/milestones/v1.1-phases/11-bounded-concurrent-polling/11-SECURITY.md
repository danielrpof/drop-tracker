---
phase: 11
slug: bounded-concurrent-polling
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-17
---

# Phase 11 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| operator environment → process config | `MUSICBRAINZ_POLL_WORKERS` / `DEEZER_POLL_WORKERS` are operator-supplied integers that directly size a goroutine pool | worker-count integers |
| MusicBrainz API → worker closure | externally-supplied release-group slices processed inside concurrent goroutines | release-group JSON |
| Deezer API → worker closure | externally-supplied album slices processed inside concurrent goroutines | album JSON |
| poller worker → shared `*rate.Limiter` | many goroutines now queue on one token bucket that previously saw one caller at a time | request tokens |
| cron tick → cycle-overlap guard | a second tick can arrive while N workers are in flight, not just while one fetch is in flight | cycle state |
| concurrent poller workers → shared `events.track_count` row | two workers polling different artists that credit the same release group now mutate one row concurrently | track-count integer |
| MusicBrainz release detail → stored baseline | an externally-supplied track count becomes persisted comparison state | track-count integer |
| detection code → database | the compare-and-set statement is the only new SQL authored in this phase | parameterized SQL |
| test process → shared fixture database | one package's test statements can destroy state every other package's tests depend on | schema/table state |
| test code → production notifier source | a testability seam added to production code is production code | spacing-wait duration |
| operator environment → process (DSN) | `DATABASE_URL` carries both a password and now a pool-sizing parameter (`pool_max_conns`) this code reads | connection string incl. credential |
| process → Postgres | computed `MaxConns` determines how many server-side connection slots this process claims out of a shared, finite `max_connections` | connection count |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-11-01 (plan 01) | Denial of Service | `config.Load` / `poller.New` worker-pool sizing | medium | mitigate | `envDefault:"3"`/`"5"` bound the pool when unset; `Load` rejects `<=0` with named error (config.go:79-84); `New` independently rejects `<=0` (poller.go:200-205) | closed |
| T-11-02 (plan 01) | Denial of Service (self-inflicted) | `RunMusicBrainzCycle` worker closure | medium | mitigate | Buffered-channel semaphore + `sync.WaitGroup`, no shared error/cancellation object (poller.go:304-380); `errgroup` grep-gated to 0 | closed |
| T-11-03 (plan 01) | Denial of Service | goroutine lifetime on context cancellation | medium | mitigate | Semaphore acquisition selects on `ctx.Done()` (poller.go:310-315); dispatch loop breaks (not returns) so `wg.Wait()` always reached | closed |
| T-11-04 (plan 01) | Information Disclosure | `poll cycle complete` log line | low | accept | Line carries only `artist_count`/`duration_ms` (poller.go:397-400), no artist identity or credential | closed |
| T-11-05 (plan 01) | Tampering | MusicBrainz release-group slices in worker goroutines | low | accept | Entry passed as goroutine parameter, no shared indexing | closed |
| T-11-SC (plan 01) | Tampering | package-manager installs | low | accept | No new dependency; `go.mod` unchanged | closed |
| T-11-06 (plan 02) | Denial of Service | outbound request rate to MusicBrainz/Deezer under concurrent workers | **high** | mitigate | Shared `*rate.Limiter` left completely unwrapped (`git status --porcelain internal/musicbrainz internal/deezer` empty); empirically proven with real captured request timing via `TestMusicBrainzCycle_ConcurrentPollingStaysInsideRateLimit` / `TestDeezerCycle_ConcurrentPollingStaysInsideRateLimit` | closed |
| T-11-07 (plan 02) | Denial of Service (self-inflicted) | `RunDeezerCycle` worker closure | medium | mitigate | Mirrors T-11-02's shape exactly (poller.go:454-524) | closed |
| T-11-08 (plan 02) | Denial of Service | cycle-overlap guard under concurrency | medium | mitigate | `dzRunning`/`mbRunning` released only after worker join; proven a second cycle is rejected while workers in flight and the two sources' guards remain independent | closed |
| T-11-09 (plan 02) | Denial of Service | goroutine lifetime on context cancellation in `RunDeezerCycle` | medium | mitigate | Same cancellation-select pattern mirrored from T-11-03 (poller.go:468-473) | closed |
| T-11-10 (plan 02) | Repudiation | per-artist log attribution under interleaved concurrent output | low | mitigate | Every per-artist record carries `cycle_id`/`source`/`artist_mbid`/`artist_name`; emission order deliberately unasserted (D-07) | closed |
| T-11-11 (plan 02) | Tampering | Deezer album slices inside worker goroutines | low | accept | Same per-goroutine parameter isolation as T-11-05 | closed |
| T-11-SC (plan 02) | Tampering | package-manager installs | low | accept | No new dependency | closed |
| T-11-12 (plan 03) | Tampering (data integrity) | `events.track_count` deluxe-change baseline under concurrent callers | **high** | mitigate | Former two-statement TOCTOU lost-update race closed by one atomic statement with a `FOR UPDATE` row lock (events.sql:70-80); proven by `TestAdvanceGroupBaseline_ConcurrentRace` (`-race -count=10`), hand-falsified against a non-atomic implementation (reliably reproduced the bug — stored 10 instead of the correct max 12) | closed |
| T-11-13 (plan 03) | Tampering | write-once display-snapshot columns | medium | mitigate | New statement's `SET` list touches `track_count` only (events.sql:75-76); `title`/`artist_name`/`release_date`/`cover_art_url` stay write-once via `InsertEvent`'s conflict clause | closed |
| T-11-14 (plan 03) | Repudiation | lost `deluxe_change` notification in advance-then-insert crash window | medium | accept | Closing requires transaction machinery this codebase doesn't have; accepted with a "Known, accepted edge" doc paragraph (musicbrainz.go:278-294) + `logger.Warn` on the reachable non-crash failure path (musicbrainz.go:393) + a backstop truth in the plan | closed |
| T-11-15 (plan 03) | Denial of Service | row-lock contention on a popular shared release group | low | accept | Lock scoped to one row; worker pools bounded to 3/5 | closed |
| T-11-16 (plan 03) | Elevation of Privilege / Injection | the new SQL statement | low | mitigate | Statement lives only in `queries/events.sql`, reached via sqlc-generated parameterized code; `grep` for raw SQL in `internal/detection/*.go` returns 0 | closed |
| T-11-17 (plan 03) | Tampering | retention filter accidentally added to the replacement query | medium | mitigate | Phase 10's `TestRetention_DetectionStateQueriesStayUnfiltered` repointed at the replacement query with a stronger single-row assertion; enumeration comment updated | closed |
| T-11-SC (plan 03) | Tampering | package-manager installs | low | accept | No new dependency | closed |
| T-11-18 (plan 04) | Denial of Service | shared fixture database availability to concurrently-running test packages | medium | mitigate | Destructive `DROP SCHEMA public` removed, replaced with dedicated `migrate_scratch` schema via `search_path` DSN param; two `to_regclass` isolation assertions prove both directions | closed |
| T-11-19 (plan 04) | Tampering | production notifier behavior, via the test-only spacing seam | medium | mitigate | Seam is an unexported package-level var (notifier.go:56-67) mirroring the existing `dbOpTimeout` precedent; setter lives in `export_test.go` (test-binary only); falsification check confirms the rewritten assertions still detect a real regression | closed |
| T-11-20 (plan 04) | Repudiation | suite made green by masking rather than fixing | medium | mitigate | No parallelism-limiting flag introduced (`grep -c '-p 1'` returns 0 in Makefile and all CI workflows); five-run stability result recorded; todo closed with confirmed root causes | closed |
| T-11-21 (plan 04) | Information Disclosure | derived scratch DSN in test output/logs | low | accept | Pre-existing `redactDSN`/`redactError` (migrate.go:167-193) unaffected by the added `search_path` param, which carries no credential material | closed |
| T-11-SC (plan 04) | Tampering | package-manager installs | low | accept | `net/url` stdlib only | closed |
| T-11-01 (plan 05) | Information Disclosure | `internal/db/pool.go`'s new `pgx.ParseConfig` override-detection call | **high** | mitigate | `dsnSetsMaxConns`'s error path built via `redactedTarget(dsn)` (pool.go:177-180), matching every other error path in the file; raw `dsn` never interpolated into any error message | closed |
| T-11-02 (plan 05) | Denial of Service | computed `MaxConns` vs Postgres `max_connections` | medium | mitigate | `pollWorkers` clamped to `[0, 1000]` before conversion (pool.go:85-93); operator's `pool_max_conns` stays authoritative even below the computed default | closed |
| T-11-03 (plan 05) | Tampering | npm/pip/cargo/go installs | low | accept | `pgx/v5` already a direct require; `go.mod` diff empty | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

**Note on threat-ID collisions:** Plan 05 reuses IDs `T-11-01`/`T-11-02`/`T-11-03` from plan 01 for unrelated threats. Each row above is disambiguated by its `(plan NN)` suffix; treat the ID alone as non-unique within this phase.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-11-01 | T-11-04 (plan 01) | `poll cycle complete` log line carries only aggregate counters, no PII/credentials | gsd-security-auditor | 2026-08-17 |
| AR-11-02 | T-11-05 (plan 01) | Each worker owns an independent slice with no shared indexing | gsd-security-auditor | 2026-08-17 |
| AR-11-03 | T-11-SC (plans 01-04) | No new package-manager dependency introduced by this phase | gsd-security-auditor | 2026-08-17 |
| AR-11-04 | T-11-11 (plan 02) | Deezer worker goroutines have the same per-goroutine isolation as MusicBrainz | gsd-security-auditor | 2026-08-17 |
| AR-11-05 | T-11-14 (plan 03) | Advance-then-insert crash window requires transaction machinery this codebase doesn't have; closing it is disproportionate to the window's size (between two commits). Compensating controls: documented doc comment, Warn log on the reachable half, backstop truth in plan. | gsd-security-auditor | 2026-08-17 |
| AR-11-06 | T-11-15 (plan 03) | Row-lock scoped to one row; worker pools already bounded | gsd-security-auditor | 2026-08-17 |
| AR-11-07 | T-11-21 (plan 04) | Existing DSN redaction already covers the new `search_path` parameter, which carries no credential | gsd-security-auditor | 2026-08-17 |
| AR-11-08 | T-11-03 (plan 05) | No new module dependency; `pgx/v5` already direct-required | gsd-security-auditor | 2026-08-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-17 | 28 (24 unique + 4 duplicate T-11-SC entries) | 28 | 0 | gsd-security-auditor |

**Follow-up (non-blocking):** the auditor flagged that `pool.go`'s wrapped `pgx.ParseConfig`/`pgxpool.ParseConfig` errors rely on `%w`-wrapping the raw pgx error object rather than the `redactError`-style scrubbing `internal/db/migrate.go` applies to its own pgx errors. This pattern pre-dates plan 11-05 (plan 11-05 mirrored the existing convention rather than introducing a weaker one) and is not a threat this phase introduced. Recorded as a candidate hardening item for a future phase, not a blocker here.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-17
