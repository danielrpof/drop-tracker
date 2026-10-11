import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useRef, useState } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"

import { useScrollAnchor } from "~/lib/useScrollAnchor"

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
