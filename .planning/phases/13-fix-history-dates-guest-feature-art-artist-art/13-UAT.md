---
status: testing
phase: 13-fix-history-dates-guest-feature-art-artist-art
source: [13-VERIFICATION.md]
started: 2026-08-24T23:18:44Z
updated: 2026-08-24T23:18:44Z
---

## Current Test

number: 1
name: MusicBrainz recording-lookup response shape
expected: |
  Field names/nesting match (`releases[].date`, `releases[].release-group.id`).
awaiting: user response

## Tests

### 1. MusicBrainz recording-lookup response shape
expected: From a machine that can reach musicbrainz.org, run `curl "https://musicbrainz.org/ws/2/recording/<a-real-recording-mbid>?inc=releases+release-groups&fmt=json"` and compare the returned nesting against `RecordingRelease`/`recordingLookupResponse`'s struct tags in `internal/musicbrainz/recording_lookup.go`. Field names/nesting should match (`releases[].date`, `releases[].release-group.id`). A mismatch would decode silently to zero values that look exactly like D-03's no-releases fallback.
result: [pending]

### 2. Accept-or-fix decision on the three code-review warnings
expected: Review WR-01/WR-02/WR-03 in `13-REVIEW.md` (all confirmed still present by verification) and decide whether to apply the drafted fixes now or explicitly accept them as tracked follow-up debt. WR-01: `earlierDate` can panic on a MusicBrainz date shorter than 4 characters. WR-02: `Backfill`'s `Stats` can double-increment `Unmatched`+`Errored` for the same artist. WR-03: `Service.Add` leaks the `ActivityGate` registration if `Matcher.Match` panics.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
