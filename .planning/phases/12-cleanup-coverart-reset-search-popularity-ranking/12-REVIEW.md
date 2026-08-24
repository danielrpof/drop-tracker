---
phase: 12-cleanup-coverart-reset-search-popularity-ranking
reviewed: 2026-08-23T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/deezer/search.go
  - internal/deezer/search_test.go
  - internal/httpserver/search.go
  - internal/httpserver/search_test.go
  - internal/musicbrainz/search.go
  - internal/musicbrainz/search_test.go
  - web/app/components/common/CoverArt.test.tsx
  - web/app/components/common/CoverArt.tsx
  - web/app/components/watchlist/SearchBox.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.tsx
  - web/app/lib/api.ts
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 12: Code Review Report

**Reviewed:** 2026-08-23
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

Reviewed all three loose ends closed by this phase: (1) `CoverArt.tsx`'s `useEffect`-based `failed` reset keyed on `src` (D-01), (2) Deezer's `nb_fan`-descending stable sort inside `internal/deezer.SearchArtists` (D-04), and (3) MusicBrainz `country` as a disambiguation fallback threaded from `internal/musicbrainz.Artist` through `internal/httpserver.SearchArtist` to `web/app/lib/api.ts` and rendered in `SearchResultsColumns.tsx` (D-09/D-10). `go build ./...`, `go vet ./...`, and `tsc --noEmit` are all clean, and the diff against `d23c4f0034a407d3ba23e2776afef482cf21ae44^` is tightly scoped to the decisions in 12-CONTEXT.md — no scope creep.

The sort comparator, stable-sort tie-break, disambiguation/country nil-pointer mapping, wire-shape key-set pin, and the `useEffect([src])` reset are all correct and each has a targeted regression test proving the specific behavior (fan-count descending, tie stability, order-preservation on the MusicBrainz path, reset-on-change vs. no-reset-on-same-src). No correctness, security, or data-loss defects were found in the code this phase actually touched.

Two lower-severity items are worth cleaning up: a literal duplicated comment block introduced by this diff in `internal/deezer/search.go`, and a pre-existing (not touched by this diff, but present in a reviewed file) misleading error message in `SearchResultsColumns.tsx` when multiple search sources fail simultaneously.

## Warnings

### WR-01: "showing X results only" message is inaccurate when every other source is also down

**File:** `web/app/components/watchlist/SearchResultsColumns.tsx:91-96`
**Issue:** `SourceColumn`'s error branch always renders:
```tsx
{sourceLabel(sourceName)} is unavailable right now — showing{" "}
{otherSourceNames.map(sourceLabel).join(", ")} results only.
```
`otherSourceNames` is computed purely from `sourceNames.filter((name) => name !== sourceName)` (line 52 in `SearchResultsColumns`) — it never checks whether those other sources are themselves `status === "error"`. If both `musicbrainz` and `deezer` return `status: "error"` in the same response (a real, tested backend scenario — see `internal/httpserver/search_test.go`'s `TestSearch_BothSourcesFailed`), the MusicBrainz column tells the user "showing Deezer results only" while the Deezer column is simultaneously rendering its own "unavailable" message with zero results anywhere on the page. This is a real, user-visible incorrect claim, not just a style nit.

This line is unchanged by this phase's diff (it predates D-10's `secondaryLabel` edit a few lines below it), but the file is in this review's explicit scope and the bug is directly observable in the reviewed code with no additional context needed.

**Fix:** Filter `otherSourceNames` down to sources that are actually healthy before rendering the message, and change copy to handle the both-down case:
```tsx
const healthyOtherSources = otherSourceNames.filter(
  (name) => response.sources[name]?.status === "ok"
)
// ...
{result.status === "error" && (
  <p className="text-label text-muted-foreground">
    {sourceLabel(sourceName)} is unavailable right now
    {healthyOtherSources.length > 0
      ? ` — showing ${healthyOtherSources.map(sourceLabel).join(", ")} results only.`
      : "."}
  </p>
)}
```
(Passing the full `SourceResult` map, or each other source's status, into `SourceColumn`/`SearchResultsColumns` as needed.)

## Info

### IN-01: Duplicated comment block above the D-04 sort call

**File:** `internal/deezer/search.go:92-102`
**Issue:** This phase's diff inserts the same 5-line comment twice, back to back, immediately before the single `slices.SortStableFunc` call:
```go
	// D-04: rank by fan count (popularity) descending. SortStableFunc, not
	// SortFunc -- artists sharing a fan count (very common; an unmatched
	// artist decodes to the zero value) must keep Deezer's own upstream
	// relevance order as the tiebreaker rather than having ties scrambled
	// arbitrarily. The swapped argument order (b, a) produces descending.
	// D-04: rank by fan count (popularity) descending. SortStableFunc, not
	// SortFunc -- artists sharing a fan count (very common; an unmatched
	// artist decodes to the zero value) must keep Deezer's own upstream
	// relevance order as the tiebreaker rather than having ties scrambled
	// arbitrarily. The swapped argument order (b, a) produces descending.
	slices.SortStableFunc(artists, func(a, b Artist) int {
		return cmp.Compare(b.NbFan, a.NbFan)
	})
```
The sort logic itself is correct and appears only once — this is purely a leftover duplicate from editing/copy-paste, not a functional defect — but it reads as an unfinished edit and should be cleaned up before merge.
**Fix:** Delete one of the two identical comment blocks.

---

_Reviewed: 2026-08-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
