---
created: 2026-08-11T18:30:00.000Z
title: SearchBox AbortController never cancels the underlying fetch
area: ui
severity: minor
files:
  - web/app/lib/api.ts:100-122,200-203
  - web/app/components/watchlist/SearchBox.tsx:44-68
---

## Problem

Surfaced by Phase 6 code review (WR-04, `.planning/phases/06-frontend-release-history/06-REVIEW.md`).

`SearchBox`'s doc comment claims "a fresh AbortController is created per debounced search and the prior one is aborted before the new one starts" — but `apiFetch` never receives or forwards any `AbortSignal` to `fetch()`:

```ts
async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
```

and `searchArtists` calls `apiFetch<SearchResponse>(...)` with no `init` argument at all. `abortRef.current?.abort()` only flips `controller.signal.aborted` for the manual `if (controller.signal.aborted) return` guards — it never cancels the underlying HTTP request. Every keystroke's debounced search runs to completion against the live MusicBrainz/Deezer-backed `GET /search` endpoint even after being superseded, consuming the shared rate-limit budget (`internal/musicbrainz`/`internal/deezer` limiters, shared process-wide) for requests whose results are discarded.

## Solution

Thread the signal through:

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
