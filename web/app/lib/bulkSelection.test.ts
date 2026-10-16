import { describe, expect, it } from "vitest"

import {
  applyRange,
  effectiveSelection,
  selectAllState,
  tagsOnEvery,
  toggleAllVisible,
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
