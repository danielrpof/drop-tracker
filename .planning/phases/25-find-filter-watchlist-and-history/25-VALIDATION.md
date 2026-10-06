---
phase: "25"
slug: "find-filter-watchlist-and-history"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-10-05"
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Derived from `25-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` against real Postgres (`testutil.NewTestPool` / `NewIsolatedTestPool`); Vitest 4.1.10 + Testing Library + jsdom (TZ `UTC`, `mockReset: true`) |
| **Config file** | `web/vitest.config.ts`; Go: Makefile (`COVERAGE_THRESHOLD_BACKEND ?= 80`) |
| **Quick run command** | Go: `TEST_DATABASE_URL='postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable' go test ./internal/watchlist/ ./internal/events/ ./internal/httpserver/ -run '<names>' -count=1 -v` · Web: `corepack pnpm --dir web exec vitest run <files> --coverage.enabled=false` + `corepack pnpm --dir web run typecheck` |
| **Full suite command** | `go vet ./...`, `golangci-lint run`, `make test`, `make coverage-gate`, `git add internal/db/sqlc && make sqlc-check`, `corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}"`, `corepack pnpm --dir web test` |
| **Estimated runtime** | ~120 seconds (full); ~10–20 seconds (quick) |

---

## Sampling Rate

- **After every task commit:** narrowest Go `-run` / `vitest run <files>` for touched files, plus `typecheck` for any `web/` change
- **After every plan wave:** `go test ./internal/watchlist/ ./internal/events/ ./internal/httpserver/ -count=1`, `corepack pnpm --dir web test`, `make sqlc-check`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

Filled by the planner per task; requirement → test map below is the contract.

| Req ID | Behavior | Test Type | Automated Command | File Exists | Status |
|--------|----------|-----------|-------------------|-------------|--------|
| WLVW-01 | fold/trim/name-only search | unit | `vitest run app/lib/watchlistView.test.ts` | ❌ W0 | ⬜ pending |
| WLVW-01 | typing narrows live; "N of M"; zero extra requests | component | `vitest run app/routes/watchlist.test.tsx` | ✅ extend | ⬜ pending |
| WLVW-02 | name collator sort, date-added, id-asc stable tie-break | unit | `vitest run app/lib/watchlistView.test.ts` | ❌ W0 | ⬜ pending |
| WLVW-03 | latest/next release semantics in service projection | Go integration | `go test ./internal/watchlist/ -run 'TestService_List_LatestRelease\|TestService_ProjectionParity' -count=1 -v` | ❌ W0 | ⬜ pending |
| WLVW-03 | JSON exposes fields; single query | Go HTTP | `go test ./internal/httpserver/ -run 'TestWatchlist_List_LatestRelease' -count=1 -v` | ❌ W0 | ⬜ pending |
| WLVW-03 | null-last both directions; `formatReleaseDate` precisions | unit | `vitest run app/lib/watchlistView.test.ts app/lib/format.test.ts` | ❌ W0 / ✅ | ⬜ pending |
| WLVW-04 | any-of tag filter, chip toggle, URL `tags=` | unit + component | `vitest run app/lib/watchlistView.test.ts app/components/watchlist/TagChips.test.tsx app/components/watchlist/WatchlistToolbar.test.tsx app/routes/watchlist.test.tsx` | ❌ W0 / ✅ | ⬜ pending |
| WLVW-05 | muted / custom release-types toggles, AND composition | unit + component | same as WLVW-04 | ❌ W0 | ⬜ pending |
| WLVW-06 | composition, empty state, Clear filters, sticky cards, announcements | component | `vitest run app/routes/watchlist.test.tsx` | ✅ extend | ⬜ pending |
| WLVW-06 | URL writer: `preventScrollReset`, replace vs push, no stale clobber | unit | `vitest run app/lib/useUrlParams.test.tsx` | ❌ W0 | ⬜ pending |
| HIST-02 | tag filter composes; retention on first page and `has_older_events`; removed artist included; bad id → 400 | Go integration + HTTP | `go test ./internal/httpserver/ -run 'TestRetention_TagFilter\|TestListEvents_TagFilter\|TestHandleListEvents_Validation' -count=1 -v` | ❌ W0 | ⬜ pending |
| HIST-02 | History Tag control, tag empty state, URL state | component | `vitest run app/components/history/HistoryFilters.test.tsx app/routes/history.test.tsx app/lib/api.test.ts` | ✅ extend | ⬜ pending |
| SC5 | one `listWatchlist` call on open; zero while filtering | component | `vitest run app/routes/watchlist.test.tsx` | ✅ extend | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

### Planner task map (2026-10-05)

| Plan / Task | Requirement | Automated command (narrowest) |
|-------------|-------------|-------------------------------|
| 25-01 T1 (tracer) | WLVW-03 | `go test ./internal/httpserver/ -run 'TestWatchlist_List_LatestRelease'` + `make sqlc-check` |
| 25-01 T2 | WLVW-03 | `go test ./internal/watchlist/ -run 'TestService_List_LatestRelease\|TestService_ProjectionParity'` |
| 25-02 T1 (tracer) | HIST-02 | `go test ./internal/httpserver/ -run 'TestListEvents_TagFilter$'` + `make sqlc-check` |
| 25-02 T2 | HIST-02 | `go test ./internal/httpserver/ -run 'TestListEvents_TagFilter\|TestRetention_TagFilter\|TestHandleListEvents_Validation\|TestRetention_DetectionStateQueriesStayUnfiltered'` |
| 25-03 T1 (tracer) | WLVW-03 | `vitest run app/routes/watchlist.test.tsx` + `typecheck` |
| 25-03 T2 | WLVW-03 | `vitest run app/lib/format.test.ts app/routes/watchlist.test.tsx` |
| 25-04 T1 (tracer) | WLVW-01, WLVW-06 | `vitest run app/routes/watchlist.test.tsx` + `typecheck` |
| 25-04 T2 | WLVW-01, WLVW-06 | `vitest run app/lib/watchlistView.test.ts app/lib/useUrlParams.test.tsx app/components/watchlist/WatchlistToolbar.test.tsx` |
| 25-04 T3 | WLVW-06 | `vitest run app/routes/watchlist.test.tsx app/lib/watchlistView.test.ts app/components/watchlist/WatchlistToolbar.test.tsx` |
| 25-05 T1 (tracer) | WLVW-02, WLVW-03 | `vitest run app/routes/watchlist.test.tsx` |
| 25-05 T2 | WLVW-02, WLVW-03 | `vitest run app/lib/watchlistView.test.ts app/components/watchlist/WatchlistToolbar.test.tsx app/routes/watchlist.test.tsx` |
| 25-06 T1 (tracer) | HIST-02 | `vitest run app/routes/history.test.tsx app/lib/api.test.ts` |
| 25-06 T2 | HIST-02 | `vitest run app/components/history/HistoryFilters.test.tsx app/lib/tags.test.ts` |
| 25-06 T3 | HIST-02 | `vitest run app/routes/history.test.tsx app/components/history/HistoryFilters.test.tsx` |
| 25-07 T1 (tracer) | WLVW-04 | `vitest run app/routes/watchlist.test.tsx` |
| 25-07 T2 | WLVW-04 | `vitest run app/components/watchlist/WatchlistTagFilter.test.tsx app/lib/watchlistView.test.ts` |
| 25-07 T3 | WLVW-04, WLVW-06 | `vitest run app/routes/watchlist.test.tsx app/lib/watchlistView.test.ts` |
| 25-08 T1 (tracer) | WLVW-05, WLVW-06 | `vitest run app/lib/watchlistView.test.ts app/components/watchlist/WatchlistToolbar.test.tsx app/routes/watchlist.test.tsx` |
| 25-08 T2 | WLVW-04 | `vitest run app/components/watchlist/TagChips.test.tsx app/routes/watchlist.test.tsx` + full `pnpm test` |
| 25-09 T1 (tracer) | HIST-02 | `vitest run app/routes/history.test.tsx` |
| 25-09 T2 | HIST-02 | `vitest run app/lib/historyParams.test.ts app/components/history/HistoryFilters.test.tsx app/routes/history.test.tsx` |
| 25-10 T1 (tracer) | all | `make web` + bundle string grep + `go build ./...` |
| 25-10 T2 | all | full Definition of Done chain + blast-radius diff against `9030bfa` |

Go commands run with `TEST_DATABASE_URL='postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable' ... -count=1 -v`. Web commands run as `corepack pnpm --dir web exec vitest run <files> --coverage.enabled=false`. Every task has an automated verify, so no three tasks in a row lack one.

---

## Wave 0 Requirements

- [ ] `web/app/lib/test/fixtures.ts` — `makeEntry()`; migrate the 8 `WatchlistEntry` fixture files
- [ ] `web/app/lib/watchlistView.test.ts` — fold/filter/sort/parse/select-visible
- [ ] `web/app/lib/useUrlParams.test.tsx` — write semantics (extend `routeStub.tsx` with optional `initialEntry`)
- [ ] `web/app/components/watchlist/WatchlistToolbar.test.tsx` — keep-open / focus tests; stub `scrollIntoView`
- [ ] `mockListTags.mockResolvedValue([])` in `history.test.tsx` / `HistoryFilters.test.tsx` setup
- [ ] Go seed helper combining explicit `release_date` + `created_at` + `artist_tags` link

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Feel of live filtering with 50+ artists; keyboard flow through toolbar | WLVW-01..06 | Perceived latency and focus ergonomics | Seed 50+ artists, open Watchlist, type/sort/filter with keyboard only; confirm one network request in devtools |
| History tag filter with "load older" across retention edge | HIST-02 | End-to-end UX across pages | Pick a tag in History, page with "load older" until exhausted; no event older than retention appears |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
