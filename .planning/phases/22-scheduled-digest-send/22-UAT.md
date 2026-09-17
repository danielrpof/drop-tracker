---
status: testing
phase: 22-scheduled-digest-send
source: [22-VERIFICATION.md]
started: 2026-09-16T23:00:00Z
updated: 2026-09-16T23:00:00Z
---

## Current Test

number: 1
name: Push this branch (or merge per the documented 21+22+23 release-sequencing rule) and observe the `build-scan` job's new "Boot the built image and assert it resolves the digest zone" step in a real GitHub Actions run.
expected: |
  The step's `docker logs` poll finds both `"msg":"digest zone resolved"` and `"zone":"America/New_York"` in the booted `drop-tracker:scan` container's output within 60s, and the step exits 0. If the embedded `time/tzdata` were missing or broken, the step would exit 1 with the `::error::` annotation and dump the container logs/exit code.
awaiting: user response

## Tests

### 1. Alpine image resolves America/New_York in a real CI boot
expected: The step's `docker logs` poll finds both `"msg":"digest zone resolved"` and `"zone":"America/New_York"` in the booted `drop-tracker:scan` container's output within 60s, and the step exits 0.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
