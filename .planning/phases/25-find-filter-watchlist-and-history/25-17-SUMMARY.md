---
phase: 25-find-filter-watchlist-and-history
plan: 17
subsystem: ui
tags: [scroll-anchor, gap-closure, tdd, react]
status: complete
requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "useScrollAnchor (25-05/25-11) and the sticky-card rewrite (25-15)"
provides:
  - "anchorCandidates(el): chip, next card, previous card"
  - "useScrollAnchor corrects by the first still-connected candidate, so a clicked card that filters itself out no longer collapses the page"
affects: [25-18 real-browser re-check of G-25-4, gsd-verify-work]
actuals:
  tokens: 14000
  tasks: 2
  commits: 2
plan_head_before: 4bf3d134b3b6d4d909b7ff767759991777ba6eb6
tech-stack:
  added: []
  patterns:
    - "Scroll anchor is an ordered candidate list; the lone chip is a one-element list"
key-files:
  created: []
  modified:
    - web/app/lib/useScrollAnchor.ts
    - web/app/lib/useScrollAnchor.test.tsx
    - web/app/routes/watchlist.test.tsx
key-decisions:
  - "User decision 2026-10-11 (gap planning), WR-02: fall back to the clicked card's next, then previous, sibling when the chip is gone"
  - "List heading left out of the candidates: it sits above the list and never moves during filtering"
requirements-completed: [WLVW-04]
coverage:
  - id: D1
    description: "When the clicked card filters itself out, the nearest surviving neighbour keeps its on-screen position"
    requirement: WLVW-04
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#corrects by the previous card's drift when the last clicked card leaves the list (WR-02)"
        status: pass
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#corrects by the next card's drift when a middle clicked card leaves the list (WR-02)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Fallback order, chip precedence, all-gone no-op, and candidate shape"
    requirement: WLVW-04
    verification:
      - kind: test
        ref: "web/app/lib/useScrollAnchor.test.tsx#useScrollAnchor fallback candidates (WR-02)"
        status: pass
      - kind: test
        ref: "web/app/lib/useScrollAnchor.test.tsx#anchorCandidates"
        status: pass
    human_judgment: false
  - id: D3
    description: "Real layout, browser clamping, and native scroll anchoring on a long list"
    requirement: WLVW-04
    human_judgment: true
    rationale: "jsdom has no layout; the browser re-check joins UAT test 4 (25-REVIEW IN-05) and is owned by 25-18"
duration: 5 min
completed: 2026-10-12
---

# Phase 25 Plan 17: Fallback Scroll Anchor Summary

**`useScrollAnchor` now captures the chip plus the clicked card's next and previous sibling cards and corrects by the first still connected, so un-pressing a chip whose card leaves the any-of filtered list keeps the nearest surviving card in place (WR-02).**

## Accomplishments

- `anchorCandidates(el)` returns the chip, then `el.closest("li")`'s next and previous element siblings (HTMLElements only); an element outside any `li` yields just itself.
- The hook's ref holds an ordered list of `{ el, top }`; the key-change layout effect picks the first connected entry, so a connected chip still wins and the G-25-4 shrink, grow and 25-15 sticky cases are unchanged. No connected candidate means no scroll API call. `window.scrollBy` stays the only scroll API.
- The previous "skips the correction when the clicked card leaves the list" route test was rewritten as the previous-neighbour case; a middle-card next-neighbour case was added.
- `useScrollAnchor`'s signature is unchanged, so `watchlist.tsx`, `TagChips.tsx` and `useUrlParams.ts` were not touched; no package changes.

## Task Commits

1. Task 1 (tracer): `3729e5c` fix(25-17): fall back to the clicked card's neighbours when it leaves the list
2. Task 2: `fe62df6` test(25-17): pin the scroll anchor's fallback order, empty case, and chip precedence

Each task was committed once after GREEN (plan commit discipline).

## RED Evidence

Task 1 (against the pre-change hook), both new route tests:
- `corrects by the previous card's drift when the last clicked card leaves the list (WR-02)`: `AssertionError: expected "bound " to be called 1 times, but got 0 times` at `expect(scrollBy).toHaveBeenCalledTimes(1)`.
- `corrects by the next card's drift when a middle clicked card leaves the list (WR-02)`: same assertion, `got 0 times`.

Task 2: the hook was already implemented by Task 1, so the new hook cases (f-j) passed on first run; they pin the behaviour rather than drive it. No defect surfaced in the hook.

## Design Notes

Discretion recorded: the list heading is not a candidate. It sits above the list, so filtering never moves it within the document, and a correction based on it could only fight the browser clamp.

## Deviations from Plan

None - plan executed exactly as written. (Case j was split into three `anchorCandidates` tests: middle, edge with a missing neighbour, and outside any `li`.)

## Verification

- `vitest run` of useScrollAnchor, TagChips, watchlist route and useUrlParams tests: 122 passed.
- `corepack pnpm --dir web run typecheck`: clean.
- Per task commit: `prettier --check "**/*.{ts,tsx}"` clean and full `corepack pnpm --dir web test` green with coverage thresholds met (597 tests after Task 1, 604 after Task 2; 31 files).
- Grep gates: `export function anchorCandidates` count 1, no `scrollTo(`/`scrollIntoView` in the hook, and `git diff` shows no change to `watchlist.tsx`, `TagChips.tsx`, `useUrlParams.ts`, `package.json` or `pnpm-lock.yaml`.
- Go gates and the bundle rebuild are deferred to 25-18 (no Go file changed).
- Human check (browser, long list: sticky card, last card leaves, middle card leaves) is queued for UAT / 25-18.

## Known Stubs

None.

## Threat Flags

None. T-25-46 (at most three candidates, ref cleared before measuring, early return when none connected) and T-25-47 (`window.scrollBy` only) are mitigated as planned.

## Next

Ready for 25-18.

## Self-Check: PASSED

- FOUND: web/app/lib/useScrollAnchor.ts, web/app/lib/useScrollAnchor.test.tsx, web/app/routes/watchlist.test.tsx
- FOUND commits: 3729e5c, fe62df6
