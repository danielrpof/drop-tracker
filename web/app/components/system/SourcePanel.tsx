import { TriangleAlert } from "lucide-react"

import { OutcomeBadge } from "~/components/system/OutcomeBadge"
import { Card, CardContent, CardHeader } from "~/components/ui/card"
import { Separator } from "~/components/ui/separator"
import {
  formatAbsoluteTime,
  formatDuration,
  formatIsoTitle,
  formatRelativeTime,
} from "~/lib/format"
import { sourceDisplayName } from "~/lib/sources"
import type { StatusRun, StatusSource } from "~/lib/api"

export interface SourcePanelProps {
  sourceKey: string
  source: StatusSource
}

// findCleanRun is the D-08 scan: the newest history entry that is both
// outcome "ok" AND zero-errored. A "Completed with errors" run has
// outcome "ok" too but wears the amber badge -- calling it the last clean
// run would contradict its own badge, so artists_errored === 0 is a
// mandatory second clause, not just outcome === "ok".
function findCleanRun(history: StatusRun[]): StatusRun | undefined {
  return history.find((r) => r.outcome === "ok" && r.artists_errored === 0)
}

// SourcePanel renders one source's SYS-01 panel: last-run summary, the D-08
// clean-run line, and the conditional skip/escalation lines. The history
// table plan 19-05 adds sits below the trailing Separator this panel ends
// with.
export function SourcePanel({ sourceKey, source }: SourcePanelProps) {
  const name = sourceDisplayName(sourceKey)
  const { last_run, history, last_skipped_at, consecutive_skips } = source
  const cleanRun = findCleanRun(history)
  // Driven by the latest run alone (D-07): one shutdown-cancelled run during
  // a deploy trips this briefly, a real crash loop keeps it lit.
  const latestIsCancelled = last_run?.outcome === "cancelled"

  return (
    <Card>
      <CardHeader>
        <h2 className="text-heading font-semibold text-foreground">{name}</h2>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        {last_run ? (
          <>
            <div className="flex items-center gap-2">
              <OutcomeBadge run={last_run} />
              <time
                dateTime={last_run.finished_at}
                title={formatIsoTitle(last_run.finished_at)}
                className="text-body text-foreground"
              >
                {formatAbsoluteTime(last_run.finished_at)}
              </time>
              <span className="text-body text-muted-foreground">
                {formatDuration(last_run.duration_ms)}
              </span>
            </div>

            <p className="text-body text-foreground">{last_run.summary}</p>

            {last_run.artists_errored > 0 && (
              <p className="text-label text-muted-foreground">
                {last_run.artists_checked} checked · {last_run.artists_skipped}{" "}
                skipped ·{" "}
                <span className="text-status-warn">
                  {last_run.artists_errored} errored
                </span>{" "}
                · {last_run.events_recorded} events
              </p>
            )}

            <p className="text-label text-muted-foreground">
              {cleanRun ? (
                <>Last clean run {formatRelativeTime(cleanRun.finished_at)}</>
              ) : (
                "No clean run in recent history"
              )}
            </p>
          </>
        ) : (
          <p className="text-body text-muted-foreground">
            No poll cycles recorded for {name} yet.
          </p>
        )}

        {consecutive_skips > 0 && (
          <p className="flex items-center gap-1 text-label text-status-warn">
            <TriangleAlert className="size-3" aria-hidden="true" />
            Skipped {consecutive_skips} consecutive cycle
            {consecutive_skips === 1 ? "" : "s"}, most recently{" "}
            <time
              dateTime={last_skipped_at ?? undefined}
              title={
                last_skipped_at ? formatIsoTitle(last_skipped_at) : undefined
              }
            >
              {formatAbsoluteTime(last_skipped_at)}
            </time>
            .
          </p>
        )}

        {latestIsCancelled && (
          <p className="flex items-center gap-1 text-label text-status-warn">
            <TriangleAlert className="size-3" aria-hidden="true" />
            Recent cycles are being interrupted — check for a restart or crash
            loop.
          </p>
        )}
      </CardContent>

      <Separator />
    </Card>
  )
}
