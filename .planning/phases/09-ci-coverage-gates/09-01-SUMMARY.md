---
phase: 09-ci-coverage-gates
plan: 01
subsystem: infra
tags: [go-tool-cover, coverpkg, coverage-gate, makefile, ci-cd]

# Dependency graph
requires:
  - phase: 08-frontend-test-suite
    provides: frontend-test CI job created as report-only (not consumed by this backend-only plan, but sets the phase's job-extension precedent)
provides:
  - "Makefile COVER_PKGS variable — go list minus internal/db/sqlc, joined via paste"
  - "Makefile COVERAGE_THRESHOLD_BACKEND ?= 80 — the committed CICD-11 floor"
  - "Makefile coverage-gate target — parses coverage.out, log-only pass/fail, fails closed on missing/unparseable profile"
  - "test-integration extended with -coverprofile=coverage.out -coverpkg=$(COVER_PKGS)"
  - "09-BASELINE-BACKEND.md — measured backend starting baseline (83.5% aggregate, PASS at 80%) with D-09-ordered gap-closing priorities"
affects: [09-03-gap-closing-backend, 09-05-ci-wiring]

# Actuals (#2632)
actuals:
  tokens: 3158
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns: ["Hand-rolled Makefile shell-gate idiom (matches sqlc-version-check)", "-coverpkg package-list exclusion for cross-package coverage scoping"]

key-files:
  created: [.planning/phases/09-ci-coverage-gates/09-BASELINE-BACKEND.md]
  modified: [Makefile]

key-decisions:
  - "Backend coverage baseline measured locally without -race (same pre-existing, code-independent MSYS2/cgo toolchain limitation documented in 01-02/01-03) — Makefile's test-integration target itself retains -race unchanged for CI"
  - "Did not mark CICD-11 complete in REQUIREMENTS.md: this plan only builds the measure/gate mechanism and baseline; 09-03 (gap-closing) and 09-05 (CI wiring) also claim CICD-11 and must land before the requirement is truly satisfied end-to-end"

patterns-established:
  - "coverage-gate target: file-size test for missing/empty profile, awk-extracted total-line percentage, awk BEGIN-block float comparison (never shell [ / test), shell if/else does the pass/fail reporting"

requirements-completed: []  # CICD-11 intentionally left unmarked — see key-decisions

coverage:
  - id: D1
    description: "make test-integration produces a coverage.out that measures cmd/server and excludes internal/db/sqlc"
    requirement: "CICD-11"
    verification:
      - kind: other
        ref: "go tool cover -func=coverage.out | grep -c 'cmd/server' (>=1) and | grep -c 'internal/db/sqlc' (==0)"
        status: pass
    human_judgment: false
  - id: D2
    description: "make coverage-gate correctly gates pass/fail/missing-profile paths with decimal-safe comparison"
    requirement: "CICD-11"
    verification:
      - kind: other
        ref: "make coverage-gate COVERAGE_THRESHOLD_BACKEND=0 (PASS, exit 0); COVERAGE_THRESHOLD_BACKEND=100 (FAIL, non-zero); coverage.out renamed away (FAIL, non-zero)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Backend starting baseline measured and recorded before enforcement, with D-09-ordered gap-closing priorities"
    requirement: "CICD-11"
    verification:
      - kind: other
        ref: ".planning/phases/09-ci-coverage-gates/09-BASELINE-BACKEND.md"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-13
status: complete
---

# Phase 9 Plan 01: Backend Coverage Measurement & Gate Summary

**Instrumented `make test-integration` with `-coverpkg`-scoped coverage measurement (cmd/server in, generated sqlc out), added a hand-rolled `coverage-gate` Makefile target pinned at 80%, and measured/recorded the real backend baseline at 83.5% aggregate.**

## Performance

- **Duration:** ~25 min
- **Tasks:** 2
- **Files modified:** 2 (1 modified, 1 created)

## Accomplishments
- `Makefile`'s `test-integration` target now produces a correctly-scoped `coverage.out` — every first-party package including `cmd/server` (D-05) is measured, and generated `internal/db/sqlc` (D-04) is excluded, via a single `-coverpkg=$(COVER_PKGS)` flag rather than post-processing the profile
- New `coverage-gate` target: fails closed on a missing/empty profile, fails closed on an unparseable total, compares decimal percentages via `awk` (never shell `[`/`test`), and reports log-only pass/fail — all three paths (pass, fail, missing-profile) exercised for real, not assumed
- Real backend starting baseline measured and recorded in `09-BASELINE-BACKEND.md`: **83.5% aggregate, PASSES the 80% floor by 3.5 points**, with a D-09-ordered zero-coverage function map (`cmd/server` boot/shutdown orchestration first, then the MusicBrainz search-source wrapper, then the DSN-redaction helper) for 09-03 to target
- Re-checked the folded flaky-test todo under coverage instrumentation across 4 consecutive `-p 1` runs — no notifier timing flakes, no poller schema-visibility race observed in any run; the todo's own scope for this phase is satisfied

## Task Commits

1. **Task 1: End-to-end backend coverage path — instrument, parse, compare, exit** - `997120f` (feat)
2. **Task 2: Measure and record the backend starting baseline, and re-check the folded flake todo** - `1294524` (docs)

## Files Created/Modified
- `Makefile` - Added `COVER_PKGS`/`COVERAGE_THRESHOLD_BACKEND` variables, extended `test-integration` with `-coverprofile`/`-coverpkg`, added `coverage-gate` target
- `.planning/phases/09-ci-coverage-gates/09-BASELINE-BACKEND.md` - Measured backend baseline: aggregate total, gate verdict at 80%, zero-coverage function map, D-09 prioritization, run duration, and the 4-run flake re-check

## Decisions Made
- Measured the local baseline without `-race`: this dev machine's Go 1.26.5 + MSYS2 GCC 15.2.0 toolchain cannot run `-race`-instrumented binaries (confirmed this session via `ThreadSanitizer failed to allocate ... (error code: 87)` on an isolated single-package run with no other Go processes active — same pre-existing, code-independent limitation documented in `01-02-SUMMARY.md`/`01-03-SUMMARY.md`). The `Makefile`'s `test-integration` target itself retains `-race` unchanged, unaffected by this local limitation — CI (Linux, standard toolchain) runs it as written.
- Left `CICD-11` unmarked in `REQUIREMENTS.md`: this plan (09-01) is one of three plans claiming `CICD-11` in this phase (09-01 builds the mechanism + baseline, 09-03 closes the coverage gap, 09-05 wires the gate into CI). Marking the requirement complete after only the measurement mechanism exists would be inaccurate — CI does not yet enforce anything.

## Deviations from Plan

None — plan executed exactly as written. The `-race` toolchain workaround for local verification is not a deviation from the plan's instructions (which explicitly required keeping `-race` in the `Makefile` target unchanged); it is a documented, precedented limitation of this dev machine's local verification environment only, with the identical prior-phase precedent cited above.

## Issues Encountered
- Local `-race` execution fails with a ThreadSanitizer allocation error on this Windows dev machine's cgo toolchain — pre-existing, environment-specific, not caused by this plan's changes (reproduced identically on an untouched single package). Worked around for local verification only by running the identical `-coverprofile`/`-coverpkg` invocation without `-race`; the `Makefile` target itself is unchanged and will run `-race` correctly in CI's Linux runner.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `make coverage-gate` is ready for 09-05 to wire into `.github/workflows/full-pipeline.yml`'s `test` job as an appended step — no CI YAML was touched in this plan, per the phase's explicit sequencing.
- 09-03's gap-closing work has a concrete, D-09-ordered target list from `09-BASELINE-BACKEND.md`: `cmd/server` boot/shutdown (`run`/`main`), the MusicBrainz search-source wrapper (`internal/httpserver/search.go`), and the DSN-redaction helper (`internal/db/pool.go` `redactedTarget`).
- No blockers. The folded flake todo's re-check found no reproduction under this phase's instrumentation at `-p 1`; no new blocker surfaced for future work.

---
*Phase: 09-ci-coverage-gates*
*Completed: 2026-08-13*
