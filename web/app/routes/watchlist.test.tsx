import {
  act,
  render,
  screen,
  waitFor,
  waitForElementToBeRemoved,
  within,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createRoutesStub, useNavigate } from "react-router"
import { toast } from "sonner"
import { describe, expect, it, vi } from "vitest"

import App from "~/root"
import { authStore } from "~/lib/authStore"
import {
  addWatchlist,
  attachTag,
  deleteTag,
  detachTag,
  listEvents,
  listTags,
  listWatchlist,
  mergeTag,
  removeWatchlist,
  renameTag,
  searchArtists,
  type TagSummary,
  type WatchlistEntry,
} from "~/lib/api"
import { makeEntry } from "~/lib/test/fixtures"
import { LocationProbe, renderRoute } from "~/lib/test/routeStub"

import Watchlist from "./watchlist"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")
// Toaster is never mounted here, so toast() is already a no-op; mocking it
// only makes the Undo action's onClick reachable (D-27).
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
  latest_release_date: null,
  next_release_date: null,
}

// Lets a test hold a mocked request open and settle it at a chosen moment.
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
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

  it("a tag created while the first vocabulary load is in flight survives that load", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([entry, second])
    const first = deferred<TagSummary[]>()
    mockListTags
      .mockReturnValueOnce(first.promise)
      .mockResolvedValue([{ id: 9, name: "dembow", carrier_count: 1 }])
    mockAttachTag.mockResolvedValue({ id: 9, name: "dembow" })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )
    await userEvent.type(
      screen.getByRole("combobox", { name: "Add tag to Drake" }),
      "dembow{Enter}"
    )
    await screen.findByText("dembow")
    await userEvent.keyboard("{Escape}")
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add tag to Drake" })
      ).toBeInTheDocument()
    )

    await act(async () => {
      first.resolve([])
    })

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Rihanna" })
    )
    await userEvent.type(
      screen.getByRole("combobox", { name: "Add tag to Rihanna" }),
      "dem"
    )

    await screen.findByRole("option", { name: "dembow" })
    expect(mockListTags).toHaveBeenCalledTimes(2)
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

  it("deleting a tag in Manage tags removes it from the '+ tag' autocomplete, with no extra listTags call", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([
      { ...entry, tags: [{ id: 1, name: "reggaeton" }] },
      { ...second, tags: [] },
    ])
    mockListTags.mockResolvedValue([
      { id: 1, name: "reggaeton", carrier_count: 1 },
      { id: 2, name: "drill", carrier_count: 0 },
    ])
    mockDeleteTag.mockResolvedValueOnce({ carrier_count: 1 })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")

    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })

    await userEvent.click(
      screen.getByRole("button", { name: "Delete tag reggaeton" })
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Delete tag" })
    )

    await userEvent.click(screen.getByRole("button", { name: "Close" }))
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Manage tags" })
      ).not.toBeInTheDocument()
    )

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "r")

    await screen.findByRole("option", { name: "drill" })
    expect(
      screen.queryByRole("option", { name: "reggaeton" })
    ).not.toBeInTheDocument()

    await userEvent.type(input, "eggaeton")

    await screen.findByRole("option", { name: "Create “reggaeton”" })
    expect(
      screen.queryByRole("option", { name: "reggaeton" })
    ).not.toBeInTheDocument()

    expect(mockListTags).toHaveBeenCalledTimes(1)
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  it("a '+ tag' vocabulary fetch that settles after a Manage tags delete does not bring the deleted tag back", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([
      { ...entry, tags: [{ id: 1, name: "reggaeton" }] },
      { ...second, tags: [] },
    ])
    const fresh: TagSummary[] = [
      { id: 1, name: "reggaeton", carrier_count: 1 },
      { id: 2, name: "drill", carrier_count: 0 },
    ]
    let settleFirst: (tags: TagSummary[]) => void = () => {}
    const pending = new Promise<TagSummary[]>((resolve) => {
      settleFirst = resolve
    })
    mockListTags.mockReturnValueOnce(pending).mockResolvedValue(fresh)
    mockDeleteTag.mockResolvedValueOnce({ carrier_count: 1 })

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )
    await waitFor(() => expect(mockListTags).toHaveBeenCalledTimes(1))
    await userEvent.keyboard("{Escape}")
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add tag to Drake" })
      ).toBeInTheDocument()
    )

    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })
    expect(mockListTags).toHaveBeenCalledTimes(2)

    await userEvent.click(
      await screen.findByRole("button", { name: "Delete tag reggaeton" })
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Delete tag" })
    )
    await userEvent.click(screen.getByRole("button", { name: "Close" }))
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Manage tags" })
      ).not.toBeInTheDocument()
    )

    await act(async () => {
      settleFirst(fresh)
    })

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "r")

    await screen.findByRole("option", { name: "drill" })
    expect(
      screen.queryByRole("option", { name: "reggaeton" })
    ).not.toBeInTheDocument()

    await userEvent.type(input, "eggaeton")

    await screen.findByRole("option", { name: "Create “reggaeton”" })
    expect(
      screen.queryByRole("option", { name: "reggaeton" })
    ).not.toBeInTheDocument()

    expect(mockListTags).toHaveBeenCalledTimes(2)
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  // Open Manage tags (load 1 stays pending), close it, reopen it (load 2).
  async function openCloseReopenManageTags() {
    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })
    await userEvent.click(screen.getByRole("button", { name: "Close" }))
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Manage tags" })
      ).not.toBeInTheDocument()
    )
    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })
  }

  it("a Manage tags GET /tags from an earlier open that settles after a delete does not bring the deleted tag back", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([
      { ...entry, tags: [{ id: 1, name: "reggaeton" }] },
      { ...second, tags: [] },
    ])
    const fresh: TagSummary[] = [
      { id: 1, name: "reggaeton", carrier_count: 1 },
      { id: 2, name: "drill", carrier_count: 0 },
    ]
    const afterDelete: TagSummary[] = [
      { id: 2, name: "drill", carrier_count: 0 },
    ]
    const first = deferred<TagSummary[]>()
    mockListTags
      .mockReturnValueOnce(first.promise)
      .mockResolvedValueOnce(fresh)
      .mockResolvedValue(afterDelete)
    mockDeleteTag.mockResolvedValueOnce({ carrier_count: 1 })

    renderRoute(Watchlist, "/")
    await screen.findByText("Drake")

    await openCloseReopenManageTags()
    expect(mockListTags).toHaveBeenCalledTimes(2)

    await userEvent.click(
      await screen.findByRole("button", { name: "Delete tag reggaeton" })
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Delete tag" })
    )
    await userEvent.click(screen.getByRole("button", { name: "Close" }))
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Manage tags" })
      ).not.toBeInTheDocument()
    )

    await act(async () => {
      first.resolve(fresh)
    })

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Drake" })
    )
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "r")

    await screen.findByRole("option", { name: "drill" })
    expect(
      screen.queryByRole("option", { name: "reggaeton" })
    ).not.toBeInTheDocument()

    await userEvent.type(input, "eggaeton")

    await screen.findByRole("option", { name: "Create “reggaeton”" })
    expect(
      screen.queryByRole("option", { name: "reggaeton" })
    ).not.toBeInTheDocument()

    expect(mockListTags).toHaveBeenCalledTimes(2)
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)

    await userEvent.keyboard("{Escape}")
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add tag to Drake" })
      ).toBeInTheDocument()
    )
    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })
    await screen.findByRole("button", { name: "Delete tag drill" })
    expect(
      screen.queryByRole("button", { name: "Delete tag reggaeton" })
    ).not.toBeInTheDocument()
    expect(mockListTags).toHaveBeenCalledTimes(3)
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  it("a Manage tags GET /tags from an earlier open that settles after a rename does not undo it in '+ tag'", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([
      { ...entry, tags: [{ id: 1, name: "Latin" }] },
      { ...second, tags: [] },
    ])
    const old: TagSummary[] = [
      { id: 1, name: "Latin", carrier_count: 1 },
      { id: 2, name: "drill", carrier_count: 0 },
    ]
    const first = deferred<TagSummary[]>()
    mockListTags.mockReturnValueOnce(first.promise).mockResolvedValueOnce(old)
    mockRenameTag.mockResolvedValueOnce({
      kind: "renamed",
      tag: { id: 1, name: "Latino" },
    })

    renderRoute(Watchlist, "/")
    await screen.findByText("Drake")

    await openCloseReopenManageTags()
    expect(mockListTags).toHaveBeenCalledTimes(2)

    await userEvent.click(
      await screen.findByRole("button", { name: "Rename tag Latin" })
    )
    const renameInput = screen.getByRole("textbox", {
      name: "New name for Latin",
    })
    await userEvent.clear(renameInput)
    await userEvent.type(renameInput, "Latino{Enter}")
    await screen.findByRole("button", { name: "Rename tag Latino" })

    await userEvent.click(screen.getByRole("button", { name: "Close" }))
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Manage tags" })
      ).not.toBeInTheDocument()
    )

    await act(async () => {
      first.resolve(old)
    })

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Rihanna" })
    )
    const input = screen.getByRole("combobox", { name: "Add tag to Rihanna" })
    await userEvent.type(input, "Latin")

    await screen.findByRole("option", { name: "Latino" })
    await screen.findByRole("option", { name: "Create “Latin”" })
    expect(
      screen.queryByRole("option", { name: "Latin" })
    ).not.toBeInTheDocument()

    expect(mockListTags).toHaveBeenCalledTimes(2)
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  it("a Manage tags GET /tags from an earlier open that settles after a merge does not bring the merged-away tag back", async () => {
    const second: WatchlistEntry = { ...entry, id: 43, name: "Rihanna" }
    mockListWatchlist.mockResolvedValue([
      { ...entry, tags: [{ id: 5, name: "rap" }] },
      { ...second, tags: [] },
    ])
    const old: TagSummary[] = [
      { id: 5, name: "rap", carrier_count: 1 },
      { id: 9, name: "trap", carrier_count: 0 },
    ]
    const first = deferred<TagSummary[]>()
    mockListTags.mockReturnValueOnce(first.promise).mockResolvedValueOnce(old)
    mockRenameTag.mockResolvedValueOnce({
      kind: "collision",
      target: { id: 9, name: "trap" },
      carrierCountAfterMerge: 1,
    })
    mockMergeTag.mockResolvedValueOnce({
      id: 9,
      name: "trap",
      carrier_count: 1,
    })

    renderRoute(Watchlist, "/")
    await screen.findByText("Drake")

    await openCloseReopenManageTags()
    expect(mockListTags).toHaveBeenCalledTimes(2)

    await userEvent.click(
      await screen.findByRole("button", { name: "Rename tag rap" })
    )
    const renameInput = screen.getByRole("textbox", {
      name: "New name for rap",
    })
    await userEvent.clear(renameInput)
    await userEvent.type(renameInput, "trap{Enter}")

    await screen.findByText("Merge “rap” into “trap”?")
    await userEvent.click(screen.getByRole("button", { name: "Merge tags" }))
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Rename tag rap" })
      ).not.toBeInTheDocument()
    )

    await userEvent.click(screen.getByRole("button", { name: "Close" }))
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Manage tags" })
      ).not.toBeInTheDocument()
    )

    await act(async () => {
      first.resolve(old)
    })

    await userEvent.click(
      screen.getByRole("button", { name: "Add tag to Rihanna" })
    )
    const input = screen.getByRole("combobox", { name: "Add tag to Rihanna" })
    await userEvent.type(input, "rap")

    await screen.findByRole("option", { name: "trap" })
    await screen.findByRole("option", { name: "Create “rap”" })
    expect(
      screen.queryByRole("option", { name: "rap" })
    ).not.toBeInTheDocument()

    expect(mockListTags).toHaveBeenCalledTimes(2)
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

  // The remove toast's Undo must restore the entry's note (D-27).
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

  it("renders each card's latest release and a conditional Upcoming segment (D-03)", async () => {
    mockListWatchlist.mockResolvedValue([
      {
        ...entry,
        id: 1,
        name: "Drake",
        latest_release_date: "2024-05-17",
        next_release_date: "2027",
      },
      { ...entry, id: 2, name: "Kendrick" },
    ])

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    const [drake, kendrick] = screen.getAllByRole("listitem")

    expect(
      within(drake).getByText("Latest release: 17 May 2024")
    ).toBeInTheDocument()
    expect(within(drake).getByText("Upcoming: 2027")).toBeInTheDocument()
    expect(within(kendrick).getByText("No releases yet")).toBeInTheDocument()
    expect(within(kendrick).queryByText(/Upcoming:/)).not.toBeInTheDocument()
  })

  it("shows Upcoming alongside No releases yet when only next_release_date is set", async () => {
    mockListWatchlist.mockResolvedValue([
      makeEntry({ id: 1, name: "Drake", next_release_date: "2026-11" }),
    ])

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    const card = screen.getByRole("listitem")
    expect(within(card).getByText("No releases yet")).toBeInTheDocument()
    expect(within(card).getByText("Upcoming: Nov 2026")).toBeInTheDocument()
  })

  it("renders a release date containing markup as literal text (T-25-09)", async () => {
    mockListWatchlist.mockResolvedValue([
      makeEntry({ id: 1, name: "Drake", latest_release_date: "<b>2024</b>" }),
    ])

    renderRoute(Watchlist, "/")

    await screen.findByText("Drake")
    const card = screen.getByRole("listitem")
    expect(
      within(card).getByText("Latest release: <b>2024</b>")
    ).toBeInTheDocument()
    expect(card.querySelector("b")).toBeNull()
  })
})

describe("Watchlist name filter (WLVW-01)", () => {
  const filterEntries = [
    makeEntry({ id: 1, name: "Bad Bunny" }),
    makeEntry({ id: 2, name: "Beyoncé", disambiguation: "bad influence" }),
    makeEntry({ id: 3, name: "A$AP Rocky" }),
  ]

  function WatchlistWithProbe() {
    return (
      <>
        <LocationProbe />
        <Watchlist />
      </>
    )
  }

  it("narrows the list on every keystroke, ignores disambiguation, and keeps the count in step", async () => {
    mockListWatchlist.mockResolvedValue(filterEntries)

    renderRoute(Watchlist, "/")

    await screen.findByText("Bad Bunny")
    expect(screen.getByText("3 of 3 artists")).toBeInTheDocument()

    await userEvent.type(
      screen.getByRole("textbox", { name: "Filter watchlist" }),
      "bad"
    )

    expect(screen.getByText("Bad Bunny")).toBeInTheDocument()
    expect(screen.queryByText("Beyoncé")).not.toBeInTheDocument()
    expect(screen.queryByText("A$AP Rocky")).not.toBeInTheDocument()
    expect(screen.getByText("1 of 3 artists")).toBeInTheDocument()
  })

  it("writes q to the URL after the debounce", async () => {
    mockListWatchlist.mockResolvedValue(filterEntries)

    renderRoute(WatchlistWithProbe, "/")
    await screen.findByText("Bad Bunny")

    await userEvent.type(
      screen.getByRole("textbox", { name: "Filter watchlist" }),
      "bad"
    )

    await waitFor(() =>
      expect(screen.getByTestId("location-search")).toHaveTextContent("?q=bad")
    )
  })

  it("reproduces the filtered view from ?q= on load, with the input pre-filled", async () => {
    mockListWatchlist.mockResolvedValue(filterEntries)

    renderRoute(Watchlist, "/", "/?q=bey")

    await screen.findByText("Beyoncé")
    expect(screen.queryByText("Bad Bunny")).not.toBeInTheDocument()
    expect(
      screen.getByRole("textbox", { name: "Filter watchlist" })
    ).toHaveValue("bey")
  })

  it("issues exactly one GET /watchlist while filtering and never asks for tags or events (SC5)", async () => {
    mockListWatchlist.mockResolvedValue(filterEntries)

    renderRoute(Watchlist, "/")
    await screen.findByText("Bad Bunny")

    await userEvent.type(
      screen.getByRole("textbox", { name: "Filter watchlist" }),
      "beyonce"
    )

    expect(screen.getByText("Beyoncé")).toBeInTheDocument()
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
    expect(vi.mocked(listTags)).not.toHaveBeenCalled()
    expect(vi.mocked(listEvents)).not.toHaveBeenCalled()
  })
})

describe("Watchlist filtered state (WLVW-06)", () => {
  const three = [
    makeEntry({ id: 1, name: "Bad Bunny" }),
    makeEntry({ id: 2, name: "Beyoncé" }),
    makeEntry({ id: 3, name: "A$AP Rocky" }),
  ]
  const filterBox = () =>
    screen.getByRole("textbox", { name: "Filter watchlist" })

  function WatchlistWithProbe() {
    return (
      <>
        <LocationProbe />
        <Watchlist />
      </>
    )
  }

  // LocationProbe's <output> is also an implicit status role.
  function status() {
    const region = screen
      .getAllByRole("status")
      .find((el) => el.getAttribute("aria-atomic") === "true")
    if (!region) throw new Error("route status region not found")
    return region
  }

  function lastUndo() {
    const calls = vi.mocked(toast.success).mock.calls
    const call = calls[calls.length - 1] as [
      string,
      { action?: { onClick: () => void } },
    ]
    return call[1].action
  }

  it("replaces the list with the filtered-to-zero state and clears everything but sort", async () => {
    mockListWatchlist.mockResolvedValue(three)

    renderRoute(WatchlistWithProbe, "/", "/?q=zzz&sort=latest-desc")

    await screen.findByRole("heading", {
      name: "No artists match these filters",
    })
    expect(
      screen.getByText(
        "Try a different name or fewer filters, or clear them to see all 3 artists."
      )
    ).toBeInTheDocument()
    expect(screen.getByText("0 of 3 artists")).toBeInTheDocument()
    expect(screen.queryByRole("list")).not.toBeInTheDocument()
    const clearButtons = screen.getAllByRole("button", {
      name: "Clear filters",
    })
    expect(clearButtons).toHaveLength(1)

    await userEvent.click(clearButtons[0])

    await waitFor(() =>
      expect(screen.getByTestId("location-search")).toHaveTextContent(
        /^\?sort=latest-desc$/
      )
    )
    expect(filterBox()).toHaveValue("")
    expect(filterBox()).toHaveFocus()
    expect(status()).toHaveTextContent(
      "Filters cleared. Showing 3 of 3 artists."
    )
    expect(screen.getByText("Bad Bunny")).toBeInTheDocument()
  })

  it("shows the toolbar Clear filters only for a non-blank query with results", async () => {
    mockListWatchlist.mockResolvedValue(three)

    renderRoute(Watchlist, "/")
    await screen.findByText("Bad Bunny")
    const clear = () =>
      screen.queryAllByRole("button", { name: "Clear filters" })
    expect(clear()).toHaveLength(0)

    await userEvent.type(filterBox(), "   ")
    expect(clear()).toHaveLength(0)

    await userEvent.clear(filterBox())
    await userEvent.type(filterBox(), "bad")
    expect(clear()).toHaveLength(1)
    expect(screen.getByText("Bad Bunny")).toBeInTheDocument()

    await userEvent.type(filterBox(), "zzz")
    await screen.findByRole("heading", {
      name: "No artists match these filters",
    })
    expect(clear()).toHaveLength(1)
    expect(
      clear()[0].closest("[aria-label='Filter and sort artists']")
    ).toBeNull()
  })

  it("announces the count 500ms after the last keystroke", async () => {
    mockListWatchlist.mockResolvedValue(three)

    renderRoute(Watchlist, "/")
    await screen.findByText("Bad Bunny")
    expect(status()).toHaveTextContent("")

    await userEvent.type(filterBox(), "bad")
    await waitFor(
      () => expect(status()).toHaveTextContent("Showing 1 of 3 artists."),
      { timeout: 2000 }
    )
  })

  it("announces when nothing matches", async () => {
    mockListWatchlist.mockResolvedValue(three)

    renderRoute(Watchlist, "/")
    await screen.findByText("Bad Bunny")

    await userEvent.type(filterBox(), "zzz")
    await waitFor(
      () =>
        expect(status()).toHaveTextContent("No artists match these filters."),
      { timeout: 2000 }
    )
  })

  it("keeps an artist added from search visible under an active filter, until the search text changes", async () => {
    const drizzy = makeEntry({ id: 99, name: "Drizzy", mbid: "mbid-99" })
    mockListWatchlist
      .mockResolvedValueOnce([three[0], three[1]])
      .mockResolvedValueOnce([three[0], three[1], drizzy])
    vi.mocked(searchArtists).mockResolvedValue({
      query: "Drizzy",
      sources: {
        musicbrainz: {
          status: "ok",
          artists: [
            {
              source: "musicbrainz",
              id: "mbid-99",
              name: "Drizzy",
              disambiguation: null,
              country: null,
              type: "Person",
              image_url: null,
            },
          ],
        },
      },
    })
    mockAddWatchlist.mockResolvedValue(drizzy)

    renderRoute(Watchlist, "/", "/?q=bad")
    await screen.findByText("Bad Bunny")
    expect(screen.queryByText("Beyoncé")).not.toBeInTheDocument()

    await userEvent.type(
      screen.getByRole("textbox", { name: "Search artists" }),
      "Drizzy"
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Add to Watchlist" })
    )

    await screen.findByRole("button", { name: "Remove Drizzy from watchlist" })
    expect(screen.getByText("2 of 3 artists")).toBeInTheDocument()

    await userEvent.type(filterBox(), "x")
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Remove Drizzy from watchlist" })
      ).not.toBeInTheDocument()
    )
  })

  it("keeps an Undo-restored artist visible under an active filter", async () => {
    const bunny = three[0]
    const restored = makeEntry({ id: 77, name: "Bad Bunny", mbid: bunny.mbid })
    mockListWatchlist
      .mockResolvedValueOnce([bunny, three[1]])
      .mockResolvedValueOnce([three[1], restored])
    mockRemoveWatchlist.mockResolvedValueOnce(undefined)
    mockAddWatchlist.mockResolvedValue(restored)

    renderRoute(Watchlist, "/", "/?q=bad")
    await screen.findByText("Bad Bunny")

    await userEvent.click(
      screen.getByRole("button", { name: "Remove Bad Bunny from watchlist" })
    )
    await waitFor(() => expect(mockRemoveWatchlist).toHaveBeenCalled())
    await userEvent.type(filterBox(), "x")

    lastUndo()?.onClick()

    await screen.findByRole("button", {
      name: "Remove Bad Bunny from watchlist",
    })
    expect(screen.getByText("1 of 2 artists")).toBeInTheDocument()
  })

  it("renders no section or toolbar while loading or with no artists", async () => {
    const pending = deferred<WatchlistEntry[]>()
    mockListWatchlist.mockReturnValueOnce(pending.promise)

    const view = renderRoute(Watchlist, "/")
    expect(
      screen.queryByRole("textbox", { name: "Filter watchlist" })
    ).not.toBeInTheDocument()
    expect(view.container.querySelectorAll(".h-24")).toHaveLength(3)

    await act(async () => {
      pending.resolve([])
    })
    await screen.findByRole("heading", { name: "No artists yet" })
    expect(
      screen.queryByRole("heading", { name: "Your artists" })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole("textbox", { name: "Filter watchlist" })
    ).not.toBeInTheDocument()
  })

  it("shows no toolbar when the load fails", async () => {
    mockListWatchlist.mockRejectedValueOnce(new Error("down"))

    renderRoute(Watchlist, "/")

    await screen.findByRole("heading", {
      name: "Couldn't load your watchlist.",
    })
    expect(
      screen.queryByRole("textbox", { name: "Filter watchlist" })
    ).not.toBeInTheDocument()
  })
})

describe("Watchlist sort (WLVW-02, WLVW-03)", () => {
  const sortEntries = [
    makeEntry({
      id: 1,
      name: "Alpha",
      latest_release_date: "2023-01-01",
      created_at: "2026-03-01T00:00:00Z",
    }),
    makeEntry({
      id: 2,
      name: "Bravo",
      latest_release_date: null,
      created_at: "2026-02-01T00:00:00Z",
    }),
    makeEntry({
      id: 3,
      name: "Charlie",
      latest_release_date: "2025-06-01",
      created_at: "2026-01-01T00:00:00Z",
    }),
  ]

  function WatchlistWithProbe() {
    return (
      <>
        <LocationProbe />
        <Watchlist />
      </>
    )
  }

  const names = () =>
    screen
      .getAllByRole("listitem")
      .map((li) => li.querySelector("span[title]")?.getAttribute("title"))

  it("reorders the cards and writes the URL when a sort is chosen, without refetching", async () => {
    mockListWatchlist.mockResolvedValue(sortEntries)

    renderRoute(WatchlistWithProbe, "/")
    await screen.findByText("Alpha")
    expect(names()).toEqual(["Alpha", "Bravo", "Charlie"])

    await userEvent.click(screen.getByRole("combobox", { name: "Sort by" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "Latest release (newest)" })
    )

    await waitFor(() => expect(names()).toEqual(["Charlie", "Alpha", "Bravo"]))
    expect(screen.getByTestId("location-search")).toHaveTextContent(
      /^\?sort=latest-desc$/
    )
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  it("renders the order from the URL on reload", async () => {
    mockListWatchlist.mockResolvedValue(sortEntries)

    renderRoute(Watchlist, "/", "/?sort=added-asc")

    await screen.findByText("Alpha")
    expect(names()).toEqual(["Charlie", "Bravo", "Alpha"])
    expect(screen.getByRole("combobox", { name: "Sort by" })).toHaveTextContent(
      "Date added (oldest)"
    )
  })

  // LocationProbe's <output> is also an implicit status role.
  function status() {
    const region = screen
      .getAllByRole("status")
      .find((el) => el.getAttribute("aria-atomic") === "true")
    if (!region) throw new Error("route status region not found")
    return region
  }

  it.each(["bogus", "name-asc"])(
    "drops sort=%s from the URL and renders Name (A-Z)",
    async (value) => {
      mockListWatchlist.mockResolvedValue(sortEntries)

      renderRoute(WatchlistWithProbe, "/", `/?sort=${value}`)

      await screen.findByText("Alpha")
      await waitFor(() =>
        expect(screen.getByTestId("location-search")).toHaveTextContent(/^$/)
      )
      expect(names()).toEqual(["Alpha", "Bravo", "Charlie"])
      expect(
        screen.getByRole("combobox", { name: "Sort by" })
      ).toHaveTextContent("Name (A–Z)")
    }
  )

  it("announces the sort immediately with no count message after it", async () => {
    mockListWatchlist.mockResolvedValue(sortEntries)

    renderRoute(Watchlist, "/")
    await screen.findByText("Alpha")

    await userEvent.click(screen.getByRole("combobox", { name: "Sort by" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "Date added (newest)" })
    )

    expect(status()).toHaveTextContent("Sorted by Date added (newest).")
    await new Promise((r) => setTimeout(r, 700))
    expect(status()).toHaveTextContent("Sorted by Date added (newest).")
    expect(status()).not.toHaveTextContent("Showing")
  })

  it("cancels a pending count announcement when the sort changes", async () => {
    mockListWatchlist.mockResolvedValue(sortEntries)

    renderRoute(Watchlist, "/")
    await screen.findByText("Alpha")

    await userEvent.type(
      screen.getByRole("textbox", { name: "Filter watchlist" }),
      "a"
    )
    await userEvent.click(screen.getByRole("combobox", { name: "Sort by" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "Date added (newest)" })
    )
    await new Promise((r) => setTimeout(r, 700))

    expect(status()).toHaveTextContent("Sorted by Date added (newest).")
  })

  it("keeps the sort when Clear filters runs", async () => {
    mockListWatchlist.mockResolvedValue(sortEntries)

    renderRoute(WatchlistWithProbe, "/", "/?q=alp&sort=latest-desc")
    await screen.findByText("Alpha")

    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }))

    await waitFor(() =>
      expect(screen.getByTestId("location-search")).toHaveTextContent(
        /^\?sort=latest-desc$/
      )
    )
  })

  it("drops a sticky card when the sort changes", async () => {
    const drizzy = makeEntry({ id: 99, name: "Drizzy", mbid: "mbid-99" })
    mockListWatchlist
      .mockResolvedValueOnce(sortEntries)
      .mockResolvedValueOnce([...sortEntries, drizzy])
    vi.mocked(searchArtists).mockResolvedValue({
      query: "Drizzy",
      sources: {
        musicbrainz: {
          status: "ok",
          artists: [
            {
              source: "musicbrainz",
              id: "mbid-99",
              name: "Drizzy",
              disambiguation: null,
              country: null,
              type: "Person",
              image_url: null,
            },
          ],
        },
      },
    })
    mockAddWatchlist.mockResolvedValue(drizzy)

    renderRoute(Watchlist, "/", "/?q=alp")
    await screen.findByText("Alpha")

    await userEvent.type(
      screen.getByRole("textbox", { name: "Search artists" }),
      "Drizzy"
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Add to Watchlist" })
    )
    await screen.findByRole("button", { name: "Remove Drizzy from watchlist" })

    await userEvent.click(screen.getByRole("combobox", { name: "Sort by" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "Date added (newest)" })
    )

    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Remove Drizzy from watchlist" })
      ).not.toBeInTheDocument()
    )
  })
})

describe("Watchlist tag filter (WLVW-04)", () => {
  const reggaeton = { id: 3, name: "reggaeton" }
  const trap = { id: 7, name: "trap" }
  const tagEntries = [
    makeEntry({ id: 1, name: "Bad Bunny", tags: [reggaeton] }),
    makeEntry({ id: 2, name: "Daddy Yankee", tags: [reggaeton] }),
    makeEntry({ id: 3, name: "Travis Scott", tags: [trap] }),
  ]

  function WatchlistWithProbe() {
    return (
      <>
        <LocationProbe />
        <Watchlist />
      </>
    )
  }

  const shown = () =>
    screen
      .queryAllByRole("button", {
        name: /^Remove .* from watchlist$/,
        hidden: true,
      })
      .map((b) => b.getAttribute("aria-label")?.slice(7, -15))

  const tagInput = () => screen.getByLabelText("Filter by tags, any of")

  it("narrows to the carriers of a picked tag, keeps count and URL in step, and never asks for tags", async () => {
    mockListWatchlist.mockResolvedValue(tagEntries)

    renderRoute(WatchlistWithProbe, "/")
    await screen.findByText("Bad Bunny")

    await userEvent.click(tagInput())
    await userEvent.click(
      await screen.findByRole("option", { name: /reggaeton/ })
    )

    await waitFor(() => expect(shown()).toEqual(["Bad Bunny", "Daddy Yankee"]))
    expect(screen.getByText("2 of 3 artists")).toBeInTheDocument()
    expect(screen.getByTestId("location-search")).toHaveTextContent(
      /^\?tags=3$/
    )
    expect(mockListTags).not.toHaveBeenCalled()
    expect(mockListWatchlist).toHaveBeenCalledTimes(1)
  })

  it("reproduces a multi-tag selection from the URL with any-of semantics", async () => {
    mockListWatchlist.mockResolvedValue(tagEntries)

    renderRoute(Watchlist, "/", "/?tags=3,7")
    await screen.findByText("Bad Bunny")

    expect(shown()).toEqual(["Bad Bunny", "Daddy Yankee", "Travis Scott"])
    expect(
      screen.getByRole("button", { name: "Remove trap from tag filter" })
    ).toBeInTheDocument()
    expect(mockListTags).not.toHaveBeenCalled()
  })
})

describe("Watchlist tag filter consistency (D-13)", () => {
  const rap = { id: 5, name: "rap" }
  const trap = { id: 9, name: "trap" }
  const rows = [
    makeEntry({ id: 1, name: "Bad Bunny", tags: [rap] }),
    makeEntry({ id: 2, name: "Daddy Yankee", tags: [trap] }),
    makeEntry({ id: 3, name: "Travis Scott", tags: [trap] }),
  ]

  function BackButton() {
    const navigate = useNavigate()
    return <button onClick={() => navigate(-1)}>history back</button>
  }

  function WatchlistWithProbe() {
    return (
      <>
        <LocationProbe />
        <BackButton />
        <Watchlist />
      </>
    )
  }

  const search = () => screen.getByTestId("location-search").textContent
  const shown = () =>
    screen
      .queryAllByRole("button", {
        name: /^Remove .* from watchlist$/,
        hidden: true,
      })
      .map((b) => b.getAttribute("aria-label")?.slice(7, -15))
  const filterChip = (name: string) =>
    screen.queryByRole("button", {
      name: `Remove ${name} from tag filter`,
      hidden: true,
    })

  async function openManageTags() {
    await userEvent.click(screen.getByRole("button", { name: "Manage tags" }))
    await screen.findByRole("heading", { name: "Manage tags" })
  }

  async function deleteTagInDialog(name: string) {
    await userEvent.click(
      screen.getByRole("button", { name: `Delete tag ${name}` })
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Delete tag" })
    )
  }

  async function mergeRapIntoTrap() {
    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag rap" })
    )
    const input = screen.getByRole("textbox", { name: "New name for rap" })
    await userEvent.clear(input)
    await userEvent.type(input, "trap{Enter}")
    await screen.findByText("Merge “rap” into “trap”?")
    await userEvent.click(screen.getByRole("button", { name: "Merge tags" }))
  }

  function mockVocabulary() {
    mockListTags.mockResolvedValue([
      { id: 5, name: "rap", carrier_count: 1 },
      { id: 9, name: "trap", carrier_count: 2 },
    ])
  }

  it("drops URL tag ids that no loaded artist carries, with replace", async () => {
    mockListWatchlist.mockResolvedValue(rows)

    renderRoute(WatchlistWithProbe, "/", "/?tags=5,99")
    await screen.findByText("Bad Bunny")

    await waitFor(() => expect(search()).toBe("?tags=5"))
    expect(shown()).toEqual(["Bad Bunny"])
  })

  it("clears a tags param that matches nothing and shows the full list", async () => {
    mockListWatchlist.mockResolvedValue(rows)

    renderRoute(WatchlistWithProbe, "/", "/?tags=99")

    await waitFor(() => expect(search()).toBe(""))
    await screen.findByText("Bad Bunny")
    expect(shown()).toEqual(["Bad Bunny", "Daddy Yankee", "Travis Scott"])
  })

  it("drops a tag from the filter when Manage tags deletes it", async () => {
    mockListWatchlist.mockResolvedValue(rows)
    mockVocabulary()
    mockDeleteTag.mockResolvedValueOnce({ carrier_count: 1 })

    renderRoute(WatchlistWithProbe, "/", "/?tags=5")
    await screen.findByText("Bad Bunny")
    expect(filterChip("rap")).toBeInTheDocument()

    await openManageTags()
    await deleteTagInDialog("rap")

    await waitFor(() => expect(search()).toBe(""))
    expect(filterChip("rap")).not.toBeInTheDocument()
  })

  it("swaps a merged-away tag for its target in the filter", async () => {
    mockListWatchlist.mockResolvedValue(rows)
    mockVocabulary()
    mockRenameTag.mockResolvedValueOnce({
      kind: "collision",
      target: trap,
      carrierCountAfterMerge: 3,
    })
    mockMergeTag.mockResolvedValueOnce({ ...trap, carrier_count: 3 })

    renderRoute(WatchlistWithProbe, "/", "/?tags=5")
    await screen.findByText("Bad Bunny")

    await openManageTags()
    await mergeRapIntoTrap()

    await waitFor(() => expect(search()).toBe("?tags=9"))
    expect(filterChip("trap")).toBeInTheDocument()
    expect(filterChip("rap")).not.toBeInTheDocument()
  })

  it("de-duplicates when both sides of a merge were selected", async () => {
    mockListWatchlist.mockResolvedValue(rows)
    mockVocabulary()
    mockRenameTag.mockResolvedValueOnce({
      kind: "collision",
      target: trap,
      carrierCountAfterMerge: 3,
    })
    mockMergeTag.mockResolvedValueOnce({ ...trap, carrier_count: 3 })

    renderRoute(WatchlistWithProbe, "/", "/?tags=5,9")
    await screen.findByText("Bad Bunny")

    await openManageTags()
    await mergeRapIntoTrap()

    await waitFor(() => expect(search()).toBe("?tags=9"))
  })

  it("drops a deleted tag again when Back brings it into the URL", async () => {
    mockListWatchlist.mockResolvedValue(rows)
    mockVocabulary()
    mockDeleteTag.mockResolvedValueOnce({ carrier_count: 1 })

    renderRoute(WatchlistWithProbe, "/")
    await screen.findByText("Bad Bunny")

    await userEvent.click(screen.getByLabelText("Filter by tags, any of"))
    await userEvent.click(await screen.findByRole("option", { name: /^rap/ }))
    await userEvent.click(await screen.findByRole("option", { name: /^trap/ }))
    await waitFor(() => expect(search()).toBe("?tags=5%2C9"))
    await userEvent.keyboard("{Escape}")

    await openManageTags()
    await deleteTagInDialog("rap")
    await waitFor(() => expect(search()).toBe("?tags=9"))
    await userEvent.keyboard("{Escape}")

    await userEvent.click(
      screen.getByRole("button", { name: "history back", hidden: true })
    )

    await waitFor(() => expect(search()).toBe("?tags=9"))
    expect(filterChip("rap")).not.toBeInTheDocument()
  })

  it("keeps the filter, its chip, and the card when the last carrier is detached in-session", async () => {
    mockListWatchlist.mockResolvedValue(rows)
    mockDetachTag.mockResolvedValue(undefined)

    renderRoute(WatchlistWithProbe, "/", "/?tags=5")
    await screen.findByText("Bad Bunny")

    await userEvent.click(
      screen.getByRole("button", { name: "Remove tag rap from Bad Bunny" })
    )

    expect(search()).toBe("?tags=5")
    expect(filterChip("rap")).toBeInTheDocument()
    expect(shown()).toEqual(["Bad Bunny"])
    expect(mockDetachTag).toHaveBeenCalledWith(1, 5)
  })

  it("keeps the tag filter and the sticky added card across the refresh after an add", async () => {
    const drizzy = makeEntry({ id: 99, name: "Drizzy", mbid: "mbid-99" })
    mockListWatchlist
      .mockResolvedValueOnce(rows)
      .mockResolvedValueOnce([...rows, drizzy])
    vi.mocked(searchArtists).mockResolvedValue({
      query: "Drizzy",
      sources: {
        musicbrainz: {
          status: "ok",
          artists: [
            {
              source: "musicbrainz",
              id: "mbid-99",
              name: "Drizzy",
              disambiguation: null,
              country: null,
              type: "Person",
              image_url: null,
            },
          ],
        },
      },
    })
    mockAddWatchlist.mockResolvedValue(drizzy)

    renderRoute(WatchlistWithProbe, "/", "/?tags=5")
    await screen.findByText("Bad Bunny")

    await userEvent.type(
      screen.getByRole("textbox", { name: "Search artists" }),
      "Drizzy"
    )
    await userEvent.click(
      await screen.findByRole("button", { name: "Add to Watchlist" })
    )

    await screen.findByRole("button", { name: "Remove Drizzy from watchlist" })
    expect(search()).toBe("?tags=5")
    expect(filterChip("rap")).toBeInTheDocument()
    expect(shown()).toEqual(["Bad Bunny", "Drizzy"])
    expect(mockListWatchlist).toHaveBeenCalledTimes(2)
  })
})
