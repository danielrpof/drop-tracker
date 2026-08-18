# Phase 8: Frontend Test Suite - Pattern Map

**Mapped:** 2026-08-12
**Files analyzed:** 10 (5 new test files, 3 new config/setup files, 1 shared test helper, 1 CI workflow edit, plus 3 source-file bug fixes)
**Analogs found:** N/A for structure (greenfield frontend tests — no `*.test.tsx` exists yet). RESEARCH.md's Code Examples section already supplies working, project-specific code for every new file; this document supplements that with concrete excerpts read directly from the *components under test* and the one applicable in-repo naming/philosophy analog (Go tests).

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `web/vitest.config.ts` | config | — | `web/vite.config.ts` (must NOT reuse `reactRouter()` plugin — see Pitfall 1 in RESEARCH.md) | partial (structure, not content) |
| `web/vitest.setup.ts` | config | — | none in repo (RESEARCH.md Code Examples supplies it verbatim) | no analog |
| `web/app/lib/test/routeStub.tsx` | utility (test helper) | request-response (router stub) | none in repo; RESEARCH.md Pattern 2 supplies it verbatim | no analog |
| `web/app/components/watchlist/PreferenceToggles.test.tsx` | test | request-response (optimistic update + rollback) | source: `web/app/components/watchlist/PreferenceToggles.tsx`; naming/philosophy: `internal/watchlist/service_test.go` | role-match (source read directly; no sibling test file exists) |
| `web/app/components/watchlist/SearchBox.test.tsx` | test | request-response (debounced, abortable fetch) | source: `web/app/components/watchlist/SearchBox.tsx` | role-match |
| `web/app/components/history/HistoryFilters.test.tsx` | test | CRUD (read-only list population) + event-driven (onChange) | source: `web/app/components/history/HistoryFilters.tsx` | role-match |
| `web/app/routes/watchlist.test.tsx` | test (route-level) | CRUD (list/add/remove) | source: `web/app/routes/watchlist.tsx` + `web/app/components/watchlist/WatchlistRow.tsx` | role-match |
| `web/app/components/history/EventCard.test.tsx` | test | transform (pure render, no API) | source: `web/app/components/history/EventCard.tsx` | role-match |
| `web/app/components/history/EventCard.tsx` (bug fix: fallback badge) | component | transform | itself — fix `EVENT_BADGE[event.event_type]` lookup at line 39 | n/a (fix in place) |
| `web/app/lib/api.ts` + `SearchBox.tsx` (bug fix: AbortSignal threading) | service / component | request-response | itself — `apiFetch` (api.ts:100), `searchArtists` (api.ts:200), `SearchBox.runSearch` (SearchBox.tsx:44) | n/a (fix in place) |
| `web/app/components/history/EventCard.tsx` (bug fix: `encodeURIComponent`) | component | transform | itself — `guestFeatureHref` (EventCard.tsx:108) | n/a (fix in place) |
| `.github/workflows/full-pipeline.yml` (new `frontend-test` job) | config (CI) | batch | existing Go `test`/`lint`/`vet`/`gitleaks`/`trivy-fs` jobs in the same file | role-match (verify exact job syntax by reading the file directly during planning — not reproduced here since RESEARCH.md's Code Examples section already has a verified job block with pinned SHAs) |

## Pattern Assignments

### `web/vitest.config.ts` (config)

**Analog:** `web/vite.config.ts` (read in full above) — copy the `~` alias resolution intent, but do **not** reuse the file's plugins.

**Existing `vite.config.ts` in full** (`web/vite.config.ts:1-22`):
```ts
import { reactRouter } from "@react-router/dev/vite"
import tailwindcss from "@tailwindcss/vite"
import { defineConfig } from "vite"

export default defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [tailwindcss(), reactRouter()],
  server: {
    proxy: {
      "/health": "http://localhost:8080",
      "/search": "http://localhost:8080",
      "/watchlist": "http://localhost:8080",
      "/events": "http://localhost:8080",
    },
  },
})
```

**Why it cannot be reused directly:** production config uses `tsconfigPaths: true` (a plugin-driven resolver) plus `reactRouter()`, which RESEARCH.md Pitfall 1 confirms breaks under Vitest. `tsconfig.json`'s only path alias is `"~/*": ["./app/*"]` (`web/tsconfig.json:16-18`) — replicate this as a manual `resolve.alias` in `vitest.config.ts` instead of pulling in `tsconfigPaths`/`vite-tsconfig-paths`.

**Use RESEARCH.md's verified `vitest.config.ts` code example verbatim** (already alias-correct, `environment: "jsdom"`, `setupFiles: ["./vitest.setup.ts"]`) — do not re-derive.

---

### `web/app/lib/api.ts` — the mock boundary (read in full above, `web/app/lib/api.ts:1-204`)

Every test file that imports a function (not just a type) from `~/lib/api` must `vi.mock("~/lib/api")` at the top of the file per D-06. Key exports test files will mock:

- `listWatchlist(): Promise<WatchlistEntry[]>` (api.ts:147-149) — used by `HistoryFilters.tsx` and `watchlist.tsx`
- `addWatchlist(params): Promise<WatchlistEntry>` (api.ts:154-172) — used by `watchlist.tsx` (add + undo)
- `updateWatchlistPreferences(id, params): Promise<WatchlistEntry>` (api.ts:177-189) — used by `PreferenceToggles.tsx`
- `removeWatchlist(id): Promise<void>` (api.ts:193-195) — used by `watchlist.tsx`
- `searchArtists(query): Promise<SearchResponse>` (api.ts:200-203) — used by `SearchBox.tsx`
- `ApiError` class (api.ts:84-92) — `watchlist.tsx`'s `handleAddSearchResult` special-cases `err.status === 409`; a test proving "409 on add is treated as success" needs `mockRejectedValue(new ApiError(409, "..."))`.

**apiFetch core** (api.ts:100-122) — this is the function the folded AbortController bug fix touches. Currently it does **not** accept or forward a signal:
```ts
async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  ...
}
```
Fix threads `signal` through via `init` (searchArtists already takes no signal param today — the GREEN fix adds one, per RESEARCH.md Pattern 3). RED test: assert `fetch`/`searchArtists` is called WITHOUT a forwarded signal reaching `apiFetch`'s `fetch()` call before the fix; GREEN test: `expect(searchArtists).toHaveBeenCalledWith(query, expect.any(AbortSignal))` after `SearchBox.runSearch` is updated to pass `controller.signal` through.

**Wire types available as fixture shapes** (api.ts:12-77): `EventItem`, `EventsPage`, `WatchlistEntry`, `SearchArtist`, `SourceResult`, `SearchResponse` — copy field names verbatim into test fixtures (see `WatchlistEntry` fixture example already in RESEARCH.md Pattern 1, lines 206-211 of that doc — all fields present, no need to re-derive).

---

### `web/app/components/watchlist/PreferenceToggles.test.tsx` (test)

**Source under test:** `web/app/components/watchlist/PreferenceToggles.tsx` (read in full above, 146 lines)

**Core optimistic-update + rollback pattern to assert against** (`PreferenceToggles.tsx:41-56`):
```tsx
async function toggleReleaseType(type: string, next: boolean) {
  const previous = entry.release_types
  const optimistic = next ? [...previous, type] : previous.filter((t) => t !== type)

  onEntryChange(entry.id, { release_types: optimistic })   // optimistic, fires immediately
  setReleasePending(true)
  try {
    const updated = await updateWatchlistPreferences(entry.id, { releaseTypes: optimistic })
    onEntryChange(entry.id, { release_types: updated.release_types })
  } catch {
    onEntryChange(entry.id, { release_types: previous })   // rollback on failure
    toast.error("Couldn't update preferences — try again.")
  } finally {
    setReleasePending(false)
  }
}
```
Same shape repeats for `toggleMutedEventType` (lines 58-73) against `muted_event_types`. A single test file should cover both axes since D-05 floor is "one test per surface," but the rollback behavior is the specific behavior named in success criterion 2 — RESEARCH.md Pattern 1 already gives the exact working test for the release-type axis; mirror it once more for the mute axis if time allows within floor scope (optional, not required by D-05).

**Checkbox query target** — checkboxes render via the project's own `Checkbox` wrapper (`~/components/ui/checkbox`, imported at `PreferenceToggles.tsx:4`) with `aria-label` set explicitly per checkbox (e.g. `aria-label="Single release type"` at line 92, `aria-label="Mute new release alerts"` at line 122) — use `screen.getByRole("checkbox", { name: "..." })` matching these exact strings, not `getByLabelText` (no `<label htmlFor>` association — the `<label>` wraps the checkbox instead, at lines 88-95).

**Toast import** — `import { toast } from "sonner"` (line 2) is called directly (`toast.error(...)`, line 52/69) without a mounted `<Toaster />` — per RESEARCH.md Pitfall 5, do not render `<Toaster />` in this test; if `ResizeObserver is not defined` surfaces, add the stub to `vitest.setup.ts`.

---

### `web/app/components/watchlist/SearchBox.test.tsx` (test)

**Source under test:** `web/app/components/watchlist/SearchBox.tsx` (read in full above, 111 lines)

**Debounce + search-supersession pattern** (`SearchBox.tsx:44-68`):
```tsx
function runSearch(query: string) {
  abortRef.current?.abort()
  const controller = new AbortController()
  abortRef.current = controller

  setLoading(true)
  searchArtists(query)
    .then((response) => {
      if (controller.signal.aborted) return
      onResults(response)
    })
    .catch(() => {
      if (controller.signal.aborted) return
      onResults(null)
    })
    .finally(() => {
      if (controller.signal.aborted) return
      setLoading(false)
    })
}
```
`SEARCH_DEBOUNCE_MS = 300` (line 12) — tests must either use `vi.useFakeTimers()` + `vi.advanceTimersByTime(300)` or `await vi.waitFor(...)` with real timers; RESEARCH.md Pattern 3 uses the latter (`vi.waitFor` around the `searchArtists` assertion), which is the simpler choice given `userEvent.type` already awaits internally.

**Input query target** — the `<Input>` (from `~/components/ui/input`) is wrapped in a `<label>` with visible text "Search artists" (line 91-92) with no explicit `aria-label`/`htmlFor` pairing shown in this excerpt — verify at test-authoring time whether `getByLabelText("Search artists")` resolves (label wraps input directly, which RTL does support) before assuming it works; fall back to `getByRole("textbox")` if not.

**AbortController folded-bug RED test target:** currently `searchArtists(query)` (line 50) is called with only `query`, no signal — RESEARCH.md Pattern 3's RED assertion is `expect(searchArtists).not.toHaveBeenCalledWith(query, expect.any(AbortSignal))` (or equivalently check `mock.calls[0]` has length 1) before the fix, then GREEN after `SearchBox.tsx:50` and `api.ts:200-203`'s `searchArtists` signature both add a `signal` param threaded to `apiFetch`.

---

### `web/app/components/history/HistoryFilters.test.tsx` (test)

**Source under test:** `web/app/components/history/HistoryFilters.tsx` (read in full above, 95 lines)

**listWatchlist population + onChange reporting pattern** (`HistoryFilters.tsx:37-52`, `61-63`, `80-83`):
```tsx
useEffect(() => {
  let cancelled = false
  listWatchlist()
    .then((entries) => { if (!cancelled) setArtists(entries) })
    .catch(() => { if (!cancelled) setArtists([]) })
  return () => { cancelled = true }
}, [])
```
```tsx
onChange={(e) => {
  const raw = e.target.value
  onChange({ ...value, artistId: raw === "" ? null : Number(raw) })
}}
```
Test should mock `listWatchlist` to resolve with a fixture array, `findByRole("option", { name: <artist.name> })` (or assert the `<select>`'s populated `<option>` list) to prove population, then `userEvent.selectOptions` on the "Artist" or "Event type" `<select>` and assert the `onChange` spy prop was called with `{ ...value, artistId: N }` / `{ ...value, eventType: "..." }` — this proves "reports filter changes upward" per the Phase Requirements → Test Map row.

**No router/`vi.mock` needed for router context** — `HistoryFilters` uses no router hooks; only `vi.mock("~/lib/api")` for `listWatchlist`.

---

### `web/app/routes/watchlist.test.tsx` (test, route-level)

**Source under test:** `web/app/routes/watchlist.tsx` (read in full above, 205 lines) + `web/app/components/watchlist/WatchlistRow.tsx` (read in full above, 57 lines)

**Critical finding (confirmed by direct read):** `WatchlistRow.tsx` never imports `~/lib/api` — its remove button only calls the `onRemove` prop (`WatchlistRow.tsx:51`, `onClick={() => onRemove(entry)}`). The actual `removeWatchlist` API call lives in `watchlist.tsx`'s `handleRemove` (lines 122-154):
```tsx
async function handleRemove(entry: WatchlistEntry) {
  setEntries((rows) => (rows ? rows.filter((r) => r.id !== entry.id) : rows))
  try {
    await removeWatchlist(entry.id)
  } catch {
    toast.error(`Couldn't remove ${entry.name} — it may already be gone.`)
    refresh()
    return
  }
  toast.success(`Removed ${entry.name} from your watchlist.`, { ... })
}
```
This confirms RESEARCH.md's Summary/Pattern 2/Anti-Patterns guidance: the "remove control triggers the remove API call" behavior can only be proven by rendering `Watchlist` (the route default export, `watchlist.tsx:32`), not `WatchlistRow` alone. Use `renderRoute`/`createRoutesStub` from the new `web/app/lib/test/routeStub.tsx` helper — RESEARCH.md Pattern 2 already supplies both the helper and the full test verbatim (mocking `listWatchlist` + `removeWatchlist`, rendering the stub, `userEvent.click` on `getByRole("button", { name: "Remove Drake from watchlist" })` — matching the exact `aria-label` template at `WatchlistRow.tsx:50`: `` `Remove ${entry.name} from watchlist` ``).

**Remove button aria-label exact string to match in tests** (`WatchlistRow.tsx:50`):
```tsx
aria-label={`Remove ${entry.name} from watchlist`}
```

**Loading/error/empty states** (`watchlist.tsx:171-189`) are also present if the floor test wants to assert initial skeleton-then-populated transition, but D-05 floor only requires the named remove-triggers-API-call behavior — do not expand scope.

---

### `web/app/components/history/EventCard.test.tsx` (test, three RED-then-GREEN pairs + no-mock rendering tests)

**Source under test:** `web/app/components/history/EventCard.tsx` (read in full above, 156 lines) — no API import, only `import type { EventItem } from "~/lib/api"` (line 3), so **no `vi.mock` needed** for this file at all.

**Bug 1 — fallback badge, RED test target** (`EventCard.tsx:39`):
```tsx
const badge = EVENT_BADGE[event.event_type]
```
`EVENT_BADGE` (lines 17-36) is `Record<EventItem["event_type"], {...}>` covering only the three known literals — an event with an out-of-union `event_type` makes `badge` `undefined`, and the next line (`badge.color` at line 62, `badge.emoji`/`badge.label` at 64-65) throws. RED test constructs a fixture with `event_type: "unknown_type" as EventItem["event_type"]` (RESEARCH.md Pitfall 6's exact cast pattern) and asserts the render does NOT throw / renders a fallback badge — this fails against current code. GREEN fix adds a default fallback entry/branch in `EVENT_BADGE` lookup (e.g. `EVENT_BADGE[event.event_type] ?? { label: "Unknown", emoji: "❓", color: "var(--color-muted)" }` or equivalent) at line 39.

**Bug 2 — `guestFeatureHref` missing `encodeURIComponent`, RED test target** (`EventCard.tsx:108-116`):
```tsx
function guestFeatureHref(event: EventItem): string | null {
  if (event.source === "musicbrainz") {
    return `https://musicbrainz.org/recording/${event.external_id}`
  }
  if (event.source === "deezer") {
    return `https://www.deezer.com/track/${event.external_id}`
  }
  return null
}
```
Per RESEARCH.md's Anti-Patterns section, do NOT export `guestFeatureHref` to unit-test it directly — assert on the rendered `<a href>` instead: render a `guest_feature`-type `EventCard` with an `external_id` containing a character requiring encoding (e.g. `"abc def/g"`), then `screen.getByRole("link").toHaveAttribute("href", expect.stringContaining(encodeURIComponent("abc def/g")))`. GREEN fix wraps `event.external_id` in `encodeURIComponent(...)` at both interpolation sites (lines 110, 113).

**Bug 3 test and Bug 1 test both touch `EventCardBody`'s dispatch switch** (`EventCard.tsx:78-89`) — same file, can share fixture-building helpers within the test file (not exported, local to the test file, matching D-06's "no shared mock module" narrow-seam philosophy extended to fixtures).

---

## Shared Patterns

### API mocking (D-06)
**Source:** `web/app/lib/api.ts` (all named exports above)
**Apply to:** `PreferenceToggles.test.tsx`, `SearchBox.test.tsx`, `HistoryFilters.test.tsx`, `watchlist.test.tsx` (every file whose component imports a *function*, not just a type, from `~/lib/api`)
```tsx
vi.mock("~/lib/api")
const mockFn = vi.mocked(someApiFunction)
mockFn.mockResolvedValue(fixtureValue)   // or .mockRejectedValue(new Error(...)) / new ApiError(status, msg)
```
`EventCard.test.tsx` is the one exception — it imports only the `EventItem` type, so it needs no `vi.mock` at all (confirmed by direct read of `EventCard.tsx:3`).

### Router-context stub (D-03)
**Source:** RESEARCH.md Pattern 2, to be created at `web/app/lib/test/routeStub.tsx` (RESEARCH.md's recommended path — Claude's Discretion per Open Question 2, no locked path exists)
**Apply to:** `watchlist.test.tsx` only, for this phase's floor scope (no other named surface renders through the router).

### Toast-without-Toaster (Pitfall 5)
**Source:** `sonner`'s `toast` import pattern, used identically in `PreferenceToggles.tsx:2` and `watchlist.tsx:2`
**Apply to:** `PreferenceToggles.test.tsx`, `watchlist.test.tsx` — never render `<Toaster />`; add a defensive `ResizeObserver` stub to `vitest.setup.ts` only if a test actually throws `ReferenceError: ResizeObserver is not defined`.

### Go test naming/co-location philosophy (non-code pattern, D-02)
**Source:** `internal/watchlist/service_test.go` (read above) — `Test{FunctionName}_{Behavior}` naming (e.g. `TestService_Add_DuplicateReturnsErrDuplicate`), no table-driven tests, individual test functions with distinct descriptive names, `t.Helper()`-marked local helpers.
**Apply to:** All new `*.test.tsx` files — per RESEARCH.md's own note (line 76 of RESEARCH.md), the frontend suite should use idiomatic RTL/Vitest `it("does X when Y", ...)` sentence-style naming (matching RESEARCH.md's own code examples, e.g. `it("rolls back the optimistic toggle when the PATCH call fails", ...)`), NOT force Go's `Test_X_Y` identifier casing — but DOES carry over: co-location beside source, one-behavior-per-test (no table-driven equivalents), and small `t.Helper()`-style local fixture-building functions kept file-local rather than centralized.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `web/vitest.setup.ts` | config | — | No prior test setup file exists; RESEARCH.md Code Examples supplies the one-line verbatim content (`import "@testing-library/jest-dom/vitest"`) |
| `web/app/lib/test/routeStub.tsx` | utility | request-response | No shared frontend test-utility module exists yet in this codebase; RESEARCH.md Pattern 2 supplies verbatim content, loosely modeled on `internal/testutil/`'s "dedicated, reusable, non-test-suffixed package" precedent per RESEARCH.md Open Question 2 |

## Metadata

**Analog search scope:** `web/app/**`, `web/*.config.ts`, `internal/watchlist/*_test.go` (for naming-convention reference only), `.github/workflows/full-pipeline.yml` (referenced via RESEARCH.md, not independently re-read — its verified job block is already in RESEARCH.md's Code Examples)
**Files scanned:** `web/app/lib/api.ts`, `web/app/components/watchlist/{PreferenceToggles,SearchBox,WatchlistRow}.tsx`, `web/app/components/history/{EventCard,HistoryFilters}.tsx`, `web/app/routes/watchlist.tsx`, `web/vite.config.ts`, `web/tsconfig.json`, `web/package.json`, `internal/watchlist/service_test.go`
**Pattern extraction date:** 2026-08-12
