import { type RouteConfig, index, route } from "@react-router/dev/routes"

// D-01: two tabs/routes, "Watchlist" and "History". This task registers
// only the History route (index at "/" plus a stable "/history" URL for
// the tab bar's link) -- plan 06-03 adds "/watchlist" alongside it. Both
// entries point at the same file, so each needs an explicit, distinct id
// -- React Router derives an id from the file path by default, and two
// routes backed by one file would otherwise collide.
export default [
  index("routes/history.tsx", { id: "history-index" }),
  route("history", "routes/history.tsx", { id: "history-path" }),
] satisfies RouteConfig
