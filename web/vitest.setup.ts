// Registers @testing-library/jest-dom's DOM matchers (toBeInTheDocument,
// toBeChecked, toHaveAttribute, ...) for every test file, and (because
// tsconfig's include already covers this file) supplies their TypeScript
// augmentation to tsc.
import "@testing-library/jest-dom/vitest"

import { cleanup } from "@testing-library/react"
import { afterEach } from "vitest"

// @testing-library/react's auto-cleanup only self-registers when it detects
// a global `afterEach` (Jest-style globals). This config imports test APIs
// explicitly rather than enabling Vitest's `test.globals` option, so
// cleanup must be wired by hand -- without it, each test's rendered tree
// accumulates in the jsdom document and later `getByRole` queries match
// multiple elements across tests.
afterEach(() => {
  cleanup()
})
