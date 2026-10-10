import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createRef } from "react"
import { describe, expect, it, vi } from "vitest"

import { WatchlistToolbar } from "~/components/watchlist/WatchlistToolbar"

function setup(query: string) {
  const onQueryChange = vi.fn()
  const filterInputRef = createRef<HTMLInputElement>()
  render(
    <WatchlistToolbar
      query={query}
      onQueryChange={onQueryChange}
      filterInputRef={filterInputRef}
      showClear={false}
      onClearFilters={() => {}}
      sort="name-asc"
      onSortChange={() => {}}
      tagOptions={[]}
      selectedTags={[]}
      onSelectedTagsChange={() => {}}
      muted={false}
      onMutedChange={() => {}}
      custom={false}
      onCustomChange={() => {}}
    />
  )
  return { onQueryChange, input: screen.getByRole("textbox") }
}

describe("WatchlistToolbar name filter", () => {
  it("labels the group, the input, and the placeholder", () => {
    const { input } = setup("")

    expect(
      screen.getByRole("group", { name: "Filter and sort artists" })
    ).toBeInTheDocument()
    expect(screen.getByRole("textbox", { name: "Filter watchlist" })).toBe(
      input
    )
    expect(input).toHaveAttribute("placeholder", "Filter by name…")
  })

  it("renders no clear × while the query is empty", () => {
    setup("")

    expect(
      screen.queryByRole("button", { name: "Clear name filter" })
    ).not.toBeInTheDocument()
  })

  it("clears through the × and keeps focus in the input", async () => {
    const { onQueryChange, input } = setup("bad")

    await userEvent.click(
      screen.getByRole("button", { name: "Clear name filter" })
    )

    expect(onQueryChange).toHaveBeenCalledWith("")
    expect(input).toHaveFocus()
  })

  it("clears a non-empty value on Esc", async () => {
    const { onQueryChange, input } = setup("bad")

    input.focus()
    await userEvent.keyboard("{Escape}")

    expect(onQueryChange).toHaveBeenCalledWith("")
    expect(input).toHaveFocus()
  })

  it("does nothing on Esc when empty or on Enter", async () => {
    const { onQueryChange, input } = setup("")

    input.focus()
    await userEvent.keyboard("{Escape}")
    await userEvent.keyboard("{Enter}")

    expect(onQueryChange).not.toHaveBeenCalled()
  })
})

describe("WatchlistToolbar sort", () => {
  it("lists the six options in order and reports the chosen key", async () => {
    const onSortChange = vi.fn()
    render(
      <WatchlistToolbar
        query=""
        onQueryChange={() => {}}
        filterInputRef={createRef<HTMLInputElement>()}
        showClear={false}
        onClearFilters={() => {}}
        sort="name-asc"
        onSortChange={onSortChange}
        tagOptions={[]}
        selectedTags={[]}
        onSelectedTagsChange={() => {}}
        muted={false}
        onMutedChange={() => {}}
        custom={false}
        onCustomChange={() => {}}
      />
    )

    const trigger = screen.getByRole("combobox", { name: "Sort by" })
    expect(trigger).toHaveTextContent("Name (A–Z)")
    await userEvent.click(trigger)
    const options = await screen.findAllByRole("option")
    expect(options.map((o) => o.textContent)).toEqual([
      "Name (A–Z)",
      "Name (Z–A)",
      "Date added (newest)",
      "Date added (oldest)",
      "Latest release (newest)",
      "Latest release (oldest)",
    ])

    await userEvent.click(
      screen.getByRole("option", { name: "Date added (oldest)" })
    )
    expect(onSortChange).toHaveBeenCalledWith("added-asc")
  })
})

describe("WatchlistToolbar Show only", () => {
  function renderToggles(muted: boolean, custom: boolean) {
    const onMutedChange = vi.fn()
    const onCustomChange = vi.fn()
    render(
      <WatchlistToolbar
        query=""
        onQueryChange={() => {}}
        filterInputRef={createRef<HTMLInputElement>()}
        showClear={false}
        onClearFilters={() => {}}
        sort="name-asc"
        onSortChange={() => {}}
        tagOptions={[]}
        selectedTags={[]}
        onSelectedTagsChange={() => {}}
        muted={muted}
        onMutedChange={onMutedChange}
        custom={custom}
        onCustomChange={onCustomChange}
      />
    )
    return { onMutedChange, onCustomChange }
  }

  it("renders the legend and two unpressed toggles with no Check", () => {
    renderToggles(false, false)

    expect(screen.getByText("Show only")).toBeInTheDocument()
    for (const name of ["Has muted events", "Custom release types"]) {
      const button = screen.getByRole("button", { name })
      expect(button).toHaveAttribute("aria-pressed", "false")
      expect(button.querySelector("svg")).toBeNull()
    }
  })

  it("shows aria-pressed and a Check only on the active toggle", () => {
    renderToggles(true, false)

    const on = screen.getByRole("button", { name: "Has muted events" })
    expect(on).toHaveAttribute("aria-pressed", "true")
    expect(on.querySelector("svg")).toHaveAttribute("aria-hidden", "true")
    expect(
      screen
        .getByRole("button", { name: "Custom release types" })
        .querySelector("svg")
    ).toBeNull()
  })

  it("reports the flipped value and keeps focus on the clicked button", async () => {
    const { onMutedChange, onCustomChange } = renderToggles(true, false)
    const custom = screen.getByRole("button", { name: "Custom release types" })

    await userEvent.click(
      screen.getByRole("button", { name: "Has muted events" })
    )
    await userEvent.click(custom)

    expect(onMutedChange).toHaveBeenCalledWith(false)
    expect(onCustomChange).toHaveBeenCalledWith(true)
    expect(custom).toHaveFocus()
  })
})
