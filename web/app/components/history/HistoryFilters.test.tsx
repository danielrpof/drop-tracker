import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

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
  },
]

const emptyValue: HistoryFiltersValue = { artistId: null, eventType: null }

function getArtistSelect() {
  return screen.getByRole("combobox", { name: "Artist" })
}

function getEventTypeSelect() {
  return screen.getByRole("combobox", { name: "Event type" })
}

describe("HistoryFilters", () => {
  it("populates the artist select from listWatchlist", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()

    render(<HistoryFilters value={emptyValue} onChange={onChange} />)

    expect(
      await screen.findByRole("option", { name: "Drake" }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole("option", { name: "Bad Bunny" }),
    ).toBeInTheDocument()
  })

  it("reports the whole new value upward when an artist is chosen", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()

    render(<HistoryFilters value={emptyValue} onChange={onChange} />)
    await screen.findByRole("option", { name: "Drake" })

    await userEvent.selectOptions(getArtistSelect(), "Drake")

    expect(onChange).toHaveBeenCalledWith({ ...emptyValue, artistId: 10 })
  })

  it("reports artistId as null, never 0, when 'All artists' is chosen", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()

    const selectedValue: HistoryFiltersValue = { artistId: 10, eventType: null }
    render(<HistoryFilters value={selectedValue} onChange={onChange} />)
    await screen.findByRole("option", { name: "Drake" })

    await userEvent.selectOptions(getArtistSelect(), "All artists")

    expect(onChange).toHaveBeenCalledWith({ ...selectedValue, artistId: null })
    expect(onChange).not.toHaveBeenCalledWith(
      expect.objectContaining({ artistId: 0 }),
    )
  })

  it("reports the whole new value upward when an event type is chosen", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()

    render(<HistoryFilters value={emptyValue} onChange={onChange} />)
    await screen.findByRole("option", { name: "Drake" })

    await userEvent.selectOptions(getEventTypeSelect(), "New release")

    expect(onChange).toHaveBeenCalledWith({
      ...emptyValue,
      eventType: "new_release",
    })
  })

  it("reports eventType as null, never an empty string, when 'All event types' is chosen", async () => {
    mockListWatchlist.mockResolvedValue(artists)
    const onChange = vi.fn()

    const selectedValue: HistoryFiltersValue = {
      artistId: null,
      eventType: "new_release",
    }
    render(<HistoryFilters value={selectedValue} onChange={onChange} />)
    await screen.findByRole("option", { name: "Drake" })

    await userEvent.selectOptions(getEventTypeSelect(), "All event types")

    expect(onChange).toHaveBeenCalledWith({
      ...selectedValue,
      eventType: null,
    })
    expect(onChange).not.toHaveBeenCalledWith(
      expect.objectContaining({ eventType: "" }),
    )
  })
})
