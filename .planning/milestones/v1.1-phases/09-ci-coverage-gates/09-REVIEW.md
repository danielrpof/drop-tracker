---
phase: 09-ci-coverage-gates
reviewed: 2026-08-13T00:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - .github/workflows/full-pipeline.yml
  - .gitignore
  - Makefile
  - cmd/server/main.go
  - cmd/server/main_test.go
  - internal/logging/logging_test.go
  - internal/webassets/embed_test.go
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/lib/api.test.ts
  - web/app/root.test.tsx
  - web/app/routes/history.test.tsx
  - web/app/routes/watchlist.test.tsx
  - web/package.json
  - web/pnpm-lock.yaml
  - web/vitest.config.ts
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 09: Code Review Report

**Reviewed:** 2026-08-13T00:00:00Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

This phase adds backend (Makefile/CI) and frontend (Vitest) coverage gates, plus the new test files needed to clear both thresholds, and refactors `cmd/server/main.go` so `run()` takes an injected `context.Context` for direct whitebox testing. I read every listed file in full and additionally executed the actual gates rather than only reading them:

- `pnpm test` in `web/` was run to completion: 9 test files, 41 tests, all passing, with coverage 78.06% statements / 71.57% branches / 75.75% functions / 79.75% lines — all above the configured 70% floor, and the process exits 0, confirming the frontend gate is real and currently green (branches is the tightest margin at 71.57% vs. 70%).
- `make test-integration` was attempted against a real dockerized Postgres to verify the backend gate end-to-end. It could not complete in this sandboxed Windows environment: every package fails with `ThreadSanitizer failed to allocate ... (error code: 87)` before any test logic runs, which is a `-race`/TSan memory-allocation limitation of this specific sandbox (Windows container, restricted address-space), not a defect in the reviewed code. The `coverage-gate` Makefile logic itself was verified by careful manual trace of the `awk`/`grep` pipeline instead (see below) — no correctness defect found in it.

The `cmd/server/main.go` change is a clean, mechanical refactor: signal handling (`signal.NotifyContext` + `stop()`) moves from inside `run()` up into `main()`, and `run` now takes `ctx context.Context` as a parameter. The timing of `stop()` is preserved correctly (previously `defer`red inside `run`, now called explicitly right after `run` returns in `main`), and `main_test.go`'s two new tests exercise both the fail-fast config-load-error branch and the full boot → healthy → graceful-shutdown-on-cancel branch. No functional regression found in the diff.

No critical/security issues were found. Two warnings and two info-level items are listed below — all are about test robustness and minor dependency placement, not correctness defects that would block a merge.

## Warnings

### WR-01: Boot test doesn't fail fast on an early `run()` error, hiding the real cause behind a generic timeout

**File:** `cmd/server/main_test.go:79-99`
**Issue:** `TestRun_BootServesHealthThenGracefulShutdownOnCancel` starts `run(ctx)` in a goroutine and polls `GET /health` in a plain loop for up to 10 seconds, only reading from `done` in the two branches that already know the outcome. If `run()` returns an error *before* the listener ever comes up (e.g. migrations fail, or the reserved port is grabbed by another process between `ln.Close()` and `httpSrv.ListenAndServe()` — a real possibility given the test's own port-reservation comment), the polling loop has no way to observe that and will keep retrying `http.Get` against a connection that is refused for the full 10-second deadline, then fail with "server never became healthy before the deadline" — a message that gives no hint that `run()` actually returned an error immediately. This makes a real regression in the boot path slower to diagnose and, on a loaded CI runner, more prone to a bare timeout rather than a clear failure.
**Fix:** Select on `done` alongside the health-check poll so an early `run()` error surfaces immediately with its real message:
```go
deadline := time.After(10 * time.Second)
ticker := time.NewTicker(50 * time.Millisecond)
defer ticker.Stop()
for {
    select {
    case runErr := <-done:
        t.Fatalf("run() returned before the server became healthy: %v", runErr)
    case <-deadline:
        t.Fatal("server never became healthy before the deadline")
    case <-ticker.C:
        resp, err := http.Get(healthURL)
        if err == nil {
            defer resp.Body.Close()
            if resp.StatusCode == http.StatusOK {
                goto healthy
            }
        }
    }
}
healthy:
```

### WR-02: `COVER_PKGS` package filter uses an unanchored substring match

**File:** `Makefile:26`
**Issue:** `COVER_PKGS = $(shell go list ./... | grep -v '/internal/db/sqlc' | paste -sd, -)` excludes the generated sqlc package from the coverage-instrumented package set, which is the intent (documented at lines 18-26). However `grep -v '/internal/db/sqlc'` matches the string anywhere in the import path, not just the exact `internal/db/sqlc` package. If a future package is added whose path merely contains that substring (e.g. `internal/db/sqlcgen`, `internal/db/sqlc/v2`, or even a hypothetically nested `internal/db/sqlc/testutil`), it would be silently dropped from `-coverpkg` too, understating true coverage or masking an untested package without any error — the opposite of this phase's goal (catching untested code, per `09-CONTEXT.md D-05`'s stated concern about `cmd/server` silently never entering the profile).
**Fix:** Anchor the exclusion to the full package path boundary, e.g. `grep -vE '(^|/)internal/db/sqlc$'` (or filter on the exact known set of paths from `go list` rather than a free-text `grep`).

## Info

### IN-01: `shadcn` CLI listed as a runtime dependency, not a dev dependency

**File:** `web/package.json:25`
**Issue:** `"shadcn": "^4.16.2"` sits under `dependencies`, but `shadcn` is a code-generation CLI (used to scaffold/update the `app/components/ui/**` primitives) that is never imported by any runtime or build-time module in `app/`. Every other tool-only package in this file (`prettier`, `typescript`, `tailwindcss`, `vitest`, etc.) is correctly placed under `devDependencies`. Leaving it in `dependencies` doesn't break anything functionally (it's excluded from coverage and not `go:embed`-ded since only the built `dist/` output is embedded), but it does misrepresent the package's actual runtime footprint to anyone auditing `dependencies` for what ships.
**Fix:** Move `"shadcn": "^4.16.2"` to `devDependencies`.

### IN-02: Reserved-port boot test has an inherent (small) TOCTOU race

**File:** `cmd/server/main_test.go:50-58`
**Issue:** The test closes a listener it just opened on `127.0.0.1:0` and then hands the freed port number to `run()` via `HTTP_PORT` a few lines later. Between `ln.Close()` and `httpSrv.ListenAndServe()` binding the same port inside `run()`, any other process on the runner could theoretically claim that port, causing an unrelated bind failure that looks like a boot regression. This is a well-known, generally-accepted Go testing idiom (and the code comment already justifies it over a hardcoded port), so it's noted here only as a residual, low-probability flake source — not something that needs to change for this phase, especially once WR-01 above makes any resulting failure self-explanatory via the surfaced `run()` error.
**Fix:** No action required; documenting as accepted risk. If flakiness is observed in CI, consider a short retry-with-new-port loop.

---

_Reviewed: 2026-08-13T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
