import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { deleteTag, listTags, type TagSummary } from "~/lib/api"

import { ManageTagsDialog } from "./ManageTagsDialog"

vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  listTags: vi.fn(),
  deleteTag: vi.fn(),
}))
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

const mockListTags = vi.mocked(listTags)
const mockDeleteTag = vi.mocked(deleteTag)

function renderDialog() {
  const onOpenChange = vi.fn()
  const onLoaded = vi.fn()
  const onDeleted = vi.fn()
  render(
    <ManageTagsDialog
      open
      onOpenChange={onOpenChange}
      onLoaded={onLoaded}
      onDeleted={onDeleted}
    />
  )
  return { onOpenChange, onLoaded, onDeleted }
}

describe("ManageTagsDialog", () => {
  it("shows three skeleton rows while loading", () => {
    mockListTags.mockReturnValue(new Promise(() => {}))

    renderDialog()

    expect(screen.getByText("Manage tags")).toBeInTheDocument()
    expect(document.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(3)
  })

  it("shows the error state with a Retry action that refetches", async () => {
    mockListTags.mockRejectedValueOnce(new Error("network down"))

    renderDialog()

    await screen.findByRole("heading", { name: "Couldn't load tags." })
    expect(screen.getByText("Please try again.")).toBeInTheDocument()

    mockListTags.mockResolvedValueOnce([])
    await userEvent.click(screen.getByRole("button", { name: "Retry" }))

    await screen.findByRole("heading", { name: "No tags yet" })
    expect(mockListTags).toHaveBeenCalledTimes(2)
  })

  it("shows the empty-vocabulary state", async () => {
    mockListTags.mockResolvedValue([])

    renderDialog()

    await screen.findByRole("heading", { name: "No tags yet" })
    expect(
      screen.getByText(
        "Add a tag from any artist's card on the Watchlist and it will appear here."
      )
    ).toBeInTheDocument()
  })

  it("renders rows sorted case-insensitively by name, with zero-artist rows shown (not hidden) and correct pluralization", async () => {
    const vocabulary: TagSummary[] = [
      { id: 3, name: "reggaeton", carrier_count: 12 },
      { id: 1, name: "Drill", carrier_count: 1 },
      { id: 2, name: "latin", carrier_count: 0 },
    ]
    mockListTags.mockResolvedValue(vocabulary)

    const { onLoaded } = renderDialog()

    await screen.findByRole("list", { name: "Tags" })
    const rows = screen.getAllByRole("listitem")
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveTextContent("Drill")
    expect(rows[0]).toHaveTextContent("· 1 artist")
    expect(rows[1]).toHaveTextContent("latin")
    expect(rows[1]).toHaveTextContent("· 0 artists")
    expect(rows[2]).toHaveTextContent("reggaeton")
    expect(rows[2]).toHaveTextContent("· 12 artists")

    expect(onLoaded).toHaveBeenCalledWith([
      { id: 1, name: "Drill", carrier_count: 1 },
      { id: 2, name: "latin", carrier_count: 0 },
      { id: 3, name: "reggaeton", carrier_count: 12 },
    ])
  })

  it("delete confirm calls deleteTag, toasts with the server's count, removes the row, and calls onDeleted", async () => {
    mockListTags.mockResolvedValue([{ id: 1, name: "drill", carrier_count: 3 }])
    mockDeleteTag.mockResolvedValueOnce({ carrier_count: 3 })

    const { onDeleted } = renderDialog()

    await screen.findByText("drill")
    await userEvent.click(
      screen.getByRole("button", { name: "Delete tag drill" })
    )

    const confirmButton = await screen.findByRole("button", {
      name: "Delete tag",
    })
    await userEvent.click(confirmButton)

    await waitFor(() => expect(mockDeleteTag).toHaveBeenCalledWith(1))
    await waitFor(() => expect(onDeleted).toHaveBeenCalledWith(1))
    await waitFor(() =>
      expect(screen.queryByText("drill")).not.toBeInTheDocument()
    )
  })

  it("uses the n = 0 delete confirm copy for a zero-carrier tag", async () => {
    mockListTags.mockResolvedValue([{ id: 1, name: "drill", carrier_count: 0 }])

    renderDialog()

    await screen.findByText("drill")
    await userEvent.click(
      screen.getByRole("button", { name: "Delete tag drill" })
    )

    expect(await screen.findByText("Delete “drill”?")).toBeInTheDocument()
    expect(
      screen.getByText(
        "No artists on your watchlist carry this tag, and this can't be undone."
      )
    ).toBeInTheDocument()
  })
})
