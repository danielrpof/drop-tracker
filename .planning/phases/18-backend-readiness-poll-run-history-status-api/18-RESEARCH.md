# Phase 18: Backend — Readiness, Status Surface & App Version - Research

**Researched:** 2026-09-09
**Domain:** Additive operator-observability endpoints + an in-process history seam on an existing Go single-binary service (chi + pgx + sqlc + robfig/cron)
**Confidence:** HIGH — every integration point read directly from source this session at cited line ranges; the two external facts (golang-migrate `schema_migrations` shape, Go `-ldflags -X` + Docker build-arg) cross-checked against multiple sources.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

CONTEXT.md for this phase uses `<decisions>` / `<deferred>` prose blocks rather than
the `## Decisions` / `## Claude's Discretion` / `## Deferred Ideas` headings. Reproduced
verbatim below. **The ROADMAP.md "Phase 18" → "Notes for the phase planner" block is the
primary brief and carries additional binding detail — the planner MUST read it.**

### Locked Decisions

**`/ready` probe**

- **D-01 (unchanged): Ready condition is `!dirty && applied >= expected`.** Phase 16's
  ahead-of-source guard (`migrate.go:298-302`) deliberately lets a rolled-back binary boot
  and serve against a newer additive schema; strict `==` would report that healthy instance
  not-ready forever and flap the deferred Phase 17 deploy gate. `expected` = the existing
  `maxSourceVersion` walk, exported as `db.ExpectedSchemaVersion()`, called once at boot.
  Rationale one-liner + the Phase-16 reference go in the handler comment.
- **D-02 (unchanged): `503` body carries a machine reason enum** — exactly one of
  `db_unreachable`, `schema_behind`, `schema_dirty`. No DSN, driver text, path, or free-form
  message. Raw cause → `httplog.SetAttrs` only.
- **D-03 (unchanged): `/ready` body shape** — `200`:
  `{"status":"ready","schema_applied":<int>,"schema_expected":<int>}`. `503`: the same fields
  best-effort (`schema_applied` may be null if the DB is unreachable) **plus**
  `"reason":"<enum>"`. Key names locked — Phase 19's readiness badge parses this directly.
- **D-04 (unchanged): `/ready` mirrors `/health`'s registration** — root router, **both** the
  gate-configured and inert branches (structural exemption, never a path-string match),
  3s-bounded DB check on the shared pool, no side effects, never `401` on a gated instance
  with no cookie.
- **NEW: schema-version read is shared with `/status`.** One `db.SchemaVersion(ctx, pool)`
  helper — hand-rolled pgx `SELECT version, dirty FROM schema_migrations` (that table is
  golang-migrate-owned, outside the schema dir, so it cannot be a sqlc query). `/ready` and
  `/status`'s `instance` block both call it.

**Run-history store**

- **D-05 SUPERSEDED — skip-overlap coalescing is gone.** A skipped tick is **not** a run
  entry. It bumps a per-source `consecutiveSkips` counter and stamps `lastSkippedAt` on the
  store; both are surfaced in `/status`. The next real run resets `consecutiveSkips` to 0.
  (The store field + method land here; the `RecordSkip` *call* from `runCycle` is Phase 18.1.)
  REQUIREMENTS RUN-02 was reworded to match.
- **D-06 SUPERSEDED — no `outcome` CHECK constraint.** No table, no SQL. The `Outcome` field
  is a Go string, valid values `ok` / `error` / `cancelled` (no `partial`, no
  `skipped_overlap`), validated in Go if at all.
- **D-07 SUPERSEDED — no detached context.** `RecordRun` is a synchronous in-memory mutex
  append that returns in microseconds; a shutdown-cancelled cycle records its entry with no
  `context.WithoutCancel` dance. `ctx` stays in the seam signature for symmetry with
  `EventRecorder` / `Notifier`, but it is not load-bearing. (The call semantics —
  log-and-swallow — are Phase 18.1.)
- **D-08 SUPERSEDED — ring buffer, not a pruned table.** `internal/pollruns.Store` holds
  `[]RunResult` per source, cap `const N = 50` (named const in the `pollruns` package,
  one-line comment on the number). No prune query, no INSERT+DELETE CTE, no
  cross-source-deadlock surface. Correct under two sources recording near-simultaneously =
  one `sync.Mutex` (or `RWMutex`) on the store.
- **D-09 (unchanged): `Summary` is a stored string, composed deterministically** from the
  counts + outcome enum at record time — e.g. `"ok — 12 checked, 1 errored, 3 events"`.
  Never from a driver error, upstream response, DSN, webhook URL, or path. Held on the
  `RunResult` value.
- **D-10 SUPERSEDED — no migration, no sqlc for run history.** `RunResult` is a plain Go
  struct: `Source`, `CycleID`, `StartedAt`, `FinishedAt`, `DurationMS`, `ArtistsChecked` (=
  entries dispatched to a worker), `ArtistsSkipped`, `ArtistsErrored`, `EventsRecorded`,
  `Outcome`, `Summary`. The **only** new sqlc query this phase adds is `CountWatchlist` in
  `queries/watchlist.sql` for `/status`'s `watchlist_size` — regenerate and run
  `make sqlc-check` locally (no CI counterpart).

**`/status` API (contract frozen here)**

- **D-11 (adjusted): `/status` returns** — `runs` (last per source), `history` (last N per
  source, newest first), a per-source skip signal (`last_skipped_at`, `consecutive_skips`),
  `watchlist_size`, `poll_interval`, and `instance` (`app_version`, `schema_applied`,
  `schema_expected`). Empty `runs`/`history` on a fresh instance is a valid `200`, not an
  error. Key names, `poll_interval` encoding, and whether `instance` reuses `/ready`'s key
  names — planner's call, kept internally consistent and **documented for Phase 19**
  (contract-freeze point).
- **D-12 (unchanged): `/status` leaks nothing** — counts, timestamps, enum values, the
  composed `Summary`, the app version, two integers. `401` without a session (unchanged gate
  enforcement). DB failure → raw to `httplog.SetAttrs`, fixed body.

**App version**

- **D-13 CHANGED — `${{ github.sha }}` build-arg, not the svu tag.** The pipeline builds the
  image once in `build-scan`, saves the tarball, and `release` pushes it byte-for-byte
  (07-REVIEW CR-02). The svu `next` tag is computed in `release`, *after* the build —
  injecting it would mean either computing svu inside the sensitive `build-scan` job or
  rebuilding in `release` (breaking the single-build guarantee). Instead: pass
  `--build-arg VERSION=${{ github.sha }}` on `build-scan`'s `docker/build-push-action` step
  (SHA is free, no svu, no `fetch-depth` change), `ARG VERSION` + `-ldflags "-X …=$VERSION"`
  in the Dockerfile builder stage, `"dev"` fallback. Show a short SHA in the about block.
  Touches `.github/workflows/full-pipeline.yml` + `Dockerfile` (shared-file hazard, but one
  line each, `release` job untouched).

**Concurrency**

- **D-14 MOVED to Phase 18.1.** The `runCycle` counter aggregation is the whole reason for
  the split. This phase's `pollruns.Store` mutex is trivial and inspectable; the fan-out
  counter correctness lives in 18.1 with the looped invariant test.

**`events_recorded` source — LOCKED (was Claude's discretion)**

- **Widen the `EventRecorder` seam** (`DetectMusicBrainz` / `DetectDeezer`) to `(int, error)`.
  The downstream `SELECT count(*) … WHERE created_at >= started_at` alternative was rejected —
  `started_at` is the app process's wall clock and `events.created_at` is Postgres's, so
  positive app-vs-DB skew silently undercounts. The widening happens in **Phase 18.1** (it
  touches `internal/detection` + `runCycle`); this phase's `RunResult` just carries the
  `EventsRecorded int` field.

### Claude's Discretion (planner's call, per D-11)

- Exact JSON key names inside `runs` / `history`.
- `poll_interval` encoding (int seconds vs duration string).
- Whether `instance` reuses `/ready`'s key names.
- Ring-buffer data-structure choice (head/count ring vs bounded reslice).
- `RWMutex` vs `Mutex` on the store.
- Where `RunResult` / `RunRecorder` / `N` are declared (package layering).
- Whether `db.ExpectedSchemaVersion()` is exported or computed-at-boot-and-passed.

### Deferred Ideas (OUT OF SCOPE)

- Everything that edits `runCycle` / `internal/detection` → **Phase 18.1**.
- SPA System view → Phase 19.
- Auto-refresh (OBS-01), "poll now" (OBS-02), paginated history (OBS-03), poll-failure
  alerting (OBS-04) — deferred.
- A dedicated `/version` endpoint — rejected; folded into `/status`'s `instance` block.
- A readiness/DB-reachability *history* log — out of scope.
- Promoting `redactDSN` / `redactError` to a shared package — not needed; nothing free-text
  is stored.
- `git describe`-style version string (needs `fetch-depth: 0` on `build-scan`) — a short SHA
  is enough for v1.4.
- `partial` outcome value, `skipped_overlap` run entries, `context.WithoutCancel` in
  `RecordRun`, the `cancelled` run entry, the concurrency-invariant test, `EventRecorder`
  widening — all Phase 18.1.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RDY-01 | `GET /ready` → `200` when DB reachable AND applied schema is current (`>= expected`) and not dirty; `503` otherwise with a minimal secret-free JSON body | New `db.ExpectedSchemaVersion()` (reuses `maxSourceVersion` over the embedded FS) + new `db.SchemaVersion(ctx, pool)` (hand-rolled pgx `SELECT version, dirty FROM schema_migrations`) + new `handleReady` mirroring `handleHealth`. Ready condition `!dirty && applied >= expected` (D-01). Body per D-03. |
| RDY-02 | `/ready` reachable unauthenticated at that exact path in both gated + inert modes; short DB timeout; shared pool (no new conn); no side effects | Register `r.Get("/ready", s.handleReady)` on the root router in **both** branches of `server.go`, exactly like `r.Get("/health", …)` at `server.go:164`. Reuse `s.db.Ping` (already the shared `*pgxpool.Pool`) + a new `SchemaVersioner` seam. Bound with `context.WithTimeout(r.Context(), 3s)` — reuse `healthPingTimeout` or a sibling const. |
| RDY-03 | `/health` keeps its exact v1.3 behaviour/contract | `handleHealth` is not touched. Golden test pins body `{"status":"ok","db":"up"}` / 200 and `{"status":"degraded","db":"down"}` / 503, `Content-Type: application/json`, 3s ping bound (`health.go:17-44`, `health_test.go`). |
| RUN-02 (skip signal only) | An overlap-skipped tick surfaces as a per-source signal (last-skipped ts + consecutive-skip count) in `/status`, not a run entry | `pollruns.Store` holds `lastSkippedAt time.Time` + `consecutiveSkips int` per source; `RecordSkip(source string)` bumps + stamps; `RecordRun` resets `consecutiveSkips` to 0. The **call** from `runCycle`'s CAS-skip branch (`poller.go:278-281`) is Phase 18.1 — this phase only builds the method + field + wires the store as `poller.RunRecorder`. |
| RUN-04 | History bounded to last N per source, compile-time constant, correct under two sources recording near-simultaneously | `const N = 50` in `internal/pollruns`; ring buffer per source; one `sync.Mutex`/`RWMutex` on the `Store`. Unit test: record 60 for each of two sources concurrently, assert exactly 50 retained per source, newest-first, looped. |
| STAT-01 | Gated `GET /status` → JSON: last run per source, last N runs, watchlist size, poll interval | `handleStatus` in `registerDataRoutes` (inherits `gate.Authenticate` + `X-Instance-Gated` for free; `GET` so CSRF-exempt). Reads `pollruns.Store` via a new `StatusStore` seam, `watchlist_size` via the new `CountWatchlist` sqlc query, `poll_interval` from `cfg.PollInterval` passed into `httpserver.New`, `instance` block from the shared schema read + `buildinfo.Version`. |
| STAT-02 | `/status` exposes counts, timestamps, enum values only — never DSN/webhook/path/raw driver error | `RunResult` carries no free-text error field (only the deterministically-composed `Summary`, D-09). DB-failure path mirrors `handleListEvents` (`events.go:124-128`): raw error → `httplog.SetAttrs`, fixed `"internal error"` 500. Golden redaction test. |
</phase_requirements>

## Summary

Phase 18 is four additive backend deliverables that never touch `runCycle` or
`internal/detection`. Every piece attaches to a seam pattern the codebase already
uses repeatedly (consumer-declared narrow interfaces + functional options), and the
only new persistent state is an in-process mutex-guarded ring buffer — no migration,
no `poll_runs` table (ADR-0001).

The risk profile is low. There is **no schema migration this phase** (the highest
migration on disk is `000007`, so `db.ExpectedSchemaVersion()` returns `7`), the one new
sqlc query (`CountWatchlist`) is a trivial `SELECT count(*)`, and the ring-buffer mutex is
inspectable by eye (the fan-out counter aggregation that actually needs `-race` is
deliberately deferred to Phase 18.1). The three real hazards are: (1) `/ready` using `==`
instead of `>=` for the version comparison (D-01, PITFALLS #5), (2) any response path
leaking raw driver text (STAT-02 / RDY-01, PITFALLS #7), and (3) the shared-CI-file edit
for app-version injection colliding with the `release` job's single-build guarantee — which
D-13's `${{ github.sha }}` build-arg approach is specifically designed to avoid.

**The `/status` JSON contract is frozen at the end of this phase.** Phase 19 types
`web/app/lib/api.ts` against the real Go response body, so the planner must pick concrete
key names, document the shape, and not leave it to a guess.

**Primary recommendation:** Build in four plan-waves — (18a) `/ready` + `db` helpers
[independent, ship first], (18b) `internal/pollruns` package + `poller.RunRecorder` seam +
no-op default [independent of 18a], (18c) `CountWatchlist` sqlc + `/status` handler +
`httpserver.New` options [depends on 18b], (18d) app-version injection (`internal/buildinfo`
+ Dockerfile `ARG` + CI `build-args`) [independent] — then a composition-root wiring commit
in `cmd/server/main.go` that ties all four together.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `/ready` readiness decision (`!dirty && applied >= expected`) | API / Backend (`httpserver.handleReady`) | Database (`schema_migrations` read via pgx) | The decision logic is HTTP-handler concern; the raw `version, dirty` read is a DB concern behind a seam. Never re-open a `database/sql` handle — use the shared `*pgxpool.Pool` (RDY-02). |
| "Expected schema version" | Database (`internal/db.ExpectedSchemaVersion`) | — | Owns the embedded migration FS + `maxSourceVersion` walk; computed once at boot, passed as a value. |
| Poll-run history storage | In-process memory (`internal/pollruns.Store`, mutex + ring buffer) | — | ADR-0001: not a DB table. Single instance, history resets on restart by design. |
| Poll-run history *production* | API/Poller boundary (`poller.RunRecorder` seam) | — | Declared in `internal/poller` (consumer), wired to the real store at the composition root. **Not called by `runCycle` this phase.** |
| `/status` JSON assembly | API / Backend (`httpserver.handleStatus`) | In-process memory + Database (watchlist count) + Build metadata | Composes three seams: `StatusStore` (ring buffer), watchlist count (sqlc), schema read (shared with `/ready`), plus the `buildinfo.Version` value. |
| `watchlist_size` | Database (`CountWatchlist` sqlc query) | — | The right house pattern is a generated query, not a hand-rolled `len()` in the handler (D-10). |
| App version string | Build / CI (Docker `ARG VERSION` + `-ldflags -X` + `build-args: VERSION=${{ github.sha }}`) | API / Backend (`internal/buildinfo.Version` read into `/status`) | The value is a build-time fact injected at link time; `"dev"` when unset. The `release` job stays untouched (it `docker load`s the exact scanned tarball). |

## Standard Stack

No new third-party dependencies. Everything is stdlib + packages already in `go.mod`.

### Core (already present — used as-is)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.x (in `go.mod`) | route registration for `/ready`, `/status` | already the router; `/health` is registered as `r.Get("/health", …)` and `/ready` mirrors it exactly `[VERIFIED: internal/httpserver/server.go:164]` |
| `github.com/go-chi/httplog/v3` | v3.x (in `go.mod`) | raw-error sink via `httplog.SetAttrs` on the request context | the established leak-free pattern — `handleHealth` uses `httplog.SetAttrs(r.Context(), slog.String("db_error", err.Error()))` `[VERIFIED: internal/httpserver/health.go:38]`; `handleListEvents` uses the same for store errors `[VERIFIED: internal/httpserver/events.go:125]` |
| `github.com/jackc/pgx/v5` (+ `pgxpool`) | v5.x (in `go.mod`) | hand-rolled `SELECT version, dirty FROM schema_migrations` on the shared pool | `*pgxpool.Pool` has `QueryRow(ctx, sql, args...) pgx.Row`; `sql.ErrNoRows` equivalent is `pgx.ErrNoRows` |
| `github.com/sqlc-dev/sqlc` (CLI) | **v1.31.1** (pinned) | generate `CountWatchlist` | `Makefile` pins `SQLC_VERSION := v1.31.1` and `sqlc-check` fails on a version mismatch `[VERIFIED: Makefile:17,116-121]` |
| `log/slog` | stdlib | structured logging | already the logging core |
| `sync` | stdlib | `sync.Mutex` / `sync.RWMutex` on `pollruns.Store` | mirrors `poller`'s own `atomic.Bool` guards and `notifier`'s `atomic.Bool` CAS |
| `container/ring` OR a plain bounded slice | stdlib | ring buffer body | see "Ring-buffer data structure" below — recommend a plain `[]RunResult` with head/count, not `container/ring` (untyped `any` elements) |

### sqlc config (verified)

```yaml
# sqlc.yaml [VERIFIED: sqlc.yaml:1-14]
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries"
    schema: "internal/db/migrations"
    gen:
      go:
        package: "sqlc"
        out: "internal/db/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_interface: true
        emit_pointers_for_null_types: true
```

`emit_interface: true` → `CountWatchlist` is added to the generated `sqlc.Querier`
interface (`internal/db/sqlc/querier.go`) and a `CountWatchlist(ctx) (int64, error)` method
lands in `internal/db/sqlc/watchlist.sql.go`. This does **not** ripple into
`internal/watchlist.Store` (a hand-written narrow interface, `service.go:99-104`, which has
`Add` / `List` / `UpdatePreferences` / `Remove` and no count) unless you deliberately widen it.

**Installation:** none.

**Version verification:** `sqlc` is pinned and guarded; no version bump. `go.mod` is
`go 1.26` `[VERIFIED: go.mod:3]` so `context.WithoutCancel` / `context.WithTimeout` are all
available (not needed this phase anyway — D-07).

## Package Legitimacy Audit

Not applicable — this phase installs **zero** external packages. Every import is stdlib or
already in `go.mod` and audited in prior phases.

## Architecture Patterns

### System Architecture Diagram

```
                         ┌──────────────────────── cmd/server/main.go (composition root) ─────────────────────┐
                         │  config.Load → logging.New → weak-pass WARN → logInstanceGateStatus                 │
                         │    → db.RunMigrations(ctx, cfg.DatabaseURL, logger)          [migrate.go]           │
                         │  ★ → expected := db.ExpectedSchemaVersion()   (once, uint=7 today)                  │
                         │    → pool := db.NewPool(ctx, cfg.DatabaseURL, mbW+dzW)                              │
                         │  ★ → runs := pollruns.NewStore()                                                    │
                         │    → detector, store, eventsStore, clients …                                        │
                         │  ★ → srv := httpserver.New(pool, store, eventsStore, sources, logger,               │
                         │            WithAuthGate(...),                                                       │
                         │            WithReadiness(db.SchemaVersionerFor(pool), expected),                    │
                         │            WithStatus(runs, watchlistCounter, db.SchemaVersionerFor(pool),          │
                         │                       expected, buildinfo.Version, cfg.PollInterval))               │
                         │    → notif := notifier.Select(...)                                                  │
                         │  ★ → pollr := poller.New(store, mb, dz, detector, notif, cfg.PollInterval, logger,  │
                         │            WithMusicBrainzWorkers(...), WithDeezerWorkers(...),                     │
                         │            poller.WithRunRecorder(runs))   ← wired, NOT yet called by runCycle      │
                         └────────────┬───────────────────────────────────────────────┬─────────────────────────┘
                                      │                                               │
              GET /ready (ungated,    ▼                                               ▼
              both branches)   ┌─────────────────────┐                     ┌──────────────────────────┐
           ───────────────────▶│ httpserver.Server   │                     │ internal/poller.Poller   │
              GET /status      │  handleHealth (∅Δ)  │                     │  runCycle (∅Δ this phase) │
              (gated Group)    │ ★ handleReady        │                     │  p.runs RunRecorder ─────┐│
           ───────────────────▶│ ★ handleStatus       │                     │   (default noopRunRec.)  ││
                               │  seams:             │                     └──────────────────────────┘│
                               │   db Pinger  ───────┼── Ping(ctx) ──▶ *pgxpool.Pool                   │
                               │ ★ SchemaVersioner ──┼── SchemaVersion(ctx) ─▶ SELECT version,dirty    │
                               │ ★ StatusStore ──────┼── Snapshot() ───┐      FROM schema_migrations   │
                               │ ★ WatchlistCounter ─┼── Count(ctx) ─┐ │                               │
                               └─────────────────────┘               │ │                               │
                                                                     │ ▼                               ▼
                                                    sqlc CountWatchlist   ┌─────────────────────────────────┐
                                                    SELECT count(*)       │ ★ internal/pollruns.Store       │
                                                    FROM watchlist        │   mu  sync.Mutex                 │
                                                                          │   sources map[string]*srcState  │
                                                                          │     ring []RunResult (cap N=50) │
                                                                          │     lastSkippedAt, skips int     │
                                                                          │   RecordRun / RecordSkip /      │
                                                                          │   Snapshot                      │
                                                                          └─────────────────────────────────┘

  Build path (D-13):  CI build-scan job: docker/build-push-action  --build-arg VERSION=${{ github.sha }}
                      Dockerfile go-build stage:  ARG VERSION=dev
                        go build -ldflags="-w -s -X <module>/internal/buildinfo.Version=$VERSION"
                      release job: docker load <scanned tarball> → tag → push   (UNCHANGED — same bytes)
```

### Component Responsibilities

| File | Change | What it owns |
|------|--------|--------------|
| `internal/db/migrate.go` | MODIFIED | `+ ExpectedSchemaVersion() (uint, error)` — `iofs.New(migrationsFS, "migrations")` + `maxSourceVersion` (existing, `migrate.go:326-341`). `+ SchemaVersion(ctx, RowQuerier) (version uint, dirty bool, err error)` — hand-rolled pgx. Optionally `+ SchemaVersionerFor(pool)` thin adapter. |
| `internal/httpserver/ready.go` | NEW | `ReadinessChecker`/`SchemaVersioner` seam + `handleReady` + `readyResponse` type + `readyCheckTimeout` const (or reuse `healthPingTimeout`). |
| `internal/httpserver/status.go` | NEW | `StatusStore` + `WatchlistCounter` seams + `handleStatus` + the frozen response envelope types + `statusInstance` type. |
| `internal/httpserver/server.go` | MODIFIED | register `/ready` on root router in both branches (next to `server.go:164`); add `/status` to `registerDataRoutes` (`server.go:197-204`); add `Server` fields + `WithReadiness` / `WithStatus` options into the existing `serverConfig` / `Option` machinery (`server.go:42-51`). |
| `internal/pollruns/pollruns.go` | NEW | `const N = 50`; `RunResult` struct; `Store` (mutex + `map[string]*sourceState`); `NewStore()`; `RecordRun(ctx, RunResult) error`; `RecordSkip(source string)`; `Snapshot()` (+ its snapshot view types). |
| `internal/poller/poller.go` | MODIFIED (additive only) | declare `RunRecorder` interface next to `Notifier` (`poller.go:99-101`); `+ p.runs RunRecorder` field on `Poller` (`poller.go:136-169`); `+ noopRunRecorder{}` default; `+ WithRunRecorder(RunRecorder) Option` (mirror `WithMusicBrainzWorkers`, `poller.go:113-115`). **`runCycle` and both cycle methods untouched.** |
| `internal/buildinfo/buildinfo.go` | NEW | `var Version = "dev"` + (optional) `func Short() string` truncating to 12 chars. |
| `cmd/server/main.go` | MODIFIED | `expected := db.ExpectedSchemaVersion()` after `db.RunMigrations` (`main.go:137`); `runs := pollruns.NewStore()`; pass `WithReadiness` + `WithStatus` into `httpserver.New` (`main.go:235`); pass `poller.WithRunRecorder(runs)` into `poller.New` (`main.go:258`); read `buildinfo.Version`. |
| `queries/watchlist.sql` | MODIFIED | `+ -- name: CountWatchlist :one` / `SELECT count(*) FROM watchlist;` |
| `internal/db/sqlc/*` | REGENERATED | `sqlc generate` → `watchlist.sql.go` + `querier.go` + `models.go` diff; commit; `make sqlc-check` green locally. |
| `.github/workflows/full-pipeline.yml` | MODIFIED | `build-scan` job → `docker/build-push-action` step (`full-pipeline.yml:576`) gains `build-args: VERSION=${{ github.sha }}`. **`release` job untouched.** |
| `Dockerfile` | MODIFIED | `go-build` stage: `ARG VERSION=dev` before the `go build` RUN (`Dockerfile:60-61`); `-ldflags="-w -s -X <module>/internal/buildinfo.Version=$VERSION"`. Update the file's top comment (`Dockerfile:15-19`) to carve out the build-info exception (see Pitfall 6). |

### Pattern 1: Consumer-declared narrow seam + no-op default (mirror `notifier.NoOp`)

**What:** the consuming package declares the minimal interface it needs; the concrete type
is injected at the composition root; an always-non-nil no-op default means the call site
never nil-checks.

**Verified precedent — `poller.Notifier`:**
```go
// [VERIFIED: internal/poller/poller.go:99-101]
type Notifier interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
}
```
```go
// [VERIFIED: internal/notifier/notifier.go:57-62]
// NoOp is D-10's inert Sink, returned by Select when DISCORD_WEBHOOK_URL is
// unset, so poller.go's Notifier seam is always non-nil.
type NoOp struct{}
func (NoOp) NotifyPending(ctx context.Context, logger *slog.Logger) error { return nil }
```
`poller.New` takes `notifier` as a positional arg and it is "always non-nil (a real one, or
notifier.NoOp), so neither cycle method ever nil-checks this field"
`[VERIFIED: internal/poller/poller.go:95-98]`.

**Apply to `RunRecorder`:** because Phase 18 does **not** add `RunRecorder` as a positional
`New` param (that would touch every `poller.New` call site — 40+ in `poller_test.go`), use a
functional option with a private no-op default:
```go
// in internal/poller/poller.go, next to Notifier
type RunRecorder interface {
	RecordRun(ctx context.Context, result pollruns.RunResult) error
	RecordSkip(source string)
}

type noopRunRecorder struct{}
func (noopRunRecorder) RecordRun(context.Context, pollruns.RunResult) error { return nil }
func (noopRunRecorder) RecordSkip(string) {}

var _ RunRecorder = noopRunRecorder{}

// on Poller struct:  runs RunRecorder
// in New, after building p:  p.runs = noopRunRecorder{}
// then opts loop applies WithRunRecorder if present (poller.go:196-198)

func WithRunRecorder(r RunRecorder) Option {
	return func(p *Poller) { if r != nil { p.runs = r } }
}
```

### Pattern 2: Functional options keep `New` additive (verified — critical here)

`httpserver.New` has **~60 call sites across the test suite** (`events_test.go`,
`watchlist_test.go`, `search_test.go`, `spa_test.go`, `server_test.go`, `boot_e2e_test.go`).
Its signature is already variadic:
```go
// [VERIFIED: internal/httpserver/server.go:106]
func New(db Pinger, store watchlist.Store, eventsStore events.Store, sources []SearchSource, logger *slog.Logger, opts ...Option) *Server {
```
```go
// [VERIFIED: internal/httpserver/server.go:42-51]
type serverConfig struct {
	gatePassphrase    string
	gateAlerter       authgate.Alerter
	trustProxyHeaders bool
}
type Option func(*serverConfig)
```
**Add `WithReadiness(...)` and `WithStatus(...)` as new `Option`s.** Every existing
`httpserver.New(...)` call compiles unchanged. When an option is absent, the handler returns
`503 {"reason":"not_configured"}` (for `/ready`) or `500`/`503` (for `/status`) so the route
table stays byte-for-byte identical in both cases and the "gated vs inert" branch structure
is untouched. `poller.Option` is the same shape (`poller.go:108`, applied at `poller.go:196-198`).

### Pattern 3: `/health`-style structural route exemption (verified)

```go
// [VERIFIED: internal/httpserver/server.go:161-182]
// /health is exempt as an exact registered path (D-03): chi matches the
// literal "/health", never a prefix ...
r.Get("/health", s.handleHealth)

if gate != nil {
	r.Post("/session", gate.HandleLogin)
	r.Delete("/session", gate.HandleLogout)
	r.Group(func(pr chi.Router) {
		pr.Use(gate.Authenticate)
		pr.Use(gate.RequireCSRFHeader)
		registerDataRoutes(pr, s)
	})
} else {
	registerDataRoutes(r, s)
}
```
`/ready` goes on the line **immediately after `r.Get("/health", …)`** — that line is outside
`if gate != nil`, so it is registered once and is identical in both branches. Never add it
to `registerDataRoutes` and never path-match it in gate middleware.

`registerDataRoutes` is where `/status` goes:
```go
// [VERIFIED: internal/httpserver/server.go:197-204]
func registerDataRoutes(r chi.Router, s *Server) {
	r.Get("/search", s.handleSearch)
	r.Post("/watchlist", s.handleAddWatchlist)
	r.Get("/watchlist", s.handleListWatchlist)
	r.Patch("/watchlist/{id}", s.handleUpdateWatchlist)
	r.Delete("/watchlist/{id}", s.handleRemoveWatchlist)
	r.Get("/events", s.handleListEvents)
}
```
Adding `r.Get("/status", s.handleStatus)` here means `/status` inherits `gate.Authenticate`
+ `gate.RequireCSRFHeader` + the `X-Instance-Gated` header on the gated path, and is served
flat + unauthenticated on the inert path — exactly like `/events` today. `GET` → CSRF-exempt.

### Pattern 4: `handleHealth` is the exact template for `handleReady` (verified)

```go
// [VERIFIED: internal/httpserver/health.go:13-44]
const healthPingTimeout = 3 * time.Second

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
	defer cancel()

	resp := healthResponse{Status: "ok", DB: "up"}
	status := http.StatusOK

	if err := s.db.Ping(ctx); err != nil {
		resp.Status, resp.DB = "degraded", "down"
		status = http.StatusServiceUnavailable
		httplog.SetAttrs(r.Context(), slog.String("db_error", err.Error()))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
```
`handleReady` is this plus: after a successful `Ping`, call `s.schema.SchemaVersion(ctx)`;
map `(err != nil) → db_unreachable`, `(dirty) → schema_dirty`, `(applied < expected) →
schema_behind`; otherwise `ready`. `pgx.ErrNoRows` (empty `schema_migrations`) → treat as
`schema_behind` with `schema_applied: null` (see State of the Art note — this cannot happen
given the boot order but must be handled). Raw schema error → `httplog.SetAttrs` only.

### Ring-buffer data structure (Claude's discretion — recommendation)

Three options, all correct for N=50:
1. **Head/count ring on a fixed `[N]RunResult`** — `buf [N]RunResult; head, count int`. Zero
   allocation after construction, O(1) append, unambiguous. `Snapshot` walks `count` entries
   newest-first from `(head-1+N)%N`. **Recommended** — most inspectable, no backing-array
   surprises.
2. **Bounded slice + reslice** — `append`, then `if len(b) > N { b = b[len(b)-N:] }`. Simple,
   but the backing array slides forward; memory stays bounded at ~2N because `append`
   reallocates+compacts once the sliding offset exhausts capacity. Acceptable, slightly
   subtle.
3. `container/ring` — rejected: elements are `any`, forcing a type assertion on every read
   and losing compile-time safety for no benefit.

Whichever you pick, `Snapshot` **must deep-copy** the slice it returns (a fresh
`[]RunResult`) while holding the lock — handing the handler a slice that aliases the ring is
a data race the moment the next cycle appends. `RunResult` is all value types (strings,
ints, `time.Time`), so a plain slice copy is a full deep copy.

### Package layering for `RunResult` / `RunRecorder` / `N` (Claude's discretion — recommendation)

**Recommended:** `RunResult`, `N`, and `Store` all live in `internal/pollruns`. `poller`
declares the `RunRecorder` *interface* (consumer-declared seam) and imports `internal/pollruns`
**only for the `RunResult` type** — exactly as `poller` already imports `internal/musicbrainz`
and `internal/deezer` purely for the DTO types in its `ReleaseGroupSource` / `AlbumSource` /
`EventRecorder` seams (`poller.go:28-30, 66-89`). `internal/httpserver` imports
`internal/pollruns` for the `Snapshot` view types. Dependency DAG stays acyclic:
`poller → pollruns`, `httpserver → pollruns`, `main → {poller, httpserver, pollruns}`.
`pollruns` imports nothing from this repo.

**Alternative** (also fine): put `RunResult` in `poller` next to the `RunRecorder`
interface, `pollruns` imports `poller`. Downside: `httpserver` then transitively depends on
`poller` (pulls `robfig/cron` into its dep tree). Prefer the recommended layout.

### Anti-Patterns to Avoid

- **`/ready` version check with `==`.** D-01 / PITFALLS #5. A cleanly rolled-back binary
  (embedded max 6, DB at 7) is *ready*. Use `applied >= expected`.
- **`/ready` re-walking the embedded FS per request.** Call `db.ExpectedSchemaVersion()`
  **once** at boot in `main.go`; pass the `uint`. `iofs.New` + `maxSourceVersion` allocate.
- **`/ready` opening a fresh `database/sql` handle** (what `runMigrationsOnce` does,
  `migrate.go:272-277`). Use the shared `*pgxpool.Pool` already handed to `httpserver.New`.
- **`/ready` inside the gate `Group`** → 401 to every uptime monitor + the future Phase 17
  deploy gate. Root router, both branches, exact path.
- **A free-text `error` / `last_error` / `detail` string anywhere on `RunResult` or the
  `/status` envelope.** D-09 / D-12 / STAT-02. Only the composed `Summary` string, and it is
  built from counts + the outcome enum, never from `err.Error()`.
- **Widening `watchlist.Store` for the count.** It has ~5 stub implementations in
  `httpserver` tests. Use a separate `WatchlistCounter` seam or add a `watchlist.Service`
  method behind a new narrow interface.
- **Adding `RunRecorder` as a positional `poller.New` param.** 40+ test call sites. Option only.
- **Calling `p.runs.RecordRun` / `RecordSkip` from `runCycle` this phase.** That is Phase
  18.1's entire scope. The seam is wired but inert.
- **A Postgres trigger or a `poll_runs` table.** ADR-0001 — designed out.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Read the applied migration version | `migrate.NewWithInstance(...).Version()` | hand-rolled `SELECT version, dirty FROM schema_migrations` on the shared `*pgxpool.Pool` | `migrate.NewWithInstance` opens a fresh `database/sql` conn every call (`migrate.go:272-291`); the table is a documented single-row `(version bigint, dirty boolean)` shape `[VERIFIED: web — golang-migrate schema_migrations]` — a 1-row primary-key-less read is trivial |
| "Expected" migration version | a hand-typed `const expectedSchema = 7` | `db.ExpectedSchemaVersion()` reusing `maxSourceVersion(src)` over `migrationsFS` | a literal drifts silently the next time a migration is added; the walk is already written and battle-tested (`migrate.go:326-341`, used by the ahead-of-source guard) |
| `watchlist_size` | `len(store.List(ctx))` in the handler | `CountWatchlist` sqlc query (`SELECT count(*) FROM watchlist`) | `List` does a `JOIN artists` and returns every row's full projection (`queries/watchlist.sql:14-19`) — pulling N rows to count them; a `count(*)` is one integer. D-10 mandates the query. |
| JSON encoding | manual string building | `json.NewEncoder(w).Encode(...)` | every handler in the package does this (`health.go:43`, `events.go:146`) |
| Bounded history | a slice you manually `[1:]` at every call site | one `pollruns.Store` method that owns the bound | RUN-04's "compile-time constant, correct under two sources" is a single-owner invariant |
| Ring buffer | `container/ring` | a typed head/count ring or bounded reslice | `container/ring` is `any`-typed; loses compile-time safety |
| Redaction of driver errors on the `/status` / `/ready` error path | promoting `redactDSN`/`redactError` | store no free-text at all; raw error → `httplog.SetAttrs`; fixed body | D-12: "No `redactDSN`/`redactError` promotion needed because nothing free-text is stored" |

**Key insight:** the codebase has already solved every sub-problem here — the phase is
almost entirely *composition* of existing patterns. The only genuinely new primitive is the
mutex-guarded ring buffer, and that is ~40 lines.

## Runtime State Inventory

This is an additive feature phase, not a rename/refactor/migration. **No runtime state
inventory applies.** For completeness against the checklist:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — the ring buffer is in-process and resets on restart by design (ADR-0001). No new DB rows (no migration). | None |
| Live service config | None — no new external service, no new cron entry, no config file. | None |
| OS-registered state | None. | None |
| Secrets/env vars | **None added.** RUN-04 explicitly forbids a retention env var; `app_version` is a build arg, not runtime config; `poll_interval` reuses the existing `POLL_INTERVAL`. | None |
| Build artifacts / installed packages | `internal/db/sqlc/*.go` is regenerated (new `CountWatchlist`); the Docker image gains an embedded `buildinfo.Version` string. Neither is stale-prone: `make sqlc-check` catches sqlc drift locally, and the image is rebuilt every CI run. | Run `sqlc generate` + commit; run `make sqlc-check` before pushing. |

**Nothing found in any category that requires a data migration.** Verified by: no migration
file added (highest on disk stays `000007` — `[VERIFIED: internal/db/migrations/ listing]`);
`grep` for env-var additions in `internal/config/config.go` — the `Config` struct is
unchanged this phase.

## Common Pitfalls

### Pitfall 1: `/ready` uses `==` instead of `>=` for the version comparison

**What goes wrong:** a binary rolled back to an older embedded max, serving correctly
against a newer additive schema (the exact scenario Phase 16's ahead-of-source guard
supports — `migrate.go:298-302`), reports `503 schema_behind` forever. When Phase 17's
health-gated rollback is un-deferred, it polls `/ready` and concludes the rollback failed.
**Why it happens:** "at the expected version" reads naturally as equality. **How to avoid:**
ready condition is `!dirty && applied >= expected` (D-01). Document the Phase-16 rationale in
the handler comment. **Warning signs:** an `==` in the handler; `expected` as a literal;
`/ready` 503 while `/health` is 200. **Test:** a case where `applied = expected + 1` asserts
`200 ready`.

### Pitfall 2: A response path leaks raw driver / DSN / webhook text

**What goes wrong:** `/ready` or `/status` returns `err.Error()` in the JSON body on a DB
failure — pgx connection errors embed the DSN (with password); that is the entire reason
`redactDSN`/`redactError` exist (`migrate.go:114-194`). `/status` is gated but a weak
passphrase (the boot WARN path, `main.go:124`) still gets in. **Why it happens:** the
redaction helpers are unexported in `internal/db`; `err.Error()` into a field is the path of
least resistance. **How to avoid:** (a) `RunResult` carries **no** free-text error field —
only `Summary`, composed from counts + the outcome enum (D-09); (b) every handler's
DB-failure path is `httplog.SetAttrs(r.Context(), slog.String("<x>_error", err.Error()))` +
a fixed body, exactly like `handleHealth` (`health.go:38`) and `handleListEvents`
(`events.go:125-127`). **Warning signs:** any `error` / `message` / `detail` / `last_error`
string field in a response struct; `err.Error()` reaching an `Encode` call; the strings
`://` or `password=` in a test fixture. **Test:** feed a DSN-bearing and a webhook-bearing
error through the `/status` store seam, assert neither string appears in the response body
(golden test in the spirit of `TestHealth_Down`'s leak loop, `health_test.go:113-118`).

### Pitfall 3: `/status` empty-history path is treated as an error

**What goes wrong:** a fresh instance (every deploy, and this phase ships with `runCycle`
never calling `RecordRun`) has an empty ring buffer. A handler that assumes a last-run
exists returns 500 or a malformed body. **Why it happens:** dev always has history within a
poll interval. **How to avoid:** empty `runs` / `history` is a valid `200` (D-11) — encode
`runs` as `{}` (or per-source `null`) and `history` as `{}` or `{"musicbrainz":[],"deezer":[]}`.
The `instance` block, `watchlist_size`, and `poll_interval` are always populated. **Test:**
`handleStatus` with a fresh `pollruns.NewStore()` → assert `200` and a decodable body with
empty run collections + a populated `instance`.

### Pitfall 4: `Snapshot()` returns a slice aliasing the ring buffer

**What goes wrong:** the handler ranges over the returned slice while the next poll cycle
appends to the same backing array → data race + torn reads. **Why it happens:** returning
`s.sources[src].ring` directly under the lock "looks" safe because the lock is held during
the return. **How to avoid:** copy into a fresh `[]RunResult` inside the critical section;
return the copy. `RunResult` is all value types so `copy(dst, src)` is a full deep copy.
**Warning signs:** `Snapshot` returns a field directly; no `make([]RunResult, ...)` in it.

### Pitfall 5: `CountWatchlist` sqlc drift ships green

**What goes wrong:** you add `-- name: CountWatchlist` to `queries/watchlist.sql` but forget
`sqlc generate`, or generate with the wrong sqlc version. CI stays green — there is **no CI
sqlc check** (`Makefile:126-128` `sqlc-check` is local-only, confirmed by the CLAUDE.md
Definition of Done and the Makefile comment `[VERIFIED: Makefile:19-21]`). **How to avoid:**
`make sqlc-check` (which runs `sqlc-version-check` then `sqlc generate` then
`git diff --exit-code -- internal/db/sqlc/`) before every commit that touches `queries/`.
Commit the regenerated `watchlist.sql.go`, `querier.go`, and any `models.go` delta together
with the `.sql` change. **Warning signs:** an uncommitted `internal/db/sqlc/` diff; a
`CountWatchlist` method that doesn't exist on `sqlc.Querier`.

### Pitfall 6: The Dockerfile forbids `ARG` carrying a value

**What goes wrong:** the Dockerfile's top comment is explicit
`[VERIFIED: Dockerfile:15-19]`: *"No ENV or ARG instruction anywhere in this file may carry
a configuration value ... a baked value here would both break that invariant and risk a
secret landing in a committed image layer."* A reviewer will bounce `ARG VERSION` against
that rule unless the intent is made explicit. **Why it happens:** the rule was written for
*runtime configuration*; a build-commit SHA is neither runtime config nor a secret, but the
comment says "any ... configuration value." **How to avoid:** in the same commit that adds
`ARG VERSION=dev`, amend that comment block to carve out the exception — "except build
provenance metadata (`VERSION`), which is a non-secret build-time fact injected into the
binary via `-ldflags -X`, never read at runtime as configuration." D-13 already sanctions
this; the code comment must agree. **Warning signs:** a diff adding `ARG` with no
corresponding comment update; CI Trivy flagging a new image layer (it won't for a SHA, but
verify).

### Pitfall 7: `-ldflags -X` path is wrong → silently ignored

**What goes wrong:** `-X` takes `importpath.name`; if the import path is wrong the linker
**silently ignores it** and `Version` stays `"dev"` on a CI image `[VERIFIED: web —
"If you get the path wrong, there's no error or warning"]`. **How to avoid:** the module
path is `github.com/danielrpof/drop-tracker` `[VERIFIED: internal/httpserver/server.go:14]`,
so the flag is `-X 'github.com/danielrpof/drop-tracker/internal/buildinfo.Version=$VERSION'`.
**Test:** a CI smoke assertion (or a manual check on the first built image) that
`GET /status` `instance.app_version` is a 40-hex SHA, not `"dev"`, on a `build-scan`-built
image. Add a `buildinfo` unit test that the zero value is `"dev"`.

### Pitfall 8: `/ready` on a gated instance returns 401

**What goes wrong:** `/ready` accidentally registered inside the gate `Group` (or gated via
a path allowlist in middleware) → uptime monitors and the deploy gate get 401. **How to
avoid:** register on the root router in both branches (Pattern 3). **Test:** build a server
with `httpserver.WithAuthGate("passphrase", false, nil)` (the `newGatedServer` helper,
`server_test.go:229-236`), `GET /ready` with no cookie, assert the status is **not** 401
(it is 200 or 503 depending on the fake schema/ping state).

## Code Examples

### `db.ExpectedSchemaVersion` (reuse the existing source wiring)

```go
// internal/db/migrate.go — new exported func. Reuses the SAME iofs source
// RunMigrations uses (migrate.go:209) and the SAME walk the ahead-of-source
// guard uses (migrate.go:299), so the number can never drift from what
// RunMigrations would apply.
func ExpectedSchemaVersion() (uint, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return 0, fmt.Errorf("load embedded migrations: %w", err)
	}
	defer func() { _ = src.Close() }()
	v, ok := maxSourceVersion(src) // [VERIFIED: internal/db/migrate.go:326-341]
	if !ok {
		return 0, errors.New("db: embedded migration source is empty")
	}
	return v, nil
}
```
`maxSourceVersion` verbatim behaviour: `"walks src to its highest migration version,
returning (0, false) if the source is empty ... relies only on the documented
os.ErrNotExist end-of-source signal from source.Driver.Next"`
`[VERIFIED: internal/db/migrate.go:320-341]`. Today it returns `7`
`[VERIFIED: internal/db/migrations/ — highest file is 000007_backfill_events_watched_artist_name.up.sql]`.

### `db.SchemaVersion` (hand-rolled pgx read of the golang-migrate table)

```go
// internal/db/schema_version.go — new file.
// schema_migrations is golang-migrate-owned (created outside internal/db/migrations/,
// so sqlc never sees it). It is a single-row table: (version bigint, dirty boolean).
// [VERIFIED: web — golang-migrate schema_migrations: "version | dirty ... 3 | f (1 row)"]

// RowQuerier is the minimal surface SchemaVersion needs; *pgxpool.Pool satisfies it.
type RowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func SchemaVersion(ctx context.Context, q RowQuerier) (version uint, dirty bool, err error) {
	var v int64
	err = q.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&v, &dirty)
	if errors.Is(err, pgx.ErrNoRows) {
		// Empty table = golang-migrate created it but no migration is recorded.
		// Cannot happen given main.go's boot order (RunMigrations runs first and
		// always leaves a row), but handle it: applied version 0.
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read schema_migrations: %w", err)
	}
	return uint(v), dirty, nil
}
```
The dirty column: `"FALSE when migrations run successfully; when a migration fails to
execute, the dirty column has the value true"` `[VERIFIED: web — golang-migrate
schema_migrations]`. golang-migrate's own `ErrNilVersion` maps to "no row" here.

### `handleReady` (mirror `handleHealth` exactly)

```go
// internal/httpserver/ready.go — new file.

type SchemaVersioner interface {
	SchemaVersion(ctx context.Context) (version uint, dirty bool, err error)
}

// readyResponse — D-03 locked shape. reason is "" (omitted) on 200.
type readyResponse struct {
	Status         string  `json:"status"`                   // "ready" | "not_ready"
	SchemaApplied  *uint   `json:"schema_applied"`           // null when DB unreachable
	SchemaExpected uint    `json:"schema_expected"`
	Reason         string  `json:"reason,omitempty"`         // db_unreachable|schema_behind|schema_dirty
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout) // reuse the 3s const
	defer cancel()

	resp := readyResponse{Status: "ready", SchemaExpected: s.expectedSchema}
	code := http.StatusOK

	fail := func(reason string, applied *uint) {
		resp.Status, resp.Reason, resp.SchemaApplied = "not_ready", reason, applied
		code = http.StatusServiceUnavailable
	}

	switch {
	case s.schema == nil: // WithReadiness not supplied
		fail("not_configured", nil)
	default:
		if err := s.db.Ping(ctx); err != nil {
			httplog.SetAttrs(r.Context(), slog.String("ready_db_error", err.Error()))
			fail("db_unreachable", nil)
			break
		}
		applied, dirty, err := s.schema.SchemaVersion(ctx)
		if err != nil {
			httplog.SetAttrs(r.Context(), slog.String("ready_schema_error", err.Error()))
			fail("db_unreachable", nil)
			break
		}
		a := applied
		resp.SchemaApplied = &a
		switch {
		case dirty:
			fail("schema_dirty", &a)
		case applied < s.expectedSchema: // D-01: >= is ready, NOT ==
			fail("schema_behind", &a)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}
```

### `/status` response envelope — RECOMMENDED FROZEN CONTRACT (Phase 19 types against this)

```jsonc
// GET /status  →  200 (gated: 401 without a valid session)
{
  "poll_interval_seconds": 900,          // int; cfg.PollInterval.Seconds() truncated to int
  "watchlist_size": 12,                  // int; CountWatchlist
  "instance": {
    "app_version": "a1b2c3d4e5f6",       // short SHA (<=12 chars) on a CI image, "dev" locally
    "schema_applied": 7,                  // int, null only if DB unreachable at request time
    "schema_expected": 7                  // int
  },
  "sources": {
    "musicbrainz": {
      "last_run": {                      // null on a fresh instance
        "cycle_id": "musicbrainz-42",
        "started_at": "2026-09-09T12:00:00Z",
        "finished_at": "2026-09-09T12:00:03Z",
        "duration_ms": 3120,
        "artists_checked": 12,           // entries dispatched to a worker
        "artists_skipped": 0,
        "artists_errored": 1,
        "events_recorded": 3,
        "outcome": "ok",                 // "ok" | "error" | "cancelled"
        "summary": "ok — 12 checked, 1 errored, 3 events"
      },
      "history": [ /* newest-first, <= 50 RunResult objects, same shape as last_run */ ],
      "last_skipped_at": null,           // RFC3339 string or null
      "consecutive_skips": 0             // int
    },
    "deezer": { "last_run": null, "history": [], "last_skipped_at": null, "consecutive_skips": 0 }
  }
}
```

**Rationale for this shape (planner may adjust key names but SHOULD lock and document):**
- `sources` as an object keyed by the source string (`"musicbrainz"` / `"deezer"` — the
  existing consts `sourceMusicBrainz` / `sourceDeezer`, `poller.go:45-46`) — groups
  `last_run` + `history` + the skip signal per source in one place, so Phase 19 renders one
  card per source without cross-referencing three top-level maps. CONTEXT D-11 names `runs`
  and `history` as separate keys; this folds them under `sources` which is internally
  cleaner. **If the planner prefers the literal D-11 shape**, use top-level `runs` (map
  source→RunResult|null), `history` (map source→[]RunResult), `skips` (map
  source→{last_skipped_at, consecutive_skips}) — either is acceptable per D-11, just freeze
  one.
- `poll_interval_seconds` as an int, not a Go duration string (`"15m0s"`) — a machine
  consumer and a JS frontend both want a number; `time.Duration.Seconds()` → truncate to
  int. `cfg.PollInterval` default is `15m` `[VERIFIED: internal/config/config.go:24]` →
  `900`. (Note CONTEXT prose says "30–60 min"; the actual configured default is 15m.)
- `instance` uses `schema_applied` / `schema_expected` — the **same key names as `/ready`'s
  D-03 body** (D-11 leaves this to the planner; reusing them means Phase 19 has one
  schema-version concept, not two).
- No `error` / `detail` field anywhere (STAT-02).

### `handleStatus` error handling (mirror `handleListEvents`)

```go
// internal/httpserver/status.go
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	count, err := s.watchlistCounter.Count(ctx)
	if err != nil {
		httplog.SetAttrs(ctx, slog.String("status_watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error") // [VERIFIED: events.go:126]
		return
	}

	// schema read is best-effort: /status still returns 200 with schema_applied:null
	// if the DB blips, because the ring buffer + counts are the payload's point.
	var appliedPtr *uint
	if a, _, verr := s.schema.SchemaVersion(ctx); verr == nil {
		appliedPtr = &a
	} else {
		httplog.SetAttrs(ctx, slog.String("status_schema_error", verr.Error()))
	}

	snap := s.statusStore.Snapshot() // in-memory, cannot fail

	resp := buildStatusResponse(count, appliedPtr, s.expectedSchema, buildinfo.Version, s.pollInterval, snap)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
```
`writeError` is the existing shared helper (used at `events.go:83,126`).

### `internal/pollruns` skeleton

```go
package pollruns

import (
	"context"
	"sync"
	"time"
)

// N bounds the per-source history ring. 50 ≈ 1–2 days at a 15–60 min poll
// interval (RUN-04: compile-time constant, deliberately not an env var).
const N = 50

// RunResult is the immutable summary of one completed poll cycle. Composed
// entirely from counts + the outcome enum at record time — never from a
// driver error, upstream response, DSN, webhook URL, or path (D-09/D-12).
type RunResult struct {
	Source         string    // sourceMusicBrainz | sourceDeezer
	CycleID        string    // "<source>-<n>", the existing correlation id
	StartedAt      time.Time
	FinishedAt     time.Time
	DurationMS     int64
	ArtistsChecked int // entries dispatched to a worker (post shouldDispatch)
	ArtistsSkipped int // entries skipped before dispatch (Deezer nil-DeezerID)
	ArtistsErrored int
	EventsRecorded int
	Outcome        string // "ok" | "error" | "cancelled"
	Summary        string // e.g. "ok — 12 checked, 1 errored, 3 events"
}

type sourceState struct {
	ring             []RunResult // newest-last; len <= N
	lastSkippedAt    time.Time   // zero value = never skipped
	consecutiveSkips int
}

type Store struct {
	mu      sync.Mutex
	sources map[string]*sourceState
}

func NewStore() *Store { return &Store{sources: make(map[string]*sourceState)} }

// RecordRun appends r to its source's ring (trimming to N) and resets that
// source's consecutive-skip counter. ctx is accepted for seam symmetry with
// EventRecorder/Notifier; the append never blocks on it (D-07).
func (s *Store) RecordRun(_ context.Context, r RunResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stateLocked(r.Source)
	st.ring = append(st.ring, r)
	if len(st.ring) > N {
		st.ring = st.ring[len(st.ring)-N:]
	}
	st.consecutiveSkips = 0
	return nil
}

// RecordSkip bumps the source's consecutive-skip count and stamps the
// last-skipped time. No run entry is created (D-05 superseded: a skipped
// tick is a signal, not a run).
func (s *Store) RecordSkip(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stateLocked(source)
	st.consecutiveSkips++
	st.lastSkippedAt = time.Now()
}

func (s *Store) stateLocked(source string) *sourceState {
	st := s.sources[source]
	if st == nil {
		st = &sourceState{}
		s.sources[source] = st
	}
	return st
}

// SourceSnapshot is a race-free copy handed to the /status handler.
type SourceSnapshot struct {
	LastRun          *RunResult
	History          []RunResult // newest-first copy
	LastSkippedAt    *time.Time
	ConsecutiveSkips int
}

func (s *Store) Snapshot() map[string]SourceSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]SourceSnapshot, len(s.sources))
	for src, st := range s.sources {
		hist := make([]RunResult, len(st.ring))
		for i, r := range st.ring { // reverse into newest-first
			hist[len(st.ring)-1-i] = r
		}
		snap := SourceSnapshot{History: hist, ConsecutiveSkips: st.consecutiveSkips}
		if len(hist) > 0 {
			lr := hist[0]
			snap.LastRun = &lr
		}
		if !st.lastSkippedAt.IsZero() {
			t := st.lastSkippedAt
			snap.LastSkippedAt = &t
		}
		out[src] = snap
	}
	return out
}
```

## State of the Art

| Old Approach (superseded) | Current Approach | When Changed | Impact |
|---------------------------|------------------|--------------|--------|
| `poll_runs` Postgres table + sqlc + prune-on-insert CTE (v1.4 milestone research: ARCHITECTURE.md §(d), PITFALLS #2) | In-process ring buffer in `internal/pollruns` (ADR-0001) | 2026-09-09 design grilling | No migration, no `sqlc` for run history, no prune race, no cross-source deadlock. Only `CountWatchlist` sqlc query remains. |
| Skip-overlap coalesced into a run entry with `outcome='skipped_overlap'` (CONTEXT D-05, orig REQUIREMENTS RUN-02) | Separate per-source signal (`last_skipped_at` + `consecutive_skips`), no run entry | 2026-09-09 grilling | `RUN-02` reworded. The store field lands this phase; the `RecordSkip` call is 18.1. |
| `RecordRun` with `context.WithoutCancel(ctx)` + `recordRunTimeout` for shutdown survival (CONTEXT D-07, ARCHITECTURE Pattern 5) | Synchronous in-memory mutex append; `ctx` is vestigial in the signature | 2026-09-09 grilling | No detached-context dance. A shutdown-cancelled cycle records instantly. |
| App version = the svu `next` tag via `-ldflags` (CONTEXT D-13 original) | `--build-arg VERSION=${{ github.sha }}` in `build-scan`; svu tag computed later in `release` | 2026-09-09 grilling | Preserves the single-build guarantee (07-REVIEW CR-02); `release` job untouched; About block shows a SHA, not the semver tag. |
| `EventRecorder` returns `error`; `events_recorded` via downstream `SELECT count(*)` | Widen `EventRecorder` to `(int, error)` — **in Phase 18.1** | 2026-09-09 grilling | Clock-skew undercount rejected. This phase's `RunResult` just carries the `int` field, always 0 until 18.1. |

**golang-migrate `schema_migrations` semantics (verified this session):**
- Single row, columns `version bigint`, `dirty boolean`
  `[VERIFIED: web — golang-migrate schema_migrations table structure]`.
- `dirty = true` iff a migration failed mid-apply; blocks further migrations
  `[VERIFIED: web]`.
- Empty table (no row) = golang-migrate's `ErrNilVersion` state = fresh DB before any
  migration recorded. **In this codebase this state is unreachable at HTTP-serve time**
  because `db.RunMigrations` runs to completion before `db.NewPool` and `httpserver.New`
  (`main.go:137` → `main.go:146` → `main.go:235`) `[VERIFIED: cmd/server/main.go:137-238]`.
  Handle it anyway (applied = 0 → `schema_behind`).
- The ahead-of-source guard already reads this table: `m.Version()` returns
  `(cur, dirty, verr)` and the guard no-ops when `verr == nil && !dirty && cur > smax`
  `[VERIFIED: internal/db/migrate.go:298-302]` — so `/ready`'s `applied > expected` case is
  a *supported, healthy* state, reinforcing D-01's `>=`.

**Go `-ldflags -X` + Docker build-arg (verified this session):**
- `-X importpath.name=value` replaces a package-level `string` var at link time
  `[VERIFIED: web — Go FAQ / Alex Ellis]`.
- **A wrong import path is silently ignored — no error** `[VERIFIED: web]`. Use the full
  module path `github.com/danielrpof/drop-tracker/internal/buildinfo.Version`.
- Standard pattern: `ARG VERSION` (stage-scoped) → `go build -ldflags "-X ...=$VERSION"` →
  `docker build --build-arg VERSION=...` `[VERIFIED: web — multiple sources]`.
- The current Dockerfile flags are `-trimpath -ldflags="-w -s"`
  `[VERIFIED: Dockerfile:60-61]`; append the `-X` inside the same quoted `-ldflags` string.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `*pgxpool.Pool` satisfies a `QueryRow(ctx, sql, args...) pgx.Row` interface for the hand-rolled `schema_migrations` read (not just `Ping`). | Code Examples / `db.SchemaVersion` | LOW — pgxpool.Pool has had `QueryRow` since pgx v4; if the exact signature differs, the adapter in `main.go` wraps it. Verify against the pinned pgx version at implementation time (`go doc github.com/jackc/pgx/v5/pgxpool.Pool.QueryRow`). |
| A2 | The frozen `/status` envelope key names in this doc are acceptable to the planner and to Phase 19. | Code Examples / frozen contract | MEDIUM — this is explicitly the planner's call (D-11). The planner must lock names and write them into the plan / a contract doc before Phase 19 starts. The *shape* (per-source grouping, int seconds, no error field) is the load-bearing part; exact spellings are cosmetic. |
| A3 | Reusing `healthPingTimeout` (3s) for `/ready`'s combined ping+schema-read bound is sufficient. | Pattern 4 / `handleReady` | LOW — a 1-row PK-less read on an already-open pool is sub-millisecond; 3s covers a slow connection acquire. D-01/RDY-02 say "short timeout", 3s matches `/health`. A sibling `readyCheckTimeout` const is equally fine. |
| A4 | The Dockerfile top-comment amendment (Pitfall 6) satisfies review; no deeper policy change (e.g. a lint rule) blocks `ARG VERSION`. | Pitfall 6 | LOW — D-13 already sanctions the approach; this is a comment edit. If a repo policy check greps the Dockerfile for `ARG`, that check would need updating too — grep for such a check at plan time. |
| A5 | `internal/buildinfo` is the right home for `Version` (vs `package main`). | Component Responsibilities | LOW — a `-X main.Version` also works but `package main` can't be imported by `/status`'s handler; `internal/buildinfo` is importable by `httpserver` and `main`. CONTEXT explicitly floats "`internal/buildinfo` vs main package" as discretion. |
| A6 | No CI job asserts `app_version != "dev"` on the built image today, so verifying the injection works end-to-end is a manual / new-smoke-check step. | Pitfall 7 / Validation | MEDIUM — if the injection silently fails (wrong `-X` path), `/status` shows `"dev"` in production and nobody notices. Recommend adding a one-line assertion to an existing CI step or the `n1-boot` / boot-e2e path. |

## Open Questions

1. **Where does `watchlist_size` counting live?**
   - What we know: D-10 mandates a `CountWatchlist` sqlc query in `queries/watchlist.sql`.
     `internal/watchlist.Store` (the narrow httpserver-facing interface) has no count method
     and has ~5 test stubs.
   - What's unclear: whether to (a) add `Count(ctx) (int, error)` to `watchlist.Service`
     behind a new narrow `httpserver.WatchlistCounter` seam, or (b) widen `watchlist.Store`
     (touches all stubs).
   - Recommendation: **(a)** — new `WatchlistCounter` seam declared in `httpserver`,
     implemented by `watchlist.Service` (or a 3-line adapter over `sqlc.Queries` in
     `main.go`). Zero churn on existing stubs. Passed via `WithStatus(...)`.

2. **Does `poller` gain the `RunRecorder` field/option this phase even though `runCycle`
   never calls it?**
   - What we know: ROADMAP "Notes for the phase planner" says yes explicitly — "The
     `RunRecorder` interface + a no-op default + real-store wiring in `main.go` is in scope;
     calling `RecordRun` from `runCycle` is not."
   - Recommendation: **yes** — build the seam, field, option, no-op default, and `main.go`
     wiring. This makes Phase 18.1 a pure `runCycle` edit with no new plumbing.

3. **`app_version`: full SHA or truncated at injection?**
   - What we know: `${{ github.sha }}` is 40 hex chars; CONTEXT wants "a short SHA in the
     about block".
   - Recommendation: inject the **full** SHA (traceability), truncate to 12 chars **in the
     `/status` handler / a `buildinfo.Short()` helper** for display. Keeps the CI change to
     one line (`build-args: VERSION=${{ github.sha }}`), no extra `cut` step.

4. **`instance.schema_applied` on a `/status` request when the DB is momentarily down —
   `null` + still `200`, or `503`?**
   - Recommendation: **`null` + `200`.** `/status` is an operator panel, not a probe; the
     ring buffer + counts are the payload's point, and `/ready` is the endpoint that turns
     red on a DB blip. Log the schema error to `httplog.SetAttrs`. (The `watchlist_size`
     count failing is different — that's a `500`, mirroring `handleListEvents`.)

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build | ✓ | `go 1.26` (`go.mod:3`); Docker builder `golang:1.26.6-alpine3.24` (`Dockerfile:41`) | — |
| `sqlc` CLI | `CountWatchlist` codegen | assumed ✓ (used every prior phase) | **must be v1.31.1** (`Makefile:17`); `sqlc-version-check` hard-fails otherwise | none — wrong version blocks `make sqlc-check` |
| PostgreSQL (via `make db-up` / docker compose) | integration tests for `db.SchemaVersion`, `CountWatchlist` | ✓ (`docker compose up -d --wait postgres`, `Makefile:46-47`) | pinned in `docker-compose.yml` | pure-Go unit tests cover `handleReady`/`handleStatus`/`pollruns` logic with fakes; DB only needed for the two integration tests |
| Docker + Buildx | app-version injection E2E verification | ✓ in CI (`docker/setup-buildx-action`, `full-pipeline.yml:573-574`) | — | local `make build` produces `Version == "dev"` (expected) |
| `go test -race` | — | ✗ (ThreadSanitizer allocation failure on this WSL2 box; absent from CI) | — | `pollruns.Store` mutex is inspectable; RUN-04 correctness proven by a looped exact-assertion test (the milestone's stated `-race` substitute). `make test-short` (which has `-race`, `Makefile:57`) is unusable here — use `go test` / `make test`. |

**Missing dependencies with no fallback:** none — every blocking dependency is present or
has a viable pure-Go path.

**Missing dependencies with fallback:** `go test -race` (mitigated by mutex simplicity +
looped invariant test); a live CI image for app-version E2E (mitigated by a `buildinfo` unit
test + a recommended CI smoke assertion, A6).

## Validation Architecture

`workflow.nyquist_validation: true` — this section is required.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`t.Fatalf` / table-driven; no testify in `internal/httpserver` or `internal/poller` — verified by reading `health_test.go`, `poller_test.go`) |
| Config file | none — `go test` |
| Quick run command | `go test ./internal/httpserver/ ./internal/pollruns/ ./internal/db/ ./internal/poller/ -count=1` (pure-Go subset, no DB) |
| Full suite command | `make db-up && make test` (integration; the DoD gate) then `make coverage-gate` (80% backend floor) then `make sqlc-check` (local-only drift gate) |
| DB-backed tests | `internal/db` schema-version integration test + `CountWatchlist` integration test need `make db-up`; everything else runs with fakes |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RDY-01 | `applied >= expected && !dirty` → 200 with `schema_applied`/`schema_expected`; `applied < expected` → 503 `schema_behind`; `dirty` → 503 `schema_dirty`; ping fail → 503 `db_unreachable` | unit (httptest + fake `SchemaVersioner` + `stubPinger`) | `go test ./internal/httpserver/ -run TestReady -x` | ❌ Wave 0 — `internal/httpserver/ready_test.go` |
| RDY-01 | **rollback case: `applied = expected + 1` → 200 `ready`** (the `>=` not `==` check) | unit | `go test ./internal/httpserver/ -run TestReady_AheadOfSource` | ❌ Wave 0 |
| RDY-01 | body carries no DSN / driver text / path on the 503 path | unit (golden: fake schema returns a DSN-bearing error, assert body has no `://` / `password`) | `go test ./internal/httpserver/ -run TestReady_NoLeak` | ❌ Wave 0 |
| RDY-01 | `db.ExpectedSchemaVersion()` returns the embedded max (7 today) | unit (no DB — embedded FS) | `go test ./internal/db/ -run TestExpectedSchemaVersion` | ❌ Wave 0 — `internal/db/schema_version_test.go` |
| RDY-01 | `db.SchemaVersion` reads real `schema_migrations` (version + dirty) | integration (`make db-up`) | `go test ./internal/db/ -run TestSchemaVersion_Integration` | ❌ Wave 0 |
| RDY-02 | `/ready` reachable at exact path, **non-401**, on a `WithAuthGate` server with no cookie | unit (mirror `server_test.go:229-236` `newGatedServer`) | `go test ./internal/httpserver/ -run TestReady_GatedNo401` | ❌ Wave 0 |
| RDY-02 | `/ready` reachable in the inert (no-passphrase) branch too | unit | `go test ./internal/httpserver/ -run TestReady_InertBranch` | ❌ Wave 0 |
| RDY-02 | DB check is bounded — a ping that blocks on `<-ctx.Done()` fails the probe within ~3s, doesn't hang (mirror `TestHealth_DownOnTimeout`, `health_test.go:121-147`) | unit | `go test ./internal/httpserver/ -run TestReady_Timeout` | ❌ Wave 0 |
| RDY-02 | shared pool, no new connection, no side effects | code review + the fact the seam only exposes `SchemaVersion(ctx)` and the impl uses the injected `*pgxpool.Pool` | manual: `grep -n 'sql.Open\|pgxpool.New\|migrate.New' internal/httpserver/ready.go` → must be empty | n/a |
| RDY-03 | `/health` body/status/timeout **unchanged** | unit (existing `health_test.go` must still pass byte-for-byte; add a pin if not already exact) | `go test ./internal/httpserver/ -run TestHealth` | ✅ `health_test.go` (4 tests: Up/Down/DownOnTimeout/Concurrent) |
| RUN-02 (skip signal) | `RecordSkip` bumps `consecutive_skips` and stamps `last_skipped_at`; `RecordRun` resets `consecutive_skips` to 0 | unit | `go test ./internal/pollruns/ -run TestStore_Skip` | ❌ Wave 0 — `internal/pollruns/pollruns_test.go` |
| RUN-02 (skip signal) | the skip signal surfaces in `/status` (`sources.<x>.last_skipped_at` / `consecutive_skips`) | unit (httptest + fake `StatusStore` returning a snapshot with a skip stamp) | `go test ./internal/httpserver/ -run TestStatus_SkipSignal` | ❌ Wave 0 |
| RUN-04 | history bounded to N=50 per source | unit | `go test ./internal/pollruns/ -run TestStore_RingBound` (record 60, assert len 50, newest-first) | ❌ Wave 0 |
| RUN-04 | correct under two sources recording near-simultaneously | unit (looped ~1000×: two goroutines each `RecordRun` 60× for their own source, assert each source ends with exactly 50 and no interleaved/torn `RunResult`) — the milestone's stated `-race` substitute | `go test ./internal/pollruns/ -run TestStore_TwoSourceConcurrent -count=1` | ❌ Wave 0 |
| RUN-04 | `poller.RunRecorder` seam exists, is wired to the real store, is inert (no `runCycle` call) | unit (assert `var _ poller.RunRecorder = (*pollruns.Store)(nil)`; grep `runCycle` for `p.runs.` → must be empty this phase) | `go test ./internal/poller/ -run TestRunRecorder_Wired` + `grep -n 'p.runs' internal/poller/poller.go` | ❌ Wave 0 |
| STAT-01 | gated `GET /status` returns JSON with last run per source, last N runs, watchlist size, poll interval, `instance` block | unit (httptest + fakes) | `go test ./internal/httpserver/ -run TestStatus_Shape` | ❌ Wave 0 — `internal/httpserver/status_test.go` |
| STAT-01 | `401` without a session on a gated server | unit (mirror the gated `/events` test) | `go test ./internal/httpserver/ -run TestStatus_Gated401` | ❌ Wave 0 |
| STAT-01 | **empty-history instance → 200** (not an error), `instance`/`watchlist_size`/`poll_interval` still populated | unit (fresh `pollruns.NewStore()`) | `go test ./internal/httpserver/ -run TestStatus_EmptyHistory` | ❌ Wave 0 |
| STAT-01 | `watchlist_size` reflects `CountWatchlist` | integration (`make db-up`) for the query + unit for the handler wiring | `go test ./internal/db/ -run TestCountWatchlist_Integration` | ❌ Wave 0 |
| STAT-01 | `poll_interval_seconds` renders `cfg.PollInterval` as an int (900 for the 15m default) | unit | `go test ./internal/httpserver/ -run TestStatus_PollInterval` | ❌ Wave 0 |
| STAT-02 | no DSN / webhook / path / raw driver error on any `/status` path — **golden redaction test** | unit (fake `WatchlistCounter` + fake schema each return an error whose text embeds `postgres://u:p@h/db` and a Discord webhook URL; assert the response body and status contain neither) | `go test ./internal/httpserver/ -run TestStatus_NoLeak` | ❌ Wave 0 |
| STAT-02 | `RunResult` has no free-text error field | compile-time + code review (`grep -n 'Error\|Detail\|Message' internal/pollruns/pollruns.go` on struct fields) | manual | n/a |
| app version | `buildinfo.Version` defaults to `"dev"` | unit | `go test ./internal/buildinfo/` | ❌ Wave 0 |
| app version | injected `-ldflags -X` reaches `/status` `instance.app_version` | **manual / new CI smoke** — build the image with `--build-arg VERSION=deadbeef...`, run it, `curl -s localhost:8080/status \| jq -r .instance.app_version` → must not be `"dev"` | manual (document exact commands in the plan; recommend wiring into `n1-boot` or boot-e2e per A6) | ❌ |
| app version | `release` job pushes the byte-identical scanned image | manual: inspect `full-pipeline.yml` diff — `release` job (`full-pipeline.yml:608-682`) must be untouched; it `docker load`s the tar (`:652-653`) and only `tag`+`push` (`:660-664`) | code review | ✅ (verified this session — no rebuild in `release`) |

### Sampling Rate

- **Per task commit:** `go test ./internal/httpserver/ ./internal/pollruns/ ./internal/db/ ./internal/poller/ ./internal/buildinfo/ -count=1` (+ `go vet ./...`, `golangci-lint run` per the DoD).
- **Per wave merge:** `make db-up && make test && make coverage-gate && make sqlc-check`.
- **Phase gate:** full suite green + `make sqlc-check` clean + manual app-version E2E on a
  locally-built image + the `/gsd-verify-work` UAT pass. `corepack pnpm` frontend gates are
  **not applicable** — this phase touches no `web/` files.

### Wave 0 Gaps

- [ ] `internal/httpserver/ready_test.go` — RDY-01, RDY-02 (all cases incl. gated-no-401, timeout, ahead-of-source, no-leak)
- [ ] `internal/httpserver/status_test.go` — STAT-01, STAT-02, RUN-02 skip-signal surfacing (fakes for `StatusStore` / `WatchlistCounter` / `SchemaVersioner`)
- [ ] `internal/pollruns/pollruns_test.go` — RUN-04 (ring bound, two-source concurrent looped), RUN-02 (skip counter + reset)
- [ ] `internal/db/schema_version_test.go` — `ExpectedSchemaVersion` (unit) + `SchemaVersion` (integration, `make db-up`)
- [ ] `internal/db/` — `CountWatchlist` integration test (may live in an existing watchlist-query test file)
- [ ] `internal/poller/poller_test.go` — add `fakeRunRecorder` (mirror `fakeEventRecorder` / `fakeNotifier`, `poller_test.go:179-241`) + a test that `WithRunRecorder` wires it and `runCycle` never calls it this phase
- [ ] `internal/buildinfo/buildinfo_test.go` — default `"dev"`
- [ ] Fakes to add: `fakeSchemaVersioner`, `fakeStatusStore`, `fakeWatchlistCounter` in `internal/httpserver` (mirror `stubPinger` `health_test.go:35-47` and `stubEventsStore` `events_test.go:41`)
- [ ] Recommended: a CI smoke assertion that a `build-scan`-built image reports a non-`"dev"` `app_version` (A6)

## Security Domain

`workflow.security_enforcement: true`, ASVS L1, `block_on: high`.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | Consumer-declared seams keep the DB handle out of `poller`; `/ready` unauthenticated by design (ops probe), `/status` behind the existing passphrase gate. |
| V2 Authentication | no (`/ready` deliberately unauthenticated; `/status` reuses the shipped gate unchanged) | `/status` inherits `gate.Authenticate` via `registerDataRoutes` — no new auth code. |
| V3 Session Management | no | No session changes; `/status` reuses `dt_session` cookie handling. |
| V4 Access Control | yes | `/ready` structurally exempt (root router, both branches — never a path allowlist). `/status` inside the gated `Group`. A test asserts `/ready` is non-401 gated and `/status` is 401 ungated. |
| V5 Input Validation | minimal | `/ready` and `/status` take **no** query params, no path params, no body. Nothing to validate. |
| V6 Cryptography | no | none. |
| V7 Error Handling & Logging | **yes — primary** | Raw errors → `httplog.SetAttrs` on the request context only; response bodies are fixed strings + typed fields. Mirrors `handleHealth` / `handleListEvents`. |
| V8 Data Protection | yes | `RunResult` carries no free-text; `Summary` is composed from counts + enum (D-09). No DSN/webhook/path can enter the ring buffer. |
| V9 Communications | no | inbound-only, no new outbound calls. |
| V14 Config | yes | `app_version` injected as a non-secret build arg; Dockerfile comment amended to keep the "no config values in ARG" invariant honest (Pitfall 6). No new env var. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| DB error string in `/ready` / `/status` JSON leaks DSN-with-password or Discord webhook URL | Information Disclosure | Store no free-text; raw error → `httplog.SetAttrs`; fixed response body. Golden leak test on both endpoints (`TestReady_NoLeak`, `TestStatus_NoLeak`). |
| `/ready` behind the gate → operators disable the gate to make monitoring work | Denial of Service (indirect) | Structural root-router exemption in both branches; test asserts non-401 on a gated server. |
| `/ready` DB call unbounded → goroutine pile-up under aggressive orchestrator probing | Denial of Service | `context.WithTimeout(r.Context(), 3s)` around ping + schema read (reuse `healthPingTimeout`). |
| `/status` schema/count read on every open operator tab → pool pressure | Denial of Service | In-scope mitigation is server-side only (bounded queries, shared pool); the fetch-cadence control is Phase 19's job (fetch-on-mount + Refresh, no polling). Note for Phase 19. |
| `/ready` exposes exact schema version numbers unauthenticated | Information Disclosure (low) | Accepted — a migration count is not sensitive and the deploy gate needs it. Documented, not mitigated. |
| App-version SHA baked into a public image reveals the exact commit | Information Disclosure (low) | Accepted — the repo/image are the developer's portfolio; a SHA is already implicit in a public repo. |
| `CountWatchlist` sqlc drift ships an untested query | Tampering (supply chain) | `make sqlc-check` before commit (local-only gate — must be in the plan's DoD checklist). |
| `-ldflags -X` wrong path → `app_version` silently `"dev"` in prod | (integrity of provenance metadata) | Full module path in the flag; `buildinfo` unit test; recommended CI smoke assertion (A6). |

No high-severity threats identified — the phase adds two read-only endpoints and an
in-memory buffer, with the established leak-free error pattern applied throughout.

## Sources

### Primary (HIGH confidence — read directly this session)

- `internal/httpserver/server.go` — `Server` struct (`:29-37`), `New` signature + `serverConfig`/`Option` (`:42-51,:106`), `/health` registration + gate branch (`:161-191`), `registerDataRoutes` (`:197-204`)
- `internal/httpserver/health.go` — `handleHealth`, `healthResponse`, `healthPingTimeout` (`:13-44`)
- `internal/httpserver/health_test.go` — `stubPinger` seam pattern, leak-loop assertion, timeout test (`:35-47,:113-147`)
- `internal/httpserver/events.go` — `handleListEvents` error handling / `writeError` / `httplog.SetAttrs` (`:80-128`)
- `internal/httpserver/server_test.go` — `newGatedServer` helper, `gatedRoutes` table (`:229-257`)
- `internal/poller/poller.go` — package doc "no DB connection" (`:1-15`), consumer seams (`:61-101`), `Notifier` interface (`:99-101`), `Option` + `WithMusicBrainzWorkers` (`:103-123`), `Poller` struct (`:136-169`), `New` + opts loop (`:178-198`), `runCycle` CAS guard (`:270-282`), source consts (`:45-46`)
- `internal/poller/poller_test.go` — `fakeEventRecorder` / `fakeNotifier` doubles (`:179-241`), ~40 `New` call sites
- `internal/notifier/notifier.go` — `NoOp` / `Sink` / `Select` no-op-default pattern (`:48-105`)
- `internal/db/migrate.go` — `migrationsFS` embed (`:22-23`), `RunMigrations` + `iofs.New` (`:208-214`), ahead-of-source guard reading `schema_migrations` (`:298-302`), `maxSourceVersion` walk (`:320-341`), `redactDSN`/`redactError` (`:114-194`)
- `internal/db/migrations/` — directory listing, highest = `000007` → `ExpectedSchemaVersion()` == 7
- `internal/db/sqlc/health.sql.go`, `watchlist.sql.go` — generated-code shape; `emit_interface` / `Querier`
- `internal/config/config.go` — `Config` struct, `PollInterval time.Duration` env `POLL_INTERVAL` default `15m` (`:24`)
- `cmd/server/main.go` — boot order: `RunMigrations` (`:137`) → `NewPool` (`:146`) → `httpserver.New(pool, ...)` (`:235-238`) → `notifier.Select` (`:250`) → `poller.New(..., WithMusicBrainzWorkers, WithDeezerWorkers)` (`:258`) → `pollr.Start` (`:262`)
- `internal/events/service.go` — `Store` narrow-interface pattern (`:82-84`)
- `internal/watchlist/service.go` — `Store` interface (no count) (`:99-104`)
- `sqlc.yaml` — full config (`:1-14`)
- `Makefile` — `SQLC_VERSION` pin (`:17`), `sqlc-check` local-only comment + target (`:19-21,:116-128`), `build` target (plain `go build`, no ldflags), `test-short` has `-race` (`:57`), `db-up` (`:46-47`)
- `Dockerfile` — "no config value in ARG" comment (`:15-19`), Go builder stage `golang:1.26.6` (`:41`), `go build -trimpath -ldflags="-w -s"` (`:60-61`)
- `.github/workflows/full-pipeline.yml` — `build-scan` job + `docker/build-push-action` step (`:556-606`), `release` job loads the scanned tar and only tags/pushes — no rebuild (`:608-682`)
- `go.mod` — `go 1.26` (`:3`)
- `.planning/config.json` — `nyquist_validation: true`, `security_enforcement: true`, ASVS L1
- `.planning/phases/18-.../18-CONTEXT.md`, `.planning/ROADMAP.md` "Phase 18" + planner notes, `.planning/REQUIREMENTS.md`, `docs/adr/0001-*.md`, `.planning/research/ARCHITECTURE.md` + `PITFALLS.md`, `.planning/STATE.md`

### Secondary (MEDIUM confidence — web, cross-checked)

- golang-migrate `schema_migrations` table structure: `(version bigint, dirty boolean)`, single row, `dirty=true` on failed migration — [Better Stack: Database migrations in Go with golang-migrate](https://betterstack.com/community/guides/scaling-go/golang-migrate/), [oneuptime: Go database migrations](https://oneuptime.com/blog/post/2026-01-07-go-database-migrations/view), [coffeebytes: Go migration tutorial with migrate](https://coffeebytes.dev/en/go/go-migration-tutorial-with-migrate/)
- Go `-ldflags -X` + Docker `--build-arg`/`ARG` pattern; wrong import path is silently ignored — [Alex Ellis: Inject build-time vars with Golang](https://blog.alexellis.io/inject-build-time-vars-golang/), [DEV: Injecting Version Info at Build Time in Go With -ldflags](https://dev.to/gabrielanhaia/injecting-version-info-at-build-time-in-go-with-ldflags-m9j), [Go FAQ: How to Use ldflags to Inject Version Info](https://www.gofaq.org/en/how-to-use-ldflags-to-inject-version-info-at-build-time/)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps; every seam pattern verified against source at cited lines.
- Architecture / integration points: HIGH — boot order, route registration, `New` signatures, seam shapes all read directly this session.
- Pitfalls: HIGH for codebase mechanics; MEDIUM for the golang-migrate table shape and ldflags behaviour (web, multi-source).
- Frozen `/status` contract: MEDIUM — the shape is well-grounded but key names are explicitly the planner's to lock (A2).

**Research date:** 2026-09-09
**Valid until:** ~2026-10-09 for the codebase findings (stable repo); the two web facts are about long-stable tooling behaviour and are not time-sensitive.
