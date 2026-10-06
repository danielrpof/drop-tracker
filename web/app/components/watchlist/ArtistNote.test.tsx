import { useState } from "react"

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, onTestFinished, vi } from "vitest"

import { ApiError, type WatchlistEntry, updateNote } from "~/lib/api"

import { ArtistNote, type ArtistNoteProps } from "./ArtistNote"

// Partial mock keeps the real ApiError class intact -- a bare
// vi.mock("~/lib/api") automock would erase it, silently breaking the 400
// length-error instanceof check (mirrors TagChips.test.tsx's rationale).
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  updateNote: vi.fn(),
}))

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

  it("the note textarea keeps the 16px base size below md and switches to the label size only from md up", async () => {
    render(
      <ArtistNote entry={entry} onEntryChange={vi.fn()} announce={vi.fn()} />
    )

    await userEvent.click(
      screen.getByRole("button", { name: "Add note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    expect(textarea).toHaveClass("text-base")
    expect(textarea).toHaveClass("md:text-label")
    expect(textarea).not.toHaveClass("text-label")
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

  // Wrapper mimics the real Watchlist route: onEntryChange feeds straight
  // back into the entry prop, the way handleEntryChange/setEntries does at
  // the route level, so a post-save re-render sees the fresh note and the
  // pencil/"add note" focus target that depends on it actually exists in
  // the DOM.
  function Wrapper(props: {
    initial: WatchlistEntry
    announce: ArtistNoteProps["announce"]
  }) {
    const [entry, setEntry] = useState(props.initial)
    return (
      <ArtistNote
        entry={entry}
        onEntryChange={(_id, patch) => setEntry((e) => ({ ...e, ...patch }))}
        announce={props.announce}
      />
    )
  }

  it("while saving: Save reads 'Saving…' and is disabled, the textarea is readOnly with aria-busy, Cancel is disabled, and Esc does nothing", async () => {
    let resolveUpdate!: (value: WatchlistEntry) => void
    mockUpdateNote.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveUpdate = resolve
        })
    )
    const withNote = { ...entry, note: "old note" }

    render(
      <ArtistNote entry={withNote} onEntryChange={vi.fn()} announce={vi.fn()} />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Edit note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.clear(textarea)
    // Ctrl+Enter saves while keeping focus in the textarea (unlike clicking
    // Save), matching the flow the "Esc does nothing while saving" case
    // needs to exercise.
    await userEvent.type(textarea, "new text{Control>}{Enter}{/Control}")

    const savingButton = screen.getByRole("button", { name: "Saving…" })
    expect(savingButton).toBeDisabled()
    expect(textarea).toHaveAttribute("readonly")
    expect(textarea).toHaveAttribute("aria-busy", "true")
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled()

    await userEvent.keyboard("{Escape}")
    expect(
      screen.getByRole("textbox", { name: "Note for Drake" })
    ).toBeInTheDocument()

    resolveUpdate({ ...withNote, note: "new text" })
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Saving…" })).toBeNull()
    )
  })

  it("plain Enter inserts a newline instead of saving", async () => {
    render(
      <ArtistNote entry={entry} onEntryChange={vi.fn()} announce={vi.fn()} />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Add note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.type(textarea, "line one{Enter}line two")

    expect(textarea).toHaveValue("line one\nline two")
    expect(mockUpdateNote).not.toHaveBeenCalled()
  })

  it("a rejected save keeps the textarea open with the text intact, shows the generic error copy, and re-enables Save", async () => {
    mockUpdateNote.mockRejectedValueOnce(new Error("network down"))
    const withNote = { ...entry, note: "old note" }

    render(
      <ArtistNote entry={withNote} onEntryChange={vi.fn()} announce={vi.fn()} />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Edit note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.clear(textarea)
    await userEvent.type(textarea, "new text")
    await userEvent.click(screen.getByRole("button", { name: "Save" }))

    await screen.findByText(
      "Couldn't save the note — your text is still here. Try again."
    )
    expect(textarea).toHaveValue("new text")
    expect(screen.getByRole("button", { name: "Save" })).not.toBeDisabled()
  })

  it("a 400 length rejection shows the server-backstop copy", async () => {
    mockUpdateNote.mockRejectedValueOnce(
      new ApiError(400, "note must be at most 500 characters")
    )
    const withNote = { ...entry, note: "old note" }

    render(
      <ArtistNote entry={withNote} onEntryChange={vi.fn()} announce={vi.fn()} />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Edit note for Drake" })
    )

    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.clear(textarea)
    await userEvent.type(textarea, "new text")
    await userEvent.click(screen.getByRole("button", { name: "Save" }))

    await screen.findByText("Notes can be at most 500 characters.")
  })

  it("a successful save focuses the pencil and announces 'Note saved for {artist}.' when the saved note is non-empty", async () => {
    mockUpdateNote.mockResolvedValue({ ...entry, note: "crate digger" })
    const announce = vi.fn()

    render(<Wrapper initial={entry} announce={announce} />)
    await userEvent.click(
      screen.getByRole("button", { name: "Add note for Drake" })
    )
    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.type(textarea, "crate digger")
    await userEvent.click(screen.getByRole("button", { name: "Save" }))

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Edit note for Drake" })
      ).toHaveFocus()
    )
    expect(announce).toHaveBeenCalledWith("Note saved for Drake.")
  })

  it("a successful save focuses 'add note' and announces 'Note cleared for {artist}.' when the note was cleared", async () => {
    const withNote = { ...entry, note: "old note" }
    mockUpdateNote.mockResolvedValue({ ...withNote, note: null })
    const announce = vi.fn()

    render(<Wrapper initial={withNote} announce={announce} />)
    await userEvent.click(
      screen.getByRole("button", { name: "Edit note for Drake" })
    )
    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })
    await userEvent.clear(textarea)
    await userEvent.type(textarea, "   ")
    await userEvent.click(screen.getByRole("button", { name: "Save" }))

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add note for Drake" })
      ).toHaveFocus()
    )
    expect(announce).toHaveBeenCalledWith("Note cleared for Drake.")
  })

  it("the hidden counter region announces threshold crossings at 450 and 500, and nothing on other keystrokes", async () => {
    render(
      <ArtistNote entry={entry} onEntryChange={vi.fn()} announce={vi.fn()} />
    )
    await userEvent.click(
      screen.getByRole("button", { name: "Add note for Drake" })
    )
    const textarea = screen.getByRole("textbox", { name: "Note for Drake" })

    // fireEvent.change (not userEvent.type's per-keystroke simulation) for
    // the bulk jumps -- still exercises the real onChange handler, just
    // without ~450 individually-timed key events.
    fireEvent.change(textarea, { target: { value: "a".repeat(449) } })
    expect(screen.queryByText("50 characters left.")).toBeNull()

    fireEvent.change(textarea, { target: { value: "a".repeat(450) } })
    await screen.findByText("50 characters left.")

    fireEvent.change(textarea, { target: { value: "a".repeat(500) } })
    await screen.findByText("Note limit reached — 500 characters.")
  })

  it("shows the 'more' toggle only when the note paragraph actually overflows, and expanding removes the clamp", async () => {
    const withNote = { ...entry, note: "a long note that wraps a couple lines" }

    // jsdom performs no layout, so stub the measurement (scrollHeight >
    // clientHeight signals overflow) before the component's own layout
    // effect reads it.
    Object.defineProperty(HTMLParagraphElement.prototype, "scrollHeight", {
      configurable: true,
      value: 100,
    })
    Object.defineProperty(HTMLParagraphElement.prototype, "clientHeight", {
      configurable: true,
      value: 40,
    })

    render(
      <ArtistNote entry={withNote} onEntryChange={vi.fn()} announce={vi.fn()} />
    )

    const toggle = await screen.findByRole("button", { name: "more" })
    expect(toggle).toHaveAttribute("aria-expanded", "false")
    expect(toggle).toHaveAttribute("aria-controls", "note-1")
    expect(
      document.getElementById("note-1")?.className.includes("line-clamp-2")
    ).toBe(true)

    await userEvent.click(toggle)

    expect(screen.getByRole("button", { name: "less" })).toHaveAttribute(
      "aria-expanded",
      "true"
    )
    expect(
      document.getElementById("note-1")?.className.includes("line-clamp-2")
    ).toBe(false)
  })

  it("an expanded note keeps its 'less' toggle mounted and focused after the grown paragraph re-measures, and 'less' collapses it again", async () => {
    const observers = new Set<{ cb: () => void }>()
    class FakeResizeObserver {
      cb: () => void
      constructor(cb: () => void) {
        this.cb = cb
      }
      observe() {
        observers.add(this)
      }
      unobserve() {}
      disconnect() {
        observers.delete(this)
      }
    }
    vi.stubGlobal("ResizeObserver", FakeResizeObserver)
    onTestFinished(() => {
      vi.unstubAllGlobals()
    })

    // Clamped paragraphs overflow; an unclamped one has grown to full height.
    Object.defineProperty(HTMLParagraphElement.prototype, "scrollHeight", {
      configurable: true,
      value: 100,
    })
    Object.defineProperty(HTMLParagraphElement.prototype, "clientHeight", {
      configurable: true,
      get(this: HTMLElement) {
        return this.classList.contains("line-clamp-2") ? 40 : 100
      },
    })

    render(
      <ArtistNote
        entry={{ ...entry, note: "a long note that wraps well past two lines" }}
        onEntryChange={vi.fn()}
        announce={vi.fn()}
      />
    )

    const toggle = await screen.findByRole("button", { name: "more" })
    await userEvent.click(toggle)

    act(() => {
      observers.forEach((o) => o.cb())
    })

    const less = screen.getByRole("button", { name: "less" })
    expect(less).toBe(toggle)
    expect(less).toHaveAttribute("aria-expanded", "true")
    expect(less).toHaveFocus()
    expect(document.getElementById("note-1")).not.toHaveClass("line-clamp-2")

    await userEvent.click(toggle)

    const more = screen.getByRole("button", { name: "more" })
    expect(more).toBe(toggle)
    expect(more).toHaveAttribute("aria-expanded", "false")
    expect(more).toHaveFocus()
    expect(document.getElementById("note-1")).toHaveClass("line-clamp-2")
  })

  it("renders no more/less toggle when the note fits in two lines", () => {
    const withNote = { ...entry, note: "short note" }

    Object.defineProperty(HTMLParagraphElement.prototype, "scrollHeight", {
      configurable: true,
      value: 40,
    })
    Object.defineProperty(HTMLParagraphElement.prototype, "clientHeight", {
      configurable: true,
      value: 40,
    })

    render(
      <ArtistNote entry={withNote} onEntryChange={vi.fn()} announce={vi.fn()} />
    )

    expect(screen.queryByRole("button", { name: "more" })).toBeNull()
  })
})
