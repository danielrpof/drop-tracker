# Phase 15: PR Coverage-Diff Comment - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-02
**Phase:** 15-pr-coverage-diff-comment
**Areas discussed:** Baseline storage, One comment vs. two actions, Comment content, Baseline publish trigger

---

## Baseline storage

### How the baseline is stored

| Option | Description | Selected |
|--------|-------------|----------|
| Actions cache + restore-keys | `cache/save` on push-to-main under `coverage-baseline-main-<sha>` / prefix; PR restores by prefix. Simplest, zero repo noise; ~7-day idle eviction → absolute-only fallback. | ✓ |
| 90-day retention artifact | `fgrosse/go-coverage-report`'s native mode auto-pulls baseline from latest main run artifact; frontend side still needs a fiddly cross-run download. | |
| Orphan coverage-baseline branch | Commit the numbers to a dedicated orphan branch each main build. Permanent, never evicts; extra commit per merge + more machinery. | |

**User's choice:** Actions cache + restore-keys
**Notes:** 7-day eviction accepted — SC #3 already covers the absolute-only fallback gracefully.

### Cache payload

| Option | Description | Selected |
|--------|-------------|----------|
| Full profiles | Cache `coverage.out` + `coverage-summary.json`. Same parser both sides; keeps a future per-file view open with no re-key. | ✓ |
| Just the numbers | Cache a tiny JSON of percentages. Minimal; CICD-15 would force a format change later. | |

**User's choice:** Full profiles

### Stale restored baseline

| Option | Description | Selected |
|--------|-------------|----------|
| Use it silently | A prefix restore-keys hit is "most recent main baseline we have" — diff, no caveat. | ✓ |
| Show baseline commit + age | Footer: `baseline: main@abc1234 (3 commits behind)`. | |
| Warn only if very stale | Note only when >N commits / >7 days old. | |

**User's choice:** Use it silently

### Job-failure posture

| Option | Description | Selected |
|--------|-------------|----------|
| Never red, always | Whole job best-effort; swallow every failure; exit 0. A red check on a reporting feature is unacceptable. | ✓ |
| Red only on internal bugs | Infra failures degrade silently; a genuine parsing bug fails the job so it's noticed. Hard to draw the line in YAML. | |

**User's choice:** Never red, always

---

## One comment vs. two actions

### Route to exactly one comment

| Option | Description | Selected |
|--------|-------------|----------|
| Custom script + sticky comment | ~50-line renderer reads both profiles, diffs vs cached baseline, posts one table via `marocchino/sticky-pull-request-comment`. Own the diff logic for two formats; matches Phase 09's "no third-party coverage action" posture. | ✓ |
| k1LoW/octocov | One tool, one config, one comment; introduces a datastore concept overlapping the cache decision. | |
| Accept two comments | `fgrosse` + `davelosert` as-is. Least code; contradicts SC #1/#2 — would need the ROADMAP amended. | |

**User's choice:** Custom script + sticky comment

### Renderer language

| Option | Description | Selected |
|--------|-------------|----------|
| Bash | Pure `run:` shell — `go tool cover`, `jq`, awk, heredoc. No toolchain. | |
| Go | Small `cmd/coverage-report` main package, unit-testable, typed. Fits the Go portfolio framing. | ✓ |
| Node/JS script | `web/scripts/`-style JS — natural for the Vitest JSON, more for the Go side. | |

**User's choice:** Go

### Placement / coverage denominator

| Option | Description | Selected |
|--------|-------------|----------|
| cmd/coverage-report, excluded | New main package; kept OUT of `COVER_PKGS` so it never moves the 80% number. Own unit tests still run. | ✓ |
| internal/tools/covreport | Under `internal/` as lib + thin cmd wrapper; same exclusion. | |
| cmd/coverage-report, included | Left IN the denominator. Simplest rule; couples the CI helper to the product gate. | |

**User's choice:** cmd/coverage-report, excluded

### Sticky mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| marocchino, hidden marker | `marocchino/sticky-pull-request-comment@<sha>` (v3.0.5), fixed `header:`, hidden HTML marker, edit-in-place; concurrency `cancel-in-progress: true`. | ✓ |
| gh CLI, self-managed marker | `gh pr comment` + search-then-edit on a `<!-- coverage-report -->` marker. One fewer action to pin; reimplements marocchino. | |

**User's choice:** marocchino, hidden marker

---

## Comment content

### Body layout

| Option | Description | Selected |
|--------|-------------|----------|
| Table: coverage, Δ, gate | 2-row table (Backend/Frontend): Coverage %, Δ vs main (signed pp), Gate (80%/70%), ✅/⚠️ status vs gate. | ✓ |
| Just coverage + Δ | Two rows, two columns; no gate column. | |
| Table + per-file changed | Summary table + collapsed `<details>` of changed files. Edges into deferred CICD-15. | |

**User's choice:** Table: coverage, Δ, gate

### Frontend axis

| Option | Description | Selected |
|--------|-------------|----------|
| Lines only | Headline the Frontend row with `lines` % + Δ; gate column still 70%. | ✓ |
| All four axes | statements/branches/functions/lines each with % + Δ. Matches the Vitest gate exactly; wider table. | |
| Lines headline + others collapsed | Lines on the row; the rest in a `<details>`. | |

**User's choice:** Lines only

### No-baseline copy

| Option | Description | Selected |
|--------|-------------|----------|
| Dash + short footnote | Δ shows `—`; one-line footer explains "no main baseline cached yet (first run or evicted)". | ✓ |
| Dash only | `—`, no explanation. | |
| Inline "n/a (no baseline)" | Δ cell literally says so per row. | |

**User's choice:** Dash + short footnote

### Smaller content choices (multi-select)

| Option | Description | Selected |
|--------|-------------|----------|
| Delta precision: 2 decimals | `82.41%` / `-0.12pp`. | ✓ |
| Zero-delta shows `±0.00pp` | "No change" visibly distinct from "no baseline". | ✓ |
| Footer: commit + timestamp | PR head short-sha + UTC timestamp so a stale comment is self-evident. | ✓ |
| Title line with marker | Visible `## Coverage` heading above the table. | ✓ |

**User's choice:** all four

---

## Baseline publish trigger

### How the baseline is published

| Option | Description | Selected |
|--------|-------------|----------|
| Steps in existing test jobs | `cache/save` step added to `test` and `frontend-test`, guarded to push-on-main. Reuses coverage those jobs already produce; no new job, no artifact hand-off for the save. | ✓ |
| Dedicated baseline job | New `coverage-baseline` job `needs: [test, frontend-test]`; needs the two jobs to expose profiles as artifacts. | |

**User's choice:** Steps in existing test jobs

### Save even if the gate failed on main?

| Option | Description | Selected |
|--------|-------------|----------|
| Save only if gate passed | `cache/save` after the gate step, `if: success()`. A sub-threshold main is broken; don't enshrine it. Red main keeps the last-good baseline. | ✓ |
| Always save the number | `if: always()` (main-push only). Baseline = "what main is", not "what it should be". | |

**User's choice:** Save only if gate passed

### How the PR's current profiles reach the comment job

| Option | Description | Selected |
|--------|-------------|----------|
| Upload as artifacts on every run | `test` + `frontend-test` always upload the profiles as ~1-day artifacts; `coverage-comment` downloads both. Mirrors build-scan→release. | ✓ |
| Upload only on pull_request | Same, guarded `if: pull_request`. Marginally less churn on main. | |

**User's choice:** Upload as artifacts on every run

### Checkout depth on the comment job

| Option | Description | Selected |
|--------|-------------|----------|
| fetch-depth: 0 | Full history, matching gitleaks/release. Safe for any git op the footer / future view needs. | ✓ |
| Default (depth 1) + no git ops | Shallow; tool works purely from downloaded profiles + cached baseline. | |
| No checkout at all | Only downloads artifacts; requires the tool shipped as an artifact too. | |

**User's choice:** fetch-depth: 0

---

## Claude's Discretion

- Exact cache key shape; whether backend/frontend share one key or use two prefixes.
- Exact `make` target name and whether number-printing is a new target or folded into the Go tool.
- Footer wording, heading text, ✅/⚠️ glyph choice.
- Artifact names and exact retention days (~1 day).
- `continue-on-error` at job level vs. per-step.
- Whether the Go tool or a separate step owns the `$GITHUB_STEP_SUMMARY` fork/degradation path.
- How the Go tool is delivered to the job (`go run` off the full checkout vs. uploaded binary).

## Deferred Ideas

- CICD-15 — patch/diff-level (uncovered-new-lines) coverage. Separate Future requirement / own phase.
- Fork-PR comment posting via `pull_request` + `workflow_run` split. Not built now; fork PRs degrade to job summary.
- All four Vitest axes in the comment (lines-only chosen for now).
- More durable baseline store (orphan branch / repo variable) if 7-day eviction proves annoying.
- Baseline-staleness annotation in the comment (baseline SHA is cached, so cheap to add later).
