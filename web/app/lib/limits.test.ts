import { describe, expect, it } from "vitest"

import { charCount, noteLength, tagNameLength } from "./limits"

describe("charCount", () => {
  it("counts code points, not UTF-16 units", () => {
    const s = "🎵".repeat(32)
    expect(s.length).toBe(64)
    expect(charCount(s)).toBe(32)
  })
})

describe("tagNameLength", () => {
  it("counts NFC-normalized code points", () => {
    expect(tagNameLength("é".repeat(32))).toBe(32)
  })

  it("trims and collapses internal whitespace", () => {
    expect(tagNameLength("  a   b  ")).toBe(3)
  })
})

describe("noteLength", () => {
  it("trims outer whitespace", () => {
    expect(noteLength("  x  ")).toBe(1)
  })

  it("counts emoji as one each", () => {
    expect(noteLength("🎵".repeat(500))).toBe(500)
  })

  it("does not NFC-normalize, matching the server", () => {
    expect(noteLength("é".repeat(3))).toBe(6)
  })
})
