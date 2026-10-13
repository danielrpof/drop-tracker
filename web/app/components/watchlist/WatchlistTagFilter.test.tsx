import { useState } from "react"
import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { sortTagsByName } from "~/lib/tags"
import type { TagFilterOption } from "~/lib/watchlistView"

import { WatchlistTagFilter } from "./WatchlistTagFilter"

const OPTIONS: TagFilterOption[] = [
  { id: 3, name: "reggaeton", count: 2 },
  { id: 5, name: "rap", count: 4 },
  { id: 7, name: "trap", count: 1 },
]

// Mirrors the route: selection lives in the parent and is rebuilt by id.
function Harness({
  options = OPTIONS,
  initial = [],
  onChange,
}: {
  options?: TagFilterOption[]
  initial?: number[]
  onChange: (ids: number[]) => void
}) {
  const [ids, setIds] = useState(initial)
  const selected = sortTagsByName(options.filter((o) => ids.includes(o.id)))
  return (
    <>
      <span id="watchlist-tags-label">Tags: any of</span>
      <WatchlistTagFilter
        options={options}
        selected={selected}
        onChange={(next) => {
          setIds(next)
          onChange(next)
        }}
      />
    </>
  )
}

function setup(props: Partial<Parameters<typeof Harness>[0]> = {}) {
  const onChange = vi.fn()
  render(<Harness onChange={onChange} {...props} />)
  return { onChange, input: screen.getByLabelText("Filter by tags, any of") }
}

describe("WatchlistTagFilter", () => {
  it("keeps the popup open, the input empty, and focus in the input after a pick (A2)", async () => {
    const { onChange, input } = setup()

    await userEvent.click(input)
    await userEvent.click(
      await screen.findByRole("option", { name: /^rap\d+$/ })
    )

    expect(onChange).toHaveBeenLastCalledWith([5])
    expect(screen.getByRole("listbox")).toBeInTheDocument()
    expect(input).toHaveValue("")
    expect(input).toHaveFocus()
  })

  it("deselects a selected option when it is clicked again", async () => {
    const { onChange, input } = setup({ initial: [5] })

    await userEvent.click(input)
    const option = await screen.findByRole("option", { name: /^rap\d+$/ })
    expect(option).toHaveAttribute("aria-selected", "true")
    await userEvent.click(option)

    expect(onChange).toHaveBeenLastCalledWith([])
  })

  it("narrows by a case-insensitive contains match (A1)", async () => {
    const { input } = setup()

    await userEvent.type(input, "REG")

    const options = await screen.findAllByRole("option")
    expect(options).toHaveLength(1)
    expect(options[0]).toHaveTextContent("reggaeton")
  })

  it("matches in the middle of a name, not only the start", async () => {
    const { input } = setup()

    await userEvent.type(input, "ap")

    const names = (await screen.findAllByRole("option")).map((o) =>
      o.textContent?.replace(/\d+$/, "")
    )
    expect(names).toEqual(["rap", "trap"])
  })

  it("says so when typed text matches no tag", async () => {
    const { input } = setup()

    await userEvent.type(input, "zzz")

    expect(await screen.findByText("No tags match “zzz”")).toBeInTheDocument()
    expect(screen.queryByRole("option")).not.toBeInTheDocument()
  })

  it("explains an empty vocabulary instead of offering options", async () => {
    const { input } = setup({ options: [] })

    await userEvent.click(input)

    expect(
      await screen.findByText(
        "No tags yet — add one with “+ tag” on any artist's card."
      )
    ).toBeInTheDocument()
    expect(screen.queryByRole("option")).not.toBeInTheDocument()
  })

  it("removes the last chip on Backspace in the empty input", async () => {
    const { onChange, input } = setup({ initial: [3, 7] })

    await userEvent.click(input)
    await userEvent.keyboard("{Backspace}")

    expect(onChange).toHaveBeenLastCalledWith([3])
    expect(input).toHaveFocus()
  })

  it("removes a chip through its x and ends with focus in the input", async () => {
    const { onChange, input } = setup({ initial: [3] })

    await userEvent.click(
      screen.getByRole("button", { name: "Remove reggaeton from tag filter" })
    )

    expect(onChange).toHaveBeenLastCalledWith([])
    expect(input).toHaveFocus()
  })

  it("closes on Esc, and a second Esc does nothing", async () => {
    const { onChange, input } = setup()

    await userEvent.click(input)
    await screen.findByRole("listbox")
    await userEvent.keyboard("{Escape}")
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument()

    await userEvent.keyboard("{Escape}")
    expect(onChange).not.toHaveBeenCalled()
    expect(input).toHaveFocus()
  })

  it("keeps the selection when Esc is pressed with the popup closed", async () => {
    const { onChange, input } = setup({ initial: [3] })

    await userEvent.click(input)
    await userEvent.keyboard("{Escape}{Escape}")

    expect(onChange).not.toHaveBeenCalled()
    expect(
      screen.getByRole("button", { name: "Remove reggaeton from tag filter" })
    ).toBeInTheDocument()
  })

  it("shows the All tags placeholder only while nothing is selected", () => {
    setup()
    expect(screen.getByLabelText("Filter by tags, any of")).toHaveAttribute(
      "placeholder",
      "All tags"
    )
  })

  it("drops the placeholder once a tag is selected", () => {
    setup({ initial: [3] })
    expect(screen.getByLabelText("Filter by tags, any of")).toHaveAttribute(
      "placeholder",
      ""
    )
  })

  it("renders chips sorted by name regardless of click order", async () => {
    const { input } = setup()

    await userEvent.click(input)
    await userEvent.click(
      await screen.findByRole("option", { name: /^trap\d+$/ })
    )
    await userEvent.click(
      await screen.findByRole("option", { name: /^rap\d+$/ })
    )
    await userEvent.click(
      await screen.findByRole("option", { name: /^reggaeton\d+$/ })
    )

    const removes = screen
      .getAllByRole("button", { name: /from tag filter$/ })
      .map((b) => b.getAttribute("aria-label"))
    expect(removes).toEqual([
      "Remove rap from tag filter",
      "Remove reggaeton from tag filter",
      "Remove trap from tag filter",
    ])
  })

  it("shows each tag's carrier count on its option row", async () => {
    const { input } = setup()

    await userEvent.click(input)
    const row = await screen.findByRole("option", { name: /^rap\d+$/ })

    expect(within(row).getByText("4")).toHaveClass("tabular-nums")
  })

  it("carries the full name on the chip title for truncated tags", () => {
    const long = "x".repeat(32)
    setup({ options: [{ id: 9, name: long, count: 1 }], initial: [9] })

    expect(screen.getByTitle(long)).toHaveClass("truncate")
    expect(
      screen.getByRole("button", { name: `Remove ${long} from tag filter` })
    ).toBeInTheDocument()
  })
})
