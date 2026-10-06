---
phase: 24-artist-tags-notes
plan: 04
subsystem: ui
tags: [react, react-router, tags, watchlist, accessibility]

requires:
  - phase: 24-01
    provides: "POST/DELETE /watchlist/{id}/tags, GET /watchlist enriched with tags []TagRef and note *string"
provides:
  - "web/app/lib/api.ts: TagRef interface, WatchlistEntry.tags/note, detachTag(entryId, tagId)"
  - "web/app/components/watchlist/TagChips.tsx: TagChips component, TagActions interface"
  - "web/app/routes/watchlist.tsx: addTag/removeTag functional updaters (D-24), announce(), route-level role=status region"
affects: [24-05-tag-autocomplete, 24-06-manage-tags, 24-07-artist-note]

actuals:
  tokens: 6175
  tasks: 2
  commits: 3
  plan_head_before: a9da36ede1bc02b560f027232582816eb3ff7490

tech-stack:
  added: []
  patterns:
    - "Functional per-item route-level updaters (addTag/removeTag) instead of whole-array optimistic snapshots, so concurrent chip add/remove on one row never clobber each other (D-24, amends D-03)"
    - "Layout-effect focus follow: a pending-focus-index ref set synchronously by the click handler, consumed by a useLayoutEffect keyed on entry.tags once the DOM actually shrinks, cleared (never consumed) on a failed rollback"
    - "Self-referencing focus-within on the chip (its own size) plus a named group/chip for the descendant label's un-truncate classes, since group-focus-within cannot target the same element that declares the group"

key-files:
  created:
    - web/app/components/watchlist/TagChips.tsx
    - web/app/components/watchlist/TagChips.test.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/components/watchlist/WatchlistRow.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
    - web/app/components/watchlist/PreferenceToggles.test.tsx
    - web/app/components/watchlist/SearchResultsColumns.test.tsx
    - web/app/components/history/HistoryFilters.test.tsx

key-decisions:
  - "Chip removal's focus move and announce() call fire synchronously at the optimistic removeTag() call, not after detachTag resolves -- matching D-03's optimistic-feedback timing and keeping the behavior independently testable without awaiting the network call."
  - "Two comments (the raw-HTML-prop mention in TagChips.tsx, and role=\"status\" in a watchlist.tsx comment) were rephrased to avoid the plan's own literal-string grep gates (dangerouslySetInnerHTML count, role=\"status\" exactly-once) tripping on prose rather than real usage."

patterns-established:
  - "TagActions (addTag/removeTag) is the contract every later tag-mutating component in this phase (autocomplete, Manage tags) drives through the same route-level functional updaters."

requirements-completed: [TAG-01, TAG-02, TAG-07]

coverage:
  - id: D1
    description: "Each Watchlist card renders its artist's tags as neutral secondary Badge chips straight from GET /watchlist's enrichment -- no per-card fetch -- with HTML-looking tag names rendered as plain text (Phase 06 XSS posture)."
    requirement: TAG-01
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#renders tag names as literal text, including a name that looks like HTML"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#renders nothing when the entry has no tags"
        status: pass
    human_judgment: false
  - id: D2
    description: "A chip's x removes the tag through the real DELETE /watchlist/{id}/tags/{tag_id} wrapper via route-level functional removeTag/addTag updaters (D-24), restoring the chip at its original index and firing a toast on failure."
    requirement: TAG-02
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#clicking a chip's × calls actions.removeTag then detachTag through the real DELETE wrapper"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#restores the chip at its original index and fires the toast when detachTag rejects"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#removes one tag chip and calls detachTag once when its × is clicked"
        status: pass
      - kind: unit
        ref: "app/lib/api.test.ts#detachTag DELETEs /watchlist/{entryId}/tags/{tagId} carrying the CSRF header and resolves on 204"
        status: pass
    human_judgment: false
  - id: D3
    description: "Removing a chip moves keyboard focus to the sibling ×'s (next, else previous), never pulls focus back after a failed removal, and announces the removal -- swapping in the 10-tag-freed hint -- through exactly one route-level role=status region."
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#moves focus to the new first chip's × when the first of three chips is removed"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#moves focus to the previous chip's × when the last of three chips is removed"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#does not pull focus back when a removal fails and the chip is restored"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#announces the plain remove message when the artist had fewer than 10 tags"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#announces the max-tags-freed message when the artist had exactly 10 tags"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#mounts exactly one route-level status region, present before the watchlist even loads (UI-SPEC [R6])"
        status: pass
    human_judgment: false
  - id: D4
    description: "A 32-character unbroken tag name in a 375px-wide card truncates with a pointer title, exposes the full name through the × aria-label, and un-truncates visibly while its × has keyboard focus (UI-SPEC [R4] backstop truth)."
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#the label carries group-focus-within/chip un-truncate classes and title for pointer users"
        status: pass
    human_judgment: true
    rationale: "The plan's own must_haves flags this specific truth as verification: backstop -- real visual truncation and the focus-within un-truncate wrap need a rendered browser at 375px width; jsdom does not compute layout, so the unit test can only assert the class names and title attribute are present, not that they visually truncate/un-truncate correctly."

duration: ~20min
completed: 2026-09-23
status: complete
---

# Phase 24 Plan 04: Watchlist Tag Chips (Display + Remove) Summary

**`TagChips` renders GET /watchlist's tags as neutral secondary-Badge chips on every Watchlist card, with a × that removes one tag through the real `DELETE /watchlist/{id}/tags/{tag_id}` route via route-level functional `addTag`/`removeTag` updaters (D-24), full keyboard-focus follow-through, and a single route-level `role="status"` announcer.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-23T16:44:00-05:00 (approx.)
- **Completed:** 2026-09-23T16:54:00-05:00 (approx.)
- **Tasks:** 2 (Task 1 tracer: chips render + × removes end to end; Task 2 TDD: focus/announcements/long-name handling)
- **Files modified:** 10 (2 created, 8 modified)

## Accomplishments

- `WatchlistEntry` gains `tags: TagRef[]` and `note: string | null` (D-28's singular `note`); `detachTag(entryId, tagId)` issues the real `DELETE /watchlist/{id}/tags/{tag_id}` through `apiFetch`, carrying the CSRF header and resolving `undefined` on 204.
- `TagChips` renders each tag as a locked-markup `Badge variant="secondary"` chip (`h-6 max-w-full gap-1 pr-0 pl-2 text-label font-normal`) with a plain-JSX-text label (an HTML-looking tag name renders literally, no `dangerouslySetInnerHTML` anywhere in `web/app`) and a `Button ghost icon-xs` × carrying `aria-label="Remove tag {name} from {artist}"`.
- Removal is optimistic through route-level functional `addTag`/`removeTag` updaters (D-24): each touches only its own entry's tag array, never a whole-array snapshot, so concurrent removals on one row can't clobber each other. A failed `detachTag` restores the chip at its original index and fires `toast.error`.
- Removing a chip moves keyboard focus to the next chip's × (else the previous one), via a `useLayoutEffect` keyed on `entry.tags` actually shrinking; a failed removal's rollback never pulls focus back, because the pending-focus-index ref is cleared in the failure path before the chip is restored.
- One route-level `<div role="status" aria-atomic="true" className="sr-only">`, mounted in every route state (loading, error, empty, populated), announces `Removed "{tag}" from {artist}.` or, when the artist had 10 tags before removal, `Removed "{tag}" from {artist}. You can add tags again.`
- The chip label carries `title={name}` for pointer users and `group-focus-within/chip:whitespace-normal group-focus-within/chip:break-all` (with `focus-within:h-auto focus-within:min-h-6 focus-within:py-0.5` on the chip itself) so a keyboard user reading a truncated name via the focused × sees it un-truncate. The chip-row container is `flex flex-wrap` with no `overflow-hidden`/`max-h-`/`line-clamp` anywhere.

## Task Commits

Each task was committed atomically (Task 2 followed TDD RED->GREEN):

1. **Task 1: Tags render as chips and × removes one end to end** — `5123b0e` (feat)
2. **Task 2 RED: failing tests for chip-row focus, announcements, and long-name handling** — `5a27317` (test)
3. **Task 2 GREEN: chip-row focus, route-level status announcements, and long-name un-truncate** — `0593168` (feat)

**Plan metadata:** commit follows this SUMMARY.

## Files Created/Modified

- `web/app/components/watchlist/TagChips.tsx` (new) — the chip row: render, remove, focus follow, announce
- `web/app/components/watchlist/TagChips.test.tsx` (new) — 13 tests across both tasks
- `web/app/lib/api.ts` — `TagRef`, `WatchlistEntry.tags`/`.note`, `detachTag`
- `web/app/lib/api.test.ts` — `detachTag` method/path/CSRF-header/204 test
- `web/app/components/watchlist/WatchlistRow.tsx` — mounts `TagChips` in the existing name column, no other layout change
- `web/app/routes/watchlist.tsx` — `addTag`/`removeTag` functional updaters, `announce`/`statusMessage`, the route-level status region
- `web/app/routes/watchlist.test.tsx` — chip-removal integration test, status-region-count test
- `web/app/components/watchlist/PreferenceToggles.test.tsx`, `SearchResultsColumns.test.tsx`, `web/app/components/history/HistoryFilters.test.tsx` — `tags: []`/`note: null` added to every `WatchlistEntry` fixture literal so typecheck and runtime stay green

## Decisions Made

- Focus move and the status-region announcement both fire synchronously at the optimistic `removeTag()` call rather than after `detachTag` resolves, matching D-03's optimistic-feedback timing and keeping both behaviors testable without awaiting the network round trip.
- `group-focus-within/chip` cannot target the element that declares `group/chip` itself (Tailwind's group variant compiles to a descendant selector), so the chip's own size change on focus uses plain self-referencing `focus-within:*` classes, while `group/chip` is reserved for the descendant label's un-truncate classes.
- Two source comments (the raw-HTML-prop mention in `TagChips.tsx`, and a `role="status"` mention in a `watchlist.tsx` comment) were rephrased to avoid literally matching the plan's own grep-based acceptance gates (`dangerouslySetInnerHTML` count, `role="status"` exactly-once) on prose rather than real usage — caught during acceptance-criteria verification, fixed before either task's commit.

## Deviations from Plan

None — plan executed exactly as written. (See "Decisions Made" above for two verification-loop catches, not scope or behavior changes.)

## Issues Encountered

- Two of the Task 2 focus-movement tests initially left an unhandled `TypeError: Cannot read properties of undefined (reading 'catch')`, because `vi.mock("~/lib/api")`'s auto-mocked `detachTag` returns `undefined` unless a test configures a resolution, and `TagChips.tsx`'s `detachTag(...).catch(...)` chain assumes a real Promise. Fixed by adding `mockDetachTag.mockResolvedValueOnce(undefined)` to both tests before their first assertion -- not a production bug, since the real `detachTag` always returns a Promise.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `TagActions` (`addTag`/`removeTag`) and the route-level `announce`/status region are the exact seam 24-05's "+ tag" autocomplete plugs into (`actions.addTag` already accepts an optional insertion index for the pending-chip case).
- `web/app/components/ui/badge.tsx` and `button.tsx`'s `icon-xs` size are proven in real use; 24-06's Manage tags dialog and 24-07's note block can lean on the same locked chip markup precedent.
- No blockers.

---

*Phase: 24-artist-tags-notes*
*Completed: 2026-09-23*

## Self-Check: PASSED

- All key files verified present on disk (`TagChips.tsx`, `TagChips.test.tsx`, `api.ts` -- `FOUND` for each).
- All 3 task commit hashes verified present in `git log --oneline --all` (5123b0e, 5a27317, 0593168).
- All acceptance criteria across Tasks 1-2 re-run and passing (vitest target suites, `typecheck`, the `dangerouslySetInnerHTML` grep, `role="status"`/`aria-atomic="true"` exactly-once counts, `You can add tags again.`/`group-focus-within/chip` presence, chip-row container class string).
- Plan-level `<verification>` re-run: `prettier --check` clean, `typecheck` clean, `corepack pnpm --dir web test` green -- 244 tests passed, coverage 89.09%/81.3%/87.19%/91.12% (stmts/branch/funcs/lines, all >= 70% gate), `git diff --exit-code -- web/package.json web/pnpm-lock.yaml` clean (no dependency change).
