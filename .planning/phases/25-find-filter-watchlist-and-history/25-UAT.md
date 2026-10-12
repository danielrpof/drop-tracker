---
status: diagnosed
phase: 25-find-filter-watchlist-and-history
source: [25-VERIFICATION.md]
started: 2026-10-10T18:15:00Z
updated: 2026-10-12T21:40:00Z
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

### 4. Scroll position holds (re-test after 25-11/15/17/18)
expected: (1) After removing a tag from a card that has a second tag, clicking its second chip label keeps the clicked card where it was on screen, and its label keeps the focus ring. (2)(3) Un-pressing a chip on a card that leaves keeps the next surviving neighbour (or the previous one) where it was on screen, clamped only when the page is shorter than the window. (4) Toggling Has muted events never makes the router reset scroll. There's no fight between the anchor scrollBy and native overflow-anchor.
result: pass
waiver: "User passed without a real-browser run (2026-10-12): seeding 50+ artists was too much setup. Covered only by jsdom anchor tests that stub getBoundingClientRect/scrollBy."
previous_result: issue (reported: "it drops to 0 after clicking a tag") — gap resolved, re-testing
### 5. Contrast of chip × and pressed-toggle Check
expected: Both meet 3:1 non-text contrast against #27272a (about 5.8:1 per UI-SPEC).
result: pass

### 6. 375px truncation
expected: 32-char tags (tag filter, chip label with Check, History Tag trigger), long artist names, long query and a malformed date truncate (title carries full label) or wrap without overlap; clear × never overlaps query text; focus-within un-truncate works on chip labels.
result: pass

### 7. History tag filter across the retention edge (re-test after 25-12/14)
expected: With EVENT_RETENTION_DAYS=45: pick a tag, press Load more until it disappears. No card shows a Detected date older than the retention window, even when its release date is years old. Reload keeps the tag, and Back restores the previous combination.
result: pass
previous_result: issue (reported: "using load more shows events older than the retention edge, rest pass") — gap resolved, re-testing
### 8. Watchlist and History filter state stay separate
expected: Watchlist owns q/sort/tags/muted/custom, History owns artist/type/tag, nav links carry no params.
result: pass

### 9. Search result name readable at 375px (re-test after 25-13/16)
expected: Search `Bad Bunny`. At 375px and 640px the Deezer artist name is readable and the `Search Mus…` pill truncates before the name does. Hovering the pill shows the native tooltip with the full hint. At 1024px the full label shows.
result: pass
previous_result: issue (reported: "on 375px, when searching an artist, only the first letter shows in the search and the rest is truncated, this is because the entire \"Search Musicbrainz to add\" button isnt truncated, it should be the opposite, have the musicbrainz button truncated shorter so more of the artitst name can be shown in the search") — gap resolved, re-testing
### 10. Edited card survives tag merge/delete; Back shows no stale card (25-18)
expected: Filter by a tag, remove it from a card, then open Manage tags and merge that tag into another (or delete it while another tag stays selected). The edited card stays visible. Separately, make a card sticky under a toggled filter, then press Back. Back never shows the stale card, not even for one frame.
result: pass

### 11. Decision — run of adjacent leaving cards (review WR-01)
expected: At `?tags=A,B`, un-press a chip on a card whose next and previous cards also have only that tag. Today no correction runs and the page can collapse under the viewport. Decide whether to accept this as a documented limitation or widen the search to the nearest surviving card.
result: issue
reported: "widen"
severity: minor

### 12. Decision — focus when the clicked card leaves (review WR-03)
expected: With two tags selected, press Enter on a chip label so the card leaves the list. Today focus falls to <body>. Decide whether to accept this, or move focus to the surviving fallback card's chip label (or the list heading) and add a 'Toggled, card leaves' focus row.
result: issue
reported: "anchor"
severity: minor

### 13. Decision — Deezer hint accessible description (review WR-02)
expected: With a screen reader on a Deezer result row, the disabled button announces only 'Search MusicBrainz to add, button, unavailable', without the reason. Decide whether to add aria-describedby pointing at an sr-only hint, or accept the current behaviour.
result: issue
reported: "describe"
severity: minor

## Summary

total: 13
passed: 10
issues: 3
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-25-4
  truth: "The router never resets scroll, and after a chip-label click far down a 50+ artist list the clicked card stays where it was on screen. When the filtered list is shorter than the window, the browser clamps it as far as the page allows. Toggling `Has muted events` never makes the router reset scroll; if the filtered list is then shorter than the window, the browser clamp moving the page is expected (user decision 2026-10-11)."
  status: resolved
  resolved_by: 25-18-PLAN.md
  resolved_at: 2026-10-12
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
  decision: "User chose (2026-10-11): anchor the clicked card. After a chip-label filter, the clicked chip/card keeps its on-screen position (the browser clamps it when the page is short). Reword the truth to: the router never resets scroll and the clicked card stays where it was on screen. Addendum (2026-10-11): user narrowed test 4 for the Has muted events toggle to promise only that the router never resets scroll. The browser clamp on a short filtered list is a documented, expected limitation, not a failure."
  debug_session: ".planning/debug/chip-click-scroll-reset.md"

- gap_id: G-25-9
  truth: "At 375px and 640px, each Deezer search-result row shows enough of the artist name to read it, and the `Search MusicBrainz to add` button truncates before the name does."
  status: resolved
  resolved_by: 25-16-PLAN.md
  resolved_at: 2026-10-12
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
  truth: "Retention counts from detection, the card's `Detected` date, not the release date. Through Load more under a tag, no card ever shows a Detected date older than the retention window, even when its release date is years old. Reload keeps the tag, and Back restores the previous combination. Run with `EVENT_RETENTION_DAYS=45` so rows exist on both sides of the edge."
  status: resolved
  resolved_by: 25-14-PLAN.md
  resolved_at: 2026-10-12
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

- gap_id: G-25-11
  truth: "When the clicked card leaves under a multi-tag filter, the anchor falls back to the nearest surviving card in either direction, not just the two immediate neighbours, so a run of adjacent leaving cards never collapses the page under the viewport (clamped only when the filtered page is shorter than the window)."
  status: failed
  reason: "User decision (review WR-01): widen. User reported: widen"
  severity: minor
  test: 11
  root_cause: "useScrollAnchor checks only the clicked card's immediate next and previous neighbours. When both also leave (interleaved single-tag cards at ?tags=A,B), no fallback anchor is found and scrollBy never runs (verifier probe: cards A1..E5, clicked C3, B2 and D4 also leave, A1 and E5 survive)."
  artifacts:
    - path: "web/app/lib/useScrollAnchor.ts"
      issue: "fallback looks only at the two immediate neighbours"
    - path: ".planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md"
      issue: "line ~399-400 records 'with neither left, no correction runs'. Update to nearest-surviving-card"
  missing:
    - "Search outward (next first, then previous, alternating or next-then-previous per UI-SPEC order) for the nearest card that survives the new filter and anchor on it"
    - "Unit test for the A1..E5 run case (clicked C3, B2/D4 leave → anchor on D-side survivor E5 or A1 per chosen order)"
    - "Update UI-SPEC/CONTEXT decision text to record the widened rule"
  decision: "User chose (2026-10-12): widen the fallback search to the nearest surviving card."

- gap_id: G-25-12
  truth: "When a keyboard user toggles a chip label and the card leaves the list, focus moves to a chip label on the fallback anchor card (the same surviving card the scroll anchor uses), never to <body>. The UI-SPEC focus table has a 'Toggled, card leaves' row."
  status: failed
  reason: "User decision (review WR-03): anchor. User reported: anchor"
  severity: minor
  test: 12
  root_cause: "The UI-SPEC focus table defines only 'Toggled, card still visible'. When the clicked card unmounts, its focused label button is disconnected and focus falls to <body> (verifier probe: activeElement=BODY). The 25-08 truth ('keeps focus on the label button') can't hold once the label unmounts."
  artifacts:
    - path: "web/app/lib/useScrollAnchor.ts"
      issue: "resolves the fallback anchor card but doesn't move focus"
    - path: "web/app/routes/watchlist.tsx"
      issue: "chip toggle focus handling assumes the clicked card stays mounted"
    - path: ".planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md"
      issue: "focus table is missing a 'Toggled, card leaves' row"
  missing:
    - "After the filter re-render, if the clicked label is disconnected and a fallback anchor card exists (G-25-11 widened search), focus a chip label on that card (prefer one for a selected tag) with preventScroll"
    - "Define the behaviour when no card survives (e.g. focus the list heading or empty-state)"
    - "Unit test: Enter on a leaving card's chip → document.activeElement is the fallback card's chip label"
    - "Add the 'Toggled, card leaves' row to the UI-SPEC focus table"
  decision: "User chose (2026-10-12): move focus to the fallback anchor card's chip label. Depends on G-25-11."

- gap_id: G-25-13
  truth: "A screen reader on a Deezer search-result row announces the disabled 'Search MusicBrainz to add' button together with the reason (DEEZER_ADD_HINT) through its accessible description, with no visual change."
  status: failed
  reason: "User decision (review WR-02): describe. User reported: describe"
  severity: minor
  test: 13
  root_cause: "Under WR-04 the title moved to the wrapper span (a disabled button gets no pointer events), so the button itself has no accessible description (verifier probe: toHaveAccessibleDescription fails). SearchResultsColumns.test.tsx asserts the button has no title attribute."
  artifacts:
    - path: "web/app/components/watchlist/SearchResultsColumns.tsx"
      issue: "lines ~195-201: the disabled button has no aria-describedby. The hint lives only in the wrapper's title"
    - path: "web/app/components/watchlist/SearchResultsColumns.test.tsx"
      issue: "asserts not.toHaveAttribute('title') but not the accessible description"
  missing:
    - "Render an sr-only span with DEEZER_ADD_HINT and a per-row unique id (useId), and set aria-describedby on the disabled button. Keep the wrapper title for hover."
    - "Test: the button toHaveAccessibleDescription(DEEZER_ADD_HINT)"
  decision: "User chose (2026-10-12): add aria-describedby pointing at an sr-only hint."

## Execution Waivers

- waiver: "CI-only race (user decision 2026-10-11). The user accepted running the gap-closure integration suite without -race locally, because cgo is disabled on this Windows box. CI's race job remains the gate. Applies to gap-closure plans 25-11..25-14 only."
- waiver: "CI-only race extended (user decision 2026-10-12). The user accepted running the gap-closure integration suite without -race locally for gap-closure plans 25-19..25-21. CI's race job remains the gate."
