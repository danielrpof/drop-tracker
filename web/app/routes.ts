import { type RouteConfig, index, route } from "@react-router/dev/routes"

// D-01: two tabs/routes, "Watchlist" and "History". The Watchlist tab is
// this app's landing route -- an operator with an empty history starts
// there, not on an empty History feed -- so it is registered as both the
// index route and a stable "/watchlist" URL for the tab bar's link,
// mirroring 06-01's index+history-index pattern. Both watchlist entries
// point at the same file, so each needs an explicit, distinct id -- React
// Router derives an id from the file path by default, and two routes
// backed by one file would otherwise collide.
export default [
  index("routes/watchlist.tsx", { id: "watchlist-index" }),
  route("watchlist", "routes/watchlist.tsx", { id: "watchlist-path" }),
  route("history", "routes/history.tsx", { id: "history-path" }),
] satisfies RouteConfig
