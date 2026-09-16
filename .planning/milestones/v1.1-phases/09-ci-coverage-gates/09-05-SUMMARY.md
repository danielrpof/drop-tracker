---
phase: 09-ci-coverage-gates
plan: 05
subsystem: cicd
tags: [github-actions, coverage-gate, build-scan, needs-graph]

# Dependency graph
requires:
  - phase: 09-ci-coverage-gates (plan 03)
    provides: The backend coverage-gate Makefile target and a measured 87.1% aggregate clearing the 80% floor
  - phase: 09-ci-coverage-gates (plan 04)
    provides: The frontend 70% coverage.thresholds committed in vitest.config.ts, verified to actually fire
provides:
  - "The test job's final step runs make coverage-gate, failing the job under the 80% backend floor, with no if and no continue-on-error"
  - "build-scan's needs array includes frontend-test as a sixth entry, completing the gating graph so either coverage gate transitively blocks build-scan and release"
affects: []

# Actuals (#2632)
actuals:
  tokens: 310
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Coverage-gate step reads the profile the prior integration-test step already wrote -- no re-run of the suite, matching the repo's 'CI and a developer run the same make target' convention"

key-files:
  created: []
  modified:
    - .github/workflows/full-pipeline.yml

key-decisions:
  - "timeout-minutes on the test job left at 15 (unchanged) -- 09-BASELINE-BACKEND.md's measured wall-clock duration (61s) is well under the plan's 7-minute decision threshold for raising it to 25"
  - "Precondition check for Task 1 (make coverage-gate exits 0 locally) required regenerating coverage.out via go test, since coverage.out is gitignored and this worktree had none on disk; ran the identical -coverprofile/-coverpkg invocation without -race, matching the same pre-existing local ThreadSanitizer/MSYS2 toolchain limitation documented in 01-02-SUMMARY.md, 01-03-SUMMARY.md, and 09-BASELINE-BACKEND.md -- confirmed 87.1% PASS, matching 09-03's recorded closing measurement"

patterns-established: []

requirements-completed: [CICD-11, CICD-12]

coverage:
  - id: D1
    description: "The test job's final step runs the same coverage gate a developer runs locally (make coverage-gate), at the committed 80% threshold, with no escape hatch"
    requirement: "CICD-11"
    verification:
      - kind: unit
        ref: ".github/workflows/full-pipeline.yml jobs.test.steps[-1] (YAML-parsed)"
        status: pass
      - kind: unit
        ref: "make coverage-gate (local, 87.1% PASS)"
        status: pass
    human_judgment: false
  - id: D2
    description: "build-scan requires all six upstream jobs including frontend-test, with the five pre-existing entries unchanged in order"
    requirement: "CICD-12"
    verification:
      - kind: unit
        ref: ".github/workflows/full-pipeline.yml jobs.build-scan.needs (YAML-parsed)"
        status: pass
    human_judgment: false
  - id: D3
    description: "On a real pipeline run, a deliberately under-threshold push turns the corresponding job red and build-scan is skipped rather than started"
    requirement: "CICD-11, CICD-12"
    verification:
      - kind: manual
        ref: "End-of-phase human check (scratch-branch backend/frontend threshold-raise experiments) -- not run this session, see Backstop Verification below"
        status: pending
    human_judgment: true

duration: ~35min
completed: 2026-08-13
status: complete
---

# Phase 9 Plan 5: Wire Both Coverage Gates Into the Pipeline Summary

**Added the backend `make coverage-gate` step as the final step of the `test` job and appended `frontend-test` to `build-scan`'s `needs` array — the two changes that turn both measured, locally-green coverage gates into pipeline gates blocking `build-scan` and, transitively, `release`.**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-08-13
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- `test` job gained a `Backend coverage gate (80)` step (`run: make coverage-gate`) as its fourth and final step, immediately after the existing `Run integration tests` step. No `if`, no `continue-on-error`. `timeout-minutes` stays at 15 — the recorded local wall-clock duration (61s, from `09-BASELINE-BACKEND.md`) is well under the plan's 7-minute decision threshold for raising it.
- `build-scan`'s `needs` array grew from five entries to six (`[vet, lint, test, gitleaks, trivy-fs, frontend-test]`), all five prior entries kept in their original order, one-line append plus an inline comment citing `08-CONTEXT.md` D-04 and `09-CONTEXT.md` D-11.
- `release`'s `needs` (`[build-scan]`) and its `if` condition are untouched — the blocking stays transitive through `build-scan` alone.
- No new third-party GitHub Action introduced (`grep -c 'uses:'` stayed at 26 before and after both tasks).
- Both locally-green gates re-verified unaffected by the wiring change: `make coverage-gate` → 87.1% PASS; `pnpm --dir web test` → 78.06/71.57/75.75/79.75, exit 0.

## Task Commits

Each task was committed atomically:

1. **Task 1: Run the backend coverage gate inside the test job** - `85338fe` (feat)
2. **Task 2: Make both coverage failures block the build, scan, and release chain** - `bf9ba17` (feat)

_No separate plan-metadata commit in worktree mode — STATE.md/ROADMAP.md are owned by the orchestrator after wave merge; this SUMMARY and REQUIREMENTS.md updates land in a follow-up commit per the worktree protocol._

## Files Created/Modified

- `.github/workflows/full-pipeline.yml` — added the `test` job's coverage-gate step (Task 1); appended `frontend-test` to `build-scan`'s `needs` (Task 2)

## Completed Gating Graph

```
build-scan needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]
release    needs: [build-scan]
```

A backend aggregate under 80% fails `test`; a frontend axis under 70% fails `frontend-test`; either failure means `build-scan` is skipped (GitHub Actions' documented `needs` behavior), which transitively blocks `release` — no image is built, scanned, tagged, pushed to the registry, or accompanied by an SBOM.

## Decisions Made

- Left `timeout-minutes: 15` unchanged on the `test` job — the plan's own decision rule only raises it to 25 if the recorded baseline duration exceeds 7 minutes, and `09-BASELINE-BACKEND.md` records 61 seconds.
- Task 1's precondition (`make coverage-gate` exits 0 locally, as plan 09-03 left it) required regenerating `coverage.out` first — it's gitignored and this fresh worktree had none on disk. Ran `go test ./... -count=1 -p 1 -coverprofile=coverage.out -coverpkg=...` (the same invocation 09-01/09-03 used for local measurement) without `-race`, since `-race`-instrumented binaries still fail to allocate under this dev machine's Go 1.26.5 + MSYS2 GCC toolchain (the same pre-existing, code-independent limitation documented in `01-02-SUMMARY.md`, `01-03-SUMMARY.md`, and `09-BASELINE-BACKEND.md`). Result: 87.1% PASS, matching 09-03's recorded closing measurement exactly — confirms the precondition was met without needing a side-effecting `-race` run.

## Deviations from Plan

None — plan executed exactly as written. Both tasks' acceptance criteria were satisfied without any auto-fix, architectural question, or scope change.

## Backstop Verification (not run this session)

The plan's `must_haves.truths` entry with `verification: backstop` — "On a real pipeline run, a deliberately under-threshold push turns the corresponding job red and `build-scan` is skipped rather than started" — and Task 2's `<human-check>` block (scratch-branch backend/frontend threshold-raise experiments, `gh run watch`) require observing a real GitHub Actions run. This cannot be produced from a local session or this worktree. Per this plan's `<parallel_execution>` instructions, this is recorded here as requiring a real pipeline run rather than treated as a blocker:

1. Not yet run: temporarily raise `COVERAGE_THRESHOLD_BACKEND` above 87.1% on a scratch branch, push, confirm `test` goes red and `build-scan` is reported skipped.
2. Not yet run: temporarily raise all four Vitest threshold axes above their current measured figures on a scratch branch, push, confirm `frontend-test` goes red and `build-scan` is again skipped.
3. Not yet run: restore both thresholds, push, confirm the full pipeline goes green through `build-scan`.

Everything else in this plan's `<verification>` block — YAML validity, the `needs` graph structure, absence of `if`/`continue-on-error` escape hatches, the unchanged `uses:` count, and both gates passing locally — was verified statically and is recorded as PASS above.

## Self-Check: PASSED

All commits verified present in `git log` (`85338fe`, `bf9ba17`). Modified file verified present on disk (`.github/workflows/full-pipeline.yml`) with both task diffs confirmed via `git diff bf5266d bf9ba17`.

---
*Phase: 09-ci-coverage-gates*
*Completed: 2026-08-13*
