---
phase: 24-artist-tags-notes
plan: 02
subsystem: api
tags: [postgres-locking-cte, sqlc, pgx, tags, merge-transaction]

requires:
  - phase: 24-01
    provides: "internal/tags package (NormalizeName, Service.Attach/Detach, tags.DB seam, error sentinels), tags/artist_tags/watchlist.note schema"
provides:
  - "GET /tags -- whole vocabulary with watched-only carrier counts, stable lower(name) order"
  - "PATCH /tags/{id} -- rename with a 409 collision body carrying everything the merge confirm needs"
  - "POST /tags/{id}/merge -- confirmed merge, never inserts, keeps the target's stored casing"
  - "DELETE /tags/{id} -- removes a tag everywhere, reports the watched-carrier count"
affects: [24-04..07-frontend, 24-05-autocomplete]

actuals:
  tokens: 21908
  tasks: 3
  commits: 3
  plan_head_before: 77f9f24

tech-stack:
  added: []
  patterns:
    - "Collision detection from a unique-violation constraint name, never a pre-check (Rename calls RenameTag before any GetTagByName lookup)"
    - "Data-modifying WITH CTE (DeleteTagCountingCarriers) where a sibling read CTE reads the pre-statement snapshot, so a delete's own cascade never contaminates its own reported count"
    - "Merge transaction: delete-duplicates then UPDATE (never INSERT) then delete-source, all under an ORDER BY id FOR UPDATE lock -- the only statement order that cannot transiently exceed the per-artist cap trigger"

key-files:
  modified:
    - queries/tags.sql
    - internal/db/sqlc/tags.sql.go
    - internal/db/sqlc/querier.go
    - internal/tags/service.go
    - internal/tags/service_test.go
    - internal/httpserver/tags.go
    - internal/httpserver/server.go
    - internal/httpserver/tags_test.go

key-decisions:
  - "parseTagID generalized to take the path-param name (\"id\" on /tags/{id} routes, \"tag_id\" on the watchlist-scoped attach/detach routes) rather than adding a second near-duplicate parser -- matches the plan's literal parseTagID(r, \"id\") call sites."
  - "D-22's 404/409 interpretation for merge, as the plan's own context-drift note specifies: 404 tag not found when the source or target id no longer exists, 400 when into equals the source id (ErrMergeIntoSelf), and a target renamed since a prior 409 merges under its current stored name (D-23) -- the residual (target renamed from a second tab mid-confirm) is accepted as T-24-15, unchanged from the plan."

patterns-established:
  - "CollisionError{Target Tag; CarrierCountAfterMerge int64} is the one shape both a 409 rename response and a subsequent confirmed Merge's 200 response draw from -- TestService_Merge_MatchesPriorCollisionCount pins that the two numbers agree for the same pair."

requirements-completed: [TAG-01, TAG-03, TAG-05, TAG-06]

coverage:
  - id: D1
    description: "GET /tags returns the whole vocabulary as [{id, name, carrier_count}], never null, ordered by lower(name) then id, including zero-link tags and tags only removed artists carry (TAG-05, D-11, D-12, D-13)."
    requirement: TAG-05
    verification:
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_List_EmptyVocabularyReturnsNonNilEmptySlice"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_List_CarrierCountsIncludeZeroAndRemoved"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_List_OrderedByLowerNameThenID"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_ListEndToEnd"
        status: pass
    human_judgment: false
  - id: D2
    description: "PATCH /tags/{id} applies a rename everywhere including a case-only rename; a collision with a different tag returns 409 with the target's stored casing and the post-merge union count, changing nothing about the source (D-09, D-22, D-23, TAG-05 SC3)."
    requirement: TAG-05
    verification:
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Rename_AppliesEverywhere"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Rename_CaseOnlySameTagIsPlainRename"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Rename_CollisionReturnsCollisionErrorAndChangesNothing"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_RenameEndToEnd_AppliesToWatchlist"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_Rename_CollisionReturns409WithBody"
        status: pass
    human_judgment: false
  - id: D3
    description: "POST /tags/{id}/merge unions memberships via delete-then-UPDATE, never INSERT, keeps the target's stored casing, and succeeds on a 10-tag artist that carries only the source (ADR 0004 required test 3, D-19, D-23)."
    requirement: TAG-05
    verification:
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Merge_UnionsMembershipsAndDeletesSource"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Merge_AtCap"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Merge_AtCapBothSourceAndTarget"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Merge_MatchesPriorCollisionCount"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_MergeEndToEnd"
        status: pass
    human_judgment: false
  - id: D4
    description: "DELETE /tags/{id} removes a tag from every artist (including removed ones), reports the watched-carrier count it deleted from, and leaves every watchlist entry's preferences and note byte-identical (TAG-06, D-11, SC4)."
    requirement: TAG-06
    verification:
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Delete_RemovesEverywhereAndReturnsWatchedCount"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_DeleteEndToEnd_LeavesWatchlistUntouched"
        status: pass
      - kind: integration
        ref: "internal/tags/service_test.go#TestService_Delete_TwiceReturnsErrTagNotFoundSecondTime"
        status: pass
    human_judgment: false
  - id: D5
    description: "All four vocabulary routes answer 401 without a session; the three write routes (rename, merge, delete) answer 403 without X-Requested-With and reach the store zero times (T-24-10, T-24-11)."
    requirement: TAG-05
    verification:
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_Vocabulary_Gated401NoCookie"
        status: pass
      - kind: integration
        ref: "internal/httpserver/tags_test.go#TestTags_Vocabulary_GatedForbiddenWithoutCSRFHeader"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-09-23
status: complete
---

# Phase 24 Plan 02: Tag Vocabulary (List, Rename, Merge, Delete) Summary

**GET/PATCH/POST-merge/DELETE `/tags` routes over a new `queries/tags.sql` group and `internal/tags.Service` additions -- watched-only carrier counts, unique-violation-driven rename collisions carrying everything the merge confirm needs, and a delete-then-UPDATE merge transaction that never inserts.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 3 (Task 1 the GET /tags tracer, Task 2 rename/collision/delete, Task 3 merge transaction + full vocabulary gate/CSRF coverage)
- **Files modified:** 8

## Accomplishments

- `ListTags` LEFT-JOINs through `artist_tags`/`watchlist` so `count(w.id)` counts watched carriers only (D-11), while zero-link tags (D-12) and tags only removed artists carry (D-13) still surface, ordered by `lower(name)` then `id` (TAG-05) -- proven with a mixed-casing five-tag ordering test and an empty-vocabulary `[]` (never null) assertion.
- `Service.Rename` calls `RenameTag` before any `GetTagByName` lookup, so a collision is detected from the `tags_name_lower_idx` unique-violation constraint name, never a pre-check (D-09, D-22); on collision it returns `*CollisionError{Target, CarrierCountAfterMerge}` using the target's own stored casing (D-23) and retries once if the collider vanished between statements. A case-only rename or a rename to a tag's own current name never collides, because a row's own index entry cannot conflict with itself.
- `Service.Delete` uses one data-modifying CTE (`DeleteTagCountingCarriers`) where the count-CTE reads the pre-statement snapshot, so the reported watched-carrier count is exactly what the delete removed, independent of the DELETE's own cascade into `artist_tags`; every watchlist row's preferences and note are proven byte-identical before/after (SC4).
- `Service.Merge` locks both tags with `ORDER BY id FOR UPDATE` (deadlock-proof, and it parks a concurrent attach/rename of either tag), then deletes duplicate source links, `UPDATE`s the rest onto the target, and deletes the source -- never `INSERT`, so a 10-tag artist merging in a tag it already carries never transiently holds 11 links (D-19, ADR 0004). ADR 0004 required test 3 (a 10-tag artist carrying only the source) and the both-source-and-target companion case (10 -> 9) both pass, and a prior 409's `carrier_count_after_merge` is proven to equal what the confirmed `Merge` actually returns.
- All four vocabulary routes (`GET /tags`, `PATCH /tags/{id}`, `POST /tags/{id}/merge`, `DELETE /tags/{id}`) registered inside `registerDataRoutes`, inheriting `gate.Authenticate` + `gate.RequireCSRFHeader` automatically; `TestTags_Vocabulary_Gated401NoCookie` and `TestTags_Vocabulary_GatedForbiddenWithoutCSRFHeader` cover all four in table-driven subtests, the latter also asserting zero store calls on the three write routes.

## Task Commits

Each task was committed atomically (full-stack SQL+service+handler work landed as one `feat` commit per task, matching plan 24-01's established convention for tracer/integration tasks -- see Decisions Made):

1. **Task 1: GET /tags end to end with watched-only carrier counts** — `057bb2b` (feat)
2. **Task 2: Rename with 409 collision, and delete with a carrier count** — `f6e2630` (feat)
3. **Task 3: Merge transaction (never inserts) and gate/CSRF coverage for all vocabulary routes** — `072fb51` (feat)

**Plan metadata:** commit follows this SUMMARY.

## Files Created/Modified

- `queries/tags.sql` — `ListTags`, `RenameTag`, `GetTagByName`, `CountCarriersForTags`, `DeleteTagCountingCarriers`, `LockTagsForMerge`, `DeleteDuplicateSourceLinks`, `RepointSourceLinks`, `DeleteTag`, `CountCarriers`
- `internal/db/sqlc/tags.sql.go`, `internal/db/sqlc/querier.go` — sqlc-generated methods/row types for the above
- `internal/tags/service.go` — `Summary`, `CollisionError`, `ErrMergeIntoSelf`; `Service.List/Rename/Merge/Delete`
- `internal/tags/service_test.go` — `TestService_List*`, `TestService_Rename*`, `TestService_Delete*`, `TestService_Merge*`
- `internal/httpserver/tags.go` — `TagStore` gains `List/Rename/Delete/Merge`; `handleListTags`, `handleRenameTag`, `handleDeleteTag`, `handleMergeTag`; `renameTagRequest`, `tagCollisionResponse`, `deleteTagResponse`, `mergeTagRequest`; `parseTagID` generalized to take a param name
- `internal/httpserver/server.go` — `GET /tags`, `PATCH /tags/{id}`, `DELETE /tags/{id}`, `POST /tags/{id}/merge` registered in `registerDataRoutes`
- `internal/httpserver/tags_test.go` — `fakeTagStore` extended (`listFunc`/`renameFunc`/`deleteFunc`/`mergeFunc` plus call counters); `TestTags_ListEndToEnd`, `TestTags_Rename*`, `TestTags_Delete*`, `TestTags_Merge*`, `TestTags_Vocabulary_Gated401NoCookie`, `TestTags_Vocabulary_GatedForbiddenWithoutCSRFHeader`

## Decisions Made

- `parseTagID` widened from a hardcoded `"tag_id"` reader to `parseTagID(r, param string)`, with the existing detach call site updated to `parseTagID(r, "tag_id")` and the three new `/tags/{id}` routes using `parseTagID(r, "id")` -- avoids a near-duplicate parser and matches the plan's literal call-site text.
- Each task's full query+service+handler+test work landed as a single `feat` commit rather than a strict per-task RED/GREEN split. This mirrors plan 24-01's own precedent (its tracer and full-mapping tasks, both `tdd="true"`, each landed as one commit): a query/service/handler/route trio has to exist together to compile, so a genuinely-failing intermediate RED state isn't practical the way it is for a pure function like `NormalizeName`. `workflow.tdd_mode` is not enabled in `.planning/config.json`, so the plan-level RED/GREEN/REFACTOR gate (`gsd_run check tdd-red-evidence`) does not apply to this plan's `type: execute` frontmatter.
- D-22's merge 404/409 interpretation resolved exactly as the plan's own context-drift note specifies (400 `ErrMergeIntoSelf` when `into` equals the source id; 404 when either id is gone; a renamed target merges under its current stored name) -- no deviation, just confirming the implementation matches the plan text verbatim.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None. One test-authoring mistake (an HTTP end-to-end merge assertion expected the seed tag name `"trap"` instead of the actual seeded `"http-trap"`) was caught and fixed before the task's tests were reported passing -- not a deviation from the plan, just normal test-writing iteration.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `GET /tags` is ready as the autocomplete source and the Manage-tags-dialog data source plan 24-04/24-05 need (D-30).
- `PATCH`/`POST merge`/`DELETE /tags/{id}` give the frontend everything the D-16 alert-dialog confirmations need: the 409 collision body for the merge-confirm dialog, and the `carrier_count` for both the merge and delete toasts.
- No blockers.

---

*Phase: 24-artist-tags-notes*
*Completed: 2026-09-23*

## Self-Check: PASSED

- All 8 key files verified present on disk (`FOUND` for every entry in `files_modified`).
- All 3 task commit hashes verified present in `git log --oneline --all` (057bb2b, f6e2630, 072fb51).
- All acceptance criteria across Tasks 1-3 re-run and passing (list/rename/collision/delete/merge/gate/CSRF tests).
- Plan-level `<verification>` re-run: `sqlc-check` clean (idempotent regenerate, no diff), `go vet`/`golangci-lint` clean, full `go test ./... -count=1` green (repo-wide, `-race` unavailable on this Windows box per documented limitation), `coverage-report` measured 89.85% (floor 80%), `git diff --exit-code -- go.mod go.sum internal/db/migrations` clean (no dependency or schema change).
