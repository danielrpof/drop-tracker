import { Loader2, TagPlus, TagX, Trash2, X } from "lucide-react"
import { useEffect, useRef, useState, type RefObject } from "react"

import { Button } from "~/components/ui/button"
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "~/components/ui/combobox"
import { Separator } from "~/components/ui/separator"
import { BulkPreferencesPopover } from "~/components/watchlist/BulkPreferencesPopover"
import {
  DISMISS_REASONS,
  TagCombobox,
} from "~/components/watchlist/TagCombobox"
import type {
  BulkAddTagOutcome,
  BulkOutcome,
  BulkRemoveTagOutcome,
  PreferencesDelta,
  TagRef,
  WatchlistEntry,
} from "~/lib/api"
import {
  removableTags,
  tagsOnEvery,
  type RemovableTag,
} from "~/lib/bulkSelection"
import { useBarClearance } from "~/lib/useBarClearance"
import { useTagVocabulary } from "~/lib/useTagVocabulary"

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

// Picks one tag already carried by the selection; cannot create (D-12).
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

// Sticky selection bar (D-10). The route hosts the remove dialog so it
// outlives the bar's unmount.
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
  const [mode, setMode] = useState<"idle" | "add-tag" | "remove-tag">("idle")
  const [pending, setPending] = useState(false)
  const [prefsOpen, setPrefsOpen] = useState(false)
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

  useBarClearance(wrapperRef)

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
            <BulkPreferencesPopover
              selected={selected}
              disabled={inFlight}
              onOpenChange={setPrefsOpen}
              onApply={onApplyPreferences}
              onSucceeded={onSucceeded}
            />
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
