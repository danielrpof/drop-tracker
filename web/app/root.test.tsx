import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { ComponentType } from "react"
import { createRoutesStub } from "react-router"
import { beforeEach, describe, expect, it, vi } from "vitest"

import App, { ErrorBoundary } from "./root"

// deleteSession is a controllable spy; the auth store is driven directly.
// sonner is stubbed so the Log out toasts are assertable and the real
// Toaster (imported transitively via ~/components/ui/sonner) is inert.
vi.mock("~/lib/api")
vi.mock("sonner", () => ({
  Toaster: () => null,
  toast: { success: vi.fn(), error: vi.fn() },
}))

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
      <ErrorBoundary params={{}} error={routeErrorResponse(404, "Not Found")} />
    )

    expect(screen.getByRole("heading", { name: "404" })).toBeInTheDocument()
    expect(
      screen.getByText("The requested page could not be found.")
    ).toBeInTheDocument()
  })

  it("shows the generic Error message and the response's status text for a non-404 route error", () => {
    render(
      <ErrorBoundary
        params={{}}
        error={routeErrorResponse(500, "Internal Server Error")}
      />
    )

    expect(screen.getByRole("heading", { name: "Error" })).toBeInTheDocument()
    expect(screen.getByText("Internal Server Error")).toBeInTheDocument()
  })

  it("falls back to the default details copy when a non-404 route error carries no status text", () => {
    render(<ErrorBoundary params={{}} error={routeErrorResponse(500, "")} />)

    expect(screen.getByRole("heading", { name: "Error" })).toBeInTheDocument()
    expect(
      screen.getByText("An unexpected error occurred.")
    ).toBeInTheDocument()
  })

  it("shows an unrecognized error value's default Oops copy with no stack trace", () => {
    const { container } = render(
      <ErrorBoundary params={{}} error={"not an Error"} />
    )

    expect(screen.getByRole("heading", { name: "Oops!" })).toBeInTheDocument()
    expect(
      screen.getByText("An unexpected error occurred.")
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
          { index: true, Component: () => <div>Watchlist page</div> },
          { path: "history", Component: () => <div>History page</div> },
        ],
      },
    ])
    return render(<Stub initialEntries={[path]} />)
  }

  it("marks the Watchlist tab active and the History tab inactive on /", () => {
    renderAppAt("/")

    expect(screen.getByRole("link", { name: "Watchlist" }).className).toContain(
      "border-accent-indigo"
    )
    expect(screen.getByRole("link", { name: "History" }).className).toContain(
      "border-transparent"
    )
  })

  it("marks the History tab active and the Watchlist tab inactive on /history", () => {
    renderAppAt("/history")

    expect(screen.getByRole("link", { name: "History" }).className).toContain(
      "border-accent-indigo"
    )
    expect(screen.getByRole("link", { name: "Watchlist" }).className).toContain(
      "border-transparent"
    )
  })
})

describe("App — instance passphrase gate", () => {
  let GatedApp: ComponentType
  let authStore: (typeof import("~/lib/authStore"))["authStore"]
  let mockDeleteSession: ReturnType<typeof vi.fn>
  let toastError: ReturnType<typeof vi.fn>

  beforeEach(async () => {
    // Storage reset MUST come first -- before vi.resetModules() and the
    // dynamic re-import below. jsdom keeps one sessionStorage per file and
    // vi.resetModules() does not clear it; authStore seeds gateActive from
    // the store at import time, so a gate recorded by an earlier case would
    // otherwise leak into "does not render the Log out control when the gate
    // is not active".
    sessionStorage.clear()
    // Fresh module registry so App and the test share one authStore instance.
    vi.resetModules()
    ;({ authStore } = await import("~/lib/authStore"))
    mockDeleteSession = vi.mocked((await import("~/lib/api")).deleteSession)
    toastError = vi.mocked((await import("sonner")).toast.error)
    ;({ default: GatedApp } = await import("./root"))
  })

  function renderAppAt(path: string) {
    const Stub = createRoutesStub([
      {
        path: "/",
        Component: GatedApp,
        children: [
          { index: true, Component: () => <div>Watchlist page</div> },
          { path: "history", Component: () => <div>History page</div> },
        ],
      },
    ])
    return render(<Stub initialEntries={[path]} />)
  }

  function logoutButton() {
    return screen.getByRole("button", { name: /log out/i })
  }

  it("renders the passphrase screen and no nav when the store is unauthenticated", () => {
    authStore.markUnauthenticated()

    renderAppAt("/")

    expect(
      screen.getByRole("heading", { name: "Enter the instance passphrase" })
    ).toBeInTheDocument()
    expect(screen.queryByRole("navigation")).not.toBeInTheDocument()
    expect(screen.queryByText("Watchlist page")).not.toBeInTheDocument()
  })

  it("renders the nav and the routed page when the store is authenticated", () => {
    renderAppAt("/")

    expect(screen.getByRole("navigation")).toBeInTheDocument()
    expect(screen.getByText("Watchlist page")).toBeInTheDocument()
  })

  it("does not render the Log out control when the gate is not active", () => {
    renderAppAt("/")

    expect(
      screen.queryByRole("button", { name: /log out/i })
    ).not.toBeInTheDocument()
  })

  it("renders the Log out control once the gate is active", () => {
    authStore.markAuthenticated() // sets gateActive true, authed stays true

    renderAppAt("/")

    expect(logoutButton()).toBeInTheDocument()
  })

  it("keeps the Log out control after a document reload with a valid cookie — no 401, no login (G-14-2 regression)", async () => {
    // Simulate a full document reload while dt_session is still valid: the
    // browser session already recorded the gate as active on a previous
    // page load, this load fires no 401 (loader fetch returns 200) and
    // performs no login. gateActive must be seeded from the session store,
    // so the control is still there -- with no mark* call anywhere here.
    sessionStorage.setItem("dt_gate_active", "1")
    vi.resetModules()
    ;({ authStore } = await import("~/lib/authStore"))
    const { default: ReloadedApp } = await import("./root")

    const Stub = createRoutesStub([
      {
        path: "/",
        Component: ReloadedApp,
        children: [
          { index: true, Component: () => <div>Watchlist page</div> },
          { path: "history", Component: () => <div>History page</div> },
        ],
      },
    ])
    render(<Stub initialEntries={["/"]} />)

    expect(logoutButton()).toBeInTheDocument()
  })

  it("returns to the passphrase screen after a successful logout", async () => {
    authStore.markAuthenticated()
    mockDeleteSession.mockResolvedValueOnce(undefined)

    renderAppAt("/")
    await userEvent.click(logoutButton())

    expect(
      await screen.findByRole("heading", {
        name: "Enter the instance passphrase",
      })
    ).toBeInTheDocument()
    expect(mockDeleteSession).toHaveBeenCalled()
  })

  // 14-UI-SPEC E5 held-out backstop check: a failing DELETE /session must
  // still clear local auth state (a stale HttpOnly cookie just yields a 401
  // next fetch) and surface a failure toast.
  it("still returns to the passphrase screen and shows a failure toast when logout fails (E5 backstop)", async () => {
    authStore.markAuthenticated()
    mockDeleteSession.mockRejectedValueOnce(new Error("500"))

    renderAppAt("/")
    await userEvent.click(logoutButton())

    expect(
      await screen.findByRole("heading", {
        name: "Enter the instance passphrase",
      })
    ).toBeInTheDocument()
    expect(toastError).toHaveBeenCalledWith("Couldn't log out.")
  })

  it("is harmless to click Log out twice — the control is never disabled and the delete is idempotent", async () => {
    authStore.markAuthenticated()
    let resolve: () => void = () => {}
    mockDeleteSession.mockReturnValue(
      new Promise<void>((r) => {
        resolve = r
      })
    )

    renderAppAt("/")
    const button = logoutButton()
    await userEvent.click(button)
    await userEvent.click(button)

    expect(button).toBeEnabled()
    expect(mockDeleteSession).toHaveBeenCalledTimes(2)

    resolve()
    expect(
      await screen.findByRole("heading", {
        name: "Enter the instance passphrase",
      })
    ).toBeInTheDocument()
  })
})
