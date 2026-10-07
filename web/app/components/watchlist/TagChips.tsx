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

import { TagCombobox } from "./TagCombobox"

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

// FocusRequest is a single pending "move focus once the DOM catches up"
// instruction (UI-SPEC focus table rows a/b), consumed by the two layout
// effects below. "chip"/"grow" depend on entry.tags actually changing;
// "add-button" depends on the trailing slot re-rendering as "+ tag".
type FocusRequest =
  | { kind: "chip"; index: number }
  | { kind: "grow" }
  | { kind: "add-button" }
  | null

// TagChips renders entry.tags as neutral Badge chips under the artist name
// (D-01, D-04, UI-SPEC [R1]). A tag name is plain JSX text only -- never
// raw HTML -- so an HTML-looking name renders literally (Phase 06 XSS
// posture). The row always renders -- it holds at least the "+ tag"
// trigger, never an empty gap (D-02).
export function TagChips({ entry, actions, announce }: TagChipsProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const addButtonRef = useRef<HTMLButtonElement>(null)
  const focusRequestRef = useRef<FocusRequest>(null)
  const prevTagCountRef = useRef(entry.tags.length)
  // Optimistic chips keyed by a temp id (D-24): survive a concurrent
  // refresh() because they live here, not in entry.tags.
  const [pending, setPending] = useState<PendingTag[]>([])
  const [editorOpen, setEditorOpen] = useState(false)

  const count = entry.tags.length + pending.length
  const atCap = count >= MAX_TAGS_PER_ARTIST

  // Chip removal/grow focus: entry.tags is the source of truth for "which
  // chip's × comes next" once the DOM has actually shrunk or grown to
  // match (UI-SPEC focus table rows a/b).
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
    actions.loadVocabulary()
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

  // handleCommit is TagCombobox's onCommit (both a picked existing tag and
  // a typed new name land here -- the combobox already resolved which).
  // The pending chip shows the typed casing immediately; on success it is
  // replaced by the real chip carrying the server's stored casing (TAG-03).
  // The pick that reaches the cap closes the editor immediately (the
  // pending item already counts toward it) and focuses the new chip's ×
  // once it lands, or "+ tag" again if the attach fails.
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
        actions.rememberTag(tag)
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
