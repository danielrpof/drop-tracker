---
phase: 03-external-clients-search
reviewed: 2026-08-07T00:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - cmd/server/main.go
  - go.mod
  - go.sum
  - internal/deezer/albums.go
  - internal/deezer/albums_test.go
  - internal/deezer/client.go
  - internal/deezer/search.go
  - internal/deezer/search_test.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/health_test.go
  - internal/httpserver/search.go
  - internal/httpserver/search_test.go
  - internal/httpserver/server.go
  - internal/httpserver/server_test.go
  - internal/httpserver/watchlist_test.go
  - internal/musicbrainz/client.go
  - internal/musicbrainz/releasegroups.go
  - internal/musicbrainz/releasegroups_test.go
  - internal/musicbrainz/search.go
  - internal/musicbrainz/search_test.go
  - internal/poller/poller.go
  - internal/poller/poller_test.go
findings:
  critical: 0
  warning: 2
  info: 5
  total: 7
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-08-07
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

## Summary

This phase adds hand-rolled MusicBrainz and Deezer clients, the `GET /search` fan-out endpoint, and the cron-driven poller. Traced through both clients' `doRequest` seams, the pagination/overlap-guard logic in `poller.go`, and the shutdown/draining sequence in `cmd/server/main.go`: rate limiting is genuinely enforced on every outbound call (no call site bypasses `doRequest`), upstream error text is never echoed to `GET /search` callers, URL construction correctly escapes path segments (`url.PathEscape`) and query params (`url.Values.Encode`) so there's no path-traversal/redirect vector, context cancellation is honored end-to-end, and the LIFO-defer ordering that drains the poller before closing the DB pool in `main.go` is correct. No crash, data-loss, or security-boundary defect was found.

Two warning-level issues remain: unescaped user input flowing into MusicBrainz's Lucene query grammar, and an asymmetry between the two clients' pagination behavior that will become a real data-completeness bug once Phase 4 builds diff logic on top of it. The rest are maintainability/consistency notes.

## Warnings

### WR-01: MusicBrainz search query is not escaped for Lucene special characters

**File:** `internal/musicbrainz/search.go:52-59`
**Issue:** `SearchArtists` concatenates the caller-supplied, trimmed query directly into MusicBrainz's Lucene query grammar with no escaping:
```go
q.Set("query", "artist:"+trimmed)
```
Lucene gives special meaning to `+ - ! ( ) { } [ ] ^ " ~ * ? : \ && ||` and the bare tokens `AND`/`OR`/`NOT`. An artist name or user-typed query containing any of these (e.g. a disambiguation-style name with parentheses, an embedded `:` or `"`, a leading `-`) either produces a query MusicBrainz's parser rejects — which `handleSearch` degrades to `"status":"error"` for that source, not a client-visible parse error — or silently changes what is matched (a stray `OR`/`NOT` widening/narrowing the search beyond the literal string the user typed). `maxSearchQueryRunes` in `internal/httpserver/search.go:22` bounds length but does nothing about grammar. There is no test in `internal/musicbrainz/search_test.go` exercising a query containing any Lucene metacharacter, so this gap has no regression coverage.
**Fix:** Escape (or quote-and-escape) the term before concatenation:
```go
var luceneSpecial = regexp.MustCompile(`([+\-!(){}\[\]^"~*?:\\]|&&|\|\|)`)

func escapeLucene(s string) string {
    return luceneSpecial.ReplaceAllString(s, `\$1`)
}
...
q.Set("query", "artist:"+escapeLucene(trimmed))
```
and add a fixture-backed test asserting a query like `Wu-Tang (Clan)` round-trips as a literal artist-name search.

### WR-02: `deezer.ArtistAlbums` has no pagination, unlike `musicbrainz.ReleaseGroupsByArtist`

**File:** `internal/deezer/albums.go:56-103`, `internal/poller/poller.go:34-36,259`
**Issue:** `ReleaseGroupsByArtist` explicitly paginates, bounded and sequential, up to `maxReleaseGroupPages * releaseGroupPageSize` (1000) release-groups. `ArtistAlbums` has no equivalent loop: it issues exactly one request, capped by `clampLimit` at 100, and the poller calls it with a fixed page size of 50:
```go
const deezerAlbumPageSize = 50
...
albums, err := p.dz.ArtistAlbums(ctx, *entry.DeezerID, deezerAlbumPageSize)
```
This phase's poller only logs `item_count` and writes nothing (D-04), so nothing breaks *today*. But the client-level gap carries forward unchanged: any watched artist with more than 50 Deezer catalog entries (the package's own fixture cites `"total": 78`) will silently never have its older releases fetched, and Phase 4's diff logic will inherit that truncation as a real data-completeness bug — a release could fall outside the 50-item window and never be detected. Nothing in the code documents an assumption about Deezer's sort order (e.g. "newest first, so a fixed window is safe") that would justify skipping pagination, in contrast to `ReleaseGroupsByArtist`'s explicit reasoning for why MusicBrainz needs the bounded loop it has.
**Fix:** Either add a bounded pagination loop to `ArtistAlbums` mirroring `ReleaseGroupsByArtist`'s shape (sequential, capped, terminate on a short/empty page), or, if a single page is intentional because Deezer returns newest-first, record that assumption in a comment and add a test proving it against a live-verified fixture so Phase 4 doesn't have to rediscover the limitation the hard way.

## Info

### IN-01: Inconsistent empty-`Type` fallback between the two search source adapters

**File:** `internal/httpserver/search.go:82-104` (musicBrainzSource) vs `internal/httpserver/search.go:123-149` (deezerSource)
**Issue:** `deezerSource.SearchArtists` falls back an empty upstream `Type` to `"artist"`:
```go
artistType := a.Type
if artistType == "" {
    artistType = "artist"
}
```
`musicBrainzSource.SearchArtists` has no equivalent — `Type: a.Type` passes through empty MusicBrainz `type` fields (a legitimate MusicBrainz state) as `"type": ""`, while the same situation on Deezer surfaces as `"type": "artist"`. The response envelope is deliberately source-agnostic (D-01/D-02), so this asymmetry pushes a per-source special case onto every consumer of `GET /search`.
**Fix:** Apply the same fallback to `musicBrainzSource`, or add a comment explaining why the two sources are allowed to diverge here.

### IN-02: `RunMusicBrainzCycle`/`RunDeezerCycle` duplicate a large amount of boilerplate

**File:** `internal/poller/poller.go:175-277`
**Issue:** The overlap-guard CAS, cycle-ID stamping, `store.List` call, per-entry loop with `ctx.Err()` check, per-artist error logging, and success logging are structurally identical between the two methods, differing only in the source-specific call and Deezer's nil-`DeezerID` skip branch. Currently the guard/log-field shape matches between the two, but nothing enforces that a future fix applied to one method also lands on the other.
**Fix:** Consider extracting a shared `runCycle(ctx, source string, running *atomic.Bool, fn func(watchlist.Entry) (int, error)) error` helper once a third poll source exists; not necessary at two call sites.

### IN-03: `doRequest`/`cancelReadCloser`/`clampLimit` are duplicated near-verbatim across the two client packages

**File:** `internal/musicbrainz/client.go:33-37,112-164` vs `internal/deezer/client.go:22-37,147-196`
**Issue:** Both packages independently define the same `maxLimit`/`defaultLimit` constants, the same `doRequest` timeout-wrapping/rate-limiting/header-setting sequence, the same `cancelReadCloser` body-wrapper type, and the same `clampLimit` function — differing only in the MusicBrainz variant setting a `User-Agent` header. This is ~70 lines of copy-pasted plumbing per package; a bug fix (e.g. to the cancel-func lifecycle, which is subtle enough to warrant its own paragraph of comments in both files) has to be applied twice by hand.
**Fix:** Not urgent given only two clients exist, but if a third external client is added in a later phase, consider extracting a shared `internal/httpclient` package providing the rate-limited-`doRequest` + `cancelReadCloser` + `clampLimit` seam, parameterized by an optional header-setter.

### IN-04: `shutdownTimeout` and `pollDrainTimeout` are two independently declared 10s constants

**File:** `cmd/server/main.go:33,38`
**Issue:** Both constants are `10 * time.Second` with no structural link between them:
```go
const shutdownTimeout = 10 * time.Second
...
const pollDrainTimeout = 10 * time.Second
```
A future change to one budget (e.g. lengthening the HTTP graceful-shutdown window) is easy to make without noticing the poller drain budget probably should move too, or without an explicit note that they're allowed to diverge.
**Fix:** Either share one constant with a comment explaining both timeouts intentionally use the same budget, or add a comment on each noting they're independently tunable.

### IN-05: `TestSearch_FanOutIsConcurrent` proves concurrency via a fixed sleep, which can flake under a loaded runner

**File:** `internal/httpserver/search_test.go:264-312`
**Issue:** The test starts two blocking source goroutines, then does
```go
time.Sleep(150 * time.Millisecond)
mu.Lock()
got := maxInFlight
mu.Unlock()
```
to give both goroutines "a chance to reach the blocking point" before asserting `maxInFlight >= 2`. On a sufficiently starved CI runner (goroutine scheduling delay, GC pause, or a loaded shared build agent), 150ms may not be enough for both goroutines to have been scheduled and incremented `inFlight` before the assertion runs, producing an intermittent, hard-to-reproduce test failure unrelated to the code under test.
**Fix:** Not urgent (150ms is a generous margin for two goroutines that do no real work before blocking), but a synchronization-based version — e.g. each source goroutine signals a `sync.WaitGroup`/channel on entry, and the test waits on both signals before reading `maxInFlight` — would remove the timing dependency entirely.

---

_Reviewed: 2026-08-07_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
