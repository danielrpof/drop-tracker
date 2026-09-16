---
status: complete
phase: 18-backend-readiness-poll-run-history-status-api
source: [18-VERIFICATION.md]
started: 2026-09-09T00:00:00Z
updated: 2026-09-10T04:15:00Z
---

## Current Test

[testing complete]

## Tests

### 1. CI build-provenance + single-build guarantee on the first `main` push
expected: The `build-scan` job's "Verify build provenance landed in the binary" step runs and passes (extracted binary contains the commit SHA); the `release` job does a `docker load` of the scanned tarball and no `docker build` of its own (single-build guarantee, 07-REVIEW CR-02, intact).
result: pass
verified: |
  CI run 34435266885 (push of 886f936 to main), Full Pipeline green.
  build-scan → "Verify build provenance landed in the binary": "build provenance OK:
  binary carries 886f936c46f9ce1bda45f204b936e867a89fa551".
  release → "Load scanned image": "Loaded image: drop-tracker:scan", then tag+push
  ghcr.io/danielrpof/drop-tracker:v1.9.0. No docker build step in the release job.
  (First push run 34434758203 had build-scan/release skipped by an unrelated
  js-yaml HIGH CVE in trivy-fs; fixed in 886f936, re-verified on the green run.)

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
result: pass
verified: |
  Driven against a real `docker compose up --build` instance (gate active,
  INSTANCE_PASSPHRASE set, image built flagless → app_version "dev"). 2026-09-10.
  - GET /ready unauth healthy → 200 {"status":"ready","schema_applied":7,"schema_expected":7}
  - GET /ready unauth, Postgres stopped → 503
    {"status":"not_ready","schema_applied":null,"schema_expected":7,"reason":"db_unreachable"}
    (machine reason, no DSN/driver/path; never 401 on the gated instance)
  - GET /status no session → 401 {"error":"unauthenticated"} (gate body, not a partial payload)
  - GET /status with session → 200, X-Instance-Gated: 1; body matches
    docs/api/status-contract.md "fresh instance" example field-for-field:
    poll_interval_seconds 900 (== container POLL_INTERVAL=15m, operator-confirmed),
    watchlist_size 12 (== GET /watchlist count, operator-confirmed against the SPA view),
    instance.app_version "dev", schema_applied/expected 7,
    sources = {musicbrainz, deezer} each last_run null / history [] / last_skipped_at null
    / consecutive_skips 0. No error/detail/message/last_error key anywhere.
  - GET /status while DB fully down → 500 {"error":"internal error"} (contract-correct:
    watchlist-count failure = 500, no partial answer).

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
