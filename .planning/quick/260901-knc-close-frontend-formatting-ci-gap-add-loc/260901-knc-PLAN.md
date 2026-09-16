---
phase: quick-260901-knc
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .pre-commit-config.yaml
  - .claude/CLAUDE.md
autonomous: true
requirements: [QUICK-260901-KNC]

estimate:
  tokens: 45000
  raw_tokens: 25000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "A `prettier --write` pre-commit hook exists that auto-fixes web/ TS/TSX in the working tree, mirroring the golangci-lint `--fix` pattern already in the file."
    - "The hook runs prettier with cwd=web/, so its output is byte-identical to what CI's `frontend-test` job checks."
    - "`.claude/CLAUDE.md` opens with a Definition of Done checklist an agent reads before its first edit."
    - "The DoD section sits outside every GSD marker block, so `generate-claude-md` cannot overwrite it."
    - "The DoD forbids `git commit --no-verify` and requires prettier formatting before staging frontend changes."
  artifacts:
    - .pre-commit-config.yaml
    - .claude/CLAUDE.md
  key_links:
    - "hook entry -> cwd=web/ -> web/.prettierrc plugin resolution (breaks if cwd is repo root)"
    - "hook glob `**/*.{ts,tsx}` -> identical to full-pipeline.yml frontend-test `prettier --check` glob"
    - "DoD section position -> above `<!-- GSD:project-start -->` -> survives generate-claude-md"
---

<objective>
Close the frontend-formatting CI gap with two edits: a local `prettier --write`
pre-commit hook, and a Definition of Done checklist at the top of `.claude/CLAUDE.md`.

Purpose: CI's `frontend-test` job runs `prettier --check` and has already gone red on
hand-formatted TSX once (quick task 260901-jre, commit 30839fb, reformatted 6 Phase 14
files after the fact). There is currently no local gate that catches this before a
commit exists, and nothing tells an agent to format before staging.

Output: `.pre-commit-config.yaml` gains a third local gate; `.claude/CLAUDE.md` gains a
leading, hand-maintained DoD section.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.pre-commit-config.yaml
@.claude/CLAUDE.md
@.github/workflows/full-pipeline.yml
@Makefile
@web/package.json
@web/.prettierrc
</context>

<planning_observations>
Every claim below was verified live at planning time on this machine. Do not re-derive;
do not substitute a "more standard" command for a grounded one.

1. **Prettier MUST run with cwd=`web/`.** Running it from the repo root fails outright:
   `Cannot find package 'prettier-plugin-tailwindcss' imported from
   C:\CodeProjects\drop-tracker\noop.js`. Plugin resolution (and `.prettierrc`'s
   `tailwindStylesheet: "app/app.css"`) is cwd-relative. A hook that runs from the repo
   root would either crash or emit different output than CI — which checks from `web/`.

2. **Bare `pnpm` is NOT on this machine's PATH.** `command -v pnpm` -> not found.
   pnpm 11.23.0 is available only via `corepack pnpm` (corepack is at
   `/c/Program Files/nodejs/corepack`; `~/AppData/Local/node/corepack/lastKnownGood.json`
   pins pnpm 11.23.0). `web/package.json` has no `packageManager` field, so corepack
   resolves via lastKnownGood. CI is unaffected — `pnpm/action-setup` puts real pnpm v11
   on PATH there.

3. **`corepack pnpm --dir web exec` sets cwd to `web/`** — verified directly:
   `corepack pnpm --dir web exec node -e "console.log(process.cwd())"` printed
   `C:\CodeProjects\drop-tracker\web`. And
   `corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}"` run from the repo root
   produced output identical to CI's `cd web && pnpm exec prettier --check "**/*.{ts,tsx}"`.
   This is the grounded hook entry.

4. **The working tree currently fails `prettier --check` on 12 files — and it is a
   CRLF artifact, not real drift.** `core.autocrlf=true`, there is no `.gitattributes`
   anywhere in the repo, and `.prettierrc` sets `endOfLine: "lf"`. Verified on
   `web/app/lib/sources.ts`: the file has CRLF terminators, and prettier's output is
   byte-identical to it once CR is stripped (`diff --strip-trailing-cr` -> identical).
   Consequence: the first `--write` run rewrites those 12 files CRLF -> LF on disk, but
   because git normalizes on add, `git diff` stays clean. That is exactly why Task 1's
   verify asserts `git diff --exit-code` rather than "no files changed on disk".
   Do NOT add `.gitattributes` — out of scope for this task.

5. **The pre-commit framework is not currently installed in this clone.**
   `.git/hooks/pre-commit` does not exist (only `.sample` files), and neither
   `pre-commit` nor `python` resolves on PATH in this shell (Python 3.12 is installed at
   `~/AppData/Local/Programs/Python/Python312` but is not PATH-registered). So
   `pre-commit run prettier-web --all-files` CANNOT be executed as a verification step
   here. Verify the hook's *entry command* directly instead (Task 1's verify does this),
   and record the `pre-commit` invocation in the SUMMARY as a deferred, environment-blocked
   check. Do not fake it, and do not install Python/pre-commit as part of this task.

6. **`make sqlc-check` has no counterpart job in `full-pipeline.yml`.** CI's jobs are
   vet, lint, test (`make test-integration` + `make coverage-gate`), gitleaks, trivy-fs,
   frontend-test, pr-title, build-scan, release. The DoD text must therefore not claim
   sqlc-check "mirrors CI" — label it local-only, as Task 2 specifies.
</planning_observations>

<tasks>

<task type="tracer">
  <name>Task 1: Add the local prettier --write pre-commit hook</name>
  <files>.pre-commit-config.yaml</files>
  <precondition>`corepack` resolves on PATH and `web/node_modules/prettier` is installed (both verified at planning time). If `corepack pnpm --dir web exec prettier --version` fails, halt — the hook entry cannot be validated.</precondition>
  <action>
Append a third hook to `.pre-commit-config.yaml` as a `repo: local` entry, placed after
the existing golangci-lint block so the file reads gitleaks -> golangci-lint -> prettier.

Hook fields: id `prettier-web`; name `prettier (web)`; `language: system`;
`pass_filenames: false`; `files: ^web/.*\.(ts|tsx)$`; and entry
`corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"`.

Four constraints on that entry, each grounded in a planning-time observation — do not
"simplify" any of them:
- `--write`, never `--check`. This hook auto-fixes the working tree exactly like the
  golangci-lint `--fix` hook above it. CI keeps `--check` as the non-bypassable backstop.
- `corepack pnpm`, not bare `pnpm` — bare `pnpm` is not on this machine's PATH
  (observation 2).
- `--dir web` is load-bearing: it sets cwd to `web/`, without which prettier cannot
  resolve `prettier-plugin-tailwindcss` at all (observations 1 and 3).
- `pass_filenames: false` with the literal glob `**/*.{ts,tsx}`. pre-commit passes
  repo-root-relative paths, which would be wrong under cwd=`web/`; the glob is instead
  the exact string CI's frontend-test job and `web/package.json`'s `format` script use,
  so all three format the same file set. `files:` still scopes the hook, deciding whether
  it runs at all based on the staged paths.

Update the file's existing header comment, matching its established style (prose
sentences explaining why, referencing the CI job by name). It currently opens with "Two
local gates before a commit exists" — that count is now wrong; make it three and add a
prettier clause naming the `frontend-test` job's format check as the CI backstop this
mirrors. Add a short inline comment on the new hook itself, in the same register as the
golangci-lint comment above it, covering: why `--write` and not `--check` (parity with
the `--fix` hook — changes land in the working tree to be seen and re-staged, never
silently committed), and why `--dir web` cannot be dropped (plugin resolution is
cwd-relative). Do not restate the CRLF finding in the config file.
  </action>
  <verify>
    <automated>corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}" && git diff --exit-code -- web/ && grep -v '^[[:space:]]*#' .pre-commit-config.yaml | grep -c 'id: prettier-web'</automated>
  </verify>
  <done>
The literal hook entry command runs successfully from the repo root and leaves
`git diff -- web/` clean (proving the only on-disk rewrites were CRLF -> LF, which git
normalizes away — observation 4). The grep gate, which strips comment lines first,
reports exactly 1 real `id: prettier-web` declaration. The header comment says three
gates, not two.
  </done>
</task>

<task type="auto">
  <name>Task 2: Add the Definition of Done section to the top of CLAUDE.md</name>
  <files>.claude/CLAUDE.md</files>
  <action>
Insert a new section at the very top of `.claude/CLAUDE.md`, ABOVE the existing line 1
`<!-- GSD:project-start source:PROJECT.md -->` marker. Position is the whole point: an
agent reads this file top-down and must hit the gate before the 140-line stack table, not
after it. Placing it outside all GSD marker blocks is what keeps `generate-claude-md`
from rewriting it — that tool does marker-bounded replacement of its 6 sections, so
content before the first marker is as safe as the hand-maintained "Agent skills" section
already sitting after the last one.

Lead with an HTML comment guard mirroring the footer note's convention — the footer says
the trailing section sits outside the GSD marker blocks so generate-claude-md never
overwrites it and to keep it last; write the top counterpart saying it sits above every
marker block for the same reason and to keep it first. Leave the existing footer note
unchanged.

Heading: `## Definition of Done — run before every commit`.

Content, short and imperative, in this order:

First, a one-line frame: every commit must clear the same gates CI enforces, run locally
first rather than outsourced to CI.

Second, a **format-frontend-first** instruction, before the numbered list: if anything
under `web/` changed, run `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"`
before staging. State the reason plainly — prettier output and
`prettier-plugin-tailwindcss` class ordering cannot be produced by hand, so
hand-formatted TSX fails CI's `frontend-test` job. This is the failure this whole task
exists to prevent; do not soften it to a suggestion.

Third, the numbered gate list, using only targets and commands verified to exist in the
Makefile and full-pipeline.yml: `go vet ./...`; `golangci-lint run`; `make test`
(integration suite, needs `make db-up` first); `make coverage-gate` (80% backend floor);
`make sqlc-check` — and label this one local-only with no CI counterpart, because it has
none (observation 6), so the list stays truthful about what CI will and will not catch;
and finally, for web changes only,
`cd web && corepack pnpm exec prettier --check "**/*.{ts,tsx}"` plus `corepack pnpm test`.

Add one short parenthetical after the list: where bare `pnpm` is on PATH, drop the
`corepack` prefix. (On this machine it is not — observation 2.)

Fourth, the bypass prohibition: never bypass the hooks with the `--no-verify` flag on a
commit. Give the reason in one sentence — the hooks (gitleaks, golangci-lint `--fix`,
prettier `--write`) are the fast local mirror of CI, so skipping them only relocates the
failure somewhere slower and more public. Close by pointing at `make hooks` to install
them, since they are not currently installed in this clone (observation 5).

Use inline backticks for commands. Keep the whole section under roughly 25 lines — it is
a checklist, not an essay, and it competes for attention with everything below it.
  </action>
  <verify>
    <automated>awk '/^## Definition of Done/{d=NR} /GSD:project-start/{m=NR} END{exit !(d>0 && m>0 && d<m)}' .claude/CLAUDE.md && grep -c 'make coverage-gate\|make sqlc-check\|prettier --write' .claude/CLAUDE.md</automated>
  </verify>
  <done>
The awk gate passes, proving the `## Definition of Done` heading exists and appears
strictly before the `GSD:project-start` marker (i.e. outside every GSD-managed block).
The grep reports a non-zero count for the referenced Make targets and the prettier write
command. The file still renders as valid markdown and the trailing hand-maintained
"Agent skills" section is untouched.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| pre-commit hook -> local dev machine | The hook executes `node_modules` code (prettier + its tailwind plugin) automatically at every commit touching `web/`. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-KNC-01 | Tampering | `prettier-plugin-tailwindcss` / prettier executed by the hook | low | accept | No new dependency is added — prettier and the plugin are already `web/` devDependencies pinned by `pnpm-lock.yaml`, already executed by CI's frontend-test job and by `pnpm run format`. The hook changes when existing trusted code runs, not what runs. |
| T-KNC-02 | Tampering | Working-tree rewrite by `--write` | low | mitigate | Mirrors the existing golangci-lint `--fix` contract: pre-commit fails the commit when a hook modifies files, so every auto-applied change surfaces for the developer to review and re-stage rather than being silently committed. |
| T-KNC-03 | Repudiation | `--no-verify` bypass | low | mitigate | Out of reach of config alone — CI's `frontend-test` job remains the non-bypassable backstop; Task 2's DoD adds the explicit written prohibition. |
| T-KNC-SC | Tampering | npm/pip/cargo installs | n/a | accept | No package-manager install occurs in this task. No Package Legitimacy Audit required. |
</threat_model>

<verification>
1. `corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}"` behaves identically
   whether invoked from the repo root (via `--dir`) or as `cd web && ... exec` — the two
   must agree, or the hook and CI have diverged.
2. `git status --short` shows only `.pre-commit-config.yaml` and `.claude/CLAUDE.md` as
   modified (plus the pre-existing untracked `.gsd/`, `.planning/agent-history.json`,
   `.planning/state.json`).
3. No Go, TS, TSX, Makefile, or workflow file is modified.
</verification>

<success_criteria>
- `.pre-commit-config.yaml` declares exactly three gates, the third being `prettier-web`
  with a `--write` entry scoped to `^web/.*\.(ts|tsx)$`.
- The hook's entry command executes successfully and leaves `git diff -- web/` clean.
- `.claude/CLAUDE.md` begins with the DoD section, above the first GSD marker.
- The DoD names only Make targets and commands that actually exist, flags `make sqlc-check`
  as having no CI counterpart, and prohibits the `--no-verify` bypass.
- No Makefile change, no `make verify` target, no `.gitattributes`, no new dependency.
</success_criteria>

<deferred>
Recorded here so they are not silently lost, and deliberately NOT actioned in this task:

- **`pre-commit run prettier-web --all-files` was never executed** — the framework is not
  installed in this clone and `python`/`pre-commit` are off PATH (observation 5). Run
  `make hooks` (may need `make hooks PYTHON=<path to Python312 python.exe>`) and then the
  hook, to close this.
- **No `.gitattributes` + `core.autocrlf=true` + `endOfLine: "lf"`** makes 12 web files
  perpetually "unformatted" locally while passing in CI (observation 4). A
  `* text=auto eol=lf` .gitattributes would end this class of noise. Out of scope; worth
  a follow-up quick task.
</deferred>

<output>
Create `.planning/quick/260901-knc-close-frontend-formatting-ci-gap-add-loc/260901-knc-SUMMARY.md` when done.
</output>
