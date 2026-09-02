---
gsd_state_version: 1.0
milestone: v1.3
milestone_name: Continuous Deployment
current_phase: 15
current_phase_name: PR Coverage-Diff Comment
status: planning
stopped_at: Phase 14 complete, ready to plan Phase 15
last_updated: "2026-09-01T21:16:01.523Z"
last_activity: 2026-09-01
last_activity_desc: Phase 14 complete, transitioned to Phase 15
state_head: 5de010b834fce6a4c259d564b55fce82d30c916d
progress:
  total_phases: 4
  completed_phases: 1
  total_plans: 7
  completed_plans: 7
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-01)

**Core value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.
**Current focus:** Phase 15 — PR Coverage-Diff Comment

## Current Position

Phase: 15 — PR Coverage-Diff Comment
Plan: Not started
Status: Ready to plan
Last activity: 2026-09-01 — Completed quick task 260901-rl9: removed the orphaned envExamplePlaceholder ("caliber") apparatus from internal/authgate (const + denylist entry + drift-guard test), orphaned by 465260c blanking the .env.example placeholder; IsWeakPassphrase behavior unchanged

## Performance Metrics

**Velocity:**

- Total plans completed: 64
- Average duration: - min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 5 | - | - |
| 02 | 8 | - | - |
| 03 | 4 | - | - |
| 04 | 4 | - | - |
| 06 | 4 | - | - |
| 07 | 4 | - | - |
| 08 | 5 | - | - |
| 09 | 5 | - | - |
| 10 | 2 | - | - |
| 11 | 5 | - | - |
| 11.1 | 5 | - | - |
| 12 | 3 | - | - |
| 13 | 3 | - | - |
| 14 | 7 | - | - |

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
| Phase 04 P04 | 40min | 2 tasks | 12 files |
| Phase 05 P01 | 55min | 2 tasks | 15 files |
| Phase 05 P02 | 45min | 2 tasks | 7 files |
| Phase 05 P03 | 40min | 2 tasks | 2 files |
| Phase 11.1 P04 | 240min | 4 tasks | 4 files |
| Phase 14 P01 | 55min | 4 tasks | 12 files |
| Phase 14 P02 | 45 min | 3 tasks | 10 files |
| Phase 14 P03 | 12 min | 3 tasks | 9 files |
| Phase 14 P04 | 11 min | 3 tasks | 10 files |
| Phase 14 P05 | 20 min | 4 tasks | 5 files |
| Phase 14 P06 | 15 min | 3 tasks | 4 files |
| Phase 14 P07 | 25 min | 4 tasks | 8 files |

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
- [Phase ?]: [Phase 04-04]: groupBaseline returns (baseline int, hasBaseline bool, error) rather than collapsing 'no baseline' and 'baseline is zero' into one value -- the load-bearing distinction preventing 04-RESEARCH.md Pitfall #1's false-positive deluxe_change on a group's first real comparison cycle
- [Phase ?]: [Phase 04-04]: preCycleSeenGroups is captured once at the top of DetectMusicBrainz, before the new_release pass inserts anything, and threaded unchanged into detectDeluxeChanges -- guarantees D-04 (a group discovered this very cycle never gets a release-detail fetch in the same cycle)
- [Phase ?]: [Phase 05-01]: Migration 000004 added previous_track_count/release_type nullable columns to events (D-04, NTFY-01); MarkNotified added as an idempotent AND notified_at IS NULL ack (D-09). Fixed migrate_test.go's hardcoded schema version (3 -> 4) as a direct, in-scope consequence.
- [Phase ?]: [Phase 05-01]: internal/discord.Client never wraps httpClient.Do's raw error (Go's *url.Error embeds the full request URL, and a Discord webhook path IS its secret token) -- returns a fixed error string instead (T-05-01).
- [Phase ?]: [Phase 05-01]: internal/notifier.Notifier uses an atomic.Bool CAS-skip guard (D-06), mirroring poller's mbRunning/dzRunning idiom, not a blocking mutex -- a slow rate-limited send burst from one cycle must never stall the other cycle's own notify call.
- [Phase ?]: [Phase 05-02]: Extracted releaseTypeForStorage helper in musicbrainz.go so the absent-PrimaryType-stores-NULL behavior is unit-testable directly, since releaseTypeAllowed filters an empty PrimaryType before DetectMusicBrainz's insert is ever reached
- [Phase ?]: [Phase 05-02]: Placed real-Postgres release_type/previous_track_count assertions in detector_test.go/deezer_test.go (the codebase's established real-Postgres test files for DetectMusicBrainz/DetectDeezer) rather than musicbrainz_test.go as the plan's frontmatter literally named, since musicbrainz_test.go is this codebase's whitebox-only, no-DB test file
- [Phase ?]: [Phase 05-03]: Added a seed cycle to the NTFY-04 through-notifier test since a bare first-cycle new_release row is always pre-notified by D-14's seed mode, leaving NotifyPending nothing to drain
- [quick/260808-pt0]: `/gsd-audit-milestone` was scoped to phases 1-5 only (user choice) since phases 6-7 are unplanned (`Plans: TBD`) -- auditing them now would only report "not started," already known from ROADMAP.md. Full v1.0 audit deferred until phases 6-7 complete; see `.planning/v1.0-MILESTONE-AUDIT.md`.
- [quick/260808-pt0]: Closed Phase 5's two open backstop-tier truths partially -- the 256-rune title boundary and total-embed-budget checks were closeable by an automated test and now are (`internal/notifier/format_test.go`); the live Discord rendering/mention-suppression check is not automatable and stays open, tracked under Blockers/Concerns rather than silently dropped.
- [Phase 06-01]: React Router SPA Mode (`ssr: false`) scaffold embedded via `internal/webassets`'s `//go:embed all:build/client`, with a chi `NotFound`-registered fallback serving `index.html` for client-side routes while every explicit API route still resolves first; `Makefile`'s `web` target rebuilds and replaces the committed `internal/webassets/build/client/` tree so a Node-less clone still builds/tests/vets clean.
- [Phase 06]: All XSS-surfaced threats (event titles, artist names/disambiguation, search-result text from MusicBrainz/Deezer) closed via plain-JSX-text-node rendering only -- a repo-wide `dangerouslySetInnerHTML` grep across `web/app/` returns zero matches.
- [Phase 06-03]: Optimistic preference-toggle rollback (T-06-13, D-12 prohibition) -- pre-toggle state is restored and a failure toast fires on any non-OK PATCH response, so the UI never keeps showing an unsaved value as persisted; confirmed live via UAT (forced PATCH failure, checkbox reverted, toast shown).
- [Phase 06-04]: Search-as-you-type debounced ~300ms with `AbortController` supersession cancellation, backed by the existing per-source `rate.Limiter` and a fixed `searchResultLimit` of 10 -- bounds outbound MusicBrainz/Deezer traffic regardless of client behavior.
- [Phase 06 Security]: `/gsd-secure-phase 06` closed all 23 registered threats across the phase's 4 plans (19 mitigated, 4 accepted with documented rationale) via L1 grep-depth verification against the implementation -- ASVS level 1, no deeper auditor pass required. See 06-SECURITY.md.
- [Phase 06 UAT]: Search-result popularity ranking / same-name disambiguation accepted as a valid, out-of-scope enhancement rather than a defect; captured as backlog Phase 999.1 in ROADMAP.md.
- [Phase 05 UAT, closed 2026-08-11]: Retroactively closed the last deferred Phase 5 verification gap (Live Discord rendering + mention-suppression check). First run exposed a real bug -- diagnosed via `/gsd-debug` to two AND-gated causes: (1) local dev DB collided with an unrelated agent worktree's Postgres container squatting on host port 5432, so seeded UAT rows never reached the database the app actually queried; (2) no timeout existed anywhere in the notifier's DB call path, so a dead socket wedged the notify pass forever. Fixed via bounded pgxpool/query timeouts (`internal/db/pool.go`, `internal/notifier/notifier.go`, commit `479c781`) and remapping `docker-compose.yml`'s Postgres port to 5433 to stop colliding with other worktrees. Both 05-UAT.md tests now pass; 05-VERIFICATION.md status set to `passed`. Full investigation: `.planning/debug/resolved/notify-pass-hangs-forever.md`.
- [v1.1 Roadmap]: 4 phases derived from the 10 v1.1 requirements — Frontend Test Suite, CI Coverage Gates, Event Retention Window, Bounded Concurrent Polling.
- [v1.1 Roadmap]: Research proposed 5 phases with the two coverage gates split (backend gate first, frontend gate third). Merged into one Phase 09 instead — both gates edit the same `.github/workflows/full-pipeline.yml`, and splitting them left two single-requirement phases. Ordering constraint research cared about is preserved: the frontend suite (Phase 08) still lands before any gate measures it.
- [v1.1 Roadmap]: Event retention is soft-delete/filter, locked — never hard delete. Rows past the window are hidden from History/API but stay in the table so dedup keys, `events.track_count` deluxe baselines, and per-source seed-mode state survive. The `release_group_baselines` migration that hard delete would have required is rejected. Phase 10's success criteria 3-5 exist to prove each of the three regressions hard delete would have reintroduced.
- [v1.1 Roadmap]: Coverage thresholds are 80% backend / 70% frontend per REQUIREMENTS.md (CICD-11/12), not the flat 70%/70% research assumed. If a measured baseline lands under its threshold, Phase 09 closes the gap with real tests rather than lowering the number.
- [v1.1 Roadmap]: Bounded concurrent polling (Phase 11) lands last — highest correctness risk of the milestone, and it benefits from executing behind working coverage gates.
- [Phase 09 UAT, closed 2026-08-13]: Closed the phase's backstop-tier human-verification truth (real GitHub Actions run proving the coverage gate fires and `build-scan` is skipped) live on scratch branch `test/coverage-gate-ci-check` — never `main`. All three cases directly observed: backend threshold raised past measured 87.1% → `test` red, `build-scan`/`release` skipped (run 31724487315); frontend thresholds raised past measured 78/72/76/80% → `frontend-test` red, same skip (run 31724744670); both restored to 80/70 → full pipeline green through `build-scan` (run 31724954534). Branch deleted after (local + remote).
- [Phase 09 Security]: `/gsd-secure-phase 09` closed all 25 registered threats (22 mitigated, 3 accepted) via L1 grep-depth verification plus the live CI runs above as direct evidence for the two highest-severity threats (T-09-19 `needs` graph, T-09-24 scratch-branch discipline). See 09-SECURITY.md.
- [Phase ?]: [Phase 11.1-04]: Task 4's .env.example DATABASE_URL port fix (5433 -> 5432) performed by the human developer directly, since no agent tool in this workflow can Read/Edit that file (permission-denied, reproducing 11-REVIEW.md IN-02).
- [Phase ?]: [Phase 11.1-04]: go test -race unusable on this Windows dev machine (ThreadSanitizer allocation failure under memory pressure) -- pre-existing documented limitation; substituted plain go test for make test-integration's verification, coverage-gate confirmed 87.1% matching prior baseline.
- [Phase 13]: Code review (13-REVIEW.md) surfaced 3 warning-severity findings (0 critical) -- WR-01 earlierDate panic on a MusicBrainz date <4 chars, WR-02 Backfill Stats double-increment on a failed RecordArtMatchAttempt, WR-03 ActivityGate leak if Matcher.Match panics (cancel/end not deferred). User chose "fix now" during UAT; all three fixed RED-then-GREEN with regression tests (commits a77041e/1029f5b, 5ca9f4a/abf0cdc, df9406d/722fe8e), full build/vet/lint/test clean after.
- [Phase 13 UAT]: The one human-verification item that couldn't be checked in this sandbox (MusicBrainz ws/2/recording response shape vs. recording_lookup.go's [ASSUMED] struct tags -- WSL2 can't reach musicbrainz.org) was confirmed live by the user against a real recording (03dbaf8a-23ce-43d6-8e69-2778ec82ab61) -- releases[].date and releases[].release-group.id both match exactly. Assumption A1 (13-RESEARCH.md) closed.
- [Phase 13 Security]: `/gsd-secure-phase 13` closed all 22 registered threats (21 from the plans' STRIDE registers + 1 new supply-chain finding: 13-01 added `@testing-library/dom` as a devDependency, undocumented by the plan's "zero npm packages" claim -- verified as the official first-party peer dependency of an already-approved package, dev-only, accepted as AR-13-05). The gsd-security-auditor subagent hit a session usage limit mid-run; verification was completed inline (ASVS L1 grep-depth) against the same threat register rather than retried. See 13-SECURITY.md.
- [quick/260826-hea]: Isolated `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier` onto its own `detection_notify_test` Postgres schema via `testutil.NewIsolatedTestPool`, the last test in the repo making a real `NotifyPending` call against the shared fixture's default schema (root-caused, left unfixed, in 260826-gj8). The RED-half sentinel probe against the unmodified test proved the bug is not hypothetical: it collaterally marked 45 genuine live-app pending Discord notifications as sent in one run; those rows were restored to `notified_at IS NULL` before proceeding. GREEN probe and full `internal/detection`/`internal/notifier` suites confirmed clean after the fix.
- [v1.3 Roadmap]: 4 phases derived from the 21 v1.3 requirements — Instance Passphrase Gate (14), PR Coverage-Diff Comment (15), Rollback-Safe Migrations (16), Automated VPS Deploy with Health-Gated Rollback (17). Phase numbering continues from v1.2's Phase 13; it does not reset.
- [v1.3 Roadmap]: Research (SUMMARY.md) and PITFALLS.md both proposed a 3-phase split with deploy + migration-safety + VPS provisioning as one final phase. Split into 4 instead: migration safety (MGRT-01/02) was pulled out as its own Phase 16 placed *before* the deploy job. Rationale — PITFALLS.md #8 (a non-backward-compatible migration bricking rollback) is the milestone's highest-cost failure mode and is cross-cutting: the expand/contract rule binds every migration from now on, not just those written during the deploy phase. Building the safety precondition last, inside the heaviest phase, would put it after the thing it protects. It is also CI-only, so it needs no VPS and can be verified independently.
- [v1.3 Roadmap]: VPS provisioning (DPLY-07) and the HTTPS reverse proxy (DPLY-08) deliberately stayed *inside* Phase 17 rather than becoming a separate provisioning phase. Splitting them would have straddled DPLY-04 ("production compose file **and rollout script** are versioned in the repo; secrets and pinned tag live only on the VPS") across two phases — the runbook and the rollout artifacts are one interlocking deliverable.
- [v1.3 Roadmap]: Exposure ordering is the binding constraint on phase order — Phase 14 (passphrase gate) must be merged before Phase 17 makes the instance publicly reachable, so the instance is never briefly public-and-open. PITFALLS.md's network-layer allowlist is kept as **belt-and-suspenders during Phase 17 provisioning** (added to DPLY-07) — the app gate is the load-bearing control, the firewall is the backstop while the proxy + gate are being brought up.
- [v1.3 Roadmap]: Phases 15, 16, and 17 all edit `.github/workflows/full-pipeline.yml` — the same shared-file hazard the repo already documented for `frontend-test` (Phase 08 → Phase 09). Sequencing 15 → 16 → 17 keeps the edits serialized. Phase 17 additionally requires a small change to the existing `release` job to expose `outputs.version`.
- [v1.3 Roadmap]: Phase 14 is a joint backend + frontend slice, not split. Server-side `401` contract and SPA `401`-interception are one feature (PITFALLS.md #23) — splitting them would leave a half-phase that renders a visibly broken app.
- [v1.3 Roadmap]: Phase 17 flagged as needing a discuss/spec pass before planning. Open decisions: documented-and-accepted swap-gap downtime vs. mitigation, and post-deploy image-prune policy. TLS reverse-proxy choice is **recommended Caddy on-VPS** (self-hosted ethos, no third party in the request path) — lock at the Phase 17 discuss but before Phase 15 planning since it blocks the first deploy. Phase 14's decisions are all resolved (D-01…D-18). Phase 15's open decision: baseline storage (Actions cache vs. orphan branch).
- [v1.3 Roadmap]: Phase 14 plan-hardening pass (2026-08-27) — added `TRUST_PROXY_HEADERS` env var gating `middleware.RealIP` (D-14 was a comment-only "accept"; now fail-safe by default); undelayed 429 + `maxConcurrentLogins` semaphore on the login handler (D-12); `gateActive` Log out signal locked as D-18 (was a UAT assumption); 90-day cap clarified as per-authentication (D-06); `14-VALIDATION.md` content authored (formal `/gsd-validate-phase 14` still pending).
- [v1.3 Roadmap]: OPS-04 (VPS Postgres backup) — the **basic backup + restore procedure is folded into DPLY-07** (Phase 17 runbook) as the schema-recovery path for a rollback that outran an irreversible migration (PITFALLS #8). OPS-04 remains in Future for *scheduled* backups + off-box retention. Schedule the basic procedure as Phase 16 or 17 work, not "someday".
- [Phase 14]: [14-01] Session cookie name = "dt_session" (Task 1 option-a): bare name with explicit Secure/HttpOnly/SameSite=Lax/Path=/ attributes, not D-09's literal __Host- prefix. __Host- is rejected by Chrome on http://localhost (research A1); every guarantee it enforces is set and asserted in our own code. Operator-approved deviation from D-09.
- [Phase 14]: [14-01] internal/authgate is a stateless HMAC-SHA256 signed-cookie gate: key = SHA256(domain-prefix || INSTANCE_PASSPHRASE) (D-01, rotation is revoke-all), no server-side session store (survives restart, D-08), gate is group-scoped chi middleware after the 4 existing middlewares (D-05), inert when INSTANCE_PASSPHRASE empty (GATE-07).
- [Phase 14]: [14-01] Phase 14 adds two env vars not one: INSTANCE_PASSPHRASE + TRUST_PROXY_HEADERS (default false, gates middleware.RealIP per D-14). Session timings stay hardcoded (D-07). .env.example edited by operator (denied to agent tools, Phase 11.1-04 limitation).
- [Phase 14]: [14-03] SPA gate: authStore is a plain pub/sub module (authed optimistic-true, gateActive=D-18) poked by a single apiFetch 401 interceptor; X-Requested-With: drop-tracker injected centrally on every non-GET (D-15 client half); PassphraseScreen + gateActive-gated Log out. GATE-05/06 complete.
- [Phase 14]: [14-03] Frontend test pattern: singleton-store tests reset via vi.resetModules()+dynamic import; a test needing the real ApiError class uses a partial mock of ~/lib/api (importOriginal spread + vi.fn spies) instead of a bare automock which breaks instanceof/err.status.
- [Phase 14]: [Phase 14]: [14-05] G-14-1 closed. docker-compose.yml app.environment: forwards INSTANCE_PASSPHRASE/TRUST_PROXY_HEADERS as ${VAR:-default} interpolations (env_file: .env still primary); cmd/server logInstanceGateStatus emits one secret-free Info line per boot (active/inert), sourced from cfg.InstancePassphrase not a second os.Getenv; TestDockerComposeWiresGateEnvVars makes a dropped gate env entry a CI failure. docker compose config banned as a verify step (inlines env_file secrets). Operator reconciled the live .env (denied to agent tools).
- [Phase 14]: [14-06] gateActive persisted to sessionStorage (key dt_gate_active, value "1"): seeded once at module load via a typeof+try/catch guarded reader, written through by both mark* functions; isGateActive still returns the cached module boolean (useSyncExternalStore contract); authed stays volatile/unpersisted (D-16); root.tsx unchanged. Closes G-14-2 — Log out control survives a document reload for the browser session.
- [Phase 14]: [14-07] G-14-3 closed: gate.Authenticate sets X-Instance-Gated: 1 on its proven-valid-cookie path (before next.ServeHTTP, never on the 401 returns); apiFetch latches it via new authStore.markGateActive() — a one-way latch that never reads/writes authed (D-16) and never clears. Marker registered only in server.go's gate != nil branch, so D-18's ungated guarantee now holds structurally. root.tsx and internal/httpserver/server.go unmodified.
- [Phase 14]: [14-07] D-18's gateActive trigger set is now three: an observed 401, a completed login, or an observed gated response (X-Instance-Gated marker). The third strengthens the ungated guarantee (marker only exists on the gated code path) rather than weakening it. Recorded in SUMMARY, not by editing 14-CONTEXT.md.
- [Phase 14]: [14-07] WR-01 retired: the typeof sessionStorage probe moved inside the existing try in readPersistedGateActive/persistGateActive, covered by a jsdom test that redefines the global with a throwing getter. One catch now covers all three storage failure modes (absent, throwing methods, throwing accessor).
- [Phase 14 UAT, closed 2026-09-01]: All 5 UAT tests pass. Test 5 (G-14-3 re-run: carried-cookie fresh gated session renders the Log out control on first authed view, survives refresh/tab-nav/add-artist, absent on an ungated instance) confirmed by the operator in a real browser against `docker compose up --build`. G-14-1/G-14-2/G-14-3 all resolved. Phase 14 marked complete, transitioned to Phase 15.
- [Phase 14 Security]: `/gsd-secure-phase 14` (State B, ASVS L1, block_on high) — 67 threats across the 7 plans' STRIDE registers, 66 closed, `threats_open: 0`. No auditor spawn (short-circuit: register authored at plan time + L1). One below-threshold open item recorded as T-14-CACHE-01 (14-REVIEW WR-01): gated authenticated 2xx responses set no `Cache-Control: no-store` / `Vary: Cookie` — medium, non-blocking, recommendation logged for the same middleware that stamps `X-Instance-Gated`. See 14-SECURITY.md.

### Pending Todos

None yet.

### Blockers/Concerns

- ⚠️ [Phase 03] musicbrainz.org's TLS handshake fails from this developer's WSL2 network path (confirmed environmental via plain curl, not app code) -- Deezer unaffected. If future live testing on this machine needs real MusicBrainz data, expect the same failure; see PROJECT.md Context and Broken Windows Ledger entry #3 (waived).
- ⚠️ [Phase 17, v1.3] No VPS is provisioned yet, and the TLS reverse-proxy choice (Caddy on-VPS vs. Cloudflare Tunnel) is undecided. Both block Phase 17's first real deploy and its rollback drill. Resolve in Phase 17's discuss/spec pass before planning.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260806-hfn | Add a gitleaks pre-commit hook so secrets are caught locally before commit | 2026-08-06 | 18ad467 | [260806-hfn-add-a-gitleaks-pre-commit-hook-so-secret](./quick/260806-hfn-add-a-gitleaks-pre-commit-hook-so-secret/) |
| 260808-pt0 | Close out Phase 5: commit docs, cleanup stray binary, mark phase complete, close backstop truncation test | 2026-08-08 | cbe73af | [260808-pt0-close-out-phase-5-commit-docs-cleanup-st](./quick/260808-pt0-close-out-phase-5-commit-docs-cleanup-st/) |
| 260817-cfu | Bump the Dockerfile's Go builder-stage base image from golang:1.26.5-alpine3.24 to a patched release to fix 8 HIGH-severity stdlib CVEs failing the Trivy build-scan gate in CI | 2026-08-17 | 4f58465 | [260817-cfu-bump-the-dockerfile-s-go-builder-stage-b](./quick/260817-cfu-bump-the-dockerfile-s-go-builder-stage-b/) |
| 260823-sp6 | Add a note in CLAUDE.md instructing agents to use graphify (the knowledge graph in graphify-out/) whenever possible for codebase questions, to save tokens and be more efficient | 2026-08-23 | 336e2ef | [260823-sp6-add-a-note-in-claude-md-instructing-agen](./quick/260823-sp6-add-a-note-in-claude-md-instructing-agen/) |
| 260823-t7n | Prevent graphify knowledge graph from being bloated with stale/completed planning docs: add .graphifyignore excluding archival planning content and note the convention in CLAUDE.md's graphify section | 2026-08-23 | e2006ab | [260823-t7n-prevent-graphify-knowledge-graph-from-be](./quick/260823-t7n-prevent-graphify-knowledge-graph-from-be/) |
| 260824-1gy | Fix code-review finding: extract shared rate-limited HTTP client seam from musicbrainz/deezer clients into internal/httpclient | 2026-08-24 | 0d0050f | [260824-1gy-fix-code-review-finding-extract-shared-r](./quick/260824-1gy-fix-code-review-finding-extract-shared-r/) |
| 260824-23o | Fix code-review finding #2: factor RunMusicBrainzCycle/RunDeezerCycle's duplicated bounded-fan-out logic in internal/poller/poller.go into a shared runCycle method | 2026-08-24 | f65f959 | [260824-23o-fix-code-review-finding-2-factor-runmusi](./quick/260824-23o-fix-code-review-finding-2-factor-runmusi/) |
| 260824-2fw | Fix code-review finding #3: extract shared trimAndCap helper for handleAddWatchlist's repeated field validation in internal/httpserver/watchlist.go | 2026-08-24 | ba8f3d9 | [260824-2fw-fix-code-review-finding-3-extract-shared](./quick/260824-2fw-fix-code-review-finding-3-extract-shared/) |
| 260824-2q4 | Fix code-review finding #4: centralize frontend source-identity logic (isAddableSource/identityField) instead of inline musicbrainz/deezer string comparisons | 2026-08-24 | d8bdd3d | [260824-2q4-fix-code-review-finding-4-centralize-fro](./quick/260824-2q4-fix-code-review-finding-4-centralize-fro/) |
| 260824-339 | Fix code-review findings #5 and #6: extract nilIfEmpty helper in internal/httpserver/search.go, and fix stale /watchlist route literal in web/app/routes/watchlist.test.tsx | 2026-08-24 | 50b554f | [260824-339-fix-code-review-findings-5-and-6-extract](./quick/260824-339-fix-code-review-findings-5-and-6-extract/) |
| 260825-09r | Add MusicBrainz url-rels Deezer link (Tier 0) and alias-based strict-match retry (Tier 1) to artistart.Matcher | 2026-08-25 | ecd5024 | [260825-09r-add-musicbrainz-url-rels-deezer-link-tie](./quick/260825-09r-add-musicbrainz-url-rels-deezer-link-tie/) |
| 260825-g6i | Improve History tab: show which watchlist artist is featured on guest_feature events, and order the feed by release chronology instead of detection order | 2026-08-25 | f6a0f31 | [260825-g6i-improve-history-tab-show-which-watchlist](./quick/260825-g6i-improve-history-tab-show-which-watchlist/) |
| 260826-gj8 | Bound deluxe-change rechecks to release-groups released within a rolling window (or backoff schedule) so old catalog entries stop being re-fetched from MusicBrainz every poll cycle | 2026-08-26 | 0b07d90 | [260826-gj8-bound-deluxe-change-rechecks-to-release-](./quick/260826-gj8-bound-deluxe-change-rechecks-to-release-/) |
| 260826-hea | Isolate TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier onto a dedicated Postgres schema so its real NotifyPending call can no longer mark the live dev app's pending Discord notifications as sent | 2026-08-26 | acef51e | [260826-hea-isolate-testdetectmusicbrainz-guestfeatu](./quick/260826-hea-isolate-testdetectmusicbrainz-guestfeatu/) |
| 260826-ia0 | Fix flaky timing tolerance in poller rate-limit tests (TestMusicBrainzCycle_ConcurrentPollingStaysInsideRateLimit and TestDeezerCycle_ConcurrentPollingStaysInsideRateLimit) | 2026-08-26 | 9519de5 | [260826-ia0-fix-flaky-timing-tolerance-in-poller-rat](./quick/260826-ia0-fix-flaky-timing-tolerance-in-poller-rat/) |
| 260826-iu0 | Fix Trivy build-scan CI failure: upgrade runtime-stage Alpine packages at build time to clear CVE-2026-14456 in libssl3/libcrypto3 | 2026-08-26 | 9afdfa2 | [260826-iu0-fix-trivy-build-scan-ci-failure-bump-run](./quick/260826-iu0-fix-trivy-build-scan-ci-failure-bump-run/) |
| 260901-jre | Fix commit pipeline blockers: line-scoped //nolint for 3 golangci-lint findings (2× gosec G115 in authgate/session.go, 1× staticcheck SA1019 on middleware.RealIP) + prettier reformat of 6 Phase 14 frontend files | 2026-09-01 | 30839fb | [260901-jre-fix-commit-pipeline-blockers-golangci-li](./quick/260901-jre-fix-commit-pipeline-blockers-golangci-li/) |
| 260901-knc | Close frontend-formatting CI gap: add local prettier --write pre-commit hook (mirrors golangci-lint --fix pattern) + Definition of Done checklist at top of CLAUDE.md | 2026-09-01 | bf041b2 | [260901-knc-close-frontend-formatting-ci-gap-add-loc](./quick/260901-knc-close-frontend-formatting-ci-gap-add-loc/) |
| 260901-lvn | Add repo-root .gitattributes pinning the tree to LF (overrides core.autocrlf) + forced re-checkout to clear 455 working-tree CRLF files; ends the prettier stat-dirty loop | 2026-09-01 | 1524333 | [260901-lvn-add-repo-root-gitattributes-and-renormal](./quick/260901-lvn-add-repo-root-gitattributes-and-renormal/) |
| 20 | Add comment-conciseness rule to CLAUDE.md + CONVENTIONS.md (stop multi-paragraph comment essays; authStore.ts cited as anti-pattern) | 2026-09-01 | 5de010b | — |
| 260901-muu | Trim essay-style comments in 6 most comment-dense files (authStore.ts + config/pool/detector/musicbrainz/notifier) — 999→349 comment lines, no logic changes, diff-gated comments-only | 2026-09-01 | eed828a | [260901-muu-trim-essay-style-code-comments-in-6-file](./quick/260901-muu-trim-essay-style-code-comments-in-6-file/) |
| 260901-rl9 | Remove orphaned envExamplePlaceholder ("caliber") apparatus from internal/authgate — const + doc comment + knownDefaults entry + TestWeakPassphrase_EnvExamplePlaceholderOnDenylist + table case; orphaned by 465260c blanking .env.example's INSTANCE_PASSPHRASE placeholder. Generic denylist + 16-rune floor untouched; IsWeakPassphrase behavior byte-for-byte unchanged (91.0% coverage held) | 2026-09-01 | e555b34 | [260901-rl9-remove-orphaned-envexampleplaceholder-ap](./quick/260901-rl9-remove-orphaned-envexampleplaceholder-ap/) |

### Roadmap Evolution

- Phase 11.1 inserted after Phase 11: Address tech debt: v1.1 cleanup (URGENT)
- Phase 12 added: Cleanup: CoverArt Reset & Search Popularity Ranking — bundles the deferred CoverArt.tsx image-reset bug with backlog Phase 999.1 (search popularity/disambiguation), which was promoted and folded into this phase
- Phase 13 added: Fix History Dates, Guest-Feature Art & Artist Art — bundles three post-Phase-12 display/data bugs (History tab missing release dates, guest-feature cards missing album art, MusicBrainz artist art not rendering) with backlog Phase 999.2 (Deezer artist-art backfill), which was absorbed where it overlaps with the artist-art bug
- v1.3 milestone opened (2026-08-27): Phases 14-17 added — Instance Passphrase Gate, PR Coverage-Diff Comment, Rollback-Safe Migrations, Automated VPS Deploy with Health-Gated Rollback. Phase numbering continued from 13 rather than resetting.
- Phase 16 (Rollback-Safe Migrations) split out of the research-recommended single deploy phase so the N-1 schema-compatibility guarantee is in force before anything can auto-roll-back.

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Operations | OPS-04 — VPS Postgres backup + restore runbook | Future requirement; recommended prerequisite for Phase 17 | v1.3 roadmap |
| Operations | OPS-05 — ghcr.io image-retention / cleanup job | Future requirement | v1.3 roadmap |
| Deployment | DPLY-09 — near-zero-downtime deploy (connection draining / second container) | Future requirement | v1.3 roadmap |
| Access Gate | GATE-08 — session signing-key rotation without logging everyone out | Future requirement | v1.3 roadmap |
| CI/CD | CICD-15 — patch/diff-level coverage in the PR comment | Future requirement | v1.3 roadmap |

## Session Continuity

Last session: 2026-09-01T18:48:00Z
Stopped at: Phase 14 complete (UAT 5/5, security cleared), ready to plan Phase 15
Resume file: None

## Operator Next Steps

- `/clear` then `/gsd-plan-phase 15` (or `/gsd-discuss-phase 15` first) — PR Coverage-Diff Comment. Open decision: baseline storage (Actions cache vs. orphan branch).
- Phase 15's edit target is `.github/workflows/full-pipeline.yml` — the same shared file Phases 16 and 17 touch; keep the 15 → 16 → 17 order.
- Before planning Phase 17, run its discuss/spec pass — the TLS reverse-proxy choice blocks the first deploy
- Non-blocking follow-up from Phase 14 security: consider `Cache-Control: no-store` on the gated response path (T-14-CACHE-01 / 14-REVIEW WR-01)
