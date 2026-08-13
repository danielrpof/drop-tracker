---
phase: 08-frontend-test-suite
verified: 2026-08-13T02:44:39Z
status: passed
score: 8/8 must-haves verified
behavior_unverified: 0
overrides_applied: 0
deferred:
  - truth: "The frontend-test CI job blocks build-scan/release when the frontend suite fails"
    addressed_in: "Phase 9"
    evidence: "ROADMAP.md Phase 9 success criterion 3: 'A coverage failure on either side blocks the downstream build/scan/release jobs — no image is built, scanned, or pushed to ghcr.io when a gate trips.' Phase 9 Notes: 'frontend is a new job added to the parallel tier and to build-scan's needs:'. Phase 8's own 08-01-PLAN.md Task 3 explicitly scopes this out (D-04, Open Question 1/A3: 'Do NOT add frontend-test to build-scan's needs array in this phase... Phase 9's success criterion 3 explicitly owns making a frontend failure block those jobs, and it edits this same job.')"
---

# Phase 8: Frontend Test Suite Verification Report

**Phase Goal:** The React frontend's watchlist, search, and history surfaces are covered by a real component test suite, so a regression in the UI is caught by a test run instead of by hand-clicking the app.
**Verified:** 2026-08-13T02:44:39Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP Success Criteria + PLAN must-haves) | Status | Evidence |
|---|---|---|---|
| 1 | A single command (`pnpm --dir web test`) runs the frontend suite (Vitest + RTL, jsdom) locally and in CI, and exits non-zero on a component regression (ROADMAP SC1) | VERIFIED | Ran `pnpm test` from `web/` directly: `Test Files 5 passed (5)`, `Tests 16 passed (16)`, exit code 0. `web/vitest.config.ts` sets `environment: "jsdom"`; no `passWithNoTests` (grep count 0). CI: `.github/workflows/full-pipeline.yml` line 83 declares `frontend-test:` job that runs `pnpm test` on every push/PR. Non-zero-on-regression proven three separate times in the SUMMARYs via RED commits (`43e1b40`, `1bf8cec`, `df1344f`) that were observed failing before their GREEN fix commits — all four commits verified present in `git log`. |
| 2 | Watchlist row's remove control triggers `removeWatchlist` with the entry's id (ROADMAP SC2, named example) | VERIFIED | `web/app/routes/watchlist.test.tsx`: renders route via `renderRoute`, clicks "Remove Drake from watchlist", asserts `expect(mockRemoveWatchlist).toHaveBeenCalledWith(42)`. Test passes in isolation and in full suite. |
| 3 | A preference toggle rolls back its optimistic state when the PATCH call fails (ROADMAP SC2, named example) | VERIFIED | `web/app/components/watchlist/PreferenceToggles.test.tsx` test 2: rejects `updateWatchlistPreferences`, asserts optimistic `onEntryChange(1, {release_types: ["album","single"]})` fires immediately, then `waitFor` asserts `toHaveBeenLastCalledWith(1, {release_types: ["album"]})` — the ordered pair proves real rollback, not just final state. |
| 4 | Search surface has a passing test asserting user-visible behavior (ROADMAP SC2) | VERIFIED | `web/app/components/watchlist/SearchBox.test.tsx`: debounce-collapse test asserts `toHaveBeenCalledTimes(1)` and forwarded response via `onResults`. Two additional tests prove AbortSignal threading and supersession-abort (closed the folded AbortController bug). |
| 5 | History/event-filter surface has a passing test asserting user-visible behavior (ROADMAP SC2) | VERIFIED | `web/app/components/history/HistoryFilters.test.tsx`: asserts artist list populates from `listWatchlist`, selecting an artist reports the value upward, and clearing reports `artistId: null` (never `0`). |
| 6 | Tests mock `web/app/lib/api.ts`, not raw fetch; no real network request; suite passes with no server running (ROADMAP SC3, TEST-02) | VERIFIED | Every test file that imports a function from `~/lib/api` calls a bare `vi.mock("~/lib/api")` (4 files, confirmed via grep). Zero matches for `(global\|globalThis)\.fetch` or `importActual` across `web/app/**/*.test.tsx`. `EventCard.test.tsx` imports only the `EventItem` type (no function), correctly has no mock. Full suite passes offline (13.5s run, no server started). |
| 7 | Components needing router context render through one shared `createRoutesStub` helper, established once and reused (ROADMAP SC4) | VERIFIED | `web/app/lib/test/routeStub.tsx` exports exactly one function `renderRoute`, built on `createRoutesStub` from `react-router`. Only consumer needing router context (`watchlist.test.tsx`) imports and uses it; no `MemoryRouter` or ad hoc router wrapping found anywhere in `web/app`. |
| 8 | The suite never reports a vacuous green / is order-independent / idempotent (PLAN 08-01 must-have) | VERIFIED | `grep -c 'passWithNoTests' web/vitest.config.ts` = 0. Ran `pnpm exec vitest run --sequence.shuffle`: 5 files / 16 tests passed with a random seed. Ran `pnpm test` twice in a row: both green. `mockReset: true` set in config (prevents cross-test state leakage). |

**Score:** 8/8 truths verified (0 present-but-behavior-unverified)

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | `frontend-test` CI job is not added to `build-scan`'s `needs:` array, so a failing frontend suite does not currently block the Docker build/scan/release pipeline (flagged CRITICAL in 08-REVIEW.md, CR-01) | Phase 9 | ROADMAP.md Phase 9 goal: "a drop in coverage on either language blocks the build before anything is packaged or published"; SC3: "A coverage failure on either side blocks the downstream build/scan/release jobs"; Notes: "frontend is a new job added to the parallel tier and to `build-scan`'s `needs:`". This exact gap is also explicitly self-documented as deliberate in 08-01-PLAN.md's Task 3 (`Do NOT add frontend-test to build-scan's needs array in this phase`) and its Assumptions section (A3/Open Question 1), citing Phase 9 SC3 by name as the owner. **Assessment:** ROADMAP SC1 for Phase 8 only requires the command itself to exit non-zero locally and in CI — it does not require blocking the release pipeline. That specific requirement (blocking build/scan/release on failure) is Phase 9's SC3, not Phase 8's SC1. Confirmed as an intentional, tracked scope boundary rather than an unaddressed gap of this phase. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `web/vitest.config.ts` | Standalone Vitest config: `~` alias, jsdom, setupFiles, mockReset, no passWithNoTests | VERIFIED | Confirmed all properties present; never imports `web/vite.config.ts` or React Router dev plugin. |
| `web/vitest.setup.ts` | jest-dom matcher registration + cleanup wiring | VERIFIED | Imports `@testing-library/jest-dom/vitest`; manually wires `afterEach(cleanup)` (needed because `test.globals` is not enabled). |
| `web/package.json` | `test`/`test:watch` scripts + 5 devDependencies | VERIFIED | `"test": "vitest run"` confirmed; `vitest`, `jsdom`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event` all present in devDependencies. |
| `web/app/components/history/HistoryFilters.test.tsx` | History/event-filter surface tests | VERIFIED | 3 tests, all passing. |
| `web/app/routes/watchlist.test.tsx` | Route-level remove-triggers-API test | VERIFIED | 1 test, passing, asserts specific id. |
| `web/app/components/watchlist/PreferenceToggles.test.tsx` | Optimistic update + rollback tests | VERIFIED | 2 tests, passing. |
| `web/app/components/watchlist/SearchBox.test.tsx` | Debounce + AbortSignal tests | VERIFIED | 3 tests, passing. |
| `web/app/components/history/EventCard.test.tsx` | Badge rendering + fallback + href-encoding tests | VERIFIED | 7 tests, passing (3 known types + 1 fallback + 2 encoding + 1 UUID-unchanged guard). |
| `web/app/lib/test/routeStub.tsx` | Shared `renderRoute` router-stub helper | VERIFIED | Exports exactly one function, built on `createRoutesStub`. |
| `.github/workflows/full-pipeline.yml` -> `frontend-test` job | New CI job, parallel tier, SHA-pinned actions | VERIFIED | Job present at line 83; checkout/pnpm-setup/node-setup/install/test step order matches plan; both new action SHAs are 40-char with trailing `# vX.Y.Z` comments; `cache-dependency-path: web/pnpm-lock.yaml` present. |
| `web/app/lib/api.ts` -> `searchArtists(query, signal?)` | Widened to accept and forward AbortSignal | VERIFIED | `signal?: AbortSignal` parameter present, forwarded into `apiFetch`'s init object. |
| `web/app/components/history/EventCard.tsx` -> fallback badge + `encodeURIComponent` | Unknown-type fallback + href escaping | VERIFIED | `UNKNOWN_EVENT_BADGE` constant present as default on lookup; `encodeURIComponent` appears exactly 2 times (one per source branch). |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `web/vitest.config.ts` | `web/app/` | `~` alias | WIRED | `resolve.alias: { "~": path.resolve(__dirname, "./app") }` |
| `web/vitest.config.ts` | `web/vitest.setup.ts` | `setupFiles` | WIRED | `setupFiles: ["./vitest.setup.ts"]` |
| Test files | `web/app/lib/api.ts` | `vi.mock("~/lib/api")` | WIRED | Bare mock, no factory, in all 4 files that import a function from api.ts |
| `.github/workflows/full-pipeline.yml` | `web/package.json` | `pnpm test` | WIRED | `frontend-test` job's final step runs `pnpm test` from `web/` working directory |
| `web/app/routes/watchlist.test.tsx` | `web/app/lib/test/routeStub.tsx` | `renderRoute` import | WIRED | Only consumer of the shared router-stub helper, used correctly |
| `web/app/components/watchlist/SearchBox.tsx` | `web/app/lib/api.ts` | `searchArtists(query, controller.signal)` | WIRED | Confirmed at `SearchBox.tsx:50` |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Full suite runs green from clean state | `pnpm test` (from `web/`) | `Test Files 5 passed (5)`, `Tests 16 passed (16)`, exit 0 | PASS |
| Suite exits non-zero on discovery-related regression | Config check: `passWithNoTests` absent | grep count 0 | PASS |
| Suite is order-independent | `pnpm exec vitest run --sequence.shuffle` | 5 files / 16 tests passed | PASS |
| `pnpm typecheck` exits 0 | `react-router typegen && tsc` | exit 0, no errors | PASS |
| RED commits genuinely failed before their GREEN fix | `git log --oneline` cross-reference | `43e1b40`, `1bf8cec`, `df1344f` (RED) and `14003dd`, `4f51937`, `daee355` (GREEN) all present as distinct commits in history | PASS |
| No test reaches the real network | grep for `global.fetch`/`globalThis.fetch`/`importActual` across `web/app/**/*.test.tsx` | 0 matches | PASS |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` convention exists in this project; phase does not declare probes.

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|---|---|---|---|---|
| TEST-01 | 08-01, 08-02, 08-03, 08-04, 08-05 | Vitest + RTL suite covering watchlist list/row, preference-toggle, search, history/event-filter surfaces | SATISFIED | All 4 named surfaces have passing tests (see Observable Truths 2-5); REQUIREMENTS.md already marks TEST-01 `[x]` Complete, confirmed by codebase evidence above. |
| TEST-02 | 08-01, 08-02, 08-03 | Tests mock `web/app/lib/api.ts`, not raw fetch | SATISFIED | Bare `vi.mock("~/lib/api")` in all files with an API import; zero raw-fetch patches found. |

No orphaned requirements — REQUIREMENTS.md's traceability table maps only TEST-01 and TEST-02 to Phase 8, both accounted for.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| `web/app/components/watchlist/SearchBox.test.tsx` | 56-61 | Stale comment claims "Both fail against current source" — no longer true since the GREEN fix (`14003dd`) landed in the same file/plan | Info | Already documented as WR-01 in the committed 08-REVIEW.md. Does not affect test correctness or any success criterion; purely a doc-comment drift issue. |
| `web/app/routes/watchlist.test.tsx` | whole file | Single happy-path test for a destructive action (no failure-path / no post-remove UI-state assertion) | Info | Already documented as WR-02 in 08-REVIEW.md. ROADMAP SC2 only requires "at least one passing test asserting user-visible behavior" for the watchlist surface — satisfied. Coverage depth beyond the floor is a quality improvement, not a phase-goal blocker. |
| `web/app/components/history/EventCard.tsx` | 93,95,97 | Single-quote string literals violate `.prettierrc`'s `singleQuote: false` | Info | Already documented as IN-01 in 08-REVIEW.md. No CI format-check step exists yet to enforce this; cosmetic only. |
| `web/app/components/history/HistoryFilters.test.tsx` | whole file | No test exercises the `eventType` filter field, only the `artistId` axis | Info | Already documented as IN-02 in 08-REVIEW.md. ROADMAP SC2's floor ("at least one passing test asserting user-visible behavior") is satisfied for this surface via the artist-select tests. |

No TBD/FIXME/XXX debt markers found in any file modified by this phase (checked all 11 created/modified files under `web/`).

### Human Verification Required

None required for the phase's success criteria. All four observable truths (SC1-SC4) plus the deferred CI-gating question are verifiable from the codebase and a local test run.

### Gaps Summary

No gaps found. All 4 ROADMAP success criteria and all PLAN-level must-haves across the five plans (08-01 through 08-05) are verified against the actual codebase: the harness exists and runs green/idempotent/order-independent; all four named UI surfaces (watchlist row, preference toggles, search, history/event-filter) have passing tests asserting the specific user-visible behaviors named in the roadmap; every test that touches the API boundary mocks `~/lib/api` bare with zero raw-fetch/network escape hatches; and the one shared `renderRoute`/`createRoutesStub` helper is the sole router-context seam in use.

One item from the code review (CR-01: `frontend-test` not wired into `build-scan`'s `needs:`) was evaluated against the phase's success criteria and found to be an intentional, explicitly-tracked scope boundary — Phase 8's own plan documents (08-01-PLAN.md D-04/A3) and the ROADMAP's own Phase 9 definition (goal, SC3, and Notes) all independently confirm that wiring the frontend-test job into the release-blocking `needs:` graph is Phase 9's responsibility, not Phase 8's. ROADMAP SC1 for this phase asks only that the command "exits non-zero when a component regresses," which is true both locally and in the `frontend-test` CI job as it stands today — it does not ask that job to gate the release pipeline. This item is recorded as `deferred`, not as a gap.

---

_Verified: 2026-08-13T02:44:39Z_
_Verifier: Claude (gsd-verifier)_
