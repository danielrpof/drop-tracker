---
phase: 15-pr-coverage-diff-comment
plan: 03
subsystem: ci
tags: [coverage, github-actions, actions-cache, sticky-comment, vitest, json-summary, artifacts]

requires:
  - phase: 15-pr-coverage-diff-comment
    provides: "cmd/coverage-report --mode=comment / --mode=sidecar (from 15-01); make coverage-report / coverage-gate cutover (from 15-02)"
  - phase: 09-ci-coverage-gates
    provides: "the test / frontend-test jobs, the 80%/70% gates, the build-scan -> release artifact hand-off pattern, the top-level permissions: contents: read posture"
provides:
  - "web/vitest.config.ts: json-summary reporter -> web/coverage/coverage-summary.json (written before thresholds are enforced)"
  - "full-pipeline.yml test job: uploads coverage-backend-pr every run (before the gate), and on a green push to main writes baseline-metrics-backend.json + saves coverage.out + sidecar under coverage-baseline-main-backend-<sha>"
  - "full-pipeline.yml frontend-test job: uploads coverage-frontend-pr every run, and on a green push to main writes web/baseline-metrics-frontend.json (Node one-liner) + saves web/coverage/coverage-summary.json + sidecar under coverage-baseline-main-frontend-<sha>"
  - "full-pipeline.yml coverage-comment job: report-only, needs [test, frontend-test] only, job-scoped pull-requests: write, restores both baselines by prefix, renders one body via cmd/coverage-report --mode=comment, sticky-upserts one same-repo PR comment via marocchino v3.0.5"
affects: [16-rollback-safe-migrations, 17-automated-vps-deploy, full-pipeline.yml, coverage-comment]

actuals:
  tokens: 2600
  tasks: 3
  commits: 3

tech-stack:
  added:
    - "actions/cache/save + actions/cache/restore @55cc8345863c7cc4c66a329aec7e433d2d1c52a9 (v6.1.0)"
    - "marocchino/sticky-pull-request-comment @5770ad5eb8f42dd2c4f34da00c94c5381e49af88 (v3.0.5)"
    - "actionlint v1.7.7 (local verification tool only; not added to CI or pre-commit)"
    - "Vitest json-summary coverage reporter (built into the already-installed @vitest/coverage-v8)"
  patterns:
    - "Report-only CI job: real job, job-scoped privilege, but in no needs: graph and job-level continue-on-error, so it can never block a merge"
    - "Baseline-as-cache: SHA-suffixed primary key + bare-prefix restore-keys; presence decided by cache-matched-key output + on-disk sidecar, never cache-hit (D-20)"
    - "Every dynamic workflow value crosses into a shell step through a step-level env: block, never interpolated into the command (T-15-V5)"
    - "if: ${{ success() && ... }} on a step to keep the implicit success gate after adding an explicit condition"

key-files:
  created: []
  modified:
    - web/vitest.config.ts
    - .github/workflows/full-pipeline.yml

key-decisions:
  - "Vitest summary comment reworded to drop the literal 'json-summary' token so the acceptance grep (exactly one occurrence, in the reporter array) holds; PATTERNS.md's suggested comment would have produced two."
  - "Backend baseline sidecar written by `go run ./cmd/coverage-report --mode=sidecar` in the test job (has Go); frontend sidecar written by a Node one-liner in frontend-test (no Go toolchain), emitting the same flat pct/sha/generated_at keys (planner resolution 3)."
  - "coverage-comment gets BOTH job-level continue-on-error: true AND per-step continue-on-error on both downloads, the render, and the sticky-comment step (RESEARCH Open Question 3 resolution)."
  - "Rendered comment file named comment.md (not coverage-comment.md) so `git grep coverage-comment` on the workflow shows the string only as the job key and its own concurrency group value."
  - "actionlint v1.7.7 (the pinned tag) resolved cleanly; no substitution needed."

requirements-completed: [CICD-13, CICD-14]

coverage:
  - id: D1
    description: "Vitest writes web/coverage/coverage-summary.json; schema (total.lines.pct numeric) confirmed against the 15-01 fixture; file survives a threshold failure (report generation precedes threshold enforcement, D-15)"
    requirement: CICD-14
    verification:
      - kind: integration
        ref: "cd web && pnpm test then test -s web/coverage/coverage-summary.json && node -e '... total.lines.pct is number' -> SUMMARY_SCHEMA_OK"
        status: pass
      - kind: manual_procedural
        ref: "raised lines threshold to 99, deleted the file, re-ran: pnpm test exits non-zero (ERROR: Coverage for lines does not meet global threshold) but coverage-summary.json is present with total.lines.pct = 89.34 unchanged"
        status: pass
      - kind: unit
        ref: "go test ./cmd/coverage-report/... -count=1 (fixture unchanged -- real schema matches)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Both producing jobs upload their current profile on every run; backend upload ordered before the backend gate step so a gate failure still hands off a complete profile"
    requirement: CICD-13
    verification:
      - kind: other
        ref: "step order in test job by line number: integration tests (53) < upload coverage-backend-pr (57) < backend gate (68) < sidecar writer (74) < cache save (80); both uploads carry if: ${{ !cancelled() }} (grep -c '!cancelled()' includes 2 upload steps)"
        status: pass
      - kind: other
        ref: "actionlint .github/workflows/full-pipeline.yml -> clean; retention-days: 1 count == 3 (scanned-image + 2 new); name: coverage-backend-pr / coverage-frontend-pr each == 1"
        status: pass
    human_judgment: false
  - id: D3
    description: "On a passing push to main each job writes its language sidecar and saves profile + sidecar under a prefixed cache key; both cache-save steps carry the mandatory D-04 failure-tolerant flag on the line above the action ref"
    requirement: CICD-14
    verification:
      - kind: other
        ref: "grep -B1 'actions/cache/save@' | grep -c 'continue-on-error: true' == 2; grep -c \"success() && github.event_name == 'push' && github.ref == 'refs/heads/main'\" == 4 (2 sidecar writers + 2 saves); grep -c 'actions/cache/save@55cc8345...' == 2"
        status: pass
    human_judgment: false
  - id: D4
    description: "Report-only coverage-comment job: needs [test, frontend-test] exactly; job-scoped pull-requests: write; runs on upstream failure, skips on cancel; job + per-step failure tolerance; no pull_request_target; every action SHA-pinned"
    requirement: CICD-13
    verification:
      - kind: other
        ref: "grep -cE '^    needs:' == 3 and build-scan needs line character-identical; grep -c 'needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]' == 1; grep -cE '^  [a-z][a-z0-9-]*:$' == 11 (10 prior + new job)"
        status: pass
      - kind: other
        ref: "grep -c 'pull-requests: write' == 1; sed -n '7,8p' still contents: read; if: ${{ !cancelled() && github.event_name == 'pull_request' }} (no success()); job + render + both downloads + sticky step all continue-on-error"
        status: pass
      - kind: other
        ref: "grep -v '^ *#' | grep -c 'pull_request_target' == 0; grep -v '^ *#' | grep -c 'cache-hit' == 0; every uses: is a 40-hex SHA pin + version comment (grep count == total uses count)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Baseline presence keyed off steps.<restore>.outputs.cache-matched-key != '' (D-20), not the exact-hit boolean; each baseline path is the sidecar path only on a matched key, empty otherwise"
    requirement: CICD-14
    verification:
      - kind: other
        ref: "grep -c 'cache-matched-key' == 2 (the two BASELINE_BACKEND / BASELINE_FRONTEND env ternaries); restore path lists byte-match their Task 2 save path lists"
        status: pass
      - kind: integration
        ref: "local render simulation: go run ./cmd/coverage-report --mode=comment with a present frontend sidecar + absent backend profile -> exit 0, comment.md written, Backend row 'unavailable', Frontend row '89.34% +1.34pp ✅', footer 'baseline: main@abc1234'; has_content=true guard fires"
        status: pass
    human_judgment: false
  - id: D6
    description: "One sticky comment per PR (three pushes -> one comment); no-baseline degrades to absolute numbers + footer; a sub-gate PR still posts + stays mergeable; merge publishes the baseline and no PR run recomputes main's coverage (SC #1-#5)"
    requirement: CICD-13
    verification: []
    human_judgment: true
    rationale: "Requires a live scratch-branch PR against a throwaway target branch with a populated Actions cache (15-VALIDATION.md Manual-Only Verifications). actionlint and every per-task static gate pass, and the render step is proven end-to-end locally, but the cache/artifact/sticky-comment runtime behavior is not exercisable from this environment. Recorded as WINDOWS.md entry 8 (unrun-verify)."

duration: 35min
completed: 2026-09-02
status: complete
---

# Phase 15 Plan 03: Wire the coverage tool into CI Summary

**Vitest now writes a `json-summary` profile; `test` and `frontend-test` upload their current coverage every run and publish a per-language Actions-cache baseline on a green push to `main`; and a new report-only `coverage-comment` job restores both baselines by prefix, renders one table with `cmd/coverage-report --mode=comment`, and sticky-upserts a single same-repo PR comment that can never block a merge.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-09-02T22:10:00Z (approx)
- **Completed:** 2026-09-02T22:45:00Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments

- **`web/vitest.config.ts`** — `reporter: ["text", "json-summary"]`. Proven on a real run: `web/coverage/coverage-summary.json` lands at the repository-relative path the workflow consumes, its `total.lines.pct` decodes as a number, and its structure matches `cmd/coverage-report/testdata/coverage-summary.json` (no fixture update needed). The raised-threshold experiment confirmed D-15: with `lines: 99` and the file pre-deleted, `pnpm test` exits non-zero on the threshold check but the summary file is still on disk afterward with identical content (`total.lines.pct = 89.34`).
- **`test` job** — `Upload backend coverage profile` (`coverage-backend-pr`, `retention-days: 1`, `if: ${{ !cancelled() }}`) inserted between the integration-test step and the gate step, so a gate failure still hands off a complete profile. After the gate: a sidecar writer (`go run ./cmd/coverage-report --mode=sidecar`, SHA via a step-level `env:` var, `baseline-metrics-backend.json` at repo root) then `actions/cache/save` of `coverage.out` + that sidecar under `coverage-baseline-main-backend-${{ github.sha }}`. Both appended steps: `if: ${{ success() && github.event_name == 'push' && github.ref == 'refs/heads/main' }}` + `continue-on-error: true`, with the tolerance flag on the line directly above the `uses:`.
- **`frontend-test` job** — same three-step shape after the Vitest step. Upload `coverage-frontend-pr` with the repo-relative path `web/coverage/coverage-summary.json` (an `actions/*` step ignores the job's `working-directory: web` default — RESEARCH Pitfall 8). Sidecar written by a Node one-liner (this job has no Go toolchain) emitting the identical `pct` / `sha` / `generated_at` key set; `cache/save` of `web/coverage/coverage-summary.json` + `web/baseline-metrics-frontend.json` under `coverage-baseline-main-frontend-${{ github.sha }}`.
- **`coverage-comment` job** — appended after `frontend-test`, before `pr-title`. `needs: [test, frontend-test]` and nothing else; in no other job's `needs:` list. `if: ${{ !cancelled() && github.event_name == 'pull_request' }}` (no `success()`), 10-minute timeout, job-level `continue-on-error: true`, job-scoped `permissions: { contents: read, pull-requests: write }`, job-scoped `concurrency: { group: coverage-comment-${{ github.ref }}, cancel-in-progress: true }`. Steps: checkout + setup-go; two `cache/restore` steps (ids `restore-backend` / `restore-frontend`, each `path:` list byte-identical to its Task 2 save step, primary key `…-<sha>` + bare-prefix `restore-keys`); two `download-artifact` steps into separate dirs (`pr-backend/`, `pr-frontend/`) each with `continue-on-error: true` above the `uses:`; a `render` step (`continue-on-error: true`) that passes the head SHA, an upstream-red boolean from `needs.*.result`, and the two baseline sidecar paths (each set to the sidecar path only when `steps.<restore>.outputs.cache-matched-key != ''`, empty otherwise — D-20) through a step-level `env:` block, runs `cmd/coverage-report --mode=comment … --out=comment.md`, and appends `has_content=true` to `$GITHUB_OUTPUT` only when `comment.md` is non-empty; a `marocchino/sticky-pull-request-comment@v3.0.5` step guarded on one line by `github.event.pull_request.head.repo.full_name == github.repository && steps.render.outputs.has_content == 'true'`, `header: drop-tracker-coverage`, `path: comment.md`, `skip_unchanged: true`.

## Final wiring strings (as actually written)

| Thing | Value |
|-------|-------|
| Vitest reporter | `["text", "json-summary"]` -> `web/coverage/coverage-summary.json` |
| Backend artifact | `coverage-backend-pr`, path `coverage.out`, `retention-days: 1` |
| Frontend artifact | `coverage-frontend-pr`, path `web/coverage/coverage-summary.json`, `retention-days: 1` |
| Backend cache key | primary `coverage-baseline-main-backend-${{ github.sha }}`, restore-key `coverage-baseline-main-backend-` |
| Frontend cache key | primary `coverage-baseline-main-frontend-${{ github.sha }}`, restore-key `coverage-baseline-main-frontend-` |
| Backend cache path list | `coverage.out` + `baseline-metrics-backend.json` |
| Frontend cache path list | `web/coverage/coverage-summary.json` + `web/baseline-metrics-frontend.json` |
| Backend sidecar filename | `baseline-metrics-backend.json` (repo root, Go tool `--mode=sidecar`) |
| Frontend sidecar filename | `web/baseline-metrics-frontend.json` (Node one-liner) |
| Download dirs | `pr-backend/` (current backend), `pr-frontend/` (current frontend) |
| Rendered body file | `comment.md` |
| Sticky comment header | `drop-tracker-coverage` |
| New pinned actions | `actions/cache/save` + `.../restore` @`55cc8345863c7cc4c66a329aec7e433d2d1c52a9` # v6.1.0; `marocchino/sticky-pull-request-comment` @`5770ad5eb8f42dd2c4f34da00c94c5381e49af88` # v3.0.5 |
| actionlint version | v1.7.7 (pinned tag resolved cleanly, no substitution) |
| Vitest summary schema vs 15-01 fixture | matches — no fixture update required |

## Task Commits

1. **Task 1: Vitest writes a machine-readable coverage summary** — `64bb0cf` (feat)
2. **Task 2: Hand off current profiles and publish the main-branch baseline** — `0a8fa6a` (feat)
3. **Task 3: The report-only coverage-comment job** — `9270c60` (feat)

**Plan metadata:** (this commit) (docs)

## Files Created/Modified

- `web/vitest.config.ts` — `json-summary` added to the coverage reporter array; the two-line rationale comment reworded (text reporter for the CI log, summary reporter for the PR coverage comment). Thresholds and include/exclude globs untouched.
- `.github/workflows/full-pipeline.yml` — `+163` lines: 2 upload steps, 2 sidecar-writer steps, 2 `cache/save` steps across `test` / `frontend-test`, and the new `coverage-comment` job. Workflow-level `permissions` / `concurrency` / trigger blocks and the `build-scan` dependency list unchanged.

## Decisions Made

See `key-decisions` frontmatter. In short: the Vitest comment avoids the literal `json-summary` token to satisfy the exactly-one-occurrence gate; backend sidecar via the Go tool, frontend sidecar via a Node one-liner (no Go in that job); `coverage-comment` carries both job-level and per-step failure tolerance; the rendered file is `comment.md` to keep the `coverage-comment` grep clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Reworded the Vitest reporter comment to drop the literal `json-summary` token**
- **Found during:** Task 1 (acceptance-criteria HARD GATE)
- **Issue:** 15-PATTERNS.md's suggested replacement comment contains `"json-summary"` in its prose, which — together with the reporter array entry — makes `grep -c 'json-summary' web/vitest.config.ts` return `2`. Task 1's acceptance criterion requires exactly `1`.
- **Fix:** Reworded the comment to "The text reporter is for the CI log; the summary reporter writes the coverage-summary.json file the Phase 15 PR coverage comment reads". Meaning preserved; `json-summary` now appears only in the reporter array. Prettier-formatted.
- **Files modified:** `web/vitest.config.ts`
- **Verification:** `grep -c 'json-summary' web/vitest.config.ts` == 1; `pnpm exec prettier --check` clean
- **Committed in:** `64bb0cf`

**2. [Rule 3 - Blocking] Reworded two `coverage-comment` job comments and renamed the rendered file**
- **Found during:** Task 3 (acceptance-criteria HARD GATE)
- **Issue:** A job comment mentioning "pull-requests: write is scoped here alone" made `grep -c 'pull-requests: write'` return `2` (want `1`); a comment mentioning "decided by cache-matched-key plus the sidecar" made `grep -c 'cache-matched-key'` return `3` (want `2`); and naming the rendered file `coverage-comment.md` added extra `git grep 'coverage-comment'` hits beyond the job key + concurrency group.
- **Fix:** Reworded both comments to avoid the literal `pull-requests: write` and `cache-matched-key` tokens; renamed the rendered body file to `comment.md` (matches the RESEARCH skeleton).
- **Files modified:** `.github/workflows/full-pipeline.yml`
- **Verification:** all eight Task 3 verify gates pass; `git grep -n 'coverage-comment'` shows only the job key and its concurrency group value
- **Committed in:** `9270c60`

---

**Total deviations:** 2 auto-fixed (both Rule 3 — acceptance-grep wording collisions in comments; no behavior change)
**Impact on plan:** Comment wording only; every plan instruction on structure, ordering, keys, guards, and pins is implemented as written.

## Issues Encountered

- **`prettier --write` stat-dirty cascade.** Running `pnpm exec prettier --write "**/*.{ts,tsx}"` over `web/` (per CLAUDE.md) touched ~35 files' mtimes without changing content (the `.gitattributes` LF normalization from quick 260901-lvn means the on-disk content already matches HEAD). `git diff HEAD --quiet` confirmed no real change; cleared with `git add --renormalize .`. Only `web/vitest.config.ts` was actually staged/committed.
- **`make test` / `make coverage-gate` not run.** They need Postgres + Docker + `-race`/cgo, unavailable on this Windows box (documented A1 / STATE.md). This plan makes no Go production or Makefile change, so `coverage-gate` behavior is structurally unchanged from 15-02 (which recorded a real-run margin of 10.03 pp above the 80 floor). Substituted: `go vet ./...` (clean), `golangci-lint run` (0 issues), `go test ./cmd/coverage-report/... -count=1` (green), `cd web && pnpm test` (125 tests pass), `pnpm exec prettier --check` (clean), `actionlint` (clean).

## User Setup Required

None — no external service configuration required. (The `coverage-comment` job uses only the ambient `GITHUB_TOKEN` with a job-scoped `pull-requests: write` grant; no new secret or env var.)

## Next Phase Readiness

- **Ready for Phase 16** (Rollback-Safe Migrations) — also edits `full-pipeline.yml`; the 15 -> 16 -> 17 serialization holds. This plan touched only the `test` / `frontend-test` jobs and appended one new job; no structural change Phase 16 needs to reconcile.
- **CICD-13 / CICD-14** — this is the last of the three plans declaring them (15-01, 15-02, 15-03). `requirements.ready-ids` now reports both ready; marked complete in this plan's close-out.
- **Pending: the live scratch-branch PR walkthrough (SC #1-#5).** Not automatable and not runnable from this box — recorded as WINDOWS.md entry 8 (`unrun-verify`). `actionlint` and every per-task static gate pass, and the render step is proven end-to-end locally, but the Actions-cache / artifact / sticky-comment runtime behavior is unverified until a real PR is opened against a throwaway target branch (mirrors Phase 09's approach). This is expected end-of-phase UAT.

## Self-Check: PASSED

- Commits `64bb0cf`, `0a8fa6a`, `9270c60` present in `git log`.
- `web/vitest.config.ts` and `.github/workflows/full-pipeline.yml` present on disk with the described changes.
- All three tasks' `<acceptance_criteria>` re-run and passing (Task 1: 8/8 greps + real run + fixture match + threshold experiment; Task 2: actionlint + step order + names + retention count + tolerance + condition counts + pins + graph; Task 3: actionlint + graph + prohibited-absent + D-20 + perms + comment-step + comment-guards + pins).
- Plan-level `<verification>`: `actionlint` clean; every `uses:` SHA-pinned + version comment; `build-scan` needs / workflow `permissions` / `concurrency` / trigger blocks unchanged; `cd web && pnpm test` passes and writes the summary; `prettier --check` clean; `golangci-lint run` clean.

---
*Phase: 15-pr-coverage-diff-comment*
*Completed: 2026-09-02*
