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

function renderCombobox(vocabulary: TagRef[] | null = null) {
  const onCommit = vi.fn()
  const onClose = vi.fn()
  render(
    <TagCombobox
      entry={entry}
      vocabulary={vocabulary}
      namesOnArtist={new Set()}
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
})
