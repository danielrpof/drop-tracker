# Phase 03 — External API Coverage Matrix

**Produced:** 2026-08-07 (plan time)
**Integrations in scope:** MusicBrainz `ws/2` (`https://musicbrainz.org/ws/2`), Deezer public API (`https://api.deezer.com`)

Full coverage is the default. Every capability below starts as `INTEGRATE`; this table is the
**subtraction record**. Each `OPT-OUT` carries a one-line reason. The CONTEXT.md decisions
(D-10, D-11, D-12) already scoped most of these — this table makes those scoping calls visible
as explicit, reasoned opt-outs rather than silent omissions.

Both matrices below start from the same full-coverage baseline. Deezer's opt-outs are decided
independently of MusicBrainz's, not carried over — so a first-class/fallback asymmetry cannot
accumulate unnoticed.

---

## MusicBrainz `ws/2`

| capability | decision | reason |
|---|---|---|
| `GET /ws/2/artist?query=` (artist search) | INTEGRATE | WLST-01 / CLNT-03 — D-11 |
| `GET /ws/2/release-group?artist=` (browse release-groups by artist) | INTEGRATE | CLNT-01 — D-10 |
| `GET /ws/2/artist/{mbid}` (artist lookup) | OPT-OUT | artist search already returns every field the watchlist add-path needs (mbid, name, disambiguation, type); no second consumer |
| `GET /ws/2/release-group/{mbid}` (release-group lookup) | OPT-OUT | not needed yet — browse-by-artist returns the full release-group record this phase logs |
| `GET /ws/2/release?release-group=` (releases + tracklists) | OPT-OUT | not needed yet — Phase 4 DTCT-02 (deluxe/tracklist-change) is the first consumer; D-10 scopes Phase 3 to release-groups |
| `GET /ws/2/recording?artist=` (recordings by artist-credit) | OPT-OUT | not needed yet — Phase 4 DTCT-03 (guest-feature detection) is the first consumer; D-10 defers this query shape |
| `GET /ws/2/recording/{mbid}` (recording lookup) | OPT-OUT | not needed yet — follows the recordings deferral above |
| `inc=` sub-query expansion (`artist-credits`, `releases`, `media`, `tags`, `ratings`) | OPT-OUT | not needed yet — each `inc` bundle exists to serve a deferred detection query; adding them now inflates every response with fields nothing reads |
| `GET /ws/2/work`, `/label`, `/area`, `/place`, `/event`, `/series`, `/instrument`, `/url` | OPT-OUT | explicitly out of scope — the watched entity is the artist (WLST-07 adds producers in v2, not these) |
| `GET /ws/2/discid`, `/isrc`, `/iswc` (identifier lookups) | OPT-OUT | explicitly out of scope — REQUIREMENTS.md excludes audio fingerprinting / ISRC-based dedupe |
| Cover Art Archive (`coverartarchive.org/release-group/{mbid}`) | OPT-OUT | not needed yet — Phase 5 NTFY-01 needs cover art; Deezer's `cover` field already supplies an image for the Deezer half |
| `GET /ws/2/collection` and collection editing | OPT-OUT | explicitly out of scope — requires a MusicBrainz account/auth; single-operator deployable keeps zero upstream credentials |
| Submission/write endpoints (rating, tag, barcode, `POST /ws/2/*`) | OPT-OUT | explicitly out of scope — drop-tracker is a read-only consumer of MusicBrainz; it never writes to the community database |
| OAuth2 authentication | OPT-OUT | explicitly out of scope — every endpoint integrated above is anonymous-readable (live-verified with zero auth headers, 03-RESEARCH.md) |

## Deezer public API

| capability | decision | reason |
|---|---|---|
| `GET /search/artist?q=` (artist search) | INTEGRATE | WLST-01 / CLNT-03 — the Deezer half of D-01's combined search proxy |
| `GET /artist/{id}/albums` (artist albums) | INTEGRATE | CLNT-02 — D-12 |
| `GET /search` (global search) | OPT-OUT | not needed — D-11's equivalent for Deezer scopes search to artists; a global search returns tracks/albums/playlists nothing consumes |
| `GET /search/album`, `/search/track`, `/search/playlist`, `/search/radio`, `/search/user` | OPT-OUT | explicitly out of scope — D-11 rejected searching by release/song title; user and playlist search have no requirement |
| `GET /artist/{id}` (artist lookup) | OPT-OUT | not needed — artist search already returns name, link, picture, `nb_album` |
| `GET /artist/{id}/top` (top tracks) | OPT-OUT | not needed — D-12 rules out track-level data; Deezer is a secondary release signal, not a popularity source |
| `GET /artist/{id}/related`, `/radio`, `/fans`, `/playlists` | OPT-OUT | explicitly out of scope — REQUIREMENTS.md excludes a recommendation engine |
| `GET /album/{id}`, `GET /album/{id}/tracks` | OPT-OUT | not needed yet — D-12 excludes track-level data; if Phase 4 ever wants Deezer-side tracklist diffing it re-decides then |
| `GET /track/{id}` | OPT-OUT | not needed yet — follows the track-level deferral above |
| `GET /chart`, `/genre`, `/editorial`, `/radio` | OPT-OUT | explicitly out of scope — editorial/chart discovery is a recommendation surface, excluded in REQUIREMENTS.md |
| `GET /user/*`, playlist create/modify, favourites | OPT-OUT | explicitly out of scope — requires OAuth and a Deezer account; single-operator deployable holds no user tokens |
| Deezer OAuth2 / `access_token` flows | OPT-OUT | explicitly out of scope — both integrated endpoints are anonymous-readable (live-verified, 03-RESEARCH.md); adding OAuth would introduce a secret this phase has no use for |
| Streaming / playback SDK | OPT-OUT | explicitly out of scope — REQUIREMENTS.md excludes in-app playback (licensing/DRM) |

---

## Notes

- **Both integrated MusicBrainz endpoints and both integrated Deezer endpoints were live-verified**
  during research (03-RESEARCH.md → Code Examples); the response shapes recorded there are the
  fixtures the `httptest.Server`-backed tests use, since CLAUDE.md forbids live external calls in CI.
- **Rate-limit stewardship applies to every `INTEGRATE` row.** One `rate.Limiter` per source
  (D-07) gates *all* outbound traffic to that source — the search proxy and the poll cycle share
  the same client instance, so total outbound rate is bounded regardless of inbound `/search` volume.
- **Deferred rows are not free re-decisions.** A capability marked "not needed yet" is re-decided
  from the same full-coverage baseline when its consumer phase (4, 5) plans — not silently inherited
  as an opt-out.
