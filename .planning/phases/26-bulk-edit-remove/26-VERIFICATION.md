---
phase: 26-bulk-edit-remove
verified: 2026-10-19T18:11:43Z
status: passed
score: 165/165 must-haves verified (5/5 roadmap success criteria; 160/160 plan truths, of which 8 held-out backstop truths are verified by human UAT 26-UAT.md)
covered_files:
  - .planning/REQUIREMENTS.md
  - .planning/phases/26-bulk-edit-remove/26-01-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-01-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-02-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-02-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-03-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-03-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-04-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-04-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-05-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-05-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-06-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-06-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-07-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-07-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-08-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-08-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-09-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-09-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-10-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-10-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-11-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-11-SUMMARY.md
  - .planning/phases/26-bulk-edit-remove/26-12-PLAN.md
  - .planning/phases/26-bulk-edit-remove/26-12-SUMMARY.md
  - cmd/server/main.go
  - docs/adr/0004-per-artist-tag-cap-trigger.md
  - internal/db/sqlc/bulk.sql.go
  - internal/db/sqlc/querier.go
  - internal/httpserver/bulk.go
  - internal/httpserver/bulk_test.go
  - internal/httpserver/server.go
  - internal/httpserver/tags.go
  - internal/httpserver/tags_test.go
  - internal/tags/bulk.go
  - internal/tags/bulk_test.go
  - internal/watchlist/bulk.go
  - internal/watchlist/bulk_test.go
  - queries/bulk.sql
  - web/app/app.css
  - web/app/components/common/ConfirmDialog.test.tsx
  - web/app/components/common/ConfirmDialog.tsx
  - web/app/components/ui/checkbox.tsx
  - web/app/components/ui/popover.tsx
  - web/app/components/watchlist/BulkActionBar.test.tsx
  - web/app/components/watchlist/BulkActionBar.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagCombobox.test.tsx
  - web/app/components/watchlist/TagCombobox.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/lib/api.test.ts
  - web/app/lib/api.ts
  - web/app/lib/bulkSelection.test.ts
  - web/app/lib/bulkSelection.ts
  - web/app/lib/format.test.ts
  - web/app/lib/format.ts
  - web/app/root.tsx
  - web/app/routes/watchlist.bulk.test.tsx
  - web/app/routes/watchlist.tsx
covered_digest: "v1:sha256:3f76243eaa391ef79588ac0370941ecf0ed8de304262e541d9143c77f61e2095"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 157/165
  gaps_closed:
    - "Shift+click range selection through the 44px label (26-04 backstop): UAT #2 first failed, root-caused to onKeyDownCapture clearing shiftRef on held-Shift auto-repeat, fixed in f3105f1, re-confirmed by the user (UAT #2 pass)"
    - "8 held-out backstop truths routed to human: all satisfied by UAT #1-#6"
    - "26-12 end-to-end UAT walk: UAT #7 pass"
    - "BULK-04 test-tier prohibition (no partial removal): accepted by the user, UAT #8 pass, and re-spot-checked here"
    - "go test -race in CI: UAT #9 pass, CI run 39298714930 green at e2b0499"
  gaps_remaining: []
  regressions: []
coincidental_reliance_items:
  - truth: "[26-05] A concurrent single-axis write committed while the bulk statement waits is not lost, and a row deleted concurrently becomes `gone`."
    reason: incidental-ordering
    harden: "TestService_BulkUpdatePreferences_ConcurrentAxisWriteIsNotLost and _ConcurrentDeleteIsGone wait a fixed 300ms instead of confirming the bulk session is blocked. Poll pg_blocking_pids before committing the holder, as TestService_BulkAttach_WaitsOnConcurrentTenthLink already does (REVIEW WR-02)."
  - truth: "[26-02] Two overlapping bulk adds of different tags with reversed id lists both succeed with no 40P01."
    reason: fixture-only
    harden: "The fixture seeds watchlist ids in artist-id order and ANY() ignores array order, so the test would pass with `ORDER BY a.id` removed. Seed watchlist entries in reverse artist order, or reword the ADR 0004 'Pinned by' line (REVIEW WR-03). The truth itself holds structurally: LockRows sits above the Sort, so rows lock in artist-id order."
advisory:
  - finding: "REVIEW WR-01: bulk remove-tag Undo re-adds by the client-captured tag name (watchlist.tsx:696, 705-708). A rename during the 8s Undo window or in another tab makes Undo create a new tag under the old name."
    category: other
    reason: "Narrow defect in the Undo add-on only. SC2 (bulk add and remove) and the Undo prohibition (applied ids only) are unaffected. Code is unchanged since the 2026-10-16 review. Fix or explicitly accept before ship."
    evidence_status: "code-read only; not exercised by a failing test"
---

# Phase 26: Bulk Edit & Remove Verification Report

**Phase Goal:** The user can change many artists at once (tags, release-type filters, mutes, or removal) by selecting them on the Watchlist. Every change applies in one step, and removal requires confirming an explicit count.
**Verified:** 2026-10-19T18:11:43Z
**Status:** passed
**Re-verification:** Yes. This supersedes the 2026-10-17 `human_needed` report (157/165). All findings below were re-derived against the current tree, not copied.

## What changed since the previous report

Non-planning commits after the previous verification (`git diff --stat c193636 HEAD -- . ':!graphify-out' ':!.planning'`) touch exactly seven files:

| Commit | Change | Phase impact |
|---|---|---|
| `f3105f1` | `WatchlistRow.tsx`: `onKeyDownCapture` now ignores `Shift/Control/Alt/Meta` keydowns (`MODIFIER_KEYS`), so a held Shift's auto-repeat no longer clears `shiftRef` before the checkbox `onChange` reads it. `watchlist.bulk.test.tsx`: +2 tests ("keeps the range when the held Shift auto-repeats mid-click", "treats a keyboard toggle as plain after an abandoned Shift press") | Fixes the only UAT issue (26-04 backstop). Re-read: the fix is a 3-line guard; the abandoned-Shift path still resets the flag on any non-modifier key. |
| `82aaf8b` | Embedded bundle rebuilt (`watchlist-Ci1EeO6B.js`, manifest, `index.html`) | Re-verified byte-identical to a fresh build (below). |
| `e2b0499` | `golang.org/x/text` v0.39.0 to v0.41.0 (and indirect `x/sync` v0.21.0 to v0.22.0) | Dependency hygiene unrelated to phase logic. `go build`, `go vet` and the full integration suite pass on it. It also makes the old report's "go.mod unchanged" blast-radius line obsolete (see below). |

No backend, SQL, or other web file changed. `git diff e2b0499 HEAD` outside `.planning`/`graphify-out` is empty, so CI run 39298714930 (head SHA `e2b0499`) covers the current code.

## Goal Achievement

### Roadmap Success Criteria

| # | Success criterion | Status | Evidence |
|---|---|---|---|
| 1 | Select individual artists and "select all visible", which selects exactly what search and filters show and never hidden artists. The count is always on screen. | ✓ VERIFIED | `effective = effectiveSelection(visible, selectedIds)` (`watchlist.tsx:809`) over the same `visible` the cards render. Select-all uses `toggleAllVisible` (`watchlist.tsx:552`). `bulkSelection.ts` at 100% line coverage. Route tests for filtered-view removal, hidden-selection inertness, and disabled select-all pass in the 782-test web run. Shift ranges now work in a real browser (UAT #2). |
| 2 | Add or remove a tag on all selected artists in one action. Artists that already have (or lack) the tag are unaffected, and an artist pushed past the 10-tag cap is reported by name. | ✓ VERIFIED | `BulkAttach`/`BulkDetach` (`internal/tags/bulk.go`) with ordered `FOR NO KEY UPDATE OF a` pre-lock, count-guarded insert, `had` CTE; cap skips carry the artist name; UI names them via `formatNameList`. `TestService_BulkAttach*`, `TestService_BulkDetach*`, `TestBulkTags_*` pass against live Postgres. UAT #4 (cap-skip toast names the at-cap artist) and #7 pass. |
| 3 | Set release-type filters and muted event types on all selected artists in one action, and every selected artist shows the new settings afterwards. | ✓ VERIFIED | `BulkUpdateWatchlistPreferences` is one statement (MATERIALIZED locked CTE, per-type (current ∪ set) − unset in canonical order, COALESCE nil guards, `IS DISTINCT FROM` update). UI sends only changed types and then refreshes. `TestService_BulkUpdatePreferences*`, `TestBulk_PreferencesEndToEnd` pass. UAT #5 (one PATCH, mixed type omitted) and #7 pass. |
| 4 | Removing the selection first asks for confirmation stating how many artists will be removed. Confirming removes all of them, or none if any part fails. | ✓ VERIFIED | Route-level destructive `ConfirmDialog` titled `Remove {n} artists from your watchlist?`. Backend is one `DELETE … WHERE id = ANY(...) RETURNING id` (`queries/bulk.sql`). Atomicity re-checked this run (below): with a trigger raising on the last of 3 rows, the production-shaped statement fails and all 3 rows remain. UAT #1 (60-char names wrap in the dialog), #7, #8 pass. |
| 5 | After any bulk action the selection clears and the Watchlist shows the result immediately, with no page reload and no checkbox left pointing at a removed artist. | ✓ VERIFIED | `setSelectedIds(EMPTY_IDS)` runs on every success path (`watchlist.tsx:561, 575`), removal filters removed and gone ids out of `entries` and then `refresh()`es. Route test "removes two checked cards with one confirmed request" passes. UAT #7 pass. |

### Plan Must-Have Truths (12 plans, 160 truths)

| Plan | Truths | Verified by tests | Backstop verified by UAT | Key evidence |
|---|---|---|---|---|
| 26-01 bulk delete | 15 | 14 | 1 (race) = UAT #9 | `TestService_BulkRemove`, gating 401/403 tests, ID bounds tests; CI `-race` green |
| 26-02 bulk add-tag | 16 | 16 | 0 | `TestService_BulkAttach*`, `TestBulkTags_Add*`, ADR 0004 amendment |
| 26-03 checkboxes/bar/confirm | 18 | 17 | 1 (60-char wrap) = UAT #1 | ConfirmDialog and route dialog-copy tests |
| 26-04 select-all/shift/lock | 14 | 13 | 1 (shift via label) = UAT #2 | `bulkSelection.test.ts`, shift-range tests incl. the new auto-repeat test |
| 26-05 bulk preferences | 12 | 12 | 0 | `TestService_BulkUpdatePreferences*`, `TestBulk_Preferences*` |
| 26-06 focus/announce/clearance | 14 | 13 | 1 (375px fit) = UAT #3 | focus, announcement and clearance suites |
| 26-07 bulk remove-tag | 11 | 11 | 0 | `TestService_BulkDetach*`, `TestBulkTags_Remove*` |
| 26-08 bulk add-tag UI | 12 | 10 | 2 (long text) = UAT #4 | "Watchlist bulk add tag" route tests |
| 26-09 entry-free TagCombobox | 10 | 10 | 0 | `TagCombobox.test.tsx`, `BulkActionBar.test.tsx` |
| 26-10 popover/tri-state | 16 | 15 | 1 (375px fit) = UAT #5 | "Watchlist bulk preferences" route tests, `buildAxisDelta` tests |
| 26-11 remove-tag UI + Undo | 17 | 16 | 1 (truncation) = UAT #6 | "Watchlist bulk remove tag" route tests |
| 26-12 rebuild + DoD | 5 | 5 | 0 (e2e walk = UAT #7) | bundle freshness and gates (below) |

**Score:** 165/165 (5 SCs + 152 test-verified plan truths + 8 `verification: backstop` truths verified by human UAT #1 to #6 and #9; 0 present-behavior-unverified). The 8 backstop truths were treated as satisfied by UAT results as instructed. UAT 26-UAT.md is `status: complete`, 9 passed, 0 issues, 0 pending, and its one logged gap (shift-click) is `resolved`. Two truths are VERIFIED with a coincidental-reliance advisory (frontmatter). That is advisory only and does not affect the score.

### Required Artifacts

| Artifact | Status | Details |
|---|---|---|
| `queries/bulk.sql` | ✓ VERIFIED | Read in full: BulkDeleteWatchlist, LockWatchlistArtistsForBulkTag, BulkAttachTag, BulkUpdateWatchlistPreferences, GetTagByID, BulkDetachTag. All substantive. |
| `internal/db/sqlc/bulk.sql.go` | ✓ VERIFIED | `sqlc diff` clean (exit 0, `~/go/bin/sqlc`). |
| `internal/watchlist/bulk.go`, `internal/tags/bulk.go`, `internal/httpserver/bulk.go` | ✓ VERIFIED | Handlers `handleBulkDelete/AddTag/RemoveTag/Preferences`, `BulkRemove`, `BulkUpdatePreferences`, `BulkAttach`, `BulkDetach` all present. |
| `web/app/lib/bulkSelection.ts`, `BulkActionBar.tsx`, `popover.tsx` | ✓ VERIFIED | Present, imported and used by `watchlist.tsx`. |
| `web/app/components/watchlist/WatchlistRow.tsx` | ✓ VERIFIED | Shift fix present; `shiftRef` read and reset in the checkbox `onChange`. |
| `internal/webassets/build/client` | ✓ VERIFIED | `pnpm run build` then `diff -rq web/build/client internal/webassets/build/client` reports identical. Contains `Select all visible` (`watchlist-Ci1EeO6B.js`) and `/watchlist/bulk/tags/remove` (`api-CsK1pMlc.js`). |

### Key Link Verification

| From | To | Status |
|---|---|---|
| `server.go:297-300` | `POST /watchlist/bulk/delete`, `POST /watchlist/bulk/tags/add`, `POST /watchlist/bulk/tags/remove`, `PATCH /watchlist/bulk/preferences` registered inside the gated, CSRF-protected group | ✓ WIRED (401/403 tests pass) |
| `cmd/server/main.go:314` | `httpserver.WithWatchlistBulk(store)` | ✓ WIRED |
| `watchlist.tsx` | `bulkRemoveWatchlist` (760), `bulkAddTag` (650, 708), `bulkRemoveTag` (674), `bulkUpdatePreferences` (730) | ✓ WIRED |
| `WatchlistRow` pointer-down | `shiftRef` then checkbox `onChange` then `onSelectChange(id, next, shift)` | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|---|---|---|---|---|
| Bar `{N} selected` | `effectiveSelection(visible, selectedIds)` | `listWatchlist()` rows | Yes | ✓ FLOWING |
| Remove-tag options, preference tri-states | live `entries` tags and types | `GET /watchlist` | Yes | ✓ FLOWING |
| Post-action list | `entries` | `refresh()` then `GET /watchlist` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks and Gates (run this session)

| Check | Command | Result | Status |
|---|---|---|---|
| Build | `go build ./...` | exit 0 | ✓ PASS |
| go vet | `go vet ./...` | clean | ✓ PASS |
| golangci-lint | `~/go/bin/golangci-lint run` | 0 issues | ✓ PASS |
| Integration suite | `TEST_DATABASE_URL=… go test ./... -count=1` (no `-race`, no cgo on host) | every package `ok`, 0 FAIL | ✓ PASS |
| Coverage gate | `go test ./… -coverprofile … -coverpkg=<COVER_PKGS>` then `go run ./cmd/coverage-report --mode=total` | 90.87% (≥ 80) | ✓ PASS |
| sqlc drift | `sqlc diff` | exit 0, no diff | ✓ PASS |
| Prettier | `pnpm exec prettier --check "**/*.{ts,tsx}"` | all formatted | ✓ PASS |
| Typecheck | `tsc --noEmit` | exit 0 | ✓ PASS |
| Web suite + coverage | `vitest run --coverage` | 34 files, 782 tests pass (+2 vs. prior); 95.55 / 91.14 / 96.47 / 97.37 | ✓ PASS |
| Shift fix tests | `vitest run … -t "auto-repeats"` and `-t "abandoned Shift"` | 1 passed each | ✓ PASS |
| Bundle freshness | fresh build vs. `internal/webassets/build/client` | identical | ✓ PASS |
| BULK-04 atomicity | Rolled-back psql tx in `drop-tracker-postgres-1`: 3 artists/watchlist rows, a BEFORE DELETE trigger raising on one, then the production DELETE shape | `ERROR: injected`, 3 rows remain after savepoint rollback, 0 rows leaked after the final ROLLBACK. `watchlist` has no inbound FKs. | ✓ PASS |
| CI | `gh run view 39298714930` | head `e2b0499`, conclusion success. vet, lint, test (`-race`), frontend-test, migration-check, n1-boot, build-scan, gitleaks and trivy-fs all success. pr-title, coverage-comment and release skipped. | ✓ PASS |
| Blast radius | `git diff` against the phase baselines over migrations, package.json/lock, notifier/detection/discord/musicbrainz/deezer/poller/events, events/watchlist/tags SQL, `service.go` x2, `httpserver/watchlist.go` | empty | ✓ PASS |

Note: the previous report's blast-radius line also listed `go.mod/sum` as unchanged. They now differ only by the `x/text` and `x/sync` bump in `e2b0499`, which is outside phase 26 scope and passes all gates and CI including trivy-fs.

### Probe Execution

No probes are declared and there is no `scripts/*/tests/probe-*.sh`. Skipped.

### Requirements Coverage

| Requirement | Source plans | Status | Evidence |
|---|---|---|---|
| BULK-01 | 26-03, 26-04, 26-06, 26-12 | ✓ SATISFIED | SC1, UAT #2, #3, #7 |
| BULK-02 | 26-02, 26-07, 26-08, 26-09, 26-11, 26-12 | ✓ SATISFIED | SC2, UAT #4, #6, #7 |
| BULK-03 | 26-05, 26-10, 26-12 | ✓ SATISFIED | SC3, UAT #5 |
| BULK-04 | 26-01, 26-03, 26-06, 26-12 | ✓ SATISFIED | SC4, UAT #1, #8, #9 |
| BULK-05 | 26-03, 26-06, 26-08, 26-10, 26-11, 26-12 | ✓ SATISFIED | SC5, UAT #7 |

All 5 IDs mapped to Phase 26 in REQUIREMENTS.md (checked, "Complete") are claimed by plans. None are orphaned.

### Prohibitions

| Plan | Prohibition | Tier | Disposition |
|---|---|---|---|
| 26-01 | MUST NOT leave a partial removal | test | ✓ Resolved. There is still no committed failure-injection test, so the evidence is structural (one DELETE statement, no watchlist FKs) plus the rolled-back failure injection re-run this session. The user accepted this explicitly in UAT #8. Optional hardening: commit a Go test that forces the DELETE to fail and asserts every row survives. |
| 26-02 | MUST NOT skip an at-cap artist silently | test | ✓ `TestService_BulkAttach` (cap with name), `TestBulkTags_AddSkipWireShape` |
| 26-03 | MUST NOT remove without showing the count or drop the disclosure | test | ✓ dialog-copy tests |
| 26-03/08/10/11 | MUST NOT issue more than one request per bulk action | test | ✓ `toHaveBeenCalledTimes(1)` tests; UAT #5 and #7 confirm in the network panel |
| 26-04 | MUST NOT send, count or announce hidden selections | test | ✓ filtered-view removal test |
| 26-05 | MUST NOT change unnamed types or the note | test | ✓ `TestService_BulkUpdatePreferences`, `TestBulk_PreferencesValidation` |
| 26-08 | MUST NOT leave a cap-skipped artist out of the toast | test | ✓ at-cap naming test; UAT #4 |
| 26-10 | MUST NOT send untouched or returned-to-initial types, full sets, or notes | test | ✓ one-type delta test; UAT #5 |
| 26-11 | MUST NOT re-add during Undo outside `applied` | test | ✓ Undo-scope test |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| (phase-modified files) | n/a | TBD / FIXME / XXX / TODO / HACK / placeholder | none found | n/a |
| `web/app/routes/watchlist.tsx` | 696, 705 | REVIEW WR-01: Undo re-adds by stale client-side tag name | ⚠️ Warning (advisory, non-blocking) | Affects only Undo after a concurrent rename. Fix or accept before ship. |
| `internal/watchlist/bulk_test.go` | 413-478 | REVIEW WR-02: fixed 300ms concurrency window | ⚠️ Warning (advisory) | Weak regression guard; the behavior holds. |
| `internal/tags/bulk_test.go` | 239-297 | REVIEW WR-03: reversed-list deadlock test can't fail without `ORDER BY` | ⚠️ Warning (advisory) | Weak regression guard; ordering holds structurally. |

### Human Verification Required

None outstanding. All 9 human items from the previous report are resolved in `26-UAT.md` (9 passed, 0 issues).

### Gaps Summary

No gaps. Every roadmap success criterion and BULK-01 to BULK-05 is backed by substantive, wired, tested code, and the only defect found in UAT (Shift+click range) is fixed, covered by two new tests, shipped in the embedded bundle, and re-confirmed by the user in a real browser. Residual non-blocking items: REVIEW WR-01 (stale-name Undo), the weak concurrency regression guards WR-02 and WR-03, and an optional committed negative test for the BULK-04 atomicity prohibition.

---

_Verified: 2026-10-19T18:11:43Z_
_Verifier: Claude (gsd-verifier)_
