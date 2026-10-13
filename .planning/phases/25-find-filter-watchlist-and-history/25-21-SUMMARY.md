---
phase: 25-find-filter-watchlist-and-history
plan: 21
subsystem: ui
tags: [react, a11y, aria-describedby, go-embed, gap-closure, ship]

requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "Fallback scroll anchor (25-19) and keyboard focus move (25-20)"
provides:
  - "Disabled Deezer button announces DEEZER_ADD_HINT through aria-describedby on a per-row useId sr-only span"
  - "25-UI-SPEC page-order SearchResultsColumns note records the accessible description (G-25-13)"
  - "Rebuilt internal/webassets/build/client carrying the G-25-11, G-25-12, and G-25-13 fixes"
affects: [25-verify-work]

actuals:
  tokens: 6000
  tasks: 3
  commits: 3
plan_head_before: 92288414c53a268e16651667f9f1fb2136412eb5

tech-stack:
  added: []
  patterns:
    - "Per-row useId hint id with sr-only description beside a disabled control; wrapper keeps the hover title"

key-files:
  created: []
  modified:
    - web/app/components/watchlist/SearchResultsColumns.tsx
    - web/app/components/watchlist/SearchResultsColumns.test.tsx
    - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md
    - internal/webassets/build/client

key-decisions:
  - "G-25-13 (user decision 2026-10-12, review WR-02): describe the disabled Deezer button via aria-describedby to an sr-only hint; no visible note, wrapper title and 40% cap kept."

requirements-completed: [WLVW-01, WLVW-04]

coverage:
  - id: D1
    description: "Disabled Deezer button has accessible description DEEZER_ADD_HINT, name unchanged, per-row unique ids, no description on other buttons"
    requirement: WLVW-01
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#caps the Deezer hint wrapper, titles it, and truncates the button label (G-25-9, WR-04)"
        status: pass
      - kind: unit
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#describes each Deezer button with its own sr-only hint (G-25-13)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Embedded SPA bundle carries all three gap fixes and the Go embed compiles"
    requirement: WLVW-04
    verification:
      - kind: other
        ref: "marker counts vs 9d3413f (parentElement 9->10, data-chip-label 0->3, aria-describedby 3->4) plus go build ./..."
        status: pass
    human_judgment: false
  - id: D3
    description: "Real screen reader announcement, browser focus ring, and scroll anchoring on the rebuilt binary"
    requirement: WLVW-04
    verification: []
    human_judgment: true
    rationale: "Real-browser layout, focus rings, and assistive-tech output; jsdom has no screen reader. /gsd-verify-work re-test items for G-25-11, G-25-12, G-25-13."

duration: 9min
completed: 2026-10-12
status: complete
---

# Phase 25 Plan 21: Deezer hint accessible description and SPA rebuild Summary

**The disabled `Search MusicBrainz to add` button now announces its reason through `aria-describedby` to a per-row `useId` sr-only span (no visual change), and the embedded SPA is rebuilt so the Go binary serves the G-25-11, G-25-12, and G-25-13 fixes.**

## Performance

- **Duration:** about 9 min
- **Completed:** 2026-10-12T22:28Z
- **Tasks:** 3
- **Files modified:** 4 paths (bundle directory counts as one)

## Accomplishments
- Closed G-25-13 (review WR-02): `DEEZER_ADD_HINT` is exported, `SearchResultRow` calls `useId()` once, and the disabled button points at an sr-only sibling span. The wrapper title, 40% cap classes, and the button's single truncating label child are untouched.
- 25-UI-SPEC page-order note records the accessible description, tagged `G-25-13, user decision 2026-10-12`; cumulative UI-SPEC diff against `9d3413f` is two removed and three added lines.
- `internal/webassets/build/client` rebuilt from the frozen lockfile; `go build ./...` compiles the embed.

## Task Commits

1. **Task 1: Disabled Deezer button announces its reason through aria-describedby** - `69d6f9e` (fix, tracer)
2. **Task 2: UI-SPEC accessible-description note** - `8d24c8a` (docs)
3. **Task 3: Rebuild the embedded SPA** - `5358e5b` (build)

## RED evidence

Before the change, `vitest run app/components/watchlist/SearchResultsColumns.test.tsx` gave 3 failed: the `toHaveAccessibleDescription` assertion, the two-row id test (`aria-describedby` null), and the row `textContent` test (the sr-only hint was absent).

## Marker counts (embedded bundle, `9d3413f` -> now)

- `parentElement`: 9 -> 10 (25-19)
- `data-chip-label`: 0 -> 3 (25-20)
- `aria-describedby`: 3 -> 4 (25-21)

## Definition of Done results

- `go vet ./...`: clean. `golangci-lint run`: 0 issues.
- Integration suite with coverage profile (no `-race`): no `FAIL` lines at all, including `internal/testutil`; backend coverage 90.32% (gate 80).
- `sqlc-check`: `sqlc version` v1.31.1 matched `SQLC_VERSION` before generate; `git diff --exit-code -- internal/db/sqlc/` clean.
- Web: typecheck clean, prettier check clean, 622 tests pass; coverage statements 95.34%, branches 90.84%, functions 96.12%, lines 97.16% (gate 70% on all four).
- Blast radius against `9d3413f`: empty for `*.go`, queries/, internal/db, internal/events, go.mod, go.sum, web/package.json, web/pnpm-lock.yaml, ui/, useUrlParams.ts, root.tsx, 25-CONTEXT.md, 25-VERIFICATION.md. Changed `web/app` files are only the eight gap-plan files. 25-UAT.md unchanged since `1cb2a7b` and still carries the "CI-only race extended" waiver.

## Accepted exceptions

Accepted exception: CI-only race (user decision 2026-10-12, 25-UAT.md). Source: the 25-UAT.md ## Execution Waivers line "CI-only race extended", which covers 25-19..25-21. Reason: cgo is disabled locally, no Go source changed in 25-19 to 25-21 (blast-radius check pins every `*.go` file), and CI's Linux test job runs `-race`.

## Decisions Made
- Followed the locked G-25-13 decision exactly; the review's optional visible touch note was not added (UAT truth says no visual change).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None. The hint span's only child is the static constant (T-25-56); no package changed (T-25-SC); all three markers grew in the bundle (T-25-57); blast radius and UI-SPEC diff verified (T-25-58, T-25-59).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Ready for `/gsd-verify-work` re-test of G-25-11, G-25-12, and G-25-13 (human-check: built binary against 50+ artists, keyboard chip toggle, screen reader on a Deezer row).

## Self-Check: PASSED

- Commits `69d6f9e`, `8d24c8a`, `5358e5b` exist; `git rev-list --count 9228841..HEAD` = 3.
- Modified files and the rebuilt bundle exist; the acceptance criteria for all three tasks re-ran green.

---
*Phase: 25-find-filter-watchlist-and-history*
*Completed: 2026-10-12*
