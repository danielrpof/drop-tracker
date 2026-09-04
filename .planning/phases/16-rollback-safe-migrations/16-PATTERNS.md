# Phase 16: Rollback-Safe Migrations - Pattern Map

**Mapped:** 2026-09-04
**Files analyzed:** 10 (2 modify-source, 3 new Go, 1 new test, 1 new prose, 3 config/CI edits)
**Analogs found:** 9 / 10 (README.md has no in-repo prose analog — structure comes from RESEARCH.md §"Rule Documentation")

All analog paths below are git-tracked source in the main worktree (no gitignored
capability mirrors involved — this repo has no `.gsd/capabilities/` tree).

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/db/migrate.go` (MODIFY) | migration/boot utility | batch / transform | its own `RunMigrations`/`runMigrationsOnce` body | self (extract-seam refactor) |
| `internal/db/migrate_ahead_test.go` (NEW, `package db`) | test (integration, real Postgres) | batch | `internal/db/migrate_test.go` + `internal/testutil/postgres.go` | exact |
| `cmd/migration-check/main.go` (NEW) | CLI tool (CI guard) | file-I/O + transform (SQL scan, `git show` shell) | `cmd/coverage-report/main.go` | role-match (both stdlib CI helpers; different input format) |
| `cmd/migration-check/main_test.go` (NEW, `package main`) | test (whitebox unit) | transform | `cmd/coverage-report/main_test.go` | exact |
| `cmd/migration-check/testdata/*.sql` (NEW) | test fixtures | — | `cmd/coverage-report/testdata/` golden files | role-match (golden→SQL fixture) |
| `cmd/migrate/main.go` (NEW; name at discretion) | CLI helper (thin entrypoint) | request-response (calls `db.RunMigrations`) | `cmd/server/main.go` migration sequencing (`main.go:137-139`) + `cmd/coverage-report/main.go:40-45` thin-main shape | role-match |
| `.golangci.yml` (MODIFY) | config | — | existing `cmd/coverage-report` G304 carve-out (`.golangci.yml:42-44`) | exact |
| `Makefile` (MODIFY) | config | — | existing `COVER_PKGS` `grep -vE` exclusion (`Makefile:36`) | exact |
| `.github/workflows/full-pipeline.yml` (MODIFY) | CI config | event-driven | `test` job (Postgres), `release` job (`svu`, `fetch-depth: 0`), `build-scan.needs:` array, `coverage-comment` (env-not-interpolate) | exact / role-match |
| `internal/db/migrations/README.md` (NEW) | prose doc | — | *no in-repo analog* — use RESEARCH.md §"Rule Documentation" structure; cite `internal/db/migrations/000007_backfill_events_watched_artist_name.up.sql` | no analog |

---

## Pattern Assignments

### `internal/db/migrate.go` (MODIFY — extract seam + ahead-of-source guard)

**Analog:** the current file itself (`internal/db/migrate.go:207-298`). This is a
behaviour-preserving refactor plus one new guarded branch (RESEARCH Finding 1).

**Current `RunMigrations` body to split** (lines 207-246): `iofs.New(migrationsFS,
"migrations")` → `newRetryConfig(opts...)` → `redactDSN(dsn)` → retry loop calling
`runMigrationsOnce(ctx, dsn, src)`.

**D-18 seam refactor** (RESEARCH §"Pattern 1", lines 347-367) — keep `RunMigrations`
signature byte-identical, move body from the `newRetryConfig` line onward into a new
unexported function:

```go
func RunMigrations(ctx context.Context, dsn string, logger *slog.Logger, opts ...RetryOption) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}
	return runMigrationsWithSource(ctx, dsn, logger, src, opts...)
}

// runMigrationsWithSource holds the former RunMigrations body from newRetryConfig
// onward. Unexported; migrate_ahead_test.go drives it with a synthetic truncated source.
func runMigrationsWithSource(ctx context.Context, dsn string, logger *slog.Logger, src source.Driver, opts ...RetryOption) error {
	cfg := newRetryConfig(opts...)
	target := redactDSN(dsn)
	// ... existing retry loop unchanged, still calling runMigrationsOnce(ctx, dsn, src) ...
}
```

- `source.Driver` is already imported (`migrate.go:15`). No new exported symbol.
  `migrationsFS` stays unexported (`migrate.go:21-22`).
- Production call site `db.RunMigrations(ctx, cfg.DatabaseURL, logger)`
  (`cmd/server/main.go:137`) is untouched.

**Ahead-of-source guard** — new helper + branch inside `runMigrationsOnce`
(`migrate.go:263-298`), placed after `migrate.NewWithInstance(...)` (line 279) and
**before** the `go func() { done <- m.Up() }()` block (lines 284-287). Verified
prototype from RESEARCH Finding 1 / §"Code Examples" lines 668-676:

```go
// maxSourceVersion walks a source.Driver to its highest migration version.
func maxSourceVersion(src source.Driver) (uint, bool) {
	v, err := src.First()
	if err != nil {
		return 0, false
	}
	for {
		next, err := src.Next(v)
		if errors.Is(err, os.ErrNotExist) {
			return v, true
		}
		if err != nil {
			return 0, false
		}
		v = next
	}
}

// inside runMigrationsOnce, after migrate.NewWithInstance:
if cur, dirty, verr := m.Version(); verr == nil && !dirty {
	if smax, ok := maxSourceVersion(src); ok && cur > smax {
		return nil // DB schema ahead of embedded source: rollback scenario, no-op (D-17)
	}
}
```

- `errors` is imported; add `os` to the import block (currently absent — `migrate.go:3-19`).
- `m.Version()` returns `migrate.ErrNilVersion` for a fresh DB → the `verr == nil`
  guard skips that case so a from-scratch boot still applies normally.
- `dirty && ahead` is deliberately left to error (`ErrDirty` today, stays).

**Error-handling convention already in file (preserve):** `runMigrationsOnce` wraps
every failure with `fmt.Errorf("<verb>: %w", err)` (lines 266, 271, 276, 281, 292);
`ErrNoChange` is treated as success (lines 291-294); DSN never logged raw —
`redactDSN`/`redactError` (lines 167-193).

---

### `internal/db/migrate_ahead_test.go` (NEW — `package db`, in-package)

**Analog:** `internal/db/migrate_test.go` (scratch-schema technique) +
`internal/testutil/postgres.go`.

**Package declaration:** must be `package db` (not `package db_test` like
`migrate_test.go:1`) to reach unexported `runMigrationsWithSource`.

**DB gating** (`internal/testutil/postgres.go:29-45`):
```go
dsn := testutil.RequirePostgresDSN(t) // skips on -short or missing TEST_DATABASE_URL
```

**Scratch-schema isolation** (`migrate_test.go:127-143`, `:159-195`) — copy `scratchSchemaDSN`
+ the drop/create/cleanup dance verbatim, with a distinct schema name (e.g.
`migrate_ahead_scratch`):
```go
func scratchSchemaDSN(dsn, schema string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
// setup:  DROP SCHEMA IF EXISTS migrate_ahead_scratch CASCADE ; CREATE SCHEMA ...
// t.Cleanup (LIFO — register sqlDB.Close first, then the drop): migrate_test.go:164-195
```

**Synthetic truncated source** (RESEARCH §"iofs over a truncated source" lines 202-212,
CONFIRMED working) — build `fstest.MapFS` with `NNNNNN_x.up.sql` / `.down.sql` entries
containing `SELECT 1;`, then `iofs.New(mapFS, "m")`:
```go
const n = 5
fullSrc := mapFSSource(t, n+1) // synthetic 000001..000006
nSrc    := mapFSSource(t, n)   // synthetic 000001..000005
```

**Test shape** (RESEARCH §"Code Examples" lines 680-703, §"Pattern 3" lines 380-387):
1. `runMigrationsWithSource(ctx, scratchDSN, discardLogger, fullSrc)` → prime scratch DB to `n+1`.
2. `runMigrationsWithSource(ctx, scratchDSN, discardLogger, nSrc)` → **assert `nil`** (RED against unmodified code — that RED is the SC #4 confirmation; GREEN after the guard).
3. Assert `SELECT version, dirty FROM migrate_ahead_scratch.schema_migrations` still `(n+1, false)` — pattern from `migrate_test.go:208-219`.
4. Negative case `_DirtyAheadStillErrors`: force `dirty=true` at `n+1`, assert `runMigrationsWithSource` still errors.

**Discard logger** (`migrate_test.go:202`): `slog.New(slog.NewTextHandler(io.Discard, nil))`.

---

### `cmd/migration-check/main.go` (NEW — stdlib CI guard)

**Analog:** `cmd/coverage-report/main.go`.

**Thin `main()` → `run()` with injected writer** (`coverage-report/main.go:40-45`):
```go
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("migration-check", flag.ContinueOnError)
	// ...
	if err := fs.Parse(args); err != nil {
		return err
	}
	// ...
}
```

**Flags** — mirror `coverage-report/main.go:48-60` style (`fs.String`, `fs.Bool`). Needs
roughly: `--added` (comma/newline migration file list from the `changes` job),
`--prev-tag` (`svu current` output), and a hidden test seam.

**Injected seams for tests** (RESEARCH §"Pattern 2" lines 369-378; mirrors
`coverage-report`'s `nowUTC` package-var clock at `main.go:37-38`):
```go
// package var so main_test.go can stub it without a real git repo
var gitShow = func(tag, path string) ([]byte, error) {
	return exec.Command("git", "show", tag+":"+path).Output() // argv slice, never sh -c
}
```

**Input validation before shelling `git`** (RESEARCH §"Security Domain" lines 838, 849):
validate tag against `^v?[0-9]+(\.[0-9]+){0,2}(-[0-9A-Za-z.-]+)?$`, path against a fixed
allowlist (`queries/*.sql`, `internal/db/migrations/*.up.sql`, `internal/db/sqlc/*.go`).
The `expand-shipped-in` annotation value is attacker-influenceable — validate hard.
Pattern precedent: `coverage-report/main.go:82-96` `validSHA` (char-by-char allowlist, the
only gate by which an external string reaches output).

**Comment/statement scanning** — hand-rolled, stdlib only (`bufio`, `strings`, `regexp`),
matching `coverage-report`'s line-by-line hand parser (`main.go:164-235`). RESEARCH
§"D-08" lines 463-471 gives the mechanics: strip `--` and `/* */`, split on `;`
respecting `'…'` and `$tag$…$tag$`, uppercase-copy for keyword match, keep original for
identifiers + 1-based line numbers.

**Two finding classes with distinct messages** (RESEARCH §"D-08" lines 495-498):
*backward-incompatible* (DROP/RENAME/type-narrow/SET NOT NULL/ADD CHECK — names the
expand/contract + N-1 rule, points at `internal/db/migrations/README.md`) and
*unsafe-forward* (`ADD COLUMN … NOT NULL` no `DEFAULT` — explains non-empty-table
lock/failure, points at README's NOT-NULL note).

**Scan `*.up.sql` ONLY** — never `*.down.sql` (RESEARCH Anti-Patterns line 392 / Pitfall C:
every down file has `DROP TABLE`/`DROP COLUMN`; the app never runs `Down()`).

**D-15 previous-release query cross-reference** — the largest sub-feature. Extraction
approach and blind-spot matrix: RESEARCH §"D-15" lines 400-455. High-confidence positions
only for deterministic-red (explicit `INSERT (...)`/`SELECT` lists, `ON CONFLICT`,
`RETURNING`, qualified `alias.col`, single-table `*`); leave ambiguous to the boot job
(annotation cannot override this check, so a false-red is unrecoverable).

---

### `cmd/migration-check/main_test.go` (NEW — `package main` whitebox)

**Analog:** `cmd/coverage-report/main_test.go`.

**Whitebox declaration + rationale comment** (`coverage-report/main_test.go:1-5`):
```go
package main
// Whitebox tests: package main so it can drive the unexported run function
// and the pure parse/render helpers directly (mirrors cmd/server/main_test.go).
```

**Golden-file `-update` flag** (`coverage-report/main_test.go:19`, `:59-72`) — use for
pinning failure-message wording:
```go
var update = flag.Bool("update", false, "rewrite golden files from current output")
// goldenPath := filepath.Join("testdata", golden); if *update { os.WriteFile(...) }
```

**Stub the git seam** instead of a real repo:
```go
func withStubGitShow(t *testing.T, fn func(tag, path string) ([]byte, error)) {
	t.Helper()
	prev := gitShow
	gitShow = fn
	t.Cleanup(func() { gitShow = prev })
}
```
(same package-var swap + `t.Cleanup` restore pattern as `withFixedClock`,
`coverage-report/main_test.go:21-27`).

**Table-driven cases with SQL fixtures in `testdata/`** (RESEARCH §"Pattern 2" line 378;
`coverage-report` reads fixtures via `filepath.Join("testdata", name)`).

---

### `cmd/migrate/main.go` (NEW — `go run` HEAD-schema helper; name at discretion)

**Analogs:** `cmd/server/main.go:1-4, 137-139` (migration sequencing) +
`cmd/coverage-report/main.go:40-45` (thin main shape).

**Shape** (RESEARCH §"GitHub Actions Wiring" line 571, §"Open Questions" 3 lines 754-756):
a tiny dedicated `cmd/` package — keeps `cmd/server`'s boot path untouched, trivially
`go run`-able. Reads only `DATABASE_URL` from env (not argv), so **G304 likely does not
apply** — confirm during planning.

```go
package main

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := db.RunMigrations(ctx, dsn, logger); err != nil { // same path cmd/server uses
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
```
Error wrap `"run migrations: %w"` is copied verbatim from `cmd/server/main.go:138`.

---

### `.golangci.yml` (MODIFY — G304 carve-out)

**Analog:** the existing `cmd/coverage-report` carve-out (`.golangci.yml:37-44`), exact
copy with the path swapped:
```yaml
      - path: '^cmd/migration-check/'
        linters:
          - gosec
```
Add directly after the `^cmd/coverage-report/` block (line 44). Keep the explanatory
comment style (2-4 lines, cites D-19, "CI-controlled, not user input"). `cmd/migrate`
only reads env — add it to the carve-out only if the planner finds it opens an argv path.

---

### `Makefile` (MODIFY — COVER_PKGS exclusion)

**Analog:** the existing anchored `grep -vE` at `Makefile:36`:
```make
COVER_PKGS = $(shell go list ./... | grep -vE '(^|/)(internal/db/sqlc|cmd/coverage-report)$$' | paste -sd, -)
```
Extend the alternation (RESEARCH §"Wave 0 Gaps" line 822):
```make
COVER_PKGS = $(shell go list ./... | grep -vE '(^|/)(internal/db/sqlc|cmd/coverage-report|cmd/migration-check)$$' | paste -sd, -)
```
Keep the anchoring `(^|/)...$$` (doubled `$$` mandatory — see the comment at
`Makefile:22-35`). Add `cmd/migrate` too **only if** it ships without its own `_test.go`.
`cmd/migration-check` keeps its own tests and still compiles under `vet`/`lint`.

---

### `.github/workflows/full-pipeline.yml` (MODIFY — 3 new jobs + needs wiring)

**All new jobs:** `permissions: contents: read` only (top-level default, `line 7-8`;
public ghcr package, no `packages: write`). SHA-pin any new `uses:` with a trailing
`# vX.Y.Z` comment (convention throughout the file, e.g. `line 20`). Dynamic values
cross into `run:` via `env:`, never string-interpolated (`coverage-comment` job,
lines 254-268; `release` job `BASELINE_SHA` line 78).

**Checkout action (verbatim, pin in use everywhere):**
```yaml
      - name: Checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
```

**Postgres bring-up analog — `test` job (`lines 43-54`):** currently `make test-integration`
→ `make db-up` (`Makefile:44-45` = `docker compose up -d --wait postgres`). RESEARCH
Finding 2 / Pitfall D (lines 240, 642) says do NOT use `services: postgres` on `n1-boot`
(starts on every push, ignores step `if:`). Instead an `if:`-gated `docker run -d
--network host ... postgres:16` step + a `pg_isready` wait loop. DSN literal
`postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable`
matches `Makefile:9` `TEST_DATABASE_URL`.

**`svu` resolution analog — `release` job (`lines 362-381`), copy this exactly:**
```yaml
      - name: Checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0
      - name: Set up Go
        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: Compute next version with svu
        run: |
          set -e
          go install github.com/caarlos0/svu/v3@v3.4.1
          SVU="$(go env GOPATH)/bin/svu"
          CURRENT="$("$SVU" current)"
```
For the new jobs add `fetch-tags: true` under `with:` alongside `fetch-depth: 0` (S8 hard
requirement, RESEARCH lines 540, 588). `n1-boot` uses `svu current` (not `next`).

**`changes` prelude job** — full skeleton in RESEARCH §"`changes` prelude job" lines
525-564: `fetch-depth: 0` + `fetch-tags: true`, `outputs: {migrations_changed, migration_files}`,
diff-base bash (`$BASE_REF` present → PR merge-base `origin/${BASE_REF}...HEAD`;
`$BEFORE` all-zeroes/unreachable → `git merge-base origin/main HEAD...HEAD`; else
`${BEFORE}...${SHA}`), `git diff --name-only --diff-filter=AM "$range" -- 'internal/db/migrations/*.up.sql'`.

**`n1-boot` job** — step list in RESEARCH §"`n1-boot` job" lines 566-580. Every expensive
step (`svu`, postgres `docker run`, `go run ./cmd/migrate`, `docker pull`/`run` N-1 image,
health/read/write probes) carries `if: needs.changes.outputs.migrations_changed == 'true'`.
**Job stays unconditional — no job-level `if:`** (RESEARCH Finding 2 / Pitfall B lines
216-242, 636: a skipped `needs:` job skips `build-scan` and `release`). D-04 skip-green is
an in-job step (`echo "::notice::..."; exit 0`), not a job `if:` (D-19a).

N-1 image run env: `-e DATABASE_URL=<dsn> -e HTTP_PORT=8080`, **no `INSTANCE_PASSPHRASE`**
(only `DATABASE_URL` is `notEmpty` in `internal/config`; GATE-07 inert path). Probes:
`/health` sustained 200 → `GET /watchlist` (`200 []`) → `GET /events`
(`200 {events:[],...}`) → `POST /watchlist` with `image_url` set (bypasses art matcher —
pure DB write, no external call) → `201` → re-`GET /watchlist` asserts row present.

**`build-scan.needs:` (`line 300`)** — current:
```yaml
    needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]
```
Append `migration-check` and `n1-boot` (and `changes` if `build-scan` reads its outputs;
otherwise `changes` is transitive). Both appended jobs unconditional. The comment at
`lines 297-299` is the precedent for documenting a `needs:` append.

**Report-only anti-pattern — do NOT follow `coverage-comment` (`lines 186-282`):** D-11
says these block; `coverage-comment` joins no release-path `needs:` graph. The relevant
pattern from that job is only the env-not-interpolate discipline.

---

### `internal/db/migrations/README.md` (NEW — no in-repo prose analog)

The repo has **no** hand-written README.md outside `node_modules`. Structure is fully
specified in RESEARCH §"Rule Documentation" lines 502-519 (five sections: two rules stated
separately per S4; the N-1 invariant; concrete expand→backfill→contract walkthrough;
"before you merge a migration" checklist sourced from `PITFALLS.md` Pitfall 8; what
`cmd/migration-check` enforces + annotation syntax).

**In-tree precedent to cite in the walkthrough:**
`internal/db/migrations/000007_backfill_events_watched_artist_name.up.sql` — an idempotent
`UPDATE events e SET watched_artist_name = a.name FROM artists a WHERE a.id = e.artist_id
AND e.watched_artist_name IS NULL` (the `WHERE … IS NULL` makes it safe to re-run and a
no-op on a fresh DB). `000006` added the nullable column (expand); `000007` is the
backfill; a future release could contract.

**Optional presence test** (RESEARCH line 807, on-brand — repo has `.env.example` parity
tests): `go test ./internal/db -run TestMigrationsReadmeMentionsRules` asserting key
phrases present.

---

### `.claude/CLAUDE.md` (MODIFY — one-line pointer)

**Analog:** the existing "Definition of Done — run before every commit" numbered list
(items 1-6). Add one line pointing migration authors at
`internal/db/migrations/README.md` before adding/editing a migration (D-09). Keep it to
one line — CLAUDE.md's own "Comment discipline" section rewards brevity.

---

## Shared Patterns

### Error wrapping (Go)
**Source:** `internal/db/migrate.go` throughout (`fmt.Errorf("<verb>: %w", err)`),
`cmd/coverage-report/main.go:103-111`.
**Apply to:** `internal/db/migrate.go` seam, `cmd/migration-check`, `cmd/migrate`.
Every returned error names the operation and wraps the cause with `%w`.

### Thin `main()` → testable `run()`
**Source:** `cmd/coverage-report/main.go:40-80`, `cmd/server/main.go` (per its test comment).
**Apply to:** `cmd/migration-check/main.go`, `cmd/migrate/main.go`.
```go
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```
`flag.NewFlagSet(name, flag.ContinueOnError)`; return `fs.Parse` errors.

### Package-var seam stubbed by `t.Cleanup`
**Source:** `cmd/coverage-report/main.go:37-38` (`nowUTC`) + `main_test.go:21-27`
(`withFixedClock`).
**Apply to:** `cmd/migration-check` (`gitShow`), `internal/db/migrate_ahead_test.go` is
in-package so it needs no seam — it calls `runMigrationsWithSource` directly.

### Real-Postgres test gating + scratch-schema isolation
**Source:** `internal/testutil/postgres.go:29-45` (`RequirePostgresDSN`),
`internal/db/migrate_test.go:127-143` (`scratchSchemaDSN` via `search_path` query param),
`:159-195` (drop/create/`t.Cleanup` LIFO).
**Apply to:** `internal/db/migrate_ahead_test.go`. Distinct fixed schema name; one test
binary per package makes a fixed name safe.

### Untrusted-string allowlist gate before it reaches output or a subprocess
**Source:** `cmd/coverage-report/main.go:82-96` (`validSHA`, char-by-char).
**Apply to:** `cmd/migration-check` — tag regex + path allowlist before any
`exec.Command("git", ...)`; argv slice form, never `sh -c` (RESEARCH §"Security Domain").

### CI: SHA-pinned actions + `# vX.Y.Z` comment; `contents: read`; env not interpolation
**Source:** `.github/workflows/full-pipeline.yml` throughout (`line 20`, `line 7-8`,
`lines 254-268`).
**Apply to:** all three new jobs.

### Anchored `grep -vE` / scoped linter carve-out for a CI helper package
**Source:** `Makefile:36`, `.golangci.yml:37-44`.
**Apply to:** `Makefile` COVER_PKGS + `.golangci.yml` for `cmd/migration-check`.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/db/migrations/README.md` | prose doc | — | No hand-written README exists anywhere in the repo tree (only vendored `node_modules` copies). Structure comes from `16-RESEARCH.md` §"Rule Documentation" (lines 502-519); content anchor is `000007_backfill_events_watched_artist_name.up.sql`. |

Partial-only match (planner should lean on RESEARCH for the delta):

- `cmd/migration-check` SQL scanner + `git show` cross-reference: `cmd/coverage-report`
  gives the *structure* (thin main, whitebox tests, golden files, injected clock/seam,
  stdlib-only hand parser) but its input is a coverage profile, not SQL. The
  tokenizer/DDL-pattern logic and the D-15 identifier extraction are net-new — see
  RESEARCH §"D-08" (lines 459-498) and §"D-15" (lines 400-455) including the blind-spot
  matrix.
- `cmd/migrate` HEAD-schema helper: no existing standalone `go run` DB helper.
  `cmd/server/main.go:137-139` shows the `db.RunMigrations` call + error-wrap; the thin
  entrypoint shape is `cmd/coverage-report`.

---

## Metadata

**Analog search scope:** `internal/db/`, `internal/testutil/`, `cmd/`,
`.github/workflows/`, repo root (`Makefile`, `.golangci.yml`), `internal/db/migrations/`.
**Files scanned:** `internal/db/migrate.go`, `internal/db/migrate_test.go`,
`internal/testutil/postgres.go`, `cmd/coverage-report/main.go`,
`cmd/coverage-report/main_test.go`, `cmd/server/main.go`,
`.github/workflows/full-pipeline.yml`, `Makefile`, `.golangci.yml`, plus both upstream
artifacts (`16-CONTEXT.md`, `16-RESEARCH.md` in full).
**Pattern extraction date:** 2026-09-04
