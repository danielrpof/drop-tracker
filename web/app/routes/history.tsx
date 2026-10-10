import { Loader2 } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"

import { EmptyState } from "~/components/common/EmptyState"
import {
  HistoryFilters,
  type HistoryFiltersValue,
} from "~/components/history/HistoryFilters"
import { EventCard } from "~/components/history/EventCard"
import { Button } from "~/components/ui/button"
import { Skeleton } from "~/components/ui/skeleton"
import {
  listEvents,
  listTags,
  type EventItem,
  type TagSummary,
  type WatchlistEntry,
} from "~/lib/api"
import { canonicalHistoryParams, parseHistoryParams } from "~/lib/historyParams"
import { useUrlParams } from "~/lib/useUrlParams"

// EventCardSkeleton is the loading placeholder for one card: a hero-art
// block plus text bars (UI-SPEC "loading/History feed" backstop). Rendered
// both while the initial fetch is in flight and, appended below the
// existing cards, while a "Load more" append is in flight.
function EventCardSkeleton() {
  return (
    <div className="flex gap-4 rounded-2xl bg-card p-4 ring-1 ring-foreground/10">
      <Skeleton className="h-24 w-24 shrink-0 rounded-md" />
      <div className="flex flex-1 flex-col gap-2 py-1">
        <Skeleton className="h-5 w-2/3" />
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    </div>
  )
}

const SKELETON_COUNT = 3

function setOrDelete(
  p: URLSearchParams,
  key: string,
  v: string | number | null
) {
  if (v === null) p.delete(key)
  else p.set(key, String(v))
}

// emptyStateCopy picks the empty-state message (D-05, D-06, D-10). Order is
// locked by the UI-SPEC: retention first, since a filtered view that is
// retention-hidden is better explained by retention than by "no matches".
function emptyStateCopy(
  hasOlderEvents: boolean,
  tag: { name: string } | null,
  isFiltered: boolean
) {
  if (hasOlderEvents) {
    return {
      heading: "Older than your retention window",
      body: "There's release history for this view — it's just outside your retention window. Nothing was deleted.",
    }
  }
  if (tag) {
    return {
      heading: `No events for “${tag.name}”`,
      body: isFiltered
        ? `Nothing matches “${tag.name}” with the current artist and event-type filters.`
        : `None of the artists tagged “${tag.name}” have release activity in your retention window.`,
    }
  }
  if (isFiltered) {
    return {
      heading: "No matching events",
      body: "Try a different artist or event type.",
    }
  }
  return {
    heading: "No release activity yet",
    body: "Add an artist to your watchlist to start tracking new releases, features, and deluxe editions.",
  }
}

// fetchHistoryPage calls listEvents() with the active filters and an
// optional cursor -- the one place history.tsx talks to the API, so both
// the initial-fetch effect and "Load more" share identical param-building.
function fetchHistoryPage(filters: HistoryFiltersValue, cursor: string | null) {
  return listEvents({
    artistId: filters.artistId ?? undefined,
    eventType: filters.eventType ?? undefined,
    tagId: filters.tagId ?? undefined,
    cursor: cursor ?? undefined,
  })
}

// History renders the HIST-01/UI-03 release feed: type-specific EventCards,
// artist/event-type filters that compose, cursor-based "Load more" paging,
// and a distinct message for every empty/loading/partial/error state
// (D-05 through D-08, D-16).
export default function History() {
  const { params, write } = useUrlParams()
  const paramsKey = params.toString()
  // Keyed on the string so the page-one effect deps stay primitive.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const filters = useMemo(() => parseHistoryParams(params), [paramsKey])
  const [tagsState, setTagsState] = useState<{
    status: "loading" | "loaded" | "error"
    tags: TagSummary[]
  }>({ status: "loading", tags: [] })
  const [events, setEvents] = useState<EventItem[]>([])
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  // hasOlderEvents (DATA-02, D-06) distinguishes "no release activity ever"
  // from "release activity exists but every event is outside the retention
  // window" -- only the initial-fetch value drives the empty state, but
  // handleLoadMore's fetch also sets it to keep both fetch paths symmetric
  // with how nextCursor is already threaded.
  const [hasOlderEvents, setHasOlderEvents] = useState(false)
  const [initialLoading, setInitialLoading] = useState(true)
  const [appendLoading, setAppendLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [appendError, setAppendError] = useState<string | null>(null)
  // reloadToken bumps to re-run the initial-fetch effect on Retry without
  // changing the filters themselves -- re-issuing the exact same request.
  const [reloadToken, setReloadToken] = useState(0)
  const tagTriggerRef = useRef<HTMLButtonElement>(null)
  const tagIdChecked = useRef(false)
  const artistIdChecked = useRef(false)
  const filtersRef = useRef(filters)
  filtersRef.current = filters

  // D-13: malformed params are rewritten away with replace.
  useEffect(() => {
    const canonical = canonicalHistoryParams(params)
    if (canonical.toString() === paramsKey) return
    write(
      (p) => {
        for (const key of ["artist", "type", "tag"]) {
          const v = canonical.get(key)
          if (v === null) p.delete(key)
          else p.set(key, v)
        }
      },
      { replace: true }
    )
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paramsKey, write])

  // Once per source: a failed GET /tags is not proof the tag id is stale.
  useEffect(() => {
    if (tagsState.status !== "loaded" || tagIdChecked.current) return
    tagIdChecked.current = true
    if (
      filters.tagId !== null &&
      !tagsState.tags.some((t) => t.id === filters.tagId)
    ) {
      write((p) => p.delete("tag"), { replace: true })
    }
  }, [tagsState, filters.tagId, write])

  // Same rule for the artist id, checked when HistoryFilters' watchlist loads.
  const handleArtistsLoaded = (artists: WatchlistEntry[]) => {
    if (artistIdChecked.current) return
    artistIdChecked.current = true
    const id = filtersRef.current.artistId
    if (id !== null && !artists.some((a) => a.artist_id === id)) {
      write((p) => p.delete("artist"), { replace: true })
    }
  }

  // One GET /tags per mount; a failure degrades the Tag menu silently.
  useEffect(() => {
    let cancelled = false
    listTags()
      .then((tags) => {
        if (!cancelled) setTagsState({ status: "loaded", tags })
      })
      .catch(() => {
        if (!cancelled) setTagsState({ status: "error", tags: [] })
      })
    return () => {
      cancelled = true
    }
  }, [])

  // On mount and on any filter change, reset the accumulated page list and
  // fetch page one -- appending filtered results onto unfiltered ones would
  // show rows that do not match the active filter.
  useEffect(() => {
    let cancelled = false
    setInitialLoading(true)
    setError(null)
    setAppendError(null)
    setEvents([])
    setNextCursor(null)
    setHasOlderEvents(false)

    fetchHistoryPage(filters, null)
      .then((page) => {
        if (cancelled) return
        setEvents(page.events)
        setNextCursor(page.next_cursor)
        setHasOlderEvents(page.has_older_events)
      })
      .catch(() => {
        if (!cancelled) setError("Couldn't load release history.")
      })
      .finally(() => {
        if (!cancelled) setInitialLoading(false)
      })

    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters.artistId, filters.eventType, filters.tagId, reloadToken])

  const handleLoadMore = () => {
    if (nextCursor == null || appendLoading) return
    setAppendLoading(true)
    setAppendError(null)

    fetchHistoryPage(filters, nextCursor)
      .then((page) => {
        // Append, never replace -- and de-dupe defensively by id so a
        // double-fired request (should the disabled-button guard above
        // ever be bypassed) can never render the same event twice.
        setEvents((prev) => {
          const seen = new Set(prev.map((e) => e.id))
          const fresh = page.events.filter((e) => !seen.has(e.id))
          return [...prev, ...fresh]
        })
        setNextCursor(page.next_cursor)
        setHasOlderEvents(page.has_older_events)
      })
      .catch(() => {
        setAppendError("Couldn't load release history.")
      })
      .finally(() => {
        setAppendLoading(false)
      })
  }

  const handleRetry = () => {
    setReloadToken((t) => t + 1)
  }

  const isFiltered = filters.artistId != null || filters.eventType != null

  const activeTag =
    filters.tagId === null
      ? null
      : {
          name:
            tagsState.tags.find((t) => t.id === filters.tagId)?.name ??
            "this tag",
        }

  const handleFiltersChange = (next: HistoryFiltersValue) => {
    write((p) => {
      setOrDelete(p, "artist", next.artistId)
      setOrDelete(p, "type", next.eventType)
      setOrDelete(p, "tag", next.tagId)
    })
  }

  const showAllTags = () => {
    write((p) => p.delete("tag"))
    tagTriggerRef.current?.focus()
  }

  return (
    <div className="flex flex-col gap-6 p-8">
      <h1 className="text-display font-semibold text-foreground">History</h1>

      <HistoryFilters
        value={filters}
        onChange={handleFiltersChange}
        tags={tagsState.tags}
        tagsStatus={tagsState.status}
        tagTriggerRef={tagTriggerRef}
        onArtistsLoaded={handleArtistsLoaded}
      />

      {error && (
        <EmptyState
          heading="Couldn't load release history."
          body="Please try again."
          action={
            <Button variant="secondary" onClick={handleRetry}>
              Retry
            </Button>
          }
        />
      )}

      {!error && initialLoading && (
        <div className="flex flex-col gap-4">
          {Array.from({ length: SKELETON_COUNT }).map((_, i) => (
            <EventCardSkeleton key={i} />
          ))}
        </div>
      )}

      {!error &&
        !initialLoading &&
        events.length === 0 &&
        (() => {
          const { heading, body } = emptyStateCopy(
            hasOlderEvents,
            activeTag,
            isFiltered
          )
          // The tag branch only applies when retention doesn't take over.
          const action =
            activeTag && !hasOlderEvents ? (
              <Button variant="secondary" onClick={showAllTags}>
                Show all tags
              </Button>
            ) : undefined
          return <EmptyState heading={heading} body={body} action={action} />
        })()}

      {!error && !initialLoading && events.length > 0 && (
        <div className="flex flex-col gap-4">
          {events.map((event) => (
            <EventCard key={event.id} event={event} />
          ))}

          {appendLoading &&
            Array.from({ length: SKELETON_COUNT }).map((_, i) => (
              <EventCardSkeleton key={`append-skeleton-${i}`} />
            ))}

          {appendError && (
            <p className="text-label text-destructive">{appendError}</p>
          )}

          {nextCursor != null && (
            <Button
              onClick={handleLoadMore}
              disabled={appendLoading}
              aria-busy={appendLoading}
              className="self-center"
            >
              {appendLoading ? (
                <>
                  <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                  Loading…
                </>
              ) : (
                "Load more"
              )}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
