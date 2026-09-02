# Phase 15: PR Coverage-Diff Comment - Context

**Gathered:** 2026-09-02
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
- A new `coverage-comment` job in `.github/workflows/full-pipeline.yml`, `if: github.event_name == 'pull_request'`, `needs: [test, frontend-test]`, `permissions: { contents: read, pull-requests: write }` (job-scoped only), its own `concurrency` group with `cancel-in-progress: true`.
- A small **Go tool** at `cmd/coverage-report/` that reads both current coverage profiles + the cached baseline profiles, computes the deltas, and renders one markdown table. Unit-tested. **Excluded** from the Makefile `COVER_PKGS` list so it never moves the 80% product-coverage number.
- Baseline publication: `cache/save` steps added to the existing `test` and `frontend-test` jobs, guarded to `push` on `refs/heads/main` and to a passing coverage gate.
- PR-side current-coverage hand-off: `test` and `frontend-test` upload `coverage.out` / `coverage-summary.json` as short-retention artifacts the `coverage-comment` job downloads.
- `web/vitest.config.ts` gains `json-summary` (and `json`) coverage reporters. `web/coverage/` is already gitignored.
- A `make coverage-report` target (or equivalent) that prints just the number `coverage-gate` already computes, so the comment and the gate never disagree.

**In scope:** CICD-13, CICD-14 (see `.planning/REQUIREMENTS.md`).

**Out of scope:**
- **CICD-15** — patch/diff-level (uncovered-new-lines) coverage. Explicitly a Future requirement; the comment shows file/total deltas only.
- Fork-PR comment posting. Fork PRs get a read-only `GITHUB_TOKEN`; the feature degrades to the job summary and never blocks the PR. `pull_request_target` is **prohibited outright**.
- Any application-code change. The only Go added is the CI helper at `cmd/coverage-report/`.
- Per-package / per-file coverage tables in the comment body.

</domain>

<decisions>
## Implementation Decisions

### Baseline storage
- **D-01:** Baseline is stored in the **GitHub Actions cache** under a `<sha>`-suffixed key with the stable prefix `coverage-baseline-main-` (per language), restored on PR runs via `restore-keys` prefix match. Accept the ~7-day idle-eviction trade-off — an idle `main` loses the baseline and that PR run shows absolute numbers only (D-06 handles the copy). Rejected: 90-day retention artifact, orphan `coverage-baseline` branch. — **Reversibility:** reversible — swapping to an artifact or branch later is a self-contained workflow edit; no consumer outside `full-pipeline.yml`.
- **D-02:** The cache entry holds the **full coverage profiles** — `coverage.out` (Go) and `coverage-summary.json` (Vitest) — plus the baseline commit SHA, not just pre-computed percentages. Keeps the comment's parser identical on the baseline and current sides and leaves the door open for a future changed-files view without re-keying the cache.
- **D-03:** A restored baseline that is **stale** (a prefix `restore-keys` hit from an older `main` SHA) is diffed against **silently** — no "baseline N commits behind" caveat in the comment. The drift from intervening `main` commits is tolerable noise; the delta stays directionally correct.
- **D-04:** The entire `coverage-comment` job is **best-effort / never red** — every failure mode (cache API error, comment POST 5xx, malformed profile, fork read-only token) is swallowed and the job exits 0. A silently broken reporting job is acceptable for a report-only nicety; a red check on a reporting feature is not. (`continue-on-error` at job level, or every step guarded, whichever the planner finds cleaner.)

### One comment, not two
- **D-05:** Exactly one comment (SC #1/#2) is achieved with a **custom rendering step + `marocchino/sticky-pull-request-comment`** (SHA-pinned, v3.0.5), **not** the two ready-made single-language actions (`fgrosse/go-coverage-report` + `davelosert/vitest-coverage-report-action`, which produce two comments) and **not** `k1LoW/octocov` (all-in-one, introduces a datastore concept that overlaps D-01). This preserves the repo's Phase-09 posture: coverage stays two independent single-language mechanisms with literal thresholds, no third-party coverage *gating* action. — **Reversibility:** costly — undoing means re-modelling the comment as two actions and amending ROADMAP SC #1 ("exactly one comment").
- **D-06:** The renderer is a **Go tool** at `cmd/coverage-report/` — a small, unit-testable `main` package (fits the Go portfolio framing; parses `go tool cover -func` output and the Vitest JSON summary; emits the markdown). Rejected: an inline bash `run:` step, a Node/JS script.
- **D-07:** `cmd/coverage-report/` is **excluded from the Makefile `COVER_PKGS`** list (extend the existing anchored `grep -vE` that already drops `internal/db/sqlc`), so this CI helper never enters the 80% product-coverage denominator. Its own `go test ./cmd/coverage-report/...` still runs in CI. Rejected: leaving it in the denominator "because all our Go code counts".
- **D-08:** Stickiness is `marocchino/sticky-pull-request-comment` with a fixed `header:` and its hidden HTML marker — find prior comment, edit in place, create only if absent. `concurrency: { group: coverage-comment-${{ github.ref }}, cancel-in-progress: true }` so a rapid re-push supersedes an in-flight comment. Rejected: hand-rolled `gh pr comment` + self-managed marker search.

### Comment content
- **D-09:** Body is a **visible `## Coverage` heading** above a **2-row markdown table** (Backend / Frontend) with columns: **Coverage %**, **Δ vs main** (signed, percentage points), **Gate** (`80%` / `70%`), and a **status mark** (✅ / ⚠️) versus that gate. Rejected: coverage + Δ only (no gate column); table + per-file `<details>` (edges into deferred CICD-15).
- **D-10:** The **Frontend row uses the Vitest `lines` axis only** for its headline % and Δ; the Gate column still reads `70%`. The other three axes (statements / branches / functions) are not shown. Rejected: all four axes; lines-headline with the rest in a `<details>`.
- **D-11:** **No-baseline state** (SC #3 — first run after ship, or evicted cache): the Δ column shows `—` for both rows and a one-line footer reads roughly _"Delta unavailable — no `main` baseline cached yet (first run or evicted). Absolute coverage shown."_ Rejected: bare `—` with no explanation; inline `n/a (no baseline)` per row.
- **D-12:** Numeric formatting: **2 decimal places** (`82.41%`, `-0.12pp`); an unchanged value renders as **`±0.00pp`** so "no change" is visually distinct from "no baseline" (`—`). A **footer line** carries the PR head short-SHA the numbers came from and a UTC timestamp, so a stale comment is self-evident.

### Baseline publish trigger
- **D-13:** Baseline is published by **`cache/save` steps inside the existing `test` and `frontend-test` jobs**, each guarded `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`. No dedicated `coverage-baseline` job. Each job saves its own language's profile under its own prefixed key.
- **D-14:** The `cache/save` step runs **only after that job's coverage gate passed** (`test`'s `make coverage-gate` / `frontend-test`'s Vitest thresholds). A sub-threshold `main` is a broken state and must not be enshrined as the baseline — a red `main` leaves the previous good baseline in place. Rejected: `if: always()` "save whatever main actually is".
- **D-15:** The PR's **current** coverage profiles reach the `coverage-comment` job as **short-retention (~1-day) artifacts uploaded on every run** of `test` / `frontend-test` (mirrors how `build-scan` → `release` already hand off the scanned image). Rejected: guarding the upload to `pull_request` only (marginal churn saving, less consistent). The `coverage-comment` job never re-runs any test suite.
- **D-16:** The `coverage-comment` job checks out with **`fetch-depth: 0`**, consistent with the `gitleaks` and `release` jobs — safe for any `git merge-base` / commit-count the footer or a future view might need. Rejected: default shallow checkout; no checkout at all.

### Locked upstream (not re-discussed — carried from ROADMAP / REQUIREMENTS / research)
- Report-only; `coverage-comment` in **no** release-path `needs:` graph — `needs: [test, frontend-test]` and nothing else; nothing `needs:` it.
- `pull-requests: write` scoped to the `coverage-comment` job only — never workflow-wide (the workflow default stays `contents: read`).
- Same-repo branches only; fork PRs degrade to the job summary; `pull_request_target` prohibited.
- Merge to `main` publishes that run's coverage as the baseline; **no PR run recomputes `main`'s coverage** (SC #5).
- Reuse the coverage the `test` / `frontend-test` jobs already produce — no second test invocation anywhere.
- All third-party actions SHA-pinned with a version comment (CLAUDE.md).

### Claude's Discretion
- Exact cache key shape and whether backend/frontend share one key or use two prefixes (D-13 implies two; planner confirms).
- Exact `make` target name (`coverage-report` vs other) and whether the number-printing logic is a new target or folded into the Go tool invocation.
- The precise footer wording, heading text, and the ✅/⚠️ glyph choice.
- Artifact names and retention days (D-15 says ~1 day).
- Whether `continue-on-error` sits at job level or per-step (D-04).
- Whether the Go tool also owns the "post to `$GITHUB_STEP_SUMMARY`" fork/degradation path or a separate step does.
- How the Go tool is delivered to the job (built in-job via `go run` off the `fetch-depth: 0` checkout, vs. an uploaded binary) — D-16 gives it a full checkout, so `go run ./cmd/coverage-report` is available.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/ROADMAP.md` §"Phase 15: PR Coverage-Diff Comment" — goal, 5 success criteria, the `needs:` / permissions / fork / `pull_request_target` constraints in _Notes_
- `.planning/REQUIREMENTS.md` — CICD-13, CICD-14 (locked text); CICD-15 (deferred — patch/diff coverage, explicitly out of scope)

### v1.3 research (this milestone)
- `.planning/research/PITFALLS.md` items **24–27** — the checklist this phase is judged against: `GITHUB_TOKEN` PR-comment perms (24), fork-PR degradation + `pull_request_target` prohibition (25), comment spam / sticky-upsert (26), baseline fetch on first run / shallow checkout (27). Also the "Looks Done But Isn't" §"Coverage comment" line and the Pitfall-to-Phase mapping rows 24–27.
- `.planning/research/ARCHITECTURE.md` §"Feature 3 — Coverage-diff PR comment" — producing-job table, `needs: [test, frontend-test]` rationale, the guards YAML block, cache-baseline mechanism, NEW-vs-MODIFIED table
- `.planning/research/STACK.md` §"Feature 3 — PR coverage-diff comment (CICD-13)" — SHA pins: `marocchino/sticky-pull-request-comment` v3.0.5 (`5770ad5eb8f42dd2c4f34da00c94c5381e49af88`); the custom-script + sticky-comment route is the documented alternative this phase adopts; fork-PR / `pull_request_target` warning; `json-summary` + `json` Vitest reporter requirement
- `.planning/research/FEATURES.md` §"Feature 3" — table-stakes list (total % + signed Δ, backend + frontend), single sticky comment, anti-features (hard merge gate, whole-repo per-package table), reuse of existing coverage runs

### Existing code this phase modifies or mirrors
- `.github/workflows/full-pipeline.yml` — `test` job (`make test-integration` → `coverage.out`, then `make coverage-gate`), `frontend-test` job (`pnpm test` → Vitest coverage/thresholds), `release` job's job-scoped `permissions` pattern, `build-scan` → `release` artifact hand-off pattern (`upload-artifact` v7 / `download-artifact` v8, pinned), the top-level `permissions: contents: read` and `concurrency` block
- `Makefile` — `COVER_PKGS` (anchored `go list ... | grep -vE '(^|/)internal/db/sqlc$$'` — extend to also drop `cmd/coverage-report`), `coverage-gate` target (the `go tool cover -func | grep '^total:'` parse the comment must match), `COVERAGE_THRESHOLD_BACKEND ?= 80`
- `web/vitest.config.ts` — `coverage.reporter` (currently `["text"]`, "writes nothing to disk" posture — add `json-summary`, `json`), `coverage.thresholds` (70 on all four axes), `coverage.include` / `coverage.exclude` globs
- `.gitignore` — `web/coverage/` and `*.out` / `coverage.out` already ignored; no new entry needed for the reporter output
- Phase 09 coverage-gate decisions: `09-CONTEXT.md` D-01 (hand-rolled gate, no prerequisites), D-03 (log-only, no artifact upload), D-04 (`internal/db/sqlc` exclusion), D-05 (`cmd/server` kept in) — the model for D-07's exclusion of `cmd/coverage-report`
- Phase 08/09 frontend test decisions: `08-CONTEXT.md` D-04 / D-08, `09-CONTEXT.md` D-11 — "a step inside an already-blocking job, not a new top-level job" reasoning (note: this phase's `coverage-comment` *is* a new job, but deliberately outside every `needs:` graph, which is the safe case that reasoning was protecting against)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`make coverage-gate`'s parse** (`go tool cover -func=coverage.out | grep '^total:' | awk '{...}'`) — the Go tool must produce a total identical to this so the comment and the 80% gate never disagree; factor it into a shared `make coverage-report` target or replicate exactly.
- **`build-scan` → `release` artifact hand-off** (`actions/upload-artifact@…v7` / `actions/download-artifact@…v8`, both already SHA-pinned in the workflow) — the exact pattern for D-15's current-profile hand-off to `coverage-comment`.
- **`release` job's `permissions: { contents: write, packages: write }`** (job-scoped) — the model for scoping `pull-requests: write` to `coverage-comment` alone (Pitfall 24).
- **`gitleaks` / `release` jobs already use `fetch-depth: 0`** — precedent for D-16.
- **Vitest v8 coverage provider** (`@vitest/coverage-v8` 4.1.10) already installed for the 70% thresholds gate — adding `json-summary`/`json` reporters needs no new npm dependency.

### Established Patterns
- **`COVER_PKGS` exclusion is an anchored regex** (`(^|/)internal/db/sqlc$$`, doubled `$$` for make) explicitly designed so a future package isn't dropped by a loose substring — D-07 extends this same pattern for `cmd/coverage-report`.
- **CI coverage is two independent single-language mechanisms** (Makefile literal `80`, `vitest.config.ts` literal `70`) — Phase 09 rejected a third-party coverage action for "one greppable threshold literal per side". D-05's custom-script route preserves this; two ready-made actions would not.
- **Third-party actions are SHA-pinned with a trailing `# vX.Y.Z` comment** — every `uses:` in `full-pipeline.yml` follows this; `marocchino/sticky-pull-request-comment` must too.
- **Top-level `permissions: contents: read`; jobs opt into more** — never widen the workflow default.
- **New `cmd/*` main packages** compile under `go vet ./...` and `golangci-lint run ./...` automatically — the Go tool inherits the existing `vet` / `lint` jobs for free.

### Integration Points
- `.github/workflows/full-pipeline.yml` — new `coverage-comment` job; `cache/save` + `upload-artifact` steps added to `test` and `frontend-test`.
- `Makefile` — `COVER_PKGS` grep extended; new number-printing target.
- `web/vitest.config.ts` — `coverage.reporter` array.
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

- **CICD-15 — patch/diff-level coverage** (percentage of lines *added or changed in this PR* that are covered). A separate, already-catalogued Future requirement; `fgrosse/go-coverage-report` does this for Go and the frontend would need lcov + a diff-cover step. Revisit as its own phase. D-02 (cache full profiles) is chosen partly so this doesn't need a baseline-format change later.
- **Fork-PR comment posting via `pull_request` + `workflow_run` two-workflow split** — the correct fork-safe pattern per Pitfall 25 / STACK.md. Not built now (this repo's PRs are same-repo branches); fork PRs degrade to the job summary. Adopt only if external contributors actually appear.
- **All four Vitest axes in the comment** — D-10 shows `lines` only; expanding to statements/branches/functions is a low-cost follow-up if the single number proves insufficient.
- **A more durable baseline store** (orphan branch or repo variable) — D-01 starts with Actions cache; revisit only if 7-day eviction on an idle `main` becomes annoying in practice.
- **Baseline-staleness annotation in the comment** (`baseline: main@abc1234, N commits behind`) — considered and rejected for now (D-03, diff silently); the baseline SHA is cached (D-02) so this is cheap to add later.

### Reviewed Todos (not folded)
None — `todo.match-phase 15` returned no matches.

</deferred>

---

*Phase: 15-pr-coverage-diff-comment*
*Context gathered: 2026-09-02*
