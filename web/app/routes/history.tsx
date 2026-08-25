import { Loader2 } from "lucide-react"
import { useEffect, useState } from "react"

import { EmptyState } from "~/components/common/EmptyState"
import {
  HistoryFilters,
  type HistoryFiltersValue,
} from "~/components/history/HistoryFilters"
import { EventCard } from "~/components/history/EventCard"
import { Button } from "~/components/ui/button"
import { Skeleton } from "~/components/ui/skeleton"
import { listEvents, type EventItem } from "~/lib/api"

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

// emptyStateCopy selects the correct one of the three empty-state messages
// (D-05, D-06). Order is locked by 10-UI-SPEC.md's Trigger condition and
// supersedes 10-PATTERNS.md's opposite ordering: hasOlderEvents is checked
// FIRST, ahead of isFiltered, because a filtered-and-retention-hidden view
// is better explained by retention than by "no matching events." The
// retention branch never names EVENT_RETENTION_DAYS or a day count -- the
// API exposes only a boolean, so no configured window length reaches here.
function emptyStateCopy(hasOlderEvents: boolean, isFiltered: boolean) {
  if (hasOlderEvents) {
    return {
      heading: "Older than your retention window",
      body: "There's release history for this view — it's just outside your retention window. Nothing was deleted.",
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
    cursor: cursor ?? undefined,
  })
}

// History renders the HIST-01/UI-03 release feed: type-specific EventCards,
// artist/event-type filters that compose, cursor-based "Load more" paging,
// and a distinct message for every empty/loading/partial/error state
// (D-05 through D-08, D-16).
export default function History() {
  const [filters, setFilters] = useState<HistoryFiltersValue>({
    artistId: null,
    eventType: null,
  })
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
  }, [filters.artistId, filters.eventType, reloadToken])

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

  return (
    <div className="flex flex-col gap-6 p-8">
      <h1 className="text-display font-semibold text-foreground">History</h1>

      <HistoryFilters value={filters} onChange={setFilters} />

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
          const { heading, body } = emptyStateCopy(hasOlderEvents, isFiltered)
          return <EmptyState heading={heading} body={body} />
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
