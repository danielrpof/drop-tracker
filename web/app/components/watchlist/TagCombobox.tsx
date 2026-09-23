import { useState } from "react"

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
import { buildTagSuggestions, type TagSuggestion } from "~/lib/tags"

export interface TagComboboxProps {
  entry: WatchlistEntry
  vocabulary: TagRef[] | null
  onArtist: TagRef[]
  pendingNames: string[]
  onCommit: (name: string) => void
  onClose: () => void
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

  const suggestions = buildTagSuggestions({
    query,
    vocabulary,
    onArtist,
    pendingNames,
  })
  const items = suggestions.state === "items" ? suggestions.items : []

  function commit(item: TagSuggestion) {
    onCommit(itemLabel(item))
    setQuery("")
  }

  return (
    <Combobox<TagSuggestion>
      items={items}
      value={null}
      onValueChange={(item) => {
        if (item) commit(item)
      }}
      inputValue={query}
      onInputValueChange={setQuery}
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
          onClose()
        }
      }}
    >
      <ComboboxInput
        maxLength={32}
        placeholder="Tag name"
        aria-label={`Add tag to ${entry.name}`}
        className="h-8 w-full max-w-60 text-label"
        showTrigger={false}
        autoFocus
      />
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
