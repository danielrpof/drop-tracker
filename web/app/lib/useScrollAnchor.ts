import { useCallback, useEffectEvent, useLayoutEffect, useRef } from "react"

// Chip first, then every sibling card nearest-first (next before previous at
// each distance): fallbacks if the clicked card leaves the filtered list (G-25-11).
export function anchorCandidates(el: HTMLElement): HTMLElement[] {
  const card = el.closest("li")
  const parent = card?.parentElement
  if (!card || !parent) return [el]
  const cards = Array.from(parent.children).filter(
    (c): c is HTMLElement => c instanceof HTMLElement
  )
  const at = cards.indexOf(card)
  const out = [el]
  for (let d = 1; at + d < cards.length || at - d >= 0; d++) {
    if (at + d < cards.length) out.push(cards[at + d])
    if (at - d >= 0) out.push(cards[at - d])
  }
  return out
}

// G-25-4: a filter can collapse the page under the viewport; keep the clicked
// element, or its nearest surviving neighbour, where it was on screen.
// Consumed by the next `key` change, before paint. G-25-12: if a keyboard
// capture's focused element left, onFocusLost gets the surviving card (or null).
export function useScrollAnchor(
  key: string,
  onFocusLost?: (fallback: HTMLElement | null) => void
): (el: HTMLElement, viaKeyboard?: boolean) => void {
  const anchor = useRef<{
    el: HTMLElement
    candidates: { el: HTMLElement; top: number }[]
    hadFocus: boolean
  } | null>(null)
  const reportFocusLost = useEffectEvent((fallback: HTMLElement | null) =>
    onFocusLost?.(fallback)
  )

  const capture = useCallback((el: HTMLElement, viaKeyboard = false) => {
    anchor.current = {
      el,
      candidates: anchorCandidates(el).map((c) => ({
        el: c,
        top: c.getBoundingClientRect().top,
      })),
      hadFocus: viaKeyboard && el.contains(document.activeElement),
    }
  }, [])

  useLayoutEffect(() => {
    const captured = anchor.current
    anchor.current = null
    if (!captured) return
    const a = captured.candidates.find((c) => c.el.isConnected)
    if (a) {
      const delta = a.el.getBoundingClientRect().top - a.top
      if (delta !== 0) window.scrollBy(0, delta)
    }
    const active = document.activeElement
    if (
      captured.hadFocus &&
      !captured.el.isConnected &&
      (!active || active === document.body)
    ) {
      reportFocusLost(a?.el ?? null)
    }
  }, [key])

  return capture
}
