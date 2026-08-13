import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { listWatchlist, removeWatchlist, type WatchlistEntry } from "~/lib/api"
import { renderRoute } from "~/lib/test/routeStub"

import Watchlist from "./watchlist"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")

const mockListWatchlist = vi.mocked(listWatchlist)
const mockRemoveWatchlist = vi.mocked(removeWatchlist)

const entry: WatchlistEntry = {
  id: 42,
  artist_id: 7,
  mbid: "mbid-drake",
  name: "Drake",
  deezer_id: null,
  disambiguation: null,
  image_url: null,
  release_types: ["album"],
  muted_event_types: [],
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
}

describe("Watchlist route", () => {
  it("calls removeWatchlist with the entry's id when its remove control is clicked", async () => {
    mockListWatchlist.mockResolvedValue([entry])
    mockRemoveWatchlist.mockResolvedValue(undefined)

    renderRoute(Watchlist, "/watchlist")

    await screen.findByText("Drake")

    await userEvent.click(
      screen.getByRole("button", { name: "Remove Drake from watchlist" }),
    )

    expect(mockRemoveWatchlist).toHaveBeenCalledWith(42)
  })
})
