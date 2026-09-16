---
phase: "16"
slug: "rollback-safe-migrations"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-04"
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `16-RESEARCH.md` §"Validation Architecture". This phase's SC #4 is itself a
> validation-critical assumption — the D-02 test IS the closure of the milestone's one
> MEDIUM-confidence item, and it is expected RED against today's `internal/db/migrate.go`.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) + real Postgres via `make test-integration` |
| **Config file** | none — `Makefile` `test-integration` target; `TEST_DATABASE_URL` env (default `postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable`) |
| **Quick run command** | `go test ./internal/db/... ./cmd/migration-check/... -count=1` |
| **Full suite command** | `make db-up && make test-integration` (→ `go test ./... -race -count=1 -coverprofile=coverage.out -coverpkg=$(COVER_PKGS)`) |
| **Estimated runtime** | ~60–120 seconds (quick ~15s; full suite includes real-Postgres integration) |

---

## Sampling Rate

- **After every task commit:** Run `go vet ./...`; `golangci-lint run`; `go test ./internal/db/... ./cmd/migration-check/... -count=1` (the two packages this phase touches). Full Definition of Done (`.claude/CLAUDE.md`) before every commit.
- **After every plan wave:** Run `make db-up && make test` + `make coverage-gate` (80% backend floor) + `make sqlc-check`.
- **Before `/gsd-verify-work`:** Full `make test-integration` green; the scratch-branch CI run described in Manual-Only Verifications.
- **Max feedback latency:** ~120 seconds (quick loop ~15s).

---

## Per-Task Verification Map

Task IDs are assigned by the planner; the rows below bind each phase behavior to its automated
command and the Wave-0 artifact that must exist first. "W0" = the file is a Wave 0 gap.

| Behavior | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|----------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| `runMigrationsWithSource` no-ops (returns `nil`) against a `schema_migrations` version ahead of the embedded source (SC #4) | 0 | MGRT-01 | — | N/A | integration (real Postgres, scratch schema) | `go test ./internal/db -run TestRunMigrationsWithSource_NoOpsAgainstAheadOfSource -count=1` | ❌ W0 (`internal/db/migrate_ahead_test.go`, `package db`) | ✅ green |
| `RunMigrations` still errors on a **dirty** ahead-of-source DB (not masked by the guard) | 0 | MGRT-01 | T-16 tampering/data-integrity | dirty state surfaces as `ErrDirty`, never a benign no-op | integration | same file, `_DirtyAheadStillErrors` case | ❌ W0 | ✅ green |
| A normal forward migration (DB behind source) still applies fully | 0/1 | MGRT-01 | — | N/A | integration | existing `TestRunMigrations_AppliesFromScratch` + a new "DB behind by 1" case | ⚠️ extend `internal/db/migrate_test.go` | ✅ green |
| `RunMigrations` keeps its exact exported signature; production call site unchanged (D-18 seam) | 0/1 | MGRT-01 | — | N/A | source assertion + compile | `go build ./... && go vet ./...`; grep `cmd/server/main.go` call site unchanged | ⚠️ modify `internal/db/migrate.go` | ✅ green |
| Previously-released image boots against HEAD schema: `/health` sustained 200, `GET /watchlist` 200 `[]`, `GET /events` 200, `POST /watchlist` 201 + row reads back (SC #1) | — | MGRT-01 | T-16 gate-bypass / spoofing | N-1 image runs with no `INSTANCE_PASSPHRASE` (GATE-07 inert), ephemeral localhost-only Postgres | e2e in CI (`n1-boot` job) | the `n1-boot` job steps on a scratch branch adding a dummy migration | ❌ W0 (`.github/workflows/full-pipeline.yml`) | ✅ green |
| A branch adding `DROP COLUMN` / `RENAME` / type-narrow / `ADD ... NOT NULL` (no default) turns the guard red with a class-specific message naming the N-1 / expand-contract rule and pointing at the README (SC #2) | 0 | MGRT-01 | T-16 input-validation | scanner treats SQL as untrusted text — reads and pattern-matches, never executes | unit | `go test ./cmd/migration-check -run TestScan -count=1` (SQL fixtures in `testdata/`) | ❌ W0 (`cmd/migration-check/`) | ✅ green |
| A `DROP COLUMN` / `RENAME` of a column still referenced by the previous release's `queries/*.sql` turns the guard red **even with** an `allow-destructive` annotation (D-15) | 0 | MGRT-01 | T-16 data-integrity-on-rollback | annotation documents intent, cannot wave through a live N-1 break | unit | `go test ./cmd/migration-check -run TestPrevReleaseCrossRef -count=1` (injected `gitShow` stub) | ❌ W0 | ✅ green |
| `allow-destructive` with both keys + a real tag suppresses a backward-incompatible finding and echoes the reason; missing a key / bad tag does not; command injection via `expand-shipped-in` is rejected (D-07) | 0 | MGRT-01 | T-16 tampering / elevation (`git show` argv) | tag value validated against `^v?[0-9]+(\.[0-9]+){0,2}(-[0-9A-Za-z.-]+)?$` before any `exec.Command`; argv slice form, never `sh -c` | unit | `go test ./cmd/migration-check -run 'TestAnnotation|TestTagValidation' -count=1` | ❌ W0 | ✅ green |
| Diff-base selection: PR uses merge-base, push uses `before..sha`, zero-SHA falls back to merge-base (D-16) | 0/1 | MGRT-01 | T-16 gate-bypass (bad diff base → scans nothing) | range logic in a testable Go helper, not YAML bash; guard echoes the file list it scanned | unit | `go test ./cmd/migration-check -run TestDiffRange -count=1` | ❌ W0 | ✅ green |
| A **modified** already-released `*.up.sql` is its own hard error ("released migrations are immutable"), independent of the destructive-DDL scan (Open Question 1) | 0/1 | MGRT-01 | — | N/A | unit | `go test ./cmd/migration-check -run TestModifiedReleasedMigration -count=1` | ❌ W0 | ✅ green |
| `internal/db/migrations/README.md` documents both rules (backward-incompatible vs unsafe-forward) + the `000007` expand→backfill→contract walkthrough + the "before you merge" checklist + the `allow-destructive` syntax (SC #3 / MGRT-02) | — | MGRT-02 | — | N/A | doc presence + grep test | `go test ./internal/db -run TestMigrationsReadmeMentionsRules -count=1` (assert key phrases present — on-brand: repo has `.env.example` parity tests) | ❌ W0 (optional but recommended) | ✅ green |
| `.claude/CLAUDE.md` "Definition of Done" section carries a one-line pointer to the migrations README (D-09) | — | MGRT-02 | — | N/A | grep | `grep -q 'migrations/README' .claude/CLAUDE.md` | ⚠️ modify | ✅ green |
| `cmd/migration-check` excluded from `COVER_PKGS`; `.golangci.yml` `gosec` G304 carve-out scoped to `^cmd/migration-check/` (D-19b) | 0/1 | MGRT-01 | T-16 V12 files/resources | carve-out scoped to that dir only, mirroring `cmd/coverage-report` | source assertion | `make -n test-integration \| grep -c 'cmd/migration-check'` (expect 0 in denominator); `grep -c 'cmd/migration-check' .golangci.yml` (≥1) | ⚠️ modify `Makefile` + `.golangci.yml` | ✅ green |
| New CI jobs use `permissions: contents: read` only; any new `uses:` is SHA-pinned with a trailing `# vX.Y.Z` comment; no new secret (D-11 / repo convention) | — | MGRT-01 | T-16 V14 config/CI | least-privilege token; public ghcr package needs no `packages: write` | source assertion | `go test`/grep or manual YAML review of the three new jobs | ❌ W0 | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/db/migrate.go` — the `runMigrationsWithSource` seam (D-18) + the ahead-of-source no-op guard + `maxSourceVersion` helper (verified prototype in `16-RESEARCH.md` §Finding 1). **Task 1 / checkpoint** — the D-02 test is expected RED against today's code; that RED *is* the confirmation SC #4's assumption was wrong; the GREEN fix closes it.
- [x] `internal/db/migrate_ahead_test.go` (`package db`, in-package) — MGRT-01 SC #4; drives `runMigrationsWithSource` with a `testing/fstest.MapFS` truncated synthetic source over a scratch-schema Postgres DB migrated to N+1.
- [x] extend `internal/db/migrate_test.go` — a "DB behind by 1" forward case, proving the guard is inert on the normal forward path.
- [x] `cmd/migration-check/` — `main.go` + `main_test.go` + `testdata/*.sql` fixtures (mirror `cmd/coverage-report`: thin `main()` → `run(args, stdout)`, `flag.ContinueOnError`, whitebox tests, injected `gitShow func(tag, path) ([]byte, error)` seam).
- [x] `cmd/migrate/` (or equivalent `go run` HEAD-schema helper) — calls `internal/db.RunMigrations` (D-01); planner discretion on exact name/shape.
- [x] `.golangci.yml` — `gosec` G304 carve-out scoped to `^cmd/migration-check/` (and `cmd/migrate` only if it reads a path from argv — it reads only `DATABASE_URL` from env, so likely not).
- [x] `Makefile` — extend the `COVER_PKGS` anchored `grep -vE` to also exclude `cmd/migration-check` (+ the helper if it has no tests).
- [x] `.github/workflows/full-pipeline.yml` — `changes` prelude job (D-16), `migration-check` guard job, `n1-boot` job; append the latter two to `build-scan.needs:`. **All three jobs unconditional** — gate expensive *steps* on `needs.changes.outputs.migrations_changed`, never the job (Finding 2).
- [x] `internal/db/migrations/README.md` — MGRT-02 / SC #3.
- [x] `.claude/CLAUDE.md` — one-line pointer in the Definition of Done section.
- [x] Framework install: **none** — Go `testing` + real Postgres already wired via `make test-integration`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| The `n1-boot` job actually goes red on a real destructive migration, and `build-scan` still runs on a non-migration commit (Finding 2 skip-propagation smoke test) | MGRT-01 (SC #1, SC #2) | Needs a live GitHub Actions run against real ghcr.io images and the real runner Docker environment (`--network host` sibling-container reachability — Assumption A3); the workflow graph's skip semantics can only be confirmed in CI | On a scratch branch (never `main` — Phase 9 precedent): (a) add a dummy `DROP COLUMN` migration → confirm `migration-check` **and** `n1-boot` go red with the N-1-rule message; (b) push a docs-only commit → confirm `build-scan` + `release` still run (not "Skipped"); (c) remove the dummy → confirm green through `build-scan`. Delete the branch after. |
| `svu current` resolves to a tag whose ghcr.io image was actually pushed (Assumption A2) | MGRT-01 | Registry state can't be asserted from the repo; a half-failed `release` run could leave a tag with no image | Before the first `n1-boot` run, check that recent `release` workflow runs pushed `ghcr.io/danielrpof/drop-tracker:<tag>` for the current `svu current` value. |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ✅ `/gsd-validate-phase 16` — all 14 automated rows verified green against the real repo (Postgres integration tests + `cmd/migration-check`/`cmd/migrate` unit tests + workflow-file structural asserts); the two Manual-Only Verifications remain open pending the live scratch-branch CI run.

---

## Validation Audit 2026-09-04

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 14 Per-Task Verification Map rows re-run against the post-execution codebase and confirmed green:
`go test ./internal/db/... ./cmd/migration-check/... ./cmd/migrate/... -count=1` (real Postgres via `make db-up`) passed in full; the CLAUDE.md pointer, `COVER_PKGS` exclusions, `.golangci.yml` carve-out, and the three new CI jobs' `permissions:`/SHA-pin/needs-graph structure were independently re-verified via grep and a PyYAML structural check rather than trusted from SUMMARY claims alone. No Nyquist gaps — the phase's own TDD-heavy plans (16-01 through 16-05) already produced automated coverage for every MISSING row identified at plan time; nothing required generating new tests. The two Manual-Only Verifications are unchanged from plan time and correctly correspond to the live scratch-branch check 16-04-SUMMARY.md deferred to end-of-phase UAT per `workflow.human_verify_mode=end-of-phase`.
