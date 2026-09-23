---
phase: 24-artist-tags-notes
plan: 01
subsystem: api
tags: [postgres-trigger, sqlc, pgx, tags, watchlist]

requires:
  - phase: 02-watchlist-core
    provides: watchlist.Store, internal/httpserver's registerDataRoutes pattern
provides:
  - "Migration 000010: tags, artist_tags, artist_tags_cap_trigger, watchlist.note"
  - "internal/tags package: NormalizeName, Service.Attach/Detach, error sentinels"
  - "POST/DELETE /watchlist/{id}/tags"
  - "GET /watchlist enriched with tags []TagRef and note *string (never null)"
affects: [24-02-tag-vocabulary, 24-03-notes-endpoint, 24-04..07-frontend]

actuals:
  tokens: 21512
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "BEFORE INSERT locking trigger with post-lock existence re-check for a non-bypassable per-row cap (ADR 0004)"
    - "Two parallel ARRAY(...) subqueries zipped in Go, not json_agg, for array-of-struct sqlc enrichment"
    - "get-or-create via single INSERT ... ON CONFLICT ((lower(name))) DO UPDATE ... RETURNING, no fallback SELECT"

key-files:
  created:
    - internal/db/migrations/000010_tags_and_notes.up.sql
    - internal/db/migrations/000010_tags_and_notes.down.sql
    - internal/db/tags_schema_test.go
    - queries/tags.sql
    - internal/tags/normalize.go
    - internal/tags/service.go
    - internal/tags/normalize_test.go
    - internal/tags/service_test.go
    - internal/httpserver/tags.go
    - internal/httpserver/tags_test.go
    - internal/watchlist/zip_test.go
  modified:
    - internal/db/migrate_test.go
    - internal/db/schema_version_test.go
    - docs/adr/0004-per-artist-tag-cap-trigger.md
    - queries/watchlist.sql
    - internal/watchlist/service.go
    - internal/watchlist/service_test.go
    - internal/httpserver/server.go
    - cmd/server/main.go

key-decisions:
  - "tags.Service.Detach and queries/tags.sql's DetachTag landed inside Task 2's commit rather than exactly at Task 3 -- writing the get-or-create/attach/detach SQL trio together was simpler than splitting DetachTag out, and it changes no behavior or test coverage boundary."
  - "TagStore (the httpserver-facing interface) gained Detach in Task 3, matching the plan's literal task split even though the service-layer method existed a commit earlier -- the HTTP route and 400/404/409 mapping are what Task 3 actually delivers."
  - "Database collation confirmed live: en_US.utf8 (datcollate/datctype), so lower()'s REGGAETON/reggaeton and REGGAETON-accented/reggaeton-accented identity tests are real proof, not accidentally-passing C/POSIX behavior."

patterns-established:
  - "Constraint-name error mapping (mapTagError) mirrors watchlist.Service's existing pgErr.ConstraintName switch -- one const block (tagCapConstraint) keeping the trigger's raised name and the Go literal in lockstep."

requirements-completed: [TAG-01, TAG-02, TAG-03, TAG-04, TAG-07]

coverage:
  - id: D1
    description: "Migration 000010 creates tags/artist_tags/the cap trigger/watchlist.note with every cap enforced at the database, refusing an 11th link, a too-long/untrimmed name, a case/accent duplicate, and a blank/over-long note -- independent of any API code (SC1, SC2)."
    requirement: TAG-04
    verification:
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagCapTrigger_RawInsertRefused"
        status: pass
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestTrigger_SkipExisting"
        status: pass
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagCapTrigger_SameTagConcurrentAt9"
        status: pass
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagNameChecks"
        status: pass
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_TagNameUniqueLower"
        status: pass
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_NoteChecks"
        status: pass
      - kind: integration
        ref: "internal/db/tags_schema_test.go#TestSchema_WatchlistDeleteKeepsTagLinks"
        status: pass
    human_judgment: false
  - id: D2
    description: "POST /watchlist/{id}/tags normalizes, finds-or-creates, and links a tag in one transaction; GET /watchlist shows it via the same single enrichment query, tags never null (SC2 API half, D-26, D-30)."
    requirement: TAG-01
    verification:
      - kind: unit
        ref: "internal/tags/normalize_test.go#TestNormalizeName"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_AttachEndToEnd"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Identity_ASCIICaseAndWhitespace"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Identity_NonASCIICaseFold"
        status: pass
    human_judgment: false
  - id: D3
    description: "Attach and detach are idempotent (D-21) and hold the 10-tag cap under real concurrent HTTP requests, both routes are gated/CSRF-protected, and tags survive a watchlist remove/re-add (TAG-07, TAG-02)."
    requirement: TAG-02
    verification:
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Attach_AtCapReturnsErrTagCapReachedAndRollsBackOrphan"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_Attach_ConcurrentCapRace"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_Detach_ConcurrentSameLinkBoth204"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_Gated401NoCookie"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_GatedForbiddenWithoutCSRFHeader"
        status: pass
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestService_Remove_LeavesArtistTagsIntact"
        status: pass
      - kind: unit
        ref: "internal/watchlist/zip_test.go#TestZipTags"
        status: pass
    human_judgment: false

duration: ~120min
completed: 2026-09-23
status: complete
---

# Phase 24 Plan 01: Migration, Tag Attach/Detach, Cap Trigger Summary

**Migration 000010 (tags/artist_tags/watchlist.note + a locking, skip-existing cap trigger from ADR 0004), an `internal/tags` package with normalize/attach/detach, and POST/DELETE `/watchlist/{id}/tags` enriching `GET /watchlist` with never-null `tags`/`note` via two parallel `ARRAY(...)` subqueries.**

## Performance

- **Duration:** ~120 min
- **Tasks:** 3 (Task 1 migration+schema tests, Task 2 tracer attach end-to-end, Task 3 detach/mapping/concurrency/gate)
- **Files modified:** 23 (11 created, 12 modified)

## Accomplishments

- Migration `000010_tags_and_notes` ships `tags`, `artist_tags`, `check_artist_tags_max_per_artist()`/`artist_tags_cap_trigger`, and `watchlist.note` with every cap inline; `cmd/migration-check` reports no finding; schema version bumped 9 → 10.
- `internal/db/tags_schema_test.go` proves all three ADR 0004 database-level guarantees against real Postgres, including a deliberately forced (not hoped-for) concurrent-same-tag race using `pg_stat_activity` lock-wait polling.
- `internal/tags` package: `NormalizeName` (NFC + trim/collapse + control-char/length validation), `Service.Attach` (get-or-create + link in one transaction, D-29) and `Service.Detach` (idempotent, D-21), with constraint-name error mapping to `ErrTagCapReached`/`ErrEntryNotFound`/`ErrNameTooLong`/`ErrNameInvalid`.
- `queries/watchlist.sql`'s `ListWatchlist` enriched with two parallel `ARRAY(...)` subqueries (`tag_ids`, `tag_names`), zipped in Go by `watchlist.zipTags` into a non-nil `[]TagRef` — never `json_agg`, avoiding the open sqlc/pgx `interface{}` bug.
- `POST /watchlist/{id}/tags` and `DELETE /watchlist/{id}/tags/{tag_id}` registered inside `registerDataRoutes`, inheriting `gate.Authenticate` + `gate.RequireCSRFHeader` automatically; full 400/404/409/204 mapping, a looped (10x) real concurrent cap race, and a real concurrent same-link detach race all pass.

## Task Commits

Each task was committed atomically (Task 2 followed TDD RED→GREEN for `NormalizeName`):

1. **Task 1: Migration 000010 and its database-level invariants** — `2463157` (feat)
2. **Task 2 RED: failing test for `tags.NormalizeName`** — `9109160` (test)
3. **Task 2 GREEN: implement `tags.NormalizeName`** — `72084e9` (feat)
4. **Task 2: attach a tag end-to-end and enrich `GET /watchlist`** — `a45fe35` (feat)
5. **Task 3: detach, full error mapping, concurrency, gate/CSRF coverage** — `cbf07dc` (feat)

**Plan metadata:** commit follows this SUMMARY.

## Files Created/Modified

- `internal/db/migrations/000010_tags_and_notes.up.sql` / `.down.sql` — tags/artist_tags/cap trigger/watchlist.note
- `internal/db/tags_schema_test.go` — seven schema-level tests including the forced concurrency race
- `queries/tags.sql` — `GetWatchlistArtistID`, `GetOrCreateTag`, `AttachTag`, `DetachTag`
- `queries/watchlist.sql` — `ListWatchlist` gains `w.note`, `tag_ids`, `tag_names`
- `internal/tags/normalize.go`, `service.go` (+ their `_test.go`) — the new package
- `internal/watchlist/service.go` — `TagRef`, `Entry.Tags`/`Entry.Note`, `zipTags`
- `internal/httpserver/tags.go` (+ `tags_test.go`) — attach/detach handlers, `TagStore`, `WithTags`
- `internal/httpserver/server.go` — `Server.tags`, both routes registered inside `registerDataRoutes`
- `cmd/server/main.go` — `httpserver.WithTags(tags.NewService(pool))`
- `internal/db/migrate_test.go`, `schema_version_test.go` — schema version 9 → 10
- `docs/adr/0004-per-artist-tag-cap-trigger.md` — Consequences note on the post-lock re-check
- `internal/watchlist/service_test.go`, `zip_test.go` — tag-survival and zip-contract tests

## Decisions Made

- `tags.Service.Detach` and `DetachTag` SQL landed in Task 2's commit rather than exactly at Task 3 (writing the attach/detach SQL trio together was simpler); `TagStore`'s `Detach` method, the HTTP route, and its 400/404/409/204 mapping are still exactly what Task 3 delivers, matching the plan's intended task boundary at the interface/handler level.
- Live database collation confirmed as `en_US.utf8` (`SELECT datcollate, datctype FROM pg_database`), closing 24-RESEARCH.md's open Assumption A1 — the `REGGAETÓN`/`reggaetón` identity tests are proof, not luck.
- `go test ./... -race` is unusable on this Windows dev box (cgo toolchain broken, pre-existing documented limitation) — substituted plain `go test ./... -count=1` for local verification; CI's Linux `test` job remains the authoritative `-race` gate.

## Deviations from Plan

None — plan executed exactly as written. (See "Decisions Made" above for one minor sequencing note: `Service.Detach`/`DetachTag` were written a commit earlier than Task 3's literal position, with no behavior or scope change.)

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `internal/tags` package, `queries/tags.sql`, and the enriched `GET /watchlist` payload are ready for plan 24-02 (tag vocabulary: rename/merge/delete) and 24-03 (notes endpoint) to build on.
- `docs/adr/0004-per-artist-tag-cap-trigger.md`'s Consequences section now documents the post-lock existence re-check for future readers of the merge transaction (24-02).
- No blockers.

---

*Phase: 24-artist-tags-notes*
*Completed: 2026-09-23*

## Self-Check: PASSED

- All key files verified present on disk (11 created files, all `FOUND`).
- All 5 task commit hashes verified present in `git log --oneline --all` (2463157, 9109160, 72084e9, a45fe35, cbf07dc).
- All acceptance criteria across Tasks 1-3 re-run and passing (schema tests, tracer end-to-end, detach/mapping/concurrency/gate tests).
- Plan-level `<verification>` re-run: `migration-check` exits 0, `sqlc-check` clean (no diff after regenerate), `go vet`/`golangci-lint` clean, full `go test ./... -count=1` green (repo-wide, -race unavailable on this Windows box per documented limitation), `coverage-report` measured 90.58% (floor 80%), no `go.mod`/`go.sum` diff.
