---
phase: "26"
slug: "bulk-edit-remove"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-10-15"
---

# Phase 26 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` against real Postgres (`testutil.NewIsolatedTestPool`); Vitest 4.1.10 + Testing Library (jsdom) |
| **Config file** | `Makefile` (`test-integration`), `web/vitest.config.ts` |
| **Quick run command** | `TEST_DATABASE_URL='postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable' go test ./internal/tags ./internal/watchlist ./internal/httpserver -run 'Bulk' -race -count=1` / `corepack pnpm --dir web exec vitest run <touched test files> --coverage.enabled=false` |
| **Full suite command** | `make test && make coverage-gate && make sqlc-check` and `corepack pnpm --dir web test` |
| **Estimated runtime** | ~120 seconds |

---

## Sampling Rate

- **After every task commit:** Run the matching quick-run command (plus `go vet ./...` / `golangci-lint run` for Go, `prettier --write` then touched vitest files for web)
- **After every plan wave:** Run the full suite command (`make db-up` first)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

Filled in by the planner (2026-10-15). `TDB` below stands for `TEST_DATABASE_URL='postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable'`,
and `VT` for `corepack pnpm --dir web exec vitest run … --coverage.enabled=false`. No `-race` on this Windows box (CI's Linux job is the race gate).
Every test file is created by the task that first needs it, so there is no separate Wave 0 scaffold plan.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 26-01-T1 | 01 | 1 | BULK-04 | T-26-06 | One DELETE statement; already-gone reported | integration (http e2e) | `TDB go test ./internal/httpserver/ -run 'TestBulk_DeleteEndToEnd' -count=1 -v` + sqlc-check chain | ✅ | ✅ green |
| 26-01-T2 | 01 | 1 | BULK-04 | T-26-06 | Artists, tags, pending events survive; note goes | integration | `TDB go test ./internal/watchlist/ -run 'TestService_BulkRemove' -count=1 -v` | ✅ | ✅ green |
| 26-01-T3 | 01 | 1 | BULK-04 | T-26-01..05 | 401/403 with zero store calls; 503; id/body bounds | http | `TDB go test ./internal/httpserver/ -run 'TestBulk_' -count=1 -v` | ✅ (T1) | ✅ green |
| 26-02-T1 | 02 | 2 | BULK-02 | T-26-08 | Pre-lock + guarded insert; cap skip named | integration (http e2e) | `TDB go test ./internal/httpserver/ -run 'TestBulkTags_AddEndToEnd' -count=1 -v` + sqlc-check chain | ✅ | ✅ green |
| 26-02-T2 | 02 | 2 | BULK-02 | T-26-08..10 | No deadlock; waits on concurrent 10th link; no orphan tag | integration | `TDB go test ./internal/tags/ -run 'TestService_BulkAttach' -count=1 -v` | ✅ | ✅ green |
| 26-02-T3 | 02 | 2 | BULK-02 | T-26-11..14 | Gate/CSRF/503/precedence/wire; ADR 0004 amended | http + doc | `TDB go test ./internal/httpserver/ -run 'TestBulkTags_\|TestTags_' -count=1 -v` | ✅ | ✅ green |
| 26-03-T1 | 03 | 2 | BULK-01, BULK-04, BULK-05 | T-26-15, T-26-16, T-26-18 | One confirmed request; route-hosted dialog | route | `VT app/routes/watchlist.bulk.test.tsx` | ✅ | ✅ green |
| 26-03-T2 | 03 | 2 | BULK-04, BULK-05 | T-26-18 | Copy, gone note, failure, too-many | unit + route | `VT app/lib/format.test.ts app/lib/api.test.ts app/routes/watchlist.bulk.test.tsx app/routes/watchlist.test.tsx` + full web suite | ✅ | ✅ green |
| 26-04-T1 | 04 | 3 | BULK-01 | T-26-17 | Select all visible under a filter sends only visible ids | route | `VT app/routes/watchlist.bulk.test.tsx app/components/watchlist/PreferenceToggles.test.tsx` | ✅ | ✅ green |
| 26-04-T2 | 04 | 3 | BULK-01 | T-26-17 | Hidden selections inert; select-all tri-state; Clear empties hidden | unit + route | `VT app/lib/bulkSelection.test.ts app/routes/watchlist.bulk.test.tsx` | ✅ | ✅ green |
| 26-04-T3 | 04 | 3 | BULK-01 | T-26-45 | Shift ranges in visible order; in-flight lock | unit + route | `VT app/lib/bulkSelection.test.ts app/routes/watchlist.bulk.test.tsx app/routes/watchlist.test.tsx` + full web suite | ✅ | ✅ green |
| 26-05-T1 | 05 | 3 | BULK-03 | T-26-21 | Delta CTE; nil-slice guard end to end | integration (http e2e) | `TDB go test ./internal/httpserver/ -run 'TestBulk_PreferencesEndToEnd' -count=1 -v` + sqlc-check chain | ✅ | ✅ green |
| 26-05-T2 | 05 | 3 | BULK-03 | T-26-20..22 | Canonical order; no-op rows; D-21 sentinels; concurrency | integration | `TDB go test ./internal/watchlist/ -run 'TestService_BulkUpdatePreferences' -count=1 -v` | ✅ | ✅ green |
| 26-05-T3 | 05 | 3 | BULK-03 | T-26-19, T-26-23 | Over-post note → 400; gate; wire | http | `TDB go test ./internal/httpserver/ -run 'TestBulk_' -count=1 -v` | ✅ | ✅ green |
| 26-06-T1 | 06 | 4 | BULK-04, BULK-05 | T-26-25 | Focus fallback chain; finalFocus at close | route | `VT app/routes/watchlist.bulk.test.tsx app/components/common/ConfirmDialog.test.tsx app/components/watchlist/ManageTagsDialog.test.tsx` | ✅ | ✅ green |
| 26-06-T2 | 06 | 4 | BULK-01 | T-26-26 | Announcements (counts only); Esc clears | route | `VT app/routes/watchlist.bulk.test.tsx app/routes/watchlist.test.tsx` | ✅ | ✅ green |
| 26-06-T3 | 06 | 4 | BULK-01 | T-26-27 | Clearance variable; toasts above bar; scroll padding | component | `VT app/components/watchlist/BulkActionBar.test.tsx app/routes/watchlist.bulk.test.tsx` + full web suite | ✅ | ✅ green |
| 26-07-T1 | 07 | 4 | BULK-02 | T-26-30 | Detach scoped to selected carriers; tag survives | integration (http e2e) | `TDB go test ./internal/httpserver/ -run 'TestBulkTags_RemoveEndToEnd' -count=1 -v` + sqlc-check chain | ✅ | ✅ green |
| 26-07-T2 | 07 | 4 | BULK-02 | T-26-28, T-26-29 | 404 tag_not_found; gate; wire | integration + http | `TDB go test ./internal/tags/ ./internal/httpserver/ -run 'TestService_BulkDetach\|TestBulkTags_Remove' -count=1 -v` | ✅ | ✅ green |
| 26-08-T1 | 08 | 5 | BULK-02, BULK-05 | T-26-33, T-26-34 | One add-tag request from the inline picker | route | `VT app/routes/watchlist.bulk.test.tsx app/components/watchlist/BulkActionBar.test.tsx app/components/watchlist/TagCombobox.test.tsx app/components/watchlist/TagChips.test.tsx` | ✅ | ✅ green |
| 26-08-T2 | 08 | 5 | BULK-02, BULK-05 | T-26-32, T-26-35 | Cap names in visible order; toast matrix; sticky | route | `VT app/routes/watchlist.bulk.test.tsx app/lib/api.test.ts app/routes/watchlist.test.tsx` + full web suite | ✅ | ✅ green |
| 26-09-T1 | 09 | 6 | BULK-02 | T-26-46 | Entry-free TagCombobox; card copy unchanged | component + route | `VT app/components/watchlist/TagCombobox.test.tsx app/components/watchlist/TagChips.test.tsx app/routes/watchlist.bulk.test.tsx` | ✅ | ✅ green |
| 26-09-T2 | 09 | 6 | BULK-02 | T-26-47 | Inline mode labels, Esc/Cancel, pending, remember | unit + component | `VT app/components/watchlist/BulkActionBar.test.tsx app/lib/bulkSelection.test.ts app/components/watchlist/TagCombobox.test.tsx` + full web suite | ✅ | ✅ green |
| 26-10-T1 | 10 | 7 | BULK-03, BULK-05 | T-26-37, T-26-SC | Only changed types sent; lockfile unchanged | route | `VT app/routes/watchlist.bulk.test.tsx` + `git diff --quiet -- web/package.json web/pnpm-lock.yaml` | ✅ | ✅ green |
| 26-10-T2 | 10 | 7 | BULK-03 | T-26-37 | Tri-state machine; popover contract | unit + component | `VT app/lib/bulkSelection.test.ts app/components/watchlist/BulkActionBar.test.tsx` | ✅ | ✅ green |
| 26-10-T3 | 10 | 7 | BULK-03, BULK-05 | T-26-36, T-26-38 | Failure keeps choices; sticky under preference filters | route | `VT app/routes/watchlist.bulk.test.tsx app/lib/api.test.ts app/routes/watchlist.test.tsx` + full web suite | ✅ | ✅ green |
| 26-11-T1 | 11 | 8 | BULK-02, BULK-05 | T-26-40 | One remove-tag request; Undo-bearing toast | route | `VT app/routes/watchlist.bulk.test.tsx` | ✅ | ✅ green |
| 26-11-T2 | 11 | 8 | BULK-02 | T-26-41, T-26-42 | Undo only to `applied`; 404 → failure + refresh | route | `VT app/routes/watchlist.bulk.test.tsx` | ✅ | ✅ green |
| 26-11-T3 | 11 | 8 | BULK-02 | T-26-39 | Options, counts, empty states, final bar order | unit + component | `VT app/lib/bulkSelection.test.ts app/components/watchlist/BulkActionBar.test.tsx app/lib/api.test.ts` + full web suite | ✅ | ✅ green |
| 26-12-T1 | 12 | 9 | BULK-01..05 | T-26-43 | Embedded bundle carries the phase UI | static | `make web` (or its recipe) + bundle string greps + `go build ./...` | n/a | ✅ green |
| 26-12-T2 | 12 | 9 | BULK-01..05 | T-26-SC2, T-26-44 | Full DoD; blast radius vs `f793ffc` | full suite | `make db-up` + vet + lint + `make test` (recipe minus `-race` where cgo is off) + coverage-gate + sqlc-check, web DoD chain, blast-radius diff | n/a | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Each item below is created inside the task that first needs it. There is no standalone scaffold plan, because tracer-first
puts every test file next to its code.

- [x] `queries/bulk.sql` + `sqlc generate`: 26-01-T1 (then extended by 26-02-T1, 26-05-T1, and 26-07-T1)
- [x] `internal/httpserver/bulk_test.go`: 26-01-T1; `internal/watchlist/bulk_test.go`: 26-01-T2; `internal/tags/bulk_test.go`: 26-02-T2
- [x] `web/app/routes/watchlist.bulk.test.tsx`: 26-03-T1; `web/app/lib/bulkSelection.test.ts`: 26-04-T2; `format.test.ts` / `api.test.ts` additions: 26-03-T2
- [x] `web/app/components/watchlist/BulkActionBar.test.tsx`: 26-06-T3

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| One network request per bulk action | All | Network-panel observation | DevTools → Network while running each action |
| 375px action bar layout, popover, long names/tags | BULK-01..04 | Visual | Resize to 375px, select artists, open each action |
| Shift-click range select through label | BULK-01 | Browser `shiftKey` on label clicks (RESEARCH A1) | Click one card, shift-click another, confirm range |
| Focus never behind the sticky bar; contrast | BULK-01 | Visual/a11y | Tab through cards with bar visible |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-10-19

---

## Validation Audit 2026-10-19

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 31 mapped tasks have automated tests that exist and pass: Go `Bulk` tests (tags, watchlist, httpserver) green against Postgres, and 16 web test files / 464 tests green. Manual-only items are unchanged (visual, network panel, shift-click browser behaviour; the UAT shift-click gap was fixed in f3105f1).
