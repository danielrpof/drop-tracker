---
phase: 12
slug: cleanup-coverart-reset-search-popularity-ranking
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-24
---

# Phase 12 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| third-party catalogue → browser DOM | `src` originates from MusicBrainz/Deezer/`events.cover_art_url` and is written into an `<img src>` attribute | image URL string |
| image load event → component state | the browser's `error` event is the only trigger that sets the failure flag | boolean UI state |
| Deezer `/search/artist` → `internal/deezer` | untrusted third-party JSON crosses here and is decoded into typed Go structs | artist JSON incl. `nb_fan` |
| `internal/deezer` → `internal/httpserver` | in-process call; the returned `[]Artist` is adapted to the wire shape by `deezerSource` | typed `Artist` struct |
| MusicBrainz `/ws/2/artist` → `internal/musicbrainz` | untrusted third-party JSON crosses here and is decoded into typed Go structs | artist JSON incl. `country` |
| `internal/httpserver` → browser | `GET /search`'s JSON body crosses the HTTP boundary; only fields declared on `SearchArtist` may cross | `SearchArtist` wire struct |
| `SearchArtist` → DOM | third-party catalogue strings are rendered into the search-results tree | disambiguation/country text |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-12-01 | Information Disclosure | `CoverArt.tsx` `<img src>` | low | accept | Pre-existing sink, unchanged by this phase; no raw-HTML injection escape hatch introduced. | closed |
| T-12-02 | Denial of Service | `CoverArt.tsx` `useEffect([src])` | low | mitigate | Verified: dependency array is exactly `[src]` (`web/app/components/common/CoverArt.tsx:32-34`); no-op state write is bailed by React; regression test covers the unchanged-src guard. | closed |
| T-12-03 | Tampering | `deezer.Artist.NbFan` decode | low | mitigate | Verified: field is typed `int` (`internal/deezer/search.go:27`), decoded through existing `decodeChecked` path — no untyped map, no new decode path. | closed |
| T-12-04 | Denial of Service | `Client.SearchArtists` sort placement | medium | mitigate | Verified: sort call (`internal/deezer/search.go:103`) sits after `decodeChecked`'s error return (line 84); an HTTP-200 in-body quota error short-circuits before sorting. Regression test `TestSearchArtists_QuotaErrorInBodyWithHTTP200` covers this. | closed |
| T-12-05 | Information Disclosure | `deezer.Artist.NbFan` → HTTP response | low | mitigate | Verified: `NbFan`/fan-count does not appear anywhere in `internal/httpserver/search.go`; kept server-side only as a sort key. | closed |
| T-12-06 | Tampering / Information Disclosure | `SearchResultRow` secondary label in `SearchResultsColumns.tsx` | medium | mitigate | Verified: no `dangerouslySetInnerHTML` in the file; country/disambiguation render as plain JSX text, escaped by React same as before. | closed |
| T-12-07 | Tampering | `musicbrainz.Artist.Country` decode | low | mitigate | Verified: field is typed `string` (`internal/musicbrainz/search.go:45`), decoded through the existing typed-decode path exactly like sibling `Disambiguation`. | closed |
| T-12-08 | Information Disclosure | `SearchArtist` wire struct | medium | mitigate | Verified: `TestSearchArtist_WireShapeKeySet` (`internal/httpserver/search_test.go:647`) pins the exact seven-key JSON set — any silently added field (incl. a future fan-count leak) fails the suite. | closed |
| T-12-09 | Spoofing | same-named artist disambiguation | low | accept | Country fallback is a display hint only, not an identity signal; watchlist adds still resolve identity by MBID via the existing `canAdd` gate, unchanged by this phase. | closed |
| T-12-SC | Tampering | npm/pip/cargo installs | high | accept | Phase runs zero package-manager installs across all three plans (12-RESEARCH.md §Package Legitimacy Audit: not applicable). No install task exists to gate. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-12-01 | T-12-01 | `<img src>` sink pre-existing, unchanged by this phase — no new attack surface | phase plan (12-01) | 2026-08-18 |
| AR-12-02 | T-12-09 | Display-only hint; identity resolution stays keyed on MBID, unaffected | phase plan (12-03) | 2026-08-18 |
| AR-12-03 | T-12-SC | Zero package-manager installs across all three plans in this phase | phase plan (12-01/02/03) | 2026-08-18 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-24 | 10 | 10 | 0 | orchestrator (L1 grep-depth, short-circuit per ASVS level 1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-24
