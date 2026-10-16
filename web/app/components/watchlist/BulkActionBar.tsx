import { Trash2, X } from "lucide-react"
import type { RefObject } from "react"

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
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-2 z-40 flex justify-center px-2 sm:bottom-6 sm:px-8">
      <div
        role="region"
        aria-label="Selection actions"
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
