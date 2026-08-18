# Phase 11: Bounded Concurrent Polling - Pattern Map

**Mapped:** 2026-08-16
**Files analyzed:** 9
**Analogs found:** 9 / 9 (all modifications to existing files; RESEARCH.md's own Code Examples/Architecture sections are authoritative and are reproduced here verbatim where load-bearing)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/poller/poller.go` (`RunMusicBrainzCycle`/`RunDeezerCycle` per-artist loops) | service (scheduler/orchestrator) | event-driven fan-out (batch of independent fetch+detect ops) | `internal/httpserver/search.go` (`handleSearch`) | role-match (only existing concurrent-fan-out precedent in the codebase; lacks a size bound, which this phase adds) |
| `internal/detection/detector.go` (`groupBaseline`+`setGroupBaseline` → `advanceGroupBaseline`) | service (DB-backed CAS) | CRUD (atomic read-modify-write) | same file, `insertEvent` (lines 62-68) | exact — same file's own idempotent-write pattern (`ON CONFLICT DO NOTHING` → `affected > 0`) is the model for "one query, one bool-ish result, wrapped error" |
| `internal/detection/musicbrainz.go` (`detectDeluxeChanges` branching) | service (business logic / event emission) | transform (branch on CAS result → conditional event insert) | same file, existing `detectDeluxeChanges` (unchanged surrounding logic, only the two-call sequence collapses to one) | exact — modifying in place, not introducing a new shape |
| `internal/config/config.go` (+`MusicBrainzPollWorkers`, `DeezerPollWorkers`) | config | request-response (parse-once at boot) | same file, `MusicBrainzRateLimitPerSec`/`DeezerRateLimitPer5s` (lines 34-35) + `EventRetentionDays` manual-validation block (lines 44, 55-63) | exact |
| `queries/events.sql` (+`AdvanceGroupTrackCountBaseline`, remove `GroupTrackCountBaseline`/`SetGroupTrackCountBaseline`) | model (sqlc query definition) | CRUD (atomic CAS) | same file, existing `GroupTrackCountBaseline`/`SetGroupTrackCountBaseline` queries being replaced | exact |
| `internal/db/sqlc/events.sql.go` | model (generated) | CRUD | regenerated via `sqlc generate` — no hand-editing, no analog needed | n/a (generated) |
| `cmd/server/main.go` (`poller.New` gains two worker-count args) | config/wiring | request-response (constructor wiring) | same file, existing `poller.New(...)` call site threading `cfg.MusicBrainzRateLimitPerSec`/`cfg.DeezerRateLimitPer5s` into the poller constructor | exact (not read this session — small, mechanical addition; existing call site is the analog) |
| `internal/poller/poller_test.go` (replace `TestMusicBrainzCycle_Sequential`; add concurrency-bound + rate-limit-honored tests) | test | event-driven | same file, existing `fakeReleaseGroupSource`/`fakeAlbumSource` fakes (already track `inFlight`/`maxInFlight` via `atomic.Int32`) and `TestMusicBrainzCycle_PerArtistErrorContinuesCycle` | exact — fakes need zero changes, only new assertions |
| `internal/detection/detector_test.go` (new `TestAdvanceGroupBaseline_ConcurrentRace`) | test | event-driven (race test) | no existing race test in this package — closest analog is `internal/poller/poller_test.go`'s use of goroutines + `sync.WaitGroup` in fakes | role-match, no exact analog |
| `internal/notifier/notifier.go`+`notifier_test.go` (inject `Clock` interface, folded-in flaky-test fix) | utility/provider | request-response | no existing `Clock`/injectable-time abstraction in the codebase — net-new pattern | no analog found |
| `internal/db/migrate_test.go` (`TestRunMigrations_AppliesFromScratch` schema isolation fix, folded-in flaky-test fix) | test | file-I/O / DB setup | same file — existing test structure, only the shared-`public`-schema drop needs replacing with a scoped schema | exact (modifying in place) |

## Pattern Assignments

### `internal/poller/poller.go` (service, event-driven fan-out)

**Analog:** `internal/httpserver/search.go` `handleSearch` (lines 161-204), extended with a size bound per RESEARCH.md Pattern 1.

**Current sequential loop being replaced** (`internal/poller/poller.go` lines 229-258, MusicBrainz half; Deezer half is structurally identical at lines 299-336):
```go
for _, entry := range entries {
    if err := ctx.Err(); err != nil {
        return err
    }

    groups, err := p.mb.ReleaseGroupsByArtist(ctx, entry.MBID)
    if err != nil {
        logger.Error("poll artist failed",
            slog.String("artist_mbid", entry.MBID),
            slog.String("artist_name", entry.Name),
            slog.String("musicbrainz_error", err.Error()),
        )
        continue
    }

    logger.Info("poll result",
        slog.String("artist_mbid", entry.MBID),
        slog.String("artist_name", entry.Name),
        slog.Int("item_count", len(groups)),
    )

    if err := p.events.DetectMusicBrainz(ctx, logger, entry, groups); err != nil {
        logger.Error("detection failed",
            slog.String("artist_mbid", entry.MBID),
            slog.String("artist_name", entry.Name),
            slog.String("detection_error", err.Error()),
        )
        continue
    }
}
```

**Concurrent fan-out analog** (`internal/httpserver/search.go` lines 172-204) — house style for goroutine lifecycle:
```go
sources := make(map[string]sourceResult, len(s.sources))
sourceErrs := make(map[string]string, len(s.sources))
var mu sync.Mutex
var wg sync.WaitGroup

for _, src := range s.sources {
    wg.Add(1)
    go func(src SearchSource) {
        defer wg.Done()

        artists, err := src.SearchArtists(r.Context(), q, searchResultLimit)

        var result sourceResult
        var errText string
        if err != nil {
            errText = err.Error()
            result = sourceResult{Status: "error", Error: "source unavailable", Artists: []SearchArtist{}}
        } else {
            if artists == nil {
                artists = []SearchArtist{}
            }
            result = sourceResult{Status: "ok", Artists: artists}
        }

        mu.Lock()
        sources[src.Name()] = result
        if errText != "" {
            sourceErrs[src.Name()] = errText
        }
        mu.Unlock()
    }(src)
}
wg.Wait()
```
Note the `mu.Lock()`/`Unlock()` around shared-map writes — search.go's shared-state pattern to reuse *only if* the poller needs to aggregate results across workers (it currently does not: each worker only logs and calls `p.events.Detect*`, no shared map). Also note search.go's own comment (lines 206-209) about `httplog.SetAttrs` needing single-goroutine-only calls — a caution to carry over: `logger.Info`/`logger.Error` calls from worker goroutines are safe (`slog.Logger` is documented concurrency-safe), but any non-`slog` shared mutation from within a worker closure needs the same `mu.Lock()` discipline search.go uses.

**Bounded version to write (per RESEARCH.md Pattern 1, adds the missing size bound search.go doesn't need):**
```go
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
            logger.Error("poll artist failed", /* ... same fields as today ... */)
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

**Cycle-end duration/throughput log line (D-04/D-05)** — placed after `wg.Wait()`, before the existing `NotifyPending` call (lines 260-268, unchanged):
```go
logger.Info("poll cycle complete",
    slog.Int("artist_count", len(entries)),
    slog.Int64("duration_ms", time.Since(cycleStart).Milliseconds()),
)
```
`cycleStart := time.Now()` must be captured right after the overlap-guard `CompareAndSwap` succeeds (mirrors where `cycleID`/`logger.With(...)` is already set up, lines ~213-223, not re-read this session but confirmed present from the file header context).

**Imports** (lines 14-27) — no new imports needed for the buffered-channel-semaphore approach (`sync` is not yet imported; add `"sync"` alongside the existing `"sync/atomic"`). If the errgroup alternative is chosen instead, add `"golang.org/x/sync/errgroup"` and promote it in `go.mod` (`go get golang.org/x/sync@v0.21.0 && go mod tidy`).

**Error handling pattern:** every per-artist failure path uses `logger.Error(...)` then `continue`/`return` — never wraps and returns the error to the caller. This must be preserved unchanged inside each worker closure (PERF-03).

---

### `internal/detection/detector.go` (service, CRUD/CAS)

**Analog (same file):** `insertEvent` (lines 62-68) — the file's existing model for "one sqlc call, interpret its result, wrap the error":
```go
func (d *Detector) insertEvent(ctx context.Context, params sqlc.InsertEventParams) (newlyDetected bool, err error) {
    affected, err := d.q.InsertEvent(ctx, params)
    if err != nil {
        return false, fmt.Errorf("detection: insert event: %w", err)
    }
    return affected > 0, nil
}
```

**Being replaced** — `groupBaseline` (lines 124-130) + `setGroupBaseline` (lines 138-147):
```go
func (d *Detector) groupBaseline(ctx context.Context, groupMBID string) (baseline int, hasBaseline bool, err error) {
    row, err := d.q.GroupTrackCountBaseline(ctx, &groupMBID)
    if err != nil {
        return 0, false, fmt.Errorf("detection: group baseline: %w", err)
    }
    return int(row.Baseline), row.HasBaseline, nil
}

func (d *Detector) setGroupBaseline(ctx context.Context, groupMBID string, count int) error {
    trackCount := int32(count) //nolint:gosec // count is a real-world album/release track count (always well under int32 range)
    if _, err := d.q.SetGroupTrackCountBaseline(ctx, sqlc.SetGroupTrackCountBaselineParams{
        ExternalID: groupMBID,
        TrackCount: &trackCount,
    }); err != nil {
        return fmt.Errorf("detection: set group baseline: %w", err)
    }
    return nil
}
```

**New atomic replacement (RESEARCH.md Code Examples, Pattern 2):**
```go
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

**Doc-comment precedent for the accepted-edge-case option (Pitfall 1):** `isSeedMode`'s doc comment (lines 70-90) is this file's existing precedent for documenting a known, accepted correctness edge rather than closing it with new machinery — reuse this same "Known, accepted edge:" comment style if the crash-window notification-loss risk is left undocumented-but-present rather than wrapped in an explicit transaction.

---

### `internal/detection/musicbrainz.go` (`detectDeluxeChanges`)

**Analog:** the file's own current branching logic at lines 353-375 (insert-event-then-advance-baseline order) — RESEARCH.md Pitfall 1 flags that collapsing to advance-then-insert changes crash-safety; this is a planner-level decision (transaction wrap vs. documented accepted edge), not a copy-paste pattern. No external analog needed — modify in place, re-deriving the `switch` on `advanced`/`hadBaseline` per RESEARCH.md's Architecture Diagram:
```go
switch {
case !advanced:                 // fresh count not an increase -- no-op (unchanged outcome)
case advanced && !hadBaseline:  // silently established -- no event (unchanged outcome)
case advanced && hadBaseline:   // fire deluxe_change event with previousTrackCount = previousBaseline (unchanged outcome)
}
```

---

### `internal/config/config.go` (config, request-response)

**Analog (same file):** existing rate-limit sibling fields (lines 34-35):
```go
MusicBrainzRateLimitPerSec float64 `env:"MUSICBRAINZ_RATE_LIMIT_PER_SEC" envDefault:"1"`
DeezerRateLimitPer5s       int     `env:"DEEZER_RATE_LIMIT_PER_5S" envDefault:"50"`
```

**New fields to add (RESEARCH.md Code Examples, locked defaults D-02/D-03):**
```go
MusicBrainzPollWorkers int `env:"MUSICBRAINZ_POLL_WORKERS" envDefault:"3"`
DeezerPollWorkers      int `env:"DEEZER_POLL_WORKERS" envDefault:"5"`
```

**Manual-validation pattern to mirror (lines 55-63)** — `caarlos0/env/v11` has no numeric-minimum struct tag, so non-positive values must be rejected manually in `Load`, same placement (after `env.Parse`'s own aggregate-error return):
```go
if cfg.EventRetentionDays <= 0 {
    return nil, fmt.Errorf("EVENT_RETENTION_DAYS must be a positive integer, got %d", cfg.EventRetentionDays)
}
```
Apply the identical shape for `MusicBrainzPollWorkers`/`DeezerPollWorkers` (reject `<= 0`).

**File header comment convention (lines 25-27):** fields are grouped by the phase that introduces them — add a `// Phase 11 — ...` comment block above the two new fields, following this file's own stated convention rather than appending to the "Phase 3-5" block.

---

## Shared Patterns

### Per-artist error isolation (never propagate into shared cancellation)
**Source:** `internal/poller/poller.go` lines 234-257 (`continue` after logged error) + RESEARCH.md Pitfall 3
**Apply to:** every worker closure in both `RunMusicBrainzCycle` and `RunDeezerCycle`
```go
// inside worker closure — log and return, never return the error to a shared object
if err != nil {
    logger.Error("poll artist failed", /* fields */)
    return // NOT `errChan <- err` or `return err` to an errgroup.WithContext group
}
```
Explicitly forbidden: `g, ctx := errgroup.WithContext(ctx)` — cancels sibling workers on first error (RESEARCH.md Pitfall 3, confirmed via pkg.go.dev/golang.org/x/sync/errgroup).

### Structured logging fields already established
**Source:** `internal/poller/poller.go` lines 236-240, 244-248 (existing `poll artist failed`/`poll result` lines)
**Apply to:** all new/modified log lines in this phase — reuse `slog.String("artist_mbid", ...)`, `slog.String("artist_name", ...)`, plus new `slog.Int("artist_count", ...)` / `slog.Int64("duration_ms", ...)` per D-04/D-05. No `worker_id` field (D-07 — explicitly rejected).

### Atomic CAS via locking CTE + `UPDATE ... RETURNING`
**Source:** RESEARCH.md Pattern 2 (`queries/events.sql`, new `AdvanceGroupTrackCountBaseline`)
```sql
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
**Apply to:** the sole PERF-04 CAS surface — no other query in this phase needs this pattern.

### Manual post-`env.Parse` validation for new numeric config
**Source:** `internal/config/config.go` lines 55-63 (`EventRetentionDays` non-positive rejection)
**Apply to:** `MusicBrainzPollWorkers`/`DeezerPollWorkers` (reject `<= 0`, matching this file's established idiom).

### `.env.example` parity
**Source:** existing project convention (enforced by a reflection-based parity test per PROJECT.md Key Decisions) — not read this session, but every existing `env:"..."` tag in `config.go` has a corresponding line in `.env.example`. Add `MUSICBRAINZ_POLL_WORKERS=3` and `DEEZER_POLL_WORKERS=5` there.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/notifier/notifier.go` / `notifier_test.go` (`Clock` interface injection, folded-in flaky-test fix) | provider/utility | request-response | No existing injectable-time abstraction anywhere in the codebase (confirmed by RESEARCH.md's grep) — this is a net-new pattern; RESEARCH.md's own Pitfall 7 is the closest thing to a spec (`Now() time.Time`, `Sleep(time.Duration)`, real impl in production, fake/manually-advanced impl in the 4 flaking tests: `TestNotifyPending_SpacingAppliedEvenAfterFailedSend`, `TestNotifyPending_CrossCycleRecoveryAfterOutage`, `TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows`, `TestNotifyPending_SendFails_LeavesNotifiedAtNullAndRePicksUpNextPass`, at `internal/notifier/notifier_test.go` lines 360, 459, 568). |
| `internal/detection/detector_test.go` (`TestAdvanceGroupBaseline_ConcurrentRace`) | test | event-driven race test | No existing intentional-race integration test in this package; closest structural precedent is `internal/poller/poller_test.go`'s goroutine + `sync.WaitGroup` + `atomic.Int32` fakes (lines 133-166, 242-276) — reuse that same `atomic`/`sync.WaitGroup` shape to spin up two goroutines calling `advanceGroupBaseline` concurrently on the same `groupMBID`, then assert the final stored `track_count` equals the true max, run under `-race`. |

## Metadata

**Analog search scope:** `internal/poller/`, `internal/detection/`, `internal/config/`, `internal/httpserver/`, `internal/notifier/`, `internal/db/`, `queries/`
**Files read directly this session:** `internal/poller/poller.go` (lines 1-27, 225-339), `internal/detection/detector.go` (lines 60-160), `internal/config/config.go` (full), `internal/httpserver/search.go` (lines 155-210); remaining file details (musicbrainz.go, poller_test.go, notifier_test.go, migrate_test.go, events.sql, db.go, pool.go) taken from RESEARCH.md's own directly-verified excerpts (all cited with `[VERIFIED: file:line]` tags in that document) to avoid redundant re-reads of files RESEARCH.md already fully covered this session.
**Pattern extraction date:** 2026-08-16
