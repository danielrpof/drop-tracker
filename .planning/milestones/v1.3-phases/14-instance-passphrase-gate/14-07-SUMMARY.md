---
phase: 14-instance-passphrase-gate
plan: 07
subsystem: auth
tags: [instance-gate, spa, chi-middleware, response-header, sessionstorage, react, vitest]

requires:
  - phase: 14-instance-passphrase-gate
    provides: "gate.Authenticate middleware, authStore pub/sub, apiFetch single funnel, gateActive-gated Log out control (14-01/14-03), sessionStorage-backed gateActive (14-06)"
provides:
  - "Self-identifying gated responses: gate.Authenticate sets X-Instance-Gated: 1 on its proven-valid-cookie success path"
  - "authStore.markGateActive() — a one-way latch that sets/persists/notifies gateActive without ever reading or writing authed"
  - "apiFetch latches the X-Instance-Gated marker before its 401/204/!ok branches; never clears when the marker is absent"
  - "WR-01 retired: the typeof sessionStorage probe now sits inside the try in both storage helpers, covered by a throwing-accessor jsdom test"
  - "14-UAT.md Test 5 precondition opens with a G-14-3 re-run sequence (carried-over valid cookie, no typed passphrase)"
affects: [phase-17-vps-deploy, gsd-verify-work]

actuals:
  tokens: 7150
  tasks: 4
  commits: 6

tech-stack:
  added: []
  patterns:
    - "Two-sided byte-for-byte wire literal (X-Instance-Gated) pinned in the Go const, the TS const, and both test suites — same pattern as the X-Requested-With CSRF pair (D-15)"
    - "One-way monotonic client latch: early-return once set, never cleared, scoped to markGateActive only (not back-ported to markAuthenticated/markUnauthenticated)"

key-files:
  created: []
  modified:
    - "internal/authgate/gate.go — instanceGatedHeaderName/Value const block + one w.Header().Set on Authenticate's success path"
    - "internal/authgate/gate_test.go — positive marker case + negative matrix (gated 401, gated exempt route, ungated instance)"
    - "web/app/lib/authStore.ts — markGateActive() method; typeof probe moved inside the try in both storage helpers; header comment updated"
    - "web/app/lib/authStore.test.ts — 7 markGateActive cases + 1 throwing-accessor WR-01 case"
    - "web/app/lib/api.ts — INSTANCE_GATED_HEADER/VALUE consts + one latch call in apiFetch before the 401 branch"
    - "web/app/lib/api.test.ts — 6 latch cases + session-store reset in the auth-behaviour beforeEach"
    - "web/app/root.test.tsx — G-14-3 regression, deterministic + end-to-end (importActual rung)"
    - ".planning/phases/14-instance-passphrase-gate/14-UAT.md — Test 5 precondition G-14-3 re-run sub-block"

key-decisions:
  - "X-Instance-Gated: 1 is set ONLY on Authenticate's proven-valid-cookie path, before next.ServeHTTP, never on the two 401 returns — so the marker means exactly 'this response passed the gate' and its absence proves nothing (exempt routes carry none)"
  - "The marker is registered only inside server.go's gate != nil branch (Authenticate lives there), so D-18's ungated-instance guarantee now holds structurally rather than by the mere absence of a 401 — internal/httpserver/server.go was NOT modified"
  - "markGateActive is a one-way latch (early-returns once set); the early return is scoped to it only and must not be back-ported to the two mark* siblings, which keep their always-notify contract"
  - "WR-01 folded in as a severable fix commit (624e84b), not deferred — the throwing-accessor case IS reproducible in jsdom via a configurable getter on the global"
  - "D-18's gateActive trigger set is now three: an observed 401, a completed login, or an observed gated response — recorded here, not by editing 14-CONTEXT.md"

patterns-established:
  - "Response-header gating marker: a middleware stages a fixed header on w before next.ServeHTTP (precedent: setSessionCookie on the D-06 renewal path)"

requirements-completed: [GATE-06]

coverage:
  - id: D1
    description: "gate.Authenticate marks every response that passes the gate with X-Instance-Gated: 1 on its success path; a gated 401 and a gated exempt route carry nothing"
    requirement: GATE-06
    verification:
      - kind: integration
        ref: "internal/authgate/gate_test.go#TestGate_InstanceGatedMarker_PresentOnAuthenticatedSuccess"
        status: pass
      - kind: integration
        ref: "internal/authgate/gate_test.go#TestGate_InstanceGatedMarker_AbsentOnUnauthenticatedAndUngated"
        status: pass
    human_judgment: false
  - id: D2
    description: "An ungated instance (option-less httpserver.New) emits no marker — D-18's ungated guarantee holds structurally"
    requirement: GATE-06
    verification:
      - kind: integration
        ref: "internal/authgate/gate_test.go#TestGate_InstanceGatedMarker_AbsentOnUnauthenticatedAndUngated/ungated_instance_carries_nothing"
        status: pass
    human_judgment: false
  - id: D3
    description: "authStore.markGateActive() latches/persists/notifies gateActive once, never touches authed, never clears, is safe under hostile storage"
    requirement: GATE-06
    verification:
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#authStore — markGateActive latches a gated authed load (G-14-3)"
        status: pass
    human_judgment: false
  - id: D4
    description: "apiFetch latches the marker before the 401/204/!ok branches (every gated response shape) and never clears when the marker is absent"
    requirement: GATE-06
    verification:
      - kind: unit
        ref: "web/app/lib/api.test.ts#apiFetch auth behaviour — plan 14-07 Task 1 latch cases"
        status: pass
    human_judgment: false
  - id: D5
    description: "The Log out control appears on a clean authed load once the gate signal latches — no 401, no login, no reload — proven end to end (HTTP header → apiFetch → store → React)"
    requirement: GATE-06
    verification:
      - kind: integration
        ref: "web/app/root.test.tsx#latches the Log out control end-to-end"
        status: pass
      - kind: integration
        ref: "web/app/root.test.tsx#renders the Log out control on a clean authed load once the gate signal latches"
        status: pass
    human_judgment: false
  - id: D6
    description: "All three storage failure modes (absent store, throwing methods, throwing accessor) leave the gate inactive without throwing — WR-01 retired"
    requirement: GATE-06
    verification:
      - kind: unit
        ref: "web/app/lib/authStore.test.ts#does not throw and leaves the gate inactive when the sessionStorage accessor itself throws (WR-01)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Operator real-browser re-run of 14-UAT.md Test 5 against docker compose up --build: fresh session + carried-over valid dt_session cookie + gate ACTIVE → Log out control present on first authed view; ungated negative check"
    requirement: GATE-06
    verification: []
    human_judgment: true
    rationale: "G-14-3 was operator-reported from a real browser session on the built Docker image. The automated suite proves the whole chain in jsdom (header → apiFetch → store → rendered control) and the Go marker contract, but the end-to-end behaviour on the built image — which 14-UAT.md Test 5 reconciles against — needs a human in a real browser. Test 5's new G-14-3 re-run sub-block is the procedure."

duration: 25min
completed: 2026-09-01
status: complete
---

# Phase 14 Plan 07: Close G-14-3 — Self-Identifying Gated Responses Summary

**A gated instance now marks every response that passes `gate.Authenticate` with `X-Instance-Gated: 1`, and `apiFetch` latches that marker into a new one-way `authStore.markGateActive()` — so a browser session already holding a valid `dt_session` cookie renders the Log out control on its first authenticated load, with no 401, no typed login, and no reload.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-09-01T17:30:00Z (approx)
- **Completed:** 2026-09-01T17:55:00Z
- **Tasks:** 4
- **Files modified:** 8

## Accomplishments

- **G-14-3 closed.** The server attaches a fixed `X-Instance-Gated: 1` response header on `Authenticate`'s proven-valid-cookie path (before `next.ServeHTTP`, never on the two 401 returns). The SPA's single `apiFetch` funnel reads it and calls a new `authStore.markGateActive()`. A gated 2xx is now self-identifying — no new endpoint, no boot-time probe, no extra round trip (D-16 intact).
- **`markGateActive` is a one-way latch.** It sets `gateActive`, calls `persistGateActive()`, notifies once, then early-returns on every later call. It NEVER reads or writes `authed` — a gated response resolving after a 401 cannot resurrect a dead session (D-16). The early return is scoped to this method; `markAuthenticated`/`markUnauthenticated` keep their always-notify contract, guarded by a dedicated test.
- **D-18's ungated guarantee is now structural.** The marker's only write site is inside `Authenticate`, which `server.go` registers solely in the `gate != nil` branch, so an ungated instance emits nothing by construction. Proven by a Go test against an option-less `httpserver.New`.
- **WR-01 retired.** Both `readPersistedGateActive` and `persistGateActive` now run the `typeof sessionStorage` probe *inside* their existing `try`, so one `catch` covers all three failure modes: absent store, throwing methods, and a present-but-throwing property accessor. Covered by a jsdom test that redefines the global with a throwing getter — a standing human-verification item is retired.
- **14-UAT.md Test 5** precondition opens with a `G-14-3 re-run` sub-block: carried-over valid cookie, `dt_gate_active` cleared, no typed passphrase, no reload — so the operator's re-run exercises the defect instead of passing through the login path.

## Task Commits

1. **Task 1: RED — marker + latch regression coverage** — `0ef2978` (test)
   - Deviation fix (my own RED test reused one `Response` across two reads): `089c11a` (test)
2. **Task 2: GREEN — self-identifying gated responses, latched end to end** — `b170127` (feat) — `type="tracer"`
3. **Task 3: WR-01 storage-guard hardening** — `9da6dc8` (test, RED) then `624e84b` (fix)
4. **Task 4: 14-UAT.md Test 5 G-14-3 re-run sequence** — `be5f8e9` (docs)

**Plan metadata:** _(this SUMMARY + STATE/ROADMAP/REQUIREMENTS)_

## Output confirmations (per plan `<output>`)

- **Wire literal:** `X-Instance-Gated: 1`. Pinned on both implementation sides (`internal/authgate/gate.go` const `instanceGatedHeaderName`/`instanceGatedHeaderValue`; `web/app/lib/api.ts` const `INSTANCE_GATED_HEADER`/`INSTANCE_GATED_VALUE`) **and** independently in both test suites (`gate_test.go` inline literal ×10, `api.test.ts` ×5, `authStore.test.ts`/`root.test.tsx` via the marker in Response headers).
- **Header placement:** set only on `Authenticate`'s success path, as the first statement after `Verify` succeeds, before the sliding-renewal branch and before `next.ServeHTTP`. `internal/httpserver/server.go` and `web/app/root.tsx` were **NOT** modified (confirmed: `git diff 04cc449..HEAD` for those paths is empty).
- **Go negative matrix passes**, including the option-less ungated-server case (`TestGate_InstanceGatedMarker_AbsentOnUnauthenticatedAndUngated/ungated_instance_carries_nothing`).
- **`markGateActive`** does not touch `authed` (asserted), early-returns once latched (asserted: two calls → one notify), and never clears (asserted in `api.test.ts` "never clears" case). The two existing `mark*` functions kept their always-notify contract (asserted by "does not back-port the one-shot latch").
- **Task 1 root-level end-to-end ladder:** **Rung 1 (`vi.importActual`)** was used and works. `root.test.tsx` mocks `~/lib/api` at file scope; the e2e case calls `vi.importActual<typeof import("~/lib/api")>("~/lib/api")` after the `beforeEach` has reset the registry and imported the store, so the real api module and the test share one `authStore` instance. No conversion to a partial mock was needed; the case was not dropped.
- **Task 3 throwing-accessor test:** written and passing. `Object.defineProperty(globalThis, "sessionStorage", { configurable: true, get() { throw ... } })` with a `try/finally` restore of the captured descriptor. WR-01 does **not** remain a human-only check for the jsdom-reproducible case; the sandboxed-iframe real-browser case stays as 14-VERIFICATION human item 2 (out of this plan's automatable scope).
- **D-18 trigger set is now three** (an observed 401, a completed login, or an observed gated response). This strengthens the ungated guarantee — the `X-Instance-Gated` marker only exists on the gated code path — rather than weakening it. Recorded here; `14-CONTEXT.md` was not edited.

## Files Created/Modified

- `internal/authgate/gate.go` — `instanceGatedHeaderName`/`instanceGatedHeaderValue` const block with a byte-for-byte-contract comment; one `w.Header().Set(instanceGatedHeaderName, instanceGatedHeaderValue)` on `Authenticate`'s success path.
- `internal/authgate/gate_test.go` — `TestGate_InstanceGatedMarker_PresentOnAuthenticatedSuccess` + `TestGate_InstanceGatedMarker_AbsentOnUnauthenticatedAndUngated` (3 sub-cases).
- `web/app/lib/authStore.ts` — `markGateActive()` method (one-way latch, never touches `authed`); `typeof sessionStorage` probe moved inside the `try` in both storage helpers; header comment rewritten to name three triggers and three storage failure modes.
- `web/app/lib/authStore.test.ts` — new `markGateActive` describe (7 cases) + WR-01 throwing-accessor case in the D-18 describe.
- `web/app/lib/api.ts` — `INSTANCE_GATED_HEADER`/`INSTANCE_GATED_VALUE` consts; one latch call in `apiFetch` immediately after `fetch` resolves, before the 401 branch.
- `web/app/lib/api.test.ts` — 6 latch cases; `sessionStorage.clear()` as the first statement of the auth-behaviour `beforeEach`; `jsonResponse` extended with an optional headers arg.
- `web/app/root.test.tsx` — deterministic + end-to-end G-14-3 regression cases; `act` import added; `afterEach(vi.unstubAllGlobals)` for the gate describe.
- `.planning/phases/14-instance-passphrase-gate/14-UAT.md` — Test 5 precondition `G-14-3 re-run` sub-block (no `result`/`status`/`severity`/Gaps field touched).

## Decisions Made

See `key-decisions` frontmatter. In short: the marker is meaning-exact (success path only), structurally ungated-safe (registered only in the `gate != nil` branch), and the client latch is one-way and `authed`-blind by construction.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] RED test reused one `Response` across two `fetch` reads**
- **Found during:** Task 2 (running the full frontend suite after GREEN)
- **Issue:** The Task 1 "latches once" case used `fetchSpy.mockResolvedValue(jsonResponse(...))`, returning the same `Response` object twice. A `Response` body can only be read once, so the second `apiFetch` threw `TypeError: Body is unusable`.
- **Fix:** Switched to `fetchSpy.mockImplementation(() => Promise.resolve(jsonResponse(...)))` so each call gets a fresh `Response`.
- **Files modified:** `web/app/lib/api.test.ts`
- **Verification:** Full `vitest run` → 125/125 pass.
- **Committed in:** `089c11a` (separate `test` commit, kept out of the Task 2 diff so Task 2's diff-scope criterion stays exact).

---

**Total deviations:** 1 auto-fixed (1 bug, in this plan's own new test). **Impact:** none on the production diff or plan scope; the fix is a one-line test-helper change.

## Issues Encountered

- One acceptance-criterion grep (`sessionStorage.clear()` must precede the first `vi.resetModules()` in `api.test.ts`) initially failed because a comment I wrote contained the literal string `vi.resetModules()`, which `grep` counted. Reworded the comment to say "the module-registry reset". No behavioural impact.

## User Setup Required

None — no external service configuration. No new env var, route, schema, migration, or dependency (`go.mod`, `go.sum`, `web/package.json`, `web/pnpm-lock.yaml` byte-identical to pre-plan).

## Next Phase Readiness

- **Ready for `/gsd-verify-work`** to reconcile 14-UAT.md Test 5 against `gaps_closed: [G-14-3]`.
- **Operator human check remains** (coverage D7): real-browser re-run of 14-UAT.md Test 5 on `docker compose up --build` — the new `G-14-3 re-run` sub-block is the procedure. The committed `internal/webassets/build/client/` tree is deliberately not regenerated (14-06 carry-forward); the Dockerfile rebuilds `web/` from source.
- Reverting `b170127` alone restores exactly pre-plan behaviour (`reversible`); reverting `624e84b` independently leaves the G-14-3 closure intact.

## Self-Check: PASSED

- All 6 task commits present on `main` (`0ef2978`, `089c11a`, `b170127`, `9da6dc8`, `624e84b`, `be5f8e9`).
- All modified production files present on disk.
- `go build ./...`, `go vet ./...`, `go test ./...` → exit 0.
- `go test ./internal/authgate/ -run InstanceGated` → exit 0 (inverts Task 1's RED gate).
- `cd web && node_modules/.bin/vitest run` → 12 files / 125 tests pass; coverage 88.18 / 78.36 / 86.33 / 89.34 (all > 70).
- `cd web && node_modules/.bin/react-router build` → exit 0.
- `git diff 04cc449..HEAD` touches exactly the 8 files in `files_modified`; `go.mod`, `go.sum`, `web/package.json`, `web/pnpm-lock.yaml`, `internal/webassets/`, `web/app/root.tsx`, `internal/httpserver/server.go` all byte-identical to pre-plan.

---
*Phase: 14-instance-passphrase-gate*
*Completed: 2026-09-01*
