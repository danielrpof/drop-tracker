---
phase: 21-real-time-digest-mutual-exclusion
reviewed: 2026-09-16T00:00:00Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - CONTEXT.md
  - cmd/server/main.go
  - docs/adr/0002-one-outbox-one-sender-lock.md
  - internal/detection/detector_test.go
  - internal/notifier/notifier.go
  - internal/notifier/notifier_test.go
  - internal/notifier/suppress_test.go
  - internal/notifier/timeout_test.go
  - internal/webassets/build/client/assets/manifest-056e0136.js
  - internal/webassets/build/client/assets/system-C6s0fhiM.js
  - internal/webassets/build/client/index.html
  - web/app/components/system/DigestSettings.test.tsx
  - web/app/components/system/DigestSettings.tsx
findings:
  critical: 0
  warning: 2
  info: 0
  total: 2
status: issues_found
---

# Phase 21: Code Review Report

**Reviewed:** 2026-09-16
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

## Summary

This phase adds the digest-mode gate to `internal/notifier.Notifier.NotifyPending`: a required `SettingsReader`, a top-of-pass digest-mode check before any row is listed, a per-send mid-pass re-read that stops the pass at the next send boundary, a `CreatedAt`-anchored stale-release cutoff (replacing the old `time.Now()` anchor), and transition-only mode-change logging. `cmd/server/main.go`, `internal/detection/detector_test.go` are updated only to thread the new required constructor argument through. `docs/adr/0002-one-outbox-one-sender-lock.md` and `CONTEXT.md` are new/updated documentation for the same change. The frontend change adds a static helper line to `DigestSettings.tsx` plus matching tests; the two build artifacts are the corresponding regenerated Vite bundle and manifest.

I traced every return path through `NotifyPending` for the specific question the task asked about: whether a settings-read error can ever leave the `notifying` CAS guard held. It cannot — `defer n.notifying.Store(false)` is registered immediately after a successful CAS and every subsequent return (top-of-pass read failure, digest-on gate, `ListUnnotified` failure, mid-loop read failure, mid-loop digest-on gate, `MarkNotified` failure, spacing-wait context cancellation, and normal completion) flows through it. The mid-pass re-read is correctly placed before each `Send` call, so the ADR's "at most one message already in flight" residual-delivery claim holds given sends are strictly serial in this loop. The freshness-cutoff math (`anchor.AddDate(0, 0, -n.maxAgeDays-1)`) is exercised by an extensive, deliberately boundary-focused table test and matches the ADR/comment's stated rationale. The frontend's JSX expression-container wrapping of the static helper string (`{"While on, ..."}`) is not an XSS surface: React auto-escapes JSX child text regardless of whether it is written as a literal text node or as a string inside `{}` — no `dangerouslySetInnerHTML`/`innerHTML` is used anywhere in the diff. The two build artifacts are consistently regenerated (old-hash files renamed away, `index.html`'s `<script>` references and the manifest's own `url`/`version` fields all point at the new hash, and the new bundle's minified text contains the exact new helper string).

Two findings below, both in the newly-added Go code: one behavioral (an existing log invariant is silently broken by the new mid-pass early-return path, with no test covering the interaction) and one straightforward formatting defect in a new test file.

## Warnings

### WR-01: Mid-pass digest-mode flip can silently swallow the stale-event suppression summary log

**File:** `internal/notifier/notifier.go:267-338`
**Issue:** The `suppressed > 0` summary log ("notify pass suppressed stale events") sits after the `for` loop, so it only fires when the loop runs to completion. The mid-pass digest-mode re-read (added this phase, lines 284-293) can `return nil` from inside the loop before it finishes. If a pass suppresses one or more stale/backlog rows and then, later in the same pass, observes digest mode turning on before reaching (or while re-checking) a subsequent row, the pass returns early and the summary line for the suppression that already happened is never logged.

This directly undermines the log's own stated purpose, in the comment immediately above it: "suppressed is emitted only when something was suppressed, so over-suppression -- the one real risk -- stays visible." Before this phase, the loop had no early-return path other than a hard `MarkNotified`/`Send`-adjacent error, so this gap did not exist. `ListUnnotified`'s ordering (oldest first, `created_at, id`) makes stale backlog rows likely to be encountered before fresher ones in a mixed batch, so this is not a corner case reachable only by contrived orderings — it is the ordinary shape of "an old backlog plus newly detected fresh events in the same pass, with an operator toggling digest mode while it runs."

There is no data-loss risk (the suppressed rows are still correctly acked before the early return), but it is an observability regression: exactly the "silently dropped row" scenario the suppression summary log exists to make visible can now go unlogged when it coincides with a digest-mode toggle.

No test in `notifier_test.go` or `suppress_test.go` exercises "suppression occurred, then the pass exits early via the mid-loop digest gate" — `TestNotifyPending_MidPass_SuppressionAcksDoNotReRead` only proves the reader isn't re-consulted for suppressed rows and never observes digest mode flipping on, and `TestNotifyPending_StaleRowsAckedWithoutSending` never enables digest mode.

**Fix:** Emit the suppression summary before the mid-loop early return as well (or unconditionally via a `defer`/helper invoked on every exit path once `suppressed > 0`), e.g.:
```go
if cfg.DigestEnabled {
    if suppressed > 0 {
        logger.Info("notify pass suppressed stale events",
            slog.Int("suppressed_count", suppressed),
            slog.Int("pending_count", len(events)),
            slog.Int("max_release_age_days", n.maxAgeDays),
        )
    }
    n.observeDigestMode(logger, true, len(events)-i, true)
    return nil
}
```
and add a regression test that suppresses at least one row, then flips digest mode on mid-pass, and asserts the summary line is still present.

### WR-02: `internal/notifier/timeout_test.go` is not gofmt-formatted

**File:** `internal/notifier/timeout_test.go:83-86`
**Issue:** The new `wedgingSettingsReader` struct's field alignment is not gofmt-canonical:
```go
type wedgingSettingsReader struct {
	calls atomic.Int32
	wedgeFirstCallOnly bool
}
```
`gofmt -l internal/notifier/timeout_test.go` flags this file (confirmed by running it); `gofmt -d` shows the only diff is the missing alignment padding after `calls`. `golangci-lint run` and `go vet` both pass clean on this package, so nothing in this project's currently-configured Definition-of-Done gates (no `gofmt`/`goimports`/`gofumpt` entry in `.golangci.yml`'s `formatters:` section, no standalone `gofmt -l` step in the Makefile or CI workflow) actually catches this — it will merge as-is unless caught here.

**Fix:** Run `gofmt -w internal/notifier/timeout_test.go` (or add a `formatters: {enable: [gofmt]}` block to `.golangci.yml` so golangci-lint's own `--fix` catches this class of issue going forward, per this repo's stated reliance on the pre-commit hook as "the fast local mirror of CI").

---

_Reviewed: 2026-09-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
