---
status: testing
phase: 19-frontend-system-view
source: [19-VERIFICATION.md]
started: 2026-09-10T22:05:00Z
updated: 2026-09-10T22:05:00Z
---

## Current Test

number: 1
name: Live gated-instance smoke test
expected: |
  On a running gated instance with a real Postgres database, open the System tab and confirm
  both source panels render with real run data; the About block shows the real version, schema,
  database pill, watchlist size, and poll interval; clicking Refresh updates the freshness stamp
  while existing content stays on screen; logging out mid-view yields to the passphrase screen
  with no error flash.
awaiting: user response

## Tests

### 1. Live gated-instance smoke test
expected: All of the above render and behave correctly against real infrastructure — both source panels show real run data, the About block shows real version/schema/database/watchlist/poll-interval values, Refresh updates the freshness stamp without clearing existing content, and logout mid-view yields cleanly to the passphrase screen with no error flash.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
