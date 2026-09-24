import { useLayoutEffect, useRef, useState } from "react"

import { NotebookPen, Pencil } from "lucide-react"

import { Button } from "~/components/ui/button"
import { Textarea } from "~/components/ui/textarea"
import { type WatchlistEntry, updateNote } from "~/lib/api"

export interface ArtistNoteProps {
  entry: WatchlistEntry
  onEntryChange: (id: number, patch: Partial<WatchlistEntry>) => void
  announce: (message: string) => void
}

// ArtistNote renders the D-05/D-06 note block under the tag chip row: a
// clamped display with an edit pencil when a note exists, an "add note"
// trigger when it doesn't, and an in-place editor that saves through the
// dedicated PUT /watchlist/{id}/note endpoint -- never PATCH (D-25), since
// clearing a note is an explicit request, not the "leave this axis
// untouched" an absent PATCH key means. Note text is plain JSX text only
// (T-24-37), never dangerouslySetInnerHTML.
export function ArtistNote({
  entry,
  onEntryChange,
  announce,
}: ArtistNoteProps) {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(entry.note ?? "")
  const [error, setError] = useState("")
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Opens with focus in the textarea and the caret at the end (UI-SPEC
  // focus table row (c)).
  useLayoutEffect(() => {
    if (!editing) return
    const el = textareaRef.current
    if (!el) return
    el.focus()
    const end = el.value.length
    el.setSelectionRange(end, end)
  }, [editing])

  function openEditor() {
    setValue(entry.note ?? "")
    setError("")
    setEditing(true)
  }

  function closeEditor() {
    setEditing(false)
    setError("")
  }

  const trimmed = value.trim()
  const currentNormalized = entry.note ?? ""
  const saveDisabled = trimmed === currentNormalized

  async function handleSave() {
    if (saveDisabled) return
    const next = trimmed === "" ? null : trimmed
    try {
      const updated = await updateNote(entry.id, next)
      onEntryChange(entry.id, { note: updated.note })
      setEditing(false)
      setError("")
    } catch {
      setError("Couldn't save the note — your text is still here. Try again.")
    }
  }

  if (editing) {
    return (
      <div className="mt-1 flex flex-col gap-1">
        <Textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.preventDefault()
              closeEditor()
            }
          }}
          maxLength={500}
          className="max-h-40 overflow-y-auto text-label md:text-label"
          aria-label={`Note for ${entry.name}`}
          placeholder="Add a note about this artist"
        />
        <div className="flex items-center justify-between gap-2">
          <span
            className={`text-label tabular-nums ${
              value.length >= 450 ? "text-foreground" : "text-muted-foreground"
            }`}
          >
            {value.length}/500
          </span>
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={closeEditor}>
              Cancel
            </Button>
            <Button
              variant="default"
              size="sm"
              className="min-w-20"
              disabled={saveDisabled}
              onClick={() => void handleSave()}
            >
              Save
            </Button>
          </div>
        </div>
        <p aria-live="polite" className="text-label text-destructive">
          {error}
        </p>
      </div>
    )
  }

  if (entry.note === null) {
    return (
      <div className="mt-1">
        <Button
          variant="ghost"
          size="xs"
          className="h-6 text-label text-muted-foreground hover:text-foreground"
          aria-label={`Add note for ${entry.name}`}
          onClick={openEditor}
        >
          <NotebookPen aria-hidden="true" className="size-3" />
          add note
        </Button>
      </div>
    )
  }

  return (
    <div className="mt-1 flex items-start gap-1">
      <div className="min-w-0 flex-1">
        <p
          id={`note-${entry.id}`}
          className="line-clamp-2 text-label break-words whitespace-pre-line text-muted-foreground"
        >
          {entry.note}
        </p>
      </div>
      <Button
        variant="ghost"
        size="icon-xs"
        aria-label={`Edit note for ${entry.name}`}
        onClick={openEditor}
      >
        <Pencil aria-hidden="true" className="size-3" />
      </Button>
    </div>
  )
}
