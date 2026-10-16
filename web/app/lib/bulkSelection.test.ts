import { describe, expect, it } from "vitest"

import {
  applyRange,
  buildAxisDelta,
  cycleTriState,
  effectiveSelection,
  initialTriStates,
  selectAllState,
  tagsOnEvery,
  toggleAllVisible,
  type TriState,
} from "~/lib/bulkSelection"
import { makeEntry } from "~/lib/test/fixtures"

const visible = [
  makeEntry({ id: 3, name: "Bad Bunny" }),
  makeEntry({ id: 1, name: "Drake" }),
  makeEntry({ id: 2, name: "SZA" }),
]

describe("effectiveSelection", () => {
  it("keeps visible order and drops hidden ids", () => {
    const out = effectiveSelection(visible, new Set([2, 3, 99]))
    expect(out.map((e) => e.id)).toEqual([3, 2])
  })

  it("counts a card once however it became visible", () => {
    expect(effectiveSelection(visible, new Set([1]))).toHaveLength(1)
  })

  it("is empty for an empty selection or an empty view", () => {
    expect(effectiveSelection(visible, new Set())).toEqual([])
    expect(effectiveSelection([], new Set([1]))).toEqual([])
  })
})

describe("selectAllState", () => {
  it.each([
    [0, 0, "unchecked"],
    [3, 0, "unchecked"],
    [3, 2, "indeterminate"],
    [3, 3, "checked"],
    [1, 1, "checked"],
    [1, 0, "unchecked"],
  ] as const)("(%i, %i) is %s", (n, effective, want) => {
    expect(selectAllState(n, effective)).toBe(want)
  })
})

describe("toggleAllVisible", () => {
  it("adds every visible id when some are unselected", () => {
    const out = toggleAllVisible(visible, new Set([1, 99]))
    expect([...out].sort((a, b) => a - b)).toEqual([1, 2, 3, 99])
  })

  it("removes only visible ids when all are selected", () => {
    const out = toggleAllVisible(visible, new Set([1, 2, 3, 99]))
    expect([...out]).toEqual([99])
  })

  it("does not mutate its input", () => {
    const selected = new Set([1])
    toggleAllVisible(visible, selected)
    expect([...selected]).toEqual([1])
  })

  it("leaves the selection alone for an empty view", () => {
    expect([...toggleAllVisible([], new Set([5]))]).toEqual([5])
  })
})

describe("applyRange", () => {
  const order = [10, 20, 30, 40, 50]

  it("sets the inclusive span forward", () => {
    const out = applyRange(order, 20, 40, true, new Set([20]))
    expect([...out].sort((a, b) => a - b)).toEqual([20, 30, 40])
  })

  it("sets the inclusive span backward", () => {
    const out = applyRange(order, 40, 20, true, new Set([40]))
    expect([...out].sort((a, b) => a - b)).toEqual([20, 30, 40])
  })

  it("deselects the span when next is false", () => {
    const out = applyRange(order, 10, 30, false, new Set([10, 20, 30, 99]))
    expect([...out]).toEqual([99])
  })

  it.each([
    ["null", null],
    ["absent from order", 99],
    ["equal to the clicked id", 30],
  ])("toggles only the clicked id when the anchor is %s", (_, anchor) => {
    const out = applyRange(order, anchor, 30, true, new Set([10]))
    expect([...out].sort((a, b) => a - b)).toEqual([10, 30])
    const off = applyRange(order, anchor, 30, false, new Set([10, 30]))
    expect([...off]).toEqual([10])
  })

  it("never adds an id outside the order", () => {
    const out = applyRange(order, 10, 50, true, new Set())
    expect(out.has(99)).toBe(false)
    expect(out.size).toBe(5)
  })

  it("does not mutate its input", () => {
    const selected = new Set([10])
    applyRange(order, 10, 30, true, selected)
    expect([...selected]).toEqual([10])
  })
})

describe("tagsOnEvery", () => {
  const rap = { id: 1, name: "rap" }
  const latin = { id: 2, name: "latin" }
  const rnb = { id: 3, name: "rnb" }

  it("is empty for an empty selection", () => {
    expect(tagsOnEvery([])).toEqual([])
  })

  it("keeps only shared tags, in the first entry's order", () => {
    const out = tagsOnEvery([
      makeEntry({ id: 1, name: "A1", tags: [latin, rap] }),
      makeEntry({ id: 2, name: "A2", tags: [rap, rnb, latin] }),
    ])
    expect(out).toEqual([latin, rap])
  })

  it("is empty when nothing is shared", () => {
    expect(
      tagsOnEvery([
        makeEntry({ id: 1, name: "A1", tags: [rap] }),
        makeEntry({ id: 2, name: "A2", tags: [rnb] }),
      ])
    ).toEqual([])
  })

  it("returns every tag of a single entry", () => {
    expect(
      tagsOnEvery([makeEntry({ id: 1, name: "A1", tags: [rap, latin] })])
    ).toEqual([rap, latin])
  })
})

describe("initialTriStates", () => {
  const types = ["album", "ep", "deluxe"]

  it("is checked when all have it, unchecked when none, indeterminate when mixed", () => {
    const out = initialTriStates(
      [
        makeEntry({ id: 1, name: "A1", release_types: ["album", "ep"] }),
        makeEntry({ id: 2, name: "A2", release_types: ["album"] }),
      ],
      "release_types",
      types
    )
    expect(out).toEqual({
      album: "checked",
      ep: "indeterminate",
      deluxe: "unchecked",
    })
  })

  it("never reports indeterminate for a single artist", () => {
    const out = initialTriStates(
      [makeEntry({ id: 1, name: "A1", release_types: ["ep"] })],
      "release_types",
      types
    )
    expect(Object.values(out)).not.toContain("indeterminate")
  })

  it("reads the mute axis independently of release types", () => {
    const out = initialTriStates(
      [
        makeEntry({ id: 1, name: "A1", muted_event_types: ["guest_feature"] }),
        makeEntry({ id: 2, name: "A2" }),
      ],
      "muted_event_types",
      ["new_release", "guest_feature"]
    )
    expect(out).toEqual({
      new_release: "unchecked",
      guest_feature: "indeterminate",
    })
  })
})

describe("cycleTriState", () => {
  function walk(initial: TriState, clicks: number): TriState[] {
    const seen: TriState[] = []
    let current = initial
    for (let i = 0; i < clicks; i++) {
      current = cycleTriState(initial, current)
      seen.push(current)
    }
    return seen
  }

  it("cycles a mixed start through checked, unchecked, indeterminate", () => {
    expect(walk("indeterminate", 4)).toEqual([
      "checked",
      "unchecked",
      "indeterminate",
      "checked",
    ])
  })

  it("toggles a checked start between unchecked and checked", () => {
    expect(walk("checked", 3)).toEqual(["unchecked", "checked", "unchecked"])
  })

  it("toggles an unchecked start between checked and unchecked", () => {
    expect(walk("unchecked", 3)).toEqual(["checked", "unchecked", "checked"])
  })
})

describe("buildAxisDelta", () => {
  const types = ["album", "single", "ep", "deluxe"]
  const initial: Record<string, TriState> = {
    album: "checked",
    single: "unchecked",
    ep: "indeterminate",
    deluxe: "checked",
  }

  it("is empty when nothing changed", () => {
    expect(buildAxisDelta(types, initial, { ...initial })).toEqual({
      set: [],
      unset: [],
    })
  })

  it("emits changed types in canonical order, whatever the click order", () => {
    const current: Record<string, TriState> = {
      ...initial,
      deluxe: "unchecked",
      album: "unchecked",
    }
    expect(buildAxisDelta(types, initial, current)).toEqual({
      set: [],
      unset: ["album", "deluxe"],
    })
  })

  it("sets checked and unsets unchecked", () => {
    const current: Record<string, TriState> = {
      ...initial,
      single: "checked",
      ep: "unchecked",
    }
    expect(buildAxisDelta(types, initial, current)).toEqual({
      set: ["single"],
      unset: ["ep"],
    })
  })

  it("omits a mixed type cycled back to indeterminate", () => {
    expect(
      buildAxisDelta(types, initial, { ...initial, ep: "indeterminate" })
    ).toEqual({ set: [], unset: [] })
  })
})
