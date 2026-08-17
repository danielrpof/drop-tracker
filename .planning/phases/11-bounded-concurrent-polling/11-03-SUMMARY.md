---
phase: 11-bounded-concurrent-polling
plan: 03
subsystem: database
tags: [postgres, sqlc, pgx, concurrency, cas, race-condition]

# Dependency graph
requires:
  - phase: 11-bounded-concurrent-polling (plan 01)
    provides: bounded concurrent poller fan-out that makes two artists sharing a release group race each other in the same process
provides:
  - "AdvanceGroupTrackCountBaseline: one atomic FOR UPDATE + UPDATE...RETURNING statement replacing the former two-statement baseline read/write"
  - "Detector.advanceGroupBaseline, the Go wrapper detectDeluxeChanges now branches on"
  - "TestAdvanceGroupBaseline_ConcurrentRace, the phase's canonical proof that the lost-update race is closed"
affects: [11-bounded-concurrent-polling (remaining waves), any future phase touching internal/detection or queries/events.sql]

# Actuals (#2632)
actuals:
  tokens: 10355
  tasks: 3
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Locking-CTE atomic compare-and-set: a `WITH x AS (SELECT ... FOR UPDATE) UPDATE ... FROM x ... RETURNING` statement replaces a check-then-act SELECT+UPDATE pair, closing a lost-update race entirely rather than narrowing it (11-RESEARCH.md Pattern 2)."
    - "Whitebox real-Postgres test file for an unexported method, mirroring filter_test.go's `package detection` + reused helper convention, when the blackbox `detection_test` files can't see the method under test."

key-files:
  created:
    - internal/detection/baseline_test.go
  modified:
    - queries/events.sql
    - internal/db/sqlc/events.sql.go
    - internal/db/sqlc/querier.go
    - internal/detection/detector.go
    - internal/detection/musicbrainz.go
    - internal/httpserver/events_test.go

key-decisions:
  - "Combined Task 1 (query replacement) and Task 2 (Go re-derivation) into a single commit instead of the plan's two separate task commits, because this repo's golangci-lint pre-commit hook typechecks the whole module against the staged tree (pre-commit stashes unstaged changes before running), and Task 1 alone leaves internal/detection uncompilable by design until Task 2's callers land."
  - "advanceGroupBaseline's tests live in a new whitebox file (internal/detection/baseline_test.go, package detection) rather than in detector_test.go as the plan's action text literally said, because detector_test.go/deezer_test.go are package detection_test (blackbox) and cannot see an unexported method -- filter_test.go already establishes the whitebox real-Postgres pattern this file follows, and its filterTestMBID/insertFilterTestArtist/noRecordingSource/noReleaseDetailSource helpers were reused rather than duplicated."
  - "Accepted (not closed) the advance-then-insert crash window per 11-RESEARCH.md Pitfall 1's recommended resolution: documented as a Known, accepted edge in detectDeluxeChanges' doc comment, with a Warn log on the reachable (non-crash) failure path, mirroring the notifier's WR-03 precedent. No transaction wiring added."
  - "-race could not be run in this sandbox (ThreadSanitizer fails to allocate memory here for any package, confirmed on an unrelated internal/config test) -- a pre-existing, already-documented Windows dev-box limitation (STATE.md Phase 01-02/01-03/01-04 decisions), not something introduced by this plan. All verification below ran without -race; -race is expected to run clean in CI/WSL2 per the existing project pattern."

requirements-completed: [PERF-04]

coverage:
  - id: D1
    description: "One atomic compare-and-set (AdvanceGroupTrackCountBaseline) replaces the two-statement baseline read/write, with generated bindings committed and drift-free."
    requirement: "PERF-04"
    verification:
      - kind: integration
        ref: "make sqlc-check"
        status: pass
      - kind: integration
        ref: "internal/httpserver/events_test.go#TestRetention_DetectionStateQueriesStayUnfiltered/deluxe_baseline_survives_(criterion_5)"
        status: pass
    human_judgment: false
  - id: D2
    description: "detectDeluxeChanges branches on the single atomic call and produces byte-identical outcomes for all three cases, proven by the full pre-existing deluxe-change suite passing unmodified."
    requirement: "PERF-04"
    verification:
      - kind: integration
        ref: "internal/detection/musicbrainz_test.go and detector_test.go deluxe-change suite (12 named tests, e.g. TestDetectMusicBrainz_DeluxeChange_FiresOnIncrease, _NoEventOnEqualCount, _PerGroupErrorIsolated, TestDetectDeezer_NeverProducesDeluxeChange)"
        status: pass
    human_judgment: false
  - id: D3
    description: "A deliberate race between two callers sharing one release group leaves the correct (higher) baseline stored, proven by asserting final stored state, and shown to fail against a non-atomic implementation."
    requirement: "PERF-04"
    verification:
      - kind: integration
        ref: "internal/detection/baseline_test.go#TestAdvanceGroupBaseline_ConcurrentRace (run at -count=10)"
        status: pass
      - kind: manual_procedural
        ref: "Hand-run falsification: a temporary deterministic non-atomic two-round-trip form reliably stored 10 instead of the true max 12; harness deleted, not committed"
        status: pass
    human_judgment: false
  - id: D4
    description: "The advance-then-insert crash window is an explicit, documented, loudly-logged accepted edge with a recorded backstop truth."
    requirement: "PERF-04"
    verification:
      - kind: unit
        ref: "grep -ci 'known, accepted edge' internal/detection/musicbrainz.go (>=1) and grep -c 'logger.Warn' internal/detection/musicbrainz.go (>=1)"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-17
status: complete
---

# Phase 11 Plan 03: Atomic Deluxe-Change Baseline Compare-and-Set Summary

**Closed PERF-04's lost-update race by replacing `groupBaseline`+`setGroupBaseline`'s check-then-act pair with one `FOR UPDATE`-locked `UPDATE...RETURNING` statement (`AdvanceGroupTrackCountBaseline`), re-derived `detectDeluxeChanges`' branching from its single result, and proved the fix with a deliberate two-caller race asserting on final stored state.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-08-17
- **Tasks:** 3
- **Files modified:** 6 modified, 1 created

## Accomplishments

- `queries/events.sql`: `AdvanceGroupTrackCountBaseline` (`:many`) — a `FOR UPDATE`-locking CTE plus `UPDATE ... FROM ... RETURNING` — replaces the former `GroupTrackCountBaseline` (`:one`) SELECT and `SetGroupTrackCountBaseline` (`:execrows`) UPDATE. Keyed on `external_id` (a deliberate narrowing from the removed read query's `release_group_mbid`, since for a musicbrainz `new_release` row the two columns hold the same value).
- Regenerated `internal/db/sqlc/events.sql.go`/`querier.go` (sqlc v1.31.1) — the two superseded methods are gone; the new one returns `[]*int32` (sqlc's natural shape for a single-column `RETURNING`), not a named-field row struct as the research snippet sketched.
- `Detector.advanceGroupBaseline(ctx, groupMBID, count) (advanced, hadBaseline bool, previousBaseline int, err error)` replaces `groupBaseline`+`setGroupBaseline`, adapted to the actual generated slice-of-pointer shape.
- `detectDeluxeChanges` re-derives its three-way branching (no-op / establish / fire event) from one call instead of a read then two possible writes; every pre-existing deluxe-change test (12 named tests across `detector_test.go` and `deezer_test.go`) passes unmodified, proving no observable outcome changed.
- `TestAdvanceGroupBaseline_ConcurrentRace` (two racing counts, an eight-way race, and an identical-count race) and `TestAdvanceGroupBaseline_SingleCallerContract` (five single-caller cases) — the phase's canonical proof for ROADMAP criterion 4, in a new whitebox file since the method under test is unexported.
- Phase 10's `TestRetention_DetectionStateQueriesStayUnfiltered` criterion-5 subtest repointed at the replacement query with a strictly stronger single-row assertion.
- The advance-then-insert ordering reversal (11-RESEARCH.md Pitfall 1) is documented as a `Known, accepted edge` in `detectDeluxeChanges`' doc comment, with a `logger.Warn` on the reachable (non-crash) insert-failure path, mirroring the notifier's WR-03 precedent.

## Task Commits

1. **Tasks 1+2 (combined — see Deviations): Replace baseline read/write with one atomic CAS, re-derive branching** - `9930b4f` (feat)
2. **Task 3: Race the baseline and assert final state** - `8ff0d65` (test)

_Note: Tasks 1 and 2 were combined into a single commit; see Deviations below for why._

## Files Created/Modified

- `queries/events.sql` - `AdvanceGroupTrackCountBaseline` replaces the two superseded queries; Phase 10's unfiltered-query enumeration comment updated to name the replacement
- `internal/db/sqlc/events.sql.go`, `internal/db/sqlc/querier.go` - regenerated bindings (sqlc generate, drift-free per `make sqlc-check`)
- `internal/detection/detector.go` - `advanceGroupBaseline` replaces `groupBaseline`/`setGroupBaseline`
- `internal/detection/musicbrainz.go` - `detectDeluxeChanges` branching re-derived; accepted-edge doc paragraph and Warn log added
- `internal/httpserver/events_test.go` - criterion-5 subtest repointed at `AdvanceGroupTrackCountBaseline`, keyed on `external_id`, asserting a stronger single-row result
- `internal/detection/baseline_test.go` (new) - `TestAdvanceGroupBaseline_ConcurrentRace` and `TestAdvanceGroupBaseline_SingleCallerContract`, plus local helpers (`int32Ptr`, `insertBaselineNewRelease`, `readBaselineTrackCount`)

## Decisions Made

- **Combined Task 1 + Task 2 into one commit.** The plan's acceptance criteria explicitly designed Task 1 to leave `go build ./...` failing until Task 2's callers land ("the two removed `Detector` methods are the only remaining callers, which is the intended, visible coupling"). This repo's `.pre-commit-config.yaml` runs `golangci-lint` locally before every commit, and `pre-commit` stashes unstaged changes before running hooks (confirmed empirically: attempting the Task-1-only commit with Task 2's edits already in the working tree still failed, with `pre-commit` logging `[INFO] Stashing unstaged files...`). A Task-1-only commit is therefore unbuildable at the moment the hook runs, regardless of what's already on disk. Both tasks landed in one `feat` commit instead, with the combination itself documented in the commit body.
- **New whitebox test file instead of extending `detector_test.go`.** The plan's Task 3 action text says these tests belong in "the internal test package alongside the file's existing whitebox tests," naming `detector_test.go` as the target file — but `detector_test.go` (and `deezer_test.go`) are `package detection_test` (blackbox), which cannot call the unexported `advanceGroupBaseline`. `filter_test.go` already is `package detection` (whitebox) with its own real-Postgres helpers for exactly this situation (testing an unexported function/method against real Postgres). `internal/detection/baseline_test.go` was created as a sibling whitebox file, reusing `filter_test.go`'s `filterTestMBID`/`insertFilterTestArtist`/`noRecordingSource`/`noReleaseDetailSource` rather than duplicating them.
- **`-race` verification ran without `-race`.** This Windows dev-box sandbox's ThreadSanitizer fails to allocate memory for any package (confirmed on an unrelated, untouched `internal/config` test, not just this plan's changes) — a pre-existing, already-documented limitation of this specific environment (see STATE.md's Phase 01-02/01-03/01-04 decisions on the same machine's cgo/-race toolchain gap). All verification in this plan ran with plain `go test`; the plan's `-race` requirement is expected to pass in CI/WSL2 per the existing project pattern, and no code in this plan introduces a Go-level data race (the concurrency-safety mechanism is Postgres's own row lock, not a Go primitive).
- **Falsification performed by hand, not left to a temporary code revert.** Rather than temporarily reverting `queries/events.sql`/`detector.go` to the old two-query form (which would require regenerating and re-reverting sqlc output mid-task), a self-contained temporary test file (`falsify_temp_test.go`, never committed) reproduced the pre-Task-1 two-round-trip check-then-act with explicit synchronization (both callers read the stale baseline, the higher-count write commits first, then the lower-count write commits second) and confirmed it reliably clobbers the correct value (stored 10 instead of the true max 12). An earlier, unsynchronized probabilistic version of this check (racing two goroutines over 20 attempts with a short sleep) never failed on this machine — Go's runtime/pgx connection-acquisition ordering happened to consistently favor the correct outcome by accident, which is itself worth recording: a probabilistic falsification check on this specific toolchain is not reliable, but a deterministic one is.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Combined Task 1 and Task 2 commits**
- **Found during:** Attempting to commit Task 1 alone
- **Issue:** The plan designs Task 1 to leave `go build ./...` broken until Task 2 lands, but this repo's `golangci-lint` pre-commit hook typechecks the whole module against the staged tree (with unstaged changes stashed away), so a Task-1-only commit can never pass the hook
- **Fix:** Implemented Task 2's Go changes before committing, then staged and committed Task 1's and Task 2's files together in a single `feat` commit
- **Files modified:** queries/events.sql, internal/db/sqlc/events.sql.go, internal/db/sqlc/querier.go, internal/httpserver/events_test.go, internal/detection/detector.go, internal/detection/musicbrainz.go
- **Verification:** `go build ./... && go vet ./...` clean at the combined commit; `golangci-lint` pre-commit hook passed
- **Committed in:** `9930b4f`

**2. [Rule 3 - Blocking] New whitebox test file instead of adding to `detector_test.go`**
- **Found during:** Task 3
- **Issue:** The plan's action text says the new race/single-caller tests belong in `detector_test.go` as "the internal test package," but `detector_test.go` is `package detection_test` (blackbox) and cannot call the unexported `advanceGroupBaseline`
- **Fix:** Created `internal/detection/baseline_test.go` as `package detection` (whitebox), reusing `filter_test.go`'s existing whitebox real-Postgres helpers rather than duplicating them
- **Files modified:** internal/detection/baseline_test.go (new)
- **Verification:** `go build ./... && go vet ./...` clean; both new test functions compile and pass
- **Committed in:** `8ff0d65`

---

**Total deviations:** 2 auto-fixed (both Rule 3 - blocking issues in the plan's design/text vs. this repo's actual tooling/package layout)
**Impact on plan:** Neither changed the delivered behavior or test coverage described in the plan — both were necessary adaptations to make the plan's own acceptance criteria achievable given this repo's real pre-commit hook and package structure. No scope creep.

## Issues Encountered

- Docker port 5433 (the project's `TEST_DATABASE_URL` default) was already bound by a sibling worktree agent's own postgres container from this same wave. Rather than starting a duplicate, this plan's verification ran directly against that already-running, already-migrated shared instance (confirmed via `docker exec ... psql -c '\dt'` that all four expected tables existed) — the same pattern the project's own `Makefile` comment anticipates for shared-DSN use across concurrent test runs.
- An initial unsynchronized falsification attempt (racing two goroutines with a fixed sleep, no explicit ordering, over 20 attempts) never reproduced the lost update on this machine — recorded above under Decisions Made, and resolved with a deterministically-sequenced version instead.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- PERF-04 is satisfied: the deluxe-change baseline compare-and-set is atomic at the database level, proven by a falsifiable race test.
- `internal/detection`'s public surface (`Detector.DetectMusicBrainz`) is unchanged; downstream callers (the poller's concurrent fan-out from plan 11-01) need no changes.
- No blockers for the remaining Phase 11 waves.

---
*Phase: 11-bounded-concurrent-polling*
*Completed: 2026-08-17*
