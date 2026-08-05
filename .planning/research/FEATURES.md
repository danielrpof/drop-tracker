# Feature Research

**Domain:** Personal/self-hosted music release tracker (hip-hop / reggaeton / R&B focus) — artist watchlist + new-release/guest-feature/deluxe-reissue notifier
**Researched:** 2026-08-04
**Confidence:** MEDIUM (data-model claims cross-checked against MusicBrainz official docs and multiple independent client implementations; comparable-tool feature claims are LOW-confidence web search but corroborated across 6+ independent products)

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = the tool doesn't do its one job. All of these are already captured as Active requirements in PROJECT.md — this table validates and details them.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Search-to-add artist (live catalog search proxy) | Every comparable tool (MusicHarbor, MusicButler, crabhands, BEEPR, deemon) lets you type an artist name and pick the right catalog entry rather than typing a raw ID | LOW-MEDIUM | Proxy MusicBrainz `artist` search + Deezer `/search/artist`; return enough disambiguation (name, disambiguation comment, image/country) to avoid adding the wrong "Bad Bunny" |
| Watchlist CRUD (add/remove/list artist) | Baseline for any "track these things" tool; PROJECT.md already locks this in | LOW | Standard REST CRUD over Postgres; dedupe by external catalog ID, not by name string (name collisions are common — reggaeton has many same-named acts) |
| Scheduled polling of watched artists | The core mechanic — without polling nothing gets detected | LOW-MEDIUM | robfig/cron per PROJECT.md; needs per-source (MB vs Deezer) rate-limit awareness — MusicBrainz enforces ~1 req/sec |
| New-release detection (own artist's new album/single) | This is the product's core promise | MEDIUM | See "Diffing Data Model" section below — key off release-group-id (MB) / album-id (Deezer), not release-id |
| Guest-feature detection (artist appears on someone else's track) | **Critical differentiator for this genre** — hip-hop/reggaeton/R&B fans track features as heavily as an artist's own drops (e.g. a Bad Bunny verse on someone else's single) | MEDIUM-HIGH | Requires parsing artist-credit (MB) or `contributors` (Deezer) on tracks that are NOT the watched artist's own release — a fundamentally different query pattern than "new release by artist X" |
| Deluxe/tracklist-change detection | Deluxe reissues ("Album (Deluxe)", "Complete Edition") are extremely common release patterns in this genre and are a real, wanted signal — not just noise to filter | MEDIUM-HIGH | Must distinguish "new tracks added to an existing release-group" from "cosmetic remaster/reissue" — see false-positive section |
| Discord webhook notification | Locked in PROJECT.md as the notification sink | LOW | Simple POST to webhook URL; batch multiple detections per poll cycle into one message to avoid spam |
| Idempotent "seen" store / no duplicate alerts | Any polling-based notifier that re-alerts on the same release destroys user trust immediately | LOW-MEDIUM | Diff engine must be safe to run repeatedly against the same source data (poll failures, retries) without re-notifying |
| `/health` liveness/readiness endpoint | Standard operational expectation, also explicit PROJECT.md requirement | LOW | Trivial handler; check DB connectivity |
| Release metadata on alert (title, artist, cover art, release date, type) | A bare "new release detected" message with no context is useless — user needs to know *what* dropped | LOW | Already implied by diff engine output; just needs to be carried through to the Discord embed |

### Differentiators (Competitive Advantage)

Features that set the product apart within this specific niche. Should map to Core Value (reliable detection + CI/CD showcase), not scope creep.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Feature-alert as a distinct alert type from own-release alert | Most comparable tools (MusicHarbor, MusicButler, crabhands, BEEPR) only track an artist's *own* catalog additions — none surface "artist X appeared as a guest on artist Y's new track" as a first-class alert. This is the single most genre-relevant gap to fill for hip-hop/reggaeton/R&B, where guest verses are a primary consumption driver | MEDIUM-HIGH | Depends on guest-feature detection (table stakes) being solid; the differentiator is *surfacing it distinctly* in the notification (e.g. "🎤 Feature Alert" vs "💿 New Release") |
| Release-type filtering per watchlist entry (album/single/EP/deluxe/compilation) | Lidarr proves this pattern works well (default "studio albums only" profile); lets a user watch an artist for albums only and skip singles-spam, or vice versa | LOW-MEDIUM | Requires MusicBrainz `primary-type`/`secondary-type` (album, single, EP + secondary: compilation, remix, live) on release-groups; Deezer has a coarser `record_type` field |
| Per-artist notification preferences (mute deluxe noise, alert only on genuinely new tracks) | Reduces alert fatigue from reissue/remaster churn without losing signal on genuine deluxe track additions | MEDIUM | Enhancement on top of watchlist CRUD — add a preferences column/table; enhances but doesn't replace the deluxe-detection diff logic |
| Dual-source reconciliation (MusicBrainz + Deezer cross-check) | MusicBrainz community-edited data can lag on same-day major-label hip-hop/reggaeton drops; Deezer's commercial catalog is often faster/more current for these genres, but lacks a release-group concept. Cross-checking reduces both false negatives (MB hasn't caught up) and false positives (Deezer flat-album churn) | HIGH | Real complexity add — good "differentiator" but also the best candidate to defer past v1 (see MVP section) |
| Per-artist audit/history view of what was detected and why | Nobody in the comparable-tool landscape exposes *why* something was flagged (which field changed, which release-group). For a CI/CD/DevOps portfolio piece this is also a great demo of the diff engine's correctness/observability | LOW-MEDIUM | Natural extension of the "seen" store — just needs a read view over stored diff events, no new detection logic |
| Configurable poll interval per source | MusicBrainz and Deezer have different rate limits and different data freshness characteristics; letting Deezer poll faster than MusicBrainz (or vice versa) is a legitimate operational feature | LOW | robfig/cron already supports this; just needs to be per-source config rather than one global interval |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but would balloon scope well past a v1 CI/CD portfolio piece. Every comparable real-world tool researched (deemon, Lidarr, Releasarr) eventually grows toward at least one of these — deliberately not building them here keeps the project's actual point (pipeline maturity) intact.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Recommendation engine ("you might also like") | Feels like a natural extension of "tracking music you like" | Recommendation is a genuinely hard, orthogonal ML/data problem; adds a whole new domain of complexity with zero relevance to the CI/CD practice goal | Stick to explicit watchlist — user decides who to track, tool never guesses |
| Auto-download / acquisition (deemix/*arr-stack style, as deemon and Releasarr both do) | "Since we already detect new releases, why not grab them" | Legal gray area, pulls in an entirely separate download/indexer subsystem (SABnzbd, indexers, file management), massively expands attack surface and scope | Notification-only; user acts on the Discord alert manually via their own streaming service |
| Multi-user auth / accounts / SSO | Feels like a "real product" needs logins | PROJECT.md scopes this as a single deployable service for one operator (personal/portfolio use); OAuth/session management/RBAC is a large, orthogonal complexity spike that doesn't showcase CI/CD skills | Single shared instance, no per-user accounts; if isolation is ever needed, a simple API key is sufficient — not full auth |
| In-app music playback / streaming integration | "Why alert me and make me open Spotify separately" | Requires licensed streaming SDKs, playback state, DRM concerns — an entirely different product category | Alert links out to the source (Deezer/MusicBrainz page); playback stays in the user's existing streaming app |
| Mobile push notifications / native app | Comparable tools (BEEPR, MusicHarbor) lead with push notifications | Requires APNs/FCM infrastructure, device token management, a mobile client — none of which exercises the DevOps pipeline this project exists to practice | Discord webhook is the notification sink per PROJECT.md; Discord already has a mobile app that receives it |
| Full historical backfill / mirroring an artist's entire discography | Feels complete/thorough | Turns the tool into a metadata warehouse instead of a forward-looking watcher; massively increases initial poll volume and storage for zero notification value | Seed the "seen" store with current state on first poll (baseline), only alert on changes going forward |
| Audio fingerprinting / ISRC-based dedupe for false-positive reduction | Seems like the "correct" rigorous way to distinguish a true remaster from a genuinely new recording | High implementation cost (needs audio analysis or a third-party fingerprinting API) for a problem that release-group-id-based diffing already handles well enough for a v1 portfolio tool | Key diffing off MusicBrainz release-group-id + track-list-shape; accept some irreducible noise as a known, documented limitation |
| Producer/label tracking as a first-class watchlist entity | Natural extension of "watch things related to music I like" | Already explicitly deferred in PROJECT.md's Out of Scope; adding it now would require a different data model per entity type | Stays as documented future work, not v1 |

## Diffing Data Model: What "New Release" Actually Requires

This directly determines the diff engine's schema and query shape.

**MusicBrainz** separates two levels that must both be understood:
- **release-group** — the abstract "album" concept with a stable ID and a `primary-type`/`secondary-type` (Album, Single, EP, + secondary types like Compilation, Remix, Live). This is the correct unit to diff against for "is this a new release" — a new release-group ID appearing for a watched artist is a genuine new-release signal.
- **release** — a specific edition (country, format, deluxe/bonus-disc, remaster) that belongs to a release-group. Deluxe editions with bonus tracks are their own release, but MusicBrainz deliberately keeps them **inside the same release-group** as the standard edition — as do plain reissues and remasters. This means: a new *release* inside an *existing* release-group is the deluxe/tracklist-change signal, not a new-release signal.
- Guest features are represented via **artist-credit** with a `joinphrase` (e.g. `" feat. "`) attached at the release-group, release, and recording (track) level. Detecting "artist X guested on someone else's track" means querying/browsing recordings where X appears in the artist-credit list but is not the primary/first credited artist — a different query shape than "browse release-groups by artist."

**Deezer** has a flatter model: `/album` and `/track` endpoints, no release-group equivalent. Track objects carry `contributors` (feature credits) when detail fields are requested, and album objects carry `release_date` and cover art. Because there's no grouping concept, a deluxe reissue on Deezer typically shows up as a **new album ID** with an overlapping-but-larger tracklist rather than being linked back to the original album. This is the main reason MusicBrainz's release-group model is the better backbone for the diff engine's "is this genuinely new" decision, with Deezer used as a secondary/faster-updating signal source.

**Recommended diff keys:**
1. New release-group ID for a watched artist (MB) → **new release** alert.
2. New release ID inside an *existing* release-group, with a track-list superset of what's stored → **deluxe/tracklist-change** alert.
3. New recording where the watched artist appears in artist-credit but is not the primary artist, and the release/release-group is not already attributed to them → **guest-feature** alert.
4. Deezer album ID new for the artist, tracklist not previously seen (title/duration fuzzy match against MB) → secondary confirmation signal / fallback when MB is slow to update.

## False-Positive / False-Negative Risk in Diffing

No authoritative "how real tools solve this" documentation was found (LOW confidence on this specific sub-question) — but the underlying data-model facts (MEDIUM confidence, MusicBrainz official docs) plus how Lidarr is documented to behave give a reasonably solid basis for the following:

- **"Remaster" is an unreliable label.** It's effectively a marketing term with no consistent metadata backing — some remasters change little, some non-remaster reissues change audio. Do not use free-text title matching ("contains 'remaster'") as a filter; it will both over- and under-trigger.
- **Reissues/remasters reuse the same release-group by design** (per MusicBrainz style guides), which is precisely why keying off release-group-id rather than release-id is the correct false-positive mitigation — a remaster or regional reissue does not create a new release-group, so it will not fire a "new release" alert if the diff engine only watches release-group IDs.
- **Deluxe reissues are the genuinely ambiguous case**, because they legitimately add new content (bonus tracks) while staying in the same release-group. The mitigation is to diff at the **track-list level within a release-group** (new track titles/recording IDs appearing) rather than at the release-group level alone — this is what distinguishes "cosmetic reissue, no new tracks" (suppress) from "deluxe added 4 new songs" (alert, and label it as a deluxe/tracklist-change rather than a plain new-release).
- **Deezer-only false positives**: because Deezer albums are flat (no release-group), a Deezer-sourced "new album" signal for a reissue with no MusicBrainz correlate is a real false-positive risk. Mitigate by treating Deezer detections as provisional and cross-checking against MusicBrainz's release-group grouping when available, only auto-confirming Deezer-only detections after a short debounce window (e.g. still present on next poll) to filter transient/duplicate catalog entries.
- **Realistic residual risk to document, not solve**: some irreducible noise will remain (a real new song with a title change between polls, a mis-tagged deluxe that MusicBrainz editors haven't yet folded into the right release-group). Given the portfolio/CI-CD framing, the correct engineering response is to log and surface these as known diff-engine limitations (and write tests around the specific scenarios above) rather than chase perfect precision with audio fingerprinting or ISRC reconciliation (see Anti-Features).

## Feature Dependencies

```
[MusicBrainz/Deezer HTTP clients]
    └──requires──> [Search-to-add proxy]
    └──requires──> [Diff engine (all detection types)]

[Watchlist CRUD]
    └──requires──> [DB schema / migrations]

[Diff engine: new-release detection]
    └──requires──> [Watchlist CRUD]  (source of artists to poll)
    └──requires──> ["Seen" store schema keyed on release-group-id / album-id]
    └──requires──> [MusicBrainz/Deezer HTTP clients]

[Diff engine: guest-feature detection]
    └──requires──> [Diff engine: new-release detection]  (same client/scheduler plumbing)
    └──requires──> [artist-credit / contributors parsing]  (harder query shape, higher complexity)

[Diff engine: deluxe/tracklist-change detection]
    └──requires──> [Diff engine: new-release detection]
    └──requires──> [Track-list-level diffing within an existing release-group]

[Discord notifier]
    └──requires──> [Diff engine output (any detection type)]

[Per-artist notification preferences] ──enhances──> [Watchlist CRUD]
[Release-type filtering] ──enhances──> [Watchlist CRUD] and [Diff engine: new-release detection]
[Dual-source reconciliation (MB+Deezer)] ──enhances──> [Diff engine: new-release detection]
    └──conflicts with──> [v1 simplicity — real complexity add, best deferred]

[Audit/history view] ──requires──> ["Seen" store] + [Diff engine event logging]
```

### Dependency Notes

- **New-release detection requires Watchlist CRUD + "seen" store schema:** the diff engine has nothing to poll without a watchlist, and nothing to compare against without a persisted prior-state store — both must land before any detection logic is written.
- **Guest-feature and deluxe/tracklist detection both build on new-release detection's plumbing** (same HTTP clients, same scheduler, same "seen" store) but require materially harder query/diff logic (artist-credit parsing; track-list-level diffing within a release-group) — plan these as follow-on phases, not bundled with the first pass at new-release detection.
- **Per-artist notification preferences and release-type filtering are enhancements, not blockers** — they can land after the core diff engine works, as small additive schema changes.
- **Dual-source reconciliation conflicts with v1 simplicity** — it's a genuine differentiator but adds enough complexity (cross-source entity matching, confidence scoring) that it should be explicitly deferred rather than attempted alongside the core detection logic.

## MVP Definition

### Launch With (v1)

Minimum viable product — matches PROJECT.md's Active requirements exactly; this is what validates the concept and is the CI/CD pipeline's actual payload.

- [ ] Search-to-add artist (MusicBrainz + Deezer search proxy) — without it the watchlist can only be built by hand-entering opaque IDs
- [ ] Watchlist CRUD (add/remove/list) — the whole tool's reason to exist
- [ ] Scheduled polling (robfig/cron) — the mechanism that makes detection possible at all
- [ ] New-release detection keyed on release-group-id (MB) — the core, highest-value detection type; ship this before the harder guest/deluxe cases
- [ ] Discord webhook notification with release metadata (title, artist, cover, date, type) — closes the loop; without it detection is invisible
- [ ] `/health` endpoint — required for the CI/CD deploy story, not just a feature nicety

### Add After Validation (v1.x)

Add once the core new-release path is proven reliable in practice (no duplicate alerts, no missed obvious releases).

- [ ] Guest-feature detection as a distinct alert type — trigger: core new-release detection is stable and the genre-specific value proposition needs to be demonstrated
- [ ] Deluxe/tracklist-change detection — trigger: once track-list-level diffing within a release-group is needed anyway for feature detection, extend it to deluxe reissues
- [ ] Per-artist notification preferences / release-type filtering — trigger: once real polling data shows enough alert volume/noise to justify filtering controls
- [ ] Audit/history view of diff events — trigger: useful once there's enough detection history to make a timeline meaningful, and doubles as a demo of pipeline observability

### Future Consideration (v2+)

Defer until the core tool has proven itself and the CI/CD pipeline goals are already met.

- [ ] Dual-source (MusicBrainz + Deezer) reconciliation — defer: real complexity (entity matching across sources) with a narrower payoff than getting core detection right first
- [ ] Producer tracking as a watchlist entity — defer: already explicitly out-of-scope in PROJECT.md, different data model per entity type

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|----------------------|----------|
| Search-to-add proxy | HIGH | LOW | P1 |
| Watchlist CRUD | HIGH | LOW | P1 |
| Scheduled polling | HIGH | LOW | P1 |
| New-release detection (release-group-id) | HIGH | MEDIUM | P1 |
| Discord notification | HIGH | LOW | P1 |
| `/health` endpoint | MEDIUM (high for CI/CD story) | LOW | P1 |
| Guest-feature detection | HIGH (genre-specific) | MEDIUM-HIGH | P2 |
| Deluxe/tracklist-change detection | MEDIUM-HIGH | MEDIUM-HIGH | P2 |
| Per-artist notification preferences | MEDIUM | MEDIUM | P2 |
| Release-type filtering | MEDIUM | LOW-MEDIUM | P2 |
| Audit/history view | MEDIUM | LOW-MEDIUM | P2 |
| Dual-source (MB+Deezer) reconciliation | MEDIUM | HIGH | P3 |
| Recommendation engine | LOW (out of project's core value) | HIGH | Anti-feature |
| Auto-download/acquisition | LOW (out of project's core value) | HIGH | Anti-feature |
| Multi-user auth/SSO | LOW (not scoped by PROJECT.md) | MEDIUM-HIGH | Anti-feature |

**Priority key:**
- P1: Must have for launch (matches PROJECT.md Active requirements)
- P2: Should have, add once P1 is proven reliable
- P3: Nice to have, defer past v1

## Competitor / Comparable-Tool Feature Analysis

| Feature | MusicHarbor / MusicButler / crabhands / BEEPR (consumer alert apps) | deemon (Deezer CLI monitor) / Lidarr (MB-backed *arr stack) | Our Approach |
|---------|------------------------------------------------------------------|---------------------------------------------------------------|--------------|
| Add artist | Search-based, source-catalog-backed | deemon: name/ID/URL; Lidarr: MusicBrainz search | Search-proxy against MusicBrainz + Deezer (matches Lidarr/deemon pattern, not a local-only list) |
| Detection unit | Not documented at data-model level (proprietary) | Lidarr: MusicBrainz release-group = "Album" | Same as Lidarr — release-group-id as the new-release key |
| Guest features | Not supported by any researched consumer app | Not supported | **Differentiator** — first-class guest-feature alert type |
| Deluxe/reissue handling | Not documented | Lidarr: filters by Metadata Profile (e.g. studio-only), doesn't specifically flag deluxe additions | Track-list-level diff within release-group, surfaced as its own alert type |
| Notification channel | Push (mobile), email | deemon: email; Releasarr: unclear | Discord webhook only (matches PROJECT.md scope) |
| Auto-download | No (consumer apps) | Yes (deemon via deemix, Releasarr via SABnzbd/indexers) | **Explicitly not built** (anti-feature) |
| Release-type filtering | Some (MusicHarbor Pro: label following, advanced filters) | Lidarr: yes, per-artist Metadata Profile | v1.x addition, per-artist |
| Self-hosted / open | No (consumer apps are proprietary) | Yes (deemon, Lidarr, Releasarr all self-hosted OSS) | Self-hosted, single Go binary (matches the OSS self-hosted category, not the consumer-app category) |

## Sources

- [MusicBrainz — Release](https://musicbrainz.org/doc/Release)
- [MusicBrainz — Release Group](https://musicbrainz.org/doc/Release_Group)
- [MusicBrainz — Release Group / Type](https://musicbrainz.org/doc/Release_Group/Type)
- [MusicBrainz — Artist Credits](https://musicbrainz.org/doc/Artist_Credits)
- [MusicBrainz API docs](https://musicbrainz.org/doc/MusicBrainz_API)
- [browniebroke/deezer-python — contributor parsing commit](https://github.com/browniebroke/deezer-python/commit/bd02ec41c20ddbff2cc399f496f64b8f095c4854)
- [PublicAPI — Deezer API overview](https://publicapi.dev/deezer-api)
- [MusicHarbor overview](https://mwm.ai/apps/musicharbor-track-new-music/1440405750)
- [MusicButler](https://www.musicbutler.io/)
- [crabhands](https://www.crabhands.com/)
- [BEEPR](https://beeprapp.com/)
- [FriendsTapes](https://www.friendstapes.com/)
- [deemon (PyPI)](https://pypi.org/project/deemon/)
- [deemon (GitHub)](https://github.com/digitalec/deemon)
- [Releasarr (GitHub)](https://github.com/Makario1337/Releasarr)
- [Lidarr — Concepts (Servarr Wiki)](https://wiki.servarr.com/lidarr/concepts)
- [Lidarr — FAQ (Servarr Wiki)](https://wiki.servarr.com/lidarr/faq)
- [Lidarr — Don't automatically monitor old albums (GitHub issue #2031)](https://github.com/Lidarr/Lidarr/issues/2031)

---
*Feature research for: Music release tracker (hip-hop/reggaeton/R&B), drop-tracker portfolio project*
*Researched: 2026-08-04*
