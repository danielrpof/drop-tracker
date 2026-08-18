# Phase 8: Frontend Test Suite - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

The React frontend's watchlist, search, and history surfaces get a real component test suite (Vitest + React Testing Library, jsdom) that mocks the app's API boundary (`web/app/lib/api.ts`), not raw `fetch`. One command runs the suite locally and in CI, failing non-zero on a component regression. This phase delivers TEST-01 and TEST-02 — it does not touch coverage *thresholds* (that's Phase 9), the retention window, or concurrent polling.

</domain>

<decisions>
## Implementation Decisions

### Tooling
- **D-01:** Vitest + React Testing Library + jsdom, per ROADMAP.md. A separate `vitest.config.ts` is required — React Router's Vite plugin is incompatible with reusing `web/vite.config.ts` directly (already noted in ROADMAP.md's Phase 8 entry).
- **D-02:** Test files co-locate beside source (`*.test.tsx`), mirroring the Go `_test.go` convention already established in this repo (`internal/*/*_test.go`).
- **D-03:** Components needing router context render through one shared helper built on React Router's `createRoutesStub`, established once and reused — per success criterion 4, not re-litigated here.

### CI wiring scope
- **D-04:** This phase adds a new job to `.github/workflows/full-pipeline.yml` that runs the Vitest suite on every push — report-only, no coverage threshold. Phase 9 then only needs to add the coverage-gate step to this same job (splitting job-creation from job-gating, matching Phase 9's roadmap note that frontend is "a new job added to the parallel tier").

### Coverage depth ambition
- **D-05:** Floor only, per success criteria — write exactly the tests success criterion 2 names (one per surface: watchlist row, preference toggle, search, history/event-filter) proving the specific behaviors listed, plus whatever's needed to prove the 3 folded bug fixes below. Do not proactively expand into every component branch (e.g. all three `EventCard` event-type bodies) — that's Phase 9's territory if a measured coverage number calls for it.

### API-mock strategy
- **D-06:** `vi.mock('~/lib/api')` per test file, with `vi.mocked(fn).mockResolvedValue(...)` / `.mockRejectedValue(...)` set per test. No new shared mock-api helper module — matches this codebase's existing preference for narrow, colocated seams over shared indirection (`internal/watchlist.Store`, `httpserver.Pinger`).

### Bug-fix scope for folded todos
- **D-07:** Each folded bug (see Folded Todos below) gets its own RED-then-GREEN commit pair — a test proving the bug fails against current code, then a minimal fix commit that makes it pass — mirroring this codebase's established TDD convention (e.g. Phase 2's "RED-phase tests... committed together... GREEN implementation in its own separate feat commit"). Not bundled into the surface's general test-writing commit.

### Claude's Discretion
None — all discussed areas resolved to a specific choice.

### Folded Todos
- **EventCard crashes History route on unrecognized event_type** (`.planning/todos/pending/2026-08-11-eventcard-crashes-history-route-on-unrecognized-event-type.md`, major) — `EVENT_BADGE[event.event_type]` in `web/app/components/history/EventCard.tsx:39` has no fallback for an event_type outside the known union; indexing returns `undefined` and the next line throws, crashing the whole History route (caught only by the top-level ErrorBoundary). Folded because writing a real test for EventCard's rendering behavior is the natural place to prove the fallback badge works — fix via RED-then-GREEN per D-07.
- **SearchBox AbortController never cancels the underlying fetch** (`.planning/todos/pending/2026-08-11-searchbox-abortcontroller-never-cancels-the-underlying-fetch.md`, minor) — `apiFetch` in `web/app/lib/api.ts` never forwards an `AbortSignal` to `fetch()`, so `SearchBox`'s doc comment claim ("a fresh AbortController is created per debounced search and the prior one is aborted") is untrue at the network level — every keystroke's search still runs to completion even after being superseded. Folded because success criterion 4's search test needs to assert "a stale response never overwrites a newer query's results," which requires this fix to be true, not just the `if (controller.signal.aborted) return` guards currently in place. Fix via RED-then-GREEN per D-07: thread `signal` through `apiFetch`/`searchArtists`/`SearchBox.runSearch`.
- **guestFeatureHref missing encodeURIComponent on external_id** (`.planning/todos/pending/2026-08-11-guestfeaturehref-missing-encodeuricomponent-on-external-id.md`, cosmetic) — `web/app/components/history/EventCard.tsx:108-116` interpolates `event.external_id` directly into a URL path with no `encodeURIComponent`. Lowest relevance of the three, but touches the same file the EventCard test already needs to open — folded to close it in the same pass rather than leave a known gap in a file this phase is already editing. Fix via RED-then-GREEN per D-07.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` (§ Frontend Testing, lines 73-76) — TEST-01, TEST-02 exact requirement text
- `.planning/ROADMAP.md` (§ Phase 8: Frontend Test Suite) — goal, success criteria, and the `vitest.config.ts`-must-be-separate note

### Folded bug reports (full problem/solution detail)
- `.planning/todos/pending/2026-08-11-eventcard-crashes-history-route-on-unrecognized-event-type.md`
- `.planning/todos/pending/2026-08-11-searchbox-abortcontroller-never-cancels-the-underlying-fetch.md`
- `.planning/todos/pending/2026-08-11-guestfeaturehref-missing-encodeuricomponent-on-external-id.md`

### Codebase conventions
- `.planning/codebase/TESTING.md` — Go test conventions this phase's frontend conventions should echo where sensible (naming, co-location, table-driven vs. individual test functions)
- `.planning/codebase/STRUCTURE.md` (§ React Component, § Test Utilities) — where new test files and any shared router-stub helper belong

No other external specs — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/app/lib/api.ts` — the single fetch boundary (`apiFetch`) all five endpoint wrappers funnel through; this is the exact module success criterion 3 requires mocking, not raw `fetch`. Wire types (`WatchlistEntry`, `EventItem`, `SearchResponse`, `SearchArtist`) are already defined here and can be reused directly as test fixture shapes.
- `web/app/components/watchlist/WatchlistRow.tsx`, `PreferenceToggles.tsx`, `SearchBox.tsx` — the watchlist/search surfaces named in success criterion 2.
- `web/app/components/history/EventCard.tsx` — the history surface; already dispatches on `event_type` to three distinct body renderers (`NewReleaseBody`, `GuestFeatureBody`, `DeluxeChangeBody`), useful as the natural seam for the EventCard test's assertions.

### Established Patterns
- `PreferenceToggles.tsx` already implements optimistic update + rollback-on-failure (pre-toggle state saved in `previous`, restored via `onEntryChange` in the `catch` block, `toast.error` on failure) — this is exactly the behavior success criterion 2 names ("a preference toggle rolls back its optimistic state when the call fails"); the test just needs to force `updateWatchlistPreferences` to reject.
- `SearchBox.tsx` already debounces at 300ms (`SEARCH_DEBOUNCE_MS`) and creates a fresh `AbortController` per search — the *intent* is correct, only the `fetch()` cancellation wiring (folded todo above) is missing.
- Go test conventions (`internal/*/*_test.go`): `Test{FunctionName}_{Behavior}` naming, `t.Helper()`-marked helpers, narrow-interface stubs with func-fields. No table-driven tests in this codebase currently — individual test functions with distinct names are preferred. The frontend suite should pick its own idiomatic RTL/Vitest naming rather than force Go conventions, but the co-location and "one behavior per test" philosophy carries over (D-02).

### Integration Points
- `.github/workflows/full-pipeline.yml` — where the new ungated frontend-test CI job (D-04) gets added, in the same parallel tier Phase 9's roadmap note describes.
- `web/package.json` — currently has zero test-related dependencies or scripts (`devDependencies` list: `@react-router/dev`, `@tailwindcss/vite`, `@types/*`, `prettier`, `tailwindcss`, `typescript`, `vite`). Vitest, `@testing-library/react`, `@testing-library/jest-dom`, `jsdom`, and any coverage provider all need to be added fresh.

</code_context>

<specifics>
## Specific Ideas

No specific UI/visual requirements beyond what's captured in decisions above — this is a testing phase, not a design phase.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

### Reviewed Todos (not folded)
None — discussion stayed within phase scope.

</deferred>

---

*Phase: 08-Frontend Test Suite*
*Context gathered: 2026-08-12*
