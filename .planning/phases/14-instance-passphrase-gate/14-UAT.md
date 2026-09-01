---
status: testing
phase: 14-instance-passphrase-gate
source: [14-VERIFICATION.md]
started: 2026-08-29T17:38:38Z
updated: 2026-08-31T00:00:00Z
---

## Current Test

[UAT complete for this round — Tests 1-4 PASS, Test 5 recorded as issue G-14-2]

note: |
  Round 2 (post plan 14-05):
  - Test 1 PASS, Test 2 PASS, Test 3 PASS (unchanged), Test 4 PASS — all
    operator-reported.
  - G-14-1 (gate never engaged) is CLOSED: plan 14-05 wired docker-compose,
    added the boot-status log, and the operator reconciled the live .env;
    every gate-behaviour test now passes.
  - Test 5 / gap G-14-2 (open): the Log out control disappears after a page
    reload while still logged in. Presentation-only (server 401 enforcement
    unaffected), severity warning. Needs a gap plan.
  Next: /gsd-plan-phase 14 --gaps  (G-14-2 only; G-14-1 already resolved).

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

  Positive check (the G-14-2 fix):
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
result: issue
reported: "log out button disappeared when i added a new artist. [access to the watchlist was unaffected — still fully usable]"
severity: warning
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
passed: 4
issues: 1
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
  status: failed
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
