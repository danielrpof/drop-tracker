---
status: diagnosed
trigger: "G-25-9 (UAT test 9): at 375px the watchlist search shows only the first letter of the artist name; the 'Search MusicBrainz to add' button does not truncate"
created: 2026-10-11T00:00:00Z
updated: 2026-10-11T00:00:00Z
goal: find_root_cause_only
---

## Current Focus

hypothesis: CONFIRMED. The squeezed text is the Deezer search-result row's artist name, not a search input. In SearchResultRow the name column is `flex-1 min-w-0` (flex-basis 0, so it only gets leftover space), while the trailing "Search MusicBrainz to add" Button has a 205px max-content basis (25-char label, `whitespace-nowrap`) and `shrink-0`, with no width cap. At 375px only 10px is left for the name.
test: done. Headless Chrome against localhost:8080 with /search and /watchlist stubbed and non-GET blocked, plus runtime CSS differential experiments A-G
expecting: n/a
next_action: return ROOT CAUSE FOUND to orchestrator (goal: find_root_cause_only; no fix applied)
bug_class: Bohrbug (deterministic CSS flex layout)
reasoning_checkpoint:
  hypothesis: "The Deezer result row's name shows ~1 letter at 375px because the trailing disabled Button keeps its full 205px max-content width (nowrap label + shrink-0, no max-width), and the sibling name column is flex-1 (basis 0) + min-w-0, so it receives only the 10px of leftover space"
  confirming_evidence:
    - "Measured at 375px: li 311, button 205, name col 10, 0/9 chars before the ellipsis; screenshot shows 'B' and 'K' next to the full pill"
    - "Capping the button at 96px (E) gives the name 119px and 9/9 chars; shortening the label (D) gives 74px and 8/9"
    - "MusicBrainz row with the shorter 'Add to Watchlist' (135px) button shows 8/9 chars in the same layout"
  falsification_test: "If capping or shortening the button did not widen the name column, or the name stayed squeezed with the button removed, the hypothesis would be wrong. Both experiments widened it"
  fix_rationale: "n/a (diagnose-only). Direction: constrain the button's width or label so the name column gets the space. Making the button shrinkable alone is insufficient (exp. B)"
  blind_spots: "Measured with null image_url (placeholder CoverArt, same 48px box), stub data, and Chrome headless with Inter Variable. Real Deezer names and Safari font metrics differ slightly but cannot cover a ~195px deficit"
  candidate_causes:
    - "code: Button label length + whitespace-nowrap + shrink-0 with no max-width (SearchResultsColumns.tsx:190-198, button.tsx:7)"
    - "code: name column flex-basis 0 (flex-1), so button shrinkability never kicks in (SearchResultsColumns.tsx:174)"
    - "config/layout: route-wide p-8 padding at 375px (watchlist.tsx:491), a minor contributor (+32px)"
    - "environment: viewport/font metrics; ruled out as primary (deficit is structural, reproduces at 640px too)"
  and_gate: "yes. Two code conditions combine: the unconstrained 205px nowrap button AND the 0-basis name column. Exp. B (button shrinkable only) and F (name basis auto only) each change nothing; only G (both) or capping/shortening the button (D/E) helps. Page p-8 padding is an aggravating, non-necessary factor"

## Symptoms

expected: Typing an artist name into the Watchlist search at 375px viewport width leaves most of the query visible; the adjacent "Search MusicBrainz to add" button shrinks/truncates instead.
actual: User reported that at 375px, when searching an artist, only the first letter shows and the rest is truncated, because the "Search MusicBrainz to add" button is never truncated. User wants the opposite: shorten the MusicBrainz button so more of the artist name shows.
errors: None reported
reproduction: Test 9 in .planning/phases/25-find-filter-watchlist-and-history/25-UAT.md (ad-hoc, found during the 375px truncation test 6)
started: Discovered during UAT

## Eliminated

- hypothesis: The watchlist search INPUT (SearchBox or the WatchlistToolbar filter input) is squeezed by an adjacent button
  evidence: SearchBox.tsx renders the input alone in a `w-full max-w-xl` label with no sibling button. Measured at 375px, the input is 311px wide with value "Bad Bunny" fully visible (clientWidth 309 == scrollWidth 309). "Search MusicBrainz to add" renders only in SearchResultsColumns.tsx SearchResultRow, never next to any input.
  timestamp: 2026-10-11T00:00:00Z

- hypothesis: Making the button shrinkable (flex-shrink:1 / min-width:0) is enough to let the name win
  evidence: exp. B left the button at 205px and the name at 10px. The name column is flex-1 (flex-basis 0%), so the line has +10px free space and never overflows, and flex-shrink never runs
  timestamp: 2026-10-11T00:00:00Z

- hypothesis: Route-level p-8 padding is the primary cause
  evidence: exp. C (p-4) only raised the name from 10px to 42px (4/9 chars), and at 640px (sm two-column grid) the name is 0px regardless. Padding is an aggravating factor, not the cause
  timestamp: 2026-10-11T00:00:00Z

- hypothesis: A phase 25 regression
  evidence: 25-UI-SPEC.md:188 marks "SearchBox + SearchResultsColumns (unchanged, D-01)". git log shows SearchResultsColumns.tsx last touched in phase 06 (74c9129, 8418587, aeb472b squash). This is pre-existing phase 6 layout, surfaced by phase 25's 375px UAT pass
  timestamp: 2026-10-11T00:00:00Z

## Evidence

- timestamp: 2026-10-11T00:00:00Z
  checked: .planning/debug/knowledge-base.md
  found: no entry about layout, truncation, or 375px (only guest-feature-label-missing and backlog-songs-trigger-discord)
  implication: no known-pattern candidate

- timestamp: 2026-10-11T00:00:00Z
  checked: grep "MusicBrainz to add" in web/app
  found: the only render site is web/app/components/watchlist/SearchResultsColumns.tsx:190-198, the disabled trailing Button in SearchResultRow for Deezer (non-addable) results, className="shrink-0", size="sm". Its sibling name column is `flex min-w-0 flex-1` with a `truncate` span for artist.name.
  implication: the button sits next to the search RESULT's artist name, not next to the toolbar search input. The "search" text squeezed to one letter is most likely the Deezer result row's name.

- timestamp: 2026-10-11T00:00:00Z
  checked: web/app/components/ui/button.tsx:7 (buttonVariants base) and :26 (size sm)
  found: the base class already includes `shrink-0` and `whitespace-nowrap`; size sm is `h-8 px-3` text-sm. No max-width, no truncation of the label
  implication: the button's flex basis equals its full single-line label width, and it never shrinks

- timestamp: 2026-10-11T00:00:00Z
  checked: SearchResultsColumns.tsx:172-184 row structure
  found: li `flex items-center gap-3 p-3`; CoverArt fixed 48px (shrink-0); name column `flex min-w-0 flex-1 flex-col`; the name span has `truncate`; the type span (line 178) has NO truncate
  implication: the name column takes leftover space only. The type line ("artist") can overflow its column and collide with the button

- timestamp: 2026-10-11T00:00:00Z
  checked: headless Chrome (CDP) against localhost:8080, /search + /watchlist stubbed, all non-GET blocked (request log was only "STUB /watchlist", "STUB /search?q=Bad+Bunny"), typed "Bad Bunny"
  found: |
    375px: MusicBrainz row (Add to Watchlist, btn 135) name col 80px, 8/9 chars. Deezer row (Search MusicBrainz to add, btn 205) name col 10px, 0/9 and 0/14 chars fully visible before the ellipsis
    640px (sm, two columns): Deezer li 276, btn 205, name col 0px, nothing visible
    768px: Deezer name col 39px, 4/9 and 3/14 chars
    1024px: Deezer name col 167px, full names visible
    Toolbar SearchBox input at 375px: 311px wide, query fully visible. No horizontal page overflow at any width
    Screenshot row-375.png: Deezer rows show "B" / "K" beside the full pill, and the "artist" type text runs under the button's left edge
  implication: reproduces the UAT report exactly, and is worse at the sm breakpoint (640px) than at 375px. Math at 375: 311 li - 24 padding - 48 cover - 24 gaps - 205 button = 10px

- timestamp: 2026-10-11T00:00:00Z
  checked: differential runtime-CSS experiments at 375px, Deezer row (no source edits)
  found: |
    A baseline: btn 205, name 10, 0/9
    B button flex-shrink:1 + min-width:0: btn 205, name 10, 0/9 (no effect)
    C page padding p-8 -> p-4: btn 205, name 42, 4/9
    D label shortened to "MusicBrainz only": btn 141, name 74, 8/9
    E button max-width 96px + overflow hidden: btn 96, name 119, 9/9
    F name col flex:1 1 auto only: btn 205, name 10, 0/9 (no effect)
    G F + B together: btn 156, name 59, 6/9
  implication: the cause is the unconstrained button width combined with the 0-basis name column. Fixing either flex property alone does nothing. Capping or shortening the button, or changing both basis and shrink, restores the name

## Resolution

root_cause: In SearchResultRow (web/app/components/watchlist/SearchResultsColumns.tsx:172-198), the Deezer row's disabled "Search MusicBrainz to add" Button keeps its full single-line max-content width (205px; buttonVariants base `shrink-0 whitespace-nowrap` plus an explicit className="shrink-0", and no max-width). The sibling name column is `flex-1 min-w-0` (flex-basis 0), so it only receives leftover space. At 375px, 311px li - 24px padding - 48px cover - 24px gaps - 205px button = 10px for the name, so the truncating span shows about one glyph and an ellipsis. Because the name basis is 0, the line never overflows, so making the button shrinkable alone has no effect. Pre-existing phase 6 layout (unchanged in phase 25 per UI-SPEC D-01). Secondary: the type span (line 178) lacks `truncate`, so "artist" spills under the button.
fix:
verification:
files_changed: []
