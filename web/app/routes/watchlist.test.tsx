import {
  act,
  render,
  screen,
  waitForElementToBeRemoved,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createRoutesStub } from "react-router"
import { describe, expect, it, vi } from "vitest"

import App from "~/root"
import { authStore } from "~/lib/authStore"
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

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")

    await userEvent.click(
      screen.getByRole("button", { name: "Remove Drake from watchlist" })
    )

    expect(mockRemoveWatchlist).toHaveBeenCalledWith(42)
  })

  it("removes the row from the DOM after a successful remove, with no rollback re-fetch", async () => {
    mockListWatchlist.mockResolvedValue([entry])
    mockRemoveWatchlist.mockResolvedValue(undefined)

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")

    // The row removal is optimistic and synchronous (setEntries fires before
    // the DELETE call), so by the time userEvent.click's promise resolves
    // the row is already gone -- waitForElementToBeRemoved must start
    // observing while "Drake" still exists, before the click fires, or its
    // initial-existence check throws.
    const removalPromise = waitForElementToBeRemoved(() =>
      screen.queryByText("Drake")
    )

    await userEvent.click(
      screen.getByRole("button", { name: "Remove Drake from watchlist" })
    )

    await removalPromise

    await screen.findByRole("heading", { name: "No artists yet" })

    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  it("renders the error state with a retry control after a failed initial fetch, and retry re-issues the request", async () => {
    mockListWatchlist.mockRejectedValueOnce(new Error("network down"))

    renderRoute(Watchlist, "/")

    await screen.findByRole("heading", {
      name: "Couldn't load your watchlist.",
    })
    const retryButton = screen.getByRole("button", { name: "Retry" })

    mockListWatchlist.mockResolvedValueOnce([])

    await userEvent.click(retryButton)

    await screen.findByRole("heading", { name: "No artists yet" })

    expect(mockListWatchlist).toHaveBeenCalledTimes(2)
  })

  it("shows the empty state when the watchlist has no entries", async () => {
    mockListWatchlist.mockResolvedValue([])

    renderRoute(Watchlist, "/")

    await screen.findByRole("heading", { name: "No artists yet" })
    expect(screen.getByText("Search above to add one.")).toBeInTheDocument()
  })

  it("re-fetches the true server state when removeWatchlist fails, so the row reappears", async () => {
    mockListWatchlist.mockResolvedValueOnce([entry])
    mockRemoveWatchlist.mockRejectedValueOnce(new Error("network down"))

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")

    mockListWatchlist.mockResolvedValueOnce([entry])

    await userEvent.click(
      screen.getByRole("button", { name: "Remove Drake from watchlist" })
    )

    // The optimistic removal happens immediately, then the failed DELETE's
    // catch branch calls refresh() -- the row reappears because the server
    // never actually deleted it, rather than staying gone.
    await screen.findByText("Drake")

    expect(mockListWatchlist).toHaveBeenCalledTimes(2)
  })
})

describe("Watchlist route under the passphrase gate (GATE-05 / UI-SPEC E6)", () => {
  function renderUnderApp() {
    const Stub = createRoutesStub([
      {
        path: "/",
        Component: App,
        children: [{ index: true, Component: Watchlist }],
      },
    ])
    return render(<Stub initialEntries={["/"]} />)
  }

  it("re-runs the route's list fetch after the store flips unauthenticated then authenticated", async () => {
    mockListWatchlist.mockResolvedValue([entry])

    renderUnderApp()
    await screen.findByText("Drake")
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)

    // A 401 elsewhere flips the store: App swaps to the passphrase screen.
    await act(async () => {
      authStore.markUnauthenticated()
    })
    await screen.findByRole("heading", {
      name: "Enter the instance passphrase",
    })

    // Logging back in remounts <Outlet/>, so Watchlist's mount effect
    // re-fetches with no retry-queue machinery (D-16).
    await act(async () => {
      authStore.markAuthenticated()
    })
    await screen.findByText("Drake")
    expect(mockListWatchlist).toHaveBeenCalledTimes(2)
  })

  // 14-UI-SPEC E6 held-out backstop check: a post-login route fetch that
  // returns 401 again must re-show the gate, not a broken authed shell.
  // ~/lib/api is mocked here, so this mock stands in for the real apiFetch
  // 401 interceptor that flips the store (proven for real in api.test.ts).
  it("re-shows the passphrase gate when a post-login route fetch returns 401", async () => {
    mockListWatchlist.mockResolvedValueOnce([entry])

    renderUnderApp()
    await screen.findByText("Drake")

    mockListWatchlist.mockImplementation(() => {
      authStore.markUnauthenticated()
      return Promise.reject(new Error("401"))
    })

    await act(async () => {
      authStore.markUnauthenticated()
    })
    await act(async () => {
      authStore.markAuthenticated()
    })

    await screen.findByRole("heading", {
      name: "Enter the instance passphrase",
    })
  })
})
