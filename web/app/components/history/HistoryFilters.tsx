import { useEffect, useId, useRef, useState } from "react"

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

// D-13: a native `<select>` was tried first and fixed with a scoped
// `[color-scheme:dark]` token (see commit 8844812), but Chromium on Windows
// renders the native popup via the OS combobox control, which only themes
// the hover/selected row -- the idle row background stays light-mode white
// while the text stays the app's near-white foreground, so every
// non-hovered option is illegible. No CSS-only fix can reach that OS-owned
// surface, so both controls are hand-rolled DOM-rendered comboboxes instead
// (never a Radix/shadcn Select import -- CONTEXT.md/RESEARCH.md's
// no-component-library constraint still holds; only the "native select is
// fine" assumption was wrong).
const triggerClassName =
  "flex h-9 min-w-40 items-center justify-between gap-2 rounded-md border border-input bg-input/30 px-3 text-body text-foreground outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"

const optionClassName =
  "cursor-pointer px-3 py-1.5 text-body text-popover-foreground data-[highlighted=true]:bg-accent"

interface ComboboxOption<TValue extends string> {
  value: TValue | ""
  label: string
}

interface ComboboxProps<TValue extends string> {
  label: string
  options: ComboboxOption<TValue>[]
  value: TValue | null
  onSelect: (value: TValue | null) => void
}

// Combobox is a small hand-rolled dropdown shared by the Artist and Event
// type controls (D-06, D-13). It renders its own DOM-based listbox instead
// of relying on the browser's native `<option>` popup, so its styling is
// fully controlled by app CSS -- see the module comment above for why.
function Combobox<TValue extends string>({
  label,
  options,
  value,
  onSelect,
}: ComboboxProps<TValue>) {
  const [open, setOpen] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(0)
  const containerRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listboxId = useId()

  const selectedIndex = options.findIndex((opt) => (opt.value === "" ? value === null : opt.value === value))
  const selectedOption = options[selectedIndex === -1 ? 0 : selectedIndex]

  useEffect(() => {
    if (!open) return

    function handlePointerDown(e: MouseEvent) {
      if (!containerRef.current?.contains(e.target as Node)) {
        setOpen(false)
      }
    }

    document.addEventListener("mousedown", handlePointerDown)
    return () => document.removeEventListener("mousedown", handlePointerDown)
  }, [open])

  function openAt(index: number) {
    setHighlightedIndex(index < 0 ? 0 : index)
    setOpen(true)
  }

  function commitSelection(index: number) {
    const option = options[index]
    if (!option) return
    onSelect(option.value === "" ? null : option.value)
    setOpen(false)
    triggerRef.current?.focus()
  }

  function handleTriggerKeyDown(e: React.KeyboardEvent<HTMLButtonElement>) {
    if (!open) {
      if (e.key === "ArrowDown" || e.key === "ArrowUp" || e.key === "Enter" || e.key === " ") {
        e.preventDefault()
        openAt(selectedIndex === -1 ? 0 : selectedIndex)
      }
      return
    }

    if (e.key === "ArrowDown") {
      e.preventDefault()
      setHighlightedIndex((i) => Math.min(i + 1, options.length - 1))
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      setHighlightedIndex((i) => Math.max(i - 1, 0))
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault()
      commitSelection(highlightedIndex)
    } else if (e.key === "Escape") {
      e.preventDefault()
      setOpen(false)
      triggerRef.current?.focus()
    }
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={triggerRef}
        type="button"
        role="combobox"
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-controls={listboxId}
        aria-label={label}
        className={triggerClassName}
        onClick={() => (open ? setOpen(false) : openAt(selectedIndex === -1 ? 0 : selectedIndex))}
        onKeyDown={handleTriggerKeyDown}
      >
        <span>{selectedOption?.label}</span>
        <span aria-hidden="true" className="text-muted-foreground">
          {"▾"}
        </span>
      </button>

      {open && (
        <ul
          id={listboxId}
          role="listbox"
          aria-label={label}
          className="absolute top-full left-0 z-50 mt-1 min-w-full overflow-hidden rounded-md border border-input bg-popover text-popover-foreground shadow-md"
        >
          {options.map((opt, index) => (
            <li
              key={opt.value === "" ? "__all__" : opt.value}
              role="option"
              aria-selected={opt.value === "" ? value === null : opt.value === value}
              data-highlighted={index === highlightedIndex}
              className={optionClassName}
              onMouseEnter={() => setHighlightedIndex(index)}
              onClick={() => commitSelection(index)}
            >
              {opt.label}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

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

  const artistOptions: ComboboxOption<string>[] = [
    { value: "", label: "All artists" },
    ...artists.map((artist) => ({ value: String(artist.artist_id), label: artist.name })),
  ]

  const eventTypeOptions: ComboboxOption<EventItem["event_type"]>[] = [
    { value: "", label: "All event types" },
    ...EVENT_TYPE_OPTIONS,
  ]

  return (
    <div className="flex flex-wrap gap-4">
      <label className="flex flex-col gap-1 text-label text-muted-foreground">
        Artist
        <Combobox
          label="Artist"
          options={artistOptions}
          value={value.artistId === null ? null : String(value.artistId)}
          onSelect={(raw) => onChange({ ...value, artistId: raw === null ? null : Number(raw) })}
        />
      </label>

      <label className="flex flex-col gap-1 text-label text-muted-foreground">
        Event type
        <Combobox
          label="Event type"
          options={eventTypeOptions}
          value={value.eventType}
          onSelect={(raw) => onChange({ ...value, eventType: raw })}
        />
      </label>
    </div>
  )
}
