---
status: complete
phase: 13-fix-history-dates-guest-feature-art-artist-art
source: [13-VERIFICATION.md]
started: 2026-08-24T23:18:44Z
updated: 2026-08-25T00:15:00Z
---

## Current Test

[testing complete]

## Tests

### 1. MusicBrainz recording-lookup response shape
expected: From a machine that can reach musicbrainz.org, run `curl "https://musicbrainz.org/ws/2/recording/<a-real-recording-mbid>?inc=releases+release-groups&fmt=json"` and compare the returned nesting against `RecordingRelease`/`recordingLookupResponse`'s struct tags in `internal/musicbrainz/recording_lookup.go`. Field names/nesting should match (`releases[].date`, `releases[].release-group.id`). A mismatch would decode silently to zero values that look exactly like D-03's no-releases fallback.
result: pass
verified_against: "recording 03dbaf8a-23ce-43d6-8e69-2778ec82ab61 (\"Helluva Price\"), live response 2026-08-24"
notes: |
  Confirmed: releases[].date present directly on each release object (not only nested under release-events), releases[].release-group.id and .title present and correctly shaped. Response carries many extra fields (country, status, barcode, release-events, etc.) not mapped in the Go structs -- harmlessly ignored by encoding/json. Assumption A1 (13-RESEARCH.md) holds; no fix needed.

### 2. Accept-or-fix decision on the three code-review warnings
expected: Review WR-01/WR-02/WR-03 in `13-REVIEW.md` (all confirmed still present by verification) and decide whether to apply the drafted fixes now or explicitly accept them as tracked follow-up debt. WR-01: `earlierDate` can panic on a MusicBrainz date shorter than 4 characters. WR-02: `Backfill`'s `Stats` can double-increment `Unmatched`+`Errored` for the same artist. WR-03: `Service.Add` leaks the `ActivityGate` registration if `Matcher.Match` panics.
result: pass
decision: "fix now"
resolution: |
  All three fixed with RED-then-GREEN regression tests, verified via full go build/vet/lint/test (all clean):
  - WR-01: internal/detection/musicbrainz.go, filter dates <4 chars before earlierDate. Test a77041e, fix 1029f5b.
  - WR-02: internal/artistart/backfill.go, count failed RecordArtMatchAttempt as Errored only. Test 5ca9f4a, fix abf0cdc.
  - WR-03: internal/watchlist/service.go, scope ActivityGate/cancel cleanup to a deferred closure. Test df9406d, fix 722fe8e.

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
