---
phase: quick/261011-moj
plan: 01
type: execute
wave: 1
depends_on: []
autonomous: true
requirements: [BULK-01, BULK-02, BULK-03, BULK-04, BULK-05]

files_modified:
  - .planning/phases/26-bulk-edit-remove/26-CONTEXT.md
  - .planning/phases/26-bulk-edit-remove/26-UI-SPEC.md

estimate:
  tokens: 50000
  raw_tokens: 50000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "Both Phase 26 docs point at 261011-moj-DECISIONS.md (UI review U1-U4) beside their existing grill-amendment note, and the Q1-Q10 grill wording stays intact"
    - "26-UI-SPEC says ConfirmDialog for bulk remove is rendered by web/app/routes/watchlist.tsx at route level, not inside BulkActionBar, whose Remove only opens it (U1), in the scope bullet, the inventory row, and the bulk remove flow"
    - "Every bulk success in 26-UI-SPEC (add tag, remove tag, preferences, remove) runs close, then clear, then focus, with focus moved by an effect after the bar unmounts, and finalFocus is the bar's Remove on failure and the fallback target on success, picked when the dialog closes (U1)"
    - "26-UI-SPEC specifies html scroll-padding-bottom var(--bulk-bar-clearance, 0px), declared once in web/app/app.css, with a covered UI Considerations row and a UAT item that tabbing never leaves focus behind the bar (U2)"
    - "26-UI-SPEC makes the phone action bar one row: count min-w-0 flex-1, five icon-only size-11 controls with Clear included, gap-1, wrapper bottom-2 px-2, inline mode adding a second row below sm, desktop unchanged (U3)"
    - "26-UI-SPEC gives the bulk remove-tag success toast an Undo action at 8000ms that re-adds the tag by name to applied through bulk add-tag, with restore success, cap-warning and failure toasts, while bulk artist removal keeps no Undo (U4)"
    - "26-CONTEXT D-10, D-15 and D-16 carry the U3, U4 and U1 sentences; D-01..D-22 keep their numbers and nothing else in the file changes"
    - "26-UI-SPEC frontmatter still reads status: approved, the Checker Sign-Off dimension verdicts and approval line are byte-identical, the tally reads 59 rows with 42 covered, and no file outside .planning/ changes"
  artifacts:
    - path: .planning/phases/26-bulk-edit-remove/26-CONTEXT.md
      provides: "Phase 26 decisions with the UI review pointer and the U1/U3/U4 sentences on D-16, D-10 and D-15"
      contains: "one row at phone width (icon-only actions)"
    - path: .planning/phases/26-bulk-edit-remove/26-UI-SPEC.md
      provides: "Phase 26 UI contract amended with UI review U1-U4"
      contains: "**Success order (U1).**"
  key_links:
    - from: "26-CONTEXT.md amendments line and 26-UI-SPEC.md header blockquote"
      to: ".planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md"
      via: "relative repo path in the amendment note"
      pattern: "261011-moj-DECISIONS.md"
    - from: "26-UI-SPEC Spacing exception and Focus clearance paragraph"
      to: "web/app/app.css html rule"
      via: "specified scroll-padding-bottom declaration (implemented in Phase 26, not here)"
      pattern: "scroll-padding-bottom: var\\(--bulk-bar-clearance, 0px\\)"
    - from: "26-UI-SPEC Bulk remove flow and Success order paragraph"
      to: "web/app/components/common/ConfirmDialog.tsx handleConfirm"
      via: "names the close-after-onConfirm behaviour that forces the route-level host"
      pattern: "only after `onConfirm` resolves"
---

<objective>
Amend the Phase 26 planning docs in place with the locked UI review decisions U1–U4 from `261011-moj-DECISIONS.md`: a route-level remove dialog with a close → clear → focus order (U1), focus never hidden behind the bar (U2), a one-row phone action bar (U3), and Undo on the bulk remove-tag toast (U4). UI review decisions win over the current docs wherever they differ. The post-discuss grill amendments (Q1–Q10) and the Checker Sign-Off verdicts stay intact.

Purpose: `/gsd-plan-phase 26` and its executors copy from 26-UI-SPEC.md and 26-CONTEXT.md. If the docs stay stale, the phase would ship a dialog that drops focus on `<body>`, a bar that can hide the focused checkbox, a two-row phone bar using about 20% of the viewport, and a remove-tag that can only be reversed by rebuilding the selection by hand.
Output: edited 26-CONTEXT.md and 26-UI-SPEC.md. Docs only.

Tracer task omitted on purpose: this is a docs-only change with no runtime layers to slice through.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.claude/CLAUDE.md
@.planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md
@.planning/phases/26-bulk-edit-remove/26-CONTEXT.md
@.planning/phases/26-bulk-edit-remove/26-UI-SPEC.md

Constraints for every task:
- Docs only. Touch only the two files in `files_modified`. Do NOT edit `web/app/app.css`: U2 only *specifies* the `html` rule for Phase 26 to implement. Do not edit ROADMAP.md, and do not stage `.planning/milestone.lock`.
- Amend in place with targeted Edit calls. Never Write a whole existing doc. Keep the grill amendments (Q1–Q10 wording, D-19..D-22) and the Checker Sign-Off dimension lines and approval line exactly as they are. Frontmatter line 4 stays `status: approved`.
- The DECISIONS file is locked. Do not revisit, soften, or extend any U.
- CLAUDE.md prose discipline: short additions, plain sentences, one decision reference per point, backticked identifiers. Keep each doc's existing voice.
- Locate targets by heading, decision ID, bullet lead-in, or table row label, not by line number. Edits shift lines.
- Text after "with:" or on its own indented line under an item is the exact new text. Table rows are given as plain lines; paste them as-is.
- Facts confirmed by the planner: `ConfirmDialog.handleConfirm` (`web/app/components/common/ConfirmDialog.tsx`) calls `onOpenChange(false)` in `finally`, after `onConfirm` settles, and `finalFocus` is a `RefObject<HTMLElement | null>` prop. `web/app/app.css` already has an `html { @apply font-sans; }` rule inside `@layer base`. The single-artist remove toast in `web/app/routes/watchlist.tsx` already uses a sonner `action` with label `Undo`. 26-UI-SPEC currently tallies 57 UI Considerations rows: 40 covered, 7 backstop, and 10 dismissed.
- Commit each task atomically with a conventional `docs(quick-261011-moj): ...` message and no AI attribution trailer of any kind.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Amend 26-CONTEXT.md with the UI review pointer and the U1/U3/U4 sentences on D-16, D-10 and D-15</name>
  <files>.planning/phases/26-bulk-edit-remove/26-CONTEXT.md</files>
  <read_first>
    - .planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md (the whole file)
    - .planning/phases/26-bulk-edit-remove/26-CONTEXT.md (the amendments line under Status, and D-10, D-15, D-16)
  </read_first>
  <action>
Apply these four edits to 26-CONTEXT.md, each as a targeted Edit that appends to an existing line. Do not add or remove lines.

1. Amendments line (the line starting `**Post-discuss grill amendments (2026-10-11):**`, directly under `**Status:** Ready for planning`). Append one space and this sentence to the end of that same line:
UI review amendments U1–U4 in `.planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md` add to D-10, D-15 and D-16, and 26-UI-SPEC.md carries the detail.

2. D-10 (U3). Append one space and this sentence to the end of the `- **D-10:**` bullet, after "(SC1).":
The bar is one row at phone width (icon-only actions), per UI review U3.

3. D-15 (U4). Append one space and this sentence to the end of the `- **D-15:**` bullet, after "(TAG-07).":
Bulk remove-tag offers Undo (re-add by name to `applied` via bulk add-tag); this does not reverse D-15, which covers artist removal only (UI review U4).

4. D-16 (U1). Append one space and this sentence to the end of the `- **D-16:**` bullet:
The dialog is hosted at route level in `web/app/routes/watchlist.tsx`, not inside the action bar, whose `Remove` only opens it (UI review U1).

Leave everything else unchanged, including D-01..D-09, D-11..D-14, D-17..D-22, Claude's Discretion, canonical refs, Deferred Ideas, and the footer.

Commit: `docs(quick-261011-moj): add UI review U1, U3, U4 to Phase 26 context`, staging only 26-CONTEXT.md.
  </action>
  <verify>
    <automated>cd C:/CodeProjects/drop-tracker && F=.planning/phases/26-bulk-edit-remove/26-CONTEXT.md && sed -n 1,8p $F | grep -F 'Post-discuss grill amendments (2026-10-11)' | grep -F '261011-gq7-DECISIONS.md' | grep -qF '.planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md' && grep '^- \*\*D-10:\*\*' $F | grep -F 'always on screen (SC1)' | grep -qF 'The bar is one row at phone width (icon-only actions), per UI review U3.' && grep '^- \*\*D-15:\*\*' $F | grep -F 'Confirm only, with no Undo toast' | grep -qF 'Bulk remove-tag offers Undo (re-add by name to `applied` via bulk add-tag); this does not reverse D-15, which covers artist removal only (UI review U4).' && grep '^- \*\*D-16:\*\*' $F | grep -F 'reuses `ConfirmDialog`' | grep -F 'route level' | grep -qF 'not inside the action bar, whose `Remove` only opens it (UI review U1).' && ( for n in 01 02 03 04 05 06 07 08 09 10 11 12 13 14 15 16 17 18 19 20 21 22; do grep -q "^- \*\*D-$n:\*\*" $F || { echo "MISSING D-$n"; exit 1; }; done ) && ! grep -q '^- \*\*D-23:\*\*' $F && grep -qF '### Post-grill additions (2026-10-11)' $F && grep -qF '**Selection is derived, not trimmed** (grill Q1)' $F && grep -qF '**Status:** Ready for planning' $F && N=$(git diff HEAD --numstat -- $F) && test "$(printf '%s\n' "$N" | awk '{s+=$1} END{print s+0}')" -le 4 && test "$(printf '%s\n' "$N" | awk '{s+=$2} END{print s+0}')" -le 4 && echo TASK1_OK</automated>
  </verify>
  <done>The verify command prints TASK1_OK. The amendments line points at both DECISIONS files. D-10, D-15 and D-16 end with the U3, U4 and U1 sentences. D-01..D-22 are intact with no D-23, and the diff touches at most four lines.</done>
</task>

<task type="auto">
  <name>Task 2: Amend 26-UI-SPEC.md for U1 (route-level dialog, close → clear → focus) and U2 (focus clearance), plus the header pointer</name>
  <files>.planning/phases/26-bulk-edit-remove/26-UI-SPEC.md</files>
  <read_first>
    - .planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md (rows U1 and U2, and the closing paragraph about the header note)
    - .planning/phases/26-bulk-edit-remove/26-UI-SPEC.md (header blockquotes and Scope; Used this phase table; Spacing Exceptions; Action bar "Toast clearance"; Interaction Contract: Bulk add tag, Bulk preferences, Bulk remove, Focus management; UI Considerations tally and overflow rows; Checker Sign-Off verification items)
    - web/app/components/common/ConfirmDialog.tsx (handleConfirm, to confirm the close-after-onConfirm order the new text cites)
  </read_first>
  <action>
Apply these edits to 26-UI-SPEC.md, each as a targeted Edit. Do not change the frontmatter.

A. Header and scope
1. Header blockquote. Directly after the blockquote line starting `> **Post-discuss grill amendments (2026-10-11):**`, add a line containing only `>`, then this line, so both sit in one blockquote:
> **UI review amendments (2026-10-11):** decisions U1–U4 in `.planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md` amend this spec further and win where they differ: the route-level remove dialog and the close → clear → focus order on success (U1), focus never hidden behind the bar (U2), a one-row phone action bar (U3), and Undo on the bulk remove-tag toast (U4).
2. Scope. Replace the whole bullet that begins `- A new` and names `web/app/components/watchlist/BulkActionBar.tsx` with:
- A new `web/app/components/watchlist/BulkActionBar.tsx` docks at the bottom of the viewport while one or more artists are selected (D-10). It hosts the inline add-tag (`TagCombobox`, D-12) and remove-tag pickers, the Preferences popover (D-06, D-07), and the `Remove` trigger. The route hosts the `ConfirmDialog` it opens (D-16, U1).
3. Used this phase table, the row whose Component cell is `` `ConfirmDialog` (common) ``. Replace its Notes cell (keep the first two cells) with:
Bulk remove confirmation (D-16), `variant="destructive"`. Rendered by `web/app/routes/watchlist.tsx` at route level, not inside `BulkActionBar`; the bar's `Remove` only opens it (U1). `finalFocus` is passed explicitly and picked when the dialog closes: the bar's `Remove` on failure, the fallback target on success.

B. Focus clearance (U2)
4. Spacing Exceptions, the bullet starting `**Page bottom clearance is computed, not a token.**`. Append one space and these sentences to its end:
The same variable keeps focus clear of the bar: `html` carries `scroll-padding-bottom: var(--bulk-bar-clearance, 0px)`, declared once on the existing `html` rule in `web/app/app.css`. With the variable unset it resolves to 0, so other pages are unaffected (U2).
5. Layout, Action bar. Directly after the `**Toast clearance:**` paragraph (it ends "so other pages are unaffected."), add one blank line and this paragraph:
**Focus clearance (U2, WCAG 2.4.11):** while the bar is mounted, the `html` `scroll-padding-bottom` rule (see Spacing exceptions) makes a card checkbox that receives keyboard focus near the bottom of the viewport scroll clear of the bar, so focus is never hidden behind it. The rule lives once in `web/app/app.css`, and with the variable unset it resolves to 0.

C. Close → clear → focus on every bulk success (U1)
6. Bulk add tag, step 3 (**Success**). Keep its first sub-bullet (the `refresh()` one) unchanged. Replace its second and third sub-bullets with these two, at the same 3-space indent:
   - Inline mode exits first. Then the selection clears, which unmounts the bar, and the outcome toast fires (copy below).
   - Focus then moves to the fallback-chain target in an effect that runs after the bar has unmounted (U1, see **Success order** under Focus management).
7. Bulk preferences. Replace the whole `- **Success:**` bullet with:
- **Success:** the popover closes first. `refresh()` runs and the affected cards (`applied`) are marked sticky. Then the selection clears, which unmounts the bar, and the toast fires. Focus moves to the fallback-chain target after the bar has unmounted (U1).
8. Bulk remove. Replace its first bullet (the one where `Remove` opens `ConfirmDialog`) with:
- `Remove` opens `ConfirmDialog` (`variant="destructive"`). The dialog is rendered by `watchlist.tsx` at route level, not inside the bar; the bar's `Remove` only opens it, and the route passes a ref to that button into the bar (U1). base-ui's initial focus lands on `Cancel`, which renders first.
9. Bulk remove. Replace the whole `- **Success**` bullet with:
- **Success** (ids already gone count as removed, per D-03), in this order (U1): (1) the dialog closes (`onOpenChange(false)`); (2) then `selectedIds` empties, which unmounts the bar; (3) then focus moves to the fallback-chain target (see **Success order** under Focus management). Removed cards disappear, and the toast fires with **no Undo**. `refresh()` always runs (Q2).
10. Bulk remove. Directly after its `- **Cancel or Esc:**` bullet, add:
- **`finalFocus` (U1):** chosen when the dialog closes, not when it first renders: the bar's `Remove` on cancel, Esc, or failure, since the bar is still mounted then, and the fallback-chain target on success.
11. Focus management table. Replace the row whose first cell is `Any bulk action` with:
| Any bulk action | succeeded | **fallback chain**, after the surface closes and the bar unmounts (U1) |
12. Fallback chain. Replace the single sentence directly after the numbered list (the one that begins "Pass it to") with this paragraph:
**Success order (U1).** On every bulk success: (1) the surface closes (the dialog's `onOpenChange(false)`, the popover closed, or inline mode exited); (2) then `selectedIds` is emptied, which unmounts the bar; (3) then focus moves to the fallback-chain target in an effect that runs after the bar has unmounted (e.g. a `pendingFocus` flag consumed by `useEffect`). Never clear the selection inside `onConfirm` before the dialog has closed. `ConfirmDialog.handleConfirm` calls `onOpenChange(false)` only after `onConfirm` resolves, so a dialog rendered inside the bar would unmount first and focus would land on `<body>`; that is why the route hosts it. Its `finalFocus` is the bar's `Remove` on failure and the fallback target on success, picked when the dialog closes, not when it first renders.

D. UI Considerations and UAT (U2)
13. Directly after the row starting `| overflow | B8 toasts |`, insert:
| overflow | B1/B3 focus near the bar | ✅ covered | `html` carries `scroll-padding-bottom: var(--bulk-bar-clearance, 0px)`, so a card checkbox focused near the viewport bottom scrolls clear of the bar and focus is never hidden behind it (WCAG 2.4.11, U2) |
Then update the tally sentence to read exactly: Applicable state considerations resolved: **58 rows — 41 covered, 7 backstop, 10 dismissed, 0 unresolved.** Keep the parenthetical line about the probe's 22 rows unchanged.
14. Checker Sign-Off, verification items. Directly after the item about a failure toast appearing above the bar on desktop and at 375px, add:
- [ ] Tab through card checkboxes with the bar visible: the focused checkbox is never behind the bar.

Leave everything else unchanged, including the frontmatter, the grill (Q1–Q10) wording, every Checker Sign-Off dimension line and the approval line, and all U3/U4 targets (Task 3 owns them).

Commit: `docs(quick-261011-moj): add U1 focus order and U2 focus clearance to Phase 26 UI-SPEC`, staging only 26-UI-SPEC.md.
  </action>
  <verify>
    <automated>cd C:/CodeProjects/drop-tracker && F=.planning/phases/26-bulk-edit-remove/26-UI-SPEC.md && sed -n 4p $F | grep -qx 'status: approved' && grep -A2 -F 'Post-discuss grill amendments (2026-10-11)' $F | grep -q '^> \*\*UI review amendments (2026-10-11):\*\*' && grep '^> \*\*UI review amendments' $F | grep -qF '.planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md' && grep -qF 'The route hosts the `ConfirmDialog` it opens (D-16, U1).' $F && grep -F '| `ConfirmDialog` (common) |' $F | grep -qF 'at route level, not inside `BulkActionBar`' && grep -F '**Page bottom clearance is computed, not a token.**' $F | grep -qF 'scroll-padding-bottom: var(--bulk-bar-clearance, 0px)' && grep -qF '**Focus clearance (U2, WCAG 2.4.11):**' $F && grep -qF 'Inline mode exits first.' $F && grep -qF 'the popover closes first.' $F && grep -qF 'the route passes a ref to that button into the bar (U1)' $F && grep -qF 'in this order (U1)' $F && grep -qF '**`finalFocus` (U1):**' $F && grep -qF '| Any bulk action | succeeded | **fallback chain**, after the surface closes and the bar unmounts (U1) |' $F && grep -qF '**Success order (U1).**' $F && grep -qF 'pendingFocus' $F && grep -qF 'Never clear the selection inside `onConfirm` before the dialog has closed' $F && grep -qF 'only after `onConfirm` resolves' $F && grep -qF '| overflow | B1/B3 focus near the bar | ✅ covered |' $F && grep -qF '**58 rows — 41 covered, 7 backstop, 10 dismissed, 0 unresolved.**' $F && test "$(awk '/\| ✅ covered \|/{n++} END{print n+0}' $F)" = 41 && grep -qF -- '- [ ] Tab through card checkboxes with the bar visible: the focused checkbox is never behind the bar.' $F && ! grep -qF 'Remove trigger for `ConfirmDialog`' $F && ! grep -qF 'focus it directly in the other success paths' $F && ! grep -qF 'The selection clears, the bar unmounts, and the outcome toast fires' $F && ! grep -qF 'the bar unmounts, the toast fires, and focus follows' $F && ! grep -qF '`finalFocus` follows the fallback chain' $F && ! grep -qF 'succeeded (bar unmounts)' $F && ! grep -qF '**57 rows' $F && grep -qF 'Post-discuss grill amendments (2026-10-11)' $F && grep -qF '**D-01…D-22** are locked' $F && grep -qF '**Hidden selections (Q1, D-11):**' $F && grep -qF '`refresh()` runs, with no skeletons, and every id in `applied` is marked sticky (D-13)' $F && grep -qF '`refresh()` runs and the affected cards (`applied`) are marked sticky' $F && grep -qF '`refresh()` always runs (Q2).' $F && grep -qF 'targets the picked tag by **id** (D-22)' $F && grep -qF 'carried?: number' $F && grep -qF 'alerts already found for them may still be sent' $F && grep -qF '**Approval:** approved 2026-10-11 (gsd-ui-checker, 7/7 pass, 2 FLAGs accepted)' $F && H=$(git show HEAD:$F) && test "$(printf '%s\n' "$H" | grep -E '^- \[x\] Dimension|^\*\*Approval:\*\*')" = "$(grep -E '^- \[x\] Dimension|^\*\*Approval:\*\*' $F)" && echo TASK2_OK</automated>
  </verify>
  <done>The verify command prints TASK2_OK. The header blockquote points at the U1–U4 DECISIONS file. The scope bullet, the inventory row, and the bulk remove flow put `ConfirmDialog` at route level. Add tag, preferences, and remove successes follow close → clear → focus, with the **Success order** paragraph naming the `pendingFocus` effect and the `finalFocus` rule. The `html` `scroll-padding-bottom` rule is specified in Spacing and Layout, with a covered row and a UAT item. The tally reads 58/41. The grill wording and Checker Sign-Off verdicts are unchanged.</done>
</task>

<task type="auto">
  <name>Task 3: Amend 26-UI-SPEC.md for U3 (one-row phone action bar) and U4 (Undo on the bulk remove-tag toast)</name>
  <files>.planning/phases/26-bulk-edit-remove/26-UI-SPEC.md</files>
  <read_first>
    - .planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md (rows U3 and U4)
    - .planning/phases/26-bulk-edit-remove/26-UI-SPEC.md (Deliberately NOT used; Spacing table and Control sizes; Action bar Wrapper, Bar, Desktop, Phone, Inline mode; Bulk remove tag; Copywriting "Selection and action bar" and Toasts tables; UI Considerations B3 rows, B8 rows and tally; Data Contract bullets)
  </read_first>
  <action>
Apply these edits to 26-UI-SPEC.md, each as a targeted Edit. Do not change the frontmatter, the Checker Sign-Off dimension lines, or anything Task 2 added.

A. One-row phone action bar (U3)
1. Spacing table, the `| xs | 4px |` row. Append to its usage cell, before the closing pipe: `; ` then `gap-1` between action-bar items below `sm` (U3)
2. Spacing table. Replace the whole `| sm | 8px |` row with:
| sm | 8px | Action bar inner padding `p-2`, and `gap-2` between its items at `sm` and up; bar viewport inset `bottom-2 px-2` below `sm` (U3); `gap-2` between `Select all visible` and its text; `gap-2` between the popover's checkbox options |
3. Spacing table. Replace the whole `| md | 16px |` row with:
| md | 16px | Popover built-in `p-4 gap-4`; `gap-4` between the select-all control and the `N of M artists` count; card `p-4` (unchanged) |
4. Replace the whole `**Control sizes (multiples of 4):**` line with:
**Control sizes (multiples of 4):** checkbox box `size-4` (16px); selection hit area `size-11` (44px); bar buttons `h-9` (36px) at `sm` and up, and icon-only `size-11` (44px) below `sm`, Clear included (U3); inline picker input `h-9`, `sm:w-60` (240px); popover `w-80` (320px); remove-tag listbox `max-h-72` (288px).
5. Action bar. Replace the whole `**Wrapper:**` line with:
**Wrapper:** `fixed inset-x-0 bottom-2 z-40 flex justify-center px-2 pointer-events-none sm:bottom-6 sm:px-8`.
6. Action bar. Replace the whole `**Bar:**` line with the line below. It swaps the column stack for a wrapping row with `gap-1` below `sm`, and resolves to the same centered row with `gap-2` at `sm` and up, so desktop is unchanged:
**Bar:** `<div role="region" aria-label="Selection actions" class="pointer-events-auto flex w-full flex-wrap items-center gap-1 rounded-2xl bg-popover p-2 text-popover-foreground shadow-2xl ring-1 ring-foreground/10 sm:w-auto sm:max-w-full sm:flex-nowrap sm:gap-2 motion-safe:animate-in motion-safe:fade-in-0 motion-safe:slide-in-from-bottom-4 motion-safe:duration-150">`
7. Action bar, the Phone block (the bold label starting `**Phone (< 640px)`, the fenced diagram under it, and the two bullets after the fence). Make three sub-edits:
   a. Change the bold label line to: **Phone (< 640px), one row (U3):**
   b. Inside that fenced diagram, keep both fence lines and replace the two diagram lines between them with this single line:
[ 12 selected ]  [TagPlus] [TagX] [Sliders] [Trash2] [X]      ← gap-1; each control size-11 (44px), icon-only
   c. Replace the two bullets after the fence (about `sr-only` action text and about `Clear`) with these five bullets:
- `{N} selected` is `min-w-0 flex-1` (visible text `{N} selected`, `tabular-nums`).
- The five controls are icon-only `size-11` (44px) `ghost` buttons. Below `sm`, each action's text is `sr-only`: use `<span class="sr-only sm:not-sr-only">Add tag</span>`, so the accessible name is identical at every width, and add `title` with the same text.
- `Clear` is icon-only `X` below `sm`, with `<span class="sr-only sm:not-sr-only">Clear</span>`, `aria-label="Clear selection"` (unchanged), and `title="Clear selection"`. It keeps its visible text at `sm` and up.
- Width check at 375px: the bar is 359px, and `p-2` leaves 343px inside. Five 44px controls and five 4px gaps take 240px, which leaves 103px for the count, enough for `1000 selected`.
- One row is about 60px tall. The earlier phone layout took about 130px, roughly 20% of a 667px viewport.
8. Replace the whole `**Inline mode (add-tag or remove-tag active):**` paragraph with:
**Inline mode (add-tag or remove-tag active):** at `sm` and up, the action group is replaced by `[picker input, sm:w-60] [Cancel]`, and the count and Clear stay where they are. Below `sm`, the four action icons hide and the picker row `[input flex-1] [Cancel]` renders as a second row under `[count … Clear]` (e.g. `order-last basis-full sm:order-none sm:basis-auto` on the picker group); the bar returns to one row when inline mode exits (U3). Only one inline mode can be active at a time, and Preferences cannot be open alongside it.
9. Copywriting, Selection and action bar table. Replace the row whose first cell starts `| Clear (visible` with:
| Clear (visible / `aria-label` / `title`) | `Clear` at `sm` and up, icon-only `X` below `sm` with `sr-only` text / `Clear selection` / `Clear selection` |
10. UI Considerations. Replace the whole `| populated | B3 bar |` row with:
| populated | B3 bar | ✅ covered | Desktop is one row (`{N} selected`, four ghost actions, Clear); phone is also one row, with icon-only `size-11` actions and Clear carrying `sr-only` text, and inline mode adds a second row (U3) |
11. UI Considerations. Replace the whole `| overflow | B3 at 375px |` row with:
| overflow | B3 at 375px | 🧪 backstop | One row: `{N} selected` (`min-w-0 flex-1`) and five icon-only `size-11` controls with `gap-1` fit the 343px inner width and leave 103px for the count, so a 4-digit count (`1000 selected`) fits with no horizontal scroll; inline mode adds a second row (U3). Held-out visual test at 375px |

B. Undo on the bulk remove-tag toast (U4)
12. Deliberately NOT used. Replace the bullet that cites D-15 with:
- **Undo for bulk artist removal** (D-15, deferred). This covers removing artists only; the bulk remove-tag toast does offer Undo (U4).
13. Bulk remove tag. Keep its existing paragraph unchanged. Directly after it, add one blank line and this paragraph:
**Undo (U4):** when `applied` is non-empty, the success toast carries an `Undo` action (sonner `action`, as the single-artist remove toast in `watchlist.tsx` already does) and stays 8000ms. Undo sends **one** bulk add-tag request with the tag's **name** and the remove response's `applied` ids, through the same bulk add-tag endpoint (normalization, get-or-create, the cap rule D-01). It does not touch the current selection, since a new one may exist, and does not mount the bar. There is no extra UI while it is in flight. On success `refresh()` runs, the restored ids are marked sticky (D-13), and the restore toast fires. Cap skips, possible only if other tags were added in between, use the restore warning, and a failure fires the restore failure toast (copy below). Bulk artist removal keeps D-15: no Undo.
14. Toasts table. Replace the row whose Trigger cell is `Remove tag, applied` with:
| Remove tag, applied | `Removed “{tag}” from {n} artist{s}.` (n = artists that carried it) | — | `toast.success`, 8000ms, `Undo` action when `applied` is non-empty (U4) |
15. Toasts table. Directly after that row (so both sit above the `gone` ids row, which then also covers restores), insert:
| Undo remove tag, restored | `Restored “{tag}” on {n} artist{s}.` | — | `toast.success`, default |
| Undo remove tag, some at cap | `Restored “{tag}” on {n} artist{s}.` | `{k} already {k=1 ? "has" : "have"} 10 tags: {names}.` | `toast.warning`, 8000ms |
16. Toasts table. Directly after the `Remove tag failed` row, insert:
| Undo remove tag failed | `Couldn't restore “{tag}” — try again.` | — | `toast.error` |
17. UI Considerations. Directly after the row starting `| populated | B8 after action |`, insert:
| interaction | B8 remove-tag Undo | ✅ covered | When `applied` is non-empty, the remove-tag success toast offers `Undo` for 8000ms; Undo re-adds the tag by name to those ids in one bulk add-tag request, leaves the current selection and bar alone, then runs `refresh()` and marks the restored ids sticky, with restore success, cap-warning, and failure toasts (U4) |
18. Data Contract. Directly after the bullet starting `- Remove-tag toast copy uses`, add:
- The `applied` ids from remove-tag are reused for its Undo, which re-adds the tag by name through the bulk add-tag endpoint (U4).
19. Update the UI Considerations tally sentence to read exactly: Applicable state considerations resolved: **59 rows — 42 covered, 7 backstop, 10 dismissed, 0 unresolved.** Keep the parenthetical line about the probe's 22 rows unchanged.

Leave everything else unchanged, including the Desktop block, the B6 375px row, the Destructive actions summary, and every Checker Sign-Off line.

Commit: `docs(quick-261011-moj): add U3 one-row phone bar and U4 remove-tag Undo to Phase 26 UI-SPEC`, staging only 26-UI-SPEC.md.
  </action>
  <verify>
    <automated>cd C:/CodeProjects/drop-tracker && F=.planning/phases/26-bulk-edit-remove/26-UI-SPEC.md && sed -n 4p $F | grep -qx 'status: approved' && grep -F '| xs | 4px |' $F | grep -qF '`gap-1` between action-bar items below `sm` (U3)' && grep -F '| sm | 8px |' $F | grep -qF 'bar viewport inset `bottom-2 px-2` below `sm` (U3)' && grep -qF 'icon-only `size-11` (44px) below `sm`, Clear included (U3)' $F && grep -qF '**Wrapper:** `fixed inset-x-0 bottom-2 z-40 flex justify-center px-2 pointer-events-none sm:bottom-6 sm:px-8`.' $F && grep -qF 'flex w-full flex-wrap items-center gap-1 rounded-2xl' $F && grep -qF 'sm:flex-nowrap sm:gap-2' $F && grep -qF '**Phone (< 640px), one row (U3):**' $F && grep -qF '[ 12 selected ]  [TagPlus] [TagX] [Sliders] [Trash2] [X]' $F && grep -qF '`{N} selected` is `min-w-0 flex-1`' $F && grep -qF '<span class="sr-only sm:not-sr-only">Clear</span>' $F && grep -qF 'title="Clear selection"' $F && grep -qF 'enough for `1000 selected`' $F && grep -qF 'renders as a second row under `[count … Clear]`' $F && grep -qF 'the bar returns to one row when inline mode exits (U3)' $F && grep -qF '| Clear (visible / `aria-label` / `title`) |' $F && grep -qF 'phone is also one row' $F && grep -F '| overflow | B3 at 375px | 🧪 backstop |' $F | grep -qF 'leave 103px for the count' && ! grep -qF 'bottom-4 px-4' $F && ! grep -qF 'inset-x-0 bottom-4' $F && ! grep -qF 'grid-cols-4' $F && ! grep -qF 'Two-row layout' $F && ! grep -qF 'phone is two rows' $F && ! grep -qF 'keeps its visible text at every width' $F && ! grep -qF 'on phone, row 2' $F && ! grep -qF 'w-full flex-col gap-2' $F && ! grep -qF 'and `h-11` (44px) icon-only' $F && ! grep -qF '| Clear (visible / `aria-label`) |' $F && grep -qF '**Undo for bulk artist removal** (D-15, deferred).' $F && ! grep -qF '**Undo for bulk remove** (D-15' $F && grep -qF '**Undo (U4):**' $F && grep -qF "Undo sends **one** bulk add-tag request" $F && grep -qF 'the restored ids are marked sticky (D-13)' $F && grep -F '| Remove tag, applied |' $F | grep -F '8000ms' | grep -qF '`Undo` action when `applied` is non-empty (U4)' && grep -F '| Undo remove tag, restored |' $F | grep -qF 'Restored “{tag}” on {n} artist{s}.' && grep -F '| Undo remove tag, some at cap |' $F | grep -F 'Restored “{tag}” on {n} artist{s}.' | grep -qF '`toast.warning`, 8000ms' && grep -qF "| Undo remove tag failed | \`Couldn't restore “{tag}” — try again.\` |" $F && awk '/^\| Undo remove tag, restored \|/{a=NR} /^\| Any of the above with/{b=NR} END{exit !(a && b && a<b)}' $F && grep -qF '| interaction | B8 remove-tag Undo | ✅ covered |' $F && grep -qF 'The `applied` ids from remove-tag are reused for its Undo' $F && grep -qF '**59 rows — 42 covered, 7 backstop, 10 dismissed, 0 unresolved.**' $F && test "$(awk '/\| ✅ covered \|/{n++} END{print n+0}' $F)" = 42 && test "$(awk '/\| 🧪 backstop \|/{n++} END{print n+0}' $F)" = 7 && test "$(awk '/\| ➖ dismissed \|/{n++} END{print n+0}' $F)" = 10 && grep -qF '**Success order (U1).**' $F && grep -qF 'pendingFocus' $F && grep -qF 'scroll-padding-bottom: var(--bulk-bar-clearance, 0px)' $F && grep -q '^> \*\*UI review amendments (2026-10-11):\*\*' $F && grep -qF 'Post-discuss grill amendments (2026-10-11)' $F && grep -qF '**Hidden selections (Q1, D-11):**' $F && grep -qF '`refresh()` always runs (Q2).' $F && grep -qF 'targets the picked tag by **id** (D-22)' $F && grep -qF '**Approval:** approved 2026-10-11 (gsd-ui-checker, 7/7 pass, 2 FLAGs accepted)' $F && H=$(git show HEAD:$F) && test "$(printf '%s\n' "$H" | grep -E '^- \[x\] Dimension|^\*\*Approval:\*\*')" = "$(grep -E '^- \[x\] Dimension|^\*\*Approval:\*\*' $F)" && G=$(git status --porcelain) && test -z "$(printf '%s\n' "$G" | awk 'NF{print $NF}' | grep -vE '^\.planning/')" && echo TASK3_OK</automated>
  </verify>
  <done>The verify command prints TASK3_OK. The phone bar is one row with icon-only `size-11` controls, Clear included, `gap-1`, and a `bottom-2 px-2` wrapper. Inline mode adds a second row below `sm`, and desktop is unchanged. The Clear copy row, the B3 populated row, and the B3 375px row describe the single row. The remove-tag toast carries `Undo` at 8000ms, with restore success, cap-warning, and failure rows, a Bulk remove tag Undo paragraph, a B8 Undo row, and a Data Contract note. The Deliberately NOT used bullet limits no-Undo to artist removal. The tally reads 59/42/7/10. The grill and Task 2 wording and the Checker Sign-Off verdicts are intact, and nothing outside `.planning/` changed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| planning docs → Phase 26 planner/executors | The amended docs become the contract later code is written from. No runtime boundary changes in this task. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-moj-01 | Tampering | 26-CONTEXT.md / 26-UI-SPEC.md drifting from the locked U1–U4 decisions | medium | mitigate | Per-item positive and negative greps in Tasks 1–3 check every U1–U4 doc impact and confirm the superseded wording is gone |
| T-moj-02 | Tampering | Collateral loss of the Q1–Q10 grill amendments or the Checker Sign-Off verdicts | medium | mitigate | Tasks 2 and 3 re-grep the grill markers and compare the dimension and approval lines byte-for-byte against `git show HEAD`; Task 1 caps the CONTEXT diff at four lines |
| T-moj-03 | Tampering | Accidental edits outside `.planning/` (for example implementing U2 in `web/app/app.css` early) | low | mitigate | Task 3 verify fails if any path outside `.planning/` is modified |
</threat_model>

<verification>
- TASK1_OK, TASK2_OK and TASK3_OK all print.
- Doc-impact coverage, cell by cell against DECISIONS.md:
  - U1, UI-SPEC: inventory row (T2 item 3), Bulk remove flow (T2 items 8, 9, 10), Focus management table and fallback-chain paragraph (T2 items 11, 12), Bulk add tag and Preferences success steps (T2 items 6, 7), scope bullet (T2 item 2).
  - U1, CONTEXT: D-16 sentence (T1 item 4).
  - U2, UI-SPEC: Spacing "Page bottom clearance" exception (T2 item 4), Layout section (T2 item 5), UI Considerations row (T2 item 13), UAT item (T2 item 14).
  - U3, UI-SPEC: Spacing `sm` gap and inset rows plus the 8px inset below `sm` (T3 items 1–3), Control sizes (T3 item 4), Wrapper classes (T3 items 5, 6), Phone block (T3 item 7), Inline mode paragraph (T3 item 8), Clear copy row (T3 item 9), B3 populated row (T3 item 10), B3 375px backstop row (T3 item 11).
  - U3, CONTEXT: D-10 (T1 item 2).
  - U4, UI-SPEC: Bulk remove tag flow (T3 item 13), Toasts table (T3 items 14–16), Deliberately NOT used bullet (T3 item 12), UI Considerations B8 row (T3 item 17), Data Contract note (T3 item 18).
  - U4, CONTEXT: D-15 (T1 item 3).
  - Header pointers: UI-SPEC blockquote (T2 item 1), CONTEXT amendments line (T1 item 1).
  - Checker Sign-Off verdicts are unchanged (T2 and T3 byte compare). The tally is kept consistent per commit (T2 item 13 → 58/41, T3 item 19 → 59/42).
- `git log` shows three `docs(quick-261011-moj): ...` commits with no attribution trailers, each touching only its task's file.
- No Go or web gates are needed: no source file changes.
</verification>

<success_criteria>
- 26-UI-SPEC.md and 26-CONTEXT.md reflect every U1–U4 doc impact in DECISIONS.md, amended in place.
- Both docs point at `261011-moj-DECISIONS.md` next to the existing grill-amendment note, and the Q1–Q10 wording is untouched.
- UI-SPEC `status: approved` and the Checker Sign-Off verdicts are unchanged. The UI Considerations tally matches the rows (59 total: 42 covered, 7 backstop, 10 dismissed).
- `web/app/app.css` and every other file outside `.planning/` are untouched.
- Committed as docs-only, with no AI attribution trailer.
</success_criteria>

<output>
Create `.planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-SUMMARY.md` when done
</output>
