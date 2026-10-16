import { render, screen } from "@testing-library/react"
import { createRef } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"

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
