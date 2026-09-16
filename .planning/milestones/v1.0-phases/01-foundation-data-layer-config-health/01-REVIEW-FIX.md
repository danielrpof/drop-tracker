---
phase: 01-foundation-data-layer-config-health
fixed_at: 2026-08-05T18:08:09Z
review_path: .planning/phases/01-foundation-data-layer-config-health/01-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 01: Code Review Fix Report

**Fixed at:** 2026-08-05T18:08:09Z
**Source review:** .planning/phases/01-foundation-data-layer-config-health/01-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (fix_scope: critical_warning -- CR-01 plus WR-01..WR-04; IN-01..IN-04 excluded by scope)
- Fixed: 5
- Skipped: 0

**Verification environment:** All fixes were applied and verified inside an isolated git worktree (`gsd-reviewfix/01-8594`, fast-forwarded onto `main` on cleanup). `go build`, `go vet`, and `go test` were run in that worktree. A live Postgres 16 container (`drop-tracker-postgres-1`, already running locally on port 5432) was available and used to run the full DB-backed test suite (`TestRunMigrations_AppliesFromScratch`, `TestRunMigrations_IsIdempotent`, `TestSQLCPing`, `TestBootToHealth_EndToEnd`, `TestBootToHealth_MigrationsAreIdempotent`) in addition to the pure-unit tests, so these results are reproducible from the same tree state that was fast-forwarded onto `main`.

## Fixed Issues

### CR-01: `redactDSN` leaks the raw password when DATABASE_URL uses libpq keyword/value form

**Files modified:** `internal/db/migrate.go`, `internal/db/migrate_test.go`
**Commit:** `fc3c02d`
**Applied fix:** Replaced the hand-rolled `url.Parse`-based `redactDSN` with `pgconn.ParseConfig(dsn)` (from `github.com/jackc/pgx/v5/pgconn`, already a transitive dependency via `pgx/v5`), which correctly understands both the URL DSN form and libpq's keyword/value form -- matching the approach `internal/db/pool.go`'s `redactedTarget` already used via `pgxpool.ParseConfig`. Removed the now-unused `net/url` and `strings` imports. Added `closedPortKeywordValueDSN` and `TestRunMigrations_NeverLogsDSN_KeywordValueForm` (mirroring the existing `TestRunMigrations_NeverLogsDSN`) to `migrate_test.go` to lock in the fix and catch regressions of this class going forward.
**Verification:** `go build`, `go vet`, and the full `internal/db` package test suite pass, including the new keyword/value-form test, both standalone and against a live Postgres instance (which exercises the real apply-from-scratch migration path).

### WR-01: Migration attempt itself is not context-aware -- only the inter-retry delay is

**Files modified:** `internal/db/migrate.go`
**Commit:** `efc9830`
**Applied fix:** `runMigrationsOnce` now takes `ctx context.Context`. It calls `sqlDB.PingContext(ctx)` immediately after opening the connection (catching an unreachable/slow-to-accept database before any migration work begins), then runs `m.Up()` on a background goroutine and `select`s on its completion versus `ctx.Done()`, so a hang mid-migration (e.g. a stuck advisory lock or a connection that accepts but never responds) can no longer block `RunMigrations` past the caller's context cancellation. `RunMigrations` now short-circuits and returns immediately if `runMigrationsOnce` returns a context-cancellation error, rather than looping into another retry/backoff cycle.
**Verification:** `go build`, `go vet` pass. Full `internal/db` suite (including `TestRunMigrations_HonoursContextCancellation`, `TestRunMigrations_RetriesThenFails`, and both `NeverLogsDSN` tests) passes both standalone and against a live Postgres instance -- `TestRunMigrations_AppliesFromScratch` specifically exercises the new goroutine-wrapped `m.Up()` on the success path.
**Note:** This introduces a concurrency change (a background goroutine racing `sqlDB.Close()` on cancellation, which `database/sql` supports safely but which automated syntax/unit checks cannot fully prove is race-free without a working race detector -- CGO/gcc is unavailable in this environment, so `go test -race` could not be run here). **Status: fixed: requires human verification** -- recommend running `go test -race ./internal/db/...` in CI (where a C toolchain is present) before this phase is considered fully closed.

### WR-02: No HTTP server timeouts configured

**Files modified:** `cmd/server/main.go`
**Commit:** `f2e3223`
**Applied fix:** Replaced the bare `http.ListenAndServe(addr, srv.Router())` call with an explicitly constructed `*http.Server` carrying `ReadHeaderTimeout: 5s`, `ReadTimeout: 15s`, `WriteTimeout: 15s`, `IdleTimeout: 60s` (named constants, documented). Also added an `errors.Is(err, http.ErrServerClosed)` guard around the `ListenAndServe` error check, anticipating the graceful-shutdown path added in WR-03.
**Verification:** `go build`, `go vet` pass. Manually smoke-tested: built the binary, ran it against the live Postgres container with `HTTP_PORT=18085`, confirmed `GET /health` returns `200 {"status":"ok","db":"up"}`, then terminated the process cleanly.

### WR-03: No graceful shutdown -- `defer pool.Close()` is effectively unreachable

**Files modified:** `cmd/server/main.go`
**Commit:** `275551e`
**Applied fix:** `run()` now derives `ctx` from `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` (reused for migrations, pool construction, and the HTTP lifecycle). `ListenAndServe` runs on a background goroutine reporting into a buffered error channel; `run()` then `select`s between that channel and `ctx.Done()`. On signal, it logs, builds a 10-second-bounded shutdown context, and calls `httpSrv.Shutdown(shutdownCtx)` before returning -- making the deferred `pool.Close()` and any future scheduler shutdown (Phase 3) reachable on a normal SIGTERM/SIGINT rather than only on a listener error.
**Verification:** `go build`, `go vet` pass. Manually smoke-tested boot-and-serve against the live Postgres container (see WR-02). **Could not fully exercise real OS-level SIGTERM delivery in this Windows development sandbox** -- Windows has no true POSIX SIGTERM, so `kill <pid>` from the bash tool terminates the process directly rather than routing through Go's signal handler; the graceful-shutdown code path itself is the standard, widely-used `signal.NotifyContext` + `srv.Shutdown` idiom and will function as written under the Linux container environment this project targets. **Status: fixed: requires human verification** -- recommend a manual `docker run` + `docker stop` (which sends SIGTERM) smoke test, or a CI job that sends SIGTERM to the built binary, to confirm graceful shutdown end-to-end on the target platform before this phase is considered fully closed.

### WR-04: `.gitignore` is a near-verbatim Python project template, not this project's

**Files modified:** `.gitignore`
**Commit:** `f7f80ab`
**Applied fix:** Replaced the ~224-line Python-template `.gitignore` with a Go-appropriate one covering Go build artifacts (`/bin/`, `*.exe`, `*.test`, `*.out`, `coverage.out`, `coverage.html`), Go workspace files, the future frontend build output (`node_modules/`, `dist/` -- per CLAUDE.md's documented React+Vite-embedded-via-go:embed architecture), environment/secrets (`.env`, `.envrc`), editor directories (`.vscode/`, `.idea/`), and common OS cruft (`.DS_Store`, `Thumbs.db`).
**Verification:** Confirmed via `git ls-files -i -c --exclude-standard` that no currently-tracked file becomes newly ignored by the replacement file (empty result). No syntax checker applies to `.gitignore`; Tier 1 re-read confirms content is well-formed.

## Skipped Issues

None -- all in-scope findings were fixed.

---

_Fixed: 2026-08-05T18:08:09Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
