---
status: testing
phase: 18-backend-readiness-poll-run-history-status-api
source: [18-VERIFICATION.md]
started: 2026-09-09T00:00:00Z
updated: 2026-09-09T00:00:00Z
---

## Current Test

number: 1
name: CI build-provenance + single-build guarantee on the first `main` push
expected: |
  On the first push to `main` after this phase merges, the `build-scan` job's
  "Verify build provenance landed in the binary" step runs and passes (the
  extracted server binary contains the commit SHA), and the `release` job shows a
  `docker load` of the scanned tarball with no `docker build` of its own.
awaiting: user response

## Tests

### 1. CI build-provenance + single-build guarantee on the first `main` push
expected: The `build-scan` job's "Verify build provenance landed in the binary" step runs and passes (extracted binary contains the commit SHA); the `release` job does a `docker load` of the scanned tarball and no `docker build` of its own (single-build guarantee, 07-REVIEW CR-02, intact).
result: [pending]

### 2. `/status` + `/ready` through the SPA against a real running instance
expected: |
  `docker compose up --build` with `INSTANCE_PASSPHRASE` set. Log in through the SPA,
  then fetch `/status` in the same browser session:
  - body matches `docs/api/status-contract.md` field for field
  - `sources` carries `musicbrainz` and `deezer`, each with `last_run` null and `history` `[]` on a fresh instance
  - `poll_interval_seconds` matches `POLL_INTERVAL`
  - `watchlist_size` matches the watchlist view
  - `instance.app_version` is `dev` for a local (flagless) image
  Then fetch `/status` in a private window with no session → `401`.
  Also hit `/ready` unauthenticated on the gated instance → `200`/`503` (never `401`),
  body carries `schema_applied` / `schema_expected` and no DSN/driver text.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
