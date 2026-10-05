---
phase: quick/261005-ezu
plan: 01
type: execute
wave: 1
depends_on: []
autonomous: true
requirements: [WLVW-03, WLVW-04, WLVW-06]

files_modified:
  - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md
  - .planning/REQUIREMENTS.md

estimate:
  tokens: 35000
  raw_tokens: 35000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "The 25-UI-SPEC banner names G1–G7, including G6 (History URL-state move is its own plan) and G7 (both comboboxes stay; base-ui unification is a backlog todo)"
    - "The upstream-contract bullet says D-01…D-16 and G1–G7 are locked and that a G wins where it conflicts with a D"
    - "The Upcoming line (G3) is covered everywhere the latest-release line is: Scope, Typography Label row, accent 'Never used for' list, the contrast row, the formatReleaseDate table, and the Data Contract bullet naming both latest_release_date and next_release_date"
    - "The Watchlist card additions section records the optional single wrapping line (latest release · Upcoming, aria-hidden separator, flex flex-wrap gap-x-2) as planner/executor discretion with copy unchanged"
    - "A '### Router & sticky-state rules' section sits between 'URL state' and 'History Tag filter' and states: preventScrollReset on every setSearchParams via one shared helper; an explicit stickyIds set cleared only by G4 user actions, never by an entries change; adds and Undo restores join stickyIds; URL tag-id validation runs only on the first successful payload load"
    - "No line of 25-UI-SPEC says a refresh or payload reload prunes the tag filter; the tag-carrier bullet, the invalid-params bullet, and the W3 last-carrier row all say validation happens only on the first successful payload load"
    - "UI Considerations has two new W5 partial rows (add under active filter; refresh after mutation) and the tally reads 35 applicable — 30 covered, 5 backstop, 0 unresolved"
    - "Checker Sign-Off lists three new UAT items: scroll position kept on chip click/filter toggle, add under 'Has muted events' shows the new card, refresh after add keeps a selected tag"
    - "REQUIREMENTS.md WLVW-03 excludes upcoming dates"
    - "25-UI-SPEC frontmatter still reads status: approved, 25-CONTEXT.md is untouched, and no file outside .planning/ changes"
  artifacts:
    - path: .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md
      provides: "Phase 25 UI contract aligned with grill overrides G1–G7 plus router scroll and sticky-state rules"
      contains: "### Router & sticky-state rules"
    - path: .planning/REQUIREMENTS.md
      provides: "WLVW-03 wording that excludes upcoming dates (G3)"
      contains: "that is not upcoming"
  key_links:
    - from: "25-UI-SPEC.md Router & sticky-state rules (scroll reset)"
      to: "web/app/root.tsx:37 ScrollRestoration"
      via: "cited file:line plus react-router 7.18.2 scrollTo(0, 0) unless preventScrollReset"
      pattern: "preventScrollReset: true"
    - from: "25-UI-SPEC.md Router & sticky-state rules (sticky set)"
      to: "web/app/routes/watchlist.tsx refresh() / handleAddSearchResult"
      via: "names the refresh() call sites that must not clear stickiness or the tag filter"
      pattern: "stickyIds"
---

<objective>
Apply the ui-ux-pro-max review findings to the Phase 25 UI-SPEC (and one REQUIREMENTS line) so the contract matches the post-discuss grill overrides G1–G7 and the real router and refresh behavior, before Phase 25 planning consumes it.

Purpose: 25-UI-SPEC.md is the contract the Phase 25 planner and executors copy from. Stale G-range wording, missing Upcoming-line coverage, and the missing scroll-reset and sticky-set rules would each ship as a bug (scroll jumps to the top on every chip click; refresh() silently un-sticks cards or prunes the user's tag filter).
Output: edited 25-UI-SPEC.md and REQUIREMENTS.md. Docs only.

Tracer task omitted on purpose: this is a docs-only change with no runtime layers to slice through.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.claude/CLAUDE.md
@.planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md
@.planning/phases/25-find-filter-watchlist-and-history/25-CONTEXT.md

Constraints that apply to every task:
- Docs only. Do not touch any file outside `.planning/`. Do not touch `25-CONTEXT.md`.
- Do not change the 25-UI-SPEC frontmatter (line 4 stays `status: approved`).
- CLAUDE.md concision: short additions, one design-doc reference per point, never re-argue a decision. Keep the spec's existing voice (plain sentences, bold lead-ins, backticked identifiers).
- Line numbers below are from the file as it stands before Task 1. Task 1 adds a paragraph, so Task 2 must locate its targets by section heading and bullet lead-in, not by line number.
- Facts already confirmed by the planner: `web/app/root.tsx` line 37 renders `ScrollRestoration`; installed react-router 7.18.2 returns early only when `preventScrollReset === true`, otherwise calls `window.scrollTo(0, 0)`; `watchlist.tsx` calls `refresh()` after a successful add (and on a 409), after an Undo restore (`addWatchlist(...).then(refresh)`), and after a failed remove; `addWatchlist` resolves to the new `WatchlistEntry`, so the added id is available from the POST response.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix stale G-range and Upcoming-line wording, record the one-line layout option, reword WLVW-03</name>
  <files>.planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md, .planning/REQUIREMENTS.md</files>
  <read_first>
    - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md (lines 13, 22, 36, 133, 157, 174, 255-298, 594)
    - .planning/phases/25-find-filter-watchlist-and-history/25-CONTEXT.md (Post-discuss grill overrides G1–G7)
    - .planning/REQUIREMENTS.md (line 27, WLVW-03)
  </read_first>
  <action>
Edit 25-UI-SPEC.md with targeted Edit calls (never a whole-file Write):

1. Banner, line 13 (finding A1): it currently names only the first five overrides. Replace the whole blockquote line with: "> **Grill overrides (2026-10-04):** G1–G7 in `25-CONTEXT.md` supersede the matching parts of this spec, and the affected sections below are already updated. In short, latest release ignores retention, there is a separate Upcoming line, cards are sticky instead of removed, the name filter folds `$ ø æ ß`, History's move to URL state is its own plan (G6), and both comboboxes stay, with base-ui unification left to a backlog todo (G7)." Keep it on line 13.

2. Upstream-contract bullet, line 36 (finding A2): it currently says the D decisions are locked and that this spec overrides none of them. Replace the text after the path and colon with: "decisions **D-01…D-16** and the post-discuss grill overrides **G1–G7** are locked; where a G conflicts with a D, the G wins".

3. Scope bullet, line 22 (finding A3): change the sentence to "`WatchlistRow` gains a latest-release line (D-03) and, when an upcoming date exists, an Upcoming line (G3)." and keep the TagChips sentence that follows unchanged.

4. Upcoming coverage alongside the latest-release line (finding A4):
   - Line 157 accent list: replace "the latest-release line" with "the latest-release and Upcoming lines", so it reads "...the count, the latest-release and Upcoming lines, the toolbar \"Clear filters\" (ghost)...".
   - Line 174 contrast row: the first cell becomes "Count, latest-release and Upcoming lines, toolbar labels"; other cells unchanged.
   - Same-class consistency fix, Typography table line 133 Label row: replace "the **latest-release line**" with "the **latest-release and Upcoming lines**".

5. formatReleaseDate (finding A6), Watchlist card additions section:
   - In the sentence ending "...pure helper `formatReleaseDate(raw)` in `web/app/lib/format.ts` (D-15):" change the ending to "(D-15), which formats both fields:".
   - Table header line 274 becomes "| Stored value | Rendered |".
   - The `null` row's Rendered cell becomes: "latest: the line reads `No releases yet` (no stored non-upcoming `new_release` event at any age, G1); next: no Upcoming line renders".

6. Layout discretion (finding C): directly after the paragraph ending "...no ICU variant can emit `Sept`." and before the "**Chip label as a filter toggle (D-08):**" paragraph, insert one paragraph: "**Layout discretion (planner/executor):** the latest-release and Upcoming lines may render as one wrapping line, `Latest release: 17 May 2024 · Upcoming: 12 Dec 2026`, with the `·` separator `aria-hidden` and the row `flex flex-wrap gap-x-2`, to keep cards compact. Copy is unchanged either way." Leave the layout diagram and Copywriting tables as they are.

7. Data Contract bullet, line 594 (finding A5): replace the bullet with "- `latest_release_date` and `next_release_date` both come from the single `GET /watchlist` query and the shared POST/PATCH projection (Phase 24 D-26). The SPA never computes either."

Then edit REQUIREMENTS.md line 27 (finding D, per G3): replace the parenthetical so the line reads "- [ ] **WLVW-03**: User can sort by latest release (the artist's newest own `new_release` event date that is not upcoming); artists with none sort last in both directions". Change nothing else in REQUIREMENTS.md.
  </action>
  <verify>
    <automated>cd C:/CodeProjects/drop-tracker && F=.planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md && R=.planning/REQUIREMENTS.md && sed -n 13p $F | grep -q '(G6)' && sed -n 13p $F | grep -q '(G7)' && test "$(grep -c 'nothing here overrides them' $F)" -eq 0 && grep -q 'where a G conflicts with a D, the G wins' $F && grep -q 'an Upcoming line (G3)' $F && grep -q 'the count, the latest-release and Upcoming lines, the toolbar' $F && grep -q '^| Count, latest-release and Upcoming lines, toolbar labels |' $F && grep -q 'latest-release and Upcoming lines\*\*' $F && grep -q 'which formats both fields:' $F && grep -q '^| Stored value | Rendered |$' $F && grep -q 'no Upcoming line renders' $F && grep -q 'Layout discretion (planner/executor)' $F && grep -q 'flex flex-wrap gap-x-2' $F && grep -q 'and `next_release_date` both come from the single `GET /watchlist` query' $F && grep -q 'event date that is not upcoming); artists with none sort last' $R && sed -n 4p $F | grep -qx 'status: approved' && echo TASK1_OK</automated>
  </verify>
  <done>Running the verify command prints TASK1_OK. The banner names G1–G7 with G6 and G7 summarized. Line 36 says G wins over D. The Upcoming line appears in Scope, Typography, the accent list, the contrast row, the formatReleaseDate table, and the Data Contract. The layout-discretion paragraph is in the card section. WLVW-03 excludes upcoming dates. The frontmatter is unchanged.</done>
</task>

<task type="auto">
  <name>Task 2: Add Router and sticky-state rules, remove refresh-time pruning wording, extend the UI Considerations rows and UAT list</name>
  <files>.planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md</files>
  <read_first>
    - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md (Interaction Contract: "Composition, count, and Clear filters", "URL state"; UI Considerations table and tally; Checker Sign-Off)
    - web/app/root.tsx (line 37, to confirm the citation)
    - web/app/routes/watchlist.tsx (refresh, handleAddSearchResult, handleRemove and its Undo action, to confirm the citations)
  </read_first>
  <action>
Edit 25-UI-SPEC.md with targeted Edit calls. Locate targets by heading and lead-in, because Task 1 shifted line numbers.

1. New section (finding B): insert after the last bullet of "### URL state (D-12, D-13)" (the "Tab links" bullet) and before "### History Tag filter (HIST-02, D-11)", with one blank line on each side:
   - Heading: "### Router & sticky-state rules"
   - Bullet 1: "- **Scroll reset (D-12):** `web/app/root.tsx:37` mounts `<ScrollRestoration />`, and react-router 7.18.2 calls `window.scrollTo(0, 0)` on every navigation, push or replace, unless it passes `preventScrollReset: true`. Every filter, sort, and typing change is a navigation, so Watchlist and History write params only through one shared helper that always passes `{ preventScrollReset: true }`, plus `replace: true` where this spec says replace. Otherwise a chip click 60 cards down jumps to the top."
   - Bullet 2: "- **Sticky set (G4):** keep an explicit `stickyIds` set. A card is visible when it matches the filters or `stickyIds.has(id)`, and an in-card edit adds its card's id. Only the G4 user actions clear the set (search text, sort, tag selection, a toggle, Clear filters, a chip-click filter, reload). An `entries` change never clears or recomputes it, because `watchlist.tsx` calls `refresh()` (a full `GET /watchlist`) after add-from-search, after an Undo restore, and after a failed remove."
   - Bullet 3: "- **Adds are sticky (G4):** an artist added from search (`handleAddSearchResult`) or restored by Undo joins `stickyIds` (id from the POST response), so an add under an active filter never succeeds invisibly."
   - Bullet 4: "- **Tag-id validation runs once (D-13):** URL tag ids are checked against the payload-derived set only on the first successful payload load. After that the tag filter changes only by user action or the Manage tags `onDeleted`/`onMerged` callbacks; `refresh()` never mutates it."

2. Superseded wording, so nothing implies a reload or refresh prunes the filter (finding B4):
   - "Composition, count, and Clear filters" section, the "**Tags that lose their last carrier:**" bullet: replace its text after the bold lead-in with "neither an in-session chip detach nor a later `refresh()` prunes the active tag filter. The tag stays selected until the user deselects it, Manage tags deletes or merges it, or a reload's first successful payload load drops it (D-13)."
   - "URL state" section, the "**Invalid params**" bullet: replace the Watchlist clause that validates tag ids whenever the payload loads with "a tag id absent from the payload-derived set (Watchlist), checked once on the first successful payload load (see Router & sticky-state rules);". Leave the rest of the bullet unchanged.
   - UI Considerations table, the "| partial | W3 last carrier detached |" row: replace its Resolution cell with "Neither an in-session detach nor a later `refresh()` prunes the tag filter. It changes only by user action, the Manage tags delete/merge callbacks, or a reload's first successful payload load (D-13)".

3. UI Considerations rows: directly after the "| partial | W5 in-card edit |" row, insert two rows:
   - "| partial | W5 add under active filter | ✅ covered | An artist added from search or restored by Undo joins the sticky set, so it shows even when it misses the active filters (G4) |"
   - "| partial | W5 refresh after mutation | ✅ covered | The `refresh()` after an add, an Undo, or a failed remove leaves both stickiness and the tag filter unchanged (G4) |"
   Then change the tally line above the table to "Applicable state considerations resolved: **35 applicable — 30 covered, 5 backstop, 0 unresolved.**" (keep the em dash and bold exactly as the existing line formats them).

4. Checker Sign-Off: append three items at the end of the "Verification items to carry into UAT / ui-review" list, after the existing sticky-card item and before the "**Approval:**" line:
   - "- [ ] Scrolled far down the Watchlist, clicking a card chip or toggling a filter keeps the scroll position (no jump to the top)."
   - "- [ ] With `Has muted events` on, adding an artist from search shows its new card."
   - "- [ ] With a tag selected, the `refresh()` after adding an artist leaves that tag in the filter."

Do not edit the "Sticky cards (G4)" bullet, the Approval line, or the frontmatter.
  </action>
  <verify>
    <automated>cd C:/CodeProjects/drop-tracker && F=.planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md && test "$(grep -c '^### Router & sticky-state rules$' $F)" -ge 1 && awk '/^### URL state/{u=NR} /^### Router & sticky-state rules/{r=NR} /^### History Tag filter/{h=NR} END{exit !(r>u && h>r)}' $F && grep -q 'preventScrollReset: true' $F && test "$(grep -c 'stickyIds' $F)" -ge 2 && test "$(grep -c 'checked when the payload loads' $F)" -eq 0 && test "$(grep -c 'pruned then' $F)" -eq 0 && test "$(grep -c 'Pruning happens on the next payload load' $F)" -eq 0 && test "$(grep -c 'first successful payload load' $F)" -ge 4 && test "$(grep -c '^| partial | W5' $F)" -ge 4 && grep -q '30 covered, 5 backstop, 0 unresolved' $F && test "$(grep -c '^- \[ \]' $F)" -ge 9 && sed -n 4p $F | grep -qx 'status: approved' && git diff --quiet c61be50 -- .planning/phases/25-find-filter-watchlist-and-history/25-CONTEXT.md && CH="$(git diff --name-only c61be50)" && test -z "$(printf '%s\n' "$CH" | grep -v '^\.planning/')" && echo TASK2_OK</automated>
  </verify>
  <done>Running the verify command prints TASK2_OK. The Router & sticky-state rules section sits between URL state and History Tag filter and has its four rules. No wording says a refresh or reload prunes the tag filter. The table has four W5 partial rows and the tally reads 35/30/5/0. Sign-Off lists nine open UAT items. 25-CONTEXT.md and every non-.planning file are unchanged.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| none | Docs-only edit to planning artifacts. No runtime code, input handling, or deployment surface changes |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-q261005-01 | Information disclosure | git commit of .planning docs | low | mitigate | The gitleaks pre-commit hook runs on the commit. Never pass `--no-verify` (CLAUDE.md) |
| T-q261005-02 | Tampering | 25-UI-SPEC contract integrity | low | mitigate | Verify gates assert `status: approved` is unchanged, 25-CONTEXT.md is unchanged, and no file outside .planning/ is modified |
</threat_model>

<verification>
- Both task verify commands print TASK1_OK and TASK2_OK.
- `git diff --stat c61be50` (the planning-time base) shows only 25-UI-SPEC.md, REQUIREMENTS.md, and quick-task planning files under `.planning/`.
- Skim the new section and the edited lines once for CLAUDE.md concision: no added paragraph runs over about 3 lines, and each point cites one design reference.
- Go, web, and sqlc Definition-of-Done gates do not apply because nothing outside .planning/ changes. The commit still runs through the hooks, with no `--no-verify` and no AI attribution trailer.
</verification>

<success_criteria>
- Every review finding (A1–A6, B1–B4 plus its row, tally, and UAT additions, C, and D) is reflected in the two files.
- 25-UI-SPEC.md is still `status: approved`, and 25-CONTEXT.md is byte-identical.
- No source, test, or build file changes.
</success_criteria>

<output>
Create `.planning/quick/261005-ezu-apply-phase-25-ui-spec-review-fixes/261005-ezu-SUMMARY.md` when done.
</output>
