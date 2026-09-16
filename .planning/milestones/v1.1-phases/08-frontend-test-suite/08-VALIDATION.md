---
phase: 08
slug: frontend-test-suite
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-12
reconciled: 2026-08-17
reconciled_note: >
  This file was written during planning (pre-execution) with placeholder task IDs
  ("08-xx-xx") and all rows pending. Reconciled post-execution against the real
  PLAN/SUMMARY files and the already-complete 08-VERIFICATION.md (2026-08-13,
  status: passed, 8/8 must-haves verified, zero gaps). No new tests were generated
  by this reconciliation -- coverage was already complete; this pass only replaces
  the placeholder map with the real one and flips status/nyquist_compliant to match
  reality. Performed as part of Phase 11.1's D-09 cleanup item.
---

# Phase 08 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 4.1.10 (new this phase — no prior frontend test framework existed) |
| **Config file** | `web/vitest.config.ts` (new, separate from `web/vite.config.ts` — React Router's Vite plugin is incompatible with Vitest, per D-01) |
| **Quick run command** | `pnpm --dir web exec vitest run <changed test file>` |
| **Full suite command** | `pnpm --dir web test` (`vitest run`) |
| **Estimated runtime** | ~10-20 seconds (small suite, jsdom, no DB) |

---

## Sampling Rate

- **After every task commit:** Run `pnpm --dir web exec vitest run <changed test file>`
- **After every plan wave:** Run `pnpm --dir web test` (full suite)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 20 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-D1 | 01 | 1 | TEST-01 | — | `pnpm --dir web test` runs the suite in jsdom, exits non-zero on regression, no `passWithNoTests` escape hatch | infra | `pnpm --dir web test` | ✅ | ✅ green |
| 08-01-D2 | 01 | 1 | TEST-01 | — | Suite is idempotent and order-independent | infra | `pnpm --dir web test` (x2) + `--sequence.shuffle` | ✅ | ✅ green |
| 08-01-D3 | 01 | 1 | TEST-01 | — | HistoryFilters: artist list populates, selection reports upward, clear reports `artistId: null` never `0` | component | `pnpm --dir web exec vitest run app/components/history/HistoryFilters.test.tsx` | ✅ | ✅ green |
| 08-01-D4 | 01 | 1 | TEST-02 | — | Mocks `~/lib/api`, not raw fetch | grep check | `grep -c 'vi.mock("~/lib/api")'` | ✅ | ✅ green |
| 08-01-D5 | 01 | 1 | TEST-01 | — | `frontend-test` job added to Full Pipeline's parallel tier, SHA-pinned, `build-scan`'s `needs:` untouched | infra | yaml parse + regex check on `.github/workflows/full-pipeline.yml` | ✅ | ✅ green |
| 08-02-D1 | 02 | 2 | TEST-01 | — | Watchlist row's remove control triggers `removeWatchlist` with the entry's exact id (route-level via shared stub) | component/route | `pnpm --dir web exec vitest run app/routes/watchlist.test.tsx` | ✅ | ✅ green |
| 08-02-D2 | 02 | 2 | TEST-01 | — | Preference toggle rolls back optimistic state on PATCH failure (ordered call-pair proof) | component | `pnpm --dir web exec vitest run app/components/watchlist/PreferenceToggles.test.tsx` | ✅ | ✅ green |
| 08-02-D3 | 02 | 2 | TEST-01 | — | One shared `createRoutesStub`-based `renderRoute` helper is the sole router-context seam | grep check | `grep -c 'createRoutesStub' web/app/lib/test/routeStub.tsx`; no `MemoryRouter` anywhere | ✅ | ✅ green |
| 08-02-D4 | 02 | 2 | TEST-02 | — | Both new test files mock `~/lib/api`, pass with no server running | unit | `pnpm --dir web exec vitest run app/routes/watchlist.test.tsx app/components/watchlist/PreferenceToggles.test.tsx` | ✅ | ✅ green |
| 08-02-D5 | 02 | 2 | TEST-01 | — | `pnpm --dir web typecheck` exits 0 | infra | `pnpm --dir web typecheck` | ✅ | ✅ green |
| 08-03-D1 | 03 | 2 | TEST-01 | — | Search: a keystroke burst collapses into exactly one debounced `searchArtists` call, response forwarded via `onResults` | component | `pnpm --dir web exec vitest run app/components/watchlist/SearchBox.test.tsx` | ✅ | ✅ green |
| 08-03-D2 | 03 | 2 | TEST-01 | T-08-01 | Superseded search's `AbortSignal` reaches `searchArtists` and reports aborted (folded bug, RED-then-GREEN: `43e1b40`→`14003dd`) | component | same file, 2 tests | ✅ | ✅ green |
| 08-03-D3 | 03 | 2 | TEST-02 | — | Bare `vi.mock("~/lib/api")`, no raw fetch, no server needed | grep check | `grep -c 'vi.mock("~/lib/api")'` == 1 | ✅ | ✅ green |
| 08-04-D1 | 04 | 2 | TEST-01 | T-08-01 | EventCard falls back to a neutral badge for an unrecognized `event_type` instead of crashing the route (folded bug, RED-then-GREEN: `1bf8cec`→`4f51937`) | component | `pnpm --dir web exec vitest run app/components/history/EventCard.test.tsx` | ✅ | ✅ green |
| 08-05-D1 | 05 | 3 | TEST-01 | T-08-01 | `guestFeatureHref` percent-encodes `external_id` on both MusicBrainz and Deezer branches (folded bug, RED-then-GREEN: `df1344f`→`daee355`) | component | same file, 3 added cases (`abc def/g#h` id) | ✅ | ✅ green |
| 08-01-D6 | 01 | 1 | TEST-01 | — | `frontend-test` CI job actually runs green in a real GitHub Actions run | infra | N/A locally — requires a live GH Actions run | ✅ | ✅ resolved (see Manual-Only) |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Reconciled 2026-08-17 from 08-VERIFICATION.md (2026-08-13, status: passed, 8/8 truths, 0 gaps) and the five 08-0N-SUMMARY.md `coverage:` blocks. All rows verified against real commits and the current codebase — none re-derived from assumption.*

---

## Wave 0 Requirements

- [x] `web/vitest.config.ts` — framework config (commit `bb435ae`)
- [x] `web/vitest.setup.ts` — jest-dom matcher registration + manual `afterEach(cleanup)` (commit `bb435ae`)
- [x] `web/app/lib/test/routeStub.tsx` — shared `createRoutesStub` helper, D-03 (commit `0f6309b`)
- [x] Framework install: `vitest@4.1.10 jsdom@30.0.1 @testing-library/react@16.3.2 @testing-library/jest-dom@7.0.1 @testing-library/user-event@14.6.4` (commit `bb435ae`)
- [x] `web/package.json` — `test`/`test:watch` scripts (commit `bb435ae`)

---

## Manual-Only Verifications

- **08-01-D6** — `frontend-test` CI job actually running green in a real GitHub Actions run. Not observable from a local worktree session at execution time (flagged `human_judgment: true` in 08-01-SUMMARY.md). **Resolved 2026-08-13**: confirmed via three real GitHub Actions runs (31724487315, 31724744670, 31724954534) on the scratch branch `test/coverage-gate-ci-check`, recorded in STATE.md's `[Phase 09 UAT, closed 2026-08-13]` entry and cross-referenced in `.planning/v1.1-MILESTONE-AUDIT.md`'s amendment note. Branch has since been deleted; the run IDs remain independently checkable against GitHub's own history.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags (suite runs via `vitest run`, never bare `vitest` watch mode)
- [x] Feedback latency < 20s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** reconciled 2026-08-17 against 08-VERIFICATION.md (passed, 0 gaps) — no outstanding items.

---

## Validation Audit 2026-08-17

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 (none needed — 08-VERIFICATION.md already confirmed full coverage) |
| Escalated | 0 |
| Placeholder rows replaced with real task IDs/commits | 16 |
