import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import {
  ApiError,
  listEvents,
  removeWatchlist,
  type EventsPage,
} from "~/lib/api"

// This is the one file in the suite that stubs the runtime's own fetch --
// apiFetch is the boundary every other test mocks around (TEST-02), and
// there is no seam below it. The stub is installed fresh per test and torn
// down in afterEach so it can never leak into another file. Held as a local
// reference (not re-read via a module-mock helper) so this file never
// touches the module-mocking machinery it exists to bypass.
let fetchMock: ReturnType<typeof vi.fn>

describe("apiFetch (via the exported endpoint wrappers)", () => {
  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal("fetch", fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("throws ApiError carrying the HTTP status and the server's error message for a non-OK JSON body", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "artist not found" }), {
        status: 404,
        statusText: "Not Found",
      }),
    )

    const err = await listEvents().then(
      () => {
        throw new Error("expected listEvents() to reject")
      },
      (e: unknown) => e,
    )

    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(404)
    expect((err as ApiError).message).toBe("artist not found")
  })

  it("falls back to the response's status text when the error body is not valid JSON", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("not json{{{", {
        status: 500,
        statusText: "Internal Server Error",
      }),
    )

    const err = await listEvents().then(
      () => {
        throw new Error("expected listEvents() to reject")
      },
      (e: unknown) => e,
    )

    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(500)
    expect((err as ApiError).message).toBe("Internal Server Error")
  })

  it("resolves to undefined for a no-content response without attempting to parse a body", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await expect(removeWatchlist(42)).resolves.toBeUndefined()
  })

  it("resolves an OK response to the parsed body", async () => {
    const page: EventsPage = {
      events: [],
      next_cursor: null,
      has_older_events: false,
    }
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify(page), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    )

    await expect(listEvents()).resolves.toEqual(page)
  })

  describe("listEvents query-string construction", () => {
    beforeEach(() => {
      fetchMock.mockResolvedValue(
        new Response(
          JSON.stringify({ events: [], next_cursor: null, has_older_events: false }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      )
    })

    it("carries no query string when called with no arguments", async () => {
      await listEvents()

      expect(fetchMock.mock.calls[0][0]).toBe("/events")
    })

    it("carries exactly the cursor parameter when called with only a cursor", async () => {
      await listEvents({ cursor: 42 })

      expect(fetchMock.mock.calls[0][0]).toBe("/events?cursor=42")
    })

    it("carries all three parameters with the wire-level names when called with all three", async () => {
      await listEvents({ artistId: 7, eventType: "new_release", cursor: 42 })

      // The Go handler expects snake_case query params -- artistId/eventType
      // do not match artist_id/event_type verbatim.
      expect(fetchMock.mock.calls[0][0]).toBe(
        "/events?artist_id=7&event_type=new_release&cursor=42",
      )
    })
  })
})
