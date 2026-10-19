---
phase: 26-bulk-edit-remove
reviewed: 2026-10-16T00:00:00Z
depth: standard
files_reviewed: 32
files_reviewed_list:
  - cmd/server/main.go
  - docs/adr/0004-per-artist-tag-cap-trigger.md
  - internal/httpserver/bulk.go
  - internal/httpserver/bulk_test.go
  - internal/httpserver/server.go
  - internal/httpserver/tags.go
  - internal/httpserver/tags_test.go
  - internal/tags/bulk.go
  - internal/tags/bulk_test.go
  - internal/watchlist/bulk.go
  - internal/watchlist/bulk_test.go
  - queries/bulk.sql
  - web/app/app.css
  - web/app/components/common/ConfirmDialog.tsx
  - web/app/components/common/ConfirmDialog.test.tsx
  - web/app/components/ui/checkbox.tsx
  - web/app/components/ui/popover.tsx
  - web/app/components/watchlist/BulkActionBar.tsx
  - web/app/components/watchlist/BulkActionBar.test.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagCombobox.tsx
  - web/app/components/watchlist/TagCombobox.test.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/lib/api.ts
  - web/app/lib/api.test.ts
  - web/app/lib/bulkSelection.ts
  - web/app/lib/bulkSelection.test.ts
  - web/app/lib/format.ts
  - web/app/lib/format.test.ts
  - web/app/root.tsx
  - web/app/routes/watchlist.tsx
  - web/app/routes/watchlist.bulk.test.tsx
findings:
  critical: 0
  warning: 3
  info: 7
  total: 10
status: issues_found
---

# Phase 26: Code Review Report

**Reviewed:** 2026-10-16
**Depth:** standard
**Files Reviewed:** 32
**Status:** issues_found

## Summary

I reviewed the four bulk routes, the `tags`/`watchlist` bulk services, `queries/bulk.sql`, the ADR 0004 amendment, and the React selection, action bar and confirm flow, using the diff against `5c0d214`.

The server side holds up when traced through:
- Every bulk route is registered inside `registerDataRoutes`, so it inherits `gate.Authenticate` and `RequireCSRFHeader`.
- Bodies are capped with `MaxBytesReader` and decoded with `DisallowUnknownFields`, so an over-posted `note` is rejected.
- Ids are bounded to 1000, deduped, and checked to be at least 1.
- `watchlist_artist_id_key UNIQUE (artist_id)` makes the `entryByArtist` map in `BulkAttach` safe.
- The ordered `FOR NO KEY UPDATE` pre-lock and the separate guarded insert match the ADR amendment.
- The data-modifying `updated` CTE in `BulkUpdateWatchlistPreferences` runs even though the final SELECT does not reference it, which is Postgres semantics, and the `locked` CTE re-reads rows after a lock wait (EvalPlanQual).
- `Merge` never locks `artists` rows, so it cannot deadlock with the bulk pre-lock.

On the frontend:
- There is no conditional hook ordering in `watchlist.tsx`.
- `ConfirmDialog` blocks dismissal while pending, so a late outcome cannot leak into a later dialog.
- The Esc-to-clear handler is guarded for both the inline picker and the portaled popover.

The findings are:
- **Undo uses a stale name.** Remove-tag Undo re-adds the tag using a tag name the client captured earlier. The server already loads the real name but throws it away.
- **Two weak concurrency tests.** The new concurrency tests can pass without exercising the property they claim to pin.
- **Smaller issues.** Duplicated cap constants and some small quality items.

## Warnings

### WR-01: Remove-tag Undo re-adds by a stale client-side name; the server throws away the real name

**File:** `web/app/routes/watchlist.tsx:696`, `web/app/routes/watchlist.tsx:705-720`, `internal/tags/bulk.go:146`, `internal/httpserver/bulk.go:210-214`
**Issue:** The Undo closure captures `tag.name` from the `TagRef` the picker built out of the last `listWatchlist` snapshot (`removableTags(selected)`). It then calls `bulkAddTag(ids, name)`, which is get-or-create by name.
- **Same tab:** within the 8 s toast window, the user renames the tag in Manage Tags and then clicks Undo. `applyTagChange` updates `entries`, but the closure still holds the old name. Undo creates a second tag under the old name and links it to the `applied` artists, instead of restoring the tag that was removed.
- **Another tab:** the same happens when the tag was renamed elsewhere and `entries` are stale. In that case it happens even with no rename inside the window.

`BulkDetach` already resolves the tag's current name (`s.q.GetTagByID(ctx, tagID)` at `internal/tags/bulk.go:146`) but discards it, and `bulkRemoveTagResponse` carries no tag.

U4 chose re-add-by-name on purpose. This finding is only about *which* name is used.
**Fix:** Return the resolved tag from the remove endpoint and use it for Undo. Before sending, prefer the vocabulary's current name for that id:
```go
// internal/tags/bulk.go
type BulkDetachResult struct {
	Tag     Tag
	Applied []int64
	Gone    []int64
}
// ...
row, err := s.q.GetTagByID(ctx, tagID)
// ...
result.Tag = Tag{ID: row.ID, Name: row.Name}

// internal/httpserver/bulk.go
type bulkRemoveTagResponse struct {
	Tag     tagResponse    `json:"tag"`
	Applied []int64        `json:"applied"`
	Carried int            `json:"carried"`
	Skipped []bulkSkipWire `json:"skipped"`
}
```
```ts
// watchlist.tsx: Undo uses the server's tag, re-resolved at click time
onClick: () => undoRemoveTag(tagNamesRef.current.get(out.tag.id) ?? out.tag.name, out.applied),
```

### WR-02: Watchlist bulk concurrency tests use a fixed 300 ms window and can pass without the race ever happening

**File:** `internal/watchlist/bulk_test.go:413-432`, `internal/watchlist/bulk_test.go:458-478`
**Issue:** `TestService_BulkUpdatePreferences_ConcurrentAxisWriteIsNotLost` and `..._ConcurrentDeleteIsGone` start the bulk call in a goroutine and only check that it has not finished after 300 ms. Then they commit the blocking transaction.

If the goroutine has not reached the database within 300 ms (scheduler delay, slow pool acquire on a loaded CI runner), the commit lands first. The bulk statement then runs against already-committed state and produces the same expected result. The test passes without exercising the lock wait or EvalPlanQual path it documents. It cannot tell a regression (for example, dropping `FOR UPDATE` from the `locked` CTE) apart from a lucky schedule.

`TestService_BulkAttach_WaitsOnConcurrentTenthLink` in `internal/tags/bulk_test.go:332-347` already uses the robust pattern.
**Fix:** Before committing, poll until the bulk session is actually blocked on the holder:
```go
pid := tx.Conn().PgConn().PID()
deadline := time.Now().Add(5 * time.Second)
for {
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))", int32(pid)).Scan(&n); err != nil {
		t.Fatalf("poll blockers: %v", err)
	}
	if n >= 1 {
		break
	}
	if time.Now().After(deadline) {
		t.Fatal("BulkUpdatePreferences never blocked on the held row lock")
	}
	time.Sleep(20 * time.Millisecond)
}
```

### WR-03: `ConcurrentOverlappingNoDeadlock` cannot fail if the ordering it pins is removed

**File:** `internal/tags/bulk_test.go:239-297` (pins `queries/bulk.sql:12`, `ORDER BY a.id`)
**Issue:** The test feeds one goroutine `entries` and the other `reversed`, on the assumption that input order drives lock order. It does not.
- `LockWatchlistArtistsForBulkTag` filters with `w.id = ANY(@entry_ids)`.
- A btree ScalarArrayOp scan sorts the array keys, and a seq scan returns rows in physical order. Either way, rows come back in an order that does not depend on the array's element order.
- Entries and artists are also seeded in the same order, so watchlist id order equals artist id order.

So with `ORDER BY a.id` deleted, both transactions still lock in the same order and never deadlock. The test is listed in the ADR as the pin for the ordering guarantee, but it gives no protection against a regression.
**Fix:** Build a case where the plan order differs from artist-id order. For example, seed artists A1 < A2 and create their watchlist entries in reverse, so that W(A2) < W(A1). Then confirm the test fails with the `ORDER BY` removed before relying on it. Or reword the ADR's "Pinned by" line so it no longer claims this test pins the ordering.

## Info

### IN-01: The tag cap literal `10` is duplicated in SQL and route copy

**File:** `queries/bulk.sql:26`, `web/app/routes/watchlist.tsx:616-617,630`
**Issue:** The guarded insert hardcodes `< 10`, with only a comment pointing at `tags.MaxTagsPerArtist`. The route hardcodes "10 tags" three times, even though `MAX_TAGS_PER_ARTIST` exists in `~/lib/limits`.

If the cap changes in the trigger and in `MaxTagsPerArtist` but not in the query, bulk add breaks one of two ways. Either it silently reports artists with 10 or more tags as `cap` skips, or the trigger backstop turns a skip into a whole-batch 409.
**Fix:** Pass the cap as a parameter (`AND (...) < @max_tags::int`, fed by `tags.MaxTagsPerArtist`), and use `MAX_TAGS_PER_ARTIST` in the toast copy.

### IN-02: A bulk add or restore that applies to nobody still shows a success toast and clears the selection

**File:** `web/app/routes/watchlist.tsx:625-639`, `internal/tags/bulk.go:103-106`
**Issue:** When every sent id is `gone` (no cap skips), `n === 0` falls through to `toast.success("Added “x” to 0 artists.")` and the bar's `onSucceeded` clears the selection. The response's `tag.id` is `0`, because the transaction rolled back.
**Fix:** Treat `applied.length === 0 && caps.length === 0` as its own warning case (the gone description alone), or return `null` so the bar keeps the selection.

### IN-03: The Undo path does not `remember` a tag it may have recreated

**File:** `web/app/routes/watchlist.tsx:705-720`
**Issue:** `commitAddTag` calls `remember(outcome.tag)` when anything applied, but `undoRemoveTag` does not. If the tag was deleted or renamed between the remove and the Undo, get-or-create makes a new id that the loaded vocabulary never learns about, so the combobox suggestions stay stale.
**Fix:** Have the route reach the vocabulary (for example, pass `remember` up from the provider, or move Undo into the bar's scope) and call it when `out.applied.length > 0`.

### IN-04: The `"gone"` reason is defined twice, and `carried` duplicates `applied.length`

**File:** `internal/httpserver/bulk.go:266`, `internal/httpserver/bulk.go:247-254`
**Issue:**
- `skipReasonGone` duplicates `tags.SkipReasonGone`. The preferences path uses the local constant and the remove-tag path uses the `tags` one, so the same wire value has two sources.
- `Carried` is always `len(result.Applied)`.

**Fix:** Use `tags.SkipReasonGone` (or one shared constant) in both places, and drop `carried` or document why it exists separately.

### IN-05: Inconsistent bulk 400 bodies

**File:** `internal/httpserver/bulk.go:234-236`, `internal/httpserver/bulk.go:109-113`
**Issue:**
- Every other bulk 400 carries a `code`, but an invalid `tag_id` uses `writeError` without one.
- A body over `maxBulkBodyBytes` surfaces as a generic `invalid request body` 400 rather than 413.

**Fix:** Use `writeErrorCode(w, 400, "bulk_invalid_tag_id", ...)`, and map `*http.MaxBytesError` to `http.StatusRequestEntityTooLarge`.

### IN-06: A non-component export from a component module

**File:** `web/app/components/watchlist/TagCombobox.tsx:42`
**Issue:** `DISMISS_REASONS` is now exported from a component file, so Vite Fast Refresh falls back to a full reload for this module in dev.
**Fix:** Move it to `~/lib/tags` or a small `~/lib/combobox` module and import it from both components.

### IN-07: Bar clearance is not re-measured when the bottom inset changes at a breakpoint

**File:** `web/app/components/watchlist/BulkActionBar.tsx:368-386`
**Issue:** `--bulk-bar-clearance` folds in `getComputedStyle(wrapper).bottom` (`bottom-2` vs `sm:bottom-6`), but only a `ResizeObserver` on the wrapper triggers a re-measure. Crossing the `sm` breakpoint without the bar's size changing leaves the variable off by 16px.
**Fix:** Also re-measure on a `matchMedia("(min-width: 40rem)")` change listener, or on window `resize`.

---

_Reviewed: 2026-10-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
