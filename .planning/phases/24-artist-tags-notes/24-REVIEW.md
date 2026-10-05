---
phase: 24-artist-tags-notes
reviewed: 2026-10-05T00:00:00Z
depth: standard
review_kind: incremental
diff_base: c61be5075b83d5ad154207d542e54ccf18d4cf27
files_reviewed: 3
files_reviewed_list:
  - web/app/components/common/ConfirmDialog.test.tsx
  - web/app/components/common/ConfirmDialog.tsx
  - web/app/components/watchlist/ManageTagsDialog.test.tsx
findings:
  critical: 0
  warning: 0
  info: 3
  total: 3
status: issues_found
---

# Phase 24: Code Review Report (incremental re-review, gap closure 24-13 / G-24-5)

**Reviewed:** 2026-10-05T00:00:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

> This report replaces the previous 24-REVIEW.md (24-12 increment, diff_base `a689ab7`) for the **24-13 increment only**: `c61be50..HEAD` on the three files listed above. Findings from earlier increments are not restated here.

## Summary

Plan 24-13 adds `className="wrap-anywhere"` to `AlertDialogTitle` and `AlertDialogDescription` inside the shared `ConfirmDialog`, with a 2-line rationale comment. It also adds one unit test and one ManageTagsDialog integration test (two 32-char unspaced tag names in a merge confirm).

What I checked:
- **The fix works in the shipped CSS.** The rebuilt embedded bundle `internal/webassets/build/client/assets/root-Dg0NMU0m.css` contains `.wrap-anywhere{overflow-wrap:anywhere}`. Tailwind is locked at 4.3.0 in `pnpm-lock.yaml`.
- **`cn`/tailwind-merge keeps the class.** Its group (overflow-wrap) is separate from the primitive's `text-balance`/`md:text-pretty` (text-wrap), and the new `toHaveClass` assertions pass against the real `cn` merge.
- **The layout reasoning is sound.** `AlertDialogHeader` is a `place-items-center` grid with an implicit `auto` column. Its items get `min-width:auto`, so a long unbreakable run would set the min-content width. `overflow-wrap:anywhere` is the only value that lowers that min-content contribution (`break-word` does not). `AlertDialogContent` is `w-full max-w-xs/sm:max-w-md`, which gives the header a definite width. Because the title and description are fit-content inside that width, ordinary words are not broken early.
- **Both ConfirmDialog call sites are covered**, since the fix is in the shared component. These are the delete confirm (`Delete “{name}” from N artists?`) and the merge confirm (`ManageTagsDialog.tsx:406-441`). No other consumers exist.
- **Toasts with the same names already wrap.** Sonner's stylesheet sets `overflow-wrap: anywhere`, so they need no change.
- **Tests and formatting pass.** Both test files pass (27/27, run with `--coverage.enabled=false` because the subset trips the global threshold). Prettier `--check` is clean on all three files. `mockReset: true` in `vitest.config.ts` isolates the new tests' `mockResolvedValue` setup from the other tests.

No correctness, security, or data-loss defects in this increment. The three Info items are about how strong the tests are and latent dependency drift.

## Info

### IN-01: The unit test claims long-name coverage but uses a short title, and the `truncate` negation can never fail

**File:** `web/app/components/common/ConfirmDialog.test.tsx:52-63` (also `web/app/components/watchlist/ManageTagsDialog.test.tsx:393`)
**Issue:** The new unit test is named "...so a long unspaced tag name wraps instead of overflowing", but it renders the shared fixture title `Delete “rap”?`, which has no long unspaced run. It only proves the class is present. Separately, `expect(title).not.toHaveClass("truncate")` (in both new tests) can never fail. Neither `ConfirmDialog` nor the vendored `AlertDialogTitle` (`web/app/components/ui/alert-dialog.tsx`) has ever applied `truncate`, so the assertion guards nothing.
**Fix:** Rename the unit test to describe what it checks (for example, "title and description carry wrap-anywhere"), or render it with a 32-char unspaced title. Replace the `truncate` negation with something that would catch a real regression to clipping, e.g. `expect(title).not.toHaveClass("truncate", "whitespace-nowrap", "overflow-hidden")`. Otherwise drop it.

### IN-02: The fix depends on Tailwind 4.1+, but the declared range allows 4.0.x, where the utility is silently missing

**File:** `web/package.json:44`
**Issue:** Tailwind added `wrap-anywhere` in v4.1. `"tailwindcss": "^4"` still allows 4.0.x, and there the class compiles to nothing with no build error. The regression guard checks only the class name, so the tests would stay green while G-24-5 reappears in the UI. The lockfile pins 4.3.0, so this is only a risk if the lockfile is regenerated against a constrained registry or downgraded. It is latent, not active.
**Fix:** Raise the floor to match what the code needs: `"tailwindcss": "^4.1"` (and `"@tailwindcss/vite": "^4.1"`).

### IN-03: The ConfirmDialog header comment is still 9 lines, against the 1-3 line rule (not touched in this increment)

**File:** `web/app/components/common/ConfirmDialog.tsx:26-34`
**Issue:** The new 2-line JSX comment at lines 71-72 follows `.claude/CLAUDE.md` comment discipline. The component's header block above it is 9 lines that re-explain focus order, pending UI, and toast ownership. That is the "multi-paragraph header" anti-pattern the project rules name. It predates 24-13 and is listed here only because the file was in scope.
**Fix:** Cut it down to intent plus one design-doc reference, e.g. `// D-16 shared confirm (merge/delete/bulk remove). Cancel is first in DOM so it takes initial focus; the caller owns toasts.`

---

_Reviewed: 2026-10-05T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
