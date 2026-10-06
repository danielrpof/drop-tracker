---
phase: 24-artist-tags-notes
reviewed: 2026-10-06T19:58:38Z
depth: standard
review_kind: incremental
diff_base: 7d27a2db1914bf83723ce5fbdabcf4161c3e93aa
files_reviewed: 4
files_reviewed_list:
  - web/app/components/watchlist/ArtistNote.test.tsx
  - web/app/components/watchlist/ArtistNote.tsx
  - web/app/components/watchlist/ManageTagsDialog.test.tsx
  - web/app/components/watchlist/ManageTagsDialog.tsx
findings:
  critical: 0
  warning: 1
  info: 4
  total: 5
status: issues_found
---

# Phase 24: Code Review Report (incremental re-review, gap closure 24-14 / G-24-9..11)

**Reviewed:** 2026-10-06T19:58:38Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

> This report replaces the previous 24-REVIEW.md (24-13 increment) and covers only the **24-14 increment**: `7d27a2d..HEAD` on the four files listed above. It does not repeat findings from earlier increments. The embedded bundle under `internal/webassets/build/client` was out of scope. It was only grepped, read-only, to check CSS rule order for IN-01.

## Summary

The three fixes do what the plan says:

- **G-24-9 (ArtistNote overflow latch).** Adding `expanded` to the deps with an early return disconnects the ResizeObserver synchronously in the commit that removes `line-clamp-2`. The grown paragraph is never measured, so the toggle stays mounted and focused. Collapsing re-measures in the layout effect before paint. I traced the RED-to-GREEN path of the new fake-observer test and confirmed it really exercises the bug.
- **G-24-10 (Manage tags hand-off).** The pending flag is armed by `open` and consumed only by a successful load. I traced it against base-ui's `FloatingFocusManager` initial-focus path (`queueMicrotask` -> `enqueueFocus` rAF -> `shouldFocus`). If the load resolves before the queued focus fires, `focusMovedInside` makes base-ui skip, and the hand-off wins. Retry after an error unmounts the Retry button, so focus falls to the body or the popup, and both are allowed. Close-while-loading disarms the flag before the stale load lands. Reopen re-arms it, and `load()` always moves status through `loading`, so the `[status]` effect fires again. The guard test is not vacuous: after the failed delete, focus is on the body, so a re-arming implementation would fail it.
- **G-24-11 (textarea size).** The bare `text-label` is gone, and `text-base` now applies below md.

One real defect remains. The hand-off finds its target with a positional `li button` selector, but rename mode survives closing the dialog. On reopen, the selector can land on a stale rename row's **Cancel** button instead of a Rename button (WR-01).

The Info items cover:

- `md:text-label` is shadowed in the emitted CSS (IN-01).
- A tailwind-merge misclassification in the same file. It predates this diff and already drops `text-label` from the "add note" button (IN-02).
- Test hygiene and coverage gaps (IN-03, IN-04).

## Warnings

### WR-01: Initial-focus hand-off can land on a stale rename row's Cancel, because rename state survives close/reopen

**File:** `web/app/components/watchlist/ManageTagsDialog.tsx:150` (selector), `:123-127` (open effect), `:81-82` (state)
**Issue:** `ManageTagsDialog` is always mounted (`web/app/routes/watchlist.tsx:337`), so its state outlives each open. Nothing resets `renameTarget` or `renameValue` when the dialog closes:

- `onOpenChange` is passed straight to `<Dialog>`.
- The `[open]` effect only arms the flag and calls `load()`.

A user can close the dialog while rename mode is open, by clicking the x or pressing outside the popup. (Esc in the input is `stopPropagation`'d, so that path is safe.) On reopen, the first load renders that tag's row in rename mode again, with the old typed text. The new hand-off then picks its target with

```ts
const target = list?.querySelector<HTMLButtonElement>("li button")
```

This assumes the first button in row 1 is Rename. When row 1 is the stale rename row, its first button is **Cancel**: the `<Input>` is not a button. Focus lands on Cancel, contradicting UI-SPEC focus row (d) and the hand-off's own comment ("hands focus to row 1's Rename"). The rename input is not focused or selected either, because the `[renameTarget]` effect does not re-run when the value has not changed. When the stale row is row k > 1, row 1's Rename gets focus while row k sits in rename mode with text from an earlier session.

The stale state predates this diff. The new selector turns it into a misdirected programmatic focus move.

**Fix:** Reset per-session UI state when the dialog opens, and target Rename explicitly instead of by position:

```tsx
useEffect(() => {
  initialFocusPendingRef.current = open
  if (open) {
    setRenameTarget(null)
    setRenameValue("")
    cancelFocusIndexRef.current = null
    load()
  }
  // eslint-disable-next-line react-hooks/exhaustive-deps
}, [open])
```

Mark the Rename button, for example `data-rename-trigger`, and select it with `list?.querySelector<HTMLButtonElement>("li [data-rename-trigger]")`. Add a test: open, click Rename on row 1, close with `setOpen(false)`, reopen, resolve the load, then assert that "Rename tag X" has focus and that no rename input is rendered.

## Info

### IN-01: `md:text-label` on the note textarea is shadowed by the Textarea's own `md:text-sm`

**File:** `web/app/components/watchlist/ArtistNote.tsx:179` and `ArtistNote.test.tsx:69-82`
**Issue:**

- tailwind-merge (unconfigured, `~/lib/utils.ts`) treats `text-label` as a text colour, so `cn()` keeps both `md:text-sm` (vendored base) and `md:text-label`.
- In the emitted CSS (`Root-*.css`), the `@media (width>=48rem)` block emits `.md\:text-label{...}` **before** `.md\:text-sm{...}`, at equal specificity. `md:text-sm` therefore wins `font-size` and `line-height`.

The size is 14px either way, so the G-24-11 goal (16px below md) holds. But from md up the line-height is text-sm's 1.25rem, not the label token's 1.5. `md:text-label` contributes only `font-weight: 400`. The G-24-11 test asserts that `md:text-label` is present, which suggests the label token applies when it does not. The plan's flagged assumption anticipated this ("whichever rule the CSS order lets win"). Recording the actual winner here.
**Fix:** Register the custom size tokens with tailwind-merge (see IN-02), so that `md:text-label` replaces `md:text-sm` during merging. Alternatively, drop `md:text-label` and rely on the base `md:text-sm`, and change the test to assert that the bare `text-label` is absent and `text-base` is present.

### IN-02: Outside the diff, the "add note" button already loses its `text-label` size to the same tailwind-merge misclassification

**File:** `web/app/components/watchlist/ArtistNote.tsx:229`
**Issue:** `className="h-6 text-label text-muted-foreground hover:text-foreground"` passes through `Button`'s `cn()`. tailwind-merge treats `text-label` and `text-muted-foreground` as the same "text colour" group and keeps only the last one. Verified locally:

```
twMerge(<xs variant>, "h-6 text-label text-muted-foreground ...")
  -> "... text-xs h-6 text-muted-foreground hover:text-foreground"
```

`text-label` is dropped, so "add note" renders at the xs variant's `text-xs` (12px) instead of the 14px Label size. The same hazard applies to any `cn()` call that combines a custom `text-{display,heading,body,label}` token with a colour. This line predates 24-14. I list it only because the G-24-11 work touched this exact class-merge path without catching it.
**Fix:** In `web/app/lib/utils.ts`, use `extendTailwindMerge({ extend: { classGroups: { "font-size": [{ text: ["display", "heading", "body", "label"] }] } } })`. Then check the other `cn()` call sites that pass these tokens.

### IN-03: The G-24-9 test leaks a class-dependent `clientHeight` getter on `HTMLParagraphElement.prototype` and does not check that the observer re-attaches

**File:** `web/app/components/watchlist/ArtistNote.test.tsx:400-410`
**Issue:**

- The test restores the `ResizeObserver` global with `onTestFinished`, but leaves its `scrollHeight` and `clientHeight` prototype overrides in place for the rest of the file. It follows the pattern of the existing test at 349-356, which also leaks. The next test re-stubs both properties, so the file passes today. Any later test that renders a note without stubbing would inherit "clamped means overflows" behaviour, depending on test order.
- The test also never asserts that collapsing re-attaches a ResizeObserver. A regression that left the observer disconnected after 'less', which would break the "re-measured on resize" part of the UI-SPEC contract, would still pass.

**Fix:** Save the original descriptors, or `delete` the own properties, in `onTestFinished`. After the final `userEvent.click(toggle)`, assert `expect(observers.size).toBe(1)`.

### IN-04: Several G-24-10 must-have branches have no test

**File:** `web/app/components/watchlist/ManageTagsDialog.test.tsx:126-182`
**Issue:** The plan's truths list behaviours that only the two new tests pin: the hand-off itself, and no re-arm after a failed-delete reload. These branches of the new effect have no test:

- An empty vocabulary keeps focus on Close.
- An error followed by Retry still hands off.
- A close and reopen re-arms the hand-off.
- Focus the user already moved inside the popup is left alone (`fromPopupChrome` false).

A refactor of the flag or the `fromPopupChrome` predicate could break any of these without a failing test.
**Fix:** Add three short tests, each using `deferred`:

1. Resolve `[]` and assert Close keeps focus.
2. Reject, click Retry, resolve with tags, and assert "Rename tag X" has focus.
3. Run `setOpen(false)` and then `setOpen(true)` after a first hand-off, and assert the hand-off happens again on the second load.

---

_Reviewed: 2026-10-06T19:58:38Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
