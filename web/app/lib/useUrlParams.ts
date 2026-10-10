import { useCallback, useRef } from "react"
import { useSearchParams } from "react-router"

export function urlWriteOptions(opts: { replace?: boolean } = {}) {
  return { replace: opts.replace ?? false, preventScrollReset: true as const }
}

// The one URL writer. It builds on a ref so two writes in one tick compose,
// and always passes preventScrollReset so filtering never jumps the page.
export function useUrlParams() {
  const [params, setParams] = useSearchParams()
  const latestRef = useRef(params)
  latestRef.current = params

  const write = useCallback(
    (
      mutate: (next: URLSearchParams) => void,
      opts: { replace?: boolean } = {}
    ) => {
      const next = new URLSearchParams(latestRef.current)
      mutate(next)
      latestRef.current = next
      setParams(next, urlWriteOptions(opts))
    },
    [setParams]
  )

  return { params, write }
}
