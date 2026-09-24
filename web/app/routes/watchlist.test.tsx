import {
  act,
  render,
  screen,
  waitFor,
  waitForElementToBeRemoved,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createRoutesStub } from "react-router"
import { toast } from "sonner"
import { describe, expect, it, vi } from "vitest"

import App from "~/root"
import { authStore } from "~/lib/authStore"
import {
  addWatchlist,
  attachTag,
  deleteTag,
  detachTag,
  listTags,
  listWatchlist,
  mergeTag,
  removeWatchlist,
  renameTag,
  type WatchlistEntry,
} from "~/lib/api"
import { renderRoute } from "~/lib/test/routeStub"

import Watchlist from "./watchlist"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")
// renderRoute stubs Watchlist directly, so root.tsx's <Toaster/> is never
// mounted -- a real sonner toast() call is already a silent no-op in these
// tests. Mocking it just makes the Undo action's onClick reachable for the
// plan 24-07 Task 2 Undo case below (D-27), with no behavior change for any
// existing test in this file.
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

const mockListWatchlist = vi.mocked(listWatchlist)
const mockAddWatchlist = vi.mocked(addWatchlist)
const mockRemoveWatchlist = vi.mocked(removeWatchlist)
const mockDetachTag = vi.mocked(detachTag)
const mockListTags = vi.mocked(listTags)
const mockAttachTag = vi.mocked(attachTag)
const mockDeleteTag = vi.mocked(deleteTag)
const mockRenameTag = vi.mocked(renameTag)
const mockMergeTag = vi.mocked(mergeTag)

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

  it("a pick that brings the artist to 10 tags shows the 'max 10 tags' hint through the real route wiring (D-14)", async () => {
    const nineTags = Array.from({ length: 9 }, (_, i) => ({
      id: i + 1,
      name: `tag${i}`,
    }))
    mockListWatchlist.mockResolvedValue([{ ...entry, tags: nineTags }])
    mockListTags.mockResolvedValue([])
    mockAttachTag.mockResolvedValue({ id: 99, name: "drill" })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )

    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "drill{Enter}")

    await screen.findByText("max 10 tags")
    expect(
      screen.queryByRole("button", { name: "Add tag to Drake" })
    ).toBeNull()
  })

  it("opens Manage tags and deletes a tag carried by two entries, so both cards lose the chip with no extra listWatchlist call", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([
      { ...entry, tags: [{ id: 1, name: "reggaeton" }] },
      { ...second, tags: [{ id: 1, name: "reggaeton" }] },
    ])
    mockListTags.mockResolvedValue([
      { id: 1, name: "reggaeton", carrier_count: 2 },
    ])
    mockDeleteTag.mockResolvedValueOnce({ carrier_count: 2 })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    expect(screen.getAllByText("reggaeton")).toHaveLength(2)

    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })

    await userEvent.click(
      screen.getByRole("button", { name: "Delete tag reggaeton" })
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Delete tag" })
    )

    await waitFor(() =>
      expect(screen.queryAllByText("reggaeton")).toHaveLength(0)
    )
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  it("opens Manage tags and renames a tag carried by two entries, so both cards show the new name with no extra listWatchlist call", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([
      { ...entry, tags: [{ id: 1, name: "Latin" }] },
      { ...second, tags: [{ id: 1, name: "Latin" }] },
    ])
    mockListTags.mockResolvedValue([{ id: 1, name: "Latin", carrier_count: 2 }])
    mockRenameTag.mockResolvedValueOnce({
      kind: "renamed",
      tag: { id: 1, name: "latin" },
    })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    expect(screen.getAllByText("Latin")).toHaveLength(2)

    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })

    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag Latin" })
    )
    const input = screen.getByRole("textbox", { name: "New name for Latin" })
    await userEvent.clear(input)
    await userEvent.type(input, "latin{Enter}")

    await waitFor(() => expect(screen.queryAllByText("Latin")).toHaveLength(0))
    expect(screen.getAllByText("latin")).toHaveLength(3)
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  it("opens Manage tags, confirms a rename collision's merge, and every card carrying the source shows the target instead, with no extra listWatchlist call", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([
      { ...entry, tags: [{ id: 5, name: "rap" }] },
      { ...second, tags: [{ id: 5, name: "rap" }] },
    ])
    mockListTags.mockResolvedValue([
      { id: 5, name: "rap", carrier_count: 2 },
      { id: 9, name: "trap", carrier_count: 1 },
    ])
    mockRenameTag.mockResolvedValueOnce({
      kind: "collision",
      target: { id: 9, name: "trap" },
      carrierCountAfterMerge: 3,
    })
    mockMergeTag.mockResolvedValueOnce({
      id: 9,
      name: "trap",
      carrier_count: 3,
    })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    expect(screen.getAllByText("rap")).toHaveLength(2)

    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })

    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag rap" })
    )
    const input = screen.getByRole("textbox", { name: "New name for rap" })
    await userEvent.clear(input)
    await userEvent.type(input, "trap{Enter}")

    await screen.findByText("Merge “rap” into “trap”?")
    await userEvent.click(screen.getByRole("button", { name: "Merge tags" }))

    await waitFor(() => expect(screen.queryAllByText("rap")).toHaveLength(0))
    expect(screen.getAllByText("trap")).toHaveLength(3)
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  // plan 24-07 Task 2: the remove toast's Undo restores the entry's note
  // (D-27) -- Undo must never silently drop it.
  function undoAction() {
    const call = vi.mocked(toast.success).mock.calls[0] as [
      string,
      { action?: { onClick: () => void } },
    ]
    return call[1].action
  }

  it("Undo re-adds the artist with its note", async () => {
    const withNote: WatchlistEntry = { ...entry, note: "crate digger" }
    mockListWatchlist.mockResolvedValueOnce([withNote])
    mockRemoveWatchlist.mockResolvedValueOnce(undefined)
    mockAddWatchlist.mockResolvedValue(withNote)

    renderRoute(Watchlist, "/")
    await screen.findByText("Drake")

    await userEvent.click(
      screen.getByRole("button", { name: "Remove Drake from watchlist" })
    )
    await waitFor(() => expect(mockRemoveWatchlist).toHaveBeenCalled())

    undoAction()?.onClick()

    await waitFor(() =>
      expect(mockAddWatchlist).toHaveBeenCalledWith(
        expect.objectContaining({ note: "crate digger" })
      )
    )
  })

  it("Undo for an entry with no note sends no note value", async () => {
    mockListWatchlist.mockResolvedValueOnce([entry])
    mockRemoveWatchlist.mockResolvedValueOnce(undefined)
    mockAddWatchlist.mockResolvedValue(entry)

    renderRoute(Watchlist, "/")
    await screen.findByText("Drake")

    await userEvent.click(
      screen.getByRole("button", { name: "Remove Drake from watchlist" })
    )
    await waitFor(() => expect(mockRemoveWatchlist).toHaveBeenCalled())

    undoAction()?.onClick()

    await waitFor(() => expect(mockAddWatchlist).toHaveBeenCalled())
    expect(mockAddWatchlist.mock.calls[0][0].note).toBeUndefined()
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
