---
created: 2026-09-05T15:42:17.604Z
title: Bump frontend deps to clear trivy-fs HIGH CVEs
area: tooling
severity: major
files:
  - web/package.json
  - web/pnpm-lock.yaml
  - .github/workflows/full-pipeline.yml (trivy-fs job)
---

## Problem

The `trivy-fs` job in `.github/workflows/full-pipeline.yml` fails on `main` with 6 HIGH
vulnerabilities in frontend transitive dependencies. `trivy-fs` runs with
`severity: CRITICAL,HIGH` and `exit-code: 1`, and `build-scan` (and therefore `release`)
`needs:` it — so the whole publish half of the pipeline is blocked once the currently
unpushed `main` commits (36 ahead as of 2026-09-05) are pushed.

Vulnerable packages (from Trivy output on runs 33974944661 / 33975084671):

| Library | CVEs | Installed | Fixed in |
|---------|------|-----------|----------|
| `browserslist` | CVE-2026-73088 (prototype pollution), CVE-2026-73089 (DoS) | 4.28.2 | 4.28.7 |
| `fast-uri` | CVE-2026-75899, CVE-2026-75931, CVE-2026-75975, CVE-2026-76172 (SSRF / host-confusion / URI-parsing) | 3.1.5 | 2.4.5, 3.1.6, 4.1.3 |

Both are transitive under `web/`. All six CVEs were disclosed after the frontend was last
touched — this is dependency drift, not a regression in project code.

Surfaced during Phase 16 UAT (`/gsd-verify-work 16`) while verifying the migration guard
jobs; it is unrelated to Phase 16 and was recorded there as a non-gap note.

## Solution

Likely a `pnpm.overrides` (or `resolutions`) bump in `web/package.json` pinning
`browserslist >= 4.28.7` and `fast-uri >= 3.1.6`, then regenerate `web/pnpm-lock.yaml`
(`corepack pnpm --dir web install`). Verify:

1. `corepack pnpm --dir web install` resolves cleanly, no peer-dep breakage.
2. `cd web && corepack pnpm test` and `corepack pnpm exec prettier --check "**/*.{ts,tsx}"` still pass.
3. Re-run Trivy locally if available (`trivy fs web/` or `trivy fs .`) — 0 HIGH/CRITICAL.
4. Push a scratch branch and confirm the `trivy-fs` job goes green and `build-scan` runs.

Prefer the narrowest override that clears the CVEs; check whether a direct dependency
already offers a newer minor that pulls the fixed transitive versions without an override.

Handle via `/gsd-quick`.
