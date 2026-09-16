# Phase 15: PR Coverage-Diff Comment - Pattern Map

**Mapped:** 2026-09-02
**Files analyzed:** 6 (2 new, 4 modified)
**Analogs found:** 6 / 6

All analog paths below are git-tracked source in this repo (no submodule, no gitignored mirror). Verified against `git ls-files`.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/coverage-report/main.go` | CLI tool (`main` pkg) | file-I/O / transform | `cmd/server/main.go` | role-match (only other `cmd/*`; different data flow — server is long-running, this is parse-and-exit) |
| `cmd/coverage-report/main_test.go` | test | file-I/O / transform | `internal/authgate/weak_test.go` (table tests) + `internal/config/config_test.go` (`repoRoot` file-relative fixtures) + `cmd/server/main_test.go` (whitebox `main` pkg) | exact (composite) |
| `cmd/coverage-report/testdata/` | test fixtures | — | none exist in repo | no analog — see "No Analog Found" |
| `.github/workflows/full-pipeline.yml` (MODIFY) | CI config | event-driven | `release` job (job-scoped `permissions` + `concurrency`) + `build-scan`→`release` artifact hand-off + `test`/`frontend-test` jobs | exact |
| `Makefile` (MODIFY) | build config | transform | existing `COVER_PKGS` line + `coverage-gate` target | exact (self-analog) |
| `web/vitest.config.ts` (MODIFY) | test config | — | existing `coverage.reporter` array | exact (self-analog) |
| `.golangci.yml` (MODIFY) | lint config | — | existing `_test\.go$` → `gosec` exclusion rule | exact (self-analog) |

## Pattern Assignments

### `cmd/coverage-report/main.go` (CLI tool, file-I/O + transform)

**Analog:** `cmd/server/main.go`

**Package-doc + `main`/`run` split** (`cmd/server/main.go:1-4`, `83-108`) — copy this exact shape so the logic is unit-testable without invoking the process:
```go
// Command coverage-report ...
package main

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error { ... }
```
`cmd/server` uses `func run(ctx context.Context) error`; here take `args []string` (or a `flag.FlagSet`) so tests drive `--mode`/`--profile` directly. `cmd/server/main_test.go:1-6` documents the whitebox rationale ("calling the unexported run function -- whitebox, same package").

**Error wrapping convention** (`cmd/server/main.go:109-112`): `fmt.Errorf("load config: %w", err)` — lowercase prefix, `%w`. Apply to every I/O failure (`open profile: %w`, `parse profile: %w`).

**Constants with intent comments, no header blocks** (`cmd/server/main.go:35-63`) and CLAUDE.md "Comment discipline" — 1–3 line why-comments only. The `coverage.out` grammar and the half-up-to-2dp rounding rule each earn one short comment (cite nothing longer than `D-06`).

**stdout vs stderr discipline** (RESEARCH §Mechanism 8): `--mode=total` prints ONLY the bare number to stdout; every diagnostic to stderr — mirrors `cmd/server/main.go:96` (`fmt.Fprintln(os.Stderr, err)`).

**Imports** — stdlib only (`bufio`, `os`, `strconv`, `strings`, `fmt`, `flag`, `encoding/json` for the Vitest summary + sidecar). Do NOT add `golang.org/x/tools/cover` (D-06). Module path for the new package: `github.com/danielrpof/drop-tracker/cmd/coverage-report`.

**`coverage.out` parse** (RESEARCH §Mechanism 3): skip the `mode:` line; for each remaining line split on the **last** `:` (file field can contain `:`), then fields are `startLine.col,endLine.col numStmts count`; accumulate `sum(numStmts)` and `sum(numStmts where count>0)`; percent = covered/total*100, round half-up to 2 dp. `count>0` ⇔ covered in all three modes.

**No auth/validation pattern** — CLI tool, inputs are CI-controlled paths (this is exactly why D-19 adds the `gosec` carve-out rather than input sanitization).

---

### `cmd/coverage-report/main_test.go` (test, `package main` whitebox)

**Analog:** `internal/authgate/weak_test.go` (table-test idiom) + `internal/config/config_test.go` (file fixtures) + `cmd/server/main_test.go` (whitebox `main`)

**Table-test structure** (`internal/authgate/weak_test.go:16-49`) — the established repo idiom:
```go
cases := []struct {
	name   string
	in     string
	weak   bool
	reason string
}{
	{"empty is not weak (gate disabled)", "", false, ""},
	...
}
for _, tc := range cases {
	t.Run(tc.name, func(t *testing.T) {
		reason, weak := IsWeakPassphrase(tc.in)
		if weak != tc.weak {
			t.Fatalf("weak = %v, want %v", weak, tc.weak)
		}
	})
}
```
Use `tc` as the loop var, `t.Run(tc.name, ...)`, `t.Fatalf`/`t.Errorf` with `"got = %v, want %v"` phrasing. Nested subtests also seen in `internal/authgate/gate_test.go:617-654`.

**Locating `testdata/` from the test** (`internal/config/config_test.go:17-29`) — `runtime.Caller(0)` + `filepath.Join(filepath.Dir(thisFile), ...)` so tests pass regardless of `go test` working directory. For `testdata/` the simpler idiom is a relative `"testdata/foo.out"` (Go guarantees CWD = package dir during tests) — but the `runtime.Caller` helper is the repo precedent if anything reaches outside the package dir.

**Whitebox `package main`** (`cmd/server/main_test.go:1`) — test file declares `package main`, calls unexported `run`. Capture stdout by having `run` take an `io.Writer` param, or call the pure parse function directly (preferred — keep `run` thin, test the parser).

**Golden-file comparison** — no repo precedent (see below). Recommended minimal idiom: read `testdata/expected_comment.md`, `bytes.Equal`, and support `-update` flag to rewrite goldens (`go test ./cmd/coverage-report -run TestComment -update`). Keep it simple; there is no existing helper to reuse.

---

### `cmd/coverage-report/testdata/`

No analog — first `testdata/` directory in the repo (`Glob **/testdata/**` → none). Create fixtures by hand: a small real-shaped `coverage.out` (`mode: atomic` + a dozen block lines, some `count 0` some `count>0`), a `coverage-summary.json` (istanbul shape from RESEARCH §Mechanism 4), a `baseline-metrics.json` sidecar, and golden markdown outputs for: normal delta, no-baseline (`—`), unchanged (`±0.00pp`), per-row `unavailable`. Planner supplies exact contents from RESEARCH.

---

### `.github/workflows/full-pipeline.yml` (MODIFY, CI config, event-driven)

**Analog A — job-scoped `permissions` + job-scoped `concurrency`:** `release` job (`full-pipeline.yml:180-198`), also `pr-title` (`:121-127`) for the PR-only pattern:
```yaml
  release:
    needs: [build-scan]
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    timeout-minutes: 20
    concurrency:
      group: release-${{ github.ref }}
      cancel-in-progress: false
    permissions:
      contents: write
      packages: write
```
For `coverage-comment`: `needs: [test, frontend-test]`, `if: ${{ !cancelled() && github.event_name == 'pull_request' }}`, `permissions: { contents: read, pull-requests: write }`, `concurrency: { group: coverage-comment-${{ github.ref }}, cancel-in-progress: true }`. Do NOT touch workflow-level `permissions: contents: read` (`:7-8`). Do NOT add `coverage-comment` to `build-scan.needs` (`:137`).

**Analog B — artifact hand-off:** `build-scan` upload (`full-pipeline.yml:169-178`) → `release` download (`:219-223`):
```yaml
      - name: Upload scanned image artifact
        if: github.event_name == 'push' && github.ref == 'refs/heads/main'
        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1
        with:
          name: scanned-image
          path: /tmp/drop-tracker-scan.tar
          retention-days: 1
```
```yaml
      - name: Download scanned image artifact
        uses: actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8.0.1
        with:
          name: scanned-image
          path: /tmp
```
For D-15: reuse these exact pinned SHAs and `retention-days: 1`. Drop the push/main guard (upload every run), add `if: ${{ !cancelled() }}`, use names without `:` (`coverage-backend-pr`, `coverage-frontend-pr`). On the two `download-artifact` steps in `coverage-comment`, ADD `continue-on-error: true` (D-18 — missing artifact fails the step).

**Analog C — `test` job step insertion point** (`full-pipeline.yml:53-60`):
```yaml
      - name: Run integration tests
        run: make test-integration
      - name: Backend coverage gate (80)
        run: make coverage-gate
```
Insert `upload-artifact` (of `coverage.out`) BETWEEN these two steps (D-15/D-18 ordering). Append `cache/save` AFTER `make coverage-gate` with `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` + `continue-on-error: true` (D-13/D-14/D-04).

**Analog D — `frontend-test` job** (`full-pipeline.yml:89-119`): note `defaults.run.working-directory: web` — the profile lands at `web/coverage/coverage-summary.json`. Append `upload-artifact` + `cache/save` AFTER `pnpm test` (`:118-119`); the profile is written by `pnpm test` itself so there is no "before the gate" insertion — the `if: ${{ !cancelled() }}` on the upload covers the sub-70 non-zero exit.

**Analog E — reused action pins** (already in file, copy verbatim): `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1` (`:20`), `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0` (`:22`). **New pins** (RESEARCH §Standard Stack, verified this session): `actions/cache/restore` + `actions/cache/save@55cc8345863c7cc4c66a329aec7e433d2d1c52a9 # v6.1.0`, `marocchino/sticky-pull-request-comment@5770ad5eb8f42dd2c4f34da00c94c5381e49af88 # v3.0.5`. SHA-pin + trailing `# vX.Y.Z` comment is the repo convention — every `uses:` in this file follows it.

**Fork guard** (RESEARCH §Mechanism 2 / Pitfall 4): `if: github.event.pull_request.head.repo.full_name == github.repository` on the marocchino step + `$GITHUB_STEP_SUMMARY` fallback + job-level `continue-on-error` (D-04).

---

### `Makefile` (MODIFY, transform)

**Analog — the `COVER_PKGS` line itself** (`Makefile:34`, rationale comment `:19-33`):
```make
COVER_PKGS = $(shell go list ./... | grep -vE '(^|/)internal/db/sqlc$$' | paste -sd, -)
```
D-07 change — extend the anchored alternation, keep the doubled `$$`:
```make
COVER_PKGS = $(shell go list ./... | grep -vE '(^|/)(internal/db/sqlc|cmd/coverage-report)$$' | paste -sd, -)
```
The existing comment already explains why the anchor matters (`09-REVIEW.md WR-02`) — extend it by one line, don't re-argue.

**Analog — `coverage-gate` target** (`Makefile:81-97`), the line to replace is `:86`:
```make
	@coverage=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{v=$$3; print substr(v, 1, length(v)-1)}'); \
```
becomes (D-17 / RESEARCH §Mechanism 8):
```make
	@coverage=$$(go run ./cmd/coverage-report --mode=total --profile=coverage.out); \
```
Keep the `-s coverage.out` guard (`:82-85`), the `-z "$$coverage"` guard (`:87-90`), the `echo` (`:91`), the `awk` threshold compare against `$(COVERAGE_THRESHOLD_BACKEND)` (`:92`), the PASS/FAIL exits (`:93-97`). `COVERAGE_THRESHOLD_BACKEND ?= 80` (`:40`) stays literal and untouched.

**Analog — `.PHONY` line** (`Makefile:1`) and phony helper targets like `sqlc-version-check` (`:99-104`): add `coverage-report` to `.PHONY` and add:
```make
coverage-report:
	@go run ./cmd/coverage-report --mode=total --profile=coverage.out
```
Match the existing `@`-prefixed recipe style and the leading-tab (not spaces) indentation.

---

### `web/vitest.config.ts` (MODIFY, test config)

**Analog — the `coverage.reporter` line** (`web/vitest.config.ts:36-38`):
```ts
      // Text reporter only -- log-only output, writes nothing to disk
      // (mirrors the backend's D-03 posture).
      reporter: ["text"],
```
D-06 change:
```ts
      // "text" for the CI log; "json-summary" writes coverage/coverage-summary.json
      // for the Phase 15 PR coverage comment (web/coverage/ is gitignored).
      reporter: ["text", "json-summary"],
```
Do NOT add `"json"` (D-06 review note — deferred to CICD-15). Do NOT set `reportsDirectory` — default `./coverage` relative to `web/` is correct and already gitignored (`.gitignore:19`). `thresholds` block (`:56-61`) unchanged. After editing, run `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` per CLAUDE.md before staging.

---

### `.golangci.yml` (MODIFY, lint config)

**Analog — the existing `gosec` exclusion rule** (`.golangci.yml:27-36`):
```yaml
  exclusions:
    rules:
      - path: '_test\.go$'
        linters:
          - gosec
```
D-19 — add a sibling entry after it:
```yaml
      - path: '^cmd/coverage-report/'
        linters:
          - gosec
```
Add a 2–3 line comment mirroring the style of the existing one (`:29-33`): the tool opens CI-controlled coverage-file paths from argv → G304 is not a real path-traversal surface. Anchored `^cmd/coverage-report/` preferred over bare substring (RESEARCH §Mechanism 7). Scope stays minimal: only `gosec`, only this dir.

## Shared Patterns

### Error wrapping (Go)
**Source:** `cmd/server/main.go:109-145` (`fmt.Errorf("load config: %w", err)`)
**Apply to:** all of `cmd/coverage-report/main.go`
Lowercase action-phrase prefix + `%w`. Fatal path prints to stderr and `os.Exit(1)` (`cmd/server/main.go:95-98`).

### Table tests
**Source:** `internal/authgate/weak_test.go:16-49`
**Apply to:** `cmd/coverage-report/main_test.go`
Anonymous `[]struct` slice with a `name` field, `for _, tc := range cases`, `t.Run(tc.name, ...)`, `t.Fatalf`/`t.Errorf`.

### File-relative test fixtures
**Source:** `internal/config/config_test.go:17-29` (`runtime.Caller` → repo root helper)
**Apply to:** `cmd/coverage-report/main_test.go` only if a fixture must be read from outside the package dir; otherwise plain `testdata/` relative paths.

### CI action pinning
**Source:** every `uses:` in `.github/workflows/full-pipeline.yml` (e.g. `:20`, `:39`, `:174`)
**Apply to:** all new `uses:` lines
`uses: owner/action@<40-char-sha> # vX.Y.Z` — enforced by CLAUDE.md.

### Job-scoped privilege escalation
**Source:** `release` (`full-pipeline.yml:195-197`), `pr-title` (`:125-126`)
**Apply to:** the `coverage-comment` job
Never widen workflow-level `permissions`; the job opts into `pull-requests: write` alone.

### continue-on-error for report-only surfaces
**Sourceः**  no direct analog — `build-scan`'s save steps are guarded by `if:` not `continue-on-error`. This is a new pattern (D-04). Apply per-step `continue-on-error: true` on both `cache/save` steps (mandatory) and job-level or per-step on `coverage-comment` (planner's call).

### Comment discipline
**Source:** CLAUDE.md "Comment discipline"; anti-pattern `web/app/lib/authStore.ts`
**Apply to:** all six files
1–3 line why-comments; one design-doc ref (`D-06`, `D-17`) is enough; extend existing rationale comments by a line rather than re-arguing.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `cmd/coverage-report/testdata/` | fixtures | — | No `testdata/` dir exists anywhere in the repo yet. Planner defines fixture contents from RESEARCH §Mechanism 3/4. |
| Golden-file test helper (`-update` flag idiom) | test infra | — | No golden-file testing precedent in the repo. Introduce the minimal standard Go idiom (`bytes.Equal` vs `testdata/*.golden` + `-update` flag). |
| `actions/cache` restore/save usage | CI step | event-driven | The workflow currently uses `type=gha` docker layer cache only (`full-pipeline.yml:154-155`), never `actions/cache` directly. Follow RESEARCH §Mechanism 1 (key shape, `restore-keys` prefix, `cache-matched-key` detection per D-20). |
| `marocchino/sticky-pull-request-comment` | CI step | request-response | No PR-commenting action in the repo. Follow RESEARCH §Mechanism 2 (`header:` marker, `path:` input, fork guard). |

## Metadata

**Analog search scope:** `cmd/`, `internal/`, `.github/workflows/`, repo root config files, `web/`
**Files scanned:** `cmd/server/main.go`, `cmd/server/main_test.go`, `internal/config/config_test.go`, `internal/authgate/weak_test.go`, `.github/workflows/full-pipeline.yml`, `Makefile`, `.golangci.yml`, `web/vitest.config.ts`; Glob for `**/testdata/**` (none), Grep for table-test / file-read idioms
**Pattern extraction date:** 2026-09-02
