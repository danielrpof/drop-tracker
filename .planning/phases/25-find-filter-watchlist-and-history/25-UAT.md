---
status: complete
phase: 25-find-filter-watchlist-and-history
source: [25-VERIFICATION.md]
started: 2026-10-10T18:15:00Z
updated: 2026-10-11T12:35:00Z
---

## Current Test

[testing complete]

## Tests

### 1. One request on open, instant search/sort/filter (50+ artists)
expected: Exactly one GET /watchlist on open and zero further requests while searching/sorting/filtering. Beyoncé matches `beyonce`. No browser history-throttling warning. Reload and Back reproduce the filtered view.
result: pass

### 2. Sort stability on real data
expected: Equal-valued artists never swap when direction flips; every `No releases yet` artist stays at the bottom under both latest-release directions.
result: pass

### 3. Add/refresh under active filters
expected: Selected tag survives the refresh after adding an artist; an artist added under `Has muted events` is visible; unchecking a last mute keeps the card until the next filter change.
result: pass

### 4. Scroll position holds
expected: Clicking a chip label or toggling `Has muted events` far down a 50+ artist list never jumps to the top.
result: issue
reported: "it drops to 0 after clicking a tag"
severity: major

### 5. Contrast of chip × and pressed-toggle Check
expected: Both meet 3:1 non-text contrast against #27272a (about 5.8:1 per UI-SPEC).
result: pass

### 6. 375px truncation
expected: 32-char tags (tag filter, chip label with Check, History Tag trigger), long artist names, long query and a malformed date truncate (title carries full label) or wrap without overlap; clear × never overlaps query text; focus-within un-truncate works on chip labels.
result: pass

### 7. History tag filter across the retention edge
expected: No event older than the retention window ever appears through Load more; reload keeps the tag; Back restores the previous combination. Note whether changing the Tag mid-Load-more leaks rows from the previous filter (advisory WR-01).
result: issue
reported: "using load more shows events older than the retention edge, rest pass"
severity: major

### 8. Watchlist and History filter state stay separate
expected: Watchlist owns q/sort/tags/muted/custom, History owns artist/type/tag, nav links carry no params.
result: pass

### 9. Search query stays readable at 375px (ad-hoc, found during test 6)
expected: At 375px the watchlist search input shows enough of the typed artist name to read it; the "Search MusicBrainz to add" button truncates/shrinks before the query does.
result: issue
reported: "on 375px, when searching an artist, only the first letter shows in the search and the rest is truncated, this is because the entire \"Search Musicbrainz to add\" button isnt truncated, it should be the opposite, have the musicbrainz button truncated shorter so more of the artitst name can be shown in the search"
severity: minor

## Summary

total: 9
passed: 6
issues: 3
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-25-4
  truth: "Clicking a chip label or toggling `Has muted events` far down a 50+ artist list never jumps to the top."
  status: failed
  reason: "User reported: it drops to 0 after clicking a tag"
  severity: major
  test: 4
  artifacts: []  # Filled by diagnosis
  missing: []    # Filled by diagnosis

- gap_id: G-25-9
  truth: "At 375px the watchlist search input shows enough of the typed artist name to read it; the \"Search MusicBrainz to add\" button truncates/shrinks before the query does."
  status: failed
  reason: "User reported: on 375px, when searching an artist, only the first letter shows in the search and the rest is truncated, this is because the entire \"Search Musicbrainz to add\" button isnt truncated, it should be the opposite, have the musicbrainz button truncated shorter so more of the artitst name can be shown in the search"
  severity: minor
  test: 9
  artifacts: []  # Filled by diagnosis
  missing: []    # Filled by diagnosis

- gap_id: G-25-7
  truth: "No event older than the retention window ever appears through Load more on the History tag filter"
  status: failed
  reason: "User reported: using load more shows events older than the retention edge, rest pass"
  severity: major
  test: 7
  artifacts: []  # Filled by diagnosis
  missing: []    # Filled by diagnosis
