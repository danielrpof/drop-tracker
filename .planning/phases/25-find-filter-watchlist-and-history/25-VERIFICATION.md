---
phase: 25-find-filter-watchlist-and-history
verified: 2026-10-10T18:10:00Z
status: human_needed
score: 5/5 must-haves verified
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
  - internal/events/service.go
  - internal/httpserver/events.go
  - internal/httpserver/events_test.go
  - internal/httpserver/watchlist_test.go
  - internal/watchlist/service.go
  - internal/watchlist/service_test.go
  - queries/events.sql
  - queries/watchlist.sql
  - web/app/components/history/HistoryFilters.test.tsx
  - web/app/components/history/HistoryFilters.tsx
  - web/app/components/ui/combobox.tsx
  - web/app/components/watchlist/ArtistNote.test.tsx
  - web/app/components/watchlist/PreferenceToggles.test.tsx
  - web/app/components/watchlist/SearchResultsColumns.test.tsx
  - web/app/components/watchlist/TagChips.test.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagCombobox.test.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/components/watchlist/WatchlistTagFilter.test.tsx
  - web/app/components/watchlist/WatchlistTagFilter.tsx
  - web/app/components/watchlist/WatchlistToolbar.test.tsx
  - web/app/components/watchlist/WatchlistToolbar.tsx
  - web/app/lib/api.test.ts
  - web/app/lib/api.ts
  - web/app/lib/format.test.ts
  - web/app/lib/format.ts
  - web/app/lib/historyParams.test.ts
  - web/app/lib/historyParams.ts
  - web/app/lib/tags.test.ts
  - web/app/lib/tags.ts
  - web/app/lib/test/fixtures.ts
  - web/app/lib/test/routeStub.tsx
  - web/app/lib/useUrlParams.test.tsx
  - web/app/lib/useUrlParams.ts
  - web/app/lib/watchlistView.test.ts
  - web/app/lib/watchlistView.ts
  - web/app/routes/history.test.tsx
  - web/app/routes/history.tsx
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
covered_digest: "v1:sha256:333e92bee150e360b5010c846044d5564ccf76a5a980df8128fedab4a85ca21d"
behavior_unverified: 0
overrides_applied: 0
advisory:
  - finding: "WR-01: a stale History 'Load more' response for filter A can append its rows (and cursor) onto filter B's feed after a filter change mid-request"
    category: other
    reason: "history.tsx handleLoadMore has no filter-identity guard (confirmed by reading lines ~217-241). Race-only; the server applies the retention cutoff on every request, so SC4's retention clause can never be violated by it, but the tag clause can be transiently violated. The handler predates this phase (artist/type filters already hit it). Resolve by keying appends on the filter key as 25-REVIEW.md suggests."
    evidence_status: "code-read only; no failing test"
  - finding: "WR-02: sticky cards (G4) survive filter changes made via browser Back/Forward"
    category: other
    reason: "watchlist.tsx clears stickyIds only from UI handlers; no effect clears it on URL-driven view changes (confirmed). Edge case after an in-card edit followed by Back/Forward; SC3 composition holds on every UI-driven path and is test-pinned. Resolve with a filter-key effect as 25-REVIEW.md suggests."
    evidence_status: "code-read only; no failing test"
  - finding: "WR-03: placeholder/malformed release_date text (e.g. 0000-00-00) is accepted by the latest/upcoming LATERALs, shown raw, and sorts as the oldest dated entry"
    category: other
    reason: "Raw rendering is the plan's accepted backstop (25-03 UI W6); the sort placement is a data-quality edge, not observed in real data."
    evidence_status: "none provided"
  - finding: "WR-04: the 'cancels a pending count announcement' test is wall-clock dependent and may not detect its regression"
    category: other
    reason: "Test-quality issue only; production code clears countTimer in handleSortChange (confirmed)."
    evidence_status: "none provided"
human_verification:
  - test: "Run the built binary against a dev DB with 50+ watched artists. Open the Watchlist with the network panel open, type `beyonce`, hold a key in Filter watchlist, then sort, toggle tag/muted/custom filters."
    expected: "Exactly one GET /watchlist on open and zero further requests while searching/sorting/filtering. Beyoncé matches `beyonce`. No browser history-throttling warning. Reload and Back reproduce the filtered view."
    why_human: "Network-panel request counting and history throttling are only observable in a real browser (SC1, SC5; 25-04/25-10 human-check)."
  - test: "Sort by each mode and flip its direction on real data where several artists share a name or latest-release date and some show `No releases yet`."
    expected: "Equal-valued artists never swap when direction flips; every `No releases yet` artist stays at the bottom under both latest-release directions."
    why_human: "Comparator is unit-pinned; perceived stability on the real payload is a UI-SPEC UAT item (SC2; 25-05 human-check)."
  - test: "Select a tag in `Tags: any of`, then add an artist from search and let the list refresh. With `Has muted events` on, add an artist. Uncheck an artist's last mute under `Has muted events`."
    expected: "Selected tag survives the refresh; the newly added artist is visible under the active toggle; the edited card stays until the next filter change."
    why_human: "Real add/refresh flows (G4) in the running app (25-07/25-08 human-check)."
  - test: "Scroll far down a 50+ artist Watchlist, click a card's tag chip label, then toggle `Has muted events`."
    expected: "Scroll position holds on every click (no jump to top)."
    why_human: "Scroll restoration is browser-only (25-08 human-check)."
  - test: "Measure the toolbar chip × and the pressed-toggle Check against the #27272a chip fill."
    expected: "Both meet 3:1 non-text contrast (about 5.8:1 per UI-SPEC)."
    why_human: "Contrast must be measured on the rendered app (25-07/25-08 human-check)."
  - test: "At a 375px viewport: a 32-character unbroken tag selected in the Watchlist tag filter, in a card chip label with the Check icon, and in the History Tag trigger; a long artist name in the History Artist trigger; a long query in Filter watchlist; a malformed latest-release date on a card."
    expected: "Everything truncates (title carries the full label) or wraps without overlapping; the clear × never overlaps query text; focus-within un-truncate still works on chip labels."
    why_human: "Held-out backstop visual checks (25-03 W6, 25-04 W1, 25-06 H1, 25-07 W3, 25-08 W7); jsdom cannot assert layout."
  - test: "On History, pick a tag whose artists have events on both sides of the retention window, press Load more until it disappears, reload, then press Back. Also try changing the Tag while a Load more is in flight (WR-01)."
    expected: "No event older than the retention window ever appears; reload keeps the tag; Back restores the previous combination. Note whether a mid-flight filter change leaks rows from the previous filter (known advisory WR-01)."
    why_human: "Multi-page UX across the retention edge with real data, plus browser Back (SC4; 25-09 human-check)."
  - test: "Prohibition review (judgment tier, 25-09): confirm Watchlist and History filter state is never shared or cross-linked and the nav links stay param-free."
    expected: "Watchlist owns q/sort/tags/muted/custom, History owns artist/type/tag, nav links carry no params."
    why_human: "unverified-prohibition — human review recommended. Non-authoritative LLM verdict: holds (root.tsx unchanged since 72f26da; each route's canonicalizer rewrites only its own keys)."
---

# Phase 25: Find & Filter — Watchlist and History Verification Report

**Phase Goal:** With a 50+ artist watchlist, the user can find any artist or group of artists in seconds, by name, tag, or preference state and in the order they choose, and can narrow the History feed to the same tags.
**Verified:** 2026-10-10T18:10:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Typing part of a name narrows the Watchlist as the user types; the view always shows "N of M artists" | ✓ VERIFIED | `watchlist.tsx` filters on local `query` state (URL write debounced 300 ms) through `selectVisible` → `matchesName` (NFD fold + ø/æ/ß/$ table). Header renders `formatArtistCount(visible.length, entries.length)` whenever the list is non-empty, including the zero-match state ("0 of M"). Route tests in `watchlist.test.tsx` (WLVW-01 block, `?q=bey` reload test) pass. |
| 2 | Sort by name A–Z/Z–A, date added newest/oldest, latest release; ties stable across re-sorts; no-release artists at bottom both directions | ✓ VERIFIED | `watchlistView.ts` `compareForSort`: six explicit comparators, each falling back to `artist_id` (not reversed per direction, D-16); `latestCompare` returns nulls last regardless of `dir`. `latest_release_date` comes from a LATERAL in `ListWatchlist`/`GetWatchlistEntry` (new_release, not upcoming at its own precision vs UTC today). Unit tests pin collator ties, identical instants, partial dates, nulls last in both directions; `TestService_List_LatestRelease` and `TestWatchlist_List_LatestRelease` PASS against Postgres. |
| 3 | Filter by one or more tags, muted event types, non-default release types; search/sort/filters combine; zero-match shows an empty state with one clear-filters action restoring the full list | ✓ VERIFIED | `selectVisible` ANDs name, `matchesAnyTag`, `hasMutedEvents`, `hasCustomReleaseTypes` (set comparison vs DEFAULT_RELEASE_TYPES mirroring Go), then sorts. Zero-match renders "No artists match these filters" with one `Clear filters` button; toolbar Clear is hidden when `visible.length === 0`, so only one action shows. `clearFilters` deletes q/tags/muted/custom (sort kept by design D-10). Tests "composes name, tag, muted and custom with AND", "replaces the list with the filtered-to-zero state and clears everything but sort" pass. See WR-02 note below. |
| 4 | Filter History by tag alongside existing filters; includes removed artists; never older than retention on first page or load-older | ✓ VERIFIED | `queries/events.sql`: `tag_id` is an `EXISTS` on `artist_tags` (keyed on `artists.id`, so removed artists still match) inside the same retention-aware `ListEvents` (cutoff is a required `sqlc.arg`) and in `HasOlderEvents`. Handler validates `tag_id` as positive int. `GET /tags` uses LEFT JOINs so tags carried only by removed artists still list. History UI writes `tag` to the URL and passes `tagId` to `listEvents` for both page one and Load more. Integration tests `TestListEvents_TagFilter` (artists with no watchlist row), `TestRetention_TagFilterComposesWithCutoff` (first page + cursor paging at limit=1, aged event never returned), `TestListEvents_TagFilterComposesWithOtherFilters` all PASS. See WR-01 note below. |
| 5 | Opening the Watchlist makes one request regardless of size; search/sort/filter make no further requests | ✓ VERIFIED | The route's only mount fetch is `refresh()` → `listWatchlist()`. `TagVocabularyProvider` loads `GET /tags` lazily (`ensureLoaded` only on chip-editor open); filter options come from `tagFilterOptions(entries)`. All filtering is pure client-side. Test "issues exactly one GET /watchlist while filtering and never asks for tags or events (SC5)" passes. Real-browser network-panel check routed to human verification. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `queries/watchlist.sql` | latest/next release LATERALs in List and Get | ✓ VERIFIED | Both queries carry identical LATERAL pair; sqlc regenerated (`internal/db/sqlc/watchlist.sql.go`) |
| `internal/watchlist/service.go` | `LatestReleaseDate`/`NextReleaseDate` on Entry | ✓ VERIFIED | Mapped in `entryFromRow`, JSON `latest_release_date`/`next_release_date` |
| `queries/events.sql` | tag_id filter in ListEvents + HasOlderEvents | ✓ VERIFIED | Same predicate in both, within retention query |
| `internal/events/service.go`, `internal/httpserver/events.go` | TagID threaded from handler to queries | ✓ VERIFIED | Parsed with `parseOptionalPositiveInt64`, passed to both queries |
| `web/app/lib/watchlistView.ts` | parse/canonicalize, match, sort, select | ✓ VERIFIED | 215 lines, used by `watchlist.tsx` |
| `web/app/components/watchlist/WatchlistToolbar.tsx` / `WatchlistTagFilter.tsx` | search, sort, tag any-of, muted/custom toggles, clear | ✓ VERIFIED | Rendered in `watchlist.tsx` with live props |
| `web/app/components/watchlist/TagChips.tsx` / `WatchlistRow.tsx` | chip click-to-filter, latest-release line | ✓ VERIFIED | `toggleFilterTag` wired via `tagActions` |
| `web/app/lib/historyParams.ts`, `web/app/lib/useUrlParams.ts` | URL-backed History filters | ✓ VERIFIED | Used by `history.tsx` |
| `web/app/components/history/HistoryFilters.tsx` | Tag control | ✓ VERIFIED | Receives `tags` from route's `GET /tags`, emits `tagId` |
| `internal/webassets/build/client` | rebuilt embedded SPA | ✓ VERIFIED | `watchlist-BuIICAgv.js` contains "Latest release (oldest)" and "No artists match these filters"; `api-*.js` contains `tag_id` |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `watchlist.tsx` | `GET /watchlist` | `listWatchlist()` in `refresh` on mount | ✓ WIRED |
| `watchlist.tsx` | `watchlistView.selectVisible` | `visible` computed each render from `entries` + URL view + local query | ✓ WIRED |
| `WatchlistToolbar` | URL params | `handleSortChange`/`handleTagsChange`/`handleFlagChange` → `useUrlParams().write` | ✓ WIRED |
| `history.tsx` | `GET /events?tag_id=` | `fetchHistoryPage` → `listEvents({tagId})` (page one + Load more) | ✓ WIRED |
| `handleListEvents` | `ListEvents`/`HasOlderEvents` | `events.ListParams.TagID` | ✓ WIRED |
| `ListWatchlist` row | API JSON | `entryFromRow` → `LatestReleaseDate` | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Real Data | Status |
|----------|---------------|--------|-----------|--------|
| Watchlist list/count | `entries` | `GET /watchlist` → `ListWatchlist` SQL | Yes | ✓ FLOWING |
| Latest-release line / sort | `latest_release_date` | LATERAL over `events` | Yes | ✓ FLOWING |
| Tag filter options | `tagOptions` | derived from `entries[].tags` (Phase 24 `tag_names`/ids projection) | Yes | ✓ FLOWING |
| History feed | `events` | `GET /events` with `tag_id` → `ListEvents` | Yes | ✓ FLOWING |
| History Tag menu | `tagsState.tags` | `GET /tags` → `ListTags` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Tag filter incl. removed artists, retention on first page + cursor pages, AND composition | `go test -count=1 -run 'TestListEvents_TagFilter\|TestRetention_TagFilterComposesWithCutoff\|TestListEvents_TagFilterComposesWithOtherFilters\|TestWatchlist_List_LatestRelease' ./internal/httpserver/` | 4 PASS | ✓ PASS |
| Latest/next release semantics + projection parity | `go test -count=1 -run 'TestService_List_LatestRelease\|TestService_ProjectionParity' ./internal/watchlist/` | 2 PASS | ✓ PASS |
| Phase web suites (view model, routes, toolbar, tag filter, history filters/params) | `corepack pnpm exec vitest run --coverage.enabled=false <7 phase test files>` | 7 files, 201 tests passed | ✓ PASS |

Full DoD (vet, golangci-lint, coverage 90.19%, sqlc drift, prettier, 566 web tests) was reported by 25-10 and not re-run in full here; the targeted runs above independently confirm the criterion-bearing tests.

### Probe Execution

Step 7c: SKIPPED (no probes declared or conventional `scripts/*/tests/probe-*.sh` for this phase).

### Prohibitions

| Plan | Prohibition | Tier | Disposition |
|------|-------------|------|-------------|
| 25-01 | No upcoming date in `latest_release_date`; Postgres decides, not the browser | test | ✓ enforced — `TestService_List_LatestRelease` (only-upcoming, mixed-precision cases) PASS |
| 25-02 | No out-of-retention event under a tag filter; tag filter not a separate query | test | ✓ enforced — `TestRetention_TagFilterComposesWithCutoff` PASS; predicate lives in `ListEvents`/`HasOlderEvents` |
| 25-02 | Must not exclude removed artists' events | test | ✓ enforced — `TestListEvents_TagFilter` (no watchlist rows) PASS |
| 25-04 | No API request while searching/sorting/filtering; no GET /tags or /events from Watchlist | test | ✓ enforced — SC5 route test PASS |
| 25-04 | In-card edit / add / Undo never vanish under a filter (G4) | test | ✓ enforced — sticky tests in `watchlist.test.tsx` PASS |
| 25-08 | Chip label click changes no server state | test | ✓ enforced — "clicking the label toggles the filter and never attaches or detaches" PASS |
| 25-09 | Failed GET /tags must not drop/misreport active tag filter | test | ✓ enforced — "keeps the tag filter applied and says so when GET /tags fails" PASS |
| 25-09 | No shared/cross-linked filter state between routes | judgment | ⚠️ unverified-prohibition — human review recommended (non-authoritative LLM verdict: holds) |
| 25-03/04/06/07/08 | Long-text truncation at 375px | backstop | ⚠️ insufficient_spec → human verification |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WLVW-01 | 25-04, 25-10 | Search watchlist by name | ✓ SATISFIED | Truth 1 |
| WLVW-02 | 25-05, 25-10 | Sort by name / date added with stable tie-break | ✓ SATISFIED | Truth 2 |
| WLVW-03 | 25-01, 25-03, 25-05, 25-10 | Sort by latest release (not upcoming); none sort last both directions | ✓ SATISFIED | Truth 2, backend LATERAL + tests |
| WLVW-04 | 25-07, 25-08, 25-10 | Filter by one or more tags | ✓ SATISFIED | Truth 3 |
| WLVW-05 | 25-08, 25-10 | Filter to muted / non-default release types | ✓ SATISFIED | Truth 3 |
| WLVW-06 | 25-04, 25-07, 25-08, 25-10 | Combine; "N of M"; empty state with clear-filters | ✓ SATISFIED | Truths 1, 3 |
| HIST-02 | 25-02, 25-06, 25-09, 25-10 | History by tag, combined, retention-respecting | ✓ SATISFIED | Truth 4 |

All 7 phase IDs are claimed by at least one plan; REQUIREMENTS.md maps no additional IDs to Phase 25. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (all phase-modified source files) | — | TBD/FIXME/XXX/TODO/HACK | none found | — |
| `web/app/routes/history.tsx` | ~217-241 | WR-01: no filter-identity guard on Load more | ⚠️ Warning | Race-only leak of previous-filter rows; retention unaffected |
| `web/app/routes/watchlist.tsx` | ~84-90, handlers | WR-02: stickies not cleared on URL-driven view change | ⚠️ Warning | After Back/Forward, a just-edited card can remain visible though it fails the restored filter |
| `queries/watchlist.sql` | LATERALs | WR-03: malformed dates accepted | ℹ️ Info | Data-quality edge |
| `web/app/routes/watchlist.test.tsx` | ~1419 | WR-04: timing-dependent test | ℹ️ Info | Test quality only |

### Assessment of WR-01 / WR-02 against success criteria

- **WR-01 vs SC4.** Confirmed in code. Every request, stale or not, carries the server-side retention cutoff, so "never shows an event older than the retention window" holds unconditionally. The race can briefly show rows for the previous artist/type/tag selection, which strains "lists events for artists carrying that tag" only when the user changes the filter while a Load more is in flight. The defect predates this phase (artist and type filters already triggered it). It does not falsify SC4 on the normal path. Classified as **WARNING**, fix recommended.
- **WR-02 vs SC3.** Confirmed in code. The sticky exception (G4) is an intentional design rule, and every UI-driven filter change clears it (test-pinned). Only browser Back/Forward after an in-card edit leaves a non-matching card visible. Search, sort, and filters still combine for every non-sticky card, and Clear filters still restores the full list. Classified as **WARNING**, fix recommended.

### Human Verification Required

1. **One request / no requests while filtering (SC1, SC5).** Test: 50+ artists, network panel, type `beyonce`, hold a key, sort and filter. Expected: one `GET /watchlist`, nothing else, no history-throttling warning, and reload/Back reproduce the view. Why human: needs a real browser network panel.
2. **Sort stability on real data (SC2).** Test: flip every sort direction where ties and `No releases yet` artists exist. Expected: ties never swap, and no-release artists stay at the bottom both ways. Why human: perceived stability on the real payload.
3. **Add/refresh under filters (G4).** Test: a selected tag survives an add plus refresh, an artist added under `Has muted events` shows, and unchecking a last mute keeps the card until the next filter change. Why human: these are real add flows.
4. **Scroll position.** Test: a chip label click and a toggle far down the list. Expected: no jump to the top. Why human: scroll restoration only shows in a browser.
5. **Contrast.** Test: the toolbar chip × and the pressed-toggle Check against #27272a. Expected: 3:1 or better. Why human: needs measurement on the rendered app.
6. **375px truncation backstops.** Test: 32-character tags in the filter, card chip, and History trigger, a long artist name, a long query, and a malformed date. Expected: text truncates or wraps cleanly. Why human: jsdom can't assert layout.
7. **History retention edge plus Back (SC4).** Test: page through the retention edge with Load more under a tag, reload, then press Back, and also try changing the tag mid-Load-more (WR-01). Expected: no aged-out events, and the tag persists. Why human: needs a real dataset and browser navigation.
8. **Judgment-tier prohibition (25-09).** Test: confirm the two routes never share or cross-link filter state. Status: unverified-prohibition, human review recommended. The LLM verdict is non-authoritative and says it holds.

### Gaps Summary

No blocking gaps. All five ROADMAP success criteria and all seven requirement IDs are backed by real, wired code with passing behavioral tests. That includes Postgres integration tests for the tag filter across removed artists and the retention edge, and the comparator tests for stable, nulls-last sorting. The status is `human_needed` because of the browser-only UAT items the plans deferred to end of phase, the 375px backstop checks, and one judgment-tier prohibition. Review warnings WR-01 and WR-02 are real edge-case defects but don't falsify SC3 or SC4 on the normal path; they're recorded as advisory with recommended fixes.

---

_Verified: 2026-10-10T18:10:00Z_
_Verifier: Claude (gsd-verifier)_
