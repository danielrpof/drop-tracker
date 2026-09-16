---
phase: 11-bounded-concurrent-polling
reviewed: 2026-08-17T00:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - Makefile
  - cmd/server/main.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/db/migrate_test.go
  - internal/db/pool.go
  - internal/db/pool_timeout_test.go
  - internal/db/sqlc/events.sql.go
  - internal/db/sqlc/querier.go
  - internal/detection/baseline_test.go
  - internal/detection/detector.go
  - internal/detection/musicbrainz.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/events_test.go
  - internal/notifier/export_test.go
  - internal/notifier/notifier.go
  - internal/notifier/notifier_test.go
  - internal/poller/poller.go
  - internal/poller/poller_test.go
  - internal/testutil/postgres.go
  - queries/events.sql
  - .env.example
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 11: Code Review Report

**Reviewed:** 2026-08-17
**Depth:** standard (with deep cross-file tracing on the CTE-locking and pool-sizing paths per the review brief)
**Files Reviewed:** 21
**Status:** issues_found

## Summary

This phase adds bounded worker-pool fan-out to `RunMusicBrainzCycle`/`RunDeezerCycle`, replaces a check-then-act deluxe-change baseline update with a single `FOR UPDATE` CTE, and sizes `pgxpool`'s `MaxConns` off the configured worker ceiling via a second `pgx.ParseConfig` call used only to detect an operator-supplied `pool_max_conns`.

I traced the concurrency-bearing code paths by hand against the Go memory model and Postgres's READ COMMITTED / `FOR UPDATE` re-evaluation semantics, not just by reading the (extensive) accompanying tests:

- **Worker-pool bound (`poller.go`)**: the semaphore acquire/release, the double `ctx.Err()` check (dispatch-loop select + per-worker recheck), the panic-recovery wrapper, and the `wg.Wait()`-before-cycleErr-report ordering are all correct. I could not find a goroutine leak, a semaphore-permanently-held path, or a data race on `cycleErr`/`mbRunning`/`dzRunning` (all writes to `cycleErr` happen only on the dispatch-loop's own goroutine; the atomics are used correctly for the CAS overlap guards).
- **`AdvanceGroupTrackCountBaseline` CTE (`queries/events.sql` / `detector.go`)**: the `WITH existing AS (... FOR UPDATE) UPDATE ... FROM existing` pattern is the correct, standard Postgres idiom for a race-free compare-and-set. Postgres does not inline a CTE containing a locking clause, and a transaction that blocks on the `FOR UPDATE` row lock re-evaluates against the newly-committed row once unblocked (READ COMMITTED's EvalPlanQual re-check) — so the lost-update race this replaces is genuinely closed, not just narrowed. `TestAdvanceGroupBaseline_ConcurrentRace` empirically confirms this.
- **Pool sizing (`db/pool.go`)**: `poolMaxConnsForWorkers`'s clamp-then-convert to `int32` cannot overflow; `dsnSetsMaxConns`'s separate `pgx.ParseConfig` call is the only way to observe `pool_max_conns` before `pgxpool.ParseConfig` consumes it from `RuntimeParams`, and the reasoning in the comments matches actual `pgxpool` behavior.

I found no BLOCKER-level bugs in the reviewed diff. The two WARNING-level items below are real, but narrow, and one of them is already partially (if imprecisely) documented by the authors themselves.

## Warnings

### WR-01: `detectDeluxeChanges`'s documented "accepted edge" undersells its own blast radius

**File:** `internal/detection/musicbrainz.go:295-416` (the `default:` branch at ~366-399), cross-referenced with `queries/events.sql:42-80`

**Issue:** The method's doc comment (lines 278-294) documents that a crash/error between `advanceGroupBaseline`'s commit and the `InsertEvent` call permanently loses *that one group's* notification, and frames this as a narrow, accepted residual. What the comment does not mention: on an `InsertEvent` error, the `default:` branch does `return fmt.Errorf(...)` immediately, escaping the `for _, g := range freshGroups` loop entirely (musicbrainz.go:398). This means every *other* release-group later in that same artist's `freshGroups` slice — for which `advanceGroupBaseline` had not yet even run — is silently skipped for the rest of *this* cycle's deluxe-change pass too, not just the one group whose insert failed.

In practice the blast radius is bounded (each group is reconsidered fresh on the *next* poll cycle, since `freshGroups`/`preCycleSeen` are recomputed every cycle), so this is not data loss for those other groups — only a one-cycle delay. Only the specific group whose baseline had already advanced when the insert failed loses its notification permanently, exactly as documented. I confirmed this same "abort the whole per-event-type loop on the first `InsertEvent` error" pattern also exists, identically, in `DetectMusicBrainz`'s own new_release loop (musicbrainz.go:118-120) and in `detectGuestFeatures` (musicbrainz.go:212-214) — so this is pre-existing Detector architecture, not something phase 11 introduced. Phase 11's atomic-CTE change does, however, make the interaction sharper: the baseline for the failing group has *already durably moved* by the time the early return happens, in a way the two-statement predecessor never risked in the same statement.

**Fix:** Tighten the doc comment on `detectDeluxeChanges` (musicbrainz.go:278-294) to state explicitly that an `InsertEvent` failure also skips any remaining groups in this artist's `freshGroups` slice for the rest of the current cycle (delayed, not lost, for those), so a future reader doesn't assume the failure is scoped to exactly one group the way the release-detail-fetch error path (which does `continue`, not `return`) is.

### WR-02: `advanceGroupBaseline`-then-`InsertEvent` ordering is a real, if narrow, notification-loss window that this phase's own pool-hardening work makes more, not less, likely to trigger

**File:** `internal/detection/musicbrainz.go:366-399`

**Issue:** This is the specific case WR-01's comment already documents (correctly) as an accepted edge, but it's worth flagging on its own because of what else is in this same phase: `internal/db/pool.go`'s entire raison d'être this session is hardening against exactly the class of failure that would trigger this window — a DB call that fails or hangs mid-cycle (wedged connection, exhausted pool, etc.). If `pool_max_conns` is undersized relative to `mbWorkers+dzWorkers` (an operator override, per `TestPoolConfig_RespectsExplicitMaxConnsInDSN`'s deliberately-supported case), connection-acquire contention under load is more likely, which is precisely the kind of transient failure most likely to land a caller in this window: baseline advances, `InsertEvent` fails, and that release-group's deluxe-change notification is gone forever with no retry path (unlike every other failure mode in this codebase, which the whole notifier/detector design otherwise takes pains to make retry-safe — D-09's re-pickup contract, WR-03's mark-notified-failed handling, etc.).

**Fix:** No code change required to ship this phase (the trade-off is real and reasonably argued), but consider tracking this as a follow-up: either wrap `advanceGroupBaseline` + `InsertEvent` in an explicit transaction once `Detector` gains a transaction-capable seam (the comment already anticipates this), or emit a distinguishable metric/log signal (beyond the existing `Warn` line) so an operator can detect how often this window is actually being hit in production rather than only reading about the risk in a code comment.

## Info

### IN-01: `PoolConfig`'s two internal parse failures share identical wrapped error text

**File:** `internal/db/pool.go:160-186`

**Issue:** `PoolConfig` wraps both `pgxpool.ParseConfig(dsn)`'s failure (line 163) and `dsnSetsMaxConns(dsn)`'s internal `pgx.ParseConfig(dsn)` failure (line 179) with the identical message `"parse pool config for %s: %w"`. Since the comment on line 172-176 explains the second failure is "defensive rather than expected," a production log line reading this text gives no way to tell which of the two parse calls actually failed, which matters for triage if the two parsers ever do diverge on some DSN in practice (they are, after all, two separate calls into two separate functions of the pgx module).

**Fix:**
```go
explicitMaxConns, err := dsnSetsMaxConns(dsn)
if err != nil {
    return nil, fmt.Errorf("parse pool_max_conns override for %s: %w", redactedTarget(dsn), err)
}
```

### IN-02: `.env.example` could not be reviewed — sandboxed out by tool permissions

**File:** `.env.example`

**Issue:** Both the `Read` tool and `Bash cat .env.example` were denied by this session's permission settings ("File is in a directory that is denied by your permission settings" / "Permission to use Bash ... has been denied"), so the new `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` entries this phase should have added to it were not directly inspected. This gap is substantially mitigated by `internal/config/config_test.go`'s `TestEnvExampleCompleteness`, which asserts (and would fail CI on) any drift between `Config`'s `env` struct tags and `.env.example`'s keys — so a missing or misspelled key would already be caught mechanically. What that test cannot catch is documentation quality (e.g., whether the new entries' inline comments correctly explain the worker-count semantics and the D-02 defaults).

**Fix:** Not a code defect; re-run this review (or a scoped follow-up) in an environment where `.env.example` is readable, or have a human confirm the two new lines read sensibly.

---

_Reviewed: 2026-08-17_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
