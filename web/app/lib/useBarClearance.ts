import { useLayoutEffect, type RefObject } from "react"

// Publishes the bar's height as --bulk-bar-clearance so the page padding and
// toasts stay clear of it (see root.tsx).
export function useBarClearance(ref: RefObject<HTMLElement | null>) {
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const root = document.documentElement
    const measure = () => {
      const inset = parseFloat(getComputedStyle(el).bottom)
      const bottom = Number.isFinite(inset) ? inset : 0
      const height = el.getBoundingClientRect().height
      root.style.setProperty("--bulk-bar-clearance", `${height + bottom + 8}px`)
    }
    measure()
    const observer =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(measure)
    observer?.observe(el)
    return () => {
      observer?.disconnect()
      root.style.removeProperty("--bulk-bar-clearance")
    }
  }, [ref])
}
