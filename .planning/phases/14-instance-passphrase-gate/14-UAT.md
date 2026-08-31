---
status: partial
phase: 14-instance-passphrase-gate
source: [14-VERIFICATION.md]
started: 2026-08-29T17:38:38Z
updated: 2026-08-31T00:00:00Z
---

## Current Test

[testing paused — 1 blocker issue, 2 tests blocked behind it]

## Tests

### 1. Real-browser cookie behaviour (Chrome + Firefox, http://localhost)
expected: The gate works end-to-end in a real browser; the bare-name `dt_session` cookie (option-a, no `__Host-` prefix) is accepted and replayed by both Chrome and Firefox over plain `http://localhost`. Correct passphrase unlocks and survives a refresh; wrong passphrase shows the fixed inline message; Log out returns to the form.
result: issue
reported: "i set the instance passphrase, ran docker compose up --build and when opening localhost, the watchlist is there, no passphrase form is shown, everything is accesible"
severity: blocker

### 2. PassphraseScreen visual conformance to 14-UI-SPEC
expected: Running SPA matches the approved UI-SPEC pillars — viewport-centred `max-w-sm` `bg-card` card, `gap-6` rhythm, indigo accent reserved to the Unlock fill + input focus ring, destructive colour reserved to error text, dark surface. (Coverage item D8, `human_judgment: true`.)
result: blocked
blocked_by: prior-phase
reason: "Passphrase screen never renders — blocked by the Test 1 gate-bypass blocker (G-14-1)."

### 3. docker compose up with no INSTANCE_PASSPHRASE configured
expected: Stack starts, all seven v1.2 routes answer as before, no passphrase prompt, no new required variable.
result: pass

### 4. Live Discord brute-force alert
expected: With `DISCORD_WEBHOOK_URL` set, drive >20 failed logins within 5 minutes against a running instance. Exactly one brute-force alert embed arrives in the Discord channel, carrying only a count and window (no passphrase, no fragment, no length); no further alert for 15 minutes.
result: blocked
blocked_by: prior-phase
reason: "Cannot exercise the login/brute-force path — blocked by the Test 1 gate-bypass blocker (G-14-1)."

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
  artifacts: []  # Filled by diagnosis
  missing: []    # Filled by diagnosis
