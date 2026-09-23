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
import { Skeleton } from "~/components/ui/skeleton"
import { deleteTag, listTags, type TagSummary } from "~/lib/api"

export interface ManageTagsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onLoaded: (tags: TagSummary[]) => void
  onDeleted: (tagId: number) => void
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
}: ManageTagsDialogProps) {
  const [status, setStatus] = useState<Status>("loading")
  const [tags, setTags] = useState<TagSummary[]>([])
  const [deleteTarget, setDeleteTarget] = useState<TagSummary | null>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const focusRequestRef = useRef<FocusRequest>(null)

  function load() {
    setStatus("loading")
    listTags()
      .then((result) => {
        const sorted = sortTags(result)
        setTags(sorted)
        setStatus("loaded")
        onLoaded(sorted)
      })
      .catch(() => setStatus("error"))
  }

  useEffect(() => {
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

  async function handleConfirmDelete() {
    if (!deleteTarget) return
    const tag = deleteTarget
    const index = tags.findIndex((t) => t.id === tag.id)
    try {
      const { carrier_count } = await deleteTag(tag.id)
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
              {tags.map((tag) => (
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
                    className="text-muted-foreground hover:text-destructive"
                    aria-label={`Delete tag ${tag.name}`}
                    onClick={() => setDeleteTarget(tag)}
                  >
                    Delete
                  </Button>
                </li>
              ))}
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
    </>
  )
}
