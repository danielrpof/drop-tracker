# Phase 11: Bounded Concurrent Polling - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-16
**Phase:** 11-Bounded Concurrent Polling
**Areas discussed:** Pool sizing & config shape, Speedup observability, Baseline CAS approach (PERF-04), Concurrent log ordering

---

## Todo Cross-Reference

| Todo | Match reason | Folded? |
|------|--------------|---------|
| Fix flaky tests under parallel `go test ./...` (shared-DB contention + notifier timing) | Keyword match: "under", "test", "phase", "poller", "two" (score 0.6) | ✓ Folded in |

**User's choice:** Fold it in — Phase 11 is already touching poller concurrency and adding real concurrent-artist tests, so this is the natural place to also fix the pre-existing notifier/poller test flakiness.

---

## Pool sizing & config shape

| Option | Description | Selected |
|--------|-------------|----------|
| Independent per-source vars | `MUSICBRAINZ_POOL_SIZE` / `DEEZER_POOL_SIZE`-style split, mirroring existing independent rate limiters/guards | ✓ |
| One shared var | Single `POLL_WORKER_POOL_SIZE` for both cycles | |

**User's choice:** Independent per-source vars.
**Notes:** MB's 1 req/sec limiter and Deezer's ~10 req/sec limiter behave very differently under the same pool size.

| Option | Description | Selected |
|--------|-------------|----------|
| MB=3, Deezer=5 | Smaller default for the tightly rate-limited source, larger for the faster one | ✓ |
| 5 and 5 | Same default, top of the 3-5 range | |
| 3 and 3 | Same default, bottom of the 3-5 range | |

**User's choice:** MB=3, Deezer=5.

| Option | Description | Selected |
|--------|-------------|----------|
| `MUSICBRAINZ_POLL_WORKERS` / `DEEZER_POLL_WORKERS` | Distinct from existing `*_RATE_LIMIT_*` vars | ✓ |
| `MUSICBRAINZ_POOL_SIZE` / `DEEZER_POOL_SIZE` | Shorter, names the mechanism directly | |

**User's choice:** `MUSICBRAINZ_POLL_WORKERS` / `DEEZER_POLL_WORKERS`.

---

## Speedup observability

| Option | Description | Selected |
|--------|-------------|----------|
| Add duration_ms to the existing cycle-end log line | Production logs show the speedup, not just a one-time verification test | ✓ |
| Test-only, no production log change | Speedup proven once during verification only | |

**User's choice:** Add `duration_ms` to the existing cycle-end log line.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — duration_ms + artist_count | Makes artists/sec derivable straight from logs | ✓ |
| duration_ms only | Artist count already inferable from per-artist log lines | |

**User's choice:** Yes — `duration_ms` + `artist_count`.

---

## Baseline CAS approach (PERF-04)

| Option | Description | Selected |
|--------|-------------|----------|
| Single atomic UPDATE ... RETURNING | One SQL statement does read+compare+write atomically; Postgres row lock serializes concurrent callers | ✓ |
| SELECT ... FOR UPDATE transaction | Wrap the existing two-call shape in a transaction with a row lock | |

**User's choice:** Single atomic `UPDATE ... RETURNING`.
**Notes:** The establish-vs-advance branching (no baseline yet → silently establish, existing lower baseline → fire event and advance) must be preserved; exact `RETURNING`-clause shape left to research/planning.

---

## Concurrent log ordering

| Option | Description | Selected |
|--------|-------------|----------|
| Interleaved is fine | Each log line is self-labeled (cycle_id/source/artist_mbid/artist_name); no ordering code needed | ✓ |
| Other (buffer/order/worker_id) | — | |

**User's choice:** Interleaved is fine.

---

## Claude's Discretion

- Worker-pool implementation primitive (goroutines + WaitGroup + semaphore vs. `errgroup`), constrained to preserve PERF-03's per-artist error isolation.
- Exact `RETURNING`-clause shape and whether a companion has-baseline signal is still needed after the atomic UPDATE (D-06).
- Whether MusicBrainz and Deezer share one pool implementation or get two independently-instantiated pool objects (config is locked independent per D-01; runtime object structure is not).
- Flaky-test fix approach (clock injection vs. per-package DB isolation vs. `-p 1` pinning vs. accept-as-known-flake) for the folded todo.

## Deferred Ideas

None — discussion stayed within phase scope.
