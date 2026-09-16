import { TriangleAlert } from "lucide-react"

import { Card, CardContent, CardHeader } from "~/components/ui/card"
import { formatPollInterval } from "~/lib/format"
import { cn } from "~/lib/utils"
import type { StatusInstance } from "~/lib/api"

export interface AboutInstanceProps {
  instance: StatusInstance
  watchlistSize: number
  pollIntervalSeconds: number
}

// AboutInstance renders the SYS-02 About block from an already-loaded status
// payload. Database reachability (D-01) and the schema-drift flag (D-02) are
// both derived from instance.schema_applied alone -- this file issues no
// network request of its own, and consults no separate readiness endpoint.
export function AboutInstance({
  instance,
  watchlistSize,
  pollIntervalSeconds,
}: AboutInstanceProps) {
  const { app_version, schema_applied, schema_expected } = instance
  const reachable = schema_applied !== null

  return (
    <Card>
      <CardHeader>
        <h2 className="text-heading font-semibold text-foreground">
          About this instance
        </h2>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-[auto_1fr] items-center gap-x-4 gap-y-2">
          <dt className="text-label text-muted-foreground">Version</dt>
          <dd className="font-mono text-body text-foreground">{app_version}</dd>

          <dt className="text-label text-muted-foreground">Schema</dt>
          <dd className="text-body">
            {schema_applied === null ? (
              <span className="text-foreground">—</span>
            ) : schema_applied === schema_expected ? (
              <span className="text-foreground">schema {schema_applied}</span>
            ) : (
              <span className="inline-flex items-center gap-1 text-status-warn">
                <TriangleAlert className="size-3" aria-hidden="true" />
                applied {schema_applied} · expects {schema_expected}
              </span>
            )}
          </dd>

          <dt className="text-label text-muted-foreground">Database</dt>
          <dd className="text-body">
            <span
              className={cn(
                "inline-flex items-center rounded-full px-2 py-0.5 text-label",
                reachable
                  ? "bg-status-ok/15 text-status-ok"
                  : "bg-destructive/10 text-destructive"
              )}
            >
              {reachable ? "Database reachable" : "Database unreachable"}
            </span>
          </dd>

          <dt className="text-label text-muted-foreground">Watchlist</dt>
          <dd className="text-body text-foreground">
            {watchlistSize} artist{watchlistSize === 1 ? "" : "s"}
          </dd>

          <dt className="text-label text-muted-foreground">Poll interval</dt>
          <dd className="text-body text-foreground">
            Every {formatPollInterval(pollIntervalSeconds)}
          </dd>
        </dl>
      </CardContent>
    </Card>
  )
}
