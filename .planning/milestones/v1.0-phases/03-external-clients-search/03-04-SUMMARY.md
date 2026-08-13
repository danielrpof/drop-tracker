---
phase: 03-external-clients-search
plan: 04
subsystem: scheduler
tags: [robfig-cron, golang, rate-limiting, graceful-shutdown, log-slog]

# Dependency graph
requires:
  - phase: 03-external-clients-search
    plan: 02
    provides: "internal/deezer.AlbumLister (ArtistAlbums) and the process-wide *deezer.Client/rate.Limiter pair constructed in cmd/server/main.go"
  - phase: 03-external-clients-search
    plan: 03
    provides: "internal/musicbrainz.ReleaseGroupLister (ReleaseGroupsByArtist), bounded/paced/sequential pagination over release-groups"
provides:
  - "internal/poller: two independent robfig/cron-scheduled poll cycles (RunMusicBrainzCycle, RunDeezerCycle) on Config.PollInterval, each reading the live watchlist through watchlist.Store, polling one artist at a time, log-only (D-04)"
  - "Per-source overlap guard (atomic.Bool CompareAndSwap, ErrCycleInProgress) that skips and warns on an overlapping tick rather than queuing it, released via defer so an error or panic can never wedge a source (D-09)"
  - "cmd/server/main.go wires the poller from the same *musicbrainz.Client/*deezer.Client instances httpserver.New already uses, and drains it (bounded by pollDrainTimeout) after defer pool.Close() via LIFO ordering"
affects: [04-detection-engine]

# Actuals (#2632)
actuals:
  tokens: 12000
  tasks: 3
  commits: 5

# Tech tracking
tech-stack:
  added: ["github.com/robfig/cron/v3 v3.0.1"]
  patterns:
    - "Two atomic.Bool guards (mbRunning/dzRunning), never one shared mutex or lock -- a compare-and-swap that returns ErrCycleInProgress on an already-running source, released via defer so it survives both an error return and a panic. Reusable for any future per-source overlap guard (e.g. a Discord notifier poll)."
    - "Poller.Start retains a cancellable child context (runCtx/runCancel) that every cron-dispatched job closure reads; Stop cancels it only after its own bounding context expires, consuming cron.Cron.Stop()'s returned drain context rather than ignoring it -- the exact shape 03-RESEARCH.md's pitfall 4 called for."
    - "cron.Cron.Stop()'s returned context only tracks jobs cron's own dispatch loop started (an internal sync.WaitGroup) -- a cycle invoked directly by test code is invisible to it. Tests that assert Stop's drain semantics must drive a real (short) interval and a real tick, not call the cycle method directly; overlap-guard tests, which only need CAS behavior, still call cycle methods directly per 03-RESEARCH.md guidance."

key-files:
  created:
    - internal/poller/poller.go
    - internal/poller/poller_test.go
  modified:
    - cmd/server/main.go
    - go.mod
    - go.sum

key-decisions:
  - "go.mod: robfig/cron/v3 landed as an indirect dependency in task 1 (go get with nothing importing it yet, so go mod tidy would have stripped it) and only became a direct dependency once task 2 actually imported it into poller.go and go mod tidy ran again -- task 1's own acceptance-criteria grep for 'outside the indirect block' is satisfied cumulatively by the end of task 2, not standalone after task 1, since the plan's own action text explicitly defers cron wiring to task 2."
  - "Stop()'s drain-semantics tests (TestStop_ReturnsNilOnceInFlightCycleFinishes, TestStop_ReturnsCallerContextErrorWhenCycleOutlivesIt, TestStop_NoFurtherCycleBeginsAfterStop) drive a real short cron interval (30-50ms) and wait for a real dispatched tick, rather than calling the cycle methods directly as the overlap-guard tests do -- cron.Cron.Stop()'s returned context is backed by an internal WaitGroup that only tracks jobs cron itself started, so a directly-invoked cycle would make Stop() return immediately regardless of whether it had actually finished, silently passing a test that proved nothing."
  - "REQUIREMENTS.md already showed CLNT-01/CLNT-02 checked off from plans 03-02/03-03 (their summaries closed them on the strength of the underlying client fetch methods existing) even though the requirement text names scheduled polling specifically -- this plan is what actually delivers that scheduled-polling behavior; requirements.mark-complete is re-run here as a no-op confirmation, and the discrepancy is noted rather than silently left."

patterns-established:
  - "Pattern: package-level atomic.Uint64 cycle-id counter rendered as '<source>-<n>' and bound onto a per-cycle logger via logger.With(...) at the top of the cycle method -- every log line inside that cycle inherits the same correlation id with no explicit threading through each call site. Reusable for Phase 4's diff engine if it ever needs its own per-run correlation id outside an HTTP request context."

requirements-completed: [CLNT-01, CLNT-02]

coverage:
  - id: D1
    description: "RunMusicBrainzCycle and RunDeezerCycle read the live watchlist through watchlist.Store and poll one artist at a time, sequentially, with no worker pool or goroutine fan-out"
    requirement: "CLNT-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_CallsSourceOncePerEntry"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_Sequential"
        status: pass
    human_judgment: false
  - id: D2
    description: "A nil deezer_id is skipped with a logged reason and zero HTTP requests; a non-nil id is dereferenced and passed through unchanged"
    requirement: "CLNT-02"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_SkipsNilDeezerID"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every successful artist/source pair logs one structured record carrying source, artist_mbid, artist_name, item_count, and a cycle_id shared within a cycle and distinct across cycles"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_LogsStructuredResult"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_CycleIDSharedAcrossRecordsDiffersBetweenCycles"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_LogsStructuredResult"
        status: pass
      - kind: integration
        ref: "manual run: local binary against real Postgres, two watchlisted artists (one without deezer_id), POLL_INTERVAL=8s -- observed matching cycle_id per cycle, item_count:36 from a real api.deezer.com call, and the skip log line for the nil-deezer_id artist"
        status: pass
    human_judgment: false
  - id: D4
    description: "A per-artist error is logged and the cycle continues to the next artist rather than aborting; a store.List error aborts the cycle with a non-nil error and zero source calls"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_PerArtistErrorContinuesCycle"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_ListErrorReturnsZeroCallsNonNilError"
        status: pass
      - kind: integration
        ref: "manual run: MusicBrainz calls failed with the sandbox's documented no-outbound-TLS-to-musicbrainz.org limitation (EOF) on every artist, and the cycle logged an error per artist and continued rather than aborting"
        status: pass
    human_judgment: false
  - id: D5
    description: "An overlapping tick for a source is skipped (ErrCycleInProgress, zero store/source calls, one warn log) rather than queued; the guard is per-source and releases on completion, error, and panic"
    requirement: "CLNT-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_OverlapGuard_SkipsWhileInFlight"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestMusicBrainzCycle_GuardReleasesOnPanic"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestDeezerCycle_RunsIndependentlyDuringMusicBrainzCycle"
        status: pass
    human_judgment: false
  - id: D6
    description: "New registers exactly two independent cron entries on @every <interval>, and Stop drains an in-flight cron-dispatched cycle bounded by the caller's context"
    requirement: "CLNT-01"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestNew_RegistersEveryIntervalSpecOnBothEntries"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestStop_ReturnsNilOnceInFlightCycleFinishes"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestStop_ReturnsCallerContextErrorWhenCycleOutlivesIt"
        status: pass
      - kind: unit
        ref: "internal/poller/poller_test.go#TestPoller_StartStop_LifecycleWithRealCronTick"
        status: pass
    human_judgment: false
  - id: D7
    description: "cmd/server/main.go constructs the poller from the same mbClient/dzClient instances handed to httpserver.New (one limiter per source, whole-process budget), and drains it after defer pool.Close() so LIFO ordering guarantees the poller stops before the pool closes"
    verification:
      - kind: unit
        ref: "grep gate: musicbrainz.NewClient(/deezer.NewClient(/rate.NewLimiter( each appear exactly once/twice as required, defer pool.Close() line precedes pollr.Stop line"
        status: pass
      - kind: manual_procedural
        ref: "clean SIGTERM delivery to a backgrounded Windows process could not be exercised in this sandbox (same gap noted for WR-03 in Phase 1's UAT); the deferred drain was observed firing correctly on an early-return path (port-bind failure) during manual testing, and drain-before-close ordering itself is proven by internal/poller's own Stop() unit tests"
        status: pass
    human_judgment: true
    rationale: "A real SIGTERM-triggered graceful shutdown against the live binary (poller drains, then pool closes, with no connection error in between) could not be exercised in this Windows sandbox -- signals sent via MSYS kill terminate the process directly rather than being caught by Go's os/signal, the same limitation Phase 1's UAT documented for WR-03. A human with access to a POSIX shell (or WSL2, as Phase 1 used) should confirm the full log-line ordering under a real SIGTERM."

duration: 25min
completed: 2026-08-07
status: complete
---

# Phase 3 Plan 4: Scheduler — Two Independent Poll Cycles Summary

**`internal/poller` runs two independent `robfig/cron` jobs (MusicBrainz, Deezer) on `Config.PollInterval`, each sequentially polling the live watchlist through the existing `watchlist.Store` seam, log-only with a per-source overlap guard and a draining shutdown wired into `cmd/server/main.go`.**

## Performance

- **Duration:** ~25 min (commits span 17:52–18:07; plus live verification against real Postgres/Deezer and cleanup afterward)
- **Started:** 2026-08-07T17:52:42-05:00
- **Completed:** 2026-08-07T18:07:24-05:00
- **Tasks:** 3 completed
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments
- `internal/poller.Poller` runs `RunMusicBrainzCycle` and `RunDeezerCycle`, each reading the live watchlist through the existing `watchlist.Store` interface (no new query, D-05) and calling its source exactly once per artist, strictly sequentially with a plain `for` loop -- no worker pool, no goroutine, so the operator-configured per-source rate limiter is never multiplied by concurrency (D-07). A nil `deezer_id` is skipped with a logged reason and zero HTTP requests; there is no name-search fallback (D-06).
- Every artist/source pair produces exactly one structured `slog` record carrying `source`, `artist_mbid`, `artist_name`, `item_count`, and a per-cycle `cycle_id` (a package-level `atomic.Uint64` counter rendered `<source>-<n>`) -- all records within one cycle share the same id, and successive cycles get distinct ones. A per-artist error is logged and the cycle continues rather than aborting; a `store.List` failure aborts the whole cycle with a non-nil error and zero source calls. Neither cycle writes to the database this phase (D-04) -- Phase 4 replaces the log line with real diff logic.
- Each cycle is guarded by its own `atomic.Bool` (`mbRunning`/`dzRunning` -- two separate fields, never one shared mutex, per D-08) via `CompareAndSwap`: an overlapping tick returns `ErrCycleInProgress` immediately with zero store reads, zero source calls, and one warn-level log naming the source (D-09). The guard releases via `defer`, so it clears on an error return *and* on a panic -- a wedged flag would otherwise silently stop that source polling for the process's lifetime.
- `New` registers exactly two independent `cron.AddFunc` entries on `"@every <interval>"`, one per cycle method, so MusicBrainz's slower pace can never delay or block Deezer's faster one. `Start` retains a cancellable child context every dispatched job runs with; `Stop` stops scheduling, then waits on `cron.Cron.Stop()`'s own drain context bounded by the caller's context -- if the caller's context expires first, the retained cycle context is cancelled so an in-flight request unwinds instead of blocking forever.
- `cmd/server/main.go` constructs the poller from the *same* `mbClient`/`dzClient` instances already handed to `httpserver.New`, so search traffic and poll traffic draw from one `rate.Limiter` per source rather than two. The drain is deferred *after* `defer pool.Close()`, so Go's LIFO ordering guarantees the poller drains before the pool closes, bounded by a new `pollDrainTimeout` (10s) so a hung upstream can't make shutdown unkillable.
- Live-verified against a real Postgres and `api.deezer.com`: two watchlisted artists (one without a `deezer_id`) each produced the expected `poll result`/skip log lines with a shared `cycle_id` per cycle; a real Deezer call returned `item_count: 36`. MusicBrainz calls failed with the same sandbox no-outbound-TLS-to-`musicbrainz.org` limitation already documented in 03-01/03-02/03-03, and the cycle correctly logged an error per artist and continued.

## Task Commits

Each task committed atomically, with a RED test commit ahead of its GREEN implementation commit per `tdd="true"`:

1. **Task 1: The two poll cycles — sequential, log-only, nil-deezer_id skipped**
   - `217bc5f` test(03-04): add failing tests for poll cycle behavior
   - `6fafaa0` feat(03-04): add sequential, log-only poll cycles for both sources
2. **Task 2: Two independent cron jobs, per-source overlap guard, and a draining Stop**
   - `1edadbb` test(03-04): add failing tests for overlap guard, cron registration, and drain
   - `3dfcfb8` feat(03-04): add per-source overlap guard, two cron jobs, and draining Stop
3. **Task 3: Start and drain the poller from cmd/server, sharing one client per source**
   - `8a2990f` feat(03-04): start and drain the poller from cmd/server, sharing clients

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/poller/poller.go` - `Poller`, `New`, `ReleaseGroupSource`, `AlbumSource`, `ErrCycleInProgress`, `RunMusicBrainzCycle`, `RunDeezerCycle`, `Start`, `Stop`, package-level `nextCycleID`
- `internal/poller/poller_test.go` - `stubStore`, `fakeReleaseGroupSource`, `fakeAlbumSource` doubles; 30 test functions covering the full sequential/skip/log/error/guard/cron/drain behavior surface
- `cmd/server/main.go` - constructs and starts the poller from the shared `mbClient`/`dzClient`, defers a bounded drain after `defer pool.Close()`, updates the package doc comment
- `go.mod`, `go.sum` - `github.com/robfig/cron/v3 v3.0.1` (direct dependency)

## Decisions Made
- `github.com/robfig/cron/v3` was added via `go get` in task 1 per the plan's staged action but landed as an `// indirect` dependency (nothing imported it yet) rather than direct -- running `go mod tidy` at that point would have stripped it entirely, so it was left as-is until task 2 actually imported `cron` into `poller.go`, at which point `go mod tidy` correctly promoted it to a direct dependency. Task 1's own acceptance-criteria grep for "outside the indirect block" is satisfied cumulatively by the end of task 2, consistent with the plan's own text ("The cron wiring itself lands in task 2").
- `Stop()`'s three drain-semantics tests drive a real short cron interval (30-50ms) rather than calling `RunMusicBrainzCycle`/`RunDeezerCycle` directly, unlike the overlap-guard tests. `cron.Cron.Stop()`'s returned context is backed by an internal `sync.WaitGroup` that only tracks jobs *cron itself* dispatched -- a directly-invoked cycle is invisible to it, so a test built that way would see `Stop()` return `nil` immediately regardless of whether the manually-launched goroutine had actually finished, passing without proving anything.
- Live-verified end-to-end against real Postgres and `api.deezer.com` (not just `httptest.Server` fixtures) by building the binary, adding two watchlist entries via the running HTTP API, and observing the poll-cycle log lines over several real cron ticks, then cleaning up both the watchlist rows and the docker-compose Postgres container afterward.

## Deviations from Plan

None — plan executed exactly as written. All tasks matched their `<behavior>`/`<action>`/`<acceptance_criteria>` specification; no auto-fixes were required.

## Issues Encountered
- **Same sandbox no-outbound-TLS-to-`musicbrainz.org` limitation documented in 03-01/03-02/03-03** resurfaced during live verification: every `RunMusicBrainzCycle` call failed with `EOF` on the outbound TLS handshake, while `RunDeezerCycle` succeeded against real `api.deezer.com`. This is environmental, not an application defect, and it incidentally proved the per-artist-error-continues-cycle behavior live (both artists still got attempted, both errors logged, cycle returned nil).
- **Clean SIGTERM delivery to a backgrounded Windows process could not be exercised** in this sandbox — the same gap Phase 1's UAT documented for WR-03 (MSYS `kill -TERM` terminates the process directly rather than being caught by Go's `os/signal`, since there's no real POSIX signal delivery on Windows outside WSL2). The full "shutdown signal received → poller stopped → pool closes → process exit" log ordering was not observed live end-to-end; it is instead covered by `internal/poller`'s own `Stop()` unit tests (which do prove the drain-before-return semantics under a real cron tick) and by observing the deferred drain fire correctly on an early-return path (a port-bind failure) during manual testing. A human with a POSIX shell or WSL2 should confirm the live SIGTERM ordering, matching how Phase 1's equivalent gap was closed.

## User Setup Required

None - no external service configuration required. (The poller reuses the existing `mbClient`/`dzClient`/`DATABASE_URL` configuration; no new environment variable was introduced.)

## Next Phase Readiness
- CLNT-01 and CLNT-02 are now genuinely fulfilled by scheduled polling (previously marked complete by 03-02/03-03 on the strength of the underlying fetch methods existing — see Decisions Made). Phase 4's diff engine can replace `RunMusicBrainzCycle`/`RunDeezerCycle`'s log-only bodies with real diff-against-"seen"-store logic and inherit the overlap guard (D-09) directly, per the plan's stated intent.
- Full test suite (`go test -short ./... -count=1`, and `TEST_DATABASE_URL=... go test ./... -count=1` against a real docker-compose Postgres) is green; `go build ./...` and `go vet ./...` are clean. `go test -race` could not be run in this sandbox — pre-existing mingw64/cgo toolchain limitation already documented for Phase 1/2 plans, unrelated to this plan's code.
- A live SIGTERM-triggered graceful-shutdown check (poller drains before the pool closes, no connection error in between) remains open for human verification in an environment that can deliver real POSIX signals (WSL2 or Linux), mirroring how Phase 1's WR-03 gap was closed.

---
*Phase: 03-external-clients-search*
*Completed: 2026-08-07*

## Self-Check: PASSED

All 2 claimed created files (`internal/poller/poller.go`, `internal/poller/poller_test.go`) exist on disk; `cmd/server/main.go`, `go.mod`, `go.sum` (modified) exist on disk. All 5 claimed commit hashes (`217bc5f`, `6fafaa0`, `1edadbb`, `3dfcfb8`, `8a2990f`) resolve in `git log --oneline --all`.
