import { useLayoutEffect, useRef, useState } from "react"

import { Plus, X } from "lucide-react"
import { toast } from "sonner"

import { Badge } from "~/components/ui/badge"
import { Button } from "~/components/ui/button"
import {
  ApiError,
  attachTag,
  detachTag,
  type TagRef,
  type WatchlistEntry,
} from "~/lib/api"
import { MAX_TAG_LENGTH, MAX_TAGS_PER_ARTIST } from "~/lib/limits"
import { useTagVocabulary } from "~/lib/useTagVocabulary"

import { TagCombobox } from "./TagCombobox"

// TagActions is the route-level functional-updater pair TagChips drives;
// each touches only its own tag so concurrent changes don't clobber (D-24).
export interface TagActions {
  addTag(entryId: number, tag: TagRef, index?: number): void
  removeTag(entryId: number, tagId: number): void
}

// pendingIdCounter backs the temp ids of optimistic chips (D-24).
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

// FocusRequest is a pending "move focus once the DOM catches up" request.
// "chip"/"grow" wait for entry.tags to change; "add-button" for "+ tag".
type FocusRequest =
  | { kind: "chip"; index: number }
  | { kind: "grow" }
  | { kind: "add-button" }
  | null

// TagChips renders entry.tags as Badge chips (D-01). A tag name is plain JSX
// text only, never raw HTML, so an HTML-looking name renders literally.
export function TagChips({ entry, actions, announce }: TagChipsProps) {
  const { vocabulary, ensureLoaded, remember } = useTagVocabulary()
  const containerRef = useRef<HTMLDivElement>(null)
  const addButtonRef = useRef<HTMLButtonElement>(null)
  const focusRequestRef = useRef<FocusRequest>(null)
  const prevTagCountRef = useRef(entry.tags.length)
  // Optimistic chips live here, not in entry.tags, so a concurrent
  // refresh() can't wipe them (D-24).
  const [pending, setPending] = useState<PendingTag[]>([])
  const [editorOpen, setEditorOpen] = useState(false)

  const count = entry.tags.length + pending.length
  const atCap = count >= MAX_TAGS_PER_ARTIST

  // Chip removal/grow focus: entry.tags says which chip's × comes next once
  // the DOM has shrunk or grown to match.
  useLayoutEffect(() => {
    const changed = entry.tags.length !== prevTagCountRef.current
    prevTagCountRef.current = entry.tags.length
    const req = focusRequestRef.current
    if (!changed || !req || req.kind === "add-button") return
    focusRequestRef.current = null
    const index = req.kind === "grow" ? entry.tags.length - 1 : req.index
    const buttons =
      containerRef.current?.querySelectorAll<HTMLButtonElement>("button")
    buttons?.[index]?.focus()
  }, [entry.tags])

  // "+ tag" focus: fires once the trailing slot actually re-renders as the
  // button again -- after the editor closes (Esc) or a 10th-pick attach
  // fails and the cap hint reverts (UI-SPEC focus table row a).
  useLayoutEffect(() => {
    const req = focusRequestRef.current
    if (req?.kind !== "add-button" || !addButtonRef.current) return
    focusRequestRef.current = null
    addButtonRef.current.focus()
  }, [pending, editorOpen, entry.tags])

  function handleRemove(tag: TagRef, index: number) {
    const hadTen = entry.tags.length === MAX_TAGS_PER_ARTIST
    const newLength = entry.tags.length - 1
    focusRequestRef.current =
      newLength === 0
        ? { kind: "add-button" }
        : { kind: "chip", index: Math.min(index, newLength - 1) }

    actions.removeTag(entry.id, tag.id)
    announce(
      hadTen
        ? `Removed “${tag.name}” from ${entry.name}. You can add tags again.`
        : `Removed “${tag.name}” from ${entry.name}.`
    )

    detachTag(entry.id, tag.id).catch(() => {
      focusRequestRef.current = null
      actions.addTag(entry.id, tag, index)
      toast.error(
        `Couldn't remove “${tag.name}” from ${entry.name} — try again.`
      )
    })
  }

  function handleOpen() {
    ensureLoaded()
    setEditorOpen(true)
  }

  // attachErrorMessage maps a rejected attach to its exact UI-SPEC Toasts
  // copy by API error code; anything else gets the generic one.
  function attachErrorMessage(err: unknown, name: string): string {
    if (err instanceof ApiError) {
      if (err.code === "tag_cap_reached") {
        return `${entry.name} already has ${MAX_TAGS_PER_ARTIST} tags — remove one first.`
      }
      if (err.code === "tag_name_too_long") {
        return `Tags can be at most ${MAX_TAG_LENGTH} characters.`
      }
    }
    return `Couldn't add “${name}” to ${entry.name} — try again.`
  }

  // The pending chip shows the typed casing until the real chip replaces it
  // with the server's casing (TAG-03). A pick reaching the cap closes the
  // editor and focuses the new chip's ×, or "+ tag" if the attach fails.
  function handleCommit(name: string) {
    const reachesCap = count + 1 >= MAX_TAGS_PER_ARTIST
    const tempId = nextPendingId()
    setPending((p) => [...p, { tempId, name }])

    if (reachesCap) {
      setEditorOpen(false)
      focusRequestRef.current = { kind: "grow" }
    }

    attachTag(entry.id, name)
      .then((tag) => {
        setPending((p) => p.filter((item) => item.tempId !== tempId))
        actions.addTag(entry.id, tag)
        remember(tag)
        announce(
          reachesCap
            ? `Added “${tag.name}” to ${entry.name}. Max ${MAX_TAGS_PER_ARTIST} tags reached.`
            : `Added “${tag.name}” to ${entry.name}.`
        )
      })
      .catch((err: unknown) => {
        setPending((p) => p.filter((item) => item.tempId !== tempId))
        if (reachesCap) {
          focusRequestRef.current = { kind: "add-button" }
        }
        toast.error(attachErrorMessage(err, name))
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
          {/* aria-disabled, not disabled, keeps the chip focusable for the
              cap-pick focus move. */}
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
          vocabulary={vocabulary}
          onArtist={entry.tags}
          pendingNames={pending.map((p) => p.name)}
          onCommit={handleCommit}
          onClose={(reason) => {
            setEditorOpen(false)
            // A blur-closed editor leaves focus wherever the user already
            // moved it -- only Esc explicitly returns focus to "+ tag"
            // (UI-SPEC focus table row a).
            if (reason === "escape") {
              focusRequestRef.current = { kind: "add-button" }
            }
          }}
        />
      ) : atCap ? (
        <span className="text-label text-muted-foreground">
          max {MAX_TAGS_PER_ARTIST} tags
        </span>
      ) : (
        <Button
          ref={addButtonRef}
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
