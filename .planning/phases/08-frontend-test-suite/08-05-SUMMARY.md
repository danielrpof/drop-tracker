---
phase: 08-frontend-test-suite
plan: 05
subsystem: frontend-tests
tags: [vitest, react-testing-library, event-card, url-encoding, bug-fix, red-green]
dependency_graph:
  requires:
    - 08-04 (EventCard.test.tsx file, file-local fixture builder, four passing badge tests)
  provides:
    - Three new EventCard.test.tsx cases asserting href percent-encoding on both source branches
    - encodeURIComponent escaping in guestFeatureHref for musicbrainz and deezer URLs
  affects:
    - Guest-feature anchor hrefs in the History surface (now safe against external_id containing URL-significant characters)
tech_stack:
  added: []
  patterns:
    - File-local fixture builder with partial overrides, reused as-is from 08-04 (no second builder, no shared module)
    - RED-then-GREEN commit pair for a folded bug fix (per D-07)
    - toHaveAttribute("href", ...) against the rendered anchor rather than the unexported helper or the jsdom-resolved href property
key_files:
  created: []
  modified:
    - web/app/components/history/EventCard.test.tsx
    - web/app/components/history/EventCard.tsx
decisions: []
metrics:
  duration: "~15 minutes"
  completed: 2026-08-12
status: complete
actuals:
  tokens: 1071
  tasks: 2
  commits: 2
---

# Phase 8 Plan 05: EventCard Guest-Feature Href Encoding Fix Summary

Closed the third and last folded todo for this phase: `guestFeatureHref` interpolated `event.external_id`
directly into a URL path with no escaping, on both the MusicBrainz and Deezer branches. Fixed with a
RED-then-GREEN pair extending 08-04's existing `EventCard.test.tsx`.

## What Was Built

**Task 1 (RED):** Added three cases to `web/app/components/history/EventCard.test.tsx`, reusing 08-04's
file-local `buildEvent` fixture builder with partial overrides — no second builder, no shared fixtures
module. A shared id constant (`abc def/g#h`) carrying a space, a forward slash and a hash exercises three
escapes at once across the musicbrainz and deezer cases; the third case pins an ordinary UUID-shaped id to
its unchanged URL as a guard against an over-broad fix. All three assert `toHaveAttribute("href", ...)`
against the anchor found via `getByRole("link", { name: <title> })` — the attribute, not the jsdom-resolved
`href` property, which would have percent-encoded the space on its own and passed against unfixed source.
This left the suite red: 2 failing (musicbrainz, deezer), 5 passing (08-04's four plus the UUID guard,
which passes unmodified).

**Task 2 (GREEN):** Wrapped `event.external_id` in `encodeURIComponent(...)` at both interpolation sites
in `guestFeatureHref` (`web/app/components/history/EventCard.tsx`) — the MusicBrainz recording URL and the
Deezer track URL. Extended the function's existing head comment with one sentence explaining why: the id
is an unvalidated third-party string typed only as `string`, so today's UUID-shaped values are a fact
about the current upstreams, not a guarantee the code can rely on. No other change — the `null` return for
an unrecognized source, `GuestFeatureBody`'s markup, and `web/app/lib/api.ts` are all untouched.

## Verification

- `pnpm --dir web exec vitest run app/components/history/EventCard.test.tsx`: RED commit — 5 passed,
  2 failed (musicbrainz and deezer encoding cases), non-zero exit exactly as required. GREEN commit — all
  7 passed.
- `pnpm --dir web test` (full suite): 5 files, 16 tests, all passed after the GREEN commit.
- `pnpm --dir web typecheck`: exits 0 both before and after the GREEN commit — no cast needed since
  `source`/`external_id` are both plain `string` on `EventItem`.
- `git diff -- EventCard.test.tsx | grep -c '^-[^-]'` on the RED commit: 0 — additions only, 08-04's four
  tests and fixture builder untouched.
- `grep -v '^\s*//' EventCard.tsx | grep -c encodeURIComponent`: 2 — one per source branch.
- `grep -c '^export' EventCard.tsx`: 2 — unchanged export count; `guestFeatureHref` stays unexported.
- `git diff --stat` between the RED and GREEN commits lists exactly one file: `EventCard.tsx`.
- No `.skip(`, `.todo(`, `vi.mock`, `.href` (property access), or `data-testid` present in the touched
  files.
- No unexpected deletions in either commit (`git diff --diff-filter=D` empty for both).

## Deviations from Plan

None — plan executed exactly as written.

## Commits

- `df1344f` — `test(08-05): assert guest-feature href escapes external_id on both sources`
- `daee355` — `fix(08-05): escape external_id in guestFeatureHref on both source branches`

## Self-Check: PASSED

- FOUND: `web/app/components/history/EventCard.test.tsx` (modified, contains three new cases)
- FOUND: `web/app/components/history/EventCard.tsx` (modified, contains `encodeURIComponent`)
- FOUND: commit `df1344f` in `git log --oneline --all`
- FOUND: commit `daee355` in `git log --oneline --all`
