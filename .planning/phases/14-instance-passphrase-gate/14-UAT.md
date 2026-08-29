---
status: testing
phase: 14-instance-passphrase-gate
source: [14-VERIFICATION.md]
started: 2026-08-29T17:38:38Z
updated: 2026-08-29T17:38:38Z
---

## Current Test

number: 1
name: Real-browser cookie behaviour (Chrome + Firefox, http://localhost)
expected: |
  Set INSTANCE_PASSPHRASE, run the server, open http://localhost:8080 in Chrome AND Firefox.
  The passphrase form renders; the correct passphrase unlocks and stays unlocked across a page
  refresh; a wrong passphrase shows the fixed inline message; Log out returns to the form.
  The bare-name dt_session cookie (option-a, no __Host- prefix) is accepted and replayed by
  both browsers over plain http://localhost.
awaiting: user response

## Tests

### 1. Real-browser cookie behaviour (Chrome + Firefox, http://localhost)
expected: The gate works end-to-end in a real browser; the bare-name `dt_session` cookie (option-a, no `__Host-` prefix) is accepted and replayed by both Chrome and Firefox over plain `http://localhost`. Correct passphrase unlocks and survives a refresh; wrong passphrase shows the fixed inline message; Log out returns to the form.
result: [pending]

### 2. PassphraseScreen visual conformance to 14-UI-SPEC
expected: Running SPA matches the approved UI-SPEC pillars — viewport-centred `max-w-sm` `bg-card` card, `gap-6` rhythm, indigo accent reserved to the Unlock fill + input focus ring, destructive colour reserved to error text, dark surface. (Coverage item D8, `human_judgment: true`.)
result: [pending]

### 3. docker compose up with no INSTANCE_PASSPHRASE configured
expected: Stack starts, all seven v1.2 routes answer as before, no passphrase prompt, no new required variable.
result: [pending]

### 4. Live Discord brute-force alert
expected: With `DISCORD_WEBHOOK_URL` set, drive >20 failed logins within 5 minutes against a running instance. Exactly one brute-force alert embed arrives in the Discord channel, carrying only a count and window (no passphrase, no fragment, no length); no further alert for 15 minutes.
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
