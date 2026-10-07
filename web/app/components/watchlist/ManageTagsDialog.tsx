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
import type { TagRef, TagSummary } from "~/lib/api"
import {
  MAX_TAG_LENGTH,
  TAG_COUNTER_THRESHOLD,
  tagNameLength,
} from "~/lib/limits"
import { useTagVocabulary } from "~/lib/useTagVocabulary"

export interface ManageTagsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
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

const NO_TAGS: TagSummary[] = []

function pluralize(n: number): string {
  return n === 1 ? "artist" : "artists"
}

// FocusRequest is a single pending "move focus once the list re-renders"
// instruction (UI-SPEC focus table row (d)), consumed by the layout effect
// below. Delete waits until the removed row is gone from the list.
type FocusRequest =
  | { kind: "tag"; id: number }
  | { kind: "after-delete"; id: number; index: number }
  | null

// ManageTagsDialog (D-07, D-08) is the global tag-management surface opened
// from the Watchlist header. It reloads the vocabulary on every open
// (D-30) and lets the user delete a tag through a count-stating
// ConfirmDialog rendered inside its own React tree (so base-ui registers it
// as a nested dialog). Task 2/3 add rename and merge on top of this.
export function ManageTagsDialog({
  open,
  onOpenChange,
}: ManageTagsDialogProps) {
  const { vocabulary, status, reload, rename, merge, remove } =
    useTagVocabulary()
  const tags = vocabulary ?? NO_TAGS
  const [focusRequest, setFocusRequest] = useState<FocusRequest>(null)
  const [deleteTarget, setDeleteTarget] = useState<TagSummary | null>(null)
  const [renameTarget, setRenameTarget] = useState<TagSummary | null>(null)
  const [renameValue, setRenameValue] = useState("")
  const renameLength = tagNameLength(renameValue)
  const renameOverLimit = renameLength > MAX_TAG_LENGTH
  const [renamePending, setRenamePending] = useState(false)
  const [collisionTarget, setCollisionTarget] =
    useState<CollisionTarget | null>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const cancelFocusIndexRef = useRef<number | null>(null)
  const renameInputRef = useRef<HTMLInputElement>(null)
  const renameSaveRef = useRef<HTMLButtonElement>(null)
  const initialFocusPendingRef = useRef(false)

  useEffect(() => {
    initialFocusPendingRef.current = open
    if (open) reload()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  useLayoutEffect(() => {
    const req = focusRequest
    if (!req || status !== "loaded") return
    const rows = listRef.current?.querySelectorAll<HTMLLIElement>("li")
    if (req.kind === "after-delete") {
      if (tags.some((t) => t.id === req.id)) return
      setFocusRequest(null)
      if (tags.length === 0) {
        document
          .querySelector<HTMLButtonElement>('[data-slot="dialog-close"]')
          ?.focus()
        return
      }
      rows?.[Math.min(req.index, tags.length - 1)]
        ?.querySelector<HTMLButtonElement>("button")
        ?.focus()
      return
    }
    setFocusRequest(null)
    const index = tags.findIndex((t) => t.id === req.id)
    rows?.[index]?.querySelector<HTMLButtonElement>("button")?.focus()
  }, [tags, status, focusRequest])

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
    if (
      renamePending ||
      renameOverLimit ||
      trimmed === "" ||
      trimmed === tag.name
    )
      return
    setRenamePending(true)
    try {
      const result = await rename(tag.id, trimmed)
      if (result.kind === "renamed") {
        setFocusRequest({ kind: "tag", id: tag.id })
        setRenameTarget(null)
        setRenameValue("")
        setRenamePending(false)
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
      const merged = await merge(sourceId, target.id)
      setFocusRequest({ kind: "tag", id: target.id })
      setRenameTarget(null)
      setRenameValue("")
      toast.success(
        `Merged “${sourceName}” into “${merged.name}” — ${merged.carrier_count} ${pluralize(merged.carrier_count)} now carry it.`
      )
    } catch {
      toast.error(
        `Couldn't merge “${sourceName}” into “${target.name}” — try again.`
      )
    }
  }

  async function handleConfirmDelete() {
    if (!deleteTarget) return
    const tag = deleteTarget
    const index = tags.findIndex((t) => t.id === tag.id)
    try {
      const { carrier_count } = await remove(tag.id)
      setFocusRequest({ kind: "after-delete", id: tag.id, index })
      toast.success(
        carrier_count === 0
          ? `Deleted “${tag.name}”.`
          : `Deleted “${tag.name}” from ${carrier_count} ${pluralize(carrier_count)}.`
      )
    } catch {
      toast.error(`Couldn't delete “${tag.name}” — try again.`)
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

          {(status === "idle" || status === "loading") && (
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
                <Button variant="secondary" onClick={reload}>
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
                      readOnly={renamePending}
                      aria-label={`New name for ${tag.name}`}
                      className="h-8 flex-1"
                    />
                    {renameLength >= TAG_COUNTER_THRESHOLD && (
                      <span
                        className={`shrink-0 text-label tabular-nums ${
                          renameOverLimit
                            ? "text-destructive"
                            : "text-muted-foreground"
                        }`}
                      >
                        {renameLength}/{MAX_TAG_LENGTH}
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
                        renameOverLimit ||
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
