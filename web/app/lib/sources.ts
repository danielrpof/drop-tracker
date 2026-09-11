// sources.ts is the single source of truth for the frontend's
// per-search-source business rules, otherwise re-derived independently at
// each call site. Three rules live here:
//
//   1. isAddableSource -- only a MusicBrainz-sourced search result can be
//      added to the watchlist, because a Deezer numeric catalog id has no
//      relation to a real mbid (POST /watchlist treats mbid as the
//      artist's canonical identity with no format validation --
//      internal/httpserver/watchlist.go). Used by
//      SearchResultsColumns.tsx's `canAdd` and watchlist.tsx's
//      handleAddSearchResult guard.
//   2. identityField -- which WatchlistEntry field identifies an existing
//      row for a given source's search result (mbid for MusicBrainz,
//      deezer_id for Deezer). Used by SearchResultsColumns.tsx's
//      `alreadyWatching` cross-reference.
//   3. sourceDisplayName / SOURCE_ORDER -- the real product capitalization
//      and fixed panel order for the System view (Phase 19), since naive
//      title-casing the "musicbrainz" key yields the wrong brand spelling.

export type IdentityField = "mbid" | "deezer_id"

export function isAddableSource(sourceName: string): boolean {
  return sourceName === "musicbrainz"
}

export function identityField(sourceName: string): IdentityField {
  return sourceName === "deezer" ? "deezer_id" : "mbid"
}

const SOURCE_DISPLAY_NAMES: Record<string, string> = {
  musicbrainz: "MusicBrainz",
  deezer: "Deezer",
}

export function sourceDisplayName(sourceName: string): string {
  return SOURCE_DISPLAY_NAMES[sourceName] ?? sourceName
}

export const SOURCE_ORDER = ["musicbrainz", "deezer"] as const
