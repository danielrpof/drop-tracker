---
phase: 02-watchlist-core
reviewed: 2026-08-06T00:00:00Z
depth: standard
files_reviewed: 31
files_reviewed_list:
  - .env.example
  - .gitignore
  - cmd/server/main.go
  - go.mod
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/db/migrate.go
  - internal/db/migrate_test.go
  - internal/db/migrations/000002_watchlist.down.sql
  - internal/db/migrations/000002_watchlist.up.sql
  - internal/db/sqlc/artists.sql.go
  - internal/db/sqlc/db.go
  - internal/db/sqlc/health.sql.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc/querier.go
  - internal/db/sqlc/watchlist.sql.go
  - internal/db/sqlc_test.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/health_test.go
  - internal/httpserver/server.go
  - internal/httpserver/server_test.go
  - internal/httpserver/watchlist.go
  - internal/httpserver/watchlist_test.go
  - internal/watchlist/normalize_test.go
  - internal/watchlist/service.go
  - internal/watchlist/service_test.go
  - Makefile
  - queries/artists.sql
  - queries/health.sql
  - queries/watchlist.sql
  - sqlc.yaml
findings:
  critical: 1
  warning: 2
  info: 2
  total: 5
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-08-06T00:00:00Z
**Depth:** standard
**Files Reviewed:** 31
**Status:** issues_found

## Summary

This is a fresh full re-review of Phase 02 (watchlist-core) covering the entire current diff, including the two gap-closure plans (02-05/02-06) that landed after the previously-committed 02-REVIEW.md. That prior review's two Warnings — `UpsertArtist` silently dropping `disambiguation`/`image_url` on re-add, and `UpdatePreferences`'s unhandled not-found race / lost-update race under concurrent PATCH — are both independently confirmed fixed in this pass: `queries/artists.sql`/`internal/db/sqlc/artists.sql.go` now `COALESCE`s all three nullable metadata columns, and `UpdateWatchlistPreferences` is now a single CASE-based statement whose lost-update and not-found races are proven closed by deterministic held-lock tests (`TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost`, `TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound`).

The implementation is careful and well-tested overall: duplicate-add handling is precise (matches on SQLSTATE + constraint name, never error text), the D-08/D-11 default/empty-vs-nil preference semantics are correct and thoroughly exercised, and the HTTP layer consistently guards against over-posting, oversized bodies, and out-of-allow-list values. This pass's one Critical finding is in `internal/db/migrate.go`'s DSN-redaction logic: the regex used to scrub secrets out of migration error text only recognizes URL-form DSNs, not the keyword/value form that `config.Config.DatabaseURL` explicitly permits and that this project's own tests exist specifically to guard against. Two Warning-level and two Info-level quality/consistency issues are also documented below.

Note: `.env.example` could not be opened directly by this review's tooling (both the `Read` tool and a `Grep`/`Bash` fallback returned "denied by your permission settings" for this specific path); its shape is nonetheless indirectly verified by `internal/config/config_test.go`'s `TestEnvExampleCompleteness` and `TestDotEnvIsNotTracked`, so no finding is recorded against it.

## Critical Issues

### CR-01: DSN-redaction regex does not cover keyword/value-form connection strings, leaving a secret-leak gap in migration error logging

**File:** `internal/db/migrate.go:77-109` (used at `internal/db/migrate.go:138`, `RunMigrations`'s retry-warning log line, and in the final returned error)

**Issue:**
`redactError` exists specifically to guarantee that "any error text that happens to embed a raw DSN verbatim can still be scrubbed before it reaches a log line or a returned error" (its own doc comment), because `RunMigrations` logs `redactError(err)` on every failed attempt (`logger.Warn(..., slog.String("error", lastErrMsg))`) and embeds the same text in the error it ultimately returns.

The only pattern it strips is:

```go
var userInfoPattern = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^/@\s]*@`)

func redactError(err error) string {
	return userInfoPattern.ReplaceAllString(err.Error(), "")
}
```

This matches only the URL form of a DSN (`scheme://user:pass@host/...`). `internal/config/config.go`'s `DatabaseURL` field has no format constraint, and the sibling `redactDSN` function (used to compute the safe `target` string logged alongside each retry) explicitly delegates to `pgconn.ParseConfig` — not a hand-rolled parser — precisely because, per its own doc comment, a libpq keyword/value-form DSN (`host=... user=... password=... dbname=...`) is a legitimate, accepted input: this project's own regression test (`internal/db/migrate_test.go`'s `TestRunMigrations_NeverLogsDSN_KeywordValueForm`) exists specifically to guard against "the CR-01 regression class" from a prior review round, where a hand-rolled parser silently echoed this DSN form back verbatim.

`redactError`, however, was never given the same treatment: its regex requires a `scheme://...@` prefix and will not match or redact any `password=...`-style segment. If a raw keyword/value DSN (or a fragment of one — e.g. from a `pgconn.ParseConfig`/`sql.Open` failure on a malformed `DATABASE_URL`, or from any other dependency's error wrapping that happens to echo the connection string) ever appears verbatim inside `err.Error()`, the real Postgres password would pass through `redactError` untouched and land in the retry-warning log line and/or the final wrapped error returned by `RunMigrations` — directly contradicting this project's explicit secrets-handling requirement (CLAUDE.md: "all secrets via environment variables only... nothing real ever committed; gitleaks enforced") and this function's own stated contract.

Critically, the existing `TestRunMigrations_NeverLogsDSN_KeywordValueForm` regression test does **not** actually exercise the vulnerable path: it uses `closedPortKeywordValueDSN`, which produces a TCP *connection-refused* error (`dial tcp ...: connect: connection refused`). A dial-refused error never contains the DSN at all, so this test passes identically whether or not `redactError` handles the keyword/value form — it only proves the `redactDSN`-computed `target` string is safe, not that `redactError` is. The actual risk path — a malformed/unparseable `DATABASE_URL` (e.g. an operator typo) causing `sql.Open("pgx", dsn)` or the underlying config parse to fail with an error that embeds the raw connection string — is untested and, based on the regex alone, unprotected.

**Fix:**
Extend `redactError` to also scrub keyword/value-style credentials, not just URL-form userinfo, e.g.:

```go
var userInfoPattern = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^/@\s]*@`)
var kvPasswordPattern = regexp.MustCompile(`(?i)\bpassword=\S+`)

func redactError(err error) string {
	msg := userInfoPattern.ReplaceAllString(err.Error(), "")
	msg = kvPasswordPattern.ReplaceAllString(msg, "password=<redacted>")
	return msg
}
```

And add a regression test that actually forces a DSN-parse failure (e.g. `RunMigrations(ctx, "host=127.0.0.1 password=VerySecretPassw0rd dbname=x ==invalid==", ...)`) and asserts the password never appears in the captured log/error output — the current keyword/value test should be renamed or supplemented so it's clear which failure mode (dial vs. parse) each test actually covers.

## Warnings

### WR-01: `Service.UpdatePreferences` has no independent guard against a no-op call; only the HTTP handler enforces "at least one axis supplied"

**File:** `internal/watchlist/service.go:216-241`

**Issue:** The WLST-05/06 contract ("a body that supplies neither key is rejected") is enforced only in `internal/httpserver/watchlist.go`'s `handleUpdateWatchlist` (`if req.ReleaseTypes == nil && req.MutedEventTypes == nil { writeError(..., "no preferences supplied") }`). `watchlist.Service.UpdatePreferences` itself performs no equivalent check: called directly with `PreferencesParams{}` (both fields nil — something any non-HTTP caller, such as a future admin tool, the Phase 3+ scheduler, or a test, is free to do), it still issues the full `UPDATE ... SET updated_at = now() WHERE id = $5` round trip and returns success, silently bumping `updated_at` with no field actually changed. This is a real gap between the documented domain-level contract and what the domain layer actually guarantees; `watchlist.Store`/`Service` is explicitly the reusable API surface later phases are told to build on, so the HTTP layer's check alone is not a durable substitute for validating the invariant at the domain boundary.

**Fix:** Add the same guard inside `Service.UpdatePreferences`, before any database call, e.g.:

```go
if p.ReleaseTypes == nil && p.MutedEventTypes == nil {
	return Entry{}, errors.New("no preferences supplied")
}
```

(or a dedicated sentinel error consistent with `ErrDuplicate`/`ErrNotFound`/`ErrInvalidReleaseType`, so callers can `errors.Is` against it), so the invariant holds regardless of caller.

### WR-02: `POST /watchlist` and `PATCH /watchlist/{id}` decoders silently accept trailing data after a valid JSON body

**File:** `internal/httpserver/watchlist.go:90-96` (`handleAddWatchlist`) and `internal/httpserver/watchlist.go:223-229` (`handleUpdateWatchlist`)

**Issue:** Both handlers do:

```go
dec := json.NewDecoder(r.Body)
dec.DisallowUnknownFields()
if err := dec.Decode(&req); err != nil { ... }
```

`json.Decoder.Decode` only reads a single JSON value off the stream; it does not verify the stream is exhausted afterward. A request body like `{"mbid":"x","name":"y"}{"anything":"else, even garbage"}` decodes successfully (the second value is simply never read) and is processed as a normal, valid request. This weakens the otherwise-deliberate strict-validation posture applied everywhere else in this file (`DisallowUnknownFields`, `MaxBytesReader`, rune-count limits on `mbid`/`name`) and is a well-known Go `encoding/json` footgun. There's no immediate exploit path today (nothing downstream re-parses the discarded bytes), but it's inconsistent with the file's stated intent of rejecting anything unexpected in the request body, and a missing-input-validation gap is worth closing rather than carrying forward.

**Fix:** After decoding, assert the stream is exhausted:

```go
if err := dec.Decode(&struct{}{}); err != io.EOF {
	writeError(w, http.StatusBadRequest, "invalid request body")
	return
}
```

## Info

### IN-01: Redundant double panic recovery in the middleware stack

**File:** `internal/httpserver/server.go:52-63`

**Issue:** `httplog.RequestLogger` is configured with `RecoverPanics: true` (line 55), and `middleware.Recoverer` is also registered as the innermost middleware (line 63). Both are capable of converting a handler panic into a logged 500 response; as wired, `middleware.Recoverer` sits closer to the handler and will catch the panic first in practice, making `httplog`'s own `RecoverPanics: true` option dead weight for this specific stack ordering. Not a bug, but worth consolidating to one recovery layer so it's unambiguous which one is actually responsible when debugging a panic-handling issue later.

**Fix:** Either drop `RecoverPanics: true` from the `httplog.Options` (since `middleware.Recoverer` already covers it) or drop `middleware.Recoverer` and rely on `httplog`'s built-in recovery — pick one and note why in a comment.

### IN-02: `handleUpdateWatchlist` doesn't apply the same fail-fast allow-list pre-check that `handleAddWatchlist` does

**File:** `internal/httpserver/watchlist.go:116-137` (present, `handleAddWatchlist`) vs. `internal/httpserver/watchlist.go:212-258` (absent, `handleUpdateWatchlist`)

**Issue:** `handleAddWatchlist` explicitly pre-validates `release_types`/`muted_event_types` membership against `watchlist.ReleaseTypes`/`watchlist.EventTypes` before ever calling the store, documented as a deliberate "fail-fast... non-bypassable backstop" optimization (T-02-13). `handleUpdateWatchlist` has no equivalent pre-check and goes straight from body-decode to calling `s.watchlist.UpdatePreferences`. Functionally this is not a defect — `Service.UpdatePreferences` still validates via `normalizeSet` before any database call either way, and `TestWatchlist_Patch_InvalidValueReturns400` confirms the 400 is still produced — but it's an inconsistent pattern between two structurally similar handlers in the same file, which makes the codebase harder to reason about ("why does one route pre-check and the other doesn't?").

**Fix:** Either add the same membership pre-check loop to `handleUpdateWatchlist` for consistency, or remove it from `handleAddWatchlist` and rely uniformly on the service-layer validation (simpler, one less place to keep in sync with `watchlist.ReleaseTypes`/`EventTypes`).

---

_Reviewed: 2026-08-06T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
