// Standalone Vitest config: never imports from, merges with, or references
// web/vite.config.ts, and never loads the React Router Vite dev plugin --
// that plugin does virtual-module resolution that only works inside
// `react-router dev`/`build`, not Vitest's own Vite instance (08-RESEARCH.md
// Pitfall 1).
import path from "node:path"
import { defineConfig } from "vitest/config"

export default defineConfig({
  resolve: {
    // Replicates tsconfig.json's one path alias ("~/*" -> "./app/*") -- a
    // manual alias is sufficient for this project's single mapping, so
    // vite-tsconfig-paths is not pulled in as a dependency.
    alias: { "~": path.resolve(__dirname, "./app") },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
    // mockReset is load-bearing, not hygiene: every test file sets a
    // per-test resolved/rejected value on an auto-mocked ~/lib/api (D-06).
    // Without a per-test reset, one test's queued mock value would silently
    // satisfy the next test's expectation.
    mockReset: true,
    // The "zero test files still passes" escape hatch is deliberately left
    // unset here -- a run that discovers no test files must exit non-zero,
    // so the suite can never report a vacuous green.
  },
})
