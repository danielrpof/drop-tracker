---
phase: 02-watchlist-core
plan: 07
subsystem: watchlist-api
tags: [go, json, postgres, watchlist, gap-closure, domain-boundary]

requires:
  - phase: 02-watchlist-core (plan 04)
    provides: PATCH /watchlist/{id} handler and the neither-axis 400 this plan relocates
  - phase: 02-watchlist-core (plan 06)
    provides: single-statement UpdateWatchlistPreferences this plan's guard runs ahead of
provides:
  - "watchlist.ErrNoPreferencesSupplied: a package-level sentinel returned by Service.UpdatePreferences before any database call when neither preference axis is supplied -- closing the domain-boundary gap where only the HTTP handler enforced the rule"
  - "decodeJSONBody: one shared JSON decode path in internal/httpserver/watchlist.go that rejects a body carrying more than one JSON value, used by both handleAddWatchlist and handleUpdateWatchlist"
affects: []

actuals:
  tokens: 5686
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Domain-boundary rejection guard as the first statement of a Service method, ahead of both validation and the database call, so the rejection's precedence over other error paths (e.g. not-found) is structurally guaranteed rather than incidental to statement order"
    - "Shared JSON decode helper that asserts stream exhaustion via a second dec.Decode into a throwaway struct requiring errors.Is(err, io.EOF) -- the standard Go idiom for rejecting a body with more than one concatenated top-level JSON value, since DisallowUnknownFields only constrains keys inside the decoded object"

key-files:
  created: []
  modified:
    - internal/watchlist/service.go
    - internal/watchlist/service_test.go
    - internal/httpserver/watchlist.go
    - internal/httpserver/watchlist_test.go

key-decisions:
  - "Kept the sentinel's identifier out of handleUpdateWatchlist's doc comment (described in prose instead) so the file's grep-verified invariant of exactly one ErrNoPreferencesSupplied reference (the error-switch case) holds without a redundant doc-comment mention miscounting as a second copy of the condition"
  - "decodeJSONBody returns a plain fmt.Errorf on the trailing-value failure rather than a new sentinel, since both call sites already collapse every decode failure to the same fixed 400 message -- classifying the failure would add a distinction no caller uses"

requirements-completed: [WLST-02, WLST-05, WLST-06]

coverage:
  - id: D1
    description: "Service.UpdatePreferences rejects a call supplying neither preference axis with ErrNoPreferencesSupplied before any database call, and this precedence outranks the not-found path for an unknown id"
    requirement: "WLST-05"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_NeitherAxisReturnsErrNoPreferencesSupplied"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_NeitherAxisOutranksUnknownID"
        status: pass
    human_judgment: false
  - id: D2
    description: "PATCH /watchlist/{id} with an empty body still answers 400 with the byte-identical {\"error\":\"no preferences supplied\"} body, now sourced from the domain sentinel through the handler's error switch instead of a duplicated handler-side check"
    requirement: "WLST-06"
    verification:
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_NoPreferencesSuppliedReturns400"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_EmptyBodyStillRejectedEndToEnd"
        status: pass
    human_judgment: false
  - id: D3
    description: "Both POST /watchlist and PATCH /watchlist/{id} reject a body carrying a second JSON value after the first (object, array, scalar, or non-JSON), with the store never invoked, while a body followed only by whitespace is still accepted"
    requirement: "WLST-02"
    verification:
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Add_BodyMustContainExactlyOneJSONValue"
        status: pass
      - kind: unit
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_BodyMustContainExactlyOneJSONValue"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every pre-existing UpdatePreferences/Add/Patch behaviour is unchanged, go.mod/go.sum are byte-identical, and go test ./... -short still skips database-backed tests cleanly"
    requirement: "WLST-05"
    verification:
      - kind: integration
        ref: "go test ./... -count=1 (full suite against fixture Postgres, all pre-existing tests unedited)"
        status: pass
      - kind: automated
        ref: "go test ./... -short -count=1"
        status: pass
      - kind: automated
        ref: "git diff --exit-code -- go.mod go.sum"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-08-06
status: complete
---

# Phase 2 Plan 07: Close Gap G-02-1 — Domain-Boundary Guard and Trailing-JSON Rejection Summary

`Service.UpdatePreferences` now rejects a call supplying neither preference axis with a dedicated `ErrNoPreferencesSupplied` sentinel before any database call, closing the gap where only `handleUpdateWatchlist` enforced this rule (WR-01) -- and both watchlist JSON routes now share one `decodeJSONBody` helper that asserts the request body held exactly one JSON value, rejecting a body with a second value concatenated after the first that was previously silently discarded (WR-02).

## Performance

- **Duration:** ~40 min
- **Completed:** 2026-08-06
- **Tasks:** 2
- **Files modified:** 4 (0 created, 4 modified)

## Accomplishments

- `watchlist.ErrNoPreferencesSupplied` added as a fourth package-level sentinel alongside `ErrDuplicate`/`ErrNotFound`/`ErrInvalidReleaseType`/`ErrInvalidEventType`, with message text `"no preferences supplied"` -- byte-identical to the wire contract `handleUpdateWatchlist` already produced
- `Service.UpdatePreferences`'s first statement now rejects an update with both `PreferencesParams` pointer fields nil, ahead of validation and the database call -- pinned by a real-Postgres test asserting `updated_at` is untouched (proof no write reached the row) and a second test proving the guard outranks `ErrNotFound` for an unknown id
- `handleUpdateWatchlist`'s own two-condition guard is deleted; the handler now forwards an empty request to the domain and translates the sentinel to 400 through its existing `errors.Is` switch, with a fixed literal (not `err.Error()`) so a future wrap of the sentinel can never change the response body
- `decodeJSONBody` replaces both hand-copied `json.NewDecoder`/`DisallowUnknownFields`/`Decode` blocks in `handleAddWatchlist` and `handleUpdateWatchlist`: after the primary decode succeeds, a second decode into a throwaway `struct{}{}` must report `io.EOF`, or the body carried more than one JSON value and is rejected with the same 400/message every other malformed body already produces
- Six new tests (two real-Postgres service-level, four handler-level table-driven/stub-backed) all proven red against the prior implementation for their own distinct reason, then green; every pre-existing `TestService_UpdatePreferences_*` and `TestWatchlist_*` test passes unedited
- Full suite green (`go test ./... -count=1` against fixture Postgres and `go test ./... -short`), `go.mod`/`go.sum` byte-identical, `internal/httpserver/watchlist.go` constructs a JSON decoder in exactly one non-comment place and references `ErrNoPreferencesSupplied` exactly once

## Task Commits

Both tasks followed the RED/GREEN TDD cycle:

1. **Task 1 (RED):** `3d1841c` (test) -- `TestService_UpdatePreferences_NeitherAxisReturnsErrNoPreferencesSupplied`, `TestService_UpdatePreferences_NeitherAxisOutranksUnknownID`, `TestWatchlist_Patch_NoPreferencesSuppliedReturns400`, `TestWatchlist_Patch_EmptyBodyStillRejectedEndToEnd` -- confirmed failing (compile error on the undefined sentinel)
2. **Task 1 (GREEN):** `8f15af6` (feat) -- sentinel added, `UpdatePreferences` guarded as its first statement, handler's duplicate check collapsed onto `errors.Is`
3. **Task 2 (RED):** `90e4ad1` (test) -- `TestWatchlist_Add_BodyMustContainExactlyOneJSONValue`, `TestWatchlist_Patch_BodyMustContainExactlyOneJSONValue` (table-driven: object/array/scalar/non-JSON trailing suffixes), confirmed failing (201/200 instead of 400, store called instead of not)
4. **Task 2 (GREEN):** `2736d71` (feat) -- `decodeJSONBody` helper added, both call sites rewritten to use it

## Files Created/Modified

- `internal/watchlist/service.go` -- `ErrNoPreferencesSupplied` sentinel added; `UpdatePreferences` guarded as its first statement; doc comments updated
- `internal/watchlist/service_test.go` -- two new real-Postgres tests for the sentinel and its precedence over `ErrNotFound`
- `internal/httpserver/watchlist.go` -- `decodeJSONBody` helper added next to `writeError`; both decode call sites rewritten; `handleUpdateWatchlist`'s duplicate guard deleted and its error switch extended; doc comments updated
- `internal/httpserver/watchlist_test.go` -- four new tests: two stub/end-to-end tests for the neither-axis 400 translation, two table-driven tests for trailing-JSON-value rejection on both routes

## Decisions Made

- Kept the sentinel's identifier out of `handleUpdateWatchlist`'s doc comment (described in prose instead), since the plan's verification requires exactly one `ErrNoPreferencesSupplied` reference in the file (the error-switch case) and a doc-comment mention would have counted as a second occurrence under the grep-based check
- `decodeJSONBody` returns a plain `fmt.Errorf` on the trailing-value failure rather than a new sentinel, since both call sites already collapse every decode failure -- syntax error, type error, or trailing value -- to the same fixed `"invalid request body"` 400; classifying the failure would add a distinction no caller uses, consistent with the plan's instruction that the helper deliberately not classify its failure mode

## Deviations from Plan

None -- both tasks executed exactly as written, including the exact guard position (first statement, ahead of validation), the fixed-literal (not `err.Error()`) requirement on the sentinel's 400 translation, and the exhaustion-check idiom (`errors.Is(err, io.EOF)` against a second decode into a throwaway struct) specified for `decodeJSONBody`.

## Issues Encountered

The first pass of `handleUpdateWatchlist`'s rewritten doc comment named `watchlist.ErrNoPreferencesSupplied` literally, which made the file's `grep -c 'ErrNoPreferencesSupplied'` verification report 2 instead of the required 1 (the doc comment plus the error-switch case). Reworded the comment to describe the behavior in prose without repeating the identifier; re-ran the grep check to confirm exactly 1, then re-ran the full test suite to confirm no regression.

## Next Phase Readiness

- Gap `G-02-1` is closed on both findings (WR-01 and WR-02); `02-UAT.md`'s gap entry can be reconciled on the next `/gsd-verify-work` pass
- The "reject neither axis" contract for WLST-05/WLST-06 is now enforced at the domain boundary `watchlist.Store` documents itself to be -- Phase 3's search proxy and Phase 4's poller, both non-HTTP callers of `watchlist.Store`, inherit this guard automatically rather than needing to reimplement it
- Both watchlist JSON routes share one decode path; a future handler added to this file has no old decode block to copy that would reintroduce the trailing-data hole
- `G-02-2` (the separate `internal/db/migrate.go` `redactError` keyword/value-form DSN gap) remains open and is out of this plan's scope -- it targets a Phase 1 file with zero Phase 2 commits against it and is tracked independently per `02-UAT.md`

---
*Phase: 02-watchlist-core*
*Completed: 2026-08-06*

## Self-Check: PASSED

- `internal/watchlist/service.go` — FOUND
- `internal/watchlist/service_test.go` — FOUND
- `internal/httpserver/watchlist.go` — FOUND
- `internal/httpserver/watchlist_test.go` — FOUND
- `.planning/phases/02-watchlist-core/02-07-SUMMARY.md` — FOUND
- Commit `3d1841c` — FOUND in `git log --oneline --all`
- Commit `8f15af6` — FOUND in `git log --oneline --all`
- Commit `90e4ad1` — FOUND in `git log --oneline --all`
- Commit `2736d71` — FOUND in `git log --oneline --all`
