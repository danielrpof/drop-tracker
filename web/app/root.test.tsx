import { render, screen } from "@testing-library/react"
import { createRoutesStub } from "react-router"
import { describe, expect, it } from "vitest"

import App, { ErrorBoundary } from "./root"

// isRouteErrorResponse (react-router) duck-types on these four fields --
// there is no public constructor for the real ErrorResponseImpl class, so a
// plain object matching this shape is the documented way to simulate one in
// a unit test.
function routeErrorResponse(status: number, statusText: string) {
  return { status, statusText, internal: false, data: null }
}

describe("ErrorBoundary", () => {
  it("shows the 404 message and copy for a 404 route error", () => {
    render(
      <ErrorBoundary
        params={{}}
        error={routeErrorResponse(404, "Not Found")}
      />,
    )

    expect(screen.getByRole("heading", { name: "404" })).toBeInTheDocument()
    expect(
      screen.getByText("The requested page could not be found."),
    ).toBeInTheDocument()
  })

  it("shows the generic Error message and the response's status text for a non-404 route error", () => {
    render(
      <ErrorBoundary
        params={{}}
        error={routeErrorResponse(500, "Internal Server Error")}
      />,
    )

    expect(screen.getByRole("heading", { name: "Error" })).toBeInTheDocument()
    expect(screen.getByText("Internal Server Error")).toBeInTheDocument()
  })

  it("falls back to the default details copy when a non-404 route error carries no status text", () => {
    render(<ErrorBoundary params={{}} error={routeErrorResponse(500, "")} />)

    expect(screen.getByRole("heading", { name: "Error" })).toBeInTheDocument()
    expect(
      screen.getByText("An unexpected error occurred."),
    ).toBeInTheDocument()
  })

  it("shows an unrecognized error value's default Oops copy with no stack trace", () => {
    const { container } = render(
      <ErrorBoundary params={{}} error={"not an Error"} />,
    )

    expect(screen.getByRole("heading", { name: "Oops!" })).toBeInTheDocument()
    expect(
      screen.getByText("An unexpected error occurred."),
    ).toBeInTheDocument()
    expect(container.querySelector("pre")).not.toBeInTheDocument()
  })
})

describe("App", () => {
  function renderAppAt(path: string) {
    const Stub = createRoutesStub([
      {
        path: "/",
        Component: App,
        children: [
          { path: "watchlist", Component: () => <div>Watchlist page</div> },
          { path: "history", Component: () => <div>History page</div> },
        ],
      },
    ])
    return render(<Stub initialEntries={[path]} />)
  }

  it("marks the Watchlist tab active and the History tab inactive on /watchlist", () => {
    renderAppAt("/watchlist")

    expect(screen.getByRole("link", { name: "Watchlist" }).className).toContain(
      "border-accent-indigo",
    )
    expect(screen.getByRole("link", { name: "History" }).className).toContain(
      "border-transparent",
    )
  })

  it("marks the History tab active and the Watchlist tab inactive on /history", () => {
    renderAppAt("/history")

    expect(screen.getByRole("link", { name: "History" }).className).toContain(
      "border-accent-indigo",
    )
    expect(screen.getByRole("link", { name: "Watchlist" }).className).toContain(
      "border-transparent",
    )
  })
})
