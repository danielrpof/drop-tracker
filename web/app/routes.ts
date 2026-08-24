import { type RouteConfig, index, route } from "@react-router/dev/routes"

// D-01: two tabs/routes, "Watchlist" and "History". The Watchlist tab is
// this app's landing route -- an operator with an empty history starts
// there, not on an empty History feed. The API already owns /watchlist, so
// the UI deliberately uses the root path to keep browser refreshes on the
// SPA instead of returning the watchlist JSON response.
export default [
  index("routes/watchlist.tsx", { id: "watchlist-index" }),
  route("history", "routes/history.tsx", { id: "history-path" }),
] satisfies RouteConfig
