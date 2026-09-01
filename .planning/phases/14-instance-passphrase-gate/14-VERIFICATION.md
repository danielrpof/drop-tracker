---
phase: 14-instance-passphrase-gate
verified: 2026-09-01T18:07:04Z
status: human_needed
score: 7/7 must-haves verified (GATE-01..GATE-07 in code + tests; G-14-1, G-14-2, G-14-3 all closed in code and covered by passing automated regressions)
behavior_unverified: 0
overrides_applied: 0
overrides: []
re_verification:
  previous_status: human_needed
  previous_score: "8/8 — GATE-01..GATE-07 + G-14-2; blocked at UAT Test 5 by G-14-3"
  gaps_closed:
    - "G-14-3 — on a gated instance, a browser session that already holds a valid dt_session cookie and performs NO typed login and receives NO 401 rendered no Log out control on its first authenticated load, because the SPA had no way to discover the instance was gated (authStore.gateActive was set true only by markAuthenticated on a typed login or markUnauthenticated on a 401; a clean-cookie session saw neither). Plan 14-07: (a) internal/authgate/gate.go Authenticate now sets X-Instance-Gated: 1 on w.Header() on its proven-valid-cookie success path (gate.go:200), before next.ServeHTTP, and on neither 401 return path; (b) the const block instanceGatedHeaderName/Value (gate.go:127-130) is a byte-for-byte contract mirrored in web/app/lib/api.ts INSTANCE_GATED_HEADER/VALUE (api.ts:18-19); (c) apiFetch latches the marker (api.ts:146-148) BEFORE the 401/204/!ok branches, calling the new authStore.markGateActive(); (d) markGateActive (authStore.ts:171-178) is a one-way latch — early-returns once gateActive is set, writes gateActive only, never reads or writes authed (D-16), never clears; (e) the marker's only write site is inside Authenticate, which server.go registers solely in the gate != nil branch (server.go:172-173), so an ungated instance emits nothing structurally (D-18); (f) web/app/root.tsx and internal/httpserver/server.go are unmodified (git diff empty); (g) Go marker matrix TestGate_InstanceGatedMarker_* passes (positive + gated-401 + gated-exempt-route + option-less-ungated-server all assert empty string on the negatives); (h) frontend suite 125/125, coverage 88.18/78.36/86.33/89.34 (all > 70); root.test.tsx end-to-end case drives a real 200 carrying the header through the real apiFetch into the real authStore into React and asserts the Log out control mounts; (i) react-router build exit 0."
    - "WR-01 (14-VERIFICATION prior residual — the typeof sessionStorage probe sat OUTSIDE the try/catch in readPersistedGateActive/persistGateActive, so a present-but-throwing storage accessor could white-screen the SPA at module init). Plan 14-07 Task 3 moved the typeof probe INSIDE the existing try in both helpers (authStore.ts:96-97 and :109-110; exactly two try blocks, one per helper), so one catch now covers all three failure modes. Covered by an automated jsdom test that redefines globalThis.sessionStorage with a throwing getter (authStore.test.ts:192). Prior human item 2 is retired."
  gaps_remaining: []
  regressions: []
behavior_unverified_items: []
human_verification:
  - test: "Re-run 14-UAT.md Test 5 in a real browser against `docker compose up --build` with the gate ACTIVE (boot log `logInstanceGateStatus` confirms). Follow Test 5's new `G-14-3 re-run` sub-block FIRST: from a session that has already unlocked, either close the tab and open a new one OR delete only the `dt_gate_active` Session Storage entry in devtools (do NOT use the Log out control, do NOT clear cookies — the `dt_session` cookie must survive). Open the app in that fresh tab: you must be let straight in with no passphrase form AND the Log out control must be present in the nav on that first authenticated view, once the first data request completes — with no passphrase typed and no reload. Then the positive check: unlock with the passphrase, full browser refresh (control persists, no re-login), navigate Watchlist <-> History, add an artist, refresh again (control persists for the browser session). Then the negative check: clear `dt_gate_active` (or close the tab) and load an instance with NO passphrase configured — the Log out control must be ABSENT."
    expected: "On a gated instance the Log out control is present on the first authenticated view of a carried-cookie session and survives refresh + tab-nav + add-artist for the browser session; on an ungated instance it is absent after the storage entry is cleared. Access to the watchlist never depends on the control being visible — the server 401 is the sole enforcement."
    why_human: "G-14-3 was operator-reported from a real browser session on the built Docker image. The automated suite proves the entire chain (Go marker contract; header -> apiFetch -> store -> rendered control end-to-end in jsdom), but 14-UAT.md Test 5 still records `result: issue` and reconciles against real-browser behaviour on the built image. Coverage item D7 is `human_judgment: true`."
gaps: []
---

# Phase 14: Instance Passphrase Gate — Verification Report (re-verification after G-14-3 closure, plan 14-07)

**Phase Goal:** A drop-tracker instance on a public URL only exposes its watchlist data, search proxy, and event history to someone who knows the instance passphrase — while local dev, docker-compose, and the existing test suites keep working with no passphrase configured at all. This must land before anything in this milestone makes the app publicly reachable, so the instance is never briefly public-and-open.

**Verified:** 2026-09-01
**Status:** human_needed
**Re-verification:** Yes — after G-14-3 gap closure (plan 14-07)

## Re-verification Summary

Rounds 14-01…14-05 verified all seven GATE requirement IDs implemented, wired, and test-covered (8/8), with UAT Tests 1–4 operator-PASS. G-14-1 (gate inert in the compose stack) closed by 14-05. G-14-2 (Log out control lost on a full document reload) closed by 14-06 with a passing regression. UAT round 3 then opened **G-14-3** (severity: major): on a gated instance a browser session already holding a valid `dt_session` cookie — no typed login, no 401 — rendered **no Log out control** on its first authenticated load, because the SPA had no signal that the instance was gated at all. 14-06's `sessionStorage` seeding only helped *after* a 401 or a login had recorded the flag once.

**Plan 14-07 closed G-14-3.** Verified directly against the codebase:

| Fix | Evidence | Status |
|-----|----------|--------|
| `Authenticate` marks every proven-valid-cookie response | `internal/authgate/gate.go:200` — `w.Header().Set(instanceGatedHeaderName, instanceGatedHeaderValue)` staged before `next.ServeHTTP`, after `Verify` succeeds; not on either of the two `writeJSONError(... 401 ...)` returns (`:184`, `:190`) | ✓ VERIFIED |
| Marker is a two-sided byte-for-byte contract | `gate.go:127-130` const `instanceGatedHeaderName="X-Instance-Gated"` / `instanceGatedHeaderValue="1"`; `web/app/lib/api.ts:18-19` `INSTANCE_GATED_HEADER` / `INSTANCE_GATED_VALUE` identical; contract comment on each side names the other file | ✓ VERIFIED |
| `apiFetch` latches before the 401 / 204 / !ok branches | `api.ts:146-148` — `if (res.headers.get(INSTANCE_GATED_HEADER) === INSTANCE_GATED_VALUE) authStore.markGateActive()`, placed immediately after `await fetch`, above `res.status === 401` (`:155`) | ✓ VERIFIED |
| `markGateActive` is a one-way latch, `authed`-blind | `authStore.ts:171-178` — early-returns on `gateActive`; sets `gateActive`, `persistGateActive()`, `notify()`; never references `authed` | ✓ VERIFIED |
| Ungated instance emits nothing — structurally | `internal/httpserver/server.go:166-182` — `gate.Authenticate` registered only inside `if gate != nil` `r.Group`; `else` branch calls `registerDataRoutes(r, s)` flat. Marker's only write site is inside `Authenticate`. Not added to `securityResponseHeaders` (`:211-216`, unchanged). Go test `.../ungated_instance_carries_nothing_(D-18)` asserts empty string | ✓ VERIFIED |
| `web/app/root.tsx` NOT modified | `git diff 04cc449..HEAD -- web/app/root.tsx internal/httpserver/server.go` empty; `root.tsx:118` still `{gateActive && <LogoutButton />}` | ✓ VERIFIED |
| WR-01 (prior residual) retired | `authStore.ts` — `try {` at `:96` precedes `typeof sessionStorage` at `:97`; `try {` at `:109` precedes `typeof` at `:110`; exactly two `try {` in the file. `authStore.test.ts:192` drives a throwing `sessionStorage` getter and asserts no throw + gate inactive | ✓ VERIFIED |
| Regression coverage | `internal/authgate/gate_test.go` — `TestGate_InstanceGatedMarker_PresentOnAuthenticatedSuccess` + `TestGate_InstanceGatedMarker_AbsentOnUnauthenticatedAndUngated` (3 sub-cases). `go test ./internal/authgate/ -run InstanceGated` → PASS. `web/app/lib/authStore.test.ts` +7 `markGateActive` cases + 1 WR-01 case; `web/app/lib/api.test.ts` +6 latch cases; `web/app/root.test.tsx` +2 G-14-3 cases (deterministic + end-to-end via `vi.importActual`). `vitest run` → **125/125 pass**, coverage 88.18 / 78.36 / 86.33 / 89.34 (all > 70) | ✓ VERIFIED |
| SPA-mode production build | `cd web && node_modules/.bin/react-router build` → **exit 0**, `build/client/index.html` generated | ✓ VERIFIED |
| Backend suite unaffected | `go build ./...` exit 0; `go test ./...` — all packages PASS | ✓ VERIFIED |
| Diff scope | `git diff --stat 04cc449..HEAD` — production changes confined to `internal/authgate/gate.go`, `web/app/lib/api.ts`, `web/app/lib/authStore.ts` (+ test files + planning docs). `go.mod`, `go.sum`, `web/package.json`, lockfile, `internal/webassets/`, `root.tsx`, `server.go` untouched | ✓ VERIFIED |

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | **SC1 / GATE-01** — with a passphrase set, `/search` `/watchlist` `/events` `/watchlist/{id}` return `401` without a valid cookie; `/health`, `POST/DELETE /session`, SPA shell + static bundle stay public | ✓ VERIFIED | `server.go:164-187` structural exemption (data routes only inside `r.Group{ Authenticate; RequireCSRFHeader }`); `/health` an exact path, `/session` registered outside the group; `NotFound` → SPA in both branches. Named backend tests pass. UAT Test 1 PASS. No backend enforcement change in 14-07. |
| 2 | **SC2 / GATE-05** — opening a gated instance shows a passphrase form; correct passphrase restores the UI, wrong does not | ✓ VERIFIED | `api.ts:155-158` single-401 funnel → `authStore.markUnauthenticated()` → `root.tsx:105` early return `<PassphraseScreen/>`; `createSession` (`api.ts:276`) only resolves on 204, `PassphraseScreen` calls `markAuthenticated()` after resolve. UAT Test 1 PASS. |
| 3 | **SC3 / GATE-02, GATE-06** — browser stays authenticated across a container restart; Log out returns to the form | ✓ VERIFIED | Stateless HMAC cookie (`authgate` `Verify`/`Sign`), no server store; sliding renewal on the success path (`gate.go:201-208`); `HandleLogout` → 204 + `Max-Age=0`; `LogoutButton` (`root.tsx:66-89`) clears local state in `finally`. UAT Test 1 PASS. |
| 4 | **SC4 / GATE-03, GATE-04** — cookie `HttpOnly`/`Secure`/`SameSite=Lax`/bounded lifetime; per-IP `429` throttle undelayed; constant-time compare; bounded login concurrency | ✓ VERIFIED | `gate.go:218-228` `setSessionCookie`; `subtle.ConstantTimeCompare` + `hmac.Equal` in the login path; `loginThrottle` + `loginSlots` semaphore + sweeper goroutine. Named tests pass. UAT Test 4 PASS (live Discord brute-force alert). |
| 5 | **SC5 / GATE-07** — with no passphrase every route behaves as pre-v1.3; test suites + `docker compose up` pass with no passphrase | ✓ VERIFIED | `server.go:180-182` `else` branch registers `registerDataRoutes(r, s)` flat — no `/session`, no `Authenticate`, no `RequireCSRFHeader`. Marker never emitted (only write site is inside `Authenticate`). `go test ./...` all pass; `vitest run` 125/125; `react-router build` exit 0 — all with no passphrase. UAT Test 3 PASS. |
| 6 | **GATE-06 (user-facing) / G-14-2** — the Log out control stays available for the whole browser session after a full document reload while the `dt_session` cookie is still valid | ✓ VERIFIED | `authStore.ts:123` storage-seeded `gateActive` init; `:140`/`:147` write-through in both `mark*`; `root.test.tsx:189` G-14-2 regression (reload with seeded flag, no `mark*`, Log out still rendered) passes. |
| 7 | **GATE-06 (user-facing) / G-14-3** — on a gated instance, a browser session already holding a valid `dt_session` cookie renders the Log out control on its first authenticated load — no 401, no typed login, no reload | ✓ VERIFIED (code + automated end-to-end regression) → real-browser re-run of UAT Test 5 is the outstanding human check | Server marker (`gate.go:200`), two-sided contract, `apiFetch` latch (`api.ts:146`), `markGateActive` latch (`authStore.ts:171`). `root.test.tsx:229` end-to-end case: real 200 + `X-Instance-Gated` header → real `apiFetch` → real `authStore` → React → Log out control mounts. Go negative matrix passes incl. option-less ungated server. `react-router build` exit 0. UAT Test 5 (`result: issue`) needs a human re-run on the built Docker image (D7 `human_judgment: true`). |

**Score:** 7/7 truths verified in code and by passing automated regressions (Go marker matrix + frontend 125/125 incl. an end-to-end chain test + SPA build). UAT Tests 1–4 operator-PASS. One human item remains: the real-browser re-run of UAT Test 5 (G-14-3's origin, coverage D7). No `behavior_unverified` truths — the marker→latch→render state transition is exercised by an automated end-to-end test; the human item concerns the built Docker image + real browser cookie handling, not an unexercised transition.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/authgate/gate.go` | marker const block + one `w.Header().Set` on `Authenticate`'s success path before `next.ServeHTTP` | ✓ VERIFIED | Const block `:127-130` with byte-for-byte contract comment `:105-126`; single write site `:200`; on neither 401 return. `grep -c 'X-Instance-Gated'` = 3 (comment + 2 const lines). |
| `internal/authgate/gate_test.go` | positive marker case + negative matrix (gated 401, gated exempt route, ungated instance) | ✓ VERIFIED | `:752` positive (asserts `== "1"`); `:780` negative matrix, 3 sub-cases each asserting `== ""`. `go test -run InstanceGated` PASS. |
| `web/app/lib/authStore.ts` | `markGateActive()` one-way latch, `authed`-blind; `typeof` probe inside `try` in both helpers | ✓ VERIFIED | `:171-178` latch; `:96-97`/`:109-110` probe inside `try`; exactly 2 `try {`; exactly 1 `getItem` + 1 `setItem`; 0 `localStorage` in executable code. Exported surface unchanged (`isAuthed`, `isGateActive`, `markAuthenticated`, `markUnauthenticated`, `markGateActive`, `subscribe` + `useAuthed`/`useGateActive`). |
| `web/app/lib/authStore.test.ts` | markGateActive coverage (latch, persist, notify-once, `authed` untouched, no-resurrect-after-401, hostile storage) + WR-01 throwing-accessor | ✓ VERIFIED | New describe `:232` (7 cases incl. `never resurrects a dead session` `:249`); WR-01 case `:192`. All pass. |
| `web/app/lib/api.ts` | marker consts + one latch call in `apiFetch` before the 401 branch | ✓ VERIFIED | Consts `:18-19`; single latch `:146-148` above `res.status === 401` (`:155`). `grep -c markGateActive` = 1. |
| `web/app/lib/api.test.ts` | latch across 2xx / 204 / non-OK, no-header no-latch, never-clears, session-store reset | ✓ VERIFIED | 6 cases `:284-340`; `sessionStorage.clear()` first in the auth-behaviour `beforeEach`. All pass. |
| `web/app/root.test.tsx` | G-14-3 regression — Log out control appears on a clean authed load, no 401/login/reload | ✓ VERIFIED | `:215` deterministic (`markGateActive` inside `act`, absent→present transition); `:229` end-to-end (`vi.importActual` for api, stubbed `fetch` resolving a marker-carrying 200, real `listWatchlist()` inside `act`). Rung 1 of the ladder used. Both pass. |
| `web/app/root.tsx` | unchanged | ✓ VERIFIED | `git diff` empty; D-18 branch `:118` unchanged. |
| `internal/httpserver/server.go` | unchanged | ✓ VERIFIED | `git diff` empty; `Authenticate` still registered only in `gate != nil` branch. |
| `.planning/phases/14-instance-passphrase-gate/14-UAT.md` | Test 5 precondition gains a `G-14-3 re-run` sub-block | ✓ VERIFIED | `grep -c '^precondition:'` = 2; `grep -c 'G-14-3 re-run'` ≥ 1; `grep -c 'dt_gate_active'` = 2. No `result`/`prior_result`/`status`/`severity`/`reported`/`root_cause`/`total`/`passed`/`issues` line modified; `## Gaps` section unchanged (G-14-3 entry present, `status: failed`, awaiting this reconciliation). |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `gate.Authenticate` success path | `X-Instance-Gated: 1` on the `ResponseWriter` | `w.Header().Set(instanceGatedHeaderName, instanceGatedHeaderValue)` at `gate.go:200`, before `next.ServeHTTP` (`:209`) | ✓ WIRED — Go test asserts a gated authed 200 carries the value |
| response header | `apiFetch` | `res.headers.get(INSTANCE_GATED_HEADER) === INSTANCE_GATED_VALUE` at `api.ts:146` (before the 401/204/!ok branches) | ✓ WIRED — api.test.ts latch cases across 200/204/non-OK |
| `apiFetch` | `authStore.markGateActive()` | single call `api.ts:147` | ✓ WIRED |
| `markGateActive` | `gateActive` module boolean + `sessionStorage["dt_gate_active"]` | `authStore.ts:175-176` sets flag + `persistGateActive()`; early-returns if already set (`:172`) | ✓ WIRED — one-way, never touches `authed`, never clears |
| `gateActive` | `{gateActive && <LogoutButton />}` | `useGateActive()` → `useSyncExternalStore(subscribe, isGateActive, isGateActive)`; `root.tsx:103` + `:118` | ✓ WIRED — `root.tsx` unchanged; branch repairs itself once the signal reaches the store |
| marker write site | gated path only | registered solely inside `server.go:172-173` `if gate != nil` `r.Group` | ✓ WIRED — ungated instance emits nothing (Go test `ungated_instance_carries_nothing_(D-18)`) |
| header literal | two-language byte-for-byte contract | `gate.go` const ↔ `api.ts` const, each comment naming the other file; pinned independently in both test suites | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `gate.go` | `X-Instance-Gated` response header | one fixed literal `"1"`, set only after `Verify` succeeds | ✓ — no passphrase, cookie value, token, count, timing, or user data; discloses strictly less than the existing 401 | ✓ FLOWING |
| `api.ts` | latch trigger | `res.headers.get(...)` on a real `fetch` response | ✓ — real header off a real gated response | ✓ FLOWING |
| `authStore.ts` | `gateActive` | `markGateActive()` from the latch, or `sessionStorage.getItem("dt_gate_active")` at module load, or either `mark*` | ✓ — one fixed literal under one key is the entire persistence surface | ✓ FLOWING |
| `root.tsx` | `gateActive` (via `useGateActive`) | `authStore.isGateActive()` cached boolean | ✓ | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go marker matrix | `go test ./internal/authgate/ -run InstanceGated -v` | `TestGate_InstanceGatedMarker_PresentOnAuthenticatedSuccess` PASS; `..._AbsentOnUnauthenticatedAndUngated` PASS (gated_401 / gated_exempt_route / ungated_instance_(D-18) all PASS) | ✓ PASS |
| Full backend suite | `go build ./... && go test ./...` | build exit 0; all packages PASS | ✓ PASS |
| Full frontend suite + coverage | `cd web && node_modules/.bin/vitest run` | 12 files / 125 tests passed; coverage 88.18 / 78.36 / 86.33 / 89.34 (all > 70) | ✓ PASS |
| SPA-mode production build (Node prerender safety) | `cd web && node_modules/.bin/react-router build` | exit 0; `build/client/index.html` generated | ✓ PASS |
| WR-01 fix placement | `grep -n 'try {' web/app/lib/authStore.ts` | line 96 (before `typeof` at 97) and line 109 (before `typeof` at 110); exactly 2 | ✓ PASS |
| Diff scope | `git diff --stat 04cc449..HEAD` | production: `gate.go`, `api.ts`, `authStore.ts` only; `root.tsx`, `server.go`, `go.mod`, `go.sum`, `web/package.json`, lockfile, `internal/webassets/` untouched | ✓ PASS |
| Debt markers in the 14-07 surface | grep `TODO/FIXME/XXX/HACK/TBD` in modified files | none | ✓ PASS |
| Real-browser re-run of UAT Test 5 on the built Docker image | — | not runnable here | ? SKIP → human (item 1) |

### Probe Execution

No probes declared for this phase (not a migration/tooling phase). N/A.

### Requirements Coverage

| Requirement | Source Plans | Status | Evidence |
|-------------|--------------|--------|----------|
| GATE-01 | 14-01, 14-04, 14-05 | ✓ SATISFIED | Truth 1; UAT Test 1 PASS. Unchanged by 14-06/14-07. |
| GATE-02 | 14-01 | ✓ SATISFIED | Truth 3 (stateless HMAC cookie survives a new Manager). |
| GATE-03 | 14-01, 14-04 | ✓ SATISFIED | Truth 4 (cookie attributes + constant-time compare). |
| GATE-04 | 14-02 | ✓ SATISFIED | Truth 4 (per-IP throttle, undelayed 429, bounded concurrency); UAT Test 4 PASS. |
| GATE-05 | 14-03 | ✓ SATISFIED | Truth 2; UAT Test 1 PASS. |
| GATE-06 | 14-01, 14-03, 14-06, 14-07 | ✓ SATISFIED | Truth 3 (logout) + Truth 6 (durable across reload, G-14-2) + Truth 7 (present on a clean-cookie first load, G-14-3 — code + automated regression; real-browser re-run → human). |
| GATE-07 | 14-01, 14-04, 14-05 | ✓ SATISFIED | Truth 5; UAT Test 3 PASS. Marker never emitted on the inert path (structural). |

REQUIREMENTS.md maps exactly `GATE-01 … GATE-07` to Phase 14 (line 110) and marks all seven `[x]` Complete (lines 80–86). **No orphaned requirements.** GATE-08 (signing-key rotation) is explicitly out of scope (line 57). Every PLAN's `requirements:` frontmatter (14-01…14-07) resolves to an ID in this set; the three gap-closure plans (14-05 G-14-1, 14-06 G-14-2, 14-07 G-14-3) all carry `requirements: [GATE-0x]` that is already in the phase set.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | none | — | No debt markers, stubs, hollow props, or hardcoded-empty data paths in the 14-07 surface. The one fixed literal `"1"` (server header value and `dt_gate_active` storage value) is the entire new data surface, by design. |

### Residual Risks (not blockers)

| Item | Source | Disposition |
|------|--------|-------------|
| **CR-WR-01 — gated responses carry `X-Instance-Gated` (and the watchlist/events bodies) with no `Cache-Control: no-store` or `Vary: Cookie`** anywhere in the Go code | 14-REVIEW.md (14-07 delta) WR-01 | **WARNING.** Pre-existing risk: a shared/intermediary cache or misconfigured reverse proxy in front of the single binary could store one user's authenticated `GET /watchlist` 200 and replay it. 14-07 compounds it — the recipient would also latch `gateActive` and see a Log out control they never earned. No such cache exists in the current single-binary deployment; the phase goal's server-side 401 enforcement is intact. **Recommend fixing before Phase 17 makes the app publicly reachable** (one middleware on the protected group setting `Cache-Control: no-store`, ideally beside the marker so the two cannot drift). Not a blocker for Phase 14 as-is. |
| CR-WR-02 — `markGateActive` never retries a failed `persistGateActive()` (early-returns once the in-memory flag is set), unlike the two `mark*` siblings which persist unconditionally and self-heal | 14-REVIEW.md WR-02 | Info/minor. Narrow condition (a browser where `getItem` works but `setItem` intermittently throws); cosmetic — the control re-latches on the first API response of the next load. Optional one-line fix or document as a known limitation. |
| CR-IN-01 — the ungated-instance guarantee is coupled to `server.go:173` keeping `pr.Use(gate.Authenticate)` inside the `gate != nil` branch; no type-level enforcement | 14-REVIEW.md IN-01 | Info. Mitigated by `TestGate_InstanceGatedMarker_AbsentOnUnauthenticatedAndUngated/ungated_instance...`; treat that sub-test as a gate for any future middleware-wiring refactor. |
| CR-IN-02 — the latch silently no-ops if the API is ever served cross-origin without `Access-Control-Expose-Headers: X-Instance-Gated` | 14-REVIEW.md IN-02 | Info. Fine today (single binary, same origin, no CORS per CLAUDE.md). Worth a one-line comment if the API is ever split. |
| CR-IN-03 — `POST`/`DELETE /session` and a post-`Authenticate` CSRF 403 are not pinned in the marker negative matrix | 14-REVIEW.md IN-03 | Info. Test-robustness only. |
| WR-01 real-browser sandboxed-`<iframe>` case (prior 14-VERIFICATION human item 2) | 14-07 Task 3 | **Retired.** The `typeof` probe now sits inside the `try` in both helpers and an automated jsdom test drives a throwing `sessionStorage` getter (`authStore.test.ts:192`). The 14-07 SUMMARY notes the sandboxed-iframe *real-browser* variant is technically still human-only, but the fix is verified and the failure mode is faithfully reproduced in jsdom — no longer tracked as an open human item. |
| Pre-existing `tsc --noEmit` failure on a stale `react-router typegen` artifact (`app/root.tsx(19,28): TS2307`) | `deferred-items.md`, 14-06/14-07 SUMMARY | Info. Confirmed pre-existing; `react-router build` runs typegen itself and passes, so CI/Docker unaffected. Logged for a separate quick task. |
| Compose `environment:` can clobber a valid `.env` passphrase with an empty string when compose is run from outside the repo root | prior round | WARNING carried forward. Mitigated by the boot-status log line. Phase 17 runbook. |
| Cookie name lacks `__Host-` prefix (D-09, operator option-a); `DeriveKey` = single unsalted SHA-256; `middleware.RealIP` has no trusted-proxy CIDR allowlist | prior-round code review | Accepted locked decisions (D-01, D-09, D-14); Phase 17 TLS/runbook owns the residuals. |
| `go test -race` unusable on this Windows dev box (ThreadSanitizer allocation failure) | STATE.md | Accepted — CI runs the same suite under `-race`. |

### Human Verification Required

1. **Re-run 14-UAT.md Test 5 in a real browser against `docker compose up --build`.** Do the new `G-14-3 re-run` sub-block FIRST while the browser session is genuinely fresh: from a session that has already unlocked, close the tab (or delete only the `dt_gate_active` Session Storage entry in devtools — do NOT use Log out, do NOT clear cookies), then open the app in a fresh tab. You must be let straight in with no passphrase form AND the Log out control must be present in the nav on that first authenticated view (once the first data request completes). Then the positive check (unlock, full refresh, tab-nav, add-artist, refresh — control persists for the browser session) and the negative check (clear `dt_gate_active` or close the tab, load an ungated instance — control absent). *Why human:* G-14-3 was operator-reported from a real browser on the built Docker image; the automated suite proves the whole chain in jsdom + the Go marker contract, but 14-UAT.md Test 5 still records `result: issue` and reconciles against real-browser behaviour on the built image. Coverage D7 is `human_judgment: true`.

**Already satisfied (no action needed):** UAT Tests 1–4 (operator-reported PASS in round 2/3, unchanged by 14-06/14-07). G-14-1 (gate reachability) and G-14-2 (control durable across reload) closed, the latter with a passing automated regression. Prior 14-VERIFICATION human item 2 (WR-01 storage accessor) retired by the Task 3 fix + jsdom test.

### Gaps Summary

No blocking gaps. All seven GATE requirement IDs are implemented, wired end-to-end, and covered by a green backend suite (`go test ./...`) and a green frontend suite (125/125, coverage > 70 on all axes), plus a passing SPA production build.

**G-14-3 — the Log out control absent on a fresh gated session holding a carried-over valid cookie — is closed in code:** `gate.Authenticate` now stamps `X-Instance-Gated: 1` on every proven-valid-cookie response (and on neither 401 path), `apiFetch` latches that marker into a new one-way `authStore.markGateActive()` before its 401/204/!ok branches, `markGateActive` writes `gateActive` only and never touches `authed` or clears, the marker's only write site is inside `Authenticate` (registered solely in `server.go`'s `gate != nil` branch, so an ungated instance emits nothing structurally), `root.tsx` and `server.go` are unchanged, and an end-to-end regression drives a real marker-carrying 200 through the real `apiFetch` and store into React and asserts the control mounts. The prior 14-VERIFICATION residual WR-01 (`typeof` probe outside the `try`) is also fixed and now has automated jsdom coverage.

Status is `human_needed` (not `passed`) because 14-UAT.md Test 5 — G-14-3's own reconciliation test — still records `result: issue` and must be re-run by a human in a real browser against the built Docker image (coverage D7, `human_judgment: true`). One WARNING-level code-review finding (CR-WR-01: gated responses carry the new marker and the data bodies with no `Cache-Control: no-store`) is a pre-existing caching/replay risk that 14-07 compounds; it does not block the Phase 14 goal (no shared cache in the current deployment, server-side 401 enforcement intact) but is **recommended for closure before Phase 17 makes the instance publicly reachable**.

---

_Verified: 2026-09-01_
_Verifier: Claude (gsd-verifier)_
_Re-verification after G-14-3 closure (plan 14-07)_
