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

export type SortKey =
  | "name-asc"
  | "name-desc"
  | "added-desc"
  | "added-asc"
  | "latest-desc"
  | "latest-asc"

export const DEFAULT_SORT: SortKey = "name-asc"

export const SORT_OPTIONS: readonly { value: SortKey; label: string }[] = [
  { value: "name-asc", label: "Name (A–Z)" },
  { value: "name-desc", label: "Name (Z–A)" },
  { value: "added-desc", label: "Date added (newest)" },
  { value: "added-asc", label: "Date added (oldest)" },
  { value: "latest-desc", label: "Latest release (newest)" },
  { value: "latest-asc", label: "Latest release (oldest)" },
]

function isSortKey(v: string | null): v is SortKey {
  return SORT_OPTIONS.some((o) => o.value === v)
}

export interface WatchlistView {
  q: string
  sort: SortKey
}

export function parseWatchlistParams(params: URLSearchParams): WatchlistView {
  const sort = params.get("sort")
  return {
    q: params.get("q") ?? "",
    sort: isSortKey(sort) ? sort : DEFAULT_SORT,
  }
}

// Rewrites the keys this view owns to their canonical form (D-13); other
// keys pass through untouched.
export function canonicalWatchlistParams(
  params: URLSearchParams
): URLSearchParams {
  const next = new URLSearchParams(params)
  const { sort } = parseWatchlistParams(params)
  if (sort === DEFAULT_SORT) next.delete("sort")
  else next.set("sort", sort)
  return next
}

const nameCollator = new Intl.Collator(undefined, { sensitivity: "base" })

type Comparator = (a: WatchlistEntry, b: WatchlistEntry) => number

const byId: Comparator = (a, b) => a.artist_id - b.artist_id

// Latest release compares raw text so "2024" sorts before "2024-05" (D-15);
// a null always lands last, in both directions.
function latestCompare(dir: 1 | -1): Comparator {
  return (a, b) => {
    const x = a.latest_release_date
    const y = b.latest_release_date
    if (x === null && y === null) return byId(a, b)
    if (x === null) return 1
    if (y === null) return -1
    if (x === y) return byId(a, b)
    return (x < y ? -1 : 1) * dir || byId(a, b)
  }
}

// Each direction is its own comparator: reversing an ascending sort would
// flip id ties and float nulls to the top (D-16).
export function compareForSort(sort: SortKey): Comparator {
  switch (sort) {
    case "name-asc":
      return (a, b) => nameCollator.compare(a.name, b.name) || byId(a, b)
    case "name-desc":
      return (a, b) => -nameCollator.compare(a.name, b.name) || byId(a, b)
    case "added-asc":
      return (a, b) =>
        Date.parse(a.created_at) - Date.parse(b.created_at) || byId(a, b)
    case "added-desc":
      return (a, b) =>
        Date.parse(b.created_at) - Date.parse(a.created_at) || byId(a, b)
    case "latest-asc":
      return latestCompare(1)
    case "latest-desc":
      return latestCompare(-1)
  }
}

// An entry is visible when it matches every active axis or is sticky (G4).
export function selectVisible(
  entries: WatchlistEntry[],
  view: WatchlistView,
  stickyIds: ReadonlySet<number>
): WatchlistEntry[] {
  return entries
    .filter(
      (entry) => stickyIds.has(entry.id) || matchesName(entry.name, view.q)
    )
    .sort(compareForSort(view.sort))
}

export function isFiltered(view: WatchlistView): boolean {
  return view.q.trim() !== ""
}

// What Clear filters deletes; sort is deliberately absent (D-10).
export const CLEAR_FILTER_KEYS = ["q", "tags", "muted", "custom"] as const

export function formatArtistCount(n: number, m: number): string {
  return `${n} of ${m} artist${m === 1 ? "" : "s"}`
}
