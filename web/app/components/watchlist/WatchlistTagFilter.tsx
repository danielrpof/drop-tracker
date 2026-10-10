import {
  Combobox,
  ComboboxChip,
  ComboboxChips,
  ComboboxChipsInput,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxItem,
  ComboboxList,
  useComboboxAnchor,
} from "~/components/ui/combobox"
import type { TagFilterOption } from "~/lib/watchlistView"

export interface WatchlistTagFilterProps {
  options: TagFilterOption[]
  selected: TagFilterOption[]
  onChange(ids: number[]): void
}

// Any-of tag filter on the vendored multi-select parts; never creates a tag.
export function WatchlistTagFilter({
  options,
  selected,
  onChange,
}: WatchlistTagFilterProps) {
  const anchor = useComboboxAnchor()

  return (
    <Combobox
      multiple
      items={options}
      value={selected}
      onValueChange={(next) =>
        onChange(next.map((t) => t.id).sort((a, b) => a - b))
      }
      isItemEqualToValue={(a, b) => a.id === b.id}
      itemToStringLabel={(t) => t.name}
    >
      <ComboboxChips ref={anchor}>
        {selected.map((tag) => (
          <ComboboxChip
            key={tag.id}
            className="h-6 max-w-full bg-secondary text-label font-normal text-secondary-foreground"
            removeLabel={`Remove ${tag.name} from tag filter`}
          >
            <span className="min-w-0 truncate" title={tag.name}>
              {tag.name}
            </span>
          </ComboboxChip>
        ))}
        <ComboboxChipsInput
          aria-labelledby="watchlist-tags-label"
          aria-label="Filter by tags, any of"
          placeholder={selected.length === 0 ? "All tags" : ""}
        />
      </ComboboxChips>
      <ComboboxContent anchor={anchor}>
        <ComboboxList>
          {(tag: TagFilterOption) => (
            <ComboboxItem key={tag.id} value={tag}>
              {tag.name}
              <span className="ml-auto text-muted-foreground tabular-nums">
                {tag.count}
              </span>
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}
