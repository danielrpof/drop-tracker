---
phase: 10-event-retention-window
verified: 2026-08-13T22:43:20Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 10: Event Retention Window Verification Report

**Phase Goal:** Users see recent release history instead of an ever-growing scroll, while the system keeps every row it needs to stay correct — nothing is ever deleted.
**Verified:** 2026-08-13T22:43:20Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Operator can set the retention window with an env var; unset defaults to 90 days | ✓ VERIFIED | `internal/config/config.go:44` — `EventRetentionDays int \`env:"EVENT_RETENTION_DAYS" envDefault:"90"\``. `Load()` (lines 61-63) rejects `<= 0` with an error naming the variable, failing boot. `.env.example:13` documents `EVENT_RETENTION_DAYS=90`. Live test run: `TestLoad_EventRetentionDaysDefaultsTo90`, `TestLoad_EventRetentionDaysOverride`, `TestLoad_EventRetentionDaysRejectsNonPositive` (subtests 0/-1/-90) all PASS. |
| 2 | History UI and events API return no event older than the retention window, consistently across feed, filters, and pagination | ✓ VERIFIED | `queries/events.sql` `ListEvents` carries `AND created_at >= sqlc.arg('cutoff')::timestamptz` (required `sqlc.arg`, not nullable `sqlc.narg`). `internal/events/service.go` computes the cutoff once per request and passes it to both `ListEvents` and `HasOlderEvents`. Live test run: `TestListEvents_RetentionExcludesAgedOutRows`, `TestListEvents_RetentionBoundaryIsInclusive` (>= confirmed, not >), `TestListEvents_RetentionPagesNeverRepeatAnID` (cross-page disjointness), `TestListEvents_HasOlderEventsRespectsFilters` (artist-scoped) all PASS against real Postgres. Frontend: `has_older_events` wired through `web/app/lib/api.ts` → `web/app/routes/history.tsx`'s `emptyStateCopy`, confirmed by passing `history.test.tsx` suite. |
| 3 | No event row is deleted — an aged-out event is still in the DB and dedup key intact | ✓ VERIFIED | `queries/events.sql` contains zero `DELETE FROM`/`TRUNCATE` statements (`grep -ciE "delete from|truncate"` → 0). `TestListEvents_RetentionExcludesAgedOutRows` asserts a direct `SELECT count(*)` still returns both rows after the aged-out one is hidden from the API. `TestRetention_DetectionStateQueriesStayUnfiltered/dedup_key_intact_(criterion_3)` asserts `ListExternalIDs` still returns the 200-day-old row's external_id. PASS. |
| 4 | An artist whose entire visible history aged out does not fall back into seed mode or re-announce its back catalogue | ✓ VERIFIED | `queries/events.sql` `HasAnyEvent` is byte-identical to its pre-phase form (no cutoff predicate). `TestRetention_DetectionStateQueriesStayUnfiltered/seed_mode_not_reset_(criterion_4)` asserts `HasAnyEvent` returns true for the artist+source of a 200-day-old-only event. Acceptance-criterion regression check documented in 10-01-SUMMARY.md (temporarily adding a retention predicate to `HasAnyEvent` made this subtest fail; reverted). PASS. |
| 5 | A deluxe/tracklist-change baseline recorded before the window still fires a deluxe alert when the release group's tracklist later expands | ✓ VERIFIED | `GroupTrackCountBaseline` is untouched (no cutoff). `TestRetention_DetectionStateQueriesStayUnfiltered/deluxe_baseline_survives_(criterion_5)` asserts `has_baseline == true` and the seeded `track_count` is returned for the 200-day-old event's `release_group_mbid`. PASS. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### Locked Design Decision Compliance

Soft-delete/filter (not hard delete) confirmed: `ListEvents` is the only query in `queries/events.sql` carrying a retention predicate; `ListExternalIDs`, `HasAnyEvent`, `GroupTrackCountBaseline`, and `ListUnnotified` are unmodified. No `DELETE`/`TRUNCATE` statement exists anywhere in the file. This was independently confirmed by running the actual grep/test commands, not merely reading the plan's claims.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | `Config.EventRetentionDays` field + validation | ✓ VERIFIED | Field present, tagged, validated post-`env.Parse`; confirmed by passing tests |
| `.env.example` | `EVENT_RETENTION_DAYS` key | ✓ VERIFIED | Line 13: `EVENT_RETENTION_DAYS=90` |
| `queries/events.sql` | `ListEvents` cutoff param + `HasOlderEvents` query | ✓ VERIFIED | Both present; four detection-state queries confirmed untouched by direct read |
| `internal/db/sqlc/events.sql.go` | Regenerated `ListEventsParams.Cutoff`, `HasOlderEventsParams` | ✓ VERIFIED | `make sqlc-check` exits 0 (clean diff against regenerated output) |
| `internal/events/service.go` | `Service.retentionDays`, widened `NewService`, cutoff computation, `Page.HasOlderEvents` | ✓ VERIFIED | Read directly; cutoff built with `Valid: true` explicit (avoids NULL-comparison DoS) |
| `internal/httpserver/events.go` | `eventsResponse.HasOlderEvents` / `has_older_events` JSON field | ✓ VERIFIED | Present, wired from `page.HasOlderEvents` |
| `web/app/lib/api.ts` | `EventsPage.has_older_events` | ✓ VERIFIED | Non-nullable boolean field present |
| `web/app/routes/history.tsx` | Third empty-state branch, `hasOlderEvents` state | ✓ VERIFIED | `emptyStateCopy()` checks `hasOlderEvents` before `isFiltered`, matching locked UI-SPEC ordering |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `config.Load()` | `cmd/server/main.go` | `cfg.EventRetentionDays` | ✓ WIRED | `main.go:107`: `events.NewService(sqlc.New(pool), cfg.EventRetentionDays)` |
| `events.Service.List` | `sqlc.ListEventsParams.Cutoff` | cutoff computed once, reused for both queries | ✓ WIRED | `service.go:123-147` — same `cutoffParam` passed to `ListEvents` and `HasOlderEvents` |
| `HasOlderEvents SQL` | `history.tsx` empty-state branch | `Page.HasOlderEvents` → `eventsResponse.HasOlderEvents` → `EventsPage.has_older_events` → `hasOlderEvents` state | ✓ WIRED | Full chain read end-to-end; confirmed by passing `TestHandleListEvents_HasOlderEventsSignal` and `history.test.tsx` |
| `recordingQuerier` test double | `sqlc.Querier` interface | explicit `HasOlderEvents` stub method | ✓ WIRED | Present; the load-bearing "limit above max is clamped" subtest at line 331 confirmed passing (would nil-panic if the stub were missing) |

### Behavioral Spot-Checks (live test execution, not SUMMARY claims)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Config default/override/rejection | `go test ./internal/config/... -run "TestLoad_EventRetentionDays" -v` | 3/3 tests, 3/3 subtests PASS | ✓ PASS |
| Retention excludes aged-out rows, rows still in DB | `go test ./internal/httpserver/... -run TestListEvents_RetentionExcludesAgedOutRows` (real Postgres) | PASS | ✓ PASS |
| Boundary inclusivity (>=, not >) | `go test ./internal/httpserver/... -run TestListEvents_RetentionBoundaryIsInclusive` | PASS | ✓ PASS |
| Cross-page disjointness | `go test ./internal/httpserver/... -run TestListEvents_RetentionPagesNeverRepeatAnID` | PASS | ✓ PASS |
| Detection-state queries stay unfiltered (SC 3-5) | `go test ./internal/httpserver/... -run TestRetention_DetectionStateQueriesStayUnfiltered -v` | 5/5 subtests PASS | ✓ PASS |
| `has_older_events` signal correctness | `go test ./internal/httpserver/... -run TestHandleListEvents_HasOlderEventsSignal -v` | 3/3 subtests PASS | ✓ PASS |
| `has_older_events` respects artist_id filter | `go test ./internal/httpserver/... -run TestListEvents_HasOlderEventsRespectsFilters` | PASS | ✓ PASS |
| Frontend empty-state branches + priority | `pnpm run test -- history.test.tsx api.test.ts` | 9 files / 43 tests PASS | ✓ PASS |
| Full regression: config + events + httpserver | `go test ./internal/config/... ./internal/events/... ./internal/httpserver/... -count=1 -p 1` | all PASS, no regressions | ✓ PASS |
| `go build ./...` / `go vet ./...` | — | clean | ✓ PASS |
| `make sqlc-check` | — | exit 0 (clean diff) | ✓ PASS |
| `tsc --noEmit` | — | exit 0 | ✓ PASS |

All checks above were executed live against a running Postgres 16 container (`drop-tracker-postgres-1`, port 5432) during this verification — not read from SUMMARY.md claims.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| DATA-01 | 10-01 | Event-retention window configurable via env var, defaulting to 90 days | ✓ SATISFIED | `Config.EventRetentionDays`, `.env.example`, all three config tests PASS |
| DATA-02 | 10-01, 10-02 | History/API queries exclude aged-out events; underlying rows and detection state untouched | ✓ SATISFIED | `ListEvents` cutoff predicate + `HasOlderEvents` query + `TestRetention_DetectionStateQueriesStayUnfiltered` (5 subtests) all PASS |

**Note:** `.planning/REQUIREMENTS.md` traceability table (lines 185-186) still marks DATA-01 and DATA-02 as "Pending" — this is a bookkeeping status field, not an implementation gap; codebase evidence above confirms both requirements are functionally satisfied. This status field is typically updated during the ship/complete-milestone step, not during phase verification, so it is noted here informationally rather than as a gap.

### Anti-Patterns Found

None. Scanned all phase-modified files (`internal/config/config.go`, `internal/events/service.go`, `internal/httpserver/events.go`, `queries/events.sql`, `web/app/routes/history.tsx`, `web/app/lib/api.ts`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers, empty-implementation stubs, and hardcoded-empty-data patterns. Zero matches. `grep -rc "dangerouslySetInnerHTML" web/app/` returns 0 (Phase 6 invariant preserved).

### Human Verification Required

None. All must-haves are backend/frontend logic verifiable through automated tests, which were executed live during this verification (not merely read from SUMMARY.md).

### Gaps Summary

No gaps found. All five roadmap success criteria are independently confirmed against the live codebase and a real Postgres database:

1. Env var configurability + fail-fast validation — confirmed by passing config tests.
2. Consistent retention filtering across feed/filters/pagination — confirmed by passing HTTP-layer tests including cross-page disjointness.
3-5. Dedup key, seed-mode signal, and deluxe baseline all survive retention filtering — confirmed by the single most load-bearing test in the phase (`TestRetention_DetectionStateQueriesStayUnfiltered`), which pairs each positive assertion with a contrast subtest proving the retention filter is actually wired (not merely absent by omission).

The soft-delete/filter design decision is honored throughout: zero `DELETE`/`TRUNCATE` statements exist in the touched SQL file, and the four detection-state queries remain byte-identical to their pre-phase form.

---

_Verified: 2026-08-13T22:43:20Z_
_Verifier: Claude (gsd-verifier)_
