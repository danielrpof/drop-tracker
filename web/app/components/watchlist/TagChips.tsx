import { useLayoutEffect, useRef } from "react"

import { X } from "lucide-react"
import { toast } from "sonner"

import { Badge } from "~/components/ui/badge"
import { Button } from "~/components/ui/button"
import { detachTag, type TagRef, type WatchlistEntry } from "~/lib/api"

// TagActions is the route-level functional-updater pair TagChips drives
// (D-24, amends D-03): each call touches only its own tag, never a
// whole-array snapshot, so concurrent removals on one row can't clobber
// each other.
export interface TagActions {
  addTag(entryId: number, tag: TagRef, index?: number): void
  removeTag(entryId: number, tagId: number): void
}

export interface TagChipsProps {
  entry: WatchlistEntry
  actions: TagActions
  announce: (message: string) => void
}

// TagChips renders entry.tags as neutral Badge chips under the artist name
// (D-01, D-04, UI-SPEC [R1]). A tag name is plain JSX text only -- never
// raw HTML -- so an HTML-looking name renders literally (Phase 06 XSS
// posture).
export function TagChips({ entry, actions, announce }: TagChipsProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  // Index (in the post-removal array) to focus once entry.tags actually
  // shrinks -- set synchronously by handleRemove, consumed by the layout
  // effect below, and cleared on a failed detach so a restore never pulls
  // focus back (UI-SPEC focus table row b).
  const pendingFocusIndexRef = useRef<number | null>(null)
  const prevTagCountRef = useRef(entry.tags.length)

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

  if (entry.tags.length === 0) {
    return null
  }

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
    </div>
  )
}
