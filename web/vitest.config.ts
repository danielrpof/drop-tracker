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
    coverage: {
      // Resolved empirically (09-RESEARCH.md Open Question 1): config
      // presence alone does NOT activate coverage collection on
      // vitest@4.1.10 -- a bare `pnpm test` printed no coverage table until
      // this flag was added. Kept here (not a --coverage CLI flag or a
      // package.json script change) so `pnpm test` stays a single unchanged
      // `vitest run` invocation per D-08.
      enabled: true,
      provider: "v8",
      // Text reporter only -- log-only output, writes nothing to disk
      // (mirrors the backend's D-03 posture).
      reporter: ["text"],
      // Load-bearing, not boilerplate: Vitest 4 removed coverage.all, so
      // only files imported during the test run are measured by default.
      // Without this glob, an entirely untested route (app/routes/history.tsx)
      // and the API module every test mocks out (app/lib/api.ts) would never
      // enter the denominator at all (09-RESEARCH.md Pitfall 3).
      include: ["app/**/*.{ts,tsx}"],
      exclude: [
        "app/components/ui/**", // shadcn primitives, not first-party (D-06)
        "app/lib/test/**", // test-only helpers (routeStub.tsx)
        "**/*.test.{ts,tsx}", // the test files themselves
      ],
    },
  },
})
