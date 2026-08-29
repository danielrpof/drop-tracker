import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { ComponentType } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"

// Partial mock: createSession/deleteSession become controllable spies (so
// no real apiFetch ever reaches the runtime's fetch -- TEST-02), but the
// real ApiError class is kept so `err instanceof ApiError` and `err.status`
// work in the component's catch branch. The module registry is reset per
// case because authStore is a singleton PassphraseScreen imports -- a fresh
// import keeps the component and the test pointing at one instance.
vi.mock("~/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/api")>()
  return { ...actual, createSession: vi.fn(), deleteSession: vi.fn() }
})

let PassphraseScreen: ComponentType
let mockCreateSession: ReturnType<typeof vi.fn>
let authStore: (typeof import("~/lib/authStore"))["authStore"]
let ApiError: (typeof import("~/lib/api"))["ApiError"]

beforeEach(async () => {
  vi.resetModules()
  const api = await import("~/lib/api")
  ApiError = api.ApiError
  mockCreateSession = vi.mocked(api.createSession)
  ;({ authStore } = await import("~/lib/authStore"))
  ;({ default: PassphraseScreen } = await import(
    "~/components/auth/PassphraseScreen"
  ))
  // The gate is only shown when the store is unauthenticated.
  authStore.markUnauthenticated()
})

function getField() {
  return screen.getByLabelText("Passphrase") as HTMLInputElement
}

function getUnlockButton() {
  return screen.getByRole("button")
}

describe("PassphraseScreen", () => {
  it("renders the fixed heading and body copy", () => {
    render(<PassphraseScreen />)

    expect(
      screen.getByRole("heading", { name: "Enter the instance passphrase" })
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        "This drop-tracker instance is private. Enter the passphrase to view the watchlist and release history."
      )
    ).toBeInTheDocument()
  })

  it("renders a password field with no placeholder that takes focus on mount", async () => {
    render(<PassphraseScreen />)

    const field = getField()
    expect(field).toHaveAttribute("type", "password")
    expect(field).not.toHaveAttribute("placeholder")
    await waitFor(() => expect(field).toHaveFocus())
  })

  it("shows the Unlock button enabled with an empty field", () => {
    render(<PassphraseScreen />)

    const button = getUnlockButton()
    expect(button).toHaveTextContent("Unlock")
    expect(button).toBeEnabled()
  })

  it("submits the exact typed value and, on resolve, marks the store authenticated", async () => {
    mockCreateSession.mockResolvedValueOnce(undefined)
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "correct-horse")
    await userEvent.click(getUnlockButton())

    await waitFor(() => expect(authStore.isAuthed()).toBe(true))
    expect(mockCreateSession).toHaveBeenCalledWith("correct-horse")
  })

  it("disables the input and shows the Unlocking… label while the request is in flight", async () => {
    let resolve: () => void = () => {}
    mockCreateSession.mockReturnValueOnce(
      new Promise<void>((r) => {
        resolve = r
      })
    )
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "pw")
    await userEvent.click(getUnlockButton())

    await waitFor(() => expect(getUnlockButton()).toHaveTextContent("Unlocking…"))
    expect(getUnlockButton()).toBeDisabled()
    expect(getField()).toBeDisabled()
    expect(getField()).toHaveValue("pw")

    resolve()
    await waitFor(() => expect(authStore.isAuthed()).toBe(true))
  })

  it("on a 401 shows the wrong-passphrase copy, keeps the store unauthenticated, retains the value, and keeps focus", async () => {
    mockCreateSession.mockRejectedValueOnce(new ApiError(401, "unauthenticated"))
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "wrong-pass")
    await userEvent.click(getUnlockButton())

    await screen.findByText("That passphrase isn't correct. Check it and try again.")
    expect(authStore.isAuthed()).toBe(false)
    expect(getField()).toHaveValue("wrong-pass")
    await waitFor(() => expect(getField()).toHaveFocus())
  })

  it("on a 429 shows the throttle copy", async () => {
    mockCreateSession.mockRejectedValueOnce(new ApiError(429, "rate limited"))
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "pw")
    await userEvent.click(getUnlockButton())

    await screen.findByText(
      "Too many attempts. Wait about a minute, then try again."
    )
  })

  // 14-UI-SPEC E4 held-out backstop check: the network / 5xx path is the
  // least-exercised branch. A thrown rejection with no status (what a real
  // fetch failure produces) must select the connection copy -- NOT the 401
  // copy -- and the form must stay usable: the button re-enables immediately.
  it("on a network failure shows the connection copy and re-enables the button immediately (E4 backstop)", async () => {
    mockCreateSession.mockRejectedValueOnce(new Error("network down"))
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "pw")
    await userEvent.click(getUnlockButton())

    await screen.findByText(
      "Couldn't reach the server. Check your connection and try again."
    )
    expect(
      screen.queryByText("That passphrase isn't correct. Check it and try again.")
    ).not.toBeInTheDocument()
    expect(getUnlockButton()).toBeEnabled()
    expect(getUnlockButton()).toHaveTextContent("Unlock")
  })

  it("also shows the connection copy on a 5xx status", async () => {
    mockCreateSession.mockRejectedValueOnce(new ApiError(503, "unavailable"))
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "pw")
    await userEvent.click(getUnlockButton())

    await screen.findByText(
      "Couldn't reach the server. Check your connection and try again."
    )
    expect(getUnlockButton()).toBeEnabled()
  })

  it("after a 401 keeps the button disabled until the field is edited, then reverts to Unlock", async () => {
    mockCreateSession.mockRejectedValueOnce(new ApiError(401, "unauthenticated"))
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "wrong")
    await userEvent.click(getUnlockButton())
    await screen.findByText("That passphrase isn't correct. Check it and try again.")

    expect(getUnlockButton()).toBeDisabled()

    await userEvent.type(getField(), "x")

    expect(getUnlockButton()).toBeEnabled()
    expect(getUnlockButton()).toHaveTextContent("Unlock")
    expect(
      screen.queryByText("That passphrase isn't correct. Check it and try again.")
    ).not.toBeInTheDocument()
  })

  it("clears the previous error message at the start of the next submit", async () => {
    mockCreateSession.mockRejectedValueOnce(new Error("network down"))
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "pw")
    await userEvent.click(getUnlockButton())
    await screen.findByText(
      "Couldn't reach the server. Check your connection and try again."
    )

    mockCreateSession.mockResolvedValueOnce(undefined)
    await userEvent.click(getUnlockButton())

    await waitFor(() =>
      expect(
        screen.queryByText(
          "Couldn't reach the server. Check your connection and try again."
        )
      ).not.toBeInTheDocument()
    )
    expect(authStore.isAuthed()).toBe(true)
  })

  it("never renders the submitted value as readable text (D-13)", async () => {
    mockCreateSession.mockRejectedValueOnce(new ApiError(401, "unauthenticated"))
    render(<PassphraseScreen />)

    await userEvent.type(getField(), "sup3rSecretValue")
    await userEvent.click(getUnlockButton())
    await screen.findByText("That passphrase isn't correct. Check it and try again.")

    expect(screen.queryByText("sup3rSecretValue")).not.toBeInTheDocument()
    // The value is still held for correction -- just never as a text node.
    expect(getField()).toHaveValue("sup3rSecretValue")
  })
})
