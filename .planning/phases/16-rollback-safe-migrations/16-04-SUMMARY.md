---
phase: 16-rollback-safe-migrations
plan: 04
subsystem: ci
tags: [github-actions, ci-wiring, migrations, rollback-safety, n1-boot]

# Dependency graph
requires:
  - phase: 16-rollback-safe-migrations (plan 01)
    provides: cmd/migrate go run HEAD-schema helper, invoked unchanged by n1-boot's Apply HEAD schema step
  - phase: 16-rollback-safe-migrations (plan 02)
    provides: cmd/migration-check's --mode=scan two-class DDL guard, invoked unchanged by the migration-check job
  - phase: 16-rollback-safe-migrations (plan 03)
    provides: cmd/migration-check's --mode=changed-files diff-base selection and --prev-tag cross-reference, invoked unchanged by the changes and migration-check jobs
provides:
  - changes prelude job (D-16) computing the migration diff base once per run via cmd/migration-check --mode=changed-files, publishing migrations_changed/migration_files as job outputs
  - migration-check guard job running cmd/migration-check --mode=scan against the diff and the resolved previous release tag, unconditional and read-only
  - n1-boot job pulling the previous release image and proving it boots, stays sustained-healthy, and both reads and writes against a throwaway Postgres migrated to HEAD's schema
  - build-scan.needs: extended with migration-check and n1-boot, both blocking the release path
affects: []

# Actuals (#2632)
actuals:
  tokens: 3070
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Unconditional job / gated step (RESEARCH.md Finding 2): a skipped needs: job skips its dependents, so changes, migration-check, and n1-boot all carry no job-level if: -- only n1-boot's expensive steps are gated on needs.changes.outputs.migrations_changed, so a non-migration push completes the job in runner spin-up time alone"
    - "In-job true-bootstrap short-circuit: a bootstrap step writes proceed=true/false to its own step output based on whether the resolved previous-release tag is empty, and every subsequent expensive step's if: ANDs that output in -- D-19a's 'in-job step, not a job-level if:' requirement implemented as a step-output gate chain rather than a single flag"
    - "Postgres via a step-gated docker run, never the services: key -- a services: container ignores step if: and starts on every push regardless of migration content (RESEARCH Pitfall D)"

key-files:
  created: []
  modified:
    - .github/workflows/full-pipeline.yml

key-decisions:
  - "The changes and migration-check jobs' bash steps are thin wrappers around cmd/migration-check's own --mode=changed-files and --mode=scan flags -- the diff-base selection, GITHUB_OUTPUT writing, and scan logic all live in the Go tool built by Plans 02/03, not reimplemented in workflow YAML. RESEARCH.md's illustrative bash diff-base script was superseded by Plan 03's actual --mode=changed-files implementation before this plan started."
  - "Split the single conceptual workflow edit into three separate incremental edits (changes+migration-check job insertion, then n1-boot job insertion, then build-scan.needs: update) so each PLAN.md task landed as its own atomic commit, matching task_commit_protocol -- verified each task's full acceptance-criteria command set against the intermediate file state before committing, not just against the final state."
  - "One in-flight self-correction: an early draft of the n1-boot job comment said 'no docker login' in prose, which collided with Task 2's own acceptance criterion (grep -c 'docker login' == 0, a literal text grep over the whole file including comments). Reworded to 'no registry authentication step' before committing -- caught by running the plan's own verify command before, not after, the commit."
  - "MGRT-01 and MGRT-02 both marked complete: requirements.ready-ids confirmed MGRT-01 unblocked once this plan (the last of the four MGRT-01-declaring plans) landed; MGRT-02 was already marked complete by 16-05."

patterns-established:
  - "GitHub Actions job graph for a diff-gated CI check: one cheap prelude job computes shared diff-base outputs; downstream jobs consume needs.<prelude>.outputs.* in step-level env: blocks, never re-deriving the diff themselves."

requirements-completed: [MGRT-01, MGRT-02]

coverage:
  - id: D1
    description: "changes job computes the migration diff base exactly once per run via cmd/migration-check --mode=changed-files, publishing migrations_changed and migration_files as job outputs consumed by both migration-check and n1-boot"
    requirement: "MGRT-01"
    verification:
      - kind: other
        ref: "python: d['jobs']['changes']['outputs'] contains migrations_changed and migration_files; no job-level if: on changes or migration-check"
        status: pass
      - kind: other
        ref: "go run ./cmd/migration-check --mode=changed-files --event-name=push --before=0000...0 --sha=$(git rev-parse HEAD) -> EXIT=0"
        status: pass
    human_judgment: false
  - id: D2
    description: "migration-check job resolves the previous release tag with the same pinned svu the release job uses and runs the D-08/D-15 scan against the diff, unconditionally and read-only (contents: read only)"
    requirement: "MGRT-01"
    verification:
      - kind: other
        ref: "grep -c 'svu/v3@v3.4.1' .github/workflows/full-pipeline.yml == 2 (release + migration-check); grep -c 'fetch-tags: true' == 2 after Task 1"
        status: pass
      - kind: other
        ref: "go run ./cmd/migration-check --mode=scan --files='' --prev-tag=v1.7.0 -> EXIT=0, prints scanned-file line"
        status: pass
    human_judgment: false
  - id: D3
    description: "n1-boot is unconditional (no job-level if:, no services: key), needs: [changes], permissions: contents: read only, and every step but checkout/setup-go/teardown carries a step-level if: on needs.changes.outputs.migrations_changed"
    requirement: "MGRT-01"
    verification:
      - kind: other
        ref: "python: n1-boot has no 'if' key, no 'services' key, needs==['changes'], permissions=={'contents':'read'}, 11 steps carry a step-level if:"
        status: pass
    human_judgment: false
  - id: D4
    description: "n1-boot never configures the Phase 14 instance-gate variable, never pulls a floating tag, and its failure path names internal/db/migrations/README.md"
    requirement: "MGRT-01"
    verification:
      - kind: other
        ref: "python json.dumps(jobs['n1-boot']): PASSPHRASE_HITS=0, LATEST_HITS=0, README_HITS=1"
        status: pass
    human_judgment: false
  - id: D5
    description: "The workflow contains no docker login step and no new secret reference; the pull step attempts the pull at most twice"
    requirement: "MGRT-01"
    verification:
      - kind: other
        ref: "grep -c 'docker login' .github/workflows/full-pipeline.yml == 0; Pull previous release image step body is a single '||' fallback (2 attempts)"
        status: pass
    human_judgment: false
  - id: D6
    description: "build-scan.needs: is exactly the six original entries plus migration-check and n1-boot appended, in order, with no job in the needs: array carrying a job-level if:, and every needs: reference in the whole file resolves to an existing job"
    requirement: "MGRT-01"
    verification:
      - kind: other
        ref: "python: build-scan['needs'] == [vet, lint, test, gitleaks, trivy-fs, frontend-test, migration-check, n1-boot]; DANGLING=[] across the whole jobs graph"
        status: pass
    human_judgment: false
  - id: D7
    description: "MGRT-01 and MGRT-02 are both marked complete in REQUIREMENTS.md once this, the last plan declaring MGRT-01, lands"
    requirement: "MGRT-01, MGRT-02"
    verification:
      - kind: other
        ref: "requirements.ready-ids confirmed MGRT-01 ready; requirements.mark-complete applied both checkbox and traceability-table surfaces"
        status: pass
    human_judgment: false

duration: ~8min
completed: 2026-09-04
status: complete
---

# Phase 16 Plan 04: CI Wiring — the changes prelude, migration-check guard, and n1-boot job Summary

**Three new unconditional GitHub Actions jobs wire Plans 01-03's Go tooling into `full-pipeline.yml` — a `changes` prelude, a `migration-check` guard, and an `n1-boot` job that pulls and boots the previous release against HEAD's schema — and both checks now block the release path via `build-scan.needs:`, closing MGRT-01/MGRT-02.**

## Performance

- **Duration:** ~8 min (commit-to-commit span)
- **Started:** 2026-09-04T17:46:34Z (Task 1 commit)
- **Completed:** 2026-09-04T17:51Z (Task 3 commit)
- **Tasks:** 3 (all `type="auto"`, fully autonomous, no checkpoint tasks)
- **Files modified:** 1 (`.github/workflows/full-pipeline.yml`)

## Accomplishments
- Added the `changes` prelude job: computes the migration diff base exactly once per run by shelling `cmd/migration-check --mode=changed-files` (the Go tool Plan 03 built, which already writes `migrations_changed`/`migration_files` to `$GITHUB_OUTPUT` itself), publishing both as job outputs consumed downstream
- Added the `migration-check` guard job: resolves the previous release tag with the same pinned `svu` sequence the `release` job uses, then runs `cmd/migration-check --mode=scan` against the diff and that tag — unconditional, read-only, `contents: read` only
- Added the `n1-boot` job: pulls the previous release image (`svu current`, never a floating tag), brings up a throwaway Postgres via a gated `docker run` step (never `services:`), applies HEAD's schema through `go run ./cmd/migrate` (the exact boot path Plan 01 hardened), then proves the old binary reaches a sustained-healthy `/health`, reads `GET /watchlist`/`GET /events` on an empty DB, and writes via `POST /watchlist` (bypassing the art matcher with a supplied `image_url`) followed by a re-`GET` confirming the row landed
- All three new jobs are unconditional — no job-level `if:` on any of them (RESEARCH.md Finding 2: a skipped `needs:` job skips its dependents, so gating a job instead of its steps would silently disable `build-scan`/`release` on the ~95% of pushes that touch no migration); only `n1-boot`'s 11 expensive steps carry a step-level `if:` gate chain (`migrations_changed == 'true' && bootstrap.outputs.proceed == 'true'`)
- Appended `migration-check` and `n1-boot` to `build-scan.needs:` (now eight entries) and extended the precedent comment above it to record why both stay unconditional and what a job-level `if:` would silently do
- MGRT-01 and MGRT-02 both marked complete in `REQUIREMENTS.md`

## Task Commits

Each task was committed atomically:

1. **Task 1: The changes prelude job and the migration-check guard job** - `bc3336e` (feat)
2. **Task 2: The n1-boot job — pull the previous release and prove it boots, reads, and writes** - `38f8f5e` (feat)
3. **Task 3: Make both checks blocking (automated portion)** - `c79b754` (feat)

## Files Created/Modified
- `.github/workflows/full-pipeline.yml` - `changes` job (Task 1), `migration-check` job (Task 1), `n1-boot` job (Task 2), `build-scan.needs:` extension + comment (Task 3)

## Decisions Made
See `key-decisions` in the frontmatter above — the thin-wrapper design around the already-built Go tooling (RESEARCH.md's illustrative bash diff script was superseded by Plan 03's real `--mode=changed-files` before this plan started), the three-way split of one conceptual workflow edit into three atomic task commits with each task's full acceptance-criteria set re-verified against the intermediate file state, the in-flight `docker login` comment-text self-correction (caught by running the plan's own verify command before committing, not after), and the MGRT-01/MGRT-02 completion.

## Deviations from Plan

None — plan executed exactly as written for all three tasks' automated portions.

## Scope Note: Task 3's Human-Verification Portion Deferred

Task 3's `<verify>` block specifies a `<human-check>`: a live scratch-branch CI run (destructive migration → both checks red, `build-scan` not reached; docs-only push → both checks green in seconds with expensive steps skipped, `build-scan` runs, confirming Finding 2's skip-propagation fix; additive migration → both checks green with `n1-boot` actually pulling and booting the previous image), followed by deleting the scratch branch locally and remotely.

This project's `workflow.human_verify_mode` is `end-of-phase` (the default per `.planning/config.json`). Per the executor's standard protocol, a planner-emitted `<verify><human-check>` on an `auto` task is deferred to end-of-phase UAT rather than performed by the plan executor — this plan intentionally did not push any branch to origin or trigger a live GitHub Actions run. Only the automated structural `<verify>` commands (the Python/grep YAML-shape asserts) were run and are recorded in `coverage` above.

**Deferred to end-of-phase UAT — the live scratch-branch three-part CI verification described in `16-04-PLAN.md` Task 3's `<verify><human-check>` block:**
1. Confirm a recent `release` run actually pushed `ghcr.io/danielrpof/drop-tracker` at the tag `svu current` currently resolves to (`v1.7.0` as of this session) — RESEARCH Assumption A2.
2. On a scratch branch (never `main`, delete after — Phase 09 precedent, `test/coverage-gate-ci-check`): (a) a throwaway `DROP COLUMN` migration referencing a column the previous release's queries still use → expect `migration-check` red (D-15 cross-reference) and `n1-boot` red, `build-scan` not reached; (b) remove the dummy migration, push a docs-only commit → expect `changes` reports no migration change, `migration-check` and `n1-boot` both green in seconds with expensive steps skipped, and **`build-scan` runs** (not "Skipped") — this is the single highest-value observation confirming Finding 2's fix actually works live, not just structurally; (c) a purely additive nullable `ADD COLUMN` migration → expect both checks green with `n1-boot` having genuinely pulled and booted the previous image.
3. Record all three run URLs and per-job statuses, then delete the scratch branch locally and remotely.

RESEARCH.md Assumptions A2 (registry has an image for the resolved tag), A3 (`--network host` sibling-container reachability), and A6 (GitHub's skip-propagation behavior as currently documented) are the three facts this live run is the first real test of — none of them are verifiable by reading the YAML.

## Issues Encountered
- An early draft of the `n1-boot` "Pull previous release image" step's comment used the literal phrase "no docker login" in prose, which collided with Task 2's own acceptance criterion (`grep -c 'docker login' .github/workflows/full-pipeline.yml == 0`, a plain-text grep over the whole file including comments — YAML comments are invisible to the Python/PyYAML structural checks but not to `grep`). Caught by running the plan's own verify command before committing; reworded to "no registry authentication step" with no behavior change.

## User Setup Required

None for the automated portion. The deferred human-check (see above) requires GitHub Actions access to push a scratch branch and observe three live workflow runs — this is end-of-phase UAT, not local setup.

## Next Phase Readiness
- Phase 16 (Rollback-Safe Migrations) is now feature-complete: all 5 plans (16-01 through 16-05) executed. MGRT-01 and MGRT-02 both marked complete in `REQUIREMENTS.md`.
- The one open item is the deferred live scratch-branch CI verification documented above — this is the natural first UAT item for the phase's end-of-phase human verification pass, and it is the only way to confirm RESEARCH.md's Finding 2 fix (skip-propagation) actually holds on a real GitHub Actions run rather than by static YAML-shape assertion.
- No blockers for Phase 17 (Automated VPS Deploy with Health-Gated Rollback), which depends on this phase's N-1 rollback guarantee being in force before anything auto-rolls-back.

---
*Phase: 16-rollback-safe-migrations*
*Completed: 2026-09-04*

## Self-Check: PASSED

`.github/workflows/full-pipeline.yml` found on disk with all three jobs (`changes`, `migration-check`, `n1-boot`) and the extended `build-scan.needs:`; all three task commit hashes (`bc3336e`, `38f8f5e`, `c79b754`) found in `git log`.
