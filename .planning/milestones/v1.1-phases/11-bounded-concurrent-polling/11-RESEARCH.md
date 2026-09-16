# Phase 11: Bounded Concurrent Polling - Research

**Researched:** 2026-08-16
**Domain:** Go concurrency (bounded worker pools), Postgres atomic compare-and-set, `go test` cross-package isolation
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Independent per-source env vars, not one shared pool-size config — mirrors the codebase's existing pattern of fully independent per-source rate limiters (`mbLimiter`/`dzLimiter`) and overlap guards (`mbRunning`/`dzRunning`); MusicBrainz's 1 req/sec limit and Deezer's ~10 req/sec limit behave very differently under the same pool size.
- **D-02:** Default pool sizes: MusicBrainz = 3, Deezer = 5. MusicBrainz's tight 1 req/sec limiter means extra MB workers mostly queue on the limiter — a smaller default avoids piling up idle goroutines; Deezer's faster limit benefits more from the larger default. Both stay within REQUIREMENTS.md PERF-01's stated 3-5 range.
- **D-03:** Env var names: `MUSICBRAINZ_POLL_WORKERS` and `DEEZER_POLL_WORKERS` — named to read as gating poll-cycle concurrency specifically, distinct from the existing `*_RATE_LIMIT_*` vars. Follow the existing `caarlos0/env` struct-tag pattern in `internal/config/config.go` (`env:"..." envDefault:"..."`).
- **D-04:** Add a `duration_ms` field to the cycle-end log line (alongside the existing per-artist `poll result` lines and the notifier-drain call at the end of each cycle) so a real deployment's logs demonstrate the speedup from success criterion 1, not just a one-time verification-time test.
- **D-05:** That same cycle-end log line also carries `artist_count` (how many watchlist entries this cycle polled) — makes throughput (artists/sec) derivable directly from logs without cross-referencing watchlist size separately.
- **D-06:** The deluxe-change baseline read-then-write (`detector.groupBaseline` SELECT + `detector.setGroupBaseline` UPDATE in `internal/detection/detector.go`) becomes a single atomic `UPDATE ... RETURNING` statement — the row-level lock Postgres takes during the UPDATE serializes any two concurrent callers racing on the same `external_id`, closing the check-then-act window entirely rather than narrowing it. — **Reversibility:** costly — replaces two separate sqlc queries (`GroupTrackCountBaseline`, `SetGroupTrackCountBaseline`) with one combined query; downstream detection code (`detectDeluxeChanges`'s establish-vs-advance branching) needs to be re-derived from the single statement's `RETURNING` result instead of two sequential Go-level reads, so reverting means re-splitting the query and re-deriving the two-step Go logic.
  - The statement must still preserve today's two distinct outcomes: a group with no baseline yet silently establishes one (no event fires), while a group with an existing lower baseline both fires a `deluxe_change` event and advances the baseline. The atomic UPDATE's `RETURNING` clause needs to give the Go caller enough information (e.g. the previous value, or a NULL-vs-non-NULL distinction) to keep telling those two cases apart.
- **D-07:** No ordering/grouping mechanism needed for per-artist log lines under concurrency — interleaved-but-labeled output is acceptable. Every `poll result` / `poll artist failed` line already carries `cycle_id`, `source`, `artist_mbid`, and `artist_name`, which is enough to reconstruct per-cycle, per-artist context regardless of emission order. No buffering, no `worker_id` field, no output reordering.
- **Folded todo:** Fix flaky tests under parallel `go test ./...` (shared-DB contention + notifier timing) — folded into Phase 11's scope. Four `internal/notifier` tests flaking on real-time sleep/spacing assertions; one `internal/poller` test flaking on a shared-DB schema-visibility race. Solution approach (clock injection vs. per-package DB isolation vs. `-p 1` pinning vs. accept-as-known-flake) left to research/planning — this research resolves it (see Common Pitfalls 6 and 7).

### Claude's Discretion

- The exact worker-pool implementation primitive (raw goroutines + `sync.WaitGroup` + buffered channel/semaphore, mirroring `internal/httpserver/search.go`'s existing concurrent-fan-out pattern, vs. `golang.org/x/sync/errgroup`) is left to research/planning. Whichever is chosen must preserve PERF-03's per-artist error isolation — a worker's own error must be logged and that artist skipped, never propagated in a way that cancels sibling workers still in flight (this rules out naive `errgroup.WithContext` usage where a worker returns its error directly, since that cancels the shared context for all other in-flight workers).
- The exact `RETURNING`-clause shape and whether a companion `HasBaseline`-equivalent flag is still needed post-atomic-UPDATE (D-06) is left to research/planning, constrained by the outcome-preservation note above.
- Whether the worker pool is implemented as one pool shared by MusicBrainz-cycle and Deezer-cycle machinery, or two independently-instantiated pools (mirroring D-01's independent-config decision), is left to research/planning — D-01 only locks the *configuration* surface as independent, not necessarily two separate runtime pool objects, though independent pools is the natural implementation given D-01.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope. The one candidate scope-adjacent item (flaky test suite fix) was explicitly folded in rather than deferred (see Folded Todo above under Locked Decisions).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-------------------|
| PERF-01 | Per-source polling (MusicBrainz, Deezer) uses a bounded, env-configurable concurrent worker pool (default 3-5 workers) instead of strictly sequential per-artist iteration | Standard Stack (Pattern 1, buffered-channel semaphore) + Code Examples (config additions) + Validation Architecture (concurrency-bound test replacing `TestMusicBrainzCycle_Sequential`) |
| PERF-02 | Concurrent polling preserves the existing per-source rate limiter and per-source cycle-overlap guard | Don't Hand-Roll (`rate.Limiter` already concurrency-safe, no change needed) + Common Pitfall 5 (empirical burst verification) + existing overlap-guard tests confirmed unaffected |
| PERF-03 | A single artist's polling error does not abort the rest of that cycle's batch (errors are logged and skipped, not fatal) | Pattern 1 (worker never propagates its error) + Common Pitfall 3 (`errgroup.WithContext` rejected) + Alternatives Considered table |
| PERF-04 | Concurrent updates to a shared release-group's deluxe-change baseline cannot lose an update (baseline compare-and-set is atomic at the database level) | Architecture Pattern 2 (`AdvanceGroupTrackCountBaseline` CTE + `UPDATE...RETURNING`), Code Examples (full SQL + Go wrapper), Common Pitfall 1 (crash-window follow-on risk), Validation Architecture (`TestAdvanceGroupBaseline_ConcurrentRace`) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

Directives from `.claude/CLAUDE.md` (project root's checked-in CLAUDE.md, per `.planning/config.json`'s `claude_md_path`) that constrain this phase's implementation:

- **DB access via sqlc only, never raw SQL in Go application code.** The new `AdvanceGroupTrackCountBaseline` statement must be added to `queries/events.sql` and go through `sqlc generate`, exactly like every other query in this codebase — no `pool.Exec`/`pool.QueryRow` with an inline string in `internal/detection/detector.go`.
- **`pgx/v5` only, never `lib/pq`.** Not directly at risk in this phase (no new driver code), but any transaction-wiring work considered for Pitfall 1 must use `pgx.Tx` (already what `sqlc`'s generated `WithTx` expects), not `database/sql`'s transaction type.
- **`caarlos0/env/v11` stays env-var-only, no numeric-minimum struct tag exists.** The new `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` fields need the same manual post-`env.Parse` validation pattern `config.Load` already uses for `EVENT_RETENTION_DAYS` (reject non-positive values) — the library itself cannot enforce this.
- **Unit tests must mock MusicBrainz/Deezer via `httptest.Server`, no live external calls in CI.** This phase's new concurrency tests operate entirely on the existing `ReleaseGroupSource`/`AlbumSource`/`EventRecorder` fakes in `poller_test.go` (already concurrency-safe, see Common Pitfall 2) — no new live-HTTP test surface is introduced.
- **All secrets/config via environment variables only, nothing committed.** The two new env vars carry no secret material (plain integers) — no `.env.example` parity risk beyond adding the two new keys with their documented defaults (existing project convention, enforced by a reflection-based parity test per PROJECT.md Key Decisions).
- **`golangci-lint` v2 config schema, `go vet` as a fast-fail step before it.** Unaffected by this phase directly, but any new `//nolint:gosec` comment added for an `int32` truncation in the new query wrapper (mirroring `setGroupBaseline`'s existing identical comment) must carry the same justification style already established in `internal/detection/detector.go`/`musicbrainz.go`.

## Summary

This phase turns two sequential per-artist loops (`RunMusicBrainzCycle`, `RunDeezerCycle` in `internal/poller/poller.go`) into bounded-concurrency loops, without touching either source's existing rate limiter or overlap guard, and without losing the deluxe-change baseline's correctness when two artists sharing a release-group race each other. The codebase already has exactly one precedent for concurrent fan-out (`internal/httpserver/search.go`'s `handleSearch`: `sync.WaitGroup` + closures + a mutex-guarded shared map), but that precedent fans out to a small, *fixed* number of sources with no pool-size bound — this phase needs an actual bound, which search.go's pattern doesn't provide as-is.

The highest-risk part of this phase is not the worker pool itself (a buffered-channel semaphore or `errgroup.Group.SetLimit` both trivially bound concurrency) — it's `internal/detection/detector.go`'s baseline read (`groupBaseline`) + write (`setGroupBaseline`), which today is a classic check-then-act race. Two workers processing different artists that both credit the same release-group can both read the old baseline before either writes, and the *second* writer can silently clobber the first writer's correct, higher value with its own (now-stale) lower one — a genuine lost update, not just a `go test -race` data race (both statements are already individually safe SQL statements; the race is at the Go-level two-statement sequence). The fix is a single atomic `UPDATE ... RETURNING` statement, built from a locking CTE, that reads the current value and conditionally writes the new one in one round trip under one row lock — closing the window entirely rather than narrowing it.

A second, unplanned finding: this phase's own scope (concurrent poller tests, `-race`-clean suite) is the natural place to fix the pre-existing `internal/poller`/`internal/notifier` flaky-test todo folded in by CONTEXT.md. Reading `internal/db/migrate_test.go` directly (not just the todo's paraphrase) locates the actual root cause: `TestRunMigrations_AppliesFromScratch` issues a raw `DROP SCHEMA public CASCADE` against the shared `TEST_DATABASE_URL` Postgres instance, on a bare `sql.Open` connection that bypasses golang-migrate's own advisory-lock serialization entirely. Under default `go test ./...` package-level parallelism, this drops tables out from under any other package's concurrently-running integration test — which is exactly the `relation "artists" does not exist` failure the todo describes.

**Primary recommendation:** Use a fixed-size buffered channel as a semaphore, paired with `sync.WaitGroup`, freshly created per cycle invocation (not a persistent pool) — this is the minimal extension of the existing `search.go` house style that adds a size bound, requires zero new dependencies, and completely avoids `errgroup.WithContext`'s cancel-on-first-error footgun the CONTEXT.md explicitly calls out. Pair it with a single atomic `UPDATE ... RETURNING` (via a `FOR UPDATE`-locking CTE) replacing `groupBaseline`+`setGroupBaseline`, and fix the flaky-test todo by moving `TestRunMigrations_AppliesFromScratch`'s schema-drop off the shared integration DSN.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Bounded worker-pool concurrency for poll cycles | Backend/Service (`internal/poller`) | — | Scheduling of concurrent fetches is a poller-internal concern; no other tier is involved |
| Per-source rate limiting under concurrency | Backend/API-client (`internal/musicbrainz`, `internal/deezer`, via `golang.org/x/time/rate.Limiter`) | — | Already tier-correct and already concurrency-safe (`rate.Limiter` is documented safe for concurrent callers) — no change needed |
| Per-source cycle-overlap guard | Backend/Service (`internal/poller`, `mbRunning`/`dzRunning` atomics) | — | Unchanged by this phase; concurrency only changes what happens *inside* one already-guarded cycle |
| Deluxe-change baseline compare-and-set | Database/Storage (Postgres, `events.track_count`) | Backend/Service (`internal/detection`) | The atomicity guarantee this phase needs (PERF-04) can only be provided by the database's own row-level locking; Go-level mutexes cannot serialize two separate process instances or even two goroutines racing across two round trips without one |
| Per-artist error isolation | Backend/Service (`internal/poller`, worker closure) | — | Must stay a per-worker concern; must never leak into a shared cancellation signal (context or errgroup) that would abort sibling workers |
| Worker-pool-size config surface | Backend/Config (`internal/config`) | — | New `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` env vars, same tier as the existing `*_RATE_LIMIT_*` vars |
| Cycle-duration/throughput observability | Backend/Service (`internal/poller`, structured log line) | Operations (log aggregation) | `duration_ms`/`artist_count` are computed and emitted poller-side; consumption is an ops-tier concern out of this phase's scope |
| `go test` cross-package DB isolation (flaky-test fix) | Testing/CI (`internal/db`, `internal/testutil`) | — | The flake is a raw-SQL statement in one package's test racing another package's live queries against a shared fixture DB — a test-infrastructure concern, not a production-code concern |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `sync` (`WaitGroup`) + a buffered `chan struct{}` semaphore | Go 1.26 stdlib [VERIFIED: go.mod:3 — `go 1.26`] | Bounded concurrent fan-out primitive for both `RunMusicBrainzCycle` and `RunDeezerCycle` | Zero new dependencies; a direct, minimal extension of `internal/httpserver/search.go`'s existing `sync.WaitGroup`-based fan-out pattern (search.go lines 174–204, [VERIFIED: internal/httpserver/search.go:174-204]) — that pattern already establishes this codebase's house style for goroutine lifecycle (`wg.Add`/`defer wg.Done()`/`wg.Wait()` join point), it just fans out to a small fixed set with no bound; adding a buffered-channel semaphore is the smallest change that adds the missing bound |
| `golang.org/x/time/rate` (`*rate.Limiter`) | v0.15.0 [VERIFIED: go.mod:13] | Per-source outbound rate ceiling, unchanged by this phase | Already locked project-wide (CLAUDE.md); `Limiter.Wait`/`Reserve`/`Allow` are documented safe for concurrent use by multiple goroutines [CITED: pkg.go.dev/golang.org/x/time/rate] — concurrency in the poller does not require touching this package at all, it "just works" as more callers queue on the same shared bucket |
| `github.com/caarlos0/env/v11` | v11.4.1 [VERIFIED: go.mod:6] | New `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` config fields | Already locked project-wide; follow the existing `env:"..." envDefault:"..."` struct-tag pattern already used for `MusicBrainzRateLimitPerSec`/`DeezerRateLimitPer5s` [VERIFIED: internal/config/config.go:34-35 — `MusicBrainzRateLimitPerSec float64 \`env:"MUSICBRAINZ_RATE_LIMIT_PER_SEC" envDefault:"1"\`` / `DeezerRateLimitPer5s int \`env:"DEEZER_RATE_LIMIT_PER_5S" envDefault:"50"\``] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/sync/errgroup` (`errgroup.Group`, zero-value + `SetLimit`, **not** `WithContext`) | v0.21.0, already present as an **indirect** dependency [VERIFIED: go.mod:20 — `golang.org/x/sync v0.21.0 // indirect`] | Accepted alternative to the buffered-channel semaphore if the planner prefers `SetLimit`'s built-in bound over hand-rolling one | Only if using the **zero-value** `errgroup.Group{}` (no `errgroup.WithContext` call) — `errgroup.WithContext`'s derived context is cancelled on the *first* non-nil error any worker returns to the group, which is exactly the naive-usage footgun CONTEXT.md's Claude's-Discretion section calls out as forbidden (it would abort sibling in-flight workers, violating PERF-03) [CITED: pkg.go.dev/golang.org/x/sync/errgroup — WithContext's context "is canceled the first time a function passed to Go returns a non-nil error"]. If chosen, every worker closure passed to `g.Go` must swallow its own error (log it, return `nil`) exactly as the sequential loop's `continue` does today — never return the real per-artist error into the group. Promote from `// indirect` to a direct `require` in `go.mod` via `go mod tidy` once imported. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Buffered-channel semaphore + `sync.WaitGroup` (recommended) | `golang.org/x/sync/errgroup` with `SetLimit`, zero-value (no `WithContext`) | `SetLimit` gives the bound "for free" without hand-rolling a channel, and is a well-known idiom — but it adds a dependency promotion and requires discipline (every worker must return `nil` to the group, never its real error) to avoid accidentally reaching for `WithContext` later. The channel-semaphore has zero such foot-guns because there is no shared error/context object to misuse. |
| `golang.org/x/sync/errgroup` with `WithContext` | — | **Rejected outright** per CONTEXT.md's explicit instruction — cancels the shared context on the first worker error, aborting sibling in-flight workers, which directly violates PERF-03. Not a legitimate option for this phase regardless of pool-size configuration. |
| One long-lived, persistent worker-pool object with pre-spawned goroutines and a job queue | Fresh semaphore/`errgroup` created per cycle invocation (recommended) | A persistent pool needs its own start/stop lifecycle wired into `Poller.Start`/`Stop`, and risks a goroutine leak or a stale-worker-count bug if `Stop` races an in-flight cycle. A cycle already re-lists the watchlist fresh every run (`entries, err := p.store.List(ctx)`) — creating a fresh bounded semaphore of the same lifetime (one per cycle call, discarded when the cycle returns) is simpler, needs no new struct fields beyond the configured pool-size ints, and cannot leak across cycles. |

**Installation:**
No new package install is required — `golang.org/x/sync` is already vendored (indirect, `go.sum` already contains it) and stdlib `sync`/`sync/atomic` need no install. If the planner picks the `errgroup` alternative, promote it to a direct dependency:
```bash
go get golang.org/x/sync@v0.21.0
go mod tidy
```

**Version verification:** `golang.org/x/time v0.15.0`, `golang.org/x/sync v0.21.0`, `github.com/caarlos0/env/v11 v11.4.1` all confirmed present and pinned in the project's own `go.mod` this session [VERIFIED: go.mod:1-22] — no registry lookup needed since nothing new is being added to the dependency graph, only an existing indirect dependency's possible promotion to direct.

## Package Legitimacy Audit

**Not applicable this phase.** No new external package is introduced — `golang.org/x/sync` is already present in `go.sum` as an indirect transitive dependency [VERIFIED: go.mod:20], and the only change under the errgroup alternative is a `go.mod` `require` block promotion (indirect → direct), not a new install. `golang.org/x/sync` and `golang.org/x/time` are both first-party Go extended-standard-library modules published under `golang.org/x/*` by the Go team itself — the highest-trust tier of the Go ecosystem, not subject to the slopsquatting risk this gate exists to catch.

## Architecture Patterns

### System Architecture Diagram

```
                    cron tick (per source, independent schedules)
                              |
                              v
                  overlap guard: mbRunning/dzRunning
                  CompareAndSwap(false, true)  --skip if already running-->  ErrCycleInProgress (unchanged)
                              |
                              v
                  cycleStart := time.Now(); entries := store.List(ctx)
                              |
                              v
              +----------------------------------------------+
              |  bounded fan-out (NEW this phase)             |
              |  sem := make(chan struct{}, poolSize)         |
              |  for each entry: wg.Add(1); sem <- struct{}{} |
              |    go func(entry) {                           |
              |      defer wg.Done(); defer <-sem              |
              |      groups, err := source.Fetch(ctx, entry)  |  <-- rate.Limiter.Wait() gates
              |      [on err: log + return, no propagation]   |      each call regardless of
              |      events.Detect*(ctx, logger, entry, ...)  |      concurrent caller count
              |      [on err: log + return, no propagation]   |
              |    }(entry)                                   |
              |  wg.Wait()  <-- join point                     |
              +----------------------------------------------+
                              |
                              v
              logger.Info("poll cycle complete",
                artist_count=len(entries),
                duration_ms=time.Since(cycleStart))
                              |
                              v
                  notifier.NotifyPending(ctx, logger)   (unchanged, still after the join)
                              |
                              v
                  defer mbRunning/dzRunning.Store(false)


   Inside events.Detect* (only the deluxe-change path changes):

   detectDeluxeChanges, per release-group already in preCycleSeen:
       maxCount := max(TrackCount() across fetched releases)
       if maxCount == 0: skip (unchanged)
       advanced, previousBaseline, hadBaseline, err :=
           detector.advanceGroupBaseline(ctx, groupMBID, maxCount)  <-- NEW: one atomic
                                                                          UPDATE...RETURNING
                                                                          statement (was 2
                                                                          separate queries)
       switch {
       case !advanced:            // fresh count not an increase -- no-op (unchanged outcome)
       case advanced && !hadBaseline:  // silently established -- no event (unchanged outcome)
       case advanced && hadBaseline:   // fire deluxe_change event with previousTrackCount
                                        // = previousBaseline (unchanged outcome)
       }
```

A reader can trace the primary use case (watchlist entry in, event row + notification out) by following the fan-out box top-to-bottom; the only new decision point this phase adds is the single atomic baseline statement inside the detection seam, which is a drop-in replacement for the two-statement read/write that already existed there.

### Recommended Project Structure

No new files or directories — this phase edits existing files in place:
```
internal/poller/poller.go        # RunMusicBrainzCycle/RunDeezerCycle: sequential loop -> bounded fan-out
internal/poller/poller_test.go   # existing fakes already concurrency-safe (see Common Pitfalls); add new pool-size/race tests
internal/detection/detector.go   # groupBaseline+setGroupBaseline -> single advanceGroupBaseline
internal/detection/musicbrainz.go # detectDeluxeChanges: re-derive establish-vs-advance branching from advanceGroupBaseline's return values
internal/config/config.go        # + MUSICBRAINZ_POLL_WORKERS, DEEZER_POLL_WORKERS
queries/events.sql               # GroupTrackCountBaseline + SetGroupTrackCountBaseline -> AdvanceGroupTrackCountBaseline
internal/db/sqlc/events.sql.go   # regenerated via `sqlc generate`
internal/db/migrate_test.go      # flaky-test fix: TestRunMigrations_AppliesFromScratch's schema-drop moved off the shared fixture DSN
internal/notifier/notifier_test.go # flaky-test fix: clock injection for the 4 timing-sensitive tests
cmd/server/main.go               # poller.New gains two worker-count args, threaded from cfg
```

### Pattern 1: Bounded concurrent fan-out with per-worker error isolation

**What:** A buffered channel of size N used purely as a counting semaphore (acquire = send, release = receive), combined with a `sync.WaitGroup` for the join, created fresh at the top of each cycle method.

**When to use:** Any time a fixed-size batch of independent, fallible operations needs a concurrency ceiling and per-item error isolation, with no shared cancellation signal.

**Example:**
```go
// Source: this project's own internal/httpserver/search.go (lines 174-204,
// [VERIFIED: internal/httpserver/search.go:174-204]) extended with a
// buffered-channel bound -- search.go's own fan-out has no size cap because
// it always fans out to exactly len(s.sources) goroutines, which this
// pattern's watchlist-sized fan-out cannot assume.
sem := make(chan struct{}, poolSize)
var wg sync.WaitGroup
for _, entry := range entries {
    if err := ctx.Err(); err != nil {
        return err // preserves today's early-exit-on-cancellation behavior
    }
    wg.Add(1)
    sem <- struct{}{} // blocks here once poolSize workers are in flight
    go func(entry watchlist.Entry) {
        defer wg.Done()
        defer func() { <-sem }()

        groups, err := p.mb.ReleaseGroupsByArtist(ctx, entry.MBID)
        if err != nil {
            logger.Error("poll artist failed", /* ... */)
            return // NEVER propagate -- mirrors today's `continue`
        }
        logger.Info("poll result", /* ... */)

        if err := p.events.DetectMusicBrainz(ctx, logger, entry, groups); err != nil {
            logger.Error("detection failed", /* ... */)
            return
        }
    }(entry)
}
wg.Wait()
```

### Pattern 2: Atomic compare-and-set via a locking CTE + `UPDATE ... RETURNING`

**What:** One SQL statement that reads a row's current value under `FOR UPDATE` (via a CTE), then conditionally updates it, returning the pre-update value so the caller can still distinguish "no baseline yet" from "baseline existed and was lower" — all under one row lock instead of two round trips.

**When to use:** Any read-then-conditionally-write sequence against a single row that must survive concurrent callers racing the same key — the general Postgres idiom for "atomic compare-and-set with old-value retrieval," confirmed via multiple independent sources (PostgreSQL's own `UPDATE`/`WITH` documentation plus community CTE-locking-pattern writeups) [CITED: postgresql.org/docs/current/sql-update.html; corroborating community sources on CTE `FOR UPDATE` + `UPDATE ... FROM` patterns].

**Example (new query, `queries/events.sql`):**
```sql
-- name: AdvanceGroupTrackCountBaseline :many
-- Atomic replacement for the former two-statement GroupTrackCountBaseline
-- SELECT + SetGroupTrackCountBaseline UPDATE (PERF-04). The CTE's FOR UPDATE
-- takes a row lock on the group's own new_release row; a second concurrent
-- caller racing the same external_id blocks on that lock until the first
-- transaction commits, then re-evaluates against the just-committed value --
-- this is what closes the check-then-act window entirely rather than just
-- narrowing it. Zero rows returned means no advance happened: the fresh
-- count was not an increase over the already-committed baseline (D-02's
-- "equal or lower: no event" case, now enforced by Postgres itself). One row
-- returned means the write landed; previous_track_count NULL vs non-NULL is
-- the has_baseline distinction GroupTrackCountBaseline used to report,
-- letting the caller keep branching on "silently established" vs "advanced,
-- fire an event" from this one call's result.
WITH current AS (
    SELECT track_count FROM events
    WHERE event_type = 'new_release' AND source = 'musicbrainz' AND external_id = $1
    FOR UPDATE
)
UPDATE events e
SET track_count = $2
FROM current
WHERE e.event_type = 'new_release' AND e.source = 'musicbrainz' AND e.external_id = $1
  AND (current.track_count IS NULL OR $2::int > current.track_count)
RETURNING current.track_count AS previous_track_count;
```

**Go wrapper (`internal/detection/detector.go`):**
```go
// advanceGroupBaseline atomically compares count against groupMBID's
// currently-committed baseline and writes it as the new baseline only if
// it's a genuine increase (or no baseline exists yet) -- replacing the
// former groupBaseline (SELECT) + setGroupBaseline (UPDATE) pair, whose
// two-round-trip gap PERF-04 exists to close.
func (d *Detector) advanceGroupBaseline(ctx context.Context, groupMBID string, count int) (advanced, hadBaseline bool, previousBaseline int, err error) {
    trackCount := int32(count) //nolint:gosec // see setGroupBaseline's existing identical justification
    rows, err := d.q.AdvanceGroupTrackCountBaseline(ctx, sqlc.AdvanceGroupTrackCountBaselineParams{
        ExternalID: groupMBID,
        TrackCount: &trackCount,
    })
    if err != nil {
        return false, false, 0, fmt.Errorf("detection: advance group baseline: %w", err)
    }
    if len(rows) == 0 {
        return false, false, 0, nil
    }
    row := rows[0]
    if row.PreviousTrackCount != nil {
        return true, true, int(*row.PreviousTrackCount), nil
    }
    return true, false, 0, nil
}
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bounded concurrency limiting | A custom worker-pool struct with pre-spawned goroutines, a job channel, and its own `Start`/`Stop` lifecycle | A buffered channel used as a semaphore, created fresh per cycle call (Pattern 1 above) | A persistent pool needs lifecycle wiring into `Poller.Start`/`Stop` and risks leaking goroutines or racing `Stop` against an in-flight cycle; a per-cycle semaphore has none of that because it's garbage the moment `wg.Wait()` returns |
| Compare-and-set for the baseline | An application-level `sync.Mutex`/`sync.Map` keyed by release-group MBID inside `Detector` | A single Postgres `UPDATE ... RETURNING` statement (Pattern 2 above) | An in-process mutex only serializes goroutines *within one process* — it does nothing for two separate `drop-tracker` instances (a future horizontal-scale scenario the project's own README/config already anticipates guarding against via Postgres, per PROJECT.md's "advisory-lock" note on `robfig/cron`), and it adds Go-level lock-ordering/deadlock risk for zero benefit over a database that already does row-level locking correctly |
| Rate limiting under concurrency | A wrapper mutex/queue around the existing `*rate.Limiter` to "make it safe for concurrent callers" | The existing `*rate.Limiter` unchanged | Already documented safe for concurrent use [CITED: pkg.go.dev/golang.org/x/time/rate] — wrapping it in anything would only add latency and a new place for a bug, for a property the library already guarantees |

**Key insight:** This phase's actual complexity is entirely in the *database* statement (Pattern 2), not the Go concurrency primitive (Pattern 1) — the worker pool itself is a well-trodden, boring pattern; the baseline CAS is the one piece of state that genuinely needs a new mechanism, and Postgres already has the right primitive (`FOR UPDATE` row locking) for it.

## Runtime State Inventory

Not applicable — this is a concurrency/correctness refactor of existing in-process and in-database logic, not a rename, rebrand, or migration. No stored data, live service config, OS-registered state, secrets, or build artifacts carry a name or identifier this phase changes. The one schema change (`AdvanceGroupTrackCountBaseline` replacing two sqlc queries) is a query-definition change, not a column/table rename — `events.track_count` itself is untouched.

## Common Pitfalls

### Pitfall 1: Reordering the baseline-advance and the event-insert can silently swallow a notification

**What goes wrong:** The current sequential code inserts the `deluxe_change` event row *first*, then advances the baseline second (`internal/detection/musicbrainz.go:353-375` [VERIFIED: internal/detection/musicbrainz.go:353-375 — `insertEvent(...)` at line 353, then `d.setGroupBaseline(ctx, g.MBID, maxCount)` at line 373]). If the new atomic `advanceGroupBaseline` call is placed *before* the event insert (the natural order once the two are collapsed into one CAS call followed by a conditional `insertEvent`), a process crash between the two leaves the baseline already advanced but the event never recorded. The *next* cycle's `detectDeluxeChanges` will then compute the same (or lower) `maxCount` against the *already-advanced* baseline, find it's not an increase, and silently do nothing — the deluxe-change notification is permanently lost, not just delayed.

**Why it happens:** The old ordering was accidentally crash-safe: a crash after the event insert but before the baseline write just means the baseline write retries harmlessly next cycle (idempotent value), and `insertEvent`'s `ON CONFLICT DO NOTHING` makes a duplicate event-insert attempt a no-op too. Collapsing the read-then-write into one atomic statement and running it *first* removes that accidental safety net.

**How to avoid:** Either (a) wrap the `AdvanceGroupTrackCountBaseline` call and the subsequent `InsertEvent` call in a single explicit Postgres transaction (`pool.Begin(ctx)` → `sqlc.New(pool).WithTx(tx)` → ... → `tx.Commit(ctx)`) so both commit or neither does — this is new machinery for the codebase (no production code currently opens an explicit transaction; only `internal/db/sqlc/db.go`'s generated `WithTx` method exists today [VERIFIED: internal/db/sqlc/db.go:20,28 — `func New(db DBTX) *Queries` / `func (q *Queries) WithTx(tx pgx.Tx) *Queries`]), or (b) accept and explicitly document the residual crash window as a known, accepted edge case — mirroring this codebase's own existing precedent for documenting an accepted edge rather than closing it (`isSeedMode`'s doc comment in `internal/detection/detector.go:78-83` [VERIFIED: internal/detection/detector.go:78-83]). Flag this decision explicitly for the planner; PERF-04's literal text ("baseline compare-and-set is atomic") is satisfied either way — this pitfall is about a *different*, adjacent regression risk the redesign could introduce if not considered.

**Warning signs:** A test that kills the process (or simulates a crash) between the CAS call and the event insert, then re-runs `detectDeluxeChanges` and asserts an event *was* eventually recorded, would catch this; its absence from the plan is itself a warning sign.

### Pitfall 2: Existing poller test doubles already track concurrency, but one existing test hard-asserts sequential-only behavior

**What goes wrong:** `internal/poller/poller_test.go`'s `fakeReleaseGroupSource` and `fakeAlbumSource` already track `inFlight`/`maxInFlight` via `atomic.Int32` and a mutex-guarded slice [VERIFIED: internal/poller/poller_test.go:133-166, 242-276] — they are *already* concurrency-safe and need no changes for this phase. However, `TestMusicBrainzCycle_Sequential` explicitly asserts `mb.maxInFlight <= 1` with the comment "artists must be polled one at a time (D-07)" [VERIFIED: internal/poller/poller_test.go:337-350]. This test will fail (correctly) the moment concurrency lands, and its intent is now backwards.

**Why it happens:** The test was written to lock in *last* phase's sequential-only guarantee; this phase deliberately reverses that guarantee.

**How to avoid:** Replace `TestMusicBrainzCycle_Sequential` (and its likely Deezer-side twin, if one exists further in the file) with a test asserting `maxInFlight` reaches up to `min(poolSize, len(entries))` — using the exact same fake and its existing `inFlight`/`maxInFlight` fields, no new test infrastructure needed. Grep the whole test file for other hard sequential-only assertions before treating this as the only one.

**Warning signs:** Any lingering test whose name or comment says "sequential" or "one at a time" after this phase lands is either stale (needs updating/removal) or is now testing something else entirely (needs renaming so its intent is clear).

### Pitfall 3: `errgroup.WithContext` cancels sibling workers on the first error — confirmed, not hypothetical

**What goes wrong:** If the planner or an implementer reaches for `errgroup.WithContext` "because it's the idiomatic Go concurrency helper," the derived context is cancelled the moment the *first* worker returns a non-nil error [CITED: pkg.go.dev/golang.org/x/sync/errgroup]. Every other worker still reading `ctx` downstream (rate limiter waits, HTTP requests, DB calls) would then observe cancellation and abort — turning one artist's transient failure into a whole-cycle abort, directly violating PERF-03.

**Why it happens:** `errgroup.WithContext` is the most commonly copy-pasted errgroup usage in tutorials/blog posts precisely *because* fail-fast cancellation is usually desirable — it is the wrong default for this specific use case (independent, isolatable per-item work), not a general anti-pattern.

**How to avoid:** Use the buffered-channel semaphore (Pattern 1) which has no shared context/error object at all, or the zero-value `errgroup.Group{}` with `SetLimit` and **never call `WithContext`**, with every worker closure always returning `nil` to the group regardless of its internal (logged) error.

**Warning signs:** Any `g, ctx := errgroup.WithContext(ctx)` in the diff is an immediate correctness red flag for this phase specifically.

### Pitfall 4: The pgxpool's default `MaxConns` can become the new bottleneck once both cycles run concurrent workers

**What goes wrong:** `internal/db/pool.go`'s `PoolConfig`/`NewPool` never sets `pgxpool.Config.MaxConns` explicitly [VERIFIED: internal/db/pool.go:61-99 — no `cfg.MaxConns` assignment anywhere in `PoolConfig`], so it defaults to `max(4, runtime.NumCPU())` [CITED: pkg.go.dev/github.com/jackc/pgx/v5/pgxpool]. With MusicBrainz's default pool size of 3 and Deezer's default of 5 (D-02), and both cycles able to run concurrently (they are two independent cron entries with independent overlap guards — nothing stops a MusicBrainz cycle and a Deezer cycle overlapping each other), up to 8 goroutines can be simultaneously calling into the shared `Detector`'s `sqlc.Queries` (itself backed by one shared `*pgxpool.Pool`) at once. On a small/CI machine where `runtime.NumCPU()` is low (e.g. a 2-4 vCPU GitHub Actions runner), the connection pool itself — not the worker-pool size or the rate limiter — could become the actual throughput ceiling, undermining Criterion 1's "measurably faster" claim without anyone noticing why.

**Why it happens:** The pool size was never tuned for concurrent detection workloads because none existed before this phase.

**How to avoid:** During verification (not just implementation), measure actual cycle wall-time with realistic worker-pool sizes and note whether the DB pool saturates (pgxpool exposes `pool.Stat()` with `AcquireCount`/`EmptyAcquireCount`/`AcquireDuration` for exactly this check). If it does, either raise `MaxConns` explicitly in `PoolConfig`, or document the finding — the ROADMAP.md Notes line already anticipates this exact risk ("confirm the DB pool has not become the new bottleneck").

**Warning signs:** Cycle duration doesn't meaningfully improve as `*_POLL_WORKERS` increases past a small number, or `pool.Stat().EmptyAcquireCount` is non-zero during a load test.

### Pitfall 5: `rate.Limiter` has a documented (rare, high-throughput-only) concurrent-goroutine burst edge case

**What goes wrong:** A reported upstream issue shows `golang.org/x/time/rate.Limiter.Reserve` can, under many goroutines calling it in tight succession at very high request rates (~200k req/s in the reported case), receive a timestamp that is momentarily behind the limiter's internal `last` clock due to `time.Now()` call interleaving, causing a brief over-admission above the configured rate [CITED: github.com/golang/go/issues/65508].

**Why it happens:** The bug requires extremely high call frequency to expose — the timestamp race window is on the order of the scheduling jitter between two near-simultaneous `time.Now()` calls, which only matters when the configured rate itself is high enough that this jitter is a meaningful fraction of one token's time budget.

**How to avoid:** This project's rates (MusicBrainz ~1/sec, Deezer ~10/sec) are many orders of magnitude below the regime where this has been observed to matter, and the worker-pool sizes (3-5) are far below the goroutine counts in the reported case — treat this as a documented, LOW-real-world-risk edge case for this specific traffic profile, not a blocker. Still, verify Criterion 2 empirically: assert on real captured request timestamps under concurrent load (not just "the library is documented safe"), since that is the only way to be certain for this project's own configuration rather than trusting a general claim.

**Warning signs:** None expected in this project's traffic profile; include this only so the planner doesn't need to independently discover and worry about it.

### Pitfall 6: The flaky-test root cause is a raw schema-drop, not a golang-migrate race

**What goes wrong:** The folded-in todo describes the `internal/poller` flake as a "shared-DB schema-visibility race" without a confirmed root cause. It is tempting to "fix" this by adding locking or retry logic around `db.RunMigrations` calls — but `golang-migrate`'s Postgres driver already serializes concurrent `Up()` calls via `pg_advisory_lock` (which blocks until free, it does not error) [CITED: github.com/golang-migrate/migrate — postgres driver source/issue discussion], so two packages both calling `RunMigrations` concurrently were never the actual race.

**Why it happens:** `internal/db/migrate_test.go`'s `TestRunMigrations_AppliesFromScratch` opens its own **bare** `sql.Open("pgx", dsn)` connection and issues `DROP SCHEMA public CASCADE; CREATE SCHEMA public` directly [VERIFIED: internal/db/migrate_test.go:126-145 — `sqlDB, err := sql.Open("pgx", dsn)` at line 138, `sqlDB.ExecContext(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")` at line 143], entirely outside golang-migrate's own locking mechanism (which only guards `migrate.Up`/`migrate.Down` calls, not arbitrary raw SQL a test chooses to run). This statement drops every table — including `artists`, which `internal/poller`'s integration tests query — with nothing serializing it against any other package's concurrently-running `go test` binary process querying the same shared `TEST_DATABASE_URL` instance.

**How to avoid:** Do not "fix" this by adding retries or locks around `RunMigrations`. Fix it at the actual source: either (a) give `TestRunMigrations_AppliesFromScratch` its own isolated database/schema (a per-test or per-package Postgres schema, e.g. `CREATE SCHEMA test_migrate_<random>` with `search_path` scoped to it) instead of operating on `public` — the schema the shared fixture's every other package also targets — or (b) pin `go test ./...` to `-p 1` in CI/`make test` as documented in the todo's option 3 (simplest, but slows the full suite and doesn't fix the root cause for local `-p`-default runs), or (c) both: root-fix the schema-drop test's isolation AND leave `-p` at its default, since options (a) and (c) are what actually resolves the described flake without a suite-wide slowdown. The notifier package's 4 flaky tests are a **separate, unrelated** root cause (see Pitfall 7) — do not conflate the two fixes.

**Warning signs:** Any fix that touches `RunMigrations`'s locking/retry behavior without touching `migrate_test.go`'s schema-drop is treating the wrong root cause.

### Pitfall 7: The notifier flaky tests need clock injection, not DB isolation

**What goes wrong:** The 4 flaking `internal/notifier` tests (`TestNotifyPending_SpacingAppliedEvenAfterFailedSend`, `TestNotifyPending_CrossCycleRecoveryAfterOutage`, `TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows`, `TestNotifyPending_SendFails_LeavesNotifiedAtNullAndRePicksUpNextPass`) assert on real-time `time.Now()` deltas between captured request timestamps [VERIFIED: internal/notifier/notifier_test.go:360,459,568 — `timestamps = append(timestamps, time.Now())` appears at all three line numbers], with no injectable clock anywhere in `internal/notifier` today (confirmed by grep — no `Clock` interface or type exists in the package). Under CPU/scheduling contention from `go test ./...`'s default cross-package parallelism, these timing assertions can spuriously fail even though the notifier's actual spacing/backoff logic is correct.

**Why it happens:** This is unrelated to the poller's DB-schema race (Pitfall 6) — it is wall-clock sensitivity, not shared-state contention. Fixing DB isolation does nothing for these 4 tests.

**How to avoid:** Introduce an injectable `Clock` interface (e.g. `Now() time.Time`, `Sleep(time.Duration)`) into `internal/notifier`, defaulting to a real-time implementation in production and a fake/manually-advanced clock in these 4 tests — this is the todo's own option 1, and is the only option of the three listed that fixes the tests' determinism rather than just reducing how often the underlying timing sensitivity gets exercised. `-p 1` (option 3) or accept-as-known-flake (option 4) both leave the actual sensitivity in place.

**Warning signs:** A "fix" that only changes `make test`/CI invocation flags without touching `internal/notifier`'s source has not actually removed the timing sensitivity, only reduced its trigger frequency.

## Code Examples

### Config additions (`internal/config/config.go`)

```go
// Source: this project's own file, VERIFIED: internal/config/config.go:33-35
// (existing sibling pattern) -- new fields follow the exact same
// env+envDefault struct-tag shape, added to the same "Phase 3-5" grouped
// block or a new "Phase 11" comment block per the file's own convention of
// grouping fields by the phase that introduces them.
MusicBrainzPollWorkers int `env:"MUSICBRAINZ_POLL_WORKERS" envDefault:"3"`
DeezerPollWorkers      int `env:"DEEZER_POLL_WORKERS" envDefault:"5"`
```

D-02 locks these defaults (MusicBrainz=3, Deezer=5); PERF-01 requires "default in the 3-5 range" for both, which these satisfy independently per D-01.

### Cycle-end duration/throughput log line (D-04, D-05)

```go
// Placed at the end of RunMusicBrainzCycle/RunDeezerCycle, after the
// worker-pool join (wg.Wait()) and alongside the existing NotifyPending
// call -- measures only the per-artist fan-out's wall time, not delivery
// latency, so the metric cleanly reflects the concurrency speedup PERF-01's
// success criterion asks for.
logger.Info("poll cycle complete",
    slog.Int("artist_count", len(entries)),
    slog.Int64("duration_ms", time.Since(cycleStart).Milliseconds()),
)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Sequential per-artist loop, one outbound request in flight per source at a time | Bounded concurrent fan-out, `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` in flight | This phase (Phase 11) | Cycle wall-time drops toward `max` of the rate-limiter-bound time and the pool-bound time, instead of their sum across every artist |
| Two-statement check-then-act baseline read/write (`groupBaseline` + `setGroupBaseline`) | Single atomic `UPDATE ... RETURNING` via a locking CTE (`advanceGroupBaseline`) | This phase (Phase 11) | Closes the concurrent-artist lost-update race entirely rather than narrowing it; also removes two sqlc-generated methods in favor of one |

**Deprecated/outdated:**
- `Detector.groupBaseline`/`Detector.setGroupBaseline` and the underlying `GroupTrackCountBaseline`/`SetGroupTrackCountBaseline` sqlc queries: superseded by `advanceGroupBaseline`/`AdvanceGroupTrackCountBaseline`. Remove both old queries and their generated Go code as part of this phase (D-06 explicitly calls this "costly — replaces two separate sqlc queries").

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `errgroup.Group.SetLimit` (needed for the errgroup alternative) is available in `golang.org/x/sync v0.21.0` — not independently verified against that exact pinned version's changelog this session, only general knowledge that `SetLimit` predates v0.21.0 by a wide margin | Standard Stack / Alternatives Considered | Low — if wrong, the buffered-channel semaphore (the primary recommendation, not the alternative) is unaffected; only the errgroup alternative would need adjustment, and the planner is free to skip it entirely |
| A2 | No production code path other than `internal/db/sqlc/db.go`'s generated `WithTx` currently needs explicit transaction wiring — confirmed by grepping the whole repo for `.Begin(`/`BeginTx`/`pgx.Tx`, which returned only the generated file and one unrelated test file | Common Pitfalls / Pitfall 1 | Low — this is a repo-wide grep result, not a training-knowledge guess; if a transaction helper exists elsewhere under a name the grep pattern missed, the planner would rediscover it while implementing Pitfall 1's fix |

**If this table is empty:** N/A — two low-risk assumptions are logged above; everything else load-bearing in this research (query shapes, existing code behavior, config patterns, test fixture behavior) was confirmed by directly reading the relevant source files this session.

## Open Questions

1. **Should the baseline-advance and the deluxe_change event-insert share one explicit DB transaction (Pitfall 1)?**
   - What we know: PERF-04's literal requirement (baseline correctness under concurrency) is satisfied by the single atomic `UPDATE...RETURNING` alone, with or without wrapping the subsequent `InsertEvent` call in the same transaction.
   - What's unclear: Whether the crash-window notification-loss risk introduced by reordering (advance-then-insert instead of today's insert-then-advance) is acceptable to leave undocumented-but-present, or needs closing with new transaction-wiring machinery this codebase doesn't have yet.
   - Recommendation: Planner should make this an explicit decision point (transaction wiring vs. documented accepted edge, mirroring `isSeedMode`'s existing precedent) rather than an implicit side effect of however the refactor happens to land.

2. **Exact worker-pool implementation: buffered-channel semaphore vs. `errgroup.Group{}` + `SetLimit`?**
   - What we know: Both are correct and safe for PERF-03's error-isolation requirement, given the errgroup path's discipline constraint (always return `nil` to the group).
   - What's unclear: Which one the planner/executor will find easier to write tests against, given `poller_test.go`'s existing `fakeReleaseGroupSource`/`fakeAlbumSource` fakes already expose `inFlight`/`maxInFlight` counters that work identically regardless of which primitive drives concurrency.
   - Recommendation: Default to the buffered-channel semaphore (Pattern 1) as the primary recommendation above — it needs no new dependency promotion and has no shared-object footgun surface at all — but this is genuinely a coin-flip-quality decision the planner can make either way without materially changing the phase's risk profile.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Postgres (via docker-compose / CI service container) | `TestUtil.NewTestPool`, all integration tests including new PERF-04 race test | ✓ (project convention, already required by every prior phase) | matches existing `docker-compose.yml` pin | — |
| `go test -race` | Criterion 4's explicit `-race`-clean requirement | ✓ (stdlib toolchain feature, no install needed) | Go 1.26 toolchain [VERIFIED: go.mod:3] | — |
| `sqlc` CLI | Regenerating `internal/db/sqlc/events.sql.go` after the query-file change | Not probed this session (already a required, previously-installed project dev tool per CLAUDE.md/Makefile `sqlc` target) | v1.31.1 per CLAUDE.md tech-stack doc | — |

**Missing dependencies with no fallback:** None identified.
**Missing dependencies with fallback:** None identified.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `go test -race` |
| Config file | none — plain `go test ./...` per `Makefile`'s existing `test`/`test-short`/`test-integration` targets |
| Quick run command | `go test ./internal/poller/... ./internal/detection/... -short` (unit-only, no DB) |
| Full suite command | `TEST_DATABASE_URL=... go test ./... -race` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|-------------|
| PERF-01 | Worker-pool size is env-configurable (3-5 default); a multi-artist cycle finishes measurably faster than sequential | unit (concurrency bound) + integration (timing) | `go test ./internal/poller/... -run TestMusicBrainzCycle_Concurrent -v` (new test, replacing `TestMusicBrainzCycle_Sequential`'s inverted assertion) | ❌ Wave 0 — new test |
| PERF-02 | Concurrent polling stays inside the rate limit; overlap guard still works | unit (existing `fakeReleaseGroupSource`/`fakeAlbumSource` already track `inFlight`) + a new real-timestamp-burst assertion (Pitfall 5) | `go test ./internal/poller/... -run TestMusicBrainzCycle_RateLimitHonored -v` | ❌ Wave 0 — new test; existing `TestMusicBrainzCycle_OverlapGuard_SkipsWhileInFlight` [VERIFIED: internal/poller/poller_test.go:1114-1163] already covers the overlap-guard half and needs no change |
| PERF-03 | A single artist's error doesn't abort the cycle | unit | Existing `TestMusicBrainzCycle_PerArtistErrorContinuesCycle` [VERIFIED: internal/poller/poller_test.go:428-458] and `TestPoller_RunMusicBrainzCycle_DetectionErrorIsolatedPerArtist` [VERIFIED: internal/poller/poller_test.go:1003-1048] already assert this under sequential execution — extend or duplicate to run under concurrency with multiple simultaneous failures | ✅ existing tests to extend |
| PERF-04 | Baseline CAS cannot lose an update under concurrent racing artists | integration, real Postgres, `-race` | `go test ./internal/detection/... -run TestAdvanceGroupBaseline_ConcurrentRace -race -v` | ❌ Wave 0 — new test, the phase's own highest-value test |

### Sampling Rate

- **Per task commit:** `go test ./internal/poller/... ./internal/detection/... -short`
- **Per wave merge:** `TEST_DATABASE_URL=... go test ./... -race`
- **Phase gate:** Full suite green (including `-race`) before `/gsd-verify-work`, per Criterion 4's explicit `-race` requirement

### Wave 0 Gaps

- [ ] `internal/poller/poller_test.go` — replace `TestMusicBrainzCycle_Sequential`'s `maxInFlight <= 1` assertion with a concurrency-bound assertion (Pitfall 2); add the Deezer-side twin if one exists
- [ ] `internal/poller/poller_test.go` — new rate-limit-honored-under-concurrency test using real captured timestamps (Pitfall 5), not just trusting `rate.Limiter`'s documented safety
- [ ] `internal/detection/detector_test.go` (or a new file) — `TestAdvanceGroupBaseline_ConcurrentRace`: two goroutines racing `advanceGroupBaseline` on the same `groupMBID` with different counts, asserting the final stored `track_count` equals the true maximum, run under `-race`. This is the phase's canonical proof for Criterion 4.
- [ ] `internal/notifier` — `Clock` interface + fake implementation for the 4 flaky tests (Pitfall 7)
- [ ] `internal/db/migrate_test.go` — schema isolation fix for `TestRunMigrations_AppliesFromScratch` (Pitfall 6)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | No auth surface touched by this phase |
| V3 Session Management | no | No session surface touched by this phase |
| V4 Access Control | no | No access-control surface touched by this phase |
| V5 Input Validation | no (unchanged) | The fetched `groups`/`albums`/`releases` slices this phase's workers process are the same externally-supplied data already defensively range-iterated per T-04-01/T-04-12 [VERIFIED: internal/detection/musicbrainz.go:91,193,288,315 — comments "range only -- ... is an externally-supplied slice"]; concurrency does not change how each worker validates its own slice, since each worker still processes one entry's own independent slice with no shared indexing |
| V6 Cryptography | no | Not touched |
| V11 Business Logic (informal ASVS mapping, race-condition-adjacent) | yes | This is the actual security-relevant surface of the phase: a TOCTOU (time-of-check-to-time-of-use) race in the baseline read/write, closed via the atomic CAS in Pattern 2 |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| Check-then-act race on shared mutable state (the deluxe-change baseline) allowing a lost update | Tampering (data integrity) | Single atomic `UPDATE ... RETURNING` under a `FOR UPDATE` row lock (Pattern 2) — this phase's entire PERF-04 mandate |
| Goroutine-leak / unbounded-fan-out denial-of-service if a pool bound is accidentally omitted or misconfigured to an unreasonably large value | Denial of Service | `caarlos0/env`'s struct-tag `envDefault` provides a safe default (3/5) even if the operator's env var is unset; no numeric-minimum enforcement exists in `caarlos0/env/v11` (per `internal/config/config.go:55-56`'s own comment about `EVENT_RETENTION_DAYS` needing manual validation for exactly this reason [VERIFIED: internal/config/config.go:55-63]) — the planner should add the same manual-validation pattern (reject non-positive `*_POLL_WORKERS`) in `config.Load` for consistency and safety |
| A worker's error inadvertently cancelling sibling workers (self-inflicted denial of a cycle's own correctness, not an external attacker) | Denial of Service (self-inflicted) | Never use `errgroup.WithContext`; use the buffered-channel semaphore or disciplined zero-value `errgroup.Group{}` (Pitfall 3) |

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/poller/poller.go` — full file, current sequential-loop implementation
- `internal/detection/detector.go` — full file, `groupBaseline`/`setGroupBaseline` current implementation
- `internal/detection/musicbrainz.go` — full file, `detectDeluxeChanges` current implementation and branching
- `internal/config/config.go` — full file, existing `caarlos0/env` struct-tag pattern
- `internal/httpserver/search.go` — full file, the codebase's existing concurrent-fan-out precedent
- `queries/events.sql` — the actual SQL text behind `GroupTrackCountBaseline`/`SetGroupTrackCountBaseline`
- `internal/db/sqlc/events.sql.go`, `internal/db/sqlc/db.go` — generated code confirming query shapes and `WithTx` availability
- `internal/db/migrations/000003_events.up.sql`, `000004_events_display_fields.up.sql` — `events` table schema, confirming `track_count` is a plain nullable `INT`
- `internal/db/pool.go` — confirms no explicit `MaxConns` override
- `internal/poller/poller_test.go` (partial — first ~1230 of 1474 lines) — confirms existing fakes are already concurrency-safe and locates the one test needing replacement
- `internal/notifier/notifier_test.go`, `internal/db/migrate_test.go` — grepped/read to root-cause the folded-in flaky-test todo
- `go.mod` — confirms pinned versions of `golang.org/x/time`, `golang.org/x/sync`, `caarlos0/env`
- `.planning/config.json` — confirms `nyquist_validation: true`, `security_enforcement: true`, ASVS level 1

### Secondary (MEDIUM confidence — web search cross-checked against official docs)
- pkg.go.dev/golang.org/x/sync/errgroup — `WithContext` cancellation semantics
- postgresql.org/docs/current/sql-update.html plus community CTE+`FOR UPDATE` writeups — atomic `UPDATE...RETURNING` pattern
- github.com/golang-migrate/migrate (issue discussion + driver source) — `pg_advisory_lock`-based serialization of concurrent `Up()` calls
- pkg.go.dev/github.com/jackc/pgx/v5/pgxpool — default `MaxConns = max(4, runtime.NumCPU())`
- pkg.go.dev/golang.org/x/time/rate — `Limiter` documented safe for concurrent use

### Tertiary (LOW confidence — noted for completeness only)
- github.com/golang/go/issues/65508 — reported `rate.Limiter` burst edge case at extreme (~200k req/s) throughput; explicitly assessed as not materially relevant to this project's traffic profile (Pitfall 5)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages, all versions confirmed directly in `go.mod`
- Architecture: HIGH — every pattern is grounded in this project's own existing code (search.go's fan-out, detector.go's baseline methods) read directly this session
- Pitfalls: HIGH for Pitfalls 1, 2, 3, 4, 6, 7 (all grounded in direct file reads); MEDIUM for Pitfall 5 (grounded in a cross-checked web source, but the underlying bug report is about a traffic regime far outside this project's own)

**Research date:** 2026-08-16
**Valid until:** 2026-09-15 (30 days — stable domain: Go stdlib concurrency and Postgres SQL semantics do not shift quickly; the one time-sensitive claim, `golang.org/x/sync`'s current version, should be re-checked if this research is reused past that window)
