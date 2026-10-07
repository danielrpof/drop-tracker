import type { TagRef } from "~/lib/api"

// normalizeTagKey mirrors the server's identity rule (D-31): trim, collapse
// any interior whitespace run to one space, fold NFC-decomposed characters
// to their precomposed form, then lower-case. Applying .normalize("NFC")
// before .toLowerCase() is what keeps the client's exact-match pinning in
// agreement with Postgres's collation-driven lower() (24-01 confirmed
// en_US.utf8 live).
export function normalizeTagKey(s: string): string {
  return s.trim().replace(/\s+/g, " ").normalize("NFC").toLowerCase()
}

export type TagSuggestion =
  | { kind: "existing"; tag: TagRef }
  | { kind: "create"; name: string }

export type SuggestionState =
  | { state: "already-on"; name: string }
  | { state: "empty-vocabulary" }
  | { state: "items"; items: TagSuggestion[] }

// buildTagSuggestions is the pure ordering rule behind the "+ tag"
// combobox (UI-SPEC "Tag autocomplete", D-13, D-30). vocabulary is the
// whole vocabulary or null when not yet loaded/failed; onArtist and
// pendingNames are subtracted from candidates and checked for an
// already-on match (D-24: a pending chip counts as "on" before the
// server confirms it).
export function buildTagSuggestions({
  query,
  vocabulary,
  onArtist,
  pendingNames,
}: {
  query: string
  vocabulary: TagRef[] | null
  onArtist: TagRef[]
  pendingNames: string[]
}): SuggestionState {
  const q = query.trim().replace(/\s+/g, " ")
  const qKey = normalizeTagKey(q)

  if (qKey) {
    const onArtistMatch = onArtist.find(
      (tag) => normalizeTagKey(tag.name) === qKey
    )
    if (onArtistMatch) {
      return { state: "already-on", name: onArtistMatch.name }
    }
    const pendingMatch = pendingNames.find(
      (name) => normalizeTagKey(name) === qKey
    )
    if (pendingMatch !== undefined) {
      return { state: "already-on", name: pendingMatch }
    }
  }

  const onArtistKeys = new Set(onArtist.map((tag) => normalizeTagKey(tag.name)))
  const candidates = (vocabulary ?? []).filter(
    (tag) => !onArtistKeys.has(normalizeTagKey(tag.name))
  )
  const exactMatch = candidates.find(
    (tag) => normalizeTagKey(tag.name) === qKey
  )

  const items: TagSuggestion[] = []
  if (qKey && exactMatch) {
    items.push({ kind: "existing", tag: exactMatch })
  } else if (q) {
    items.push({ kind: "create", name: q })
  }

  const partial = candidates
    .filter(
      (tag) => tag !== exactMatch && normalizeTagKey(tag.name).includes(qKey)
    )
    .sort((a, b) => {
      const cmp = a.name.localeCompare(b.name, undefined, {
        sensitivity: "base",
      })
      return cmp !== 0 ? cmp : a.id - b.id
    })
    .map((tag): TagSuggestion => ({ kind: "existing", tag }))
  items.push(...partial)

  if (vocabulary !== null && vocabulary.length === 0 && !q) {
    return { state: "empty-vocabulary" }
  }

  return { state: "items", items }
}
