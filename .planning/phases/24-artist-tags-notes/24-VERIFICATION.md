---
phase: 24-artist-tags-notes
verified: 2026-10-05T15:59:09Z
status: human_needed
score: 5/5 roadmap success criteria verified; 24-12 gap-closure must-haves 13/13 truths verified, 5/5 prohibitions hold (WR-08 closed)
covered_files:
  - .planning/REQUIREMENTS.md
  - .planning/phases/24-artist-tags-notes/24-01-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-01-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-02-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-02-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-03-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-03-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-04-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-04-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-05-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-05-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-06-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-06-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-07-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-07-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-08-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-08-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-09-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-09-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-10-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-10-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-11-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-11-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-12-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-12-SUMMARY.md
  - cmd/server/main.go
  - docs/adr/0004-per-artist-tag-cap-trigger.md
  - internal/db/migrate_test.go
  - internal/db/migrations/000010_tags_and_notes.down.sql
  - internal/db/migrations/000010_tags_and_notes.up.sql
  - internal/db/migrations/000011_artist_tags_cap_on_update.down.sql
  - internal/db/migrations/000011_artist_tags_cap_on_update.up.sql
  - internal/db/migrations/000012_artist_tags_cap_concurrent_detach.down.sql
  - internal/db/migrations/000012_artist_tags_cap_concurrent_detach.up.sql
  - internal/db/schema_version_test.go
  - internal/db/tags_schema_test.go
  - internal/httpserver/server.go
  - internal/httpserver/tags.go
  - internal/httpserver/watchlist.go
  - internal/tags/normalize.go
  - internal/tags/service.go
  - internal/watchlist/service.go
  - internal/webassets/build/client/assets/watchlist-DcOAtrK5.js
  - internal/webassets/build/client/index.html
  - queries/tags.sql
  - queries/watchlist.sql
  - web/app/components/common/ConfirmDialog.tsx
  - web/app/components/watchlist/ArtistNote.tsx
  - web/app/components/watchlist/ManageTagsDialog.test.tsx
  - web/app/components/watchlist/ManageTagsDialog.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagCombobox.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/lib/api.ts
  - web/app/lib/tags.ts
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
covered_digest: "v1:sha256:e8d9463f96fb0ff26c0225610d5e3ed1c745ef11e04edbebd1f3512b72a08789"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: "5/5 roadmap SCs; 1 gap-closure must-have FAILED (24-11 TAG-06 concurrency edge + prohibition, WR-08)"
  gaps_closed:
    - "24-11 / TAG-06 concurrency edge and its prohibition (WR-08): ManageTagsDialog.load() now captures a generation and drops any superseded settle; mutation success calls invalidateLoad(). The exact reproduction this verifier used last round is now the committed tracer route test and passes. The merge path (unpinned per WR-09) was confirmed by a throwaway test plus a mutation check."
  gaps_remaining: []
  regressions: []
advisory:
  - finding: "WR-09: the invalidateLoad() calls in handleConfirmMerge (ManageTagsDialog.tsx:219) and handleConfirmDelete (:252) are not pinned by any committed test"
    category: other
    reason: >-
      Behavior is correct today. A throwaway test of the reachable merge path
      (rename Enter, close, reopen with load N pending, collision, confirm merge,
      then settle N with the pre-merge list) passed, and it failed with the source
      row revived once line 219 was removed. With both lines 219 and 252 removed,
      the committed suite still passed 44/44, so a future refactor could silently
      reopen WR-08 on the merge path. The delete-path refetch branch is unreachable
      (Delete renders only in status "loaded"; the pending ConfirmDialog blocks
      close/reopen), so that call is defensive. Resolve by committing the merge test
      sketched in 24-REVIEW.md WR-09. This is test-coverage debt, not a goal gap.
    evidence_status: "mutation evidence shows the coverage hole; no behavioral failure"
  - finding: "IN-18: a Manage tags load from an open that is never reopened stays current and can overwrite a fresher route vocabulary"
    category: other
    reason: >-
      The only local change it can undo is a just-created tag (rememberTag), which is
      not in the 24-11/24-12 truth's enumerated list (load, delete, rename, merge).
      The Create item get-or-creates the same normalized tag, so "one tag per name"
      holds. Deleted, renamed, or merged tags cannot be revived this way because those
      mutations require a reopen, and the reopen's load() supersedes the old one.
      Cosmetic only.
    evidence_status: "code reasoning; no must-have affected"
human_verification:
  - test: "End-to-end tags + note CRUD in a real browser through the go:embed build"
    expected: "Create/pick/remove chips; reload; remove + re-add the artist and tags return; add/edit/clear a note and reload; delete a tag in Manage tags and '+ tag' no longer offers it as existing"
    why_human: "Full user flow across reloads and the embedded bundle"
  - test: "Slow-network (DevTools Slow 3G) stale-vocabulary races (24-11 and 24-12 Task 3 human-checks, deferred end-of-phase)"
    expected: "(a) Open '+ tag', then Manage tags, delete a tag before the first GET /tags returns. (b) Open Manage tags, close it, reopen it, then delete or rename a tag and close. In both cases '+ tag' offers only Create for the deleted or old name, never an existing option"
    why_human: "Real network timing through the browser; jsdom cannot reproduce throttled ordering"
  - test: "Combobox popup overflow (24-05 backstop): 30+ tags, a 32-char name, narrow viewport"
    expected: "Popup scrolls inside its own bounds; no clipped option"
    why_human: "Visual layout"
  - test: "Long tag chip at 375px (24-04 backstop)"
    expected: "Chip truncates; title and the x aria-label carry the full name"
    why_human: "Visual layout"
  - test: "Merge confirm title wrap at 375px with two 32-char names (24-06 backstop)"
    expected: "Title wraps, does not truncate"
    why_human: "Visual layout"
  - test: "Manage tags row truncation at 375px (24-06 backstop)"
    expected: "'· {n} artists' stays visible"
    why_human: "Visual layout"
  - test: "UI-SPEC contrast/hit-area audit of chips, x, '+ tag', note pencil"
    expected: "Meets UI-SPEC contrast and 44px-equivalent hit areas"
    why_human: "Visual/accessibility judgment"
  - test: "Delete-vs-attach race (24-02 backstop)"
    expected: "No artist_tags row references a deleted tag (FK cascade guarantees this structurally); confirm or accept"
    why_human: "Concurrency judgment item carried from 24-02"
---

# Phase 24: Artist Tags & Notes Verification Report

**Phase Goal:** The user can label any watchlist artist with free-form tags and a short note right on its Watchlist card, and manage the tag vocabulary globally. There is one tag per name regardless of casing or stray whitespace, and tags stay with the artist across a remove and re-add.
**Verified:** 2026-10-05T15:59:09Z
**Status:** human_needed
**Re-verification:** Yes, after gap-closure round 3 (plan 24-12, closing WR-08)

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Type a tag on a card, pick from autocomplete or create on the fly, see a chip, remove it. Tags persist across reload and remove + re-add | VERIFIED (regression) | 24-12 touched no backend file (`git diff a689ab7` over `*.go`, `internal/db`, `watchlist.tsx`, package files is empty). Full Go suite green against live PG. `TestSchema_WatchlistDeleteKeepsTagLinks` passes. Web suite 341/341. |
| 2 | `Reggaeton ` attaches existing `reggaeton`. Over 32 chars or an 11th tag is refused from UI or API, and the DB refuses it even when the API is bypassed | VERIFIED (regression) | Targeted run of `TagCap*`, `TagNameChecks`, `NoteChecks`, `WatchlistDeleteKeepsTagLinks`, `Delete`, `Merge`, `Rename` across `internal/db`, `internal/tags`, `internal/httpserver`: 52 PASS, 0 SKIP, 0 FAIL. Dev DB `12\|f`. |
| 3 | Rename once, new name everywhere. Renaming onto an existing name confirms naming both tags; nothing merges without it | VERIFIED | Rename/merge dialog and route tests pass. The WR-08 caveat from last round is closed: the new rename and merge route variants show a late first-open GET cannot undo either. |
| 4 | Delete a tag globally after a confirmation stating carrier count; tag disappears from every artist; artists untouched | VERIFIED | Server delete + count confirm unchanged and tested. The stale-response revival path is closed (tracer test, see below). |
| 5 | Add, edit, clear a plain-text note up to 500 chars, shown after reload | VERIFIED (regression) | `TestSchema_NoteChecks` passes; ArtistNote tests are in the green web suite. |

**Score:** 5/5 roadmap truths verified (0 present-but-behavior-unverified).

### Previous gap (WR-08 / TAG-06): CLOSED

The code at `web/app/components/watchlist/ManageTagsDialog.tsx:91-120`:
- `load()` captures `const gen = ++loadGen.current` and sets `loadInFlight`.
- `.then` and `.catch` both `return` while `gen !== loadGen.current`, before any `setTags`, `setStatus`, or `onLoaded`.
- `invalidateLoad()` bumps the generation and calls `load()` only while a load is in flight.
- It is the first statement of the success paths at `:180` (rename, `renamed` branch only), `:219` (merge), and `:252` (delete).

The dialog stays mounted across close (`watchlist.tsx` renders it unconditionally), so the refs survive a close/reopen.

**The exact reproduction from the previous round now passes.** The committed tracer `a Manage tags GET /tags from an earlier open that settles after a delete does not bring the deleted tag back` drives the same sequence:
1. First `listTags` deferred, close, reopen.
2. Delete `reggaeton`, close.
3. Settle the first load with the pre-delete list.
4. "+ tag" offers `drill` and `Create “reggaeton”`, never an existing `reggaeton`.
5. `listTags` is called 2 times, `listWatchlist` once. A reopen makes exactly one more call (3 in total) and has no `reggaeton` row.

### WR-09 judgment: advisory test-coverage debt, not a goal gap

The latest review notes that nothing pins the `invalidateLoad()` calls in `handleConfirmMerge` and `handleConfirmDelete`. I checked this independently.

1. **The merge path behaves correctly. I observed it directly instead of trusting presence.** I wrote a throwaway dialog test using the review's sequence:
   - Rename `rap` to `trap` and press Enter (pending), then close and reopen; load N is pending.
   - The rename resolves as a collision. Confirm the merge.
   - Settle N with the pre-merge list containing `rap`.

   Result: **PASS**. There is no `rap` row, `onLoaded` was last called with `[trap(1)]`, and `listTags` was called 3 times.
2. **Mutation check.** With line 219 (`invalidateLoad()` in merge) removed, the same throwaway test **FAILED**: the revived `rap` row was found. The line does the work the plan claims.
3. **The review's coverage claim is confirmed.** With lines 219 and 252 both removed, the committed dialog and route suites still passed 44/44.
4. **The delete branch is unreachable for refetch.** The Delete button renders only in `"loaded"` status, so `loadInFlight` is false whenever a delete starts. The pending ConfirmDialog also blocks close/reopen. Call 252 is defensive.
5. All throwaway edits were reverted: the test file was deleted and `ManageTagsDialog.tsx` was restored from a backup. `git status` shows only the pre-existing untracked `.planning/milestone.lock`.

**Verdict:** every truth is satisfied by the code as shipped, and I have direct behavioral evidence for the one path no committed test covers. The missing regression pin is a WARNING-grade advisory: committing the WR-09 test sketch is recommended, but it does not block. It does not falsify a must-have, so it does not meet the bar for gaps_found.

### Gap-closure plan 24-12 must-haves

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | TAG-06 concurrency edge: a GET /tags settling after a fresher Manage tags load, delete, rename, or merge never overwrites the route vocabulary | VERIFIED | Load-over-load: tracer, route variants, and the stale-success dialog test. Rename mid-reopen: the no-strand test. Merge mid-reopen: the throwaway test plus the mutation (above). Delete mid-load: unreachable. |
| 2 | WR-08 verifier scenario (offers drill, Create “reggaeton”; listTags 2, listWatchlist 1) | VERIFIED | tracer passes |
| 3 | Reopen makes exactly one more listTags (3); no deleted row | VERIFIED | tracer tail assertions |
| 4 | Late first-open response does not undo a rename (Latino / Create “Latin”) or a merge (trap / Create “rap”); 2 listTags each | VERIFIED | both route variants pass |
| 5 | Older open's late success never re-shows the deleted row or reaches onLoaded; a late failure never shows the error state | VERIFIED | stale-success and stale-failure dialog tests pass |
| 6 | Rename succeeding mid-reopen refetches, no stuck skeleton, last onLoaded carries the new name | VERIFIED | no-strand test passes (`listTags` x3, `onLoaded` last `[Latino]`) |
| 7 | Re-fetch on every open, 3 skeleton rows | VERIFIED | `useEffect([open])` unchanged; skeleton test passes |
| 8 | Non-stale failure still shows `Couldn't load tags.` + Retry | VERIFIED | Retry test passes; the `.catch` current-generation path sets `"error"` |
| 9 | '+ tag' shows Create-only until loaded / on failure; attach works | VERIFIED | 24-11 route tests pass |
| 10 | Existing route and dialog tests pass unchanged | VERIFIED | The route test diff is additions only. The dialog test diff removes only the import line and the `renderDialog` body (refactored to add `setOpen`); no assertion was removed. |
| 11 | TAG-06 idempotency: deleting twice returns 404 | VERIFIED | Ran `TestService_Delete_TwiceReturnsErrTagNotFoundSecondTime` and `TestTags_Delete_NotFoundReturns404`; both PASS |
| 12 | Embedded SPA rebuilt | VERIFIED | A fresh `pnpm run build` followed by `diff -rq web/build/client internal/webassets/build/client` shows them identical (`watchlist-DcOAtrK5.js`) |
| 13 | Whole-phase DoD green | VERIFIED | See spot-checks |

**Prohibitions (all hold):**
- **No silent re-create of a just-deleted tag:** holds. The tracer shows only `Create “reggaeton”` after a late GET.
- **No extra `listTags` on normal flows:** holds. Counts are pinned at 2/3 in the route tests, and the rejected-merge test still expects exactly 2.
- **`watchlist.tsx` not edited:** holds (empty diff since `a689ab7`).
- **No assertions weakened:** holds.
- **24-12 did not edit `REQUIREMENTS.md`:** holds. Commits `94daf0d`, `d39fa79`, `3d07813`, `411006b`, and `fda47f1` do not touch it. The only later change, `4e4acef`, is a Phase 25 quick task that edits WLVW-03.

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `web/app/components/watchlist/ManageTagsDialog.tsx` | VERIFIED | Generation guard plus `invalidateLoad()` at all 3 success paths; comments are 1-2 lines each |
| `web/app/components/watchlist/ManageTagsDialog.test.tsx` | VERIFIED | `deferred`, `setOpen`, and 3 new substantive tests; 21 tests pass |
| `web/app/routes/watchlist.test.tsx` | VERIFIED | tracer + rename/merge variants; 23 tests pass |
| `internal/webassets/build/client` | VERIFIED | Byte-identical to a fresh build |
| Backend artifacts (migrations 000010-000012, tags/watchlist services, httpserver, ADR 0004) | VERIFIED (regression) | Unchanged since last round; suites green |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `load()` generation | `setTags`/`setStatus`/`onLoaded` | `gen !== loadGen.current` early return in `.then` and `.catch` | WIRED (tests + last round's reproduction) |
| rename/merge/delete success | fresh GET after commit | `invalidateLoad()` → `load()` while in flight | WIRED (rename: committed test; merge: throwaway + mutation; delete: unreachable branch) |
| `onLoaded` | `handleTagsLoaded` → route `vocabulary` | now only fed by the newest load | WIRED |
| `loadVocabulary` (route) | `vocabulary` | `vocabGen` (24-11, unchanged) | WIRED |
| `make web` recipe | committed embed tree | copy | WIRED (diff clean) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| TagCombobox | `vocabulary` | GET /tags (route lazy load + newest Manage tags load), patched on create/rename/merge/delete | yes | FLOWING (stale dialog loads are now dropped) |
| TagChips / ManageTagsDialog / ArtistNote | entries, tags, note | GET /watchlist, GET /tags | yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 24-12 dialog + route tests | `vitest run ManageTagsDialog.test.tsx watchlist.test.tsx --coverage.enabled=false` | 44/44 | PASS |
| WR-09 merge mid-reopen (throwaway, deleted) | dialog test per the review's sketch | pass | PASS |
| WR-09 mutation (line 219 removed) | same throwaway test | fails: `rap` revived | PASS (the line is load-bearing) |
| WR-09 coverage hole (lines 219 and 252 removed) | committed suites | 44/44 still pass | CONFIRMED (advisory) |
| Web suite (once) | `pnpm test` | 23 files, 341/341, 92.32/85.49/91.81/94.34 (floor 70) | PASS |
| Prettier / typecheck | `prettier --check`; `pnpm run typecheck` | clean / clean | PASS |
| Embedded bundle | fresh build + `diff -rq` | identical | PASS |
| go vet / golangci-lint | `go vet ./...`; `~/go/bin/golangci-lint run` | clean / 0 issues | PASS |
| sqlc drift | `sqlc generate && git diff --exit-code internal/db/sqlc/` | clean | PASS |
| Full Go suite + coverage (once) | `go test ./... -count=1 -p 1 -coverpkg=<COVER_PKGS>` (live PG via docker compose, no -race on Windows) | every package ok; total 89.33% (floor 80) | PASS |
| Targeted TAG Go tests | `-run 'Delete\|TagCap\|...'` in db/tags/httpserver | 52 PASS, 0 SKIP | PASS |
| Dev DB | `schema_migrations` | `12\|f` | PASS |

### Probe Execution

No probes are declared for this phase, so Step 7c does not apply.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| TAG-01 | 24-01, 02, 04, 05 | SATISFIED | unchanged; suites green |
| TAG-02 | 24-01, 04 | SATISFIED | unchanged |
| TAG-03 | 24-01, 02, 05 | SATISFIED | unchanged |
| TAG-04 | 24-01, 05, 08, 10 | SATISFIED | API/UI/DB caps hold, including under concurrent detach (000012) |
| TAG-05 | 24-02, 06, 11, 12 (vehicle) | SATISFIED | rename/merge flows tested; a late GET can no longer undo them |
| TAG-06 | 24-02, 06, 09, 11, 12 | SATISFIED | WR-08 closed; idempotency tests pass |
| TAG-07 | 24-01, 03, 04 | SATISFIED | unchanged |
| NOTE-01 | 24-03, 07 | SATISFIED | unchanged |

All 8 IDs are claimed by plans, and none are orphaned. **Traceability for the orchestrator:** `REQUIREMENTS.md` still shows all 8 as `[ ]` and "Gaps Found" (lines 11-21, 84-91). After the human verification items are signed off, all 8 can move to Complete.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| ManageTagsDialog.tsx | 219, 252 | load-bearing (219) / defensive (252) calls with no regression test (WR-09) | Warning / advisory | A future refactor could silently reopen WR-08 on the merge path |
| ManageTagsDialog.tsx | 117-120 | unconditional bump is a no-op when idle (IN-17) | Info | Readability |
| ManageTagsDialog.tsx | 122-125 | a never-reopened load stays current (IN-18) | Info / advisory | Cosmetic; can only drop a just-created tag from suggestions |
| ManageTagsDialog.tsx | 127-140 | focus request consumed while the list is unmounted on the rename-mid-reopen path (IN-19) | Info | UI-SPEC focus row (d) in a rare path; not a must-have |
| watchlist.test.tsx | tracer | Close clicked without waiting for the delete confirm to close (IN-20) | Info | Potential flake; passed in every run here |
| 3 test files | — | `deferred<T>()` duplicated (IN-21) | Info | Maintainability |
| files changed by 24-12 | — | TBD/FIXME/XXX/TODO/HACK | none | — |

### Human Verification Required

These are the frontmatter `human_verification` items, carried forward. The only update is that the Slow 3G item now also covers the 24-12 close/reopen path.
1. **End-to-end CRUD in a real browser** through the go:embed build.
2. **Slow 3G stale-vocabulary races:** (a) the 24-11 '+ tag' then delete path; (b) the 24-12 close/reopen then delete/rename path.
3. **Combobox overflow** with 30+ tags and a 32-char name.
4. **375px layouts:** chip truncation, merge title wrap, and Manage tags row count.
5. **UI-SPEC contrast/hit-area audit.**
6. **Delete-vs-attach race confirmation** (24-02 backstop).

### Gaps Summary

There are no gaps. Round 3 closed WR-08:
- Manage tags drops any `GET /tags` that a newer load or a successful mutation has superseded.
- The previous round's deterministic reproduction is now a committed, passing tracer test.
- I confirmed the rename and merge variants of the edge directly, including the merge path the committed suite does not pin.
- All five roadmap success criteria hold, and every 24-12 must-have and prohibition holds.
- The backend, DB cap, and whole-phase gates are green: vet, lint, sqlc, full Go suite at 89.33%, prettier, typecheck, and web suite at 341/341.

What remains:
- **Advisory debt (WR-09):** commit the merge-mid-reopen test from 24-REVIEW.md.
- **Human checks (8 items):** the end-to-end browser flow, real-network timing, and visual items. These set the status to `human_needed`. No later phase is needed, so nothing is deferred.

---

_Verified: 2026-10-05T15:59:09Z_
_Verifier: Claude (gsd-verifier)_
