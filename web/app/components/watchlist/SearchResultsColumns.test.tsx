import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { SearchArtist, SearchResponse, WatchlistEntry } from "~/lib/api"

import { SearchResultsColumns } from "./SearchResultsColumns"

// This component takes its API data as props and reports actions via the
// onAdd callback prop -- it never imports ~/lib/api itself, so no module
// mock is needed here.

const addableArtist: SearchArtist = {
  source: "musicbrainz",
  id: "mbid-addable",
  name: "Addable Artist",
  disambiguation: null,
  country: null,
  type: "Group",
  image_url: null,
}

const watchedArtist: SearchArtist = {
  source: "musicbrainz",
  id: "mbid-watched",
  name: "Watched Artist",
  disambiguation: "US rapper",
  country: null,
  type: "Person",
  image_url: null,
}

const deezerArtist: SearchArtist = {
  source: "deezer",
  id: "deezer-1",
  name: "Deezer Only Artist",
  disambiguation: null,
  country: null,
  type: "Artist",
  image_url: null,
}

const countryOnlyArtist: SearchArtist = {
  source: "musicbrainz",
  id: "mbid-country-only",
  name: "Ambiguous Drake",
  disambiguation: null,
  country: "CA",
  type: "Person",
  image_url: null,
}

const bothHintsArtist: SearchArtist = {
  source: "musicbrainz",
  id: "mbid-both-hints",
  name: "Disambiguated Drake",
  disambiguation: "Canadian rapper",
  country: "CA",
  type: "Person",
  image_url: null,
}

const watchlistEntry: WatchlistEntry = {
  id: 1,
  artist_id: 1,
  mbid: "mbid-watched",
  name: "Watched Artist",
  deezer_id: null,
  disambiguation: null,
  image_url: null,
  release_types: [],
  muted_event_types: [],
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
}

describe("SearchResultsColumns", () => {
  it("shows the per-source unavailable message naming the other source when a source errors", () => {
    const response: SearchResponse = {
      query: "drake",
      sources: {
        musicbrainz: { status: "ok", artists: [addableArtist] },
        deezer: { status: "error", error: "unavailable", artists: [] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={null}
        onAdd={vi.fn()}
      />
    )

    expect(
      screen.getByText(
        "Deezer is unavailable right now — showing MusicBrainz results only."
      )
    ).toBeInTheDocument()
  })

  it("shows a no-matches message when a source returns zero results", () => {
    const response: SearchResponse = {
      query: "nobody",
      sources: {
        musicbrainz: { status: "ok", artists: [] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={null}
        onAdd={vi.fn()}
      />
    )

    expect(screen.getByText('No matches for "nobody"')).toBeInTheDocument()
  })

  it("disables the control with 'Already watching' for a result already on the watchlist", () => {
    const response: SearchResponse = {
      query: "watched",
      sources: {
        musicbrainz: { status: "ok", artists: [watchedArtist] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={[watchlistEntry]}
        onAdd={vi.fn()}
      />
    )

    expect(
      screen.getByRole("button", { name: "Already watching" })
    ).toBeDisabled()
  })

  it("disables the control with a MusicBrainz-only hint for a Deezer result", () => {
    const response: SearchResponse = {
      query: "deezer only",
      sources: {
        deezer: { status: "ok", artists: [deezerArtist] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={null}
        onAdd={vi.fn()}
      />
    )

    expect(
      screen.getByRole("button", { name: "Search MusicBrainz to add" })
    ).toBeDisabled()
  })

  it("falls back to the country code when disambiguation is blank", () => {
    const response: SearchResponse = {
      query: "drake",
      sources: {
        musicbrainz: { status: "ok", artists: [countryOnlyArtist] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={null}
        onAdd={vi.fn()}
      />
    )

    expect(screen.getByText("CA")).toBeInTheDocument()
  })

  it("prefers disambiguation over country when both are present", () => {
    const response: SearchResponse = {
      query: "drake",
      sources: {
        musicbrainz: { status: "ok", artists: [bothHintsArtist] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={null}
        onAdd={vi.fn()}
      />
    )

    expect(screen.getByText("Canadian rapper")).toBeInTheDocument()
    expect(screen.queryByText("CA")).toBeNull()
  })

  it("renders no secondary label when disambiguation and country are both absent", () => {
    const response: SearchResponse = {
      query: "deezer only",
      sources: {
        deezer: { status: "ok", artists: [deezerArtist] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={null}
        onAdd={vi.fn()}
      />
    )

    const row = screen.getByRole("listitem")
    expect(row.textContent).toBe(
      `${deezerArtist.name}${deezerArtist.type}Search MusicBrainz to add`
    )
  })

  it("calls onAdd with the source name and artist when Add to Watchlist is clicked, and stays busy until it resolves", async () => {
    let resolveAdd: () => void = () => {}
    const onAdd = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveAdd = resolve
        })
    )

    const response: SearchResponse = {
      query: "addable",
      sources: {
        musicbrainz: { status: "ok", artists: [addableArtist] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={null}
        onAdd={onAdd}
      />
    )

    const addButton = screen.getByRole("button", { name: "Add to Watchlist" })
    await userEvent.click(addButton)

    expect(onAdd).toHaveBeenCalledWith("musicbrainz", addableArtist)
    expect(addButton).toHaveAttribute("aria-busy", "true")

    resolveAdd()

    await waitFor(() => expect(addButton).toHaveAttribute("aria-busy", "false"))
  })
})
