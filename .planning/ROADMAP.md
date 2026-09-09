# Roadmap: drop-tracker

## Overview

drop-tracker was built outward from the data layer: Postgres schema + config + health-checked skeleton, then a tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications, then the embedded React UI, then single-image containerization and the full GitHub Actions CI/CD pipeline that is the actual point of the project. v1.1–v1.2 hardened it (frontend tests, CI coverage gates, event retention, concurrent polling, display-bug cleanup). v1.3 delivered the deployment-readiness chain that needs no host — passphrase gate, PR coverage-diff comment, rollback-safe migrations — and deferred the actual VPS deploy until hardware exists. v1.4 makes the running service legible: a readiness probe distinct from liveness, an in-process poll-cycle history behind a DB-free seam (`docs/adr/0001` — a ring buffer, not a table), a gated `/status` JSON contract, and an operator System panel in the SPA. Phase 18 was split into 18 (additive) + 18.1 (the risky `runCycle` instrumentation) after a design grilling.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-7 (shipped 2026-08-12)
- ✅ **v1.1 Hardening & Scale Readiness** — Phases 8-11.1 (shipped 2026-08-17)
- ✅ **v1.2 Cleanup & Display Fixes** — Phases 12-13 (shipped 2026-08-24)
- ✅ **v1.3 Continuous Deployment** — Phases 14-17 (shipped partial 2026-09-09; **Phase 17 deferred**)
- 🔄 **v1.4 Operator Observability** — Phases 18, 18.1, 19 (in progress; Phase 18 was split into 18 + 18.1 after a design grilling)

Full phase-by-phase detail for every shipped milestone is archived under `.planning/milestones/v[X.Y]-ROADMAP.md`. Requirement archives: `.planning/milestones/v[X.Y]-REQUIREMENTS.md`. Accomplishment summaries: `.planning/MILESTONES.md`.

**Deferred:** **Phase 17 — Automated VPS Deploy with Health-Gated Rollback** (DPLY-01…08). Blocked on a provisioned VPS + domain the developer does not have yet. `discuss-phase` context was already gathered — archived at `.planning/milestones/v1.3-phases/17-automated-vps-deploy-with-health-gated-rollback/` (`17-CONTEXT.md`, `17-DISCUSSION-LOG.md`). Un-defer it as its own milestone cycle once a box exists; the `/ready` probe from v1.4 is being built for its health-gate to consume.

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-7) — SHIPPED 2026-08-12</summary>

- [x] Phase 1: Foundation — Data Layer, Config & Health
- [x] Phase 2: Watchlist Core
- [x] Phase 3: External Clients & Search
- [x] Phase 4: Detection Engine
- [x] Phase 5: Discord Notifications
- [x] Phase 6: Frontend & Release History
- [x] Phase 7: Containerization & CI/CD Pipeline

</details>

<details>
<summary>✅ v1.1 Hardening & Scale Readiness (Phases 8-11.1) — SHIPPED 2026-08-17</summary>

- [x] Phase 8: Frontend Test Suite
- [x] Phase 9: CI Coverage Gates
- [x] Phase 10: Event Retention Window
- [x] Phase 11: Bounded Concurrent Polling
- [x] Phase 11.1: Address tech debt: v1.1 cleanup (INSERTED)

</details>

<details>
<summary>✅ v1.2 Cleanup & Display Fixes (Phases 12-13) — SHIPPED 2026-08-24</summary>

- [x] Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking
- [x] Phase 13: Fix History Dates, Guest-Feature Art & Artist Art

</details>

<details>
<summary>✅ v1.3 Continuous Deployment (Phases 14-17) — SHIPPED PARTIAL 2026-09-09</summary>

- [x] Phase 14: Instance Passphrase Gate (7/7 plans) — completed 2026-09-01
- [x] Phase 15: PR Coverage-Diff Comment (3/3 plans) — completed 2026-09-03
- [x] Phase 16: Rollback-Safe Migrations (5/5 plans) — completed 2026-09-05
- [ ] Phase 17: Automated VPS Deploy with Health-Gated Rollback — **DEFERRED** (no VPS); context archived, carries forward as its own milestone

</details>

### 🔄 v1.4 Operator Observability (Phases 18, 18.1, 19) — IN PROGRESS

- [ ] **Phase 18: Backend — Readiness, Status Surface & App Version** - `/ready` probe, in-process run-history ring buffer + seams, gated `GET /status` (contract frozen here), SHA-based app version
- [ ] **Phase 18.1: Poll-Cycle Instrumentation** - the `runCycle` change: widened `EventRecorder`, channel-fold counters, `RecordRun`/`RecordSkip` wiring, concurrency-invariant test
- [ ] **Phase 19: Frontend — System View** - operator status panel in the SPA rendering `/status`

> **Split rationale (2026-09-09 design grilling).** The original single Phase 18 bundled a schema migration, two endpoints, a shared-CI-file change, *and* the riskiest concurrency change in the codebase (`runCycle` counter aggregation with no `go test -race` anywhere). It was split so the risky `runCycle` edit (18.1) is reviewed in isolation from the additive work (18). The grilling also replaced the `poll_runs` **table** with an **in-process ring buffer** (`docs/adr/0001`) — eliminating the prune-on-insert race, the cross-source deadlock, the skip-row-eviction hazard, the migration, and the local-only `sqlc-check` drift gate. Superseded CONTEXT decisions: D-05 (skip coalescing → separate per-source signal), D-06/D-08/D-10 (`poll_runs` table → ring buffer), D-07 (detached-context recorder call → synchronous in-memory append), D-13 (svu `-ldflags` → `${GITHUB_SHA}` build-arg). `events_recorded` is locked to seam-widening (not the downstream count — clock-skew undercount).

## Phase Details

### Phase 18: Backend — Readiness, Status Surface & App Version
**Goal**: An operator (or a machine) can tell a live drop-tracker apart from a merely-running process, and the `/status` JSON contract Phase 19 types against exists and is frozen — all without touching the poll cycle's hot path.
**Depends on**: Nothing (builds on shipped v1.3 code)
**Requirements**: RDY-01, RDY-02, RDY-03, RUN-02 (skip signal), RUN-04, STAT-01, STAT-02
**Success Criteria** (what must be TRUE):
  1. An unauthenticated `GET /ready` against a healthy instance returns `200` carrying `schema_applied` and `schema_expected`; with Postgres stopped, or the schema behind or dirty, the same request returns `503` with a machine reason (`db_unreachable` / `schema_behind` / `schema_dirty`) and no DSN, driver text, or internal path — and it answers identically (never `401`) on a passphrase-gated instance carrying no session cookie.
  2. `/health` on the same build answers exactly as it did in v1.3 — same status code, same body, same timeout behaviour. Readiness was added beside liveness, not on top of it.
  3. `GET /status` behind the gate returns JSON with: the last run per source, the last N runs per source, each source's last-skipped timestamp + consecutive-skip count, the current watchlist size, the configured poll interval, and an `instance` block (`app_version`, `schema_applied`, `schema_expected`). The same request without a session returns `401`. On a fresh instance with no cycles yet the run lists are empty (not an error). No response field on any path contains a DSN, webhook URL, filesystem path, or raw driver error string.
  4. `app_version` is the build's short commit SHA on a CI-built image and `"dev"` on a flagless local build; the image the `release` job pushes is byte-for-byte the one `build-scan` scanned (the single-build guarantee, 07-REVIEW CR-02, is intact).
  5. `internal/pollruns.Store` holds the last N (=50) run entries per source in memory behind a mutex; the `poller.RunRecorder` seam interface exists and is wired to the real store, inert only because `runCycle` does not call it yet (that is Phase 18.1). `StatusStore` reads the buffer for `/status`.
**Plans**: 4 plans

Plans:
- [ ] 18-01-PLAN.md — `/ready` probe: `db.ExpectedSchemaVersion` + `db.SchemaVersion`, the `SchemaVersioner` seam, `handleReady`, root-router registration in both branches, boot wiring (wave 1)
- [ ] 18-02-PLAN.md — `internal/pollruns` ring buffer + the `poller.RunRecorder` seam, no-op default and `WithRunRecorder` — declared and inert, `runCycle` untouched (wave 1)
- [ ] 18-03-PLAN.md — app version: `internal/buildinfo`, Dockerfile `VERSION` argument + link flag, CI build argument and a provenance assertion (wave 1)
- [ ] 18-04-PLAN.md — gated `GET /status`: `CountWatchlist`, the `StatusStore`/`WatchlistCounter` seams, `handleStatus`, the frozen contract in `docs/api/status-contract.md`, composition-root wiring (wave 2)

**Notes for the phase planner**

*Scope discipline.* This phase does **not** touch `internal/poller/poller.go`'s `runCycle` or `internal/detection`. If a task needs to, it belongs in Phase 18.1. The `RunRecorder` interface + a no-op default + real-store wiring in `main.go` is in scope; calling `RecordRun` from `runCycle` is not.

*`/ready` (RDY-01..03).* `db.ExpectedSchemaVersion()` — new exported func in `internal/db/migrate.go` reusing the existing `maxSourceVersion` walk over the embedded FS, called **once** at boot in `main.go`, passed into `httpserver.New`. `db.SchemaVersion(ctx, pool)` — hand-rolled pgx `SELECT version, dirty FROM schema_migrations` (that table is golang-migrate-owned, not in the schema dir, so it cannot be sqlc). Ready condition: `!dirty && applied >= expected` (D-01 — Phase 16's ahead-of-source guard deliberately lets a rolled-back binary serve a newer additive schema; strict `==` would report that healthy instance not-ready forever and flap the deferred Phase 17 gate). `/ready` registered on the **root router in both the gated and inert branches**, mirroring `/health` (D-04), 3s-bounded DB read (reuse `healthPingTimeout` or a sibling const), raw error to `httplog.SetAttrs` only. Add a test asserting non-`401` on a gated server with no cookie.

*Run-history store (RUN-04, RUN-02 skip signal).* `internal/pollruns.Store` — a struct holding, per source, a ring buffer (`[]RunResult`, cap `const N = 50`, one-line comment saying where the number comes from) plus `lastSkippedAt time.Time` and `consecutiveSkips int`, all under one `sync.Mutex` (or `RWMutex`). Methods: `RecordRun(ctx, RunResult) error` and `RecordSkip(source string)` (satisfying the `poller.RunRecorder` seam, declared in `internal/poller`), and `Recent(...)` for `httpserver.StatusStore`. `ctx` stays in `RecordRun`'s signature for seam symmetry with `EventRecorder`/`Notifier` even though the in-memory append never blocks on it. `RunResult` fields: `Source`, `CycleID`, `StartedAt`, `FinishedAt`, `DurationMS`, `ArtistsChecked`, `ArtistsSkipped`, `ArtistsErrored`, `EventsRecorded`, `Outcome` (`ok`/`error`/`cancelled`), `Summary` (composed deterministically from the counts + outcome at record time — never from error text). Document `ArtistsChecked` = "entries dispatched to a worker" in the struct comment. `RecordSkip` bumps `consecutiveSkips` and stamps `lastSkippedAt`; `RecordRun` resets `consecutiveSkips` to 0. This store is the seam both `poller` (write) and `httpserver` (read) get wired to at the composition root — the poller still holds no DB handle *and* no direct store reference beyond the interface.

*`/status` (STAT-01/02).* Gated `GET /status` in `registerDataRoutes` (inherits `gate.Authenticate` + the `X-Instance-Gated` header for free; `GET` so CSRF-exempt). `StatusStore` seam declared in `httpserver`, implemented by `pollruns.Store`. The `instance` block needs `schema_applied` (live `db.SchemaVersion` read — same helper as `/ready`), `schema_expected` (the boot value), `app_version` (the build var). `watchlist_size` needs **one new `CountWatchlist` sqlc query** in `queries/watchlist.sql` (`SELECT count(*)` — there is none today; this is the right house pattern, not a hand-rolled count in the handler) — regenerate sqlc and run `make sqlc-check` locally (no CI counterpart). `poll_interval` encoding (int seconds vs duration string), the exact key names inside `runs`/`history`, and whether `instance` reuses `/ready`'s key names — planner's call, just keep them internally consistent and **document the frozen shape for Phase 19** (this is the contract-freeze point; Phase 19 must not start against a guess). DB-failure path on `/status`: raw error to `httplog.SetAttrs`, fixed body, never driver text.

*App version (feeds STAT-01's `instance` block, SYS-02).* `--build-arg VERSION=${{ github.sha }}` on the `docker/build-push-action` step in `build-scan` (SHA is free — no svu, no `fetch-depth: 0` change); `ARG VERSION` + `-ldflags "-X <pkg>.Version=$VERSION"` in the Dockerfile builder stage (~line 60); a `main`-package or small `internal/buildinfo` var, `"dev"` when the flag is absent. This edits `.github/workflows/full-pipeline.yml` and `Dockerfile` — the shared-CI-file hazard Phases 15/16/17 all hit, but the change is one line each and does not touch the `release` job or the single-build guarantee. Show a short SHA (first 7-12 chars) in the about block.

*Security posture (non-negotiable, inherited).* Run entries carry **counts + an outcome enum + a composed summary string only** — no free-text error, no driver text. Per-artist failure detail stays in the `cycle_id`-correlated structured logs. `/ready` and `/status` DB failures log raw to `httplog.SetAttrs` and return a fixed body. No `redactDSN`/`redactError` promotion needed because nothing free-text is stored.

### Phase 18.1: Poll-Cycle Instrumentation
**Goal**: Every poll cycle records exactly one run entry through the `RunRecorder` seam — with correct counts even under the worker fan-out — and polling stays green no matter what the recorder does.
**Depends on**: Phase 18 (the `pollruns.Store`, the `RunRecorder` seam interface, and `RunResult` all exist; this phase makes `runCycle` call them)
**Requirements**: RUN-01, RUN-02 (cancelled entry), RUN-03
**Success Criteria** (what must be TRUE):
  1. After a poll interval elapses, the run buffer holds exactly one entry per source per cycle carrying source, started/finished, artists checked, artists skipped, artists errored, events recorded, an outcome, and a leak-free summary — and a shutdown-interrupted cycle appears with a `cancelled` outcome rather than going missing.
  2. An overlap-skipped tick records **no** run entry; it bumps that source's consecutive-skip count and last-skipped timestamp (visible in `/status`), and the next real run resets the count.
  3. The per-cycle `artists_checked` / `artists_skipped` / `artists_errored` / `events_recorded` counts are exact under the worker fan-out: a looped (~1000×) invariant test with a fake source where K artists error and P panic asserts `checked == errored + succeeded` and `errored == K + P` every iteration. Aggregation is a **channel fold** (each worker emits exactly one result value; the parent folds single-threaded after `wg.Wait()`), not shared mutable ints — `go test -race` is unavailable locally and absent from CI, so correctness is by construction, not by detector.
  4. Polling stays green regardless of the recorder: a recorder that errors still leaves the cycle logging "poll cycle complete" and returning its normal result; the `RecordRun` call never wedges a source's overlap guard (`defer running.Store(false)`) and never measurably extends shutdown.
  5. `events_recorded` is real: `EventRecorder.DetectMusicBrainz` / `DetectDeezer` return `(int, error)`; the count threads back through both `fetchAndRecord` closures and is folded with the other counters. It is never hard-coded to 0 while displayed.
**Plans**: TBD

**Notes for the phase planner**

*This is the milestone's single riskiest change — keep it minimal.* Do not restructure `runCycle`'s ctx-cancellation races or per-worker panic isolation (`poller.go:293-377` are carefully reasoned). The change is: (1) `fetchAndRecord` gains an `(int, error)` return; (2) `EventRecorder.DetectMusicBrainz`/`DetectDeezer` widen to `(int, error)` — touches `internal/detection` + ~6 test call sites, mechanical; (3) a per-cycle result channel sized to the dispatched count, one send per worker (including the `recover()` path, which must emit an errored result — restructure the worker body so the recover returns a value rather than a bare `defer`); (4) fold after `wg.Wait()`; (5) a `defer`-registered `RecordRun` call after the CAS succeeds so it fires on the `ok` / `error` / `cancelled` exit paths but **not** the pre-CAS skip return; (6) a `p.runs.RecordSkip(source)` call at the `!running.CompareAndSwap` branch before returning `ErrCycleInProgress`. `RecordRun` failure is **logged and swallowed**, exactly like `NotifyPending` (`poller.go:394`). No `context.WithoutCancel` needed — the in-memory append is synchronous and returns in microseconds even during shutdown (this is why D-07's detached-context dance is gone).

*`artists_checked` vs `artists_skipped`.* MB dispatches every entry (`artists_skipped` always 0). Deezer's `shouldDispatch` skips nil-`deezer_id` entries *before* the semaphore — those are `artists_skipped`, never `artists_checked`. `artists_checked` = entries actually handed to a worker. The two plus errored must reconcile against the watchlist size for the operator.

*`duration_ms` / `finished_at`.* Measure at the existing "poll cycle complete" log point (after `wg.Wait()`, before `NotifyPending`), consistent with the current log line — notifier delivery time is excluded.

*`outcome` is `ok | error | cancelled` only.* No `partial` — a cycle that finished with some artists errored is `ok` with `artists_errored > 0`; the derived "degraded" label is Phase 19's job, not stored data. `error` = the cycle machinery failed (e.g. watchlist `List` errored). `cancelled` = `cycleErr` is `context.Canceled`/`DeadlineExceeded`.

*Concurrency-correctness section is a required plan deliverable* (the project's stated substitute for `-race`). It must spell out: the channel is buffered to the dispatched count so no worker blocks on send; every worker path emits exactly one result (success / errored / panicked / ctx-bailed); the fold is single-threaded; the looped ~1000× exact-equality test with erroring *and* panicking fake artists.

### Phase 19: Frontend — System View
**Goal**: An operator opens the app and can see, on one screen, whether the scheduler is doing its job — per source, right now and over the last N cycles.
**Depends on**: Phase 18 (freezes the `/status` JSON contract — `web/app/lib/api.ts`'s discipline is to type against the real Go response body, not a guess). Phase 18.1 populates the run data but does not change the contract, so Phase 19 can start once Phase 18 lands; the first-run empty state it must build anyway covers the window before 18.1.
**Requirements**: SYS-01, SYS-02, SYS-03
**Success Criteria** (what must be TRUE):
  1. A "System" tab in the SPA's main navigation opens a System view showing, for each source, the last run's time, outcome, duration, and counts, plus how long it has been since that source's last successful run.
  2. The same view shows a recent-runs history table, the current watchlist size, the configured poll interval, and an about block (app version, schema version, database reachable).
  3. On a freshly deployed or freshly migrated instance with no poll history yet — the normal state for the first poll interval after every deploy — the view shows explicit first-run copy, distinct from both the error state and the loaded-but-empty state. No endless spinner, no blank card, no "Invalid Date".
  4. The view fetches once on mount and otherwise only when the operator clicks Refresh; leaving the tab open produces no steady `/status` traffic, and the panel shows an explicit "as of" timestamp so the operator can tell how fresh it is.
  5. When the session expires, the view yields to the existing passphrase screen and re-fetches after login, with no stray requests firing behind the login screen.
**Plans**: TBD
**UI hint**: yes

**Notes for the phase planner**

- Run `/gsd-ui-phase 19` first — this phase needs its own UI-SPEC before planning.
- Structure mirrors the shipped `history.tsx` pattern exactly (fetch-on-mount, `initialLoading` skeleton, three-way empty/error/first-run copy, Retry). There is **no polling anywhere in this SPA today** — don't invent one. If auto-refresh is ever wanted it belongs in OBS-01, and even then no faster than 30–60s with a `visibilitychange` pause and a `clearInterval` cleanup.
- Route through the existing `apiFetch` so the System view inherits the D-16 global-401 interceptor and the `X-Instance-Gated` latch for free. `/ready` is not wrapped (unauthenticated, ops-only) — if the about block wants a readiness badge it needs a bare `fetch` that tolerates a 503 body.
- Guard every timestamp render against `null`, and handle watchlist size 0 with its own copy. Render outcomes in human phrasing, not raw enum values.
- Definition of Done: `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before staging, then `corepack pnpm test` — hand-formatted TSX fails CI's `frontend-test` job.

## Progress

- **v1.0–v1.2:** shipped.
- **v1.3:** shipped partial — 3 of 4 phases (14, 15, 16). Phase 17 deferred, DPLY-01…08 carried forward.
- **v1.4:** in progress.

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 18. Backend — Readiness, Status Surface & App Version | 0/4 | Planned | - |
| 18.1. Poll-Cycle Instrumentation | 0/TBD | Not started | - |
| 19. Frontend — System View | 0/TBD | Not started | - |

## Backlog

*(none currently)*
