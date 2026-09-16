---
phase: 02-watchlist-core
plan: 02
subsystem: watchlist-api
tags: [postgres, pgx, watchlist, validation, error-handling]

requires:
  - phase: 02-watchlist-core (plan 01)
    provides: internal/watchlist.Store/Service, POST /watchlist tracer, artists/watchlist schema
provides:
  - "Real SQLSTATE 23505 -> watchlist.ErrDuplicate translation in Service.Add (409, D-09)"
  - "normalizeSet -- shared allow-list validation/canonicalisation for both preference axes"
  - "Optional release_types/muted_event_types on POST /watchlist (D-11)"
  - "64 KiB request-body ceiling and 36/512-rune mbid/name caps on the add path"
affects: [02-03, 02-04]

actuals:
  tokens: 6879
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "pgx v5 error translation: errors.As(err, *pgconn.PgError) + pgerrcode.UniqueViolation + exact ConstraintName -- never error-message string matching"
    - "Handler-side fail-fast membership check against the same exported allow-list a service-layer normalizeSet re-validates as backstop -- two call sites, one shared list, not scattered switches"
    - "Pointer-to-slice request DTO fields (*[]string) to distinguish an absent JSON key from an explicit empty array"

key-files:
  created:
    - internal/watchlist/service_test.go
    - internal/watchlist/normalize_test.go
  modified:
    - internal/watchlist/service.go
    - internal/httpserver/watchlist.go
    - internal/httpserver/watchlist_test.go
    - go.mod

key-decisions:
  - "normalizeSet lives in service.go (unexported); its unit test (TestNormalizeSet) lives in a separate internal-package file (normalize_test.go, package watchlist) since Go test files cannot call an unexported function from an external _test package -- service_test.go stays package watchlist_test for the real-Postgres tests, consistent with plan 02-01's fixture pattern"
  - "Handler performs its own fail-fast membership check against watchlist.ReleaseTypes/EventTypes before ever calling Store.Add, so an invalid preference value never reaches the store (addFunc not called) -- Service.Add's normalizeSet remains the non-bypassable backstop for any caller that reaches the service directly, so the allow-list itself (the exported var) is still the single source of truth, only the membership check is duplicated at two call sites"
  - "normalizeSet silently collapses duplicate submissions rather than rejecting them (a preference array is semantically a set) and returns values in canonical allow-list order rather than submission order (byte-stable response bodies) -- both were left unspecified by 02-CONTEXT.md and 02-RESEARCH.md and are recorded here per the plan's own instruction"

patterns-established:
  - "pgx v5 error-code translation: import github.com/jackc/pgx/v5/pgconn (never the legacy github.com/jackc/pgconn), check SQLSTATE code AND constraint name together"

requirements-completed: [WLST-02, WLST-05, WLST-06]

coverage:
  - id: D1
    description: "Duplicate-add returns 409 without disturbing stored preferences (D-09)"
    requirement: "WLST-02"
    verification:
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_Add_DuplicateReturnsErrDuplicate"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_Add_DuplicateLeavesPreferencesUntouched"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_Add_ReusesExistingArtistRow"
        status: pass
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Add_DuplicateReturns409"
        status: pass
    human_judgment: false
  - id: D2
    description: "Optional initial release_types/muted_event_types accepted, validated against the allow-list, normalised to canonical order, and persisted exactly (D-11)"
    requirement: "WLST-05"
    verification:
      - kind: unit
        ref: "internal/watchlist/normalize_test.go#TestNormalizeSet"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_Add_DefaultsWhenPreferencesOmitted"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_Add_DBDefaultsMatchGoAllowList"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_Add_PersistsSuppliedPreferences"
        status: pass
    human_judgment: false
  - id: D3
    description: "Out-of-allow-list release/event type values rejected with 400 naming the offending value, before any write"
    requirement: "WLST-06"
    verification:
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestService_Add_RejectsUnknownPreferenceValues"
        status: pass
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Add_InvalidPreferenceValueReturns400"
        status: pass
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestCheckConstraintRejectsUnknownValue"
        status: pass
    human_judgment: false
  - id: D4
    description: "Oversized bodies and overlong mbid/name rejected with 400 before any database call"
    requirement: "WLST-02"
    verification:
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Add_RejectsOversizeBody"
        status: pass
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Add_RejectsOverlongFields"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-08-06
status: complete
---

# Phase 2 Plan 02: Duplicate-Add 409, Optional Preferences, Request Bounds Summary

Completed the add slice: `Service.Add` now translates Postgres SQLSTATE 23505 on `watchlist_artist_id_key` into a 409 that never disturbs stored preferences, `POST /watchlist` accepts and validates optional `release_types`/`muted_event_types` against a shared allow-list, and the request surface is bounded by a 64 KiB body ceiling and 36/512-rune field caps.

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-08-06
- **Tasks:** 2
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments
- `Service.Add` detects a duplicate watchlist entry by matching `pgerrcode.UniqueViolation` AND the exact constraint name `watchlist_artist_id_key` (never error-message text) and returns `ErrDuplicate`, which the handler already translated to 409 in plan 02-01
- Proved mechanically (not just by inspection) that a rejected duplicate leaves the existing entry's `release_types`/`muted_event_types` byte-for-byte unchanged, and that re-adding after only the `watchlist` row was deleted reuses the existing `artists` row rather than creating a second one
- `normalizeSet` is the single validation/de-duplication/canonical-ordering routine for both preference axes, used by `Service.Add` before any database write
- `POST /watchlist` now accepts optional `release_types`/`muted_event_types` as `*[]string` (distinguishing an absent key from an explicit empty array), rejects out-of-allow-list values with 400 naming the value at the handler layer (before ever calling the store) and again at the service layer (`normalizeSet`, the non-bypassable backstop)
- Request body bounded to 64 KiB via `http.MaxBytesReader`; `mbid`/`name` capped at 36/512 runes (not bytes) before any database call
- A DB-defaults-parity test (`TestService_Add_DBDefaultsMatchGoAllowList`) and a raw-SQL CHECK-constraint test (`TestCheckConstraintRejectsUnknownValue`) prove the Go allow-list constants and the migration's column defaults/CHECK constraints have not drifted apart

## Task Commits

Each task followed the RED/GREEN TDD cycle with separate commits:

1. **Task 1: Duplicate-add returns 409 without disturbing stored preferences**
   - `f65a135` (test) - failing tests for duplicate-409 handling
   - `0c19af0` (feat) - SQLSTATE-based ErrDuplicate translation
2. **Task 2: Optional initial preferences, allow-list validation, request bounds**
   - `b9db03b` (test) - failing tests for normalizeSet, preferences, and request bounds
   - `75cd8b0` (feat) - normalizeSet, optional preferences DTO fields, body/field bounds

## Files Created/Modified
- `internal/watchlist/service.go` - `normalizeSet`, SQLSTATE 23505 -> `ErrDuplicate` translation, `Add` validates both preference axes before any database call
- `internal/watchlist/service_test.go` - first real-Postgres service-layer test file: duplicate detection, preferences-untouched, artist-row reuse, D-08 defaults, DB/Go allow-list parity, supplied-preferences persistence, unknown-value rejection, raw CHECK-constraint proof
- `internal/watchlist/normalize_test.go` - `TestNormalizeSet` table-driven unit test (package `watchlist`, since `normalizeSet` is unexported)
- `internal/httpserver/watchlist.go` - `MaxBytesReader` body ceiling, rune-counted `mbid`/`name` caps, `ReleaseTypes`/`MutedEventTypes` `*[]string` DTO fields, fail-fast preference-value check, 409/400 branches for the new sentinels
- `internal/httpserver/watchlist_test.go` - `TestWatchlist_Add_DuplicateReturns409`, `TestWatchlist_Add_InvalidPreferenceValueReturns400`, `TestWatchlist_Add_RejectsOversizeBody`, `TestWatchlist_Add_RejectsOverlongFields`
- `go.mod` - `jackc/pgerrcode` promoted from indirect to direct at the unchanged pin (`go mod tidy`, no version change)

## Decisions Made
- `normalizeSet`'s unit test needed its own internal-package file (`normalize_test.go`) because it tests an unexported function -- `service_test.go` stays in the external `watchlist_test` package for the real-Postgres tests, matching plan 02-01's established fixture pattern
- The handler validates preference values against `watchlist.ReleaseTypes`/`EventTypes` directly (fail-fast, before calling `Store.Add`) in addition to `Service.Add`'s `normalizeSet` backstop -- this satisfies the plan's explicit test requirement that `addFunc` is never called for an invalid preference value, while keeping the allow-list itself (the exported var) as the single source of truth even though the membership check runs at two call sites
- `normalizeSet` silently de-duplicates rather than rejecting duplicate submissions, and returns canonical allow-list order rather than submission order -- both choices were left unspecified by 02-CONTEXT.md/02-RESEARCH.md and are recorded per the plan's explicit instruction to do so

## Deviations from Plan

None functionally -- both tasks were built to the plan's `<action>` and `<behavior>` specs exactly, including the RED/GREEN TDD commit split. One documentation-level note:

**Acceptance-criteria grep pattern mismatch (non-functional).** The plan's literal acceptance check `grep -q 'ReleaseTypes \*\[\]string' internal/httpserver/watchlist.go` assumes a single space between the field name and its type. gofmt aligns struct field declarations with multiple spaces (consistent with the file's pre-existing style, and `gofmt -l` confirms `watchlist.go` needed no reformatting), so the literal single-space grep does not match even though the field (`ReleaseTypes *[]string` with gofmt alignment) is present and correctly typed. Verified with `grep -qE 'ReleaseTypes +\*\[\]string'` instead. Not a code defect -- flagging so a future grep-based check in this phase uses `+` for whitespace.

## Issues Encountered
A stale `server.exe` process (unrelated to this session, left running on port 8099 from an earlier PID) shadowed the freshly built binary during manual curl verification, making `POST /watchlist` return 404 as if the route were unregistered. Diagnosed via `netstat`/`tasklist`, killed the stale process, and re-verified against a freshly built binary on a clean port -- confirmed 201 then 409 on repeat POST, and 400 with the offending value named for an unknown `release_types` entry.

## Next Phase Readiness
- `internal/watchlist.Service.Add` is now fully hardened: duplicate detection, optional preferences, allow-list validation, and request bounds are all real and tested against Postgres
- `normalizeSet` and the `ReleaseTypes`/`EventTypes` allow-lists are ready for plan 02-04 (`UpdatePreferences`) to reuse without modification
- No blockers for plan 02-03 (`List`/`Remove`) or plan 02-04 (`UpdatePreferences`)

---
*Phase: 02-watchlist-core*
*Completed: 2026-08-06*

## Self-Check: PASSED

- `internal/watchlist/service.go` -- FOUND
- `internal/watchlist/service_test.go` -- FOUND
- `internal/watchlist/normalize_test.go` -- FOUND
- `internal/httpserver/watchlist.go` -- FOUND
- `internal/httpserver/watchlist_test.go` -- FOUND
- Commit `f65a135` -- FOUND in `git log --oneline --all`
- Commit `0c19af0` -- FOUND in `git log --oneline --all`
- Commit `b9db03b` -- FOUND in `git log --oneline --all`
- Commit `75cd8b0` -- FOUND in `git log --oneline --all`
