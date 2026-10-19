import {
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react"
import { toast } from "sonner"

import type { ConfirmDialogProps } from "~/components/common/ConfirmDialog"
import type { BulkActionBarProps } from "~/components/watchlist/BulkActionBar"
import {
  ApiError,
  bulkAddTag,
  bulkRemoveTag,
  bulkRemoveWatchlist,
  bulkUpdatePreferences,
  type BulkAddTagOutcome,
  type BulkOutcome,
  type BulkRemoveOutcome,
  type BulkRemoveTagOutcome,
  type PreferencesDelta,
  type TagRef,
  type WatchlistEntry,
} from "~/lib/api"
import {
  addTagToast,
  failureToast,
  plural,
  preferencesToast,
  removeTagToast,
  removeWatchlistToast,
  type ToastSpec,
} from "~/lib/bulkOutcomeMessages"
import {
  applyRange,
  effectiveSelection,
  selectAllState,
  toggleAllVisible,
  type SelectAllState,
} from "~/lib/bulkSelection"
import { formatNameList } from "~/lib/format"
import { EMPTY_IDS } from "~/lib/watchlistView"

const BULK_REMOVE_FAILED =
  "Couldn't remove the selected artists — nothing was removed. Try again."

type RemoveOutcome =
  | { ok: true; out: BulkRemoveOutcome }
  | { ok: false; error: unknown }

function showToast(
  spec: ToastSpec,
  action?: { label: string; onClick: () => void }
) {
  const { level, title, ...rest } = spec
  const options = { ...rest, ...(action && { action }) }
  if (Object.keys(options).length === 0) toast[level](title)
  else toast[level](title, options)
}

function removeDialogCopy(names: string[]) {
  const one = names.length === 1
  return {
    title: one
      ? `Remove ${names[0]} from your watchlist?`
      : `Remove ${names.length} artists from your watchlist?`,
    description: one
      ? "Their note is deleted but their tags are kept; alerts already found for them may still be sent, and re-adding them later may alert you about releases from while they were removed."
      : `${formatNameList(names)}. Their notes are deleted but their tags are kept; alerts already found for them may still be sent, and re-adding any of them later may alert you about releases from while they were removed.`,
    actionLabel: one ? "Remove 1 artist" : `Remove ${names.length} artists`,
  }
}

export interface UseBulkSelectionOptions {
  entries: WatchlistEntry[] | null
  visible: WatchlistEntry[]
  // A selection range never survives a change of what the user sees.
  filterKey: string
  query: string
  setEntries: Dispatch<SetStateAction<WatchlistEntry[] | null>>
  refresh: () => unknown
  markStickyMany: (ids: readonly number[]) => void
  announce: (message: string) => void
}

export interface BulkSelection {
  // Selected ∩ visible; ids hidden by a filter stay selected but inert (D-11).
  effective: WatchlistEntry[]
  selectedIds: ReadonlySet<number>
  inFlight: boolean
  onSelectChange: (id: number, next: boolean, shift: boolean) => void
  selectAll: {
    state: SelectAllState
    disabled: boolean
    onChange: () => void
  }
  // Attach to the elements focus falls back to after a bulk action.
  refs: {
    pageHeading: RefObject<HTMLHeadingElement | null>
    selectAll: RefObject<HTMLElement | null>
    clearFilters: RefObject<HTMLButtonElement | null>
  }
  bar: BulkActionBarProps
  confirmDialog: ConfirmDialogProps
}

export function useBulkSelection({
  entries,
  visible,
  filterKey,
  query,
  setEntries,
  refresh,
  markStickyMany,
  announce,
}: UseBulkSelectionOptions): BulkSelection {
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<number>>(EMPTY_IDS)
  const [inFlight, setInFlight] = useState(false)
  // ids and names are snapshotted at open and kept while the dialog closes.
  const [removeConfirm, setRemoveConfirm] = useState<{
    open: boolean
    ids: number[]
    names: string[]
  }>({ open: false, ids: [], names: [] })
  const [pendingFocus, setPendingFocus] = useState(false)
  const removeOutcomeRef = useRef<RemoveOutcome | null>(null)
  const removeCloseKindRef = useRef<"success" | "return">("return")
  const anchorRef = useRef<number | null>(null)
  const pageHeadingRef = useRef<HTMLHeadingElement | null>(null)
  const selectAllRef = useRef<HTMLElement | null>(null)
  const clearFiltersRef = useRef<HTMLButtonElement | null>(null)
  const removeButtonRef = useRef<HTMLButtonElement | null>(null)
  // Undo outlives the render that built it, so it reads the live visible order.
  const visibleIdsRef = useRef<number[]>([])

  const effective = effectiveSelection(visible, selectedIds)
  visibleIdsRef.current = visible.map((e) => e.id)

  useEffect(() => {
    anchorRef.current = null
  }, [filterKey, query])

  useEffect(() => {
    if (selectedIds.size === 0) anchorRef.current = null
  }, [selectedIds])

  // First of: Select all visible, the filtered-to-zero Clear filters, the h1.
  function resolveFallbackTarget(): HTMLElement | null {
    if (entries && entries.length > 0) {
      return visible.length > 0 ? selectAllRef.current : clearFiltersRef.current
    }
    return pageHeadingRef.current
  }

  // Focus moves only once the bar has unmounted and nothing is in flight.
  useEffect(() => {
    if (!pendingFocus || inFlight || effective.length > 0) return
    setPendingFocus(false)
    resolveFallbackTarget()?.focus()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- resolveFallbackTarget reads this render's values
  }, [pendingFocus, inFlight, effective.length])

  function onSelectChange(id: number, next: boolean, shift: boolean) {
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
  function onSelectAllChange() {
    const next = toggleAllVisible(visible, selectedIds)
    setSelectedIds(next)
    const n = effectiveSelection(visible, next).length
    announce(
      n > 0 ? `Selected ${n} visible artist${plural(n)}.` : "Selection cleared."
    )
  }

  function finishSuccess() {
    setSelectedIds(EMPTY_IDS)
    anchorRef.current = null
    setPendingFocus(true)
  }

  function clearSelection() {
    finishSuccess()
    announce("Selection cleared.")
  }

  // `max` comes from the response body; Go and TS share no constant.
  function showFailure(error: unknown, fallbackMessage: string) {
    const max =
      error instanceof ApiError && error.code === "bulk_too_many_ids"
        ? (error.body as { max?: unknown } | undefined)?.max
        : undefined
    showToast(
      failureToast(typeof max === "number" ? max : null, fallbackMessage)
    )
  }

  function reportAddOutcome(
    outcome: BulkAddTagOutcome,
    sentIds: number[],
    variant: "add" | "restore" = "add"
  ) {
    const nameOf = (id: number) => entries?.find((e) => e.id === id)?.name ?? ""
    showToast(addTagToast(outcome, sentIds, nameOf, variant))
  }

  // Resolves null on failure, which keeps the bar's selection and inline editor.
  async function addTag(name: string): Promise<BulkAddTagOutcome | null> {
    const sentIds = effective.map((e) => e.id)
    setInFlight(true)
    let out: BulkAddTagOutcome
    try {
      out = await bulkAddTag(sentIds, name)
    } catch (error) {
      setInFlight(false)
      showFailure(
        error,
        `Couldn't add “${name}” to the selected artists — try again.`
      )
      return null
    }
    setInFlight(false)
    markStickyMany(out.applied)
    refresh()
    reportAddOutcome(out, sentIds)
    return out
  }

  async function removeTag(tag: TagRef): Promise<BulkRemoveTagOutcome | null> {
    const sentIds = effective.map((e) => e.id)
    setInFlight(true)
    let out: BulkRemoveTagOutcome
    try {
      out = await bulkRemoveTag(sentIds, tag.id)
    } catch (error) {
      setInFlight(false)
      showFailure(
        error,
        `Couldn't remove “${tag.name}” from the selected artists — try again.`
      )
      if (error instanceof ApiError && error.status === 404) refresh()
      return null
    }
    setInFlight(false)
    markStickyMany(out.applied)
    refresh()
    showToast(
      removeTagToast(tag.name, out),
      out.applied.length > 0
        ? {
            label: "Undo",
            onClick: () => undoRemoveTag(tag.name, out.applied),
          }
        : undefined
    )
    return out
  }

  // Leaves selection and bar alone; only ids whose link was removed get the tag back.
  async function undoRemoveTag(name: string, ids: number[]) {
    let out: BulkAddTagOutcome
    try {
      out = await bulkAddTag(ids, name)
    } catch {
      toast.error(`Couldn't restore “${name}” — try again.`)
      return
    }
    markStickyMany(out.applied)
    refresh()
    const order = visibleIdsRef.current
    const shown = order.filter((id) => ids.includes(id))
    const hidden = ids.filter((id) => !order.includes(id)).sort((a, b) => a - b)
    reportAddOutcome(out, [...shown, ...hidden], "restore")
  }

  // Resolves null on failure, which keeps the popover open.
  async function applyPreferences(
    delta: PreferencesDelta
  ): Promise<BulkOutcome | null> {
    const sentIds = effective.map((e) => e.id)
    setInFlight(true)
    let out: BulkOutcome
    try {
      out = await bulkUpdatePreferences(sentIds, delta)
    } catch (error) {
      setInFlight(false)
      showFailure(error, "Couldn't update preferences — try again.")
      return null
    }
    setInFlight(false)
    markStickyMany(out.applied)
    refresh()
    showToast(preferencesToast(out.applied.length, out.skipped))
    return out
  }

  function openRemoveConfirm() {
    setRemoveConfirm({
      open: true,
      ids: effective.map((e) => e.id),
      names: effective.map((e) => e.name),
    })
  }

  // Never throws or touches selection or entries; those wait for the dialog to close.
  async function confirmRemove() {
    setInFlight(true)
    try {
      const out = await bulkRemoveWatchlist(removeConfirm.ids)
      removeOutcomeRef.current = { ok: true, out }
    } catch (error) {
      removeOutcomeRef.current = { ok: false, error }
    }
  }

  function onRemoveDialogOpenChange(open: boolean) {
    if (open) return
    const outcome = removeOutcomeRef.current
    removeOutcomeRef.current = null
    removeCloseKindRef.current = outcome?.ok ? "success" : "return"
    setRemoveConfirm((prev) => ({ ...prev, open: false }))
    setInFlight(false)
    if (!outcome) return
    if (!outcome.ok) {
      showFailure(outcome.error, BULK_REMOVE_FAILED)
      return
    }
    const gone = new Set([...outcome.out.removed, ...outcome.out.already_gone])
    setEntries((rows) => (rows ? rows.filter((r) => !gone.has(r.id)) : rows))
    finishSuccess()
    showToast(removeWatchlistToast(gone.size, outcome.out.already_gone.length))
    refresh()
  }

  return {
    effective,
    selectedIds,
    inFlight,
    onSelectChange,
    selectAll: {
      state: selectAllState(visible.length, effective.length),
      disabled: visible.length === 0 || inFlight,
      onChange: onSelectAllChange,
    },
    refs: {
      pageHeading: pageHeadingRef,
      selectAll: selectAllRef,
      clearFilters: clearFiltersRef,
    },
    bar: {
      selected: effective,
      inFlight,
      onRemove: openRemoveConfirm,
      onAddTag: addTag,
      onRemoveTag: removeTag,
      onApplyPreferences: applyPreferences,
      onSucceeded: finishSuccess,
      onClear: clearSelection,
      removeButtonRef,
    },
    confirmDialog: {
      open: removeConfirm.open,
      onOpenChange: onRemoveDialogOpenChange,
      ...removeDialogCopy(removeConfirm.names),
      pendingLabel: "Removing…",
      variant: "destructive",
      onConfirm: confirmRemove,
      finalFocus: () =>
        removeCloseKindRef.current === "success"
          ? resolveFallbackTarget()
          : removeButtonRef.current,
    },
  }
}
