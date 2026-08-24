---
phase: quick-260824-2fw
plan: 01
subsystem: api
tags: [go, chi, refactor, validation]

requires: []
provides:
  - "trimAndCap(v string, maxRunes int) (string, bool) private helper in internal/httpserver/watchlist.go"
affects: [httpserver]

actuals:
  tokens: 825
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Shared trim + rune-count + compare mechanics extracted into a single helper (trimAndCap), with callers retaining ownership of blank checks, error messages, and status codes"

key-files:
  created: []
  modified:
    - internal/httpserver/watchlist.go

key-decisions:
  - "trimAndCap returns (trimmed string, withinLimit bool) rather than mutating a pointer, so both the mbid/name combined-blank-check call shape and the three pointer-reassignment optional-field call shapes stay simple two-line changes"

patterns-established: []

requirements-completed: [QUICK-260824-2fw]

coverage:
  - id: D1
    description: "handleAddWatchlist's five field validations (mbid, name, deezer_id, disambiguation, image_url) all call a single shared trimAndCap helper instead of repeating strings.TrimSpace/utf8.RuneCountInString/comparison inline five times"
    requirement: "QUICK-260824-2fw"
    verification:
      - kind: unit
        ref: "go test ./internal/httpserver/... -v -run TestWatchlist_Add"
        status: pass
      - kind: unit
        ref: "go test ./internal/httpserver/... -v (full package suite)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every field's error message text, JSON field name, and HTTP status code (400) is byte-identical to before the change; watchlist_test.go is unmodified"
    requirement: "QUICK-260824-2fw"
    verification:
      - kind: unit
        ref: "go test ./internal/httpserver/... -v (blank-field, overlong-field, overlong-optional-metadata, trims-optional-metadata-whitespace tests all pass unchanged)"
        status: pass
      - kind: other
        ref: "git diff --name-only -- internal/httpserver/watchlist_test.go (empty output)"
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-08-24
status: complete
---

# Quick Task 260824-2fw: Extract shared trimAndCap helper Summary

**Extracted the trim + rune-count + compare mechanics repeated five times in `handleAddWatchlist` into one private `trimAndCap` helper, with zero behavior change.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-08-24T06:50:00Z
- **Completed:** 2026-08-24T06:55:00Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- Added `trimAndCap(v string, maxRunes int) (trimmed string, withinLimit bool)` as a private helper in `internal/httpserver/watchlist.go`, carrying the trim + rune-count + compare mechanics shared by all five field validations
- Migrated `handleAddWatchlist`'s mbid/name blank-then-length checks to call `trimAndCap`, preserving the combined blank check exactly as before
- Migrated the three optional-metadata blocks (`req.DeezerID`, `req.Disambiguation`, `req.ImageURL`) to call `trimAndCap`, preserving each field's own error message, status code, and pointer-reassignment-on-success pattern
- Confirmed `handleAddWatchlist` now has exactly five `trimAndCap` call sites and zero direct `strings.TrimSpace`/`utf8.RuneCountInString` calls of its own
- Confirmed `handleUpdateWatchlist` was untouched (no trim/cap idiom exists there)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add trimAndCap helper and migrate the mbid/name checks** - `9c2f3b4` (refactor)
2. **Task 2: Migrate the three optional-metadata checks and verify the whole file** - `4ed72cb` (refactor)

_Note: Plan metadata/SUMMARY commit handled by orchestrator per quick-task constraints._

## Files Created/Modified
- `internal/httpserver/watchlist.go` - Added `trimAndCap` helper; rewired all five of `handleAddWatchlist`'s field validations (mbid, name, deezer_id, disambiguation, image_url) to call it

## Decisions Made
- `trimAndCap` returns `(trimmed string, withinLimit bool)` rather than a pointer-mutating signature, since this lets the mbid/name call site retain the combined blank check (needs both trimmed values before either length check fires) and lets each optional-field call site do a simple `v, ok := trimAndCap(...)` + reassign-on-success, matching the plan's `planning_findings`.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

`internal/httpserver/watchlist.go`'s field-validation duplication (code review finding #3) is closed. No blockers for future work in this file or package.

---
*Phase: quick-260824-2fw*
*Completed: 2026-08-24*

## Self-Check: PASSED

- FOUND: internal/httpserver/watchlist.go
- FOUND: .planning/quick/260824-2fw-fix-code-review-finding-3-extract-shared/260824-2fw-SUMMARY.md
- FOUND: 9c2f3b4 (Task 1 commit)
- FOUND: 4ed72cb (Task 2 commit)
