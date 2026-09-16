---
phase: 19-frontend-system-view
plan: "04"
subsystem: ui
tags: [react, typescript, badges, status-dashboard, vitest]

requires:
  - phase: 19-frontend-system-view/19-01
    provides: the /system route + getStatus() wire types this plan wires the badge/About/panel components into
  - phase: 19-frontend-system-view/19-02
    provides: format.ts's formatAbsoluteTime/formatIsoTitle/formatRelativeTime/formatDuration/formatPollInterval
  - phase: 19-frontend-system-view/19-03
    provides: sourceDisplayName/SOURCE_ORDER and the --color-status-ok/--color-status-warn theme tokens
provides:
  - "classifyOutcome + OutcomeBadge (web/app/components/system/OutcomeBadge.tsx) -- the five D-07 outcome tiers, one classifier shared by the panel and (in 19-05) the history table"
  - "AboutInstance (web/app/components/system/AboutInstance.tsx) -- the five-row About block, reachability/drift both derived from instance.schema_applied alone"
  - "SourcePanel (web/app/components/system/SourcePanel.tsx) -- the D-08 clean-run scan, the conditional counts/skip/escalation lines, ending in a Separator for 19-05's history table"
  - "system.tsx's loaded body: About block -> empty-watchlist callout -> one panel per source in fixed SOURCE_ORDER, replacing plan 19-01's bare Version row"
affects: [19-05]

actuals:
  tokens: 6911
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "classifyOutcome is a pure switch with a mandatory default arm returning a tier descriptor (label + className/variant/icon), rendered by a thin OutcomeBadge wrapper -- one classifier, reused verbatim by 19-05's history table rows"
    - "orderedSourceKeys(sources) in system.tsx puts SOURCE_ORDER's known keys first, then appends any payload key SOURCE_ORDER doesn't recognise, so an unrecognised source is never silently dropped"

key-files:
  created:
    - web/app/components/system/OutcomeBadge.tsx
    - web/app/components/system/OutcomeBadge.test.tsx
    - web/app/components/system/AboutInstance.tsx
    - web/app/components/system/SourcePanel.tsx
  modified:
    - web/app/routes/system.tsx
    - web/app/routes/system.test.tsx

key-decisions:
  - "AboutInstance and SourcePanel both use a bare <h2 className=\"text-heading\"> inside CardHeader rather than CardTitle, per the plan's documented override instruction -- CardTitle's built-in 16px/500 would need overriding anyway"
  - "A doc-comment in AboutInstance.tsx that literally spelled out fetch('/ready') tripped the plan's own grep -c \"fetch(\"/\"/ready\" acceptance gates against comment text, not code -- reworded to describe the absence without using either literal substring"
  - "Added three tests beyond the plan's explicit acceptance-criteria list (a baseline healthy-run rendering case, and an unknown-source-key-appended case) to close coverage gaps against the plan's own must_haves.truths and Task 3's action-text guarantee, before writing this summary"

patterns-established:
  - "Tinted-fill status badges use className (bg-status-ok/15 text-status-ok, etc.) merged over Badge's variant prop; destructive/secondary tiers use the variant prop directly -- the split documented in OutcomeTier's own type"

requirements-completed: [SYS-01, SYS-02, SYS-03]

coverage:
  - id: D1
    description: "classifyOutcome/OutcomeBadge render all five D-07 tiers (Success, Completed with errors, Failed, Interrupted, unrecognised-fallback) correctly, with the Interrupted tier alone carrying a leading icon, and never throw or echo a raw/empty outcome value"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/components/system/OutcomeBadge.test.tsx#OutcomeBadge > renders the Success tier, green status-ok tinted fill"
        status: pass
      - kind: unit
        ref: "web/app/components/system/OutcomeBadge.test.tsx#OutcomeBadge > renders the Completed with errors tier, amber status-warn, no leading icon"
        status: pass
      - kind: unit
        ref: "web/app/components/system/OutcomeBadge.test.tsx#OutcomeBadge > renders the Failed tier using the badge's destructive variant"
        status: pass
      - kind: unit
        ref: "web/app/components/system/OutcomeBadge.test.tsx#OutcomeBadge > renders the Interrupted tier, amber status-warn, with a leading icon distinguishing it from Completed with errors"
        status: pass
      - kind: unit
        ref: "web/app/components/system/OutcomeBadge.test.tsx#OutcomeBadge > renders an unrecognised outcome as a grey badge with a title-cased fallback label, never the raw value"
        status: pass
      - kind: unit
        ref: "web/app/components/system/OutcomeBadge.test.tsx#OutcomeBadge > renders every tier, including the unrecognised one, without throwing"
        status: pass
    human_judgment: false
  - id: D2
    description: "The About block's Schema and Database rows are both derived solely from instance.schema_applied -- one line naming the version when in sync, both numbers with a warning treatment on drift, an em dash plus the unreachable pill when null -- with zero requests to a separate readiness endpoint"
    requirement: SYS-02
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders a single schema line when schema_applied equals schema_expected"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders both schema numbers when schema_applied and schema_expected differ"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the unreachable pill and an em-dash schema row when schema_applied is null"
        status: pass
      - kind: other
        ref: "grep -rn \"/ready\" web/app/components/system web/app/routes/system.tsx | wc -l -> 0; grep -c \"fetch(\" web/app/components/system/AboutInstance.tsx -> 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "The empty-watchlist callout renders exactly when watchlist_size is zero, with the locked title/description and a working link to the watchlist root, and is absent otherwise"
    requirement: SYS-02
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > shows the empty-watchlist callout with a link to the watchlist root when watchlist_size is zero"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > hides the empty-watchlist callout when watchlist_size is greater than zero"
        status: pass
    human_judgment: false
  - id: D4
    description: "A source with a last run renders its outcome badge, absolute finished time inside a <time> with a title attribute, humanized duration, and the run's summary verbatim, with the per-field counts line appearing only when artists_errored > 0"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the last-run badge, verbatim summary, duration, and the clean-run line for a healthy run"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders 'No clean run in recent history' when every ok run in history errored some artists"
        status: pass
    human_judgment: false
  - id: D5
    description: "The D-08 clean-run scan finds the newest history entry that is both outcome ok AND artists_errored === 0, and a source whose only ok runs all errored some artists renders the literal 'No clean run in recent history' -- proving the errored-count clause is enforced, not just the outcome clause"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders 'No clean run in recent history' when every ok run in history errored some artists"
        status: pass
    human_judgment: false
  - id: D6
    description: "A source with no runs renders the locked runless line, and a non-zero consecutive-skips count renders the warn-colored skip line with a leading icon and the absolute last-skip time, together, even when that source has no runs"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders both the runless line and the skip line for a runless-but-skipping source"
        status: pass
    human_judgment: false
  - id: D7
    description: "A source whose latest run is cancelled renders both the Interrupted badge and the per-source escalation line; a source whose latest run is not cancelled renders no escalation line"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the Interrupted badge and the escalation line when the latest run is cancelled"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > does not render the escalation line when the latest run is ok"
        status: pass
    human_judgment: false
  - id: D8
    description: "Panels render in the fixed SOURCE_ORDER (MusicBrainz before Deezer) regardless of the payload's own key order, and a source key present in the payload but absent from SOURCE_ORDER still renders, appended after the known panels rather than being dropped"
    requirement: SYS-01
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > renders the MusicBrainz panel before the Deezer panel even when the fixture lists them in reverse"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#System route > still renders a source key absent from SOURCE_ORDER, appended after the known panels"
        status: pass
    human_judgment: false
  - id: D9
    description: "The full frontend suite (16 files) stays green with all four coverage axes above the 70% floor, and prettier --check reports the tree clean, after this plan's changes"
    verification:
      - kind: unit
        ref: "corepack pnpm --dir web test -- 16 files / 208 tests, statements 89.69%, branches 81.63%, functions 87.57%, lines 91.03%"
        status: pass
      - kind: other
        ref: "corepack pnpm --dir web exec prettier --check \"**/*.{ts,tsx}\" -- All matched files use Prettier code style!"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-10
status: complete
---

# Phase 19 Plan 4: Outcome Badges, the About Block, and Per-Source Panels Summary

**The five D-07 outcome badge tiers, the schema/reachability About block, and the per-source panel (D-08 clean-run scan, skip/escalation lines) wired into `system.tsx`'s loaded body -- the phase's real operator-facing surface area.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-10T21:00:00-05:00 (approx.)
- **Completed:** 2026-09-10T21:13:50-05:00
- **Tasks:** 3
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- `OutcomeBadge.tsx`: `classifyOutcome` (pure switch, mandatory `default` arm) maps every run to one of five tiers -- Success (green), Completed with errors (amber, client-derived from `artists_errored > 0` since no partial outcome exists on the wire), Failed (destructive), Interrupted (amber + leading `Ban` icon, distinct from the other amber tier), and a grey title-cased fallback for any value outside the frozen set, including an empty string
- `AboutInstance.tsx`: the five-row About `Card` -- Version (monospace, no link, D-03), Schema (single line in sync / both numbers with a warning icon on drift / em dash when null, D-02), Database (reachable/unreachable pill derived solely from `schema_applied`, zero readiness requests, D-01), Watchlist (raw pluralized count), Poll interval (humanized via `format.ts`)
- `system.tsx` replaced plan 19-01's bare Version row with `AboutInstance` as the first element under the heading, followed by the empty-watchlist `Alert` callout (locked "Nothing to poll" copy + a link to `/`) whenever `watchlist_size === 0`
- `SourcePanel.tsx`: last-run badge + absolute `<time>` (dateTime + title) + humanized duration, the verbatim `summary` line, a counts line gated on `artists_errored > 0`, the D-08 clean-run scan (`outcome === "ok" && artists_errored === 0`, newest-first), the runless line, the skip line (independent of whether the source has runs), and the cancelled-latest-run escalation line -- ending in a `Separator` for plan 19-05's history table
- `system.tsx` now renders one `SourcePanel` per source, iterating a fixed `SOURCE_ORDER`-first ordering that still appends any source key the payload carries but `SOURCE_ORDER` doesn't recognise, rather than dropping it
- Full frontend suite: 16 files / 208 tests passing, coverage 89.69% statements / 81.63% branches / 87.57% functions / 91.03% lines -- all above the 70% floor

## Task Commits

Each task was committed atomically:

1. **Task 1: classifyOutcome and the five badge tiers** - `d71273f` (feat)
2. **Task 2: The About block and the empty-watchlist callout** - `4a79ca0` (feat)
3. **Task 3: The per-source panel -- last run, clean-run scan, skip and escalation lines** - `5d4fa6d` (feat, amended twice with additional tests before this summary)

## Files Created/Modified

- `web/app/components/system/OutcomeBadge.tsx` (new) - `classifyOutcome` + `OutcomeBadge`, the five D-07 tiers
- `web/app/components/system/OutcomeBadge.test.tsx` (new) - 9 tests covering every tier, the icon presence/absence split, and the empty-string fallback
- `web/app/components/system/AboutInstance.tsx` (new) - the five-row About block, D-01/D-02/D-03
- `web/app/components/system/SourcePanel.tsx` (new) - the per-source panel, D-07/D-08/D-09
- `web/app/routes/system.tsx` - wires `AboutInstance`, the empty-watchlist callout, and one `SourcePanel` per source (fixed order) into the loaded body
- `web/app/routes/system.test.tsx` - extended with schema-sync/drift/null, watchlist-empty/non-empty, healthy-run baseline, no-clean-run, runless-but-skipping, cancelled-escalation (+ its ok counter-case), fixed-order, and unknown-source-key cases

## Decisions Made

- Used a bare `<h2 className="text-heading">` inside `CardHeader` in both `AboutInstance` and `SourcePanel` rather than `CardTitle`, per the plan's own documented override instruction (`CardTitle`'s built-in 16px/500 weight would otherwise need overriding on every use)
- Split "Completed with errors" and "Interrupted" as two visually-adjacent amber tiers distinguished only by the Interrupted tier's leading `Ban` icon, matching the UI-SPEC's explicit glanceability requirement
- Added three tests beyond the plan's literal acceptance-criteria list (a baseline healthy-run rendering case and an unknown-source-key-appended case) before writing this summary, closing coverage gaps against the plan's `must_haves.truths` and Task 3's action-text guarantee that an unrecognised source key still renders

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A doc-comment's own example text tripped the plan's grep acceptance gates**
- **Found during:** Task 2 (The About block and the empty-watchlist callout)
- **Issue:** `AboutInstance.tsx`'s header comment originally documented the D-01 guarantee by writing `no fetch('/ready') anywhere in this file` -- correct in intent, but the literal substrings `fetch(` and `/ready` in that comment made the plan's own acceptance greps (`grep -c "fetch(" AboutInstance.tsx` and `grep -rn "/ready" ...`) both print 1 instead of the required 0, even though the file issues no actual network request.
- **Fix:** Reworded the comment to describe the absence structurally ("this file issues no network request of its own, and consults no separate readiness endpoint") without using either literal substring.
- **Files modified:** `web/app/components/system/AboutInstance.tsx`
- **Verification:** `grep -c "fetch(" web/app/components/system/AboutInstance.tsx` -> 0; `grep -rn "/ready" web/app/components/system web/app/routes/system.tsx | wc -l` -> 0
- **Committed in:** `4a79ca0` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Comment-only fix; no behavior change. Confirms the D-01 guarantee (no second request) holds both in the code and in the acceptance gate that checks it.

## Issues Encountered

One test-authoring correction: the empty-watchlist callout's title (`AlertTitle`) renders as a `<div>`, not a semantic heading, so an initial `screen.findByRole("heading", { name: "Nothing to poll" })` assertion never matched. Caught immediately by the failing test; switched to `screen.findByText(...)`, matching how the component actually renders. No implementation change was needed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `classifyOutcome`/`OutcomeBadge` are ready for plan 19-05's history table rows -- same classifier, second call site, per the plan's own `key_links`.
- `SourcePanel.tsx` ends every panel with a `Separator`, leaving the exact insertion point plan 19-05's per-source history table needs, with no further edits to this plan's files required.
- `system.tsx`'s render-precedence chain (`loadError` / `initialLoading && !data` / `data`) is unchanged in shape; plan 19-05 is expected to add the `first-run` vs `loaded` shape derivation (`deriveLoadedShape`) inside the `data` branch, per 19-01-SUMMARY.md's stated split of responsibility.
- No blockers or concerns.

## Self-Check: PASSED

All created/modified files verified present on disk; all three commit hashes (`d71273f`, `4a79ca0`, `5d4fa6d`) verified in `git log --oneline --all`.

---

*Phase: 19-frontend-system-view*
*Completed: 2026-09-10*
