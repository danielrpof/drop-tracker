# Phase 9: CI Coverage Gates - Research

**Researched:** 2026-08-13
**Domain:** CI/CD coverage enforcement (Go `go tool cover` + Vitest `@vitest/coverage-v8`)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Hand-rolled shell check — `go tool cover -func=coverage.out`, grep the `total:` line, compare against 80 in a shell/awk step, `exit 1` on failure. Matches this repo's established pattern of hand-rolling simple mechanical checks (Discord webhook client, DSN redaction) rather than pulling in a dependency for a one-line comparison.
- **D-02:** The coverage profile comes from an extended `make test-integration` (real Postgres DB, `-race`, `-p 1`) — the same invocation CI's existing `test` job already runs. Most of this codebase's meaningful coverage lives in DB-backed integration tests (per `.planning/codebase/TESTING.md`); a `-short`-only profile would undercount badly and likely can't reach 80% aggregate.
- **D-03:** Coverage report is log-only — print the total % and pass/fail to the job log, no artifact upload. Matches current scope; CICD-13 (PR coverage-diff comment) is explicitly deferred to v2.
- **D-04:** Generated sqlc code (`internal/db/sqlc/`) is excluded from the Go coverage calculation — it's generated and mechanically correct by construction (`sqlc-check` already diffs it against schema); testing it directly would just re-test sqlc itself.
- **D-05:** `cmd/server` (main.go wiring — config load, DB connect, router setup, graceful shutdown) IS included in the Go coverage calculation. It's real, hand-written orchestration logic — the graceful-shutdown and migration-retry behavior it wires together is exactly the kind of thing Phase 1's UAT had to verify by hand. Excluding it would hide untested wiring behind the aggregate number.
- **D-06:** On the frontend, both React Router's generated route-typegen output and shadcn UI primitives (`web/app/components/ui/`) are excluded from the 70% calculation — neither is hand-written first-party logic, same rationale as excluding sqlc.
- **D-07:** Vitest's `v8` coverage provider over `istanbul`. Neither is installed yet — this phase adds `@vitest/coverage-v8` to `web/package.json`.
- **D-08:** The 70% threshold is enforced via Vitest's own built-in `coverage.thresholds` config in `vitest.config.ts` — `vitest run` fails non-zero automatically when under threshold, no separate check script needed.
- **D-09:** If a measured baseline lands under its threshold, prioritize the most meaningful uncovered behavior first — not just whatever closes the numeric gap fastest.
- **D-10:** Gap-closing tests are plain test-add commits, not RED-then-GREEN — no bug is expected, so a normal passing test lands in its own commit. RED-then-GREEN only applies if a gap-closing test happens to uncover an actual defect.
- **D-11:** Backend coverage extends the existing `test` job; frontend coverage extends the existing `frontend-test` job (already created in Phase 8 as report-only). Neither side needs a new job. `build-scan`'s `needs:` list gets `frontend-test` added alongside `[vet, lint, test, gitleaks, trivy-fs]`.

### Claude's Discretion

None — all discussed areas resolved to a specific choice.

### Deferred Ideas (OUT OF SCOPE)

None from this phase's discussion. CICD-13 (PR coverage-diff comment) and per-package/diff coverage gating remain out of scope per `REQUIREMENTS.md`, not newly deferred here.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CICD-11 | CI fails the build if backend Go test coverage falls below 80% | `## Code Examples` (Makefile/`go tool cover` gate), `## Common Pitfalls` (Pitfall 1, 2, 4) confirm the exact `-coverpkg` mechanism needed so `internal/db/sqlc` is excluded and `cmd/server` is included in the same invocation |
| CICD-12 | CI fails the build if frontend test coverage falls below 70% | `## Code Examples` (`vitest.config.ts` coverage block), `## Common Pitfalls` (Pitfall 3) confirm Vitest 4 removed `coverage.all`, so `coverage.include` must be set explicitly or D-06's exclude list is measuring against a near-empty, self-inflating denominator |

</phase_requirements>

## Summary

Both gates are mechanically simple — a `go tool cover -func` total-line parse on the backend, Vitest's own built-in `coverage.thresholds` on the frontend — but two non-obvious tool-semantics facts change what "simple" means in practice for this codebase, and both were exactly what CONTEXT.md's research-focus list asked to verify.

**Backend:** `go test`'s coverage instrumentation defaults to *self-package only* — a package with zero `_test.go` files (like generated `internal/db/sqlc/`) never appears in the merged profile at all when `-coverpkg` is omitted. That means D-04's sqlc exclusion is **free** under a plain `go test ./... -coverprofile=coverage.out`. But D-05 wants the opposite for `cmd/server`: its untested orchestration logic (`cmd/server/main.go`, 205 lines, zero test files today) must count *against* the aggregate, which requires widening coverage measurement across package boundaries with `-coverpkg`. A single invocation can satisfy both: `-coverpkg` set to `go list ./...` with `internal/db/sqlc` filtered out — this is the "package-list exclusion" option CONTEXT.md asked to evaluate, and it is the recommended mechanism over post-processing `coverage.out` with `grep -v`.

**Frontend:** Vitest 4 **removed** the `coverage.all` option that older tutorials and blog posts still describe. The current default is "only files imported during the test run are measured" — with `coverage.include` left unset, an entirely untested route file (this repo currently has one: `app/routes/history.tsx`, zero test file) simply never appears in the coverage report and cannot drag the percentage down. That silently defeats D-06's intent (an honest first-party denominator with generated/vendor code carved out) unless `coverage.include` is set explicitly to the app's own source globs, with `exclude` then removing `components/ui/**` and route-typegen output. This is the single highest-risk gotcha in this phase: getting `coverage.include` wrong produces a technically-passing 70% gate that is measuring almost nothing.

Both baselines are currently unmeasured and, given the untested `cmd/server`, `internal/logging`, `internal/webassets` packages on the backend and the untested `history.tsx` route on the frontend, real gap-closing test work (D-09/D-10) should be budgeted as a first-class part of this phase's plan, not a contingency.

**Primary recommendation:** Extend `make test-integration` with `-coverprofile=coverage.out -coverpkg=<go-list-minus-sqlc>` and a hand-rolled `awk`-based total-line gate at 80%; add `@vitest/coverage-v8@4.1.10` with an explicit `coverage.include`/`coverage.exclude`/`coverage.thresholds` block in `vitest.config.ts` at 70%; measure both baselines first, and treat any package/route with zero prior tests (`cmd/server`, `history.tsx`) as an expected, in-scope gap-closing target rather than a surprise.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Backend coverage measurement & gate | CI (GitHub Actions `test` job) | Build tooling (`Makefile`) | Coverage instrumentation is a `go test` build-time concern; the gate step lives in the same job that already runs the instrumented test binary, per D-11 |
| Frontend coverage measurement & gate | CI (GitHub Actions `frontend-test` job) | Build tooling (`vitest.config.ts`) | `vitest run --coverage`'s threshold check is entirely internal to the Vitest process; the CI job just needs to observe its exit code, per D-08/D-11 |
| Pipeline gating (block `build-scan`) | CI (`full-pipeline.yml` `needs:` graph) | — | `build-scan`'s `needs:` array is the sole mechanism controlling whether a coverage failure blocks the image build/scan/push chain (already true for `test`; `frontend-test` needs adding per D-11) |
| Coverage exclusion policy (sqlc, shadcn ui, route-typegen) | Build tooling (`Makefile` `-coverpkg`, `vitest.config.ts` `coverage.exclude`) | — | Exclusion is a measurement-scope decision made once at the tool-invocation layer, not something CI orchestration needs to know about |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go tool cover` (Go stdlib) | bundled with `go1.26` | Backend coverage profile + `-func` total-line report | Already the tool `TESTING.md` documents as the manual coverage command; zero new dependency, matches D-01's hand-rolled-check pattern |
| `@vitest/coverage-v8` | 4.1.10 [VERIFIED: npm registry — `npm view @vitest/coverage-v8@4.1.10 version` returns `4.1.10`, exact match to the already-pinned `vitest@4.1.10` in `web/package.json:45`] | V8-native frontend coverage provider | D-07 locks this over istanbul; native V8 instrumentation, no source-rewrite step, current Vitest-team-recommended default provider |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| GNU `awk` | 5.0.0 [VERIFIED: local — `awk --version` this session; ubuntu-latest GitHub Actions runners ship `gawk` preinstalled, `[ASSUMED]` for the CI runner specifically since not verified on that exact image this session] | Float-safe percentage comparison in the backend gate step | Bash has no native floating-point comparison; `awk -v cov="$COVERAGE" -v thresh=80 'BEGIN{exit !(cov+0>=thresh)}'` is the portable idiom |
| `paste` (GNU coreutils) | 8.32 [VERIFIED: local — `paste --version` this session] | Joins `go list ./...` output into a comma-separated `-coverpkg` pattern list | `go list ./... | grep -v /internal/db/sqlc | paste -sd, -` — standard on any GNU/Linux CI runner |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled `awk` total-line gate (D-01) | A third-party coverage-check GitHub Action (e.g. a coverage-gate marketplace action) | Would need a new pinned-SHA action (CICD-08 obligation) for a one-line percentage comparison this codebase's own conventions already avoid — rejected, matches D-01 |
| `-coverpkg` package-list exclusion (recommended) | Post-process `coverage.out` with `grep -v '/internal/db/sqlc/'` before `go tool cover -func` | Post-processing is a second file-mutation step that must stay in sync with the package path string by hand and doesn't also solve D-05 (getting `cmd/server` measured requires `-coverpkg` regardless, so once `-coverpkg` is in use, using it to *also* exclude sqlc is strictly simpler than adding a second grep step) |
| `coverage.include` set explicitly (recommended) | Leave `coverage.include` unset and rely on Vitest 4's "only files imported during test run" default | Silently excludes any zero-test file (like `history.tsx` today) from the denominator entirely, producing a 70%-passing number that doesn't reflect true first-party coverage — defeats D-06's intent |

**Installation:**
```bash
cd web && pnpm add -D @vitest/coverage-v8@4.1.10
```

**Version verification:** `npm view @vitest/coverage-v8@4.1.10 version` returns `4.1.10`; `npm view vitest@4.1.10 version` returns `4.1.10` — exact version match confirmed this session against `web/package.json`'s already-pinned `vitest` version.

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|--------------|---------|-------------|
| `@vitest/coverage-v8` | npm | First published 2023-06-06; `4.1.10` published 2026-07-06 [VERIFIED: `npm view @vitest/coverage-v8 time.created`] | ~34,000,000/week [VERIFIED: `gsd-tools query package-legitimacy check`] | `github.com/vitest-dev/vitest` (monorepo, official) | OK | Approved |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

No `postinstall` script present (`npm view @vitest/coverage-v8 scripts.postinstall` returns empty) [VERIFIED: npm registry, this session]. Package name was discovered via D-07's locked decision (user/CONTEXT-supplied, not researcher-guessed), then independently confirmed to exist, version-match, and carry a clean legitimacy verdict — no `[ASSUMED]` tag needed for this package.

## Architecture Patterns

### System Architecture Diagram

```
git push / PR
      │
      ▼
┌─────────────────────────────── parallel tier ───────────────────────────────┐
│  vet   lint   test(+coverage gate)   gitleaks   trivy-fs   frontend-test(+coverage gate)  │
└───────────────────────────────────────────────────────────────────────────┘
      │  (any job failing blocks the next tier — existing `needs:` behavior)
      ▼
  build-scan   needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]  ← frontend-test added here (D-11)
      │
      ▼
   release   needs: [build-scan]

Inside `test` job:
  make test-integration
    └─ go test ./... -race -count=1 -p 1 -coverprofile=coverage.out -coverpkg=<pkgs minus sqlc>
         └─ go tool cover -func=coverage.out | grep total: | awk '{...}'  → strip "%"
              └─ awk -v cov=... -v thresh=80 'BEGIN{exit !(cov+0>=thresh)}'  → exit 1 on fail

Inside `frontend-test` job:
  pnpm test  (== `vitest run`, unchanged CI step per D-08)
    └─ vitest run reads vitest.config.ts's coverage block
         └─ @vitest/coverage-v8 instruments coverage.include-matched files
              └─ coverage.thresholds check → vitest run itself exits 1 on fail (no separate script)
```

### Recommended Project Structure

No new files — both gates extend existing artifacts:
```
Makefile                      # test-integration target gets -coverprofile/-coverpkg flags + gate step
.github/workflows/full-pipeline.yml  # test job: add coverage-gate step; frontend-test job: no new step needed (D-08); build-scan needs: gets frontend-test appended
web/vitest.config.ts          # coverage: { provider, include, exclude, thresholds } block added
web/package.json              # devDependencies gets @vitest/coverage-v8
```

### Pattern 1: Backend coverage gate as a Makefile-owned step

**What:** `test-integration` produces `coverage.out`; a new `Makefile` target (or an inline CI step calling the same commands) parses and gates it, so `make test-integration` stays usable standalone for local iteration and CI just calls the same target plus one gate step.
**When to use:** Any time the gate logic should be runnable identically in CI and locally (this repo's established convention — every other CI check maps to a `make` target or a single tool invocation).
**Example:**
```makefile
# Source: go help testflag (-coverpkg default), verified this session via
# `go list ./...` and `go list ./... | grep -v /internal/db/sqlc | paste -sd, -`
COVER_PKGS := $(shell go list ./... | grep -v /internal/db/sqlc | paste -sd, -)

test-integration: db-up
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./... -race -count=1 -p 1 \
		-coverprofile=coverage.out -coverpkg=$(COVER_PKGS)

test: test-integration

coverage-gate:
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total: | awk '{print substr($$3, 1, length($$3)-1)}'); \
	echo "Backend coverage: $${COVERAGE}%"; \
	awk -v cov="$$COVERAGE" -v thresh=80 'BEGIN { if (cov+0 < thresh) { print "FAIL: below 80% threshold"; exit 1 } else { print "PASS" } }'
```
`internal/db/sqlc` has zero `_test.go` files [VERIFIED: `find internal/db/sqlc -name "*_test.go"` returned nothing, this session] and is filtered out of `COVER_PKGS`; `cmd/server` has zero `_test.go` files today too [VERIFIED: `ls cmd/server/` returned only `main.go`, this session] but stays IN `COVER_PKGS` — so it appears in the profile at 0% and pulls the aggregate down until D-05's intent (real tests for it) is honored.

### Pattern 2: Frontend coverage.include as the denominator, not an afterthought

**What:** Because Vitest 4 dropped `coverage.all`, `coverage.include` is not optional boilerplate here — it is the only way to make untested files count against the 70% threshold at all.
**When to use:** Always, whenever the intent (per D-06) is "measure all first-party source, carve out generated/vendor pieces" rather than "measure whatever tests happen to import."
**Example:**
```typescript
// Source: vitest.dev/config/coverage (fetched this session) + vitest.dev/guide/coverage;
// coverage.all removal in Vitest 4 cross-confirmed via vitest-dev/vitest#6956.
coverage: {
  provider: "v8",
  // Without this, only files imported during the test run are measured
  // (coverage.all was removed in Vitest 4) -- an entirely untested route
  // like app/routes/history.tsx would simply never appear in the report.
  include: ["app/**/*.{ts,tsx}"],
  exclude: [
    "app/components/ui/**",        // shadcn primitives, not first-party (D-06)
    "app/lib/test/**",              // test-only helpers (routeStub.tsx)
    "**/*.test.{ts,tsx}",
  ],
  thresholds: {
    lines: 70,
    functions: 70,
    branches: 70,
    statements: 70,
  },
},
```
Route-typegen output (`.react-router/types/**`) [VERIFIED: `web/tsconfig.json:6` — `".react-router/types/**/*"` is a `tsconfig.json` `include` entry, confirming this is where React Router's generated types actually live] does not need its own `coverage.exclude` glob: it sits outside `include`'s `app/**` glob entirely, so it is never matched in the first place.

### Anti-Patterns to Avoid

- **Copying a `coverage.all: true` example from a pre-2026 blog post:** that option no longer exists in Vitest 4 and either does nothing or fails config validation — use `coverage.include` instead (see Pitfall 3).
- **Passing `-coverpkg=./...` unfiltered:** would pull `internal/db/sqlc` back into the measured set, reversing D-04.
- **Grepping `coverage.out` post-hoc as the primary mechanism:** works, but is a second maintenance surface (the exact line-prefix format `path:startline.col,endline.col numstmt count` must be matched correctly) when `-coverpkg` already solves both D-04 and D-05 in one flag.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Frontend threshold enforcement / exit-code propagation | A separate Node script that parses Vitest's coverage JSON and calls `process.exit(1)` | Vitest's built-in `coverage.thresholds` (D-08) | `vitest run` already exits 1 on an unmet threshold [CITED: cross-verified web search, Vitest 1.x+ behavior, current at 4.1.10] — a wrapper script would duplicate logic Vitest already owns and risks drifting out of sync with its own coverage numbers |
| Coverage percentage float comparison | A one-off Python/Node script just to compare two floats | `awk` (already on every CI runner and this dev machine) | The comparison is a single boolean; pulling in a script runtime for it does not match D-01's "hand-roll simple mechanical checks" convention |

**Key insight:** Both ecosystems' native tooling (`go tool cover`, Vitest's `coverage.thresholds`) already does everything CICD-11/12 need — the actual engineering risk in this phase is exclusion-scope correctness (getting `-coverpkg` and `coverage.include` right), not building new gate machinery.

## Common Pitfalls

### Pitfall 1: Assuming `go test ./... -coverprofile=coverage.out` alone measures cross-package calls

**What goes wrong:** A plan that adds `-coverprofile=coverage.out` without `-coverpkg` will correctly exclude `internal/db/sqlc` (D-04, for free) but will ALSO fail to measure `cmd/server` — because `cmd/server` has zero `_test.go` files, it never appears in the profile under self-package-only coverage, silently violating D-05's intent (it doesn't get excluded loudly, it just vanishes, and the aggregate ends up *higher* than it should be, not lower).
**Why it happens:** `go help testflag`'s `-coverpkg` documentation: "The default is for each test to analyze only the package being tested." [CITED: go.dev / go help testflag, cross-verified via WebSearch this session] — this is easy to miss since `-coverprofile` alone looks sufficient.
**How to avoid:** Always pair `-coverprofile` with an explicit `-coverpkg` package list (see Pattern 1) built from `go list ./...` minus `internal/db/sqlc`.
**Warning signs:** Baseline measurement comes back suspiciously high (e.g. >90%) on the very first run before any gap-closing work — a strong sign `cmd/server` (and possibly `internal/logging`, `internal/webassets`, both also zero-test-file packages today [VERIFIED: `find internal/webassets internal/logging internal/testutil -name "*_test.go"` returned nothing, this session]) never actually entered the profile.

### Pitfall 2: Underestimating `cmd/server`'s gap-closing cost

**What goes wrong:** Treating this phase as "just add a gate step" and discovering mid-plan that `cmd/server/main.go` (205 lines [VERIFIED: `wc -l cmd/server/main.go`, this session], real branching logic — config load, migration run, pool connect, graceful shutdown on `ctx.Done()` vs. serve-error paths) has zero tests and, once measured via `-coverpkg`, drags the aggregate down non-trivially.
**Why it happens:** `main()`/`run()` in a single-binary Go service is exactly the kind of code that's easy to leave untested because it's "just wiring" — but D-05 explicitly rejects excluding it, and this codebase's own internal non-test source is ~4,969 lines [VERIFIED: `find internal -name "*.go" ! -name "*_test.go" ! -path "*/sqlc/*" | xargs wc -l`, this session] against `cmd/server`'s 205 — roughly 4% of the codebase by line count, currently at 0% coverage.
**How to avoid:** Budget a dedicated task/plan for `cmd/server` coverage explicitly (not folded silently into "backend gate step") — likely a real-Postgres integration test that invokes `run()` (or an extracted, more directly testable seam) with `signal.NotifyContext`-driven shutdown, exercising at minimum: successful boot + graceful SIGTERM shutdown, and a config-load-failure early-return path.
**Warning signs:** Baseline measurement (once `-coverpkg` is correctly widened) lands meaningfully under 80% specifically because of `cmd/server`'s 0%, not because of broadly-distributed small gaps.

### Pitfall 3: `coverage.all` removal in Vitest 4 silently deflates the frontend denominator

**What goes wrong:** A `vitest.config.ts` coverage block copied from a tutorial or older Vitest docs snapshot that sets `all: true` either does nothing (unknown key) or fails Vitest's config validation, and omitting `coverage.include` entirely means only files actually `import`ed by a test file are measured at all — currently `app/routes/history.tsx` has no test file [VERIFIED: `find app -name "*.test.tsx" -o -name "*.test.ts"` under `web/`, this session, lists only `EventCard.test.tsx`, `HistoryFilters.test.tsx`, `PreferenceToggles.test.tsx`, `SearchBox.test.tsx`, `watchlist.test.tsx` — no `history.test.tsx`] and, unless something else in the test suite happens to import it, it will not appear in the coverage report at all under Vitest 4's default, letting a 70% "pass" hide an entire untested route.
**Why it happens:** `coverage.all` existed in Vitest 1–3 and was removed in Vitest 4 (tracked in `vitest-dev/vitest#6956`, cross-confirmed via WebSearch this session); default behavior changed from "measure every `include`-matched file" to "measure only files touched by a test run," and most still-indexed blog content predates this change.
**How to avoid:** Set `coverage.include: ["app/**/*.{ts,tsx}"]` explicitly (Pattern 2) so `history.tsx` and any other currently-untested first-party file is forced into the denominator, then measure the real baseline before assuming 70% is already met.
**Warning signs:** A first-pass baseline measurement that comes back suspiciously close to 100% with almost no test files existing — a sign `coverage.include` is unset and the report is only reflecting the handful of files the existing Phase 8 tests happen to import.

### Pitfall 4: Bash integer comparison silently mis-gating a float percentage

**What goes wrong:** A gate written as `if [ "$COVERAGE" -lt 80 ]` fails at runtime (or worse, behaves unpredictably) the first time `$COVERAGE` is a decimal like `82.3`, since POSIX `[` / `test` only supports integer comparison.
**Why it happens:** `go tool cover -func`'s total line reports coverage to one decimal place (e.g. `total: (statements) 82.3%`) [CITED: cross-verified web search on `go tool cover -func` output format, this session], so the extracted value is virtually never a whole number.
**How to avoid:** Use `awk` for the comparison (Pattern 1) — `awk -v cov="$COVERAGE" -v thresh=80 'BEGIN{exit !(cov+0>=thresh)}'` handles floats natively and is already the tool used to extract the percentage in the same step.
**Warning signs:** A CI run that errors with `integer expression expected` instead of a clean pass/fail message.

## Code Examples

### Full backend gate step (CI-facing)

```yaml
# Source: pattern derived from go help testflag + go tool cover docs,
# verified this session against this repo's actual package list.
- name: Run integration tests with coverage
  run: |
    go test ./... -race -count=1 -p 1 \
      -coverprofile=coverage.out \
      -coverpkg=$(go list ./... | grep -v /internal/db/sqlc | paste -sd, -)
- name: Backend coverage gate (80%)
  run: |
    COVERAGE=$(go tool cover -func=coverage.out | grep total: | awk '{print substr($3, 1, length($3)-1)}')
    echo "Backend coverage: ${COVERAGE}%"
    awk -v cov="$COVERAGE" -v thresh=80 'BEGIN { if (cov+0 < thresh) { print "FAIL: coverage below 80% threshold"; exit 1 } print "PASS" }'
```
Per D-11/D-02 this should extend the existing `test` job's single `Run integration tests` step (calling `make test-integration` with the flags folded into the `Makefile` target, per Pattern 1) rather than adding new standalone CI steps that bypass the `Makefile` — keeping `make test`/`make test-integration` usable identically by a human running it locally.

### Full frontend coverage config

```typescript
// web/vitest.config.ts -- coverage block to add inside the existing `test: {}` object.
// Source: vitest.dev/config/coverage (this session) + coverage.all removal
// cross-confirmed via vitest-dev/vitest#6956.
test: {
  environment: "jsdom",
  setupFiles: ["./vitest.setup.ts"],
  mockReset: true,
  coverage: {
    provider: "v8",
    include: ["app/**/*.{ts,tsx}"],
    exclude: [
      "app/components/ui/**",
      "app/lib/test/**",
      "**/*.test.{ts,tsx}",
    ],
    thresholds: {
      lines: 70,
      functions: 70,
      branches: 70,
      statements: 70,
    },
  },
},
```
`pnpm test` (== `vitest run`, D-08) does not need a `--coverage` flag added to the CI step — once `coverage` is configured in `vitest.config.ts` with `thresholds` present, `vitest run` collects coverage and enforces thresholds automatically [CITED: cross-verified web search, Vitest coverage docs, this session]. Verify this locally once implemented: if it does not, `web/package.json`'s `test` script (`"test": "vitest run"`) would need `--coverage` appended.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Vitest `coverage.all: true` to force every `include`-matched file into the report regardless of test execution | `coverage.all` removed; only files imported during the test run are measured unless `coverage.include` explicitly widens the set | Vitest 4 (this project pins `4.1.10`) [CITED: `vitest-dev/vitest#6956`, cross-verified web search this session] | Any config or blog example written before Vitest 4 that sets `all: true` is stale; `coverage.include` is now load-bearing for an honest denominator, not optional |

**Deprecated/outdated:**
- `coverage.all` (Vitest config key): removed in Vitest 4; do not follow tutorials referencing it.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | ubuntu-latest GitHub Actions runners ship GNU `awk`/`gawk` and GNU `paste` preinstalled | Standard Stack (Supporting), Code Examples | Low — extremely well-established fact about the runner image; if wrong, the gate step would fail loudly on first CI run (`awk: command not found`) rather than silently mis-gating, and is trivially fixed by adding `apt-get install gawk` |
| A2 | `vitest run` does not require a `--coverage` CLI flag once `coverage.thresholds` is set in `vitest.config.ts` — coverage collection triggers from config presence alone | Code Examples | Medium — if wrong, `pnpm test` would silently run without coverage instrumentation and the threshold check would never fire, producing a false-pass CI job; must be verified locally the first time this is implemented (run `pnpm test` and confirm a coverage summary prints) before trusting the gate in CI |

## Open Questions

1. **Does `vitest run` (no `--coverage` flag) actually invoke coverage collection when `coverage` is configured but `test.coverage.enabled` is not explicitly set to `true`?**
   - What we know: Vitest's coverage guide shows `"coverage": "vitest run --coverage"` as a *separate* package.json script in its own example, which could mean `--coverage` (or `coverage.enabled: true`) is required to activate collection even when `coverage.thresholds` is configured.
   - What's unclear: Whether merely defining `coverage.thresholds`/`coverage.provider` in config is sufficient to turn coverage collection ON for the plain `test` script (`"test": "vitest run"`, unchanged per D-08), or whether an explicit `coverage: { enabled: true }` key or a `--coverage` CLI flag is additionally required.
   - Recommendation: The planner/executor should verify this empirically as the very first implementation step — add the config, run `pnpm test` locally, and confirm coverage output actually appears before writing any gap-closing tests. If it does not activate automatically, add `enabled: true` to the `coverage` block (a one-line, low-risk fix) rather than changing the CI step, keeping D-08's "no separate check script / same `pnpm test` step" intact.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Go toolchain | Backend coverage instrumentation | ✓ | go1.26.5 (local); CI uses `go.mod`'s `go 1.26` via `actions/setup-go` | — |
| GNU awk | Backend gate float comparison | ✓ (local) | 5.0.0; CI runner assumed preinstalled [ASSUMED, see A1] | — |
| GNU paste (coreutils) | `-coverpkg` list construction | ✓ (local) | 8.32; CI runner assumed preinstalled [ASSUMED, see A1] | — |
| Docker (for `make db-up`) | Real-Postgres integration tests (D-02) | ✓ | 29.6.2 | — |
| Node.js | Frontend coverage tooling | ✓ | v22.21.1 (matches CI's `node-version: '22'`) | — |
| pnpm | Frontend package install/test | ✓ | 11.8.0 (matches CI's `pnpm/action-setup` `version: 11`) | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — everything this phase needs is already present locally and in the existing CI pipeline.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework (backend) | Go stdlib `testing` + `go tool cover`, no assertion library [VERIFIED: `.planning/codebase/TESTING.md` §Test Framework, this session] |
| Framework (frontend) | Vitest 4.1.10 + `@vitest/testing-library/react` (Phase 8), `@vitest/coverage-v8` added this phase |
| Config file (backend) | `Makefile` `test-integration` target (no separate coverage config file) |
| Config file (frontend) | `web/vitest.config.ts` |
| Quick run command | Backend: `go test ./... -short -race -count=1` (no coverage, per D-02 — coverage only runs on the full `test-integration` path). Frontend: `pnpm test` (already collects coverage once configured) |
| Full suite command | Backend: `make test-integration` (now with `-coverprofile`/`-coverpkg`). Frontend: `pnpm test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| CICD-11 | `test` job fails when aggregate Go coverage < 80% | CI gate (not a unit test — an infrastructure behavior) | `make test-integration` then the coverage-gate step (Pattern 1) | ❌ — new Makefile target/CI step this phase |
| CICD-12 | `frontend-test` job fails when aggregate frontend coverage < 70% | CI gate via Vitest built-in threshold | `pnpm test` (unchanged script, D-08) | ❌ — new `vitest.config.ts` coverage block this phase |

Both requirements are inherently CI-infrastructure-behavior, not testable via a unit test asserting on application code — the correct verification is: (1) confirm the gate step fails when a synthetic coverage.out/report is deliberately under threshold (can be smoke-tested locally by temporarily lowering the threshold constant and confirming pass/fail flips), and (2) confirm `build-scan` does not run when either upstream job fails (verified by `needs:` graph inspection, not a runtime test).

### Sampling Rate

- **Per task commit:** Backend gap-closing tests run via `go test ./... -short -race -count=1` (fast) during development; the full coverage gate only needs verifying once per plan, not per commit, since it requires the real-Postgres `test-integration` path.
- **Per wave merge:** `make test-integration` (full suite + coverage) and `pnpm test` (frontend, full suite + coverage) both green.
- **Phase gate:** Both gates enforced in `full-pipeline.yml` before `/gsd-verify-work`; a locally-reproduced failing-then-passing gate transition (lower threshold, confirm fail; restore to 80/70, confirm pass) is the closest thing to a "test of the gate itself."

### Wave 0 Gaps

- [ ] `Makefile` — `COVER_PKGS` variable + `-coverprofile`/`-coverpkg` flags on `test-integration`, plus a `coverage-gate` (or equivalently-named) target
- [ ] `web/vitest.config.ts` — `coverage` block (provider/include/exclude/thresholds)
- [ ] `web/package.json` — `@vitest/coverage-v8@4.1.10` devDependency
- [ ] Baseline measurement for both sides — must run BEFORE the threshold is committed as an enforced gate, per CONTEXT.md success criterion 4 ("both starting baselines are measured and recorded before enforcement")
- [ ] Likely gap-closing tests for `cmd/server` (0 tests today, real branching logic, ~4% of backend LOC) and `app/routes/history.tsx` (0 tests today) — budget as explicit tasks, not contingency

*(Framework itself is already installed and working per Phase 1 (backend) and Phase 8 (frontend) — no framework-install gap, only config/measurement gaps.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|-------------------|
| V2 Authentication | No | This phase touches only CI configuration and test tooling, no auth surface |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A |
| V5 Input Validation | No | No user-facing input surface introduced |
| V6 Cryptography | No | N/A |

### Known Threat Patterns for this phase's stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| Supply-chain risk from a new npm dependency (`@vitest/coverage-v8`) | Tampering | Package Legitimacy Audit (above) — verdict OK, official `vitest-dev` monorepo package, no postinstall script, version-pinned in `web/package.json` matching `pnpm-lock.yaml` |
| CI workflow YAML injection via untrusted input | Tampering | Not applicable — D-03 keeps the coverage report log-only (no PR comment, no untrusted string interpolated into a `run:` step); this phase adds no new `${{ github.event.* }}` interpolation to `full-pipeline.yml` |
| A coverage-gate bypass (e.g. gate step accidentally set to always-pass, or `-coverpkg` silently widened to include `sqlc` again in a future edit) | Tampering | Not a runtime security threat for this single-operator portfolio project, but worth a plan-checker note: the `coverage-gate` step's `exit 1` path should be exercised at least once (temporarily lower the threshold, confirm the job goes red) before trusting it in CI long-term |

No new secrets, auth surface, or externally-reachable endpoints are introduced by this phase — CICD-11/12 are internal CI-tooling requirements with no ASVS-relevant runtime attack surface.

## Sources

### Primary (HIGH confidence)
- `npm view @vitest/coverage-v8@4.1.10 version` / `npm view vitest@4.1.10 version` — this session, exact version match confirmed
- `gsd-tools query package-legitimacy check --ecosystem npm @vitest/coverage-v8` — verdict OK, this session
- `go list ./...` / `go list ./... | grep -v /internal/db/sqlc | paste -sd, -` — this session, exact package list confirmed against this repo
- Direct file reads this session: `Makefile`, `.github/workflows/full-pipeline.yml`, `web/vitest.config.ts`, `web/package.json`, `web/tsconfig.json`, `cmd/server/main.go`, `.planning/codebase/TESTING.md`

### Secondary (MEDIUM confidence)
- `go.dev` / `go help testflag` (`-coverpkg` default behavior) — WebSearch, cross-verified across two independent queries this session
- `vitest.dev/config/coverage`, `vitest.dev/guide/coverage` — WebFetch this session, current `4.1.10`-versioned docs
- `github.com/vitest-dev/vitest` issue #6956 (`coverage.all` removal) — WebSearch this session

### Tertiary (LOW confidence)
- None retained — all WebSearch findings that could be cross-verified against an official source were promoted to Secondary; no unverified single-source claims are presented as fact in this document

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — exact version match verified against npm registry, package legitimacy verdict OK
- Architecture: HIGH — `-coverpkg` and `coverage.include` mechanisms verified against official docs and this repo's actual package/file structure this session
- Pitfalls: HIGH — both headline pitfalls (Go self-package coverage default, Vitest 4's `coverage.all` removal) are grounded in official docs (`go help testflag`, `vitest-dev/vitest#6956`) plus this session's direct inspection of this repo's actual untested files (`cmd/server`, `history.tsx`, `internal/logging`, `internal/webassets`)

**Research date:** 2026-08-13
**Valid until:** 2026-09-12 (30 days — stable tooling, but Vitest's fast release cadence and this project's still-unmeasured baselines warrant re-checking before a much-later re-plan)
