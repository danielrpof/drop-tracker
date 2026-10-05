---
phase: 24-artist-tags-notes
reviewed: 2026-10-04T00:00:00Z
depth: standard
review_kind: incremental
diff_base: 835e021bfca1c6d52875c3f259c37eef8466382a
files_reviewed: 9
files_reviewed_list:
  - docs/adr/0004-per-artist-tag-cap-trigger.md
  - internal/db/migrate_test.go
  - internal/db/migrations/000012_artist_tags_cap_concurrent_detach.down.sql
  - internal/db/migrations/000012_artist_tags_cap_concurrent_detach.up.sql
  - internal/db/schema_version_test.go
  - internal/db/tags_schema_test.go
  - web/app/components/watchlist/ManageTagsDialog.test.tsx
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
findings:
  critical: 0
  warning: 4
  info: 15
  total: 19
status: issues_found
---

# Phase 24: Code Review Report (incremental re-review, gap closure 24-10 / 24-11)

**Reviewed:** 2026-10-04T00:00:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

This incremental re-review covers only the gap-closure commits since `835e021`:
- **24-10:** migration 000012 re-creates `check_artist_tags_max_per_artist()` so that both skip-existing checks lock the existing link `FOR KEY SHARE`. It also adds the forced-interleaving schema tests and amends ADR 0004.
- **24-11:** a `vocabGen` generation guard in `watchlist.tsx` drops stale `loadVocabulary` responses. It also adds a deferred-promise route test and de-flakes the merge-confirm Esc test.

Findings from earlier rounds whose files did not change are carried forward and marked **(carried)**. I did not re-verify them.

**Status of the prior round's findings:**
- **CR-01: resolved.** I traced the 000012 body through every interleaving below. I also ran `TestSchema_TagCapTrigger_ConcurrentDetachRace`, `TestSchema_Migration000012_DownUpRoundTrip`, `TestSchema_TagCapTrigger_DistinctTagConcurrentAt9` and `TestSchema_TagCapTrigger_UpdatePaths` with `-count=3` against the local Postgres. All of them ran, none were skipped, and all passed.
  - **INSERT ... ON CONFLICT re-attach during a detach.** The first `FOR KEY SHARE` conflicts with the DELETE's row lock and waits for it. If the detach commits, the READ COMMITTED re-check drops the deleted row, so `FOUND` is false. The trigger then takes the artist `FOR NO KEY UPDATE` lock, re-checks, and counts the 9 committed links. A third attach blocks on the artist lock, then counts 10 and is refused. If the detach rolls back, the row is found and stays locked until commit, so it cannot disappear before the `ON CONFLICT` arbitration.
  - **UPDATE move during a detach.** Same path. The moved row's own `FOR UPDATE` lock is on `(B,T)`, not on `(A,T)`, so the trigger never waits on itself.
  - **The third attach goes first.** It counts the uncommitted delete as still present (10) and is conservatively refused. Or, once the delete has committed, it wins the artist lock, and the re-attach later counts 10 and is refused. The cap holds either way.
  - **Two same-tag attaches at 9.** An uncommitted insert is invisible, so it is not locked. Behavior is unchanged, as the ADR states.
  - **Deadlocks.** I found no new lock cycle. The lock order inside the trigger is always "link KEY SHARE, then return" or "artist, then link KEY SHARE". `Attach` takes the `tags` row lock in `GetOrCreateTag` before `AttachTag`. `Merge` takes both `tags` locks in id order before it touches any link. `Detach` and `DeleteTagCountingCarriers` never lock `artists`. The only new waits are a detach, or a merge's link rewrite, queuing behind an in-flight re-attach that holds `KEY SHARE` on that same link. These are short, single-statement waits, not cycles.
  - **Rollback (N-1).** The previous binary is unaffected, because `CREATE OR REPLACE FUNCTION` is additive. The down file is byte-identical to 000011's function body.
- **WR-06: partially addressed.** Stale responses from the route's own `loadVocabulary` are now dropped. But the route still applies every `GET /tags` that `ManageTagsDialog` issues, unconditionally through `onLoaded` → `handleTagsLoaded`, so a stale dialog fetch can still put a deleted tag back into the autocomplete (WR-08).
- **WR-07: resolved.** The `unchanged-artist` subtest now changes `tag_id` too. Without the `TG_OP` guard, the fresh tag would fall through to the count at 10 and be refused, so the subtest now pins the guard.
- **IN-09: resolved.** `TestSchema_TagCapTrigger_DistinctTagConcurrentAt9` is the forced, DB-level distinct-tag 10th/11th race.

**Test reliability.** The new Go race tests are deterministic. Each gate waits until `pg_stat_activity.wait_event_type = 'Lock'` for a specific backend PID, not on a sleep. Every exit path releases S2 and rolls back S1 before `wg.Wait()`, so a failure cannot hang `pool.Close`. At most three connections are held at once, which is under the test pool's `MaxConns` of 4 (`poolMaxConnsForWorkers(0)`). The 5 s poll deadlines are generous for a 50 ms poll. I found no flakiness risk beyond the duplication noted in IN-15. Each test drops and recreates its isolated schema, so the round-trip test's `down.sql` cannot leak into other tests.

## Warnings

### WR-08: Manage tags' own `GET /tags` is still unguarded, so a stale dialog load can bring a deleted tag back (residual WR-06)

**File:** `web/app/routes/watchlist.tsx:186-190` (`handleTagsLoaded`), fed by `web/app/components/watchlist/ManageTagsDialog.tsx:92-107`
**Issue:** `handleTagsLoaded` bumps `vocabGen` and then writes the dialog's list and `"loaded"` unconditionally. The dialog's `load()` has no staleness guard either: it calls `setTags`, `setStatus` and `onLoaded` whenever a request settles, including after the dialog has closed. Its `useEffect([open])` starts a new load on every open without discarding the previous one. On a slow network:
1. Open Manage tags. GET A starts.
2. Close it, then reopen it. GET B starts.
3. B settles. The list renders, and `handleTagsLoaded(B)` runs.
4. Delete tag X. `dropTagFromEntries` bumps the generation and filters X out.
5. A settles. The dialog runs `setTags(A)`, so X reappears in the list, and `onLoaded(A)` → `handleTagsLoaded` puts X back into the route vocabulary with status `"loaded"`.

`"+ tag"` never refetches once the status is `"loaded"`. So X is offered as an `existing` suggestion, and picking it silently re-creates the tag through `GetOrCreateTag`. That is the original WR-01/WR-06 symptom, reached through the one `GET /tags` path the guard does not cover. A rename or merge applied between steps 3 and 5 is undone the same way. The new route test does not cover this, because it only makes the route's own first `listTags` slow.
**Fix:** Guard the dialog's loads with their own generation, and invalidate any in-flight load when a mutation succeeds:
```tsx
// ManageTagsDialog.tsx
const loadGen = useRef(0)
function load() {
  const gen = ++loadGen.current
  setStatus("loading")
  listTags()
    .then((result) => {
      if (gen !== loadGen.current) return
      const sorted = sortTags(result)
      setTags(sorted)
      setStatus("loaded")
      onLoaded(sorted)
    })
    .catch(() => {
      if (gen === loadGen.current) setStatus("error")
    })
}
// in the success paths of handleConfirmDelete / handleSaveRename (renamed) /
// handleConfirmMerge: loadGen.current++
```
Add a route or dialog test with two `listTags` calls in flight (close and reopen), where the older call settles after a delete.

### WR-02 (carried): A background `refresh()` overwrites optimistic tag changes, and nothing reconciles afterward

**File:** `web/app/routes/watchlist.tsx:56-61`, `web/app/components/watchlist/TagChips.tsx:109-131, 173-183`
**Issue:** `refresh()` still replaces `entries` wholesale. A `GET /watchlist` that returns before an in-flight detach or attach commits undoes the chip change, and nothing corrects it later.
**Fix:** Track in-flight attaches and detaches per entry and re-apply them inside `refresh`'s updater. Alternatively, sequence refreshes with a request counter, and run a reconciling refresh after each tag mutation settles.

### WR-04 (carried): Client length caps and counters count UTF-16 code units, but the server counts code points

**File:** `web/app/components/watchlist/TagCombobox.tsx:99-105, 135, 143-152`; `web/app/components/watchlist/ArtistNote.tsx:123-127, 175, 190`; `web/app/components/watchlist/ManageTagsDialog.tsx:314-321`
**Issue:** `maxLength` and `.length` count an astral or decomposed character as 2. Legal emoji or CJK-extension tags and notes get cut short, and the counters are wrong.
**Fix:** Use `[...s.normalize("NFC")].length`, and enforce the cap in `onChange` instead of with `maxLength`.

### WR-05 (carried): `t.Fatalf` is called from non-test goroutines in the concurrent cap-race test

**File:** `internal/httpserver/tags_test.go:861-869, 909-916`
**Issue:** `postTag` calls `t.Fatalf` inside goroutines that `TestTags_Attach_ConcurrentCapRace` spawns. `FailNow` from another goroutine exits only that goroutine, so a transport failure shows up as a confusing status mismatch.
**Fix:** Return `(int, error)`. In the goroutines, call `t.Errorf` and return, then check `t.Failed()` after `wg.Wait()`.

## Info

### IN-12: The `vocabGen` comment overstates what bumps it, and the guard depends on an unstated invariant

**File:** `web/app/routes/watchlist.tsx:52-54, 195-216`
**Issue:** The comment says the ref is "Bumped by every local vocabulary change", but `rememberTag` does not bump it, and must not. A bump while the status is `"loading"` makes the settle a no-op, which leaves the status stuck at `"loading"` with `vocabulary === null`. `loadVocabulary` only refetches from `"idle"`/`"error"`, so the combobox would never load. Today this is safe only because drop, rename and merge always run after `handleTagsLoaded` has set `"loaded"`. That invariant appears only in the plan, not in the code. Separately, `rememberTag` is a no-op while the first load is in flight (`v === null`). If that `GET` was served before the create committed, the new tag is missing from the vocabulary until Manage tags is opened. The effect is cosmetic: the server's `GetOrCreateTag` still resolves the existing tag.
**Fix:** Reword the comment to "Bumped by every write that supersedes an in-flight GET /tags", and add one line noting that a bump must also leave the status non-`"loading"`. Optionally, buffer tags remembered during loading and merge them into the settled list.

### IN-13: The new route test does not pin the bumps in drop, rename and merge

**File:** `web/app/routes/watchlist.test.tsx:385-458`, `web/app/routes/watchlist.tsx:123, 140, 158`
**Issue:** In the tested sequence, `handleTagsLoaded` bumps the generation before the delete. The bump in `handleTagsLoaded` alone discards the stale response, so the test still passes if the three `vocabGen.current++` lines in `dropTagFromEntries`, `renameTagInEntries` and `mergeTagInEntries` are deleted. Given the invariant in IN-12, those bumps are currently redundant. Nothing protects them if the invariant changes later.
**Fix:** Either document them as defensive, or drop them and rely on `handleTagsLoaded` with a comment explaining why.

### IN-14: The ADR cites a test that does not pin the 000012 change

**File:** `docs/adr/0004-per-artist-tag-cap-trigger.md:85`
**Issue:** `TestSchema_TagCapTrigger_DistinctTagConcurrentAt9` passes on the 000011 body too. It pins Decision item 2 (the artist lock), not the concurrent-detach fix. Listing it under the 000012 amendment suggests it guards that fix.
**Fix:** Move it to the Consequences bullet about concurrent 10th/11th attaches, or drop it from the amendment's "Pinned by" line.

### IN-15: The `pg_stat_activity` Lock-wait poll is duplicated three times

**File:** `internal/db/tags_schema_test.go` (`TestSchema_TagCapTrigger_SameTagConcurrentAt9`, `runDetachRace` `waitingOnLock` ~l.312-318, `TestSchema_TagCapTrigger_DistinctTagConcurrentAt9` ~l.532-545)
**Issue:** The same query-plus-deadline loop is hand-written three times, with slightly different failure messages. A fix to one copy, such as handling a PID that has vanished, will not reach the others.
**Fix:** Extract `waitForLockWait(t, ctx, pool, pid, within time.Duration) bool` and use it in all three.

### IN-16: The de-flaked Esc test hides a real sub-frame window where the rename input still handles keys

**File:** `web/app/components/watchlist/ManageTagsDialog.test.tsx:369-373`; `web/app/components/watchlist/ManageTagsDialog.tsx:303-312`
**Issue:** Gating on Cancel focus is a correct fix for the test. But the window it avoids exists in the product too. Between the moment the merge `ConfirmDialog` mounts and the moment it takes focus, the rename input's `onKeyDown` still runs:
- **Esc:** calls `cancelRename` with `stopPropagation`. The rename row closes while the merge confirm stays open, and that confirm's `finalFocus` (`renameSaveRef`) is now unmounted.
- **Enter:** `renamePending` is already false, so a second `renameTag` request is sent.

Only a very fast double key press can hit this.
**Fix:** Ignore Enter and Esc in the rename input while `collisionTarget !== null`.

### IN-11 (carried, widened): Comments still name migration 000010 as where the cap trigger is defined

**File:** `queries/tags.sql:18`, `internal/tags/service.go:14, 35`
**Issue:** The function in effect is now 000012's. It covers `UPDATE OF artist_id` and locks the existing link. The literal `10` now also appears in 000012's body (see IN-03).
**Fix:** Refer to "the artist_tags cap trigger (ADR 0004)" rather than to a migration number.

### IN-01 (carried): The migration comment says tag identity is accent-insensitive, but it is not
**File:** `internal/db/migrations/000010_tags_and_notes.up.sql:14`
**Fix:** Change it to "Case-insensitive identity".

### IN-02 (carried): The `attachTag` doc comment swaps the meanings of 201 and 200
**File:** `web/app/lib/api.ts:371-373`
**Fix:** Swap the two status codes in the comment.

### IN-03 (carried): Limit values are hard-coded in several places instead of shared
**File:** `TagChips.tsx:18-20`, `ManageTagsDialog.tsx:314, 319-321`, `ArtistNote.tsx:123-126, 175, 187-190`, `internal/httpserver/tags.go:351`, `internal/tags/service.go:28`
**Fix:** Import `MAX_TAGS_PER_ARTIST` and `MAX_TAG_LENGTH`. Build the messages from `MaxNameRunes` and `MaxTagsPerArtist`.

### IN-04 (carried): `DeleteTagCountingCarriers` can under-report when an attach commits at the same moment
**File:** `queries/tags.sql:67-75`
**Fix:** Lock the tag row `FOR UPDATE` before counting, or document the count as best-effort.

### IN-05 (carried): A second rename collision during the retry returns 500 instead of 409
**File:** `internal/tags/service.go:203-210`
**Fix:** Map `tags_name_lower_idx` to a `CollisionError`, or loop back into the collision branch.

### IN-06 (carried): `watchlist_note_not_blank` is mapped to "note contains invalid characters"
**File:** `internal/watchlist/service.go:532-533`
**Fix:** Give it its own sentinel, or treat it as a clear.

### IN-07 (carried): A merge target that is not in the dialog's loaded list disappears from it
**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:200-211`
**Fix:** When the target is missing, insert `merged` into the list. Reset the rename state when `open` becomes false.

### IN-08 (carried): The client sorts tags in a different order than the server
**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:47-54`, `web/app/lib/tags.ts:79-84`
**Fix:** Compare `toLowerCase()` keys with a plain `<`, then fall back to id.

### IN-10 (carried): Earlier comments break the project's 1-3 line comment rule
**File:** for example `queries/watchlist.sql:11-25, 62-66`, `internal/watchlist/service.go:382-387, 405-411`, `internal/httpserver/watchlist.go:136-140`, `ArtistNote.tsx:15-21, 35-41`, `TagChips.tsx:22-27`
**Issue:** This round's additions follow the rule: the 000012 header is 3 lines, and the `vocabGen` and test comments are 1-2 lines. The older blocks remain.
**Fix:** Cut each block down to its core intent and one reference.

---

_Reviewed: 2026-10-04T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
