import { screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { ApiError, getStatus, type StatusResponse } from "~/lib/api"
import { renderRoute } from "~/lib/test/routeStub"

import System from "./system"

// Partial mock keeps the real ApiError class intact -- a bare
// vi.mock("~/lib/api") automock would erase it, silently breaking the
// mount-401 case's instanceof check (RESEARCH Pitfall 5).
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  getStatus: vi.fn(),
}))

const mockGetStatus = vi.mocked(getStatus)

function makeStatus(overrides: Partial<StatusResponse> = {}): StatusResponse {
  return {
    poll_interval_seconds: 900,
    watchlist_size: 12,
    instance: {
      app_version: "a1b2c3d4e5f6",
      schema_applied: 7,
      schema_expected: 7,
    },
    sources: {
      musicbrainz: {
        last_run: null,
        history: [],
        last_skipped_at: null,
        consecutive_skips: 0,
      },
      deezer: {
        last_run: null,
        history: [],
        last_skipped_at: null,
        consecutive_skips: 0,
      },
    },
    ...overrides,
  }
}

describe("System route", () => {
  it("fetches once on mount and renders the heading and app_version", async () => {
    mockGetStatus.mockResolvedValueOnce(makeStatus())

    renderRoute(System, "/system")

    await screen.findByRole("heading", { name: "System" })
    await screen.findByText("a1b2c3d4e5f6")

    expect(mockGetStatus).toHaveBeenCalledTimes(1)
  })

  it("issues no further call while the view remains mounted", async () => {
    mockGetStatus.mockResolvedValueOnce(makeStatus())

    renderRoute(System, "/system")

    await screen.findByText("a1b2c3d4e5f6")
    expect(mockGetStatus).toHaveBeenCalledTimes(1)

    // No interval, timer, or listener exists anywhere in this view (D-10) --
    // flushing pending microtasks and re-asserting the same count is what
    // pins that absence, rather than trusting it structurally.
    await Promise.resolve()
    await Promise.resolve()

    expect(mockGetStatus).toHaveBeenCalledTimes(1)
  })

  it("renders no error surface when the mount fetch 401s", async () => {
    mockGetStatus.mockRejectedValueOnce(new ApiError(401, "unauthenticated"))

    renderRoute(System, "/system")

    await screen.findByRole("heading", { name: "System" })
    // Wait for the rejected promise's catch/finally to settle before
    // asserting the error surface never appeared.
    await Promise.resolve()
    await Promise.resolve()

    expect(
      screen.queryByRole("heading", { name: "Couldn't load system status." })
    ).not.toBeInTheDocument()
  })

  it("renders a single schema line when schema_applied equals schema_expected", async () => {
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        instance: {
          app_version: "a1b2c3d4e5f6",
          schema_applied: 7,
          schema_expected: 7,
        },
      })
    )

    renderRoute(System, "/system")

    await screen.findByText("schema 7")
  })

  it("renders both schema numbers when schema_applied and schema_expected differ", async () => {
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        instance: {
          app_version: "a1b2c3d4e5f6",
          schema_applied: 8,
          schema_expected: 7,
        },
      })
    )

    renderRoute(System, "/system")

    await screen.findByText(/applied 8/)
    expect(screen.getByText(/expects 7/)).toBeInTheDocument()
  })

  it("renders the unreachable pill and an em-dash schema row when schema_applied is null", async () => {
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        instance: {
          app_version: "a1b2c3d4e5f6",
          schema_applied: null,
          schema_expected: 7,
        },
      })
    )

    renderRoute(System, "/system")

    await screen.findByText("Database unreachable")
    expect(screen.getByText("—")).toBeInTheDocument()
  })

  it("shows the empty-watchlist callout with a link to the watchlist root when watchlist_size is zero", async () => {
    mockGetStatus.mockResolvedValueOnce(makeStatus({ watchlist_size: 0 }))

    renderRoute(System, "/system")

    await screen.findByText("Nothing to poll")
    const link = screen.getByRole("link", { name: "Go to the Watchlist" })
    expect(link).toHaveAttribute("href", "/")
  })

  it("hides the empty-watchlist callout when watchlist_size is greater than zero", async () => {
    mockGetStatus.mockResolvedValueOnce(makeStatus({ watchlist_size: 12 }))

    renderRoute(System, "/system")

    await screen.findByRole("heading", { name: "System" })
    await screen.findByText("a1b2c3d4e5f6")

    expect(screen.queryByText("Nothing to poll")).not.toBeInTheDocument()
  })
})
