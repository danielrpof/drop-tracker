---
phase: quick/261007-lc4
plan: 01
subsystem: comments
tags: [comment-discipline, phase-24, tags, notes]
status: complete
commits: 4
plan_head_before: 28ad936daeb8bae8df95113c1ebbe8d141651a10
requirements: [TAG-01, TAG-04, TAG-05, TAG-06, NOTE-01]
actuals:
  tokens: 60000
  tasks: 3
  commits: 4
---

# Quick 261007-lc4: Phase 24 comment cleanup Summary

Phase 24's Go, SQL and web comments now follow the comment discipline in `.claude/CLAUDE.md`: 1-3 line why-notes with at most one design-doc id. Four inaccurate comments are fixed. The change is comment-only, which `comment-gates.sh ast` confirms.

## Commits

- `ff5bd50` Go services, handlers and queries, plus the regenerated `internal/db/sqlc` doc comments
- `db23665` web app
- `55dec18` Phase-24 Go test comments
- `be98a62` one-word follow-up (see "SPA rebuild")

## Comment lines per file (`comment-gates.sh counts`)

| File | Before | After |
|------|--------|-------|
| cmd/server/main.go | 250 | 250 |
| internal/db/tags_schema_test.go | 56 | 46 |
| internal/httpserver/server.go | 144 | 140 |
| internal/httpserver/tags.go | 46 | 33 |
| internal/httpserver/tags_test.go | 59 | 47 |
| internal/httpserver/watchlist.go | 121 | 117 |
| internal/httpserver/watchlist_test.go | 153 | 150 |
| internal/tags/normalize.go | 15 | 11 |
| internal/tags/normalize_test.go | 4 | 2 |
| internal/tags/service.go | 57 | 40 |
| internal/tags/service_test.go | 36 | 30 |
| internal/watchlist/service.go | 199 | 179 |
| internal/watchlist/service_test.go | 184 | 169 |
| queries/tags.sql | 45 | 22 |
| queries/watchlist.sql | 54 | 43 |
| web/app/components/common/ConfirmDialog.tsx | 13 | 5 |
| web/app/components/watchlist/ArtistNote.test.tsx | 18 | 15 |
| web/app/components/watchlist/ArtistNote.tsx | 22 | 14 |
| web/app/components/watchlist/ManageTagsDialog.tsx | 23 | 19 |
| web/app/components/watchlist/TagChips.test.tsx | 10 | 8 |
| web/app/components/watchlist/TagChips.tsx | 37 | 23 |
| web/app/components/watchlist/TagCombobox.tsx | 22 | 12 |
| web/app/lib/api.ts | 170 | 152 |
| web/app/lib/tags.ts | 12 | 5 |
| web/app/lib/useTagVocabulary.tsx | 6 | 6 |
| web/app/routes/watchlist.test.tsx | 29 | 25 |
| web/app/routes/watchlist.tsx | 65 | 61 |

Totals count whole files, so pre-Phase-24 comments are included and were left alone. `limits.ts` was not touched (the gate never flagged it).

## Accuracy fixes

- `WithTags` doc (`internal/httpserver/tags.go`) and the `cmd/server/main.go` comment named only attach/detach. Both now say the store backs every tag route (attach/detach plus the /tags list, rename, merge and delete routes) and that they answer 503 without it.
- `server.go` stated the tag-route gating rationale twice. It is now one comment above the first tag route. A one-line "Global tag vocabulary." comment stays above the /tags routes. The `ast` gate compares gofmt output of comment-stripped source, and gofmt keeps a blank line there only while a comment sits between the two route groups.
- `lib/tags.ts` `buildTagSuggestions` claimed pending names are subtracted from candidates. In fact only on-artist tags are removed, and a query matching an on-artist or pending name returns the already-on state. The comment now says that.
- `ManageTagsDialog`, `TagChips`, `api.ts` and `tags.ts`: plan/task narration removed ("Task 2/3 add rename and merge", "see plan objective", "(24-01)"). Test section headers in the Go and web tests lost their `Task N` / `Plan 24-0N` prefixes.

## Stale UPDATE-vs-trigger claim removed

`tags/service.go` `Merge` and `queries/tags.sql` `RepointSourceLinks` both said an UPDATE never fires the cap trigger. Since migration 000011 there is a `BEFORE UPDATE OF artist_id` trigger. It does not fire for merge's `tag_id`-only update, so the merge behavior is correct but the stated reason was wrong. Both comments now say merge moves links via UPDATE and never inserts, and point to ADR 0004. The free-floating merge comment in `queries/tags.sql` is down to one line pointing at the ADR.

## 4-line blocks (WARN)

One remains: `queries/watchlist.sql:16-19`. It is the ListWatchlist tag-array note: three lines of text plus the `--` separator line that joins it to the older paragraph above. It keeps the `mispair` and sqlc#3438 reasoning, which would invite a "simplification" to `json_agg` if lost.

## Known inaccuracies reported, not fixed

- Migration 000010's "case/accent-insensitive identity" comment is inaccurate. Applied migrations are immutable, so it was left alone.
- The doc-comment form check (`golangci-doccomments.yml`) reports 1 issue, which is not from Phase 24: `internal/db/migrate.go:25`, ST1022 on `DefaultMaxAttempts` (its comment starts "Default retry/backoff parameters..."). That file is unchanged by this task and the comment predates Phase 24, so it was left alone. The plan's baseline of "0 issues" did not hold. Every identifier touched here passes.
- No comment revealed a real bug.

## SPA rebuild

The first rebuild was not byte-identical: Tailwind scans `web/app` source, including comments, and the word "shrink" in a `TagChips.test.tsx` comment generated a spurious `.shrink{flex-shrink:1}` rule, which changed the CSS and root chunk hashes. Rewording that comment (`be98a62`) made the rebuild byte-identical to the committed bundle: `git status --porcelain -- internal/webassets` is empty, so no SPA commit was needed.

## Gate results

- `comment-gates.sh all`: ast OK, blocks OK (1 WARN above), narration OK (was 17 hits), notes OK. Baseline was 99 FAIL lines.
- `go vet ./...` clean. `golangci-lint run`: 0 issues.
- Doc-comment config: 1 pre-existing issue (see above), 0 in touched code.
- Integration suite (`TEST_DATABASE_URL`, `-count=1`, coverprofile): 25 packages `ok`, no `FAIL`/`panic`. The `no such tool "covdata"` exit-1 is the known quirk from 261007-ka1. Coverage total: 90.18 (floor 80).
- `sqlc generate` (v1.31.1) leaves `internal/db/sqlc` clean.
- Web: prettier `--check` clean, `typecheck` clean, vitest 25 files / 382 tests pass.
- Hooks (gitleaks, golangci-lint, prettier) passed on every commit. No `--no-verify`, no AI trailers.

## Deviations from Plan

None to the code. The one-line "Global tag vocabulary." comment in `server.go` and the extra `be98a62` commit are the only things beyond the plan's task list.

## Self-Check: PASSED

All four commits exist (`git log` shows `ff5bd50`, `db23665`, `55dec18`, `be98a62`), the working tree is clean for all scoped paths, and every gate above ran on the final tree.
