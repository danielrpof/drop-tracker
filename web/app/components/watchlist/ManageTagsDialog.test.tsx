import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import {
  deleteTag,
  listTags,
  mergeTag,
  renameTag,
  type TagSummary,
} from "~/lib/api"

import { ManageTagsDialog } from "./ManageTagsDialog"

vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  listTags: vi.fn(),
  deleteTag: vi.fn(),
  renameTag: vi.fn(),
  mergeTag: vi.fn(),
}))
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

const mockListTags = vi.mocked(listTags)
const mockDeleteTag = vi.mocked(deleteTag)
const mockRenameTag = vi.mocked(renameTag)
const mockMergeTag = vi.mocked(mergeTag)

function renderDialog() {
  const onOpenChange = vi.fn()
  const onLoaded = vi.fn()
  const onDeleted = vi.fn()
  const onRenamed = vi.fn()
  const onMerged = vi.fn()
  render(
    <ManageTagsDialog
      open
      onOpenChange={onOpenChange}
      onLoaded={onLoaded}
      onDeleted={onDeleted}
      onRenamed={onRenamed}
      onMerged={onMerged}
    />
  )
  return { onOpenChange, onLoaded, onDeleted, onRenamed, onMerged }
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

  it("opens rename mode with the current name selected, and disables Save for the same name (casing included) or blank", async () => {
    mockListTags.mockResolvedValue([{ id: 3, name: "Latin", carrier_count: 4 }])

    renderDialog()

    await screen.findByText("Latin")
    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag Latin" })
    )

    const input = screen.getByRole("textbox", { name: "New name for Latin" })
    expect(input).toHaveValue("Latin")
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled()

    await userEvent.clear(input)
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled()

    await userEvent.type(input, "Latin")
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled()

    await userEvent.type(input, "o")
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled()
  })

  it("renames a tag on Enter: shows Saving..., toasts, updates the row, refocuses Rename, and calls onRenamed", async () => {
    mockListTags.mockResolvedValue([{ id: 3, name: "Latin", carrier_count: 4 }])
    let resolveRename!: (v: {
      kind: "renamed"
      tag: { id: number; name: string }
    }) => void
    mockRenameTag.mockReturnValueOnce(
      new Promise((res) => {
        resolveRename = res
      })
    )

    const { onRenamed } = renderDialog()

    await screen.findByText("Latin")
    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag Latin" })
    )
    const input = screen.getByRole("textbox", { name: "New name for Latin" })
    await userEvent.clear(input)
    await userEvent.type(input, "latin{Enter}")

    await waitFor(() => expect(mockRenameTag).toHaveBeenCalledWith(3, "latin"))
    expect(input).toHaveAttribute("readonly")
    expect(screen.getByRole("button", { name: "Saving…" })).toBeDisabled()

    resolveRename({ kind: "renamed", tag: { id: 3, name: "latin" } })

    await waitFor(() => expect(screen.getByText("latin")).toBeInTheDocument())
    expect(screen.queryByText("Latin")).not.toBeInTheDocument()
    expect(onRenamed).toHaveBeenCalledWith({ id: 3, name: "latin" })
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Rename tag latin" })
      ).toHaveFocus()
    )
  })

  it("starting a rename on another row cancels the first and discards its text", async () => {
    mockListTags.mockResolvedValue([
      { id: 1, name: "drill", carrier_count: 2 },
      { id: 2, name: "trap", carrier_count: 5 },
    ])

    renderDialog()

    await screen.findByText("drill")
    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag drill" })
    )
    const firstInput = screen.getByRole("textbox", {
      name: "New name for drill",
    })
    await userEvent.clear(firstInput)
    await userEvent.type(firstInput, "unsaved")

    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag trap" })
    )

    expect(
      screen.queryByRole("textbox", { name: "New name for drill" })
    ).not.toBeInTheDocument()
    expect(screen.getByText("drill")).toBeInTheDocument()
    expect(
      screen.getByRole("textbox", { name: "New name for trap" })
    ).toBeInTheDocument()
  })

  it("Esc in the rename input cancels the rename, focuses that row's Rename, and leaves the dialog open", async () => {
    mockListTags.mockResolvedValue([{ id: 1, name: "drill", carrier_count: 2 }])

    renderDialog()

    await screen.findByText("drill")
    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag drill" })
    )
    const input = screen.getByRole("textbox", { name: "New name for drill" })
    await userEvent.type(input, "{Escape}")

    expect(
      screen.queryByRole("textbox", { name: "New name for drill" })
    ).not.toBeInTheDocument()
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Rename tag drill" })
      ).toHaveFocus()
    )
    expect(screen.getByText("Manage tags")).toBeInTheDocument()
    expect(mockRenameTag).not.toHaveBeenCalled()
  })

  it("keeps a rejected rename's input open with its text and toasts the failure", async () => {
    mockListTags.mockResolvedValue([{ id: 1, name: "drill", carrier_count: 2 }])
    mockRenameTag.mockRejectedValueOnce(new Error("network down"))

    renderDialog()

    await screen.findByText("drill")
    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag drill" })
    )
    const input = screen.getByRole("textbox", { name: "New name for drill" })
    await userEvent.clear(input)
    await userEvent.type(input, "trap{Enter}")

    await waitFor(() => expect(mockRenameTag).toHaveBeenCalled())
    await waitFor(() =>
      expect(
        screen.getByRole("textbox", { name: "New name for drill" })
      ).toHaveValue("trap")
    )
  })

  it("shows the {n}/32 counter once the rename input reaches 25 characters", async () => {
    mockListTags.mockResolvedValue([{ id: 1, name: "drill", carrier_count: 2 }])

    renderDialog()

    await screen.findByText("drill")
    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag drill" })
    )
    const input = screen.getByRole("textbox", { name: "New name for drill" })

    expect(screen.queryByText("24/32")).not.toBeInTheDocument()

    await userEvent.clear(input)
    await userEvent.type(input, "a".repeat(25))

    expect(screen.getByText("25/32")).toBeInTheDocument()
  })

  async function openCollisionConfirm() {
    mockListTags.mockResolvedValue([
      { id: 5, name: "rap", carrier_count: 3 },
      { id: 9, name: "trap", carrier_count: 4 },
    ])
    mockRenameTag.mockResolvedValueOnce({
      kind: "collision",
      target: { id: 9, name: "trap" },
      carrierCountAfterMerge: 7,
    })

    const result = renderDialog()

    await screen.findByText("rap")
    await userEvent.click(
      screen.getByRole("button", { name: "Rename tag rap" })
    )
    const input = screen.getByRole("textbox", { name: "New name for rap" })
    await userEvent.clear(input)
    await userEvent.type(input, "TRAP{Enter}")

    await screen.findByText("Merge “rap” into “trap”?")
    return { input, ...result }
  }

  it("a collision opens a merge ConfirmDialog naming both tags with focus on Cancel, without calling mergeTag", async () => {
    await openCollisionConfirm()

    expect(
      screen.getByText(
        "7 artists will carry “trap”, and “rap” will be deleted."
      )
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus()
    )
    expect(mockMergeTag).not.toHaveBeenCalled()
  })

  it("Cancel on the merge confirm returns to the still-open rename input with its text intact and focuses Save", async () => {
    const { input } = await openCollisionConfirm()

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }))

    expect(
      screen.queryByText("Merge “rap” into “trap”?")
    ).not.toBeInTheDocument()
    expect(input).toHaveValue("TRAP")
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Save" })).toHaveFocus()
    )
    expect(mockMergeTag).not.toHaveBeenCalled()
  })

  it("Esc on the merge confirm closes only it, leaving Manage tags open and the rename input intact", async () => {
    const { input } = await openCollisionConfirm()

    // Escape must land on the open confirm, not the rename input mid-refocus.
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus()
    )
    await userEvent.keyboard("{Escape}")

    await waitFor(() =>
      expect(
        screen.queryByText("Merge “rap” into “trap”?")
      ).not.toBeInTheDocument()
    )
    expect(screen.getByText("Manage tags")).toBeInTheDocument()
    expect(input).toHaveValue("TRAP")
    expect(mockMergeTag).not.toHaveBeenCalled()
  })

  it("focus cannot tab out of the open merge ConfirmDialog", async () => {
    await openCollisionConfirm()

    const cancelButton = screen.getByRole("button", { name: "Cancel" })
    const mergeButton = screen.getByRole("button", { name: "Merge tags" })
    await waitFor(() => expect(cancelButton).toHaveFocus())

    await userEvent.tab()
    expect(mergeButton).toHaveFocus()

    // base-ui's focus trap wraps through an inert sentinel guard before
    // landing back on Cancel -- wait for that redirect rather than
    // asserting on the guard's transient intermediate focus.
    await userEvent.tab()
    await waitFor(() => expect(cancelButton).toHaveFocus())
  })

  it("confirming the merge shows Merging..., disables both buttons, calls mergeTag by id, toasts, removes the source row, updates the target's count, and focuses the target's Rename", async () => {
    const { onMerged } = await openCollisionConfirm()
    let resolveMerge!: (v: {
      id: number
      name: string
      carrier_count: number
    }) => void
    mockMergeTag.mockReturnValueOnce(
      new Promise((res) => {
        resolveMerge = res
      })
    )

    await userEvent.click(screen.getByRole("button", { name: "Merge tags" }))

    await waitFor(() => expect(mockMergeTag).toHaveBeenCalledWith(5, 9))
    expect(screen.getByRole("button", { name: "Merging…" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled()

    resolveMerge({ id: 9, name: "trap", carrier_count: 7 })

    await waitFor(() =>
      expect(screen.queryByText("rap")).not.toBeInTheDocument()
    )
    expect(screen.getByText("trap")).toBeInTheDocument()
    expect(screen.getByText("· 7 artists")).toBeInTheDocument()
    expect(onMerged).toHaveBeenCalledWith(5, {
      id: 9,
      name: "trap",
      carrier_count: 7,
    })
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Rename tag trap" })
      ).toHaveFocus()
    )
  })

  it("a rejected merge closes the ConfirmDialog, toasts the failure, and re-fetches the list", async () => {
    await openCollisionConfirm()
    mockMergeTag.mockRejectedValueOnce(new Error("network down"))
    mockListTags.mockResolvedValueOnce([
      { id: 5, name: "rap", carrier_count: 3 },
      { id: 9, name: "trap", carrier_count: 4 },
    ])

    await userEvent.click(screen.getByRole("button", { name: "Merge tags" }))

    await waitFor(() =>
      expect(
        screen.queryByText("Merge “rap” into “trap”?")
      ).not.toBeInTheDocument()
    )
    await waitFor(() => expect(mockListTags).toHaveBeenCalledTimes(2))
  })
})
