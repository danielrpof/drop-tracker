---
phase: 24-artist-tags-notes
plan: 08
subsystem: database
tags: [postgres, plpgsql, trigger, migration, sqlc, gap-closure]

requires:
  - phase: 24-artist-tags-notes
    provides: migration 000010 (tags/artist_tags schema, INSERT-only cap trigger, ADR 0004)
provides:
  - Migration 000011, an additive pair that extends the per-artist tag cap trigger to fire on `BEFORE UPDATE OF artist_id`, closing verification gap 1 (SC2 / TAG-04, review WR-03)
  - Five new DB-level tests proving the raw-UPDATE bypass is refused, every legal UPDATE (merge, unchanged-artist, below-cap move) still works, and the 000011 pair round-trips
  - ADR 0004 amendment documenting the UPDATE coverage
  - 000011 applied to the live dev database
affects: [24-verification, 24-09]

actuals:
  tokens: 4706
  tasks: 3
  commits: 4
plan_head_before: b5f51de65dac2edcf362e5af63566a1dc3958138

tech-stack:
  added: []
  patterns:
    - "Column-scoped trigger (`BEFORE UPDATE OF artist_id`) reusing the same locking/skip-existing function as the INSERT trigger, so one function covers both firing events without duplicating the cap logic"

key-files:
  created:
    - internal/db/migrations/000011_artist_tags_cap_on_update.up.sql
    - internal/db/migrations/000011_artist_tags_cap_on_update.down.sql
  modified:
    - internal/db/tags_schema_test.go
    - internal/db/schema_version_test.go
    - internal/db/migrate_test.go
    - docs/adr/0004-per-artist-tag-cap-trigger.md

key-decisions:
  - "New additive migration 000011 rather than editing 000010, since 000010 is already applied to the dev database and every developer fixture even though it has not shipped in a release tag yet -- golang-migrate never re-runs an applied version."
  - "The UPDATE trigger is column-scoped (`BEFORE UPDATE OF artist_id`), not a bare `BEFORE UPDATE`, so merge's `UPDATE artist_tags SET tag_id` never fires it (D-19 preserved)."
  - "`make` is not installed on this Windows dev box in the current shell session; the Definition of Done gates were run via the exact underlying commands from Makefile (`go vet`, golangci-lint binary, the coverage-report/coverage-gate tool, `sqlc generate` + `git diff`) rather than `make <target>`. sqlc v1.31.1 was reinstalled via `go install` (pinned version, CGO_ENABLED=0) since it was not present in this session's GOPATH/bin."

requirements-completed: [TAG-04]

coverage:
  - id: D1
    description: "A raw UPDATE artist_tags SET artist_id onto a 10-link artist is refused by the database with SQLSTATE 23514 / constraint artist_tags_max_per_artist; the target keeps 10 links and the moved link stays on its original artist"
    requirement: "TAG-04"
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagCapTrigger_RawUpdateRefused"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every legal UPDATE keeps working: merge's UPDATE SET tag_id, an unchanged-artist UPDATE (free), and a below-cap move; a two-row move that would breach the cap is refused as a whole statement, and a duplicate move fails with a primary-key violation rather than a silent success"
    requirement: "TAG-04"
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagCapTrigger_UpdatePaths"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Merge_AtCap"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_MergeEndToEnd"
        status: pass
    human_judgment: false
  - id: D3
    description: "The 000011 up/down pair round-trips: the down file restores the 000010 (INSERT-only) behavior, and the up file re-closes the UPDATE path"
    requirement: "TAG-04"
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_Migration000011_DownUpRoundTrip"
        status: pass
    human_judgment: false
  - id: D4
    description: "000011 is applied to the live dev database (schema_migrations = 11|f, artist_tags_cap_update_trigger present), ADR 0004 documents the UPDATE coverage, and the full backend Definition of Done (vet, lint, integration suite, coverage-gate, sqlc-check, migration-check) is green"
    requirement: "TAG-04"
    verification:
      - kind: other
        ref: "docker compose exec postgres psql -tAc 'SELECT version, dirty FROM schema_migrations' -> 11|f; psql pg_trigger query -> artist_tags_cap_update_trigger"
        status: pass
      - kind: other
        ref: "go vet ./...; golangci-lint run; go test ./... -coverprofile=coverage.out; cmd/coverage-report --mode=total (89.27%); sqlc generate + git diff; go test ./cmd/migration-check/"
        status: pass
    human_judgment: false

duration: ~50min
completed: 2026-09-24
status: complete
---

# Phase 24 Plan 08: Cap on UPDATE OF artist_id Summary

**Additive migration 000011 extends the per-artist tag cap trigger to fire on `BEFORE UPDATE OF artist_id`, closing the raw-UPDATE bypass reproduced in phase verification (gap 1 / SC2 / TAG-04, review WR-03), while merge's `UPDATE ... SET tag_id` stays unaffected (D-19).**

## Performance

- **Duration:** ~50 min
- **Tasks:** 3 completed
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments

- Database now refuses an 11th tag link whether it arrives by INSERT or by an UPDATE that moves an existing link onto an already-full artist, with the same `23514` / `artist_tags_max_per_artist` error the API already maps to a 409.
- Every legal UPDATE keeps working: merge's `UPDATE ... SET tag_id`, an unchanged-artist no-op UPDATE, and a below-cap move all succeed; a cap-breaking two-row move and a duplicate-tag move are both refused without silently succeeding.
- The 000011 up/down pair round-trips (proven by executing both files directly against an isolated Postgres schema) and is now applied to the live dev database.
- ADR 0004 amended to document that the database guarantee now covers both INSERT and UPDATE of `artist_id`.

## Task Commits

Each task was committed atomically, split into RED/GREEN/characterization commits per the tdd="true" task attribute:

1. **Task 1 RED: add failing test for cap on UPDATE of artist_id** - `4e662d2` (test)
2. **Task 1 GREEN: cap artist_tags on UPDATE OF artist_id (migration 000011)** - `d489911` (feat)
3. **Task 2: characterize legal UPDATE paths and the 000011 round-trip** - `684c68f` (test)
4. **Task 3: amend ADR 0004, apply 000011 to live dev DB, run Definition of Done** - `19d5373` (docs)

_TDD tasks produced RED-then-GREEN commit pairs; Task 2's tests characterize behavior already delivered by Task 1's migration, so it landed as a single test-only commit._

**Plan metadata:** committed alongside this SUMMARY (STATE.md/ROADMAP.md update).

## Files Created/Modified

- `internal/db/migrations/000011_artist_tags_cap_on_update.up.sql` - re-creates `check_artist_tags_max_per_artist()` with an early return for unchanged-`artist_id` UPDATEs, and attaches `artist_tags_cap_update_trigger BEFORE UPDATE OF artist_id`
- `internal/db/migrations/000011_artist_tags_cap_on_update.down.sql` - drops the new trigger and restores the 000010 function body byte for byte
- `internal/db/tags_schema_test.go` - `TestSchema_TagCapTrigger_RawUpdateRefused`, `TestSchema_TagCapTrigger_UpdatePaths` (5 subtests), `TestSchema_Migration000011_DownUpRoundTrip`
- `internal/db/schema_version_test.go` - `expectedSchemaVersionOnDisk` bumped 10 -> 11
- `internal/db/migrate_test.go` - from-scratch assertion now expects `(11, false)`
- `docs/adr/0004-per-artist-tag-cap-trigger.md` - added `## Amendment: UPDATE OF artist_id (migration 000011)`

## Decisions Made

- New additive migration (000011) rather than editing the already-applied 000010, per `internal/db/migrations/README.md`'s immutability rule for shipped-to-a-database migrations.
- Column-scoped `BEFORE UPDATE OF artist_id` trigger, not a bare `BEFORE UPDATE`, to keep merge's `UPDATE ... SET tag_id` outside the trigger's firing conditions.
- Reused sequential-executor's own bash session to run Definition of Done gates manually (`go vet`, the pinned golangci-lint binary already cached from pre-commit, the coverage-report/coverage-gate logic from the Makefile, `sqlc generate` + `git diff`) since `make` is not on this session's PATH; installed `sqlc` v1.31.1 fresh via `go install` (CGO_ENABLED=0) since it was missing from this session's `GOPATH/bin`.

## Deviations from Plan

None - plan executed exactly as written. The only adjustment was tooling access (see Decisions Made): `make` and `sqlc` were not present in this bash session, so their exact underlying commands were run directly instead of through the `make` wrapper, with sqlc reinstalled at the pinned v1.31.1 version. No plan content, migration SQL, or test behavior was changed as a result.

## Issues Encountered

None. All acceptance criteria and verification commands passed on the first attempt after implementation; no auto-fixes or repairs were needed.

## User Setup Required

None - no external service configuration required. The migration was applied to the local dev database as part of this plan's Task 3.

## Next Phase Readiness

- SC2 (TAG-04) now fully holds: the database refuses an 11th tag link via both INSERT and UPDATE of `artist_id`, with sqlc output unchanged and every merge/attach regression test green.
- Gap 2 (deleted tag lingering in the "+ tag" autocomplete) remains open and is scoped to plan 24-09, not this plan.
- 24-VERIFICATION.md's human-verification items (browser-only checks) remain open pending gap closure completion.

## Self-Check: PASSED

- All created/modified files confirmed present on disk (`000011_*.sql` pair, `tags_schema_test.go`, ADR 0004, this SUMMARY).
- `git log --oneline --all --grep="24-08"` returns 5 commits (RED, GREEN, characterization, ADR/DoD, plan metadata).
- Re-ran the plan-level `<verification>` block after the final commit: `cmd/migration-check` clean, `TestSchema|TestTrigger_|TestExpectedSchemaVersion|TestRunMigrations` pass, merge/attach regression suites pass, live `schema_migrations` reads `11|f`, and migration 000010 remains byte-identical to `447baa0`.

---
*Phase: 24-artist-tags-notes*
*Completed: 2026-09-24*
