---
phase: 26-bulk-edit-remove
plan: 12
subsystem: infra
tags: [spa, go-embed, vite, definition-of-done, release-gate]
status: complete

requires:
  - phase: 26-bulk-edit-remove
    provides: "All Phase 26 web sources (26-01..26-11): selection, action bar, bulk pickers, Preferences popover, bulk remove confirmation"
provides:
  - "Rebuilt internal/webassets/build/client bundle carrying the Phase 26 UI"
  - "Green Definition of Done run and an empty blast-radius diff for the phase"
affects: [26-verify-work]

actuals:
  tokens: 119800
  tasks: 2
  commits: 1
plan_head_before: 5a1adb749800d7bd93126d437088e318d0386d85

tech-stack:
  added: []
  patterns:
    - "No CI bundle-drift check exists, so the embedded SPA is rebuilt as an explicit final task and asserted by grepping the phase's UI strings"

key-files:
  created:
    - internal/webassets/build/client/assets/watchlist-sDRhVZ_n.js
    - internal/webassets/build/client/assets/root-BkJRl_n9.css
  modified:
    - internal/webassets/build/client/index.html

key-decisions:
  - "Task 2 made no commit: every gate passed, so there was no gate fix to commit"

requirements-completed: [BULK-01, BULK-02, BULK-03, BULK-04, BULK-05]

coverage:
  - id: D1
    description: "The Go-embedded SPA serves the Phase 26 UI (Select all visible, Selection actions, Preferences for, None of the selected artists have tags.)"
    requirement: BULK-01
    verification:
      - kind: command
        ref: "grep -rl <phase strings> internal/webassets/build/client/assets && go build ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "Every Definition of Done gate is green and the blast radius against f793ffc is empty"
    requirement: BULK-05
    verification:
      - kind: command
        ref: "go vet, golangci-lint, go test (no -race), coverage 90.72%, sqlc-check, web typecheck/prettier/test, blast-radius git diff"
        status: pass
    human_judgment: false
  - id: D3
    description: "Real-browser behavior (one request per bulk action, filter and selection interplay, toast placement over the bar at 375px, focus never behind the bar, 3:1 contrast, Shift+click range, Undo on remove-tag)"
    requirement: BULK-03
    human_judgment: true
    rationale: "Network panel, layout, contrast, focus and feel checks need a real browser; they become 26-UAT.md items"

duration: 7 min
completed: 2026-10-16
---

# Phase 26 Plan 12: Rebuild embedded SPA and run the phase gate Summary

**The go:embed SPA is rebuilt from this phase's web/ sources so a Go-only build serves the bulk edit and remove UI, and the full Definition of Done passes with an empty blast-radius diff.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-10-16T23:52:08Z
- **Completed:** 2026-10-16T23:59:12Z
- **Tasks:** 2
- **Files modified:** 23 (the bundle tree: hashed assets replaced, `index.html` updated)

## Accomplishments

- Ran the `make web` recipe by hand (`make` is not installed): frozen install, build, replace `internal/webassets/build/client`. `web/package.json` and `web/pnpm-lock.yaml` did not drift.
- Confirmed `Select all visible`, `Selection actions`, `Preferences for`, and `None of the selected artists have tags.` all appear in `assets/watchlist-sDRhVZ_n.js`, and `go build ./...` compiles the embed.
- Ran the full Definition of Done; every gate is green (see Gate Results).
- Blast radius against `f793ffc` is empty: no migration, Go module, npm package, out-of-scope package, outbox query, or single-entry contract file changed.

## Task Commits

1. **Task 1: Rebuild the embedded SPA** - `e1aa421` (feat)
2. **Task 2: Definition of Done and blast radius** - no commit (no gate fix needed)

## Gate Results

| Gate | Result |
| --- | --- |
| `make db-up` | Recipe ran: `docker compose up -d --wait postgres`, healthy |
| `go vet ./...` | clean |
| `golangci-lint run` | 0 issues |
| `make test` | Recipe minus `-race`: 25 packages ok |
| coverage-gate | 90.72% against the 80% floor |
| sqlc-check | `sqlc` v1.31.1, `generate` produced no diff under `internal/db/sqlc/` |
| web typecheck | clean |
| prettier check | clean |
| web test | 34 files, 780 tests passed; coverage 95.6 statements, 91.2 branches, 96.47 functions, 97.37 lines (floor 70) |
| blast-radius diff vs `f793ffc` | empty |

## Files Created/Modified

- `internal/webassets/build/client/` - rebuilt bundle; hashed asset files replace the previous build's, with `watchlist-sDRhVZ_n.js` carrying the phase UI.

## Decisions Made

None beyond the plan; Task 2 committed nothing because no gate failed.

## Deviations from Plan

### Environment substitutions (not code deviations)

- **`make` absent:** `make web`, `make db-up`, `make test`, `make coverage-gate` and `make sqlc-check` were run as their Makefile recipes (24-09 and 25-10 precedent). `make db-up` and `make web` ran as recipe, not target; `make test` ran as its recipe because `make` is absent.
- **`-race` dropped:** `go env CGO_ENABLED` prints 0 and there is no gcc, so `go test` ran without `-race`. CI's `test` job runs `make test-integration` with `-race` on Linux, so the race gate still runs before merge.
- **`go: no such tool "covdata"`:** printed once for `internal/testutil` (no test files); every package with tests passed and `coverage.out` was written, so this is the documented 25-10 environmental deviation.

### Observed flake (no code change)

- On the first backend run, `internal/db` `TestCountWatchlist_Integration` failed once (`CountWatchlist after insert = 19, want 18`). It reads a global `count(*)` from `watchlist`, so a concurrent package writing the same table can shift the count. The test is unchanged by this phase (last touched in the phase-6 squash), passed in isolation, and passed on the full-suite rerun. Left unfixed as out of scope; worth a follow-up to make the assertion tolerant of concurrent writers.

**Total deviations:** 0 auto-fixed. **Impact:** none on the shipped bundle or gates.

## Issues Encountered

None blocking. The single backend flake above is the only red result seen, and it did not recur.

## Known Stubs

None. This plan only regenerates build output.

## Threat Flags

None. No new endpoints, auth paths, or schema changes. T-26-SC2, T-26-43 and T-26-44 are mitigated: lockfile unchanged, bundle changed and carries the phase strings, and the out-of-scope diff is empty.

## UAT Hand-off (human check, end-of-phase)

Run the built binary against the dev database with 30+ watched artists, some with 10 tags, and walk the 26-UI-SPEC "Verification items to carry into UAT" list:

1. Each bulk action issues exactly one request (network panel).
2. Filter to a tag, select all visible, then clear the filter: the count is unchanged and no newly visible artist is selected.
3. Select some artists, then filter so some are hidden: the count drops to the visible ones and returns when the filter clears. Unchecking `Select all visible` leaves hidden selections in place.
4. Bulk-add a tag to a selection that includes an artist at 10 tags: the toast names that artist, and every other card shows the chip.
5. A mixed release type opens indeterminate, and Apply with it untouched leaves each artist's own value.
6. Bulk remove shows the count, three names, `and N more`, and the pending-alert and seed-mode sentence. After confirming, no checkbox stays checked and no reload occurs.
7. With the bar visible, a failure toast appears above it at desktop width and at 375px.
8. Tab through card checkboxes with the bar visible: the focused checkbox is never behind the bar.
9. Unchecked selection boxes and the indeterminate glyph measure at least 3:1.

Also: Shift+click range through the label, and Undo on a bulk remove-tag toast. These become 26-UAT.md items for `/gsd-verify-work`.

## Next Phase Readiness

All 12 plans of Phase 26 are complete. Ready for `/gsd-verify-work 26`.

## Self-Check: PASSED

- `internal/webassets/build/client/assets/watchlist-sDRhVZ_n.js` exists on disk.
- Commit `e1aa421` exists on `gsd/phase-26-bulk-edit-remove`.
