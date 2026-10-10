import type { HistoryFiltersValue } from "~/components/history/HistoryFilters"
import type { EventItem } from "~/lib/api"

export const HISTORY_EVENT_TYPES: readonly EventItem["event_type"][] = [
  "new_release",
  "guest_feature",
  "deluxe_change",
]

const ID_PATTERN = /^[1-9]\d*$/

function parseId(raw: string | null): number | null {
  if (raw === null || !ID_PATTERN.test(raw)) return null
  const id = Number(raw)
  return Number.isSafeInteger(id) ? id : null
}

// D-13: anything malformed parses to null ("all") rather than throwing.
export function parseHistoryParams(
  params: URLSearchParams
): HistoryFiltersValue {
  const type = params.get("type")
  return {
    artistId: parseId(params.get("artist")),
    eventType: HISTORY_EVENT_TYPES.find((t) => t === type) ?? null,
    tagId: parseId(params.get("tag")),
  }
}

// Rewrites only the History keys from the parse; foreign keys are untouched.
export function canonicalHistoryParams(
  params: URLSearchParams
): URLSearchParams {
  const parsed = parseHistoryParams(params)
  const next = new URLSearchParams(params)
  const entries: [string, string | null][] = [
    ["artist", parsed.artistId === null ? null : String(parsed.artistId)],
    ["type", parsed.eventType],
    ["tag", parsed.tagId === null ? null : String(parsed.tagId)],
  ]
  for (const [key, value] of entries) {
    if (value === null) next.delete(key)
    else next.set(key, value)
  }
  return next
}
