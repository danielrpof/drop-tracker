---
status: testing
phase: 15-pr-coverage-diff-comment
source: [15-VERIFICATION.md]
started: 2026-09-02T23:03:10Z
updated: 2026-09-02T23:03:10Z
---

## Current Test

number: 1
name: Same-repo PR produces exactly one coverage comment with both rows and deltas (SC #1)
expected: |
  On a scratch-branch PR in this repo, after the coverage-comment job finishes: exactly one
  PR comment appears carrying the hidden header `drop-tracker-coverage`, a `## Coverage` table
  with a Backend row and a Frontend row, each showing a 2-decimal percentage and a signed pp
  delta (em-dash if no baseline is cached yet).
awaiting: user response

## Tests

### 1. Same-repo PR produces exactly one coverage comment with both rows and deltas (SC #1)
expected: Single comment, header `drop-tracker-coverage`, `## Coverage` table with Backend + Frontend rows, each with 2dp % and a signed pp delta (or em-dash if no baseline).
result: [pending]

### 2. Three pushes leave one comment, edited in place (SC #2)
expected: After pushing two more commits to the same PR, still exactly one coverage comment, edited in place — no duplicates; a no-op re-push emits no edit notification (`skip_unchanged`).
result: [pending]

### 3. No-baseline CI run posts absolute numbers + delta-unavailable footer, job green (SC #3, CI path)
expected: With no baseline cache entry (first run after ship, or the entry deleted), the comment still posts with absolute coverage numbers, em-dash deltas, and the "Delta not available yet — no main baseline cached" footer; no error; the coverage-comment job is green. (Tool-level behavior already unit-tested and reproduced locally.)
result: [pending]

### 4. Coverage drop shows in the comment with a warning glyph; PR stays mergeable (SC #4, merge-button confirmation)
expected: Push a change dropping backend below 80% or a Vitest axis below 70%. The comment posts the real (lower) number with the warning glyph; the producing gate job (`test` / `frontend-test`) goes red; the `coverage-comment` job stays green/neutral; the PR remains mergeable with no new required check — branch-protection required checks do not include `coverage-comment`. (Never-blocker property already structurally verified.)
result: [pending]

### 5. Merge publishes the baseline; next PR diffs against it; no PR job recomputes main's coverage (SC #5)
expected: Merging the scratch PR (to main, or a throwaway target branch running the pipeline as main-like) executes the "Write backend/frontend baseline sidecar" + "Save ... coverage baseline" steps (gated `success() && push && refs/heads/main`). A fresh PR opened afterward shows a numeric delta with a `baseline: main@<sha>` provenance line. No PR-triggered job runs the backend/frontend suite against main's tree.
result: [pending]

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps
