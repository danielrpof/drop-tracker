---
phase: 25-find-filter-watchlist-and-history
verified: 2026-10-12T20:32:03Z
status: human_needed
score: 18/21 must-haves verified
covered_files:
  - .planning/REQUIREMENTS.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-01-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-01-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-02-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-02-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-03-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-03-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-04-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-04-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-05-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-05-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-06-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-06-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-07-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-07-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-08-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-08-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-09-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-09-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-10-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-10-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-11-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-11-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-12-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-12-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-13-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-13-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-14-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-14-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-15-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-15-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-16-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-16-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-17-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-17-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-18-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-18-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-CONTEXT.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-REVIEW.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-UAT.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md
  - internal/events/service.go
  - internal/httpserver/events.go
  - internal/httpserver/events_test.go
  - internal/httpserver/watchlist_test.go
  - internal/watchlist/service.go
  - internal/watchlist/service_test.go
  - queries/events.sql
  - queries/watchlist.sql
  - web/app/components/history/EventCard.test.tsx
  - web/app/components/history/EventCard.tsx
  - web/app/components/history/HistoryFilters.test.tsx
  - web/app/components/history/HistoryFilters.tsx
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.tsx
  - web/app/components/watchlist/TagChips.test.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/components/watchlist/WatchlistTagFilter.tsx
  - web/app/components/watchlist/WatchlistToolbar.tsx
  - web/app/lib/format.test.ts
  - web/app/lib/format.ts
  - web/app/lib/historyParams.ts
  - web/app/lib/useScrollAnchor.test.tsx
  - web/app/lib/useScrollAnchor.ts
  - web/app/lib/useUrlParams.ts
  - web/app/lib/watchlistView.test.ts
  - web/app/lib/watchlistView.ts
  - web/app/routes/history.test.tsx
  - web/app/routes/history.tsx
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
covered_digest: "v1:sha256:936afd60e0b433d234625edec48053e8537d5d60a8d7a9d5cd84924ec969eb8e"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 9/11
  gaps_closed:
    - "G-25-4 sticky path (prior WR-01): a chip-label click on a sticky card keeps the same label element connected and focused, and scrollBy runs once with the drift. The prior verifier's red probe is now the committed route test watchlist.test.tsx:2392, which passes."
    - "Prior WR-03 regression: per the 2026-10-11 user decision, a Manage tags merge or delete of a selected tag now keeps sticky cards (route tests :1709 and :1740 pass), and the post-paint filterKey effect is gone"
    - "Prior decision items 10-12 (WR-02, WR-03, WR-04) were decided by the user on 2026-10-11 and implemented in 25-15, 25-16, and 25-17, as recorded in UI-SPEC and CONTEXT G4"
  gaps_remaining: []
  regressions:
    - "Deezer hint pill: moving `title` from the disabled <button> to a wrapper <span> (25-16, per the user decision) removed the button's accessible description. Before, the title on the button was its description; now the probe finds none. No must-have names the description, so this is a decision item, not a gap."
human_verification:
  - test: "UAT test 4 re-test (25-17 and 25-18 human-check). Use a real browser with 50+ artists, scrolled far down the Watchlist. (1) Filter by a tag, remove it from a card that also carries a second tag, then click that card's second chip label. (2) With two tags selected, un-press a chip on the last card that carries only that tag. (3) Do the same on a middle card that has staying cards below it. (4) Toggle `Has muted events`."
    expected: "(1) The clicked card stays where it was on screen and its label keeps the focus ring. (2) and (3) The next surviving neighbour (or, failing that, the previous one) stays where it was on screen, clamped only when the filtered page is shorter than the window. (4) The router never resets scroll. Also note any fight between the hook's scrollBy and native `overflow-anchor`."
    why_human: "jsdom has no layout. All anchor tests stub getBoundingClientRect and scrollBy (review IN-03)."
  - test: "25-18 human-check (3) and (4). Filter by a tag, remove it from a card, then open Manage tags and merge that tag into another, or delete it while another tag stays selected. Separately, make a card sticky under a toggled filter, then press Back."
    expected: "The edited card stays visible after the merge or delete. Back never shows the stale card, not even for one frame."
    why_human: "Same-frame paint behaviour in a real browser (the commit-probe route test covers it in jsdom)."
  - test: "UAT test 7 re-test. On History with `EVENT_RETENTION_DAYS=45`, pick a tag, press Load more until it disappears, reload, then press Back."
    expected: "No card shows a `Detected` date older than the retention window, even when its release date is years old. Reload keeps the tag, and Back restores the previous combination."
    why_human: "Multi-page UX on real data plus browser Back. UAT test 7 is still `result: issue` pending this re-test."
  - test: "UAT test 9 re-test plus 25-16 human-check. Search `Bad Bunny` and view the Deezer column at 375px, 640px, and 1024px, hovering the truncated `Search Mus…` pill."
    expected: "The artist name is readable and the pill truncates before the name does. Hovering the pill shows the native tooltip with the full hint. At 1024px the full label shows."
    why_human: "Real widths and native tooltips (insufficient_spec for jsdom)."
  - test: "Decision (review WR-01, run of adjacent leavers). At `?tags=A,B`, un-press a chip on a card whose next and previous cards also carry only that tag."
    expected: "Today, no correction runs and the page can collapse under the viewport (verifier probe: cards A1..E5, clicked C3, B2 and D4 leave too, A1 and E5 survive, scrollBy never called). This matches the recorded user decision ('with neither left, no correction runs'). Decide whether to accept it as a documented limitation or widen the search to the nearest surviving card, as review WR-01 proposes."
    why_human: "Product decision. The locked decision covers only the two immediate neighbours. Interleaved single-tag cards make this common on a 50+ list."
  - test: "Decision (review WR-03, focus when the clicked card leaves). With two tags selected, press Enter on a card's chip label so that the card leaves the list."
    expected: "Today focus falls to <body> (verifier probe: activeElement=BODY, label disconnected). The UI-SPEC focus table defines only 'Toggled, card still visible'. The 25-08 truth ('keeps focus on the label button') is unqualified but cannot hold once the label unmounts. Decide on a focus target (the surviving fallback card's chip label, or the list heading) and add a 'Toggled, card leaves' row, or accept the current behaviour."
    why_human: "Undefined contract. Keyboard users get sent back to the top of the tab order."
  - test: "Decision (review WR-02, Deezer hint accessible description). Use a screen reader on a Deezer search-result row."
    expected: "Today the disabled button announces only 'Search MusicBrainz to add, button, unavailable' without the reason (verifier probe: toHaveAccessibleDescription fails), and the test asserts `not.toHaveAttribute(\"title\")`. Decide whether to add `aria-describedby` to an sr-only hint, as review WR-02 proposes, or accept the change."
    why_human: "A side effect of the WR-04 decision that the user didn't weigh. Touch users also still can't reach the hint, which 25-16 accepted."
---

# Phase 25: Find & Filter (Watchlist and History) Verification Report

**Phase Goal:** With a 50+ artist watchlist, the user can find any artist or group of artists in seconds, by name, tag, or preference state and in the order they choose, and can narrow the History feed to the same tags.
**Verified:** 2026-10-12T20:32:03Z
**Status:** human_needed
**Re-verification:** Yes, after gap-closure plans 25-15 to 25-18 (G-25-4 sticky path; review WR-01 to WR-04 decisions)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: typing narrows the list as you type; "N of M artists" always shows | ✓ VERIFIED | Regression check. Full web suite 604/604 (orchestrator gate). `watchlist.test.tsx` passes in my 191-test run. |
| 2 | SC2: sort by name, date added, or latest release; stable ties; no-release artists last both ways | ✓ VERIFIED | `TestService_List_LatestRelease`, `TestService_ProjectionParity`, and `TestWatchlist_List_LatestRelease` PASS against local Postgres (this run). `watchlistView.test.ts` passes. |
| 3 | SC3: tag, muted, and custom filters combine with search and sort; zero matches gives one Clear filters | ✓ VERIFIED | Regression check. Route and view suites pass, and the filter code paths are unchanged apart from the sticky handling. |
| 4 | SC4: History tag filter alongside existing filters, includes removed artists, never past retention on first page or Load more | ✓ VERIFIED | `TestListEvents_TagFilter`, `TestRetention_TagFilterComposesWithCutoff`, `TestRetention_TagFilterKeepsOldReleasesDetectedInWindow`, `TestListEvents_TagFilterComposesWithOtherFilters`, and `TestHasOlderEvents_*` PASS (this run). No Go, SQL, or events change since 8ccde67. |
| 5 | SC5: one request on open, none for search, sort, or filter | ✓ VERIFIED | Regression check. SC5 route test passes. |
| 6 | G-25-4 sticky path (25-15 T1, prior gap): a chip click on a sticky card keeps the same label connected and focused, and scrollBy runs once with the drift | ✓ VERIFIED | `watchlist.test.tsx:2392` replays the prior red probe exactly (`/?tags=3`, detach reggaeton, click `Filter by trap`). It asserts `?tags=3%2C7`, `isConnected`, the same node, `toHaveFocus`, and `scrollBy(0,-280)` once. It passes. The sync `clearSticky()` was removed from `handleTagsChange`/`handleFlagChange`/`handleSortChange` (`watchlist.tsx:475-507`). |
| 7 | Sticky set keyed to the filterKey and reconciled during render; dropped in the same render as any G4 or Back/Forward filter change | ✓ VERIFIED | `watchlist.tsx:89-101` calls `reconcileSticky(sticky, {key, loc, replace})` during render. The commit-probe test `:2068` asserts the first layout commit after Back already excludes the stale card. Passes. |
| 8 | Turning a filter off again never revives a previously sticky card | ✓ VERIFIED | `:2113` passes. The design note in 25-15 explains why it uses a render-time reset rather than a derived `key === filterKey`. |
| 9 | WR-03 decision: the route's own tag rewrites (merge/delete, first-load validation, canonicalization) carry stickies | ✓ VERIFIED | Four `writeKeepingSticky(` call sites (`writeTagIds`, canonical effect, `replaceSelectedTag`). Route tests `:1709` (merge) and `:1740` (delete) pass, and the follow-up toggle in `:1709` drops the card. |
| 10 | A carry lives only until its first landing and applies only to a replace landing on its target | ✓ VERIFIED | `watchlistView.ts:197-220` is pure. Unit tests F-I in `watchlistView.test.ts` pass. The production replace flag comes from `useNavigationType() === NavigationType.Replace`, and every G4 write is a push (`write(...)` with no `replace`). |
| 11 | Typing clears stickies in the same render; Clear filters clears explicitly only when no URL axis is active | ✓ VERIFIED | `handleQueryChange` batches `setQuery` with `clearSticky()`, and test `:1198` passes. With URL axes active, Clear filters is a push that changes filterKey, the same drop mechanism tests `:1469`/`:2113` exercise. The q-only branch has no test (review IN-02, advisory). Its effect is effectively unobservable, because no filter remains to hide a card. |
| 12 | Router contract unchanged; every write passes preventScrollReset | ✓ VERIFIED | `useUrlParams.ts` was last changed in 115e13f (25-04). `urlWriteOptions` always returns `preventScrollReset: true`. |
| 13 | WR-04 decision (25-16): the wrapper span carries the title and the 40% cap; the button gets `w-full`, keeps the truncating span, and keeps the exact accessible name | ✓ VERIFIED | `SearchResultsColumns.tsx:196-203`. The test `caps the Deezer hint wrapper, titles it...` passes, and `getByRole(name: "Search MusicBrainz to add")` resolves. `ui/button.tsx` is unchanged. See the decision item for the lost description. |
| 14 | WR-02 decision (25-17): candidates are chip, next card, previous card; the first connected one wins; a chip's own drift wins; no candidate means no scroll call; the API shape is unchanged | ✓ VERIFIED | `useScrollAnchor.ts:5-32`. The hook fallback cases and the `anchorCandidates` tests pass. Route tests `:2330` (previous neighbour, `scrollBy(0,-200)`) and `:2367` (next neighbour, `scrollBy(0,-400)`) pass. Only `window.scrollBy` is used. |
| 15 | 25-17 T2: when the clicked card leaves, a surviving neighbour holds its position "so the page no longer collapses" | ? UNCERTAIN (decision) | Holds when an immediate neighbour survives. A verifier probe at `?tags=3,7` (A1[3], B2..D4[7], E5[3]; un-press C3's trap) gave `shown=[A1,E5]`, `scrollBy` calls `[]`. The collapse returns for a run of adjacent leavers. This matches the recorded decision text (UI-SPEC D-12: "with neither left, no correction runs"), so it is escalated as a decision rather than failed. |
| 16 | G-25-7: retention counts from detection; cards show `Detected {date}` | ✓ VERIFIED | Regression check. `TestRetention_TagFilterKeepsOldReleasesDetectedInWindow` PASS. EventCard and format suites pass in the full suite. Bundle `Detected` count is 3 (unchanged). |
| 17 | G-25-9: at 375 and 640px the Deezer row shows a readable name and the pill truncates first | ⚠️ insufficient_spec → human | The class contract is verified (truth 13). Readability is a layout truth, routed to UAT test 9. |
| 18 | 25-18: embedded SPA rebuilt with all four fixes | ✓ VERIFIED | Marker counts vs 8ccde67: `carryTo` 0→5, `previousElementSibling` 0→1, `w-full` 39→40. No `web/app` commit after 24d8aaa. `go build ./cmd/server` OK (orchestrator). |
| 19 | 25-18: UI-SPEC (3 lines) and CONTEXT G4 (1 line) record the decisions; UAT and VERIFICATION left untouched by the executor | ✓ VERIFIED | `git diff -U0 8ccde67` shows exactly the three UI-SPEC bullets and the one amended CONTEXT G4 bullet. `25-UAT.md` and `25-VERIFICATION.md` have no diff since 8ccde67. |
| 20 | 25-18: Definition of Done gates and empty blast radius | ✓ VERIFIED | The blast-radius diff vs 8ccde67 (`*.go`, queries, internal/db, internal/events, go.mod/sum, package files, ui/, useUrlParams, root.tsx, TagChips.tsx) is empty. Typecheck and prettier are clean (this run). Go 25/25 packages and web 604/604 pass (orchestrator). Local `-race` is skipped under the accepted CI-only exception. |
| 21 | D-08 / 25-08 T: the chip label keeps focus on itself | ? UNCERTAIN (decision) | Holds whenever the card stays (truth 6 and the 25-08 tests). When the clicked card leaves, the probe shows the label disconnected and `activeElement=BODY`. The UI-SPEC focus table scopes the rule to "card still visible" and defines no target for "card leaves", so this is escalated as a decision. The behaviour dates from 25-08 and was not introduced by 25-15 to 25-18. |

**Score:** 18/21 truths verified (0 present but behavior-unverified; 2 uncertain, decision; 1 insufficient_spec, human).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/app/routes/watchlist.tsx` | render-time `reconcileSticky`, `writeKeepingSticky` | ✓ VERIFIED | Contains `reconcileSticky(` (1) and `writeKeepingSticky(` (4 call sites), and imports `NavigationType`. No `prevFilterKey` effect remains. |
| `web/app/lib/watchlistView.ts` | `StickyState`, `reconcileSticky`, `carrySticky`, `EMPTY_IDS` | ✓ VERIFIED | All four exported, pure (no router import). Used by the route. |
| `web/app/lib/watchlistView.test.ts` | unit tests F-I | ✓ VERIFIED | Pass |
| `web/app/routes/watchlist.test.tsx` | sticky chip, Back commit probe, toggle-off, merge/delete, fallback neighbours | ✓ VERIFIED | All named tests are present and pass |
| `web/app/lib/useScrollAnchor.ts` | `anchorCandidates` plus the first-connected correction | ✓ VERIFIED | 35 lines. Imported by `watchlist.tsx` (`useScrollAnchor(filterKey)`). |
| `web/app/lib/useScrollAnchor.test.tsx` | fallback order, all-gone, chip wins, shape | ✓ VERIFIED | Pass |
| `web/app/components/watchlist/SearchResultsColumns.tsx` | `DEEZER_ADD_HINT`, titled capped wrapper | ✓ VERIFIED | `title={DEEZER_ADD_HINT}` count 1 |
| `internal/webassets/build/client` | rebuilt bundle | ✓ VERIFIED | Marker counts above |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| router location commit | sticky reconcile | `reconcileSticky(...)` → `setSticky` during render → `selectVisible(..., stickyIds)` | ✓ WIRED | `watchlist.tsx:95-101, 471` |
| `replaceSelectedTag`, `writeTagIds`, canonical effect | sticky carry | `writeKeepingSticky` → `carrySticky(prev, from, to)` inside the synchronous `write` mutator | ✓ WIRED | `useUrlParams.write` calls `mutate(next)` synchronously before `setParams` |
| `useNavigationType()` | replace flag | `navigationType === NavigationType.Replace` | ✓ WIRED | `watchlist.tsx:98` |
| `toggleFilterTag` | `useScrollAnchor` | `captureScrollAnchor(anchor)` → `anchorCandidates` → layout effect on `filterKey` → `scrollBy` | ✓ WIRED | The prior PARTIAL (sticky remount) is closed by truth 6 |
| Wrapper span title | native tooltip | wrapper has no `pointer-events-none` | ✓ WIRED (hover → human) | |
| `web/` | `internal/webassets/build/client` | make web recipe | ✓ WIRED | |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| Watchlist list | `visible` | `listWatchlist()` → `entries` → `selectVisible(entries, view, stickyIds)` | Yes | ✓ FLOWING |
| Sticky set | `stickyIds` | `markSticky` from in-card edits and adds → `reconcileSticky` | Yes | ✓ FLOWING |
| Scroll anchor | candidate tops | live `getBoundingClientRect` | Yes (stubbed in jsdom) | ✓ FLOWING |
| EventCard Detected | `event.created_at` | GET /events → `ListEvents` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Gap-closure web suites | `vitest run useScrollAnchor, watchlistView, watchlist route, SearchResultsColumns, TagChips, useUrlParams` | 6 files, 191 passed | ✓ PASS |
| History retention and tag filter, latest-release sort (Go) | `go test ./internal/httpserver/ ./internal/watchlist/ -run 'TestRetention_\|TestListEvents_TagFilter\|TestHasOlderEvents_\|TestService_List_LatestRelease\|TestService_ProjectionParity\|TestWatchlist_List_LatestRelease'` | 9 PASS | ✓ PASS |
| Typecheck and prettier | `pnpm run typecheck`, `prettier --check` (8 changed files) | clean | ✓ PASS |
| Full suites (orchestrator, not repeated) | `go test ./... -count=1`; `pnpm test` | 25 packages ok; 604/604 | ✓ PASS |
| Probe A: focus when clicked card leaves (review WR-03) | temporary route test, deleted after the run | `connected:false, active:"BODY"` | ⚠️ reproduces (decision) |
| Probe B: run of adjacent leavers (review WR-01) | temporary route test, deleted after the run | `shown:[A1,E5], scrollByCalls:[]` | ⚠️ reproduces (decision) |
| Probe C: Deezer button accessible description (review WR-02) | temporary component test, deleted after the run | `absent` | ⚠️ reproduces (decision) |

The probe file was deleted after the run, and `git status --porcelain web` was empty afterwards.

### Probe Execution

Step 7c: SKIPPED. No `scripts/*/tests/probe-*.sh` is declared or conventional for this phase.

### Prohibitions (gap-closure plans)

| Plan | Prohibition | Tier | Disposition |
|------|-------------|------|-------------|
| 25-15 | No change to `useUrlParams.ts`; never drop preventScrollReset | test | ✓ enforced: blast diff empty, `useUrlParams.test.tsx` passes |
| 25-15 | No carry across a G4 or Back/Forward filter change; a carry never outlives its first landing | test | ✓ enforced: unit tests F-I, route tests `:2068`, `:2113`, and the toggle tail of `:1709` |
| 25-15, 25-18 | No npm package, module, migration, or Go change | test | ✓ enforced: blast diff vs 8ccde67 empty |
| 25-16 | No edit to `ui/button.tsx`; text, accessible name, and hint copy unchanged | test | ✓ enforced: diff empty, the name query resolves, `DEEZER_ADD_HINT` text matches |
| 25-17 | Only `window.scrollBy`, no height padding; no change to `useUrlParams`, `watchlist.tsx`, or `TagChips` in that plan | test | ✓ enforced: hook greps clean. `watchlist.tsx`'s last change is from 25-15. |
| 25-18 | No edit to 25-UAT.md or 25-VERIFICATION.md | test | ✓ enforced: no diff since 8ccde67 (this report is the verifier's own write) |
| 25-04 (prior) | MUST NOT remove a card from view as a consequence of the user's own in-card edit (G4) | test | ✓ now enforced: the prior WR-03 flag is resolved by the user decision and route tests `:1709` and `:1740` |
| 25-09 (prior) | No shared filter state between routes | judgment | ✓ resolved by human (UAT test 8 passed) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WLVW-01 | 25-04, 25-10, 25-13, 25-14, 25-16, 25-18 | Search the watchlist by name | ✓ SATISFIED | Truth 1. Deezer-row layout pending UAT 9. |
| WLVW-02 | 25-05, 25-10 | Sort by name or date added, stable | ✓ SATISFIED | Truth 2 |
| WLVW-03 | 25-01, 25-03, 25-05, 25-10 | Sort by latest release, none last | ✓ SATISFIED | Truth 2 |
| WLVW-04 | 25-07, 25-08, 25-10, 25-11, 25-14, 25-15, 25-17, 25-18 | Filter by one or more tags | ✓ SATISFIED | Truths 3, 6, 14. The prior sticky-path gap is closed. Two decision items (truths 15 and 21) are polish on the chip-click path, not on filtering. |
| WLVW-05 | 25-08, 25-10 | Muted / custom release-type filters | ✓ SATISFIED | Truth 3 |
| WLVW-06 | 25-04, 25-07, 25-08, 25-10, 25-15, 25-18 | Combine; N of M; empty state | ✓ SATISFIED | Truths 1, 3, 7-11 |
| HIST-02 | 25-02, 25-06, 25-09, 25-10, 25-12, 25-14 | History by tag, retention-respecting | ✓ SATISFIED | Truths 4, 16. Retention-edge re-test pending (UAT 7). |

All 7 IDs are claimed by at least one plan, and REQUIREMENTS.md maps no other IDs to Phase 25, so none are orphaned. Bookkeeping: REQUIREMENTS.md still lists WLVW-02, -03, -05, and HIST-02 as "Gaps Found" with unchecked boxes. That is stale relative to the code and is the orchestrator's to update.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| gap-closure source and test files (8) | — | TBD/FIXME/XXX/TODO/HACK | none found | — |
| `web/app/lib/useScrollAnchor.ts` | 5-14 | Only immediate neighbours are candidates; header says "nearest surviving neighbour" (review WR-01) | ⚠️ Warning | Collapse returns for runs of adjacent leavers (Probe B). Decision item. |
| `web/app/components/watchlist/SearchResultsColumns.tsx` | 196-203 | `title` on a generic span; the button lost its accessible description (review WR-02) | ⚠️ Warning | Screen-reader users lose the reason (Probe C). Decision item. |
| `web/app/routes/watchlist.tsx` | 491-497 | No focus target when the clicked card unmounts (review WR-03) | ⚠️ Warning | Focus goes to `<body>` (Probe A). Decision item. |
| `web/app/routes/watchlist.tsx` | 517 | Hand-derived Clear-filters predicate, q-only branch untested (review IN-02) | ℹ️ Info | Fragile if keys change |
| `web/app/routes/watchlist.test.tsx` | 2336-2351 | Dead read-order stub (review IN-01) | ℹ️ Info | Misleading test comment |

### Assessment of the 25-REVIEW.md warnings

None of the three warnings breaks a must-have as the locked user decisions scope it, and none contradicts a locked decision. Each one reproduces deterministically, though, and each leaves a gap in a contract the user hasn't settled, so all three are escalated as decisions, not gaps.

- **Review WR-01 (two-neighbour limit).** It does not break the WR-02 decision. UI-SPEC D-12 records "next sibling card, then its previous one ... with neither left, no correction runs" as the user's own wording, and 25-17 T3 pins the no-candidate no-op. It does make 25-17 T2's "page no longer collapses" overclaim (truth 15, UNCERTAIN), and it brings back the original G-25-4 symptom on a path that is likely with interleaved single-tag cards on a 50+ list. A decision is needed: accept and fix the header comment, or widen the search per the review's sketch.
- **Review WR-02 (accessible description lost).** It does not break a must-have. 25-16 only promises the accessible name, which is unchanged, and the wrapper-title layout is the user's WR-04 decision. It is a real accessibility regression relative to Phase 6, and the user likely didn't weigh it when choosing the wrapper. The `aria-describedby` fix is small and is compatible with the decision.
- **Review WR-03 (focus to body).** It does not break a locked decision. The UI-SPEC focus table only covers "card still visible". The 25-08 truth reads more broadly, but no label can keep focus once it unmounts, and this behaviour predates the gap closure. It needs a contract row (truth 21, UNCERTAIN).

### Human Verification Required

1. **UAT 4 re-test: scroll anchor on a real long list.** Test: the sticky-card chip click, last card leaves, middle card leaves, then toggle `Has muted events`. Expected: the clicked card (or its surviving neighbour) holds its on-screen position, the label keeps focus on the sticky path, and the router never resets scroll. Why human: no layout in jsdom.
2. **Manage tags merge/delete after a card went sticky, and Back after a sticky card.** Expected: the card survives the merge or delete, and Back shows no stale frame. Why human: real paint timing.
3. **UAT 7 re-test: History retention edge plus reload and Back.** Expected: no card's Detected date falls outside the window, and the tag persists. Why human: real data and navigation.
4. **UAT 9 re-test: Deezer row at 375, 640, and 1024px, plus hover tooltip.** Expected: the name is readable, the pill truncates first, and hover shows the full hint. Why human: real widths and tooltips.
5. **Decision, review WR-01.** Accept the two-neighbour limit, or widen to the nearest surviving card?
6. **Decision, review WR-03.** Where should focus go when the clicked card leaves the list?
7. **Decision, review WR-02.** Restore the hint as the button's accessible description via `aria-describedby`?

### Gaps Summary

The previous report's one gap is closed. A chip click on a sticky card no longer unmounts it. The sticky set is now reconciled during render against the filter key, the router location key, and the navigation type. The verifier's earlier red probe is now a committed, passing route test that checks node identity, focus, and the scroll correction. The prior WR-03 regression is resolved per the user's decision: the route's own tag rewrites carry sticky cards, with route and unit tests. The WR-02 and WR-04 decisions are implemented exactly as recorded. The bundle carries all fixes, the blast radius is empty, and every original success criterion holds under regression tests (Go History/retention and latest-release tests re-run green, web 604/604).

No blocker remains. The status is human_needed for two reasons. First, the real-browser re-tests: UAT 4, 7, and 9 are still `issue` in 25-UAT.md until re-run. Second, the three new review warnings each reproduce deterministically in contracts the user has not yet decided: a run of adjacent leavers gets no anchor, focus goes to `<body>` when the clicked card leaves, and the Deezer hint's accessible description is gone. If the user wants any of them fixed, they fit one small follow-up plan touching `useScrollAnchor.ts`, `watchlist.tsx`, and `SearchResultsColumns.tsx`.

---

_Verified: 2026-10-12T20:32:03Z_
_Verifier: Claude (gsd-verifier)_
