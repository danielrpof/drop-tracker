---
phase: quick-260901-knc
plan: 01
subsystem: infra
tags: [pre-commit, prettier, ci, tooling, claude-md]

requires: []
provides:
  - "Local prettier --write pre-commit hook scoped to web/ TS/TSX, mirroring CI's frontend-test prettier --check"
  - "Definition of Done checklist at the top of .claude/CLAUDE.md, above every GSD marker block"
affects: [frontend, ci, "quick tasks touching web/"]

actuals:
  tokens: 900
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "repo:local pre-commit hook running a package-manager-scoped formatter with pass_filenames: false + literal glob"
    - "Hand-maintained CLAUDE.md section placed before the first GSD marker so generate-claude-md cannot overwrite it"

key-files:
  created: []
  modified:
    - .pre-commit-config.yaml
    - .claude/CLAUDE.md

key-decisions:
  - "Hook entry uses `corepack pnpm --dir web exec` (not bare pnpm) because bare pnpm is not on this machine's PATH; --dir web is load-bearing for prettier-plugin-tailwindcss resolution"
  - "--write (not --check) in the hook, matching the golangci-lint --fix contract; CI keeps --check as the non-bypassable backstop"
  - "DoD section flags `make sqlc-check` as local-only with no CI counterpart, keeping the checklist truthful about what CI catches"

patterns-established:
  - "Frontend formatting is gated locally before a commit exists, not only in CI"

requirements-completed: [QUICK-260901-KNC]

coverage:
  - id: D1
    description: "A prettier --write pre-commit hook auto-fixes web/ TS/TSX, running with cwd=web/ so its output matches CI's frontend-test prettier --check"
    requirement: "QUICK-260901-KNC"
    verification:
      - kind: other
        ref: "corepack pnpm --dir web exec prettier --write \"**/*.{ts,tsx}\" && git diff --exit-code -- web/  (clean); grep -c 'id: prettier-web' (comment-stripped) == 1"
        status: pass
      - kind: other
        ref: "prettier --check parity: `corepack pnpm --dir web exec prettier --check` (exit 0) vs `cd web && corepack pnpm exec prettier --check` (exit 0)"
        status: pass
    human_judgment: false
  - id: D2
    description: ".claude/CLAUDE.md opens with a Definition of Done checklist above every GSD marker block, forbidding git commit --no-verify and requiring prettier formatting before staging frontend changes"
    requirement: "QUICK-260901-KNC"
    verification:
      - kind: other
        ref: "awk gate: '## Definition of Done' heading present and strictly before 'GSD:project-start' marker — pass; grep -c for 'make coverage-gate|make sqlc-check|prettier --write' == 3"
        status: pass
    human_judgment: false

duration: 2min
completed: 2026-09-01
status: complete
---

# Quick Task 260901-knc: Close Frontend-Formatting CI Gap Summary

**A local `prettier --write` pre-commit hook (cwd=web/ via corepack pnpm) plus a Definition of Done checklist at the top of `.claude/CLAUDE.md`, so hand-formatted TSX is caught before a commit exists rather than going red in CI's frontend-test job.**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-09-01T20:00:01Z
- **Completed:** 2026-09-01T20:01:53Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- `.pre-commit-config.yaml` now declares three local gates: gitleaks -> golangci-lint -> prettier-web. The new `repo: local` hook runs `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"`, scoped by `files: ^web/.*\.(ts|tsx)$` with `pass_filenames: false`.
- The file's header comment was updated from "Two local gates" to three, with a prettier clause naming the `frontend-test` job as the CI backstop it mirrors. An inline comment on the hook explains `--write` (parity with golangci-lint `--fix`) and why `--dir web` cannot be dropped (cwd-relative plugin resolution).
- `.claude/CLAUDE.md` gains a leading `## Definition of Done — run before every commit` section, positioned above the `<!-- GSD:project-start -->` marker (and guarded by an HTML comment mirroring the footer note's convention) so `generate-claude-md` cannot overwrite it.
- The DoD leads with a format-frontend-first instruction, then a numbered gate list (`go vet ./...`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check` flagged local-only, and web-only prettier `--check` + `pnpm test`), a corepack parenthetical, and a `--no-verify` prohibition pointing at `make hooks`.

## Task Commits

1. **Task 1: Add the local prettier --write pre-commit hook** - `32d64f3` (ci)
2. **Task 2: Add the Definition of Done section to the top of CLAUDE.md** - `bf041b2` (docs)

## Files Created/Modified

- `.pre-commit-config.yaml` - Third `repo: local` hook `prettier-web`; header + inline comments updated
- `.claude/CLAUDE.md` - New leading Definition of Done checklist above the first GSD marker

## Decisions Made

- Followed the plan's `<planning_observations>` verbatim: `corepack pnpm --dir web exec` rather than bare `pnpm`, `--write` not `--check`, `--dir web` retained, literal `**/*.{ts,tsx}` glob with `pass_filenames: false`.
- `make sqlc-check` labelled local-only in the DoD (observation 6 — it has no `full-pipeline.yml` counterpart).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Running `prettier --write` rewrote 12 `web/` files CRLF -> LF on disk (observation 4: `core.autocrlf=true`, no `.gitattributes`, `.prettierrc` `endOfLine: "lf"`). `git diff --exit-code -- web/` stayed clean because git normalizes on read, but `git status` flagged them stat-dirty. Restored with a targeted `git checkout -- web/` so only the two intended files (`.pre-commit-config.yaml`, `.claude/CLAUDE.md`) were committed. No `.gitattributes` added — explicitly out of scope.

## Deferred (from PLAN, not actioned here)

- **`pre-commit run prettier-web --all-files` was never executed** — the pre-commit framework is not installed in this clone and `python`/`pre-commit` are off PATH (observation 5). The hook's *entry command* was verified directly instead. Close by running `make hooks` (may need `make hooks PYTHON=<path to Python312 python.exe>`) then the hook.
- **No `.gitattributes` + `core.autocrlf=true` + `endOfLine: "lf"`** leaves 12 web files perpetually "unformatted" locally while passing in CI. A `* text=auto eol=lf` `.gitattributes` would end this class of noise — worth a follow-up quick task.

## Self-Check

- `.pre-commit-config.yaml` modified and committed in `32d64f3` — FOUND
- `.claude/CLAUDE.md` modified and committed in `bf041b2` — FOUND
- Commit `32d64f3` in `git log` — FOUND
- Commit `bf041b2` in `git log` — FOUND
- Task 1 verify (`prettier --write` + `git diff --exit-code -- web/` + `grep -c 'id: prettier-web'` == 1) — PASS
- Task 2 verify (awk heading-before-marker gate + `grep -c` == 3) — PASS
- Overall verification: `prettier --check` parity between `--dir web` and `cd web` invocations (both exit 0) — PASS
- No Go, TS, TSX, Makefile, or workflow file modified — PASS

## Self-Check: PASSED

## Next Phase Readiness

- No blockers. The hook takes effect for any developer who has run `make hooks`; CI's `frontend-test` `prettier --check` step remains the non-bypassable backstop regardless.

---
*Phase: quick-260901-knc*
*Completed: 2026-09-01*
