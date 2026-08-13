// The single seam for router-context rendering (D-03, ROADMAP success
// criterion 4). Tests that need to render a component inside React
// Router's context import `renderRoute` from here rather than wrapping a
// router themselves -- do not add a memory-router wrapper of your own;
// `createRoutesStub` is React Router's own documented testing primitive
// for exactly this.
import type { ComponentType } from "react"

import { render } from "@testing-library/react"
import { createRoutesStub } from "react-router"

export function renderRoute(Component: ComponentType, path: string) {
  const Stub = createRoutesStub([{ path, Component }])
  return render(<Stub initialEntries={[path]} />)
}
