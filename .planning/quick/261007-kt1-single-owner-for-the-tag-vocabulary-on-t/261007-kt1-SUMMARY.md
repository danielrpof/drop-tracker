---
phase: quick/261007-kt1
plan: 01
subsystem: web
tags: [react, tags, state, stale-response]
requires: []
provides:
  - "web/app/lib/useTagVocabulary.tsx: single owner of GET /tags (TagVocabularyProvider, useTagVocabulary, rewriteTags, TagChange)"
affects: [phase-25 plans 25-07]
key-files:
  created:
    - web/app/lib/useTagVocabulary.tsx
    - web/app/lib/useTagVocabulary.test.tsx
  modified:
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
    - web/app/components/watchlist/ManageTagsDialog.tsx
    - web/app/components/watchlist/ManageTagsDialog.test.tsx
    - web/app/components/watchlist/TagChips.tsx
    - web/app/components/watchlist/TagChips.test.tsx
    - internal/webassets/build/client/**
    - .planning/phases/25-find-filter-watchlist-and-history/25-07-PLAN.md
    - .planning/phases/25-find-filter-watchlist-and-history/25-PATTERNS.md
decisions:
  - "Row rewrite stays in the route: the module emits one TagChange via onChange; the route's applyTagChange maps the module's pure rewriteTags over entries."
requirements: [TAG-01, TAG-05, TAG-06]
status: complete
commits: 4
plan_head_before: 55ca31a1a54f1d784c7593fc86eafc594a794949
actuals:
  tokens: 60000
  tasks: 3
  commits: 4
completed: 2026-10-07
---

# Quick 261007-kt1: Single owner for the tag vocabulary Summary

One `useTagVocabulary` module now owns GET /tags with a single generation counter. Every local change (remember, rename, merge, delete) supersedes any in-flight load. This replaces two separate copies and counters in the route and ManageTagsDialog, and fixes a live bug where a tag created during the first load could be dropped by the stale response.

## Commits

| Task | Commit | Message |
| ---- | ------ | ------- |
| 1 | cdb497c | refactor: add the tag vocabulary owner with one stale-response rule |
| 2 | b69ac5d | refactor: route, Manage tags and chips share one tag vocabulary owner |
| 3a | e9dc57e | chore: rebuild embedded SPA with the single tag vocabulary owner |
| 3b | 7455c40 | docs: point Phase 25 plans at the applyTagChange seam |

## Task 2 RED output (before any production edit)

`vitest run app/routes/watchlist.test.tsx -t "survives that load"` against the unmodified route:

```
FAIL  app/routes/watchlist.test.tsx > Watchlist route > a tag created while the first vocabulary load is in flight survives that load
TestingLibraryElementError: Unable to find role="option" and name "dembow"
```

It passes after the cut-over. The `watchlist.test.tsx` diff is 43 added lines and 0 deleted.

## Seam map for Phase 25 (old name -> new name)

| Old | New |
| --- | --- |
| route `dropTagFromEntries(id)` | `applyTagChange` branch `{kind: "deleted", tagId}` |
| route `renameTagInEntries(tag)` | `applyTagChange` branch `{kind: "renamed", tag}` |
| route `mergeTagInEntries(sourceId, target)` | `applyTagChange` branch `{kind: "merged", sourceId, target}` |
| ManageTagsDialog `onDeleted/onRenamed/onMerged/onLoaded` | removed; `TagVocabularyProvider onChange={applyTagChange}` |
| route `vocabulary`, `loadVocabulary`, `rememberTag`, `vocabGen` | `useTagVocabulary()` (`vocabulary`, `ensureLoaded`, `remember`) |
| `TagActions.vocabulary/loadVocabulary/rememberTag` | removed; `TagActions` holds `addTag` / `removeTag` only |

The 25-07 plan and 25-PATTERNS now name `applyTagChange`. The 25-07 key_link pattern is `onChange=\\{applyTagChange\\}`.

## Row-rewrite placement decision

Rows are route state, and Phase 25 adds sticky ids and URL writes on the same events. So the module does not touch `entries`. It emits one discriminated `onChange(TagChange)` and exports the pure `rewriteTags`, and the route's single `applyTagChange` applies it. The meaning of each change is defined once and unit-tested without rendering the route.

## Intentional behavior change

After a failed Manage tags load, the next "+ tag" open retries GET /tags. Before, the route kept its older copy and did not retry.

## Deviations from Plan

None - plan executed as written. Implementation notes:
- ManageTagsDialog focus: a `useState` focus request is consumed by a layout effect keyed on the list, status and request. Delete uses an "after-delete" request that waits until the removed row is gone, so it cannot resolve against a pre-removal list.
- `25-07-PLAN.md` uses CRLF line endings; the edits preserved them.

## Gates run

- Web: `prettier --check`, `typecheck`, and the full `vitest run` (25 files, 382 tests) all pass with the coverage thresholds. Module coverage: 100% statements for `useTagVocabulary.tsx`.
- Go: `go vet ./...` and `go test ./internal/webassets/...` pass.
- Not run: golangci-lint, `make test`, coverage-gate and sqlc-check. No Go source changed (only the embedded SPA assets), as the plan allows. `make` is not installed and `-race` is unavailable (no cgo).
- Retired-seam greps are clean. `listTags` has one non-test caller (the module).

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

Files exist: `useTagVocabulary.tsx`, `useTagVocabulary.test.tsx`, and the rebuilt bundle `watchlist-DCNGWDcG.js` (contains the provider error string). Commits cdb497c, b69ac5d, e9dc57e, 7455c40 are on the branch.
