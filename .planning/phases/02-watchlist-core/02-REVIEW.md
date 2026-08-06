---
phase: 02-watchlist-core
reviewed: 2026-08-06T00:00:00Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - cmd/server/main.go
  - go.mod
  - internal/db/migrate.go
  - internal/db/migrate_test.go
  - internal/db/migrations/000002_watchlist.down.sql
  - internal/db/migrations/000002_watchlist.up.sql
  - internal/db/redact_test.go
  - internal/db/sqlc/artists.sql.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc/querier.go
  - internal/db/sqlc/watchlist.sql.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/health_test.go
  - internal/httpserver/server.go
  - internal/httpserver/server_test.go
  - internal/httpserver/watchlist.go
  - internal/httpserver/watchlist_test.go
  - internal/watchlist/normalize_test.go
  - internal/watchlist/service.go
  - internal/watchlist/service_test.go
  - queries/artists.sql
  - queries/watchlist.sql
  - sqlc.yaml
findings:
  critical: 0
  warning: 5
  info: 2
  total: 7
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-08-06T00:00:00Z
**Depth:** standard
**Files Reviewed:** 23
**Status:** issues_found

## Summary

This review covers the file set named across all 8 plans of phase 02 (watchlist-core), including
the two gap-closure plans 02-07 (G-02-1: domain-boundary guard on `UpdatePreferences` +
shared JSON-decode path rejecting trailing values) and 02-08 (G-02-2: libpq keyword/value-form
DSN password redaction in `redactError`).

Both gap-closure items are confirmed fixed by direct code reading, not just by their own tests:
`watchlist.Service.UpdatePreferences` now rejects a neither-axis call with `ErrNoPreferencesSupplied`
before any database call (`internal/watchlist/service.go:231-233`), ahead of the id lookup
(`TestService_UpdatePreferences_NeitherAxisOutranksUnknownID` pins the ordering); and
`internal/db/migrate.go`'s `redactError`/`redactDSN` now handle both the URL-userinfo and the
libpq keyword/value password forms via `kvPasswordPattern`, with `redact_test.go`'s shared
`dsnFixtures` table pinning both helpers against the same coverage list so they cannot silently
diverge again. `decodeJSONBody` (`internal/httpserver/watchlist.go:67-77`) is confirmed to be the
single shared decode path for both `POST /watchlist` and `PATCH /watchlist/{id}`, and correctly
rejects a second JSON value concatenated after a well-formed body.

The rest of the domain and HTTP layer is careful and well-tested: duplicate-add handling matches
on SQLSTATE + constraint name (never error text), the D-08/D-11 default/empty-vs-nil preference
semantics are correct, `UpdateWatchlistPreferences`'s single-statement CASE rewrite genuinely
closes the lost-update and not-found races it claims to (verified against the SQL, not just the
tests), and the HTTP layer consistently guards against over-posting, oversized bodies, and
out-of-allow-list values.

No Critical/blocking defects were found in this pass. Five Warnings and two Info items are
recorded below — mostly latent robustness gaps in exported-but-currently-unexercised code paths
(retry backoff math, DSN redaction over-reach) and one API-shape inconsistency worth closing
before Phase 3/4 build non-HTTP callers on top of `watchlist.Store`.

## Warnings

### WR-01: Exponential backoff formula overflows to zero delay for large `maxAttempts`

**File:** `internal/db/migrate.go:188-191`
**Issue:** The backoff delay is computed as `cfg.baseDelay * time.Duration(uint64(1)<<uint(attempt-1))`. Per the Go spec, shifting a `uint64` by 64 or more yields `0`, not saturation. Once `attempt-1 >= 64` (i.e. `attempt >= 65`), the shift evaluates to `0`, so `delay` becomes `0` and the `if delay > cfg.maxDelay { delay = cfg.maxDelay }` clamp never triggers (`0` is never greater than `maxDelay`) — instead of staying capped at `maxDelay`, every subsequent retry fires with **zero** backoff, i.e. a retry storm against the database instead of the intended capped exponential backoff.

This is unreachable via the current production call site (`cmd/server/main.go` calls `db.RunMigrations` with no options, so `DefaultMaxAttempts = 6` always applies), but `WithMaxAttempts` is exported specifically so callers can override it, and nothing validates or bounds the value a caller supplies. A future caller (or a config-driven override) passing `maxAttempts >= 65` silently loses the backoff cap.

**Fix:**
```go
shift := attempt - 1
if shift > 62 { // headroom below 64 to avoid the uint64 shift-to-zero case entirely
    shift = 62
}
delay := cfg.baseDelay * time.Duration(uint64(1)<<uint(shift))
if delay > cfg.maxDelay || delay <= 0 {
    delay = cfg.maxDelay
}
```
Also consider validating `maxAttempts > 0` in `newRetryConfig` (see WR-02) rather than silently accepting `0`/negative values.

### WR-02: `RetryOption` values are unvalidated, producing a malformed final error for `maxAttempts <= 0`

**File:** `internal/db/migrate.go:65-75, 205`
**Issue:** `newRetryConfig` applies `RetryOption`s with no bounds checking. If a caller supplies `WithMaxAttempts(0)` (or a negative value), the retry loop (`for attempt := 1; attempt <= cfg.maxAttempts; attempt++`) never executes, so `lastErrMsg` is never assigned and keeps its zero value (`""`). Execution falls straight through to:
```go
return fmt.Errorf("migrations failed after %d attempts against %s: %s", cfg.maxAttempts, target, lastErrMsg)
```
producing a message like `migrations failed after 0 attempts against host=...: ` with a trailing empty reason and no indication that migrations were never actually attempted. Not reachable today (no call site overrides `maxAttempts` below the default of 6) but exported, undocumented-as-unsafe, and easy to hit by a future caller.
**Fix:** Validate in `newRetryConfig` (or in `WithMaxAttempts`) and clamp/error on `maxAttempts < 1`, e.g. `if cfg.maxAttempts < 1 { cfg.maxAttempts = 1 }`.

### WR-03: `kvPasswordPattern`'s unquoted branch over-consumes trailing query-string parameters

**File:** `internal/db/migrate.go:110` (pattern), exercised in `internal/db/redact_test.go:41-44`
**Issue:** For a DSN carrying the password as a URL query parameter followed by further parameters — e.g. `postgres://127.0.0.1:5432/drop_tracker?password=local-test-fixture-password&sslmode=disable` — `userInfoPattern` does not match (no `@`), so the whole string falls to `kvPasswordPattern`'s unquoted alternative, `\S+`. Since `&` is not whitespace, `\S+` greedily consumes `local-test-fixture-password&sslmode=disable` as one match, and the replacement `password=<redacted>` silently swallows `&sslmode=disable` along with the password.

This is the safe direction for the "never leak a secret" test (`TestRedactError_NeverEchoesPassword`/`TestRedactDSN_NeverEchoesPassword` still pass, since the password substring really is gone), but it violates the redaction design's explicit second goal — "scrubbing by destroying the whole message would ... make a failed migration ... undiagnosable ... a different bug, equally real" (see the doc comment above `redactError`, and `TestRedactError_KeepsDiagnosticContext`). No test asserts that trailing, non-secret query parameters survive redaction for the URL-query-parameter password form specifically, so this regression is currently invisible to the suite.
**Fix:** Give the unquoted alternative a delimiter-aware character class instead of blanket `\S+`, e.g. `[^\s&'"]+`, so it stops at `&` (and other DSN-relevant delimiters) as well as whitespace. Add a fixture asserting `sslmode=disable` (or similar trailing content) survives redaction for this DSN form.

### WR-04: `AddParams`' preference fields use a different nil-semantics convention than `PreferencesParams`, inviting a silent default-flip for non-HTTP callers

**File:** `internal/watchlist/service.go:66-74` (`AddParams`) vs `78-81` (`PreferencesParams`)
**Issue:** `PreferencesParams` deliberately uses `*[]string` so "untouched" (nil pointer) and "explicitly empty" (non-nil pointer to an empty slice) can never be confused by construction. `AddParams.ReleaseTypes`/`MutedEventTypes`, which encode the exact same "apply D-08 defaults" vs "use exactly this set" distinction (`Service.Add`'s `if p.ReleaseTypes == nil` check at `service.go:117`), are instead plain `[]string` — where a Go nil slice and a `[]string{}`/`make([]string, 0)` empty slice are semantically distinct but trivially conflated by ordinary construction idioms, with no compiler signal either way.

`internal/httpserver/watchlist.go` gets this right today only because it manually reconstructs the nil-vs-non-nil distinction from a `*[]string` request DTO field before populating `AddParams` (lines 173-178). `Store` is explicitly documented (`service.go:83-86`) as the reusable surface later phases (the poller, search proxy) will call directly, not just through HTTP — a non-HTTP caller constructing `AddParams{ReleaseTypes: []string{}}` intending "no preference specified" would silently get "watch nothing" instead of the D-08 "watch everything" default, with no error and no test coverage to catch it (no existing test constructs `AddParams` with a non-nil-but-empty `ReleaseTypes`).
**Fix:** Change `AddParams.ReleaseTypes`/`MutedEventTypes` to `*[]string`, matching `PreferencesParams`'s already-correct pattern, and update `Service.Add`'s nil-check accordingly.

### WR-05: No authentication/authorization on any watchlist-mutating route

**File:** `internal/httpserver/server.go:65-69`
**Issue:** `POST /watchlist`, `PATCH /watchlist/{id}`, and `DELETE /watchlist/{id}` are registered with no auth middleware, API key check, or other access control anywhere in the reviewed files — any caller able to reach the process can add, mute, or delete watchlist entries. This may be an intentional, documented scope decision for a later phase (nothing in `CLAUDE.md`'s Phase-02-relevant constraints mandates auth here), but it is a real, current-state gap in a data-mutating API and is called out explicitly since "authorization gaps" is an in-scope security check for this review. Recommend either confirming this is deferred to a tracked later phase, or adding a minimal gate (e.g. a shared-secret header) before this is exposed beyond localhost/trusted network.

## Info

### IN-01: `maxAddWatchlistBodyBytes` constant name no longer matches its usage

**File:** `internal/httpserver/watchlist.go:24, 117, 250`
**Issue:** The constant is named for the `POST /watchlist` (add) path but is also applied verbatim to `PATCH /watchlist/{id}` (`handleUpdateWatchlist`, line 250). The name is misleading to a reader auditing only the PATCH handler.
**Fix:** Rename to `maxWatchlistBodyBytes` (or similar) to reflect that it's shared across both mutating routes.

### IN-02: Optional artist metadata fields (`deezer_id`, `disambiguation`, `image_url`) are neither trimmed nor length-bounded, and cannot be explicitly cleared

**File:** `internal/httpserver/watchlist.go:98-106, 166-178`; `queries/artists.sql:12-20`
**Issue:** Unlike `mbid`/`name`, these three optional fields are passed through to `AddParams` with no `strings.TrimSpace` and no rune-count ceiling (only the overall 65536-byte body cap applies). Separately, because `UpsertArtist`'s `ON CONFLICT` clause uses `COALESCE(EXCLUDED.<col>, artists.<col>)` for all three, and JSON `null` is indistinguishable from an omitted key once decoded into a `*string` (both produce a nil pointer), there is currently no way for a client to explicitly blank a previously-set `disambiguation`/`image_url`/`deezer_id` via this API — only to leave it unset on first add or overwrite it with a new non-empty value. This appears to be an intentional design choice (extensively documented across `queries/artists.sql`, `artists.sql.go`, and `service_test.go`'s `TestService_Add_OmittedMetadataSurvivesReAdd`), so it is recorded here as a product-completeness note rather than a defect: if a future UI needs to clear one of these fields, the current contract has no way to express that intent.

---

_Reviewed: 2026-08-06T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
