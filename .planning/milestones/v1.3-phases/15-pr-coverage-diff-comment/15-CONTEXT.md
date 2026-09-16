# Phase 15: PR Coverage-Diff Comment - Context

**Gathered:** 2026-09-02
**Refined:** 2026-09-02 (grill-with-docs review — D-02/03/04/06/08/12/16 amended, D-17–D-20 added; see "Grill Refinements" below)
**Status:** Ready for planning

<domain>
## Phase Boundary

A CI-only, report-only feature: every pull request from a **same-repo branch** carries exactly
**one** sticky comment on `full-pipeline.yml` showing backend (Go) and frontend (Vitest) total
coverage plus each one's delta versus the `main`-branch baseline. The comment is always
report-only — it joins no release-path `needs:` graph and can never turn a PR red or block a
merge; the pre-existing `make coverage-gate` (80% backend) and Vitest `coverage.thresholds`
(70% frontend) stay the only coverage blockers.

**Delivered as:**
- A new `coverage-comment` job in `.github/workflows/full-pipeline.yml`, `if: ${{ !cancelled() && github.event_name == 'pull_request' }}` (D-18 — runs on an upstream gate failure, skips on cancel), `needs: [test, frontend-test]`, `permissions: { contents: read, pull-requests: write }` (job-scoped only), default shallow checkout (D-16 superseded), its own `concurrency` group with `cancel-in-progress: true` (D-08 — redundant-by-design).
- A small **Go tool** at `cmd/coverage-report/` that reads the current coverage profiles + the cached baseline **metrics sidecar** (D-02), computes the deltas, and renders one markdown table. Backend total is hand-parsed from `coverage.out`'s raw statement counts (D-06), not scraped from `go tool cover -func`. Unit-tested. **Excluded** from the Makefile `COVER_PKGS` list so it never moves the 80% product-coverage number; **carved out of `gosec`** in `.golangci.yml` (D-19).
- Baseline publication: `cache/save` steps added to the existing `test` and `frontend-test` jobs, guarded to `push` on `refs/heads/main` and to a passing coverage gate, carrying `continue-on-error: true` (D-04). Each saves its language's full profile **plus** the precomputed-metrics sidecar.
- PR-side current-coverage hand-off: `test` and `frontend-test` upload `coverage.out` / `coverage-summary.json` as short-retention artifacts the `coverage-comment` job downloads — upload step ordered **before** the gate step, `if: ${{ !cancelled() }}` (D-15/D-18).
- `web/vitest.config.ts` gains the **`json-summary`** coverage reporter (not `json` — the full `coverage-final.json` is large and only `davelosert`'s action, rejected in D-05, needed it; add `json` when CICD-15's frontend diff-cover needs it). `web/coverage/` is already gitignored.
- A `make coverage-report` target backed by the Go tool that prints the backend total; `make coverage-gate` is refactored to consume it (D-17), so the comment and the gate share one algorithm and can never disagree.

**In scope:** CICD-13, CICD-14 (see `.planning/REQUIREMENTS.md`).

**Out of scope:**
- **CICD-15** — patch/diff-level (uncovered-new-lines) coverage. Explicitly a Future requirement; the comment shows file/total deltas only.
- Fork-PR comment posting. Fork PRs get a read-only `GITHUB_TOKEN`; the feature degrades to the job summary and never blocks the PR. `pull_request_target` is **prohibited outright**.
- Any application-code (`internal/`, `cmd/server/`) change. The only Go added is the CI helper at `cmd/coverage-report/`. (Tooling files *are* touched: `Makefile` `coverage-gate`/`COVER_PKGS`, `.golangci.yml`, `web/vitest.config.ts`, `full-pipeline.yml` — all per the decisions below.)
- Per-package / per-file coverage tables in the comment body.

</domain>

<decisions>
## Implementation Decisions

### Baseline storage
- **D-01:** Baseline is stored in the **GitHub Actions cache** under a `<sha>`-suffixed key with the stable prefix `coverage-baseline-main-` (per language), restored on PR runs via `restore-keys` prefix match. Accept the ~7-day idle-eviction trade-off — an idle `main` loses the baseline and that PR run shows absolute numbers only (D-06 handles the copy). Rejected: 90-day retention artifact, orphan `coverage-baseline` branch. — **Reversibility:** reversible — swapping to an artifact or branch later is a self-contained workflow edit; no consumer outside `full-pipeline.yml`.
- **D-02 (amended, grill review):** The cache entry holds **both** the full coverage profiles — `coverage.out` (Go) and `coverage-summary.json` (Vitest) — **and a small precomputed-metrics sidecar** (`baseline-metrics.json`: backend total %, frontend `lines` %, baseline commit SHA, UTC timestamp). **The delta math reads the sidecar, not the profiles.** The full profiles are cached only to leave the door open for a future changed-files view (CICD-15) without re-keying. *Why the sidecar:* `go tool cover -func` — and any Go profile-to-percentage step — resolves the profile against the **checked-out source tree**. A PR-head checkout cannot reliably re-derive the total for an *older baseline* profile (renamed/moved/deleted files error or misattribute). Precomputing the percentage on the `main` side, where the tree matches the profile, removes that dependency entirely. The sidecar is written by the same `make coverage-report` step that D-17 introduces, so the baseline number and the current number come from one algorithm.
- **D-03 (amended, grill review):** A restored baseline that is **stale** (a prefix `restore-keys` hit from an older `main` SHA) is diffed against **silently** — no "baseline N commits behind" caveat. BUT the footer carries `baseline: main@<short-sha>` (from the D-02 sidecar) as **provenance** — the reference point the delta is measured against. Distinguish provenance (kept: a reader must be able to see *which* `main` the −0.30pp is versus) from a staleness annotation (still rejected: no commit-count, no "drift may apply" editorializing).
- **D-04 (amended, grill review):** **Two "never red" surfaces, not one.** (1) The entire `coverage-comment` job is best-effort — every failure mode (cache API error, comment POST 5xx, malformed/missing profile, fork read-only token) is swallowed and the job exits 0. (2) The new `cache/save` baseline-publish steps inside `test` / `frontend-test` (D-13) carry **`continue-on-error: true`** — a baseline-write failure (key already exists on a re-run, cache service 5xx, quota) must never turn a green `main` build red over a reporting-baseline write. A silently broken reporting mechanism is acceptable; a red check on a reporting feature is not. (`continue-on-error` at job level for the comment job, or every step guarded — planner's call; per-step on the two `cache/save` steps is mandatory.)

### One comment, not two
- **D-05:** Exactly one comment (SC #1/#2) is achieved with a **custom rendering step + `marocchino/sticky-pull-request-comment`** (SHA-pinned, v3.0.5), **not** the two ready-made single-language actions (`fgrosse/go-coverage-report` + `davelosert/vitest-coverage-report-action`, which produce two comments) and **not** `k1LoW/octocov` (all-in-one, introduces a datastore concept that overlaps D-01). This preserves the repo's Phase-09 posture: coverage stays two independent single-language mechanisms with literal thresholds, no third-party coverage *gating* action. — **Reversibility:** costly — undoing means re-modelling the comment as two actions and amending ROADMAP SC #1 ("exactly one comment").
- **D-06 (amended, grill review):** The renderer is a **Go tool** at `cmd/coverage-report/` — a small, unit-testable `main` package (fits the Go portfolio framing; emits the markdown). It computes the **backend total by hand-parsing `coverage.out`'s raw statement counts** — `mode:` line then `file:startLine.col,endLine.col numStmts count` rows; total % = `sum(numStmts where count>0) / sum(numStmts) * 100`, rounded half-up to 2 dp. It does **not** scrape `go tool cover -func` text. *Why:* (a) source-tree-independent, so the same code computes the baseline number on `main` and the current number on the PR head (D-02); (b) `go tool cover -func` emits only 1 decimal (`82.4%`), which cannot express D-12's 2-dp display; (c) one algorithm can then feed both the comment and the gate (D-17). The frontend side reads `coverage-summary.json`'s `total.lines.pct` directly (self-contained, already effectively 2 dp). No new module dependency — the profile format is ~15 lines of hand-parsing; `golang.org/x/tools/cover` is not pulled in. Rejected: an inline bash `run:` step, a Node/JS script, scraping `-func` output.
- **D-07:** `cmd/coverage-report/` is **excluded from the Makefile `COVER_PKGS`** list (extend the existing anchored `grep -vE` that already drops `internal/db/sqlc`), so this CI helper never enters the 80% product-coverage denominator. Its own `go test ./cmd/coverage-report/...` still runs in CI. Rejected: leaving it in the denominator "because all our Go code counts".
- **D-08 (note added, grill review):** Stickiness is `marocchino/sticky-pull-request-comment` with a fixed `header:` and its hidden HTML marker — find prior comment, edit in place, create only if absent. `concurrency: { group: coverage-comment-${{ github.ref }}, cancel-in-progress: true }` so a rapid re-push supersedes an in-flight comment. *Note:* this job-scoped `concurrency` is deliberately **redundant** with the workflow-level `cancel-in-progress: ${{ github.event_name == 'pull_request' }}` — kept as defense if that top-level gating is ever narrowed, and as an explicit statement of intent at the job. No behavior change today. Rejected: hand-rolled `gh pr comment` + self-managed marker search.

### Comment content
- **D-09:** Body is a **visible `## Coverage` heading** above a **2-row markdown table** (Backend / Frontend) with columns: **Coverage %**, **Δ vs main** (signed, percentage points), **Gate** (`80%` / `70%`), and a **status mark** (✅ / ⚠️) versus that gate. Rejected: coverage + Δ only (no gate column); table + per-file `<details>` (edges into deferred CICD-15).
- **D-10:** The **Frontend row uses the Vitest `lines` axis only** for its headline % and Δ; the Gate column still reads `70%`. The other three axes (statements / branches / functions) are not shown. Rejected: all four axes; lines-headline with the rest in a `<details>`.
- **D-11:** **No-baseline state** (SC #3 — first run after ship, or evicted cache; detected per D-20): the Δ column shows `—` for both rows and a one-line footer reads roughly _"Delta unavailable — no `main` baseline cached yet (first run or evicted). Absolute coverage shown."_ In this state the footer's `baseline: main@…` provenance line (D-12) is omitted — there is no baseline to name. Rejected: bare `—` with no explanation; inline `n/a (no baseline)` per row.
- **D-12 (amended, grill review):** Numeric formatting: **2 decimal places** for both rows (`82.41%`, `-0.12pp`) — genuinely achievable now that D-06 hand-parses raw counts rather than scraping 1-dp `-func` output. An unchanged value renders as **`±0.00pp`** so "no change" is visually distinct from "no baseline" (`—`). The **footer line** carries: the PR head short-SHA the current numbers came from, `baseline: main@<short-sha>` (D-03), a UTC timestamp, and — when D-18 applies — a one-line note if an upstream test job was red.

### Baseline publish trigger
- **D-13:** Baseline is published by **`cache/save` steps inside the existing `test` and `frontend-test` jobs**, each guarded `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`. No dedicated `coverage-baseline` job. Each job saves its own language's profile under its own prefixed key.
- **D-14:** The `cache/save` step runs **only after that job's coverage gate passed** (`test`'s `make coverage-gate` / `frontend-test`'s Vitest thresholds). A sub-threshold `main` is a broken state and must not be enshrined as the baseline — a red `main` leaves the previous good baseline in place. Rejected: `if: always()` "save whatever main actually is".
- **D-15 (amended, grill review):** The PR's **current** coverage profiles reach the `coverage-comment` job as **short-retention (~1-day) artifacts uploaded on every run** of `test` / `frontend-test` (mirrors how `build-scan` → `release` already hand off the scanned image). Rejected: guarding the upload to `pull_request` only (marginal churn saving, less consistent). The `coverage-comment` job never re-runs any test suite. **Ordering constraint (D-18):** the upload step sits **between** the test/build step and the gate/threshold step, and carries `if: ${{ !cancelled() }}` — so a coverage-*gate* failure (backend 79%, or a Vitest axis <70) still hands off a complete profile. For the frontend, verify Vitest has written `coverage/coverage-summary.json` before the threshold check aborts `pnpm test` (report generation precedes threshold evaluation — confirm during planning).
- **D-16 (superseded, grill review):** The `coverage-comment` job uses the **default shallow checkout** (`fetch-depth: 1`), enough for `go run ./cmd/coverage-report`. Nothing in scope needs git history: the footer's PR head SHA comes from `${{ github.event.pull_request.head.sha }}` (event context, no git), the timestamp from the runner clock, the deltas from downloaded artifacts + the D-02 cache sidecar. `fetch-depth: 0` was rejected — a full unshallow on every PR re-push is a recurring clone cost on a job whose point is fast feedback, and the `gitleaks`/`release` precedent doesn't transfer (those genuinely walk history: secret scanning across commits, `svu` tag walk). Revisit `fetch-depth: 0` only when CICD-15's changed-files view needs `git merge-base` — a reversible one-line change. Rejected: no checkout at all (would require shipping the tool as a prebuilt binary artifact — extra artifact, extra failure mode, for a ~30s saving that's noise next to the `needs:` wait).

### Grill Refinements (2026-09-02) — added decisions

- **D-17: One coverage algorithm for both the comment and the gate.** A new `make coverage-report` target runs the D-06 Go tool in "print the backend total only" mode. `make coverage-gate` is **refactored to consume that number** instead of its own `go tool cover -func=coverage.out | grep '^total:' | awk` pipeline. This makes "the comment and the 80% gate can never disagree" structural rather than coincidental — one parse, one rounding rule, one number. The literal `80` threshold and the pass/fail `echo`s stay in the Makefile (Phase 09's "one greppable threshold literal per side" posture is preserved — only the *measured* number now comes from the tool, not the threshold). `coverage-gate` gains an effective dependency on `go run ./cmd/coverage-report` (Go toolchain) — negligible, since `coverage.out` cannot exist without the Go toolchain in the first place. CONTEXT §"Existing Code Insights" already listed "factor it into a shared `make coverage-report` target" as an option; this decision takes it. — **Reversibility:** moderate — reverting means restoring the awk pipeline in `coverage-gate` and accepting possible boundary disagreement with the comment. — **Planner must check:** switching the gate from `go tool cover -func`'s 1-dp rounded `total:` to the tool's 2-dp raw computation can move the enforced number by up to ~0.05pp. Confirm the current *raw* backend total has margin above 80 before shipping D-17 (`go tool cover -func` currently shows the rounded value — compute the exact `covered/total` ratio); if it's within 0.05 of the floor, land D-17 and any coverage top-up in the same PR so `main` never goes red on the cutover.
- **D-18: The comment still posts when a PR fails a coverage gate.** SC #4's headline case — tests pass, backend coverage is 79% — currently fails the `test` job, and `coverage-comment`'s default skip-on-upstream-failure would suppress the comment exactly when it is most useful. Fix: (a) current-profile upload ordered before the gate step, `if: ${{ !cancelled() }}` (D-15); (b) `coverage-comment` runs `if: ${{ !cancelled() && github.event_name == 'pull_request' }}` — not the implicit `success()`; (c) `download-artifact` steps tolerate a missing artifact (`continue-on-error` or an existence check), the Go tool renders a per-row `unavailable` when a profile is absent or unparseable, and the footer notes when an upstream job was red. Net effect: a *gate* failure → real number + ⚠️ shown; a *suite/compile* failure → that row degrades to `unavailable` (D-04 swallow). Rejected: narrowing SC #4 in the plan to "above-gate drops only".
- **D-19: `cmd/coverage-report` gets a scoped lint carve-out.** The tool reads coverage-file paths from argv/env and opens them → `gosec` G304 (file inclusion via variable) fires on production code, reddening the `lint` job. Add a `path: '^cmd/coverage-report/'` exclusion for `gosec` in `.golangci.yml`, mirroring the existing `_test.go` gosec carve-out (same rationale: the flagged pattern is not a real path-traversal surface — the paths are CI-controlled workflow inputs). Rejected: a single inline `//nosec G304` (less consistent with the established carve-out pattern; more likely to be copied around later).
- **D-20: No-baseline detection keys off `cache-matched-key`, not `cache-hit`.** `actions/cache/restore` with `restore-keys:` prefix matching sets `cache-hit: 'false'` on *every* prefix match — `cache-hit` is only `true` on an exact primary-key hit, which never happens here (the key is `<prefix>-<sha>` and the PR never knows `main`'s latest SHA). The job must treat "baseline present" as `steps.<restore>.outputs.cache-matched-key != ''` **and** the sidecar file existing on disk; anything else is the D-11 no-baseline state. A plan that checks `cache-hit` will report "no baseline" on every single PR.

### Locked upstream (not re-discussed — carried from ROADMAP / REQUIREMENTS / research)
- Report-only; `coverage-comment` in **no** release-path `needs:` graph — `needs: [test, frontend-test]` and nothing else; nothing `needs:` it.
- `pull-requests: write` scoped to the `coverage-comment` job only — never workflow-wide (the workflow default stays `contents: read`).
- Same-repo branches only; fork PRs degrade to the job summary; `pull_request_target` prohibited.
- Merge to `main` publishes that run's coverage as the baseline; **no PR run recomputes `main`'s coverage** (SC #5).
- Reuse the coverage the `test` / `frontend-test` jobs already produce — no second test invocation anywhere.
- All third-party actions SHA-pinned with a version comment (CLAUDE.md).

### Claude's Discretion
- Exact cache key shape and whether backend/frontend share one key or use two prefixes (D-13 implies two; planner confirms).
- Exact `make coverage-report` internals — a thin target that shells the Go tool vs. the tool reading a `--mode=total` flag (D-06/D-17 fix *that* it's the Go tool and *that* the gate consumes it; the flag/target shape is open).
- The precise footer wording, heading text, and the ✅/⚠️ glyph choice.
- Artifact names and retention days (D-15 says ~1 day).
- Whether the comment job's `continue-on-error` sits at job level or per-step (D-04) — the two `cache/save` steps' `continue-on-error: true` is *not* discretionary.
- Whether the Go tool itself or a separate step owns the "post to `$GITHUB_STEP_SUMMARY`" fork/degradation path.
- The exact hand-parse rounding rule for the backend total (D-06 says half-up to 2 dp; planner confirms it matches whatever `coverage-gate` compares after D-17).

### Decided by grill review (no longer discretionary)
- Baseline delta source: the D-02 cache **sidecar**, not a re-parse of the cached profile.
- Comment-job checkout depth: **default shallow** (D-16 superseded); tool delivered via `go run ./cmd/coverage-report` off that checkout.
- Comment-job `if:`: `${{ !cancelled() && github.event_name == 'pull_request' }}` (D-18), not implicit `success()`.
- Vitest reporters: add **`json-summary` only** — not `json` (see below).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/ROADMAP.md` §"Phase 15: PR Coverage-Diff Comment" — goal, 5 success criteria, the `needs:` / permissions / fork / `pull_request_target` constraints in _Notes_
- `.planning/REQUIREMENTS.md` — CICD-13, CICD-14 (locked text); CICD-15 (deferred — patch/diff coverage, explicitly out of scope)

### v1.3 research (this milestone)
- `.planning/research/PITFALLS.md` items **24–27** — the checklist this phase is judged against: `GITHUB_TOKEN` PR-comment perms (24), fork-PR degradation + `pull_request_target` prohibition (25), comment spam / sticky-upsert (26), baseline fetch on first run / shallow checkout (27). Also the "Looks Done But Isn't" §"Coverage comment" line and the Pitfall-to-Phase mapping rows 24–27.
- `.planning/research/ARCHITECTURE.md` §"Feature 3 — Coverage-diff PR comment" — producing-job table, `needs: [test, frontend-test]` rationale, the guards YAML block, cache-baseline mechanism, NEW-vs-MODIFIED table. *Grill-review divergence:* ARCHITECTURE's "default behavior (skip if an upstream job fails) is correct" is **overridden by D-18** — a coverage-*gate* failure is not a test failure, and the comment must still post for it.
- `.planning/research/STACK.md` §"Feature 3 — PR coverage-diff comment (CICD-13)" — SHA pins: `marocchino/sticky-pull-request-comment` v3.0.5 (`5770ad5eb8f42dd2c4f34da00c94c5381e49af88`); the custom-script + sticky-comment route is the documented alternative this phase adopts; fork-PR / `pull_request_target` warning. *Grill-review divergence:* STACK's "`json-summary` + `json`" pair was for the rejected `davelosert` action — this phase adds **`json-summary` only** (D-06 review note).
- `.planning/research/FEATURES.md` §"Feature 3" — table-stakes list (total % + signed Δ, backend + frontend), single sticky comment, anti-features (hard merge gate, whole-repo per-package table), reuse of existing coverage runs

### Existing code this phase modifies or mirrors
- `.github/workflows/full-pipeline.yml` — `test` job (`make test-integration` → `coverage.out`, then `make coverage-gate`), `frontend-test` job (`pnpm test` → Vitest coverage/thresholds), `release` job's job-scoped `permissions` pattern, `build-scan` → `release` artifact hand-off pattern (`upload-artifact` v7 / `download-artifact` v8, pinned), the top-level `permissions: contents: read` and `concurrency` block
- `Makefile` — `COVER_PKGS` (anchored `go list ... | grep -vE '(^|/)internal/db/sqlc$$'` — extend to also drop `cmd/coverage-report`), `coverage-gate` target (**refactored per D-17** to consume `make coverage-report`'s number instead of its own `go tool cover -func | grep '^total:' | awk` pipeline — the literal `80` comparison stays), `COVERAGE_THRESHOLD_BACKEND ?= 80`
- `web/vitest.config.ts` — `coverage.reporter` (currently `["text"]`, "writes nothing to disk" posture — add **`json-summary`** only, not `json`), `coverage.thresholds` (70 on all four axes), `coverage.include` / `coverage.exclude` globs
- `.golangci.yml` — `linters.exclusions.rules` (currently one `_test.go` → `gosec` carve-out) — **add a `^cmd/coverage-report/` → `gosec` carve-out** (D-19)
- `.gitignore` — `web/coverage/` and `*.out` / `coverage.out` already ignored; no new entry needed for the reporter output
- Phase 09 coverage-gate decisions: `09-CONTEXT.md` D-01 (hand-rolled gate, no prerequisites), D-03 (log-only, no artifact upload), D-04 (`internal/db/sqlc` exclusion), D-05 (`cmd/server` kept in) — the model for D-07's exclusion of `cmd/coverage-report`
- Phase 08/09 frontend test decisions: `08-CONTEXT.md` D-04 / D-08, `09-CONTEXT.md` D-11 — "a step inside an already-blocking job, not a new top-level job" reasoning (note: this phase's `coverage-comment` *is* a new job, but deliberately outside every `needs:` graph, which is the safe case that reasoning was protecting against)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`make coverage-gate`'s current parse** (`go tool cover -func=coverage.out | grep '^total:' | awk '{...}'`) — **D-17 replaces this**: the Go tool owns the one hand-parse of raw `coverage.out` statement counts, `coverage-report` prints it, and `coverage-gate` consumes it. The comment and the gate agree by construction, not by two implementations staying in sync.
- **`build-scan` → `release` artifact hand-off** (`actions/upload-artifact@…v7` / `actions/download-artifact@…v8`, both already SHA-pinned in the workflow) — the exact pattern for D-15's current-profile hand-off to `coverage-comment`.
- **`release` job's `permissions: { contents: write, packages: write }`** (job-scoped) — the model for scoping `pull-requests: write` to `coverage-comment` alone (Pitfall 24).
- **`gitleaks` / `release` jobs use `fetch-depth: 0`** — *not* a precedent for `coverage-comment` (D-16 superseded): those jobs walk git history (secret scan, `svu` tag walk); the comment job does not.
- **`.golangci.yml`'s `_test.go` → `gosec` exclusion rule** — the exact shape D-19 mirrors for `cmd/coverage-report`.
- **Vitest v8 coverage provider** (`@vitest/coverage-v8` 4.1.10) already installed for the 70% thresholds gate — adding the `json-summary` reporter needs no new npm dependency.

### Established Patterns
- **`COVER_PKGS` exclusion is an anchored regex** (`(^|/)internal/db/sqlc$$`, doubled `$$` for make) explicitly designed so a future package isn't dropped by a loose substring — D-07 extends this same pattern for `cmd/coverage-report`.
- **CI coverage is two independent single-language mechanisms** (Makefile literal `80`, `vitest.config.ts` literal `70`) — Phase 09 rejected a third-party coverage action for "one greppable threshold literal per side". D-05's custom-script route preserves this; two ready-made actions would not.
- **Third-party actions are SHA-pinned with a trailing `# vX.Y.Z` comment** — every `uses:` in `full-pipeline.yml` follows this; `marocchino/sticky-pull-request-comment` must too.
- **Top-level `permissions: contents: read`; jobs opt into more** — never widen the workflow default.
- **New `cmd/*` main packages** compile under `go vet ./...` and `golangci-lint run ./...` automatically — the Go tool inherits the existing `vet` / `lint` jobs for free, which is also why D-19's `gosec` carve-out is needed (the `lint` job *will* see G304 on the file reads).

### Integration Points
- `.github/workflows/full-pipeline.yml` — new `coverage-comment` job (`if: !cancelled() && pull_request`, D-18); `cache/save` (with `continue-on-error`, D-04) + `upload-artifact` (before the gate step, D-15/D-18) steps added to `test` and `frontend-test`.
- `Makefile` — `COVER_PKGS` grep extended (D-07); `coverage-gate` refactored to consume the new `coverage-report` target (D-17).
- `web/vitest.config.ts` — `coverage.reporter` array gains `json-summary` (D-06 review note).
- `.golangci.yml` — `gosec` carve-out for `cmd/coverage-report` (D-19).
- `cmd/coverage-report/` — new Go main package + its `_test.go`.

</code_context>

<specifics>
## Specific Ideas

- **One comment is a hard requirement**, not a preference — SC #1 ("exactly one comment") and SC #2 ("three pushes leave one comment, not three") were treated as locked, and the two-ready-made-actions route was rejected specifically because it produces two.
- The renderer being **a real, tested Go program** rather than an inline shell blob was a deliberate portfolio choice — it's a small `cmd/` tool with unit tests, kept out of the product coverage number so it doesn't distort the very metric it reports.
- The comment should read calmly under a coverage drop: show the drop, mark it against the gate, and stay out of the way — the actual gate's red check is the signal that matters, the comment is context.
- Keep the new configuration/among-jobs surface minimal: no new job on the release path, no new secret, no new env var, no new npm dependency, one new small Go package.

</specifics>

<deferred>
## Deferred Ideas

- **CICD-15 — patch/diff-level coverage** (percentage of lines *added or changed in this PR* that are covered). A separate, already-catalogued Future requirement; `fgrosse/go-coverage-report` does this for Go and the frontend would need lcov + a diff-cover step. Revisit as its own phase. D-02 caches the full profiles (alongside the metrics sidecar) partly so this doesn't need a baseline-format change later; that phase is also when `fetch-depth: 0` (D-16) and the Vitest `json` reporter get added.
- **Fork-PR comment posting via `pull_request` + `workflow_run` two-workflow split** — the correct fork-safe pattern per Pitfall 25 / STACK.md. Not built now (this repo's PRs are same-repo branches); fork PRs degrade to the job summary. Adopt only if external contributors actually appear.
- **All four Vitest axes in the comment** — D-10 shows `lines` only; expanding to statements/branches/functions is a low-cost follow-up if the single number proves insufficient.
- **A more durable baseline store** (orphan branch or repo variable) — D-01 starts with Actions cache; revisit only if 7-day eviction on an idle `main` becomes annoying in practice.
- **Baseline-staleness *annotation*** (`, N commits behind`, "drift may apply") — still rejected (D-03, diff silently). Note the grill review *did* fold in the bare `baseline: main@<short-sha>` provenance line (not an annotation — just the delta's reference point); the commit-count / drift-warning remains out.

### Reviewed Todos (not folded)
None — `todo.match-phase 15` returned no matches.

</deferred>

---

*Phase: 15-pr-coverage-diff-comment*
*Context gathered: 2026-09-02*
