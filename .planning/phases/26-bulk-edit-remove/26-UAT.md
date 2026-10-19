---
status: partial
phase: 26-bulk-edit-remove
source: [26-VERIFICATION.md]
started: 2026-10-17T00:17:48Z
updated: 2026-10-19T00:00:00Z
---

## Current Test

[testing paused — 2 items outstanding]

## Tests

### 1. Bulk remove confirmation with long names: select three artists whose names are 60+ unbroken characters and open Remove (26-03 backstop).
expected: The description wraps all three names inside the dialog with no overflow, and the notes/tags/pending-alert/seed-mode sentence is fully visible.
result: pass

### 2. Shift+click range through the 44px label: click one card checkbox, Shift+click a card three rows down on the label area, then repeat after changing sort (26-04 backstop).
expected: The inclusive range in visible order is selected. After the sort change, the anchor is reset and only the clicked card toggles.
result: pass
fixed: "shift+click fixed (debug shift-click-range-select); user confirmed 2026-10-19"
reported: "part A fail, shift clicking only selects that box and not the boxes between. part B fail, the previous card before changing the sort is still selected as well"
severity: major
note: "Part B's retained prior selection is by design (selection persists across view changes; only the anchor resets). Part B cannot be meaningfully confirmed until Part A ranges work."

### 3. Bar layout and clearance at 1280px and 375px with 30+ artists: scroll to the bottom, Tab through card checkboxes, trigger a failure toast, and select every artist so the count reaches 3-4 digits (26-06 backstop).
expected: The last card clears the bar, a focused checkbox is never behind the bar, toasts stack above the bar at both widths, and at 375px the bar is one row of icon-only 44px controls with no horizontal scroll and `1000 selected` fully visible.
result: pass

### 4. Bulk add-tag long text: select three long-named artists, one of them at 10 tags, and bulk-add a 32-character tag at 1280px and 375px (26-08 backstops).
expected: The input scrolls the long name inside itself, the counter never pushes Cancel off the bar, and the warning toast wraps the names and the tag with no clipping while naming the at-cap artist.
result: pass

### 5. Preferences popover at 375px: select three artists with different release types, open Preferences, leave one mixed type untouched, Apply, then reopen (26-10 backstop).
expected: The popover fits on screen. The mixed type shows a dash with aria-checked=mixed, and the network panel shows one PATCH whose body omits the untouched type.
result: pass

### 6. Remove-tag picker long text: give two selected artists a 32-character unbroken tag, open Remove tag at 1280px and 375px, and arrow through a 20-tag list (26-11 backstop).
expected: The tag name truncates with an ellipsis, `{c} of {N}` stays fully visible, the list caps at about 288px with the highlighted option in view, and the inline row wraps to a second line at 375px.
result: pass

### 7. End-to-end UAT walk of the 26-UI-SPEC 'Verification items to carry into UAT' list against the built binary (26-12): one request per bulk action, select-all under a filter, hidden-selection counts, cap-skip naming, indeterminate preferences, remove-confirm copy, toasts above the bar, focus never behind the bar, 3:1 contrast on unchecked and indeterminate boxes, and Undo on a bulk remove-tag toast.
expected: Every item behaves as UI-SPEC and 26-VALIDATION.md 'Manual-Only Verifications' describe.
result: pass

### 8. Unverified test-tier prohibition (26-01, BULK-04): 'MUST NOT leave a partial removal'. No committed test injects a mid-statement failure into BulkDeleteWatchlist.
expected: Accept the structural guarantee (one DELETE statement, no watchlist FKs) plus this verifier's rolled-back failure-injection spot-check, or add a test that forces the DELETE to fail and asserts every row survives.
result: pass
note: accepted structural guarantee (single DELETE, no watchlist FKs) plus verifier spot-check

### 9. Confirm `go test -race ./...` is green in CI for this branch.
expected: The CI integration job passes with -race. It could not run locally because cgo is unavailable on this Windows host.
result: blocked
blocked_by: third-party
reason: branch not pushed; needs a push to run CI

## Summary

total: 9
passed: 8
issues: 0
pending: 0
skipped: 0
blocked: 1

## Gaps

- truth: "Shift+click selects the inclusive range in visible order; after a sort change the anchor resets and only the clicked card toggles"
  status: resolved
  reason: "User reported: part A fail, shift clicking only selects that box and not the boxes between. part B fail, the previous card before changing the sort is still selected as well (retained selection is by design; unconfirmable until part A works)"
  severity: major
  test: 2
  root_cause: "onKeyDownCapture in WatchlistRow cleared shiftRef on every keydown, including auto-repeat of the held Shift key, before the checkbox change handler read it"
  artifacts: ["web/app/components/watchlist/WatchlistRow.tsx", "web/app/routes/watchlist.bulk.test.tsx"]
  missing: []
  debug_session: ".planning/debug/resolved/shift-click-range-select.md"
