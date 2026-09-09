---
phase: 15-pr-coverage-diff-comment
plan: 01
subsystem: ci
tags: [coverage, go-tooling, markdown-render, golden-tests, github-actions, vitest]

requires:
  - phase: 09-ci-coverage-gates
    provides: the Makefile `coverage-gate` (80% backend) and `web/vitest.config.ts` `coverage.thresholds` (70% frontend) literals this tool mirrors
provides:
  - "cmd/coverage-report — a stdlib-only Go main package with three --mode arms (total, sidecar, comment)"
  - "the exact PR-comment markdown table (## Coverage heading, fixed Backend-then-Frontend rows, 2-dp percentages, signed pp deltas, gate literals, status glyphs, provenance footer)"
  - "--mode=total: the bare 2-decimal backend number make coverage-gate consumes in plan 15-02"
  - "--mode=sidecar: the flat pct/sha/generated_at baseline JSON the publish steps write in plan 15-03"
  - "the .golangci.yml gosec carve-out for cmd/coverage-report (D-19)"
  - "12 committed testdata fixtures incl. four comment-*.golden.md bodies"
affects: [15-02, 15-03, coverage-gate, full-pipeline.yml]

actuals:
  tokens: 9500
  tasks: 3
  commits: 6

tech-stack:
  added: []
  patterns:
    - "Golden-file testing with an -update flag (first golden-file precedent in the repo)"
    - "main/run split with run(args []string, stdout io.Writer) error for whitebox testability (mirrors cmd/server)"
    - "Injectable clock (nowUTC package var) so rendered output is deterministic under test"
    - "Comment mode never returns a non-nil error — every input failure degrades to an unavailable row"

key-files:
  created:
    - cmd/coverage-report/main.go
    - cmd/coverage-report/main_test.go
    - cmd/coverage-report/testdata/backend-profile.txt
    - cmd/coverage-report/testdata/backend-profile-boundary.txt
    - cmd/coverage-report/testdata/backend-profile-malformed.txt
    - cmd/coverage-report/testdata/backend-profile-hostile-paths.txt
    - cmd/coverage-report/testdata/coverage-summary.json
    - cmd/coverage-report/testdata/coverage-summary-boundary.json
    - cmd/coverage-report/testdata/baseline-metrics-backend.json
    - cmd/coverage-report/testdata/baseline-metrics-frontend.json
    - cmd/coverage-report/testdata/comment-normal.golden.md
    - cmd/coverage-report/testdata/comment-no-baseline.golden.md
    - cmd/coverage-report/testdata/comment-unchanged.golden.md
    - cmd/coverage-report/testdata/comment-unavailable.golden.md
  modified:
    - .golangci.yml

key-decisions:
  - "Comment renderer emits ONLY compile-time literals, tool-computed numbers, a tool-generated RFC3339 timestamp, and a 7-40-lowercase-hex-validated short SHA. No file path, profile line, or unvalidated JSON field is interpolated (T-15-03)."
  - "A single validSHA(s) (7-40 lowercase hex) gates every SHA into the body: a bad --sha is a hard error in sidecar mode; a bad --head-sha or sidecar sha field is silently dropped from the footer, run still exits 0 (D-04)."
  - "Sidecar pct is written as a json.RawMessage of the same %.2f string total mode prints, so the two are byte-identical (D-17)."
  - "No-baseline footer reworded to 'Delta not available yet — ...' so it never contains the literal token 'unavailable', keeping it distinct from the per-row unavailable cell string (D-11)."

requirements-completed: [CICD-13, CICD-14]

coverage:
  - id: D1
    description: "Comment renderer: ## Coverage heading, fixed Backend-then-Frontend rows, 2-dp percentages, signed pp deltas, gate literals 80%/70%, status glyph via >= comparison, provenance footer"
    requirement: CICD-13
    verification:
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestRenderComment_Golden"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestRenderComment_GoldenHasFixedShape"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestStatusMark_AtGateBoundary"
        status: pass
    human_judgment: false
  - id: D2
    description: "Backend total = statement-weighted hand-parse of a Go coverage profile (last-colon path split), round-half-up to 2 dp; frontend = total.lines.pct of the Vitest json-summary"
    requirement: CICD-13
    verification:
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestBackendTotalPct"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestFrontendLinesPct"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestParseBlockLine_LastColonSplit"
        status: pass
    human_judgment: false
  - id: D3
    description: "Graceful degradation: missing/empty/malformed profile renders a single unavailable row (em-dash delta and status) while the other row keeps its real number; comment mode always exits 0"
    requirement: CICD-14
    verification:
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestRenderComment_MissingProfile"
        status: pass
    human_judgment: false
  - id: D4
    description: "No-baseline state (empty flag or missing file): both delta cells em-dash, one-line footer, provenance line omitted; unchanged value renders ±0.00pp, provably distinct from the em-dash"
    requirement: CICD-14
    verification:
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestRenderComment_NoBaseline"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestRenderComment_Unchanged"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestDelta"
        status: pass
    human_judgment: false
  - id: D5
    description: "--mode=total prints only the bare 2-dp number to stdout, errors with empty stdout on a bad profile; --mode=sidecar writes a flat pct/sha/generated_at JSON whose pct is byte-identical to total's output"
    requirement: CICD-14
    verification:
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestModeTotal_PrintsOnlyNumber"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestModeTotal_MissingProfile"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestModeSidecar_Roundtrip"
        status: pass
    human_judgment: false
  - id: D6
    description: "Input hardening (T-15-03 V5): a coverage file cannot carry text into the comment body; commit-SHA format validation (7-40 lowercase hex) is the sole gate"
    requirement: CICD-13
    verification:
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestRenderComment_NoUntrustedInterpolation"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestSHAValidation"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestSidecar_RejectsBadSHA"
        status: pass
      - kind: unit
        ref: "cmd/coverage-report/main_test.go#TestModeComment_RejectsBadHeadSHA"
        status: pass
    human_judgment: false
  - id: D7
    description: ".golangci.yml gosec carve-out for ^cmd/coverage-report/ (D-19) so the package's coverage-file reads do not redden the lint job on G304"
    requirement: CICD-13
    verification:
      - kind: other
        ref: "golangci-lint run (exits 0, no G304 attributed to cmd/coverage-report/main.go)"
        status: pass
    human_judgment: false
  - id: D8
    description: "The rendered table format, footer variants, CLI flags, and sidecar key set are frozen for plans 15-02 and 15-03 to wire against"
    verification: []
    human_judgment: true
    rationale: "Downstream consumption by the Makefile refactor (15-02) and the CI workflow job (15-03) is not exercised by any test in this plan — it is verified when those plans wire against the strings recorded below."

# Metrics
duration: ~8 min (this session only)
completed: 2026-09-02
status: complete
---

# Phase 15 Plan 01: cmd/coverage-report Go tool Summary

**A stdlib-only Go tool that hand-parses a Go coverage profile and a Vitest json-summary, reads baseline sidecars, and renders the single never-red PR coverage table — plus `--mode=total` for the gate and `--mode=sidecar` for the baseline publish.**

## Performance

- **Duration:** ~8 min (this session); the plan ran across two sessions after a prior quota interruption
- **Started (this session):** 2026-09-02T22:07:06Z
- **Completed:** 2026-09-02T22:16:00Z
- **Tasks:** 3 (Task 1 completed in the prior session; Tasks 2 and 3 this session)
- **Files created:** 14 (2 Go, 12 testdata); 1 modified (`.golangci.yml`)

## Two-session execution note

Task 1 (the end-to-end tracer) was implemented and committed as `a9c497c` in a prior
executor session that then hit a usage limit. This session verified `a9c497c` (build, vet,
golangci-lint, `TestRenderComment_Golden`) before starting, then executed Tasks 2 and 3.
The original start time is unrecoverable; the duration above covers this session only.

## Accomplishments

- **Comment renderer** — `## Coverage` heading, a fixed two-row table (Backend then Frontend, literal sequence, never map-derived), 2-decimal percentages, signed `pp` deltas computed as the difference of the two rounded printed numbers, `80%`/`70%` gate literals, `✅`/`⚠️` status chosen with a `>=` comparison against the gate constant, and a footer carrying the head short-SHA, `baseline: main@<short>` provenance, and a UTC timestamp. Also appended to `$GITHUB_STEP_SUMMARY` when set.
- **Full degradation contract** — a missing / empty / unparseable profile renders that row as `unavailable` with em-dash delta and status while the other row keeps its real number; comment mode returns `nil` for every input combination, so the process always exits 0 (D-04, D-18). No-baseline → both deltas em-dash, a one-line footer, no provenance line. Unchanged value → `±0.00pp`, a provably different string from the no-baseline em-dash.
- **`--mode=total`** — writes only the bare `%.2f` backend number and a newline to the `stdout` writer; a bad profile is a hard error with empty stdout (the condition `make coverage-gate` depends on in 15-02).
- **`--mode=sidecar`** — writes `{"pct": N.NN, "sha": "...", "generated_at": "<RFC3339 UTC>"}`; `pct` is a `json.RawMessage` of the same `%.2f` string total mode prints, so the two are byte-identical.
- **Input hardening (T-15-03)** — a single `validSHA` (7–40 lowercase hex) is the only path a SHA string reaches the body. `TestRenderComment_NoUntrustedInterpolation` renders against a fixture whose file-path fields carry `<script>`, backticks, `*md*`, `|`, and `[md](http://evil)` and asserts (deriving the forbidden strings from the fixture at runtime) that none reach the body, while the backend percentage is still correct.
- **`.golangci.yml`** — scoped `^cmd/coverage-report/` → `gosec` carve-out (D-19), landed with Task 1.

## Rendered comment format (frozen for 15-02 / 15-03)

Body, normal case (`testdata/comment-normal.golden.md`):

```
## Coverage

| Area | Coverage | Δ vs main | Gate | Status |
| --- | --- | --- | --- | --- |
| Backend | 86.84% | +0.53pp | 80% | ✅ |
| Frontend | 72.30% | -1.20pp | 70% | ✅ |

head abc1234 · 2026-09-02T12:00:00Z
baseline: main@def5678
```

Footer lines, in order:
1. `head <short-sha> · <RFC3339 UTC>` — or just `<RFC3339 UTC>` when `--head-sha` is absent/invalid.
2. `baseline: main@<short-sha>` — only when at least one sidecar was read and carried a valid SHA.
3. `Delta not available yet — no main baseline cached (first run or evicted). Absolute coverage shown.` — only when no sidecar was read at all (mutually exclusive with line 2).
4. `Note: an upstream CI job was red; a coverage row may be unavailable.` — only when `--upstream-red=true`.

Degraded row: `| Backend | unavailable | — | 80% | — |` (label and gate stay literal; coverage/delta/status collapse).

## CLI surface

| Flag | Modes | Default | Notes |
|------|-------|---------|-------|
| `--mode` | all | `""` | `total` \| `sidecar` \| `comment`; anything else is an error |
| `--profile` | all | `""` | path to the Go coverage profile (`coverage.out`) |
| `--frontend-summary` | comment | `""` | path to the Vitest `coverage-summary.json` |
| `--baseline-backend` | comment | `""` | backend sidecar path; empty or missing file = no baseline (not an error) |
| `--baseline-frontend` | comment | `""` | frontend sidecar path; same semantics |
| `--head-sha` | comment | `""` | PR head SHA; rejected (not 7–40 lc hex) → omitted from the footer |
| `--upstream-red` | comment | `false` | adds the upstream-red footer line when true |
| `--out` | comment, sidecar | `""` | output file path; a comment-mode write failure goes to stderr only |
| `--sha` | sidecar | `""` | SHA to stamp; rejected → hard error, file not written |
| `-update` | test only | `false` | rewrites the golden files |

## Sidecar JSON (frozen for 15-03)

Flat object, exactly three keys, one decoder (`readSidecar`) reads both language sidecars:

```json
{"pct": 86.84, "sha": "def5678def5678def5678def5678def5678def56", "generated_at": "2026-08-30T04:15:00Z"}
```

- `pct` — JSON number, always 2 decimals, identical to `--mode=total` output for the same profile.
- `sha` — string; consumers must format-validate before display (the tool does).
- `generated_at` — RFC3339 UTC.

## Task Commits

1. **Task 1: End-to-end tracer** — `a9c497c` (feat) — *prior session*
2. **Task 2 RED: degradation and boundary tests** — `8d7c601` (test)
3. **Task 2 GREEN: golden coverage for degraded/boundary rows** — `195ac7c` (feat)
4. **Task 3 RED: total/sidecar mode and hardening tests** — `332deaa` (test)
5. **Task 3 GREEN: total and sidecar modes with SHA validation** — `20e6d68` (feat)

_Tasks 2 and 3 are `tdd="true"`: RED (`test(15-01)`) then GREEN (`feat(15-01)`), no REFACTOR needed._

## Decisions Made

See `key-decisions` frontmatter. In short: the renderer is literals + numbers + one validated SHA only; `validSHA` is the single SHA gate; sidecar `pct` is byte-identical to total output; the no-baseline footer avoids the token "unavailable".

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected two baseline sidecar fixtures with 41-character SHA fields**
- **Found during:** Task 3 (GREEN — wiring `validSHA` into `shortSHA`)
- **Issue:** `testdata/baseline-metrics-backend.json` and `-frontend.json` (authored in Task 1) carried a 41-character `sha` value — one over a real 40-char git SHA. The new `validSHA` (7–40 lowercase hex) correctly rejected them, which dropped the `baseline: main@…` provenance line and regressed `TestRenderComment_Golden` (Task 1) and `TestRenderComment_MissingProfile` (Task 2).
- **Fix:** Set both `sha` fields to a valid 40-char value (`def5678…def56`). Short-SHA is still `def5678`, so no golden file changed.
- **Files modified:** `cmd/coverage-report/testdata/baseline-metrics-backend.json`, `cmd/coverage-report/testdata/baseline-metrics-frontend.json`
- **Verification:** full `go test ./cmd/coverage-report/... -count=1` green after the fix
- **Committed in:** `20e6d68`

**2. [Rule 1 - Bug] Reworded the no-baseline footer to drop the literal "unavailable"**
- **Found during:** Task 2 (GREEN — acceptance-criteria HARD GATE)
- **Issue:** Task 1's footer text "Delta unavailable — …" collided with the per-row `unavailable` cell string; Task 2's own verify expects exactly two `unavailable` occurrences in a fully-degraded no-baseline body, but the footer made it three.
- **Fix:** Reworded to "Delta not available yet — no main baseline cached (first run or evicted). Absolute coverage shown." D-11 specifies the wording only "roughly"; meaning is preserved.
- **Files modified:** `cmd/coverage-report/main.go` (+ regenerated `comment-no-baseline.golden.md`)
- **Verification:** `grep -c 'unavailable'` on the degraded no-baseline body returns 2; Task 2 tests green
- **Committed in:** `195ac7c`

**3. [Rule 3 - Blocking] Two extra hardening tests beyond the plan's named set**
- **Found during:** Task 3
- **Issue:** The plan names `TestModeTotal_*`, `TestModeSidecar_Roundtrip`, `TestSHAValidation`, `TestRenderComment_NoUntrustedInterpolation`, and describes (in `<behavior>`) a rejected `--sha` and a rejected `--head-sha` with no dedicated test name.
- **Fix:** Added `TestSidecar_RejectsBadSHA` and `TestModeComment_RejectsBadHeadSHA` to cover those two behaviors explicitly.
- **Files modified:** `cmd/coverage-report/main_test.go`
- **Committed in:** `332deaa` / `20e6d68`

---

**Total deviations:** 3 (2 Rule 1 bug fixes, 1 Rule 3 test addition)
**Impact on plan:** All within `cmd/coverage-report` and its fixtures; no scope creep, no production consumer touched. The two Task-1 fixture bugs surfaced only because Task 3 tightened SHA handling exactly as the plan intended.

## Task 2 GREEN note — no new production code

Task 1's tracer already implemented the full render/degradation path end-to-end (that is what a
tracer does). Task 2's GREEN commit therefore adds the three golden bodies and one footer
reword; `TestDelta` and `TestStatusMark_AtGateBoundary` pass against the tracer implementation
and stand as regression guards. This is consistent with `references/tdd.md` ("feature may
already exist — investigate").

## Issues Encountered

None beyond the deviations above.

## Verification

- `go build ./... && go vet ./... && golangci-lint run` — all exit 0
- `go test ./cmd/coverage-report/... -count=1` — 17 test functions pass
- `git diff --stat -- go.mod go.sum` — no change (stdlib only, D-06)
- every file under `cmd/coverage-report/testdata/` is git-trackable (`git check-ignore` exits non-zero for each)
- `go list -deps ./cmd/coverage-report | grep -c '^golang\.org/x'` → `0`
- `--mode=total` against a valid profile prints one line matching `^[0-9]+\.[0-9]{2}$`; against a missing profile exits non-zero with zero bytes on stdout
- **Not run this plan:** `make test` / `make coverage-gate` — no DB or cross-package change here, and this Windows box cannot run the `-race` integration suite (documented A1 / STATE.md). The new package has no in-repo consumer and is not yet excluded from `COVER_PKGS` (that is plan 15-02), so `make coverage-gate` behavior is unchanged by this plan.

## Next Phase Readiness

- **Ready for 15-02** (Makefile: `COVER_PKGS` exclusion, `make coverage-report`, D-17 `coverage-gate` cutover). 15-02 must still run `make test-integration` on a real machine and compare `go tool cover -func` vs `go run ./cmd/coverage-report --mode=total` before the cutover (RESEARCH Pitfall 7 / A1).
- **Ready for 15-03** (CI wiring). The rendered table strings, footer variants, CLI flags, and sidecar key set are frozen above.
- `CICD-13` / `CICD-14` remain **Pending** in REQUIREMENTS.md — both are also declared by 15-02 and 15-03; `requirements.ready-ids` keeps them blocked until those plans finish. This is expected.

## Self-Check: PASSED

---
*Phase: 15-pr-coverage-diff-comment*
*Completed: 2026-09-02*
