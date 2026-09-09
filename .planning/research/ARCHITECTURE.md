# Architecture Research

**Domain:** Operator observability endpoints + persistence seam on an existing Go single-binary service (drop-tracker v1.4)
**Researched:** 2026-09-09
**Confidence:** HIGH (all integration points read directly from source at cited lines; no external-doc guessing)

This document answers "how do the four v1.4 features attach to the existing architecture?" It is written for the roadmapper (Phase 18/19 split) and the phase planners (concrete file/function/line integration points, the exact `RunRecorder` interface, a `poll_runs` DDL, and a build order).

The four features:

1. `GET /ready` — readiness probe that also checks migration-version drift.
2. `poll_runs` table + `RunRecorder` seam — persist one summary row per poll cycle without giving the poller a DB connection.
3. gated `GET /status` JSON endpoint — expose recent poll-run history.
4. React "System" view — surface `/status` in the SPA behind the existing auth flow.

---

## Standard Architecture

### System Overview (existing, with v1.4 additions marked ★)

```
┌──────────────────────────────────────────────────────────────────────┐
│  cmd/server/main.go — composition root (boot order fixed)             │
│  config → logging → weak-pass WARN → gate-status log                  │
│    → db.RunMigrations → db.NewPool → clients/limiters                 │
│    → detection.New → artistart → watchlist.NewService → eventsStore   │
│    → notifier.Select → poller.New → httpserver.New                    │
│  ★ + db.ExpectedSchemaVersion (once, pre-listener)                    │
│  ★ + pollruns.NewRecorder(pool) wired via poller.WithRunRecorder      │
│  ★ + pollruns.NewStore(pool) wired into httpserver.New                │
└───────────────┬──────────────────────────────────┬───────────────────┘
                │                                  │
   ┌────────────▼─────────────┐        ┌───────────▼────────────────────┐
   │ internal/poller          │        │ internal/httpserver (chi)      │
   │  Poller — NO DB conn      │        │  Server{db,watchlist,events,   │
   │  runCycle(...) engine     │        │         sources,gate ★+status  │
   │   ├ ReleaseGroupSource    │        │         ★+ready ★+expectedVer} │
   │   ├ AlbumSource           │        │  /health  (ungated)            │
   │   ├ EventRecorder         │        │  ★ /ready (ungated)            │
   │   ├ Notifier              │        │  gated Group:                  │
   │   ★ + RunRecorder seam    │        │   /search /watchlist /events   │
   │      RecordRun(ctx,Result)│───────▶│   ★ /status                    │
   └────────────┬─────────────┘        │  NotFound → embedded SPA        │
                │                       └───────────┬────────────────────┘
   ┌────────────▼─────────────┐        ┌────────────▼───────────────────┐
   │ ★ internal/pollruns      │        │ web/app (React Router SPA)     │
   │  Recorder  (sqlc + pool) │        │  ★ routes.ts + /system route   │
   │  Store     (sqlc + pool) │        │  ★ root.tsx nav tab            │
   └────────────┬─────────────┘        │  ★ lib/api.ts getStatus()      │
                │                       │  ★ routes/system.tsx          │
   ┌────────────▼─────────────────────────────────────────────────────┐
   │ Postgres — sqlc queries (queries/*.sql), migrations              │
   │  internal/db/migrations/*.up.sql (iofs-embedded)                 │
   │  ★ 000008_poll_runs.up.sql  +  queries/pollruns.sql             │
   │  schema_migrations (golang-migrate-owned, read by /ready)        │
   └─────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Implementation for v1.4 |
|-----------|----------------|-------------------------|
| `internal/db.ExpectedSchemaVersion()` ★ | Report the max migration version embedded in this binary | New exported func in `internal/db/migrate.go`, reuses `migrationsFS` + `maxSourceVersion` |
| `httpserver` `ReadinessChecker` seam ★ | Read `schema_migrations` (version, dirty) via the live pool | New consumer-declared interface in `httpserver`, thin `*pgxpool.Pool` wrapper |
| `httpserver.handleReady` ★ | Compose ping + schema check into a `/ready` JSON response | New handler file `internal/httpserver/ready.go`, registered next to `/health` |
| `poller.RunRecorder` seam ★ | Accept one `RunResult` at cycle end; poller stays DB-free | New consumer-declared interface in `internal/poller/poller.go` |
| `internal/pollruns.Recorder` ★ | Persist a `poll_runs` row + prune to newest N (sqlc + pool) | New package mirroring `internal/events` shape |
| `internal/pollruns.Store` ★ | Read "latest N per source" for `/status` | Same package, second narrow type |
| `httpserver.handleStatus` ★ | Serve gated `GET /status` from `pollruns.Store` | New handler file `internal/httpserver/status.go`, added to `registerDataRoutes` |
| `web/app/routes/system.tsx` ★ | Render `/status` payload; mirrors `history.tsx` fetch/loading/error shape | New route + `components/system/*` |

---

## Feature-by-Feature Integration

### (a) `GET /ready` and the migration-version check

**Where the "expected" version comes from — without duplicating `migrate.go`'s source wiring.**

`internal/db/migrate.go` already owns everything needed:

- `migrationsFS` embed (`migrate.go:22-23`)
- `iofs.New(migrationsFS, "migrations")` (`migrate.go:209`, inside `RunMigrations`)
- `maxSourceVersion(src source.Driver) (uint, bool)` (`migrate.go:326-341`) — already walks a source to its highest version using the documented `os.ErrNotExist` end signal, already used by the ahead-of-source no-op guard at `migrate.go:298-302`.

**Recommendation:** add one exported function to `internal/db/migrate.go` — no new file, no new source wiring:

```go
// ExpectedSchemaVersion returns the highest migration version embedded in
// this binary. It reuses RunMigrations' own iofs source, so the number can
// never drift from what RunMigrations would apply.
func ExpectedSchemaVersion() (uint, error) {
    src, err := iofs.New(migrationsFS, "migrations")
    if err != nil {
        return 0, fmt.Errorf("load embedded migrations: %w", err)
    }
    defer func() { _ = src.Close() }()
    v, ok := maxSourceVersion(src)
    if !ok {
        return 0, errors.New("db: embedded migration source is empty")
    }
    return v, nil
}
```

Call it **once** in `cmd/server/main.go`, in the boot sequence right after `db.RunMigrations` succeeds (`main.go:137-139`) and before `httpserver.New` (`main.go:235`). Pass the resulting `uint` into `httpserver.New`. Do **not** walk the source per request.

**Where the "applied" version comes from.** `schema_migrations` is a golang-migrate-owned single-row table (`version bigint, dirty boolean`). It is not in `internal/db/migrations/`, so **sqlc does not know it** — a sqlc query is not an option without polluting the schema dir. Two clean choices:

- **Recommended:** a hand-rolled pgx query on the existing pool. This mirrors how golang-migrate itself reads the table and keeps the check on the already-open connection (no fresh `sql.Open` per probe, unlike `runMigrationsOnce` at `migrate.go:272-277`):
  ```go
  // internal/httpserver/ready.go
  type ReadinessChecker interface {
      SchemaVersion(ctx context.Context) (version uint, dirty bool, err error)
  }
  ```
  Implemented by a tiny wrapper in `main.go` (or a 15-line `internal/db` helper `SchemaVersion(ctx, pool)`) running:
  `SELECT version, dirty FROM schema_migrations`.
- Rejected: reuse `runMigrationsWithSource`-style wiring to build a `migrate.Instance` and call `m.Version()` — opens a second `database/sql` handle on every hit, slow, and duplicates the driver setup.

This is the **same consumer-declared-seam pattern** as `httpserver.Pinger` (`server.go:20-27`) — a test substitutes a fake with no real DB.

**`/ready` handler** (`internal/httpserver/ready.go`, new file; mirror `health.go` exactly):

- Route registration: `r.Get("/ready", s.handleReady)` immediately after `r.Get("/health", s.handleHealth)` at `server.go:164` — **outside** the gate in both branches (ops/orchestrator probe, must be unauthenticated, same rationale as `/health` D-03).
- Logic, bounded by a `context.WithTimeout` like `healthPingTimeout` (`health.go:17,29`):
  1. `s.db.Ping(ctx)` fails → `503`, `{"status":"not_ready","db":"down"}`.
  2. `s.ready.SchemaVersion(ctx)` errors → `503`, `{"status":"not_ready","schema":"unknown"}`.
  3. `dirty == true` → `503`, `{"status":"not_ready","schema":"dirty"}`.
  4. `applied < expected` → `503`, `{"status":"not_ready","schema":"behind","schema_version":<applied>,"expected_version":<expected>}` (migrations not fully applied, or a newer binary against an un-migrated DB).
  5. `applied >= expected` && not dirty && db up → `200`, `{"status":"ready","db":"up","schema":"ok","schema_version":<applied>}`.
- **Decision flag for the planner:** step 5 treats `applied > expected` (the N-1 rollback / ahead-of-source case that `migrate.go:298-302` deliberately tolerates) as **ready**. The stricter alternative (`applied == expected` only) would report a cleanly rolled-back binary as not-ready. Recommend `>=`; the planner should confirm against the deferred Phase 17 deploy model.
- Error text (the raw schema/ping error) goes to `httplog.SetAttrs` only, never the body — same rule as `health.go:38`.

**New vs modified:**

| File | Change |
|------|--------|
| `internal/db/migrate.go` | MODIFIED — add `ExpectedSchemaVersion()` (+ optionally `SchemaVersion(ctx, pool)`) |
| `internal/httpserver/ready.go` | NEW — `ReadinessChecker` seam + `handleReady` |
| `internal/httpserver/server.go` | MODIFIED — register `/ready`; accept checker + expected version (see (c) for how) |
| `cmd/server/main.go` | MODIFIED — call `ExpectedSchemaVersion()`, build checker, pass both to `New` |
| `internal/httpserver/ready_test.go` | NEW |

---

### (b) `RunRecorder` seam — keeping the poller DB-free

**This is the riskiest edit. `runCycle` (`poller.go:270-399`) is shared by `RunMusicBrainzCycle` and `RunDeezerCycle` and has careful ctx-cancellation + per-worker panic isolation that must not regress.**

#### Seam shape: one `RecordRun` call at cycle end (not start/finish)

A single terminal call is strongly preferred:

- It keeps the poller's "no DB connection" principle intact — `runCycle` calls a narrow interface exactly as it already calls `EventRecorder` and `Notifier` (`poller.go:80-101`).
- A start/finish pair would need an in-progress row, an UPDATE path, and crash-recovery semantics (orphaned "running" rows) — disproportionate for a portfolio observability feature. `/status` showing terminal rows for the last N cycles is sufficient.
- Trade-off accepted: a hard process crash mid-cycle leaves no row for that cycle. Per-artist panics are already recovered inside the worker (`poller.go:337-345`), so a single bad artist never costs the row.

**Consumer-declared interface, added to `internal/poller/poller.go` next to `Notifier` (`poller.go:91-101`):**

```go
// RunRecorder is the narrow seam runCycle uses to persist one poll-cycle
// summary row at the end of every cycle. Declared here, in the consumer,
// exactly as EventRecorder and Notifier are, so the Poller still holds no
// database connection itself (see the package comment) and a test can
// substitute a fake. cmd/server/main.go wires the sqlc-backed
// internal/pollruns.Recorder.
type RunRecorder interface {
    RecordRun(ctx context.Context, result RunResult) error
}

// RunResult is the immutable summary of one completed poll cycle.
type RunResult struct {
    Source         string    // "musicbrainz" | "deezer" (the existing source const)
    CycleID        string    // the existing "<source>-<n>" correlation id
    StartedAt      time.Time // cycleStart (poller.go:286)
    FinishedAt     time.Time
    DurationMS     int64     // already computed at poller.go:381
    ArtistsPolled  int       // entries for which fetchAndRecord was invoked
    ArtistsErrored int       // fetchAndRecord returned non-nil, or the worker panicked
    Outcome        string    // "success" | "error" | "cancelled"
}
```

**`EventsRecorded` is deliberately NOT on `RunResult`.** Today the `EventRecorder` methods return only `error` (`detection/musicbrainz.go:46`, `detection/deezer.go:33`); the per-cycle `inserted` count exists only as a local for logging (`musicbrainz.go:70,104`; `deezer.go:58,99`). Two ways to get `events_recorded` into the row:

- **Recommended (minimal runCycle risk): compute it downstream in `pollruns.Recorder`.** `RecordRun` runs `SELECT count(*) FROM events WHERE source = $1 AND created_at >= $2` using `StartedAt`. Safe because the per-source overlap guard (`poller.go:278-281`) guarantees no other cycle of the same source ran in that window, and `events.created_at` defaults to `now()` (`000003_events.up.sql:40`) which is always `>= StartedAt` (captured in Go before `store.List`). Guest-feature / deluxe rows also carry `source='musicbrainz'` (`000003_events.up.sql:9-11`) so they count toward MB runs — desirable. Zero poller/detection changes for the count.
- Alternative (more precise, more invasive): widen `EventRecorder.DetectMusicBrainz` / `DetectDeezer` to `(int, error)`, thread the count through the `fetchAndRecord` closures, sum with an `atomic.Int64` in `runCycle`. Touches the detection package signatures + ~6 test call sites + both closures. Only choose this if the planner wants exact attribution independent of wall-clock.

#### Wiring `RunRecorder` into the `Poller` — functional option, not a constructor param

Add via `poller.WithRunRecorder(r RunRecorder) Option`, mirroring `WithMusicBrainzWorkers` (`poller.go:110-123`). The `p.runs` field defaults to a private `noopRunRecorder{}` so `runCycle` never nil-checks it (same discipline as the always-non-nil `Notifier`, `poller.go:96-98`). This keeps `poller.New`'s signature stable — `main.go:258` stays a pure additive `poller.WithRunRecorder(pollruns.NewRecorder(pool))` argument.

#### The exact `runCycle` change (minimal, spelled out)

Signature — one change: `fetchAndRecord` gains an `error` return.

```
BEFORE (poller.go:270):
  fetchAndRecord func(ctx context.Context, logger *slog.Logger, entry watchlist.Entry)
AFTER:
  fetchAndRecord func(ctx context.Context, logger *slog.Logger, entry watchlist.Entry) error
```

Body — five localized edits, none touching the CAS guard, the semaphore/`select`-on-`ctx.Done()` dispatch, or `wg.Wait()`:

1. **After `cycleStart` (`poller.go:286`), before the dispatch loop:** declare counters.
   ```go
   var artistsPolled, artistsErrored atomic.Int64
   ```

2. **Inside the worker goroutine, in the panic-recovery block (`poller.go:337-345`):** add `artistsErrored.Add(1)` alongside the existing `logger.Error("poll worker panicked", ...)`. A panicked artist is an errored artist.

3. **Inside the worker goroutine, replacing the bare `fetchAndRecord(ctx, logger, entry)` call (`poller.go:360`):**
   ```go
   artistsPolled.Add(1)
   if err := fetchAndRecord(ctx, logger, entry); err != nil {
       artistsErrored.Add(1)
   }
   ```
   This sits *after* the existing in-flight `if err := ctx.Err(); err != nil { return }` check (`poller.go:356-358`), so a worker that never runs its fetch is not counted as polled — correct.

4. **The two closures in `RunMusicBrainzCycle` / `RunDeezerCycle` (`poller.go:417-442`, `476-501`)** change from bare `return` after a logged error to `return err` (fetch error at `:419-425` / `:478-484`; detection error at `:434-441` / `:493-500`) and `return nil` at the end. `shouldDispatch` is unchanged — a skipped nil-`DeezerID` entry (`poller.go:465-474`) is never counted in `artistsPolled`, which is correct (it was not polled).

5. **After the post-join `ctx.Err()` re-check (`poller.go:375-377`) and the existing `"poll cycle complete"` log (`poller.go:379-382`), before the `if cycleErr != nil { return cycleErr }` return (`poller.go:384-386`):**
   ```go
   outcome := "success"
   switch {
   case errors.Is(cycleErr, context.Canceled), errors.Is(cycleErr, context.DeadlineExceeded):
       outcome = "cancelled"
   case cycleErr != nil:
       outcome = "error"
   }
   recCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordRunTimeout)
   if err := p.runs.RecordRun(recCtx, RunResult{
       Source: source, CycleID: cycleID, StartedAt: cycleStart, FinishedAt: time.Now(),
       DurationMS: time.Since(cycleStart).Milliseconds(),
       ArtistsPolled: int(artistsPolled.Load()), ArtistsErrored: int(artistsErrored.Load()),
       Outcome: outcome,
   }); err != nil {
       logger.Error("record poll run failed", slog.String("run_recorder_error", err.Error()))
   }
   cancel()
   ```
   - `context.WithoutCancel` (Go 1.21+; the module is `go 1.26`) is load-bearing: a shutdown-cancelled cycle must still write its row. This is the *only* place `runCycle` needs a detached context; `recordRunTimeout` is a new package const (~5s), mirroring `healthPingTimeout`.
   - It is placed **before** the `cycleErr != nil` early return so error/cancelled cycles are still recorded — that is the whole point of the feature.
   - A `RecordRun` failure is **logged, not returned** — identical treatment to `NotifyPending` (`poller.go:394-396`): an observability write must never turn a successful detection cycle into a failed one.

**The CAS overlap-skip path (`poller.go:278-281`) is deliberately left untouched** — no `poll_runs` row for a skipped tick. It returns `ErrCycleInProgress` before `cycleID`/`cycleStart` even exist, the existing `Warn` log already covers it, and writing DB rows on that hot guard adds risk for no value. Note this exclusion for the planner (an `outcome='skipped'` row was considered and rejected).

**New vs modified for (b):**

| File | Change |
|------|--------|
| `internal/poller/poller.go` | MODIFIED — `RunRecorder`/`RunResult` types, `p.runs` field + `WithRunRecorder` option + noop default, `recordRunTimeout` const, the 5 `runCycle` edits, `fetchAndRecord` return in both closures |
| `internal/poller/poller_test.go` (+ fakes) | MODIFIED — fake `RunRecorder`, closure signatures |
| `internal/pollruns/recorder.go` | NEW — sqlc-backed `Recorder` (INSERT + prune, + optional `events` count subquery) |
| `internal/pollruns/pollruns_test.go` | NEW — integration test (`make db-up`) |
| `queries/pollruns.sql` | NEW — see (d) |
| `internal/db/migrations/000008_poll_runs.up.sql` / `.down.sql` | NEW — see (d) |
| `internal/db/sqlc/*` | REGENERATED — `make sqlc-check` gate |
| `cmd/server/main.go` | MODIFIED — `pollruns.NewRecorder(pool)` via `poller.WithRunRecorder` |

---

### (c) `GET /status` — on the existing `Server`, inside the gated Group

Yes. `/status` exposes operational data (poll timings, watchlist-derived counts, error counts) — it belongs behind the same gate as `/events` and `/watchlist`. It is a `GET`, so it is CSRF-exempt automatically.

- **New seam:** `httpserver` declares its own narrow interface (same pattern as `events.Store` at `events/service.go:82-84`, referenced from `server.go:106`):
  ```go
  // internal/httpserver/status.go
  type StatusStore interface {
      RecentPollRuns(ctx context.Context, limit int32) ([]pollruns.Run, error)
  }
  ```
  Implemented by `internal/pollruns.Store` (sqlc + pool), constructed in `main.go` and passed to `httpserver.New`.
- **Route registration:** add `r.Get("/status", s.handleStatus)` to `registerDataRoutes` (`server.go:197-204`). Because that function is called on the gated sub-router when a passphrase is set and on the root router otherwise (`server.go:166-182`), `/status` inherits `gate.Authenticate` + `gate.RequireCSRFHeader` with zero extra wiring, and `X-Instance-Gated` is set on its responses for free.
- **Handler** (`internal/httpserver/status.go`, new file): fetch newest N per source (two `RecentPollRuns` calls, or one query — see (d)), assemble the envelope, encode. Mirror `handleListEvents` error handling exactly (`events.go:124-128`): raw store error → `httplog.SetAttrs` + fixed `"internal error"` 500, never raw DB text.
- **Response envelope** (keep scoped; the planner can extend):
  ```json
  {
    "poll_interval_seconds": 3600,
    "poll_runs": {
      "musicbrainz": [ { "cycle_id": "musicbrainz-42", "started_at": "...", "finished_at": "...",
                         "duration_ms": 1234, "artists_polled": 12, "artists_errored": 0,
                         "events_recorded": 3, "outcome": "success", "error": null }, ... ],
      "deezer": [ ... ]
    }
  }
  ```
  `poll_interval_seconds` is available from `cfg.PollInterval` at `main.go` construction; pass it into `New`. Optional additions the planner may want: `watchlist_count` (needs a new `CountWatchlist` sqlc query — `queries/watchlist.sql` currently has no count), `ready` block (fold in the (a) schema check).

**How `New`'s signature grows (applies to both (a) and (c)).** `server.go:48-51` and `:106` establish the rule: every existing 5-arg `New` call stays a pure additive change. Two viable approaches — planner picks:

- **Functional options** `WithReadiness(checker, expectedVersion)` and `WithStatus(store, pollInterval)`: zero churn on the ~15 existing `New` call sites in tests; handlers return `503 "not configured"` when the option is absent. Best for `/ready` (degrades gracefully, ops-only).
- **Positional params**: honest (main.go always wires them), matches how `eventsStore` was added as the 3rd param, but touches every `New(...)` test call. Acceptable for `statusStore` since `/status` is always registered.
  Recommendation: `WithReadiness(...)` option for (a); `WithStatus(...)` option for (c), with `/status` returning 503 when unconfigured so the route table stays identical.

**New vs modified for (c):**

| File | Change |
|------|--------|
| `internal/httpserver/status.go` | NEW — `StatusStore` seam + `handleStatus` + wire types |
| `internal/httpserver/server.go` | MODIFIED — `registerDataRoutes` gains `/status`; `New` gains option(s) |
| `internal/httpserver/status_test.go` | NEW — gated + inert cases (mirror `events_test.go`) |
| `internal/pollruns/store.go` | NEW — sqlc-backed `Store.RecentPollRuns` |
| `cmd/server/main.go` | MODIFIED — build `pollruns.NewStore(pool)`, pass via option |

---

### (d) `poll_runs` schema, indexes, and prune-on-insert

**Migration `internal/db/migrations/000008_poll_runs.up.sql` (purely additive — new table, no expand/contract concern, N-1-safe because the previous release never queries it):**

```sql
-- v1.4 OPS: one summary row per completed poll cycle (RunRecorder seam).
-- Brand-new table: additive-only, no old binary references it, so the
-- README's expand/contract rule imposes nothing here. Every NOT NULL is on
-- a fresh table (never an ALTER ADD COLUMN NOT NULL), so cmd/migration-check
-- has nothing to flag. Plain CREATE INDEX (not CONCURRENTLY) is fine inside
-- golang-migrate's per-file transaction on an empty table.
CREATE TABLE poll_runs (
    id               BIGSERIAL PRIMARY KEY,
    source           TEXT        NOT NULL,
    cycle_id         TEXT        NOT NULL,
    started_at       TIMESTAMPTZ NOT NULL,
    finished_at      TIMESTAMPTZ NOT NULL,
    duration_ms      BIGINT      NOT NULL,
    artists_polled   INTEGER     NOT NULL,
    artists_errored  INTEGER     NOT NULL,
    events_recorded  INTEGER     NOT NULL,
    outcome          TEXT        NOT NULL,
    error            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT poll_runs_source_valid  CHECK (source  IN ('musicbrainz', 'deezer')),
    CONSTRAINT poll_runs_outcome_valid CHECK (outcome IN ('success', 'error', 'cancelled'))
);

-- Query pattern is exclusively "latest N for one source" — this index makes
-- it an index-only backwards scan with no sort node.
CREATE INDEX poll_runs_source_started_idx ON poll_runs (source, started_at DESC);
```

`.down.sql`: `DROP TABLE IF EXISTS poll_runs;` (exists for the pair; the app never runs it — README rule).

Column notes: `error` is the only nullable column (`NULL` on success; redacted text on failure — reuse `internal/db`'s redaction discipline if the error can carry a DSN, though poller errors here are watchlist/upstream errors, not DSN-bearing). `events_recorded` is `NOT NULL` — the `Recorder` always computes it (0 if the subquery approach finds nothing).

**Read query — one statement covers "latest N per source" for both sources** using a lateral join, so `handleStatus` makes a single round trip:

```sql
-- name: RecentPollRunsBySource :many
-- One row block per source, newest-first, capped at $1 per source.
SELECT r.id, r.source, r.cycle_id, r.started_at, r.finished_at, r.duration_ms,
       r.artists_polled, r.artists_errored, r.events_recorded, r.outcome, r.error, r.created_at
FROM (VALUES ('musicbrainz'), ('deezer')) AS s(source)
CROSS JOIN LATERAL (
    SELECT * FROM poll_runs p
    WHERE p.source = s.source
    ORDER BY p.started_at DESC, p.id DESC
    LIMIT sqlc.arg('per_source')::int
) r
ORDER BY r.source, r.started_at DESC, r.id DESC;
```

(If the planner prefers dead-simple: two `ListRecentPollRuns($source, $limit)` calls. Either is fine; the lateral version is one round trip.)

**Prune-on-insert — two explicit statements in a transaction, no trigger, no self-referential CTE.**

A single-statement CTE that both `INSERT`s and prunes is **incorrect** here: data-modifying CTEs all see the pre-statement snapshot, so the prune's `SELECT ... LIMIT N` subquery cannot see the row just inserted (off-by-one, wrong set pruned). Do not use that shape.

Instead, `pollruns.Recorder.RecordRun` opens a pgx transaction (`pool.Begin`), builds `q := sqlc.New(tx)`, and runs two queries in sequence:

```sql
-- name: InsertPollRun :exec
INSERT INTO poll_runs (
    source, cycle_id, started_at, finished_at, duration_ms,
    artists_polled, artists_errored, events_recorded, outcome, error
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: PrunePollRuns :exec
-- Keep only the newest $2 rows for $1. Runs as its own statement AFTER
-- InsertPollRun in the same tx, so the just-inserted row is visible and
-- counts toward the retained set. Explicit SQL, no trigger (project rule).
DELETE FROM poll_runs
WHERE source = $1
  AND id NOT IN (
      SELECT id FROM poll_runs
      WHERE source = $1
      ORDER BY started_at DESC, id DESC
      LIMIT $2
  );
```

The retention count `N` is a constant in `internal/pollruns` (suggest 50 per source — plenty for a "recent runs" view, bounded table growth). The transaction is nice-to-have, not load-bearing: if `PrunePollRuns` fails after `InsertPollRun` commits, the table briefly holds `>N` rows and the next run prunes them. If the `events_recorded` subquery approach from (b) is used, it runs as a third read (`CountEventsSince`) before the INSERT, inside the same tx.

**sqlc mechanics:** `queries/pollruns.sql` is picked up by `sqlc.yaml` (`queries: "queries"`, `schema: "internal/db/migrations"`) — the `000008` migration must exist before `sqlc generate`. `make sqlc-check` is a local-only gate (no CI counterpart per CLAUDE.md) — the planner must run it.

---

### (e) React "System" view

Mirrors the existing two-tab structure precisely.

- **`web/app/routes.ts`** (MODIFIED — currently `routes.ts:8-11`, two entries): add
  ```ts
  route("system", "routes/system.tsx", { id: "system-path" }),
  ```
- **`web/app/root.tsx`** (MODIFIED): add a third `<NavLink to="/system" className={tabLinkClassName}>System</NavLink>` in the `<nav>` (`root.tsx:111-119`), after the History link. **No extra gating needed** — `App` already returns `<PassphraseScreen />` before any nav markup when `!authed` (`root.tsx:105-107`), and every route renders under that guard. The System tab is a normal view; it shows always (unlike `LogoutButton`, which is `gateActive`-conditional at `root.tsx:118`).
- **`web/app/lib/api.ts`** (MODIFIED): add wire types + one wrapper, typed against the Go `statusResponse` struct exactly (the file's stated discipline — `api.ts:1-7`):
  ```ts
  export interface PollRun {
    cycle_id: string
    started_at: string
    finished_at: string
    duration_ms: number
    artists_polled: number
    artists_errored: number
    events_recorded: number
    outcome: "success" | "error" | "cancelled"
    error: string | null
  }
  export interface SystemStatus {
    poll_interval_seconds: number
    poll_runs: { musicbrainz: PollRun[]; deezer: PollRun[] }
  }
  export async function getStatus(): Promise<SystemStatus> {
    return apiFetch<SystemStatus>("/status")
  }
  ```
  Routing through `apiFetch` (`api.ts:123`) gives the System view the D-16 global-401 interceptor (`api.ts:155-158`) and the `X-Instance-Gated` latch (`api.ts:146-148`) for free — same as every other endpoint. `/ready` is **not** wrapped (no auth, ops-only) unless the planner wants a readiness badge in the System view, in which case add `getReady()` calling a bare `fetch("/ready")` (it must tolerate a 503 body).
- **`web/app/routes/system.tsx`** (NEW): structurally a copy of `history.tsx` (`routes/history.tsx`) — `useEffect` fetch on mount, `initialLoading` skeleton, `error` + `EmptyState` + Retry button (`history.tsx:74-125,165-175`). Render each source's runs as a small table or card list. New presentational components under `web/app/components/system/` (mirror `components/history/`).
- **`web/app/routes/system.test.tsx`** (NEW) — mirror `history.test.tsx`.
- **Definition of Done:** `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before staging (CLAUDE.md), plus `pnpm test`.

**New vs modified for (e):**

| File | Change |
|------|--------|
| `web/app/routes.ts` | MODIFIED — third route |
| `web/app/root.tsx` | MODIFIED — third nav tab |
| `web/app/lib/api.ts` | MODIFIED — `PollRun`/`SystemStatus` types + `getStatus()` |
| `web/app/routes/system.tsx` | NEW |
| `web/app/routes/system.test.tsx` | NEW |
| `web/app/components/system/*` | NEW |

---

## Data Flow

### `/ready` request flow

```
orchestrator GET /ready
  → chi root router (ungated, before gate Group)
  → handleReady: ctx = WithTimeout(r.Context(), readyTimeout)
      → s.db.Ping(ctx)                    ── down → 503
      → s.ready.SchemaVersion(ctx)        ── err  → 503 ;  dirty → 503
      → compare applied vs s.expectedSchemaVersion (set once at boot)
  → 200 {"status":"ready","schema_version":N}  |  503 {...reason}
```

### Poll cycle → `poll_runs` → `/status` → System view

```
cron tick → RunMusicBrainzCycle → runCycle(ctx, &mbRunning, "musicbrainz", ...)
  CAS guard ─ skip → Warn + ErrCycleInProgress (NO row, unchanged)
  store.List → dispatch workers (bounded, ctx-aware) → fetchAndRecord per artist
      worker: artistsPolled++ ; err → artistsErrored++ ; panic → recover + artistsErrored++
  wg.Wait → ctx.Err() re-check → "poll cycle complete" log
  ★ p.runs.RecordRun(WithoutCancel(ctx)+timeout, RunResult{counts, outcome})
        → pollruns.Recorder: BEGIN
            (opt) SELECT count(*) FROM events WHERE source=$1 AND created_at>=started_at
            INSERT INTO poll_runs (...)
            DELETE FROM poll_runs WHERE source=$1 AND id NOT IN (newest N)
          COMMIT
  ★ NotifyPending (unchanged) ; return cycleErr

browser → /system mount → getStatus() → GET /status (gated Group)
  → handleStatus → pollruns.Store.RecentPollRuns(N)
      → RecentPollRunsBySource (one round trip, lateral join)
  → JSON envelope → system.tsx renders per-source run tables
```

---

## Architectural Patterns (already established in this repo — reuse, don't invent)

### Pattern 1: Consumer-declared narrow seam

**What:** the consuming package declares the minimal interface it needs; the concrete type is injected at the composition root. `httpserver.Pinger` (`server.go:20-27`), `poller.ReleaseGroupSource`/`EventRecorder`/`Notifier` (`poller.go:61-101`), `events.Store` (`events/service.go:82-84`).
**Apply to v1.4:** `httpserver.ReadinessChecker`, `httpserver.StatusStore`, `poller.RunRecorder` — all declared in the consumer, all fakeable with no DB.
**Trade-off:** one more interface per boundary; pays for itself the first time a test needs to run without Postgres.

### Pattern 2: Functional options keep `New` signatures additive

**What:** `poller.Option` (`poller.go:103-123`), `httpserver.Option` (`server.go:48-72`), `db.RetryOption` (`migrate.go:42-64`). `New` builds from defaults, applies each option.
**Apply to v1.4:** `poller.WithRunRecorder`, `httpserver.WithReadiness`, `httpserver.WithStatus`. Every existing `New(...)` call site is untouched.

### Pattern 3: One stateless `sqlc.New(pool)` wrapper per consumer

**What:** `main.go` deliberately creates 5+ `sqlc.New(pool)` instances (`main.go:176,215,223,250,294`) — `sqlc.Queries` is a stateless wrapper over the shared pool; do not consolidate.
**Apply to v1.4:** `pollruns.NewRecorder(pool)` and `pollruns.NewStore(pool)` each get their own `sqlc.New(pool)` (or share the pool and call `sqlc.New(tx)` for the transactional prune).

### Pattern 4: Structured-log-and-continue for non-critical writes

**What:** a notifier delivery failure is logged, never returned, so it can't fail a good detection cycle (`poller.go:388-396`). Per-artist errors logged inside the worker, never propagated (`poller.go:420-425`).
**Apply to v1.4:** a `RecordRun` failure is logged (`"record poll run failed"`) and swallowed — observability must never break polling.

### Pattern 5: Detached context for shutdown-surviving work

**What:** `main.go` uses fresh `context.Background()` + timeout for drain paths (`main.go:271,307,338`) so a SIGTERM-cancelled `ctx` doesn't abort cleanup.
**Apply to v1.4:** `runCycle`'s `RecordRun` call wraps `context.WithoutCancel(ctx)` + a short timeout, so a cycle cancelled by shutdown still persists its (likely `outcome:"cancelled"`) row.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Giving the `Poller` a DB handle for `poll_runs`

**What people do:** pass `*pgxpool.Pool` or `sqlc.Querier` into `poller.New` to write the run row directly.
**Why it's wrong:** the package comment (`poller.go:1-15`) and the `Poller` struct's deliberate absence of a DB field are a load-bearing design decision — the poller is unit-tested with fakes and no Postgres. A DB handle here also re-opens the question of pool sizing (`main.go:141-146` sizes `MaxConns` against poll-worker count precisely).
**Do this instead:** the `RunRecorder` seam. The DB lives in `internal/pollruns`, wired at the composition root.

### Anti-Pattern 2: Self-referential prune CTE in the INSERT statement

**What people do:** `WITH ins AS (INSERT ... RETURNING), del AS (DELETE ... WHERE id NOT IN (SELECT ... LIMIT N)) SELECT`.
**Why it's wrong:** all data-modifying sub-statements see the snapshot from the start of the statement — the `DELETE`'s `SELECT` cannot see the just-`INSERT`ed row, so it retains `N` *old* rows and the new one becomes `N+1`, then the next run deletes the wrong one. Also less readable than two statements.
**Do this instead:** `InsertPollRun` then `PrunePollRuns` as two `:exec` queries in one pgx transaction — matches "explicit SQL in queries/*.sql, no triggers."

### Anti-Pattern 3: A Postgres trigger for pruning

**What people do:** `CREATE TRIGGER ... AFTER INSERT ON poll_runs`.
**Why it's wrong:** the project explicitly prefers explicit SQL in `queries/*.sql`; triggers hide behaviour from the sqlc-visible surface and from `cmd/migration-check`'s reasoning.
**Do this instead:** the explicit `PrunePollRuns` query.

### Anti-Pattern 4: Re-walking the embedded migration source per `/ready` hit

**What people do:** call `iofs.New` + `maxSourceVersion` inside `handleReady`.
**Why it's wrong:** allocates and walks the embed FS on every probe (orchestrators hit `/ready` every few seconds).
**Do this instead:** `db.ExpectedSchemaVersion()` once at boot in `main.go`, pass the `uint` into `httpserver.New`.

### Anti-Pattern 5: A stricter-than-necessary `runCycle` change

**What people do:** restructure the worker goroutine, add a results channel, collect per-artist outcomes into a slice.
**Why it's wrong:** `runCycle`'s ctx-cancellation races and panic isolation (`poller.go:293-377`) are carefully reasoned in comments; a restructure risks regressing `wg.Wait()` reachability or the double `ctx.Err()` check.
**Do this instead:** two `atomic.Int64` counters + one `error` return on `fetchAndRecord`. Nothing else moves.

---

## Integration Points

### Internal boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `poller` ↔ `pollruns.Recorder` | `RunRecorder.RecordRun(ctx, RunResult)` — one call at cycle end | Detached ctx; errors logged not returned |
| `httpserver` ↔ `pollruns.Store` | `StatusStore.RecentPollRuns(ctx, N)` | Gated route; error → fixed 500 text |
| `httpserver` ↔ `db` (readiness) | `ReadinessChecker.SchemaVersion(ctx)` + boot-time `ExpectedSchemaVersion()` | Ungated route; hand-rolled pgx read of `schema_migrations` |
| `main.go` ↔ all new pieces | explicit constructor wiring in the fixed boot order | `ExpectedSchemaVersion` after `RunMigrations`, before `NewPool`; recorder/store after `NewPool` |
| `web/app` ↔ `/status` | `getStatus()` through `apiFetch` | Inherits 401 interceptor + gate latch |

### External services

No new external services. `/ready` and `/status` are inbound-only. The `poll_runs` write adds one short transaction per poll cycle (≤ 2× `PollInterval` frequency, default hourly) — negligible pool pressure.

---

## Scaling Considerations

| Scale | Adjustment |
|-------|------------|
| Single instance (current + planned) | No change. `poll_runs` grows by ≤ 2 rows/interval, pruned to `2 × N` (~100) total. `/status` is a bounded index scan. |
| Multiple instances (not planned; `robfig/cron` has no leader election per STACK.md) | `poll_runs` would interleave rows from concurrent pollers; the per-source overlap guard is per-process, so the `events` count subquery for `events_recorded` could over-count. Would need a Postgres advisory lock around the cycle (already flagged in STACK.md as the prerequisite for multi-instance). |
| `/ready` under aggressive probing | Already bounded by a `WithTimeout`; the `schema_migrations` read is a single-row primary-key-less scan of a 1-row table — trivial. |

---

## Suggested Build Order (Phase 18/19 split)

**Dependency chain:** `000008` migration → sqlc regen → `pollruns.Recorder` → `poller.RunRecorder` edits → `pollruns.Store` → `/status` handler → `getStatus()` → System view. `/ready` is independent of all of it.

### Phase 18 — Backend: readiness + poll-run persistence + `/status`

| # | Step | Depends on | Files |
|---|------|-----------|-------|
| 18.1 | `GET /ready` | nothing | `db.ExpectedSchemaVersion` + `SchemaVersion`; `httpserver/ready.go` + `WithReadiness` option; `main.go` wiring; `ready_test.go` |
| 18.2 | `poll_runs` migration + sqlc queries | nothing (but do after 18.1 to avoid two concurrent edits to `internal/db`) | `000008_poll_runs.{up,down}.sql`; `queries/pollruns.sql`; `make sqlc-check` |
| 18.3 | `pollruns.Recorder` + `poller.RunRecorder` seam + `runCycle` edits | 18.2 | `internal/pollruns/recorder.go`; `poller.go` (5 edits + types + option); `main.go` (`WithRunRecorder`); poller + pollruns tests |
| 18.4 | `pollruns.Store` + gated `GET /status` | 18.2, 18.3 | `internal/pollruns/store.go`; `httpserver/status.go` + `WithStatus`; `registerDataRoutes` +1 line; `status_test.go`; `main.go` wiring |

Rationale for ordering: 18.1 ships standalone operator value immediately and is the lowest-risk change. 18.2 is a pure schema/codegen step that unblocks everything downstream. 18.3 is the **single riskiest step** (the `runCycle` edit) and gets its own slice with focused review. 18.4 is thin once 18.3 exists.

### Phase 19 — Frontend: System view

| # | Step | Depends on | Files |
|---|------|-----------|-------|
| 19.1 | `api.ts` wire types + `getStatus()` | 18.4 (contract must be final) | `web/app/lib/api.ts` |
| 19.2 | Route + nav tab | 19.1 | `web/app/routes.ts`; `web/app/root.tsx` |
| 19.3 | `system.tsx` + components + tests | 19.1, 19.2 | `web/app/routes/system.tsx`; `components/system/*`; `system.test.tsx`; prettier + `pnpm test` |

**Why split 18/19 at the API boundary:** the SPA is `go:embed`-ed into the binary (no separate deploy), and `api.ts`'s explicit discipline is to type against the *real* Go response body. Freezing the `/status` JSON shape in 18.4 before starting 19.1 means the frontend never re-guesses the contract. If the roadmapper prefers a single phase, 18.4 + 19.x can merge, but keep 18.3 (the `runCycle` edit) as its own reviewable unit regardless.

---

## Sources

- `cmd/server/main.go` (boot order, DI wiring, detached-context drain pattern) — read directly, HIGH
- `internal/poller/poller.go` (runCycle mechanics, consumer seams, atomic guards, overlap CAS) — read directly, HIGH
- `internal/httpserver/server.go`, `health.go`, `events.go` (route registration, gate Group, Pinger/Store seam pattern, error handling) — read directly, HIGH
- `internal/db/migrate.go` (`migrationsFS`, `maxSourceVersion`, `RunMigrations`, ahead-of-source guard) — read directly, HIGH
- `internal/db/migrations/README.md` (expand/contract, N-1 invariant, `cmd/migration-check` scope) — read directly, HIGH
- `internal/db/migrations/000003_events.up.sql`, `queries/events.sql` (table + CTE + index conventions, `source` semantics, `created_at DEFAULT now()`) — read directly, HIGH
- `internal/events/service.go`, `internal/detection/detector.go` + `musicbrainz.go`/`deezer.go` (Store/Service shape, `EventRecorder` returns only `error`, `inserted` is log-only) — read directly, HIGH
- `web/app/lib/api.ts`, `authStore.ts`, `root.tsx`, `routes.ts`, `routes/history.tsx` (SPA route + nav + fetch-wrapper + auth-latch patterns) — read directly, HIGH
- `sqlc.yaml`, `go.mod` (`queries/` + `schema:` dirs, `go 1.26` → `context.WithoutCancel` available) — read directly, HIGH
- STACK.md (robfig/cron has no leader election; advisory-lock prerequisite for multi-instance) — repo doc, MEDIUM

---
*Architecture research for: operator observability on an existing Go single-binary service*
*Researched: 2026-09-09*
