---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 04
current_phase_name: detection-engine
status: executing
stopped_at: Completed 04-03-PLAN.md
last_updated: "2026-08-08T03:18:27.641Z"
last_activity: 2026-08-07
last_activity_desc: Phase 04 execution started
progress:
  total_phases: 4
  completed_phases: 3
  total_plans: 21
  completed_plans: 20
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-08)

**Core value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.
**Current focus:** Phase 04 — detection-engine

## Current Position

Phase: 04 (detection-engine) — EXECUTING
Plan: 4 of 4
Status: Ready to execute
Last activity: 2026-08-07 — Phase 04 execution started

Progress: [██████████] 95%

## Performance Metrics

**Velocity:**

- Total plans completed: 17
- Average duration: - min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 5 | - | - |
| 02 | 8 | - | - |
| 03 | 4 | - | - |

**Recent Trend:**

- Last 5 plans: -
- Trend: -

*Updated after each plan completion*
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 01 P01 | 90m | 2 tasks | 16 files |
| Phase 01 P02 | 30m | 2 tasks | 2 files |
| Phase 01 P03 | 45m | 2 tasks | 3 files |
| Phase 01 P04 | 65m | 2 tasks | 7 files |
| Phase 01 P05 | 35m | 2 tasks | 2 files |
| Phase 02 P01 | 75m | 1 tasks | 18 files |
| Phase 02 P02 | 20m | 2 tasks | 6 files |
| Phase 02 P03 | 55m | 2 tasks | 8 files |
| Phase 02 P04 | 40m | 2 tasks | 9 files |
| Phase 02 P05 | 15min | 2 tasks | 4 files |
| Phase 02 P06 | 25min | 2 tasks | 6 files |
| Phase 02 P07 | 40min | 2 tasks | 4 files |
| Phase 02 P08 | 15min | 2 tasks | 3 files |
| Phase 03 P01 | 20min | 2 tasks | 13 files |
| Phase 03 P02 | 30min | 3 tasks | 8 files |
| Phase 03 P03 | 30min | 2 tasks | 3 files |
| Phase 03 P04 | 25min | 3 tasks | 5 files |
| Phase 04 P01 | 30min | 3 tasks | 13 files |
| Phase 04 P02 | 25min | 3 tasks | 13 files |
| Phase 04 P03 | 45min | 2 tasks | 10 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap]: 7 phases derived from requirement categories — Foundation, Watchlist Core, External Clients & Search, Detection Engine, Discord Notifications, Frontend & Release History, Containerization & CI/CD Pipeline
- [Roadmap]: Guest-feature and deluxe/tracklist-change detection (DTCT-02, DTCT-03) kept in v1 Phase 4 rather than deferred, per REQUIREMENTS.md v1 scope — research suggested deferring these but REQUIREMENTS.md lists them as Active v1 requirements
- [Phase ?]: Used golang-migrate's pgx/v5 database driver (not the lib/pq-backed generic postgres driver) to keep lib/pq out of the dependency graph per CLAUDE.md
- [Phase ?]: Added explicit request_id log attribute + X-Request-Id response header middleware since go-chi/httplog/v3 has no built-in request-ID field
- [Phase ?]: 01-02: No production changes needed — 01-01's Rule 2 deviations (echoRequestID middleware, httpserver.Pinger seam) already satisfied this plan's requirements; both tasks are test-only commits proving pre-existing behavior.
- [Phase ?]: [Phase 01-03]: Renamed MusicBrainzUA to MusicBrainzUserAgent and completed the Config struct to all 9 fields through Phase 5 (Discord webhook, poll interval, MusicBrainz UA/rate limit, Deezer rate limit) with a reflection-based .env.example parity test
- [Phase ?]: [Phase 01-03]: TestLoad_AggregatesAllMissing asserts on caarlos0/env's Go field name (HTTPPort) rather than the env tag (HTTP_PORT) for type-conversion errors, verified against the library's actual ParseError.Error() implementation
- [Phase ?]: [Phase 01-04]: Installed sqlc v1.31.1 with CGO_ENABLED=0 (pure-Go/WASM parser backend) since this dev machine's mingw64 gcc cc1.exe cannot execute — same pre-existing cgo toolchain break already documented for -race in 01-02/01-03
- [Phase ?]: [Phase 01-04]: Installed GNU Make 4.4.1 via winget (ezwinports.make, per-user, user-approved) after choco install failed on a non-admin permissions error; make targets (build/run/test/test-short/test-integration/sqlc/sqlc-check/db-up/db-down) verified against all acceptance criteria
- [Phase ?]: [Phase 01-05]: Made db.RunMigrations retry policy injectable (WithMaxAttempts/WithBaseDelay/WithMaxDelay) with DSN-to-safe-description redaction on every retry log line and the final error, without touching cmd/server/main.go's call site
- [Phase 01 UAT/Security]: Graceful shutdown (WR-03) and the WR-01 migration-cancellation goroutine were confirmed by hand on this Windows dev box's WSL2 (real SIGTERM + `go test -race`), closing the two human-verification gaps this Windows sandbox couldn't exercise natively.
- [Phase 01 Security]: `/gsd-secure-phase 01` closed all 18 registered threats (`threats_open: 0`). Two mitigation claims were corrected in the process: added `sqlc-version-check` (T-01-13, Makefile had no version guard) and a test proving secret-bearing config fields can never have their value echoed by a type-conversion error (T-01-09, the original "never echoes values" claim was inaccurate for non-secret typed fields). See 01-SECURITY.md.
- [Phase ?]: [Phase 02-01]: httpserver.New widened to a three-arg constructor (db Pinger, store watchlist.Store, logger) -- Pinger stayed untouched, all eight existing call sites updated in the same commit
- [Phase ?]: [Phase 02-01]: text[] + CHECK constraint chosen over native Postgres enum for release_types/muted_event_types, since both value sets are expected to grow (Phase 4 may rename them)
- [Phase ?]: [Phase 02-01]: Fixed internal/db/migrate_test.go's from-scratch reset (drop whole public schema, not just schema_migrations) since 000002 now creates real domain tables a bare reset left behind
- [Phase ?]: [Phase 02-02]: normalizeSet's unit test lives in a separate internal-package file (normalize_test.go, package watchlist) since it tests an unexported function; service_test.go stays package watchlist_test for the real-Postgres tests
- [Phase ?]: [Phase 02-02]: Handler performs its own fail-fast membership check against watchlist.ReleaseTypes/EventTypes before calling Store.Add, so an invalid preference value never reaches the store -- Service.Add's normalizeSet remains the non-bypassable backstop
- [Phase ?]: [Phase 02-03]: RED-phase tests for both List and Remove tasks were written and committed together (one test commit covering both tasks) since drafting happened in one pass; each task's GREEN implementation still landed in its own separate feat commit
- [Phase ?]: [Phase 02-03]: TestService_List_EmptyReturnsNonNilSlice queries actual watchlist row count rather than assuming a truly empty table (testutil.NewTestPool resets schema, not table contents), asserting non-nil always and length-zero only when the count is genuinely zero
- [Phase ?]: [Phase 02-04]: UpdatePreferences reads the current row via the existing ListWatchlist query filtered by id in Go rather than adding a dedicated single-row query, keeping the phase's sqlc query surface at exactly five
- [Phase ?]: [Phase 02-04]: errNotImplemented sentinel removal folded into task 1's GREEN commit (replacing UpdatePreferences's body removed the last reference); task 2's placeholder gate was a grep-based verification, not a second removal step
- [Phase ?]: [Phase 02-05]: Widened UpsertArtist's ON CONFLICT SET list to COALESCE disambiguation and image_url the same way deezer_id already was, closing gap G-02-2a (WR-01) -- regenerated sqlc output (artists.sql.go, querier.go) and two real-Postgres tests pin both the refresh-on-supplied and preserve-on-omitted halves of the contract
- [Phase ?]: [Phase 02-06]: Rewrote UpdateWatchlistPreferences as a single data-modifying CTE (per-axis CASE/ELSE reading the row version the UPDATE itself locked) instead of a two-statement read-then-write, closing gap G-02-2b's lost-update and not-found-on-delete races; qualified every column reference inside the CTE's UPDATE to satisfy sqlc's ambiguity check
- [Phase ?]: quick/260806-hfn: gitleaks pre-commit hook added and proven end-to-end; full-history scan found 4 pre-existing findings (fake test-fixture password), resolved via documented acceptance (not suppression) after 4 human checkpoints -- no history rewrite, no force-push
- [Phase ?]: [Phase 02-07]: Closed gap G-02-1 (WR-01, WR-02) -- moved the neither-axis PATCH guard into Service.UpdatePreferences as ErrNoPreferencesSupplied (first statement, ahead of validation and the id lookup) and replaced both hand-copied JSON decode blocks in internal/httpserver/watchlist.go with a shared decodeJSONBody helper that rejects a body carrying a second JSON value
- [Phase ?]: [Phase 02-08]: Closed gap G-02-2 (CR-01) -- added kvPasswordPattern to redactError for libpq keyword/value-form and query-parameter DSN passwords, gated by a unit-level dsnFixtures table shared with redactDSN since the reachable pgconn.ParseConfig failure path does not currently leak under pinned pgx v5.10.0's own self-redaction
- [Phase ?]: [Phase 03-01]: doRequest wraps ctx in context.WithTimeout only when httpClient.Timeout > 0 -- a zero Timeout means unbounded in net/http's convention, and wrapping it unconditionally created an already-expired deadline that failed every httptest.Server-backed test
- [Phase ?]: [Phase 03-01]: the WithTimeout cancel func is attached to the response body via a cancelReadCloser instead of deferred in doRequest, so the deadline bounds the caller's body read without truncating a healthy response
- [Phase ?]: [Phase 03-01]: WLST-01 and CLNT-03 left unmarked in REQUIREMENTS.md -- both require MusicBrainz AND Deezer; plan 03-02 (which also lists both IDs) is deferred to close them
- [Phase ?]: [Phase 03-02]: Recovered task 2 (Deezer artist-albums fetch) from a stalled prior executor run by verifying the uncommitted implementation against the plan's behavior/action/acceptance-criteria spec before trusting and committing it, preserving the RED-then-GREEN commit split
- [Phase ?]: [Phase 03-02]: WLST-01, CLNT-02, CLNT-03 marked complete -- 03-01 deliberately left WLST-01/CLNT-03 unmarked pending Deezer, which this plan supplies
- [Phase ?]: [Phase 03-03]: Both TDD tasks' RED tests committed together (drafting in one pass), but each task's GREEN implementation landed in its own separate feat commit -- task 1 implements the single-page fetch only, task 2 extends it into the bounded pagination loop
- [Phase ?]: [Phase 03-03]: Fixed releaseGroupFixture's release-group-count (61 -> 1) to match its static single-entry body once real pagination landed -- the mismatched count kept the loop re-fetching the same page until hitting the page ceiling
- [Phase ?]: [Phase 03-04]: robfig/cron/v3 landed indirect in task 1 (go get, nothing imports it yet) and only became a direct dependency once task 2 imported it and go mod tidy re-ran -- task 1's own indirect-block acceptance grep is satisfied cumulatively by end of task 2, per the plan's own staged-action text
- [Phase ?]: [Phase 03-04]: Stop()'s drain-semantics tests drive a real short cron interval and wait for a real dispatched tick rather than calling cycle methods directly -- cron.Cron.Stop()'s returned context only tracks cron-dispatched jobs via its own internal WaitGroup, so a directly-invoked cycle would make Stop() return immediately regardless of whether it had finished
- [Phase ?]: [Phase 03-04]: CLNT-01/CLNT-02 were already checked off in REQUIREMENTS.md by 03-02/03-03 on the strength of the underlying client fetch methods existing, even though the requirement text names scheduled polling specifically -- this plan is what actually delivers that behavior; requirements.mark-complete re-run here as a no-op confirmation
- [Phase 03 UAT]: Live MusicBrainz search UAT test failed with sources.musicbrainz.status:"error" on a real WSL2 dev machine; diagnosed as environmental (WSL2 TLS/MTU path issue to musicbrainz.org specifically, reproduced identically with plain curl bypassing this codebase's HTTP client entirely) -- not a drop-tracker defect. Backstop-assumption test (Deezer quota-error shape / MusicBrainz throttling) was knowingly skipped rather than forcing abusive live traffic against a third party. Both closed via human-approved gate override (marked pass in 03-UAT.md with full context preserved) so the automated completion predicate reflects the resolved outcome -- see 03-VERIFICATION.md Acknowledged Gaps.
- [Phase ?]: [Phase 04-01]: Task 1 checkpoint resolved as option-a -- mutable track_count column on events, not a separate release_group_baselines table; column created now, populated starting in plan 04-04
- [Phase ?]: [Phase 04-01]: newTestPoller took a variadic trailing EventRecorder parameter so the widened poller.New constructor did not require touching all existing test call sites
- [Phase ?]: [Phase 04-02]: Existing 04-01 test fixtures updated to carry ReleaseTypes/PrimaryType since Task 1's D-17 filter now rejects an entry with no ReleaseTypes -- a real watchlist entry always has ReleaseTypes populated per Phase 2 D-08 defaults
- [Phase ?]: [Phase 04-02]: DetectDeezer structurally mirrors DetectMusicBrainz (mute check, seed-mode check, seen-set lookup, per-item filter, insert) rather than sharing a generic helper, since the two sources differ in id formatting and filter input (RecordType vs PrimaryType)
- [Phase ?]: [Phase 04-03]: Centralized seedMode/notifiedAt computation to the top of DetectMusicBrainz, shared by both new_release and guest_feature passes -- an independent per-pass isSeedMode call would have seen the other pass's just-inserted rows and flipped seed mode mid-cycle, unseeding a newly-watched artist's guest-feature catalogue on their first poll
- [Phase ?]: [Phase 04-03]: isGuestFeature/displayArtistName unit tests live in a new internal/detection/musicbrainz_test.go (package detection, whitebox) since isGuestFeature is unexported -- mirrors filter_test.go's convention

### Pending Todos

None yet.

### Blockers/Concerns

- ⚠️ [Phase 03] musicbrainz.org's TLS handshake fails from this developer's WSL2 network path (confirmed environmental via plain curl, not app code) -- Deezer unaffected. If future live testing on this machine needs real MusicBrainz data, expect the same failure; see PROJECT.md Context and Broken Windows Ledger entry #3 (waived).

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260806-hfn | Add a gitleaks pre-commit hook so secrets are caught locally before commit | 2026-08-06 | 18ad467 | [260806-hfn-add-a-gitleaks-pre-commit-hook-so-secret](./quick/260806-hfn-add-a-gitleaks-pre-commit-hook-so-secret/) |

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-08-08T03:18:27.598Z
Stopped at: Completed 04-03-PLAN.md
Resume file: None
