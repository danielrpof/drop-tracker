import { describe, expect, it } from "vitest"

import {
  identityField,
  isAddableSource,
  SOURCE_ORDER,
  sourceDisplayName,
} from "./sources"

describe("isAddableSource", () => {
  it("returns true for musicbrainz", () => {
    expect(isAddableSource("musicbrainz")).toBe(true)
  })

  it("returns false for deezer", () => {
    expect(isAddableSource("deezer")).toBe(false)
  })

  it("returns false for an unrecognised source", () => {
    expect(isAddableSource("spotify")).toBe(false)
  })
})

describe("identityField", () => {
  it("returns deezer_id for deezer", () => {
    expect(identityField("deezer")).toBe("deezer_id")
  })

  it("returns mbid for musicbrainz", () => {
    expect(identityField("musicbrainz")).toBe("mbid")
  })

  it("returns mbid for an unrecognised source", () => {
    expect(identityField("spotify")).toBe("mbid")
  })
})

describe("sourceDisplayName", () => {
  it("returns MusicBrainz for musicbrainz", () => {
    expect(sourceDisplayName("musicbrainz")).toBe("MusicBrainz")
  })

  it("returns Deezer for deezer", () => {
    expect(sourceDisplayName("deezer")).toBe("Deezer")
  })

  it("passes an unrecognised key through unchanged rather than throwing", () => {
    expect(sourceDisplayName("spotify")).toBe("spotify")
  })

  it("returns a defined string for an empty key", () => {
    expect(sourceDisplayName("")).toBe("")
  })
})

describe("SOURCE_ORDER", () => {
  it("is exactly the two known keys, MusicBrainz first", () => {
    expect(SOURCE_ORDER).toEqual(["musicbrainz", "deezer"])
  })
})
