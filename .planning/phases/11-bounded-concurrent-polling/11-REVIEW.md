---
phase: 11-bounded-concurrent-polling
reviewed: 2026-08-17T06:24:47Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - cmd/server/main.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/db/migrate_test.go
  - internal/db/sqlc/events.sql.go
  - internal/db/sqlc/querier.go
  - internal/detection/baseline_test.go
  - internal/detection/detector.go
  - internal/detection/musicbrainz.go
  - internal/httpserver/events_test.go
  - internal/notifier/export_test.go
  - internal/notifier/notifier.go
  - internal/notifier/notifier_test.go
  - internal/poller/poller.go
  - internal/poller/poller_test.go
  - internal/testutil/postgres.go
  - queries/events.sql
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 11: Code Review Report

**Reviewed:** 2026-08-17T06:24:47Z
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

Phase 11 adds bounded worker-pool fan-out to both poll cycles, replaces the
two-statement deluxe-baseline read/write with a single atomic
compare-and-set query, and fixes several root causes of test flakiness
under Go's default package-level test parallelism.

I traced the semaphore/WaitGroup dispatch loop in `poller.go` line by line
for both `RunMusicBrainzCycle` and `RunDeezerCycle`: defer ordering for
panic recovery → semaphore release → `wg.Done()` is correct (LIFO
guarantees `recover()` runs before the slot is freed and before the
WaitGroup is decremented), the double `ctx.Err()` check (in the dispatch
loop's `select` and again inside each worker) correctly closes the race
window that exists when `workers >= len(entries)` (verified against the
`TestMusicBrainzCycle_ContextCancelledStopsIteration` /
`TestDeezerCycle_ContextCancelledStopsIteration` reasoning, which forces
`workers=1` specifically to make that ordering deterministic rather than
scheduler-luck-dependent — this checks out), and `wg.Wait()` is always
reached on every code path (including cancellation), so no goroutine can
outlive `RunMusicBrainzCycle`/`RunDeezerCycle`'s return.

I also traced `AdvanceGroupTrackCountBaseline` (`queries/events.sql` /
`events.sql.go`) against Postgres's documented READ COMMITTED
re-fetch-and-re-evaluate behavior for `FOR UPDATE`: a second concurrent
caller blocked on the CTE's row lock is guaranteed to see the
already-committed value once unblocked (not a stale pre-lock snapshot),
and `TestAdvanceGroupBaseline_ConcurrentRace`'s three sub-tests correctly
assert on the value read back from the database rather than trusting the
functions' own return values. This closes the lost-update race PERF-04
targets; I did not find a way to make two racing callers lose the higher
count.

No BLOCKER-level defects found. Three WARNINGs are worth fixing: an
identifier-injection pattern in a new test-support function (low practical
risk, but a real anti-pattern the codebase shouldn't establish as
precedent), a genuine (if narrow and already-logged) permanent-data-loss
window introduced by re-ordering the deluxe-baseline advance ahead of the
event insert, and a test-fragility trap in the notifier package's new
package-level test seam.

## Warnings

### WR-01: Unescaped identifier interpolation in dynamic DDL (`testutil.NewIsolatedTestPool`)

**File:** `internal/testutil/postgres.go:113-154` (specifically lines 125 and 128)

**Issue:** `NewIsolatedTestPool(t *testing.T, schema string)` builds `DROP
SCHEMA`/`CREATE SCHEMA` statements via unescaped `fmt.Sprintf("DROP SCHEMA
IF EXISTS %s CASCADE", schema)` and `fmt.Sprintf("CREATE SCHEMA %s",
schema)`. Postgres identifiers cannot be parameterized as bind variables in
DDL, so this pattern is sometimes unavoidable — but it must quote/validate
the identifier rather than interpolate it raw. Every current call site
passes a hardcoded literal (`"notifier_test"`), so there is no live
exploit path today, but the function signature accepts an arbitrary `string`
with zero validation, and this is exactly the kind of helper that gets
reused later with a less-trusted input (e.g., a schema name derived from
`t.Name()` or an env var) without anyone re-auditing the DDL construction.
This is the same class of defect ASVS V5/SQL-injection guidance flags, just
scoped to a DDL identifier instead of a data value.

**Fix:**
```go
import "github.com/jackc/pgx/v5"

// ...
sanitized := pgx.Identifier{schema}.Sanitize()
if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", sanitized)); err != nil {
    t.Fatalf("testutil.NewIsolatedTestPool: drop schema %s (setup): %v", schema, err)
}
if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", sanitized)); err != nil {
    t.Fatalf("testutil.NewIsolatedTestPool: create schema %s: %v", schema, err)
}
```
At minimum, add a defensive `regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)`
guard on `schema` at the top of the function and `t.Fatal` if it doesn't
match, so any future caller passing a non-literal value fails loudly
instead of silently executing arbitrary SQL.

### WR-02: Baseline-advance-before-insert ordering introduces a permanent, unrecoverable event-loss window

**File:** `internal/detection/musicbrainz.go:278-404` (see the `Known,
accepted edge` doc comment above `detectDeluxeChanges`, and the `default:`
branch's insert call around line 371-399); `internal/detection/detector.go:135-152`

**Issue:** The former two-statement design (`groupBaseline` SELECT, then
`setGroupBaseline` UPDATE only *after* `InsertEvent` succeeded) meant a
failed insert left the baseline untouched, so the next poll cycle would
naturally retry the same comparison. The new single-statement CAS
(`AdvanceGroupTrackCountBaseline`) necessarily commits the new baseline
*before* the caller can know whether to fire an event — the CAS's own
return value is what decides that. If `InsertEvent` then fails (a
transient connection drop, a pool exhaustion moment, etc.), the baseline
has already durably advanced past the point that would trigger detecting
this change again: the *event row itself* is never created, and no future
cycle can ever recreate it, because the comparison that would have fired
it can never recur. This is different from the notifier's WR-03 window
(a row that already exists just fails to get marked `notified_at` — fully
recoverable next pass); here the underlying detected-event record is
permanently and silently lost from the database, not just undelivered.

This is called out explicitly in the code's own doc comment as an accepted
trade-off, and the failure path is logged at `Warn` (distinguishable from a
generic DB error) and the error is still propagated up through
`RunMusicBrainzCycle`'s per-artist error log — so it is *observable*, just
not *recoverable*. Given this project's stated core value ("reliably
detects and notifies on new releases"), a class of DB hiccup that
permanently drops a detected domain event (not merely delays its delivery)
deserves more than an accepted-risk comment, especially since the
underlying cause (no transaction-capable seam on `Detector`, which only
holds a `sqlc.Querier`) is a solvable architecture gap, not a fundamental
limitation.

**Fix:** Give `Detector` a transaction-capable seam (e.g. accept a
`*pgxpool.Pool` or a narrow `BeginTx`-only interface alongside
`sqlc.Querier`) so `AdvanceGroupTrackCountBaseline` and the subsequent
`InsertEvent` can be wrapped in one explicit `pgx.Tx`:
```go
tx, err := d.pool.Begin(ctx)
if err != nil { return err }
defer tx.Rollback(ctx) // no-op if committed
qtx := d.q.WithTx(tx)
rows, err := qtx.AdvanceGroupTrackCountBaseline(ctx, ...)
// ... decide whether to insert ...
if err := qtx.InsertEvent(ctx, ...); err != nil { return err } // tx never commits, baseline never advances
return tx.Commit(ctx)
```
This fully closes the window while preserving PERF-04's atomicity goal, at
the cost of the transaction-plumbing work the current doc comment declines
to justify. If that plumbing is deliberately deferred to a later phase,
downgrade this from an implicit "documented and fine" to an explicit
follow-up backlog item with a tracked issue reference, since the current
comment reads as a closed decision rather than an open risk.

### WR-03: `notifier` package-level test seams (`dbOpTimeout`, `spacingWait`) are only safe because no test currently calls `t.Parallel()`

**File:** `internal/notifier/notifier.go:56,67`; `internal/notifier/export_test.go:21-26`

**Issue:** `dbOpTimeout` and `spacingWait` are mutable package-level `var`s
that production code reads and that `notifier_test.go`/`export_test.go`
overwrite for the duration of a test via save-swap-restore
(`SetSpacingWaitForTest`). This works today because none of
`notifier_test.go`'s tests call `t.Parallel()` (confirmed: no matches in
the package). But this is a latent trap: this same phase's own stated goal
was fixing flakiness caused by tests running concurrently under Go's
default parallelism, and a future contributor adding `t.Parallel()` to
speed up this package's real-Postgres integration tests (a very natural
thing to do, since several of them spin up their own isolated schema and
have no reason not to run in parallel) would silently reintroduce a data
race on these two globals with no compiler or vet warning — `go test -race`
would catch it, but only if run, and CLAUDE.md doesn't currently mandate
`-race` in CI for this repo's test step.

**Fix:** Either (a) add a comment at the top of `notifier_test.go` stating
explicitly "no test in this file may call `t.Parallel()` without first
moving `dbOpTimeout`/`spacingWait` off package-level vars," or (b) do the
more robust fix now: thread `spacing`-style values through `Notifier`'s
own fields (already partially done for `spacing`) and make `dbOpTimeout`
an unexported field with a functional-option override for tests, removing
the shared mutable global entirely. Given this phase's whole premise is
closing parallelism-driven flakiness, leaving a new parallelism trap in
the same commit range is worth closing now rather than deferring.

## Info

### IN-01: `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` have no upper bound

**File:** `internal/config/config.go:54-55,79-84`

**Issue:** `Load()` rejects non-positive worker counts but places no
ceiling on either value. An operator typo (e.g. `MUSICBRAINZ_POLL_WORKERS=30000`
instead of `3`) would silently pass validation and allocate a
30000-capacity semaphore channel plus fan out that many goroutines per
cycle, which — combined with the tightly-limited MusicBrainz rate limiter
(default burst 1) — would mostly just mean 30000 goroutines parked in
`limiter.Wait`, but is still an unnecessary and unbounded resource
footprint for a config mistake that's easy to catch at boot.

**Fix:**
```go
const maxPollWorkers = 50 // generous ceiling; no realistic watchlist needs more

if cfg.MusicBrainzPollWorkers <= 0 || cfg.MusicBrainzPollWorkers > maxPollWorkers {
    return nil, fmt.Errorf("MUSICBRAINZ_POLL_WORKERS must be between 1 and %d, got %d", maxPollWorkers, cfg.MusicBrainzPollWorkers)
}
```
Apply the same pattern to `DeezerPollWorkers`.

### IN-02: `AdvanceGroupTrackCountBaseline`'s `:many` return shape is easy to misuse

**File:** `internal/db/sqlc/events.sql.go:60-78`; `internal/db/sqlc/querier.go:39`

**Issue:** The query is guaranteed by the `(event_type, source,
external_id)` unique constraint to return 0 or 1 rows, but sqlc generates
it as `:many` (`[]*int32`), so every caller must remember to check
`len(rows) == 0` and only ever look at `rows[0]`. `detector.go`'s
`advanceGroupBaseline` does this correctly today, but the generated
signature itself doesn't communicate the 0-or-1 invariant — a future
caller could easily write `rows[0]` without the length check and panic on
an index-out-of-range for the legitimate "no advance" case.

**Fix:** Change the query annotation to `:one` and have it return
`(pgtype.Timestamptz`-style nullable, i.e. accept `pgx.ErrNoRows` as the
"missing row" signal, translating it explicitly in Go — mirrors how
`UpdateWatchlistPreferences`'s doc comment already describes translating
`pgx.ErrNoRows` to `ErrNotFound`. This isn't required for correctness (the
current code is correct), but it would make misuse a compile-time-adjacent
error (a single `Scan` failing loudly) instead of a runtime panic risk for
the next caller.

---

_Reviewed: 2026-08-17T06:24:47Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
