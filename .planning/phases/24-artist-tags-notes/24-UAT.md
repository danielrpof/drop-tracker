---
status: diagnosed
phase: 24-artist-tags-notes
source: [24-VERIFICATION.md]
started: 2026-10-05T16:00:54Z
updated: 2026-10-05T18:30:00Z
---

## Current Test

[testing complete]

## Tests

### 1. End-to-end tags + note CRUD in a real browser through the go:embed build
expected: Create/pick/remove chips; reload; remove + re-add the artist and tags return; add/edit/clear a note and reload; delete a tag in Manage tags and '+ tag' no longer offers it as existing
result: pass

### 2. Slow-network (DevTools Slow 3G) stale-vocabulary races
expected: (a) Open '+ tag', then Manage tags, delete a tag before the first GET /tags returns. (b) Open Manage tags, close it, reopen it, then delete or rename a tag and close. In both cases '+ tag' offers only Create for the deleted or old name, never an existing option
result: pass

### 3. Combobox popup overflow: 30+ tags, a 32-char name, narrow viewport
expected: Popup scrolls inside its own bounds; no clipped option
result: pass

### 4. Long tag chip at 375px
expected: Chip truncates; title and the x aria-label carry the full name
result: pass
note: "Initial report (no truncation with lowercase 32-char name) was a fixture issue; re-test with 32 W glyphs truncates as specified."

### 5. Merge confirm title wrap at 375px with two 32-char names
expected: Title wraps, does not truncate
result: pass
note: "Initial report (title overflowed sideways) fixed by 24-13 wrap-anywhere; re-test passes."

### 6. Manage tags row truncation at 375px
expected: '· {n} artists' stays visible
result: pass

### 7. UI-SPEC contrast/hit-area audit of chips, x, '+ tag', note pencil
expected: Meets UI-SPEC contrast and 44px-equivalent hit areas
result: pass

### 8. Delete-vs-attach race
expected: No artist_tags row references a deleted tag (FK cascade guarantees this structurally); confirm or accept
result: pass

### 9. Expand and collapse a long note
expected: "more" expands a clamped note; "less" stays visible and collapses it again; focus stays on the toggle
result: issue
reported: "UI audit (24-UI-REVIEW.md fix 1, confirmed in code): after expanding, the more/less button disappears, so the note can't be collapsed and focus drops to body"
severity: blocker
source: ui-review

### 10. Manage tags initial focus
expected: Opening Manage tags focuses row 1's Rename once the list loads (UI-SPEC focus contract)
result: issue
reported: "UI audit (24-UI-REVIEW.md fix 2): dialog mounts during loading, focus lands on close x and is never moved once rows load"
severity: minor
source: ui-review

### 11. Note textarea font size on mobile
expected: Note editor textarea is 16px below md (no iOS focus zoom), 14px from md up
result: issue
reported: "UI audit (24-UI-REVIEW.md fix 3): className 'text-label md:text-label' forces 14px on mobile"
severity: minor
source: ui-review

## Summary

total: 11
passed: 8
issues: 3
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-24-4
  truth: "Long tag chip truncates at 375px; title and the x aria-label carry the full name"
  status: resolved
  resolved_by: retest (not a defect)
  resolved_at: 2026-10-05
  reason: "User reported: Chip does not truncate, it shows the full tag name at 375px"
  severity: major
  test: 4
  root_cause: "Likely not a defect: below sm the row is flex-col, so the chip gets ~311px; 32 narrow lowercase chars at 14px (~240px) plus the x fit without truncating. The truncation chain (Badge max-w-full, span min-w-0 truncate, parent min-w-0 flex-1) is intact. Re-test with 32 wide glyphs (W) to exercise truncation."
  artifacts:
    - path: "web/app/components/watchlist/TagChips.tsx"
      issue: "none confirmed; truncation only triggers when the chip exceeds the column width"
  missing:
    - "Re-test with a 32-char wide-glyph tag (WWWW...) at 375px"

- gap_id: G-24-5
  truth: "Merge confirm title wraps at 375px with two 32-char names, does not truncate or overflow"
  status: resolved
  resolved_by: 24-13-PLAN.md
  resolved_at: 2026-10-05
  reason: "User reported: the text overdlows sideways"
  severity: major
  test: 5
  root_cause: "AlertDialogTitle/Description in ConfirmDialog have no overflow-wrap rule, so a 32-char tag name with no spaces is one unbreakable token wider than the ~300px dialog content at 375px and overflows sideways."
  artifacts:
    - path: "web/app/components/common/ConfirmDialog.tsx"
      issue: "title and description lack break-words / overflow-wrap:anywhere"
  missing:
    - "Add overflow-wrap:anywhere (Tailwind wrap-anywhere or [overflow-wrap:anywhere]) to AlertDialogTitle and AlertDialogDescription in ConfirmDialog"
    - "Unit test asserting the wrap class on title/description"

- gap_id: G-24-9
  truth: "An expanded note keeps its 'less' toggle and can be collapsed again; focus stays on the toggle"
  status: failed
  reason: "UI audit: more/less button disappears after expanding a note"
  severity: blocker
  test: 9
  root_cause: "ArtistNote's useLayoutEffect measures overflow as scrollHeight > clientHeight via ResizeObserver. Expanding removes line-clamp-2, the paragraph grows to full height, the observer re-measures, overflow becomes false, and the {overflow && <button>} toggle unmounts (focus falls to body)."
  artifacts:
    - path: "web/app/components/watchlist/ArtistNote.tsx"
      issue: "lines 89-102 overflow measure ignores expanded state; line 252 gates the toggle on overflow"
  missing:
    - "Keep overflow true while expanded (only re-measure when collapsed / clamped), so the 'less' toggle stays mounted"
    - "Component test: stub overflow, click more, simulate a re-measure with no overflow, assert 'less' still rendered and focused, click collapses"

- gap_id: G-24-10
  truth: "Opening Manage tags moves focus to row 1's Rename once the list loads"
  status: failed
  reason: "UI audit: initial focus stays on close x"
  severity: minor
  test: 10
  root_cause: "ManageTagsDialog mounts while status is loading, so the dialog's initial focus lands on the close button; nothing moves focus when status becomes loaded."
  artifacts:
    - path: "web/app/components/watchlist/ManageTagsDialog.tsx"
      issue: "lines 95-113 load path has no focus hand-off on loaded"
  missing:
    - "On the first transition to loaded within an open, focus row 1's Rename button (only if focus is still on the dialog/close, never stealing from a user action)"
    - "Component test asserting focus moves to the first Rename after load"

- gap_id: G-24-11
  truth: "Note textarea is 16px below md and 14px from md up"
  status: failed
  reason: "UI audit: 'text-label md:text-label' forces 14px on mobile"
  severity: minor
  test: 11
  root_cause: "ArtistNote.tsx:179 has an unprefixed text-label that overrides the 16px base below md."
  artifacts:
    - path: "web/app/components/watchlist/ArtistNote.tsx"
      issue: "line 179 className"
  missing:
    - "Drop the unprefixed text-label so only md:text-label remains"
    - "Rebuild embedded SPA (internal/webassets/build/client) after all fixes"
