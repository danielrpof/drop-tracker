---
phase: 25-find-filter-watchlist-and-history
plan: 18
subsystem: ui
tags: [embedded-spa, gap-closure, ui-spec, definition-of-done]
status: complete
requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "WR-01/WR-03 sticky carry (25-15), WR-04 Deezer wrapper (25-16), WR-02 fallback anchor (25-17)"
provides:
  - "internal/webassets/build/client rebuilt from the gap-closure web/, so the Go binary serves all four review fixes"
  - "25-UI-SPEC and the CONTEXT G4 line record the 2026-10-11 gap-planning decisions"
  - "Green Definition of Done and an empty blast-radius diff against 8ccde67"
affects: [gsd-verify-work re-test of G-25-4 and WR-02/03/04]
actuals:
  tokens: 40000
  tasks: 3
  commits: 2
plan_head_before: 20a874064fc0353d6add8601d045d9120ca28c2b
tech-stack:
  added: []
  patterns:
    - "Bundle freshness proven by marker counts against the planning-head embedded tree"
key-files:
  created: []
  modified:
    - internal/webassets/build/client
    - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md
    - .planning/phases/25-find-filter-watchlist-and-history/25-CONTEXT.md
key-decisions:
  - "User decision 2026-10-11 (gap planning): WR-01..WR-04 recorded on the UI-SPEC lines they govern; CONTEXT G4 amended in place for WR-03"
requirements-completed: [WLVW-01, WLVW-04, WLVW-06]
coverage:
  - id: D1
    description: "Embedded SPA carries the sticky carry, fallback anchor, and Deezer wrapper fixes"
    requirement: WLVW-04
    verification:
      - kind: command
        ref: "marker counts under internal/webassets/build/client/assets vs 8ccde67, then go build ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "UI-SPEC (three lines) and CONTEXT G4 (one line) record the WR decisions"
    requirement: WLVW-04
    verification:
      - kind: command
        ref: "git diff -U0 8ccde67 line counts: UI-SPEC 3/3, CONTEXT 1/1"
        status: pass
    human_judgment: false
  - id: D3
    description: "Full Definition of Done and blast radius"
    requirement: WLVW-01
    verification:
      - kind: command
        ref: "go vet, golangci-lint, go test with coverage (90.29), sqlc-check, typecheck, prettier, web suite (604 tests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Real-browser focus, scroll anchoring, Back/Forward, and Deezer pill hover"
    requirement: WLVW-04
    human_judgment: true
    rationale: "Layout, focus, and tooltips need a real browser; human_verify_mode is end-of-phase and the items go to /gsd-verify-work"
duration: 5 min
completed: 2026-10-12
---

# Phase 25 Plan 18: Ship the Review Fixes Summary

**The embedded SPA is rebuilt so a Go-only build serves the WR-01..WR-04 fixes, the UI-SPEC and locked G4 line record the user's 2026-10-11 decisions, and every Definition of Done gate is green.**

## Accomplishments

- Rebuilt `internal/webassets/build/client` with the frozen-lockfile recipe (install, build, replace). `package.json` and `pnpm-lock.yaml` did not drift, and `go build ./...` compiles the embed.
- Marker counts under `internal/webassets/build/client/assets` (before at `8ccde67` -> after):
  - `carryTo`: 0 -> 5
  - `previousElementSibling`: 0 -> 1
  - `w-full`: 39 -> 40
- 25-UI-SPEC: exactly three lines changed (page-order Deezer note WR-04, D-12 scroll bullet WR-02, sticky-set bullet WR-01/WR-03).
- 25-CONTEXT: the single G4 bullet keeps its text and gains the `Amended 2026-10-11 (WR-03, user decision 2026-10-11)` sentence; the diff against `8ccde67` is one line removed, one added.
- 25-UAT.md and 25-VERIFICATION.md untouched.

## Task Commits

1. Task 1 (tracer): `24d8aaa` build(25-18): rebuild the embedded SPA with the review fixes
2. Task 2: `48d663b` docs(25-18): record the WR-01..WR-04 decisions in the UI-SPEC and CONTEXT G4 line
3. Task 3: no commit (all gates passed first time, nothing to fix)

## Definition of Done

| Gate | Result |
| ---- | ------ |
| `go vet ./...` | clean |
| `golangci-lint run` | 0 issues |
| Integration suite with coverage profile (no `-race`) | no `FAIL` lines; only the known `go: no such tool "covdata"` from `internal/testutil` |
| Backend coverage gate (`coverage-report --mode=total`) | 90.29 (floor 80) |
| `sqlc-check` (version guard first) | `v1.31.1` matches `SQLC_VERSION`; `sqlc generate` leaves `internal/db/sqlc/` clean |
| `web typecheck` | clean |
| `prettier --check "**/*.{ts,tsx}"` | clean |
| `web test` | 31 files, 604 tests passed; coverage statements 95.28, branches 90.62, functions 96.1, lines 97.12 (floor 70) |
| Blast radius vs `8ccde67` | empty (no `*.go`, queries, db, events, go.mod/sum, package files, ui/, useUrlParams, root.tsx, TagChips.tsx, UAT, VERIFICATION) |

`make` and bare `pnpm` are not on this box's PATH, so each Makefile recipe was run directly (`golangci-lint` and `sqlc` from `$HOME/go/bin`). The dev Postgres was down at task start, so I ran the `db-up` recipe (`docker compose up -d --wait postgres`) before the suite.

## Accepted exceptions

Accepted exception: CI-only race (carried from user decision 2026-10-11, 25-14)

Reason: cgo is disabled locally so `go test -race` cannot run here. No Go source changed in 25-15 to 25-18 (the blast-radius check pins every `*.go` file), and CI's Linux test job runs `-race`.

## Deviations from Plan

None - plan executed exactly as written.

## Human Checks (queued for /gsd-verify-work)

Real browser, long Watchlist scrolled down: (1) remove a tag from a card and click its second chip label, (2) un-press a chip on a single-tag card with two tags selected, (3) Manage tags merge/delete of a selected tag after a card went sticky, (4) Back after a sticky card under a toggled filter; at 375px hover the truncated Deezer pill. Expected results are in the plan's Task 3 human-check.

## Known Stubs

None.

## Threat Flags

None. T-25-SC (frozen lockfile, package files unchanged), T-25-48 (marker counts above baseline), T-25-49 (empty blast radius), and T-25-50 (exact doc diffs) are mitigated as planned.

## Next

Phase 25 gap-closure plans are complete; ready for /gsd-verify-work re-verification.

## Self-Check: PASSED

- FOUND: internal/webassets/build/client, 25-UI-SPEC.md, 25-CONTEXT.md
- FOUND commits: 24d8aaa, 48d663b
