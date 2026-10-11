import { useCallback, useLayoutEffect, useRef } from "react"

// G-25-4: a filter can collapse the page under the viewport; keep the clicked
// element where it was on screen. Consumed by the next `key` change, before paint.
export function useScrollAnchor(key: string): (el: HTMLElement) => void {
  const anchor = useRef<{ el: HTMLElement; top: number } | null>(null)

  const capture = useCallback((el: HTMLElement) => {
    anchor.current = { el, top: el.getBoundingClientRect().top }
  }, [])

  useLayoutEffect(() => {
    const a = anchor.current
    anchor.current = null
    if (!a || !a.el.isConnected) return
    const delta = a.el.getBoundingClientRect().top - a.top
    if (delta !== 0) window.scrollBy(0, delta)
  }, [key])

  return capture
}
