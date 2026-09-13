import { useEffect, useRef, useState } from "react"

import { Card, CardContent, CardHeader } from "~/components/ui/card"
import { Switch } from "~/components/ui/switch"
import { cn } from "~/lib/utils"
import {
  ApiError,
  updateDigestSettings,
  type NotificationSettings,
} from "~/lib/api"

export interface DigestSettingsProps {
  settings: NotificationSettings
  onSaved: (next: NotificationSettings) => void
}

type SaveStatus = "idle" | "saved" | "failed"

// DigestSettings renders the DGST-01 digest-mode row as an instant-apply
// control (D-02): no Save button, every change persists immediately. The
// rendered value is `pending` (the optimistic overlay) when present and the
// `settings` prop otherwise -- clearing the overlay on either success or
// failure is what gives the keep-stale-on-failure posture for free, with no
// separate rollback bookkeeping. Task 1 renders only the Digest mode row;
// the cadence and last-sent rows are plan 20-04's addition.
export function DigestSettings({ settings, onSaved }: DigestSettingsProps) {
  const [pending, setPending] = useState<NotificationSettings | null>(null)
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState<SaveStatus>("idle")
  // savingRef is the re-entrancy guard, checked and set synchronously before
  // the first await -- the rendered `saving` state alone would let two
  // interactions dispatched in the same task both start a request (19-05
  // proved this with a real double-click on a state-only guard).
  const savingRef = useRef(false)
  const mountedRef = useRef(true)
  const clearTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      if (clearTimerRef.current) clearTimeout(clearTimerRef.current)
    }
  }, [])

  const displayed = pending ?? settings

  async function handleDigestModeChange(next: boolean) {
    if (savingRef.current) return
    savingRef.current = true
    setSaving(true)

    if (clearTimerRef.current) {
      clearTimeout(clearTimerRef.current)
      clearTimerRef.current = null
    }

    setPending({ ...displayed, digest_enabled: next })

    try {
      const saved = await updateDigestSettings({
        digestEnabled: next,
        digestCadence: displayed.digest_cadence,
      })
      if (!mountedRef.current) return
      onSaved(saved)
      setPending(null)
      setStatus("saved")
      clearTimerRef.current = setTimeout(() => {
        if (mountedRef.current) setStatus("idle")
      }, 2000)
    } catch (err) {
      if (!mountedRef.current) return
      // apiFetch already flipped authStore on a 401 -- <App> swaps to
      // <PassphraseScreen> on its own, mirroring system.tsx's own catch.
      if (err instanceof ApiError && err.status === 401) return
      setPending(null)
      setStatus("failed")
    } finally {
      savingRef.current = false
      if (mountedRef.current) setSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <h2 className="text-heading font-semibold text-foreground">
          Digest notifications
        </h2>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-[auto_1fr] items-center gap-x-4 gap-y-2">
          <dt
            id="digest-mode-label"
            className="text-label text-muted-foreground"
          >
            Digest mode
          </dt>
          <dd className="flex items-center gap-1">
            <Switch
              checked={displayed.digest_enabled}
              onCheckedChange={handleDigestModeChange}
              disabled={saving}
              aria-labelledby="digest-mode-label"
            />
            <span className="text-body text-foreground">
              {displayed.digest_enabled ? "On" : "Off"}
            </span>
          </dd>
        </dl>

        {status !== "idle" && (
          // Muted, not the reserved run-health palette (19-UI-SPEC.md): a
          // save confirmation is a completed user action, not a health
          // signal.
          <p
            role="status"
            aria-live="polite"
            className={cn(
              "mt-2 text-label",
              status === "saved" ? "text-muted-foreground" : "text-destructive"
            )}
          >
            {status === "saved"
              ? "Saved."
              : "Couldn't save — reverted to the previous value."}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
