# Phase 25 — UI Review

**Audited:** 2026-10-12
**Baseline:** 25-UI-SPEC.md (approved 2026-10-04, with G1–G7 overrides from 25-CONTEXT.md)
**Screenshots:** not captured. A server answered on :8080, but Playwright isn't installed (`npx --no-install playwright` refused), so this is a code-only audit.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | All locked strings match verbatim. The tag-filter input's locked `aria-label` is overridden by `aria-labelledby` |
| 2. Visuals | 3/4 | The hierarchy matches the spec (h2 + count, toolbar, cards). The `·` separator dangles when the release line wraps, and the 5 backstop visual checks were never rendered |
| 3. Color | 4/4 | No hardcoded colors. Accent only on focus rings and the empty-state CTA. Filter-active states are neutral and carry a Check icon |
| 4. Typography | 3/4 | The new surfaces use only the 4 tokens and 2 weights. A `font-medium` (500) remains in a phase-touched file, SearchResultsColumns.tsx:179 |
| 5. Spacing | 4/4 | Every toolbar gap and width matches the spec (`gap-4`, `gap-1`, `gap-2`, `sm:w-64`, `sm:w-56`, `sm:min-w-60 sm:max-w-md`, `max-h-72`, `max-w-64`) |
| 6. Experience Design | 3/4 | State coverage is thorough (3 empty states, loading and degraded Tag trigger, sticky cards, focus fallback). The long-text and overflow backstops are still unverified in a browser |

**Overall: 20/24**

---

## Top 3 Priority Fixes

1. **WARNING: The tag-filter input's accessible name is not the locked copy.** `WatchlistTagFilter.tsx` sets both `aria-labelledby="watchlist-tags-label"` and `aria-label="Filter by tags, any of"`. `aria-labelledby` takes precedence, so screen readers announce "Tags: any of" and the locked `aria-label` is dead code. Users still get a usable name, but the copy contract isn't met. Fix: drop one of the two. Keep `aria-label` to honor the contract, or update the spec to accept the visible label.
2. **WARNING: The 5 backstop visual checks were never rendered.** These are: a 32-character tag chip in the toolbar at 375px, a 32-character tag or long artist name in the History trigger, a long name-filter query next to the clear ×, an active card chip with a 32-character label in a narrow card, and a malformed date. Only unit tests and class assertions cover them, so truncation and overflow regressions at phone width would ship unseen. Fix: install Playwright (`corepack pnpm --dir web add -D playwright` or a global install), then capture 375px and 1440px views of `/?tags=…` and `/history?tag=…` with long fixtures.
3. **WARNING: The `·` separator dangles in the release line.** In `WatchlistRow.tsx:43-56` the separator is its own flex item inside `flex flex-wrap gap-x-2`. When `Latest release: …` and `Upcoming: …` don't fit on one line in a narrow card (`sm:flex-row` with the PreferenceToggles column beside it), the `·` ends the first line or starts the second. Fix: wrap the `·` and the Upcoming text in one `<span className="whitespace-nowrap">` so the separator travels with the Upcoming text, or render the two as separate lines, which the spec also allows.

Minor recommendations:
- `SearchResultsColumns.tsx:179` uses `font-medium` (500). Phase 25 edited this file (25-13/16/21), and the typography contract allows only weights 400 and 600, apart from Button's built-in weight. Change it to `font-semibold` or `font-normal`.
- `WatchlistRow.tsx:20-29` keeps a 10-line header comment that predates the phase and doesn't mention the new release lines. That breaks the project's comment discipline (CLAUDE.md), and the spec's own comment-rewrite note points the same way.
- The active card chip's ring (`ring-foreground/30`, about 2.6:1) is decorative by spec. Confirm in the browser that the `size-3` Check is legible against `#27272a`. It is the only non-color cue.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)
- Verified verbatim against the contract:
  - `Your artists`, the count via `formatArtistCount` (`lib/watchlistView.ts:249`, pluralized on M), `Filter watchlist`, `Filter by name…`, `Clear name filter`, `Sort by`, `Tags: any of`, `All tags`, `Show only`, `Has muted events`, `Custom release types`, `Clear filters` (WatchlistToolbar.tsx)
  - The popup empties `No tags yet — add one with “+ tag” on any artist's card.` and `No tags match “{q}”` (WatchlistTagFilter.tsx)
  - `Remove {tag} from tag filter`
  - `No artists match these filters` with body ``Try a different name or fewer filters, or clear them to see all {M} artist{s}.`` (watchlist.tsx:630-633)
  - The announcements `Showing {N} of {M} artists.`, `No artists match these filters.`, `Sorted by {label}.`, and `Filters cleared. Showing …` (watchlist.tsx:158-161, 520, 534)
  - History `No events for “{tag}”` with both body variants, the `this tag` fallback, `Show all tags`, `Loading tags…`, and `Tag unavailable` (history.tsx:57-80, 257-263; HistoryFilters.tsx:276-284)
  - The card chip's `Filter by {tag}`
- Defect: the tag-filter input's name conflict (Priority Fix 1).
- No generic labels (Submit, OK, Click here) were introduced.

### Pillar 2: Visuals (3/4)
- The focal point matches the spec. The `h2` (`text-heading font-semibold`) and the muted `tabular-nums` count sit above a toolbar placed on the page background with no panel, and cards stay the only `bg-card` surfaces.
- Icon-only controls all have names: the clear × (`Clear name filter`), the chip × (`Remove tag … from …`), and the card remove (`Remove {name} from watchlist`). Decorative icons carry `aria-hidden`.
- The card chip label and its × are two sibling buttons with separate hover fills (TagChips.tsx), which meets D-08.
- Defects:
  - The dangling separator (Priority Fix 3).
  - No rendered evidence of the wrap behavior at 375px.
  - The toolbar's `sm:items-end` alignment of `h-9` controls next to the wrapping `min-h-9` chips container was never seen. With many tags selected, the chips container grows taller and the other controls align to its bottom. That is by spec, but unverified.

### Pillar 3: Color (4/4)
- There are 0 hardcoded hex or `rgb(` values in `components/watchlist`, `components/history`, and `routes`.
- Accent use is limited to the spec's two reservations: inherited `focus-visible:ring-ring/50` rings, and the default-variant `Clear filters` in the filtered-to-zero EmptyState (watchlist.tsx:633).
- A pressed toggle is `secondary` (zinc-800) with a Check icon. Off is `outline`. Neither uses accent (WatchlistToolbar.tsx:41-60).
- The toolbar chip overrides the registry fill with `bg-secondary text-secondary-foreground`. The vendored remove × was changed from `opacity-50` to `text-muted-foreground hover:text-foreground` (ui/combobox.tsx:258), as the contract requires.
- The ghost toolbar Clear filters is muted, and History's `Show all tags` is `secondary`. Both are correct.

### Pillar 4: Typography (3/4)
- Phase-new surfaces use only `text-display`, `text-heading`, `text-body`, and `text-label`, at weights `font-semibold` and `font-normal`.
- The `ComboboxChip` built-in `text-xs font-medium` is overridden with `text-label font-normal`.
- The History trigger uses `text-body`, and the count uses `tabular-nums`.
- Defect: `SearchResultsColumns.tsx:179` uses `font-medium`, the only off-contract weight in the audited directories, in a file this phase modified three times.

### Pillar 5: Spacing (4/4)
- Every value matches the spec:
  - Toolbar: `flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-end`, with control columns at `gap-1`, toggles at `gap-2`, and widths `sm:w-64`, `sm:w-56`, and `sm:min-w-60 sm:max-w-md sm:flex-1`.
  - Section and heading row: `gap-4` and `gap-2`. Page: `gap-6 p-8`.
  - History: the listbox is `max-h-72 overflow-y-auto`, and the trigger is `min-w-40 max-w-64` (HistoryFilters.tsx:48, 190).
- The 24px dense-target exception is honored: the chip label is `h-6`, and the clear × and chip × are `icon-xs`. No arbitrary `[..px]` spacing was introduced. The `focus-visible:ring-[3px]` value is the project's standard ring.

### Pillar 6: Experience Design (3/4)
- **Loading:** the section renders only after the payload loads, and skeletons render until then. The History Tag control shows `Loading tags…` and `Tag unavailable` for a degraded fetch.
- **Empty:** three distinct empty states, with no duplicate Clear filters (`showClear` requires `visible.length > 0`, watchlist.tsx:615-617).
- **Error:** route-level errors are unchanged.
- **Interaction details:**
  - Esc on the name filter clears it only when it is non-empty.
  - Esc on a closed tag popup is cancelled, so it can't wipe the chips (WatchlistTagFilter.tsx:226-230).
  - The popup stays open on item press.
  - Sticky cards, the scroll anchor, and the keyboard-only focus fallback to the surviving card or the `tabIndex={-1}` heading are all present.
- Deductions:
  - Five backstop states have no visual verification (Priority Fix 2).
  - The tag-filter accessible-name conflict affects screen-reader UX.
- No destructive actions were added, so no confirmation is needed.

Registry audit: `components.json` is present. The UI-SPEC declares no third-party registries (`registries: {}`), so 0 third-party blocks were checked and there are no flags.

---

## Files Audited
- web/app/components/watchlist/WatchlistToolbar.tsx
- web/app/components/watchlist/WatchlistTagFilter.tsx
- web/app/components/watchlist/WatchlistRow.tsx
- web/app/components/watchlist/TagChips.tsx
- web/app/components/watchlist/SearchResultsColumns.tsx (typography grep)
- web/app/components/ui/combobox.tsx (vendored edit)
- web/app/components/history/HistoryFilters.tsx
- web/app/routes/watchlist.tsx
- web/app/routes/history.tsx
- web/app/lib/watchlistView.ts
