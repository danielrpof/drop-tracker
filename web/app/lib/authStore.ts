// authStore is the SPA's shared authentication signal — a plain,
// framework-free pub/sub module so `~/lib/api` can import it without pulling
// React into a module that most test files mock wholesale, and so unit
// tests can drive it directly.
//
// Two module-level booleans:
//
//   - `authed`   — starts `true` (optimistic; there is no boot-time
//     `GET /session` check, D-16). The first API `401` flips it `false`,
//     which makes `<App>` render `<PassphraseScreen>` instead of the routed
//     page. A successful login flips it back to `true`.
//
//   - `gateActive` — starts `false`, set `true` the first time the app
//     observes a `401` OR completes a login in this browser session. This
//     implements locked decision **D-18** (which resolved 14-UI-SPEC's one
//     open item): the **Log out** control renders only when `gateActive` is
//     `true`, so an instance with no `/session` route registered never
//     shows a control that would call a route that does not exist.
//     `gateActive` is presentation-only — it is NEVER an access-control
//     signal. The server `401` remains the sole enforcement (a plan
//     prohibition).
//
// Both `mark*` functions set `gateActive` to `true`, because either signal
// proves the instance is gated. They are convergent and safe to call
// repeatedly with the same argument — several simultaneous `401`s all land
// on one consistent state (GATE-05 concurrency edge). Calling a `mark`
// function always notifies subscribers, even when the state did not change,
// so an idempotent repeat is observable but harmless.
import { useSyncExternalStore } from "react"

let authed = true
let gateActive = false

const listeners = new Set<() => void>()

function notify(): void {
  for (const listener of listeners) {
    listener()
  }
}

export const authStore = {
  isAuthed: (): boolean => authed,
  isGateActive: (): boolean => gateActive,

  markAuthenticated(): void {
    authed = true
    gateActive = true
    notify()
  },

  markUnauthenticated(): void {
    authed = false
    gateActive = true
    notify()
  },

  subscribe(listener: () => void): () => void {
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
    }
  },
}

// useAuthed / useGateActive expose the two signals to React via
// useSyncExternalStore. `authStore.isAuthed` / `authStore.isGateActive` are
// reused as both the client and the server snapshot: the server snapshot is
// simply the optimistic default before any subscription, which is exactly
// what these getters return on a fresh load.
export function useAuthed(): boolean {
  return useSyncExternalStore(
    authStore.subscribe,
    authStore.isAuthed,
    authStore.isAuthed
  )
}

export function useGateActive(): boolean {
  return useSyncExternalStore(
    authStore.subscribe,
    authStore.isGateActive,
    authStore.isGateActive
  )
}
