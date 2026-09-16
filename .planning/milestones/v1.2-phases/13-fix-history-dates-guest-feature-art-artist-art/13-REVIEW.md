---
phase: 13-fix-history-dates-guest-feature-art-artist-art
reviewed: 2026-08-24T00:00:00Z
depth: standard
files_reviewed: 27
files_reviewed_list:
  - cmd/server/main.go
  - internal/artistart/activity.go
  - internal/artistart/activity_test.go
  - internal/artistart/backfill.go
  - internal/artistart/backfill_test.go
  - internal/artistart/match.go
  - internal/artistart/match_test.go
  - internal/db/migrate_test.go
  - internal/db/migrations/000005_artists_art_match_attempted_at.down.sql
  - internal/db/migrations/000005_artists_art_match_attempted_at.up.sql
  - internal/db/sqlc/artists.sql.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc/querier.go
  - internal/detection/detector.go
  - internal/detection/detector_test.go
  - internal/detection/filter_test.go
  - internal/detection/musicbrainz.go
  - internal/detection/musicbrainz_test.go
  - internal/musicbrainz/recording_lookup.go
  - internal/musicbrainz/recording_lookup_test.go
  - internal/poller/poller_test.go
  - internal/watchlist/service.go
  - internal/watchlist/service_test.go
  - queries/artists.sql
  - web/app/components/history/EventCard.test.tsx
  - web/app/components/history/EventCard.tsx
  - web/package.json
findings:
  critical: 0
  warning: 3
  info: 1
  total: 4
status: issues_found
resolution:
  WR-01: "fixed in 1029f5b (test in a77041e)"
  WR-02: "fixed in abf0cdc (test in 5ca9f4a)"
  WR-03: "fixed in 722fe8e (test in df9406d)"
  IN-01: "addressed as a side effect of WR-02's fix -- attemptErrByMbid is now exercised for the unmatched branch"
---

# Phase 13: Code Review Report

**Reviewed:** 2026-08-24T00:00:00Z
**Depth:** standard
**Files Reviewed:** 27
**Status:** issues_found

## Summary

Phase 13 adds three related pieces: (1) a guest-feature per-recording release lookup that stores `release_date`/`release_group_mbid`/`cover_art_url` on `guest_feature` events and surfaces the date on the History cards, (2) a new `artistart` package implementing D-08's Deezer-artist-art match rule plus a tie-break, wired into `watchlist.Service.Add` at add-time and into a new startup `Backfill` sweep coordinated via an `ActivityGate`, and (3) the supporting migration/query/sqlc plumbing (`art_match_attempted_at` cooldown column).

The implementation is unusually well-documented and heavily tested (`go vet` is clean, `go test ./internal/artistart/... ./internal/musicbrainz/...` passes). Most of the fail-closed/D-09 logic, the tie-break, the ActivityGate concurrency, and the cooldown query are exercised by targeted tests. However, direct code tracing surfaced one precision-based date-comparison helper that can panic on malformed (but plausible, given MusicBrainz's community-editable nature) upstream data, an accounting bug in `Backfill`'s `Stats` that violates its own documented invariant, and a cleanup-on-panic gap in the add-time art-match's `ActivityGate` registration. None of these are exercised by the existing test suite — the `stubStore.upsertErrByMbid`/`attemptErrByMbid` fields built specifically to test the affected `Backfill` branches are defined but never populated by any test.

## Warnings

**Resolution (2026-08-24):** All three warnings below were fixed at the user's request during UAT (`fixnow`). Each fix follows RED→GREEN: a regression test reproducing the exact defect, then the minimal patch. See `resolution` in this file's frontmatter for commit hashes.

### WR-01: `earlierDate` can panic on a MusicBrainz date shorter than 4 characters

**File:** `internal/detection/musicbrainz.go:570-584` (helper introduced by this phase; called from `earliestReleaseDate` at line 565, itself called from `detectGuestFeatures` at line 257 on every newly-detected guest-feature recording)

**Issue:** `earlierDate`'s doc comment claims: "Both a and b are guaranteed non-empty and at least 4 characters (a full year) by `earliestReleaseDate`'s empty-string filter." That filter (`musicbrainz.go:558`) only excludes the exact empty string `""` — it does not check length. If a recording has two or more releases and one carries a non-empty `Date` shorter than 4 characters (e.g. a malformed/community-edited value like `"9"` or `"20"`), `earlierDate` executes `a[:4]` / `b[:4]` on a string shorter than 4 bytes and panics with a slice-bounds-out-of-range error.

This is reachable from live, semi-trusted, community-editable MusicBrainz data — the same threat model this exact file explicitly defends against elsewhere (`isGuestFeature`'s explicit `len(rec.ArtistCredit) == 0` guard before indexing position 0, and the repeated "T-04-12, ASVS V5" / "externally-supplied" comments throughout `musicbrainz.go`). The poller's per-artist worker does `recover()` around each artist (`internal/poller/poller.go:337-345`), so this would not crash the whole process, but it would silently abort the remainder of that one artist's detection cycle for that poll (including any later groups/recordings still to be processed in the same `DetectMusicBrainz` call) every time it is hit, recovering only on the next scheduled poll.

No test in `internal/detection/musicbrainz_test.go`'s `TestEarliestReleaseDate` table exercises a date shorter than 4 characters — every date fixture is a real year or longer.

**Fix:** Treat any date whose length is under 4 (or that fails a cheap all-digit check on the first 4 bytes) the same as an empty date in `earliestReleaseDate`'s filter, so `earlierDate` never receives a too-short string:

```go
func earliestReleaseDate(releases []musicbrainz.RecordingRelease) string {
	earliest := ""
	for _, r := range releases {
		if len(r.Date) < 4 { // malformed/empty: never a candidate
			continue
		}
		...
```

### WR-02: `Backfill`'s `Stats.Unmatched`/`Stats.Errored` can both increment for the same artist, breaking the documented invariant

**File:** `internal/artistart/backfill.go:162-178`

**Issue:** The `Stats` doc comment (lines 51-55) states: "Matched + Unmatched + Errored sum consistently against [Visited] ... since every branch below increments exactly one before moving to the next artist." The unmatched branch violates this:

```go
if !res.Matched {
	stats.Unmatched++
	if err := store.RecordArtMatchAttempt(ctx, a.Mbid); err != nil {
		stats.Errored++          // <-- also increments Errored
		logger.Error(...)
	}
	stats.Visited++
	continue
}
```

If `RecordArtMatchAttempt` fails for an unmatched artist, both `stats.Unmatched` and `stats.Errored` are incremented for that same artist, so `Matched + Unmatched + Errored > Visited`. This contradicts the doc comment and inflates both the `unmatched` and `errored` counts in the `"artist art backfill complete"` summary log line, which D-11 calls out as "the sweep's single operational signal that matching is actually working in production." Compare with the equivalent path in the matched branch (lines 206-219), which correctly increments only `Errored` (not `Matched`) when `RecordArtMatchAttempt` fails after a successful match — that branch keeps the invariant intact.

This path is untested: `backfill_test.go`'s `stubStore.attemptErrByMbid` field exists specifically to drive this failure, but no test populates it for an unmatched artist (confirmed via search — `attemptErrByMbid` is only referenced in its own field declaration and read, never assigned in any test). Likewise `upsertErrByMbid` is declared but never exercised, so the sibling `UpsertArtist`-failure branch (lines 186-204) also has no direct test coverage, though that branch's counting logic is correct.

**Fix:** Don't double-count; either treat the record-attempt failure as authoritative for this artist:

```go
if !res.Matched {
	if err := store.RecordArtMatchAttempt(ctx, a.Mbid); err != nil {
		stats.Errored++
		logger.Error("artist art backfill: record attempt failed (unmatched)", ...)
	} else {
		stats.Unmatched++
	}
	stats.Visited++
	continue
}
```
and add a test that sets `attemptErrByMbid` for an artist whose `Match` result is unmatched, asserting `Matched+Unmatched+Errored == Visited`.

### WR-03: `watchlist.Service.Add` leaks the `ActivityGate` registration if `Matcher.Match` panics

**File:** `internal/watchlist/service.go:203-220`

**Issue:**
```go
var end func()
if s.activityGate != nil {
	end = s.activityGate.Begin()
}

res, matchErr := s.matcher.Match(matchCtx, p.MBID, p.Name)
cancel()
if end != nil {
	end()
}
```
`cancel()` and `end()` are plain statements after the blocking `Match` call, not `defer`red. If `s.matcher.Match` panics (a real possibility for an interface-typed dependency backed by HTTP clients doing JSON decoding, exactly the "malformed upstream response" class of failure this codebase explicitly designs against elsewhere — see WR-01 and `internal/poller/poller.go`'s per-worker `recover()`), `end()` is never called. `ActivityGate.count` (an `atomic.Int32`) is then permanently left incremented, so `Active()` reports `true` forever afterward. This defeats D-10's whole purpose: every subsequent call to `artistart.Backfill`'s `waitForActivityGate` would see the gate as permanently active and wait the full `backfillActivityMaxWait` (5s) before every single artist for the remaining lifetime of the process, rather than the intended short yield.

`cancel()` being skipped is comparatively harmless (the `WithTimeout` context still expires on its own timer and is GC'd), but the `ActivityGate` leak has an ongoing behavioral effect. `middleware.Recoverer`-style panic recovery elsewhere in the HTTP stack would keep the request from crashing the process, but nothing in this function itself protects `end()`'s invocation.

**Fix:**
```go
var end func()
if s.activityGate != nil {
	end = s.activityGate.Begin()
}
matchCtx, cancel := context.WithTimeout(ctx, matchTimeout)
defer cancel()
if end != nil {
	defer end()
}
res, matchErr := s.matcher.Match(matchCtx, p.MBID, p.Name)
```

## Info

### IN-01: Untested `stubStore` error-injection fields in `backfill_test.go`

**File:** `internal/artistart/backfill_test.go:67-70`

**Issue:** `upsertErrByMbid` and `attemptErrByMbid` are defined on `stubStore` with doc comments describing exactly the purpose of driving per-artist write failures ("let a single test drive a per-artist write failure without a second stub type"), but no test in the file ever populates either map. This is what let WR-02 go unnoticed.

**Fix:** Add tests covering: `UpsertArtist` failing for a matched artist (asserts no `RecordArtMatchAttempt` call, per the existing comment at lines 193-195), and `RecordArtMatchAttempt` failing for both a matched and an unmatched artist (asserts the `Stats` invariant `Matched+Unmatched+Errored == Visited` holds in all cases).

---

_Reviewed: 2026-08-24T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
