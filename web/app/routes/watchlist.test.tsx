import {
  act,
  render,
  screen,
  waitFor,
  waitForElementToBeRemoved,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createRoutesStub } from "react-router"
import { describe, expect, it, vi } from "vitest"

import App from "~/root"
import { authStore } from "~/lib/authStore"
import {
  attachTag,
  detachTag,
  listTags,
  listWatchlist,
  removeWatchlist,
  type WatchlistEntry,
} from "~/lib/api"
import { renderRoute } from "~/lib/test/routeStub"

import Watchlist from "./watchlist"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")

const mockListWatchlist = vi.mocked(listWatchlist)
const mockRemoveWatchlist = vi.mocked(removeWatchlist)
const mockDetachTag = vi.mocked(detachTag)
const mockListTags = vi.mocked(listTags)
const mockAttachTag = vi.mocked(attachTag)

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
  tags: [],
  note: null,
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

  it("mounts exactly one route-level status region, present before the watchlist even loads (UI-SPEC [R6])", async () => {
    mockListWatchlist.mockResolvedValue([entry])

    renderRoute(Watchlist, "/")

    const regions = screen.getAllByRole("status")
    expect(regions).toHaveLength(1)
    expect(regions[0]).toHaveAttribute("aria-atomic", "true")

    await screen.findByText("Drake")
    expect(screen.getAllByRole("status")).toHaveLength(1)
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

  it("removes one tag chip and calls detachTag once when its × is clicked", async () => {
    mockListWatchlist.mockResolvedValue([
      {
        ...entry,
        tags: [
          { id: 1, name: "reggaeton" },
          { id: 2, name: "latin" },
        ],
      },
    ])
    mockDetachTag.mockResolvedValue(undefined)

    renderRoute(Watchlist, "/")

    await screen.findByText("reggaeton")
    expect(screen.getByText("latin")).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole("button", {
        name: "Remove tag reggaeton from Drake",
      })
    )

    expect(screen.queryByText("reggaeton")).not.toBeInTheDocument()
    expect(screen.getByText("latin")).toBeInTheDocument()
    expect(mockDetachTag).toHaveBeenCalledTimes(1)
    expect(mockDetachTag).toHaveBeenCalledWith(42, 1)
  })

  it("never calls listTags at mount, and calls it exactly once across two rows' '+ tag' opens", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([entry, second])
    mockListTags.mockResolvedValue([])

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    expect(mockListTags).not.toHaveBeenCalled()

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )
    await waitFor(() => expect(mockListTags).toHaveBeenCalledTimes(1))

    // The open combobox's popup layer marks the rest of the page inert
    // (base-ui's own background-isolation behavior) -- close it first so
    // Rihanna's row is reachable again before opening its own editor.
    await userEvent.keyboard("{Escape}")
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add tag to Drake" })
      ).toBeInTheDocument()
    )

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Rihanna" })
    )
    expect(mockListTags).toHaveBeenCalledTimes(1)
  })

  it("typing a name and pressing Enter attaches it and shows the server's resolved casing", async () => {
    mockListWatchlist.mockResolvedValue([entry])
    mockListTags.mockResolvedValue([])
    mockAttachTag.mockResolvedValue({ id: 9, name: "reggaeton" })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )

    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "Reggaeton{Enter}")

    expect(mockAttachTag).toHaveBeenCalledWith(42, "Reggaeton")
    await screen.findByText("reggaeton")
  })

  it("still lets Enter attach via Create when listTags rejects", async () => {
    mockListWatchlist.mockResolvedValue([entry])
    mockListTags.mockRejectedValue(new Error("network down"))
    mockAttachTag.mockResolvedValue({ id: 5, name: "drill" })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )

    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "drill{Enter}")

    expect(mockAttachTag).toHaveBeenCalledWith(42, "drill")
    await screen.findAllByText("drill")
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
