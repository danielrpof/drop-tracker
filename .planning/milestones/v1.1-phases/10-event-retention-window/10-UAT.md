---
status: complete
phase: 10-event-retention-window
source: [10-01-SUMMARY.md, 10-02-SUMMARY.md]
started: 2026-08-17T05:27:45Z
updated: 2026-08-17T05:58:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Recent events still show normally in History
expected: Open the History page. Releases from within your retention window (last EVENT_RETENTION_DAYS days, default 90) still show up exactly as before — nothing about ordinary browsing changed.
result: pass

### 2. Retention window hides old events and explains why the page looks empty
expected: |
  Set EVENT_RETENTION_DAYS to a small value (e.g. 1) in your .env and restart
  the server, so any existing events fall outside the window. Reload History
  with no artist/type filter active. The page does not say "No release
  activity yet" or "No matching events" — it shows a dedicated message
  ("Older than your retention window" / "There's release history for this
  view — it's just outside your retention window. Nothing was deleted.")
  telling you history exists but is outside the window, not that nothing
  ever happened.
result: pass

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0

## Gaps

[none yet]
