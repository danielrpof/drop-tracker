import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useRef, useState } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"

import { anchorCandidates, useScrollAnchor } from "~/lib/useScrollAnchor"

function Harness() {
  const [key, setKey] = useState("a")
  const [shown, setShown] = useState(true)
  const target = useRef<HTMLDivElement>(null)
  const capture = useScrollAnchor(key)
  return (
    <>
      {shown && <div ref={target} data-testid="target" />}
      <button onClick={() => target.current && capture(target.current)}>
        capture
      </button>
      <button onClick={() => setKey((k) => k + "x")}>bump</button>
      <button
        onClick={() => {
          setShown(false)
          setKey((k) => k + "x")
        }}
      >
        drop
      </button>
    </>
  )
}

describe("useScrollAnchor", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  function setup(initialTop: number) {
    const scrollBy = vi.spyOn(window, "scrollBy").mockImplementation(() => {})
    render(<Harness />)
    const state = { top: initialTop }
    vi.spyOn(
      screen.getByTestId("target"),
      "getBoundingClientRect"
    ).mockImplementation(
      () =>
        ({
          top: state.top,
          bottom: state.top + 10,
          left: 0,
          right: 10,
          width: 10,
          height: 10,
          x: 0,
          y: state.top,
          toJSON: () => ({}),
        }) as DOMRect
    )
    const click = (name: string) =>
      userEvent.click(screen.getByRole("button", { name }))
    return { scrollBy, state, click }
  }

  it("scrolls by the anchor's drift after the key changes", async () => {
    const { scrollBy, state, click } = setup(400)

    await click("capture")
    state.top = 150
    await click("bump")

    expect(scrollBy).toHaveBeenCalledTimes(1)
    expect(scrollBy).toHaveBeenCalledWith(0, -250)
  })

  it("calls no scroll API when the anchor did not move", async () => {
    const { scrollBy, click } = setup(400)

    await click("capture")
    await click("bump")

    expect(scrollBy).not.toHaveBeenCalled()
  })

  it("skips the correction when the anchor leaves the DOM", async () => {
    const { scrollBy, state, click } = setup(400)

    await click("capture")
    state.top = 150
    await click("drop")

    expect(scrollBy).not.toHaveBeenCalled()
  })

  it("does nothing on a key change without a capture", async () => {
    const { scrollBy, state, click } = setup(400)

    state.top = 150
    await click("bump")

    expect(scrollBy).not.toHaveBeenCalled()
  })

  it("consumes the anchor on one key change only", async () => {
    const { scrollBy, state, click } = setup(400)

    await click("capture")
    state.top = 150
    await click("bump")
    state.top = 50
    await click("bump")

    expect(scrollBy).toHaveBeenCalledTimes(1)
  })
})

type Shown = { prev: boolean; card: boolean; next: boolean }

function ListHarness() {
  const [key, setKey] = useState("a")
  const [shown, setShown] = useState<Shown>({
    prev: true,
    card: true,
    next: true,
  })
  const chip = useRef<HTMLButtonElement>(null)
  const capture = useScrollAnchor(key)
  const drop = (next: Shown) => () => {
    setShown(next)
    setKey((k) => k + "x")
  }
  return (
    <>
      <ul>
        {shown.prev && <li data-testid="prev" />}
        {shown.card && (
          <li data-testid="card">
            <button ref={chip} data-testid="chip">
              chip
            </button>
          </li>
        )}
        {shown.next && <li data-testid="next" />}
      </ul>
      <button onClick={() => chip.current && capture(chip.current)}>
        capture
      </button>
      <button onClick={() => setKey((k) => k + "x")}>bump</button>
      <button onClick={drop({ prev: true, card: false, next: true })}>
        drop card
      </button>
      <button onClick={drop({ prev: true, card: false, next: false })}>
        drop card and next
      </button>
      <button onClick={drop({ prev: false, card: false, next: false })}>
        drop all
      </button>
    </>
  )
}

describe("useScrollAnchor fallback candidates (WR-02)", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  function setup(tops: Record<string, number>) {
    const scrollBy = vi.spyOn(window, "scrollBy").mockImplementation(() => {})
    render(<ListHarness />)
    for (const id of ["prev", "card", "next", "chip"]) {
      vi.spyOn(
        screen.getByTestId(id),
        "getBoundingClientRect"
      ).mockImplementation(
        () =>
          ({
            top: tops[id],
            bottom: tops[id] + 10,
            left: 0,
            right: 10,
            width: 10,
            height: 10,
            x: 0,
            y: tops[id],
            toJSON: () => ({}),
          }) as DOMRect
      )
    }
    const click = (name: string) =>
      userEvent.click(screen.getByRole("button", { name }))
    return { scrollBy, click }
  }

  it("corrects by the next card when the clicked card leaves", async () => {
    const tops = { prev: 100, card: 300, next: 500, chip: 310 }
    const { scrollBy, click } = setup(tops)

    await click("capture")
    tops.next = 350
    tops.prev = 90
    await click("drop card")

    expect(scrollBy).toHaveBeenCalledTimes(1)
    expect(scrollBy).toHaveBeenCalledWith(0, -150)
  })

  it("corrects by the previous card when the card and its next both leave", async () => {
    const tops = { prev: 100, card: 300, next: 500, chip: 310 }
    const { scrollBy, click } = setup(tops)

    await click("capture")
    tops.prev = 40
    await click("drop card and next")

    expect(scrollBy).toHaveBeenCalledTimes(1)
    expect(scrollBy).toHaveBeenCalledWith(0, -60)
  })

  it("calls no scroll API when no candidate survives", async () => {
    const tops = { prev: 100, card: 300, next: 500, chip: 310 }
    const { scrollBy, click } = setup(tops)

    await click("capture")
    await click("drop all")

    expect(scrollBy).not.toHaveBeenCalled()
  })

  it("prefers the connected chip over a neighbour's drift", async () => {
    const tops = { prev: 100, card: 300, next: 500, chip: 300 }
    const { scrollBy, click } = setup(tops)

    await click("capture")
    tops.chip = 250
    tops.next = 100
    await click("bump")

    expect(scrollBy).toHaveBeenCalledTimes(1)
    expect(scrollBy).toHaveBeenCalledWith(0, -50)
  })
})

describe("anchorCandidates", () => {
  it("lists the chip, then the next and previous cards", () => {
    render(
      <ul>
        <li data-testid="a" />
        <li data-testid="b">
          <button data-testid="chip" />
        </li>
        <li data-testid="c" />
      </ul>
    )

    expect(anchorCandidates(screen.getByTestId("chip"))).toEqual([
      screen.getByTestId("chip"),
      screen.getByTestId("c"),
      screen.getByTestId("a"),
    ])
  })

  it("omits a missing neighbour", () => {
    render(
      <ul>
        <li>
          <button data-testid="chip" />
        </li>
        <li data-testid="b" />
      </ul>
    )

    expect(anchorCandidates(screen.getByTestId("chip"))).toEqual([
      screen.getByTestId("chip"),
      screen.getByTestId("b"),
    ])
  })

  it("returns only the element outside any list item", () => {
    render(<div data-testid="solo" />)

    expect(anchorCandidates(screen.getByTestId("solo"))).toEqual([
      screen.getByTestId("solo"),
    ])
  })
})
