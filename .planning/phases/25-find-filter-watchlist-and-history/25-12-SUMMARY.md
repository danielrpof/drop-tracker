---
phase: 25-find-filter-watchlist-and-history
plan: 12
subsystem: ui
status: complete
tags: [react, vitest, history, retention, go, integration-test]

requires:
  - phase: 10-retention
    provides: retention keyed on events.created_at (D-01)
provides:
  - formatDetectedDate helper (local D Mon YYYY, null when unparseable)
  - Detected line on every History event card, from created_at
  - Go regression pinning "detected recently, released long ago" under a tag filter
affects: [25-14, UAT test 7 rewording]

plan_head_before: 9623db6ae794ced7ea67495e67a3b31148ecccb9
actuals:
  tokens: 2540
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Show the date a retention rule keys on, rather than adding retention logic to the UI"

key-files:
  created: []
  modified:
    - web/app/lib/format.ts
    - web/app/lib/format.test.ts
    - web/app/components/history/EventCard.tsx
    - web/app/components/history/EventCard.test.tsx
    - internal/httpserver/events_test.go

key-decisions:
  - "Per-card Detected line instead of an end-of-feed retention note: the History route has no retention window length, and the confusion appears on every Load more page"
  - "Retention stays on created_at (Phase 10 D-01); queries/events.sql, sqlc output and internal/events are untouched"

requirements-completed: [HIST-02]

coverage:
  - id: D1
    description: "Every History card (new_release, guest_feature, deluxe_change) shows 'Detected {D Mon YYYY}' in a <time> with dateTime and title; unparseable created_at renders no line"
    requirement: HIST-02
    verification:
      - kind: unit
        ref: "web/app/components/history/EventCard.test.tsx#shows the Detected date from created_at"
        status: pass
      - kind: unit
        ref: "web/app/lib/format.test.ts#formatDetectedDate"
        status: pass
    human_judgment: false
  - id: D2
    description: "Under a tag filter, a 2015 release detected yesterday is returned on the first page and every cursor page; a today-dated release detected 120 days ago never is; has_older_events is true"
    requirement: HIST-02
    verification:
      - kind: integration
        ref: "internal/httpserver/events_test.go#TestRetention_TagFilterKeepsOldReleasesDetectedInWindow"
        status: pass
    human_judgment: false
---

# Phase 25 Plan 12: History card Detected date Summary

**History cards now show `Detected {D Mon YYYY}` from `created_at`, the date retention counts from, and a Go integration test pins that old releases detected in-window stay visible while recent releases detected out-of-window stay hidden.**

## Accomplishments

- `formatDetectedDate(iso)` in `web/app/lib/format.ts` returns the local `D Mon YYYY` (same shape as `formatReleaseDate`) or `null` for unparseable input.
- `EventCard` renders one `Detected <time dateTime title>` line after the type-specific body, so all three card variants get it without touching the body components. A malformed `created_at` omits the line (no `Invalid Date`, no em dash).
- `TestRetention_TagFilterKeepsOldReleasesDetectedInWindow` plus the `insertTestEventDatedAt` helper pin G-25-7: the "leak" was release-date ordering over a back catalogue detected recently, not a retention defect. It passed on first run, as the plan expected.
- No change to `queries/events.sql`, `internal/db`, or `internal/events`.

## TDD Gate Compliance

- RED: `9eff68f` test(25-12). 7 new cases failed: 3 `formatDetectedDate` (function not yet exported) and 4 EventCard Detected assertions. The malformed-`created_at` case passed already, since no Detected line existed.
- GREEN: `bc24fd2` feat(25-12). All new and existing format, EventCard and history route tests pass; typecheck exits 0.
- REFACTOR: none needed.
- Task 2 is a test-only pin of existing behavior (`bc22dbe`), by design.

## Task Commits

1. Task 1 RED: `9eff68f` (test)
2. Task 1 GREEN: `bc24fd2` (feat)
3. Task 2: `bc22dbe` (test)

## Verification

- `vitest run` for format, EventCard, history route: 92 passed. Full `pnpm test`: passes with coverage summary at 97% lines.
- `pnpm run typecheck`: exit 0. Prettier clean.
- `go test ./internal/httpserver/ -run 'TestRetention_|TestListEvents_TagFilter|TestHasOlderEvents_'`: all PASS including the new test. `go test ./...`: all packages ok.
- `go vet ./...` and `golangci-lint run ./internal/httpserver/...`: clean.
- `git diff --name-only -- queries internal/db internal/events`: empty.
- Acceptance greps: `formatDetectedDate(event.created_at)` count 1; `dangerouslySetInnerHTML` count 0.

## Deviations from Plan

None - plan executed exactly as written. The tracer gate (`HUMAN_VERIFY_MODE` end-of-phase, automated-only verify) was satisfied by re-running the Task 1 verify, which passed.

## Issues Encountered

None. `make coverage-gate` and `make sqlc-check` were not run: the plan touches no sqlc input and only adds one Go test helper; the full Go suite passed.

## Known Stubs

None.

## Threat Flags

None. T-25-33/34/35 mitigations are in place (plain JSX text through a Date parse; no query change with a regression test; null on malformed input).

## Next Phase Readiness

25-14 can reword UAT test 7 and the verification check to point at the Detected line.

## Self-Check: PASSED

- Modified files exist; commits `9eff68f`, `bc24fd2`, `bc22dbe` present in `git log`.
