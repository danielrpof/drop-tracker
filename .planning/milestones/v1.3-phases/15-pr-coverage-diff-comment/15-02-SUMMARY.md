---
phase: 15-pr-coverage-diff-comment
plan: 02
subsystem: ci
tags: [coverage, makefile, go-tooling, coverage-gate, coverpkg]

requires:
  - phase: 15-pr-coverage-diff-comment
    provides: "cmd/coverage-report --mode=total (bare 2-decimal backend %) from plan 15-01"
  - phase: 09-ci-coverage-gates
    provides: "the Makefile coverage-gate target, COVER_PKGS anchored exclusion, and the COVERAGE_THRESHOLD_BACKEND ?= 80 literal"
provides:
  - "make coverage-report — phony target printing the backend total (2 decimals) via the plan-15-01 tool; loud stderr + non-zero exit when no profile exists"
  - "COVER_PKGS extended to also drop cmd/coverage-report from the 80% backend denominator (D-07); cmd/server and every other package still in"
  - "coverage-gate measures by consuming the tool's --mode=total number instead of scraping go tool cover -func — one algorithm shared with the PR comment (D-17)"
  - "backendTotalPct now merges duplicate profile blocks, so it matches go tool cover -func on a real merged go-test profile"
  - "recorded cutover margin: old gate 90.0%, tool 90.03%, ~10.03pp above the 80 floor"
affects: [15-03, coverage-gate, full-pipeline.yml]

actuals:
  tokens: 4200
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "One coverage algorithm for both the merge-blocking gate and the report: coverage-gate shells `go run ./cmd/coverage-report --mode=total` (D-17)"
    - "Go coverage-profile parsing merges basic blocks by position before statement-weighting (matches x/tools/cover), required for a `go test ./...` merged profile"

key-files:
  created: []
  modified:
    - Makefile
    - cmd/coverage-report/main.go
    - cmd/coverage-report/main_test.go

key-decisions:
  - "coverage-gate and coverage-report both invoke `go run ./cmd/coverage-report --mode=total --profile=coverage.out` directly (not `$(MAKE) coverage-report`), giving two greppable call sites of the one algorithm (D-17)."
  - "backendTotalPct was summing numStmts per profile line; a real merged `go test ./...` profile repeats every block ~10x, so it reported 7.97% where the gate reported 90.0%. Fixed by merging blocks on their `path:sL.sC,eL.eC` position key and summing counts before weighting."
  - "The cutover margin was measured on a substituted run (plain `go test ./... -p 1`, race detector dropped) because this dev box cannot build the race runtime (documented A1 / STATE.md limitation). Coverage total is unaffected by the race flag."

patterns-established:
  - "Makefile measured-number seam: the threshold literal stays greppable in the Makefile, the measured value comes from a tool (D-17, Phase 09 posture)."

requirements-completed: [CICD-13, CICD-14]

coverage:
  - id: D1
    description: "cmd/coverage-report is excluded from COVER_PKGS (the 80% backend gate denominator) while cmd/server and internal/db/sqlc handling is unchanged (D-07)"
    requirement: CICD-13
    verification:
      - kind: integration
        ref: "make -n test-integration | grep -c 'cmd/coverage-report' == 0; 'cmd/server' >= 1; 'internal/db/sqlc' == 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "make coverage-report prints the backend total (2 decimals) to stdout; with no coverage profile it writes a diagnostic to stderr and exits non-zero"
    requirement: CICD-14
    verification:
      - kind: integration
        ref: "make coverage-report with coverage.out moved away: exit non-zero, 0 bytes stdout, non-empty stderr; with a real profile: prints 90.03"
        status: pass
    human_judgment: false
  - id: D3
    description: "coverage-gate measures the backend total by consuming the tool's --mode=total output rather than scraping go tool cover -func; 80 literal, report echo, comparison and PASS/FAIL exits unchanged"
    requirement: CICD-13
    verification:
      - kind: integration
        ref: "make coverage-gate prints 'Backend coverage: 90.03% (required: 80%)' + PASS (exit 0); make coverage-gate COVERAGE_THRESHOLD_BACKEND=100 prints FAIL and exits non-zero; `grep -v '^#' Makefile | grep -c 'cover -func'` == 0; `grep -c 'coverage-report --mode=total' Makefile` == 2"
        status: pass
    human_judgment: false
  - id: D4
    description: "backendTotalPct matches go tool cover -func on a real merged go-test profile (merges duplicate blocks)"
    requirement: CICD-14
    verification:
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestBackendTotalPct_MergesDuplicateBlocks"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestBackendTotalPct (single-copy fixture, 86.84)"
        status: pass
      - kind: integration
        ref: "go run ./cmd/coverage-report --mode=total == 90.03 vs `go tool cover -func` total 90.0% on the same real profile"
        status: pass
    human_judgment: false
  - id: D5
    description: "Cutover margin recorded per D-17 planner-must-check: pre-refactor gate %, tool %, and margin above 80"
    verification:
      - kind: manual_procedural
        ref: "measured on a real integration run: old gate 90.0%, tool 90.03%, margin 10.03pp (>= 0.05, no top-up)"
        status: pass
    human_judgment: false

duration: 24min
completed: 2026-09-02
status: complete
---

# Phase 15 Plan 02: Wire the coverage tool into the build Summary

**`make coverage-gate` now measures the 80% backend floor by shelling `cmd/coverage-report --mode=total` (one algorithm shared with the PR comment, D-17), `cmd/coverage-report` is out of the coverage denominator it reports on (D-07), and a real-run cutover margin of 10.03pp above 80 is recorded.**

## Performance

- **Duration:** 24 min
- **Started:** 2026-09-02T22:06:00Z
- **Completed:** 2026-09-02T22:31:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- **`COVER_PKGS` exclusion extended (D-07)** — the anchored alternation now drops `cmd/coverage-report` alongside the generated sqlc package. `make -n test-integration` proves `cmd/server` is still in the `-coverpkg` list and both `cmd/coverage-report` and `internal/db/sqlc` are out. The rationale comment gained one line naming D-07; the doubled `$$` anchor survived the edit.
- **`make coverage-report` target added** — prints the backend total (2 decimals) via `go run ./cmd/coverage-report --mode=total --profile=coverage.out`, mirrors `coverage-gate`'s profile guard so a standalone call with no profile writes a one-line stderr diagnostic and exits non-zero. Added to `.PHONY` on line 1.
- **`coverage-gate` cutover (D-17)** — the measurement line now assigns from a command substitution invoking the tool; the `-s` profile guard, the empty-output guard, the `Backend coverage: … (required: 80%)` echo, the `awk` threshold comparison against `COVERAGE_THRESHOLD_BACKEND`, and both PASS/FAIL exits are byte-for-byte unchanged. `go tool cover -func` no longer appears in any non-comment Makefile line.
- **Cutover margin measured and recorded** — on a real integration run: old gate `90.0%`, tool `90.03%`, margin `10.03pp` above the 80 floor. Well clear of the 0.05pp danger zone, so no coverage top-up was required.

## Task Commits

1. **Task 1: Exclude the tool from the denominator + add coverage-report** — `a56530e` (chore)
2. **Task 2 deviation: merge duplicate profile blocks in backendTotalPct** — `b1967f6` (fix)
3. **Task 2: consume the tool from coverage-gate** — `6def7cc` (chore)

**Plan metadata:** (this commit) (docs)

## Files Created/Modified

- `Makefile` — `.PHONY` + `coverage-report` target; `COVER_PKGS` alternation extended (D-07); `coverage-gate` measurement line shells the tool (D-17); two rationale comments extended by one line each.
- `cmd/coverage-report/main.go` — `backendTotalPct` merges basic blocks by position key before statement-weighting; `parseBlockLine` returns the position key as a new value.
- `cmd/coverage-report/main_test.go` — new `TestBackendTotalPct_MergesDuplicateBlocks`; two existing call sites updated for the new `parseBlockLine` signature; `TestParseBlockLine_LastColonSplit` asserts the new `block` return.

## Cutover measurement (D-17 planner-must-check evidence)

| Value | Result |
|-------|--------|
| Pre-refactor gate percentage (`go tool cover -func` scrape) | 90.0% |
| Post-refactor tool percentage (`--mode=total`, 2 dp) | 90.03% |
| Margin above the 80 floor | 10.03 pp |
| Coverage top-up required? | No (margin ≥ 0.05 pp) |

**Race detector substitution:** the margin was measured on `go test ./... -p 1 -count=1 -coverprofile=coverage.out -coverpkg=$(COVER_PKGS)` — plain `go test`, no `-race`. This box cannot build the race runtime (`runtime/cgo: cgo.exe: exit status 2`, the documented A1 / STATE.md cgo-toolchain limitation). The race flag changes execution timing, not which statements execute, so the coverage total is unaffected. `-p 1` (serialised packages) was additionally needed: the full `./...` suite at default parallelism flaked one DB-backed poller test (`TestPoller_RunDeezerCycle_RecordsNewRelease`, passes in isolation) via cross-package fixture contention — a pre-existing known class of issue (STATE.md quick tasks 260826-hea / 260826-ia0), not caused by this plan. Serialised, the whole suite is green.

## Decisions Made

- Both `coverage-report` and `coverage-gate` call the tool directly rather than the gate calling `$(MAKE) coverage-report` — this satisfies the plan's "two call sites of `coverage-report --mode=total`" acceptance criterion and matches 15-PATTERNS.md.
- The `backendTotalPct` fix merges on the `path:startLine.startCol,endLine.endCol` position string and sums counts (covered iff the merged count > 0), which reproduces `golang.org/x/tools/cover` merge semantics for all three profile modes without adding the dependency.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `backendTotalPct` inflated the denominator on a real merged profile**
- **Found during:** Task 2, Step 3 (running the tool in total mode against the fresh `coverage.out`)
- **Issue:** `backendTotalPct` did `total += numStmts` for every profile line. A real `go test ./... -coverprofile` run concatenates one full block set per test binary under `-coverpkg`, so the merged `coverage.out` repeats every block ~10x (23,127 lines / 2,393 unique here), almost all copies with `count 0`. The tool reported `7.97%` where `go tool cover -func` and the old gate reported `90.0%`. Wiring that number into `coverage-gate` (D-17) would have turned `main` bright red — this is a straight bug, not the ~0.05pp rounding shift D-17's margin check anticipates.
- **Fix:** `backendTotalPct` now accumulates `numStmts` and summed `count` per unique block position, then weights over the merged set. `parseBlockLine` returns the position key. Matches `x/tools/cover` semantics.
- **Files modified:** `cmd/coverage-report/main.go`, `cmd/coverage-report/main_test.go`
- **Verification:** new `TestBackendTotalPct_MergesDuplicateBlocks` (raw sum would give 30%, merged gives 60%); existing `TestBackendTotalPct` / hostile-path / boundary fixtures still pass (no duplicates → unchanged); `go run ./cmd/coverage-report --mode=total` now prints `90.03` vs `go tool cover -func` `90.0%` on the same real profile.
- **Committed in:** `b1967f6`
- **Ledger:** recorded as WINDOWS.md entry 7, marked `fixed`.

---

**Total deviations:** 1 auto-fixed (1 Rule 1 bug)
**Impact on plan:** The fix is contained to `cmd/coverage-report` and is a prerequisite for Task 2's refactor being correct — without it the D-17 cutover fails its own success criterion. No scope creep; the plan's Task 2 measurement step is exactly what surfaced it (RESEARCH Pitfall 7 / A1 anticipated the need to compare the two numbers on a real run).

## Issues Encountered

- The full `go test ./...` suite flaked one poller test at default parallelism (see "Cutover measurement" above). Resolved by serialising with `-p 1`, matching the Makefile's own historical `-p 1` pin. Not a defect introduced by this plan.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Ready for 15-03** (CI wiring: the `coverage-comment` job, `cache/save` + `upload-artifact` steps, `web/vitest.config.ts` `json-summary` reporter). The tool's `--mode=total` and `--mode=comment` surfaces are now proven against a real merged profile, not just hand-crafted fixtures.
- `CICD-13` / `CICD-14` stay **blocked** in REQUIREMENTS.md — both are also declared by 15-03; `requirements.ready-ids` keeps them pending until that plan produces its SUMMARY. Expected, not a failure.
- The `backendTotalPct` merge fix also hardens the `--mode=sidecar` path that 15-03 wires into the baseline `cache/save` steps.

## Self-Check: PASSED

- Commits `a56530e`, `b1967f6`, `6def7cc` all present in git history.
- All modified files present on disk.
- Task 1 + Task 2 acceptance criteria re-run and passing (COVER_PKGS exclusion, `coverage-report` no-profile behaviour, `coverage-gate` two-decimal PASS, impossible-threshold FAIL, `cover -func` gone, two `--mode=total` call sites).
- Definition of Done: `go vet ./...`, `golangci-lint run`, `make sqlc-check`, `make coverage-gate` all exit 0; `make test` substituted with `go test ./... -p 1` (no `-race`, documented A1 limit) — green.

---
*Phase: 15-pr-coverage-diff-comment*
*Completed: 2026-09-02*
