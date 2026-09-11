import { TriangleAlert } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link } from "react-router"

import { EmptyState } from "~/components/common/EmptyState"
import { AboutInstance } from "~/components/system/AboutInstance"
import { Alert, AlertDescription, AlertTitle } from "~/components/ui/alert"
import { Button } from "~/components/ui/button"
import { Skeleton } from "~/components/ui/skeleton"
import { ApiError, getStatus, type StatusResponse } from "~/lib/api"

// System is the SYS-01/02/03 operator status view. Plan 19-01's tracer
// proved the full route/nav/fetch/render stack end to end with one real
// payload field; this plan replaces that bare Version row with the real
// About block (D-01/D-02/D-03) and the empty-watchlist callout (D-05).
// Per-source panels (Task 3) and the first-run/loaded shape derivation
// (plan 19-05) build on the same render-precedence chain established here.
// Fetch happens on mount and on a Retry bump only (D-10): no background
// refresh mechanism exists anywhere in this file, by design.
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
        <>
          <AboutInstance
            instance={data.instance}
            watchlistSize={data.watchlist_size}
            pollIntervalSeconds={data.poll_interval_seconds}
          />

          {data.watchlist_size === 0 && (
            <Alert>
              <TriangleAlert aria-hidden="true" />
              <AlertTitle>Nothing to poll</AlertTitle>
              <AlertDescription>
                No artists on your watchlist — the scheduler has nothing to
                check each cycle.{" "}
                <Link
                  to="/"
                  className="text-primary underline-offset-4 hover:underline"
                >
                  Go to the Watchlist
                </Link>
              </AlertDescription>
            </Alert>
          )}
        </>
      )}
    </div>
  )
}
