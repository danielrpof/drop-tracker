import { useEffect, useLayoutEffect, useRef, useState } from "react"

import { toast } from "sonner"

import { ConfirmDialog } from "~/components/common/ConfirmDialog"
import { EmptyState } from "~/components/common/EmptyState"
import { Button } from "~/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog"
import { Input } from "~/components/ui/input"
import { Skeleton } from "~/components/ui/skeleton"
import {
  deleteTag,
  listTags,
  mergeTag,
  renameTag,
  type TagRef,
  type TagSummary,
} from "~/lib/api"

export interface ManageTagsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onLoaded: (tags: TagSummary[]) => void
  onDeleted: (tagId: number) => void
  onRenamed: (tag: TagRef) => void
  onMerged: (sourceId: number, target: TagSummary) => void
}

// CollisionTarget is set when renameTag resolves a collision (D-09): the
// merge ConfirmDialog it drives never computes the post-merge count itself
// -- carrierCountAfterMerge comes straight from the server's 409 body.
interface CollisionTarget {
  sourceId: number
  sourceName: string
  target: TagRef
  carrierCountAfterMerge: number
}

type Status = "loading" | "loaded" | "error"

function sortTags(tags: TagSummary[]): TagSummary[] {
  return [...tags].sort((a, b) => {
    const cmp = a.name.localeCompare(b.name, undefined, {
      sensitivity: "base",
    })
    return cmp !== 0 ? cmp : a.id - b.id
  })
}

function pluralize(n: number): string {
  return n === 1 ? "artist" : "artists"
}

// FocusRequest is a single pending "move focus once the list re-renders"
// instruction (UI-SPEC focus table row (d)), consumed by the layout effect
// below -- mirrors TagChips' FocusRequest pattern.
type FocusRequest = { kind: "row"; index: number } | { kind: "close" } | null

// ManageTagsDialog (D-07, D-08) is the global tag-management surface opened
// from the Watchlist header. It re-fetches the vocabulary on every open
// (D-30) and lets the user delete a tag through a count-stating
// ConfirmDialog rendered inside its own React tree (so base-ui registers it
// as a nested dialog). Task 2/3 add rename and merge on top of this.
export function ManageTagsDialog({
  open,
  onOpenChange,
  onLoaded,
  onDeleted,
  onRenamed,
  onMerged,
}: ManageTagsDialogProps) {
  const [status, setStatus] = useState<Status>("loading")
  const [tags, setTags] = useState<TagSummary[]>([])
  const [deleteTarget, setDeleteTarget] = useState<TagSummary | null>(null)
  const [renameTarget, setRenameTarget] = useState<TagSummary | null>(null)
  const [renameValue, setRenameValue] = useState("")
  const [renamePending, setRenamePending] = useState(false)
  const [collisionTarget, setCollisionTarget] =
    useState<CollisionTarget | null>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const focusRequestRef = useRef<FocusRequest>(null)
  const cancelFocusIndexRef = useRef<number | null>(null)
  const renameInputRef = useRef<HTMLInputElement>(null)
  const renameSaveRef = useRef<HTMLButtonElement>(null)
  // Drops a GET /tags that a newer load or a successful mutation superseded (WR-08).
  const loadGen = useRef(0)
  const loadInFlight = useRef(false)
  const initialFocusPendingRef = useRef(false)

  function load() {
    const gen = ++loadGen.current
    loadInFlight.current = true
    setStatus("loading")
    listTags()
      .then((result) => {
        if (gen !== loadGen.current) return
        loadInFlight.current = false
        const sorted = sortTags(result)
        setTags(sorted)
        setStatus("loaded")
        onLoaded(sorted)
      })
      .catch(() => {
        if (gen !== loadGen.current) return
        loadInFlight.current = false
        setStatus("error")
      })
  }

  // A mutation supersedes any in-flight load; refetch so the dialog never
  // strands on its skeleton after that response is dropped.
  function invalidateLoad() {
    loadGen.current++
    if (loadInFlight.current) load()
  }

  useEffect(() => {
    initialFocusPendingRef.current = open
    if (open) load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  useLayoutEffect(() => {
    const req = focusRequestRef.current
    if (!req) return
    focusRequestRef.current = null
    if (req.kind === "close") {
      document
        .querySelector<HTMLButtonElement>('[data-slot="dialog-close"]')
        ?.focus()
      return
    }
    const row =
      listRef.current?.querySelectorAll<HTMLLIElement>("li")[req.index]
    row?.querySelector<HTMLButtonElement>("button")?.focus()
  }, [tags])

  // The first load of each open hands focus to row 1's Rename (G-24-10);
  // later reloads, and focus the user already moved, are left alone.
  useLayoutEffect(() => {
    if (status !== "loaded" || !initialFocusPendingRef.current) return
    initialFocusPendingRef.current = false
    const list = listRef.current
    const target = list?.querySelector<HTMLButtonElement>("li button")
    if (!list || !target) return
    const popup = list.closest('[data-slot="dialog-content"]')
    const active = document.activeElement
    const closeButton = popup?.querySelector('[data-slot="dialog-close"]')
    const fromPopupChrome =
      !active ||
      active === document.body ||
      !popup?.contains(active) ||
      active === popup ||
      active === closeButton
    if (fromPopupChrome) target.focus()
  }, [status])

  // Selects the rename input's text once it mounts (UI-SPEC: "the rename
  // input opens with its text selected"), and focuses it.
  useEffect(() => {
    if (!renameTarget) return
    renameInputRef.current?.focus()
    renameInputRef.current?.select()
  }, [renameTarget])

  // Cancel/Esc doesn't touch `tags`, so the tags-keyed layout effect above
  // never fires for it -- this effect handles that focus return separately
  // (UI-SPEC focus table row (d): rename Cancel/Esc -> that row's Rename).
  useEffect(() => {
    if (renameTarget !== null) return
    const index = cancelFocusIndexRef.current
    if (index === null) return
    cancelFocusIndexRef.current = null
    const row = listRef.current?.querySelectorAll<HTMLLIElement>("li")[index]
    row?.querySelector<HTMLButtonElement>("button")?.focus()
  }, [renameTarget])

  function startRename(tag: TagSummary) {
    setRenameTarget(tag)
    setRenameValue(tag.name)
  }

  function cancelRename(index: number) {
    cancelFocusIndexRef.current = index
    setRenameTarget(null)
    setRenameValue("")
  }

  async function handleSaveRename(tag: TagSummary) {
    const trimmed = renameValue.trim()
    if (renamePending || trimmed === "" || trimmed === tag.name) return
    setRenamePending(true)
    try {
      const result = await renameTag(tag.id, trimmed)
      if (result.kind === "renamed") {
        invalidateLoad()
        setTags((prev) => {
          const updated = prev.map((t) =>
            t.id === tag.id ? { ...t, name: result.tag.name } : t
          )
          const sorted = sortTags(updated)
          const newIndex = sorted.findIndex((t) => t.id === tag.id)
          focusRequestRef.current = { kind: "row", index: newIndex }
          return sorted
        })
        setRenameTarget(null)
        setRenameValue("")
        setRenamePending(false)
        onRenamed(result.tag)
        toast.success(`Renamed “${tag.name}” to “${result.tag.name}”.`)
      } else {
        // A collision opens the merge ConfirmDialog (D-09); rename mode
        // stays open with its typed text until the user confirms or
        // cancels the merge. No merge request is ever sent without that
        // confirmation.
        setRenamePending(false)
        setCollisionTarget({
          sourceId: tag.id,
          sourceName: tag.name,
          target: result.target,
          carrierCountAfterMerge: result.carrierCountAfterMerge,
        })
      }
    } catch {
      setRenamePending(false)
      toast.error(`Couldn't rename “${tag.name}” — try again.`)
    }
  }

  async function handleConfirmMerge() {
    if (!collisionTarget) return
    const { sourceId, sourceName, target } = collisionTarget
    try {
      const merged = await mergeTag(sourceId, target.id)
      invalidateLoad()
      setTags((prev) => {
        const withoutSource = prev.filter((t) => t.id !== sourceId)
        const updated = withoutSource.map((t) =>
          t.id === target.id
            ? { ...t, name: merged.name, carrier_count: merged.carrier_count }
            : t
        )
        const sorted = sortTags(updated)
        const newIndex = sorted.findIndex((t) => t.id === target.id)
        focusRequestRef.current = { kind: "row", index: newIndex }
        return sorted
      })
      setRenameTarget(null)
      setRenameValue("")
      onMerged(sourceId, merged)
      toast.success(
        `Merged “${sourceName}” into “${merged.name}” — ${merged.carrier_count} ${pluralize(merged.carrier_count)} now carry it.`
      )
    } catch {
      toast.error(
        `Couldn't merge “${sourceName}” into “${target.name}” — try again.`
      )
      load()
    }
  }

  async function handleConfirmDelete() {
    if (!deleteTarget) return
    const tag = deleteTarget
    const index = tags.findIndex((t) => t.id === tag.id)
    try {
      const { carrier_count } = await deleteTag(tag.id)
      invalidateLoad()
      setTags((prev) => {
        const remaining = prev.filter((t) => t.id !== tag.id)
        focusRequestRef.current =
          remaining.length === 0
            ? { kind: "close" }
            : { kind: "row", index: Math.min(index, remaining.length - 1) }
        return remaining
      })
      onDeleted(tag.id)
      toast.success(
        carrier_count === 0
          ? `Deleted “${tag.name}”.`
          : `Deleted “${tag.name}” from ${carrier_count} ${pluralize(carrier_count)}.`
      )
    } catch {
      toast.error(`Couldn't delete “${tag.name}” — try again.`)
      load()
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-h-[calc(100dvh-2rem)] grid-rows-[auto_1fr] sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Manage tags</DialogTitle>
            <DialogDescription>
              Rename or delete tags across all artists. Counts include only
              artists on your watchlist.
            </DialogDescription>
          </DialogHeader>

          {status === "loading" && (
            <div className="flex flex-col gap-2">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          )}

          {status === "error" && (
            <EmptyState
              heading="Couldn't load tags."
              body="Please try again."
              action={
                <Button variant="secondary" onClick={load}>
                  Retry
                </Button>
              }
            />
          )}

          {status === "loaded" && tags.length === 0 && (
            <EmptyState
              heading="No tags yet"
              body="Add a tag from any artist's card on the Watchlist and it will appear here."
            />
          )}

          {status === "loaded" && tags.length > 0 && (
            <ul
              ref={listRef}
              aria-label="Tags"
              className="-mx-2 max-h-[min(60vh,28rem)] divide-y divide-border overflow-y-auto overscroll-contain px-2"
            >
              {tags.map((tag, index) =>
                renameTarget?.id === tag.id ? (
                  <li key={tag.id} className="flex items-center gap-2 py-2">
                    <Input
                      ref={renameInputRef}
                      value={renameValue}
                      onChange={(e) => setRenameValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault()
                          void handleSaveRename(tag)
                        } else if (e.key === "Escape") {
                          e.preventDefault()
                          e.stopPropagation()
                          cancelRename(index)
                        }
                      }}
                      maxLength={32}
                      readOnly={renamePending}
                      aria-label={`New name for ${tag.name}`}
                      className="h-8 flex-1"
                    />
                    {renameValue.length >= 25 && (
                      <span className="shrink-0 text-label text-muted-foreground tabular-nums">
                        {renameValue.length}/32
                      </span>
                    )}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => cancelRename(index)}
                    >
                      Cancel
                    </Button>
                    <Button
                      ref={renameSaveRef}
                      variant="default"
                      size="sm"
                      className="min-w-20"
                      disabled={
                        renamePending ||
                        renameValue.trim() === "" ||
                        renameValue.trim() === tag.name
                      }
                      onClick={() => void handleSaveRename(tag)}
                    >
                      {renamePending ? "Saving…" : "Save"}
                    </Button>
                  </li>
                ) : (
                  <li key={tag.id} className="flex items-center gap-4 py-2">
                    <div className="flex min-w-0 flex-1 items-baseline gap-1">
                      <span
                        className="truncate text-body text-foreground"
                        title={tag.name}
                      >
                        {tag.name}
                      </span>
                      <span className="shrink-0 text-label whitespace-nowrap text-muted-foreground">
                        · {tag.carrier_count} {pluralize(tag.carrier_count)}
                      </span>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`Rename tag ${tag.name}`}
                      onClick={() => startRename(tag)}
                    >
                      Rename
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-muted-foreground hover:text-destructive"
                      aria-label={`Delete tag ${tag.name}`}
                      onClick={() => setDeleteTarget(tag)}
                    >
                      Delete
                    </Button>
                  </li>
                )
              )}
            </ul>
          )}
        </DialogContent>
      </Dialog>

      {deleteTarget && (
        <ConfirmDialog
          open
          onOpenChange={(next) => {
            if (!next) setDeleteTarget(null)
          }}
          title={
            deleteTarget.carrier_count === 0
              ? `Delete “${deleteTarget.name}”?`
              : `Delete “${deleteTarget.name}” from ${deleteTarget.carrier_count} ${pluralize(deleteTarget.carrier_count)}?`
          }
          description={
            deleteTarget.carrier_count === 0
              ? "No artists on your watchlist carry this tag, and this can't be undone."
              : "The tag is removed everywhere and this can't be undone; the artists stay on your watchlist."
          }
          actionLabel="Delete tag"
          pendingLabel="Deleting…"
          variant="destructive"
          onConfirm={handleConfirmDelete}
        />
      )}

      {collisionTarget && (
        <ConfirmDialog
          open
          onOpenChange={(next) => {
            if (!next) setCollisionTarget(null)
          }}
          title={`Merge “${collisionTarget.sourceName}” into “${collisionTarget.target.name}”?`}
          description={`${collisionTarget.carrierCountAfterMerge} ${pluralize(collisionTarget.carrierCountAfterMerge)} will carry “${collisionTarget.target.name}”, and “${collisionTarget.sourceName}” will be deleted.`}
          actionLabel="Merge tags"
          pendingLabel="Merging…"
          variant="default"
          onConfirm={handleConfirmMerge}
          finalFocus={renameSaveRef}
        />
      )}
    </>
  )
}
