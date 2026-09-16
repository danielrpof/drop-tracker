# Milestones

## v1.4 Operator Observability (Shipped: 2026-09-11)

**Phases completed:** 3 phases, 12 plans, 31 tasks
**Known verification overrides:** 0 newly acknowledged, 4 carried forward from a prior close (see STATE.md Deferred Items)

**Key accomplishments:**

- `GET /ready` — an unauthenticated, 3s-bounded readiness probe that tells a live drop-tracker apart from a merely-running process via a machine reason enum, added beside `/health` without changing liveness in any observable way.
- `internal/pollruns.Store` — a mutex-guarded per-source ring of the last 50 `RunResult` values plus a scalar skip signal, with `RecordRun` owning summary composition and outcome normalization, and a wired-but-inert `poller.RunRecorder` seam ready for Phase 18.1's call site.
- The shipped binary now carries the commit it was built from — `internal/buildinfo.Version` injected via `-ldflags -X` in the Dockerfile, `${{ github.sha }}` passed by CI's `build-scan` job, `"dev"` for any flagless local build, and a pipeline step that greps the built binary to catch a silently-ignored link flag.
- `GET /status` ships as a gated JSON operator panel — poll interval, watchlist size, an instance block, and a per-source object carrying the last run, the last 50 runs newest-first, and the skip signal — with its wire shape frozen in `docs/api/status-contract.md` and one `pollruns.Store` wired at the composition root as both the poller's recorder and the server's reader.
- `EventRecorder` widened to `(int, error)` across every layer and call site; `runCycle` now folds per-worker `artistResult` values single-threaded off a `len(entries)`-buffered channel and writes exactly one `pollruns.RunResult` per completed cycle through a `defer`-registered `RecordRun`.
- Four new `internal/poller` unit tests pin every non-happy `runCycle` exit: an overlap-skipped tick that writes a skip signal and no history entry, a watchlist `List` failure recorded as `outcome=error` with zero counters and no leaked driver text, a shutdown-cancelled cycle (mid-flight and pre-dispatch) recorded as `outcome=cancelled`, and a recorder that errors or stalls without ever failing, retrying, or wedging the cycle.
- `TestRunCycle_CounterInvariant` — a 1000-iteration, exact-equality invariant test that drives `runCycle`'s fan-out with 3 erroring and 2 panicking artists (plus a Deezer leg with 4 nil-`deezer_id` entries) and asserts every recorded counter reconciles on every iteration — plus a full green Definition-of-Done gate with the `-race` substitution recorded as WINDOWS.md entry 13 and `18.1-VALIDATION.md` signed off.
- A real `/system` nav tab, route, and `getStatus()` wrapper that fetch `GET /status` once on mount through the existing `apiFetch` 401 pipeline and render `instance.app_version` as live data.
- `web/app/lib/format.ts` — the six pure formatters D-09/D-09-a/SYS-01/SYS-02 require (relative time, absolute time, ISO title, clock stamp, duration, poll interval), table-driven-tested under a `TZ=UTC` pin proven by a self-check case.
- `sourceDisplayName`/`SOURCE_ORDER` in `sources.ts`, two run-health `@theme` tokens in `app.css`, and a hand-written (not CLI-trusted) shadcn `Table` family in `web/app/components/ui/table.tsx` — three leaf primitives for plans 19-04/19-05 with no file overlap with 19-02.
- The five D-07 outcome badge tiers, the schema/reachability About block, and the per-source panel (D-08 clean-run scan, skip/escalation lines) wired into `system.tsx`'s loaded body -- the phase's real operator-facing surface area.
- The per-source recent-runs table with its cap caption, the finished five-state render machine (first-run predicate, error-hides-Refresh, keep-stale Refresh with its own re-entrancy/mounted guards, and the as-of freshness stamp), and the phase-closing embedded SPA bundle rebuild.

---

## v1.3 Continuous Deployment (Shipped: 2026-09-09)

**Phases completed:** 4 phases, 15 plans, 46 tasks

**Key accomplishments:**

- Stateless HMAC-SHA256 session-cookie gate: `internal/authgate` (codec + `Manager` middleware + `/session` handlers + `Alerter` seam), an `httpserver.WithAuthGate` functional option that moves the six data routes behind a protected chi Group, and the `INSTANCE_PASSPHRASE` / `TRUST_PROXY_HEADERS` config + boot wiring — proven end-to-end (401 → login → 200 → logout) and inert when unconfigured.
- Per-IP `golang.org/x/time/rate` login throttle (burst 5, `rate.Every(12s)`, `429` on the sixth attempt), a fixed 250ms–1s jittered delay wrapping only the two passphrase-comparison paths, a `maxConcurrentLogins=32` semaphore that sheds excess with an undelayed `503`, an alert-only process-wide failed-attempt counter (20 within 5m → one Discord alert per 15m cooldown) posting through the existing webhook sink, and one structured `slog` audit line per auth outcome carrying `source_ip` — the passphrase reaches no log line on any path.
- A framework-free `authStore` (authed + D-18 `gateActive`) poked by a single `apiFetch` 401 interceptor, the verbatim-approved full-screen `<PassphraseScreen>`, and a `gateActive`-gated Log out control — so a gated instance renders a login prompt instead of a broken page and a successful login restores the watchlist/history UI with fresh data, no reload.
- `authgate.RequireCSRFHeader` on the protected Group and at the top of `HandleLogin`/`HandleLogout` (403 `{"error":"missing required header"}` on any gated state-changing request lacking `X-Requested-With: drop-tracker`), a `securityResponseHeaders` middleware setting `Referrer-Policy: no-referrer` on every response, and `authgate.IsWeakPassphrase` feeding a single non-blocking boot WARN — the phase's last hardening layer, landing after the SPA already sends the header so nothing breaks.
- `docker-compose.yml` now forwards `INSTANCE_PASSPHRASE` / `TRUST_PROXY_HEADERS` as `${VAR:-default}` interpolations (with `env_file: .env` still primary), `cmd/server` emits one secret-free Info line per boot stating whether the instance gate is active or inert, `TestDockerComposeWiresGateEnvVars` turns a dropped gate env entry into a CI failure instead of a UAT surprise, and the operator reconciled the live `.env` — the passphrase gate that silently did nothing through a whole Phase 14 UAT round is now engaged and observable.
- A gated instance now marks every response that passes `gate.Authenticate` with `X-Instance-Gated: 1`, and `apiFetch` latches that marker into a new one-way `authStore.markGateActive()` — so a browser session already holding a valid `dt_session` cookie renders the Log out control on its first authenticated load, with no 401, no typed login, and no reload.
- A stdlib-only Go tool that hand-parses a Go coverage profile and a Vitest json-summary, reads baseline sidecars, and renders the single never-red PR coverage table — plus `--mode=total` for the gate and `--mode=sidecar` for the baseline publish.
- `make coverage-gate` now measures the 80% backend floor by shelling `cmd/coverage-report --mode=total` (one algorithm shared with the PR comment, D-17), `cmd/coverage-report` is out of the coverage denominator it reports on (D-07), and a real-run cutover margin of 10.03pp above 80 is recorded.
- Vitest now writes a `json-summary` profile; `test` and `frontend-test` upload their current coverage every run and publish a per-language Actions-cache baseline on a green push to `main`; and a new report-only `coverage-comment` job restores both baselines by prefix, renders one table with `cmd/coverage-report --mode=comment`, and sticky-upserts a single same-repo PR comment that can never block a merge.
- The old binary's boot migration now no-ops against a newer schema instead of crash-looping — golang-migrate v4.19.1's real behavior was verified live to contradict the phase's founding assumption, and the fix + hermetic RED-then-GREEN proof + `cmd/migrate` CI helper all land together.
- A stdlib-only, unit-tested Go CI guard now turns a branch red when a migration carries a backward-incompatible (N-1-breaking) or unsafe-forward (deploy-hazard) SQL statement, with class-specific README-citing messages and a checkpoint-locked, shape-validated `allow-destructive` escape-hatch annotation.
- `cmd/migration-check` now computes the correct diff base for every GitHub Actions event shape (closing the direct-to-main no-op) and deterministically reds a migration that drops or renames an object the previously-released binary still queries — even through a well-formed `allow-destructive` annotation.
- Three new unconditional GitHub Actions jobs wire Plans 01-03's Go tooling into `full-pipeline.yml` — a `changes` prelude, a `migration-check` guard, and an `n1-boot` job that pulls and boots the previous release against HEAD's schema — and both checks now block the release path via `build-scan.needs:`, closing MGRT-01/MGRT-02.
- A five-section `internal/db/migrations/README.md` documents backward-incompatible vs. unsafe-forward migrations as separate rules, the N-1 rollback invariant, an expand/backfill/contract walkthrough anchored on migrations 000006/000007 already in the tree, a before-you-merge checklist, and the exact `cmd/migration-check` annotation syntax locked at Plan 02's checkpoint — guarded by a doc-presence test and routed to from CLAUDE.md's Definition of Done.

---

## v1.2 Cleanup & Display Fixes (Shipped: 2026-08-24)

**Phases completed:** 2 phases (12-13), 6 plans, 17 tasks

**Key accomplishments:**

- Fixed `CoverArt.tsx`'s stale-placeholder bug — the shared component now resets its failed-load state via `useEffect([src])` when `src` changes on a retained instance, fixing History, Watchlist, and search-result rows at once (Phase 12)
- Added Deezer fan-count-based popularity ranking to artist search (`Client.SearchArtists` sorts descending by `NbFan`, stable on ties) and a MusicBrainz `country`-code disambiguation fallback for when `disambiguation` is blank — absorbing backlog Phase 999.1 (Phase 12)
- History cards for guest-feature and deluxe-change events now show a release date, sourced via a new `internal/musicbrainz.ReleasesForRecording` per-recording lookup with a precision-aware earliest-date rule and a 20-lookup-per-cycle rate cap (Phase 13)
- Guest-feature release cards now show album art, matching the existing new-release card behavior (Phase 13)
- MusicBrainz artists now get real artist art via a new hand-rolled `internal/artistart` matcher (strict close-name + guarded shared-album-title tie-break, fail-closed on ambiguity) wired into both add-time resolution and a cooldown-bounded startup backfill sweep, coordinated by a shared `ActivityGate` so both stay within MusicBrainz's rate budget — absorbing backlog Phase 999.2 (Phase 13)
- Three code-review warnings surfaced during Phase 13 UAT (a date-parsing panic, a stats double-count, and an `ActivityGate` leak on panic) were fixed in place with regression tests rather than deferred

---

## v1.1 Hardening & Scale Readiness (Shipped: 2026-08-17)

**Phases completed:** 5 phases (08-11.1), 22 plans

**Key accomplishments:**

- Stood up a Vitest + React Testing Library test suite covering the watchlist, search, and history React surfaces, mocking the app's own API boundary rather than raw fetch (Phase 8)
- Wired CI coverage gates that block `build-scan`/`release` when backend Go coverage drops below 80% or frontend coverage drops below 70%, proven live on real GitHub Actions runs in both the red and green direction (Phase 9)
- Added a configurable event-retention window (default 90 days) that hides aged-out history from the UI/API while leaving every row and all detection state (dedup keys, deluxe-change baselines, seed-mode signal) fully intact — soft-delete only, zero hard deletes (Phase 10)
- Replaced sequential per-artist polling with a bounded, env-configurable worker pool for both MusicBrainz and Deezer, and closed a real lost-update race on shared deluxe-change baselines with an atomic `FOR UPDATE`-locked compare-and-set (Phase 11)
- Closed out the milestone's own tech debt: replaced a native `<select>` that failed accessibility/contrast on Windows Chromium with a hand-rolled `aria-activedescendant` combobox, added a blocking `prettier --check` CI gate, and reconciled Nyquist validation status across Phases 8-10 (Phase 11.1)

---
