import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { detachTag, type WatchlistEntry } from "~/lib/api"

import { TagChips } from "./TagChips"

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
