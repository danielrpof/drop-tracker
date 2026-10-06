---
phase: 24-artist-tags-notes
plan: 13
subsystem: ui
tags: [react, tailwind, confirm-dialog, gap-closure, embedded-spa, g-24-5]
status: complete

requires:
  - phase: 24-artist-tags-notes
    provides: ConfirmDialog (24-06), merge/delete confirms in ManageTagsDialog (24-05..24-09)
provides:
  - "wrap-anywhere on ConfirmDialog's AlertDialogTitle and AlertDialogDescription: a 32-character unspaced tag name wraps inside the dialog at 375px instead of overflowing (G-24-5, D-09, D-16)"
  - A ConfirmDialog class-pin test and a ManageTagsDialog two-32-character merge confirm test
  - Refreshed embedded SPA bundle whose CSS carries overflow-wrap:anywhere
affects: [24-verification, 26-bulk-remove]

actuals:
  tokens: 1000
  tasks: 2
  commits: 3
plan_head_before: e0cff84ff44ac8fe85d4f6c0d3d993628d13f3f5

tech-stack:
  added: []
  patterns:
    - "Shared ConfirmDialog owns overflow-wrap:anywhere on title and description; consumers inherit it, vendored ui/alert-dialog.tsx stays untouched"

key-files:
  created: []
  modified:
    - web/app/components/common/ConfirmDialog.tsx
    - web/app/components/common/ConfirmDialog.test.tsx
    - web/app/components/watchlist/ManageTagsDialog.test.tsx
    - internal/webassets/build/client

key-decisions:
  - "wrap-anywhere (overflow-wrap: anywhere), not break-word or word-break: AlertDialogHeader is a place-items-center grid, so the title sizes to fit-content and only anywhere counts break points toward min-content. Ordinary-word titles still break only at spaces."

requirements-completed: [TAG-05, TAG-06]

coverage:
  - id: G-24-5
    description: "Merge confirm title naming two 32-character tags renders in full with wrap-anywhere on the heading and the consequence sentence, no truncate, mergeTag not called before confirmation"
    requirement: "TAG-05"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/ManageTagsDialog.test.tsx#a merge confirm naming two 32-character tags renders the full title and consequence in wrapping elements"
        status: pass
      - kind: unit
        ref: "web/app/components/common/ConfirmDialog.test.tsx#title and description carry wrap-anywhere so a long unspaced tag name wraps instead of overflowing"
        status: pass
    human_judgment: false
  - id: G-24-5-visual
    description: "At 375px in a real browser the merge and delete confirm titles wrap inside the dialog with no horizontal scroll, ellipsis, or clipping; short word titles break only at spaces"
    requirement: "TAG-05"
    human_judgment: true
    rationale: "jsdom computes no layout; the visual wrapping is the plan's Task 2 human check, deferred to phase-level UAT (human_verify_mode: end-of-phase)."
  - id: E-embed
    description: "Embedded bundle rebuilt; built CSS contains overflow-wrap:anywhere"
    requirement: "TAG-06"
    verification:
      - kind: command
        ref: "grep -E 'overflow-wrap: ?anywhere' internal/webassets/build/client/assets/*.css"
        status: pass
    human_judgment: false
---

# Phase 24 Plan 13: Merge confirm title wraps at narrow widths Summary

**`wrap-anywhere` on ConfirmDialog's title and description so a 32-character unspaced tag name wraps inside the 375px merge/delete confirm (G-24-5), with the embedded SPA rebuilt.**

## Performance

- **Started:** 2026-10-05T16:51:36Z
- **Completed:** 2026-10-05T16:58Z
- **Tasks:** 2
- **Files modified:** 3 source/test files plus the embedded bundle

## Accomplishments

- ConfirmDialog passes `className="wrap-anywhere"` to `AlertDialogTitle` and `AlertDialogDescription`; the merge confirm, delete confirm and Phase 26's future bulk remove inherit it. The vendored `ui/alert-dialog.tsx` is unchanged.
- Fail-first evidence (tracer RED): on pre-fix code both new tests failed on `expect(element).toHaveClass("wrap-anywhere")`, receiving only `text-heading font-semibold ...`. After the fix, all 27 tests across both files pass, including every pre-existing one.
- `internal/webassets/build/client` rebuilt; `root-Dg0NMU0m.css` contains `overflow-wrap:anywhere` (the old bundle had no such rule), so a Go-only build serves the fix.
- Whole-phase Definition of Done green (see below).

## Task Commits

1. **Task 1: wrap long unspaced tag names in ConfirmDialog (tracer)** - `05140ae` (fix)
2. **Task 2: rebuild embedded SPA + phase gate** - `845a876` (chore)

`bdbea6c` (docs(25): record phase planning state) landed between the two on the same branch from other work; it is not part of this plan, which is why the measured `commits: 3` exceeds the 2 plan-owned code commits.

## Gate Results

- `go vet ./...`: pass. `golangci-lint run`: 0 issues.
- Full integration suite (no `-race`): every package `ok`. Coverage total 89.33 (gate 80): pass. `sqlc generate` + `git diff --exit-code internal/db/sqlc/`: clean.
- Prettier check, typecheck pass. `pnpm --dir web test`: 23 files, 343 tests pass; coverage 92.32 / 85.49 / 91.81 / 94.34 (threshold 70).
- Blast radius: no diff against `main` in notifier/detection/discord/musicbrainz/deezer, go.mod/go.sum, package.json or lockfile. Since `0cbc334`, `web/app` changed only in the three planned files; `REQUIREMENTS.md` and `alert-dialog.tsx` untouched.

## Deviations from Plan

None - plan executed exactly as written.

### Environment notes

- `make` is not on PATH on this box, so the `web`, test, coverage and sqlc recipes were run directly (same as 24-12), with `~/go/bin` prepended to PATH and `-race` omitted (CI's Linux job is the race gate).
- The Go test run prints `go: no such tool "covdata"` and exits 1 under `-coverpkg` (a toolchain quirk for the test-less `internal/db/sqlc` package). No package reported FAIL, the profile was written, and the coverage gate computed 89.33.

## Known Stubs

None.

## Threat Flags

None.

## Pending human check

UAT test 5 re-run at 375px through the go:embed build (31 `W` + `A` / `B` tags, merge and delete confirms, plus a short-word merge) is deferred to phase-level UAT per the plan.

## Self-Check: PASSED

- `ConfirmDialog.tsx`, both test files and the rebuilt bundle exist; commits `05140ae` and `845a876` exist in `git log`.
