---
phase: 24-artist-tags-notes
verified: 2026-10-05T01:05:00Z
status: gaps_found
score: 5/5 roadmap success criteria verified; 1 gap-closure must-have FAILED (24-11 TAG-06 concurrency edge + its prohibition, WR-08, reproduced)
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
covered_digest: "v1:sha256:561e72fc88fdca04ebfb46fb5af62e528dcc115c4b49b45643186a83d259167f"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 4/5
  gaps_closed:
    - "SC2 / TAG-04 DB cap under a concurrent detach (CR-01): migration 000012 locks the existing link FOR KEY SHARE; TestSchema_TagCapTrigger_ConcurrentDetachRace (insert re-attach + update move) and TestSchema_Migration000012_DownUpRoundTrip (down reaches 11, up refused at 10) pass against live PG, run by this verifier"
    - "WR-06 (route's own '+ tag' GET /tags settling late): vocabGen guard in loadVocabulary; deferred-promise route test passes"
    - "WR-07 (unchanged-artist subtest did not pin the TG_OP guard): subtest now also changes tag_id; passes"
  gaps_remaining:
    - "Stale GET /tags can still revive a deleted tag in the '+ tag' vocabulary, now via ManageTagsDialog's own unguarded load() -> onLoaded -> handleTagsLoaded (review WR-08). Same symptom as WR-06; declared 24-11 must-have and prohibition both falsified"
  regressions: []
gaps:
  - truth: "24-11 / TAG-06 concurrency edge: a GET /tags response that settles after a fresher local vocabulary change, whether a Manage tags load, delete, rename, or merge, never overwrites the route vocabulary. Prohibition: picking a suggestion must never silently re-create a tag the user just deleted, even when a GET /tags response settles after the delete."
    status: failed
    severity: blocker
    reason: >-
      Reproduced by this verifier with a deterministic throwaway vitest
      (created, run, deleted; working tree left clean): open Manage tags with
      the first listTags deferred, close, reopen (second listTags resolves),
      delete 'reggaeton', close, then settle the first listTags with the old
      list. Typing 'r' in '+ tag' offers 'reggaeton' as an existing option
      (assertion failed: found role=option 'reggaeton'); listTags was called
      exactly twice, so status is 'loaded' and nothing refetches. Root cause:
      ManageTagsDialog.load() (ManageTagsDialog.tsx:92-102) has no staleness
      guard and calls setTags/onLoaded on every settle, even after close;
      handleTagsLoaded (watchlist.tsx:186-190) bumps vocabGen and writes
      unconditionally, so a stale dialog response always wins. attachTag
      sends the name (api.ts:374-383), so picking the stale suggestion makes
      GetOrCreateTag silently re-create the deleted tag. The same overwrite
      can undo a rename or merge. Roadmap SC4 still holds as written (server
      delete, count confirm, artists untouched); what fails is the explicit
      must-have and prohibition the 24-11 gap-closure plan declared for
      exactly this symptom class.
    artifacts:
      - path: "web/app/components/watchlist/ManageTagsDialog.tsx"
        issue: "load() (lines 92-102) applies every listTags settle (setTags, setStatus, onLoaded) with no generation check; useEffect([open]) starts a new load per open without invalidating the previous one; mutation success paths do not invalidate an in-flight load"
      - path: "web/app/routes/watchlist.tsx"
        issue: "handleTagsLoaded (186-190) trusts every onLoaded call; vocabGen only guards the route's own loadVocabulary"
      - path: "web/app/routes/watchlist.test.tsx"
        issue: "The 24-11 test only defers the route's first listTags; no test has two dialog loads in flight"
    missing:
      - "Generation ref in ManageTagsDialog: `const gen = ++loadGen.current` in load(); apply setTags/setStatus/onLoaded (and the error status) only while `gen === loadGen.current`; bump loadGen in the success paths of handleConfirmDelete, handleSaveRename (renamed) and handleConfirmMerge (review WR-08 fix sketch)"
      - "Route (or dialog) test: first Manage tags listTags deferred, close + reopen with the second resolved, delete X, close, settle the first with a list containing X; assert '+ tag' does not offer X as existing, the dialog list does not show X on reopen, and listTags call counts are unchanged"
      - "Optional same pattern for rename/merge (late list must not undo them)"
      - "Rebuild the embedded SPA (make web steps) and re-run the web suite"
      - "OR, if the user accepts the window, add the override below and leave TAG-06 Complete"
human_verification:
  - test: "End-to-end tags + note CRUD in a real browser through the go:embed build"
    expected: "Create/pick/remove chips; reload; remove + re-add the artist and tags return; add/edit/clear a note and reload; delete a tag in Manage tags and '+ tag' no longer offers it as existing"
    why_human: "Full user flow across reloads and the embedded bundle"
  - test: "Slow-network (DevTools Slow 3G) stale-vocabulary race (24-11 Task 2 human-check, deferred end-of-phase)"
    expected: "Open '+ tag', then Manage tags, delete a tag before the first GET /tags returns; the deleted tag is never offered as existing. After the WR-08 fix also try close/reopen Manage tags under throttling"
    why_human: "Real network timing through the browser"
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
**Verified:** 2026-10-05T01:05:00Z
**Status:** gaps_found
**Re-verification:** Yes, after gap-closure round 2 (plans 24-10 and 24-11)

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Type a tag on a card, pick from autocomplete or create on the fly, see a chip, remove it. Tags persist across reload and remove + re-add | VERIFIED (regression) | Full Go suite green against live PG (internal/tags, watchlist, httpserver, db all ok). `TestSchema_WatchlistDeleteKeepsTagLinks` passes. Web suite 335/335. |
| 2 | `Reggaeton ` attaches existing `reggaeton`. Over 32 chars or an 11th tag is refused from UI or API, and the DB refuses it even when the API is bypassed | VERIFIED (gap closed) | 000012 replaces both skip-existing `IF EXISTS` checks with `PERFORM ... FOR KEY SHARE; IF FOUND`. I ran `TestSchema_TagCapTrigger_ConcurrentDetachRace` (insert re-attach + update move): S3 refused 23514/`artist_tags_max_per_artist`, artist at 10, T present, U absent. `TestSchema_Migration000012_DownUpRoundTrip` passes, which proves fail-first in-run: on the 000011 body the same forced race reaches 11, on 000012 it is refused. The test gates on `pg_stat_activity.wait_event_type='Lock'`, not sleeps. `DistinctTagConcurrentAt9`, `SameTagConcurrentAt9`, `RawInsertRefused`, `RawUpdateRefused`, 5 `UpdatePaths` subtests, `TagNameChecks` all pass. Dev DB `12\|f`, public function contains `FOR KEY SHARE`, both triggers present. |
| 3 | Rename once, new name everywhere. Renaming onto an existing name confirms naming both tags; nothing merges without it | VERIFIED (regression) | Merge-confirm Esc test now gated on Cancel focus; ManageTagsDialog suite 18/18 three consecutive runs with coverage disabled. Tag/httpserver merge tests pass. Caveat: a late stale dialog GET can undo a rename in the autocomplete (WR-08, gap 1). |
| 4 | Delete a tag globally after a confirmation stating carrier count; tag disappears from every artist; artists untouched | VERIFIED (as written) | Server delete + count confirm unchanged and tested; the route drops the tag from cards and vocabulary. The stale-response revival path (WR-08) does not undo the server delete, but it violates the 24-11 must-have below. |
| 5 | Add, edit, clear a plain-text note up to 500 chars, shown after reload | VERIFIED (regression) | `TestSchema_NoteChecks` passes; ArtistNote tests in the green web suite. |

**Score:** 5/5 roadmap truths verified (0 present-but-behavior-unverified).

### Gap-closure plan must-haves

**24-10 (TAG-04): 10/10 verified.**
- INSERT re-attach race refused at 10: VERIFIED (ran).
- UPDATE move race refused, B empty: VERIFIED (ran).
- Hole reproduces on 000011 body, closed on 000012: VERIFIED (round-trip test ran).
- Distinct-tag 10th/11th at 9: VERIFIED (ran).
- Prior guarantees: VERIFIED (all prior cap/skip/merge-shape/duplicate-move/000011 round-trip tests ran green).
- WR-07 guard pinned: VERIFIED by code reading. The subtest now changes `tag_id` to a fresh tag, so without the `TG_OP` early return it falls to the count at 10. The SUMMARY's mutation evidence is consistent with that.
- Empty and encoding edges: VERIFIED.
- 000010/000011 unchanged: VERIFIED (`git diff --exit-code 77de93d` clean).
- migration-check, sqlc, versions, dev DB: VERIFIED. migration-check found nothing on 000012 up; sqlc v1.31.1 produced no diff; `expectedSchemaVersionOnDisk = 12`; from-scratch wants `(12, false)`; dev DB is `12|f`.
- ADR amendment: VERIFIED (present, names the tests). Prohibitions hold: no edits to old migrations, the trigger only refuses, and 24-10 did not edit REQUIREMENTS.md.
- Minor: IN-14. `DistinctTagConcurrentAt9` is listed under the 000012 amendment even though it pins the artist lock, not the detach fix. Info only.

**24-11 (TAG-06, TAG-05): 11/12 verified, 1 FAILED.**
- '+ tag' fetch settling after a Manage tags delete is discarded: VERIFIED (route test passes).
- Non-deleted tags still offered; listTags called twice, listWatchlist once: VERIFIED.
- Create still works on listTags failure, and a non-stale failure sets error: VERIFIED (test "still lets Enter attach via Create when listTags rejects", plus the code at watchlist.tsx:205-207).
- Existing route tests pass: VERIFIED.
- **TAG-06 concurrency edge ("a GET /tags response that settles after a fresher local vocabulary change ... never overwrites the route vocabulary"): FAILED.** Reproduced, see gap 1.
- TAG-06 idempotency, TAG-05 adjacency/Esc, empty and ordering edges: VERIFIED. Go suite green; Esc test 3/3 coverage-disabled runs.
- Embedded bundle rebuilt: VERIFIED. A fresh `pnpm run build` is byte-identical to `internal/webassets/build/client` (`diff -rq` clean, `watchlist-FROF-wNL.js`).
- Whole-phase DoD: VERIFIED. go vet, golangci-lint 0 issues, full Go suite, sqlc, prettier and web suite are all green. I did not re-run the coverage gate; the SUMMARY reports 89.33%.
- **Prohibition "picking a suggestion must never silently re-create a tag the user just deleted, even when a GET /tags response settles after the delete": VIOLATED** (same reproduction). `attachTag` posts the name, so `GetOrCreateTag` re-creates the tag.

### WR-08 decision: goal-blocking gap (BLOCKER), not a warning

1. **Reproduced, not inferred.** I wrote a throwaway vitest in `web/app/routes/`, ran it, and deleted it; the working tree is clean. Sequence:
   - Manage tags opens with the first `listTags` deferred, then closes.
   - It reopens and the second `listTags` resolves.
   - Delete `reggaeton`, then close.
   - Settle the first `listTags` with the pre-delete list.
   - Type `r` in "+ tag": `reggaeton` is rendered as an existing option, and `listTags` was called exactly twice.

   The assertion failed: `expected document not to contain element, found <div role="option">reggaeton</div>`.
2. **It falsifies declared must-haves, not just a review nicety.** 24-11 promoted this symptom class to a must-have truth worded for *any* `GET /tags` response, explicitly naming "a Manage tags load", and to a prohibition "even when a GET /tags response settles after the delete." Both fail on the path WR-08 describes. Under the verifier rules, a FAILED must-have truth is a BLOCKER.
3. **Why this differs from the prior round's WR-06 = WARNING.** Then, the symptom was outside any declared must-have, and only the 24-09 prohibition was partially affected. Now the gap-closure plan whose sole purpose was to close this symptom declared it as a must-have and did not close it.
4. **Scope of the blocker.** All five roadmap SCs hold as written. The server delete and the data model are correct, and "one tag per name" still holds: a re-created tag is a single new row. The fix is small (a dialog-side generation ref plus one test). If you judge the window acceptable (it needs out-of-order `GET /tags` responses across a close/reopen plus a delete), use the override below. That turns the status into `human_needed`.

```yaml
overrides:
  - must_have: "24-11 / TAG-06 concurrency edge: a GET /tags response that settles after a fresher local vocabulary change, whether a Manage tags load, delete, rename, or merge, never overwrites the route vocabulary"
    reason: "Only reachable when an earlier Manage tags GET /tags outlives a close/reopen and a delete; server state stays correct and a reload clears it. Accepted for a single-user app; tracked as WR-08."
    accepted_by: "<name>"
    accepted_at: "<ISO timestamp>"
```

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/db/migrations/000012_artist_tags_cap_concurrent_detach.up.sql` | VERIFIED | `CREATE OR REPLACE` of the shared function. Both skip-existing checks use `FOR KEY SHARE`. Keeps the TG_OP early return, the artist `FOR NO KEY UPDATE`, and the unchanged RAISE literal. Applied on dev DB. |
| `internal/db/migrations/000012_...down.sql` | VERIFIED | Restores the `IF EXISTS` body (000011 shape); the round-trip test proves the behavior reverts. |
| `internal/db/tags_schema_test.go` | VERIFIED | `runDetachRace` forces the interleaving with Lock-wait gating and safe cleanup. The race, round-trip, distinct-tag and WR-07 tests are substantive and pass. |
| `internal/db/schema_version_test.go`, `migrate_test.go` | VERIFIED | Pinned to 12 / `(12, false)`. |
| `docs/adr/0004-per-artist-tag-cap-trigger.md` | VERIFIED | Concurrent-detach amendment present. |
| `web/app/routes/watchlist.tsx` | VERIFIED (partial vs gap 1) | `vocabGen` guard is correct for `loadVocabulary`. `handleTagsLoaded` is unguarded against stale dialog loads. |
| `web/app/routes/watchlist.test.tsx` | VERIFIED | Deferred-promise test passes. It does not cover two dialog loads. |
| `web/app/components/watchlist/ManageTagsDialog.test.tsx` | VERIFIED | Focus-gated Esc test; all four original assertions intact. |
| `web/app/components/watchlist/ManageTagsDialog.tsx` | STUB-free, but GAP | `load()` has no staleness guard (gap 1). |
| `internal/webassets/build/client` | VERIFIED | In sync with a fresh build. |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| 000012 function | `artist_tags_cap_trigger` + `artist_tags_cap_update_trigger` | same function OID, no trigger DDL | WIRED (pg_trigger lists both; behavior tests pass through both paths) |
| skip-existing `FOR KEY SHARE` | waits on in-flight DELETE/key-UPDATE, then artist lock + count | row lock | WIRED (race tests) |
| RAISE `artist_tags_max_per_artist` | `ErrTagCapReached` → HTTP 409 | unchanged literal | WIRED (httpserver/tags suites green) |
| `loadVocabulary` | `vocabulary` | `gen === vocabGen.current` | WIRED |
| ManageTagsDialog `load()` → `onLoaded` | `handleTagsLoaded` → `vocabulary` | unguarded | **BROKEN under a stale dialog response (gap 1)** |
| `make web` | committed embed tree | copy | WIRED (diff clean) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| TagCombobox | `vocabulary` | GET /tags (route lazy load + Manage tags loads), patched on create/rename/merge/delete | yes | FLOWING. A stale Manage tags load can clobber it (gap 1). |
| TagChips / ManageTagsDialog / ArtistNote | entries, tags, note | GET /watchlist, GET /tags | yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Cap trigger + schema tests | `go test ./internal/db -v -run 'TestSchema_TagCapTrigger\|TestTrigger\|TestSchema_Migration00001\|TagNameChecks\|NoteChecks\|WatchlistDeleteKeepsTagLinks\|ExpectedSchemaVersion\|FromScratch'` | all PASS, no SKIP (ConcurrentDetachRace 2/2, UpdatePaths 5/5, both round-trips) | PASS |
| Full Go suite (once) | `go test ./... -count=1` (live PG, no -race on Windows) | 25 packages ok | PASS |
| go vet / golangci-lint | `go vet ./...`; `golangci-lint run` | clean / 0 issues | PASS |
| sqlc drift | `sqlc generate && git diff --exit-code internal/db/sqlc/` | clean (v1.31.1) | PASS |
| migration-check | `--mode=scan --files 000012 up/down` | no findings | PASS |
| Dev DB | `schema_migrations`, `pg_proc`, `pg_trigger` | `12\|f`, `FOR KEY SHARE` present, 2 triggers | PASS |
| Web suite | `pnpm test` | 23 files, 335/335, 92.12/84.99/91.79/94.27 | PASS |
| Prettier | `prettier --check` | clean | PASS |
| Esc test stability | ManageTagsDialog suite, coverage disabled, x3 | 18/18 each | PASS |
| Embedded bundle | fresh `pnpm run build` + `diff -rq` | identical | PASS |
| **WR-08 stale dialog load** | throwaway vitest (deleted after) | deleted tag offered as existing | **FAIL (gap 1)** |

### Probe Execution

No probes declared for this phase; Step 7c not applicable.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| TAG-01 | 24-01, 02, 04, 05 | SATISFIED | unchanged; suites green |
| TAG-02 | 24-01, 04 | SATISFIED | unchanged |
| TAG-03 | 24-01, 02, 05 | SATISFIED | unchanged |
| TAG-04 | 24-01, 05, 08, 10 | SATISFIED | CR-01 closed; API/UI/DB caps all hold, including under concurrent detach |
| TAG-05 | 24-02, 06, 11 | SATISFIED | rename/merge flows tested. A stale dialog GET can undo a rename in the autocomplete only (gap 1 side effect) |
| TAG-06 | 24-02, 06, 09, 11 | **BLOCKED (partial)** | Delete + count confirm works. The 24-11 concurrency must-have and prohibition fail (gap 1) |
| TAG-07 | 24-01, 03, 04 | SATISFIED | unchanged |
| NOTE-01 | 24-03, 07 | SATISFIED | unchanged |

All 8 IDs are claimed by plans; none are orphaned. **Traceability inconsistency:** REQUIREMENTS.md marks TAG-06 `[x]` / Complete (lines 16, 89; set in 34f1e10 during 24-09), while TAG-04 and the rest read "Gaps Found". Given this verdict, TAG-04 can move to Complete, and TAG-06 should read Gaps Found until gap 1 is fixed or overridden. The orchestrator should reconcile this.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| web/app/components/watchlist/ManageTagsDialog.tsx | 92-102 | unguarded async state write + callback after close | Blocker | gap 1 (WR-08) |
| web/app/routes/watchlist.tsx | 52-54 | comment says "every local vocabulary change" bumps vocabGen; `rememberTag` does not (IN-12) | Info | Comment accuracy |
| web/app/routes/watchlist.test.tsx | 385-458 | test passes even if the drop/rename/merge bumps are removed (IN-13) | Info | Bumps unpinned |
| docs/adr/0004-per-artist-tag-cap-trigger.md | 85 | cites `DistinctTagConcurrentAt9` as pinning 000012 (IN-14) | Info | Doc precision |
| internal/db/tags_schema_test.go | ~312, ~532 | Lock-wait poll duplicated 3x (IN-15) | Info | Maintainability |
| files changed by 24-10/24-11 | — | TBD/FIXME/XXX/TODO/HACK | none | — |

The carried warnings WR-02, WR-04 and WR-05 are unchanged and are not must-haves.

### Human Verification Required

These are listed in the frontmatter `human_verification` and remain open after gap 1 closes:
- End-to-end CRUD in a real browser.
- The 24-11 slow-network race check.
- Combobox overflow.
- 375px chip, merge title, and Manage tags row layouts.
- Contrast/hit-area audit.
- The delete-vs-attach race confirmation.

### Gaps Summary

Round 2 closed the blocker it targeted:
- Migration 000012 makes the DB tag cap hold under a concurrent detach.
- I confirmed this with forced-interleaving tests, including a round-trip test that shows the hole open on the old body and closed on the new one.
- 24-10's must-haves all hold.
- The route's own stale `+ tag` fetch is now discarded.

One gap remains, and it is the same user-visible symptom as WR-06 reached through a different path. Manage tags' own `GET /tags` has no staleness guard, and the route accepts every dialog load unconditionally. A late response from an earlier open therefore puts a deleted tag back into the autocomplete, and picking it silently re-creates the tag. I reproduced this deterministically. It falsifies the TAG-06 concurrency must-have and the prohibition that 24-11 itself declared, so it is classified as a blocker. Either close it with a dialog-side generation guard and a two-loads-in-flight test, or accept it via the override above. No later phase covers it, so nothing is deferred.

---

_Verified: 2026-10-05T01:05:00Z_
_Verifier: Claude (gsd-verifier)_
