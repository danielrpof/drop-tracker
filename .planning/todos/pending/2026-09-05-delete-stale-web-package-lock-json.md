---
created: 2026-09-05T16:45:00.000Z
title: Delete the stale tracked web/package-lock.json
area: tooling
severity: minor
files:
  - web/package-lock.json
---

## Problem

`web/package-lock.json` is tracked in git alongside the authoritative
`web/pnpm-lock.yaml`. It is a second, drifting npm lockfile that nobody maintains —
the project uses pnpm everywhere (Dockerfile, CI `frontend-test`, `make web`, the
Definition of Done). Two lockfiles for one `package.json` is a standing source of
confusion about which one is authoritative.

Trivy scans it too: `trivy-fs` runs `scan-ref: .` (repo-wide), so
`web/package-lock.json` is a live input to the security gate. It contributed **zero**
of the 6 HIGH findings fixed in quick task 260905-fa4 — it already happened to carry
`browserslist@4.28.8` and `fast-uri@3.1.6`, both past the fixed versions — but that is
luck, not maintenance. The next CVE disclosed against a package pinned only in this
file will fail `trivy-fs` with no corresponding pnpm-side fix, and the regen command
(`corepack pnpm ... install`) will not touch it.

## Solution

`git rm web/package-lock.json`. Confirm nothing references it:

1. No `npm ci` / `npm install` in any Dockerfile stage, Makefile target, CI job, or
   `package.json` script (the repo is pnpm-only).
2. `git ls-files | xargs grep -l package-lock.json` returns only the file itself.
3. After removal: `corepack pnpm --dir web install --frozen-lockfile`, `corepack pnpm
   --dir web test`, and a local `trivy fs .` all still pass.

Optionally add `package-lock.json` to `web/.gitignore` so it does not silently return.

Handle via `/gsd-quick`.
