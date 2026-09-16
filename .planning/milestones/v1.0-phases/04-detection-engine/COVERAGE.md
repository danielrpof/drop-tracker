# API Coverage — MusicBrainz / Deezer (Phase 4: Detection Engine)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.
>
> Scope: the MusicBrainz `ws/2` and Deezer public-API capability surface relevant to
> detecting new releases, guest features, and deluxe/tracklist changes for watchlisted
> artists (DTCT-01 through DTCT-05). Capabilities already delivered in Phase 3 are listed
> so the surface is complete, not just the delta.

## MusicBrainz

| capability | decision | reason |
|---|---|---|
| musicbrainz: artist search (`/ws/2/artist?query=`) | INTEGRATE | already built Phase 3 (WLST-01 / CLNT-03) — supplies the artist MBID every detection path keys on |
| musicbrainz: release-groups-by-artist browse (`/ws/2/release-group?artist=`) | INTEGRATE | already built Phase 3 — the DTCT-01 `new_release` diff source, no new endpoint needed |
| musicbrainz: release-detail by release-group (`/ws/2/release?release-group=&inc=media`) | INTEGRATE | D-01, new this phase — the DTCT-02 track-count signal; `media[].track-count` summed across discs |
| musicbrainz: recording-by-artist-credit browse (`/ws/2/recording?artist=&inc=artist-credits`) | INTEGRATE | D-05, new this phase — the DTCT-03 guest-feature source; positional credit filter applied client-side (D-06) |
| musicbrainz: cover-art-archive URL construction (`coverartarchive.org/release-group/{mbid}/front`) | INTEGRATE | D-12 display snapshot — deterministic URL, no extra HTTP call (04-RESEARCH.md Pitfall #6) |
| musicbrainz: self-imposed rate limiting + mandatory User-Agent (`doRequest`) | INTEGRATE | already built Phase 3 (D-07) — both new fetch methods route through the same helper (ASVS V13) |
| musicbrainz: bounded pagination (`limit`/`offset`, page ceiling) | INTEGRATE | already built Phase 3 — D-05 reuses maxPages=10 x pageSize=100 for the recording browse |
| musicbrainz: artist lookup by MBID (`/ws/2/artist/{mbid}`) | OPT-OUT | artist master data (`name`, `disambiguation`, `image_url`) is captured at watchlist-add time from search and refreshed by `UpsertArtist`; detection needs no per-cycle artist re-lookup, and one extra request per artist per cycle would spend limiter budget for zero detection signal |
| musicbrainz: release-group lookup with `inc=artist-credits` (release-group-level guest credits) | OPT-OUT | D-06 scopes guest detection to the recording level positionally; release-group-level credits would double-count the same collaboration as both a `new_release` and a `guest_feature` |
| musicbrainz: `inc=recordings` on the release-detail fetch (per-track listing) | OPT-OUT | D-01 needs only the total track count, which `inc=media` already carries; `inc=recordings` inflates every response body without changing the DTCT-02 signal |
| musicbrainz: `/ws/2/release-group?artist=&type=` server-side type filter | OPT-OUT | already decided Phase 3 (`releasegroups.go` doc comment) — filtering at fetch time makes a later preference change silently invisible until a full refetch; D-17 filters at detection time instead |
| musicbrainz: label / work / area / place / event browse endpoints | OPT-OUT | out of domain — this project watches artists (WLST-07 producers are v2, not v1) |
| musicbrainz: OAuth-authenticated write endpoints (collections, tags, ratings) | OPT-OUT | drop-tracker is read-only against MusicBrainz; no write capability is in scope for any v1 requirement |

## Deezer

| capability | decision | reason |
|---|---|---|
| deezer: artist search (`/search/artist`) | INTEGRATE | already built Phase 3 (WLST-01 / CLNT-03) |
| deezer: artist-albums (`/artist/{id}/albums`) | INTEGRATE | already built Phase 3; D-15 makes it the second `new_release` source, keyed on Deezer's own numeric album-id namespace via the `source` discriminator |
| deezer: album `cover` field as the D-12 snapshot cover art | INTEGRATE | the artist-albums response already carries it (`[VERIFIED: live Deezer response, 03-RESEARCH.md:415]`) — no extra call |
| deezer: album `record_type` as the D-17 release-type filter input | INTEGRATE | `album`/`single`/`ep` map directly onto `watchlist.ReleaseTypes` |
| deezer: self-imposed rate limiting (~50 req/5s) | INTEGRATE | already built Phase 3 (D-07) — detection adds no Deezer requests beyond the existing per-artist album fetch |
| deezer: track/credit-level data | OPT-OUT | Deezer has no track/credit-level fetch capability at all — D-08 |
| deezer: tracklist/deluxe detection | OPT-OUT | `Album.Tracklist` is a URL, not real track data — D-03 |
| deezer: album lookup by id (`/album/{id}`) | OPT-OUT | every D-12 snapshot field (title, cover, release_date, record_type) is already present on the artist-albums response; one extra request per album per cycle would multiply Deezer request volume for zero new signal |
| deezer: `/artist/{id}` lookup | OPT-OUT | same rationale as the MusicBrainz artist lookup — artist master data is captured at add time, not re-fetched per cycle |
| deezer: OAuth / user-library endpoints | OPT-OUT | multi-user auth is explicitly Out of Scope (REQUIREMENTS.md); drop-tracker reads only public catalog endpoints |
| cross-source: Deezer <-> MusicBrainz entity reconciliation | OPT-OUT | explicitly Out of Scope (REQUIREMENTS.md) — the `source` column keeps the two ID namespaces separate rather than reconciling them |

## Coverage summary

- MusicBrainz capabilities enumerated: 13 (7 INTEGRATE, 6 OPT-OUT)
- Deezer capabilities enumerated: 11 (5 INTEGRATE, 6 OPT-OUT)
- New endpoints introduced this phase: 2 (`/ws/2/release?release-group=`, `/ws/2/recording?artist=`)
- New external dependencies introduced this phase: 0

*Produced during Phase 4 planning, 2026-08-08.*
