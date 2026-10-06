---
phase: 24-artist-tags-notes
plan: 12
subsystem: ui
tags: [react, race-condition, tags, gap-closure, embedded-spa, wr-08]
status: complete

requires:
  - phase: 24-artist-tags-notes
    provides: route vocabGen guard (24-11), Manage tags dialog (24-05..24-09), migration 000012 (24-10)
provides:
  - "loadGen/loadInFlight refs and invalidateLoad() in ManageTagsDialog: a GET /tags from an earlier Manage tags open that settles after a newer load, delete, rename or merge is dropped, so it never reaches the dialog list or onLoaded (TAG-06, WR-08)"
  - A tracer route test that drives the verifier's exact close/reopen/delete scenario, plus rename and merge variants
  - Three dialog tests (stale success, stale failure, no-strand) and a setOpen helper on renderDialog
  - Refreshed embedded SPA bundle in internal/webassets/build/client
  - Whole-phase Definition of Done green
affects: [24-verification]

actuals:
  tokens: 4400
  tasks: 3
  commits: 3
plan_head_before: 697eab246bcdbc8f1d123a638475825607b268dd

tech-stack:
  added: []
  patterns:
    - "Dialog-side generation ref: load() captures ++loadGen, settles apply only while current; mutation success calls invalidateLoad(), which bumps and refetches only when a load was in flight"

key-files:
  created: []
  modified:
    - web/app/components/watchlist/ManageTagsDialog.tsx
    - web/app/components/watchlist/ManageTagsDialog.test.tsx
    - web/app/routes/watchlist.test.tsx
    - internal/webassets/build/client

key-decisions:
  - "invalidateLoad() restarts load() while loadInFlight is true. A bare generation bump would drop a reopen's only pending load and strand the dialog on its skeleton (T-24-59). Reversible, local to one component."
  - "watchlist.tsx left untouched: once the dialog only reports its newest load, handleTagsLoaded's unconditional write is correct."

requirements-completed: [TAG-06]

coverage:
  - id: E1
    description: "Manage tags opens with its first listTags pending, closes, reopens, deletes reggaeton, closes; the first listTags then settles with the pre-delete list. '+ tag' offers drill and never reggaeton, offers Create for reggaeton; listTags called 2 times, listWatchlist once; reopening makes exactly one more listTags call (3) and shows only post-delete tags"
    requirement: "TAG-06"
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.test.tsx#a Manage tags GET /tags from an earlier open that settles after a delete does not bring the deleted tag back"
        status: pass
    human_judgment: false
  - id: E2
    description: "The same late first-open response does not undo a rename or a merge in '+ tag'"
    requirement: "TAG-06"
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.test.tsx#a Manage tags GET /tags from an earlier open that settles after a rename does not undo it in '+ tag'"
        status: pass
      - kind: unit
        ref: "web/app/routes/watchlist.test.tsx#a Manage tags GET /tags from an earlier open that settles after a merge does not bring the merged-away tag back"
        status: pass
    human_judgment: false
  - id: E3
    description: "Inside the dialog an older open's late success never re-shows a deleted row or reaches onLoaded, a late failure never replaces a rendered list with the error state, and a rename succeeding mid-reopen refetches instead of stranding on the skeleton"
    requirement: "TAG-06"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/ManageTagsDialog.test.tsx (stale success, stale failure, no-strand)"
        status: pass
    human_judgment: false
  - id: E4
    description: "Real network timing through the go:embed build under Slow 3G: close/reopen Manage tags, delete or rename, then '+ tag' offers only Create for the deleted/old name"
    requirement: "TAG-06"
    verification: []
    human_judgment: true
    rationale: "Plan marks this human-check deferred to phase-level UAT (human_verify_mode: end-of-phase); jsdom cannot reproduce real throttled timing"
---

# Phase 24 Plan 12: Manage tags stale GET /tags guard Summary

**ManageTagsDialog now drops any GET /tags superseded by a newer load or a successful delete/rename/merge, closing WR-08 so a late first-open response can no longer revive a deleted tag in the dialog or in "+ tag".**

## Performance

- **Duration:** about 6 min (2026-10-05T01:30:45Z to 01:36:40Z)
- **Tasks:** 3
- **Files:** 3 web/app source files plus the embedded bundle (3 files renamed/modified)

## Accomplishments

- `load()` captures `++loadGen.current`; `.then` and `.catch` return early on a stale generation and clear `loadInFlight` before any state write.
- `invalidateLoad()` is the first statement of the success path in `handleConfirmDelete`, `handleSaveRename` (renamed branch only) and `handleConfirmMerge`. It bumps the generation and refetches only when a load was in flight, so normal flows keep their exact `listTags` call counts.
- Tracer test reproduces the verifier's scenario end to end; rename and merge variants and three dialog tests pin the rest.
- Embedded SPA rebuilt; whole-phase Definition of Done green.

## Task Commits

1. **Task 1 (tracer): generation guard in load()** - `94daf0d` (fix)
2. **Task 2: invalidateLoad + variants + dialog tests** - `d39fa79` (fix)
3. **Task 3: embedded SPA rebuild** - `3d07813` (chore)

## Fail-first evidence

- **Tracer on pre-fix code:** failed at `web/app/routes/watchlist.test.tsx:537`, `expect(queryByRole("option", { name: "reggaeton" })).not.toBeInTheDocument()` found the revived `reggaeton` option. Passes after Task 1.
- **No-strand test on Task 1 code:** failed at `ManageTagsDialog.test.tsx:562`, `expected "vi.fn()" to be called 3 times, but got 2 times` (no refetch after the rename). Passes after `invalidateLoad`.
- **Four pins on Task 1 code:** the rename route variant, the merge route variant, the stale-success pin and the stale-failure pin all passed on Task 1 code (43 of 44 tests passed; only no-strand failed).
- **Mutation check (nothing committed):**
  - Removing the `.then` early return: 5 failures (stale-success pin, the rename and merge route variants, the tracer, and no-strand). Both route variants and the stale-success pin fail as required.
  - Removing the `.catch` early return: the stale-failure pin failed, plus no-strand.
  - Both restored; the file matched HEAD afterward.

## Verification results

- `vitest` on the two files: 44 passed (21 dialog, 23 route); every pre-existing test is unchanged and passing.
- `go vet ./...` clean; `golangci-lint run` 0 issues.
- Full integration suite (no `-race`, Makefile `COVER_PKGS` list): every package `ok`; aggregate coverage 89.27% (floor 80).
- `sqlc generate` with `git diff --exit-code -- internal/db/sqlc/`: clean (sqlc v1.31.1).
- Targeted TAG-05/TAG-06 Go tests: all PASS, no SKIP.
- `prettier --check` clean; `typecheck` clean; `pnpm test` 341 tests, coverage 92.32 / 85.49 / 91.81 / 94.34 (floor 70).
- Blast radius: no change vs `main` in notifier/detection/discord/musicbrainz/deezer, go.mod/go.sum, web/package.json, web/pnpm-lock.yaml; `.planning/REQUIREMENTS.md` untouched since `756c8e3`; the only `web/app` files changed since `756c8e3` are the dialog, its test and the route test (`watchlist.tsx` untouched).

## Deviations from Plan

### Auto-fixed Issues

None - plan executed exactly as written. Environment notes, not deviations:

- `make` is not on PATH on this box. Ran the `web` recipe lines directly, `make coverage-gate`'s comparison via `go run ./cmd/coverage-report --mode=total --profile=coverage.out` (89.27 >= 80), and `sqlc generate` plus `git diff --exit-code` for `sqlc-check`, as 24-11 did.
- `golangci-lint` and `sqlc` run from `~/go/bin` (not on the default PATH).
- `-race` not used (known Windows blocker in STATE.md; CI Linux is the race gate).

**Total deviations:** 0. **Impact:** none.

## Known Stubs

None.

## Threat Flags

None. No new route, request shape or rendering path.

## Human check pending

The Slow 3G close/reopen walkthrough through the built binary (plan Task 3 `human-check`) is deferred to phase-level UAT per `human_verify_mode: end-of-phase`.

## Self-Check: PASSED

- Commits `94daf0d`, `d39fa79`, `3d07813` present in `git log`.
- Modified files exist; acceptance greps passed (`gen !== loadGen.current` x2, `invalidateLoad()` x4 non-comment, `loadInFlight.current = true` x1, `= false` x2, `setOpen` in the dialog test).
