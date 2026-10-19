import type { BulkAxisDelta, TagRef, WatchlistEntry } from "~/lib/api"
import { sortTagsByName } from "~/lib/tags"

// Pure helpers; type-only api imports because route tests automock ~/lib/api.

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

// Sets the inclusive anchor..clicked span, in visible order, to `next`. Without
// a usable anchor it toggles only the clicked id. Spans only `order` (T-26-45).
export function applyRange(
  order: readonly number[],
  anchor: number | null,
  clicked: number,
  next: boolean,
  selected: ReadonlySet<number>
): Set<number> {
  const out = new Set(selected)
  const from = anchor === null ? -1 : order.indexOf(anchor)
  const to = order.indexOf(clicked)
  const ids =
    from === -1 || to === -1
      ? [clicked]
      : order.slice(Math.min(from, to), Math.max(from, to) + 1)
  for (const id of ids) {
    if (next) out.add(id)
    else out.delete(id)
  }
  return out
}

// Tags carried by every selected entry, in the first entry's order.
export function tagsOnEvery(selected: readonly WatchlistEntry[]): TagRef[] {
  if (selected.length === 0) return []
  return selected[0].tags.filter((tag) =>
    selected.every((e) => e.tags.some((t) => t.id === tag.id))
  )
}

export interface RemovableTag {
  id: number
  name: string
  count: number
}

// Union of tags carried by at least one selected entry, with carrier counts.
export function removableTags(
  selected: readonly WatchlistEntry[]
): RemovableTag[] {
  const byId = new Map<number, RemovableTag>()
  for (const entry of selected) {
    for (const tag of entry.tags) {
      const found = byId.get(tag.id)
      if (found) {
        found.count += 1
        found.name = tag.name
      } else {
        byId.set(tag.id, { id: tag.id, name: tag.name, count: 1 })
      }
    }
  }
  return sortTagsByName([...byId.values()])
}

export type TriState = "checked" | "unchecked" | "indeterminate"

export type PreferenceAxis = "release_types" | "muted_event_types"

// Checked when every entry has the type, unchecked when none does (D-06).
export function initialTriStates(
  selected: readonly WatchlistEntry[],
  axis: PreferenceAxis,
  types: readonly string[]
): Record<string, TriState> {
  const out: Record<string, TriState> = {}
  for (const type of types) {
    const on = selected.filter((e) => e[axis].includes(type)).length
    out[type] =
      on === 0
        ? "unchecked"
        : on === selected.length
          ? "checked"
          : "indeterminate"
  }
  return out
}

// A mixed start cycles through all three states; a uniform start toggles.
export function cycleTriState(initial: TriState, current: TriState): TriState {
  if (initial === "indeterminate") {
    if (current === "indeterminate") return "checked"
    return current === "checked" ? "unchecked" : "indeterminate"
  }
  return current === "checked" ? "unchecked" : "checked"
}

// Only types that moved off their initial state are sent, in canonical order.
export function buildAxisDelta(
  types: readonly string[],
  initial: Record<string, TriState>,
  current: Record<string, TriState>
): BulkAxisDelta {
  const delta: BulkAxisDelta = { set: [], unset: [] }
  for (const type of types) {
    if (current[type] === initial[type]) continue
    if (current[type] === "checked") delta.set.push(type)
    else if (current[type] === "unchecked") delta.unset.push(type)
  }
  return delta
}
