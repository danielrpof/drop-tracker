import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { classifyOutcome, OutcomeBadge } from "./OutcomeBadge"

describe("classifyOutcome", () => {
  it("classifies an ok run with zero errored artists as Success", () => {
    const tier = classifyOutcome({ outcome: "ok", artists_errored: 0 })
    expect(tier.label).toBe("Success")
    expect(tier.className).toContain("bg-status-ok")
  })

  it("classifies an unrecognised outcome with a title-cased fallback, not the raw value", () => {
    const tier = classifyOutcome({ outcome: "degraded", artists_errored: 0 })
    expect(tier.label).not.toBe("degraded")
    expect(tier.label).toBe("Degraded")
    expect(tier.variant).toBe("secondary")
  })

  it("classifies an empty-string outcome with a non-empty fallback label", () => {
    const tier = classifyOutcome({ outcome: "", artists_errored: 0 })
    expect(tier.label).toBe("Unknown")
    expect(tier.label.length).toBeGreaterThan(0)
  })
})

describe("OutcomeBadge", () => {
  it("renders the Success tier, green status-ok tinted fill", () => {
    render(<OutcomeBadge run={{ outcome: "ok", artists_errored: 0 }} />)
    const badge = screen.getByText("Success")
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveClass("bg-status-ok/15")
  })

  it("renders the Completed with errors tier, amber status-warn, no leading icon", () => {
    const { container } = render(
      <OutcomeBadge run={{ outcome: "ok", artists_errored: 3 }} />
    )
    const badge = screen.getByText("Completed with errors")
    expect(badge).toHaveClass("bg-status-warn/15")
    expect(container.querySelector("svg")).not.toBeInTheDocument()
  })

  it("renders the Failed tier using the badge's destructive variant", () => {
    render(<OutcomeBadge run={{ outcome: "error", artists_errored: 0 }} />)
    expect(screen.getByText("Failed")).toBeInTheDocument()
  })

  it("renders the Interrupted tier, amber status-warn, with a leading icon distinguishing it from Completed with errors", () => {
    const { container } = render(
      <OutcomeBadge run={{ outcome: "cancelled", artists_errored: 0 }} />
    )
    const badge = screen.getByText("Interrupted")
    expect(badge).toHaveClass("bg-status-warn/15")
    expect(container.querySelector("svg")).toBeInTheDocument()
  })

  it("renders an unrecognised outcome as a grey badge with a title-cased fallback label, never the raw value", () => {
    render(<OutcomeBadge run={{ outcome: "degraded", artists_errored: 0 }} />)
    expect(screen.getByText("Degraded")).toBeInTheDocument()
    expect(screen.queryByText("degraded")).not.toBeInTheDocument()
  })

  it("renders every tier, including the unrecognised one, without throwing", () => {
    const cases: Array<{ outcome: string; artists_errored: number }> = [
      { outcome: "ok", artists_errored: 0 },
      { outcome: "ok", artists_errored: 2 },
      { outcome: "error", artists_errored: 0 },
      { outcome: "cancelled", artists_errored: 0 },
      { outcome: "some_future_value", artists_errored: 0 },
      { outcome: "", artists_errored: 0 },
    ]

    for (const run of cases) {
      expect(() => render(<OutcomeBadge run={run} />)).not.toThrow()
    }
  })
})
