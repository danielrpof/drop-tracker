---
phase: 03-external-clients-search
reviewed: 2026-08-07T00:00:00Z
depth: standard
files_reviewed: 20
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
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-08-07
**Depth:** standard
**Files Reviewed:** 20 (source) + test files
**Status:** issues_found

## Summary

This phase adds hand-rolled MusicBrainz and Deezer clients, the `GET /search` fan-out endpoint, and the cron-driven poller. The implementation is disciplined about the things the design docs call out explicitly: rate limiting is enforced on every outbound call through a single `doRequest` seam in both clients, upstream error text is never echoed to callers (verified by dedicated leak tests), context cancellation is respected throughout, and the overlap-guard/graceful-shutdown logic in `poller.go` and `main.go` is well reasoned and heavily tested (including the panic-releases-the-guard and cancel-mid-drain cases).

No blocker-level defects were found. Two warning-level issues affect correctness/robustness of the two external-facing features this phase adds (search relevance and Deezer catalog completeness), and a couple of minor inconsistencies are noted as info.

## Warnings

### WR-01: MusicBrainz search query is not escaped for Lucene special characters

**File:** `internal/musicbrainz/search.go:52-56`
**Issue:** `SearchArtists` builds the MusicBrainz Lucene query by raw string concatenation:
```go
q := url.Values{}
q.Set("query", "artist:"+trimmed)
```
`trimmed` is the caller-supplied search string, forwarded verbatim into MusicBrainz's Lucene query grammar. Lucene treats `" ( ) [ ] { } ^ ~ * ? : \ + - ! AND OR NOT`-style tokens specially. A user searching for an artist name/alias containing any of these (parentheses in a disambiguation-style name, a leading `-` or `!`, an embedded `:`/`"`, etc.) will either get a query MusicBrainz's parser rejects outright (surfacing to the client as `"musicbrainz": {"status": "error"}` — search silently degraded) or, more subtly, a query that means something different than the literal artist name (e.g. a stray `OR`/`AND`/`NOT` token widening or narrowing the match in a way the user never intended). This is unescaped user input flowing into a structured query DSL — the same bug class as the SQL/NoSQL injection family, just against a read-only public search API rather than this service's own data store, so it degrades search relevance/availability rather than crossing a security boundary.

There is no test in `internal/musicbrainz/search_test.go` covering a query containing Lucene metacharacters, which is why this gap wasn't caught.

**Fix:** Escape Lucene special characters before concatenation (or quote the whole term and escape embedded quotes), e.g.:
```go
var luceneSpecial = regexp.MustCompile(`([+\-!(){}\[\]^"~*?:\\]|&&|\|\|)`)

func escapeLucene(s string) string {
    return luceneSpecial.ReplaceAllString(s, `\$1`)
}
...
q.Set("query", "artist:"+escapeLucene(trimmed))
```
and add a test asserting a query like `Wu-Tang (Clan)` round-trips as a literal artist-name search rather than a parse error or unintended boolean query.

### WR-02: Deezer's `ArtistAlbums` has no pagination, unlike MusicBrainz's `ReleaseGroupsByArtist`

**File:** `internal/deezer/albums.go:56-103`, `internal/poller/poller.go:36,259`
**Issue:** `musicbrainz.ReleaseGroupsByArtist` explicitly paginates (bounded, sequential, up to `maxReleaseGroupPages * releaseGroupPageSize` = 1000 release-groups) so a prolific artist's full discography is fetched. `deezer.ArtistAlbums` has no equivalent loop — it issues exactly one request, capped by `clampLimit` at `maxLimit` (100), and the poller calls it with a fixed `deezerAlbumPageSize = 50`:
```go
albums, err := p.dz.ArtistAlbums(ctx, *entry.DeezerID, deezerAlbumPageSize)
```
This phase's poller only logs `item_count` and does no diffing yet (by design, per the package doc comment), but the client-level gap will carry forward unchanged into Phase 4's diff logic, where it becomes a real data-completeness bug: any watched artist with more than 50 albums/singles/EPs on Deezer (the package's own test fixture cites a `"total": 78` example) will have older catalog entries silently never fetched, and if Deezer's ordering ever changes or isn't strictly newest-first, a *new* release could fall outside the 50-item window and never be seen. Nothing in the code or comments documents an assumption about Deezer's default sort order that would justify skipping pagination here (contrast with `ReleaseGroupsByArtist`'s explicit T-03-12 reasoning for *why* MusicBrainz needs bounded pagination).

**Fix:** Either (a) add a bounded pagination loop to `ArtistAlbums` mirroring `ReleaseGroupsByArtist`'s shape (sequential, capped, terminate on short/empty page), or (b) if single-page is intentional because Deezer returns newest-first, add a comment recording that assumption plus a test proving the assumption against a live-verified fixture, so a future reader/Phase-4 implementer doesn't have to rediscover the limitation.

### WR-03: Duplicate hardcoded 10s shutdown timeouts in `main.go`

**File:** `cmd/server/main.go:33,38`
**Issue:** `shutdownTimeout` and `pollDrainTimeout` are two independently declared constants that both happen to be `10 * time.Second`. There's no structural link between them, so a future change to one (e.g. lengthening HTTP shutdown grace) is easy to make without noticing the poller drain budget should probably move too, or vice versa.
**Fix:** Not urgent, but consider either a single shared constant with a comment explaining both timeouts intentionally share a budget, or distinct comments explicitly noting they are allowed to diverge.

## Info

### IN-01: Inconsistent empty-`Type` fallback between search source adapters

**File:** `internal/httpserver/search.go:82-104` (musicBrainzSource) vs `internal/httpserver/search.go:123-149` (deezerSource)
**Issue:** `deezerSource.SearchArtists` defaults an empty upstream `Type` to `"artist"`:
```go
artistType := a.Type
if artistType == "" {
    artistType = "artist"
}
```
`musicBrainzSource.SearchArtists` has no equivalent fallback — `Type: a.Type` is passed through as-is, so a MusicBrainz artist with an unset `type` field (a legitimate, documented MusicBrainz state) surfaces as `"type": ""` in the API response while the equivalent Deezer case surfaces as `"type": "artist"`. A frontend consuming this endpoint has to special-case per-source blank handling even though the response envelope is deliberately source-agnostic (D-01/D-02).
**Fix:** Either apply the same `"artist"` fallback to the MusicBrainz adapter for consistency, or document why the two sources intentionally differ here.

### IN-02: `RunMusicBrainzCycle`/`RunDeezerCycle` duplicate a large amount of boilerplate

**File:** `internal/poller/poller.go:175-277`
**Issue:** The two cycle methods (guard CAS, cycle-ID stamping, `store.List`, per-entry loop with `ctx.Err()` check, per-artist error logging, success logging) are structurally identical except for the source-specific call and the Deezer nil-check branch. This is a reasonable amount of duplication for two methods this size, but as Phase 4 adds a third/fourth source or diff logic, the duplicated skeleton will make it easy for a future edit to fix a bug in one cycle and forget the other (as already nearly happened with the guard/log-field parity between the two — currently they match, but nothing enforces that they continue to).
**Fix:** Consider extracting a shared `runCycle(ctx, source string, running *atomic.Bool, fn func(watchlist.Entry) (int, error))` helper once a third call site exists; not necessary to do now given only two sources.

---

_Reviewed: 2026-08-07_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
