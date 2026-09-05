---
phase: quick/260905-kfv
plan: 01
type: execute
wave: 1
depends_on: []
autonomous: true
requirements: [16-ARCH-C1, 16-REVIEW-CR-01, 16-REVIEW-WR-01]

files_modified:
  - internal/sqlscan/lex.go
  - internal/sqlscan/ident.go
  - internal/sqlscan/parse.go
  - internal/sqlscan/queryrefs.go
  - internal/sqlscan/lex_test.go
  - internal/sqlscan/parse_test.go
  - internal/sqlscan/queryrefs_test.go
  - internal/sqlscan/internal_test.go
  - internal/sqlscan/testdata/prevrelease/artists.sql
  - internal/sqlscan/testdata/prevrelease/events.sql
  - internal/sqlscan/testdata/prevrelease/params.sql
  - internal/sqlscan/testdata/prevrelease/low_confidence_join.sql
  - internal/sqlscan/testdata/prevrelease/schema_qualified.sql
  - internal/sqlscan/testdata/prevrelease/case_folding.sql
  - cmd/migration-check/main.go
  - cmd/migration-check/main_test.go
  - cmd/migration-check/testdata/prevrelease/params.sql
  - cmd/migration-check/testdata/prevrelease/schema_qualified.sql
  - cmd/migration-check/testdata/prevrelease/case_folding.sql
  - .planning/todos/pending/2026-09-05-unify-sqlscan-quote-state-machines.md
  - .planning/todos/pending/2026-09-05-resolve-d15-prev-release-query-files-from-prev-tag.md

estimate:
  tokens: 195000
  raw_tokens: 130000
  tasks: 4
  confidence: low

must_haves:
  truths:
    - "`go test ./cmd/migration-check/ -count=1` passes after every commit, with `cmd/migration-check/testdata/mixed_findings.golden.txt` byte-identical to its pre-refactor content (never regenerated with `-update`)."
    - "`internal/sqlscan` is stdlib-only: its transitive dependency list contains no module-path dependency and never `os/exec`."
    - "`internal/sqlscan` counts toward the coverage gate — it is absent from the Makefile `COVER_PKGS` exclusion, and its own per-package statement coverage is at least 80%."
    - "A schema-qualified DDL table name and a query-side table reference resolve to the same normalized key, so the D-15 cross-reference cannot be bypassed by writing `public.events` (CR-01 bug class, structurally prevented)."
    - "`crossReferenceFinding` recovers a renamed column's old name from a typed field, not by re-parsing a rendered display string (WR-01 resolved)."
    - "Every finding message renders from raw (as-written) identifier spellings, so normalizing the parse layer changes no output byte."
    - "All git seams, path/tag allowlists, the annotation grammar, the finding struct and its renderers stay in `cmd/migration-check`; `internal/sqlscan` reads no files and spawns no subprocess."
    - "`.golangci.yml` grants `internal/sqlscan` no path-scoped linter carve-out; the existing gosec G304 carve-out stays scoped to `cmd/migration-check`."
  artifacts:
    - internal/sqlscan/lex.go
    - internal/sqlscan/ident.go
    - internal/sqlscan/parse.go
    - internal/sqlscan/queryrefs.go
    - internal/sqlscan/lex_test.go
    - internal/sqlscan/parse_test.go
    - internal/sqlscan/queryrefs_test.go
    - internal/sqlscan/internal_test.go
    - internal/sqlscan/testdata/prevrelease/
    - .planning/todos/pending/2026-09-05-unify-sqlscan-quote-state-machines.md
    - .planning/todos/pending/2026-09-05-resolve-d15-prev-release-query-files-from-prev-tag.md
  key_links:
    - "`sqlscan.Parse` -> `cmd/migration-check`'s `Action` type switch -> `findingClass` (the rollback-rule seam that replaces ~15 regex matches)."
    - "`sqlscan.SchemaColumns` -> `sqlscan.QueryColumnRefs` star expansion -> `RefSet.High` -> `crossReferenceFinding` (the D-15 chain)."
    - "Normalized identifiers on BOTH the DDL side and the query-reference side -> the single lookup key that CR-01 broke."
    - "Makefile `COVER_PKGS` -> `internal/sqlscan` must stay inside the coverage denominator."
---

<objective>
Extract an `internal/sqlscan` package out of `cmd/migration-check/main.go` (Phase 16
architecture review, candidate 1): the SQL comment/quote lexer, the DDL structural
parser, and the D-15 query-reference extractor become one flat, stdlib-only,
exported-interface package. `cmd/migration-check` keeps everything that is policy or
I/O: rollback rules, remediation messages, the finding struct and its renderers, the
annotation grammar, every git seam and allowlist, and the CI plumbing.

Purpose: `main.go` is 1,469 lines mixing three parsing engines with the CI policy layer.
Splitting at the parse/policy seam also structurally kills two review findings —
normalized identifiers on both sides of the D-15 lookup remove CR-01's bug class (the
scan side stored `public.events`, the ref side stored `events`, the lookup missed), and
a typed `RenameColumn{From, To}` removes WR-01 (the `"old -> new"` display string being
re-parsed with `strings.Cut` to recover data).

Output: `internal/sqlscan` (4 source files, 4 test files, 6 copied fixtures), a reduced
`cmd/migration-check`, and two filed follow-up todos.

This is a **behavior-preserving refactor**. `cmd/migration-check/testdata/mixed_findings.golden.txt`
must be byte-identical after every single commit. It is never regenerated with `-update`.

Preserved-as-is (do NOT fix in this change): a double-quoted DDL identifier is stripped
of its quotes by `StripIdent` before normalization, so it can never match a byte-exact
quoted query-side reference. That asymmetry exists today and stays; changing it is a
behavior change, not a refactor.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@.planning/STATE.md
@cmd/migration-check/main.go
@cmd/migration-check/main_test.go
@Makefile
@.golangci.yml
</context>

<design_contract>

The exported surface below is **settled**. Do not re-derive it, do not add types to it,
do not simplify it. Tasks reference this block by name.

```go
package sqlscan // internal/sqlscan — stdlib only, no file I/O, no subprocess

// ---- lexer (Task 1) ----

// RawStatement is both the lexer's unit of output and, from Task 2 on, the
// Statement fallback for anything Parse does not structurally recognise.
type RawStatement struct {
    Text string // trimmed statement text, semicolon removed
    Line int    // 1-based line of the statement's first non-blank byte
}

func StripComments(src string) string          // was stripComments
func SplitStatements(src string) []RawStatement // was splitStatements
func SplitTopLevelCommas(s string) []string    // was splitTopLevelCommas

// unexported, moved as-is: copySingleQuoted, dollarTagAt, isValidDollarTag,
// copyDollarQuoted, isBlank

// ---- identifiers (Task 1) ----

func NormalizeIdent(s string) string        // lower-case unless "quoted" (then byte-exact)
func StripSchemaQualifier(table string) string
func StripIdent(s string) string            // trim surrounding quotes + trailing ;,()

// ---- typed DDL model (Task 2) ----

// Statement is a sealed interface (unexported marker method). Parse is TOTAL:
// it never returns an error and never drops a statement — anything it does not
// recognise comes back as RawStatement.
type Statement interface{ isStatement() }

type CreateTable struct {
    Line    int
    Name    string   // normalized + schema-stripped
    RawName string   // as written (post-StripIdent), for message rendering only
    Columns []string // normalized column names, constraint clauses already filtered
}
type AlterTable struct {
    Line    int
    Name    string
    RawName string
    Actions []Action
}
type DropTable struct {
    Line    int
    Name    string
    RawName string
}
// RawStatement (above) is the fourth Statement.

// Action is a sealed interface. One ALTER clause yields at most one Action,
// chosen by the SAME precedence the old classifyAlterClause switch used:
// DropColumn -> RenameTable -> RenameColumn -> AlterColumnType -> SetNotNull ->
// AddCheck -> AddColumn. A clause matching none of them is not represented
// (exactly mirroring today's "no finding" outcome).
type Action interface{ isAction() }

type DropColumn      struct{ Column, RawColumn string }
type RenameColumn    struct{ From, To, RawFrom, RawTo string }
type AddColumn       struct{ Column, RawColumn string; NotNull, HasDefault bool }
type AlterColumnType struct{ Column, RawColumn string }
type SetNotNull      struct{ Column, RawColumn string }
type AddCheck        struct{}
type RenameTable     struct{ To, RawTo string }

func Parse(sql string) []Statement

// SchemaColumns builds the "all columns of table X" set used to expand
// SELECT * / RETURNING *. It reads CreateTable.Columns and AddColumn actions only.
func SchemaColumns(stmts []Statement) map[string][]string

// ---- query references (Task 3) ----

type TableColumn struct{ Table, Column string }
type Ref struct{ Table, Column, QueryFile, QueryName string }

// RefSet owns the high/low/params confidence tiers (RESEARCH Pitfall E):
// deterministic-red only on High; Low is informational; Params are never
// asserted as a column of any table.
type RefSet struct {
    High   []Ref
    Low    map[TableColumn]bool
    Params map[string]bool
}

// QueryColumnRefs parses one queries/*.sql file's text. schemaCols is
// SchemaColumns' output and is what makes star expansion possible.
// Always returns non-nil Low/Params maps.
func QueryColumnRefs(file, sql string, schemaCols map[string][]string) RefSet

func (r *RefSet) Merge(other RefSet)                     // lazily inits maps
func (r RefSet) Lookup(table, column string) (Ref, bool) // normalizes both args
func (r RefSet) LookupAnyColumn(table string) (Ref, bool)
func (r RefSet) HasLow(table, column string) bool
```

**Locked-design reconciliation (one item, recorded not re-litigated):** the grilling
notes sketch `QueryColumnRefs(file, sql string) RefSet` while also specifying
`SchemaColumns` as the source of "SELECT * / RETURNING * expansion". Those two cannot
both hold with a two-argument signature — star expansion has nowhere to read the column
set from, and the moved `TestExtractReferences` star cases would have to be deleted
(a scope reduction). The third parameter is the minimal reconciliation; the name and
return type are unchanged.

**Naming note:** the sketch writes `AddColumn{Col, ...}`; the contract uses `Column`
uniformly across `DropColumn`/`AddColumn`/`AlterColumnType`/`SetNotNull`. Cosmetic,
for cross-type consistency.

**Lookup normalization:** `Lookup`/`LookupAnyColumn`/`HasLow` run the table argument
through `StripSchemaQualifier` then `NormalizeIdent`, and the column argument through
`NormalizeIdent`. This preserves `TestExtractReferences_IdentifierCaseFolding`
(unquoted `IMAGE_URL` matches a stored `image_url`; quoted `"Mixed"` matches byte-exact
and bare `mixed` does not) and makes a schema-qualified lookup structurally impossible
to miss.

**Raw vs normalized:** "Raw" means the value today's `describe()` renders — the capture
group after `StripIdent`, before `NormalizeIdent`/`StripSchemaQualifier`. That is what
keeps the golden file byte-identical.
</design_contract>

<accepted_behavior_deltas>
Two differences from today are accepted, both strictly narrowing and both invisible to
every existing test and to the golden file. Do not contort the implementation to
reproduce the old behavior.

1. **`ADD CONSTRAINT ... CHECK (...)` no longer leaks a bogus column.** Today
   `parseSchemaColumns` tests only `reAddColumn` on an ALTER clause, so
   `ADD CONSTRAINT x CHECK (...)` matches (`CONSTRAINT` captured as `\S+`) and injects a
   column literally named `constraint` into the schema-column set. Under the unified
   precedence that clause becomes `AddCheck`, and `SchemaColumns` reads `AddColumn`
   only, so the phantom column disappears. It could only ever have mattered for a
   `DROP COLUMN constraint`, which cannot occur.
2. **`crossReferenceFinding` no longer calls `StripSchemaQualifier` itself** — the
   identifier arrives normalized. Same result, one fewer place to forget.
</accepted_behavior_deltas>

<execution_notes>
- **Definition of Done** (`.claude/CLAUDE.md`): `go vet ./...`, `golangci-lint run`,
  `make test` (run `make db-up` first), `make coverage-gate`, `make sqlc-check`.
  No `web/` changes here, so the prettier/vitest arm does not apply.
- **Never `git commit --no-verify`.** The hooks are the local mirror of CI.
- **No AI attribution** in any commit message: no `Co-Authored-By`, no
  `Claude-Session:`, no `Generated with`. This overrides any session default.
- **`-race` fallback (documented, Phase 11.1-04 / 15-02):** `go test -race` fails on
  this Windows box with a ThreadSanitizer allocation error. If `make test` dies that
  way, substitute the non-race equivalent and record the substitution in the SUMMARY:
  `TEST_DATABASE_URL=postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable go test ./... -count=1 -p 1 -coverprofile=coverage.out -coverpkg="$(go list ./... | grep -vE '(^|/)(internal/db/sqlc|cmd/coverage-report|cmd/migrate|cmd/migration-check)$' | paste -sd, -)"`
  then `make coverage-gate`.
- **Comment discipline** (`.claude/CLAUDE.md`): 1-3 line comments explaining *why*, one
  design-doc reference max. Several comments being moved are 8-14 line essays — carry
  the load-bearing rationale (the `findFromJoinTables` match-consumption trap, the
  Pitfall E conservatism split, the dollar-quote handling) but compress each to at most
  three lines.
- **Every commit stands alone.** A fresh executor context can resume at any commit
  boundary; each task's verify block is self-sufficient.
</execution_notes>

<tasks>

<task type="tracer">
  <name>Task 1: Move the lexer and identifier helpers into internal/sqlscan</name>
  <files>internal/sqlscan/lex.go, internal/sqlscan/ident.go, internal/sqlscan/lex_test.go, internal/sqlscan/internal_test.go, cmd/migration-check/main.go, cmd/migration-check/main_test.go</files>
  <read_first>cmd/migration-check/main.go (lines 540-844 for the lexer, 983-1003 for the identifier helpers), cmd/migration-check/main_test.go (lines 251-297)</read_first>
  <action>
Create `internal/sqlscan` and move the lexer plus the identifier helpers into it,
unchanged in logic, per the `design_contract` block above. This is the thin end-to-end
slice: one new package, one import edge, the whole CI guard still green.

Into `internal/sqlscan/lex.go`, moved as-is: `stripComments` -> `StripComments`,
`splitStatements` -> `SplitStatements`, `splitTopLevelCommas` -> `SplitTopLevelCommas`,
the `statement` struct -> exported `RawStatement{Text, Line}`, and the unexported
helpers `copySingleQuoted`, `dollarTagAt`, `isValidDollarTag`, `copyDollarQuoted`,
`isBlank`. Put the package doc comment here: one short paragraph naming the three
engines the package will hold and the fact that it reads no files and spawns no
subprocess.

Into `internal/sqlscan/ident.go`: `normalizeIdent` -> `NormalizeIdent`,
`stripSchemaQualifier` -> `StripSchemaQualifier`, `stripIdent` -> `StripIdent`.

Do NOT unify the two hand-rolled single-quote/dollar-quote state machines
(`StripComments` has one, `SplitStatements` has another). That is deliberately deferred
to a follow-up todo filed in Task 4 — unifying them here would make this commit a
behavior change rather than a move.

In `cmd/migration-check/main.go`: delete the moved functions and the `statement` type,
import `github.com/danielrpof/drop-tracker/internal/sqlscan`, and repoint every call
site (`scanFile`, `classifyStatement`, `classifyAlterClause`, `parseSchemaColumns`,
`extractBlockReferences`, `prevReleaseRefs`'s add/has methods, `crossReferenceFinding`)
at the exported names. The DDL regexes, the query regexes and the finding pipeline all
stay exactly where they are — this task moves the lexer only.

Move the three lexer test funcs out of `cmd/migration-check/main_test.go` into
`internal/sqlscan/lex_test.go` as blackbox `package sqlscan_test`, keeping their names
and bodies: `TestStripComments`, `TestSplitStatements_RespectsStringLiteralSemicolons`,
`TestSplitTopLevelCommas_IgnoresCommasInsideParens`.

Add coverage for the dollar-quote paths, which no migration-check fixture exercises and
which would otherwise sink the package's per-package coverage number. In
`lex_test.go` (blackbox): a `SplitStatements` case proving a semicolon inside a
`$$ ... ; ... $$` body does not split the statement, and a `StripComments` case proving
a `--` sequence inside a dollar-quoted body is not stripped. In
`internal/sqlscan/internal_test.go` (whitebox `package sqlscan`, the single whitebox
file the design allows): `dollarTagAt` recognises `$$` and `$body$` but rejects `$ x`
(no closing `$`) and `$a b$` (space in tag), and `copyDollarQuoted` copies the
remainder verbatim when the closing tag is absent.

Do NOT add `internal/sqlscan` to the Makefile `COVER_PKGS` grep exclusion — the new
package is product code and counts toward the 80% floor. Do NOT add a `.golangci.yml`
path carve-out for it either; the existing gosec G304 exclusion stays scoped to
`cmd/migration-check`, which is where the file reads remain. If `golangci-lint` flags
something in the moved code, fix the code; a line-scoped `//nolint` with a one-line
justification is acceptable as a last resort, a path-scoped carve-out is not.

Commit with a message describing the move (behavior-preserving, no logic change) and no
attribution trailer.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go vet ./... &amp;&amp; go test ./cmd/migration-check/ ./internal/sqlscan/ -count=1 &amp;&amp; golangci-lint run</automated>
    <automated>git diff --exit-code HEAD -- cmd/migration-check/testdata/mixed_findings.golden.txt</automated>
    <automated>go list -f '{{join .Deps "\n"}}' ./internal/sqlscan | grep -E '\.|^os/exec$' ; echo "expect: no lines above"</automated>
    <automated>grep 'COVER_PKGS =' Makefile | grep -c sqlscan ; echo "expect: 0"</automated>
    <automated>grep -v '^\s*#' .golangci.yml | grep -c sqlscan ; echo "expect: 0"</automated>
    <automated>go test ./internal/sqlscan/ -covermode=set -coverprofile=coverage.sqlscan.out &amp;&amp; go tool cover -func=coverage.sqlscan.out | tail -1 &amp;&amp; rm -f coverage.sqlscan.out</automated>
  </verify>
  <done>`internal/sqlscan` exists with `lex.go`, `ident.go`, `lex_test.go` and `internal_test.go`; `cmd/migration-check` imports it and no longer defines the lexer or the identifier helpers; the full migration-check suite is green; the golden file is unmodified in git; the package's dependency list is stdlib-only; the Makefile and `.golangci.yml` are untouched. One commit.</done>
  <reversibility rating="reversible">A package move behind an unchanged public CLI contract; revertible with one `git revert`.</reversibility>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Introduce the typed Parse model and rewire the DDL scan</name>
  <files>internal/sqlscan/parse.go, internal/sqlscan/parse_test.go, cmd/migration-check/main.go, cmd/migration-check/main_test.go</files>
  <read_first>internal/sqlscan/lex.go, cmd/migration-check/main.go (lines 363-474 for the finding struct and renderers, 542-549 for scanFile, 745-844 for the DDL regexes and classifiers, 1005-1062 for parseSchemaColumns)</read_first>
  <behavior>
    - `Parse` on `DROP TABLE public.events` yields one `DropTable` with `Name` "events" and `RawName` "public.events", `Line` 1.
    - `Parse` on an `ALTER TABLE` with a comma-separated action list yields one `AlterTable` whose `Actions` are in clause order, one Action per recognised clause.
    - Each of the seven Action types is produced by its own clause form, using the old switch's precedence: `ADD CONSTRAINT x CHECK (...)` is `AddCheck`, never `AddColumn`.
    - `AddColumn` records `NotNull` and `HasDefault` independently; `ADD COLUMN n text NOT NULL DEFAULT 'x'` is `NotNull=true, HasDefault=true`.
    - `RenameColumn` carries `From` and `To` as separate fields (no joined display string anywhere in the model).
    - `Parse` is total: `CREATE INDEX ... ;` comes back as `RawStatement`, never dropped, never an error; `Parse("")` returns an empty slice.
    - `CreateTable.Columns` excludes `CONSTRAINT`/`PRIMARY KEY`/`UNIQUE`/`CHECK`/`FOREIGN KEY` clauses.
    - `SchemaColumns` merges `CreateTable.Columns` with every `AddColumn` action's column, keyed by normalized table name.
  </behavior>
  <action>
Write `internal/sqlscan/parse_test.go` (blackbox `package sqlscan_test`) FIRST, covering
every case in the `behavior` block above plus the moved `TestParseSchemaColumns`
(rename it `TestSchemaColumns`, keep its fixture SQL and its exact-column-set
assertion, and drive it through `sqlscan.SchemaColumns(sqlscan.Parse(sql))`). These
tests are the reason the package can survive its own per-package coverage gate: the
migration-check fixtures that exercise these paths are staying behind in
`cmd/migration-check`, so they contribute nothing to `go test ./internal/sqlscan/`.

Then write `internal/sqlscan/parse.go` per the `design_contract` block: the `Statement`
and `Action` sealed interfaces, the ten concrete types, `Parse`, and `SchemaColumns`.
Move the DDL regexes verbatim from `cmd/migration-check/main.go` — `reDropTable`,
`reAlterTable`, `reDropColumn`, `reRenameTbl`, `reRenameCol`, `reAlterType`,
`reSetNotNull`, `reAddCheck`, `reAddColumn`, `reNotNull`, `reDefault`, `reCreateTable`,
and `tableDefKeywords`. `Parse` runs `StripComments` then `SplitStatements` then
classifies each statement; `RawStatement` is the fallback. Populate every identifier
field twice: `Raw*` is the capture after `StripIdent`, the normalized field is
`NormalizeIdent(...)` for a column and `NormalizeIdent(StripSchemaQualifier(...))` for a
table. Every clause in an `ALTER TABLE` action list goes through `SplitTopLevelCommas`
and the locked precedence order.

Then rewire `cmd/migration-check`. `scanFile` becomes a loop over `sqlscan.Parse` with a
type switch over `Statement`, and a nested switch over `Action` for the `AlterTable`
case — those two switches ARE the rollback rules, and they stay here. Delete the twelve
DDL regexes, `classifyStatement`, `classifyAlterClause`, `tableDefKeywords`,
`reCreateTable` and `parseSchemaColumns` from `main.go`; `buildPrevReleaseRefs` now
calls `sqlscan.SchemaColumns(sqlscan.Parse(string(data)))`.

Give `finding` two new fields, `rawTable` and `rawObject`, and change their meaning:
`table` and `object` now hold the NORMALIZED identifiers (for `object` on a
`rename_column`, that is the OLD column name alone), while `rawTable`/`rawObject` hold
the as-written spellings. `describe()` renders `rawTable`/`rawObject` exclusively, so
the golden output does not move a byte — for `rename_column`, `rawObject` is
`RawFrom + " -> " + RawTo`, assembled at finding-construction time. `crossReferenceFinding`
then reads `f.table`/`f.object` directly: the `StripSchemaQualifier` call and the
`strings.Cut(f.object, " -> ")` re-parse both disappear. That pair of edits is what
retires CR-01's bug class and WR-01. Update `newFinding`'s signature to take both the
normalized and the raw spelling rather than threading a single string.

Every existing `cmd/migration-check` test stays where it is and stays green unchanged —
in particular `TestScan_PatternDetection`, `TestScan_GoldenFailureMessage`,
`TestPrevReleaseCrossRef_RenameColumnIsRed` and
`TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed`, which is now a regression test
for a bug the type system prevents.

Commit with a message naming the typed model and the two review findings it retires. No
attribution trailer.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go vet ./... &amp;&amp; go test ./cmd/migration-check/ ./internal/sqlscan/ -count=1 &amp;&amp; golangci-lint run</automated>
    <automated>git diff --exit-code HEAD -- cmd/migration-check/testdata/mixed_findings.golden.txt</automated>
    <automated>grep -v '^\s*//' cmd/migration-check/main.go | grep -c 'strings.Cut' ; echo "expect: 0"</automated>
    <automated>grep -v '^\s*//' cmd/migration-check/main.go | grep -cE 'regexp.MustCompile\(`\(\?is\)\^(DROP|ALTER|ADD|RENAME|CREATE)' ; echo "expect: 0"</automated>
    <automated>go test ./internal/sqlscan/ -covermode=set -coverprofile=coverage.sqlscan.out &amp;&amp; go tool cover -func=coverage.sqlscan.out | tail -1 &amp;&amp; rm -f coverage.sqlscan.out</automated>
  </verify>
  <done>`sqlscan.Parse` and `sqlscan.SchemaColumns` exist and are covered by a blackbox suite; `cmd/migration-check` builds findings from a type switch over `Statement`/`Action` and holds no DDL regex; `describe()` renders raw spellings and the golden file is unchanged in git; `crossReferenceFinding` performs no schema stripping and no `strings.Cut`; the whole migration-check suite is green. One commit.</done>
  <reversibility rating="costly">Reshapes the `finding` struct and the classifier; reverting after Task 3 lands would mean unwinding two commits, so keep the commits separate and bisectable.</reversibility>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Move the query-reference extractor and gate sqlscan's own coverage</name>
  <files>internal/sqlscan/queryrefs.go, internal/sqlscan/queryrefs_test.go, internal/sqlscan/internal_test.go, internal/sqlscan/testdata/prevrelease/, cmd/migration-check/main.go, cmd/migration-check/main_test.go, cmd/migration-check/testdata/prevrelease/</files>
  <read_first>cmd/migration-check/main.go (lines 904-1000 for the ref types, 1064-1367 for the extractor, 1386-1469 for the wiring), cmd/migration-check/main_test.go (lines 703-917 for the suites that move), internal/sqlscan/parse.go</read_first>
  <precondition>Docker is running and `make db-up` brings up the Postgres service on the `TEST_DATABASE_URL` host/port — `make test` and `make coverage-gate` in this task's verify need a live database.</precondition>
  <behavior>
    - `QueryColumnRefs` reproduces every assertion in the moved `TestExtractReferences` suite: INSERT column list and ON CONFLICT target are High; EXCLUDED and table-qualified columns resolve through the alias map; `a.*` and single-table bare `SELECT *` expand via `schemaCols`; bare `RETURNING *` resolves to the INSERT target; a CTE name never resolves to a migratable table; a subquery's bare WHERE column flattens to the single real table; `sqlc.arg`/`@name` land in `Params` and never in `High` or `Low`; a bare column across a two-table join is Low, never High; `public.events` resolves to `events`.
    - Identifier case folding is unchanged: unquoted `IMAGE_URL`/`Image_Url` match a stored `image_url`; quoted `"Mixed"` matches byte-exact; bare `Mixed` and `mixed` do not.
    - `RefSet.Merge` unions `High`, `Low` and `Params` across files and works against a zero-value receiver.
    - `LookupAnyColumn` hits on any column of the named table and misses on a table with no references at all.
    - `HasLow` is true only for the low tier and never promotes a Low reference into a Lookup hit.
  </behavior>
  <action>
Copy all six fixtures from `cmd/migration-check/testdata/prevrelease/` into
`internal/sqlscan/testdata/prevrelease/` (`artists.sql`, `events.sql`, `params.sql`,
`low_confidence_join.sql`, `schema_qualified.sql`, `case_folding.sql`), then delete
`params.sql`, `schema_qualified.sql` and `case_folding.sql` from the migration-check
copy — after this task, the only migration-check tests reading fixtures are the
`TestPrevReleaseCrossRef_*` suite, which needs `events.sql`, `artists.sql` and
`low_confidence_join.sql` only. Keep those three.

Write `internal/sqlscan/queryrefs_test.go` (blackbox `package sqlscan_test`) by moving
`TestExtractReferences` (rename to `TestQueryColumnRefs`) and
`TestExtractReferences_IdentifierCaseFolding` (rename to
`TestQueryColumnRefs_IdentifierCaseFolding`) with their subtests and assertions intact,
plus a local copy of the `prevReleaseSchemaCols` fixture map and the
`readPrevReleaseFixture` helper. Rewrite the assertions against the new API:
`refs := sqlscan.QueryColumnRefs(file, content, prevReleaseSchemaCols)` and
`refs.Lookup(table, col)` / `refs.HasLow(...)` / `refs.Params[...]`. Add
`TestRefSet_MergeAndLookups` covering `Merge` (including a zero-value receiver),
`LookupAnyColumn` (hit and miss) and `HasLow` — none of those three are reachable from
the moved suites, and all three are on the D-15 critical path.

Move `TestFindFromJoinTables_AdjacentFromJoinBothFound` into the existing whitebox
`internal/sqlscan/internal_test.go`, unchanged. Do not export `findFromJoinTables`,
`nextToken`, `extractParams`, `splitQueryBlocks`, `expandStar` or `classifyBareColumn`
to test them — the whitebox file exists precisely so they can stay unexported.

Write `internal/sqlscan/queryrefs.go`: move `extractReferences`,
`extractBlockReferences`, `findFromJoinTables`, `extractParams`, `splitQueryBlocks`,
`expandStar`, `classifyBareColumn`, `nextToken`, the `queryRef`/`tableColumn`/
`queryBlock`/`fromJoinTable` types, `sqlKeywords` and every query regex
(`reNameMarker`, `reSqlcNamedParam`, `reAtParam`, `reFromJoinKeyword`,
`reIdentWithDot`, `reIdentSimple`, `reWithCTEName`, `reInsertIntoCols`,
`reOnConflictCols`, `reSelectSeg`, `reQualifiedRef`, `reQualifiedQuoted`,
`reStarQualified`, `reBareStarSelect`, `reBareStarReturn`, `reWhereBareCol`,
`reBareSelectItem`). Reshape the accumulator: `prevReleaseRefs` becomes the exported
`RefSet` with `High []Ref` / `Low map[TableColumn]bool` / `Params map[string]bool`,
`queryRef` flattens into `Ref{Table, Column, QueryFile, QueryName}`, and `addHigh`/
`addLow` become unexported methods that keep dropping a reference whose table or column
normalizes to empty. Public entry point is `QueryColumnRefs(file, sql, schemaCols)`,
returning a `RefSet` for that one file with non-nil maps.

Rewire `cmd/migration-check`: delete everything just moved, including
`newPrevReleaseRefs` and the `hasHigh`/`hasLow`/`hasHighAnyColumn` methods.
`buildPrevReleaseRefs` now returns a `*sqlscan.RefSet`, building it by merging one
`QueryColumnRefs` call per `prevReleaseQueryFiles` entry. `crossReferenceFinding`
switches to `refs.Lookup` / `refs.LookupAnyColumn`. Everything else stays put:
`prevReleaseQueryFiles`, the CWD-dependent schema glob, `gitShow`, `readAtTag`,
`allowedGitShowPaths`, `pathAllowedForGitShow`, `reTagShape` and the whole
changed-files half of the tool. The CWD-dependent glob and the hand-synced query-file
list are deliberately untouched here — Task 4 files them as a follow-up.

Then run the coverage gate for real. `internal/sqlscan` is inside the 80% backend floor
denominator, and `go test ./internal/sqlscan/` counts only sqlscan's own tests. If the
per-package number is under 80%, read the `go tool cover -func` output, find the
uncovered functions, and add targeted blackbox tests for them — do not lower a
threshold, do not add the package to the Makefile exclusion, and do not delete code to
raise the ratio.

Commit with a message naming the extractor move and the RefSet reshape. No attribution
trailer.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go vet ./... &amp;&amp; go test ./cmd/migration-check/ ./internal/sqlscan/ -count=1 &amp;&amp; golangci-lint run</automated>
    <automated>git diff --exit-code HEAD -- cmd/migration-check/testdata/mixed_findings.golden.txt</automated>
    <automated>go test ./internal/sqlscan/ -covermode=set -coverprofile=coverage.sqlscan.out &amp;&amp; go tool cover -func=coverage.sqlscan.out | awk '/^total:/{gsub(/%/,"",$3); printf "sqlscan per-package coverage: %s%%\n", $3; exit ($3+0 >= 80) ? 0 : 1}' &amp;&amp; rm -f coverage.sqlscan.out</automated>
    <automated>go list -f '{{join .Deps "\n"}}' ./internal/sqlscan | grep -E '\.|^os/exec$' ; echo "expect: no lines above"</automated>
    <automated>grep 'COVER_PKGS =' Makefile | grep -c sqlscan ; echo "expect: 0"</automated>
    <automated>make db-up &amp;&amp; make test &amp;&amp; make coverage-gate</automated>
  </verify>
  <done>`sqlscan.QueryColumnRefs`, `RefSet` and its lookup/merge methods exist and carry the moved suites plus a merge/lookup test; `cmd/migration-check` holds no query regex, no ref accumulator and no extractor; the six fixtures live under `internal/sqlscan/testdata/prevrelease/` with the three now-unused copies deleted from migration-check; `go test ./internal/sqlscan/` reports at least 80% statement coverage; `make coverage-gate` passes; the golden file is unchanged in git. One commit.</done>
  <reversibility rating="costly">Deletes ~300 lines from `main.go` and reshapes the D-15 accumulator; a revert after this point means rebuilding the whole extractor, so the commit must land green.</reversibility>
</task>

<task type="auto">
  <name>Task 4: File the two follow-up todos and run the full Definition of Done</name>
  <files>.planning/todos/pending/2026-09-05-unify-sqlscan-quote-state-machines.md, .planning/todos/pending/2026-09-05-resolve-d15-prev-release-query-files-from-prev-tag.md</files>
  <read_first>.planning/todos/pending/2026-09-05-delete-stale-web-package-lock-json.md (for the file format)</read_first>
  <precondition>Docker is running and `make db-up` brings up the Postgres service — the final Definition of Done run needs a live database.</precondition>
  <action>
File two pending todos, matching the frontmatter shape of the existing pending todo
(`created`, `title`, `area`, `severity`, `files`) followed by `## Problem` and
`## Solution` sections. Both are out of scope for this change and must not be
implemented here.

Todo one — `2026-09-05-unify-sqlscan-quote-state-machines.md`, area `tooling`, severity
`minor`, files `internal/sqlscan/lex.go`. Problem: `StripComments` and
`SplitStatements` each hand-roll their own single-quote and dollar-quote scanner over
the same grammar, so a fix to one silently leaves the other wrong; they were moved
as-is to keep the extraction behavior-preserving. Solution: now that the module
boundary exists, fold both into one lexer pass and prove equivalence against the
existing blackbox lexer suite plus the migration-check golden file.

Todo two — `2026-09-05-resolve-d15-prev-release-query-files-from-prev-tag.md`, area
`tooling`, severity `minor`, files `cmd/migration-check/main.go`. This is candidate 5
of the Phase 16 architecture review. Problem: `buildPrevReleaseRefs` resolves the
previous release's schema columns via `filepath.Glob` of the *process working
directory*, so the D-15 verdict depends on where the binary is invoked from, and
`prevReleaseQueryFiles` is a hardcoded list hand-synced to `sqlc.yaml` that silently
under-scans when a new query file is added. Solution: resolve both from the
`--prev-tag` through the existing `readAtTag`/`gitShow` seam (which already gates paths
against `allowedGitShowPaths`), killing the CWD dependency and the hand-synced list.
Note that the seam and its allowlist already exist, so this is a wiring change, not new
attack surface.

Do NOT file todos for CR-01's bug class or WR-01 — both are retired by Task 2's design
(normalized identifiers on both sides of the lookup, and a typed `RenameColumn{From, To}`).
Record that in the SUMMARY instead.

Then run the full Definition of Done one final time, including `make sqlc-check` (which
the earlier tasks skipped since no query or schema file changed). Commit the todos with
a docs-scoped message and no attribution trailer.
  </action>
  <verify>
    <automated>ls .planning/todos/pending/2026-09-05-unify-sqlscan-quote-state-machines.md .planning/todos/pending/2026-09-05-resolve-d15-prev-release-query-files-from-prev-tag.md</automated>
    <automated>go vet ./... &amp;&amp; golangci-lint run &amp;&amp; make db-up &amp;&amp; make test &amp;&amp; make coverage-gate &amp;&amp; make sqlc-check</automated>
    <automated>git diff --exit-code HEAD -- cmd/migration-check/testdata/mixed_findings.golden.txt</automated>
    <automated>git log --format=%B -n 4 | grep -icE 'co-authored-by|claude-session|generated with' ; echo "expect: 0"</automated>
  </verify>
  <done>Both todo files exist with valid frontmatter; the full Definition of Done passes end to end including `make sqlc-check`; no commit in this change carries an attribution trailer; the golden file is byte-identical to its pre-refactor content.</done>
  <reversibility rating="reversible">Documentation only.</reversibility>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| CI workflow -> `migration-check` argv | `--files`, `--prev-tag`, `--before`, `--sha`, `--base-ref` arrive as workflow inputs and reach `git` argv and `os.ReadFile` |
| repository SQL -> parser | untrusted-ish text (any contributor's migration or query file) is lexed and regex-matched |
| `internal/sqlscan` -> `cmd/migration-check` | the new package boundary: strings in, typed values out, no I/O either way |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-KFV-01 | Tampering | identifier normalization across the D-15 lookup | high | mitigate | Both sides normalize inside `sqlscan`; `Lookup`/`LookupAnyColumn` additionally strip a schema qualifier, so `public.events` cannot bypass the non-overridable cross-reference. Pinned by `TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed` staying green in Tasks 2 and 3. |
| T-KFV-02 | Elevation of Privilege | new package's dependency surface | high | mitigate | `internal/sqlscan` never imports `os/exec`, `os` or `path/filepath` in non-test code; every git seam, tag allowlist (`reTagShape`) and path allowlist (`pathAllowedForGitShow`) stays in `cmd/migration-check`. Gated in Tasks 1 and 3 by a `go list -f '{{join .Deps ...}}'` check that must print nothing. |
| T-KFV-03 | Tampering | linter carve-out scope | medium | mitigate | No `.golangci.yml` path exclusion is added for `internal/sqlscan`; the gosec G304 carve-out stays scoped to `cmd/migration-check`, where the file reads remain. Gated by a comment-filtered grep in Task 1. |
| T-KFV-04 | Repudiation | coverage denominator | medium | mitigate | `internal/sqlscan` is deliberately NOT added to the Makefile `COVER_PKGS` exclusion, so ~700 lines of parsing code cannot silently leave the 80% floor's denominator. Gated in Tasks 1 and 3, plus an explicit per-package 80% check in Task 3. |
| T-KFV-05 | Tampering | golden-file drift | high | mitigate | `mixed_findings.golden.txt` is never regenerated with `-update`; every task verifies `git diff --exit-code HEAD` against it in addition to running the test that compares output to it. |
| T-KFV-06 | Denial of Service | regex behavior on moved patterns | low | accept | Every regex is moved verbatim, compiled once at package init, and run over repo-local SQL files by a CI helper. Go's RE2 engine has no catastrophic-backtracking class. No new pattern is authored. |
| T-KFV-SC | Tampering | dependency installs | high | mitigate | This change adds zero dependencies — `internal/sqlscan` is stdlib-only and `go.mod`/`go.sum` are untouched. No package-legitimacy checkpoint is required; a diff touching `go.mod` is a signal the plan was exceeded. |
</threat_model>

<verification>
1. `cmd/migration-check/testdata/mixed_findings.golden.txt` is byte-identical to its
   pre-refactor content at every one of the four commits, and `git log -p` shows it in
   no commit's diff.
2. `go test ./cmd/migration-check/ -count=1` is green at every commit — every test
   listed as "stays" in the design still lives in `cmd/migration-check/main_test.go`:
   `TestScan_GoldenFailureMessage`, all `TestScan_Annotated*` / `TestAnnotation_*` /
   `TestParseAnnotation_*`, all `TestDiffRange` / `TestChangedFiles_*` /
   `TestModifiedReleasedMigration` / `TestFilterMigrationUpFiles_*` /
   `TestValidCommitishAndValidBranchRef`, `TestGitShow_*`, `TestPrevReleaseCrossRef_*`
   (including the CR-01 regression), `TestScan_PatternDetection`, and the remaining
   `TestScan_*` pipeline tests.
3. `go test ./internal/sqlscan/ -cover` reports at least 80% statement coverage from
   sqlscan's own tests alone.
4. `make coverage-gate` passes with `internal/sqlscan` inside the denominator.
5. `go list -f '{{join .Deps "\n"}}' ./internal/sqlscan` prints no path containing a
   dot and no `os/exec`.
6. `cmd/migration-check/main.go` contains no DDL regex, no query regex, no lexer, no
   `strings.Cut` on a display string, and no `StripSchemaQualifier` call inside
   `crossReferenceFinding`.
7. Full Definition of Done green: `go vet ./...`, `golangci-lint run`, `make test`,
   `make coverage-gate`, `make sqlc-check`.
8. No commit message in this change contains `Co-Authored-By`, `Claude-Session` or
   `Generated with`.
</verification>

<success_criteria>
- `internal/sqlscan` exists as one flat, stdlib-only package with the exact exported
  surface in the `design_contract` block — `Parse`, `QueryColumnRefs`, `SchemaColumns`,
  the sealed `Statement`/`Action` type sets, `RefSet` and the identifier/lexer helpers.
- `cmd/migration-check/main.go` retains every policy and I/O concern named in the
  design and nothing else; its line count drops by roughly half.
- Four commits, each independently green and bisectable, with the golden file
  byte-identical after each.
- Per-package `internal/sqlscan` coverage at least 80%, and the aggregate backend gate
  still passing.
- CR-01's bug class and WR-01 are structurally retired (documented in the SUMMARY, no
  todos filed for them); the two genuine follow-ups are filed in
  `.planning/todos/pending/`.
</success_criteria>

<output>
Create `.planning/quick/260905-kfv-extract-an-internal-sqlscan-module-out-o/260905-kfv-SUMMARY.md` when done.
Record in it: the per-package sqlscan coverage number and the aggregate
`make coverage-gate` number, whether the `-race` fallback was needed, the two accepted
behavior deltas, and the note that CR-01's bug class and WR-01 are retired by design
rather than by a follow-up todo.
</output>
