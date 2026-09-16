---
phase: 01-foundation-data-layer-config-health
reviewed: 2026-08-05T17:55:09Z
depth: standard
files_reviewed: 24
files_reviewed_list:
  - .env.example
  - .gitignore
  - cmd/server/main.go
  - docker-compose.yml
  - go.mod
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/db/migrate.go
  - internal/db/migrate_test.go
  - internal/db/migrations/000001_init.down.sql
  - internal/db/migrations/000001_init.up.sql
  - internal/db/pool.go
  - internal/db/sqlc/db.go
  - internal/db/sqlc/health.sql.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc_test.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/health.go
  - internal/httpserver/health_test.go
  - internal/httpserver/server.go
  - internal/httpserver/server_test.go
  - internal/logging/logging.go
  - internal/testutil/postgres.go
  - Makefile
  - queries/health.sql
  - sqlc.yaml
findings:
  critical: 1
  warning: 4
  info: 4
  total: 9
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-08-05T17:55:09Z
**Depth:** standard
**Files Reviewed:** 24
**Status:** issues_found

## Summary

This phase implements config loading, migration/pool bootstrapping, and a `/health` endpoint, with an evident and deliberate focus on never leaking the Postgres DSN (password) into logs or responses — `internal/db/migrate.go` and `internal/db/pool.go` both carry doc comments to that effect, and there are dedicated tests (`TestRunMigrations_NeverLogsDSN`, `TestNoDSNInLogs`) proving it for URL-style DSNs.

That focus has a real gap: `internal/db/migrate.go`'s `redactDSN` only handles `scheme://user:pass@host/db` connection strings. When `DATABASE_URL` is supplied in libpq's keyword/value form (`host=... user=... password=... dbname=...` — a form pgx and `golang-migrate` both accept, and `config.go` places no format constraint on the value), `redactDSN` does not fail safe: it silently falls through to embedding the **entire raw DSN, password included**, into the string used for retry-warning log lines and the final returned error. This is confirmed by direct reproduction (see CR-01) and is not covered by any existing test — every DSN-redaction test in the suite uses the URL form only. `internal/db/pool.go`'s equivalent `redactedTarget` does not have this flaw, because it delegates to `pgxpool.ParseConfig` instead of hand-rolling a `url.Parse`-based parser — the inconsistency between the two files is itself informative: one file solved this correctly, the other didn't.

Beyond that blocker, the remaining findings are quality/hardening gaps typical of a "foundation" phase: no context propagation into the actual migration attempt (only the inter-retry sleep is cancellable), no HTTP server timeouts or graceful shutdown wiring in `cmd/server/main.go`, and a couple of minor documentation/config hygiene items (`.gitignore` copied wholesale from a Python template, a mis-encoded character in `.env.example`, case-sensitive log-level parsing).

## Critical Issues

### CR-01: `redactDSN` leaks the raw password when DATABASE_URL uses libpq keyword/value form

**File:** `internal/db/migrate.go:85-98`
**Issue:**

```go
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "database=<unparseable>"
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return fmt.Sprintf("host=%s", u.Host)
	}
	return fmt.Sprintf("host=%s database=%s", u.Host, dbName)
}
```

`redactDSN` is the *only* place `RunMigrations` derives the string it is allowed to log or return (`target := redactDSN(dsn)`, used in the per-attempt `logger.Warn(...)` call and in the final `fmt.Errorf("migrations failed after %d attempts against %s: %s", ...)`). It assumes `dsn` is always a URL (`postgres://user:pass@host/db`). pgx (and `golang-migrate`'s pgx driver) also accepts the libpq keyword/value form, e.g.:

```
host=localhost port=5432 user=drop_tracker password=Secret123 dbname=drop_tracker sslmode=disable
```

`config.go`'s `DATABASE_URL` field has no format validation (`env:"DATABASE_URL,notEmpty"` only), so this is a value an operator can legitimately set. Reproduced directly:

```go
redactDSN("host=localhost port=5432 user=drop_tracker password=Secret123 dbname=drop_tracker sslmode=disable")
// => "host= database=host=localhost port=5432 user=drop_tracker password=Secret123 dbname=drop_tracker sslmode=disable"
```

`url.Parse` on a string with no `scheme://` treats the whole input as an opaque path, so `u.Path` becomes the entire connection string and it gets echoed back verbatim, password included, under the misleading label `database=...`. This is worse than doing nothing: it looks redacted (`host=... database=...`) while actually containing the full credential. Every retry-warning log line and the final returned error (which `cmd/server/main.go` prints to stderr via `fmt.Fprintln(os.Stderr, err)`) would carry the plaintext password whenever a keyword/value DSN fails to connect (e.g. during exactly the container-cold-start retry window this function exists to handle).

No test in `migrate_test.go` exercises a keyword/value-form DSN — every test (`closedPortDSN`, `TestRunMigrations_NeverLogsDSN`) uses the URL form, so this regressed silently past the existing DSN-redaction test suite. Note `internal/db/pool.go`'s `redactedTarget` does not share this bug because it uses `pgxpool.ParseConfig(dsn)` (which correctly understands both DSN forms) rather than `url.Parse`.

**Fix:** Reuse pgx's own config parser (which already redacts safely for both DSN forms — see `pgconn.ParseConfigError`/`ConnectError`) instead of hand-rolling one with `url.Parse`:

```go
import "github.com/jackc/pgx/v5/pgconn"

func redactDSN(dsn string) string {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return "database=<unparseable>"
	}
	return fmt.Sprintf("host=%s database=%s", cfg.Host, cfg.Database)
}
```

Add a test mirroring `TestRunMigrations_NeverLogsDSN` but with a keyword/value-form `closedPortDSN` variant, so this class of regression is caught going forward.

## Warnings

### WR-01: Migration attempt itself is not context-aware — only the inter-retry delay is

**File:** `internal/db/migrate.go:129-134, 164-186`
**Issue:** `RunMigrations`'s loop only checks `ctx.Done()` in the `select` guarding the sleep *between* attempts (lines 150-154). The actual work — `runMigrationsOnce(dsn, src)` at line 130 — takes no `context.Context` at all: `sql.Open`, `pgxmigrate.WithInstance`, and `m.Up()` run with no deadline or cancellation path. If the target database is reachable at the TCP level but never responds (a silently-dropped connection, a misbehaving proxy, a stuck advisory lock held by another instance), a single attempt can hang indefinitely and the passed-in `ctx` will never be consulted until (if ever) that attempt returns. `TestRunMigrations_HonoursContextCancellation` only proves cancellation is honoured while waiting *between* fast-failing (connection-refused) attempts — it does not cover a hang mid-attempt.
**Fix:** Thread `ctx` through to bound each attempt, e.g. run `m.Up()` in a goroutine and `select` on its completion vs. `ctx.Done()`, or use `sql.Open` + `sqlDB.PingContext(ctx)` first with a per-attempt timeout derived from `ctx`/a configurable attempt timeout.

### WR-02: No HTTP server timeouts configured

**File:** `cmd/server/main.go:54`
**Issue:** `http.ListenAndServe(addr, srv.Router())` constructs an `http.Server` with all zero-value timeouts (`ReadTimeout`, `ReadHeaderTimeout`, `WriteTimeout`, `IdleTimeout`). A client that opens a connection and sends headers/body slowly (or never) can hold a goroutine and connection open indefinitely — the classic Slowloris-style resource-exhaustion pattern, and specifically the case `go vet`'s `gosec` G112 rule exists to catch. Given this project's stated emphasis on CI/CD and security tooling (Trivy, gitleaks) as its core value proposition, an unconfigured `http.Server` on the primary listener is a notable gap.
**Fix:**
```go
srv := &http.Server{
	Addr:              addr,
	Handler:           s.Router(),
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       15 * time.Second,
	WriteTimeout:      15 * time.Second,
	IdleTimeout:       60 * time.Second,
}
if err := srv.ListenAndServe(); err != nil { ... }
```

### WR-03: No graceful shutdown — `defer pool.Close()` is effectively unreachable

**File:** `cmd/server/main.go:44-58`
**Issue:** `run()` blocks forever inside `http.ListenAndServe`, which only returns on a listener error. There is no `signal.NotifyContext`/`os/signal` handling anywhere in `main.go`, so an operator-issued SIGTERM/SIGINT (the normal way a container orchestrator stops this process) terminates the process immediately without Go ever running the deferred `pool.Close()` at line 48 or allowing in-flight requests to finish. The `defer` only fires on the rare path where `ListenAndServe` itself returns an error (e.g., port already bound).
**Fix:** Wire `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` and call `srv.Shutdown(ctx)` on cancellation before returning from `run()`, so the pool close and any future scheduler shutdown (Phase 3) have a real hook.

### WR-04: `.gitignore` is a near-verbatim Python project template, not this project's

**File:** `.gitignore:1-224`
**Issue:** The file is dominated (lines 1-219) by Python-ecosystem patterns (`__pycache__/`, Django, Flask, Scrapy, Jupyter, PyInstaller, Poetry, PDM, pixi, mypy/Pyre/pytype caches, `.streamlit/secrets.toml`, etc.) that have no relevance to a Go project — CLAUDE.md explicitly states "Tech stack: Go (not Python)". Only the final 4 lines (`# Go` section: `/bin/`, `*.exe`, `coverage.out`) are actually applicable. This is harmless in that it doesn't cause incorrect behavior, but it signals the file was copy-pasted wholesale from an unrelated template rather than curated, and it's missing some Go-idiomatic entries worth considering (e.g. `vendor/` if vendoring is ever adopted, `*.test` binaries from `go test -c`).
**Fix:** Trim to a Go-appropriate `.gitignore` (binary output, `*.test`, `coverage.out`, `.env`, editor directories), keeping only what's relevant.

## Info

### IN-01: `.env.example` contains a mis-encoded character

**File:** `.env.example:5`
**Issue:** The `LOG_FORMAT` inline comment (`# json|text — use text for local dev readability`) renders with a corrupted multi-byte sequence in place of the em dash (mojibake), indicating the file was saved/edited with an encoding mismatch at some point. Cosmetic, but suggests the file wasn't round-tripped through UTF-8-safe tooling.
**Fix:** Re-save `.env.example` as UTF-8 and replace the corrupted character with a plain ASCII separator (e.g. `-` or `--`) to avoid re-triggering the same issue on non-UTF-8-default systems (this repo is developed on Windows).

### IN-02: Log level parsing is case-sensitive with a silent fallback

**File:** `internal/logging/logging.go:43-54`
**Issue:** `parseLevel` matches only exact lowercase strings (`"debug"`, `"warn"`, `"error"`); anything else — including a plausible operator typo like `LOG_LEVEL=DEBUG` or `LOG_LEVEL=Warn` — silently resolves to `slog.LevelInfo` with no warning logged anywhere. This is consistent with the documented intent ("defaulting to Info for anything unrecognized rather than failing startup"), but a case-insensitive comparison would remove an easy-to-hit foot-gun at near-zero cost.
**Fix:** `switch strings.ToLower(level) { ... }`.

### IN-03: Exponential backoff shift can silently produce zero delay for large `maxAttempts`

**File:** `internal/db/migrate.go:140-143`
**Issue:** `delay := cfg.baseDelay * time.Duration(uint64(1)<<uint(attempt-1))`. For `attempt-1 >= 64`, the left shift on a `uint64` yields `0` (Go shift semantics, not a panic), producing `delay == 0`, which is *not* greater than `maxDelay` so the subsequent clamp (`if delay > cfg.maxDelay`) never corrects it — attempts would fire back-to-back with no backoff at all. Unreachable with the current `DefaultMaxAttempts = 6`, but `WithMaxAttempts` is a public option with no upper-bound validation, so a future caller passing a large value would get silently-broken backoff rather than a clamped `maxDelay`.
**Fix:** Cap the shift exponent, e.g. `shift := attempt - 1; if shift > 62 { shift = 62 }`, before computing `delay`.

### IN-04: Overlapping panic recovery (`httplog`'s `RecoverPanics: true` and `middleware.Recoverer`)

**File:** `internal/httpserver/server.go:45-56`
**Issue:** `httplog.RequestLogger` is configured with `RecoverPanics: true` (its own panic-recovery-and-log path), and `middleware.Recoverer` is also chained immediately after it. Both mechanisms will independently attempt to recover a panic occurring in a downstream handler, which is redundant and slightly obscures which one is actually responsible for producing the eventual 500 response in a given case. Not incorrect as written, but worth an explicit comment or removing one of the two paths for clarity.
**Fix:** Either drop `RecoverPanics: true` from the `httplog.Options` (relying solely on `middleware.Recoverer`) or drop `middleware.Recoverer` from the chain, and document the choice.

---

_Reviewed: 2026-08-05T17:55:09Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
