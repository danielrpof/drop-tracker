---
status: resolved
phase: 02-watchlist-core
source: [02-VERIFICATION.md]
started: 2026-08-06T19:30:00Z
updated: 2026-08-06T21:30:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Decide disposition of new-WR-01 and new-WR-02 (in-scope, Warning-severity, code review findings)
expected: A recorded decision (accept as documented risk for v1, or open a follow-up plan) for each finding, the same pattern already used for the prior WR-01/WR-02 pair.
result: issue
reported: "fix"
severity: major

### 2. Out-of-scope flag: CR-01 (Critical, Phase 1 file, currently unresolved)
expected: A recorded accept-or-fix decision for internal/db/migrate.go's redactError only stripping URL-form DSN userinfo, not libpq keyword/value-form password=... — tracked separately from Phase 2 since the file has zero Phase 2 commits against it, but flagged because it is Critical severity and directly implicates CLAUDE.md's "all secrets via environment variables only... nothing real ever committed" constraint.
result: issue
reported: "fix"
severity: major

## Summary

total: 2
passed: 0
issues: 2
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-02-1
  truth: "A recorded decision (accept as documented risk for v1, or open a follow-up plan) for each finding, the same pattern already used for the prior WR-01/WR-02 pair."
  status: resolved
  reason: "User reported: fix"
  severity: major
  test: 1
  root_cause: "WR-01: Service.UpdatePreferences (internal/watchlist/service.go:216-263) has no guard against both PreferencesParams.ReleaseTypes and MutedEventTypes being nil -- the WLST-05/06 'reject neither axis supplied' contract is enforced only in the HTTP layer (internal/httpserver/watchlist.go:231-234), so any non-HTTP caller reaching the service directly gets a silent no-op success. WR-02: both handleAddWatchlist (internal/httpserver/watchlist.go:90-96) and handleUpdateWatchlist (internal/httpserver/watchlist.go:223-229) call dec.Decode(&req) exactly once and never verify the JSON stream is exhausted afterward, so a body with trailing JSON after a valid object decodes successfully with the extra data silently discarded."
  artifacts:
    - path: "internal/watchlist/service.go:216-263"
      issue: "UpdatePreferences missing domain-boundary no-op guard (WR-01)"
    - path: "internal/httpserver/watchlist.go:90-96"
      issue: "handleAddWatchlist decoder does not check for trailing JSON after a valid body (WR-02)"
    - path: "internal/httpserver/watchlist.go:223-229"
      issue: "handleUpdateWatchlist decoder does not check for trailing JSON after a valid body (WR-02)"
  missing:
    - "Add a package-level sentinel (e.g. ErrNoPreferencesSupplied) returned early by Service.UpdatePreferences when both fields are nil, consistent with ErrDuplicate/ErrNotFound/ErrInvalidReleaseType"
    - "Collapse handleUpdateWatchlist's existing HTTP-layer nil/nil check to an errors.Is match against the new sentinel (single source of truth)"
    - "After each dec.Decode(&req) succeeds in both handlers, call dec.Decode(&struct{}{}) and require errors.Is(err, io.EOF); reject with 400 otherwise"
  debug_session: ".planning/debug/watchlist-updatepreferences-noop-guard-and-json-trailing-data.md"
  source: "02-REVIEW.md WR-01, WR-02"

- gap_id: G-02-2
  truth: "A recorded accept-or-fix decision for internal/db/migrate.go's redactError only stripping URL-form DSN userinfo, not libpq keyword/value-form password=... constraint."
  status: resolved
  reason: "User reported: fix"
  severity: major
  test: 2
  root_cause: "redactError (internal/db/migrate.go:107-109) only strips URL-form DSN userinfo via userInfoPattern (scheme://user:pass@...); it has no equivalent handling for libpq keyword/value-form credentials (password=...), even though config.Config.DatabaseURL accepts both forms and the sibling redactDSN function was deliberately built (via pgconn.ParseConfig) to handle both. The existing regression test TestRunMigrations_NeverLogsDSN_KeywordValueForm does not exercise this gap -- it triggers a TCP dial-refused error, which never contains DSN text, so it passes identically whether or not the gap is fixed. Empirically confirmed during diagnosis: with pinned pgx v5.10.0, the one reachable runtime path through RunMigrations (a pgconn.ParseConfig failure) does not currently leak the raw password because pgconn.ParseConfigError.Error() already self-redacts password=... before redactError runs -- this does not invalidate the finding (redactError's own contract is still broken, and pgx's incidental self-redaction is not something this codebase controls or should rely on), but it means the fix's regression test must exercise redactError directly at the unit level against a constructed error string, not by trying to force an end-to-end pgconn parse failure."
  artifacts:
    - path: "internal/db/migrate.go:107-109"
      issue: "redactError has no keyword/value-form password coverage"
    - path: "internal/db/migrate.go:82"
      issue: "userInfoPattern only matches URL-form DSN userinfo"
    - path: "internal/db/migrate_test.go:271-294"
      issue: "TestRunMigrations_NeverLogsDSN_KeywordValueForm exercises a dial-refused path, not a DSN-parse-failure path, so it doesn't test the actual gap"
  missing:
    - "Add a second regex (e.g. kvPasswordPattern = regexp.MustCompile(`(?i)\\bpassword=\\S+`)) applied in redactError after the existing URL-form strip, replacing with password=<redacted>"
    - "Add a unit test calling redactError directly against a constructed error/string containing a keyword/value password fragment, asserting the password is absent from the output (do not attempt to force a live pgconn parse failure -- it will not leak given pgx's own internal redaction)"
    - "Rename or annotate the existing TestRunMigrations_NeverLogsDSN_KeywordValueForm to clarify it only covers the dial-failure path, not the parse-failure path"
  debug_session: ".planning/debug/migrate-redacterror-keyword-value-dsn-leak.md"
  source: "02-REVIEW.md CR-01"
