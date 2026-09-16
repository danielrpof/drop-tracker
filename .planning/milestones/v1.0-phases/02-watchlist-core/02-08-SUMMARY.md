---
phase: 02-watchlist-core
plan: 08
subsystem: database
tags: [security, redaction, regexp, dsn, secrets, gitleaks, testing]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: "internal/db/migrate.go's RunMigrations, redactDSN, redactError, and the retry/backoff policy"
provides:
  - "kvPasswordPattern in redactError, closing the libpq keyword/value-form DSN password leak (G-02-2/CR-01)"
  - "internal/db/redact_test.go: a shared dsnFixtures table pinning redactError and redactDSN against one list of accepted DSN forms"
  - "an honestly named and documented migration test (TestRunMigrations_NeverLogsDSN_KeywordValueForm_DialFailurePath) that no longer claims to guard a gap it never exercised"
affects: [internal/db, security, gap-closure]

# Actuals (#2632)
actuals:
  tokens: 3460
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "In-package test file (package db) for unexported-function unit tests, external package db_test for real-Postgres integration tests -- same split as internal/watchlist"
    - "One shared fixture table (dsnFixtures) drives tests for two sibling functions with the same stated guarantee, so neither can silently outpace the other's coverage again"

key-files:
  created:
    - internal/db/redact_test.go
  modified:
    - internal/db/migrate.go
    - internal/db/migrate_test.go

key-decisions:
  - "Regression test for the keyword/value leak is unit-level (constructed error text run through redactError directly), not an end-to-end RunMigrations repro -- diagnosis confirmed the one reachable pgconn.ParseConfig failure path does not currently leak under pinned pgx v5.10.0, because ParseConfigError.Error() self-redacts before redactError ever sees the text. An integration-level test would pass identically before and after the fix, which is exactly the flaw this plan corrects in the pre-existing test."
  - "kvPasswordPattern's quoted alternative is listed before the unquoted one in the regex alternation, since Go's regexp prefers the earlier alternative at a given starting position -- an unquoted branch first would truncate a single-quoted value at its first internal space"
  - "redactError runs the userinfo strip before the keyword/value substitution, since the userinfo strip removes a whole scheme://user:pass@ span including the scheme, and running it first means the keyword/value pass only ever sees what survived"
  - "Every fixture password is the project's existing non-entropic marker local-test-fixture-password, or a phrase built from it, never a new realistic-looking secret -- keeps the gitleaks pre-commit hook from flagging its own regression tests"

patterns-established:
  - "A doc-comment cross-reference between sibling functions with an identical stated guarantee (redactDSN <-> redactError), each pointing at the other and at the shared test file, so a future reader auditing one finds the other"

requirements-completed: [OPS-02, OPS-03]

coverage:
  - id: D1
    description: "redactError strips libpq keyword/value-form DSN passwords (canonical, whitespace-padded, single-quoted-with-spaces, differently-cased) and password-bearing URL query parameters, not only URL-form userinfo"
    requirement: "OPS-03"
    verification:
      - kind: unit
        ref: "internal/db/redact_test.go#TestRedactError_NeverEchoesPassword"
        status: pass
    human_judgment: false
  - id: D2
    description: "Redaction is surgical -- host, database name, and surrounding failure text survive; a visible redaction placeholder replaces the password"
    requirement: "OPS-03"
    verification:
      - kind: unit
        ref: "internal/db/redact_test.go#TestRedactError_KeepsDiagnosticContext"
        status: pass
    human_judgment: false
  - id: D3
    description: "Error text merely mentioning the word password without assigning one (e.g. a Postgres auth-failure message) passes through byte-identical"
    requirement: "OPS-03"
    verification:
      - kind: unit
        ref: "internal/db/redact_test.go#TestRedactError_LeavesNonDSNTextAlone"
        status: pass
    human_judgment: false
  - id: D4
    description: "redactDSN and redactError are pinned against one shared dsnFixtures table, so a DSN form added for one helper is automatically asserted against the other"
    requirement: "OPS-02"
    verification:
      - kind: unit
        ref: "internal/db/redact_test.go#TestRedactDSN_NeverEchoesPassword"
        status: pass
    human_judgment: false
  - id: D5
    description: "The migration test that only exercises a TCP dial-failure says so in its name and comment, and no longer claims to guard the keyword/value redaction gap it never exercised"
    verification:
      - kind: unit
        ref: "internal/db/migrate_test.go#TestRunMigrations_NeverLogsDSN_KeywordValueForm_DialFailurePath"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-08-06
status: complete
---

# Phase 02 Plan 08: Close redactError's keyword/value DSN password leak (G-02-2/CR-01) Summary

**Added `kvPasswordPattern` to `internal/db/migrate.go`'s `redactError` so libpq keyword/value-form and query-parameter DSN passwords are stripped alongside the URL-userinfo form it already handled, proven by a new unit-level `dsnFixtures` table shared with `redactDSN` and a corrected, honestly-named migration test.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 2
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments

- `redactError` now strips password credentials from every DSN form `config.Config.DatabaseURL` accepts: URL-form userinfo, keyword/value canonical/whitespace-padded/single-quoted/differently-cased spellings, and URL query-parameter passwords -- while leaving host, database name, surrounding failure text, and non-assigning mentions of "password" untouched.
- Added `internal/db/redact_test.go` (in-package `package db`, following the `normalize_test.go` precedent) with a shared `dsnFixtures` table that pins both `redactError` and `redactDSN` against one list of accepted DSN forms, closing the exact coverage-divergence CR-01 identified.
- Corrected `migrate_test.go`'s keyword/value migration test name and doc comments (both the test and its `closedPortKeywordValueDSN` helper), which had claimed to guard the CR-01 regression class despite exercising only a TCP dial-refused error that never contains DSN text.

## Task Commits

Each task was committed atomically (task 1 as a TDD RED/GREEN pair, task 2 as one test-only commit):

1. **Task 1 (RED): failing test for redactError keyword/value coverage** - `9dedc4a` (test)
2. **Task 1 (GREEN): give redactError keyword/value coverage** - `cd7fe8a` (feat)
3. **Task 2: pin redactDSN and redactError to one shared DSN-form table, rename the dial-failure test** - `3c5922e` (test)

## Files Created/Modified

- `internal/db/redact_test.go` - New: `dsnFixtures` table plus `TestRedactError_NeverEchoesPassword`, `TestRedactError_KeepsDiagnosticContext`, `TestRedactError_LeavesNonDSNTextAlone`, `TestRedactDSN_NeverEchoesPassword`
- `internal/db/migrate.go` - Added `kvPasswordPattern` and rewrote `redactError` to apply it after the existing userinfo strip; extended both functions' doc comments
- `internal/db/migrate_test.go` - Renamed `TestRunMigrations_NeverLogsDSN_KeywordValueForm` to `..._DialFailurePath`; corrected its doc comment and `closedPortKeywordValueDSN`'s doc comment to stop claiming CR-01 regression coverage they never provided

## Decisions Made

See `key-decisions` in frontmatter. In short: fix `redactError` for real, but gate the regression test at the unit level (constructed error text) rather than trying to force an end-to-end `pgconn.ParseConfig` failure through `RunMigrations` -- the diagnosis in `.planning/debug/migrate-redacterror-keyword-value-dsn-leak.md` established that path doesn't currently leak under pgx v5.10.0's own incidental self-redaction, so an integration-level test would have passed identically before and after the fix, exactly the flaw being corrected in the pre-existing test.

## Deviations from Plan

None - plan executed exactly as written. Both tasks' `<action>`, `<behavior>`, and `<done>` sections were followed as specified; the RED phase was verified honestly by temporarily reverting `migrate.go`'s change (via `git stash`, safe here since this is a sequential run on the main working tree, not a worktree) and confirming every keyword/value and query-parameter fixture failed as predicted before restoring the fix.

## Issues Encountered

None. All automated `<verify>` steps in both tasks passed on first attempt after implementation, including the full `internal/db` package and full suite against the real Postgres fixture (`docker compose up -d --wait postgres`), and `pre-commit run gitleaks --all-files` reported no new finding at every checkpoint.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Gap G-02-2 (CR-01, the review's one Critical finding) is closed. `redactError` now meets its own stated contract independently of pgx's undocumented internal behaviour.
- `go.mod` and `go.sum` are byte-identical to before this plan -- no dependency movement.
- `internal/watchlist/`, `internal/httpserver/`, `queries/`, and `internal/db/sqlc/` are untouched, confirming this plan's independence from plan 02-07 (already completed sequentially on this same checkout, commits `3d1841c..b1620cc`).
- `/gsd-verify-work` can reconcile gap G-02-2 as closed on resume.

---
*Phase: 02-watchlist-core*
*Completed: 2026-08-06*
