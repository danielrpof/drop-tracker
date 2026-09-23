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
  if (entry.tags.length === 0) {
    return null
  }

  async function handleRemove(tag: TagRef, index: number) {
    actions.removeTag(entry.id, tag.id)
    try {
      await detachTag(entry.id, tag.id)
    } catch {
      actions.addTag(entry.id, tag, index)
      toast.error(
        `Couldn't remove “${tag.name}” from ${entry.name} — try again.`
      )
    }
  }

  return (
    <div className="mt-1 flex flex-wrap items-center gap-2">
      {entry.tags.map((tag, index) => (
        <Badge
          key={tag.id}
          variant="secondary"
          className="h-6 max-w-full gap-1 pr-0 pl-2 text-label font-normal"
        >
          <span className="min-w-0 truncate" title={tag.name}>
            {tag.name}
          </span>
          <Button
            variant="ghost"
            size="icon-xs"
            className="-ml-1 text-muted-foreground hover:text-foreground"
            aria-label={`Remove tag ${tag.name} from ${entry.name}`}
            onClick={() => {
              void handleRemove(tag, index)
            }}
          >
            <X aria-hidden="true" className="size-3" />
          </Button>
        </Badge>
      ))}
    </div>
  )
}
