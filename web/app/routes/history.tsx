import { useEffect, useState } from "react"

import { CoverArt } from "~/components/common/CoverArt"
import { EmptyState } from "~/components/common/EmptyState"
import { Skeleton } from "~/components/ui/skeleton"
import { type EventItem, listEvents } from "~/lib/api"

// History renders the HIST-01/UI-03 release feed: fetches listEvents() in
// an effect on mount and renders one row per returned event, showing its
// CoverArt, title, artist name, and event type -- proving real server data,
// including a real image and a real empty path, reaches the browser. This
// task's job is the end-to-end proof only; type-specific cards, filters,
// "load more" and richer state handling belong to plan 06-02.
export default function History() {
  const [events, setEvents] = useState<EventItem[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    listEvents()
      .then((page) => {
        if (!cancelled) setEvents(page.events)
      })
      .catch(() => {
        if (!cancelled) setError("Couldn't load release history.")
      })

    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="flex flex-col gap-6 p-8">
      <h1 className="text-display font-semibold text-foreground">History</h1>

      {error && (
        <EmptyState heading="Couldn't load release history." body="Please try again." />
      )}

      {!error && events === null && (
        <div className="flex flex-col gap-4">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-24 w-full rounded-md" />
          ))}
        </div>
      )}

      {!error && events !== null && events.length === 0 && (
        <EmptyState
          heading="No release activity yet"
          body="Add an artist to your watchlist to start tracking new releases, features, and deluxe editions."
        />
      )}

      {!error && events !== null && events.length > 0 && (
        <ul className="flex flex-col gap-4">
          {events.map((event) => (
            <li
              key={event.id}
              className="flex items-center gap-4 rounded-md bg-card p-4"
            >
              <CoverArt src={event.cover_art_url} alt={event.title} size={64} />
              <div className="flex flex-col gap-1">
                <span className="text-heading font-semibold text-foreground">
                  {event.title}
                </span>
                <span className="text-body text-muted-foreground">{event.artist_name}</span>
                <span className="text-label text-muted-foreground">{event.event_type}</span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
