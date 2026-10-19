import { SlidersHorizontal } from "lucide-react"
import { useEffect, useRef, useState } from "react"

import { Button } from "~/components/ui/button"
import { Checkbox } from "~/components/ui/checkbox"
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "~/components/ui/popover"
import type { BulkOutcome, PreferencesDelta, WatchlistEntry } from "~/lib/api"
import {
  buildAxisDelta,
  cycleTriState,
  initialTriStates,
  type PreferenceAxis,
  type TriState,
} from "~/lib/bulkSelection"
import {
  EVENT_TYPE_ORDER,
  MUTE_OPTIONS,
  RELEASE_OPTIONS,
  RELEASE_TYPE_ORDER,
  type PreferenceOption,
} from "~/lib/preferenceOptions"

type PrefStates = Record<PreferenceAxis, Record<string, TriState>>

function computePrefStates(selected: WatchlistEntry[]): PrefStates {
  return {
    release_types: initialTriStates(
      selected,
      "release_types",
      RELEASE_TYPE_ORDER
    ),
    muted_event_types: initialTriStates(
      selected,
      "muted_event_types",
      EVENT_TYPE_ORDER
    ),
  }
}

export interface BulkPreferencesPopoverProps {
  selected: WatchlistEntry[]
  disabled: boolean
  onOpenChange: (open: boolean) => void
  // Resolves null on failure, which keeps the popover open.
  onApply: (delta: PreferencesDelta) => Promise<BulkOutcome | null>
  onSucceeded: () => void
}

const TRIGGER_CLASS = "size-11 px-0 sm:h-9 sm:w-auto sm:px-3"

export function BulkPreferencesPopover({
  selected,
  disabled,
  onOpenChange,
  onApply,
  onSucceeded,
}: BulkPreferencesPopoverProps) {
  const albumRef = useRef<HTMLElement | null>(null)
  const applyRef = useRef<HTMLButtonElement>(null)
  const focusApplyRef = useRef(false)
  const [open, setOpen] = useState(false)
  const [pending, setPending] = useState(false)
  const [initial, setInitial] = useState<PrefStates>(() =>
    computePrefStates(selected)
  )
  const [current, setCurrent] = useState<PrefStates>(initial)
  const count = selected.length

  // A failed apply disables then re-enables Apply, so refocus it after commit.
  useEffect(() => {
    if (!pending && focusApplyRef.current) {
      focusApplyRef.current = false
      applyRef.current?.focus()
    }
  }, [pending])

  function changeOpen(next: boolean) {
    if (pending) return
    if (next) {
      const fresh = computePrefStates(selected)
      setInitial(fresh)
      setCurrent(fresh)
    }
    setOpen(next)
    onOpenChange(next)
  }

  function cycle(axis: PreferenceAxis, type: string) {
    setCurrent((prev) => ({
      ...prev,
      [axis]: {
        ...prev[axis],
        [type]: cycleTriState(initial[axis][type], prev[axis][type]),
      },
    }))
  }

  const delta: PreferencesDelta = {
    releaseTypes: buildAxisDelta(
      RELEASE_TYPE_ORDER,
      initial.release_types,
      current.release_types
    ),
    mutedEventTypes: buildAxisDelta(
      EVENT_TYPE_ORDER,
      initial.muted_event_types,
      current.muted_event_types
    ),
  }
  const changed =
    delta.releaseTypes.set.length +
      delta.releaseTypes.unset.length +
      delta.mutedEventTypes.set.length +
      delta.mutedEventTypes.unset.length >
    0

  async function apply() {
    if (pending || !changed) return
    setPending(true)
    const outcome = await onApply(delta)
    setPending(false)
    if (!outcome) {
      focusApplyRef.current = true
      return
    }
    setOpen(false)
    onOpenChange(false)
    onSucceeded()
  }

  function renderOption(
    axis: PreferenceAxis,
    option: PreferenceOption,
    ref?: typeof albumRef
  ) {
    const state = current[axis][option.type]
    return (
      <label
        key={option.type}
        className="flex min-h-11 items-center gap-2 text-label text-foreground"
      >
        <Checkbox
          ref={ref}
          className="border-muted-foreground"
          aria-label={option.aria}
          checked={state === "checked"}
          indeterminate={state === "indeterminate"}
          disabled={pending}
          onCheckedChange={() => cycle(axis, option.type)}
        />
        {option.label}
      </label>
    )
  }

  return (
    <Popover open={open} onOpenChange={changeOpen}>
      <PopoverTrigger
        render={
          <Button
            variant="ghost"
            title="Preferences"
            className={TRIGGER_CLASS}
            disabled={disabled}
          />
        }
      >
        <SlidersHorizontal aria-hidden="true" />
        <span className="sr-only sm:not-sr-only">Preferences</span>
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="center"
        sideOffset={8}
        className="w-80"
        initialFocus={albumRef}
      >
        <PopoverHeader>
          <PopoverTitle>
            Preferences for {count} artist{count === 1 ? "" : "s"}
          </PopoverTitle>
          <PopoverDescription>
            Only the types you change are updated. A dash means the selected
            artists differ.
          </PopoverDescription>
        </PopoverHeader>
        <fieldset className="flex flex-col gap-1">
          <legend className="text-label text-muted-foreground">
            Release types
          </legend>
          <div className="grid grid-cols-2 gap-x-2">
            {RELEASE_OPTIONS.map((o) =>
              renderOption(
                "release_types",
                o,
                o.type === "album" ? albumRef : undefined
              )
            )}
          </div>
        </fieldset>
        <fieldset className="flex flex-col gap-1">
          <legend className="text-label text-muted-foreground">
            Mute alerts
          </legend>
          <div className="grid grid-cols-2 gap-x-2">
            {MUTE_OPTIONS.map((o) => renderOption("muted_event_types", o))}
          </div>
        </fieldset>
        <div className="flex justify-end gap-2">
          <Button
            variant="ghost"
            disabled={pending}
            onClick={() => changeOpen(false)}
          >
            Cancel
          </Button>
          <Button
            ref={applyRef}
            variant="default"
            disabled={!changed || pending}
            onClick={apply}
          >
            {pending ? "Applying…" : "Apply"}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
