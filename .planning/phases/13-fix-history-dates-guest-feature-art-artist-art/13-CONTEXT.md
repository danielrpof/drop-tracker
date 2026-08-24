# Phase 13: Fix History Dates, Guest-Feature Art & Artist Art - Context

**Gathered:** 2026-08-24
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase closes three post-Phase-12 display/data bugs, all confirmed by reading the actual code (not assumed):

1. **History dates missing** — `new_release` cards already render `release_date` (`NewReleaseBody` in `EventCard.tsx`). `deluxe_change` events HAVE `release_date` populated in the DB (`internal/detection/musicbrainz.go:392`) but `DeluxeChangeBody` never renders it — pure frontend gap. `guest_feature` events have NO `release_date` in the DB at all — `detectGuestFeatures` (`internal/detection/musicbrainz.go:203-211`) never sets it, because MusicBrainz's recording browse response carries no release/date info to source it from.
2. **Guest-feature album art missing** — same root cause as the date gap: `detectGuestFeatures`'s `InsertEvent` call never sets `CoverArtUrl`, unlike `new_release`/`deluxe_change` which both call `coverArtURLForReleaseGroup(mbid)`. `RecordingsByArtist` (`internal/musicbrainz/recordings.go`) returns only `MBID`/`Title`/`ArtistCredit` per recording — no release-group linkage to build a cover art URL from.
3. **MusicBrainz artist art not rendering** — there is no MusicBrainz↔Deezer artist matching anywhere in the codebase today. Every artist is added via a MusicBrainz-only search result (`SearchResultsColumns.tsx` explicitly disables adding via Deezer results — "this project has no cross-source identity resolution between the two catalogues"), and MusicBrainz search results never carry an `image_url` (MusicBrainz has no artist images). `watchlist.tsx`'s `handleAddSearchResult` passes `imageUrl: result.image_url ?? undefined`, which is always `undefined` for a MusicBrainz add — so `artists.image_url` is NULL forever for every artist, confirmed against `internal/httpserver/watchlist.go`'s add handler (accepts but never derives `ImageURL`). This absorbs backlog Phase 999.2 in full.

No new capabilities beyond these three (plus the artist-art backfill needed to make #3 actually visible on existing watchlist rows) are in scope.

</domain>

<decisions>
## Implementation Decisions

### Guest-feature date & cover-art sourcing
- **D-01:** For each genuinely NEW guest-feature recording (i.e. only on insert, not for every row `RecordingsByArtist` returns), do an extra per-event MusicBrainz lookup: `GET /ws/2/recording/{mbid}?inc=releases+release-groups`. Use it to get a release-group MBID (cover art via the existing `coverArtURLForReleaseGroup` helper — no new cover-art mechanism) and a release date. Shares the existing MusicBrainz rate limiter; cost is bounded to actual new guest-feature insertions per cycle, not the full browse result set. — **Reversibility:** reversible — an additional read-only fetch, no schema or contract change.
- **D-02:** When a recording has releases with different dates (single, then album/reissue), use the **earliest** release date among them — matches the `FirstReleaseDate` semantics already used for `new_release` events, and reflects when the guest feature was first actually released.
- **D-03:** When the lookup finds no releases at all for a recording (rare — unreleased/promo-only credits), behave exactly like today: `CoverArt`'s existing placeholder, and `event.release_date` stays null → renders `NewReleaseBody`'s existing "Release date unknown" fallback text.
- **Note for planner:** `internal/musicbrainz` will need a new client method (e.g. `ReleasesForRecording` or similar, analogous to `ReleasesByReleaseGroup`) to make this MusicBrainz call. `internal/detection/musicbrainz.go`'s `detectGuestFeatures` is where the `InsertEvent` call gets the new `ReleaseDate`/`CoverArtUrl` args wired in (mirroring the `new_release`/`deluxe_change` pattern at lines 102-113 and 380-393).

### Deluxe-change date rendering
- **D-04:** Add the release date to `DeluxeChangeBody` (`web/app/components/history/EventCard.tsx`) on the same line, before the track-count delta: `"{date} · {prev} → {current} tracks"` — matches `NewReleaseBody`'s existing `"{release_type} · {date}"` separator style for visual consistency across all three card types.
- **D-05:** Fall back to the same `"Release date unknown"` text `NewReleaseBody` already uses when `release_date` is null — no new fallback copy.
- **Note for planner:** This is a frontend-only change — `release_date` is already populated in the DB and already flows through `EventItem` in `web/app/lib/api.ts` (confirm the TS type includes it; if `EventItem` doesn't expose `release_date` for the frontend today, that's part of this fix too).

### Artist-art backfill scope
- **D-06:** Artist-art matching applies BOTH to new adds going forward AND as a one-time backfill over artists already on the watchlist with `image_url IS NULL`.
- **D-07:** The backfill runs as a one-time migration/startup pass, sweeping every artist with `image_url IS NULL` once when the app starts running this phase's code — not folded into the existing poll cycle, not a separate scheduled job.
- **Note for planner:** Backfill scope is artist rows only (`artists.image_url`/`artists.deezer_id`) — this phase does not touch event/detection logic, per the same boundary backlog Phase 999.2 originally drew against PROJECT.md's rejection of full dual-source reconciliation.

### Artist match confidence & tie-breaking
- **D-08:** Match strategy (confirmed from backlog Phase 999.2, unchanged): search Deezer by artist name (`internal/deezer`); require close name equality as the PRIMARY match signal; use a shared album/release title only as a TIE-BREAKER when multiple same-named Deezer artists come back — never as the sole match signal (album titles collide across unrelated artists and MB/Deezer titles diverge in casing/edition tags).
- **D-09:** Fail closed on no confident match: leave `image_url` (and `deezer_id`, if not otherwise supplied) NULL rather than attach a possibly-wrong artist's photo — existing `CoverArt` placeholder renders as it does today. This is a one-way behavioral commitment in the sense that a wrong-artist photo, once attached and cached/seen by the user, is worse than no photo — matching D-06/D-09's cost-conscious precedent from Phase 12. — **Reversibility:** reversible — a stricter/looser threshold can be tuned later without any migration; the *choice* to fail closed rather than best-guess is the one being locked here.

### Claude's Discretion
None — every gray area reached an explicit user decision this session. (D-01's "which recording endpoint/method name" implementation detail is left to the planner/researcher, per the note under D-01.)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Prior phase precedent
- `.planning/phases/12-cleanup-coverart-reset-search-popularity-ranking/12-CONTEXT.md` — D-04/D-06/D-09 established the "no per-search-result extra API call, cost-conscious" precedent this phase's D-01 deliberately departs from (D-01's extra call is per-*event*-on-insert, not per-search-result, which is a materially different cost profile — call this out explicitly during planning so it isn't flagged as contradicting Phase 12's rejected pattern)
- `.planning/ROADMAP.md` §"Phase 13" and §"Backlog" (pre-edit, now absorbed) — locked phase goal; original Phase 999.2 backlog text (Deezer artist-art backfill) is fully superseded by this CONTEXT.md's D-06 through D-09

### Guest-feature detection origin
- `internal/detection/musicbrainz.go` — `DetectMusicBrainz`, `detectGuestFeatures`, `detectDeluxeChanges`, `coverArtURLForReleaseGroup` — the exact functions this phase's guest-feature fix (D-01–D-03) extends
- `internal/musicbrainz/recordings.go` — `Recording` struct and `RecordingsByArtist`; confirms today's recording browse has no release linkage, motivating D-01's extra lookup

### Artist add / image flow origin
- `web/app/routes/watchlist.tsx` — `handleAddSearchResult`, confirms `imageUrl: result.image_url ?? undefined` is the only current path for setting an artist's art
- `web/app/components/watchlist/SearchResultsColumns.tsx` — confirms no cross-source add path exists today (Deezer-sourced results are add-disabled)
- `internal/httpserver/watchlist.go` — `handleAddWatchlist`, confirms the server accepts but never derives `ImageURL`/`DeezerID`
- `internal/watchlist/service.go` — `AddParams`, `Entry`, `UpsertArtist` — the seam where backfill-written `image_url`/`deezer_id` values land

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `coverArtURLForReleaseGroup` (`internal/detection/musicbrainz.go:503`) — reused as-is for guest-feature cover art once D-01's lookup returns a release-group MBID; no new cover-art URL scheme needed
- `nullableString` helper (`internal/detection/musicbrainz.go`) — reused for the new guest-feature `ReleaseDate` field, same as `new_release`/`deluxe_change`
- `NewReleaseBody`'s date-fallback pattern (`EventCard.tsx`) — reused verbatim by D-05 for `DeluxeChangeBody`
- `internal/deezer` client — reused for the D-08 name-search match (existing `SearchArtists`/fan-count sort from Phase 12)

### Established Patterns
- `InsertEventParams`'s `ReleaseDate`/`CoverArtUrl` are write-once display-snapshot fields (per `internal/db/migrations/000003_events.up.sql`'s own doc comment) — D-01's new values for guest-feature rows follow this same write-once-at-insert contract, no update path needed
- Nullable `*string` wire/DB fields for optional upstream data (`Disambiguation`, `ImageURL`, `Country` from Phase 12) — any new field this phase introduces should follow the same nullability convention
- Fail-closed-on-low-confidence is now a two-time precedent (Phase 12's D-06/D-09 popularity/disambiguation decisions, and this phase's D-09) — worth naming as a project-level convention if a future phase touches upstream-data matching again

### Integration Points
- `internal/detection/musicbrainz.go`'s `detectGuestFeatures` — where D-01/D-02/D-03's new lookup and its results attach to the existing `InsertEvent` call
- `web/app/components/history/EventCard.tsx`'s `DeluxeChangeBody` — where D-04/D-05's date rendering lands
- `web/app/lib/api.ts`'s `EventItem` type — verify `release_date` is already exposed for `deluxe_change`/`guest_feature` rows, not just `new_release`
- A new artist-art-matching module (naming TBD by planner, e.g. alongside `internal/deezer` or as its own package) — where D-06–D-09's match-and-backfill logic lives; needs a way to run once at startup (D-07) and to run at add-time (D-06) without duplicating match logic between the two call sites

</code_context>

<specifics>
## Specific Ideas

No additional specific UI/UX references beyond what's captured in Decisions above.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 13-fix-history-dates-guest-feature-art-artist-art*
*Context gathered: 2026-08-24*
