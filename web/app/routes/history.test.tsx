import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { toast } from "sonner"
import { beforeEach, describe, expect, it, vi } from "vitest"

import {
  listEvents,
  listTags,
  listWatchlist,
  type EventItem,
  type EventsPage,
} from "~/lib/api"
import { LocationProbe, renderRoute } from "~/lib/test/routeStub"

import History from "./history"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

const mockListEvents = vi.mocked(listEvents)
// HistoryFilters (rendered by History) calls listWatchlist on mount to
// populate the artist select -- it must resolve or its unhandled effect
// promise breaks every case below, not just the ones that touch filters.
const mockListWatchlist = vi.mocked(listWatchlist)
// The automock returns undefined, which would break History's mount-time
// listTags().then(); every case needs a resolved default.
const mockListTags = vi.mocked(listTags)

beforeEach(() => {
  mockListTags.mockResolvedValue([])
})

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

    // A realistic base64url token -- next_cursor is opaque (quick task
    // 260825-g6i), so this must not be a plain integer literal. Not a
    // secret -- it's a base64url-encoded {"d":"2026-01-01","i":100} cursor
    // fixture, but its entropy trips gitleaks' generic-api-key heuristic.
    const nextCursorToken = "eyJkIjoiMjAyNi0wMS0wMSIsImkiOjEwMH0" // gitleaks:allow

    const firstPage: EventsPage = {
      events: [eventA, eventB],
      next_cursor: nextCursorToken,
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
      expect.objectContaining({ cursor: nextCursorToken })
    )
  })

  it("forwards a next_cursor containing base64url characters (-, _) unmodified, never parsed or coerced", async () => {
    mockListWatchlist.mockResolvedValue([])

    // base64url uses "-" and "_" in place of standard base64's "+" and "/"
    // -- this token is opaque and must survive verbatim, not be parsed,
    // compared, or arithmetically manipulated by the client. Not a secret
    // -- see nextCursorToken's comment above.
    const dashUnderscoreToken = "eyJkIjoiMjAyNi0wOC0yNSIsImkiOjk5OTk5OX0-_" // gitleaks:allow

    const eventA = makeEvent({ id: 1, title: "Event Alpha" })
    const eventB = makeEvent({ id: 2, title: "Event Beta" })

    mockListEvents.mockResolvedValueOnce({
      events: [eventA],
      next_cursor: dashUnderscoreToken,
      has_older_events: false,
    })

    renderRoute(History, "/history")

    await screen.findByText(eventA.title)

    mockListEvents.mockResolvedValueOnce({
      events: [eventB],
      next_cursor: null,
      has_older_events: false,
    })

    await userEvent.click(screen.getByRole("button", { name: "Load more" }))

    await screen.findByText(eventB.title)

    expect(mockListEvents).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: dashUnderscoreToken })
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

  it("lists tags from GET /tags once, sorted by name, and a picked tag narrows the feed and survives Load more", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockResolvedValue([
      { id: 5, name: "Afrobeats", carrier_count: 1 },
      { id: 3, name: "reggaeton", carrier_count: 2 },
    ])
    mockListEvents.mockResolvedValue({
      events: [makeEvent({ id: 1, title: "Event Alpha" })],
      next_cursor: null,
      has_older_events: false,
    })

    renderRoute(History, "/history")
    await screen.findByText("Event Alpha")

    await userEvent.click(screen.getByRole("combobox", { name: "Tag" }))
    const options = await screen.findAllByRole("option")
    expect(options.map((o) => o.textContent)).toEqual([
      "All tags",
      "Afrobeats",
      "reggaeton",
    ])

    const narrowed = "eyJkIjoiMjAyNi0wMS0wMSIsImkiOjEwMH0" // gitleaks:allow
    mockListEvents.mockResolvedValueOnce({
      events: [makeEvent({ id: 2, title: "Tagged One" })],
      next_cursor: narrowed,
      has_older_events: false,
    })
    await userEvent.click(screen.getByRole("option", { name: "reggaeton" }))
    await screen.findByText("Tagged One")
    expect(mockListEvents).toHaveBeenLastCalledWith(
      expect.objectContaining({ tagId: 3 })
    )

    mockListEvents.mockResolvedValueOnce({
      events: [makeEvent({ id: 3, title: "Tagged Two" })],
      next_cursor: null,
      has_older_events: false,
    })
    await userEvent.click(screen.getByRole("button", { name: "Load more" }))
    await screen.findByText("Tagged Two")
    expect(mockListEvents).toHaveBeenLastCalledWith(
      expect.objectContaining({ tagId: 3, cursor: narrowed })
    )
    expect(mockListTags).toHaveBeenCalledTimes(1)
  })

  it("drops a Load more response that lands after the filter changed", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockResolvedValue([
      { id: 3, name: "reggaeton", carrier_count: 1 },
    ])
    const cursor = "eyJkIjoiMjAyNi0wMS0wMSIsImkiOjEwMH0" // gitleaks:allow
    mockListEvents.mockResolvedValueOnce({
      events: [makeEvent({ id: 1, title: "Unfiltered One" })],
      next_cursor: cursor,
      has_older_events: false,
    })

    renderRoute(History, "/history")
    await screen.findByText("Unfiltered One")

    let resolveStale!: (page: EventsPage) => void
    mockListEvents.mockReturnValueOnce(
      new Promise<EventsPage>((resolve) => {
        resolveStale = resolve
      })
    )
    await userEvent.click(screen.getByRole("button", { name: "Load more" }))

    mockListEvents.mockResolvedValueOnce({
      events: [makeEvent({ id: 2, title: "Tagged One" })],
      next_cursor: null,
      has_older_events: false,
    })
    await userEvent.click(screen.getByRole("combobox", { name: "Tag" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "reggaeton" })
    )
    await screen.findByText("Tagged One")

    resolveStale({
      events: [makeEvent({ id: 9, title: "Stale Leak" })],
      next_cursor: null,
      has_older_events: false,
    })
    await new Promise((r) => setTimeout(r, 20))

    expect(screen.queryByText("Stale Leak")).not.toBeInTheDocument()
    expect(screen.getByText("Tagged One")).toBeInTheDocument()
  })
})

describe("History tag empty state", () => {
  const emptyPage = {
    events: [],
    next_cursor: null,
    has_older_events: false,
  }

  async function renderWithTagPicked(hasOlder = false) {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockResolvedValue([
      { id: 3, name: "reggaeton", carrier_count: 2 },
    ])
    mockListEvents.mockResolvedValue({
      ...emptyPage,
      has_older_events: hasOlder,
    })
    renderRoute(History, "/history")
    await screen.findByRole("heading", { level: 2 })
    await userEvent.click(screen.getByRole("combobox", { name: "Tag" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "reggaeton" })
    )
  }

  it("explains a tag-only empty feed and offers Show all tags", async () => {
    await renderWithTagPicked()

    await screen.findByRole("heading", { name: "No events for “reggaeton”" })
    expect(
      screen.getByText(
        "None of the artists tagged “reggaeton” have release activity in your retention window."
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Show all tags" })
    ).toBeInTheDocument()
  })

  it("uses the combined-filter body when an event type is also active", async () => {
    await renderWithTagPicked()
    await userEvent.click(screen.getByRole("combobox", { name: "Event type" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "New release" })
    )

    await screen.findByText(
      "Nothing matches “reggaeton” with the current artist and event-type filters."
    )
  })

  it("keeps the retention copy over the tag copy when has_older_events is true", async () => {
    await renderWithTagPicked(true)

    await screen.findByRole("heading", {
      name: "Older than your retention window",
    })
    expect(
      screen.queryByRole("button", { name: "Show all tags" })
    ).not.toBeInTheDocument()
  })

  it("Show all tags clears only the tag, keeps the event type, and refocuses the Tag trigger", async () => {
    await renderWithTagPicked()
    await userEvent.click(screen.getByRole("combobox", { name: "Event type" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "New release" })
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Show all tags" })
    )

    await waitFor(() =>
      expect(mockListEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({
          tagId: undefined,
          eventType: "new_release",
        })
      )
    )
    expect(screen.getByRole("combobox", { name: "Tag" })).toHaveFocus()
  })
})

function HistoryWithProbe() {
  return (
    <>
      <LocationProbe />
      <History />
    </>
  )
}

const emptyEventsPage = {
  events: [],
  next_cursor: null,
  has_older_events: false,
}

describe("History URL state (HIST-02)", () => {
  it("fetches page one with tag_id from the URL before GET /tags resolves", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockReturnValue(new Promise(() => {}))
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?tag=3")

    await waitFor(() =>
      expect(mockListEvents).toHaveBeenCalledWith(
        expect.objectContaining({ tagId: 3 })
      )
    )
  })

  it("keeps tag in the URL when another filter changes", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockResolvedValue([
      { id: 3, name: "reggaeton", carrier_count: 2 },
    ])
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?tag=3")
    await screen.findByRole("heading", { level: 2 })

    await userEvent.click(screen.getByRole("combobox", { name: "Event type" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "Guest feature" })
    )

    const search = screen.getByTestId("location-search")
    await waitFor(() => {
      expect(search).toHaveTextContent("tag=3")
      expect(search).toHaveTextContent("type=guest_feature")
    })
  })

  it("reads artist and type from the URL into the request", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(
      HistoryWithProbe,
      "/history",
      "/history?artist=12&type=deluxe_change"
    )

    await waitFor(() =>
      expect(mockListEvents).toHaveBeenCalledWith(
        expect.objectContaining({ artistId: 12, eventType: "deluxe_change" })
      )
    )
  })
})

describe("History stale and malformed params (D-13)", () => {
  const probe = () => screen.getByTestId("location-search")

  it("drops malformed params with replace and never requests with them", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(
      HistoryWithProbe,
      "/history",
      "/history?artist=abc&type=bogus&tag=0"
    )

    await waitFor(() => expect(probe()).toBeEmptyDOMElement())
    for (const [args] of mockListEvents.mock.calls) {
      expect(args).toEqual(
        expect.objectContaining({
          artistId: undefined,
          eventType: undefined,
          tagId: undefined,
        })
      )
    }
  })

  it("drops a tag absent from the loaded tags and refetches unfiltered", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockResolvedValue([
      { id: 3, name: "reggaeton", carrier_count: 2 },
    ])
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?tag=99")

    await waitFor(() => expect(probe()).toBeEmptyDOMElement())
    await waitFor(() =>
      expect(mockListEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ tagId: undefined })
      )
    )
  })

  it("keeps a tag that is in the loaded tags", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockResolvedValue([
      { id: 3, name: "reggaeton", carrier_count: 2 },
    ])
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?tag=3")

    await screen.findByRole("heading", { name: "No events for “reggaeton”" })
    expect(probe()).toHaveTextContent("tag=3")
  })

  it("keeps the tag filter applied and says so when GET /tags fails", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockRejectedValue(new Error("network down"))
    mockListEvents.mockResolvedValue(emptyEventsPage)
    const user = userEvent.setup()

    renderRoute(HistoryWithProbe, "/history", "/history?tag=3")

    const trigger = screen.getByRole("combobox", { name: "Tag" })
    await waitFor(() => expect(trigger).toHaveTextContent("Tag unavailable"))
    expect(probe()).toHaveTextContent("tag=3")
    expect(mockListEvents).toHaveBeenLastCalledWith(
      expect.objectContaining({ tagId: 3 })
    )
    await user.click(trigger)
    expect(await screen.findAllByRole("option")).toHaveLength(1)
    expect(toast.error).not.toHaveBeenCalled()
  })

  it("names the empty state 'this tag' while the tag name cannot be resolved", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockRejectedValue(new Error("network down"))
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?tag=3")

    await screen.findByRole("heading", { name: "No events for “this tag”" })
  })

  it("reads 'Loading tags…' while GET /tags is pending with a tag in the URL", () => {
    mockListWatchlist.mockResolvedValue([])
    mockListTags.mockReturnValue(new Promise(() => {}))
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?tag=3")

    expect(screen.getByRole("combobox", { name: "Tag" })).toHaveTextContent(
      "Loading tags…"
    )
  })

  it("drops an artist absent from the loaded watchlist", async () => {
    mockListWatchlist.mockResolvedValue([])
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?artist=42")

    await waitFor(() => expect(probe()).toBeEmptyDOMElement())
    await waitFor(() =>
      expect(mockListEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ artistId: undefined })
      )
    )
  })

  it("keeps the artist and says so when the watchlist fetch fails", async () => {
    mockListWatchlist.mockRejectedValue(new Error("network down"))
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?artist=42")

    const trigger = screen.getByRole("combobox", { name: "Artist" })
    await waitFor(() => expect(trigger).toHaveTextContent("Artist unavailable"))
    expect(probe()).toHaveTextContent("artist=42")
    expect(mockListEvents).toHaveBeenLastCalledWith(
      expect.objectContaining({ artistId: 42 })
    )
  })

  it("reads 'Loading artists…' while the watchlist is pending with an artist in the URL", () => {
    mockListWatchlist.mockReturnValue(new Promise(() => {}))
    mockListEvents.mockResolvedValue(emptyEventsPage)

    renderRoute(HistoryWithProbe, "/history", "/history?artist=42")

    expect(screen.getByRole("combobox", { name: "Artist" })).toHaveTextContent(
      "Loading artists…"
    )
  })
})
