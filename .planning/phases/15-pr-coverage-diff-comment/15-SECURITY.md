---
phase: "15"
slug: "pr-coverage-diff-comment"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-03"
---

# Phase 15 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Built retroactively from the `<threat_model>` blocks in `15-01-PLAN.md`, `15-02-PLAN.md`,
and `15-03-PLAN.md` (all three plans carried a plan-time STRIDE register — this is not a
retroactive-STRIDE reconstruction). Verified at L1 grep depth against the implementation:
`.github/workflows/full-pipeline.yml`, `cmd/coverage-report/`, and `Makefile`.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| coverage profile / summary / sidecar files → rendered PR comment body | File content derives from repo source (file paths) and from a cache entry; the body is HTML-rendered by GitHub on a PR page | Untrusted file-path strings, cached JSON metrics |
| CLI arguments from workflow expressions → the tool | `--head-sha`, `--sha`, `--upstream-red` originate in the GitHub event context and reach the comment body | Head SHA, boolean flags from event context |
| tool stdout → Makefile command substitution | A subprocess's stdout becomes the value the merge-blocking 80% gate compares | Backend coverage percentage (2-dp) |
| Actions cache (base-branch scoped) → the comment job | Cached bytes written by a previous main-branch run are read on the PR head | Baseline coverage sidecars, profiles |
| `GITHUB_TOKEN` (pull-requests: write) → GitHub comment API | The only credential the comment job touches | Ambient job token |
| fork pull request → the same workflow | A fork's token is read-only regardless of the permission block | Read-only token, contributor-controlled source |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-15-V1 | Elevation of Privilege | prohibited privileged PR trigger | high | mitigate | `pull_request_target` appears nowhere; workflow triggers on ordinary `pull_request` and the job asserts `github.event_name == 'pull_request'`. Grep for the trigger returns zero. | closed |
| T-15-03 | Tampering | `cmd/coverage-report` comment renderer | medium | mitigate | Body built only from compile-time literals, tool-computed numbers, a tool-generated timestamp, and a format-validated SHA (`validSHA`, `main.go:85`). `TestRenderComment_NoUntrustedInterpolation` renders the hostile-paths fixture and asserts no `<script>`/backtick/markdown escapes leak. | closed |
| T-15-V5 | Tampering | rendered comment body / workflow | medium | mitigate | Every dynamic workflow value (`HEAD_SHA`, `UPSTREAM_RED`, baseline paths) crosses into the render step via a step-level `env:` block (`full-pipeline.yml:255-259`), never interpolated into the shell command. | closed |
| T-15-07 | Denial of Service | `cmd/coverage-report` comment mode | medium | mitigate | Every parse failure degrades to an `unavailable` row; comment mode returns nil unconditionally (process exits 0 on any input). A malformed cached profile cannot redden a PR. | closed |
| T-15-V4 | Elevation of Privilege | job permissions | medium | mitigate | `pull-requests: write` is granted on `coverage-comment` alone (`full-pipeline.yml:197-199`); the workflow-level block stays `contents: read` (line 7-8). | closed |
| T-15-V14a | Tampering | supply chain — third-party actions | medium | mitigate | Every `uses:` in the workflow is a 40-hex SHA pin with a trailing version comment (grep for non-pinned refs returns zero). Legitimacy audit OK verdicts recorded in RESEARCH. | closed |
| T-15-06 | Denial of Service | PR / main-branch check status | medium | mitigate | Job-level `continue-on-error: true` plus per-step tolerance on both downloads, the render, and the comment step. Nothing `needs:` the job, so it cannot block a merge. | closed |
| T-15-10 | Tampering | `coverage-gate` measurement seam | medium | mitigate | Only stdout is captured by the command substitution; diagnostics go to stderr, and the empty-output guard (`Makefile:97-100`) converts a tool build failure into an explicit gate failure, not a silent pass. | closed |
| T-15-11 | Denial of Service | `main` branch CI | medium | mitigate | D-17 rounding cutover margin measured on a live integration run before the refactor landed; SUMMARY records the recorded margin ≥ 0.05 points above 80. | closed |
| T-15-09 | Tampering | supply chain — Go module graph | low | mitigate | `cmd/coverage-report` is standard-library only (`go list -deps` shows no external module); `go.mod`/`go.sum` unchanged since Phase 07. | closed |
| T-15-12 | Tampering | coverage denominator | low | mitigate | `COVER_PKGS` exclusion is an anchored alternation `(^|/)(internal/db/sqlc\|cmd/coverage-report)$` (`Makefile:36`), not a substring match — a similarly named future package cannot be silently dropped. | closed |
| T-15-V14b | Tampering | baseline cache poisoning | low | mitigate | Cache restore keys are scoped `coverage-baseline-main-*`; save steps run only on `push` to `refs/heads/main` after `success()` (`full-pipeline.yml:80-81, 176-177`). A PR run cannot write a baseline other runs restore. | closed |
| T-15-V4b | Information Disclosure | fork pull requests | low | mitigate | Comment step guarded to same-repo PRs (`github.event.pull_request.head.repo.full_name == github.repository`, `full-pipeline.yml:276`); a fork degrades to the job summary the tool writes unconditionally. | closed |
| T-15-08 | Information Disclosure | rendered body and job summary | low | accept | Body contains only coverage percentages, commit SHAs, and a timestamp. The tool reads no environment variable other than `GITHUB_STEP_SUMMARY`. | closed |
| T-15-13 | Elevation of Privilege | build tooling | low | accept | No new dependency, credential, or network access. The tool is invoked with a repo-relative package path only. | closed |
| T-15-08b | Information Disclosure | job environment | low | accept | No `env:` block in the new job references a repository secret; the only credential is the ambient token consumed by the comment action's own input. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-15-01 | T-15-08 | Rendered comment/job-summary content is non-sensitive by construction — coverage percentages, commit SHAs, and a timestamp only. No secret is in this package's scope. | Caliber66 | 2026-09-03 |
| R-15-02 | T-15-13 | The coverage tool adds no dependency, credential, or network access; invoked with a repo-relative path only. | Caliber66 | 2026-09-03 |
| R-15-03 | T-15-08b | The `coverage-comment` job references no repository secret; its only credential is the ambient `GITHUB_TOKEN` consumed by the sticky-comment action's own input. | Caliber66 | 2026-09-03 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-03 | 16 | 16 | 0 | /gsd-secure-phase (L1, State B) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-03
