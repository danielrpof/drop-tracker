import {
  Links,
  Meta,
  NavLink,
  Outlet,
  Scripts,
  ScrollRestoration,
  isRouteErrorResponse,
} from "react-router"

import { Toaster } from "~/components/ui/sonner"

import type { Route } from "./+types/root"
import "./app.css"

// D-13: dark theme is this app's only theme (no light/dark toggle exists
// or is planned), so class="dark" is applied unconditionally rather than
// gated behind a theme provider.
export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body className="min-h-screen bg-background text-foreground">
        {children}
        <Toaster theme="dark" />
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  )
}

// tabLinkClassName styles each nav item as a tab: the indigo accent
// underlines only the active tab (06-UI-SPEC.md Color: "active tab
// underline/indicator" is one of the three reserved uses of the accent).
function tabLinkClassName({ isActive }: { isActive: boolean }) {
  return [
    "px-4 py-3 text-body font-semibold border-b-2 transition-colors",
    isActive
      ? "border-accent-indigo text-foreground"
      : "border-transparent text-muted-foreground hover:text-foreground",
  ].join(" ")
}

// App is the root layout: D-01's two-tab bar (Watchlist, History) above the
// routed page content. The two tabs stay fully decoupled per D-03/D-04 --
// neither route's component references the other's data. 
export default function App() {
  return (
    <div className="flex min-h-screen flex-col">
      <nav className="flex border-b border-border px-8">
        <NavLink to="/" className={tabLinkClassName}>
          Watchlist
        </NavLink>
        <NavLink to="/history" className={tabLinkClassName}>
          History
        </NavLink>
      </nav>
      <main className="flex-1">
        <Outlet />
      </main>
    </div>
  )
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let message = "Oops!"
  let details = "An unexpected error occurred."
  let stack: string | undefined

  if (isRouteErrorResponse(error)) {
    message = error.status === 404 ? "404" : "Error"
    details =
      error.status === 404
        ? "The requested page could not be found."
        : error.statusText || details
  } else if (import.meta.env.DEV && error && error instanceof Error) {
    details = error.message
    stack = error.stack
  }

  return (
    <main className="container mx-auto p-4 pt-16">
      <h1>{message}</h1>
      <p>{details}</p>
      {stack && (
        <pre className="w-full overflow-x-auto p-4">
          <code>{stack}</code>
        </pre>
      )}
    </main>
  )
}
