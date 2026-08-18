import { useState } from "react"
import { toast } from "sonner"

import { Checkbox } from "~/components/ui/checkbox"
import { type WatchlistEntry, updateWatchlistPreferences } from "~/lib/api"

// PreferenceToggles renders the two D-12 preference axes -- release-type
// filters and muted event types -- as inline checkboxes directly in the
// row, no modal, no separate save step. Each checkbox fires its PATCH
// immediately and sends exactly one axis in the request body (T-06-14):
// the API treats an absent key as "leave this axis untouched" and an
// explicit empty array as "watch/mute nothing on this axis," two distinct
// states, so a toggle handler must never resolve/send the other axis.
export interface PreferenceTogglesProps {
  entry: WatchlistEntry
  onEntryChange: (id: number, patch: Partial<WatchlistEntry>) => void
}

// onEntryChange takes a partial patch keyed by axis, not a full-row
// replacement: toggling release-type and mute checkboxes in quick
// succession fires two independent, concurrently in-flight PATCHes, and
// the server correctly serializes the writes (row lock + CASE-based
// partial update in internal/watchlist), but each response's HTTP body is
// only a snapshot of the row as of that PATCH's own commit -- it does not
// know about the other axis's just-applied change. Merging a full-row
// response into state would let the slower response's stale snapshot
// clobber the other axis's already-applied value if it resolves second. A
// partial patch merged by the parent's functional state update (against
// the current row, not a stale closure) only ever touches its own axis.
export function PreferenceToggles({
  entry,
  onEntryChange,
}: PreferenceTogglesProps) {
  const [releasePending, setReleasePending] = useState(false)
  const [mutePending, setMutePending] = useState(false)

  // toggleReleaseType and toggleMutedEventType apply the optimistic change
  // through onEntryChange immediately, then fire the single-axis PATCH.
  // The pre-toggle array is kept in `previous` so a failed write can
  // restore the row to the server's true prior value -- a rolled-back
  // toggle must never stay visually "on" after a failed write (T-06-13,
  // the "MUST NOT display an optimistically-updated preference toggle as
  // saved when its PATCH failed" prohibition).
  async function toggleReleaseType(type: string, next: boolean) {
    const previous = entry.release_types
    const optimistic = next
      ? [...previous, type]
      : previous.filter((t) => t !== type)

    onEntryChange(entry.id, { release_types: optimistic })
    setReleasePending(true)
    try {
      const updated = await updateWatchlistPreferences(entry.id, {
        releaseTypes: optimistic,
      })
      onEntryChange(entry.id, { release_types: updated.release_types })
    } catch {
      onEntryChange(entry.id, { release_types: previous })
      toast.error("Couldn't update preferences — try again.")
    } finally {
      setReleasePending(false)
    }
  }

  async function toggleMutedEventType(type: string, next: boolean) {
    const previous = entry.muted_event_types
    const optimistic = next
      ? [...previous, type]
      : previous.filter((t) => t !== type)

    onEntryChange(entry.id, { muted_event_types: optimistic })
    setMutePending(true)
    try {
      const updated = await updateWatchlistPreferences(entry.id, {
        mutedEventTypes: optimistic,
      })
      onEntryChange(entry.id, { muted_event_types: updated.muted_event_types })
    } catch {
      onEntryChange(entry.id, { muted_event_types: previous })
      toast.error("Couldn't update preferences — try again.")
    } finally {
      setMutePending(false)
    }
  }

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:gap-6">
      <fieldset className="flex flex-col gap-1.5" disabled={releasePending}>
        <legend className="text-label text-muted-foreground">
          Release types
        </legend>
        <div className="flex flex-wrap gap-3">
          <label className="flex min-h-11 items-center gap-1.5 py-1 text-label text-foreground">
            <Checkbox
              checked={entry.release_types.includes("album")}
              onCheckedChange={(checked) => toggleReleaseType("album", checked)}
              aria-label="Album release type"
            />
            Album
          </label>
          <label className="flex min-h-11 items-center gap-1.5 py-1 text-label text-foreground">
            <Checkbox
              checked={entry.release_types.includes("single")}
              onCheckedChange={(checked) =>
                toggleReleaseType("single", checked)
              }
              aria-label="Single release type"
            />
            Single
          </label>
          <label className="flex min-h-11 items-center gap-1.5 py-1 text-label text-foreground">
            <Checkbox
              checked={entry.release_types.includes("ep")}
              onCheckedChange={(checked) => toggleReleaseType("ep", checked)}
              aria-label="EP release type"
            />
            EP
          </label>
          <label className="flex min-h-11 items-center gap-1.5 py-1 text-label text-foreground">
            <Checkbox
              checked={entry.release_types.includes("deluxe")}
              onCheckedChange={(checked) =>
                toggleReleaseType("deluxe", checked)
              }
              aria-label="Deluxe release type"
            />
            Deluxe
          </label>
        </div>
      </fieldset>

      <fieldset className="flex flex-col gap-1.5" disabled={mutePending}>
        <legend className="text-label text-muted-foreground">
          Mute alerts
        </legend>
        <div className="flex flex-wrap gap-3">
          <label className="flex min-h-11 items-center gap-1.5 py-1 text-label text-foreground">
            <Checkbox
              checked={entry.muted_event_types.includes("new_release")}
              onCheckedChange={(checked) =>
                toggleMutedEventType("new_release", checked)
              }
              aria-label="Mute new release alerts"
            />
            New release
          </label>
          <label className="flex min-h-11 items-center gap-1.5 py-1 text-label text-foreground">
            <Checkbox
              checked={entry.muted_event_types.includes("guest_feature")}
              onCheckedChange={(checked) =>
                toggleMutedEventType("guest_feature", checked)
              }
              aria-label="Mute guest feature alerts"
            />
            Guest feature
          </label>
          <label className="flex min-h-11 items-center gap-1.5 py-1 text-label text-foreground">
            <Checkbox
              checked={entry.muted_event_types.includes("deluxe_change")}
              onCheckedChange={(checked) =>
                toggleMutedEventType("deluxe_change", checked)
              }
              aria-label="Mute deluxe change alerts"
            />
            Deluxe change
          </label>
        </div>
      </fieldset>
    </div>
  )
}
