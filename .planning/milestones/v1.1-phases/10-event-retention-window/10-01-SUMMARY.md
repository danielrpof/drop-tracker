---
phase: 10-event-retention-window
plan: "01"
subsystem: events-retention
tags: [config, sql, retention, http-api, testing]
status: complete
dependency-graph:
  requires:
    - internal/config/config.go (Config struct, Load())
    - internal/events/service.go (Service, NewService, List)
    - queries/events.sql (ListEvents)
    - internal/db/sqlc (generated ListEvents/ListExternalIDs/HasAnyEvent/GroupTrackCountBaseline/ListUnnotified)
  provides:
    - Config.EventRetentionDays (env EVENT_RETENTION_DAYS, default 90, fail-fast on non-positive)
    - events.NewService(q, retentionDays) widened constructor
    - ListEvents' created_at >= cutoff retention predicate
    - insertTestEventAt test helper
  affects:
    - cmd/server/main.go (eventsStore wiring)
    - internal/httpserver (GET /events response no longer includes aged-out rows)
tech-stack:
  added: []
  patterns:
    - Retention cutoff computed in the Go domain service (events.Service.List), never at the HTTP boundary and never as a SQL-side now() expression -- matches the existing PageSize-clamp precedent
    - pgtype.Timestamptz built with Valid explicitly set to true (seedNotifiedAt precedent) to avoid a zero-value struct silently marshaling as SQL NULL
    - SQL-predicate boundary tests query sqlc.Queries directly with a fixed, explicit cutoff parameter instead of racing a non-injectable time.Now() inside the service under test
key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - .env.example
    - queries/events.sql
    - internal/db/sqlc/events.sql.go
    - internal/db/sqlc/querier.go
    - internal/events/service.go
    - cmd/server/main.go
    - internal/httpserver/boot_e2e_test.go
    - internal/httpserver/events_test.go
decisions:
  - "ListEventsParams.Cutoff generated as pgtype.Timestamptz, confirming RESEARCH.md assumption A1 -- no deviation needed at that call site."
  - "Final EVENT_RETENTION_DAYS validation error text: \"EVENT_RETENTION_DAYS must be a positive integer, got %d\" (manual post-env.Parse check in config.Load, per D-03)."
  - "TestListEvents_RetentionBoundaryIsInclusive queries sqlc.Queries.ListEvents directly with a fixed, explicit Cutoff instead of going through events.Service.List with a wall-clock-derived window -- Service.List's internal time.Now() is not injectable, so any margin wide enough to avoid a clock race was also wide enough to satisfy a strict '>' as well as '>=', making a Service-level version unable to actually catch a >= -> > regression. Verified: the rewritten test fails when the SQL predicate is temporarily flipped to '>', confirming it pins the correct boundary."
metrics:
  duration: ~35m
  completed: 2026-08-13
actuals:
  tokens: 7264
  tasks: 3
  commits: 3
---

# Phase 10 Plan 01: Event Retention Window Summary

Wired an operator-configurable `EVENT_RETENTION_DAYS` (default 90) end-to-end so `GET /events` hides events older than the window while every detection-state query (dedup keys, seed-mode check, deluxe baseline, pending-notification queue) keeps seeing the full, unfiltered `events` table -- proven by automated tests, not just documented as a design intent.

## What Was Built

**Task 1 (tracer):** Added `Config.EventRetentionDays` (env `EVENT_RETENTION_DAYS`, default 90, plain `int` per D-02) with a manual post-`env.Parse` fail-fast check rejecting `<= 0` (D-03). Added the matching `.env.example` line. Extended `queries/events.sql`'s `ListEvents` with a single new `AND created_at >= sqlc.arg('cutoff')::timestamptz` predicate (required `sqlc.arg`, never `sqlc.narg`), leaving `ListExternalIDs`, `HasAnyEvent`, `GroupTrackCountBaseline`, and `ListUnnotified` byte-identical. Regenerated sqlc output (`ListEventsParams.Cutoff` came out as `pgtype.Timestamptz`, confirming RESEARCH.md's A1 assumption). Widened `events.Service`/`NewService` to accept `retentionDays int` and compute the cutoff (`time.Now()` minus N days, `pgtype.Timestamptz{Valid: true}` explicit) inside `List`, mirroring the existing PageSize-clamp "clamped here, not at the HTTP boundary" convention. Updated all six `events.NewService(` call sites (`cmd/server/main.go`, `boot_e2e_test.go`, four in `events_test.go`) plus one new call site the tracer test itself introduces. Added `insertTestEventAt` (explicit `created_at`) and `TestListEvents_RetentionExcludesAgedOutRows`, which seeds a 120-day-old and a 1-day-old event, asserts `GET /events` returns only the recent one, and asserts both rows are still physically present in the table via a direct count query.

**Task 2:** Added `TestLoad_EventRetentionDaysDefaultsTo90`, `..._Override`, and `..._RejectsNonPositive` (table over `0`/`-1`/`-90`) to `internal/config/config_test.go`. Added `TestListEvents_RetentionBoundaryIsInclusive` and `TestListEvents_RetentionPagesNeverRepeatAnID` to `events_test.go`. The boundary test required a design change from the plan's literal instruction (see Deviations) to actually pin D-04's `>=` semantics reliably.

**Task 3:** Added `TestRetention_DetectionStateQueriesStayUnfiltered`, the phase's single most load-bearing test: seeds one 200-day-old `musicbrainz`/`new_release` event with a populated `release_group_mbid`, non-zero `track_count`, and `notified_at` left NULL, then asserts via five named subtests that `ListExternalIDs`, `HasAnyEvent`, `GroupTrackCountBaseline`, and `ListUnnotified` all still see it (roadmap success criteria 3-5), paired with a contrast subtest proving the same row IS excluded from `Service.List`'s retention-filtered feed -- without that contrast, the first four assertions would also pass on a build where the filter was never wired in at all.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Design correction] `TestListEvents_RetentionBoundaryIsInclusive` rewritten to test the SQL predicate directly, not through `events.Service.List`**
- **Found during:** Task 2
- **Issue:** The plan instructed inserting a boundary-row via `events.Service.List` with a wall-clock-derived timestamp ("a small safety margin toward the visible side"). `Service.List` always recomputes `time.Now()` internally (not injectable), so a boundary row timed relative to the *test's* clock races the *service's own*, slightly later `time.Now()` read. Any margin wide enough to reliably avoid that race (verified with a 2-minute margin) turned out to also be wide enough to satisfy a strict `>` just as well as `>=` -- the test passed identically whether the SQL predicate was `>=` or `>`, meaning it could not actually catch a D-04 regression despite appearing to test it.
- **Fix:** Rewrote the test to call `sqlc.Queries.ListEvents` directly with a fixed, explicit `Cutoff` parameter equal to one row's `created_at`, and a second row one minute earlier. This removes the wall-clock race entirely and tests exactly what D-04 locks: the SQL predicate's own boundary inclusivity.
- **Verification:** Confirmed by temporarily flipping the SQL predicate to `>` and regenerating -- the margin-based version continued to pass (proving it was not load-bearing); the rewritten version failed as expected. Reverted after confirming, `>=` is what's committed.
- **Files modified:** `internal/httpserver/events_test.go`
- **Commit:** afa6f4a

**2. [Documentation-only] Task 1's acceptance criterion "6 matches" for `events.NewService(` undercounts by one**
- **Found during:** Task 1
- **Issue:** The plan's own action text for Task 1 instructs building a *new*, seventh `events.NewService(` call site inside the new tracer test (`TestListEvents_RetentionExcludesAgedOutRows`), on top of the six pre-existing call sites RESEARCH.md's corrections section names. The acceptance criterion's literal grep count of 6 does not account for this new call site the same task explicitly adds.
- **Resolution:** Followed the explicit instruction to build a real `events.Service` in the new tracer test (necessary for a genuine end-to-end proof). Actual count after Task 1 is 7 (`main.go` x1, `boot_e2e_test.go` x1, `events_test.go` x5). No functional impact -- purely a stale acceptance-criteria number in the plan text.
- **Files modified:** none (no code change, just noting the count discrepancy)
- **Commit:** n/a (documentation note only)

No other deviations. Every other acceptance criterion in the plan (grep checks for `sqlc.arg('cutoff')`/absence of `sqlc.narg('cutoff')`, the single `created_at >= ` occurrence, `queries/events.sql` diff confined to `ListEvents`, `make sqlc-check` exit 0, `go vet`/`golangci-lint` clean) was verified exactly as written.

## Verification Results

- `go build ./...`: clean
- `go vet ./...`: clean
- `golangci-lint run` (scoped to touched packages -- a separate worktree's stale cached path polluted an unscoped repo-wide run's gosec output, unrelated to this plan's code): 0 issues
- `make sqlc-check`: exits 0 (generated `internal/db/sqlc/` output matches `queries/events.sql`)
- `go test ./internal/config/... -run TestEnvExampleCompleteness -short -count=1`: PASS
- `go test ./internal/httpserver/... -run TestListEvents_RetentionExcludesAgedOutRows -count=1 -p 1`: PASS
- `go test ./internal/config/... -run "TestLoad_EventRetentionDays" -short -count=1 -v`: PASS (default, override, and all three non-positive subtests)
- `go test ./internal/httpserver/... -run "TestListEvents_Retention" -count=1 -p 1 -v`: PASS (exclusion, boundary, pagination)
- `go test ./internal/httpserver/... -run TestRetention_DetectionStateQueriesStayUnfiltered -count=1 -p 1 -v`: PASS (all 5 subtests)
- `go test ./internal/config/... -run TestLoad_AggregatesAllMissing -short -count=1`: still PASS (new validation did not displace the aggregate-error path)
- Boundary-pin checks: flipping `<= 0` to `< 0` in `config.go` makes the `0` subtest fail (reverted); flipping `>=` to `>` in `ListEvents` makes `TestListEvents_RetentionBoundaryIsInclusive` fail (reverted); adding a retention predicate to `HasAnyEvent` makes the seed-mode subtest of `TestRetention_DetectionStateQueriesStayUnfiltered` fail (reverted)
- `make test-integration` (`-race`): fails to run in this environment -- ThreadSanitizer cannot allocate memory on this Windows dev box's cgo toolchain (pre-existing, documented in STATE.md for Phase 01-02/01-03, unrelated to this plan). Re-ran the identical suite without `-race`: all packages pass, including `internal/events` and `internal/httpserver`.
- `git diff queries/events.sql` (full plan span): changes confined entirely to the `ListEvents` statement and its comment block; the other four statements are byte-identical

## Key Findings for Downstream Plans

- **`ListEventsParams.Cutoff` type:** `pgtype.Timestamptz` (confirms RESEARCH.md assumption A1 exactly -- no adaptation needed).
- **`EVENT_RETENTION_DAYS` validation error text:** `"EVENT_RETENTION_DAYS must be a positive integer, got %d"` (e.g. `"EVENT_RETENTION_DAYS must be a positive integer, got 0"`), returned directly from `config.Load()`, not wrapped by `env.Parse`'s aggregate error.

## Self-Check: PASSED

- FOUND: internal/config/config.go (EventRetentionDays field + validation)
- FOUND: internal/config/config_test.go (three new retention test funcs)
- FOUND: .env.example (EVENT_RETENTION_DAYS=90 line)
- FOUND: queries/events.sql (cutoff predicate on ListEvents only)
- FOUND: internal/db/sqlc/events.sql.go (ListEventsParams.Cutoff pgtype.Timestamptz)
- FOUND: internal/db/sqlc/querier.go (updated ListEvents doc comment)
- FOUND: internal/events/service.go (retentionDays field, widened NewService, cutoff computation)
- FOUND: cmd/server/main.go (eventsStore wired with cfg.EventRetentionDays)
- FOUND: internal/httpserver/boot_e2e_test.go (updated call site)
- FOUND: internal/httpserver/events_test.go (insertTestEventAt + 5 new test funcs)
- FOUND commit 64ee861 (Task 1)
- FOUND commit afa6f4a (Task 2)
- FOUND commit 5070174 (Task 3)
