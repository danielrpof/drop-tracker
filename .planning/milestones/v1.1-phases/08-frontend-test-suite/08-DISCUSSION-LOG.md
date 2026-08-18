# Phase 8: Frontend Test Suite - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-12
**Phase:** 08-Frontend Test Suite
**Areas discussed:** Folded todos, CI wiring scope, Coverage depth ambition, API-mock strategy, Bug-fix scope for folded todos

---

## Folded Todos (pre-discussion, cross-reference step)

| Todo | Severity | Selected |
|------|----------|----------|
| EventCard crashes History route on unrecognized event_type | major | ✓ |
| SearchBox AbortController never cancels the underlying fetch | minor | ✓ |
| guestFeatureHref missing encodeURIComponent on external_id | cosmetic | ✓ |
| Fix flaky tests under parallel `go test ./...` | minor | (excluded — already tagged `resolves_phase: 9`, not presented) |

**User's choice:** All three presented UI todos folded into Phase 8.
**Notes:** The flaky-Go-test todo was excluded from the presented options because it was already routed to Phase 9 via a prior `resolves_phase` tag (2026-08-12 session, commit `4a6a58b`).

---

## CI wiring scope

| Option | Description | Selected |
|--------|-------------|----------|
| Add ungated CI job now | New job in `.github/workflows/full-pipeline.yml` runs the Vitest suite on every push, fails only on test failure (no coverage threshold) | ✓ |
| Local-only, defer all CI to Phase 9 | No workflow file changes this phase; Phase 9 creates the CI job and the coverage gate together | |

**User's choice:** Add ungated CI job now.
**Notes:** None.

---

## Coverage depth ambition

| Option | Description | Selected |
|--------|-------------|----------|
| Floor only, per success criteria | Exactly the tests success criterion 2 names, plus what's needed for the 3 folded fixes | ✓ |
| Broader branch coverage | Also cover every component branch (all EventCard bodies, loading/empty/error states) while in there | |

**User's choice:** Floor only, per success criteria.
**Notes:** None.

---

## API-mock strategy

| Option | Description | Selected |
|--------|-------------|----------|
| vi.mock() per test file | Each test file mocks `~/lib/api` directly, sets its own resolved/rejected values | ✓ |
| Shared typed mock-api helper module | New `web/app/lib/api.mock.ts` with reusable typed factory helpers | |

**User's choice:** vi.mock() per test file.
**Notes:** None.

---

## Bug-fix scope for folded todos

| Option | Description | Selected |
|--------|-------------|----------|
| RED-then-GREEN, own commit per fix | Each fix gets a failing-test commit then a separate minimal-fix commit | ✓ |
| Bundle fix into the surface's test-writing commit | Fix lands inside the same commit as the surface's general test file | |

**User's choice:** RED-then-GREEN, own commit per fix.
**Notes:** None.

---

## Claude's Discretion

None — all discussed areas resolved to a specific user choice.

## Deferred Ideas

None — discussion stayed within phase scope.
