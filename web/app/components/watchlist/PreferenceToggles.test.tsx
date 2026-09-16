import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { type WatchlistEntry, updateWatchlistPreferences } from "~/lib/api"

import { PreferenceToggles } from "./PreferenceToggles"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")

const mockUpdateWatchlistPreferences = vi.mocked(updateWatchlistPreferences)

// release_types starts as exactly ["album"] and muted_event_types is
// empty, so toggling Single on produces an unambiguous optimistic array of
// ["album", "single"] and an unambiguous rollback target of ["album"].
const entry: WatchlistEntry = {
  id: 1,
  artist_id: 10,
  mbid: "mbid-drake",
  name: "Drake",
  deezer_id: null,
  disambiguation: null,
  image_url: null,
  release_types: ["album"],
  muted_event_types: [],
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
}

describe("PreferenceToggles", () => {
  it("calls updateWatchlistPreferences with the optimistic array when a release type is toggled on", async () => {
    mockUpdateWatchlistPreferences.mockResolvedValue({
      ...entry,
      release_types: ["album", "single"],
    })
    const onEntryChange = vi.fn()

    render(<PreferenceToggles entry={entry} onEntryChange={onEntryChange} />)

    await userEvent.click(
      screen.getByRole("checkbox", { name: /^Single release type/ })
    )

    expect(mockUpdateWatchlistPreferences).toHaveBeenCalledWith(1, {
      releaseTypes: ["album", "single"],
    })
  })

  it("rolls back the optimistic release-type toggle when the PATCH call fails", async () => {
    mockUpdateWatchlistPreferences.mockRejectedValue(new Error("network error"))
    const onEntryChange = vi.fn()

    render(<PreferenceToggles entry={entry} onEntryChange={onEntryChange} />)

    await userEvent.click(
      screen.getByRole("checkbox", { name: /^Single release type/ })
    )

    // optimistic update fired immediately
    expect(onEntryChange).toHaveBeenCalledWith(1, {
      release_types: ["album", "single"],
    })

    // rollback after the rejected PATCH resolves
    await waitFor(() =>
      expect(onEntryChange).toHaveBeenLastCalledWith(1, {
        release_types: ["album"],
      })
    )
  })
})
