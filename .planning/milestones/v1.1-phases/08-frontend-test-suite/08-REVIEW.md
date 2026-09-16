---
phase: 08-frontend-test-suite
reviewed: 2026-08-12T00:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - .github/workflows/full-pipeline.yml
  - web/app/components/history/EventCard.test.tsx
  - web/app/components/history/EventCard.tsx
  - web/app/components/history/HistoryFilters.test.tsx
  - web/app/components/watchlist/PreferenceToggles.test.tsx
  - web/app/components/watchlist/SearchBox.test.tsx
  - web/app/components/watchlist/SearchBox.tsx
  - web/app/lib/api.ts
  - web/app/lib/test/routeStub.tsx
  - web/app/routes/watchlist.test.tsx
  - web/package.json
  - web/pnpm-lock.yaml
  - web/vitest.config.ts
  - web/vitest.setup.ts
findings:
  critical: 1
  warning: 2
  info: 2
  total: 5
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-08-12T00:00:00Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

The frontend test suite itself (Vitest config, setup, and the component/route
test files) is well-constructed: consistent `vi.mock("~/lib/api")` seams, no
shared fixture module leakage, deliberate assertions on wire-level details
(literal `href` attributes rather than DOM-resolved ones, exact debounce call
counts), and `mockReset: true` correctly wired to prevent cross-test bleed.
`api.ts` and `SearchBox.tsx`/`EventCard.tsx` are both defensively written
against unvalidated network data (unknown `event_type`, arbitrary `source`
strings) and consistently escape untrusted string values before interpolating
them into URLs.

The one substantive defect is in the CI wiring, not the test code: the new
`frontend-test` job added in this phase (confirmed via diff against
`71c7622`) is never added to `build-scan`'s `needs:` list, so a failing
frontend suite cannot block the Docker build/scan/release pipeline — the
CI gate this phase exists to provide doesn't actually gate anything yet.
Two warnings and two info items round out the rest: a stale, factually
incorrect comment in `SearchBox.test.tsx`, a `.prettierrc`-violating quote
style in `EventCard.tsx`, and a test-coverage asymmetry between
`watchlist.test.tsx` (single happy-path test) and `PreferenceToggles.test.tsx`
(explicit success + rollback coverage) for what is structurally the same
kind of optimistic-mutation UI.

## Critical Issues

### CR-01: New `frontend-test` CI job is not wired into the release gate

**File:** `.github/workflows/full-pipeline.yml:119-121`
**Issue:** This phase adds the `frontend-test` job (lines 83-105, confirmed
new via `git diff 71c7622..HEAD`), which runs `pnpm test` (the Vitest suite
reviewed here). However `build-scan`'s `needs:` list was not updated to
include it:

```yaml
build-scan:
  needs: [vet, lint, test, gitleaks, trivy-fs]
```

`build-scan` feeds directly into `release` (`needs: [build-scan]`, gated on
`push` to `main`). Because `frontend-test` runs as an independent, unlinked
job, a broken/failing frontend test suite does not prevent `build-scan` or
`release` from running to completion — the Docker image (which embeds the
built React UI via `go:embed` per `CLAUDE.md`) can still be built, scanned,
and pushed to `ghcr.io` and tagged as a release even when the frontend tests
are red. This defeats the stated purpose of adding this job: gating the
pipeline on frontend correctness. Depending on GitHub branch-protection
config (not visible from this file), this may also mean `frontend-test`
isn't even a required status check for merging PRs to `main`.
**Fix:**
```yaml
build-scan:
  needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]
```

## Warnings

### WR-01: Comment in SearchBox.test.tsx contradicts the actual source it documents

**File:** `web/app/components/watchlist/SearchBox.test.tsx:56-61`
**Issue:** The comment above the two AbortSignal tests states:

> "Both fail against current source: runSearch calls `searchArtists(query)`
> with one argument, so no signal ever reaches the request (see the folded
> todo)."

This is no longer true. `SearchBox.tsx:50` already calls
`searchArtists(query, controller.signal)` — the signal is passed as the
second argument. The comment appears to describe a pre-fix state of the
source (referencing "the folded todo") that has since been resolved, but was
never updated to match. A future maintainer reading this comment while
investigating a real regression in abort behavior would be misled into
believing the current, passing tests are only "accidentally" passing or
that a known gap still exists, when in fact the feature is implemented and
covered.
**Fix:** Update or remove the stale claim, e.g.:
```ts
// Proves SearchBox's own doc-comment claim -- "a fresh AbortController is
// created per debounced search and the prior one is aborted before the new
// one starts" -- holds at the request level (searchArtists receives the
// AbortSignal as its second argument), not only at the discard-the-stale-
// result callback level.
```

### WR-02: `watchlist.test.tsx` covers only the happy path for a destructive action

**File:** `web/app/routes/watchlist.test.tsx:31-46`
**Issue:** The single test in this file asserts that clicking the remove
control calls `removeWatchlist(42)`, but never asserts on the resulting UI
state: it doesn't confirm the entry actually disappears from the list after
a successful removal, and there is no test for `removeWatchlist` rejecting
(network/API failure). Contrast with `PreferenceToggles.test.tsx` in the
same phase, which explicitly tests both the optimistic-update success path
and the rollback-on-rejection path for a structurally similar
mutate-then-reconcile UI. For a destructive, non-undoable action (removing a
watched artist), a regression that silently removes the row from local
state even when the DELETE request fails (or fails to remove it when the
request succeeds) would not be caught by the current suite.
**Fix:** Add at least:
```ts
it("removes the row from the list after a successful DELETE", async () => {
  mockListWatchlist.mockResolvedValue([entry])
  mockRemoveWatchlist.mockResolvedValue(undefined)
  renderRoute(Watchlist, "/watchlist")
  await screen.findByText("Drake")
  await userEvent.click(screen.getByRole("button", { name: "Remove Drake from watchlist" }))
  await waitFor(() => expect(screen.queryByText("Drake")).not.toBeInTheDocument())
})

it("does not lose the row when removeWatchlist rejects", async () => {
  mockListWatchlist.mockResolvedValue([entry])
  mockRemoveWatchlist.mockRejectedValue(new Error("network error"))
  renderRoute(Watchlist, "/watchlist")
  await screen.findByText("Drake")
  await userEvent.click(screen.getByRole("button", { name: "Remove Drake from watchlist" }))
  await waitFor(() => expect(mockRemoveWatchlist).toHaveBeenCalled())
  expect(screen.getByText("Drake")).toBeInTheDocument()
})
```

## Info

### IN-01: `EventCard.tsx` switch statement uses single quotes, violating `.prettierrc`

**File:** `web/app/components/history/EventCard.tsx:93,95,97`
**Issue:** `web/.prettierrc` sets `"singleQuote": false`, and every other
string literal in this file (and its sibling test file) uses double quotes.
The `EventCardBody` switch is the one exception:
```ts
switch (event.event_type) {
  case 'new_release':
  case 'guest_feature':
  case 'deluxe_change':
```
There's no `format:check`/lint-on-CI step wired into `full-pipeline.yml`
today, so this won't fail a build, but it's a real deviation from the
project's own configured formatting rule and will produce unrelated diff
noise the next time someone runs `pnpm format`.
**Fix:** Run `pnpm format` (or manually switch to double quotes) so the file
matches `.prettierrc`.

### IN-02: `HistoryFilters.test.tsx` never exercises the event-type filter

**File:** `web/app/components/watchlist/../history/HistoryFilters.test.tsx`
**Issue:** All three tests in this file interact only with the "Artist"
combobox (`getArtistSelect`). `HistoryFiltersValue` also has an `eventType`
field (visible in `emptyValue`), but no test selects an event type or
asserts `onChange` is called with a changed `eventType`. This is a coverage
gap for a component this phase specifically added tests for.
**Fix:** Add a test analogous to the existing artist-select tests, e.g.
selecting an event type option and asserting
`onChange` was called with `{ ...emptyValue, eventType: "new_release" }`
(or equivalent, pending `HistoryFilters`'s actual option values/labels).

---

_Reviewed: 2026-08-12T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
