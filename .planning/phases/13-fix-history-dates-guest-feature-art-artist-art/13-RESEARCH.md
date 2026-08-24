# Phase 13: Fix History Dates, Guest-Feature Art & Artist Art - Research

**Researched:** 2026-08-24
**Domain:** Go backend (MusicBrainz/Deezer client extension, detection-engine insertion, sqlc queries), React frontend (EventCard rendering)
**Confidence:** MEDIUM (backend seams and existing code are HIGH/VERIFIED; the new MusicBrainz recording-lookup response shape is ASSUMED, matching this codebase's own established convention for unreachable/unverified MusicBrainz endpoints)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Guest-feature date & cover-art sourcing**
- D-01: For each genuinely NEW guest-feature recording (only on insert, not for every row `RecordingsByArtist` returns), do an extra per-event MusicBrainz lookup: `GET /ws/2/recording/{mbid}?inc=releases+release-groups`. Use it to get a release-group MBID (cover art via the existing `coverArtURLForReleaseGroup` helper) and a release date. Shares the existing MusicBrainz rate limiter; cost is bounded to actual new guest-feature insertions per cycle. Reversibility: reversible.
- D-02: When a recording has releases with different dates, use the **earliest** release date among them.
- D-03: When the lookup finds no releases at all, behave exactly like today: placeholder cover art, `event.release_date` stays null.
- Note for planner: `internal/musicbrainz` needs a new client method (e.g. `ReleasesForRecording`) analogous to `ReleasesByReleaseGroup`. `detectGuestFeatures`'s `InsertEvent` call gets the new `ReleaseDate`/`CoverArtUrl` args wired in.

**Deluxe-change date rendering**
- D-04: Add the release date to `DeluxeChangeBody` on the same line, before the track-count delta: `"{date} · {prev} → {current} tracks"`.
- D-05: Fall back to the same `"Release date unknown"` text `NewReleaseBody` already uses when `release_date` is null.
- Note for planner: frontend-only change — `release_date` is already populated in the DB; confirm the TS type includes it.

**Artist-art backfill scope**
- D-06: Artist-art matching applies BOTH to new adds going forward AND as a one-time backfill over artists already on the watchlist with `image_url IS NULL`.
- D-07: The backfill runs as a one-time migration/startup pass, sweeping every artist with `image_url IS NULL` once when the app starts running this phase's code — not folded into the poll cycle, not a scheduled job.
- Note for planner: backfill scope is artist rows only (`artists.image_url`/`artists.deezer_id`) — this phase does not touch event/detection logic.

**Artist match confidence & tie-breaking**
- D-08: Search Deezer by artist name; require close name equality as the PRIMARY match signal; use a shared album/release title only as a TIE-BREAKER when multiple same-named Deezer artists come back — never as the sole match signal.
- D-09: Fail closed on no confident match: leave `image_url` (and `deezer_id`, if not otherwise supplied) NULL rather than attach a possibly-wrong artist's photo. Reversibility: reversible (threshold tunable later; the *choice* to fail closed is locked).

### Claude's Discretion
None — every gray area reached an explicit user decision this session. (D-01's "which recording endpoint/method name" implementation detail is left to the planner/researcher, per the note under D-01 — resolved below.)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

No REQUIREMENTS.md IDs are mapped to this phase. Per the phase description and CONTEXT.md, the `<decisions>` block above is the authoritative scope — there is no separate requirement-ID table to reconcile.
</phase_requirements>

## Summary

This phase is three narrow, independently-shippable bug fixes plus one absorbed backfill, all confirmed against actual code this session. Two of the three bugs (deluxe-change date, artist art) are almost entirely mechanical once the seams below are used correctly; the guest-feature date/art fix is the only piece that touches a genuinely new MusicBrainz API shape this codebase has never called before (a single-entity lookup, not a browse), and that shape could not be verified against a live response this session (MusicBrainz lookups by a fabricated MBID 404'd, as expected) — it is tagged `[ASSUMED]` below and should be re-verified against a live response at execution time, exactly the same caution this project's own `04-RESEARCH.md` already applied to the sibling `Recording`/`Release` browse shapes.

A significant finding not called out explicitly in CONTEXT.md's Decisions: `EventItem`'s TypeScript type **already** exposes `release_date` (`web/app/lib/api.ts:21`), and the Go `Event` struct already serializes it for every event type (`internal/events/service.go:41`). CONTEXT.md's flagged uncertainty here is resolved — no `api.ts` change is needed for D-04/D-05. Similarly, `artists.deezer_id` and `artists.image_url` columns **already exist** in the schema (`000002_watchlist.up.sql`), so no new migration is needed for the artist-art bug — only a new sqlc query and a backfill/add-time write path.

A second finding CONTEXT.md's Decisions section does not explicitly cover: the Phase Boundary goal states History cards for "single/feature/deluxe" all lack dates, but the locked Decisions (D-01–D-05) only wire a date into `new_release` (already working), `deluxe_change` (D-04/D-05), and the *backend* storage for `guest_feature` (D-01–D-03) — no decision assigns a **frontend** rendering change to `GuestFeatureBody`, which today renders no date at all. Once D-01 populates `release_date` for guest_feature rows, that data has nowhere to render without an additional `GuestFeatureBody` edit. This gap is flagged prominently under Open Questions — it is very likely in-scope (the phase's own stated goal requires it) but wasn't discussed as a named decision.

**Primary recommendation:** Extend `internal/musicbrainz` with a single-entity `ReleasesForRecording` lookup method (new file, mirrors `releases.go`'s error/rate-limit conventions but NOT its pagination), widen the existing `RecordingSource` interface in `internal/detection/detector.go` (rather than `Detector`'s constructor) so `detectGuestFeatures` gains the new call with minimal test-surface churn, fix `DeluxeChangeBody` (and almost certainly `GuestFeatureBody`) in `EventCard.tsx`, and build a small new `internal/artistart`-style package wrapping `deezer.Client.SearchArtists` for D-08/D-09's name-match logic, called from both `watchlist.Service.Add` (D-06 add-time) and a new one-time startup pass in `cmd/server/main.go` (D-07 backfill) — both writing through the already-non-destructive `UpsertArtist` query.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Guest-feature release-date/cover-art sourcing | API/Backend (`internal/musicbrainz`, `internal/detection`) | — | New external HTTP call + diff/insert logic; no frontend or DB schema change needed (columns already exist) |
| Deluxe-change date rendering | Browser/Client (`web/app/components/history/EventCard.tsx`) | — | Pure presentational change; data already flows through the API unchanged |
| Guest-feature date rendering (the unassigned gap) | Browser/Client (`EventCard.tsx`) | — | Same file/pattern as deluxe-change; blocked only by D-01 supplying the data |
| Artist-art matching (name search + confidence gate) | API/Backend (new package, `internal/deezer`) | — | Third-party API call + fuzzy-match logic; must never live in the browser tier (no Deezer credentials/rate-limit budget there) |
| Artist-art add-time wiring | API/Backend (`internal/watchlist.Service.Add` or `internal/httpserver`) | — | Runs synchronously inside the existing add request; DB write via existing `UpsertArtist` |
| Artist-art backfill sweep | API/Backend (`cmd/server/main.go`, one-time startup pass) | Database/Storage (`artists` table) | A batch job, not a request-scoped operation; owned by the single-binary process's boot sequence, per D-07 |

## Standard Stack

No new external dependencies. This phase extends existing hand-rolled clients (`internal/musicbrainz`, `internal/deezer`) and existing sqlc-generated queries — it does not introduce a new library.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| (none new) | — | — | Phase reuses `internal/musicbrainz`, `internal/deezer`, `sqlc`, `pgx/v5` already locked project-wide |

### Alternatives Considered
Not applicable — no new library decision in this phase.

**Installation:** No new packages to install.

## Package Legitimacy Audit

**Not applicable.** This phase introduces zero new Go modules and zero new npm packages — every capability is built from clients, interfaces, and query files that already exist in this repository (`internal/musicbrainz`, `internal/deezer`, `internal/db/sqlc`, `queries/artists.sql`). No `npm view` / `pip index` / package-legitimacy check is required.

## Architecture Patterns

### System Architecture Diagram

```
Guest-feature date/art (D-01–D-03):

  robfig/cron poll cycle
        │
        ▼
  DetectMusicBrainz(entry, groups)
        │
        ▼
  detectGuestFeatures(entry)
        │
        ├─► d.recordings.RecordingsByArtist(mbid)   [existing browse call]
        │         │
        │         ▼
        │   for each rec not in seen-set:
        │         │
        │         ▼
        │   d.recordings.ReleasesForRecording(rec.MBID)   ◄── NEW per-event lookup
        │         │  GET /ws/2/recording/{mbid}?inc=releases+release-groups
        │         ▼
        │   pick earliest-dated release → { Date, ReleaseGroup.MBID }
        │         │
        │         ▼
        └─► d.insertEvent(InsertEventParams{ ReleaseDate, CoverArtUrl, ... })
                  │
                  ▼
            events table (release_date, cover_art_url columns already exist)
                  │
                  ▼
            GET /events → EventItem.release_date/cover_art_url (already wired)
                  │
                  ▼
            EventCard.tsx → GuestFeatureBody (NEEDS a rendering addition — see Open Questions)


Artist-art match + backfill (D-06–D-09):

  (a) Add-time                              (b) Startup backfill (D-07)
  POST /watchlist                           cmd/server/main.go run()
        │                                          │
        ▼                                          ▼
  watchlist.Service.Add(p)                  ListArtistsMissingImage()  ◄── NEW sqlc query
        │  p.ImageURL == nil?                      │  (image_url IS NULL)
        ▼                                          ▼
  artistart.Match(ctx, name)  ◄────────── shared matching logic ──────────┐
        │  deezer.Client.SearchArtists(name)                              │
        │  D-08: close name match, tie-break on shared album title        │
        │  D-09: fail closed → nil, nil, matched=false                    │
        ▼                                                                 │
  UpsertArtist(mbid, deezer_id?, name, disambiguation=nil, image_url?) ◄──┘
        │  (COALESCE-based: nil fields never blank an existing value)
        ▼
  artists table (image_url, deezer_id already exist — no migration)
        │
        ▼
  GET /watchlist → WatchlistEntry.image_url → CoverArt component (already null-safe)
```

### Recommended Project Structure

No new top-level directories are required except one small new backend package for the artist-art matcher:

```
internal/
├── musicbrainz/
│   └── recording_lookup.go      # NEW: ReleasesForRecording + its response types
├── detection/
│   └── musicbrainz.go           # detectGuestFeatures: new lookup call + insertEvent wiring
├── artistart/                   # NEW package (name TBD by planner) — D-08/D-09 match logic
│   └── match.go
├── db/sqlc/                     # regenerated after queries/artists.sql gains a new query
web/app/components/history/
└── EventCard.tsx                 # DeluxeChangeBody + (very likely) GuestFeatureBody edits
```

### Pattern 1: Single-entity MusicBrainz lookup vs. existing browse pattern

**What:** Every existing MusicBrainz client method (`SearchArtists`, `ReleaseGroupsByArtist`, `RecordingsByArtist`, `ReleasesByReleaseGroup`) is a *browse* call: a query-param filter (`?artist=<mbid>` or `?release-group=<mbid>`), a paginated envelope with `<entity>-count`/`<entity>-offset` fields, and a bounded sequential pagination loop (`maxReleaseGroupPages`/`maxRecordingPages`/`maxReleasePages`, all `= 10`). D-01's new call is structurally different: it is an **entity lookup by MBID in the URL path** (`/recording/{mbid}`), with `inc=releases+release-groups` requesting nested linked data, and the response is the recording entity itself — not a count/offset envelope.

**When to use:** Use the lookup-by-path-segment shape (this pattern), not the browse-by-query-param shape, for `ReleasesForRecording`.

**Example (mirrors `internal/musicbrainz/releases.go`'s conventions for rate-limiting/error-handling, adapted for a lookup instead of a browse):**
```go
// Source: pattern verified against internal/musicbrainz/releases.go (VERIFIED,
// read this session); the recording-lookup URL shape and the "linked entities
// capped at 25" fact are [CITED: musicbrainz.org/doc/MusicBrainz_API] (fetched
// this session); the exact combined "releases[].release-group" nesting is
// [ASSUMED] -- see Assumptions Log.
func (c *Client) ReleasesForRecording(ctx context.Context, mbid string) ([]RecordingRelease, error) {
	trimmed := strings.TrimSpace(mbid)
	if trimmed == "" {
		return nil, ErrEmptyMBID
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	u, err := url.Parse(c.baseURL + "/recording/" + url.PathEscape(trimmed))
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("inc", "releases+release-groups")
	q.Set("fmt", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: releases for recording: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz: releases for recording: unexpected status %d", resp.StatusCode)
	}

	var env recordingLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("musicbrainz: decode recording lookup response: %w", err)
	}
	return env.Releases, nil
}
```

No pagination loop is needed for v1: MusicBrainz caps linked entities at 25 per lookup (`[CITED: musicbrainz.org/doc/MusicBrainz_API]`, fetched this session — "the number of linked entities returned is always limited to 25"), and a single recording having more than 25 distinct releases is not a realistic case for this project's domain (unlike a release-group's release count, which `ReleasesByReleaseGroup` does bound at 10 pages for). Document the 25-item cap as an accepted, low-probability truncation edge (mirroring this codebase's existing "page ceiling reached" logging idiom) rather than building pagination machinery for it.

### Pattern 2: Widen `RecordingSource`, don't widen `Detector`'s constructor

**What:** `Detector`'s constructor (`internal/detection/detector.go:54`) is `func New(q sqlc.Querier, recordings RecordingSource, releases ReleaseDetailSource) *Detector` `[VERIFIED: internal/detection/detector.go:54]`. `RecordingSource` today is:
```go
// [VERIFIED: internal/detection/detector.go:27-29]
type RecordingSource interface {
	RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error)
}
```
`cmd/server/main.go:135` wires `detection.New(sqlc.New(pool), mbClient, mbClient)` `[VERIFIED: cmd/server/main.go:135]` — the same `*musicbrainz.Client` instance satisfies both `RecordingSource` and `ReleaseDetailSource` today, and would trivially satisfy a third method too.

**When to use:** Add `ReleasesForRecording(ctx context.Context, mbid string) ([]musicbrainz.RecordingRelease, error)` **to the existing `RecordingSource` interface**, not as a third constructor parameter. `detectGuestFeatures` already holds `d.recordings` (a `RecordingSource`) and is exactly the method that needs the new call — no signature widening of `Detector`/`New` is required, and `cmd/server/main.go`'s wiring line does not change at all (mbClient already implements both existing methods and would trivially implement a third).

**Why this matters:** `internal/detection/detector_test.go` constructs `detection.New(...)` with a single shared `fakeRecordingSource{}`/`&fakeReleaseDetailSource{}` pair at **over 30 call sites** `[VERIFIED: grep of internal/detection/detector_test.go, 30+ matches for "detection.New(...fakeRecordingSource...)"]`. Widening `RecordingSource`'s interface only requires adding one new method to the single `fakeRecordingSource` type (`internal/detection/detector_test.go:42`) — a one-place edit. Adding a third constructor parameter to `Detector`/`New` would require editing every one of those 30+ call sites, a much larger and riskier diff for the same outcome.

### Anti-Patterns to Avoid
- **Adding a 4th sqlc.New(pool) instance just for the backfill:** `cmd/server/main.go` already constructs three (`store`, `detector`, `eventsStore`, `notif` — actually four, all wrapping the same `pool`) `sqlc.Queries` instances `[VERIFIED: cmd/server/main.go:104,112,135,158]` — this is the established, intentional pattern (`sqlc.Queries` is a stateless wrapper). The backfill pass should follow the same idiom (its own `sqlc.New(pool)`, or reuse `store`'s underlying `sqlc.Querier` if convenient) rather than inventing a different DB-access pattern.
- **Re-deriving "is this artist-art match confident" logic in two places:** CONTEXT.md's own Integration Points note explicitly warns against duplicating match logic between the add-time and backfill call sites. Put D-08/D-09's decision logic in exactly one function (in the new `internal/artistart`-style package) that both call sites invoke.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Non-destructive artist metadata update (image_url/deezer_id) | A new `UPDATE artists SET image_url = ...` query for the backfill | The existing `UpsertArtist` query (`queries/artists.sql`) | Already implements exactly the semantics both call sites need: `COALESCE(EXCLUDED.image_url, artists.image_url)` means passing `nil` for fields the caller doesn't know about never blanks an existing value `[VERIFIED: internal/db/sqlc/artists.sql.go:17-19, quoted below]`. Building a second, narrower update query would duplicate this non-destructive-merge logic and risk it drifting out of sync with `UpsertArtist`'s own semantics. |
| Cover-art URL construction for guest-feature rows | A second cover-art URL builder | `coverArtURLForReleaseGroup(mbid string) string` (`internal/detection/musicbrainz.go:507`) `[VERIFIED: internal/detection/musicbrainz.go:507-509]` | Deterministic Cover Art Archive URL pattern, already used identically by `new_release` and `deluxe_change` — no new cover-art mechanism needed per D-01. |
| Nullable-string conversion for the new ReleaseDate field | A second empty-string-to-nil helper | `nullableString(s string) *string` (`internal/detection/detector.go:108`) `[VERIFIED: internal/detection/detector.go:104-113, quoted below]` | Exact same convention `new_release`/`deluxe_change` already use for `ReleaseDate`. |

**Key insight:** Nearly everything this phase needs already exists as a reusable seam in this codebase — the actual net-new code is the MusicBrainz lookup method, the artist-name-match function, one new sqlc query, and two small frontend edits. Resist the temptation to build parallel versions of `UpsertArtist`, `coverArtURLForReleaseGroup`, or `nullableString`.

## Common Pitfalls

### Pitfall 1: Comparing MusicBrainz partial dates as plain strings is (usually) safe — but know why
**What goes wrong:** D-02 requires picking the "earliest" release date among a recording's linked releases. MusicBrainz dates are opaque, variable-precision strings (`"2020"`, `"2020-01"`, `"2020-01-01"`, or `""` for undated) `[VERIFIED: internal/musicbrainz/releases.go:52-55, quoted: "Date is kept as the opaque partial-date string MusicBrainz returns"]`.
**Why it happens:** A naive "parse as a date and compare" approach either fails on partial dates or silently invents a day/month the source data never asserted (the exact rationale `ReleaseGroup.FirstReleaseDate` and `Release.Date` already document for *not* parsing into `time.Time` `[VERIFIED: internal/musicbrainz/releasegroups.go:36-40, internal/musicbrainz/releases.go:52-55]`).
**How to avoid:** Plain lexicographic string comparison of two non-empty MusicBrainz partial dates correctly reproduces chronological ordering in this codebase's actual usage, because every value always starts with a 4-digit year and a less-precise prefix (`"2020"`) is guaranteed to be a string-prefix of any more-precise value sharing that year (`"2020-01"`, `"2020-01-01"`) — Go's `<` on strings is safe here. **Always filter out empty-string dates before comparing** (an empty string sorts first lexicographically, which would incorrectly "win" as earliest).
**Warning signs:** A guest_feature event's rendered date is suspiciously always the release with the least-precise date, or a comparison including an undated release silently produces a null date when a real dated release was available.

### Pitfall 2: The new per-recording lookup response shape is unverified this session
**What goes wrong:** Trusting the `[ASSUMED]` `releases[].release-group.id` nesting below without a live-response check could make the new field silently decode to its zero value (Go's `encoding/json` never errors on a field-name mismatch) — exactly the failure mode `04-RESEARCH.md` already documented for this project's other two MusicBrainz assumption-tagged shapes (`ReleaseGroup`, `Release`) `[VERIFIED: internal/musicbrainz/recordings.go:37-47, internal/musicbrainz/releases.go:33-43 — both carry the identical warning comment]`.
**Why it happens:** This developer's WSL2 network path cannot reach `musicbrainz.org` (`[VERIFIED: .planning/STATE.md, Blockers/Concerns — "musicbrainz.org's TLS handshake fails from this developer's WSL2 network path"]`), and this research session's own two live-lookup attempts against fabricated recording MBIDs both 404'd (expected — the MBIDs were not real) rather than confirming the shape.
**How to avoid:** Before or during implementation, run one real `curl`/`WebFetch` against a known-real MusicBrainz recording MBID with `?inc=releases+release-groups&fmt=json` from an environment that can reach `musicbrainz.org`, and adjust the `RecordingRelease`/`recordingLookupResponse` struct tags if the actual shape differs. Add the same kind of `[ASSUMED]` doc-comment warning this codebase's existing `Recording`/`Release` types already carry.
**Warning signs:** `detectGuestFeatures`'s new lookup silently never populates `ReleaseDate`/`CoverArtUrl` for genuinely-released guest features (DTCT-03-equivalent of `04-RESEARCH.md`'s Common Pitfall #3 — a silently-zero-valued field looks identical to "no releases found," so D-03's fallback path masks the bug).

### Pitfall 3: Reordering `cmd/server/main.go`'s construction sequence for the backfill/add-time matcher
**What goes wrong:** `dzClient` (the `*deezer.Client` the artist-art matcher needs) is currently constructed *after* `store := watchlist.NewService(sqlc.New(pool))` `[VERIFIED: cmd/server/main.go:104 precedes 145]`. If `watchlist.Service.Add` gains a dependency on a Deezer-backed matcher (D-06), `dzClient`'s construction must move above `store`'s, or the matcher must be injected via a setter/later-wiring step.
**How to avoid:** Move `dzClient := deezer.NewClient(dzLimiter, nil)` (and its `dzLimiter`) above the `store := watchlist.NewService(...)` line, or pass the matcher into `NewService` as an added constructor parameter once `dzClient` exists — either way, this is a small, mechanical reordering, not a redesign.

## Code Examples

### `UpsertArtist` — the non-destructive write seam both D-06 call sites should reuse verbatim
```sql
-- Source: queries/artists.sql (VERIFIED, read this session)
INSERT INTO artists (mbid, deezer_id, name, disambiguation, image_url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (mbid) DO UPDATE
    SET name = EXCLUDED.name,
        deezer_id = COALESCE(EXCLUDED.deezer_id, artists.deezer_id),
        disambiguation = COALESCE(EXCLUDED.disambiguation, artists.disambiguation),
        image_url = COALESCE(EXCLUDED.image_url, artists.image_url),
        updated_at = now()
RETURNING *;
```
Go signature: `func (q *Queries) UpsertArtist(ctx context.Context, arg UpsertArtistParams) (Artist, error)` where `UpsertArtistParams` is `{ Mbid string; DeezerID *string; Name string; Disambiguation *string; ImageUrl *string }` `[VERIFIED: internal/db/sqlc/artists.sql.go:24-30]`. Both the add-time and backfill call sites can call this directly: pass the artist's own already-known `Mbid`/`Name` unchanged, `Disambiguation: nil` (preserves whatever is already stored), and only `DeezerID`/`ImageUrl` set to the match result.

### `nullableString` — reused as-is for the new guest-feature `ReleaseDate`
```go
// Source: internal/detection/detector.go:104-113 (VERIFIED, read this session)
// nullableString turns an empty string into a nil *string. MusicBrainz
// returns "" (never omits the field) for a group's undated
// first-release-date, and this project's *string column convention treats
// SQL NULL, not an empty string literal, as "no value."
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

### `detectGuestFeatures`'s current insert (the exact call D-01's lookup result plugs into)
```go
// Source: internal/detection/musicbrainz.go:203-211 (VERIFIED, read this session)
newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
	ArtistID:   entry.ArtistID,
	Source:     sourceMusicBrainz,
	EventType:  eventTypeGuestFeature,
	ExternalID: rec.MBID,
	Title:      rec.Title,
	ArtistName: displayArtistName(rec, entry.Name),
	NotifiedAt: notifiedAt,
})
```
D-01/D-02/D-03 add `ReleaseGroupMbid`, `ReleaseDate`, and `CoverArtUrl` fields to this same struct literal, computed from the new lookup — mirroring the `new_release` insert's shape exactly (`internal/detection/musicbrainz.go:104-117`, which already sets all three from `g.FirstReleaseDate`/`coverArtURLForReleaseGroup(mbid)`).

### `DeluxeChangeBody` — current state and D-04's target shape
```tsx
// Source: web/app/components/history/EventCard.tsx:159-169 (VERIFIED, read this session)
function DeluxeChangeBody({ event }: { event: EventItem }) {
  const current = event.track_count ?? "?"
  if (event.previous_track_count == null) {
    return <p className="text-label text-muted-foreground">{current} tracks</p>
  }
  return (
    <p className="text-label text-muted-foreground">
      {event.previous_track_count} → {current} tracks
    </p>
  )
}
```
`NewReleaseBody`'s existing fallback/separator pattern to mirror (D-05):
```tsx
// Source: web/app/components/history/EventCard.tsx:107-115 (VERIFIED, read this session)
function NewReleaseBody({ event }: { event: EventItem }) {
  const dateLabel = event.release_date ?? "Release date unknown"
  return (
    <p className="text-label text-muted-foreground">
      {event.release_type ? `${event.release_type} · ` : ""}
      {dateLabel}
    </p>
  )
}
```
D-04's literal target format is `"{date} · {prev} → {current} tracks"` — i.e. prepend `${dateLabel} · ` to both of `DeluxeChangeBody`'s existing return branches, using the identical `event.release_date ?? "Release date unknown"` expression.

### `EventItem` — already exposes `release_date` for every event type (resolves CONTEXT.md's flagged uncertainty)
```typescript
// Source: web/app/lib/api.ts:12-28 (VERIFIED, read this session)
export interface EventItem {
  id: number
  artist_id: number
  source: string
  event_type: "new_release" | "guest_feature" | "deluxe_change"
  external_id: string
  release_group_mbid: string | null
  title: string
  artist_name: string
  release_date: string | null
  cover_art_url: string | null
  track_count: number | null
  previous_track_count: number | null
  release_type: string | null
  notified_at: string | null
  created_at: string
}
```
No `api.ts` edit needed — `release_date` is untyped per-event-type (it's one flat interface for all three), so it is already available to `DeluxeChangeBody` and `GuestFeatureBody` alike.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `guest_feature` events store no release date/cover art | D-01–D-03 add a per-insert MusicBrainz recording lookup | This phase | Guest-feature cards gain both a date and cover art, matching `new_release`/`deluxe_change` |
| Every artist's `image_url` is permanently NULL (MusicBrainz has no artist images, and no cross-source ID resolution exists) | D-06–D-09 add a Deezer-name-match-and-backfill pass | This phase (absorbing backlog Phase 999.2) | Existing and new watchlist artists get real photos from Deezer where a confident name match exists; no wrong-artist photos ever attached (fail-closed) |

**Deprecated/outdated:** None — this phase does not remove or replace any existing mechanism; it is purely additive.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `GET /ws/2/recording/{mbid}?inc=releases+release-groups` response nests a `"release-group"` object (with at least an `"id"`) inside each entry of the top-level `"releases"` array, and each release entry carries a `"date"` field in the same opaque partial-date string format as the browse endpoints. | Architecture Patterns > Pattern 1; Code Examples | If the actual field name/nesting differs, Go's silent zero-value decode means `ReleasesForRecording` would compile and run but never populate `ReleaseDate`/`CoverArtUrl` — guest-feature cards would look exactly like today's bug, with no error surfaced. Must be checked against a live response before/during implementation (see Pitfall 2). |
| A2 | MusicBrainz's documented "linked entities capped at 25" limit applies to this specific `inc=releases+release-groups` recording lookup the same way it applies generally to lookup `inc` parameters, and a single recording exceeding 25 linked releases is rare enough that no pagination loop is needed for v1. | Architecture Patterns > Pattern 1 | If a prolific guest-feature recording (e.g., a heavily-reissued classic track) has >25 linked releases, the earliest release could be silently excluded from the truncated first 25, producing a later-than-actual date. Low blast radius (a wrong-but-plausible date, not a crash). |
| A3 | The recommended integration point for D-06's add-time artist match is `watchlist.Service.Add` (not the HTTP handler layer), and the recommended startup-pass location is inside `cmd/server/main.go`'s `run()`, after pool/dzClient construction and before `pollr.Start(ctx)`. | Architecture Patterns; Common Pitfalls #3 | CONTEXT.md's Integration Points note names both call sites' *need* to share logic but does not lock the exact call-site layer or blocking-vs-async startup timing — this is a planner-discretion architectural recommendation, not a locked decision. A different reasonable placement (e.g., HTTP handler layer, or an async goroutine post-listen) would not contradict any CONTEXT.md decision. |

**If this table is empty:** N/A — see rows above.

## Open Questions

1. **`GuestFeatureBody` has no rendering decision assigned to it, but the phase's own goal statement requires one.**
   - What we know: CONTEXT.md's Phase Boundary section states History cards for "single/feature/deluxe" all currently lack a shown release date. D-04/D-05 assign the *deluxe_change* frontend fix explicitly. D-01–D-03 make guest_feature's `release_date` exist in the DB/API for the first time. `GuestFeatureBody` (`EventCard.tsx:136-154`) today renders only the linked title, no date at all.
   - What's unclear: No D-number explicitly assigns a `GuestFeatureBody` rendering change, nor specifies its exact layout (e.g., whether the date goes on its own line, or is combined with the existing "Featured on {title}" line, or follows `NewReleaseBody`'s "· "-separator convention).
   - Recommendation: Treat this as in-scope (the phase cannot be "actually resolved" per its own no-repeat-phases mandate without it) and mirror D-05's exact fallback text/expression (`event.release_date ?? "Release date unknown"`). Confirm the exact placement with the user during planning/discuss if any ambiguity remains, since CONTEXT.md's discretion table is empty (no gray areas were left open) — this is a genuine gap in the locked decisions, not a discretion area.

2. **Error-handling policy for a per-recording `ReleasesForRecording` lookup failure inside `detectGuestFeatures`'s loop.**
   - What we know: D-03 covers the case where the lookup *succeeds* but finds no releases (fallback: placeholder art, null date). The outer recording-browse call (`RecordingsByArtist`) already has an established policy — log and return `nil` for the whole pass (`internal/detection/musicbrainz.go:177-185`).
   - What's unclear: If the *new* per-recording lookup itself errors (network/5xx) for one specific recording mid-loop, should that recording still be inserted as a guest_feature event with `ReleaseDate`/`CoverArtUrl` left nil (fail open, matching D-03's spirit), or should it be skipped entirely this cycle (retried next cycle, consistent with `detectDeluxeChanges`'s per-group error-skip pattern at `internal/detection/musicbrainz.go:330-338`)?
   - Recommendation: Skipping-and-retrying-next-cycle (mirroring `detectDeluxeChanges`'s existing per-item error-isolation pattern) is more consistent with this codebase's established idiom than silently inserting a degraded row, but this is a planner-level implementation choice CONTEXT.md did not lock — flag for confirmation during planning if it isn't obvious from the plan's own error-handling conventions.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| musicbrainz.org reachability | Live-verifying the new `ReleasesForRecording` response shape (Pitfall 2) | ✗ (from this WSL2 dev machine, per `.planning/STATE.md`'s documented, waived blocker) | — | Verify from a different network path (CI, a teammate's machine, or a non-WSL2 host) before trusting the `[ASSUMED]` shape in production; Deezer is unaffected (`api.deezer.com` reachability is fine per existing Phase 3/12 work) |

**Missing dependencies with no fallback:** None — the MusicBrainz reachability gap is a verification convenience issue, not a hard implementation blocker (the client method can be written and unit-tested against `httptest.Server` fixtures regardless, exactly as `04-RESEARCH.md`'s own `Release`/`Recording` types were).

**Missing dependencies with fallback:** musicbrainz.org direct verification (see above).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | Go stdlib `testing` + `httptest.Server` (client packages), real-Postgres integration tests via `testutil.NewTestPool` (detection package) |
| Frontend framework | Vitest 4.1.10 + `@testing-library/react` `[VERIFIED: web/package.json, "test": "vitest run"]` |
| Config file | `web/vitest.config.ts` (assumed present, not read this session — not load-bearing for this phase's test additions, which follow existing `*.test.tsx` conventions) |
| Quick run command (Go) | `go test ./internal/musicbrainz/... ./internal/detection/...` |
| Quick run command (frontend) | `cd web && npm test -- EventCard` |
| Full suite command | `make test` (Go), `cd web && npm test` (frontend) — mirrors this repo's existing CI gate structure |

### Phase Requirements → Test Map
| Behavior | Test Type | Automated Command | File Exists? |
|----------|-----------|-------------------|-------------|
| `ReleasesForRecording` decodes the new lookup envelope, builds the correct request URL/path, escapes the mbid, handles empty-mbid/non-OK-status | unit (httptest) | `go test ./internal/musicbrainz/... -run TestReleasesForRecording` | ❌ Wave 0 — new file, mirrors `internal/musicbrainz/releases_test.go`'s existing structure |
| `detectGuestFeatures` inserts `ReleaseDate`/`CoverArtUrl` for a genuinely-new recording, using the earliest release's date, and falls back to nil/placeholder when no releases are found (D-01/D-02/D-03) | unit (whitebox, real-Postgres) | `go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature` | ❌ Wave 0 — extends `internal/detection/detector_test.go`; needs `fakeRecordingSource` widened with a `ReleasesForRecording` stub method |
| `DeluxeChangeBody` renders the date with the D-04 separator and D-05 fallback | unit (RTL) | `cd web && npm test -- EventCard` | ❌ Wave 0 — extends existing `web/app/components/history/EventCard.test.tsx`, whose `buildEvent()` fixture already carries `release_date` |
| `GuestFeatureBody` renders the date (see Open Question 1) | unit (RTL) | `cd web && npm test -- EventCard` | ❌ Wave 0 — same file, contingent on Open Question 1's resolution |
| Artist-art matcher: close-name-match wins, ambiguous same-name results tie-broken on shared album title, no-confident-match fails closed to nil (D-08/D-09) | unit (stub Deezer searcher, no real HTTP) | `go test ./internal/artistart/... ` (or wherever the planner places the new package) | ❌ Wave 0 — brand-new package/file |
| Backfill sweep updates only `image_url IS NULL` artists, is non-destructive to `deezer_id`/`disambiguation`, and runs exactly once at startup (D-06/D-07) | integration (real-Postgres) | `go test ./cmd/server/... -run TestBackfill` or an `internal/artistart` integration test using `sqlc.New(pool)` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** the quick-run command scoped to the package just touched
- **Per wave merge:** `make test` (Go) and `cd web && npm test` (frontend)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/musicbrainz/recording_lookup_test.go` (or appended to `recordings_test.go`) — new file, no existing coverage for a lookup-by-path-segment MusicBrainz call
- [ ] `internal/detection/detector_test.go`'s `fakeRecordingSource` — needs a `ReleasesForRecording` stub method added (single-place edit, per Pattern 2 above)
- [ ] New package for D-08/D-09 match logic — no existing tests, no existing file
- [ ] Backfill integration test — no existing coverage for a startup-time DB sweep
- Framework install: none — Go `testing`/`httptest` and Vitest are both already fully set up in this repo

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Phase touches no auth surface |
| V3 Session Management | no | Phase touches no session surface |
| V4 Access Control | no | Phase adds no new access-controlled resource |
| V5 Input Validation | yes | The new `ReleasesForRecording` method must not build its request URL by naive string concatenation of a caller-influenced mbid — use `url.PathEscape` (mirrors this codebase's existing `url.Values.Set`-based query-encoding convention used everywhere else in `internal/musicbrainz`/`internal/deezer`) |
| V6 Cryptography | no | No new cryptographic operation in this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Response-body echo in error messages (this codebase's own established, repeatedly-cited convention: "Never echo the response body -- only the status code" `[VERIFIED: internal/musicbrainz/releases.go:169-171, internal/musicbrainz/releasegroups.go:144-146, internal/musicbrainz/recordings.go:161-163 — same comment text at each of the three existing browse methods]`) | Information Disclosure | The new `ReleasesForRecording`'s non-OK-status branch must follow the identical pattern: `fmt.Errorf("musicbrainz: releases for recording: unexpected status %d", resp.StatusCode)` — never wrap or log `resp.Body`'s contents |
| Unbounded/hostile response driving unbounded work | Denial of Service | Not applicable to a single-entity lookup the same way it is to a browse endpoint (no count/offset field to be lied about) — MusicBrainz's own "capped at 25 linked entities" ceiling (Pitfall 1 / Assumption A2) already bounds this call's response size without any client-side pagination loop needed |
| Third-party artist name (from MusicBrainz, stored in `artists.name`) passed into a Deezer search query | Tampering (of match outcome, not injection) | `deezer.Client.SearchArtists` already encodes the query via `url.Values.Set` (`[VERIFIED: internal/deezer/search.go:56-59]`) — no raw string interpolation risk. D-09's fail-closed behavior is itself the relevant mitigation for a spoofed/malicious name producing a wrong-artist photo: no confident match, no photo attached |

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/detection/musicbrainz.go` — `DetectMusicBrainz`, `detectGuestFeatures`, `detectDeluxeChanges`, `coverArtURLForReleaseGroup`, `isGuestFeature`, `displayArtistName`
- `internal/detection/detector.go` — `Detector`, `New`, `RecordingSource`, `ReleaseDetailSource`, `nullableString`, `insertEvent`
- `internal/musicbrainz/recordings.go`, `releases.go`, `releasegroups.go`, `search.go`, `client.go` — existing client method conventions, rate-limiting/error-handling idiom
- `internal/musicbrainz/releases_test.go`, `recordings_test.go` — existing test conventions (`newTestClient`, `unlimitedLimiter`, fixture style)
- `internal/detection/detector_test.go` — `fakeRecordingSource`/`fakeReleaseDetailSource` call-site count (grep, 30+ matches)
- `internal/deezer/client.go`, `search.go` — `SearchArtists` signature, `Artist` struct, existing D-04 fan-count sort
- `internal/watchlist/service.go` — `AddParams`, `Entry`, `Store`, `Service.Add`, `UpsertArtist` call site
- `internal/db/migrations/000002_watchlist.up.sql`, `000003_events.up.sql`, `000004_events_display_fields.up.sql` — confirms `artists.deezer_id`/`image_url` and `events.release_date`/`cover_art_url` already exist, no migration needed
- `internal/db/sqlc/artists.sql.go`, `querier.go`, `queries/artists.sql`, `queries/watchlist.sql` — `UpsertArtist`'s exact COALESCE semantics, sqlc query-file convention
- `internal/httpserver/watchlist.go`, `search.go` — `handleAddWatchlist`'s current pass-through of `ImageURL`, `SearchArtist` wire shape
- `web/app/components/history/EventCard.tsx`, `EventCard.test.tsx` — `DeluxeChangeBody`/`GuestFeatureBody`/`NewReleaseBody` current implementations, test fixture convention
- `web/app/lib/api.ts` — `EventItem` already includes `release_date`
- `web/app/routes/watchlist.tsx`, `web/app/components/watchlist/SearchResultsColumns.tsx` — confirms only MusicBrainz-sourced adds reach `addWatchlist`, `image_url`/`deezer_id` never derived client-side
- `web/app/components/common/CoverArt.tsx`, `WatchlistRow.tsx` — confirms the rendering side (`CoverArt`) already handles a populated `image_url` gracefully; no frontend change needed for bug #3 beyond the backend match/backfill
- `cmd/server/main.go` — full boot sequence (`run()`), construction order of `pool`/`store`/`detector`/`mbClient`/`dzClient`/`pollr`
- `.planning/STATE.md`, `.planning/phases/13-.../13-CONTEXT.md` — locked decisions, prior-phase precedent, documented MusicBrainz WSL2 reachability blocker
- `.planning/config.json` — confirms `nyquist_validation: true` and `security_enforcement: true` (both sections included above)

### Secondary (MEDIUM confidence — fetched this session, partial corroboration)
- `musicbrainz.org/doc/MusicBrainz_API` — confirms recording lookups support `inc=releases,release-groups`-equivalent subqueries and that "the number of linked entities returned is always limited to 25"
- `musicbrainz.org/doc/MusicBrainz_API/Examples` — confirms a recording lookup's `releases` array contains simplified release objects with `id`/`title` at minimum, but the fetched example did not include `release-groups` in the same query, so the exact combined nesting (Assumption A1) remains unconfirmed by a fetched example

### Tertiary (LOW confidence)
- None used without a corroborating primary/secondary source

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies, all reused code read directly this session
- Architecture (existing-seam reuse: `UpsertArtist`, `coverArtURLForReleaseGroup`, `nullableString`, `RecordingSource`): HIGH — verified by reading the actual source
- Architecture (new MusicBrainz recording-lookup shape): MEDIUM/LOW — corroborated by official docs pages but not a live fetched example combining both `inc` values; tagged `[ASSUMED]`
- Pitfalls: HIGH for the reordering/interface-widening pitfalls (directly observed in code); MEDIUM for the MusicBrainz shape pitfall (inherent to the ASSUMED tag above)

**Research date:** 2026-08-24
**Valid until:** 30 days (stable internal codebase conventions); the MusicBrainz recording-lookup shape assumption should be re-verified at implementation time regardless of elapsed time, per Pitfall 2
