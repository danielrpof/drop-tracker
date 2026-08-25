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
- **D-02:** When a recording has releases with different dates (single, then album/reissue), use the **earliest** release date among them — matches the `FirstReleaseDate` semantics already used for `new_release` events, and reflects when the guest feature was first actually released. **Amended by the grilling round (Q2):** plain lexicographic string comparison is only chronologically correct across *different* years or across dates of *equal* precision. Two same-year dates where one is a strict string-prefix of the other (e.g. `"2020"` vs `"2020-01-05"`) are NOT safely ordered by `<` — the shorter, vaguer value would win by sorting first even though "vague" does not mean "earlier" (a reissue/compilation frequently carries a year-only date while the original release carries a full day, and the original is the one that's actually earliest). The rule is therefore: compare leading 4-digit years first (different years decide it outright); when years match and one date is a strict prefix of the other, the **more precise** (longer) date wins as "earliest," since a specific recorded date is stronger evidence than an admittedly-vague one; when years match and neither is a prefix of the other (equal precision, e.g. `"2020-03"` vs `"2020-01"`), fall back to plain lexicographic comparison, which is correct at equal precision.
- **D-03:** When the lookup finds no releases at all for a recording (rare — unreleased/promo-only credits), behave exactly like today: `CoverArt`'s existing placeholder, and `event.release_date` stays null → renders `NewReleaseBody`'s existing "Release date unknown" fallback text.
- **Note for planner:** `internal/musicbrainz` will need a new client method (e.g. `ReleasesForRecording` or similar, analogous to `ReleasesByReleaseGroup`) to make this MusicBrainz call. `internal/detection/musicbrainz.go`'s `detectGuestFeatures` is where the `InsertEvent` call gets the new `ReleaseDate`/`CoverArtUrl` args wired in (mirroring the `new_release`/`deluxe_change` pattern at lines 102-113 and 380-393).
- **D-13 (grilling round, Q5):** Cap the number of NEW per-recording `ReleasesForRecording` lookups `detectGuestFeatures` will perform for a single artist within a single poll cycle at `maxNewGuestFeatureLookupsPerCycle = 20`. The poll cycle already dispatches artists concurrently across a bounded worker pool (`internal/poller`'s `WithMusicBrainzWorkers`), but every worker shares the *same* process-wide MusicBrainz `rate.Limiter` — so one newly-added, feature-heavy artist (a first "seed" cycle surfacing dozens of historical guest-feature recordings at once) can still consume the entire cycle's shared ~1 req/sec budget and stretch that cycle's wall-clock duration for every other artist competing for the same limiter, even though they run on separate goroutines. Once the cap is hit for an artist in a cycle, the remaining not-yet-looked-up recordings for that artist are simply left alone this cycle — they never enter the seen-store, so they're naturally reconsidered on the next cycle (same "not seen → retried later" contract already used for a lookup error). — **Reversibility:** reversible — a pure pacing bound, no schema or contract change; the cap value is a tunable constant.

### Deluxe-change date rendering
- **D-04:** Add the release date to `DeluxeChangeBody` (`web/app/components/history/EventCard.tsx`) on the same line, before the track-count delta: `"{date} · {prev} → {current} tracks"` — matches `NewReleaseBody`'s existing `"{release_type} · {date}"` separator style for visual consistency across all three card types.
- **D-05:** Fall back to the same `"Release date unknown"` text `NewReleaseBody` already uses when `release_date` is null — no new fallback copy.
- **Note for planner:** This is a frontend-only change — `release_date` is already populated in the DB and already flows through `EventItem` in `web/app/lib/api.ts` (confirm the TS type includes it; if `EventItem` doesn't expose `release_date` for the frontend today, that's part of this fix too).

### Artist-art backfill scope
- **D-06:** Artist-art matching applies BOTH to new adds going forward AND as a one-time backfill over artists already on the watchlist with `image_url IS NULL`.
- **D-07:** The backfill runs as a one-time migration/startup pass, sweeping every artist with `image_url IS NULL` once when the app starts running this phase's code — not folded into the existing poll cycle, not a separate scheduled job. **Amended by the grilling round (Q4):** "one-time" only holds *within a single process lifetime* as originally written — there is no persisted marker distinguishing "never attempted" from "attempted and failed closed," so every restart re-sweeps every still-image-less artist from scratch. Given this project's own stated purpose is demonstrating CI/CD pipeline maturity (frequent redeploys are the expected steady state, not an edge case), that would mean the same handful of D-09 fail-closed artists get re-queried against Deezer on every single deploy, indefinitely, for no new information. D-07 now additionally means: sweep every artist with `image_url IS NULL` **and** (`art_match_attempted_at IS NULL` **or** `art_match_attempted_at` older than a 24-hour cooldown) — see D-12.
- **Note for planner:** Backfill scope is artist rows only (`artists.image_url`/`artists.deezer_id`) — this phase does not touch event/detection logic, per the same boundary backlog Phase 999.2 originally drew against PROJECT.md's rejection of full dual-source reconciliation.
- **D-12 (grilling round, Q4):** Add a nullable `artists.art_match_attempted_at timestamptz` column (new migration `000005`) recorded for **every** artist the backfill sweep visits, whether it matched, failed to match, or errored — this is the one intentional exception to "this phase does not touch event/detection logic," and to the sibling plans' original "no migration" invariant, because there is no other way to distinguish "never tried" from "tried and failed" without persisted state. `ListArtistsMissingImage`'s predicate becomes `image_url IS NULL AND (art_match_attempted_at IS NULL OR art_match_attempted_at < now() - INTERVAL '24 hours')`, so a fail-closed artist is retried at most once per day (bounded worst-case cost across frequent restarts) rather than on every single process start, while still eventually re-checking artists whose Deezer catalogue entry might appear later. Recording the attempt is a separate, minimal write (`RecordArtMatchAttempt`) from `UpsertArtist` — it must fire even on a `Matched: false` outcome, where D-09 already forbids calling `UpsertArtist` at all. — **Reversibility:** reversible — the cooldown window is a tunable constant/column; dropping the column later loses only the pacing optimization, not any user-visible state.

### Artist match confidence & tie-breaking
- **D-08:** Match strategy (confirmed from backlog Phase 999.2, unchanged): search Deezer by artist name (`internal/deezer`); require close name equality as the PRIMARY match signal; use a shared album/release title only as a TIE-BREAKER when multiple same-named Deezer artists come back — never as the sole match signal (album titles collide across unrelated artists and MB/Deezer titles diverge in casing/edition tags). **Amended by the grilling round (Q6):** the tie-break's title comparison is normalized-equality **or** one normalized title being a substring of the other, gated by a minimum normalized length (`minTieBreakTitleLength = 4` runes) to reject spurious short-substring collisions. Rationale: real-world edition suffixes ("(Deluxe)", "(Remastered 2020)") are common enough that exact-only comparison would make the tie-break path effectively never resolve anything in practice, defeating the reason it exists; substring containment still requires genuine, non-trivial title overlap, so it doesn't weaken D-09's fail-closed posture — it only rescues cases that were unambiguous to begin with. **Also amended (Q3):** `normalizeArtistName`'s folding now includes a hand-rolled Latin diacritic fold (à/á/â/ã/ä/å→a, è/é/ê/ë→e, ì/í/î/ï→i, ò/ó/ô/õ/ö→o, ù/ú/û/ü→u, ñ→n, ç→c, ý/ÿ→y, and so on) in addition to the existing trim/lowercase/whitespace-collapse/typographic-apostrophe fold — reggaeton and Latin R&B names (this project's own stated genre scope) are exactly the population most likely to differ between MusicBrainz and Deezer only in diacritic rendering, and D-08's strict-equality primary signal would otherwise silently fail every one of them. A hand-rolled fold table is used instead of pulling in `golang.org/x/text/unicode/norm`, preserving this phase's (and the project's) zero-new-dependency posture.
- **D-09:** Fail closed on no confident match: leave `image_url` (and `deezer_id`, if not otherwise supplied) NULL rather than attach a possibly-wrong artist's photo — existing `CoverArt` placeholder renders as it does today. This is a one-way behavioral commitment in the sense that a wrong-artist photo, once attached and cached/seen by the user, is worse than no photo — matching D-06/D-09's cost-conscious precedent from Phase 12. — **Reversibility:** reversible — a stricter/looser threshold can be tuned later without any migration; the *choice* to fail closed rather than best-guess is the one being locked here.
- **D-11 (grilling round, Q3):** The backfill's `Stats` gains a computed `MatchRatePercent()` (matched / visited, 0 when visited is 0), included in the existing single Info summary log line. This phase does not add a dashboard or persisted metrics table — no new infrastructure — but the doc comment on `Stats` must record an explicit operational expectation: if the logged match rate is persistently low (informal threshold: under ~40%), that's a signal to revisit `normalizeArtistName`'s folding rules, not evidence that the artists aren't on Deezer. Without this, D-08/D-09's strict fail-closed design has no feedback loop, and the phase's core value proposition (artists actually get real photos) is otherwise unverified beyond "the code runs."

### Rate-limiter contention between add-time matching and the startup backfill
- **D-10 (grilling round, Q1):** `watchlist.Service.Add`'s add-time match (D-06) and `artistart.Backfill`'s startup sweep (D-07) share the same `dzClient`/`mbClient` instances and therefore the same process-wide rate limiters (T-13-12/T-13-13's own stated rationale for reusing those instances). Nothing in the original design analyzed what happens when both run at once — which is guaranteed to happen on every deploy with a nonzero `image_url IS NULL` backlog, since the backfill starts right after boot. Add an `artistart.ActivityGate`: a small shared counter that `Service.Add` increments/decrements around its own `Match` call, and that `Backfill` checks before every per-artist match attempt — when an add is in flight, `Backfill` yields with a short, bounded backoff (never indefinitely, so the sweep always makes forward progress) before proceeding. This gives interactive adds implicit priority over the background sweep without opening a second rate budget.
  - **Q1(b) considered and rejected:** a fully separate rate-limiter/token-bucket for the backfill (rather than a shared one with priority-yielding) was considered and is NOT adopted. MusicBrainz's self-imposed ~1 req/sec ceiling is an external, whole-service constraint (per this project's own stack decisions) — a second independent limiter would not add capacity, it would just let the backfill exceed the real external budget the single limiter exists to enforce, risking 429/503s from MusicBrainz itself. Deezer has more headroom (~10 req/sec) where a split budget would be safer, but splitting only one of the two upstreams adds real complexity (a second `deezer.Client`/limiter pair) for a benefit the priority-yielding gate (D-10's primary mechanism) already captures. If production logs show the yielding gate is insufficient, revisit a Deezer-only split budget then — not preemptively.
  - **Reversibility:** reversible — `ActivityGate` is an additional, optional-shaped parameter; removing it restores today's unmitigated contention, not a redesign.

### Claude's Discretion
None — every gray area reached an explicit user decision this session, and the grilling round above (D-10–D-13) resolved every challenge raised without leaving anything to planner discretion.

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
- `internal/detection/musicbrainz.go`'s `detectGuestFeatures` — where D-01/D-02/D-03's new lookup and its results attach to the existing `InsertEvent` call; also where D-13's per-cycle lookup cap is enforced
- `web/app/components/history/EventCard.tsx`'s `DeluxeChangeBody` — where D-04/D-05's date rendering lands
- `web/app/lib/api.ts`'s `EventItem` type — verify `release_date` is already exposed for `deluxe_change`/`guest_feature` rows, not just `new_release`
- A new artist-art-matching module (naming TBD by planner, e.g. alongside `internal/deezer` or as its own package) — where D-06–D-09's match-and-backfill logic lives; needs a way to run once at startup (D-07) and to run at add-time (D-06) without duplicating match logic between the two call sites; also where D-10's `ActivityGate` and D-11's `MatchRatePercent` land
- `internal/db/migrations/` — gains one new migration (`000005`, D-12) adding `artists.art_match_attempted_at`; this is the one schema touch in an otherwise migration-free phase, added specifically to bound repeated backfill cost across restarts

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
