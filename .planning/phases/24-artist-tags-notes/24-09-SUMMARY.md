---
phase: 24-artist-tags-notes
plan: 09
subsystem: frontend
tags: [react, vitest, tags, gap-closure, golangci-lint, sqlc]

requires:
  - phase: 24-artist-tags-notes
    provides: plan 24-06's Manage tags delete/rename/merge wiring, plan 24-08's DB-level cap-on-UPDATE closure
provides:
  - "Route-level vocabulary sync on tag delete: dropTagFromEntries now filters the deleted tag out of `vocabulary`, so every row's \"+ tag\" autocomplete stops offering it as an existing suggestion (TAG-06, D-17, D-30)"
  - A new route test proving the deleted name is offered only via Create, other tags remain suggested, and listTags/listWatchlist are each called exactly once
  - The embedded SPA bundle (internal/webassets/build/client) rebuilt with the fix, so a Go-only clone serves it
  - Whole-phase Definition of Done green (go vet, golangci-lint, full integration suite, 80% backend coverage gate, sqlc-check, prettier, 70% frontend coverage)
affects: [24-verification]

actuals:
  tokens: 865
  tasks: 2
  commits: 3
plan_head_before: 375e88946d2fb99f8bcf5aae08e9f2a9a5c8f4c3

tech-stack:
  added: []
  patterns:
    - "dropTagFromEntries now matches renameTagInEntries/mergeTagInEntries's shape: a functional setEntries updater plus a functional setVocabulary updater, so every Manage tags mutation keeps both state slices in sync the same way"

key-files:
  created: []
  modified:
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
    - internal/webassets/build/client/index.html
    - internal/webassets/build/client/assets/watchlist-*.js
    - internal/webassets/build/client/assets/manifest-*.js

key-decisions:
  - "golangci-lint reinstalled at the pinned v2.13.2 via `go install` -- the pre-commit-cached binary (built with go1.25) refused to run against this module's `go 1.26` directive (\"the Go language version used to build golangci-lint is lower than the targeted Go version\"), a toolchain drift that occurred on this dev box between the 24-08 session and this one."
  - "`make` and bare `pnpm` remain absent from this session's PATH (per 24-08's precedent) -- ran the Makefile's `web` and Definition of Done targets' exact underlying commands directly (corepack pnpm install --frozen-lockfile / run build, the coverage-report/coverage-gate logic, sqlc generate + git diff) instead of through `make`."

requirements-completed: [TAG-06]

coverage:
  - id: D1
    description: "Deleting a tag in Manage tags removes it from every row's \"+ tag\" autocomplete: typing the deleted name offers only Create, other tags are still suggested, and neither listTags nor listWatchlist is called an extra time"
    requirement: "TAG-06"
    verification:
      - kind: integration
        ref: "web/app/routes/watchlist.test.tsx#deleting a tag in Manage tags removes it from the '+ tag' autocomplete, with no extra listTags call"
        status: pass
    human_judgment: false
  - id: D2
    description: "The embedded SPA bundle is rebuilt with the fix, and the whole phase's Definition of Done (go vet, golangci-lint, full integration suite, 80% backend coverage gate, sqlc-check, prettier check, 70% frontend coverage) is green, with no out-of-scope files touched"
    verification:
      - kind: other
        ref: "go vet ./...; golangci-lint run (v2.13.2); go test ./... -count=1 -coverprofile=coverage.out -coverpkg=...; cmd/coverage-report --mode=total (89.33%); sqlc generate + git diff; corepack pnpm --dir web exec prettier --check; corepack pnpm --dir web test (334/334, 92.06/85.04/91.79/94.23%)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Real-browser check: delete a tag from Manage tags, close the dialog, open \"+ tag\" on any card, type the deleted tag's name -- only Create is offered, other tags are still suggested"
    verification: []
    human_judgment: true
    rationale: "Deferred to phase-level UAT per workflow.human_verify_mode: end-of-phase -- this plan's Task 2 <verify> carries this as a <human-check>, consolidated into the phase's end-of-phase UAT batch rather than a mid-flight checkpoint."

duration: ~35min
completed: 2026-09-24
status: complete
---

# Phase 24 Plan 09: Autocomplete Vocabulary Sync on Delete Summary

**A one-statement fix (`setVocabulary` filter in `dropTagFromEntries`) closes the last phase-24 verification gap, and the whole-phase Definition of Done is green on a rebuilt embedded SPA.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-09-24T21:30:00Z
- **Completed:** 2026-09-24T22:05:00Z
- **Tasks:** 2 completed
- **Files modified:** 5 (2 source, 3 embedded-bundle artifacts)

## Accomplishments

- Deleting a tag in Manage tags now disappears from every card **and** from every row's "+ tag" autocomplete, with no refetch -- the deleted name can only come back through an explicit `Create`. Closes verification gap 2 (24-VERIFICATION.md, code review WR-01).
- A new route test (`web/app/routes/watchlist.test.tsx`) proves this via TDD: it fails on the stale-vocabulary code (RED), passes after the one-line `setVocabulary` fix (GREEN), and asserts `listTags`/`listWatchlist` are each called exactly once across the whole flow.
- The embedded SPA bundle is rebuilt and committed, so a Go-only clone serves the fix.
- The full phase-24 Definition of Done is green: `go vet`, `golangci-lint` (0 issues), the full Go integration suite (28 packages, all `ok`), 89.33% backend coverage (floor 80%), `sqlc-check` clean, `prettier --check` clean, and the frontend suite (334/334 tests, 92.06/85.04/91.79/94.23% on the four coverage axes, floor 70%).
- Confirmed blast radius: no dependency file or `internal/notifier`/`detection`/`discord`/`musicbrainz`/`deezer` package changed against `main`; only the route and its test changed under `web/app` since the planning-time head `447baa0`.

## Task Commits

Each task was committed atomically, RED/GREEN split for the TDD tracer task:

1. **Task 1 RED: add failing test for stale vocabulary after tag delete** - `ca94046` (test)
2. **Task 1 GREEN: drop deleted tag from route vocabulary** - `a716c12` (feat)
3. **Task 2: rebuild embedded SPA and close phase 24 Definition of Done** - `54dab01` (docs)

**Plan metadata:** committed alongside this SUMMARY (STATE.md/ROADMAP.md update).

_Task 1 is `type="tracer" tdd="true"`: RED then GREEN, no REFACTOR commit (the fix was already minimal -- a single functional-updater line matching the existing rename/merge pattern). The tracer feedback gate was re-run end-to-end after GREEN (interactive, `human_verify_mode: end-of-phase`, `<verify>` carries only `<automated>` entries) and passed, so execution continued straight to Task 2 with no checkpoint._

## Files Created/Modified

- `web/app/routes/watchlist.tsx` - `dropTagFromEntries` now also calls `setVocabulary((v) => (v ? v.filter((t) => t.id !== tagId) : v))`; comment trimmed to 2 lines per project comment discipline
- `web/app/routes/watchlist.test.tsx` - new test: "deleting a tag in Manage tags removes it from the '+ tag' autocomplete, with no extra listTags call"
- `internal/webassets/build/client/index.html`, `.../assets/watchlist-*.js`, `.../assets/manifest-*.js` - rebuilt SPA bundle (content-hashed filenames changed, confirming new content)

## Decisions Made

- golangci-lint reinstalled at the pinned v2.13.2 via `go install` after the pre-commit-cached binary (built with go1.25) refused to run against this module's `go 1.26` directive -- a toolchain drift on this dev box since the 24-08 session, not caused by this plan's changes.
- `make` and bare `pnpm` are still absent from this session's PATH (as documented in 24-08-SUMMARY.md); ran the Makefile's `web` target and Definition of Done gates via their exact underlying commands instead.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] golangci-lint version/toolchain mismatch**
- **Found during:** Task 2 (Definition of Done)
- **Issue:** The pre-commit-cached golangci-lint binary (built with go1.25) refused to run: "the Go language version (go1.25) used to build golangci-lint is lower than the targeted Go version (1.26)". This dev box's Go toolchain had moved to 1.26 since the 24-08 session.
- **Fix:** Reinstalled golangci-lint at the exact pinned version (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2`), matching `.pre-commit-config.yaml`'s pinned rev. Verified `golangci-lint version` reports `2.13.2` before running.
- **Files modified:** None (tooling-only; no source or config file changed)
- **Verification:** `golangci-lint run` completed cleanly with "0 issues" against the same `.golangci.yml`
- **Committed in:** N/A (no repo change -- local toolchain fix only)

---

**Total deviations:** 1 auto-fixed (1 blocking, tooling-only)
**Impact on plan:** No scope creep -- purely a local dev-environment toolchain fix required to run an unmodified Definition of Done gate. No plan content, source, or config changed as a result.

## Issues Encountered

None beyond the golangci-lint toolchain mismatch above (documented as a deviation, not an issue requiring plan-content change). All acceptance criteria and verification commands passed after that fix.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both phase-24 verification gaps are now closed: gap 1 (DB cap on UPDATE, 24-08) and gap 2 (autocomplete vocabulary sync, this plan).
- The whole-phase Definition of Done is green on the current `internal/webassets/build/client` embed.
- 24-VERIFICATION.md's remaining items are all browser-only human-verification checks (listed under "Human Verification Required" in that report), consolidated for phase-level UAT per `workflow.human_verify_mode: end-of-phase` -- including this plan's Task 2 `<human-check>` (D3 above).
- No further plans are scoped for this phase; ready for `/gsd-verify-work 24`.

## Self-Check: PASSED

- `web/app/routes/watchlist.tsx` and `web/app/routes/watchlist.test.tsx` confirmed present on disk with the expected content (`setVocabulary` filter; new test title).
- `internal/webassets/build/client` confirmed rebuilt (content-hashed asset filenames changed from the pre-plan commit).
- `git log --oneline --all --grep="24-09"` returns 3 commits (RED, GREEN, DoD/rebuild).
- Re-ran the plan-level `<verification>` block after the final commit: `corepack pnpm --dir web exec vitest run app/routes/watchlist.test.tsx --coverage.enabled=false` (19/19 pass), `prettier --check` + `typecheck` clean, `go vet`/`golangci-lint`/full integration suite/`coverage-gate` (89.33%)/`sqlc-check` all pass, and the blast-radius diffs against `main` and `447baa0` are both exactly as required.

---
*Phase: 24-artist-tags-notes*
*Completed: 2026-09-24*
