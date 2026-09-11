import { useEffect, useRef, useState } from "react"

import { EmptyState } from "~/components/common/EmptyState"
import { Button } from "~/components/ui/button"
import { Skeleton } from "~/components/ui/skeleton"
import { ApiError, getStatus, type StatusResponse } from "~/lib/api"

// System is the SYS-01/02/03 operator status view. This tracer proves the
// full route/nav/fetch/render stack end to end with one real payload field
// (instance.app_version) -- panels, badges, formatters, and the history
// table are later plans' expansion of the same render-precedence chain
// established here. Fetch happens on mount and on a Retry bump only (D-10):
// no background refresh mechanism exists anywhere in this file, by design.
export default function System() {
  const [data, setData] = useState<StatusResponse | null>(null)
  const [initialLoading, setInitialLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)
  const [reloadToken, setReloadToken] = useState(0)
  // mountedRef is unused by this tracer's own logic but is seeded here so a
  // later plan's refresh handler (its own in-flight guard, independent of
  // this mount effect's cancelled flag) has it ready.
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    let cancelled = false
    setInitialLoading(true)
    setLoadError(false)

    getStatus()
      .then((body) => {
        if (cancelled) return
        setData(body)
      })
      .catch((err) => {
        if (cancelled) return
        // apiFetch already flipped authStore on a 401 -- <App> swaps to
        // <PassphraseScreen> on its own. Setting loadError here would flash
        // an error surface behind the login screen.
        if (err instanceof ApiError && err.status === 401) return
        setLoadError(true)
      })
      .finally(() => {
        if (!cancelled) setInitialLoading(false)
      })

    return () => {
      cancelled = true
      mountedRef.current = false
    }
  }, [reloadToken])

  const handleRetry = () => {
    setReloadToken((t) => t + 1)
  }

  return (
    <div className="flex flex-col gap-6 p-8">
      <h1 className="text-display font-semibold text-foreground">System</h1>

      {loadError && (
        <EmptyState
          heading="Couldn't load system status."
          body="The server didn't return a status. Try again in a moment."
          action={
            <Button variant="secondary" onClick={handleRetry}>
              Retry
            </Button>
          }
        />
      )}

      {!loadError && initialLoading && !data && (
        <Skeleton className="h-24 w-full" />
      )}

      {!loadError && data && (
        <div className="flex flex-col gap-2">
          <span className="text-label text-muted-foreground">Version</span>
          <span className="font-mono text-body text-foreground">
            {data.instance.app_version}
          </span>
        </div>
      )}
    </div>
  )
}
