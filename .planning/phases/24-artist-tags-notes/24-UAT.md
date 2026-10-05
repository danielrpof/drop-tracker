---
status: complete
phase: 24-artist-tags-notes
source: [24-VERIFICATION.md]
started: 2026-10-05T16:00:54Z
updated: 2026-10-05T17:05:00Z
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

## Summary

total: 8
passed: 8
issues: 0
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
