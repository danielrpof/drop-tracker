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

export interface TagComboboxProps {
  entry: WatchlistEntry
  vocabulary: TagRef[] | null
  namesOnArtist: Set<string>
  onCommit: (name: string) => void
  onClose: () => void
}

type ComboItem =
  | { kind: "create"; name: string }
  | { kind: "existing"; tag: TagRef }

function itemLabel(item: ComboItem): string {
  return item.kind === "create" ? item.name : item.tag.name
}

// TagCombobox is the "+ tag" editor (TAG-01, D-02): a single-value,
// creatable base-ui Combobox. This task builds items with a plain
// contains-match; plan 24-05 Task 2's buildTagSuggestions replaces this with
// the deterministic pinning/already-on/empty rules (D-13, D-30, D-31).
export function TagCombobox({
  entry,
  vocabulary,
  namesOnArtist,
  onCommit,
  onClose,
}: TagComboboxProps) {
  const [query, setQuery] = useState("")

  const trimmed = query.trim()
  const lowerQuery = trimmed.toLowerCase()
  const candidates = (vocabulary ?? []).filter(
    (tag) =>
      !namesOnArtist.has(tag.name) &&
      tag.name.toLowerCase().includes(lowerQuery)
  )
  const items: ComboItem[] = [
    ...(trimmed ? [{ kind: "create", name: trimmed } as const] : []),
    ...candidates.map((tag) => ({ kind: "existing", tag }) as const),
  ]

  function commit(item: ComboItem) {
    onCommit(itemLabel(item))
    setQuery("")
  }

  return (
    <Combobox<ComboItem>
      items={items}
      value={null}
      onValueChange={(item) => {
        if (item) commit(item)
      }}
      inputValue={query}
      onInputValueChange={setQuery}
      itemToStringLabel={itemLabel}
      autoHighlight
      defaultOpen
      onOpenChange={(open) => {
        if (!open) onClose()
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
        <ComboboxEmpty>Type a name to create a tag</ComboboxEmpty>
      </ComboboxContent>
    </Combobox>
  )
}
