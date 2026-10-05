---
status: complete
phase: 24-artist-tags-notes
source: [24-VERIFICATION.md]
started: 2026-10-05T16:00:54Z
updated: 2026-10-05T16:34:55Z
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
result: issue
reported: "Chip does not truncate, it shows the full tag name at 375px"
severity: major

### 5. Merge confirm title wrap at 375px with two 32-char names
expected: Title wraps, does not truncate
result: issue
reported: "the text overdlows sideways"
severity: major

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
passed: 6
issues: 2
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-24-4
  truth: "Long tag chip truncates at 375px; title and the x aria-label carry the full name"
  status: failed
  reason: "User reported: Chip does not truncate, it shows the full tag name at 375px"
  severity: major
  test: 4
  artifacts: []
  missing: []

- gap_id: G-24-5
  truth: "Merge confirm title wraps at 375px with two 32-char names, does not truncate or overflow"
  status: failed
  reason: "User reported: the text overdlows sideways"
  severity: major
  test: 5
  artifacts: []
  missing: []
