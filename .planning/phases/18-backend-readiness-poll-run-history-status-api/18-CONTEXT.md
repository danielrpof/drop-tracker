# Phase 18: Backend — Readiness, Status Surface & App Version - Context

**Gathered:** 2026-09-09
**Revised:** 2026-09-09 (post design-grilling — scope narrowed, `poll_runs` table → in-process ring buffer, Phase 18 split into 18 + 18.1)
**Status:** Ready for planning

<domain>
## Phase Boundary

Make a live drop-tracker legible to an operator (or a machine) **without
touching the poll cycle's hot path** (that is Phase 18.1). Four additive
backend deliverables:

1. **`GET /ready`** — unauthenticated readiness probe alongside the unchanged
   `/health` liveness probe. `200` (with `schema_applied` / `schema_expected`)
   when the DB is reachable and the schema is current and clean; `503` with a
   machine reason enum otherwise. Leak-free body.
2. **`internal/pollruns.Store`** — an in-process ring buffer holding the last
   N=50 run entries per source, plus each source's last-skipped timestamp and
   consecutive-skip count. **Not a database table** (`docs/adr/0001`). It
   satisfies both the `poller.RunRecorder` write seam (interface declared in
   `internal/poller`, wired to a real store here but not yet *called* —
   Phase 18.1 does that) and the `httpserver.StatusStore` read seam.
3. **`GET /status`** — passphrase-gated JSON: last run per source, last N runs
   per source, per-source skip signal, watchlist size, poll interval, and an
   `instance` block (`app_version`, `schema_applied`, `schema_expected`).
   **This phase freezes the `/status` JSON contract that Phase 19 types
   against.** Ships returning empty run lists (no cycles recorded until 18.1).
4. **App version** — injected at image build via `--build-arg VERSION=${{
   github.sha }}` + Dockerfile `ARG` + `-ldflags -X`; `"dev"` fallback for a
   flagless local build. Feeds `/status`'s `instance` block.

Requirements are the contract: RDY-01…03, RUN-02 (skip signal only), RUN-04,
STAT-01/02 (`.planning/REQUIREMENTS.md`).

**Not in this phase:** anything that edits `runCycle` or `internal/detection`
— the `RunRecorder` *call*, counter aggregation, `EventRecorder` widening,
the `cancelled` run entry, the concurrency-invariant test (all Phase 18.1);
the SPA System view (Phase 19); auto-refresh, "poll now", alerting, metrics.

</domain>

<decisions>
## Implementation Decisions

The ROADMAP.md "Phase 18" → "Notes for the phase planner" block is the
primary brief and carries the detail. Decisions below are the load-bearing
ones; where a prior D-NN is superseded it is marked.

### `/ready` probe

- **D-01 (unchanged): Ready condition is `!dirty && applied >= expected`.**
  Phase 16's ahead-of-source guard (`migrate.go:298-302`) deliberately lets a
  rolled-back binary boot and serve against a newer additive schema; strict
  `==` would report that healthy instance not-ready forever and flap the
  deferred Phase 17 deploy gate. `expected` = the existing `maxSourceVersion`
  walk, exported as `db.ExpectedSchemaVersion()`, called once at boot.
  Rationale one-liner + the Phase-16 reference go in the handler comment.

- **D-02 (unchanged): `503` body carries a machine reason enum** — exactly one
  of `db_unreachable`, `schema_behind`, `schema_dirty`. No DSN, driver text,
  path, or free-form message. Raw cause → `httplog.SetAttrs` only.

- **D-03 (unchanged): `/ready` body shape** — `200`:
  `{"status":"ready","schema_applied":<int>,"schema_expected":<int>}`. `503`:
  the same fields best-effort (`schema_applied` may be null if the DB is
  unreachable) **plus** `"reason":"<enum>"`. Key names locked — Phase 19's
  readiness badge parses this directly.

- **D-04 (unchanged): `/ready` mirrors `/health`'s registration** — root
  router, **both** the gate-configured and inert branches (structural
  exemption, never a path-string match), 3s-bounded DB check on the shared
  pool, no side effects, never `401` on a gated instance with no cookie.

- **NEW: schema-version read is shared with `/status`.** One
  `db.SchemaVersion(ctx, pool)` helper — hand-rolled pgx
  `SELECT version, dirty FROM schema_migrations` (that table is
  golang-migrate-owned, outside the schema dir, so it cannot be a sqlc
  query). `/ready` and `/status`'s `instance` block both call it.

### Run-history store (was "poll_runs history")

- **D-05 SUPERSEDED — skip-overlap coalescing is gone.** A skipped tick is
  **not** a run entry. It bumps a per-source `consecutiveSkips` counter and
  stamps `lastSkippedAt` on the store; both are surfaced in `/status`. The
  next real run resets `consecutiveSkips` to 0. (The store field + method
  land here; the `RecordSkip` *call* from `runCycle` is Phase 18.1.)
  REQUIREMENTS RUN-02 was reworded to match.

- **D-06 SUPERSEDED — no `outcome` CHECK constraint.** No table, no SQL. The
  `Outcome` field is a Go string, valid values `ok` / `error` / `cancelled`
  (no `partial`, no `skipped_overlap`), validated in Go if at all.

- **D-07 SUPERSEDED — no detached context.** `RecordRun` is a synchronous
  in-memory mutex append that returns in microseconds; a shutdown-cancelled
  cycle records its entry with no `context.WithoutCancel` dance. `ctx` stays
  in the seam signature for symmetry with `EventRecorder` / `Notifier`, but
  it is not load-bearing. (The call semantics — log-and-swallow — are
  Phase 18.1.)

- **D-08 SUPERSEDED — ring buffer, not a pruned table.**
  `internal/pollruns.Store` holds `[]RunResult` per source, cap `const N =
  50` (named const in the `pollruns` package, one-line comment on the
  number). No prune query, no INSERT+DELETE CTE, no cross-source-deadlock
  surface. Correct under two sources recording near-simultaneously = one
  `sync.Mutex` (or `RWMutex`) on the store.

- **D-09 (unchanged): `Summary` is a stored string, composed deterministically
  from the counts + outcome enum at record time** — e.g. `"ok — 12 checked,
  1 errored, 3 events"`. Never from a driver error, upstream response, DSN,
  webhook URL, or path. Held on the `RunResult` value.

- **D-10 SUPERSEDED — no migration, no sqlc for run history.** `RunResult`
  is a plain Go struct: `Source`, `CycleID`, `StartedAt`, `FinishedAt`,
  `DurationMS`, `ArtistsChecked` (= entries dispatched to a worker),
  `ArtistsSkipped`, `ArtistsErrored`, `EventsRecorded`, `Outcome`, `Summary`.
  The **only** new sqlc query this phase adds is `CountWatchlist` in
  `queries/watchlist.sql` for `/status`'s `watchlist_size` — regenerate and
  run `make sqlc-check` locally (no CI counterpart).

### `/status` API (contract frozen here)

- **D-11 (adjusted): `/status` returns** — `runs` (last per source),
  `history` (last N per source, newest first), a per-source skip signal
  (`last_skipped_at`, `consecutive_skips`), `watchlist_size`, `poll_interval`,
  and `instance` (`app_version`, `schema_applied`, `schema_expected`). Empty
  `runs`/`history` on a fresh instance is a valid `200`, not an error. Key
  names, `poll_interval` encoding, and whether `instance` reuses `/ready`'s
  key names — planner's call, kept internally consistent and **documented for
  Phase 19** (contract-freeze point).

- **D-12 (unchanged): `/status` leaks nothing** — counts, timestamps, enum
  values, the composed `Summary`, the app version, two integers. `401`
  without a session (unchanged gate enforcement). DB failure → raw to
  `httplog.SetAttrs`, fixed body.

### App version

- **D-13 CHANGED — `${{ github.sha }}` build-arg, not the svu tag.** The
  pipeline builds the image once in `build-scan`, saves the tarball, and
  `release` pushes it byte-for-byte (07-REVIEW CR-02). The svu `next` tag is
  computed in `release`, *after* the build — injecting it would mean either
  computing svu inside the sensitive `build-scan` job or rebuilding in
  `release` (breaking the single-build guarantee). Instead: pass
  `--build-arg VERSION=${{ github.sha }}` on `build-scan`'s
  `docker/build-push-action` step (SHA is free, no svu, no `fetch-depth`
  change), `ARG VERSION` + `-ldflags "-X …=$VERSION"` in the Dockerfile
  builder stage, `"dev"` fallback. Show a short SHA in the about block.
  Touches `.github/workflows/full-pipeline.yml` + `Dockerfile` (shared-file
  hazard, but one line each, `release` job untouched).

### Concurrency

- **D-14 MOVED to Phase 18.1.** The `runCycle` counter aggregation is the
  whole reason for the split. This phase's `pollruns.Store` mutex is trivial
  and inspectable; the fan-out counter correctness lives in 18.1 with the
  looped invariant test.

### `events_recorded` source — LOCKED (was Claude's discretion)

- **Widen the `EventRecorder` seam** (`DetectMusicBrainz` / `DetectDeezer`)
  to `(int, error)`. The downstream `SELECT count(*) … WHERE created_at >=
  started_at` alternative was rejected — `started_at` is the app process's
  wall clock and `events.created_at` is Postgres's, so positive app-vs-DB
  skew silently undercounts. The widening happens in **Phase 18.1** (it
  touches `internal/detection` + `runCycle`); this phase's `RunResult` just
  carries the `EventsRecorded int` field.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `.planning/ROADMAP.md` → "Phase 18" → "Notes for the phase planner" — the
  **primary brief**. The split rationale block above it lists every
  superseded decision.
- `docs/adr/0001-in-process-ring-buffer-for-poll-run-history.md` — why there
  is no `poll_runs` table.
- `.planning/REQUIREMENTS.md` — RDY-01…03, RUN-02 (skip signal), RUN-04,
  STAT-01/02 are this phase's contract. RUN-01/03 and the `cancelled` half of
  RUN-02 are **Phase 18.1**. The "Out of Scope" table binds.
- `.planning/research/ARCHITECTURE.md` — seam list and integration points.
  **Note:** its `poll_runs` DDL, prune queries, and `pollruns.Recorder`
  (sqlc) sections are superseded by ADR-0001 — read them for the seam shapes
  (`ReadinessChecker`, `RunRecorder`, `StatusStore`) and the `/ready`
  handler logic, ignore the table/sqlc mechanics.
- `.planning/research/PITFALLS.md` — Pitfalls #5/#6 (`/ready` correctness,
  gating, cost, timeout) and #7 (`/status` leakage) apply directly. Pitfalls
  #2 and #4 (prune races, skip-row eviction) are **designed out** by the ring
  buffer. Pitfall #1 (counter race) is **Phase 18.1**.
- `.planning/phases/18-backend-readiness-poll-run-history-status-api/18-DISCUSSION-LOG.md`
  — audit trail incl. the 2026-09-09 grilling session.

### Codebase — seams and contracts this phase extends
- `internal/httpserver/health.go` — `handleHealth`, `healthResponse`,
  `healthPingTimeout`. `/ready` mirrors this (D-04). `/health` frozen (RDY-03).
  Note: `handleHealth` **already pings the DB** — `/ready` adds only the
  schema-state check on top.
- `internal/httpserver/server.go` — `Server` struct (holds a `Pinger`, not a
  pool — a new seam or a widened one is needed for `SchemaVersion`);
  `registerDataRoutes` (gated); the gated-vs-inert branch structure; `Option`
  functional-option shape for additive `New` growth.
- `internal/db/migrate.go` — `maxSourceVersion` (line ~326), the
  ahead-of-source guard (line ~298); source of `expected` for D-01.
- `internal/poller/poller.go` — declare the `RunRecorder` interface next to
  `Notifier` (~line 91), add a `p.runs` field + `WithRunRecorder` option +
  no-op default. **Do not touch `runCycle`.**
- `internal/poller/poller_test.go` — `EventRecorder` seam + `fakeEventRecorder`
  double: the pattern `RunRecorder`'s test double follows.
- `queries/watchlist.sql` + `sqlc.yaml` — where `CountWatchlist` lands;
  regenerate sqlc, run `make sqlc-check` (local-only).
- `.github/workflows/full-pipeline.yml` `build-scan` job (~line 576) — the
  `docker/build-push-action` step gets `--build-arg VERSION`. **Do not touch
  the `release` job.**
- `Dockerfile` builder stage `go build -trimpath -ldflags="-w -s"` (~line 60)
  — add `ARG VERSION` + `-X`.
- `cmd/server/main.go` — composition root: `ExpectedSchemaVersion()` after
  `RunMigrations`; build `pollruns.NewStore()`, wire it as both `RunRecorder`
  (via `poller.WithRunRecorder`) and `httpserver.StatusStore`; pass the
  readiness checker + expected version + app-version var + poll interval into
  `httpserver.New`.

### Environment
- `.planning/WINDOWS.md` — `go test -race` unavailable; governs Phase 18.1,
  not this phase (the store mutex is inspectable).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable assets
- `handleHealth` / `healthResponse` / `healthPingTimeout` — `/ready` is a
  near-copy plus the schema check.
- `maxSourceVersion(src)` — `expected` for D-01, exported as
  `ExpectedSchemaVersion()`.
- `notifier.Select` / `notifier.NoOp` pattern — a `RunRecorder` that is
  always non-nil at the wiring site (no-op when unconfigured) so `runCycle`
  (in 18.1) never nil-checks the seam.
- `EventRecorder` seam + `fakeEventRecorder` double — the exact pattern
  `RunRecorder` and its test double follow.
- `events.Store` seam referenced from `server.go` — the pattern
  `StatusStore` follows.

### Established patterns
- Consumer-declared narrow seams (`watchlist.Store`, `poller.EventRecorder`,
  `httpserver.Pinger`); `RunRecorder`, `ReadinessChecker`/`SchemaVersioner`,
  `StatusStore` follow suit.
- Structural route exemptions in `server.go` (gated vs inert branch), never a
  path-string allowlist.
- Functional options keep `New` signatures additive.
- sqlc codegen committed; `make sqlc-check` is the only drift gate (local).

### Integration points
- `cmd/server/main.go` — one shared wiring change for all four deliverables.
- `internal/httpserver/server.go` — `/ready` on the root router (both
  branches), `/status` in `registerDataRoutes` (gated).
- `internal/poller/poller.go` — seam declaration + option + field + no-op
  default ONLY. The call site is Phase 18.1.

</code_context>

<specifics>
## Specific Ideas

- `/ready` `503` reason vocabulary is exactly three values:
  `db_unreachable`, `schema_behind`, `schema_dirty`.
- `Summary` examples: `"ok — 12 checked, 1 errored, 3 events"`,
  `"error — 8 checked, 4 errored"`, `"cancelled — 5 checked, 0 errored
  (shutdown)"`.
- App-version fallback string for a flagless local build: `"dev"`.
- Ring buffer depth: N = 50 per source ≈ 12h at the default `POLL_INTERVAL=15m`
  (config field `PollInterval time.Duration`; `/status` renders it as
  `poll_interval_seconds` = 900 by default — see 18-RESEARCH.md).
- `db.ExpectedSchemaVersion()` returns **7** today — highest migration on disk
  is `000007`, and this phase adds no migration. A healthy instance reports
  `schema_applied: 7, schema_expected: 7`. (18-RESEARCH.md corrected the
  pre-grilling "8" assumption.)
- `/status` on a fresh instance: `runs: {}` / `history: {}` (or empty
  arrays) + a populated `instance` block + `watchlist_size` + `poll_interval`
  — a valid `200`, and the state Phase 19's first-run copy renders.

</specifics>

<deferred>
## Deferred Ideas

- Everything that edits `runCycle` / `internal/detection` → **Phase 18.1**.
- SPA System view → Phase 19.
- Auto-refresh (OBS-01), "poll now" (OBS-02), paginated history (OBS-03),
  poll-failure alerting (OBS-04) — deferred.
- A dedicated `/version` endpoint — rejected; folded into `/status`'s
  `instance` block.
- A readiness/DB-reachability *history* log — out of scope; poll-run history
  + a live `/ready` is the "health history" surface.
- Promoting `redactDSN` / `redactError` to a shared package — not needed;
  nothing free-text is stored.
- `git describe`-style version string (needs `fetch-depth: 0` on
  `build-scan`) — a short SHA is enough for v1.4.

</deferred>

---

*Phase: 18-backend-readiness-poll-run-history-status-api (scope: Readiness, Status Surface & App Version)*
*Context revised post-grilling: 2026-09-09*
