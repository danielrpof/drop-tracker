import { describe, expect, it } from "vitest"

import type { BulkAddTagOutcome, BulkRemoveTagOutcome } from "~/lib/api"
import {
  addTagToast,
  failureToast,
  preferencesToast,
  removeTagToast,
  removeWatchlistToast,
} from "~/lib/bulkOutcomeMessages"

const tag = { id: 7, name: "trap" }
const nameOf = (id: number) => `Artist ${id}`

function add(
  applied: number[],
  skipped: BulkAddTagOutcome["skipped"]
): BulkAddTagOutcome {
  return { tag, applied, skipped }
}

describe("addTagToast", () => {
  it.each([
    {
      name: "plain success",
      outcome: add([1, 2], []),
      expected: { level: "success", title: "Added “trap” to 2 artists." },
    },
    {
      name: "singular",
      outcome: add([1], []),
      expected: { level: "success", title: "Added “trap” to 1 artist." },
    },
    {
      name: "gone note on success",
      outcome: add([1], [{ id: 2, reason: "gone" }]),
      expected: {
        level: "success",
        title: "Added “trap” to 1 artist.",
        description: "1 artist was already removed from your watchlist.",
      },
    },
    {
      name: "some at cap, named from the server",
      outcome: add([1], [{ id: 2, name: "Kid", reason: "cap" }]),
      expected: {
        level: "warning",
        title: "Added “trap” to 1 artist.",
        description: "1 already has 10 tags: Kid.",
        duration: 8000,
      },
    },
    {
      name: "all at cap, with gone note",
      outcome: add(
        [],
        [
          { id: 2, name: "Kid", reason: "cap" },
          { id: 3, reason: "gone" },
        ]
      ),
      expected: {
        level: "warning",
        title:
          "Couldn't add “trap” — every selected artist already has 10 tags.",
        description: "Kid. 1 artist was already removed from your watchlist.",
        duration: 8000,
      },
    },
  ])("$name", ({ outcome, expected }) => {
    expect(addTagToast(outcome, [1, 2, 3], nameOf)).toEqual(expected)
  })

  it("names cap skips in sent order, falling back to nameOf", () => {
    const toast = addTagToast(
      add(
        [],
        [
          { id: 9, reason: "cap" },
          { id: 4, name: "Four", reason: "cap" },
        ]
      ),
      [4, 9],
      nameOf
    )
    expect(toast.description).toBe("Four and Artist 9.")
  })

  it("uses restore wording", () => {
    expect(addTagToast(add([1, 2], []), [1, 2], nameOf, "restore").title).toBe(
      "Restored “trap” on 2 artists."
    )
    expect(
      addTagToast(
        add([], [{ id: 1, name: "Kid", reason: "cap" }]),
        [1],
        nameOf,
        "restore"
      ).title
    ).toBe("Couldn't restore “trap” — every artist already has 10 tags.")
  })
})

describe("removeTagToast", () => {
  const out = (
    carried: number,
    skipped: BulkRemoveTagOutcome["skipped"] = []
  ): BulkRemoveTagOutcome => ({ applied: [], carried, skipped })

  it("reports the carrier count and keeps the toast long", () => {
    expect(removeTagToast("trap", out(1))).toEqual({
      level: "success",
      title: "Removed “trap” from 1 artist.",
      duration: 8000,
    })
  })

  it("adds the gone note", () => {
    expect(
      removeTagToast("trap", out(3, [{ id: 1, reason: "gone" }])).description
    ).toBe("1 artist was already removed from your watchlist.")
  })
})

describe("preferencesToast / removeWatchlistToast", () => {
  it("pluralises and adds the gone note", () => {
    expect(preferencesToast(2, [])).toEqual({
      level: "success",
      title: "Updated preferences for 2 artists.",
    })
    expect(
      preferencesToast(1, [{ reason: "gone" }, { reason: "gone" }])
    ).toEqual({
      level: "success",
      title: "Updated preferences for 1 artist.",
      description: "2 artists were already removed from your watchlist.",
    })
    expect(removeWatchlistToast(3, 1)).toEqual({
      level: "success",
      title: "Removed 3 artists from your watchlist.",
      description: "1 artist was already removed from your watchlist.",
    })
  })
})

describe("failureToast", () => {
  it("uses the fallback unless the server reported a cap", () => {
    expect(failureToast(null, "Nope.")).toEqual({
      level: "error",
      title: "Nope.",
    })
    expect(failureToast(1000, "Nope.").title).toBe(
      "Too many artists selected — select at most 1000 and try again."
    )
  })
})
