---
phase: 16-rollback-safe-migrations
plan: 05
subsystem: database
tags: [migrations, golang-migrate, documentation, ci-tooling]

# Dependency graph
requires:
  - phase: 16-rollback-safe-migrations
    provides: "16-02's checkpoint-locked allow-destructive annotation grammar (option a, recorded in 16-02-SUMMARY.md), which this plan documents verbatim; 16-03's D-15 previous-release query cross-reference, which this plan explains as non-overridable"
provides:
  - "internal/db/migrations/README.md — MGRT-02's standing constraint, five sections: the two rules stated separately (backward-incompatible vs. unsafe-forward), the N-1 rollback invariant, an expand/backfill/contract walkthrough anchored on 000006/000007, a before-you-merge checklist sourced from PITFALLS.md Pitfall 8, and what cmd/migration-check enforces (including the exact allow-destructive annotation syntax)"
  - "internal/db/migrations_readme_test.go — a package db_test doc-presence test asserting 9 required phrases plus a minimum-line-count floor, so a future edit that guts a section turns the Go suite red"
  - "a one-line Definition-of-Done pointer in .claude/CLAUDE.md routing anyone adding or editing a migration to the README before they write SQL"
affects: [16-04-ci-wiring-n1-boot-job]

# Actuals (#2632)
actuals:
  tokens: 2742
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "doc-presence test: table-driven phrase-containment assertions against a README read from disk, plus a min-line-count floor so a phrase-stuffed stub can't pass — same on-brand pattern as the repo's .env.example parity tests"

key-files:
  created:
    - internal/db/migrations/README.md
    - internal/db/migrations_readme_test.go
  modified:
    - .claude/CLAUDE.md

key-decisions:
  - "Documented the allow-destructive annotation grammar exactly as locked at Plan 02's checkpoint (option a: file-scoped, single-line, bare-token tag with a trailing free-text reason) — no alternate phrasing, per the plan's precondition and 16-02-SUMMARY.md's recorded decision."
  - "CLAUDE.md's one-line pointer was placed as its own bold-lead paragraph directly after the existing 'Format frontend first' callout, mirroring that paragraph's structure, rather than folded into the numbered gate list — keeps the six numbered gates untouched and the pointer visually scannable."

patterns-established:
  - "Doc-presence test as a drift guard: a README documenting a machine-enforced contract (here, cmd/migration-check's two finding classes and its annotation grammar) gets a table-driven phrase test asserting the two never silently diverge."

requirements-completed: [MGRT-02]

coverage:
  - id: D1
    description: "internal/db/migrations/README.md documents the two rule classes separately, the N-1 invariant, the expand/backfill/contract walkthrough anchored on 000006/000007, the pre-merge checklist, and what cmd/migration-check enforces including the exact allow-destructive annotation syntax"
    requirement: "MGRT-02"
    verification:
      - kind: other
        ref: "wc -l internal/db/migrations/README.md (138, floor 60); grep -c backward-incompatible/unsafe-forward/000006/000007/migration-check:allow-destructive, grep -ci concurrently/immutable/idempotent — all >= 1"
        status: pass
      - kind: unit
        ref: "internal/db/migrations_readme_test.go#TestMigrationsReadme_ContainsRequiredPhrases"
        status: pass
    human_judgment: false
  - id: D2
    description: "A doc-presence test guards the README's required phrases and non-trivial length against silent drift"
    requirement: "MGRT-02"
    verification:
      - kind: unit
        ref: "internal/db/migrations_readme_test.go#TestMigrationsReadme_ContainsRequiredPhrases"
        status: pass
      - kind: unit
        ref: "internal/db/migrations_readme_test.go#TestMigrationsReadme_IsNonTrivial"
        status: pass
    human_judgment: false
  - id: D3
    description: ".claude/CLAUDE.md carries exactly one new line pointing to the migrations README, and the six numbered Definition-of-Done gates are unchanged"
    requirement: "MGRT-02"
    verification:
      - kind: other
        ref: "grep -c 'internal/db/migrations/README.md' .claude/CLAUDE.md == 1; git diff --stat confirms .claude/CLAUDE.md gained 2 lines (blank + pointer), no other line changed"
        status: pass
    human_judgment: false
  - id: D4
    description: "Adding README.md inside internal/db/migrations/ does not change the go:embed migrations glob set — the existing migration tests still pass unchanged"
    requirement: "MGRT-02"
    verification:
      - kind: integration
        ref: "TEST_DATABASE_URL=... go test ./internal/db/... -count=1 -v (18 pre-existing tests + 2 new doc-presence tests, all PASS, no FAIL/SKIP)"
        status: pass
    human_judgment: false

duration: ~25min
completed: 2026-09-04
status: complete
---

# Phase 16 Plan 05: internal/db/migrations/README.md — the expand/contract rule as a standing constraint Summary

**A five-section `internal/db/migrations/README.md` documents backward-incompatible vs. unsafe-forward migrations as separate rules, the N-1 rollback invariant, an expand/backfill/contract walkthrough anchored on migrations 000006/000007 already in the tree, a before-you-merge checklist, and the exact `cmd/migration-check` annotation syntax locked at Plan 02's checkpoint — guarded by a doc-presence test and routed to from CLAUDE.md's Definition of Done.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-09-04T17:31Z (session start, after required-reading pass)
- **Completed:** 2026-09-04T17:56Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- Wrote `internal/db/migrations/README.md` (138 lines): the two rule classes stated separately per S4 (backward-incompatible breaks rollback; unsafe-forward breaks/locks the deploy), the N-1 invariant naming both proving mechanisms (`n1-boot` CI job and the `internal/db` ahead-of-source test), a concrete expand→backfill→contract walkthrough citing `000006`/`000007` verbatim, a copy-paste before-you-merge checklist sourced from PITFALLS.md Pitfall 8, and a section documenting `cmd/migration-check`'s two finding classes plus the exact locked `allow-destructive` annotation syntax and its non-overridable D-15 cross-reference limit
- Added `internal/db/migrations_readme_test.go` (`package db_test`): a table-driven test over 9 required phrases (both finding-class names, the N-1 phrasing, `000006`, `000007`, `idempotent`, `immutable`, `concurrently`, the `migration-check:allow-destructive` prefix) plus a 60-line floor guarding against a phrase-stuffed stub
- Added a one-line Definition-of-Done pointer to `.claude/CLAUDE.md` routing migration authors to the README, leaving the six existing numbered gates untouched and adding no restatement of the rule itself
- Confirmed the README does not disturb `internal/db`'s `//go:embed migrations/*.sql` glob — all 18 pre-existing `internal/db` tests plus the 2 new doc-presence tests pass with zero FAIL/SKIP

## Task Commits

Each task was committed atomically:

1. **Task 1: internal/db/migrations/README.md — the expand/contract rule as a standing constraint** - `c8ea34c` (docs)
2. **Task 2: Route people to the rule — CLAUDE.md pointer and a doc-presence test** - `0bd748c` (test)

**Plan metadata:** recorded in this commit (docs: complete plan)

## Files Created/Modified
- `internal/db/migrations/README.md` - the five-section migration rule doc (MGRT-02, D-09, D-10)
- `internal/db/migrations_readme_test.go` - `package db_test` doc-presence test guarding the README's required phrases and length
- `.claude/CLAUDE.md` - one-line Definition-of-Done pointer to the migrations README

## Decisions Made
See `key-decisions` in the frontmatter above — the annotation grammar was transcribed verbatim from Plan 02's locked checkpoint decision, and the CLAUDE.md pointer's placement mirrors the existing "Format frontend first" callout style.

## Deviations from Plan

None - plan executed exactly as written. Both tasks' acceptance criteria (line count, all required phrase greps, the doc-presence test, the CLAUDE.md one-line-only pointer, and the unchanged embed set) were verified directly before each commit.

## Issues Encountered
- `make test` (`go test ... -race`) fails to build on this Windows dev box — pre-existing documented cgo/ThreadSanitizer limitation (`.planning/WINDOWS.md`; identical note in 16-01-SUMMARY.md and 16-02-SUMMARY.md). Substituted the established workaround: the same `go test ./... -coverprofile=coverage.out -coverpkg=$COVER_PKGS` invocation without `-race`, with `-p 1`. All packages passed; `make coverage-gate` then ran against that profile and passed (90.05%, floor 80%).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- 16-04 (CI wiring: `changes` prelude, `migration-check` guard job, `n1-boot` job, `build-scan.needs:` append) can now reference `internal/db/migrations/README.md` from the guard's failure message text exactly as documented here — the annotation syntax, the two finding-class names, and the D-15 non-overridable limit are all byte-consistent with what 16-02/16-03 already implemented.
- No blockers. This plan closes MGRT-02 (marked complete in `REQUIREMENTS.md`). MGRT-01 stays open pending 16-04.

---
*Phase: 16-rollback-safe-migrations*
*Completed: 2026-09-04*

## Self-Check: PASSED

All 3 created/modified files found on disk; both task commit hashes (`c8ea34c`, `0bd748c`) found in git log.
