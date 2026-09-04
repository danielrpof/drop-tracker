---
phase: 16-rollback-safe-migrations
plan: 03
subsystem: ci
tags: [ci-tooling, static-analysis, sql, git, migrations, golangci-lint]

# Dependency graph
requires:
  - phase: 16-rollback-safe-migrations (plan 02)
    provides: cmd/migration-check's scan pipeline, two finding classes (backward-incompatible/unsafe-forward), allow-destructive annotation grammar + parseAnnotation/shouldSuppress -- this plan builds on that package without restructuring it
provides:
  - "--mode=changed-files: diffRange diff-base selection (PR three-dot merge-base, push two-dot before..sha, all-zeroes/unreachable-before merge-base fallback against origin/main) + migrations_changed/migration_files GitHub Actions outputs + the modified-released-migration immutability hard error"
  - "gitShow package-var seam behind readAtTag (tag shape allowlist + fixed three-glob path allowlist), argv-slice exec.Command(\"git\", \"show\", ...) only"
  - "extractReferences: previous-release queries/*.sql -> high/low confidence (table, column) reference sets, covering INSERT/ON CONFLICT/RETURNING column lists, EXCLUDED.col and table.col in DO UPDATE SET, alias-resolved qualified refs, single-table SELECT */RETURNING * schema expansion, CTE exclusion, subquery flattening, and sqlc.arg/narg/@param collection"
  - "classCrossRef finding class wired into run's scan path: a DROP/RENAME COLUMN or DROP/RENAME TABLE still referenced by the previous release's queries turns the guard red regardless of a well-formed allow-destructive annotation"
affects: [16-04-ci-wiring-n1-boot-job, 16-05-migrations-readme]

# Actuals (#2632)
actuals:
  tokens: 19292
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Manual keyword-position tokenizer (findFromJoinTables) instead of one combined 'FROM/JOIN + table + optional trailing alias' regex -- the combined form let the alias group swallow the next clause's own FROM/JOIN keyword, silently dropping the second table from an adjacent 'FROM t\\nJOIN u' pair"
    - "Hardcoded queries/*.sql file list (prevReleaseQueryFiles) instead of a local filesystem glob -- gives tests a stable, known path to target with a failing gitShow stub and avoids a CWD mismatch between `go run` at the repo root (production) and `go test`'s package-directory convention"
    - "path.Match (not filepath.Match) for every git-path allowlist/glob check -- git always emits forward-slash paths regardless of host OS; filepath.Match's separator-awareness is platform-dependent and would let `*` cross `/` on Windows"
    - "Cross-reference findings are pulled out of a file's finding list BEFORE the suppression predicate is applied, then appended back after -- shouldSuppress only ever sees the remaining findings, so a classCrossRef finding is structurally never suppressible"

key-files:
  created:
    - cmd/migration-check/testdata/prevrelease/*.sql (6 fixtures: artists, case_folding, events, low_confidence_join, params, schema_qualified)
    - cmd/migration-check/testdata/prevref_rename_column.sql
    - cmd/migration-check/testdata/prevref_drop_column_no_ref_annotated.sql
    - cmd/migration-check/testdata/prevref_low_confidence_annotated.sql
  modified:
    - cmd/migration-check/main.go
    - cmd/migration-check/main_test.go
    - cmd/migration-check/testdata/mixed_findings.sql
    - cmd/migration-check/testdata/mixed_findings.golden.txt

key-decisions:
  - "Push event diff range is the literal two-dot <before>..<sha> form, not RESEARCH's example three-dot <before>...<sha> -- the plan's own must_haves.truths locked two-dot for push (only pull_request and the zero/unreachable-before fallback use three-dot merge-base form); implemented exactly as the plan specifies since PLAN.md is the authoritative contract over RESEARCH.md's illustrative bash."
  - "D-15 schema-column data (for SELECT */RETURNING * expansion) is gathered best-effort from a local internal/db/migrations/*.up.sql glob relative to CWD, not from a second git-ls-tree-style enumeration seam -- none of Task 3's required deterministic-red positions (explicit column lists, ON CONFLICT targets, qualified refs) need star expansion, so a glob miss (e.g. under `go test`'s package-directory CWD) degrading to 'no expansion' is safe and keeps the surface minimal."
  - "RENAME COLUMN's cross-reference lookup splits finding.object on \" -> \" to recover just the OLD column name before checking the high-confidence set -- the DDL scanner's own describe()-friendly object string is 'old -> new' combined, which would never match a stored single-column reference verbatim."
  - "mixed_findings.sql/mixed_findings.golden.txt extended with a third statement (DROP COLUMN notified_at, referenced only by a stubbed events.sql in the golden test) so the golden file demonstrates all three finding classes -- backward-incompatible, unsafe-forward, and prev-release-reference -- in one run, satisfying Task 3's explicit 'three distinct remediation paragraphs' acceptance criterion without disturbing the pre-existing DROP COLUMN release_type / ADD COLUMN foo NOT NULL statements' own classes."

patterns-established:
  - "D-15 conservatism split: only a HIGH-confidence reference position (explicit column list, ON CONFLICT target, RETURNING item, qualified alias.col/table.col, single-table star expansion, or a bare column when the query has exactly one real table) can produce a deterministic, non-overridable red. A LOW-confidence position (bare unqualified column in a multi-table query) never reds -- it is left to the n1-boot behavioural backstop, since this check has no allow-destructive escape hatch and a false red here is unrecoverable."
  - "classCrossRef bypasses shouldSuppress structurally, not via a special-cased conditional: findings are partitioned into crossRefFindings/otherFindings before the existing annotation-suppression switch runs, then crossRefFindings are appended back unconditionally after it."

requirements-completed: []

coverage:
  - id: D1
    description: "--mode=changed-files computes the correct diff range for a pull_request, a push with a reachable before, a push with an all-zeroes before, and a push with an unreachable before (falling back to the merge-base against origin/main in the last two cases) -- closing the S2 direct-to-main no-op"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestDiffRange"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestChangedFiles_AddedAndModified"
        status: pass
      - kind: other
        ref: "go run ./cmd/migration-check --mode=changed-files --event-name=push --before=0000000000000000000000000000000000000000 --sha=\"$(git rev-parse HEAD)\" -> exits 0, prints migrations_changed="
        status: pass
    human_judgment: false
  - id: D2
    description: "Every ref-shaped input (--base-ref, --before, --sha) is rejected before it can reach a git argv element when it is not shape-valid; every git invocation is exec.Command(\"git\", ...) argv-slice form, never sh -c"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestDiffRange (rejection subtests)"
        status: pass
      - kind: other
        ref: "grep -c 'exec.Command(\"git\"' cmd/migration-check/main.go == 3; grep -cE 'exec\\.Command\\(\"(sh|bash|cmd)\"' cmd/migration-check/main.go == 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "A modified already-released *.up.sql file is its own hard error naming immutability, independent of whether its SQL is destructive"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestModifiedReleasedMigration"
        status: pass
    human_judgment: false
  - id: D4
    description: "gitShow's tag and path arguments are gated (tag shape allowlist, fixed three-glob path allowlist) before the subprocess is ever spawned; a rejected path never invokes the stub"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestGitShow_RejectsPathOutsideAllowlist"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestGitShow_RejectsMalformedTag"
        status: pass
    human_judgment: false
  - id: D5
    description: "extractReferences builds the previous release's high/low confidence (table, column) reference set from queries/*.sql: explicit column lists, ON CONFLICT targets, EXCLUDED/table-qualified DO UPDATE SET columns, alias-resolved qualified refs, single-table star expansion, CTE exclusion, subquery flattening, sqlc param collection, and Postgres-correct identifier case folding"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestExtractReferences"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestExtractReferences_IdentifierCaseFolding"
        status: pass
    human_judgment: false
  - id: D6
    description: "A DROP COLUMN, RENAME COLUMN, DROP TABLE, or RENAME TABLE whose object is still high-confidence-referenced by the previous release's queries turns the guard red even with a well-formed allow-destructive annotation; the annotation's reason is echoed alongside a message stating it cannot override a live reference"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestPrevReleaseCrossRef_AnnotationCannotOverride"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestPrevReleaseCrossRef_RenameColumnIsRed"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestPrevReleaseCrossRef_DropTableIsRed"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestPrevReleaseCrossRef_RenameTableIsRed"
        status: pass
    human_judgment: false
  - id: D7
    description: "A low-confidence-only reference never cross-reference reds, and an unreferenced column's plain finding still gets suppressed normally by a valid annotation"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestPrevReleaseCrossRef_LowConfidenceIsNotRed"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestPrevReleaseCrossRef_NoReferenceStillPlainFindingSuppressedByAnnotation"
        status: pass
    human_judgment: false
  - id: D8
    description: "An empty --prev-tag (true bootstrap) skips the cross-reference sub-check with a printed notice and never affects the exit code; a supplied tag whose queries/*.sql cannot be read is a hard error naming the tag and the file"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestPrevReleaseCrossRef_NoPriorTagSkips"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestPrevReleaseCrossRef_GitShowFailureIsRed"
        status: pass
    human_judgment: false
  - id: D9
    description: "The real repo's shipped internal/db/migrations/*.up.sql set scores zero findings when cross-referenced against the real v1.7.0 tag -- no false positives on additive history"
    requirement: "MGRT-01"
    verification:
      - kind: other
        ref: "go run ./cmd/migration-check --mode=scan --files=<all internal/db/migrations/*.up.sql> --prev-tag=v1.7.0 -> EXIT=0"
        status: pass
    human_judgment: false

duration: ~20min active (commit-to-commit span; excludes required-reading/research time)
completed: 2026-09-04
status: complete
---

# Phase 16 Plan 03: cmd/migration-check changed-files mode, gitShow seam, and D-15 cross-reference Summary

**`cmd/migration-check` now computes the correct diff base for every GitHub Actions event shape (closing the direct-to-main no-op) and deterministically reds a migration that drops or renames an object the previously-released binary still queries — even through a well-formed `allow-destructive` annotation.**

## Performance

- **Duration:** ~20 min active (first commit 12:05, last commit 12:24 local time), across three sequential task commits
- **Started:** 2026-09-04T12:05:24-05:00 (Task 1's commit)
- **Completed:** 2026-09-04T12:24:46-05:00 (Task 3's commit)
- **Tasks:** 3 (all `type="auto" tdd="true"`, fully autonomous, no checkpoints)
- **Files modified:** 13 (2 Go source files modified across all 3 tasks; 11 SQL/golden testdata files created or modified)

## Accomplishments
- `--mode=changed-files` computes the git diff range per event shape: `pull_request` uses `origin/<base-ref>...HEAD` (three-dot merge-base), `push` uses the literal `<before>..<sha>` two-dot range, and an all-zeroes or unreachable `before` falls back to the merge-base against `origin/main` — closing the S2 gap where a destructive migration pushed straight to `main` was previously invisible to the guard
- Every ref-shaped CLI input (`--base-ref`, `--before`, `--sha`) is gated through a shape allowlist before it can reach a git argv element; a modified already-released `*.up.sql` file is its own hard error naming immutability
- `gitShow` package-var seam behind `readAtTag`, gating the tag (same shape allowlist as the `allow-destructive` annotation) and the path (fixed three-glob allowlist: `queries/*.sql`, `internal/db/migrations/*.up.sql`, `internal/db/sqlc/*.go`) before the subprocess is ever spawned
- `extractReferences` parses a previous-release `queries/*.sql` file's sqlc query blocks into high/low confidence `(table, column)` reference sets, covering every construct RESEARCH.md's D-15 section enumerated: explicit `INSERT`/`ON CONFLICT`/`RETURNING` column lists, `EXCLUDED.col`/`table.col` in `DO UPDATE SET`, alias-resolved qualified references, single-table `SELECT *`/`RETURNING *` schema expansion, CTE-name exclusion from the real-table set, subquery flattening, and `sqlc.arg`/`sqlc.narg`/`@param` collection into a separate never-a-column bag
- A new `classCrossRef` finding class is wired into `run`'s scan path, after `classify` and before the suppression predicate: a `DROP COLUMN`, `RENAME COLUMN`, `DROP TABLE`, or `RENAME TABLE` whose object hits the high-confidence reference set turns the guard red **regardless of a well-formed `allow-destructive` annotation** — the annotation's tag/reason are echoed into the message alongside an explicit statement that it cannot override a live reference
- Confirmed against the real repo: the full set of shipped `internal/db/migrations/*.up.sql` files scores zero findings when cross-referenced against the real `v1.7.0` tag — no false positives on genuinely additive history
- 44 passing test cases in `cmd/migration-check` by the end of the plan (up from 23 at the start), zero `FAIL`/`SKIP`

## Task Commits

Each task was committed atomically:

1. **Task 1: changed-files mode — diff-base selection, ref validation, and the immutable-migration error** - `49e0d93` (feat)
2. **Task 2: Previous-release query identifier extraction behind the gitShow seam** - `4d1399a` (feat)
3. **Task 3: The D-15 cross-reference — an annotation cannot wave through a live N-1 break** - `c0a92d5` (feat)

**Plan metadata:** recorded in this commit (docs: complete plan)

## Files Created/Modified
- `cmd/migration-check/main.go` - `diffRange`, `commitExists`/`gitDiffNames` seams, `runChangedFiles` (Task 1); `gitShow` seam, `readAtTag`, `extractReferences`/`extractBlockReferences`, `findFromJoinTables` tokenizer, `parseSchemaColumns`, `prevReleaseRefs` (Task 2); `classCrossRef` finding class, `buildPrevReleaseRefs`, `crossReferenceFinding`, `runScan` rewiring, `--prev-tag` flag (Task 3)
- `cmd/migration-check/main_test.go` - `TestDiffRange`/`TestChangedFiles_*`/`TestModifiedReleasedMigration` (Task 1); `TestGitShow_*`/`TestExtractReferences*`/`TestFindFromJoinTables_*`/`TestParseSchemaColumns` (Task 2); `TestPrevReleaseCrossRef_*` (Task 3)
- `cmd/migration-check/testdata/prevrelease/*.sql` - 6 fixtures built from the real four query files' distinctive statements (Task 2)
- `cmd/migration-check/testdata/prevref_*.sql` - 3 new Task 3 fixtures (rename-column, unreferenced-column-annotated, low-confidence-annotated)
- `cmd/migration-check/testdata/mixed_findings.sql` / `.golden.txt` - extended with a third statement so the golden file demonstrates all three finding classes in one run

## Decisions Made
See `key-decisions` in the frontmatter above — the push-event two-dot range (locked by the plan's own `must_haves.truths` over RESEARCH's illustrative three-dot bash), the best-effort local schema glob for star expansion, the RENAME COLUMN old-name-recovery for cross-reference lookups, and the golden-file extension to a three-class demonstration.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed a regex-consumption bug in the FROM/JOIN alias scanner**
- **Found during:** Task 2, while writing `TestExtractReferences`'s low-confidence two-table-join case
- **Issue:** The original combined `\b(?:FROM|JOIN)\s+(table)\b(?:\s+(?:AS\s+)?(alias))?` regex let the optional trailing-alias group match the *next* clause's own `FROM`/`JOIN` keyword (e.g. `FROM widgets_a\nJOIN widgets_b ...` captured `"JOIN"` as `widgets_a`'s alias). Since `regexp.FindAllStringSubmatch` never re-examines bytes already consumed by a prior match, the real second `JOIN widgets_b` occurrence was then silently skipped entirely, dropping `widgets_b` from the real-table set.
- **Fix:** Replaced the single combined regex with `findFromJoinTables`, which first locates every bare `FROM`/`JOIN` keyword position (a regex that matches nothing else, so two adjacent clauses are always found as two separate hits), then hand-tokenizes the table name and optional alias following each one.
- **Files modified:** `cmd/migration-check/main.go`
- **Verification:** `TestFindFromJoinTables_AdjacentFromJoinBothFound` pins the fix directly; `TestExtractReferences`'s low-confidence-join subtest depends on it and passes.
- **Committed in:** `4d1399a` (Task 2 commit)

**2. [Rule 1 - Bug] Split `finding.object` on `" -> "` before the RENAME COLUMN cross-reference lookup**
- **Found during:** Task 3, `TestPrevReleaseCrossRef_RenameColumnIsRed`
- **Issue:** The DDL scanner's `describe()`-friendly `finding.object` for a `RENAME COLUMN` is the combined display string `"<old> -> <new>"` (e.g. `"image_url -> art_url"`). Looking that combined string up directly in the high-confidence set (keyed on bare single column names) never matched, so a renamed-but-still-referenced column silently passed instead of cross-reference-redding.
- **Fix:** `crossReferenceFinding` now splits `rename_column` findings' `object` on `" -> "` and looks up only the old name.
- **Files modified:** `cmd/migration-check/main.go`
- **Verification:** `TestPrevReleaseCrossRef_RenameColumnIsRed` passes.
- **Committed in:** `c0a92d5` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 — bugs found and fixed during TDD, before the task's own commit).
**Impact on plan:** Both fixes are internal correctness fixes to code written earlier in this same plan (not pre-existing code); no scope creep, no plan restructuring.

## Issues Encountered
- Adding `crossRefNotice` (the "true bootstrap, skipped" line) to every `--mode=scan` invocation without an explicit `--prev-tag` initially broke the pre-existing Task 1/Task 2 golden-file test, since none of those invocations pass `--prev-tag` and the notice is prepended by design. Resolved by regenerating `mixed_findings.golden.txt` via the existing `-update` flag once the golden test itself was updated to pass `--prev-tag` and stub a real cross-reference hit — this is exactly the "three distinct remediation paragraphs" extension Task 3's acceptance criteria explicitly called for, not an unplanned side effect.
- `make test` (`go test ... -race`) fails to build on this Windows dev box — pre-existing, documented cgo/ThreadSanitizer limitation (see `.planning/WINDOWS.md`, and every prior Phase 16 SUMMARY's identical note). Substituted the established workaround: `go test ./... -coverprofile=coverage.out -coverpkg=$COVER_PKGS -p 1` without `-race`, confirmed by prior phases as the accepted local equivalent. `make coverage-gate`'s underlying tool then ran against that profile and passed (90.05%, floor 80%).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness
- 16-04 (CI workflow wiring: the `changes` prelude job, the `migration-check` job invoking `--mode=changed-files` then `--mode=scan --prev-tag=$(svu current)`, and the `n1-boot` job) can now consume every CLI surface this plan built: `--mode=changed-files` with `--event-name`/`--before`/`--sha`/`--base-ref`, and `--mode=scan`'s new `--prev-tag` flag. `migrations_changed`/`migration_files` are already emitted in `$GITHUB_OUTPUT` key=value form.
- `svu current` on this repo resolves to a real tag (`v1.7.0`) with existing history, so 16-04's `n1-boot` job wiring has a genuine non-bootstrap case to exercise, not just the empty-`--prev-tag` skip path.
- No blockers. MGRT-01 stays open — `requirements.ready-ids` confirms it blocked pending 16-04 (16-04 also declares MGRT-01 and has not executed yet).

---
*Phase: 16-rollback-safe-migrations*
*Completed: 2026-09-04*

## Self-Check: PASSED

All 12 created/modified files found on disk; all 3 task commit hashes (`49e0d93`, `4d1399a`, `c0a92d5`) found in git log.
