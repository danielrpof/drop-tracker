# Phase 8: Frontend Test Suite - Research

**Researched:** 2026-08-12
**Domain:** React component testing (Vitest + React Testing Library + jsdom) for a React Router v7 SPA-mode frontend
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Vitest + React Testing Library + jsdom, per ROADMAP.md. A separate `vitest.config.ts` is required — React Router's Vite plugin is incompatible with reusing `web/vite.config.ts` directly (already noted in ROADMAP.md's Phase 8 entry).
- **D-02:** Test files co-locate beside source (`*.test.tsx`), mirroring the Go `_test.go` convention already established in this repo (`internal/*/*_test.go`).
- **D-03:** Components needing router context render through one shared helper built on React Router's `createRoutesStub`, established once and reused — per success criterion 4, not re-litigated here.
- **D-04:** This phase adds a new job to `.github/workflows/full-pipeline.yml` that runs the Vitest suite on every push — report-only, no coverage threshold. Phase 9 then only needs to add the coverage-gate step to this same job.
- **D-05:** Floor only, per success criteria — write exactly the tests success criterion 2 names (one per surface: watchlist row, preference toggle, search, history/event-filter) proving the specific behaviors listed, plus whatever's needed to prove the 3 folded bug fixes. Do not proactively expand into every component branch — that's Phase 9's territory.
- **D-06:** `vi.mock('~/lib/api')` per test file, with `vi.mocked(fn).mockResolvedValue(...)` / `.mockRejectedValue(...)` set per test. No new shared mock-api helper module.
- **D-07:** Each folded bug (EventCard fallback badge, SearchBox AbortController, guestFeatureHref encoding) gets its own RED-then-GREEN commit pair — a test proving the bug fails against current code, then a minimal fix commit that makes it pass. Not bundled into the surface's general test-writing commit.

### Claude's Discretion

None — all discussed areas resolved to a specific choice.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TEST-01 | Frontend has a Vitest + React Testing Library test suite covering the watchlist list/row, preference-toggle, search, and history/event-filter component and route surface | Standard Stack (Vitest 4.1.10 + RTL 16.3.2 + jsdom 30.0.1), Architecture Patterns (per-surface test placement, including the route-level insight for WatchlistRow below), Code Examples |
| TEST-02 | Frontend tests mock the app's API boundary (`web/app/lib/api.ts`) rather than intercepting raw fetch/network calls | Architecture Patterns (`vi.mock('~/lib/api')` pattern), Don't Hand-Roll, Code Examples |

</phase_requirements>

## Summary

This phase adds Vitest 4 + React Testing Library 16 + jsdom 30 to a project that currently has **zero** frontend test tooling. The project's production Vite config (`web/vite.config.ts`) cannot be reused for tests because its `reactRouter()` plugin (from `@react-router/dev/vite`) is explicitly documented as dev-server/build-only and breaks under Vitest — confirmed by the React Router team and reproduced in multiple community Vitest+React-Router-v7 setups. The fix, per CONTEXT.md D-01, is a wholly separate `web/vitest.config.ts` that never imports `reactRouter()`, replicates only the one thing tests need from the production config (the `~/` → `./app` path alias), and sets `test.environment: 'jsdom'`.

The most important architectural finding from reading the actual component source (not just the phase description) is that **none of the five named/folded surfaces call React Router hooks**, and **`WatchlistRow.tsx` never imports `~/lib/api` at all** — it only receives `onRemove`/`onEntryChange` callback props. Success criterion 2's example ("the watchlist row's remove control triggers the remove API call") can only be proven by rendering the `Watchlist` route component (`web/app/routes/watchlist.tsx`, which does call `removeWatchlist`) with the API boundary mocked — not by unit-testing `WatchlistRow` in isolation. This route-level render is also the natural (and, on the current codebase, only clearly-needed) place to exercise the shared `createRoutesStub` helper D-03/success-criterion-4 mandate — even though `Watchlist` itself doesn't call router hooks either, rendering it through the shared stub is cheap, matches React Router's own documented testing pattern, and gives the suite forward-compatible router-context insurance if a future phase adds `<Link>`/`useNavigate` to any of these trees.

`PreferenceToggles.tsx`, `SearchBox.tsx`, and `HistoryFilters.tsx` each import functions directly from `~/lib/api` and can be unit-tested in true isolation with a per-file `vi.mock('~/lib/api')` — no router stub needed for these three. `EventCard.tsx` only imports the `EventItem` *type* from `~/lib/api` (no API calls) — its three RED-then-GREEN bug-fix tests are pure rendering assertions with no mocking required at all.

**Primary recommendation:** Install `vitest@4.1.10`, `jsdom@30.0.1`, `@testing-library/react@16.3.2`, `@testing-library/jest-dom@7.0.1`, `@testing-library/user-event@14.6.4` as devDependencies; create `web/vitest.config.ts` (separate from `vite.config.ts`) with a manual `~` alias and `environment: 'jsdom'`; build one shared `web/app/lib/test/routeStub.tsx` helper wrapping `createRoutesStub`; mock `~/lib/api` per test file (not a shared mock module); defer `@vitest/coverage-v8` installation to Phase 9 (nothing in this phase's success criteria requires computing coverage).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Component rendering & assertions | Browser / Client (test-simulated via jsdom) | — | Vitest + jsdom simulates the DOM; no real browser or server involved |
| API boundary mocking | Browser / Client | — | `~/lib/api` is the client-side fetch wrapper; mocking it stays entirely within the frontend test process, never touches the Go API tier |
| Router context stubbing | Browser / Client | — | `createRoutesStub` is a React Router client-side testing utility; it never talks to the real Go server or a real browser router |
| CI test execution | CI / Build pipeline | — | New GitHub Actions job runs `pnpm vitest run`; report-only per D-04, no deploy/build coupling this phase |
| Coverage measurement | CI / Build pipeline | — | Explicitly deferred to Phase 9 (CICD-12); this phase's job must pass with test *results* only, no coverage gate |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `vitest` | 4.1.10 [VERIFIED: npm registry] | Test runner, assertion API (`expect`), mocking (`vi.mock`/`vi.fn`) | Vite-native test runner; peer-compatible with this project's `vite@^8` (`vitest`'s own `peerDependencies` list `vite: '^6.0.0 \|\| ^7.0.0 \|\| ^8.0.0'`, confirmed via `npm view vitest@4.1.10 peerDependencies` this session) |
| `jsdom` | 30.0.1 [VERIFIED: npm registry] | DOM simulation environment for Vitest (`test.environment: 'jsdom'`) | The standard non-browser DOM implementation Vitest documents for component testing; required as an explicit devDependency (Vitest does not bundle it) |
| `@testing-library/react` | 16.3.2 [VERIFIED: npm registry] | Render components, query the DOM by role/text/label | Peer-compatible with React 19 (`peerDependencies.react: '^18.0.0 \|\| ^19.0.0'`, confirmed via `npm view` this session, matches this project's `react@^19.2.6`); the exact package React Router's own official testing docs import from (`reactrouter.com/start/framework/testing`, fetched this session) |
| `@testing-library/jest-dom` | 7.0.1 [VERIFIED: npm registry] | DOM assertion matchers (`toBeInTheDocument`, `toBeChecked`, `toHaveAttribute`) | Standard companion to RTL; peer-compatible with `vitest >= 0.32` |
| `@testing-library/user-event` | 14.6.4 [VERIFIED: npm registry] | Realistic user interaction simulation (click, type) over RTL's lower-level `fireEvent` | The library React Router's own testing docs use for simulating clicks (`userEvent.click(...)`); more realistic event sequencing than `fireEvent` for checkbox/button interactions |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| — | — | — | No additional runtime test dependency needed. Vite's built-in esbuild JSX transform (driven by `tsconfig.json`'s `"jsx": "react-jsx"`, read this session) handles `.tsx` compilation without an explicit `@vitejs/plugin-react`; only add `@vitejs/plugin-react@6.0.5` [VERIFIED: npm registry, peer `vite: '^8.0.0'`] as a fallback if the executor hits a JSX-transform error the esbuild default doesn't cover (see Pitfall 2). |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Manual `resolve.alias` for `~` in `vitest.config.ts` | `vite-tsconfig-paths` package | The project has exactly one static alias (`~/* → ./app/*`, confirmed by reading `web/tsconfig.json` this session). A manual `resolve: { alias: { "~": path.resolve(__dirname, "app") } }` is zero new dependencies and trivially correct for one alias; `vite-tsconfig-paths` earns its keep only if the alias set grows. Recommend the manual alias. |
| `vi.mock('~/lib/api')` per test file (D-06, locked) | A shared `__mocks__/api.ts` or MSW (Mock Service Worker) handler set | D-06 is a locked decision — MSW/shared-mock alternatives are out of scope for this phase, noted only for completeness. MSW would be the standard choice for a *shared, cross-suite* API mock but the codebase's `internal/watchlist.Store`/`httpserver.Pinger` seam-per-file precedent (cited directly in CONTEXT.md D-06) argues against it here. |
| `@testing-library/jest-dom` matchers via `expect.extend` | Vitest's own built-in DOM matchers | Vitest does not ship DOM-specific matchers (`toBeInTheDocument`, etc.) — `@testing-library/jest-dom` is still required even on the newest Vitest. |

**Installation:**
```bash
cd web
pnpm add -D vitest@4.1.10 jsdom@30.0.1 @testing-library/react@16.3.2 @testing-library/jest-dom@7.0.1 @testing-library/user-event@14.6.4
```

**Version verification:** All five versions above were confirmed current via `npm view <pkg> version` against the live npm registry this session (2026-08-12). `@vitest/coverage-v8` (also `4.1.10`, exact-pinned to the installed `vitest` version per its own `peerDependencies`) is deliberately **not** installed in this phase — see Summary and D-05.

## Package Legitimacy Audit

| Package | Registry | Age (latest publish) | Downloads/wk | Source Repo | Verdict | Disposition |
|---------|----------|-----------------------|--------------|-------------|---------|-------------|
| `vitest` | npm | 2026-07-06 | 89,744,366 | github.com/vitest-dev/vitest | OK | Approved |
| `@testing-library/react` | npm | 2026-01-19 | 52,626,096 | github.com/testing-library/react-testing-library | OK | Approved |
| `@testing-library/jest-dom` | npm | 2026-08-09 | 59,391,099 | github.com/testing-library/jest-dom | SUS ("too-new" — most recent *version publish date*, not package age) | Approved with checkpoint — see note below |
| `@testing-library/user-event` | npm | 2026-08-11 | 46,360,054 | github.com/testing-library/user-event | SUS ("too-new") | Approved with checkpoint — see note below |
| `jsdom` | npm | 2026-07-29 | 91,501,189 | github.com/jsdom/jsdom | SUS ("too-new") | Approved with checkpoint — see note below |
| `@vitest/coverage-v8` | npm | 2026-07-06 | 34,009,446 | github.com/vitest-dev/vitest | OK | Not installed this phase (deferred to Phase 9) |

**Packages removed due to `[SLOP]` verdict:** none.

**Packages flagged as suspicious `[SUS]`:** `@testing-library/jest-dom`, `@testing-library/user-event`, `jsdom`. In all three cases the *only* signal is "too-new," meaning the automated legitimacy check saw a version published within the last few days/weeks and flagged it on recency alone — not on low downloads, missing source repo, or any other fraud indicator. All three packages have 46M–91M weekly downloads and a verified official GitHub org repo (`testing-library`, `jsdom`), which is strong counter-evidence of a slopsquat. **The planner must still add a `checkpoint:human-verify` task before installing these three packages**, per the Package Legitimacy Gate protocol — this note explains why the checkpoint is expected to pass trivially, not why it can be skipped.

## Architecture Patterns

### System Architecture Diagram

```
 Test file (*.test.tsx, co-located beside source)
   │
   ├─ vi.mock('~/lib/api')  ─────────────► replaces api.ts's named exports
   │                                        with vi.fn() stand-ins (D-06)
   │
   ├─ render(<Component ... />)            plain RTL render for
   │     OR                                PreferenceToggles / SearchBox /
   │   render(<RouteStub initialEntries={["/watchlist"]} />)   HistoryFilters / EventCard
   │     (web/app/lib/test/routeStub.tsx, wraps createRoutesStub)
   │                                        route-level render for the
   │                                        WatchlistRow "remove triggers
   │                                        API call" behavior (see Summary)
   │
   ├─ userEvent.click(screen.getByRole(...))   simulates the user action
   │
   ├─ vi.mocked(apiFn).mockResolvedValue(...)  or .mockRejectedValue(...)
   │        (set BEFORE the interaction that triggers the call)
   │
   └─ expect(...)                          assert on:
        - DOM state (screen.getByRole/getByText + jest-dom matchers)
        - mock call arguments (expect(apiFn).toHaveBeenCalledWith(...))
        - callback prop calls (expect(onRemoveSpy).toHaveBeenCalledWith(entry))

 vitest.config.ts (separate from vite.config.ts, D-01)
   │
   ├─ NO reactRouter() plugin (incompatible with Vitest — Pitfall 1)
   ├─ resolve.alias: "~" → "./app"          (replicates tsconfig.json path)
   ├─ test.environment: "jsdom"
   └─ test.setupFiles: ["./vitest.setup.ts"]  registers jest-dom matchers

 .github/workflows/full-pipeline.yml
   │
   └─ new `frontend-test` job (same parallel tier as `vet`/`lint`/`test`/
      `gitleaks`/`trivy-fs`; also added to build-scan's `needs` array so a
      broken frontend suite blocks the build like the Go `test` job does)
        1. pnpm/action-setup (pinned SHA, version matching local dev: 11)
        2. actions/setup-node (pinned SHA, node-version: '22', cache: 'pnpm',
           cache-dependency-path: web/pnpm-lock.yaml)
        3. pnpm install --frozen-lockfile   (working-directory: web)
        4. pnpm vitest run                  (working-directory: web)
```

### Recommended Project Structure
```
web/
├── vitest.config.ts              # NEW — separate from vite.config.ts (D-01)
├── vitest.setup.ts                # NEW — registers @testing-library/jest-dom matchers
├── app/
│   ├── lib/
│   │   ├── api.ts                 # existing — the mock boundary (TEST-02)
│   │   └── test/
│   │       └── routeStub.tsx      # NEW — the one shared createRoutesStub helper (D-03)
│   ├── components/
│   │   ├── watchlist/
│   │   │   ├── WatchlistRow.tsx
│   │   │   ├── PreferenceToggles.tsx
│   │   │   ├── PreferenceToggles.test.tsx   # NEW — co-located (D-02)
│   │   │   ├── SearchBox.tsx
│   │   │   └── SearchBox.test.tsx           # NEW — co-located
│   │   └── history/
│   │       ├── EventCard.tsx
│   │       ├── EventCard.test.tsx           # NEW — co-located
│   │       ├── HistoryFilters.tsx
│   │       └── HistoryFilters.test.tsx      # NEW — co-located
│   └── routes/
│       ├── watchlist.tsx
│       └── watchlist.test.tsx     # NEW — co-located; route-level render proves
│                                    #        WatchlistRow's "remove triggers API
│                                    #        call" behavior (see Summary) via the
│                                    #        shared routeStub helper
```

### Pattern 1: Per-file typed API mock (D-06)
**What:** `vi.mock('~/lib/api')` at the top of a test file, with `vi.mocked(fn)` giving full TypeScript type safety on `.mockResolvedValue`/`.mockRejectedValue`.
**When to use:** Any test whose component imports a function (not just a type) from `~/lib/api` — `PreferenceToggles.tsx`, `SearchBox.tsx`, `HistoryFilters.tsx`, and the route-level `watchlist.tsx` test.
**Example:**
```tsx
// Source: vitest.dev mocking guide + testing-library.com patterns (2026),
// applied to this project's exact api.ts exports (web/app/lib/api.ts:177-189,
// read this session — quoted verbatim below)
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { PreferenceToggles } from "./PreferenceToggles"
import { updateWatchlistPreferences, type WatchlistEntry } from "~/lib/api"

vi.mock("~/lib/api")

// updateWatchlistPreferences(id: number, params: { releaseTypes?: string[]; mutedEventTypes?: string[] }): Promise<WatchlistEntry>
const mockUpdate = vi.mocked(updateWatchlistPreferences)

const entry: WatchlistEntry = {
  id: 1, artist_id: 10, mbid: "mbid-1", name: "Drake", deezer_id: null,
  disambiguation: null, image_url: null, release_types: ["album"],
  muted_event_types: [], created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
}

it("rolls back the optimistic toggle when the PATCH call fails", async () => {
  mockUpdate.mockRejectedValue(new Error("network error"))
  const onEntryChange = vi.fn()

  render(<PreferenceToggles entry={entry} onEntryChange={onEntryChange} />)
  await userEvent.click(screen.getByRole("checkbox", { name: "Single release type" }))

  // optimistic update fired immediately
  expect(onEntryChange).toHaveBeenCalledWith(1, { release_types: ["album", "single"] })

  // rollback after the rejected PATCH resolves
  await waitFor(() =>
    expect(onEntryChange).toHaveBeenLastCalledWith(1, { release_types: ["album"] }),
  )
})
```

### Pattern 2: Shared `createRoutesStub` helper (D-03, success criterion 4)
**What:** One reusable helper wrapping React Router v7's `createRoutesStub`, built once, imported by every test that needs router context (or, per the Summary's architectural finding, needs to render a route component to prove behavior that lives one level above the leaf component being described in a success criterion).
**When to use:** Rendering `web/app/routes/watchlist.tsx`'s `Watchlist` component to prove the "remove control triggers the remove API call" behavior named in success criterion 2.
**Example:**
```tsx
// web/app/lib/test/routeStub.tsx
// Source: reactrouter.com/start/framework/testing (fetched this session) —
// createRoutesStub(routes) returns a Stub component; render it with
// initialEntries to select the starting path. React Router's own docs
// explicitly scope this utility to unit-testing reusable components, which
// is exactly this phase's use case (D-05: floor only, no route-level
// framework-mode testing beyond what's needed to prove the named behaviors).
import { createRoutesStub } from "react-router"
import type { ComponentType } from "react"

export function renderRoute(Component: ComponentType, initialPath: string) {
  const Stub = createRoutesStub([{ path: initialPath, Component }])
  return { Stub, initialEntries: [initialPath] as const }
}
```
```tsx
// web/app/routes/watchlist.test.tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderRoute } from "~/lib/test/routeStub"
import Watchlist from "./watchlist"
import { listWatchlist, removeWatchlist, type WatchlistEntry } from "~/lib/api"

vi.mock("~/lib/api")

it("clicking a row's remove control calls removeWatchlist with that entry's id", async () => {
  const entry: WatchlistEntry = { id: 42, artist_id: 1, mbid: "m1", name: "Drake",
    deezer_id: null, disambiguation: null, image_url: null, release_types: [],
    muted_event_types: [], created_at: "", updated_at: "" }
  vi.mocked(listWatchlist).mockResolvedValue([entry])
  vi.mocked(removeWatchlist).mockResolvedValue(undefined)

  const { Stub, initialEntries } = renderRoute(Watchlist, "/watchlist")
  render(<Stub initialEntries={[...initialEntries]} />)

  await screen.findByText("Drake")
  await userEvent.click(screen.getByRole("button", { name: "Remove Drake from watchlist" }))

  expect(removeWatchlist).toHaveBeenCalledWith(42)
})
```

### Pattern 3: Testing an abort-superseded request (folded bug fix, D-07)
**What:** Prove `SearchBox`'s superseded-request cancellation is real at the network level, not just the `if (controller.signal.aborted) return` guard that already exists.
**When to use:** The RED test for the SearchBox AbortController folded bug — must first prove `searchArtists`/`apiFetch` do NOT forward a signal today (RED, fails against current code), then after the GREEN fix, prove a stale response's callback never fires.
**Example:**
```tsx
// Source: applied pattern from Vitest mocking docs + vitest-dev/vitest#8374
// (fetched this session — jsdom/Vitest override AbortController globally,
// so assert against the mock call args rather than relying on real network
// abort semantics)
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { SearchBox } from "./SearchBox"
import { searchArtists } from "~/lib/api"

vi.mock("~/lib/api")

it("threads the AbortSignal into searchArtists (GREEN, post-fix)", async () => {
  vi.mocked(searchArtists).mockResolvedValue({ query: "dr", sources: {} })
  const onResults = vi.fn()

  render(<SearchBox onResults={onResults} />)
  await userEvent.type(screen.getByLabelText("Search artists"), "dr")

  await vi.waitFor(() =>
    expect(searchArtists).toHaveBeenCalledWith("dr", expect.any(AbortSignal)),
  )
})
```

### Anti-Patterns to Avoid
- **Unit-testing `WatchlistRow.tsx` in isolation to prove SC2's "remove triggers the API call":** `WatchlistRow` never imports `~/lib/api` (verified by reading the file this session) — it only calls the `onRemove` prop. A `WatchlistRow`-only test can prove "clicking remove calls `onRemove(entry)`" but cannot prove the API call happens; that requires the route-level render described in Pattern 2.
- **Exporting `guestFeatureHref` purely to unit-test it:** per the codebase's established Go convention of testing through the public/observable surface rather than reaching into unexported internals (`.planning/codebase/TESTING.md`, read this session — package-external tests, narrow-interface stubs), keep `guestFeatureHref` unexported and assert on the rendered `<a href>` attribute instead (`screen.getByRole('link').toHaveAttribute('href', ...)`).
- **Relying on `.toHaveValue()` against a Base UI `Checkbox.Root`:** Base UI's checkbox renders a `span[role="checkbox"]` with a visually-hidden sibling `<input>` — `.toHaveValue()` targets native input value semantics and does not read correctly off the `span`. Use `getByRole('checkbox', { name: ... })` + `.toBeChecked()` instead — `@testing-library/jest-dom`'s `toBeChecked()` explicitly supports `role="checkbox"` elements with a valid `aria-checked` attribute (confirmed from the jest-dom README this session), which is exactly what Base UI's `Checkbox.Root` sets.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Router context for tests | A custom `MemoryRouter`-wrapping test utility | React Router v7's own `createRoutesStub` | It's the framework's own documented testing primitive (reactrouter.com/start/framework/testing), purpose-built for exactly this |
| DOM assertion helpers (`toBeInTheDocument`, `toBeChecked`) | Hand-rolled `element.textContent`/`element.getAttribute` checks | `@testing-library/jest-dom` matchers | Standard, well-tested, and gives readable failure messages; hand-rolled checks silently diverge from ARIA semantics (e.g. the Base UI checkbox case above) |
| Typed mock functions for `~/lib/api` | Manually casting `apiFn as any` and tracking calls in a local array | `vi.mock` + `vi.mocked(fn).mockResolvedValue/mockRejectedValue` | Vitest's `vi.mocked()` exists specifically to give TypeScript-safe access to mock methods on an auto-mocked import; hand-rolling loses type checking on the mock's return shape |

**Key insight:** every "don't hand-roll" item above already has a first-party, actively-maintained, official tool this project is about to install — the risk in this phase isn't inventing bad abstractions, it's mis-scoping which surface actually owns the behavior a success criterion describes (see the `WatchlistRow` vs. route-level finding in the Summary).

## Common Pitfalls

### Pitfall 1: `web/vite.config.ts`'s `reactRouter()` plugin breaks under Vitest
**What goes wrong:** Vitest fails to load or transform files because `@react-router/dev/vite`'s plugin does virtual-module resolution (`virtual:react-router/server-build`, etc.) that only makes sense inside `react-router dev`/`react-router build`, not Vitest's own Vite instance.
**Why it happens:** React Router's own team has stated the plugin "has not been designed for use with tools such as Vitest" (confirmed via community reports + GitHub discussions this session).
**How to avoid:** Never import or reference `web/vite.config.ts` from `vitest.config.ts` (D-01 already locks this). Build `vitest.config.ts` as a fully independent file with its own minimal `plugins`/`resolve` — do not use `mergeConfig()` against the production config, since that would re-pull in `reactRouter()`.
**Warning signs:** Errors mentioning `virtual:react-router/*`, `Cannot find module`, or a hang during Vitest's file transform step.

### Pitfall 2: `.tsx` test files fail to transform without an explicit React plugin (contingent)
**What goes wrong:** If Vite's default esbuild JSX transform doesn't pick up `tsconfig.json`'s `"jsx": "react-jsx"` setting inside `vitest.config.ts`'s isolated config context, `.tsx` test files fail to parse.
**Why it happens:** Community examples of "React Router + Vitest, no `reactRouter()` plugin" configs generally work with zero explicit React plugin (Vite's esbuild-based default JSX transform is normally sufficient) — but this is a MEDIUM-confidence claim (not exhaustively confirmed against this exact project's `vitest.config.ts` in isolation).
**How to avoid:** If the executor hits a JSX-transform error, add `@vitejs/plugin-react@6.0.5` [VERIFIED: npm registry, peer `vite: ^8.0.0`] to `vitest.config.ts`'s `plugins` array — do not add `reactRouter()` back.
**Warning signs:** `Unexpected token` / `Unexpected "<"` errors when a test file that renders JSX runs.

### Pitfall 3: `jsdom@30.0.1`'s `engines.node` requirement is narrower than this dev machine's installed Node
**What goes wrong:** `jsdom@30.0.1`'s `package.json` declares `"engines": { "node": "^22.22.2 || ^24.15.0 || >=26.0.0" }` (confirmed via `npm view jsdom@30.0.1 engines` this session). This dev machine reports `node --version` → `v22.21.1`, which is *below* `22.22.2` and therefore outside every satisfied range.
**Why it happens:** jsdom's own release notes periodically bump the minimum supported Node patch version for V8/ICU-related reasons.
**How to avoid:** Neither npm nor pnpm hard-fail on an engines mismatch by default (`npm config get engine-strict` → `false`, confirmed this session; no `.npmrc`/`engine-strict` override exists in this repo). Local installs/test runs will likely still work with only a warning. In CI, pin `actions/setup-node`'s `node-version: '22'` (major-only) — GitHub's Node dist index will resolve to the latest 22.x patch at runtime, which will already satisfy `^22.22.2` by the time this phase executes (2026-08-12 is well past that patch's release). If a developer's local Node is genuinely too old and something breaks, upgrade Node rather than downgrading jsdom.
**Warning signs:** An `EBADENGINE` warning during `pnpm install`, or (rarer) a jsdom internal crash on startup.

### Pitfall 4: `pnpm/action-setup` must run before `actions/setup-node`'s pnpm cache step
**What goes wrong:** `actions/setup-node`'s built-in `cache: 'pnpm'` option shells out to `pnpm store path` to locate the cache directory — if `pnpm` isn't on `PATH` yet, that step fails.
**Why it happens:** Ordering-dependent GitHub Actions step composition; this is a well-documented requirement of `pnpm/action-setup` + `actions/setup-node`'s pnpm caching integration.
**How to avoid:** In the new `frontend-test` job, place the `pnpm/action-setup` step *before* `actions/setup-node`.
**Warning signs:** `Unable to locate executable file: pnpm` during the cache-restore step.

### Pitfall 5: Sonner's `toast.error(...)` runs inside `PreferenceToggles`'s catch block, but no `<Toaster />` is mounted in these unit tests
**What goes wrong:** `sonner`'s `<Toaster />` component (which actually renders toasts to the DOM) uses browser APIs like `ResizeObserver`, which jsdom does not implement — but `PreferenceToggles.tsx` only calls the `toast` function (`sonner`'s internal store dispatcher), never renders `<Toaster />` itself. This should be harmless (dispatching to an internal store doesn't touch the DOM), but is unconfirmed against this exact `sonner@2.0.8` version in a jsdom test.
**Why it happens:** `sonner`'s architecture separates the `toast()` call (pure store update) from the `<Toaster />` consumer (DOM rendering + `ResizeObserver` usage) — confirmed as the general library design, not confirmed against this exact test scenario.
**How to avoid:** Do not render `<Toaster />` in these component tests (it's not needed to prove the rollback behavior, and D-06 doesn't require mocking `sonner`). If a `ResizeObserver is not defined` error surfaces anyway, add a minimal `ResizeObserver` stub to `vitest.setup.ts` (`vi.stubGlobal('ResizeObserver', class { observe(){}; unobserve(){}; disconnect(){} })`) as a defensive, low-cost addition — cheaper to add preemptively than to debug reactively.
**Warning signs:** `ReferenceError: ResizeObserver is not defined` thrown from inside a `toast.error(...)` call.

### Pitfall 6: TypeScript's `EventItem["event_type"]` union blocks constructing the RED-test fixture for the fallback-badge bug
**What goes wrong:** `EVENT_BADGE` is typed `Record<EventItem["event_type"], ...>` where `event_type` is `"new_release" | "guest_feature" | "deluxe_change"` (quoted verbatim from `web/app/lib/api.ts:16`, read this session). A test fixture needs an event whose `event_type` is *outside* this union (to simulate an unvalidated value from the API) — TypeScript will reject that literal at compile time.
**Why it happens:** The bug is a *runtime* gap (no validation on data from `GET /events`) that the type system correctly can't represent as a normal literal.
**How to avoid:** Cast deliberately: `{ ...baseEvent, event_type: "unknown_type" as EventItem["event_type"] }` — this is the standard, intentional pattern for testing "the type system says this can't happen, but the network boundary doesn't enforce it" scenarios, not a type-safety violation to avoid.
**Warning signs:** A TS2322 type error on the fixture object literal if the cast is omitted.

## Code Examples

### `vitest.config.ts` (new, separate from `vite.config.ts` per D-01)
```ts
// Source: pattern confirmed against akoskm.com/react-router-vitest-example's
// working vite.config.ts (fetched verbatim this session) and vitest.dev's
// own environment/setupFiles docs. This project keeps a fully separate file
// rather than a `process.env.VITEST`-conditional shared config, matching
// D-01's explicit requirement.
import path from "node:path"
import { defineConfig } from "vitest/config"

export default defineConfig({
  resolve: {
    alias: { "~": path.resolve(__dirname, "./app") },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
  },
})
```

### `vitest.setup.ts` (new)
```ts
// Source: testing-library.com + vitest.dev standard setup pattern (2026)
import "@testing-library/jest-dom/vitest"
```

### `package.json` script additions
```jsonc
{
  "scripts": {
    "test": "vitest run",
    "test:watch": "vitest"
  }
}
```

### GitHub Actions job addition (`.github/workflows/full-pipeline.yml`)
```yaml
  frontend-test:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    defaults:
      run:
        working-directory: web
    steps:
      - name: Checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - name: Set up pnpm
        # Source: GitHub API this session — pnpm/action-setup latest release
        # tag v6.0.10 resolved to this commit SHA via
        # `curl https://api.github.com/repos/pnpm/action-setup/git/ref/tags/v6.0.10`
        uses: pnpm/action-setup@ff378ebe6b225b0680b81c1ad4498ae0d1d3a5e3 # v6.0.10
        with:
          version: 11
      - name: Set up Node
        # Source: GitHub API this session — actions/setup-node latest
        # release tag v7.0.0 resolved to this commit SHA via
        # `curl https://api.github.com/repos/actions/setup-node/git/ref/tags/v7.0.0`
        uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0
        with:
          node-version: '22'
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - name: Install dependencies
        run: pnpm install --frozen-lockfile
      - name: Run Vitest suite
        run: pnpm test
```
Add `frontend-test` to `build-scan`'s `needs: [vet, lint, test, gitleaks, trivy-fs]` array so a broken frontend suite blocks the build the same way a broken Go test job already does — this is a recommendation (D-04 says "report-only, no coverage threshold," which this research reads as "no coverage percentage gate," not "test failures don't block the build"); confirm this reading with the user if the planner treats it as ambiguous.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Jest + `ts-jest`/`babel-jest` for React component testing | Vitest (Vite-native, esbuild-based transform) | Vitest has been the community-default for Vite-based React projects for several years now | This project already runs on Vite 8 for its dev/build pipeline — Vitest reuses that same toolchain instead of introducing a second, slower transform pipeline |
| `enzyme` shallow rendering | `@testing-library/react` (DOM-query-by-role/text, not implementation-detail rendering) | Enzyme has been effectively unmaintained for React 18+/19 | RTL's philosophy ("test what the user sees," query by ARIA role) is also why the Base UI checkbox pitfall above resolves cleanly via `getByRole` + `aria-checked` |

**Deprecated/outdated:** N/A — this is a greenfield test-tooling install, not a migration off an existing framework.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Vite's default esbuild JSX transform (no `@vitejs/plugin-react`) is sufficient for `.tsx` test files in `vitest.config.ts` once `reactRouter()` is removed | Standard Stack (Supporting), Pitfall 2 | If wrong, first test run fails on a JSX parse error — low-cost fix (add `@vitejs/plugin-react@6.0.5`, already version-verified as a fallback) |
| A2 | Calling `sonner`'s `toast.error(...)` without a mounted `<Toaster />` does not touch `ResizeObserver` or otherwise throw in jsdom | Pitfall 5 | If wrong, `PreferenceToggles`'s rollback test throws on the `catch` branch; fix is a one-line `ResizeObserver` stub in `vitest.setup.ts`, already documented as the mitigation |
| A3 | `build-scan`'s `needs` array should include the new `frontend-test` job (test failures block the build), reading D-04's "report-only, no coverage threshold" as scoped to coverage percentage only | Code Examples (GitHub Actions job) | If wrong (user intended the frontend job to never block anything this phase), the fix is a one-line removal from `needs` — low risk either way, but worth an explicit confirm since it changes pipeline gating behavior |
| A4 | Rendering the `Watchlist` route component through the shared `createRoutesStub` helper (rather than a plain `render()`) is the right way to prove SC2's watchlist-row-remove behavior, satisfying SC4's router-stub mandate in the same test | Summary, Architecture Patterns (Pattern 2) | If wrong, the planner could instead render `Watchlist` with plain `render()` (works fine technically, since `Watchlist` uses no router hooks) and build the shared `createRoutesStub` helper as pure scaffolding for a future phase instead — either choice satisfies both success criteria, this is a stylistic/forward-compatibility call, not a correctness risk |

## Open Questions

1. **Does `build-scan` gate on `frontend-test`?**
   - What we know: D-04 says the new job is "report-only, no coverage threshold." The existing Go `test` job already gates `build-scan` via its `needs` array.
   - What's unclear: Whether "report-only" was meant to also exempt test *failures* (not just coverage) from blocking the build this phase.
   - Recommendation: Default to gating (add `frontend-test` to `build-scan`'s `needs`), consistent with the existing Go test job's role and CICD-01's precedent ("Every push runs ... the Go test suite ... before any build/publish step"); flag for a quick user confirm during planning if the plan-checker treats this as a scope question.

2. **Exact location for the shared `routeStub.tsx` helper.**
   - What we know: D-03 requires "one shared helper, established once and reused." `.planning/codebase/STRUCTURE.md`'s "Test Utilities" section (a generic, templated doc, not phase-08-specific) offers two options: co-located under `web/app/__tests__/` or alongside other utilities.
   - What's unclear: STRUCTURE.md doesn't have a phase-08-specific precedent to point to (no shared frontend test helper exists yet in this codebase).
   - Recommendation: `web/app/lib/test/routeStub.tsx` — keeps it importable via the same `~/lib/...` alias convention as `~/lib/api`, and mirrors the Go codebase's `internal/testutil/` precedent (a dedicated, reusable, non-test-suffixed package) closely enough to be recognizable. This is Claude's Discretion under D-01/D-03 (CONTEXT.md's Claude's Discretion section is empty, but the *exact path* wasn't part of what was discussed) — the planner may choose differently without contradicting any locked decision.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | Running `pnpm`/`vitest` locally and in CI | ✓ | v22.21.1 (local dev machine, confirmed via `node --version` this session) | See Pitfall 3 — below `jsdom@30`'s `^22.22.2` floor; works today with only a warning, no hard block |
| pnpm | Package install/scripts | ✓ | 11.8.0 (local, confirmed via `pnpm --version` this session) | Pin CI's `pnpm/action-setup` to major version `11` to match |
| GitHub Actions `ubuntu-latest` runner Node | CI `frontend-test` job | ✓ (via `actions/setup-node`) | Resolved at runtime from `node-version: '22'` | None needed — GitHub's Node 22.x dist index is well past `22.22.2` as of this research date |

**Missing dependencies with no fallback:** none.

**Missing dependencies with fallback:** Local Node patch version (Pitfall 3) — non-blocking, documented fallback above.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 4.1.10 (new this phase — no prior frontend test framework existed) |
| Config file | `web/vitest.config.ts` (new, separate from `web/vite.config.ts` per D-01) |
| Quick run command | `pnpm --dir web test` (single-shot, `vitest run`) |
| Full suite command | `pnpm --dir web test` (same command — the whole suite is small enough this phase that "quick" and "full" are identical; Phase 9 may split these once `--coverage` is added) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TEST-01 | Preference toggle rolls back optimistic state on PATCH failure | component | `pnpm --dir web exec vitest run app/components/watchlist/PreferenceToggles.test.tsx` | ❌ Wave 0 |
| TEST-01 | Search issues a debounced, cancellable request through `searchArtists` | component | `pnpm --dir web exec vitest run app/components/watchlist/SearchBox.test.tsx` | ❌ Wave 0 |
| TEST-01 | History/event-filter populates from `listWatchlist` and reports filter changes upward | component | `pnpm --dir web exec vitest run app/components/history/HistoryFilters.test.tsx` | ❌ Wave 0 |
| TEST-01 | Watchlist row's remove control triggers `removeWatchlist` (route-level, via shared router stub) | component/route | `pnpm --dir web exec vitest run app/routes/watchlist.test.tsx` | ❌ Wave 0 |
| TEST-01 | EventCard falls back to a default badge for an unrecognized `event_type` (folded bug, RED-then-GREEN) | component | `pnpm --dir web exec vitest run app/components/history/EventCard.test.tsx` | ❌ Wave 0 |
| TEST-01 | `guestFeatureHref` encodes `external_id` in the rendered anchor (folded bug, RED-then-GREEN) | component | `pnpm --dir web exec vitest run app/components/history/EventCard.test.tsx` | ❌ Wave 0 |
| TEST-02 | Every test above mocks `~/lib/api`, never raw `fetch` | (cross-cutting — enforced by `vi.mock('~/lib/api')` at the top of each file, D-06) | (grep check: `grep -rL "vi.mock(\"~/lib/api\")" web/app/**/*.test.tsx` should return nothing for files that import a function, not just a type, from `~/lib/api`) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `pnpm --dir web exec vitest run <changed test file>`
- **Per wave merge:** `pnpm --dir web test` (full suite)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `web/vitest.config.ts` — framework config, does not exist yet
- [ ] `web/vitest.setup.ts` — jest-dom matcher registration, does not exist yet
- [ ] `web/app/lib/test/routeStub.tsx` — shared `createRoutesStub` helper (D-03), does not exist yet
- [ ] Framework install: `pnpm add -D vitest@4.1.10 jsdom@30.0.1 @testing-library/react@16.3.2 @testing-library/jest-dom@7.0.1 @testing-library/user-event@14.6.4` (run from `web/`)
- [ ] `package.json` `"test"` script addition

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Project has no auth (single-operator deployable, per PROJECT.md/CLAUDE.md out-of-scope list) |
| V3 Session Management | no | No sessions in this project |
| V4 Access Control | no | No access-control surface touched by this phase |
| V5 Input Validation | yes | `encodeURIComponent(event.external_id)` in `EventCard.tsx`'s `guestFeatureHref` (the folded cosmetic bug, D-07) — untrusted third-party string (MusicBrainz/Deezer `external_id`) interpolated into a URL path must be escaped before it reaches an `<a href>`, preventing a malformed/attacker-influenced URL if a future upstream source ever returns a non-UUID id |
| V6 Cryptography | no | Nothing cryptographic in this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unescaped third-party string interpolated into a URL (`guestFeatureHref`) | Tampering | `encodeURIComponent()` before interpolation (the exact fix this phase's folded bug applies) — this phase's RED-then-GREEN test for that bug doubles as the regression guard for this threat |
| Client-side XSS via unescaped rendering of API-sourced text | Tampering | Already closed project-wide per Phase 6 (STATE.md: "a repo-wide `dangerouslySetInnerHTML` grep across `web/app/` returns zero matches") — no new risk introduced by this phase, since it adds tests, not new rendering surfaces |

## Sources

### Primary (HIGH confidence)
- `npm view <pkg> version / peerDependencies / engines` (this session) — exact current versions and peer/engine constraints for `vitest`, `jsdom`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`, `@vitest/coverage-v8`, `@vitejs/plugin-react`, `react-router`
- `gsd-tools query package-legitimacy check` (this session) — registry existence, download counts, source repos for all six candidate packages
- GitHub API (`api.github.com/repos/{pnpm/action-setup,actions/setup-node}/...`) (this session) — exact commit SHAs for pinned Actions versions, per CICD-08's SHA-pinning requirement
- Direct `Read` of `web/app/lib/api.ts`, `web/app/components/watchlist/{WatchlistRow,PreferenceToggles,SearchBox,SearchResultsColumns}.tsx`, `web/app/components/history/{EventCard,HistoryFilters}.tsx`, `web/app/routes/{watchlist,history}.tsx`, `web/app/root.tsx`, `web/app/routes.ts`, `web/tsconfig.json`, `web/vite.config.ts`, `web/react-router.config.ts`, `.github/workflows/full-pipeline.yml`, `.planning/codebase/{TESTING,STRUCTURE}.md` (this session) — the architectural findings in the Summary (WatchlistRow doesn't call the API, no component uses router hooks) come directly from these reads, not from documentation

### Secondary (MEDIUM confidence)
- `reactrouter.com/start/framework/testing` (fetched this session) — official `createRoutesStub` usage example, imports, and scope guidance
- `akoskm.com/react-router-vitest-example` + its linked GitHub repo's `vite.config.ts` (fetched verbatim this session) — working React Router v7 + Vitest config pattern
- `vitest.dev` mocking/environment guides, `testing-library.com` setup guides, general WebSearch results on `vi.mock`/`vi.mocked` typed patterns (this session)
- `github.com/mui/base-ui` issue #4048, `testing-library/jest-dom` README `toBeChecked()` docs (fetched this session) — Base UI checkbox testing semantics

### Tertiary (LOW confidence)
- `vitest-dev/vitest` issue #8374 (fetched this session) — AbortController/jsdom interaction bug, confirmed Node-24-specific by the reporter, not confirmed against this project's Node 22 — treated as a defensive pitfall note, not a confirmed blocker
- Community WebSearch results on `sonner` + `ResizeObserver` + jsdom (general pattern, not confirmed against this exact `sonner@2.0.8` + no-`<Toaster/>`-mounted scenario) — see Assumption A2

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every version/peer-dependency claim verified directly against the live npm registry this session
- Architecture: HIGH — the WatchlistRow/router-hook findings come from reading the actual source files this session, not inference from the phase description
- Pitfalls: MEDIUM — most pitfalls are CITED from official docs or cross-referenced community sources; two (Pitfall 2's JSX transform, Pitfall 5's sonner/ResizeObserver) are explicitly flagged as unconfirmed assumptions with cheap, documented fallbacks

**Research date:** 2026-08-12
**Valid until:** 2026-09-11 (30 days — npm-ecosystem package versions move fast; re-verify exact pinned versions if planning is delayed past this window)
