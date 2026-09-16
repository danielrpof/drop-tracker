---
phase: quick-260901-lvn
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .gitattributes
autonomous: true
requirements: [QUICK-260901-LVN]

estimate:
  tokens: 30000
  raw_tokens: 30000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "A repo-root `.gitattributes` pins every git-detected text file to LF in the repository and in the working tree, overriding this machine's `core.autocrlf=true` without anyone editing that per-machine setting."
    - "`git ls-files --eol` reports zero worktree-CRLF entries after the refresh (455 before)."
    - "Running the `prettier-web` hook (or `prettier --check`) leaves the working tree clean — no CRLF-to-LF rewrite, no stat-dirty files, `git status --porcelain --untracked-files=no` stays empty across the run."
    - "Tracked binary assets (`.woff2`, `.ico` under `internal/webassets/build/client/` and `web/public/`) carry an explicit `binary` attribute, so `text=auto` content detection can never convert them."
    - "The Go build still succeeds and `internal/webassets` still passes with the newly-LF `go:embed` asset bytes — the local embed now matches what CI and Docker have always embedded from an LF checkout."
    - "Any content staged by `git add --renormalize .` is proven line-endings-only before it is committed; a real content change halts the task."
  artifacts:
    - .gitattributes
  key_links:
    - "`.gitattributes` `* text=auto eol=lf` -> `web/.prettierrc` `endOfLine: \"lf\"` -> `.pre-commit-config.yaml` `prettier-web` hook (the three must agree or the tree is perpetually stat-dirty)"
    - "`.gitattributes` `binary` rules -> tracked `.woff2`/`.ico` assets under `internal/webassets/build/client/` (breaks the embedded font/icon bytes if `text=auto` misfires)"
    - "LF working tree -> `internal/webassets/embed.go` `go:embed all:build/client` -> served asset bytes (local build now byte-matches CI/Docker)"
    - "`.gitattributes` committed to HEAD -> `git reset --hard` in Task 2 (an uncommitted `.gitattributes` would survive on disk but silently fall out of the index)"
---

<objective>
Add a repo-root `.gitattributes` that pins the whole tree to LF, then force a
working-tree refresh so the 455 files currently checked out with CRLF land on disk
as LF.

Purpose: `core.autocrlf=true` on this Windows machine checks LF blobs out as CRLF,
while `web/.prettierrc` pins `endOfLine: "lf"`. The `prettier-web` pre-commit hook
added in quick task 260901-knc runs `prettier --write` on every commit, so it
rewrites the 12 CRLF `web/**/*.{ts,tsx}` files back to LF on every single run —
leaving the working tree perpetually stat-dirty even though `git diff` stays clean.
`.gitattributes` is the portable, per-repo, per-clone fix and overrides
`core.autocrlf` on every machine, so no one has to change a global git setting.

Output: one new `.gitattributes` at the repo root, committed, plus a fully
LF working tree.

**Established at plan time (verified, do not re-derive):**
- No `.gitattributes` exists anywhere in the repo; `core.autocrlf` is `true`.
- `git ls-files --eol` today: **561 `i/lf`, 0 `i/crlf`** in the index vs. **455 `w/crlf`,
  106 `w/lf`** in the working tree. **The index is already fully normalized.**
- Consequence: `git add --renormalize .` is expected to stage **nothing**. There is
  most likely no "line-ending normalization commit" to make — the only committed
  content change is `.gitattributes` itself. Task 1 handles both outcomes and
  authorizes the file set from the live observation, never from this note.
- Because the clean filter converts CRLF-to-LF on the way into the index, `git status`
  stays *clean* after `.gitattributes` lands. Adding the file **does not** rewrite
  anything already on disk. Task 2's explicit re-checkout is the load-bearing step.
- 9 files are `-text` (woff2, ico) and 8 are `none` (minified single-line JS); both
  classes are already correctly detected and are unaffected.
- No `testdata/`, golden, or `.snap` fixtures exist, so no byte-exact test can break.
  `internal/webassets/embed_test.go` asserts status codes and Content-Type only.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.pre-commit-config.yaml
@web/.prettierrc
</context>

<tasks>

<task type="tracer">
  <name>Task 1: Write .gitattributes, renormalize the index, and commit</name>
  <precondition>`git status --porcelain --untracked-files=no` prints nothing — there are no uncommitted modifications to tracked files. If it prints anything, HALT and report: `git add --renormalize .` would sweep unrelated work into this commit, and Task 2's `git reset --hard` would discard it.</precondition>
  <files>.gitattributes</files>
  <action>
Create `.gitattributes` at the repo root with a short comment header explaining why
it exists (name `core.autocrlf=true`, `web/.prettierrc`'s `endOfLine: "lf"`, and the
`prettier-web` hook in `.pre-commit-config.yaml` as the three parties whose
disagreement this file settles), followed by four rule groups in this order:

1. Baseline: `* text=auto eol=lf` — normalize every git-detected text file to LF in
   the repository AND check it out as LF in the working tree. This is the rule that
   overrides `core.autocrlf`.
2. LF-mandatory-regardless: `*.sh`, `Makefile`, `Dockerfile`, each `text eol=lf`. No
   `.sh` is tracked today, but the hook infrastructure and Makefile recipes are POSIX
   and a carriage return on a shebang line is a silent bad-interpreter failure.
   `Makefile` and `Dockerfile` are the repo's only extensionless tracked text files
   besides `LICENSE`, so name them explicitly rather than leaning on detection.
3. Explicit source-language rules, redundant with `text=auto` but stated so the
   guarantee does not depend on content sniffing: `*.go text eol=lf` (gofmt writes LF
   and golangci-lint reads it), `*.ts text eol=lf`, `*.tsx text eol=lf` (prettier owns
   these and pins LF).
4. Binaries: `*.woff2`, `*.ico`, `*.png`, `*.jpg`, `*.jpeg`, `*.gif`, `*.webp`, `*.pdf`
   each marked `binary`. Only woff2 and ico are tracked now (under
   `internal/webassets/build/client/` and `web/public/`); the rest are listed
   defensively so a future asset lands correctly on its first commit. `binary` is
   git's macro for `-text -diff`, which makes `text=auto` structurally unable to
   misfire on them.

Keep the file readable — comments above each group, not per line.

Then stage and renormalize:

```
git add .gitattributes
git add --renormalize .
git diff --cached --name-only
```

**MUTABLE SCOPE — authorize the file set from this live output only, never from the
plan's expectation.** Two branches:

- **If `--name-only` lists only `.gitattributes`** (the expected outcome, since the
  index is already all-LF): there is nothing to prove line-endings-only. Say so
  explicitly in the SUMMARY and commit.
- **If it lists additional paths**: prove the extra content is line-endings-only
  before committing. Run `git diff --cached -w --numstat | awk '$1 != 0 || $2 != 0'`
  — it must print nothing (whitespace-insensitive diff is empty for every path). Also
  eyeball `git diff --cached --stat` for a sanity check that the touched set is
  plausible. If any path shows a non-zero whitespace-insensitive change, **HALT** and
  report it — that is a real content edit, not a line-ending normalization, and it is
  out of scope for this task.

Commit whatever is staged. Do not use `--no-verify`; the pre-commit hooks must run.
The `prettier-web` hook is scoped by `files: ^web/.*\.(ts|tsx)$` and will not fire on
a `.gitattributes`-only commit; gitleaks and golangci-lint will, and must pass.
  </action>
  <verify>
    <automated>test "$(git check-attr eol -- web/app/root.tsx | sed 's/.*: //')" = lf && test "$(git check-attr eol -- Makefile | sed 's/.*: //')" = lf && test "$(git check-attr eol -- internal/webassets/embed.go | sed 's/.*: //')" = lf && test "$(git check-attr text -- web/public/favicon.ico | sed 's/.*: //')" = unset && test "$(git check-attr text -- internal/webassets/build/client/assets/inter-latin-wght-normal-Dx4kXJAl.woff2 | sed 's/.*: //')" = unset && git cat-file -e HEAD:.gitattributes && echo ATTRS_OK</automated>
  </verify>
  <done>`.gitattributes` exists at the repo root and is committed to HEAD. `git check-attr` resolves `eol: lf` for a `.tsx`, the `Makefile`, and a `.go` file, and resolves `text: unset` for both a tracked `.ico` and a tracked `.woff2`. Any paths beyond `.gitattributes` in the staged set were proven line-endings-only by an empty whitespace-insensitive diff before being committed.</done>
</task>

<!-- planner-discipline-allow: w/crlf -->
<!-- Rationale: the `grep -c 'w/crlf' == 0` gate below matches against the output of
     `git ls-files --eol`, not against any file this plan authors. `w/crlf` is git's
     own column value for a CRLF working-tree entry and cannot be rephrased without
     making the command wrong. No plan-authored file contains the literal, so the
     gate is not self-invalidating. -->
<task type="auto">
  <name>Task 2: Force the working-tree refresh to LF and verify the tree stops going dirty</name>
  <precondition>`.gitattributes` is committed to HEAD (Task 1 done) AND `git status --porcelain --untracked-files=no` prints nothing. Both are required: `git reset --hard` restores the index and working tree from HEAD, so it silently discards any uncommitted tracked modification, and an uncommitted `.gitattributes` would drop out of the index. If tracked modifications exist, HALT — do not stash-and-hope, report and let the operator decide.</precondition>
  <files>(no committed file changes — this task rewrites line endings on disk only)</files>
  <action>
Committing `.gitattributes` does **not** rewrite files already on disk. Because the
clean filter converts CRLF-to-LF on the way into the index, git sees the 455 CRLF
files as unmodified and `git checkout -- .` is a no-op. The tree must be forcibly
re-checked-out through the new `eol=lf` smudge filter.

Record the before-count for the SUMMARY:

```
git ls-files --eol | grep -c 'w/crlf'
```

Then run git's own documented refresh recipe (gitattributes(5), "Refreshing the
working tree after you change end-of-line normalization"), as two commands back to
back:

```
git rm --cached -r .
git reset --hard
```

Notes the executor must hold:
- `git rm --cached -r .` empties the index but touches no file on disk. Between the
  two commands the repo looks alarming (everything staged for deletion) — that is
  expected and transient. `git reset --hard` restores the index from HEAD and, because
  no index entry survives to short-circuit it, writes every file back out through the
  new smudge filter. If anything interrupts you between the two commands, re-running
  `git reset --hard` is the recovery.
- `git reset --hard` does **not** remove untracked files. `.gsd/`,
  `.planning/state.json`, `.planning/agent-history.json`, and this quick task's own
  directory all survive untouched.
- Do **not** run `git clean` at any point in this task.
- Do **not** touch `core.autocrlf`. It stays `true`; `.gitattributes` overrides it.

Then confirm the fix holds under the tool that exposed the problem. Run prettier in
`--check` mode (not the hook's `--write`) from the repo root, and re-check status
immediately after so a silent rewrite cannot hide:

```
corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}"
git status --porcelain --untracked-files=no
```

`--check` must report all files formatted, and status must still print nothing. If
you prefer the hook path, `pre-commit run prettier-web --all-files` is equivalent
(pre-commit is installed; its shim already carries the Python 3.12 interpreter path)
— but a green `prettier-web` run only proves the point if `git status
--untracked-files=no` is still empty afterwards, since that hook writes.

Finally, prove the newly-LF `go:embed` asset bytes did not break the build. The
embedded SPA tree under `internal/webassets/build/client/` was previously embedded
from a CRLF checkout on this machine and from an LF checkout in CI/Docker; this
change makes local match CI:

```
go build ./...
go test ./internal/webassets/...
```
  </action>
  <verify>
    <automated>[ "$(git ls-files --eol | grep -c 'w/crlf')" = 0 ] && [ -z "$(git status --porcelain --untracked-files=no)" ] && corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}" && [ -z "$(git status --porcelain --untracked-files=no)" ] && go build ./... && go test ./internal/webassets/...</automated>
  </verify>
  <done>`git ls-files --eol` reports 0 worktree-CRLF entries (down from 455). `prettier --check` passes across `web/**/*.{ts,tsx}` and `git status --porcelain --untracked-files=no` is empty both before and after the prettier run — the tree no longer goes stat-dirty. `go build ./...` and `go test ./internal/webassets/...` both pass against the re-checked-out embedded assets. `core.autocrlf` is still `true` (unchanged).</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| working tree -> git index | `text=auto` content detection decides whether a blob is line-ending-converted; a misclassification corrupts the stored bytes |
| HEAD -> working tree (`git reset --hard`) | a destructive checkout that overwrites on-disk state from committed state |
| working tree -> `go:embed` | `internal/webassets` bakes on-disk asset bytes into the binary at build time |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-lvn-01 | Tampering | `git reset --hard` in Task 2 | high | mitigate | Task 2 `<precondition>` asserts `git status --porcelain --untracked-files=no` is empty and HALTs otherwise, so no uncommitted tracked work can be discarded; untracked planning files are unaffected by `reset --hard`, and `git clean` is explicitly forbidden |
| T-lvn-02 | Tampering | `* text=auto` vs. tracked `.woff2`/`.ico` binaries | high | mitigate | Explicit `binary` (`-text -diff`) rules for woff2/ico/png/jpg/jpeg/gif/webp/pdf; Task 1's verify asserts `text: unset` on a real tracked `.woff2` and `.ico`; Task 2 runs `go build ./...` + `go test ./internal/webassets/...` against the re-checked-out embedded tree |
| T-lvn-03 | Tampering | scope creep in the renormalization commit | medium | mitigate | MUTABLE-SCOPE rule in Task 1: the committed set is authorized from live `git diff --cached --name-only`, and any path beyond `.gitattributes` must show an empty whitespace-insensitive diff (`git diff --cached -w --numstat` with no non-zero rows) or the task HALTs |
| T-lvn-04 | Denial of Service | future `.sh` hook scripts | low | mitigate | `*.sh text eol=lf` is present before any `.sh` is tracked, so a CRLF shebang can never produce a bad-interpreter failure in the hook or Makefile path |
| T-lvn-05 | Information Disclosure | commit contents | low | accept | This task adds one attributes file and rewrites line endings; no secret material is introduced. The gitleaks pre-commit hook runs on the commit as the standing backstop |

**Supply chain:** no npm/pip/cargo install occurs in this plan — no package is added,
removed, or upgraded — so no `T-{phase}-SC` legitimacy gate applies.
</threat_model>

<verification>
1. `.gitattributes` exists at the repo root, is committed to HEAD, and carries a
   comment header naming `core.autocrlf`, `web/.prettierrc`, and the `prettier-web`
   hook as the reason it exists.
2. `git check-attr eol` resolves to `lf` for `.tsx`, `.go`, and `Makefile`;
   `git check-attr text` resolves to `unset` for a tracked `.woff2` and a tracked
   `.ico`.
3. `git ls-files --eol | grep -c 'w/crlf'` returns `0` (455 before the refresh).
4. `corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}"` passes, and
   `git status --porcelain --untracked-files=no` is empty immediately before and
   immediately after the prettier run.
5. `go build ./...` and `go test ./internal/webassets/...` pass.
6. `git config --get core.autocrlf` still returns `true` — the per-machine setting
   was not touched.
7. The commit history contains no content change beyond `.gitattributes` unless a
   live `git add --renormalize .` staged more, in which case those paths were proven
   line-endings-only and the SUMMARY records the exact set.
</verification>

<success_criteria>
- The perpetual stat-dirty loop is gone: running the `prettier-web` hook no longer
  rewrites a single file, because nothing on disk carries CRLF anymore.
- The fix is portable — it travels with the repo for every future clone and every
  contributor, and required no change to any per-machine git config.
- No source logic changed. Committed content is `.gitattributes` plus, at most,
  line-ending-only blob normalization proven by an empty whitespace-insensitive diff.
- The local `go:embed` asset bytes now match what CI and Docker have always built.
</success_criteria>

<output>
Create `.planning/quick/260901-lvn-add-repo-root-gitattributes-and-renormal/260901-lvn-SUMMARY.md` when done.

The SUMMARY must record, from live observation rather than from this plan's
expectations:
- the exact path list `git add --renormalize .` staged (or explicitly, that it staged
  nothing beyond `.gitattributes`), and how line-endings-only was proven if it staged
  more;
- the `w/crlf` count before and after the working-tree refresh;
- confirmation that `core.autocrlf` was left at `true`.
</output>
