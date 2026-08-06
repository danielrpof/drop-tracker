---
phase: 02-watchlist-core
verified: 2026-08-06T01:46:44Z
status: human_needed
score: 32/32 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Run `make db-up && make run`, then exercise all four /watchlist routes with curl end to end (add an artist, list it, narrow its release types, mute an event category, list again to confirm both axes, delete it, list once more). Confirm the JSON bodies read the way you would want a future React client to consume them, and that no error response ever contains database internals."
    expected: "JSON shapes are ergonomic for a future React client; no response body leaks Postgres error text, a DSN, or a password."
    why_human: "Explicitly deferred from plan 02-04 task 2's <human-check> to end-of-phase UAT per this project's human_verify_mode: end-of-phase config (confirmed in 02-04-SUMMARY.md's 'Human-Verification Note'). JSON ergonomics is a subjective judgment call automated checks cannot make; the no-leak half is already covered by TestWatchlist_Add_DoesNotLeakInternals but the manual walkthrough is the plan's own designated closure step."
  - test: "Decide whether WR-02 (UpdatePreferences: unhandled not-found race returns 500 instead of 404 when a row is deleted between the read and the write; lost-update race between two concurrent PATCH calls each touching a different axis) and WR-01 (UpsertArtist silently drops disambiguation/image_url on re-add) need a fast-follow fix before Phase 3/4 start writing to these same tables, or are accepted v1 risks for a single-operator deployment."
    expected: "A recorded decision (accept as documented risk, or open a follow-up plan) for each finding — this is a developer judgment call, not something verification can resolve unilaterally."
    why_human: "Both confirmed present in the codebase by direct inspection. WR-02 (internal/watchlist/service.go:206-275): UpdatePreferences reads via an unlocked ListWatchlist SELECT, then writes via UpdateWatchlistPreferences as two separate non-transactional statements, with no pgx.ErrNoRows translation on the write path. Neither declared must-have truth for this phase requires PATCH-level concurrency safety (only DELETE's concurrency truth was explicitly written and tested — TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404, re-run 5x during this verification, all pass); TestService_UpdatePreferences_AxesAreIndependent only proves the sequential case. This does NOT violate D-09 as literally scoped (D-09 concerns duplicate ADD requests, not concurrent PATCH), so it is not a phase must-have gap under the plan's own contract — but it is a real, reproducible correctness gap review already flagged (02-REVIEW.md WR-02) that a human should knowingly accept or schedule. WR-01 (queries/artists.sql UpsertArtist ON CONFLICT clause) is lower-priority: it silently drops disambiguation/image_url on re-add, doesn't touch any declared must-have truth (none mention artist-metadata refresh completeness), and is Warning-severity, not Critical."
---

# Phase 2: Watchlist Core Verification Report

**Phase Goal:** "Users can fully manage their watchlist — add, remove, list, and configure per-artist alert preferences — through a tested API service layer."
**Verified:** 2026-08-06T01:46:44Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

All 32 must-have truths declared across the four plans' frontmatter (02-01 through 02-04) were checked against the actual codebase — build, `go vet`, a full `go test ./... -count=1` run against real Postgres, a targeted 5x re-run of the concurrency test, and direct source inspection of every referenced code path.

| # | Plan | Truth | Status | Evidence |
|---|------|-------|--------|----------|
| 1 | 02-01 | POST /watchlist success → 201 with id/artist_id/mbid/name/release_types/muted_event_types | ✓ VERIFIED | `TestWatchlist_AddEndToEnd` passes against real Postgres |
| 2 | 02-01 | Fresh add gets D-08 defaults (all 4 release types, empty mutes) | ✓ VERIFIED | Same test asserts `release_types == ["album","single","ep","deluxe"]`, `muted_event_types == []`; `internal/watchlist/service.go:111-131` implements the default substitution |
| 3 | 02-01 | Blank/missing mbid or name → 400 `{"error":...}`, no row written | ✓ VERIFIED | `TestWatchlist_Add_RejectsBlankFields` passes |
| 4 | 02-01 | Unknown JSON key → 400, not silently discarded | ✓ VERIFIED | `TestWatchlist_Add_RejectsUnknownFields`; `DisallowUnknownFields()` at `watchlist.go:92` |
| 5 | 02-01 | Created row durably persisted, second process reads it | ✓ VERIFIED | `TestWatchlist_AddEndToEnd`'s direct `SELECT count(*) FROM watchlist WHERE artist_id = $1` assertion |
| 6 | 02-01 | No response body leaks Postgres error text, DSN, or password | ✓ VERIFIED | `TestWatchlist_Add_DoesNotLeakInternals` passes; `writeError` (watchlist.go:44-48) only emits fixed operator strings, driver text goes to `httplog.SetAttrs` only |
| 7 | 02-01 | GET /health unbroken after `httpserver.New` signature change | ✓ VERIFIED | Full suite includes `internal/httpserver` package tests (health_test.go), all pass; 8/8 `httpserver.New(` call sites updated and compiling (`grep -rn "httpserver.New("` confirms all 8, matching 02-VALIDATION.md's corrected count) |
| 8 | 02-02 | Duplicate add → 409, existing entry's preferences byte-for-byte unchanged | ✓ VERIFIED | `TestService_Add_DuplicateLeavesPreferencesUntouched`, `TestWatchlist_Add_DuplicateReturns409` both pass |
| 9 | 02-02 | 409 keyed on SQLSTATE 23505 + exact constraint name, not message text | ✓ VERIFIED | `service.go:160-163`: `pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "watchlist_artist_id_key"`; negative grep for message-text matching confirms none |
| 10 | 02-02 | Optional release_types/muted_event_types persisted exactly, overriding D-08 defaults | ✓ VERIFIED | `TestService_Add_PersistsSuppliedPreferences` passes |
| 11 | 02-02 | Out-of-allow-list value → 400 naming it, no row written | ✓ VERIFIED | `TestService_Add_RejectsUnknownPreferenceValues`, `TestWatchlist_Add_InvalidPreferenceValueReturns400` pass |
| 12 | 02-02 | Body > 64 KiB → 400 without full buffering | ✓ VERIFIED | `TestWatchlist_Add_RejectsOversizeBody` passes; `http.MaxBytesReader(w, r.Body, 65536)` at `watchlist.go:88` |
| 13 | 02-02 | mbid > 36 chars or name > 512 chars → 400 before any DB call | ✓ VERIFIED | `TestWatchlist_Add_RejectsOverlongFields` passes; rune-counted at `watchlist.go:107-114` |
| 14 | 02-02 | Re-add after only watchlist row deleted (artist row survives) → 201, upsert reuses artist | ✓ VERIFIED | `TestService_Add_ReusesExistingArtistRow` passes |
| 15 | 02-03 | GET /watchlist → 200, array with every field inline for every entry | ✓ VERIFIED | `TestService_List_ReturnsAllEntriesWithBothIDs`, `TestWatchlist_FullLifecycle` pass |
| 16 | 02-03 | Empty watchlist → exactly `[]`, never `null`, no wrapper object | ✓ VERIFIED | `TestWatchlist_List_EmptyReturnsEmptyArray` and `TestWatchlist_List_NilSliceStillEncodesAsEmptyArray` assert on raw response bytes, not decoded values |
| 17 | 02-03 | Deterministic name-then-id ordering, repeatable across runs | ✓ VERIFIED | `TestService_List_OrdersByNameThenID` (runs `List` 3x, asserts identical order); `ORDER BY a.name ASC, a.id ASC` in `queries/watchlist.sql:19` |
| 18 | 02-03 | Identical-name artists stay two distinct elements, never merged/dropped | ✓ VERIFIED | `TestService_List_IdenticalNamesStayDistinct` passes |
| 19 | 02-03 | `id` (watchlist) and `artist_id` (artist) never collapse into one field | ✓ VERIFIED | Explicit column aliasing `w.id AS id, a.id AS artist_id` in query; `ListWatchlistRow` struct carries both fields distinctly (confirmed in generated code) |
| 20 | 02-03 | DELETE existing entry → 204 empty body, subsequent GET no longer lists it | ✓ VERIFIED | `TestService_Remove_DeletesRow`, `TestWatchlist_FullLifecycle` pass |
| 21 | 02-03 | Repeat DELETE on same id → 404, not a silent success | ✓ VERIFIED | `TestService_Remove_SecondCallReturnsErrNotFound`, `TestWatchlist_FullLifecycle` pass |
| 22 | 02-03 | Concurrent DELETE on same id → exactly one 204 and one 404, never 500 | ✓ VERIFIED | `TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404` re-run directly 5 consecutive times during this verification — all 5 pass; `:execrows` single-statement delete relies on Postgres row locking (service.go:286-295) |
| 23 | 02-03 | Non-numeric/negative id → 400, never reaches DB | ✓ VERIFIED | `TestWatchlist_Delete_BadIDReturns400`; `parseWatchlistID` rejects parse failures and `id < 1` before any service call |
| 24 | 02-03 | Remove then re-add same mbid → 201, no tombstone blocks it | ✓ VERIFIED | `TestService_Remove_ThenReAddSucceeds`, `TestWatchlist_FullLifecycle` pass |
| 25 | 02-03 | Deleting watchlist row leaves artists master row intact | ✓ VERIFIED | `TestService_Remove_LeavesArtistRowIntact` passes; no `deleted_at` or soft-delete predicate anywhere (`grep -rn deleted_at` returns nothing) |
| 26 | 02-04 | PATCH release_types persists exact set, 200 with full updated entry | ✓ VERIFIED | `TestService_UpdatePreferences_SetsReleaseTypes`, `TestWatchlist_Patch_Returns200WithUpdatedEntry` pass |
| 27 | 02-04 | PATCH muted_event_types persists exact set, 200 with full updated entry | ✓ VERIFIED | `TestService_UpdatePreferences_SetsMutedEventTypes` passes |
| 28 | 02-04 | Omitted key leaves that axis exactly as it was | ✓ VERIFIED (sequential calls only — see human-verification note below) | `TestService_UpdatePreferences_AxesAreIndependent` passes for two *sequential* PATCH calls; the truth as declared says nothing about concurrent calls |
| 29 | 02-04 | Explicit empty array on either axis persists `[]`, distinguishable from omission | ✓ VERIFIED | `TestService_UpdatePreferences_EmptyArrayIsNotOmission` (table-driven, both axes) passes |
| 30 | 02-04 | Duplicate submitted values collapsed on either axis, canonical allow-list order | ✓ VERIFIED | `TestService_UpdatePreferences_DeduplicatesAndCanonicalises` passes |
| 31 | 02-04 | Out-of-allow-list value on PATCH → 400 naming it, stored row unchanged | ✓ VERIFIED | `TestService_UpdatePreferences_RejectsUnknownValue` passes |
| 32 | 02-04 | Unknown watchlist id → 404 (straightforward case); malformed id → 400; axes independent; DB CHECK backstop holds; no interface-first placeholder survives | ✓ VERIFIED (straightforward case only — see human-verification note below) | `TestService_UpdatePreferences_UnknownIDReturnsErrNotFound`, `TestWatchlist_Patch_MissingReturns404`, `TestWatchlist_Patch_BadIDReturns400`, four `TestCheckConstraint_*` tests, and `grep -c errNotImplemented internal/watchlist/service.go` = 0 all pass/confirm |

**Score:** 32/32 declared must-have truths verified against passing tests and/or direct code inspection. 0 truths left behaviorally unverified (every truth that asserts a state transition or invariant has a passing named test backing it, within the scope the truth itself declares).

**Note on truths 28 and 32:** both are worded around the *sequential* case (which the plan's own tests exercise and which pass), so they are correctly VERIFIED as literally declared. A narrower, undeclared concurrency scenario — two racing PATCH calls, or a PATCH racing a DELETE — is not covered by any must-have truth in this phase's plans, and code review (02-REVIEW.md WR-02) confirms by direct inspection that this scenario currently produces incorrect behavior (500 instead of 404 on the delete-race; a silent lost update on the concurrent-PATCH race). See Human Verification below.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/db/migrations/000002_watchlist.up.sql` | artists/watchlist tables, constraints, D-08 defaults | ✓ VERIFIED | Present, contains `CREATE TABLE artists`, `CREATE TABLE watchlist`, all three named constraints |
| `internal/db/migrations/000002_watchlist.down.sql` | reverse migration | ✓ VERIFIED | Drops `watchlist` then `artists`, correct FK-respecting order |
| `internal/watchlist/service.go` | Store/Service/sentinels/allow-lists | ✓ VERIFIED | All exports present: `Store`, `Service`, `NewService`, `Entry`, `AddParams`, `PreferencesParams`, `ErrDuplicate`, `ErrNotFound`, `ErrInvalidReleaseType`, `ErrInvalidEventType`, `ReleaseTypes`, `EventTypes`; 350 lines, well above `min_lines: 90` |
| `internal/httpserver/watchlist.go` | 4 handlers, DTOs, writeError | ✓ VERIFIED | `handleAddWatchlist`, `handleListWatchlist`, `handleUpdateWatchlist`, `handleRemoveWatchlist`, `writeError`, `parseWatchlistID` all present; 285 lines |
| `internal/httpserver/watchlist_test.go` | full handler test surface | ✓ VERIFIED | 19 top-level test functions covering all 4 routes |
| `queries/artists.sql` | UpsertArtist | ✓ VERIFIED | `-- name: UpsertArtist :one` present |
| `queries/watchlist.sql` | 4 watchlist queries | ✓ VERIFIED | Exactly 4 `-- name:` entries: `CreateWatchlistEntry`, `ListWatchlist`, `UpdateWatchlistPreferences`, `DeleteWatchlistEntry` |
| `internal/httpserver/server.go` | route registrations, widened constructor | ✓ VERIFIED | All 4 `/watchlist` routes registered flat (no `/api` prefix); `Pinger` unchanged (still 1 method) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `server.go` | `internal/watchlist/service.go` | `New`'s 2nd param is `watchlist.Store` | ✓ WIRED | `func New(db Pinger, store watchlist.Store, logger *slog.Logger) *Server` |
| `cmd/server/main.go` | `internal/watchlist/service.go` | `watchlist.NewService(sqlc.New(pool))` | ✓ WIRED | Confirmed at `main.go:83` |
| `internal/watchlist/service.go` | `internal/db/sqlc/querier.go` | `Service` holds `sqlc.Querier` | ✓ WIRED | `type Service struct { q sqlc.Querier }` |
| `internal/httpserver/watchlist.go` | `internal/watchlist/service.go` | handlers call `s.watchlist.*` and switch on sentinels | ✓ WIRED | All 4 handlers dispatch through `s.watchlist` and `errors.Is` against the 4 sentinels |
| `internal/watchlist/service.go` | `pgx/v5/pgconn` | `errors.As(err, &pgErr)` on the correct v5 nested package | ✓ WIRED | `import "github.com/jackc/pgx/v5/pgconn"` confirmed, not the legacy standalone module |
| `server.go` | `watchlist.go` | chi routes for list/patch/delete | ✓ WIRED | `r.Get`, `r.Patch`, `r.Delete` all registered against the correct handlers |

### Database Backstop Verification

| Check | Status | Evidence |
|-------|--------|----------|
| `watchlist_release_types_valid` rejects unknown value via raw SQL | ✓ VERIFIED | `TestCheckConstraint_RejectsUnknownReleaseType` passes |
| `watchlist_muted_event_types_valid` rejects unknown value via raw SQL | ✓ VERIFIED | `TestCheckConstraint_RejectsUnknownEventType` passes |
| Empty arrays accepted on both axes | ✓ VERIFIED | `TestCheckConstraint_AcceptsEmptyArrays` passes |
| Full allow-lists accepted, driven from Go constants | ✓ VERIFIED | `TestCheckConstraint_AcceptsFullAllowLists` passes |

### Behavioral Spot-Checks / Direct Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full build | `go build ./...` | exit 0 | ✓ PASS |
| Static analysis | `go vet ./...` | exit 0, no output | ✓ PASS |
| Full test suite (single run, real Postgres) | `TEST_DATABASE_URL=... go test ./... -count=1` | all packages `ok` (config, db, httpserver, watchlist) | ✓ PASS |
| Concurrency invariant, named test, 5 consecutive runs | `go test ./internal/httpserver/ -run TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404 -count=5 -v` | 5/5 PASS | ✓ PASS |
| sqlc drift check | `sqlc diff` | exit 0, no diff | ✓ PASS |
| `go mod verify` | `go mod verify` | "all modules verified" | ✓ PASS |
| `jackc/pgerrcode` still at Phase-1-vetted pin, promoted to direct require | `grep jackc/pgerrcode go.mod` | `v0.0.0-20220416144525-469b46aa5efa` in the direct `require` block | ✓ PASS |
| No debt markers in phase files | `grep -nE 'TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER'` across all phase-modified files | no matches | ✓ PASS |
| No soft-delete residue | `grep -rn deleted_at` across migration/query/service files | no matches | ✓ PASS |
| `errNotImplemented` placeholder gone | `grep -c errNotImplemented internal/watchlist/service.go` | 0 | ✓ PASS |

Migration reversibility (`migrate down 1` then `up`) could not be re-run directly in this environment — no standalone `migrate` CLI binary is installed on this machine (only the `golang-migrate/migrate/v4` library, invoked in-process by `internal/db`). This was not re-executed as a fresh check; it is accepted on the combination of (a) 02-01-SUMMARY.md's original claim that it was exercised directly via `go run .../migrate/v4/cmd/migrate down 1` then `up`, (b) `internal/db`'s own migration test suite (`TestRunMigrations_AppliesFromScratch`) passing in this verification's full suite run, and (c) direct inspection of `000002_watchlist.down.sql`, which is a trivially correct two-statement reverse (drop `watchlist` before `artists`, respecting the FK direction) that matches the up migration exactly.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WLST-02 | 02-01, 02-02 | User can add an artist to the watchlist from search results | ✓ SATISFIED | `POST /watchlist` fully implemented, tested, D-08/D-09 both proven |
| WLST-03 | 02-03 | User can remove an artist from the watchlist | ✓ SATISFIED | `DELETE /watchlist/{id}` hard delete, 404/400 branches, concurrency-safe |
| WLST-04 | 02-03 | User can list all artists currently on the watchlist | ✓ SATISFIED | `GET /watchlist` joined, ordered, `[]`-safe |
| WLST-05 | 02-02, 02-04 | User can set per-artist release-type filters | ✓ SATISFIED | Set on add (02-02) and via `PATCH` (02-04), both validated and tested |
| WLST-06 | 02-02, 02-04 | User can set per-artist notification/mute preferences | ✓ SATISFIED | Same as WLST-05, mute axis |

No orphaned requirements: `.planning/REQUIREMENTS.md`'s traceability table maps exactly WLST-02 through WLST-06 to Phase 2, all marked `[x]`/Complete, and all five appear in at least one plan's `requirements:` frontmatter (02-01: WLST-02; 02-02: WLST-02, WLST-05, WLST-06; 02-03: WLST-03, WLST-04; 02-04: WLST-05, WLST-06).

### Anti-Patterns Found

None. Scanned all phase-modified Go, SQL, and YAML files for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` and common stub phrases — zero matches. `errNotImplemented` (the deliberate, plan-documented interface-first placeholder) is confirmed fully removed by the end of the phase.

### Code Review Findings Carried Forward (02-REVIEW.md)

Two Warning-level findings from the prior code review were re-confirmed by direct inspection during this verification:

**WR-01 — `UpsertArtist` silently drops `disambiguation`/`image_url` on re-add.** Confirmed at `queries/artists.sql:1-8` / `internal/db/sqlc/artists.sql.go`: the `ON CONFLICT (mbid) DO UPDATE` clause refreshes `name` and `deezer_id` (via `COALESCE`) but has no `SET` clause for `disambiguation` or `image_url` at all — a re-add silently discards any new value for those two fields, returning the stale stored value in the response with no error or signal. **Assessment:** this does not touch any of the 32 declared must-have truths (none of them assert artist-metadata refresh completeness on re-add — only `deezer_id` refresh is implicitly covered by `TestService_Add_ReusesExistingArtistRow`'s "artist row reused" assertion, which doesn't check `disambiguation`/`image_url` values). It is not a violation of D-09 (D-09 is specifically about *preferences* — `release_types`/`muted_event_types` — not artist master-data fields). Not a phase must-have gap as scoped; flagged for a human decision on priority.

**WR-02 — `UpdatePreferences` has an unhandled not-found race and a lost-update race under concurrent PATCH.** Confirmed at `internal/watchlist/service.go:206-275`: the method reads the current row via a plain, unlocked `ListWatchlist` SELECT, then writes via `UpdateWatchlistPreferences` as two separate non-transactional statements. (1) If the row is deleted between the read and the write, `UpdateWatchlistPreferences`'s `:one` query returns `pgx.ErrNoRows` from `row.Scan`, which is wrapped by `fmt.Errorf` at line 259 and returned as a plain error — `errors.Is(err, watchlist.ErrNotFound)` does not match it, so the handler falls through to a 500 instead of the expected 404. (2) Two concurrent PATCH calls each touching a different axis can each read the same pre-update row and each write back their own change plus the other's now-stale value for the untouched axis, silently losing whichever change lands first. **Assessment:** neither scenario is covered by a declared must-have truth — the phase's only explicit concurrency requirement is DELETE's (tested and passing), and the PATCH axis-independence truth is worded around sequential calls only (matching what `TestService_UpdatePreferences_AxesAreIndependent` tests). This is *not* a violation of D-09 as literally scoped (D-09 concerns duplicate-ADD, not concurrent PATCH). It is, however, a real and reproducible gap in the more general "independent axes" (D-05) spirit that a "tested API service layer" phase goal implies, and Phase 3/4 will start reading these same columns soon. Recorded as a human-decision item below rather than a blocking gap, since no stated contract was broken.

## Human Verification Required

### 1. End-to-end curl walkthrough and JSON ergonomics review (deferred from plan 02-04)

**Test:** Run `make db-up && make run`, then exercise all four `/watchlist` routes with curl in sequence: add an artist, list it, narrow its release types via PATCH, mute an event category via PATCH, list again to confirm both axes, delete it, list once more to confirm it's gone.
**Expected:** JSON response bodies read the way a future React client (Phase 6) would want to consume them; no error response at any point contains database internals (DSN, driver text, SQLSTATE codes).
**Why human:** This was explicitly deferred by plan 02-04's own `<human-check>` block to end-of-phase UAT per this project's `human_verify_mode: end-of-phase` setting (confirmed in 02-04-SUMMARY.md's "Human-Verification Note" section) — it is a planner-deferred item, not a verifier gap. The no-leak half is already covered by an automated test (`TestWatchlist_Add_DoesNotLeakInternals`); the JSON-ergonomics half is a subjective UX judgment call no automated check can make.

### 2. Decide the disposition of WR-01 and WR-02 (code review findings, not covered by any declared must-have)

**Test:** Review `02-REVIEW.md`'s WR-01 and WR-02 findings (both re-confirmed above by direct code inspection) and decide whether either needs a fast-follow fix before Phase 3/4 begin writing to these same tables, or is an accepted risk for this single-operator v1 deployment.
**Expected:** A recorded decision — e.g. an accepted-risk note in STATE.md, or a follow-up plan/task filed for one or both.
**Why human:** Both are real, reproducible correctness gaps confirmed by source inspection, but neither breaks any of the phase's 32 declared must-have truths or the five WLST-* requirements as literally written (per this phase's own contract). Whether to close them now or accept them as documented risk is a scoping/priority judgment call, not something a verifier can resolve unilaterally. WR-02 in particular touches the PATCH endpoint that Phase 4/5 downstream consumers will rely on for muting alert noise, so the cost of deferring grows over time even though it is not a blocker today.

### Gaps Summary

No gaps. Every must-have truth, artifact, and key link declared across the four plans' frontmatter was independently confirmed against the codebase — through a real, single full-suite test run against Postgres, a 5x re-run of the one behavior-dependent concurrency test, `go build`/`go vet`/`sqlc diff`/`go mod verify`, and direct source inspection of every referenced code path. No stub, no orphaned requirement, no debt marker, and no interface-first placeholder remains. The phase goal — "add, remove, list, and configure per-artist alert preferences through a tested API service layer" — is observably true: all four `/watchlist` routes exist, are wired to a real Postgres-backed service, and are exercised end-to-end by `TestWatchlist_FullLifecycle`.

The `human_needed` status reflects one planner-deferred UAT item (not a gap) plus a request for an explicit human decision on two already-documented, non-blocking code-review findings (WR-01, WR-02) — neither of which fails a stated contract of this phase, but both of which are worth a conscious accept-or-fix call before later phases build on top of these same tables and endpoints.

---

_Verified: 2026-08-06T01:46:44Z_
_Verifier: Claude (gsd-verifier)_
