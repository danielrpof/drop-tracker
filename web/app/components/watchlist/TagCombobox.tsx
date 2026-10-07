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
  MAX_TAG_LENGTH,
  TAG_COUNTER_THRESHOLD,
  tagNameLength,
} from "~/lib/limits"
import { buildTagSuggestions, type TagSuggestion } from "~/lib/tags"

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

// DISMISS_REASONS are the base-ui close reasons that mean the user wants the
// editor gone. Reason "none" (Enter with nothing selectable) must not close it.
const DISMISS_REASONS = new Set([
  "escape-key",
  "outside-press",
  "focus-out",
  "input-blur",
])

// TagCombobox is the "+ tag" editor (TAG-01): a creatable base-ui Combobox
// that only renders what buildTagSuggestions returns.
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
  // base-ui echoes the picked label through onInputValueChange right after
  // a commit; suppress that echo so the cleared input stays cleared.
  const suppressEchoRef = useRef<string | null>(null)

  const suggestions = buildTagSuggestions({
    query,
    vocabulary,
    onArtist,
    pendingNames,
  })
  const queryLength = tagNameLength(query)
  const overLimit = queryLength > MAX_TAG_LENGTH
  // An over-limit new name can't be created, so Enter must not commit it;
  // existing tags stay pickable.
  const items =
    suggestions.state === "items"
      ? suggestions.items.filter((i) => !(overLimit && i.kind === "create"))
      : []

  function commit(item: TagSuggestion) {
    const label = itemLabel(item)
    onCommit(label)
    suppressEchoRef.current = label
    setQuery("")
  }

  // Announces each 25/32 threshold crossing once (D-15); query still holds
  // the pre-change value here, so old vs. new length compares synchronously.
  function handleInputValueChange(value: string) {
    if (suppressEchoRef.current !== null) {
      const suppressed = suppressEchoRef.current
      suppressEchoRef.current = null
      if (value === suppressed) return
    }
    const prevLength = tagNameLength(query)
    const nextLength = tagNameLength(value)
    if (prevLength < MAX_TAG_LENGTH && nextLength >= MAX_TAG_LENGTH) {
      setLiveMessage(`Tag name limit reached — ${MAX_TAG_LENGTH} characters.`)
    } else if (
      prevLength < TAG_COUNTER_THRESHOLD &&
      nextLength >= TAG_COUNTER_THRESHOLD
    ) {
      setLiveMessage(
        `${MAX_TAG_LENGTH - TAG_COUNTER_THRESHOLD} characters left.`
      )
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
          placeholder="Tag name"
          aria-label={`Add tag to ${entry.name}`}
          aria-describedby={counterId}
          className="h-8 w-full max-w-60 text-label"
          showTrigger={false}
          autoFocus
        />
        {queryLength >= TAG_COUNTER_THRESHOLD && (
          <span
            id={counterId}
            className={`text-label tabular-nums ${
              overLimit
                ? "text-destructive"
                : queryLength >= MAX_TAG_LENGTH
                  ? "text-foreground"
                  : "text-muted-foreground"
            }`}
          >
            {queryLength}/{MAX_TAG_LENGTH}
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
            : overLimit
              ? `Tags can be at most ${MAX_TAG_LENGTH} characters.`
              : "Type a name to create a tag"}
        </ComboboxEmpty>
      </ComboboxContent>
    </Combobox>
  )
}
