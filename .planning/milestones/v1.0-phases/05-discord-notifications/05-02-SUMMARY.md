---
phase: 05-discord-notifications
plan: 02
subsystem: notifier
tags: [discord, embed, release-type, track-count, truncation, url-safety]
dependency-graph:
  requires:
    - "internal/detection.Detector's insert paths (musicbrainz.go, deezer.go) -- Phase 4 / 05-01"
    - "sqlc.InsertEventParams.PreviousTrackCount/.ReleaseType, sqlc.Event.PreviousTrackCount/.ReleaseType -- 05-01 migration 000004"
    - "internal/notifier/format.go's tracer-scope switch skeleton -- 05-01"
  provides:
    - "release_type persisted on new_release inserts from both MusicBrainz and Deezer"
    - "previous_track_count persisted on the MusicBrainz deluxe_change insert"
    - "internal/detection.releaseTypeForStorage -- normalized, nullable release-type helper"
    - "Complete formatEmbed: per-event-type color/emoji/link/field population, rune-safe truncation"
  affects:
    - "internal/detection/musicbrainz.go (new_release and deluxe_change insert literals)"
    - "internal/detection/deezer.go (new_release insert literal)"
    - "internal/notifier/format.go (all three switch arms, now feature-complete)"
tech-stack:
  added: []
  patterns:
    - "Rune-based truncation (utf8.RuneCountInString + []rune slicing) instead of byte-length slicing, to keep truncated multi-byte titles valid UTF-8"
    - "URL construction via url.PathEscape over a fixed path template, never string-interpolating community-editable text (title/artist_name) into a link"
    - "appendField helper that omits a field entirely rather than emitting an EmbedField with an empty Value"
key-files:
  created:
    - internal/notifier/format_test.go
  modified:
    - internal/detection/musicbrainz.go
    - internal/detection/deezer.go
    - internal/detection/musicbrainz_test.go
    - internal/detection/deezer_test.go
    - internal/detection/detector_test.go
    - internal/notifier/format.go
decisions:
  - "Extracted releaseTypeForStorage(primaryType string) *string in musicbrainz.go rather than leaving the normalization inline, so the absent-PrimaryType-stores-NULL behavior is unit-testable directly -- a real DetectMusicBrainz call can never reach the insert with an empty PrimaryType (releaseTypeAllowed filters it out first), so the plan's 'absent PrimaryType stores NULL' behavior needed a seam below the filter to be provable at all."
  - "Placed the real-Postgres release_type/previous_track_count assertions in the existing detector_test.go (MusicBrainz) and deezer_test.go (Deezer) rather than creating new integration-style tests in musicbrainz_test.go, since detector_test.go -- not musicbrainz_test.go -- is this codebase's established home for DetectMusicBrainz's real-Postgres coverage; musicbrainz_test.go stayed whitebox/no-DB, consistent with its existing isGuestFeature-only content, and got the new releaseTypeForStorage unit test instead."
  - "guest_feature rows now assert both new columns are explicitly NULL (extended TestDetectMusicBrainz_GuestFeature) rather than adding a separate test, since the existing test already queries the row this assertion needs."
metrics:
  duration: 45min
  completed: 2026-08-08
actuals:
  tokens: 8500
  tasks: 2
  commits: 5
status: complete
---

# Phase 5 Plan 2: Complete Embed Rendering and Display-Field Persistence Summary

Closes NTFY-01/02/03's literal requirements: `release_type` and `previous_track_count` are now written at detection-insert time (the two columns Phase 4 computed in memory but never persisted), and `formatEmbed` renders all three event types completely -- distinct color/emoji/links, the deluxe-change track-count delta with its three shapes, and rune-safe truncation so a community-edited title can never split a multi-byte character or blow past Discord's field limits.

## What Was Built

**Task 1 -- Persistence (TDD: RED then GREEN):** `DetectMusicBrainz`'s new_release loop now stores `release_type` via a new `releaseTypeForStorage` helper -- the same lowercase/trim normalization `releaseTypeAllowed` already applies to `PrimaryType`, wrapped in `nullableString` so an absent type stores SQL NULL rather than `""`. `DetectDeezer`'s new_release insert stores `release_type` from `RecordType` the same way (Deezer's live-observed casing is already lowercase; a comment flags non-`album` casing as an open upstream assumption carried from Phase 3). The deluxe-change insert path captures `previous_track_count` from the `groupBaseline` value immediately before `setGroupBaseline` overwrites it a few lines later (D-04) -- the old count exists nowhere else once that call lands. `guest_feature` inserts are untouched; both new columns stay NULL there, confirmed by an extended assertion on the existing guest-feature test.

**Task 2 -- Embed rendering (TDD: RED then GREEN):** `formatEmbed`'s three arms are now feature-complete. `new_release` carries an Artist field, a Release Date field (when non-nil), a title-cased Type field (when non-nil), a cover-art Thumbnail, and a link built per `ev.Source` (MusicBrainz release-group page or Deezer album page). `guest_feature` carries only an Artist field (D-02: no full artist-credit re-fetch) and links to the MusicBrainz recording. `deluxe_change` carries a Tracks field with three shapes -- both counts present renders the arrow delta ("12 → 18 tracks"), only the current count present (a pre-migration-000004 row) renders that count alone with no arrow and no panic, neither present omits the field entirely -- plus a cover-art Thumbnail and a link to the MusicBrainz release. Every URL is built via `url.PathEscape` over a fixed path template keyed only on `external_id`; `Title`/`ArtistName` (community-editable free text) are never interpolated into a link (T-05-06). A new `truncateRunes` helper caps the embed title at 256 runes and every field value at 1024, cutting on a rune boundary via `[]rune` slicing rather than byte-offset slicing, so a 300-rune multi-byte title truncates to exactly 256 valid runes with no replacement character (T-05-02). A new `appendField` helper omits a field entirely when its value is empty after truncation, rather than emitting an `EmbedField` with an empty `Value` (Discord rejects those, and an empty-valued field reads as a rendering bug).

## Deviations from Plan

### Auto-fixed Issues

None -- no bugs, missing functionality, or blocking issues were found; both tasks executed within the plan's stated scope.

### Test-placement adjustments (not Rule 1-4 fixes, documented for traceability)

The plan's frontmatter named `internal/detection/musicbrainz_test.go` as the file to extend with real-Postgres release-type/deluxe-change assertions. That file is this codebase's established whitebox-only, no-DB test file (it holds only `isGuestFeature` unit tests); the actual real-Postgres coverage for `DetectMusicBrainz` lives in `internal/detection/detector_test.go`, which already contains the exact new_release and deluxe-change tests this plan's behavior list needed to extend. Followed the codebase's real convention over the plan's literal file name: extended `detector_test.go`'s `TestDetectMusicBrainz_NewRelease` (added `release_type` assertion), `TestDetectMusicBrainz_DeluxeChange_FiresOnIncrease` (added `previous_track_count` assertion), and `TestDetectMusicBrainz_GuestFeature` (added NULL assertions for both new columns); added a new whitebox `TestReleaseTypeForStorage` to `musicbrainz_test.go` for the absent-PrimaryType-stores-NULL case, which is unreachable through the full `DetectMusicBrainz` call path (`releaseTypeAllowed` filters an empty `PrimaryType` before any insert is attempted) and therefore needed a seam below the filter to be provable at all. `deezer_test.go` was extended as named in the plan.

## Known Environment Limitations (not deviations, not fixed)

- **`-race` still fails on this Windows dev box** with the same `ThreadSanitizer failed to allocate` error documented in 05-01-SUMMARY -- re-confirmed this session against `internal/notifier`. All verification in this plan ran without `-race`; every other specified test command passed cleanly.

## Self-Check: PASSED

- `internal/detection/musicbrainz.go` -- FOUND, contains `ReleaseType:` and `PreviousTrackCount:` and `"strings"` import
- `internal/detection/deezer.go` -- FOUND, contains `ReleaseType:`
- `internal/notifier/format.go` -- FOUND, complete three-arm switch with truncation/link helpers
- `internal/notifier/format_test.go` -- FOUND
- Commit ae716f7 -- FOUND (`git log --oneline --all`)
- Commit 7fff85f -- FOUND
- Commit d6ba76c -- FOUND
- Commit c525a57 -- FOUND
- Commit b8766a7 -- FOUND
