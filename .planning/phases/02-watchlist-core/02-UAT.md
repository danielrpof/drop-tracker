---
status: testing
phase: 02-watchlist-core
source: [02-VERIFICATION.md]
started: 2026-08-06T19:30:00Z
updated: 2026-08-06T19:30:00Z
---

## Current Test

number: 1
name: Decide disposition of new-WR-01 and new-WR-02 (in-scope, Warning-severity, code review findings)
expected: |
  Review the current 02-REVIEW.md's WR-01 (Service.UpdatePreferences has no independent domain-layer no-op guard — only the HTTP handler checks "at least one axis supplied") and WR-02 (POST/PATCH JSON decoders never verify the stream is exhausted, so trailing garbage after a valid body is silently accepted) and decide whether either needs a fast-follow fix before Phase 3/4 build additional callers of internal/watchlist, or is an accepted risk for v1.
awaiting: user response

## Tests

### 1. Decide disposition of new-WR-01 and new-WR-02 (in-scope, Warning-severity, code review findings)
expected: A recorded decision (accept as documented risk for v1, or open a follow-up plan) for each finding, the same pattern already used for the prior WR-01/WR-02 pair.
result: [pending]

### 2. Out-of-scope flag: CR-01 (Critical, Phase 1 file, currently unresolved)
expected: A recorded accept-or-fix decision for internal/db/migrate.go's redactError only stripping URL-form DSN userinfo, not libpq keyword/value-form password=... — tracked separately from Phase 2 since the file has zero Phase 2 commits against it, but flagged because it is Critical severity and directly implicates CLAUDE.md's "all secrets via environment variables only... nothing real ever committed" constraint.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
