import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

// authStore holds module-level singleton state, so the module registry is
// reset before every case and re-imported fresh -- test ordering can never
// make an assertion pass by leaking a prior case's mutation.
let authStore: (typeof import("./authStore"))["authStore"]

// The literal contract, pinned from the test side so a change to what the
// implementation writes is caught here as well as there.
const GATE_ACTIVE_STORAGE_KEY = "dt_gate_active"
const GATE_ACTIVE_STORAGE_VALUE = "1"

// A simulated document reload: reset the registry and re-import so the
// module initialiser runs again. Seed sessionStorage BEFORE calling this --
// the initialiser reads the store at import time.
async function reimportStore() {
  vi.resetModules()
  return (await import("./authStore")).authStore
}

beforeEach(async () => {
  // Storage reset is load-bearing, not hygiene, and MUST be the first
  // statement -- before vi.resetModules() and before the dynamic import.
  // jsdom keeps one sessionStorage for the whole test file and
  // vi.resetModules() does not clear it; the module reads the store at
  // import time, so a case that recorded the gate would otherwise leak into
  // "starts optimistically authenticated with the gate inactive on a fresh
  // load".
  sessionStorage.clear()
  vi.resetModules()
  ;({ authStore } = await import("./authStore"))
})

afterEach(() => {
  // Restore any sessionStorage global replaced by vi.stubGlobal in the
  // hostile-storage cases below.
  vi.unstubAllGlobals()
})

describe("authStore", () => {
  it("starts optimistically authenticated with the gate inactive on a fresh load", () => {
    expect(authStore.isAuthed()).toBe(true)
    expect(authStore.isGateActive()).toBe(false)
  })

  it("markUnauthenticated sets authed false, activates the gate, and notifies subscribers", () => {
    const listener = vi.fn()
    authStore.subscribe(listener)

    authStore.markUnauthenticated()

    expect(authStore.isAuthed()).toBe(false)
    expect(authStore.isGateActive()).toBe(true)
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("markAuthenticated sets authed true, activates the gate, and notifies subscribers", () => {
    const listener = vi.fn()
    authStore.subscribe(listener)
    authStore.markUnauthenticated()
    listener.mockClear()

    authStore.markAuthenticated()

    expect(authStore.isAuthed()).toBe(true)
    expect(authStore.isGateActive()).toBe(true)
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("is idempotent: calling markUnauthenticated twice notifies each time but converges on one consistent state", () => {
    const listener = vi.fn()
    authStore.subscribe(listener)

    authStore.markUnauthenticated()
    authStore.markUnauthenticated()

    expect(listener).toHaveBeenCalledTimes(2)
    expect(authStore.isAuthed()).toBe(false)
    expect(authStore.isGateActive()).toBe(true)
  })

  it("gateActive stays true after markAuthenticated once the gate has been observed", () => {
    authStore.markUnauthenticated()
    authStore.markAuthenticated()

    expect(authStore.isGateActive()).toBe(true)
  })

  it("subscribe returns an unsubscribe function that stops further notifications", () => {
    const listener = vi.fn()
    const unsubscribe = authStore.subscribe(listener)

    authStore.markUnauthenticated()
    expect(listener).toHaveBeenCalledTimes(1)

    unsubscribe()
    authStore.markAuthenticated()
    expect(listener).toHaveBeenCalledTimes(1)
  })
})

describe("authStore — gateActive survives a browser-session reload (D-18)", () => {
  it("reports isGateActive() true on the first read after a reload that recorded the gate, with no mark* call", async () => {
    // A reload with the flag already recorded: the browser session learned
    // the instance is gated on a previous page load.
    sessionStorage.setItem(GATE_ACTIVE_STORAGE_KEY, GATE_ACTIVE_STORAGE_VALUE)

    const reloaded = await reimportStore()

    expect(reloaded.isGateActive()).toBe(true)
    // authed is still the optimistic true -- D-16 is unchanged, it is never
    // seeded from storage.
    expect(reloaded.isAuthed()).toBe(true)
  })

  it("still reports isGateActive() false on a fresh browser session with an empty session store", async () => {
    const reloaded = await reimportStore()

    expect(reloaded.isGateActive()).toBe(false)
  })

  it("markUnauthenticated writes the gate-active flag through to the session store so a later reload sees it", async () => {
    authStore.markUnauthenticated()

    expect(sessionStorage.getItem(GATE_ACTIVE_STORAGE_KEY)).toBe(
      GATE_ACTIVE_STORAGE_VALUE
    )

    const reloaded = await reimportStore()
    expect(reloaded.isGateActive()).toBe(true)
  })

  it("markAuthenticated writes the gate-active flag through to the session store so a later reload sees it", async () => {
    authStore.markAuthenticated()

    expect(sessionStorage.getItem(GATE_ACTIVE_STORAGE_KEY)).toBe(
      GATE_ACTIVE_STORAGE_VALUE
    )

    const reloaded = await reimportStore()
    expect(reloaded.isGateActive()).toBe(true)
  })

  it("is monotonic within a session: a login then a logout-style markUnauthenticated still reports isGateActive() true after a reload", async () => {
    authStore.markAuthenticated()
    authStore.markUnauthenticated()

    const reloaded = await reimportStore()
    expect(reloaded.isGateActive()).toBe(true)
  })

  it("does not throw and leaves the gate inactive when sessionStorage is undefined", async () => {
    vi.stubGlobal("sessionStorage", undefined)

    const reloaded = await reimportStore()
    expect(reloaded.isGateActive()).toBe(false)

    const listener = vi.fn()
    reloaded.subscribe(listener)

    expect(() => reloaded.markUnauthenticated()).not.toThrow()
    expect(() => reloaded.markAuthenticated()).not.toThrow()

    expect(reloaded.isAuthed()).toBe(true)
    expect(reloaded.isGateActive()).toBe(true)
    expect(listener).toHaveBeenCalledTimes(2)
  })

  it("does not throw and leaves the gate inactive when sessionStorage getItem/setItem throw", async () => {
    vi.stubGlobal("sessionStorage", {
      getItem: () => {
        throw new Error("storage access denied")
      },
      setItem: () => {
        throw new Error("storage access denied")
      },
    })

    const reloaded = await reimportStore()
    expect(reloaded.isGateActive()).toBe(false)

    const listener = vi.fn()
    reloaded.subscribe(listener)

    expect(() => reloaded.markUnauthenticated()).not.toThrow()
    expect(() => reloaded.markAuthenticated()).not.toThrow()

    expect(reloaded.isGateActive()).toBe(true)
    expect(listener).toHaveBeenCalledTimes(2)
  })
})
