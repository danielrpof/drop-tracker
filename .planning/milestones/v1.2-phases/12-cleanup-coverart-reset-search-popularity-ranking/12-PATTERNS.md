# Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking - Pattern Map

**Mapped:** 2026-08-18
**Files analyzed:** 10 (all edits to existing files, 2 net-new test files)
**Analogs found:** 10 / 10 (self-contained edits; analog is each file's own pre-existing code/sibling test file)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `web/app/components/common/CoverArt.tsx` | component | request-response (render + image load event) | itself (in-place edit) | exact |
| `web/app/components/common/CoverArt.test.tsx` (NEW) | test | event-driven | `web/app/components/watchlist/SearchResultsColumns.test.tsx` | exact (RTL component test w/ `waitFor`) |
| `internal/deezer/search.go` | service (external API client) | CRUD (decode + transform) | itself (in-place edit) | exact |
| `internal/deezer/search_test.go` | test | request-response (httptest) | itself (in-place edit, extends existing tests) | exact |
| `internal/musicbrainz/search.go` | service (external API client) | CRUD (decode) | itself (in-place edit); secondary analog `internal/deezer/search.go`'s `Artist` struct convention | exact |
| `internal/musicbrainz/search_test.go` | test | request-response (httptest) | itself (in-place edit); secondary analog `internal/deezer/search_test.go` | exact |
| `internal/httpserver/search.go` | controller/adapter | request-response | itself (in-place edit — `musicBrainzSource.SearchArtists`/`deezerSource.SearchArtists` adapters) | exact |
| `internal/httpserver/search_test.go` | test | request-response | itself (in-place edit); pattern `TestNewDeezerSource_MapsFields`-style adapter test | exact |
| `web/app/lib/api.ts` | type/model (wire-type mirror) | transform | itself (in-place edit — `SearchArtist` interface) | exact |
| `web/app/components/watchlist/SearchResultsColumns.tsx` | component | request-response (render) | itself (in-place edit — `SearchResultRow`'s disambiguation `<span>`) | exact |
| `web/app/components/watchlist/SearchResultsColumns.test.tsx` | test | event-driven | itself (in-place edit, existing file/conventions) | exact |

Every file in this phase is an in-place edit to an existing file (or a new test file sitting directly beside an existing, structurally identical sibling). There is no "different subsystem" analog search needed — the pattern to copy is each file's own current code, extended per CONTEXT.md's D-01–D-10.

## Pattern Assignments

### `web/app/components/common/CoverArt.tsx` (component, request-response)

**Analog:** itself, current implementation (read this session, full file below)

**Current full file** (`web/app/components/common/CoverArt.tsx:1-51`):
```tsx
import { Music } from "lucide-react"
import { useState } from "react"

import { cn } from "~/lib/utils"

export interface CoverArtProps {
  src?: string | null
  alt: string
  size?: number
  className?: string
}

export function CoverArt({ src, alt, size = 96, className }: CoverArtProps) {
  const [failed, setFailed] = useState(false)
  const showPlaceholder = !src || failed

  const style = { width: size, height: size }

  if (showPlaceholder) {
    return (
      <div
        role="img"
        aria-label={alt}
        className={cn(
          "flex shrink-0 items-center justify-center rounded-md bg-secondary text-muted-foreground",
          className
        )}
        style={style}
      >
        <Music className="h-1/3 w-1/3" aria-hidden="true" />
      </div>
    )
  }

  return (
    <img
      src={src}
      alt={alt}
      style={style}
      className={cn("shrink-0 rounded-md object-cover", className)}
      onError={() => setFailed(true)}
    />
  )
}
```

**D-01 fix — add one `useEffect` after the `useState` line, add `useEffect` to the import:**
```tsx
import { useEffect, useState } from "react"
// ...
export function CoverArt({ src, alt, size = 96, className }: CoverArtProps) {
  const [failed, setFailed] = useState(false)

  // D-01: reset the failed flag whenever src changes on this retained
  // instance (WatchlistRow/EventCard/SearchResultsColumns all reuse the
  // same CoverArt instance across re-renders rather than remounting via
  // key={src}). Deliberate deviation from React's own "prefer key remount"
  // guidance (react.dev/learn/you-might-not-need-an-effect) -- see
  // 12-RESEARCH.md Pitfall 1 for the accepted one-frame stale-placeholder
  // tradeoff. Do not "fix" this back to a key-based remount.
  useEffect(() => {
    setFailed(false)
  }, [src])

  const showPlaceholder = !src || failed
  // ...rest unchanged
```

No changes needed to the `showPlaceholder`/`<img onError>` logic itself, and zero call-site changes at `WatchlistRow.tsx`, `EventCard.tsx`, or `SearchResultsColumns.tsx`.

---

### `web/app/components/common/CoverArt.test.tsx` (NEW test file)

**Analog:** `web/app/components/watchlist/SearchResultsColumns.test.tsx` (existing RTL component test in the same directory tree, already imports `waitFor` for async state assertions)

**Imports pattern** (mirror `SearchResultsColumns.test.tsx:1`):
```tsx
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { CoverArt } from "./CoverArt"
```

**Core D-02 regression test pattern** (render → trigger `onError` via `fireEvent.error` → rerender with new `src` → `waitFor` placeholder clears):
```tsx
describe("CoverArt", () => {
  it("clears the failed placeholder when src changes (D-01/D-02)", async () => {
    const { rerender } = render(<CoverArt src="https://example.com/broken.png" alt="Test Artist" />)

    const img = screen.getByRole("img", { hidden: true }) // <img> before failure
    fireEvent.error(img)

    // After onError, the placeholder (role="img" div with aria-label) renders instead.
    await waitFor(() => {
      expect(screen.getByRole("img", { name: "Test Artist" })).toBeInTheDocument()
    })

    rerender(<CoverArt src="https://example.com/works.png" alt="Test Artist" />)

    // D-01's useEffect fires post-commit -- must assert with waitFor, not
    // synchronously, per 12-RESEARCH.md Pitfall 1.
    await waitFor(() => {
      const el = screen.getByAltText("Test Artist")
      expect(el.tagName).toBe("IMG")
    })
  })
})
```
Note: exact query selectors (role vs alt text) should be adjusted to whichever RTL query cleanly distinguishes the placeholder `<div role="img">` from the real `<img>` — confirm against the actual rendered DOM before finalizing, since both elements share `role="img"`/`aria-label`/`alt` naming.

---

### `internal/deezer/search.go` (service, CRUD/decode)

**Analog:** itself, current `Artist` struct + `SearchArtists` method (read this session)

**Current struct** (`internal/deezer/search.go:19-26`):
```go
type Artist struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Link    string `json:"link"`
	Picture string `json:"picture"`
	NbAlbum int    `json:"nb_album"`
	Type    string `json:"type"`
}
```

**D-03 fix — add `NbFan` alongside `NbAlbum`, same `int` convention:**
```go
type Artist struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Link    string `json:"link"`
	Picture string `json:"picture"`
	NbAlbum int    `json:"nb_album"`
	NbFan   int    `json:"nb_fan"`
	Type    string `json:"type"`
}
```

**D-04 fix — sort inside `Client.SearchArtists`, after `append(artists, env.Data...)`, before `return artists, nil`:**
```go
import (
	"cmp"
	// ...existing imports
	"slices"
)

// after decode/append, before return:
slices.SortFunc(artists, func(a, b Artist) int {
	return cmp.Compare(b.NbFan, a.NbFan) // descending
})
return artists, nil
```

**Doc comment update required** (per CONTEXT.md note): the existing "Results are returned in Deezer's own order — this method never sorts" comment on `SearchArtists` must be rewritten to state it now sorts by `NbFan` descending.

---

### `internal/deezer/search_test.go` (test, request-response/httptest)

**Analog:** itself — existing `TestSearchArtists_DecodesFixture` and `TestSearchArtists_PreservesUpstreamOrderNoSorting` (read this session, full relevant excerpts below)

**Fixture already contains the value needed** (`internal/deezer/search_test.go:24-39`, `drakeSearchFixture`):
```go
const drakeSearchFixture = `{
  "data": [
    {
      "id": 246791,
      "name": "Drake",
      "link": "https://www.deezer.com/artist/246791",
      "picture": "https://api.deezer.com/artist/246791/image",
      "nb_album": 78,
      "nb_fan": 24047501,
      "tracklist": "https://api.deezer.com/artist/246791/top?limit=50",
      "type": "artist"
    }
  ],
  ...
}`
```

**Extend `TestSearchArtists_DecodesFixture`** (add after the existing `NbAlbum` assertion, `search_test.go:114-116`):
```go
if got.NbFan != 24047501 {
	t.Errorf("NbFan = %d, want %d", got.NbFan, 24047501)
}
```

**Rewrite/rename `TestSearchArtists_PreservesUpstreamOrderNoSorting`** (currently `search_test.go:170-190`, uses `twoArtistsSearchFixture` which has no `nb_fan` set — must be extended with distinct out-of-order fan counts, not left vacuous):
```go
const twoArtistsSearchFixtureRanked = `{
  "data": [
    {"id": 1, "name": "Artist A", "link": "https://www.deezer.com/artist/1", "picture": "https://api.deezer.com/artist/1/image", "nb_album": 5, "nb_fan": 100, "type": "artist"},
    {"id": 2, "name": "Artist B", "link": "https://www.deezer.com/artist/2", "picture": "https://api.deezer.com/artist/2/image", "nb_album": 3, "nb_fan": 900, "type": "artist"}
  ],
  "total": 2
}`

func TestSearchArtists_SortsByFanCountDescending(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(twoArtistsSearchFixtureRanked))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "artist", 25)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("len(artists) = %d, want 2", len(artists))
	}
	// Upstream order is [A(100), B(900)] -- sorted output must be [B, A].
	if artists[0].Name != "Artist B" || artists[1].Name != "Artist A" {
		t.Fatalf("order = [%q, %q], want [Artist B, Artist A] (descending nb_fan)", artists[0].Name, artists[1].Name)
	}
}
```
Per 12-RESEARCH.md Pitfall 3: either delete `TestSearchArtists_PreservesUpstreamOrderNoSorting` or rewrite its body/name — do not leave both tests using the same zero-value fixture, which would pass vacuously.

---

### `internal/musicbrainz/search.go` (service, CRUD/decode)

**Analog:** itself, current `Artist` struct (read this session); secondary analog is `internal/deezer/search.go`'s parallel struct-field-addition shape

**Current struct** (`internal/musicbrainz/search.go:39-46`):
```go
type Artist struct {
	MBID           string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Disambiguation string `json:"disambiguation"`
	Type           string `json:"type"`
	Score          int    `json:"score"`
}
```

**D-09 fix — add `Country`:**
```go
type Artist struct {
	MBID           string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Disambiguation string `json:"disambiguation"`
	Country        string `json:"country"`
	Type           string `json:"type"`
	Score          int    `json:"score"`
}
```
Note (from RESEARCH.md discrepancy 1): CONTEXT.md's note only flagged the `SearchArtist` wire-shape gap — this struct-level gap in `internal/musicbrainz/search.go` must also be closed; it decodes nothing extra today despite the raw JSON already carrying `country`.

---

### `internal/musicbrainz/search_test.go` (test, request-response/httptest)

**Analog:** itself — `driveFixture` already contains `"country": "CA"` (per RESEARCH.md line 283); secondary analog `internal/deezer/search_test.go`'s `TestSearchArtists_DecodesFixture` extension pattern above.

**Extend the existing decode-fixture test** with:
```go
if got.Country != "CA" {
	t.Errorf("Country = %q, want %q", got.Country, "CA")
}
```

---

### `internal/httpserver/search.go` (controller/adapter, request-response)

**Analog:** itself — `SearchArtist` struct and `musicBrainzSource.SearchArtists`/`deezerSource.SearchArtists` adapters (read this session, full relevant excerpts below)

**Current wire struct** (`internal/httpserver/search.go:34-41`):
```go
type SearchArtist struct {
	Source         string  `json:"source"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Disambiguation *string `json:"disambiguation"`
	Type           string  `json:"type"`
	ImageURL       *string `json:"image_url"`
}
```

**D-10 fix — add `Country *string`, following the exact `Disambiguation`/`ImageURL` nullable-pointer convention:**
```go
type SearchArtist struct {
	Source         string  `json:"source"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Disambiguation *string `json:"disambiguation"`
	Country        *string `json:"country"`
	Type           string  `json:"type"`
	ImageURL       *string `json:"image_url"`
}
```

**Current `musicBrainzSource.SearchArtists` adapter loop** (`internal/httpserver/search.go:82-104`, the exact seam to edit):
```go
func (s musicBrainzSource) SearchArtists(ctx context.Context, q string, limit int) ([]SearchArtist, error) {
	artists, err := s.client.SearchArtists(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SearchArtist, 0, len(artists))
	for _, a := range artists {
		var disambiguation *string
		if a.Disambiguation != "" {
			d := a.Disambiguation
			disambiguation = &d
		}
		out = append(out, SearchArtist{
			Source:         "musicbrainz",
			ID:             a.MBID,
			Name:           a.Name,
			Disambiguation: disambiguation,
			Type:           a.Type,
			ImageURL:       nil,
		})
	}
	return out, nil
}
```

**D-10 edit — add the identical-shape `Country` pointer conversion (per RESEARCH.md Pattern 3):**
```go
	for _, a := range artists {
		var disambiguation *string
		if a.Disambiguation != "" {
			d := a.Disambiguation
			disambiguation = &d
		}
		var country *string
		if a.Country != "" {
			c := a.Country
			country = &c
		}
		out = append(out, SearchArtist{
			Source:         "musicbrainz",
			ID:             a.MBID,
			Name:           a.Name,
			Disambiguation: disambiguation,
			Country:        country,
			Type:           a.Type,
			ImageURL:       nil,
		})
	}
```

**`deezerSource.SearchArtists`** (`internal/httpserver/search.go:123-149`) — leave `Country: nil` always in the constructed `SearchArtist{...}` literal (Deezer has no country field), mirroring how it already always sets `Disambiguation: nil`. D-05 locks `NbFan` off the wire entirely — do not add it here.

---

### `internal/httpserver/search_test.go` (test, request-response)

**Analog:** itself — existing `TestNewDeezerSource_MapsFields`-style adapter test (per RESEARCH.md's Open Question 2 recommendation and Wave 0 gap list)

**Pattern to add — D-07/D-09/D-10 Go-side coverage, new tests alongside the existing adapter tests:**
```go
func TestNewMusicBrainzSource_MapsFields(t *testing.T) {
	// stub musicbrainz.ArtistSearcher returning an Artist with
	// Disambiguation == "" and Country == "CA" -- assert the adapter's
	// output SearchArtist.Disambiguation is nil and .Country is a
	// non-nil pointer to "CA". Mirrors the existing
	// TestNewDeezerSource_MapsFields shape (stub client -> adapter ->
	// assert wire struct fields).
}

func TestNewMusicBrainzSource_PreservesOrder(t *testing.T) {
	// stub returns artists in a fixed, deliberately non-alphabetical,
	// non-score-sorted order -- assert musicBrainzSource.SearchArtists's
	// output order matches exactly (D-06/D-07 insurance against D-04's
	// Deezer sort leaking into this adapter).
}
```

---

### `web/app/lib/api.ts` (type/model, transform)

**Analog:** itself — current `SearchArtist` interface (read this session)

**Current interface** (`web/app/lib/api.ts:57-65`):
```ts
export interface SearchArtist {
  source: string
  id: string
  name: string
  disambiguation: string | null
  type: string
  image_url: string | null
}
```

**D-10 fix — add `country`, mirroring `disambiguation`'s nullable shape:**
```ts
export interface SearchArtist {
  source: string
  id: string
  name: string
  disambiguation: string | null
  country: string | null
  type: string
  image_url: string | null
}
```
Do NOT add `nb_fan`/`NbFan` here (D-05).

---

### `web/app/components/watchlist/SearchResultsColumns.tsx` (component, request-response render)

**Analog:** itself — `SearchResultRow`'s current disambiguation render (read this session, `SearchResultsColumns.tsx:170-181`)

**Current render** (verified this session):
```tsx
<CoverArt src={artist.image_url} alt={artist.name} size={48} />
<div className="flex min-w-0 flex-1 flex-col gap-0.5">
  <span className="truncate text-body font-medium text-foreground">
    {artist.name}
  </span>
  <span className="text-label text-muted-foreground">{artist.type}</span>
  {artist.disambiguation !== null && (
    <span className="truncate text-label text-muted-foreground">
      {artist.disambiguation}
    </span>
  )}
</div>
```

**D-10 fix — same slot, disambiguation-or-country fallback, plain text node (no `dangerouslySetInnerHTML` per Security Domain notes):**
```tsx
{(() => {
  const label = artist.disambiguation ?? artist.country
  return (
    label !== null && (
      <span className="truncate text-label text-muted-foreground">
        {label}
      </span>
    )
  )
})()}
```
Or equivalently, precompute `const disambiguationLabel = artist.disambiguation ?? artist.country` above the JSX return and reference it in the condition/content — whichever fits `SearchResultRow`'s existing component structure more cleanly (component has other local `const`s already, e.g. `alreadyWatching`, `pending`).

---

### `web/app/components/watchlist/SearchResultsColumns.test.tsx` (test, event-driven)

**Analog:** itself — existing conventions (`render, screen, waitFor` from `@testing-library/react`, per RESEARCH.md line 219 citation)

**Pattern to add — D-10 fallback render test:**
```tsx
it("falls back to country when disambiguation is blank", () => {
  const artist: SearchArtist = {
    source: "musicbrainz",
    id: "abc-123",
    name: "Drake",
    disambiguation: null,
    country: "CA",
    type: "Person",
    image_url: null,
  }
  render(/* render SearchResultRow or its parent list with this artist */)
  expect(screen.getByText("CA")).toBeInTheDocument()
})

it("prefers disambiguation over country when both present", () => {
  // disambiguation: "Canadian rapper", country: "CA" -> assert "Canadian rapper" renders, "CA" does not
})
```

## Shared Patterns

### Nullable wire field convention (Go)
**Source:** `internal/httpserver/search.go:38,40` (`Disambiguation *string`, `ImageURL *string`) and their population sites at lines 89-93, 130-134
**Apply to:** `SearchArtist.Country` in `internal/httpserver/search.go` and its TS mirror in `web/app/lib/api.ts`
```go
var country *string
if a.Country != "" {
	c := a.Country
	country = &c
}
```

### `slices.SortFunc` + `cmp.Compare` for descending numeric sort (Go stdlib, already idiom-consistent per `internal/httpserver/watchlist.go:198,206`'s `slices.Contains` usage)
**Source:** pkg.go.dev/slices#SortFunc, applied per RESEARCH.md Pattern 2
**Apply to:** `internal/deezer/search.go`'s `Client.SearchArtists`
```go
slices.SortFunc(artists, func(a, b Artist) int {
	return cmp.Compare(b.NbFan, a.NbFan)
})
```

### httptest.Server-backed client tests (Go)
**Source:** `internal/deezer/search_test.go`'s `newTestClient` helper and fixture-constant convention (whitebox `package deezer`, unexported `baseURL` override)
**Apply to:** `internal/musicbrainz/search_test.go`'s parallel `Country` decode test — same fixture-constant + httptest.Server + assertion-block shape

### RTL component test with `waitFor` for post-effect assertions (frontend)
**Source:** `web/app/components/watchlist/SearchResultsColumns.test.tsx:1` (`import { render, screen, waitFor } from "@testing-library/react"`)
**Apply to:** `CoverArt.test.tsx`'s D-02 regression test — any assertion made after a `rerender()` that depends on `useEffect` firing must be wrapped in `waitFor`, never asserted synchronously (per Pitfall 1)

### Plain-text JSX rendering, never `dangerouslySetInnerHTML` (security)
**Source:** repo-wide invariant, confirmed zero `dangerouslySetInnerHTML` matches across `web/app/` (`.planning/STATE.md:156`)
**Apply to:** `SearchResultsColumns.tsx`'s new `artist.country` fallback render — must stay a plain `{label}` JSX text node exactly like the existing `artist.disambiguation` render

## No Analog Found

None — every file in this phase is a self-contained edit to an existing file with its own current code as the direct pattern source, or a new test file with a directly adjacent sibling test file as the analog.

## Metadata

**Analog search scope:** `internal/deezer/`, `internal/musicbrainz/`, `internal/httpserver/`, `web/app/components/common/`, `web/app/components/watchlist/`, `web/app/lib/`
**Files scanned:** 10 (all read directly this session or in the upstream research session, per 12-RESEARCH.md's "Sources / Primary" list)
**Pattern extraction date:** 2026-08-18
