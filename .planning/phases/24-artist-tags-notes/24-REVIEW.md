---
phase: 24-artist-tags-notes
reviewed: 2026-09-23T00:00:00Z
depth: standard
files_reviewed: 42
files_reviewed_list:
  - cmd/server/main.go
  - internal/authgate/gate_test.go
  - internal/db/migrate_test.go
  - internal/db/migrations/000010_tags_and_notes.down.sql
  - internal/db/migrations/000010_tags_and_notes.up.sql
  - internal/db/schema_version_test.go
  - internal/db/tags_schema_test.go
  - internal/httpserver/server.go
  - internal/httpserver/tags.go
  - internal/httpserver/tags_test.go
  - internal/httpserver/watchlist.go
  - internal/httpserver/watchlist_test.go
  - internal/poller/poller_test.go
  - internal/tags/normalize.go
  - internal/tags/normalize_test.go
  - internal/tags/service.go
  - internal/tags/service_test.go
  - internal/watchlist/service.go
  - internal/watchlist/service_test.go
  - internal/watchlist/zip_test.go
  - queries/tags.sql
  - queries/watchlist.sql
  - web/app/components/common/ConfirmDialog.test.tsx
  - web/app/components/common/ConfirmDialog.tsx
  - web/app/components/history/HistoryFilters.test.tsx
  - web/app/components/watchlist/ArtistNote.test.tsx
  - web/app/components/watchlist/ArtistNote.tsx
  - web/app/components/watchlist/ManageTagsDialog.test.tsx
  - web/app/components/watchlist/ManageTagsDialog.tsx
  - web/app/components/watchlist/PreferenceToggles.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/components/watchlist/TagChips.test.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagCombobox.test.tsx
  - web/app/components/watchlist/TagCombobox.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/lib/api.test.ts
  - web/app/lib/api.ts
  - web/app/lib/tags.test.ts
  - web/app/lib/tags.ts
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
findings:
  critical: 0
  warning: 5
  info: 10
  total: 15
status: issues_found
---

# Phase 24: Code Review Report

**Reviewed:** 2026-09-23T00:00:00Z
**Depth:** standard
**Files Reviewed:** 42
**Status:** issues_found

## Summary

I reviewed migration 000010, the tag and note queries, the `internal/tags` and `internal/watchlist` services, the HTTP handlers and route wiring, and the React tag chips, combobox, note editor, Manage tags dialog and route state.

The backend concurrency design holds up when traced. Attach and merge both lock the tag row first (`GetOrCreateTag`'s `DO UPDATE` and `LockTagsForMerge`), so they serialize. The trigger re-checks existence after taking the artist lock, and it uses a fresh snapshot per statement under READ COMMITTED. Merge never inserts. No blockers were found.

The defects are mostly in client state handling:
- A tag deleted in Manage tags stays in the "+ tag" autocomplete.
- A background `refresh()` can undo optimistic chip changes, and nothing corrects them afterward.
- Client length limits count UTF-16 code units. The server counts code points.

On the database side, the cap trigger fires on `INSERT` only. An `UPDATE ... SET artist_id` can still push an artist past 10 links, so SC2's "non-bypassable" claim does not fully hold.

## Warnings

### WR-01: Deleting a tag in Manage tags leaves it in the "+ tag" autocomplete, and picking it silently re-creates the tag

**File:** `web/app/routes/watchlist.tsx:121-130` (wired at `:331`)
**Issue:** `onDeleted` is bound to `dropTagFromEntries`, which only filters `entries`. The route's `vocabulary` state is never updated. Rename (`renameTagInEntries`, line 145) and merge (`mergeTagInEntries`, lines 168-174) both patch `vocabulary`, but delete does not. `vocabularyStatus` stays `"loaded"`, so `loadVocabulary` never refetches either. After a delete, every row's "+ tag" combobox keeps showing the deleted tag as an `existing` suggestion (`buildTagSuggestions`, `tags.ts:64-72`). Picking it sends `attachTag(entryId, name)`, which get-or-creates a new tag with a new id, resurrecting the tag the user just deleted. The UI never shows the "Create" affordance that would warn them.
**Fix:**
```tsx
function dropTagFromEntries(tagId: number) {
  setEntries((rows) =>
    rows ? rows.map((r) => ({ ...r, tags: r.tags.filter((t) => t.id !== tagId) })) : rows
  )
  setVocabulary((v) => (v ? v.filter((t) => t.id !== tagId) : v))
}
```

### WR-02: A background `refresh()` overwrites optimistic tag changes, and nothing reconciles afterward

**File:** `web/app/routes/watchlist.tsx:53-58`, `web/app/components/watchlist/TagChips.tsx:109-131, 173-183`
**Issue:** `refresh()` replaces `entries` wholesale with `setEntries(rows)`. Callers include the add-from-search success and 409 paths, Undo, remove-failure and Retry. `D-24` protects pending *adds*, which live in TagChips' local `pending` state. It does not protect anything already written into `entries`:
- **Removal lost:** the user clicks a chip's ×, so `removeTag` runs and `detachTag` starts. A `GET /watchlist` answered before the DELETE commits brings the chip back. When the detach later succeeds, nothing removes it again, so the row shows a tag the artist no longer carries.
- **Attach lost:** `attachTag` resolves and `addTag` inserts the chip. A `GET /watchlist` answered before the attach commit then drops it, and the chip is gone until the next refresh.
**Fix:** Pick one of these:
- Track in-flight detaches and attaches at route level (a per-entry set of pending-remove and pending-add tag ids), and re-apply them inside `refresh`'s `setEntries` updater.
- Or sequence refreshes with a request counter, and after each tag mutation settles, run a reconciling refresh that is issued *after* it:
```tsx
const refreshSeq = useRef(0)
const refresh = useCallback(() => {
  const seq = ++refreshSeq.current
  listWatchlist().then((rows) => { if (seq === refreshSeq.current) setEntries(rows) })
}, [])
// and in TagChips: detachTag(...).then(() => actions.refresh?.())
```

### WR-03: The cap trigger fires on INSERT only, so `UPDATE artist_tags SET artist_id = ...` bypasses the 10-tag cap

**File:** `internal/db/migrations/000010_tags_and_notes.up.sql:64-67`
**Issue:** SC2 and ADR 0004 say the database refuses an 11th link even when the API is bypassed. The trigger is `BEFORE INSERT` only. A raw `UPDATE artist_tags SET artist_id = <artist with 10 links> WHERE ...` moves links onto an artist with no check, and the cap is exceeded. The ADR argues that no code path does this today. That is an application-level promise, not the database-level guarantee the success criterion asks for. Merge's `UPDATE ... SET tag_id` must stay unaffected.
**Fix:** In a new migration, since 000010 must not be edited once applied (see `internal/db/migrations/README.md`), also fire the function for `UPDATE OF artist_id` and short-circuit when `artist_id` is unchanged:
```sql
-- at the top of check_artist_tags_max_per_artist():
IF TG_OP = 'UPDATE' AND NEW.artist_id = OLD.artist_id THEN
    RETURN NEW;
END IF;

CREATE TRIGGER artist_tags_cap_update_trigger
    BEFORE UPDATE OF artist_id ON artist_tags
    FOR EACH ROW EXECUTE FUNCTION check_artist_tags_max_per_artist();
```
If you don't add the trigger, add an ADR amendment stating that the guarantee covers inserts only.

### WR-04: Client length caps and counters count UTF-16 code units, but the server counts code points

**File:** `web/app/components/watchlist/TagCombobox.tsx:99-105, 135, 143-152`; `web/app/components/watchlist/ArtistNote.tsx:123-127, 175, 190`; `web/app/components/watchlist/ManageTagsDialog.tsx:314-321`
**Issue:** The server caps tag names at 32 and notes at 500 *code points* after NFC. `NormalizeName` says it is "never byte-counted, so a multi-byte name is not rejected for its byte length", and Postgres `char_length` agrees. The client uses `maxLength` and `string.length`, which both count UTF-16 code units:
- Every astral character (emoji, many CJK extension characters) counts as 2.
- Decomposed input (`e` + U+0301) counts as 2 before NFC.

So a legal 32-code-point emoji tag is cut off at 16 characters. A 300-emoji note cannot be typed. The `{n}/32` and `{n}/500` counters and the "limit reached" announcements show the wrong values. This defeats the TAG-04 encoding intent on the only UI path.
**Fix:** Count code points after NFC, and enforce the cap in `onChange` instead of with the `maxLength` attribute:
```ts
const codePoints = (s: string) => [...s.normalize("NFC")].length
// handleInputValueChange / handleChange: reject or clamp when codePoints(next) > MAX
// counters: {codePoints(query)}/{MAX_TAG_LENGTH}
```

### WR-05: `t.Fatalf` is called from non-test goroutines in the concurrent cap-race test

**File:** `internal/httpserver/tags_test.go:861-869, 909-916`
**Issue:** `postTag` calls `t.Fatalf` on transport errors, and `TestTags_Attach_ConcurrentCapRace` calls it from two spawned goroutines. `FailNow` must run on the test goroutine. From another goroutine it only exits that goroutine: `wg.Done()` still runs because it is deferred, `statuses[i]` stays 0, and the failure appears as a confusing `sorted statuses = [0 201]` mismatch, or as a panic after the test returns. The sibling test `TestTags_Detach_ConcurrentSameLinkBoth204` gets this right with `t.Errorf` plus `return`.
**Fix:** Have `postTag` return `(int, error)` for goroutine use. Inside the goroutines, call `t.Errorf` and return, and check `t.Failed()` after `wg.Wait()`.

## Info

### IN-01: The migration comment says tag identity is accent-insensitive, but it isn't

**File:** `internal/db/migrations/000010_tags_and_notes.up.sql:14`
**Issue:** "Case/accent-insensitive identity (TAG-03...)" is wrong. `lower(name)` folds case only. `TestSchema_TagNameUniqueLower` (`tags_schema_test.go:244-251`) inserts `reggaeton` and `reggaetón` as two distinct tags. TAG-03 is case/whitespace identity only.
**Fix:** Change the comment to "Case-insensitive identity".

### IN-02: The `attachTag` doc comment swaps the 201 and 200 meanings

**File:** `web/app/lib/api.ts:371-373`
**Issue:** It says "already existed (201) or was newly created (200)". The handler (`tags.go:118-122`) returns 201 for a new link and 200 when the link already existed.
**Fix:** Swap the two status codes in the comment.

### IN-03: Limit values are hard-coded in several places instead of shared

**File:** `web/app/components/watchlist/TagChips.tsx:18-20` (duplicates `MAX_TAGS_PER_ARTIST` exported from `web/app/lib/tags.ts:4`); `ManageTagsDialog.tsx:314, 319-321` (literal `32`/`25` instead of `MAX_TAG_LENGTH`); `ArtistNote.tsx:123-126, 175, 187-190` (`500`/`450`); `internal/httpserver/tags.go:351` and `internal/tags/service.go:28` (literal "32" and "10" in messages instead of `tags.MaxNameRunes` and `MaxTagsPerArtist`)
**Fix:** Import the existing constants. Build the messages with `fmt.Sprintf` from `MaxNameRunes` and `MaxTagsPerArtist`.

### IN-04: `DeleteTagCountingCarriers` can under-report when an attach commits at the same moment

**File:** `queries/tags.sql:67-75`
**Issue:** Under READ COMMITTED, the `counted` CTE reads the statement-start snapshot. The `DELETE` may block on an in-flight attach's row lock (from `GetOrCreateTag`'s `DO UPDATE`), then delete the newer row version and cascade the just-committed link. That link is missing from the reported `carrier_count`, so the toast says "from N artists" when N+1 lost the tag.
**Fix:** Take `SELECT ... FROM tags WHERE id = $1 FOR UPDATE` in a transaction before counting and deleting. Or document the count as best-effort.

### IN-05: A second rename collision during the retry returns 500 instead of 409

**File:** `internal/tags/service.go:203-210`
**Issue:** The retry after a vanished collider goes through `mapTagError`, which does not map `tags_name_lower_idx`. If yet another tag took the name in the meantime, the handler returns a generic 500.
**Fix:** Loop back into the collision branch, or map the unique violation to a `CollisionError`/409.

### IN-06: `watchlist_note_not_blank` is mapped to "note contains invalid characters"

**File:** `internal/watchlist/service.go:532-533`
**Issue:** A blank-note CHECK violation is reported as an invalid-characters error. The path is unreachable after `NormalizeNote`, but the mapping is misleading if it ever fires.
**Fix:** Map it to its own sentinel, or treat it as a clear.

### IN-07: A merge target that isn't in the dialog's loaded list disappears from it

**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:200-211`
**Issue:** If the rename collides with a tag created after the dialog loaded, `withoutSource.map` has no target row to update. The merged tag does not appear, and `findIndex` returns -1, so no focus move happens. `renameTarget` and `renameValue` also survive closing and reopening the dialog, so a stale rename row can reappear.
**Fix:** When the target is missing, insert `merged` into the list. Reset the rename state when `open` becomes false.

### IN-08: The client re-sorts tags with a different order than the server

**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:47-54`, `web/app/lib/tags.ts:79-84`
**Issue:** `localeCompare(..., { sensitivity: "base" })` ignores accents and uses locale rules. The server orders by `lower(name), id` (TAG-05). `reggaeton` and `reggaetón` compare equal on the client and fall back to id order, which can differ from the server's order.
**Fix:** Keep the server order for the loaded list. For re-sorts, compare `toLowerCase()` keys with a plain `<`, then fall back to id.

### IN-09: The ADR-mandated distinct-tag 10th/11th race is tested only as an unforced HTTP race

**File:** `internal/httpserver/tags_test.go:877-933`, `internal/db/tags_schema_test.go:126-201`
**Issue:** ADR 0004 requires pinning "two concurrent 10th/11th attaches (one succeeds)". Only the same-tag case is forced at the database level, using `pg_stat_activity`. The distinct-tag case relies on 10 loose HTTP iterations, which can all pass without the two transactions ever overlapping. That run would not prove the `FOR NO KEY UPDATE` lock.
**Fix:** Add a forced variant of `TestSchema_TagCapTrigger_SameTagConcurrentAt9` using two different tag ids. Assert that txB errors with `artist_tags_max_per_artist` after txA commits.

### IN-10: New comments break the project's 1-3 line comment rule

**File:** e.g. `queries/watchlist.sql:11-25, 62-66`, `internal/watchlist/service.go:382-387, 405-411`, `internal/httpserver/watchlist.go:136-140`, `web/app/components/watchlist/ArtistNote.tsx:15-21, 35-41`, `web/app/components/watchlist/TagChips.tsx:22-27`
**Issue:** `.claude/CLAUDE.md` asks for 1-3 line intent comments with a single design-doc reference. Many of the new blocks are 5-15 lines, cite several D-xx references, and argue the decision again inline.
**Fix:** Cut each block down to its core intent and one reference.

---

_Reviewed: 2026-09-23T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
