# Project Research Summary

**Project:** drop-tracker
**Domain:** Go-based release tracker with React SPA frontend — v1.1 hardening/scale-readiness milestone
**Researched:** 2026-08-12
**Confidence:** HIGH (all findings verified directly against current codebase; see sources)

## Executive Summary

The v1.1 hardening milestone adds four complementary engineering-practice improvements to an existing, working release-tracking system: a Vitest/React Testing Library frontend test suite, CI coverage gates (70% for both Go and TypeScript), a 90-day event-retention job, and bounded concurrent polling via `errgroup.SetLimit(n)`. All four are additions to existing systems, not greenfield work, and they directly support the portfolio's core goal of demonstrating rigorous CI/CD and DevOps pipeline maturity.

The research identifies three interrelated architectural risks specific to the events table's dual role as a dedup ledger, baseline store, and display history. A naive approach to retention—simply deleting rows older than 90 days—will silently reintroduce three already-fixed bugs: seed-mode reset (Pitfall 1), baseline loss (Pitfall 2), and dedup-key loss (Pitfall 3). These are not generic "be careful with deletes" caveats; they are concrete, code-specific interactions that must be designed around before retention is implemented. The recommended approach is to retain or protect the detection state (dedup keys, baselines) separately from the display history, either via a soft-delete/filter strategy or an upfront schema migration that decouples baselines from individual event rows.

For the worker pool, the research identifies two concurrency-correctness risks: a TOCTOU race on shared release-group baselines (Pitfall 4) and a rate-limiter burst/pacing change that may trip upstream API abuse heuristics (Pitfall 5). Both are solvable with targeted mitigations before the feature ships. The recommended phase order—landing coverage gates before the worker pool—ensures the new changes are caught by a working CI harness.

## Key Findings

### Recommended Stack

The v1.1 stack is **additions-only**; all core technologies from v1.0 (Go 1.23+, chi, sqlc/pgx, robfig/cron, React 19 + Vite 8) remain unchanged.

**Frontend test suite:**
- **Vitest 4.1.10+** with **React Testing Library 16.3.2+** (first line with real React 19 peer support)
- **jsdom 30.0.1** for DOM environment (stronger spec fidelity than happy-dom, worth the tradeoff for a correctness-focused suite)
- Separate `vitest.config.ts` (React Router's Vite plugin is incompatible with Vitest; do not extend existing `vite.config.ts`)
- **Critical:** Use `createRoutesStub` (React Router's own testing utility) for components that need router context; establish one shared render helper in the first test file to avoid reinventing router wrapping per test

**CI coverage gating:**
- Backend: extend existing `make test-integration` with `-covermode=atomic -coverprofile=coverage.out`, add threshold check step (70% aggregate)
- Frontend: new `pnpm vitest run --coverage` job with `coverage.thresholds` in `vitest.config.ts` (Vitest fails process on breach natively)
- **Critical:** Measure both languages' current baseline *before* hardcoding 70% — if either is currently below 70%, either scope the gate to diff/patch coverage initially or defer until a preparatory pass lifts the baseline

**Events retention:**
- Third `robfig/cron` entry inside existing `internal/poller/poller.go` (`Poller` type — reuses same scheduler, lifecycle, graceful shutdown)
- Hard delete via sqlc-generated query, 90-day window (configurable via env var, default 90)
- **Critical design decision** (must resolve before implementation): protect detection state (dedup keys, baselines) via one of:
  - Soft-delete/filter: add `created_at > now() - interval '90 days'` filter to display queries, leave rows intact for seed-mode/baseline/dedup
  - Hard-delete-with-baseline-migration: migrate `track_count` baseline to separate `release_group_baselines` table before deletion is possible
  - Recommend soft-delete as safest first path — satisfies "90-day retention UX" without touching detection correctness

**Bounded concurrent polling:**
- Use `golang.org/x/sync/errgroup.SetLimit(n)` — already an indirect dependency, no new module needed
- Per-source (MusicBrainz pool and Deezer pool sized independently), configurable via env var, default 3-5
- **Critical:** Preserve existing per-artist error handling (return `nil` from worker closures, log errors, continue — never return error and abort the cycle)
- Atomic SQL update for baseline compare-and-set to eliminate Pitfall 4's TOCTOU race (see Architecture notes below)

### Expected Features

**Table stakes (what a hardened Go+React project must have):**
- Frontend unit/component tests covering enumerated surface (WatchlistRow, PreferenceToggles, SearchBox, SearchResultsColumns, EventCard, HistoryFilters)
- CI coverage gates that fail the build (not warn-only) at 70% threshold, single aggregate number per language, wired into existing "Full Pipeline" workflow
- Scheduled events retention via in-process scheduler (robfig/cron), not external infrastructure
- Bounded concurrent polling that preserves per-source independence and respects existing per-source rate limiters

**Differentiators (polish worth the effort, not essential):**
- MSW (Mock Service Worker) for 1-2 higher-value integration-style tests (realistic request/response boundaries)
- PR coverage-diff/report comment actions (visual CI artifact, consistent with existing pipeline polish)
- Structured logging on retention runs (rows deleted, duration) — cheap observability win

**Anti-features (legitimate at larger scale, overkill here):**
- Full E2E test suite (Playwright/Cypress) — out of scope; component tests sufficient for this milestone
- Chasing 100% coverage or per-package/per-file threshold tiers (single aggregate 70% is correct for this project's size)
- `pg_cron` extension (requires infra changes; in-process scheduler matches existing architecture)
- Partition tables or soft-delete-with-purge for retention (no volume justification; straight hard delete or soft filter is correct)

### Architecture Approach

All four features integrate with the existing single-binary Go+React architecture. No new top-level components; all are extensions of existing seams:

**Frontend tests:** Co-locate `*.test.tsx` beside source files (mirroring Go's `_test.go` convention); mock API calls at the `web/app/lib/api.ts` boundary, not at `fetch` level (except for data-shape-sensitive components like History/watchlist, which should mock at `fetch` boundary with real fixture JSON to catch Go/TS serialization drift).

**CI coverage gates:** Wire into existing `full-pipeline.yml` job graph; backend half extends existing `test` job (add coverage flags + threshold check), frontend half is new `test-web` job added to parallel tier and to `build-scan`'s `needs:` array.

**Events retention:** New `robfig/cron` entry in `internal/poller` or a small new `internal/retention` package (prefer new package — keeps poller's responsibility tight). Same overlap guard pattern as MusicBrainz/Deezer cycles (CAS-based skip, not queue), same config-driven env vars.

**Concurrent polling:** Replace sequential `for _, entry := range entries` loop in `RunMusicBrainzCycle`/`RunDeezerCycle` with `errgroup.WithContext` + `SetLimit` fan-out. Preserve all three existing safety mechanisms: per-source rate limiters (unchanged — safe for concurrent callers already), per-source overlap guards (unchanged — guard whole-cycle, not intra-cycle), and per-artist error handling (log and continue, never abort).

**Critical new risk (requires atomic SQL fix):** `GroupTrackCountBaseline`/`SetGroupTrackCountBaseline` is today a check-then-act pattern (read baseline, compute new count, write back). Under concurrent workers, two artists sharing a release-group can race, both read the same baseline, both compute "count increased," and lose one update. Replace with atomic `UPDATE events SET track_count = GREATEST(track_count, $1) WHERE external_id = $2 RETURNING track_count` (SQL-level compare-and-set) — small but essential fix.

### Critical Pitfalls

1. **TTL delete silently resets seed-mode (Pitfall 1):** If retention deletes every `events` row for a long-tracked artist, `isSeedMode` flips back to true and every known release re-inserts as "new." Prevent by excluding the most recent row per `(artist_id, source)` pair, or switching to soft-delete/filter.

2. **TTL delete destroys deluxe-change baseline (Pitfall 2):** The `track_count` baseline lives on individual event rows. Deletion removes the baseline, silently suppressing deluxe alerts for reissues. Prevent by protecting `new_release` rows from hard-delete, or migrating baselines to a separate table.

3. **TTL delete collapses dedup key (Pitfall 3):** Old catalogue items re-fetch and re-insert (dedup key gone), causing duplicate notifications. Prevent by soft-delete/filter approach or explicit dedup-ledger table separate from display history.

4. **Concurrent baseline race (Pitfall 4):** Two artists sharing a `release_group_mbid` can race on baseline read-write. Prevent with atomic SQL `UPDATE ... RETURNING` instead of check-then-act in Go.

5. **Worker-pool burst/pacing changes (Pitfall 5):** N concurrent goroutines calling `rate.Limiter.Wait` can burst-spike requests, triggering upstream abuse heuristics. Prevent by explicitly tuning burst parameter to 1 (or re-verifying current burst config) once concurrency is introduced.

## Implications for Roadmap

Suggested phase structure **for the v1.1 milestone only** (these four features):

### Phase 1: Backend Coverage Gate
**Rationale:** Zero dependencies — foundation for all subsequent work. Makes existing test suite visible/gated before any new features land.
**Delivers:** Coverage measurement infrastructure, 70% threshold enforcement on Go test suite
**Depends on:** Nothing (go test already exists)
**Avoids:** Pitfall 11 (coverage merge/measurement gaps); verify measure before hardcoding 70%

### Phase 2: Frontend Test Suite
**Rationale:** Must exist before frontend coverage gate is meaningful (Pitfall 10). Greenfield tooling setup; land before any other frontend work.
**Delivers:** Vitest + React Testing Library scaffold, test-subject enumeration (8-10 component/route files), shared router render helper
**Uses:** vitest, @testing-library/react, @testing-library/jest-dom, jsdom, @vitejs/plugin-react
**Avoids:** Pitfalls 8 (router context), 9 (over-mocking network boundary)

### Phase 3: Frontend Coverage Gate
**Rationale:** Depends on Phase 2 (tests must exist first). Measure Phase 2's actual baseline before wiring hard 70% gate.
**Delivers:** Coverage measurement job, threshold enforcement, integration into existing "Full Pipeline"
**Integrates with:** Existing full-pipeline.yml CI, Phase 2's test suite
**Avoids:** Pitfall 10 (retrofit gate breaks build on unmeasured baseline)

### Phase 4: Events Retention Design & Implementation
**Rationale:** Independent of phases 2-3 but **must resolve the soft-delete-vs-hard-delete design upfront**. High correctness risk if deferred.
**Delivers:** 90-day retention job running daily, decision/implementation on detection-state protection strategy
**Design checkpoint:** Choose one path (soft-filter, hard-delete-with-baseline-migration, or other) and document explicitly before code lands
**Avoids:** Pitfalls 1-3 (seed-mode reset, baseline loss, dedup-key loss); requires targeted regression tests per each pitfall
**Schema changes:** Only if hard-delete-with-baseline-migration path chosen (adds `release_group_baselines` table)

### Phase 5: Bounded Concurrent Polling
**Rationale:** Land last — highest concurrency-correctness risk. Requires phases 1-3's coverage/test infrastructure to be in place so regressions are caught by CI rather than discovered in production.
**Delivers:** Worker pool implementation (errgroup + SetLimit), atomic baseline update fix, test-double concurrency-safety upgrades
**Uses:** golang.org/x/sync/errgroup (already indirect dependency)
**Integrates:**
- `internal/poller/poller.go` loop bodies (sequential → bounded fan-out)
- `internal/detection/detector.go` baseline read-write (check-then-act → atomic SQL)
- `internal/config/config.go` new `PollWorkerPoolSize` env var (default 4)
- `internal/poller/poller_test.go` test doubles (must become concurrency-safe)

**Avoids:** Pitfalls 4-7 (baseline race, burst pacing, panic isolation, DB pool sizing); requires targeted tests + load verification per each pitfall
**Verification:** Cycle wall-clock time improves; `-race` passes; baseline updates deterministic under concurrent access; DB pool isn't bottleneck

### Phase Ordering Rationale

1. **Backend coverage first** (Phase 1) → independent foundation, blocks nothing, enables visibility into everything that follows
2. **Frontend tests** (Phase 2) → must precede frontend coverage gate (Pitfall 10); greenfield setup, establishes patterns others inherit
3. **Frontend coverage gate** (Phase 3) → depends on Phase 2 existing; measure real baseline before 70% is hard-coded
4. **Retention design** (Phase 4) → independent of 1-3 but highest correctness risk of the milestone; design decision must be written down before implementation; can run in parallel with phases 1-3 if design is resolved separately
5. **Worker pool** (Phase 5) → land last so it's caught by working coverage/test harness; allows concurrent-polling test fixes to be reviewed against a known-working CI; lowest risk of discovering a regression in production

Phases 1-3 have no interdependency and can run in parallel if resources allow. Phase 4's design decision should be resolved before implementation regardless of phase start time. Phase 5 depends only on phases 1-3 existing (coverage/test infrastructure).

### Research Flags

**Phases needing deeper research during planning:**
- **Phase 4 (Retention):** The soft-delete-vs-hard-delete decision has no obvious "right" answer; requires explicit discussion with project stakeholders on the tradeoff (UX/discoverability vs. schema complexity). Research is done; execution requires deliberate design choice, not just assumption.
- **Phase 5 (Worker pool):** Concurrency correctness (Pitfalls 4-7) is the critical risk. Load testing during UAT (not just code review) is essential to verify the worker pool actually improves cycle wall-clock time and doesn't bottleneck on DB pool. Defer until phases 1-3 coverage/test infrastructure is stable.

**Phases with standard patterns (skip research-phase):**
- **Phase 1 (Backend coverage gate):** Well-established pattern; `go test -coverprofile` + threshold check is commodity tooling, no exotic design questions.
- **Phase 2 (Frontend test suite):** Vitest+RTL pattern is industry-standard for React 19 + Vite 8; established components exist; just needs the React Router context pattern (covered in research) to be established once in the first test file.
- **Phase 3 (Frontend coverage gate):** Straightforward once Phase 2 exists; `vitest run --coverage` + `coverage.thresholds` config is native Vitest behavior.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | **HIGH** | All versions, compatibility, and tooling verified directly against current repo (`go.mod`, `web/package.json`, `vite.config.ts`) and against official docs (vitest.dev, pkg.go.dev). Zero inference needed. |
| Features | **HIGH** | Feature scope is explicitly constrained by PROJECT.md's v1.1 Active requirements. Dependencies between features traced against current codebase architecture. Table-stakes vs. differentiators calibrated against portfolio-scale project norms. |
| Architecture | **HIGH** | All four features reverse-engineered against the actual current codebase (`internal/poller/poller.go`, `internal/detection/detector.go`, `internal/db/migrations/000003_events.up.sql`, `.github/workflows/full-pipeline.yml`, `web/vite.config.ts`). Integration points verified line-by-line. |
| Pitfalls | **HIGH** | Top 11 pitfalls identified by direct code inspection + ecosystem knowledge. Pitfalls 1-3 (retention) are code-specific; Pitfalls 4-7 (concurrency) documented in concurrent design literature. Pitfalls 8-11 (testing/coverage) are established retrofit gotchas corroborated across multiple 2025-2026 independent sources. |

**Overall confidence:** **HIGH**

All findings are grounded in either direct repo inspection (highest confidence) or corroborated across multiple independent 2025-2026 sources matching the project's specific context (stack, architecture, domain).

### Gaps to Address

- **Coverage baseline measurement:** Phase 3 must include an explicit "measure current Go coverage" step before 70% is hard-coded into CI. If actual baseline is below 70%, the gate must either (a) be scoped to diff/patch coverage initially, or (b) include a documented ratchet plan.
- **Retention soft-delete implementation:** If soft-delete/filter path is chosen, the ListEvents query must be explicitly tested to confirm the `created_at` filter is applied consistently across all display paths (History feed, API serialization, etc.).
- **Worker pool load-testing:** The claim "worker pool reduces cycle wall-clock time" must be verified via measurement during Phase 5's UAT, not assumed. Include before/after cycle duration logs and verify database pool isn't the new bottleneck.
- **Release-group baseline race test:** Pitfall 4's test (two goroutines racing on a shared release-group baseline) must be written and verified with `go test -race` during Phase 5. The `-race` flag alone won't catch this logical race; the test needs to assert final state correctness.

## Sources

### Primary (HIGH confidence — direct code inspection)

- Drop-tracker codebase: `cmd/server/main.go`, `internal/poller/poller.go`, `internal/detection/detector.go`, `internal/detection/musicbrainz.go`, `internal/events/service.go`, `internal/db/sqlc/querier.go`, `internal/db/migrations/000003_events.up.sql`, `internal/db/pool.go`, `internal/config/config.go`, `web/vite.config.ts`, `web/package.json`, `web/app/routes/*.tsx`, `web/app/components/**/*.tsx`, `web/app/lib/api.ts`, `.github/workflows/full-pipeline.yml`, `.planning/PROJECT.md`, `.planning/codebase/TESTING.md`

### Secondary (MEDIUM confidence — ecosystem consensus, multiple sources)

- Vitest, React Testing Library, React Router 7 testing: official docs + 2025-2026 community guides
- Go coverage tooling: `go.dev/doc/build-cover`, community patterns
- Concurrency patterns: `pkg.go.dev/golang.org/x/sync/errgroup`, `pkg.go.dev/golang.org/x/time/rate`
- Frontend test retrofit: community articles on Vitest/RTL adoption, testing-library GitHub issues
- Events/TTL retention: webhook/event-processing literature corroborating TTL-vs-dedup tension as recognized failure class

---

*Research completed: 2026-08-12*
*Ready for roadmap: yes*
