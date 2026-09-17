---
status: complete
phase: 22-scheduled-digest-send
source: [22-VERIFICATION.md]
started: 2026-09-16T23:00:00Z
updated: 2026-09-17T21:25:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Alpine image resolves America/New_York in a real CI boot
expected: The step's `docker logs` poll finds both `"msg":"digest zone resolved"` and `"zone":"America/New_York"` in the booted `drop-tracker:scan` container's output within 60s, and the step exits 0.
result: pass
note: |
  First push (run 35272810632) failed before reaching build-scan: the `test` job hit a
  genuine data race in this phase's own 22-02 code (TestDigestScheduler_CheckError_LoggedAndLoopContinues,
  internal/notifier/scheduler_test.go), diagnosed and fixed via quick task 260917-mfa
  (commit 58ada7a). Re-pushed; run 35276004417's build-scan job passed, with the
  "Boot the built image and assert it resolves the digest zone" step logging
  "digest zone resolved OK" -- both "msg":"digest zone resolved" and
  "zone":"America/New_York" were found in the booted container's output.

## Summary

total: 1
passed: 1
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
