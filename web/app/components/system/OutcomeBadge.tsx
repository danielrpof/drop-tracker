import { Ban, type LucideIcon } from "lucide-react"

import { Badge } from "~/components/ui/badge"
import type { StatusRun } from "~/lib/api"

// classifyOutcome is the single D-07 mapping from a run's wire fields to a
// display tier. "Completed with errors" has no wire counterpart -- the
// contract's outcome set is only ok/error/cancelled -- so it is derived here
// from artists_errored, the one place that split happens. The default arm is
// the resilience line (T-19-13): an N-1/N deploy can put a value here this
// switch has never seen, and it must degrade to a grey badge, never throw or
// echo the raw string.
export interface OutcomeTier {
  label: string
  className?: string
  variant?: "destructive" | "secondary"
  icon?: LucideIcon
}

function titleCase(value: string): string {
  if (!value) return "Unknown"
  return value.charAt(0).toUpperCase() + value.slice(1).toLowerCase()
}

export function classifyOutcome(
  run: Pick<StatusRun, "outcome" | "artists_errored">
): OutcomeTier {
  switch (run.outcome) {
    case "ok":
      return run.artists_errored > 0
        ? {
            label: "Completed with errors",
            className: "bg-status-warn/15 text-status-warn",
          }
        : { label: "Success", className: "bg-status-ok/15 text-status-ok" }
    case "error":
      return { label: "Failed", variant: "destructive" }
    case "cancelled":
      return {
        label: "Interrupted",
        className: "bg-status-warn/15 text-status-warn",
        icon: Ban,
      }
    default:
      return { label: titleCase(run.outcome), variant: "secondary" }
  }
}

// OutcomeBadge is a pure presentational leaf -- no fetch, no state, no
// formatting logic. It takes the two run fields classifyOutcome needs and
// renders the classified tier via the existing Badge component.
export function OutcomeBadge({
  run,
}: {
  run: Pick<StatusRun, "outcome" | "artists_errored">
}) {
  const tier = classifyOutcome(run)
  const Icon = tier.icon

  return (
    <Badge variant={tier.variant} className={tier.className}>
      {Icon && <Icon className="size-3" aria-hidden="true" />}
      {tier.label}
    </Badge>
  )
}
