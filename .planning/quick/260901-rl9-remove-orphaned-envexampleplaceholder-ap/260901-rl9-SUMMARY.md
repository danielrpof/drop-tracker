---
phase: quick/260901-rl9
plan: 01
subsystem: auth
tags: [authgate, passphrase-gate, dead-code, comment-discipline, D-11]

requires:
  - phase: 14-instance-passphrase-gate
    provides: internal/authgate weak-passphrase heuristic (IsWeakPassphrase, knownDefaults, minPassphraseRunes)
provides:
  - "internal/authgate stripped of the orphaned envExamplePlaceholder const, denylist entry, drift-guard test, and misleading doc comment"
  - "knownDefaults denylist reduced to 15 generic placeholder terms (was 16), IsWeakPassphrase behavior byte-for-byte unchanged"
affects: [phase-15, phase-16, phase-17, authgate]

actuals:
  tokens: 900
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - internal/authgate/weak.go
    - internal/authgate/weak_test.go

key-decisions:
  - "Deleted the envExamplePlaceholder apparatus rather than repointing it: .env.example has shipped an empty INSTANCE_PASSPHRASE= since 465260c, so the const, its denylist slot, and its drift-guard test all pinned a subject that no longer exists. The generic knownDefaults denylist and the 16-rune length floor are the real defense and were left untouched."
  - "Did not substitute a plain 'caliber' string literal into knownDefaults: at 7 runes it is flagged by minPassphraseRunes regardless, so as a generic denylist term it defends nothing."

patterns-established: []

requirements-completed: [QUICK-260901-rl9]

coverage:
  - id: D1
    description: "Orphaned envExamplePlaceholder apparatus removed from internal/authgate (const, doc comment, denylist entry, table case, drift-guard test)"
    requirement: "QUICK-260901-rl9"
    verification:
      - kind: other
        ref: "grep -rn 'envExamplePlaceholder' internal/ && grep -rni 'caliber' internal/ — both zero matches"
        status: pass
      - kind: unit
        ref: "go test ./internal/authgate/ ./cmd/server/... — green"
        status: pass
    human_judgment: false
  - id: D2
    description: "IsWeakPassphrase observable behavior unchanged; authgate statement coverage holds at the 91.0% pre-change baseline"
    requirement: "QUICK-260901-rl9"
    verification:
      - kind: unit
        ref: "go test -cover ./internal/authgate/ — coverage: 91.0% of statements"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-09-01
status: complete
---

# Phase quick/260901-rl9: Remove orphaned envExamplePlaceholder apparatus Summary

**internal/authgate stripped of the dead `envExamplePlaceholder` ("caliber") const, its denylist slot, its drift-guard test, and an actively misleading doc comment — `IsWeakPassphrase` behavior unchanged, authgate coverage still 91.0%.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-09-01T19:46:00-05:00
- **Completed:** 2026-09-01T19:59:00-05:00
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Removed `TestWeakPassphrase_EnvExamplePlaceholderOnDenylist` and the `.env.example placeholder is flagged weak` table row from `weak_test.go`; trimmed the package comment's reference to the retired identifier.
- Deleted the `envExamplePlaceholder` const and its 6-line misleading doc comment from `weak.go` (the comment described a `.env.example` state that has not existed since commit `465260c` blanked `INSTANCE_PASSPHRASE=`).
- Dropped the leading `knownDefaults` entry and rewrote the slice's doc comment to describe only the generic denylist; remaining 15 entries unchanged in original order.
- Verified zero behavior change: `IsWeakPassphrase` still flags every generic denylist word and every value shorter than 16 runes; `go test -cover ./internal/authgate/` reports 91.0%, matching the pre-change baseline.

## Task Commits

Each task was committed atomically:

1. **Task 1: Drop the drift-guard test and the placeholder table case from weak_test.go** - `aa13d5b` (test)
2. **Task 2: Delete the orphaned const from weak.go and run the Definition of Done gates** - `e555b34` (refactor)

**Plan metadata:** handled by orchestrator (docs commit)

## Files Created/Modified
- `internal/authgate/weak.go` - Removed `envExamplePlaceholder` const + doc comment; removed its `knownDefaults` entry; trimmed the `knownDefaults` doc comment to ~3 lines. `minPassphraseRunes` and `IsWeakPassphrase` untouched.
- `internal/authgate/weak_test.go` - Removed the drift-guard test function, one table row, and the package-comment reference to `envExamplePlaceholder`. Both boot-WARN tests untouched.

## Decisions Made
- Deletion over repointing: the apparatus only ever had weight as *the example file's shipped value*; with that value blanked, keeping a test that asserts against a nonexistent subject reads as coverage while proving nothing (threat register T-rl9-01/T-rl9-02, disposition accept).
- No string-literal substitute in `knownDefaults`: `"caliber"` is 7 runes, below `minPassphraseRunes = 16`, so the length floor flags it regardless — the denylist slot was strictly redundant even before `.env.example` was blanked.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `golangci-lint`, `staticcheck`, and `make` are not on PATH in this execution shell (known Go 1.26 / tooling-version mismatch, per project memory). `golangci-lint` still ran and **passed** inside the `pre-commit` hook for both commits (its managed environment has a compatible binary) — that is the authoritative local mirror of CI. `make test` / `make coverage-gate` require `make` + Docker Postgres and were not runnable here; substituted the direct equivalents `go vet ./...` (clean), `go test ./internal/authgate/ ./cmd/server/...` (green), and `go test -cover ./internal/authgate/` (91.0%, baseline). `make sqlc-check` is a no-op for this change (no `.sql`/`sqlc.yaml` touched). CI is the backstop for the make-gated checks.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- No impact on Phase 15 (PR Coverage-Diff Comment) or later v1.3 phases — this is a self-contained dead-code removal in `internal/authgate`.
- Historical `.planning/` documents that mention the retired identifier (`14-04-SUMMARY.md`, `14-UAT.md`, `14-05-PLAN.md`, `.planning/debug/passphrase-gate-bypassed.md`) were deliberately left unedited — they are an accurate record of what Phase 14 shipped.

## Self-Check: PASSED

- `internal/authgate/weak.go` — FOUND (modified, `knownDefaults` has 15 entries starting at `changeme`)
- `internal/authgate/weak_test.go` — FOUND (modified, `TestIsWeakPassphrase` has 11 table rows, drift-guard test removed)
- Commit `aa13d5b` — FOUND
- Commit `e555b34` — FOUND
- `grep -rn envExamplePlaceholder internal/` — 0 matches
- `grep -rni caliber internal/` — 0 matches

---
*Phase: quick/260901-rl9*
*Completed: 2026-09-01*
