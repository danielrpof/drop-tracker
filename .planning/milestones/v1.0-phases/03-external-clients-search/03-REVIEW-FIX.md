---
phase: 03-external-clients-search
fixed_at: 2026-08-07T23:45:12Z
review_path: .planning/phases/03-external-clients-search/03-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 03: Code Review Fix Report

**Fixed at:** 2026-08-07T23:45:12Z
**Source review:** .planning/phases/03-external-clients-search/03-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2 (WR-01, WR-02 — `fix_scope: critical_warning`, no CR/BL findings existed)
- Fixed: 2
- Skipped: 0

**Verification environment:** All fixes were made and verified inside an isolated git worktree (`workflow.use_worktrees` was unset, defaulting to `true`), then fast-forwarded onto `main`. `go build ./...`, `go vet ./...`, and `go test ./...` were run from that worktree after each fix and again after both fixes were applied; results are reproducible from `main` at the resulting commits since the worktree's tree state is now identical to `main`'s (no worktree-only dependencies were involved — this is a pure-Go project with no `node_modules`/build-cache divergence risk).

## Fixed Issues

### WR-01: MusicBrainz search query is not escaped for Lucene special characters

**Files modified:** `internal/musicbrainz/search.go`, `internal/musicbrainz/search_test.go`
**Commit:** `8ecc184`
**Applied fix:** Added a `luceneSpecial` regexp and `escapeLucene` helper to `internal/musicbrainz/search.go` that backslash-escapes every Lucene special character (`+ - ! ( ) { } [ ] ^ " ~ * ? : \` plus the two-character operators `&&`/`||`) before the trimmed query is concatenated into `artist:<query>`. `SearchArtists` now calls `escapeLucene(trimmed)` instead of concatenating the raw trimmed string. Added `TestSearchArtists_EscapesLuceneSpecialCharacters` (table-driven, 5 cases including the review's own `Wu-Tang (Clan)` example, a `:`/`"` case, brackets/caret, boolean operators, and a plain-string no-op case) asserting the exact escaped query string sent as the `query` param. Bare Lucene keywords (`AND`/`OR`/`NOT`) were intentionally left unaddressed, matching the review's own suggested fix, which only covers character-level escaping — word-boundary detection for bare boolean keywords was out of scope for this fix.

### WR-02: `deezer.ArtistAlbums` has no pagination, unlike `musicbrainz.ReleaseGroupsByArtist`

**Files modified:** `internal/deezer/albums.go`, `internal/deezer/albums_test.go`
**Commit:** `b9168fc`
**Applied fix:** Rewrote `ArtistAlbums` to paginate via Deezer's `index` offset query parameter, mirroring `musicbrainz.ReleaseGroupsByArtist`'s bounded, sequential loop shape: a new `fetchArtistAlbumsPage` helper issues one page at a time through the existing rate-limited `doRequest` seam, the loop accumulates results and advances `index` by the number of entries actually returned (never the requested page size, so a short page can't skip records), and terminates on either an empty page or once `len(albums) >= total`. A new `maxAlbumPages = 10` constant (mirroring `musicbrainz.maxReleaseGroupPages`) bounds the loop against a malformed/hostile `total` from upstream. `ArtistAlbums`'s public signature (`artistID string, limit int`) is unchanged — `limit` now sets the per-page size rather than a hard ceiling on total results — so `internal/poller/poller.go`'s `AlbumSource` interface and call site needed no changes.

Test changes: `TestArtistAlbums_DecodesFixture` was adjusted so its httptest handler only serves `habibtiAlbumsFixture` (whose live-verified `total: 78` would otherwise now trigger real pagination) on the first page (`index=0`) and an empty continuation page afterward, keeping that test scoped to first-page field decoding. Added a new pagination test suite (`TestArtistAlbums_PaginationCollectsAllPagesInOrder`, `_PacingAppliesAcrossPages`, `_StopsAsSoonAsTotalReached`, `_PageCapStopsRunawayTotal`, `_ZeroEntryPageTerminatesLoop`, `_MidFetchErrorStopsWithNoRetry`, `_CancellationBetweenPagesAborts`) mirroring `internal/musicbrainz/releasegroups_test.go`'s existing pagination coverage shape. `TestArtistAlbums_PaginationCollectsAllPagesInOrder` specifically proves the fix against the review's own cited evidence: a simulated 78-total-album artist (matching the live-verified fixture) now has every album fetched across 8 pages instead of being silently truncated at a single fixed window.

`go build ./...`, `go vet ./...`, and `go test ./...` all pass after both fixes, with no regressions in any other package (`internal/poller`, `internal/httpserver`, etc.).

## Skipped Issues

None — both in-scope findings were fixed.

---

_Fixed: 2026-08-07T23:45:12Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
