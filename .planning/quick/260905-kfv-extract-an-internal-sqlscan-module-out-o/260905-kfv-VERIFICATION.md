---
phase: quick/260905-kfv
verified: 2026-09-05T00:00:00Z
status: passed
score: 8/8 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# quick/260905-kfv: Extract internal/sqlscan — Verification Report

**Task Goal:** Extract an `internal/sqlscan` module out of `cmd/migration-check/main.go`
as a behavior-preserving refactor — a stdlib-only package holding the SQL lexer, the
typed DDL Parse model, and the D-15 query-reference extractor; `cmd/migration-check`
keeps all policy and I/O; the golden file stays byte-identical after every commit;
`internal/sqlscan` counts toward the 80% coverage gate; CR-01 and WR-01 retired by
design; two follow-up todos filed.

**Verified:** 2026-09-05
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `go test ./cmd/migration-check/ -count=1` passes; `mixed_findings.golden.txt` byte-identical, never `-update`-regenerated | ✓ VERIFIED | `go test ./cmd/migration-check/` → `ok`. Golden blob hash `74a4b02c…` identical at `c549f46`, `6e0ed78`, `dd601a6`, `a228c1c`, `6ccd998`. `git log c549f46..HEAD -- <golden>` empty (no commit touches it). `TestScan_GoldenFailureMessage` does a full `out != string(want)` byte comparison across all 3 finding classes and passes. |
| 2 | `internal/sqlscan` is stdlib-only: no module-path dep, never `os/exec` | ✓ VERIFIED | `go list -f '{{join .Deps "\n"}}' ./internal/sqlscan` → only stdlib (`bytes`, `regexp`, `strings`, `unicode`, `internal/*` runtime pkgs). No dotted path, no `os`, no `os/exec`, no `path/filepath`. `os`/`filepath` appear only in `queryrefs_test.go` for fixture reads. |
| 3 | `internal/sqlscan` counts toward the coverage gate; own per-package coverage ≥ 80% | ✓ VERIFIED | Makefile `COVER_PKGS` exclusion = `internal/db/sqlc\|cmd/coverage-report\|cmd/migrate\|cmd/migration-check` — `internal/sqlscan` absent. `go test ./internal/sqlscan/ -covermode=set` → **92.0%**. Aggregate `make coverage-gate` → **90.39%** (req 80%), PASS. |
| 4 | Schema-qualified DDL name and query-side ref resolve to the same normalized key (CR-01 bug class structurally prevented) | ✓ VERIFIED | `Parse` stores `Name = NormalizeIdent(StripSchemaQualifier(...))` on DDL side. `RefSet.Lookup`/`LookupAnyColumn` run the table arg through `NormalizeIdent(StripSchemaQualifier(...))` (queryrefs.go:59,73). `TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed` (`DROP TABLE public.events` vs query on `events`) passes; `TestQueryColumnRefs` asserts `Lookup("public.events", …)`. |
| 5 | `crossReferenceFinding` recovers a renamed column's old name from a typed field, not a re-parsed display string (WR-01 resolved) | ✓ VERIFIED | main.go:722–753: reads `f.object` directly (populated from `sqlscan.RenameColumn.From` at main.go:587). `grep -vE '^\s*//' main.go \| grep -c 'strings.Cut'` → 0. No `strings.Cut` anywhere in main.go. |
| 6 | Every finding message renders from raw (as-written) spellings, so normalizing the parse layer moves no output byte | ✓ VERIFIED | `finding` gains `rawTable`/`rawObject` (main.go:407–408); `describe()` (main.go:469–483) renders `f.rawTable`/`f.rawObject` exclusively. For `rename_column`, `rawObject = act.RawFrom + " -> " + act.RawTo` (main.go:587). Golden unchanged (truth 1). |
| 7 | All git seams / allowlists / annotation grammar / finding struct + renderers stay in `cmd/migration-check`; `internal/sqlscan` reads no files, spawns no subprocess | ✓ VERIFIED | `gitShow`, `readAtTag`, `allowedGitShowPaths`, `pathAllowedForGitShow`, `reTagShape`, `prevReleaseQueryFiles`, `reAnnotation*` all present in main.go. sqlscan non-test code imports no `os`/`exec`/`filepath` (truth 2). |
| 8 | `.golangci.yml` grants `internal/sqlscan` no path-scoped carve-out; gosec G304 stays scoped to `cmd/migration-check` | ✓ VERIFIED | `git diff c549f46..HEAD -- .golangci.yml Makefile` empty (both untouched). `.golangci.yml` has no `sqlscan` match; G304 exclusion is `- path: '^cmd/migration-check/'`. |

**Score:** 8/8 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/sqlscan/lex.go` | Lexer: StripComments, SplitStatements, SplitTopLevelCommas, RawStatement | ✓ VERIFIED | 238 lines; exported surface matches design contract |
| `internal/sqlscan/ident.go` | NormalizeIdent, StripSchemaQualifier, StripIdent | ✓ VERIFIED | 32 lines |
| `internal/sqlscan/parse.go` | Sealed Statement/Action, 10 concrete types, Parse, SchemaColumns | ✓ VERIFIED | 215 lines; all types + marker methods match contract (parse.go:11–67) |
| `internal/sqlscan/queryrefs.go` | QueryColumnRefs, RefSet + Merge/Lookup/LookupAnyColumn/HasLow | ✓ VERIFIED | 386 lines |
| `internal/sqlscan/{lex,parse,queryrefs,internal}_test.go` | Moved + new blackbox/whitebox suites | ✓ VERIFIED | 20 Test funcs incl. dollar-quote coverage, RefSet merge/lookup, whitebox `dollarTagAt`/`copyDollarQuoted`/`findFromJoinTables` |
| `internal/sqlscan/testdata/prevrelease/` | 6 copied fixtures | ✓ VERIFIED | artists, events, params, low_confidence_join, schema_qualified, case_folding |
| `cmd/migration-check/testdata/prevrelease/` | 3 unused fixtures deleted | ✓ VERIFIED | only artists, events, low_confidence_join remain |
| 2 follow-up todo files | Valid frontmatter (created/title/area/severity/files) | ✓ VERIFIED | `2026-09-05-unify-sqlscan-quote-state-machines.md`, `2026-09-05-resolve-d15-prev-release-query-files-from-prev-tag.md` — both match the reference todo shape |
| `cmd/migration-check/main.go` | Reduced to policy + I/O | ✓ VERIFIED | 1469 → 753 lines; only 5 regexes remain (branch ref, 3 annotation, tag shape) — no DDL/query regex, no lexer, no classifiers |

### Key Link Verification

| From | To | Status | Details |
|------|----|--------|---------|
| `sqlscan.Parse` → `Statement`/`Action` type switch → `findingClass` | rollback-rule seam | ✓ WIRED | `scanFile` loops `sqlscan.Parse`, switches over `sqlscan.DropTable`/`AlterTable`/`Action` types (main.go:564–594) |
| `SchemaColumns` → `QueryColumnRefs` star expansion → `RefSet` → `crossReferenceFinding` | D-15 chain | ✓ WIRED | `buildPrevReleaseRefs` → `sqlscan.SchemaColumns(sqlscan.Parse(...))` + per-file `QueryColumnRefs`; `crossReferenceFinding` uses `refs.Lookup`/`LookupAnyColumn` |
| Normalized identifiers on both DDL + query-ref side → single lookup key | CR-01 fix | ✓ WIRED | Both sides normalize + schema-strip (truth 4) |
| Makefile `COVER_PKGS` → `internal/sqlscan` inside denominator | coverage gate | ✓ WIRED | Not in exclusion list; gate passes at 90.39% |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full migration-check + sqlscan suite | `go test ./cmd/migration-check/ ./internal/sqlscan/ -count=1` | both `ok` | ✓ PASS |
| sqlscan per-package coverage | `go test ./internal/sqlscan/ -covermode=set` | 92.0% | ✓ PASS |
| Aggregate backend coverage gate | `make coverage-gate` (non-race path) | 90.39%, PASS | ✓ PASS |
| Full non-race integration suite | `go test ./... -count=1 -p 1 -coverpkg=…` | all packages `ok` | ✓ PASS |
| Static analysis | `go vet ./...` | exit 0 | ✓ PASS |
| Lint | `golangci-lint run ./internal/sqlscan/... ./cmd/migration-check/...` | 0 issues | ✓ PASS |
| sqlc drift | `make sqlc-check` | no drift, exit 0 | ✓ PASS |
| Golden byte-identity at every commit | `git rev-parse <commit>:<golden>` ×5 | all `74a4b02c…` | ✓ PASS |
| No AI attribution in commits | `git log --format=%B c549f46..HEAD \| grep -icE 'co-authored-by\|claude-session\|generated with'` | 0 | ✓ PASS |

Note: `-race` unavailable on this Windows box (documented cgo/ThreadSanitizer break, Phase 11.1-04 / 15-02). The non-race equivalent from the plan `<execution_notes>` was used, as the executor did. Not treated as a gap.

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| 16-ARCH-C1 | Extract `internal/sqlscan` (Phase 16 arch-review candidate 1) | ✓ SATISFIED | Package exists with settled exported surface; main.go halved |
| 16-REVIEW-CR-01 | Schema-qualified table name cannot bypass D-15 cross-reference | ✓ SATISFIED | Structurally prevented (truth 4); regression test green |
| 16-REVIEW-WR-01 | Renamed column old-name recovered from typed field, not display-string re-parse | ✓ SATISFIED | `RenameColumn{From,To}` typed field; no `strings.Cut` (truth 5) |

### Anti-Patterns Found

None. No `TODO`/`FIXME`/`XXX`/`HACK` markers introduced in modified source. No stub returns, no hardcoded empty data in rendering paths. Comments in moved code compressed per the plan's comment-discipline note.

### Accepted Behavior Deltas (both strictly narrowing, invisible to every test + golden)

1. `ADD CONSTRAINT x CHECK (...)` no longer leaks a phantom `constraint` column into the schema-column set (`AddCheck` action; `SchemaColumns` reads `AddColumn` only). Verified by `TestParse_AddCheckNeverAddColumn`.
2. `crossReferenceFinding` no longer calls `StripSchemaQualifier` itself — the identifier arrives normalized from `sqlscan`. Verified: no `StripSchemaQualifier` call anywhere in main.go.

### Human Verification Required

None. All truths verified programmatically.

## Gaps Summary

No gaps. This is a clean behavior-preserving refactor:

- The golden file is provably byte-identical (same git blob hash) at all four task commits and the base — the strongest possible evidence for a behavior-preserving refactor.
- `internal/sqlscan` is stdlib-only, sits inside the coverage denominator, and self-covers at 92.0%.
- CR-01's bug class and WR-01 are structurally retired by the typed model (normalized identifiers on both sides of the D-15 lookup; typed `RenameColumn{From,To}`), documented in the SUMMARY with no todos filed for them — exactly as the plan directs.
- The two genuine follow-ups (unify the quote state machines; resolve D-15 prev-release files from `--prev-tag`) are filed in `.planning/todos/pending/` with valid frontmatter.
- Full Definition of Done passes: `go vet`, `golangci-lint` (0 issues), `make test` (non-race fallback, documented), `make coverage-gate` (90.39%), `make sqlc-check`.
- No commit carries an AI-attribution trailer.

Minor observation (not a gap): intermediate-commit build/test greenness was not independently re-verified via checkout; HEAD (the cumulative result) is fully green and the golden blob is identical at each commit, so bisectability of the behavior contract holds.

---

_Verified: 2026-09-05_
_Verifier: Claude (gsd-verifier)_
