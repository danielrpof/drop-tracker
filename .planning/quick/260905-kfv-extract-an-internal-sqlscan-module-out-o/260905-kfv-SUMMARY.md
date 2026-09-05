---
phase: quick/260905-kfv
plan: 01
subsystem: tooling
tags: [migration-check, sqlscan, refactor, ddl-parser, d-15, rollback-safety]
status: complete

requires:
  - phase: 16-rollback-safe-migrations
    provides: "cmd/migration-check: the D-08 DDL scanner, the D-15 previous-release cross-reference, the allow-destructive annotation grammar"

provides:
  - "internal/sqlscan: a flat, stdlib-only package holding the SQL comment/quote lexer, the typed DDL Parse model, and the D-15 query-reference extractor (RefSet)"
  - "cmd/migration-check reduced to policy + I/O: rollback rules, remediation messages, finding struct/renderers, annotation grammar, git seams, CI plumbing (1469 -> 753 lines)"
  - "CR-01 bug class structurally retired: identifiers normalized + schema-stripped on both sides of the D-15 lookup"
  - "WR-01 retired: rename_column carries a typed RenameColumn{From,To}, no display-string re-parse"

affects: [migration-check, phase-17-deploy, d-15-cross-reference]

actuals:
  tokens: 41000
  tasks: 4
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Parse/policy seam: sqlscan owns text->typed-value parsing (total, no error, no I/O); cmd/migration-check owns which typed shape is a finding"
    - "Sealed interface + marker method (Statement/Action) for an exhaustive DDL model"
    - "Raw vs normalized identifier pair threaded through the finding struct so a normalized parse layer moves no output byte"

key-files:
  created:
    - internal/sqlscan/lex.go
    - internal/sqlscan/ident.go
    - internal/sqlscan/parse.go
    - internal/sqlscan/queryrefs.go
    - internal/sqlscan/lex_test.go
    - internal/sqlscan/parse_test.go
    - internal/sqlscan/queryrefs_test.go
    - internal/sqlscan/internal_test.go
    - internal/sqlscan/testdata/prevrelease/ (6 fixtures)
    - .planning/todos/pending/2026-09-05-unify-sqlscan-quote-state-machines.md
    - .planning/todos/pending/2026-09-05-resolve-d15-prev-release-query-files-from-prev-tag.md
  modified:
    - cmd/migration-check/main.go
    - cmd/migration-check/main_test.go

key-decisions:
  - "The two hand-rolled single-quote/dollar-quote state machines in StripComments and SplitStatements were moved as-is, not unified -- unifying is filed as a follow-up todo (would have made Task 1 a behavior change)."
  - "QueryColumnRefs takes a third schemaCols parameter (reconciling the grilling-notes two-arg sketch with SchemaColumns-driven star expansion) -- the minimal reconciliation, name and return type unchanged."
  - "-race unavailable on this Windows box (cgo toolchain: cgo.exe exit status 2, the documented Phase 11.1-04 / 15-02 limitation). Substituted the non-race equivalent (go test ./... -count=1 -p 1 -coverprofile -coverpkg=<filtered>) plus make coverage-gate."

requirements-completed: [16-ARCH-C1, 16-REVIEW-CR-01, 16-REVIEW-WR-01]
---

# quick/260905-kfv: Extract internal/sqlscan out of cmd/migration-check

## What shipped

`internal/sqlscan` is now a flat, stdlib-only package with the settled exported
surface from the plan's `<design_contract>`:

- **Lexer** (`lex.go`): `StripComments`, `SplitStatements` (-> `RawStatement`),
  `SplitTopLevelCommas`.
- **Identifiers** (`ident.go`): `NormalizeIdent`, `StripSchemaQualifier`,
  `StripIdent`.
- **Typed DDL model** (`parse.go`): sealed `Statement` (`CreateTable`,
  `AlterTable`, `DropTable`, `RawStatement`) and sealed `Action` (the seven
  clause types), `Parse` (total -- never errors, never drops a statement),
  `SchemaColumns`.
- **Query references** (`queryrefs.go`): `QueryColumnRefs`, `RefSet` with
  `High`/`Low`/`Params` tiers and `Merge` / `Lookup` / `LookupAnyColumn` /
  `HasLow`.

`cmd/migration-check/main.go` dropped from **1469 to 753 lines**. It keeps every
policy and I/O concern: the rollback-rule type switches over `Statement`/`Action`,
the two remediation paragraphs, the `finding` struct and its renderers, the
allow-destructive annotation grammar, `prevReleaseQueryFiles`, every git seam
(`gitShow`, `readAtTag`, `commitExists`, `gitDiffNames`), the tag/path allowlists,
and the whole changed-files half of the tool.

## Four commits (each independently green, golden byte-identical after each)

| # | Commit | Task |
|---|--------|------|
| 1 | `6e0ed78` | Move the lexer + identifier helpers into `internal/sqlscan` |
| 2 | `dd601a6` | Typed `sqlscan.Parse` model, rewire the DDL scan |
| 3 | `a228c1c` | Move the D-15 query-reference extractor, reshape as `RefSet` |
| 4 | `6ccd998` | File two follow-up todos |

## Coverage

- **`internal/sqlscan` per-package statement coverage: 92.0%** (`go test
  ./internal/sqlscan/ -covermode=set`, well above the 80% floor). `internal/sqlscan`
  is deliberately absent from the Makefile `COVER_PKGS` exclusion -- it counts
  toward the gate denominator.
- **Aggregate `make coverage-gate`: 90.39%** (required 80%). PASS.

## -race fallback

**Needed.** `make test` died with `runtime/cgo: cgo.exe: exit status 2` -- the
pre-existing, documented ThreadSanitizer/cgo break on this Windows dev box
(Phase 11.1-04 / 15-02 precedent). Substituted the non-race equivalent from the
plan's `<execution_notes>`:

```
TEST_DATABASE_URL=... go test ./... -count=1 -p 1 -coverprofile=coverage.out \
  -coverpkg="$(go list ./... | grep -vE '(^|/)(internal/db/sqlc|cmd/coverage-report|cmd/migrate|cmd/migration-check)$' | paste -sd, -)"
```

then `make coverage-gate`. All packages green.

## Accepted behavior deltas (both strictly narrowing, invisible to every test and the golden file)

1. **`ADD CONSTRAINT x CHECK (...)` no longer leaks a phantom `constraint`
   column.** Under the unified `Action` precedence that clause is `AddCheck`, and
   `SchemaColumns` reads `AddColumn` only, so the bogus column literally named
   `constraint` that the old `parseSchemaColumns` injected (via `reAddColumn`
   capturing `CONSTRAINT` as `\S+`) disappears. It could only ever have mattered
   for a `DROP COLUMN constraint`, which cannot occur.
2. **`crossReferenceFinding` no longer calls `StripSchemaQualifier` itself** --
   the identifier arrives normalized + schema-stripped from `sqlscan`. Same
   result, one fewer place to forget.

## CR-01 and WR-01: retired by design, no todos filed

- **CR-01's bug class** (scan side stored `public.events`, ref side stored
  `events`, lookup missed) is structurally prevented: `Parse` normalizes and
  schema-strips the DDL side, and `RefSet.Lookup` / `LookupAnyColumn` run the
  table argument through `StripSchemaQualifier` then `NormalizeIdent`. Pinned by
  `TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed` (green) and
  `TestQueryColumnRefs`'s `Lookup("public.events", "title")` assertion.
- **WR-01** (the `"old -> new"` display string re-parsed with `strings.Cut` to
  recover data) is retired: `RenameColumn{From, To, RawFrom, RawTo}` carries the
  old name in a typed field; `crossReferenceFinding` reads `f.object` directly.
  `grep -v '^\s*//' cmd/migration-check/main.go | grep -c 'strings.Cut'` -> 0.

The two genuine follow-ups (unify the quote state machines; resolve the D-15
prev-release files from `--prev-tag` instead of the process CWD) are filed in
`.planning/todos/pending/`.

## Verification (plan `<verification>`)

- [x] Golden file byte-identical at all four commits (`git diff --exit-code HEAD`
      after each; no commit's diff touches it).
- [x] `go test ./cmd/migration-check/ -count=1` green -- every "stays" test still
      lives in `main_test.go`.
- [x] `go test ./internal/sqlscan/ -cover` = 92.0% (>= 80%).
- [x] `make coverage-gate` = 90.39%, PASS.
- [x] `go list -f '{{join .Deps "\n"}}' ./internal/sqlscan` prints no dotted path
      and no `os/exec` (stdlib only).
- [x] `cmd/migration-check/main.go` holds no DDL regex, no query regex, no lexer,
      no `strings.Cut` on a display string, no `StripSchemaQualifier` in
      `crossReferenceFinding`.
- [x] `go vet ./...`, `golangci-lint run` (0 issues), `make sqlc-check` (no
      drift), `make test` (via -race fallback), `make coverage-gate` all green.
- [x] No commit message contains `Co-Authored-By`, `Claude-Session`, or
      `Generated with` (`git log --format=%B -n 4 | grep -icE ...` -> 0).
- [x] `.golangci.yml` grants `internal/sqlscan` no carve-out; the gosec G304
      exclusion stays scoped to `cmd/migration-check`.

## Self-Check: PASSED

- Created files verified present: `internal/sqlscan/{lex,ident,parse,queryrefs}.go`,
  `internal/sqlscan/{lex,parse,queryrefs,internal}_test.go`,
  `internal/sqlscan/testdata/prevrelease/` (6 fixtures), both todo files.
- Commits verified in `git log`: `6e0ed78`, `dd601a6`, `a228c1c`, `6ccd998`.
