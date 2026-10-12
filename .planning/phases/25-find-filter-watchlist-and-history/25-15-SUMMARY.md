---
phase: 25-find-filter-watchlist-and-history
plan: 15
subsystem: ui
tags: [react-router, sticky-cards, gap-closure, scroll-anchor, tdd]
status: complete
requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "useScrollAnchor and the G4 sticky-card contract from plans 25-05 and 25-11"
provides:
  - "Sticky set keyed to the filterKey and reconciled during render, so it resets in the same commit as the URL change"
  - "Pure reconcileSticky/carrySticky transitions with a carry that lives until the first landing"
  - "writeKeepingSticky for the route's own tag rewrites (Manage tags merge/delete, first-load validation, canonicalization)"
affects: [25-18 real-browser re-check of G-25-4, gsd-verify-work]
actuals:
  tokens: 52000
  tasks: 2
  commits: 2
plan_head_before: 7ae9ace5f773fed7465657e176b912b586b18ee3
tech-stack:
  added: []
  patterns:
    - "Adjust state while rendering: reconcileSticky returns prev itself when nothing landed, so the render-time setSticky converges"
    - "A pending carry is consumed by the first landing (router location key) and applies only to a replace landing on its target"
key-files:
  created: []
  modified:
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
    - web/app/lib/watchlistView.ts
    - web/app/lib/watchlistView.test.ts
key-decisions:
  - "Render-time reset instead of a purely derived `sticky.key === filterKey` check, because a derived check revives a card when a filter is turned off again"
  - "Carry lifetime tied to the write: location key consumes it on the first landing, NavigationType.Replace limits it to route-own rewrites, and the write's starting key must equal the reconciled key"
  - "A landing that leaves filterKey unchanged keeps the set (G4 clears on filter changes only), per the plan-checker iteration 2 override"
requirements-completed: [WLVW-04, WLVW-06]
coverage:
  - id: D1
    description: "A chip click on a sticky card keeps the card mounted, anchored (scrollBy drift correction) and focused"
    requirement: WLVW-04
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#keeps a sticky card's chip anchored and focused when the chip filters (WR-01)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Sticky cards are dropped in the same commit as every filter change, including Back/Forward, and never revived by turning a filter off"
    requirement: WLVW-06
    verification:
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#drops a sticky card in the same commit as Back's URL change"
        status: pass
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#does not bring a sticky card back when a toggle is turned off again"
        status: pass
    human_judgment: false
  - id: D3
    description: "The route's own tag rewrites (Manage tags merge/delete, validation, canonicalization) keep sticky cards; no carry outlives its landing"
    requirement: WLVW-06
    verification:
      - kind: test
        ref: "web/app/lib/watchlistView.test.ts#sticky set (G4, WR-03)"
        status: pass
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#keeps a sticky card when Manage tags merges a selected tag away (WR-03)"
        status: pass
      - kind: test
        ref: "web/app/routes/watchlist.test.tsx#keeps a sticky card when Manage tags deletes one of two selected tags (WR-03)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Scroll position of the clicked sticky card on a real long list in a real browser"
    requirement: WLVW-04
    human_judgment: true
    rationale: "jsdom stubs getBoundingClientRect and scrollBy; the real-browser re-check is owned by plan 25-18"
duration: 8 min
completed: 2026-10-12
---

# Phase 25 Plan 15: Sticky Cards Keyed to the Filter Summary

**Sticky-card set keyed to the filterKey and reset during render (closes G-25-4's sticky path, WR-01), with a pure, single-landing carry so the route's own tag rewrites keep sticky cards (WR-03).**

## Accomplishments

- A chip click on a sticky card no longer unmounts it: the reset now happens in the same render as the router's transitioned location commit, so `useScrollAnchor`'s captured label stays connected, the drift correction runs once, and focus stays on the label (D-08).
- Back/Forward drops stale sticky cards in the same commit as the URL (no painted frame with a stale card). The post-commit `filterKey` effect and the sync clears in `handleTagsChange`/`handleFlagChange`/`handleSortChange` are gone.
- Manage tags merge or delete of a selected tag, first-load tag validation, and canonicalization keep the user's sticky cards via `writeKeepingSticky`. G4 actions (push) and Back/Forward (pop) still clear them.
- `useUrlParams.ts` is untouched and every write still passes `preventScrollReset: true`. No package changes.

## Task Commits

1. Task 1 (tracer): `9057be9` fix(25-15): key sticky cards to the filter and reset them in the URL's render
2. Task 2: `6bb851f` feat(25-15): carry sticky cards across the route's own tag rewrites

Each task was committed once, after GREEN; no RED-only commit was made (plan commit discipline), so the gate-sequence commits `test(...)`/`feat(...)` do not appear separately. RED evidence is recorded below.

## RED Evidence

Task 1 (against the pre-change route):
- Test A `keeps a sticky card's chip anchored and focused when the chip filters (WR-01)`: `expect(label.isConnected).toBe(true)` -> received `false` (the label was unmounted by the first commit).
- Test B `drops a sticky card in the same commit as Back's URL change`: `commits[0]` was `{ search: "?muted=1", shown: ["Bad Bunny", "Bad Two", "Drake"] }`; expected `["Bad Two", "Drake"]` (Bad Bunny still listed, as the old effect clears after the location commit).
- Test C (toggle off again) passed before and after, as planned.

Task 2 (flagged-form transitions: reconcile only on key change, carry whenever `to !== prev.key`):
- F `consumes a carry on the first landing...`: `expected 'b' to be null` (`s2.carryTo`).
- G `applies a carry only to a replace landing on its target`: `expected [ 1 ] to deeply equal []` for the push landing.
- H `never carries over an unlanded filter change or onto the current filter`: `expected 'y' to be null` for `carrySticky(s0, "x", "y")`.
- I `returns the same state when nothing landed`: `expected '1' to be '2'` (`q.loc`); the identity assertion held.
- D `keeps a sticky card when Manage tags merges a selected tag away (WR-03)`: shown was `['Daddy Yankee', 'Travis Scott']`, expected Bad Bunny first.
- E `keeps a sticky card when Manage tags deletes one of two selected tags (WR-03)`: same received list, Bad Bunny missing.

## Design Notes

**Why a render-time reset and not a derived comparison.** The reviewer's sketch derives `ids` only while `sticky.key === filterKey`. That revives a sticky card when the user returns to an earlier filter key (toggle on, then off), which breaks G4. Resetting the state during render (React's "adjust state while rendering" pattern) makes any key change final; `carryTo` is how the route's own rewrites opt out. Test C pins this.

**Carry lifetime.** A carry belongs to the one write that made it:
- The router location key (`useLocation().key`, fresh per navigation) lets the first landing consume it, even a landing that leaves the filter unchanged.
- `useNavigationType() === NavigationType.Replace` limits it to a replace landing; every route-own rewrite is `{ replace: true }`, G4 writes push, and Back/Forward pop.
- The write's starting filter key must equal the reconciled key (`carrySticky(prev, from, to)`), so a write layered on an unlanded G4 change in the same tick never carries, nor does a write back onto the current key.
The transitions are pure functions in `watchlistView.ts` because same-tick write ordering cannot be driven deterministically through the route in jsdom; unit test F covers the stale-carry sequence.

## Deviations from Plan

None - plan executed exactly as written. (The `react-hooks/exhaustive-deps` disable reason on the canonicalization effect was reworded to name `writeKeepingSticky`, as the plan allowed.)

## Verification

- `vitest run app/lib/watchlistView.test.ts app/components/watchlist/TagChips.test.tsx app/routes/watchlist.test.tsx`: 163 passed.
- `corepack pnpm --dir web run typecheck`: clean.
- Per task commit: `prettier --check "**/*.{ts,tsx}"` clean and full `corepack pnpm --dir web test`: 31 files, 596 tests passed, coverage thresholds met (590 tests after Task 1).
- Grep gates: no `clearSticky`/`writeKeepingSticky` in the wrong handlers, `prevFilterKey` count 0, `useScrollAnchor(filterKey)` once, four `writeKeepingSticky(` occurrences, `watchlistView.ts` has no router import.
- `git diff --name-only 7ae9ace HEAD` lists only the four plan files (no `useUrlParams.ts`, `package.json`, or `pnpm-lock.yaml`).
- Go gates, the bundle rebuild, and the real-browser re-check are deferred to 25-18 as planned (no Go file changed).

## Known Stubs

None.

## Threat Flags

None. The new code only changes when client-side sticky state resets; no new endpoint, auth path, or trust boundary (T-25-41..43 mitigated as planned).

## Next

Ready for 25-16.

## Self-Check: PASSED

- FOUND: web/app/routes/watchlist.tsx, web/app/routes/watchlist.test.tsx, web/app/lib/watchlistView.ts, web/app/lib/watchlistView.test.ts
- FOUND commits: 9057be9, 6bb851f
