import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { EventCard } from "./EventCard"
import type { EventItem } from "~/lib/api"

// EventCard imports only the EventItem type from ~/lib/api, never a
// function -- there is no API boundary here, so this file needs no module
// mock at all (D-06 only applies where a function is imported).

// File-local fixture builder (D-06's narrow-seam philosophy: no shared
// fixtures module). Every nullable field is explicitly present so each test
// only needs to override what it cares about.
function buildEvent(overrides: Partial<EventItem> = {}): EventItem {
  return {
    id: 1,
    artist_id: 10,
    source: "musicbrainz",
    event_type: "new_release",
    external_id: "ext-1",
    release_group_mbid: null,
    title: "Test Title",
    artist_name: "Test Artist",
    release_date: "2026-01-01",
    cover_art_url: null,
    track_count: null,
    previous_track_count: null,
    release_type: "album",
    notified_at: null,
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  }
}

describe("EventCard", () => {
  it("renders the title and 'New release' badge for a new_release event", () => {
    const event = buildEvent({ event_type: "new_release", title: "Fresh Drop" })

    render(<EventCard event={event} />)

    expect(screen.getByText("Fresh Drop")).toBeInTheDocument()
    expect(screen.getByText("New release")).toBeInTheDocument()
  })

  it("renders the 'Guest feature' badge for a guest_feature event", () => {
    const event = buildEvent({
      event_type: "guest_feature",
      title: "Feature Track",
      source: "deezer",
    })

    render(<EventCard event={event} />)

    expect(screen.getByText("Guest feature")).toBeInTheDocument()
  })

  it("renders the 'Deluxe change' badge for a deluxe_change event", () => {
    const event = buildEvent({
      event_type: "deluxe_change",
      title: "Deluxe Edition",
      track_count: 20,
      previous_track_count: 16,
    })

    render(<EventCard event={event} />)

    expect(screen.getByText("Deluxe change")).toBeInTheDocument()
  })

  it("falls back to an 'Unknown' badge without throwing for an event_type outside the known union", () => {
    // Pitfall 6: the type system correctly can't represent this literal
    // normally -- the deliberate cast simulates GET /events returning an
    // event_type value the client never validated.
    const event = buildEvent({
      event_type: "surprise_type" as EventItem["event_type"],
      title: "Mystery Row",
    })

    expect(() => render(<EventCard event={event} />)).not.toThrow()

    expect(screen.getByText("Unknown")).toBeInTheDocument()
    expect(screen.getByText("Mystery Row")).toBeInTheDocument()
  })
})
