---
phase: 20-digest-settings-operator-control
plan: "04"
subsystem: ui
tags: [react-router, base-ui, shadcn, digest-settings, instant-apply, operator-control]

# Dependency graph
requires:
  - phase: 20-digest-settings-operator-control (plan 20-03)
    provides: "NotificationSettings wire type, getDigestSettings()/updateDigestSettings() wrappers, the DigestSettings card's Digest mode row with instant-apply save machinery, and /system's shared Promise.all fetch composition"
provides:
  - "Vendored web/app/components/ui/select.tsx (shadcn base-maia Select, base-ui/react/select-backed)"
  - "Cadence row: title-case Daily/Weekly select bound to the same rendered-value source as the switch, disabled-but-populated when digest mode is off (D-04), routed through a shared save() helper that always sends both fields (full-object PUT, T-20-19)"
  - "Last digest sent row: locked 'Never sent yet' literal for a null watermark, <time dateTime/title> reusing SourcePanel's formatAbsoluteTime/formatIsoTitle pair for a populated one (DGST-16)"
  - "internal/webassets/build/client rebuilt and committed from this phase's SPA source -- a Go-only clone now embeds and serves the complete Phase 20 operator surface"
affects: ["21-real-time-digest-mutual-exclusion"]

# Actuals (#2632)
actuals:
  tokens: 4489
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "shared instant-apply save() helper: both the switch and the select route through one function that always sends the full {digestEnabled, digestCadence} pair, so a single-field change can never silently clear the other axis on a full-object PUT route"
    - "null-before-formatter branch: a field with a locked non-em-dash empty-state copy takes its null check before format.ts's formatters are ever called, rather than special-casing the shared formatter module"

key-files:
  created:
    - web/app/components/ui/select.tsx
  modified:
    - web/app/components/system/DigestSettings.tsx
    - web/app/components/system/DigestSettings.test.tsx
    - internal/webassets/build/client

key-decisions:
  - "npx shadcn add select mis-resolved its cn dependency as an installable npm package a second time (same failure class as 19-03's table.tsx) -- reverted package.json/pnpm-lock.yaml to their pre-vendoring state and rewired the one bad import to ~/lib/utils instead of hand-writing the whole file from scratch, since everything else the CLI produced (styling, base-ui wiring, icon imports) was correct and reviewable."
  - "SelectValue's children render-prop (not the items= prop) maps the lowercase wire value to its title-case label, keeping the mapping colocated with the two SelectItems it must stay in sync with."
  - "Generalized the Task 1 save handler into a shared save({digestEnabled, digestCadence}) helper both handleDigestModeChange and handleCadenceChange call, rather than duplicating the re-entrancy guard, optimistic overlay, and keep-stale-on-failure logic a second time for the new control."
  - "make test's -race flag is unusable on this Windows dev box (pre-existing documented ThreadSanitizer/cgo limitation, carried since Phase 11.1/15) -- substituted the same go test invocation without -race for this plan's Definition-of-Done run; coverage-gate confirmed 90.72%, comfortably above the 80% floor."

requirements-completed: [DGST-02, DGST-16]

coverage:
  - id: D1
    description: "An operator picks Daily or Weekly from the cadence control on /system with no submit step, and the choice survives a reload"
    requirement: DGST-02
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#renders the Cadence row and saves both fields when a new cadence is chosen"
        status: pass
    human_judgment: false
  - id: D2
    description: "The cadence control stays visible, focusable, and populated with the stored value when digest mode is off -- dimmed, never hidden, never reset to a placeholder"
    requirement: DGST-02
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#keeps the cadence control visible, disabled, and populated with the stored value when digest mode is off"
        status: pass
    human_judgment: false
  - id: D3
    description: "Toggling digest mode never changes the displayed cadence, and a cadence change always sends both fields on the full-object PUT so it can never clear digest_enabled as a side effect"
    requirement: DGST-02
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#does not change the displayed cadence when digest mode is flipped on"
        status: pass
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#renders the Cadence row and saves both fields when a new cadence is chosen"
        status: pass
    human_judgment: false
  - id: D4
    description: "A failed cadence save keeps the previous cadence on screen and shows the same shared failure copy the switch's failure path uses"
    requirement: DGST-02
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#keeps the previous cadence on screen and shows the shared failure copy when a cadence save rejects"
        status: pass
    human_judgment: false
  - id: D5
    description: "A never-sent instance (digest_last_sent_at null) reads the literal 'Never sent yet', never blank and never the view's usual em-dash fallback"
    requirement: DGST-16
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#renders the literal 'Never sent yet' text and no <time> element for a null watermark"
        status: pass
    human_judgment: false
  - id: D6
    description: "A populated digest_last_sent_at renders a <time> carrying the ISO value in dateTime, the full local timestamp in title, and the shared absolute-time formatter's text -- the same pair SourcePanel already uses for finished_at"
    requirement: DGST-16
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#renders a <time> element carrying the ISO instant and the shared absolute-time text for a populated watermark"
        status: pass
    human_judgment: false
  - id: D7
    description: "The committed embedded bundle under internal/webassets/build/client is rebuilt from this phase's SPA source, so a Go binary built from a clone actually serves the digest card, and the full Definition of Done (go vet, golangci-lint, integration suite, coverage-gate, sqlc-check, frontend prettier+vitest) is green with internal/notifier, internal/poller and internal/detection carrying no diff"
    verification:
      - kind: other
        ref: "git status --porcelain internal/webassets/build/client non-empty after make web; go vet ./... clean; golangci-lint run 0 issues; go test ./... (no -race, documented Windows limitation) 24/24 packages ok; make coverage-gate 90.72% (floor 80%); make sqlc-check clean; corepack pnpm --dir web exec prettier --check clean; corepack pnpm --dir web test 225/225 passing, all four coverage axes above 70%; git diff --name-only -- internal/notifier internal/poller internal/detection empty"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-09-13
status: complete
---

# Phase 20 Plan 04: Cadence Control, Last-Sent Row & Embedded Bundle Summary

**Daily/weekly cadence Select and an honest "Never sent yet" last-sent row complete the DigestSettings card, both routed through one shared instant-apply save helper, with the embedded SPA bundle rebuilt to close the phase**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3
- **Files modified:** 3 first-party files (1 created, 2 modified) plus the generated `internal/webassets/build/client` tree

## Accomplishments
- Vendored `web/app/components/ui/select.tsx` from the official shadcn `base-maia` registry (`@base-ui/react/select`-backed, matching `Switch`'s primitive family). The add command mis-resolved its `cn` dependency into `package.json`/`pnpm-lock.yaml` again (the same failure class 19-03 hit with `table.tsx`) — reverted both files and rewired the one bad import to `~/lib/utils`, keeping the rest of the CLI's output (styling, icon imports, all five exports) intact.
- Added the `Cadence` row to `DigestSettings.tsx`: a `Select` bound to the same rendered-value source (`pending ?? settings`) as the switch, showing title-case `Daily`/`Weekly` over the lowercase wire values via `SelectValue`'s children render-prop, disabled (never unmounted) when digest mode is off or a save is in flight (D-04). The Task 1 switch handler was generalized into a shared `save({digestEnabled, digestCadence})` helper both controls now call, so a cadence change always PUTs both fields (T-20-19) without duplicating the re-entrancy guard or keep-stale-on-failure logic.
- Added the `Last digest sent` row: a null watermark renders the locked literal `Never sent yet` — the null check runs before either formatter is ever called, since `format.ts` degrades null to an em dash by design and this field deliberately overrides that. A populated watermark reuses `SourcePanel`'s exact `<time dateTime/title>` + `formatAbsoluteTime`/`formatIsoTitle` pair rather than reimplementing it.
- 13 `DigestSettings` tests now cover both rows: cadence render + full-object save, disabled-but-populated when digest mode is off, cadence surviving a digest-mode flip, a failed cadence save keeping the prior value, and both last-sent branches. No jsdom polyfill was needed — the base-ui `Select` popup drove cleanly under `userEvent` with the project's existing test setup.
- `make web` rebuilt and committed the refreshed `internal/webassets/build/client` tree as the phase's final commit. Ran the full Definition of Done: `go vet`/`golangci-lint` clean, the integration suite green (24/24 packages, `-race` substituted out per this dev box's pre-existing Windows cgo/ThreadSanitizer limitation), `coverage-gate` at 90.72% (floor 80%), `sqlc-check` clean, and the frontend pair (`prettier --check` clean, 225/225 vitest tests, all four coverage axes above 70%). `internal/notifier`, `internal/poller`, and `internal/detection` carry zero diff across the whole phase.

## Task Commits

Each task was committed atomically:

1. **Task 1: Vendor the shadcn select primitive** - `a5b1461` (feat)
2. **Task 2: Cadence control and last-sent row (RED)** - `9632084` (test)
3. **Task 2: Cadence control and last-sent row (GREEN)** - `dfceffe` (feat)
4. **Task 3: Rebuild the embedded SPA bundle and close the phase gate** - `0beceb6` (chore)

## Files Created/Modified
- `web/app/components/ui/select.tsx` - vendored shadcn `Select`/`SelectTrigger`/`SelectValue`/`SelectContent`/`SelectItem` (+ Group/Label/Separator/ScrollButtons), `cn` import fixed to `~/lib/utils`
- `web/app/components/system/DigestSettings.tsx` - Cadence row (`Select`, disabled-but-populated when off), Last digest sent row (null-literal branch before the shared formatters), generalized `save()` helper shared by both controls
- `web/app/components/system/DigestSettings.test.tsx` - 6 new cases for the cadence and last-sent rows
- `internal/webassets/build/client` - rebuilt via `make web` to embed this phase's SPA

## Decisions Made
- Reverted `package.json`/`pnpm-lock.yaml` after the `shadcn add select` command dirtied them with a spurious `cn` npm dependency, and hand-fixed the one bad import in the generated file rather than discarding the CLI's otherwise-correct output.
- Used `SelectValue`'s children render-prop (`{(value) => CADENCE_LABELS[value]}`) instead of the `Select` `items=` prop, keeping the wire-value → title-case mapping next to the two `SelectItem`s it has to stay consistent with.
- Refactored the existing digest-mode save handler into a shared `save()` function parameterized by `{digestEnabled, digestCadence}` so both controls share one re-entrancy guard, optimistic overlay, and failure path instead of duplicating it.
- Substituted a plain `go test` (no `-race`) for `make test`'s verification step, per this Windows dev box's already-documented cgo/ThreadSanitizer limitation (STATE.md, Phase 11.1/15 precedent) — `coverage-gate` still confirmed the real number (90.72%) against the resulting profile.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] shadcn add command mis-resolved `cn` as an installable package**
- **Found during:** Task 1, immediately after running `npx shadcn@latest add select`
- **Issue:** The command added `"cn": "^0.3.0"` to `web/package.json`/`pnpm-lock.yaml` and imported `cn` from that package instead of the project's own `~/lib/utils` — exactly the failure mode 19-03 documented for `table.tsx`. The plan's own acceptance gate (byte-identical manifest/lockfile) forbids this.
- **Fix:** `git checkout -- web/package.json web/pnpm-lock.yaml` to revert both, then edited the one import line in the generated `select.tsx` to `import { cn } from "~/lib/utils"`. No other part of the CLI's output needed changing — icon imports already resolved to `lucide-react` cleanly this time (no `IconPlaceholder` residue).
- **Files modified:** `web/app/components/ui/select.tsx` (import fixed); `web/package.json`/`web/pnpm-lock.yaml` reverted to their pre-task state.
- **Verification:** `git diff --name-only -- web/package.json web/pnpm-lock.yaml` prints nothing; typecheck and the full frontend suite green afterward.
- **Committed in:** `a5b1461` (Task 1 commit)

**2. [Rule 3 - Blocking] `-race` unusable for the integration suite**
- **Found during:** Task 3, running the plan's `make test` verification step
- **Issue:** `go test -race` fails at the cgo build step on this Windows dev machine (`runtime/cgo: cgo.exe: exit status 2`) — a pre-existing, previously-documented limitation (STATE.md records the same substitution for Phase 11.1-04 and Phase 15-02), not something this plan's changes caused.
- **Fix:** Ran the same `go test ./... -count=1 -coverprofile=coverage.out -coverpkg=...` invocation `test-integration` uses, with `-race` dropped, then fed the resulting `coverage.out` into `make coverage-gate` unchanged.
- **Files modified:** None (verification-only substitution, no code change).
- **Verification:** 24/24 packages `ok`, zero `FAIL` lines; `coverage-gate` read the profile and reported 90.72%, well above the 80% floor.
- **Committed in:** n/a (verification step, not a code change; documented in this SUMMARY and Task 3's commit body per the plan's Definition-of-Done requirement)

---

**Total deviations:** 2 auto-fixed (1 blocking supply-chain re-resolution, 1 blocking pre-existing tooling limitation)
**Impact on plan:** Both are environment/tooling workarounds with documented precedent elsewhere in this project's history; no scope creep, no behavior change to the shipped feature.

## Issues Encountered
None beyond the two auto-fixed items above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 20 is complete: `notification_settings` singleton + gated `GET`/`PUT /settings/notifications` (20-01/20-02), the digest-mode switch (20-03), and now the cadence control + last-sent row + rebuilt embedded bundle (20-04) deliver the full operator control surface ahead of Phase 21's mutual-exclusion gate.
- Phase 21 (Real-Time ↔ Digest Mutual Exclusion) can proceed: it reads the same `settings.Store`/`notification_settings` row this phase's SPA writes to, and the SPA panel already carries `digest_enabled`/`digest_cadence` end to end for Phase 21 to add its one-sentence "events queue while digest mode is on" copy to.
- No blockers. `internal/notifier`, `internal/poller`, and `internal/detection` are byte-for-byte unchanged across the whole phase — a fresh install with digest off behaves exactly as v1.4 shipped.

## Self-Check: PASSED

All 3 first-party files verified present on disk (`web/app/components/ui/select.tsx`, `web/app/components/system/DigestSettings.tsx`, `web/app/components/system/DigestSettings.test.tsx`); the `internal/webassets/build/client` tree verified rebuilt (`git status --porcelain` non-empty before staging, clean after commit). All four commit hashes (`a5b1461`, `9632084`, `dfceffe`, `0beceb6`) verified present in `git log`.

---
*Phase: 20-digest-settings-operator-control*
*Completed: 2026-09-13*
