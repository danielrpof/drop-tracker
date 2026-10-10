// The single seam for router-context rendering (D-03, ROADMAP success
// criterion 4). Tests that need to render a component inside React
// Router's context import `renderRoute` from here rather than wrapping a
// router themselves -- do not add a memory-router wrapper of your own;
// `createRoutesStub` is React Router's own documented testing primitive
// for exactly this.
import type { ComponentType } from "react"

import { render } from "@testing-library/react"
import { createRoutesStub, useLocation } from "react-router"

export function renderRoute(
  Component: ComponentType,
  path: string,
  initialEntry: string = path
) {
  const Stub = createRoutesStub([{ path, Component }])
  return render(<Stub initialEntries={[initialEntry]} />)
}

// Exposes the current query string so tests can assert URL writes.
export function LocationProbe() {
  return <output data-testid="location-search">{useLocation().search}</output>
}
