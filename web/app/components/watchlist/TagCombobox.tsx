import { useId, useRef, useState } from "react"

import { Plus } from "lucide-react"

import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "~/components/ui/combobox"
import type { TagRef, WatchlistEntry } from "~/lib/api"
import {
  buildTagSuggestions,
  MAX_TAG_LENGTH,
  type TagSuggestion,
} from "~/lib/tags"

// COUNTER_THRESHOLD is where the "{n}/32" counter and its screen-reader
// crossing announcement first appear (D-15).
const COUNTER_THRESHOLD = 25

export interface TagComboboxProps {
  entry: WatchlistEntry
  vocabulary: TagRef[] | null
  onArtist: TagRef[]
  pendingNames: string[]
  onCommit: (name: string) => void
  // "escape" returns focus to "+ tag" (UI-SPEC focus table row a);
  // "blur" leaves focus wherever the user already moved it.
  onClose: (reason: "escape" | "blur") => void
}

function itemLabel(item: TagSuggestion): string {
  return item.kind === "create" ? item.name : item.tag.name
}

// DISMISS_REASONS is the base-ui close reasons that mean "the user wants
// the editor gone" (Esc, an outside click, focus leaving). base-ui also
// closes its popup with reason "none" when Enter is pressed and nothing is
// selectable (e.g. the already-on state) -- that must NOT close the whole
// "+ tag" editor, only leave the input as-is (UI-SPEC "Enter does
// nothing").
const DISMISS_REASONS = new Set([
  "escape-key",
  "outside-press",
  "focus-out",
  "input-blur",
])

// TagCombobox is the "+ tag" editor (TAG-01, D-02, D-15): a single-value,
// creatable base-ui Combobox. Suggestion ordering, pinning, and the
// already-on/empty-vocabulary states all come from the pure
// buildTagSuggestions helper (D-13, D-30, D-31) -- this component only
// renders what it returns.
export function TagCombobox({
  entry,
  vocabulary,
  onArtist,
  pendingNames,
  onCommit,
  onClose,
}: TagComboboxProps) {
  const [query, setQuery] = useState("")
  const [popupOpen, setPopupOpen] = useState(true)
  const [liveMessage, setLiveMessage] = useState("")
  const counterId = useId()
  // base-ui echoes the just-picked item's label back through
  // onInputValueChange right after a commit (its own "fill" behavior) --
  // one tick after we've already cleared the input. Suppress exactly that
  // echo so the cleared input actually stays cleared.
  const suppressEchoRef = useRef<string | null>(null)

  const suggestions = buildTagSuggestions({
    query,
    vocabulary,
    onArtist,
    pendingNames,
  })
  const items = suggestions.state === "items" ? suggestions.items : []

  function commit(item: TagSuggestion) {
    const label = itemLabel(item)
    onCommit(label)
    suppressEchoRef.current = label
    setQuery("")
  }

  // handleInputValueChange announces the 25/32 threshold crossings exactly
  // once each (D-15, UI-SPEC [R6]) -- query still holds the pre-change
  // value here, so this compares old vs. new length synchronously.
  function handleInputValueChange(value: string) {
    if (suppressEchoRef.current !== null) {
      const suppressed = suppressEchoRef.current
      suppressEchoRef.current = null
      if (value === suppressed) return
    }
    if (query.length < MAX_TAG_LENGTH && value.length >= MAX_TAG_LENGTH) {
      setLiveMessage(`Tag name limit reached — ${MAX_TAG_LENGTH} characters.`)
    } else if (
      query.length < COUNTER_THRESHOLD &&
      value.length >= COUNTER_THRESHOLD
    ) {
      setLiveMessage(`${MAX_TAG_LENGTH - COUNTER_THRESHOLD} characters left.`)
    }
    setQuery(value)
  }

  return (
    <Combobox<TagSuggestion>
      items={items}
      value={null}
      onValueChange={(item) => {
        if (item) commit(item)
      }}
      inputValue={query}
      onInputValueChange={handleInputValueChange}
      itemToStringLabel={itemLabel}
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
      <div className="flex items-center gap-1">
        <ComboboxInput
          maxLength={MAX_TAG_LENGTH}
          placeholder="Tag name"
          aria-label={`Add tag to ${entry.name}`}
          aria-describedby={counterId}
          className="h-8 w-full max-w-60 text-label"
          showTrigger={false}
          autoFocus
        />
        {query.length >= COUNTER_THRESHOLD && (
          <span
            id={counterId}
            className={`text-label tabular-nums ${
              query.length >= MAX_TAG_LENGTH
                ? "text-foreground"
                : "text-muted-foreground"
            }`}
          >
            {query.length}/{MAX_TAG_LENGTH}
          </span>
        )}
      </div>
      <span aria-live="polite" className="sr-only">
        {liveMessage}
      </span>
      <ComboboxContent>
        <ComboboxList>
          {items.map((item) => (
            <ComboboxItem
              key={item.kind === "create" ? "__create__" : item.tag.id}
              value={item}
            >
              {item.kind === "create" ? (
                <>
                  <Plus aria-hidden="true" className="size-3" />
                  Create “{item.name}”
                </>
              ) : (
                item.tag.name
              )}
            </ComboboxItem>
          ))}
        </ComboboxList>
        <ComboboxEmpty>
          {suggestions.state === "already-on"
            ? `“${suggestions.name}” is already on this artist`
            : "Type a name to create a tag"}
        </ComboboxEmpty>
      </ComboboxContent>
    </Combobox>
  )
}
