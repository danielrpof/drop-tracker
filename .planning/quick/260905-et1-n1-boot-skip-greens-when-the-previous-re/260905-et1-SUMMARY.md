---
phase: quick/260905-et1
plan: 01
subsystem: infra
tags: [github-actions, ci, golang-migrate, rollback, n1-boot, phase-16]

requires:
  - phase: 16-rollback-safe-migrations
    provides: n1-boot job, maxSourceVersion ahead-of-source guard, N-1 invariant doc
provides:
  - n1-boot guardcheck step that statically detects a pre-guard N-1 image and skip-greens with a ::notice::
  - guardcheck output ANDed into all 8 expensive n1-boot steps alongside the existing bootstrap gate
  - grep-token self-test that turns a stale workflow<->Go coupling into a loud RED instead of a permanent silent skip
  - guard-adoption-window note in internal/db/migrations/README.md
  - dated amendment in 16-CONTEXT.md <revisions> recording the D-04/D-19a and D-11 amendments
affects: [phase-16-verification, phase-17-deploy, G-16-1]

actuals:
  tokens: 9000
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Second dedicated in-job gate step (guardcheck) so two skip reasons stay independently readable in the run log, rather than a job-level if: or a composed boolean"
    - "Workflow step self-tests its coupling to a Go identifier against the checked-out tree; a rename fails the job loudly"

key-files:
  created: []
  modified:
    - .github/workflows/full-pipeline.yml
    - internal/db/migrations/README.md
    - .planning/phases/16-rollback-safe-migrations/16-CONTEXT.md

key-decisions:
  - "guardcheck gates on needs.changes.outputs.migrations_changed == 'true' only (same as True-bootstrap guard), per the locked step-placement decision"
  - "Flat AND of three conditions on each downstream step (not a refactored composed boolean) to keep the diff mechanical and both skip reasons visible per call site"
  - "Empty PREV_TAG path emits a plain line, not a ::notice:: - the bootstrap step owns that message; a duplicate would read as a third invented skip reason"
  - "grep anchored to '^func maxSourceVersion(' (definition) not a bare substring, so a comment mention at the previous tag cannot forge a guard-present reading"

patterns-established:
  - "Guard-adoption window: a CI check that is provably inert until a capability it depends on reaches the released image, documented and self-clearing"

requirements-completed: [QUICK-260905-et1, G-16-1]

coverage:
  - id: D1
    description: "n1-boot skip-greens (no Postgres/pull/probe) with a ::notice:: naming the tag and G-16-1 when the resolved N-1 tag's internal/db/migrate.go lacks the ahead-of-source guard"
    requirement: "G-16-1"
    verification:
      - kind: other
        ref: "actionlint .github/workflows/full-pipeline.yml (exit 0); grep pairing/count/order gates in Task 1 <verify> all pass; hand-trace of the 5-branch shell against PREV_TAG=v1.7.0 / guard-carrying tag / nonexistent tag"
        status: pass
      - kind: manual_procedural
        ref: "live GitHub Actions run on the next migration-touching push - PENDING, cannot be exercised until then"
        status: unknown
    human_judgment: true
    rationale: "GitHub Actions if: semantics are not unit-testable from the repo. The fix is proven statically (actionlint + grep shape gates + hand-trace) but cannot be proven GREEN live until a migration-touching branch is pushed while v1.7.0 is the resolved N-1 tag. G-16-1 stays OPEN until that run confirms it."
  - id: D2
    description: "Every n1-boot step that gates on bootstrap.outputs.proceed also gates on guardcheck.outputs.proceed; Teardown unchanged"
    requirement: "QUICK-260905-et1"
    verification:
      - kind: other
        ref: "grep -c \"steps.guardcheck.outputs.proceed == 'true'\" == 8; grep -c bootstrap == 8; every bootstrap line also names guardcheck; Teardown still always() && migrations_changed == 'true'"
        status: pass
    human_judgment: false
  - id: D3
    description: "Guard-adoption window documented in the migrations README under the N-1 invariant, and a dated amendment appended inside 16-CONTEXT.md <revisions>"
    requirement: "QUICK-260905-et1"
    verification:
      - kind: unit
        ref: "go test ./internal/db/ -run TestMigrationsReadme_ContainsRequiredPhrases -count=1 (ok); grep 'guard-adoption' README; grep 'G-16-1' 16-CONTEXT; single </revisions> close"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-09-05
status: complete
---

# Quick Task 260905-et1: n1-boot guard-adoption skip-green Summary

**n1-boot now statically detects a pre-guard N-1 image and skip-greens with an explanatory `::notice::` instead of reporting a signal-free N-1 rollback break on every migration-touching branch (Phase 16 gap G-16-1).**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-05
- **Completed:** 2026-09-05
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Added a `guardcheck` step to `n1-boot` (between `bootstrap` and `Start throwaway Postgres`) that runs `git show "$PREV_TAG:internal/db/migrate.go"` and greps for the guard's function definition. Pre-guard N-1 tag -> `::notice::` + `proceed=false`; guard present -> `proceed=true`.
- ANDed `steps.guardcheck.outputs.proceed == 'true'` into all 8 expensive `n1-boot` steps (the 7 boot/probe steps plus the `failure()` explainer). `Teardown` left exactly on `always() && needs.changes.outputs.migrations_changed == 'true'`.
- Added a grep-token self-test: if `^func maxSourceVersion(` is absent from the checked-out `internal/db/migrate.go` (the guard was renamed at HEAD), the step emits `::error::` and `exit 1` rather than skip-greening forever.
- Documented the guard-adoption window in `internal/db/migrations/README.md` under "The N-1 invariant" and appended a dated amendment to `16-CONTEXT.md` `<revisions>`.

## Task Commits

1. **Task 1: Add the guard-adoption gate to n1-boot and AND it into every N-1 boot step** - `865c162` (fix)
2. **Task 2: Document the guard-adoption window in the migrations README and amend 16-CONTEXT** - `9876c4e` (docs)

Plan metadata (SUMMARY.md, STATE.md, ROADMAP.md) committed by the orchestrator.

## Shipped strings and expressions

### `guardcheck` step — exact branch messages as shipped

1. **`PREV_TAG` empty** (plain line, not a notice — the bootstrap step owns this reason):
   `no prior release tag -- the true-bootstrap step already reported this; nothing to check here`
   then `proceed=false`.

2. **grep token absent from the checked-out tree** (`::error::` + `exit 1`, no `proceed` write — step fails RED):
   `::error::guard token 'func maxSourceVersion(' is absent from the checked-out internal/db/migrate.go -- the ahead-of-source guard was renamed at HEAD and this workflow's grep token is now stale. Update the token in n1-boot's guard-adoption check; until then this job cannot tell a pre-guard N-1 image from a guard-carrying one.`

3. **`git show "$PREV_TAG:internal/db/migrate.go"` fails** (fail-safe notice + `proceed=false`):
   `::notice::previous release ${PREV_TAG}'s internal/db/migrate.go could not be read at that tag -- assuming it predates the ahead-of-source migration guard, so N-1 rollback boot is not verifiable this cycle (G-16-1). Self-clears once a guard-carrying release becomes N-1.`

4. **Captured content does not match `^func maxSourceVersion(`** — the real G-16-1 path (notice + `proceed=false`):
   `::notice::the previously-released image ${PREV_TAG} predates the ahead-of-source migration guard, so it cannot boot against any newer schema and N-1 rollback boot is not verifiable this cycle (G-16-1). This self-clears with no workflow edit once a guard-carrying release becomes the N-1 rollback target.`

5. **Otherwise** — `proceed=true`, no output.

### Final `if:` on the 8 steps

The 7 boot/probe steps (`Start throwaway Postgres`, `Apply HEAD schema`, `Pull previous release image`, `Run previous release image`, `Health probe -- sustained green`, `Read probes -- GET /watchlist and GET /events`, `Write probe -- POST /watchlist then re-GET`):

```
if: needs.changes.outputs.migrations_changed == 'true' && steps.bootstrap.outputs.proceed == 'true' && steps.guardcheck.outputs.proceed == 'true'
```

The failure explainer (`N-1 boot failure -- explain the rule`) keeps its leading `failure() &&`:

```
if: failure() && needs.changes.outputs.migrations_changed == 'true' && steps.bootstrap.outputs.proceed == 'true' && steps.guardcheck.outputs.proceed == 'true'
```

`Checkout`, `Set up Go`, `Resolve previous release tag`, `True-bootstrap guard` unchanged. `Teardown` unchanged at `always() && needs.changes.outputs.migrations_changed == 'true'`.

## Verification run

### Task 1 (`.github/workflows/full-pipeline.yml`)

| Gate | Result |
|------|--------|
| `actionlint .github/workflows/full-pipeline.yml` | exit 0, no output |
| `grep -c "steps.guardcheck.outputs.proceed == 'true'"` == 8 | pass |
| `grep -c "steps.bootstrap.outputs.proceed == 'true'"` == 8 | pass |
| every `steps.bootstrap.outputs.proceed` line also names `steps.guardcheck.outputs.proceed` (`grep -v` -> exit 1) | pass |
| `awk` order check: `bootstrap` < `guardcheck` < `Start throwaway Postgres` | pass |
| `Teardown` still `always() && needs.changes.outputs.migrations_changed == 'true'` | pass |
| `git show "v1.7.0:internal/db/migrate.go" \| grep -c '^func maxSourceVersion('` -> `0`, exit 1 | pass |

**Hand-trace (`<human-check>`) — done by hand, no branch falls through without writing `proceed`:**

- **`PREV_TAG=v1.7.0`** — branch 1 false (non-empty); branch 2 false (checked-out `internal/db/migrate.go:326` has `func maxSourceVersion(`, verified); branch 3 false (`git show` succeeds, `v1.7.0` has the file, exit 0, `PREV_MIGRATE` populated); branch 4 TRUE (`v1.7.0` content has 0 matches of `^func maxSourceVersion(`, `grep -q` exits 1, `! ...` is true) -> `::notice::` "predates the ahead-of-source migration guard" + `proceed=false` written to `"$GITHUB_OUTPUT"`. Correct — this is the live G-16-1 case for the current N-1 tag.
- **`PREV_TAG` = a tag that carries the guard** (future v1.8.0+) — branches 1-3 fall through identically (`git show` succeeds); branch 4 false (`grep -q` finds `^func maxSourceVersion(`, exits 0, `! 0` false); else branch -> `proceed=true` written. Correct — boot + probes run again with no workflow edit.
- **`PREV_TAG` = a nonexistent tag** — branch 1 false; branch 2 false; branch 3 TRUE (`git show "bogus:..."` exits 128, assignment inherits 128, `! ...` true) -> fail-safe `::notice::` "could not be read at that tag -- assuming it predates" + `proceed=false` written. Correct — same skip direction as D-04's empty-tag handling, distinct message from branch 4.
- Only branch 2 (`exit 1`) leaves `proceed` unwritten, and that is deliberate: a stale grep token means the detector itself is broken, so the step fails RED rather than defaulting either way.

### Task 2 (`README.md`, `16-CONTEXT.md`)

| Gate | Result |
|------|--------|
| `go test ./internal/db/ -run TestMigrationsReadme_ContainsRequiredPhrases -count=1` | `ok` (0.135s, no Postgres) |
| `grep -qi "guard-adoption" internal/db/migrations/README.md` | pass |
| `grep -q "G-16-1" .planning/phases/16-rollback-safe-migrations/16-CONTEXT.md` | pass |
| exactly one `</revisions>` in `16-CONTEXT.md` | pass |
| `git diff --stat` | README +10, 16-CONTEXT +17, no existing README phrase removed |

### Definition of Done (workflow-YAML + Markdown only)

| Gate | Result |
|------|--------|
| `go vet ./...` | exit 0 (no Go changes) |
| `golangci-lint run` | `0 issues.` |
| `actionlint .github/workflows/full-pipeline.yml` | exit 0 (the real gate for this change) |
| `TestMigrationsReadme_ContainsRequiredPhrases` | `ok` |
| pre-commit hooks on both commits (gitleaks / golangci / prettier) | Passed / Skipped (no matching files), never `--no-verify` |

`make db-up` / full `make test` / `make coverage-gate` / `make sqlc-check` not run — no Go source, `queries/`, migrations, or `web/` files changed; the only affected test needs no Postgres and was run directly.

## Decisions Made

None beyond the plan — all four "Claude's Discretion" points from CONTEXT were resolved as the plan text directed (step id `guardcheck`; anchored `^func maxSourceVersion(` grep; flat three-way AND per step, no composed boolean; README note placed directly after the "Two mechanisms" bullet list, amendment appended inside `<revisions>`).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Live confirmation still pending — G-16-1 stays OPEN

This fix is proven **statically only** (actionlint, the grep shape gates, and the hand-trace above). It **cannot be proven GREEN live** until the next migration-touching branch is pushed while `v1.7.0` is the resolved N-1 tag. On that run, `n1-boot` must:

- finish **GREEN**,
- show the branch-4 `::notice::` (predates the ahead-of-source guard, cites `G-16-1`) in the run log,
- show `Start throwaway Postgres`, `Pull previous release image`, and all probe steps as **skipped, not run** (no ghcr.io pull, no health/read/write probe).

The Phase 16 UAT gap **G-16-1 must be re-checked against that live run before it is marked resolved**. Until then it stays OPEN. A docs-only push should still produce a GREEN `n1-boot` in runner spin-up time (all steps skipped on `migrations_changed`), and `build-scan` must still list `n1-boot` in `needs:` with the guard jobs finishing `success` (never `skipped`).

## Self-Check: PASSED

- `.github/workflows/full-pipeline.yml` modified, committed in `865c162` — FOUND
- `internal/db/migrations/README.md` modified, committed in `9876c4e` — FOUND
- `.planning/phases/16-rollback-safe-migrations/16-CONTEXT.md` modified, committed in `9876c4e` — FOUND
- Commit `865c162` (`fix(16): skip-green n1-boot ...`) — FOUND in `git log`
- Commit `9876c4e` (`docs(16): document the n1-boot guard-adoption window ...`) — FOUND in `git log`
- No AI attribution trailer in either commit message — CONFIRMED

---
*Quick task: 260905-et1*
*Completed: 2026-09-05*
