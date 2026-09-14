---
status: complete
phase: 20-digest-settings-operator-control
source: [20-01-SUMMARY.md, 20-02-SUMMARY.md, 20-03-SUMMARY.md, 20-04-SUMMARY.md]
started: 2026-09-13T23:20:06Z
updated: 2026-09-14T00:07:57Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server/service. Clear ephemeral state (temp DBs, caches, lock files). Start the application from scratch. Server boots without errors, the migration to schema version 8 (notification_settings) completes, and a primary query (health check, homepage load, or basic API call) returns live data.
result: pass

### 2. Confirm Auto-Covered Deliverables
expected: |
  All 28 tracked deliverables across plans 20-01 through 20-04 are covered by passing automated tests (integration tests against real Postgres for the backend, unit tests for the SPA card, plus two "other" checks: a git-diff proving real-time notification code is untouched, and the full Definition-of-Done run for the rebuilt embedded bundle). None require human judgment. Summary by plan:

  **20-01 — Settings store & gated routes (7/7 auto-passed):**
  - D1 fresh-migrated DB seeds exactly one settings row (digest off, cadence daily, watermark NULL) — TestService_FreshSchemaSeedsOneRowWithDefaults
  - D2 GET/PUT round-trips through Postgres inside the gated route group — TestSettings_RoundTrip
  - D3 replaying an identical PUT is a no-op — TestService_UpdateIsIdempotent
  - D4 a second settings row is rejected by the CHECK constraint — TestService_SecondRowRejectedByCheckConstraint
  - D5 an unrecognised cadence is rejected before touching Postgres — TestService_UpdateRejectsUnknownCadence
  - D6 digest_last_sent_at stays NULL across updates — TestService_LastSentWatermarkUntouched
  - D7 notifier/poller/detection byte-for-byte unchanged — git diff (empty)

  **20-02 — HTTP hardening (8/8 auto-passed):**
  - D8 malformed/unknown/oversize-adjacent PUT bodies rejected 400, row unchanged — TestSettings_PutRejects*
  - D9 oversize PUT body rejected 4xx, never persists — TestSettings_PutRejectsOversizeBody
  - D10 defaults survive every rejected write — assertSettingsDefaults
  - D11 no session cookie answers 401 (not 403) on both verbs — TestSettings_GetGated401NoCookie / PutGated401NoCookie
  - D12 missing X-Requested-With refused 403 with zero store calls; present succeeds 200 once — TestSettings_PutGatedForbiddenWithoutCSRFHeader / PutGatedSucceedsWithCookieAndHeader
  - D13 ungated server never 401s; unconfigured server answers 503 — TestSettings_UngatedNeverAnswers401 / NotConfiguredAnswers503
  - D14 a store error leaking a DSN password/webhook token never reaches the response body — TestSettings_NoLeak
  - D15 notifier/poller/detection byte-for-byte unchanged — git diff (empty)

  **20-03 — SPA digest mode toggle (6/6 auto-passed):**
  - D1 flipping the switch persists and confirms inline ("Saved.") — DigestSettings.test.tsx
  - D2 a failed save keeps the last-known-good value and shows the failure copy — DigestSettings.test.tsx
  - D3 two interactions in one task start exactly one request; control disabled in flight on both resolve/reject — DigestSettings.test.tsx
  - D4 digest settings ride the same mount/Refresh fetch; a digest-only failure hits the existing page-level error state — system.test.tsx
  - D5 NotificationSettings wire type matches settings.go field-for-field — api.test.ts + manual read-back
  - D6 updateDigestSettings() sends full-object PUT with CSRF header, propagates ApiError on 401/400 — api.test.ts

  **20-04 — Cadence control & last-sent row (7/7 auto-passed):**
  - D1 picking Daily/Weekly saves with no submit step — DigestSettings.test.tsx
  - D2 cadence control stays visible/focusable/populated (dimmed, not hidden) when digest mode is off — DigestSettings.test.tsx
  - D3 toggling digest mode never changes displayed cadence; cadence change always sends both fields — DigestSettings.test.tsx
  - D4 a failed cadence save keeps the previous cadence and shows the shared failure copy — DigestSettings.test.tsx
  - D5 a null watermark reads the literal "Never sent yet" — DigestSettings.test.tsx
  - D6 a populated watermark renders <time dateTime/title> via the shared absolute-time formatter — DigestSettings.test.tsx
  - D7 the embedded SPA bundle is rebuilt from this phase's source and the full Definition of Done is green — git status + go vet + golangci-lint + test suite + coverage-gate + sqlc-check + prettier + vitest

  Reply "yes" to accept this automated coverage as-is, or describe anything you want to spot-check by hand instead.
result: pass

### 3. [D1] Fresh-migrated DB seeds exactly one settings row with defaults
expected: A fresh migrated database carries exactly one notification_settings row (digest off, cadence daily, watermark NULL)
result: pass
source: automated
coverage_id: 20-01-D1

### 4. [D2] GET/PUT round-trip through Postgres inside the gated route group
expected: GET/PUT /settings/notifications round-trip through Postgres, sitting inside the gated route group
result: pass
source: automated
coverage_id: 20-01-D2

### 5. [D3] Replaying an identical PUT is a no-op
expected: Replaying an identical PUT is a no-op on observable state (idempotent singleton update)
result: pass
source: automated
coverage_id: 20-01-D3

### 6. [D4] A second settings row is rejected by the CHECK constraint
expected: A second settings row (id=2) is rejected by the CHECK constraint; row count stays 1
result: pass
source: automated
coverage_id: 20-01-D4

### 7. [D5] An unrecognised cadence is rejected before touching Postgres
expected: An unrecognised digest_cadence is rejected before touching Postgres and the stored row is unchanged
result: pass
source: automated
coverage_id: 20-01-D5

### 8. [D6] digest_last_sent_at stays NULL across updates
expected: digest_last_sent_at is never written by this phase — stays NULL across multiple updates
result: pass
source: automated
coverage_id: 20-01-D6

### 9. [D7] Real-time notification behavior is byte-for-byte unchanged (20-01)
expected: Real-time notification behavior (internal/notifier, internal/poller, internal/detection) is byte-for-byte unchanged
result: pass
source: automated
coverage_id: 20-01-D7

### 10. [D8] Malformed/unknown/wrong-typed PUT bodies rejected 400
expected: A PUT carrying an unrecognised, omitted, or wrong-typed cadence -- or an extra unknown field, or a trailing concatenated JSON value -- is rejected 400 with the stored row unchanged
result: pass
source: automated
coverage_id: 20-02-D8

### 11. [D9] Oversize PUT body rejected 4xx, never persists
expected: An oversize PUT body (past the shared 64 KiB ceiling) is rejected 4xx, never 5xx, and never persists
result: pass
source: automated
coverage_id: 20-02-D9

### 12. [D10] Digest defaults survive every rejected write
expected: Digest defaults (disabled/daily) survive every rejected write on a fresh schema
result: pass
source: automated
coverage_id: 20-02-D10

### 13. [D11] No session cookie answers 401, not 403
expected: A gated instance with no session cookie answers 401 (not 403) on both verbs, and the 401 body carries no settings key
result: pass
source: automated
coverage_id: 20-02-D11

### 14. [D12] Missing X-Requested-With refused 403 with zero store calls
expected: A gated PUT with a real session but no X-Requested-With header is refused 403 with the store never called; with the header it succeeds 200 with the store called exactly once
result: pass
source: automated
coverage_id: 20-02-D12

### 15. [D13] Ungated never 401s; unconfigured answers 503
expected: An inert (ungated) server never answers 401 on either verb; a server built without WithSettings answers 503 with the shared fixed body on both verbs
result: pass
source: automated
coverage_id: 20-02-D13

### 16. [D14] No secret leak in store-error responses
expected: A store error whose text embeds a DSN password and a webhook token never reaches the raw response body on either verb
result: pass
source: automated
coverage_id: 20-02-D14

### 17. [D15] Real-time notification behavior is byte-for-byte unchanged (20-02)
expected: Real-time notification behavior (internal/notifier, internal/poller, internal/detection) is byte-for-byte unchanged
result: pass
source: automated
coverage_id: 20-02-D15

### 18. [D1] Flipping the digest-mode switch persists and confirms inline
expected: An operator on /system sees the Digest notifications card, flips the digest-mode switch, and the change is persisted and confirmed inline (Saved.)
result: pass
source: automated
coverage_id: 20-03-D1

### 19. [D2] A failed save keeps the last-known-good value on screen
expected: A failed save keeps the last-known-good value on screen and shows the locked failure copy
result: pass
source: automated
coverage_id: 20-03-D2

### 20. [D3] Double interaction starts one request; control disabled in flight
expected: Two switch interactions dispatched in the same task start exactly one request, and the control is disabled while a save is in flight on both the resolve and reject paths
result: pass
source: automated
coverage_id: 20-03-D3

### 21. [D4] Digest settings ride the shared mount/Refresh fetch
expected: Digest settings ride the same single mount/Refresh Promise.all fetch as the status payload, and a digest-settings-only failure produces the existing page-level error state, never a separate surface
result: pass
source: automated
coverage_id: 20-03-D4

### 22. [D5] Wire type matches the backend field-for-field
expected: NotificationSettings wire type matches internal/httpserver/settings.go's json tags character-for-character, including digest_last_sent_at typed string | null
result: pass
source: automated
coverage_id: 20-03-D5

### 23. [D6] updateDigestSettings() sends full-object PUT with CSRF header
expected: updateDigestSettings() sends full-object PUT semantics carrying the centrally-injected CSRF header, and both wrappers propagate a real ApiError on 401/400
result: pass
source: automated
coverage_id: 20-03-D6

### 24. [D1] Picking Daily/Weekly saves with no submit step
expected: An operator picks Daily or Weekly from the cadence control on /system with no submit step, and the choice survives a reload
result: pass
source: automated
coverage_id: 20-04-D1

### 25. [D2] Cadence control stays visible/populated (dimmed) when digest is off
expected: The cadence control stays visible, focusable, and populated with the stored value when digest mode is off -- dimmed, never hidden, never reset to a placeholder
result: pass
source: automated
coverage_id: 20-04-D2

### 26. [D3] Toggling digest mode never changes the displayed cadence
expected: Toggling digest mode never changes the displayed cadence, and a cadence change always sends both fields on the full-object PUT so it can never clear digest_enabled as a side effect
result: pass
source: automated
coverage_id: 20-04-D3

### 27. [D4] A failed cadence save keeps the previous cadence on screen
expected: A failed cadence save keeps the previous cadence on screen and shows the same shared failure copy the switch's failure path uses
result: pass
source: automated
coverage_id: 20-04-D4

### 28. [D5] A never-sent instance reads "Never sent yet"
expected: A never-sent instance (digest_last_sent_at null) reads the literal 'Never sent yet', never blank and never the view's usual em-dash fallback
result: pass
source: automated
coverage_id: 20-04-D5

### 29. [D6] A populated watermark renders a formatted <time> element
expected: A populated digest_last_sent_at renders a <time> carrying the ISO value in dateTime, the full local timestamp in title, and the shared absolute-time formatter's text -- the same pair SourcePanel already uses for finished_at
result: pass
source: automated
coverage_id: 20-04-D6

### 30. [D7] Embedded bundle rebuilt and the full Definition of Done is green
expected: The committed embedded bundle under internal/webassets/build/client is rebuilt from this phase's SPA source, so a Go binary built from a clone actually serves the digest card, and the full Definition of Done (go vet, golangci-lint, integration suite, coverage-gate, sqlc-check, frontend prettier+vitest) is green with internal/notifier, internal/poller and internal/detection carrying no diff
result: pass
source: automated
coverage_id: 20-04-D7

## Summary

total: 30
passed: 30
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
