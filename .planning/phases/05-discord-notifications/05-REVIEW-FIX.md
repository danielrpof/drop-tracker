---
phase: 05-discord-notifications
fixed_at: 2026-08-08T21:48:18Z
review_path: .planning/phases/05-discord-notifications/05-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 05: Code Review Fix Report

**Fixed at:** 2026-08-08T21:48:18Z
**Source review:** .planning/phases/05-discord-notifications/05-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (CR-01 through WR-04; IN-01 through IN-03 out of scope for this pass)
- Fixed: 5
- Skipped: 0

**Verification environment:** All builds (`go build ./...`), vets (`go vet ./...`), and package
tests (`go test ./internal/discord/... ./internal/notifier/... -count=1`) ran inside the isolated
worktree at `C:\Users\danie\AppData\Local\Temp\sv-05-reviewfix-N6hVJP`, against the same real
Postgres instance the main checkout uses (`drop-tracker-postgres-1`, `TEST_DATABASE_URL` pointed
at `localhost:5432`). These numbers are reproducible from the main checkout after the worktree's
commits are fast-forwarded onto `main` (see cleanup tail).

## Fixed Issues

### CR-01: Discord webhook payload never suppresses mention parsing

**Files modified:** `internal/discord/client.go`, `internal/discord/client_test.go`
**Commit:** `3a51e02`
**Applied fix:** Added an `allowedMentions` type (`{"parse":[]}`) and a new `AllowedMentions`
field on `webhookPayload`, set unconditionally in `sendAttempt` so every outbound POST disables
Discord's default mention resolution. Added a regression test
(`TestSend_AllowedMentionsAlwaysSuppressed`) asserting the marshalled request body always
contains `"allowed_mentions":{"parse":[]}` and that an `@everyone` field value survives
unescaped into the payload (the fix is transport-layer, not string-mangling display data).

### WR-04: Unbounded/unclamped `retry_after` value from a 429 response

**Files modified:** `internal/discord/client.go`, `internal/discord/client_test.go`
**Commit:** `bf4953d`
**Applied fix:** Added a `maxRetryAfter` (30s) ceiling in `sendAttempt`'s 429 handling, alongside
the existing `<= 0` floor guard. Declared as a `var` (not `const`) specifically so the regression
test (`TestSend_429RetryAfterClamped`) can shrink it to keep the test fast/deterministic instead
of waiting out a real 30s window; it feeds an absurd 3600s `retry_after` and confirms the
inter-request gap stays near the (shrunk) clamp rather than the unclamped value.

### WR-01: A failed `Send` skips the inter-send spacing entirely

**Files modified:** `internal/notifier/notifier.go`, `internal/notifier/notifier_test.go`
**Commit:** `d343ee2`
**Applied fix:** Restructured `NotifyPending`'s loop so the spacing wait (`select` on
`time.After(n.spacing)` / `ctx.Done()`) runs unconditionally between iterations, regardless of
whether `Send` succeeded, failed, or `MarkNotified` failed. Added
`TestNotifyPending_SpacingAppliedEvenAfterFailedSend`, which measures elapsed time across three
consecutive rows sent through a sender that always fails, confirming both inter-send gaps still
meet the configured spacing.

### WR-02: `defaultSpacing`'s doc comment didn't reconcile with the stated rate-limit ceiling

**Files modified:** `internal/notifier/notifier.go`
**Commit:** `a72fd73`
**Applied fix:** Rewrote the doc comment on `defaultSpacing` to cite Discord's actual documented
per-webhook limit (5 requests / 2 seconds, ~150/min) instead of the stale "~30-per-minute" figure,
and explained that 400ms sits right at that ceiling (not comfortably under it) — matching the
math the reviewer worked out. Comment-only change; verified via `go build`/`go vet` plus the full
`internal/notifier` test suite (no logic touched).

### WR-03: A `MarkNotified` failure after a successful `Send` can cause a duplicate post

**Files modified:** `internal/notifier/notifier.go`, `internal/notifier/notifier_test.go`
**Commit:** `aba2366`
**Applied fix:** Added a `logger.Warn(...)` call (event ID, event type, error) immediately before
the existing `return fmt.Errorf(...)` on the `MarkNotified` error path, so this specific
"Discord accepted it, DB didn't record it" duplicate-post window is distinguishable in production
logs from a generic DB outage at the `ListUnnotified` level, per the review's suggestion (a). Added
`markNotifiedFailingQuerier`, a thin `sqlc.Querier`-embedding wrapper that overrides only
`MarkNotified` to fail while delegating everything else to the real Postgres-backed querier, and
`TestNotifyPending_MarkNotifiedFails_LogsWarnAndReturnsError`, which lands squarely in the
"Send succeeded, MarkNotified failed" window and asserts both the Warn-level log line and that
`notified_at` stays NULL.

## Skipped Issues

None — all 5 in-scope findings were fixed.

---

_Fixed: 2026-08-08T21:48:18Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
