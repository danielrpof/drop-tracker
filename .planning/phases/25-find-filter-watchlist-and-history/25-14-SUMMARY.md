---
phase: 25-find-filter-watchlist-and-history
plan: 14
subsystem: ui
tags: [go-embed, spa-bundle, uat, gap-closure, docs]
requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "Gap fixes from plans 25-11 (scroll anchor), 25-12 (Detected line), 25-13 (Deezer button cap)"
provides:
  - "Rebuilt internal/webassets/build/client serving G-25-4, G-25-7, and G-25-9 fixes from a Go-only build"
  - "UI-SPEC, UAT, and VERIFICATION contracts reworded to the user's 2026-10-11 decisions"
  - "Green Definition of Done run and blast-radius check against 461267f"
affects: [gsd-verify-work re-verification of G-25-4, G-25-7, G-25-9]
actuals:
  tokens: 175000
  tasks: 3
  commits: 2
plan_head_before: 900706e25bb213359e61da02d27a40dc7756b709
tech-stack:
  added: []
  patterns:
    - "Bundle-fix proof: compare marker counts against the pre-change tree, never bare presence"
key-files:
  created: []
  modified:
    - internal/webassets/build/client
    - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md
    - .planning/phases/25-find-filter-watchlist-and-history/25-UAT.md
    - .planning/phases/25-find-filter-watchlist-and-history/25-VERIFICATION.md
key-decisions:
  - "G-25-4 decision line extended append-only with the 2026-10-11 addendum; every original character kept"
requirements-completed: [WLVW-01, WLVW-04, HIST-02]
coverage:
  - id: D1
    description: "Embedded SPA rebuilt from the gap-closure web/ so the Go binary serves the scroll anchor, Detected line, and capped Deezer button"
    requirement: WLVW-01
    verification:
      - kind: other
        ref: "marker counts vs 461267f: Detected 2->3, scrollBy 0->1, max-w-\\[40\\%\\] 0->1; go build ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "UI-SPEC, UAT (tests 4/7/9 and three gap truths), and VERIFICATION (checks 4 and 7) reworded; no result/status/severity/root_cause/Summary count change"
    requirement: HIST-02
    verification:
      - kind: other
        ref: "Task 2 verify: exact-match decision line, UAT diff has no result/status lines, VERIFICATION status/score unchanged"
        status: pass
    human_judgment: false
  - id: D3
    description: "Definition of Done gates and blast radius against 461267f"
    requirement: WLVW-04
    verification:
      - kind: integration
        ref: "go vet, golangci-lint, go test ./... with coverage (90.32%), sqlc version guard + generate + diff, typecheck, prettier, web suite (95.16/90.47/95.8/97.09)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Real-browser re-test of UAT tests 4, 7 (EVENT_RETENTION_DAYS=45), and 9 at 375/640/1024px"
    verification: []
    human_judgment: true
    rationale: "Scroll, layout, and retention-edge data only show in a real browser (end-of-phase human verify, /gsd-verify-work)"
duration: 4min
completed: 2026-10-11
status: complete
---

# Phase 25 Plan 14: Embed Rebuild, Contract Rewording, and Definition of Done Summary

**Embedded SPA rebuilt so a Go-only build serves all three gap fixes, the UI-SPEC/UAT/VERIFICATION contracts reworded to the user's 2026-10-11 decisions, and every Definition of Done gate green with an empty blast radius against 461267f.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-10-11T20:31:19Z
- **Completed:** 2026-10-11T20:35:06Z
- **Tasks:** 3
- **Files modified:** 17 (14 bundle files, 3 docs)

## Accomplishments
- Ran the `make web` recipe directly (frozen install, build, replace, copy). The lockfile did not drift. Marker counts under `internal/webassets/build/client/assets` rose strictly above the `461267f` tree: `Detected` 2 to 3, `scrollBy` 0 to 1, `max-w-\[40\%\]` 0 to 1. `go build ./...` compiles the embed.
- UI-SPEC: D-12 bullet renamed "Scroll reset and anchoring (D-12, G-25-4)" and names `useScrollAnchor` and the browser clamp. The Load more bullet says retention counts from `created_at` (Phase 10 D-01). The Copywriting table has the `Detected {date}` row. The SearchResultsColumns note carries the G-25-9 exception. The carry item says the clicked card keeps its on-screen position.
- UAT: `expected:` of tests 4, 7, 9 and the three gap `truth:` lines reworded. G-25-4's `decision:` is its `461267f` form with only the closing quote moved after the addendum (exact full-line match verified). No `result:`, `status:`, `severity:`, `root_cause:`, or Summary count touched.
- VERIFICATION: human checks 4 and 7 reworded in the frontmatter list and the body. `status: human_needed` and `score:` unchanged.
- Definition of Done: vet clean, golangci-lint 0 issues, backend coverage 90.32% (gate 80), sqlc 1.31.1 matched the Makefile before generate and the diff is clean, typecheck and prettier pass, web coverage 95.16 statements / 90.47 branches / 95.8 functions / 97.09 lines (gate 70).

## Task Commits

1. **Task 1: Rebuild the embedded SPA** - `afb881c` (chore)
2. **Task 2: Reword UI-SPEC, UAT, VERIFICATION** - `1d3017f` (docs)
3. **Task 3: Definition of Done and blast radius** - no commit (no gate fix needed)

**Plan metadata:** committed separately (docs: complete plan)

## Files Created/Modified
- `internal/webassets/build/client` - rebuilt hashed assets and `index.html` (superseded hashed files deleted)
- `.planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md` - D-12 anchoring, retention wording, Detected row, G-25-9 exception, carry item
- `.planning/phases/25-find-filter-watchlist-and-history/25-UAT.md` - reworded tests 4/7/9, gap truths, G-25-4 decision addendum
- `.planning/phases/25-find-filter-watchlist-and-history/25-VERIFICATION.md` - human checks 4 and 7

## Decisions Made
None - followed plan as specified.

## Accepted exceptions

Accepted exception: CI-only race (user decision 2026-10-11)

Reason: cgo is disabled on this Windows box, so `go test -race` cannot run locally. The local integration run omitted `-race`. CI's Linux test job runs `-race` and is the race gate.

## Deviations from Plan

None - plan executed exactly as written.

The plan's known `go: no such tool "covdata"` quirk for `internal/testutil` did not appear in this run: no `FAIL` or `--- FAIL` line was in the test log, and every package reported `ok`.

## TDD Gate Compliance

Not applicable (no `tdd="true"` tasks, plan type `execute`).

## Issues Encountered
None. The tracer gate ran in end-of-phase mode with an automated-only verify, so the verify result stood in for a re-run and expansion continued.

## Known Stubs
None.

## Threat Flags
None. No new endpoints, auth paths, or trust-boundary schema changes. T-25-SC, T-25-38, T-25-39, and T-25-40 mitigations all held: frozen lockfile, marker-count check, empty blast-radius diff, and the append-only decision check.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
Ready for `/gsd-verify-work`. Three real-browser re-test items are queued: test 4 (chip-label click far down a 50+ artist list; muted toggle only promises no router reset), test 7 (run with `EVENT_RETENTION_DAYS=45`, tag, Load more to the end, reload, Back), and test 9 (Deezer rows at 375/640/1024px).

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-11*

## Self-Check: PASSED
