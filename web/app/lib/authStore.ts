// authStore holds the SPA's two auth signals in a framework-free pub/sub
// module so `~/lib/api` can import it without pulling in React.
//   - `authed`: optimistic-true, volatile, never persisted — a persisted copy
//     is a client-side authz cache, the failure mode D-16 forbids.
//   - `gateActive`: per-browser-session, monotonic, presentation-only,
//     sessionStorage-backed (D-18, G-14-2, G-14-3). The server `401` stays the
//     sole enforcement.
import { useSyncExternalStore } from "react"

// The sessionStorage key this module owns, plus the one literal ever written
// under it — kept together so the write and the read cannot drift apart.
const GATE_ACTIVE_STORAGE_KEY = "dt_gate_active"
const GATE_ACTIVE_STORAGE_VALUE = "1"

// Only session-store touchpoints: readPersistedGateActive (seed at module
// load) and persistGateActive (write, from both mark* functions). The `typeof`
// probe stays INSIDE the `try` — a present-but-throwing accessor is not
// suppressed by `typeof` (WR-01). Both swallow every failure and never throw.
function readPersistedGateActive(): boolean {
  try {
    if (typeof sessionStorage === "undefined") {
      return false
    }
    return (
      sessionStorage.getItem(GATE_ACTIVE_STORAGE_KEY) ===
      GATE_ACTIVE_STORAGE_VALUE
    )
  } catch {
    return false
  }
}

function persistGateActive(): void {
  try {
    if (typeof sessionStorage === "undefined") {
      return
    }
    sessionStorage.setItem(GATE_ACTIVE_STORAGE_KEY, GATE_ACTIVE_STORAGE_VALUE)
  } catch {
    // Storage denied or full — the in-memory boolean still carries the
    // gate for this page load; only cross-reload durability is lost.
  }
}

let authed = true
// Seed from the session store, not a literal `false`, so a document reload
// observes what a previous page load learned (G-14-2).
let gateActive = readPersistedGateActive()

const listeners = new Set<() => void>()

function notify(): void {
  for (const listener of listeners) {
    listener()
  }
}

export const authStore = {
  isAuthed: (): boolean => authed,
  isGateActive: (): boolean => gateActive,

  // mark* calls converge: repeated or concurrent calls with the same argument
  // settle on one consistent state (GATE-05 concurrency edge).
  markAuthenticated(): void {
    authed = true
    gateActive = true
    persistGateActive()
    notify()
  },

  markUnauthenticated(): void {
    authed = false
    gateActive = true
    persistGateActive()
    notify()
  },

  // markGateActive latches the server's `X-Instance-Gated` marker via
  // `apiFetch` (G-14-3). It never reads or writes `authed` (D-16), and its
  // early return is scoped to this method only — it runs on every API
  // response, unlike the two discrete-event mark* functions above.
  markGateActive(): void {
    if (gateActive) {
      return
    }
    gateActive = true
    persistGateActive()
    notify()
  },

  subscribe(listener: () => void): () => void {
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
    }
  },
}

// useAuthed / useGateActive expose the signals to React via
// useSyncExternalStore. The snapshot getters return the cached module boolean
// and must never re-read the store.
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
