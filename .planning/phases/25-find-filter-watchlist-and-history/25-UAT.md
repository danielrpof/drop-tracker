---
status: testing
phase: 25-find-filter-watchlist-and-history
source: [25-VERIFICATION.md]
started: 2026-10-10T18:15:00Z
updated: 2026-10-10T18:15:00Z
---

## Current Test

number: 1
name: One request on open, instant search/sort/filter (50+ artists)
expected: |
  Exactly one GET /watchlist on open and zero further requests while searching/sorting/filtering. Beyoncé matches `beyonce`. No browser history-throttling warning. Reload and Back reproduce the filtered view.
awaiting: user response

## Tests

### 1. One request on open, instant search/sort/filter (50+ artists)
expected: Exactly one GET /watchlist on open and zero further requests while searching/sorting/filtering. Beyoncé matches `beyonce`. No browser history-throttling warning. Reload and Back reproduce the filtered view.
result: [pending]

### 2. Sort stability on real data
expected: Equal-valued artists never swap when direction flips; every `No releases yet` artist stays at the bottom under both latest-release directions.
result: [pending]

### 3. Add/refresh under active filters
expected: Selected tag survives the refresh after adding an artist; an artist added under `Has muted events` is visible; unchecking a last mute keeps the card until the next filter change.
result: [pending]

### 4. Scroll position holds
expected: Clicking a chip label or toggling `Has muted events` far down a 50+ artist list never jumps to the top.
result: [pending]

### 5. Contrast of chip × and pressed-toggle Check
expected: Both meet 3:1 non-text contrast against #27272a (about 5.8:1 per UI-SPEC).
result: [pending]

### 6. 375px truncation
expected: 32-char tags (tag filter, chip label with Check, History Tag trigger), long artist names, long query and a malformed date truncate (title carries full label) or wrap without overlap; clear × never overlaps query text; focus-within un-truncate works on chip labels.
result: [pending]

### 7. History tag filter across the retention edge
expected: No event older than the retention window ever appears through Load more; reload keeps the tag; Back restores the previous combination. Note whether changing the Tag mid-Load-more leaks rows from the previous filter (advisory WR-01).
result: [pending]

### 8. Watchlist and History filter state stay separate
expected: Watchlist owns q/sort/tags/muted/custom, History owns artist/type/tag, nav links carry no params.
result: [pending]

## Summary

total: 8
passed: 0
issues: 0
pending: 8
skipped: 0
blocked: 0

## Gaps
