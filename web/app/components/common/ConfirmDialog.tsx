import { useState, type RefObject } from "react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "~/components/ui/alert-dialog"

export interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  actionLabel: string
  pendingLabel: string
  variant: "default" | "destructive"
  onConfirm: () => Promise<void>
  finalFocus?: RefObject<HTMLElement | null>
}

// ConfirmDialog (D-16) is the one reusable confirm dialog for merge and
// delete -- built on the vendored AlertDialog, Phase 26's bulk remove reuses
// it too. Cancel renders before the action button in DOM order, which is
// what gives it base-ui's default initial focus (the first tabbable
// element) without an explicit ref -- the safe default for a destructive or
// one-way action. While onConfirm runs, both buttons disable and the action
// shows pendingLabel at a fixed width so nothing shifts. The caller owns
// toasts: this component only needs to know the attempt is over (resolved
// or rejected) so it can close itself.
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  actionLabel,
  pendingLabel,
  variant,
  onConfirm,
  finalFocus,
}: ConfirmDialogProps) {
  const [pending, setPending] = useState(false)

  async function handleConfirm() {
    setPending(true)
    try {
      await onConfirm()
    } catch {
      // The caller is responsible for reporting failures (toast +
      // re-fetch); this component only needs to close once the attempt
      // is over.
    } finally {
      setPending(false)
      onOpenChange(false)
    }
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next)
      }}
    >
      <AlertDialogContent size="default" finalFocus={finalFocus}>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            variant={variant}
            disabled={pending}
            className="min-w-28"
            onClick={handleConfirm}
          >
            {pending ? pendingLabel : actionLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
