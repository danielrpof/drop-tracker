---
status: diagnosed
phase: 25-find-filter-watchlist-and-history
source: [25-VERIFICATION.md]
started: 2026-10-10T18:15:00Z
updated: 2026-10-11T13:10:00Z
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
  root_cause: "No code scrolls the page. Clicking a chip label filters 50+ cards down to a few, so the document gets no taller than the window (body is min-h-screen) and the browser clamps scrollY to max(0, scrollHeight - innerHeight), which is 0. preventScrollReset already works. UI-SPEC D-12 treated ScrollRestoration as the only cause of a jump to the top. The muted toggle drops the same way, but the toolbar can only be reached near the top of the page, so it doesn't show."
  artifacts:
    - path: "web/app/routes/watchlist.tsx"
      issue: "toggleFilterTag/handleTagsChange filter the list without keeping the clicked card in place"
    - path: ".planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md"
      issue: "D-12 (line ~399, 639) assumes preventScrollReset alone keeps scrollY"
  missing:
    - "Product decision: literal scrollY preservation is impossible when the filtered list fits in one screen; recommended contract is 'router never resets scroll; clicked card stays at its on-screen position (clamped)'"
    - "Record the chip's getBoundingClientRect().top before writing the filter, and after the re-render scroll by the difference (useLayoutEffect)"
    - "Browser-level test (not jsdom) that the clicked chip stays roughly in place"
  decision: "User chose (2026-10-11): anchor the clicked card. After a chip-label filter, the clicked chip/card keeps its on-screen position (the browser clamps it when the page is short). Reword the truth to: the router never resets scroll and the clicked card stays where it was on screen."
  debug_session: ".planning/debug/chip-click-scroll-reset.md"

- gap_id: G-25-9
  truth: "At 375px the watchlist search input shows enough of the typed artist name to read it; the \"Search MusicBrainz to add\" button truncates/shrinks before the query does."
  status: failed
  reason: "User reported: on 375px, when searching an artist, only the first letter shows in the search and the rest is truncated, this is because the entire \"Search Musicbrainz to add\" button isnt truncated, it should be the opposite, have the musicbrainz button truncated shorter so more of the artitst name can be shown in the search"
  severity: minor
  test: 9
  root_cause: "The text cut to one letter is the artist name in the Deezer search-result row (SearchResultRow in SearchResultsColumns.tsx), not the search input. The 'Search MusicBrainz to add' button is shrink-0 with no width cap (205px). The name column is flex-1 min-w-0 with a 0 starting width, so at 375px it gets 10px. Removing shrink-0 alone does nothing. The type span (line 178) also has no truncate. The layout dates from phase 6; 640px (the sm two-column grid) is worse, with a 0px name."
  artifacts:
    - path: "web/app/components/watchlist/SearchResultsColumns.tsx"
      issue: "lines 190-198 Deezer button shrink-0 and no cap; line 174 name column starts at 0 width; line 178 type span has no truncate"
    - path: "web/app/routes/watchlist.tsx"
      issue: "line 491 p-8 padding at 375px makes it worse"
  missing:
    - "Cap the button width and put the label in an inner <span className='truncate'>, or use a shorter label below sm/md with the full text in title/aria-label, or stack the button under the name"
    - "Add truncate to the type span"
    - "Check at 375px and 640px; update SearchResultsColumns.test.tsx:163,224 if the label changes"
  debug_session: ".planning/debug/search-input-squeezed-375.md"

- gap_id: G-25-7
  truth: "No event older than the retention window ever appears through Load more on the History tag filter"
  status: failed
  reason: "User reported: using load more shows events older than the retention edge, rest pass"
  severity: major
  test: 7
  root_cause: "Not a leak. ListEvents applies the created_at >= cutoff condition on every page and on the tag-filtered variant (verified through the API and the DB: 0 rows detected more than 90 days ago). Retention counts from detection time (Phase 10 D-01), but quick task 260825-g6i made the feed sort by release_date DESC, and EventCard shows only release_date. An artist's back catalogue is inserted with today's created_at, so Load more pages back through years-old releases that retention correctly keeps. The dev DB has no events past the retention edge (oldest detection is 49 days ago), so test 7's precondition couldn't be met."
  artifacts:
    - path: "queries/events.sql"
      issue: "ListEvents filters on created_at but sorts by release_date; correct against D-01 but surprises the user"
    - path: "web/app/components/history/EventCard.tsx"
      issue: "shows only release_date, so the date retention uses is invisible"
    - path: "web/app/routes/history.tsx"
      issue: "nothing in a non-empty feed marks where the retention edge is"
    - path: "internal/httpserver/events_test.go (TestRetention_TagFilterComposesWithCutoff)"
      issue: "uses insertTestEventAt with no release_date, so it never covers 'detected recently, released long ago'"
  missing:
    - "Product decision: (A) keep D-01 and fix the UAT spec and UI (show the detection date or add an end-of-feed retention note, plus an automated test), or (B) switch retention to release_date, which reverses D-01 and hides newly added artists' back catalogues"
    - "Re-run test 7 with EVENT_RETENTION_DAYS=45 so there are rows on both sides of the edge"
  decision: "User chose (2026-10-11): keep D-01 (retention counts from detection time) and clarify the UI. Show the detected date on History event cards (or an end-of-feed retention note), add a detected-recently/released-long-ago regression test, and reword UAT test 7 / the verification check to say retention counts from detection."
  debug_session: ".planning/debug/history-load-more-retention-leak.md"

## Execution Waivers

- waiver: "CI-only race (user decision 2026-10-11). The user accepted running the gap-closure integration suite without -race locally, because cgo is disabled on this Windows box. CI's race job remains the gate. Applies to gap-closure plans 25-11..25-14 only."
