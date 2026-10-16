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
  bulkAddTag,
  bulkRemoveWatchlist,
  listTags,
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
const mockAddTag = vi.mocked(bulkAddTag)
const mockListTags = vi.mocked(listTags)

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
  }, 30000)

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

async function shiftClick(name: string) {
  const user = userEvent.setup()
  await user.keyboard("{Shift>}")
  await user.click(screen.getByRole("checkbox", { name: `Select ${name}` }))
  await user.keyboard("{/Shift}")
}

describe("Watchlist shift ranges and in-flight lock", () => {
  it("selects the range in visible order, not id order", async () => {
    await renderWatchlist(trio())
    await check("Bad Bunny")
    await shiftClick("SZA")

    expect(screen.getByText("3 selected")).toBeInTheDocument()
    for (const name of ["Bad Bunny", "Drake", "SZA"]) {
      expect(
        screen.getByRole("checkbox", { name: `Select ${name}` })
      ).toBeChecked()
    }
  })

  it("acts as a plain toggle with no anchor", async () => {
    await renderWatchlist(trio())
    await shiftClick("SZA")

    expect(screen.getByText("1 selected")).toBeInTheDocument()
  })

  it("resets the anchor when the filter changes", async () => {
    await renderWatchlist(trio())
    await check("Bad Bunny")
    await userEvent.type(filterBox(), "a")
    await shiftClick("SZA")

    expect(screen.getByText("2 selected")).toBeInTheDocument()
    expect(
      screen.getByRole("checkbox", { name: "Select Drake" })
    ).not.toBeChecked()
  })

  it("resets the anchor when the sort changes", async () => {
    await renderWatchlist(trio())
    await check("Bad Bunny")
    await userEvent.click(screen.getByRole("combobox", { name: "Sort by" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "Name (Z–A)" })
    )
    await shiftClick("SZA")

    expect(screen.getByText("2 selected")).toBeInTheDocument()
    expect(
      screen.getByRole("checkbox", { name: "Select Drake" })
    ).not.toBeChecked()
  })

  it("locks every selection control while a bulk request is in flight", async () => {
    await renderWatchlist(trio())
    let settle!: (v: { removed: number[]; already_gone: number[] }) => void
    mockBulkRemove.mockReturnValue(new Promise((res) => (settle = res)))
    await check("Drake")

    const dialog = await openBulkRemove()
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Remove 1 artist" })
    )

    const pending = await within(dialog).findByRole("button", {
      name: "Removing…",
    })
    expect(pending).toBeDisabled()
    expect(
      within(dialog).getByRole("button", { name: "Cancel" })
    ).toBeDisabled()
    const lock = { hidden: true } as const
    expect(selectAllHidden()).toHaveAttribute("data-disabled")
    for (const name of ["Drake", "SZA", "Bad Bunny"]) {
      expect(
        screen.getByRole("checkbox", { name: `Select ${name}`, ...lock })
      ).toHaveAttribute("data-disabled")
    }
    expect(
      screen.getByRole("textbox", { name: "Filter watchlist", hidden: true })
    ).not.toBeDisabled()

    settle({ removed: [1], already_gone: [] })
    await waitFor(() =>
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument()
    )
  })
})

async function openBulkRemove() {
  await userEvent.click(screen.getByRole("button", { name: "Remove" }))
  return screen.findByRole("alertdialog")
}

const selectAllHidden = () =>
  screen.getByRole("checkbox", { name: "Select all visible", hidden: true })

describe("Watchlist focus after bulk actions", () => {
  const removeBtn = () => screen.getByRole("button", { name: "Remove" })

  async function removeSelected(label: string) {
    const dialog = await openBulkRemove()
    await userEvent.click(within(dialog).getByRole("button", { name: label }))
  }

  it("focuses Select all visible when artists remain", async () => {
    const entries = trio()
    await renderWatchlist(entries)
    removeOutcome([1, 2])
    mockList.mockResolvedValue([entries[2]])
    await check("Drake", "SZA")
    await removeSelected("Remove 2 artists")

    await waitFor(() => expect(bar()).not.toBeInTheDocument())
    await waitFor(() => expect(selectAll()).toHaveFocus())
  })

  it("focuses Clear filters when the removal empties the filtered view", async () => {
    const entries = trio()
    await renderWatchlist(entries)
    removeOutcome([1])
    mockList.mockResolvedValue([entries[1], entries[2]])
    await userEvent.type(filterBox(), "dra")
    await userEvent.click(selectAll())
    await removeSelected("Remove 1 artist")

    const clear = await screen.findByRole("button", { name: "Clear filters" })
    await waitFor(() => expect(clear).toHaveFocus())
  })

  it("focuses the page heading when every artist is removed", async () => {
    const entries = [makeEntry({ id: 1, name: "Drake" })]
    await renderWatchlist(entries)
    removeOutcome([1])
    mockList.mockResolvedValue([])
    await check("Drake")
    await removeSelected("Remove 1 artist")

    expect(await screen.findByText("No artists yet")).toBeInTheDocument()
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { level: 1, name: "Watchlist" })
      ).toHaveFocus()
    )
  })

  it("returns focus to Remove on Cancel", async () => {
    await renderWatchlist(trio())
    await check("Drake")
    const dialog = await openBulkRemove()
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Cancel" })
    )

    await waitFor(() => expect(removeBtn()).toHaveFocus())
  })

  it("returns focus to Remove on Esc", async () => {
    await renderWatchlist(trio())
    await check("Drake")
    await openBulkRemove()
    await userEvent.keyboard("{Escape}")

    await waitFor(() => expect(removeBtn()).toHaveFocus())
    expect(screen.getByText("1 selected")).toBeInTheDocument()
  })

  it("returns focus to Remove when the request fails", async () => {
    await renderWatchlist(trio())
    mockBulkRemove.mockRejectedValue(new Error("network"))
    await check("Drake")
    await removeSelected("Remove 1 artist")

    await waitFor(() => expect(removeBtn()).toHaveFocus())
    expect(screen.getByText("1 selected")).toBeInTheDocument()
  })

  it("focuses Select all visible after Clear", async () => {
    await renderWatchlist(trio())
    await check("Drake")
    await userEvent.click(
      screen.getByRole("button", { name: "Clear selection" })
    )

    await waitFor(() => expect(selectAll()).toHaveFocus())
  })
})

describe("Watchlist selection announcements", () => {
  const status = () => screen.getByRole("status")
  const FIRST = "1 selected. Selection actions follow the artist list."

  it("announces when the first card is checked, and not for the next", async () => {
    await renderWatchlist(trio())
    await check("Drake")
    expect(status()).toHaveTextContent(FIRST)

    await userEvent.click(screen.getByRole("combobox", { name: "Sort by" }))
    await userEvent.click(
      await screen.findByRole("option", { name: "Name (Z–A)" })
    )
    expect(status()).toHaveTextContent("Sorted by")

    await check("SZA")
    expect(status()).toHaveTextContent("Sorted by")
  })

  it("announces the visible count for Select all visible", async () => {
    await renderWatchlist(trio())
    await userEvent.click(selectAll())
    expect(status()).toHaveTextContent("Selected 3 visible artists.")

    await userEvent.click(selectAll())
    expect(status()).toHaveTextContent("Selection cleared.")

    await userEvent.type(filterBox(), "dra")
    await userEvent.click(selectAll())
    expect(status()).toHaveTextContent("Selected 1 visible artist.")
  })

  it("announces Selection cleared on Clear", async () => {
    await renderWatchlist(trio())
    await check("Drake")
    await userEvent.click(
      screen.getByRole("button", { name: "Clear selection" })
    )
    expect(status()).toHaveTextContent("Selection cleared.")
  })

  it("clears the whole selection on Esc in the bar and refocuses", async () => {
    await renderWatchlist(trio())
    await check("SZA")
    await userEvent.type(filterBox(), "dra")
    await check("Drake")

    screen.getByRole("button", { name: "Remove" }).focus()
    await userEvent.keyboard("{Escape}")

    expect(bar()).not.toBeInTheDocument()
    expect(status()).toHaveTextContent("Selection cleared.")
    await waitFor(() => expect(selectAll()).toHaveFocus())
    await userEvent.clear(filterBox())
    expect(
      screen.getByRole("checkbox", { name: "Select SZA" })
    ).not.toBeChecked()
  })

  it("appends the selected count to the filter announcement", async () => {
    await renderWatchlist(trio())
    await check("Drake", "SZA")
    await userEvent.type(filterBox(), "dra")

    await waitFor(
      () =>
        expect(status()).toHaveTextContent(
          "Showing 1 of 3 artists. 1 selected."
        ),
      { timeout: 2000 }
    )
  })

  it("leaves the suffix off when nothing is selected", async () => {
    await renderWatchlist(trio())
    await userEvent.type(filterBox(), "dra")

    await waitFor(
      () => expect(status()).toHaveTextContent("Showing 1 of 3 artists."),
      { timeout: 2000 }
    )
    expect(status()).not.toHaveTextContent("selected.")
  })

  it("does not write Selection cleared after a bulk remove", async () => {
    const entries = trio()
    await renderWatchlist(entries)
    removeOutcome([1])
    mockList.mockResolvedValue([entries[1], entries[2]])
    await check("Drake")
    const dialog = await openBulkRemove()
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Remove 1 artist" })
    )

    await waitFor(() => expect(bar()).not.toBeInTheDocument())
    expect(status()).not.toHaveTextContent("Selection cleared.")
  })
})

describe("Watchlist bar clearance", () => {
  it("pads the page bottom by the bar clearance", async () => {
    await renderWatchlist(trio())
    const page = screen
      .getByRole("heading", { level: 1, name: "Watchlist" })
      .closest("div")?.parentElement
    expect(page).toHaveClass("pb-[calc(2rem+var(--bulk-bar-clearance,0px))]")
  })

  it("sets the clearance variable only while the bar is mounted", async () => {
    await renderWatchlist(trio())
    const read = () =>
      document.documentElement.style.getPropertyValue("--bulk-bar-clearance")
    expect(read()).toBe("")

    await check("Drake")
    expect(read()).not.toBe("")

    await userEvent.click(
      screen.getByRole("button", { name: "Clear selection" })
    )
    expect(read()).toBe("")
  })
})

describe("Watchlist bulk add tag", () => {
  // Types a new tag into the bar's inline picker and picks Create.
  async function createFromBar(count: number, tag: string) {
    await userEvent.click(screen.getByRole("button", { name: "Add tag" }))
    const input = await screen.findByRole("combobox", {
      name: `Add tag to ${count} selected artist${count === 1 ? "" : "s"}`,
    })
    await userEvent.type(input, tag)
    await userEvent.click(
      await screen.findByRole("option", { name: `Create “${tag}”` })
    )
  }

  it("tags two selected artists from the inline picker in one request", async () => {
    await renderWatchlist(trio())
    mockListTags.mockResolvedValue([])
    mockAddTag.mockResolvedValue({
      tag: { id: 7, name: "rap" },
      applied: [1, 2],
      skipped: [],
    })
    await check("Drake", "SZA")
    expect(mockListTags).not.toHaveBeenCalled()

    await createFromBar(2, "rap")

    await waitFor(() => expect(bar()).not.toBeInTheDocument())
    expect(mockListTags).toHaveBeenCalledTimes(1)
    expect(mockAddTag).toHaveBeenCalledTimes(1)
    expect(mockAddTag).toHaveBeenCalledWith([1, 2], "rap")
    expect(toast.success).toHaveBeenCalledWith("Added “rap” to 2 artists.")
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(selectAll()).toHaveFocus())
  })

  const RAP = { id: 7, name: "rap" }
  const GONE_ONE = "1 artist was already removed from your watchlist."

  async function addRap(
    names: string[],
    outcome: Parameters<typeof mockAddTag.mockResolvedValue>[0]
  ) {
    mockListTags.mockResolvedValue([])
    mockAddTag.mockResolvedValue(outcome)
    await check(...names)
    await createFromBar(names.length, "rap")
    await waitFor(() => expect(bar()).not.toBeInTheDocument())
  }

  it("reports a plain success without a warning when every artist took the tag", async () => {
    await renderWatchlist(trio())
    await addRap(["Drake", "SZA"], { tag: RAP, applied: [1, 2], skipped: [] })

    expect(toast.success).toHaveBeenCalledWith("Added “rap” to 2 artists.")
    expect(toast.warning).not.toHaveBeenCalled()
  })

  it("warns with the at-cap artist named when only some took the tag", async () => {
    await renderWatchlist(trio())
    await addRap(["Drake", "SZA"], {
      tag: RAP,
      applied: [1],
      skipped: [{ id: 2, name: "SZA", reason: "cap" }],
    })

    expect(toast.warning).toHaveBeenCalledWith("Added “rap” to 1 artist.", {
      description: "1 already has 10 tags: SZA.",
      duration: 8000,
    })
    expect(toast.success).not.toHaveBeenCalled()
  })

  it("names at-cap artists in visible order, not the id order the server uses", async () => {
    const entries = [
      makeEntry({ id: 1, name: "Drake" }),
      makeEntry({ id: 2, name: "SZA" }),
      makeEntry({ id: 3, name: "Bad Bunny" }),
      makeEntry({ id: 4, name: "Future" }),
    ]
    await renderWatchlist(entries)
    await addRap(["Bad Bunny", "Drake", "Future", "SZA"], {
      tag: RAP,
      applied: [1],
      // Future carries no server name, so its entry name is used.
      skipped: [
        { id: 2, name: "SZA", reason: "cap" },
        { id: 3, name: "Bad Bunny", reason: "cap" },
        { id: 4, reason: "cap" },
      ],
    })

    expect(mockAddTag).toHaveBeenCalledWith([3, 1, 4, 2], "rap")
    expect(toast.warning).toHaveBeenCalledWith("Added “rap” to 1 artist.", {
      description: "3 already have 10 tags: Bad Bunny, Future and SZA.",
      duration: 8000,
    })
  })

  it("explains an all-at-cap outcome and does not teach the vocabulary the tag", async () => {
    await renderWatchlist(trio())
    await addRap(["Drake", "SZA"], {
      tag: { id: 0, name: "rap" },
      applied: [],
      skipped: [
        { id: 1, name: "Drake", reason: "cap" },
        { id: 2, name: "SZA", reason: "cap" },
      ],
    })

    expect(toast.warning).toHaveBeenCalledWith(
      "Couldn't add “rap” — every selected artist already has 10 tags.",
      { description: "Drake and SZA.", duration: 8000 }
    )

    await check("Drake")
    await userEvent.click(screen.getByRole("button", { name: "Add tag" }))
    await userEvent.type(
      await screen.findByRole("combobox", {
        name: "Add tag to 1 selected artist",
      }),
      "rap"
    )
    expect(
      await screen.findByRole("option", { name: "Create “rap”" })
    ).toBeInTheDocument()
  })

  it("appends the gone sentence to a success, or to the cap sentence", async () => {
    await renderWatchlist(trio())
    await addRap(["Drake", "SZA"], {
      tag: RAP,
      applied: [1],
      skipped: [{ id: 2, reason: "gone" }],
    })
    expect(toast.success).toHaveBeenCalledWith("Added “rap” to 1 artist.", {
      description: GONE_ONE,
    })

    await check("Bad Bunny", "Drake", "SZA")
    mockAddTag.mockResolvedValue({
      tag: RAP,
      applied: [3],
      skipped: [
        { id: 2, name: "SZA", reason: "cap" },
        { id: 1, reason: "gone" },
      ],
    })
    // The vocabulary now holds `rap`, so the second pick is an existing option.
    await userEvent.click(screen.getByRole("button", { name: "Add tag" }))
    await userEvent.type(
      await screen.findByRole("combobox", {
        name: "Add tag to 3 selected artists",
      }),
      "rap"
    )
    await userEvent.click(await screen.findByRole("option", { name: "rap" }))
    await waitFor(() => expect(toast.warning).toHaveBeenCalledTimes(1))
    expect(toast.warning).toHaveBeenCalledWith("Added “rap” to 1 artist.", {
      description: `1 already has 10 tags: SZA. ${GONE_ONE}`,
      duration: 8000,
    })
  })

  it("reports zero when every selected artist was already removed", async () => {
    await renderWatchlist(trio())
    await addRap(["Drake", "SZA"], {
      tag: { id: 0, name: "rap" },
      applied: [],
      skipped: [
        { id: 1, reason: "gone" },
        { id: 2, reason: "gone" },
      ],
    })

    expect(toast.success).toHaveBeenCalledWith("Added “rap” to 0 artists.", {
      description: "2 artists were already removed from your watchlist.",
    })
  })

  it("keeps the selection and returns focus to Add tag when the request fails", async () => {
    await renderWatchlist(trio())
    mockListTags.mockResolvedValue([])
    mockAddTag.mockRejectedValue(new Error("network"))
    await check("Drake", "SZA")
    await createFromBar(2, "rap")

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Couldn't add “rap” to the selected artists — try again."
      )
    )
    expect(screen.getByText("2 selected")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Add tag" })).toHaveFocus()
    )
    expect(mockList).toHaveBeenCalledTimes(1)
  })

  it("reports the id cap of the server instead of the generic failure", async () => {
    await renderWatchlist(trio())
    mockListTags.mockResolvedValue([])
    mockAddTag.mockRejectedValue(
      new ApiError(
        400,
        "too many ids",
        { error: "too many ids", code: "bulk_too_many_ids", max: 1000 },
        "bulk_too_many_ids"
      )
    )
    await check("Drake")
    await createFromBar(1, "rap")

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Too many artists selected — select at most 1000 and try again."
      )
    )
    expect(screen.getByText("1 selected")).toBeInTheDocument()
  })

  it("keeps an affected card visible past the tag filter until the next filter change", async () => {
    const old = { id: 5, name: "old" }
    mockList.mockResolvedValue([
      makeEntry({ id: 1, name: "Drake", tags: [old] }),
      makeEntry({ id: 2, name: "SZA", tags: [old] }),
    ])
    const Stub = createRoutesStub([{ path: "/", Component: Watchlist }])
    render(<Stub initialEntries={["/?tags=5"]} />)
    await screen.findByRole("checkbox", { name: "Select Drake" })

    mockListTags.mockResolvedValue([{ ...old, carrier_count: 2 }])
    mockAddTag.mockResolvedValue({ tag: RAP, applied: [1, 2], skipped: [] })
    // Another tab dropped `old` from Drake before the refresh lands.
    mockList.mockResolvedValue([
      makeEntry({ id: 1, name: "Drake", tags: [RAP] }),
      makeEntry({ id: 2, name: "SZA", tags: [old, RAP] }),
    ])
    await check("Drake", "SZA")
    await createFromBar(2, "rap")

    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2))
    const drake = await screen.findByRole("checkbox", { name: "Select Drake" })
    await waitFor(() => expect(drake.closest("li")).toHaveTextContent("rap"))
    expect(drake.closest("li")).not.toHaveTextContent("old")

    await userEvent.type(filterBox(), "a")
    await waitFor(() =>
      expect(
        screen.queryByRole("checkbox", { name: "Select Drake" })
      ).not.toBeInTheDocument()
    )
    expect(
      screen.getByRole("checkbox", { name: "Select SZA" })
    ).toBeInTheDocument()
  })
})
