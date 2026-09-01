---
status: awaiting_human_verify
trigger: "fail, no log out button present"
created: 2026-09-01T00:00:00Z
updated: 2026-09-01T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED — the SPA has no signal that an instance is gated other than a 401 or a completed login; a fresh browser session carrying an already-valid `dt_session` cookie triggers neither, so `gateActive` stays `false` all session and `{gateActive && <LogoutButton />}` never renders.
test: Static trace of every `gateActive` / `authStore.mark*` call site (client) + every gated response path (server).
expecting: If confirmed, no client or server code path sets `gateActive=true` on a clean authed load.
next_action: Hand root cause to `/gsd-plan-phase 14 --gaps` (gap G-14-3). Recommended fix direction below.
bug_class: bohrbug
reasoning_checkpoint: null
tdd_checkpoint: null

## Symptoms

expected: On a gated instance (`INSTANCE_PASSPHRASE` set), an authenticated browser always shows the **Log out** control in the nav (`root.tsx:118` — `{gateActive && <LogoutButton />}`), for the whole browser session.
actual: No Log out control renders at all. The user ran `docker compose up --build`, the `dt_session` cookie "was already set" (valid, carried over from a prior UAT test — the user did NOT type the passphrase this session), and there was no Log out button from the first authenticated render onward. Watchlist access is completely unaffected.
errors: None (no console errors, no white-screen).
reproduction: 14-UAT.md Test 5. `INSTANCE_PASSPHRASE` set; `docker compose up --build`; open the app in a browser session that (a) already holds a valid `dt_session` cookie and (b) has no `dt_gate_active` entry in `sessionStorage` (fresh session, or cleared during earlier negative-check testing). You are let straight in; no Log out control appears.
started: Present since plan 14-03 (the auth-store design). Plan 14-06 (sessionStorage persistence of `gateActive`) narrowed but did not close it — it only helps once a 401 or login has set the flag once in the session.

## Eliminated

- hypothesis: Stale embedded SPA bundle (`internal/webassets/build/client` last rebuilt at f6a0f31, pre-Phase-14).
  evidence: The Dockerfile builds `web/` from source in stage 1 (`node:26` → `pnpm run build` → `COPY --from=web-build`), and `.dockerignore` excludes the committed tree. `docker compose up --build` ships current source. Tests 1–2 (real-browser PassphraseScreen) passed on the same build path.
  timestamp: 2026-09-01

- hypothesis: `gateActive` lost specifically on reload (the G-14-2 framing).
  evidence: User clarified the control was absent from the first authenticated render, before any reload. The reload path is now sessionStorage-backed (14-06) and its jsdom regression passes; the failure is on initial load, not reload.
  timestamp: 2026-09-01

- hypothesis: CSS / visibility (`ml-auto` on the button, dark theme).
  evidence: `{gateActive && <LogoutButton />}` — when `gateActive` is false the element is not in the DOM at all, not merely hidden. Root cause is the boolean, not layout.
  timestamp: 2026-09-01

## Evidence

- timestamp: 2026-09-01
  checked: `web/app/lib/authStore.ts` — every writer of `gateActive`.
  found: `gateActive` is set `true` in exactly two places: `markAuthenticated()` and `markUnauthenticated()`. Module-load seed is `readPersistedGateActive()` → `sessionStorage["dt_gate_active"] === "1"`. No other writer. No boot-time probe (`authed` is optimistic `true`, D-16, no startup `GET /session`).
  implication: `gateActive` can only become true reactively — via a login or a 401.

- timestamp: 2026-09-01
  checked: `web/app/components/auth/PassphraseScreen.tsx` and `web/app/lib/api.ts` — every `mark*` call site.
  found: `markAuthenticated()` ← only `PassphraseScreen.tsx:62`, after a successful `createSession()` (i.e. the user typed the passphrase). `markUnauthenticated()` ← `api.ts:134` (any 401) and `root.tsx:74` (Log out click).
  implication: With a valid cookie and no typed login, none of the three fire. `createSession` is never called; no fetch returns 401; no logout.

- timestamp: 2026-09-01
  checked: `internal/httpserver/server.go:164-185` — gated route registration; `internal/authgate/gate.go` / `login.go` / `session.go` — response headers.
  found: When the gate is enabled, gated data routes sit behind `r.Group{ gate.Authenticate; gate.RequireCSRFHeader }`. A request with a valid `dt_session` cookie gets a normal 2xx with **no distinguishing header, cookie, or body field** — byte-identical in shape to an ungated instance. There is no `GET /session` route (only `POST` and `DELETE`). `gate.Authenticate` sets nothing on the response on the success path.
  implication: The server never tells the client "this instance is gated" on a successful authed response. The 401 is the only gating signal, and a valid cookie suppresses it.

- timestamp: 2026-09-01
  checked: `web/app/lib/authStore.ts:104` and 14-06-SUMMARY.md D-16 notes.
  found: `authed` starts optimistically `true` with no verification. On a fresh load the routed page renders immediately; its loader fetch returns 200 (valid cookie); the 401 interceptor is never hit.
  implication: The optimistic-authed design (correct for its own purpose) is exactly what denies the SPA any occasion to observe the gate on a clean load.

## Resolution

root_cause: |
  The SPA has no way to discover that an instance is gated except by receiving an HTTP 401 or by the user completing a passphrase login. `authStore.gateActive` — the sole condition on rendering the **Log out** control (`root.tsx:118`, D-18) — is written `true` only by `markAuthenticated()` (typed login) or `markUnauthenticated()` (a 401 / an explicit logout). On the first load of a browser session that already carries a valid `dt_session` cookie, none of these occur: `authed` is optimistically `true` (D-16, no boot-time `GET /session`), every gated fetch returns 200, and the server attaches no gating marker to a successful authed response (gated 2xx responses are shape-identical to an ungated instance; there is no `GET /session` route). So `gateActive` stays `false` for the entire session and the Log out control never mounts.

  Plan 14-06 (seed `gateActive` from `sessionStorage["dt_gate_active"]`) only helps *after* a 401 or login has set and persisted the flag once within the session — it does nothing for a session that never sees either. The original G-14-2 diagnosis mischaracterised the defect as "volatile module state lost on reload"; the deeper gap, present since 14-03, is the absence of any gated-load signal. This is presentation-only — server-side 401 enforcement (GATE-01) is fully intact and access is never affected.

fix: (not applied — diagnosis only; `/gsd-plan-phase 14 --gaps` will plan it)

  Recommended direction — server emits a gating signal on every response that passes `gate.Authenticate`, client latches it:
  1. `internal/authgate/gate.go` — in `Authenticate`, on the success path set a response header, e.g. `X-Instance-Gated: 1`, before calling `next.ServeHTTP` (wrap the `ResponseWriter` or set the header before the handler writes). This makes every gated 2xx self-identifying with no new endpoint or round-trip.
  2. `web/app/lib/api.ts` — in `apiFetch`, after `fetch` resolves, if `res.headers.get("X-Instance-Gated") === "1"` call a new `authStore.markGateActive()`.
  3. `web/app/lib/authStore.ts` — add `markGateActive()`: sets `gateActive = true`, `persistGateActive()`, `notify()` — WITHOUT touching `authed` (keep the `authed` / `gateActive` separation; `markGateActive` is the "gated but not necessarily via a 401" path).
  4. Regression tests: authStore unit (`markGateActive` sets + persists + notifies, leaves `authed` untouched); api.ts (a 200 carrying `X-Instance-Gated: 1` latches `gateActive`); root.test.tsx (Log out control present after a clean authed load that only saw 200s carrying the header).

  Alternatives considered: (a) a real public `GET /session` returning `{gated: bool}` probed once on boot — more explicit but adds a route + a request + boot-time async state; (b) a non-HttpOnly `dt_gated=1` companion cookie the SPA reads via `document.cookie` — survives reload natively but adds a second cookie and another thing to clear on logout. Option 1 fits the existing single-`apiFetch`-chokepoint + custom-header (`X-Requested-With`) conventions best.

verification: |
  Self-verification is static only (Windows dev box cannot run the Docker/browser E2E). Human must re-run 14-UAT.md Test 5 against `docker compose up --build` after the fix lands:
  - fresh browser session + already-valid `dt_session` cookie + gate ACTIVE → Log out control present on first load, persists across refresh / tab-nav / add-artist for the session;
  - negative: clear `dt_gate_active` sessionStorage (or close the tab), load an UNGATED instance → no Log out control, and no `X-Instance-Gated` header on its responses.

oracle_type: specified
files_changed: []
