import { useState } from "react"

import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import {
  ApiError,
  updateDigestSettings,
  type NotificationSettings,
} from "~/lib/api"

import { DigestSettings } from "./DigestSettings"

// Partial mock keeps the real ApiError class intact -- a bare
// vi.mock("~/lib/api") automock would erase it, silently breaking the 401
// branch's instanceof check.
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  updateDigestSettings: vi.fn(),
}))

const mockUpdateDigestSettings = vi.mocked(updateDigestSettings)

function makeSettings(
  overrides: Partial<NotificationSettings> = {}
): NotificationSettings {
  return {
    digest_enabled: false,
    digest_cadence: "daily",
    digest_last_sent_at: null,
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  }
}

// The base-ui Switch renders aria-disabled rather than the native `disabled`
// attribute, so jest-dom's toBeDisabled() (which only recognises form
// elements) doesn't apply here.
function isDisabled(el: HTMLElement) {
  return el.getAttribute("aria-disabled") === "true"
}

// ControlledDigestSettings feeds a successful onSaved payload back in as the
// next `settings` prop, the way system.tsx's own setDigest does -- a bare
// render() with a static prop and a spy onSaved can never show the
// post-save rendered value, since DigestSettings itself never reads back
// its own write.
function ControlledDigestSettings({
  initial,
  onSaved,
}: {
  initial: NotificationSettings
  onSaved?: (next: NotificationSettings) => void
}) {
  const [settings, setSettings] = useState(initial)
  return (
    <DigestSettings
      settings={settings}
      onSaved={(next) => {
        onSaved?.(next)
        setSettings(next)
      }}
    />
  )
}

describe("DigestSettings", () => {
  it("renders the card title, the Digest mode label, and Off for a defaults payload", () => {
    render(<DigestSettings settings={makeSettings()} onSaved={vi.fn()} />)

    expect(
      screen.getByRole("heading", { name: "Digest notifications" })
    ).toBeInTheDocument()
    expect(screen.getByText("Digest mode")).toBeInTheDocument()
    expect(screen.getByText("Off")).toBeInTheDocument()
    expect(
      screen.getByRole("switch", { name: "Digest mode" })
    ).not.toBeChecked()
  })

  it("saves the toggle, calling updateDigestSettings once with the full payload, then shows On and Saved.", async () => {
    const onSaved = vi.fn()
    mockUpdateDigestSettings.mockResolvedValueOnce(
      makeSettings({ digest_enabled: true })
    )

    render(
      <ControlledDigestSettings initial={makeSettings()} onSaved={onSaved} />
    )

    await userEvent.click(screen.getByRole("switch", { name: "Digest mode" }))

    expect(mockUpdateDigestSettings).toHaveBeenCalledTimes(1)
    expect(mockUpdateDigestSettings).toHaveBeenCalledWith({
      digestEnabled: true,
      digestCadence: "daily",
    })

    await screen.findByText("On")
    await screen.findByText("Saved.")
    expect(onSaved).toHaveBeenCalledWith(makeSettings({ digest_enabled: true }))
  })

  it("keeps Off on screen and shows the failure copy when the save rejects", async () => {
    mockUpdateDigestSettings.mockRejectedValueOnce(new Error("boom"))

    render(<DigestSettings settings={makeSettings()} onSaved={vi.fn()} />)

    await userEvent.click(screen.getByRole("switch", { name: "Digest mode" }))

    await screen.findByText("Couldn't save — reverted to the previous value.")
    expect(screen.getByText("Off")).toBeInTheDocument()
    expect(
      screen.getByRole("switch", { name: "Digest mode" })
    ).not.toBeChecked()
  })

  it("starts exactly one request when two interactions are dispatched in the same task", async () => {
    let resolveUpdate: (value: NotificationSettings) => void = () => {}
    mockUpdateDigestSettings.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveUpdate = resolve
        })
    )

    render(<DigestSettings settings={makeSettings()} onSaved={vi.fn()} />)

    const user = userEvent.setup()
    const toggle = screen.getByRole("switch", { name: "Digest mode" })

    // Fire both interactions before awaiting anything so the in-flight
    // window is genuinely open when the second click lands.
    void user.click(toggle)
    void user.click(toggle)

    await waitFor(() =>
      expect(mockUpdateDigestSettings).toHaveBeenCalledTimes(1)
    )

    resolveUpdate(makeSettings({ digest_enabled: true }))
    await screen.findByText("Saved.")

    expect(mockUpdateDigestSettings).toHaveBeenCalledTimes(1)
  })

  it("disables the switch while a save is in flight and re-enables it once it resolves", async () => {
    let resolveUpdate: (value: NotificationSettings) => void = () => {}
    mockUpdateDigestSettings.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveUpdate = resolve
        })
    )

    render(<DigestSettings settings={makeSettings()} onSaved={vi.fn()} />)

    const toggle = screen.getByRole("switch", { name: "Digest mode" })
    await userEvent.click(toggle)

    expect(isDisabled(toggle)).toBe(true)

    resolveUpdate(makeSettings({ digest_enabled: true }))
    await waitFor(() => expect(isDisabled(toggle)).toBe(false))
  })

  it("disables the switch while a save is in flight and re-enables it once it rejects", async () => {
    let rejectUpdate: (err: unknown) => void = () => {}
    mockUpdateDigestSettings.mockImplementationOnce(
      () =>
        new Promise((_resolve, reject) => {
          rejectUpdate = reject
        })
    )

    render(<DigestSettings settings={makeSettings()} onSaved={vi.fn()} />)

    const toggle = screen.getByRole("switch", { name: "Digest mode" })
    await userEvent.click(toggle)

    expect(isDisabled(toggle)).toBe(true)

    rejectUpdate(new Error("boom"))
    await waitFor(() => expect(isDisabled(toggle)).toBe(false))
  })

  it("swallows a 401 without writing the failure state, mirroring system.tsx's own catch", async () => {
    mockUpdateDigestSettings.mockRejectedValueOnce(
      new ApiError(401, "unauthenticated")
    )

    render(<DigestSettings settings={makeSettings()} onSaved={vi.fn()} />)

    await userEvent.click(screen.getByRole("switch", { name: "Digest mode" }))

    await waitFor(() =>
      expect(
        isDisabled(screen.getByRole("switch", { name: "Digest mode" }))
      ).toBe(false)
    )
    expect(
      screen.queryByText("Couldn't save — reverted to the previous value.")
    ).not.toBeInTheDocument()
  })
})
