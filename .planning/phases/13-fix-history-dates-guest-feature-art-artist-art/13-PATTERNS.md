# Phase 13: Fix History Dates, Guest-Feature Art & Artist Art - Pattern Map

**Mapped:** 2026-08-24
**Files analyzed:** 8 (create/modify)
**Analogs found:** 8 / 8

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/musicbrainz/recording_lookup.go` (new) | service/client method | request-response (single-entity lookup) | `internal/musicbrainz/releases.go` (`ReleasesByReleaseGroup`/`fetchReleasePage`) | role-match (browse vs lookup shape differs, error/rate-limit convention identical) |
| `internal/musicbrainz/recording_lookup_test.go` (new) | test | request-response | `internal/musicbrainz/releases_test.go` | role-match |
| `internal/detection/detector.go` (modify) | interface/wiring | CRUD (seen-store) | itself — widen `RecordingSource` in place | exact |
| `internal/detection/musicbrainz.go` (modify — `detectGuestFeatures`) | service/detection | CRUD (diff-and-insert) | itself — mirror `new_release` insert block (lines 102-117) | exact |
| `internal/detection/detector_test.go` (modify — `fakeRecordingSource`) | test double | request-response | itself — add one stub method | exact |
| `web/app/components/history/EventCard.tsx` (modify — `DeluxeChangeBody`, `GuestFeatureBody`) | component | transform (render) | itself — `NewReleaseBody` (lines 107-115) | exact |
| `internal/artistart/match.go` (new package) | service | request-response (external search + match) | `internal/deezer/search.go` (`SearchArtists`) for the Deezer-call half; no direct analog for the match/tie-break logic itself | role-match |
| `internal/artistart/match_test.go` (new) | test | request-response | `internal/deezer` test conventions (stub `doRequest`/`httptest.Server`) | role-match |
| `queries/artists.sql` + regenerated `internal/db/sqlc/artists.sql.go` (new query, e.g. `ListArtistsMissingImage`) | model/query | CRUD (read) | `queries/artists.sql`'s existing `UpsertArtist` (write pattern); no existing "list missing X" query to copy structurally — follow `ListWatchlist`-style plain SELECT in `queries/watchlist.sql` | partial match |
| `internal/watchlist/service.go` (modify — `Add`) | service | CRUD | itself — the existing `UpsertArtist` call at lines 138-144 | exact |
| `cmd/server/main.go` (modify — construction order + startup backfill pass) | wiring/main | batch (one-time startup sweep) | itself — existing four-`sqlc.New(pool)`-instances idiom (`store`, `detector`, `dzClient`-adjacent lines 104,121,135,145) | role-match |

## Pattern Assignments

### `internal/musicbrainz/recording_lookup.go` (new client method, request-response)

**Analog:** `internal/musicbrainz/releases.go`

**Imports pattern** (releases.go lines 1-11):
```go
package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)
```
(`ReleasesForRecording` needs no `strconv` since there's no pagination limit/offset param.)

**Core lookup-by-path-segment pattern** — this is a NEW shape (not the existing browse-by-query-param shape). Full reference implementation already drafted in RESEARCH.md and verified against `releases.go`'s conventions:
```go
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
Uses `c.doRequest` (shared rate-limiter + User-Agent seam) exactly like `fetchReleasePage` (releases.go:162) — never bypass it.

**Error handling pattern** (releases.go lines 168-171) — copy verbatim, never echo response body:
```go
if resp.StatusCode != http.StatusOK {
	// Never echo the response body -- only the status code, which is
	// operator-facing and carries no upstream text (T-04-23, V13).
	return releaseEnvelope{}, fmt.Errorf("musicbrainz: releases by release group: unexpected status %d", resp.StatusCode)
}
```

**Type/doc-comment convention to mirror** (releases.go lines 33-62) — `[ASSUMED]` tagging on unverified response shapes, and opaque partial-date strings never parsed into `time.Time`:
```go
// Release is a single ws/2/release browse result, with per-medium
// track-count detail via inc=media. Date is kept as the opaque partial-date
// string MusicBrainz returns...
type Release struct {
	MBID   string   `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Date   string   `json:"date"`
	Media  []Medium `json:"media"`
}
```
Apply the same `[ASSUMED]` doc-comment style to `RecordingRelease`/`recordingLookupResponse` per RESEARCH.md's Assumption A1 — flag the `releases[].release-group.id` nesting as unverified.

**No pagination needed** — MusicBrainz caps linked entities at 25; document as an accepted truncation edge (mirrors the `pageCeilingReached` logging idiom already used in `internal/detection/musicbrainz.go` lines 226, 339).

---

### `internal/detection/detector.go` (widen `RecordingSource`)

**Analog:** itself, lines 27-29

**Current:**
```go
type RecordingSource interface {
	RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error)
}
```
**Add one method** (per RESEARCH.md Pattern 2 — do NOT touch `New`'s signature or `cmd/server/main.go`'s wiring line 135, `mbClient` already satisfies a widened interface):
```go
type RecordingSource interface {
	RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error)
	ReleasesForRecording(ctx context.Context, mbid string) ([]musicbrainz.RecordingRelease, error)
}
```

`nullableString` (detector.go lines 104-113) is reused as-is for the new guest-feature `ReleaseDate` field:
```go
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

---

### `internal/detection/musicbrainz.go` (`detectGuestFeatures` — wire in D-01/D-02/D-03)

**Analog:** the `new_release` insert block in the SAME file, lines 102-117 (exact pattern to mirror):
```go
mbid := g.MBID
coverArt := coverArtURLForReleaseGroup(mbid)
newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
	ArtistID:         entry.ArtistID,
	Source:           sourceMusicBrainz,
	EventType:        eventTypeNewRelease,
	ExternalID:       mbid,
	ReleaseGroupMbid: &mbid,
	Title:            g.Title,
	ArtistName:       entry.Name,
	ReleaseDate:      nullableString(g.FirstReleaseDate),
	CoverArtUrl:      &coverArt,
	TrackCount:       nil,
	ReleaseType:      releaseTypeForStorage(g.PrimaryType),
	NotifiedAt:       notifiedAt,
})
```

**Current guest-feature insert to extend** (lines 203-211, this is the exact call site):
```go
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
Add `ReleaseGroupMbid`, `ReleaseDate: nullableString(earliestDate)`, `CoverArtUrl: &coverArt` (only set when a release-group MBID was found — D-03 leaves both nil/placeholder otherwise), computed just above this call via `d.recordings.ReleasesForRecording(ctx, rec.MBID)` + earliest-date selection (D-02, string-prefix-safe lexicographic comparison per RESEARCH.md Pitfall 1 — filter empty-string dates before comparing) + `coverArtURLForReleaseGroup(releaseGroupMBID)` (line 507, reused as-is).

**Per-item error isolation to mirror** — `detectDeluxeChanges`'s per-group error-skip pattern (lines 330-338), recommended for the new per-recording lookup failure (Open Question 2's recommended resolution):
```go
releases, err := d.releases.ReleasesByReleaseGroup(ctx, g.MBID)
if err != nil {
	logger.Error("release detail fetch failed",
		slog.String("artist_mbid", entry.MBID),
		slog.String("release_group_mbid", g.MBID),
		slog.String("musicbrainz_error", err.Error()),
	)
	continue
}
```

---

### `internal/detection/detector_test.go` (`fakeRecordingSource` — add one stub method)

**Analog:** itself, lines 37-47 (single-place edit per Pattern 2; 30+ call sites construct `fakeRecordingSource{}` unchanged, only the type gains a method):
```go
// fakeRecordingSource is a controllable double for detection.RecordingSource,
// ...
type fakeRecordingSource struct {
	recordings []musicbrainz.Recording
	err        error
}

func (f fakeRecordingSource) RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error) {
	...
}
```
Add a `releasesForRecording map[string][]musicbrainz.RecordingRelease` (or similar) field + a `ReleasesForRecording` method returning a per-mbid canned result, defaulting to empty/no-op when omitted (matches the existing comment convention at line 1115: "omitted ... no-op ... contributes no ...").

---

### `web/app/components/history/EventCard.tsx` (`DeluxeChangeBody`, `GuestFeatureBody`)

**Analog:** `NewReleaseBody` in the same file, lines 107-115:
```tsx
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

**Current `DeluxeChangeBody`** (lines 159-169) to extend per D-04/D-05:
```tsx
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
Target shape (prepend `${dateLabel} · ` using the identical fallback expression to both branches):
```tsx
function DeluxeChangeBody({ event }: { event: EventItem }) {
  const dateLabel = event.release_date ?? "Release date unknown"
  const current = event.track_count ?? "?"
  if (event.previous_track_count == null) {
    return (
      <p className="text-label text-muted-foreground">
        {dateLabel} · {current} tracks
      </p>
    )
  }
  return (
    <p className="text-label text-muted-foreground">
      {dateLabel} · {event.previous_track_count} → {current} tracks
    </p>
  )
}
```

**Current `GuestFeatureBody`** (lines 136-154) — no date rendering assigned by CONTEXT.md but flagged as a required gap (Open Question 1); mirror the identical `?? "Release date unknown"` fallback expression, placed on its own line below the existing "Featured on ..." line to avoid disrupting the existing link markup:
```tsx
function GuestFeatureBody({ event }: { event: EventItem }) {
  const href = guestFeatureHref(event)
  const dateLabel = event.release_date ?? "Release date unknown"
  const titleLine = !href ? (
    <p className="text-label text-muted-foreground">{event.title}</p>
  ) : (
    <p className="text-label text-muted-foreground">
      Featured on{" "}
      <a href={href} target="_blank" rel="noreferrer" className="underline underline-offset-2 hover:text-foreground">
        {event.title}
      </a>
    </p>
  )
  return (
    <>
      {titleLine}
      <p className="text-label text-muted-foreground">{dateLabel}</p>
    </>
  )
}
```
(Confirm exact placement/layout choice with the plan — this is the one file with no locked D-number; keep it minimal and consistent with the other two bodies' fallback text.)

`EventItem` (`web/app/lib/api.ts` lines 12-28) already exposes `release_date: string | null` for all three event types — **no `api.ts` change needed**.

---

### `internal/artistart/match.go` (new package, D-08/D-09 match logic)

**Analog:** `internal/deezer/search.go`'s `SearchArtists` (the external-call half); no existing analog for the confidence-gate/tie-break logic itself — this is genuinely new domain logic, built on top of the existing client.

**Reused as-is — the search call:**
```go
// Source: internal/deezer/search.go:46 (VERIFIED)
func (c *Client) SearchArtists(ctx context.Context, query string, limit int) ([]Artist, error)
```
Returns `[]Artist{ID int64, Name string, Link string, Picture string, NbAlbum int, NbFan int, Type string}`, already sorted by `NbFan` descending (search.go lines 93-105) — useful as the natural popularity tiebreaker if D-08's own tie-break (shared album title) still leaves ties.

**Match function skeleton** (new — follow the project's existing "narrow interface declared by the consumer" convention, e.g. `detection.RecordingSource`/`ReleaseDetailSource` at `internal/detection/detector.go` lines 21-39, for testability without a real HTTP client):
```go
// ArtistSearcher is the narrow seam artistart depends on -- declared here,
// in the consumer, mirroring detection.RecordingSource/ReleaseDetailSource,
// so a test can substitute a stub instead of a real deezer.Client.
type ArtistSearcher interface {
	SearchArtists(ctx context.Context, query string, limit int) ([]deezer.Artist, error)
}

// Match implements D-08 (close-name-match primary signal, shared-album-title
// tie-break only) and D-09 (fail closed to zero value on no confident
// match) -- the single function both the add-time and backfill call sites
// invoke, so match logic never duplicates between them.
func Match(ctx context.Context, searcher ArtistSearcher, name string, knownAlbumTitles []string) (deezerID *string, imageURL *string, matched bool, err error) {
	...
}
```

**Error handling / never-echo-body convention** to carry through (search.go lines 72-76):
```go
if resp.StatusCode != http.StatusOK {
	// Never echo the response body -- only the status code (mirrors
	// T-03-01/V13 in internal/musicbrainz).
	return nil, fmt.Errorf("deezer: search artists: unexpected status %d", resp.StatusCode)
}
```

---

### `queries/artists.sql` + `internal/db/sqlc/artists.sql.go` (new `ListArtistsMissingImage`-style query)

**Analog:** `UpsertArtist` (write pattern, `queries/artists.sql` / `internal/db/sqlc/artists.sql.go` lines 12-62) for the non-destructive write shape both D-06 call sites reuse verbatim — **do not build a second narrower UPDATE query**, call `UpsertArtist` directly from both the add-time and backfill call sites:
```sql
-- name: UpsertArtist :one
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
Go signature: `func (q *Queries) UpsertArtist(ctx context.Context, arg UpsertArtistParams) (Artist, error)` where `UpsertArtistParams{ Mbid string; DeezerID *string; Name string; Disambiguation *string; ImageUrl *string }` — both call sites pass the artist's already-known `Mbid`/`Name` unchanged, `Disambiguation: nil` (preserve), and only `DeezerID`/`ImageUrl` set from the match result.

The NEW query needed is only a read — `ListArtistsMissingImage` (or similar name), a plain `SELECT * FROM artists WHERE image_url IS NULL` — no existing "list missing X" query exists in this codebase to copy structurally; follow the plain-SELECT, sqlc-`:many` convention used by `ListWatchlist` in `queries/watchlist.sql` (not read this session in full, but this is the established sqlc query-file idiom project-wide per RESEARCH.md's Sources list).

---

### `internal/watchlist/service.go` (`Add` — D-06 add-time wiring)

**Analog:** itself, the existing `UpsertArtist` call, lines 138-144:
```go
artist, err := s.q.UpsertArtist(ctx, sqlc.UpsertArtistParams{
	Mbid:           p.MBID,
	DeezerID:       p.DeezerID,
	Name:           p.Name,
	Disambiguation: p.Disambiguation,
	ImageUrl:       p.ImageURL,
})
```
Per RESEARCH.md Assumption A3, wire the `artistart.Match` call inside `Service.Add`, before this `UpsertArtist` call, only when `p.ImageURL == nil` (D-06: matching only applies when the caller didn't already supply art) — populate `p.DeezerID`/`p.ImageURL` from the match result before this existing call, so the call itself is unchanged.

`Service` currently holds only `q sqlc.Querier` (lines 95-102) — adding an `ArtistSearcher`/`artistart` dependency requires widening `Service`'s struct and `NewService`'s constructor, which per Pitfall 3 requires reordering `cmd/server/main.go`'s `dzClient` construction (currently at line 145) above `store := watchlist.NewService(...)` (currently at line 104).

---

### `cmd/server/main.go` (construction-order fix + D-07 startup backfill pass)

**Analog:** the existing multi-`sqlc.New(pool)`-instance idiom already used four times in this file (lines 104, 121→135, 145):
```go
store := watchlist.NewService(sqlc.New(pool))          // line 104
mbClient := musicbrainz.NewClient(...)                  // line 121
detector := detection.New(sqlc.New(pool), mbClient, mbClient)  // line 135
dzClient := deezer.NewClient(dzLimiter, nil)            // line 145
```
Per Pitfall 3: move `dzClient`'s construction (and its rate limiter) above `store`'s, or pass the matcher into `NewService` as an added constructor parameter once `dzClient` exists. The one-time D-07 backfill sweep is a new small block after `dzClient`/`store`/`detector` are all constructed and before `pollr.Start(ctx)` (line 170) — follow the same "own `sqlc.New(pool)` instance, stateless wrapper" idiom rather than threading `store`'s querier through if that's inconvenient.

## Shared Patterns

### Never echo response body on non-2xx status
**Source:** `internal/musicbrainz/releases.go:168-171`, `internal/deezer/search.go:72-76` (identical convention, both clients)
**Apply to:** `ReleasesForRecording` (new), any error path inside `internal/artistart`'s Deezer calls
```go
if resp.StatusCode != http.StatusOK {
	return nil, fmt.Errorf("<pkg>: <method>: unexpected status %d", resp.StatusCode)
}
```

### Opaque partial-date strings — never parse into time.Time, compare lexicographically after filtering empty
**Source:** `internal/musicbrainz/releases.go:50-55`, RESEARCH.md Pitfall 1
**Apply to:** the new guest-feature earliest-release-date selection (D-02) inside `detectGuestFeatures`

### Narrow consumer-declared interface for testability (no concrete client type in a dependent package's signature)
**Source:** `internal/detection/detector.go:21-39` (`RecordingSource`, `ReleaseDetailSource`)
**Apply to:** `internal/artistart`'s `ArtistSearcher` interface, and the widened `RecordingSource` in `detector.go`

### Non-destructive COALESCE-based upsert — never write a second narrower update query
**Source:** `internal/db/sqlc/artists.sql.go:12-62` (`UpsertArtist`)
**Apply to:** both `watchlist.Service.Add`'s add-time write and the startup backfill's per-artist write — call `UpsertArtist` directly, do not add a new `UPDATE artists SET image_url = ...` query

### Fallback display text `?? "Release date unknown"`
**Source:** `web/app/components/history/EventCard.tsx:108` (`NewReleaseBody`)
**Apply to:** `DeluxeChangeBody` (D-05) and `GuestFeatureBody` (Open Question 1) — identical expression, no new copy

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/artistart` match/tie-break decision logic (the D-08/D-09 core, not the Deezer call itself) | service | event-driven/transform | No existing fuzzy-name-match-with-tie-break logic anywhere in this codebase; built fresh on top of `deezer.SearchArtists`, following the project's narrow-interface convention only |
| `ListArtistsMissingImage`-style query | model/query | batch (one-time read) | No existing "list rows where nullable column IS NULL" query to copy structurally; plain sqlc `:many` SELECT, trivial enough not to need a closer analog |

## Metadata

**Analog search scope:** `internal/musicbrainz`, `internal/detection`, `internal/deezer`, `internal/watchlist`, `internal/db/sqlc`, `queries/`, `web/app/components/history`, `web/app/lib`, `cmd/server`
**Files scanned:** `releases.go`, `recordings.go` (referenced), `musicbrainz.go`, `detector.go`, `detector_test.go`, `search.go`, `service.go`, `artists.sql.go`, `EventCard.tsx`, `main.go`
**Pattern extraction date:** 2026-08-24
