import { useLayoutEffect, useRef, useState } from "react"

import { Plus, X } from "lucide-react"
import { toast } from "sonner"

import { Badge } from "~/components/ui/badge"
import { Button } from "~/components/ui/button"
import {
  attachTag,
  detachTag,
  type TagRef,
  type WatchlistEntry,
} from "~/lib/api"

import { TagCombobox } from "./TagCombobox"

// MAX_TAGS_PER_ARTIST mirrors the DB trigger's cap (D-14, TAG-04): the "+
// tag" trailing slot swaps to the "max 10 tags" hint at this count.
const MAX_TAGS_PER_ARTIST = 10

// TagActions is the route-level functional-updater pair TagChips drives
// (D-24, amends D-03): each call touches only its own tag, never a
// whole-array snapshot, so concurrent removals on one row can't clobber
// each other. vocabulary/loadVocabulary/rememberTag back the "+ tag"
// autocomplete (D-30): the vocabulary loads on first use, is kept in route
// state, and grows as tags are created.
export interface TagActions {
  addTag(entryId: number, tag: TagRef, index?: number): void
  removeTag(entryId: number, tagId: number): void
  vocabulary: TagRef[] | null
  loadVocabulary(): void
  rememberTag(tag: TagRef): void
}

// pendingIdCounter backs the module-level temp-id sequence for optimistic
// chips (D-24): a per-row pending set keyed by a temporary id, never wiped
// by a concurrent refresh().
let pendingIdCounter = 0
function nextPendingId(): string {
  pendingIdCounter += 1
  return `pending-${pendingIdCounter}`
}

export interface TagChipsProps {
  entry: WatchlistEntry
  actions: TagActions
  announce: (message: string) => void
}

interface PendingTag {
  tempId: string
  name: string
}

// TagChips renders entry.tags as neutral Badge chips under the artist name
// (D-01, D-04, UI-SPEC [R1]). A tag name is plain JSX text only -- never
// raw HTML -- so an HTML-looking name renders literally (Phase 06 XSS
// posture). The row always renders -- it holds at least the "+ tag"
// trigger, never an empty gap (D-02).
export function TagChips({ entry, actions, announce }: TagChipsProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  // Index (in the post-removal array) to focus once entry.tags actually
  // shrinks -- set synchronously by handleRemove, consumed by the layout
  // effect below, and cleared on a failed detach so a restore never pulls
  // focus back (UI-SPEC focus table row b).
  const pendingFocusIndexRef = useRef<number | null>(null)
  const prevTagCountRef = useRef(entry.tags.length)
  // Optimistic chips keyed by a temp id (D-24): survive a concurrent
  // refresh() because they live here, not in entry.tags.
  const [pending, setPending] = useState<PendingTag[]>([])
  const [editorOpen, setEditorOpen] = useState(false)

  useLayoutEffect(() => {
    const removed = entry.tags.length < prevTagCountRef.current
    prevTagCountRef.current = entry.tags.length
    const target = pendingFocusIndexRef.current
    pendingFocusIndexRef.current = null
    if (!removed || target === null) return
    const buttons =
      containerRef.current?.querySelectorAll<HTMLButtonElement>("button")
    buttons?.[target]?.focus()
  }, [entry.tags])

  function handleRemove(tag: TagRef, index: number) {
    const hadTen = entry.tags.length === 10
    const newLength = entry.tags.length - 1
    pendingFocusIndexRef.current =
      newLength === 0 ? null : Math.min(index, newLength - 1)

    actions.removeTag(entry.id, tag.id)
    announce(
      hadTen
        ? `Removed “${tag.name}” from ${entry.name}. You can add tags again.`
        : `Removed “${tag.name}” from ${entry.name}.`
    )

    detachTag(entry.id, tag.id).catch(() => {
      pendingFocusIndexRef.current = null
      actions.addTag(entry.id, tag, index)
      toast.error(
        `Couldn't remove “${tag.name}” from ${entry.name} — try again.`
      )
    })
  }

  function handleOpen() {
    actions.loadVocabulary()
    setEditorOpen(true)
  }

  // handleCommit is TagCombobox's onCommit (both a picked existing tag and
  // a typed new name land here -- the combobox already resolved which).
  // The pending chip shows the typed casing immediately; on success it is
  // replaced by the real chip carrying the server's stored casing (TAG-03).
  function handleCommit(name: string) {
    const tempId = nextPendingId()
    setPending((p) => [...p, { tempId, name }])

    attachTag(entry.id, name)
      .then((tag) => {
        setPending((p) => p.filter((item) => item.tempId !== tempId))
        actions.addTag(entry.id, tag)
        actions.rememberTag(tag)
      })
      .catch(() => {
        setPending((p) => p.filter((item) => item.tempId !== tempId))
        toast.error(`Couldn't add “${name}” to ${entry.name} — try again.`)
      })
  }

  return (
    <div ref={containerRef} className="mt-1 flex flex-wrap items-center gap-2">
      {entry.tags.map((tag, index) => (
        <Badge
          key={tag.id}
          variant="secondary"
          className="group/chip h-6 max-w-full gap-1 pr-0 pl-2 text-label font-normal focus-within:h-auto focus-within:min-h-6 focus-within:py-0.5"
        >
          <span
            className="min-w-0 truncate group-focus-within/chip:break-all group-focus-within/chip:whitespace-normal"
            title={tag.name}
          >
            {tag.name}
          </span>
          <Button
            variant="ghost"
            size="icon-xs"
            className="-ml-1 text-muted-foreground hover:text-foreground"
            aria-label={`Remove tag ${tag.name} from ${entry.name}`}
            onClick={() => handleRemove(tag, index)}
          >
            <X aria-hidden="true" className="size-3" />
          </Button>
        </Badge>
      ))}
      {pending.map((item) => (
        <Badge
          key={item.tempId}
          variant="secondary"
          className="group/chip h-6 max-w-full gap-1 pr-0 pl-2 text-label font-normal focus-within:h-auto focus-within:min-h-6 focus-within:py-0.5"
        >
          <span
            className="min-w-0 truncate group-focus-within/chip:break-all group-focus-within/chip:whitespace-normal"
            title={item.name}
          >
            {item.name}
          </span>
          {/* aria-disabled, not disabled: a native-disabled button loses
              focusability, breaking the "10th pick focuses the new chip's
              ×" contract once this chip resolves (see plan objective). */}
          <Button
            variant="ghost"
            size="icon-xs"
            className="-ml-1 text-muted-foreground hover:text-foreground"
            aria-label={`Remove tag ${item.name} from ${entry.name}`}
            aria-disabled="true"
          >
            <X aria-hidden="true" className="size-3" />
          </Button>
        </Badge>
      ))}
      {editorOpen ? (
        <TagCombobox
          entry={entry}
          vocabulary={actions.vocabulary}
          onArtist={entry.tags}
          pendingNames={pending.map((p) => p.name)}
          onCommit={handleCommit}
          onClose={() => setEditorOpen(false)}
        />
      ) : (
        <Button
          variant="ghost"
          size="xs"
          className="h-6 text-label text-muted-foreground hover:text-foreground"
          aria-label={`Add tag to ${entry.name}`}
          onClick={handleOpen}
        >
          <Plus aria-hidden="true" className="size-3" />
          tag
        </Button>
      )}
    </div>
  )
}
