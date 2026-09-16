---
phase: quick-260901-lvn
plan: 01
subsystem: infra
tags: [git, gitattributes, line-endings, prettier, pre-commit, go-embed]

requires:
  - phase: quick-260901-knc
    provides: local prettier --write pre-commit hook that exposed the perpetual stat-dirty loop
provides:
  - repo-root .gitattributes pinning every text file to LF in repo and working tree
  - fully LF working tree on this core.autocrlf=true Windows machine
  - explicit binary attributes for tracked woff2/ico assets under go:embed
affects: [pre-commit, ci, frontend-formatting, webassets]

actuals:
  tokens: 12000
  tasks: 2
  commits: 1

tech-stack:
  added: []
  patterns:
    - "Line-ending policy lives in repo-root .gitattributes (* text=auto eol=lf), overriding per-machine core.autocrlf"

key-files:
  created:
    - .gitattributes
  modified: []

key-decisions:
  - "git add --renormalize . staged nothing beyond .gitattributes — the index was already 100% LF (561 i/lf, 0 i/crlf), so there was no line-ending normalization commit to make"
  - "The load-bearing step was the forced re-checkout (git rm --cached -r . then git reset --hard), not the renormalize"
  - "core.autocrlf left at true — .gitattributes overrides it, no per-machine config touched"

patterns-established:
  - "Refreshing the working tree after an eol-normalization change: git rm --cached -r . then git reset --hard (never git clean)"

requirements-completed: [QUICK-260901-LVN]

coverage:
  - id: D1
    description: "Repo-root .gitattributes pins the tree to LF and overrides core.autocrlf=true"
    requirement: "QUICK-260901-LVN"
    verification:
      - kind: automated
        ref: "git check-attr eol -- web/app/root.tsx / Makefile / internal/webassets/embed.go => lf; git check-attr text -- *.ico / *.woff2 => unset"
        status: pass
    human_judgment: false
  - id: D2
    description: "Working tree refreshed to LF — 455 w/crlf entries now 0, tree no longer goes stat-dirty under prettier"
    requirement: "QUICK-260901-LVN"
    verification:
      - kind: automated
        ref: "git ls-files --eol | grep -c 'w/crlf' => 0; prettier --check web/**/*.{ts,tsx} passes; git status --porcelain --untracked-files=no empty before+after"
        status: pass
    human_judgment: false
  - id: D3
    description: "Newly-LF go:embed asset bytes still build and pass tests"
    requirement: "QUICK-260901-LVN"
    verification:
      - kind: automated
        ref: "go build ./... ; go test ./internal/webassets/..."
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-09-01
status: complete
---

# Quick Task 260901-lvn: Repo-root .gitattributes + working-tree LF renormalization Summary

**Added a repo-root `.gitattributes` (`* text=auto eol=lf` + explicit `binary` rules for woff2/ico) and forced a working-tree re-checkout, taking this `core.autocrlf=true` Windows machine from 455 CRLF working-tree files to 0 and ending the perpetual `prettier-web` stat-dirty loop.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-09-01T20:48:00Z
- **Completed:** 2026-09-01T20:53:00Z
- **Tasks:** 2
- **Files modified:** 1 committed (`.gitattributes`); 455 files re-checked-out to LF on disk (no committed content change)

## Accomplishments

- Created `.gitattributes` at the repo root with a comment header naming `core.autocrlf=true`, `web/.prettierrc`'s `endOfLine: "lf"`, and the `prettier-web` pre-commit hook as the three disagreeing parties this file settles. Four rule groups: baseline `* text=auto eol=lf`; LF-mandatory `*.sh`/`Makefile`/`Dockerfile`; explicit `*.go`/`*.ts`/`*.tsx eol=lf`; `binary` for `*.woff2 *.ico *.png *.jpg *.jpeg *.gif *.webp *.pdf`.
- Confirmed `git add --renormalize .` staged **nothing beyond `.gitattributes`** — the index was already fully LF, exactly as the plan anticipated. No line-endings-only proof was required because no extra path was staged.
- Forced the working-tree refresh with git's documented recipe (`git rm --cached -r .` then `git reset --hard`). `w/crlf` count went from **455 to 0**.
- Verified the fix holds under the tool that exposed it: `prettier --check "**/*.{ts,tsx}"` passes and `git status --porcelain --untracked-files=no` is empty both before and after the run — the tree no longer goes stat-dirty.
- Verified the re-checked-out `go:embed` asset tree: `go build ./...` and `go test ./internal/webassets/...` both pass. Local embedded bytes now match what CI and Docker have always built from an LF checkout.
- `core.autocrlf` left at `true` — untouched.

## Task Commits

1. **Task 1: Write .gitattributes, renormalize the index, and commit** — `1524333` (chore)
2. **Task 2: Force the working-tree refresh to LF and verify** — no commit (rewrites line endings on disk only, no committed file change)

_Plan metadata commit (SUMMARY.md, STATE.md) handled by the orchestrator._

## Files Created/Modified

- `.gitattributes` (created) — repo-root line-ending policy: `* text=auto eol=lf`, LF-mandatory for shell/Makefile/Dockerfile, explicit source-language rules, `binary` for image/font assets.

## Requested SUMMARY facts (from live observation)

- **`git add --renormalize .` staged:** only `.gitattributes` — nothing else. The index was already 100% LF, so there was no normalization content to commit and nothing to prove line-endings-only.
- **`w/crlf` count:** 455 before the working-tree refresh, 0 after.
- **`core.autocrlf`:** was `true` before, still `true` after — the per-machine setting was not touched.

## Decisions Made

- Committed `.gitattributes` alone; no separate normalization commit exists because the index carried no CRLF blobs. This matches the plan's "Established at plan time" note.
- Used `git rm --cached -r .` + `git reset --hard` as the forced re-checkout. `git clean` was never run. Untracked files (`.gsd/`, `.planning/agent-history.json`, `.planning/state.json`, this task's own directory) survived untouched.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `git ls-files --eol | grep -c 'w/crlf'` returns `0`, which makes `grep` exit non-zero and short-circuited one `&&` command chain. Cosmetic only — re-ran the individual checks separately; all pass. No impact on outcome.

## Self-Check: PASSED

- `.gitattributes` — FOUND (committed at HEAD `1524333`, `git cat-file -e HEAD:.gitattributes` succeeds)
- Commit `1524333` — FOUND in `git log`
- `git check-attr` gate (eol=lf for tsx/Makefile/go, text=unset for ico/woff2) — PASSED (`ATTRS_OK`)
- `git ls-files --eol | grep -c 'w/crlf'` — 0
- `prettier --check` — PASSED, tree clean before and after
- `go build ./...` — PASSED
- `go test ./internal/webassets/...` — PASSED
- `core.autocrlf` — still `true`

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The perpetual stat-dirty loop from quick task 260901-knc's prettier hook is resolved. No blockers.
- The fix is portable — it travels with the repo for every future clone and contributor with no per-machine git config change.

---
*Phase: quick-260901-lvn*
*Completed: 2026-09-01*
