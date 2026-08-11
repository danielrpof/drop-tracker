import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { EmptyState } from "~/components/common/EmptyState"
import { Button } from "~/components/ui/button"
import { Skeleton } from "~/components/ui/skeleton"
import { WatchlistRow } from "~/components/watchlist/WatchlistRow"
import { type WatchlistEntry, addWatchlist, listWatchlist, removeWatchlist } from "~/lib/api"

// Watchlist renders the UI-02 management surface: fetches listWatchlist()
// on mount and renders the entries in exactly the order the server
// returned them -- ListWatchlist already orders by artist name then artist
// id, a deterministic total order even for two artists sharing a name, so
// this route never re-sorts client-side. Per D-03/D-04 this tab is a pure
// management list with no cross-reference to History-tab state; every
// release-activity signal lives on the History tab only.
//
// `refresh` re-issues the fetch without resetting `entries` to null first,
// so a background re-sync (task 2's post-mutation refresh, plan 06-04's
// post-add refresh) never flashes the loading skeletons over an already-
// populated list -- only the very first mount renders them.
export default function Watchlist() {
  const [entries, setEntries] = useState<WatchlistEntry[] | null>(null)
  const [error, setError] = useState(false)

  const refresh = useCallback(() => {
    setError(false)
    listWatchlist()
      .then((rows) => setEntries(rows))
      .catch(() => setError(true))
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  // handleEntryChange is passed down to PreferenceToggles (via
  // WatchlistRow) so its optimistic-update-then-rollback logic writes
  // straight into this route's own entries state -- the single source of
  // truth every row renders from.
  function handleEntryChange(updated: WatchlistEntry) {
    setEntries((rows) => (rows ? rows.map((r) => (r.id === updated.id ? updated : r)) : rows))
  }

  // handleRemove implements the UI-SPEC's destructive-confirmation policy:
  // no blocking dialog, the DELETE fires on click and the row disappears
  // immediately. On success, a toast with a 5s-auto-dismissing Undo action
  // re-adds the artist via addWatchlist -- honestly labelled as restoring
  // *default* preferences, not the row's prior custom release-type/mute
  // settings, since the DELETE is a real hard delete and nothing is cached
  // client-side across the toast window. On failure, refresh() re-fetches
  // so the list reflects the row's true server state rather than the
  // optimistic removal.
  async function handleRemove(entry: WatchlistEntry) {
    setEntries((rows) => (rows ? rows.filter((r) => r.id !== entry.id) : rows))

    try {
      await removeWatchlist(entry.id)
    } catch {
      toast.error(`Couldn't remove ${entry.name} — it may already be gone.`)
      refresh()
      return
    }

    toast.success(`Removed ${entry.name} from your watchlist.`, {
      description:
        "Undo re-adds this artist with default preferences -- its previous release-type and mute settings are not restored.",
      duration: 5000,
      action: {
        label: "Undo",
        onClick: () => {
          addWatchlist({
            mbid: entry.mbid,
            name: entry.name,
            deezerId: entry.deezer_id ?? undefined,
            disambiguation: entry.disambiguation ?? undefined,
            imageUrl: entry.image_url ?? undefined,
          })
            .then(refresh)
            .catch(() => {
              toast.error(`Couldn't restore ${entry.name}. Try adding it again.`)
            })
        },
      },
    })
  }

  return (
    <div className="flex flex-col gap-6 p-8">
      <h1 className="text-display font-semibold text-foreground">Watchlist</h1>

      {error && (
        <EmptyState
          heading="Couldn't load your watchlist."
          body="Please try again."
          action={<Button onClick={refresh}>Retry</Button>}
        />
      )}

      {!error && entries === null && (
        <div className="flex flex-col gap-4">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-24 w-full rounded-md" />
          ))}
        </div>
      )}

      {!error && entries !== null && entries.length === 0 && (
        <EmptyState heading="No artists yet" body="Search above to add one." />
      )}

      {!error && entries !== null && entries.length > 0 && (
        <ul className="flex flex-col gap-4">
          {entries.map((entry) => (
            <WatchlistRow
              key={entry.id}
              entry={entry}
              onEntryChange={handleEntryChange}
              onRemove={handleRemove}
            />
          ))}
        </ul>
      )}
    </div>
  )
}
