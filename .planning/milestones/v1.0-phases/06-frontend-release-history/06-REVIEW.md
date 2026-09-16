---
phase: 06-frontend-release-history
reviewed: 2026-08-11T00:00:00Z
depth: standard
files_reviewed: 25
files_reviewed_list:
  - .gitignore
  - cmd/server/main.go
  - internal/db/sqlc/events.sql.go
  - internal/db/sqlc/querier.go
  - internal/events/service.go
  - internal/httpserver/events.go
  - internal/httpserver/events_test.go
  - internal/httpserver/server.go
  - internal/httpserver/spa_test.go
  - internal/webassets/embed.go
  - Makefile
  - queries/events.sql
  - web/app/app.css
  - web/app/components/common/CoverArt.tsx
  - web/app/components/common/EmptyState.tsx
  - web/app/components/history/EventCard.tsx
  - web/app/components/history/HistoryFilters.tsx
  - web/app/components/watchlist/PreferenceToggles.tsx
  - web/app/components/watchlist/SearchBox.tsx
  - web/app/components/watchlist/SearchResultsColumns.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/lib/api.ts
  - web/app/root.tsx
  - web/app/routes.ts
  - web/app/routes/history.tsx
  - web/app/routes/watchlist.tsx
findings:
  critical: 1
  warning: 4
  info: 1
  total: 6
status: issues_found
---

# Phase 06: Code Review Report

**Reviewed:** 2026-08-11
**Depth:** standard
**Files Reviewed:** 25
**Status:** issues_found

## Summary

The Go backend additions for this phase (`internal/events`, `internal/httpserver/events.go`, the `events.sql`/sqlc generated code, and the `webassets` SPA-fallback wiring) are solid: pagination is correctly keyset-based and race-safe, error responses never leak driver text, page-size clamping lives in the domain layer as intended, and the handler/service/store layering mirrors the existing watchlist pattern cleanly. Test coverage for `GET /events` and the SPA fallback is thorough and exercises the real edge cases (empty vs. nil slices, malformed params, keyset boundaries, real-Postgres pagination).

The frontend has one data-integrity bug that undermines a core feature: adding an artist from the Deezer search column stores the Deezer catalog ID in the field the rest of the system treats as a MusicBrainz ID, silently breaking MusicBrainz-based release detection for that artist going forward. Three further issues affect correctness/robustness of the Watchlist and History UI under specific conditions (stale preference overwrite race, "already watching" cross-reference tied to the same mbid mix-up, a badge lookup with no defensive fallback), plus a misleading abort-control comment and a minor URL-encoding gap.

## Critical Issues

### CR-01: Adding an artist via the Deezer search column stores the Deezer ID as `mbid`, breaking MusicBrainz polling for that artist

**File:** `web/app/routes/watchlist.tsx:72-80`
**Issue:** `handleAddSearchResult` unconditionally sets `mbid: result.id`, regardless of which source produced the result:

```ts
async function handleAddSearchResult(sourceName: string, result: SearchArtist) {
  try {
    await addWatchlist({
      mbid: result.id,
      name: result.name,
      imageUrl: result.image_url ?? undefined,
      disambiguation: result.disambiguation ?? undefined,
      deezerId: sourceName === "deezer" ? result.id : undefined,
    })
```

`SearchArtist.id` is source-specific (`internal/httpserver/search.go`): for MusicBrainz results it is the real MBID (`a.MBID`), but for Deezer results it is `strconv.FormatInt(a.ID, 10)` — a Deezer catalog id with no relation to MusicBrainz at all. The backend's `POST /watchlist` treats `mbid` as the artist's canonical identity and never validates its format (`internal/httpserver/watchlist.go` comment: "format is not validated"), so this numeric Deezer id is written straight into `artists.mbid`.

That column is exactly what the poller uses to call MusicBrainz: `internal/poller/poller.go:234` — `p.mb.ReleaseGroupsByArtist(ctx, entry.MBID)`. Any artist added from the Deezer results column therefore gets a bogus "mbid" and every subsequent MusicBrainz poll cycle for that artist will fail (404/lookup miss) — new-release, guest-feature and deluxe-change detection sourced from MusicBrainz silently never works for that artist, with no user-visible error. This is a permanent, silent data-integrity bug for a core feature (release detection), not just a cosmetic UI issue.

**Fix:** Only populate `mbid` when the result actually came from MusicBrainz. Either require an MBID before allowing a Deezer-only add (disable/hide the "Add to Watchlist" action for Deezer-sourced results until a matching MusicBrainz identity is resolved), or thread through a proper cross-source resolution step. At minimum, do not pass the Deezer catalog id as `mbid`:

```ts
await addWatchlist({
  mbid: sourceName === "musicbrainz" ? result.id : /* resolve real MBID or block add */ "",
  name: result.name,
  imageUrl: result.image_url ?? undefined,
  disambiguation: result.disambiguation ?? undefined,
  deezerId: sourceName === "deezer" ? result.id : undefined,
})
```

## Warnings

### WR-01: "Already watching" cross-reference only checks `mbid`, ignoring source

**File:** `web/app/components/watchlist/SearchResultsColumns.tsx:106-109`
**Issue:**
```ts
alreadyWatching={
  watchlistEntries?.some((entry) => entry.mbid === artist.id) ??
  false
}
```
This is used for both the MusicBrainz and Deezer columns. For the Deezer column, `artist.id` is a Deezer catalog id, which should be compared against `entry.deezer_id`, not `entry.mbid`. Today this check happens to "work" for Deezer results only because of CR-01 (the Deezer id gets stored into `mbid` on add) — once CR-01 is fixed, this comparison will never match a Deezer search result against an already-watchlisted artist that was added via Deezer, and users will be able to attempt duplicate adds (caught only by the 409 backstop, but the UX regresses).

**Fix:** Dispatch the comparison on `sourceName`:
```ts
alreadyWatching={
  watchlistEntries?.some((entry) =>
    sourceName === "deezer" ? entry.deezer_id === artist.id : entry.mbid === artist.id,
  ) ?? false
}
```

### WR-02: Concurrent independent-axis preference PATCHes can overwrite each other's UI state

**File:** `web/app/components/watchlist/PreferenceToggles.tsx:30-62`, `web/app/routes/watchlist.tsx:54-56`
**Issue:** `toggleReleaseType` and `toggleMutedEventType` each PATCH one axis independently and, on success, call `onEntryChange(updated)` with the *entire* row the server returned — `handleEntryChange` in `watchlist.tsx` then blindly replaces the whole entry in state:
```ts
function handleEntryChange(updated: WatchlistEntry) {
  setEntries((rows) => (rows ? rows.map((r) => (r.id === updated.id ? updated : r)) : rows))
}
```
If a user toggles a release-type checkbox and a mute checkbox in quick succession (each fieldset only disables its own axis while pending, so this is possible), two independent PATCH requests are in flight concurrently. The server correctly serializes the two writes at the DB level (row lock + CASE-based partial update), so the row ends up correct in Postgres either way. But each PATCH's HTTP response reflects the row as it existed at the moment *that* UPDATE committed — if the slower request's response arrives at the browser after the faster one, its stale snapshot (missing the other axis's just-applied change) overwrites the client's `entries` state, so the UI can display the wrong checkbox state for one axis until the next full refresh.

**Fix:** Merge only the axis actually being updated instead of replacing the whole row, e.g. have `onEntryChange` accept a partial update, or have each toggle handler apply only its own field from the response:
```ts
onEntryChange({ ...entry, release_types: updated.release_types })
```

### WR-03: `EventCard` badge lookup has no fallback for an unexpected `event_type`

**File:** `web/app/components/history/EventCard.tsx:39`
**Issue:**
```ts
export function EventCard({ event }: EventCardProps) {
  const badge = EVENT_BADGE[event.event_type]
  ...
          <Badge ... style={{ backgroundColor: badge.color }}>
```
`EVENT_BADGE` is keyed by the three known `event_type` literals, but the value actually comes from the database via `GET /events` with no runtime validation against that union — a future event type, a manual DB edit, or a bug elsewhere in detection could produce a value not in `EVENT_BADGE`. `badge` would be `undefined`, and `badge.color`/`badge.emoji`/`badge.label` throw a `TypeError`, crashing the whole History route (caught only by the top-level `ErrorBoundary`, which shows the generic "Oops!" page for the entire tab instead of just that one card).

**Fix:** Fall back to a default badge instead of indexing unconditionally:
```ts
const badge = EVENT_BADGE[event.event_type] ?? { label: event.event_type, emoji: "❔", color: "var(--color-secondary)" }
```

### WR-04: Search abort control doesn't actually cancel the in-flight request

**File:** `web/app/lib/api.ts:100-122, 200-203`, `web/app/components/watchlist/SearchBox.tsx:44-68`
**Issue:** `SearchBox`'s doc comment claims "a fresh AbortController is created per debounced search and the prior one is aborted before the new one starts" — but `apiFetch` never receives or forwards any `AbortSignal` to `fetch()`:
```ts
async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
```
and `searchArtists` calls `apiFetch<SearchResponse>(...)` with no `init` argument at all. `abortRef.current?.abort()` therefore only flips `controller.signal.aborted` for the purpose of the manual `if (controller.signal.aborted) return` guards — it never cancels the underlying HTTP request. Every keystroke's debounced search runs to completion against the live MusicBrainz/Deezer-backed `GET /search` endpoint even after being superseded, consuming the shared rate-limit budget (`internal/musicbrainz`/`internal/deezer` limiters, shared process-wide per `cmd/server/main.go`) for requests whose results are discarded.

**Fix:** Thread the signal through:
```ts
async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  ...
}
export async function searchArtists(query: string, signal?: AbortSignal): Promise<SearchResponse> {
  const qs = new URLSearchParams({ q: query })
  return apiFetch<SearchResponse>(`/search?${qs.toString()}`, { signal })
}
```
and pass `controller.signal` from `SearchBox.runSearch`.

## Info

### IN-01: `guestFeatureHref` doesn't URL-encode `external_id`

**File:** `web/app/components/history/EventCard.tsx:108-116`
**Issue:**
```ts
function guestFeatureHref(event: EventItem): string | null {
  if (event.source === "musicbrainz") {
    return `https://musicbrainz.org/recording/${event.external_id}`
  }
  if (event.source === "deezer") {
    return `https://www.deezer.com/track/${event.external_id}`
  }
  return null
}
```
`external_id` is interpolated directly into the URL path without `encodeURIComponent`. In practice MusicBrainz/Deezer ids are UUIDs/numeric strings so this is low-risk today, but it's not guaranteed by any type or validation, and a value containing `?`, `#`, or `/` would produce a malformed or unintended link.

**Fix:**
```ts
return `https://musicbrainz.org/recording/${encodeURIComponent(event.external_id)}`
```

---

_Reviewed: 2026-08-11_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
