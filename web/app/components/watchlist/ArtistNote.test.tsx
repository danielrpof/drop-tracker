import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { type WatchlistEntry, updateNote } from "~/lib/api"

import { ArtistNote } from "./ArtistNote"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")

const mockUpdateNote = vi.mocked(updateNote)

const entry: WatchlistEntry = {
  id: 1,
  artist_id: 10,
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

describe("ArtistNote", () => {
  it("renders note text literally, never as HTML (T-24-37)", () => {
    render(
      <ArtistNote
        entry={{ ...entry, note: "<b>not bold</b>" }}
        onEntryChange={vi.fn()}
        announce={vi.fn()}
      />
    )

    expect(screen.getByText("<b>not bold</b>")).toBeInTheDocument()
    expect(document.querySelector("b")).toBeNull()
  })

  it("shows only 'add note' when there is no note, and opens with an empty focused textarea", async () => {
    render(
      <ArtistNote entry={entry} onEntryChange={vi.fn()} announce={vi.fn()} />
    )

    expect(
      screen.queryByRole("button", { name: "Edit note for Drake" })
    ).toBeNull()

    await userEvent.click(
      screen.getByRole("button", { name: "Add note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    expect(textarea).toHaveValue("")
    expect(textarea).toHaveFocus()
  })

  it("Save calls updateNote with the trimmed text and patches the entry", async () => {
    mockUpdateNote.mockResolvedValue({ ...entry, note: "crate digger" })
    const onEntryChange = vi.fn()

    render(
      <ArtistNote
        entry={entry}
        onEntryChange={onEntryChange}
        announce={vi.fn()}
      />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Add note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.type(textarea, "  crate digger  ")
    await userEvent.click(screen.getByRole("button", { name: "Save" }))

    expect(mockUpdateNote).toHaveBeenCalledWith(1, "crate digger")
    await waitFor(() =>
      expect(onEntryChange).toHaveBeenCalledWith(1, { note: "crate digger" })
    )
  })

  it("Save of whitespace-only text calls updateNote with null (clears the note)", async () => {
    const withNote = { ...entry, note: "old note" }
    mockUpdateNote.mockResolvedValue({ ...withNote, note: null })

    render(
      <ArtistNote entry={withNote} onEntryChange={vi.fn()} announce={vi.fn()} />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Edit note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.clear(textarea)
    await userEvent.type(textarea, "   ")
    await userEvent.click(screen.getByRole("button", { name: "Save" }))

    expect(mockUpdateNote).toHaveBeenCalledWith(1, null)
  })

  it("Save is disabled while the trimmed text equals the current note", async () => {
    const withNote = { ...entry, note: "old note" }

    render(
      <ArtistNote entry={withNote} onEntryChange={vi.fn()} announce={vi.fn()} />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Edit note for Drake" })
    )

    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled()
  })

  it("Esc closes the editor without calling updateNote", async () => {
    render(
      <ArtistNote entry={entry} onEntryChange={vi.fn()} announce={vi.fn()} />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Add note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.type(textarea, "abc")
    await userEvent.keyboard("{Escape}")

    expect(screen.queryByRole("textbox", { name: "Note for Drake" })).toBeNull()
    expect(
      screen.getByRole("button", { name: "Add note for Drake" })
    ).toBeInTheDocument()
    expect(mockUpdateNote).not.toHaveBeenCalled()
  })
})
