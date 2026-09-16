---
phase: 14-instance-passphrase-gate
plan: 06
subsystem: ui
tags: [react, sessionStorage, useSyncExternalStore, authStore, vitest, react-router]

# Dependency graph
requires:
  - phase: 14-instance-passphrase-gate (plan 14-03)
    provides: "authStore pub/sub module, gateActive/authed signals, D-18 LogoutButton gating in root.tsx"
  - phase: 14-instance-passphrase-gate (plan 14-05)
    provides: "G-14-1 close — gate engages end-to-end; boot-status log; Test 1 .env precondition"
provides:
  - "gateActive seeded from sessionStorage at module load and written through on both mark* functions (D-18 'in this browser session' implemented literally)"
  - "Guarded sessionStorage access (typeof + try/catch) on read, write, module load, and render path — safe under the react-router SPA-mode Node prerender and under a denying/throwing browser store"
  - "Regression coverage: the Log out control survives a simulated document reload with no 401 and no login (G-14-2)"
  - "14-UAT.md Test 5 re-run precondition (positive reload check + ungated negative check with the dt_gate_active clear step)"
affects: [gsd-verify-work, 14-UAT Test 5, phase-14 verification]

actuals:
  tokens: 3000
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Browser-session-scoped SPA signal: a single fixed literal under one sessionStorage key, seeded once at module load, never re-read by the useSyncExternalStore snapshot getter"
    - "Bidirectional storage guard: typeof check (Node prerender) + try/catch (denied/throwing browser store), documented in-file so a later reader does not simplify it away"

key-files:
  created: []
  modified:
    - "web/app/lib/authStore.ts — GATE_ACTIVE_STORAGE_KEY/VALUE consts, readPersistedGateActive()/persistGateActive() helpers, storage-seeded initialiser, write-through in both marks, rewritten header contract"
    - "web/app/lib/authStore.test.ts — 7 new cases (reload-survival, fresh-session, write-through x2, monotonic, undefined-storage, throwing-storage), per-case sessionStorage.clear() ordered before re-import, afterEach unstub"
    - "web/app/root.test.tsx — G-14-2 regression case (reload with seeded flag, no mark*, Log out still rendered) + sessionStorage.clear() in the gate describe's beforeEach"
    - ".planning/phases/14-instance-passphrase-gate/14-UAT.md — Test 5 precondition block"

key-decisions:
  - "Recorded value is the fixed literal \"1\" under key dt_gate_active — one key, one value, one read site, one write site (asserted by acceptance greps and by an explicit persisted-value test assertion)"
  - "isGateActive keeps returning the cached module boolean; storage is read exactly once at module load, never in the snapshot getter, preserving the useSyncExternalStore contract"
  - "authed is NOT persisted — stays volatile optimistic-true (D-16); root.tsx is NOT modified (D-18 branch already correct once the signal is durable)"

patterns-established:
  - "Session-scoped presentation signal backed by sessionStorage, not localStorage: a cross-session store would render a Log out control in a fresh session on an instance whose passphrase was since removed"

requirements-completed: [GATE-06]

coverage:
  - id: D1
    description: "gateActive survives a full document reload for the browser session — the Log out control stays rendered after a reload with a valid cookie, no 401, no login (G-14-2 truth / GATE-06)"
    requirement: GATE-06
    verification:
      - kind: unit
        ref: "web/app/root.test.tsx#keeps the Log out control after a document reload with a valid cookie — no 401, no login (G-14-2 regression)"
        status: pass
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#reports isGateActive() true on the first read after a reload that recorded the gate, with no mark* call"
        status: pass
    human_judgment: false
  - id: D2
    description: "Both mark* functions write the gate-active flag through to sessionStorage so either a 401 or a completed login makes the control durable (D-18)"
    requirement: GATE-06
    verification:
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#markUnauthenticated writes the gate-active flag through to the session store so a later reload sees it"
        status: pass
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#markAuthenticated writes the gate-active flag through to the session store so a later reload sees it"
        status: pass
    human_judgment: false
  - id: D3
    description: "Fresh/empty session store still yields isGateActive() false, and gateActive is monotonic within a session (logout/401/reload never clears it)"
    verification:
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#still reports isGateActive() false on a fresh browser session with an empty session store"
        status: pass
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#is monotonic within a session: a login then a logout-style markUnauthenticated still reports isGateActive() true after a reload"
        status: pass
    human_judgment: false
  - id: D4
    description: "Undefined sessionStorage and throwing sessionStorage both leave the gate inactive and never throw; module load, both marks, and the render path all complete normally"
    verification:
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#does not throw and leaves the gate inactive when sessionStorage is undefined"
        status: pass
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#does not throw and leaves the gate inactive when sessionStorage getItem/setItem throw"
        status: pass
    human_judgment: false
  - id: D5
    description: "react-router SPA-mode production build (ssr:false Node prerender) still succeeds — no module-level unguarded sessionStorage dereference"
    verification:
      - kind: other
        ref: "cd web && node_modules/.bin/react-router build"
        status: pass
    human_judgment: false
  - id: D6
    description: "14-UAT.md Test 5 carries an explicit re-run precondition: positive reload check plus the ungated negative check with the dt_gate_active clear step"
    verification:
      - kind: manual_procedural
        ref: "test $(grep -c '^precondition:' .planning/phases/14-instance-passphrase-gate/14-UAT.md) = 2 && grep -c dt_gate_active 14-UAT.md >= 1"
        status: pass
    human_judgment: false
  - id: D7
    description: "Operator re-run of UAT Test 5 in a real browser (docker compose up --build): Log out control persists across refresh + tab nav + add-artist, and is absent on an ungated instance after clearing the entry"
    verification: []
    human_judgment: true
    rationale: "The gap was operator-reported from a real browser session; the automated regression proves the module contract but the end-to-end reload behaviour on the built Docker image is what G-14-2 reconciliation (Test 5 result field) depends on, and that is done by /gsd-verify-work on resume off this plan's gap_ids."

# Metrics
duration: 15 min
completed: 2026-09-01
status: complete
gaps_closed: [G-14-2]
---

# Phase 14 Plan 06: Close G-14-2 — persist gateActive for the browser session Summary

**gateActive is now seeded from sessionStorage at module load and written through on both mark* functions, so the Log out control survives a full document reload for the life of the browser session — with authed still volatile, root.tsx untouched, and every storage access guarded for the Node prerender and a hostile browser store.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-01T03:06:00Z
- **Completed:** 2026-09-01T03:21:08Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- **G-14-2 fixed at its single root cause:** `web/app/lib/authStore.ts` initialiser changed from a literal `false` to `readPersistedGateActive()`. A document reload while `dt_session` is still valid now observes what the previous page load learned, so `useGateActive()` → the `{gateActive && <LogoutButton />}` branch in `root.tsx` renders the control with **no change to `root.tsx`**.
- **Write-through on both signals (D-18):** `markAuthenticated` and `markUnauthenticated` each call `persistGateActive()` before `notify()`, so either an observed 401 or a completed login makes the control durable for the rest of the browser session. The convergent, idempotent, always-notifies contract from 14-03 is preserved exactly — no early return, no dirty check.
- **Bidirectional storage guard:** every access goes through `readPersistedGateActive()` / `persistGateActive()`, each with a `typeof sessionStorage === "undefined"` check (the `react-router build` SPA-mode Node prerender evaluates this module) **and** a `try`/`catch` (a browser can deny or throw on storage). Both helpers return `false` / swallow on every failure and never throw. One read site (`sessionStorage.getItem`), one write site (`sessionStorage.setItem`).
- **Both hostile-storage branches covered by tests:** an `undefined` session store (exercises the `typeof` branch) and an object whose `getItem`/`setItem` throw (exercises the `catch` branch) — in both, module import does not throw, `isGateActive()` is `false`, and both `mark*` functions still complete, flip the in-memory gate, and notify subscribers.
- **`react-router build` succeeds** (`exit 0`) — the Node prerender evaluates `root.tsx` and transitively `authStore.ts` without a `ReferenceError`, so the Docker image build is unaffected.
- **`authed` was NOT persisted** — it stays a volatile module-level boolean initialised to optimistic `true` (D-16); a persisted copy would be a client-side authorization cache. Asserted by a comment-filtered negative grep and by the reload-survival test asserting `isAuthed()` is still `true` from optimism, not from storage.
- **14-UAT.md Test 5 precondition** added: the gated-instance prerequisite (Test 1 applies first), the positive reload sequence (unlock → confirm control → full reload → tab nav → add artist → reload again), and the ungated negative check requiring the operator to close the tab or clear the `dt_gate_active` Session Storage entry first.
- Full web suite green: **109 tests pass** (was 101; +8 cases), all four coverage axes above 70 (statements 87.93, branches 78.05, functions 86.23, lines 89.1). `go build ./...` and `go vet ./...` clean (no Go file touched).

## Task Commits

1. **Task 1: RED — regression coverage for gateActive surviving a document reload** — `8fac9ae` (test)
2. **Task 2: GREEN — back gateActive with sessionStorage** — `d28907e` (feat)
3. **Task 3: 14-UAT.md Test 5 re-run precondition** — `e337a79` (docs)

**Plan metadata:** _this commit_ (docs: complete plan)

_Task 2 is `type="tracer"`; its `<verify>` (full `vitest run` + `react-router build`) was re-run end-to-end and passed before proceeding to Task 3. Mode is `yolo` / auto-chain inactive, and Task 3 is a docs-only edit that does not build on Task 2's code, so no interactive tracer checkpoint was surfaced._

## Files Created/Modified

- `web/app/lib/authStore.ts` — `GATE_ACTIVE_STORAGE_KEY` (`dt_gate_active`) + `GATE_ACTIVE_STORAGE_VALUE` (`"1"`) consts; `readPersistedGateActive()` (guarded, one read site) and `persistGateActive()` (guarded, one write site); `let gateActive = readPersistedGateActive()`; `persistGateActive()` call in both marks before `notify()`; header comment rewritten to state the new contract, the D-18 scope, why `localStorage` was rejected, and why the guard exists in both directions.
- `web/app/lib/authStore.test.ts` — new describe block with 7 cases; `sessionStorage.clear()` as the first statement of `beforeEach` (ordered before `vi.resetModules()` and the dynamic import, with a comment explaining why); `afterEach(() => vi.unstubAllGlobals())`; `reimportStore()` helper; storage-key literal pinned from the test side; explicit persisted-value assertion in two cases.
- `web/app/root.test.tsx` — `sessionStorage.clear()` as the first statement of the gate describe's `beforeEach` (with comment); one new case: reload with a seeded `dt_gate_active` flag, re-import both the store and the root module, render, assert the Log out control is present, with no `mark*` call.
- `.planning/phases/14-instance-passphrase-gate/14-UAT.md` — `precondition:` block inserted between the Test 5 heading and its `expected:` line. Additions only — no `result`/`prior_result`/`status`/`severity`/`reported`/`root_cause`/`total`/`passed`/`issues` field modified, no line in the `## Gaps` section changed.

## Decisions Made

- **Recorded value is `"1"` under `dt_gate_active`** — one fixed literal, held in a module const beside the key so the write and the comparison cannot drift.
- **Storage is read exactly once, at module load.** `isGateActive` continues to return the cached module boolean because it is handed to `useSyncExternalStore` as both the client and the server snapshot; a getter that re-read storage on every call would break that hook's contract.
- **`sessionStorage`, not `localStorage`.** D-18 scopes the signal to this browser session. A cross-session store would keep a Log out control on screen in a brand-new session on an instance whose passphrase had since been removed — and that control would call an unregistered route.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Pre-existing `tsc --noEmit` error, out of scope:** `node_modules/.bin/tsc --noEmit -p tsconfig.json` exits non-zero with `app/root.tsx(19,28): error TS2307: Cannot find module './+types/root'` — a stale/missing `react-router typegen` artifact. Confirmed pre-existing by stashing this plan's `authStore.ts` change and re-running (same single error). It is not caused by this plan (no `root.tsx` edit, no type change to `authStore`'s surface), and `react-router build` — which runs typegen itself and is the plan's real build gate — passes cleanly. Logged to `.planning/phases/14-instance-passphrase-gate/deferred-items.md`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- G-14-2 is closed in code and proven by an automated regression test. `/gsd-verify-work` should reconcile 14-UAT.md Test 5 (`result: issue` → re-run) on resume off this plan's `gap_ids: [G-14-2]` frontmatter — the operator re-run in a real browser against `docker compose up --build` is the remaining human-judgment step (coverage D7).
- No blockers. `web/package.json` and lockfile untouched — zero new dependencies. Reverting `d28907e` alone restores today's behaviour (the `reversible` rating holds).

## Self-Check: PASSED

- `web/app/lib/authStore.ts`, `web/app/lib/authStore.test.ts`, `web/app/root.test.tsx`, `.planning/phases/14-instance-passphrase-gate/14-UAT.md` — all present on disk with the changes.
- Commits `8fac9ae`, `d28907e`, `e337a79` present in `git log`.
- `cd web && node_modules/.bin/vitest run` → exit 0, 109 passed, coverage 87.93/78.05/86.23/89.1 (all ≥ 70).
- `cd web && node_modules/.bin/react-router build` → exit 0.
- `cd web && node_modules/.bin/vitest run authStore root --coverage.enabled=false` → exit 0 (inverts Task 1 RED).
- `go build ./...` → exit 0; `go vet ./...` → exit 0.
- `git status --porcelain` shows no residual changes in `web/` or `14-UAT.md` from this plan; `web/app/root.tsx`, `web/app/lib/api.ts`, `web/package.json`, lockfiles, `internal/webassets/` all untouched.
- authStore.ts acceptance greps: `GATE_ACTIVE_STORAGE_KEY`=3, `sessionStorage.getItem`=1, `sessionStorage.setItem`=1, `typeof sessionStorage`=2, `try {`=2, `readPersistedGateActive`=3, `persistGateActive`=4, executable `localStorage`=0, executable `authed = readPersistedGateActive`=0.

---
*Phase: 14-instance-passphrase-gate*
*Completed: 2026-09-01*
