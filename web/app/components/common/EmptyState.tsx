import type { ReactNode } from "react"

// EmptyState is a styled dark-theme empty-state block (D-16): a heading, a
// body line, and an optional action slot -- never bare placeholder text.
// Shared by every list-collection empty state in this phase (Watchlist,
// History, filtered-to-zero History, per-source search columns), each of
// which supplies its own locked copy from 06-UI-SPEC.md's Copywriting
// Contract.
export interface EmptyStateProps {
  heading: string
  body: string
  action?: ReactNode
}

export function EmptyState({ heading, body, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-4 rounded-md bg-card px-6 py-16 text-center">
      <div className="flex flex-col gap-2">
        <h2 className="text-heading font-semibold text-foreground">{heading}</h2>
        <p className="text-body text-muted-foreground">{body}</p>
      </div>
      {action}
    </div>
  )
}
