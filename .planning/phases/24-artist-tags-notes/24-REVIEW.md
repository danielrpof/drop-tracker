---
phase: 24-artist-tags-notes
reviewed: 2026-09-24T00:00:00Z
depth: standard
review_kind: incremental
diff_base: 43c1d568d8411862cc528285548a579ca6fc3506
files_reviewed: 7
files_reviewed_list:
  - internal/db/migrations/000011_artist_tags_cap_on_update.up.sql
  - internal/db/migrations/000011_artist_tags_cap_on_update.down.sql
  - internal/db/tags_schema_test.go
  - internal/db/migrate_test.go
  - internal/db/schema_version_test.go
  - web/app/routes/watchlist.tsx
  - web/app/routes/watchlist.test.tsx
findings:
  critical: 1
  warning: 5
  info: 11
  total: 17
status: issues_found
---

# Phase 24: Code Review Report (incremental re-review)

**Reviewed:** 2026-09-24T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

This is an **incremental re-review** covering only the gap-closure changes since `43c1d56`:
- **24-08:** migration 000011 adds `artist_tags_cap_update_trigger BEFORE UPDATE OF artist_id`, plus the schema tests for it.
- **24-09:** `dropTagFromEntries` now also removes a deleted tag from the route's autocomplete vocabulary.

The earlier full review (2026-09-23, 42 files) is replaced by this report. Its findings that were out of scope here are carried forward below, marked **(carried)**. I did not re-verify them, but none of their files changed since `diff_base`.

**Status of the prior findings in scope:**
- **WR-01: resolved.** `watchlist.tsx:128` filters `vocabulary` on delete. The new route test fails without the fix: the deleted tag is dropped from Drake's chips, so the old code would have offered it as an `existing` option. It also passes with the fix (19/19 in `watchlist.test.tsx`). One narrower case still exists: a stale in-flight `listTags` response can put the deleted tag back (WR-06).
- **WR-03: resolved as stated.** A raw `UPDATE ... SET artist_id` onto an artist with 10 links is now refused, a merge's `SET tag_id` does not fire the trigger, and the down/up pair round-trips. The targeted `internal/db` tests pass against the local Postgres. **However, SC2 ("the database refuses an 11th link") still does not hold.** The shared trigger function, first written in 000010 and copied unchanged into 000011, has a concurrency hole that reaches 11 links through INSERT and UPDATE alike. I reproduced it (CR-01). The prior review traced the trigger's re-check and called it sound. That was wrong: the re-check only guards against concurrent *inserts* of the same link, not concurrent *deletes* of it.

## Critical Issues

### CR-01: A concurrent detach of the same link lets the cap trigger skip its lock and count, so an artist reaches 11 tags

**File:** `internal/db/migrations/000011_artist_tags_cap_on_update.up.sql:13-27` (same logic in `000010_tags_and_notes.up.sql:38-52`)
**Issue:** Both existence short-circuits use a plain `IF EXISTS (SELECT 1 FROM artist_tags WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id)`. That check sees a link another transaction is deleting but has not committed. The trigger returns `NEW` without taking the artist lock or counting, on the assumption that "the link exists, so no net increase". Then:
- the INSERT's `ON CONFLICT` check, or the UPDATE's PK check, waits for the deleter;
- the deleter commits, so the conflict is gone and the row is written;
- this transaction never held the artist lock, so a third transaction that attaches a different tag in between counts 9 committed links and succeeds.

Detach (`DELETE FROM artist_tags ...`) takes no lock on `artists`, so nothing serializes it with the trigger. BEFORE ROW triggers are not re-run after the conflict wait.

Reproduced on the project's `postgres:16` container, in a scratch schema using the exact function body from 000011. Artist 1 started with 10 links, and artist 2 carried tag 1:
```
S1: BEGIN; DELETE FROM artist_tags WHERE artist_id=1 AND tag_id=1; <sleep>; COMMIT;
S2: BEGIN; INSERT INTO artist_tags VALUES (1,1) ON CONFLICT DO NOTHING; <sleep>; COMMIT;
    -- or: UPDATE artist_tags SET artist_id=1 WHERE artist_id=2 AND tag_id=1
S3 (after S1 commits, before S2 commits): INSERT INTO artist_tags VALUES (1,11);
=> artist 1 ends with 11 links, for both the INSERT and the UPDATE variant of S2.
```
The same interleaving is reachable through the API: a detach of T, a re-attach of T, and an attach of U, all concurrent on one artist. The window is small, but the whole purpose of this trigger (ADR 0004, SC2) is that it cannot be bypassed.
**Fix:** Lock the existing link row instead of just testing that it is visible. `FOR KEY SHARE` conflicts with `DELETE` and with a key-changing `UPDATE`. So the check waits for an in-flight delete, re-evaluates under READ COMMITTED, and falls through to the artist lock and count if the row is gone. If the row survives, the lock holds it for the rest of the transaction. Apply this to both checks:
```sql
PERFORM 1 FROM artist_tags
 WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id
   FOR KEY SHARE;
IF FOUND THEN
    RETURN NEW;
END IF;

PERFORM 1 FROM artists WHERE id = NEW.artist_id FOR NO KEY UPDATE;

PERFORM 1 FROM artist_tags
 WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id
   FOR KEY SHARE;
IF FOUND THEN
    RETURN NEW;
END IF;
```
I verified this fix against the same three-session script: S3 is refused with `artist_tags_max_per_artist`, and the artist stays at 10 for both variants. The existing same-tag-at-9 behavior is unaffected, because uncommitted inserts are invisible and so not locked. 000011 has not been released, so it can be edited in place per `internal/db/migrations/README.md`. Any database already at version 11 will not re-run it, though, so a new 000012 that re-creates the function is the safer option. Add a forced `pg_stat_activity`-gated test in the style of `TestSchema_TagCapTrigger_SameTagConcurrentAt9` that pins this interleaving. Amend ADR 0004's Consequences section to cover it.

## Warnings

### WR-06: A stale in-flight vocabulary fetch can put a deleted tag back into the autocomplete

**File:** `web/app/routes/watchlist.tsx:188-197` (interacts with `:128` and `:180-183`)
**Issue:** This is what remains of WR-01. `loadVocabulary` writes its result unconditionally when the request settles. Suppose the user opens "+ tag" (the request starts, status `"loading"`), then opens Manage tags and deletes a tag before that first `listTags()` resolves, which takes a slow network. The late response then overwrites both `handleTagsLoaded`'s fresher list and the delete filter with a list that still contains the deleted tag, and sets status to `"loaded"`, so it is never refetched. Picking that suggestion silently re-creates the tag, which is exactly the WR-01 symptom. The same overwrite also undoes a rename or merge applied during the window.
**Fix:** Discard stale responses. For example, bump a generation ref in `handleTagsLoaded`, `dropTagFromEntries`, `renameTagInEntries` and `mergeTagInEntries`, and have `loadVocabulary` apply its result only if the generation still matches what it saw when it started:
```tsx
const vocabGen = useRef(0)
function loadVocabulary() {
  if (vocabularyStatus !== "idle" && vocabularyStatus !== "error") return
  const gen = ++vocabGen.current
  setVocabularyStatus("loading")
  listTags()
    .then((s) => {
      if (gen !== vocabGen.current) return
      setVocabulary(s.map(({ id, name }) => ({ id, name })))
      setVocabularyStatus("loaded")
    })
    .catch(() => { if (gen === vocabGen.current) setVocabularyStatus("error") })
}
// in handleTagsLoaded / drop / rename / merge: vocabGen.current++
```

### WR-07: The "unchanged-artist" subtest passes even without the new short-circuit, so it pins nothing

**File:** `internal/db/tags_schema_test.go:447-462` (guards `000011_artist_tags_cap_on_update.up.sql:9-11`)
**Issue:** `UPDATE artist_tags SET artist_id = artist_id` leaves `(artist_id, tag_id)` unchanged. The function's first `EXISTS` check finds the row's own pre-update version and returns `NEW` before it ever reaches the count. Delete the `TG_OP = 'UPDATE' AND NEW.artist_id = OLD.artist_id` guard and this subtest still passes. The guard is the only new logic in the function. It matters only when `artist_id` is in the SET list but `tag_id` changes, for example `SET artist_id = artist_id, tag_id = $x` at 10 links. Without the guard, that statement is falsely refused as an 11th link.
**Fix:** Make the subtest change `tag_id` as well, so it fails without the guard:
```go
tag, err := pool.Exec(ctx,
    "UPDATE artist_tags SET artist_id = artist_id, tag_id = $2 WHERE artist_id = $1 AND tag_id = $3",
    artist, freshTagID, oneOfTheTenTagIDs)
// want: no error, RowsAffected == 1, artist still has 10 links
```

### WR-02 (carried): A background `refresh()` overwrites optimistic tag changes, and nothing reconciles afterward

**File:** `web/app/routes/watchlist.tsx:53-58`, `web/app/components/watchlist/TagChips.tsx:109-131, 173-183`
**Issue:** `refresh()` still replaces `entries` wholesale. A `GET /watchlist` that returns before an in-flight detach or attach commits undoes the chip change, and nothing corrects it later. 24-09 did not touch this.
**Fix:** Track in-flight attaches and detaches per entry and re-apply them inside `refresh`'s updater. Or sequence refreshes with a request counter and run a reconciling refresh after each tag mutation settles (see the prior report for the snippet).

### WR-04 (carried): Client length caps and counters count UTF-16 code units, but the server counts code points

**File:** `web/app/components/watchlist/TagCombobox.tsx:99-105, 135, 143-152`; `web/app/components/watchlist/ArtistNote.tsx:123-127, 175, 190`; `web/app/components/watchlist/ManageTagsDialog.tsx:314-321`
**Issue:** `maxLength` and `.length` count astral and decomposed characters as 2. Legal emoji or CJK-extension tags and notes get cut short, and the counters are wrong.
**Fix:** Use `[...s.normalize("NFC")].length`, and enforce the cap in `onChange` instead of with `maxLength`.

### WR-05 (carried): `t.Fatalf` is called from non-test goroutines in the concurrent cap-race test

**File:** `internal/httpserver/tags_test.go:861-869, 909-916`
**Issue:** `postTag` calls `t.Fatalf` inside goroutines that `TestTags_Attach_ConcurrentCapRace` spawns. `FailNow` from another goroutine only exits that goroutine, so a transport failure shows up as a confusing status mismatch.
**Fix:** Return `(int, error)`. In the goroutines, call `t.Errorf` and return, then check `t.Failed()` after `wg.Wait()`.

## Info

### IN-11: Comments still name migration 000010 as where the cap trigger is defined

**File:** `queries/tags.sql:18`, `internal/tags/service.go:14, 35`
**Issue:** The trigger function now in effect is the one 000011 re-created, and the cap now also covers `UPDATE OF artist_id`. These comments point readers to 000010 only. The literal `10` is now also duplicated in 000011's function body (see IN-03).
**Fix:** Refer to "the artist_tags cap triggers (000010/000011, ADR 0004)", or just to ADR 0004.

### IN-01 (carried): The migration comment says tag identity is accent-insensitive, but it isn't
**File:** `internal/db/migrations/000010_tags_and_notes.up.sql:14`
**Fix:** Change it to "Case-insensitive identity".

### IN-02 (carried): The `attachTag` doc comment swaps the 201 and 200 meanings
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

### IN-07 (carried): A merge target that isn't in the dialog's loaded list disappears from it
**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:200-211`
**Fix:** When the target is missing, insert `merged` into the list. Reset the rename state when `open` becomes false.

### IN-08 (carried): The client re-sorts tags with a different order than the server
**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:47-54`, `web/app/lib/tags.ts:79-84`
**Fix:** Compare `toLowerCase()` keys with a plain `<`, then fall back to id.

### IN-09 (carried): The ADR-mandated distinct-tag 10th/11th race is tested only as an unforced HTTP race
**File:** `internal/httpserver/tags_test.go:877-933`, `internal/db/tags_schema_test.go:173-252`
**Fix:** Add a forced two-transaction variant using distinct tag ids. It pairs naturally with CR-01's new forced test.

### IN-10 (carried): New comments break the project's 1-3 line comment rule
**File:** e.g. `queries/watchlist.sql:11-25, 62-66`, `internal/watchlist/service.go:382-387, 405-411`, `internal/httpserver/watchlist.go:136-140`, `ArtistNote.tsx:15-21, 35-41`, `TagChips.tsx:22-27`
**Issue:** The gap-closure changes themselves follow the rule: the 000011 header and the new `dropTagFromEntries` comment are both short. The earlier blocks remain.
**Fix:** Cut each block down to its core intent and one reference.

---

_Reviewed: 2026-09-24T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
