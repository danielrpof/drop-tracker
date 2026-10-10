import { describe, expect, it } from "vitest"

import { makeEntry } from "~/lib/test/fixtures"
import {
  SORT_OPTIONS,
  canonicalWatchlistParams,
  compareForSort,
  foldName,
  formatArtistCount,
  matchesName,
  parseWatchlistParams,
  selectVisible,
  type SortKey,
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
      { q: "bad", sort: "name-asc", tagIds: [] },
      new Set()
    ).map((e) => e.id)
    expect(ids).toEqual([1, 3])
  })

  it("unions sticky ids with matches", () => {
    const ids = selectVisible(
      entries,
      { q: "bad", sort: "name-asc", tagIds: [] },
      new Set([2])
    ).map((e) => e.id)
    expect(ids).toEqual([1, 3, 2])
  })

  it("returns an empty list for no entries", () => {
    expect(
      selectVisible([], { q: "x", sort: "name-asc", tagIds: [] }, new Set([1]))
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

const ALL_SORTS = SORT_OPTIONS.map((o) => o.value)

function sorted(sort: SortKey, rows: ReturnType<typeof makeEntry>[]) {
  return [...rows].sort(compareForSort(sort)).map((e) => e.id)
}

describe("compareForSort", () => {
  it.each(ALL_SORTS)(
    "leaves empty and single lists unchanged in %s",
    (sort) => {
      const one = makeEntry({ id: 1, name: "Drake" })
      expect(sorted(sort, [])).toEqual([])
      expect(sorted(sort, [one])).toEqual([1])
    }
  )

  it("lists the six options in order with en-dash labels", () => {
    expect(SORT_OPTIONS.map((o) => o.label)).toEqual([
      "Name (A–Z)",
      "Name (Z–A)",
      "Date added (newest)",
      "Date added (oldest)",
      "Latest release (newest)",
      "Latest release (oldest)",
    ])
  })

  describe("name", () => {
    const rows = [
      makeEntry({ id: 2, name: "beyonce" }),
      makeEntry({ id: 1, name: "Beyoncé" }),
    ]

    it.each(["name-asc", "name-desc"] as const)(
      "keeps collator-equal names in id order under %s",
      (sort) => {
        expect(sorted(sort, rows)).toEqual([1, 2])
      }
    )

    it("sorts accented names with their base letter", () => {
      const list = [
        makeEntry({ id: 1, name: "Zed" }),
        makeEntry({ id: 2, name: "Ángel" }),
        makeEntry({ id: 3, name: "angel2" }),
      ]
      expect(sorted("name-asc", list)).toEqual([2, 3, 1])
      expect(sorted("name-desc", list)).toEqual([1, 3, 2])
    })
  })

  describe("date added", () => {
    const rows = [
      makeEntry({ id: 1, name: "a", created_at: "2026-01-01T00:00:00Z" }),
      makeEntry({ id: 2, name: "b", created_at: "2026-01-01T00:00:00.5Z" }),
    ]

    it("orders by instant, not by string", () => {
      expect(sorted("added-desc", rows)).toEqual([2, 1])
      expect(sorted("added-asc", rows)).toEqual([1, 2])
    })

    it.each(["added-asc", "added-desc"] as const)(
      "breaks identical instants on id under %s",
      (sort) => {
        const same = [
          makeEntry({ id: 5, name: "a", created_at: "2026-01-01T00:00:00Z" }),
          makeEntry({ id: 4, name: "b", created_at: "2026-01-01T00:00:00Z" }),
        ]
        expect(sorted(sort, same)).toEqual([4, 5])
      }
    )
  })

  describe("latest release", () => {
    const rows = [
      makeEntry({ id: 1, name: "a", latest_release_date: "2024-05-17" }),
      makeEntry({ id: 2, name: "b", latest_release_date: null }),
      makeEntry({ id: 3, name: "c", latest_release_date: "2024" }),
      makeEntry({ id: 4, name: "d", latest_release_date: "2024-05" }),
      makeEntry({ id: 5, name: "e", latest_release_date: null }),
    ]

    it("orders partial dates as the start of the period, nulls last", () => {
      expect(sorted("latest-asc", rows)).toEqual([3, 4, 1, 2, 5])
      expect(sorted("latest-desc", rows)).toEqual([1, 4, 3, 2, 5])
    })

    it.each(["latest-asc", "latest-desc"] as const)(
      "ignores next_release_date and ties on id under %s",
      (sort) => {
        const same = [
          makeEntry({
            id: 9,
            name: "a",
            latest_release_date: "2024",
            next_release_date: "2026-01",
          }),
          makeEntry({
            id: 8,
            name: "b",
            latest_release_date: "2024",
            next_release_date: "2030",
          }),
        ]
        expect(sorted(sort, same)).toEqual([8, 9])
      }
    )

    it("sorts an announced-only artist with the no-release group", () => {
      const list = [
        makeEntry({ id: 1, name: "a", next_release_date: "2027" }),
        makeEntry({ id: 2, name: "b", latest_release_date: "2020" }),
      ]
      expect(sorted("latest-asc", list)).toEqual([2, 1])
      expect(sorted("latest-desc", list)).toEqual([2, 1])
    })
  })

  it("sorts a sticky entry that fails the filter among the matches", () => {
    const list = [
      makeEntry({ id: 1, name: "Zed Bad" }),
      makeEntry({ id: 2, name: "Alpha" }),
      makeEntry({ id: 3, name: "Mid Bad" }),
    ]
    const ids = selectVisible(
      list,
      { q: "bad", sort: "name-asc", tagIds: [] },
      new Set([2])
    ).map((e) => e.id)
    expect(ids).toEqual([2, 3, 1])
  })
})

describe("canonicalWatchlistParams", () => {
  it.each(["", "sort=name-asc", "sort=bogus"])(
    "drops an absent, default, or unknown sort (%s)",
    (qs) => {
      expect(canonicalWatchlistParams(new URLSearchParams(qs)).toString()).toBe(
        ""
      )
    }
  )

  it("keeps a valid sort and leaves other keys alone", () => {
    const out = canonicalWatchlistParams(
      new URLSearchParams("q=bad&sort=latest-desc&other=1")
    )
    expect(out.toString()).toBe("q=bad&sort=latest-desc&other=1")
  })

  it("does not mutate its input", () => {
    const input = new URLSearchParams("sort=bogus")
    canonicalWatchlistParams(input)
    expect(input.toString()).toBe("sort=bogus")
  })
})
