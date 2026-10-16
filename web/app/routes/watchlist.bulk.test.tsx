import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createRoutesStub } from "react-router"
import { toast } from "sonner"
import { beforeEach, describe, expect, it, vi } from "vitest"

import {
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
})
