import { describe, expect, it } from "vitest"

import type { TagRef } from "~/lib/api"

import { buildTagSuggestions, normalizeTagKey } from "./tags"

function refs(names: string[]): TagRef[] {
  return names.map((name, i) => ({ id: i + 1, name }))
}

describe("normalizeTagKey", () => {
  it("trims and collapses interior whitespace runs to one space", () => {
    expect(normalizeTagKey("  Hip   Hop ")).toBe("hip hop")
  })

  it("treats an NFC-decomposed name as identical to its precomposed form (D-31)", () => {
    // "REGGAETÓN" here is precomposed (Ó = U+00D3); the decomposed form
    // below spells Ó as O (U+004F) + COMBINING ACUTE ACCENT (U+0301) --
    // .normalize("NFC") must fold both to the same key.
    const decomposed = "REGGAETÓN"
    expect(normalizeTagKey(decomposed)).toBe(normalizeTagKey("reggaetón"))
  })
})

describe("buildTagSuggestions", () => {
  const vocabulary = refs(["reggaeton", "reggae", "latin"])

  it("pins the exact match first with no Create item when the query resolves to an existing tag, even with trailing whitespace", () => {
    const result = buildTagSuggestions({
      query: "Reggaeton ",
      vocabulary,
      onArtist: [],
      pendingNames: [],
    })

    expect(result).toEqual({
      state: "items",
      items: [{ kind: "existing", tag: vocabulary[0] }],
    })
  })

  it("pins Create first, then contains-matches sorted by name, when the query matches nothing exactly", () => {
    const result = buildTagSuggestions({
      query: "reg",
      vocabulary,
      onArtist: [],
      pendingNames: [],
    })

    expect(result).toEqual({
      state: "items",
      items: [
        { kind: "create", name: "reg" },
        { kind: "existing", tag: vocabulary[1] }, // reggae
        { kind: "existing", tag: vocabulary[0] }, // reggaeton
      ],
    })
  })

  it("returns the already-on state, in the tag's stored casing, when the query matches a tag already on the artist", () => {
    const onArtist = refs(["latin"])
    const result = buildTagSuggestions({
      query: "LATIN",
      vocabulary,
      onArtist,
      pendingNames: [],
    })

    expect(result).toEqual({ state: "already-on", name: "latin" })
  })

  it("returns the already-on state when the query matches a pending chip's name", () => {
    const result = buildTagSuggestions({
      query: "drill",
      vocabulary,
      onArtist: [],
      pendingNames: ["drill"],
    })

    expect(result).toEqual({ state: "already-on", name: "drill" })
  })

  it("returns the empty-vocabulary state for an empty, loaded vocabulary and an empty query", () => {
    const result = buildTagSuggestions({
      query: "",
      vocabulary: [],
      onArtist: [],
      pendingNames: [],
    })

    expect(result).toEqual({ state: "empty-vocabulary" })
  })

  it("offers only Create for a non-empty query when the vocabulary has not loaded (null)", () => {
    const result = buildTagSuggestions({
      query: "x",
      vocabulary: null,
      onArtist: [],
      pendingNames: [],
    })

    expect(result).toEqual({
      state: "items",
      items: [{ kind: "create", name: "x" }],
    })
  })

  it("offers no items for an empty query when the vocabulary has not loaded (null)", () => {
    const result = buildTagSuggestions({
      query: "",
      vocabulary: null,
      onArtist: [],
      pendingNames: [],
    })

    expect(result).toEqual({ state: "items", items: [] })
  })

  it("never suggests a tag already on the artist, even as a partial match", () => {
    const onArtist = refs(["reggae"])
    const result = buildTagSuggestions({
      query: "reg",
      vocabulary,
      onArtist,
      pendingNames: [],
    })

    expect(result).toEqual({
      state: "items",
      items: [
        { kind: "create", name: "reg" },
        { kind: "existing", tag: vocabulary[0] }, // reggaeton only
      ],
    })
  })
})
