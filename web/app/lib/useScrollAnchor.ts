import { useCallback, useLayoutEffect, useRef } from "react"

// Chip first, then the card's next and previous siblings: a fallback when the
// clicked card itself leaves the filtered list (WR-02).
export function anchorCandidates(el: HTMLElement): HTMLElement[] {
  const card = el.closest("li")
  return [el, card?.nextElementSibling, card?.previousElementSibling].filter(
    (c): c is HTMLElement => c instanceof HTMLElement
  )
}

// G-25-4: a filter can collapse the page under the viewport; keep the clicked
// element, or its nearest surviving neighbour, where it was on screen.
// Consumed by the next `key` change, before paint.
export function useScrollAnchor(key: string): (el: HTMLElement) => void {
  const anchor = useRef<{ el: HTMLElement; top: number }[] | null>(null)

  const capture = useCallback((el: HTMLElement) => {
    anchor.current = anchorCandidates(el).map((c) => ({
      el: c,
      top: c.getBoundingClientRect().top,
    }))
  }, [])

  useLayoutEffect(() => {
    const candidates = anchor.current
    anchor.current = null
    const a = candidates?.find((c) => c.el.isConnected)
    if (!a) return
    const delta = a.el.getBoundingClientRect().top - a.top
    if (delta !== 0) window.scrollBy(0, delta)
  }, [key])

  return capture
}
