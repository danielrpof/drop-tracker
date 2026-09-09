---
phase: 16-rollback-safe-migrations
verified: 2026-09-04T00:00:00Z
status: passed
human_verified_at: 2026-09-05T17:10:00Z
human_verified_note: "UAT test 1 ran live (see 16-UAT.md). Surfaced gap G-16-1 (n1-boot false-red during the guard-adoption window); fixed by quick task 260905-et1 (n1-boot guardcheck step) + 260905-fa4 (blocking trivy-fs CVEs), and confirmed on CI runs 33978945980 (guardcheck fires) and 33979094225 (build-scan runs). The N-1 boot job's live-behavior dimension is satisfied via the documented guard-adoption skip-green path until a guard-carrying release becomes N-1."
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "On a scratch branch (never main, delete after — Phase 09 precedent): (a) add a throwaway migration DROP-ping a column the previous release's queries still reference and push — expect migration-check red (D-15 cross-reference message naming the tag/query) and n1-boot red (N-1 rule message), build-scan not reached. (b) remove the dummy migration, push a docs-only commit — expect changes reports no migration change, migration-check and n1-boot both green in seconds with expensive steps shown skipped, and build-scan RUNS (not \"Skipped\"). (c) push a purely additive nullable ADD COLUMN migration — expect both checks green with n1-boot having actually pulled and booted the previous release image."
    expected: "(a) both guard jobs red, build-scan not reached; (b) build-scan runs despite migration-check/n1-boot being trivially green — confirms RESEARCH Finding 2's skip-propagation fix holds on a real GitHub Actions run; (c) n1-boot genuinely pulls ghcr.io image at the previous release tag and passes health/read/write probes."
    why_human: "GitHub's skip-propagation semantics, ghcr.io registry state (a half-failed release could leave a tag with no image), and `--network host` sibling-container reachability on the actual runner are facts about the live world that cannot be confirmed by reading the workflow YAML or by any test that runs in this repo checkout. This is Task 3 of 16-04-PLAN.md's deliberately-deferred `<human-check>` (workflow.human_verify_mode=end-of-phase); 16-04-SUMMARY.md documents the deferral explicitly and it is not treated as a gap here."
---

# Phase 16: Rollback-Safe Migrations Verification Report

**Phase Goal:** Rolling the app image back to its previous release is *provably* safe against the schema the newer release already applied — enforced by a CI check and a written rule, not by remembering.
**Verified:** 2026-09-04
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A CI check boots the previously-released image against a database migrated to the current branch's schema and passes only if that older binary starts and stays healthy | ⚠️ PRESENT + WIRED, live-behavior unverified | `n1-boot` job exists in `.github/workflows/full-pipeline.yml` (lines 380-528), unconditional with 11 step-level gates on `needs.changes.outputs.migrations_changed`, no `services:` key, pulls `ghcr.io/danielrpof/drop-tracker:$PREV_TAG` (immutable tag only, `LATEST_HITS=0` confirmed by direct grep), sustained-8x-200 health probe, read probes (`GET /watchlist`, `GET /events`), write probe (`POST /watchlist` with `image_url` supplied, confirmed via `internal/watchlist/service.go:203` that this bypasses the art matcher). Structurally complete and correctly wired; **has never been exercised against a real GitHub Actions runner** — routed to human verification below. |
| 2 | A branch carrying a destructive migration turns that check red with a message naming the N-1 compatibility rule rather than an opaque boot error | ✓ VERIFIED | Live run: `go run ./cmd/migration-check --mode=scan --files='cmd/migration-check/testdata/drop_column.sql'` → exit 1, message names "expand/contract rule and the N-1 rollback invariant" and cites `internal/db/migrations/README.md`. 44 test cases pass in `cmd/migration-check`, including `TestPrevReleaseCrossRef_AnnotationCannotOverride` and (post-code-review-fix) `TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed`. |
| 3 | The expand/contract rule is documented as a standing constraint where someone writing a migration will actually encounter it | ✓ VERIFIED | `internal/db/migrations/README.md` (138 lines) present with both rule classes stated separately, N-1 invariant, 000006/000007 walkthrough, pre-merge checklist, and the exact annotation syntax — all required phrases greppable and pinned by `TestMigrationsReadme_ContainsRequiredPhrases` (live run: PASS). `.claude/CLAUDE.md` carries the one-line pointer (line 9, confirmed by direct read). |
| 4 | The older binary's boot migration succeeds against an ahead-of-source schema (no-ops rather than failing), proven by an automated test | ✓ VERIFIED | Live run against real Postgres: `TestRunMigrationsWithSource_NoOpsAgainstAheadOfSource`, `_DirtyAheadStillErrors`, `_BehindBySourceStillMigratesForward`, `_FreshDatabaseAppliesFromScratch`, `_IsIdempotentAtEqualVersion` — all 5 PASS in 16.975s against a scratch Postgres schema. Guard code inspected directly in `internal/db/migrate.go:298-302`; matches the documented predicate exactly (`verr == nil && !dirty` then `ok && cur > smax` → nil). |

**Score:** 4/4 truths verified (0 present-behavior-unverified at the truth level; the live-CI dimension of Truth 1 is a separate, explicitly-deferred UAT item per workflow.human_verify_mode=end-of-phase, not a truth failure)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/db/migrate.go` | `runMigrationsWithSource` seam + `maxSourceVersion` + ahead-of-source guard | ✓ VERIFIED | Read in full; matches PLAN spec exactly. `RunMigrations` exported signature byte-identical; `cmd/server/main.go:137` call site unchanged (`db.RunMigrations(ctx, cfg.DatabaseURL, logger)`). |
| `internal/db/migrate_ahead_test.go` | SC #4 hermetic proof | ✓ VERIFIED | 5 tests, all pass live against real Postgres. |
| `cmd/migrate/main.go` | `go run` HEAD-schema helper | ✓ VERIFIED | `db.RunMigrations(` present, no `os.Args` use; `TestRun_MissingDatabaseURL` and `TestRun_AppliesHeadSchema` pass live. |
| `cmd/migration-check/main.go` | Two-class DDL guard, changed-files mode, D-15 cross-reference | ✓ VERIFIED | 44 passing tests (live run); behavior confirmed via two direct CLI invocations against real fixtures and a real repo migration file. |
| `internal/db/migrations/README.md` | MGRT-02 standing constraint doc | ✓ VERIFIED | 138 lines, all 8 required phrase-groups present via direct grep. |
| `internal/db/migrations_readme_test.go` | Doc-presence drift guard | ✓ VERIFIED | Live run: PASS, all 9 phrase subtests + length floor. |
| `.claude/CLAUDE.md` | One-line pointer | ✓ VERIFIED | Line 9 confirmed by direct read; six numbered DoD gates unchanged. |
| `Makefile` | `COVER_PKGS` excludes `cmd/migrate`, `cmd/migration-check` | ✓ VERIFIED | Line 38 confirmed by grep. |
| `.golangci.yml` | gosec carve-out `^cmd/migration-check/` | ✓ VERIFIED | Confirmed by grep, scoped correctly. |
| `.github/workflows/full-pipeline.yml` | `changes`, `migration-check`, `n1-boot` jobs + `build-scan.needs:` extension | ✓ VERIFIED | Read in full (lines 296-539). All three jobs unconditional (no job-level `if:`), `n1-boot` has no `services:` key and 11 step-level gates, `build-scan.needs:` is exactly the 6 original entries plus `migration-check, n1-boot` appended. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/db/migrate.go` (`RunMigrations`) | `runMigrationsWithSource` | direct call | ✓ WIRED | Confirmed in source. |
| `cmd/migrate/main.go` | `internal/db.RunMigrations` | direct call | ✓ WIRED | Confirmed by grep + passing test. |
| `cmd/migration-check` findings | `internal/db/migrations/README.md` | message text | ✓ WIRED | Confirmed by live CLI output containing the literal path. |
| `.github/workflows/full-pipeline.yml` (`changes` job) | `cmd/migration-check --mode=changed-files` | shell invocation | ✓ WIRED | Confirmed by direct read of the job step. |
| `.github/workflows/full-pipeline.yml` (`n1-boot` job) | `cmd/migrate` | `go run ./cmd/migrate` step | ✓ WIRED | Confirmed by direct read (line 442). |
| `.github/workflows/full-pipeline.yml` (`build-scan`) | `migration-check`, `n1-boot` jobs | `needs:` array | ✓ WIRED | Confirmed by direct read (line 539): `needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test, migration-check, n1-boot]`. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Ahead-of-source guard no-ops against real Postgres | `go test ./internal/db/... -run TestRunMigrationsWithSource -v` | 5/5 PASS | ✓ PASS |
| Destructive migration reds with correct message | `go run ./cmd/migration-check --mode=scan --files='cmd/migration-check/testdata/drop_column.sql'` | exit 1, README-citing message | ✓ PASS |
| Safe migration passes | `go run ./cmd/migration-check --mode=scan --files='internal/db/migrations/000007_...up.sql'` | exit 0 | ✓ PASS |
| `cmd/migration-check` full suite | `go test ./cmd/migration-check/... -v` | 44 tests, 0 FAIL/SKIP | ✓ PASS |
| `cmd/migrate` suite | `go test ./cmd/migrate/... -v` | 2/2 PASS | ✓ PASS |
| README doc-presence test | `go test ./internal/db/... -run TestMigrationsReadme -v` | PASS | ✓ PASS |
| `go vet ./...` | — | clean | ✓ PASS |
| `golangci-lint run` | — | 0 issues | ✓ PASS |
| `make sqlc-check` | — | clean (no drift) | ✓ PASS |
| Live GitHub Actions run of `n1-boot`/skip-propagation | — | not run | ? SKIP (routed to human verification) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| MGRT-01 | 16-01, 16-02, 16-03, 16-04 | CI check boots previous image, fails on backward-incompatible migration | ✓ SATISFIED | All artifacts + tests verified above; `REQUIREMENTS.md` marks `[x]` and traceability table shows `Complete`. Live-CI confirmation is the sole open item (human_needed). |
| MGRT-02 | 16-05 | Expand/contract rule documented as standing constraint | ✓ SATISFIED | README + doc-presence test + CLAUDE.md pointer all verified live. `REQUIREMENTS.md` marks `[x]` / `Complete`. |

No orphaned requirements — REQUIREMENTS.md maps exactly MGRT-01 and MGRT-02 to Phase 16, and both plans across the 5 SUMMARYs declare them.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` found in any of the 8 phase-modified core files | — | Clean |
| `cmd/migration-check/main.go` | 791, 1440-1445 | WR-01 (from 16-REVIEW.md): `rename_column` finding's `object` display string doubles as a data-extraction source for the D-15 cross-reference lookup — a future display-format edit could silently break the lookup with no compiler/test signal | ℹ️ Info | Non-blocking; explicitly tracked as a documented follow-up in 16-REVIEW.md's Post-Review Outcome, not a functional defect today (regression-tested paths still pass). Not a must-have failure. |
| `cmd/migration-check/main.go` (pre-fix) | 1437-1450 | CR-01 (from 16-REVIEW.md): schema-qualified table names bypassed the D-15 non-overridable cross-reference | — | **Already fixed** in commit `13a1a24`, confirmed live: `TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed` passes against current code. |

### Human Verification Required

### 1. Live scratch-branch CI run — skip-propagation, registry state, and n1-boot behavior

**Test:** On a scratch branch (never `main`, delete after — Phase 09 precedent `test/coverage-gate-ci-check`):
(a) Add a throwaway migration `DROP COLUMN`-ing a column the previous release's queries still reference, push. (b) Remove the dummy migration, push a docs-only commit. (c) Push a purely additive nullable `ADD COLUMN` migration.

**Expected:**
(a) `migration-check` red (D-15 cross-reference message naming the tag and query) and `n1-boot` red (N-1 rule message), `build-scan` not reached.
(b) `changes` reports no migration change; `migration-check` and `n1-boot` both green in seconds with their expensive steps shown as skipped; **`build-scan` runs** (not "Skipped") — this is the single highest-value observation, confirming RESEARCH Finding 2's skip-propagation fix holds on a real run rather than only in the static YAML-shape assertion already verified above.
(c) Both checks green, with `n1-boot` having genuinely pulled and booted the previous release image against a throwaway Postgres.

**Why human:** This exercises facts about the live world — GitHub Actions' actual skip-propagation semantics, whether ghcr.io genuinely has an image pushed for the tag `svu current` resolves to (a half-failed release could leave a tag with no image), and whether a container on `--network host` can reach a sibling Postgres container on the actual runner. None of these are observable by reading the workflow YAML or by any test executable from this repository checkout. This is Task 3 of `16-04-PLAN.md`'s own `<human-check>` block, deliberately deferred by the executor per `workflow.human_verify_mode=end-of-phase` (documented explicitly in `16-04-SUMMARY.md`'s "Scope Note" section) — not an execution gap.

### Gaps Summary

No gaps found. All must-have truths, artifacts, and key links are present, substantive, and correctly wired; every automated test claimed in the SUMMARYs was independently re-run in this verification session (not merely trusted from SUMMARY text) and passed, including the two Postgres-integration suites, the 44-case `cmd/migration-check` table, and the doc-presence test. The one code-review finding that would have been a functional gap (CR-01, schema-qualified table names bypassing the non-overridable D-15 check) was already found and fixed with a regression test during this same phase's own review cycle, and that regression test passes against current code. The single open item is a legitimate, explicitly-deferred live-CI verification (RESEARCH Finding 2's skip-propagation behavior, ghcr.io registry state, and runner container reachability) that cannot be confirmed by any check runnable from a local checkout — it is routed to human verification / end-of-phase UAT exactly as the deferred `<human-check>` block and this phase's `human_verify_mode` config specify.

---

_Verified: 2026-09-04_
_Verifier: Claude (gsd-verifier)_
