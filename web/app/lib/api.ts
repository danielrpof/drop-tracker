// Typed fetch wrappers for all five backend endpoints (health is not
// wrapped -- no UI surface consumes it). Every wire shape below is typed
// against the real Go response bodies read directly from
// internal/httpserver's handlers and internal/events/internal/watchlist's
// domain structs, not guessed from documentation, so plans 06-02/06-03/
// 06-04 never need to edit this file -- only import from it (06-01-PLAN.md
// Step 6).

// ---- Wire types --------------------------------------------------------

// EventItem mirrors internal/events.Event's JSON shape exactly.
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

// EventsPage mirrors internal/httpserver/events.go's eventsResponse
// envelope.
export interface EventsPage {
  events: EventItem[]
  next_cursor: number | null
}

// WatchlistEntry mirrors internal/watchlist.Entry's JSON shape. GET
// /watchlist returns a bare array of these -- no envelope.
export interface WatchlistEntry {
  id: number
  artist_id: number
  mbid: string
  name: string
  deezer_id: string | null
  disambiguation: string | null
  image_url: string | null
  release_types: string[]
  muted_event_types: string[]
  created_at: string
  updated_at: string
}

// SearchArtist mirrors internal/httpserver/search.go's SearchArtist.
export interface SearchArtist {
  source: string
  id: string
  name: string
  disambiguation: string | null
  type: string
  image_url: string | null
}

// SourceResult mirrors internal/httpserver/search.go's sourceResult (the
// per-source entry inside GET /search's sources map).
export interface SourceResult {
  status: "ok" | "error"
  error?: string
  artists: SearchArtist[]
}

// SearchResponse mirrors internal/httpserver/search.go's searchResponse
// envelope: sources is keyed by source name ("musicbrainz", "deezer"), no
// cross-source merge (D-02).
export interface SearchResponse {
  query: string
  sources: Record<string, SourceResult>
}

// ---- Error type ---------------------------------------------------------

// ApiError carries the HTTP status and the server's fixed {"error": "..."}
// message, so callers can branch on status (e.g. 409 vs 500) without
// re-parsing the response body themselves.
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}

// ---- Fetch core ----------------------------------------------------------

// apiFetch is the single fetch path every wrapper below funnels through:
// it parses a D-13 {"error": "..."} body on failure and throws ApiError, so
// every caller gets the same error shape regardless of which endpoint
// failed.
async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)

  if (res.status === 204) {
    return undefined as T
  }

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) {
        message = body.error
      }
    } catch {
      // Body wasn't valid JSON (or was empty) -- fall back to statusText,
      // set above.
    }
    throw new ApiError(res.status, message)
  }

  return (await res.json()) as T
}

// ---- Endpoint wrappers ----------------------------------------------------

// listEvents fetches one page of the history feed (HIST-01). params are
// forwarded as query string values when present -- this task's history.tsx
// only ever calls listEvents() with no params (the plain GET /events shape
// handleListEvents implements today); artist_id/event_type/cursor filtering
// is plan 06-02's addition, wired here now so that plan never edits this
// file.
export async function listEvents(params?: {
  artistId?: number
  eventType?: string
  cursor?: number
}): Promise<EventsPage> {
  const search = new URLSearchParams()
  if (params?.artistId != null) search.set("artist_id", String(params.artistId))
  if (params?.eventType) search.set("event_type", params.eventType)
  if (params?.cursor != null) search.set("cursor", String(params.cursor))
  const qs = search.toString()
  return apiFetch<EventsPage>(`/events${qs ? `?${qs}` : ""}`)
}

// listWatchlist fetches every watchlisted artist (WLST-04) -- a bare array,
// no envelope.
export async function listWatchlist(): Promise<WatchlistEntry[]> {
  return apiFetch<WatchlistEntry[]>("/watchlist")
}

// addWatchlist adds an artist to the watchlist (WLST-02, D-10): a single
// click on a search result with the D-08 defaults, no inline preference
// fields.
export async function addWatchlist(params: {
  mbid: string
  name: string
  deezerId?: string
  disambiguation?: string
  imageUrl?: string
}): Promise<WatchlistEntry> {
  return apiFetch<WatchlistEntry>("/watchlist", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      mbid: params.mbid,
      name: params.name,
      deezer_id: params.deezerId,
      disambiguation: params.disambiguation,
      image_url: params.imageUrl,
    }),
  })
}

// updateWatchlistPreferences applies a partial preferences update (WLST-05,
// WLST-06, D-12): an omitted axis is left untouched by the server -- callers
// pass only the axis they're changing.
export async function updateWatchlistPreferences(
  id: number,
  params: { releaseTypes?: string[]; mutedEventTypes?: string[] },
): Promise<WatchlistEntry> {
  return apiFetch<WatchlistEntry>(`/watchlist/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      release_types: params.releaseTypes,
      muted_event_types: params.mutedEventTypes,
    }),
  })
}

// removeWatchlist hard-deletes a watchlist entry (WLST-03, D-10). A 204
// carries no body -- apiFetch returns undefined for it.
export async function removeWatchlist(id: number): Promise<void> {
  await apiFetch<void>(`/watchlist/${id}`, { method: "DELETE" })
}

// searchArtists fans out to every configured source (WLST-01, D-01, D-02,
// D-03): the response's sources map is returned as-is, one entry per
// source, never merged.
export async function searchArtists(query: string): Promise<SearchResponse> {
  const qs = new URLSearchParams({ q: query })
  return apiFetch<SearchResponse>(`/search?${qs.toString()}`)
}
