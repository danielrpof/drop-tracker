import { describe, expect, it } from "vitest"

import { canonicalHistoryParams, parseHistoryParams } from "~/lib/historyParams"

const parse = (qs: string) => parseHistoryParams(new URLSearchParams(qs))

describe("parseHistoryParams", () => {
  it("parses valid artist, type, and tag", () => {
    expect(parse("artist=12&type=new_release&tag=3")).toEqual({
      artistId: 12,
      eventType: "new_release",
      tagId: 3,
    })
  })

  it.each([
    ["artist=abc", "artistId"],
    ["artist=0", "artistId"],
    ["artist=-1", "artistId"],
    ["artist=1.5", "artistId"],
    ["tag=-1", "tagId"],
    ["tag=0", "tagId"],
    ["tag=007", "tagId"],
    ["type=bogus", "eventType"],
    ["type=", "eventType"],
  ] as const)("treats %s as null", (qs, key) => {
    expect(parse(qs)[key]).toBeNull()
  })

  it("accepts each allow-listed event type", () => {
    for (const t of ["new_release", "guest_feature", "deluxe_change"]) {
      expect(parse(`type=${t}`).eventType).toBe(t)
    }
  })
})

describe("canonicalHistoryParams", () => {
  it("drops exactly the invalid keys and keeps foreign keys", () => {
    const next = canonicalHistoryParams(
      new URLSearchParams("artist=abc&type=guest_feature&tag=0&q=drake")
    )
    expect(next.toString()).toBe("type=guest_feature&q=drake")
  })

  it("leaves an already-canonical query unchanged", () => {
    const qs = "artist=12&type=new_release&tag=3"
    expect(canonicalHistoryParams(new URLSearchParams(qs)).toString()).toBe(qs)
  })
})
