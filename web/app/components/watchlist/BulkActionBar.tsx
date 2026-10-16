import { Trash2, X } from "lucide-react"
import { useLayoutEffect, useRef, type RefObject } from "react"

import { Button } from "~/components/ui/button"
import { Separator } from "~/components/ui/separator"
import type { WatchlistEntry } from "~/lib/api"

export interface BulkActionBarProps {
  selected: WatchlistEntry[]
  inFlight: boolean
  onRemove: () => void
  onClear: () => void
  removeButtonRef: RefObject<HTMLButtonElement | null>
}

const ACTION_CLASS = "size-11 px-0 sm:h-9 sm:w-auto sm:px-3"

// Sticky selection bar (D-10). It only opens the remove dialog; the route
// hosts it so the dialog outlives the bar's unmount (U1).
export function BulkActionBar({
  selected,
  inFlight,
  onRemove,
  onClear,
  removeButtonRef,
}: BulkActionBarProps) {
  const wrapperRef = useRef<HTMLDivElement>(null)

  // Publishes the bar's height so the page, toasts and scroll padding clear it (U2).
  useLayoutEffect(() => {
    const wrapper = wrapperRef.current
    if (!wrapper) return
    const root = document.documentElement
    const measure = () => {
      const inset = parseFloat(getComputedStyle(wrapper).bottom)
      const bottom = Number.isFinite(inset) ? inset : 0
      const height = wrapper.getBoundingClientRect().height
      root.style.setProperty("--bulk-bar-clearance", `${height + bottom + 8}px`)
    }
    measure()
    const observer =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(measure)
    observer?.observe(wrapper)
    return () => {
      observer?.disconnect()
      root.style.removeProperty("--bulk-bar-clearance")
    }
  }, [])

  return (
    <div
      ref={wrapperRef}
      className="pointer-events-none fixed inset-x-0 bottom-2 z-40 flex justify-center px-2 sm:bottom-6 sm:px-8"
    >
      <div
        role="region"
        aria-label="Selection actions"
        // 26-08/26-10 guard this so Esc in an inline picker or popover closes only that layer.
        onKeyDown={(e) => {
          if (e.key === "Escape") onClear()
        }}
        className="pointer-events-auto flex w-full flex-wrap items-center gap-1 rounded-2xl bg-popover p-2 text-popover-foreground shadow-2xl ring-1 ring-foreground/10 motion-safe:animate-in motion-safe:duration-150 motion-safe:fade-in-0 motion-safe:slide-in-from-bottom-4 sm:w-auto sm:max-w-full sm:flex-nowrap sm:gap-2"
      >
        <p className="min-w-0 flex-1 px-0 text-label text-foreground tabular-nums sm:flex-none sm:px-2">
          {selected.length} selected
        </p>
        <Separator orientation="vertical" className="hidden h-6 sm:block" />
        <Button
          variant="ghost"
          ref={removeButtonRef}
          title="Remove"
          className={`${ACTION_CLASS} hover:text-destructive`}
          disabled={inFlight}
          onClick={onRemove}
        >
          <Trash2 aria-hidden="true" />
          <span className="sr-only sm:not-sr-only">Remove</span>
        </Button>
        <Separator orientation="vertical" className="hidden h-6 sm:block" />
        <Button
          variant="ghost"
          aria-label="Clear selection"
          title="Clear selection"
          className={ACTION_CLASS}
          disabled={inFlight}
          onClick={onClear}
        >
          <X aria-hidden="true" />
          <span className="sr-only sm:not-sr-only">Clear</span>
        </Button>
      </div>
    </div>
  )
}
