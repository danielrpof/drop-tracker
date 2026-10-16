import { Loader2, TagPlus, Trash2, X } from "lucide-react"
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type RefObject,
} from "react"

import { Button } from "~/components/ui/button"
import { Separator } from "~/components/ui/separator"
import { TagCombobox } from "~/components/watchlist/TagCombobox"
import type { BulkAddTagOutcome, WatchlistEntry } from "~/lib/api"
import { tagsOnEvery } from "~/lib/bulkSelection"
import { useTagVocabulary } from "~/lib/useTagVocabulary"

export interface BulkActionBarProps {
  selected: WatchlistEntry[]
  inFlight: boolean
  onRemove: () => void
  onAddTag: (name: string) => Promise<BulkAddTagOutcome | null>
  onSucceeded: () => void
  onClear: () => void
  removeButtonRef: RefObject<HTMLButtonElement | null>
}

const ACTION_CLASS = "size-11 px-0 sm:h-9 sm:w-auto sm:px-3"

// Sticky selection bar (D-10). It only opens the remove dialog; the route
// hosts it so the dialog outlives the bar's unmount (U1).
export function BulkActionBar({
  selected,
  inFlight,
  onRemove,
  onAddTag,
  onSucceeded,
  onClear,
  removeButtonRef,
}: BulkActionBarProps) {
  const wrapperRef = useRef<HTMLDivElement>(null)
  const addTagButtonRef = useRef<HTMLButtonElement>(null)
  const focusAddTagRef = useRef(false)
  const pendingRef = useRef(false)
  const [mode, setMode] = useState<"idle" | "add-tag">("idle")
  const [pending, setPending] = useState(false)
  // The bar renders inside TagVocabularyProvider, so the vocabulary lives here (D-12).
  const { vocabulary, ensureLoaded, remember } = useTagVocabulary()
  const count = selected.length

  // The Add tag button remounts when the mode returns to idle, so focus after commit.
  useEffect(() => {
    if (mode === "idle" && focusAddTagRef.current) {
      focusAddTagRef.current = false
      addTagButtonRef.current?.focus()
    }
  }, [mode])

  function exitAddTag(focusButton: boolean) {
    focusAddTagRef.current = focusButton
    setMode("idle")
  }

  function openAddTag() {
    ensureLoaded()
    setMode("add-tag")
  }

  async function commitAddTag(name: string) {
    if (pendingRef.current) return
    pendingRef.current = true
    setPending(true)
    const outcome = await onAddTag(name)
    pendingRef.current = false
    setPending(false)
    if (!outcome) {
      exitAddTag(true)
      return
    }
    if (outcome.applied.length > 0) remember(outcome.tag)
    exitAddTag(false)
    onSucceeded()
  }

  // Publishes the bar's height so the page, toasts and scroll padding clear it (U2).
  useLayoutEffect(() => {
    const wrapper = wrapperRef.current
    if (!wrapper) return
    const root = document.documentElement
    const measure = () => {
      const inset = parseFloat(getComputedStyle(wrapper).bottom)
      const bottom = Number.isFinite(inset) ? inset : 0
      const height = wrapper.getBoundingClientRect().height
      root.style.setProperty("--bulk-bar-clearance", `${height + bottom + 8}px`)
    }
    measure()
    const observer =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(measure)
    observer?.observe(wrapper)
    return () => {
      observer?.disconnect()
      root.style.removeProperty("--bulk-bar-clearance")
    }
  }, [])

  return (
    <div
      ref={wrapperRef}
      className="pointer-events-none fixed inset-x-0 bottom-2 z-40 flex justify-center px-2 sm:bottom-6 sm:px-8"
    >
      <div
        role="region"
        aria-label="Selection actions"
        // Esc in an inline picker closes only that layer (26-10 extends this).
        onKeyDown={(e) => {
          if (e.key === "Escape" && mode === "idle") onClear()
        }}
        className="pointer-events-auto flex w-full flex-wrap items-center gap-1 rounded-2xl bg-popover p-2 text-popover-foreground shadow-2xl ring-1 ring-foreground/10 motion-safe:animate-in motion-safe:duration-150 motion-safe:fade-in-0 motion-safe:slide-in-from-bottom-4 sm:w-auto sm:max-w-full sm:flex-nowrap sm:gap-2"
      >
        <p className="min-w-0 flex-1 px-0 text-label text-foreground tabular-nums sm:flex-none sm:px-2">
          {selected.length} selected
        </p>
        <Separator orientation="vertical" className="hidden h-6 sm:block" />
        {mode === "idle" ? (
          <>
            <Button
              variant="ghost"
              ref={addTagButtonRef}
              title="Add tag"
              className={ACTION_CLASS}
              disabled={inFlight}
              onClick={openAddTag}
            >
              <TagPlus aria-hidden="true" />
              <span className="sr-only sm:not-sr-only">Add tag</span>
            </Button>
            <Button
              variant="ghost"
              ref={removeButtonRef}
              title="Remove"
              className={`${ACTION_CLASS} hover:text-destructive`}
              disabled={inFlight}
              onClick={onRemove}
            >
              <Trash2 aria-hidden="true" />
              <span className="sr-only sm:not-sr-only">Remove</span>
            </Button>
          </>
        ) : (
          <div className="order-last flex basis-full items-center gap-1 sm:order-none sm:basis-auto">
            <div className="min-w-0 flex-1 sm:flex-none">
              <TagCombobox
                inputLabel={`Add tag to ${count} selected artist${count === 1 ? "" : "s"}`}
                alreadyOnMessage={(name) =>
                  `“${name}” is already on every selected artist`
                }
                vocabulary={vocabulary}
                onArtist={tagsOnEvery(selected)}
                pendingNames={[]}
                disabled={pending || inFlight}
                inputClassName="h-9 w-full max-w-none sm:w-60"
                onCommit={commitAddTag}
                onClose={(reason) => {
                  if (pendingRef.current) return
                  exitAddTag(reason === "escape")
                }}
              />
            </div>
            {pending ? (
              <span className="flex items-center gap-1 px-2 text-label text-muted-foreground">
                <Loader2
                  aria-hidden="true"
                  className="size-4 motion-safe:animate-spin"
                />
                Adding…
              </span>
            ) : (
              <Button
                variant="ghost"
                className="h-11 sm:h-9"
                disabled={inFlight}
                // Pointer-down too: the input's blur would unmount Cancel before its click.
                onPointerDown={() => exitAddTag(true)}
                onClick={() => exitAddTag(true)}
              >
                Cancel
              </Button>
            )}
          </div>
        )}
        <Separator orientation="vertical" className="hidden h-6 sm:block" />
        <Button
          variant="ghost"
          aria-label="Clear selection"
          title="Clear selection"
          className={ACTION_CLASS}
          disabled={inFlight}
          onClick={onClear}
        >
          <X aria-hidden="true" />
          <span className="sr-only sm:not-sr-only">Clear</span>
        </Button>
      </div>
    </div>
  )
}
