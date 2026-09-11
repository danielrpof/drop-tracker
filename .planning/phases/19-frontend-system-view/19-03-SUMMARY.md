---
phase: 19-frontend-system-view
plan: "03"
subsystem: ui
tags: [react, typescript, tailwind-v4, shadcn, vitest]

requires:
  - phase: 19-frontend-system-view
    provides: "19-01's system.tsx render-precedence skeleton and getStatus() wire types this plan's primitives will later plug into"
provides:
  - "sourceDisplayName(name) + SOURCE_ORDER in web/app/lib/sources.ts, with test coverage for the module for the first time"
  - "--color-status-ok / --color-status-warn run-health theme tokens in web/app/app.css"
  - "the vendored shadcn Table family (7 exports) in web/app/components/ui/table.tsx, no importers yet"
affects: [19-04, 19-05]

actuals:
  tokens: 1702
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "sourceDisplayName is a lookup, not a title-case transform, to avoid mangling MusicBrainz's real capitalization -- same header-comment density convention (1-3 lines per rule) as the two existing sources.ts rules"
    - "table.tsx hand-written to card.tsx's cn()-only, data-slot-attribute shape rather than trusting a CLI-vendored file verbatim, since the CLI's registry resolution disagreed with the project's own utils alias"

key-files:
  created:
    - web/app/lib/sources.test.ts
    - web/app/components/ui/table.tsx
  modified:
    - web/app/lib/sources.ts
    - web/app/app.css

key-decisions:
  - "npx shadcn add table resolved the registry correctly but installed an actual npm package named \"cn\" (added to package.json/pnpm-lock.yaml) and imported from it instead of the project's ~/lib/utils -- reverted the manifest/lockfile changes and hand-wrote table.tsx per the plan's documented offline-fallback shape instead of trusting the CLI output"
  - "Excluded TableFooter from the exports -- the plan's task action names exactly seven components (no TableFooter), unlike 19-RESEARCH.md's Environment Availability row which mentions an 8-component fallback; the plan's own acceptance criteria is the binding list"

patterns-established:
  - "A CLI-vendoring task's zero-dependency-graph-change gate is verified by hash/diff before trusting the tool's own success report, not just checking the target file exists"

requirements-completed: [SYS-01, SYS-02]

coverage:
  - id: D1
    description: "sourceDisplayName returns MusicBrainz/Deezer real capitalization, passes an unrecognised key through unchanged, and SOURCE_ORDER is the fixed MusicBrainz-then-Deezer tuple"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/lib/sources.test.ts#sourceDisplayName > returns MusicBrainz for musicbrainz"
        status: pass
      - kind: unit
        ref: "web/app/lib/sources.test.ts#sourceDisplayName > passes an unrecognised key through unchanged rather than throwing"
        status: pass
      - kind: unit
        ref: "web/app/lib/sources.test.ts#SOURCE_ORDER > is exactly the two known keys, MusicBrainz first"
        status: pass
    human_judgment: false
  - id: D2
    description: "--color-status-ok and --color-status-warn exist inside the existing @theme block as an additive-only change, and the production Tailwind v4 build stays clean"
    requirement: SYS-02
    verification:
      - kind: other
        ref: "grep -c anchored match on web/app/app.css prints 2; git diff --stat shows insertions only"
        status: pass
      - kind: other
        ref: "corepack pnpm --dir web exec vite build --mode production exits 0 with no @theme error"
        status: pass
    human_judgment: false
  - id: D3
    description: "table.tsx exports all seven Table components, is repo-formatted, adds zero dependencies to package.json/pnpm-lock.yaml, and has no importers yet"
    requirement: SYS-02
    verification:
      - kind: other
        ref: "grep -c TableCaption web/app/components/ui/table.tsx prints 2; git diff --name-only -- web/package.json web/pnpm-lock.yaml prints nothing"
        status: pass
      - kind: unit
        ref: "corepack pnpm --dir web test -- full suite (174 tests, 15 files) green with all four coverage axes above 70%"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-09-10
status: complete
---

# Phase 19 Plan 3: Source Display Names, Run-Health Theme Tokens, and the Vendored Table Summary

**`sourceDisplayName`/`SOURCE_ORDER` in `sources.ts`, two run-health `@theme` tokens in `app.css`, and a hand-written (not CLI-trusted) shadcn `Table` family in `web/app/components/ui/table.tsx` — three leaf primitives for plans 19-04/19-05 with no file overlap with 19-02.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-10T20:52:00Z (approx.)
- **Completed:** 2026-09-11T01:59:14Z
- **Tasks:** 3
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments
- Added `sourceDisplayName` (a lookup, not a title-case transform) and the fixed `SOURCE_ORDER` tuple to `sources.ts`, extending its header comment to a third numbered rule; added `sources.test.ts`, giving the module test coverage for the first time (11 tests covering all three rules)
- Added `--color-status-ok` (`#22c55e`) and `--color-status-warn` (`#f59e0b`) to the existing `@theme` block in `app.css`, beside `--color-event-*`, with a three-line coexistence rationale; confirmed additive-only via `git diff --stat` and a clean production Tailwind v4 build
- Vendored `web/app/components/ui/table.tsx` with all seven required exports (`Table`, `TableHeader`, `TableBody`, `TableRow`, `TableHead`, `TableCell`, `TableCaption`), formatted to repo style, zero new dependencies, no importers yet
- Full frontend suite green: 174 tests / 15 files, coverage 89.17% statements / 80.68% branches / 86.7% functions / 90.59% lines — all above the 70% floor

## Task Commits

Each task was committed atomically:

1. **Task 1: Source display names and the fixed panel order** - `7a1dcdc` (feat)
2. **Task 2: The two run-health theme tokens** - `46b06d7` (feat)
3. **Task 3: Vendor the shadcn table component** - `5ad8755` (feat)

## Files Created/Modified
- `web/app/lib/sources.ts` - Added `sourceDisplayName(name)` lookup + `SOURCE_ORDER` tuple, extended header comment to three numbered rules
- `web/app/lib/sources.test.ts` (new) - Covers `isAddableSource`, `identityField`, `sourceDisplayName` (incl. unknown-key and empty-string passthrough), and `SOURCE_ORDER`
- `web/app/app.css` - Added `--color-status-ok` / `--color-status-warn` inside the existing `@theme` block, with a three-line coexistence comment
- `web/app/components/ui/table.tsx` (new) - Hand-written `Table` family matching `card.tsx`'s `cn()`-only, `data-slot`-attribute shape; `Table` self-wraps in an `overflow-x-auto` container

## Decisions Made
- Task 3 deviated from the plan's primary path (trust the CLI's vendored output) after the CLI-run's own acceptance gate failed: see Deviations below.
- Followed the plan's own seven-export list literally (no `TableFooter`), since that is the task's binding acceptance criteria — 19-RESEARCH.md's Environment Availability row mentions an 8-component fallback but the plan's task action and acceptance criteria both enumerate exactly seven.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `npx shadcn add table` added an unwanted npm dependency and a broken import**
- **Found during:** Task 3 (Vendor the shadcn table component)
- **Issue:** Running `corepack pnpm exec shadcn add table --yes` from `web/` fetched the registry successfully and wrote `app/components/ui/table.tsx`, but the CLI resolved the registry item's `cn` dependency as an installable npm package rather than the project's own `~/lib/utils` alias — it added `"cn": "^0.2.6"` to `package.json` (and a corresponding `pnpm-lock.yaml` update) and generated `import { cn } from "cn"` in the vendored file. This violated two hard acceptance criteria: `git diff --name-only -- web/package.json web/pnpm-lock.yaml` must print nothing, and the file's only project import must be `cn` from `~/lib/utils`.
- **Fix:** Reverted `package.json`/`pnpm-lock.yaml` via `git checkout --`, confirmed byte-identical to the pre-task state, then hand-wrote `table.tsx` per the plan's own documented offline-fallback instructions — matching `card.tsx`'s shape, importing `cn` from `~/lib/utils`, exporting exactly the seven components the plan names.
- **Files modified:** `web/app/components/ui/table.tsx` (rewritten); `web/package.json`/`web/pnpm-lock.yaml` (reverted to pre-task state, net zero diff)
- **Verification:** `git diff --name-only -- web/package.json web/pnpm-lock.yaml` prints nothing; `grep -n "^import" web/app/components/ui/table.tsx` shows only `react` and `~/lib/utils`; full suite green
- **Committed in:** `5ad8755` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** The fix keeps the task's explicit zero-dependency-graph-change gate intact; no scope creep, no third-party package entered the dependency graph.

## Issues Encountered
None beyond the Task 3 deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `sourceDisplayName`/`SOURCE_ORDER` are ready for `system.tsx`'s panel iteration (plan 19-04), replacing `Object.keys` ordering.
- `--color-status-ok`/`--color-status-warn` are ready for `OutcomeBadge` and the About block (plan 19-04) via `bg-status-ok/15`/`text-status-warn` utilities.
- `table.tsx`'s exports are ready for `SourceHistoryTable` (plan 19-05); no importers exist yet, as intended.
- No blockers or concerns. Plan 19-02 (format.ts, vitest.config.ts) landed on disk with no file overlap, confirmed before starting.

## Self-Check: PASSED

All created/modified files verified present on disk; all three commit hashes (`7a1dcdc`, `46b06d7`, `5ad8755`) verified in `git log --oneline --all`.

---
*Phase: 19-frontend-system-view*
*Completed: 2026-09-10*
