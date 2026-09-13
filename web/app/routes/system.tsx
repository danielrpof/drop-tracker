import { Loader2, RefreshCw, TriangleAlert } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link } from "react-router"

import { EmptyState } from "~/components/common/EmptyState"
import { AboutInstance } from "~/components/system/AboutInstance"
import { DigestSettings } from "~/components/system/DigestSettings"
import { SourcePanel } from "~/components/system/SourcePanel"
import { Alert, AlertDescription, AlertTitle } from "~/components/ui/alert"
import { Button } from "~/components/ui/button"
import { Card, CardContent, CardHeader } from "~/components/ui/card"
import { Separator } from "~/components/ui/separator"
import { Skeleton } from "~/components/ui/skeleton"
import {
  ApiError,
  getDigestSettings,
  getStatus,
  type NotificationSettings,
  type StatusResponse,
} from "~/lib/api"
import { formatClock, formatIsoTitle, formatPollInterval } from "~/lib/format"
import { SOURCE_ORDER } from "~/lib/sources"

// System is the SYS-01/02/03 operator status view. Fetch happens on mount,
// on a Retry bump, and on a manual Refresh click -- and nowhere else (D-10).
// No periodic re-fetch, no page-visibility-driven re-fetch, and no
// focus-driven re-fetch exist anywhere in this file, by design; auto-refresh
// is the deferred OBS-01 follow-up.

// orderedSourceKeys puts SOURCE_ORDER's known keys first (MusicBrainz, then
// Deezer) regardless of the payload's own map key order, then appends any
// key the payload carries that SOURCE_ORDER doesn't know about -- dropping
// an unrecognised source silently is worse than an unordered extra panel.
function orderedSourceKeys(sources: StatusResponse["sources"]): string[] {
  const known = SOURCE_ORDER.filter((key) => key in sources)
  const rest = Object.keys(sources).filter(
    (key) => !(SOURCE_ORDER as readonly string[]).includes(key)
  )
  return [...known, ...rest]
}

// deriveLoadedShape is computed during render from the freshest payload,
// never stored in component state -- storing it would break the D-04
// guarantee that a Refresh returning an emptied buffer (a restart between
// fetches) can fall back to first-run instead of holding stale panels. The
// consecutive_skips clause is load-bearing, not defensive: a source whose
// every first cycle is overlap-skipped records no run row but does bump
// that counter, and without the clause such an instance would show
// reassuring first-run copy while every cycle is in fact failing to run.
export function deriveLoadedShape(
  data: StatusResponse
): "first-run" | "loaded" {
  const allRunless = Object.values(data.sources).every(
    (s) =>
      s.last_run === null && s.history.length === 0 && s.consecutive_skips === 0
  )
  return allRunless ? "first-run" : "loaded"
}

// SystemSkeleton mirrors the loaded layout (an About-block card, then two
// source-panel cards each ending in a table shimmer) rather than a bare
// spinner, so the loading state reads as "the same page, still arriving"
// instead of a blank page.
function SystemSkeleton() {
  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-40" />
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-full" />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-40" />
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-4 w-1/2" />
        </CardContent>
      </Card>

      {Array.from({ length: 2 }).map((_, i) => (
        <Card key={i}>
          <CardHeader>
            <Skeleton className="h-5 w-32" />
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-4 w-1/2" />
          </CardContent>
          <Separator />
          <CardContent className="pt-0">
            <Skeleton className="h-24 w-full" />
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

export default function System() {
  const [data, setData] = useState<StatusResponse | null>(null)
  const [digest, setDigest] = useState<NotificationSettings | null>(null)
  const [initialLoading, setInitialLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [refreshError, setRefreshError] = useState(false)
  const [asOf, setAsOf] = useState<Date | null>(null)
  const [reloadToken, setReloadToken] = useState(0)
  // mountedRef backstops handleRefresh: the mount effect's own cleanup flag
  // only covers requests that effect itself issued, not a later
  // event-handler-initiated fetch that resolves after this view unmounted.
  const mountedRef = useRef(true)
  // refreshingRef is the re-entrancy guard checked and set synchronously,
  // before React has committed the refreshing state update -- two clicks
  // dispatched in the same task both close over the same pre-render
  // `refreshing` value, so the state variable alone cannot stop a genuine
  // double-click from starting a second fetch.
  const refreshingRef = useRef(false)

  useEffect(() => {
    mountedRef.current = true
    let cancelled = false
    setInitialLoading(true)
    setLoadError(false)
    setRefreshError(false)

    // Digest settings ride this same mount fetch (D-10): one combined fetch,
    // one loadError surface covers both -- no separate digest-only fetch or
    // error path exists anywhere in this file.
    Promise.all([getStatus(), getDigestSettings()])
      .then(([statusBody, digestBody]) => {
        if (cancelled) return
        setData(statusBody)
        setDigest(digestBody)
        setAsOf(new Date())
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

  // handleRefresh carries its own re-entrancy and mounted guards (D-11):
  // the mount effect's cleanup flag does not cover a fetch this handler
  // itself started, so a resolve after the session expired and the outlet
  // remounted must not write state on a dead component or leave a request
  // unaccounted-for behind the login screen. It never clears existing data
  // and never falls back to the skeleton -- this is a dashboard an operator
  // watches, not a feed.
  const handleRefresh = async () => {
    if (refreshingRef.current) return
    refreshingRef.current = true
    setRefreshing(true)
    setRefreshError(false)

    try {
      const [next, nextDigest] = await Promise.all([
        getStatus(),
        getDigestSettings(),
      ])
      if (!mountedRef.current) return
      setData(next)
      setDigest(nextDigest)
      setAsOf(new Date())
    } catch (err) {
      if (!mountedRef.current) return
      if (err instanceof ApiError && err.status === 401) return
      setRefreshError(true)
    } finally {
      refreshingRef.current = false
      if (mountedRef.current) setRefreshing(false)
    }
  }

  const refreshDisabled = refreshing || (initialLoading && !data)

  return (
    <div className="flex flex-col gap-6 p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-display font-semibold text-foreground">System</h1>

        {!loadError && (
          <div className="flex items-center gap-4">
            {asOf && (
              <time
                dateTime={asOf.toISOString()}
                title={formatIsoTitle(asOf.toISOString())}
                className="text-label text-muted-foreground"
              >
                as of {formatClock(asOf)}
              </time>
            )}
            <Button
              variant="secondary"
              onClick={handleRefresh}
              disabled={refreshDisabled}
              aria-busy={refreshing}
            >
              {refreshing ? (
                <>
                  <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                  Refreshing…
                </>
              ) : (
                <>
                  <RefreshCw className="size-4" aria-hidden="true" />
                  Refresh
                </>
              )}
            </Button>
          </div>
        )}
      </div>

      {refreshError && (
        <p
          role="status"
          aria-live="polite"
          className="text-label text-destructive"
        >
          Couldn't refresh — still showing data as of{" "}
          {asOf ? formatClock(asOf) : "—"}.
        </p>
      )}

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

      {!loadError && initialLoading && !data && <SystemSkeleton />}

      {!loadError && data && (
        <>
          <AboutInstance
            instance={data.instance}
            watchlistSize={data.watchlist_size}
            pollIntervalSeconds={data.poll_interval_seconds}
          />

          {digest && <DigestSettings settings={digest} onSaved={setDigest} />}

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

          {deriveLoadedShape(data) === "first-run" ? (
            <EmptyState
              heading="No poll cycles yet"
              body={
                data.instance.schema_applied === null
                  ? "Can't reach the database — poll results won't be recorded until it's back. See the About section below."
                  : `The scheduler runs every ${formatPollInterval(data.poll_interval_seconds)}. The first results will appear here after the next cycle.`
              }
            />
          ) : (
            orderedSourceKeys(data.sources).map((key) => (
              <SourcePanel
                key={key}
                sourceKey={key}
                source={data.sources[key]}
              />
            ))
          )}
        </>
      )}
    </div>
  )
}
