import type { Config } from "@react-router/dev/config"

export default {
  // Config options...
  // 06-RESEARCH.md Pattern 1 / Pitfall 1: this project has no Node runtime
  // in production (CLAUDE.md: single Go binary, go:embed-served SPA), so
  // ssr must be false -- otherwise `react-router build` also emits a
  // build/server/ Node server bundle nothing in this architecture ever
  // runs.
  ssr: false,
} satisfies Config
