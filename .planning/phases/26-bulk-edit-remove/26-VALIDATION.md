---
phase: "26"
slug: "bulk-edit-remove"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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

Filled in by the planner/executor per task. Requirement → test map from RESEARCH.md:

| Req ID | Behavior | Test Type | Automated Command | File Exists | Status |
|--------|----------|-----------|-------------------|-------------|--------|
| BULK-01 | Effective selection = selected ∩ visible (incl. sticky); select-all-visible; range select | unit | `vitest run app/lib/bulkSelection.test.ts` | ❌ W0 | ⬜ pending |
| BULK-01 | Bar count / select-all tri-state use effective selection; hidden selection inert | route | `vitest run app/routes/watchlist.bulk.test.tsx` | ❌ W0 | ⬜ pending |
| BULK-02 | Bulk attach: room applies, existing link unaffected, cap skipped by name, gone reported | integration | `go test ./internal/tags -run TestService_BulkAttach -race -count=1` | ❌ W0 | ⬜ pending |
| BULK-02 | All-capped new tag leaves no tag row; concurrent bulk adds don't deadlock | integration | `go test ./internal/tags -run 'TestService_BulkAttach_(AllCapped\|Concurrent)' -race -count=1` | ❌ W0 | ⬜ pending |
| BULK-02 | Bulk detach: carriers only, unknown tag → 404 | integration + http | `go test ./internal/tags ./internal/httpserver -run 'BulkDetach\|BulkTagRemove'` | ❌ W0 | ⬜ pending |
| BULK-03 | Per-type delta, canonical order, no-op rows untouched, nil slices never wipe, 400 cases | integration + http | `go test ./internal/watchlist ./internal/httpserver -run 'BulkUpdatePreferences\|BulkPreferences'` | ❌ W0 | ⬜ pending |
| BULK-03 | Tri-state popover sends only changed types | component | `vitest run app/components/watchlist/BulkActionBar.test.tsx` | ❌ W0 | ⬜ pending |
| BULK-04 | All-or-nothing delete, already-gone reported, tags survive | integration | `go test ./internal/watchlist -run TestService_BulkRemove` | ❌ W0 | ⬜ pending |
| BULK-04 | Confirm states count + names + disclosure; one request | route | `vitest run app/routes/watchlist.bulk.test.tsx` | ❌ W0 | ⬜ pending |
| BULK-05 | Selection clears, single refresh, no checkbox for removed artist | route | `vitest run app/routes/watchlist.bulk.test.tsx` | ❌ W0 | ⬜ pending |
| All | 401/403 (store never called), 503 without option, id/body limits | http | `go test ./internal/httpserver -run 'Bulk' -race -count=1` | ❌ W0 | ⬜ pending |
| All | sqlc generated code committed and clean | static | `git add internal/db/sqlc && make sqlc-check` | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `queries/bulk.sql` + `sqlc generate`
- [ ] `internal/tags/bulk_test.go`, `internal/watchlist/bulk_test.go`, `internal/httpserver/bulk_test.go`
- [ ] `web/app/lib/bulkSelection.test.ts`, `format.test.ts` / `api.test.ts` additions
- [ ] `web/app/components/watchlist/BulkActionBar.test.tsx`, `web/app/routes/watchlist.bulk.test.tsx`

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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
