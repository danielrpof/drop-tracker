---
phase: 25-find-filter-watchlist-and-history
verified: 2026-10-11T20:55:00Z
status: gaps_found
score: 9/11 must-haves verified
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
  - .planning/phases/25-find-filter-watchlist-and-history/25-REVIEW.md
  - .planning/phases/25-find-filter-watchlist-and-history/25-UAT.md
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
covered_digest: "v1:sha256:4138975a44bd5bc51f09251a687f30275bd7c88eae97a8da29c4ae0e159b9faa"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 5/5
  gaps_closed:
    - "G-25-7: History cards show `Detected {date}` from created_at; retention stays on created_at; detected-recently/released-long-ago regression test passes"
    - "G-25-9 (code level): Deezer button capped at max-w-[40%] below lg with a truncating label span; type line truncates (real-browser layout check still pending)"
    - "G-25-4 main path: a chip-label click on a non-sticky card anchors the clicked chip in both directions (jsdom route tests)"
    - "Prior review WR-01..WR-04 (2cd98c2 review) fixed in 344d69b, 000b090, 4bdc558, 9cc7050"
  gaps_remaining:
    - "G-25-4: chip-label click on a sticky card (one just edited or added under the filter) loses the scroll anchor and focus"
  regressions:
    - "Review WR-03: since 000b090, a Manage tags merge or delete of a selected tag clears sticky cards (probe test red). Whether that breaks G4 depends on how the contract is read; the user needs to decide (see human_verification)"
gaps:
  - truth: "G-25-4: the router never resets scroll, and after a chip-label click far down a 50+ artist list the clicked card stays where it was on screen (clamped by the browser on short pages)"
    status: partial
    reason: "Fails when the clicked card is sticky (G4: edited in-card or added under the active filter) and does not match the old filter. handleTagsChange calls clearSticky() (sync lane), and React Router 7.18.2 applies the navigation inside React.startTransition (node_modules/react-router/dist/development/chunk-62JRHF6Z.mjs ~6752). So the first commit renders the old filter without the sticky card and unmounts it. The second commit applies the new filter and mounts a new DOM node. useScrollAnchor's captured chip is then disconnected, so no correction runs and focus falls to <body>. A verifier probe test went red: start at /?tags=3, remove reggaeton from Bad Bunny (now sticky), click Bad Bunny's `Filter by trap`. Result: URL ?tags=3,7, card shown again, label.isConnected=false, activeElement=BODY, window.scrollBy never called (expected (0,-280)). This also breaks the 25-11 truth that the chip label keeps focus on itself (D-08) on this path. This is review WR-01."
    artifacts:
      - path: "web/app/routes/watchlist.tsx"
        issue: "handleTagsChange (~441-448) clears stickyIds in a separate sync-lane state write that commits before the router's transition-lane location update; toggleFilterTag (~459) relies on the anchor surviving both commits"
      - path: "web/app/lib/useScrollAnchor.ts"
        issue: "Correct for a stable element, but the anchored element is destroyed by the intermediate commit, so the isConnected guard skips the correction"
      - path: "web/app/routes/watchlist.test.tsx"
        issue: "G-25-4 describe (~2100-2211) only uses fresh, non-sticky rows; no test combines a sticky card with a chip click"
    missing:
      - "Make sticky invalidation part of the same render as the URL change (e.g. key the sticky set to the filterKey it was created under and derive it during render, as 25-REVIEW.md WR-01 proposes), so the clicked sticky card is never unmounted between commits"
      - "Route test: make a card sticky (detach a selected tag), click one of its other chip labels, assert window.scrollBy was called with the drift and the label still has focus and is the same element"
human_verification:
  - test: "Scroll far down a 50+ artist Watchlist, click a card's chip label far down the list, then toggle `Has muted events`."
    expected: "The router never resets scroll. After the chip-label click far down a 50+ artist list, the clicked card stays where it was on screen, clamped by the browser as far as the page allows when the filtered list is shorter than the window. The muted toggle only promises that the router never resets scroll; a browser clamp on a short filtered list is expected (user decision 2026-10-11)."
    why_human: "Scroll restoration is browser-only (25-08 human-check)."
  - test: "On History with `EVENT_RETENTION_DAYS=45`, pick a tag, press Load more until it disappears, reload, then press Back. Also try changing the Tag while a Load more is in flight (WR-01)."
    expected: "Retention counts from detection, the card's `Detected` date, not the release date. No card ever shows a Detected date older than the retention window, even when its release date is years old; reload keeps the tag; Back restores the previous combination. Note whether a mid-flight filter change leaks rows from the previous filter (known advisory WR-01)."
    why_human: "Multi-page UX across the retention edge with real data, plus browser Back (SC4; 25-09 human-check)."
  - test: "UAT test 9 re-run: search an artist with Deezer results (e.g. `Bad Bunny`) and view the results at 375px, 640px, and 1024px."
    expected: "At 375px and 640px, each Deezer search-result row shows enough of the artist name to read it, and the `Search MusicBrainz to add` button truncates before the name does. At 1024px the full label shows; MusicBrainz rows look as before."
    why_human: "Layout truth (backstop, insufficient_spec for jsdom); class-level tests pass but widths need a real browser."
  - test: "Decision (review WR-02): with `?tags=A,B` selected, un-press a chip on a card that carries only that tag among the selected ones, far down the list."
    expected: "The clicked card leaves the list, so there is nothing to anchor. Plan 25-11 deliberately skips the correction here, so the page can collapse under the viewport the same way the original G-25-4 report described. Decide: accept this as a documented limitation (like the muted-toggle addendum), or require a fallback anchor (a neighbouring card) per 25-REVIEW.md WR-02."
    why_human: "Product decision. The user's 2026-10-11 G-25-4 decision doesn't cover a clicked card that leaves the list."
  - test: "Decision (review WR-03): filter by tag `rap`, remove `rap` from card X (X stays, sticky), then open Manage tags and merge `rap` into another tag X doesn't carry (or delete `rap` while another tag stays selected)."
    expected: "Currently X disappears: the filterKey effect added in 000b090 clears the sticky set on the route's own replaceSelectedTag write. A verifier probe test confirmed this. Decide whether a Manage tags merge or delete of a selected tag counts as a change to tag selection under G4, which would make this acceptable, or must keep sticky cards. The UI-SPEC says only G4 user actions clear the set."
    why_human: "The G4 contract (25-UI-SPEC lines 374, 400) can be read either way; only the user can settle it."
  - test: "Decision (review WR-04): at 375px, hover or tap the truncated `Search Mus…` pill on a Deezer row."
    expected: "The pill is disabled with `disabled:pointer-events-none`, so its explanatory `title` never shows. Decide whether the truncated label is acceptable as is, or the title/tooltip should move to a wrapper that receives pointer events."
    why_human: "UX judgment. The unreachable title dates from Phase 6, but truncation now leans on it."
---

# Phase 25: Find & Filter — Watchlist and History Verification Report

**Phase Goal:** With a 50+ artist watchlist, the user can find any artist or group of artists in seconds, by name, tag, or preference state and in the order they choose, and can narrow the History feed to the same tags.
**Verified:** 2026-10-11T20:55:00Z
**Status:** gaps_found
**Re-verification:** Yes. This re-verification follows UAT gap closure plans 25-11 to 25-14 for G-25-4, G-25-7, and G-25-9.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: Typing part of a name narrows the Watchlist as you type; "N of M artists" always shows | ✓ VERIFIED | Regression check. `watchlist.test.tsx` passes in full (part of the 212-test run below). UAT test 1 passed. |
| 2 | SC2: Sort by name, date added, or latest release, with stable ties and no-release artists last in both directions | ✓ VERIFIED | `watchlistView.test.ts` passes. `TestService_List_LatestRelease`, `TestService_ProjectionParity`, and `TestWatchlist_List_LatestRelease` PASS against Postgres. `queries/watchlist.sql` now also ignores malformed dates (4bdc558). UAT test 2 passed. |
| 3 | SC3: Tag, muted, and custom filters combine with search and sort; zero matches shows an empty state with one Clear filters action | ✓ VERIFIED | `watchlistView`, `WatchlistToolbar`, `WatchlistTagFilter`, and `watchlist.test.tsx` all pass. Sticky cards are now cleared on Back/Forward (000b090, test "drops a sticky card when Back restores a filter it does not match"). UAT test 3 passed. See WR-03 under Human Verification. |
| 4 | SC4: History filters by tag alongside the existing filters, includes removed artists, and never shows an event older than retention on the first page or via load-older | ✓ VERIFIED | `TestListEvents_TagFilter`, `TestRetention_TagFilterComposesWithCutoff`, `TestRetention_TagFilterKeepsOldReleasesDetectedInWindow` (new), and `TestListEvents_TagFilterComposesWithOtherFilters` all PASS. The stale Load more race is fixed (344d69b `feedKeyRef` guard). |
| 5 | SC5: Opening the Watchlist makes one request; search, sort, and filter make none | ✓ VERIFIED | The SC5 route test passes. UAT test 1 passed (network panel). |
| 6 | G-25-4: the router never resets scroll, and the clicked card stays where it was on screen after a chip-label click (clamped on short pages) | ✗ FAILED (partial) | Main path works. `useScrollAnchor` captures the chip's rect top, and a layout effect keyed on `filterKey` scrolls by the drift. The route tests (shrink: scrollBy(0,-280); grow: scrollBy(0,280); card leaves the list: no call) and 5 hook cases pass. `useUrlParams.ts` is unchanged, so `preventScrollReset: true` still applies. The sticky-card path fails: a verifier probe went red (anchor element remounted, no scroll correction, focus on BODY). See Gaps. |
| 7 | G-25-7: retention counts from detection; each History card shows `Detected {date}`; Load more under a tag never returns an event detected outside the window, even for years-old releases | ✓ VERIFIED | `EventCard.tsx` renders `Detected <time dateTime={created_at} title={formatIsoTitle(...)}>` after the body of all three variants and omits the line when `formatDetectedDate` returns null. EventCard and format tests pass. The new Go test walks limit=1 cursor pages: the 2015 release detected yesterday is returned, the one detected 120 days ago never is, and `has_older_events` is true. It PASSES. The SQL, sqlc code, and `internal/events` are unchanged since 461267f. |
| 8 | G-25-9: at 375px and 640px a Deezer row shows a readable artist name, and the hint button truncates before the name does | ⚠️ insufficient_spec → human | The code contract is verified. The Button has `max-w-[40%] shrink-0 lg:max-w-none` and an inner `<span className="truncate">`, the type span truncates, and the accessible name and `title` are unchanged. The three new class-level tests pass. Whether the name is readable is a layout truth that jsdom can't check, so it goes to human UAT test 9. |
| 9 | Embedded SPA rebuilt with all three fixes (25-14) | ✓ VERIFIED | Marker counts under `internal/webassets/build/client/assets`, compared with 461267f: `Detected` 2→3, `scrollBy` 0→1, `max-w-\[40\%\]` 0→1. No `web/` source changed after `afb881c`. |
| 10 | UI-SPEC, UAT, and VERIFICATION contracts reworded per the user's 2026-10-11 decisions (25-14) | ✓ VERIFIED | UI-SPEC line 399 ("Scroll reset and anchoring (D-12, G-25-4)"), line 414 (created_at, D-01), line 486 (`Detected {date}` row), and line 188 (G-25-9 exception). The UAT diff since 461267f touches 0 result, status, severity, root_cause, or count lines and exactly one decision line, G-25-4's addendum. Checks 4 and 7 below keep the 25-14 wording. |
| 11 | No change to retention SQL, sqlc, events, migrations, modules, lockfile, ui/, useUrlParams, or root.tsx since 461267f | ✓ VERIFIED | `git diff --name-only 461267f HEAD -- queries internal/db internal/events go.mod go.sum web/package.json web/pnpm-lock.yaml web/app/components/ui web/app/lib/useUrlParams.ts web/app/root.tsx` is empty. |

**Score:** 9 of 11 truths verified. 1 failed (partial) and 1 is insufficient_spec (human). 0 are present but behavior-unverified.

### Required Artifacts (gap closure)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/app/lib/useScrollAnchor.ts` | capture plus a layout-effect drift correction | ✓ VERIFIED | 21 lines. Imported by `watchlist.tsx` (`useScrollAnchor(filterKey)`). 5 hook tests pass. |
| `web/app/components/watchlist/TagChips.tsx` | label passes `e.currentTarget` | ✓ VERIFIED | `onClick={(e) => actions.toggleFilterTag(tag, e.currentTarget)}` |
| `web/app/routes/watchlist.tsx` | `toggleFilterTag` captures before writing | ✓ WIRED | Has the sticky-card defect (gap) |
| `web/app/lib/format.ts` | `formatDetectedDate` | ✓ VERIFIED | Null when unparseable. Used by EventCard. |
| `web/app/components/history/EventCard.tsx` | Detected line | ✓ VERIFIED | Plain JSX. `dangerouslySetInnerHTML` count is 0. |
| `internal/httpserver/events_test.go` | `insertTestEventDatedAt` plus the new retention test | ✓ VERIFIED | Order-checked (`sameIDs`) and terminates within 5 pages. PASS. |
| `web/app/components/watchlist/SearchResultsColumns.tsx` | capped Deezer button and truncating label and type | ✓ VERIFIED | Only the Deezer branch changed |
| `internal/webassets/build/client` | rebuilt bundle | ✓ VERIFIED | Marker counts above |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| TagChips label onClick | `watchlist.tsx toggleFilterTag` | `toggleFilterTag(tag, e.currentTarget)` | ✓ WIRED |
| `toggleFilterTag` | `useScrollAnchor` layout effect | `captureScrollAnchor(anchor)` → write tags → `filterKey` change → `scrollBy` | ⚠️ PARTIAL: breaks when an intermediate sticky-clear commit unmounts the anchor |
| GET /events `created_at` | EventCard Detected line | `formatDetectedDate(event.created_at)` | ✓ WIRED |
| `ListEvents created_at >= cutoff` | Go regression | `insertTestEventDatedAt` cursor walk | ✓ WIRED |
| Capped Deezer button | name column `flex min-w-0 flex-1` | `max-w-[40%] lg:max-w-none` | ✓ WIRED (layout → human) |
| `web/` | `internal/webassets/build/client` | make web recipe | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Real Data | Status |
|----------|---------------|--------|-----------|--------|
| EventCard Detected line | `event.created_at` | GET /events → `events.Event.CreatedAt` → `ListEvents` row | Yes | ✓ FLOWING |
| Scroll anchor | chip rect top | live DOM `getBoundingClientRect` | Yes | ✓ FLOWING (jsdom-stubbed in tests) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Gap-closure web suites | `vitest run useScrollAnchor, TagChips, watchlist route, useUrlParams, format, EventCard, history route, SearchResultsColumns` | 8 files, 212 tests passed | ✓ PASS |
| Original phase web suites (regression) | `vitest run watchlistView, WatchlistToolbar, WatchlistTagFilter, HistoryFilters, historyParams` | 5 files, 110 tests passed | ✓ PASS |
| Retention and tag filter (incl. new test) | `go test ./internal/httpserver/ -run 'TestRetention_\|TestListEvents_TagFilter\|TestHasOlderEvents_'` | 6 PASS | ✓ PASS |
| Latest release (regression) | `go test ./internal/watchlist/ ./internal/httpserver/ -run 'TestService_List_LatestRelease\|TestService_ProjectionParity\|TestWatchlist_List_LatestRelease'` | 3 PASS | ✓ PASS |
| Typecheck, prettier, go vet (httpserver) | `pnpm run typecheck`, `prettier --check`, `go vet` | clean | ✓ PASS |
| **Probe: sticky-card chip click (review WR-01)** | temporary test (deleted after the run): `/?tags=3`, detach reggaeton from Bad Bunny, click its `Filter by trap` | `?tags=3%2C7`, card shown, label.isConnected=false, activeElement=BODY, scrollBy calls `[]` | ✗ FAIL |
| **Probe: sticky card vs Manage tags merge (review WR-03)** | temporary test (deleted after the run): `/?tags=5`, detach rap from Kendrick, merge rap→trap | `?tags=9`, Kendrick gone (shown: Bad Bunny, Travis Scott) | ✗ FAIL (contract ambiguous → decision) |

Neither probe file was kept, and `git status --porcelain web` was empty afterwards. The full DoD was reported green by 25-14 (backend coverage 90.32%, web 95.16/90.47/95.8/97.09). This run did not repeat the full suite.

### Probe Execution

Step 7c: SKIPPED. No `scripts/*/tests/probe-*.sh` is declared or conventional for this phase.

### Prohibitions (gap-closure plans)

| Plan | Prohibition | Tier | Disposition |
|------|-------------|------|-------------|
| 25-11 | No change to `useUrlParams.ts`; never drop preventScrollReset | test | ✓ enforced: blast diff empty, `useUrlParams.test.tsx` passes |
| 25-11 | No height padding, `scrollTo(0,0)`, or `scrollIntoView` | test | ✓ enforced: grep of `watchlist.tsx`, `useScrollAnchor.ts`, and `TagChips.tsx` is empty |
| 25-11 / 25-14 | No npm package, module, or migration change | test | ✓ enforced: blast diff empty |
| 25-12 | No change to events SQL, sqlc, or `internal/events` | test | ✓ enforced: blast diff empty, new retention test PASS |
| 25-12 | Detected date never rendered as raw HTML | test | ✓ enforced: `dangerouslySetInnerHTML` count 0 |
| 25-13 | No edit to `ui/button.tsx`; copy and accessible name unchanged | test | ✓ enforced: blast diff covers `web/app/components/ui`; `getByRole(name: "Search MusicBrainz to add")` test passes |
| 25-14 | No UAT result, status, severity, root_cause, or count change; decision append-only | test | ✓ enforced: 0 forbidden lines in the diff, 1 decision line changed (G-25-4) |
| 25-04 (prior) | MUST NOT remove a card from view as a consequence of the user's own in-card edit (G4) | test | ⚠️ flagged: WR-03 probe shows a sticky card removed after a Manage tags merge. Whether that counts as a "tag selection change" is a human decision |
| 25-09 (prior) | No shared filter state between routes | judgment | ✓ resolved by human (UAT test 8 passed) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WLVW-01 | 25-04, 25-10, 25-13, 25-14 | Search the watchlist by name | ✓ SATISFIED | Truth 1. G-25-9 is a 375px add-flow polish item (visual check pending) |
| WLVW-02 | 25-05, 25-10 | Sort by name or date added, stable | ✓ SATISFIED | Truth 2 |
| WLVW-03 | 25-01, 25-03, 25-05, 25-10 | Sort by latest release, none last | ✓ SATISFIED | Truth 2 |
| WLVW-04 | 25-07, 25-08, 25-10, 25-11, 25-14 | Filter by one or more tags | ⚠️ SATISFIED, with a G-25-4 sub-path gap | Filtering works (Truth 3). The chip-click scroll and focus contract fails on sticky cards (Truth 6) |
| WLVW-05 | 25-08, 25-10 | Muted / custom release-type filters | ✓ SATISFIED | Truth 3 |
| WLVW-06 | 25-04, 25-07, 25-08, 25-10 | Combine; N of M; empty state | ✓ SATISFIED | Truths 1, 3 |
| HIST-02 | 25-02, 25-06, 25-09, 25-10, 25-12, 25-14 | History by tag, retention-respecting | ✓ SATISFIED | Truths 4, 7 |

All 7 IDs are claimed by at least one plan. REQUIREMENTS.md maps no other IDs to Phase 25, so no requirements are orphaned.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (gap-closure source and test files) | — | TBD/FIXME/XXX/TODO/HACK | none found | — |
| `web/app/routes/watchlist.tsx` | ~441-465 | Sticky clear in a separate sync-lane commit ahead of the router's transition commit (WR-01) | 🛑 Blocker | Probe red. The G-25-4 anchor and D-08 focus are lost on sticky cards |
| `web/app/lib/useScrollAnchor.ts` | 15 | No fallback when the clicked card leaves the list (WR-02) | ⚠️ Warning | Collapse returns on un-press with 2+ tags selected. Accepted in the plan, pending a user decision |
| `web/app/routes/watchlist.tsx` | 153-161 | `filterKey` effect clears stickies on the route's own `replaceSelectedTag` and canonical writes, after paint (WR-03) | ⚠️ Warning | Probe red for the merge case. One-frame stale sticky flash on Back |
| `web/app/components/watchlist/SearchResultsColumns.tsx` | 192-201 | Truncated label's only fallback is a `title` on a pointer-events-none disabled button (WR-04) | ⚠️ Warning | Truncated label unexplained below lg. Dates from Phase 6 |
| `web/app/lib/format.ts` | 7-19 | Stale long header comment (IN-01) | ℹ️ Info | CLAUDE.md comment discipline |
| `web/app/components/history/EventCard.tsx` | 86-96 | Mixed date formats on one card (IN-02) | ℹ️ Info | `2015-04-08` above `Detected 11 Oct 2026` |

### Assessment of 25-REVIEW.md warnings against the gap-closure truths

- **WR-01 undermines G-25-4. It is a blocker.** I confirmed the two-commit ordering in the React Router 7.18.2 source (`React3.startTransition(() => setStateImpl(newState))`, the default when `useTransitions` is unset), and a probe reproduced the failure deterministically. The clicked card is still in the list after the click, so plan 25-11's "card leaves the list" carve-out doesn't apply. The truth the user chose ("the clicked card stays where it was on screen") fails on this path, and the D-08 promise that focus stays on the label fails with it. Focus loss on sticky cards predates 25-11, but the scroll anchor is new gap-closure behavior.
- **WR-02 vs G-25-4 needs a decision, not a gap.** When the clicked card leaves the filtered list, there is no card left to stay on screen. Plan 25-11 truth 4 deliberately skips the correction, and a test pins that behavior. But the original symptom (the page drops after clicking a tag) comes back on that path, and the user's decision doesn't address it. Escalated in human_verification.
- **WR-03 is a regression from 000b090** (the earlier review's sticky-on-Back fix), not from 25-11 to 25-14. The probe confirms that a Manage tags merge of a selected tag removes a sticky card. Whether that violates G4 depends on whether a merge counts as a "tag selection change". Escalated as a decision. Note that the WR-01 fix (a render-derived sticky set keyed on `filterKey`) removes the post-paint flash, but on its own it would still drop stickies on a merge unless `applyTagChange` carries the set across.
- **WR-04 doesn't falsify G-25-9.** The name now gets the row, and the accessible name and title are unchanged. The unreachable tooltip is a UX follow-up. Escalated as a decision.

### Human Verification Required

These are listed even though the status is `gaps_found`, so the real-browser re-tests and decisions aren't lost. Items 1, 2, 3, 5, 6, and 8 passed in UAT on 2026-10-11 and are closed. Items 4 and 7 keep the wording 25-14 applied.

1. **One request / no requests while filtering (SC1, SC5).** Passed in UAT test 1 (2026-10-11). Closed.
2. **Sort stability on real data (SC2).** Passed in UAT test 2. Closed.
3. **Add/refresh under filters (G4).** Passed in UAT test 3. Closed.
4. **Scroll position.** Test: click a card's chip label far down the list, then toggle `Has muted events`. Expected: the router never resets scroll, and the clicked card stays where it was on screen (clamped by the browser on a short filtered list, an expected limitation per the 2026-10-11 decision). Why human: scroll restoration only shows in a browser.
5. **Contrast.** Passed in UAT test 5. Closed.
6. **375px truncation backstops.** Passed in UAT test 6. Closed.
7. **History retention edge plus Back (SC4).** Test: on History with `EVENT_RETENTION_DAYS=45`, pick a tag, press Load more until it disappears, reload, then press Back, and also try changing the tag mid-Load-more (WR-01). Expected: retention counts from detection, so no card shows a Detected date older than the window (even for years-old releases), and the tag persists. Why human: needs a real dataset and browser navigation.
8. **Judgment-tier prohibition (25-09).** Passed in UAT test 8. Closed.
9. **Deezer row at 375/640/1024px (G-25-9, UAT test 9).** Test: search `Bad Bunny` and view the Deezer column at each width. Expected: a readable artist name next to a truncated pill below 1024px, and the full label at 1024px. Why human: layout.
10. **Decision, WR-02.** Should un-pressing a chip whose card then leaves the list (2+ tags selected) keep a neighbouring card anchored, or should the collapse there be a documented limitation?
11. **Decision, WR-03.** Should a Manage tags merge or delete of a selected tag clear sticky cards (does it count as a G4 "tag selection" change)?
12. **Decision, WR-04.** Is the truncated, tooltip-less `Search Mus…` pill acceptable below lg, or should the title move to a wrapper?

### Gaps Summary

The gap closure achieved most of what it set out to do. G-25-7 is fully closed in code, with the Detected line and a passing retention regression test, and SC4 is now stronger. G-25-9 is closed at the code level and only needs the real-browser width check. The bundle carries all three fixes, the contracts carry the user's wording, and the blast radius is clean. The original phase's five success criteria still hold under regression tests.

One gap remains, in G-25-4. The scroll anchor works for ordinary cards. When the clicked card is sticky (just edited in-card or added under the active filter), the route clears stickies in an earlier commit than the router's transition-lane navigation. That unmounts and remounts the card, so the anchor and the label's focus are both lost. A probe test reproduces this deterministically. The fix belongs in `watchlist.tsx`: invalidate stickies in the same render as the URL change, plus a route test that clicks a chip on a sticky card. The fix interacts with WR-03, so settle the WR-03 decision (item 11) before planning, and plan WR-02 (item 10) together with it if the user wants a fallback anchor.

---

_Verified: 2026-10-11T20:55:00Z_
_Verifier: Claude (gsd-verifier)_
