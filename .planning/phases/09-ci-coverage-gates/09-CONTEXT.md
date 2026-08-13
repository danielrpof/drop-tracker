# Phase 9: CI Coverage Gates - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

The Full Pipeline (`.github/workflows/full-pipeline.yml`) stops merely running tests and starts enforcing coverage on both languages: the existing `test` job (Go, via `make test-integration`) gets a coverage-gate step failing under 80%, and the existing `frontend-test` job (added in Phase 8, currently report-only) gets a coverage-gate step failing under 70%. A failing gate on either side must block `build-scan` (and therefore `release`) the same way `vet`/`lint`/`test`/`gitleaks`/`trivy-fs` already do. If either measured baseline lands under its threshold, this phase closes the gap with real tests targeting the most meaningful uncovered behavior — never by lowering the number. This phase does not touch the retention window (Phase 10) or concurrent polling (Phase 11).

</domain>

<decisions>
## Implementation Decisions

### Backend coverage gate mechanism
- **D-01:** Hand-rolled shell check — `go tool cover -func=coverage.out`, grep the `total:` line, compare against 80 in a shell/awk step, `exit 1` on failure. Matches this repo's established pattern of hand-rolling simple mechanical checks (Discord webhook client, DSN redaction) rather than pulling in a dependency for a one-line comparison.
- **D-02:** The coverage profile comes from an extended `make test-integration` (real Postgres DB, `-race`, `-p 1`) — the same invocation CI's existing `test` job already runs. Most of this codebase's meaningful coverage lives in DB-backed integration tests (per `.planning/codebase/TESTING.md`); a `-short`-only profile would undercount badly and likely can't reach 80% aggregate.
- **D-03:** Coverage report is log-only — print the total % and pass/fail to the job log, no artifact upload. Matches current scope; CICD-13 (PR coverage-diff comment) is explicitly deferred to v2.

### Coverage scope & exclusions
- **D-04:** Generated sqlc code (`internal/db/sqlc/`) is excluded from the Go coverage calculation — it's generated and mechanically correct by construction (`sqlc-check` already diffs it against schema); testing it directly would just re-test sqlc itself.
- **D-05:** `cmd/server` (main.go wiring — config load, DB connect, router setup, graceful shutdown) IS included in the Go coverage calculation. It's real, hand-written orchestration logic — the graceful-shutdown and migration-retry behavior it wires together is exactly the kind of thing Phase 1's UAT had to verify by hand. Excluding it would hide untested wiring behind the aggregate number.
- **D-06:** On the frontend, both React Router's generated route-typegen output and shadcn UI primitives (`web/app/components/ui/`) are excluded from the 70% calculation — neither is hand-written first-party logic, same rationale as excluding sqlc.

### Frontend coverage provider
- **D-07:** Vitest's `v8` coverage provider (native to Node's V8 engine, no source-instrumentation step, fastest, current Vitest/Node default recommendation) over `istanbul`. Neither is installed yet — this phase adds `@vitest/coverage-v8` to `web/package.json`.
- **D-08:** The 70% threshold is enforced via Vitest's own built-in `coverage.thresholds` config in `vitest.config.ts` — `vitest run` fails non-zero automatically when under threshold, no separate check script needed. Keeps the `frontend-test` CI job (added in Phase 8) a single `pnpm test` step exactly as it is today.

### Baseline gap-closing depth
- **D-09:** If a measured baseline lands under its threshold, prioritize the most meaningful uncovered behavior first — not just whatever closes the numeric gap fastest. An untested error path can matter more than an easy-to-cover getter, even if the getter would close the gap quicker. This project's whole point is demonstrating rigorous CI/CD practice, so thoroughness over the numerically-fastest path fits the portfolio goal.
- **D-10:** Gap-closing tests are plain test-add commits, not RED-then-GREEN — no bug is expected (the code already works, it's just untested), so a normal passing test lands in its own commit. RED-then-GREEN only applies if a gap-closing test happens to uncover an actual defect (mirrors Phase 8's folded-todo bug-fix precedent), which then gets its own separate fix commit.

### Job wiring (carried forward from Phase 8, not re-discussed)
- **D-11:** Backend coverage extends the existing `test` job; frontend coverage extends the existing `frontend-test` job (already created in Phase 8 as report-only, per `08-CONTEXT.md` D-04). Neither side needs a new job. `build-scan`'s `needs:` list gets `frontend-test` added alongside the existing `[vet, lint, test, gitleaks, trivy-fs]`.

### Claude's Discretion
None — all discussed areas resolved to a specific choice.

### Folded Todos
- **Fix flaky tests under parallel `go test ./...`** (`.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md`, minor, `resolves_phase: 9`) — Two flakiness classes surfaced during Phase 6's post-merge gates: (1) `internal/notifier` tests asserting on real-time sleep/spacing behavior flake under package-level parallelism, (2) `internal/poller` hit a one-off `relation "artists" does not exist` against the shared Postgres instance, pointing to a schema-visibility race when packages run concurrently. Both reproduce only without `-p 1`; CI's `test` job already runs `make test-integration` which is `-p 1`, so CI itself isn't currently exposed. Folded because this phase adds `-coverprofile` instrumentation to that same `make test-integration` invocation (D-02) — worth confirming coverage instrumentation doesn't change timing/scheduling enough to surface the notifier flakiness even under `-p 1`, and worth resolving properly since the todo is explicitly tagged for this phase.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` (§ CI/CD Pipeline, lines 78-81) — CICD-11, CICD-12 exact requirement text; also § Out of Scope for "Per-package/diff coverage gating" and "Mutation testing" exclusions already locked
- `.planning/ROADMAP.md` (§ Phase 9: CI Coverage Gates, lines 274-292) — goal, success criteria, and the "both gates edit the same file" / job-extension notes

### CI pipeline (the file this phase edits)
- `.github/workflows/full-pipeline.yml` — `test` job (lines 43-54) and `frontend-test` job (lines 83-105) both get coverage-gate steps added; `build-scan`'s `needs:` (line 120) gets `frontend-test` appended

### Prior phase context
- `.planning/phases/08-frontend-test-suite/08-CONTEXT.md` (D-04) — confirms `frontend-test` job already exists as report-only, created specifically so Phase 9 only adds a gate step, not a new job
- `.planning/PROJECT.md` (§ Key Decisions, Phase 8 row) — "the `frontend-test` CI job runs in `full-pipeline.yml`'s parallel tier but is deliberately not yet wired into `build-scan`'s `needs:` — that blocking wiring is Phase 9's job"

### Build/test tooling
- `Makefile` (`test-integration` target, lines with `-p 1` comment) — the invocation D-02 extends with `-coverprofile`
- `web/package.json` — `test` script (`vitest run`), no coverage provider installed yet
- `web/vitest.config.ts` — where `coverage.thresholds` (D-08) gets added; note its header comment on why it never imports `web/vite.config.ts`

### Codebase conventions
- `.planning/codebase/TESTING.md` (§ Coverage, lines 381-393) — confirms no coverage enforcement exists yet; documents the manual `go test ./... -coverprofile=coverage.out` / `go tool cover -html` commands D-01 builds on

### Folded todo (full problem detail)
- `.planning/todos/pending/2026-08-11-fix-flaky-tests-under-parallel-go-test.md`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `.github/workflows/full-pipeline.yml`'s `test` job — already runs `make test-integration` with Go/setup-go pinned actions; the coverage step is an additional step appended here, not a new job.
- `.github/workflows/full-pipeline.yml`'s `frontend-test` job — already runs `pnpm install --frozen-lockfile` + `pnpm test` with pnpm/Node caching wired; D-08's threshold enforcement means `pnpm test` itself starts failing under threshold with zero CI YAML changes needed beyond adding the coverage provider dependency and config.
- `Makefile`'s `test-integration` target — single place to add `-coverprofile=coverage.out`, reused by both CI and local `make test`.

### Established Patterns
- Every third-party GitHub Action in this workflow is pinned to a commit SHA with a version comment (`# v7.0.1` etc.) per CICD-08 — any new action introduced would need the same treatment, though D-01's hand-rolled choice avoids needing a new action at all.
- `build-scan`'s existing `needs:` array (`[vet, lint, test, gitleaks, trivy-fs]`) is the exact pattern `frontend-test` gets appended to.
- Comments in `full-pipeline.yml` reference REVIEW.md finding IDs inline (e.g. `07-REVIEW.md CR-02`) when explaining a non-obvious step — the coverage-gate steps should follow the same convention if their rationale isn't self-evident from the step name.

### Integration Points
- `web/vitest.config.ts`'s `test` block is where `coverage: { provider: 'v8', thresholds: { ... }, exclude: [...] }` gets added (D-07, D-08, D-06).
- `web/package.json`'s `devDependencies` needs `@vitest/coverage-v8` added, version-matched to the already-pinned `vitest@4.1.10`.

</code_context>

<specifics>
## Specific Ideas

No specific UI/visual requirements — this is a CI/tooling phase, not a design phase. No particular coverage-report format or dashboard was requested.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (CICD-13's PR coverage-diff comment and per-package/diff gating remain out of scope per REQUIREMENTS.md, not newly deferred here.)

### Reviewed Todos (not folded)
None — the one matching todo was folded, see Folded Todos above.

</deferred>

---

*Phase: 09-CI Coverage Gates*
*Context gathered: 2026-08-12*
