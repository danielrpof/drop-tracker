---
created: 2026-09-05T16:46:00.000Z
title: Move shadcn from dependencies to devDependencies in web/package.json
area: tooling
severity: minor
files:
  - web/package.json
  - web/pnpm-lock.yaml
---

## Problem

`shadcn` sits in `dependencies` in `web/package.json`. It is a CLI (`bin: dist/index.js`),
invoked ad hoc as `npx shadcn@latest add <component>` per `web/README.md`, and is imported
by no application code. It belongs in `devDependencies`.

Because it is in `dependencies`, its whole transitive graph is treated as production
surface — which is the only reason `browserslist` and `fast-uri` findings reached the
`trivy-fs` gate in the first place (quick task 260905-fa4).

## WARNING — reclassification hides findings, it does not fix them

Trivy excludes devDependencies by default for `pnpm-lock.yaml` v9 (the CI job does not
set `--include-dev-deps`). Moving `shadcn` to `devDependencies` makes any current or
future finding under its subtree **disappear from the `trivy-fs` report while the
vulnerable versions stay pinned in the lockfile**. This was confirmed empirically during
260905-fa4 research: a scratch reclassification scanned clean while the lockfile still
pinned `browserslist@4.28.2` / `fast-uri@3.1.5`.

Treat this as a **correctness cleanup only**. It must **never substitute** for a real
version fix (an `overrides:` bump + lockfile regen, as done in 260905-fa4). Do this change
only when the subtree is already clean, and never reach for it to make a red `trivy-fs`
go green.

## Solution

1. Move `"shadcn"` from `dependencies` to `devDependencies` in `web/package.json`.
2. `corepack pnpm@11.8.0 --dir web install --lockfile-only`, then a full
   `--frozen-lockfile` install; `corepack pnpm --dir web test` and
   `prettier --check` stay green.
3. Local `trivy fs .` at `CRITICAL,HIGH` still reports 0 — verify the subtree was already
   clean, so this change removes attack surface rather than masking a finding.
4. Related open question: whether the pinned CLI belongs in the manifest at all (a
   `pnpm dlx shadcn@latest add ...` workflow needs no manifest entry) — see the deferred
   "remove shadcn from the manifest entirely" option in 260905-fa4 RESEARCH §3c.

Handle via `/gsd-quick`.
