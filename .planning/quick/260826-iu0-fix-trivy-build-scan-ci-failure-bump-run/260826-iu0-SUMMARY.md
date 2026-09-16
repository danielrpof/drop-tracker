---
phase: quick-260826-iu0
plan: 01
subsystem: infra
tags: [docker, alpine, trivy, cve, ci-cd, dockerfile]

# Dependency graph
requires:
  - phase: quick-260817-cfu
    provides: "Prior Trivy build-scan CVE fix precedent (Go builder-stage base image bump) that established the local Docker+Trivy verification workflow this plan reused"
provides:
  - "Runtime-stage Alpine package upgrade pattern (apk upgrade --no-cache && apk add --no-cache ...) that keeps a digest-pinned base image current against newly published distro security patches"
affects: [ci-cd, docker, security]

# Actuals (#2632)
actuals:
  tokens: 549
  tasks: 2
  commits: 1

# Tech tracking
tech-stack:
  added: []
  patterns: ["Digest-pinned base image + build-time apk upgrade for the runtime stage, to close the gap between a frozen image layer and the distro's continuously updated package repository"]

key-files:
  created: []
  modified: ["Dockerfile"]

key-decisions:
  - "Combined apk upgrade --no-cache with the existing apk add --no-cache ca-certificates into a single RUN/layer rather than adding a second RUN, per the plan's one-instruction-one-layer constraint"
  - "Did not touch the alpine:3.24 digest pin — re-pinning was already confirmed impossible (docker pull alpine:3.24 resolves to the identical digest), so build-time apk upgrade is the only lever that moves libssl3/libcrypto3 off 3.5.7-r0"

patterns-established:
  - "Pattern: when a Trivy finding names a package fix already published in the distro's live repo index but not yet baked into the pinned base-image digest, patch via 'apk upgrade --no-cache' in the same layer as the stage's existing apk install, with a comment documenting the pinned-digest-vs-repo-patch-cadence tradeoff so a future reader doesn't 'fix' it by reverting."

requirements-completed: [QUICK-260826-iu0]

coverage:
  - id: D1
    description: "Runtime stage's RUN line upgrades installed Alpine packages via the live repo index before installing ca-certificates, in one combined layer, closing the libssl3/libcrypto3 CVE-2026-14456 HIGH findings without unpinning the base image"
    requirement: "QUICK-260826-iu0"
    verification:
      - kind: other
        ref: "docker build -t drop-tracker:scan . (three-stage build completes)"
        status: pass
      - kind: other
        ref: "MSYS_NO_PATHCONV=1 docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.70.0 image --severity CRITICAL,HIGH --exit-code 1 drop-tracker:scan"
        status: pass
      - kind: other
        ref: "docker run --rm --entrypoint sh drop-tracker:scan -c \"apk list -I | grep -E 'libssl3|libcrypto3'\" (reports 3.5.8-r0 for both)"
        status: pass
      - kind: other
        ref: "unfiltered trivy image scan grep for CVE-2026-14456 (not found)"
        status: pass
      - kind: other
        ref: "docker run --rm --entrypoint sh drop-tracker:scan -c \"test -x /usr/local/bin/server && wget --help\" (runtime-ok) and docker run --rm --entrypoint id (uid=10001/gid=10001)"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-08-26
status: complete
---

# Quick Task 260826-iu0: Fix Trivy build-scan CI failure Summary

**Patched the runtime stage's Alpine packages at build time (`apk upgrade --no-cache`) to pull libssl3/libcrypto3 3.5.8-r0 from Alpine's live repo index, clearing the two HIGH CVE-2026-14456 findings blocking CI's Trivy `build-scan` gate — without unpinning the `alpine:3.24` base image digest.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-08-26T18:35:00Z (approx)
- **Completed:** 2026-08-26T18:43:46Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- Runtime stage's single `RUN apk add --no-cache ca-certificates` line replaced with `RUN apk upgrade --no-cache && apk add --no-cache ca-certificates`, in the same layer, preceded by a new comment explaining the pinned-digest-vs-repo-patch-cadence rationale as a general pattern
- Verified with a real local `docker build` that the image still builds cleanly through all three stages
- Verified with CI's exact Trivy gate (`aquasec/trivy:0.70.0 image --severity CRITICAL,HIGH --exit-code 1`) that the scan now exits 0 with zero CRITICAL/HIGH findings
- Confirmed `libssl3-3.5.8-r0` and `libcrypto3-3.5.8-r0` are installed, and confirmed CVE-2026-14456 is absent from a full unfiltered Trivy scan (not just filtered out by severity)
- Confirmed the blast radius of the blanket `apk upgrade` didn't break anything the runtime depends on: entrypoint binary present/executable, busybox `wget` (used by `HEALTHCHECK`) still runs, container still starts as uid/gid 10001

## Task Commits

Each task was committed atomically:

1. **Task 1: Patch the runtime stage's Alpine packages at build time, in the existing ca-certificates layer** - `9afdfa2` (fix)
2. **Task 2: Prove the CVE is gone under CI's exact Trivy gate and the image still functions** - verification-only, no file changes, no separate commit

## Files Created/Modified
- `Dockerfile` - Runtime stage (stage 3) `RUN` line now upgrades installed Alpine packages from the live repo index before installing ca-certificates, with an explanatory comment; no other lines changed

## Decisions Made
- Combined the upgrade and install into one `RUN`/one layer rather than two, matching the file's existing "one instruction, one purpose" convention and avoiding an extra image layer
- Did not attempt to re-pin the `alpine:3.24` digest — already verified impossible (`docker pull alpine:3.24` resolves to the same digest currently pinned), confirming build-time `apk upgrade` is the only available lever
- Left `.github/workflows/full-pipeline.yml`, `HEALTHCHECK`, `USER`, and all other Dockerfile lines untouched, per the plan's explicit scope discipline

## Deviations from Plan

None - plan executed exactly as written. Both tasks completed with no auto-fixes, no blockers, and no architectural questions.

## Issues Encountered

The plan's suggested `apk info -v libssl3 libcrypto3` command doesn't print version strings in this Alpine/apk-tools release (it prints description/URL/size fields instead of version-name lines). Used `apk list -I | grep -E 'libssl3|libcrypto3'` instead, which is a standard equivalent apk query and returned the expected `libcrypto3-3.5.8-r0` / `libssl3-3.5.8-r0` output. This is a verification-command substitution only; it does not affect the `<verify>` automated check in Task 2's frontmatter, which used the combined Trivy + `apk info -v` line and also passed (both packages listed, just without visible version numbers in that particular invocation's output — the `apk list -I` run above is what confirms the actual version).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

CI's `build-scan` Trivy gate should now pass on the next pipeline run against this commit — the local reproduction used CI's exact image-build command and exact Trivy flags/severity/exit-code, with zero CRITICAL/HIGH findings observed. No blockers. The `alpine:3.24` digest pin remains unchanged, so this fix will need to be revisited if a future distro-level CVE surfaces again while the same digest stays pinned (the pattern established here — build-time `apk upgrade` in the runtime stage's existing package-install layer — applies directly).

## Self-Check: PASSED

- FOUND: Dockerfile
- FOUND: .planning/quick/260826-iu0-fix-trivy-build-scan-ci-failure-bump-run/260826-iu0-SUMMARY.md
- FOUND: commit 9afdfa2

---
*Phase: quick-260826-iu0*
*Completed: 2026-08-26*
