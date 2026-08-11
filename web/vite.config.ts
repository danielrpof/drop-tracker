import { reactRouter } from "@react-router/dev/vite"
import tailwindcss from "@tailwindcss/vite"
import { defineConfig } from "vite"

export default defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [tailwindcss(), reactRouter()],
  server: {
    // 06-RESEARCH.md Pitfall 4: in dev, `react-router dev` runs its own
    // server on this Vite dev server's port, separate from the Go API's
    // HTTP_PORT (internal/config/config.go default 8080) -- proxying keeps
    // dev-mode fetch() calls same-origin from the browser's perspective, so
    // production (where the SPA is genuinely same-origin, served by the Go
    // binary) never needs CORS middleware at all.
    proxy: {
      "/health": "http://localhost:8080",
      "/search": "http://localhost:8080",
      "/watchlist": "http://localhost:8080",
      "/events": "http://localhost:8080",
    },
  },
})
