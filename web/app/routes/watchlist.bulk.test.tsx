import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createRoutesStub } from "react-router"
import { toast } from "sonner"
import { beforeEach, describe, expect, it, vi } from "vitest"

import {
  ApiError,
  type WatchlistEntry,
  bulkRemoveWatchlist,
  listWatchlist,
} from "~/lib/api"
import { makeEntry } from "~/lib/test/fixtures"

import Watchlist from "./watchlist"

// Every function export is stubbed so no real apiFetch can run; ApiError stays real.
vi.mock("~/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/api")>()
  return Object.fromEntries(
    Object.entries(actual).map(([key, value]) => [
      key,
      key === "ApiError" || typeof value !== "function" ? value : vi.fn(),
    ])
  )
})
vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() },
}))

const mockList = vi.mocked(listWatchlist)
const mockBulkRemove = vi.mocked(bulkRemoveWatchlist)

async function renderWatchlist(entries: WatchlistEntry[]) {
  mockList.mockResolvedValue(entries)
  const Stub = createRoutesStub([{ path: "/", Component: Watchlist }])
  render(<Stub initialEntries={["/"]} />)
  await screen.findByRole("checkbox", { name: `Select ${entries[0].name}` })
}

function removeOutcome(removed: number[], gone: number[] = []) {
  mockBulkRemove.mockResolvedValue({ removed, already_gone: gone })
}

const bar = () => screen.queryByRole("region", { name: "Selection actions" })

async function check(...names: string[]) {
  for (const name of names) {
    await userEvent.click(
      screen.getByRole("checkbox", { name: `Select ${name}` })
    )
  }
}

const trio = () => [
  makeEntry({ id: 1, name: "Drake" }),
  makeEntry({ id: 2, name: "SZA" }),
  makeEntry({ id: 3, name: "Bad Bunny" }),
]

beforeEach(() => {
  vi.clearAllMocks()
})

describe("Watchlist bulk remove", () => {
  it("removes two checked cards with one confirmed request", async () => {
    const entries = trio()
    await renderWatchlist(entries)
    removeOutcome([1, 2])
    mockList.mockResolvedValue([entries[2]])

    await check("Drake", "SZA")
    const region = screen.getByRole("region", { name: "Selection actions" })
    expect(within(region).getByText("2 selected")).toBeInTheDocument()

    await userEvent.click(
      within(region).getByRole("button", { name: "Remove" })
    )
    const dialog = await screen.findByRole("alertdialog")
    expect(
      within(dialog).getByText("Remove 2 artists from your watchlist?")
    ).toBeInTheDocument()
    expect(dialog.textContent).toContain(
      "Drake and SZA. Their notes are deleted"
    )

    await userEvent.click(
      within(dialog).getByRole("button", { name: "Remove 2 artists" })
    )

    await waitFor(() => expect(bar()).not.toBeInTheDocument())
    expect(mockBulkRemove).toHaveBeenCalledTimes(1)
    expect(mockBulkRemove).toHaveBeenCalledWith([1, 2])
    expect(
      screen.queryByRole("checkbox", { name: "Select Drake" })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole("checkbox", { name: "Select SZA" })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole("checkbox", { name: "Select Bad Bunny" })
    ).toHaveAttribute("aria-checked", "false")
    expect(toast.success).toHaveBeenCalledWith(
      "Removed 2 artists from your watchlist."
    )
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2))
  })

  const SINGULAR =
    "Their note is deleted but their tags are kept; alerts already found for them may still be sent, and re-adding them later may alert you about releases from while they were removed."
  const PLURAL_TAIL =
    "Their notes are deleted but their tags are kept; alerts already found for them may still be sent, and re-adding any of them later may alert you about releases from while they were removed."

  async function openRemove() {
    await userEvent.click(screen.getByRole("button", { name: "Remove" }))
    return screen.findByRole("alertdialog")
  }

  it("words the dialog for a single artist", async () => {
    await renderWatchlist(trio())
    await check("Drake")

    const dialog = await openRemove()
    expect(
      within(dialog).getByText("Remove Drake from your watchlist?")
    ).toBeInTheDocument()
    expect(within(dialog).getByText(SINGULAR)).toBeInTheDocument()
    expect(
      within(dialog).getByRole("button", { name: "Remove 1 artist" })
    ).toBeInTheDocument()
  })

  it("lists three names and a remainder for fourteen selected artists", async () => {
    const entries = Array.from({ length: 14 }, (_, i) =>
      makeEntry({ id: i + 1, name: `Artist ${String(i).padStart(2, "0")}` })
    )
    await renderWatchlist(entries)
    for (const box of screen.getAllByRole("checkbox", {
      name: /^Select Artist/,
    }))
      fireEvent.click(box)

    const dialog = await openRemove()
    const text = within(dialog).getByText(/and 11 more\./).textContent ?? ""
    expect(
      text.startsWith("Artist 00, Artist 01, Artist 02 and 11 more.")
    ).toBe(true)
    expect(text.endsWith(PLURAL_TAIL)).toBe(true)
    expect(
      within(dialog).getByText("Remove 14 artists from your watchlist?")
    ).toBeInTheDocument()
  }, 30000)

  it("counts already-gone ids in the toast, which carries no action", async () => {
    const entries = trio()
    await renderWatchlist(entries)
    removeOutcome([1], [2])
    mockList.mockResolvedValue([entries[2]])
    await check("Drake", "SZA")

    const dialog = await openRemove()
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Remove 2 artists" })
    )

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1))
    expect(toast.success).toHaveBeenCalledWith(
      "Removed 2 artists from your watchlist.",
      { description: "1 artist was already removed from your watchlist." }
    )
    const options = vi.mocked(toast.success).mock.calls[0][1]
    expect(options).not.toHaveProperty("action")
  })

  it("keeps the cards and selection when the request fails", async () => {
    await renderWatchlist(trio())
    mockBulkRemove.mockRejectedValue(new Error("network"))
    await check("Drake", "SZA")

    const dialog = await openRemove()
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Remove 2 artists" })
    )

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Couldn't remove the selected artists — nothing was removed. Try again."
      )
    )
    await waitFor(() =>
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument()
    )
    expect(screen.getByRole("checkbox", { name: "Select Drake" })).toBeChecked()
    expect(screen.getByText("2 selected")).toBeInTheDocument()
    expect(mockList).toHaveBeenCalledTimes(1)
  })

  it("reports the server's id cap and keeps the selection", async () => {
    await renderWatchlist(trio())
    mockBulkRemove.mockRejectedValue(
      new ApiError(
        400,
        "too many ids",
        { error: "too many ids", code: "bulk_too_many_ids", max: 1000 },
        "bulk_too_many_ids"
      )
    )
    await check("Drake")

    const dialog = await openRemove()
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Remove 1 artist" })
    )

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Too many artists selected — select at most 1000 and try again."
      )
    )
    expect(screen.getByText("1 selected")).toBeInTheDocument()
  })

  it("sends nothing on Cancel and keeps the selection", async () => {
    await renderWatchlist(trio())
    await check("Drake", "SZA")

    const dialog = await openRemove()
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Cancel" })
    )

    await waitFor(() =>
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument()
    )
    expect(mockBulkRemove).not.toHaveBeenCalled()
    expect(screen.getByText("2 selected")).toBeInTheDocument()
  })

  it("clears the selection from the bar without sending anything", async () => {
    await renderWatchlist(trio())
    await check("Drake")

    await userEvent.click(
      screen.getByRole("button", { name: "Clear selection" })
    )

    expect(bar()).not.toBeInTheDocument()
    expect(mockBulkRemove).not.toHaveBeenCalled()
    expect(
      screen.getByRole("checkbox", { name: "Select Drake" })
    ).not.toBeChecked()
  })
})

const filterBox = () =>
  screen.getByRole("textbox", { name: "Filter watchlist" })
const selectAll = () =>
  screen.getByRole("checkbox", { name: "Select all visible" })

describe("Watchlist select all visible", () => {
  it("selects only the filtered view and removes exactly it", async () => {
    await renderWatchlist(trio())
    removeOutcome([1])
    await check("SZA")

    await userEvent.type(filterBox(), "dra")
    expect(bar()).not.toBeInTheDocument()

    await userEvent.click(selectAll())
    expect(screen.getByText("1 selected")).toBeInTheDocument()

    await userEvent.clear(filterBox())
    expect(screen.getByText("2 selected")).toBeInTheDocument()
    expect(selectAll()).toHaveAttribute("aria-checked", "mixed")

    await userEvent.type(filterBox(), "dra")
    await userEvent.click(screen.getByRole("button", { name: "Remove" }))
    const dialog = await screen.findByRole("alertdialog")
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Remove 1 artist" })
    )

    await waitFor(() => expect(mockBulkRemove).toHaveBeenCalledTimes(1))
    expect(mockBulkRemove).toHaveBeenCalledWith([1])
    await waitFor(() =>
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument()
    )

    await userEvent.clear(filterBox())
    expect(
      screen.getByRole("checkbox", { name: "Select SZA" })
    ).toHaveAttribute("aria-checked", "false")
  })
  it("keeps a hidden selection inert and brings it back with the filter", async () => {
    await renderWatchlist(trio())
    await check("Drake", "SZA")

    await userEvent.type(filterBox(), "dra")
    expect(screen.getByText("1 selected")).toBeInTheDocument()

    await userEvent.clear(filterBox())
    expect(screen.getByText("2 selected")).toBeInTheDocument()
  })

  it("keeps a hidden selection when select all is checked then unchecked", async () => {
    await renderWatchlist(trio())
    await check("SZA")
    await userEvent.type(filterBox(), "dra")

    await userEvent.click(selectAll())
    expect(selectAll()).toHaveAttribute("aria-checked", "true")
    await userEvent.click(selectAll())
    expect(bar()).not.toBeInTheDocument()

    await userEvent.clear(filterBox())
    expect(screen.getByText("1 selected")).toBeInTheDocument()
    expect(screen.getByRole("checkbox", { name: "Select SZA" })).toBeChecked()
  })

  it("disables select all when the filter matches nothing", async () => {
    await renderWatchlist(trio())
    await userEvent.type(filterBox(), "zzz")

    expect(selectAll()).toHaveAttribute("data-disabled")
    expect(screen.getByText("No artists match these filters")).toBeVisible()
  })

  it("never shows indeterminate for a single card", async () => {
    await renderWatchlist([makeEntry({ id: 1, name: "Drake" })])
    expect(selectAll()).toHaveAttribute("aria-checked", "false")
    await check("Drake")
    expect(selectAll()).toHaveAttribute("aria-checked", "true")
  })

  it("shows mixed when only some visible cards are selected", async () => {
    await renderWatchlist(trio())
    await check("Drake")
    expect(selectAll()).toHaveAttribute("aria-checked", "mixed")

    await userEvent.click(selectAll())
    expect(screen.getByText("3 selected")).toBeInTheDocument()
  })

  it("empties hidden selections too when Clear is pressed", async () => {
    await renderWatchlist(trio())
    await check("SZA")
    await userEvent.type(filterBox(), "dra")
    await check("Drake")

    await userEvent.click(
      screen.getByRole("button", { name: "Clear selection" })
    )
    await userEvent.clear(filterBox())

    for (const name of ["Drake", "SZA", "Bad Bunny"]) {
      expect(
        screen.getByRole("checkbox", { name: `Select ${name}` })
      ).toHaveAttribute("aria-checked", "false")
    }
  })
})
