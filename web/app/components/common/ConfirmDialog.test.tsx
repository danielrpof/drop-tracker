import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { ConfirmDialog } from "./ConfirmDialog"

// deferred gives each test explicit control over when onConfirm settles, so
// the "pending" render (disabled buttons + pendingLabel) can be asserted
// before letting the promise resolve or reject.
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function renderDialog(onConfirm: () => Promise<void>, onOpenChange = vi.fn()) {
  render(
    <ConfirmDialog
      open
      onOpenChange={onOpenChange}
      title="Delete “rap”?"
      description="No artists on your watchlist carry this tag, and this can't be undone."
      actionLabel="Delete tag"
      pendingLabel="Deleting…"
      variant="destructive"
      onConfirm={onConfirm}
    />
  )
  return { onOpenChange }
}

describe("ConfirmDialog", () => {
  it("renders the title, description, and action copy", () => {
    renderDialog(() => Promise.resolve())

    expect(screen.getByText("Delete “rap”?")).toBeInTheDocument()
    expect(
      screen.getByText(
        "No artists on your watchlist carry this tag, and this can't be undone."
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Delete tag" })
    ).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument()
  })

  it("title and description carry wrap-anywhere so a long unspaced tag name wraps instead of overflowing", () => {
    renderDialog(() => Promise.resolve())

    const title = screen.getByRole("heading", { name: "Delete “rap”?" })
    expect(title).toHaveClass("wrap-anywhere")
    expect(title).not.toHaveClass("truncate")
    expect(
      screen.getByText(
        "No artists on your watchlist carry this tag, and this can't be undone."
      )
    ).toHaveClass("wrap-anywhere")
  })

  it("Cancel closes without calling onConfirm", async () => {
    const onConfirm = vi.fn(() => Promise.resolve())
    const { onOpenChange } = renderDialog(onConfirm)

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }))

    expect(onConfirm).not.toHaveBeenCalled()
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it("the action calls onConfirm once, shows the pending label, and disables both buttons while pending, then closes on success", async () => {
    const d = deferred<void>()
    const onConfirm = vi.fn(() => d.promise)
    const { onOpenChange } = renderDialog(onConfirm)

    await userEvent.click(screen.getByRole("button", { name: "Delete tag" }))

    expect(onConfirm).toHaveBeenCalledTimes(1)
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Deleting…" })
      ).toBeInTheDocument()
    )
    expect(screen.getByRole("button", { name: "Deleting…" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled()

    d.resolve()

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it("closes even when onConfirm rejects, without throwing", async () => {
    const d = deferred<void>()
    const onConfirm = vi.fn(() => d.promise)
    const { onOpenChange } = renderDialog(onConfirm)

    await userEvent.click(screen.getByRole("button", { name: "Delete tag" }))
    d.reject(new Error("network down"))

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })
})
