import { useState } from "react"

import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import {
  ApiError,
  attachTag,
  detachTag,
  type TagRef,
  type WatchlistEntry,
} from "~/lib/api"

import { TagChips, type TagActions } from "./TagChips"

// Partial mock keeps the real ApiError class intact -- a bare
// vi.mock("~/lib/api") automock would erase it, silently breaking the
// 409/400 toast-mapping branches' instanceof checks.
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  attachTag: vi.fn(),
  detachTag: vi.fn(),
}))
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

const mockDetachTag = vi.mocked(detachTag)
const mockAttachTag = vi.mocked(attachTag)

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

// baseTagActions stubs the vocabulary/loadVocabulary/rememberTag seam
// TagChips also drives (D-30) -- these tests exercise the chip row itself,
// not the "+ tag" combobox, so the stubs stay inert.
function baseTagActions(): Pick<
  TagActions,
  "vocabulary" | "loadVocabulary" | "rememberTag"
> {
  return {
    vocabulary: null,
    loadVocabulary: vi.fn(),
    rememberTag: vi.fn(),
  }
}

function renderChips() {
  const addTag = vi.fn()
  const removeTag = vi.fn()
  const announce = vi.fn()
  render(
    <TagChips
      entry={entry}
      actions={{ addTag, removeTag, ...baseTagActions() }}
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

  it("renders only the '+ tag' trigger when the entry has no tags -- no empty-row gap (D-02)", () => {
    render(
      <TagChips
        entry={{ ...entry, tags: [] }}
        actions={{ addTag: vi.fn(), removeTag: vi.fn(), ...baseTagActions() }}
        announce={vi.fn()}
      />
    )

    expect(
      screen.getByRole("button", { name: "Add tag to Drake" })
    ).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /Remove tag/ })).toBeNull()
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
    ...baseTagActions(),
  }

  return <TagChips entry={entry} actions={actions} announce={onAnnounce} />
}

function tagRefs(names: string[]): TagRef[] {
  return names.map((name, i) => ({ id: i + 1, name }))
}

function chipRemoveButton(name: string, artist = "Drake") {
  return screen.getByRole("button", {
    name: `Remove tag ${name} from ${artist}`,
  })
}

describe("TagChips — focus, announcements, and long-name handling", () => {
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

  it("focuses '+ tag' when the only chip is removed", async () => {
    mockDetachTag.mockResolvedValueOnce(undefined)
    render(
      <Harness
        initialEntry={{ ...entry, tags: tagRefs(["only"]) }}
        onAnnounce={vi.fn()}
      />
    )

    await userEvent.click(chipRemoveButton("only"))

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add tag to Drake" })
      ).toHaveFocus()
    )
  })
})

async function openEditor(artist = "Drake") {
  await userEvent.click(
    screen.getByRole("button", { name: `Add tag to ${artist}` })
  )
  return screen.findByRole("combobox", { name: `Add tag to ${artist}` })
}

describe("TagChips — cap hint, refusal toasts, and close focus", () => {
  it("shows 'max 10 tags' (not a button) at 10 tags, with no '+ tag'; removing a chip restores it", async () => {
    mockDetachTag.mockResolvedValueOnce(undefined)
    const tenTags = tagRefs(Array.from({ length: 10 }, (_, i) => `tag${i}`))
    render(
      <Harness
        initialEntry={{ ...entry, tags: tenTags }}
        onAnnounce={vi.fn()}
      />
    )

    expect(screen.getByText("max 10 tags")).toBeInTheDocument()
    expect(screen.getByText("max 10 tags").tagName).toBe("SPAN")
    expect(
      screen.queryByRole("button", { name: "Add tag to Drake" })
    ).toBeNull()

    await userEvent.click(chipRemoveButton("tag0"))

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add tag to Drake" })
      ).toBeInTheDocument()
    )
    expect(screen.queryByText("max 10 tags")).toBeNull()
  })

  it("a pick that brings a 9-tag artist to 10 closes the editor, focuses the new chip's ×, and announces the max-reached variant", async () => {
    mockAttachTag.mockResolvedValueOnce({ id: 99, name: "drill" })
    const onAnnounce = vi.fn()
    const nineTags = tagRefs(Array.from({ length: 9 }, (_, i) => `tag${i}`))
    render(
      <Harness
        initialEntry={{ ...entry, tags: nineTags }}
        onAnnounce={onAnnounce}
      />
    )

    const input = await openEditor()
    await userEvent.type(input, "drill{Enter}")

    await waitFor(() => expect(chipRemoveButton("drill")).toHaveFocus())
    expect(
      screen.queryByRole("combobox", { name: "Add tag to Drake" })
    ).toBeNull()
    expect(onAnnounce).toHaveBeenCalledWith(
      "Added “drill” to Drake. Max 10 tags reached."
    )
  })

  it("a failed 10th attach restores '+ tag' and focuses it", async () => {
    mockAttachTag.mockRejectedValueOnce(new Error("network down"))
    const nineTags = tagRefs(Array.from({ length: 9 }, (_, i) => `tag${i}`))
    render(
      <Harness
        initialEntry={{ ...entry, tags: nineTags }}
        onAnnounce={vi.fn()}
      />
    )

    const input = await openEditor()
    await userEvent.type(input, "drill{Enter}")

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add tag to Drake" })
      ).toHaveFocus()
    )
  })

  it("a successful pick under 10 tags leaves focus in the cleared input and announces the plain message", async () => {
    mockAttachTag.mockResolvedValueOnce({ id: 5, name: "drill" })
    const { announce } = renderChips()

    const input = await openEditor()
    await userEvent.type(input, "drill{Enter}")

    await waitFor(() =>
      expect(announce).toHaveBeenCalledWith("Added “drill” to Drake.")
    )
    expect(input).toHaveFocus()
    expect(input).toHaveValue("")
  })

  it("maps a tag_cap_reached refusal to the cap toast, whatever the message", async () => {
    mockAttachTag.mockRejectedValueOnce(
      new ApiError(409, "anything", undefined, "tag_cap_reached")
    )
    renderChips()
    const { toast } = await import("sonner")

    const input = await openEditor()
    await userEvent.type(input, "drill{Enter}")

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Drake already has 10 tags — remove one first."
      )
    )
  })

  it("maps a tag_name_too_long refusal to the length toast, whatever the message", async () => {
    mockAttachTag.mockRejectedValueOnce(
      new ApiError(400, "anything", undefined, "tag_name_too_long")
    )
    renderChips()
    const { toast } = await import("sonner")

    const input = await openEditor()
    await userEvent.type(input, "drill{Enter}")

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Tags can be at most 32 characters."
      )
    )
  })

  it("does not branch on message text: the old length message without a code gets the generic toast", async () => {
    mockAttachTag.mockRejectedValueOnce(
      new ApiError(400, "tag name must be at most 32 characters")
    )
    renderChips()
    const { toast } = await import("sonner")

    const input = await openEditor()
    await userEvent.type(input, "drill{Enter}")

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Couldn't add “drill” to Drake — try again."
      )
    )
  })

  it("maps any other failure to the generic toast", async () => {
    mockAttachTag.mockRejectedValueOnce(new Error("network down"))
    renderChips()
    const { toast } = await import("sonner")

    const input = await openEditor()
    await userEvent.type(input, "drill{Enter}")

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Couldn't add “drill” to Drake — try again."
      )
    )
  })

  it("Esc closes the editor in one press and focuses '+ tag', without attaching the typed text", async () => {
    renderChips()

    const input = await openEditor()
    await userEvent.type(input, "drill")
    await userEvent.keyboard("{Escape}")

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add tag to Drake" })
      ).toHaveFocus()
    )
    expect(mockAttachTag).not.toHaveBeenCalled()
  })

  it("blur closes the editor without pulling focus back, and does not attach the typed text", async () => {
    render(
      <div>
        <Harness initialEntry={entry} onAnnounce={vi.fn()} />
        <button>elsewhere</button>
      </div>
    )

    const input = await openEditor()
    await userEvent.type(input, "drill")
    await userEvent.click(screen.getByText("elsewhere"))

    await waitFor(() =>
      expect(
        screen.queryByRole("combobox", { name: "Add tag to Drake" })
      ).toBeNull()
    )
    expect(screen.getByText("elsewhere")).toHaveFocus()
    expect(mockAttachTag).not.toHaveBeenCalled()
  })
})
