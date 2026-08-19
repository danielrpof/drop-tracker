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
})
