import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"

import {
  deleteTag,
  listTags,
  mergeTag,
  renameTag,
  type RenameTagResult,
  type TagRef,
  type TagSummary,
} from "~/lib/api"

// Single owner of GET /tags. The vocabulary loads lazily, never at mount (D-30).
export type TagChange =
  | { kind: "renamed"; tag: TagRef }
  | { kind: "merged"; sourceId: number; target: TagSummary }
  | { kind: "deleted"; tagId: number }

export type VocabularyStatus = "idle" | "loading" | "loaded" | "error"

export interface TagVocabulary {
  vocabulary: TagSummary[] | null
  status: VocabularyStatus
  ensureLoaded(): void
  reload(): void
  remember(tag: TagRef): void
  rename(id: number, name: string): Promise<RenameTagResult>
  merge(sourceId: number, targetId: number): Promise<TagSummary>
  remove(id: number): Promise<{ carrier_count: number }>
}

function sortTags(tags: TagSummary[]): TagSummary[] {
  return [...tags].sort((a, b) => {
    const cmp = a.name.localeCompare(b.name, undefined, {
      sensitivity: "base",
    })
    return cmp !== 0 ? cmp : a.id - b.id
  })
}

function applyChange(list: TagSummary[], change: TagChange): TagSummary[] {
  switch (change.kind) {
    case "renamed":
      return list.map((t) =>
        t.id === change.tag.id ? { ...t, name: change.tag.name } : t
      )
    case "merged":
      return list
        .filter((t) => t.id !== change.sourceId)
        .map((t) =>
          t.id === change.target.id
            ? {
                ...t,
                name: change.target.name,
                carrier_count: change.target.carrier_count,
              }
            : t
        )
    case "deleted":
      return list.filter((t) => t.id !== change.tagId)
  }
}

export function rewriteTags(tags: TagRef[], change: TagChange): TagRef[] {
  switch (change.kind) {
    case "renamed":
      return tags.some((t) => t.id === change.tag.id)
        ? tags.map((t) => (t.id === change.tag.id ? change.tag : t))
        : tags
    case "deleted":
      return tags.some((t) => t.id === change.tagId)
        ? tags.filter((t) => t.id !== change.tagId)
        : tags
    case "merged": {
      if (!tags.some((t) => t.id === change.sourceId)) return tags
      if (tags.some((t) => t.id === change.target.id)) {
        return tags.filter((t) => t.id !== change.sourceId)
      }
      const target = { id: change.target.id, name: change.target.name }
      return tags.map((t) => (t.id === change.sourceId ? target : t))
    }
  }
}

const TagVocabularyContext = createContext<TagVocabulary | null>(null)

export function TagVocabularyProvider({
  onChange,
  children,
}: {
  onChange?: (change: TagChange) => void
  children: ReactNode
}) {
  const [vocabulary, setVocabularyState] = useState<TagSummary[] | null>(null)
  const [status, setStatusState] = useState<VocabularyStatus>("idle")
  const vocabularyRef = useRef<TagSummary[] | null>(null)
  const statusRef = useRef<VocabularyStatus>("idle")
  // Every local change bumps the generation so a later-settling GET /tags
  // is dropped (WR-06).
  const genRef = useRef(0)
  const inFlightRef = useRef(false)
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange

  const setVocabulary = useCallback((next: TagSummary[]) => {
    vocabularyRef.current = next
    setVocabularyState(next)
  }, [])
  const setStatus = useCallback((next: VocabularyStatus) => {
    statusRef.current = next
    setStatusState(next)
  }, [])

  const fetchVocabulary = useCallback(() => {
    const gen = ++genRef.current
    inFlightRef.current = true
    setStatus("loading")
    listTags()
      .then((result) => {
        if (gen !== genRef.current) return
        inFlightRef.current = false
        setVocabulary(sortTags(result))
        setStatus("loaded")
      })
      .catch(() => {
        if (gen !== genRef.current) return
        inFlightRef.current = false
        setStatus("error")
      })
  }, [setStatus, setVocabulary])

  // Refetch if the dropped response was the one in flight, so the surface
  // never strands on loading.
  const supersede = useCallback(() => {
    genRef.current++
    if (inFlightRef.current) fetchVocabulary()
  }, [fetchVocabulary])

  const ensureLoaded = useCallback(() => {
    if (statusRef.current === "idle" || statusRef.current === "error") {
      fetchVocabulary()
    }
  }, [fetchVocabulary])

  const remember = useCallback(
    (tag: TagRef) => {
      const list = vocabularyRef.current
      if (list === null) {
        if (inFlightRef.current) supersede()
        return
      }
      if (list.some((t) => t.id === tag.id)) return
      // Created on a watched artist just now; Manage tags refetches true counts on open.
      setVocabulary(sortTags([...list, { ...tag, carrier_count: 1 }]))
      supersede()
    },
    [setVocabulary, supersede]
  )

  const applyLocal = useCallback(
    (change: TagChange) => {
      const list = vocabularyRef.current
      if (list !== null) setVocabulary(sortTags(applyChange(list, change)))
      supersede()
      onChangeRef.current?.(change)
    },
    [setVocabulary, supersede]
  )

  const rename = useCallback(
    async (id: number, name: string) => {
      const result = await renameTag(id, name)
      if (result.kind === "renamed") {
        applyLocal({ kind: "renamed", tag: result.tag })
      }
      return result
    },
    [applyLocal]
  )

  const merge = useCallback(
    async (sourceId: number, targetId: number) => {
      let target: TagSummary
      try {
        target = await mergeTag(sourceId, targetId)
      } catch (err) {
        fetchVocabulary()
        throw err
      }
      applyLocal({ kind: "merged", sourceId, target })
      return target
    },
    [applyLocal, fetchVocabulary]
  )

  const remove = useCallback(
    async (id: number) => {
      let result: { carrier_count: number }
      try {
        result = await deleteTag(id)
      } catch (err) {
        fetchVocabulary()
        throw err
      }
      applyLocal({ kind: "deleted", tagId: id })
      return result
    },
    [applyLocal, fetchVocabulary]
  )

  const value = useMemo<TagVocabulary>(
    () => ({
      vocabulary,
      status,
      ensureLoaded,
      reload: fetchVocabulary,
      remember,
      rename,
      merge,
      remove,
    }),
    [
      vocabulary,
      status,
      ensureLoaded,
      fetchVocabulary,
      remember,
      rename,
      merge,
      remove,
    ]
  )

  return (
    <TagVocabularyContext.Provider value={value}>
      {children}
    </TagVocabularyContext.Provider>
  )
}

export function useTagVocabulary(): TagVocabulary {
  const ctx = useContext(TagVocabularyContext)
  if (!ctx) {
    throw new Error(
      "useTagVocabulary must be used inside TagVocabularyProvider"
    )
  }
  return ctx
}
