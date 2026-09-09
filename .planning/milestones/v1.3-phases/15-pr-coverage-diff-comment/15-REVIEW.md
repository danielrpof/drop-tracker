---
phase: 15-pr-coverage-diff-comment
reviewed: 2026-09-02T00:00:00Z
depth: standard
files_reviewed: 18
files_reviewed_list:
  - cmd/coverage-report/main.go
  - cmd/coverage-report/main_test.go
  - .github/workflows/full-pipeline.yml
  - Makefile
  - .golangci.yml
  - web/vitest.config.ts
  - cmd/coverage-report/testdata/comment-normal.golden.md
  - cmd/coverage-report/testdata/comment-no-baseline.golden.md
  - cmd/coverage-report/testdata/comment-unchanged.golden.md
  - cmd/coverage-report/testdata/comment-unavailable.golden.md
  - cmd/coverage-report/testdata/backend-profile.txt
  - cmd/coverage-report/testdata/backend-profile-boundary.txt
  - cmd/coverage-report/testdata/backend-profile-malformed.txt
  - cmd/coverage-report/testdata/backend-profile-hostile-paths.txt
  - cmd/coverage-report/testdata/baseline-metrics-backend.json
  - cmd/coverage-report/testdata/baseline-metrics-frontend.json
  - cmd/coverage-report/testdata/coverage-summary.json
  - cmd/coverage-report/testdata/coverage-summary-boundary.json
findings:
  critical: 0
  warning: 6
  info: 5
  total: 11
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-09-02
**Depth:** standard
**Files Reviewed:** 18
**Status:** issues_found

## Summary

The `cmd/coverage-report` tool and its wiring into `full-pipeline.yml` are well-constructed. The
locked constraints are honored: `coverage-comment` has `needs: [test, frontend-test]` and nothing
else, is absent from the `build-scan`/`release` needs graph, scopes `pull-requests: write` to the
job (workflow-level stays `contents: read`), uses `pull_request` (not `pull_request_target`), guards
the comment upsert to same-repo PRs so forks degrade to the job summary, and is `continue-on-error`
plus off the merge-gate graph so it can never block. The basic-block merge-by-position fix (commit
`b1967f6`) is correct — the merge key is the full `path:startLine.startCol,endLine.endCol` string and
counts are summed across repeated segments, matching `x/tools/cover` semantics and the
`TestBackendTotalPct_MergesDuplicateBlocks` assertion. SHA handling is solid: every SHA reaching the
rendered body passes `validSHA` (7–40 lowercase hex) via `shortSHA`, and the hostile-path fixture
test proves coverage-file paths are never interpolated into the markdown. Comment mode returns `nil`
on every bad-input path.

No blocking defects were found. Six warnings concern a gate-rounding leniency, a token-exposure
surface in the CI job, and three test-coverage gaps on degradation paths that are the reason the job
exists.

## Warnings

### WR-01: `round2` half-up rounding lets sub-threshold coverage pass the 80% floor

**File:** `cmd/coverage-report/main.go:152`, consumed by `Makefile:101-112`
**Issue:** `round2(x)` is `math.Floor(x*100+0.5)/100`. `make coverage-gate` compares the tool's
2-decimal `--mode=total` output (`round2`'d) against the threshold with `awk 'cov + 0 >= thresh + 0'`.
A true backend coverage of 79.995%–79.999% rounds up to `80.00` and passes an 80% floor that
CLAUDE.md and `Makefile:38-42` describe as "the required floor, not a tunable." D-17 routed the gate
through this same rounded value, so the leniency is now the enforced behavior.
**Fix:** Compare the unrounded percentage against the threshold for the gate decision (keep `round2`
only for display), or truncate rather than round-half-up when producing the gate number:
```go
// gate/total mode: floor to 2dp so the displayed number never overstates coverage
func floor2(x float64) float64 { return math.Floor(x*100) / 100 }
```

### WR-02: `coverage-comment` runs PR-authored code while holding a write-scoped token

**File:** `.github/workflows/full-pipeline.yml:191-268`
**Issue:** The job checks out the PR head (default `persist-credentials: true`, so `GITHUB_TOKEN` is
written into `.git/config`) and then runs `go run ./cmd/coverage-report`, compiling and executing
`main.go` as it exists on the PR branch, in a job that declares `pull-requests: write`. For fork PRs
GitHub forcibly downgrades the token to read-only, so the exposure is real only for same-repo branch
PRs — where the author already has write access — which keeps severity low. But the tool never needs
git credentials, so persisting a write-capable token alongside PR-controlled build code is an
unnecessary surface.
**Fix:** Add `with: { persist-credentials: false }` to the checkout step in the `coverage-comment`
job. The `marocchino/sticky-pull-request-comment` step gets its token via the action's own input /
`GITHUB_TOKEN`, not `.git/config`, so nothing downstream breaks.

### WR-03: the `--upstream-red=true` render path has no test

**File:** `cmd/coverage-report/main_test.go` (all comment-mode cases pass `--upstream-red=false`)
**Issue:** `render` emits `"Note: an upstream CI job was red; a coverage row may be unavailable."`
only when `d.upstreamRed` is true (`main.go:329-331`). Surfacing the number when an upstream gate
failed is the stated reason the job runs on `!cancelled()`, yet no golden or unit test exercises the
true branch. A regression that drops or malforms that footer line ships silently.
**Fix:** Add a golden case (or extend `TestStatusMark_AtGateBoundary`) with `--upstream-red=true`
asserting the note line is present, plus one with `false` asserting it is absent.

### WR-04: `backend-profile-malformed.txt` fixture is committed but unreferenced; the header-error path is untested

**File:** `cmd/coverage-report/testdata/backend-profile-malformed.txt` (no reference in `main_test.go`)
**Issue:** The fixture (content present, no `mode:` header) is dead — no test loads it. As a result
`backendTotalPct`'s `"first line %q is not a coverage mode header"` branch (`main.go:175-177`) and
comment-mode's degradation on a *present-but-unparseable* profile (distinct from the missing-file
case covered by `TestRenderComment_MissingProfile`) are both uncovered.
**Fix:** Add a test that runs comment mode with `--profile=testdata/backend-profile-malformed.txt`
and asserts the Backend row degrades to `unavailable` and `run` still returns `nil`; and a
`backendTotalPct` unit test asserting the header error.

### WR-05: mixed baseline availability renders an unexplained em-dash delta

**File:** `cmd/coverage-report/main.go:324-328`, `426-427`
**Issue:** `noBaseline` is `!bPresent && !fPresent`. When exactly one sidecar is present (e.g.
backend baseline cached, frontend evicted), `noBaseline` is false, so the "Delta not available yet —
no main baseline cached" footer is suppressed, yet the row without a baseline still renders `—` in
the Δ column with no explanation anywhere in the comment. A reviewer sees a bare em-dash and cannot
tell it apart from "unavailable".
**Fix:** Track baseline presence per row and either emit a per-area note, or fall back to the
no-baseline footer whenever *any* row lacks a baseline while showing which area is affected.

### WR-06: `round2` rounds negative deltas toward positive infinity, understating coverage drops

**File:** `cmd/coverage-report/main.go:152`, used by `formatDelta` at `main.go:357`
**Issue:** `math.Floor(x*100+0.5)/100` rounds half toward +∞, not half away from zero. A 0.125pp
coverage *drop* displays as `-0.12pp` (magnitude rounded down) while a 0.125pp *gain* displays as
`+0.13pp`. For a coverage-diff comment, understating regressions is the wrong direction to bias.
**Fix:** Round the magnitude and reapply the sign, or use `math.Round` (half away from zero):
```go
func round2(x float64) float64 { return math.Round(x*100) / 100 }
```
Re-check the `TestDelta` "rounds half up to two dp" case (0.005 → `+0.01pp`) still holds — it does
with `math.Round`.

## Info

### IN-01: `.golangci.yml` comment cites the wrong linter version

**File:** `.golangci.yml:4-11`
**Issue:** The header says the CI action and the pre-commit hook are "pinned to the same v2.12.2 CLI
version" / "identical v2.12.2 rev", but `.github/workflows/full-pipeline.yml:41` pins `v2.13.2` and
`.pre-commit-config.yaml:22` pins `v2.13.2`. The comment is stale in a file edited this phase.
**Fix:** Update both `v2.12.2` mentions to `v2.13.2`.

### IN-02: `run` shadows the imported `io/fs` package

**File:** `cmd/coverage-report/main.go:48`
**Issue:** `fs := flag.NewFlagSet(...)` shadows the `io/fs` import (used as `fs.ErrNotExist` in
`readSidecar`). Harmless today but a trap if `run` later needs `fs.ErrNotExist`.
**Fix:** Rename the local to `flags` or `fset`.

### IN-03: `sidecar.GeneratedAt` is decoded but never read

**File:** `cmd/coverage-report/main.go:264`
**Issue:** `readSidecar` populates `GeneratedAt` but nothing consumes it; `render` uses only `Pct`
and `SHA`.
**Fix:** Drop the field, or use it (e.g. show baseline age in the footer).

### IN-04: `run`'s `stdout` parameter is ignored by comment and sidecar modes

**File:** `cmd/coverage-report/main.go:47`, `390-443`
**Issue:** Only `runTotal` writes to the injected `stdout`. Comment/sidecar diagnostics go straight
to `os.Stderr`/`os.Stdout`/files, so tests cannot capture the diagnostic lines
(`"backend coverage unavailable: ..."` etc.) without OS-level redirection.
**Fix:** Thread a `stderr io.Writer` (or a small logger) through `commentParams` for the diagnostic
sink.

### IN-05: `coverage-comment` restores profile blobs it never uses

**File:** `.github/workflows/full-pipeline.yml:214-231`
**Issue:** The restore steps pull `coverage.out` and `web/coverage/coverage-summary.json` from the
baseline cache, but the render step reads only the sidecars (`baseline-metrics-*.json`) and the
freshly downloaded PR profiles (`pr-backend/`, `pr-frontend/`). The restored profiles are dead
payload — extra cache size and a misleading signal that they matter.
**Fix:** Drop `coverage.out` / `coverage-summary.json` from both the save (`test` and `frontend-test`
jobs) and restore path lists, keeping only the sidecar files.

---

_Reviewed: 2026-09-02_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
