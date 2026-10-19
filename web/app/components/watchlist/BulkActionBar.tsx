import {
  Loader2,
  SlidersHorizontal,
  TagPlus,
  TagX,
  Trash2,
  X,
} from "lucide-react"
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type RefObject,
} from "react"

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
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "~/components/ui/combobox"
import { Separator } from "~/components/ui/separator"
import {
  DISMISS_REASONS,
  TagCombobox,
} from "~/components/watchlist/TagCombobox"
import type {
  BulkAddTagOutcome,
  BulkAxisDelta,
  BulkOutcome,
  BulkRemoveTagOutcome,
  PreferencesDelta,
  TagRef,
  WatchlistEntry,
} from "~/lib/api"
import {
  buildAxisDelta,
  cycleTriState,
  initialTriStates,
  removableTags,
  tagsOnEvery,
  type RemovableTag,
  type PreferenceAxis,
  type TriState,
} from "~/lib/bulkSelection"
import { useTagVocabulary } from "~/lib/useTagVocabulary"
import {
  EVENT_TYPE_ORDER,
  MUTE_OPTIONS,
  RELEASE_OPTIONS,
  RELEASE_TYPE_ORDER,
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

export interface BulkActionBarProps {
  selected: WatchlistEntry[]
  inFlight: boolean
  onRemove: () => void
  onAddTag: (name: string) => Promise<BulkAddTagOutcome | null>
  onRemoveTag: (tag: TagRef) => Promise<BulkRemoveTagOutcome | null>
  onApplyPreferences: (delta: PreferencesDelta) => Promise<BulkOutcome | null>
  onSucceeded: () => void
  onClear: () => void
  removeButtonRef: RefObject<HTMLButtonElement | null>
}

const ACTION_CLASS = "size-11 px-0 sm:h-9 sm:w-auto sm:px-3"

// Single-value, non-creatable picker over the tags the selection carries (D-12).
function RemoveTagPicker({
  selected,
  disabled,
  onCommit,
  onClose,
}: {
  selected: WatchlistEntry[]
  disabled: boolean
  onCommit: (tag: TagRef) => void
  onClose: (reason: "escape" | "blur") => void
}) {
  const [query, setQuery] = useState("")
  const [popupOpen, setPopupOpen] = useState(true)
  const options = removableTags(selected)
  const needle = query.trim().toLowerCase()
  const items = options.filter((t) => t.name.toLowerCase().includes(needle))
  const total = selected.length

  return (
    <Combobox<RemovableTag>
      items={items}
      filter={null}
      value={null}
      onValueChange={(tag) => {
        if (tag) onCommit({ id: tag.id, name: tag.name })
      }}
      inputValue={query}
      onInputValueChange={setQuery}
      itemToStringLabel={(t) => t.name}
      isItemEqualToValue={(a, b) => a.id === b.id}
      autoHighlight
      open={popupOpen}
      onOpenChange={(open, details) => {
        if (open) {
          setPopupOpen(true)
          return
        }
        if (DISMISS_REASONS.has(details.reason)) {
          setPopupOpen(false)
          onClose(details.reason === "escape-key" ? "escape" : "blur")
        }
      }}
    >
      <ComboboxInput
        placeholder="Tag to remove"
        aria-label={`Remove a tag from ${total} selected artist${total === 1 ? "" : "s"}`}
        className="h-9 w-full sm:w-60"
        disabled={disabled}
        showTrigger={false}
        autoFocus
      />
      <ComboboxContent>
        <ComboboxList className="max-h-72 overflow-y-auto">
          {items.map((tag) => (
            <ComboboxItem key={tag.id} value={tag}>
              <span className="min-w-0 truncate">{tag.name}</span>
              <span className="ml-auto shrink-0 text-muted-foreground tabular-nums">
                {tag.count} of {total}
              </span>
            </ComboboxItem>
          ))}
        </ComboboxList>
        <ComboboxEmpty>
          {options.length === 0
            ? "None of the selected artists have tags."
            : `No selected artist has a tag matching “${query}”`}
        </ComboboxEmpty>
      </ComboboxContent>
    </Combobox>
  )
}

// Sticky selection bar (D-10). It only opens the remove dialog; the route
// hosts it so the dialog outlives the bar's unmount (U1).
export function BulkActionBar({
  selected,
  inFlight,
  onRemove,
  onAddTag,
  onRemoveTag,
  onApplyPreferences,
  onSucceeded,
  onClear,
  removeButtonRef,
}: BulkActionBarProps) {
  const wrapperRef = useRef<HTMLDivElement>(null)
  const addTagButtonRef = useRef<HTMLButtonElement>(null)
  const removeTagButtonRef = useRef<HTMLButtonElement>(null)
  const focusAfterIdleRef = useRef<"add-tag" | "remove-tag" | null>(null)
  const pendingRef = useRef(false)
  const albumRef = useRef<HTMLElement | null>(null)
  const applyRef = useRef<HTMLButtonElement>(null)
  const focusApplyRef = useRef(false)
  const [mode, setMode] = useState<"idle" | "add-tag" | "remove-tag">("idle")
  const [pending, setPending] = useState(false)
  const [prefsOpen, setPrefsOpen] = useState(false)
  const [prefsPending, setPrefsPending] = useState(false)
  const [prefsInitial, setPrefsInitial] = useState<PrefStates>(() =>
    computePrefStates(selected)
  )
  const [prefsCurrent, setPrefsCurrent] = useState<PrefStates>(prefsInitial)
  // The bar renders inside TagVocabularyProvider, so the vocabulary lives here (D-12).
  const { vocabulary, ensureLoaded, remember } = useTagVocabulary()
  const count = selected.length

  // The inline buttons remount when the mode returns to idle, so focus after commit.
  useEffect(() => {
    if (mode !== "idle" || !focusAfterIdleRef.current) return
    const target =
      focusAfterIdleRef.current === "add-tag"
        ? addTagButtonRef.current
        : removeTagButtonRef.current
    focusAfterIdleRef.current = null
    target?.focus()
  }, [mode])

  // A failed apply disables then re-enables Apply, so refocus it after commit.
  useEffect(() => {
    if (!prefsPending && focusApplyRef.current) {
      focusApplyRef.current = false
      applyRef.current?.focus()
    }
  }, [prefsPending])

  function handlePrefsOpenChange(open: boolean) {
    if (prefsPending) return
    if (open) {
      const initial = computePrefStates(selected)
      setPrefsInitial(initial)
      setPrefsCurrent(initial)
    }
    setPrefsOpen(open)
  }

  function cyclePref(axis: PreferenceAxis, type: string) {
    setPrefsCurrent((prev) => ({
      ...prev,
      [axis]: {
        ...prev[axis],
        [type]: cycleTriState(prefsInitial[axis][type], prev[axis][type]),
      },
    }))
  }

  const prefsDelta: PreferencesDelta = {
    releaseTypes: buildAxisDelta(
      RELEASE_TYPE_ORDER,
      prefsInitial.release_types,
      prefsCurrent.release_types
    ),
    mutedEventTypes: buildAxisDelta(
      EVENT_TYPE_ORDER,
      prefsInitial.muted_event_types,
      prefsCurrent.muted_event_types
    ),
  }
  const prefsChanged =
    prefsDelta.releaseTypes.set.length +
      prefsDelta.releaseTypes.unset.length +
      prefsDelta.mutedEventTypes.set.length +
      prefsDelta.mutedEventTypes.unset.length >
    0

  async function applyPreferences() {
    if (prefsPending || !prefsChanged) return
    setPrefsPending(true)
    const outcome = await onApplyPreferences(prefsDelta)
    setPrefsPending(false)
    if (!outcome) {
      focusApplyRef.current = true
      return
    }
    setPrefsOpen(false)
    onSucceeded()
  }

  function renderPrefOption(
    axis: PreferenceAxis,
    option: { type: string; label: string; aria: string },
    ref?: RefObject<HTMLElement | null>
  ) {
    const state = prefsCurrent[axis][option.type]
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
          disabled={prefsPending}
          onCheckedChange={() => cyclePref(axis, option.type)}
        />
        {option.label}
      </label>
    )
  }

  function exitInline(focus: "add-tag" | "remove-tag" | null) {
    focusAfterIdleRef.current = focus
    setMode("idle")
  }

  function openAddTag() {
    ensureLoaded()
    setMode("add-tag")
  }

  async function commitAddTag(name: string) {
    if (pendingRef.current) return
    pendingRef.current = true
    setPending(true)
    const outcome = await onAddTag(name)
    pendingRef.current = false
    setPending(false)
    if (!outcome) {
      exitInline("add-tag")
      return
    }
    if (outcome.applied.length > 0) remember(outcome.tag)
    exitInline(null)
    onSucceeded()
  }

  async function commitRemoveTag(tag: TagRef) {
    if (pendingRef.current) return
    pendingRef.current = true
    setPending(true)
    const outcome = await onRemoveTag(tag)
    pendingRef.current = false
    setPending(false)
    if (!outcome) {
      exitInline("remove-tag")
      return
    }
    exitInline(null)
    onSucceeded()
  }

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
        // Esc in an inline picker or the popover closes only that layer.
        onKeyDown={(e) => {
          if (e.key === "Escape" && mode === "idle" && !prefsOpen) onClear()
        }}
        className="pointer-events-auto flex w-full flex-wrap items-center gap-1 rounded-2xl bg-popover p-2 text-popover-foreground shadow-2xl ring-1 ring-foreground/10 motion-safe:animate-in motion-safe:duration-150 motion-safe:fade-in-0 motion-safe:slide-in-from-bottom-4 sm:w-auto sm:max-w-full sm:flex-nowrap sm:gap-2"
      >
        <p className="min-w-0 flex-1 px-0 text-label text-foreground tabular-nums sm:flex-none sm:px-2">
          {selected.length} selected
        </p>
        <Separator orientation="vertical" className="hidden h-6 sm:block" />
        {mode === "idle" ? (
          <>
            <Button
              variant="ghost"
              ref={addTagButtonRef}
              title="Add tag"
              className={ACTION_CLASS}
              disabled={inFlight || prefsOpen}
              onClick={openAddTag}
            >
              <TagPlus aria-hidden="true" />
              <span className="sr-only sm:not-sr-only">Add tag</span>
            </Button>
            <Button
              variant="ghost"
              ref={removeTagButtonRef}
              title="Remove tag"
              className={ACTION_CLASS}
              disabled={inFlight || prefsOpen}
              onClick={() => setMode("remove-tag")}
            >
              <TagX aria-hidden="true" />
              <span className="sr-only sm:not-sr-only">Remove tag</span>
            </Button>
            <Popover open={prefsOpen} onOpenChange={handlePrefsOpenChange}>
              <PopoverTrigger
                render={
                  <Button
                    variant="ghost"
                    title="Preferences"
                    className={ACTION_CLASS}
                    disabled={inFlight}
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
                    {
                      "Only the types you change are updated. A dash means the selected artists differ."
                    }
                  </PopoverDescription>
                </PopoverHeader>
                <fieldset className="flex flex-col gap-1">
                  <legend className="text-label text-muted-foreground">
                    Release types
                  </legend>
                  <div className="grid grid-cols-2 gap-x-2">
                    {RELEASE_OPTIONS.map((o) =>
                      renderPrefOption(
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
                    {MUTE_OPTIONS.map((o) =>
                      renderPrefOption("muted_event_types", o)
                    )}
                  </div>
                </fieldset>
                <div className="flex justify-end gap-2">
                  <Button
                    variant="ghost"
                    disabled={prefsPending}
                    onClick={() => handlePrefsOpenChange(false)}
                  >
                    Cancel
                  </Button>
                  <Button
                    ref={applyRef}
                    variant="default"
                    disabled={!prefsChanged || prefsPending}
                    onClick={applyPreferences}
                  >
                    {prefsPending ? "Applying…" : "Apply"}
                  </Button>
                </div>
              </PopoverContent>
            </Popover>
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
          </>
        ) : (
          <div className="order-last flex basis-full items-center gap-1 sm:order-none sm:basis-auto">
            <div className="min-w-0 flex-1 sm:flex-none">
              {mode === "add-tag" ? (
                <TagCombobox
                  inputLabel={`Add tag to ${count} selected artist${count === 1 ? "" : "s"}`}
                  alreadyOnMessage={(name) =>
                    `“${name}” is already on every selected artist`
                  }
                  vocabulary={vocabulary}
                  onArtist={tagsOnEvery(selected)}
                  pendingNames={[]}
                  disabled={pending || inFlight}
                  inputClassName="h-9 w-full max-w-none sm:w-60"
                  onCommit={commitAddTag}
                  onClose={(reason) => {
                    if (pendingRef.current) return
                    exitInline(reason === "escape" ? "add-tag" : null)
                  }}
                />
              ) : (
                <RemoveTagPicker
                  selected={selected}
                  disabled={pending || inFlight}
                  onCommit={commitRemoveTag}
                  onClose={(reason) => {
                    if (pendingRef.current) return
                    exitInline(reason === "escape" ? "remove-tag" : null)
                  }}
                />
              )}
            </div>
            {pending ? (
              <span className="flex items-center gap-1 px-2 text-label text-muted-foreground">
                <Loader2
                  aria-hidden="true"
                  className="size-4 motion-safe:animate-spin"
                />
                {mode === "add-tag" ? "Adding…" : "Removing tag…"}
              </span>
            ) : (
              <Button
                variant="ghost"
                className="h-11 sm:h-9"
                disabled={inFlight}
                // Pointer-down too: the input's blur would unmount Cancel before its click.
                onPointerDown={() => exitInline(mode)}
                onClick={() => exitInline(mode)}
              >
                Cancel
              </Button>
            )}
          </div>
        )}
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
