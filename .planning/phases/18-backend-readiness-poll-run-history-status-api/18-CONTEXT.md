# Phase 18: Backend — Readiness, Poll-Run History & Status API - Context

**Gathered:** 2026-09-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Make a live drop-tracker legible to an operator (or a machine) without opening
container logs or a psql shell. Three backend deliverables:

1. **`GET /ready`** — unauthenticated readiness probe alongside the unchanged
   `/health` liveness probe. `200` when the DB is reachable and the schema is
   current and clean; `503` otherwise with a leak-free body.
2. **`poll_runs` table + `RunRecorder` seam** — one summary row per poll-cycle
   invocation per source (timings, work counters, outcome enum, composed
   summary string), recorded through a seam so the poller holds no DB handle,
   pruned to the last N rows per source on write.
3. **`GET /status`** — passphrase-gated JSON: last run per source, last N runs,
   watchlist size, poll interval, and an `instance` block. **This phase freezes
   the `/status` JSON contract that Phase 19 types against.**

Requirements are the contract: RDY-01…03, RUN-01…04, STAT-01/02
(`.planning/REQUIREMENTS.md`).

**Not in this phase:** the SPA System view (Phase 19), auto-refresh / live
updates, a manual "poll now" trigger, poll-failure alerting, Prometheus/metrics.

</domain>

<decisions>
## Implementation Decisions

### `/ready` probe

- **D-01: Ready condition is `applied >= expected && !dirty`**, not strict
  `applied == expected`. Phase 16's ahead-of-source guard deliberately lets a
  rolled-back binary boot and serve against a newer additive schema; strict
  `==` would report that healthy instance not-ready forever and flap the future
  Phase 17 deploy gate. `expected` = the existing `maxSourceVersion` walk over
  the embedded migrations, exported for reuse (research calls this
  `db.ExpectedSchemaVersion()`). A `dirty` schema still fails readiness even
  when `applied >= expected`. Put the rationale in the handler comment (one
  line + the `D-01` / Phase 16 reference).
  — **Reversibility:** costly — the `>=` semantics are what the Phase 17 deploy
  gate will be written against; changing to `==` later silently changes deploy
  behaviour for a rolled-back instance.

- **D-02: `503` body carries a machine reason enum**, exactly one of
  `db_unreachable`, `schema_behind`, `schema_dirty`. No DSN, no driver text, no
  filesystem path, no free-form message. A monitor or the Phase 17 gate can
  tell "DB down" from "mid-migration" without reading logs. The raw cause (ping
  error, etc.) goes to `httplog.SetAttrs` only, mirroring `handleHealth`.

- **D-03: `/ready` response body shape** — `200`:
  `{"status":"ready","schema_applied":<int>,"schema_expected":<int>}`. `503`:
  the same three fields (best-effort; `schema_applied` may be null/omitted if
  the DB is unreachable) **plus** `"reason":"<enum>"`. Exact key names above are
  locked because Phase 19's readiness badge does a bare `fetch` against this
  and needs to parse it.
  — **Reversibility:** one-way — published HTTP contract consumed by Phase 19
  and (later) Phase 17; renaming a key is a breaking change.

- **D-04: `/ready` mirrors `/health`'s registration** — registered at that exact
  path on the root router in **both** the gate-configured and inert branches of
  `server.go` (structural exemption, never a path-string match), bounds its DB
  check with a short timeout (reuse the `healthPingTimeout = 3s` value or a
  sibling constant), queries the shared pool (no new connection), has no side
  effects, and never returns `401` on a gated instance carrying no cookie.

### `poll_runs` history

- **D-05: Overlap-skipped ticks coalesce to one `skipped_overlap` row per
  gap.** Between any two real (`ok` / `error` / `cancelled`) runs for a source,
  at most one `skipped_overlap` row exists; a repeated skip bumps a counter
  column on that row (e.g. `skipped_count`) and updates its `finished`/summary
  rather than inserting a new row. This satisfies RUN-02's "records a
  `skipped_overlap` row" while preventing a long cycle overrun from flushing
  real history through the prune (research Pitfall #4). The instrumentation
  point moves to where `ErrCycleInProgress` is observable (the failed
  `CompareAndSwap` in `runCycle`), and the recorder needs an
  "is there already an open skip row since this source's last real run?"
  check — an upsert, not a blind insert.
  — **Reversibility:** costly — the coalescing rule shapes the table's row
  semantics and the prune query; changing it later is a data-model change.

- **D-06: `skipped_overlap` is a valid `outcome` value** — it goes in the
  inline `CHECK` constraint alongside `ok`, `error`, `cancelled`. All four
  outcomes are enumerated in the `CREATE TABLE`'s inline `CHECK` (a later
  `ALTER TABLE ... ADD CHECK` is classified backward-incompatible by
  `cmd/migration-check` — see canonical refs).

- **D-07: A shutdown-interrupted cycle records a `cancelled` row via
  `context.WithoutCancel` + its own short timeout.** The recorder call detaches
  from the cancelled cycle context so the row still lands during graceful
  shutdown; its own short bound means a hung DB cannot measurably extend
  shutdown. A recorder that errors or hangs still leaves the cycle logging
  "poll cycle complete" and returning its normal result (RUN-03) — the recorder
  call is log-and-swallow, never propagated, never blocking the overlap guard's
  `defer running.Store(false)`.

- **D-08: Retention N = 50 per source** (compile-time constant, no env var —
  locked by REQUIREMENTS "out of scope" and RUN-04). Put it where a reader
  finds it (a named `const` in the `pollruns` package with a one-line comment).
  `skipped_overlap` coalescing (D-05) means the 50 real runs are not competed
  for by skip rows. Prune-on-write must be correct under both sources pruning
  near-simultaneously — research recommends a single atomic source-scoped
  INSERT+DELETE CTE with a keyset cutoff (mirrors Phase 11's
  `AdvanceGroupTrackCountBaseline` CTE pattern).

- **D-09: `summary` is a stored string, composed deterministically from the
  counts and outcome enum at insert time** — e.g. `"ok — 12 checked, 1 errored,
  3 events"`, `"error — 8 checked, 4 errored"`, `"skipped_overlap — previous
  cycle still running (x3)"`. Built only from the numeric/enum column values,
  never from a driver error, upstream response, DSN, webhook URL, or path. The
  "why" of an `error` outcome stays in the `cycle_id`-correlated structured
  logs. Stored (not composed downstream) so it is stable and greppable.

- **D-10: `poll_runs` columns** — `source`, `started`, `finished`,
  `artists_checked`, `artists_errored`, `events_recorded`, `outcome`,
  `summary`, plus the `skipped_count` counter for D-05. Index on
  `(source, started DESC)` per research. Pure-additive migration
  `000008_poll_runs` — bare `CREATE TABLE` + plain `CREATE INDEX`, all `CHECK`s
  inline, paired `.down.sql`, no `CREATE INDEX CONCURRENTLY`, no
  trigger/`CREATE FUNCTION` for the prune (the prune is application-side in the
  recorder). Regenerate and commit `queries/pollruns.sql` sqlc output locally —
  `make sqlc-check` has no CI counterpart.

### `/status` API (contract frozen here)

- **D-11: `/status` returns STAT-01's four things plus an `instance` object.**
  Shape (key names locked — Phase 19 types against them):
  - `runs`: last run per source (keyed or tagged by `source`)
  - `history`: the last N runs (N = 50 per source per D-08; ordered newest
    first)
  - `watchlist_size`: current count
  - `poll_interval`: the configured interval (seconds or a duration string —
    planner picks the encoding, document it)
  - `instance`: `{ "app_version": <string>, "schema_applied": <int>,
    "schema_expected": <int> }`
  "Database reachable" is implicit — a gated `/status` that responded at all
  proves it. Phase 19's readiness badge still does its own bare `/ready` call
  for the live check.
  — **Reversibility:** one-way — published contract; Phase 19 is built against
  it and must not re-guess it.

- **D-12: `/status` leaks nothing** — counts, timestamps, enum values, the
  composed `summary` string (D-09), the app version, and two integers only. No
  DSN, webhook URL, filesystem path, or raw driver error string on any field or
  any path. `/status` without a session returns `401` (gate enforcement is
  unchanged; `/status` sits inside the gated group). A DB failure serving
  `/status` logs raw to `httplog.SetAttrs` and returns a fixed body.

### App version

- **D-13: App version is injected at build time via `-ldflags -X`** into a
  `main`-package (or small `internal/buildinfo`) variable, fed the
  svu-computed tag. Local `go build` / `make build` without the flag falls back
  to `"dev"`. This adds a Dockerfile change and a `.github/workflows/full-pipeline.yml`
  change to this phase.
  — **Reversibility:** costly — touches the shared CI workflow file (the same
  shared-file hazard Phases 15/16/17 all hit) and the Dockerfile build stage.
  **Research item for the planner:** the svu tag is currently computed in the
  `release` job, which runs *after* `build-scan` builds/scans the image. Settle
  how the version reaches the image build given that ordering — options include
  computing svu earlier, passing it as a build arg, or accepting that the
  scanned image and the released image are built at different steps. Do not let
  this balloon the phase; a commit-SHA fallback for `build-scan`'s image and the
  real tag on the `release` image is acceptable if cleaner.

### Concurrency correctness (locked constraint, not a choice)

- **D-14: The per-cycle counters (`artists_checked`, `artists_errored`,
  `events_recorded`) are aggregated across `runCycle`'s worker goroutines with
  `sync/atomic` (or a channel fold), never plain ints.** `go test -race` is
  unavailable on the dev box and absent from CI (`.planning/WINDOWS.md`) — the
  usual backstop does not exist. The plan MUST carry an explicit
  concurrency-correctness section covering: atomic counters; the worker
  `recover()` path (`poller.go:337`) also incrementing `artists_errored`; and a
  looped (~1000×) exact-equality invariant test with both erroring and
  panicking artists. Treat the test as a required deliverable.

### Claude's Discretion

- **`events_recorded` source** — widen the `EventRecorder` seam
  (`DetectMusicBrainz` / `DetectDeezer`) to return `(int, error)`, **or**
  compute downstream in the recorder via
  `SELECT count(*) FROM events WHERE source = $1 AND created_at >= started`.
  Research leans seam-widening (more precise, but touches `internal/detection`
  signatures, both `fetchAndRecord` closures, and ~6 test call sites). The
  downstream count is safe because the per-source overlap guard means no other
  cycle of that source ran in the window. **Planner picks one explicitly and
  records it.** The rejected option is tracked as OBS-05. Do not hard-code the
  field to 0 while still displaying it.
- Exact JSON encoding of `poll_interval` (int seconds vs duration string),
  precise key names inside `runs` / `history`, and whether `instance` schema
  fields reuse the `/ready` key names — planner's call, just keep them
  internally consistent and documented for Phase 19.
- Whether `ExpectedSchemaVersion` lives in `internal/db` as an exported func or
  is computed once at boot and passed down as a value — implementation detail.

### Contract reconciliation settled

- **RUN-01 / RUN-02 vs research Pitfall #4:** resolved in favour of the
  requirements (D-05, D-06) — `skipped_overlap` rows ARE kept, but coalesced so
  a burst cannot evict real history. No REQUIREMENTS.md amendment needed.
- **RUN-01 "short outcome summary":** satisfied by the stored `summary` column
  (D-09). No RUN-01 amendment needed.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & research
- `.planning/REQUIREMENTS.md` — RDY-01…03, RUN-01…04, STAT-01/02 are the
  contract; the "Out of Scope" table (no retention env var, no per-artist
  rows, no metrics endpoint) binds this phase.
- `.planning/research/SUMMARY.md` — v1.4 research synthesis; the three
  cross-cutting decisions and the top-5 pitfalls.
- `.planning/research/PITFALLS.md` — counter race (#1), prune-on-insert race
  (#2), recorder failure/hang (#3), overlap-skip row eviction (#4),
  `/ready` `==` breaks rollback (#5). Read before planning 18.3/18.4.
- `.planning/research/ARCHITECTURE.md` — the seam list (`ExpectedSchemaVersion`,
  `ReadinessChecker`, `RunRecorder`, `pollruns.Recorder`, `StatusStore`).
- `.planning/research/STACK.md`, `.planning/research/FEATURES.md` — no new
  dependencies; feature scoping.

### Roadmap hand-off
- `.planning/ROADMAP.md` → "Phase 18" → "Notes for the phase planner" — the
  suggested 18.1–18.4 wave order, the highest-risk concurrency item, the
  migration + CI facts, and the security posture. **This is the primary
  planner brief; these decisions layer on top of it.**

### Codebase — existing seams and contracts this phase extends
- `internal/httpserver/health.go` — `handleHealth`, `healthResponse`,
  `healthPingTimeout`; `/ready` mirrors this exactly (D-04). `/health` contract
  is frozen (RDY-03).
- `internal/httpserver/server.go` — root router registration; the gated vs
  inert branch structure `/ready` and `/status` must both slot into.
- `internal/poller/poller.go` — `runCycle` (line ~270), the CAS overlap guard
  (`ErrCycleInProgress`, line ~278), the worker `recover()` path (line ~337),
  `nextCycleID` / `cycle_id`. Instrumentation target for 18.3.
- `internal/poller/poller_test.go` — `EventRecorder` seam assertion,
  `fakeEventRecorder`; the pattern any new seam double follows.
- `internal/db/migrate.go` — `maxSourceVersion` (line ~326), `runMigrationsOnce`
  ahead-of-source guard; source of `expected` for D-01. Also the home of the
  unexported `redactDSN` / `redactError` (promote to a shared exported package
  only if a free-text error genuinely must be stored — D-09 says it must not).
- `internal/db/migrations/README.md` — the expand/contract rule; read before
  writing `000008_poll_runs`.
- `cmd/migration-check/` — classifies `DROP`/`ALTER`/`RENAME`; a pure-additive
  `CREATE TABLE` with inline `CHECK`s produces zero findings (D-06, D-10).
- `queries/` + `sqlc.yaml` — where `queries/pollruns.sql` lands; regenerate
  sqlc output and commit it locally (no CI counterpart to `make sqlc-check`).
- `.github/workflows/full-pipeline.yml` — the `release` job computes the svu
  tag; `build-scan` builds the image before that. D-13's build-time version
  inject edits this file (shared-file hazard).
- `Dockerfile` — builder stage `go build -trimpath -ldflags="-w -s"` (line
  ~60); D-13 adds `-X`.

### Environment
- `.planning/WINDOWS.md` — `go test -race` unavailable (ThreadSanitizer alloc
  failure); no `-race` in CI either. Governs D-14.
- `.planning/PROJECT.md` "Context" — MusicBrainz TLS failure over this dev
  box's WSL2 is environmental and waived (affects any live MusicBrainz poll
  testing, not this phase's logic).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `handleHealth` / `healthResponse` / `healthPingTimeout` — `/ready` is a
  near-copy: same timeout-bounded DB check, same "raw error to `httplog`,
  fixed body to client" discipline.
- `maxSourceVersion(src)` in `internal/db/migrate.go` — already walks the
  embedded migrations to the highest version; `expected` for D-01 is this,
  exported or lifted.
- `runCycle`'s CAS guard + `cycle_id` machinery — the skip/cancel
  instrumentation points (D-05, D-07) already exist as observable events.
- Phase 11's `AdvanceGroupTrackCountBaseline` atomic CTE — the model for D-08's
  race-safe prune-on-write.
- `notifier.Select` / `notifier.NoOp` pattern — a `RunRecorder` that is always
  non-nil at the wiring site (a no-op recorder when unconfigured), so
  `runCycle` never nil-checks the seam (ARCHITECTURE.md anti-pattern).
- `EventRecorder` seam + `fakeEventRecorder` double — the exact pattern
  `RunRecorder` and its test double follow.

### Established Patterns
- Consumer-declared narrow seams (`watchlist.Store`, `poller.EventRecorder`,
  `httpserver.Pinger`); `RunRecorder`, `ReadinessChecker`, `StatusStore` follow
  suit — interface declared in the consumer, implemented in a new
  `internal/pollruns` package.
- Structural route exemptions in `server.go` (gated vs inert branch), never a
  path-string allowlist — `/ready` and `/status` register in the right branch.
- `ON CONFLICT` / atomic-CTE idempotency; per-operation `context.WithTimeout`
  on every DB call under the poller's signal-derived context (that context has
  no deadline, only cancellation — see ARCHITECTURE.md anti-pattern).
- sqlc codegen committed to the tree; `make sqlc-check` is the only drift gate
  (local-only).

### Integration Points
- `cmd/server/main.go` — the composition root: wires `RunRecorder` into
  `poller.New` (functional option, like `EventRecorder`), the
  `ReadinessChecker` / `StatusStore` into `httpserver.New`, and the app-version
  var into whatever reads it. 18.1–18.4 share this one wiring change.
- `internal/httpserver/server.go` — new routes on the root router (`/ready`
  ungated, `/status` in the gated group).
- `internal/poller/poller.go` `runCycle` — counter aggregation + the
  `RecordRun` call on the three exit paths (success, error, cancelled) and the
  skip path.

</code_context>

<specifics>
## Specific Ideas

- `/ready` `503` reason vocabulary is exactly three values: `db_unreachable`,
  `schema_behind`, `schema_dirty`. Not a free-form string.
- `summary` examples the operator should see: `"ok — 12 checked, 1 errored,
  3 events"`, `"error — 8 checked, 4 errored"`, `"cancelled — 5 checked,
  0 errored (shutdown)"`, `"skipped_overlap — previous cycle still running
  (x3)"`.
- App-version fallback string for a flagless local build: `"dev"`.
- History depth the operator can see: ~50 cycles per source ≈ 1–2 days at a
  typical 30–60 min poll interval.

</specifics>

<deferred>
## Deferred Ideas

- **Auto-refresh / live updates on the System view** — OBS-01, already deferred;
  no polling anywhere in this codebase and none to be added here.
- **Manual "poll now" trigger** — OBS-02, deferred.
- **Paginated / filterable poll-run history beyond the last N** — OBS-03,
  deferred; the 50-row window is the whole story for this milestone.
- **Poll-failure alerting (Discord alert after M consecutive errors)** — OBS-04,
  deferred.
- **`events_recorded` via the widened `(int, error)` seam** — OBS-05, the
  tracked fallback if the planner picks the downstream-count approach instead
  (or vice versa); only one lands in v1.4.
- **A dedicated `/version` endpoint** — considered for sourcing app version;
  rejected in favour of folding it into `/status`'s `instance` block (D-11) so
  the System view stays a single fetch.
- **Promoting `redactDSN` / `redactError` to a shared exported package** —
  only needed if a free-text error must be stored; D-09 avoids that, so this
  stays deferred unless the planner finds a genuine need.

</deferred>

---

*Phase: 18-backend-readiness-poll-run-history-status-api*
*Context gathered: 2026-09-09*
