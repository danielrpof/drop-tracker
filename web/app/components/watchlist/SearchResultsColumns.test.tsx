import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { SearchArtist, SearchResponse, WatchlistEntry } from "~/lib/api"

import { DEEZER_ADD_HINT, SearchResultsColumns } from "./SearchResultsColumns"

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
  tags: [],
  note: null,
  latest_release_date: null,
  next_release_date: null,
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

  it("caps the Deezer hint wrapper, titles it, and truncates the button label (G-25-9, WR-04)", () => {
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

    const button = screen.getByRole("button", {
      name: "Search MusicBrainz to add",
    })
    const wrapper = button.parentElement as HTMLElement
    expect(wrapper.tagName).toBe("SPAN")
    expect(wrapper).toHaveClass("max-w-[40%]", "shrink-0", "lg:max-w-none")
    expect(wrapper).not.toHaveClass("pointer-events-none")
    expect(wrapper.getAttribute("title")).toContain("MusicBrainz identity")
    expect(button).toBeDisabled()
    expect(button).toHaveClass("w-full")
    expect(button).not.toHaveClass("max-w-[40%]")
    expect(button).not.toHaveAttribute("title")
    expect(button.children).toHaveLength(1)
    const label = button.children[0]
    expect(label.tagName).toBe("SPAN")
    expect(label).toHaveClass("truncate")
    expect(label.textContent).toBe("Search MusicBrainz to add")
    expect(button).toHaveAccessibleDescription(DEEZER_ADD_HINT)
  })

  it("describes each Deezer button with its own sr-only hint (G-25-13)", () => {
    const secondDeezer: SearchArtist = {
      ...deezerArtist,
      id: "deezer-2",
      name: "Second Deezer Artist",
    }
    const response: SearchResponse = {
      query: "deezer",
      sources: {
        deezer: { status: "ok", artists: [deezerArtist, secondDeezer] },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={null}
        onAdd={vi.fn()}
      />
    )

    const buttons = screen.getAllByRole("button", {
      name: "Search MusicBrainz to add",
    })
    expect(buttons).toHaveLength(2)
    const ids = buttons.map((b) => b.getAttribute("aria-describedby"))
    expect(ids[0]).toBeTruthy()
    expect(ids[1]).toBeTruthy()
    expect(ids[0]).not.toBe(ids[1])
    for (const [i, button] of buttons.entries()) {
      const hint = document.getElementById(ids[i] as string) as HTMLElement
      expect(hint.textContent).toBe(DEEZER_ADD_HINT)
      expect(hint).toHaveClass("sr-only")
      expect(button.contains(hint)).toBe(false)
      expect(button.closest("li")?.contains(hint)).toBe(true)
    }
  })

  it("truncates the type line in Deezer and MusicBrainz rows (G-25-9)", () => {
    const response: SearchResponse = {
      query: "mixed",
      sources: {
        musicbrainz: { status: "ok", artists: [addableArtist] },
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

    expect(screen.getByText("Artist")).toHaveClass("truncate")
    expect(screen.getByText("Group")).toHaveClass("truncate")
  })

  it("leaves the Add to Watchlist and Already watching buttons uncapped", () => {
    const response: SearchResponse = {
      query: "mixed",
      sources: {
        musicbrainz: {
          status: "ok",
          artists: [addableArtist, watchedArtist],
        },
      },
    }

    render(
      <SearchResultsColumns
        response={response}
        watchlistEntries={[watchlistEntry]}
        onAdd={vi.fn()}
      />
    )

    for (const name of ["Add to Watchlist", "Already watching"]) {
      const button = screen.getByRole("button", { name })
      expect(button).not.toHaveClass("max-w-[40%]")
      expect(button.textContent).toBe(name)
      expect(button.children).toHaveLength(0)
      expect(button).not.toHaveAttribute("aria-describedby")
    }
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
      `${deezerArtist.name}${deezerArtist.type}Search MusicBrainz to add${DEEZER_ADD_HINT}`
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
