import { useEffect, useState } from "react"

import { listWatchlist, type EventItem, type WatchlistEntry } from "~/lib/api"

// HistoryFiltersValue carries both filter axes -- null means "all" on that
// axis (HIST-01, D-06). Both axes are independent and compose.
export interface HistoryFiltersValue {
  artistId: number | null
  eventType: EventItem["event_type"] | null
}

export interface HistoryFiltersProps {
  value: HistoryFiltersValue
  onChange: (value: HistoryFiltersValue) => void
}

// EVENT_TYPE_OPTIONS populates the event-type control from the three fixed
// values -- no server round trip needed, unlike the artist control below.
const EVENT_TYPE_OPTIONS: { value: EventItem["event_type"]; label: string }[] = [
  { value: "new_release", label: "New release" },
  { value: "guest_feature", label: "Guest feature" },
  { value: "deluxe_change", label: "Deluxe change" },
]

const selectClassName =
  "h-9 rounded-md border border-input bg-input/30 px-3 text-body text-foreground outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 [color-scheme:dark]"

// HistoryFilters renders the artist and event-type controls (D-06). The
// artist list is populated from listWatchlist() -- called independently
// here since D-03 keeps the History and Watchlist tabs fully decoupled and
// no cross-tab state is shared. Changing either control reports the full
// new value upward; history.tsx resets the accumulated page list and
// refetches from the first page on any change.
export function HistoryFilters({ value, onChange }: HistoryFiltersProps) {
  const [artists, setArtists] = useState<WatchlistEntry[]>([])

  useEffect(() => {
    let cancelled = false
    listWatchlist()
      .then((entries) => {
        if (!cancelled) setArtists(entries)
      })
      .catch(() => {
        // The artist filter is a convenience control, not the primary
        // feed -- a failed watchlist fetch just leaves the "All artists"
        // option as the only choice rather than erroring the whole page.
        if (!cancelled) setArtists([])
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="flex flex-wrap gap-4">
      <label className="flex flex-col gap-1 text-label text-muted-foreground">
        Artist
        <select
          className={selectClassName}
          value={value.artistId ?? ""}
          onChange={(e) => {
            const raw = e.target.value
            onChange({ ...value, artistId: raw === "" ? null : Number(raw) })
          }}
        >
          <option value="">All artists</option>
          {artists.map((artist) => (
            <option key={artist.artist_id} value={artist.artist_id}>
              {artist.name}
            </option>
          ))}
        </select>
      </label>

      <label className="flex flex-col gap-1 text-label text-muted-foreground">
        Event type
        <select
          className={selectClassName}
          value={value.eventType ?? ""}
          onChange={(e) => {
            const raw = e.target.value as EventItem["event_type"] | ""
            onChange({ ...value, eventType: raw === "" ? null : raw })
          }}
        >
          <option value="">All event types</option>
          {EVENT_TYPE_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </label>
    </div>
  )
}
