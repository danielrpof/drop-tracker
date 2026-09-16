// Typed fetch wrappers for all five backend endpoints (health is not
// wrapped -- no UI surface consumes it). Every wire shape below is typed
// against the real Go response bodies read directly from
// internal/httpserver's handlers and internal/events/internal/watchlist's
// domain structs, not guessed from documentation, so plans 06-02/06-03/
// 06-04 never need to edit this file -- only import from it (06-01-PLAN.md
// Step 6).

import { authStore } from "~/lib/authStore"

// X-Instance-Gated is a byte-for-byte contract with the server:
// internal/authgate/gate.go sets exactly this header (with the value below) on
// every response that passes gate.Authenticate. Changing one side without the
// other in the same commit silently stops the Log out control from ever
// appearing on a gated instance -- there is no compiler or runtime error on
// either side. This is the response-side sibling of the X-Requested-With CSRF
// header (D-15).
const INSTANCE_GATED_HEADER = "X-Instance-Gated"
const INSTANCE_GATED_VALUE = "1"

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
  watched_artist_name: string | null
  release_date: string | null
  cover_art_url: string | null
  track_count: number | null
  previous_track_count: number | null
  release_type: string | null
  notified_at: string | null
  created_at: string
}

// EventsPage mirrors internal/httpserver/events.go's eventsResponse
// envelope. has_older_events (DATA-02, D-06) is never null on the wire --
// it mirrors the Go bool exactly -- and tells the History route whether
// this scope has any event hidden by the retention window, distinct from
// "no events ever." next_cursor is an opaque token (quick task
// 260825-g6i replaced the raw numeric cursor with an encoded feed
// position that carries both a release date and an event id) -- the only
// correct client behaviour is to send it back verbatim as the next
// request's cursor; callers must not parse, compare, or arithmetically
// manipulate it.
export interface EventsPage {
  events: EventItem[]
  next_cursor: string | null
  has_older_events: boolean
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
  country: string | null
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

// KnownOutcome is the wire's closed set (ok | error | cancelled), widened
// with a bare-string arm so an N-1/N deploy value keeps literal autocomplete
// without breaking typecheck (D-04, D-07 of 19-CONTEXT.md).
export type KnownOutcome = "ok" | "error" | "cancelled"

// StatusRun mirrors internal/httpserver/status.go's statusRun exactly. Used
// by both StatusSource.last_run and every StatusSource.history element --
// the run object carries no source key, since the source name is already
// the map key in StatusResponse.sources.
export interface StatusRun {
  cycle_id: string
  started_at: string
  finished_at: string
  duration_ms: number
  artists_checked: number
  artists_skipped: number
  artists_errored: number
  events_recorded: number
  outcome: KnownOutcome | (string & {})
  summary: string
}

// StatusSource mirrors statusSource. history is allocated zero-length by
// the handler, so it always encodes as an array, never null.
export interface StatusSource {
  last_run: StatusRun | null
  history: StatusRun[]
  last_skipped_at: string | null
  consecutive_skips: number
}

// StatusInstance mirrors statusInstance. schema_applied is null only when
// the database was unreachable at request time; schema_expected is a plain
// Go uint and is never absent.
export interface StatusInstance {
  app_version: string
  schema_applied: number | null
  schema_expected: number
}

// StatusResponse mirrors internal/httpserver/status.go's statusResponse --
// the frozen GET /status contract (docs/api/status-contract.md). sources is
// always keyed by exactly "musicbrainz" and "deezer", including on a fresh
// instance with no recorded cycles.
export interface StatusResponse {
  poll_interval_seconds: number
  watchlist_size: number
  instance: StatusInstance
  sources: Record<string, StatusSource>
}

// NotificationSettings mirrors internal/httpserver/settings.go's
// settingsResponse exactly -- the GET/PUT /settings/notifications wire
// contract (DGST-01, DGST-16).
export interface NotificationSettings {
  digest_enabled: boolean
  digest_cadence: "daily" | "weekly"
  digest_last_sent_at: string | null
  updated_at: string
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
  // D-15 CSRF: every non-GET request to a gated route must carry a custom
  // header a cross-site attacker cannot set without a CORS preflight the
  // server denies. Injected here once so it covers every wrapper --
  // including removeWatchlist/deleteSession, which otherwise send no
  // headers at all -- rather than per wrapper. Server-side enforcement
  // (RequireCSRFHeader) lands in plan 14-04.
  const method = init?.method ?? "GET"
  const headers =
    method === "GET"
      ? init?.headers
      : { ...init?.headers, "X-Requested-With": "drop-tracker" }

  const res = await fetch(path, { ...init, headers })

  // G-14-3: a gated instance marks every response that passed gate.Authenticate
  // with X-Instance-Gated. Latch it here -- BEFORE the 401 / 204 / !ok branches
  // below, because a gated 204, a gated non-OK and a gated error body all
  // passed the gate and all carry the marker. The latch is one-way and
  // monotonic within the browser session (as 14-06 established for the
  // storage-backed flag): it is NEVER cleared when the marker is absent,
  // because exempt routes on a gated instance legitimately carry none -- so
  // absence proves nothing. The latch never touches the optimistic auth flag.
  if (res.headers.get(INSTANCE_GATED_HEADER) === INSTANCE_GATED_VALUE) {
    authStore.markGateActive()
  }

  // D-16 global 401 interceptor: this is the ONLY place client code flips
  // auth state on a 401. Any gated endpoint that returns 401 (initial load,
  // mid-session expiry, a race right after login) funnels through here, so
  // <App> renders <PassphraseScreen> without per-wrapper handling. The
  // ApiError still carries status 401 so a caller that cares can branch.
  if (res.status === 401) {
    authStore.markUnauthenticated()
    throw new ApiError(401, "unauthenticated")
  }

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
  cursor?: string
}): Promise<EventsPage> {
  const search = new URLSearchParams()
  if (params?.artistId != null) search.set("artist_id", String(params.artistId))
  if (params?.eventType) search.set("event_type", params.eventType)
  if (params?.cursor != null) search.set("cursor", params.cursor)
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
  params: { releaseTypes?: string[]; mutedEventTypes?: string[] }
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
// source, never merged. An optional signal lets a caller (SearchBox) cancel
// a superseded search at the request level rather than only discarding its
// resolved value -- apiFetch already forwards its whole init object to
// fetch, so placing the signal in init here is sufficient.
export async function searchArtists(
  query: string,
  signal?: AbortSignal
): Promise<SearchResponse> {
  const qs = new URLSearchParams({ q: query })
  return apiFetch<SearchResponse>(`/search?${qs.toString()}`, { signal })
}

// ---- Session (instance passphrase gate) ---------------------------------

// createSession logs the browser in: POST /session with the passphrase in
// the JSON body only (Pitfall 14 -- the value never touches a path, a query
// string, or a GET). It relies on apiFetch's 204 handling to resolve with
// no value. It deliberately does NOT call authStore.markAuthenticated()
// itself -- PassphraseScreen does that only after this promise resolves, so
// a rejected login can never flip auth state (GATE-05).
export async function createSession(passphrase: string): Promise<void> {
  await apiFetch<void>("/session", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ passphrase }),
  })
}

// deleteSession logs the browser out: DELETE /session, which responds 204 +
// Set-Cookie Max-Age=0 (GATE-06, D-10 -- client-local logout only).
export async function deleteSession(): Promise<void> {
  await apiFetch<void>("/session", { method: "DELETE" })
}

// ---- Status (operator observability) --------------------------------------

// getStatus fetches the gated operator status surface (SYS-01, SYS-02,
// SYS-03). Routed through apiFetch so it inherits the D-16 401 interceptor
// and the X-Instance-Gated latch -- the System view needs no per-view auth
// code.
export async function getStatus(): Promise<StatusResponse> {
  return apiFetch<StatusResponse>("/status")
}

// ---- Digest settings (operator control) -----------------------------------

// getDigestSettings fetches the gated digest settings resource (DGST-01,
// DGST-16). Routed through apiFetch like every other wrapper -- no
// per-wrapper auth code needed.
export async function getDigestSettings(): Promise<NotificationSettings> {
  return apiFetch<NotificationSettings>("/settings/notifications")
}

// updateDigestSettings always sends both fields (D-02 instant-apply,
// full-object semantics matching the Go DTO) -- there is no partial-update
// variant of this endpoint, unlike updateWatchlistPreferences.
export async function updateDigestSettings(params: {
  digestEnabled: boolean
  digestCadence: "daily" | "weekly"
}): Promise<NotificationSettings> {
  return apiFetch<NotificationSettings>("/settings/notifications", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      digest_enabled: params.digestEnabled,
      digest_cadence: params.digestCadence,
    }),
  })
}
