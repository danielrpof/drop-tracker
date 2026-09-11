---
phase: 19-frontend-system-view
plan: "05"
subsystem: ui
tags: [react, typescript, tables, state-machine, vitest, go-embed]

requires:
  - phase: 19-frontend-system-view/19-04
    provides: OutcomeBadge/classifyOutcome, AboutInstance, SourcePanel (ending in a trailing Separator), and system.tsx's loaded body this plan extends
provides:
  - "SourceHistoryTable (web/app/components/system/SourceHistoryTable.tsx) -- the eight-column per-source recent-runs table in contract order, with the three-condition cap/count/absent caption"
  - "deriveLoadedShape (web/app/routes/system.tsx) -- the D-04 first-run vs loaded predicate, computed during render every fetch, never latched into state"
  - "system.tsx's finished five-state render machine: loading skeleton, error (Refresh hidden), first-run, loaded, plus the session-expired 401 path inherited from apiFetch"
  - "handleRefresh -- its own re-entrancy ref guard and mounted guard, keep-stale-on-failure semantics, and the as-of freshness stamp taken from the client clock at fetch resolution"
  - "the regenerated internal/webassets/build/client embedded SPA bundle carrying the finished System view"
affects: []

actuals:
  tokens: 6807
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "deriveLoadedShape is a pure function called during render, never useState -- a Refresh returning an emptied ring buffer after a restart falls back to first-run automatically because the shape is recomputed from the freshest payload on every render, not cached"
    - "handleRefresh's re-entrancy guard is a useRef boolean checked and set synchronously before the first await, not the refreshing state variable -- two clicks dispatched in the same task both close over the same pre-render state snapshot, so only a ref (mutated in place, not through React's scheduler) reliably blocks the second call"

key-files:
  created:
    - web/app/components/system/SourceHistoryTable.tsx
  modified:
    - web/app/components/system/SourcePanel.tsx
    - web/app/routes/system.tsx
    - web/app/routes/system.test.tsx
    - internal/webassets/build/client

key-decisions:
  - "Re-entrancy guard uses a useRef, not the refreshing state boolean -- a real double-click test proved the state-only guard let both clicks through when the mocked response resolved before the second click's onClick fired; the fix generalizes to any two same-task invocations, not just the double-click case, since a ref mutation is visible to every closure immediately rather than only after React commits a re-render"
  - "The double-click test's mock deliberately never resolves (`new Promise(() => {})`) so the second half of the click genuinely lands while the first request is still in flight -- a mock that resolves instantly (the initial attempt) let the first request finish before the second click fired, which tested sequential clicks rather than a real race"
  - "Three pre-existing SourcePanel/history tests (from 19-04) broke as a direct, in-scope consequence of this plan's changes and were fixed here: two ordering tests whose all-null/zero-skips fixtures accidentally satisfied the new first-run predicate (fixed by giving one source a non-zero skip count, preserving the ordering assertion under the new state machine), and one healthy-run test whose Success/duration text became ambiguous once the same run's data also appears in its own history table row (fixed with getAllByText length assertions)"

patterns-established:
  - "SourceHistoryTable renders nothing (not an empty table shell) at zero history entries -- the runless panel line from 19-04 stands in, matching the UI-SPEC's explicit ruling-out of a headers-with-no-rows shape"

requirements-completed: [SYS-02, SYS-03]

coverage:
  - id: D1
    description: "Each source panel renders its own eight-column recent-runs table (Cycle/Started/Outcome/Duration/Checked/Skipped/Errored/Events) in contract order with no client-side sort, reusing OutcomeBadge for the outcome cell rather than re-deriving a tier"
    requirement: SYS-02
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the last-run badge, verbatim summary, duration, and the clean-run line for a healthy run"
        status: pass
      - kind: other
        ref: "grep -c \"OutcomeBadge\" web/app/components/system/SourceHistoryTable.tsx -> 3; grep -c \"sort(\" -> 0; grep -c \"Completed with errors\" -> 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "The history-table caption fires the fifty-entry cap copy at exactly 50 entries, the pluralized since-last-restart count copy between 1 and 49, and no table at all (and therefore no caption) at zero entries"
    requirement: SYS-02
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the cap caption when a source's history has fifty entries"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the singular count caption for a single-entry history"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders no table element when a source has zero history entries"
        status: pass
    human_judgment: false
  - id: D3
    description: "The first-run state fires only when every source simultaneously has a null last run, empty history, and zero consecutive skips; a source with a non-zero skip count or any run data renders loaded instead, and the state is recomputed every successful fetch rather than latched"
    requirement: SYS-03
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the first-run state naming the humanized poll interval when no source has ever run"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders both the runless line and the skip line for a runless-but-skipping source"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > moves from loaded to first-run when a Refresh returns an all-runless payload"
        status: pass
      - kind: other
        ref: "grep -c \"useState.*viewState\\|setViewState\" web/app/routes/system.tsx -> 0; grep -c \"consecutive_skips\" -> 2"
        status: pass
    human_judgment: false
  - id: D4
    description: "The error state hides the header Refresh control entirely, offering Retry as the sole recovery affordance; the loading state renders a card-skeleton layout mirroring the loaded shape rather than a bare spinner, with Refresh present but disabled"
    requirement: SYS-03
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > hides the Refresh control in the error state"
        status: pass
    human_judgment: false
  - id: D5
    description: "Refresh keeps all previously rendered data on screen while in flight and on a non-401 failure, showing the locked inline failed-refresh line in a polite live region and holding the freshness stamp at the last success; a double-click issues no third fetch"
    requirement: SYS-03
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > keeps previously rendered values on screen and shows the failed-refresh line when a Refresh fails"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > issues exactly two total fetch calls across mount plus a double-clicked Refresh"
        status: pass
      - kind: other
        ref: "grep -c \"aria-live\" web/app/routes/system.tsx -> 1"
        status: pass
    human_judgment: false
  - id: D6
    description: "A Refresh rejecting with a session expiry sets no state and leaves no request unaccounted for behind the login screen -- neither the error state nor the inline failure line renders, and a Refresh resolving after the view has unmounted emits no React unmounted-component warning"
    requirement: SYS-03
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders neither the error state nor the inline failure line when a Refresh rejects with a session expiry"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > sets no state and emits no unmounted-component warning when a Refresh resolves after unmount"
        status: pass
      - kind: other
        ref: "grep -c \"mountedRef\" web/app/routes/system.tsx -> 7"
        status: pass
    human_judgment: false
  - id: D7
    description: "The as-of freshness stamp is taken from the client clock at fetch resolution (never a payload field), renders in both the first-run and loaded states, and updates only on a successful fetch"
    requirement: SYS-03
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the first-run state naming the humanized poll interval when no source has ever run"
        status: pass
      - kind: other
        ref: "manual read-back -- system.tsx's header <time> block renders unconditionally whenever !loadError and asOf is set, independent of the first-run/loaded branch"
        status: pass
    human_judgment: false
  - id: D8
    description: "No interval timer, page-visibility listener, or window-focus refetch exists anywhere in the System view's files; no dangerouslySetInnerHTML entered the SPA"
    verification:
      - kind: other
        ref: "grep -rn \"setInterval|setTimeout|visibilitychange|addEventListener\" web/app/routes/system.tsx web/app/components/system | wc -l -> 0; grep -rn \"dangerouslySetInnerHTML\" web/app | wc -l -> 0"
        status: pass
    human_judgment: false
  - id: D9
    description: "Frontend and backend Definition of Done both green: full frontend suite (16 files/205 tests) with all four coverage axes above 70%, prettier clean; backend go build/vet/golangci-lint/coverage-gate (90.89%)/sqlc-check all clean with no Go file touched this phase; the embedded SPA bundle regenerated with a confirmed non-empty diff (including a new system-*.js chunk) and committed; no file outside web/, the bundle, and .planning changed this phase"
    verification:
      - kind: unit
        ref: "corepack pnpm --dir web test -- 16 files / 205 tests, statements 90.21% / branches 83.17% / functions 88.13% / lines 91.77%"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./... && golangci-lint run -- clean; make coverage-gate -- 90.89% pass; make sqlc-check -- no drift; make web && git status --porcelain internal/webassets/build/client | wc -l -> 28 (non-empty); git diff --name-only <phase-base>..HEAD -- . ':!web' ':!internal/webassets' ':!.planning' -> empty"
        status: pass
    human_judgment: true
    rationale: "The plan's <verify> block also names a human-check step (a real gated instance, real database, live browser confirmation of Refresh/logout/panel rendering) that this sandboxed execution cannot perform -- carried forward as the phase's outstanding UAT item, same as 18.1's live-instance status check."

duration: 45min
completed: 2026-09-11
status: complete
---

# Phase 19 Plan 5: Recent-Runs History Table, Five-State Machine, and the Embedded Bundle Summary

**The per-source recent-runs table with its cap caption, the finished five-state render machine (first-run predicate, error-hides-Refresh, keep-stale Refresh with its own re-entrancy/mounted guards, and the as-of freshness stamp), and the phase-closing embedded SPA bundle rebuild.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-09-11T02:20:00Z (approx.)
- **Completed:** 2026-09-11T03:05:00Z (approx.)
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified — 1 of the 4 being the regenerated embedded bundle directory)

## Accomplishments

- `SourceHistoryTable.tsx`: the eight-column recent-runs table (Cycle/Started/Outcome/Duration/Checked/Skipped/Errored/Events) rendered in exact contract order (no client sort), reusing `OutcomeBadge` for the outcome cell; a zero-entry history renders nothing (the runless panel line stands in); the three-condition caption (50-entry cap, pluralized since-last-restart count, absent at zero) matches the UI-SPEC verbatim
- `SourcePanel.tsx` now renders the table below its trailing `Separator`, so each panel reads summary first, then table
- `system.tsx`'s `deriveLoadedShape` computes first-run vs loaded during render from the freshest payload every time — never stored in state — so a Refresh that returns an emptied ring buffer after a restart correctly falls back to first-run copy naming the humanized poll interval (with the database-unreachable variant when `schema_applied` is null)
- The render-precedence chain is complete: `error` hides the header Refresh control entirely (Retry is the sole recovery affordance there); `loading` renders a card-skeleton mirroring the loaded layout (About-block skeleton + two source-panel skeletons) with Refresh present but disabled
- `handleRefresh` is a standalone handler with its own `refreshingRef` re-entrancy guard (a ref, not the `refreshing` state variable — a real double-click test proved the state-only version let both clicks through) and its own `mountedRef` check before every state write; it never clears existing data, sets the as-of stamp only on success, and on a non-401 failure keeps the stale data while raising the locked inline failure line inside a polite live region
- A session-expiry rejection during Refresh sets no state at all and emits no React unmounted-component warning when it resolves after the view has unmounted, closing ROADMAP success criterion 5
- Full frontend suite: 16 files / 205 tests passing, coverage 90.21% statements / 83.17% branches / 88.13% functions / 91.77% lines; backend build/vet/lint/coverage-gate (90.89%)/sqlc-check all clean and unchanged
- `internal/webassets/build/client` regenerated via `make web` (confirmed non-empty diff, including a new `system-*.js` chunk) and committed as the phase's closing artifact

## Task Commits

Each task was committed atomically:

1. **Task 1: The per-source recent-runs table and its cap caption** - `cef4d1b` (feat)
2. **Task 2: The five-state machine, the keep-stale Refresh, and the freshness stamp** - `55fe9f0` (feat)
3. **Task 3: Phase gate — full Definition of Done, backend regression, embedded bundle rebuild** - `433ad77` (chore)

## Files Created/Modified

- `web/app/components/system/SourceHistoryTable.tsx` (new) - the eight-column history table + caption logic
- `web/app/components/system/SourcePanel.tsx` - wires the table in below the trailing Separator
- `web/app/routes/system.tsx` - `deriveLoadedShape`, the completed render-precedence chain, `handleRefresh` with its ref-based re-entrancy guard, the header Refresh/freshness-stamp row, and the failed-refresh live-region line
- `web/app/routes/system.test.tsx` - 10 new cases (cap/singular/zero-entry captions; first-run rendering; loaded→first-run on Refresh; Refresh hidden in error; double-click re-entrancy; failed-refresh keep-stale; session-expiry-during-Refresh; resolve-after-unmount) plus 3 pre-existing tests fixed as a direct consequence of this plan's changes
- `internal/webassets/build/client` - regenerated embedded SPA bundle (28 changed paths, including a new `system-*.js` chunk)

## Decisions Made

- Re-entrancy guard is a `useRef` boolean set/checked synchronously before the first `await`, not the `refreshing` state variable — the state-only version failed a real double-click test because both clicks in the same task closed over the same pre-render `refreshing` value; the double-click test itself uses a never-resolving mock so the second click genuinely lands while the first request is in flight, rather than the first request completing before the second click fires
- Fixed three pre-existing tests from 19-04 as a direct, in-scope consequence of this plan's own state-machine and table changes: two panel-ordering tests whose all-null/zero-skip fixtures accidentally satisfied the new first-run predicate (given one source a non-zero skip count instead), and one healthy-run test whose "Success"/duration text became ambiguous once the same run's data also appears in its own new history-table row (switched to `getAllByText` length assertions)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Refresh's re-entrancy guard used React state instead of a ref, letting a genuine double-click issue two requests**
- **Found during:** Task 2 (writing the double-click regression test)
- **Issue:** The initial `handleRefresh` checked `if (refreshing) return` against the `refreshing` state variable. A real `userEvent.dblClick` against a mock resolving near-instantly produced 3 total fetch calls (1 mount + 2 from the double-click) instead of the required 2, because both click events' closures read the same pre-render `refreshing` value before React committed the state update from the first click.
- **Fix:** Added a `refreshingRef` (`useRef(false)`) set/checked synchronously at the top of `handleRefresh`, before the state setter and before the first `await`. Rewrote the double-click test to hold the mocked response open (`new Promise(() => {})`) so the assertion genuinely exercises an in-flight second click rather than two sequential completed requests.
- **Files modified:** `web/app/routes/system.tsx`, `web/app/routes/system.test.tsx`
- **Verification:** `issues exactly two total fetch calls across mount plus a double-clicked Refresh` passes; full suite green.
- **Committed in:** `55fe9f0` (Task 2 commit)

**2. [Rule 1 - Bug] Three pre-existing tests broke against the new first-run/loaded state machine and the new history table**
- **Found during:** Task 2 (running the full suite after adding `deriveLoadedShape`)
- **Issue:** Two panel-ordering tests (from 19-04) used all-null, zero-skip fixtures for both sources — exactly the payload shape that now triggers `first-run`, so the panels those tests asserted on stopped rendering. One healthy-run test asserted a single `"Success"` and a single `"5.0s"`, which Task 1's new history table (rendering the same run a second time, in its own row) made ambiguous.
- **Fix:** Gave one source a non-zero `consecutive_skips` in the two ordering fixtures (forcing `loaded` while preserving the ordering assertion's intent); switched the healthy-run test's badge/duration assertions to `getAllByText(...).toHaveLength(2)`.
- **Files modified:** `web/app/routes/system.test.tsx`
- **Verification:** Full suite green, 205/205 tests passing.
- **Committed in:** `cef4d1b` (Task 1), `55fe9f0` (Task 2)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both fixes are direct, in-scope consequences of this plan's own changes (the re-entrancy guard and the state-machine/table additions) — no scope creep, no architectural change.

## Issues Encountered

None beyond the two deviations above, both resolved inline.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 19 (Frontend — System View) is complete: all three plans' requirements (SYS-01, SYS-02, SYS-03) are implemented, tested, and the embedded bundle carries the finished System route.
- The plan's `<verify>` block names one human-check step this sandboxed execution cannot perform: opening the System tab on a real gated instance with a live database and confirming panels/About block/Refresh/logout render correctly in a browser. This carries forward as the phase's outstanding UAT item, consistent with how 18.1's live-instance status check was tracked.
- No blockers or concerns for closing the phase.

## Self-Check: PASSED

All created/modified files verified present on disk; all three commit hashes (`cef4d1b`, `55fe9f0`, `433ad77`) verified in `git log --oneline --all`.

---

*Phase: 19-frontend-system-view*
*Completed: 2026-09-11*
