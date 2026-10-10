import type { WatchlistEntry } from "~/lib/api"

// Pure view-model: never import runtime values from ~/lib/api here, route
// tests automock that module.

const FOLD: Record<string, string> = { $: "s", ø: "o", æ: "ae", ß: "ss" }

// NFD does not decompose ø, æ, ß, so they go through a table (G5).
export function foldName(s: string): string {
  return s
    .normalize("NFD")
    .replace(/\p{M}/gu, "")
    .toLowerCase()
    .replace(/[$øæß]/g, (c) => FOLD[c])
}

export function matchesName(name: string, query: string): boolean {
  const trimmed = query.trim()
  if (trimmed === "") return true
  return foldName(name).includes(foldName(trimmed))
}

export interface WatchlistView {
  q: string
}

export function parseWatchlistParams(params: URLSearchParams): WatchlistView {
  return { q: params.get("q") ?? "" }
}

// An entry is visible when it matches every active axis or is sticky (G4).
export function selectVisible(
  entries: WatchlistEntry[],
  view: WatchlistView,
  stickyIds: ReadonlySet<number>
): WatchlistEntry[] {
  return entries.filter(
    (entry) => stickyIds.has(entry.id) || matchesName(entry.name, view.q)
  )
}

export function formatArtistCount(n: number, m: number): string {
  return `${n} of ${m} artist${m === 1 ? "" : "s"}`
}
