import { FilterX, ListFilter, X } from "lucide-react"

import { Button } from "~/components/ui/button"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "~/components/ui/input-group"

export interface WatchlistToolbarProps {
  query: string
  onQueryChange(next: string): void
  filterInputRef: React.RefObject<HTMLInputElement | null>
  showClear: boolean
  onClearFilters(): void
}

export function WatchlistToolbar({
  query,
  onQueryChange,
  filterInputRef,
  showClear,
  onClearFilters,
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
