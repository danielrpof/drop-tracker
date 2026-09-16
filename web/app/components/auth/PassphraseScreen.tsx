import { useEffect, useState } from "react"

import { Button } from "~/components/ui/button"
import { Input } from "~/components/ui/input"
import { ApiError, createSession } from "~/lib/api"
import { authStore } from "~/lib/authStore"

// Every user-visible string is fixed by 14-UI-SPEC's Copywriting Contract
// and used verbatim -- none is data-derived, so all copy wraps inside the
// fixed-width card, which grows vertically with no truncation (E1/E4
// long-text). The submitted passphrase is NEVER rendered as text, put in a
// message, a toast, a console call, or any DOM node other than the
// type=password input itself (D-13, secret-safety copy rule).
const HEADING = "Enter the instance passphrase"
const BODY =
  "This drop-tracker instance is private. Enter the passphrase to view the watchlist and release history."
const FIELD_LABEL = "Passphrase"
const LABEL_IDLE = "Unlock"
const LABEL_SUBMITTING = "Unlocking…"

const ERROR_WRONG = "That passphrase isn't correct. Check it and try again."
const ERROR_THROTTLED =
  "Too many attempts. Wait about a minute, then try again."
const ERROR_CONNECTION =
  "Couldn't reach the server. Check your connection and try again."

const FIELD_ID = "passphrase"

// PassphraseScreen is the full-screen gate <App> renders instead of the
// routed page whenever authStore reports unauthenticated (GATE-05, D-16).
// Layout is fixed by the UI spec: a viewport-centred fixed-width card that
// scrolls within the full-height wrapper on a short viewport rather than
// clipping (E1 overflow).
export default function PassphraseScreen() {
  const [value, setValue] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // After a 401 or 429 the button stays disabled until the field is edited
  // (E3 error); a network / 5xx error re-enables it immediately.
  const [lockedUntilEdit, setLockedUntilEdit] = useState(false)

  // Autofocus on mount (E2 empty), and restore focus after a settled submit
  // that produced an error so a typo can be corrected without losing the
  // field (E2 error). The field is disabled while the request is in flight,
  // which drops focus; this puts it back once the field is interactive again.
  useEffect(() => {
    if (!submitting) {
      document.getElementById(FIELD_ID)?.focus()
    }
  }, [submitting, error])

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (submitting) return

    // Clear any previous message so only one outcome is ever visible (E4).
    setError(null)
    setSubmitting(true)
    try {
      await createSession(value)
      // Only flip auth state after the promise resolves -- a rejected login
      // must never mark the store authenticated.
      authStore.markAuthenticated()
    } catch (err) {
      const status = err instanceof ApiError ? err.status : undefined
      if (status === 401) {
        setError(ERROR_WRONG)
        setLockedUntilEdit(true)
      } else if (status === 429) {
        setError(ERROR_THROTTLED)
        setLockedUntilEdit(true)
      } else {
        // No status -- a thrown network failure -- or a 5xx: the connection
        // copy, and the button re-enables immediately.
        setError(ERROR_CONNECTION)
        setLockedUntilEdit(false)
      }
    } finally {
      setSubmitting(false)
    }
  }

  function handleChange(event: React.ChangeEvent<HTMLInputElement>) {
    setValue(event.target.value)
    if (lockedUntilEdit) {
      setLockedUntilEdit(false)
      setError(null)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-8">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm rounded-md bg-card p-8"
      >
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-2">
            <h1 className="text-heading font-semibold text-foreground">
              {HEADING}
            </h1>
            <p className="text-body text-muted-foreground">{BODY}</p>
          </div>

          <div className="flex flex-col gap-2">
            <label
              htmlFor={FIELD_ID}
              className="text-label text-muted-foreground"
            >
              {FIELD_LABEL}
            </label>
            <Input
              id={FIELD_ID}
              type="password"
              autoComplete="current-password"
              value={value}
              onChange={handleChange}
              disabled={submitting}
            />
            {error && <p className="text-label text-destructive">{error}</p>}
          </div>

          <Button type="submit" disabled={submitting || lockedUntilEdit}>
            {submitting ? LABEL_SUBMITTING : LABEL_IDLE}
          </Button>
        </div>
      </form>
    </div>
  )
}
