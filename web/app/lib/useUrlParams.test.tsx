import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useNavigate } from "react-router"
import { describe, expect, it } from "vitest"

import { LocationProbe, renderRoute } from "~/lib/test/routeStub"
import { urlWriteOptions, useUrlParams } from "~/lib/useUrlParams"

describe("urlWriteOptions", () => {
  it("always prevents scroll reset and defaults to push", () => {
    expect(urlWriteOptions()).toEqual({
      replace: false,
      preventScrollReset: true,
    })
  })

  it("carries replace through", () => {
    expect(urlWriteOptions({ replace: true })).toEqual({
      replace: true,
      preventScrollReset: true,
    })
  })
})

function Harness() {
  const { write } = useUrlParams()
  const navigate = useNavigate()
  return (
    <>
      <LocationProbe />
      <button
        onClick={() => {
          write((p) => p.set("a", "1"))
          write((p) => p.set("b", "2"))
        }}
      >
        write twice
      </button>
      <button onClick={() => void navigate("/?x=1")}>navigate</button>
      <button onClick={() => write((p) => p.set("a", "1"))}>write once</button>
    </>
  )
}

describe("useUrlParams", () => {
  it("lets two writes in one handler both land", async () => {
    renderRoute(Harness, "/")

    await userEvent.click(screen.getByRole("button", { name: "write twice" }))

    const search = screen.getByTestId("location-search").textContent ?? ""
    expect(search).toContain("a=1")
    expect(search).toContain("b=2")
  })

  it("keeps params set by an external navigation on the next write", async () => {
    renderRoute(Harness, "/")

    await userEvent.click(screen.getByRole("button", { name: "navigate" }))
    await userEvent.click(screen.getByRole("button", { name: "write once" }))

    const search = screen.getByTestId("location-search").textContent ?? ""
    expect(search).toContain("x=1")
    expect(search).toContain("a=1")
  })
})
