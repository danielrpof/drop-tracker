---
phase: 26-bulk-edit-remove
verified: 2026-10-17T00:15:19Z
status: human_needed
score: 157/165 must-haves verified (5/5 roadmap success criteria; 152/160 plan truths, 8 held-out backstop truths routed to human)
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
covered_digest: "v1:sha256:ada28c3f535e1699577cabd9f012503c09071254ef7b2fbf2b2287b7679cb4ca"
behavior_unverified: 0
overrides_applied: 0
coincidental_reliance_items:
  - truth: "[26-05] A concurrent single-axis write committed while the bulk statement waits is not lost, and a row deleted concurrently becomes `gone`."
    reason: incidental-ordering
    harden: "TestService_BulkUpdatePreferences_ConcurrentAxisWriteIsNotLost and _ConcurrentDeleteIsGone wait a fixed 300ms instead of confirming the bulk session is blocked. Poll pg_blocking_pids before committing the holder, as TestService_BulkAttach_WaitsOnConcurrentTenthLink already does (REVIEW WR-02)."
  - truth: "[26-02] Two overlapping bulk adds of different tags with reversed id lists both succeed with no 40P01."
    reason: fixture-only
    harden: "The fixture seeds watchlist ids in artist-id order and ANY() ignores array order, so the test would pass with `ORDER BY a.id` removed. Seed watchlist entries in reverse artist order, or reword the ADR 0004 'Pinned by' line (REVIEW WR-03). The truth itself holds structurally: LockRows sits above the Sort, so rows lock in artist-id order."
human_verification:
  - test: "Bulk remove confirmation with long names: select three artists whose names are 60+ unbroken characters and open Remove (26-03 backstop)."
    expected: "The description wraps all three names inside the dialog with no overflow, and the notes/tags/pending-alert/seed-mode sentence is fully visible."
    why_human: "Real text wrapping is not computed by jsdom."
  - test: "Shift+click range through the 44px label: click one card checkbox, Shift+click a card three rows down on the label area, then repeat after changing sort (26-04 backstop)."
    expected: "The inclusive range in visible order is selected. After the sort change, the anchor is reset and only the clicked card toggles."
    why_human: "Real pointer events on the label hit area are a browser behavior (RESEARCH A1)."
  - test: "Bar layout and clearance at 1280px and 375px with 30+ artists: scroll to the bottom, Tab through card checkboxes, trigger a failure toast, and select every artist so the count reaches 3-4 digits (26-06 backstop)."
    expected: "The last card clears the bar, a focused checkbox is never behind the bar, toasts stack above the bar at both widths, and at 375px the bar is one row of icon-only 44px controls with no horizontal scroll and `1000 selected` fully visible."
    why_human: "Layout, scroll-padding, toast offset and viewport fit are not computed by jsdom."
  - test: "Bulk add-tag long text: select three long-named artists, one of them at 10 tags, and bulk-add a 32-character tag at 1280px and 375px (26-08 backstops)."
    expected: "The input scrolls the long name inside itself, the counter never pushes Cancel off the bar, and the warning toast wraps the names and the tag with no clipping while naming the at-cap artist."
    why_human: "Visual overflow and wrapping need a real browser."
  - test: "Preferences popover at 375px: select three artists with different release types, open Preferences, leave one mixed type untouched, Apply, then reopen (26-10 backstop)."
    expected: "The popover fits on screen. The mixed type shows a dash with aria-checked=mixed, and the network panel shows one PATCH whose body omits the untouched type."
    why_human: "Popover collision handling and viewport fit need a real browser."
  - test: "Remove-tag picker long text: give two selected artists a 32-character unbroken tag, open Remove tag at 1280px and 375px, and arrow through a 20-tag list (26-11 backstop)."
    expected: "The tag name truncates with an ellipsis, `{c} of {N}` stays fully visible, the list caps at about 288px with the highlighted option in view, and the inline row wraps to a second line at 375px."
    why_human: "Truncation and scroll-into-view are visual."
  - test: "End-to-end UAT walk of the 26-UI-SPEC 'Verification items to carry into UAT' list against the built binary (26-12): one request per bulk action, select-all under a filter, hidden-selection counts, cap-skip naming, indeterminate preferences, remove-confirm copy, toasts above the bar, focus never behind the bar, 3:1 contrast on unchecked and indeterminate boxes, and Undo on a bulk remove-tag toast."
    expected: "Every item behaves as UI-SPEC and 26-VALIDATION.md 'Manual-Only Verifications' describe."
    why_human: "Real-browser network panel, contrast measurement, focus and feel."
  - test: "Unverified test-tier prohibition (26-01, BULK-04): 'MUST NOT leave a partial removal'. No committed test injects a mid-statement failure into BulkDeleteWatchlist."
    expected: "Accept the structural guarantee (one DELETE statement, no watchlist FKs) plus this verifier's rolled-back failure-injection spot-check, or add a test that forces the DELETE to fail and asserts every row survives."
    why_human: "Test-tier prohibition with no wired negative test: flagged rather than passed (ADR-550 D4 fail-closed)."
  - test: "Confirm `go test -race ./...` is green in CI for this branch."
    expected: "The CI integration job passes with -race. It could not run locally because cgo is unavailable on this Windows host."
    why_human: "Race-detector evidence exists only in CI."
---

# Phase 26: Bulk Edit & Remove Verification Report

**Phase Goal:** The user can change many artists at once (tags, release-type filters, mutes, or removal) by selecting them on the Watchlist. Every change applies in one step, and removal requires confirming an explicit count.
**Verified:** 2026-10-17T00:15:19Z
**Status:** human_needed
**Re-verification:** No. This is the initial verification.

## Goal Achievement

### Roadmap Success Criteria

| # | Success criterion | Status | Evidence |
|---|---|---|---|
| 1 | Select individual artists and "select all visible", which selects exactly what search and filters show and never hidden artists. The count is always on screen. | ✓ VERIFIED | Selection is derived, not trimmed: `effective = effectiveSelection(visible, selectedIds)` (`watchlist.tsx:809`), where `visible` uses the same `selectVisible(entries, {...view, q: query}, stickyIds)` the cards render. Select-all uses `toggleAllVisible(visible, selectedIds)`. The bar reads `{N} selected` from `effective.length`. Tests: "selects only the filtered view and removes exactly it" (sends `[1]` with a hidden selection present), "keeps a hidden selection inert…", "keeps a hidden selection when select all is checked then unchecked", "disables select all when the filter matches nothing". |
| 2 | Add or remove a tag on all selected artists in one action. Artists that already have (or lack) the tag are unaffected, and an artist pushed past the 10-tag cap is reported by name. | ✓ VERIFIED | `BulkAttach` runs in one tx: `GetOrCreateTag`, then the ordered `FOR NO KEY UPDATE OF a` pre-lock, then a count-guarded insert with `ON CONFLICT DO NOTHING` and a `had` CTE. A cap skip carries `row.Name` (`tags/bulk.go:88`). `BulkDetach` deletes only that tag's links on the selected artists. The UI names cap skips through `formatNameList` (`watchlist.tsx:596-640`). Tests: `TestService_BulkAttach`, `_AllCappedLeavesNoTag`, `TestService_BulkDetach`, "warns with the at-cap artist named…", "names at-cap artists in visible order…", "removes one carried tag from the selection in one request". |
| 3 | Set release-type filters and muted event types on all selected artists in one action, and every selected artist shows the new settings afterwards. | ✓ VERIFIED | `BulkUpdateWatchlistPreferences` is one statement with a MATERIALIZED locked CTE, a per-type (current ∪ set) − unset rebuild in canonical order, COALESCE nil guards, and an `IS DISTINCT FROM` update. The popover sends only changed types via `buildAxisDelta`, then `markStickyMany(applied)` and `refresh()`. Tests: `TestService_BulkUpdatePreferences`, `_NilSlicesNeverWipe`, `TestBulk_PreferencesEndToEnd`, "mutes one type on a mixed selection with a one-type delta", "sends only Deluxe when only Deluxe was toggled, in one request". |
| 4 | Removing the selection first asks for confirmation stating how many artists will be removed. Confirming removes all of them, or none if any part fails. | ✓ VERIFIED | A route-level `ConfirmDialog` (`variant="destructive"`) shows the title `Remove {n} artists from your watchlist?` and the action `Remove {n} artists` / `Remove 1 artist`, with the D-17 disclosure sentence. The backend is a single `DELETE … WHERE id = ANY(...) RETURNING id` with no watchlist FKs. Verifier spot-check: a BEFORE DELETE trigger raising on the last of 3 ids made the production-shaped statement fail, and all 3 rows remained (fully rolled back). UI test "keeps the cards and selection when the request fails". See the flagged prohibition under Human Verification. |
| 5 | After any bulk action the selection clears and the Watchlist shows the result immediately, with no page reload and no checkbox left pointing at a removed artist. | ✓ VERIFIED | `finishBulkSuccess()` runs `setSelectedIds(EMPTY_IDS)` after every success: remove via `handleRemoveDialogOpenChange`, and add-tag, remove-tag and preferences via the bar's `onSucceeded`. Remove filters the removed and already-gone ids out of `entries` locally and then calls `refresh()` (no skeleton). Test "removes two checked cards with one confirmed request" asserts the bar unmounts, the removed checkboxes are gone, the remaining box is unchecked, and the list was refetched once. |

### Plan Must-Have Truths (12 plans, 160 truths)

| Plan | Truths | Verified | Backstop → human | Key evidence |
|---|---|---|---|---|
| 26-01 bulk delete | 15 | 14 | 1 (race backstop) | `TestService_BulkRemove` (dup/order/gone/repeat/empty), `_KeepsArtistTagsAndPendingEvents`, `TestBulk_Gated401NoCookie`, `_GatedForbiddenWithoutCSRFHeader`, `_UnavailableWithoutOption`, `_DeleteReachesBulkHandler`, `_DeleteIDValidation` (1000/1001/64KiB) |
| 26-02 bulk add-tag | 16 | 16 | 0 | `TestService_BulkAttach*` (4 cases incl. concurrency), `TestBulkTags_Add*` (7 cases), ADR 0004 amendment present |
| 26-03 checkboxes/bar/confirm | 18 | 17 | 1 (60-char wrap) | "words the dialog for a single artist", "lists three names and a remainder…", "counts already-gone ids…", "reports the server's id cap…" |
| 26-04 select-all/shift/lock | 14 | 13 | 1 (shift via label) | `bulkSelection.test.ts`, the shift-range tests, "locks every selection control while a bulk request is in flight" |
| 26-05 bulk preferences | 12 | 12 | 0 | `TestService_BulkUpdatePreferences*` (5), `TestBulk_Preferences*` (6) |
| 26-06 focus/announce/clearance | 14 | 13 | 1 (375px fit) | "Watchlist focus after bulk actions" (7), "selection announcements" (7), "bar clearance" (2), `ConfirmDialog.test.tsx` |
| 26-07 bulk remove-tag | 11 | 11 | 0 | `TestService_BulkDetach`, `_UnknownTag`, `TestBulkTags_Remove*` (5) |
| 26-08 bulk add-tag UI | 12 | 10 | 2 (long text) | "Watchlist bulk add tag" (10 route tests) |
| 26-09 entry-free TagCombobox | 10 | 10 | 0 | `TagCombobox.test.tsx`, `BulkActionBar.test.tsx` inline cases |
| 26-10 popover/tri-state | 16 | 15 | 1 (375px fit) | "Watchlist bulk preferences" (6), `buildAxisDelta`/`cycleTriState` unit tests; `web/package.json` and the lockfile are unchanged |
| 26-11 remove-tag UI + Undo | 17 | 16 | 1 (truncation) | "Watchlist bulk remove tag" (11), incl. "restores the tag to exactly the applied artists…" and "gives a bulk artist removal toast no Undo" |
| 26-12 rebuild + DoD | 5 | 5 | 0 | Fresh `pnpm run build` is byte-identical to `internal/webassets/build/client`. All gates green (below). Blast-radius diff is empty. |

**Score:** 157/165 (5 SCs + 152 plan truths verified; 8 `verification: backstop` truths are `insufficient_spec` and routed to human; 0 present-behavior-unverified). Two truths are VERIFIED with a coincidental-reliance advisory (see frontmatter). This is advisory only and does not affect the score.

Plan truths were verified through the test enumeration and the green suites below. The SC-critical paths were also read end to end: `queries/bulk.sql`, `internal/{watchlist,tags,httpserver}/bulk.go`, `watchlist.tsx`, `bulkSelection.ts` and the BulkActionBar wiring.

### Required Artifacts

| Artifact | Status | Details |
|---|---|---|
| `queries/bulk.sql` | ✓ VERIFIED | 5 queries: BulkDeleteWatchlist, LockWatchlistArtistsForBulkTag, BulkAttachTag, BulkUpdateWatchlistPreferences, GetTagByID, BulkDetachTag |
| `internal/db/sqlc/bulk.sql.go` | ✓ VERIFIED | `sqlc diff` is clean (v1.31.1 matches the Makefile pin) |
| `internal/watchlist/bulk.go` | ✓ VERIFIED | BulkRemove, BulkUpdatePreferences, ErrPreferenceConflict |
| `internal/tags/bulk.go` | ✓ VERIFIED | BulkAttach (tx, ordered pre-lock, rollback when nothing applied), BulkDetach (404 before the delete) |
| `internal/httpserver/bulk.go` | ✓ VERIFIED | 4 handlers, shared id bounds and dedupe, MaxBytesReader, 503 guards |
| `web/app/lib/bulkSelection.ts` | ✓ VERIFIED | All helpers exported and used by the route and bar |
| `web/app/components/watchlist/BulkActionBar.tsx` | ✓ VERIFIED | region, Add tag / Remove tag / Preferences / Remove / Clear, clearance variable |
| `web/app/components/ui/popover.tsx` | ✓ VERIFIED | Vendored; no dependency change |
| `internal/webassets/build/client` | ✓ VERIFIED | Matches a fresh build. Contains `Select all visible`, `Preferences for`, and `/watchlist/bulk/tags/remove` |

### Key Link Verification

| From | To | Status |
|---|---|---|
| `server.go` registerDataRoutes | 4 bulk handlers (`r.Post/Patch("/watchlist/bulk/...")`) | ✓ WIRED (inside the gated and CSRF group, which the 401/403 tests prove) |
| `cmd/server/main.go` | `httpserver.WithWatchlistBulk(store)` | ✓ WIRED |
| `BulkRemove` | `s.q.BulkDeleteWatchlist(` | ✓ WIRED |
| `BulkAttach` | `qtx.LockWatchlistArtistsForBulkTag(` before `qtx.BulkAttachTag(`, then `tx.Commit` only when something applied | ✓ WIRED |
| `BulkUpdatePreferences` | `AllReleaseTypes: ReleaseTypes` and the COALESCE guard | ✓ WIRED |
| `watchlist.tsx` | `bulkRemoveWatchlist(` / `bulkAddTag(` / `bulkRemoveTag(` / `bulkUpdatePreferences(` / `undoRemoveTag(` | ✓ WIRED |
| `api.ts` | `apiFetch` (injects X-Requested-With) to `/watchlist/bulk/*` | ✓ WIRED |
| `BulkActionBar` | `useTagVocabulary()`, `buildAxisDelta(`, `onSucceeded()` | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|---|---|---|---|---|
| BulkActionBar `{N} selected` | `selected` prop | `effectiveSelection(visible, selectedIds)` over `listWatchlist()` rows | Yes | ✓ FLOWING |
| Remove-tag options | `removableTags(selected)` | Live entries' `tags` | Yes | ✓ FLOWING |
| Preferences tri-states | `initialTriStates(selected, axis, types)` | Live entries' `release_types` / `muted_event_types` | Yes | ✓ FLOWING |
| Post-action list | `entries` | `refresh()` → `GET /watchlist` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks and Gates

| Check | Command | Result | Status |
|---|---|---|---|
| go vet | `go vet ./...` | clean | ✓ PASS |
| golangci-lint | `~/go/bin/golangci-lint run` | 0 issues | ✓ PASS |
| Integration suite (no -race, no cgo) | `TEST_DATABASE_URL=… go test ./... -count=1` | 26 packages ok, 0 FAIL, exit 0 | ✓ PASS |
| Coverage gate | re-run with the expanded `COVER_PKGS`, then `go run ./cmd/coverage-report --mode=total` | 90.72% (≥ 80). The exit 1 on this run came from the missing local `covdata` tool for the test-less `internal/testutil` package; 25 ok, 0 FAIL | ✓ PASS |
| sqlc drift | `sqlc diff` | clean | ✓ PASS |
| Prettier | `pnpm exec prettier --check "**/*.{ts,tsx}"` | all formatted | ✓ PASS |
| Typecheck | `tsc --noEmit` | exit 0 | ✓ PASS |
| Web suite + coverage | `vitest run --coverage` | 34 files, 780 tests pass; 95.6 / 91.2 / 96.47 / 97.37 | ✓ PASS |
| Embedded bundle freshness | `pnpm run build` then `diff -rq web/build/client internal/webassets/build/client` | identical | ✓ PASS |
| Blast radius | `git diff --name-only f793ffc` (and `5c0d214`) over migrations, go.mod/sum, package.json/lock, notifier/detection/discord/musicbrainz/deezer/poller/events, events/watchlist/tags SQL, service.go ×2, httpserver/watchlist.go | empty | ✓ PASS |
| BULK-04 atomicity | Rolled-back psql tx in `drop-tracker-postgres-1`: 3 rows, a trigger raising on one id, then the production DELETE shape | ERROR, 3 rows remain | ✓ PASS |
| `go test -race` | not runnable (no cgo on host) | n/a | ? SKIP → human (CI) |

### Probe Execution

No probes are declared and there is no `scripts/*/tests/probe-*.sh`. Skipped.

### Requirements Coverage

| Requirement | Source plans | Description | Status | Evidence |
|---|---|---|---|---|
| BULK-01 | 26-03, 26-04, 26-06, 26-12 | Select multiple artists, incl. "select all visible" | ✓ SATISFIED | SC1 |
| BULK-02 | 26-02, 26-07, 26-08, 26-09, 26-11, 26-12 | Add or remove a tag on all selected in one action | ✓ SATISFIED | SC2 |
| BULK-03 | 26-05, 26-10, 26-12 | Set release-type filters and muted event types on all selected in one action | ✓ SATISFIED | SC3 |
| BULK-04 | 26-01, 26-03, 26-06, 26-12 | Remove all selected after a count confirmation; all-or-nothing | ✓ SATISFIED (prohibition flagged) | SC4 |
| BULK-05 | 26-03, 26-06, 26-08, 26-10, 26-11, 26-12 | Selection clears and the list reflects the result without reload | ✓ SATISFIED | SC5 |

All 5 IDs mapped to Phase 26 in REQUIREMENTS.md are claimed by at least one plan. None are orphaned.

### Prohibitions

| Plan | Prohibition | Tier | Disposition |
|---|---|---|---|
| 26-01 | MUST NOT leave a partial removal | test | ⚠️ Flagged, unverified-prohibition. No committed failure-injection test; the evidence is structural plus the verifier's spot-check. Human review recommended. |
| 26-02 | MUST NOT skip an at-cap artist silently | test | ✓ Enforced by `TestService_BulkAttach` (cap with name) and `TestBulkTags_AddSkipWireShape` |
| 26-03 | MUST NOT remove without showing the count or drop the disclosure | test | ✓ "words the dialog…" and "lists three names…" assert the full sentence |
| 26-03/08/10/11 | MUST NOT issue more than one request per bulk action | test | ✓ `toHaveBeenCalledTimes(1)` in the remove, add-tag, preferences, remove-tag and Undo tests |
| 26-04 | MUST NOT send, count or announce hidden selections | test | ✓ "selects only the filtered view and removes exactly it" |
| 26-05 | MUST NOT change unnamed types or the note | test | ✓ `TestService_BulkUpdatePreferences`, `TestBulk_PreferencesValidation` (note over-post → 400) |
| 26-08 | MUST NOT leave a cap-skipped artist out of the toast | test | ✓ "warns with the at-cap artist named…" |
| 26-10 | MUST NOT send untouched or returned-to-initial types, full sets, or notes | test | ✓ "sends only Deluxe when only Deluxe was toggled" |
| 26-11 | MUST NOT re-add during Undo outside `applied` | test | ✓ "restores the tag to exactly the applied artists and leaves a newer selection alone" |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| (all phase-modified files) | n/a | TBD / FIXME / XXX | none found | n/a |
| (all phase-modified files) | n/a | TODO / HACK / placeholder stubs | none found | n/a |
| `web/app/routes/watchlist.tsx` | 696 | REVIEW WR-01: Undo re-adds by the client-captured tag name. A rename during the 8s window (or in another tab) makes Undo create a new tag under the old name. | ⚠️ Warning | Affects only the U4 Undo add-on after a concurrent rename. It does not undermine SC2: bulk add and remove themselves are correct, and the Undo prohibition (applied ids only) holds. Fix before ship is recommended (the server already loads the tag in `BulkDetach`). |
| `internal/watchlist/bulk_test.go` | 413-478 | REVIEW WR-02: fixed 300ms concurrency window | ⚠️ Warning (advisory) | A weak regression guard. The truth holds (EvalPlanQual on the locked CTE); recorded as coincidental-reliance. |
| `internal/tags/bulk_test.go` | 239-297 | REVIEW WR-03: the reversed-list deadlock test can't fail without `ORDER BY` | ⚠️ Warning (advisory) | A weak regression guard. The ordering holds structurally; recorded as coincidental-reliance. The ADR "Pinned by" claim overstates it. |
| `web/app/routes/watchlist.tsx` | 625-639 | REVIEW IN-02: an all-gone add-tag shows success "Added … to 0 artists." | ℹ️ Info | Planned and tested ("reports zero when every selected artist was already removed"). |

### Human Verification Required

See the `human_verification` frontmatter. There are 9 items:
- 6 real-browser layout and long-text backstops (26-03, 26-04, 26-06, 26-08, 26-10, 26-11)
- the 26-12 end-to-end UAT walk
- the flagged BULK-04 test-tier prohibition
- CI `-race` confirmation

### Gaps Summary

No blocking gaps. Every roadmap success criterion and requirement (BULK-01 to BULK-05) is backed by substantive, wired code and passing tests. The test suites cover backend per-id outcomes, gating, CSRF, bounds and concurrency, and frontend selection derivation, one-request actions, confirm copy, failure paths and focus.

The status is `human_needed`, not `passed`, for three reasons:
1. 8 held-out visual and interaction backstop truths need a real browser.
2. The BULK-04 "no partial removal" prohibition is test-tier with no committed negative test. The structural guarantee and a rolled-back spot-check support it, but policy requires flagging it.
3. `-race` evidence exists only in CI.

REVIEW WR-01 (stale-name Undo) is a real but narrow defect in the Undo add-on and should be fixed or explicitly accepted before ship. WR-02 and WR-03 weaken regression protection, not current behavior.

---

_Verified: 2026-10-17T00:15:19Z_
_Verifier: Claude (gsd-verifier)_
