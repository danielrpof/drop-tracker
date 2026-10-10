import type { WatchlistEntry } from "~/lib/api"

// makeEntry builds a WatchlistEntry with neutral defaults; overrides win.
export function makeEntry(
  overrides: Partial<WatchlistEntry> & Pick<WatchlistEntry, "id" | "name">
): WatchlistEntry {
  return {
    artist_id: overrides.id + 1000,
    mbid: `mbid-${overrides.id}`,
    deezer_id: null,
    disambiguation: null,
    image_url: null,
    release_types: ["album", "single", "ep", "deluxe"],
    muted_event_types: [],
    tags: [],
    note: null,
    latest_release_date: null,
    next_release_date: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  }
}
