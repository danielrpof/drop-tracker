---
status: testing
phase: 24-artist-tags-notes
source: [24-VERIFICATION.md]
started: 2026-10-05T16:00:54Z
updated: 2026-10-05T16:00:54Z
---

## Current Test

number: 1
name: End-to-end tags + note CRUD in a real browser through the go:embed build
expected: |
  Create/pick/remove chips; reload; remove + re-add the artist and tags return; add/edit/clear a note and reload; delete a tag in Manage tags and '+ tag' no longer offers it as existing
awaiting: user response

## Tests

### 1. End-to-end tags + note CRUD in a real browser through the go:embed build
expected: Create/pick/remove chips; reload; remove + re-add the artist and tags return; add/edit/clear a note and reload; delete a tag in Manage tags and '+ tag' no longer offers it as existing
result: [pending]

### 2. Slow-network (DevTools Slow 3G) stale-vocabulary races
expected: (a) Open '+ tag', then Manage tags, delete a tag before the first GET /tags returns. (b) Open Manage tags, close it, reopen it, then delete or rename a tag and close. In both cases '+ tag' offers only Create for the deleted or old name, never an existing option
result: [pending]

### 3. Combobox popup overflow: 30+ tags, a 32-char name, narrow viewport
expected: Popup scrolls inside its own bounds; no clipped option
result: [pending]

### 4. Long tag chip at 375px
expected: Chip truncates; title and the x aria-label carry the full name
result: [pending]

### 5. Merge confirm title wrap at 375px with two 32-char names
expected: Title wraps, does not truncate
result: [pending]

### 6. Manage tags row truncation at 375px
expected: '· {n} artists' stays visible
result: [pending]

### 7. UI-SPEC contrast/hit-area audit of chips, x, '+ tag', note pencil
expected: Meets UI-SPEC contrast and 44px-equivalent hit areas
result: [pending]

### 8. Delete-vs-attach race
expected: No artist_tags row references a deleted tag (FK cascade guarantees this structurally); confirm or accept
result: [pending]

## Summary

total: 8
passed: 0
issues: 0
pending: 8
skipped: 0
blocked: 0

## Gaps
