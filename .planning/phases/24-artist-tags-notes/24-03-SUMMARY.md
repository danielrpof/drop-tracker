---
phase: 24-artist-tags-notes
plan: 03
subsystem: api
tags: [sqlc, pgx, postgres-check-constraint, watchlist, notes]

requires:
  - phase: 24-01
    provides: "watchlist.note column, Entry.Tags/Entry.Note, zipTags, the watchlist_note_length/watchlist_note_not_blank CHECK constraints"
  - phase: 24-02
    provides: "internal/tags vocabulary routes (GET/PATCH/DELETE /tags, POST /tags/{id}/merge) -- unrelated surface, same phase"
provides:
  - "PUT /watchlist/{id}/note -- trim / empty-to-null / 500-rune-cap note editing (D-25, NOTE-01)"
  - "POST /watchlist accepts an optional note (D-27, Undo restoration path)"
  - "POST /watchlist and PATCH /watchlist/{id} answer on the same tags + note projection as GET /watchlist (D-26)"
affects: [24-07-frontend-note-editor]

actuals:
  tokens: 14625
  tasks: 2
  commits: 2
  plan_head_before: 317c9e6

tech-stack:
  added: []
  patterns:
    - "Service.get(id) re-reads through GetWatchlistEntry (a byte-for-byte ListWatchlist projection narrowed to one row) so every mutating route (Add, UpdatePreferences, UpdateNote) returns exactly what GET would show -- the sqlc.ListWatchlistRow(...) struct conversion only compiles while the two projections stay identical, making drift a build error rather than a runtime surprise."
    - "Handler-level fail-fast note validation (watchlist.NormalizeNote called directly in handleAddWatchlist) mirrors the existing preference-validation shape, with Service.Add's own NormalizeNote call as the non-bypassable backstop -- three-layer validation extended to a third field."

key-files:
  created: []
  modified:
    - queries/watchlist.sql
    - internal/db/sqlc/watchlist.sql.go
    - internal/db/sqlc/querier.go
    - internal/watchlist/service.go
    - internal/watchlist/service_test.go
    - internal/httpserver/watchlist.go
    - internal/httpserver/watchlist_test.go
    - internal/httpserver/server.go
    - internal/authgate/gate_test.go
    - internal/poller/poller_test.go

key-decisions:
  - "GetWatchlistEntry is written as ListWatchlist's exact select list and joins narrowed to WHERE w.id = $1, so sqlc.ListWatchlistRow(row) is a legal Go type conversion between the two generated structs -- the compiler itself enforces D-26's 'same projection everywhere' guarantee rather than relying on comment discipline to keep two hand-maintained SELECT lists in sync."
  - "entryFromRow was extracted from List's loop body so List and the new get(ctx, id) helper build an Entry the same way; List's own behavior and tests are unchanged, this is a pure refactor landed as part of Task 1."
  - "toEntry was deleted once Add's last caller of it (its own return statement) switched to s.get(ctx, entry.ID) -- golangci-lint's unused check confirmed nothing else referenced it."

requirements-completed: [TAG-07]

coverage:
  - id: D1
    description: "PUT /watchlist/{id}/note trims, caps at 500 runes (Unicode code points), stores NULL for null/empty/whitespace-only text, and rejects a body with the note key absent or of the wrong JSON type; the updated entry (with tags) shows immediately on GET /watchlist (NOTE-01, D-25, D-26)."
    requirement: NOTE-01
    verification:
      - kind: unit
        ref: "internal/watchlist/service_test.go#TestNormalizeNote"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Note_UpdateExistingReturnsEntryWithNoteAndTags"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Note_UpdateMissingReturnsErrNotFound"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Note_TooLongReturnsErrNoteTooLong"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_NoteEndToEnd"
        status: pass
    human_judgment: false
  - id: D2
    description: "PUT /watchlist/{id}/note is gated: 401 without a session, 403 and zero store calls without the CSRF header (T-24-18, T-24-19)."
    requirement: NOTE-01
    verification:
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Note_Gated401NoCookie"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Note_GatedForbiddenWithoutCSRFHeader"
        status: pass
    human_judgment: false
  - id: D3
    description: "POST /watchlist accepts an optional note under the same normalization rules, applied before any database write so a rejected note leaves no artists row; PATCH /watchlist/{id} still rejects a note key with 400 (D-25, D-27)."
    requirement: NOTE-01
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Add_WithNoteReturnsNormalizedNote"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Add_WithTooLongNoteLeavesNoArtistsRow"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Add_WithNoteEndToEnd"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Add_WithTooLongNoteReturns400"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_RejectsNoteKey"
        status: pass
    human_judgment: false
  - id: D4
    description: "POST /watchlist, PATCH /watchlist/{id}, PUT .../note and GET /watchlist all answer through the identical tags + note projection, and a re-added artist comes back with its surviving tags and no note unless Undo supplies one (TAG-07, D-10, D-26, D-27)."
    requirement: TAG-07
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Add_ReAddReturnsSurvivingTags"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Add_ReAddRestoresNote"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_UpdatePreferences_ReturnsTagsAndNote"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_ProjectionParity"
        status: pass
      - kind: integration
        ref: "internal/httpserver/watchlist_test.go#TestWatchlist_Patch_ReturnsTagsAndNote"
        status: pass
    human_judgment: false

duration: ~50min
completed: 2026-09-23
status: complete
---

# Phase 24 Plan 03: Watchlist Note Endpoint + Shared Projection Summary

**A dedicated `PUT /watchlist/{id}/note` (trim/cap/clear), an optional note on `POST /watchlist` for Undo, and one shared `GetWatchlistEntry`/`Service.get` projection so `POST`, `PATCH`, and the note `PUT` all answer with the exact `tags` + `note` shape `GET /watchlist` does.**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-09-23T21:24:00Z (approx.)
- **Completed:** 2026-09-23T22:14:01Z
- **Tasks:** 2 completed
- **Files modified:** 10

## Accomplishments

- `GetWatchlistEntry` copies `ListWatchlist`'s select list and joins byte-for-byte, narrowed to one row -- the `sqlc.ListWatchlistRow(row)` struct conversion in `Service.get` only compiles while the two stay identical, so D-26's "same projection everywhere" guarantee is enforced by the Go compiler, not comment discipline.
- `NormalizeNote` (trim, empty/whitespace-to-nil, NUL rejection, 500-rune cap counted in Unicode code points) backs both `PUT /watchlist/{id}/note` and the optional `note` on `POST /watchlist`; `Service.UpdateNote` maps the `watchlist_note_length`/`watchlist_note_not_blank` CHECK constraints as the non-bypassable backstop.
- `Add` and `UpdatePreferences` were rewired to return through the new `s.get(ctx, id)` helper instead of hand-building an `Entry` -- `toEntry` (always-empty `Tags`) is gone, and `UpdatePreferences`'s prior placeholder-tags comment is resolved for real.
- A re-added artist's response now carries its surviving tags (TAG-07) and, on the Undo path, the note the remove toast's `POST` re-supplies (D-27) -- proven both at the service layer and end-to-end over real Postgres.
- All three `watchlist.Store` test doubles (httpserver's func-field `stubStore`, authgate's fixed-return `stubStore`, poller's call-counting `stubStore`) gained `UpdateNote`; poller's existing "cycle never writes" assertion now also checks `noteCalls == 0`.

## Task Commits

Each task was committed atomically:

1. **Task 1: PUT /watchlist/{id}/note end to end (tracer)** - `b8002bf` (feat)
2. **Task 2: POST note + shared tags/note projection** - `f9b6868` (feat)

**Plan metadata:** commit follows this SUMMARY.

## Files Created/Modified

- `queries/watchlist.sql` - `GetWatchlistEntry`, `UpdateWatchlistNote`; `CreateWatchlistEntry` gains a `note` column
- `internal/db/sqlc/watchlist.sql.go`, `internal/db/sqlc/querier.go` - sqlc-regenerated types/methods for the above
- `internal/watchlist/service.go` - `MaxNoteRunes`, `ErrNoteTooLong`, `ErrNoteInvalid`, `NormalizeNote`, `entryFromRow`, `Service.get`, `Service.UpdateNote`; `Add`/`UpdatePreferences` now return through `get`; `toEntry` removed
- `internal/watchlist/service_test.go` - `TestNormalizeNote`, `TestService_Note*`, `TestService_Add_With*`, `TestService_Add_ReAdd*`, `TestService_UpdatePreferences_ReturnsTagsAndNote`, `TestService_ProjectionParity`
- `internal/httpserver/watchlist.go` - `updateNoteRequest`, `handleUpdateNote`; `addWatchlistRequest.Note` + handler-level note validation
- `internal/httpserver/watchlist_test.go` - `TestWatchlist_NoteEndToEnd`, `TestWatchlist_Note_Gated*`, `TestWatchlist_Add_WithNote*`, `TestWatchlist_Patch_RejectsNoteKey`, `TestWatchlist_Patch_ReturnsTagsAndNote`
- `internal/httpserver/server.go` - `r.Put("/watchlist/{id}/note", s.handleUpdateNote)` registered inside `registerDataRoutes`
- `internal/authgate/gate_test.go`, `internal/poller/poller_test.go` - `UpdateNote` added to the `watchlist.Store` test doubles

## Decisions Made

- `GetWatchlistEntry`'s select list is a literal copy of `ListWatchlist`'s (not a shared SQL fragment, which sqlc doesn't support) -- the drift guard is the Go-level struct conversion, documented inline at both the query and the `Service.get` call site.
- Handler-level `NormalizeNote` call in `handleAddWatchlist` (fail-fast, before the store call) plus `Service.Add`'s own `NormalizeNote` call (non-bypassable backstop) is intentional duplication, matching the existing preference-validation three-layer pattern rather than relying on the service layer alone.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `PUT /watchlist/{id}/note`, the optional `POST /watchlist` note, and the shared `tags` + `note` projection on `POST`/`PATCH`/`PUT`/`GET` are ready for plan 24-07's SPA note editor and Undo wiring.
- NOTE-01 stays unmarked in REQUIREMENTS.md -- plan 24-07 also declares it (shared-ID gate) and is the plan that closes it; TAG-07 is marked complete here.
- No blockers.

---

*Phase: 24-artist-tags-notes*
*Completed: 2026-09-23*

## Self-Check: PASSED

- Both key files' referenced commits verified present in `git log --oneline --all` (b8002bf, f9b6868).
- All acceptance criteria across Tasks 1-2 re-run and passing (grep checks for query names, `sqlc.ListWatchlistRow(`, `NormalizeNote(`, route registration, `SetAttrs` note-text-free, `toEntry` removal, `updateWatchlistRequest` note-free).
- Plan-level `<verification>` re-run: `make sqlc-check` equivalent (docker sqlc generate, confirmed idempotent -- no diff on a second run), `go vet ./...` clean, `golangci-lint run ./...` reports 0 issues, full `go test ./... -count=1 -coverprofile=coverage.out` green across all 24 packages (`-race` unavailable on this Windows box, documented limitation; CI's Linux `test` job is the authoritative `-race` gate), `cmd/coverage-report --mode=total` measured 89.33% (floor 80%), `git diff --exit-code -- go.mod go.sum internal/db/migrations` clean (no dependency or schema change).
