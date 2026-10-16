import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createRef } from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { listTags, type BulkAddTagOutcome } from "~/lib/api"
import { TagVocabularyProvider } from "~/lib/useTagVocabulary"
import { makeEntry } from "~/lib/test/fixtures"

import { BulkActionBar, type BulkActionBarProps } from "./BulkActionBar"

// Stubs every function export so no real apiFetch can run.
vi.mock("~/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/api")>()
  return Object.fromEntries(
    Object.entries(actual).map(([key, value]) => [
      key,
      key === "ApiError" || typeof value !== "function" ? value : vi.fn(),
    ])
  )
})

const VAR = "--bulk-bar-clearance"
const rootVar = () => document.documentElement.style.getPropertyValue(VAR)

function renderBar(overrides: Partial<BulkActionBarProps> = {}) {
  const defaults: BulkActionBarProps = {
    selected: [
      makeEntry({ id: 1, name: "Drake" }),
      makeEntry({ id: 2, name: "SZA" }),
    ],
    inFlight: false,
    onRemove: vi.fn(),
    onAddTag: vi.fn(),
    onSucceeded: vi.fn(),
    onClear: vi.fn(),
    removeButtonRef: createRef<HTMLButtonElement>(),
  }
  return render(
    <TagVocabularyProvider onChange={vi.fn()}>
      <BulkActionBar
        {...({ ...defaults, ...overrides } as BulkActionBarProps)}
      />
    </TagVocabularyProvider>
  )
}

function stubHeight(height: number) {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
    height,
  } as DOMRect)
}

beforeEach(() => {
  vi.mocked(listTags).mockResolvedValue([
    { id: 10, name: "reggaeton", carrier_count: 4 },
  ])
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe("BulkActionBar markup", () => {
  it("names the region and shows the count", () => {
    renderBar()
    const region = screen.getByRole("region", { name: "Selection actions" })
    expect(region).toBeInTheDocument()
    expect(screen.getByText("2 selected")).toHaveClass("tabular-nums")
  })

  it("gives Remove and Clear stable names with matching titles", () => {
    renderBar()
    const remove = screen.getByRole("button", { name: "Remove" })
    const clear = screen.getByRole("button", { name: "Clear selection" })
    expect(remove).toHaveAttribute("title", "Remove")
    expect(clear).toHaveAttribute("title", "Clear selection")
    expect(screen.getByText("Remove")).toHaveClass("sr-only", "sm:not-sr-only")
    expect(screen.getByText("Clear")).toHaveClass("sr-only", "sm:not-sr-only")
  })

  it("disables every button while in flight", () => {
    renderBar({ inFlight: true })
    for (const button of screen.getAllByRole("button")) {
      expect(button).toBeDisabled()
    }
  })

  it("attaches the remove button ref", () => {
    const removeButtonRef = createRef<HTMLButtonElement>()
    renderBar({ removeButtonRef })
    expect(removeButtonRef.current).toBe(
      screen.getByRole("button", { name: "Remove" })
    )
  })
})

describe("BulkActionBar clearance variable", () => {
  it("writes height plus inset plus 8px and follows resizes", () => {
    stubHeight(60)
    let notify: () => void = () => {}
    const observe = vi.fn()
    const disconnect = vi.fn()
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(cb: () => void) {
          notify = cb
        }
        observe = observe
        disconnect = disconnect
      }
    )

    const { unmount } = renderBar()
    expect(rootVar()).toBe("68px")
    expect(observe).toHaveBeenCalledTimes(1)

    stubHeight(120)
    notify()
    expect(rootVar()).toBe("128px")

    unmount()
    expect(disconnect).toHaveBeenCalled()
    expect(rootVar()).toBe("")
  })

  it("still sets the variable once without ResizeObserver", () => {
    stubHeight(60)
    vi.stubGlobal("ResizeObserver", undefined)

    const { unmount } = renderBar()
    expect(rootVar()).toBe("68px")

    unmount()
    expect(rootVar()).toBe("")
  })
})

describe("BulkActionBar inline add-tag mode", () => {
  const addButton = () => screen.getByRole("button", { name: "Add tag" })

  async function openPicker() {
    await userEvent.click(addButton())
    return screen.findByRole("combobox")
  }

  function outcome(applied: number[]): BulkAddTagOutcome {
    return {
      applied,
      skipped: [],
      tag: { id: 99, name: "hyperpop" },
    }
  }

  it("labels the input by count and swaps the other actions for Cancel", async () => {
    renderBar()
    const input = await openPicker()

    expect(input).toHaveAccessibleName("Add tag to 2 selected artists")
    // hidden:true because the open popup aria-hides the rest of the page.
    expect(
      screen.queryByRole("button", { name: "Remove", hidden: true })
    ).toBeNull()
    expect(
      screen.queryByRole("button", { name: "Add tag", hidden: true })
    ).toBeNull()
    expect(screen.getByText("2 selected")).toBeInTheDocument()
    expect(screen.getByTitle("Clear selection")).toBeVisible()
    expect(screen.getByText("Cancel")).toBeVisible()
  })

  it("uses the singular label for one selected artist", async () => {
    renderBar({ selected: [makeEntry({ id: 1, name: "Drake" })] })
    const input = await openPicker()

    expect(input).toHaveAccessibleName("Add tag to 1 selected artist")
  })

  it("closes on Esc without sending or clearing, and focuses Add tag", async () => {
    const onAddTag = vi.fn()
    const onClear = vi.fn()
    renderBar({ onAddTag, onClear })
    await openPicker()

    await userEvent.keyboard("{Escape}")

    await waitFor(() => expect(addButton()).toHaveFocus())
    expect(onAddTag).not.toHaveBeenCalled()
    expect(onClear).not.toHaveBeenCalled()
  })

  it("closes on Cancel without sending or clearing, and focuses Add tag", async () => {
    const onAddTag = vi.fn()
    const onClear = vi.fn()
    renderBar({ onAddTag, onClear })
    await openPicker()

    await userEvent.click(screen.getByText("Cancel"))

    await waitFor(() => expect(addButton()).toHaveFocus())
    expect(onAddTag).not.toHaveBeenCalled()
    expect(onClear).not.toHaveBeenCalled()
  })

  it("disables the input and shows Adding… while the request is pending", async () => {
    let resolve: (o: BulkAddTagOutcome | null) => void = () => {}
    const onAddTag = vi.fn(
      () => new Promise<BulkAddTagOutcome | null>((r) => (resolve = r))
    )
    renderBar({ onAddTag })
    const input = await openPicker()

    await userEvent.type(input, "hyperpop{Enter}")

    expect(onAddTag).toHaveBeenCalledWith("hyperpop")
    expect(screen.getByText("Adding…")).toBeInTheDocument()
    expect(
      screen.getByLabelText("Add tag to 2 selected artists")
    ).toBeDisabled()
    expect(screen.queryByText("Cancel")).toBeNull()

    resolve(null)
    await waitFor(() => expect(addButton()).toHaveFocus())
  })

  it("closes the picker and focuses Add tag when the request fails", async () => {
    const onSucceeded = vi.fn()
    renderBar({ onAddTag: vi.fn().mockResolvedValue(null), onSucceeded })
    const input = await openPicker()

    await userEvent.type(input, "hyperpop{Enter}")

    await waitFor(() => expect(addButton()).toHaveFocus())
    expect(onSucceeded).not.toHaveBeenCalled()
  })

  it("does not remember a tag that landed nowhere", async () => {
    const onSucceeded = vi.fn()
    renderBar({
      onAddTag: vi.fn().mockResolvedValue(outcome([])),
      onSucceeded,
    })
    const input = await openPicker()
    await userEvent.type(input, "hyperpop{Enter}")
    await waitFor(() => expect(onSucceeded).toHaveBeenCalledTimes(1))

    await userEvent.click(addButton())
    await userEvent.type(await screen.findByRole("combobox"), "hyper")

    expect(await screen.findByText("Create “hyper”")).toBeInTheDocument()
    expect(screen.queryByRole("option", { name: "hyperpop" })).toBeNull()
  })

  it("remembers the returned tag once it lands on an artist", async () => {
    const onSucceeded = vi.fn()
    renderBar({
      onAddTag: vi.fn().mockResolvedValue(outcome([1])),
      onSucceeded,
    })
    const input = await openPicker()
    await screen.findByRole("option", { name: "reggaeton" })
    await userEvent.type(input, "hyperpop{Enter}")
    await waitFor(() => expect(onSucceeded).toHaveBeenCalledTimes(1))

    await userEvent.click(addButton())
    await userEvent.type(await screen.findByRole("combobox"), "hyper")

    expect(
      await screen.findByRole("option", { name: "hyperpop" })
    ).toBeInTheDocument()
  })

  it("lists a tag already on every selected artist as already-on", async () => {
    const shared = { id: 10, name: "reggaeton" }
    renderBar({
      selected: [
        makeEntry({ id: 1, name: "Drake", tags: [shared] }),
        makeEntry({ id: 2, name: "SZA", tags: [shared] }),
      ],
    })
    const input = await openPicker()

    await userEvent.type(input, "reggaeton")

    expect(
      await screen.findByText(
        /^“reggaeton” is already on every selected artist/
      )
    ).toBeInTheDocument()
  })

  it("still clears on Esc from a bar button while idle", async () => {
    const onClear = vi.fn()
    renderBar({ onClear })
    addButton().focus()

    await userEvent.keyboard("{Escape}")

    expect(onClear).toHaveBeenCalledTimes(1)
  })
})
