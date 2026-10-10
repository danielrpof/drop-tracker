import { createRef } from "react"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { HistoryFilters, type HistoryFiltersValue } from "./HistoryFilters"
import { listWatchlist, type WatchlistEntry } from "~/lib/api"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")

const mockListWatchlist = vi.mocked(listWatchlist)

const artists: WatchlistEntry[] = [
  {
    id: 1,
    artist_id: 10,
    mbid: "mbid-drake",
    name: "Drake",
    deezer_id: null,
    disambiguation: null,
    image_url: null,
    release_types: ["album"],
    muted_event_types: [],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    tags: [],
    note: null,
    latest_release_date: null,
    next_release_date: null,
  },
  {
    id: 2,
    artist_id: 20,
    mbid: "mbid-badbunny",
    name: "Bad Bunny",
    deezer_id: "123",
    disambiguation: null,
    image_url: null,
    release_types: ["album", "single"],
    muted_event_types: [],
    created_at: "2026-01-02T00:00:00Z",
    updated_at: "2026-01-02T00:00:00Z",
    tags: [],
    note: null,
    latest_release_date: null,
    next_release_date: null,
  },
]

const emptyValue: HistoryFiltersValue = {
  artistId: null,
  eventType: null,
  tagId: null,
}

const baseProps = { tags: [], tagsStatus: "loaded" as const }

// getArtistSelect / getEventTypeSelect return the trigger button (D-13: a
// hand-rolled combobox, not a native <select> -- see HistoryFilters.tsx's
// module comment for why). The trigger must be clicked to open the listbox
// before its options are queryable, unlike a native <select> which keeps
// every <option> in the DOM/accessibility tree even while closed -- this is
// actually more correct test behavior, since a real user must open the
// dropdown before seeing its options too.
function getArtistSelect() {
  return screen.getByRole("combobox", { name: "Artist" })
}

function getEventTypeSelect() {
  return screen.getByRole("combobox", { name: "Event type" })
}

async function chooseOption(
  user: ReturnType<typeof userEvent.setup>,
  trigger: HTMLElement,
  optionName: string
) {
  await user.click(trigger)
  await user.click(await screen.findByRole("option", { name: optionName }))
}

describe("HistoryFilters", () => {
  // jsdom has no scrollIntoView, and mockReset clears stubs between tests.
  beforeEach(() => {
    Element.prototype.scrollIntoView = vi.fn()
  })

  it("populates the artist combobox from listWatchlist", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()
    const user = userEvent.setup()

    render(
      <HistoryFilters {...baseProps} value={emptyValue} onChange={onChange} />
    )
    await user.click(getArtistSelect())

    expect(
      await screen.findByRole("option", { name: "Drake" })
    ).toBeInTheDocument()
    expect(
      screen.getByRole("option", { name: "Bad Bunny" })
    ).toBeInTheDocument()
  })

  it("reports the whole new value upward when an artist is chosen", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()
    const user = userEvent.setup()

    render(
      <HistoryFilters {...baseProps} value={emptyValue} onChange={onChange} />
    )

    await chooseOption(user, getArtistSelect(), "Drake")

    expect(onChange).toHaveBeenCalledWith({ ...emptyValue, artistId: 10 })
  })

  it("reports artistId as null, never 0, when 'All artists' is chosen", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()
    const user = userEvent.setup()

    const selectedValue: HistoryFiltersValue = {
      ...emptyValue,
      artistId: 10,
    }
    render(
      <HistoryFilters
        {...baseProps}
        value={selectedValue}
        onChange={onChange}
      />
    )

    await chooseOption(user, getArtistSelect(), "All artists")

    expect(onChange).toHaveBeenCalledWith({ ...selectedValue, artistId: null })
    expect(onChange).not.toHaveBeenCalledWith(
      expect.objectContaining({ artistId: 0 })
    )
  })

  it("reports the whole new value upward when an event type is chosen", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()
    const user = userEvent.setup()

    render(
      <HistoryFilters {...baseProps} value={emptyValue} onChange={onChange} />
    )

    await chooseOption(user, getEventTypeSelect(), "New release")

    expect(onChange).toHaveBeenCalledWith({
      ...emptyValue,
      eventType: "new_release",
    })
  })

  it("reports eventType as null, never an empty string, when 'All event types' is chosen", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()
    const user = userEvent.setup()

    const selectedValue: HistoryFiltersValue = {
      ...emptyValue,
      eventType: "new_release",
    }
    render(
      <HistoryFilters
        {...baseProps}
        value={selectedValue}
        onChange={onChange}
      />
    )

    await chooseOption(user, getEventTypeSelect(), "All event types")

    expect(onChange).toHaveBeenCalledWith({
      ...selectedValue,
      eventType: null,
    })
    expect(onChange).not.toHaveBeenCalledWith(
      expect.objectContaining({ eventType: "" })
    )
  })

  it("closes the dropdown when clicking outside its container", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()
    const user = userEvent.setup()

    render(
      <HistoryFilters {...baseProps} value={emptyValue} onChange={onChange} />
    )
    const trigger = getArtistSelect()

    await user.click(trigger)
    expect(screen.getByRole("listbox", { name: "Artist" })).toBeInTheDocument()

    await user.click(document.body)

    expect(
      screen.queryByRole("listbox", { name: "Artist" })
    ).not.toBeInTheDocument()
  })

  it("opens via ArrowDown while closed and commits the highlighted option with Enter", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()
    const user = userEvent.setup()

    render(
      <HistoryFilters {...baseProps} value={emptyValue} onChange={onChange} />
    )
    const trigger = getEventTypeSelect()
    trigger.focus()

    // Closed -> ArrowDown opens the listbox at the currently selected option
    // ("All event types", index 0) instead of moving a highlight.
    await user.keyboard("{ArrowDown}")
    expect(
      await screen.findByRole("listbox", { name: "Event type" })
    ).toBeInTheDocument()

    // Open -> ArrowDown now moves the highlight forward to "New release".
    await user.keyboard("{ArrowDown}")
    await user.keyboard("{Enter}")

    expect(onChange).toHaveBeenCalledWith({
      ...emptyValue,
      eventType: "new_release",
    })
  })

  it("moves the highlight back up with ArrowUp and Escape closes without changing the value", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()
    const user = userEvent.setup()

    render(
      <HistoryFilters {...baseProps} value={emptyValue} onChange={onChange} />
    )
    const trigger = getEventTypeSelect()

    await user.click(trigger)
    await user.keyboard("{ArrowDown}")
    await user.keyboard("{ArrowUp}")
    await user.keyboard("{Escape}")

    expect(
      screen.queryByRole("listbox", { name: "Event type" })
    ).not.toBeInTheDocument()
    expect(onChange).not.toHaveBeenCalled()
    expect(trigger).toHaveFocus()
  })

  it("falls back to only the 'All artists' option when listWatchlist rejects", async () => {
    mockListWatchlist.mockRejectedValue(new Error("network down"))
    const onChange = vi.fn()
    const user = userEvent.setup()

    render(
      <HistoryFilters {...baseProps} value={emptyValue} onChange={onChange} />
    )
    await user.click(getArtistSelect())

    expect(
      await screen.findByRole("option", { name: "All artists" })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole("option", { name: "Drake" })
    ).not.toBeInTheDocument()
  })

  describe("Tag control", () => {
    const tagValue: HistoryFiltersValue = { ...emptyValue, tagId: 3 }
    const getTagSelect = () => screen.getByRole("combobox", { name: "Tag" })

    it("reads 'Loading tags…' while tags load with a tag selected, and 'All tags' with none", () => {
      mockListWatchlist.mockResolvedValue([])
      const { rerender } = render(
        <HistoryFilters
          value={tagValue}
          onChange={vi.fn()}
          tags={[]}
          tagsStatus="loading"
        />
      )
      expect(getTagSelect()).toHaveTextContent("Loading tags…")

      rerender(
        <HistoryFilters
          value={emptyValue}
          onChange={vi.fn()}
          tags={[]}
          tagsStatus="loading"
        />
      )
      expect(getTagSelect()).toHaveTextContent("All tags")
    })

    it("reads 'Tag unavailable' after a failed load with a tag selected, and 'All tags' with none", async () => {
      mockListWatchlist.mockResolvedValue([])
      const user = userEvent.setup()
      const { rerender } = render(
        <HistoryFilters
          value={tagValue}
          onChange={vi.fn()}
          tags={[]}
          tagsStatus="error"
        />
      )
      expect(getTagSelect()).toHaveTextContent("Tag unavailable")

      rerender(
        <HistoryFilters
          value={emptyValue}
          onChange={vi.fn()}
          tags={[]}
          tagsStatus="error"
        />
      )
      expect(getTagSelect()).toHaveTextContent("All tags")

      await user.click(getTagSelect())
      expect(await screen.findAllByRole("option")).toHaveLength(1)
    })

    it("points a supplied tagTriggerRef at the Tag trigger", () => {
      mockListWatchlist.mockResolvedValue([])
      const ref = createRef<HTMLButtonElement>()
      render(
        <HistoryFilters
          {...baseProps}
          value={emptyValue}
          onChange={vi.fn()}
          tagTriggerRef={ref}
        />
      )
      expect(ref.current).toBe(getTagSelect())
    })

    it("sets a title and truncation on the trigger label, and caps the trigger width", () => {
      mockListWatchlist.mockResolvedValue([])
      const longName = "x".repeat(32)
      render(
        <HistoryFilters
          value={tagValue}
          onChange={vi.fn()}
          tags={[{ id: 3, name: longName, carrier_count: 1 }]}
          tagsStatus="loaded"
        />
      )
      const label = screen.getByTitle(longName)
      expect(label).toHaveClass("truncate")
      expect(getTagSelect()).toHaveClass("max-w-64")
    })
  })

  describe("listbox hardening", () => {
    it("caps every listbox height and scrolls on overflow", async () => {
      mockListWatchlist.mockResolvedValue(artists)
      const user = userEvent.setup()
      render(
        <HistoryFilters {...baseProps} value={emptyValue} onChange={vi.fn()} />
      )

      await user.click(getEventTypeSelect())
      const listbox = screen.getByRole("listbox", { name: "Event type" })
      expect(listbox).toHaveClass("max-h-72", "overflow-y-auto")
    })

    it("scrolls the newly highlighted option into view on ArrowDown", async () => {
      mockListWatchlist.mockResolvedValue(artists)
      const user = userEvent.setup()
      render(
        <HistoryFilters {...baseProps} value={emptyValue} onChange={vi.fn()} />
      )

      await user.click(getEventTypeSelect())
      await user.keyboard("{ArrowDown}")

      const highlighted = screen.getByRole("option", { name: "New release" })
      const scroll = vi.mocked(Element.prototype.scrollIntoView)
      expect(scroll).toHaveBeenCalledWith({ block: "nearest" })
      expect(scroll.mock.contexts).toContain(highlighted)
    })
  })
})
