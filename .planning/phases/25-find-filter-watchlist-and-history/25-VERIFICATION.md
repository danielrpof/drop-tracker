---
phase: 25-find-filter-watchlist-and-history
verified: 2026-10-13T02:17:13Z
status: passed
score: 23/24 must-haves verified
covered_files:

  - .planning/phases/25-find-filter-watchlist-and-history/25-01-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-02-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-03-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-04-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-05-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-06-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-07-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-08-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-09-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-10-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-11-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-12-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-13-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-14-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-15-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-16-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-17-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-18-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-19-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-20-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-21-PLAN.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-01-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-02-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-03-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-04-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-05-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-06-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-07-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-08-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-09-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-10-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-11-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-12-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-13-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-14-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-15-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-16-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-17-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-18-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-19-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-20-SUMMARY.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-21-SUMMARY.md
  - .planning/REQUIREMENTS.md
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

covered_digest: "v1:sha256:e7a9a4f63dd3f9851369da1e716c2defbf07ef6e98985b73de70a439c6f5dcca"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 18/21
  gaps_closed:
    - "G-25-11 (review WR-01, user decision 'widen'): anchorCandidates now returns the chip plus every sibling card nearest-first, next before previous at each distance. The prior verifier's A1..E5 probe is now committed as hook and route tests and passes. Prior truth 15 (UNCERTAIN) is closed."
    - "G-25-12 (review WR-03, user decision 'anchor'): a keyboard toggle that removes its card moves focus to the fallback card's pressed chip label, or to the `Your artists` heading when nothing survives. A mouse click moves no focus. Prior truth 21 (UNCERTAIN) is closed."
    - "G-25-13 (review WR-02, user decision 'describe'): the disabled Deezer button has DEEZER_ADD_HINT as its accessible description through aria-describedby on a per-row useId sr-only span. The prior regression item is closed."
    - "G-25-9 layout (prior truth 17, insufficient_spec): UAT test 9 passed on 2026-10-12."
  gaps_remaining: []
  regressions: []
human_verification:

  - test: "G-25-11 re-test, plus review WR-02 (distant survivor). Run the rebuilt binary with 50+ artists, scroll far down, and select two tags. (a) Un-press a chip on a card whose immediate neighbours also leave, a run of two or three on each side. (b) Un-press a tag that removes a long run, five or more cards on each side, so the nearest survivor was below the fold before the click."
    expected: "(a) The nearest surviving card keeps its on-screen position and the page does not collapse. (b) Today the code pins the survivor at its old viewport offset (`useScrollAnchor.ts:55`), even when that offset was off-screen. The viewport can then show cards from far above the click point, and neither nearest survivor is visible. Record what you see, then decide: accept D-12 literally ('where it was on screen'), or aim the fallback at the click position or clamp it into the viewport, as review WR-02 proposes. Also note any fight with native `overflow-anchor`."
    why_human: "jsdom has no layout. Every anchor test stubs getBoundingClientRect and scrollBy. The distant-survivor contract is undecided. The committed route test even uses a survivor top of 900, below jsdom's 768px fold, so it pins this behaviour without asserting it. UAT test 4 was passed under a no-browser waiver, before 25-19 widened the search."
  - test: "G-25-12 re-test. On the rebuilt binary with two tags selected, Tab to a chip label and press Enter (then Space) so that its card leaves. Try it on the last card, a middle card, a run of leaving cards, and a toggle that leaves no card. Then repeat one case with a mouse click."
    expected: "Focus lands on a pressed chip label of the surviving card the scroll anchor used, with a visible focus ring, and never on <body>. With no survivor, focus goes to the `Your artists` heading. That heading has `outline-none` (review IN-02), so check whether you can tell where focus went. A mouse click moves no focus. If the fallback card ends up off-screen (see the previous item), the focused chip is off-screen too, because it is focused with `preventScroll`. Record whether that happens on a realistic run."
    why_human: "Real focus-ring visibility, on-screen position after preventScroll, and real browser `detail` values for Enter and Space. The route tests prove the focus target in jsdom only."
  - test: "G-25-13 re-test, plus review WR-01 (double announcement). With NVDA or VoiceOver, search `Bad Bunny` and read a Deezer result row in browse mode (virtual cursor), then with object navigation."
    expected: "The disabled `Search MusicBrainz to add` button is announced with its reason (the DEEZER_ADD_HINT sentence), and nothing changes visually. Today the hint lives in a visible-to-AT `sr-only` sibling, so browse mode will likely read the 130-character sentence a second time as plain text, and some readers may expose the wrapper `title` as a third copy. Decide whether to switch the span to `hidden`. That keeps the accessible description, because aria-describedby follows a direct reference into hidden content, but it departs from the literal 'sr-only hint' wording of the decision."
    why_human: "jsdom has no screen reader. 25-21 coverage D3 is `human_judgment: true`. Whether the hint is read twice depends on the AT."
---

# Phase 25: Find & Filter (Watchlist and History) Verification Report

**Phase Goal:** With a 50+ artist watchlist, the user can find any artist or group of artists in seconds, by name, tag, or preference state and in the order they choose, and can narrow the History feed to the same tags.
**Verified:** 2026-10-12T22:41:48Z
**Status:** human_needed
**Re-verification:** Yes. This run follows gap-closure plans 25-19, 25-20, and 25-21, which closed UAT gaps G-25-11, G-25-12, and G-25-13.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: typing narrows the list as you type, and "N of M artists" always shows | ✓ VERIFIED | Regression check. Full web suite exit 0 in this run (coverage 95.34/90.84/96.12/97.16). UAT 1 pass. |
| 2 | SC2: sort by name, date added, or latest release, with stable ties and no-release artists last in both directions | ✓ VERIFIED | Re-run this run against local Postgres: `TestService_List_LatestRelease`, `TestService_ProjectionParity`, and `TestWatchlist_List_LatestRelease` PASS. UAT 2 pass. |
| 3 | SC3: tag, muted, and custom filters combine with search and sort, and zero matches gives one Clear filters | ✓ VERIFIED | Regression check. Route and view suites pass (209/209 in the scoped run). UAT 3 pass. |
| 4 | SC4: the History tag filter works alongside the existing filters, includes removed artists, and never goes past retention on the first page or on Load more | ✓ VERIFIED | Re-run this run: `TestListEvents_TagFilter`, `TestRetention_TagFilterComposesWithCutoff`, `TestRetention_TagFilterKeepsOldReleasesDetectedInWindow`, `TestListEvents_TagFilterComposesWithOtherFilters`, and `TestHasOlderEvents_*` PASS. UAT 7 pass. No Go, SQL, or events change since `9d3413f`. |
| 5 | SC5: one request on open, and none for search, sort, or filter | ✓ VERIFIED | Regression check. The SC5 route test passes. UAT 1 pass. |
| 6 | G-25-4 sticky path: a chip click on a sticky card keeps the same label connected and focused | ✓ VERIFIED | Regression check. The route test `keeps a sticky card's chip anchored and focused...` passes. `watchlist.tsx` changed only in 25-20's focus wiring. |
| 7 | The sticky set is reconciled during render and dropped in the same render as a G4 or Back/Forward filter change | ✓ VERIFIED | Regression check. The commit-probe route test passes. |
| 8 | Turning a filter off again never revives a sticky card | ✓ VERIFIED | Regression check. Passes. |
| 9 | WR-03 decision: the route's own tag rewrites carry stickies | ✓ VERIFIED | Regression check. Merge and delete route tests pass. UAT 10 pass. |
| 10 | A carry lives only until its first landing | ✓ VERIFIED | Regression check. `watchlistView.test.ts` F-I pass. |
| 11 | Typing clears stickies in the same render | ✓ VERIFIED | Regression check. Passes. |
| 12 | The router contract is unchanged, and every write passes preventScrollReset | ✓ VERIFIED | `git diff --name-only 9d3413f -- web/app/lib/useUrlParams.ts` is empty. `useUrlParams.test.tsx` passes. |
| 13 | WR-04 decision: the wrapper span carries the title and the 40% cap, and the button keeps its accessible name | ✓ VERIFIED | `SearchResultsColumns.tsx:197-211` is unchanged apart from the 25-21 additions. The name query resolves. UAT 9 pass. |
| 14 | Fallback rules: a connected chip wins, no surviving candidate means no scroll call, a zero drift calls nothing, and the capture stays backward compatible | ✓ VERIFIED | `useScrollAnchor.ts:49-57`. The pre-existing hook cases pass unedited. The new parameters `onFocusLost?` and `viaKeyboard?` are optional. |
| 15 | G-25-11: when the clicked card leaves, the anchor falls back to the nearest survivor in either direction, next before previous at each distance, with no cap. A run of adjacent leavers no longer collapses the page. | ✓ VERIFIED | `anchorCandidates` walks outward over `card.parentElement.children` (`useScrollAnchor.ts:5-19`). These tests pass: hook `run of leaving cards (G-25-11)` (2 cases), `anchorCandidates` ordering (`[chip, D4, B2, E5, A1]`), and route `corrects by the nearest survivor's drift when a run of cards leaves (G-25-11)` (`?tags=7`, `[Arcangel, Eladio Carrion]`, `scrollBy(0,-600)` once). 25-19 SUMMARY records the RED run (5 failed before the change). The distant-survivor behaviour is a decision item (review WR-02), not a breach of this truth. |
| 16 | G-25-7: retention counts from detection, and cards show `Detected {date}` | ✓ VERIFIED | `TestRetention_TagFilterKeepsOldReleasesDetectedInWindow` PASS (this run). UAT 7 pass. |
| 17 | G-25-9: at 375 and 640px the Deezer name is readable and the pill truncates first | ✓ VERIFIED (human) | UAT test 9 `result: pass` (2026-10-12). This closes the prior insufficient_spec item. |
| 18 | G-25-12: a keyboard toggle that removes its card moves focus to a chip label on the fallback anchor card, preferring a pressed one, or to the tabIndex -1 `Your artists` heading when no card survives, and never to <body> | ✓ VERIFIED | `watchlist.tsx:195-205` uses constant selectors and `(target ?? headingRef.current)?.focus({ preventScroll: true })`. The heading (`:599-606`) has `tabIndex={-1}` and stays mounted while `entries.length > 0`, including the filtered-to-zero state. Route tests pass: last card → previous card's pressed label, middle card → next card's, the run case lands on the scroll-anchor card, and no survivor → heading. This is a behaviour-dependent truth, and the tests exercise the focus transition. |
| 19 | G-25-12 scope: focus moves only for a keyboard toggle (`detail === 0`) when the label held focus at click time and focus actually fell to the document | ✓ VERIFIED | `TagChips.tsx:200` passes `e.detail === 0`. The hook records `viaKeyboard && el.contains(document.activeElement)` and gates on `!captured.el.isConnected && (active is body or null)` (`:45, :58-65`). These tests pass: hook `onFocusLost` stays quiet for mouse (detail 1), never-focused, card-stays, and focus-moved-away; route `a mouse click that removes the card moves no focus`; TagChips `Enter on the focused label reports a keyboard toggle`. |
| 20 | G-25-12: the focus move uses preventScroll and runs after the scroll correction | ✓ VERIFIED | `scrollBy` (`:56`) runs before `reportFocusLost` (`:64`) in the same layout effect, and the call site uses `preventScroll: true`. Hook test `reports the surviving card after the scroll correction for a keyboard capture` passes. The callback goes through `useEffectEvent`, so the effect stays keyed on `[key]`. |
| 21 | G-25-13: the disabled Deezer button has DEEZER_ADD_HINT as its accessible description through aria-describedby on a per-row useId sr-only span, with the name and the wrapper title and cap unchanged. Ids are unique per row, and the Add and Already watching buttons have no description. | ✓ VERIFIED | `SearchResultsColumns.tsx:161, 206, 211-213`. Test `:200` asserts `toHaveAccessibleDescription(DEEZER_ADD_HINT)`. `describes each Deezer button with its own sr-only hint (G-25-13)` checks distinct ids. Test `:286` asserts `not.toHaveAttribute("aria-describedby")` on the other buttons. All pass. |
| 22 | G-25-13: a real screen reader announces the reason, once and without a visual change | ? UNCERTAIN (human) | The accname mechanism is verified (truth 21), but real AT output can't be checked here (25-21 D3 is `human_judgment: true`). Review WR-01 predicts a duplicate browse-mode read because the span is `sr-only`, not `hidden`. See human item 3. |
| 23 | UI-SPEC records all three decisions | ✓ VERIFIED | `git diff -U0 9d3413f -- 25-UI-SPEC.md` shows 2 removed and 3 added lines. The D-12 bullet now carries `widened to the nearest surviving card by G-25-11, user decision 2026-10-12` (the old `with neither left` text is gone). Line 426 adds the `Card chip label \| Toggled, card leaves` row right after `Toggled, card still visible`. The page-order note carries `G-25-13, user decision 2026-10-12` (count 1). |
| 24 | 25-21: the embedded SPA is rebuilt with all three fixes, the DoD gates pass, and the blast radius is empty | ✓ VERIFIED | Bundle markers vs `9d3413f`: `parentElement` 9→10, `data-chip-label` 0→3, `aria-describedby` 3→4. `watchlist-*.js` contains `toggleFilterTag(n,e.currentTarget,e.detail===0)` and `preventScroll:!0`. The bundle commit `5358e5b` follows the last `web/app` commit `69d6f9e`. The blast-radius diff (`*.go`, queries, internal/db, internal/events, go.mod/sum, package files, ui/, useUrlParams, root.tsx, CONTEXT) is empty. Only the eight gap-plan `web/app` files changed. In this run: typecheck clean, prettier clean, full web suite exit 0 with thresholds met. Go vet, golangci-lint, the coverage gate (90.32%), and sqlc-check come from 25-21-SUMMARY. No Go source changed, so the prior gate state carries. Local `-race` is skipped under the recorded "CI-only race extended" waiver. |

**Score:** 23/24 truths verified (0 present but behavior-unverified; 1 uncertain, routed to human).

### Advisory (New Scope, Unevidenced)

None. Both review warnings are in files this round modified, and both reproduce from the code, so they are reported as Warnings below rather than as advisories. Neither is a Blocker.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/app/lib/useScrollAnchor.ts` | `anchorCandidates` outward walk; `useScrollAnchor(key, onFocusLost?)`; `useEffectEvent` | ✓ VERIFIED | 69 lines, contains `parentElement` and `useEffectEvent`, uses only `window.scrollBy` (no `scrollTo` or `scrollIntoView`). Imported and used by `watchlist.tsx:205`. |
| `web/app/lib/useScrollAnchor.test.tsx` | run cases, ordering cases, onFocusLost cases | ✓ VERIFIED | All pass |
| `web/app/components/watchlist/TagChips.tsx` | `data-chip-label`, `e.detail === 0` | ✓ VERIFIED | `:196, :200` |
| `web/app/components/watchlist/TagChips.test.tsx` | viaKeyboard false for mouse, true for Enter | ✓ VERIFIED | Pass |
| `web/app/routes/watchlist.tsx` | `focusAfterCardLeaves`, `toggleFilterTag(tag, anchor?, viaKeyboard?)`, heading `tabIndex={-1}` | ✓ VERIFIED | `:195-205, :500-506, :599-606` |
| `web/app/routes/watchlist.test.tsx` | G-25-11 run route case; G-25-12 describe (5 cases) | ✓ VERIFIED | All pass |
| `web/app/components/watchlist/SearchResultsColumns.tsx` | exported `DEEZER_ADD_HINT`, `useId`, `aria-describedby={hintId}`, sr-only span | ✓ VERIFIED | `:161, :206, :211-213` |
| `web/app/components/watchlist/SearchResultsColumns.test.tsx` | accessible description, id uniqueness, other buttons undescribed | ✓ VERIFIED | Pass |
| `25-UI-SPEC.md` | D-12 widened bullet, focus row, page-order note | ✓ VERIFIED | See truth 23 |
| `internal/webassets/build/client` | rebuilt bundle | ✓ VERIFIED | See truth 24 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `TagChips` label onClick | `toggleFilterTag` → `captureScrollAnchor` | `actions.toggleFilterTag(tag, e.currentTarget, e.detail === 0)` → `captureScrollAnchor(anchor, viaKeyboard)` | ✓ WIRED | `TagChips.tsx:200`, `watchlist.tsx:506` |
| `captureScrollAnchor` | `anchorCandidates` | capture maps every candidate to `{el, top}` | ✓ WIRED | `useScrollAnchor.ts:41-44` |
| `useScrollAnchor(filterKey, focusAfterCardLeaves)` | layout effect | `useEffectEvent` → `onFocusLost(a?.el ?? null)` after `scrollBy` | ✓ WIRED | `useScrollAnchor.ts:34-36, 49-66` |
| `focusAfterCardLeaves` | chip label button | `querySelector('[data-chip-label][aria-pressed="true"]')`, then `[data-chip-label]`, then `headingRef` | ✓ WIRED | Constant selectors only |
| disabled Deezer `Button` | sr-only hint span | `aria-describedby={hintId}` ↔ `id={hintId}` | ✓ WIRED | `toHaveAccessibleDescription` passes |
| `web/` | `internal/webassets/build/client` | make web recipe | ✓ WIRED | Bundle markers present |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| Watchlist list | `visible` | `listWatchlist()` → `selectVisible(entries, view, stickyIds)` | Yes | ✓ FLOWING |
| Scroll anchor | candidate tops | live `getBoundingClientRect` over the rendered `<ul>` children | Yes (stubbed in jsdom) | ✓ FLOWING |
| Focus target | fallback card | first connected candidate from the same capture | Yes | ✓ FLOWING |
| Deezer description | `DEEZER_ADD_HINT` | static constant (intended) | n/a | ✓ |
| EventCard Detected | `event.created_at` | GET /events → `ListEvents` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Gap-closure web suites | `vitest run useScrollAnchor, watchlist route, TagChips, SearchResultsColumns, watchlistView, useUrlParams --coverage.enabled=false` | 6 files, 209 passed | ✓ PASS |
| Named G-25-11, G-25-12, and G-25-13 tests | same run with `--reporter=verbose` | all 17 named cases ✓ | ✓ PASS |
| Full web suite, run once | `corepack pnpm --dir web test` | exit 0; Statements 95.34%, Branches 90.84%, Functions 96.12%, Lines 97.16% | ✓ PASS |
| History retention and tag filter, latest-release sort (Go) | `TEST_DATABASE_URL=... go test ./internal/httpserver/ ./internal/watchlist/ -run 'TestRetention_\|TestListEvents_TagFilter\|TestHasOlderEvents_\|TestService_List_LatestRelease\|TestService_ProjectionParity\|TestWatchlist_List_LatestRelease'` | 9 PASS | ✓ PASS |
| Typecheck and prettier | `pnpm run typecheck`; `prettier --check` on the 8 changed files | clean | ✓ PASS |
| Bundle markers | grep counts under `internal/webassets/build/client/assets` vs `9d3413f` | 9→10, 0→3, 3→4 | ✓ PASS |

### Probe Execution

Step 7c: SKIPPED. No `scripts/*/tests/probe-*.sh` is declared or conventional for this phase.

### Prohibitions (gap-closure plans 25-19 to 25-21)

| Plan | Prohibition | Tier | Disposition |
|------|-------------|------|-------------|
| 25-19 | No height padding; no scroll API other than `window.scrollBy` | test | ✓ enforced: no `scrollTo(`, `scrollIntoView`, or style height in the hook or route |
| 25-19 | No change to watchlist.tsx, TagChips.tsx, useUrlParams.ts, or package files in that plan | test | ✓ enforced: `a4f8b09` touches only the hook, its test, and the route test |
| 25-20 | Selectors are constant literals | test | ✓ enforced: both selectors are string literals |
| 25-20 | No focus move for mouse or touch, card stays, label not focused, or focus moved away | test | ✓ enforced: four hook cases plus the route mouse case |
| 25-20, 25-21 | No change to useUrlParams, components/ui, package files, Go, or migrations | test | ✓ enforced: blast diff vs `9d3413f` empty |
| 25-21 | No visual change to the Deezer row | test | ✓ enforced: the hint is `sr-only` and the wrapper classes are unchanged (the test asserts the class) |
| 25-21 | No executor edit to UAT, VERIFICATION, or CONTEXT | test | ✓ enforced: no commits to them since `9d3413f`, and UAT has none since `1cb2a7b` |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WLVW-01 | 25-04, 25-10, 25-13, 25-14, 25-16, 25-18, 25-21 | Search the watchlist by name | ✓ SATISFIED | Truths 1, 13, 17, 21. Real-AT check pending (human item 3). |
| WLVW-02 | 25-05, 25-10 | Sort by name or date added, stable | ✓ SATISFIED | Truth 2 |
| WLVW-03 | 25-01, 25-03, 25-05, 25-10 | Sort by latest release, none last | ✓ SATISFIED | Truth 2 |
| WLVW-04 | 25-07, 25-08, 25-10, 25-11, 25-14, 25-15, 25-17 to 25-21 | Filter by one or more tags | ✓ SATISFIED | Truths 3, 6, 14, 15, 18-20 |
| WLVW-05 | 25-08, 25-10 | Muted and custom release-type filters | ✓ SATISFIED | Truth 3 |
| WLVW-06 | 25-04, 25-07, 25-08, 25-10, 25-15, 25-18 | Combine; N of M; empty state | ✓ SATISFIED | Truths 1, 3, 7-11 |
| HIST-02 | 25-02, 25-06, 25-09, 25-10, 25-12, 25-14 | History by tag, retention-respecting | ✓ SATISFIED | Truths 4, 16. UAT 7 pass. |

All 7 IDs are claimed by at least one plan, and REQUIREMENTS.md maps no other ID to Phase 25, so none are orphaned. Bookkeeping: REQUIREMENTS.md still shows WLVW-02, WLVW-03, WLVW-05, and HIST-02 as unchecked and "Gaps Found". That is stale relative to the code and to the UAT passes, and it is the orchestrator's to update. 25-UAT.md also still records G-25-11, G-25-12, and G-25-13 as `status: failed` and tests 11-13 as `issue`. Those are the user's decision answers and are now implemented. /gsd-verify-work owns that file.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| 8 gap-closure source and test files | — | TBD/FIXME/XXX/TODO/HACK | none found | — |
| `web/app/lib/useScrollAnchor.ts` | 53-57 | A fallback survivor is pinned at its capture-time viewport offset, even when that offset was off-screen (review WR-02) | ⚠️ Warning | On long runs the viewport can jump to unrelated cards, and the G-25-12 focus target can sit off-screen. Human item 1 (decision). |
| `web/app/components/watchlist/SearchResultsColumns.tsx` | 211-213 | The description element is `sr-only` (still in the a11y tree), not `hidden` (review WR-01) | ⚠️ Warning | The hint is likely read twice in browse mode. Human item 3 (decision). |
| `web/app/routes/watchlist.tsx` | 197-203 | A chipless first survivor sends focus to the heading even when a nearer card with chips exists (review IN-01) | ℹ️ Info | Rare. It needs a sticky card plus the last tag un-pressed. It still never falls to `<body>`. |
| `web/app/routes/watchlist.tsx` | 603 | The focusable heading keeps `outline-none` (review IN-02) | ℹ️ Info | No visible indicator in the no-survivor path. Folded into human item 2. |
| `web/app/components/watchlist/TagChips.tsx` | 198 | The comment says `detail` is 0 "only" for Enter or Space (review IN-03) | ℹ️ Info | Inaccurate comment. The hook's focus guards make the behaviour safe. |
| `SearchResultsColumns.test.tsx` | 200, 234 | The hint copy is pinned only via the imported constant (review IN-04) | ℹ️ Info | A copy regression would still pass |
| `watchlist.test.tsx`, `watchlist.tsx:531` | — | Carried-over review IN-05 and IN-06 | ℹ️ Info | Unchanged from the prior round |

### Assessment of the 25-REVIEW.md warnings

Neither warning breaks a must-have or reopens a gap. Both sit in contracts the user's decision wording doesn't settle, so both are escalated as decisions inside the real-browser and screen-reader re-tests.

- **Review WR-01 (duplicate announcement).** The G-25-13 must-have is met: the accessible description is computed and asserted, with no visual change. The user's decision literally says "an sr-only hint", and 25-21 implemented exactly that. The double read in browse mode is a predicted AT quality issue, not a missing behaviour. The `hidden` fix keeps the description and the tests (`textContent` still includes hidden nodes). It only deviates from the literal "sr-only" wording, so the user needs to choose it.
- **Review WR-02 (distant fallback off-screen).** The G-25-11 must-have ("nearest surviving card ... never collapses the page under the viewport") and the 25-19 truth (the survivor "keeps its on-screen position") are both met literally. `scrollBy` always runs when any card survives, and the survivor returns to its captured offset. When that offset was itself off-screen, though, the user lands on cards far from the click point. That is the G-25-4 "lost my place" symptom from the other side, on long runs only. G-25-12 inherits it, because focus goes to that card's chip with `preventScroll`, which the plan mandated. The UI-SPEC D-12 wording ("where it was on screen") doesn't say what to do with a survivor that was not on screen. This needs a product decision plus a real-browser look, not a gap closure. If the user picks the review's clamp or aim-at-click fix, it is a small change to `useScrollAnchor.ts:55` plus one hook test.

### Human Verification Required

1. **G-25-11 real-browser re-test, plus the distant-survivor decision (review WR-02).** Test: on 50+ artists, un-press a chip that removes a short run (2-3 cards each side) and then a long run (5+ each side). Expected: the nearest survivor holds its on-screen position and the page doesn't collapse. For the long run, record whether the viewport lands on unrelated cards, then decide between accepting D-12 literally and clamping or aiming at the click point. Why human: jsdom has no layout, and UAT 4 was passed without a browser before the search was widened.
2. **G-25-12 keyboard re-test.** Test: press Enter and Space on a chip label so its card leaves (last, middle, run, no survivor), then repeat with a mouse click. Expected: a visible focus ring on the fallback card's pressed chip label (or the heading), never on `<body>`, and no focus move for the mouse. Check that the focused element is on screen, and whether the heading's missing outline matters. Why human: real focus rings, preventScroll position, and real `detail` values.
3. **G-25-13 screen-reader re-test, plus the double-read decision (review WR-01).** Test: NVDA or VoiceOver on a Deezer row in browse mode. Expected: the reason is announced with the disabled button and nothing changes visually. Record whether it is read twice, then decide whether to switch `sr-only` to `hidden`. Why human: jsdom has no screen reader.

### Gaps Summary

All three gaps from the prior round are closed in code and covered by passing tests.

- **G-25-11.** The scroll-anchor fallback now walks outward to the nearest surviving card. The prior verifier's A1..E5 probe is a committed hook test and route test.
- **G-25-12.** A keyboard toggle that removes its card moves focus to the fallback card's pressed chip label, or to the `Your artists` heading when nothing survives. It never falls to `<body>`, and mouse clicks are left alone. Route tests cover last card, middle card, run, no survivor, and mouse, and hook tests cover every guard.
- **G-25-13.** The disabled Deezer button exposes DEEZER_ADD_HINT as its accessible description through a per-row `useId` id.

The UI-SPEC records all three decisions. The embedded bundle carries all three fixes, and the blast radius is empty. Every original success criterion still holds: in this run the Go History, retention, and latest-release tests re-ran green against Postgres, and the full web suite exited 0 above its coverage thresholds.

No blocker remains. The status is human_needed because:

- G-25-11, G-25-12, and G-25-13 need the real-browser and screen-reader re-tests that jsdom cannot perform.
- Two review warnings ride along as decisions inside those re-tests:
  - Review WR-02: a distant survivor is pinned off-screen, and focus can follow it there.
  - Review WR-01: the sr-only hint is likely read twice.

If the user wants either one changed, both fit in one small follow-up touching `useScrollAnchor.ts` and `SearchResultsColumns.tsx`.

---

_Verified: 2026-10-12T22:41:48Z_
_Verifier: Claude (gsd-verifier)_
