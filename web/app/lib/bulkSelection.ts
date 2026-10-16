import type { WatchlistEntry } from "~/lib/api"

// Pure selection helpers: type-only imports, because route tests automock ~/lib/api.

export type SelectAllState = "checked" | "indeterminate" | "unchecked"

// selectedIds is never trimmed; hidden selections stay inert here (D-11).
export function effectiveSelection(
  visible: WatchlistEntry[],
  selected: ReadonlySet<number>
): WatchlistEntry[] {
  return visible.filter((e) => selected.has(e.id))
}

export function selectAllState(
  visibleCount: number,
  effectiveCount: number
): SelectAllState {
  if (visibleCount === 0 || effectiveCount === 0) return "unchecked"
  return effectiveCount >= visibleCount ? "checked" : "indeterminate"
}

// Unchecking drops only the visible ids; hidden selections survive (grill Q9).
export function toggleAllVisible(
  visible: WatchlistEntry[],
  selected: ReadonlySet<number>
): Set<number> {
  const out = new Set(selected)
  const allSelected =
    visible.length > 0 && visible.every((e) => selected.has(e.id))
  for (const e of visible) {
    if (allSelected) out.delete(e.id)
    else out.add(e.id)
  }
  return out
}
