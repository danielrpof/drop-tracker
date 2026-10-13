---
phase: 25-find-filter-watchlist-and-history
reviewed: 2026-10-12T22:34:33Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.tsx
  - web/app/components/watchlist/TagChips.test.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/lib/useScrollAnchor.test.tsx
  - web/app/lib/useScrollAnchor.ts
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
findings:
  critical: 0
  warning: 2
  info: 6
  total: 8
status: issues_found
---

# Phase 25: Code Review Report (gap closure 25-19 to 25-21)

**Reviewed:** 2026-10-12T22:34:33Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

_This report replaces the 2026-10-12T20:25:38Z report (0 critical, 3 warning, 3 info), which is kept in git history at `fc4b2d4`. It covers `fc4b2d4..HEAD` for the eight files above._

## Summary

This round covers 25-19 (G-25-11, every sibling card as a fallback anchor), 25-20 (G-25-12, focus after a keyboard toggle removes its card) and 25-21 (G-25-13, `aria-describedby` on the disabled Deezer button). The four scoped test files pass locally (`vitest run --coverage.enabled=false`, 147 tests).

Status of the prior warnings:

| Prior | Verdict | Notes |
|-------|---------|-------|
| WR-01 (fallback checks only the two immediate neighbours) | **Resolved** | `anchorCandidates` walks outward over all sibling cards, next before previous at each distance (`useScrollAnchor.ts:5-19`). The old "no candidate survives" test now drops every card, and new hook and route tests cover runs of leaving cards. A new problem with *distant* survivors is described in WR-02 below. |
| WR-02 (Deezer hint has no accessible description) | **Resolved, with a regression** | The button now has a description through `aria-describedby` (`SearchResultsColumns.tsx:206, 211-213`), and the test asserts `toHaveAccessibleDescription`. The description element is a visible-to-AT `sr-only` span, so the hint is read twice. See WR-01 below. The touch gap is closed by user decision (no visible note). |
| WR-03 (focus falls to `<body>` when the clicked card leaves) | **Resolved** | The keyboard-only move is detected with `detail === 0`, gated on held focus and on focus having fallen to the document, and reported through `useEffectEvent`. It targets the fallback card's pressed chip, otherwise the `tabIndex={-1}` heading. I traced last-card, middle-card, run, no-survivor, mouse and focus-moved-away paths. They behave as the UI-SPEC row says. |

Prior IN-01 and IN-02 were outside this round's plans and are still open (IN-05 and IN-06 below). Prior IN-03 carries over as IN-07.

I traced the capture-to-layout-effect timing for the toggle path. The anchor is captured synchronously in the click. The key change, the sticky reset (`reconcileSticky` during render) and the card unmount all land in one commit, so the layout effect sees the final DOM. `useEffectEvent` impls are committed before layout effects run, so `reportFocusLost` always calls the current `focusAfterCardLeaves`. I found no security issues. The new selectors are constant literals, and the hint span renders a static constant.

## Warnings

### WR-01: The Deezer hint is read twice in browse mode, because the description element is an `sr-only` span rather than a hidden one

**File:** `web/app/components/watchlist/SearchResultsColumns.tsx:198-213`; `web/app/components/watchlist/SearchResultsColumns.test.tsx:234-237, 346-348`
**Issue:** `sr-only` hides the span visually but keeps it in the accessibility tree as ordinary text. In browse mode (NVDA, JAWS, VoiceOver virtual cursor), the reader reaches the button and announces "Search MusicBrainz to add, button, unavailable, This project can't yet match…", taking the hint from `aria-describedby`. It then reaches the sibling span and reads the same 130-character sentence again as plain text.

The wrapper `title` is a third source of the same text. Some readers expose it on the generic `<span>`. The new tests lock the doubled tree in: `toHaveClass("sr-only")` at line 236, and the row `textContent` assertion at line 347 expects the hint to be part of the row's text.
**Fix:** Keep the referenced element out of the tree but still referenceable. Per accname, `aria-describedby` follows a direct reference into hidden content:
```tsx
<span id={hintId} hidden>
  {DEEZER_ADD_HINT}
</span>
```
`toHaveAccessibleDescription` still passes, because dom-accessibility-api follows direct references into hidden nodes. Replace `toHaveClass("sr-only")` with `toHaveAttribute("hidden")`. The row `textContent` assertion is unchanged, since `textContent` includes hidden nodes. If you want `getByRole` and `toBeVisible` checks to prove it is out of the tree, add `expect(screen.queryByText(DEEZER_ADD_HINT)).not.toBeVisible()`.

### WR-02: A distant fallback survivor is pinned at its original off-screen offset, so the viewport jumps and the G-25-12 focus target can land off-screen with `preventScroll`

**File:** `web/app/lib/useScrollAnchor.ts:53-57`; `web/app/routes/watchlist.tsx:197-203`
**Issue:** The correction keeps the chosen candidate at the viewport-relative `top` it had at capture time. For the chip, or an adjacent card, that is near the click point. 25-19 now lets the candidate be any distance away, so it can be a card that was off-screen at capture.

Worked example: cards are about 166px apart in an 800px viewport, the clicked card is at y=400, and the four cards on each side leave with it (an un-pressed tag that most cards carry, with the 50+ artists used in UAT). The chosen survivor `C+5` was at y≈1230, below the fold. After `scrollBy(0, -1494)` it is at 1230 again. The viewport now shows `C-7..C-11`, cards that were more than 1000px above the user's position. Neither nearest survivor is on screen. That is the G-25-4 "I lost my place" symptom, now in the other direction.

G-25-12 then focuses `C+5`'s chip with `{ preventScroll: true }` (`watchlist.tsx:202`). A sighted keyboard user's focus is on an element outside the viewport, and the next Tab or Shift+Tab scrolls the page again.

The route test at `watchlist.test.tsx` "corrects by the nearest survivor's drift when a run of cards leaves (G-25-11)" uses a survivor top of 900, which is already below an 800px fold, so it pins this behaviour without noticing it.
**Fix:** When the anchor is a fallback (`a.el !== captured.el`), aim for the click position instead of the survivor's own old offset. At minimum, clamp the target into the viewport:
```ts
const rect = a.el.getBoundingClientRect()
const want =
  a.el === captured.el
    ? a.top
    : Math.min(Math.max(captured.candidates[0].top, 0), window.innerHeight - rect.height)
const delta = rect.top - want
if (delta !== 0) window.scrollBy(0, delta)
```
`candidates[0]` is the chip, so its captured top is where the user was looking. Add a route or hook case where the survivor is more than one viewport away, and assert that it ends near the click point. If the UI-SPEC D-12 wording ("where it was on screen") is meant literally for distant survivors, record that decision instead. In either case, the focus target should not stay off-screen: after focusing, call `target.scrollIntoView({ block: "nearest" })` when its rect is outside `[0, innerHeight]`.

## Info

### IN-01: A chipless fallback card sends focus to the heading even when a nearer card with chips survives

**File:** `web/app/routes/watchlist.tsx:197-203`; `web/app/lib/useScrollAnchor.ts:53, 64`
**Issue:** `onFocusLost` receives only the first connected candidate. If that card has no `[data-chip-label]`, focus goes to the heading, which can be far from the click. This can happen when a sticky card (one that fails `q`, `muted` or `custom`) un-presses the last selected tag. `tagIds` becomes empty, the sticky reset removes the clicked card, and the nearest survivor can be an untagged artist.
**Fix:** Pass the ordered list of connected candidates (or let the route walk them) and take the first one that has a chip label, before falling back to the heading.

### IN-02: The newly focusable heading keeps `outline-none`, so programmatic focus has no visible indicator

**File:** `web/app/routes/watchlist.tsx:599-604`
**Issue:** `outline-none` dates from 25-04, when the `<h2>` could not take focus. In the no-survivor path, a sighted keyboard user's focus now lands on it with no ring. A `tabIndex={-1}` heading is not in the tab order, so this is not a strict 2.4.7 failure, but the user cannot see where focus went.
**Fix:** Use `focus-visible:ring-[3px] focus-visible:ring-ring/50` (the same ring the chip labels use), or document in the UI-SPEC that this is an intentional choice.

### IN-03: The `detail === 0` comment overstates the signal

**File:** `web/app/components/watchlist/TagChips.tsx:198-200`
**Issue:** "detail is 0 only for an Enter or Space activation" is not accurate. A programmatic `.click()` and many assistive-tech default actions also dispatch `detail: 0`. The 25-20 summary already notes this, and the held-focus and focus-fell-to-document guards in the hook make it safe, but the inline comment tells the next reader something false.
**Fix:** Reword it, for example: "`detail` is 0 for keyboard or synthetic activation. The hook's focus guards make that safe (G-25-12)."

### IN-04: The hint copy is no longer pinned by any test

**File:** `web/app/components/watchlist/SearchResultsColumns.test.tsx:7, 200, 234`
**Issue:** The new assertions compare against the imported `DEEZER_ADD_HINT`, so a copy change or an accidentally emptied constant still passes. The only literal check is `toContain("MusicBrainz identity")` on the wrapper title (line 190).
**Fix:** Assert the UI-SPEC literal once, for example `expect(DEEZER_ADD_HINT).toBe("This project can't yet match …")`, or use a literal in `toHaveAccessibleDescription`.

### IN-05: Dead read-order stub in the previous-card route test (carried over from prior IN-01)

**File:** `web/app/routes/watchlist.test.tsx:2335-2349`
**Issue:** It is unchanged. The label is measured once and is disconnected after the commit, so the `120` branch never runs, and the comment ("key the rect on read order") misdescribes why the test passes.
**Fix:** Drop the stub, or replace it with `expect(label.isConnected).toBe(false)`.

### IN-06: `clearFilters` hand-derives "will the filter key change" (carried over from prior IN-02)

**File:** `web/app/routes/watchlist.tsx:531`
**Issue:** It is unchanged. The predicate duplicates `filterKeyOf` minus `sort`/`q`, and there is still no sticky-then-Clear-filters test.
**Fix:** Use `if (filterKeyOf({ ...view, tagIds: [], muted: false, custom: false }) === filterKey) clearSticky()`, or call `clearSticky()` unconditionally.

### IN-07: Real-browser checks are still outstanding (carried over from prior IN-03)

**File:** `web/app/lib/useScrollAnchor.test.tsx`; `web/app/routes/watchlist.test.tsx:2392-2600`; `web/app/components/watchlist/SearchResultsColumns.test.tsx:167-239`
**Issue:** Every anchor and focus test stubs `getBoundingClientRect` and `scrollBy`, and jsdom has no screen reader. 25-21 D3 is `human_judgment: true`. The open items are native `overflow-anchor` interplay and page-end clamping, WR-02's distant-survivor case, focus-ring visibility after the move, and how often NVDA/VoiceOver announce the Deezer hint (WR-01).
**Fix:** In `/gsd-verify-work` on the rebuilt binary, run these three checks. First, un-press a tag that removes a long run of cards in a list of 50+ artists. Second, press Enter on the last card's chip and confirm focus is visible and on screen. Third, read a Deezer row in browse mode.

---

_Reviewed: 2026-10-12T22:34:33Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
