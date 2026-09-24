import { useCallback, useEffect, useState } from "react"
import { Tags } from "lucide-react"
import { toast } from "sonner"

import { EmptyState } from "~/components/common/EmptyState"
import { Button } from "~/components/ui/button"
import { Skeleton } from "~/components/ui/skeleton"
import { ManageTagsDialog } from "~/components/watchlist/ManageTagsDialog"
import { SearchBox } from "~/components/watchlist/SearchBox"
import { SearchResultsColumns } from "~/components/watchlist/SearchResultsColumns"
import { WatchlistRow } from "~/components/watchlist/WatchlistRow"
import {
  ApiError,
  type SearchArtist,
  type SearchResponse,
  type TagRef,
  type TagSummary,
  type WatchlistEntry,
  addWatchlist,
  listTags,
  listWatchlist,
  removeWatchlist,
} from "~/lib/api"
import { isAddableSource } from "~/lib/sources"

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
  const [searchResponse, setSearchResponse] = useState<SearchResponse | null>(
    null
  )
  const [statusMessage, setStatusMessage] = useState("")
  // vocabulary loads lazily on first "+ tag" (or Manage tags) open, never
  // at mount (D-30, Phase 25 SC5) -- the Watchlist route stays one request.
  const [vocabulary, setVocabulary] = useState<TagRef[] | null>(null)
  const [vocabularyStatus, setVocabularyStatus] = useState<
    "idle" | "loading" | "loaded" | "error"
  >("idle")
  const [manageTagsOpen, setManageTagsOpen] = useState(false)

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
  // truth every row renders from. It merges a partial patch onto whatever
  // the row currently is (the functional setEntries updater reads the
  // latest state, never a stale closure), not a full-row replacement --
  // see PreferenceToggles' onEntryChange doc comment for why: two
  // concurrent single-axis PATCHes must never let one axis's response
  // clobber the other axis's already-applied value.
  function handleEntryChange(id: number, patch: Partial<WatchlistEntry>) {
    setEntries((rows) =>
      rows ? rows.map((r) => (r.id === id ? { ...r, ...patch } : r)) : rows
    )
  }

  // addTag/removeTag are the D-24 functional per-item updaters TagChips
  // drives directly (not a whole-array snapshot): each only ever touches
  // its own entry's tags array, so concurrent chip add/remove on one row
  // can never clobber each other. addTag is a no-op when the tag is
  // already present (idempotent attach, D-21) and inserts at a clamped
  // index so a failed-remove rollback restores the chip's original
  // position.
  function addTag(entryId: number, tag: TagRef, index?: number) {
    setEntries((rows) =>
      rows
        ? rows.map((r) => {
            if (r.id !== entryId || r.tags.some((t) => t.id === tag.id)) {
              return r
            }
            const tags = [...r.tags]
            const at =
              index === undefined
                ? tags.length
                : Math.max(0, Math.min(index, tags.length))
            tags.splice(at, 0, tag)
            return { ...r, tags }
          })
        : rows
    )
  }

  function removeTag(entryId: number, tagId: number) {
    setEntries((rows) =>
      rows
        ? rows.map((r) =>
            r.id === entryId
              ? { ...r, tags: r.tags.filter((t) => t.id !== tagId) }
              : r
          )
        : rows
    )
  }

  // dropTagFromEntries removes one tag id from every entry's tags array
  // (Manage tags delete, TAG-06, D-17) -- a functional updater in the same
  // shape as addTag/removeTag, so it can never clobber a concurrent chip
  // add/remove on an unrelated row.
  function dropTagFromEntries(tagId: number) {
    setEntries((rows) =>
      rows
        ? rows.map((r) => ({
            ...r,
            tags: r.tags.filter((t) => t.id !== tagId),
          }))
        : rows
    )
  }

  // renameTagInEntries applies a completed Manage tags rename to every
  // card's chip with that id (TAG-05, D-17) -- same functional-updater
  // shape as dropTagFromEntries, so it never clobbers a concurrent chip
  // add/remove on an unrelated row.
  function renameTagInEntries(tag: TagRef) {
    setEntries((rows) =>
      rows
        ? rows.map((r) => ({
            ...r,
            tags: r.tags.map((t) => (t.id === tag.id ? tag : t)),
          }))
        : rows
    )
    setVocabulary((v) => (v ? v.map((t) => (t.id === tag.id ? tag : t)) : v))
  }

  // mergeTagInEntries applies a confirmed Manage tags merge (TAG-05, D-17,
  // D-19) to every card: an entry that already carries the target just
  // drops the source (no duplicate chip), otherwise the source chip is
  // replaced in place with the target -- same functional-updater shape as
  // dropTagFromEntries/renameTagInEntries.
  function mergeTagInEntries(sourceId: number, target: TagRef) {
    setEntries((rows) =>
      rows
        ? rows.map((r) => {
            if (!r.tags.some((t) => t.id === sourceId)) return r
            if (r.tags.some((t) => t.id === target.id)) {
              return { ...r, tags: r.tags.filter((t) => t.id !== sourceId) }
            }
            return {
              ...r,
              tags: r.tags.map((t) => (t.id === sourceId ? target : t)),
            }
          })
        : rows
    )
    setVocabulary((v) =>
      v
        ? v.some((t) => t.id === target.id)
          ? v.filter((t) => t.id !== sourceId)
          : v.map((t) => (t.id === sourceId ? target : t))
        : v
    )
  }

  // handleTagsLoaded is ManageTagsDialog's onLoaded (D-30): it replaces the
  // whole route vocabulary with what the dialog just fetched, so the "+
  // tag" autocomplete on every row sees the same fresh vocabulary Manage
  // tags just loaded, instead of leaving a possibly-stale one in place.
  function handleTagsLoaded(tags: TagSummary[]) {
    setVocabulary(tags.map((t) => ({ id: t.id, name: t.name })))
    setVocabularyStatus("loaded")
  }

  // loadVocabulary fetches GET /tags only from "idle"/"error" (a failed
  // load retries on the next open), so repeated "+ tag" opens across rows
  // never refetch once it has loaded (D-30, T-24-30).
  function loadVocabulary() {
    if (vocabularyStatus !== "idle" && vocabularyStatus !== "error") return
    setVocabularyStatus("loading")
    listTags()
      .then((summaries) => {
        setVocabulary(summaries.map((s) => ({ id: s.id, name: s.name })))
        setVocabularyStatus("loaded")
      })
      .catch(() => setVocabularyStatus("error"))
  }

  // rememberTag inserts a newly created tag into an already-loaded
  // vocabulary, so other rows and Manage tags see it without a reload.
  function rememberTag(tag: TagRef) {
    setVocabulary((v) =>
      v === null || v.some((t) => t.id === tag.id) ? v : [...v, tag]
    )
  }

  // announce feeds the single route-level status region below (UI-SPEC
  // [R6]: one contextual, atomic message, never one live region per chip).
  function announce(message: string) {
    setStatusMessage(message)
  }

  // handleAddSearchResult wires SearchResultsColumns' "Add to Watchlist"
  // click into addWatchlist, sending neither preference axis so the API
  // applies its own D-08 defaults (release-type/mute preferences are edited
  // afterward via each row's inline PreferenceToggles, not at add time).
  // On success it refreshes the loaded entries so the new artist appears
  // below and the "Already watching" cross-reference (D-11) picks it up in
  // the same pass. 06-RESEARCH.md Pitfall 3: a fast click can beat this
  // route's own initial GET /watchlist fetch, so the cross-reference has
  // not yet caught this result as already-added -- the resulting 409 in
  // that narrow window means this artist is genuinely being tracked, so it
  // is treated as success (refresh) rather than an error toast. Any other
  // failure shows the generic add-failure toast; SearchResultsColumns'
  // per-row pending state returns the result to an addable state on its
  // own once this promise settles.
  //
  // Only a MusicBrainz result carries a real mbid -- a Deezer result's `id`
  // is a Deezer catalog id with no relation to MusicBrainz, and POST
  // /watchlist treats mbid as the artist's canonical identity with no
  // format validation (internal/httpserver/watchlist.go), so passing it
  // through would silently and permanently break that artist's
  // MusicBrainz-sourced release detection. SearchResultsColumns already
  // disables the "Add to Watchlist" action for Deezer results (this
  // project has no cross-source identity resolution), so sourceName here
  // should always be "musicbrainz"; the guard below is defense-in-depth,
  // not the primary safeguard.
  async function handleAddSearchResult(
    sourceName: string,
    result: SearchArtist
  ) {
    if (!isAddableSource(sourceName)) {
      toast.error("Can't add this artist yet -- search MusicBrainz instead.")
      return
    }
    try {
      await addWatchlist({
        mbid: result.id,
        name: result.name,
        imageUrl: result.image_url ?? undefined,
        disambiguation: result.disambiguation ?? undefined,
        deezerId: undefined,
      })
      refresh()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        refresh()
        return
      }
      toast.error("Couldn't add this artist. Try again in a moment.")
    }
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
              toast.error(
                `Couldn't restore ${entry.name}. Try adding it again.`
              )
            })
        },
      },
    })
  }

  return (
    <div className="flex flex-col gap-6 p-8">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-display font-semibold text-foreground">
          Watchlist
        </h1>
        <Button variant="secondary" onClick={() => setManageTagsOpen(true)}>
          <Tags aria-hidden="true" />
          Manage tags
        </Button>
      </div>

      <div role="status" aria-atomic="true" className="sr-only">
        {statusMessage}
      </div>

      <ManageTagsDialog
        open={manageTagsOpen}
        onOpenChange={setManageTagsOpen}
        onLoaded={handleTagsLoaded}
        onDeleted={dropTagFromEntries}
        onRenamed={renameTagInEntries}
        onMerged={mergeTagInEntries}
      />

      <div className="flex flex-col gap-6">
        <SearchBox onResults={setSearchResponse} />
        {searchResponse && (
          <SearchResultsColumns
            response={searchResponse}
            watchlistEntries={entries}
            onAdd={handleAddSearchResult}
          />
        )}
      </div>

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
              tagActions={{
                addTag,
                removeTag,
                vocabulary,
                loadVocabulary,
                rememberTag,
              }}
              announce={announce}
            />
          ))}
        </ul>
      )}
    </div>
  )
}
