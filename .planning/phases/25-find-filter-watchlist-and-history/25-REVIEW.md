---
phase: 25-find-filter-watchlist-and-history
reviewed: 2026-10-11T20:41:29Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - internal/httpserver/events_test.go
  - web/app/components/history/EventCard.tsx
  - web/app/components/history/EventCard.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.tsx
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagChips.test.tsx
  - web/app/lib/format.ts
  - web/app/lib/format.test.ts
  - web/app/lib/useScrollAnchor.ts
  - web/app/lib/useScrollAnchor.test.tsx
  - web/app/routes/watchlist.tsx
  - web/app/routes/watchlist.test.tsx
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 25: Code Review Report (gap closure 25-11 to 25-14)

**Reviewed:** 2026-10-11T20:41:29Z
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

_Prior review (commit 2cd98c2, 40 files) was `issues_found` with 0 critical, 4 warning, and 7 info. This report replaces it and covers only `2cd98c2..HEAD`._

## Summary

This is an incremental review of the three gap-closure plans:
- **25-11:** `useScrollAnchor` and chip-click anchoring, plus clearing sticky cards on Back/Forward.
- **25-12:** the History card's `Detected` date (`formatDetectedDate`) and a retention integration test.
- **25-13:** the Deezer "Search MusicBrainz to add" width cap and label truncation.

The backend test (`TestRetention_TagFilterKeepsOldReleasesDetectedInWindow`) is sound: it is order-checked, it terminates, cleanup cascades via `ON DELETE CASCADE`, and the time margins are wide. `formatDetectedDate` and the EventCard line are correct; tests pin TZ to UTC.

All the substantive defects are in the 25-11 scroll-anchor and sticky-card interaction:
- The hook assumes the URL change and the sticky clear land in one commit. React Router 7.18.2 commits navigations inside `startTransition` (`node_modules/react-router/dist/development/chunk-62JRHF6Z.mjs:6752`). That puts the synchronous `clearSticky()` in an earlier commit. A sticky card the user clicks on can unmount and remount, which loses both the scroll anchor and focus.
- The anchor is skipped by design when the clicked card leaves the list. That is a reachable path where the G-25-4 collapse comes back.
- The new `filterKey` effect clears sticky cards on the route's own programmatic URL writes, not only on Back/Forward.

The 25-13 truncation leans on a `title` that the Button's `disabled:pointer-events-none` makes unreachable.

No security issues found.

## Warnings

### WR-01: A chip click on a sticky card unmounts the card before the filter commits, dropping the scroll anchor and focus

**File:** `web/app/routes/watchlist.tsx:441-448, 459-465` (with `web/app/lib/useScrollAnchor.ts:12-18`)
**Issue:** `toggleFilterTag` captures the chip and then calls `handleTagsChange`. That runs `write()`, which is a router navigation, and then `clearSticky()`. The two updates commit separately:
- `clearSticky()` is a discrete-event (sync-lane) state update. It commits at the end of the click.
- The navigation is async inside the router. React Router 7.18.2 then applies the new location inside `React.startTransition`, so it commits later.

The first commit therefore renders the **old** filter with stickies cleared. Take a card that was visible only because it was sticky: say the user just removed tag A while filtering by A, and the card still carries B. That commit removes the card.

The second commit applies the new filter. In this example that is `tags=A,B` with OR semantics (`matchesAnyTag`), so the card matches again and is re-mounted as a **new** DOM node. When `useScrollAnchor`'s layout effect runs on the key change, the captured chip has `isConnected === false`, so no correction is made. Two further effects follow:
- The page can visibly collapse and re-expand between the two commits.
- The focused chip button is destroyed, so focus falls to `<body>`. The TagChips contract asserts `label` keeps focus.

No test combines a sticky card with a chip click. The G-25-4 tests only use fresh, non-sticky rows.

**Fix:** Make the sticky invalidation part of the same render as the URL change instead of a separate state write. For example, key the sticky set to the filter it was created under and derive it during render:
```tsx
const [sticky, setSticky] = useState<{ key: string; ids: ReadonlySet<number> }>(
  () => ({ key: filterKey, ids: new Set() })
)
const stickyIds = sticky.key === filterKey ? sticky.ids : EMPTY_SET

function markSticky(id: number) {
  setSticky((prev) => {
    const ids = prev.key === filterKey ? prev.ids : EMPTY_SET
    return ids.has(id) ? prev : { key: filterKey, ids: new Set(ids).add(id) }
  })
}
// drop clearSticky() from handleTagsChange/handleFlagChange/handleSortChange
// and the filterKey effect; keep the q path (query is local state).
```
This also removes the post-paint flash in WR-03. Add a route test: make a card sticky, click one of its chips, and assert `scrollBy` was called and the chip still has focus.

### WR-02: The anchor is silently dropped whenever the clicked card leaves the list, so the G-25-4 collapse returns

**File:** `web/app/lib/useScrollAnchor.ts:15`, `web/app/routes/watchlist.test.tsx:2182-2210`
**Issue:** The reworded contract (25-14, UAT test 4) says that after a chip-label click "the clicked card stays where it was on screen." Tag filtering is OR (`matchesAnyTag`). So un-pressing a chip while two or more tags are selected removes every card that carries only that tag, including the clicked one. That is the exact case in `?tags=3,7`, clicking "trap".

When that happens `a.el.isConnected` is false, the hook returns, and the page collapses under the viewport with no compensation. This is the original "it drops after clicking a tag" symptom. The test at line 2182 enshrines this as the intended outcome instead of providing a fallback.
**Fix:** Capture fallback anchors at click time and use the first one still connected. Candidates are the clicked card's `<li>`, its next and previous sibling cards, and the list heading:
```ts
const capture = useCallback((els: HTMLElement[]) => {
  anchor.current = els.map((el) => ({ el, top: el.getBoundingClientRect().top }))
}, [])
// layout effect: const a = anchor.current?.find((x) => x.el.isConnected)
```
In `TagChips`/`toggleFilterTag`, pass `[chip, chip.closest("li")?.nextElementSibling, chip.closest("li")?.previousElementSibling]` filtered to elements. Then update the line-2182 test to expect a correction based on the surviving neighbor.

### WR-03: The `filterKey` effect clears sticky cards on the route's own URL writes, not just Back/Forward, and only after paint

**File:** `web/app/routes/watchlist.tsx:153-161` (interacts with `274-286`, `296-298`, `165-184`)
**Issue:** The comment says the effect exists because "Back/Forward changes filters without going through a handler." But it fires on **any** `filterKey` change, and the route itself rewrites `tags` programmatically in two places:
- `replaceSelectedTag` runs when Manage tags merges or deletes a selected tag.
- The canonicalization effect runs `settleTagIds`.

Concrete regression introduced by this diff:
1. Filter by `rap`.
2. Remove `rap` from card X. X stays because it is sticky.
3. Open Manage tags and merge `rap` into `hip-hop`. `replaceSelectedTag(3, 9)` rewrites `tags=9`, `filterKey` changes, and `clearSticky()` runs.
4. X vanishes, even though the user never changed the filter. Before 2cd98c2 it stayed.

Separately, the effect runs after paint. On Back the first painted frame still shows the stale sticky cards, and they disappear one frame later.
**Fix:** Preferred: the render-derived sticky set from WR-01 fixes the flash. For the merge case, have `applyTagChange` carry the sticky set across the key it is about to write. Alternatively, set a `selfWriteRef` before `replaceSelectedTag` or the canonical `write`, and skip the clear when the key change came from that write:
```ts
const selfWriteRef = useRef(false)
// in replaceSelectedTag / canonical effect, before write(): selfWriteRef.current = true
useEffect(() => {
  if (prevFilterKey.current === filterKey) return
  prevFilterKey.current = filterKey
  if (selfWriteRef.current) { selfWriteRef.current = false; return }
  clearSticky()
}, [filterKey])
```

### WR-04: The truncated "Search MusicBrainz to add" label relies on a `title` tooltip that can never appear

**File:** `web/app/components/watchlist/SearchResultsColumns.tsx:192-201`, `web/app/components/watchlist/SearchResultsColumns.test.tsx:167-193`
**Issue:** Below `lg`, the Deezer button is now capped at `max-w-[40%]` and its label truncates. In a 375px row the cap is about 115px, so the label reads roughly "Search Mus…". At `sm`, each of the two grid columns is about 285px, so the label is similarly cut.

The only full explanation is the `title` attribute. But `buttonVariants` includes `disabled:pointer-events-none` (`web/app/components/ui/button.tsx:7`), and this button is always `disabled`, so hovering never shows the native tooltip. Touch devices have no hover at all. Sighted users below `lg` therefore cannot read either the label or why the action is unavailable. The test asserts `toHaveAttribute("title")` as if it were a working fallback.
**Fix:** Move the `title` (or a real Tooltip) to a wrapper that receives pointer events, and put the width cap on that wrapper. Or use a short label that does not need truncating below `lg`:
```tsx
<span title={DEEZER_ADD_HINT} className="max-w-[40%] shrink-0 lg:max-w-none">
  <Button variant="secondary" size="sm" disabled className="w-full">
    <span className="truncate">Search MusicBrainz to add</span>
  </Button>
</span>
```
If you keep the tooltip, change the test to assert the wrapper's `title`.

## Info

### IN-01: The `format.ts` header comment is stale

**File:** `web/app/lib/format.ts:7-19`
**Issue:** The header claims "Six formatters, six call sites" and lists them. The module now also exports `formatReleaseDate` and `formatDetectedDate`, so the inventory is wrong. It is also the long header style that CLAUDE.md's comment discipline asks to avoid.
**Fix:** Replace the inventory with a 1-3 line intent summary. Per-function comments already document each formatter.

### IN-02: One History card mixes two date formats

**File:** `web/app/components/history/EventCard.tsx:86-96` (vs `122`, `176`, `212`)
**Issue:** The new line renders `Detected 11 Oct 2026`. The release date in all three bodies is still the raw ISO string, `event.release_date ?? "Release date unknown"`, which renders as `2015-04-08`. The `formatDetectedDate` comment says it "match[es] formatReleaseDate", but EventCard never calls `formatReleaseDate`. One card now shows `2015-04-08` directly above `Detected 11 Oct 2026`.
**Fix:** Render release dates through `formatReleaseDate(event.release_date)` in `NewReleaseBody`, `GuestFeatureBody` and `DeluxeChangeBody`. Then update the `getByText(/2015-04-08/)` assertion.

### IN-03: The local-calendar behavior of `formatDetectedDate` is untested

**File:** `web/app/lib/format.test.ts:142-153`
**Issue:** Vitest pins `TZ=UTC` (`web/vitest.config.ts`), so `getDate()` and `getUTCDate()` give identical results. Swapping to UTC accessors, or adopting a different local-day rule, would pass every case. The function's stated intent is a local date.
**Fix:** Add a case that derives the expectation from local accessors on an instant near midnight. Alternatively, document that local-vs-UTC is covered only by `formatAbsoluteTime`'s existing TZ self-check.

### IN-04: The sort-announcement tests can pass without cancellation on a slow runner

**File:** `web/app/routes/watchlist.test.tsx:1404-1451`
**Issue:** `shouldAdvanceTime: true` advances the fake clock with real time. If typing "a" and picking the sort option take more than 500ms of real time, the count announcement fires **before** the sort. The sort message then overwrites it, and "cancels a pending count announcement" passes even if `handleSortChange` stopped clearing `countTimer`.
**Fix:** Before the sort pick, assert that the timer is still pending, for example `expect(status()).not.toHaveTextContent("Showing")` right after typing. That way a vacuous run fails instead of passing.

### IN-05: G-25-4 has jsdom-only coverage, while the gap's debug plan required a browser-level check

**File:** `web/app/lib/useScrollAnchor.test.tsx`, `web/app/routes/watchlist.test.tsx:2099-2211`
**Issue:** Every anchor test mocks `getBoundingClientRect` and `scrollBy`. Three things the real failure depends on are never exercised:
- the two-commit transition ordering (WR-01),
- native CSS scroll anchoring (`overflow-anchor`),
- browser clamping on short pages.

The G-25-4 diagnosis listed "Browser-level test (not jsdom) that the clicked chip stays roughly in place" as a required artifact.
**Fix:** Add a Playwright-style check, or record a manual UAT step that covers a sticky-card chip click and an un-press with two or more tags selected (WR-01, WR-02).

---

_Reviewed: 2026-10-11T20:41:29Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
