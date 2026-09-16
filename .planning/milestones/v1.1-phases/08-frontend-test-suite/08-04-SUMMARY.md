---
phase: 08-frontend-test-suite
plan: 04
subsystem: frontend-tests
tags: [vitest, react-testing-library, event-card, bug-fix, red-green]
dependency_graph:
  requires:
    - 08-01 (Vitest harness, jsdom environment, `~` alias, jest-dom matchers)
  provides:
    - EventCard.test.tsx (four passing tests: three known event_type badges, one fallback-badge regression guard)
    - Guarded EVENT_BADGE lookup in EventCard.tsx
  affects:
    - History route rendering (an unrecognized event_type no longer crashes the whole route)
tech_stack:
  added: []
  patterns:
    - File-local fixture builder with partial overrides (no shared fixtures module, per D-06)
    - RED-then-GREEN commit pair for a folded bug fix (per D-07)
key_files:
  created:
    - web/app/components/history/EventCard.test.tsx
  modified:
    - web/app/components/history/EventCard.tsx
decisions: []
metrics:
  duration: "~15 minutes"
  completed: 2026-08-12
status: complete
actuals:
  tokens: 1062
  tasks: 2
  commits: 2
---

# Phase 8 Plan 04: EventCard Rendering Tests + Fallback-Badge Bug Fix Summary

Closed the folded EventCard crash bug (an `event_type` outside the known union took down the whole
History route) with a RED-then-GREEN test/fix pair, and added the first real test coverage for
`EventCard.tsx`'s three known badge types.

## What Was Built

**Task 1 (RED):** `web/app/components/history/EventCard.test.tsx` — four tests. Three assert the
existing `new_release`/`guest_feature`/`deluxe_change` badges render correctly (all passed against
unfixed source). The fourth builds a fixture whose `event_type` is cast outside the known union
(Pitfall 6's documented pattern) and asserts rendering does not throw and shows an `Unknown` badge —
this test failed against unfixed source with a render-time `TypeError: Cannot read properties of
undefined (reading 'color')`, reproducing the History-route crash described in the folded todo. No
module mock was added — `EventCard.tsx` imports only the `EventItem` type from `~/lib/api`, never a
function, so there is no API boundary to stub.

**Task 2 (GREEN):** `web/app/components/history/EventCard.tsx` — added a module-local
`UNKNOWN_EVENT_BADGE` constant (label `Unknown`, neutral glyph, `var(--color-muted-foreground)`
background, an existing `app.css` token) and applied it as the default (`??`) on the `EVENT_BADGE`
lookup. An `event_type` outside the record's known keys now yields the fallback badge instead of
`undefined`, so one unrecognized row can no longer crash the whole History route through the
top-level error boundary.

## Verification

- `pnpm --dir web exec vitest run app/components/history/EventCard.test.tsx`: RED commit — 3 passed,
  1 failed (non-zero exit, exactly as required). GREEN commit — all 4 passed.
- `pnpm --dir web test` (full suite): 2 files, 7 tests, all passed after the GREEN commit.
- `pnpm --dir web typecheck`: exits 0 both before and after the GREEN commit — the deliberate cast on
  the RED fixture kept it type-valid throughout.
- Manually reverted the `?? UNKNOWN_EVENT_BADGE` default and re-ran the test file: the fourth test
  failed again as expected, then restored the fix — confirms the fix is load-bearing for the test, not
  coincidental.
- `git show --stat` on the RED commit (`1bf8cec`) lists only `EventCard.test.tsx`.
- `git diff --stat` between the RED and GREEN commits lists exactly one file:
  `EventCard.tsx` (`web/app/lib/api.ts` untouched — the `event_type` union was not widened).

## Deviations from Plan

None — plan executed exactly as written.

## Commits

- `1bf8cec` — `test(08-04): add EventCard rendering tests, including failing unrecognized-event-type case`
- `4f51937` — `fix(08-04): fall back to a neutral badge for an unrecognized event_type`

## Self-Check: PASSED

- FOUND: `web/app/components/history/EventCard.test.tsx`
- FOUND: `web/app/components/history/EventCard.tsx` (modified, contains `UNKNOWN_EVENT_BADGE`)
- FOUND: commit `1bf8cec` in `git log --oneline --all`
- FOUND: commit `4f51937` in `git log --oneline --all`
