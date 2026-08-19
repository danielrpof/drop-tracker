# Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking - Research

**Researched:** 2026-08-18
**Domain:** React state-reset patterns (frontend) + Go slice sort / wire-shape extension (backend)
**Confidence:** HIGH

## Summary

This phase closes two small, pre-scoped tech-debt items with no new dependencies and no new capabilities. CONTEXT.md already locked every implementation decision (D-01 through D-10); this research grounds those decisions against the actual current code so the planner can write precise `<action>` blocks, and surfaces two discrepancies between CONTEXT.md's stated assumptions and the code as it exists today.

**Discrepancy 1 (important):** CONTEXT.md's D-09 says MusicBrainz's `country` field is "already present in MusicBrainz's base `/ws/2/artist` search response at zero extra API-call cost" — true of the *raw JSON response* (verified in `internal/musicbrainz/search_test.go`'s own `driveFixture`, which already contains `"country": "CA"`), but the `musicbrainz.Artist` Go struct in `internal/musicbrainz/search.go` does **not** currently decode it — only `MBID`, `Name`, `SortName`, `Disambiguation`, `Type`, `Score` are declared, and the struct's own doc comment explicitly says "MusicBrainz's response carries more (country, life-span, etc.) with no consumer yet." The planner must add a task to add `Country string `json:"country"`` to that struct, not just to the wire-shape `SearchArtist` (D-10's note only calls out the wire-shape gap, not this one).

**Discrepancy 2 (minor):** the Deezer `Artist` struct in `internal/deezer/search.go` also does not yet decode `nb_fan` — `NbFan` is absent from the struct despite the field being present in the live fixture (`internal/deezer/search_test.go`'s `drakeSearchFixture` already contains `"nb_fan": 24047501`). D-03 already calls this out correctly ("Add `NbFan`... to `internal/deezer.Artist`") — flagged here only to confirm CONTEXT.md's claim is accurate and the exact fixture line to reuse.

**Primary recommendation:** Both fixes are additive, in-place edits with no call-site fan-out beyond the two named seams (`CoverArt.tsx` itself; `internal/deezer`, `internal/musicbrainz`, `internal/httpserver/search.go`, `web/app/lib/api.ts`, `SearchResultsColumns.tsx`). No new packages, no schema/migration changes, no wire-breaking changes — `SearchArtist.Country *string` is additive per D-10's note.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| CoverArt error-state reset | Browser / Client | — | Pure client-side `useState`/render bug inside a shared React component; no server involvement |
| Deezer fan-count capture + sort | API / Backend | — | `internal/deezer.Client.SearchArtists` decodes and sorts before the result ever reaches `internal/httpserver` (D-04 explicitly locks the sort inside `internal/deezer`, not the adapter) |
| MusicBrainz relevance-order preservation | API / Backend | — | No new logic — a regression test (D-07) proving `internal/httpserver/search.go`'s musicbrainz path never sorts, guarding against the Deezer sort work leaking across sources |
| Country fallback capture (MusicBrainz) | API / Backend | Browser / Client | Backend: decode `country` in `internal/musicbrainz.Artist` and thread it through `SearchArtist.Country` in `internal/httpserver/search.go`. Client: render it as a fallback in `SearchResultsColumns.tsx`'s existing disambiguation slot (D-10) |

## User Constraints (from CONTEXT.md)

<user_constraints>
### Locked Decisions

**CoverArt reset mechanism**
- D-01: Fix via `useEffect` keyed on `src` that resets `failed` to `false` — not a `key={src}` remount. Keeps the component instance retained; no call-site changes needed at any of the three consumers (`WatchlistRow.tsx`, `EventCard.tsx`, `SearchResultsColumns.tsx`) since the fix lives entirely inside `CoverArt.tsx`.
- D-02: Add a regression test proving the reset: render with a failing `src`, trigger `onError`, rerender with a new `src`, assert the placeholder clears.

**Deezer popularity signal**
- D-03: Add `NbFan` (`nb_fan` JSON field — already present in Deezer's live `/search/artist` response, confirmed in `internal/deezer/search_test.go`'s fixture) to `internal/deezer.Artist`.
- D-04: Sort `SearchArtists` results by fan count descending **inside `internal/deezer`'s client itself**, not in the `httpserver` adapter (`deezerSource.SearchArtists` in `internal/httpserver/search.go`). — **Reversibility:** reversible — pure internal sort logic, no wire-format change.
- D-05: `SearchArtist`'s wire shape (source/id/name/disambiguation/type/image_url) stays **unchanged** — fan count is a server-side sort key only, never exposed to the frontend.
- Note for planner: `internal/deezer/search.go`'s existing doc comment ("Results are returned in Deezer's own order — this method never sorts") will need updating to reflect the new sort behavior.

**MusicBrainz ranking strategy**
- D-06: No new ranking logic for MusicBrainz. `internal/musicbrainz.SearchArtists` already returns results pre-sorted by MusicBrainz's own relevance `score` — trust that order as-is. MusicBrainz's search API has no true popularity signal in its base response, and fetching one would require an extra per-result API call on a rate-limited, search-as-you-type endpoint — explicitly rejected as not worth the latency/quota cost.
- D-07: Add a pipeline-order test proving `GET /search`'s musicbrainz column preserves the client's returned order end-to-end (insurance against the Deezer sort work in D-04 accidentally leaking into the MusicBrainz path).

**Same-name disambiguation**
- D-08: Ranking alone was judged insufficient — add a supplementary fallback hint for when MusicBrainz's `disambiguation` field is blank.
- D-09: Use MusicBrainz's `country` field (e.g. `"CA"`) as the fallback. It is already present in MusicBrainz's base `/ws/2/artist` search response at zero extra API-call cost (live-verified in `.planning/milestones/v1.0-phases/03-external-clients-search/03-RESEARCH.md`'s Drake example: `"country": "CA"` returned alongside `"disambiguation": "Canadian rapper"` with no extra `inc` param). Life-span/founding-year was considered and rejected — not present in the base response, would need a separate per-result lookup (same cost tradeoff D-06 already rejected).
- D-10: Render the fallback in the **same UI slot** as disambiguation in `SearchResultRow` (`SearchResultsColumns.tsx`) — show MusicBrainz's `disambiguation` text when present, fall back to the country code when blank. No new UI element added.
- Note for planner: `SearchArtist`'s wire shape will need a new field to carry country (currently only `Disambiguation *string` exists) — this is additive, not a breaking change to the existing shape.

### Claude's Discretion
None — every gray area reached an explicit user decision this session.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. (Life-span/founding-year disambiguation and MusicBrainz per-result popularity lookups were considered and explicitly rejected for cost reasons, not deferred as future work — see D-06 and D-09.)
</user_constraints>

## Project Constraints (from CLAUDE.md)

- Go (not Python), chi router, sqlc — not applicable to this phase's changes (no DB, no new routes)
- Hand-rolled MusicBrainz/Deezer clients (`internal/musicbrainz`, `internal/deezer`) — already the pattern; this phase extends them in place, no new client library
- Testing: unit tests use `httptest.Server` to mock MusicBrainz/Deezer, no live external calls in CI — the existing `search_test.go` files in both packages already follow this; new tests for D-03/D-09's field additions must follow the same pattern
- All secrets via environment variables only — not applicable, no secrets touched
- React + Vite frontend, built and embedded via `go:embed` — CoverArt.tsx fix is a pure frontend change; no backend embed step needed since it doesn't touch `internal/webassets`

## Standard Stack

No new libraries required. This phase is a pure extension of already-adopted, in-repo patterns.

### Core (already in use, no version change)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | ^19.2.6 `[VERIFIED: web/package.json:22]` | `CoverArt.tsx`'s `useState`/`useEffect` | Already the project's UI framework |
| Go stdlib `slices` | go 1.26 toolchain `[VERIFIED: go.mod:3]` | `slices.SortFunc` for D-04's descending fan-count sort | Already imported elsewhere in the codebase (`slices.Contains` in `internal/httpserver/watchlist.go:198,206` and `internal/httpserver/events.go:85` `[VERIFIED: internal/httpserver/watchlist.go:198,206]`) — no new import needed, consistent with existing idiom |
| Go stdlib `cmp` | go 1.26 toolchain | `cmp.Compare` inside the `slices.SortFunc` comparator | Standard pairing with `slices.SortFunc` per official docs (see Code Examples) — avoids integer-subtraction overflow that a hand-rolled `a.NbFan - b.NbFan` comparator risks |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `slices.SortFunc` + `cmp.Compare` | `sort.Slice` | `sort.Slice` uses reflection and an untyped `less(i, j int) bool` closure — slightly slower and less type-safe than the generic `slices.SortFunc`. The codebase has no existing `sort.Slice` usage to mirror (grep found none in production code), so `slices.SortFunc` is the cleaner, idiom-consistent choice given `slices.Contains` is already the established stdlib-sort-adjacent import. |
| `useEffect` reset (D-01, locked) | `key={src}` remount | React's own official docs actually recommend `key` for this exact "reset all state on prop change" case (see Common Pitfalls below) — but D-01 explicitly rejects it to avoid touching all three call sites. This is a locked, deliberate deviation from the framework's stated best practice, not an oversight; the planner must not "fix" it back to `key`. |

**Installation:** None — no `npm install` / `go get` needed for this phase.

**Version verification:** `go.mod` pins `go 1.26` `[VERIFIED: go.mod:3]`; `web/package.json` pins `react@^19.2.6`, `vitest@4.1.10`, `@testing-library/react@16.3.2`, `@testing-library/user-event@14.6.4` `[VERIFIED: web/package.json:22,39,34,35]`. No package version changes are needed for this phase's scope.

## Package Legitimacy Audit

Not applicable — this phase introduces zero new external packages (no `npm install`, no `go get`). Every dependency used (React, Go stdlib `slices`/`cmp`, Vitest, Testing Library) is already installed and locked in `web/package.json` / `go.mod`.

## Architecture Patterns

### System Architecture Diagram

```
Frontend (client tier)                          Backend (API tier)
────────────────────────                        ─────────────────────────────────────────

 WatchlistRow.tsx ─┐
 EventCard.tsx ────┼─► CoverArt.tsx              GET /search?q=...
 SearchResultsCols ┘     │  useState(failed)          │
                          │  useEffect([src]) ──►      ▼
                          │  resets failed=false   handleSearch (search.go)
                          ▼                             │  fan-out (goroutines)
                     <img onError=setFailed(true)>       ├──► musicBrainzSource.SearchArtists
                                                          │       │
 SearchResultsColumns.tsx                                │       ▼
   artist.disambiguation                                 │  musicbrainz.Client.SearchArtists
     ?? artist.country (D-10 fallback)  ◄─────────────────┤    GET /ws/2/artist?query=...
                                                          │    decode Artist{..., Country}  (NEW field)
                                                          │    return in MusicBrainz's own
                                                          │    relevance-score order (no sort)
                                                          │
                                                          └──► deezerSource.SearchArtists
                                                                  │
                                                                  ▼
                                                             deezer.Client.SearchArtists
                                                               GET /search/artist?q=...
                                                               decode Artist{..., NbFan}  (NEW field)
                                                               slices.SortFunc: NbFan desc (NEW)
                                                               return sorted []Artist
```

A reader can trace the primary CoverArt-fix use case (prop changes on a retained instance → `useEffect` fires → placeholder clears) and the primary search use case (`GET /search` → per-source fan-out → MusicBrainz preserves relevance order / Deezer now sorts by fan count → `SearchArtist.Country` fallback rendered client-side) end to end above.

### Recommended Project Structure

No new files/folders. All edits land in existing files:
```
web/app/components/common/CoverArt.tsx           # D-01 useEffect reset
web/app/components/common/CoverArt.test.tsx      # D-02 regression test (NEW FILE — none exists yet)
internal/deezer/search.go                        # D-03 NbFan field, D-04 sort
internal/deezer/search_test.go                   # tests for NbFan decode + sort order
internal/musicbrainz/search.go                   # D-09 Country field (gap CONTEXT.md's note didn't call out)
internal/musicbrainz/search_test.go              # test for Country decode
internal/httpserver/search.go                    # D-10 SearchArtist.Country field, both adapters
internal/httpserver/search_test.go               # D-07 pipeline-order test, Country mapping test
web/app/lib/api.ts                               # SearchArtist.country: string | null mirrored type
web/app/components/watchlist/SearchResultsColumns.tsx  # D-10 disambiguation-or-country fallback render
web/app/components/watchlist/SearchResultsColumns.test.tsx  # test for the fallback render
```

### Pattern 1: Reset derived state during render vs. in an Effect (context for D-01's locked choice)
**What:** React's official guidance for "reset all state when a prop changes" is to remount via `key`, not `useEffect`, because the `useEffect` path renders once with stale state, then re-renders after the effect commits.
**When to use:** D-01 has already chosen `useEffect` for this phase, overriding the framework's default recommendation, specifically to avoid a `key={src}` remount at three call sites. Document this reasoning inline in `CoverArt.tsx`'s comment so a future reader doesn't "fix" it back to `key`.
**Example:**
```jsx
// Source: react.dev/learn/you-might-not-need-an-effect (fetched this session)
// React's own recommended pattern for FULL state reset on prop change:
export default function ProfilePage({ userId }) {
  return <Profile userId={userId} key={userId} />
}
// CoverArt.tsx deliberately does NOT use this pattern (D-01) -- see below.
```
```tsx
// CoverArt.tsx's chosen pattern (D-01): useEffect keyed on src.
const [failed, setFailed] = useState(false)

useEffect(() => {
  setFailed(false)
}, [src])
```

### Pattern 2: `slices.SortFunc` with `cmp.Compare` for descending numeric sort (D-04)
**What:** Sort a `[]Artist` slice in place by `NbFan` descending, using the stdlib generic sort already idiom-consistent with this codebase's existing `slices.Contains` usage.
**When to use:** Inside `internal/deezer.Client.SearchArtists`, immediately before returning `artists` — after the `append(artists, env.Data...)` line, before `return artists, nil`.
**Example:**
```go
// Source: pkg.go.dev/slices#SortFunc (fetched this session)
// cmp(a, b) returns negative when a<b, positive when a>b, zero when equal.
import "cmp"
import "slices"

slices.SortFunc(artists, func(a, b Artist) int {
	return cmp.Compare(b.NbFan, a.NbFan) // swapped args => descending
})
```

### Pattern 3: Additive nullable wire field (mirrors existing `Disambiguation`/`ImageURL` convention)
**What:** `SearchArtist.Country *string` follows the same nullable-pointer convention `Disambiguation *string` and `ImageURL *string` already use `[VERIFIED: internal/httpserver/search.go:38,40]` (quoted: `` Disambiguation *string `json:"disambiguation"` `` and `` ImageURL       *string `json:"image_url"` ``).
**When to use:** `musicBrainzSource.SearchArtists`'s adapter loop (`internal/httpserver/search.go:82-104`) — populate `Country` the same way `Disambiguation` is populated today (empty-string check → pointer, else nil). `deezerSource.SearchArtists` leaves `Country: nil` always (Deezer has no country field), mirroring how it already leaves `Disambiguation: nil` always.
**Example:**
```go
// Source: internal/httpserver/search.go:88-93 (existing Disambiguation pattern, read this session)
var disambiguation *string
if a.Disambiguation != "" {
	d := a.Disambiguation
	disambiguation = &d
}
// New Country field follows the identical shape:
var country *string
if a.Country != "" {
	c := a.Country
	country = &c
}
```

### Anti-Patterns to Avoid
- **Sorting in the `httpserver` adapter instead of the `deezer` client:** D-04 explicitly locks the sort location to `internal/deezer.Client.SearchArtists` itself, not `deezerSource.SearchArtists` in `internal/httpserver/search.go`. Sorting in the adapter would leave the raw client's public contract lying (its own doc comment already claims "never sorts" and D-04 corrects that comment, not the adapter's).
- **Exposing `NbFan` on the wire:** D-05 locks `SearchArtist`'s wire shape unchanged. Do not add `NbFan`/`nb_fan` to `SearchArtist` or `web/app/lib/api.ts`'s `SearchArtist` type — it is a server-side sort key only.
- **Fetching MusicBrainz life-span/extra data per result:** D-06/D-09 already rejected this for latency/quota cost on a debounced search-as-you-type endpoint. Do not add an `inc=` param or second per-artist request.

## Don't Hand-Roll

Not applicable in the traditional sense — both fixes are small, self-contained additions to existing hand-rolled code (`internal/deezer`, `internal/musicbrainz` per CLAUDE.md's explicit "hand-roll these clients" constraint). No general-purpose problem here (auth, validation, crypto, etc.) that a library would better solve.

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Sorting Go slices | A hand-rolled bubble/insertion sort or `sort.Interface` boilerplate | `slices.SortFunc` (stdlib, Go 1.21+) | Already the project's idiom (`slices.Contains` elsewhere); generic, no reflection, no extra type boilerplate |

**Key insight:** This phase's small blast radius (two independent, self-contained fixes) means the main risk isn't hand-rolling something that already exists — it's scope creep into the two things this phase explicitly rejected (MusicBrainz popularity fetch, life-span disambiguation). Stay inside D-01–D-10.

## Common Pitfalls

### Pitfall 1: `useEffect` reset causes a one-frame stale-placeholder flash
**What goes wrong:** When `src` changes and the previous load had failed, the component first re-renders with the *old* `failed=true` state (since `useEffect` only runs after the render commits), showing the placeholder for one frame, then the effect fires, calls `setFailed(false)`, and a second render shows the real `<img>`.
**Why it happens:** This is exactly the pattern React's own docs warn about: *"🔴 Avoid: Resetting state on prop change in an Effect... This is inefficient because the component renders with stale values first, then re-renders after the Effect runs"* `[CITED: react.dev/learn/you-might-not-need-an-effect]`.
**How to avoid:** D-01 already accepts this tradeoff deliberately in exchange for not touching three call sites — do not "fix" this by switching to `key={src}` (that reopens exactly the call-site-change cost D-01 rejected) or by adjusting state during render (that changes the mechanism D-01 locked). Document the one-frame flash as a known, accepted cost in `CoverArt.tsx`'s comment.
**Warning signs:** A regression test (D-02) that renders, fails, changes `src`, and asserts *synchronously* (no `waitFor`) that the placeholder is gone will flake — the test must use `waitFor`/`act` to await the effect's post-commit re-render (Testing Library's `render` from `@testing-library/react@16.3.2` `[VERIFIED: web/package.json:34]` flushes effects inside `act`, but an assertion made before the effect's queued state update flushes will still see the stale value in some timing orders). Existing test files in this repo (`SearchResultsColumns.test.tsx`) already import `waitFor` from `@testing-library/react` for exactly this class of async-state assertion `[VERIFIED: web/app/components/watchlist/SearchResultsColumns.test.tsx:1]` (quoted: `import { render, screen, waitFor } from "@testing-library/react"`).

### Pitfall 2: Adding `Country`/`NbFan` fields without updating decode-verifying tests
**What goes wrong:** `internal/deezer/search_test.go`'s `TestSearchArtists_DecodesFixture` already contains a fixture with `nb_fan: 24047501` but its assertions (lines 101-116) never check `NbFan` — silently leaving the new field's decode unverified even after D-03 lands, unless the test is explicitly extended.
**Why it happens:** The fixture was written before the field was needed; adding the Go struct field alone does not retroactively add an assertion.
**How to avoid:** Extend `TestSearchArtists_DecodesFixture` in both `internal/deezer/search_test.go` and `internal/musicbrainz/search_test.go` with an explicit `got.NbFan != 24047501` / `got.Country != "CA"` check, not just a new standalone test — the existing fixtures (`drakeSearchFixture`, `driveFixture`) already carry the right values and don't need editing.
**Warning signs:** `go vet`/`go build` will pass with the new struct fields even if no test ever asserts their decoded value — this is a silent gap, not a compile error.

### Pitfall 3: Deezer sort test racing with the existing "preserves order" test's intent
**What goes wrong:** `internal/deezer/search_test.go` currently has `TestSearchArtists_PreservesUpstreamOrderNoSorting` (line 170) which explicitly asserts `internal/deezer` never reorders results — this test's name and body directly contradict D-04's new sort-by-fan-count behavior and must be updated/removed, not left alongside a new sort test (they will both pass only if the fixture happens to already be in fan-count order, silently hiding the fact that one test's premise is now false).
**Why it happens:** The Deezer client's "never sorts" guarantee predates this phase; D-04 changes it.
**How to avoid:** The planner must include a task to either rename/rewrite `TestSearchArtists_PreservesUpstreamOrderNoSorting` (e.g. to explicitly test fan-count-descending order using a fixture with out-of-order fan counts) or delete it and add a new `TestSearchArtists_SortsByFanCountDescending` test with a fixture where upstream order and fan-count order deliberately differ (so the test can't pass vacuously).
**Warning signs:** A green test suite where the "preserves order" test and a new "sorts by fan count" test both pass using the *same* fixture (`twoArtistsSearchFixture`, which has no `nb_fan` values set — both default to `0`) proves nothing; the fixture must be extended with distinct `nb_fan` values.

### Pitfall 4: `SearchArtist.Country` omitted from `web/app/lib/api.ts` breaks the D-10 render before it starts
**What goes wrong:** `SearchResultsColumns.tsx`'s `SearchResultRow` reads `artist.country` for the D-10 fallback, but `web/app/lib/api.ts`'s `SearchArtist` interface (currently 6 fields: `source, id, name, disambiguation, type, image_url` `[VERIFIED: web/app/lib/api.ts:58-65]`, quoted: `` disambiguation: string | null `` / `` image_url: string | null ``) has no `country` field yet — TypeScript will not error on reading a nonexistent property in a plain object literal test fixture unless `strict` mode's excess-property checks apply, but any code reading `artist.country` against the *type* will fail to compile (`Property 'country' does not exist on type 'SearchArtist'`).
**Why it happens:** The Go wire shape and the TS mirror type are two independently-maintained files with no shared codegen (per this project's existing convention, `web/app/lib/api.ts`'s own top-of-file comment says wire types are typed "against the real Go response bodies... not guessed").
**How to avoid:** The planner must sequence the `web/app/lib/api.ts` type update *before or in the same task as* `SearchResultsColumns.tsx`'s fallback-render change — both belong in the same task, not split across a backend-owned and frontend-owned plan boundary, to avoid a broken intermediate compile state.
**Warning signs:** `npm run typecheck` (`react-router typegen && tsc`) failing with a missing-property error on `SearchResultsColumns.tsx`.

## Code Examples

### CoverArt.tsx's current state (before fix) — read this session
```tsx
// Source: web/app/components/common/CoverArt.tsx:20-22,48 (read this session)
export function CoverArt({ src, alt, size = 96, className }: CoverArtProps) {
  const [failed, setFailed] = useState(false)
  const showPlaceholder = !src || failed
  // ...
  // onError={() => setFailed(true)}
```
The fix (D-01) adds one `useEffect` hook after the existing `useState` line:
```tsx
useEffect(() => {
  setFailed(false)
}, [src])
```

### `internal/deezer.Artist`'s current fields (before D-03) — read this session
```go
// Source: internal/deezer/search.go:19-26 (read this session, quoted verbatim)
type Artist struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Link    string `json:"link"`
	Picture string `json:"picture"`
	NbAlbum int    `json:"nb_album"`
	Type    string `json:"type"`
}
```
D-03 adds `NbFan int `json:"nb_fan"`` alongside `NbAlbum` — same `int` type convention as the existing sibling numeric field.

### `internal/musicbrainz.Artist`'s current fields (before D-09's implied gap) — read this session
```go
// Source: internal/musicbrainz/search.go:39-46 (read this session, quoted verbatim)
type Artist struct {
	MBID           string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Disambiguation string `json:"disambiguation"`
	Type           string `json:"type"`
	Score          int    `json:"score"`
}
```
D-09 requires adding `Country string `json:"country"`` here — the raw JSON field already round-trips through the existing `driveFixture` test fixture (`internal/musicbrainz/search_test.go:33`, quoted: `` "country": "CA", ``) even though nothing decodes it into the struct today.

### `internal/httpserver.SearchArtist`'s current wire shape — read this session
```go
// Source: internal/httpserver/search.go:34-41 (read this session, quoted verbatim)
type SearchArtist struct {
	Source         string  `json:"source"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Disambiguation *string `json:"disambiguation"`
	Type           string  `json:"type"`
	ImageURL       *string `json:"image_url"`
}
```
D-10 adds `Country *string `json:"country"`` here, following the `Disambiguation`/`ImageURL` nullable-pointer convention exactly.

### `web/app/lib/api.ts`'s current mirror type — read this session
```ts
// Source: web/app/lib/api.ts:57-65 (read this session, quoted verbatim)
export interface SearchArtist {
  source: string
  id: string
  name: string
  disambiguation: string | null
  type: string
  image_url: string | null
}
```
Needs `country: string | null` added, mirroring `disambiguation`'s `string | null` shape.

### `SearchResultsColumns.tsx`'s current disambiguation render — read this session
```tsx
// Source: web/app/components/watchlist/SearchResultsColumns.tsx:176-180 (read this session, quoted verbatim)
{artist.disambiguation !== null && (
  <span className="truncate text-label text-muted-foreground">
    {artist.disambiguation}
  </span>
)}
```
D-10's fallback logic replaces the condition/content with a disambiguation-or-country choice, e.g. `artist.disambiguation ?? artist.country`, rendered only when that resolved value is non-null (same visual slot, same `<span>`).

## State of the Art

Not applicable — no library/framework version changes, no deprecated APIs involved. Both fixes use patterns already established elsewhere in this exact codebase (nullable wire fields, `slices` stdlib usage, `useEffect`/`useState` in React 19).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | MusicBrainz's `/ws/2/artist` search response reliably includes `country` for every artist that has one on file (not just the specific Drake example live-verified in a prior session) | Standard Stack / Pitfall notes, inherited from CONTEXT.md D-09 | If some artists' MusicBrainz records lack `country` entirely (not just blank `disambiguation`), the fallback will sometimes show nothing for either field — this degrades gracefully (falls through to no fallback text) rather than breaking, so risk is low, but the planner should treat `Country` as `*string`/nullable throughout (already the plan) rather than assuming always-present |
| A2 | Deezer's `nb_fan` field is present on every `/search/artist` result, not just popular ones | Standard Stack, inherited from D-03/D-04 | If `nb_fan` is occasionally absent/zero for less-popular artists, they simply sort last (JSON `int` zero-value) — degrades gracefully, no crash risk |

*(A1 and A2 both trace to CONTEXT.md's own D-09/D-03 assumptions, carried forward here rather than independently re-verified against a second live artist lookup this session — the original live verification is `.planning/milestones/v1.0-phases/03-external-clients-search/03-RESEARCH.md`, cited above.)*

## Open Questions

1. **Should the `TestSearchArtists_PreservesUpstreamOrderNoSorting` test in `internal/deezer/search_test.go` be renamed or replaced?**
   - What we know: Its current name/assertions directly contradict D-04's new sort behavior (Pitfall 3 above).
   - What's unclear: Whether the planner wants to preserve test history by renaming in place vs. delete-and-add-new.
   - Recommendation: Rename in place (`TestSearchArtists_SortsByFanCountDescending`) and rewrite its fixture to have distinct, out-of-order `nb_fan` values — keeps git blame continuity and avoids a vacuous "still passes because both values default to 0" false green.

2. **Does `internal/musicbrainz/search_test.go` need its own new "preserves order" style test extended, or is `TestSearchArtists_PreservesUpstreamOrderNoSorting` (already present, line 213) sufficient for D-06/D-07?**
   - What we know: `internal/musicbrainz/search_test.go` already has `TestSearchArtists_PreservesUpstreamOrderNoSorting` testing the *client* package directly — this proves the client never sorts, but D-07 asks for a pipeline-order test at the `GET /search` HTTP-handler level (`internal/httpserver`), a different layer.
   - What's unclear: Whether D-07's "pipeline-order" test is meant to be a new test in `internal/httpserver/search_test.go` (proving `handleSearch` doesn't introduce sorting) in addition to the existing client-level test, or a restatement of it.
   - Recommendation: Add a new test in `internal/httpserver/search_test.go` (alongside `TestNewDeezerSource_MapsFields`) — e.g. `TestNewMusicBrainzSource_PreservesOrder` — using a stub `musicbrainz.ArtistSearcher` returning artists in a fixed, deliberately-non-alphabetical/non-score order, asserting the adapter output order matches exactly. This is the layer D-04's Deezer sort work could most plausibly leak into (since both adapters are in the same file), so testing at that exact seam is the most direct insurance.

## Environment Availability

Skipped — this phase has no new external tool/service/runtime dependencies. All work happens inside the existing Go toolchain (`go 1.26`, already verified working per `.planning/STATE.md`'s phase history) and the existing Node/pnpm/Vitest frontend toolchain, both already proven functional by every prior phase's CI runs.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | Go stdlib `testing` + `net/http/httptest` |
| Backend config file | none (stdlib) |
| Backend quick run command | `go test ./internal/deezer/... ./internal/musicbrainz/... ./internal/httpserver/... -short -race -count=1` |
| Backend full suite command | `make test` (== `make test-integration`, requires `db-up`) `[VERIFIED: Makefile:71,75]` |
| Frontend framework | Vitest 4.1.10 `[VERIFIED: web/package.json:46]` |
| Frontend config file | `web/vitest.config.ts` |
| Frontend quick/full run command | `pnpm test` (== `vitest run`, coverage always enabled per `vitest.config.ts`) `[VERIFIED: web/package.json:10; web/vitest.config.ts:34]` |

### Phase Requirements -> Test Map

No REQ-IDs are mapped to this phase (`ROADMAP.md` Phase 12 lists `**Requirements**: TBD` `[VERIFIED: .planning/ROADMAP.md:52]`) — CONTEXT.md's D-01 through D-10 decisions are the authoritative scope. Mapping D-IDs to tests instead:

| Decision | Behavior | Test Type | Automated Command | File Exists? |
|----------|----------|-----------|-------------------|-------------|
| D-01/D-02 | CoverArt clears `failed` when `src` changes | unit (RTL) | `pnpm test -- CoverArt` | ❌ Wave 0 — no `CoverArt.test.tsx` exists yet |
| D-03 | `deezer.Artist.NbFan` decodes `nb_fan` | unit | `go test ./internal/deezer/... -run TestSearchArtists_DecodesFixture` | ✅ existing test extended |
| D-04 | Deezer results sort by `NbFan` descending | unit | `go test ./internal/deezer/... -run TestSearchArtists_Sort` | ❌ Wave 0 — replaces/renames `TestSearchArtists_PreservesUpstreamOrderNoSorting` |
| D-05 | `SearchArtist` wire shape unchanged (no `nb_fan` leak) | unit | `go test ./internal/httpserver/... -run TestNewDeezerSource_MapsFields` | ✅ existing test, extend assertion list to confirm no fan-count field present |
| D-06/D-07 | MusicBrainz pipeline order preserved end-to-end | unit | `go test ./internal/httpserver/... -run TestNewMusicBrainzSource` | ❌ Wave 0 — new adapter-level order test |
| D-09 | `musicbrainz.Artist.Country` decodes `country` | unit | `go test ./internal/musicbrainz/... -run TestSearchArtists_DecodesFixture` | ✅ existing test extended |
| D-10 | `SearchArtist.Country` populated by `musicBrainzSource`, `SearchResultRow` renders disambiguation-or-country fallback | unit (Go) + unit (RTL) | `go test ./internal/httpserver/... -run TestNewMusicBrainzSource_MapsFields` && `pnpm test -- SearchResultsColumns` | ❌ Wave 0 for both — Go-side mapping test doesn't exist yet; RTL fallback-render test doesn't exist yet |

### Sampling Rate
- **Per task commit:** backend — `go test ./internal/deezer/... ./internal/musicbrainz/... ./internal/httpserver/... -short -race -count=1`; frontend — `pnpm test -- CoverArt` / `pnpm test -- SearchResultsColumns` (scoped run)
- **Per wave merge:** backend — `make test` (full, real-Postgres integration suite; none of this phase's changes touch the DB, but the make target is the project's own full-suite gate); frontend — `pnpm test` (full suite, coverage gate enforced at 70% all axes per `vitest.config.ts:56-61`)
- **Phase gate:** Full suite green (`make coverage-gate` for backend 80% threshold, `pnpm test`'s built-in 70% frontend thresholds) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `web/app/components/common/CoverArt.test.tsx` — new file, covers D-01/D-02 (no existing test file for this component at all)
- [ ] `internal/deezer/search_test.go` — extend/rename `TestSearchArtists_PreservesUpstreamOrderNoSorting`, covers D-04
- [ ] `internal/httpserver/search_test.go` — new `TestNewMusicBrainzSource_MapsFields`-style Country assertion + new order-preservation test, covers D-07/D-09/D-10 (Go side)
- [ ] `web/app/components/watchlist/SearchResultsColumns.test.tsx` — extend with a disambiguation-blank/country-present fixture case, covers D-10 (frontend side)
- No new test framework/config installs needed — both `go test` and Vitest are already fully wired (`Makefile`, `vitest.config.ts`)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Phase touches no auth surface |
| V3 Session Management | no | Phase touches no session surface |
| V4 Access Control | no | Phase touches no access-control surface |
| V5 Input Validation | yes (narrow) | `Country`/`NbFan` are decoded from third-party JSON exactly like every other field in these structs today — no new validation logic needed beyond the existing `encoding/json` typed-decode convention already in place (int/string typed fields, not `map[string]any`) |
| V6 Cryptography | no | Not touched |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Reflected XSS via third-party artist metadata (`country`, `disambiguation`) rendered into the DOM | Tampering / Information Disclosure | Already mitigated project-wide: `SearchResultsColumns.tsx` renders every artist field as a plain JSX text node (no `dangerouslySetInnerHTML`), confirmed by the codebase's existing repo-wide grep-verified zero-match invariant (`.planning/STATE.md`'s Phase 06 decision log: "a repo-wide `dangerouslySetInnerHTML` grep across `web/app/` returns zero matches" `[VERIFIED: .planning/STATE.md:156]`). The new `artist.country` render must follow the exact same plain-text-node pattern the existing `artist.disambiguation` render already uses (Code Examples above) — do not introduce a new rendering path (e.g. `dangerouslySetInnerHTML`, a raw DOM write) for this one field. |
| Untrusted numeric field (`nb_fan`) used only as an internal sort key, never rendered | — | No new attack surface: `NbFan`/`Country` are typed (`int`/`string`) via `encoding/json`'s standard typed decode, matching every other field in `deezer.Artist`/`musicbrainz.Artist` — a malformed/oversized `nb_fan` value simply decodes to Go's zero value or a JSON decode error (already handled by the existing `decodeChecked`/`json.NewDecoder` error paths), not a crash or injection vector. |

## Sources

### Primary (HIGH confidence)
- `web/app/components/common/CoverArt.tsx` — read this session (current pre-fix implementation)
- `web/app/components/watchlist/WatchlistRow.tsx`, `web/app/components/history/EventCard.tsx`, `web/app/components/watchlist/SearchResultsColumns.tsx` — read this session (confirmed all three consumers pass `src`/`image_url`/`cover_art_url` through unchanged, zero call-site changes needed per D-01)
- `internal/deezer/search.go`, `internal/deezer/search_test.go` — read this session (confirmed `NbFan`/sort gaps, confirmed `nb_fan: 24047501` fixture value)
- `internal/musicbrainz/search.go`, `internal/musicbrainz/search_test.go` — read this session (confirmed `Country` field gap, confirmed `"country": "CA"` fixture value)
- `internal/httpserver/search.go`, `internal/httpserver/search_test.go` — read this session (confirmed `SearchArtist` wire shape, adapter mapping pattern, existing test conventions)
- `web/app/lib/api.ts` — read this session (confirmed frontend `SearchArtist` type has no `country` field yet)
- `go.mod`, `web/package.json`, `web/vitest.config.ts`, `Makefile`, `.github/workflows/full-pipeline.yml` — read this session (versions, test/coverage commands)

### Secondary (MEDIUM confidence)
- react.dev/learn/you-might-not-need-an-effect — fetched this session via WebFetch, official React docs on reset-state-on-prop-change patterns (`key` vs `useEffect`)
- pkg.go.dev/slices#SortFunc — fetched this session via WebFetch, official Go stdlib docs for `slices.SortFunc`'s comparator contract
- `.planning/milestones/v1.0-phases/03-external-clients-search/03-RESEARCH.md` — cited (not re-verified live this session) for the original live-verified MusicBrainz `country`/`disambiguation` response shape

### Tertiary (LOW confidence)
- None — both WebSearch-sourced questions were cross-checked against their official documentation source (react.dev, pkg.go.dev) via WebFetch before being included, elevating them out of LOW confidence.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new libraries; all versions read directly from `go.mod`/`package.json` this session
- Architecture: HIGH — every file/struct/function referenced was read directly this session, not inferred
- Pitfalls: HIGH — all four pitfalls trace to specific, quoted lines in files read this session (existing tests whose premise D-04 breaks, existing wire types missing new fields, official React docs' own stated tradeoff)

**Research date:** 2026-08-18
**Valid until:** 30 days (stable internal codebase change, no external API/library version dependency risk)
