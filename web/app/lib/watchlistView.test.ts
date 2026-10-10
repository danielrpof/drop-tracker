import { describe, expect, it } from "vitest"

import { makeEntry } from "~/lib/test/fixtures"
import {
  foldName,
  formatArtistCount,
  matchesName,
  parseWatchlistParams,
  selectVisible,
} from "~/lib/watchlistView"

describe("matchesName", () => {
  it.each([
    ["Beyoncé", "beyonce"],
    ["A$AP Rocky", "asap"],
    ["Mø", "mo"],
    ["Ænima", "aenima"],
    ["Straße", "strasse"],
    ["Bad Bunny", "BAD"],
  ])("matches %s for %s", (name, query) => {
    expect(matchesName(name, query)).toBe(true)
  })

  it("matches everything for a blank or whitespace-only query", () => {
    expect(matchesName("Drake", "")).toBe(true)
    expect(matchesName("Drake", "   ")).toBe(true)
  })

  it("ignores surrounding whitespace in the query", () => {
    expect(matchesName("Bad Bunny", "  bad ")).toBe(true)
  })

  it("rejects a non-matching name", () => {
    expect(matchesName("Drake", "bad")).toBe(false)
  })
})

describe("foldName", () => {
  it("folds Ø and Æ after lowercasing", () => {
    expect(foldName("ØÆ")).toBe("oae")
  })
})

describe("selectVisible", () => {
  const entries = [
    makeEntry({ id: 1, name: "Bad Bunny" }),
    makeEntry({ id: 2, name: "Drake" }),
    makeEntry({ id: 3, name: "Badmómó" }),
  ]

  it("keeps matches in sort order", () => {
    const ids = selectVisible(
      entries,
      { q: "bad", sort: "name-asc" },
      new Set()
    ).map((e) => e.id)
    expect(ids).toEqual([1, 3])
  })

  it("unions sticky ids with matches", () => {
    const ids = selectVisible(
      entries,
      { q: "bad", sort: "name-asc" },
      new Set([2])
    ).map((e) => e.id)
    expect(ids).toEqual([1, 3, 2])
  })

  it("returns an empty list for no entries", () => {
    expect(
      selectVisible([], { q: "x", sort: "name-asc" }, new Set([1]))
    ).toEqual([])
  })
})

describe("parseWatchlistParams", () => {
  it("reads q and defaults to empty", () => {
    expect(parseWatchlistParams(new URLSearchParams("q=bad")).q).toBe("bad")
    expect(parseWatchlistParams(new URLSearchParams("")).q).toBe("")
  })
})

describe("formatArtistCount", () => {
  it("pluralizes on the total", () => {
    expect(formatArtistCount(1, 1)).toBe("1 of 1 artist")
    expect(formatArtistCount(0, 52)).toBe("0 of 52 artists")
    expect(formatArtistCount(52, 52)).toBe("52 of 52 artists")
  })
})
