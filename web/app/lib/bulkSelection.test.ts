import { describe, expect, it } from "vitest"

import {
  effectiveSelection,
  selectAllState,
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
