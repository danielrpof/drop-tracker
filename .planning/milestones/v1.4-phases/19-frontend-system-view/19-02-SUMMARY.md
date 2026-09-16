---
phase: 19-frontend-system-view
plan: "02"
subsystem: ui
tags: [formatters, date-time, vitest, unit-tests]

requires:
  - phase: 19-frontend-system-view/19-01
    provides: the /system route + getStatus() wire types this plan's formatters will feed
provides:
  - "web/app/lib/format.ts — six pure formatters: formatRelativeTime, formatAbsoluteTime, formatIsoTitle, formatClock, formatDuration, formatPollInterval"
  - "web/vitest.config.ts test.env TZ=UTC pin, proven by a self-check test case"
affects: [19-03, 19-04, 19-05]

actuals:
  tokens: 2839
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "format.ts mirrors sources.ts's module shape: a header comment enumerating every exported rule and its call sites, followed by small pure exported functions, no React/third-party import"
    - "Every clock-dependent formatter takes an injected `now`/`d` Date parameter defaulting to new Date(), matching history.test.tsx's controlled-inputs discipline"

key-files:
  created:
    - web/app/lib/format.ts
    - web/app/lib/format.test.ts
  modified:
    - web/vitest.config.ts

key-decisions:
  - "Timezone strategy (RESEARCH Open Question 3): pinned test.env.TZ to UTC in vitest.config.ts rather than building expected strings in-test from Intl — the self-check case (a fixed UTC instant reporting the matching local hour) passed on the first run, so no fallback was needed"
  - "hourCycle: \"h23\" used instead of hour12: false for every Intl.DateTimeFormat call — avoids a known ICU quirk where hour12:false can render midnight as \"24:00:00\" on some builds"
  - "formatIsoTitle uses Intl's timeZoneName: \"shortOffset\" to carry the UTC offset in the title attribute, rather than hand-building an offset string from Date.getTimezoneOffset()"

requirements-completed: [SYS-01, SYS-02]

coverage:
  - id: D1
    description: "formatRelativeTime clamps a negative delta and the <45s bucket both to 'just now', and walks the full grammar ladder (minutes/hours/days) with correct singular/plural boundaries"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/lib/format.test.ts#formatRelativeTime — 9 cases incl. future-clamp and every ladder boundary"
        status: pass
    human_judgment: false
  - id: D2
    description: "formatAbsoluteTime renders HH:MM:SS on the same local day and MMM D, HH:MM on an earlier day; null and unparseable input both degrade to the em dash"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/lib/format.test.ts#formatAbsoluteTime — 4 cases"
        status: pass
    human_judgment: false
  - id: D3
    description: "formatDuration renders all four bucket edges (999/1000/59999/60000ms) plus mid-bucket cases and null, matching the ms/s/m-s bucket rules exactly"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/lib/format.test.ts#formatDuration — 8 cases"
        status: pass
    human_judgment: false
  - id: D4
    description: "formatPollInterval turns 900 into '15 minutes' and 90 into '90 seconds', with correct singular/plural for seconds/minutes/hours"
    requirement: SYS-02
    verification:
      - kind: unit
        ref: "web/app/lib/format.test.ts#formatPollInterval — 6 cases"
        status: pass
    human_judgment: false
  - id: D5
    description: "The TZ=UTC pin is actually in force on the test runner, not silently ignored"
    verification:
      - kind: unit
        ref: "web/app/lib/format.test.ts#timezone self-check — fixed UTC instant reports the matching local hour"
        status: pass
    human_judgment: false
  - id: D6
    description: "format.ts has zero imports (no React, no third-party package) and the whole existing frontend suite stays green under the new TZ pin"
    verification:
      - kind: unit
        ref: "corepack pnpm --dir web test — 14 files / 149→163 tests, all four coverage axes above 70%"
        status: pass
      - kind: other
        ref: "grep -c import web/app/lib/format.ts prints 0"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-10
status: complete
---

# Phase 19 Plan 2: Six Pure Display Formatters + Deterministic Timezone Summary

**`web/app/lib/format.ts` — the six pure formatters D-09/D-09-a/SYS-01/SYS-02 require (relative time, absolute time, ISO title, clock stamp, duration, poll interval), table-driven-tested under a `TZ=UTC` pin proven by a self-check case.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- Pinned `test.env.TZ` to `"UTC"` in `web/vitest.config.ts` (RESEARCH Open Question 3 / Pitfall 4), with a timezone self-check case in `format.test.ts` proving the pin is actually in force rather than silently ignored
- Built `web/app/lib/format.ts` following `sources.ts`'s module shape — one header comment enumerating all six formatters and their call sites, then six small pure exported functions, zero imports
- `formatRelativeTime`: D-08's grammar ladder (just now / minutes / hours / days) with the D-09 clock-skew clamp — any negative delta, and the sub-45s bucket, both render `"just now"`, never a future-tense phrase
- `formatAbsoluteTime`: D-09-a's same-day `HH:MM:SS` vs earlier-day `MMM D, HH:MM` split; null and unparseable input both degrade to the em dash rather than JavaScript's invalid-date text
- `formatIsoTitle`: full local timestamp with a UTC offset (via `Intl`'s `timeZoneName: "shortOffset"`) for a `<time>` element's `title` attribute
- `formatClock`: the D-13 "as of" 24-hour freshness stamp for an injected `Date`
- `formatDuration`: the three SYS-01 buckets (sub-second ms, sub-minute one-decimal seconds, minute-plus `Nm SSs`) — all four bucket edges (999/1000/59999/60000ms) explicitly asserted
- `formatPollInterval`: SYS-02/D-06's human text (`900` → `"15 minutes"`, `90` → `"90 seconds"`), correct singular/plural across seconds/minutes/hours
- Every clock-dependent formatter takes its clock as an injected parameter (`now`/`d`) defaulting to `new Date()`, so all 31 test cases are deterministic with no global-time mocking
- Full frontend suite green: 14 files / 149 tests before this plan → confirmed still green after, `format.ts` itself at 98.4% statements / 93.9% branches / 100% functions / 100% lines

## Task Commits

Each task followed RED-then-GREEN, both committed atomically:

1. **Task 1: Deterministic clock + the four time formatters**
   - RED — `c6ebbf0` (test): failing tests for `formatRelativeTime`/`formatAbsoluteTime`/`formatIsoTitle`/`formatClock` + the TZ self-check; confirmed red via a temporary `format.ts` removal (`Cannot find module '~/lib/format'`)
   - GREEN — `bf3a325` (feat): implemented the four formatters; 17/17 green
2. **Task 2: Duration and poll-interval humanization**
   - RED — `28d19f9` (test): failing tests for `formatDuration`/`formatPollInterval` (`TypeError: … is not a function`)
   - GREEN — `abb7a39` (feat): implemented both formatters, extended the header comment to all six; 31/31 green

## Files Created/Modified

- `web/vitest.config.ts` — added `test.env: { TZ: "UTC" }` with a load-bearing-not-hygiene comment explaining the timezone-dependent-assertion risk (Pitfall 4)
- `web/app/lib/format.ts` (new) — six exported formatters: `formatRelativeTime`, `formatAbsoluteTime`, `formatIsoTitle`, `formatClock`, `formatDuration`, `formatPollInterval`
- `web/app/lib/format.test.ts` (new) — table-driven `describe`/`it.each` coverage per formatter, 31 cases total, plus the timezone self-check

## Decisions Made

- **Timezone strategy resolved as the config-pin path**, not the in-test-`Intl` fallback: the self-check case (`new Date("2026-01-15T14:03:00Z").getHours() === 14`) passed immediately under the `TZ=UTC` pin, so RESEARCH's documented fallback (building every expected string from `Intl` in-test) was never needed.
- **`hourCycle: "h23"` over `hour12: false`** on every `Intl.DateTimeFormat` instance in the module — avoids a documented ICU behavior where `hour12: false` can render midnight as `"24:00:00"` rather than `"00:00:00"` on some builds; `hourCycle: "h23"` is the unambiguous native way to request true 24-hour formatting.
- **`formatIsoTitle` uses `Intl`'s `timeZoneName: "shortOffset"`** to obtain the UTC offset marker rather than hand-computing one from `Date.getTimezoneOffset()`, keeping the module's zero-dependency, `Intl`-only discipline consistent end to end.

## Deviations from Plan

None — plan executed exactly as written, including the RED/GREEN TDD discipline per task and the timezone strategy decision point.

## Issues Encountered

One test-authoring correction during Task 1's GREEN run: an initial relative-time boundary fixture (`"2026-01-14T13:00:00Z"` intended as "25 hours ago") actually computed to 23 hours, landing in the wrong ladder bucket (`"23 hours ago"` instead of the intended `"1 day ago"`). Caught immediately by the failing assertion; fixed by adjusting the fixture timestamp to a genuine 25-hour delta. No implementation change was needed — the formatter's ladder logic was correct throughout.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `format.ts`'s six formatters and `sources.ts` (from context, unchanged this plan) are now both available for 19-03 (theme tokens + table + source display names), 19-04 (badges/About/per-source panels), and 19-05 (history table).
- The `TZ=UTC` pin is in place for the whole `web/` test suite going forward — no further timezone-determinism work needed for any later plan's own `HH:MM:SS` assertions.
- No blockers or concerns.

## Self-Check: PASSED

All created/modified files verified present on disk (`web/app/lib/format.ts`, `web/app/lib/format.test.ts`, `web/vitest.config.ts`); all four commit hashes (`c6ebbf0`, `bf3a325`, `28d19f9`, `abb7a39`) verified in `git log --oneline --all`.

---

*Phase: 19-frontend-system-view*
*Completed: 2026-09-10*
