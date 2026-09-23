import { useState } from "react"

import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { detachTag, type TagRef, type WatchlistEntry } from "~/lib/api"

import { TagChips, type TagActions } from "./TagChips"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

const mockDetachTag = vi.mocked(detachTag)

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
  tags: [
    { id: 1, name: "reggaeton" },
    { id: 2, name: "<img src=x onerror=alert(1)>" },
  ],
  note: null,
}

function renderChips() {
  const addTag = vi.fn()
  const removeTag = vi.fn()
  const announce = vi.fn()
  render(
    <TagChips
      entry={entry}
      actions={{ addTag, removeTag }}
      announce={announce}
    />
  )
  return { addTag, removeTag, announce }
}

describe("TagChips", () => {
  it("renders tag names as literal text, including a name that looks like HTML", () => {
    renderChips()

    expect(screen.getByText("reggaeton")).toBeInTheDocument()
    expect(screen.getByText("<img src=x onerror=alert(1)>")).toBeInTheDocument()
    expect(screen.queryByRole("img")).not.toBeInTheDocument()
  })

  it("renders nothing when the entry has no tags", () => {
    const { container } = render(
      <TagChips
        entry={{ ...entry, tags: [] }}
        actions={{ addTag: vi.fn(), removeTag: vi.fn() }}
        announce={vi.fn()}
      />
    )

    expect(container).toBeEmptyDOMElement()
  })

  it("clicking a chip's label calls nothing", async () => {
    const { removeTag } = renderChips()

    await userEvent.click(screen.getByText("reggaeton"))

    expect(removeTag).not.toHaveBeenCalled()
    expect(mockDetachTag).not.toHaveBeenCalled()
  })

  it("clicking a chip's × calls actions.removeTag then detachTag through the real DELETE wrapper", async () => {
    mockDetachTag.mockResolvedValueOnce(undefined)
    const { removeTag } = renderChips()

    await userEvent.click(
      screen.getByRole("button", { name: "Remove tag reggaeton from Drake" })
    )

    expect(removeTag).toHaveBeenCalledWith(42, 1)
    await waitFor(() => expect(mockDetachTag).toHaveBeenCalledWith(42, 1))
  })

  it("restores the chip at its original index and fires the toast when detachTag rejects", async () => {
    mockDetachTag.mockRejectedValueOnce(new Error("network down"))
    const { addTag } = renderChips()
    const { toast } = await import("sonner")

    await userEvent.click(
      screen.getByRole("button", { name: "Remove tag reggaeton from Drake" })
    )

    await waitFor(() =>
      expect(addTag).toHaveBeenCalledWith(42, { id: 1, name: "reggaeton" }, 0)
    )
    expect(toast.error).toHaveBeenCalledWith(
      "Couldn't remove “reggaeton” from Drake — try again."
    )
  })
})

// Harness mirrors watchlist.tsx's real addTag/removeTag functional updaters
// (D-24) so a click's optimistic removal actually re-renders TagChips with
// a shrunk entry.tags -- the focus-management behaviors below depend on a
// real DOM removal, not a mocked no-op.
function Harness({
  initialEntry,
  onAnnounce,
}: {
  initialEntry: WatchlistEntry
  onAnnounce: (message: string) => void
}) {
  const [entry, setEntry] = useState(initialEntry)

  const actions: TagActions = {
    addTag(entryId, tag, index) {
      setEntry((e) => {
        if (e.id !== entryId || e.tags.some((t) => t.id === tag.id)) return e
        const tags = [...e.tags]
        const at =
          index === undefined
            ? tags.length
            : Math.max(0, Math.min(index, tags.length))
        tags.splice(at, 0, tag)
        return { ...e, tags }
      })
    },
    removeTag(entryId, tagId) {
      setEntry((e) =>
        e.id === entryId
          ? { ...e, tags: e.tags.filter((t) => t.id !== tagId) }
          : e
      )
    },
  }

  return <TagChips entry={entry} actions={actions} announce={onAnnounce} />
}

function tagRefs(names: string[]): TagRef[] {
  return names.map((name, i) => ({ id: i + 1, name }))
}

describe("TagChips — focus, announcements, and long-name handling", () => {
  function chipRemoveButton(name: string, artist = "Drake") {
    return screen.getByRole("button", {
      name: `Remove tag ${name} from ${artist}`,
    })
  }

  it("moves focus to the new first chip's × when the first of three chips is removed", async () => {
    mockDetachTag.mockResolvedValueOnce(undefined)
    const onAnnounce = vi.fn()
    render(
      <Harness
        initialEntry={{ ...entry, tags: tagRefs(["a", "b", "c"]) }}
        onAnnounce={onAnnounce}
      />
    )

    await userEvent.click(chipRemoveButton("a"))

    await waitFor(() => expect(chipRemoveButton("b")).toHaveFocus())
  })

  it("moves focus to the previous chip's × when the last of three chips is removed", async () => {
    mockDetachTag.mockResolvedValueOnce(undefined)
    const onAnnounce = vi.fn()
    render(
      <Harness
        initialEntry={{ ...entry, tags: tagRefs(["a", "b", "c"]) }}
        onAnnounce={onAnnounce}
      />
    )

    await userEvent.click(chipRemoveButton("c"))

    await waitFor(() => expect(chipRemoveButton("b")).toHaveFocus())
  })

  it("does not pull focus back when a removal fails and the chip is restored", async () => {
    mockDetachTag.mockRejectedValueOnce(new Error("network down"))
    const onAnnounce = vi.fn()
    render(
      <Harness
        initialEntry={{ ...entry, tags: tagRefs(["a", "b", "c"]) }}
        onAnnounce={onAnnounce}
      />
    )

    await userEvent.click(chipRemoveButton("b"))

    // Optimistic removal moves focus to the sibling chip immediately.
    await waitFor(() => expect(chipRemoveButton("c")).toHaveFocus())

    // The failed detach restores "b" -- focus must stay on "c", not jump.
    await waitFor(() => expect(chipRemoveButton("b")).toBeInTheDocument())
    expect(chipRemoveButton("c")).toHaveFocus()
  })

  it("announces the plain remove message when the artist had fewer than 10 tags", async () => {
    mockDetachTag.mockResolvedValueOnce(undefined)
    const onAnnounce = vi.fn()
    render(
      <Harness
        initialEntry={{ ...entry, tags: tagRefs(["a", "b", "c"]) }}
        onAnnounce={onAnnounce}
      />
    )

    await userEvent.click(chipRemoveButton("a"))

    expect(onAnnounce).toHaveBeenCalledWith("Removed “a” from Drake.")
  })

  it("announces the max-tags-freed message when the artist had exactly 10 tags", async () => {
    mockDetachTag.mockResolvedValueOnce(undefined)
    const onAnnounce = vi.fn()
    const tenTags = tagRefs(Array.from({ length: 10 }, (_, i) => `tag${i}`))
    render(
      <Harness
        initialEntry={{ ...entry, tags: tenTags }}
        onAnnounce={onAnnounce}
      />
    )

    await userEvent.click(chipRemoveButton("tag0"))

    expect(onAnnounce).toHaveBeenCalledWith(
      "Removed “tag0” from Drake. You can add tags again."
    )
  })

  it("the label carries group-focus-within/chip un-truncate classes and title for pointer users", () => {
    renderChips()

    const label = screen.getByText("reggaeton")
    expect(label).toHaveAttribute("title", "reggaeton")
    expect(label.className).toContain(
      "group-focus-within/chip:whitespace-normal"
    )
    expect(label.className).toContain("group-focus-within/chip:break-all")
  })

  it("never clips the chip row: no overflow-hidden, max-h-, or line-clamp on the container", () => {
    renderChips()

    const container = screen
      .getByText("reggaeton")
      .closest("div.flex-wrap") as HTMLElement
    expect(container).not.toBeNull()
    expect(container.className).toContain("flex-wrap")
    expect(container.className).not.toMatch(/overflow-hidden|max-h-|line-clamp/)
  })
})
