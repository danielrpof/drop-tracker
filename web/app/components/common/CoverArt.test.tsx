import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { CoverArt } from "./CoverArt"

describe("CoverArt", () => {
  it("clears the failed placeholder when src changes", async () => {
    const { rerender } = render(
      <CoverArt src="https://example.invalid/broken.png" alt="Test Artist" />
    )

    // Real <img> carries alt -- the placeholder div carries aria-label, not
    // alt, so getByAltText unambiguously selects the image element.
    fireEvent.error(screen.getByAltText("Test Artist"))

    // After onError, the placeholder replaces the <img> -- no alt-text
    // element remains.
    await waitFor(() => {
      expect(screen.queryByAltText("Test Artist")).toBeNull()
    })

    rerender(
      <CoverArt src="https://example.invalid/works.png" alt="Test Artist" />
    )

    // D-01's useEffect fires post-commit -- assert through waitFor, never
    // synchronously right after rerender (12-RESEARCH.md Pitfall 1: a
    // synchronous assertion here reads the stale failed=true state).
    await waitFor(() => {
      expect(screen.getByAltText("Test Artist")).toHaveAttribute(
        "src",
        "https://example.invalid/works.png"
      )
    })
  })

  it("keeps the placeholder when src stays the same after a load error", async () => {
    const { rerender } = render(
      <CoverArt
        src="https://example.invalid/broken.png"
        alt="Same Src Artist"
      />
    )

    fireEvent.error(screen.getByAltText("Same Src Artist"))

    await waitFor(() => {
      expect(screen.queryByAltText("Same Src Artist")).toBeNull()
    })

    rerender(
      <CoverArt
        src="https://example.invalid/broken.png"
        alt="Same Src Artist"
      />
    )

    // D-01's effect must not re-fire when the dependency (src) is
    // unchanged -- an unchanged src should never re-show a known-broken
    // image. Asserted through waitFor for consistency with Pitfall 1's
    // rule, so a spurious reset would be caught rather than raced past.
    await waitFor(() => {
      expect(screen.queryByAltText("Same Src Artist")).toBeNull()
    })
  })

  it("renders the placeholder when src is null", () => {
    render(<CoverArt src={null} alt="No Art Artist" />)

    expect(screen.queryByAltText("No Art Artist")).toBeNull()
    expect(
      screen.getByRole("img", { name: "No Art Artist" })
    ).toBeInTheDocument()
  })
})
