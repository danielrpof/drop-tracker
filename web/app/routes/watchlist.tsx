import { useCallback, useEffect, useRef, useState } from "react"
import { Tags } from "lucide-react"
import { NavigationType, useLocation, useNavigationType } from "react-router"
import { toast } from "sonner"

import { ConfirmDialog } from "~/components/common/ConfirmDialog"
import { EmptyState } from "~/components/common/EmptyState"
import { Button } from "~/components/ui/button"
import { Checkbox } from "~/components/ui/checkbox"
import { Skeleton } from "~/components/ui/skeleton"
import { BulkActionBar } from "~/components/watchlist/BulkActionBar"
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
  type BulkRemoveOutcome,
  type WatchlistEntry,
  addWatchlist,
  bulkRemoveWatchlist,
  listWatchlist,
  removeWatchlist,
} from "~/lib/api"
import {
  applyRange,
  effectiveSelection,
  selectAllState,
  toggleAllVisible,
} from "~/lib/bulkSelection"
import { formatAlreadyRemoved, formatNameList } from "~/lib/format"
import { isAddableSource } from "~/lib/sources"
import { useScrollAnchor } from "~/lib/useScrollAnchor"
import { useUrlParams } from "~/lib/useUrlParams"
import {
  CLEAR_FILTER_KEYS,
  EMPTY_IDS,
  SORT_OPTIONS,
  canonicalWatchlistParams,
  carrySticky,
  DEFAULT_SORT,
  formatArtistCount,
  isFiltered,
  parseWatchlistParams,
  reconcileSticky,
  selectVisible,
  tagFilterOptions,
  type SortKey,
  type StickyState,
  type WatchlistView,
} from "~/lib/watchlistView"
import { sortTagsByName } from "~/lib/tags"
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

const BULK_REMOVE_FAILED =
  "Couldn't remove the selected artists — nothing was removed. Try again."

type RemoveOutcome =
  | { ok: true; out: BulkRemoveOutcome }
  | { ok: false; error: unknown }

function plural(n: number): string {
  return n === 1 ? "" : "s"
}

function filterKeyOf(view: WatchlistView): string {
  return `${view.sort}|${view.tagIds.join(",")}|${view.muted}|${view.custom}`
}

export default function Watchlist() {
  const [entries, setEntries] = useState<WatchlistEntry[] | null>(null)
  const [error, setError] = useState(false)
  const [searchResponse, setSearchResponse] = useState<SearchResponse | null>(
    null
  )
  const [statusMessage, setStatusMessage] = useState("")
  const [manageTagsOpen, setManageTagsOpen] = useState(false)
  // Never trimmed to the visible set; the effective selection is derived (D-11).
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<number>>(EMPTY_IDS)
  const [bulkInFlight, setBulkInFlight] = useState(false)
  // ids and names are snapshotted at open and kept while the dialog closes.
  const [removeConfirm, setRemoveConfirm] = useState<{
    open: boolean
    ids: number[]
    names: string[]
  }>({ open: false, ids: [], names: [] })
  const removeOutcomeRef = useRef<RemoveOutcome | null>(null)
  const anchorRef = useRef<number | null>(null)
  const pageHeadingRef = useRef<HTMLHeadingElement>(null)
  const selectAllRef = useRef<HTMLElement | null>(null)
  const clearFiltersRef = useRef<HTMLButtonElement | null>(null)
  const removeButtonRef = useRef<HTMLButtonElement | null>(null)
  const removeCloseKindRef = useRef<"success" | "return">("return")
  const [pendingFocus, setPendingFocus] = useState(false)

  const { params, write } = useUrlParams()
  const location = useLocation()
  const navigationType = useNavigationType()
  const view = parseWatchlistParams(params)
  const filterKey = filterKeyOf(view)
  const [query, setQuery] = useState(view.q)
  const lastWrittenQ = useRef(view.q)
  const qTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const filterInputRef = useRef<HTMLInputElement | null>(null)
  const countTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const countRef = useRef({ n: 0, m: 0, k: 0 })
  // Cards the user just touched or added stay visible past a filter (G4). Keyed
  // to the filter: the router commits the URL in a transition, so the reset must
  // happen in that same render.
  const [sticky, setSticky] = useState<StickyState>(() => ({
    key: filterKey,
    loc: location.key,
    ids: EMPTY_IDS,
    carryTo: null,
  }))
  const current = reconcileSticky(sticky, {
    key: filterKey,
    loc: location.key,
    replace: navigationType === NavigationType.Replace,
  })
  if (current !== sticky) setSticky(current)
  const stickyIds = current.ids

  // Last known name per tag id, so a chip survives its last carrier leaving.
  const tagNamesRef = useRef(new Map<number, string>())
  // Deleted -> null, merged -> target id; maps a Back-resurrected id (D-13).
  const retiredTagIdsRef = useRef(new Map<number, number | null>())
  const tagIdsValidatedRef = useRef(false)

  function markSticky(id: number) {
    setSticky((prev) =>
      prev.ids.has(id) ? prev : { ...prev, ids: new Set(prev.ids).add(id) }
    )
  }

  function clearSticky() {
    setSticky((prev) =>
      prev.ids.size === 0 ? prev : { ...prev, ids: EMPTY_IDS }
    )
  }

  function settleTagIds(ids: number[]): number[] {
    const out = new Set<number>()
    for (const id of ids) {
      const retired = retiredTagIdsRef.current
      const next = retired.has(id) ? retired.get(id) : id
      if (next != null && tagNamesRef.current.has(next)) out.add(next)
    }
    return [...out].sort((a, b) => a - b)
  }

  // The route's own tag rewrites keep sticky cards across their landing (WR-03).
  function writeKeepingSticky(
    mutate: (next: URLSearchParams) => void,
    opts: { replace?: boolean }
  ) {
    write((p) => {
      const from = filterKeyOf(parseWatchlistParams(p))
      mutate(p)
      const to = filterKeyOf(parseWatchlistParams(p))
      setSticky((prev) => carrySticky(prev, from, to))
    }, opts)
  }

  function writeTagIds(ids: number[]) {
    writeKeepingSticky(
      (p) => {
        if (ids.length === 0) p.delete("tags")
        else p.set("tags", ids.join(","))
      },
      { replace: true }
    )
  }

  function scheduleCountAnnouncement() {
    if (countTimer.current) clearTimeout(countTimer.current)
    countTimer.current = setTimeout(() => {
      const { n, m, k } = countRef.current
      announce(
        n > 0
          ? `Showing ${formatArtistCount(n, m)}.${k > 0 ? ` ${k} selected.` : ""}`
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
      clearSticky()
    }
  }, [view.q])

  // A range never survives a change of what the user sees or an empty selection.
  useEffect(() => {
    anchorRef.current = null
  }, [filterKey, query])

  useEffect(() => {
    if (selectedIds.size === 0) anchorRef.current = null
  }, [selectedIds])

  const headingRef = useRef<HTMLHeadingElement>(null)
  // G-25-12: after a keyboard toggle removes its card, focus the fallback card's chip, else the heading.
  function focusAfterCardLeaves(fallback: HTMLElement | null) {
    const target =
      fallback?.querySelector<HTMLElement>(
        '[data-chip-label][aria-pressed="true"]'
      ) ?? fallback?.querySelector<HTMLElement>("[data-chip-label]")
    ;(target ?? headingRef.current)?.focus({ preventScroll: true })
  }

  const captureScrollAnchor = useScrollAnchor(filterKey, focusAfterCardLeaves)

  // Drop an unknown or default sort, bad tag ids, and non-"1" flags from the
  // URL without a history entry. Once validated, retired and unknown tag ids go too.
  useEffect(() => {
    const canonical = canonicalWatchlistParams(params)
    if (tagIdsValidatedRef.current) {
      const settled = settleTagIds(parseWatchlistParams(canonical).tagIds)
      if (settled.length === 0) canonical.delete("tags")
      else canonical.set("tags", settled.join(","))
    }
    if (canonical.toString() === params.toString()) return
    writeKeepingSticky(
      (p) => {
        for (const key of ["sort", "tags", "muted", "custom"]) {
          const value = canonical.get(key)
          if (value === null) p.delete(key)
          else p.set(key, value)
        }
      },
      { replace: true }
    )
    // eslint-disable-next-line react-hooks/exhaustive-deps -- settleTagIds and writeKeepingSticky only read refs and setters
  }, [params, write])

  useEffect(() => {
    return () => {
      if (qTimer.current) clearTimeout(qTimer.current)
      if (countTimer.current) clearTimeout(countTimer.current)
    }
  }, [])

  // Resolves with the loaded rows (null on failure); never touches the sticky set.
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

  // Runs once on the first successful load; later refreshes never prune (D-13).
  useEffect(() => {
    if (!entries || tagIdsValidatedRef.current) return
    tagIdsValidatedRef.current = true
    const carried = new Set(entries.flatMap((r) => r.tags.map((t) => t.id)))
    const kept = view.tagIds.filter((id) => carried.has(id))
    if (kept.length !== view.tagIds.length) writeTagIds(kept)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- first load only
  }, [entries])

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

  // Swap or drop one selected tag id, de-duplicated and ascending.
  function replaceSelectedTag(from: number, to: number | null) {
    writeKeepingSticky(
      (p) => {
        const ids = parseWatchlistParams(p).tagIds
        if (!ids.includes(from)) return
        const next = new Set(ids.filter((id) => id !== from))
        if (to !== null) next.add(to)
        if (next.size === 0) p.delete("tags")
        else p.set("tags", [...next].sort((a, b) => a - b).join(","))
      },
      { replace: true }
    )
  }

  // The only place rows' tags change for a Manage tags rename/merge/delete.
  function applyTagChange(change: TagChange) {
    if (change.kind === "renamed") {
      tagNamesRef.current.set(change.tag.id, change.tag.name)
    } else if (change.kind === "deleted") {
      retiredTagIdsRef.current.set(change.tagId, null)
      replaceSelectedTag(change.tagId, null)
    } else {
      retiredTagIdsRef.current.set(change.sourceId, change.target.id)
      tagNamesRef.current.set(change.target.id, change.target.name)
      replaceSelectedTag(change.sourceId, change.target.id)
    }
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

  function handleSelectChange(id: number, next: boolean, shift: boolean) {
    const order = visible.map((e) => e.id)
    let result: ReadonlySet<number>
    if (shift) {
      result = applyRange(order, anchorRef.current, id, next, selectedIds)
    } else if (selectedIds.has(id) === next) {
      result = selectedIds
    } else {
      const out = new Set(selectedIds)
      if (next) out.add(id)
      else out.delete(id)
      result = out
    }
    setSelectedIds(result)
    anchorRef.current = id
    const n = effectiveSelection(visible, result).length
    if (effective.length === 0 && n > 0) {
      announce(`${n} selected. Selection actions follow the artist list.`)
    }
  }

  // The box's own next value is ignored: the result depends on the visible set.
  function handleSelectAllVisible() {
    const next = toggleAllVisible(visible, selectedIds)
    setSelectedIds(next)
    const n = effectiveSelection(visible, next).length
    announce(
      n > 0 ? `Selected ${n} visible artist${plural(n)}.` : "Selection cleared."
    )
  }

  function finishBulkSuccess() {
    setSelectedIds(EMPTY_IDS)
    anchorRef.current = null
    setPendingFocus(true)
  }

  // First of: Select all visible, the filtered-to-zero Clear filters, the h1.
  function resolveFallbackTarget(): HTMLElement | null {
    if (entries && entries.length > 0) {
      return visible.length > 0 ? selectAllRef.current : clearFiltersRef.current
    }
    return pageHeadingRef.current
  }

  function clearSelection() {
    setSelectedIds(EMPTY_IDS)
    anchorRef.current = null
    setPendingFocus(true)
    announce("Selection cleared.")
  }

  // `max` comes from the response body; Go and TS share no constant.
  function showBulkFailure(error: unknown, fallbackMessage: string) {
    if (error instanceof ApiError && error.code === "bulk_too_many_ids") {
      const max = (error.body as { max?: unknown } | undefined)?.max
      if (typeof max === "number") {
        toast.error(
          `Too many artists selected — select at most ${max} and try again.`
        )
        return
      }
    }
    toast.error(fallbackMessage)
  }

  function openRemoveConfirm() {
    setRemoveConfirm({
      open: true,
      ids: effective.map((e) => e.id),
      names: effective.map((e) => e.name),
    })
  }

  // Never throws and never touches selection or entries: those wait for the
  // dialog to close (U1).
  async function confirmRemove() {
    setBulkInFlight(true)
    try {
      const out = await bulkRemoveWatchlist(removeConfirm.ids)
      removeOutcomeRef.current = { ok: true, out }
    } catch (error) {
      removeOutcomeRef.current = { ok: false, error }
    }
  }

  function handleRemoveDialogOpenChange(open: boolean) {
    if (open) return
    const outcome = removeOutcomeRef.current
    removeOutcomeRef.current = null
    removeCloseKindRef.current = outcome?.ok ? "success" : "return"
    setRemoveConfirm((prev) => ({ ...prev, open: false }))
    setBulkInFlight(false)
    if (!outcome) return
    if (!outcome.ok) {
      showBulkFailure(outcome.error, BULK_REMOVE_FAILED)
      return
    }
    const gone = new Set([...outcome.out.removed, ...outcome.out.already_gone])
    setEntries((rows) => (rows ? rows.filter((r) => !gone.has(r.id)) : rows))
    finishBulkSuccess()
    const g = outcome.out.already_gone.length
    const msg = `Removed ${gone.size} artist${plural(gone.size)} from your watchlist.`
    if (g > 0) toast.success(msg, { description: formatAlreadyRemoved(g) })
    else toast.success(msg)
    refresh()
  }

  if (entries) {
    for (const r of entries) {
      for (const t of r.tags) tagNamesRef.current.set(t.id, t.name)
    }
  }
  const tagOptions = entries ? tagFilterOptions(entries) : []
  const selectedTags = sortTagsByName(
    view.tagIds.flatMap((id) => {
      const name = tagNamesRef.current.get(id)
      if (name === undefined) return []
      return [
        { id, name, count: tagOptions.find((o) => o.id === id)?.count ?? 0 },
      ]
    })
  )

  const filterTagIds = new Set(view.tagIds)
  const visible = entries
    ? selectVisible(entries, { ...view, q: query }, stickyIds)
    : []
  const effective = effectiveSelection(visible, selectedIds)
  countRef.current = {
    n: visible.length,
    m: entries?.length ?? 0,
    k: effective.length,
  }
  const allState = selectAllState(visible.length, effective.length)

  // Focus moves only once the bar has unmounted and nothing is in flight (F4).
  useEffect(() => {
    if (!pendingFocus || bulkInFlight || effective.length > 0) return
    setPendingFocus(false)
    resolveFallbackTarget()?.focus()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- resolveFallbackTarget reads this render's values
  }, [pendingFocus, bulkInFlight, effective.length])

  function handleTagsChange(ids: number[]) {
    write((p) => {
      if (ids.length === 0) p.delete("tags")
      else p.set("tags", ids.join(","))
    })
    scheduleCountAnnouncement()
  }

  function handleFlagChange(key: "muted" | "custom", next: boolean) {
    write((p) => {
      if (next) p.set(key, "1")
      else p.delete(key)
    })
    scheduleCountAnnouncement()
  }

  function toggleFilterTag(
    tag: TagRef,
    anchor?: HTMLElement,
    viaKeyboard?: boolean
  ) {
    if (anchor) captureScrollAnchor(anchor, viaKeyboard)
    tagNamesRef.current.set(tag.id, tag.name)
    const ids = new Set(view.tagIds)
    if (!ids.delete(tag.id)) ids.add(tag.id)
    handleTagsChange([...ids].sort((a, b) => a - b))
  }

  function handleSortChange(next: SortKey) {
    write((p) => {
      if (next === DEFAULT_SORT) p.delete("sort")
      else p.set("sort", next)
    })
    if (countTimer.current) clearTimeout(countTimer.current)
    const label = SORT_OPTIONS.find((o) => o.value === next)!.label
    announce(`Sorted by ${label}.`)
  }

  function clearFilters() {
    if (qTimer.current) clearTimeout(qTimer.current)
    if (countTimer.current) clearTimeout(countTimer.current)
    setQuery("")
    lastWrittenQ.current = ""
    write((p) => {
      for (const key of CLEAR_FILTER_KEYS) p.delete(key)
    })
    if (view.tagIds.length === 0 && !view.muted && !view.custom) clearSticky()
    filterInputRef.current?.focus()
    const m = entries?.length ?? 0
    announce(`Filters cleared. Showing ${formatArtistCount(m, m)}.`)
  }

  return (
    <TagVocabularyProvider onChange={applyTagChange}>
      <div className="flex flex-col gap-6 p-8 pb-[calc(2rem+var(--bulk-bar-clearance,0px))]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <h1
            ref={pageHeadingRef}
            tabIndex={-1}
            className="text-display font-semibold text-foreground outline-none"
          >
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

        <ConfirmDialog
          open={removeConfirm.open}
          onOpenChange={handleRemoveDialogOpenChange}
          title={
            removeConfirm.ids.length === 1
              ? `Remove ${removeConfirm.names[0]} from your watchlist?`
              : `Remove ${removeConfirm.ids.length} artists from your watchlist?`
          }
          description={
            removeConfirm.ids.length === 1
              ? "Their note is deleted but their tags are kept; alerts already found for them may still be sent, and re-adding them later may alert you about releases from while they were removed."
              : `${formatNameList(removeConfirm.names)}. Their notes are deleted but their tags are kept; alerts already found for them may still be sent, and re-adding any of them later may alert you about releases from while they were removed.`
          }
          actionLabel={
            removeConfirm.ids.length === 1
              ? "Remove 1 artist"
              : `Remove ${removeConfirm.ids.length} artists`
          }
          pendingLabel="Removing…"
          variant="destructive"
          onConfirm={confirmRemove}
          finalFocus={() =>
            removeCloseKindRef.current === "success"
              ? resolveFallbackTarget()
              : removeButtonRef.current
          }
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
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2
                id="watchlist-artists-heading"
                ref={headingRef}
                tabIndex={-1}
                className="text-heading font-semibold text-foreground outline-none"
              >
                Your artists
              </h2>
              <div className="flex flex-wrap items-center gap-4">
                <label className="flex min-h-11 cursor-pointer items-center gap-2 text-label text-foreground select-none">
                  <Checkbox
                    ref={selectAllRef}
                    className="border-muted-foreground"
                    checked={allState === "checked"}
                    indeterminate={allState === "indeterminate"}
                    disabled={visible.length === 0 || bulkInFlight}
                    onCheckedChange={handleSelectAllVisible}
                  />
                  Select all visible
                </label>
                <p className="text-label text-muted-foreground tabular-nums">
                  {formatArtistCount(visible.length, entries.length)}
                </p>
              </div>
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
              tagOptions={tagOptions}
              selectedTags={selectedTags}
              onSelectedTagsChange={handleTagsChange}
              muted={view.muted}
              onMutedChange={(next) => handleFlagChange("muted", next)}
              custom={view.custom}
              onCustomChange={(next) => handleFlagChange("custom", next)}
            />
            {visible.length === 0 ? (
              <EmptyState
                heading="No artists match these filters"
                body={`Try a different name or fewer filters, or clear them to see all ${entries.length} artist${entries.length === 1 ? "" : "s"}.`}
                action={
                  <Button ref={clearFiltersRef} onClick={clearFilters}>
                    Clear filters
                  </Button>
                }
              />
            ) : (
              <ul className="flex flex-col gap-4">
                {visible.map((entry) => (
                  <WatchlistRow
                    key={entry.id}
                    entry={entry}
                    onEntryChange={handleEntryChange}
                    onRemove={handleRemove}
                    tagActions={{
                      addTag,
                      removeTag,
                      filterTagIds,
                      toggleFilterTag,
                    }}
                    announce={announce}
                    selected={selectedIds.has(entry.id)}
                    selectionDisabled={bulkInFlight}
                    onSelectChange={handleSelectChange}
                  />
                ))}
              </ul>
            )}
            {effective.length > 0 && (
              <BulkActionBar
                selected={effective}
                inFlight={bulkInFlight}
                onRemove={openRemoveConfirm}
                onClear={clearSelection}
                removeButtonRef={removeButtonRef}
              />
            )}
          </section>
        )}
      </div>
    </TagVocabularyProvider>
  )
}
