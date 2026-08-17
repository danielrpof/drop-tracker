# Phase 11: Bounded Concurrent Polling - Context

**Gathered:** 2026-08-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Each source's poll cycle (`poller.RunMusicBrainzCycle`, `poller.RunDeezerCycle`) works through its watchlist entries several artists at a time through a bounded, env-configurable worker pool, instead of the current one-artist-at-a-time sequential loop — without breaking either source's existing rate limiter, without breaking the per-source cycle-overlap guard (`mbRunning`/`dzRunning`), and without losing correctness on the shared deluxe-change baseline when two artists sharing a release group are detected in the same concurrent cycle.

In scope: the per-artist loop inside both `RunMusicBrainzCycle` and `RunDeezerCycle`, the new worker-pool-size config surface, making the deluxe-change baseline read/write atomic at the DB level, and per-artist error isolation under concurrency. Out of scope: any new capability beyond what PERF-01 through PERF-04 already specify (see Requirements below).

</domain>

<decisions>
## Implementation Decisions

### Pool sizing & config shape
- **D-01:** Independent per-source env vars, not one shared pool-size config — mirrors the codebase's existing pattern of fully independent per-source rate limiters (`mbLimiter`/`dzLimiter`) and overlap guards (`mbRunning`/`dzRunning`); MusicBrainz's 1 req/sec limit and Deezer's ~10 req/sec limit behave very differently under the same pool size.
- **D-02:** Default pool sizes: MusicBrainz = 3, Deezer = 5. MusicBrainz's tight 1 req/sec limiter means extra MB workers mostly queue on the limiter — a smaller default avoids piling up idle goroutines; Deezer's faster limit benefits more from the larger default. Both stay within REQUIREMENTS.md PERF-01's stated 3-5 range.
- **D-03:** Env var names: `MUSICBRAINZ_POLL_WORKERS` and `DEEZER_POLL_WORKERS` — named to read as gating poll-cycle concurrency specifically, distinct from the existing `*_RATE_LIMIT_*` vars. Follow the existing `caarlos0/env` struct-tag pattern in `internal/config/config.go` (`env:"..." envDefault:"..."`).

### Speedup observability
- **D-04:** Add a `duration_ms` field to the cycle-end log line (alongside the existing per-artist `poll result` lines and the notifier-drain call at the end of each cycle) so a real deployment's logs demonstrate the speedup from success criterion 1, not just a one-time verification-time test.
- **D-05:** That same cycle-end log line also carries `artist_count` (how many watchlist entries this cycle polled) — makes throughput (artists/sec) derivable directly from logs without cross-referencing watchlist size separately.

### Baseline CAS approach (PERF-04)
- **D-06:** The deluxe-change baseline read-then-write (`detector.groupBaseline` SELECT + `detector.setGroupBaseline` UPDATE in `internal/detection/detector.go`) becomes a single atomic `UPDATE ... RETURNING` statement — the row-level lock Postgres takes during the UPDATE serializes any two concurrent callers racing on the same `external_id`, closing the check-then-act window entirely rather than narrowing it. — **Reversibility:** costly — replaces two separate sqlc queries (`GroupTrackCountBaseline`, `SetGroupTrackCountBaseline`) with one combined query; downstream detection code (`detectDeluxeChanges`'s establish-vs-advance branching) needs to be re-derived from the single statement's `RETURNING` result instead of two sequential Go-level reads, so reverting means re-splitting the query and re-deriving the two-step Go logic.
- The statement must still preserve today's two distinct outcomes (04-01/Pitfall #1's whole reason for existing): a group with no baseline yet silently establishes one (no event fires), while a group with an existing lower baseline both fires a `deluxe_change` event and advances the baseline. The atomic UPDATE's `RETURNING` clause needs to give the Go caller enough information (e.g. the previous value, or a NULL-vs-non-NULL distinction) to keep telling those two cases apart — this is an implementation detail for research/planning to work out against the existing `track_count` nullable-column design, not re-litigated here.

### Concurrent log ordering
- **D-07:** No ordering/grouping mechanism needed for per-artist log lines under concurrency — interleaved-but-labeled output is acceptable. Every `poll result` / `poll artist failed` line already carries `cycle_id`, `source`, `artist_mbid`, and `artist_name`, which is enough to reconstruct per-cycle, per-artist context regardless of emission order (grep on `cycle_id` or a log aggregator does the job). No buffering, no `worker_id` field, no output reordering.

### Claude's Discretion
- The exact worker-pool implementation primitive (raw goroutines + `sync.WaitGroup` + buffered channel/semaphore, mirroring `internal/httpserver/search.go`'s existing concurrent-fan-out pattern, vs. `golang.org/x/sync/errgroup`) is left to research/planning. Whichever is chosen must preserve PERF-03's per-artist error isolation — a worker's own error must be logged and that artist skipped, never propagated in a way that cancels sibling workers still in flight (this rules out naive `errgroup.WithContext` usage where a worker returns its error directly, since that cancels the shared context for all other in-flight workers).
- The exact `RETURNING`-clause shape and whether a companion `HasBaseline`-equivalent flag is still needed post-atomic-UPDATE (D-06) is left to research/planning, constrained by the outcome-preservation note above.
- Whether the worker pool is implemented as one pool shared by MusicBrainz-cycle and Deezer-cycle machinery, or two independently-instantiated pools (mirroring D-01's independent-config decision), is left to research/planning — D-01 only locks the *configuration* surface as independent, not necessarily two separate runtime pool objects, though independent pools is the natural implementation given D-01.

### Folded Todos
- **Fix flaky tests under parallel `go test ./...` (shared-DB contention + notifier timing)** (`.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md`, filed 2026-08-11, `resolves_phase: 9`) — folded into Phase 11's scope per user decision. Originally about cross-package test parallelism (four `internal/notifier` tests flaking on real-time sleep/spacing assertions; one `internal/poller` test flaking on a shared-DB schema-visibility race), not about the poller's own concurrency — but Phase 11 is already touching poller concurrency and adding real concurrent-artist tests (PERF-04's race test in particular), so this is the natural place to also address the notifier/poller test-suite flakiness. Solution approach (clock injection vs. per-package DB isolation vs. `-p 1` pinning vs. accept-as-known-flake) is still TBD — left to research/planning.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` (PERF-01 through PERF-04, lines 90-93) — the four locked requirements this phase satisfies
- `.planning/ROADMAP.md` §"Phase 11: Bounded Concurrent Polling" (lines 332-350) — goal, success criteria, and the "Notes" line calling out that criterion 4 requires a DB-level compare-and-set (not just `-race`) and that criterion 1's speedup must be measured, not assumed
- `.planning/PROJECT.md` §"Current Milestone" and §"Key Decisions" — confirms "Bounded concurrent polling (Phase 11) lands last — highest correctness risk of the milestone" and the "Adaptive/dynamic concurrency tuning" out-of-scope note (static env-configured pool size only)

### Folded todo
- `.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md` — flaky `internal/notifier`/`internal/poller` tests under parallel `go test ./...`, folded into this phase's scope (see Decisions § Folded Todos)

### Code this phase touches
- `internal/poller/poller.go` — `RunMusicBrainzCycle`/`RunDeezerCycle`'s sequential per-artist loops (lines 229-258, 299-336) become the bounded worker pools
- `internal/detection/detector.go` — `groupBaseline`/`setGroupBaseline` (lines 124-147) become the atomic CAS per D-06
- `internal/detection/musicbrainz.go` — `detectDeluxeChanges` (lines 240-393) calls `groupBaseline`/`setGroupBaseline` and branches on `hasBaseline`; this branching must be preserved against the new atomic query's result shape
- `internal/config/config.go` — add `MUSICBRAINZ_POLL_WORKERS`/`DEEZER_POLL_WORKERS` following the existing `caarlos0/env` struct-tag pattern (lines 33-35 show the sibling `*_RATE_LIMIT_*` vars)
- `.env.example` — must stay in parity with the new config fields (existing project convention, enforced elsewhere by a reflection-based parity test per PROJECT.md Key Decisions)

No external specs beyond the above — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/httpserver/search.go`'s `handleSearch` (lines 161-200+) is the codebase's one existing precedent for bounded concurrent fan-out: `sync.WaitGroup` + a mutex-guarded shared map, joined before responding. Not identical to a worker-pool-with-fixed-size (search fans out to exactly N sources, no pool-size bound needed there), but establishes the house style for goroutine lifecycle/error handling to follow or deliberately diverge from.
- `nextCycleID atomic.Uint64` and the `cycleID`/`logger.With(...)` pattern in `poller.go` already exist and should be reused unchanged — concurrency doesn't change how a cycle gets its correlation ID, only how its per-artist work is scheduled.

### Established Patterns
- Per-source independence is the load-bearing architectural pattern (`mbRunning`/`dzRunning` fully separate atomic guards, `mbLimiter`/`dzLimiter` fully separate rate limiters) — D-01/D-02/D-03 (independent pool config) follow this same pattern rather than introducing a new shared-resource model.
- Errors are logged-and-continue at the per-artist level today (`RunMusicBrainzCycle`/`RunDeezerCycle`'s `continue` after a fetch or detection error) — PERF-03 requires this same isolation to survive the move to concurrent workers.
- `ON CONFLICT DO NOTHING` already makes event insertion idempotent/concurrency-safe at the DB level (`detector.insertEvent`) — the deluxe-change baseline (D-06) is the one piece of detection state that does NOT yet have this property, which is exactly why PERF-04 exists.

### Integration Points
- Both poll cycles call `p.events.DetectMusicBrainz`/`DetectDeezer` (the `EventRecorder` seam) per artist today; under concurrency, multiple workers will call into the same `Detector` instance (backed by one shared `pgxpool.Pool`) simultaneously — already safe for everything except the baseline CAS.
- `p.notifier.NotifyPending` is called once, at the end of each cycle, after the per-artist loop — this must still happen only after all workers for that cycle have finished (the join point), not per-worker.

</code_context>

<specifics>
## Specific Ideas

No specific UI/UX or example-based requirements — this is a backend concurrency change with no user-facing behavior change (per PROJECT.md's milestone goal: "without changing user-facing behavior"). The concrete asks captured above (env var names, log fields, CAS shape, log-ordering tolerance) are the specifics.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. The one candidate scope-adjacent item (flaky test suite fix) was explicitly folded in rather than deferred; see Decisions § Folded Todos.

### Reviewed Todos (not folded)
None — the only matching todo was folded in.

</deferred>

---

*Phase: 11-Bounded Concurrent Polling*
*Context gathered: 2026-08-16*
