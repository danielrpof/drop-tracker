---
status: complete
phase: 03-external-clients-search
source: [03-VERIFICATION.md]
started: 2026-08-07T21:50:00Z
updated: 2026-08-08T00:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Live MusicBrainz search on a network-unrestricted environment
expected: Run the built binary against a network-unrestricted environment (CI runner, or a dev machine with outbound HTTPS to musicbrainz.org) and curl 'http://localhost:PORT/search?q=drake' — confirm sources.musicbrainz.status is "ok" and artists contains real, 36-character MBID entries.
why_human: Both the original executor sandbox and the verifier's own sandbox have no outbound TLS egress to musicbrainz.org (confirmed independently — "EOF" at the TLS handshake). This is an environment limitation, not observable from source code or unit tests. Deezer's equivalent path WAS confirmed live during verification (real Deezer artists returned).
result: issue
reported: "curl against WSL2-run binary: sources.deezer.status ok with 10 real artists, but sources.musicbrainz.status was \"error\" (artists: [])."
severity: major

### 2. Live SIGTERM graceful-shutdown drain ordering
expected: Send a real SIGTERM/SIGINT to the running binary on a POSIX shell (WSL2 or Linux) with an in-flight poll cycle, and confirm the JSON log stream shows the order: "shutdown signal received" -> poller stopping/stopped -> process exit, with no database-pool/connection error logged in between.
why_human: This Windows sandbox cannot deliver a real POSIX SIGTERM to a backgrounded Go process (same limitation Phase 1's WR-03 UAT documented). The drain-before-close ordering IS proven statically (defer pool.Close() precedes the deferred pollr.Stop() call) and by internal/poller's own Stop() unit tests, but full live process-shutdown log ordering has not been observed end-to-end.
result: pass

### 3. Live upstream backstop assumptions (Deezer quota-error shape, MusicBrainz throttling)
expected: Confirm the community-sourced Deezer quota-error envelope shape (assumption A1) and MusicBrainz's live per-IP throttling response to this client's self-imposed pacing still match what the code assumes, against the real upstream APIs.
why_human: Three must-have truths across 03-01/03-02/03-03 are explicitly marked verification: backstop in PLAN frontmatter — CI/unit tests exercise only recorded fixtures, never live calls (per CLAUDE.md's testing constraint). Deezer's search/albums shape WAS spot-checked live during verification and matched fixtures exactly; the Deezer quota-error shape and MusicBrainz's live throttling response were not (would require deliberately triggering a quota breach / rate-limit violation against a live third-party API).
result: skipped
reason: Deliberately triggering a live Deezer quota breach or a MusicBrainz rate-limit violation would mean sending abusive traffic against a real third-party API; both assumptions are explicitly `verification: backstop` in PLAN frontmatter and are already covered by recorded-fixture unit tests.

## Summary

total: 3
passed: 1
issues: 1
pending: 0
skipped: 1
blocked: 0

## Gaps

- gap_id: G-03-1
  truth: "sources.musicbrainz.status is \"ok\" with real, 36-character MBID artist entries"
  status: failed
  reason: "User reported: curl against WSL2-run binary returned sources.deezer.status ok (10 real artists) but sources.musicbrainz.status was \"error\" (artists: [])."
  severity: major
  test: 1
  artifacts: []
  missing: []
