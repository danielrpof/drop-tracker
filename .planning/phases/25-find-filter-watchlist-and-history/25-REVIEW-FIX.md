---
phase: 25-find-filter-watchlist-and-history
fixed_at: 2026-10-10T18:30:00Z
review_path: .planning/phases/25-find-filter-watchlist-and-history/25-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 25: Code Review Fix Report

**Fixed at:** 2026-10-10
**Source review:** .planning/phases/25-find-filter-watchlist-and-history/25-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (Critical 0, Warning 4; Info excluded by fix_scope)
- Fixed: 4
- Skipped: 0

**Verification environment:** `workflow.use_worktrees` is `false`, so all edits, commits and gates ran in the main checkout (no worktree). The Postgres from `docker compose` (drop-tracker-postgres-1) was up and `TEST_DATABASE_URL` pointed at it, so DB-backed tests ran rather than skipping.

## Fixed Issues

### WR-01: A stale "Load more" response can append rows from the previous filter to the new feed

**Files modified:** `web/app/routes/history.tsx`, `web/app/routes/history.test.tsx`
**Commit:** 344d69b
**Applied fix:** `handleLoadMore` now captures a feed key (artist, type, tag, reload token) when the request is issued and drops the response, error, and `appendLoading` reset if the key has changed. The page-one effect also resets `appendLoading`. Added a regression test (a held Load more, then a tag change, then a late stale response). It fails without the fix and passes with it.

### WR-02: Sticky cards survive filter changes made through the URL (Back/Forward)

**Files modified:** `web/app/routes/watchlist.tsx`, `web/app/routes/watchlist.test.tsx`
**Commit:** 000b090
**Applied fix:** Added an effect that clears `stickyIds` when the URL-owned filter key (sort, tags, muted, custom) changes. The `q` resync branch (Back/Forward) now also calls `clearSticky()`. `q` is deliberately kept out of the key: its debounced echo write would otherwise wipe a card touched during the 250 ms window. The explicit handler `clearSticky()` calls are kept, so the change is minimal and idempotent. Added a Back-navigation regression test. It fails without the fix and passes with it.
**Not done:** The review also noted that the count announcement is not scheduled for URL-driven changes. I left this out. An effect-driven announcement would overwrite the "Sorted by ..." message that `handleSortChange` sets, and the handlers already schedule announcements for UI-driven changes.

### WR-03: Placeholder or malformed release dates are shown as a real "Latest release"

**Files modified:** `queries/watchlist.sql`, `internal/db/sqlc/watchlist.sql.go`, `internal/db/sqlc/querier.go`, `internal/watchlist/service_test.go`
**Commit:** 4bdc558
**Applied fix:** All four `latest` and `upcoming` LATERAL blocks (`ListWatchlist` and `GetWatchlistEntry`) now require a well-formed `YYYY`, `YYYY-MM` or `YYYY-MM-DD` date (regex) that is `>= '0001'`. sqlc 1.31.1 regenerated the Go code. Added two `TestService_List_LatestRelease` cases (placeholder/malformed dates ignored, and a malformed date never beats a real one). The first fails without the fix (it surfaced `2020-13` and `TBA`) and passes with it.
**Status note:** This is a query-logic change, so it needs a human check that the regex matches the intended accepted formats.

### WR-04: The "cancels a pending count announcement" test cannot detect the regression it is named for

**Files modified:** `web/app/routes/watchlist.test.tsx`
**Commit:** 9cc7050
**Applied fix:** Both sort-announcement tests now use `vi.useFakeTimers({ shouldAdvanceTime: true })` with `userEvent.setup({ advanceTimers })`, and `vi.advanceTimersByTime(1000)` replaces the 700 ms real sleeps. A mutation check (removing the `clearTimeout(countTimer)` line in `handleSortChange`) makes the "cancels" test fail, so it now detects the regression.
**Caveat:** Plain `useFakeTimers` hangs under vitest because RTL's `asyncWrapper` awaits a real `setTimeout`. `shouldAdvanceTime` is therefore required, and the fake clock still tracks wall-clock time while the interactions run. The test can still pass spuriously if the type and sort interactions take over 500 ms, but the 1.4 s of real sleeping is gone.

## Additional commit

- befe939: `build(25): rebuild embedded SPA after review fixes`. `web/app` source changed, so I rebuilt with the same steps as the `make web` target (`pnpm install --frozen-lockfile`, `pnpm run build`, copy `web/build/client` to `internal/webassets/build/client`). `make` is not installed here, so I ran the steps by hand.

## Verification

| Command | Result |
|---|---|
| `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before each web commit; `prettier --check` at the end | clean |
| `cd web && corepack pnpm test` (vitest, coverage thresholds on) | 30 files, 568 tests passed |
| `cd web && corepack pnpm exec tsc --noEmit` | no errors |
| `go vet ./...` | clean |
| `golangci-lint run` | 0 issues |
| `go test ./... -count=1` against Postgres | all packages ok |
| `go test ./internal/webassets ./cmd/server -count=1` after the SPA rebuild | ok |
| `sqlc generate` (v1.31.1) | regenerated; the generated diff was committed with the SQL |

Not run:
- `make test` and `make sqlc-check`: `make` is not installed on this machine. `go test ./...` and `sqlc generate` ran directly instead.
- `-race` runs: cgo is not enabled here.
- `make coverage-gate`: it needs a `coverage.out` profile, which I did not generate.

---

_Fixed: 2026-10-10_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
