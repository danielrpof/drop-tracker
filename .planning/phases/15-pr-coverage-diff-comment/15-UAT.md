---
status: complete
phase: 15-pr-coverage-diff-comment
source: [15-VERIFICATION.md]
started: 2026-09-02T23:03:10Z
updated: 2026-09-03T19:12:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Same-repo PR produces exactly one coverage comment with both rows and deltas (SC #1)
expected: Single comment, header `drop-tracker-coverage`, `## Coverage` table with Backend + Frontend rows, each with 2dp % and a signed pp delta (or em-dash if no baseline).
result: pass
verified: |
  PR #2 (danielrpof/drop-tracker), Full Pipeline run 33782262458 (event: pull_request).
  test + frontend-test green; coverage-comment job completed/success. Exactly one
  github-actions comment posted with `## Coverage` heading and a table: Backend 89.92%
  / Frontend 89.34%, 2-dp, em-dash deltas (no baseline cached), sticky marker
  `drop-tracker-coverage`.

### 2. Three pushes leave one comment, edited in place (SC #2)
expected: After pushing two more commits to the same PR, still exactly one coverage comment, edited in place — no duplicates; a no-op re-push emits no edit notification (`skip_unchanged`).
result: pass
verified: |
  PR #2, 4 pushed head SHAs (3c38b82, 27d3578, 7de3e66, db4618e). GitHub issue-comments
  API reports count: 1 throughout; the single comment keeps created=2026-09-03T17:05:05Z
  while updated advances each run (…17:20:17Z, …17:25:31Z) — edited in place via the
  `drop-tracker-coverage` sticky marker, never recreated. Note: the `skip_unchanged`
  no-op sub-case is not separately observable — the rendered body embeds the head SHA
  and a generation timestamp, so consecutive runs are never byte-identical. Minor;
  core SC #2 (N pushes → 1 edited comment) holds.

### 3. No-baseline CI run posts absolute numbers + delta-unavailable footer, job green (SC #3, CI path)
expected: With no baseline cache entry (first run after ship, or the entry deleted), the comment still posts with absolute coverage numbers, em-dash deltas, and the "Delta not available yet — no main baseline cached" footer; no error; the coverage-comment job is green. (Tool-level behavior already unit-tested and reproduced locally.)
result: pass
verified: |
  Same run 33782262458. Job log: "Cache not found for input keys:
  coverage-baseline-main-backend-… , coverage-baseline-main-backend-" (and frontend);
  render step env BASELINE_BACKEND/BASELINE_FRONTEND both empty, UPSTREAM_RED=false.
  Comment footer: "Delta not available yet — no main baseline cached (first run or
  evicted). Absolute coverage shown." coverage-comment job completed/success, no error.

### 4. Coverage drop shows in the comment with a warning glyph; PR stays mergeable (SC #4, merge-button confirmation)
expected: Push a change dropping backend below 80% or a Vitest axis below 70%. The comment posts the real (lower) number with the warning glyph; the producing gate job (`test` / `frontend-test`) goes red; the `coverage-comment` job stays green/neutral; the PR remains mergeable with no new required check — branch-protection required checks do not include `coverage-comment`. (Never-blocker property already structurally verified.)
result: pass
verified: |
  Commit db4618e added an unimported 706-line module (web/app/lib/uat-dead-code.ts),
  dropping Vitest lines coverage to 20.34%. Run 33784303141 (pull_request):
  frontend-test = completed/failure ("ERROR: Coverage for lines (20.34%) does not meet
  global threshold (70%)"); coverage-comment = completed/success. Render env
  UPSTREAM_RED=true. Comment (still count: 1, edited in place) Frontend row:
  "20.34% | — | 70% | ⚠️" plus footer line "Note: an upstream CI job was red…".
  Scratch file reverted in 1f54af8.
  Mergeability: main has no branch protection (gh api …/branches/main/protection → 404),
  so no required-check list exists. PR #2 reported mergeable: MERGEABLE throughout
  (mergeStateStatus UNSTABLE = failing non-required checks, still mergeable).
  coverage-comment is in no job's `needs:` and carries job-level continue-on-error: true.

### 5. Merge publishes the baseline; next PR diffs against it; no PR job recomputes main's coverage (SC #5)
expected: Merging the scratch PR (to main, or a throwaway target branch running the pipeline as main-like) executes the "Write backend/frontend baseline sidecar" + "Save ... coverage baseline" steps (gated `success() && push && refs/heads/main`). A fresh PR opened afterward shows a numeric delta with a `baseline: main@<sha>` provenance line. No PR-triggered job runs the backend/frontend suite against main's tree.
result: pass
verified: |
  Phase 15 pushed to origin/main at f0ad39f. Main-push run 33786944467: test +
  frontend-test green; "Write backend/frontend baseline sidecar" + "Save coverage
  baseline" steps ran (BASELINE_SHA f0ad39f). Two Actions caches created:
  coverage-baseline-main-{backend,frontend}-f0ad39fd8e6997d3cf8ec76c16b6790aaf3a98d2.
  Fresh PR #3 (head 85c3cf2), run 33794353082: coverage-comment restored both baselines
  via the bare prefix restore-key ("Cache hit for restore-key:
  coverage-baseline-main-backend-f0ad39f…" after the merge-commit primary key missed —
  D-20). Render env BASELINE_BACKEND=baseline-metrics-backend.json,
  BASELINE_FRONTEND=web/baseline-metrics-frontend.json, UPSTREAM_RED=false. Comment:
  Backend 89.92% ±0.00pp ✅ / Frontend 89.34% ±0.00pp ✅ (numeric deltas, provably
  distinct from em-dash), footer "baseline: main@f0ad39f". No PR job recomputes main's
  coverage — the coverage-comment job has no test-run step (checkout + setup-go +
  cache/restore + download-artifact + render + upsert only); test/frontend-test run
  against PR #3's own merge ref, not main's tree.

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
