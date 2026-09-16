# Phase 15: PR Coverage-Diff Comment - Research

**Researched:** 2026-09-02
**Domain:** GitHub Actions CI reporting — coverage-diff sticky PR comment, Actions-cache baseline, Go/Vitest coverage-profile parsing
**Confidence:** HIGH (external mechanisms verified against primary sources this session; one repo-runtime number could not be measured in this environment — see Assumptions Log A1)

<user_constraints>
## User Constraints (from 15-CONTEXT.md)

The 20 locked decisions D-01…D-20 (grill-refined 2026-09-02) are authoritative. This research
does **not** relitigate them — it verifies the mechanisms they rest on. Key locked points the
planner MUST honor verbatim:

### Locked Decisions (condensed — full text in 15-CONTEXT.md)
- **D-01** Baseline = GitHub Actions cache, key `coverage-baseline-main-<sha>` per language, stable prefix `coverage-baseline-main-`, restored on PR runs via `restore-keys` prefix match. ~7-day idle eviction accepted.
- **D-02** Cache entry holds the full profiles (`coverage.out`, `coverage-summary.json`) **and** a precomputed sidecar `baseline-metrics.json` (backend total %, frontend `lines` %, baseline commit SHA, UTC timestamp). **Delta math reads the sidecar, not the profiles.**
- **D-03** Stale (prefix-matched) baseline is diffed silently; footer carries `baseline: main@<short-sha>` provenance only (no "N commits behind").
- **D-04** Two "never red" surfaces: (1) entire `coverage-comment` job best-effort, exits 0 on every failure mode; (2) the two `cache/save` baseline-publish steps carry **`continue-on-error: true`** (mandatory, not discretionary).
- **D-05** One comment via a custom Go renderer + `marocchino/sticky-pull-request-comment` (SHA-pinned, v3.0.5). Not the two ready-made single-language actions; not `k1LoW/octocov`.
- **D-06** Renderer is a Go tool at `cmd/coverage-report/`. Backend total = hand-parse `coverage.out` raw statement counts: `sum(numStmts where count>0) / sum(numStmts) * 100`, round half-up to 2 dp. No `golang.org/x/tools/cover`. Frontend reads `coverage-summary.json` `total.lines.pct` directly.
- **D-07** `cmd/coverage-report/` excluded from Makefile `COVER_PKGS` (extend the anchored `grep -vE`). Its own `go test` still runs.
- **D-08** Stickiness via `marocchino` fixed `header:` + hidden marker. Job-scoped `concurrency: { group: coverage-comment-${{ github.ref }}, cancel-in-progress: true }` — deliberately redundant with workflow-level cancel.
- **D-09** Body = visible `## Coverage` heading + 2-row table (Backend / Frontend): columns Coverage %, Δ vs main (signed pp), Gate (80%/70%), status mark (✅/⚠️).
- **D-10** Frontend row = Vitest `lines` axis only for headline % and Δ; Gate column reads `70%`.
- **D-11** No-baseline state: Δ column `—` both rows + one-line footer "Delta unavailable — no main baseline cached yet (first run or evicted). Absolute coverage shown." Provenance line omitted in this state.
- **D-12** 2 dp both rows (`82.41%`, `-0.12pp`); unchanged renders `±0.00pp`. Footer: PR head short-SHA, `baseline: main@<short-sha>`, UTC timestamp, and (when D-18 applies) a one-line note if an upstream test job was red.
- **D-13** Baseline published by `cache/save` steps **inside** existing `test` and `frontend-test` jobs, guarded `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`. No dedicated job. Two prefixed keys, one per language.
- **D-14** `cache/save` runs only after that job's coverage gate passed. Red `main` keeps the previous good baseline.
- **D-15** PR current profiles reach `coverage-comment` as short-retention (~1-day) artifacts uploaded on **every** run of `test` / `frontend-test`. Upload step sits **between** the test/build step and the gate step, `if: ${{ !cancelled() }}`. `coverage-comment` never re-runs any suite.
- **D-16 (supersedes discussion)** `coverage-comment` uses **default shallow checkout** (`fetch-depth: 1`). Tool delivered via `go run ./cmd/coverage-report`.
- **D-17** New `make coverage-report` target runs the D-06 tool in "print backend total only" mode. `make coverage-gate` refactored to consume that number instead of its own `go tool cover -func | grep '^total:' | awk` pipeline. Literal `80` comparison stays in the Makefile.
- **D-18** Comment still posts on a coverage-**gate** failure. `coverage-comment` `if: ${{ !cancelled() && github.event_name == 'pull_request' }}` (not implicit `success()`). `download-artifact` steps tolerate a missing artifact; tool renders per-row `unavailable` on absent/unparseable profile; footer notes upstream red.
- **D-19** `cmd/coverage-report` gets a scoped `gosec` G304 carve-out in `.golangci.yml`, mirroring the existing `_test.go` rule. Not an inline `//nosec`.
- **D-20** No-baseline detection keys off `steps.<restore>.outputs.cache-matched-key != ''` **and** the sidecar file existing on disk — **not** `cache-hit` (which is `'false'` on every prefix match).

### Claude's Discretion
- Exact cache key shape; one shared key vs two prefixes (D-13 implies two — confirm).
- `make coverage-report` internals — thin target shelling the tool vs. a `--mode=total` flag.
- Footer wording, heading text, ✅/⚠️ glyph choice.
- Artifact names and retention days (~1 day).
- `continue-on-error` at job level vs per-step for the comment job (the two `cache/save` steps' `continue-on-error: true` is NOT discretionary).
- Whether the Go tool or a separate step owns the `$GITHUB_STEP_SUMMARY` fork/degradation path.
- Exact hand-parse rounding rule (D-06 says half-up 2 dp — confirm it matches what `coverage-gate` compares after D-17).

### Deferred Ideas (OUT OF SCOPE)
- **CICD-15** — patch/diff-level (uncovered-new-lines) coverage. Future requirement, own phase. (This is when `fetch-depth: 0` and the Vitest `json` reporter get added.)
- Fork-PR comment posting via `pull_request` + `workflow_run` split. Fork PRs degrade to job summary. `pull_request_target` prohibited outright.
- All four Vitest axes in the comment (lines-only for now).
- A more durable baseline store (orphan branch / repo variable).
- Baseline-staleness annotation (commit-count / drift warning) — rejected; provenance SHA only.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CICD-13 | On a PR from a same-repo branch, CI posts and updates in place a single comment reporting backend and frontend coverage totals plus their delta versus the main-branch baseline; report-only, never blocks the merge. | Sticky-comment mechanism verified (marocchino v3.0.5 `header:` hidden marker — §Mechanism 2). Job outside every `needs:` graph + `if: ${{ !cancelled() && github.event_name == 'pull_request' }}` + job-scoped `pull-requests: write` (§Mechanism 6, §Repo Recon). Backend total via D-06 hand-parse (§Mechanism 3); frontend via `coverage-summary.json` `total.lines.pct` (§Mechanism 4). |
| CICD-14 | Main-branch pipeline runs publish their coverage as the baseline the PR comment diffs against; comment degrades gracefully (absolute numbers only) when no baseline is available. | `cache/save` on `push`→`refs/heads/main` after the gate, `continue-on-error: true` (§Mechanism 1). PR runs can restore base/default-branch caches (§Mechanism 1). No-baseline detection via `cache-matched-key` + sidecar existence, D-20 (§Mechanism 1). `restore` does not fail on total miss (`fail-on-cache-miss: false` default). |
</phase_requirements>

## Summary

This is a CI-only, report-only phase. Every mechanism the 20 decisions depend on was verified
against a primary source this session, and all check out:

1. **Actions cache** — `actions/cache/restore` and `actions/cache/save` are separate sub-actions
   of `actions/cache` (path suffix `/restore`, `/save`), currently `v6.1.0`
   (`55cc8345863c7cc4c66a329aec7e433d2d1c52a9`). `restore` outputs `cache-hit`,
   `cache-primary-key`, `cache-matched-key`; on a `restore-keys` prefix hit `cache-hit` is the
   string `'false'` while `cache-matched-key` is set — **D-20 is correct**. A total miss does
   not fail the step (`fail-on-cache-miss` defaults `false`). `save` against an already-reserved
   key emits a warning annotation and exits 0 (no step failure) — D-04's `continue-on-error: true`
   still required for real cache-service 5xx/quota errors. PR runs **can** restore caches created
   on the base/default branch (`main`); a PR cannot write to or poison `main`'s cache scope.

2. **`marocchino/sticky-pull-request-comment`** — v3.0.5 SHA is
   `5770ad5eb8f42dd2c4f34da00c94c5381e49af88` (**confirmed via GitHub API**, matches STACK.md).
   `header:` is the hidden marker; auto-detects PR number under `pull_request`. Requires
   `pull-requests: write` (+ `contents: read` on private repos). On a fork PR the token is
   read-only → the API call 403s → **step fails** unless guarded — hence the same-repo `if:`
   guard + `$GITHUB_STEP_SUMMARY` fallback.

3. **Go `coverage.out`** — repo profile is `mode: atomic`. Grammar:
   `file:startLine.startCol,endLine.endCol numStmts count` per block. D-06's total formula
   (`sum(numStmts where count>0) / sum(numStmts) * 100`, statement-weighted) is exactly what
   `go tool cover -func`'s `total:` line computes — the tool reproduces that number with more
   decimals. `golang.org/x/tools/cover` is genuinely unnecessary (~15-line hand-parse).

4. **Vitest `json-summary`** — `@vitest/coverage-v8` 4.1.10 (already installed) writes
   `coverage/coverage-summary.json` with `total.{lines,statements,functions,branches}.{total,covered,skipped,pct}`.
   **Reports are generated before thresholds are enforced**
   (`packages/vitest/src/node/coverage.ts:223-224`) — so a sub-70 frontend run still leaves a
   complete `coverage-summary.json` on disk for D-15's hand-off. Confirmed.

5. **Artifact hand-off** — repo already pins `actions/upload-artifact` v7.0.1
   (`043fb46d1a93c77aae656e7c1c64a875d1fc6a0a`) and `actions/download-artifact` v8.0.1
   (`3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c`); both current. `download-artifact` with a
   `name:` that does not exist in the run **fails the step** (`Error: Unable to find an artifact
   with the name …`) → D-18's `continue-on-error` / existence check is required.
   `retention-days: 1` is valid (repo already uses `retention-days: 1` for `scanned-image`).
   Same-run cross-job artifact download needs no special config.

6. **`if: ${{ !cancelled() && github.event_name == 'pull_request' }}`** — dropping `success()`
   from the expression lets the job run when a `needs:` job **failed** (D-18's SC #4 case) while
   `!cancelled()` still skips it when a `needs:` job was cancelled (a re-push supersede). This
   is the correct idiom vs `always()`, which would also run on cancellation.

**Primary recommendation:** Build `cmd/coverage-report/` as a small tested `main` package with a
`--mode=total|comment` flag (or subcommand); wire `make coverage-report` → `coverage-gate` for
D-17; add three step-groups to `full-pipeline.yml` (`cache/save`+`upload-artifact` on `test`,
same on `frontend-test`, a new `coverage-comment` job); pin `actions/cache@v6.1.0` and
`marocchino/sticky-pull-request-comment@5770ad5…` with `# vX.Y.Z` comments. Verify the real
backend raw coverage total has margin above 80 on a live `make test-integration` run before the
D-17 cutover (Assumptions Log A1 — expected ~87%, LOW risk, but unmeasured here).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Compute backend total from `coverage.out` | Go CLI tool (`cmd/coverage-report`) | — | D-06: source-tree-independent parse, one algorithm shared with the gate (D-17) |
| Compute frontend `lines` % | Go CLI tool (reads Vitest `coverage-summary.json`) | Vitest (`@vitest/coverage-v8`) writes the file | D-10: `lines` axis only; file is already effectively 2 dp |
| Read baseline numbers | Go CLI tool (reads `baseline-metrics.json` sidecar) | `actions/cache/restore` fetches the sidecar | D-02: never re-parse the cached profile on the PR head |
| Persist the baseline | `actions/cache/save` in `test` / `frontend-test` | Go tool writes the sidecar during `make coverage-report` | D-13/D-02: publish on `push`→`main` after the gate |
| Render + post the comment | `marocchino/sticky-pull-request-comment` | Go tool emits the markdown to a file/stdout | D-05/D-08: custom render, third-party for idempotent upsert only |
| Fork / read-only-token degradation | `$GITHUB_STEP_SUMMARY` write | Go tool or a dedicated `run:` step | D-04/Pitfall 25: never fail, never `pull_request_target` |
| Enforce coverage (unchanged) | `make coverage-gate` (80) + Vitest `coverage.thresholds` (70) | — | Report-only phase; these stay the only blockers |

## Standard Stack

### Actions / tools this phase adds or touches

| Action / tool | Version | Pinned SHA | Purpose | Provenance |
|---------------|---------|-----------|---------|------------|
| `actions/cache/restore` | v6.1.0 | `55cc8345863c7cc4c66a329aec7e433d2d1c52a9` | Restore the main-branch coverage baseline on PR runs (prefix match) | `[VERIFIED: GitHub API repos/actions/cache/git/ref/tags/v6.1.0]` |
| `actions/cache/save` | v6.1.0 | `55cc8345863c7cc4c66a329aec7e433d2d1c52a9` | Publish the baseline on `push`→`main` after the gate | `[VERIFIED: GitHub API]` (same repo, `/save` path suffix) |
| `marocchino/sticky-pull-request-comment` | v3.0.5 | `5770ad5eb8f42dd2c4f34da00c94c5381e49af88` | Idempotent upsert of one PR comment keyed on a hidden `header:` marker | `[VERIFIED: GitHub API repos/marocchino/sticky-pull-request-comment/git/ref/tags/v3.0.5]` + `[CITED: STACK.md §Feature 3]` |
| `actions/upload-artifact` | v7.0.1 | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` | Hand off PR current profiles to `coverage-comment` | `[VERIFIED: GitHub API]` — **already pinned in `full-pipeline.yml`, reuse the exact line** |
| `actions/download-artifact` | v8.0.1 | `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c` | `coverage-comment` pulls both profiles | `[VERIFIED: GitHub API]` — **already pinned, reuse** |
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` | Default shallow checkout on `coverage-comment` for `go run` (D-16) | `[VERIFIED: full-pipeline.yml:20]` — reuse |
| `actions/setup-go` | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` | `coverage-comment` needs Go to `go run ./cmd/coverage-report` | `[VERIFIED: full-pipeline.yml:22]` — reuse |
| Go stdlib `testing` | Go 1.26 (`go.mod`) | — | Unit tests for `cmd/coverage-report` | `[VERIFIED: go.mod:3]` |
| `@vitest/coverage-v8` | 4.1.10 | — | Already installed; `json-summary` reporter needs no new dep | `[VERIFIED: web/package.json]` |

### No new dependencies

- **No new Go module.** D-06 hand-parses `coverage.out` (~15 lines). `golang.org/x/tools/cover`
  is not added. `[CITED: 15-CONTEXT.md D-06]`
- **No new npm package.** `json-summary` is a built-in reporter of the already-installed
  `@vitest/coverage-v8`. `[VERIFIED: web/package.json]`
- **No new secret, no new env var.** `GITHUB_TOKEN` (ambient) is the only credential; scoped
  `pull-requests: write` at the job. `[CITED: 15-CONTEXT.md §"Locked upstream"]`

### Alternatives Considered (all rejected by CONTEXT — do not revisit)

| Instead of | Rejected alternative | Why rejected (per CONTEXT) |
|------------|---------------------|----------------------------|
| Custom Go renderer + marocchino | `fgrosse/go-coverage-report` + `davelosert/vitest-coverage-report-action` | Produces **two** comments; violates SC #1/#2 (D-05) |
| Custom Go renderer + marocchino | `k1LoW/octocov` | Introduces a datastore concept overlapping the D-01 cache decision (D-05) |
| Actions cache baseline | Orphan `coverage-baseline` branch / 90-day artifact / repo variable | Extra machinery; 7-day eviction accepted (D-01) |
| Sidecar metrics file | Re-parse the cached profile on the PR head | PR-head source tree cannot reliably re-derive an older baseline's total (renamed/deleted files) — D-02 |
| Go tool | Inline bash `run:`, Node/JS script | Portfolio framing + unit-testability + one shared algorithm (D-06) |
| `json-summary` only | `json-summary` + `json` | Full `coverage-final.json` is large; only the rejected `davelosert` action needed it. Add `json` with CICD-15 (D-06 review note) |

## Package Legitimacy Audit

Only one non-first-party action is added. `actions/*` are GitHub-authored. No npm or Go packages
are added.

| Package | Registry | Age | Usage | Source Repo | Verdict | Disposition |
|---------|----------|-----|-------|-------------|---------|-------------|
| `marocchino/sticky-pull-request-comment` | GitHub Actions (JS/Node20 action) | Created 2019, actively maintained (v3.0.5 latest tag) | Widely used across the ecosystem; `fgrosse/go-coverage-report` uses it internally | `github.com/marocchino/sticky-pull-request-comment` (present, public) | OK | Approved — SHA-pin `5770ad5eb8f42dd2c4f34da00c94c5381e49af88` with trailing `# v3.0.5` comment per CLAUDE.md |
| `actions/cache` (`/restore`, `/save`) | GitHub first-party | Long-established | Ubiquitous | `github.com/actions/cache` | OK | Approved — pin `55cc8345863c7cc4c66a329aec7e433d2d1c52a9` `# v6.1.0` |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*The GSD `package-legitimacy check` seam targets npm/PyPI/crates; it does not cover GitHub
Actions. Verification here is: (a) SHA resolved via the GitHub tags/refs API this session, (b)
the action is the one named in the milestone STACK.md research, (c) CLAUDE.md's SHA-pin +
version-comment convention is applied.*

## Mechanism Verification (the core of this research)

### Mechanism 1 — `actions/cache` save/restore

**Sub-action structure.** `actions/cache/restore` and `actions/cache/save` are directories in
the `actions/cache` repo. Pin both to the **same** SHA with a path suffix:
`uses: actions/cache/restore@55cc8345863c7cc4c66a329aec7e433d2d1c52a9 # v6.1.0` and
`.../save@55cc8345… # v6.1.0`. Latest release is v6.1.0.
`[VERIFIED: GitHub API repos/actions/cache tags + releases/latest]`

**`restore` inputs:** `path` (required), `key` (required), `restore-keys` (newline list of
prefixes), `fail-on-cache-miss` (default `false`), `lookup-only`, `enableCrossOsArchive`.
**`restore` outputs:** `cache-hit`, `cache-primary-key`, `cache-matched-key`.
`[CITED: github.com/actions/cache/blob/main/restore/README.md]`

**D-20 confirmed.** `cache-hit` is `'true'` **only** on an exact primary-key match. On a
`restore-keys` prefix match (or a miss), `cache-hit` is the string `'false'`. On a prefix match,
`cache-matched-key` holds the actual matched key (non-empty); on a full miss it is empty.
Because the key here is `coverage-baseline-main-<sha>` and the PR never knows `main`'s latest
SHA, the primary key can never match — so **`cache-hit` is always `'false'` on every PR** and a
plan that branches on it reports "no baseline" every time. Branch on
`steps.<id>.outputs.cache-matched-key != ''` **AND** the sidecar file existing on disk.
`[VERIFIED: GitHub API + actions/cache restore README + WebSearch corroboration]`

**Total miss does not fail the step.** `fail-on-cache-miss` defaults `false`, so the first run
after the feature ships (no baseline yet) proceeds cleanly to the D-11 no-baseline path.
`[CITED: actions/cache restore README]`

**`save` inputs:** `path` (required), `key` (required), `upload-chunk-size`,
`enableCrossOsArchive`. **No outputs.** `[CITED: github.com/actions/cache/blob/main/save/README.md]`

**`save` on an already-reserved key.** When the reservation fails ("Cache already exists" / "Key
already reserved" — e.g. a re-run on the same `main` SHA, or a `push` + `pull_request` race),
`@actions/cache` catches the exception, emits a **job warning annotation**, and does **not**
propagate the failure — the step exits 0. `[VERIFIED: actions/toolkit#1642, #537, #1946 via WebSearch]`
D-04's `continue-on-error: true` on the two `cache/save` steps is therefore belt-and-suspenders
for the already-exists case but genuinely load-bearing for cache-service 5xx / quota-exceeded
errors, which *can* fail the step. **Keep it — it is mandatory per D-04.**

**Cache scope / cross-ref visibility.** A workflow run triggered by `pull_request` can restore
caches created on: its own ref, the PR **base branch**, and the repository **default branch**.
Since `main` is both the base and the default branch, PR runs **can** read the baseline written
by a `push`→`refs/heads/main` run. Caches **created** in a PR run are scoped to that PR only —
they cannot be restored by `main` or by other PRs, so **a PR cannot poison the baseline**.
`[CITED: docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching]`

**Eviction / limits.** Repo cache cap is 10 GB; entries not accessed for **7 days** are evicted.
The baseline entries are a few KB (sidecar + two small profiles) so size is a non-issue; a
`restore` counts as access, so a repo with regular PR activity keeps the baseline warm. Only a
genuinely idle `main` (no run touches the prefix for 7 days) loses it → D-11 no-baseline path.
`[CITED: dependency-caching reference; matches 15-CONTEXT.md D-01 stated trade-off]`

**Key shape recommendation (Discretion).** Two prefixes, one per language, matching D-13's
"each job saves its own language's profile under its own prefixed key":
- backend: primary `coverage-baseline-main-backend-${{ github.sha }}`, restore-key `coverage-baseline-main-backend-`
- frontend: primary `coverage-baseline-main-frontend-${{ github.sha }}`, restore-key `coverage-baseline-main-frontend-`

Each entry's `path:` = that language's profile **plus** the sidecar (D-02). The sidecar can be
per-language (`baseline-metrics-backend.json` / `-frontend.json`) or the two jobs can each write
a language-scoped sidecar; a single merged sidecar would require a cross-job write which D-13's
"no dedicated baseline job" precludes. **Recommend per-language sidecars**, merged in the Go
tool at read time.

### Mechanism 2 — `marocchino/sticky-pull-request-comment` v3.0.5

**SHA confirmed:** `5770ad5eb8f42dd2c4f34da00c94c5381e49af88`
`[VERIFIED: GitHub API repos/marocchino/sticky-pull-request-comment/git/ref/tags/v3.0.5]`

**Inputs** (`[CITED: action README]`):
`header` (hidden marker identifying the comment for updates — **not shown to users**),
`message` (inline body), `path` (read body from a file — **use this**, point at the Go tool's
output file), `recreate` (delete + recreate instead of edit), `hide_and_recreate`,
`only_update` (update only if it exists — never create), `only_create`, `append`,
`skip_unchanged` (skip the API write if the body is byte-identical — **useful to avoid a
no-op edit notification on every re-push**), `ignore_empty`, `GITHUB_TOKEN`, `number` (PR
number, only needed for `push`/`workflow_dispatch`/`workflow_run` events), `hide`,
`hide_classify`, `delete`, `owner`, `repo`.

**Marker mechanism.** The action inserts an invisible HTML comment derived from `header:` into
the comment body; on the next run it lists PR comments, finds the one carrying that marker, and
`PATCH`es it in place — creating only if none is found. This is exactly SC #2 ("three pushes
leave one comment"). Use a stable `header:` such as `coverage-report`.

**PR number.** Under `pull_request` the action auto-detects the number from the event payload —
`number:` is **not** needed. `[CITED: action README]`

**Permissions.** Requires `pull-requests: write`. `contents: read` also needed on private repos
(this repo is public, but set both at the job for clarity). `[CITED: action README + Pitfall 24]`

**Fork / read-only token.** On a PR from a fork, `GITHUB_TOKEN` is read-only regardless of the
`permissions:` block; the comment `POST`/`PATCH` returns `403 Resource not accessible by
integration` and the **step fails**. The action does not silently no-op. Mitigation (D-04 /
Pitfall 25): guard the marocchino step with
`if: github.event.pull_request.head.repo.full_name == github.repository` and, on the else path
(or unconditionally as a prior step), write the rendered table to `$GITHUB_STEP_SUMMARY`. Job
also carries `continue-on-error` (D-04) as the final backstop.
`[VERIFIED: Pitfall 25 + action README 403 semantics]`

**Runtime:** Node 20 action. No `actions/checkout` dependency of its own (uses the API).

### Mechanism 3 — Go `coverage.out` profile format

**Repo profile mode:** first line is `mode: atomic` `[VERIFIED: C:/CodeProjects/drop-tracker/coverage.out:1]`
(the committed file is stale/near-empty — 13 bytes — but the mode line is authoritative; the
Makefile runs `go test ... -race` which forces `atomic`).

**Grammar** (after the `mode:` line, one line per basic block):
```
<import-path>/<file>.go:<startLine>.<startCol>,<endLine>.<endCol> <numStmts> <count>
```
e.g. `github.com/danielrpof/drop-tracker/internal/authgate/session.go:41.2,43.16 3 1`

- `<numStmts>` — number of statements in the block.
- `<count>` — execution count. Under `set` it is 0/1; under `count`/`atomic` it is a hit
  counter. **`count > 0` ⇔ block covered** in all three modes, so D-06's `count>0` test is
  mode-independent. `[VERIFIED: 15-CONTEXT.md D-06 + Go cover tool semantics]`

**D-06 total formula confirmed.** `go tool cover -func`'s `total:` line is the
statement-weighted ratio `Σ(numStmts of covered blocks) / Σ(numStmts of all blocks) * 100`.
The hand-parse computes the identical quantity; the only difference is `-func` formats it
`%.1f%%` (1 dp) while the tool can print 2 dp (D-12). So D-17's swap changes **formatting/
rounding precision only**, not the measured quantity — subject to the boundary caveat below.
`[VERIFIED: Go cover tool `-func` implementation semantics + 15-CONTEXT.md D-06]`

**Rounding boundary caveat (D-17 "planner must check").** `go tool cover -func` uses Go
`fmt`'s round-half-to-even on `%.1f`; D-06 specifies round-half-**up** to 2 dp. At a value like
`79.995%` the old gate could print `80.0%` (pass) while the new 2-dp value `79.99%` <80 (or
vice-versa) — a swing of up to ~0.05 pp in the enforced number. **The current backend raw total
is ~87% (STATE.md: measured 87.1% at Phase 09, re-confirmed at Phase 11.1-04; `internal/authgate`
added in Phase 14 at 91.0%).** That is ~7 pp of headroom — the cutover is LOW risk — **but the
number was not measurable in this research environment** (see Assumptions Log A1). The planner
MUST run `make test-integration` then compare `go tool cover -func=coverage.out | tail -1`
against `go run ./cmd/coverage-report --mode=total` on a real run before landing D-17; if the
margin is <0.05 pp, land D-17 + a coverage top-up in the same PR.

**`golang.org/x/tools/cover` not needed.** For reference its `ProfileBlock` struct is
`{StartLine, StartCol, EndLine, EndCol, NumStmt, Count}` and `ParseProfiles(fileName)` does
exactly this parse — but a `bufio.Scanner` + `strings`/`strconv` hand-parse is ~15 lines and
avoids the module dependency (D-06). The one edge to handle: a block line's file field can
contain `:` only as the last separator before the line/col range — split on the **last** `:`.

**`-coverpkg` interaction.** `test-integration` runs with `-coverpkg=$(COVER_PKGS)`, so files
from packages with **zero** test execution still appear in `coverage.out` with `count 0` rows —
they are in the denominator. The Go tool's parse naturally includes them, matching what
`coverage-gate` reports today. This is *why* one algorithm can feed both (D-17): identical input
file, identical row set.

### Mechanism 4 — Vitest `json-summary` reporter

**`coverage-summary.json` shape** (istanbul summary format, written by `@vitest/coverage-v8`):
```json
{
  "total": {
    "lines":      { "total": 1234, "covered": 1000, "skipped": 0, "pct": 81.04 },
    "statements": { "total": 1300, "covered": 1040, "skipped": 0, "pct": 80.0  },
    "functions":  { "total": 210,  "covered": 170,  "skipped": 0, "pct": 80.95 },
    "branches":   { "total": 540,  "covered": 400,  "skipped": 0, "pct": 74.07 }
  },
  "/abs/path/app/root.tsx": { "lines": {...}, ... }
}
```
D-10 reads `total.lines.pct`. `[CITED: istanbul-lib-report summary format; davelosert action reads these exact keys]`

**Reporter config.** Add `"json-summary"` to the `reporter` array in `web/vitest.config.ts`
(currently `["text"]` → `["text", "json-summary"]`). That alone is sufficient — the default
`coverage.reportsDirectory` is `./coverage` (relative to Vitest `root`, which is `web/`), so
the file lands at `web/coverage/coverage-summary.json`. `web/coverage/` is already gitignored
(`.gitignore:19`). `coverage.reportsDirectory` does **not** need to be set.
`[VERIFIED: web/vitest.config.ts + .gitignore:19; CITED: vitest.dev/config/coverage default]`

**D-15 ordering — CONFIRMED.** Vitest generates all configured coverage reports **before** it
evaluates `coverage.thresholds`:
> "Thresholds are enforced after report generation" — `packages/vitest/src/node/coverage.ts:223-224`

So a `frontend-test` run where a Vitest axis falls below 70 still writes a **complete**
`coverage/coverage-summary.json` to disk before `pnpm test` exits non-zero. D-15's "upload the
current frontend profile before the threshold check aborts" hand-off is sound. Corroborated by
`vitest-dev/vitest#3213` ("Vitest ignores coverage thresholds if reports fail to be created" —
i.e. report generation is strictly upstream of threshold enforcement).
`[CITED: deepwiki.com/vitest-dev/vitest/4.2-coverage-collection quoting packages/vitest/src/node/coverage.ts:223-224; vitest-dev/vitest#3213]`

**Belt-and-suspenders (Discretion).** Even with the ordering guaranteed, keep the upload step's
`if: ${{ !cancelled() }}` (D-15) and let `download-artifact` on the comment side tolerate
absence (D-18). A compile error in a test file would abort before *any* report is written.

### Mechanism 5 — GitHub Actions artifact hand-off

- **Versions already pinned in `full-pipeline.yml`:** `upload-artifact@043fb46d… # v7.0.1`,
  `download-artifact@3e5f45b2… # v8.0.1`. Both are current latest.
  `[VERIFIED: GitHub API releases/latest + full-pipeline.yml:174,220]`
- **`download-artifact` with a missing `name:`** → `Error: Unable to find an artifact with the
  name: <name>` and the **step fails** (exit non-zero). D-18 requires `continue-on-error: true`
  on each `download-artifact` step in `coverage-comment`, or a preceding existence check
  (e.g. `dawidd6/action-download-artifact` has `if_no_artifact_found: warn`, but that is a new
  third-party action — prefer `continue-on-error` + a `test -f` guard the Go tool respects).
  `[VERIFIED: community discussion #50004 + WebSearch corroboration]`
- **`retention-days: 1`** is valid — `full-pipeline.yml:178` already uses it for `scanned-image`.
  Minimum is 1, maximum is repo-configured (default 90). `[VERIFIED: full-pipeline.yml:178]`
- **Same-run cross-job download** needs no token or `run-id` input — default behavior downloads
  from the current run. `[CITED: download-artifact README]`
- **Naming:** artifact names cannot contain `:` (NTFS). Use e.g. `coverage-backend-pr`,
  `coverage-frontend-pr`. `[CITED: community discussion #50004]`
- **v4+ semantics:** an artifact name must be unique within a run (no more append-to-existing).
  Each job uploads once with a distinct name — fine here.

### Mechanism 6 — `if:` / `needs:` / `permissions` / `concurrency` semantics

- **`needs: [test, frontend-test]`** gates **ordering** (the job waits for both to finish) but,
  with a custom `if:` that omits `success()`, does **not** gate on their success. `[CITED: GitHub Actions docs — job `if` + `needs`]`
- **`if: ${{ !cancelled() && github.event_name == 'pull_request' }}`**:
  - a `needs:` job **failed** → `!cancelled()` is true → **job runs** (D-18 SC #4).
  - a `needs:` job **cancelled** (concurrency supersede on re-push) → `!cancelled()` false →
    **job skipped** (desired — the newer run posts).
  - correct idiom vs `always()`, which runs even on cancellation and would leave a stale comment
    race. `[VERIFIED: GitHub Actions status-check-functions semantics]`
- **`permissions` at job level:** `{ contents: read, pull-requests: write }` on the
  `coverage-comment` job **only**. Do **not** touch the workflow-level `permissions: contents: read`
  (`full-pipeline.yml:7-8`). `release`'s job-scoped `{ contents: write, packages: write }` is the
  established pattern to mirror. `[VERIFIED: full-pipeline.yml:7,140-141,195-197; Pitfall 24]`
- **Job-scoped `concurrency`:** `group: coverage-comment-${{ github.ref }}`,
  `cancel-in-progress: true` (D-08). Group name differs from the workflow-level group
  (`${{ github.workflow }}-${{ github.ref }}`, `full-pipeline.yml:10-12`) so they nest rather
  than collide. Deliberately redundant with the workflow-level PR cancel — kept as intent +
  defense (D-08). `[VERIFIED: full-pipeline.yml:10-12]`

### Mechanism 7 — `gosec` G304 carve-out in golangci-lint v2 (D-19)

Current `.golangci.yml` (`[VERIFIED: C:/CodeProjects/drop-tracker/.golangci.yml:1-36]`):
```yaml
version: "2"
run:
  timeout: 5m
linters:
  default: standard
  enable:
    - gosec
  exclusions:
    rules:
      - path: '_test\.go$'
        linters:
          - gosec
```

The Go tool reads coverage-file paths from argv/env and calls `os.Open`/`os.ReadFile` on them →
`gosec` **G304 (Potential file inclusion via variable)** fires on production code and reddens
the `lint` job. D-19 mirrors the existing `_test.go` rule. **Precise diff** — add one rule
entry under `linters.exclusions.rules`:
```yaml
      - path: '^cmd/coverage-report/'
        linters:
          - gosec
```

Notes:
- golangci-lint v2 matches `path` as a regexp against the file path relative to the module
  root, using `/` separators. `^cmd/coverage-report/` is the anchored analog of the existing
  rule's `_test\.go$`. A bare `cmd/coverage-report/` (unanchored) also works and matches the
  looser style of the existing entry — planner's call; anchored is safer against a future
  `internal/…/cmd/coverage-report/` false match.
- Scope stays minimal: only `gosec`, only this directory. Other linters (`errcheck`,
  `staticcheck`, `unused`, `govet`) still run on the tool.
- Alternative rejected by D-19: `linters.settings.gosec.excludes: [G304]` (repo-wide) or an
  inline `//nosec G304` (inconsistent with the established carve-out pattern).
- The new `cmd/coverage-report/` `main` package is picked up automatically by the existing
  `vet` and `lint` jobs (`go vet ./...`, `golangci-lint run`) — no workflow change needed for
  linting the tool, which is exactly why the carve-out is required.

`[VERIFIED: .golangci.yml:27-36; CITED: golangci-lint v2 exclusions.rules schema]`

### Mechanism 8 — `make coverage-report` / gate refactor (D-17)

Current `coverage-gate` (`[VERIFIED: C:/CodeProjects/drop-tracker/Makefile:81-97]`):
```make
coverage-gate:
	@if [ ! -s coverage.out ]; then \
		echo "coverage.out not found or empty -- run 'make test-integration' first" >&2; \
		exit 1; \
	fi
	@coverage=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{v=$$3; print substr(v, 1, length(v)-1)}'); \
	if [ -z "$$coverage" ]; then \
		echo "failed to parse aggregate coverage total from coverage.out" >&2; \
		exit 1; \
	fi; \
	echo "Backend coverage: $${coverage}% (required: $(COVERAGE_THRESHOLD_BACKEND)%)"; \
	if awk -v cov="$$coverage" -v thresh="$(COVERAGE_THRESHOLD_BACKEND)" 'BEGIN { exit !(cov + 0 >= thresh + 0) }'; then \
		echo "PASS"; \
	else \
		echo "FAIL: $${coverage}% is below the required $(COVERAGE_THRESHOLD_BACKEND)% threshold" >&2; \
		exit 1; \
	fi
```

**Refactor shape (recommended):**
- Add a phony target:
  ```make
  coverage-report:
  	@go run ./cmd/coverage-report --mode=total --profile=coverage.out
  ```
  The tool prints **only** the bare number (e.g. `87.14`) to **stdout**; every diagnostic goes
  to **stderr**. `--mode=comment` (or a `comment` subcommand) is the other mode used by the CI
  job, taking `--profile`, `--frontend-summary`, `--baseline-dir` and emitting the markdown.
- `coverage-gate` keeps its `-s coverage.out` guard, then:
  ```make
  	@coverage=$$(go run ./cmd/coverage-report --mode=total --profile=coverage.out); \
  	... existing echo + awk threshold comparison against $(COVERAGE_THRESHOLD_BACKEND) ...
  ```
- The literal `80` (`COVERAGE_THRESHOLD_BACKEND ?= 80`, `Makefile:40`) and the PASS/FAIL
  `echo`s **stay in the Makefile** — Phase 09's "one greppable threshold literal per side"
  posture is preserved; only the *measured* number now comes from the tool. `[CITED: 15-CONTEXT.md D-17]`

**Caveats for the planner:**
- `go run` recompiles the tool each call (~1–2 s cold, faster warm via the build cache).
  Acceptable — `coverage.out` cannot exist without the Go toolchain anyway (D-17).
- `go run` prints `go: downloading …` / build errors to **stderr**; ensure `coverage-gate`
  captures only stdout (`$$(...)` does) and that a build failure surfaces as a non-empty-stderr
  + empty-stdout → the existing `-z "$$coverage"` guard already catches that. Consider
  `set -o pipefail` semantics are N/A here (no pipe).
- `sqlc-check` / other targets unaffected.
- Local dev without network: `go run ./cmd/coverage-report` uses only the stdlib (D-06) so no
  module download — safe offline.
- The `.PHONY` line (`Makefile:1`) must gain `coverage-report`.

## Repo Reconnaissance — exact anchors for the 20 decisions

### `.github/workflows/full-pipeline.yml`

| Anchor | Current state | Decision(s) touching it |
|--------|---------------|-------------------------|
| `:3-5` | `on: { push:, pull_request: }` (both unfiltered) | `cache/save` steps guard on `github.event_name == 'push' && github.ref == 'refs/heads/main'` (D-13); `coverage-comment` job `if: ${{ !cancelled() && github.event_name == 'pull_request' }}` (D-18) |
| `:7-8` | `permissions: { contents: read }` (workflow-level) | **Do not change.** New job adds `pull-requests: write` **job-scoped only** (Pitfall 24) |
| `:10-12` | `concurrency: { group: ${{ github.workflow }}-${{ github.ref }}, cancel-in-progress: ${{ github.event_name == 'pull_request' }} }` | `coverage-comment` gets its own job-level `concurrency: { group: coverage-comment-${{ github.ref }}, cancel-in-progress: true }` — redundant by design (D-08) |
| `:43-61` (`test` job) | Steps: Checkout (v7.0.1) → setup-go (v7.0.0) → `run: make test-integration` (`:53-54`, writes `coverage.out`) → `run: make coverage-gate` (`:59-60`) | **Insert between `:54` and `:59`:** `upload-artifact` of `coverage.out` as `coverage-backend-pr`, `retention-days: 1`, `if: ${{ !cancelled() }}` (D-15). **Append after `:60`:** `cache/save` of `coverage.out` + sidecar under `coverage-baseline-main-backend-${{ github.sha }}`, `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`, `continue-on-error: true` (D-13/D-14/D-04). The sidecar is produced by a `make coverage-report`-style step (D-02/D-17). |
| `:89-119` (`frontend-test` job) | `defaults.run.working-directory: web`. Steps: Checkout → pnpm (v6.0.10) → setup-node (v7.0.0) → `pnpm install --frozen-lockfile` → `pnpm exec prettier --check` (`:116-117`) → `pnpm test` (`:118-119`, runs Vitest w/ thresholds) | **Insert between `:117` and `:118`?** No — the profile is written *by* `pnpm test`. **Insert after `:119`:** `upload-artifact` of `web/coverage/coverage-summary.json` as `coverage-frontend-pr`, `retention-days: 1`, `if: ${{ !cancelled() }}` (D-15 — the `if` covers the sub-70 exit). **Append:** `cache/save` guarded to push/main + `continue-on-error: true` (D-13/D-14/D-04). Mind `working-directory: web` for the profile path. |
| `:121-131` (`pr-title` job) | Existing job-scoped `permissions: { pull-requests: read }` + `if: github.event_name == 'pull_request'` | Pattern to mirror for `coverage-comment`'s job-scoped permissions + PR-only `if` |
| `:133-137` (`build-scan.needs`) | `needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]` | **Do not add `coverage-comment`.** It joins no `needs:` graph (report-only) |
| `:169-178` | `build-scan` → `release` hand-off: `docker save` → `upload-artifact@043fb46d… # v7.0.1` `name: scanned-image` `retention-days: 1`, guarded `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` | **The exact pattern for D-15's upload steps** — copy structure, drop the push/main guard (D-15 uploads on every run), keep `retention-days: 1` |
| `:180-197` (`release` job) | `concurrency: { group: release-${{ github.ref }}, cancel-in-progress: false }`; job-scoped `permissions: { contents: write, packages: write }` | Model for job-scoped `permissions` + job-scoped `concurrency` |
| `:219-225` | `release` downloads `scanned-image` via `download-artifact@3e5f45b2… # v8.0.1` | Pattern for `coverage-comment`'s two `download-artifact` steps — **add `continue-on-error: true`** (D-18, missing artifact fails the step) |

**New `coverage-comment` job skeleton** (structure only — not a plan):
```yaml
  coverage-comment:
    needs: [test, frontend-test]
    if: ${{ !cancelled() && github.event_name == 'pull_request' }}
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
      pull-requests: write
    concurrency:
      group: coverage-comment-${{ github.ref }}
      cancel-in-progress: true
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with: { go-version-file: go.mod }
      - id: restore-backend
        uses: actions/cache/restore@55cc8345863c7cc4c66a329aec7e433d2d1c52a9 # v6.1.0
        with:
          path: <backend profile + sidecar paths>
          key: coverage-baseline-main-backend-${{ github.sha }}
          restore-keys: coverage-baseline-main-backend-
      - id: restore-frontend
        uses: actions/cache/restore@55cc8345863c7cc4c66a329aec7e433d2d1c52a9 # v6.1.0
        with:
          path: <frontend summary + sidecar paths>
          key: coverage-baseline-main-frontend-${{ github.sha }}
          restore-keys: coverage-baseline-main-frontend-
      - uses: actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8.0.1
        continue-on-error: true
        with: { name: coverage-backend-pr, path: <dir> }
      - uses: actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8.0.1
        continue-on-error: true
        with: { name: coverage-frontend-pr, path: <dir> }
      - id: render
        run: go run ./cmd/coverage-report --mode=comment ... > comment.md   # writes $GITHUB_STEP_SUMMARY too
      - if: github.event.pull_request.head.repo.full_name == github.repository
        uses: marocchino/sticky-pull-request-comment@5770ad5eb8f42dd2c4f34da00c94c5381e49af88 # v3.0.5
        with:
          header: coverage-report
          path: comment.md
    # + job-level continue-on-error OR every step guarded (D-04 — planner's call)
```

### `Makefile`

| Anchor | Current | Change |
|--------|---------|--------|
| `:1` | `.PHONY: build run test test-short test-integration coverage-gate sqlc ...` | Add `coverage-report` |
| `:34` | `COVER_PKGS = $(shell go list ./... \| grep -vE '(^\|/)internal/db/sqlc$$' \| paste -sd, -)` | Extend the anchored alternation: `grep -vE '(^\|/)(internal/db/sqlc\|cmd/coverage-report)$$'` (D-07). Keep the doubled `$$`. |
| `:40` | `COVERAGE_THRESHOLD_BACKEND ?= 80` | **Unchanged** — literal stays (D-17) |
| `:71-73` | `test-integration: db-up` → `go test ./... -race -count=1 -coverprofile=coverage.out -coverpkg=$(COVER_PKGS)` | Unchanged; produces the profile the tool + gate + artifact all consume |
| `:81-97` | `coverage-gate` — hand-rolled parse via `go tool cover -func \| grep '^total:' \| awk` | Refactor per D-17 (§Mechanism 8): replace the `coverage=$$(...)` line with `coverage=$$(go run ./cmd/coverage-report --mode=total --profile=coverage.out)`; keep the `-s coverage.out` guard, the `echo`, the `awk` threshold compare, the PASS/FAIL exits |

### `web/vitest.config.ts`

| Anchor | Current | Change |
|--------|---------|--------|
| `:38` | `reporter: ["text"],` (comment: "writes nothing to disk") | `reporter: ["text", "json-summary"],` (D-06 review note) — update the comment to note the summary file is now written for the PR coverage comment |
| `:27-34` | `coverage: { enabled: true, provider: "v8", ... }` | No `reportsDirectory` needed — default `./coverage` → `web/coverage/coverage-summary.json`, already gitignored |
| `:56-61` | `thresholds: { statements: 70, branches: 70, functions: 70, lines: 70 }` | **Unchanged** — still the only frontend blocker. D-10 reads `total.lines.pct` for the comment; the gate keeps checking all four |

### `.golangci.yml`

See §Mechanism 7 — add one `exclusions.rules` entry (`path: '^cmd/coverage-report/'`, `linters: [gosec]`) after the existing `_test\.go$` rule (`:34-36`).

### `.gitignore`

`:7` `coverage.out`, `:8` `coverage.html`, `:19` `web/coverage/` — all already ignored. **No new entry needed.** `[VERIFIED: .gitignore]`

### `cmd/` layout

Currently only `cmd/server/` (`main.go`, `main_test.go`) `[VERIFIED: ls cmd/]`. New:
`cmd/coverage-report/main.go` + `cmd/coverage-report/main_test.go` + `cmd/coverage-report/testdata/`.
Module path: `github.com/danielrpof/drop-tracker/cmd/coverage-report` (`go.mod:1`, `go 1.26`).

## Don't Hand-Roll

| Problem | Don't build | Use instead | Why |
|---------|-------------|-------------|-----|
| Find-or-create-or-edit one PR comment by marker | `gh pr comment` + `gh api` list/PATCH loop with a self-managed `<!-- marker -->` | `marocchino/sticky-pull-request-comment@v3.0.5` `header:` | D-05 locked; the action is ~the reference implementation and handles pagination, marker embedding, and the "create if absent" race |
| Baseline persistence across runs | Orphan branch commits / gist / repo variable writes via `gh api` | `actions/cache/save` + `restore` | D-01 locked; zero repo noise, native prefix restore |
| Coverage-profile → percentage | Re-run `go test` on `main` in the PR job; scrape `go tool cover -func` text | Hand-parse `coverage.out` raw counts in the Go tool (D-06) | Source-tree-independent (works on an older baseline profile), 2-dp precision, one algorithm for comment + gate (D-17) |
| Frontend coverage number | Parse Vitest `text` reporter stdout; compute from `coverage-final.json` | `json-summary` reporter → `total.lines.pct` (D-10) | Built into `@vitest/coverage-v8`; stable documented schema; already 2 dp |
| Two independent coverage numbers staying in sync | Duplicate the parse in the Makefile and the Go tool | `make coverage-report` → `coverage-gate` consumes it (D-17) | Structural agreement, not coincidental |

**Key insight:** Every "hard" part of this phase (idempotent comments, cross-run state, profile
math) has a locked, verified off-the-shelf answer. The *only* hand-written code is the ~15-line
`coverage.out` parser and the markdown renderer — both small, both unit-tested, both deliberately
kept out of the product coverage number (D-07) so the tool cannot distort the metric it reports.

## Common Pitfalls

### Pitfall 1: Detecting "no baseline" via `cache-hit` (D-20)
**What goes wrong:** `cache-hit` is `'false'` on every PR (primary key `…-<sha>` never matches).
The job reports "delta unavailable" on 100% of PRs.
**Avoid:** Branch on `steps.<restore>.outputs.cache-matched-key != ''` AND the sidecar file
existing on disk. `[VERIFIED — §Mechanism 1]`

### Pitfall 2: `download-artifact` hard-fails when the profile is missing (D-18)
**What goes wrong:** A compile error in `test` means `coverage-backend-pr` was never uploaded;
`download-artifact` then errors `Unable to find an artifact…` and the step (and, without
`continue-on-error`, the job) fails — suppressing the comment exactly when SC #4 wants it.
**Avoid:** `continue-on-error: true` on each `download-artifact` step + the Go tool renders a
per-row `unavailable` when a profile file is absent/unparseable. `[VERIFIED — §Mechanism 5]`

### Pitfall 3: `pull-requests: write` at workflow scope (Pitfall 24)
**What goes wrong:** Every job — `build-scan`, `release`, third-party actions — gains PR write.
**Avoid:** Job-scoped `permissions: { contents: read, pull-requests: write }` on
`coverage-comment` only; leave `full-pipeline.yml:7-8` untouched. `[VERIFIED — §Mechanism 6]`

### Pitfall 4: Fork PR turns red over a reporting feature (Pitfall 25)
**What goes wrong:** Read-only `GITHUB_TOKEN` on a fork PR → marocchino 403 → step fails →
external contributor's PR is red.
**Avoid:** `if: github.event.pull_request.head.repo.full_name == github.repository` on the
marocchino step + `$GITHUB_STEP_SUMMARY` fallback + job `continue-on-error` (D-04). **Never**
`pull_request_target`. `[VERIFIED — §Mechanism 2]`

### Pitfall 5: `cache/save` reddening a green `main` (D-04)
**What goes wrong:** Re-run on the same `main` SHA → key already reserved. (Warning only in
practice, but cache-service 5xx *can* fail the step.)
**Avoid:** `continue-on-error: true` on both `cache/save` steps — **mandatory, not
discretionary** (D-04). `[VERIFIED — §Mechanism 1]`

### Pitfall 6: Saving a sub-threshold `main` as the baseline (D-14)
**What goes wrong:** `if: always()` on `cache/save` enshrines a broken `main`'s low coverage as
the number every PR diffs against.
**Avoid:** `cache/save` runs *after* the gate step with the default implicit `success()` (plus
the push/main guard) — a failed gate skips the save, leaving the last good baseline.

### Pitfall 7: D-17 rounding cutover moves the enforced number (D-17)
**What goes wrong:** Switching from `-func`'s 1-dp round-half-to-even to the tool's 2-dp
round-half-up can shift the gated value up to ~0.05 pp; if `main` is within 0.05 of 80 it goes
red on the cutover commit.
**Avoid:** Measure the real raw total on a live `make test-integration` run first (expected
~87%, ~7 pp headroom); if <0.05 pp margin, land D-17 + a coverage top-up in one PR.
`[ASSUMED — see A1]`

### Pitfall 8: `working-directory: web` and the frontend profile path
**What goes wrong:** `frontend-test` sets `defaults.run.working-directory: web`, so a
`cache/save`/`upload-artifact` `path:` of `coverage/coverage-summary.json` resolves under `web/`
for `run:` steps but **`actions/*` steps are not affected by `defaults.run`** — they need the
repo-relative `web/coverage/coverage-summary.json`.
**Avoid:** Use `web/coverage/coverage-summary.json` in every `uses:`-step `path:` input.

### Pitfall 9: `go run` build noise leaking into the gate number
**What goes wrong:** `coverage=$$(go run ./cmd/coverage-report …)` captures stdout; if the tool
prints anything but the number to stdout (a log line, a trailing newline is fine), the `awk`
compare breaks.
**Avoid:** Tool writes **only** the bare number to stdout; all diagnostics to stderr. Unit-test
this (`TestMain_ModeTotal_PrintsOnlyNumber`).

## Code Examples

### `coverage.out` hand-parse (D-06) — reference shape
```go
// Source: derived from Go cover profile grammar + golang.org/x/tools/cover ProfileBlock fields
// (the module itself is NOT imported — D-06)
func backendTotalPct(r io.Reader) (float64, error) {
	sc := bufio.NewScanner(r)
	var covered, total int64
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first { // "mode: atomic"
			first = false
			if !strings.HasPrefix(line, "mode:") {
				return 0, fmt.Errorf("missing mode line")
			}
			continue
		}
		if line == "" {
			continue
		}
		// <path>:<sL>.<sC>,<eL>.<eC> <numStmts> <count>
		sp := strings.LastIndex(line, " ")
		countStr := line[sp+1:]
		rest := line[:sp]
		sp2 := strings.LastIndex(rest, " ")
		numStmts, err := strconv.ParseInt(rest[sp2+1:], 10, 64)
		if err != nil {
			return 0, err
		}
		count, err := strconv.ParseInt(countStr, 10, 64)
		if err != nil {
			return 0, err
		}
		total += numStmts
		if count > 0 {
			covered += numStmts
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, fmt.Errorf("no statements in profile")
	}
	return float64(covered) / float64(total) * 100, nil
}
```

### Round half-up to 2 dp (D-06 / D-12)
```go
func round2(x float64) float64 { return math.Floor(x*100+0.5) / 100 }
```
(Confirm against what `coverage-gate` compares after D-17 — Discretion item.)

### marocchino step (D-05 / D-08)
```yaml
# Source: github.com/marocchino/sticky-pull-request-comment README
- if: github.event.pull_request.head.repo.full_name == github.repository
  uses: marocchino/sticky-pull-request-comment@5770ad5eb8f42dd2c4f34da00c94c5381e49af88 # v3.0.5
  with:
    header: coverage-report          # hidden marker — one comment per PR
    path: comment.md                 # rendered by the Go tool
    skip_unchanged: true             # optional: no edit-notification on a no-op re-push
```

### `actions/cache/save` on push→main (D-13 / D-04)
```yaml
# Source: github.com/actions/cache/blob/main/save/README.md
- if: github.event_name == 'push' && github.ref == 'refs/heads/main'
  continue-on-error: true            # D-04 — MANDATORY
  uses: actions/cache/save@55cc8345863c7cc4c66a329aec7e433d2d1c52a9 # v6.1.0
  with:
    path: |
      coverage.out
      baseline-metrics-backend.json
    key: coverage-baseline-main-backend-${{ github.sha }}
```

## State of the Art

| Old approach | Current approach | When changed | Impact here |
|--------------|------------------|--------------|-------------|
| `actions/cache` monolithic (restore+save in one step, save only in `post`) | Split `actions/cache/restore` + `actions/cache/save` sub-actions for granular control | actions/cache v3.2+ (2023) | D-01/D-13 rely on the split — restore on PR, save on push, independently |
| `upload-artifact@v3` / `download-artifact@v3` (append-to-existing, slow) | v4+ — immutable named artifacts, unique name per run, faster, but not cross-GHES | v4 GA 2024 | Repo already on v7/v8; each job uploads one uniquely-named artifact |
| Vitest `coverage.all` (measure every matched file) | `coverage.all` removed in Vitest 4; only imported files measured unless `include` globs force them | Vitest 4 (2025) | `web/vitest.config.ts:44` already sets `include: ["app/**/*.{ts,tsx}"]` — do not remove it |
| `go tool cover -func | awk` scrape | Parse raw `coverage.out` counts directly | D-06 decision | 2-dp precision; source-tree-independent baseline math |

**Deprecated / do not use:**
- `pull_request_target` for fork coverage comments — prohibited outright (CONTEXT, REQUIREMENTS "Out of Scope", Pitfall 25).
- `fgrosse/go-coverage-report`, `davelosert/vitest-coverage-report-action`, `k1LoW/octocov` — evaluated and rejected in D-05.
- Adding `json` (full `coverage-final.json`) to the Vitest reporters now — deferred to CICD-15 (D-06 review note).

## Validation Architecture

> `workflow.nyquist_validation: true` in `.planning/config.json` — section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework (backend tool) | Go stdlib `testing` (Go 1.26 per `go.mod:3`) |
| Config file | none — standard `go test` |
| Quick run command | `go test ./cmd/coverage-report/... -count=1` |
| Full suite command | `make test-integration` (unchanged; now also exercises the `coverage-report` path via `coverage-gate`) |
| Frontend | `pnpm test` (Vitest 4.1.10) — unchanged; the new `json-summary` reporter adds a file, no new tests needed for config |

### Phase Requirements → Test Map
| Req | Behavior | Test type | Automated command | File |
|-----|----------|-----------|-------------------|------|
| CICD-13 | Parse `coverage.out` → statement-weighted total, 2 dp, round-half-up | unit | `go test ./cmd/coverage-report/... -run TestBackendTotal` | ❌ Wave 0 `cmd/coverage-report/main_test.go` + `testdata/*.out` |
| CICD-13 | Read Vitest `coverage-summary.json` → `total.lines.pct` | unit | `go test ./cmd/coverage-report/... -run TestFrontendLines` | ❌ Wave 0 + `testdata/coverage-summary.json` |
| CICD-13 | Compute signed Δ vs sidecar; `±0.00pp` for zero; `—` + footer when sidecar absent (D-11/D-12) | unit | `go test ./cmd/coverage-report/... -run TestDelta` | ❌ Wave 0 |
| CICD-13 | Render the 2-row markdown table + `## Coverage` heading + gate column + ✅/⚠️ (D-09) | unit (golden file) | `go test ./cmd/coverage-report/... -run TestRender` | ❌ Wave 0 + `testdata/*.golden.md` |
| CICD-13 | Per-row `unavailable` when a profile file is missing/unparseable (D-18) | unit | `go test ./cmd/coverage-report/... -run TestRender_MissingProfile` | ❌ Wave 0 |
| CICD-13 | `--mode=total` prints ONLY the number to stdout (D-17 / Pitfall 9) | unit | `go test ./cmd/coverage-report/... -run TestModeTotal` | ❌ Wave 0 |
| CICD-14 | `coverage-gate` still passes consuming the tool's number (regression) | integration | `make coverage-gate` (after `make test-integration`) | existing target, refactored |
| CICD-14 | `COVER_PKGS` excludes `cmd/coverage-report` (D-07) | grep/unit | `go list -f '{{.ImportPath}}' ./... | ...` or a Makefile assertion | ❌ Wave 0 (mirror 09's `COVER_PKGS` guard pattern) |
| CICD-13 | Workflow wiring: one comment, edits in place, degrades on no baseline, non-blocking, baseline on merge | **manual / live** | scratch-branch PR against a throwaway branch — **not `main`** | not automatable — see below |

### Workflow wiring — what is verifiable how
- **`act` / workflow-lint:** `actionlint` catches YAML/expression errors, unknown `needs:`, bad
  `if:`. Run it. It does **not** exercise cache/artifact/comment behavior.
- **`act` (nektos/act):** cannot faithfully emulate `actions/cache` cross-run state, artifact
  hand-off, or PR-comment API — not a reliable substitute here.
- **Live verification** (mirrors Phase 09's approach — `[VERIFIED: STATE.md Phase 09 UAT]` used
  scratch branch `test/coverage-gate-ci-check`, never `main`): open a real PR from a scratch
  branch; confirm SC #1 (one comment), SC #2 (push twice → still one), SC #3 (delete the cache
  entry / first run → "delta unavailable"), SC #4 (drop coverage below gate → comment still
  posts, ⚠️, PR still mergeable), SC #5 (merge → next PR diffs against it, no recompute).
  Delete the scratch branch after.

### Sampling Rate
- **Per task commit:** `go test ./cmd/coverage-report/... -count=1` (<5 s)
- **Per wave merge:** `go vet ./... && golangci-lint run && make test && make coverage-gate` (Definition of Done, CLAUDE.md)
- **Phase gate:** full suite green + `actionlint` clean + the live scratch-branch PR walkthrough before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `cmd/coverage-report/main.go` — the tool (parse + delta + render + `--mode` flag)
- [ ] `cmd/coverage-report/main_test.go` — table tests per the map above
- [ ] `cmd/coverage-report/testdata/` — `sample.out` (a real `mode: atomic` profile slice), `coverage-summary.json`, `baseline-metrics-*.json`, `*.golden.md`
- [ ] `COVER_PKGS`-excludes-`cmd/coverage-report` assertion (mirror `09`'s anchored-regex guard)
- [ ] `actionlint` invocation (pre-commit hook or a CI step — Discretion; not strictly required by CONTEXT)
- Framework install: none — Go stdlib `testing` already in use.

## Security Domain

> `security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high`.

### Applicable ASVS categories

| ASVS Category | Applies | Standard control for this phase |
|---------------|---------|--------------------------------|
| V1 Architecture | yes | Report-only job in no `needs:` graph; least-privilege token; documented fork degradation |
| V5 Input Validation | yes | The Go tool parses `coverage.out` / `coverage-summary.json` / sidecar — all CI-produced, but the tool must fail closed (render `unavailable`, never panic, never emit attacker-controlled markdown unescaped into the comment) |
| V4 Access Control | yes | `pull-requests: write` **job-scoped only**; `GITHUB_TOKEN` not exported to any `run:` that executes downloaded code |
| V6 Cryptography | no | No secrets, no signing in this phase |
| V14 Config | yes | All third-party actions SHA-pinned + version comment (CLAUDE.md); no new secret/env var |

### Threat patterns for this stack

| Pattern | STRIDE | Mitigation |
|---------|--------|------------|
| `pull_request_target` + checkout of PR head runs untrusted code with write token + secrets | Elevation of Privilege / Tampering | **Prohibited outright** (CONTEXT, REQUIREMENTS Out-of-Scope, Pitfall 25). Job runs on `pull_request` only. |
| Over-broad `pull-requests: write` at workflow scope | Elevation of Privilege | Job-scoped permissions; workflow default `contents: read` untouched (Pitfall 24) |
| Malicious PR poisons the coverage baseline | Tampering | Not possible — PR-created caches are scoped to that PR; `cache/save` runs only on `push`→`refs/heads/main` (§Mechanism 1) |
| Attacker-controlled content in a coverage file → markdown/HTML injection in the PR comment | Tampering / (limited) XSS in GH's renderer | Tool emits only numbers + fixed strings into the table; never interpolates a file path or arbitrary profile text into the comment body. File paths, if ever shown (CICD-15, not now), must be escaped. |
| Coverage numbers / SHAs / timestamps in the comment or job summary leak sensitive data | Information Disclosure | None of these are sensitive. No DSN / passphrase / webhook is in scope for this job (contrast the `release`/`deploy` jobs). Confirm no `env:` on the job pulls a secret. |
| Base-branch cache readable by anyone who can open a PR | Information Disclosure | Accepted — baseline holds only coverage %, SHA, timestamp, and public source profiles (§Mechanism 1) |
| Supply-chain: a compromised `marocchino` tag | Tampering | SHA-pinned to `5770ad5…` (immutable); version comment for auditability |

No `high`-severity items identified — this is a read-only CI reporting job with a job-scoped
token and no secret exposure. `/gsd-secure-phase 15` should register the above and can likely
short-circuit at L1 grep-depth (as Phase 13/14 did).

## Assumptions Log

| # | Claim | Section | Risk if wrong |
|---|-------|---------|---------------|
| A1 | Current backend **raw** coverage total is ~87% (well above the 80 floor), so the D-17 rounding cutover is safe. Based on STATE.md's "measured 87.1%" (Phase 09 UAT) + "coverage-gate confirmed 87.1%" (Phase 11.1-04) + `internal/authgate` "91.0% coverage" (quick 260901-rl9). **Could not run `make test-integration` in this environment** (needs Postgres + Docker; this Windows box has a documented `-race`/cgo break). | §Mechanism 3, §Mechanism 8, Pitfall 7 | If Phase 14 dropped backend coverage near 80, the D-17 cutover commit reddens `main`. Mitigation is cheap and already in D-17 ("planner must check"): measure on a real run, co-land a top-up if margin <0.05 pp. |
| A2 | `actions/cache/save` against an already-reserved key emits a warning and exits 0 (does not fail the step). Based on `actions/toolkit` issues #1642/#537/#1946 describing the catch-and-warn behavior in `@actions/cache`; the dedicated `save` sub-action wraps the same library. | §Mechanism 1, Pitfall 5 | Low — D-04's mandatory `continue-on-error: true` makes the exact failure mode moot. |
| A3 | Vitest `json-summary` writes `web/coverage/coverage-summary.json` (default `reportsDirectory: ./coverage` relative to `root: web/`); no `reportsDirectory` override needed. Based on Vitest docs default + the config's lack of a `root` override (Vitest infers `web/` from the config file location). | §Mechanism 4, §Repo Recon | Low — if the path differs, `actionlint`/the first live run surfaces it immediately; fix is a one-line `path:` change. |
| A4 | `coverage-summary.json` uses the istanbul summary schema with `total.lines.{total,covered,skipped,pct}`. Based on istanbul-lib-report's format and the `davelosert` action reading these exact keys — not opened from a real file this session. | §Mechanism 4 | Low — generate one real file during Wave 0 and pin it as `testdata/`; the golden test locks the schema. |

## Open Questions

1. **Per-language sidecar vs. one merged sidecar (D-02).**
   - Known: D-13 forbids a dedicated baseline job; each of `test` / `frontend-test` writes its
     own cache entry.
   - Unclear: whether to keep two sidecars (`baseline-metrics-backend.json`,
     `-frontend.json`) merged by the tool at read time, or have one job write a combined file.
   - Recommendation: two per-language sidecars, each cached alongside its language's profile;
     the Go tool reads whichever are present (a missing one → that row's Δ is `—`).

2. **`make coverage-report` output when `coverage.out` is absent.**
   - The tool needs a defined exit for "no profile" — the gate's existing `-s coverage.out`
     guard covers `coverage-gate`, but `make coverage-report` called standalone should print a
     clear stderr message + non-zero exit.
   - Recommendation: mirror the gate's guard inside the target.

3. **Job-level `continue-on-error` vs. per-step guards for `coverage-comment` (D-04, Discretion).**
   - Job-level `continue-on-error: true` is simplest and cannot leave a failure uncaught.
   - Per-step is more granular but risks missing a mode.
   - Recommendation: job-level `continue-on-error: true` **plus** `continue-on-error` on the two
     `download-artifact` steps (so later steps still run when a profile is missing — job-level
     alone would let the job "succeed" but skip the render).

4. **Does the Go tool or a `run:` step own the `$GITHUB_STEP_SUMMARY` write (D-04, Discretion)?**
   - Recommendation: the tool always writes the table to the path in `$GITHUB_STEP_SUMMARY`
     (env var present in every step) in addition to `comment.md`, so the summary is populated
     whether or not marocchino runs. Keeps one renderer, one format.

## Sources

### Primary (HIGH confidence)
- GitHub API `repos/marocchino/sticky-pull-request-comment/git/ref/tags/v3.0.5` → SHA `5770ad5eb8f42dd2c4f34da00c94c5381e49af88` (retrieved 2026-09-02)
- GitHub API `repos/actions/cache` tags + `releases/latest` → v6.1.0 `55cc8345863c7cc4c66a329aec7e433d2d1c52a9` (2026-09-02)
- GitHub API `repos/actions/upload-artifact` / `download-artifact` `releases/latest` → v7.0.1 / v8.0.1, SHAs cross-checked against `full-pipeline.yml`
- `deepwiki.com/vitest-dev/vitest/4.2-coverage-collection` quoting `packages/vitest/src/node/coverage.ts:223-224` — "Thresholds are enforced after report generation"
- `vitest-dev/vitest#3213` — report generation precedes threshold enforcement
- Repo files read this session: `.github/workflows/full-pipeline.yml`, `Makefile`, `web/vitest.config.ts`, `.golangci.yml`, `.gitignore`, `go.mod`, `web/package.json`, `coverage.out`, `.pre-commit-config.yaml`, `.planning/{ROADMAP,REQUIREMENTS,STATE,config.json}`, `15-CONTEXT.md`, `15-DISCUSSION-LOG.md`, research `ARCHITECTURE.md`/`STACK.md`/`PITFALLS.md` (Feature 3 / items 24-27)

### Secondary (MEDIUM confidence)
- `github.com/actions/cache/blob/main/{restore,save}/README.md` — inputs/outputs, `cache-matched-key` semantics
- `docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching` — cache scope (PR can restore base/default branch), 7-day eviction, 10 GB cap
- `actions/toolkit#1642`, `#537`, `#1946` — `saveCache` reserve-failure is caught → warning, not step failure
- GitHub community discussion `#50004` — `download-artifact` missing-name error, artifact naming constraints

### Tertiary (LOW confidence)
- Backend raw coverage total ~87% — inferred from STATE.md history, not measured this session (A1)

## Metadata

**Confidence breakdown:**
- External action versions / SHAs: HIGH — resolved via GitHub API this session
- Cache save/restore + `cache-matched-key` semantics (D-20): HIGH — primary docs + toolkit issues + repo-pattern corroboration
- Vitest report-before-threshold ordering (D-15): HIGH — source-line citation
- `coverage.out` grammar + D-06 formula: HIGH — Go cover tool semantics, profile mode verified in-repo
- marocchino inputs / fork behavior: MEDIUM-HIGH — README + Pitfall 25
- Backend raw coverage margin (D-17 cutover): LOW — unmeasured here (A1); risk assessed LOW from history
- `.golangci.yml` v2 exclusion shape (D-19): MEDIUM-HIGH — mirrors an existing working rule in the same file

**Research date:** 2026-09-02
**Valid until:** ~2026-10-02 for action versions (pin SHAs, so drift is cosmetic); Vitest/Go behavior is stable across the pinned versions.
