import { Loader2 } from "lucide-react"
import { useEffect, useRef, useState } from "react"

import { Input } from "~/components/ui/input"
import { searchArtists, type SearchResponse } from "~/lib/api"

// SEARCH_DEBOUNCE_MS is the wait after the last keystroke before
// searchArtists() fires -- long enough that GET /search (which fans out to
// two live third-party APIs on every call) is not issued on every
// keystroke, short enough to still read as search-as-you-type (D-02,
// T-06-19).
const SEARCH_DEBOUNCE_MS = 300

export interface SearchBoxProps {
  // onResults reports the debounced query's outcome upward: null clears the
  // results area (an empty/whitespace-only query, matching the server's own
  // "q is required" rule, or a whole-request failure), a SearchResponse
  // renders the two source columns.
  onResults: (response: SearchResponse | null) => void
}

// SearchBox is the search half of the Watchlist tab (D-02): a labelled text
// input, debounced ~300ms, with a small inline spinner while a request is
// in flight. Each keystroke supersedes any still-pending debounce/request --
// a fresh AbortController is created per debounced search and the prior one
// is aborted before the new one starts, so a response for a now-stale query
// can never overwrite a newer query's results even if it happens to resolve
// out of order.
export function SearchBox({ onResults }: SearchBoxProps) {
  const [value, setValue] = useState("")
  const [loading, setLoading] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  // Clear any pending debounce/in-flight request on unmount so a fetch that
  // resolves after the Watchlist tab has gone away never calls onResults.
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      abortRef.current?.abort()
    }
  }, [])

  function runSearch(query: string) {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setLoading(true)
    searchArtists(query, controller.signal)
      .then((response) => {
        if (controller.signal.aborted) return
        onResults(response)
      })
      .catch(() => {
        if (controller.signal.aborted) return
        // A failed GET /search as a whole (network error, a non-2xx status
        // that never reached the per-source shape) is distinct from a
        // per-source "unavailable" state inside a successful response --
        // that per-source case is SearchResultsColumns' job. This rarer
        // whole-request failure just clears the results area.
        onResults(null)
      })
      .finally(() => {
        if (controller.signal.aborted) return
        setLoading(false)
      })
  }

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const next = e.target.value
    setValue(next)

    if (debounceRef.current) clearTimeout(debounceRef.current)
    abortRef.current?.abort()

    const trimmed = next.trim()
    if (trimmed === "") {
      setLoading(false)
      onResults(null)
      return
    }

    debounceRef.current = setTimeout(
      () => runSearch(trimmed),
      SEARCH_DEBOUNCE_MS
    )
  }

  return (
    <label className="flex w-full max-w-xl flex-col gap-1 text-label text-muted-foreground">
      Search artists
      <div className="relative">
        <Input
          type="text"
          value={value}
          onChange={handleChange}
          placeholder="Search for an artist…"
          className="pr-10"
        />
        {loading && (
          <Loader2
            className="absolute top-1/2 right-3 size-4 -translate-y-1/2 animate-spin text-muted-foreground"
            aria-hidden="true"
          />
        )}
      </div>
    </label>
  )
}
