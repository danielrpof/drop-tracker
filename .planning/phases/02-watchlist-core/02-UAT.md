---
status: testing
phase: 02-watchlist-core
source: [02-VERIFICATION.md]
started: 2026-08-06T02:00:00Z
updated: 2026-08-06T02:00:00Z
---

## Current Test

number: 1
name: End-to-end curl walkthrough and JSON ergonomics review
expected: |
  JSON response bodies read the way a future React client (Phase 6) would want to consume
  them; no error response at any point contains database internals (DSN, driver text,
  SQLSTATE codes).
awaiting: user response

## Tests

### 1. End-to-end curl walkthrough and JSON ergonomics review
expected: Run `make db-up && make run`, then exercise all four `/watchlist` routes with curl in sequence: add an artist, list it, narrow its release types via PATCH, mute an event category via PATCH, list again to confirm both axes, delete it, list once more to confirm it's gone. JSON response bodies should read the way a future React client (Phase 6) would want to consume them; no error response at any point should contain database internals (DSN, driver text, SQLSTATE codes).
result: [pending]

### 2. Decide the disposition of WR-01 and WR-02 (code review findings)
expected: Review 02-REVIEW.md's WR-01 (UpsertArtist silently drops disambiguation/image_url on re-add) and WR-02 (UpdatePreferences has an unhandled not-found race — returns 500 instead of 404 when a row is deleted between read and write — and a lost-update race between two concurrent PATCH calls touching different axes of the same entry) and decide whether either needs a fast-follow fix before Phase 3/4 start writing to these same tables, or is an accepted risk for this single-operator v1 deployment.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
