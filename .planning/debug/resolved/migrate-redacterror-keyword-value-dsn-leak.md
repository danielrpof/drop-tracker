---
status: resolved
trigger: "UAT gap G-02-2 (02-UAT.md Test 2): user decision on CR-01 (Critical, 02-REVIEW.md) is 'fix'. CR-01: internal/db/migrate.go's redactError only strips URL-form DSN userinfo, not libpq keyword/value-form password=.... Investigate to confirm the finding still holds against current code -- including whether the existing regression test actually exercises the vulnerable path -- before a gap-closure plan is written. find_root_cause_only, read-only session (shared main checkout, no fix applied, no commits)."
created: 2026-08-06T18:23:04Z
updated: 2026-08-06T21:30:00Z
---

## Current Focus

hypothesis: CONFIRMED. `redactError` (`internal/db/migrate.go:107-109`) only strips URL-form DSN userinfo via `userInfoPattern`; it has no equivalent handling for libpq keyword/value-form `password=...` fragments, unlike its sibling `redactDSN` which delegates to `pgconn.ParseConfig` specifically because a regex/URL-only approach silently fails on that form. `TestRunMigrations_NeverLogsDSN_KeywordValueForm` does not exercise the vulnerable path: it triggers a TCP dial-refused error, which never contains DSN text at all, so the test passes identically whether or not `redactError` handles keyword/value form.

Refinement beyond 02-REVIEW.md (does not overturn CR-01, narrows the correct reproduction/fix approach): empirically confirmed that with the pinned pgx v5.10.0, the actually-reachable runtime path through `RunMigrations` (a malformed `DATABASE_URL` causing `sql.Open`/`PingContext` -> `pgconn.ParseConfig` to fail) does NOT currently leak the raw password, because `pgconn.ParseConfigError.Error()` already self-redacts `password=...` fragments before `redactError` ever sees them. This is incidental upstream behavior, not a documented contract (pgx's own source shows the maintainers reconsidering it), and it doesn't cover any other error source. CR-01 is still a valid, real gap in `redactError`'s own documented contract and should be fixed as defense-in-depth -- but the regression test needs to exercise `redactError` directly (unit-level, constructed error string) rather than trying to force an end-to-end `pgconn` parse failure through `RunMigrations`, since that specific path does not currently leak with today's dependency versions.

test: complete -- goal is find_root_cause_only.
expecting: n/a
next_action: none -- return ROOT CAUSE FOUND to orchestrator; a downstream gap-closure plan implements the fix per Resolution below.

## Symptoms

expected: "A recorded accept-or-fix decision for internal/db/migrate.go's redactError only stripping URL-form DSN userinfo, not libpq keyword/value-form password=... -- tracked separately from Phase 2 since the file has zero Phase 2 commits against it, but flagged because it is Critical severity and directly implicates CLAUDE.md's 'all secrets via environment variables only... nothing real ever committed' constraint."
actual: User's UAT response was "fix".
errors: None reported -- this is a static code-review finding (secret-leak gap), not a runtime crash or failing test.
reproduction: |
  UAT Test 2 in .planning/phases/02-watchlist-core/02-UAT.md.

  Underlying code-level repro (unit level -- NOT the review's originally-suggested end-to-end RunMigrations repro; see Evidence for why that specific one does not currently leak with pgx v5.10.0):
    1. Construct an error whose .Error() text embeds a raw libpq keyword/value DSN fragment, e.g. a string containing "... host=127.0.0.1 user=drop_tracker password=<value> dbname=drop_tracker ...".
    2. Run it through redactError's logic (userInfoPattern.ReplaceAllString).
    3. Observe the password is NOT stripped -- output is byte-identical to input for this DSN form (verified directly in this investigation, see Evidence).
started: Originally flagged in Phase 1 review; reconfirmed unresolved and re-flagged during Phase 02 review (02-REVIEW.md, reviewed 2026-08-06T00:00:00Z). Zero Phase 2 commits touch internal/db/migrate.go.

## Eliminated

(none -- investigation confirmed the pre-existing diagnosis directly, with one added refinement narrowing which specific reproduction path is currently reachable through this codebase's dependency graph; no alternative root-cause hypotheses were needed)

## Evidence

- timestamp: 2026-08-06T18:23:04Z
  checked: internal/db/migrate.go, full file (218 lines), current working tree (git status clean except this new debug session file; HEAD 2f5c766)
  found: |
    userInfoPattern (line 82): `[a-zA-Z][a-zA-Z0-9+.-]*://[^/@\s]*@` -- matches only the URL-form "scheme://user:pass@" prefix.
    redactDSN (lines 96-102): delegates to pgconn.ParseConfig, explicitly because (per its own doc comment, lines 88-95) a hand-rolled url.Parse-based parser "silently treats an entire keyword/value DSN as an opaque path and echoes it back verbatim, password included."
    redactError (lines 107-109): `return userInfoPattern.ReplaceAllString(err.Error(), "")` -- applies ONLY userInfoPattern; no equivalent keyword/value handling exists anywhere in the function.
    redactError is used at line 138 (retry loop, sets lastErrMsg from each failed attempt's error), embedded in the Warn log line at lines 151-155 (`slog.String("error", lastErrMsg)`), and folded into the final returned error at line 164.
    Line numbers match 02-REVIEW.md's citation (77-109) exactly -- no drift since the review.
  implication: CR-01's core claim is confirmed verbatim against current code. redactError's regex structurally cannot match or redact keyword/value-form credentials, while its sibling redactDSN was deliberately built to handle exactly this DSN form -- an internal inconsistency between two functions with the identical stated purpose (never let dsn's credentials reach a log or error).

- timestamp: 2026-08-06T18:23:04Z
  checked: internal/db/migrate_test.go, TestRunMigrations_NeverLogsDSN_KeywordValueForm (lines 271-294) and its closedPortKeywordValueDSN helper (lines 51-64); ran `go test ./internal/db/... -run TestRunMigrations_NeverLogsDSN -v`
  found: |
    closedPortKeywordValueDSN opens then immediately closes a TCP listener on 127.0.0.1 and returns a keyword/value DSN pointing at that now-closed port. RunMigrations against this DSN fails at runMigrationsOnce's sqlDB.PingContext (migrate.go:189) with a TCP dial-refused error, not a DSN-parse error.
    Test run output: both TestRunMigrations_NeverLogsDSN and TestRunMigrations_NeverLogsDSN_KeywordValueForm PASS currently (no live Postgres required for either).
  implication: Confirms 02-REVIEW.md's characterization exactly. The test's assertions (password/scheme-prefix absent from log+error) trivially hold because a dial-refused error string (e.g. "dial tcp 127.0.0.1:PORT: connect: connection refused" / "connectex: No connection could be made...") never contains DSN content at all, regardless of whether redactError handles keyword/value form. The test does not exercise redactError's actual gap and would pass unchanged even if redactError were reverted to a no-op.

- timestamp: 2026-08-06T18:23:04Z
  checked: github.com/jackc/pgx/v5 v5.10.0 source (local module cache) -- pgconn/config.go (ParseConfigWithOptions), pgconn/errors.go (ParseConfigError.Error, redactPW, ConnectError.Error), conn.go (pgx.ParseConfigWithOptions), stdlib/sql.go (Driver.Open/OpenConnector, driverConnector.Connect). Also ran two throwaway offline Go programs (outside the repo, in the session scratchpad, deleted after use -- not committed to drop-tracker) to empirically verify both halves of the claim.
  found: |
    Traced the exact call chain migrate.go uses: sql.Open("pgx", dsn) (migrate.go:183) does NOT parse the DSN eagerly -- pgx's stdlib Driver.OpenConnector just stores the name string in a struct. Parsing happens lazily on first use, here sqlDB.PingContext(ctx) at migrate.go:189 -> driverConnector.Connect -> pgx.ParseConfig(dc.name) -> pgconn.ParseConfigWithOptions. On a malformed keyword/value DSN this returns a *pgconn.ParseConfigError{ConnString: <raw dsn>, ...}. Its Error() method (pgconn/errors.go:136-145) computes `connString := redactPW(e.ConnString)` BEFORE formatting the message. redactPW (pgconn/errors.go:230-243) has its own regexes for both URL form (via url.Parse+redactURL) and keyword/value form (a `password=[^ ]*` pattern replaced with `password=xxxxx`) -- so the ConnString embedded in ParseConfigError's own .Error() output is already password-redacted by pgx itself, before migrate.go's redactError(err) ever runs on it.

    Empirically verified (throwaway program #1, reusing drop-tracker's own go.mod/go.sum for the module graph, GOPROXY=off, no live network): called pgconn.ParseConfig directly on a malformed keyword/value DSN containing a marker password value.
      input dsn:   host=127.0.0.1 port=9999 user=drop_tracker password=marker-secret-value dbname=drop_tracker sslmode=disable ==invalid==
      err.Error(): cannot parse `host=127.0.0.1 port=9999 user=drop_tracker password=xxxxx dbname=drop_tracker sslmode=disable ==invalid==`: failed to parse as keyword/value (invalid keyword/value)
      -> the marker password is absent from the error string; pgx already replaced it with "xxxxx" internally.

    Empirically verified (throwaway program #2): migrate.go's own userInfoPattern regex, executed in isolation against a constructed string containing a keyword/value password fragment, leaves that fragment completely untouched -- confirming the code-level gap in redactError is real and independent of pgx's behavior.

    Also checked pgconn's other error type with a similar risk shape, ConnectError (pgconn/errors.go:63-76), used for live-connection (not parse) failures. Its Error() only formats "failed to connect to `user=%s database=%s`:" from the already-parsed Config struct's non-secret fields -- it never includes the raw connString, so it was never a DSN-leak vector either.
  implication: |
    This refines, but does not overturn, CR-01. Given pgx v5.10.0, the specific end-to-end reproduction 02-REVIEW.md suggested (force a pgconn.ParseConfig failure through RunMigrations and check the returned error/log for the password) will NOT actually demonstrate a leak today -- pgx's own internal redaction already closes that specific path. However:
      1. redactError's own doc comment claims an independent guarantee ("any error text that happens to embed a raw DSN verbatim can still be scrubbed before it reaches a log line or a returned error") that its implementation does not fulfill for keyword/value form -- a real gap in the function's own stated contract, independent of any dependency's behavior.
      2. pgx's self-redaction is not a documented, stable guarantee. pgconn/errors.go's own comment directly above redactPW's call site (lines 137-139) shows the maintainers actively second-guessing whether ParseConfigError should keep exposing the raw ConnString at all -- a future pgx version bump could change or remove this behavior with zero signal to drop-tracker.
      3. redactError is the general-purpose safety net for ANY error text reaching RunMigrations' retry-log/return path, not specifically pgconn.ParseConfigError -- relying on one upstream error type's incidental internal behavior as the sole protection is fragile defense-in-depth, not a designed guarantee, and does not meet CLAUDE.md's absolute "all secrets via environment variables only... nothing real ever committed" bar for a project whose stated purpose includes demonstrating security rigor.
    Conclusion: fix as CR-01 originally suggested (add keyword/value handling to redactError), but write the regression test at the redactError-input level (a constructed error/string containing a keyword/value password) rather than trying to force it through a live pgconn parse failure via RunMigrations, since that specific integration path is not currently exploitable and an integration-level test would give false confidence either way (it would pass both before and after the fix, exactly like the existing TestRunMigrations_NeverLogsDSN_KeywordValueForm does).

## Resolution

root_cause: |
  internal/db/migrate.go's redactError (lines 107-109) only strips URL-form DSN userinfo via userInfoPattern (`scheme://user:pass@...`); it has no equivalent regex/logic for libpq keyword/value-form credentials (`password=...`), even though config.Config.DatabaseURL places no format constraint and explicitly accepts both forms, and the sibling function redactDSN was deliberately built (via pgconn.ParseConfig) to handle both. This is a genuine, unaddressed gap in redactError's own documented contract ("any error text that happens to embed a raw DSN verbatim can still be scrubbed"). The existing regression test, TestRunMigrations_NeverLogsDSN_KeywordValueForm, does not exercise this gap: its DSN produces a TCP dial-refused error, which never contains DSN text at all regardless of redactError's regex coverage, so the test passes identically whether or not the gap is fixed.

  Refinement (narrows the fix/test approach, does not change the accept-or-fix disposition): with the pinned pgx v5.10.0, the one reachable runtime path through RunMigrations that could trigger a DSN-parse failure (a malformed DATABASE_URL causing PingContext -> pgconn.ParseConfig to fail) does not currently leak a raw password, because pgconn's own ParseConfigError.Error() self-redacts password= fragments (via its internal redactPW helper) before redactError ever runs on the text. This is incidental upstream behavior, not a documented/stable guarantee -- pgx's own source comments show its maintainers reconsidering it -- and it does not cover any other error source that might embed a raw DSN in the future. CR-01 remains a valid Critical-severity gap to close as defense-in-depth against redactError's own incomplete contract; the fix's regression test must exercise redactError directly (unit-level, constructed error string) rather than trying to force an end-to-end pgconn parse failure, since that specific integration path does not currently leak and would not distinguish a fixed vs. unfixed redactError.
fix: (not applied -- goal is find_root_cause_only for this session; a downstream gap-closure plan implements the fix per the Suggested Fix Direction returned to the orchestrator)
verification: (not applicable -- no fix applied in this session)
files_changed: []
