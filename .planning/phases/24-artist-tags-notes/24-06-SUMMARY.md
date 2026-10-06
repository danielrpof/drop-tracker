---
phase: 24-artist-tags-notes
plan: 06
subsystem: ui
tags: [react, base-ui-dialog, base-ui-alert-dialog, shadcn, tags, watchlist, accessibility]

requires:
  - phase: 24-02
    provides: "GET /tags, PATCH /tags/{id} (409 collision body), POST /tags/{id}/merge, DELETE /tags/{id}"
  - phase: 24-05
    provides: "TagActions.vocabulary/loadVocabulary/rememberTag seam, lib/tags.ts counter/length conventions"
provides:
  - "web/app/components/ui/dialog.tsx, alert-dialog.tsx (vendored shadcn base-maia)"
  - "web/app/components/common/ConfirmDialog.tsx: the one reusable confirm (D-16), Phase 26 reuses it"
  - "web/app/components/watchlist/ManageTagsDialog.tsx: list/rename/merge/delete surface opened from the Watchlist header"
  - "web/app/lib/api.ts: ApiError.body, deleteTag, renameTag/RenameTagResult, mergeTag"
  - "web/app/routes/watchlist.tsx: Manage tags button, dropTagFromEntries/renameTagInEntries/mergeTagInEntries"
affects: [24-07-artist-note, phase-26-bulk-remove]

actuals:
  tokens: 17019
  tasks: 3
  commits: 5
  plan_head_before: 7b37a656a838090495670b8b3f0628dff969d0a3

tech-stack:
  added: []
  patterns:
    - "CollisionTarget local state carries the server's 409 body (target + carrierCountAfterMerge) straight into the merge ConfirmDialog's copy -- the client never computes the post-merge count itself (D-09, D-22)"
    - "Two independent focus-return paths off one row list: a tags-array-keyed layout effect (FocusRequest ref) for delete/rename/merge success, and a renameTarget-keyed effect (cancelFocusIndexRef) for rename Cancel/Esc, since cancel never touches the tags array and so never fires the first effect"
    - "Both nested ConfirmDialogs (delete, merge) render as siblings of <Dialog> in the same React subtree rather than literal DOM/JSX children of DialogContent -- base-ui's nested-dialog registration and focus trap work from React-tree membership, not DOM nesting, confirmed by the four focus-table (e) tests"

key-files:
  created:
    - web/app/components/ui/dialog.tsx
    - web/app/components/ui/alert-dialog.tsx
    - web/app/components/common/ConfirmDialog.tsx
    - web/app/components/common/ConfirmDialog.test.tsx
    - web/app/components/watchlist/ManageTagsDialog.tsx
    - web/app/components/watchlist/ManageTagsDialog.test.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx

key-decisions:
  - "ApiError gained an optional body field (parsed non-2xx JSON) rather than a second error type, so renameTag's 409 -> collision mapping and any future structured-error consumer share the one ApiError shape apiFetch already throws everywhere."
  - "renameTag's collision detection is purely server-driven: only a 409 ApiError whose body carries both target and a numeric carrier_count_after_merge maps to {kind: 'collision'}; anything else rethrows unchanged (D-22 -- never a client pre-check)."
  - "mergeTagInEntries drops the source chip in place (rather than always swapping) when an entry already carries the target, so a merge never produces a duplicate chip on a card that held both tags."

patterns-established:
  - "ConfirmDialog (open/onOpenChange/title/description/actionLabel/pendingLabel/variant/onConfirm/finalFocus) is the one reusable confirm over AlertDialog -- Phase 26's bulk remove reuses it unchanged (D-16)."

requirements-completed: [TAG-05, TAG-06]

coverage:
  - id: D1
    description: "A secondary Manage tags button in the Watchlist header (every state) opens ManageTagsDialog, which re-fetches GET /tags on every open and lists every tag with its watched-only count, sorted case-insensitively with ties broken by id; zero-carrier tags render '· 0 artists' and are never hidden (D-07, D-08, D-11, D-12, D-30)."
    requirement: TAG-05
    verification:
      - kind: unit
        ref: "app/components/watchlist/ManageTagsDialog.test.tsx#shows three skeleton rows while loading / shows the error state with a Retry action that refetches / shows the empty-vocabulary state / renders rows sorted case-insensitively by name, with zero-artist rows shown (not hidden) and correct pluralization"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#opens Manage tags and deletes a tag carried by two entries, so both cards lose the chip with no extra listWatchlist call"
        status: pass
    human_judgment: false
  - id: D2
    description: "Delete opens a count-stating ConfirmDialog (n=0 gets its own copy); confirming removes the tag from the list and every card with no reload and toasts the server's carrier_count, with no undo (TAG-06, D-11, D-17)."
    requirement: TAG-06
    verification:
      - kind: unit
        ref: "app/components/watchlist/ManageTagsDialog.test.tsx#delete confirm calls deleteTag, toasts with the server's count, removes the row, and calls onDeleted / uses the n = 0 delete confirm copy for a zero-carrier tag"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#opens Manage tags and deletes a tag carried by two entries, so both cards lose the chip with no extra listWatchlist call"
        status: pass
    human_judgment: false
  - id: D3
    description: "Rename is inline (Rename button -> selected-text input -> Save/Cancel); a plain or case-only rename saves, toasts, updates the row and every card with no reload, and refocuses that row's Rename; Save is disabled for an empty or identical (casing included) name; Esc cancels only the rename; a rejected rename keeps the input open with its text (TAG-05, D-09, SC3)."
    requirement: TAG-05
    verification:
      - kind: unit
        ref: "app/components/watchlist/ManageTagsDialog.test.tsx#opens rename mode with the current name selected... / renames a tag on Enter... / starting a rename on another row cancels the first... / Esc in the rename input cancels the rename... / keeps a rejected rename's input open... / shows the {n}/32 counter..."
        status: pass
      - kind: unit
        ref: "app/lib/api.test.ts#ApiError.body: keeps the parsed JSON error body / renameTag PATCHes /tags/{id}... / renameTag maps a 409 collision body... / renameTag rethrows a non-collision failure"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#opens Manage tags and renames a tag carried by two entries, so both cards show the new name with no extra listWatchlist call"
        status: pass
    human_judgment: false
  - id: D4
    description: "A rename collision with a different tag opens a merge ConfirmDialog naming both tags and the server's post-merge count, with focus on Cancel; nothing merges without confirmation; a confirmed merge removes the source row, updates the target's count, updates every card (one chip if it carried both), and toasts the server's count; a rejected merge closes the confirm, toasts, and re-fetches (D-09, D-17, D-19, D-23)."
    requirement: TAG-05
    verification:
      - kind: unit
        ref: "app/lib/api.test.ts#mergeTag POSTs /tags/{id}/merge with {into} and the CSRF header, resolving the merge response"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ManageTagsDialog.test.tsx#a collision opens a merge ConfirmDialog naming both tags with focus on Cancel, without calling mergeTag / confirming the merge shows Merging..., disables both buttons, calls mergeTag by id, toasts, removes the source row, updates the target's count, and focuses the target's Rename / a rejected merge closes the ConfirmDialog, toasts the failure, and re-fetches the list"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#opens Manage tags, confirms a rename collision's merge, and every card carrying the source shows the target instead, with no extra listWatchlist call"
        status: pass
    human_judgment: false
  - id: D5
    description: "ConfirmDialog is a reusable component over AlertDialog with the specified nested-dialog focus behavior: Esc on it closes only itself (the parent Manage tags dialog stays open); Cancel/Esc restore focus to the control that opened it; Tab cannot move focus out of it while open; Esc in the rename input does not close the parent (D-16, UI-SPEC Focus management [Mechanism])."
    requirement: TAG-05
    verification:
      - kind: unit
        ref: "app/components/common/ConfirmDialog.test.tsx (renders copy, Cancel closes without onConfirm, pending label + disabled buttons, closes on reject)"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ManageTagsDialog.test.tsx#Cancel on the merge confirm returns to the still-open rename input with its text intact and focuses Save / Esc on the merge confirm closes only it, leaving Manage tags open and the rename input intact / focus cannot tab out of the open merge ConfirmDialog / Esc in the rename input cancels the rename, focuses that row's Rename, and leaves the dialog open"
        status: pass
    human_judgment: false
  - id: D6
    description: "[UI E7 overflow + long-text backstop] A merge title naming two 32-character tags wraps inside the ConfirmDialog at 375px width and is never truncated."
    verification: []
    human_judgment: true
    rationale: "Flagged verification: backstop in the plan's must_haves -- jsdom does not compute layout/wrap at a given viewport width, so no automated test can assert this. AlertDialogTitle carries no truncation class (no truncate/whitespace-nowrap), so the markup itself does not prevent wrapping, but the rendered result needs a human/visual check."
  - id: D7
    description: "[UI E5 long-text backstop] A 32-character tag name in a Manage tags row truncates with a title, while '· {n} artists' stays shrink-0 whitespace-nowrap and visible at 375px."
    verification: []
    human_judgment: true
    rationale: "Same backstop reason as D6 -- the row markup carries truncate/title on the name and shrink-0 whitespace-nowrap on the count (unchanged from Task 1), but jsdom cannot confirm the rendered truncation/visibility at a real viewport width."

duration: ~55min
completed: 2026-09-23
status: complete
---

# Phase 24 Plan 06: Manage Tags — Rename, Merge, Delete Summary

**A `ManageTagsDialog` opened from the Watchlist header lists the tag vocabulary with watched-only counts and lets the user delete, inline-rename, or — after an explicit ConfirmDialog naming both tags and the server's post-merge count — merge tags, updating every Watchlist card in place with no reload.**

## Performance

- **Duration:** ~55 min (this continuation session; Task 1 and Task 2 RED were completed and committed by a prior executor before an API usage limit cut it off)
- **Started:** 2026-09-23 (continuation)
- **Completed:** 2026-09-23
- **Tasks:** 3 (Task 1 tracer: delete end to end; Task 2 TDD: inline rename; Task 3 TDD: collision → confirmed merge + nested-dialog focus)
- **Files modified:** 10 (6 created, 4 modified)

## Accomplishments

- Vendored `dialog`/`alert-dialog` (base-maia), with `DialogTitle`/`AlertDialogTitle` overridden to `text-heading font-semibold` per UI-SPEC — no fifth type size, no third weight.
- `ConfirmDialog.tsx` (D-16): the one reusable confirm over `AlertDialog` — title, one-sentence description, Cancel + a `default`/`destructive` action, a fixed-width pending label while `onConfirm` runs, initial focus on Cancel, optional `finalFocus`. Phase 26's bulk remove can reuse it unchanged.
- `ManageTagsDialog.tsx`: re-fetches `GET /tags` on every open (loading skeletons / error+Retry / empty state), lists tags sorted case-insensitively by name with `· {n} artist{s}` (including `· 0 artists`), and supports delete (count-stating confirm, no undo), inline rename (selected-text input, Save disabled for blank/identical text, `{n}/32` counter from 25 chars, Esc-cancels-only-the-rename via `stopPropagation`, one row in rename mode at a time), and a rename-collision → merge flow (a `ConfirmDialog` naming both tags and the server's post-merge count, `mergeTag` called only inside its `onConfirm`, by id never by name).
- `api.ts`: `ApiError.body` (the parsed non-2xx JSON, e.g. the 409's `target`/`carrier_count_after_merge`); `deleteTag`; `renameTag`/`RenameTagResult` (server-driven collision detection, never a client pre-check); `mergeTag`. All three write calls go through the one `apiFetch` path (CSRF header, tested).
- `watchlist.tsx`: header `Manage tags` button (`secondary`, `Tags` icon) in every route state; `dropTagFromEntries`/`renameTagInEntries`/`mergeTagInEntries` functional per-tag updaters (same shape as `addTag`/`removeTag`) so a delete/rename/merge updates every card and the route vocabulary with no `listWatchlist` refetch; `mergeTagInEntries` drops the source chip in place when an entry already carries the target, so no card ever shows a duplicate chip after a merge.
- Two independent focus-return mechanisms cover the full UI-SPEC focus table row (d)/(e): a `tags`-array-keyed layout effect (`FocusRequest`) for delete/rename/merge success, and a `renameTarget`-keyed effect (`cancelFocusIndexRef`) for rename Cancel/Esc, since cancelling never touches the `tags` array.
- Nested-dialog focus guarantees (UI-SPEC "Mechanism") verified with four dedicated tests: Esc on the merge `ConfirmDialog` closes only it and leaves Manage tags open with the rename input intact; Cancel restores focus to the rename Save; Tab cannot move focus out of the open `ConfirmDialog`; Esc in the rename input does not close the parent dialog.

## Task Commits

Each task was committed atomically (Tasks 2 and 3 followed TDD RED→GREEN):

1. **Task 1 (tracer): Manage tags dialog — delete one tag end to end** — `3c639f2` (feat) — completed by the prior executor
2. **Task 2 RED: failing tests for inline rename (plain and case-only)** — `421631a` (test) — completed by the prior executor
3. **Task 2 GREEN: inline rename (plain and case-only) with cards updating in place** — `80252d4` (feat) — this session
4. **Task 3 RED: failing tests for merge confirm and nested-dialog focus** — `c092156` (test) — this session
5. **Task 3 GREEN: collision → confirmed merge with nested-dialog focus guarantees** — `1cf774f` (feat) — this session

**Plan metadata:** commit follows this SUMMARY.

_Note: this plan's continuation resumed at Task 2 GREEN. The prior executor's RED commit (`421631a`) was re-run first and confirmed still failing for the expected reasons (11 tests, genuine assertions) before any implementation began; its scope was verified to cover Task 2's full `<behavior>` list with no gaps._

## Files Created/Modified

- `web/app/components/ui/dialog.tsx`, `alert-dialog.tsx` (vendored, Task 1)
- `web/app/components/common/ConfirmDialog.tsx` (new, Task 1), `ConfirmDialog.test.tsx` (new, Task 1)
- `web/app/components/watchlist/ManageTagsDialog.tsx` (new, all 3 tasks), `ManageTagsDialog.test.tsx` (new, all 3 tasks)
- `web/app/lib/api.ts`, `api.test.ts` — `ApiError.body`, `deleteTag`, `renameTag`/`RenameTagResult`, `mergeTag`
- `web/app/routes/watchlist.tsx`, `watchlist.test.tsx` — `Manage tags` header button, `dropTagFromEntries`/`renameTagInEntries`/`mergeTagInEntries`

## Decisions Made

See `key-decisions` in the frontmatter: `ApiError.body` as one shared shape for structured non-2xx errors, server-only collision detection in `renameTag`, and `mergeTagInEntries`'s drop-in-place branch for a card that already carries the target.

## Deviations from Plan

None — plan executed exactly as written. (One test-authoring fix — the tab-trap test's second `Tab` landing on base-ui's transient focus-guard sentinel before it redirects to Cancel — was caught and corrected during Task 3's GREEN acceptance loop; see Issues Encountered. Not a plan deviation.)

## Issues Encountered

The first version of the "focus cannot tab out of the open merge ConfirmDialog" test asserted `Cancel` had focus immediately after the second `Tab`, but base-ui's focus trap routes that Tab through an inert sentinel `<span data-base-ui-focus-guard>` before redirecting focus back to `Cancel` on a subsequent tick. Fixed by wrapping that assertion in `waitFor`. This is normal test-writing iteration against a real (already-relied-upon) library behavior, not an application bug or a plan deviation — the four nested-dialog focus tests all pass, confirming the trap itself works correctly.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `ConfirmDialog` is ready for Phase 26's bulk remove to reuse unchanged (D-16).
- `renameTag`/`mergeTag`/`deleteTag` and the route's per-tag functional updaters are the exact seam 24-07's `ArtistNote` work does not need to touch.
- The two backstop truths (D6/D7: merge-title wrap and long tag-name truncation at 375px) are flagged `human_judgment: true` for `/gsd-verify-work` and need a real-browser check, not an assumption to close here.
- No blockers.

---

*Phase: 24-artist-tags-notes*
*Completed: 2026-09-23*

## Self-Check: PASSED

- All 10 key files verified present on disk (`FOUND` for every entry in `files_modified`/`created`).
- All 5 task commit hashes verified present in `git log --oneline --all` (3c639f2, 421631a, 80252d4, c092156, 1cf774f).
- All acceptance criteria across Tasks 1-3 re-run and passing (target vitest suites per task, `typecheck`, the vendoring diff gate).
- Plan-level `<verification>` re-run: `prettier --check` clean, `typecheck` clean, `corepack pnpm --dir web test` green — 313 tests passed, coverage 91.82%/83.16%/90.96%/93.76% (stmts/branch/funcs/lines, all >= 70% gate), `git diff --exit-code -- web/package.json web/pnpm-lock.yaml` clean (no dependency drift), `! grep -rn "dangerouslySetInnerHTML" web/app` exits 0 (no matches).
