---
status: complete
phase: 14-instance-passphrase-gate
source: [14-VERIFICATION.md, 14-06-SUMMARY.md, 14-07-SUMMARY.md]
started: 2026-08-29T17:38:38Z
updated: 2026-09-01T18:44:08Z
---

## Current Test

[testing complete]

## Tests

### 1. Real-browser cookie behaviour (Chrome + Firefox, http://localhost)
precondition: |
  Set INSTANCE_PASSPHRASE as a KEY=VALUE line in the repo-root .env — the
  gitignored file, NOT .env.example. That is the file docker-compose.yml
  feeds the app container through its `env_file: .env` directive, and it was
  the only channel that worked before plan 14-05.
  - Editing .env.example does nothing. Compose never reads it; it is
    documentation only. This is the mistake that produced G-14-1.
  - A host-shell `export INSTANCE_PASSPHRASE=...` now also works, but only
    because plan 14-05 added `INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-}`
    to the app service `environment:` mapping in docker-compose.yml. Before
    that it silently did nothing.
  - Confirm the gate engaged before starting the browser test: read the boot
    log line added in 14-05 (`logInstanceGateStatus`). It reports the
    instance passphrase gate as active or inert on every start, prints no
    secret, and is the fast check.
  - Value-free container fallback, when the logs are not to hand:
    `docker compose run --rm --entrypoint sh app -c 'if [ -n "$INSTANCE_PASSPHRASE" ]; then echo GATE_ENV=SET; else echo GATE_ENV=EMPTY; fi'`
    — it prints only GATE_ENV=SET or GATE_ENV=EMPTY, never the passphrase.
  - Do NOT verify with `docker compose config`: it inlines env_file contents
    and would print the Discord webhook URL and the passphrase to the
    terminal.
expected: The gate works end-to-end in a real browser; the bare-name `dt_session` cookie (option-a, no `__Host-` prefix) is accepted and replayed by both Chrome and Firefox over plain `http://localhost`. Correct passphrase unlocks and survives a refresh; wrong passphrase shows the fixed inline message; Log out returns to the form.
result: pass
prior_result: issue
note: "Re-run after G-14-1 close (plan 14-05). Operator: 'Test 1 passes.' Gate engages, unlock works, survives refresh, wrong-passphrase message correct, Log out returns to form. NOTE: operator also hit a Log out-button disappearance during this session — split out as Test 5 / gap G-14-2 (Test 1's core cookie/gate behaviour still passes)."

### 2. PassphraseScreen visual conformance to 14-UI-SPEC
expected: Running SPA matches the approved UI-SPEC pillars — viewport-centred `max-w-sm` `bg-card` card, `gap-6` rhythm, indigo accent reserved to the Unlock fill + input focus ring, destructive colour reserved to error text, dark surface. (Coverage item D8, `human_judgment: true`.)
result: pass
prior_result: blocked
note: "Was blocked behind G-14-1; now unblocked. Operator: 'Test 2 passes.'"

### 3. docker compose up with no INSTANCE_PASSPHRASE configured
expected: Stack starts, all seven v1.2 routes answer as before, no passphrase prompt, no new required variable.
result: pass

### 4. Live Discord brute-force alert
expected: With `DISCORD_WEBHOOK_URL` set, drive >20 failed logins within 5 minutes against a running instance. Exactly one brute-force alert embed arrives in the Discord channel, carrying only a count and window (no passphrase, no fragment, no length); no further alert for 15 minutes.
result: pass
prior_result: blocked
note: "Was blocked behind G-14-1; now unblocked. Operator: 'test 4 pass.'"

### 5. Log out control persists across a page reload while logged in
precondition: |
  Test 5 needs a GATED instance, so Test 1's precondition applies first:
  INSTANCE_PASSPHRASE must be a KEY=VALUE line in the repo-root .env (the
  gitignored file compose feeds the container, NOT .env.example, NOT a host
  shell export unless it is also mapped in docker-compose.yml). Confirm the
  gate engaged before starting: the 14-05 boot log line
  (`logInstanceGateStatus`) reports the gate ACTIVE on start and prints no
  secret — that is the fast check.

  G-14-3 re-run (do this FIRST, while the browser session is genuinely fresh):
  - This reproduces the reported failure: a browser session that ALREADY holds
    a valid `dt_session` cookie from earlier testing and has NOT had the
    passphrase typed into it during this session. Typing the passphrase here
    instead would set the gate signal through the login path and mask the
    defect — the re-run would pass without ever exercising G-14-3.
  - Get to that state deterministically from a session that has already
    unlocked: either close the tab entirely and open a new one, OR in devtools
    (Application -> Storage -> Session Storage) delete the `dt_gate_active`
    entry for the origin. Do NOT use the Log out control and do NOT clear
    cookies — the `dt_session` cookie must survive while the per-session
    `dt_gate_active` signal does not.
  - Open the app in that fresh tab. Expected: you are let straight in with no
    passphrase form, AND the **Log out** control is present in the nav on that
    first authenticated view — with no passphrase typed and no reload. The
    control appears once the first data request of that page load completes,
    so a glance during the very first frame, before the watchlist has loaded,
    is not a failure.
  - If the control is absent here, that is the exact G-14-3 symptom. Access to
    the watchlist is unaffected either way — the server 401 is the sole
    enforcement.

  Positive check (the G-14-2 fix / the login path):
  - Unlock with the correct passphrase. Confirm the **Log out** control is in
    the nav.
  - Do a FULL document reload (browser refresh) while the dt_session cookie is
    still valid. The control must STILL be there — without logging in again
    and without any 401 firing.
  - Then navigate between the Watchlist and History tabs, add an artist, and
    reload once more. The control must persist across all of it, for as long
    as the browser session lasts.

  Negative check (guards D-18's ungated-instance rule):
  - The signal is per-tab and per-browser-session by design. To re-test the
    ungated case you MUST first either close the tab entirely, or clear the
    `dt_gate_active` sessionStorage entry for the origin in devtools
    (Application -> Storage -> Session Storage), before loading an instance
    that has NO passphrase configured.
  - Skip that step and a leftover `dt_gate_active` entry from the positive
    check will make an ungated instance appear to render a Log out control.
    That looks like a regression and is not one.

  This test is presentation-only. At no point does access to the watchlist
  depend on the Log out control being visible — the server 401 is the sole
  enforcement.
expected: After unlocking, the **Log out** control stays visible in the nav across a browser refresh / navigation (for as long as the browser session lasts), not only until the next 401. The user remains logged in and the control remains available to end the session.
result: pass
prior_result: issue
prior_reported: "fail, no log out button present"
prior_severity: major
prior_gap: G-14-3
note: "Round 4 re-run (post plan 14-07) — operator: 'pass'. G-14-3 re-run sub-block (carried cookie, no typed login, no 401), positive check (unlock + refresh + tab-nav + add-artist), and ungated negative check all confirmed in a real browser against `docker compose up --build`."
reconciled: "G-14-2 resolved by plan 14-06 (sessionStorage-backed gateActive), but round-3 operator re-run still showed NO Log out control on a gated instance — treated as a fresh regression G-14-3 per #1921, closed by plan 14-07."
root_cause: |
  `web/app/lib/authStore.ts` holds `gateActive` as a volatile module-level
  boolean initialised to `false`. `root.tsx` renders `<LogoutButton />` only
  when `gateActive === true` (D-18). `gateActive` is set `true` only by an
  in-session 401 or a login (`markAuthenticated` / `markUnauthenticated`), and
  is NOT persisted. On any full document load (refresh, or a navigation that
  reloads the bundle) while the `dt_session` cookie is still valid, the module
  re-initialises: `authed` is optimistically `true` and the route's loader
  fetch returns 200 (valid cookie) so no 401 fires — leaving `gateActive`
  `false`. Result: the user is authenticated and has full access, but the
  Log out control is gone until a 401 happens (e.g. cookie expiry) or they
  log in again. D-18's own wording ("...or completes a login in this browser
  session") implies browser-session persistence, which points at
  `sessionStorage`-backed `gateActive` rather than volatile module state.
  The "when I added a new artist" trigger is incidental — the add-artist flow
  (`handleAddSearchResult` → `addWatchlist` + `refresh`) is pure client-side
  fetch with no reload; the reload that dropped `gateActive` was the Test 1
  refresh step just before.
  Not an access-control issue — server 401 remains the sole enforcement and
  is unaffected.

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-14-1
  truth: "With INSTANCE_PASSPHRASE set, opening the app shows the passphrase form and blocks all access until unlocked"
  status: resolved
  resolved_by: [14-05]
  resolution: "Plan 14-05 forwarded INSTANCE_PASSPHRASE/TRUST_PROXY_HEADERS through docker-compose.yml (with regression test), added the secret-free boot-status log line, and documented the .env channel in Test 1. Operator reconciled the live repo-root .env; boot log reports the gate ACTIVE and Tests 1-4 all pass on re-run."
  reason: "User reported: i set the instance passphrase, ran docker compose up --build and when opening localhost, the watchlist is there, no passphrase form is shown, everything is accesible"
  severity: blocker
  test: 1
  root_cause: "Configuration gap, not a code defect. The `app` container starts with an empty INSTANCE_PASSPHRASE, so httpserver.New builds gate == nil and registers data routes flat with no auth middleware — the intended GATE-07 inert path. The container sees empty because docker-compose.yml's `app` service injects env only via `env_file: .env`, and the repo-root `.env` (gitignored, last synced ~Phase 11) has no INSTANCE_PASSPHRASE line. Phase 14 updated `.env.example` (INSTANCE_PASSPHRASE=caliber) but nothing reconciled the live `.env`, and agent tooling is sandbox-blocked from editing `.env*`. Compose also never interpolates ${INSTANCE_PASSPHRASE} nor passes a host-shell value through, so `export INSTANCE_PASSPHRASE=...` or editing `.env.example` silently does nothing — the only working channel is a KEY=VALUE line in `.env`."
  artifacts:
    - path: ".env"
      issue: "Missing INSTANCE_PASSPHRASE line (also missing TRUST_PROXY_HEADERS, NOTIFY_MAX_RELEASE_AGE_DAYS). This is the file compose feeds the container. OPERATOR ACTION — sandbox cannot edit .env*."
    - path: "docker-compose.yml"
      issue: "app service injects env only through `env_file: .env`; no `${INSTANCE_PASSPHRASE:-}` interpolation in `environment:` and no host-shell fallback, so a shell export is silently ignored."
    - path: ".env.example"
      issue: "Ships INSTANCE_PASSPHRASE=caliber as if wired, inviting the 'edit the example / assume .env inherits it' mistake."
    - path: "cmd/server/main.go"
      issue: "No boot log line stating whether the instance gate is ACTIVE or INERT — the inert path is silent."
    - path: "internal/httpserver/server.go"
      issue: "Correct as-is (GATE-07 contract) — server.go:113-117 empty passphrase -> gate == nil -> flat routes."
  missing:
    - "Operator adds INSTANCE_PASSPHRASE=<random 24+ chars> to the repo-root .env (plus TRUST_PROXY_HEADERS=false, NOTIFY_MAX_RELEASE_AGE_DAYS=7), then re-runs `docker compose up --build`. Verify: `docker compose run --rm app env | grep INSTANCE_PASSPHRASE`. This alone resolves G-14-1."
    - "Hardening: add `INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-}` to app.environment: in docker-compose.yml so a host-shell value also works."
    - "Hardening: emit one boot Info line in cmd/server/main.go stating instance gate ACTIVE vs INERT."
    - "Hardening: add an explicit 'set it in .env, not .env.example, not your shell' precondition to 14-UAT.md Test 1."
  debug_session: .planning/debug/passphrase-gate-bypassed.md

- gap_id: G-14-2
  truth: "The Log out control stays available for the whole browser session once the gate is active, not only until the next 401"
  status: resolved
  resolved_by: 14-06
  resolved_at: 2026-09-01
  resolution: "Plan 14-06 seeded gateActive from sessionStorage (key dt_gate_active, value \"1\") at module load and write-through on both mark* functions (D-18), guarded for the Node prerender and a hostile browser store; authed stays volatile (D-16), root.tsx unchanged. Frontend regression suite (authStore.test.ts +7, root.test.tsx +1) + react-router build all green; 14-VERIFICATION.md score 8/8. Coverage D7 (operator real-browser re-run of Test 5) is the remaining human check — Test 5 re-opened as [pending]."
  reason: "Operator reported the Log out button disappeared after a page reload while still logged in with full access (noticed after adding an artist)."
  severity: warning
  test: 5
  requirement: GATE-06
  root_cause: "web/app/lib/authStore.ts `gateActive` is a volatile module-level boolean (initial false); it is set true only by an in-session 401 or login and is never persisted. root.tsx gates `<LogoutButton />` on `gateActive`. After any full document load while the dt_session cookie is still valid, the module re-inits, the loader fetch returns 200, no 401 fires, and `gateActive` stays false — the user is logged in with full access but the Log out control is missing until a 401 or a fresh login. Server-side 401 enforcement is unaffected; this is presentation-only."
  artifacts:
    - path: "web/app/lib/authStore.ts"
      issue: "`gateActive` should persist for the browser session (sessionStorage) per D-18's 'in this browser session' wording; currently volatile module state. Initialise from sessionStorage, write on each mark*, wrap in try/catch for private-mode."
    - path: "web/app/root.tsx"
      issue: "Renders LogoutButton only when gateActive — correct once gateActive is durable; no change needed beyond confirming behaviour with the persisted signal."
    - path: "web/app/lib/authStore.test.ts"
      issue: "Add coverage: gateActive survives a simulated module reload when sessionStorage carries it; still starts false with empty/throwing storage."
  missing:
    - "Back `gateActive` with sessionStorage so the Log out control survives a reload for the browser session."
    - "Regression test: Log out control still present after a reload while authed (root.test.tsx or an authStore reload test)."

- gap_id: G-14-3
  truth: "On a gated instance, a browser session that already holds a valid dt_session cookie renders the Log out control on its first authenticated load — with no 401, no typed login, and no reload — and it stays visible across refresh / tab navigation / add-artist for the browser session"
  status: resolved
  resolved_by: [14-07]
  resolved_at: 2026-09-01
  uat_confirmed: "Test 5 round-4 operator re-run PASS (2026-09-01) — carried-cookie fresh session shows the Log out control on first authed view, survives refresh/tab-nav/add-artist, absent on ungated instance."
  resolution: "Plan 14-07: gate.Authenticate stamps X-Instance-Gated: 1 on every gate-passing response (gate.go:200, neither 401 path); apiFetch latches it (api.ts:146-148, before the 401/204/!ok branches) into a new authStore.markGateActive() one-shot that writes gateActive only — never authed, never clears. Marker's only write site is inside Authenticate, registered solely in server.go's gate != nil branch, so an ungated instance emits nothing structurally (D-18). Also retired the prior WR-01 residual (typeof sessionStorage probe moved inside the try in both storage helpers). Go marker matrix + jsdom end-to-end + 125/125 frontend + react-router build all green; 14-VERIFICATION.md re-verification 7/7 must-haves in code. Remaining: operator real-browser re-run of Test 5."
  reason: "User reported (round-3 re-run of Test 5, after plan 14-06's sessionStorage fix): 'fail, no log out button present'. Clarified: ran `docker compose up --build`, the dt_session login 'was already set' (valid cookie carried over from a prior test — user did NOT type the passphrase this session), and there was NO Log out button from the first authed render onward."
  severity: major
  test: 5
  requirement: GATE-06
  supersedes_context: "G-14-2 (plan 14-06) was expected to close this via sessionStorage-backed gateActive; the operator re-run shows the control still absent, so this is a fresh regression, not a G-14-2 re-open (#1921). 14-06's diagnosis mischaracterised the defect — see root_cause."
  root_cause: |
    The SPA has no way to discover an instance is gated except by receiving a 401 or completing a typed passphrase login. authStore.gateActive — the sole condition on rendering the Log out control (root.tsx:118, D-18) — is written true only by markAuthenticated() (PassphraseScreen.tsx:62, typed login) or markUnauthenticated() (api.ts:134 on a 401, or root.tsx:74 on logout). On the first load of a browser session that already carries a valid dt_session cookie, none occur: authed is optimistically true (D-16, no boot GET /session), every gated fetch returns 200, and gate.Authenticate attaches NO gating marker to a successful authed response (gated 2xx responses are shape-identical to an ungated instance; there is no GET /session route). So gateActive stays false all session and the Log out control never mounts. Plan 14-06 (seed gateActive from sessionStorage) only helps AFTER a 401/login has set the flag once in the session. Presentation-only — server 401 enforcement (GATE-01) is intact, access unaffected.
  artifacts:
    - path: "internal/authgate/gate.go"
      issue: "gate.Authenticate sets nothing on the response success path — a valid-cookie 2xx is indistinguishable from an ungated instance's response, so the client can never learn the instance is gated without a 401."
    - path: "web/app/lib/authStore.ts"
      issue: "gateActive is written only by markAuthenticated / markUnauthenticated (login or 401). No 'gated but authed via existing cookie' path. Needs a markGateActive() that sets+persists+notifies gateActive without touching authed."
    - path: "web/app/lib/api.ts"
      issue: "apiFetch inspects responses only for status 401 / 204 / !ok. It should also latch a server gating signal (e.g. X-Instance-Gated: 1) on any response and call authStore.markGateActive()."
    - path: "web/app/root.tsx"
      issue: "{gateActive && <LogoutButton />} is correct once gateActive is reliably set on a gated authed load — no change needed beyond confirming behaviour."
    - path: "web/app/lib/authStore.test.ts / web/app/lib/api.test.ts / web/app/root.test.tsx"
      issue: "No coverage for the clean-authed-load case: a session that only ever sees 200s must still end up with the Log out control once the gating signal is present."
  missing:
    - "Server: gate.Authenticate emits a gating marker on the success path (recommended: X-Instance-Gated: 1 response header) so every gated 2xx is self-identifying — no new endpoint, no extra round-trip."
    - "Client: apiFetch latches that marker → new authStore.markGateActive() (gateActive=true, persist, notify; authed untouched)."
    - "Regression: authStore markGateActive unit; api.ts latches header on a 200; root.test.tsx Log out control present after a clean authed load with no 401 and no login."
    - "Human re-run of 14-UAT.md Test 5 against docker compose up --build (fresh session + valid cookie) + the ungated negative check."
  debug_session: .planning/debug/logout-control-absent-on-fresh-gated-session.md
