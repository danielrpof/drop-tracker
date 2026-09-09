# Roadmap: drop-tracker

## Overview

drop-tracker was built outward from the data layer: Postgres schema + config + health-checked skeleton, then a tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications, then the embedded React UI, then single-image containerization and the full GitHub Actions CI/CD pipeline that is the actual point of the project. v1.1–v1.2 hardened it (frontend tests, CI coverage gates, event retention, concurrent polling, display-bug cleanup). v1.3 delivered the deployment-readiness chain that needs no host — passphrase gate, PR coverage-diff comment, rollback-safe migrations — and deferred the actual VPS deploy until hardware exists. v1.4 makes the running service legible: a readiness probe distinct from liveness, a persisted poll-cycle history behind a DB-free seam, a gated `/status` JSON contract, and an operator System panel in the SPA.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-7 (shipped 2026-08-12)
- ✅ **v1.1 Hardening & Scale Readiness** — Phases 8-11.1 (shipped 2026-08-17)
- ✅ **v1.2 Cleanup & Display Fixes** — Phases 12-13 (shipped 2026-08-24)
- ✅ **v1.3 Continuous Deployment** — Phases 14-17 (shipped partial 2026-09-09; **Phase 17 deferred**)
- 🔄 **v1.4 Operator Observability** — Phases 18-19 (in progress)

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

### 🔄 v1.4 Operator Observability (Phases 18-19) — IN PROGRESS

- [ ] **Phase 18: Backend — Readiness, Poll-Run History & Status API** - `/ready` probe, `poll_runs` + `RunRecorder` seam, gated `GET /status`
- [ ] **Phase 19: Frontend — System View** - operator status panel in the SPA rendering `/status`

## Phase Details

### Phase 18: Backend — Readiness, Poll-Run History & Status API
**Goal**: An operator (or a machine) can tell a live drop-tracker apart from a merely-running process, and can see what every poll cycle actually did, without opening container logs or a psql shell.
**Depends on**: Nothing (builds on shipped v1.3 code)
**Requirements**: RDY-01, RDY-02, RDY-03, RUN-01, RUN-02, RUN-03, RUN-04, STAT-01, STAT-02
**Success Criteria** (what must be TRUE):
  1. An unauthenticated `GET /ready` against a healthy instance returns `200` naming the applied schema version; with Postgres stopped, or the schema behind or dirty, the same request returns `503` with a short reason and no DSN, driver text, or internal path — and it answers identically (never `401`) on a passphrase-gated instance carrying no session cookie.
  2. `/health` on the same build answers exactly as it did in v1.3 — same status code, same body, same timeout behaviour. Readiness was added beside liveness, not on top of it.
  3. After a poll interval elapses, `poll_runs` holds exactly one row per source per cycle carrying source, started/finished, artists checked, artists errored, events recorded, an outcome, and a summary that leaks nothing — and an overlap-skipped tick and a shutdown-interrupted cycle each appear with their own distinguishable outcome rather than going missing.
  4. Polling stays green regardless of the recorder: a recorder that errors, or hangs, still leaves the cycle logging "poll cycle complete" and returning success, never wedges a source's overlap guard, and never measurably extends shutdown — and `poll_runs` never exceeds N rows for either source, including when both sources finish a cycle at the same instant.
  5. `GET /status` behind the gate returns JSON with the last run per source, the last N runs, the current watchlist size, and the configured poll interval; the same request without a session returns `401`; and no response field on any path contains a DSN, webhook URL, filesystem path, or raw driver error string.
**Plans**: TBD

**Notes for the phase planner**

*Suggested internal build order (waves within this one phase — NOT separate roadmap phases).* Research proposed a 18.1–18.4 breakdown; treat it as plan sequencing inside Phase 18:
- **18.1 `GET /ready`** — independent of everything else, lowest risk, ships standalone operator value and unblocks the deferred Phase 17 deploy gate. `db.ExpectedSchemaVersion()` (reusing the existing `maxSourceVersion` walk) + a `ReadinessChecker` seam + `httpserver/ready.go` registered on the root router in **both** the gated and inert branches, mirroring `/health`.
- **18.2 `poll_runs` migration + sqlc queries** — pure schema/codegen; unblocks everything downstream. Do it after 18.1 so two concurrent edits to `internal/db` don't collide.
- **18.3 `pollruns.Recorder` + `poller.RunRecorder` seam + `runCycle` instrumentation** — the single riskiest change in the milestone; give it its own reviewable slice. The poller must stay DB-connection-free (the seam is the whole point); the recorder call is log-and-swallow with its own `context.WithoutCancel` + short timeout so a shutdown-cancelled cycle still records and a hung DB can't wedge the overlap guard.
- **18.4 `pollruns.Store` + gated `GET /status`** — thin once 18.3 exists. **This wave freezes the `/status` JSON contract that Phase 19 types against — do not start Phase 19 before it lands.**

*Highest-risk item — concurrency correctness.* `runCycle` fans every watchlist entry across worker goroutines (default 3 MusicBrainz / 5 Deezer). The per-cycle `artists_checked` / `artists_errored` / `events_recorded` counters are produced inside those workers, and `go test -race` is **unavailable on this dev box** (ThreadSanitizer allocation failure under WSL2, `.planning/WINDOWS.md`) **and absent from CI**. The usual backstop does not exist here. Phase 18's plan must carry an explicit **concurrency-correctness section** covering: `sync/atomic` counters (or a channel fold) rather than plain ints; the worker `recover()` path also incrementing errored; and a looped (~1000×) exact-equality invariant test with both erroring and panicking artists. A plausible-but-wrong count will otherwise ship silently.

*Three decisions the planner must lock (research flagged, deliberately left open):*
- **(a) `/ready` ready-condition — strict `applied == expected` vs `applied >= expected && !dirty`.** Research leans `>=`: Phase 16's ahead-of-source guard deliberately lets a rolled-back binary boot and serve against a newer additive schema, and strict `==` would report that healthy instance not-ready forever — flapping the future Phase 17 deploy gate. Confirm against the Phase 17 deploy model and document the rationale in the handler comment.
- **(b) `events_recorded` source — widen the `EventRecorder` seam to `(int, error)` vs compute downstream.** Widening is more precise but touches `internal/detection` signatures, both `fetchAndRecord` closures, and ~6 test call sites. The downstream alternative is `SELECT count(*) FROM events WHERE source = $1 AND created_at >= started_at` inside `pollruns.Recorder`, safe because the per-source overlap guard means no other cycle of that source ran in the window. Either is valid; pick one explicitly. (The rejected option is tracked as OBS-05.) Do **not** hard-code the field to 0 while still displaying it.
- **(c) Retention `N` for `poll_runs`** — a compile-time constant, no new env var (locked). Research suggests 50–100 per source; 50 is conservative, 100 generous. Pick a number and put it where a reader finds it.

*Contract reconciliation the planner must settle.* RUN-01/RUN-02 require a `skipped_overlap` outcome row for an overlap-skipped tick; the research (Pitfall #4) argues the opposite — skipped ticks write no row, because a burst of junk rows during one slow cycle would evict real history through the prune. The requirements are the contract; if `skipped_overlap` rows are kept, the plan needs the outcome value in the `CHECK` constraint, the instrumentation point moved to where `ErrCycleInProgress` is observable, and an explicit answer for how a burst of skips is prevented from evicting real runs. If the research position wins instead, RUN-02 must be amended rather than quietly ignored.

*Migration + CI facts (verified, not assumptions):*
- `000008_poll_runs.up.sql` is **pure-additive** — a bare `CREATE TABLE` plus a plain `CREATE INDEX`. `cmd/migration-check` only classifies `DROP`/`ALTER`, so it produces zero findings, and the D-15 previous-release cross-reference cannot fire on a brand-new object. Keep every `CHECK` **inline in the `CREATE TABLE`** (a later `ALTER TABLE ... ADD CHECK` is classified backward-incompatible), no `CREATE INDEX CONCURRENTLY` (golang-migrate wraps each file in a transaction), no trigger/`CREATE FUNCTION` for the prune, and ship the paired `.down.sql`.
- **N-1 holds automatically:** the previous release's binary never queries `poll_runs`, so it boots unchanged against the new schema — that is what the `n1-boot` job proves. Number strictly ascending; never renumber or edit a released migration.
- **`make sqlc-check` has no CI counterpart** (CLAUDE.md). Adding `queries/pollruns.sql` requires `sqlc generate` + committing the generated output, and only the local gate catches drift — CI will stay green while the tree is wrong.

*Security posture (non-negotiable, inherited).* Prefer storing **no free-text error** in `poll_runs` — counts plus an outcome enum, with per-artist detail staying in the `cycle_id`-correlated structured logs. If a message is genuinely required, promote `internal/db`'s unexported `redactDSN`/`redactError` into a shared exported package and golden-test the status path against DSN-bearing and webhook-bearing errors. `/ready` and `/status` DB failures log raw to `httplog.SetAttrs` and return a fixed body, never driver text.

### Phase 19: Frontend — System View
**Goal**: An operator opens the app and can see, on one screen, whether the scheduler is doing its job — per source, right now and over the last N cycles.
**Depends on**: Phase 18 (the `/status` JSON contract must be frozen first — `web/app/lib/api.ts`'s discipline is to type against the real Go response body, not a guess)
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
| 18. Backend — Readiness, Poll-Run History & Status API | 0/TBD | Not started | - |
| 19. Frontend — System View | 0/TBD | Not started | - |

## Backlog

*(none currently)*
