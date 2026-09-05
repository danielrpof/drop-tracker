---
phase: quick/260905-fa4
plan: 01
subsystem: frontend-deps / CI security gate
tags: [pnpm, overrides, trivy, cve, ci]
status: complete
requires:
  - web/pnpm-workspace.yaml overrides mechanism (commit f6647ec)
provides:
  - browserslist >= 4.28.7 and fast-uri 3.1.x pinned in web/pnpm-lock.yaml
  - trivy-fs CRITICAL,HIGH clean on the frontend lockfile (local proof)
affects:
  - .github/workflows/full-pipeline.yml trivy-fs -> build-scan -> release (unblocked once pushed)
tech-stack:
  added: []
  patterns:
    - "pnpm overrides live in web/pnpm-workspace.yaml, caret ranges to stay inside parent semver"
key-files:
  created:
    - .planning/todos/pending/2026-09-05-delete-stale-web-package-lock-json.md
    - .planning/todos/pending/2026-09-05-move-shadcn-out-of-frontend-dependencies.md
  modified:
    - web/pnpm-workspace.yaml
    - web/pnpm-lock.yaml
    - internal/webassets/build/client/ (SPA chunk hashes moved)
  moved:
    - .planning/todos/pending/ -> completed/ 2026-09-05-bump-frontend-deps-to-clear-trivy-fs-high-cves.md
decisions:
  - "Caret ranges (^4.28.7 / ^3.1.6), never >=: an unconstrained >= override crosses fast-uri's major boundary (resolves 4.1.4) and bypasses ajv@8.20.0's declared ^3.0.1"
  - "Regen pinned to corepack pnpm@11.8.0 (Dockerfile:29); plain corepack pnpm is 11.25.0 here and produces peer-suffix churn"
  - "make web output (internal/webassets/build/client/) committed in the same commit as the lockfile, per precedent f6647ec"
  - "make test -race substituted with plain go test -p 1 (documented cgo/ThreadSanitizer limitation on this machine, same as Phase 11.1-04 / Phase 15)"
metrics:
  duration: ~20 min
  completed: 2026-09-05
actuals:
  tokens: 6000
  tasks: 3
  commits: 2
---

# Quick Task 260905-fa4: Bump frontend transitive deps to clear trivy-fs HIGH CVEs Summary

pnpm `overrides` bump of `browserslist` and `fast-uri` past 6 HIGH CVEs, lockfile
regenerated with the Docker-pinned pnpm, embedded SPA refreshed, local Trivy proven clean.

## What was done

### Task 1 (tracer) — overrides + lockfile regen + Trivy proof

Two lines appended to the existing `overrides:` block in `web/pnpm-workspace.yaml`,
directly after `postcss`, CRLF-preserved:

```
  browserslist: '^4.28.7'
  fast-uri: '^3.1.6'
```

`web/pnpm-workspace.yaml` diff: **+2 / -0** (`git diff --numstat` = `2  0`), one
`overrides:` key, no whole-file rewrite.

Lockfile regenerated with `corepack pnpm@11.8.0 --dir web install --lockfile-only`.

**Resolved versions the lockfile landed on:**

| Package | Before | After |
|---------|--------|-------|
| `browserslist` | 4.28.2 | **4.28.9** |
| `fast-uri` | 3.1.5 | **3.1.7** |

No `fast-uri@4.x` entry anywhere in the lockfile; `fast-uri@3.x` count = 2;
`browserslist@4.x` count = 2; no `browserslist@4.28.2` entry survives.

**Lockfile diff size: 64 changed lines** (`git diff --numstat` sum), under the 120 gate,
zero `supports-color@7.2.0` peer-suffix churn. Confined to the browserslist family plus
the two target packages:
`baseline-browser-mapping` 2.10.32 -> 2.11.21, `caniuse-lite` 1.0.30001793 -> 1.0.30001810,
`electron-to-chromium` 1.5.362 -> 1.5.422, `node-releases` 2.0.46 -> 2.0.54,
`update-browserslist-db` 1.2.3 -> 1.3.2, plus the `ajv@8.20.0` and `shadcn@4.16.2`
dependency-line updates.

**Trivy 0.70.0 (`fs --scanners vuln --severity CRITICAL,HIGH --exit-code 1`) against the
regenerated lockfile: `Total: 0`, exit 0.** Baseline reproduced at plan time with the
identical command was `Total: 6 (HIGH: 6, CRITICAL: 0)`, exit 1.

Tracer feedback gate: `<verify>` re-run end-to-end, all automated gates pass — expanded to
Task 2.

### Task 2 — full-install / DoD gates / SPA refresh / commit

| Gate | Result |
|------|--------|
| `corepack pnpm@11.8.0 --dir web install --frozen-lockfile` | exit 0 (lockfile accepted, CI parity) |
| `corepack pnpm --dir web test` (vitest) | 12 files / 125 tests passed |
| `corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}"` | "All matched files use Prettier code style" |
| `go vet ./...` | clean |
| `golangci-lint run` | 0 issues |
| `make test` (after `make db-up`) | **substituted** — see below — all packages `ok`, exit 0 |
| `make coverage-gate` | Backend coverage 90.05% (required 80%) — PASS |
| `make sqlc-check` | no diff, exit 0 |

**`make web` changed `internal/webassets/build/client/`** — the browserslist/caniuse-lite
bump moved the content-hashed chunk filenames (8 assets replaced + `index.html` updated),
exactly as precedent commit `f6647ec` did. Staged into the **same** `fix(deps):` commit as
the lockfile.

**Backend DoD gate substitution:** `make test` runs `go test ./... -race`. `-race` is
unusable on this Windows dev machine (ThreadSanitizer allocation failure under memory
pressure — pre-existing documented limitation, Phase 11.1-04 / Phase 15). Substituted
`go test ./... -count=1 -p 1 -coverprofile=coverage.out -coverpkg=$(COVER_PKGS)` (no
`-race`; `-p 1` for the known flaky poller DB test, per Phase 15). All packages passed;
`coverage.out` fed `make coverage-gate` which reported 90.05%.

Commit `18c36db` — `fix(deps): bump browserslist and fast-uri past HIGH CVEs to unblock
trivy-fs` — carries `web/pnpm-workspace.yaml` + `web/pnpm-lock.yaml` +
`internal/webassets/build/client/` (19 files, +47/-43). No `--no-verify`; pre-commit hooks
ran (gitleaks passed, golangci-lint/prettier skipped — no matching files). No AI
attribution trailer.

### Task 3 — close source todo, file two follow-ups

- `git mv` `.planning/todos/pending/2026-09-05-bump-frontend-deps-to-clear-trivy-fs-high-cves.md`
  -> `completed/`, content unchanged (100% rename).
- New: `2026-09-05-delete-stale-web-package-lock-json.md` (`area: tooling`, severity minor)
  — second drifting npm lockfile, Trivy scans it repo-wide, contributed zero of the six
  findings but is unmaintained and a future finding source.
- New: `2026-09-05-move-shadcn-out-of-frontend-dependencies.md` (`area: tooling`, severity
  minor) — explicit warning that reclassifying `shadcn` to `devDependencies` makes findings
  disappear from the Trivy report via the pnpm-v9 dev-dep exclusion **while the vulnerable
  versions stay pinned**, and must **never substitute** for a version fix. Recorded as
  correctness cleanup only.

Commit `efd9ea0` — `docs: close the frontend-dep trivy-fs todo, file two follow-ups`
(3 files, +85). Separate from the fix commit. No `--no-verify`, no AI attribution.

## Deviations from Plan

None affecting outcome. One documented gate substitution (`make test` `-race` -> plain
`go test -p 1`), pre-authorised by the plan's `<verification>` step 3 and the task
constraints.

## Scope note — this closes the local reproduction only

Trivy going green here proves the fix on the **same scanner version CI runs** (0.70.0,
`aquasecurity/trivy-action@v0.36.0`). It does **not** confirm CI green. The
CI-green confirmation is still pending a **scratch-branch push** showing:

- the `trivy-fs` job actually going green, and
- `build-scan` (which `needs: trivy-fs`) actually running.

This is the same live-proof pattern Phase 16's G-16-1 and Phase 09's coverage-gate
verification used. The source todo's own verification checklist step 4 (scratch-branch
push) stays open until that push is observed — the todo is in `completed/` because the
fix is done and locally proven, not because CI has confirmed it.

Remaining frontend-dependency hygiene work is tracked by the two follow-up todos filed in
Task 3 (delete `web/package-lock.json`; move `shadcn` to `devDependencies`).

## Self-Check: PASSED

- `web/pnpm-workspace.yaml` — FOUND, +2/-0, one `overrides:` key, both caret lines present
- `web/pnpm-lock.yaml` — FOUND, browserslist@4.28.9 / fast-uri@3.1.7, no 4.x fast-uri
- `.planning/todos/completed/2026-09-05-bump-frontend-deps-to-clear-trivy-fs-high-cves.md` — FOUND
- `.planning/todos/pending/2026-09-05-delete-stale-web-package-lock-json.md` — FOUND
- `.planning/todos/pending/2026-09-05-move-shadcn-out-of-frontend-dependencies.md` — FOUND
- commit `18c36db` — FOUND in `git log`
- commit `efd9ea0` — FOUND in `git log`
- `git status --short web/ internal/webassets/` — clean
