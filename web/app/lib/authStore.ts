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
//     page. A successful login flips it back to `true`. `authed` is
//     deliberately volatile and is NEVER written to web storage: a
//     persisted copy would be a client-side authorization cache, which is
//     the exact failure mode D-16 exists to avoid.
//
//   - `gateActive` — set `true` the first time the app, in this browser
//     session, observes ANY of three things: a `401`, a completed login, or a
//     response that passed the gate (the server marks every such response
//     with an `X-Instance-Gated` header; `apiFetch` latches it via
//     `markGateActive`). The third trigger is what closes **G-14-3**: a
//     session that already carries a valid `dt_session` cookie and so sees
//     neither a `401` nor a typed login still learns the instance is gated
//     from its first ordinary authenticated `200`. This implements locked
//     decision **D-18** (which resolved 14-UI-SPEC's one open item): the
//     **Log out** control renders only when `gateActive` is `true`, so an
//     instance with no `/session` route registered never shows a control
//     that would call a route that does not exist. The `X-Instance-Gated`
//     marker only exists on the gated code path, so that guarantee now holds
//     structurally, not merely by the absence of a `401`.
//
//     "In this browser session" (D-18) is `sessionStorage`, not a
//     module-level `let`. `gateActive` is SEEDED from the browser session
//     store at module load and WRITTEN THROUGH by both `mark*` functions,
//     so a full document reload while the `dt_session` cookie is still
//     valid — loader fetch returns 200, no `401` fires, no login happens —
//     still renders the Log out control for a user who is still logged in.
//     That reload re-runs only the module initialiser, which is why seeding
//     it from storage repairs every downstream consumer (`useGateActive` →
//     the `{gateActive && <LogoutButton />}` branch in `root.tsx`) with no
//     change to `root.tsx` at all. This closes gap G-14-2.
//
//     A per-tab, session-scoped store is the correct primitive. A
//     cross-session store (`localStorage`) is NOT: a signal that outlived
//     the browser session would show a Log out control in a brand-new
//     session on an instance whose passphrase had since been removed, and
//     that control would call a route that is not registered. `gateActive`
//     is also monotonic within a session — once recorded it is never
//     cleared by a logout, a `401`, or a reload; only ending the browser
//     session clears it.
//
//     `gateActive` is presentation-only — it is NEVER an access-control
//     signal. The server `401` remains the sole enforcement (a plan
//     prohibition, carried forward verbatim from 14-03).
//
// Every sessionStorage access is guarded in BOTH directions, and a later
// reader must not simplify either guard away. The `typeof` probe sits INSIDE
// the `try` in both helpers so ONE `catch` covers all three failure modes:
//   - the identifier is undefined, because `react-router build` runs with
//     `ssr: false` but still evaluates `root.tsx` — and transitively this
//     module — inside Node to emit `index.html`, where `sessionStorage` does
//     not exist; a bare module-scope dereference there is a build-time
//     `ReferenceError` that breaks the Docker image build (the Dockerfile
//     builds `web/` itself). `typeof` on an undeclared identifier does not
//     throw, so this branch returns cleanly whether the probe is inside the
//     `try` or not;
//   - `getItem` / `setItem` throw, because a browser can deny or throw on
//     storage access (private mode, disabled storage);
//   - the `sessionStorage` property ACCESSOR itself throws on read — a
//     sandboxed `<iframe>` without `allow-same-origin`, or a browser with
//     storage disabled by policy. `typeof` does NOT suppress a
//     present-but-throwing getter, so this is exactly why the probe must be
//     inside the `try` (14-VERIFICATION residual WR-01).
// An unguarded read at module scope in any of these cases would white-screen
// the whole SPA, since `root.tsx` imports this module at the top level.
//
// Both `mark*` functions set `gateActive` to `true`, because either signal
// proves the instance is gated. They are convergent and safe to call
// repeatedly with the same argument — several simultaneous `401`s all land
// on one consistent state (GATE-05 concurrency edge). Calling a `mark`
// function always notifies subscribers, even when the state did not change,
// so an idempotent repeat is observable but harmless.
import { useSyncExternalStore } from "react"

// The single sessionStorage key this module owns. Named after the
// `dt_session` cookie so the two Phase 14 browser-side artefacts read as
// one family. GATE_ACTIVE_STORAGE_VALUE is the one fixed literal ever
// written under it — kept beside the key so the write and the comparison
// cannot drift apart.
const GATE_ACTIVE_STORAGE_KEY = "dt_gate_active"
const GATE_ACTIVE_STORAGE_VALUE = "1"

// readPersistedGateActive is the ONLY place the session store is read, used
// exactly once by the module initialiser below. It returns `false` for
// every failure mode and never throws, logs, or re-raises.
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

// persistGateActive is the ONLY place the session store is written, called
// by both `mark*` functions. It swallows every failure and never throws.
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
// The G-14-2 fix: seed from the browser session store instead of a literal
// `false`, so a document reload observes what a previous page load learned.
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

  // markGateActive is the "gated, and this browser already holds a valid
  // cookie" path — driven by `apiFetch` latching the server's
  // `X-Instance-Gated` marker (G-14-3). It differs from the two `mark*`
  // functions above in two deliberate ways:
  //
  //   1. It NEVER reads or writes `authed`. A gated response proves the
  //      instance is gated, never that the caller is authorized; an in-flight
  //      response resolving AFTER a `401` has already flipped `authed` false
  //      must not resurrect a dead session (D-16).
  //   2. It early-returns once the gate is already recorded. The two `mark*`
  //      functions fire on discrete auth events and always notify; this one is
  //      evaluated on EVERY API response, so an unconditional `notify()` plus
  //      an unconditional synchronous storage write would run on every request
  //      for the life of the session. The early return is scoped to this
  //      method ONLY and must not be back-ported to `markAuthenticated` /
  //      `markUnauthenticated`.
  //
  // Like `gateActive` itself, this latch is one-way and monotonic within the
  // browser session — it never clears. `gateActive` stays presentation-only;
  // the server `401` remains the sole enforcement.
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

// useAuthed / useGateActive expose the two signals to React via
// useSyncExternalStore. `authStore.isAuthed` / `authStore.isGateActive` are
// reused as both the client and the server snapshot: they return the cached
// module boolean and must keep doing so — a snapshot getter that re-read an
// external store on every call would break the hook's contract. The session
// store is read once, at module load, and on nothing else.
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
