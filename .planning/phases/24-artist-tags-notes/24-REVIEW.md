---
phase: 24-artist-tags-notes
reviewed: 2026-10-04T00:00:00Z
depth: standard
review_kind: incremental
diff_base: a689ab7f5bab2cd9bd0d6166ea24a81bff9aab39
files_reviewed: 5
files_reviewed_list:
  - CONTEXT.md
  - internal/webassets/build/client/index.html
  - web/app/components/watchlist/ManageTagsDialog.test.tsx
  - web/app/components/watchlist/ManageTagsDialog.tsx
  - web/app/routes/watchlist.test.tsx
findings:
  critical: 0
  warning: 1
  info: 6
  total: 7
status: issues_found
---

# Phase 24: Code Review Report (incremental re-review, gap closure round 3 / 24-12)

**Reviewed:** 2026-10-04T00:00:00Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

This review covers only the changes since `a689ab7`. Plan 24-12 adds a `loadGen` / `loadInFlight` guard to `ManageTagsDialog.load()` and an `invalidateLoad()` call on delete, rename, and merge success. It also adds one tracer route test, two route variants, three dialog tests, and the rebuilt embedded bundle. The `CONTEXT.md` change comes from the Phase 25 commit `697eab2`, which landed on this branch. I did not review the minified assets. `index.html` changes only the manifest hash, and the rebuild commit `3d07813` touches only the manifest and the `watchlist-*.js` chunk, which is what this change should produce. Both test files pass locally (44/44).

Earlier findings are not carried forward. The only one I re-checked is WR-08.

**WR-08: resolved.** I traced every way a dialog `GET /tags` can settle:
- **Stale load across a close and reopen.** The reopen's `load()` advances `loadGen`, so the first open's `.then` and `.catch` both return before touching `setTags`, `setStatus`, or `onLoaded`. A deleted, renamed, or merged-away tag can no longer reach `handleTagsLoaded` this way. The dialog is always mounted (`watchlist.tsx:337`), so the refs survive a close.
- **Stranding on the skeleton.** Only `load()` sets `"loading"`. Only a newer generation can drop a settle, and a newer generation means a newer load, either from `load()` itself or from `invalidateLoad()`, which calls `load()` while `loadInFlight` is set. `loadInFlight` is set only by `load()` and cleared only by the current generation's settle, so `loadInFlight` true implies the status is `"loading"`. Every `"loading"` state therefore ends with a load that is still allowed to settle. A rename that fails or returns a collision during a reopen does not invalidate the load, and that load then settles normally. `listTags` is `async` (`api.ts:388`), so it cannot throw synchronously and leave `loadInFlight` set.
- **Extra `listTags` calls.** None on normal flows. A delete, rename, or merge can only start from a rendered list, which means status `"loaded"` and `loadInFlight` false, so `invalidateLoad()` only bumps the counter. The refetch happens only when a pending rename (or the merge that its collision opens) succeeds while a reopen's load is still in flight. The catch paths and Retry are unchanged.

There is still one test gap (WR-09). The merge branch of the refetch is reachable and prevents a real re-create, but no test exercises it. Most of the new route and dialog tests pin load-over-load supersession, not the mutation-time invalidation their titles describe.

## Warnings

### WR-09: The merge-time invalidation prevents a real regression, but no test covers it, and the tests' titles overstate what they pin

**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:219`. Tests: `web/app/routes/watchlist.test.tsx` (the three new route tests), `web/app/components/watchlist/ManageTagsDialog.test.tsx:471-509`
**Issue:** When `loadInFlight` is false, `invalidateLoad()` does nothing useful (see IN-17), so only its refetch branch has any effect. Here is how each call site is covered:
- **Rename (`:180`):** reachable, and pinned by the no-strand dialog test.
- **Delete (`:252`):** the refetch branch is effectively unreachable. The Delete button exists only while the status is `"loaded"`, and the pending `ConfirmDialog` blocks a close and reopen.
- **Merge (`:219`):** reachable, and covered by no test. Here is the sequence:
  1. Start a rename and press Enter.
  2. Close Manage tags, then reopen it. Load N starts.
  3. The rename returns a collision, and the merge `ConfirmDialog` opens over the skeleton.
  4. Confirm before load N settles.

  Load N was served before the merge committed, so it still contains the source tag. Without the `invalidateLoad()` on `:219`, load N would settle as current, call `onLoaded` with the source tag, and `handleTagsLoaded` would put it back into the route vocabulary with status `"loaded"`. `"+ tag"` would then offer it as an existing tag, and picking it would silently re-create it. That is the exact WR-08 symptom.

Removing `invalidateLoad()` from `handleConfirmDelete` and `handleConfirmMerge` breaks no test. In the tracer, both route variants, and the stale-success dialog pin, the reopen's own `load()` already supersedes the first open's response before the mutation happens, so the mutation's bump changes nothing. Titles such as "...that settles after a merge does not bring the merged-away tag back" read as if they pin the mutation path, but they only pin the `.then` guard.
**Fix:** Add a dialog test for the merge path that mirrors the no-strand test:
```tsx
it("a merge confirmed while a reopen's GET /tags is in flight refetches instead of reviving the source", async () => {
  const reopenLoad = deferred<TagSummary[]>()
  mockListTags
    .mockResolvedValueOnce([
      { id: 5, name: "rap", carrier_count: 1 },
      { id: 9, name: "trap", carrier_count: 0 },
    ])
    .mockReturnValueOnce(reopenLoad.promise)
    .mockResolvedValueOnce([{ id: 9, name: "trap", carrier_count: 1 }])
  const rename = deferred<Awaited<ReturnType<typeof renameTag>>>()
  mockRenameTag.mockReturnValueOnce(rename.promise)
  mockMergeTag.mockResolvedValueOnce({ id: 9, name: "trap", carrier_count: 1 })
  const { onLoaded, setOpen } = renderDialog()
  // rename rap -> trap{Enter}; setOpen(false); setOpen(true)
  // act: rename.resolve({ kind: "collision", target: { id: 9, name: "trap" }, carrierCountAfterMerge: 1 })
  // click "Merge tags"; waitFor listTags x3
  // act: reopenLoad.resolve([{ id: 5, name: "rap", ... }, { id: 9, name: "trap", ... }])
  // expect no "rap" row; onLoaded last called with [{ id: 9, name: "trap", carrier_count: 1 }]
})
```
Optionally, retitle the route variants and the stale-success pin so they say "after a reopen". That is what supersedes the stale response in those tests.

## Info

### IN-17: `invalidateLoad()`'s unconditional bump does nothing, so the function is really "refetch if a load is in flight"

**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:115-120`
**Issue:** When `loadInFlight` is false, the current generation has already settled, and every load still pending has an older generation that is already dropped. `loadGen.current++` then discards nothing. When the flag is true, `load()` bumps the counter again, so each refetch advances the generation by 2. The function behaves exactly like `if (loadInFlight.current) load()`. The unconditional bump suggests the bump itself is the guard, and that misreading is how the WR-09 test gap slipped through.
**Fix:** Reduce it to `if (loadInFlight.current) load()`, keeping the 2-line comment. Or keep the bump and add a short note that it is defensive.

### IN-18: Closing Manage tags leaves its in-flight load current, so a load from an open that is never reopened still overwrites the route vocabulary

**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:122-125`, feeding `web/app/routes/watchlist.tsx:186-190`
**Issue:** Open Manage tags and close it before the load settles. Then open `"+ tag"`, which starts the route's own `loadVocabulary`, and create a tag, which calls `rememberTag`. When the dialog's older `GET` settles, it is still current. `handleTagsLoaded` bumps `vocabGen`, which drops the route's newer in-flight load, and replaces the vocabulary with a list that lacks the new tag. That tag disappears from suggestions on other rows until the next refetch. The effect is cosmetic: the `Create` item get-or-creates the same tag. A deleted tag cannot come back this way, because deletes require a reopen, and the reopen supersedes the old load.
**Fix:** Accept it and add a one-line note. Or, in the `useEffect([open])` cleanup, run `loadGen.current++` and clear `loadInFlight.current` when `open` goes false, so only a load from the currently open dialog reaches `onLoaded`.

### IN-19: A rename that succeeds during the refetch path loses its focus target

**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:127-140, 180-188`
**Issue:** In the path WR-08 newly supports (a rename succeeds while a reopen's load is in flight), `setTags` fires the layout effect while the status is `"loading"`, so `listRef.current` is null. The effect consumes `focusRequestRef` and focuses nothing. When the refetch renders the list, no request is left, so focus never returns to the renamed row's Rename button (UI-SPEC focus row (d)).
**Fix:** For a `"row"` request, consume it only when `listRef.current` exists, so it is applied once the refetched list renders. Clamp the index to the new list's length.

### IN-20: The tracer route test clicks Close without waiting for the delete to complete

**File:** `web/app/routes/watchlist.test.tsx` (tracer, right after the `Delete tag` click)
**Issue:** The rename and merge variants wait for the mutation to show up in the DOM before they close the dialog. The tracer clicks `Close` right after `Delete tag`. It relies on the mocked `deleteTag` resolving and the `ConfirmDialog` closing within `userEvent.click`'s act flush. If either takes another tick, `Close` is still behind an inert modal.
**Fix:** Add `await waitFor(() => expect(screen.queryByRole("button", { name: "Delete tag reggaeton" })).not.toBeInTheDocument())` before clicking `Close`.

### IN-21: `deferred<T>()` is now copied in three test files

**File:** `web/app/components/watchlist/ManageTagsDialog.test.tsx:29-38`, `web/app/routes/watchlist.test.tsx:68-77`, `web/app/components/common/ConfirmDialog.test.tsx:7-18`
**Issue:** The plan asked for file-local copies, which means three identical helpers that can drift apart.
**Fix:** Move it to a shared test utility, for example `web/app/test/deferred.ts`, and import it in all three files.

### IN-22: Phase 25 glossary terms landed on the phase-24 branch, inserted in the middle of the tag/note cluster

**File:** `CONTEXT.md:75-89` (commit `697eab2`, `docs(25)`)
**Issue:** "Latest release", "Upcoming release", "Filter", and "Search" are Phase 25 terms. They are internally consistent. But they sit between "Merge" and "Note", which separates "Note" from the other watchlist-entry terms, and they ride along in a phase-24 gap-closure range.
**Fix:** Move the four entries after "Note", or into their own group. Keep them with the Phase 25 work when the branch is split for a PR.

---

_Reviewed: 2026-10-04T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
