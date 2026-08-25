import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import {
  listEvents,
  listWatchlist,
  type EventItem,
  type EventsPage,
} from "~/lib/api"
import { renderRoute } from "~/lib/test/routeStub"

import History from "./history"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")

const mockListEvents = vi.mocked(listEvents)
// HistoryFilters (rendered by History) calls listWatchlist on mount to
// populate the artist select -- it must resolve or its unhandled effect
// promise breaks every case below, not just the ones that touch filters.
const mockListWatchlist = vi.mocked(listWatchlist)

function makeEvent(overrides: Partial<EventItem> = {}): EventItem {
  return {
    id: 1,
    artist_id: 1,
    source: "musicbrainz",
    event_type: "new_release",
    external_id: "ext-1",
    release_group_mbid: null,
    title: "Default Title",
    artist_name: "Default Artist",
    watched_artist_name: "Default Artist",
    release_date: "2026-01-01",
    cover_art_url: null,
    track_count: null,
    previous_track_count: null,
    release_type: "album",
    notified_at: null,
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  }
}

describe("History route", () => {
  it("renders the error state with a retry control after a failed initial fetch, and retry re-issues the request", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListEvents.mockRejectedValueOnce(new Error("network down"))

    renderRoute(History, "/history")

    await screen.findByRole("heading", {
      name: "Couldn't load release history.",
    })
    const retryButton = screen.getByRole("button", { name: "Retry" })

    const retryEvent = makeEvent({ id: 4, title: "Retried Release" })
    mockListEvents.mockResolvedValueOnce({
      events: [retryEvent],
      next_cursor: null,
      has_older_events: false,
    })

    await userEvent.click(retryButton)

    await screen.findByText(retryEvent.title)

    expect(mockListEvents).toHaveBeenCalledTimes(2)
  })

  it("appends the next page on load more and drops an event already rendered", async () => {
    mockListWatchlist.mockResolvedValue([])

    const eventA = makeEvent({ id: 1, title: "Event Alpha" })
    const eventB = makeEvent({ id: 2, title: "Event Beta" })
    const eventC = makeEvent({ id: 3, title: "Event Charlie" })

    const firstPage: EventsPage = {
      events: [eventA, eventB],
      next_cursor: 100,
      has_older_events: false,
    }
    mockListEvents.mockResolvedValueOnce(firstPage)

    renderRoute(History, "/history")

    await screen.findByText(eventA.title)
    await screen.findByText(eventB.title)

    // Second page repeats eventB's id -- proving append-with-dedupe, not
    // append-with-duplicate.
    const secondPage: EventsPage = {
      events: [eventC, makeEvent({ id: 2, title: eventB.title })],
      next_cursor: null,
      has_older_events: false,
    }
    mockListEvents.mockResolvedValueOnce(secondPage)

    await userEvent.click(screen.getByRole("button", { name: "Load more" }))

    await screen.findByText(eventC.title)

    // Append, not replace -- the first page's titles are still present.
    expect(screen.getByText(eventA.title)).toBeInTheDocument()
    // A count assertion is what distinguishes de-duplication from a lucky
    // render -- mere presence would pass whether or not the repeat was
    // dropped.
    expect(screen.getAllByText(eventB.title)).toHaveLength(1)

    expect(mockListEvents).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: 100 })
    )
  })

  it("shows the unfiltered empty state when there is no release activity at all", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListEvents.mockResolvedValue({
      events: [],
      next_cursor: null,
      has_older_events: false,
    })

    renderRoute(History, "/history")

    await screen.findByRole("heading", { name: "No release activity yet" })
  })

  it("shows the filtered empty state, distinct from the unfiltered one, once a filter is applied", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListEvents.mockResolvedValue({
      events: [],
      next_cursor: null,
      has_older_events: false,
    })

    renderRoute(History, "/history")

    await screen.findByRole("heading", { name: "No release activity yet" })

    await userEvent.click(screen.getByRole("combobox", { name: "Event type" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "New release" })
    )

    await screen.findByRole("heading", { name: "No matching events" })
  })

  it("shows the retention empty state, not the unfiltered one, when the feed is empty but has_older_events is true", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListEvents.mockResolvedValue({
      events: [],
      next_cursor: null,
      has_older_events: true,
    })

    renderRoute(History, "/history")

    await screen.findByRole("heading", {
      name: "Older than your retention window",
    })
    expect(
      screen.getByText(
        "There's release history for this view — it's just outside your retention window. Nothing was deleted."
      )
    ).toBeInTheDocument()
  })

  it("shows the retention empty state over the filtered one when both has_older_events and a filter apply", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListEvents.mockResolvedValue({
      events: [],
      next_cursor: null,
      has_older_events: true,
    })

    renderRoute(History, "/history")

    await screen.findByRole("heading", {
      name: "Older than your retention window",
    })

    await userEvent.click(screen.getByRole("combobox", { name: "Event type" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "New release" })
    )

    await screen.findByRole("heading", {
      name: "Older than your retention window",
    })
    // The negative assertion is what actually pins the priority order --
    // mere presence of the retention heading would also pass if both
    // branches somehow rendered at once.
    expect(
      screen.queryByRole("heading", { name: "No matching events" })
    ).not.toBeInTheDocument()
  })
})
