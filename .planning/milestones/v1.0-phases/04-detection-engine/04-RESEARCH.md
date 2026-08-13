# Phase 4: Detection Engine - Research

**Researched:** 2026-08-08
**Domain:** Diff-based event detection against a Postgres seen-store; new MusicBrainz fetch endpoints (release-detail, recording-by-artist-credit); sqlc idempotent-insert patterns
**Confidence:** MEDIUM-HIGH (architecture/schema/Go patterns HIGH — grounded in this session's in-repo reads; exact MusicBrainz JSON shapes for the two *new* endpoints MEDIUM — see Assumptions Log, live verification was environmentally blocked this session)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Deluxe/tracklist-change signal (DTCT-02)**
- **D-01:** Detect an expanded tracklist via a track-count fetch — browse releases within a release-group (`/release?release-group=X&inc=media`) and compare track counts, not a title-keyword heuristic. Catches real reissues regardless of naming.
- **D-02:** Any track-count increase for a release within a group (vs. the highest previously-seen count for that group) counts as a real change worth alerting — no minimum-jump threshold.
- **D-03:** Deluxe/tracklist-change detection is MusicBrainz-only. Deezer's `Album.Tracklist` field is a URL, not real track data. **Reversibility:** reversible — Deezer-side detection can be added later without touching the MusicBrainz path.
- **D-04:** Release-level track-count detail is only fetched for release-groups already in the seen store — a brand-new group is a "new release" event, not a tracklist-change check. Avoids doubling request volume against the rate limiter every cycle for every artist.

**Guest-feature detection scope (DTCT-03)**
- **D-05:** Recordings-by-artist-credit browse (`/ws/2/recording?artist=<mbid>&inc=artist-credits`) uses the same bounded pagination pattern as `ReleaseGroupsByArtist` (maxPages=10 × pageSize=100) rather than a tighter, recording-specific cap.
- **D-06:** "Guest feature" (non-primary artist-credit) is determined positionally — the watched artist is a guest if they are not first in the recording's artist-credit list. No title-text ("feat."/"ft.") confirmation required.
- **D-07:** Guest-feature scanning runs on the same MusicBrainz poll cadence as release-group scanning — one poll cycle fetches both, gated by the same `PollInterval` and rate limiter. No new scheduler cadence.
- **D-08:** Guest-feature detection is MusicBrainz-only — Deezer's client has no track/credit-level fetch capability at all.

**Seen-store & event schema (DTCT-04)**
- **D-09:** One combined table serves as both the seen store and the event log — an event row's existence *is* what "already seen" means. A unique constraint on the dedup key enforces idempotency (DTCT-04) at the DB level. **Reversibility:** one-way — splitting into a separate seen-store + events table later means migrating existing rows and re-threading Phase 5/6's queries against two tables instead of one.
- **D-10:** The dedup/uniqueness key is a per-event-type external ID: release-group MBID for `new_release`, release MBID for `deluxe_change`, recording MBID for `guest_feature`. No synthetic content hash.
- **D-11:** A nullable `notified_at` column is added to the events table now, so Phase 5's job is `SELECT WHERE notified_at IS NULL` → post → `UPDATE notified_at`. No new table needed in Phase 5.
- **D-12:** Event rows store an inline snapshot of display data (title, cover art URL, release date, artist name) captured at detection time, not just IDs. NTFY-01 needs this data on the Discord message, and it's already in hand from the poll response — a live re-fetch at notify/render time is an unnecessary extra external call.

**First-run / seed behavior**
- **D-13:** A newly-added artist's first poll cycle creates full event rows for their existing catalog with `notified_at` pre-set to seed time (not left null) — not a thinner dedup-keys-only seed path.
- **D-14:** "First cycle" (seed mode) is detected implicitly — zero existing event rows for this artist means seed mode. No explicit `seeded_at` column.
- **D-15:** Seeding is per-source independent, not one global per-artist event — matches Phase 3 D-08's per-source independence. Prevents a flood: an artist added MusicBrainz-only whose `deezer_id` is backfilled later would otherwise have their entire Deezer catalog dumped as "new releases" the first time Deezer data appears.
- **D-16:** Removing then re-adding an artist preserves detection history and does not re-trigger a seed flood — event rows are keyed by `artist_id` (master data, Phase 2 D-03), not the watchlist row id.

**Preference filtering scope (WLST-05/WLST-06 interaction with detection)**
- **D-17:** Release-type filtering (WLST-05) is applied at detection time — a release-group whose primary-type isn't in the artist's `release_types` filter never becomes an event row at all.
- **D-18:** The mute-preference axis (WLST-06/NTFY-04) works the same way as D-17, applied at detection time — not deferred to Phase 5.

**Idempotency under partial cycle failure**
- **D-19:** A poll cycle that crashes partway through writing events for an artist relies purely on the DB's unique dedup constraint to safely no-op on re-detection next cycle — no explicit resume/checkpoint tracking.
- **D-20:** A duplicate event insert (dedup key already exists) uses `ON CONFLICT DO NOTHING` — an already-seen event's original snapshot (D-12) is never overwritten by a later poll's fetch.

### Claude's Discretion
- Exact table/column names beyond what's specified above (e.g., events table name, migration numbering).
- Exact Go project layout for the new detection logic (package name, whether it lives in `internal/poller` or a new `internal/detection` package).
- Precise structured-log field names for detection-related log lines — follow existing `slog`/`request_id`/`cycle_id` conventions from Phases 1–3.
- Whether the new MusicBrainz release-detail and recording-by-credit fetch methods live on the existing `musicbrainz.Client` or a new type.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DTCT-01 | Detect a new release-group for a watchlisted artist and record it as a "new release" event | Recommended `events` schema + `InsertEvent` idiom (Code Examples); `ReleaseGroupsByArtist` already returns everything needed (no new fetch) — diff is "MBID not yet in `events`" per artist+source |
| DTCT-02 | Detect a new release inside an existing release-group with an expanded tracklist, record as "deluxe/tracklist-change" | New `ReleasesByReleaseGroup` fetch method (Pattern 2); the **baseline-tracking gap** identified in Common Pitfalls #1 — read that section before planning the deluxe-change task |
| DTCT-03 | Detect a recording where a watchlisted artist is a non-primary artist-credit, record as "guest feature" | New `RecordingsByArtist` fetch method (Pattern 3); positional artist-credit check (Common Pitfalls #3); pagination-volume pitfall (#4) |
| DTCT-04 | Idempotent "seen" store — never re-notify for an already-recorded release/change | `ON CONFLICT DO NOTHING` + `:execrows` pattern (Code Examples, Common Pitfalls #2); unique constraint on `(event_type, source, external_id)` |
| DTCT-05 | No overlapping poll-cycle runs for the same source | Already satisfied — `[VERIFIED: internal/poller/poller.go:92-93]` `mbRunning atomic.Bool` / `dzRunning atomic.Bool`, no changes needed this phase |
</phase_requirements>

## Summary

This phase's architecture is almost entirely locked by CONTEXT.md's D-01 through D-20 — the open research surface is narrower than usual: (1) the exact JSON shape of two MusicBrainz endpoints this codebase has never called before (`/ws/2/release?release-group=` and `/ws/2/recording?artist=`), (2) the Go/sqlc mechanics of turning `ON CONFLICT DO NOTHING` into an "was this actually new" signal, and (3) a schema-design gap CONTEXT.md's decisions don't fully close: **how to persist a track-count baseline for DTCT-02 without contradicting D-20's "never overwrite a snapshot" rule.**

Live verification of the two new MusicBrainz endpoints was **not possible this session** — every attempt (`WebFetch` against `musicbrainz.org/ws/2/...` and the MusicBrainz wiki) failed, one with a raw socket error and one with an explicit Anubis anti-bot block page. This is consistent with, but broader than, the WSL2-specific TLS failure already documented in PROJECT.md/03-VERIFICATION.md (Broken Windows Ledger #3) — a fresh web search this session found independent evidence that MusicBrainz has been aggressively blocking automated/bot traffic since March 2026 due to AI-scraper load, which would explain the block reproducing even from a different network path (Anthropic's WebFetch infrastructure, not this dev machine). Treat MusicBrainz live-reachability as a **recurring environmental risk for this project**, not a one-off, and expect Phase 4's UAT to hit the same acknowledged-gap pattern Phase 3 did.

Given that, the two new endpoints' JSON shapes below are `[ASSUMED]` — reconstructed from training knowledge of MusicBrainz's long-stable, versioned `ws/2` API, corroborated where possible by web search snippets (marked `[CITED]` inline). They follow the exact same envelope convention already **live-verified** in Phase 3 (`release-group-count`/`release-group-offset`/`release-groups` → predictably `release-count`/`release-offset`/`releases` and `recording-count`/`recording-offset`/`recordings`). The plan should gate these shapes behind `httptest.Server`-fixture-driven TDD (already this project's testing convention) and treat any live-call verification as best-effort, not blocking, exactly as Phase 3's UAT did.

The most valuable finding in this research is **not** the API shapes — it's a genuine gap in D-01/D-02/D-04's specification: comparing "the highest previously-seen count for that group" requires a persisted baseline, but D-04 explicitly forbids fetching track-count data the *first* time a group is seen, and D-20 forbids overwriting an event's original snapshot. Naively taking `MAX(track_count)` over existing rows and treating "no rows yet" as a baseline of 0 will fire a **false-positive deluxe_change event on every release-group's first real comparison cycle**. See Common Pitfalls #1 for the full mechanism and a recommended fix.

**Primary recommendation:** Add one `events` table (D-09) with a `source` discriminator column (musicbrainz|deezer — needed because D-15's rationale implies Deezer independently produces `new_release` events from its own ID space, which D-10 doesn't explicitly disambiguate), a `release_group_mbid` column for deluxe-change baseline lookups, and a mutable `track_count` column used *only* for baseline-tracking (never for the D-12 display snapshot, which stays write-once via `ON CONFLICT DO NOTHING`). Build the diff logic in a new `internal/detection` package consumed by `poller.go` through the same narrow-interface seam pattern as `ReleaseGroupSource`/`AlbumSource`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| New-release / guest-feature / deluxe-change diff logic | API / Backend (`internal/detection`, new) | Database (seen-store reads/writes) | Pure Go diff logic called synchronously inside the existing poller cycle — no new HTTP surface, no new tier |
| Release-detail fetch (`/release?release-group=`) | API / Backend (`internal/musicbrainz`, extended) | External Service (MusicBrainz) | Extends the existing rate-limited client, same tier as `ReleaseGroupsByArtist` |
| Recording-by-artist-credit fetch (`/recording?artist=`) | API / Backend (`internal/musicbrainz`, extended) | External Service (MusicBrainz) | Same as above |
| Seen-store / idempotent event log (`events` table) | Database / Storage | API / Backend (sqlc queries) | `ON CONFLICT DO NOTHING` + unique constraint does the actual dedup work at the DB tier, not in Go |
| Per-artist preference filtering (D-17/D-18) | API / Backend | Database (reads `watchlist.release_types`/`muted_event_types`, already fetched via `watchlist.Store.List`) | Pure Go predicate against data already in hand — no new query needed |
| Overlap-guard / no-concurrent-cycles (DTCT-05) | API / Backend (in-process, inherited) | — | Already implemented in Phase 3 (`mbRunning`/`dzRunning`); this phase adds no new concurrency surface |

## Standard Stack

### Core
No new external dependencies. Every library this phase needs is already in `go.mod`:

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/jackc/pgx/v5` | v5.10.0 | Postgres driver, `CommandTag().RowsAffected()` backing sqlc's `:execrows` | `[VERIFIED: go.mod:11]` — already locked, no change needed |
| `github.com/sqlc-dev/sqlc` (CLI) | v1.31.1 | Generates `InsertEvent`/`ListUnnotified`/etc. from new `queries/events.sql` | `[VERIFIED: sqlc.yaml exists at repo root]`, same codegen path as Phases 1–2 |
| `net/http`, `encoding/json` (stdlib) | Go 1.26 | New `ReleasesByReleaseGroup`/`RecordingsByArtist` fetch methods on `musicbrainz.Client` | `[VERIFIED: go.mod:3]` `go 1.26`; follows `internal/musicbrainz/releasegroups.go`'s exact pattern |

**Installation:** none — `go build ./...` picks up the new files with zero `go get` needed.

## Package Legitimacy Audit

No external packages are installed this phase (confirmed via `[VERIFIED: go.mod:1-22]` — full file read this session, no new `require` entries needed for anything DTCT-01 through DTCT-05 require). The Package Legitimacy Gate is not applicable.

## Architecture Patterns

### System Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                       cmd/server (single binary)                          │
│                                                                             │
│  robfig/cron tick ──▶ Poller.RunMusicBrainzCycle(ctx)                     │
│                         │ (mbRunning CAS guard -- DTCT-05, unchanged)      │
│                         ▼                                                  │
│                    watchlist.Store.List(ctx) ──▶ Postgres: watchlist JOIN artists
│                         │ per artist, sequential
│                         ▼
│                    mb.ReleaseGroupsByArtist(ctx, mbid)  [existing, Phase 3]
│                         │
│                         ▼
│              ┌─────────────────────────────────────────┐
│              │ internal/detection.Detector (NEW)         │
│              │  .DetectMusicBrainz(ctx, entry, groups)   │
│              └─────────────────┬───────────────────────┘
│                                 │
│         ┌───────────────────────┼───────────────────────┐
│         ▼                       ▼                       ▼
│  seed-mode check         group already seen?      preference filter
│  (SELECT EXISTS,         (compare fresh MBIDs      (D-17 primary-type,
│   D-14, per source)       vs seen MBIDs)             D-18 muted types)
│         │                       │                       │
│         │              ┌────────┴────────┐              │
│         │              ▼                 ▼              │
│         │     new group -> new_release   already-seen group?
│         │     event (no track-count       -> mb.ReleasesByReleaseGroup
│         │     fetch, D-04)                    (NEW fetch method, D-01)
│         │                                     -> compare track_count vs
│         │                                        stored baseline (see
│         │                                        Pitfall #1) -> maybe
│         │                                        deluxe_change event
│         ▼
│  mb.RecordingsByArtist(ctx, mbid)  (NEW fetch method, D-05)
│         │  bounded pagination (maxPages=10 x 100, same as ReleaseGroupsByArtist)
│         ▼
│  positional filter: artist-credit[0].id != watched artist id -> guest_feature candidate
│         │
│         ▼
│  sqlc: InsertEvent (:execrows, ON CONFLICT DO NOTHING on (event_type,source,external_id))
│         │
│         ▼
│  Postgres: events table  (D-09 seen-store == event log; notified_at NULL
│                            unless seed mode, D-13)
└──────────────────────────────────────────────────────────────────────────┘

  Deezer cycle (RunDeezerCycle) follows the same shape but only ever
  produces new_release events (D-03/D-08: deluxe & guest-feature are
  MusicBrainz-only) from a *separate* ID namespace (Deezer album id, not
  an MBID) -- hence the `source` column, see Common Pitfalls #5.
```

### Recommended Project Structure
```
internal/
├── detection/                    # NEW package
│   ├── detector.go               # Detector struct wrapping sqlc.Querier; DetectMusicBrainz/DetectDeezer entry points
│   ├── musicbrainz.go            # new_release/deluxe_change/guest_feature diff logic for MB
│   ├── deezer.go                 # new_release diff logic for Deezer (no deluxe/guest — D-03/D-08)
│   ├── filter.go                 # D-17/D-18 preference predicates over watchlist.Entry
│   └── detector_test.go          # real-Postgres tests (testutil.NewTestPool) + fake-source unit tests
├── musicbrainz/
│   ├── releases.go               # NEW: ReleasesByReleaseGroup(ctx, groupMBID) ([]Release, error) -- D-01
│   ├── recordings.go             # NEW: RecordingsByArtist(ctx, artistMBID) ([]Recording, error) -- D-05
│   ├── releases_test.go
│   └── recordings_test.go
├── db/migrations/
│   ├── 000003_events.up.sql
│   └── 000003_events.down.sql
├── poller/
│   └── poller.go                 # MODIFIED: replaces logger.Info("poll result", ...) with detector calls
queries/
└── events.sql                    # NEW: InsertEvent, HasAnyEvent (seed check), MaxTrackCountForGroup, ListUnnotified (Phase 5 groundwork per D-11)
```

### Pattern 1: Narrow-interface seam for the detector (mirrors `ReleaseGroupSource`/`AlbumSource`)
**What:** `poller.go` depends on a small interface, not `*detection.Detector` directly — matching `[VERIFIED: internal/poller/poller.go:47-49,55-57]` `ReleaseGroupSource`/`AlbumSource`.
**When to use:** The single integration point between this phase's new code and Phase 3's existing poller.
**Example:**
```go
// internal/poller/poller.go — new seam alongside the existing two
type EventRecorder interface {
	DetectMusicBrainz(ctx context.Context, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error
	DetectDeezer(ctx context.Context, entry watchlist.Entry, albums []deezer.Album) error
}
var _ EventRecorder = (*detection.Detector)(nil)
```
*Source: pattern derived from `[VERIFIED: internal/poller/poller.go:42-59]`, read this session — `ReleaseGroupSource`/`AlbumSource`/`var _ ... = (*T)(nil)` compile-time assertion idiom, extended per CONTEXT.md's explicit "replaces the log statement inside RunMusicBrainzCycle/RunDeezerCycle" integration point.*

### Pattern 2: New MusicBrainz fetch method — bounded pagination (D-01, D-05)
**What:** `ReleasesByReleaseGroup`/`RecordingsByArtist` follow the exact single-page-fetch-wrapped-in-a-bounded-loop shape as `ReleaseGroupsByArtist`.
**When to use:** Both new fetch methods.
**Example:**
```go
// internal/musicbrainz/releases.go
// Source: shape mirrors [VERIFIED: internal/musicbrainz/releasegroups.go:76-154],
// read this session. Field names/envelope shape below are [ASSUMED] --
// live verification of /ws/2/release was blocked this session (see
// Assumptions Log) -- write httptest.Server fixture tests against this
// shape as the first Wave 0 task, matching how releasegroups_test.go's
// fixtures were transcribed from a live response in Phase 3.

const maxReleasePages = 10 // mirrors maxReleaseGroupPages -- a release-group's
                            // own release count is bounded by the same
                            // upstream-controlled-count risk

type Medium struct {
	Format     string `json:"format"`
	Position   int    `json:"position"`
	TrackCount int    `json:"track-count"`
}

type Release struct {
	MBID   string   `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Date   string   `json:"date"` // opaque partial-date string, same rationale as ReleaseGroup.FirstReleaseDate
	Media  []Medium `json:"media"`
}

type releaseEnvelope struct {
	Releases []Release `json:"releases"`
	Count    int       `json:"release-count"`
	Offset   int       `json:"release-offset"`
}

// TrackCount sums every medium's track-count -- required for multi-disc
// releases, where D-02's comparison must use the release's TOTAL track
// count, not any single disc's.
func (r Release) TrackCount() int {
	total := 0
	for _, m := range r.Media {
		total += m.TrackCount
	}
	return total
}

func (c *Client) ReleasesByReleaseGroup(ctx context.Context, groupMBID string) ([]Release, error) {
	// same trimmed-mbid-empty-check + bounded pagination loop as
	// ReleaseGroupsByArtist, querying:
	//   GET /release?release-group={groupMBID}&inc=media&fmt=json&limit=100&offset=N
}
```

### Pattern 3: Recording browse + positional guest-feature filter (D-05, D-06)
**What:** `RecordingsByArtist` fetches every recording the artist appears on **in any credit position** (this is the entire point of the endpoint — it is not pre-filtered to guests). The positional filter is applied client-side, after the fetch.
**Example:**
```go
// internal/musicbrainz/recordings.go
// [ASSUMED] shape -- see Assumptions Log. joinphrase/artist-credit array
// structure corroborated by web search against musicbrainz.org/doc/Artist_Credits
// and community docs this session -- [CITED: musicbrainz.org/doc/Artist_Credits]
// for the array-of-{name,joinphrase,artist} shape specifically.

type ArtistCreditEntry struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
	Artist     struct {
		MBID string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

type Recording struct {
	MBID         string              `json:"id"`
	Title        string              `json:"title"`
	ArtistCredit []ArtistCreditEntry `json:"artist-credit"`
}

type recordingEnvelope struct {
	Recordings []Recording `json:"recordings"`
	Count      int         `json:"recording-count"`
	Offset     int         `json:"recording-offset"`
}

func (c *Client) RecordingsByArtist(ctx context.Context, artistMBID string) ([]Recording, error) {
	// GET /recording?artist={artistMBID}&inc=artist-credits&fmt=json&limit=100&offset=N
	// bounded pagination loop, maxPages=10 (D-05) -- identical shape to
	// ReleaseGroupsByArtist, see maxReleaseGroupPages precedent.
}

// internal/detection/musicbrainz.go
// isGuestFeature implements D-06's positional rule. Defensive length check
// guards against a malformed/empty artist-credit array (should never
// happen per spec, but this is external, semi-trusted data) --
// indexing ArtistCredit[0] without it is a real panic risk (see
// Common Pitfalls #3).
func isGuestFeature(rec musicbrainz.Recording, watchedArtistMBID string) bool {
	if len(rec.ArtistCredit) == 0 {
		return false
	}
	return rec.ArtistCredit[0].Artist.MBID != watchedArtistMBID
}
```

### Pattern 4: Idempotent insert with `ON CONFLICT DO NOTHING` + `:execrows` (D-20)
**What:** sqlc's `:execrows` annotation "return[s] the number of affected rows from the [Result] returned by ExecContext" `[CITED: docs.sqlc.dev/en/latest/reference/query-annotations.html]`, fetched this session. Combined with `ON CONFLICT DO NOTHING`, Postgres's own command-completion tag only counts rows actually inserted — a conflicting row contributes 0 to the count. This is the standard, widely-relied-upon idiom for "insert-if-new, tell me if it was new" in Go+pgx+sqlc codebases; the official Postgres docs describe the INSERT command tag generically ("the number of rows inserted or updated") without an explicit ON CONFLICT DO NOTHING clarification `[CITED: postgresql.org/docs/current/sql-insert.html, fetched this session — ambiguous on this exact point]`, so **write a real-Postgres unit test asserting the exact row count on both first-insert and duplicate-insert**, matching this project's existing pattern of testing exact Postgres semantics rather than assuming them (see `UpsertArtist`'s COALESCE tests from Phase 2).
**Example:**
```sql
-- queries/events.sql
-- name: InsertEvent :execrows
-- 0 rows affected means the dedup key already existed (D-20) -- the
-- caller does not treat this as an error, only as "not newly detected."
INSERT INTO events (
    artist_id, source, event_type, external_id, release_group_mbid,
    title, artist_name, release_date, cover_art_url, track_count, notified_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
ON CONFLICT (event_type, source, external_id) DO NOTHING;
```
```go
affected, err := q.InsertEvent(ctx, sqlc.InsertEventParams{ /* ... */ })
if err != nil {
    return fmt.Errorf("detection: insert event: %w", err)
}
isNewlyDetected := affected > 0
```
*Source: `:execrows` annotation semantics `[CITED: docs.sqlc.dev/en/latest/reference/query-annotations.html]`, fetched this session; `ON CONFLICT DO NOTHING` idiom already established in this codebase's `UpsertArtist` (`[VERIFIED: queries/artists.sql:14]` `ON CONFLICT (mbid) DO UPDATE`) and `DeleteWatchlistEntry`'s `:execrows` pattern (`[VERIFIED: queries/watchlist.sql:58,66]` `-- name: DeleteWatchlistEntry :execrows` / `DELETE FROM watchlist WHERE id = $1;`), both read this session.*

### Anti-Patterns to Avoid
- **Treating `MAX(track_count)` with `COALESCE(...,0)` as the deluxe-change baseline:** Fires a false-positive on the very first real comparison for every release-group. See Common Pitfalls #1 — this is the single highest-risk mistake in this phase.
- **Fetching release-detail for a release-group on the same cycle it's first discovered:** Explicitly forbidden by D-04 — a brand-new group is a `new_release` event, full stop, no track-count fetch this cycle.
- **Treating every fetched recording from `RecordingsByArtist` as a guest feature:** The endpoint returns the artist's own primary-credit recordings too (it is "every recording this artist appears on," not "every recording where they're a guest"). D-06's positional filter must run on 100% of the fetched set before any event is created.
- **Assuming MusicBrainz JSON embeds a cover-art URL:** It does not (see Common Pitfalls #6) — Deezer's JSON does (`cover` field, `[VERIFIED: live Deezer response, 03-RESEARCH.md:415]`), but MusicBrainz release/release-group responses never include one.
- **Overwriting an existing event row's D-12 snapshot fields (`title`/`cover_art_url`/`release_date`/`artist_name`) on a later poll:** Directly contradicts D-20. Only the recommended `track_count` baseline-tracking column (not a display field) should ever be mutated post-insert — see Common Pitfalls #1.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| "Was this row newly inserted?" signal | A pre-check `SELECT` before every `INSERT` (TOCTOU race under concurrent/retried cycles) | `INSERT ... ON CONFLICT DO NOTHING` + `:execrows` (Pattern 4) | Postgres handles the check-and-insert atomically in one round trip; a separate SELECT reintroduces exactly the race D-19 relies on the DB constraint to avoid |
| Diffing "seen" vs "fresh" MBID sets | A hand-rolled two-pointer merge over sorted slices | A Go `map[string]struct{}` built from a `SELECT external_id FROM events WHERE artist_id=$1 AND source=$2` query, then `for _, fresh := range freshGroups { if _, seen := seenSet[fresh.MBID]; !seen { ... } }` | O(n) map lookup is simpler and no less correct than a merge-sort approach for this data volume (bounded by the same maxPages=10×100 ceiling as the fetch itself) |
| Cover art URLs for MusicBrainz-sourced events | An extra live API call to Cover Art Archive to check existence before storing | The deterministic URL pattern `https://coverartarchive.org/release-group/{mbid}/front` `[CITED: musicbrainz.org/doc/Cover_Art_Archive/API, fetched this session]` — store it unconditionally; a 404 on render is a Phase 5/6 UI concern (broken image), not a reason to add a second external call to this phase's detection path |

**Key insight:** Every "don't hand-roll" item above exists because D-19/D-20's idempotency guarantees are only as strong as the primitives they're built on — a hand-rolled existence-check-then-insert reintroduces exactly the race the DB constraint was chosen to eliminate.

## Common Pitfalls

### Pitfall 1: The deluxe-change baseline has no home in a naive events-table-only design — false positive on first real comparison
**What goes wrong:** A release-group is discovered (cycle N, `new_release` event, no track-count per D-04). Cycle N+1: the group is now "already seen," so release-detail is fetched (D-01) and its track count needs to be compared against "the highest previously-seen count for that group" (D-02). But nothing has ever recorded a track count for this group — there's no `deluxe_change` event yet (nothing changed) and the `new_release` event row has no track-count field (D-04 forbade fetching it at creation). If the comparison code does `COALESCE(MAX(track_count), 0)` over existing rows, it gets `0`, and any real track count (e.g. 12) reads as "increased from 0" — firing a spurious `deluxe_change` alert for every release-group's first-ever real comparison cycle.
**Why it happens:** CONTEXT.md's D-01/D-02/D-04 fully specify *when* to fetch and *how* to compare, but not *where the running baseline is persisted between cycles* — D-09's "one combined table" is an event log (append/no-overwrite per D-20), which is structurally awkward for tracking a mutable "current known max" value.
**How to avoid:** Add a `track_count INT` column to the `events` table used **only** for baseline-tracking (distinct from the D-12 display snapshot fields, which stay write-once). On the `new_release` row (and later, the most recent `deluxe_change` row for that group), maintain a `release_group_mbid` column so a query like `SELECT MAX(track_count) FROM events WHERE artist_id=$1 AND source='musicbrainz' AND release_group_mbid=$2 AND track_count IS NOT NULL` can find the current baseline. Critically: **distinguish "no baseline recorded yet" (`track_count IS NULL` on every row for this group) from "baseline recorded."** When no baseline exists, the first release-detail fetch should *silently establish* the baseline (an `UPDATE ... SET track_count = $fresh` on the `new_release` row, or an explicit "establishing" marker) and fire **no** event — only a fetch *after* a baseline already exists should compare and potentially fire `deluxe_change`. This mutation of `track_count` on an existing row does not violate D-20, whose stated scope is specifically the display snapshot ("title, cover art URL, release date, artist name") "preserving the historical record as first detected" — `track_count` here is operational baseline state, not display data.
**Warning signs:** Every artist's entire back-catalog of release-groups generating a `deluxe_change` alert on the second poll cycle after being added (not the first — the first is correctly suppressed by D-04).
**This is an open design gap, not a locked CONTEXT.md decision** — flag prominently for the planner; the schema/algorithm above is a recommendation, not a mandate.

### Pitfall 2: `ON CONFLICT DO NOTHING` row-count semantics are not spelled out in official docs — verify with a real test, don't assume
**What goes wrong:** Code assumes `:execrows` returns `0` on a duplicate-key no-op and `1` on a real insert, without ever confirming it against a real Postgres instance — if this assumption is subtly wrong (e.g. under some transaction-isolation edge case), D-13's seed-mode / D-19's idempotency logic silently misbehaves.
**Why it happens:** The official Postgres `INSERT` docs describe the command-tag row count only as "the number of rows inserted or updated" `[CITED: postgresql.org/docs/current/sql-insert.html, fetched this session]` without explicitly addressing the `ON CONFLICT DO NOTHING` no-op case in the text this session could retrieve.
**How to avoid:** Write a real-Postgres test (`testutil.NewTestPool`, this project's established DB-test fixture) asserting `InsertEvent` returns `1` on first insert and `0` on a duplicate-key re-insert — exactly the kind of "verify Postgres's actual behavior, don't assume" test already present in this codebase (`UpsertArtist`'s COALESCE-refresh tests, `DeleteWatchlistEntry`'s `:execrows`-affected-row tests).
**Warning signs:** A seed-mode test that passes with a mocked/fake DB but the real behavior under CI's Postgres service container silently diverges.

### Pitfall 3: `RecordingsByArtist` returns the artist's own tracks too, not just guest features — filter before creating events
**What goes wrong:** A naive implementation that creates a `guest_feature` event for every recording returned by `/ws/2/recording?artist=X` massively over-notifies — this endpoint returns every recording the artist appears on in *any* credit position, including their own primary-artist tracks (i.e., their entire recorded catalog, likely a superset even larger than `ReleaseGroupsByArtist`'s results since compilations/live albums/territory variants multiply recording MBIDs beyond release-group count).
**Why it happens:** The endpoint's semantics are "recordings linked to this artist," not "recordings where this artist guests" — D-06's positional filter (`artist-credit[0].id != watched artist`) is what turns the raw fetch into guest-feature candidates; skipping it treats the whole catalog as guest appearances.
**How to avoid:** Apply `isGuestFeature` (Pattern 3) to every fetched recording before any event-creation logic runs. Also defensively guard `len(rec.ArtistCredit) == 0` before indexing `[0]` — external, semi-trusted JSON should never be assumed well-formed.
**Warning signs:** A newly-added prolific artist immediately generates hundreds of `guest_feature` events on their first poll cycle (seed mode, D-13) instead of a much smaller true-guest-appearance count.

### Pitfall 4: `RecordingsByArtist` pagination volume can exceed `ReleaseGroupsByArtist`'s by an order of magnitude
**What goes wrong:** D-05 locks the recording browse to the same bounded-pagination ceiling as release-groups (maxPages=10 × pageSize=100 = 1000-item ceiling). A prolific mainstream artist's total recording count (every track × every release/reissue/territory variant/live album/compilation it appears on) is typically far larger than their release-group count, so hitting the 1000-item truncation ceiling is materially more likely here than it was for `ReleaseGroupsByArtist` in Phase 3.
**Why it happens:** MusicBrainz assigns a distinct recording MBID per distinct audio master, and the same song can recur across dozens of releases/compilations, each contributing a browse-result entry (sometimes the *same* recording MBID reused across releases, sometimes a genuinely distinct one for a remaster/live version).
**How to avoid:** This is already a locked decision (D-05) — no code change needed — but the plan should log `item_count` at the recording-fetch step (mirroring the existing `poll result` log line) so a truncated fetch for a prolific artist is visible in structured logs rather than silently under-detecting guest features, matching the existing "truncation is a data-completeness limit, not a failure" convention `[VERIFIED: internal/musicbrainz/releasegroups.go:72-75]`.
**Warning signs:** A very prolific guest artist (frequent-collaborator producer/rapper) consistently shows `item_count` at or near 1000 in structured logs across every cycle.

### Pitfall 5: `new_release` events need a `source` discriminator — D-10's dedup key alone is ambiguous across MusicBrainz vs. Deezer
**What goes wrong:** D-10 defines the dedup key as "release-group MBID for `new_release`" without addressing that D-15's own rationale text implies Deezer polling *also* produces `new_release` events (from Deezer's own numeric album-id space, since dual-source reconciliation is explicitly out of scope). If the `events` table's uniqueness constraint is just `(event_type, external_id)` with no source column, a MusicBrainz release-group MBID and a Deezer album ID could theoretically collide as raw strings (extremely unlikely in practice — UUID-format vs. all-digit strings — but conflates two conceptually distinct ID spaces regardless), and D-14's "seed mode" / D-15's "per-source independent" seeding logic has no column to filter on.
**Why it happens:** CONTEXT.md's D-01 through D-20 focus on the MusicBrainz-only detection types (deluxe/guest) and D-09's "one combined table," but don't explicitly re-state that `new_release` is dual-source while resolving the resulting schema implication.
**How to avoid:** Add a `source TEXT NOT NULL CHECK (source IN ('musicbrainz','deezer'))` column; scope the unique dedup constraint to `(event_type, source, external_id)`; scope D-14's seed-mode "zero existing event rows" check to `WHERE artist_id=$1 AND source=$2` per D-15's explicit per-source independence.
**This is a genuine gap between CONTEXT.md's decisions and a working schema — flag for the planner**, not a locked answer; if the planner's read of D-10 differs (e.g., decides Deezer-sourced releases are simply out of scope for DTCT-01 this phase), that's a legitimate alternate resolution, but it should be a deliberate choice, not an accidental omission.

### Pitfall 6: MusicBrainz responses never carry a cover-art URL — Deezer's do
**What goes wrong:** Code written against D-12's "inline snapshot... cover art URL" assuming the field comes straight off the MusicBrainz JSON response (the way it does for Deezer, `[VERIFIED: live Deezer response, 03-RESEARCH.md:415]` `"cover": "https://api.deezer.com/album/983217461/image"`) will find no such field and either leave `cover_art_url` perpetually null for MusicBrainz-sourced events or attempt an unplanned extra API call at detection time.
**Why it happens:** MusicBrainz's core metadata API and its cover-art hosting (the Cover Art Archive, a separate Internet Archive-backed service) are deliberately decoupled projects.
**How to avoid:** For MusicBrainz-sourced events, construct the deterministic Cover Art Archive URL client-side — no extra HTTP call needed: `https://coverartarchive.org/release-group/{release_group_mbid}/front` `[CITED: musicbrainz.org/doc/Cover_Art_Archive/API, fetched this session — "For releases: http://coverartarchive.org/release/{MBID}/front... For release-groups: http://coverartarchive.org/release-group/{MBID}/"]`. This may 404 at render time if no art has been uploaded for that release-group — that's an accepted, expected gap (not this phase's problem to solve; Phase 5/6 renders it).
**Warning signs:** Discord embeds (Phase 5) showing a broken-image icon for every MusicBrainz-sourced alert.

### Pitfall 7: The `release_types` filter's "deluxe" value is not a MusicBrainz `primary-type` — it gates event *creation*, not a type match
**What goes wrong:** Code that tries to filter `new_release` events by checking `strings.EqualFold(group.PrimaryType, "deluxe")` will never match anything — MusicBrainz's `primary-type` field only takes values like `Album`/`Single`/`EP`/`Broadcast`/`Other` `[VERIFIED: live MusicBrainz response, 03-RESEARCH.md:371]` `"primary-type": "Album"` (capitalized); "Deluxe" is not a MusicBrainz primary-type or a standard secondary-type at all — it's exactly why D-01 chose a track-count heuristic over a title/type heuristic in the first place.
**Why it happens:** The watchlist's `release_types` vocabulary (`[VERIFIED: internal/watchlist/service.go:25]` `ReleaseTypes = []string{"album", "single", "ep", "deluxe"}`) mixes two different concerns: three real MusicBrainz primary-types (lowercased) that gate `new_release` events, and one pseudo-type (`"deluxe"`) that actually gates whether `deluxe_change` events are created for this artist at all.
**How to avoid:** D-17's filter check needs two separate code paths: (1) for `new_release` events, case-insensitively match `group.PrimaryType` against `entry.ReleaseTypes` restricted to `{album, single, ep}`; (2) for `deluxe_change` events, check `"deluxe" ∈ entry.ReleaseTypes` as a boolean gate on whether deluxe-change detection runs for this artist at all, independent of any MusicBrainz type field.
**Warning signs:** An artist whose `release_types` preference omits `"deluxe"` still receiving deluxe-change alerts (filter never applied because no MB field ever equals "deluxe" to match against), or an artist who *does* want deluxe alerts never receiving them (code tries to match a non-existent MB type value).

## Code Examples

### Migration — `000003_events.up.sql` (next sequential number, `[VERIFIED: internal/db/migrations directory listing, this session]` confirms `000001_init`, `000002_watchlist` exist, `000003_*` is next per the established convention `[VERIFIED: internal/db/migrations/000002_watchlist.up.sql:1-2]` "Phase 2 (watchlist-core) introduces the first two domain tables")
```sql
-- Phase 4 (detection-engine): the seen-store / event-log table (D-09). An
-- event row's existence IS "already seen" for its (event_type, source,
-- external_id) triple -- enforced by the unique constraint below, which
-- backs DTCT-04's idempotency via ON CONFLICT DO NOTHING (D-20).
--
-- source disambiguates the external_id namespace: MusicBrainz MBIDs
-- (UUID-format) vs. Deezer numeric album ids -- new_release events can
-- come from either poll cycle (D-15's per-source-independent seeding
-- implies this); deluxe_change and guest_feature are MusicBrainz-only
-- (D-03, D-08) but still carry source='musicbrainz' for schema
-- consistency and query simplicity.
--
-- release_group_mbid is populated for new_release rows (their own
-- external_id) and for deluxe_change rows (pointing back to the parent
-- group) -- it is what makes "highest previously-seen track count for
-- this group" (D-02) queryable without a second table.
--
-- track_count is mutable baseline-tracking state, NOT part of the D-12
-- display snapshot (title/artist_name/release_date/cover_art_url), which
-- stays write-once via ON CONFLICT DO NOTHING per D-20.
CREATE TABLE events (
    id                 BIGSERIAL PRIMARY KEY,
    artist_id          BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    source             TEXT NOT NULL CHECK (source IN ('musicbrainz','deezer')),
    event_type         TEXT NOT NULL CHECK (event_type IN ('new_release','guest_feature','deluxe_change')),
    external_id        TEXT NOT NULL,
    release_group_mbid TEXT,
    title              TEXT NOT NULL,
    artist_name        TEXT NOT NULL,
    release_date       TEXT,
    cover_art_url      TEXT,
    track_count        INT,
    notified_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT events_dedup_key UNIQUE (event_type, source, external_id)
);

-- Partial index: Phase 5's D-11 query is SELECT WHERE notified_at IS NULL.
CREATE INDEX events_unnotified_idx ON events (notified_at) WHERE notified_at IS NULL;

-- Speeds up D-14's per-source seed-mode check and D-02's per-group
-- baseline lookup.
CREATE INDEX events_artist_source_idx ON events (artist_id, source);
CREATE INDEX events_release_group_idx ON events (release_group_mbid) WHERE release_group_mbid IS NOT NULL;
```

```sql
-- 000003_events.down.sql
DROP TABLE events;
```
*Note: `event_type` reuses the exact three-string vocabulary already locked Go-side (`[VERIFIED: internal/watchlist/service.go:26]` `EventTypes = []string{"new_release", "guest_feature", "deluxe_change"}`) and DB-side (`[VERIFIED: internal/db/migrations/000002_watchlist.up.sql:42-43]` `CONSTRAINT watchlist_muted_event_types_valid CHECK (muted_event_types <@ ARRAY['new_release','guest_feature','deluxe_change']::text[])`) from Phase 2. Recommend keeping these values unchanged rather than exercising CONTEXT.md's canonical_refs note about a possible renaming follow-up migration — they already fit this phase's three event types exactly.*

### Seed-mode check (D-14, per-source per D-15)
```sql
-- queries/events.sql
-- name: HasAnyEvent :one
SELECT EXISTS(
    SELECT 1 FROM events WHERE artist_id = $1 AND source = $2
) AS has_any;
```

### Diff pattern: fresh vs. seen MBIDs (DTCT-01)
```go
// internal/detection/musicbrainz.go
func (d *Detector) newReleaseGroups(ctx context.Context, artistID int64, fresh []musicbrainz.ReleaseGroup) ([]musicbrainz.ReleaseGroup, error) {
	seenIDs, err := d.q.ListExternalIDs(ctx, sqlc.ListExternalIDsParams{
		ArtistID:  artistID,
		Source:    "musicbrainz",
		EventType: "new_release",
	})
	if err != nil {
		return nil, fmt.Errorf("detection: list seen release-groups: %w", err)
	}
	seen := make(map[string]struct{}, len(seenIDs))
	for _, id := range seenIDs {
		seen[id] = struct{}{}
	}

	var newGroups []musicbrainz.ReleaseGroup
	for _, g := range fresh {
		if _, ok := seen[g.MBID]; !ok {
			newGroups = append(newGroups, g)
		}
	}
	return newGroups, nil
}
```

## State of the Art

Not applicable in the traditional sense — MusicBrainz's `ws/2` API is a long-stable, versioned surface (same conclusion Phase 3's research reached). The operational reality that changed since Phase 3's research (2026-08-07) is **access**, not the API contract: MusicBrainz has tightened bot-blocking (Anubis / IP-blocking against AI-scraper load) since roughly March 2026 per this session's web search, which is a deployment/testing risk, not an API-shape risk.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `/ws/2/release?release-group={mbid}&inc=media&fmt=json` returns an envelope shaped `{"release-count":N,"release-offset":0,"releases":[{"id","title","status","date","media":[{"format","position","track-count"}]}]}` | Pattern 2, Code Examples | If field names differ (e.g. `track_count` vs `track-count`, or `media` nested differently for multi-disc), `ReleasesByReleaseGroup` silently decodes zero-value structs (Go's `encoding/json` ignores unknown fields and leaves missing fields at zero value) rather than erroring — DTCT-02 would silently never fire. **Mitigate:** write the httptest fixture test as the very first Wave 0 task, and if a live call becomes possible again before/during execution, re-verify against a real response before trusting the fixture. |
| A2 | `/ws/2/recording?artist={mbid}&inc=artist-credits&fmt=json` returns an envelope shaped `{"recording-count":N,"recording-offset":0,"recordings":[{"id","title","artist-credit":[{"name","joinphrase","artist":{"id","name"}}]}]}` | Pattern 3, Code Examples | Same risk class as A1 — DTCT-03 would silently never fire. `artist-credit` array + `joinphrase` field structure has partial corroboration via web search this session `[CITED: musicbrainz.org/doc/Artist_Credits]`; the top-level envelope naming convention (`recording-count`/`recording-offset`) is inferred by direct analogy to the **live-verified** `release-group-count`/`release-group-offset` pattern from Phase 3, not independently confirmed this session. |
| A3 | MusicBrainz recording/release JSON never includes a cover-art URL field, requiring the Cover Art Archive's deterministic URL convention instead | Common Pitfalls #6, Don't Hand-Roll | Low risk if wrong — worst case is a redundant/unused field in the decoded struct; the Cover Art Archive URL pattern itself is `[CITED]`, cross-checked against the official Cover Art Archive API doc this session |
| A4 | `INSERT ... ON CONFLICT DO NOTHING`'s reported affected-row count excludes conflicted (skipped) rows, i.e. `:execrows` returns 0 on a no-op conflict | Pattern 4, Common Pitfalls #2 | Medium risk if wrong — this is the mechanism DTCT-04's idempotency and D-13's seed-mode logic both lean on for "was this newly detected." Extremely well-established behavior across the Go+pgx+sqlc ecosystem, but not found explicitly stated in the official Postgres docs text retrieved this session — **must** be confirmed by a real-Postgres unit test before this phase's plan is trusted, not by documentation alone. |
| A5 | `new_release` detection runs against both MusicBrainz and Deezer poll cycles (not MusicBrainz-only), requiring the `source` discriminator column | Common Pitfalls #5, schema design | If actually MusicBrainz-only (a legitimate alternate reading the planner could choose), the `source` column and its query filters are still harmless (just always `'musicbrainz'`) — low risk either way, but the planner should make this a deliberate choice given CONTEXT.md doesn't explicitly resolve it |

**None of these assumptions block planning** — all degrade gracefully to "write slightly more defensive code / add one more Wave 0 test" rather than invalidating the phase's architecture. A1/A2 (the exact MusicBrainz JSON shapes) are the ones most worth a deliberate checkpoint: recommend the plan's first task for each new fetch method be RED-phase `httptest.Server` fixture tests using the shapes above, with an explicit note in the task that the fixture is `[ASSUMED]` pending live re-verification if MusicBrainz becomes reachable again.

## Open Questions (RESOLVED)

All three questions below were resolved during phase planning. Each carries an inline
`RESOLVED:` note naming the plan and task that settles it. Nothing here blocks execution.

1. **Does `new_release` detection apply to Deezer polling, or is it MusicBrainz-only for v1?**
   - What we know: D-03/D-08 explicitly restrict deluxe-change and guest-feature to MusicBrainz-only; D-15's own rationale ("an artist added MusicBrainz-only whose `deezer_id` is backfilled later would otherwise have their entire Deezer catalog dumped as 'new releases'") strongly implies Deezer-sourced `new_release` events are in scope, but D-10's dedup-key definition doesn't explicitly address a Deezer-sourced `new_release`'s ID space.
   - What's unclear: Whether the planner should treat this as settled (dual-source `new_release`, per D-15's rationale) or flag it back to the user as a scope clarification.
   - Recommendation: Treat as settled dual-source (the `source` column design in this research handles it cleanly either way, per Assumption A5's low-risk-either-way note) — re-litigating this with the user would likely just re-derive D-15's own stated rationale.
   - **RESOLVED:** Settled as dual-source, exactly as recommended. Plan `04-02` Task 3 ("Extend detection to the Deezer poll cycle as an independent second source") implements `DetectDeezer` writing `source='deezer'`, `event_type='new_release'` rows, per D-15's rationale. Its `TestDetectDeezer_SameIDDifferentSourceCoexist` and `TestDetectDeezer_SeedsIndependentlyOfMusicBrainz` cases pin the `source`-discriminated ID space that D-10 left implicit, which also confirms Assumption A5's `source` column is load-bearing rather than always-`'musicbrainz'`.

2. **Where does the deluxe-change baseline live?**
   - What we know: See Common Pitfalls #1 in full — this is the single most important open design question in this phase.
   - What's unclear: Whether the recommended `track_count` mutable column is the right shape, or whether the planner prefers a cleaner separation (e.g. a tiny second non-event `release_group_baselines` table, explicitly scoped outside D-09's "one combined table" language since D-09 is about the event/dedup log specifically, not all persisted poll-cycle state).
   - Recommendation: Either resolution works; the plan MUST address this explicitly (with a passing test proving the first-comparison-cycle does not false-positive) rather than leaving it implicit — this is the phase's highest-risk correctness gap.
   - **RESOLVED:** Escalated to the developer rather than decided by the planner, because it is a one-way door gated on migration `000003_events`. Plan `04-01` Task 1 is a `checkpoint:decision` ("Decide where the deluxe-change track-count baseline is persisted") presenting exactly the two shapes weighed here — option-a mutable `track_count` column on `events` (this research's recommendation, and the checkpoint's stated recommendation) vs. option-b a separate `release_group_baselines` table. The false-positive hazard is closed independently of which option wins: both are establish-then-compare, and Plan `04-04` Task 1's first-comparison-cycle behavior proves the baseline-establishing fetch fires no `deluxe_change` event.

3. **Exact request shape for `ReleasesByReleaseGroup`/`RecordingsByArtist` — live-unverified this session**
   - What we know: Envelope/field-name shapes are `[ASSUMED]` per A1/A2 above, by direct analogy to Phase 3's live-verified `release-group` browse shape.
   - What's unclear: Whether MusicBrainz will be reachable (from either this dev machine or Claude's own tooling) by the time this phase executes, given the broader March-2026 bot-blocking trend found this session.
   - Recommendation: Build and merge the httptest-fixture-driven unit tests regardless (CLAUDE.md already mandates no live calls in CI) — treat any live confirmation as a nice-to-have UAT step, not a blocker, exactly matching Phase 3's already-accepted precedent (03-VERIFICATION.md Acknowledged Gaps).
   - **RESOLVED:** Fixture-based TDD, as recommended — the `[ASSUMED]` shapes are pinned by `httptest.Server` fixtures written before the client code, never by live calls. `RecordingsByArtist` (A2) is covered by Plan `04-03` Task 1 (`TestRecordingsByArtist_RequestShape` asserts the emitted query params; `_DecodesEnvelope` and `_Paginates` pin the envelope). `ReleasesByReleaseGroup` (A1) is covered by Plan `04-04` Task 1 (`TestReleasesByReleaseGroup_RequestShape`, `_DecodesEnvelope`, `_Paginates`). Live re-verification against musicbrainz.org stays best-effort UAT per Phase 3's accepted precedent (03-VERIFICATION.md Acknowledged Gaps) and is explicitly not blocking.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building/testing `internal/detection`, extended `internal/musicbrainz` | ✓ | go1.26.5 windows/amd64 `[VERIFIED: this session, go version]` | — |
| Docker (daemon) | `docker-compose.yml` Postgres fixture for `testutil.NewTestPool`-backed detection tests | ✓ (client responsive) | Docker Client 29.6.2 `[VERIFIED: this session, docker info]` | — |
| Outbound reachability to `musicbrainz.org` | Live UAT/manual verification of the two new endpoints; NOT required for CI | ✗ this session — every `WebFetch` attempt against `musicbrainz.org` failed (socket error / Anubis bot-block page) `[VERIFIED: this session, direct tool attempts]`; consistent with, and possibly broader than, PROJECT.md's already-documented WSL2-specific TLS failure | CI never needs this (CLAUDE.md mandates `httptest.Server` mocks); treat as an environmental gap exactly like Phase 3's, see 03-VERIFICATION.md Acknowledged Gaps |
| Outbound reachability to `api.deezer.com` | Live UAT of Deezer-sourced `new_release` detection | ✓ `[VERIFIED: this session, live WebFetch response received]` | — | — |

**Missing dependencies with no fallback:** none — all detection logic is testable without live MusicBrainz access per CLAUDE.md's testing constraint.

**Missing dependencies with fallback:** MusicBrainz live reachability — expect this phase's UAT to hit the same acknowledged-gap pattern as Phase 3 (see PROJECT.md Context, Broken Windows Ledger entry #3). This appears to be worsening (industry-wide bot-blocking trend, not just this dev machine's WSL2 path) — worth a one-line note in STATE.md's Blockers/Concerns once this phase starts execution.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (unchanged from Phases 1–3) |
| Config file | none — plain `go test ./...` |
| Quick run command | `go test -short ./...` (skips DB-backed tests, `[VERIFIED: internal/testutil/postgres.go:29-31]` `testing.Short()` gate) |
| Full suite command | `TEST_DATABASE_URL=... go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DTCT-01 | New release-group creates a `new_release` event row; a re-detected group does not duplicate it | integration (real Postgres, `testutil.NewTestPool`) | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectMusicBrainz_NewRelease` | ❌ Wave 0 — `internal/detection/musicbrainz_test.go` |
| DTCT-02 | Expanded tracklist on an already-seen group fires `deluxe_change`; first-ever comparison cycle does NOT false-positive (Pitfall #1) | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectMusicBrainz_DeluxeChange` | ❌ Wave 0 |
| DTCT-02 | `ReleasesByReleaseGroup` decodes the assumed JSON shape correctly, sums multi-disc `track-count` | unit (httptest.Server fixture) | `go test ./internal/musicbrainz/... -run TestReleasesByReleaseGroup -short` | ❌ Wave 0 — `internal/musicbrainz/releases_test.go` |
| DTCT-03 | Non-primary artist-credit recording fires `guest_feature`; primary-credit recordings never do (Pitfall #3) | unit (httptest fixture) + integration | `go test ./internal/detection/... -run TestIsGuestFeature -short` then `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature` | ❌ Wave 0 — `internal/detection/musicbrainz_test.go` |
| DTCT-03 | `RecordingsByArtist` decodes the assumed JSON shape, paginates bounded at maxPages=10 | unit (httptest.Server fixture) | `go test ./internal/musicbrainz/... -run TestRecordingsByArtist -short` | ❌ Wave 0 — `internal/musicbrainz/recordings_test.go` |
| DTCT-04 | `InsertEvent` returns 1 on first insert, 0 on duplicate dedup key (Pitfall #2, Assumption A4) | integration (real Postgres) | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestInsertEvent_Idempotent` | ❌ Wave 0 |
| DTCT-04 | D-13 seed-mode: first cycle for a new artist pre-sets `notified_at`; D-16: removing/re-adding an artist does not re-seed (event rows survive watchlist row deletion via artist_id, not watchlist id) | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetector_SeedMode` | ❌ Wave 0 |
| DTCT-05 | Already covered — `[VERIFIED: internal/poller/poller.go:92-93,183-187,232-236]`, no new test needed | — | (existing `internal/poller/poller_test.go` coverage from Phase 3) | ✓ already exists |
| D-17/D-18 | Release-type / muted-event-type filters gate event creation before insert (Pitfall #7 for the `deluxe` pseudo-type case specifically) | unit | `go test ./internal/detection/... -run TestFilter -short` | ❌ Wave 0 — `internal/detection/filter_test.go` |

### Sampling Rate
- **Per task commit:** `go test -short ./...`
- **Per wave merge:** `TEST_DATABASE_URL=... go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/musicbrainz/releases_test.go` — httptest.Server fixture for `/ws/2/release?release-group=` using the `[ASSUMED]` shape in Pattern 2 (A1)
- [ ] `internal/musicbrainz/recordings_test.go` — httptest.Server fixture for `/ws/2/recording?artist=` using the `[ASSUMED]` shape in Pattern 3 (A2)
- [ ] `internal/db/migrations/000003_events.up.sql` + `.down.sql` — new table
- [ ] `queries/events.sql` — `InsertEvent`, `HasAnyEvent`, `ListExternalIDs`, a track-count baseline query (shape depends on Open Question 2's resolution), `ListUnnotified` (D-11, Phase 5 groundwork)
- [ ] `internal/detection/` — new package, no existing tests
- [ ] A real-Postgres test proving `ON CONFLICT DO NOTHING`'s row-count semantics (Assumption A4) before any code relies on it

*(No new test framework install needed — `go test` remains this project's only test tool.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | No new HTTP-facing surface this phase — detection runs entirely inside the existing poller |
| V3 Session Management | No | No session state introduced |
| V4 Access Control | No | No new API routes this phase |
| V5 Input Validation | Yes | External MusicBrainz JSON (`artist-credit` array, `media` array) is semi-trusted data — guard against empty/malformed arrays before indexing (Pitfall #3's `len(rec.ArtistCredit) == 0` check); sqlc's parameterized queries already prevent SQL injection on any string field persisted from the API response (title, artist name, etc.) |
| V6 Cryptography | No | No secrets/crypto introduced |
| V13 API and Web Service | Yes | The two new MusicBrainz fetch methods must route through the existing `doRequest` helper (`[VERIFIED: internal/musicbrainz/client.go:112-135]`) so they inherit rate-limiting and the mandatory `User-Agent` header — a new fetch method that bypasses `doRequest` and calls `httpClient.Do` directly would silently escape both protections |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed/adversarial `artist-credit` array (empty, or first entry missing `artist.id`) causing a panic on `[0]` index | Denial of Service | Defensive length + nil checks before indexing (Pitfall #3) |
| A very prolific artist's fetch volume (Pitfall #4) exhausting the poll cycle's time budget across many artists | Denial of Service (self-inflicted, resource exhaustion) | Already mitigated by D-05's bounded pagination ceiling; no new mitigation needed, just visibility via structured logging |
| Storing unsanitized external title/artist-name strings that later render in Discord embeds (Phase 5) or a web UI (Phase 6) | Tampering / Injection (stored XSS if rendered unescaped in Phase 6's UI) | Out of this phase's scope to fix (Phase 5/6 own their render paths), but worth noting in the snapshot data's provenance — MusicBrainz/Deezer titles are user-editable community-wiki content and should be treated as untrusted strings by any downstream renderer, not just this phase's storage layer |

## Sources

### Primary (HIGH confidence — in-repo, read this session)
- `[VERIFIED: internal/poller/poller.go]` — full file read, `RunMusicBrainzCycle`/`RunDeezerCycle`, `mbRunning`/`dzRunning` overlap guards, `ReleaseGroupSource`/`AlbumSource` seams
- `[VERIFIED: internal/musicbrainz/releasegroups.go, client.go, search.go]` — bounded pagination pattern, `doRequest` helper, existing struct/envelope conventions
- `[VERIFIED: internal/deezer/albums.go]` — confirms `Album.Tracklist` is a URL not real data (D-03), no per-track credit fetch (D-08)
- `[VERIFIED: internal/watchlist/service.go]` — `Entry`, `ReleaseTypes`/`EventTypes` vocab, `Store` interface
- `[VERIFIED: internal/db/migrations/000002_watchlist.up.sql, .down.sql]` — CHECK constraint vocab, migration file convention
- `[VERIFIED: queries/artists.sql, queries/watchlist.sql]` — `ON CONFLICT`/`:execrows` idioms already in this codebase
- `[VERIFIED: internal/db/sqlc/models.go]` — generated struct shape convention (`pgtype.Timestamptz`, nullable `*string`)
- `[VERIFIED: internal/testutil/postgres.go]` — `RequirePostgresDSN`/`NewTestPool` DB-test fixture pattern
- `[VERIFIED: internal/watchlist/service_test.go]` — real-Postgres test convention (unique-per-test data via `t.Name()` hash, `t.Cleanup`)
- `[VERIFIED: internal/musicbrainz/releasegroups_test.go]` — httptest.Server fixture-test pattern to mirror for the two new fetch methods
- `[VERIFIED: go.mod]` — full file read, confirms zero new dependencies needed
- `[VERIFIED: .planning/phases/03-external-clients-search/03-RESEARCH.md]` — Phase 3's live-verified MusicBrainz/Deezer response shapes, reused by direct analogy for this phase's two new endpoints
- `[VERIFIED: this session]` — `go version`, `docker info` environment probes

### Secondary (MEDIUM confidence — official docs, cross-checked or partially corroborated)
- `docs.sqlc.dev/en/latest/reference/query-annotations.html` — `:execrows` semantics, fetched this session
- `musicbrainz.org/doc/Cover_Art_Archive/API` (via web search corroboration) — cover-art URL pattern, cross-checked
- `musicbrainz.org/doc/Artist_Credits` (via web search corroboration) — `artist-credit`/`joinphrase` array shape, partially corroborated
- `postgresql.org/docs/current/sql-insert.html` — INSERT command-tag row-count wording, fetched this session but explicitly notes ambiguity on the `ON CONFLICT DO NOTHING` case

### Tertiary (LOW confidence — training knowledge, not independently verified this session)
- Exact JSON field names/envelope shape for `/ws/2/release?release-group=&inc=media` and `/ws/2/recording?artist=&inc=artist-credits` (Assumptions A1/A2) — live verification blocked this session (WebFetch socket errors + Anubis bot-block), reconstructed by direct analogy to Phase 3's live-verified `release-group` browse envelope

## Metadata

**Confidence breakdown:**
- Standard Stack / no-new-dependencies: HIGH — `go.mod` read directly this session, zero ambiguity
- Architecture / schema design: MEDIUM-HIGH — the core patterns (narrow-interface seam, `ON CONFLICT DO NOTHING`) are HIGH (grounded in in-repo precedent); the deluxe-change baseline design (Pitfall #1) and the `source` column (Pitfall #5) are genuine gaps this research surfaces and recommends resolutions for, not locked facts
- External API mechanics (the two new MusicBrainz endpoints): MEDIUM — envelope/field shapes are `[ASSUMED]` by strong analogy to Phase 3's live-verified sibling endpoints, not independently re-verified this session due to an environmental block
- Pitfalls: HIGH for Pitfalls #1/#2/#5 (original analysis grounded in this session's in-repo reads of CONTEXT.md's exact decision text); MEDIUM for #3/#4/#6/#7 (well-established MusicBrainz domain knowledge, #6 cross-checked via web search this session)

**Research date:** 2026-08-08
**Valid until:** 2026-09-07 (30 days) — re-verify the two new MusicBrainz endpoint shapes (A1/A2) via a live call or Context7/official-docs lookup before/during execution if MusicBrainz reachability is restored, since this is the one area of genuinely unverified risk in an otherwise stable, well-understood domain
