---
phase: 02-watchlist-core
verified: 2026-08-06T21:00:00Z
status: passed
score: 61/61 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 43/43
  gaps_closed:
    - "G-02-1 (WR-01): Service.UpdatePreferences had no independent domain-boundary no-op guard -- closed by 02-07 (ErrNoPreferencesSupplied sentinel returned as the method's first statement, ahead of validation and the database call)"
    - "G-02-1 (WR-02): handleAddWatchlist/handleUpdateWatchlist's json.Decoder.Decode never checked the stream was exhausted, silently accepting a body with a second concatenated JSON value -- closed by 02-07 (shared decodeJSONBody helper asserting errors.Is(err, io.EOF) on a second decode)"
    - "G-02-2 (CR-01): internal/db/migrate.go's redactError only stripped URL-form DSN userinfo, not libpq keyword/value-form password=... -- closed by 02-08 (kvPasswordPattern applied after the existing userinfo strip, covering canonical/whitespace-padded/quoted/differently-cased spellings and query-parameter passwords)"
  gaps_remaining: []
  regressions: []
---

# Phase 2: Watchlist Core Verification Report

**Phase Goal:** "Users can fully manage their watchlist — add, remove, list, and configure per-artist alert preferences — through a tested API service layer."
**Verified:** 2026-08-06T21:00:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (plans 02-07, 02-08 landed since the prior VERIFICATION.md, closing gaps G-02-1 and G-02-2 recorded in 02-UAT.md)

**Note on ROADMAP `Mode: mvp`:** As in the prior verification cycle, this phase's ROADMAP goal is not in strict user-story form (confirmed previously via `user-story.validate` returning `valid: false`). This re-verification follows the same established non-MVP-flow precedent rather than requiring a refusal to verify.

**Scope of this pass:** This is a `--gaps-only` re-verification per the task instructions. Plans 02-01 through 02-06's 43 previously-verified truths were re-confirmed (full suite still green, no regression). The two new plans, 02-07 (G-02-1) and 02-08 (G-02-2), were verified in full against their own declared must-haves, independently of their own SUMMARY.md claims. A full code-review pass (`02-REVIEW.md`, commit `6687486`) ran against all 23 phase-02 source files after 02-07/02-08 landed and reports 0 Critical, 5 Warning, 2 Info. Per the task's explicit scoping instruction, none of the 5 Warning/2 Info findings were claimed as fixed by any plan in this phase — they are recorded below as known, out-of-scope issues, not as unverified claims requiring human disposition in this pass.

## Goal Achievement

### Observable Truths

All 43 must-have truths from plans 02-01 through 02-06 were re-confirmed (full suite re-run green, `go build`/`go vet` clean, no regression). The 18 new must-have truths from plans 02-07 and 02-08 were independently verified against the current codebase — source read, targeted named-test execution, and grep-based structural gates re-run directly rather than trusted from either SUMMARY.md.

| # | Plan | Truth | Status | Evidence |
|---|------|-------|--------|----------|
| 1-43 | 02-01..06 | All 43 previously-verified truths (add/duplicate/validation/list/ordering/delete/concurrency/patch/CHECK-backstop/re-add metadata refresh/single-CTE concurrency fix) | ✓ VERIFIED | Full suite re-run green (`go test ./... -count=1` against real Postgres: `internal/config`, `internal/db`, `internal/httpserver`, `internal/watchlist` all `ok`, 0 fail); `go build ./...` and `go vet ./...` both exit 0 |
| 44 | 02-07 | `Service.UpdatePreferences` with neither axis supplied returns `ErrNoPreferencesSupplied` before any database call (G-02-1, WR-01) | ✓ VERIFIED | `internal/watchlist/service.go:231-233` — guard is the method's first statement, ahead of `params` construction; `TestService_UpdatePreferences_NeitherAxisReturnsErrNoPreferencesSupplied` PASS (run by name) |
| 45 | 02-07 | A rejected empty preferences update leaves `updated_at` exactly as it was — no write reaches the database | ✓ VERIFIED | Same test's before/after `SELECT release_types, muted_event_types, updated_at` + `.Equal()` assertion on the real row, PASS |
| 46 | 02-07 | The neither-axis rule rejects before the id is looked up (outranks `ErrNotFound`) | ✓ VERIFIED | `TestService_UpdatePreferences_NeitherAxisOutranksUnknownID` PASS — an id no row holds still reports the sentinel |
| 47 | 02-07 | `PATCH /watchlist/{id}` with a body supplying neither key still answers 400 with byte-identical `{"error":"no preferences supplied"}` | ✓ VERIFIED | `TestWatchlist_Patch_NoPreferencesSuppliedReturns400` and `TestWatchlist_Patch_EmptyBodyStillRejectedEndToEnd` PASS; sentinel message text in `service.go:45` is `"no preferences supplied"`, matches handler's prior wire contract |
| 48 | 02-07 | The neither-axis rule is implemented exactly once, in `internal/watchlist`; the handler owns no second copy | ✓ VERIFIED | `grep -v '^\s*//' watchlist.go \| grep -c 'ErrNoPreferencesSupplied'` == 1 (re-run directly) |
| 49 | 02-07 | `POST /watchlist` and `PATCH /watchlist/{id}` answer 400 for a body carrying a second JSON value after the first; store never called (G-02-1, WR-02) | ✓ VERIFIED | `TestWatchlist_Add_BodyMustContainExactlyOneJSONValue` and `TestWatchlist_Patch_BodyMustContainExactlyOneJSONValue` PASS, all 4 trailing-shape subtests (object/array/scalar/non-JSON) 400 with `called=false` asserted directly against the stub's invocation flag, not just status code |
| 50 | 02-07 | A body followed only by whitespace/trailing newline is still accepted | ✓ VERIFIED | `trailing_whitespace_only` subtest in both tables PASS — 201/200 with `called=true` |
| 51 | 02-07 | Every request-body decode in `internal/httpserver/watchlist.go` runs through one shared path | ✓ VERIFIED | `grep -c 'json.NewDecoder('` == 1 (only inside `decodeJSONBody`); `grep -c 'decodeJSONBody('` == 3 (declaration + 2 call sites) |
| 52 | 02-07 | No Go module added/removed/upgraded | ✓ VERIFIED | `git diff --exit-code -- go.mod go.sum` clean |
| 53 | 02-08 | `redactError` strips a libpq keyword/value-form password from error text, not only URL-form userinfo (G-02-2, CR-01) | ✓ VERIFIED | `internal/db/migrate.go:110,148-149` — `kvPasswordPattern` applied after the userinfo strip; `TestRedactError_NeverEchoesPassword` PASS across all `dsnFixtures` entries (run by name) |
| 54 | 02-08 | Redaction covers whitespace-padded, single-quoted, and differently-cased keyword/value spellings | ✓ VERIFIED | `TestRedactError_NeverEchoesPassword` subtests for all three spellings PASS |
| 55 | 02-08 | A password supplied as a URL query parameter is redacted too | ✓ VERIFIED | `TestRedactError_NeverEchoesPassword/URL_form,_password_as_a_query_parameter,_no_userinfo` PASS |
| 56 | 02-08 | Redaction is surgical: host, database name, and surrounding failure text survive | ✓ VERIFIED | `TestRedactError_KeepsDiagnosticContext` PASS — asserts `host=...`, `dbname=...`, wrapping text, and a visible `password=<redacted>` placeholder all present |
| 57 | 02-08 | Text merely mentioning "password" without assigning one passes through byte-identical | ✓ VERIFIED | `TestRedactError_LeavesNonDSNTextAlone` PASS for both a Postgres auth-failure message and a mid-sentence mention |
| 58 | 02-08 | Both redaction helpers are pinned against one shared list of DSN forms | ✓ VERIFIED | `TestRedactDSN_NeverEchoesPassword` iterates the same `dsnFixtures` table as `TestRedactError_NeverEchoesPassword`, PASS |
| 59 | 02-08 | The dial-failure-only migration test says so in its name/comment | ✓ VERIFIED | `TestRunMigrations_NeverLogsDSN_KeywordValueForm_DialFailurePath` present in `migrate_test.go:278`; old unrenamed name absent (`grep -c '_KeywordValueForm('` == 0 in non-comment lines) |
| 60 | 02-08 | Every fixture password is the project's existing non-entropic marker, so `gitleaks` reports no new finding | ✓ VERIFIED | `python -m pre_commit run gitleaks --all-files` → "Passed" (re-run directly against current HEAD) |
| 61 | 02-08 | No Go module added/removed/upgraded | ✓ VERIFIED | `git diff --exit-code -- go.mod go.sum` clean |

**Score:** 61/61 declared must-have truths verified (43 carried forward + 18 from 02-07/02-08). 0 truths left behaviorally unverified.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/watchlist/service.go` | `ErrNoPreferencesSupplied` sentinel + domain-boundary guard | ✓ VERIFIED | Sentinel at line 45, guard at lines 231-233, first statement of `UpdatePreferences` |
| `internal/watchlist/service_test.go` | Real-Postgres proof of no-op guard and its precedence over `ErrNotFound` | ✓ VERIFIED | `TestService_UpdatePreferences_NeitherAxisReturnsErrNoPreferencesSupplied`, `TestService_UpdatePreferences_NeitherAxisOutranksUnknownID` present and passing |
| `internal/httpserver/watchlist.go` | One shared `decodeJSONBody` path, sentinel translated to 400 | ✓ VERIFIED | Helper at lines 67-77; used by both `handleAddWatchlist` (line 120) and `handleUpdateWatchlist` (line 253); error switch case at line 263 |
| `internal/httpserver/watchlist_test.go` | Handler-level 400 translation + table-driven trailing-value rejection | ✓ VERIFIED | `TestWatchlist_Patch_NoPreferencesSuppliedReturns400`, `TestWatchlist_Patch_EmptyBodyStillRejectedEndToEnd`, `TestWatchlist_Add_BodyMustContainExactlyOneJSONValue`, `TestWatchlist_Patch_BodyMustContainExactlyOneJSONValue` all present and passing |
| `internal/db/migrate.go` | `kvPasswordPattern` applied in `redactError` alongside the existing userinfo strip | ✓ VERIFIED | Pattern at line 110, applied at line 149; doc comments record coverage and the non-reliance on pgx's incidental self-redaction |
| `internal/db/redact_test.go` | In-package unit coverage of `redactError`/`redactDSN` against a shared `dsnFixtures` table | ✓ VERIFIED | File exists, `package db`, 7-entry `dsnFixtures` table, 4 test functions all present and passing |
| `internal/db/migrate_test.go` | Honestly named/documented dial-failure test | ✓ VERIFIED | `TestRunMigrations_NeverLogsDSN_KeywordValueForm_DialFailurePath` present; old name absent |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/watchlist/service.go` | `internal/httpserver/watchlist.go` | `errors.Is` against `ErrNoPreferencesSupplied` is the handler's only remaining route to 400 | ✓ WIRED | Handler's error switch at line 263; handler's own duplicate two-condition check confirmed deleted (grep count == 1, the switch case) |
| `internal/httpserver/watchlist.go decodeJSONBody` | `handleAddWatchlist` and `handleUpdateWatchlist` | Both decode sites call the one helper | ✓ WIRED | 3 non-comment occurrences (1 declaration, 2 call sites) confirmed by grep |
| `internal/db/redact_test.go dsnFixtures` | `redactError` and `redactDSN` | One shared table drives both helpers' tests | ✓ WIRED | `TestRedactError_NeverEchoesPassword` and `TestRedactDSN_NeverEchoesPassword` both range over `dsnFixtures`, confirmed by direct source read |
| `internal/db/migrate.go redactError` | `RunMigrations`' retry `Warn` line and returned error | `redactError` is the only scrubbing applied to underlying error text on both paths | ✓ WIRED | `RunMigrations` (lines 179, 196, 205) calls `redactError` once per failed attempt and reuses `lastErrMsg` for both the retry log line and the final wrapped error |

### Behavioral Spot-Checks / Direct Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full build | `go build ./...` | exit 0 | ✓ PASS |
| Static analysis | `go vet ./...` | exit 0, no output | ✓ PASS |
| Full test suite (real Postgres) | `TEST_DATABASE_URL=... go test ./... -count=1` | `internal/config`, `internal/db`, `internal/httpserver`, `internal/watchlist` all `ok`, 0 fail | ✓ PASS |
| 02-07 named tests | `go test ./internal/watchlist/... -run 'TestService_UpdatePreferences_NeitherAxis...'` and `./internal/httpserver/... -run 'TestWatchlist_Add_BodyMustContainExactlyOneJSONValue\|TestWatchlist_Patch_BodyMustContainExactlyOneJSONValue\|TestWatchlist_Patch_NoPreferencesSuppliedReturns400'` | all subtests PASS | ✓ PASS |
| 02-08 named tests | `go test ./internal/db/... -run 'TestRedactError_NeverEchoesPassword\|TestRedactError_KeepsDiagnosticContext\|TestRedactError_LeavesNonDSNTextAlone\|TestRedactDSN_NeverEchoesPassword'` | all subtests PASS | ✓ PASS |
| Structural gates re-run | `grep -c 'ErrNoPreferencesSupplied'`==1, `grep -c 'decodeJSONBody('`==3, `grep -c 'json.NewDecoder('`==1, `grep -c 'kvPasswordPattern'`==2 | all match plan's own acceptance criteria | ✓ PASS |
| gitleaks pre-commit | `python -m pre_commit run gitleaks --all-files` | "Detect hardcoded secrets.....Passed" | ✓ PASS |
| No dependency drift | `git diff --exit-code -- go.mod go.sum` | exit 0, clean | ✓ PASS |
| No debt markers in phase files | `grep -nE 'TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER'` across `internal/watchlist/`, `internal/httpserver/`, `internal/db/migrate.go`, `internal/db/migrate_test.go`, `internal/db/redact_test.go` | no matches | ✓ PASS |
| Plan-scope independence | `git diff --stat <02-08 range> -- internal/watchlist/ internal/httpserver/ queries/ internal/db/sqlc/` | empty diff | ✓ PASS — confirms 02-08 touched only `internal/db/` |
| Working tree clean at HEAD | `git status --porcelain` | empty | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WLST-02 | 02-01, 02-02, 02-05, 02-07 | User can add an artist to the watchlist from search results | ✓ SATISFIED | `POST /watchlist` implemented, tested; re-add metadata refresh correct; trailing-JSON smuggling now rejected |
| WLST-03 | 02-03 | User can remove an artist from the watchlist | ✓ SATISFIED | `DELETE /watchlist/{id}` hard delete, 404/400 branches, concurrency-safe |
| WLST-04 | 02-03 | User can list all artists currently on the watchlist | ✓ SATISFIED | `GET /watchlist` joined, ordered, `[]`-safe |
| WLST-05 | 02-02, 02-04, 02-06, 02-07 | User can set per-artist release-type filters | ✓ SATISFIED | Set on add and via `PATCH`; PATCH concurrency-safe, 404-honest, and now rejects a neither-axis call at the domain boundary |
| WLST-06 | 02-02, 02-04, 02-06, 02-07 | User can set per-artist notification/mute preferences | ✓ SATISFIED | Same as WLST-05, mute axis |
| OPS-02 | 02-08 | Structured (JSON) logs with request-ID correlation | ✓ SATISFIED (phase-scoped) | 02-08 hardens the one credential-scrubbing guarantee those logs rely on (`redactError`) — no regression to the underlying `httplog`/`slog` wiring from Phase 1 |
| OPS-03 | 02-08 | All secrets/configuration supplied via environment variables only; none committed | ✓ SATISFIED (phase-scoped) | `redactError` now meets its own stated contract for every DSN form `config.Config.DatabaseURL` accepts, proven at the unit level; `gitleaks` clean |

`.planning/REQUIREMENTS.md` traceability table maps WLST-02 through WLST-06 to Phase 2, all `[x]`/Complete (lines 13-17, 109-113). OPS-02/OPS-03 are mapped to Phase 1 as their primary phase but are legitimately re-touched here since 02-08 hardens a Phase-1-owned guarantee those requirements state; this does not change their traceability-table phase assignment, which is correct as-is. No orphaned requirements — every ID declared across all 8 plans (`WLST-02` through `WLST-06`, `OPS-02`, `OPS-03`) appears in at least one plan's `requirements:` frontmatter.

### Anti-Patterns Found

None in phase-modified files. Re-scanned `internal/watchlist/`, `internal/httpserver/`, `internal/db/migrate.go`, `internal/db/migrate_test.go`, `internal/db/redact_test.go` for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` and common stub phrases — zero matches.

### Code Review Findings — Out of Scope for This Pass

The current `02-REVIEW.md` (commit `6687486`, generated after 02-07/02-08 landed) reports **0 Critical, 5 Warning, 2 Info**. Both previously-open findings are confirmed resolved by this verification's own direct re-inspection (old WR-01/WR-02 from the pre-gap-closure review, and CR-01) — see the Observable Truths table above.

Per this verification's task instructions, the 5 new Warning and 2 new Info findings in the current review were **not claimed as fixed by any plan in this phase** and are recorded here as known, out-of-scope issues rather than routed to human verification in this pass:

1. **WR-01 (backoff shift-to-zero for `maxAttempts >= 65`)** — unreachable at the current production call site (`DefaultMaxAttempts = 6`); exported `WithMaxAttempts` is unvalidated. Not tied to any Phase 2 must-have.
2. **WR-02 (`RetryOption`s unvalidated, malformed error message for `maxAttempts <= 0`)** — same unreachability caveat. Not tied to any Phase 2 must-have.
3. **WR-03 (`kvPasswordPattern`'s unquoted branch over-consumes trailing query-string parameters)** — confirmed real by inspection: for `?password=x&sslmode=disable`, the redacted output would also swallow `&sslmode=disable`. This is the safe direction for the "never leak a secret" guarantee (no fixture in `dsnFixtures` currently pins trailing-parameter survival for this specific form), but is a residual robustness gap in code 02-08 introduced. Not tied to any declared must-have truth as literally written — none of 02-08's must-haves specify that *subsequent* query parameters after the password must survive, only that the password itself is redacted and that "the underlying failure text" broadly survives — but it is real and worth a human accept-or-fix call.
4. **WR-04 (`AddParams` vs `PreferencesParams` nil-semantics inconsistency)** — pre-existing design note, not touched by 02-07/02-08.
5. **WR-05 (no auth on watchlist-mutating routes)** — pre-existing scope note, not touched by 02-07/02-08.
6. **IN-01, IN-02** — naming/consistency notes, pre-existing.

These are carried forward as known issues for a future review/plan cycle, consistent with the task's explicit scoping instruction for this verification pass.

### Human Verification Required

None for this pass. The two items that were open after the prior verification cycle (disposition of old WR-01/WR-02, and CR-01) were resolved via `02-UAT.md`'s recorded "fix" decisions and closed by plans 02-07 and 02-08, both independently confirmed above. The 6 new Warning/Info findings from the post-02-07/02-08 review are, per this verification's task scope, recorded as known out-of-scope issues rather than new human-verification items.

### Gaps Summary

No gaps. Both gaps carried into this pass — G-02-1 (WR-01: domain-boundary no-op guard; WR-02: trailing-JSON-value rejection) and G-02-2 (CR-01: keyword/value-form DSN password redaction) — are closed, each proven by tests that were red before the fix and green after (per both SUMMARY.md's TDD commit sequences, independently re-run by name in this verification rather than trusted from the summaries). All 61 must-have truths across all 8 plans are independently confirmed against the current codebase: a full suite run against real Postgres (0 fail), all newly-added named tests re-run individually, `go build`/`go vet` clean, every plan's own grep-based structural gate re-executed directly, `gitleaks` clean, and no dependency drift. No stub, no orphaned requirement, no debt marker, and no regression in any of the 43 previously-verified truths.

The phase goal — "Users can fully manage their watchlist — add, remove, list, and configure per-artist alert preferences — through a tested API service layer" — is achieved and now additionally hardened at the domain boundary and in its credential-redaction guarantee.

---

_Verified: 2026-08-06T21:00:00Z_
_Verifier: Claude (gsd-verifier)_
