import { useState } from "react"

import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import {
  ApiError,
  updateDigestSettings,
  type NotificationSettings,
} from "~/lib/api"
import { formatAbsoluteTime, formatIsoTitle } from "~/lib/format"

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

  it("renders the Cadence row and saves both fields when a new cadence is chosen", async () => {
    const onSaved = vi.fn()
    mockUpdateDigestSettings.mockResolvedValueOnce(
      makeSettings({ digest_enabled: true, digest_cadence: "weekly" })
    )

    render(
      <ControlledDigestSettings
        initial={makeSettings({
          digest_enabled: true,
          digest_cadence: "daily",
        })}
        onSaved={onSaved}
      />
    )

    expect(screen.getByText("Cadence")).toBeInTheDocument()
    const trigger = screen.getByRole("combobox", { name: "Cadence" })
    expect(trigger).toHaveTextContent("Daily")

    await userEvent.click(trigger)
    await userEvent.click(await screen.findByRole("option", { name: "Weekly" }))

    expect(mockUpdateDigestSettings).toHaveBeenCalledTimes(1)
    expect(mockUpdateDigestSettings).toHaveBeenCalledWith({
      digestEnabled: true,
      digestCadence: "weekly",
    })
    await waitFor(() => expect(trigger).toHaveTextContent("Weekly"))
    expect(onSaved).toHaveBeenCalledWith(
      makeSettings({ digest_enabled: true, digest_cadence: "weekly" })
    )
  })

  it("keeps the cadence control visible, disabled, and populated with the stored value when digest mode is off", () => {
    render(
      <DigestSettings
        settings={makeSettings({
          digest_enabled: false,
          digest_cadence: "weekly",
        })}
        onSaved={vi.fn()}
      />
    )

    const trigger = screen.getByRole("combobox", { name: "Cadence" })
    expect(trigger).toBeInTheDocument()
    expect(trigger).toBeDisabled()
    expect(trigger).toHaveTextContent("Weekly")
  })

  it("does not change the displayed cadence when digest mode is flipped on", async () => {
    mockUpdateDigestSettings.mockResolvedValueOnce(
      makeSettings({ digest_enabled: true, digest_cadence: "daily" })
    )

    render(
      <ControlledDigestSettings
        initial={makeSettings({
          digest_enabled: false,
          digest_cadence: "daily",
        })}
      />
    )

    await userEvent.click(screen.getByRole("switch", { name: "Digest mode" }))

    await screen.findByText("On")
    expect(screen.getByRole("combobox", { name: "Cadence" })).toHaveTextContent(
      "Daily"
    )
  })

  it("keeps the previous cadence on screen and shows the shared failure copy when a cadence save rejects", async () => {
    mockUpdateDigestSettings.mockRejectedValueOnce(new Error("boom"))

    render(
      <DigestSettings
        settings={makeSettings({
          digest_enabled: true,
          digest_cadence: "daily",
        })}
        onSaved={vi.fn()}
      />
    )

    const trigger = screen.getByRole("combobox", { name: "Cadence" })
    await userEvent.click(trigger)
    await userEvent.click(await screen.findByRole("option", { name: "Weekly" }))

    await screen.findByText("Couldn't save — reverted to the previous value.")
    expect(trigger).toHaveTextContent("Daily")
  })

  it("renders the literal 'Never sent yet' text and no <time> element for a null watermark", () => {
    render(
      <DigestSettings
        settings={makeSettings({ digest_last_sent_at: null })}
        onSaved={vi.fn()}
      />
    )

    expect(screen.getByText("Never sent yet")).toBeInTheDocument()
    expect(document.querySelector("time")).not.toBeInTheDocument()
  })

  it("renders a <time> element carrying the ISO instant and the shared absolute-time text for a populated watermark", () => {
    const iso = "2026-02-01T03:04:05Z"
    render(
      <DigestSettings
        settings={makeSettings({ digest_last_sent_at: iso })}
        onSaved={vi.fn()}
      />
    )

    const timeEl = document.querySelector("time")
    expect(timeEl).not.toBeNull()
    expect(timeEl).toHaveAttribute("dateTime", iso)
    expect(timeEl).toHaveAttribute("title", formatIsoTitle(iso))
    expect(timeEl).toHaveTextContent(formatAbsoluteTime(iso))
  })
})
