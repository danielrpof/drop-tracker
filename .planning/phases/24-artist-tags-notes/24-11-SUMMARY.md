---
phase: 24-artist-tags-notes
plan: 11
subsystem: ui
tags: [react, race-condition, vocabulary, tags, gap-closure, embedded-spa]
status: complete

requires:
  - phase: 24-artist-tags-notes
    provides: Watchlist route vocabulary state (24-05..24-09), Manage tags dialog, migration 000012 (24-10)
provides:
  - "vocabGen generation ref in watchlist.tsx: a GET /tags that settles after any fresher local vocabulary change (Manage tags load, delete, rename, merge) is discarded, so a deleted tag can only come back through the explicit Create item (TAG-06, WR-06)"
  - A deferred-promise route test that drives the slow-network race
  - A focus-gated merge-confirm Esc test
  - Refreshed embedded SPA bundle in internal/webassets/build/client
  - Whole-phase Definition of Done green after both gap-closure plans
affects: [24-verification]

actuals:
  tokens: 1800
  tasks: 2
  commits: 2
plan_head_before: bf02c57699a3eaefada089b2ef5a24312897d8f4

tech-stack:
  added: []
  patterns:
    - "Generation ref (useRef counter) to drop stale async results: bump on every fresher local write, capture at request start, compare at settle"

key-files:
  created: []
  modified:
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
    - web/app/components/watchlist/ManageTagsDialog.test.tsx
    - internal/webassets/build/client

key-decisions:
  - "Stale settle is a no-op for both vocabulary and status. It cannot strand the status at loading, because handleTagsLoaded sets loaded itself and drop/rename/merge only run after Manage tags has loaded."
  - "rememberTag, addTag, removeTag and refresh left unchanged: an additive create cannot resurrect a deleted tag, and a late list missing a just-created tag only offers Create, which the server get-or-create resolves (D-29)."

requirements-completed: [TAG-06, TAG-05]

coverage:
  - id: E1
    description: "A '+ tag' GET /tags that settles after Manage tags loaded and deleted a tag is discarded: typing the deleted name offers only Create, other tags are still offered, listTags called exactly twice, listWatchlist once"
    requirement: "TAG-06"
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.test.tsx#a '+ tag' vocabulary fetch that settles after a Manage tags delete does not bring the deleted tag back"
        status: pass
    human_judgment: false
  - id: E2
    description: "Existing route behavior preserved: 24-09 delete-clears-autocomplete, Manage tags delete/rename/merge update every card with no extra listWatchlist call, Create still attaches when listTags rejects"
    requirement: "TAG-06"
    verification:
      - kind: unit
        ref: "web/app/routes/watchlist.test.tsx (20 tests)"
        status: pass
    human_judgment: false
  - id: E3
    description: "Esc on the merge confirm closes only it, leaving Manage tags open and the rename input intact, with no mergeTag call; passes 3 consecutive coverage-disabled runs"
    requirement: "TAG-05"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/ManageTagsDialog.test.tsx#Esc on the merge confirm closes only it, leaving Manage tags open and the rename input intact"
        status: pass
    human_judgment: false
  - id: E4
    description: "TAG-05/TAG-06 backend edge regressions hold: delete twice -> ErrTagNotFound/404, case-only rename is plain, collision rename, blank rename refused, zero-link merge succeeds, list ordered by lower(name) then id"
    requirement: "TAG-05"
    verification:
      - kind: integration
        ref: "internal/tags and internal/httpserver targeted -run set (7 tests, no SKIP)"
        status: pass
    human_judgment: false
  - id: E5
    description: "Whole-phase Definition of Done: go vet, golangci-lint (0 issues), full integration suite, coverage 89.33% (floor 80), sqlc generate with no diff, prettier, typecheck, web suite 335 tests at 92.12/84.99/91.79/94.27 (floor 70)"
    verification:
      - kind: other
        ref: "go vet ./...; golangci-lint run; go test ./... -coverprofile; cmd/coverage-report --mode=total; sqlc generate + git diff --exit-code; prettier --check; pnpm typecheck; pnpm test"
        status: pass
    human_judgment: false
  - id: E6
    description: "Real-browser slow-network (Slow 3G) race through the go:embed build"
    requirement: "TAG-06"
    verification:
      - kind: manual
        ref: "24-11-PLAN.md Task 2 human-check, deferred to phase-level UAT (human_verify_mode: end-of-phase)"
        status: deferred
    human_judgment: true
---

# Phase 24 Plan 11: Stale "+ tag" vocabulary response (WR-06) Summary

**A `vocabGen` generation ref in the Watchlist route now drops any GET /tags response that settles after a fresher local vocabulary change, so a late "+ tag" fetch can no longer revive a tag deleted, renamed or merged in Manage tags.**

## What was done

- **Task 1 (tracer), `067c318`:** Wrote the route test first (deferred first `listTags`, Manage tags load + delete + close, then settle the old response, then type the deleted name). Added `vocabGen` (`useRef(0)`); `loadVocabulary` captures `++vocabGen.current` and applies its result or its `error` status only while that generation is current; `handleTagsLoaded`, `dropTagFromEntries`, `renameTagInEntries` and `mergeTagInEntries` each bump it first.
- **Task 2, `22ee249`:** Added the Cancel-focus gate before Escape in the merge-confirm Esc test, ran `make web` steps, committed the refreshed embedded bundle, and ran the whole-phase gate.

## Fail-first evidence

Before the fix, the new test failed on the stale response putting the deleted tag back as an existing option:

```
watchlist.test.tsx:447:11
expected document not to contain element, found <div ... role="option" id="base-ui-_r_e1_-2">reggaeton</div>
```

After the fix all 20 route tests pass, including the 24-09 autocomplete test and the Manage tags delete, rename and merge tests.

## Esc test de-flake

Ran `ManageTagsDialog.test.tsx` with `--coverage.enabled=false` three times before changing it: 18/18 passed each time on this box, so the 3/3 failure the verifier recorded did not reproduce here. The focus gate (`waitFor(... Cancel ... toHaveFocus())` before `userEvent.keyboard("{Escape}")`) was still added, as the plan and its siblings do, to remove the ordering dependence. All four original assertions are unchanged. Three consecutive coverage-disabled runs after the change: 18/18 each.

## Deviations from Plan

### Auto-fixed Issues

None - plan executed exactly as written, with these environment adaptations (not behavior deviations):

- `make` is not on this box's PATH. The Makefile targets' underlying commands were run directly: `pnpm install --frozen-lockfile`, `pnpm run build`, `rm -rf` + `cp -r web/build/client internal/webassets/build/client` for `web`; `docker compose up -d --wait postgres` for `db-up`; `go test ./... -coverprofile ... -coverpkg ...` plus `cmd/coverage-report --mode=total` and an awk comparison for `test`/`coverage-gate`; `sqlc generate` plus `git diff --exit-code -- internal/db/sqlc/` for `sqlc-check`. Frozen install left `web/package.json` and `web/pnpm-lock.yaml` untouched.
- No `-race` (unusable on this Windows box per STATE.md; CI Linux job is authoritative).
- `sqlc` and `golangci-lint` run from `~/go/bin`.
- The Esc test did not fail pre-change on this machine (see above).

## Verification

- `go vet ./...` clean; `golangci-lint run` 0 issues.
- Full Go suite passed; backend coverage 89.33% (floor 80); `sqlc generate` (v1.31.1) produced no diff.
- Targeted TAG-05/TAG-06 Go edge tests: 7 passed, none skipped.
- `prettier --check`, `typecheck`, `pnpm test`: 23 files / 335 tests passed; coverage 92.12 stmts, 84.99 branches, 91.79 funcs, 94.27 lines.
- `git diff --name-only main -- internal/notifier internal/detection internal/discord internal/musicbrainz internal/deezer go.mod go.sum web/package.json web/pnpm-lock.yaml` prints nothing.
- `git diff --name-only 77de93d -- web/app` lists exactly the route, its test, and `ManageTagsDialog.test.tsx`.
- Bundle: `internal/webassets/build/client` changed (new `watchlist-*.js` and `manifest-*.js` hashes, `index.html`).
- `.planning/REQUIREMENTS.md` not edited, per the plan's prohibition.

## Known Stubs

None.

## Threat Flags

None. No new route or rendering path; tag names still render as plain JSX text.

## Self-Check: PASSED

- FOUND: web/app/routes/watchlist.tsx (4 x `vocabGen.current++`, `gen !== vocabGen.current`, `gen === vocabGen.current`)
- FOUND: web/app/routes/watchlist.test.tsx (new test, `toHaveBeenCalledTimes(2)`)
- FOUND commits: 067c318, 22ee249
