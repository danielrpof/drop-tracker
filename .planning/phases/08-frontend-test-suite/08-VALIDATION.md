---
phase: 08
slug: frontend-test-suite
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-12
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
| 08-W0-01 | 01 | 0 | TEST-01 | — | N/A (tooling install) | infra | `pnpm --dir web exec vitest --version` | ❌ W0 | ⬜ pending |
| 08-xx-xx | TBD | TBD | TEST-01 | — | Preference toggle rolls back optimistic state on PATCH failure | component | `pnpm --dir web exec vitest run app/components/watchlist/PreferenceToggles.test.tsx` | ❌ W0 | ⬜ pending |
| 08-xx-xx | TBD | TBD | TEST-01 | — | Search issues a debounced, cancellable request through `searchArtists` | component | `pnpm --dir web exec vitest run app/components/watchlist/SearchBox.test.tsx` | ❌ W0 | ⬜ pending |
| 08-xx-xx | TBD | TBD | TEST-01 | — | History/event-filter populates from watchlist data and reports filter changes upward | component | `pnpm --dir web exec vitest run app/components/history/HistoryFilters.test.tsx` | ❌ W0 | ⬜ pending |
| 08-xx-xx | TBD | TBD | TEST-01 | — | Watchlist row's remove control triggers `removeWatchlist` (route-level, via shared router stub) | component/route | `pnpm --dir web exec vitest run app/routes/watchlist.test.tsx` | ❌ W0 | ⬜ pending |
| 08-xx-xx | TBD | TBD | TEST-01 | T-08-01 | EventCard falls back to a default badge for an unrecognized `event_type` (folded bug, RED-then-GREEN) | component | `pnpm --dir web exec vitest run app/components/history/EventCard.test.tsx` | ❌ W0 | ⬜ pending |
| 08-xx-xx | TBD | TBD | TEST-01 | T-08-01 | `guestFeatureHref` encodes `external_id` in the rendered anchor (folded bug, RED-then-GREEN) | component | `pnpm --dir web exec vitest run app/components/history/EventCard.test.tsx` | ❌ W0 | ⬜ pending |
| 08-xx-xx | TBD | TBD | TEST-01 | T-08-01 | SearchBox's superseded/aborted fetch never overwrites newer results (folded bug, RED-then-GREEN) | component | `pnpm --dir web exec vitest run app/components/watchlist/SearchBox.test.tsx` | ❌ W0 | ⬜ pending |
| 08-xx-xx | TBD | TBD | TEST-02 | — | Every test above mocks `~/lib/api`, never raw `fetch` (cross-cutting, enforced by `vi.mock('~/lib/api')` at the top of each file, D-06) | grep check | `grep -rL "vi.mock(\"~/lib/api\")" web/app/**/*.test.tsx` returns nothing for files importing a function (not just a type) from `~/lib/api` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs are placeholders — the planner fills in real `{phase}-{plan}-{task}` IDs and wave numbers when PLAN.md files are written.*

---

## Wave 0 Requirements

- [ ] `web/vitest.config.ts` — framework config, does not exist yet
- [ ] `web/vitest.setup.ts` — jest-dom matcher registration, does not exist yet
- [ ] `web/app/lib/test/routeStub.tsx` — shared `createRoutesStub` helper (D-03), does not exist yet
- [ ] Framework install (run from `web/`): `pnpm add -D vitest@4.1.10 jsdom@30.0.1 @testing-library/react@16.3.2 @testing-library/jest-dom@7.0.1 @testing-library/user-event@14.6.4`
- [ ] `web/package.json` — add a `"test"` script (`vitest run`)

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags (suite runs via `vitest run`, never bare `vitest` watch mode)
- [ ] Feedback latency < 20s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
