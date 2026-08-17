---
phase: 11-bounded-concurrent-polling
plan: 04
subsystem: testing
tags: [go, postgres, golang-migrate, pgx, test-isolation, flaky-tests]

# Dependency graph
requires:
  - phase: 11-bounded-concurrent-polling
    provides: "11-01's bounded MusicBrainz fan-out and worker-count config, landed on this branch as the prerequisite for a meaningful suite-wide -race stability proof"
provides:
  - "internal/db/migrate_test.go: TestRunMigrations_AppliesFromScratch runs against a dedicated migrate_scratch schema instead of destructively resetting the shared fixture's public schema"
  - "internal/notifier's spacingWait seam (notifier.go, export_test.go): deterministic requested-duration spacing assertions replacing wall-clock elapsed-time measurement"
  - "internal/testutil.NewIsolatedTestPool: schema-isolated test pool, used by every internal/notifier test, preventing cross-package events-table pollution under concurrent package execution"
  - "Makefile test-integration runs at default package-level test parallelism (no -p 1), verified stable across 5 separate full-suite runs"
affects: [11-VALIDATION, future-ci-race-run]

# Actuals (#2632)
actuals:
  tokens: 8992
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "search_path DSN query-parameter schema isolation for Postgres integration tests (net/url-derived), first established in internal/db/migrate_test.go and generalised into testutil.NewIsolatedTestPool"
    - "Package-level swappable seam vars (spacingWait) for deterministic time-based test assertions, mirroring the existing dbOpTimeout precedent"

key-files:
  created:
    - internal/notifier/export_test.go
  modified:
    - internal/db/migrate_test.go
    - internal/notifier/notifier.go
    - internal/notifier/notifier_test.go
    - internal/testutil/postgres.go
    - Makefile
    - .planning/todos/completed/2026-08-11-fix-flaky-tests-under-parallel-go-test.md

key-decisions:
  - "Extended the search_path schema-isolation technique beyond the destructive migrate-from-scratch test to all of internal/notifier's DB-backed tests via a new testutil.NewIsolatedTestPool, after empirically discovering a third, previously-undiagnosed root cause of suite flakiness under default parallelism (see Deviations)."
  - "-race is unavailable in this session's Windows environment (ThreadSanitizer allocation failure, the same pre-existing cgo-toolchain limitation documented in Phase 01 and 11-01); every -race verification in this plan's tasks and acceptance criteria was substituted with repeated non-race invocations at the same default parallelism."

requirements-completed: [PERF-04]

coverage:
  - id: D1
    description: "The migrate-from-scratch test proves its own isolation with to_regclass assertions, and no longer destroys the shared fixture's default schema"
    requirement: "PERF-04"
    verification:
      - kind: unit
        ref: "internal/db/migrate_test.go#TestRunMigrations_AppliesFromScratch"
        status: pass
      - kind: integration
        ref: "go test ./internal/db/... -count=2 (two repetitions, no manual reset between)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The notifier's inter-send spacing assertions are deterministic (requested-duration, not elapsed wall-clock time), with WR-01's after-a-failed-send contract preserved"
    requirement: "PERF-04"
    verification:
      - kind: unit
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_BatchSpacingBetweenSends"
        status: pass
      - kind: unit
        ref: "internal/notifier/notifier_test.go#TestNotifyPending_SpacingAppliedEvenAfterFailedSend"
        status: pass
    human_judgment: false
  - id: D3
    description: "The full suite passes five separate consecutive runs at default package-level test parallelism, with the Makefile's -p1 workaround removed"
    requirement: "PERF-04"
    verification:
      - kind: integration
        ref: "5x go test ./... -count=1 (non-race substitute, see Issues Encountered), all green"
        status: pass
    human_judgment: true
    rationale: "The plan's own verification specifies -race; -race could not be run in this environment. A CI run with a real -race binary is the recommended follow-up and is called out explicitly as a deferred check."

# Metrics
duration: ~150min
completed: 2026-08-17
status: complete
---

# Phase 11 Plan 04: Suite-Stability Fixes for the Flaky-Test Todo Summary

**Schema-isolated the destructive migrate-from-scratch test, made the notifier's spacing assertions deterministic via a swappable seam, and — after discovering a third, previously-undiagnosed root cause (cross-package events-table pollution under concurrent test execution) — extended schema isolation to all of internal/notifier's tests, closing the folded-in flaky-test todo and removing the Makefile's `-p 1` parallelism workaround.**

## Performance

- **Duration:** ~150 min
- **Started:** 2026-08-17T03:00:00Z (approx.)
- **Completed:** 2026-08-17T06:05:00Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- `TestRunMigrations_AppliesFromScratch` no longer runs `DROP SCHEMA public CASCADE` against the shared fixture; it now proves apply-from-scratch inside a dedicated `migrate_scratch` schema, reached via a `search_path` DSN query parameter, with two `to_regclass` isolation assertions proving both that the migration landed in scratch and that the shared `public` schema was undisturbed
- `internal/notifier`'s inter-send spacing wait is now routed through an unexported `spacingWait` seam (mirroring the existing `dbOpTimeout` precedent), swappable only from the test binary via `export_test.go`'s single exported setter; the two genuinely wall-clock-sensitive tests now assert exact requested spacing durations instead of measuring real elapsed time
- Discovered and fixed a third root cause of suite flakiness under default parallelism that neither this plan's own research nor the original todo diagnosed: `NotifyPending`'s `ListUnnotified` query is deliberately global and unfiltered, so concurrently-running `internal/detection` tests' leftover unnotified rows were leaking into `internal/notifier`'s exact-count assertions, and a real `NotifyPending` call could even mark a different package's row notified out from under its own test (directly reproducing `internal/httpserver`'s `TestRetention_DetectionStateQueriesStayUnfiltered` failure with zero changes to that package). Fixed via a new `testutil.NewIsolatedTestPool`, applied to all 11 DB-backed tests in `internal/notifier/notifier_test.go`
- Five separate consecutive full-suite runs at default package-level parallelism came back green (non-`-race`, `-race` unavailable in this environment — see Issues Encountered); the Makefile's `-p 1` workaround (added by Phase 9's coverage-gate work, not by the flaky-test todo itself) has been removed
- The folded-in todo is closed at `.planning/todos/completed/2026-08-11-fix-flaky-tests-under-parallel-go-test.md`, with a `## Resolution` section recording all three confirmed root causes, the corrected per-test classification, and the five-run stability result

## Task Commits

Each task was committed atomically:

1. **Task 1: Isolate the destructive migrate-from-scratch test onto its own schema** - `a0cb857` (test)
2. **Task 2: Make the notifier's inter-send spacing assertions deterministic** - `9f8ef83` (test)
3. **Task 3: Prove suite stability under default parallelism and close the todo** - `431fe4b` (fix)

_Note: Task 2's commit also includes `internal/testutil/postgres.go` (new `NewIsolatedTestPool` helper) — an in-scope addition needed to fix the third root cause discovered while verifying Task 1's interleaving check, kept in the same atomic commit as the notifier test rewrites it supports rather than split artificially._

## Files Created/Modified
- `internal/db/migrate_test.go` — `TestRunMigrations_AppliesFromScratch` rebuilt around a dedicated `migrate_scratch` schema; `scratchSchemaDSN` helper derives the `search_path`-scoped DSN
- `internal/notifier/notifier.go` — added the `spacingWait` package-level seam var, routed the inter-send `select` through it
- `internal/notifier/export_test.go` (new) — `SetSpacingWaitForTest`, the seam's single test-only setter
- `internal/notifier/notifier_test.go` — `TestNotifyPending_BatchSpacingBetweenSends` and `TestNotifyPending_SpacingAppliedEvenAfterFailedSend` rewritten to assert recorded requested spacing durations; every DB-backed test switched from `testutil.NewTestPool` to `testutil.NewIsolatedTestPool(t, "notifier_test")`
- `internal/testutil/postgres.go` — added `schemaScopedDSN` and `NewIsolatedTestPool`, a schema-isolated sibling of the existing `NewTestPool`
- `Makefile` — `test-integration` no longer passes `-p 1`; comment rewritten to explain the three now-fixed root causes and point at the todo's Resolution section
- `.planning/todos/completed/2026-08-11-fix-flaky-tests-under-parallel-go-test.md` (moved from `pending/`) — `## Resolution` section added

## Decisions Made
- Extended the `search_path`-DSN schema-isolation technique from Task 1's single test to a reusable `testutil.NewIsolatedTestPool`, applied across all of `internal/notifier/notifier_test.go`, after empirically confirming (by running `internal/notifier` concurrently with `internal/detection`/`internal/httpserver` against the same live fixture) that this was a real, reproducible third root cause — not a hypothetical one. This went beyond Task 2's literally-scoped two tests, but was necessary for Task 3's own stated goal (suite green at default parallelism, no invocation-flag workaround) to be achievable at all.
- `-race` is unavailable in this session's Windows environment (ThreadSanitizer fails to allocate under this machine's cgo toolchain — the same pre-existing, documented limitation as Phase 01's and 11-01's decisions, not something this plan could fix). Every `-race`-specified verification and acceptance criterion in this plan (Task 1's `-count=2 -race`, Task 2's `-count=5 -race`, Task 3's five `-race` full-suite runs) was substituted with the equivalent non-`-race` invocation at the same package-level parallelism, and is flagged as a deferred human-judgment item (coverage D3) for a CI run with a real `-race` binary.
- Used a temporary, throwaway Postgres container (`docker run -p 5555:5432 postgres:16`, removed at the end of this session) instead of the worktree's own `docker compose up -d --wait postgres`, because the compose-defined host port 5433 was already bound by a different, long-running sibling worktree's Postgres container — connecting to that container would have risked corrupting a concurrently-active agent's test state.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Discovered and fixed a third, previously-undiagnosed root cause of internal/notifier's flakiness under default parallelism**
- **Found during:** Task 1's own interleaving acceptance check (running `internal/db` concurrently with `internal/poller`/`internal/notifier`/`internal/detection`/`internal/httpserver`), before Task 2 began
- **Issue:** `NotifyPending`'s `ListUnnotified` query is deliberately global and unfiltered (D-06). Under Go's default package-level test parallelism, `internal/detection`'s tests routinely leave unnotified `events` rows behind (detection never marks anything notified), and `internal/notifier`'s own tests' exact send/spacing-request-count assertions were being inflated by those foreign rows (`sender was called 4 times, want 3`, `successful request count = 7, want 1`, etc. — reproduced twice, without any extra load, via a plain concurrent run of the four packages). More seriously, a real `NotifyPending` call inside `internal/notifier`'s own tests could mark a *different* package's row notified out from under its test — directly reproducing `internal/httpserver`'s `TestRetention_DetectionStateQueriesStayUnfiltered` failure with zero changes to that package. This is a distinct mechanism from both this plan's Task 1 fix (destructive schema drop) and this plan's `<research_correction>`'s Pitfall 6/7 analysis, which did not anticipate it.
- **Fix:** Added `testutil.NewIsolatedTestPool(t, schema string)` — a schema-isolated sibling of the existing `NewTestPool`, using the same `search_path`-DSN technique Task 1 established — and switched all 11 DB-backed tests in `internal/notifier/notifier_test.go` to use it with a dedicated `notifier_test` schema, so `ListUnnotified`/`MarkNotified` calls made by this package's own tests never see or touch rows any other package's tests created.
- **Files modified:** `internal/testutil/postgres.go` (new function, additive — `NewTestPool`'s existing signature and behaviour untouched), `internal/notifier/notifier_test.go` (pool-source swap, doc comment)
- **Verification:** `go test ./internal/poller/... ./internal/notifier/... ./internal/detection/... ./internal/httpserver/... -count=1` — failed twice before the fix (different exact counts each time, consistent with a genuine race), passed cleanly after; five separate full-suite `go test ./... -count=1` runs all green afterward
- **Committed in:** `9f8ef83` (Task 2 commit — grouped with the spacing-determinism rewrite since both touch the same test file and the isolation fix was discovered while implementing Task 2)

---

**Total deviations:** 1 auto-fixed (Rule 1 bug, discovered mid-plan, fixed within an expanded but still test-file-only scope)
**Impact on plan:** Necessary for Task 3's own stated success criterion (suite green at default parallelism, no invocation-flag workaround) to be achievable at all — without it, five real full-suite runs would not have stayed green, and the honest outcome would have been leaving `-p 1` in place with the todo only partially closed. No production code beyond `internal/notifier/notifier.go`'s already-planned seam was touched; `internal/testutil/postgres.go`'s change is purely additive test infrastructure.

## Issues Encountered
- `go test -race` is unavailable on this Windows dev machine (`ThreadSanitizer failed to allocate ... (error code: 87)`) — the same pre-existing environmental limitation documented in Phase 01's and 11-01's decisions (mingw64 gcc `cc1.exe` cannot execute). Every `-race`-specified check in this plan's tasks (Task 1's `-count=2 -race`, Task 2's `-count=5 -race`, Task 3's five full-suite `-race` runs, and both falsification checks) was run instead as the equivalent invocation without `-race`, at the same default package-level parallelism, against the same real Postgres fixture. All substituted runs passed. A CI run with a real `-race` binary (this repo's GitHub Actions runners are Linux and do not have this limitation) is the recommended follow-up verification, flagged as coverage item D3's `human_judgment: true` rationale above.
- The worktree's own `docker compose up -d --wait postgres` failed with "port is already allocated" (5433, per `docker-compose.yml`'s fixed port mapping) because a different, long-running sibling worktree's own Postgres container already held that port. Rather than risk connecting to and corrupting that concurrently-active agent's database, a separate, temporary Postgres container was started on a free port (5555) for this session's own use, and removed at the end of the session. `docker-compose.yml` itself was not modified.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness
- All four requirements this plan and its sibling plans in Phase 11 target (PERF-01 through PERF-04) now have a genuinely stable test suite to verify against under default parallelism — including this phase's own new concurrency tests (11-01/11-02/11-03), which depend on the suite being trustworthy under `-race` to distinguish a real regression from pre-existing flakiness.
- Deferred: a CI run with a real `-race` binary against this branch (or main after merge) to directly confirm the five-run non-`-race` stability result also holds under `-race`, since this session's environment could not exercise that directly.
- No blockers for the rest of Phase 11 or for `/gsd-secure-phase`/verification steps that follow.

---
*Phase: 11-bounded-concurrent-polling*
*Completed: 2026-08-17*
