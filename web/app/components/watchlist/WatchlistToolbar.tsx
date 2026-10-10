import { FilterX, ListFilter, X } from "lucide-react"

import { Button } from "~/components/ui/button"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "~/components/ui/input-group"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select"
import { WatchlistTagFilter } from "~/components/watchlist/WatchlistTagFilter"
import {
  SORT_OPTIONS,
  type SortKey,
  type TagFilterOption,
} from "~/lib/watchlistView"

export interface WatchlistToolbarProps {
  query: string
  onQueryChange(next: string): void
  filterInputRef: React.RefObject<HTMLInputElement | null>
  showClear: boolean
  onClearFilters(): void
  sort: SortKey
  onSortChange(next: SortKey): void
  tagOptions: TagFilterOption[]
  selectedTags: TagFilterOption[]
  onSelectedTagsChange(ids: number[]): void
}

export function WatchlistToolbar({
  query,
  onQueryChange,
  filterInputRef,
  showClear,
  onClearFilters,
  sort,
  onSortChange,
  tagOptions,
  selectedTags,
  onSelectedTagsChange,
}: WatchlistToolbarProps) {
  return (
    <div
      role="group"
      aria-label="Filter and sort artists"
      className="flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-end"
    >
      <label className="flex flex-col gap-1 text-label text-muted-foreground sm:w-64">
        Filter watchlist
        <InputGroup>
          <InputGroupAddon>
            <ListFilter aria-hidden="true" />
          </InputGroupAddon>
          <InputGroupInput
            ref={filterInputRef}
            type="text"
            placeholder="Filter by name…"
            enterKeyHint="search"
            autoComplete="off"
            spellCheck={false}
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape" && query !== "") {
                e.preventDefault()
                onQueryChange("")
              }
            }}
          />
          {query !== "" && (
            <InputGroupAddon align="inline-end">
              <InputGroupButton
                size="icon-xs"
                aria-label="Clear name filter"
                onClick={() => {
                  onQueryChange("")
                  filterInputRef.current?.focus()
                }}
              >
                <X aria-hidden="true" />
              </InputGroupButton>
            </InputGroupAddon>
          )}
        </InputGroup>
      </label>
      <div className="flex flex-col gap-1 sm:w-56">
        <span
          id="watchlist-sort-label"
          className="text-label text-muted-foreground"
        >
          Sort by
        </span>
        <Select value={sort} onValueChange={(v) => onSortChange(v as SortKey)}>
          <SelectTrigger
            aria-labelledby="watchlist-sort-label"
            className="w-full"
          >
            <SelectValue>
              {(value: SortKey) =>
                SORT_OPTIONS.find((o) => o.value === value)?.label
              }
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {SORT_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex flex-col gap-1 sm:max-w-md sm:min-w-60 sm:flex-1">
        <span
          id="watchlist-tags-label"
          className="text-label text-muted-foreground"
        >
          Tags: any of
        </span>
        <WatchlistTagFilter
          options={tagOptions}
          selected={selectedTags}
          onChange={onSelectedTagsChange}
        />
      </div>
      {showClear && (
        <Button
          variant="ghost"
          className="h-9 text-muted-foreground hover:text-foreground"
          onClick={onClearFilters}
        >
          <FilterX aria-hidden="true" />
          Clear filters
        </Button>
      )}
    </div>
  )
}
