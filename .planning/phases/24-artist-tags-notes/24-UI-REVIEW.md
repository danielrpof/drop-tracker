# Phase 24 — UI Review

**Audited:** 2026-10-05
**Baseline:** 24-UI-SPEC.md (approved)
**Screenshots:** not captured. The app was running on :8080, but Playwright is not installed (`npx --no-install` refused), so this is a code-only audit. Human UAT (24-UAT.md, 8/8) covered the 375px layout, contrast and hit areas.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Locked strings are verbatim. The repeated "more"/"less" buttons have no artist context in their names. |
| 2. Visuals | 3/4 | The chip treatment matches [R1]. Pending chip × has no visual disabled state, and the pencil is foreground-coloured while its sibling triggers are muted. |
| 3. Color | 3/4 | Accent is limited to the declared primaries. The inline note error is still `text-destructive` on card, about 3.7:1. |
| 4. Typography | 3/4 | 4 sizes and 2 weights hold, but the textarea forces 14px at every width, which drops the spec's 16px-below-md rule. |
| 5. Spacing | 3/4 | Spacing is on the token scale. The "more"/"less" control has no 24px minimum, and the rename row uses `gap-2` instead of the normal row's `gap-4`. |
| 6. Experience Design | 2/4 | The "less" control can unmount itself once a note is expanded. The Manage tags dialog does not move initial focus to row 1's Rename. |

**Overall: 17/24**

---

## Top 3 Priority Fixes

1. **BLOCKER: "less" disappears after expanding a note.** `ArtistNote.tsx:89-102` measures `scrollHeight > clientHeight` from a ResizeObserver. Expanding removes `line-clamp-2`, the `<p>` grows, the observer fires, `overflow` becomes false, and the toggle (`:252`) unmounts. The user cannot collapse the note, and keyboard focus on the toggle is lost to `<body>`. **Fix:** latch overflow while expanded (`setOverflow(expanded || el.scrollHeight > el.clientHeight)`), or measure against a clamped clone. Add a test that stubs the grown height after expand.
2. **WARNING: Manage tags opens with focus on the close ×, not row 1's Rename** (focus table, row d). The dialog mounts in `loading`, so the only tabbable is the close button, which is last in DOM order (`ui/dialog.tsx:60`). Nothing moves focus after the load resolves (`ManageTagsDialog.tsx:95-113`). **Fix:** after the first successful load in an open session, focus `listRef` → first `li button`, or pass `initialFocus` and refocus on `status → loaded`.
3. **WARNING: The note textarea is 14px on mobile** (`ArtistNote.tsx:179`, `text-label md:text-label`). The spec keeps the Textarea's built-in `text-base md:text-sm` below md, and 16px is also what prevents iOS Safari's zoom-on-focus. **Fix:** use `md:text-label` only and let the base stay `text-base`.

Further recommendations:

4. **Pending chip × looks interactive but is not** (`TagChips.tsx:233-241`). `aria-disabled="true"` gets none of `disabled:`'s opacity or cursor styling, so it hovers like a live ×. **Fix:** add `aria-disabled:opacity-50 aria-disabled:pointer-events-none`, or a `cursor-not-allowed` class. Keep the element focusable.
5. **The note error is below 4.5:1** (`ArtistNote.tsx:215`, `text-destructive` on `--card`, about 3.7:1). The spec flags this as a project-wide follow-up. It is still unmet for this phase, so the fix needs a tracked todo.
6. **"more"/"less" has neither a target size nor context** (`ArtistNote.tsx:253-261`). It is a bare inline `<button>` with a line box of about 21px, which falls below the phase's own 24px floor [R3]. It is also announced only as "more", "more", "more"… across cards. **Fix:** add `min-h-6`, plus an sr-only suffix such as "of note for {artist}" (or `aria-label`). That suffix is an addition to locked copy, so the spec needs an amendment.
7. **The pencil colour is inconsistent** (`ArtistNote.tsx:264-272`). The ghost button renders at foreground white, while "+ tag", "add note" and the chip × are `text-muted-foreground hover:text-foreground`. In a dense card the pencil becomes the brightest glyph. **Fix:** apply the same muted/hover classes.
8. **Rename-row spacing drifts** (`ManageTagsDialog.tsx:320`, `gap-2`) from the normal row's `gap-4` (`:368`), so the row's rhythm shifts when it toggles into edit mode. The change is minor and only needs a decision to match or to document it.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)
- Every locked string matches verbatim, with typographic quotes. Checked: `+ tag`/`max 10 tags`/`Tag name`/`Create “{q}”`/already-on/empty (`TagCombobox.tsx`, `TagChips.tsx:262-273`); note copy and both error lines (`ArtistNote.tsx:24-33`); dialog title/description/row/empty/error copy (`ManageTagsDialog.tsx:278-308`); both ConfirmDialog variants and every toast (`:194-268, 411-435`); and the status-region messages (`TagChips.tsx:118-122, 178-182`, `ArtistNote.tsx:143-147`).
- WARNING: the "more"/"less" accessible names are not unique per card (finding 6).
- There is no generic "Submit/OK" copy. Pluralization is centralized (`pluralize`).

### Pillar 2: Visuals (3/4)
- The chip markup matches the locked `Badge variant="secondary" h-6 … text-label font-normal` recipe, including the focus-within unwrap (`TagChips.tsx:196-216`). This makes it distinct from the event and status badge families.
- Every icon-only control has an `aria-label` (chip ×, pencil, remove artist). Icons are `aria-hidden`.
- Header hierarchy is correct: Manage tags is `secondary` with a leading `Tags` icon (`watchlist.tsx:327-330`).
- WARNING: the pending × has no visual disabled state (finding 4), and the pencil's colour is out of family (finding 7).

### Pillar 3: Color (3/4)
- Accent (`variant="default"`) appears only on note Save (`ArtistNote.tsx:204`), rename Save (`ManageTagsDialog.tsx:354`) and Merge tags (`:438`). This matches "Accent reserved for".
- Destructive is used only on the Delete tag action, the row Delete hover (`:391`) and the note error line. This matches the spec.
- There are no hardcoded hex values. The only arbitrary value is `ring-[3px]`, which mirrors the Button focus ring.
- WARNING: the note error contrast is about 3.7:1 (finding 5).

### Pillar 4: Typography (3/4)
- Sizes used: `text-display`, `text-heading`, `text-body`, `text-label`. Weights used: `font-normal`, `font-semibold`, plus Button's pre-existing `font-medium` exception. The chip overrides Badge's `text-xs font-medium` as required.
- Counters use `tabular-nums`. The note uses `whitespace-pre-line break-words`. Long names in ConfirmDialog use `wrap-anywhere` (G-24-5).
- WARNING: the textarea's mobile size deviates from the contract (fix 3).

### Pillar 5: Spacing (3/4)
- These match the spec: the chip row `mt-1 flex-wrap gap-2`, the note block `mt-1`, the dialog list `max-h-[min(60vh,28rem)] overflow-y-auto overscroll-contain -mx-2 px-2`, rows `gap-4 py-2`, Save `min-w-20`, combobox `h-8 max-w-60`, and textarea `max-h-40`. There is no `overflow-hidden` on the chip row ([R4] honoured).
- WARNING: "more"/"less" is below the 24px floor, and the rename row uses `gap-2` (findings 6 and 8).

### Pillar 6: Experience Design (2/4)
- Strengths:
  - Chip add/remove is optimistic and rolls back in place.
  - The 10-tag cap swaps the trailing slot.
  - A failed note save keeps the text in a `readOnly` textarea, uses `aria-busy`, and shows a live error.
  - Rename collisions route through the merge confirm.
  - The nested AlertDialog keeps the parent dialog open.
  - Generation guards stop a stale vocabulary load from overwriting results (`ManageTagsDialog.tsx:92-120`).
  - Loading, error and empty states all exist inside the dialog.
- BLOCKER: the "less" toggle self-unmounts (fix 1).
- WARNING: the dialog's initial focus misses its contract (fix 2).
- Note: the ConfirmDialog closes on failure in its `finally` block (`ConfirmDialog.tsx:56-58`). That matches the spec, but after a merge failure, focus returns to the rename Save. Verify that this is acceptable.

Registry audit: shadcn is initialized, and the UI-SPEC lists only official `base-maia` blocks (dialog, alert-dialog, combobox, input-group, textarea). There are 0 third-party blocks, so there are no flags.

---

## Files Audited
- web/app/components/watchlist/TagChips.tsx
- web/app/components/watchlist/TagCombobox.tsx
- web/app/components/watchlist/ArtistNote.tsx
- web/app/components/watchlist/ManageTagsDialog.tsx
- web/app/components/watchlist/WatchlistRow.tsx
- web/app/components/common/ConfirmDialog.tsx
- web/app/components/ui/dialog.tsx
- web/app/routes/watchlist.tsx (header, status region)
