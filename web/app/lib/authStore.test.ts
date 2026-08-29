import { beforeEach, describe, expect, it, vi } from "vitest"

// authStore holds module-level singleton state, so the module registry is
// reset before every case and re-imported fresh -- test ordering can never
// make an assertion pass by leaking a prior case's mutation.
let authStore: (typeof import("./authStore"))["authStore"]

beforeEach(async () => {
  vi.resetModules()
  ;({ authStore } = await import("./authStore"))
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
