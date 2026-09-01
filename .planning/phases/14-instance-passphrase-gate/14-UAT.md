---
status: diagnosed
phase: 14-instance-passphrase-gate
source: [14-VERIFICATION.md]
started: 2026-08-29T17:38:38Z
updated: 2026-08-31T00:00:00Z
---

## Current Test

[testing paused — 1 blocker issue, 2 tests blocked behind it]

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
result: issue
reported: "i set the instance passphrase, ran docker compose up --build and when opening localhost, the watchlist is there, no passphrase form is shown, everything is accesible"
severity: blocker

### 2. PassphraseScreen visual conformance to 14-UI-SPEC
expected: Running SPA matches the approved UI-SPEC pillars — viewport-centred `max-w-sm` `bg-card` card, `gap-6` rhythm, indigo accent reserved to the Unlock fill + input focus ring, destructive colour reserved to error text, dark surface. (Coverage item D8, `human_judgment: true`.)
result: blocked
blocked_by: prior-phase
reason: "Passphrase screen never renders — blocked by the Test 1 gate-bypass blocker (G-14-1)."
note: "Unblocks once Test 1's precondition is satisfied and Test 1 passes."

### 3. docker compose up with no INSTANCE_PASSPHRASE configured
expected: Stack starts, all seven v1.2 routes answer as before, no passphrase prompt, no new required variable.
result: pass

### 4. Live Discord brute-force alert
expected: With `DISCORD_WEBHOOK_URL` set, drive >20 failed logins within 5 minutes against a running instance. Exactly one brute-force alert embed arrives in the Discord channel, carrying only a count and window (no passphrase, no fragment, no length); no further alert for 15 minutes.
result: blocked
blocked_by: prior-phase
reason: "Cannot exercise the login/brute-force path — blocked by the Test 1 gate-bypass blocker (G-14-1)."
note: "Unblocks once Test 1's precondition is satisfied and Test 1 passes."

## Summary

total: 4
passed: 1
issues: 1
pending: 0
skipped: 0
blocked: 2

## Gaps

- gap_id: G-14-1
  truth: "With INSTANCE_PASSPHRASE set, opening the app shows the passphrase form and blocks all access until unlocked"
  status: failed
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
