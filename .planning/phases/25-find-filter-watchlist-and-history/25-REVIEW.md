---
phase: 25-find-filter-watchlist-and-history
reviewed: 2026-10-10T17:30:37Z
depth: standard
files_reviewed: 40
files_reviewed_list:
  - internal/events/service.go
  - internal/httpserver/events.go
  - internal/httpserver/events_test.go
  - internal/httpserver/watchlist_test.go
  - internal/watchlist/service.go
  - internal/watchlist/service_test.go
  - queries/events.sql
  - queries/watchlist.sql
  - web/app/components/history/HistoryFilters.test.tsx
  - web/app/components/history/HistoryFilters.tsx
  - web/app/components/ui/combobox.tsx
  - web/app/components/watchlist/ArtistNote.test.tsx
  - web/app/components/watchlist/PreferenceToggles.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/components/watchlist/TagChips.test.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagCombobox.test.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/components/watchlist/WatchlistTagFilter.test.tsx
  - web/app/components/watchlist/WatchlistTagFilter.tsx
  - web/app/components/watchlist/WatchlistToolbar.test.tsx
  - web/app/components/watchlist/WatchlistToolbar.tsx
  - web/app/lib/api.test.ts
  - web/app/lib/api.ts
  - web/app/lib/format.test.ts
  - web/app/lib/format.ts
  - web/app/lib/historyParams.test.ts
  - web/app/lib/historyParams.ts
  - web/app/lib/tags.test.ts
  - web/app/lib/tags.ts
  - web/app/lib/test/fixtures.ts
  - web/app/lib/test/routeStub.tsx
  - web/app/lib/useUrlParams.test.tsx
  - web/app/lib/useUrlParams.ts
  - web/app/lib/watchlistView.test.ts
  - web/app/lib/watchlistView.ts
  - web/app/routes/history.test.tsx
  - web/app/routes/history.tsx
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
findings:
  critical: 0
  warning: 4
  info: 7
  total: 11
status: issues_found
---

# Phase 25: Code Review Report

**Reviewed:** 2026-10-10T17:30:37Z
**Depth:** standard
**Files Reviewed:** 40
**Status:** issues_found

## Summary

Reviewed the phase-25 diff (`72f26da..HEAD`). It covers the History `tag_id` filter (SQL, service, and handler), the watchlist `latest_release_date`/`next_release_date` projection, URL-backed History and Watchlist filter state (`useUrlParams`, `historyParams`, `watchlistView`), and the new toolbar, tag-filter, and chip components.

The backend changes are small and correct:
- `tag_id` is validated with the same positive-int rule as `artist_id`.
- It is bound as a sqlc parameter.
- It is applied identically in `ListEvents` and `HasOlderEvents`, so it composes with the retention cutoff and the keyset cursor.
- The integration tests cover AND-composition, cursor paging under a tag, and an unknown tag id.

I found no security issues and nothing at BLOCKER level. The defects are all in the frontend state layer:
- A filter change can let a stale "Load more" response leak rows from the previous filter into the History feed.
- Sticky watchlist cards survive filter changes made through the URL (Back/Forward).
- `ListWatchlist` and `formatReleaseDate` accept any dated text, so a placeholder date gets shown as a real release.
- One test cannot detect the regression it is named for.

## Warnings

### WR-01: A stale "Load more" response can append rows from the previous filter to the new feed

**File:** `web/app/routes/history.tsx:217-241` (interacts with the page-one effect at `188-215`)
**Issue:** `handleLoadMore` has no cancellation or filter-identity guard. Here is the sequence:
1. A "Load more" request for filter A is in flight.
2. The filter changes to B. This phase made that reachable through the Tag control and also through browser Back/Forward, because filters now live in the URL.
3. The page-one effect clears `events` and fetches page one for B.
4. The A response then lands. It appends A's rows onto B's list through `setEvents(prev => [...prev, ...fresh])` and overwrites `nextCursor` and `hasOlderEvents` with A's values.

The feed then shows events that do not match the active tag/artist/type filter, which is exactly what the effect's own comment says must never happen. The next "Load more" then sends A's cursor with B's filters, which skips or duplicates B rows. `appendLoading` also stays `true` across the filter change, so B's Load more button stays disabled until A's request settles. The handler predates this phase, but this phase is what made URL-driven filter changes reachable while a load is in flight.
**Fix:** Tag each append with the filter key it was issued for, and drop the result if the key has changed:
```tsx
const filtersKeyRef = useRef("")
filtersKeyRef.current = `${filters.artistId}|${filters.eventType}|${filters.tagId}|${reloadToken}`

const handleLoadMore = () => {
  if (nextCursor == null || appendLoading) return
  const issuedFor = filtersKeyRef.current
  setAppendLoading(true)
  setAppendError(null)
  fetchHistoryPage(filters, nextCursor)
    .then((page) => {
      if (filtersKeyRef.current !== issuedFor) return
      // ...existing append / setNextCursor / setHasOlderEvents
    })
    .catch(() => {
      if (filtersKeyRef.current === issuedFor) setAppendError("Couldn't load release history.")
    })
    .finally(() => {
      if (filtersKeyRef.current === issuedFor) setAppendLoading(false)
    })
}
```
Also reset `setAppendLoading(false)` in the page-one effect.

### WR-02: Sticky cards survive filter changes made through the URL (Back/Forward)

**File:** `web/app/routes/watchlist.tsx:141-149` (and the `clearSticky` call sites at `126`, `434`, `443`, `459`, `473`)
**Issue:** The sticky set (G4) is supposed to last only until the next filter change. Every UI handler (`handleQueryChange`, `handleTagsChange`, `handleFlagChange`, `handleSortChange`, `clearFilters`) calls `clearSticky()`. But tags, flags, sort and `q` are all URL state, and `handleTagsChange`/`handleFlagChange`/`handleSortChange` push history entries.

When the user presses Back or Forward, `view` changes and nothing clears `stickyIds`. A card that was just tagged or toggled stays in the list even though it does not match the restored filter. The count announcement is also not scheduled for that change. The `[view.q]` resync effect is the only URL-driven path, and it clears neither.
**Fix:** Clear stickies, and announce the count, whenever the filter-owned URL keys change for any reason, not only from the UI handlers:
```tsx
const filterKey = `${view.q}|${view.sort}|${view.tagIds.join(",")}|${view.muted}|${view.custom}`
const prevFilterKey = useRef(filterKey)
useEffect(() => {
  if (prevFilterKey.current === filterKey) return
  prevFilterKey.current = filterKey
  clearSticky()
}, [filterKey])
```
The explicit `clearSticky()` calls in the handlers can then be removed, except in `handleQueryChange`, which filters before the debounced URL write.

### WR-03: Placeholder or malformed release dates are shown as a real "Latest release"

**File:** `queries/watchlist.sql:38-51` (and the duplicate at `75-88`); `web/app/lib/format.ts:168-175`; `web/app/components/watchlist/WatchlistRow.tsx:55-57`
**Issue:** The `latest`/`upcoming` LATERAL subqueries accept any non-NULL `release_date` text. Deezer `new_release` rows store `a.ReleaseDate` verbatim (`internal/detection/deezer.go:88`), and only `""` is mapped to NULL. If Deezer ever sends its `0000-00-00` "unknown date" placeholder, that row:
- satisfies `<= left(today, 10)`, so it becomes the artist's latest release whenever it is the only dated one;
- falls through every regex in `formatReleaseDate`, which returns the raw string, so the card reads "Latest release: 0000-00-00";
- sorts as the oldest dated entry under "Latest release (oldest)" instead of with the nulls.

The same applies to any other value that is not `YYYY`, `YYYY-MM`, or `YYYY-MM-DD`.
**Fix:** Restrict both LATERALs to well-formed dates, for example:
```sql
AND e.release_date ~ '^[0-9]{4}(-(0[1-9]|1[0-2])(-(0[1-9]|[12][0-9]|3[01]))?)?$'
AND e.release_date >= '0001'
```
Alternatively, normalize `0000-00-00` to NULL at insert time in `nullableString`'s Deezer call site. Also add a test case with a malformed date to `TestService_List_LatestRelease`.

### WR-04: The "cancels a pending count announcement" test cannot detect the regression it is named for

**File:** `web/app/routes/watchlist.test.tsx:1419-1436` (and `1402-1417`)
**Issue:** The test types `"a"`, which starts the real 500 ms count timer. It then clicks the Sort trigger and an option through `userEvent` with real timers, sleeps 700 ms, and asserts the status reads "Sorted by …".

If the userEvent interactions take more than 500 ms, which is plausible on a loaded CI runner with jsdom plus the Base UI Select, the count announcement fires *before* the sort click. The sort message then overwrites it and the test passes even if `handleSortChange` no longer clears `countTimer`. Whether the test can fail at all depends on wall-clock speed. Both tests also add about 1.4 s of real sleeping and are coupled to the `COUNT_ANNOUNCE_DEBOUNCE_MS` value.
**Fix:** Use `vi.useFakeTimers({ shouldAdvanceTime: true })` with `userEvent.setup({ advanceTimers: vi.advanceTimersByTime })`. Type, then select the sort *before* advancing, then `vi.advanceTimersByTime(COUNT_ANNOUNCE_DEBOUNCE_MS + 100)` and assert that the status still reads "Sorted by …". Replace the `setTimeout(r, 700)` sleeps the same way.

## Info

### IN-01: Stale comments after adding the tag axis

**File:** `web/app/lib/api.ts:278-283`; `queries/events.sql:174`; `internal/events/service.go:51-56, 105-106`; `web/app/routes/watchlist.tsx:44-51`
**Issue:**
- `listEvents`'s comment still says history.tsx "only ever calls listEvents() with no params" and lists only artist_id/event_type/cursor.
- `HasOlderEvents` says it "Mirrors ListEvents' two optional filters", but there are three now.
- `ListParams`/`List` docs mention only artist/event-type filters.
- In watchlist.tsx, the route header comment (which describes `Watchlist` and `refresh`) now sits above the unrelated `Q_WRITE_DEBOUNCE_MS` constants.

**Fix:** Update or trim these comments per the project's comment-discipline rule. Move the route comment back above `export default function Watchlist`.

### IN-02: The duplicated LATERAL blocks' parity is only type-checked, not semantically checked

**File:** `queries/watchlist.sql:20-52, 57-89`; `internal/watchlist/service.go:369-379`
**Issue:** The comment says `Service.get` "only compiles while the projections stay identical". The `sqlc.ListWatchlistRow(row)` conversion only enforces matching column names and types. A predicate edit to one copy of the ~15-line `latest`/`upcoming` LATERAL pair, but not the other, still compiles. List and the single-entry responses (Add, PATCH, note) would then silently disagree. `TestService_ProjectionParity` covers only one data shape.
**Fix:** Have `GetWatchlistEntry` select from a shared SQL view (`CREATE VIEW watchlist_entries AS ...`, plus a migration), or reword the comment so it doesn't overclaim.

### IN-03: The History Combobox silently shows "All …" for a selected id missing from loaded options

**File:** `web/app/components/history/HistoryFilters.tsx:86-91, 264-281`; `web/app/routes/history.tsx:249-256`
**Issue:** `triggerLabel` only covers the `loading`/`error` states. When options are loaded but the selected id is absent, `selectedIndex === -1` falls back to `options[0]` and the trigger reads "All tags" or "All artists" while the feed is still filtered. Examples: a tag deleted in another tab after the once-per-mount check, or an artist removed elsewhere. Separately, when `GET /tags` fails, the empty state reads `No events for “this tag”`, which puts a placeholder in quotes as if it were a tag name.
**Fix:** When `value !== null && selectedIndex === -1`, show "Unknown tag"/"Unknown artist". When `tagsState.status === "error"`, use unquoted copy ("No events for the selected tag").

### IN-04: The tag-filter input's `aria-label` is dead

**File:** `web/app/components/watchlist/WatchlistTagFilter.tsx:63-67`
**Issue:** `aria-labelledby="watchlist-tags-label"` takes precedence over `aria-label`, so "Filter by tags, any of" is never exposed. The accessible name is "Tags: any of".
**Fix:** Keep only one of the two attributes.

### IN-05: Unreachable `|| byId(a, b)` in `latestCompare`

**File:** `web/app/lib/watchlistView.ts:122`
**Issue:** `x === y` is handled one line earlier, so `(x < y ? -1 : 1) * dir` is never 0 and the `|| byId` fallback can't run.
**Fix:** `return x < y ? -dir : dir` (or keep the expression and drop the fallback).

### IN-06: Ref mutated during render

**File:** `web/app/routes/watchlist.tsx:407-411`
**Issue:** `tagNamesRef.current.set(...)` runs in the render body. Under concurrent rendering, an abandoned render can still record names, and React's guidance is to treat render as pure. It is harmless today because the map only grows, but `settleTagIds` and the canonicalization effect both depend on it.
**Fix:** Populate it in a `useLayoutEffect` keyed on `entries`, or derive the map with `useMemo` and merge it into the ref in an effect.

### IN-07: A lib module depends on a component module for its type

**File:** `web/app/lib/historyParams.ts:1`
**Issue:** `lib/historyParams.ts` imports `HistoryFiltersValue` from `components/history/HistoryFilters`. That inverts the lib-to-component layering used elsewhere (for example, `watchlistView.ts` owns `WatchlistView`).
**Fix:** Move `HistoryFiltersValue` into `historyParams.ts` (or `lib/api.ts`) and import it from the component.

---

_Reviewed: 2026-10-10T17:30:37Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
