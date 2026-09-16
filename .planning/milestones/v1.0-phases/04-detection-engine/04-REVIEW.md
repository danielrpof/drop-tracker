---
phase: 04-detection-engine
reviewed: 2026-08-07T00:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - cmd/server/main.go
  - internal/db/migrate_test.go
  - internal/db/migrations/000003_events.down.sql
  - internal/db/migrations/000003_events.up.sql
  - internal/db/sqlc/events.sql.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc/querier.go
  - internal/detection/deezer.go
  - internal/detection/deezer_test.go
  - internal/detection/detector.go
  - internal/detection/detector_test.go
  - internal/detection/filter.go
  - internal/detection/filter_test.go
  - internal/detection/musicbrainz.go
  - internal/detection/musicbrainz_test.go
  - internal/musicbrainz/recordings.go
  - internal/musicbrainz/recordings_test.go
  - internal/musicbrainz/releases.go
  - internal/musicbrainz/releases_test.go
  - internal/poller/poller.go
  - internal/poller/poller_test.go
  - queries/events.sql
findings:
  critical: 0
  warning: 4
  info: 2
  total: 6
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-08-07T00:00:00Z
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

## Summary

This phase implements the diff-based detection engine (new_release / guest_feature /
deluxe_change) that sits between the MusicBrainz/Deezer pollers and the `events` seen-store
table, plus the two new MusicBrainz browse clients (`RecordingsByArtist`,
`ReleasesByReleaseGroup`) that feed it. The code is unusually well tested (real-Postgres
integration coverage for every detection branch, page-ceiling and pagination coverage for both
new browse clients, overlap-guard/panic-recovery coverage for the poller) and defensively
written (bounded pagination, `range`-only iteration with explicit comments citing ASVS V5,
per-item error isolation in most passes, parameterized SQL throughout, no eval/exec/shell
usage, no hardcoded secrets in production code).

No BLOCKER/Critical-severity defects were found — I could not construct a scenario that loses
data, crashes the process, or is exploitable as an injection/auth bypass. The findings below
are genuine correctness/robustness gaps traced through the actual call chains (not
speculative), concentrated in three areas: (1) an emergent notification-burst behavior when
`new_release` is muted and later un-muted, (2) unescaped/unvalidated use of externally-supplied
MusicBrainz ids when building URLs and dedup keys, and (3) an inconsistency in per-item error
isolation between the deluxe-change pass and its sibling passes. Two INFO-level maintainability
items round out the list.

## Warnings

### WR-01: Un-muting `new_release` after a period of being muted floods a notification burst for the entire backlog

**File:** `internal/detection/musicbrainz.go:64-139` (see also `internal/detection/detector.go:70-102`)
**Issue:**
`DetectMusicBrainz` computes `seedMode` exactly once per call, from `HasAnyEvent(artist_id,
source)` (detector.go:84-90). Seed mode is what makes a freshly-discovered backlog absorb
silently (`notified_at` set to `now()`, D-13) instead of surfacing as new alerts.

When `entry.MutedEventTypes` contains `"new_release"`, the entire new_release insert loop is
skipped (musicbrainz.go:76-84) — no rows are ever written to `events` for groups discovered
while muted. Because `preCycleSeenGroups`/`seedMode` are always derived fresh from the DB at
the top of the *next* call, and because `HasAnyEvent` will already be `true` for any artist
that has *any* prior musicbrainz-source row (from before muting, or from `guest_feature`/
`deluxe_change`), `seedMode` is `false` on the eventual un-mute cycle. The groups that
accumulated during the muted window are then inserted with `notifiedAt` = NULL (i.e.
"unnotified") all in the same cycle — exactly the "flood on discovery" case D-13's seed-mode
mechanism exists to prevent for a brand-new artist, but here reintroduced for an existing
artist via mute/un-mute.

This is a real, traceable behavior (not the already-documented D-10 "recorded under a
collaborator" edge case) and will surface as a burst of Phase 5 notifications the first time a
user un-mutes `new_release` for an artist who was muted across more than one release.

**Fix:** Either (a) explicitly document this as accepted behavior next to `eventTypeMuted`'s
new_release branch, or (b) decouple "should this row be inserted at all" from "should this row
be pre-notified": still insert muted new_release rows into the seen store every cycle (so the
backlog is captured incrementally, each cycle's own items correctly seeded/unseeded per D-13),
but simply never surface muted rows to Phase 5's notify query. Option (b) also fixes WR's
downstream deluxe-detection dependency on the new_release seen-set staying current.

### WR-02: Cover Art Archive URLs are built by raw string concatenation of an unescaped, semi-trusted MBID

**File:** `internal/detection/musicbrainz.go:446-452`
**Issue:**
```go
func coverArtURLForReleaseGroup(mbid string) string {
	return "https://coverartarchive.org/release-group/" + mbid + "/front"
}
```
`mbid` is `g.MBID`, sourced directly from a MusicBrainz JSON response. The codebase's own
comment on `isGuestFeature` (musicbrainz.go:390-396) explicitly acknowledges "MusicBrainz is
community-editable, semi-trusted data," yet this helper performs no validation (e.g. UUID-shape
check) or URL-escaping (`url.PathEscape`) before splicing the value into a URL that is persisted
to `cover_art_url` and will later be handed to Discord's embed renderer in Phase 5. A malformed
or adversarially-edited MBID (containing `/`, `?`, `#`, whitespace, or control characters) could
produce a broken or misleading URL in a user-facing notification.

**Fix:**
```go
func coverArtURLForReleaseGroup(mbid string) string {
	return "https://coverartarchive.org/release-group/" + url.PathEscape(mbid) + "/front"
}
```
Consider also validating `mbid` against MusicBrainz's UUID shape before treating it as
trustworthy enough to persist as a dedup key (see WR-03).

### WR-03: No validation that upstream ids are non-empty before being used as the events dedup key

**File:** `internal/detection/musicbrainz.go:92-122` (new_release loop), `:193-209`
(`detectGuestFeatures`), `internal/detection/deezer.go:61-84` (`DetectDeezer`)
**Issue:** `g.MBID` (release-group), `rec.MBID` (recording), and `a.ID` (Deezer album id) are
used verbatim as `ExternalID` — the value that participates in the `events_dedup_key` UNIQUE
constraint `(event_type, source, external_id)` — with no check that the value is non-empty or
well-formed. Contrast this with `internal/musicbrainz/recordings.go:94-97` and
`internal/musicbrainz/releases.go:102-105`, which both explicitly reject an empty/whitespace
MBID via `ErrEmptyMBID` before making a request. If MusicBrainz (or a future data source) ever
returns an entry with an empty/blank id — plausible for a malformed community-edited release
group, "Other"-type placeholder release, or a webhook-mocked page in a test double —
`releaseTypeAllowed`/`isGuestFeature` would still let it through, and it would collide with any
*other* empty-id entry for the same artist under the same `(event_type, source)` dedup key: the
first such row is recorded, every subsequent genuinely-distinct-but-malformed entry is silently
dropped by `ON CONFLICT DO NOTHING`, permanently losing that release from the seen store.

**Fix:** Skip (and log, so the truncation/data-quality is visible) any group/recording/album
whose external id is empty before it reaches `insertEvent`, e.g.:
```go
if strings.TrimSpace(g.MBID) == "" {
    logger.Warn("skipping release group with empty MBID", slog.String("artist_mbid", entry.MBID))
    continue
}
```

### WR-04: `detectDeluxeChanges` is not per-group error-isolated the way its sibling passes are

**File:** `internal/detection/musicbrainz.go:326-329, 333-335, 365-367`
**Issue:** The release-detail *fetch* error is isolated per group (logged, `continue`,
musicbrainz.go:296-303), matching `detectGuestFeatures`'s recording-fetch error isolation
(musicbrainz.go:176-183). But the two DB calls immediately after a successful fetch —
`d.groupBaseline` (326-329) and `d.setGroupBaseline` (333-335, 365-367) — both `return err`
directly on failure, aborting `detectDeluxeChanges` for the *entire remaining group list* of
that artist's cycle, with no per-group log line naming which group failed or which later
groups in `freshGroups` were never attempted as a result. This is inconsistent with the pattern
the rest of the file establishes (log-and-continue for per-item failures; only genuinely
systemic failures like `store.List` propagate upward), and a single transient DB blip mid-loop
(e.g. one query timing out under load) silently truncates that artist's deluxe pass for the
cycle with only a generic "detection failed" line from the poller — no indication anything
after the failing group was skipped.
**Fix:** Log the group-scoped error and `continue` to the next group, mirroring the
release-detail fetch branch immediately above it:
```go
baseline, hasBaseline, err := d.groupBaseline(ctx, g.MBID)
if err != nil {
    logger.Error("group baseline lookup failed",
        slog.String("artist_mbid", entry.MBID),
        slog.String("release_group_mbid", g.MBID),
        slog.String("db_error", err.Error()),
    )
    continue
}
```
(and similarly for both `setGroupBaseline` call sites), unless the intent really is that a DB
error should always be treated as artist-cycle-fatal — in which case the same treatment should
be applied to the release-detail fetch error for consistency, and the discrepancy should be
called out in a comment.

## Info

### IN-01: `DetectMusicBrainz` mixes an inline loop with two extracted sibling methods

**File:** `internal/detection/musicbrainz.go:64-139`
**Issue:** `DetectMusicBrainz` is ~75 lines and inlines the entire new_release mute-check +
loop, while its two sibling passes are each extracted into their own method
(`detectGuestFeatures`, `detectDeluxeChanges`). This asymmetry makes the function harder to
scan than it needs to be, and slightly obscures that all three passes follow the same shape
(mute check → seed/seen lookup → per-item filter → insert → structured log).
**Fix:** Extract the new_release branch into a `detectNewReleases(ctx, logger, entry, groups,
seedMode, notifiedAt) (map[string]struct{}, error)` returning the seen-set, mirroring the other
two passes' signatures, so `DetectMusicBrainz` reads as three parallel calls.

### IN-02: `DetectDeezer` duplicates `DetectMusicBrainz`'s new_release pass almost verbatim

**File:** `internal/detection/deezer.go:32-104`
**Issue:** The mute-check / seed-mode / seen-set / per-item filter / insert / structured-log
sequence in `DetectDeezer` is structurally identical to the new_release branch in
`musicbrainz.go`, differing only in the source constant and the per-item field mapping. A future
fix to one pass's logic (e.g. WR-01's mute/seed interaction, or WR-03's empty-id guard) is easy
to apply to one copy and forget the other, since there is no shared code enforcing they stay in
sync — the two files' own tests each cover their own pass in isolation.
**Fix:** Consider extracting a shared `detectNewRelease(ctx, logger, entry, source, items,
filterFn, toParamsFn)` helper parameterized over the per-source specifics, or at minimum a code
comment on each pass cross-referencing the other so a future edit to one prompts a check of the
other.

---

_Reviewed: 2026-08-07T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
