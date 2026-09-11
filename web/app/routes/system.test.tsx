import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import {
  ApiError,
  getStatus,
  type StatusResponse,
  type StatusRun,
} from "~/lib/api"
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

function makeRun(overrides: Partial<StatusRun> = {}): StatusRun {
  return {
    cycle_id: "musicbrainz-1",
    started_at: "2026-01-01T00:00:00Z",
    finished_at: "2026-01-01T00:05:00Z",
    duration_ms: 5000,
    artists_checked: 10,
    artists_skipped: 0,
    artists_errored: 0,
    events_recorded: 2,
    outcome: "ok",
    summary: "ok — 10 checked, 0 errored, 2 events",
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

  it("renders the last-run badge, verbatim summary, duration, and the clean-run line for a healthy run", async () => {
    const healthyRun = makeRun()
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: healthyRun,
            history: [healthyRun],
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
      })
    )

    renderRoute(System, "/system")

    // "Success" appears twice once the history table lands (19-05): the
    // panel's last-run badge and this same run's table row.
    await waitFor(() => expect(screen.getAllByText("Success")).toHaveLength(2))
    // "5.0s" also appears twice: the panel's duration span and the table's
    // Duration column for this same run.
    expect(screen.getAllByText("5.0s")).toHaveLength(2)
    expect(
      screen.getByText("ok — 10 checked, 0 errored, 2 events")
    ).toBeInTheDocument()
    expect(screen.getByText(/Last clean run/)).toBeInTheDocument()
  })

  it("renders 'No clean run in recent history' when every ok run in history errored some artists", async () => {
    const erroredOkRun = makeRun({ outcome: "ok", artists_errored: 2 })
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: erroredOkRun,
            history: [erroredOkRun],
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
      })
    )

    renderRoute(System, "/system")

    await screen.findByText("No clean run in recent history")
  })

  it("renders both the runless line and the skip line for a runless-but-skipping source", async () => {
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: null,
            history: [],
            last_skipped_at: "2026-01-01T00:10:00Z",
            consecutive_skips: 3,
          },
          deezer: {
            last_run: null,
            history: [],
            last_skipped_at: null,
            consecutive_skips: 0,
          },
        },
      })
    )

    renderRoute(System, "/system")

    await screen.findByText("No poll cycles recorded for MusicBrainz yet.")
    expect(screen.getByText(/Skipped 3 consecutive cycles/)).toBeInTheDocument()
  })

  it("renders the Interrupted badge and the escalation line when the latest run is cancelled", async () => {
    const cancelledRun = makeRun({ outcome: "cancelled" })
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: cancelledRun,
            history: [cancelledRun],
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
      })
    )

    renderRoute(System, "/system")

    // "Interrupted" appears twice once the history table lands (19-05): the
    // panel's last-run badge and this same run's table row.
    await waitFor(() =>
      expect(screen.getAllByText("Interrupted")).toHaveLength(2)
    )
    expect(
      screen.getByText(
        "Recent cycles are being interrupted — check for a restart or crash loop."
      )
    ).toBeInTheDocument()
  })

  it("does not render the escalation line when the latest run is ok", async () => {
    const okRun = makeRun({ outcome: "ok" })
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: okRun,
            history: [okRun],
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
      })
    )

    renderRoute(System, "/system")

    // "Success" appears twice once the history table lands (19-05): the
    // panel's last-run badge and this same run's table row.
    await waitFor(() => expect(screen.getAllByText("Success")).toHaveLength(2))
    expect(
      screen.queryByText(
        "Recent cycles are being interrupted — check for a restart or crash loop."
      )
    ).not.toBeInTheDocument()
  })

  it("renders the MusicBrainz panel before the Deezer panel even when the fixture lists them in reverse", async () => {
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          deezer: {
            last_run: null,
            history: [],
            last_skipped_at: null,
            consecutive_skips: 0,
          },
          // A non-zero skip count keeps this fixture out of the first-run
          // shape (D-04) so the panels this test asserts on actually render.
          musicbrainz: {
            last_run: null,
            history: [],
            last_skipped_at: "2026-01-01T00:10:00Z",
            consecutive_skips: 1,
          },
        },
      })
    )

    renderRoute(System, "/system")

    const musicbrainzHeading = await screen.findByRole("heading", {
      name: "MusicBrainz",
    })
    const deezerHeading = await screen.findByRole("heading", {
      name: "Deezer",
    })

    expect(
      musicbrainzHeading.compareDocumentPosition(deezerHeading) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  it("still renders a source key absent from SOURCE_ORDER, appended after the known panels", async () => {
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          // A non-zero skip count keeps this fixture out of the first-run
          // shape (D-04) so the panels this test asserts on actually render.
          musicbrainz: {
            last_run: null,
            history: [],
            last_skipped_at: "2026-01-01T00:10:00Z",
            consecutive_skips: 1,
          },
          deezer: {
            last_run: null,
            history: [],
            last_skipped_at: null,
            consecutive_skips: 0,
          },
          spotify: {
            last_run: null,
            history: [],
            last_skipped_at: null,
            consecutive_skips: 0,
          },
        },
      })
    )

    renderRoute(System, "/system")

    const deezerHeading = await screen.findByRole("heading", {
      name: "Deezer",
    })
    const spotifyHeading = await screen.findByRole("heading", {
      name: "spotify",
    })

    expect(
      deezerHeading.compareDocumentPosition(spotifyHeading) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  it("renders the cap caption when a source's history has fifty entries", async () => {
    const history = Array.from({ length: 50 }, (_, i) =>
      makeRun({ cycle_id: `musicbrainz-${i}` })
    )
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: history[0],
            history,
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
      })
    )

    renderRoute(System, "/system")

    await screen.findByText(
      "Showing the 50 most recent cycles for this source. Older history isn't retained — it lives only in the logs."
    )
  })

  it("renders the singular count caption for a single-entry history", async () => {
    const run = makeRun()
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: run,
            history: [run],
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
      })
    )

    renderRoute(System, "/system")

    await screen.findByText(
      "1 cycle recorded for this source since the last restart."
    )
  })

  it("renders no table element when a source has zero history entries", async () => {
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: null,
            history: [],
            last_skipped_at: "2026-01-01T00:10:00Z",
            consecutive_skips: 3,
          },
          deezer: {
            last_run: null,
            history: [],
            last_skipped_at: null,
            consecutive_skips: 0,
          },
        },
      })
    )

    renderRoute(System, "/system")

    await screen.findByText("No poll cycles recorded for MusicBrainz yet.")
    expect(screen.queryByRole("table")).not.toBeInTheDocument()
  })

  it("renders the first-run state naming the humanized poll interval when no source has ever run", async () => {
    mockGetStatus.mockResolvedValueOnce(makeStatus())

    renderRoute(System, "/system")

    await screen.findByRole("heading", { name: "No poll cycles yet" })
    expect(
      screen.getByText(/The scheduler runs every 15 minutes/)
    ).toBeInTheDocument()
  })

  it("moves from loaded to first-run when a Refresh returns an all-runless payload", async () => {
    const healthyRun = makeRun()
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: healthyRun,
            history: [healthyRun],
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
      })
    )

    renderRoute(System, "/system")

    await screen.findByRole("heading", { name: "MusicBrainz" })

    mockGetStatus.mockResolvedValueOnce(makeStatus())
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }))

    await screen.findByRole("heading", { name: "No poll cycles yet" })
    expect(
      screen.queryByRole("heading", { name: "MusicBrainz" })
    ).not.toBeInTheDocument()
  })

  it("hides the Refresh control in the error state", async () => {
    mockGetStatus.mockRejectedValueOnce(new Error("network down"))

    renderRoute(System, "/system")

    await screen.findByRole("heading", {
      name: "Couldn't load system status.",
    })
    expect(
      screen.queryByRole("button", { name: "Refresh" })
    ).not.toBeInTheDocument()
  })

  it("issues exactly two total fetch calls across mount plus a double-clicked Refresh", async () => {
    mockGetStatus.mockResolvedValueOnce(makeStatus())

    renderRoute(System, "/system")

    await screen.findByRole("button", { name: "Refresh" })
    expect(mockGetStatus).toHaveBeenCalledTimes(1)

    // The refresh call is held open (never resolved) so the second half of
    // the double-click genuinely lands while the first is still in flight --
    // a mock that resolves instantly would let the first click's request
    // complete before the second click fires, testing nothing about
    // re-entrancy.
    mockGetStatus.mockImplementation(() => new Promise(() => {}))

    await userEvent.dblClick(screen.getByRole("button", { name: "Refresh" }))

    expect(mockGetStatus).toHaveBeenCalledTimes(2)
  })

  it("keeps previously rendered values on screen and shows the failed-refresh line when a Refresh fails", async () => {
    const healthyRun = makeRun()
    mockGetStatus.mockResolvedValueOnce(
      makeStatus({
        sources: {
          musicbrainz: {
            last_run: healthyRun,
            history: [healthyRun],
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
      })
    )

    renderRoute(System, "/system")

    await screen.findByRole("heading", { name: "MusicBrainz" })

    mockGetStatus.mockRejectedValueOnce(new Error("network down"))
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }))

    await screen.findByText(/Couldn't refresh — still showing data as of/)
    expect(
      screen.getByRole("heading", { name: "MusicBrainz" })
    ).toBeInTheDocument()
    expect(screen.getAllByText("Success").length).toBeGreaterThan(0)
  })

  it("renders neither the error state nor the inline failure line when a Refresh rejects with a session expiry", async () => {
    mockGetStatus.mockResolvedValueOnce(makeStatus())

    renderRoute(System, "/system")

    await screen.findByRole("heading", { name: "No poll cycles yet" })

    mockGetStatus.mockRejectedValueOnce(new ApiError(401, "unauthenticated"))
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }))

    await Promise.resolve()
    await Promise.resolve()

    expect(
      screen.queryByRole("heading", { name: "Couldn't load system status." })
    ).not.toBeInTheDocument()
    expect(screen.queryByText(/Couldn't refresh/)).not.toBeInTheDocument()
  })

  it("sets no state and emits no unmounted-component warning when a Refresh resolves after unmount", async () => {
    mockGetStatus.mockResolvedValueOnce(makeStatus())

    const { unmount } = renderRoute(System, "/system")

    await screen.findByRole("button", { name: "Refresh" })

    let resolveRefresh: (value: StatusResponse) => void = () => {}
    mockGetStatus.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveRefresh = resolve
        })
    )

    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {})

    await userEvent.click(screen.getByRole("button", { name: "Refresh" }))
    unmount()
    resolveRefresh(makeStatus())

    await Promise.resolve()
    await Promise.resolve()

    const unmountedWarning = consoleError.mock.calls.some(
      ([msg]) => typeof msg === "string" && msg.includes("unmounted component")
    )
    expect(unmountedWarning).toBe(false)

    consoleError.mockRestore()
  })
})
