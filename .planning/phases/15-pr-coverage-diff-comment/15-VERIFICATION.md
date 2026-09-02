---
phase: 15-pr-coverage-diff-comment
verified: 2026-09-02T23:15:00Z
status: human_needed
score: 2/5 must-haves verified
behavior_unverified: 3
overrides_applied: 0
requirements_verified: [CICD-13, CICD-14]
behavior_unverified_items:
  - truth: "SC #1 — opening a same-repo PR produces exactly one comment with backend + frontend totals plus deltas vs the main baseline"
    test: "Open a PR from a scratch branch in this repo. Wait for the coverage-comment job to finish."
    expected: "Exactly one PR comment appears, carrying the hidden header drop-tracker-coverage, a ## Coverage table with a Backend row and a Frontend row, each showing a 2-decimal percentage and a signed pp delta (or an em-dash if no baseline yet)."
    why_human: "The sticky-comment upsert, the artifact hand-off, and the cache restore only execute inside a real GitHub Actions PR run — grep/unit tests cannot observe the GitHub comment API result."
  - truth: "SC #2 — pushing further commits edits the same comment in place (three pushes leave one comment)"
    test: "On the same scratch-branch PR, push two more commits. Re-check the PR conversation."
    expected: "Still exactly one coverage comment, edited in place; no duplicate comments; a no-op re-push does not emit an edit notification (skip_unchanged)."
    why_human: "Idempotent upsert keyed on the hidden header is a runtime property of marocchino/sticky-pull-request-comment plus the job concurrency group; not observable statically."
  - truth: "SC #5 — a merge to main publishes that run's coverage as the baseline the next PR diffs against, and no PR run recomputes main's coverage"
    test: "Merge the scratch PR into a throwaway target branch that runs the pipeline as 'main-like' (or merge to main). Confirm the test/frontend-test jobs run the save-baseline steps. Then open a fresh PR and confirm its comment shows a real delta with a 'baseline: main@<sha>' provenance line."
    expected: "On the merge run, 'Write backend/frontend baseline sidecar' + 'Save ... coverage baseline' steps execute (they are gated success() && push && refs/heads/main). The next PR's comment shows a numeric delta and the provenance footer. No PR-triggered job runs the backend/frontend suite against main's tree."
    why_human: "Actions cache save/restore across separate workflow runs is not exercisable locally; requires two sequential real runs."
human_verification:
  - test: "Open a same-repo scratch-branch PR; confirm one coverage comment with both rows and deltas (SC #1)."
    expected: "Single comment, header drop-tracker-coverage, Backend + Frontend rows with 2dp % and signed pp delta."
    why_human: "Requires a live GitHub Actions PR run — sticky-comment API + artifact + cache paths."
  - test: "Push two more commits to that PR; confirm still one comment, edited in place (SC #2)."
    expected: "One comment, not three."
    why_human: "Runtime idempotency of the sticky upsert."
  - test: "Run before any baseline cache exists (or delete the cache entry); confirm the comment posts with absolute numbers and the 'Delta not available yet' footer, no error, no nonsense delta (SC #3 — CI-path confirmation; tool-level behavior already unit-tested and locally reproduced)."
    expected: "Comment present, em-dash deltas, delta-unavailable footer, job green."
    why_human: "Confirms the cache-matched-key='' -> empty baseline path wiring end to end."
  - test: "Push a change dropping backend below 80% or a Vitest axis below 70%; confirm the comment posts the real number with the warning glyph, the producing job goes red, and the PR stays mergeable with no new required check (SC #4 — merge-button confirmation; never-blocker property already structurally verified)."
    expected: "Comment shows the drop + warning glyph; coverage-comment job stays green/neutral; PR mergeable."
    why_human: "Final confirmation the branch-protection required-checks set does not include coverage-comment."
  - test: "Merge to a throwaway target branch, then open a new PR; confirm delta is measured against the merged run and no PR job re-ran main's suite (SC #5)."
    expected: "New PR comment shows a numeric delta + 'baseline: main@<sha>'; no PR job runs the full suite against main."
    why_human: "Cross-run Actions cache behavior."
---

# Phase 15: PR Coverage-Diff Comment Verification Report

**Phase Goal:** Every pull request from a same-repo branch carries a single, always-current comment showing what it does to backend and frontend coverage relative to main — closing the last CI reporting gap without ever becoming a new merge blocker.
**Verified:** 2026-09-02T23:15:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

The phase goal decomposes into two halves:

1. **The report machinery** (compute both coverage numbers once, render one markdown table, publish/consume a baseline, post a sticky PR comment). This half is **built, wired, and statically verified**. The `cmd/coverage-report` tool is a real, stdlib-only, 18-test package; the Makefile cutover routes the 80% gate through the same algorithm; Vitest emits a machine-readable summary (confirmed by a real local run); the `coverage-comment` job is fully assembled and `actionlint`-clean with every action SHA-pinned.

2. **The observable runtime behavior on a live PR** (SC #1, #2, #5). This half is **not exercisable from this environment** and is recorded as WINDOWS.md entry 8 (`unrun-verify`). It routes to human verification, not a gap — consistent with the phase's own `15-VALIDATION.md` "Manual-Only Verifications" and Phase 09's precedent.

The **"never a merge blocker"** constraint (the riskiest part of the goal) IS statically verifiable and passes: `coverage-comment` carries job-level `continue-on-error: true`, has `needs: [test, frontend-test]` and nothing else, and appears in no other job's `needs:` list (`build-scan` needs unchanged, `release` needs `[build-scan]`).

### Observable Truths

| # | Truth (ROADMAP Success Criteria) | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Opening a same-repo PR produces exactly one comment with backend + frontend totals plus deltas vs the main baseline | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Renderer produces the exact table (golden tests pass); `marocchino/sticky-pull-request-comment@v3.0.5` pinned, header `drop-tracker-coverage`, `path: comment.md`. Artifact + cache + comment-API runtime not exercisable locally — see Human Verification. |
| 2 | Pushing further commits edits the same comment in place (3 pushes → 1 comment) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Fixed hidden header + `skip_unchanged: true` + job concurrency group `coverage-comment-${{ github.ref }}` `cancel-in-progress: true`. Idempotency is a runtime property. |
| 3 | No baseline available → comment still appears with absolute numbers and says delta unavailable; no error, no nonsense delta | ✓ VERIFIED | `TestRenderComment_NoBaseline` passes; reproduced live locally: `--mode=comment` with absent profiles + empty baselines exits 0 and emits `Delta not available yet — no main baseline cached ...` with em-dash deltas. CI wiring: `BASELINE_BACKEND/FRONTEND` set to `''` when `cache-matched-key == ''` (D-20, not `cache-hit`). Full CI-path confirmation listed for human as belt-and-braces. |
| 4 | A PR whose coverage drops still shows the drop and stays mergeable — only the pre-existing 80%/70% gates can block | ✓ VERIFIED | Never-blocker is structurally guaranteed: `coverage-comment` job `continue-on-error: true`, absent from `build-scan`/`release` `needs:` graph (`needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]` on build-scan is character-identical to pre-phase). Drop display: `renderRow` picks `⚠️` when `value < gate`, signed delta via `formatDelta`; covered by golden + `TestStatusMark_AtGateBoundary`. Backend upload step ordered before the gate step (line 57 vs 68) so a gate failure still hands off a profile. Merge-button confirmation listed for human. |
| 5 | A merge to main publishes that run's coverage as the baseline; no PR run recomputes main's coverage | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | "No recompute" half VERIFIED: `coverage-comment` only checks out, restores cache, downloads artifacts, and runs `--mode=comment` — it never invokes `make test`/`pnpm test`. "Publish baseline" half: sidecar-writer + `cache/save` steps gated `success() && github.event_name == 'push' && github.ref == 'refs/heads/main'`, `continue-on-error: true` — statically correct, cross-run cache behavior not exercisable locally. |

**Score:** 2/5 truths verified (SC #3, SC #4); 3 present, behavior-unverified (SC #1, SC #2, SC #5).

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `cmd/coverage-report/main.go` | stdlib-only tool: 3 modes, hand-parse profile + Vitest summary, delta math, renderer, SHA validator | ✓ VERIFIED | 491 lines; `go vet` clean; `golangci-lint run` 0 issues; `go list -deps | grep golang.org/x` = 0; `go.mod`/`go.sum` unchanged. Blocks merged by position (`b1967f6`) to match `x/tools/cover`. |
| `cmd/coverage-report/main_test.go` | golden + table tests for every mode and degraded path | ✓ VERIFIED | 18 test functions, all PASS: `TestRenderComment_Golden`, `_NoBaseline`, `_Unchanged`, `_MissingProfile`, `_NoUntrustedInterpolation`, `TestStatusMark_AtGateBoundary`, `TestModeTotal_*`, `TestModeSidecar_Roundtrip`, `TestSHAValidation`, `TestBackendTotalPct_MergesDuplicateBlocks`, etc. |
| `cmd/coverage-report/testdata/*` (12 files) | committed fixtures incl. 4 golden markdown bodies | ✓ VERIFIED | All 12 git-tracked, none gitignored. Golden bodies match the frozen format in 15-01-SUMMARY. |
| `.golangci.yml` | scoped `^cmd/coverage-report/` → `gosec` carve-out (D-19) | ✓ VERIFIED | Present, anchored, scoped to `gosec` only; lint clean, no G304 on the package. (Info: header comment still says v2.12.2 while CI pins v2.13.2 — cosmetic, per 15-REVIEW IN-01.) |
| `Makefile` | `COVER_PKGS` drops `cmd/coverage-report`; `coverage-report` phony target; `coverage-gate` consumes the tool | ✓ VERIFIED | `make -n test-integration` coverpkg contains `cmd/server`, excludes `cmd/coverage-report` and `internal/db/sqlc`. `coverage-report` on `.PHONY` line 1 and defined once. `coverage-gate` has 2 `--mode=total` call sites; `go tool cover -func` gone from non-comment lines; `COVERAGE_THRESHOLD_BACKEND ?= 80` and PASS/FAIL exits intact. |
| `web/vitest.config.ts` | `json-summary` reporter, thresholds untouched | ✓ VERIFIED | `reporter: ["text", "json-summary"]`; no `"json"`, no `reportsDirectory`; thresholds still `70` on all 4 axes. Real run present: `web/coverage/coverage-summary.json` exists with `total.lines.pct = 89.34` (number). Schema matches the 15-01 fixture path `total.lines.pct`. |
| `.github/workflows/full-pipeline.yml` — `coverage-comment` job + producer steps | report-only job, baseline publish, artifact upload | ✓ VERIFIED (static) | See Key Link table. `actionlint` exits 0. 11 top-level job keys (10 + new). All 37 `uses:` are 40-hex SHA-pinned with `# vX` comments. |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `coverage-gate` (Makefile) | `cmd/coverage-report --mode=total` | shell command substitution | ✓ WIRED | `coverage=$$(go run ./cmd/coverage-report --mode=total --profile=coverage.out)`; empty-output guard retained; tool sends diagnostics to stderr only (`TestModeTotal_MissingProfile`). |
| `test` job upload | `coverage-comment` download | artifact name `coverage-backend-pr` | ✓ WIRED | Upload path `coverage.out` → download into `pr-backend/` → render reads `pr-backend/coverage.out`. Upload `if: !cancelled()`, ordered before the gate step. |
| `frontend-test` job upload | `coverage-comment` download | artifact name `coverage-frontend-pr` | ✓ WIRED | Upload path `web/coverage/coverage-summary.json` → download into `pr-frontend/` → render reads `pr-frontend/coverage-summary.json`. Repo-relative path (Pitfall 8 honored). |
| `test`/`frontend-test` `cache/save` | `coverage-comment` `cache/restore` | key prefix `coverage-baseline-main-{backend,frontend}-` | ✓ WIRED | Save key `...-${{ github.sha }}`; restore key `...-${{ github.sha }}` + `restore-keys: ...-` (prefix). Restore `path:` lists byte-match the save `path:` lists. |
| `cache/restore` output | render step baseline flag | `steps.restore-*.outputs.cache-matched-key != ''` | ✓ WIRED | D-20 honored: branches on `cache-matched-key`, never `cache-hit` (0 occurrences of `cache-hit` outside comments). Sidecar path passed only on a match, `''` otherwise. |
| render step | sticky comment step | `steps.render.outputs.has_content == 'true'` + same-repo guard | ✓ WIRED | One-line `if:` combines `github.event.pull_request.head.repo.full_name == github.repository` with the `has_content` guard. `has_content` written only when `[ -s comment.md ]`. |
| `coverage-comment` job | release-path graph | (must be absent) | ✓ VERIFIED ABSENT | Not in `build-scan` needs (`[vet, lint, test, gitleaks, trivy-fs, frontend-test]`, unchanged) nor `release` needs (`[build-scan]`). Job-level `continue-on-error: true`. |
| `coverage-comment` permissions | workflow-level block | job-scoped `pull-requests: write` | ✓ WIRED | `pull-requests: write` appears exactly once; workflow-level `permissions:` still `contents: read` (lines 7-8 unchanged). No `pull_request_target` anywhere. Fork guard on the comment step. |

### Data-Flow Trace (Level 4)

| Value | Source | Produces Real Data | Status |
| --- | --- | --- | --- |
| Backend coverage % | `backendTotalPct` hand-parse of `pr-backend/coverage.out` (uploaded by the `test` job from a real `make test-integration` run) | Yes — statement-weighted, block-merged; matches `go tool cover -func` (90.0 vs tool 90.03 on a real merged profile, 15-02-SUMMARY) | ✓ FLOWING |
| Frontend coverage % | `frontendLinesPct` reads `total.lines.pct` from Vitest `coverage-summary.json` | Yes — real local run yields `89.34` (number) | ✓ FLOWING |
| Delta vs main | rounded current − rounded baseline sidecar `pct` | Yes when baseline present; em-dash + footer when absent (unit-tested + locally reproduced) | ✓ FLOWING / graceful STATIC fallback |
| Baseline provenance SHA | `sidecar.SHA` through `validSHA` (7–40 lc hex) → 7-char prefix | Yes — format-validated, never raw-interpolated | ✓ FLOWING |
| Comment body literals / timestamp | compile-time strings + tool-generated RFC3339 | Yes — `TestRenderComment_NoUntrustedInterpolation` proves no coverage-file text reaches the body | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Full test suite for the tool | `go test ./cmd/coverage-report/... -count=1` | `ok` — 18 test functions pass | ✓ PASS |
| Lint clean incl. gosec carve-out | `golangci-lint run ./cmd/coverage-report/...` | `0 issues` | ✓ PASS |
| Comment mode degrades on missing inputs | `go run ./cmd/coverage-report --mode=comment` with 2 nonexistent paths, empty baselines | exit 0; body has exactly 2 `unavailable`; delta-unavailable footer | ✓ PASS |
| Workflow lint | `actionlint .github/workflows/full-pipeline.yml` | exit 0, no findings | ✓ PASS |
| Vitest summary schema | `node -e "require('./web/coverage/coverage-summary.json').total.lines.pct"` | `89.34` (number) | ✓ PASS |
| coverpkg exclusion | `make -n test-integration` coverpkg list | contains `cmd/server`, excludes `cmd/coverage-report` + `internal/db/sqlc` | ✓ PASS |
| Live GitHub Actions PR run | — | not runnable from this environment | ? SKIP → human |

### Probe Execution

No `scripts/*/tests/probe-*.sh` in this repo and no probe declared by the phase. `actionlint` (the phase's declared workflow-lint gate) run above — clean.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| CICD-13 | 15-01, 15-02, 15-03 | Same-repo PR: single in-place-updated comment with backend + frontend totals + delta vs main baseline; report-only, never blocks merge | ✓ SATISFIED (static) / ? live-run pending | Tool + renderer + sticky-comment job all present and wired; never-blocker structurally guaranteed. Runtime comment behavior = human items 1, 2, 4. REQUIREMENTS.md already marks Complete. |
| CICD-14 | 15-01, 15-02, 15-03 | Main-branch runs publish coverage as the baseline; comment degrades gracefully (absolute numbers only) with no baseline | ✓ SATISFIED (static) / ? live-run pending | Baseline publish steps gated on green push to main; graceful degradation unit-tested + locally reproduced. Cross-run cache = human items 3, 5. |

No orphaned requirements — REQUIREMENTS.md maps only CICD-13/CICD-14 to Phase 15, both declared in all three plans.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| — | — | No `TODO`/`FIXME`/`XXX`/`HACK`/`PLACEHOLDER` in any phase-15 file | ℹ️ Info | Clean |
| `.golangci.yml` | 4-11 | Header cites `v2.12.2`; CI + pre-commit pin `v2.13.2` | ℹ️ Info | Stale comment only (15-REVIEW IN-01) |
| `cmd/coverage-report/testdata/backend-profile-malformed.txt` | — | Fixture committed but not referenced by any test | ℹ️ Info | Header-error path uncovered (15-REVIEW WR-04) |

15-REVIEW.md: 0 critical, 6 warnings, 5 info. Warnings are advisory quality items (gate-rounding leniency WR-01/WR-06, token surface WR-02, three test-coverage gaps WR-03/04/05) — none blocks the phase goal. Recommend addressing WR-02 (`persist-credentials: false` on the `coverage-comment` checkout) before the first real PR run.

### Human Verification Required

See frontmatter `human_verification` — 5 items, all requiring a live scratch-branch PR run on GitHub Actions (matches WINDOWS.md entry 8). In priority order:

1. **SC #1** — one comment, both rows, deltas, on PR open.
2. **SC #2** — three pushes → one comment (in-place edit).
3. **SC #3 (CI path)** — no-baseline run posts absolute numbers + delta-unavailable footer, job green. *(Tool-level behavior already verified.)*
4. **SC #4 (merge button)** — coverage drop shows in comment with warning glyph; PR stays mergeable. *(Never-blocker property already structurally verified.)*
5. **SC #5** — merge publishes the baseline; next PR diffs against it; no PR job recomputes main's coverage.

### Gaps Summary

No gaps. Every artifact exists, is substantive, is wired, and passes its tests; `actionlint` and `golangci-lint` are clean; the coverpkg exclusion, the gate cutover, and the Vitest reporter are all confirmed against real output. The report machinery is complete and the "never a merge blocker" constraint is statically guaranteed.

What remains is runtime confirmation of the five ROADMAP success criteria on a live GitHub Actions PR — the sticky-comment upsert, the artifact hand-off, and cross-run Actions-cache save/restore cannot be exercised from this environment. These are classified as human verification items (not gaps), consistent with `15-VALIDATION.md` "Manual-Only Verifications" and the phase's own un-run UAT ledger entry. Status is therefore `human_needed`.

---

_Verified: 2026-09-02T23:15:00Z_
_Verifier: Claude (gsd-verifier)_
