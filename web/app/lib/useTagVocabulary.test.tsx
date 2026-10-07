import type { ReactNode } from "react"

import { act, renderHook } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import {
  deleteTag,
  listTags,
  mergeTag,
  renameTag,
  type TagSummary,
} from "~/lib/api"

import {
  rewriteTags,
  TagVocabularyProvider,
  useTagVocabulary,
  type TagChange,
} from "./useTagVocabulary"

vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  listTags: vi.fn(),
  deleteTag: vi.fn(),
  renameTag: vi.fn(),
  mergeTag: vi.fn(),
}))

const mockListTags = vi.mocked(listTags)
const mockRenameTag = vi.mocked(renameTag)
const mockMergeTag = vi.mocked(mergeTag)
const mockDeleteTag = vi.mocked(deleteTag)

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

const A: TagSummary = { id: 1, name: "alpha", carrier_count: 2 }
const B: TagSummary = { id: 2, name: "bravo", carrier_count: 3 }

let onChange = vi.fn()

function wrapper({ children }: { children: ReactNode }) {
  return (
    <TagVocabularyProvider onChange={onChange}>
      {children}
    </TagVocabularyProvider>
  )
}

function setup() {
  return renderHook(() => useTagVocabulary(), { wrapper })
}

// Loads [A, B] and leaves the vocabulary loaded.
async function setupLoaded() {
  mockListTags.mockResolvedValueOnce([B, A])
  const hook = setup()
  await act(async () => hook.result.current.ensureLoaded())
  return hook
}

beforeEach(() => {
  vi.resetAllMocks()
  onChange = vi.fn()
})

describe("useTagVocabulary", () => {
  it("throws outside a provider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {})
    expect(() => renderHook(() => useTagVocabulary())).toThrow(
      "useTagVocabulary must be used inside TagVocabularyProvider"
    )
    spy.mockRestore()
  })

  it("does not fetch at mount, then loads once and sorts (D-30)", async () => {
    const d = deferred<TagSummary[]>()
    mockListTags.mockReturnValueOnce(d.promise)
    const { result } = setup()
    expect(mockListTags).not.toHaveBeenCalled()
    expect(result.current.status).toBe("idle")
    expect(result.current.vocabulary).toBeNull()

    act(() => result.current.ensureLoaded())
    expect(result.current.status).toBe("loading")
    act(() => result.current.ensureLoaded())
    expect(mockListTags).toHaveBeenCalledTimes(1)

    await act(async () =>
      d.resolve([
        { id: 3, name: "Zed", carrier_count: 1 },
        B,
        { id: 4, name: "alpha", carrier_count: 1 },
        A,
      ])
    )
    expect(result.current.status).toBe("loaded")
    expect(result.current.vocabulary?.map((t) => t.id)).toEqual([1, 4, 2, 3])

    act(() => result.current.ensureLoaded())
    expect(mockListTags).toHaveBeenCalledTimes(1)
  })

  it("retries from error on the next ensureLoaded", async () => {
    mockListTags.mockRejectedValueOnce(new Error("down"))
    const { result } = setup()
    await act(async () => result.current.ensureLoaded())
    expect(result.current.status).toBe("error")
    expect(result.current.vocabulary).toBeNull()

    mockListTags.mockResolvedValueOnce([A])
    await act(async () => result.current.ensureLoaded())
    expect(result.current.status).toBe("loaded")
    expect(mockListTags).toHaveBeenCalledTimes(2)
  })

  it("reload always fetches and keeps the previous vocabulary readable", async () => {
    const { result } = await setupLoaded()
    const d = deferred<TagSummary[]>()
    mockListTags.mockReturnValueOnce(d.promise)
    act(() => result.current.reload())
    expect(mockListTags).toHaveBeenCalledTimes(2)
    expect(result.current.status).toBe("loading")
    expect(result.current.vocabulary).toHaveLength(2)
    await act(async () => d.resolve([A]))
    expect(result.current.vocabulary).toEqual([A])
    expect(result.current.status).toBe("loaded")
  })

  describe("remember", () => {
    it("survives a load that was in flight when the tag was created", async () => {
      const first = deferred<TagSummary[]>()
      const second = deferred<TagSummary[]>()
      mockListTags
        .mockReturnValueOnce(first.promise)
        .mockReturnValueOnce(second.promise)
      const { result } = setup()
      act(() => result.current.ensureLoaded())
      act(() => result.current.remember({ id: 9, name: "dembow" }))
      expect(mockListTags).toHaveBeenCalledTimes(2)

      await act(async () => first.resolve([]))
      expect(result.current.vocabulary).toBeNull()
      expect(result.current.status).toBe("loading")

      await act(async () =>
        second.resolve([{ id: 9, name: "dembow", carrier_count: 1 }])
      )
      expect(result.current.status).toBe("loaded")
      expect(result.current.vocabulary?.map((t) => t.name)).toEqual(["dembow"])
    })

    it("inserts in sorted position when loaded, with no fetch", async () => {
      const { result } = await setupLoaded()
      act(() => result.current.remember({ id: 9, name: "Beta" }))
      expect(result.current.vocabulary?.map((t) => t.name)).toEqual([
        "alpha",
        "Beta",
        "bravo",
      ])
      expect(mockListTags).toHaveBeenCalledTimes(1)
      expect(onChange).not.toHaveBeenCalled()
    })

    it("is a no-op for a known id and while idle", async () => {
      const idle = setup()
      act(() => idle.result.current.remember({ id: 9, name: "x" }))
      expect(idle.result.current.vocabulary).toBeNull()
      expect(mockListTags).not.toHaveBeenCalled()

      const { result } = await setupLoaded()
      const before = result.current.vocabulary
      act(() => result.current.remember({ id: 1, name: "alpha" }))
      expect(result.current.vocabulary).toBe(before)
      expect(mockListTags).toHaveBeenCalledTimes(1)
    })
  })

  describe("a mutation supersedes an in-flight load", () => {
    async function withPendingReload() {
      const { result } = await setupLoaded()
      const stale = deferred<TagSummary[]>()
      const fresh = deferred<TagSummary[]>()
      mockListTags
        .mockReturnValueOnce(stale.promise)
        .mockReturnValueOnce(fresh.promise)
      act(() => result.current.reload())
      return { result, stale, fresh }
    }

    it("rename", async () => {
      const { result, stale, fresh } = await withPendingReload()
      mockRenameTag.mockResolvedValueOnce({
        kind: "renamed",
        tag: { id: 1, name: "omega" },
      })
      await act(async () => {
        await result.current.rename(1, "omega")
      })
      expect(mockListTags).toHaveBeenCalledTimes(3)
      expect(result.current.vocabulary?.find((t) => t.id === 1)).toEqual({
        id: 1,
        name: "omega",
        carrier_count: 2,
      })

      await act(async () => stale.resolve([A, B]))
      expect(result.current.vocabulary?.find((t) => t.id === 1)?.name).toBe(
        "omega"
      )
      expect(result.current.status).toBe("loading")

      await act(async () => fresh.resolve([B, { ...A, name: "omega" }]))
      expect(result.current.status).toBe("loaded")
    })

    it("merge", async () => {
      const { result, stale, fresh } = await withPendingReload()
      mockMergeTag.mockResolvedValueOnce({
        id: 2,
        name: "Bravo",
        carrier_count: 4,
      })
      await act(async () => {
        await result.current.merge(1, 2)
      })
      expect(mockListTags).toHaveBeenCalledTimes(3)
      expect(result.current.vocabulary).toEqual([
        { id: 2, name: "Bravo", carrier_count: 4 },
      ])

      await act(async () => stale.resolve([A, B]))
      expect(result.current.vocabulary).toHaveLength(1)

      await act(async () =>
        fresh.resolve([{ id: 2, name: "Bravo", carrier_count: 4 }])
      )
      expect(result.current.status).toBe("loaded")
    })

    it("delete", async () => {
      const { result, stale, fresh } = await withPendingReload()
      mockDeleteTag.mockResolvedValueOnce({ carrier_count: 2 })
      await act(async () => {
        await result.current.remove(1)
      })
      expect(mockListTags).toHaveBeenCalledTimes(3)
      expect(result.current.vocabulary).toEqual([B])

      await act(async () => stale.resolve([A, B]))
      expect(result.current.vocabulary).toEqual([B])

      await act(async () => fresh.resolve([B]))
      expect(result.current.status).toBe("loaded")
    })
  })

  it("applies a mutation locally without a fetch when nothing is in flight", async () => {
    const { result } = await setupLoaded()
    mockDeleteTag.mockResolvedValueOnce({ carrier_count: 2 })
    let out: { carrier_count: number } | undefined
    await act(async () => {
      out = await result.current.remove(1)
    })
    expect(out).toEqual({ carrier_count: 2 })
    expect(mockListTags).toHaveBeenCalledTimes(1)
    expect(result.current.status).toBe("loaded")
    expect(result.current.vocabulary).toEqual([B])
  })

  describe("onChange", () => {
    it("fires once per successful mutation with the matching change", async () => {
      const { result } = await setupLoaded()
      const tag = { id: 1, name: "omega" }
      mockRenameTag.mockResolvedValueOnce({ kind: "renamed", tag })
      await act(async () => {
        await result.current.rename(1, "omega")
      })
      expect(onChange).toHaveBeenLastCalledWith({ kind: "renamed", tag })

      const merged = { id: 2, name: "bravo", carrier_count: 5 }
      mockMergeTag.mockResolvedValueOnce(merged)
      await act(async () => {
        await result.current.merge(1, 2)
      })
      expect(onChange).toHaveBeenLastCalledWith({
        kind: "merged",
        sourceId: 1,
        target: merged,
      })

      mockDeleteTag.mockResolvedValueOnce({ carrier_count: 5 })
      await act(async () => {
        await result.current.remove(2)
      })
      expect(onChange).toHaveBeenLastCalledWith({ kind: "deleted", tagId: 2 })
      expect(onChange).toHaveBeenCalledTimes(3)
    })

    it("does not fire on a collision, a failure or remember", async () => {
      const { result } = await setupLoaded()
      mockRenameTag.mockResolvedValueOnce({
        kind: "collision",
        target: { id: 2, name: "bravo" },
        carrierCountAfterMerge: 4,
      })
      const before = result.current.vocabulary
      let out: unknown
      await act(async () => {
        out = await result.current.rename(1, "bravo")
      })
      expect(out).toMatchObject({ kind: "collision" })
      expect(result.current.vocabulary).toBe(before)
      expect(mockListTags).toHaveBeenCalledTimes(1)

      mockRenameTag.mockRejectedValueOnce(new Error("boom"))
      await act(async () => {
        await expect(result.current.rename(1, "x")).rejects.toThrow("boom")
      })
      expect(mockListTags).toHaveBeenCalledTimes(1)

      act(() => result.current.remember({ id: 9, name: "new" }))
      expect(onChange).not.toHaveBeenCalled()
    })

    it("reloads once after a failed merge or delete", async () => {
      const { result } = await setupLoaded()
      mockMergeTag.mockRejectedValueOnce(new Error("boom"))
      mockListTags.mockResolvedValueOnce([A, B])
      await act(async () => {
        await expect(result.current.merge(1, 2)).rejects.toThrow("boom")
      })
      expect(mockListTags).toHaveBeenCalledTimes(2)

      mockDeleteTag.mockRejectedValueOnce(new Error("boom"))
      mockListTags.mockResolvedValueOnce([A, B])
      await act(async () => {
        await expect(result.current.remove(1)).rejects.toThrow("boom")
      })
      expect(mockListTags).toHaveBeenCalledTimes(3)
      expect(onChange).not.toHaveBeenCalled()
    })

    it("calls the latest onChange after a re-render", async () => {
      const { result, rerender } = await setupLoaded()
      const next = vi.fn()
      onChange = next
      rerender()
      mockDeleteTag.mockResolvedValueOnce({ carrier_count: 0 })
      await act(async () => {
        await result.current.remove(1)
      })
      expect(next).toHaveBeenCalledWith({ kind: "deleted", tagId: 1 })
    })
  })

  it("ignores a stale failure (WR-08)", async () => {
    const { result } = await setupLoaded()
    const one = deferred<TagSummary[]>()
    const two = deferred<TagSummary[]>()
    mockListTags
      .mockReturnValueOnce(one.promise)
      .mockReturnValueOnce(two.promise)
    act(() => result.current.reload())
    act(() => result.current.reload())
    await act(async () => two.resolve([A]))
    await act(async () => one.reject(new Error("late")))
    expect(result.current.status).toBe("loaded")
    expect(result.current.vocabulary).toEqual([A])
  })
})

describe("rewriteTags", () => {
  const tags = [
    { id: 1, name: "a" },
    { id: 2, name: "b" },
    { id: 3, name: "c" },
  ]

  it("renames in place", () => {
    const change: TagChange = { kind: "renamed", tag: { id: 2, name: "B2" } }
    expect(rewriteTags(tags, change)).toEqual([
      { id: 1, name: "a" },
      { id: 2, name: "B2" },
      { id: 3, name: "c" },
    ])
  })

  it("deletes", () => {
    expect(rewriteTags(tags, { kind: "deleted", tagId: 2 })).toEqual([
      tags[0],
      tags[2],
    ])
  })

  it("merge replaces the source in place with exactly id and name", () => {
    const out = rewriteTags(tags, {
      kind: "merged",
      sourceId: 2,
      target: { id: 9, name: "T", carrier_count: 4 },
    })
    expect(out).toEqual([tags[0], { id: 9, name: "T" }, tags[2]])
    expect(Object.keys(out[1]!)).toEqual(["id", "name"])
  })

  it("merge drops the source when the target is already present", () => {
    expect(
      rewriteTags(tags, {
        kind: "merged",
        sourceId: 2,
        target: { id: 3, name: "c", carrier_count: 4 },
      })
    ).toEqual([tags[0], tags[2]])
  })

  it("returns the same array when the change does not touch it", () => {
    expect(rewriteTags(tags, { kind: "deleted", tagId: 99 })).toBe(tags)
    expect(
      rewriteTags(tags, { kind: "renamed", tag: { id: 99, name: "x" } })
    ).toBe(tags)
    expect(
      rewriteTags(tags, {
        kind: "merged",
        sourceId: 99,
        target: { id: 3, name: "c", carrier_count: 1 },
      })
    ).toBe(tags)
  })
})
