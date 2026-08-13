---
status: testing
phase: 09-ci-coverage-gates
source: [09-VERIFICATION.md]
started: 2026-08-13T17:30:00Z
updated: 2026-08-13T17:30:00Z
---

## Current Test

number: 1
name: Real CI run — under-threshold push turns the corresponding job red and `build-scan` is skipped
expected: |
  On a scratch branch (never `main`): (a) temporarily raise `COVERAGE_THRESHOLD_BACKEND` in the
  `Makefile` above the current measured 87.1%, push, and watch with `gh run watch` — the `test` job
  goes red and `build-scan` is reported skipped, not started. (b) Restore the backend threshold, then
  temporarily raise all four `web/vitest.config.ts` `coverage.thresholds` axes above their current
  measured figures, push, and confirm `frontend-test` goes red and `build-scan` is again skipped.
  (c) Restore both thresholds to 80/70, push, and confirm the full pipeline goes green through
  `build-scan`. Delete the scratch branch afterward.
awaiting: user response

## Tests

### 1. Real CI run — under-threshold push turns the corresponding job red and `build-scan` is skipped
expected: In both (a) and (b), `build-scan` is reported as skipped by GitHub Actions' `needs`
  mechanism — no image built, scanned, or pushed. In (c), the pipeline reaches `build-scan` normally.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
