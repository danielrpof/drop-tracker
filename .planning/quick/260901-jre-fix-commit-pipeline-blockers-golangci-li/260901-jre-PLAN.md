---
phase: quick-260901-jre
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/authgate/session.go
  - internal/httpserver/server.go
  - web/app/components/auth/PassphraseScreen.test.tsx
  - web/app/components/auth/PassphraseScreen.tsx
  - web/app/lib/api.test.ts
  - web/app/lib/authStore.ts
  - web/app/root.test.tsx
  - web/app/routes/watchlist.test.tsx
autonomous: true
requirements:
  - QUICK-260901-jre
user_setup: []

estimate:
  tokens: 40000
  raw_tokens: 20000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "`go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...` exits 0 with zero reported issues — the CI `lint` job's exact linter version and exact command."
    - "`middleware.RealIP` is still registered and still gated behind `gate != nil && cfg.trustProxyHeaders` — the D-14 fail-safe default is unchanged, RealIP is suppressed at the linter, not removed from the code."
    - "The two G115 findings in `unmarshalPayload` are silenced by two line-scoped directives on the two flagged conversion lines themselves — no `.golangci.yml` exclusion rule is added for gosec or staticcheck."
    - "Every git-tracked `.ts`/`.tsx` file under `web/`, read LF-normalized, is prettier-clean against `web/.prettierrc` — mirroring what CI's fresh LF checkout sees."
    - "`vitest run` still reports 125 passed / 0 failed after the formatter rewrites."
    - "`go build ./...` and `go vet ./...` both succeed, and the existing `internal/authgate` and `internal/httpserver` package tests still pass."
    - "The committed diff contains exactly the 8 files in `files_modified` and nothing else — no `go.mod`, no `go.sum`, no `.golangci.yml`, no `.github/workflows/full-pipeline.yml`, no `.env.example`, no `docker-compose.yml`, and nothing under `.planning/` outside this quick task's own directory."
    - "The web-file half of the diff is content-only: no file whose sole change is a line-ending conversion appears in the commit."
  artifacts:
    - "internal/authgate/session.go (modified — trailing suppression directive + rationale on the two flagged conversion lines)"
    - "internal/httpserver/server.go (modified — trailing suppression directive + rationale on the single RealIP registration line)"
    - "web/app/components/auth/PassphraseScreen.tsx, PassphraseScreen.test.tsx, web/app/lib/api.test.ts, web/app/lib/authStore.ts, web/app/root.test.tsx, web/app/routes/watchlist.test.tsx (reformatted by prettier)"
    - ".planning/quick/260901-jre-fix-commit-pipeline-blockers-golangci-li/260901-jre-SUMMARY.md"
  key_links:
    - "directive placement -> golangci-lint's per-line suppression: the directive must sit on the SAME source line as the flagged expression, as a trailing comment with no space between the slashes and the word. The repo already uses exactly this form in internal/detection/detector.go:268, internal/detection/musicbrainz.go:509-510, internal/httpserver/events.go:69 and internal/db/migrate.go:106 — match it."
    - "`web/.prettierrc` endOfLine:lf + this machine's core.autocrlf=true -> the git index: CI checks out LF blobs, this worktree holds CRLF. A prettier check over raw worktree bytes over-reports ~46 files; the same check over LF-normalized bytes reports exactly 6. Only the LF-normalized answer predicts CI."
    - "prettier config resolution -> the process working directory: running the check from the repo root reports all 46 tracked files as unformatted (verified — config/plugin resolution differs), while running it with cwd `web/` reports the true 6. Every prettier invocation in this plan must run from `web/`."
    - "`if gate != nil && cfg.trustProxyHeaders` -> D-14's fail-safe posture: this guard is the actual mitigation for the header-spoofing advisory that SA1019 is warning about. Suppressing the warning is only defensible while that guard stands, so Task 1's own gate asserts it survived."
---

<objective>
Turn the two red CI jobs on `origin/main` green — the `lint` job (3 golangci-lint findings) and the `frontend-test` job (prettier `--check` step) — without changing any behavior, any dependency, or any CI configuration.

Purpose: Phase 14 was executed across ~69 commits that were never pushed. This dev machine has no `golangci-lint`, no `pnpm`, and no Docker, so the lint and prettier gates never ran during the phase. The push of commit `465260c` finally ran them and two jobs went red. The doc-only commits (`9fd4025`, `465260c`) did not cause this — the findings predate them. `build-scan` and `release` are blocked behind `needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]`, so main cannot ship until both are green.

The three lint findings, confirmed against the current source:

- `internal/authgate/session.go:123:33` and `:124:31` — gosec **G115**, integer overflow conversion `uint64 -> int64`, on the two `int64(binary.BigEndian.Uint64(...))` calls inside `unmarshalPayload`.
- `internal/httpserver/server.go:133:9` — staticcheck **SA1019**, `middleware.RealIP` is deprecated by chi over IP-spoofing advisories (GHSA-3fxj-6jh8-hvhx, GHSA-rjr7-jggh-pgcp, GHSA-9g5q-2w5x-hmxf).

Both are **suppress-with-rationale**, not rewrite:

- The G115 pair is a deliberate, lossless round-trip. `marshalPayload` (same file, lines 110-111) writes `uint64(t.IssuedAt.UnixNano())` and `uint64(t.Expiry.UnixNano())` into an 8-byte big-endian wire field; `unmarshalPayload` reads the same bytes back. The reverse conversion is two's-complement exact — it restores the original `int64` bit-for-bit. There is no truncation and no reachable overflow; gosec simply cannot see the paired write.
- The SA1019 finding is already mitigated by design. Phase 14's decision **D-14** gates `middleware.RealIP` behind `TRUST_PROXY_HEADERS` (default `false`), and `server.go:121-131` carries a load-bearing comment explaining that the flag is enabled only alongside Phase 17's topology, where the container port is never published and the reverse proxy is the only party able to set `X-Forwarded-For`. The advisory's own stated mitigation — trust these headers only behind a proxy that sets them — **is** the existing design. Removing `RealIP` would discard a Phase 17 prerequisite to satisfy a linter.

The frontend failure is real formatting drift: Phase 14's frontend code was committed without prettier ever running. Verified two independent ways (prettier against the committed git blobs, and prettier against LF-normalized worktree bytes) — both agree on exactly six files.

Output: two Go files with line-scoped, rationale-carrying suppressions; six frontend files reformatted; a local run of both CI gates proving green.

Scope discipline — do NOT touch:
- **`.golangci.yml`.** Do not add a gosec or staticcheck entry to its `exclusions.rules` block, do not disable a linter, do not relax `default: standard`. A config-level exclusion would silence these checks across every file that matches the rule; a line-scoped directive silences exactly the two known-safe sites and leaves the linter armed everywhere else. The existing `_test.go` gosec exclusion stays exactly as it is.
- **`.github/workflows/full-pipeline.yml`.** Do not remove or weaken the "Check formatting" step, do not add `--ignore-path`, do not narrow the glob, do not make `frontend-test` non-blocking, do not touch `build-scan`'s `needs:` array. Making a gate stop asking is not making the code pass.
- **Behavior.** `middleware.RealIP` stays registered and stays behind its existing `gate != nil && cfg.trustProxyHeaders` guard. `marshalPayload`/`unmarshalPayload` keep their current wire format and current conversions. Nothing in this plan changes what a byte on the wire means.
- **Dependencies.** No `go get`, no `go mod tidy`, no npm/pnpm install. `go.mod`, `go.sum`, `web/package.json`, and `web/pnpm-lock.yaml` are all unchanged. Note that `go run <module>@<version>` deliberately does not touch `go.mod`; if it somehow appears modified, revert it rather than committing it.
- **`.env.example`, `docker-compose.yml`, and anything under `.planning/` except this quick task's own directory.**
- **`web/app/lib/api.ts`.** It was suspected of drift but is verified prettier-clean at HEAD. Do not reformat it, and do not "fix" any file that this plan's own gate does not report.
- **Line endings.** Do not run `dos2unix`, do not add a `.gitattributes`, do not normalize the worktree. This machine's `core.autocrlf=true` already produces LF blobs in the index; the CRLF worktree is not a defect and is not in scope.
- **The pre-existing `tsc --noEmit` failure** (`app/root.tsx(19,28): TS2307 Cannot find module './+types/root'` — a stale react-router typegen artifact). `typecheck` is not a CI job. Leave it.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@internal/authgate/session.go
@internal/httpserver/server.go
@.golangci.yml
@.github/workflows/full-pipeline.yml

**Known environment facts (verified live during planning — do not re-derive, and do not assume the opposite):**

- **No `golangci-lint` binary, no `pnpm`, and no Docker on this machine.** Run the linter through the Go module cache instead: `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...`. That module is already in the local module cache. `v2.12.2` is the exact version `.github/workflows/full-pipeline.yml` pins for the `lint` job and `.pre-commit-config.yaml` pins for the local hook, so a local verdict is a valid prediction of CI.
- **That linter run takes roughly 3-4 minutes.** The Bash tool's default timeout is 120s and will kill it mid-run, which looks identical to a failure. Pass an explicit `timeout` of at least `600000` on any call that runs it.
- **Run every prettier invocation with the working directory set to `web/`.** Running prettier from the repo root reports all 46 tracked `.ts`/`.tsx` files as unformatted — config/plugin resolution differs by cwd. This was tested both ways during planning; only the `web/`-cwd form matches CI.
- **Use `web/node_modules/.bin/prettier` directly** (as `node_modules/.bin/prettier` once cwd is `web/`). There is no `pnpm` to run `pnpm exec` or the `format` script with.
- **`core.autocrlf=true`, no `.gitattributes`.** The worktree is CRLF, the committed blobs are LF, and git normalizes to LF on `git add`. Consequence: a prettier check over raw worktree bytes over-reports massively, and the correct local gate pipes each file through `tr -d '\r'` first.
- **`go test -race` and `make test-integration` do not work here** — no Docker for the Postgres fixture, and ThreadSanitizer fails to allocate on this box (a documented pre-existing limitation, see STATE.md Phase 11.1-04). Substitute `go test -short -count=1` on the specific packages. Do not attempt `make test`, `make test-short` (it passes `-race`), or `make test-integration`. The `test` CI job is green already and is not this plan's concern.
- **No pre-commit hook is installed** (`.git/hooks/pre-commit` does not exist), so committing will not trigger a local golangci-lint or gitleaks run. Verification is this plan's job, not the hook's.
- **`web/` is currently clean** relative to HEAD — all six target files are byte-identical to their committed blobs once line endings are normalized. Any diff produced by this plan is a diff this plan created.
</context>

<tasks>

<task type="tracer">
  <name>Task 1: Silence the three golangci-lint findings with line-scoped, rationale-carrying directives</name>
  <files>internal/authgate/session.go, internal/httpserver/server.go</files>
  <read_first>internal/authgate/session.go lines 107-127 (marshalPayload and unmarshalPayload — the paired write is the whole justification), internal/httpserver/server.go lines 119-134 (the D-14 comment block and the guarded RealIP registration), and internal/detection/musicbrainz.go lines 509-510 (the repo's established directive form and rationale style).</read_first>
  <action>
Add three suppression directives, each as a trailing comment on the exact source line the linter flagged. Do not restructure any code, do not extract a helper, do not add or remove a statement. Three lines gain a trailing comment; nothing else in either file changes.

In `internal/authgate/session.go`, inside `unmarshalPayload`, the two lines that assign `t.IssuedAt` and `t.Expiry` each get a trailing `//nolint:gosec` directive naming rule G115, followed by a rationale. The rationale must state that the value being converted was itself written by `marshalPayload` as the unsigned form of the same field's `UnixNano()`, so this is the exact inverse of that write and the two's-complement round-trip is lossless. Say that the conversion is total rather than narrowing — both types are 64 bits wide, so no value is unrepresentable and no truncation is possible — and that the worst outcome from a forged or corrupted payload is a nonsense timestamp, which `Verify` already rejects through its absolute-cap and expiry checks after the constant-time MAC comparison has passed. Note that a signed decode is exactly what the fixed-width wire format documented at `payloadLen` intends. Keep each directive on its own line with its own statement — do not merge the two assignments or hoist a single directive above the function, because a directive on a preceding line does not scope to the lines below it.

In `internal/httpserver/server.go`, the single `r.Use(middleware.RealIP)` line gets a trailing `//nolint:staticcheck` directive naming rule SA1019, followed by a rationale. The rationale must state that chi deprecated this middleware over header-spoofing advisories, that the advisory's own stated mitigation is to trust these headers only behind a proxy that sets them, and that this is already precisely what the surrounding code does: decision D-14 gates the registration behind `TRUST_PROXY_HEADERS`, which defaults to `false`, and Phase 17 enables it only together with the topology where the container port is never published. Point the reader at the existing D-14 comment block directly above rather than restating it — that block is the real explanation and must survive this change byte-for-byte. Also record that removing `RealIP` is not the fix here: it is a Phase 17 prerequisite, and dropping it would leave the login throttle and audit log keyed on a proxy's own address once that topology lands.

Leave the `if gate != nil && cfg.trustProxyHeaders {` guard exactly as it is. It is the actual mitigation, and Task 1's gate asserts it is still present.

Match the repo's existing directive form exactly, as used in `internal/detection/detector.go`, `internal/detection/musicbrainz.go`, `internal/httpserver/events.go` and `internal/db/migrate.go`: two slashes immediately followed by the word, no space between them, then the linter name, then a space-separated comment marker and the prose. golangci-lint will not recognise the directive if a space is inserted after the slashes.

Do not add anything to `.golangci.yml`. Do not add a second directive for a linter that did not fire. Do not touch any other line in either file.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go vet ./... &amp;&amp; test "$(grep -cE 'int64\(binary\.BigEndian\.Uint64\(.*//nolint:gosec' internal/authgate/session.go)" = "2" &amp;&amp; test "$(grep -cE 'r\.Use\(middleware\.RealIP\).*//nolint:staticcheck' internal/httpserver/server.go)" = "1" &amp;&amp; test "$(grep -cF 'if gate != nil &amp;&amp; cfg.trustProxyHeaders {' internal/httpserver/server.go)" = "1" &amp;&amp; test "$(git diff --name-only -- .golangci.yml go.mod go.sum | wc -l)" = "0" &amp;&amp; go test -short -count=1 ./internal/authgate/... ./internal/httpserver/... &amp;&amp; go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...</automated>
  </verify>
  <done>
`go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...` exits 0 and reports zero issues across the whole module. Both `unmarshalPayload` conversion lines carry their own trailing gosec directive on the same line as the conversion; the `RealIP` registration line carries a trailing staticcheck directive on the same line. The `gate != nil && cfg.trustProxyHeaders` guard is still present and unmodified. `.golangci.yml`, `go.mod` and `go.sum` show no diff. `go build`, `go vet`, and the existing `internal/authgate` and `internal/httpserver` short tests all pass.
  </done>
</task>

<task type="auto">
  <name>Task 2: Reformat the six prettier-drifted Phase 14 frontend files</name>
  <files>web/app/components/auth/PassphraseScreen.tsx, web/app/components/auth/PassphraseScreen.test.tsx, web/app/lib/api.test.ts, web/app/lib/authStore.ts, web/app/root.test.tsx, web/app/routes/watchlist.test.tsx</files>
  <read_first>web/.prettierrc (printWidth 80, endOfLine lf, no semicolons, double quotes, es5 trailing commas, plus the tailwindcss plugin) and the "Check formatting" step in .github/workflows/full-pipeline.yml's frontend-test job.</read_first>
  <precondition>The working directory for every command in this task is `web/`, and `web/node_modules/.bin/prettier` exists (it does — the dependency tree is already installed locally even though `pnpm` itself is not available).</precondition>
  <action>
Run the formatter over exactly the six drifted files, named explicitly — not over a glob, and not over the whole tree. The drift set was established during planning by two independent methods that agreed exactly: prettier against each committed git blob, and prettier against each LF-normalized worktree file. Both reported these six and only these six.

From `web/`, invoke `node_modules/.bin/prettier --write` with these six paths as arguments: `app/components/auth/PassphraseScreen.test.tsx`, `app/components/auth/PassphraseScreen.tsx`, `app/lib/api.test.ts`, `app/lib/authStore.ts`, `app/root.test.tsx`, `app/routes/watchlist.test.tsx`.

Deliberately do NOT run the whole-tree glob the CI step uses. Under this machine's CRLF worktree, that glob rewrites every one of the 46 tracked files. Most of those rewrites are line-ending-only and would vanish at `git add` under `core.autocrlf=true`, so they would not reach the commit — but they would churn the worktree, bury the six real changes in review, and make it impossible to tell at a glance whether a seventh file was changed for a content reason. Stay surgical.

`app/lib/api.ts` is NOT in the list and must not be reformatted. It was suspected of drift but is verified clean. If the gate below ever reports a file this plan did not name, stop and report it rather than silently widening scope — an unexpected seventh file means the drift set moved and the finding needs a fresh look, not a reflexive `--write`.

After writing, review the resulting diff and confirm it is formatting only: reflowed lines at the 80-column boundary, quote and semicolon normalization, trailing-comma and indentation adjustments. If any hunk changes an identifier, a string literal's contents, a test assertion, an import specifier, or control flow, revert that file and stop — prettier does not do that, and such a hunk means something else edited the file.

Note that `--write` rewrites these six files with LF endings per `endOfLine: lf`, so their worktree line endings change from CRLF to LF. That is correct and expected: the index blobs were already LF, so the staged diff stays content-only. Do not undo it, and do not extend it to any other file.

Stage only these six paths. Then confirm nothing outside them is staged before committing.
  </action>
  <verify>
    <automated>cd web &amp;&amp; FAIL=0; for f in $(git ls-files '*.ts' '*.tsx'); do tr -d '\r' &lt; "$f" | node_modules/.bin/prettier --check --stdin-filepath "$f" &gt;/dev/null 2&gt;&amp;1 || { echo "UNFORMATTED: $f"; FAIL=1; }; done; test "$FAIL" = "0" &amp;&amp; node_modules/.bin/vitest run &amp;&amp; test "$(git diff --name-only -- package.json pnpm-lock.yaml .prettierrc | wc -l)" = "0"</automated>
  </verify>
  <done>
Every git-tracked `.ts`/`.tsx` file under `web/`, read LF-normalized, passes `prettier --check` against `web/.prettierrc` — the loop reports no `UNFORMATTED` line and exits 0. `vitest run` reports 125 passed, 0 failed. `web/package.json`, `web/pnpm-lock.yaml` and `web/.prettierrc` show no diff. The staged change set is exactly the six named files, and every hunk in it is a formatting change.
  </done>
</task>

<task type="auto">
  <name>Task 3: Prove both CI gates green together and that the diff is exactly eight files</name>
  <files>(verification only — no file changes)</files>
  <precondition>Tasks 1 and 2 are both complete and their changes are present in the working tree.</precondition>
  <action>
Run both gates back to back against the combined change set. Tasks 1 and 2 touch disjoint toolchains, but they land in one push, so the thing CI will actually evaluate is the combination — verify it as such rather than trusting two separate green runs.

Re-run the Go gate from the repo root, allowing at least 600000 ms of timeout for the linter:

    go build ./... && go vet ./...
    go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...

Then re-run the frontend gate with cwd `web/` — the LF-normalized loop from Task 2's verify, followed by `node_modules/.bin/vitest run`.

Then check the scope of the change set itself. `git status --porcelain` must list exactly the eight files in this plan's `files_modified`, plus this quick task's own planning directory, and nothing else. Confirm specifically that `go.mod`, `go.sum`, `.golangci.yml`, `.github/workflows/full-pipeline.yml`, `.env.example`, `docker-compose.yml`, `web/package.json` and `web/pnpm-lock.yaml` are all absent from it — the first two because `go run <module>@<version>` is supposed to leave them alone, the rest because weakening a gate or shifting a dependency is exactly the failure mode this plan exists to avoid.

Finally, confirm the web half of the diff carries no line-ending-only file: compare `git diff --cached --numstat` against `git diff --cached --numstat --ignore-cr-at-eol` and confirm the two agree, or equivalently confirm no staged file has a diff that disappears when carriage returns are ignored. Under `core.autocrlf=true` this should hold automatically; check it anyway, because a silent mass line-ending rewrite is the single most likely way this change becomes unreviewable.

Do NOT run `make test`, `make test-short`, `make test-integration`, or anything with `-race`. There is no Docker for the Postgres fixture and ThreadSanitizer does not work on this machine. The `test` job is already green in CI and nothing in this plan touches a code path it covers. If either gate is red, fix the finding — do not adjust the gate, the config, or the workflow.

Record in the summary: the linter's before (3 issues) and after (0 issues) counts, the six reformatted filenames, the vitest pass count, and the final file count in the diff. This task changes no files and produces no separate commit.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go vet ./... &amp;&amp; go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./... &amp;&amp; test "$(git status --porcelain -- go.mod go.sum .golangci.yml .github/workflows/full-pipeline.yml .env.example docker-compose.yml web/package.json web/pnpm-lock.yaml | wc -l)" = "0" &amp;&amp; cd web &amp;&amp; FAIL=0; for f in $(git ls-files '*.ts' '*.tsx'); do tr -d '\r' &lt; "$f" | node_modules/.bin/prettier --check --stdin-filepath "$f" &gt;/dev/null 2&gt;&amp;1 || { echo "UNFORMATTED: $f"; FAIL=1; }; done; test "$FAIL" = "0" &amp;&amp; node_modules/.bin/vitest run</automated>
  </verify>
  <done>
`go build`, `go vet`, and golangci-lint v2.12.2 all exit 0 with zero issues. The LF-normalized prettier loop reports no unformatted file. `vitest run` reports 125 passed, 0 failed. None of `go.mod`, `go.sum`, `.golangci.yml`, `.github/workflows/full-pipeline.yml`, `.env.example`, `docker-compose.yml`, `web/package.json` or `web/pnpm-lock.yaml` appears in `git status --porcelain`. The change set is exactly the eight files in `files_modified` plus this quick task's planning directory, and no staged file's diff consists solely of line-ending changes.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Untrusted client -> `X-Forwarded-For` / `X-Real-IP` -> `middleware.RealIP` -> login throttle + auth audit log | The header is attacker-controlled unless a trusted proxy is the only reachable path. This is the exact boundary SA1019 is warning about, and the one D-14 already governs. |
| Attacker-supplied session cookie -> `Verify` -> `unmarshalPayload` -> `Token` timestamps | A forged payload's bytes reach the two conversions G115 flags — but only after the constant-time HMAC comparison has already passed. |
| Developer change -> CI gate configuration | The gates (`.golangci.yml`, the prettier step, the `needs:` graph) are the control deciding whether an image may be published. A change that edits the gate instead of the code defeats it. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-JRE-01 | Spoofing | `middleware.RealIP` / client IP resolution in `internal/httpserver/server.go` | high | mitigate | Unchanged from Phase 14: registration stays behind `gate != nil && cfg.trustProxyHeaders`, `TRUST_PROXY_HEADERS` defaults to `false`, and the flag is enabled only with Phase 17's unpublished-container-port topology. This plan suppresses the linter warning about the risk, not the control against it. Task 1's gate asserts the guard literal is still present, so a future edit that ungates `RealIP` while leaving the suppression in place fails the plan's own verification rather than passing silently. |
| T-JRE-02 | Tampering | The `//nolint:staticcheck` directive on the `RealIP` line | medium | mitigate | A blanket or file-scoped suppression would hide a genuinely new chi deprecation later. Mitigated by scoping the directive to the single flagged line, naming the specific rule (SA1019), and requiring the rationale to point at the D-14 comment block that carries the real reasoning — so the next reader who deletes the guard sees why they cannot. No `.golangci.yml` exclusion is added, so `staticcheck` stays fully armed everywhere else in the module. |
| T-JRE-03 | Tampering | The two `//nolint:gosec` directives in `internal/authgate/session.go` | medium | mitigate | G115 is a real class of bug in a crypto package. Mitigated by two line-scoped directives on the two specific conversions rather than a package- or file-level exclusion; the rationale must name the paired `marshalPayload` write that makes the round-trip lossless. Task 1's gate asserts the directive count on those two lines is exactly two, so a third silently-suppressed conversion would fail verification. Residual exposure is nil: both types are 64 bits, the conversion cannot truncate, and a forged payload only reaches these lines after `hmac.Equal` passes and is then rejected by the absolute-cap and expiry checks. |
| T-JRE-04 | Tampering | CI gate configuration (`.golangci.yml`, `full-pipeline.yml`'s formatting step and `needs:` graph) | high | mitigate | The lowest-effort way to turn both jobs green is to weaken the gate. Explicitly forbidden in the objective and in both task actions; Task 1's and Task 3's gates assert `.golangci.yml` and the workflow file show no diff, and Task 3's gate re-runs the real linter at CI's pinned version rather than a narrowed invocation. |
| T-JRE-05 | Tampering | `prettier --write` over source and test files | low | accept | A formatter could in principle alter meaning. Accepted because prettier is an AST-preserving printer, the write is restricted to six explicitly named files, and the full 125-test vitest suite runs immediately afterwards as the regression gate. Task 2's action additionally requires a human-legible diff review that stops on any hunk touching an identifier, literal, assertion, import, or control flow. |
| T-JRE-06 | Repudiation | Mass CRLF-to-LF rewrite polluting the commit | low | mitigate | A whole-tree `prettier --write` on this CRLF worktree would rewrite all 46 tracked files and could bury a real change in noise. Mitigated by naming the six files explicitly instead of using a glob, and by Task 3's check that no staged file's diff consists solely of line-ending changes. |
| T-JRE-SC | Tampering | npm / pip / cargo / Go module installs | low | accept | No package is added, removed, or upgraded. `go.mod`, `go.sum`, `web/package.json` and `web/pnpm-lock.yaml` are asserted unchanged by two separate gates. The single network-reachable invocation, `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`, resolves a module already present in the local cache, is pinned to the identical version `.github/workflows/full-pipeline.yml` and `.pre-commit-config.yaml` already trust, and by design does not modify `go.mod`. Nothing new to audit. |
</threat_model>

<verification>
1. `go build ./...` and `go vet ./...` both exit 0.
2. `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...` exits 0 with zero issues — down from the 3 that are red on `origin/main` (allow at least 600000 ms; the run takes 3-4 minutes).
3. Both `unmarshalPayload` conversion lines carry their own trailing gosec directive naming G115, on the same line as the conversion.
4. The `r.Use(middleware.RealIP)` line carries a trailing staticcheck directive naming SA1019, and the `if gate != nil && cfg.trustProxyHeaders {` guard above it is unchanged.
5. `.golangci.yml` has no new exclusion rule and shows no diff at all.
6. Every git-tracked `.ts`/`.tsx` under `web/`, read LF-normalized with cwd `web/`, passes `prettier --check`.
7. `vitest run` reports 125 passed, 0 failed.
8. The change set is exactly the eight files in `files_modified`, plus this quick task's planning directory.
9. `go.mod`, `go.sum`, `.github/workflows/full-pipeline.yml`, `.env.example`, `docker-compose.yml`, `web/package.json` and `web/pnpm-lock.yaml` are all absent from `git status --porcelain`.
10. No staged file's diff consists solely of line-ending changes.
</verification>

<success_criteria>
- CI's `lint` job would pass: zero golangci-lint findings at the pinned v2.12.2, verified locally with CI's exact command.
- CI's `frontend-test` job would pass: the prettier `--check` step is clean against LF content, and the vitest suite still reports 125/125.
- Both findings are resolved by explaining them at the call site, not by weakening a linter config or removing a CI step.
- `middleware.RealIP` is still registered and still gated by `TRUST_PROXY_HEADERS` — Phase 17's prerequisite survives, and D-14's fail-safe default is intact.
- The session wire format is byte-for-byte unchanged; no behavior anywhere in the module differs.
- The commit is small and reviewable: three commented lines in Go, six mechanically reformatted frontend files, and no line-ending noise.
- The next person who hits a deprecated-but-deliberately-used dependency, or a lossless conversion gosec cannot see through, finds the reasoning in the code rather than re-deriving it.
</success_criteria>

<output>
Create `.planning/quick/260901-jre-fix-commit-pipeline-blockers-golangci-li/260901-jre-SUMMARY.md` when done.
</output>
