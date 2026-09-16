---
phase: 16-rollback-safe-migrations
reviewed: 2026-09-04T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - .claude/CLAUDE.md
  - .github/workflows/full-pipeline.yml
  - .golangci.yml
  - Makefile
  - cmd/migrate/main.go
  - cmd/migrate/main_test.go
  - cmd/migration-check/main.go
  - cmd/migration-check/main_test.go
  - internal/db/migrate.go
  - internal/db/migrate_ahead_test.go
  - internal/db/migrations/README.md
  - internal/db/migrations_readme_test.go
status: issues_found
findings:
  critical: 1
  warning: 1
  info: 0
  total: 2
outcome:
  cr01: fixed (commit 13a1a24 — schema-qualified table names now stripped before D-15 lookup; regression test TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed added, confirmed RED before / GREEN after)
  wr01: not fixed — tracked as a follow-up, non-blocking (rename_column display-string coupling; low risk, no live bug)
---

# Phase 16: Code Review Report

**Reviewed:** 2026-09-04
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

Reviewed the rollback-safety tooling added in this phase: the ahead-of-source boot
no-op guard in `internal/db/migrate.go`, the stdlib-only static DDL scanner in
`cmd/migration-check` (including its D-15 previous-release query cross-reference and
its git-subprocess seams), the `cmd/migrate` HEAD-schema CI helper, the migrations
README, and the three new CI jobs (`changes`, `migration-check`, `n1-boot`) wired into
`.github/workflows/full-pipeline.yml`.

The ahead-of-source guard in `internal/db/migrate.go` is correct: its branch conditions
(`verr == nil && !dirty`, then `ok && cur > smax`) precisely match the four documented
scenarios (fresh DB, dirty-ahead, behind, equal-version idempotency) and each has a
passing test. The CI workflow's job/step gating is also correct: `changes` and
`migration-check`/`n1-boot` are unconditional jobs with step-level `if:` gates, exactly
as the inline comments require, and `build-scan`'s `needs:` array is not itself gated.
All git-subprocess invocations in `cmd/migration-check` (`git diff`, `git cat-file -e`,
`git show`) are built as argv slices (never `sh -c`) and every dynamic argument
(`baseRef`, `before`, `sha`, `tag`, `path`) is shape-validated by an allowlist
(`validBranchRef`, `validCommitish`, `reTagShape`, `pathAllowedForGitShow`) before it
can reach a subprocess call — this part of the design holds up under adversarial input
(tests directly probe shell-metacharacter and path-traversal payloads).

However, the scanner's D-15 previous-release query cross-reference — documented in
`internal/db/migrations/README.md` as non-overridable by the `allow-destructive`
annotation ("the finding stays red regardless of the annotation") — has a real
identifier-normalization gap that lets a schema-qualified `ALTER TABLE`/`DROP TABLE`
statement silently bypass that cross-reference. See CR-01.

## Critical Issues

### CR-01: Schema-qualified table names bypass the D-15 non-overridable cross-reference check

**File:** `cmd/migration-check/main.go:1437-1450` (also `:761-807`)

**Issue:**

The scan side (`classifyStatement`/`classifyAlterClause`) never strips a schema
qualifier off the table name it puts into a `finding`, while the query-reference side
(`extractBlockReferences`, `parseSchemaColumns`) always does via `stripSchemaQualifier`
before storing into `prevReleaseRefs`:

```go
// cmd/migration-check/main.go:763-767 -- table taken as raw stripIdent(m[1]),
// no stripSchemaQualifier call.
if m := reDropTable.FindStringSubmatch(text); m != nil {
    return []finding{newFinding(path, st.line, classBackward, "drop_table", stripIdent(m[1]), "")}
}
if m := reAlterTable.FindStringSubmatch(text); m != nil {
    table := stripIdent(m[1])
```

```go
// cmd/migration-check/main.go:1437-1450 -- crossReferenceFinding looks the
// raw f.table up against refs, which were built with stripSchemaQualifier
// applied (e.g. main.go:1232, :1249).
switch f.kind {
case "drop_column":
    ref, hit = refs.hasHigh(f.table, f.object)
case "rename_column":
    old, _, _ := strings.Cut(f.object, " -> ")
    ref, hit = refs.hasHigh(f.table, old)
case "drop_table", "rename_table":
    ref, hit = refs.hasHighAnyColumn(f.table)
```

`prevReleaseRefs` is always populated with schema-stripped table names
(`normalizeIdent(stripSchemaQualifier(...))` at `main.go:1232` and `:1249`), but
`hasHigh`/`hasHighAnyColumn` only call `normalizeIdent` on the lookup key — they never
call `stripSchemaQualifier`. So for `ALTER TABLE public.events DROP COLUMN
notified_at;`, `f.table` is `"public.events"` while the stored reference key is
`"events"`; the lookup misses even though `notified_at` is genuinely still referenced
by the previous release's `queries/events.sql`.

Concretely, this means: a migration author who writes a schema-qualified `DROP COLUMN`
(or `DROP TABLE` / `RENAME TABLE` / `RENAME COLUMN`) *and* attaches a
`migration-check:allow-destructive` annotation gets the finding silently suppressed by
`shouldSuppress`, because the cross-reference (the one check the README explicitly says
"cannot override a live reference") never fires to override the suppression. This is
exactly the failure mode D-15 exists to prevent — it just requires one extra, entirely
valid piece of Postgres syntax (schema qualification) to slip past it. No migration in
this repo currently uses a schema-qualified table name, and no test fixture in
`cmd/migration-check/testdata/` (including `prevrelease/schema_qualified.sql`, which
only exercises the query-parsing side) exercises a schema-qualified `ALTER`/`DROP
TABLE` on the *scan* side — the gap is real and currently untested in either direction.

**Fix:** Normalize the same way on both sides. Simplest fix is to strip the schema
qualifier at the point of comparison in `crossReferenceFinding`:

```go
func crossReferenceFinding(refs *prevReleaseRefs, f finding, prevTag string, ann annotation, annValid bool) (finding, bool) {
	if refs == nil {
		return finding{}, false
	}
	table := stripSchemaQualifier(f.table)
	var ref queryRef
	var hit bool
	switch f.kind {
	case "drop_column":
		ref, hit = refs.hasHigh(table, f.object)
	case "rename_column":
		old, _, _ := strings.Cut(f.object, " -> ")
		ref, hit = refs.hasHigh(table, old)
	case "drop_table", "rename_table":
		ref, hit = refs.hasHighAnyColumn(table)
	default:
		return finding{}, false
	}
	...
```

(`stripSchemaQualifier` already exists at `main.go:997` and is exported at
package scope.) Add a regression fixture pairing a schema-qualified `ALTER TABLE
public.events DROP COLUMN notified_at;` scan input with a stubbed
`queries/events.sql` that references `notified_at`, asserting the `classCrossRef`
finding still fires — mirroring `TestPrevReleaseCrossRef_RenameColumnIsRed` but with a
`public.`-qualified table name.

## Warnings

### WR-01: `rename_column` findings encode two identifiers into one display string that is later re-parsed

**File:** `cmd/migration-check/main.go:791, 1440-1445`

**Issue:** `classifyAlterClause` builds the rename_column finding's `object` field as a
formatted display string, `stripIdent(m[1])+" -> "+stripIdent(m[2])` (old and new
column names joined by the literal `" -> "`), and `crossReferenceFinding` later
recovers the old name by `strings.Cut(f.object, " -> ")`. This couples a
human-readable rendering detail to a data-extraction path: any future change to the
display format (e.g. localizing the message, using `→` instead of `->`, or adding
extra whitespace) silently breaks the D-15 cross-reference's ability to recover the old
column name, with no compiler error and no test failure unless someone thinks to check
`crossReferenceFinding` specifically.

**Fix:** Add a dedicated field to `finding` (e.g. `renameFrom string`) populated
directly from `stripIdent(m[1])` at classification time, and have
`crossReferenceFinding` read `f.renameFrom` instead of re-parsing `f.object`. Keep
`object`/`describe()` purely for the human-readable rendering.

---

## Post-Review Outcome

**CR-01 — fixed in `13a1a24`.** `crossReferenceFinding` now strips the schema
qualifier off `f.table` before the `hasHigh`/`hasHighAnyColumn` lookup, matching
the normalization `prevReleaseRefs` already applies on the query-reference side.
Added `TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed` with a new
`testdata/drop_table_schema_qualified.sql` fixture; confirmed the test fails
against the pre-fix code (git-stash verified) and passes after. Full Definition
of Done re-run green (`go vet`, `golangci-lint`, full `go test ./...` via real
Postgres, `make coverage-gate` at 82.43%, `make sqlc-check`).

**WR-01 — not fixed.** Left as a documented follow-up (rename_column's
`object` display string doubling as a data-extraction source for
`crossReferenceFinding`). Non-blocking: no live bug, just a coupling risk for
a future edit to the display format.

---

_Reviewed: 2026-09-04_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
