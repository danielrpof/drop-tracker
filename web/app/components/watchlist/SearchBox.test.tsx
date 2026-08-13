import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { SearchBox } from "./SearchBox"
import { searchArtists, type SearchResponse } from "~/lib/api"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")

const mockSearchArtists = vi.mocked(searchArtists)

const searchResponse: SearchResponse = {
  query: "dra",
  sources: {
    musicbrainz: {
      status: "ok",
      artists: [
        {
          source: "musicbrainz",
          id: "mbid-drake",
          name: "Drake",
          disambiguation: null,
          type: "Person",
          image_url: null,
        },
      ],
    },
  },
}

function getSearchInput() {
  return screen.getByLabelText("Search artists")
}

describe("SearchBox", () => {
  it("collapses a keystroke burst into exactly one debounced searchArtists call and forwards the response", async () => {
    mockSearchArtists.mockResolvedValue(searchResponse)
    const onResults = vi.fn()

    render(<SearchBox onResults={onResults} />)
    await userEvent.type(getSearchInput(), "dra")

    await vi.waitFor(() => expect(mockSearchArtists).toHaveBeenCalled())

    // Debounce collapse is asserted by call count, not merely "was called" --
    // a "was called" assertion alone would still pass even if the debounce
    // were removed entirely and every keystroke issued its own call.
    expect(mockSearchArtists).toHaveBeenCalledTimes(1)
    expect(mockSearchArtists.mock.calls[0][0]).toBe("dra")

    await vi.waitFor(() => expect(onResults).toHaveBeenCalledWith(searchResponse))
  })

  // The two tests below prove SearchBox's own doc-comment claim -- "a fresh
  // AbortController is created per debounced search and the prior one is
  // aborted before the new one starts" -- is true at the request level, not
  // only at the discard-the-stale-result callback level. Both fail against
  // current source: runSearch calls searchArtists(query) with one argument,
  // so no signal ever reaches the request (see the folded todo).
  it("passes an AbortSignal as the second argument to searchArtists", async () => {
    mockSearchArtists.mockResolvedValue(searchResponse)
    const onResults = vi.fn()

    render(<SearchBox onResults={onResults} />)
    await userEvent.type(getSearchInput(), "dra")

    await vi.waitFor(() =>
      expect(mockSearchArtists).toHaveBeenCalledWith(
        "dra",
        expect.any(AbortSignal),
      ),
    )
  })

  it("aborts the superseded search's signal when a newer keystroke supersedes it", async () => {
    mockSearchArtists.mockResolvedValue(searchResponse)
    const onResults = vi.fn()

    render(<SearchBox onResults={onResults} />)
    await userEvent.type(getSearchInput(), "dra")
    await vi.waitFor(() => expect(mockSearchArtists).toHaveBeenCalledTimes(1))

    // Cast through unknown[]: searchArtists' current signature is
    // single-argument, so TS doesn't yet know about a second element --
    // Task 3 widens the signature and this cast stays valid either way.
    const firstCall = mockSearchArtists.mock.calls[0] as unknown as unknown[]
    const firstSignal = firstCall[1] as AbortSignal | undefined

    // The next keystroke supersedes the in-flight search -- SearchBox
    // aborts the prior controller on the very next change event.
    await userEvent.type(getSearchInput(), "k")

    await vi.waitFor(() => expect(firstSignal?.aborted).toBe(true))
  })
})
