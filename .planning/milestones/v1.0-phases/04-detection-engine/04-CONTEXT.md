# Phase 4: Detection Engine - Context

**Gathered:** 2026-08-08
**Status:** Ready for planning

<domain>
## Phase Boundary

The system diffs polled MusicBrainz/Deezer results against a persistent, idempotent "seen" store to detect and record three event types for watched artists: new releases, guest features, and deluxe/tracklist changes (DTCT-01/02/03). Detection runs synchronously inside the existing poll cycles in `internal/poller/poller.go` — replacing the current log-only statement — and inherits Phase 3's per-source overlap guard (DTCT-05) with no new work needed there. This phase owns a new events table (the seen store), the MusicBrainz release-detail and recording-by-credit fetch methods it needs, and per-artist preference filtering at detection time. No Discord posting (Phase 5 reads this phase's output), no UI (Phase 6).

</domain>

<decisions>
## Implementation Decisions

### Deluxe/tracklist-change signal (DTCT-02)
- **D-01:** Detect an expanded tracklist via a track-count fetch — browse releases within a release-group (`/release?release-group=X&inc=media`) and compare track counts, not a title-keyword heuristic. Catches real reissues regardless of naming.
- **D-02:** Any track-count increase for a release within a group (vs. the highest previously-seen count for that group) counts as a real change worth alerting — no minimum-jump threshold.
- **D-03:** Deluxe/tracklist-change detection is MusicBrainz-only. Deezer's `Album.Tracklist` field is a URL, not real track data — **Reversibility:** reversible — Deezer-side detection can be added later without touching the MusicBrainz path.
- **D-04:** Release-level track-count detail is only fetched for release-groups already in the seen store — a brand-new group is a "new release" event, not a tracklist-change check. Avoids doubling request volume against the rate limiter every cycle for every artist.

### Guest-feature detection scope (DTCT-03)
- **D-05:** Recordings-by-artist-credit browse (`/ws/2/recording?artist=<mbid>&inc=artist-credits`) uses the same bounded pagination pattern as `ReleaseGroupsByArtist` (maxPages=10 × pageSize=100) rather than a tighter, recording-specific cap.
- **D-06:** "Guest feature" (non-primary artist-credit) is determined positionally — the watched artist is a guest if they are not first in the recording's artist-credit list. No title-text ("feat."/"ft.") confirmation required.
- **D-07:** Guest-feature scanning runs on the same MusicBrainz poll cadence as release-group scanning — one poll cycle fetches both, gated by the same `PollInterval` and rate limiter. No new scheduler cadence.
- **D-08:** Guest-feature detection is MusicBrainz-only — Deezer's client has no track/credit-level fetch capability at all.

### Seen-store & event schema (DTCT-04)
- **D-09:** One combined table serves as both the seen store and the event log — an event row's existence *is* what "already seen" means. A unique constraint on the dedup key enforces idempotency (DTCT-04) at the DB level. — **Reversibility:** one-way — splitting into a separate seen-store + events table later means migrating existing rows and re-threading Phase 5/6's queries against two tables instead of one.
- **D-10:** The dedup/uniqueness key is a per-event-type external ID: release-group MBID for `new_release`, release MBID for `deluxe_change`, recording MBID for `guest_feature`. No synthetic content hash.
- **D-11:** A nullable `notified_at` column is added to the events table now, so Phase 5's job is `SELECT WHERE notified_at IS NULL` → post → `UPDATE notified_at`. No new table needed in Phase 5.
- **D-12:** Event rows store an inline snapshot of display data (title, cover art URL, release date, artist name) captured at detection time, not just IDs. NTFY-01 needs this data on the Discord message, and it's already in hand from the poll response — a live re-fetch at notify/render time is an unnecessary extra external call.

### First-run / seed behavior
- **D-13:** A newly-added artist's first poll cycle creates full event rows for their existing catalog with `notified_at` pre-set to seed time (not left null) — not a thinner dedup-keys-only seed path. Phase 5 skips them automatically (D-11's query already excludes non-null `notified_at`); Phase 6's history still shows them correctly as pre-existing.
- **D-14:** "First cycle" (seed mode) is detected implicitly — zero existing event rows for this artist means seed mode. No explicit `seeded_at` column.
- **D-15:** Seeding is per-source independent, not one global per-artist event — matches Phase 3 D-08's per-source independence. Prevents a flood: an artist added MusicBrainz-only whose `deezer_id` is backfilled later would otherwise have their entire Deezer catalog dumped as "new releases" the first time Deezer data appears.
- **D-16:** Removing then re-adding an artist preserves detection history and does not re-trigger a seed flood — event rows are keyed by `artist_id` (master data, Phase 2 D-03), not the watchlist row id. Removal only deletes the watchlist row (Phase 2 D-10), never `artists` or event rows, so re-adding resumes from where detection left off with no extra logic.

### Preference filtering scope (WLST-05/WLST-06 interaction with detection)
- **D-17:** Release-type filtering (WLST-05) is applied at detection time, not notify time — a release-group whose primary-type isn't in the artist's `release_types` filter never becomes an event row at all. A later filter change doesn't retroactively "unfilter" anything already skipped, and the seen-store only tracks what the user actually wanted.
- **D-18:** The mute-preference axis (WLST-06/NTFY-04) works the same way as D-17, applied at detection time — not deferred to Phase 5 as NTFY-04's literal "suppresses notifications" wording might suggest. Both preference axes are checked identically before an event row is ever created.

### Idempotency under partial cycle failure
- **D-19:** A poll cycle that crashes partway through writing events for an artist relies purely on the DB's unique dedup constraint to safely no-op on re-detection next cycle — no explicit resume/checkpoint tracking. Each detection is fully re-derivable from the next cycle's full fetch+diff (release-groups/recordings are re-fetched in full, not incrementally).
- **D-20:** A duplicate event insert (dedup key already exists) uses `ON CONFLICT DO NOTHING` — an already-seen event's original snapshot (D-12) is never overwritten by a later poll's fetch, preserving the historical record as first detected.

### Claude's Discretion
- Exact table/column names beyond what's specified above (e.g., events table name, migration numbering) — not discussed, left to planning/research.
- Exact Go project layout for the new detection logic (package name, whether it lives in `internal/poller` or a new `internal/detection` package) — architecture detail.
- Precise structured-log field names for detection-related log lines — follow existing `slog`/`request_id`/`cycle_id` conventions from Phases 1–3.
- Whether the new MusicBrainz release-detail and recording-by-credit fetch methods live on the existing `musicbrainz.Client` or a new type — implementation detail, follows the existing `ReleaseGroupsByArtist` pattern either way.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & Roadmap
- `.planning/REQUIREMENTS.md` — DTCT-01 through DTCT-05 (this phase's requirement set); WLST-05/WLST-06 (preference axes now applied here per D-17/D-18); NTFY-01/NTFY-04 (Phase 5, shapes D-11/D-12/D-18); "Out of Scope" table — no full historical backfill, dual-source reconciliation, or audio-fingerprint dedupe.
- `.planning/ROADMAP.md` §"Phase 4: Detection Engine" — goal and success criteria.
- `.planning/PROJECT.md` — constraints and Key Decisions table; Context section notes MusicBrainz TLS is broken on this dev machine's WSL2 path (environmental, not code — see Broken Windows Ledger entry #3) which will affect any live MusicBrainz testing in this phase.

### Tech stack / library choices (already locked)
- `.claude/CLAUDE.md` — hand-rolled MusicBrainz/Deezer clients, `golang.org/x/time/rate` limiting, `robfig/cron/v3` scheduling.

### Prior phase decisions this phase builds directly on
- `.planning/phases/03-external-clients-search/03-CONTEXT.md` — D-04 (poll cycle log statement is replaced by this phase's diff logic, in-place), D-07/D-08/D-09 (per-source rate limiting, independence, overlap guard — DTCT-05 inherited), D-10/D-12 (MusicBrainz fetches release-groups only, Deezer fetches albums only — the gap this phase's new fetch methods fill).
- `.planning/phases/02-watchlist-core/02-CONTEXT.md` — D-03 (artists is master data independent of watchlist membership — shapes D-16), D-05/D-06/D-07 (release_types/muted_event_types preference axes — shapes D-17/D-18), D-09/D-10 (duplicate-add 409, hard-delete removal — shapes D-16).

### Existing code (Phase 1–3)
- `internal/poller/poller.go` — `RunMusicBrainzCycle`/`RunDeezerCycle`, the exact methods this phase's diff logic replaces the log statement inside; `mbRunning`/`dzRunning` overlap guards (DTCT-05, already satisfied).
- `internal/musicbrainz/releasegroups.go` — `ReleaseGroupsByArtist`, the existing bounded-pagination pattern (`maxReleaseGroupPages`) D-01/D-05 extend for the new release-detail and recording-by-credit fetch methods.
- `internal/deezer/albums.go` — `ArtistAlbums`, confirms `Album.Tracklist` is a URL not real data (D-03) and there is no per-track credit fetch (D-08).
- `internal/watchlist/service.go` — `Store`/`Entry`/`ReleaseTypes`/`EventTypes` — the preference data D-17/D-18 read at detection time; `Entry.DeezerID` nullability (Phase 3 D-06) also applies to any Deezer-side detection.
- `internal/db/migrations/000002_watchlist.up.sql`, `queries/artists.sql`, `queries/watchlist.sql` — current schema and sqlc query conventions (explicit column aliasing, `:execrows`/`:one` sqlc annotations) the new events migration and queries should follow.

No external specs beyond the above — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/poller.Poller` — already wired with `watchlist.Store`, `ReleaseGroupSource`, `AlbumSource` narrow-interface seams (mirroring `httpserver.Pinger`); this phase's diff logic slots into the existing cycle methods rather than requiring new wiring.
- `internal/musicbrainz.Client`'s bounded-pagination pattern (`fetchReleaseGroupPage`-style single-page fetch wrapped in a bounded loop) — the template for the new release-detail and recording-by-credit fetch methods (D-01/D-05).
- `watchlist.ReleaseTypes`/`watchlist.EventTypes` — the single Go-side allow-lists already mirroring the DB CHECK constraints; detection-time filtering (D-17/D-18) reads a `watchlist.Entry`'s `ReleaseTypes`/`MutedEventTypes` fields directly, no new lookup needed.

### Established Patterns
- sqlc queries live under `queries/`, generated into `internal/db/sqlc/`, with explicit column aliasing when joining tables that share column names (see `ListWatchlist`'s `w.id AS id, a.id AS artist_id`) — the new events table will likely join `artists` the same way.
- `ON CONFLICT` upsert pattern already used in `UpsertArtist` (`queries/artists.sql`) — D-20's `ON CONFLICT DO NOTHING` follows the same SQL idiom, just without the `DO UPDATE` half.
- Migrations are plain up/down `.sql` under `internal/db/migrations/`, embedded via `go:embed`, sequentially numbered (`000001_init`, `000002_watchlist`) — the new events table is `000003_*`.
- Structured `slog` logging with `cycle_id` correlation is already established per-source in the poller (D-04, Phase 3) — detection log lines should extend the same logger, not start a new one.

### Integration Points
- New MusicBrainz fetch methods extend `internal/musicbrainz.Client` alongside `ReleaseGroupsByArtist`.
- New detection/diff logic is called from inside `RunMusicBrainzCycle`/`RunDeezerCycle` in `internal/poller/poller.go`, replacing the current `logger.Info("poll result", ...)` call.
- New events table and its sqlc queries follow the `queries/*.sql` → `internal/db/sqlc/` codegen path already established.

</code_context>

<specifics>
## Specific Ideas

No particular UI/visual references (Phase 4 is detection logic only, no UI — that's Phase 6). The recurring theme across this discussion was "reuse the exact patterns Phase 3 already established" — bounded pagination, per-source independence, narrow-interface seams — rather than introducing new mechanisms for a phase that's fundamentally an extension of the existing poll cycle.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.

</deferred>

---

*Phase: 4-Detection Engine*
*Context gathered: 2026-08-08*
