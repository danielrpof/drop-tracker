---
phase: 16-rollback-safe-migrations
plan: 01
subsystem: database
tags: [golang-migrate, postgres, ci, boot-migrations, rollback-safety]

# Dependency graph
requires:
  - phase: 15-pr-coverage-diff-comment
    provides: cmd/coverage-report structural template (thin main -> run(), whitebox tests, COVER_PKGS exclusion pattern) mirrored for cmd/migrate
provides:
  - runMigrationsWithSource unexported seam in internal/db/migrate.go (D-18)
  - maxSourceVersion helper + ahead-of-source no-op guard in runMigrationsOnce
  - hermetic real-Postgres proof that the guard's fresh/behind/equal paths are inert and its dirty-ahead path still errors
  - cmd/migrate go run helper applying HEAD's embedded schema via the app's own boot path
affects: [16-02-migration-check-scanner, 16-04-ci-wiring-n1-boot-job]

# Actuals (#2632)
actuals:
  tokens: 5000
  tasks: 3
  commits: 5

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-18 source seam: RunMigrations delegates to an unexported runMigrationsWithSource(ctx, dsn, logger, src source.Driver, opts...) so a test can drive the real boot path against a synthetic source without exporting migrationsFS"
    - "In-package test file (package db, not package db_test) to reach an unexported seam, with its own local copies of scratchSchemaDSN/RequirePostgresDSN to avoid an internal/testutil -> internal/db import cycle"
    - "cmd/ helper packages read config from env only, never argv, to avoid needing a gosec G304 carve-out"

key-files:
  created:
    - internal/db/migrate_ahead_test.go
    - cmd/migrate/main.go
    - cmd/migrate/main_test.go
  modified:
    - internal/db/migrate.go
    - Makefile

key-decisions:
  - "Confirmed RESEARCH.md Finding 1 live against the pinned golang-migrate v4.19.1: Up() against an ahead-of-source schema_migrations version returns a hard 'no migration found for version N+1' error, not ErrNoChange — the RED commit records the exact observed failure text as evidence"
  - "migrate_ahead_test.go cannot import internal/testutil (it would create an import cycle: testutil -> db, and this file is package db to reach the unexported seam) — reimplemented RequirePostgresDSN and scratchSchemaDSN locally rather than exporting new symbols from internal/db"
  - "MGRT-01 not marked complete: requirements.ready-ids reports it blocked because 16-02/16-03/16-04 also declare MGRT-01 and have not executed yet — marking it here would be premature"

patterns-established:
  - "Ahead-of-source boot guard: read m.Version() after migrate.NewWithInstance, before the Up() goroutine; nil error + not-dirty + cur > maxSourceVersion(src) => no-op nil. Dirty and fresh-DB (ErrNilVersion) cases fall through unchanged."

requirements-completed: []

coverage:
  - id: D1
    description: "The previously-released binary's boot migration no-ops against an ahead-of-source schema_migrations version instead of erroring (SC #4), proven end-to-end against a real Postgres, RED before the guard and GREEN after"
    requirement: "MGRT-01"
    verification:
      - kind: integration
        ref: "internal/db/migrate_ahead_test.go#TestRunMigrationsWithSource_NoOpsAgainstAheadOfSource"
        status: pass
    human_judgment: false
  - id: D2
    description: "Guard is inert on fresh-DB, behind-by-one, and equal-version paths, and still errors on a dirty ahead-of-source database"
    requirement: "MGRT-01"
    verification:
      - kind: integration
        ref: "internal/db/migrate_ahead_test.go#TestRunMigrationsWithSource_DirtyAheadStillErrors"
        status: pass
      - kind: integration
        ref: "internal/db/migrate_ahead_test.go#TestRunMigrationsWithSource_BehindBySourceStillMigratesForward"
        status: pass
      - kind: integration
        ref: "internal/db/migrate_ahead_test.go#TestRunMigrationsWithSource_FreshDatabaseAppliesFromScratch"
        status: pass
      - kind: integration
        ref: "internal/db/migrate_ahead_test.go#TestRunMigrationsWithSource_IsIdempotentAtEqualVersion"
        status: pass
    human_judgment: false
  - id: D3
    description: "go run ./cmd/migrate applies HEAD's embedded schema against DATABASE_URL through the exact db.RunMigrations path cmd/server uses at boot, and exits non-zero naming the required env var when it is empty"
    requirement: "MGRT-01"
    verification:
      - kind: integration
        ref: "cmd/migrate/main_test.go#TestRun_AppliesHeadSchema"
        status: pass
      - kind: unit
        ref: "cmd/migrate/main_test.go#TestRun_MissingDatabaseURL"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-04
status: complete
---

# Phase 16 Plan 01: Ahead-of-Source Boot Guard & HEAD-Schema Helper Summary

**The old binary's boot migration now no-ops against a newer schema instead of crash-looping — golang-migrate v4.19.1's real behavior was verified live to contradict the phase's founding assumption, and the fix + hermetic RED-then-GREEN proof + `cmd/migrate` CI helper all land together.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-04T09:04Z (first commit)
- **Completed:** 2026-09-04T09:14Z (last commit)
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Extracted `runMigrationsWithSource` unexported seam (D-18) from `RunMigrations`, byte-identical exported signature, production call site untouched
- Proved RESEARCH.md Finding 1 live: `migrate.Up()` against an ahead-of-source `schema_migrations` version returns a hard `no migration found for version N+1` error, not `ErrNoChange` — recorded as the RED commit's evidence
- Added `maxSourceVersion` + the ahead-of-source no-op guard in `runMigrationsOnce`, closing SC #4 with a real-Postgres proof (RED before, GREEN after)
- Pinned the guard's edges: dirty-ahead still errors, behind-by-one still migrates forward, a fresh DB still applies from scratch, equal-version calls stay idempotent
- Added `cmd/migrate`, the `go run` HEAD-schema helper the `n1-boot` CI job (16-04) will invoke, driving the exact `db.RunMigrations` boot path including this plan's guard

## Task Commits

Each task was committed atomically (Task 1 as the plan's mandated 3-commit RED->GREEN sequence):

1. **Task 1, commit 1 (seam):** `8c21be4` refactor(16-01): extract runMigrationsWithSource seam (D-18)
2. **Task 1, commit 2 (RED):** `0f2b164` test(16-01): RED proof — ahead-of-source schema errors, not ErrNoChange (SC #4)
3. **Task 1, commit 3 (GREEN):** `2673dee` feat(16-01): ahead-of-source no-op guard (SC #4 GREEN)
4. **Task 2:** `9cc55e0` test(16-01): pin the guard's edges — dirty-ahead, behind, fresh, idempotent
5. **Task 3:** `1463d75` feat(16-01): cmd/migrate — go run HEAD-schema helper for the n1-boot job

_Note: Task 1 is the plan's mandated 3-commit TDD sequence (seam -> RED -> GREEN); Task 2 adds four more sub-tests against the already-landed guard in one commit; Task 3 is a single feat commit for the new `cmd/migrate` package._

## Files Created/Modified
- `internal/db/migrate.go` - D-18 seam split (`RunMigrations` delegates to unexported `runMigrationsWithSource`); `maxSourceVersion` helper; ahead-of-source no-op guard in `runMigrationsOnce`
- `internal/db/migrate_ahead_test.go` - `package db` in-package test proving SC #4 (RED then GREEN) plus the guard's four edge cases, each on its own scratch schema
- `cmd/migrate/main.go` - thin `main()` -> `run(ctx)` reading `DATABASE_URL`, calling `db.RunMigrations`
- `cmd/migrate/main_test.go` - whitebox tests: missing-DSN error path, real-Postgres apply-from-scratch path asserting against the highest `*.up.sql` on disk
- `Makefile` - `COVER_PKGS` extended to exclude `cmd/migrate` (Phase 15 D-07 precedent)

## Decisions Made
- Confirmed RESEARCH.md Finding 1 live: `golang-migrate/migrate/v4 v4.19.1`'s `Up()` errors (does not `ErrNoChange`) against an ahead-of-source schema — the RED commit body records the exact observed failure text (`no migration found for version 6: read down for version 6 m: file does not exist`) as required evidence.
- `migrate_ahead_test.go` reimplements `RequirePostgresDSN` and `scratchSchemaDSN` locally instead of importing `internal/testutil`: the test file must be `package db` (in-package, to reach the unexported seam), and `internal/testutil` imports `internal/db` — importing it here would be a cycle. No new exported symbols were added to `internal/db` to avoid this; the duplication is confined to the test file.
- MGRT-01 left unmarked in REQUIREMENTS.md: `requirements.ready-ids` reports it `blocked` because sibling plans 16-02/16-03/16-04 also declare MGRT-01 and have not executed. Marking it complete here would be premature — it will be marked once the last plan declaring it lands.

## Deviations from Plan

None - plan executed exactly as written, including the mandated 3-commit RED->GREEN sequence for Task 1 with the observed failure text recorded in the commit-2 body.

## Issues Encountered
- `go test -race` (`make test`) fails to build on this Windows dev box (pre-existing, documented cgo/ThreadSanitizer limitation — see `.planning/WINDOWS.md`). Substituted the established workaround: the identical `go test ./... -coverprofile=coverage.out -coverpkg=$COVER_PKGS` invocation without `-race`, confirmed by prior phases (11.1-04, 15-02) as the accepted local equivalent. `make coverage-gate` was then run against that profile and passed (90.05%, floor 80%).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- 16-02 (`cmd/migration-check` scanner) can now assume the boot path's ahead-of-source behavior is real and proven, not an open research assumption.
- 16-04 (CI wiring) can invoke `go run ./cmd/migrate` exactly as this plan built it — reads `DATABASE_URL` only, no flags, exits non-zero with a message naming the missing var.
- No blockers. MGRT-01 stays open pending 16-02/03/04.

---
*Phase: 16-rollback-safe-migrations*
*Completed: 2026-09-04*

## Self-Check: PASSED

All 5 created/modified files found on disk; all 5 task commit hashes (`8c21be4`, `0f2b164`, `2673dee`, `9cc55e0`, `1463d75`) found in git log.
