---
phase: 16-rollback-safe-migrations
plan: 02
subsystem: ci
tags: [ci-tooling, static-analysis, migrations, sql, golangci-lint]

# Dependency graph
requires:
  - phase: 15-pr-coverage-diff-comment
    provides: cmd/coverage-report structural template (thin main -> run(), whitebox tests, golden files, package-var test seams, COVER_PKGS/gosec carve-out precedent) mirrored for cmd/migration-check
provides:
  - cmd/migration-check package + run(args, stdout) -- stdlib-only two-class DDL guard (D-08)
  - stripComments / splitStatements / classifyStatement scan pipeline
  - allow-destructive annotation grammar locked (option a) + parseAnnotation/shouldSuppress
  - .golangci.yml gosec G304 carve-out and Makefile COVER_PKGS exclusion for cmd/migration-check
affects: [16-03-git-cross-reference-and-ci-wiring, 16-04-ci-wiring-n1-boot-job, 16-05-migrations-readme]

# Actuals (#2632)
actuals:
  tokens: 10650
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "stdlib-only comment-strip/statement-split/classify pipeline, offset-preserving so 1-based line numbers survive stripComments"
    - "package-var-free annotation parsing: parseAnnotation reads raw pre-strip text (the annotation lives inside a comment) and returns (annotation, ok, err) so 'no annotation', 'malformed annotation', and 'valid annotation' are three distinct, individually testable outcomes"
    - "shouldSuppress is a named, single-purpose predicate deliberately kept separate from the parse/classify stages so Plan 03's D-15 previous-release query cross-reference can bypass it without restructuring suppression"

key-files:
  created:
    - cmd/migration-check/main.go
    - cmd/migration-check/main_test.go
    - cmd/migration-check/testdata/*.sql (16 fixtures + 1 golden)
  modified:
    - .golangci.yml
    - Makefile

key-decisions:
  - "Checkpoint (Task 2) decided option a: file-scoped, single-line, bare-token allow-destructive annotation with a trailing free-text reason -- \"-- migration-check:allow-destructive expand-shipped-in=v1.7.0 reason=events.old_col superseded by watched_artist_name\". Chosen over statement-scoped (option b, needs a second offset-preserving comment-to-statement pass) and quoted-reason (option c, introduces escaping rules into an immutable-file contract) because option a is the only shape with no escaping rules to get wrong later -- and this grammar is a one-way door once a released *.up.sql carries it. Recorded verbatim per the resume instruction; Task 3 implements exactly this shape."
  - "expand-shipped-in is shape-validated (^v?[0-9]+(\\.[0-9]+){0,2}(-[0-9A-Za-z.-]+)?$) at the parse boundary in Task 3, before the value is ever stored -- T-16-11. Existence-against-real-git-tags validation is explicitly deferred to Plan 03, which introduces the git show seam this value will eventually reach as exec.Command argv."
  - "A half-written annotation (only one of the two required keys) is a hard error naming the missing key and does NOT suppress the underlying finding -- verified for both annotated_missing_reason.sql and annotated_missing_tag.sql. This was chosen over silently ignoring a malformed annotation, since a silently-ignored malformed annotation is functionally indistinguishable from 'nobody added an annotation', which would surprise an author who thought they'd suppressed the finding."
  - "MGRT-01 left unmarked: requirements.ready-ids reports it blocked because sibling plans 16-03/16-04 also declare MGRT-01 and have not executed yet (confirmed via a live query, not assumed from 16-01's note)."

patterns-established:
  - "Two-class DDL guard: backward-incompatible (N-1 break: DROP/RENAME/type-narrow/SET NOT NULL/ADD CHECK) vs unsafe-forward (deploy hazard: ADD COLUMN NOT NULL with no DEFAULT) -- distinct messages, both citing internal/db/migrations/README.md, both suppressible by the same annotation (D-08 revision)."
  - "Deterministic reporting: findings sorted by (path, line) before render; scanned-file list always printed including the empty case (D-07 transparency), so a green run is never indistinguishable from a run that examined nothing."

requirements-completed: []

coverage:
  - id: D1
    description: "A branch-new migration carrying any of the seven backward-incompatible patterns or the one unsafe-forward pattern makes cmd/migration-check exit non-zero with a class-specific, README-citing message (D-08, SC #2)"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_PatternDetection"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_UnsafeForwardMessageDiffersFromBackwardIncompatible"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_GoldenFailureMessage"
        status: pass
    human_judgment: false
  - id: D2
    description: "The repo's seven shipped up-migrations and all seven down-migrations score zero findings; down files are never scanned"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_RealRepoMigrationsAreClean"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_DownFilesProduceNoFindings"
        status: pass
    human_judgment: false
  - id: D3
    description: "Output is deterministic and reproducible; a zero-file run is still visibly a zero-file run"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_OutputIsDeterministic"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_EmptyFileListPrintsScannedNoneAndExitsZero"
        status: pass
    human_judgment: false
  - id: D4
    description: "The allow-destructive annotation grammar is locked by an explicit human decision (checkpoint) and a well-formed annotation suppresses both finding classes in that file, echoing the tag and reason into stdout"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_AnnotatedDropIsSuppressedAndEchoesReason"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_AnnotatedNotNullIsSuppressed"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_AnnotatedSafeFileIsNotAnError"
        status: pass
    human_judgment: false
  - id: D5
    description: "A half-written annotation (missing expand-shipped-in or reason) is a hard error naming the missing key and does not silently suppress the underlying finding"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_AnnotationMissingReasonIsHardErrorAndDoesNotSuppress"
        status: pass
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestScan_AnnotationMissingTagIsHardErrorAndDoesNotSuppress"
        status: pass
    human_judgment: false
  - id: D6
    description: "expand-shipped-in's value is shape-validated at the parse boundary before it can ever reach a subprocess (T-16-11) -- a shell metacharacter or path separator is rejected"
    requirement: "MGRT-01"
    verification:
      - kind: unit
        ref: "cmd/migration-check/main_test.go#TestAnnotation_TagShapeIsValidated"
        status: pass
    human_judgment: false
  - id: D7
    description: "cmd/migration-check is excluded from the Makefile COVER_PKGS product-coverage denominator and carries a gosec G304 carve-out scoped to ^cmd/migration-check/ only (D-19b), mirroring the cmd/coverage-report precedent"
    requirement: "MGRT-01"
    verification:
      - kind: other
        ref: "grep -c cmd/migration-check .golangci.yml >= 1 (gosec-only rule); grep -v '^#' Makefile | grep -c cmd/migration-check >= 1"
        status: pass
    human_judgment: false

duration: ~30min active (Task 1 + Task 3; Task 2 was a human checkpoint pause between sessions)
completed: 2026-09-04
status: complete
---

# Phase 16 Plan 02: cmd/migration-check Static DDL Guard Summary

**A stdlib-only, unit-tested Go CI guard now turns a branch red when a migration carries a backward-incompatible (N-1-breaking) or unsafe-forward (deploy-hazard) SQL statement, with class-specific README-citing messages and a checkpoint-locked, shape-validated `allow-destructive` escape-hatch annotation.**

## Performance

- **Duration:** ~30 min of active execution across two sessions (Task 1 in the first, Task 3 in this continuation, separated by a human-decision checkpoint pause on Task 2)
- **Started:** 2026-09-04T09:04Z (Task 1's first commit, shared session start with 16-01)
- **Completed:** 2026-09-04T16:44Z (Task 3 commit)
- **Tasks:** 3 (Task 1 auto/tdd, Task 2 checkpoint:decision, Task 3 auto/tdd)
- **Files modified:** 22 (2 Go source + 1 Go test + 16 SQL/golden fixtures created across both tasks, 2 config files modified)

## Accomplishments
- Built `cmd/migration-check`'s stdlib-only scan pipeline (`stripComments` → `splitStatements` → `classifyStatement`) detecting the seven backward-incompatible DDL patterns and the one unsafe-forward pattern (D-08), with deterministic, sorted, always-transparent output (D-07)
- Locked the `allow-destructive` annotation grammar at the Task 2 checkpoint (option a: file-scoped, single-line, bare-token tag, trailing free-text reason) — a one-way-door decision now implemented exactly as specified
- Implemented annotation parsing, shape validation, and suppression semantics: a well-formed annotation suppresses both finding classes in its file and echoes the tag + reason into stdout; a half-written annotation is a hard error naming the missing key and suppresses nothing
- Added the `.golangci.yml` gosec G304 carve-out and `Makefile` `COVER_PKGS` exclusion for `cmd/migration-check`, mirroring the Phase 15 `cmd/coverage-report` precedent (D-19b)
- 37 passing test cases across both tasks; full Definition of Done (build, vet, lint, integration test suite, coverage gate, sqlc check) clean

## Task Commits

Each task was committed atomically:

1. **Task 1: The scanner — comment-strip, statement-split, and the two finding classes** - `840256e` (feat)
2. **Task 2: Checkpoint: lock the allow-destructive annotation grammar** - no commit (checkpoint:decision; the decision itself is recorded here and implemented in Task 3)
3. **Task 3: Annotation parsing, suppression semantics, and the CI-helper carve-outs** - `d477922` (feat)

**Plan metadata:** recorded in this commit (docs: complete plan)

_Note: this SUMMARY.md was written by a continuation agent that resumed after the Task 2 checkpoint was answered by the operator; Task 1's work was independently re-verified (git log, git show --stat, full test run) rather than trusted blindly before Task 3 began._

## Files Created/Modified
- `cmd/migration-check/main.go` - scan pipeline (Task 1) + annotation parsing/suppression (Task 3): `run`, `runScan`, `scanFile`, `stripComments`, `splitStatements`, `classifyStatement`, `parseAnnotation`, `shouldSuppress`, `buildReport`
- `cmd/migration-check/main_test.go` - whitebox table tests over `testdata/`, golden failure-message file, annotation suppression/error/tag-shape tests
- `cmd/migration-check/testdata/*.sql` - 11 Task-1 pattern fixtures + 1 golden file + 5 Task-3 annotation fixtures (`annotated_drop.sql`, `annotated_missing_reason.sql`, `annotated_missing_tag.sql`, `annotated_notnull.sql`, `annotated_safe.sql`)
- `.golangci.yml` - gosec G304 exclusion rule scoped to `^cmd/migration-check/`, placed directly after the existing `^cmd/coverage-report/` entry
- `Makefile` - `COVER_PKGS` alternation extended to also exclude `cmd/migration-check`

## Decisions Made
See `key-decisions` in the frontmatter above — the checkpoint's option-a grammar choice, the deferred tag-existence validation, the half-written-annotation-is-a-hard-error choice, and the MGRT-01 blocked-not-marked confirmation.

## Deviations from Plan

None in Task 3 — implemented exactly per PLAN.md's locked grammar and acceptance criteria.

Task 1's one deviation (already committed, not repeated here per the continuation brief): gosec G304 on the `os.ReadFile(path)` call site was suppressed with a task-scoped inline `//nolint:gosec` comment ahead of Task 3's repo-wide `.golangci.yml` carve-out, which this plan's Task 3 now adds as planned. The inline `//nolint:gosec` comment was left in place in Task 3 (harmless redundancy with the new carve-out rule; removing it was out of Task 3's stated scope and would have been unnecessary diff churn).

---

**Total deviations:** 0 in this continuation session.
**Impact on plan:** None — plan executed exactly as written for Task 3.

## Issues Encountered
- `make test` (`go test ... -race`) fails to build on this Windows dev box — pre-existing, documented cgo/ThreadSanitizer limitation (see `.planning/WINDOWS.md`, and 16-01-SUMMARY.md's identical note). Substituted the established workaround: the same `go test ./... -coverprofile=coverage.out -coverpkg=$COVER_PKGS` invocation without `-race` and with `-p 1`, confirmed by prior phases (11.1-04, 15-02, 16-01) as the accepted local equivalent. `make coverage-gate` then ran against that profile and passed (90.05%, floor 80%).
- `.golangci.yml`'s CRLF-to-LF Git warning on stage (pre-existing repo-wide `.gitattributes` LF normalization from quick-task 260901-lvn) — cosmetic, no action needed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- 16-03 (git cross-reference / D-15 previous-release query check + `gitShow` seam) can now assume `cmd/migration-check`'s scan pipeline, finding classes, and annotation suppression predicate (`shouldSuppress`) are real and tested — its cross-reference check is documented here as the deliberate bypass of that predicate, not a rework of it.
- 16-04 (CI wiring) can invoke `go run ./cmd/migration-check --mode=scan --files=...` exactly as built: prints the scanned-file list unconditionally, exits non-zero on any unsuppressed finding or annotation error, exits 0 with the scanned-file list and a "no findings" line otherwise.
- The annotation grammar Plan 05's README will document is locked and byte-exact: `-- migration-check:allow-destructive expand-shipped-in=vX.Y.Z reason=<free text>`.
- No blockers. MGRT-01 stays open pending 16-03/16-04.

---
*Phase: 16-rollback-safe-migrations*
*Completed: 2026-09-04*

## Self-Check: PASSED

All 10 created/modified files found on disk; both task commit hashes (`840256e`, `d477922`) found in git log.
