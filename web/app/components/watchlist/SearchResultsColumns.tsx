import { Loader2 } from "lucide-react"
import { useState } from "react"

import { CoverArt } from "~/components/common/CoverArt"
import { Button } from "~/components/ui/button"
import type { SearchArtist, SearchResponse, SourceResult, WatchlistEntry } from "~/lib/api"

// SOURCE_LABELS maps a GET /search source key to its display name -- falls
// back to the raw key for any source not yet known here, so a future
// third source still renders (with a less-polished label) instead of
// breaking.
const SOURCE_LABELS: Record<string, string> = {
  musicbrainz: "MusicBrainz",
  deezer: "Deezer",
}

function sourceLabel(sourceName: string): string {
  return SOURCE_LABELS[sourceName] ?? sourceName
}

export interface SearchResultsColumnsProps {
  response: SearchResponse
  watchlistEntries: WatchlistEntry[] | null
  onAdd: (sourceName: string, result: SearchArtist) => Promise<void>
}

// SearchResultsColumns renders one labelled column per key in the
// response's sources map, side by side (D-09) -- MusicBrainz and Deezer
// results are never interleaved or de-duplicated into a single list, since
// this project never computes a cross-source identity match between the
// two catalogues; presenting them as one list would assert an identity
// match nothing here actually establishes.
export function SearchResultsColumns({
  response,
  watchlistEntries,
  onAdd,
}: SearchResultsColumnsProps) {
  const sourceNames = Object.keys(response.sources)

  return (
    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
      {sourceNames.map((sourceName) => (
        <SourceColumn
          key={sourceName}
          sourceName={sourceName}
          result={response.sources[sourceName]}
          otherSourceNames={sourceNames.filter((name) => name !== sourceName)}
          query={response.query}
          watchlistEntries={watchlistEntries}
          onAdd={onAdd}
        />
      ))}
    </div>
  )
}

interface SourceColumnProps {
  sourceName: string
  result: SourceResult
  otherSourceNames: string[]
  query: string
  watchlistEntries: WatchlistEntry[] | null
  onAdd: (sourceName: string, result: SearchArtist) => Promise<void>
}

// SourceColumn renders one source's own column, branching on that source's
// status independently of the others (D-03's partial-failure contract): an
// "error" status shows the inline unavailable message in this column only,
// a zero-match "ok" status keeps the column and its label visible, and a
// populated "ok" status lists up to the API's fixed 10 results, scrolling
// internally rather than pushing the watchlist list below off screen.
function SourceColumn({
  sourceName,
  result,
  otherSourceNames,
  query,
  watchlistEntries,
  onAdd,
}: SourceColumnProps) {
  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-heading font-semibold text-foreground">
        {sourceLabel(sourceName)}
      </h2>

      {result.status === 'error' && (
        <p className="text-label text-muted-foreground">
          {sourceLabel(sourceName)} is unavailable right now — showing{" "}
          {otherSourceNames.map(sourceLabel).join(", ")} results only.
        </p>
      )}

      {result.status !== 'error' && result.artists.length === 0 && (
        <p className="text-label text-muted-foreground">
          No matches for "{query}"
        </p>
      )}

      {result.status !== 'error' && result.artists.length > 0 && (
        <ul className="flex max-h-96 flex-col gap-3 overflow-y-auto">
          {result.artists.map((artist) => (
            <SearchResultRow
              key={artist.id}
              sourceName={sourceName}
              artist={artist}
              alreadyWatching={
                watchlistEntries?.some((entry) => entry.mbid === artist.id) ??
                false
              }
              onAdd={onAdd}
            />
          ))}
        </ul>
      )}
    </div>
  )
}

interface SearchResultRowProps {
  sourceName: string
  artist: SearchArtist
  alreadyWatching: boolean
  onAdd: (sourceName: string, result: SearchArtist) => Promise<void>
}

// SearchResultRow renders one result: cover art, name, type, and a
// one-line-truncated disambiguation when present, all as plain JSX text
// nodes -- every one of these strings comes from a third-party catalogue,
// so this tree never reaches for React's raw-HTML injection escape hatch.
// The trailing control is either the disabled "Already watching" state
// (D-11, cross-referenced against the already-loaded watchlist entries by
// mbid) or the "Add to Watchlist" action, which tracks its own pending
// state so a failed add returns the row to addable without affecting any
// other row.
function SearchResultRow({
  sourceName,
  artist,
  alreadyWatching,
  onAdd,
}: SearchResultRowProps) {
  const [pending, setPending] = useState(false)

  async function handleClick() {
    setPending(true)
    try {
      await onAdd(sourceName, artist)
    } finally {
      setPending(false)
    }
  }

  return (
    <li className="flex items-center gap-3 rounded-md bg-card p-3">
      <CoverArt src={artist.image_url} alt={artist.name} size={48} />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-body font-medium text-foreground">
          {artist.name}
        </span>
        <span className="text-label text-muted-foreground">{artist.type}</span>
        {artist.disambiguation !== null && (
          <span className="truncate text-label text-muted-foreground">
            {artist.disambiguation}
          </span>
        )}
      </div>
      {alreadyWatching ? (
        <Button variant="secondary" size="sm" disabled className="shrink-0">
          Already watching
        </Button>
      ) : (
        <Button
          size="sm"
          disabled={pending}
          aria-busy={pending}
          onClick={handleClick}
          className="shrink-0"
        >
          {pending ? (
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          ) : (
            "Add to Watchlist"
          )}
        </Button>
      )}
    </li>
  )
}
