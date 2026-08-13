# Phase 9: CI Coverage Gates - Pattern Map

**Mapped:** 2026-08-13
**Files analyzed:** 8 (4 config/CI edits, 4 gap-closing test targets)
**Analogs found:** 8 / 8

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `Makefile` (edit: `test-integration`, new `coverage-gate` target) | config/build-tooling | batch (shell check) | `Makefile`'s own `sqlc-version-check` target | exact (same file, same hand-rolled-shell-gate idiom) |
| `.github/workflows/full-pipeline.yml` (edit: `test` job gate step, `build-scan.needs`) | CI workflow | event-driven | same file's `trivy-fs`/`build-scan` steps (severity-gate + `needs:` pattern) | exact |
| `web/vitest.config.ts` (edit: add `coverage` block) | config | transform (test config) | same file, existing `test:` block | exact (same file) |
| `web/package.json` (edit: add `@vitest/coverage-v8` devDependency) | config | — | same file, existing `vitest` devDependency entry | exact |
| gap-closing test: `cmd/server/main_test.go` (new) | test | request-response / event-driven (boot + graceful shutdown) | `internal/httpserver/boot_e2e_test.go` | exact (same boot-chain, same `testutil.RequirePostgresDSN` pattern) |
| gap-closing test: `internal/logging/logging_test.go` (new) | test | transform (config → logger) | `internal/httpserver/health_test.go` (simplest existing unit-test shape) | role-match |
| gap-closing test: `internal/webassets/embed_test.go` (new) | test | file-I/O (embed.FS serving) | `internal/httpserver/spa_test.go` | role-match (SPA/static-asset serving) |
| gap-closing test: `web/app/routes/history.test.tsx` (new) | test | request-response (route data fetch/render) | `web/app/routes/watchlist.test.tsx` | exact (same route-test shape: `vi.mock("~/lib/api")`, `renderRoute`) |

## Pattern Assignments

### `Makefile` (config/build-tooling, batch)

**Analog:** `Makefile` itself — `sqlc-version-check` target (lines 44-49) and `test-integration` target (lines 30-42)

**Existing hand-rolled shell-gate pattern** (lines 44-49):
```makefile
sqlc-version-check:
	@actual=$$(sqlc version); \
	if [ "$$actual" != "$(SQLC_VERSION)" ]; then \
		echo "sqlc version mismatch: want $(SQLC_VERSION), got $$actual" >&2; \
		exit 1; \
	fi
```
This is the exact idiom to copy for `coverage-gate`: a `@`-prefixed shell block, semicolon-joined, `exit 1` on failure, message to stderr. RESEARCH.md's Pattern 1 code example (COVER_PKGS shell var + `coverage-gate` target) follows this same shape — apply directly.

**Existing `test-integration` target to extend** (lines 39-42):
```makefile
test-integration: db-up
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./... -race -count=1 -p 1

test: test-integration
```
Add `-coverprofile=coverage.out -coverpkg=$(COVER_PKGS)` to the `go test` invocation. Keep the `-p 1` comment (lines 33-38) intact — it explains why sequential execution is required, still true with coverage flags added.

**Comment convention to follow** (used throughout this Makefile, e.g. lines 3-8, 33-38, 58-66): every non-obvious flag/target gets a multi-line `#` comment above it explaining *why*, often citing a research doc or a specific failure mode. Any new `COVER_PKGS`/`coverage-gate` addition should follow this same commenting density.

---

### `.github/workflows/full-pipeline.yml` (CI workflow, event-driven)

**Analog:** same file — `trivy-fs` job (severity-gate pattern, lines 69-81) and `build-scan.needs` array (line 120)

**Existing job step to extend, `test` job** (lines 43-54):
```yaml
  test:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - name: Checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - name: Set up Go
        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: Run integration tests
        run: make test-integration
```
Since `-coverprofile`/`-coverpkg` fold into the `Makefile` target itself (per RESEARCH.md's Code Examples section), this step's `run: make test-integration` line does not need to change — only a new step appended after it:
```yaml
      - name: Backend coverage gate (80%)
        run: make coverage-gate
```
(RESEARCH.md's inline-awk alternative is also valid but the `Makefile`-target form matches this repo's "every CI check maps to a `make` target" convention noted in RESEARCH.md's Architecture Patterns.)

**`needs:` array pattern to extend** (line 120):
```yaml
  build-scan:
    needs: [vet, lint, test, gitleaks, trivy-fs]
```
Append `frontend-test`: `needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]` — exact mechanical edit, per D-11.

**Inline finding-ID comment convention** (lines 145-151, referencing `07-REVIEW.md CR-02`):
```yaml
      # Save the exact image that just passed the scan gate so the release
      # job can push that artifact byte-for-byte, instead of rebuilding from
      # the Dockerfile a second time (07-REVIEW.md CR-02: ...)
```
If the coverage-gate step's rationale for its own placement (e.g. "extends `test`, not a new job — see 08-CONTEXT.md D-04") isn't obvious from the step name alone, follow this same inline-comment-with-doc-reference convention.

**`frontend-test` job — no CI YAML change needed for the gate itself** (lines 83-105): per D-08, `pnpm test` already fails non-zero once `vitest.config.ts`'s `coverage.thresholds` is set; only the `build-scan.needs` line changes on the frontend side.

---

### `web/vitest.config.ts` (config, transform)

**Analog:** same file, existing `test:` block (lines 16-27)

**Current file in full** (all 29 lines already in context — see Read above): the `test` object currently has `environment`, `setupFiles`, `mockReset`, and a comment explaining the deliberate absence of a "zero-test-files still passes" escape hatch. Insert `coverage: { ... }` as a new sibling key inside this same `test: {}` object, immediately after `mockReset: true` — matching this file's existing terse-comment-per-nonobvious-choice style (see header comment lines 1-5 for the tone/format to match: explain *why*, not just *what*).

**Exact block to add** (RESEARCH.md Code Examples, verified against Vitest 4 semantics):
```typescript
coverage: {
  provider: "v8",
  // Without this, only files imported during the test run are measured
  // (coverage.all was removed in Vitest 4) -- an entirely untested route
  // like app/routes/history.tsx would simply never appear in the report.
  include: ["app/**/*.{ts,tsx}"],
  exclude: [
    "app/components/ui/**",        // shadcn primitives, not first-party (D-06)
    "app/lib/test/**",              // test-only helpers (routeStub.tsx)
    "**/*.test.{ts,tsx}",
  ],
  thresholds: {
    lines: 70,
    functions: 70,
    branches: 70,
    statements: 70,
  },
},
```

---

### `web/package.json` (config)

**Analog:** same file, `devDependencies` block (lines 30-46)

**Insertion point:** alphabetical-ish existing ordering; add `"@vitest/coverage-v8": "4.1.10"` matching the exact pinned `"vitest": "4.1.10"` version already on line 45 (RESEARCH.md confirms exact version match is required/verified). No script changes needed — `"test": "vitest run"` (line 10) stays as-is per D-08/A2 (verify empirically per RESEARCH.md Open Question 1 before assuming).

---

### `cmd/server/main_test.go` (test, request-response/event-driven — gap-closing for D-05)

**Analog:** `internal/httpserver/boot_e2e_test.go` (full file read above, 96 lines)

**Boot-chain-replication pattern to copy** (lines 27-51):
```go
func TestBootToHealth_EndToEnd(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("HTTP_PORT", "0")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")

	cfg, err := config.Load()
	...
	logger := logging.New(cfg)
	ctx := context.Background()

	if err := db.RunMigrations(ctx, cfg.DatabaseURL, logger); err != nil { ... }
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	...
	defer pool.Close()
	...
}
```
`cmd/server`'s `run()` (main.go lines 68-205) is the actual target — since `run()` is unexported and lives in `package main`, the new test must be `package main` (not `_test` suffix package) to call `run()` directly, unlike `boot_e2e_test.go`'s `package httpserver_test` black-box style. Testable seams per RESEARCH.md Pitfall 2: (1) successful boot + graceful SIGTERM shutdown — send a signal to the process context and assert `httpSrv.Shutdown` path (main.go lines 196-204) completes within `shutdownTimeout`; (2) config-load-failure early-return path (main.go lines 69-72) — unset/corrupt a required env var and assert `run()` returns a wrapped `"load config: %w"` error without reaching migrations.

**Test-DSN helper to reuse:** `testutil.RequirePostgresDSN(t)` (imported at boot_e2e_test.go line 23) — same helper, same skip-if-no-DB semantics, must be reused rather than reimplemented.

**Signal-driven shutdown pattern to replicate** (main.go lines 82-83, 190-204):
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
...
select {
case err := <-serveErr:
	...
case <-ctx.Done():
	logger.Info("shutdown signal received, shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil { ... }
	return nil
}
```
A test cannot easily send a real OS signal in CI; the practical seam is calling `stop()` (the `context.CancelFunc` `signal.NotifyContext` returns) directly to simulate a SIGTERM being received, then asserting the `ctx.Done()` branch runs and `run()` returns nil within `shutdownTimeout`.

---

### `internal/logging/logging_test.go` (test, transform — gap-closing, currently zero test files)

**Analog:** `internal/httpserver/health_test.go` for the general "small, focused stdlib-`testing`, no assertion library" shape (per TESTING.md, confirmed in RESEARCH.md's Test Framework row: "Go stdlib `testing`... no assertion library").

**Direct target: `internal/logging/logging.go`** (full file read above, 55 lines) — already structured for testability via the `NewWithWriter(cfg, w io.Writer)` seam (lines 20-33) called out in its own doc comment: "the real constructor... tests can capture log output by injecting a different writer." Gap-closing test should call `NewWithWriter` with a `bytes.Buffer` and assert on:
- `cfg.LogFormat == "text"` → `slog.NewTextHandler` selected (line 27)
- default/`"json"` → `slog.NewJSONHandler` selected (line 29)
- `parseLevel` (lines 43-54) — each named level string plus the unrecognized-defaults-to-Info fallback branch (D-09 "untested error/fallback path over easy getter" applies directly here)
- the `slog.String("service", "drop-tracker")` attribute is present on the returned logger (line 32)

---

### `internal/webassets/embed_test.go` (test, file-I/O — gap-closing, currently zero test files)

**Analog:** `internal/httpserver/spa_test.go` (SPA/static-asset serving — closest existing role+data-flow match; read this file during implementation for its exact assertions before writing the new test, not yet read in this pass since `internal/webassets/embed.go` itself was not read either — both should be read together at plan/implementation time).

**Direct target: `internal/webassets/embed.go`** — wraps a `go:embed` tree (`build/client/`) confirmed via Glob to contain `index.html`, `favicon.ico`, and a `assets/` directory of hashed JS/CSS/font bundles. Gap-closing test should assert the embedded `fs.FS` actually serves `index.html` and at least one hashed asset path without error — mirroring whatever `spa_test.go` already asserts against the router-mounted version of this same tree, but exercising `internal/webassets`'s own exported surface directly (unit-level) rather than through `internal/httpserver`.

---

### `web/app/routes/history.test.tsx` (test, request-response — gap-closing for Pitfall 3, currently zero test file)

**Analog:** `web/app/routes/watchlist.test.tsx` (full file read above, 46 lines) — nearly identical route shape (list-fetch-then-render, mocked `~/lib/api`).

**Mock + render pattern to copy verbatim** (lines 1-14):
```tsx
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { listWatchlist, removeWatchlist, type WatchlistEntry } from "~/lib/api"
import { renderRoute } from "~/lib/test/routeStub"

import Watchlist from "./watchlist"

// D-06 / TEST-02: bare vi.mock at the top of the file, no factory, no
// passthrough -- no real apiFetch can ever reach the runtime's own fetch.
vi.mock("~/lib/api")

const mockListWatchlist = vi.mocked(listWatchlist)
const mockRemoveWatchlist = vi.mocked(removeWatchlist)
```
For `history.test.tsx`: `vi.mock("~/lib/api")`, `vi.mocked(listEvents)` (history.tsx imports `listEvents` from `~/lib/api`, line 12 of history.tsx), `renderRoute(History, "/history")` from the same `~/lib/test/routeStub` helper.

**Assertion pattern to copy** (lines 31-46):
```tsx
describe("Watchlist route", () => {
  it("calls removeWatchlist with the entry's id when its remove control is clicked", async () => {
    mockListWatchlist.mockResolvedValue([entry])
    ...
    renderRoute(Watchlist, "/watchlist")
    await screen.findByText("Drake")
    await userEvent.click(screen.getByRole("button", { name: "..." }))
    expect(mockRemoveWatchlist).toHaveBeenCalledWith(42)
  })
})
```
For `history.tsx`'s actual behavior (per full read above, lines 48-197), the most meaningful uncovered paths per D-09 are: initial fetch success (events render via `EventCard`), initial fetch error (`error` state → `EmptyState` with Retry button, lines 130-140), empty-state distinction between filtered/unfiltered (lines 150-159), and "Load more" pagination appending + de-duping by id (lines 93-116) — prioritize the error-path and de-dupe-append cases over the simple happy-path render, consistent with D-09's "untested error path matters more than an easy getter."

---

## Shared Patterns

### Hand-rolled shell/awk gate convention (backend)
**Source:** `Makefile`'s `sqlc-version-check` target (lines 44-49)
**Apply to:** `Makefile`'s new `coverage-gate` target — same `@`-prefixed, semicolon-chained, stderr-message-then-`exit 1` shape.

### Doc-referencing inline comments (both CI and Makefile)
**Source:** `.github/workflows/full-pipeline.yml` lines 145-151 (`07-REVIEW.md CR-02` reference); `Makefile` lines 3-8, 13-16
**Apply to:** Any new step/target whose rationale isn't self-evident from its name — cite `09-CONTEXT.md D-0x` or `09-RESEARCH.md Pitfall N` inline, matching this repo's established citation style.

### `testutil.RequirePostgresDSN(t)` for any real-Postgres test
**Source:** `internal/httpserver/boot_e2e_test.go` line 28
**Apply to:** `cmd/server/main_test.go` — any gap-closing test that needs a real DB connection must use this shared helper, not reimplement DSN/skip logic.

### `vi.mock("~/lib/api")` + `renderRoute` for any route test
**Source:** `web/app/routes/watchlist.test.tsx` lines 1-14, and the sibling analogs `EventCard.test.tsx`, `HistoryFilters.test.tsx` (both already exist under `web/app/components/history/`, confirming `EventCard`/`HistoryFilters` sub-components used by `history.tsx` already have their own unit coverage — `history.test.tsx` only needs to cover the route-level composition/state logic, not re-test those children's internals).
**Apply to:** `web/app/routes/history.test.tsx`

## No Analog Found

None — every file in scope has a direct or role-matched analog in the existing codebase.

## Metadata

**Analog search scope:** `Makefile`, `.github/workflows/full-pipeline.yml`, `web/vitest.config.ts`, `web/package.json`, `internal/httpserver/*_test.go`, `internal/logging/`, `internal/webassets/`, `cmd/server/main.go`, `web/app/routes/*.tsx`, `web/app/routes/*.test.tsx`
**Files scanned:** ~20
**Pattern extraction date:** 2026-08-13
