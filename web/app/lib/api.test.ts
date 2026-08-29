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
      })
    )

    const err = await listEvents().then(
      () => {
        throw new Error("expected listEvents() to reject")
      },
      (e: unknown) => e
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
      })
    )

    const err = await listEvents().then(
      () => {
        throw new Error("expected listEvents() to reject")
      },
      (e: unknown) => e
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
      })
    )

    await expect(listEvents()).resolves.toEqual(page)
  })

  describe("listEvents query-string construction", () => {
    beforeEach(() => {
      fetchMock.mockResolvedValue(
        new Response(
          JSON.stringify({
            events: [],
            next_cursor: null,
            has_older_events: false,
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }
        )
      )
    })

    it("carries no query string when called with no arguments", async () => {
      await listEvents()

      expect(fetchMock.mock.calls[0][0]).toBe("/events")
    })

    it("carries exactly the cursor parameter when called with only a cursor", async () => {
      await listEvents({ cursor: "eyJpIjo0Mn0" })

      expect(fetchMock.mock.calls[0][0]).toBe("/events?cursor=eyJpIjo0Mn0")
    })

    it("carries all three parameters with the wire-level names when called with all three", async () => {
      await listEvents({
        artistId: 7,
        eventType: "new_release",
        cursor: "eyJpIjo0Mn0",
      })

      // The Go handler expects snake_case query params -- artistId/eventType
      // do not match artist_id/event_type verbatim.
      expect(fetchMock.mock.calls[0][0]).toBe(
        "/events?artist_id=7&event_type=new_release&cursor=eyJpIjo0Mn0"
      )
    })
  })
})

// The 401 interceptor, the CSRF header, and the two session wrappers all
// touch the module-level authStore singleton, so this block resets the
// module registry per case and re-imports both modules fresh. It still
// stubs the runtime's own fetch (never mocks ~/lib/api -- the comment at
// the top of this file explains why that must not happen here).
describe("apiFetch auth behaviour (401 interceptor, CSRF header, session wrappers)", () => {
  let api: typeof import("~/lib/api")
  let authStore: (typeof import("~/lib/authStore"))["authStore"]
  let fetchSpy: ReturnType<typeof vi.fn>

  beforeEach(async () => {
    vi.resetModules()
    api = await import("~/lib/api")
    ;({ authStore } = await import("~/lib/authStore"))
    fetchSpy = vi.fn()
    vi.stubGlobal("fetch", fetchSpy)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function jsonResponse(body: unknown, status = 200) {
    return new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    })
  }

  it("flips the shared store to unauthenticated and still rejects with an ApiError carrying status 401", async () => {
    fetchSpy.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "unauthenticated" }), { status: 401 })
    )

    const err = await api.listWatchlist().then(
      () => {
        throw new Error("expected listWatchlist() to reject")
      },
      (e: unknown) => e
    )

    expect(err).toBeInstanceOf(api.ApiError)
    expect((err as InstanceType<typeof api.ApiError>).status).toBe(401)
    expect(authStore.isAuthed()).toBe(false)
    expect(authStore.isGateActive()).toBe(true)
  })

  it("leaves the store untouched on a 200 response", async () => {
    const listener = vi.fn()
    authStore.subscribe(listener)
    fetchSpy.mockResolvedValueOnce(jsonResponse([]))

    await api.listWatchlist()

    expect(authStore.isAuthed()).toBe(true)
    expect(listener).not.toHaveBeenCalled()
  })

  it("carries the X-Requested-With: drop-tracker header on POST, PATCH and DELETE but not GET", async () => {
    // A fresh Response per call -- a single Response body can only be read once.
    fetchSpy.mockImplementation(() => Promise.resolve(jsonResponse({ id: 1 })))

    await api.addWatchlist({ mbid: "m", name: "n" })
    await api.updateWatchlistPreferences(1, { releaseTypes: ["album"] })

    fetchSpy.mockImplementationOnce(() =>
      Promise.resolve(new Response(null, { status: 204 }))
    )
    await api.removeWatchlist(1)

    fetchSpy.mockImplementationOnce(() => Promise.resolve(jsonResponse([])))
    await api.listWatchlist()

    const headerFor = (i: number) =>
      new Headers((fetchSpy.mock.calls[i][1] as RequestInit).headers).get(
        "X-Requested-With"
      )

    expect(headerFor(0)).toBe("drop-tracker") // POST
    expect(headerFor(1)).toBe("drop-tracker") // PATCH
    expect(headerFor(2)).toBe("drop-tracker") // DELETE
    expect(headerFor(3)).toBeNull() // GET
  })

  it("createSession POSTs the passphrase in the JSON body of /session and resolves on 204", async () => {
    fetchSpy.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await expect(api.createSession("open-sesame")).resolves.toBeUndefined()

    const [url, init] = fetchSpy.mock.calls[0] as [string, RequestInit]
    expect(url).toBe("/session")
    expect(init.method).toBe("POST")
    expect(JSON.parse(init.body as string)).toEqual({ passphrase: "open-sesame" })
  })

  it("createSession does not flip auth state itself", async () => {
    authStore.markUnauthenticated()
    fetchSpy.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await api.createSession("open-sesame")

    expect(authStore.isAuthed()).toBe(false)
  })

  it("deleteSession issues a DELETE to /session and resolves on 204", async () => {
    fetchSpy.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await expect(api.deleteSession()).resolves.toBeUndefined()

    const [url, init] = fetchSpy.mock.calls[0] as [string, RequestInit]
    expect(url).toBe("/session")
    expect(init.method).toBe("DELETE")
  })

  it("converges on one consistent unauthenticated state when several endpoint calls fail with 401 at once (GATE-05 concurrency)", async () => {
    fetchSpy.mockResolvedValue(
      new Response(JSON.stringify({ error: "unauthenticated" }), { status: 401 })
    )

    const results = await Promise.allSettled([
      api.listWatchlist(),
      api.listEvents(),
      api.listWatchlist(),
    ])

    expect(results.map((r) => r.status)).toEqual([
      "rejected",
      "rejected",
      "rejected",
    ])
    expect(authStore.isAuthed()).toBe(false)
    expect(authStore.isGateActive()).toBe(true)
  })
})
