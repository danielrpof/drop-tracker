import { fireEvent, render, screen } from "@testing-library/react"
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

const RUN = ["A1", "B2", "C3", "D4", "E5"]

function RunHarness() {
  const [key, setKey] = useState("a")
  const [kept, setKept] = useState(RUN)
  const chip = useRef<HTMLButtonElement>(null)
  const capture = useScrollAnchor(key)
  const keep = (ids: string[]) => () => {
    setKept(ids)
    setKey((k) => k + "x")
  }
  return (
    <>
      <ul>
        {RUN.map(
          (id) =>
            kept.includes(id) && (
              <li key={id} data-testid={id}>
                {id === "C3" && (
                  <button ref={chip} data-testid="chip">
                    chip
                  </button>
                )}
              </li>
            )
        )}
      </ul>
      <button onClick={() => chip.current && capture(chip.current)}>
        capture
      </button>
      <button onClick={keep(["A1", "E5"])}>keep ends</button>
      <button onClick={keep(["A1"])}>keep first</button>
    </>
  )
}

describe("useScrollAnchor run of leaving cards (G-25-11)", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  function setup(tops: Record<string, number>) {
    const scrollBy = vi.spyOn(window, "scrollBy").mockImplementation(() => {})
    render(<RunHarness />)
    for (const id of [...RUN, "chip"]) {
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

  const tops = () => ({ A1: 0, B2: 200, C3: 400, D4: 600, E5: 800, chip: 410 })

  it("corrects by the next-side survivor when it ties on distance", async () => {
    const t = tops()
    const { scrollBy, click } = setup(t)

    await click("capture")
    t.E5 = 650
    t.A1 = -50
    await click("keep ends")

    expect(scrollBy).toHaveBeenCalledTimes(1)
    expect(scrollBy).toHaveBeenCalledWith(0, -150)
  })

  it("falls back to the previous side when the next side runs out", async () => {
    const t = tops()
    const { scrollBy, click } = setup(t)

    await click("capture")
    t.A1 = -30
    await click("keep first")

    expect(scrollBy).toHaveBeenCalledTimes(1)
    expect(scrollBy).toHaveBeenCalledWith(0, -30)
  })
})

describe("anchorCandidates", () => {
  it("lists every sibling card nearest-first, next before previous", () => {
    render(
      <ul>
        {RUN.map((id) => (
          <li key={id} data-testid={id}>
            {id === "C3" && <button data-testid="chip" />}
          </li>
        ))}
      </ul>
    )

    expect(anchorCandidates(screen.getByTestId("chip"))).toEqual(
      ["chip", "D4", "B2", "E5", "A1"].map((id) => screen.getByTestId(id))
    )
  })

  it("continues on the next side when the previous side runs out", () => {
    render(
      <ul>
        {RUN.map((id) => (
          <li key={id} data-testid={id}>
            {id === "B2" && <button data-testid="chip" />}
          </li>
        ))}
      </ul>
    )

    expect(anchorCandidates(screen.getByTestId("chip"))).toEqual(
      ["chip", "C3", "A1", "D4", "E5"].map((id) => screen.getByTestId(id))
    )
  })

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

function FocusHarness({
  survivors,
  onFocusLost,
}: {
  survivors: string[]
  onFocusLost: (fallback: HTMLElement | null) => void
}) {
  const [key, setKey] = useState("a")
  const [kept, setKept] = useState(RUN)
  const chip = useRef<HTMLButtonElement>(null)
  const capture = useScrollAnchor(key, onFocusLost)
  const leave = () => {
    setKept(survivors)
    setKey((k) => k + "x")
  }
  return (
    <>
      <ul>
        {RUN.map(
          (id) =>
            kept.includes(id) && (
              <li key={id} data-testid={id}>
                {id === "C3" && (
                  <button
                    ref={chip}
                    data-testid="chip"
                    onClick={(e) => {
                      capture(e.currentTarget, e.detail === 0)
                      leave()
                    }}
                  >
                    chip
                  </button>
                )}
              </li>
            )
        )}
      </ul>
      <button
        onClick={(e) => chip.current && capture(chip.current, e.detail === 0)}
      >
        capture only
      </button>
      <button onClick={leave}>leave</button>
      <button>elsewhere</button>
    </>
  )
}

describe("useScrollAnchor onFocusLost (G-25-12)", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  const rect = (top: number) =>
    ({
      top,
      bottom: top + 10,
      left: 0,
      right: 10,
      width: 10,
      height: 10,
      x: 0,
      y: top,
      toJSON: () => ({}),
    }) as DOMRect

  function setup(survivors: string[]) {
    const scrollBy = vi.spyOn(window, "scrollBy").mockImplementation(() => {})
    const onFocusLost = vi.fn()
    render(<FocusHarness survivors={survivors} onFocusLost={onFocusLost} />)
    // E5 drifts by -150 between capture and the layout effect.
    vi.spyOn(screen.getByTestId("E5"), "getBoundingClientRect")
      .mockReturnValueOnce(rect(800))
      .mockReturnValue(rect(650))
    return { scrollBy, onFocusLost, chip: screen.getByTestId("chip") }
  }

  it("reports the surviving card after the scroll correction for a keyboard capture", () => {
    const { scrollBy, onFocusLost, chip } = setup(["A1", "E5"])

    chip.focus()
    fireEvent.click(chip)

    expect(onFocusLost).toHaveBeenCalledTimes(1)
    expect(onFocusLost).toHaveBeenCalledWith(screen.getByTestId("E5"))
    expect(scrollBy).toHaveBeenCalledWith(0, -150)
    expect(scrollBy.mock.invocationCallOrder[0]).toBeLessThan(
      onFocusLost.mock.invocationCallOrder[0]
    )
  })

  it("reports null when no card survives", () => {
    const { scrollBy, onFocusLost, chip } = setup([])

    chip.focus()
    fireEvent.click(chip)

    expect(onFocusLost).toHaveBeenCalledTimes(1)
    expect(onFocusLost).toHaveBeenCalledWith(null)
    expect(scrollBy).not.toHaveBeenCalled()
  })

  it("stays quiet when the chip's card stays", () => {
    const { onFocusLost, chip } = setup(RUN)

    chip.focus()
    fireEvent.click(chip)

    expect(onFocusLost).not.toHaveBeenCalled()
  })

  it("stays quiet for a mouse click (detail 1) even though the chip held focus", () => {
    const { onFocusLost, chip } = setup(["A1", "E5"])

    chip.focus()
    fireEvent.click(chip, { detail: 1 })

    expect(onFocusLost).not.toHaveBeenCalled()
  })

  it("stays quiet when the chip never held focus", () => {
    const { onFocusLost, chip } = setup(["A1", "E5"])

    fireEvent.click(chip)

    expect(onFocusLost).not.toHaveBeenCalled()
  })

  it("stays quiet when focus moved elsewhere before the key change", () => {
    const { onFocusLost, chip } = setup(["A1", "E5"])

    chip.focus()
    fireEvent.click(screen.getByRole("button", { name: "capture only" }))
    screen.getByRole("button", { name: "elsewhere" }).focus()
    fireEvent.click(screen.getByRole("button", { name: "leave" }))

    expect(chip.isConnected).toBe(false)
    expect(onFocusLost).not.toHaveBeenCalled()
  })
})
