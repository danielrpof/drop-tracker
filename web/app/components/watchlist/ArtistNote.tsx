import { useLayoutEffect, useRef, useState } from "react"

import { NotebookPen, Pencil } from "lucide-react"

import { Button } from "~/components/ui/button"
import { Textarea } from "~/components/ui/textarea"
import { ApiError, type WatchlistEntry, updateNote } from "~/lib/api"

export interface ArtistNoteProps {
  entry: WatchlistEntry
  onEntryChange: (id: number, patch: Partial<WatchlistEntry>) => void
  announce: (message: string) => void
}

// FocusRequest is a single pending "move focus once the editor closes"
// instruction, consumed by the layout effect below (mirrors TagChips'
// FocusRequest pattern). hasNote decides pencil vs "add note": on
// cancel/Esc it comes from entry.note (nothing changed), on a successful
// save it comes from the fresh server response -- not from the entry prop,
// which the parent may not have re-rendered with yet by the time this
// effect runs.
type FocusRequest = { hasNote: boolean } | null

function saveErrorMessage(err: unknown): string {
  if (
    err instanceof ApiError &&
    err.status === 400 &&
    err.message === "note must be at most 500 characters"
  ) {
    return "Notes can be at most 500 characters."
  }
  return "Couldn't save the note — your text is still here. Try again."
}

// ArtistNote renders the D-05/D-06 note block under the tag chip row: a
// clamped display with an edit pencil when a note exists, an "add note"
// trigger when it doesn't, and an in-place editor that saves through the
// dedicated PUT /watchlist/{id}/note endpoint -- never PATCH (D-25), since
// clearing a note is an explicit request, not the "leave this axis
// untouched" an absent PATCH key means. Note text renders as a plain JSX
// text node only (T-24-37) -- never raw HTML.
export function ArtistNote({
  entry,
  onEntryChange,
  announce,
}: ArtistNoteProps) {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(entry.note ?? "")
  const [error, setError] = useState("")
  const [saving, setSaving] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const [overflow, setOverflow] = useState(false)
  const [counterMessage, setCounterMessage] = useState("")

  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const pencilRef = useRef<HTMLButtonElement>(null)
  const addNoteRef = useRef<HTMLButtonElement>(null)
  const noteRef = useRef<HTMLParagraphElement>(null)
  const focusRequestRef = useRef<FocusRequest>(null)
  const prevLengthRef = useRef((entry.note ?? "").length)

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

  // Consumes a pending focus move once the editor has actually closed and
  // the pencil/"add note" trigger it targets exists in the DOM again
  // (UI-SPEC focus table row (c)).
  useLayoutEffect(() => {
    if (editing) return
    const req = focusRequestRef.current
    if (!req) return
    focusRequestRef.current = null
    const target = req.hasNote ? pencilRef.current : addNoteRef.current
    target?.focus()
  }, [editing])

  // Overflow is measured on the clamped paragraph only (G-24-9): while
  // expanded the last result is kept so "less" stays mounted. jsdom has no
  // layout, so tests stub the measurement.
  useLayoutEffect(() => {
    if (editing || entry.note === null) {
      setOverflow(false)
      return
    }
    if (expanded) return
    const el = noteRef.current
    if (!el) return
    const measure = () => setOverflow(el.scrollHeight > el.clientHeight)
    measure()
    if (typeof ResizeObserver === "undefined") return
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [editing, entry.note, expanded])

  function openEditor() {
    setValue(entry.note ?? "")
    setError("")
    setExpanded(false)
    prevLengthRef.current = (entry.note ?? "").length
    setEditing(true)
  }

  function closeEditor() {
    if (saving) return
    focusRequestRef.current = { hasNote: entry.note !== null }
    setEditing(false)
    setError("")
  }

  function handleChange(next: string) {
    const prevLen = prevLengthRef.current
    prevLengthRef.current = next.length
    setValue(next)
    if (next.length === 500 && prevLen !== 500) {
      setCounterMessage("Note limit reached — 500 characters.")
    } else if (prevLen < 450 && next.length >= 450) {
      setCounterMessage("50 characters left.")
    }
  }

  const trimmed = value.trim()
  const currentNormalized = entry.note ?? ""
  const saveDisabled = trimmed === currentNormalized

  async function handleSave() {
    if (saveDisabled || saving) return
    const next = trimmed === "" ? null : trimmed
    setSaving(true)
    setError("")
    try {
      const updated = await updateNote(entry.id, next)
      focusRequestRef.current = { hasNote: !!updated.note }
      onEntryChange(entry.id, { note: updated.note })
      announce(
        updated.note
          ? `Note saved for ${entry.name}.`
          : `Note cleared for ${entry.name}.`
      )
      setEditing(false)
      setError("")
    } catch (err) {
      setError(saveErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  if (editing) {
    const counterId = `note-counter-${entry.id}`
    return (
      <div className="mt-1 flex flex-col gap-1">
        <Textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => handleChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              if (saving) return
              e.preventDefault()
              closeEditor()
            } else if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
              e.preventDefault()
              void handleSave()
            }
          }}
          maxLength={500}
          readOnly={saving}
          aria-busy={saving}
          aria-describedby={counterId}
          className="max-h-40 overflow-y-auto md:text-label"
          aria-label={`Note for ${entry.name}`}
          placeholder="Add a note about this artist"
        />
        <div className="flex items-center justify-between gap-2">
          <span
            id={counterId}
            className={`text-label tabular-nums ${
              value.length >= 450 ? "text-foreground" : "text-muted-foreground"
            }`}
          >
            {value.length}/500
          </span>
          <span aria-live="polite" className="sr-only">
            {counterMessage}
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              disabled={saving}
              onClick={closeEditor}
            >
              Cancel
            </Button>
            <Button
              variant="default"
              size="sm"
              className="min-w-20"
              disabled={saveDisabled || saving}
              onClick={() => void handleSave()}
            >
              {saving ? "Saving…" : "Save"}
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
          ref={addNoteRef}
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
          ref={noteRef}
          id={`note-${entry.id}`}
          className={`text-label break-words whitespace-pre-line text-muted-foreground ${
            expanded ? "" : "line-clamp-2"
          }`}
        >
          {entry.note}
        </p>
        {overflow && (
          <button
            type="button"
            className="rounded-sm text-label text-muted-foreground underline underline-offset-4 hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50"
            aria-expanded={expanded}
            aria-controls={`note-${entry.id}`}
            onClick={() => setExpanded((e) => !e)}
          >
            {expanded ? "less" : "more"}
          </button>
        )}
      </div>
      <Button
        ref={pencilRef}
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
