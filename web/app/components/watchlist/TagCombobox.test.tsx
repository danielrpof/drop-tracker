import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { TagRef, WatchlistEntry } from "~/lib/api"

import { TagCombobox } from "./TagCombobox"

const entry: WatchlistEntry = {
  id: 42,
  artist_id: 7,
  mbid: "mbid-drake",
  name: "Drake",
  deezer_id: null,
  disambiguation: null,
  image_url: null,
  release_types: [],
  muted_event_types: [],
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  tags: [],
  note: null,
}

function renderCombobox(
  vocabulary: TagRef[] | null = null,
  overrides: { onArtist?: TagRef[]; pendingNames?: string[] } = {}
) {
  const onCommit = vi.fn()
  const onClose = vi.fn()
  render(
    <TagCombobox
      entry={entry}
      vocabulary={vocabulary}
      onArtist={overrides.onArtist ?? []}
      pendingNames={overrides.pendingNames ?? []}
      onCommit={onCommit}
      onClose={onClose}
    />
  )
  return { onCommit, onClose }
}

describe("TagCombobox", () => {
  it("typing a new name and pressing Enter commits the typed text", async () => {
    const { onCommit } = renderCombobox()

    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "hyperpop{Enter}")

    expect(onCommit).toHaveBeenCalledWith("hyperpop")
  })

  it("picking a vocabulary option commits its stored name", async () => {
    const { onCommit } = renderCombobox([{ id: 1, name: "reggaeton" }])

    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "reg")

    await userEvent.click(await screen.findByText("reggaeton"))

    expect(onCommit).toHaveBeenCalledWith("reggaeton")
  })

  it("pins an exact match and suppresses Create, so Enter attaches the existing tag under its stored casing (TAG-03)", async () => {
    const vocabulary: TagRef[] = [
      { id: 1, name: "reggaeton" },
      { id: 2, name: "reggae" },
    ]
    const { onCommit } = renderCombobox(vocabulary)

    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "Reggaeton {Enter}")

    expect(onCommit).toHaveBeenCalledWith("reggaeton")
    expect(screen.queryByText(/Create/)).toBeNull()
  })

  it("shows the non-selectable already-on line and does nothing on Enter when the query matches a tag already on the artist", async () => {
    const onArtist: TagRef[] = [{ id: 1, name: "latin" }]
    const { onCommit } = renderCombobox([{ id: 1, name: "latin" }], {
      onArtist,
    })

    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    await userEvent.type(input, "latin{Enter}")

    // base-ui's ComboboxEmpty appends an invisible word-joiner to force
    // screen-reader re-announcement, so match by prefix, not exact text.
    expect(
      await screen.findByText(/^“latin” is already on this artist/)
    ).toBeInTheDocument()
    expect(onCommit).not.toHaveBeenCalled()
  })

  it("shows 'Type a name to create a tag' for an empty, loaded vocabulary with no query", () => {
    renderCombobox([])

    expect(screen.getByText(/^Type a name to create a tag/)).toBeInTheDocument()
  })

  it("shows 'Type a name to create a tag' before the vocabulary has loaded, with no query", () => {
    renderCombobox(null)

    expect(screen.getByText(/^Type a name to create a tag/)).toBeInTheDocument()
  })

  it("shows the muted 25/32 counter at 25 characters and switches to text-foreground at 32", async () => {
    renderCombobox()
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })

    await userEvent.type(input, "a".repeat(25))
    expect(screen.getByText("25/32")).toHaveClass("text-muted-foreground")

    await userEvent.type(input, "a".repeat(7))
    expect(screen.getByText("32/32")).toHaveClass("text-foreground")
  })

  it("does not show a counter below 25 characters", async () => {
    renderCombobox()
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })

    await userEvent.type(input, "a".repeat(24))

    expect(screen.queryByText("24/32")).toBeNull()
  })

  it("announces the 25 and 32 threshold crossings exactly once each, and nothing on other keystrokes", async () => {
    renderCombobox()
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })
    const liveRegion = document.querySelector('[aria-live="polite"]')

    await userEvent.type(input, "a".repeat(24))
    expect(liveRegion).toHaveTextContent("")

    await userEvent.type(input, "a")
    expect(liveRegion).toHaveTextContent("7 characters left.")

    await userEvent.type(input, "a".repeat(6))
    expect(liveRegion).toHaveTextContent("7 characters left.")

    await userEvent.type(input, "a")
    expect(liveRegion).toHaveTextContent(
      "Tag name limit reached — 32 characters."
    )
  })

  it("calls onClose('escape') and closes the popup when Esc is pressed", async () => {
    const { onClose } = renderCombobox()
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })

    await userEvent.click(input)
    await userEvent.keyboard("{Escape}")

    expect(onClose).toHaveBeenCalledWith("escape")
  })

  it("calls onClose('blur') when focus leaves the combobox via an outside click", async () => {
    const { onClose } = renderCombobox()
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })

    await userEvent.click(input)
    await userEvent.click(document.body)

    expect(onClose).toHaveBeenCalledWith("blur")
  })

  it("does not call onClose when Enter is pressed with nothing selectable (already-on state)", async () => {
    const onArtist: TagRef[] = [{ id: 1, name: "latin" }]
    const { onClose } = renderCombobox([{ id: 1, name: "latin" }], {
      onArtist,
    })
    const input = screen.getByRole("combobox", { name: "Add tag to Drake" })

    await userEvent.type(input, "latin{Enter}")

    expect(onClose).not.toHaveBeenCalled()
  })
})
