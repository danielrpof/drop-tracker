import { useEffect, useRef, useState } from "react"

import { Card, CardContent, CardHeader } from "~/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select"
import { Switch } from "~/components/ui/switch"
import { formatAbsoluteTime, formatIsoTitle } from "~/lib/format"
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
type Cadence = NotificationSettings["digest_cadence"]

// Title-case display labels for the lowercase wire values -- the operator
// never sees "daily"/"weekly" (D-04, UI-SPEC Copywriting Contract).
const CADENCE_LABELS: Record<Cadence, string> = {
  daily: "Daily",
  weekly: "Weekly",
}

// DigestSettings renders the DGST-01/DGST-02/DGST-16 digest settings rows
// as instant-apply controls (D-02): no Save button, every change persists
// immediately. The rendered value is `pending` (the optimistic overlay)
// when present and the `settings` prop otherwise -- clearing the overlay on
// either success or failure is what gives the keep-stale-on-failure posture
// for free, with no separate rollback bookkeeping.
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

  // save is the one instant-apply path both controls route through -- the
  // route is full-object PUT semantics, so every call sends both fields
  // regardless of which one actually changed (T-20-19).
  async function save(next: {
    digestEnabled: boolean
    digestCadence: Cadence
  }) {
    if (savingRef.current) return
    savingRef.current = true
    setSaving(true)

    if (clearTimerRef.current) {
      clearTimeout(clearTimerRef.current)
      clearTimerRef.current = null
    }

    setPending({
      ...displayed,
      digest_enabled: next.digestEnabled,
      digest_cadence: next.digestCadence,
    })

    try {
      const saved = await updateDigestSettings(next)
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

  function handleDigestModeChange(next: boolean) {
    void save({ digestEnabled: next, digestCadence: displayed.digest_cadence })
  }

  function handleCadenceChange(next: Cadence | null) {
    // The two SelectItems below are the only selectable values, and this
    // Select is neither clearable nor multiple -- null is unreachable in
    // practice, but the primitive's type still carries it.
    if (next === null) return
    void save({ digestEnabled: displayed.digest_enabled, digestCadence: next })
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
            className="self-start text-label text-muted-foreground"
          >
            Digest mode
          </dt>
          <dd className="flex flex-col gap-1">
            <div className="flex items-center gap-1">
              <Switch
                checked={displayed.digest_enabled}
                onCheckedChange={handleDigestModeChange}
                disabled={saving}
                aria-labelledby="digest-mode-label"
              />
              <span className="text-body text-foreground">
                {displayed.digest_enabled ? "On" : "Off"}
              </span>
            </div>
            {/* Always visible in both toggle positions (D-06) -- it's the
                only place stating the toggle-off flush behavior (DGST-14). */}
            <p className="text-label text-muted-foreground">
              {
                "While on, new events wait for the next digest; switching back off delivers them individually."
              }
            </p>
          </dd>

          <dt
            id="digest-cadence-label"
            className="text-label text-muted-foreground"
          >
            Cadence
          </dt>
          <dd>
            {/* Stays visible and populated when digest mode is off --
                disabled only, never unmounted or reset to a placeholder
                (D-04). */}
            <Select
              value={displayed.digest_cadence}
              onValueChange={handleCadenceChange}
              disabled={saving || !displayed.digest_enabled}
            >
              <SelectTrigger aria-labelledby="digest-cadence-label">
                <SelectValue>
                  {(value: Cadence) => CADENCE_LABELS[value]}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="daily">Daily</SelectItem>
                <SelectItem value="weekly">Weekly</SelectItem>
              </SelectContent>
            </Select>
          </dd>

          <dt className="text-label text-muted-foreground">Last digest sent</dt>
          <dd>
            {displayed.digest_last_sent_at === null ? (
              // format.ts degrades a null instant to an em dash by design;
              // this row deliberately overrides that for an honest
              // never-sent state (DGST-16) -- the null branch is taken
              // before either formatter is ever called.
              <span className="text-body text-foreground">Never sent yet</span>
            ) : (
              <time
                dateTime={displayed.digest_last_sent_at}
                title={formatIsoTitle(displayed.digest_last_sent_at)}
                className="text-body text-foreground"
              >
                {formatAbsoluteTime(displayed.digest_last_sent_at)}
              </time>
            )}
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
