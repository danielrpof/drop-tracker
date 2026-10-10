import { useCallback, useEffect, useRef, useState } from "react"
import { Tags } from "lucide-react"
import { toast } from "sonner"

import { EmptyState } from "~/components/common/EmptyState"
import { Button } from "~/components/ui/button"
import { Skeleton } from "~/components/ui/skeleton"
import { ManageTagsDialog } from "~/components/watchlist/ManageTagsDialog"
import { SearchBox } from "~/components/watchlist/SearchBox"
import { SearchResultsColumns } from "~/components/watchlist/SearchResultsColumns"
import { WatchlistRow } from "~/components/watchlist/WatchlistRow"
import { WatchlistToolbar } from "~/components/watchlist/WatchlistToolbar"
import {
  ApiError,
  type SearchArtist,
  type SearchResponse,
  type TagRef,
  type WatchlistEntry,
  addWatchlist,
  listWatchlist,
  removeWatchlist,
} from "~/lib/api"
import { isAddableSource } from "~/lib/sources"
import { useUrlParams } from "~/lib/useUrlParams"
import {
  CLEAR_FILTER_KEYS,
  DEFAULT_SORT,
  formatArtistCount,
  isFiltered,
  parseWatchlistParams,
  selectVisible,
  type SortKey,
} from "~/lib/watchlistView"
import {
  rewriteTags,
  TagVocabularyProvider,
  type TagChange,
} from "~/lib/useTagVocabulary"

// Watchlist renders the UI-02 management surface: loads GET /watchlist once
// on mount. Phase 25 D-02/D-03 add client-side find/sort over that list and
// a latest-release line on each card.
//
// `refresh` re-issues the fetch without resetting `entries` to null first,
// so a background re-sync (task 2's post-mutation refresh, plan 06-04's
// post-add refresh) never flashes the loading skeletons over an already-
// populated list -- only the very first mount renders them.
const Q_WRITE_DEBOUNCE_MS = 300
const COUNT_ANNOUNCE_DEBOUNCE_MS = 500

export default function Watchlist() {
  const [entries, setEntries] = useState<WatchlistEntry[] | null>(null)
  const [error, setError] = useState(false)
  const [searchResponse, setSearchResponse] = useState<SearchResponse | null>(
    null
  )
  const [statusMessage, setStatusMessage] = useState("")
  const [manageTagsOpen, setManageTagsOpen] = useState(false)

  const { params, write } = useUrlParams()
  const view = parseWatchlistParams(params)
  const [query, setQuery] = useState(view.q)
  const lastWrittenQ = useRef(view.q)
  const qTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const filterInputRef = useRef<HTMLInputElement | null>(null)
  const countTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const countRef = useRef({ n: 0, m: 0 })
  // Cards the user just touched or added stay visible past a filter (G4).
  const [stickyIds, setStickyIds] = useState<ReadonlySet<number>>(
    () => new Set()
  )

  function markSticky(id: number) {
    setStickyIds((prev) => (prev.has(id) ? prev : new Set(prev).add(id)))
  }

  function clearSticky() {
    setStickyIds((prev) => (prev.size === 0 ? prev : new Set()))
  }

  function scheduleCountAnnouncement() {
    if (countTimer.current) clearTimeout(countTimer.current)
    countTimer.current = setTimeout(() => {
      const { n, m } = countRef.current
      announce(
        n > 0
          ? `Showing ${formatArtistCount(n, m)}.`
          : "No artists match these filters."
      )
    }, COUNT_ANNOUNCE_DEBOUNCE_MS)
  }

  // Filtering uses the local value; the URL only follows, debounced (D-12).
  function handleQueryChange(next: string) {
    setQuery(next)
    clearSticky()
    scheduleCountAnnouncement()
    if (qTimer.current) clearTimeout(qTimer.current)
    qTimer.current = setTimeout(() => {
      lastWrittenQ.current = next
      write(
        (p) => {
          if (next === "") p.delete("q")
          else p.set("q", next)
        },
        { replace: true }
      )
    }, Q_WRITE_DEBOUNCE_MS)
  }

  // Re-sync the input only for Back/forward or external URL changes, never
  // for an echo of our own write.
  useEffect(() => {
    if (view.q !== lastWrittenQ.current) {
      if (qTimer.current) clearTimeout(qTimer.current)
      lastWrittenQ.current = view.q
      setQuery(view.q)
    }
  }, [view.q])

  useEffect(() => {
    return () => {
      if (qTimer.current) clearTimeout(qTimer.current)
      if (countTimer.current) clearTimeout(countTimer.current)
    }
  }, [])

  // Resolves with the loaded rows (null on failure); never touches stickyIds.
  const refresh = useCallback(() => {
    setError(false)
    return listWatchlist()
      .then((rows) => {
        setEntries(rows)
        return rows
      })
      .catch(() => {
        setError(true)
        return null
      })
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
    markSticky(id)
    setEntries((rows) =>
      rows ? rows.map((r) => (r.id === id ? { ...r, ...patch } : r)) : rows
    )
  }

  // Functional per-entry updaters so concurrent chip changes on one row
  // don't clobber each other (D-24). addTag is idempotent and restores the
  // chip's position on rollback.
  function addTag(entryId: number, tag: TagRef, index?: number) {
    markSticky(entryId)
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
    markSticky(entryId)
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

  // The only place rows' tags change for a Manage tags rename/merge/delete.
  function applyTagChange(change: TagChange) {
    setEntries((rows) =>
      rows
        ? rows.map((r) => {
            const tags = rewriteTags(r.tags, change)
            return tags === r.tags ? r : { ...r, tags }
          })
        : rows
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
      const added = await addWatchlist({
        mbid: result.id,
        name: result.name,
        imageUrl: result.image_url ?? undefined,
        disambiguation: result.disambiguation ?? undefined,
        deezerId: undefined,
      })
      markSticky(added.id)
      refresh()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        const rows = await refresh()
        const existing = rows?.find((r) => r.mbid === result.id)
        if (existing) markSticky(existing.id)
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
            note: entry.note ?? undefined,
          })
            .then((restored) => {
              markSticky(restored.id)
              return refresh()
            })
            .catch(() => {
              toast.error(
                `Couldn't restore ${entry.name}. Try adding it again.`
              )
            })
        },
      },
    })
  }

  const visible = entries
    ? selectVisible(entries, { ...view, q: query }, stickyIds)
    : []
  countRef.current = { n: visible.length, m: entries?.length ?? 0 }

  function handleSortChange(next: SortKey) {
    write((p) => {
      if (next === DEFAULT_SORT) p.delete("sort")
      else p.set("sort", next)
    })
    clearSticky()
  }

  function clearFilters() {
    if (qTimer.current) clearTimeout(qTimer.current)
    if (countTimer.current) clearTimeout(countTimer.current)
    setQuery("")
    lastWrittenQ.current = ""
    write((p) => {
      for (const key of CLEAR_FILTER_KEYS) p.delete(key)
    })
    clearSticky()
    filterInputRef.current?.focus()
    const m = entries?.length ?? 0
    announce(`Filters cleared. Showing ${formatArtistCount(m, m)}.`)
  }

  return (
    <TagVocabularyProvider onChange={applyTagChange}>
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
          <EmptyState
            heading="No artists yet"
            body="Search above to add one."
          />
        )}

        {!error && entries !== null && entries.length > 0 && (
          <section
            aria-labelledby="watchlist-artists-heading"
            className="flex flex-col gap-4"
          >
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <h2
                id="watchlist-artists-heading"
                className="text-heading font-semibold text-foreground outline-none"
              >
                Your artists
              </h2>
              <p className="text-label text-muted-foreground tabular-nums">
                {formatArtistCount(visible.length, entries.length)}
              </p>
            </div>
            <WatchlistToolbar
              query={query}
              onQueryChange={handleQueryChange}
              filterInputRef={filterInputRef}
              showClear={
                isFiltered({ ...view, q: query }) && visible.length > 0
              }
              onClearFilters={clearFilters}
              sort={view.sort}
              onSortChange={handleSortChange}
            />
            {visible.length === 0 ? (
              <EmptyState
                heading="No artists match these filters"
                body={`Try a different name or fewer filters, or clear them to see all ${entries.length} artist${entries.length === 1 ? "" : "s"}.`}
                action={<Button onClick={clearFilters}>Clear filters</Button>}
              />
            ) : (
              <ul className="flex flex-col gap-4">
                {visible.map((entry) => (
                  <WatchlistRow
                    key={entry.id}
                    entry={entry}
                    onEntryChange={handleEntryChange}
                    onRemove={handleRemove}
                    tagActions={{ addTag, removeTag }}
                    announce={announce}
                  />
                ))}
              </ul>
            )}
          </section>
        )}
      </div>
    </TagVocabularyProvider>
  )
}
