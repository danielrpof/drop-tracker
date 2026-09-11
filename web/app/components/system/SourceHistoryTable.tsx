import { OutcomeBadge } from "~/components/system/OutcomeBadge"
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table"
import {
  formatAbsoluteTime,
  formatDuration,
  formatIsoTitle,
} from "~/lib/format"
import type { StatusRun } from "~/lib/api"

export interface SourceHistoryTableProps {
  history: StatusRun[]
}

const HISTORY_CAP = 50

// captionText is the SYS-02 three-condition caption: at the cap, the locked
// "not retained" copy; below it, the actual count since the last restart,
// pluralized. Zero entries never reach here -- the component returns null
// before rendering any table shell (an empty header with no rows is the
// UI-SPEC's explicitly-ruled-out shape).
function captionText(count: number): string {
  if (count === HISTORY_CAP) {
    return "Showing the 50 most recent cycles for this source. Older history isn't retained — it lives only in the logs."
  }
  return `${count} cycle${count === 1 ? "" : "s"} recorded for this source since the last restart.`
}

// SourceHistoryTable renders one source's eight-column recent-runs table in
// the exact order the contract supplies -- no client-side sort, which would
// silently mask a backend ordering regression instead of surfacing it. The
// outcome cell reuses OutcomeBadge, the same classifier the panel's
// last-run line uses, so a run's tier is never re-derived a second way.
export function SourceHistoryTable({ history }: SourceHistoryTableProps) {
  if (history.length === 0) return null

  return (
    <Table>
      <TableCaption>{captionText(history.length)}</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead scope="col">Cycle</TableHead>
          <TableHead scope="col">Started</TableHead>
          <TableHead scope="col">Outcome</TableHead>
          <TableHead scope="col" className="text-right">
            Duration
          </TableHead>
          <TableHead scope="col" className="text-right">
            Checked
          </TableHead>
          <TableHead scope="col" className="text-right">
            Skipped
          </TableHead>
          <TableHead scope="col" className="text-right">
            Errored
          </TableHead>
          <TableHead scope="col" className="text-right">
            Events
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {history.map((run) => (
          <TableRow key={run.cycle_id}>
            <TableCell className="font-mono text-label text-muted-foreground">
              {run.cycle_id}
            </TableCell>
            <TableCell>
              <time
                dateTime={run.started_at}
                title={formatIsoTitle(run.started_at)}
              >
                {formatAbsoluteTime(run.started_at)}
              </time>
            </TableCell>
            <TableCell>
              <OutcomeBadge run={run} />
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {formatDuration(run.duration_ms)}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {run.artists_checked}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {run.artists_skipped}
            </TableCell>
            <TableCell
              className={
                run.artists_errored > 0
                  ? "text-right text-status-warn tabular-nums"
                  : "text-right tabular-nums"
              }
            >
              {run.artists_errored}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {run.events_recorded}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
