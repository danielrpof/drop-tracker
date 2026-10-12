---
phase: 25-find-filter-watchlist-and-history
reviewed: 2026-10-12T20:25:38Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.tsx
  - web/app/lib/useScrollAnchor.test.tsx
  - web/app/lib/useScrollAnchor.ts
  - web/app/lib/watchlistView.test.ts
  - web/app/lib/watchlistView.ts
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
findings:
  critical: 0
  warning: 3
  info: 3
  total: 6
status: issues_found
---

# Phase 25: Code Review Report (gap closure 25-15 to 25-17)

**Reviewed:** 2026-10-12T20:25:38Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

_The prior report (2026-10-11, 0 critical, 4 warning, 5 info) is replaced. This report covers `efcb6ec..HEAD` for the eight files above._

## Summary

This re-review covers gap-closure plans 25-15 (sticky set keyed to the filter, carried across the route's own rewrites), 25-16 (Deezer hint title moved to a wrapper), and 25-17 (fallback scroll-anchor candidates). The four scoped test files pass locally (`vitest run`, 158 tests).

Status of the prior warnings:

| Prior | Verdict | Notes |
|-------|---------|-------|
| WR-01 (sticky card unmounts before the filter commits) | **Resolved** | `reconcileSticky` runs during render (`watchlist.tsx:95-101`), so the reset lands in the same transition render as the location. The sync `clearSticky()` calls were removed from `handleTagsChange`, `handleFlagChange` and `handleSortChange`. A sticky card's chip click can only add a tag the card carries, so the card matches and stays mounted. The route test at `watchlist.test.tsx:2392` pins `isConnected`, node identity, focus and the `scrollBy` correction, and its RED evidence shows it failed against the old code. |
| WR-02 (anchor dropped when the clicked card leaves) | **Partially resolved** | Only the two immediate neighbours are tried. If they also leave, the collapse comes back. See WR-01 below. Focus loss on the same path is still unhandled. See WR-03 below. |
| WR-03 (`filterKey` effect clears on the route's own writes, after paint) | **Resolved** | The post-paint effect is gone. `writeKeepingSticky` and `carrySticky` keep stickies for a replace landing on the target key only. User pushes and Back/Forward pops still clear. Merge and delete route tests cover it (`watchlist.test.tsx:1709, 1740`), and the commit-probe Back test (`2068`) proves the same-commit drop. A Back change that only touches `q` still clears after paint through the `view.q` effect. That is acceptable, because `query` itself also resyncs in that effect. |
| WR-04 (title unreachable on a disabled button) | **Partially resolved** | Mouse hover now reaches the wrapper's `title`. Moving it off the button removed the hint's accessible description, and touch and keyboard users still have no way to read it. See WR-02 below. |

I traced the carry state machine (`reconcileSticky` / `carrySticky`) through these paths: merge, delete, no-op `replaceSelectedTag`, canonicalization after a pop, first-load validation, a same-tick write layered on an unlanded push, and the q debounce. It converges, and it does not keep stickies across a user filter change. I found no security issues.

## Warnings

### WR-01: The fallback anchor only checks the two immediate neighbours, so the G-25-4 collapse returns when a run of cards leaves

**File:** `web/app/lib/useScrollAnchor.ts:3-10, 12-14, 28-29`; `web/app/lib/useScrollAnchor.test.tsx:217-226`
**Issue:** `anchorCandidates` returns `[chip, card.nextElementSibling, card.previousElementSibling]`. Tag filtering is any-of (`matchesAnyTag`). When a selected chip is un-pressed, every card that carries only that tag leaves, not just the clicked one.

With the default name sort, cards that carry only the un-pressed tag are interleaved with the others, so runs of adjacent leavers are common. Example: `?tags=3,7` over ten cards where five carry only tag 3. If the clicked card's next and previous siblings also carry only tag 3, every candidate is disconnected. The layout effect then returns without correcting, which is the original "it drops after clicking a tag" symptom.

The hook test "calls no scroll API when no candidate survives" (line 217) treats this as the intended outcome. The header comment also overstates the behaviour: "or its nearest surviving neighbour" is really "one of its two adjacent cards".
**Fix:** Capture every sibling card's top at click time. After the commit, choose the connected one closest in DOM order to the clicked card:
```ts
export function anchorCandidates(el: HTMLElement): HTMLElement[] {
  const card = el.closest("li")
  if (!card?.parentElement) return [el]
  const cards = [...card.parentElement.children].filter(
    (c): c is HTMLElement => c instanceof HTMLElement
  )
  const i = cards.indexOf(card)
  const out: HTMLElement[] = [el]
  for (let d = 1; d < cards.length; d++) {
    if (cards[i + d]) out.push(cards[i + d])
    if (cards[i - d]) out.push(cards[i - d])
  }
  return out
}
```
If measuring every card at click time is a concern, cap `d` at a small window, such as 10. Then change the line-217 test so that a surviving card two positions away is used. Also fix the header comment, or the UI-SPEC D-12 line if the two-neighbour limit is intentional.

### WR-02: Moving `title` to the wrapper strips the Deezer hint's accessible description, and touch and keyboard users still cannot read it

**File:** `web/app/components/watchlist/SearchResultsColumns.tsx:195-203`; `web/app/components/watchlist/SearchResultsColumns.test.tsx:183-194`
**Issue:** Before 25-16, `title` sat on the native `<button disabled>`. Per accname/HTML-AAM, it was that button's accessible description, so a screen reader in browse mode announced "Search MusicBrainz to add, button, unavailable" followed by the reason. The title now sits on a generic `<span>`. Assistive tech does not reliably expose a `title` on a generic element, and the button has no description left.

The test asserts `expect(button).not.toHaveAttribute("title")`, which locks the regression in. The touch gap from the prior review is still open: touch has no hover, and the truncated label, around "Search Mus…" at 375px, is all that touch users see. The disabled button is also not focusable, so keyboard users cannot reach the hint either.
**Fix:** Keep the wrapper `title` for mouse users, and attach the hint to the button with `aria-describedby`:
```tsx
const hintId = useId()
// ...
<span title={DEEZER_ADD_HINT} className="max-w-[40%] shrink-0 lg:max-w-none">
  <Button variant="secondary" size="sm" disabled className="w-full" aria-describedby={hintId}>
    <span className="truncate">Search MusicBrainz to add</span>
  </Button>
  <span id={hintId} className="sr-only">{DEEZER_ADD_HINT}</span>
</span>
```
In the test, replace `not.toHaveAttribute("title")` with `expect(button).toHaveAccessibleDescription(DEEZER_ADD_HINT_TEXT)`. For touch, consider a visible one-line note under the Deezer column heading, like the existing "unavailable" `<p>`, instead of relying on per-row hover.

### WR-03: When the clicked card leaves the list, the focused chip is destroyed and focus falls to `<body>`

**File:** `web/app/routes/watchlist.tsx:491-497`; `web/app/lib/useScrollAnchor.ts:25-32`
**Issue:** 25-17 now handles scroll position on the "clicked card leaves" path, but not focus. When a selected chip is un-pressed and its card no longer matches, the focused label button unmounts with the card, and `document.activeElement` becomes `<body>`. A keyboard user (Enter/Space on the chip) is sent back to the top of the tab order.

The UI-SPEC focus table only defines "Card chip label | Toggled, card still visible | stays on that label button", so there is no rule for this case. The previous-card and next-card route tests (`watchlist.test.tsx:2330, 2367`) never assert where focus goes.
**Fix:** After the scroll correction in the layout effect, move focus to a sensible target if the captured element had focus and is now disconnected. The surviving fallback card's first chip label or the card itself works, or the list heading (`#watchlist-artists-heading`) with `tabIndex={-1}`:
```ts
const hadFocus = anchor.current?.[0]?.el === document.activeElement // capture time
// in the layout effect, after scrollBy:
if (hadFocus && !candidates[0].el.isConnected) {
  ;(a.el.querySelector<HTMLElement>("button") ?? a.el).focus({ preventScroll: true })
}
```
Add a "Toggled, card leaves" row to the UI-SPEC focus table, and assert focus in the two WR-02 route tests.

## Info

### IN-01: Dead stub and a misleading comment in the previous-card route test

**File:** `web/app/routes/watchlist.test.tsx:2336-2351`
**Issue:** The comment says "The label detaches, so key the rect on read order: 400 first, then 120". The label is read once at capture time. After the commit it is disconnected and skipped by `find(isConnected)`, so the `120` branch never runs. The test passes because of the `Daddy Yankee` `stubCardTop` alone.
**Fix:** Drop the read-order mock, or replace it with `expect(label.isConnected).toBe(false)` so the test states why the fallback is used.

### IN-02: `clearFilters` re-derives "will the filter key change" by hand, and neither branch is tested

**File:** `web/app/routes/watchlist.tsx:517`
**Issue:** `if (view.tagIds.length === 0 && !view.muted && !view.custom) clearSticky()` duplicates the knowledge that `CLEAR_FILTER_KEYS` minus `q` makes up the URL-owned part of `filterKeyOf`. Adding a key to either list silently breaks the predicate. No route test makes a card sticky and then runs Clear filters (`grep` finds no sticky assertion near any "Clear filters" click).
**Fix:** Derive the predicate from the key itself, for example `if (filterKeyOf({ ...view, tagIds: [], muted: false, custom: false }) === filterKey) clearSticky()`. Alternatively, call `clearSticky()` unconditionally, since it is idempotent and the landing reset makes it harmless. Add one sticky-then-Clear-filters test for the q-only branch.

### IN-03: The browser-level checks behind WR-01, WR-02 and WR-04 are still outstanding

**File:** `web/app/lib/useScrollAnchor.test.tsx`; `web/app/routes/watchlist.test.tsx:2222-2428`; `web/app/components/watchlist/SearchResultsColumns.test.tsx:167-199`
**Issue:** All anchor tests stub `getBoundingClientRect` and `scrollBy`. 25-15 D4, 25-16 D2, 25-17 D3 and 25-18 D4 are `human_judgment: true` and deferred to `/gsd-verify-work`. These checks are still unverified:
- how the hook's `scrollBy` interacts with native `overflow-anchor` (Chromium/Firefox) and with page-end clamping;
- the native tooltip actually appearing through `disabled:pointer-events-none`.

This carries over prior IN-05. Prior IN-01 to IN-04 (format.ts header, EventCard date mix, TZ-pinned `formatDetectedDate`, sort-announcement timing) were outside 25-15 to 25-17 and are unchanged.
**Fix:** Run the UAT steps in a real browser before closing the phase: a sticky-card chip click, un-pressing with two or more tags selected (last card, middle card, and a run of adjacent leavers), and Deezer pill hover at 375/640/1024px.

---

_Reviewed: 2026-10-12T20:25:38Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
