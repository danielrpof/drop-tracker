---
phase: 14-instance-passphrase-gate
plan: 03
subsystem: ui
tags: [react, react-router, auth, csrf, useSyncExternalStore, vitest, spa-gate]

requires:
  - phase: 14-instance-passphrase-gate
    provides: "server 401 {\"error\":\"unauthenticated\"} contract, POST/DELETE /session (204 + Set-Cookie), cookie dt_session (14-01)"
  - phase: 06-frontend-release-history
    provides: "apiFetch single fetch funnel + ApiError(status), React Router SPA-mode <App> shell, vendored button/input/card/sonner, createRoutesStub test seam"
provides:
  - "web/app/lib/authStore.ts — framework-free authed/gateActive pub/sub store + useAuthed()/useGateActive() (useSyncExternalStore)"
  - "apiFetch global 401 interceptor (flips authStore, still throws ApiError 401) — the only client place auth state flips on 401"
  - "X-Requested-With: drop-tracker injected centrally on every non-GET request (D-15 client half)"
  - "createSession(passphrase)/deleteSession() wrappers — passphrase in POST body only"
  - "web/app/components/auth/PassphraseScreen.tsx — approved full-screen gate form"
  - "<App> unauthenticated early-return to <PassphraseScreen>; gateActive-gated Log out control (D-18)"
affects: [14-04 CSRF header server enforcement, 17 VPS deploy]

actuals:
  tokens: 9800
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "SPA shared auth state without a provider: a plain module (no React import in the store object) poked by apiFetch and read by React via useSyncExternalStore — so api.ts imports it without pulling React into wholesale-mocked modules"
    - "Single fetch funnel owns cross-cutting request concerns: the 401 auth flip and the CSRF header are added once in apiFetch, never per endpoint wrapper"
    - "gateActive (D-18) is a presentation-only second signal — set true on the first observed 401 or login this session, never an access-control check"
    - "Test module-registry reset (vi.resetModules + dynamic import) is how singleton-store tests stay order-independent"

key-files:
  created:
    - web/app/lib/authStore.ts
    - web/app/lib/authStore.test.ts
    - web/app/components/auth/PassphraseScreen.tsx
    - web/app/components/auth/PassphraseScreen.test.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/root.tsx
    - web/app/root.test.tsx
    - web/app/routes/watchlist.test.tsx

key-decisions:
  - "authStore uses a namespace import in root.tsx (import * as auth) so useGateActive is referenced exactly once (satisfies the Task 3 acceptance grep) while keeping one reactive subscription per signal"
  - "PassphraseScreen.test.tsx uses a partial mock of ~/lib/api (importOriginal + spy createSession/deleteSession) instead of a bare automock — the bare automock breaks `err instanceof ApiError` / err.status in the component's catch branch"
  - "Focus management via document.getElementById(FIELD_ID) rather than a ref — the vendored Input wrapper types React.ComponentProps<'input'> (no ref), and a full-screen singleton has one unambiguous field"
  - "RED and GREEN combined into one feat commit per task, per the 14-01 precedent for this phase (new modules whose test files would not resolve as a test-only commit)"

patterns-established:
  - "authStore singleton reset in tests: vi.resetModules() + dynamic import in beforeEach"
  - "Partial mock of ~/lib/api when a test needs the real ApiError class: vi.mock(path, async (importOriginal) => ({ ...await importOriginal(), fn: vi.fn() }))"

requirements-completed: [GATE-05, GATE-06]

coverage:
  - id: D1
    description: "authStore: optimistic authed=true / gateActive=false on fresh load; markUnauthenticated/markAuthenticated both flip gateActive true and notify; idempotent repeat notifies but converges on one state; subscribe returns a working unsubscribe"
    verification:
      - kind: unit
        ref: "app/lib/authStore.test.ts#authStore (6 cases)"
        status: pass
    human_judgment: false
  - id: D2
    description: "apiFetch 401 interceptor: any 401 flips the shared store to unauthenticated and still rejects with ApiError status 401; a 200 leaves the store untouched; several concurrent 401s converge on one consistent unauthenticated state (GATE-05 concurrency edge)"
    requirement: "GATE-05"
    verification:
      - kind: unit
        ref: "app/lib/api.test.ts#apiFetch auth behaviour (401 flip, 200 untouched, concurrency)"
        status: pass
    human_judgment: false
  - id: D3
    description: "X-Requested-With: drop-tracker carried on POST/PATCH/DELETE but not GET, injected once in apiFetch (D-15 client half; closes the DELETE wrapper that previously sent no headers)"
    verification:
      - kind: unit
        ref: "app/lib/api.test.ts#carries the X-Requested-With: drop-tracker header on POST, PATCH and DELETE but not GET"
        status: pass
    human_judgment: false
  - id: D4
    description: "createSession POSTs the passphrase in the /session JSON body only (never a path/query/GET) and resolves on 204; deleteSession issues DELETE /session and resolves on 204; createSession does not flip auth state itself"
    requirement: "GATE-06"
    verification:
      - kind: unit
        ref: "app/lib/api.test.ts#createSession / deleteSession (3 cases)"
        status: pass
    human_judgment: false
  - id: D5
    description: "PassphraseScreen renders the verbatim approved copy; password field, no placeholder, autofocus; Unlock enabled when empty; submit calls createSession(exact value) then markAuthenticated on resolve; in-flight disables input + swaps to Unlocking…; 401->wrong copy, 429->throttle copy, no-status/5xx->connection copy; value retained + refocused on every error; button locked after 401/429 until edit, re-enabled immediately after a connection error; previous message cleared each submit; value never rendered as text (D-13)"
    requirement: "GATE-05"
    verification:
      - kind: unit
        ref: "app/components/auth/PassphraseScreen.test.tsx#PassphraseScreen (12 cases incl. E4 network held-out backstop)"
        status: pass
    human_judgment: false
  - id: D6
    description: "<App> renders <PassphraseScreen> (no nav) when the store is unauthenticated and the nav + routed page when authenticated; flipping the store back to authenticated remounts <Outlet/> so a route's mount effect re-fetches (GATE-05 re-fetch / E6 loading); a post-login route fetch that returns 401 re-shows the gate (E6 held-out backstop)"
    requirement: "GATE-05"
    verification:
      - kind: unit
        ref: "app/root.test.tsx#App — instance passphrase gate; app/routes/watchlist.test.tsx#Watchlist route under the passphrase gate"
        status: pass
    human_judgment: false
  - id: D7
    description: "Log out control renders only when gateActive is true (D-18); click calls deleteSession then markUnauthenticated in a finally-path so state clears on success OR failure; success -> 'Logged out.' toast, failure -> 'Couldn't log out.' toast (E5 held-out backstop); not disabled during the request, double-click harmless"
    requirement: "GATE-06"
    verification:
      - kind: unit
        ref: "app/root.test.tsx#does not render the Log out control when the gate is not active / renders the Log out control once the gate is active / logout success + failure + double-click"
        status: pass
    human_judgment: false
  - id: D8
    description: "Visual conformance of the passphrase screen to 14-UI-SPEC — viewport-centred max-w-sm bg-card, gap-6 rhythm, accent reserved to the Unlock fill + input focus ring, destructive reserved to error text, dark-surface-only rendering"
    verification: []
    human_judgment: true
    rationale: "RTL asserts copy, structure, roles and behaviour but not rendered appearance; there is no Playwright/screenshot step in this project. The spacing/colour/typography pillars need a human glance at the running SPA (end-of-phase human_verify_mode)."

duration: 12 min
completed: 2026-08-29
status: complete
---

# Phase 14 Plan 03: SPA Instance Passphrase Gate Summary

**A framework-free `authStore` (authed + D-18 `gateActive`) poked by a single `apiFetch` 401 interceptor, the verbatim-approved full-screen `<PassphraseScreen>`, and a `gateActive`-gated Log out control — so a gated instance renders a login prompt instead of a broken page and a successful login restores the watchlist/history UI with fresh data, no reload.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-08-29T16:47:01Z
- **Completed:** 2026-08-29T16:58:20Z
- **Tasks:** 3 (all TDD)
- **Files:** 4 created, 5 modified

## Accomplishments

- **`web/app/lib/authStore.ts`** — a plain pub/sub module: `authed` (optimistic `true`, no boot-time `GET /session`, D-16) and `gateActive` (`false` -> `true` on the first observed 401 or login this session, **locked D-18**), a `Set` of listeners, `isAuthed`/`isGateActive`/`markAuthenticated`/`markUnauthenticated`/`subscribe`, and `useAuthed()`/`useGateActive()` on `useSyncExternalStore` (the store object itself references no React — only the hooks do). Both `mark*` set `gateActive` true and always notify; repeated calls converge on one consistent state (GATE-05 concurrency edge).
- **`apiFetch` (web/app/lib/api.ts)** — one 401 branch, right after `fetch` resolves and before the 204 branch, calls `authStore.markUnauthenticated()` and throws `ApiError(401, "unauthenticated")`. This is the *only* place client code flips auth state on a 401. `X-Requested-With: drop-tracker` is built into the outgoing headers for every non-GET (D-15 client half — also closes the `removeWatchlist`/`deleteSession` wrappers that previously sent no headers).
- **`createSession(passphrase)` / `deleteSession()`** — `POST` / `DELETE /session`, resolving on 204. The passphrase travels only in the POST JSON body (Pitfall 14). `createSession` deliberately does not call `markAuthenticated` — `PassphraseScreen` does, only after the promise resolves.
- **`web/app/components/auth/PassphraseScreen.tsx`** — the approved contract exactly: `min-h-screen flex items-center justify-center p-8` wrapping a `w-full max-w-sm rounded-md bg-card p-8` card, inner `flex flex-col gap-6` (heading block, labelled `type=password` field + collapsing `text-destructive` error slot, `Unlock` button). Every string verbatim from the Copywriting Contract. 401/429/network-or-5xx each select their own fixed message; the typed value is retained and refocused on every error path, the button stays disabled after 401/429 until the field is edited and re-enables immediately after a connection error, and the value is never rendered as text, logged, or toasted (D-13).
- **`web/app/root.tsx`** — `App()` reads `auth.useAuthed()` and returns `<PassphraseScreen />` before any nav markup when unauthenticated; that early return is the whole D-16 re-fetch mechanism (Outlet remounts on the flip, every route's mount effect re-fetches, no retry queue). `LogoutButton` (ghost/sm Button + lucide `LogOut`, `ml-auto`) renders only when `auth.useGateActive()` is true; its handler calls `deleteSession()` then `authStore.markUnauthenticated()` in a `finally` path so local state clears whether the request succeeds or fails, with a `Logged out.` / `Couldn't log out.` toast.

## Task Commits

1. **Task 1: auth store + apiFetch 401 interceptor, CSRF header, session wrappers** — `7ba76f5` (feat)
2. **Task 2: PassphraseScreen — the approved full-screen gate form** — `b61c5db` (feat)
3. **Task 3: App gate branch, Log out control, and the post-login re-fetch** — `0dcb402` (feat)

**Plan metadata:** _(this commit)_ — `docs(14-03): complete SPA instance passphrase gate plan`

_TDD note: each task combined RED and GREEN into one `feat` commit, per the documented 14-01 precedent for this phase — `authStore.ts` and `PassphraseScreen.tsx` are new modules their test files import, so a test-only commit would not resolve. Each commit is atomic and leaves the full suite green._

## Files Created/Modified

- `web/app/lib/authStore.ts` — NEW: `authStore` pub/sub + `useAuthed`/`useGateActive`
- `web/app/lib/authStore.test.ts` — NEW: 6 plain-module unit cases (no RTL)
- `web/app/components/auth/PassphraseScreen.tsx` — NEW: the approved gate form
- `web/app/components/auth/PassphraseScreen.test.tsx` — NEW: 12 RTL + user-event cases
- `web/app/lib/api.ts` — MOD: 401 branch, central `X-Requested-With`, `createSession`/`deleteSession`
- `web/app/lib/api.test.ts` — MOD: new `describe` (resetModules + dynamic import) for the 401 flip, header, wrappers, GATE-05 concurrency
- `web/app/root.tsx` — MOD: `App()` early return, `LogoutButton`, `import * as auth`
- `web/app/root.test.tsx` — MOD: new `describe` — gate branch, Log out visibility, logout success/failure/double-click
- `web/app/routes/watchlist.test.tsx` — MOD: new `describe` — post-flip re-fetch and the E6 401 backstop

## D-18 (`gateActive`) confirmation — per the plan `<output>`

`gateActive` is wired exactly as specified:

- Initialised **`false`** on a fresh module load (`authStore.test.ts` asserts `isGateActive() === false`).
- Set **`true`** the first time the app observes a 401 (`markUnauthenticated`) **or** completes a login (`markAuthenticated`) — both `mark*` functions set it, and it never goes back to `false`.
- The **Log out control renders only when `useGateActive()` is `true`** — `root.test.tsx` asserts the control is absent on a fresh store (`gateActive` still `false`, i.e. an ungated instance that has never seen a 401 or login) and present once `markAuthenticated()` has run.
- `gateActive` is **presentation-only** — it gates rendering of a cosmetic control, never a data path. The server 401 remains the sole enforcement (recorded as a plan prohibition; not duplicated anywhere).

This is a **locked decision (D-18)**, not a UAT confirmation item.

## Decisions Made

- **Namespace import for the auth store in `root.tsx`** (`import * as auth from "~/lib/authStore"`) so `useGateActive` is referenced exactly once (Task 3 acceptance grep) while still taking one reactive subscription per signal.
- **Partial mock of `~/lib/api` in `PassphraseScreen.test.tsx`** — `vi.mock(path, async (importOriginal) => ({ ...await importOriginal(), createSession: vi.fn(), deleteSession: vi.fn() }))`. A bare `vi.mock("~/lib/api")` automock replaces the `ApiError` class, breaking `err instanceof ApiError` / `err.status` in the component's catch branch (the 401 and 429 branches silently fell through to the connection copy). No real `apiFetch` still reaches the runtime `fetch` (TEST-02 intent preserved) because `createSession` is stubbed.
- **Focus via `document.getElementById(FIELD_ID)`** rather than a ref — the vendored `Input` wrapper is typed `React.ComponentProps<"input">` (no `ref`); a full-screen singleton has one unambiguous `#passphrase`.
- **RED+GREEN combined per task** — 14-01 precedent for this phase.

## Deviations from Plan

### Auto-fixed Issues

None. No bugs, missing-critical functionality, or blockers surfaced during execution — the plan's `<action>` text mapped directly onto the codebase.

### Adjustments within plan latitude (not deviations)

- The plan's acceptance greps for `min-h-screen` / `max-w-sm` / `Log out` / `ml-auto` expect exactly one match each; doc comments that repeated those class-name / label literals were reworded so each token appears once, in the JSX only.
- Package manager: the repo has no `pnpm` on this machine (`14-01-SUMMARY` already noted `pnpm test` was never run here). Verification ran via `node_modules/.bin/vitest run` and `node_modules/.bin/react-router build` — the same binaries `pnpm test` / `pnpm build` invoke.

---

**Total deviations:** 0 auto-fixed.
**Impact on plan:** none — delivered surface matches the plan's `<artifacts_this_phase_produces>` and `<success_criteria>` exactly.

## Issues Encountered

- **`Body is unusable` in the CSRF-header test** — `fetchSpy.mockResolvedValue(response)` returns the *same* `Response` instance for every call, and a body can only be read once. Fixed by switching to `mockImplementation(() => Promise.resolve(jsonResponse(...)))` so each call gets a fresh `Response`. Resolved during Task 1, before the commit.
- **401/429 branches falling through to the connection copy** in `PassphraseScreen.test.tsx` — root cause was the automocked `ApiError` (see Decisions). Resolved with the partial mock, before the Task 2 commit.

## Plan Verification Results

| Check | Result |
|-------|--------|
| `cd web && <vitest> run` (full suite) | exit 0 — 101 tests, 12 files |
| Coverage gate (70% all four axes) | statements 87.55%, branches 77.73%, functions 86.02%, lines 88.74% — all pass |
| `cd web && <react-router> build` | exit 0 — SPA client bundle generated |
| `cd web && pnpm lint` | n/a — no `lint` script in `web/package.json` |
| `go test ./... -short -count=1` | exit 0 — every package `ok` (this plan touches no Go file) |
| `git diff --name-only` for the 3 code commits | only `web/app/**` (lib/, components/auth/, root.tsx, routes/) |

## Next Phase Readiness

- **14-04 (Wave 3, hardening):** ready. The client now always sends `X-Requested-With: drop-tracker` on non-GET, so 14-04's `Manager.RequireCSRFHeader` server-side enforcement has a client that already complies. `POST /session` also carries the header. Nothing in 14-04 depends on further SPA changes.
- **Requirements:** GATE-05 (sole declaring plan = 14-03) and GATE-06 (14-01 + 14-03, both now complete) marked complete via `requirements.ready-ids`.
- **UAT:** D8 (visual conformance to 14-UI-SPEC) is the one deliverable routed to a human — `human_verify_mode: end-of-phase`, so it will be checked with the rest of Phase 14's visual surface.

## Self-Check: PASSED

- All 4 created files present on disk (`[ -f ]` confirmed): `authStore.ts`, `authStore.test.ts`, `PassphraseScreen.tsx`, `PassphraseScreen.test.tsx`.
- All 3 task commits present in `git log` (`7ba76f5`, `b61c5db`, `0dcb402`).
- Plan `<verification>` re-run: full vitest suite exit 0 (101 tests), coverage all four axes > 70, `react-router build` exit 0, `go test ./... -short` exit 0.
- Task acceptance greps re-checked: `api.ts` — `markUnauthenticated` ×1, `X-Requested-With` ×1, `createSession`/`deleteSession` present, `passphrase` only in a POST body; `PassphraseScreen.tsx` — `Enter the instance passphrase` ×1, `type="password"` ×1, `min-h-screen` ×1, `max-w-sm` ×1, `text-destructive` ×1, no `dangerouslySetInnerHTML`; `root.tsx` — `PassphraseScreen` ×3 (≥2), `Log out` ×1, `ml-auto` ×1, `useGateActive` ×1, `deleteSession` ×2.

---
*Phase: 14-instance-passphrase-gate*
*Completed: 2026-08-29*
