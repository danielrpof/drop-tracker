---
phase: 07-containerization-ci-cd-pipeline
plan: 04
subsystem: infra
tags: [github-actions, svu, ghcr, sbom, docker, semver, ci-cd]

# Dependency graph
requires:
  - phase: 07-03
    provides: "Full Pipeline workflow with vet/lint/test/gitleaks/pr-title gates and a build-scan job that blocks on Trivy CRITICAL/HIGH"
provides:
  - "release job in .github/workflows/full-pipeline.yml — svu version compute, ghcr.io push, spdx-json SBOM, git tag, gated behind needs:[build-scan] and a main-push-only if guard"
  - "seeded v0.1.0 git tag as the svu baseline, and the first real computed release v0.2.0"
  - "real end-to-end proof: a PR that built/scanned without publishing, and a merge to main that published"
affects: [deployment, release-process]

# Actuals (#2632)
actuals:
  tokens: 1545
  tasks: 3
  commits: 5

tech-stack:
  added: ["caarlos0/svu/v3 (v3.4.1)", "docker/setup-buildx-action v4.2.0", "docker/login-action v4.6.0", "docker/build-push-action v7.3.0", "anchore/sbom-action v0.24.0"]
  patterns:
    - "Elevated registry/tag credentials (packages: write, contents: write) live only inside a job gated by needs:[build-scan] + push-to-main-only if guard, structurally unreachable from any pull_request event"
    - "Version tag creation is ordered last, after a successful image push and SBOM generation, so a git tag never outlives a failed publish"
    - "Exactly one immutable semver image tag is published — no mutable :latest tag"

key-files:
  created: []
  modified:
    - .github/workflows/full-pipeline.yml
    - internal/db/pool_timeout_test.go
    - internal/httpserver/search.go
    - README.md

key-decisions:
  - "v0.1.0 seeded as the manual baseline tag per 07-CONTEXT.md D-04; every subsequent version is svu-computed"
  - "release job kept structurally separate from build-scan (rather than folded into it) so the elevated permissions block is never present in a job reachable from pull_request events"
  - "the two data races surfaced by CI's real -race gate were fixed in-place (Rule 1) rather than deferred, since they blocked getting a green PR to prove the pipeline"

requirements-completed: [CICD-05, CICD-06, CICD-07]

coverage:
  - id: D1
    description: "release job computes a semantic version with svu, gated on a real seeded v0.1.0 baseline, and fails loudly rather than silently no-op/bump when no conventional-commit prefix is found"
    requirement: "CICD-06"
    verification:
      - kind: manual_procedural
        ref: "Task 1 local sanity check (svu current -> v0.1.0, svu next -> v0.2.0) + Task 3 real merge run log (svu current -> v0.1.0, VERSION=v0.2.0), run https://github.com/danielrpof/drop-tracker/actions/runs/31610821553"
        status: pass
    human_judgment: false
  - id: D2
    description: "scanned image pushed to ghcr.io tagged with exactly the computed semantic version, no :latest, publish path unreachable from pull requests"
    requirement: "CICD-07"
    verification:
      - kind: manual_procedural
        ref: "PR #1 (github.com/danielrpof/drop-tracker/pull/1) six checks green with no release run and no new package; merge run published ghcr.io/danielrpof/drop-tracker:v0.2.0 digest sha256:08e6bec5e239e85bbe94ad68738b9cb5e0b46e22fa8a989ce72c140028d4209c; verified public+pullable with docker logout and non-root uid 10001"
        status: pass
    human_judgment: false
  - id: D3
    description: "spdx-json SBOM generated against the exact pushed image reference and attached to the release run"
    requirement: "CICD-05"
    verification:
      - kind: manual_procedural
        ref: "Release run artifact danielrpof-drop-tracker_v0_2_0.spdx.json (30520 bytes, spdx-json), run https://github.com/danielrpof/drop-tracker/actions/runs/31610821553"
        status: pass
    human_judgment: false

duration: 32min
completed: 2026-08-12
status: complete
---

# Phase 07 Plan 04: Release Job (svu -> ghcr.io -> SBOM -> tag) Summary

**Added the release job to the Full Pipeline workflow and proved it end to end against the real GitHub remote: a PR built and scanned without publishing, then a merge to main computed v0.2.0 with svu, pushed the scanned image to ghcr.io, generated an spdx-json SBOM, and tagged the repo — closing CICD-05/06/07.**

## Performance

- **Duration:** 32 min (09:37:38 tag seed to 10:09:45 published release, America/Chicago)
- **Started:** 2026-08-12T14:37:38Z
- **Completed:** 2026-08-12T15:09:45Z
- **Tasks:** 3
- **Files modified:** 4 (`.github/workflows/full-pipeline.yml`, `internal/db/pool_timeout_test.go`, `internal/httpserver/search.go`, `README.md`)

## Accomplishments

- Seeded and verified the `v0.1.0` baseline tag that every subsequent `svu`-computed version builds on (Task 1).
- Added the `release` job: `needs: [build-scan]` + push-to-main-only `if:` guard, `fetch-depth: 0` checkout, `svu next`/`svu current` version compute with an explicit loud failure on an unrecognized commit prefix, `docker/setup-buildx-action`, `docker/login-action`, `docker/build-push-action` (single immutable semver tag, no `:latest`, GHA cache reuse), `anchore/sbom-action` (spdx-json against the pushed ref), and the git tag/push step ordered last so a tag never outlives a failed publish (Task 2).
- Verified the whole pipeline against the real GitHub remote: opened and merged a real PR (all six gates green, no publish on the PR event), then merged this worktree's release-job commit into `main` and confirmed the `release` job actually ran end to end — version computed, image pushed, SBOM attached, tag created, package public and pullable, container running as uid 10001 (Task 3, verified directly by the orchestrator).

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "a semantic version can be computed from this repo's real history"** — tag-only, no repo file commit (annotated tag `v0.1.0` created and pushed by the orchestrator directly; see Task 1 notes below)
2. **Task 2: Add the release job — version, push to ghcr.io, SBOM, then tag** - `48fb88c` (feat)
3. **Task 3: Verify the pipeline against a real pull request and a real merge** - verified directly against the real GitHub remote by the orchestrator (not a commit in this worktree); the resulting real-remote commits are `f47b97e` (chore, PR verification change), `669ea5d` (fix, real data races surfaced by CI's `-race` gate), `4bf2d5f` (PR #1 merge into `main`), and `3114663` (merge bringing this worktree's `48fb88c` into `main`)

**Plan metadata:** this commit (docs: complete plan)

_Note: Task 3 required pushing to the real GitHub remote and driving `gh pr`/`gh run`, so it was executed by the orchestrator directly rather than from inside this worktree._

## Files Created/Modified

- `.github/workflows/full-pipeline.yml` - adds the `release` job (svu version compute, ghcr.io login/push, SBOM, tag)
- `internal/db/pool_timeout_test.go` - fixed a real data race in the `blackHoleAddr` test helper (channel closed while a goroutine could still send to it), surfaced by CI's `-race` gate
- `internal/httpserver/search.go` - fixed a real data race in `handleSearch`'s fan-out goroutines concurrently calling `httplog.SetAttrs` on shared request-context state, surfaced by CI's `-race` gate
- `README.md` - trivial comment change used as the PR's verification payload (on the now-deleted scratch branch `ci/verify-full-pipeline`, merged into `main` via PR #1)

## Decisions Made

- Kept the `release` job structurally separate from `build-scan` rather than folding the publish steps into it, so the job-level `permissions:` block granting `packages: write`/`contents: write` is never present in any job reachable from a `pull_request` event (T-07-17).
- Tag creation and push are the last step of the job, after a successful image push and SBOM generation, so a failed publish never leaves an orphaned version tag (T-07-19).
- Published exactly one immutable semver image tag with no `:latest`, per D-09/T-07-20.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed two real data races found by CI's `-race` gate**
- **Found during:** Task 3 (real PR verification run)
- **Issue:** `-race` is broken on the Windows dev machine (ThreadSanitizer allocator failure, documented since Phase 01), so these races were invisible locally and had never actually been checked until this was the first time `-race` ran for real, on the workflow's `ubuntu-latest` runner. `internal/db/pool_timeout_test.go`'s `blackHoleAddr` test helper closed a channel its goroutine could still be sending to; `internal/httpserver/search.go`'s `handleSearch` called `httplog.SetAttrs` concurrently from multiple fan-out goroutines against context state with no synchronization.
- **Fix:** Reordered the test helper so `ln.Close()` unblocks `Accept` and `wg.Wait()` proves the goroutine exited before the channel is closed/drained; collected per-source error text under the existing mutex in `handleSearch` and called `SetAttrs` once, sequentially, after the goroutines rejoin.
- **Files modified:** `internal/db/pool_timeout_test.go`, `internal/httpserver/search.go`
- **Verification:** CI's `-race` gate (`test` job) went from red to green on the re-run of PR #1's Checks tab; both fixes shipped in commit `669ea5d`.
- **Committed in:** `669ea5d` (on the real GitHub remote, PR #1 branch `ci/verify-full-pipeline`, later merged into `main`)

---

**Total deviations:** 1 auto-fixed (Rule 1, two related race fixes in one commit).
**Impact on plan:** Necessary for correctness — the pipeline cannot honestly claim to have a working `-race` gate if the first real run of it is left red. No scope creep: both fixes are confined to code the CI run itself flagged. This is also the pipeline doing exactly what it exists to do: catching bugs a broken local dev environment could never surface.

## Issues Encountered

- The scratch PR branch for Task 3 was created off `main` at commit `63a0857`, before this worktree's Task 2 commit (`48fb88c`, the `release` job itself) had been merged into `main`. The first push-to-main after merging PR #1 correctly did not exercise `release` (it wasn't in the tested code yet). Resolved by merging this worktree's branch into `main` (`3114663`, no-conflict — disjoint files) and pushing again; that second push (`3114663` on `main`) was the one that actually exercised `release` end to end. Not a plan defect — a natural ordering artifact of verifying a not-yet-merged job against a real remote — and it is now fully accounted for in the verification trail above.
- A prior push-to-main run (`31607622002`, before the race fixes landed) was used to independently confirm that `build-scan`/`release` correctly never ran while `test` was red — the scan gate structurally blocked the broken build from ever reaching publish, exactly as this plan's `must_haves` require.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 07's three requirements this plan owned (CICD-05 SBOM, CICD-06 semantic versioning + tagging, CICD-07 ghcr.io publish) are closed and verified against the real registry and a real merge.
- `ghcr.io/danielrpof/drop-tracker:v0.2.0` is public, pullable without auth, and runs as non-root uid `10001`.
- No blockers for phase completion. The full CI/CD pipeline (source gates -> build-scan -> release) is proven end to end on GitHub's runners, not just locally.

---
*Phase: 07-containerization-ci-cd-pipeline*
*Completed: 2026-08-12*
