---
status: complete
phase: 08-frontend-test-suite
source: [08-01-SUMMARY.md, 08-02-SUMMARY.md, 08-03-SUMMARY.md, 08-04-SUMMARY.md, 08-05-SUMMARY.md]
started: 2026-08-13T02:58:17Z
updated: 2026-08-13T03:25:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Vitest suite fails non-zero on a component regression
expected: pnpm --dir web test runs the Vitest suite in jsdom and exits non-zero on a component regression, with no pass-with-no-tests escape hatch
result: pass
source: automated
coverage_id: 08-01/D1

### 2. Suite is idempotent and order-independent
expected: Two consecutive runs are green; a shuffled run (--sequence.shuffle) is also green
result: pass
source: automated
coverage_id: 08-01/D2

### 3. HistoryFilters behavior
expected: HistoryFilters.test.tsx asserts artist-list population from listWatchlist, upward filter reporting on artist selection, and artistId reported as null (never 0) when the artist filter is cleared
result: pass
source: automated
coverage_id: 08-01/D3

### 4. Tests mock the API boundary, not raw fetch
expected: Tests mock web/app/lib/api.ts (not raw fetch); the suite passes with no server/network
result: pass
source: automated
coverage_id: 08-01/D4

### 5. frontend-test CI job wired into Full Pipeline
expected: frontend-test job added to Full Pipeline's parallel tier, SHA-pinned actions, build-scan's needs array untouched
result: pass
source: automated
coverage_id: 08-01/D5

### 6. CI: frontend-test job runs green in GitHub Actions
expected: |
  After this branch/commit is pushed, the new `frontend-test` job in
  `.github/workflows/full-pipeline.yml`'s parallel tier shows a green check in
  the GitHub Actions run — same as `build-scan` and the other parallel jobs.
result: pass
reported: "Confirmed green in GitHub Actions run 31663467596 (Full Pipeline, main) after pushing 41 local commits."

### 7. Watchlist row remove reaches the API with the right id
expected: Clicking the watchlist row's remove control triggers removeWatchlist with the entry's numeric id, proven at the route level through the shared renderRoute helper
result: pass
source: automated
coverage_id: 08-02/D1

### 8. Preference toggle rolls back on failure
expected: A preference toggle rolls back its optimistic state through onEntryChange when the PATCH call fails, proven via the ordered optimistic-then-restore call pair
result: pass
source: automated
coverage_id: 08-02/D2

### 9. Shared router-stub helper
expected: One shared createRoutesStub helper (renderRoute) is the single router-context seam, established once and reused
result: pass
source: automated
coverage_id: 08-02/D3

### 10. Watchlist tests mock the API boundary
expected: Both new test files mock web/app/lib/api.ts and pass with no server running, individually and as part of the full parallel suite
result: pass
source: automated
coverage_id: 08-02/D4

### 11. Typecheck is clean
expected: pnpm --dir web typecheck exits 0
result: pass
source: automated
coverage_id: 08-02/D5

### 12. Confirm: watchlist remove + preference toggle tests (auto-verified)
expected: |
  Already proven by passing automated tests — confirming for the record:
  - Removing a watchlist entry calls the delete API with the entry's exact id (route-level test)
  - A preference toggle optimistically updates, then rolls back to its prior state if the save fails
  - One shared `renderRoute` router-stub helper is used everywhere a test needs router context
  - Both test files mock `web/app/lib/api.ts` and pass with no server running
  - `pnpm --dir web typecheck` exits 0

  Does this match what was built?
result: pass

### 13. Search collapses a keystroke burst into one debounced call
expected: Search surface has a passing test proving a keystroke burst collapses into exactly one debounced searchArtists call for the settled query, and the response reaches onResults
result: pass
source: automated
coverage_id: 08-03/D1

### 14. Superseded search is actually cancelled
expected: A superseded search's stale response never overwrites a newer query's results, and the superseded request is actually cancelled (AbortSignal reaches searchArtists and reports aborted on supersession), not merely discarded at the callback level
result: pass
source: automated
coverage_id: 08-03/D2

### 15. Search test mocks the API boundary
expected: The test mocks web/app/lib/api.ts (bare vi.mock, no factory/passthrough), not raw fetch, and passes with no server running
result: pass
source: automated
coverage_id: 08-03/D3

### 16. Confirm: search debounce + cancellation tests (auto-verified)
expected: |
  Already proven by passing automated tests — confirming for the record:
  - A burst of keystrokes in search collapses into exactly one debounced `searchArtists` call, and the result reaches the results list
  - A superseded (stale) search is actually cancelled via AbortSignal — its response can never overwrite a newer query's results
  - The test mocks `web/app/lib/api.ts`, not raw fetch, and passes with no server running

  Does this match what was built?
result: pass

### 17. History page — unrecognized event type doesn't crash
expected: |
  On the History page, events with known types (New Release, Guest Feature,
  Deluxe Change) show their normal colored badges. If an event ever has a
  type outside those three, the card shows a neutral "Unknown" badge instead
  of crashing the whole History page.
result: pass
reported: "Verified in CI (GitHub Actions run 31663467596): EventCard.test.tsx (7 tests) ran and passed, including the fallback-badge case for an unrecognized event_type."

### 18. Guest-feature links stay valid with unusual ids
expected: |
  On the History page, a Guest Feature event's link opens the correct
  MusicBrainz or Deezer page for that artist/release. Special characters in
  the underlying id (spaces, slashes, #) are percent-encoded in the link
  rather than breaking it.
result: pass
reported: "Verified in CI (GitHub Actions run 31663467596): EventCard.test.tsx's href-encoding cases ran and passed as part of the same 7-test file."

## Summary

total: 18
passed: 18
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
