# Phase 16: Rollback-Safe Migrations - Research

**Researched:** 2026-09-04
**Domain:** golang-migrate boot-time behaviour on an ahead-of-source schema; static SQL/DDL analysis in stdlib Go; GitHub Actions job graph wiring for an N-1 image-boot check
**Confidence:** HIGH for the golang-migrate finding (executed falsification against the pinned module) and repo integration points (read directly this session); HIGH for the GitHub Actions skip-propagation correction (GitHub docs); MEDIUM for the exact conservatism tuning of the two hand-rolled scanners

---

## Summary

The milestone's one MEDIUM-confidence assumption — that an older binary's boot `migrate.Up()` **no-ops** (`ErrNoChange`) against a `schema_migrations` version ahead of its embedded source — is **false** for the pinned stack (`golang-migrate/migrate/v4 v4.19.1`, `iofs` source, `migrate.Up()`). Verified this session by executing the real library: `Up()` returns a hard error (`no migration found for version <N+1>: read down for version <N+1> ...`, wrapping `os.ErrNotExist`), which the current `runMigrationsOnce` treats as a fatal error. The rollback-safety model as written in `16-CONTEXT.md` and `ARCHITECTURE.md` therefore does **not** hold: rolling back the image would put the old binary into the Pitfall-9 boot crash-loop. This changes the phase's scope — it now needs a small **behaviour change** to `internal/db/migrate.go` (make `RunMigrations` no-op when the DB is ahead of the embedded source), not just the "behaviour-preserving refactor" the context assumed. A working prototype of the fix is included and verified.

A second correction: `16-CONTEXT.md` D-12/S6 assumes "a skipped job in `needs:` counts as success, so the gate is intact." GitHub's documented behaviour is the opposite — **if a needed job is skipped, the dependent job is skipped too** (implicit `if: success()`). Adding a job-level `if:` to the N-1 boot job and leaving it in `build-scan.needs:` would skip `build-scan` (and `release`) on every non-migration push. The fix is to keep the boot job unconditional and gate its expensive **steps**, not the job.

The static guard (`cmd/migration-check`) and the previous-release query cross-reference are both feasible as a single stdlib-only Go tool following the `cmd/coverage-report` template. cgo-based SQL parsers (`pg_query_go`) are out (documented cgo break on the dev box); a hand-rolled comment-stripping, statement-splitting token scanner over `*.up.sql` and `queries/*.sql` is the pragmatic and on-brand choice. Its known blind spots are enumerated below; the N-1 boot job is the behavioural backstop for everything the scanner cannot see.

**Primary recommendation:** Sequence the `internal/db/migrate.go` ahead-of-source fix + its hermetic test as **task 1** and treat the RED test as a checkpoint (it is expected to fail against today's code — that failure *is* the confirmation the assumption was wrong; the GREEN fix closes it). Do not add a job-level `if:` to the boot job. Build `cmd/migration-check` from the `cmd/coverage-report` mould.

---

## User Constraints (from CONTEXT.md)

### Locked Decisions

Copied from `16-CONTEXT.md` `<decisions>` and `<revisions>` (the revision wins on any conflict). Abbreviated; the planner must read the source file.

- **D-01:** CI brings the throwaway Postgres to the current branch's schema with a `go run` helper that calls `internal/db.RunMigrations` — not a `docker build`, not the `golang-migrate` CLI.
- **D-02 (revised S3):** SC #4 is proven by a dedicated hermetic Go test in `internal/db`, running every CI run, that calls the new unexported `runMigrationsWithSource` seam (D-18) with a synthetic truncated source over a real throwaway DB migrated to N+1, and asserts `nil`. Plan it **task 1**, treat as a checkpoint — no plan B.
- **D-03 (revised S1):** N-1 boot check = poll `/health` to sustained 200, then `GET /watchlist` (`200 []`), `GET /events` (`200 {events:[]}`), then `POST /watchlist` (minimal body), assert 2xx, re-read `GET /watchlist`, assert the row is present. N-1 image runs with no `INSTANCE_PASSPHRASE`.
- **D-04:** Skip-green only when `svu current` finds no prior release tag at all (true bootstrap). Prior tag exists but pull fails → red. One pull retry before failure.
- **D-05:** Static SQL guard is distinct from the N-1 boot job; both required.
- **D-06:** Guard is a small, stdlib-only, unit-tested Go tool at `cmd/migration-check/`, mirroring `cmd/coverage-report`. Not `squawk`. Not inline bash/grep.
- **D-07 (revised S2/S5):** Guard is diff-scoped against a base computed once by the `changes` prelude job. Destructive statement in a branch-new migration is red unless it carries `-- migration-check:allow-destructive expand-shipped-in=vX.Y.0 reason=<text>` (both keys required, echoed to output). `expand-shipped-in` is validated: must be a real tag; warn if older than `svu current`.
- **D-08 (revised S4):** Guard flags two finding classes with distinct messages — *backward-incompatible* (`DROP COLUMN`, `DROP TABLE`, `RENAME` column/table, type-narrowing `ALTER COLUMN ... TYPE`) and *unsafe-forward* (`ADD COLUMN ... NOT NULL` without `DEFAULT`). Does **not** enforce "no blocking DDL" — that stays a written rule.
- **D-09:** Rule home is `internal/db/migrations/README.md`, with one-line pointers from CLAUDE.md's Definition of Done section and the `cmd/migration-check` failure message. No `docs/adr/`.
- **D-10 (revised S4):** README states the two rules separately; walkthrough built around `000007_backfill_events_watched_artist_name`; a "before you merge a migration" checklist; a section on what the tool enforces + the annotation syntax.
- **D-11 (revised S6/S7):** Both checks block a merge — appended to `build-scan`'s `needs:`. Transient-ghcr risk on a migration PR is explicitly accepted.
- **D-12 (revised S2/S6):** `changes` job + static guard run unconditionally; the N-1 boot job's real steps run only when a migration file changed. "Merge-base in all cases" withdrawn. **(See the correction in `## Common Pitfalls` — the "skipped job counts as success" premise is wrong; gate the steps, not the job.)**
- **D-13 (revised S5):** Previous released image resolved with `svu current` on a `fetch-depth: 0` + tags checkout → `ghcr.io/danielrpof/drop-tracker:<tag>`. That single tag is the only rollback target and drives D-15's cross-reference.
- **D-14 (revised S8):** `services: postgres` on the runner; `go run` helper applies HEAD schema to `localhost:5432`; `docker run --network host` the N-1 image with `DATABASE_URL` + `HTTP_PORT` (not `PORT`), no `INSTANCE_PASSPHRASE`; `curl localhost:$HTTP_PORT` for assertions. No docker-compose file added.
- **D-15 (new S1):** `cmd/migration-check` also resolves the previous release tag, reads that tag's `queries/*.sql` via `git show <tag>:queries/<f>.sql`, builds the set of `(table, column)` identifiers those queries reference (`SELECT *`/`RETURNING *` = "every column of that table"), and goes **red** for every `DROP COLUMN`/`RENAME COLUMN`/`DROP TABLE`/`RENAME TABLE` in a branch-new migration whose object is still referenced — **regardless of the `allow-destructive` annotation**. No prior tag → sub-check skipped. Prior tag but `git show` fails → red.
- **D-16 (new S2):** A `changes` prelude job (no services, `fetch-depth: 0`, fetch tags) outputs the new-migration file list + `migrations_changed`. `pull_request` → diff `origin/${{ github.base_ref }}...HEAD`. `push` → diff `${{ github.event.before }} ${{ github.sha }}`; all-zeroes `before` → merge-base against `origin/main`.
- **D-17 (new S5):** Phase 17 auto-rollback MUST target exactly the immediately-previous release tag. Recorded here; locked in Phase 17.
- **D-18 (new S3):** Add `func runMigrationsWithSource(ctx context.Context, dsn string, logger *slog.Logger, src source.Driver, opts ...RetryOption) error` holding the current `RunMigrations` body from the `newRetryConfig` line onward. `RunMigrations` keeps its exact exported signature; builds the embedded `iofs` source and delegates. `migrationsFS` stays unexported.
- **D-19 (new S8):** (a) D-04 skip-green is an in-job step, not a job-level `if:` (dead code on this repo — accepted). (b) `cmd/migration-check` gets the `gosec` G304 carve-out and `COVER_PKGS` exclusion — confirmed. (c) `GET /watchlist` `200 []` and `GET /events` `200 {events:[]}` on empty DB verified.

### Claude's Discretion

- Exact names for the new `cmd/` tool and the `go run` migration helper.
- The precise `allow-destructive` annotation grammar (tag-validation of `expand-shipped-in` is decided: yes).
- Whether the `changes` prelude, guard, and N-1 boot job are three jobs or the guard folds into `changes`. N-1 boot stays separate.
- The `changes` job's diff mechanism (`dorny/paths-filter` SHA-pinned vs. plain `git diff --name-only`).
- How D-15 extracts referenced `(table, column)` identifiers and where its blind spots are (scoped below).
- Health-poll timing (attempt count, interval, sustained-green window).
- Whether `cmd/migration-check` needs the `.golangci.yml` `gosec` carve-out (D-19b says yes — confirmed below).
- Whether `cmd/migration-check` is excluded from `COVER_PKGS` (D-19b says yes — confirmed below).
- Exact failure-message wording (must name the expand/contract rule and point at the README).
- Whether the two new jobs are one combined job or two separate jobs.

### Deferred Ideas (OUT OF SCOPE)

- Postgres backup + restore procedure → Phase 17 provisioning runbook.
- Machine-enforced "no blocking DDL" (non-`CONCURRENTLY` `CREATE INDEX`, table-rewriting `ALTER`) → written rule only.
- `down` migration authoring + a `migrate down` rollback path → the app is forward-only by design.
- A `docs/adr/` ADR for the expand/contract decision → rejected (D-09).
- Poller leader-election via Postgres advisory lock (Pitfall 10) → scaling concern, not yet.
- Multi-hop rollback (N-2+) → this phase verifies exactly one hop.
- `dirty`-`schema_migrations` recovery path → Phase 17 restore territory.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MGRT-01 | A CI check boots the previously-released image against the current branch's schema and fails the build if the older binary cannot start and stay healthy (backward-compat / N-1 check) | `## GitHub Actions Wiring` (the `changes` → boot-job design, tag resolution, health-poll + write probe); `## Finding 1` (the boot migration itself must be made ahead-of-source-safe or the old image never starts — this is a precondition for MGRT-01 being satisfiable at all); `## Finding 2` (how to keep the gate blocking without skip-propagation breaking `build-scan`) |
| MGRT-02 | The repo documents the expand/contract migration rule (additive-only per release, destructive changes split across releases, no blocking DDL in boot migrations) as a standing constraint | `## Rule Documentation` (README structure, the `000007` walkthrough, the two-class framing from S4); `## Static DDL Detection` (what the tool enforces vs. what stays prose); `PITFALLS.md` Pitfall 8 "How to avoid" list is the checklist |

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| "Old binary tolerates a newer schema" | App boot path (`internal/db/migrate.go`) | — | The rollback image runs `RunMigrations` at boot; the tolerance must live in that code, not in CI. CI only *verifies* it. This is the scope change Finding 1 forces. |
| N-1 image boots & stays healthy against HEAD schema | CI / GitHub Actions | App HTTP handlers (`/health`, `/watchlist`, `/events`) | Behavioural end-to-end check; exercises the real image against a real Postgres. |
| Deterministic every-run "no-op vs. error" guarantee | `internal/db` Go test | CI (runs the test) | D-02: the deterministic guarantee is a unit/integration test, the boot job is real-world confirmation on top. |
| Static destructive-DDL detection | `cmd/migration-check` (stdlib Go) | N-1 boot job (backstop for what patterns miss) | D-05/D-06/D-08. |
| Previous-release query cross-reference | `cmd/migration-check` (shells `git show`) | N-1 boot job (`POST /watchlist` write probe) | D-15. Deterministic counterpart to the behavioural check. |
| Diff-base computation | `changes` prelude job (GitHub Actions) | — | D-16. Computed once, consumed by guard + boot job. |
| Expand/contract rule as prose | `internal/db/migrations/README.md` | CLAUDE.md pointer, tool failure message | D-09/D-10. |

---

## Finding 1 (CRITICAL): `migrate.Up()` against an ahead-of-source schema ERRORS — it does not `ErrNoChange`

This is the milestone's single MEDIUM-confidence assumption (SC #4). **It is refuted.** `ARCHITECTURE.md` §"Boot-time migrations × rollback" states:

> "`migrate.Up()` asks the source for the migration after the current DB version, the source driver returns `os.ErrNotExist` ... and `Up()` returns `ErrNoChange` — **not** an error."

That is not what `golang-migrate/migrate/v4 v4.19.1` does.

### Mechanism (read from the pinned module source)

`[VERIFIED: golang-migrate/migrate/v4@v4.19.1/migrate.go:265-283, 532-541, 776-810 — read this session]`

- `Migrate.Up()` reads `curVersion, dirty, _ := m.databaseDrv.Version()` then, if `dirty`, returns `ErrDirty{curVersion}` immediately (line 275-277); otherwise `go m.readUp(curVersion, -1, ret)`.
- `readUp(from, -1, ...)` — with `from = curVersion` (e.g. `N+1`) and `from >= 0` — first calls `m.versionExists(uint(from))` (line 536-541). On any error it does `ret <- err; return`.
- `versionExists(version)` (line 776) calls `m.sourceDrv.ReadUp(version)` then `m.sourceDrv.ReadDown(version)`. For an `iofs` source truncated to `1..N`, migration `N+1` is in neither index, so both return `*fs.PathError{Err: fs.ErrNotExist}`. `versionExists` then returns `fmt.Errorf("no migration found for version %d: %w", version, err)` (line 807), wrapping `os.ErrNotExist`.
- Net: `Up()` returns a non-nil error, `errors.Is(err, os.ErrNotExist) == true`, `errors.Is(err, migrate.ErrNoChange) == false`.
- The `iofs` `ReadUp`/`ReadDown` returning `fs.ErrNotExist` for an absent version: `[VERIFIED: golang-migrate/migrate/v4@v4.19.1/source/iofs/iofs.go:128-158 — read this session]`.

### Falsification test (executed this session)

Ran a standalone program inside the repo module (`iofs.New(fstest.MapFS{…}, "m")` + `database/stub` with `SetVersion`) against `v4.19.1`:

```
dbVersion=7 sourceMax=7  => Up() err=no change                                           | ErrNoChange=true
dbVersion=8 sourceMax=7  => Up() err=no migration found for version 8: read down for
                            version 8 m: file does not exist                             | ErrNoChange=false
dbVersion=10 sourceMax=7 => Up() err=no migration found for version 10: read down for
                            version 10 m: file does not exist                            | ErrNoChange=false
dbVersion=5 sourceMax=7  => Up() err=<nil>   (migrates 6,7 normally)
```

Error-identity check on the `dbVersion=8, sourceMax=7` case:

```
errors.Is(err, os.ErrNotExist)      = true
errors.Is(err, migrate.ErrNoChange) = false
```

`dirty=true, dbVersion=8, sourceMax=7` → `Up()` returns `"Dirty database version 8. Fix and force version."` (the dirty check precedes `readUp`, so a dirty ahead-of-source DB is caught as `ErrDirty` regardless).

### Impact if left unaddressed

`internal/db/migrate.go` `runMigrationsOnce` treats anything other than `migrate.ErrNoChange` as a hard failure `[VERIFIED: internal/db/migrate.go:291-294 — read this session]`. So on rollback the old image would:

1. `RunMigrations` → `runMigrationsOnce` → `m.Up()` returns `"no migration found for version N+1"` → wrapped as `apply migrations: …` → not a context error → retried 6× (Default: `DefaultMaxAttempts = 6`) → `RunMigrations` returns `"migrations failed after 6 attempts …"`.
2. `cmd/server/main.go` `run()` returns `fmt.Errorf("run migrations: %w", err)` → `os.Exit(1)` `[VERIFIED: cmd/server/main.go:137-139 — read this session]`.
3. `restart: unless-stopped` → identical failure forever = **Pitfall 9 crash loop.** The N-1 rollback the whole milestone is built on does not work.

The N-1 boot job (MGRT-01) would also just go red for every migration PR, since the old image can never boot against HEAD's schema — the check would be un-passable, not a useful gate.

### Recommended fix (prototype verified this session)

Make `RunMigrations` no-op when the DB `schema_migrations` version is **ahead of** the embedded source's max version. This is a small, well-scoped behaviour change to the app boot path — and it is exactly the "boot-time migration must be idempotent and forward-only; ensure an older image seeing a newer version does nothing" item from `PITFALLS.md` Pitfall 8, which the research explicitly flagged as "verify this is the actual behaviour and not an error in your wiring."

Prototype (executed, all four cases correct):

```go
// maxSourceVersion walks a source.Driver to its highest migration version.
// Returns (0, false) if the source is empty or errors.
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
```

Then, inside `runMigrationsOnce` after `migrate.NewWithInstance(...)` and **before** the `m.Up()` goroutine:

```go
cur, dirty, verr := m.Version() // m.Version() returns migrate.ErrNilVersion for a fresh DB
if verr == nil && !dirty {
	if smax, ok := maxSourceVersion(src); ok && cur > smax {
		// DB schema is ahead of this binary's embedded migrations: a rollback
		// scenario. The old binary must operate the schema it finds, never try
		// to reconcile it. Treat as success (D-17: rollback is strictly N-1).
		return nil
	}
}
```

Prototype run with this guard in place:

```
dbVersion=7  sourceMax=7  => Up() err=no change           (unchanged)
dbVersion=8  sourceMax=7  => FIX: DB ahead, skipping Up() -> nil
dbVersion=10 sourceMax=7  => FIX: DB ahead, skipping Up() -> nil
dbVersion=5  sourceMax=7  => Up() err=<nil>  (still migrates 6,7)   (unchanged)
```

Notes for the planner:

- `m.Version()` — `[VERIFIED: golang-migrate/migrate/v4@v4.19.1/migrate.go:381-394]` — returns `(suint(v), dirty, nil)` for an applied version, `(0,false,ErrNilVersion)` for a fresh DB. Guard on `verr == nil` to skip the fresh-DB case (there `Up()` applies from scratch as normal).
- `dirty && ahead` is deliberately **left to error** (it returns `ErrDirty` today; that stays). A half-applied forward migration blocking the old binary is Phase 17 restore territory (D-02 caveat) — do not paper over it.
- Cross-check option (do not rely on it as primary): after `m.Up()`, `errors.Is(upErr, os.ErrNotExist)` also uniquely identifies the ahead-of-source case on the `Up()` path (a legitimate forward migration never fails `versionExists`, because the current DB version always exists in the source). The explicit version comparison is preferred — it is self-documenting and does not depend on golang-migrate's internal error wrapping, which is `fmt.Errorf`, not a stable sentinel.
- This does not change the production forward path at all: on every normal boot `cur <= smax`, so the guard is inert.

### `iofs` over a truncated / synthetic source — CONFIRMED

D-02 asks the planner to "confirm an `iofs` source can be built over a subset of the embedded FS without exporting `migrationsFS`."

`[VERIFIED: executed this session]` — `iofs.New(fstest.MapFS{...}, "m")` works directly; `iofs.New` accepts any `fs.FS`. Options, all stdlib, none requiring `migrationsFS` to be exported:

- `testing/fstest.MapFS` — build `1..N` and `1..N+1` maps of `NNNNNN_x.up.sql` / `.down.sql` entries with trivial `SELECT 1;` bodies. **Recommended for D-02** — fully synthetic, deterministic, no dependency on the real migration count (which is 7 today and will move).
- `fs.Sub(embeddedFS, "subdir")` — returns an `fs.FS`; usable if a real subset is wanted.
- A temp dir + `os.DirFS`.

The D-18 seam takes `src source.Driver` (what `iofs.New` returns), so the test constructs its own `iofs` source from a `fstest.MapFS` and passes it straight in — no reach into `internal/db` internals beyond the seam.

---

## Finding 2 (CRITICAL): a skipped `needs:` job SKIPS its dependents — the D-12/S6 "counts as success" premise is wrong

`16-CONTEXT.md` D-12 / S6 states: *"a skipped job in `needs:` counts as success, so the gate is intact"* and proposes a **job-level `if: needs.changes.outputs.migrations_changed == 'true'`** on the N-1 boot job while keeping it in `build-scan.needs:`.

GitHub's documented behaviour contradicts this:

`[CITED: docs.github.com/actions/using-jobs/using-jobs-in-a-workflow and .../using-conditions-to-control-job-execution]` — a job with `needs:` has an implicit `if: success()`. **"If a job fails or is skipped, all jobs that need it are skipped unless the jobs use a conditional expression that causes the job to continue"** (e.g. `always()`, `!cancelled()`, `!failure()`). `success()` does **not** treat a skipped `needs` job as satisfied.

Corroborated by long-standing community reports: `[CITED: github.com/orgs/community/discussions/45058, /26945, /25224]`.

### Impact if implemented as D-12 describes

`build-scan` currently has `needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]` and no `if:` `[VERIFIED: .github/workflows/full-pipeline.yml:296-300 — read this session]`. Append a job-level-`if:`-gated `n1-boot`, and on the ~95 % of pushes that touch no migration, `n1-boot` is **skipped** → `build-scan` is **skipped** → `release` (`needs: [build-scan]`) is **skipped**. The entire release pipeline stops running except on migration PRs. This is a worse version of the exact "non-blocking for a whole phase then forgot to wire it" defect the repo already shipped once (Phase 8→9).

### Recommended design

Keep both new jobs **unconditional** (no job-level `if:`), in `build-scan.needs:`. Gate the *expensive work* at the **step** level:

- **`changes`** — always runs. Cheap: one checkout + `git diff`. Emits `migrations_changed` and the file list as job outputs.
- **`migration-check`** (guard) — always runs. `go run ./cmd/migration-check` is cheap; it internally no-ops when `changes.outputs.migrations_changed == 'false'` (nothing to scan). Always reaches a green conclusion.
- **`n1-boot`** — always runs, but every expensive step carries `if: needs.changes.outputs.migrations_changed == 'true'`: `svu` install + tag resolve, Postgres bring-up, HEAD-schema migrate, `docker pull`/`run`, health poll, write probe. When no migration changed, all those steps skip and the job succeeds trivially in a few seconds (runner spin-up only). D-19(a)'s "in-job step that detects true-bootstrap and exits 0" fits this same pattern.

A skipped **step** does not skip the job or its dependents — only a skipped **job** does. This preserves D-12's intent (real work only on migration PRs) and the blocking gate, without the skip-propagation break.

If `services: postgres` is declared at job level it will start on every push (service containers ignore step `if:`). To avoid ~10 s of Postgres startup on non-migration pushes, prefer bringing Postgres up inside an `if:`-gated step (`docker run -d ... postgres:16 ...` or reuse `make db-up`'s `docker compose up -d --wait postgres` pattern) rather than the `services:` key. This also matches the repo's existing Postgres-in-CI approach (`test` job → `make test-integration` → `make db-up`).

The planner should still smoke-test the skip behaviour on a scratch branch before relying on it (Phase 9 set the precedent — it verified `build-scan`/`release` skip semantics live on `test/coverage-gate-ci-check`, never `main`).

---

## Standard Stack

### Core (no new dependencies)

| Library / tool | Version | Purpose | Why standard |
|---|---|---|---|
| `golang-migrate/migrate/v4` | v4.19.1 (already in `go.mod`) | boot migrations; the ahead-of-source behaviour under test | Already the app's migration engine `[VERIFIED: go.mod — read this session]` |
| `github.com/golang-migrate/migrate/v4/source` | bundled | `source.Driver` interface — the D-18 seam's parameter type | `iofs.New` returns it; `migrate.NewWithInstance` consumes it; already imported in `migrate.go` `[VERIFIED: internal/db/migrate.go:15 — read this session]` |
| `testing/fstest` (`MapFS`) | stdlib (Go 1.26) | synthetic truncated migration source for the D-02 test | stdlib; verified working with `iofs.New` this session |
| Go stdlib: `bufio`, `strings`, `regexp`, `flag`, `os/exec`, `io/fs` | Go 1.26 | `cmd/migration-check` scanning + `git show` shelling | Mirrors `cmd/coverage-report` (stdlib-only hand parser) `[VERIFIED: cmd/coverage-report/main.go:1-20 — read this session]` |
| `github.com/caarlos0/svu/v3` | v3.4.1 | resolve the previous release tag (`svu current`) in the boot job | Exact tool + version the `release` job already `go install`s `[VERIFIED: .github/workflows/full-pipeline.yml:373 — read this session]` |
| `postgres` | `postgres:16` | throwaway DB for the boot job | Matches `docker-compose.yml` `[VERIFIED: docker-compose.yml:3 — read this session]` |

### Alternatives Considered

| Instead of | Could use | Tradeoff / why rejected |
|---|---|---|
| Hand-rolled SQL scanner | `github.com/pganalyze/pg_query_go` (libpg_query) | Full real Postgres parser, but **cgo** — the dev box's `mingw64 gcc cc1.exe` cannot execute (documented repeatedly in STATE.md; sqlc runs `CGO_ENABLED=0`, `-race` unusable). Non-starter. |
| Hand-rolled SQL scanner | `github.com/auxten/postgresql-parser` / `cockroachdb/sqlparser` (pure Go) | Real parser, no cgo — but a large dependency tree for a CI helper, against the repo's hand-roll ethos (D-06) and `cmd/coverage-report` precedent. Reconsider only if the scanner's blind spots prove to bite. |
| Hand-rolled SQL scanner | `squawk` (Rust binary + rule config) | Explicitly rejected in D-06: adds a dev-tool dependency + a config file. |
| `git show <tag>:queries/*.sql` | ghcr.io image label / API query for the previous release | D-13: extra token handling, can disagree with git tags on a half-failed push. |
| `git diff --name-only` in `changes` | `dorny/paths-filter@<sha>` | paths-filter is ergonomic for the boolean but adds a third-party action for what one `git` command does. Either is acceptable (D-16 leaves it to discretion); plain `git diff` is more on-brand. |
| `services: postgres` on the boot job | `docker run -d postgres:16` inside an `if:`-gated step | See Finding 2 — a `services:` container starts on every push; an `if:`-gated `docker run` does not. |

**Installation:** none. The phase adds no entry to `go.mod`, `go.sum`, `package.json`, or any lockfile. `svu` and `postgres:16` are already used elsewhere in CI.

---

## Package Legitimacy Audit

**No external packages are added by this phase.** `cmd/migration-check` and the `go run` migration helper are stdlib-only Go; the `internal/db/migrate.go` change uses symbols already imported (`golang-migrate/migrate/v4`, `.../source`, `os`, `errors`). `svu` v3.4.1 and `postgres:16` are pre-existing CI dependencies (`release` job / `docker-compose.yml`), not introduced here.

| Package | Registry | Disposition |
|---|---|---|
| *(none)* | — | No install step in this phase |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

---

## Architecture Patterns

### System flow — the N-1 boot check

```
GitHub push / PR
   │
   ▼
┌──────────────┐   git diff (base per D-16):
│  changes     │     PR   → origin/<base_ref>...HEAD
│  (no svc)    │     push → <event.before>..<sha>  (zeros → merge-base origin/main)
│  fetch-depth │
│  0 + tags    │   outputs: migrations_changed (bool), migration_files (list)
└──────┬───────┘
       │ needs
       ├────────────────────────────────┐
       ▼                                 ▼
┌────────────────────────┐      ┌──────────────────────────────────────────┐
│ migration-check (guard)│      │ n1-boot  (always runs; expensive steps    │
│ always runs            │      │          gated on migrations_changed)      │
│                        │      │                                           │
│ go run ./cmd/          │      │  if changed:                              │
│   migration-check       │      │   1. svu current → PREV_TAG               │
│   --added <files>       │      │      (fetch-depth 0 + tags; empty →       │
│   --prev-tag <svu>      │      │       skip-green, D-04/D-19a)             │
│                        │      │   2. docker run -d postgres:16            │
│ (a) DDL pattern scan    │      │   3. go run <helper>  → HEAD schema on    │
│     of *.up.sql         │      │      localhost:5432 (calls               │
│ (b) git show <tag>:     │      │      internal/db.RunMigrations)          │
│     queries/*.sql →      │      │   4. docker pull ghcr.io/...:PREV_TAG    │
│     (table,col) set →    │      │      (1 retry, D-04)                      │
│     cross-ref DROP/      │      │   5. docker run --network host           │
│     RENAME               │      │      -e DATABASE_URL -e HTTP_PORT=8080   │
│                        │      │      (NO INSTANCE_PASSPHRASE)             │
│ two finding classes,    │      │   6. poll /health → sustained 200        │
│ class-specific messages,│      │   7. GET /watchlist (200 []),            │
│ each names the README   │      │      GET /events (200 {events:[]})       │
│                        │      │   8. POST /watchlist {mbid,name,image_url}│
│                        │      │      → 201, then GET /watchlist → row     │
│                        │      │   fail msg names the N-1 rule + README    │
└──────────┬─────────────┘      └────────────────┬─────────────────────────┘
           │  needs                              │  needs
           └───────────────┬────────────────────┘
                           ▼
                  ┌──────────────────┐
                  │  build-scan      │  needs: [vet, lint, test, gitleaks,
                  │  (unchanged body)│         trivy-fs, frontend-test,
                  └────────┬─────────┘         migration-check, n1-boot]
                           ▼
                  ┌──────────────────┐
                  │  release         │  (push to main only)
                  └──────────────────┘
```

### Pattern 1: the D-18 source seam

`[VERIFIED: internal/db/migrate.go:207-246 — read this session]` — current `RunMigrations` body: `iofs.New(migrationsFS, "migrations")` → `newRetryConfig(opts...)` → `redactDSN(dsn)` → retry loop over `runMigrationsOnce(ctx, dsn, src)`.

Refactor:

```go
func RunMigrations(ctx context.Context, dsn string, logger *slog.Logger, opts ...RetryOption) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}
	return runMigrationsWithSource(ctx, dsn, logger, src, opts...)
}

// runMigrationsWithSource holds the former RunMigrations body from newRetryConfig
// onward. Unexported; the D-02 test drives it with a synthetic truncated source.
func runMigrationsWithSource(ctx context.Context, dsn string, logger *slog.Logger, src source.Driver, opts ...RetryOption) error {
	cfg := newRetryConfig(opts...)
	target := redactDSN(dsn)
	// ... existing retry loop, unchanged, calling runMigrationsOnce(ctx, dsn, src) ...
}
```

- `source.Driver` is the right interface type. No new exported symbol. `migrationsFS` stays unexported. The `iofs.New` error path moves into `RunMigrations`'s wrapper (it already lived at the top of the old body).
- The production call site `db.RunMigrations(ctx, cfg.DatabaseURL, logger)` `[VERIFIED: cmd/server/main.go:137 — read this session]` is untouched — signature preserved.
- The ahead-of-source guard from Finding 1 lands in `runMigrationsOnce` (which already receives `src source.Driver` `[VERIFIED: internal/db/migrate.go:263 — read this session]`), after `migrate.NewWithInstance`, before the `m.Up()` goroutine. No signature change to `runMigrationsOnce`.

### Pattern 2: `cmd/migration-check` structure (mirror `cmd/coverage-report`)

`[VERIFIED: cmd/coverage-report/main.go:40-80, cmd/coverage-report/main_test.go:1-27 — read this session]`

- `package main`; thin `main()` → `os.Exit(1)` on error from `run(args []string, stdout io.Writer) error`.
- `flag.NewFlagSet("migration-check", flag.ContinueOnError)`.
- Whitebox tests (`package main`) drive `run()` and the pure helpers directly.
- Deterministic seams for tests: inject the "list of added migration files", the "previous tag", and a `gitShow func(tag, path string) ([]byte, error)` so tests don't need a real git repo (the real one shells `git show`).
- Golden-file pattern with an `-update` flag is available if failure-message wording is worth pinning.
- Table-driven tests with SQL fixtures in `testdata/`.

### Pattern 3: `internal/db` real-Postgres test (mirror `migrate_test.go`)

`[VERIFIED: internal/db/migrate_test.go:31-44, 126-143, 145-207 — read this session]`

- `testutil.RequirePostgresDSN(t)` — skips on `-short` or missing `TEST_DATABASE_URL` `[VERIFIED: internal/testutil/postgres.go:29-41 — read this session]`.
- Dedicated scratch schema via a `search_path` query param on the DSN (`scratchSchemaDSN` helper): `DROP SCHEMA IF EXISTS <name> CASCADE` + `CREATE SCHEMA <name>` in setup, drop in `t.Cleanup`. One test binary per package ⇒ a fixed schema name is safe.
- The D-02 test must be `package db` (in-package) to reach unexported `runMigrationsWithSource` — the existing `migrate_test.go` is `package db_test`, so add a new file (e.g. `migrate_ahead_test.go`, `package db`).
- D-02 test shape: build a scratch-schema DSN; construct a `fstest.MapFS` with synthetic `000001..00000(N+1)` `SELECT 1;` up+down files; migrate the scratch DB to `N+1` (call `runMigrationsWithSource` with the full `N+1` source, or `migrate.Up()` directly); then call `runMigrationsWithSource` with the `1..N` source and assert `nil` and that `schema_migrations.version` is still `N+1`, `dirty=false`.

### Anti-Patterns to Avoid

- **Job-level `if:` on a job in `build-scan.needs:`** — skips `build-scan` (Finding 2).
- **Scanning `*.down.sql`** — every down file in the repo contains `DROP TABLE` / `DROP COLUMN` `[VERIFIED: internal/db/migrations/000002_watchlist.down.sql, 000004..000006 .down.sql — read this session]`. The app never runs `Down()`. The guard must scan **`*.up.sql` only**.
- **String-matching golang-migrate error text** for the ahead-of-source case — the message is `fmt.Errorf`, not a sentinel. Use the version comparison.
- **`docker compose config` as a verify step** — inlines `.env` secrets (already a banned pattern in this repo, Phase 14 G-14-1).
- **Re-computing the diff base per job** — D-16: compute once in `changes`, consume the outputs.
- **Trusting `svu current` output into a shell without quoting** — it is a tag string; pass via env, don't interpolate (repo convention, T-15-V5).

---

## D-15: extracting the previous release's referenced `(table, column)` set from `queries/*.sql`

This is the largest single addition to `cmd/migration-check`. Scope below is grounded in the **actual** repo query files read this session: `queries/artists.sql`, `queries/events.sql`, `queries/health.sql`, `queries/watchlist.sql`.

### What the previous-release queries actually look like

`[VERIFIED: queries/*.sql — read this session]`

| File | Constructs the extractor must handle |
|---|---|
| `artists.sql` | `INSERT INTO artists (mbid, deezer_id, name, disambiguation, image_url)`; `ON CONFLICT (mbid) DO UPDATE SET name = EXCLUDED.name, deezer_id = COALESCE(EXCLUDED.deezer_id, artists.deezer_id), …`; `RETURNING *`; `SELECT a.* FROM artists a JOIN watchlist w ON w.artist_id = a.id WHERE a.image_url IS NULL AND (a.art_match_attempted_at IS NULL OR …)`; `UPDATE artists SET art_match_attempted_at = now() WHERE mbid = $1` |
| `events.sql` | `INSERT INTO events (artist_id, source, event_type, external_id, release_group_mbid, title, artist_name, release_date, cover_art_url, track_count, notified_at, previous_track_count, release_type, watched_artist_name)` (**14 named columns**); `ON CONFLICT (event_type, source, external_id) DO NOTHING`; `SELECT external_id FROM events WHERE artist_id = $1 AND source = $2 AND event_type = $3`; `SELECT EXISTS(SELECT 1 FROM events WHERE …)`; `SELECT * FROM events WHERE notified_at IS NULL ORDER BY created_at ASC, id ASC`; a data-modifying CTE `WITH existing AS (SELECT track_count FROM events WHERE … FOR UPDATE) UPDATE events e SET track_count = $2 FROM existing WHERE … RETURNING existing.track_count AS previous_track_count`; a big explicit `SELECT id, artist_id, source, …, created_at FROM events WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id')::bigint) AND … AND created_at >= sqlc.arg('cutoff')::timestamptz ORDER BY release_date DESC NULLS LAST, id DESC LIMIT sqlc.arg('page_size')` |
| `watchlist.sql` | `INSERT INTO watchlist (artist_id, release_types, muted_event_types) … RETURNING *`; `SELECT w.id AS id, a.id AS artist_id, a.mbid, a.name, a.deezer_id, a.disambiguation, a.image_url, w.release_types, w.muted_event_types, w.created_at, w.updated_at FROM watchlist w JOIN artists a ON a.id = w.artist_id`; a CTE `WITH updated AS (UPDATE watchlist SET release_types = CASE WHEN @set_release_types::boolean THEN @release_types::text[] ELSE watchlist.release_types END, … WHERE watchlist.id = @id RETURNING watchlist.id, watchlist.artist_id, …) SELECT u.id AS id, a.id AS artist_id, a.mbid, … FROM updated u JOIN artists a ON a.id = u.artist_id`; `DELETE FROM watchlist WHERE id = $1` |
| `health.sql` | `SELECT 1;` — no table refs |

### Recommended extraction approach (stdlib Go)

Split the file on `-- name: X :kind` markers (this is sqlc's own delimiter). For each query block:

1. **Strip comments** (`--` to end-of-line; `/* */`). Normalise whitespace.
2. **Build an alias → table map** from `FROM <t> [AS] <alias>` and `JOIN <t> [AS] <alias>` fragments (regex is adequate; sqlc SQL has no lateral/function-table sources in this repo). Record CTE names from `WITH <name> AS (` and **exclude** them from the real-table set (they are not migratable objects), but keep them in the alias map so `FROM existing` / `FROM updated u` resolve to "not a real table."
3. **`INSERT INTO <t> ( <col-list> )`** — `col-list` split on commas → `(t, col)` pairs. Highest-confidence signal. (artists: 5, events: 14, watchlist: 3.)
4. **`ON CONFLICT ( <col-list> )`** — `(target-table, col)` where target-table is the `INSERT INTO` table. A rename of any of these breaks the statement outright.
5. **`RETURNING <list>`** — `*` → all columns of the statement's target table; `existing.track_count` / `watchlist.id` → resolve the qualifier; bare `col` → target table.
6. **Explicit `SELECT` lists** — for each item: `a.col` / `w.id AS id` → resolve `a`/`w` via the alias map; bare `col` → ambiguous, attribute to **every** real table in this query's FROM/JOIN set (conservative).
7. **`SELECT *` / `SELECT <alias>.*`** — `<alias>.*` → all columns of that alias's table; bare `*` → if the query has exactly one real table in FROM and no JOIN (e.g. `SELECT * FROM events WHERE …`), all columns of that table; otherwise skip (leave to the boot job).
8. **`WHERE` / `JOIN ... ON` / `ORDER BY` / `GROUP BY` column refs** — `a.col` resolved via alias map; bare `col` → every real table in FROM/JOIN.
9. **`EXCLUDED.<col>` and `<table>.<col>` inside `DO UPDATE SET`** — `EXCLUDED.<col>` → target table's column; `<table>.<col>` → that table.
10. **`sqlc.arg('x')` / `sqlc.narg('x')` / `@x`** — these are **parameter** names, not necessarily columns (`cutoff`, `page_size`, `set_release_types` are not columns). Add them to a "referenced identifiers" bag and match against real column names only — a param named `page_size` simply never matches a `DROP COLUMN`, so including them over-broadens harmlessly (false-red only, and only if a param name happens to equal a dropped column name).

**"All columns of table X"** requires the previous release's schema. Two sources, both at the previous tag:

- **Primary:** parse `CREATE TABLE` + `ALTER TABLE … ADD COLUMN` out of `git show <tag>:internal/db/migrations/*.up.sql` (another bounded hand-scan — the same tokenizer the DDL detector uses, run in reverse).
- **Cross-check:** `git show <tag>:internal/db/sqlc/models.go` — the generated structs list every column as a field. Useful as a sanity check; note pgx-mode sqlc structs are positional (no `db:"col"` tag), so field-name→column mapping is `PascalCase`↔`snake_case`.

### Blind spots (enumerate — these are the boot job's responsibility, not the guard's)

| # | Blind spot | Present in this repo? | Guard behaviour | Backstop |
|---|---|---|---|---|
| B1 | Bare unqualified column in a multi-table query — can't say which table | Yes (`WHERE artist_id = $1` in `ListExternalIDs` is single-table, fine; joins in `ListWatchlist` use qualified refs) | Attribute to every real table in FROM/JOIN (conservative false-red) | boot job `GET`s |
| B2 | Bare `SELECT *` needing schema expansion | Yes: `SELECT * FROM events` (`ListUnnotified`), `RETURNING *` (artists, watchlist) | Expand only for single-table FROM / single-table `RETURNING *`; else skip | boot job |
| B3 | CTE names look like tables (`FROM existing`, `FROM updated u`) | Yes: `AdvanceGroupTrackCountBaseline`, `UpdateWatchlistPreferences` | Record `WITH <name> AS (` and exclude from real-table set | n/a (no migration drops a CTE) |
| B4 | Subqueries (`SELECT EXISTS(SELECT 1 FROM events WHERE …)`) | Yes: `HasAnyEvent`, `HasOlderEvents` | Flatten — scan all `FROM`/`WHERE` fragments in the block regardless of nesting depth | boot job |
| B5 | Dynamic / Go-built column lists | **No** — sqlc forbids them; every query is one static string | n/a | n/a |
| B6 | Schema-qualified names (`public.events`) | No (repo uses bare names) | Strip a leading `schema.` prefix defensively | — |
| B7 | Expression columns / functions (`now()`, `COALESCE(...)`, `to_regclass(...)`) | Yes (`now()`, `COALESCE(EXCLUDED.x, artists.x)`) | Only pull identifiers in `<qualifier>.<name>` or known-column positions; ignore bare function-call tokens | — |
| B8 | Quoted identifiers (`"select"`) | No | Handle `"..."` as a single identifier token defensively | — |
| B9 | Type-narrowing / `SET NOT NULL` on an existing column (not a drop/rename) | — | Out of D-15 scope by S1's own note | boot job (D-08 backstop) |

### Conservatism tuning (false-red vs. false-green)

D-15's cross-reference **cannot be overridden** by the `allow-destructive` annotation. So an over-broad false-red here is *not* merely "annoying but safe" — it permanently blocks a migration with no escape hatch. Recommended split:

- **Deterministic red** (annotation cannot override): `DROP COLUMN` / `RENAME COLUMN` / `DROP TABLE` / `RENAME TABLE` where the object appears in a **high-confidence position** in a previous-release query — an explicit `INSERT (...)` column, an explicit `SELECT` list item, an `ON CONFLICT (...)` target, a `RETURNING` list item, a **qualified** `table.col` / `alias.col` (alias resolvable), or an expansion of `SELECT * FROM <single_table>` / single-table `RETURNING *`.
- **Leave to the boot job** (no guard red): bare unqualified columns in multi-table queries where the alias map can't disambiguate, and bare `*` in multi-table queries. Rationale: a false-red with no override is worse than relying on the behavioural check for the genuinely ambiguous minority; the high-confidence positions cover every write the poller does (all in `queries/*.sql`) and every read the SPA does.
- `sqlc generate` on the **current** branch already rejects a current-branch query referencing a dropped column, so D-15 is strictly about the **previous** release's queries — never re-flag a current-branch reference.

---

## D-08: static destructive-DDL detection in `*.up.sql`

Feasibility, grounded in the 7 existing up-migrations (all additive: `SELECT 1;`, `CREATE TABLE`, `CREATE INDEX`, `ADD COLUMN … <nullable, no default>`, an idempotent `UPDATE … WHERE … IS NULL` backfill) `[VERIFIED: internal/db/migrations/000001..000007 *.up.sql — read this session]`.

### Scanner mechanics (stdlib, hand-rolled — the on-brand choice)

There is **no stdlib Postgres DDL tokenizer**. `text/scanner` (stdlib) tokenizes Go-ish syntax, not SQL comments/dollar-quoting — not worth bending. Hand-roll, matching `cmd/coverage-report`'s line-by-line hand parser:

1. Strip `--` line comments and `/* … */` block comments.
2. Split into statements on `;`, respecting `'…'` string literals and `$tag$…$tag$` dollar-quoting (none in the repo today, but defensive).
3. Per statement: collapse whitespace, uppercase a copy for keyword matching (keep the original for identifier extraction and line numbers).
4. Match patterns; report file + 1-based line of the offending statement (compute from the pre-split offset).

### Reliably detectable (regex / token scan)

| Pattern | Class | Notes |
|---|---|---|
| `DROP TABLE [IF EXISTS] <name>` | backward-incompatible | trivial |
| `ALTER TABLE <t> … DROP COLUMN [IF EXISTS] <c>` | backward-incompatible | trivial; capture `<t>`,`<c>` for the D-15 cross-ref |
| `ALTER TABLE <t> RENAME TO <new>` | backward-incompatible | trivial |
| `ALTER TABLE <t> RENAME [COLUMN] <a> TO <b>` | backward-incompatible | trivial; capture for D-15 |
| `ALTER TABLE <t> … ADD [COLUMN] <c> <type> … NOT NULL` **without** `DEFAULT` in the same column clause | unsafe-forward | split the `ADD COLUMN` clause; `NOT NULL DEFAULT x` is safe, `NOT NULL` alone is the hazard. A later separate `ALTER COLUMN … SET DEFAULT` does not help (order). |
| `ALTER TABLE <t> … ALTER COLUMN <c> SET NOT NULL` | backward-incompatible | tightening NOT NULL on an existing column — fails on existing NULLs / table scan, and breaks an old binary that inserts NULL there |
| `ALTER TABLE <t> … ADD [CONSTRAINT <name>] CHECK (…)` / `ADD CHECK (…)` | backward-incompatible | a new CHECK an old binary can violate |
| `ALTER TABLE <t> … DROP CONSTRAINT <name>` | review (recommend backward-incompatible message) | ambiguous impact; could drop a UNIQUE an `ON CONFLICT` needs |
| `ALTER TABLE <t> … ALTER COLUMN <c> DROP DEFAULT` | review | breaks an old binary that omits `<c>` and relies on the DB default |

### Hard — flag broadly, require the annotation (blind spots)

| Pattern | Why it's hard | Recommendation |
|---|---|---|
| Type narrowing: `ALTER COLUMN <c> [SET DATA] TYPE <newtype>` (`varchar(255)→varchar(100)`, `int→smallint`, `bigint→int`, `numeric(10,2)→numeric(8,2)`, `timestamptz→timestamp`) | Requires the **old** type (prior schema) *and* a widen-vs-narrow comparison. Pure-lexical can't reliably tell `varchar(100)→text` (safe) from `varchar(255)→varchar(100)` (narrowing). | Flag **every** `ALTER COLUMN … TYPE` as backward-incompatible; the `allow-destructive` annotation is the escape hatch for a genuine widening. The repo has never done one, so the friction is theoretical. |
| Added generated / identity column (`GENERATED ALWAYS AS … STORED`, `GENERATED … AS IDENTITY`) | Implicitly not-null-ish; table rewrite | Treat like `ADD COLUMN … NOT NULL` (unsafe-forward) |
| `CREATE INDEX` (non-`CONCURRENTLY`) on a large table; table-rewriting `ALTER` | "no blocking DDL" — **explicitly out of scope** (D-08); stays a written rule | none — README prose only |
| `CREATE INDEX CONCURRENTLY` in a boot migration | cannot run inside golang-migrate's per-migration transaction (Pitfall 9) | README prose only (D-08); optionally a soft warning |

### Two finding classes (S4) — message content

- **backward-incompatible (N-1 break):** `DROP COLUMN`, `DROP TABLE`, `RENAME` (column/table), `ALTER COLUMN … TYPE`, `ALTER COLUMN … SET NOT NULL`, `ADD … CHECK`. Message: names the **expand/contract rule** and the **N-1 invariant**, points at `internal/db/migrations/README.md`. Overridable by `-- migration-check:allow-destructive …` **except** a `DROP`/`RENAME COLUMN`/`TABLE` that D-15 finds still-referenced by the previous release.
- **unsafe-forward (deploy hazard):** `ADD COLUMN … NOT NULL` without `DEFAULT` (and identity/generated adds). Message: explains the non-empty-table failure / table-rewrite lock (Pitfall 9) and points at the README's "adding a NOT NULL column" note. Overridable by the same annotation (author asserts the table is empty / the lock is acceptable).

---

## Rule Documentation (MGRT-02 / D-09 / D-10)

`internal/db/migrations/README.md` — sits next to the `.sql` files, unmissable when adding a migration. Structure (S4's two-rule split):

1. **The two rules, stated plainly and separately:**
   - *Backward-incompatible changes break rollback.* The previously-released binary boots against the new schema (Phase 17 auto-rollback is strictly N-1, D-17). A `DROP COLUMN` / `RENAME` / type-narrowing in the same release the code stops using it = a latent outage. Split it: **expand** in vN, **contract** in a later release.
   - *Unsafe-forward changes break or lock the deploy.* `ADD COLUMN … NOT NULL` with no `DEFAULT`, non-`CONCURRENTLY` index builds, table-rewriting `ALTER` on a non-empty table — these never break the old binary but fail or hold a long lock on a populated table (Pitfall 9).
2. **The N-1 invariant:** the previous released tag's binary must run cleanly against `HEAD`'s schema. The CI `n1-boot` job proves it every migration PR; `internal/db`'s ahead-of-source test proves the boot no-op every run.
3. **Concrete expand → backfill → contract walkthrough** citing `000007_backfill_events_watched_artist_name` `[VERIFIED: internal/db/migrations/000007_backfill_events_watched_artist_name.up.sql — read this session]`: `000006` added `events.watched_artist_name` as a nullable column with no backfill (expand); `000007` is the idempotent `UPDATE events e SET watched_artist_name = a.name FROM artists a WHERE a.id = e.artist_id AND e.watched_artist_name IS NULL` (backfill, `WHERE … IS NULL` so it is safe to re-run and a no-op on a fresh DB); a future release could `DROP` the old path (contract) — but only once the expand release is provably not coming back.
4. **"Before you merge a migration" checklist** (copy-paste; sourced from `PITFALLS.md` Pitfall 8 "How to avoid"):
   - Can the currently-deployed version run against this schema? If no → split it.
   - Up-only, additive, embedded. New columns nullable or `DEFAULT`ed.
   - No `DROP` / `RENAME` / type-narrowing / `ADD … NOT NULL` (no default) in the release the code starts depending on it.
   - No `CREATE INDEX CONCURRENTLY` in a boot migration (can't run in the per-migration transaction).
   - Backfill migrations idempotent (`WHERE … IS NULL`), a no-op on a fresh DB.
5. **What `cmd/migration-check` enforces** and the `allow-destructive` annotation syntax (`-- migration-check:allow-destructive expand-shipped-in=vX.Y.0 reason=<text>` — both keys required; `expand-shipped-in` must be a real tag; a value older than `svu current` is a warning; the annotation cannot wave through a live N-1 break that D-15's query cross-reference catches).

One-line pointer added to `.claude/CLAUDE.md`'s **"Definition of Done — run before every commit"** section (D-09) — that section already lists the pre-commit gates `[VERIFIED: .claude/CLAUDE.md "Definition of Done" section — provided in context]`.

---

## GitHub Actions Wiring (D-14, D-16, D-13, D-04)

### `changes` prelude job (D-16)

```yaml
changes:
  runs-on: ubuntu-latest
  timeout-minutes: 5
  permissions:
    contents: read
  outputs:
    migrations_changed: ${{ steps.diff.outputs.migrations_changed }}
    migration_files: ${{ steps.diff.outputs.migration_files }}
  steps:
    - uses: actions/checkout@<sha> # v7.0.1
      with:
        fetch-depth: 0
        fetch-tags: true          # S8 hard requirement; explicit even though fetch-depth:0 implies it
    - id: diff
      env:
        BEFORE: ${{ github.event.before }}
        SHA: ${{ github.sha }}
        BASE_REF: ${{ github.base_ref }}
      run: |
        set -euo pipefail
        if [ -n "$BASE_REF" ]; then
          git fetch --no-tags origin "$BASE_REF"
          range="origin/${BASE_REF}...HEAD"                     # three-dot = merge-base
        elif [ "$BEFORE" = "0000000000000000000000000000000000000000" ] || ! git cat-file -e "${BEFORE}^{commit}" 2>/dev/null; then
          git fetch --no-tags origin main
          range="$(git merge-base origin/main HEAD)...HEAD"
        else
          range="${BEFORE}...${SHA}"
        fi
        files="$(git diff --name-only --diff-filter=AM "$range" -- 'internal/db/migrations/*.up.sql')"
        # emit ...
```

- `github.base_ref` is set **only** on `pull_request` events; its presence is the reliable PR discriminator.
- `github.event.before` all-zeroes on a new branch / first push; also guard against a force-push where `before` is unreachable (`git cat-file -e`).
- `--diff-filter=AM` — added or modified up-files. D-07's diff-scoping means a destructive statement that shipped in an earlier release never re-reddens.
- Mechanism: plain `git diff` (recommended) or `dorny/paths-filter@<sha> # v3.0.2`.

### `n1-boot` job (D-13 / D-14 / D-04)

- **Checkout** `fetch-depth: 0` + `fetch-tags: true` (needed for `svu current`).
- **Resolve previous tag** (step, `if: needs.changes.outputs.migrations_changed == 'true'`): `go install github.com/caarlos0/svu/v3@v3.4.1` then `PREV_TAG="$("$(go env GOPATH)/bin/svu" current)"`. Mirrors the `release` job exactly `[VERIFIED: .github/workflows/full-pipeline.yml:370-381 — read this session]`. Empty/error → D-04/D-19a in-job step: `echo "::notice::no prior release tag — true bootstrap, skipping N-1 check"` and pass. (Dead code on this repo: `git tag` shows `v0.1.0 … v1.7.0`; `git describe --tags --abbrev=0` = `v1.7.0` `[VERIFIED: git tag / git describe — run this session]`.)
- **Postgres** (step-gated `docker run -d --network host -e POSTGRES_USER=drop_tracker -e POSTGRES_PASSWORD=drop_tracker -e POSTGRES_DB=drop_tracker postgres:16` + a readiness wait loop on `pg_isready`), *not* `services:` (Finding 2). DSN: `postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable` (matches `Makefile` `TEST_DATABASE_URL` `[VERIFIED: Makefile:9 — read this session]`).
- **HEAD schema** (step-gated): `DATABASE_URL=<dsn> go run ./<migration-helper>` — the helper calls `internal/db.RunMigrations(ctx, os.Getenv("DATABASE_URL"), logger)`. This is the same embedded-`iofs` + bounded-retry path the app runs (D-01). The helper can be a `cmd/` package or a `-migrate` flag on an existing binary (planner discretion).
- **Pull N-1 image** (step-gated): `docker pull "ghcr.io/danielrpof/drop-tracker:$PREV_TAG" || { sleep 5; docker pull "ghcr.io/danielrpof/drop-tracker:$PREV_TAG"; }` (D-04 one retry). Public package — no `docker login`, `permissions: contents: read` only `[VERIFIED: ARCHITECTURE.md notes public ghcr package; .github/workflows top-level permissions: contents: read]`.
- **Run N-1 image** (step-gated): `docker run -d --network host -e DATABASE_URL=<dsn> -e HTTP_PORT=8080 "ghcr.io/danielrpof/drop-tracker:$PREV_TAG"`. Only `DATABASE_URL` is `notEmpty` in `internal/config` `[VERIFIED: internal/config/config.go:17-18 — read this session]`; `HTTP_PORT` default `8080`; `DISCORD_WEBHOOK_URL` optional; no `INSTANCE_PASSPHRASE` (GATE-07 inert path — routes reachable unauthenticated). The 15-minute default `POLL_INTERVAL` never fires inside the health window.
- **Health + read + write probes** (step-gated), the D-03 contract:
  - Poll `curl -fsS http://localhost:8080/health` → expect `200` `{"status":"ok","db":"up"}` `[VERIFIED: internal/httpserver/health.go:28-44 — read this session]`; sustained window (e.g. 8–10 consecutive 200s at ~2 s spacing — planner discretion).
  - `GET /watchlist` → `200` with body `[]` (empty DB — defensive nil→`[]` in the handler) `[VERIFIED: internal/httpserver/watchlist.go:264-278 — read this session]`.
  - `GET /events` → `200` with `{"events":[], "next_cursor":null, "has_older_events":false}` `[VERIFIED: internal/httpserver/events.go:29-33, 130-146 — read this session]`.
  - `POST /watchlist` with a minimal body — **supply `image_url`** so the art matcher is skipped: `{"mbid":"00000000-0000-0000-0000-000000000001","name":"CI N-1 Boot Probe","image_url":"https://example.invalid/x.png"}` → expect `201`. Then `GET /watchlist` → assert the row is present.
  - **Confirmed safe (D-03/S1 planner TODO):** `watchlist.Service.Add` calls `s.matcher.Match` (MusicBrainz/Deezer) **only if `s.matcher != nil && p.ImageURL == nil`**, and even then a match error is logged and the add proceeds (fail-open) `[VERIFIED: internal/watchlist/service.go Add() — read this session]`. Supplying `image_url` bypasses the matcher entirely — the `POST /watchlist` probe is a pure DB write (`UpsertArtist` + `CreateWatchlistEntry` INSERTs) with **no external network call**. The write assertion is viable; the S1 fallback ("rely on D-15 alone") is not needed.
- **Failure message** must name the N-1 / expand-contract rule and point at `internal/db/migrations/README.md`.

### `build-scan.needs:` (D-11)

Append `migration-check` and `n1-boot` (and, if `build-scan` reads its outputs, `changes`; otherwise `changes` is a transitive dependency via the other two). Current: `needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]` `[VERIFIED: .github/workflows/full-pipeline.yml:300 — read this session]`. **Both appended jobs must be unconditional** (Finding 2).

### Fetch-depth / tags

`release` uses `fetch-depth: 0` alone and `svu current` works `[VERIFIED: .github/workflows/full-pipeline.yml:362-373 — read this session]`; `actions/checkout` v4+ fetches tags when `fetch-depth: 0`. S8 makes `fetch-tags` explicit a hard requirement — add `fetch-tags: true` to every new job that resolves a tag or diffs history.

### Conventions to follow

`[VERIFIED: .github/workflows/full-pipeline.yml — read this session]`

- Third-party actions SHA-pinned with a trailing `# vX.Y.Z` comment.
- Top-level `permissions: contents: read`; jobs opt into more (the new jobs need only `contents: read`).
- `concurrency` block: `cancel-in-progress` only on PRs — inherited, no change.
- Dynamic values cross into `run:` via `env:`, never string-interpolated.

---

## Don't Hand-Roll

| Problem | Don't build | Use instead | Why |
|---|---|---|---|
| "Old binary tolerates newer schema" detection | A bespoke `schema_migrations` SQL query in the boot path | `migrate.Migrate.Version()` + walk `source.Driver.First()/Next()` | The library already exposes both; a raw query duplicates golang-migrate's version bookkeeping and misses the `dirty` flag |
| Diff-base computation | Per-job `git merge-base` re-derivation | One `changes` prelude job, consumed via `needs.*.outputs` (D-16) | Divergent bases across jobs = the S2 direct-to-`main` no-op bug |
| Previous-release tag resolution | ghcr.io API query / a committed rollback-floor file | `svu current` (D-13) | The `release` job already uses it; an API query needs token handling and can disagree with git tags |
| Full SQL parsing | Vendoring a Postgres grammar | Comment-strip + statement-split + targeted pattern match over `*.up.sql` (D-06) | cgo is unusable on the dev box; a pure-Go parser is a large dep against the repo's hand-roll ethos; the N-1 boot job is the behavioural backstop |
| Migration idempotency for the rollback image | Custom "is this migration already applied" logic | The ahead-of-source no-op guard (Finding 1) + golang-migrate's existing `ErrNoChange` handling | golang-migrate already handles equal-version; only the *ahead* case is missing |

**Key insight:** the milestone's rollback safety rests on one library behaviour that turned out to be the opposite of what was assumed. Verifying it against the pinned module (not training memory, not a doc) was the whole point of SC #4 — and it caught a real defect.

---

## Runtime State Inventory

This phase adds CI tooling, a test, a README, and one small boot-path guard. It renames nothing and migrates no data. For completeness against the rename/refactor checklist:

| Category | Items Found | Action Required |
|---|---|---|
| Stored data | None — the phase never touches a real database. The D-02 test and the `n1-boot` job use throwaway Postgres (scratch schema / ephemeral container). | none |
| Live service config | None — CI-only, no VPS, no external service configuration. | none |
| OS-registered state | None. | none |
| Secrets / env vars | None added. The `n1-boot` job reads no secrets (`GITHUB_TOKEN` not needed — public ghcr package). The N-1 image is run with `DATABASE_URL` + `HTTP_PORT` literals, no `INSTANCE_PASSPHRASE`. | none |
| Build artifacts / installed packages | `cmd/migration-check` and the migration helper are new `main` packages — they compile under `go vet` / `golangci-lint` and need the `COVER_PKGS` + `gosec` carve-outs (below). No `go.mod` change. | add `.golangci.yml` + `Makefile` carve-outs |

**Behaviour change (not a rename):** `internal/db/migrate.go` gains an ahead-of-source no-op branch (Finding 1). This is a genuine runtime-behaviour change to the boot path — the only one in the phase. It is inert on every forward boot (`cur <= smax`) and only fires on a rollback. Covered by the D-02 test.

---

## Common Pitfalls

### Pitfall A: assuming `migrate.Up()` no-ops on an ahead-of-source schema
**What goes wrong:** the whole rollback model is built on it; it's false (Finding 1). **How to avoid:** the D-02 test + the boot-path guard. **Warning sign:** the D-02 test passes against *unmodified* `migrate.go` — that would mean the test isn't actually exercising the ahead case.

### Pitfall B: job-level `if:` on a job in `build-scan.needs:`
**What goes wrong:** skips `build-scan` → `release` on every non-migration push (Finding 2). **How to avoid:** gate steps, not the job; keep `changes` / `migration-check` / `n1-boot` unconditional. **Warning sign:** on a docs-only PR, `build-scan` shows "Skipped."

### Pitfall C: scanning `*.down.sql`
**What goes wrong:** every down file in the repo has `DROP TABLE` / `DROP COLUMN`; the guard reddens every migration. **How to avoid:** glob `*.up.sql` only. The app never runs `Down()`.

### Pitfall D: `services: postgres` starting on every push
**What goes wrong:** ~10 s Postgres cold-start on the ~95 % of pushes with no migration change. **How to avoid:** bring Postgres up in an `if:`-gated `docker run` step, mirroring `make db-up`.

### Pitfall E: D-15 false-red with no override
**What goes wrong:** the query cross-reference can't be overridden by `allow-destructive`; an over-broad match permanently blocks a genuinely-safe migration. **How to avoid:** deterministic-red only on high-confidence positions (explicit column lists, qualified refs, single-table `*`); leave ambiguous cases to the boot job.

### Pitfall F: `dirty` schema_migrations masquerading as "ahead"
**What goes wrong:** a half-applied forward migration also blocks the old binary; treating it as a benign no-op hides a real problem. **How to avoid:** the guard checks `!dirty`; a dirty DB still returns `ErrDirty` (Phase 17 restore territory, D-02 caveat).

### Pitfall G: `github.event.before` on a new branch / force-push
**What goes wrong:** all-zeroes SHA or an unreachable commit → `git diff` errors or produces a garbage range → guard silently scans nothing or everything. **How to avoid:** the zero-SHA + `git cat-file -e` fallback to merge-base in the `changes` job.

### Pitfall H: `svu current` needs tags AND full depth
**What goes wrong:** a shallow checkout without tags → `svu current` returns empty → D-04 skip-green path fires on a repo that has tags → the N-1 check silently never runs. **How to avoid:** `fetch-depth: 0` + `fetch-tags: true` on the `n1-boot` job (S8).

### Existing milestone pitfalls (PITFALLS.md 8, 9, 10)
- **Pitfall 8** (non-backward-compatible migration bricks rollback): the phase's reason for existing. The "How to avoid" list is the README checklist (D-10). Finding 1 shows the "older image sees newer version → no-ops" bullet was *aspirational* — the code has to be made to do it.
- **Pitfall 9** (boot-migration failure → crash loop): exactly what an un-fixed Finding 1 causes on rollback. Also the reason the guard flags `ADD COLUMN … NOT NULL` (no default) and `CREATE INDEX CONCURRENTLY` stays a written rule (can't run in golang-migrate's per-migration transaction).
- **Pitfall 10** (migration lock contention): out of scope (single instance, D-17); the ahead-of-source guard reads `Version()` without the advisory lock, which is safe under the single-migrator assumption — note it, don't solve it.

---

## Code Examples

### Ahead-of-source guard (verified prototype — see Finding 1)

```go
// in internal/db/migrate.go, inside runMigrationsOnce, after migrate.NewWithInstance(...)
// and before the `go func() { done <- m.Up() }()` block:
if cur, dirty, verr := m.Version(); verr == nil && !dirty {
	if smax, ok := maxSourceVersion(src); ok && cur > smax {
		return nil // DB schema ahead of embedded source: rollback scenario, no-op (D-17)
	}
}
```

### D-02 hermetic test skeleton

```go
package db // in-package, to reach runMigrationsWithSource

func TestRunMigrationsWithSource_NoOpsAgainstAheadOfSourceSchema(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)
	scratch := "migrate_ahead_scratch"
	// DROP SCHEMA IF EXISTS ... CASCADE; CREATE SCHEMA ...; t.Cleanup(drop)
	scratchDSN := withSearchPath(dsn, scratch)

	const n = 5
	fullSrc := mapFSSource(t, n+1) // synthetic 000001..000006, SELECT 1; up+down
	nSrc := mapFSSource(t, n)      // synthetic 000001..000005

	// migrate scratch DB to n+1 with the full set
	if err := runMigrationsWithSource(ctx, scratchDSN, discardLogger, fullSrc); err != nil {
		t.Fatalf("prime to n+1: %v", err)
	}
	// the assertion: the n-migration binary must no-op, not error
	if err := runMigrationsWithSource(ctx, scratchDSN, discardLogger, nSrc); err != nil {
		t.Fatalf("ahead-of-source boot must return nil, got: %v", err) // RED against today's code
	}
	// version unchanged, not dirty
	assertSchemaVersion(t, scratchDSN, n+1, false)
}
```

### `changes` diff-base (see GitHub Actions Wiring for the full step)

```bash
if [ -n "$BASE_REF" ]; then range="origin/${BASE_REF}...HEAD"
elif [ "$BEFORE" = "0000000000000000000000000000000000000000" ]; then range="$(git merge-base origin/main HEAD)...HEAD"
else range="${BEFORE}...${SHA}"; fi
git diff --name-only --diff-filter=AM "$range" -- 'internal/db/migrations/*.up.sql'
```

---

## State of the Art

| Old assumption | Current finding | Source |
|---|---|---|
| `migrate.Up()` returns `ErrNoChange` against an ahead-of-source schema | Returns a hard error (`no migration found for version N+1`, wraps `os.ErrNotExist`); `dirty`+ahead returns `ErrDirty` | Executed falsification against `migrate/v4 v4.19.1` this session |
| A skipped `needs:` job counts as success for dependents | A skipped `needs:` job **skips** its dependents (implicit `if: success()`) | GitHub docs + community discussions |
| N-1 rollback safety is a CI-check + docs, no app code change | Also requires a boot-path behaviour change (`RunMigrations` ahead-of-source no-op) | Finding 1 |
| `POST /watchlist` in CI might need a live MusicBrainz/Deezer call | It does not, when `image_url` is supplied (matcher skipped; fail-open anyway) | `internal/watchlist/service.go` read this session |

**Deprecated / not applicable:** golang-migrate `Down()` / `Steps(-n)` — the app never runs them; the `"no migration found for version X"` error is *not* exclusive to the down path (Finding 1 shows `Up()` produces it too, for a different reason — `versionExists` on the current version).

---

## Assumptions Log

| # | Claim | Section | Risk if wrong |
|---|---|---|---|
| A1 | The ahead-of-source no-op guard added to `runMigrationsOnce` is behaviour-safe on every forward boot (guard inert when `cur <= smax`) | Finding 1 | Low — the comparison is arithmetic and covered by existing `migrate_test.go` idempotency/from-scratch tests plus the new D-02 test; but the planner should run the full `internal/db` suite after the change |
| A2 | `svu current` on this repo resolves to `v1.7.0` (or later) and `ghcr.io/danielrpof/drop-tracker:v1.7.0` exists in the registry | GitHub Actions Wiring | Medium — if the latest tag's image was never pushed (half-failed release), the pull fails → D-04 says **red**. Acceptable per D-11's accepted-risk, but the planner should sanity-check that recent `release` runs actually pushed images |
| A3 | `docker run --network host` on `ubuntu-latest` can reach a sibling `docker run -d` Postgres container on `localhost:5432` | GitHub Actions Wiring | Medium — needs both containers on host networking (or a shared user-defined network). If `--network host` for Postgres is blocked, use a named network for both. Verify in the first CI run |
| A4 | The D-15 "high-confidence positions" set (explicit `INSERT`/`SELECT` lists, `ON CONFLICT`, qualified refs, single-table `*`) covers every column the previous release's poller writes and SPA reads | D-15 | Medium — grounded in the 4 query files as they are today; a future query using bare `*` on a join would slip to the boot-job backstop. Acceptable by design |
| A5 | `actions/checkout` with `fetch-depth: 0` + `fetch-tags: true` provides everything `svu current` and the merge-base diff need | GitHub Actions Wiring | Low — `release` already works with `fetch-depth: 0` alone; adding `fetch-tags` is strictly more |
| A6 | GitHub Actions skip-propagation behaviour (Finding 2) is current as of 2026-09 and not changed by a recent platform update | Finding 2 | Low-Medium — documented behaviour + open community requests to change it; the planner's scratch-branch smoke test (Phase 9 precedent) is the confirmation |
| A7 | A `docker run` Postgres readiness wait (`pg_isready` loop) is sufficient without the compose `--wait` healthcheck | GitHub Actions Wiring | Low — standard pattern; the `go run` migration helper's own bounded retry (`DefaultMaxAttempts = 6`) is a second cushion |

---

## Open Questions

1. **Should `changes` also emit the list of *added* (`--diff-filter=A`) vs. *modified* migrations separately?**
   - What we know: D-07 diff-scopes to "migration files added on this branch"; a modification to an already-shipped up-file is unusual (migrations are immutable once released) and itself suspicious.
   - Recommendation: emit both; treat a *modified* already-released up-file as its own hard error ("released migrations are immutable") independent of the destructive-DDL scan.

2. **`expand-shipped-in` tag validation — against `git tag` or against reachable tags only?**
   - What we know: D-07 (revised) says it must be a real tag; warn if older than `svu current`.
   - Recommendation: `git rev-parse --verify "refs/tags/<value>^{commit}"` (exists at all) for the hard check; `git merge-base --is-ancestor <value-tag> HEAD` + compare to `svu current` for the "older than current" warning.

3. **Where does the `go run` migration helper live — `cmd/` package, or a flag on `cmd/server`?**
   - What we know: Claude's discretion (D-01, D-19). `cmd/server` already sequences `db.RunMigrations`.
   - Recommendation: a tiny dedicated `cmd/` package (e.g. `cmd/migrate`) — keeps `cmd/server`'s boot path untouched and is trivially `go run`-able; add it to the same `COVER_PKGS` / `gosec` carve-out list if it reads any path from argv (it only reads `DATABASE_URL` from env, so `gosec` G304 likely does not apply to it — confirm).

4. **Sustained-green window for `/health` — how long?**
   - What we know: Claude's discretion (D-03). The N-1 image's poller has a 15-minute interval, so nothing competes inside a short window.
   - Recommendation: 8 consecutive 200s at 2 s spacing (~16 s) after the first 200 — long enough to catch a delayed crash, short enough for CI. Total boot-job budget ~2–3 min.

---

## Environment Availability

| Dependency | Required by | Available | Version | Fallback |
|---|---|---|---|---|
| `golang-migrate/migrate/v4` | Finding 1 fix + D-02 test | ✓ (in `go.mod`) | v4.19.1 | — |
| Go toolchain | everything | ✓ | 1.26.5 (`go.mod` `go 1.26`) | — |
| `docker` on the CI runner | `n1-boot` job (pull + run N-1 image, run Postgres) | ✓ (ubuntu-latest; `build-scan` already uses Docker) | — | — |
| `postgres:16` image | `n1-boot` throwaway DB | ✓ (used by `docker-compose.yml`) | 16 | — |
| `svu` | `n1-boot` previous-tag resolution | ✓ via `go install` | v3.4.1 (pinned, matches `release`) | — |
| ghcr.io `drop-tracker` images | `n1-boot` pulls the N-1 image | ✓ (public, `release` job publishes `vX.Y.Z` tags) | latest = `v1.7.0` | D-04: red if a prior tag exists but the pull fails; skip-green only if no prior tag at all |
| cgo / C toolchain | *(would be needed for `pg_query_go` — NOT used)* | ✗ (documented broken on dev box) | — | Hand-rolled scanner (D-06) — no cgo |
| `go test -race` on the dev box | running the full `internal/db` suite locally | ✗ (documented TSan failure on this Windows box) | — | Plain `go test` locally; `-race` runs in CI (`make test-integration`) |

**Missing dependencies with no fallback:** none.
**Missing with fallback:** cgo (→ hand-rolled scanner); local `-race` (→ CI runs it).

---

## Validation Architecture

`workflow.nyquist_validation: true` `[VERIFIED: .planning/config.json — read this session]` — section included.

### Test Framework

| Property | Value |
|---|---|
| Framework | Go `testing` (stdlib) + real Postgres via `make test-integration` |
| Config file | none — `Makefile` `test-integration` target; `TEST_DATABASE_URL` env |
| Quick run command | `go test ./internal/db/... ./cmd/migration-check/... -count=1` (add `-short` to skip DB-backed) |
| Full suite command | `make test-integration` (→ `make db-up` then `go test ./... -race -count=1 -coverprofile=coverage.out -coverpkg=$(COVER_PKGS)`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test type | Automated command | File exists? |
|---|---|---|---|---|
| MGRT-01 (SC #4) | `RunMigrations` no-ops (returns `nil`) against a `schema_migrations` version ahead of the embedded source | integration (real Postgres, scratch schema) | `go test ./internal/db -run TestRunMigrationsWithSource_NoOpsAgainstAheadOfSource -count=1` | ❌ Wave 0 (`internal/db/migrate_ahead_test.go`, `package db`) |
| MGRT-01 (SC #4, negative) | `RunMigrations` still errors on a **dirty** ahead-of-source DB (not masked) | integration | same file, `_DirtyAheadStillErrors` case | ❌ Wave 0 |
| MGRT-01 (SC #4, forward) | a normal forward migration (DB behind source) still applies fully | integration | existing `TestRunMigrations_AppliesFromScratch` + a new "DB behind by 1" case | ⚠️ extend `migrate_test.go` |
| MGRT-01 (SC #1/#2, boot) | previously-released image boots, `/health` sustained 200, `GET /watchlist`+`/events` 200, `POST /watchlist` 201 + row reads back | e2e in CI (`n1-boot` job) | the `n1-boot` job steps | ❌ Wave 0 (workflow) |
| MGRT-01 (SC #2, static) | a branch adding `DROP COLUMN` / `RENAME` / type-narrow / `ADD NOT NULL` (no default) turns the guard red with a class-specific message naming the N-1 rule | unit | `go test ./cmd/migration-check -run TestScan -count=1` (SQL fixtures in `testdata/`) | ❌ Wave 0 |
| MGRT-01 (D-15) | a `DROP COLUMN` of a column still referenced by the previous release's `queries/*.sql` turns the guard red even with an `allow-destructive` annotation | unit | `go test ./cmd/migration-check -run TestPrevReleaseCrossRef -count=1` (injected `gitShow` stub) | ❌ Wave 0 |
| MGRT-01 (D-07) | `allow-destructive` with both keys + a real tag suppresses a backward-incompatible finding and echoes the reason; missing a key / bad tag does not | unit | `go test ./cmd/migration-check -run TestAnnotation -count=1` | ❌ Wave 0 |
| MGRT-01 (D-16) | diff-base selection: PR uses merge-base, push uses `before..sha`, zero-SHA falls back to merge-base | unit (extract the range logic into a testable Go helper, or a `bats`-free shell test) | `go test ./cmd/migration-check -run TestDiffRange` if the logic is in Go | ❌ Wave 0 (prefer putting range logic in Go, not YAML bash) |
| MGRT-02 | `internal/db/migrations/README.md` exists and documents both rules + the `000007` walkthrough + the checklist + the annotation syntax | doc presence + a grep test | `go test ./internal/db -run TestMigrationsReadmeMentionsRules` (assert key phrases present) — optional but on-brand (repo has `.env.example` parity tests) | ❌ Wave 0 (optional) |

### Sampling Rate

- **Per task commit:** `go vet ./...`; `golangci-lint run`; `go test ./internal/db/... ./cmd/migration-check/... -count=1` (the two packages this phase touches). Full Definition of Done before every commit (CLAUDE.md).
- **Per wave merge:** `make test` + `make coverage-gate` + `make sqlc-check` (no sqlc change expected, but the gate is cheap).
- **Phase gate:** full `make test-integration` green; a real CI run on a scratch branch that (a) adds a dummy destructive migration and confirms the guard + `n1-boot` go red, (b) confirms `build-scan` still runs on a non-migration commit (Finding 2 smoke test), (c) removes the dummy and confirms green. Phase 9 set this scratch-branch-CI-verification precedent.

### Wave 0 Gaps

- [ ] `internal/db/migrate_ahead_test.go` (`package db`) — MGRT-01 SC #4, drives `runMigrationsWithSource` with a `fstest.MapFS` truncated source. **Task 1 / checkpoint.**
- [ ] `internal/db/migrate.go` — the `runMigrationsWithSource` seam (D-18) + the ahead-of-source guard + `maxSourceVersion` helper.
- [ ] `cmd/migration-check/` — `main.go` + `main_test.go` + `testdata/*.sql` fixtures (mirror `cmd/coverage-report`).
- [ ] `cmd/migrate/` (or equivalent) — the `go run` HEAD-schema helper.
- [ ] `.golangci.yml` — `- path: '^cmd/migration-check/'` → `linters: [gosec]` (G304); confirm whether `cmd/migrate` needs it too.
- [ ] `Makefile` — extend `COVER_PKGS` grep: `'(^|/)(internal/db/sqlc|cmd/coverage-report|cmd/migration-check)$$'` (+ the helper if it has no tests).
- [ ] `.github/workflows/full-pipeline.yml` — `changes` job, `migration-check` job, `n1-boot` job; append the latter two to `build-scan.needs:`.
- [ ] `internal/db/migrations/README.md` — MGRT-02.
- [ ] `.claude/CLAUDE.md` — one-line pointer in the Definition of Done section.
- [ ] Framework install: none (Go `testing` + Postgres already wired).

---

## Security Domain

`workflow.security_enforcement: true`, `security_asvs_level: 1` `[VERIFIED: .planning/config.json — read this session]`.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard control |
|---|---|---|
| V1 Encoding / Injection | yes | `cmd/migration-check` shells `git show <tag>:<path>` — build argv as a slice (`exec.Command("git", "show", tag+":"+path)`), never `sh -c` with interpolation. Validate `<tag>` against `^v?[0-9]+(\.[0-9]+){0,2}(-[0-9A-Za-z.-]+)?$` and `<path>` against a fixed allowlist (`queries/*.sql`, `internal/db/migrations/*.up.sql`, `internal/db/sqlc/*.go`) before it reaches argv. The tag comes from `svu current` (tool output) and from the `expand-shipped-in` annotation (**attacker-influenceable** via a migration file in a PR) — validate the annotation value hard. |
| V5 Input Validation | yes | Migration SQL and `queries/*.sql` are treated as **untrusted text** by the scanner — it only reads and pattern-matches, never executes. The `n1-boot` job's `POST /watchlist` body is a fixed literal. |
| V6 Cryptography | no | — |
| V2 / V3 / V4 (authn / session / access control) | no | The `n1-boot` image runs with `INSTANCE_PASSPHRASE` unset (GATE-07 inert) — deliberate, so unauthenticated probes work; the container is ephemeral and only reachable from the runner. |
| V12 Files & Resources | yes | `cmd/migration-check` reads file paths from argv/env → `gosec` G304 carve-out (D-19b), scoped to that directory only, mirroring `cmd/coverage-report`. Paths are CI-controlled, not end-user input. |
| V14 Config / CI | yes | New jobs: `permissions: contents: read` only (no `packages: write`, no PR write — public ghcr package). SHA-pin any new `uses:`. No new secret. `svu` pinned to `v3.4.1`. `postgres:16` — pin to a digest if the repo starts pinning base images (it currently uses the floating `postgres:16` tag in compose, so `postgres:16` is consistent). |

### Known Threat Patterns for this phase

| Pattern | STRIDE | Standard mitigation |
|---|---|---|
| Malicious `expand-shipped-in=<value>` in a PR migration comment → command injection into `git show` / `git rev-parse` | Tampering / Elevation | Strict regex allowlist on the tag value **before** it reaches any `exec.Command`; argv slice form, never a shell string |
| A PR adds a migration that drops a column the poller still writes, with a plausible `allow-destructive` annotation | Tampering (data integrity on rollback) | D-15 cross-reference reds the build regardless of the annotation; the `n1-boot` `POST /watchlist` probe is the behavioural backstop |
| `n1-boot` job pulls a tampered `:latest`-style tag | Spoofing | D-13 resolves an immutable `vX.Y.Z` tag via `svu current`, never `:latest`; Phase 17 will additionally pin by digest |
| Skip-propagation silently disables the gate (Finding 2) | Repudiation / gate bypass | Jobs unconditional; scratch-branch CI smoke test confirms `build-scan` runs on non-migration commits and skips nothing |
| Guard passes because it scanned zero files (bad diff base) | Gate bypass | `changes` job's zero-SHA / unreachable-`before` fallback to merge-base; the guard logs the file list it scanned (D-07 "echoes into its output") |
| `n1-boot` Postgres container exposed on the runner | Info disclosure | ephemeral, `localhost`-only, throwaway credentials (`drop_tracker`/`drop_tracker`), torn down with the job |

---

## Sources

### Primary (HIGH confidence)

- `golang-migrate/migrate/v4@v4.19.1` module source, read this session: `migrate.go` (`Up` 265-283, `readUp` 532-601, `versionExists` 776-810, `Version` 381-394), `source/iofs/iofs.go` (95-158), `database/stub/stub.go` — the ahead-of-source error path.
- **Executed falsification** this session: standalone program using `iofs.New(fstest.MapFS{…})` + `database/stub` against `v4.19.1` — `Up()` returns `"no migration found for version N+1"` (`errors.Is(err, os.ErrNotExist) == true`, `errors.Is(err, migrate.ErrNoChange) == false`); prototype fix verified across 4 cases.
- Repo source read this session: `internal/db/migrate.go`, `internal/db/migrate_test.go`, `internal/db/migrations/000001..000007 *.up.sql` + `*.down.sql`, `queries/artists.sql|events.sql|health.sql|watchlist.sql`, `internal/config/config.go`, `internal/httpserver/health.go|watchlist.go|events.go`, `internal/watchlist/service.go` (`Add`), `cmd/server/main.go`, `cmd/coverage-report/main.go|main_test.go`, `.github/workflows/full-pipeline.yml`, `Makefile`, `.golangci.yml`, `docker-compose.yml`, `.pre-commit-config.yaml`, `.planning/config.json`, `.planning/research/ARCHITECTURE.md` §"Boot-time migrations × rollback", `.planning/research/PITFALLS.md` Pitfalls 8/9/10.
- `git tag` / `git describe --tags --abbrev=0` run this session → tags `v0.1.0 … v1.7.0`, current `v1.7.0`.

### Secondary (MEDIUM confidence)

- [GitHub Docs — Using jobs in a workflow](https://docs.github.com/actions/using-jobs/using-jobs-in-a-workflow) and [Using conditions to control job execution](https://docs.github.com/en/actions/using-jobs/using-conditions-to-control-job-execution) — a skipped `needs:` job skips its dependents unless a status function is used; a skipped job reports status "success" for branch-protection purposes.
- [github.com/orgs/community/discussions/45058](https://github.com/orgs/community/discussions/45058), [#26945](https://github.com/orgs/community/discussions/26945), [#25224](https://github.com/orgs/community/discussions/25224) — corroborating community reports of skip-propagation through `needs:`.
- [latchkey.dev — GitHub Actions: "Job X has been skipped" (needs result)](https://latchkey.dev/learn/github-actions/github-actions-job-skipped-needs-result) — the `if: ${{ !cancelled() && needs.X.result == 'success' }}` pattern.
- `ARCHITECTURE.md` sources list: golang-migrate issues [#702](https://github.com/golang-migrate/migrate/issues/702), [#1100](https://github.com/golang-migrate/migrate/issues/1100) (the original MEDIUM-confidence basis — now superseded by the executed test).

### Tertiary (LOW confidence)

- None relied upon.

---

## Metadata

**Confidence breakdown:**
- Finding 1 (golang-migrate ahead-of-source behaviour + fix): **HIGH** — executed against the pinned module, not inferred.
- Finding 2 (skip-propagation): **HIGH** — GitHub docs, multiple corroborating sources; planner should still scratch-branch smoke-test.
- Standard stack / no-new-deps: **HIGH** — verified against `go.mod`, `Makefile`, workflow.
- D-15 extraction scope + blind spots: **MEDIUM** — grounded in the 4 current query files; conservatism tuning is a judgement call.
- D-08 detectability matrix: **MEDIUM-HIGH** — the reliably-detectable set is solid; type-narrowing "flag everything, require annotation" is a deliberate over-approximation.
- GitHub Actions wiring details (`--network host` reachability, exact `changes` bash): **MEDIUM** — standard patterns, first CI run is the confirmation.

**Research date:** 2026-09-04
**Valid until:** ~2026-10-04 for the golang-migrate finding (pinned version — stable); re-check the GitHub Actions skip-propagation behaviour if the platform changes it (tracked in community#45058).
