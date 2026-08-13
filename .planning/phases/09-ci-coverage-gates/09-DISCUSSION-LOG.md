# Phase 9: CI Coverage Gates - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-12
**Phase:** 09-CI Coverage Gates
**Areas discussed:** Fold Todo, Backend coverage gate mechanism, Coverage scope & exclusions, Frontend coverage provider, Baseline gap-closing depth

---

## Fold Todo

| Option | Description | Selected |
|--------|-------------|----------|
| Fold it | Address flakiness as part of this phase — CI already runs `-p 1` so it's not currently biting CI, but `-coverprofile` instrumentation is a reason to double check | ✓ |
| Skip for now | Leave it pending — out of this phase's blast radius | |

**User's choice:** Fold it
**Notes:** Todo is explicitly tagged `resolves_phase: 9` and touches `internal/notifier`/`internal/poller`, packages this phase's coverage run also exercises.

---

## Backend coverage gate mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Hand-rolled shell check | `go tool cover -func` + grep/awk threshold compare, matches repo's hand-rolled-over-dependency pattern | ✓ |
| Dedicated GitHub Action | e.g. vladopajic/go-test-coverage, pinned to SHA — richer output but one more action to audit | |
| You decide | | |

**User's choice:** Hand-rolled shell check

| Option | Description | Selected |
|--------|-------------|----------|
| Extend make test-integration | Real DB, -race, -p 1 — what CI's `test` job already runs; most coverage lives in DB-backed tests | ✓ |
| New -short-only coverage target | Faster, no DB, but would undercount badly | |
| You decide | | |

**User's choice:** Extend make test-integration

| Option | Description | Selected |
|--------|-------------|----------|
| Log only | Print total % and pass/fail to job log — simplest, matches current scope | ✓ |
| Upload as artifact | Upload coverage.out/HTML as short-retention artifact | |

**User's choice:** Log only

---

## Coverage scope & exclusions

| Option | Description | Selected |
|--------|-------------|----------|
| Exclude sqlc | Generated, mechanically correct by construction | ✓ |
| Include sqlc | Counts toward aggregate, has incidental coverage anyway | |

**User's choice:** Exclude sqlc (internal/db/sqlc/)

| Option | Description | Selected |
|--------|-------------|----------|
| Include cmd/server | Real hand-written orchestration logic — graceful shutdown, migration retry | ✓ |
| Exclude cmd/server | Thin main-wiring, hard to unit test conventionally | |

**User's choice:** Include cmd/server

| Option | Description | Selected |
|--------|-------------|----------|
| Exclude both (generated route types, shadcn) | Neither is hand-written first-party logic | ✓ |
| Include shadcn, exclude generated types only | shadcn is copied into repo, could be customized | |

**User's choice:** Exclude both

---

## Frontend coverage provider

| Option | Description | Selected |
|--------|-------------|----------|
| v8 | Native, fast, current Vitest/Node default | ✓ |
| istanbul | More precise branch coverage, slower, extra dependency chain | |

**User's choice:** v8

| Option | Description | Selected |
|--------|-------------|----------|
| Vitest built-in thresholds | coverage.thresholds config fails process automatically, no extra script | ✓ |
| Separate check step | More code to write/maintain for no real benefit | |

**User's choice:** Vitest built-in thresholds

---

## Baseline gap-closing depth

| Option | Description | Selected |
|--------|-------------|----------|
| Just enough to clear the bar | Mirrors Phase 8 D-05 "floor only" philosophy | |
| Target most meaningful uncovered behavior first | More thorough — prioritize by risk/behavior significance | ✓ |

**User's choice:** Target most meaningful uncovered behavior first
**Notes:** User pushed back on the initial "just enough" recommendation ("2 is more thorough, no?"). Agreed thoroughness fits this project's portfolio goal (demonstrating rigorous CI/CD practice) better than the numerically-fastest path. Recommendation flipped accordingly.

| Option | Description | Selected |
|--------|-------------|----------|
| Plain test-add commits | No bug expected — normal passing test in its own commit | ✓ |
| Always RED-then-GREEN | Deliberate paper trail even when no bug found | |

**User's choice:** Plain test-add commits (RED-then-GREEN only if a gap-closing test uncovers a real defect)

---

## Claude's Discretion

None — all discussed areas resolved to a specific choice.

## Deferred Ideas

None — discussion stayed within phase scope.
