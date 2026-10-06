---
phase: 24-artist-tags-notes
plan: 10
subsystem: database
tags: [postgres, trigger, plpgsql, concurrency, migration, gap-closure, adr]
status: complete

requires:
  - phase: 24-artist-tags-notes
    provides: migrations 000010 (cap trigger) and 000011 (cap on UPDATE OF artist_id), ADR 0004
provides:
  - "Migration 000012: check_artist_tags_max_per_artist() locks the existing link FOR KEY SHARE in both skip-existing checks, so an uncommitted concurrent detach can no longer let a re-attach or move skip the artist lock and the count (CR-01, SC2 / TAG-04 under concurrency)"
  - Forced, pg_stat_activity-gated three-session DB tests for the INSERT re-attach and UPDATE move variants, plus the distinct-tag 10th/11th race (IN-09)
  - A 000012 down/up round-trip test that shows the hole open on the 000011 body and closed on 000012
  - A WR-07 `unchanged-artist` subtest that now fails without the TG_OP guard
  - ADR 0004 amendment for the concurrent-detach case
  - 000012 applied to the live dev database (12|f)
affects: [24-verification]

actuals:
  tokens: 4235
  tasks: 3
  commits: 3
plan_head_before: 446d54831d39782fd1ac09d13d736a117775ca5e

tech-stack:
  added: []
  patterns:
    - "Row-locked existence proof (PERFORM ... FOR KEY SHARE; IF FOUND) instead of a visibility-only IF EXISTS inside a locking trigger"
    - "Forced interleaving tests: observe the blocked backend via pg_stat_activity wait_event_type = 'Lock' before releasing the holder, with every early exit releasing/rolling back and waiting on goroutines"

key-files:
  created:
    - internal/db/migrations/000012_artist_tags_cap_concurrent_detach.up.sql
    - internal/db/migrations/000012_artist_tags_cap_concurrent_detach.down.sql
  modified:
    - internal/db/tags_schema_test.go
    - internal/db/schema_version_test.go
    - internal/db/migrate_test.go
    - docs/adr/0004-per-artist-tag-cap-trigger.md

key-decisions:
  - "Ship the fix as additive 000012 rather than editing 000011: the dev DB and fixtures already applied version 11 and golang-migrate never re-runs an applied version. 000010/000011 are byte-identical to 77de93d."
  - "The race helper (runDetachRace) was written with the update-move mode from the start (Task 1) and the update-move subtest added in Task 2, so Task 1 stayed a minimal tracer on the INSERT path."

requirements-completed: [TAG-04]

coverage:
  - id: D1
    description: "A third attach racing a concurrent uncommitted detach plus re-attach (INSERT ... ON CONFLICT DO NOTHING) of the same link is refused with 23514 / artist_tags_max_per_artist; the artist ends at exactly 10 with T present and U absent"
    requirement: "TAG-04"
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagCapTrigger_ConcurrentDetachRace/insert_re-attach"
        status: pass
    human_judgment: false
  - id: D2
    description: "Same interleaving with S2 moving the link from another artist via UPDATE ... SET artist_id is refused at 10; the source artist ends with no links"
    requirement: "TAG-04"
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagCapTrigger_ConcurrentDetachRace/update_move"
        status: pass
    human_judgment: false
  - id: D3
    description: "Two concurrent attaches of different new tags at 9 links: first succeeds, second refused, artist ends at 10"
    requirement: "TAG-04"
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagCapTrigger_DistinctTagConcurrentAt9"
        status: pass
    human_judgment: false
  - id: D4
    description: "The 000012 pair round-trips behaviorally: after down the race reaches 11 links (S3 returns early, no error); after up it is refused at 10"
    requirement: "TAG-04"
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_Migration000012_DownUpRoundTrip"
        status: pass
    human_judgment: false
  - id: D5
    description: "The unchanged-artist subtest pins the TG_OP = 'UPDATE' early return: a single-row UPDATE changing tag_id on a 10-link artist succeeds, and fails with artist_tags_max_per_artist when the guard is removed"
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagCapTrigger_UpdatePaths/unchanged-artist"
        status: pass
    human_judgment: false
  - id: D6
    description: "Earlier guarantees still hold and the backend Definition of Done is green: go vet, golangci-lint, full suite, coverage 89.27% (floor 80), sqlc generate with no diff, cmd/migration-check scan clean, live dev DB at 12|f with FOR KEY SHARE in the public function"
    verification:
      - kind: other
        ref: "go vet ./...; golangci-lint run; go test ./... -count=1 -coverprofile=coverage.out -coverpkg=...; cmd/coverage-report --mode=total (89.27); sqlc generate + git diff --exit-code internal/db/sqlc/; go test ./cmd/migration-check/; psql schema_migrations + pg_proc"
        status: pass
    human_judgment: false
---

# Phase 24 Plan 10: Cap trigger vs concurrent detach (migration 000012) Summary

**Migration 000012 re-creates the shared cap trigger function so both skip-existing checks lock the existing link `FOR KEY SHARE`, closing the CR-01 hole where a concurrent uncommitted detach plus a re-attach or move let a third attach land an 11th tag link.**

## What was done

- **Task 1 (tracer), `c204785`:** Wrote `runDetachRace` and `TestSchema_TagCapTrigger_ConcurrentDetachRace/insert re-attach` first (RED), then added 000012 up/down and bumped the version pins to 12.
- **Task 2, `b063ecc`:** Added the `update move` subtest, `TestSchema_TagCapTrigger_DistinctTagConcurrentAt9`, `TestSchema_Migration000012_DownUpRoundTrip`, and rewrote `unchanged-artist` (WR-07) to also change `tag_id` and assert `RowsAffected() == 1`, count 10, fresh tag linked, old tag unlinked.
- **Task 3, `126e506`:** ADR 0004 amendment `## Amendment: concurrent detach (migration 000012)`; applied 000012 to the live dev DB; ran the Definition of Done.

## Fail-first evidence

Before 000012 (000011 body), `TestSchema_TagCapTrigger_ConcurrentDetachRace/insert_re-attach`:

```
tags_schema_test.go:474: S3 attached while S2 skipped the artist lock (CR-01): s3Err=<nil> finalCount=11
--- FAIL: TestSchema_TagCapTrigger_ConcurrentDetachRace/insert_re-attach
```

After 000012 it passes, and `TestSchema_Migration000012_DownUpRoundTrip` pins both directions permanently (down reaches 11, up refused at 10).

WR-07 mutation check (guard line `IF TG_OP = 'UPDATE' AND NEW.artist_id = OLD.artist_id` temporarily removed from the 000012 up file, not committed):

```
tags_schema_test.go:793: unchanged-artist UPDATE: ERROR: artist already has the maximum of 10 tags (SQLSTATE 23514)
--- FAIL: TestSchema_TagCapTrigger_UpdatePaths/unchanged-artist
```

The file was restored with `git checkout`, and `git diff --exit-code -- internal/db/migrations/` exited 0 before staging.

## Deviations from Plan

### Auto-fixed Issues

None - plan executed exactly as written, with these environment adaptations (not deviations in behavior):

- `make` is not on this box's PATH (same as 24-08 and 24-09). The Makefile targets' underlying commands were run directly: `docker compose up -d --wait postgres` for `db-up`, `sqlc generate` plus `git diff --exit-code -- internal/db/sqlc/` for `sqlc-check`, and `go test ./... -coverprofile ... -coverpkg ...` plus `cmd/coverage-report --mode=total` and the awk-free comparison (89.27 >= 80) for `test`/`coverage-gate`.
- The suite ran without `-race` (unusable on this Windows box per STATE.md; CI's Linux job is the authoritative race gate).
- `sqlc` and `golangci-lint` live in `~/go/bin`, which was added to PATH for the session.

## Verification

- `go run ./cmd/migration-check --mode=scan` on 000012 up: no findings.
- 000012 down function block is byte-identical to 000011 up's (sed extraction compare).
- `git diff --exit-code 77de93d` over the four 000010/000011 files: clean.
- Full suite passed; backend coverage 89.27% (floor 80).
- `sqlc generate` produced no diff under `internal/db/sqlc/`.
- Live dev DB: `schema_migrations` = `12|f`; exactly one `public.check_artist_tags_max_per_artist` with `FOR KEY SHARE` in its source.
- `git diff --name-only main -- go.mod go.sum internal/notifier internal/detection internal/discord internal/musicbrainz internal/deezer` and `git diff --name-only 77de93d -- web internal/webassets` both print nothing.

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path, or trust-boundary schema change; the function keeps its signature and error contract.

## Self-Check: PASSED

- FOUND: internal/db/migrations/000012_artist_tags_cap_concurrent_detach.up.sql
- FOUND: internal/db/migrations/000012_artist_tags_cap_concurrent_detach.down.sql
- FOUND commits: c204785, b063ecc, 126e506
